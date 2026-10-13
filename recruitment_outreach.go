package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type OutreachItem struct {
	ID                  string `json:"id"`
	ProjectCandidateID  string `json:"projectCandidateId"`
	CandidateName       string `json:"candidateName"`
	Platform            string `json:"platform"`
	PlatformCandidateID string `json:"platformCandidateId"`
	PlatformJobID       string `json:"platformJobId"`
	ProfileURL          string `json:"profileUrl"`
	Message             string `json:"message"`
	Evidence            string `json:"evidence"`
	RedLineStatus       string `json:"redLineStatus"`
	AbilityScore        int    `json:"abilityScore"`
	Rank                int    `json:"rank"`
	Status              string `json:"status"`
	Result              string `json:"result"`
}

type OutreachBatch struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"projectId"`
	ProjectName string         `json:"projectName"`
	Status      string         `json:"status"`
	Items       []OutreachItem `json:"items"`
	CreatedAt   string         `json:"createdAt"`
	ConfirmedAt string         `json:"confirmedAt"`
}

type OutreachApproval struct {
	ItemID         string `json:"itemId"`
	Message        string `json:"message"`
	Include        bool   `json:"include"`
	ConfirmPending bool   `json:"confirmPending"`
}

func outreachFact(resume *Resume) string {
	if resume == nil {
		return ""
	}
	for _, line := range strings.Split(resume.Content, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"任职履历快照：", "【核心专业技能】", "• "} {
			if strings.HasPrefix(line, prefix) {
				fact := strings.TrimSpace(strings.TrimPrefix(line, prefix))
				if len([]rune(fact)) >= 6 && len([]rune(fact)) <= 70 && !strings.Contains(fact, "详见") && !strings.Contains(fact, "平台默认") {
					return fact
				}
			}
		}
	}
	return ""
}

func greetingMessage(name, jobTitle, fact string) string {
	if fact != "" {
		return fmt.Sprintf("您好%s，看到您简历中提到“%s”。透景正在招聘%s，想与您交流岗位情况，方便了解一下吗？", name, fact, jobTitle)
	}
	return fmt.Sprintf("您好%s，透景正在招聘%s，想与您交流岗位情况，方便了解一下吗？", name, jobTitle)
}

// The preview is a ranking snapshot. No platform message is sent here.
func (a *App) PreviewTopTenOutreach(projectID string) (OutreachBatch, error) {
	batch := OutreachBatch{ProjectID: projectID, Status: "draft", Items: make([]OutreachItem, 0)}
	project := a.GetProject(projectID)
	if project == nil {
		return batch, fmt.Errorf("招聘项目不存在")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return batch, err
	}
	batch.ProjectName = project.Name
	batch.ID = recruitmentID("outreach")
	batch.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	resumes := a.GetProjectRanking(projectID)
	seenLinks := make(map[string]bool)
	for _, resume := range resumes {
		if len(batch.Items) >= 10 {
			break
		}
		if resume.Status != "done" || resume.Analysis == nil || resume.Analysis.RedLineStatus == "failed" {
			continue
		}
		var linkID, sourcePlatform, sourceID string
		err := db.QueryRow(`SELECT pc.id,cs.platform,cs.platform_candidate_id FROM project_candidates pc
			JOIN resume_versions rv ON rv.candidate_id=pc.candidate_id
			LEFT JOIN candidate_sources cs ON cs.id=rv.source_id
			WHERE pc.project_id=? AND rv.legacy_resume_id=?`, projectID, resume.ID).Scan(&linkID, &sourcePlatform, &sourceID)
		if err != nil {
			continue
		}
		if seenLinks[linkID] {
			continue
		}
		seenLinks[linkID] = true
		if sourcePlatform == "legacy" {
			sourcePlatform = ""
			sourceID = ""
		}
		var jobID string
		_ = db.QueryRow(`SELECT platform_job_id FROM platform_job_mappings WHERE platform=? AND project_id=? LIMIT 1`, sourcePlatform, projectID).Scan(&jobID)
		name := conversationCandidateName(resume)
		if resume.Analysis.CandidateName != "" {
			name = resume.Analysis.CandidateName
		}
		fact := outreachFact(resume)
		item := OutreachItem{ID: recruitmentID("outreach_item"), ProjectCandidateID: linkID, CandidateName: name,
			Platform: sourcePlatform, PlatformCandidateID: sourceID, PlatformJobID: jobID, ProfileURL: resume.URL,
			Message: greetingMessage(name, project.JobConfig.Title, fact), Evidence: fact,
			RedLineStatus: resume.Analysis.RedLineStatus, AbilityScore: resume.Analysis.AbilityScore,
			Rank: len(batch.Items) + 1, Status: "draft"}
		batch.Items = append(batch.Items, item)
	}
	if len(batch.Items) == 0 {
		return batch, fmt.Errorf("当前项目没有已完成分析且红线未确认不满足的候选人")
	}
	tx, err := db.Begin()
	if err != nil {
		return batch, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO outreach_batches(id,project_id,status,created_at) VALUES(?,?,'draft',?)`, batch.ID, projectID, batch.CreatedAt); err != nil {
		return batch, err
	}
	for _, item := range batch.Items {
		meta, _ := json.Marshal(item)
		if _, err = tx.Exec(`INSERT INTO outreach_items(id,batch_id,project_candidate_id,platform,platform_candidate_id,platform_job_id,message,status,result_json,updated_at)
			VALUES(?,?,?,?,?,?,?,'draft',?,?)`, item.ID, batch.ID, item.ProjectCandidateID, item.Platform, item.PlatformCandidateID, item.PlatformJobID, item.Message, string(meta), batch.CreatedAt); err != nil {
			return batch, err
		}
	}
	return batch, tx.Commit()
}

