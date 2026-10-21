package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReservedPoints includes uncertain attempts. Only a verified platform receipt
// can establish delivery; reservation is not proof of an actual deduction.
type OutreachAuthorization struct {
	JobID          string `json:"jobId"`
	JobTitle       string `json:"jobTitle"`
	PerPersonLimit int    `json:"perPersonLimit"`
	TotalLimit     int    `json:"totalLimit"`
	ReservedPoints int    `json:"reservedPoints"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	ConfirmedAt    string `json:"confirmedAt"`
}

func (a *App) loadOutreachAuthorization(batchID string) (*OutreachAuthorization, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var auth OutreachAuthorization
	err = db.QueryRow(`SELECT job_id,job_title,per_person_limit,total_limit,reserved_points,status,message,confirmed_at FROM outreach_authorizations WHERE batch_id=?`, batchID).
		Scan(&auth.JobID, &auth.JobTitle, &auth.PerPersonLimit, &auth.TotalLimit, &auth.ReservedPoints, &auth.Status, &auth.Message, &auth.ConfirmedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &auth, err
}

func (a *App) outreachItemBatchID(itemID string) (string, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return "", err
	}
	var id string
	err = db.QueryRow(`SELECT batch_id FROM outreach_items WHERE id=?`, itemID).Scan(&id)
	return id, err
}

func (a *App) pauseOutreachBatch(batchID, message string) {
	if db, err := a.getRecruitmentDB(); err == nil {
		_, _ = db.Exec(`UPDATE outreach_authorizations SET status='paused',message=? WHERE batch_id=?`, message, batchID)
	}
}

func (a *App) ListOutreachCommunicationJobs(batchID, keyword string) JobConversationResult {
	batch, err := a.GetOutreachBatch(batchID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}

	// 1. 优先从本地 SQLite 缓存读取，实现秒级展示，不阻塞在候选人网页加载
	cachedJobs, _ := a.ListCachedCommunicationJobs("51job", "")
	if len(cachedJobs) > 0 {
		var matched []JobCommunicationOption
		kw := strings.TrimSpace(keyword)
		for _, job := range cachedJobs {
			if kw == "" || strings.Contains(job.Title, kw) || strings.Contains(job.ID, kw) {
				matched = append(matched, job)
			}
		}
		if len(matched) > 0 {
			suggested := a.suggestedProjectCommunicationJob(batch.ProjectID, batch, matched)
			return JobConversationResult{
				Status:         "jobs",
				Jobs:           matched,
				SuggestedJobID: suggested,
				Message:        fmt.Sprintf("已从本地目录加载 %d 个可选沟通职位", len(matched)),
			}
		}
	}

	// 2. 本地尚无缓存时，尝试通过候选人上下文在线拉取并自动持久化至本地
	for _, item := range batch.Items {
		if item.Platform != "51job" || (item.Status != "draft" && item.Status != "queued" && item.Status != "awaiting_confirmation") {
			continue
		}
		if _, err := a.verified51JobOutreachResume(item); err != nil {
			continue
		}
		result := a.List51JobCommunicationJobs(item.ResumeID, keyword)
		if result.Status == "jobs" && len(result.Jobs) > 0 {
			_ = a.SaveCommunicationJobs("51job", "", result.Jobs)
			now := time.Now().UTC().Format(time.RFC3339)
			_ = a.SaveCommunicationJobSyncSnapshot(&CommunicationJobSyncSnapshot{
				ID:           fmt.Sprintf("sync_%d", time.Now().UnixNano()),
				Platform:     "51job",
				ScopeAccount: "",
				Status:       "complete",
				TotalJobs:    len(result.Jobs),
				StartedAt:    now,
				FinishedAt:   now,
			})
			result.SuggestedJobID = a.suggestedProjectCommunicationJob(batch.ProjectID, batch, result.Jobs)
		}
		return result
	}

	if len(cachedJobs) > 0 {
		return JobConversationResult{Status: "jobs", Jobs: []JobCommunicationOption{}, Message: "未找到匹配关键词的职位"}
	}
	return JobConversationResult{Status: "error", Message: "本地尚未同步 51job 职位目录，且本批暂无可用候选人触发同步；请在 51job 浏览器中登录并同步职位"}
}

func suggestedOutreachJob(batch OutreachBatch, jobs []JobCommunicationOption) string {
	if batch.Authorization != nil {
		for _, job := range jobs {
			if job.ID == batch.Authorization.JobID {
				return job.ID
			}
		}
	}
	mappings := map[string]bool{}
	for _, item := range batch.Items {
		if item.Platform == "51job" && item.PlatformJobID != "" {
			mappings[item.PlatformJobID] = true
		}
	}
	if len(mappings) == 1 {
		for _, job := range jobs {
			if mappings[job.ID] {
				return job.ID
			}
		}
	}
	match := ""
	for _, job := range jobs {
		if strings.TrimSpace(job.Title) == strings.TrimSpace(batch.ProjectTitle) {
			if match != "" {
				return ""
			}
			match = job.ID
		}
	}
	return match
}

// HR authorizes one job and two ceilings for this batch only. Application
// routing mappings are deliberately independent of this temporary selection.
func (a *App) AuthorizeOutreachBudget(batchID, jobID, jobTitle string, perPerson, total int) (OutreachBatch, error) {
	a.outreachMutex.Lock()
	defer a.outreachMutex.Unlock()
	batch, err := a.GetOutreachBatch(batchID)
	if err != nil {
		return batch, err
	}
	if batch.Status != "confirmed" {
		return batch, fmt.Errorf("请先确认本批名单与文案")
	}
	if perPerson < 1 || total < 1 || perPerson > 1000000 || total > 10000000 {
		return batch, fmt.Errorf("请填写有效的单人及本批点数上限")
	}
	for _, item := range batch.Items {
		if item.Status == "sending" || item.Status == "partial" || item.Status == "uncertain" {
			return batch, fmt.Errorf("本批存在发送结果待核查的人选，请先核对并登记平台结果")
		}
	}
	// 校验所选职位：从本地 communication_jobs 目录校验，解耦对在线网页重拉的依赖
	cachedJobs, _ := a.ListCachedCommunicationJobs("51job", "")
	valid := false
	var companyID string
	if len(cachedJobs) > 0 {
		for _, job := range cachedJobs {
			if job.ID == jobID && (strings.TrimSpace(job.Title) == strings.TrimSpace(jobTitle) || jobTitle == "") {
				valid = true
				companyID = job.CompanyID
				break
			}
		}
	} else {
		// 容错：首次或离线场景下，只要 HR 明确传入了有效 jobID 和 jobTitle 即视为结构有效
		if strings.TrimSpace(jobID) != "" && strings.TrimSpace(jobTitle) != "" {
			valid = true
		}
	}
	if !valid {
		return batch, fmt.Errorf("所选职位未在有效职位列表中，请刷新并重新选择")
	}

	// 记录项目默认沟通职位，便于同项目后续批次优先预选
	if batch.ProjectID != "" {
		_ = a.SetProjectDefaultCommunicationJob(batch.ProjectID, "51job", "", jobID, jobTitle, companyID)
	}
	reserved := 0
	if batch.Authorization != nil {
		reserved = batch.Authorization.ReservedPoints
		if reserved > 0 && batch.Authorization.JobID != jobID {
			return batch, fmt.Errorf("本批已开始发送，不能更换沟通职位；请另建批次")
		}
	}
	if total < reserved {
		return batch, fmt.Errorf("本批上限不能低于已占用的 %d 点", reserved)
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return batch, err
	}
	_, err = db.Exec(`INSERT INTO outreach_authorizations(batch_id,job_id,job_title,per_person_limit,total_limit,reserved_points,status,message,confirmed_at)
		VALUES(?,?,?,?,?,?,'active','',?) ON CONFLICT(batch_id) DO UPDATE SET job_id=excluded.job_id,job_title=excluded.job_title,
		per_person_limit=excluded.per_person_limit,total_limit=excluded.total_limit,status='active',message='',confirmed_at=excluded.confirmed_at`,
		batchID, jobID, jobTitle, perPerson, total, reserved, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return batch, err
	}
	return a.GetOutreachBatch(batchID)
}

func checkOutreachBudget(auth *OutreachAuthorization, quote pending51JobHiQuote) error {
	if auth == nil || auth.Status != "active" {
		return fmt.Errorf("请确认本批沟通职位与点数预算后继续")
	}
	if quote.JobID != auth.JobID || quote.JobTitle != auth.JobTitle {
		return fmt.Errorf("平台沟通职位已变化，请重新确认本批职位")
	}
	if quote.Points <= 0 || quote.Points > auth.PerPersonLimit || quote.Points > auth.TotalLimit-auth.ReservedPoints {
		return fmt.Errorf("本次 %d 点超出单人上限 %d 或本批剩余 %d 点；请重新确认预算", quote.Points, auth.PerPersonLimit, auth.TotalLimit-auth.ReservedPoints)
	}
	return nil
}

// A previous version treated read timeouts as manual-only. HR may explicitly
// return these unsent items to the queue, never a send with an unclear receipt.
func (a *App) RequeueOutreachReadFailure(itemID string) (OutreachItem, error) {
	a.outreachMutex.Lock()
	defer a.outreachMutex.Unlock()
	item, err := a.loadOutreachItem(itemID)
	if err != nil {
		return item, err
	}
	readFailure := false
	for _, word := range []string{"waiting failed", "timeout", "timed out", "超时", "页面", "职位"} {
		if strings.Contains(strings.ToLower(item.Result), word) {
			readFailure = true
		}
	}
	if item.Status != "needs_manual" || !readFailure || item.ReservedPoints != 0 {
		return item, fmt.Errorf("该记录不能作为读取失败重试，请核对平台结果")
	}
	resume, err := a.verified51JobOutreachResume(item)
	if err != nil {
		return item, err
	}
	if resume.ContactStatus != "" {
		return item, fmt.Errorf("候选人已有沟通记录，请先核对平台会话")
	}
	return a.setOutreachItemStatus(item, "queued", "读取阶段未发送，等待 HR 重新确认本批职位与预算")
}

// Commit budget reservation and in-flight status before any external send.
// A crash after this point requires manual receipt reconciliation, not replay.
func (a *App) reserveOutreachPoints(batchID string, item OutreachItem, quote pending51JobHiQuote) (OutreachItem, error) {
	auth, err := a.loadOutreachAuthorization(batchID)
	if err != nil {
		return item, err
	}
	if err = checkOutreachBudget(auth, quote); err != nil {
		return item, err
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return item, err
	}
	tx, err := db.Begin()
	if err != nil {
		return item, err
	}
	defer tx.Rollback()
	item.Status, item.ReservedPoints = "sending", quote.Points
	item.Result = "已占用本批点数预算，正在核对发送回执；中断后请核查平台记录"
	raw, _ := json.Marshal(item)
	result, err := tx.Exec(`UPDATE outreach_authorizations SET reserved_points=reserved_points+? WHERE batch_id=? AND status='active' AND job_id=? AND job_title=? AND per_person_limit>=? AND total_limit-reserved_points>=?`, quote.Points, batchID, quote.JobID, quote.JobTitle, quote.Points, quote.Points)
	if err != nil {
		return item, err
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return item, fmt.Errorf("本批预算已变化，请重新确认")
	}
	result, err = tx.Exec(`UPDATE outreach_items SET status='sending',result_json=?,updated_at=? WHERE id=? AND status='awaiting_confirmation'`, string(raw), time.Now().UTC().Format(time.RFC3339Nano), item.ID)
	if err != nil {
		return item, err
	}
	n, err = result.RowsAffected()
	if err != nil || n != 1 {
		return item, fmt.Errorf("该候选人已处理，未重复发送")
	}
	return item, tx.Commit()
}
