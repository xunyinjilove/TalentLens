package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type telescopeCandidate struct {
	CandidateID    string `json:"candidateId"`
	RecordID       string `json:"recordId"`
	JobID          string `json:"jobId"`
	PlatformBucket string `json:"platformBucket"`
	Name           string `json:"name"`
	ResumeText     string `json:"resumeText"`
	ActivityLabel  string `json:"activityLabel"`
	AlreadyHi      bool   `json:"alreadyHi"`
}

type telescopePage struct {
	Status       string               `json:"status"`
	Message      string               `json:"message"`
	JobID        string               `json:"jobId"`
	JobTitle     string               `json:"jobTitle"`
	CompanyID    string               `json:"companyId"`
	AccountScope string               `json:"accountScope"`
	Total        int                  `json:"total"`
	Candidates   []telescopeCandidate `json:"candidates"`
}

type telescopeDetail struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	CandidateID string `json:"candidateId"`
	JobID       string `json:"jobId"`
	Name        string `json:"name"`
	ResumeText  string `json:"resumeText"`
	ProfileURL  string `json:"profileUrl"`
}

func readTelescopeDetail(jobID, candidateID string) (telescopeDetail, error) {
	result := telescopeDetail{}
	script := resolveTelescopeScript()
	if script == "" {
		return result, fmt.Errorf("缺少望远镜读取脚本")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 22*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script, "--mode", "detail", "--job-id", jobID, "--candidate-id", candidateID)
	hideConsoleWindow(cmd)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return result, fmt.Errorf("读取详情超时")
	}
	if json.Unmarshal(out, &result) != nil || result.Status != "ok" || result.JobID != jobID || result.CandidateID != candidateID || result.ResumeText == "" {
		if result.Message != "" {
			return result, fmt.Errorf("%s", result.Message)
		}
		return result, fmt.Errorf("平台详情身份或内容无法核验: %v", err)
	}
	return result, nil
}

type telescopeCatalogResult struct {
	Status         string                `json:"status"`
	Message        string                `json:"message"`
	AccountScope   string                `json:"accountScope"`
	ActiveJobID    string                `json:"activeJobId"`
	ActiveJobTitle string                `json:"activeJobTitle"`
	CompanyID      string                `json:"companyId"`
	Jobs           []TalentManagementJob `json:"jobs"`
}