func (a *App) GetOutreachBatch(batchID string) (OutreachBatch, error) {
	batch := OutreachBatch{Items: make([]OutreachItem, 0)}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return batch, err
	}
	err = db.QueryRow(`SELECT b.id,b.project_id,p.name,b.status,b.created_at,b.confirmed_at FROM outreach_batches b JOIN projects p ON p.id=b.project_id WHERE b.id=?`, batchID).
		Scan(&batch.ID, &batch.ProjectID, &batch.ProjectName, &batch.Status, &batch.CreatedAt, &batch.ConfirmedAt)
	if err != nil {
		return batch, err
	}
	rows, err := db.Query(`SELECT id,message,status,result_json FROM outreach_items WHERE batch_id=? ORDER BY updated_at,id`, batchID)
	if err != nil {
		return batch, err
	}
	defer rows.Close()
	for rows.Next() {
		var item OutreachItem
		var raw string
		if err := rows.Scan(&item.ID, &item.Message, &item.Status, &raw); err != nil {
			return batch, err
		}
		var meta OutreachItem
		if json.Unmarshal([]byte(raw), &meta) == nil {
			meta.ID = item.ID
			meta.Message = item.Message
			meta.Status = item.Status
			item = meta
		}
		batch.Items = append(batch.Items, item)
	}
	sort.Slice(batch.Items, func(i, j int) bool { return batch.Items[i].Rank < batch.Items[j].Rank })
	return batch, rows.Err()
}

// All current platform first-message adapters lack verified personalized-send
// receipts. Confirmation therefore records each chosen item as requiring
// manual handling; it never converts a click or default greeting into success.
func (a *App) ConfirmOutreachBatch(batchID string, approvals []OutreachApproval) (OutreachBatch, error) {
	batch, err := a.GetOutreachBatch(batchID)
	if err != nil {
		return batch, err
	}
	if batch.Status != "draft" {
		return batch, fmt.Errorf("批次已确认，不可重复发送")
	}
	if len(approvals) != len(batch.Items) {
		return batch, fmt.Errorf("请逐人确认本批全部候选人")
	}
	byID := make(map[string]OutreachItem)
	for _, item := range batch.Items {
		byID[item.ID] = item
	}
	seen := make(map[string]bool)
	db, err := a.getRecruitmentDB()
	if err != nil {
		return batch, err
	}
	tx, err := db.Begin()
	if err != nil {
		return batch, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, approval := range approvals {
		item, ok := byID[approval.ItemID]
		if !ok || seen[approval.ItemID] {
			return batch, fmt.Errorf("批次候选人列表不匹配")
		}
		seen[approval.ItemID] = true
		status, result := "excluded", "HR 未选择发送"
		if approval.Include {
			if item.RedLineStatus == "pending" && !approval.ConfirmPending {
				return batch, fmt.Errorf("候选人 %s 的红线待核实，发送前须单独勾选", item.CandidateName)
			}
			if strings.TrimSpace(approval.Message) == "" {
				return batch, fmt.Errorf("候选人 %s 的招呼语为空", item.CandidateName)
			}
			var sent int
			err = tx.QueryRow(`SELECT COUNT(*) FROM outreach_items oi JOIN outreach_batches b ON b.id=oi.batch_id
				WHERE b.project_id=? AND oi.status='sent' AND
				(oi.project_candidate_id=? OR (?<>'' AND oi.platform=? AND oi.platform_candidate_id=? AND oi.platform_job_id=?))`,
				batch.ProjectID, item.ProjectCandidateID, item.PlatformCandidateID, item.Platform, item.PlatformCandidateID, item.PlatformJobID).Scan(&sent)
			if err != nil {
				return batch, err
			}
			if sent > 0 {
				return batch, fmt.Errorf("候选人 %s 已有成功沟通记录，请核对平台会话", item.CandidateName)
			}
			status, result = "needs_manual", "平台首次定制招呼缺少可核实的自动发送能力；请在平台逐人发送并登记回执"
			if item.PlatformCandidateID == "" {
				result = "缺少稳定的平台候选人 ID；请人工核对目标后发送"
			}
		}
		meta := item
		meta.Message = approval.Message
		meta.Status = status
		meta.Result = result
		raw, _ := json.Marshal(meta)
		if _, err = tx.Exec(`UPDATE outreach_items SET message=?,status=?,result_json=?,updated_at=? WHERE id=?`, approval.Message, status, string(raw), now, item.ID); err != nil {
			return batch, err
		}
	}
	if _, err = tx.Exec(`UPDATE outreach_batches SET status='confirmed',confirmed_at=? WHERE id=?`, now, batchID); err != nil {
		return batch, err
	}
	if err = tx.Commit(); err != nil {
		return batch, err
	}
	return a.GetOutreachBatch(batchID)
}

func (a *App) RecordOutreachResult(itemID, status, platformReceipt, actor string) error {
	if status != "sent" && status != "failed" && status != "uncertain" {
		return fmt.Errorf("结果状态无效")
	}
	if strings.TrimSpace(platformReceipt) == "" || strings.TrimSpace(actor) == "" {
		return fmt.Errorf("请提供平台回执和登记人")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw, current, projectID string
	err = tx.QueryRow(`SELECT oi.result_json,oi.status,b.project_id FROM outreach_items oi JOIN outreach_batches b ON b.id=oi.batch_id WHERE oi.id=?`, itemID).Scan(&raw, &current, &projectID)
	if err != nil {
		return err
	}
	if current != "needs_manual" && current != "uncertain" {
		return fmt.Errorf("当前状态不可登记发送结果")
	}
	var item OutreachItem
	if err = json.Unmarshal([]byte(raw), &item); err != nil {
		return err
	}
	if status == "sent" {
		var prior int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM outreach_items oi JOIN outreach_batches b ON b.id=oi.batch_id
			WHERE b.project_id=? AND oi.id<>? AND oi.project_candidate_id=? AND oi.status='sent'`, projectID, itemID, item.ProjectCandidateID).Scan(&prior); err != nil {
			return err
		}
		if prior > 0 {
			return fmt.Errorf("该候选人已有成功沟通记录，请核对平台会话")
		}
	}
	item.Status = status
	item.Result = fmt.Sprintf("%s 登记平台回执：%s", strings.TrimSpace(actor), strings.TrimSpace(platformReceipt))
	encoded, _ := json.Marshal(item)
	result, err := tx.Exec(`UPDATE outreach_items SET status=?,result_json=?,updated_at=? WHERE id=? AND status IN ('needs_manual','uncertain')`,
		status, string(encoded), time.Now().UTC().Format(time.RFC3339Nano), itemID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return sql.ErrNoRows
	}
	if _, err = tx.Exec(`INSERT INTO follow_up_events(id,project_candidate_id,event_type,note,actor,created_at)
		VALUES(?,?,?,?,?,?)`, recruitmentID("followup"), item.ProjectCandidateID, "communication:platform", item.Result, strings.TrimSpace(actor), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}