type TelescopeRejectItem struct {
	CandidateID string `json:"candidateId"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
}

type TelescopeRejectResult struct {
	Total     int                   `json:"total"`
	Rejected  int                   `json:"rejected"`
	Skipped   int                   `json:"skipped"`
	Uncertain int                   `json:"uncertain"`
	Items     []TelescopeRejectItem `json:"items"`
}

// Open51JobTelescopeBrowser opens the platform's AI淘金/人才望远镜 page.
func (a *App) Open51JobTelescopeBrowser() error {
	if _, err := a.Get51JobTelescopeCatalog(); err != nil {
		return err
	}
	return a.activate51JobExistingPage("/Revision/talent/search-recommend")
}

func resolveTelescopeScript() string {
	for _, candidate := range []string{
		filepath.Join("scripts", "51job_telescope.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "51job_telescope.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "51job_telescope.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "51job_telescope.js"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// Get51JobTelescopeCatalog reads online positions and account info directly from the opened Telescope page.
func (a *App) Get51JobTelescopeCatalog() (*TalentManagementCatalog, error) {
	if !a.EnsurePlatformBrowser("51job") {
		return nil, fmt.Errorf("51job 专用浏览器未就绪")
	}
	script := resolveTelescopeScript()
	if script == "" {
		return nil, fmt.Errorf("缺少 51job 人才望远镜读取脚本")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script, "--mode", "catalog")
	hideConsoleWindow(cmd)
	out, err := cmd.Output()
	var res telescopeCatalogResult
	jsonErr := json.Unmarshal(out, &res)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("读取人才望远镜职位超时")
	}
	if jsonErr == nil && res.Status != "ok" && res.Message != "" {
		return nil, fmt.Errorf("%s", res.Message)
	}
	if err != nil {
		return nil, fmt.Errorf("读取人才望远镜职位失败: %w", err)
	}
	if jsonErr != nil {
		return nil, fmt.Errorf("人才望远镜目录回执无法解析: %w", jsonErr)
	}
	if res.Status != "ok" {
		return nil, fmt.Errorf("%s", res.Message)
	}
	return &TalentManagementCatalog{
		AccountScope: res.AccountScope,
		Jobs:         res.Jobs,
	}, nil
}

func readTelescopeCandidates(jobID string) (telescopePage, error) {
	result := telescopePage{}
	script := resolveTelescopeScript()
	if script == "" {
		return result, fmt.Errorf("缺少 51job 人才望远镜读取脚本")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script, "--job-id", jobID)
	hideConsoleWindow(cmd)
	out, err := cmd.Output()
	if jsonErr := json.Unmarshal(out, &result); jsonErr != nil {
		if ctx.Err() != nil {
			return result, fmt.Errorf("人才望远镜读取超时: %w", ctx.Err())
		}
		return result, fmt.Errorf("人才望远镜回执无法解析: %w", jsonErr)
	}
	if err != nil || result.Status != "ok" {
		if result.Message != "" {
			return result, fmt.Errorf("%s", result.Message)
		}
		return result, fmt.Errorf("人才望远镜读取失败: %w", err)
	}
	return result, nil
}

// Scan51JobTelescopeBatch reads the currently opened telescope page and uses
// the same project criteria as talent management. It never performs platform actions.
func (a *App) Scan51JobTelescopeBatch(accountScope, jobID, projectID string) (*ScreeningBatchPreview, error) {
	if jobID == "" || projectID == "" {
		return nil, fmt.Errorf("请先选择已绑定本地项目的 51job 职位")
	}

	page, err := readTelescopeCandidates(jobID)
	if err != nil {
		return nil, err
	}
	if page.JobID != jobID {
		return nil, fmt.Errorf("人才望远镜当前展示的职位与所选职位不一致，请在 51job 页面切换到对应职位")
	}
	if accountScope != "" && page.AccountScope != accountScope {
		return nil, fmt.Errorf("人才望远镜企业账号与当前选定账号不一致")
	}
	if accountScope == "" {
		if page.AccountScope != "" {
			accountScope = page.AccountScope
		} else if page.CompanyID != "" {
			accountScope = "co:" + page.CompanyID
		}
	}

	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	project := a.GetProject(projectID)
	if project == nil {
		return nil, fmt.Errorf("本地职位项目不存在：%s", projectID)
	}
	if err := upsertRecruitmentProject(db, project); err != nil {
		return nil, fmt.Errorf("同步本地职位项目失败: %w", err)
	}

	criteria, err := a.GetTalentManagementStrictCriteria(projectID)
	if err != nil {
		return nil, fmt.Errorf("读取项目筛选条件失败: %w", err)
	}
	if criteria == nil {
		return nil, fmt.Errorf("项目筛选条件尚未设置")
	}
	telescopeConfig, err := a.GetTelescopeConditionConfig(projectID)
	if err != nil {
		return nil, err
	}
	combinedVersion := telescopeHash([]string{criteria.CriteriaVersion, telescopeConfig.Version})

	now := time.Now()
	preview := &ScreeningBatchPreview{
		BatchID: recruitmentID("scrbatch"), ProjectID: projectID, Source: "telescope",
		Platform: "51job", PlatformAccount: accountScope, PlatformJobID: jobID,
		BusinessDate: now.Format("2006-01-02"), CriteriaVersion: combinedVersion,
		Status: "preview_ready", QualifiedList: []ScreeningCandidateResult{},
		UnfitList: []ScreeningCandidateResult{}, PendingList: []ScreeningCandidateResult{},
	}
	for index, cand := range page.Candidates {
		if cand.CandidateID == "" || cand.RecordID == "" || cand.JobID != jobID || cand.AlreadyHi {
			continue
		}
		var existing int
		if err := db.QueryRow(`SELECT COUNT(*) FROM platform_action_ledger WHERE project_id=? AND candidate_id=? AND status='success' AND action_type IN ('hi','reject')`, projectID, cand.CandidateID).Scan(&existing); err != nil {
			return nil, err
		}
		if existing > 0 {
			continue
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "telescope_scan_progress", map[string]interface{}{"current": index + 1, "total": len(page.Candidates), "name": cand.Name})
		}
		detail, detailErr := readTelescopeDetail(jobID, cand.CandidateID)
		profile := cand.ResumeText
		verdict := "pending_verification"
		checks := []RuleCheckItem{}
		if detailErr == nil {
			if cand.Name == "" || detail.Name == "" || (cand.Name != detail.Name &&
				!(len([]rune(cand.Name)) >= 2 && (strings.HasSuffix(cand.Name, "先生") || strings.HasSuffix(cand.Name, "女士")) && strings.HasPrefix(detail.Name, string([]rune(cand.Name)[0])))) {
				detailErr = fmt.Errorf("详情姓名与列表无法核对")
			}
		}
		if detailErr == nil {
			profile = detail.ResumeText
			verdict, checks = EvaluateCandidateTriState(profile, *criteria)
			requirementChecks := a.evaluateTelescopeConditions(db, projectID, cand.CandidateID, profile, telescopeConfig)
			checks = append(checks, requirementChecks...)
			verdict = telescopeVerdict(verdict, requirementChecks)
		} else {
			checks = append(checks, RuleCheckItem{Rule: "TELESCOPE_DETAIL", Label: "平台详情", Status: "unknown", Evidence: detailErr.Error()})
		}
		item := ScreeningCandidateResult{
			ID: recruitmentID("scr"), BatchID: preview.BatchID, ProjectID: projectID,
			CandidateID: cand.CandidateID, RecordID: cand.RecordID, CandidateName: cand.Name,
			Verdict: verdict, RuleEvidence: checks, RawProfile: profile,
			DetailLoaded: detailErr == nil, ActionStatus: "pending", PlatformStatus: "人才望远镜",
			CreatedAt: now.Format("2006-01-02 15:04:05"),
		}
		switch verdict {
		case "qualified":
			preview.QualifiedList = append(preview.QualifiedList, item)
		case "unfit":
			preview.UnfitList = append(preview.UnfitList, item)
		default:
			preview.PendingList = append(preview.PendingList, item)
		}
	}
	telescopeSortByReference(preview.QualifiedList)
	preview.QualifiedCount = len(preview.QualifiedList)
	preview.UnfitCount = len(preview.UnfitList)
	preview.PendingCount = len(preview.PendingList)
	preview.TotalCount = preview.QualifiedCount + preview.UnfitCount + preview.PendingCount
	criteriaJSON, err := json.Marshal(map[string]interface{}{"hard": criteria, "telescope": telescopeConfig})
	if err != nil {
		return nil, err
	}
	if err := persistScreeningBatch(db, preview, string(criteriaJSON), now.Format("2006-01-02 15:04:05")); err != nil {
		return nil, fmt.Errorf("保存望远镜预览失败: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO telescope_job_bindings(project_id,platform_account,platform_job_id,updated_at) VALUES(?,?,?,?)
		ON CONFLICT(project_id) DO UPDATE SET platform_account=excluded.platform_account,platform_job_id=excluded.platform_job_id,updated_at=excluded.updated_at`,
		projectID, accountScope, jobID, now.UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, fmt.Errorf("保存望远镜项目职位绑定失败: %w", err)
	}
	return preview, nil
}

// Run51JobTelescopeReject processes clear failures only. An ambiguous platform
// receipt remains uncertain and is never automatically clicked a second time.
func (a *App) Run51JobTelescopeReject(batchID string) (TelescopeRejectResult, error) {
	return a.run51JobTelescopeReject(batchID, false)
}

func (a *App) run51JobTelescopeReject(batchID string, historical bool) (TelescopeRejectResult, error) {
	platformAutoRejectMu.Lock()
	defer platformAutoRejectMu.Unlock()
	result := TelescopeRejectResult{Items: []TelescopeRejectItem{}}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return result, err
	}
	var projectID, account, jobID, version, source, scanStatus string
	err = db.QueryRow(`SELECT project_id,platform_account,platform_job_id,criteria_version,source,scan_status FROM screening_batches WHERE id=?`, batchID).
		Scan(&projectID, &account, &jobID, &version, &source, &scanStatus)
	if err != nil || source != "telescope" || scanStatus != "complete" {
		return result, fmt.Errorf("望远镜批次不存在或扫描未完成")
	}
	if !historical {
		criteria, err := a.GetTalentManagementStrictCriteria(projectID)
		config, configErr := a.GetTelescopeConditionConfig(projectID)
		if err != nil || criteria == nil || configErr != nil || config == nil || telescopeHash([]string{criteria.CriteriaVersion, config.Version}) != version {
			return result, fmt.Errorf("项目条件已变化，请重新筛选后再执行")
		}
	}
	page, err := readTelescopeCandidates(jobID)
	if err != nil || page.AccountScope != account || page.JobID != jobID {
		return result, fmt.Errorf("望远镜页面账号、职位或候选人列表未就绪；本次未操作平台")
	}
	present := make(map[string]telescopeCandidate, len(page.Candidates))
	for _, candidate := range page.Candidates {
		present[candidate.CandidateID] = candidate
	}
	rows, err := db.Query(`SELECT id,candidate_id,record_id,candidate_name,rule_evidence_json FROM screening_candidate_results WHERE batch_id=? AND verdict='unfit' AND action_status IN ('pending','receipt_pending','retrying','failed') ORDER BY created_at,id`, batchID)
	if err != nil {
		return result, err
	}
	type target struct{ id, candidateID, recordID, name, evidence string }
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.candidateID, &t.recordID, &t.name, &t.evidence); err != nil {
			rows.Close()
			return result, err
		}
		targets = append(targets, t)
	}
	rows.Close()
	script := resolveTelescopeScript()
	if script == "" {
		return result, fmt.Errorf("缺少望远镜平台操作脚本")
	}
	for _, t := range targets {
		result.Total++
		item := TelescopeRejectItem{CandidateID: t.candidateID, Name: t.name, Status: "skipped"}
		candidate, found := present[t.candidateID]
		if !found {
			item.Reason = "人才不在当前职位列表，已跳过；请重新扫描"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		var prior, priorError string
		priorErr := db.QueryRow(`SELECT status,error_message FROM platform_action_ledger WHERE project_id=? AND candidate_id=? AND action_type='reject' ORDER BY requested_at DESC LIMIT 1`, projectID, t.candidateID).Scan(&prior, &priorError)
		if priorErr != nil && priorErr != sql.ErrNoRows {
			return result, priorErr
		}
		if prior == "success" {
			item.Reason = "平台已成功处理，未重复点击"
			result.Skipped++
			_, _ = db.Exec(`UPDATE screening_candidate_results SET action_status='completed',action_error='' WHERE id=?`, t.id)
			result.Items = append(result.Items, item)
			continue
		}
		if candidate.PlatformBucket == "not_match" {
			// 51job keeps disliked candidates in its full 100-person list and moves
			// them to notMatchList. The old check wrongly required total removal.
			if prior == "uncertain" && strings.HasPrefix(priorError, "已点击不感兴趣") {
				item.Status, item.Reason = "rejected", "已核对平台：先前点击的人才现处于未匹配分组"
				result.Rejected++
				_, _ = db.Exec(`UPDATE platform_action_ledger SET status='success',receipt=?,error_message='' WHERE project_id=? AND candidate_id=? AND record_id=? AND action_type='reject'`, item.Reason, projectID, t.candidateID, t.recordID)
				_, _ = db.Exec(`UPDATE screening_candidate_results SET action_status='completed',action_receipt_at=?,action_error='' WHERE id=?`, time.Now().Format("2006-01-02 15:04:05"), t.id)
			} else {
				item.Reason = "平台已将该人才归入未匹配分组，当前没有不感兴趣按钮，已跳过点击"
				result.Skipped++
				_, _ = db.Exec(`UPDATE platform_action_ledger SET status='skipped',receipt=?,error_message='' WHERE project_id=? AND candidate_id=? AND record_id=? AND action_type='reject'`, item.Reason, projectID, t.candidateID, t.recordID)
				_, _ = db.Exec(`UPDATE screening_candidate_results SET action_status='verified_on_platform',action_receipt_at=?,action_error='' WHERE id=?`, time.Now().Format("2006-01-02 15:04:05"), t.id)
			}
			result.Items = append(result.Items, item)
			continue
		}
		var checks []RuleCheckItem
		_ = json.Unmarshal([]byte(t.evidence), &checks)
		reasonLabel := "其他原因"
		for _, check := range checks {
			if check.Status == "fail" {
				if check.Rule == "MIN_EDUCATION" {
					reasonLabel = "学历不匹配"
				}
				break
			}
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		ledgerID := recruitmentID("pal")
		_, err = db.Exec(`INSERT INTO platform_action_ledger(id,project_id,platform,platform_account,platform_job_id,candidate_id,record_id,candidate_name,action_type,confirmed_by,requested_at,receipt,status,error_message)
			VALUES(?,?,?,?,?,?,?,?,?,'自动筛选',?,'','executing','')
			ON CONFLICT(project_id,candidate_id,record_id,action_type) DO UPDATE SET
			requested_at=excluded.requested_at,status='executing',error_message='',receipt=''`, ledgerID, projectID, "51job", account, jobID, t.candidateID, t.recordID, t.name, "reject", now)
		if err != nil {
			item.Reason = "无法记录操作意图：" + err.Error()
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
		cmd := exec.CommandContext(ctx, "node", script, "--mode", "reject", "--job-id", jobID, "--candidate-id", t.candidateID, "--account-scope", account, "--reason-label", reasonLabel)
		hideConsoleWindow(cmd)
		out, runErr := cmd.Output()
		cancel()
		var receipt struct {
			Status       string `json:"status"`
			Verified     bool   `json:"verified"`
			Receipt      string `json:"receipt"`
			Message      string `json:"message"`
			AccountScope string `json:"accountScope"`
			JobID        string `json:"jobId"`
			CandidateID  string `json:"candidateId"`
		}
		parseErr := json.Unmarshal(out, &receipt)
		if runErr == nil && parseErr == nil && receipt.Status == "ok" && receipt.Verified &&
			receipt.AccountScope == account && receipt.JobID == jobID && receipt.CandidateID == t.candidateID {
			item.Status, item.Reason = "rejected", receipt.Receipt
			result.Rejected++
			_, _ = db.Exec(`UPDATE platform_action_ledger SET status='success',receipt=? WHERE project_id=? AND candidate_id=? AND record_id=? AND action_type='reject'`, item.Reason, projectID, t.candidateID, t.recordID)
			_, _ = db.Exec(`UPDATE screening_candidate_results SET action_status='completed',action_receipt_at=?,action_error='' WHERE id=?`, now, t.id)
		} else if runErr == nil && parseErr == nil && receipt.Status == "already_excluded" &&
			receipt.AccountScope == account && receipt.JobID == jobID && receipt.CandidateID == t.candidateID {
			item.Reason = receipt.Receipt
			result.Skipped++
			_, _ = db.Exec(`UPDATE platform_action_ledger SET status='skipped',receipt=?,error_message='' WHERE project_id=? AND candidate_id=? AND record_id=? AND action_type='reject'`, item.Reason, projectID, t.candidateID, t.recordID)
			_, _ = db.Exec(`UPDATE screening_candidate_results SET action_status='verified_on_platform',action_receipt_at=?,action_error='' WHERE id=?`, now, t.id)
		} else {
			item.Status = "uncertain"
			item.Reason = receipt.Message
			if receipt.Receipt != "" {
				item.Reason = receipt.Receipt
			}
			if item.Reason == "" && runErr != nil {
				item.Reason = runErr.Error()
			}
			if item.Reason == "" {
				item.Reason = "平台回执无法核实"
			}
			result.Uncertain++
			_, _ = db.Exec(`UPDATE platform_action_ledger SET status='uncertain',error_message=? WHERE project_id=? AND candidate_id=? AND record_id=? AND action_type='reject'`, item.Reason, projectID, t.candidateID, t.recordID)
			_, _ = db.Exec(`UPDATE screening_candidate_results SET action_status='receipt_pending',action_error=? WHERE id=?`, item.Reason, t.id)
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
