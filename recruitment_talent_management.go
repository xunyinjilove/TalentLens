package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TalentManagementJob struct {
	JobID    string `json:"jobId"`
	Title    string `json:"title"`
	WorkArea string `json:"workArea"`
	Degree   string `json:"degree"`
	WorkYear string `json:"workYear"`
	Salary   string `json:"salary"`
}

type TalentManagementCatalog struct {
	Status       string                `json:"status"`
	Message      string                `json:"message,omitempty"`
	AccountScope string                `json:"accountScope"`
	Jobs         []TalentManagementJob `json:"jobs"`
}

type talentManagementCandidate struct {
	CandidateID       string `json:"candidateId"`
	JobID             string `json:"jobId"`
	RecordID          string `json:"recordId"`
	Name              string `json:"name"`
	ResumeText        string `json:"resumeText"`
	HasAttachment     bool   `json:"hasAttachment"`
	DocumentsComplete bool   `json:"documentsComplete"`
	DocumentStatus    string `json:"documentStatus"`
}

type talentManagementPage struct {
	Status       string                      `json:"status"`
	Message      string                      `json:"message"`
	AccountScope string                      `json:"accountScope"`
	JobID        string                      `json:"jobId"`
	StatusIndex  int                         `json:"statusIndex"`
	PageIndex    int                         `json:"pageIndex"`
	Total        int                         `json:"total"`
	HasMore      bool                        `json:"hasMore"`
	Candidates   []talentManagementCandidate `json:"candidates"`
}

type talentManagementDetail struct {
	Status            string `json:"status"`
	Message           string `json:"message"`
	AccountScope      string `json:"accountScope"`
	JobID             string `json:"jobId"`
	CandidateID       string `json:"candidateId"`
	RecordID          string `json:"recordId"`
	Name              string `json:"name"`
	ResumeText        string `json:"resumeText"`
	HasAttachment     bool   `json:"hasAttachment"`
	DocumentsComplete bool   `json:"documentsComplete"`
	DocumentStatus    string `json:"documentStatus"`
}

type TalentManagementPageImport struct {
	ApplicationImportResult
	JobID            string `json:"jobId"`
	StatusIndex      int    `json:"statusIndex"`
	PageIndex        int    `json:"pageIndex"`
	Total            int    `json:"total"`
	HasMore          bool   `json:"hasMore"`
	InterceptedCount int    `json:"interceptedCount"`
	InterceptReason  string `json:"interceptReason"`
}

type TalentManagementSyncTask struct {
	Platform           string `json:"platform"`
	PlatformAccount    string `json:"platformAccount"`
	PlatformJobID      string `json:"platformJobId"`
	ProjectID          string `json:"projectId"`
	StatusIndex        int    `json:"statusIndex"`
	PageIndex          int    `json:"pageIndex"`
	TotalCandidates    int    `json:"totalCandidates"`
	ImportedCandidates int    `json:"importedCandidates"`
	State              string `json:"state"` // idle | running | paused | error | completed
	AutoSyncEnabled    bool   `json:"autoSyncEnabled"`
	LastSyncAt         string `json:"lastSyncAt"`
	LastError          string `json:"lastError"`
	UpdatedAt          string `json:"updatedAt"`
}

type TalentManagementStrictCriteria struct {
	ProjectID                 string  `json:"projectId"`
	MinEducation              string  `json:"minEducation"`              // 大专 / 本科 / 硕士 / 博士 / ""
	MinWorkYears              float64 `json:"minWorkYears"`              // 0 表示不限
	RejectNonLocalWillingness bool    `json:"rejectNonLocalWillingness"` // 明确拒绝在目标城市工作
	TargetCity                string  `json:"targetCity"`                // 目标工作城市
	ComplianceConfirmed       bool    `json:"complianceConfirmed"`       // HR 确认合规授权
	UpdatedAt                 string  `json:"updatedAt"`
}

type TalentManagementAuditLog struct {
	ID              string `json:"id"`
	ProjectID       string `json:"projectId"`
	Platform        string `json:"platform"`
	PlatformAccount string `json:"platformAccount"`
	PlatformJobID   string `json:"platformJobId"`
	CandidateID     string `json:"candidateId"`
	RecordID        string `json:"recordId"`
	CandidateName   string `json:"candidateName"`
	RuleCode        string `json:"ruleCode"`
	ReasonLabel     string `json:"reasonLabel"`
	Evidence        string `json:"evidence"`
	PlatformReceipt string `json:"platformReceipt"`
	ActionStatus    string `json:"actionStatus"`
	CreatedAt       string `json:"createdAt"`
}

func resolveTalentManagementImportScript() string {
	for _, candidate := range []string{
		filepath.Join("scripts", "51job_talent_management_import.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "51job_talent_management_import.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "51job_talent_management_import.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "51job_talent_management_import.js"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// Open51JobManagementBrowser 启动或呼出 51job 专属浏览器至前台，导航至人才管理页面供 HR 登录或查看
func (a *App) Open51JobManagementBrowser() error {
	a.EnsurePlatformBrowser("51job")
	time.Sleep(300 * time.Millisecond)
	return a.OpenURL("https://ehire.51job.com/Revision/talent/management")
}

// Get51JobTalentManagementCatalog reads the actual logged-in account and job IDs.
// A title alone must never determine a local project mapping.
func (a *App) Get51JobTalentManagementCatalog() (TalentManagementCatalog, error) {
	result := TalentManagementCatalog{Status: "error", Jobs: []TalentManagementJob{}}
	// 确保 51job 专用浏览器已就绪
	_ = a.EnsurePlatformBrowser("51job")

	script := resolveTalentManagementImportScript()
	if script == "" {
		return result, fmt.Errorf("缺少 51job 人才管理导入脚本")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script, "--mode", "catalog")
	hideConsoleWindow(cmd)
	out, err := cmd.Output()
	if jsonErr := json.Unmarshal(out, &result); jsonErr != nil {
		if err != nil {
			return result, fmt.Errorf("读取 51job 人才管理职位失败: %w", err)
		}
		return result, fmt.Errorf("51job 职位数据无法解析: %w", jsonErr)
	}
	if result.Status != "ok" || strings.TrimSpace(result.AccountScope) == "" {
		if result.Message == "" {
			result.Message = "51job 登录身份或职位数据尚未就绪"
		}
		return result, fmt.Errorf("%s", result.Message)
	}
	return result, nil
}

// Bind51JobTalentManagementJob verifies the live account and job ID before
// writing the durable platform-to-local-project mapping.
func (a *App) Bind51JobTalentManagementJob(accountScope, jobID, projectID string) error {
	catalog, err := a.Get51JobTalentManagementCatalog()
	if err != nil {
		return err
	}
	if catalog.AccountScope != accountScope {
		return fmt.Errorf("51job 登录账号已变化，请重新读取职位列表")
	}
	for _, job := range catalog.Jobs {
		if job.JobID == jobID {
			if err := a.SetPlatformJobMapping("51job", accountScope, jobID, projectID); err != nil {
				return err
			}
			// 初始化或更新断点任务
			_, _ = a.GetTalentManagementSyncTask("51job", accountScope, jobID)
			return nil
		}
	}
	return fmt.Errorf("所选职位不属于当前 51job 账号")
}

func runTalentManagementAdapter(timeout time.Duration, args ...string) ([]byte, error) {
	script := resolveTalentManagementImportScript()
	if script == "" {
		return nil, fmt.Errorf("缺少 51job 人才管理导入脚本")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", append([]string{script}, args...)...)
	hideConsoleWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		var platformErr struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(out, &platformErr) == nil && platformErr.Message != "" {
			return nil, fmt.Errorf("%s", platformErr.Message)
		}
		return nil, fmt.Errorf("51job 人才管理读取失败: %w", err)
	}
	return out, nil
}

// Import51JobTalentManagementPage reads one verified platform page (maximum 50
// candidates). Incomplete attachments remain pending_document and never enter
// automatic analysis or rejection.
func (a *App) Import51JobTalentManagementPage(accountScope, jobID string, statusIndex, pageIndex int) (TalentManagementPageImport, error) {
	result := TalentManagementPageImport{JobID: jobID, StatusIndex: statusIndex, PageIndex: pageIndex}
	if statusIndex < 1 || statusIndex > 4 || pageIndex < 1 {
		return result, fmt.Errorf("仅支持待处理、合适、面试、待定的有效页码")
	}
	catalog, err := a.Get51JobTalentManagementCatalog()
	if err != nil {
		return result, err
	}
	if catalog.AccountScope != accountScope {
		return result, fmt.Errorf("51job 登录账号已变化")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return result, err
	}
	var projectID string
	if err := db.QueryRow(`SELECT project_id FROM platform_job_mappings WHERE platform='51job' AND platform_account=? AND platform_job_id=?`, accountScope, jobID).Scan(&projectID); err != nil || a.GetProject(projectID) == nil {
		return result, fmt.Errorf("请先按 51job 职位 ID 绑定有效的本地专属库")
	}
	out, err := runTalentManagementAdapter(90*time.Second, "--mode", "scan", "--job-id", jobID,
		"--status-index", fmt.Sprint(statusIndex), "--page-index", fmt.Sprint(pageIndex))
	if err != nil {
		return result, err
	}
	var page talentManagementPage
	if err := json.Unmarshal(out, &page); err != nil {
		return result, fmt.Errorf("51job 名单无法解析: %w", err)
	}
	if page.Status != "ok" || page.AccountScope != accountScope || page.JobID != jobID || page.StatusIndex != statusIndex || page.PageIndex != pageIndex || len(page.Candidates) > 50 {
		return result, fmt.Errorf("51job 职位、账号或分页回执不一致")
	}
	result.Total, result.HasMore = page.Total, page.HasMore

	criteria, _ := a.GetTalentManagementStrictCriteria(projectID)
	hasStrictFilter := criteria != nil && (criteria.MinEducation != "" || criteria.MinWorkYears > 0 || (criteria.RejectNonLocalWillingness && criteria.TargetCity != ""))

	interceptedCount := 0
	lastInterceptReason := ""
	records := make([]ApplicationRecord, 0, len(page.Candidates))
	for _, candidate := range page.Candidates {
		if candidate.CandidateID == "" || candidate.RecordID == "" || candidate.JobID != jobID {
			return result, fmt.Errorf("平台候选人标识不完整")
		}
		candidateName := candidate.Name
		completed := candidate.DocumentsComplete
		resumeText := candidate.ResumeText
		documentStatus := candidate.DocumentStatus

		// 硬性条件第一层：导入源头拦截
		if hasStrictFilter && resumeText != "" {
			decision, _, reasonLabel, evidence := EvaluateCandidateStrictCriteria(resumeText, *criteria)
			if decision == "UNFIT" {
				interceptedCount++
				lastInterceptReason = evidence
				// 记录淘汰审计日志
				nowStr := time.Now().Format("2006-01-02 15:04:05")
				auditID := recruitmentID("tmaudit")
				_, _ = db.Exec(`INSERT INTO talent_management_audit_logs
					(id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, rule_code, reason_label, evidence, platform_receipt, action_status, created_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					auditID, projectID, "51job", accountScope, jobID, candidate.CandidateID, candidate.RecordID, candidateName, "EDU_BELOW_MIN", reasonLabel, evidence, "导入源头识别硬性条件不符已自动拦截排除", "archived", nowStr)

				// 登记到 applications 为 unfit_archived 状态，避免后续定时任务重复抓取
				nowIso := time.Now().UTC().Format(time.RFC3339Nano)
				appID := fmt.Sprintf("51job_%s_%s", accountScope, candidate.RecordID)
				recordJson, _ := json.Marshal(map[string]interface{}{
					"platform": "51job", "platform_account": accountScope, "application_id": candidate.RecordID,
					"platform_candidate_id": candidate.CandidateID, "platform_job_id": jobID, "candidate_name": candidateName,
					"status": "unfit_archived", "unfit_reason": evidence,
				})
				_, _ = db.Exec(`INSERT INTO applications (id, platform, platform_account, platform_application_id, platform_candidate_id, platform_job_id, project_id, status, applied_at, raw_json, created_at, updated_at)
					VALUES (?, '51job', ?, ?, ?, ?, ?, 'unfit_archived', ?, ?, ?, ?)
					ON CONFLICT(platform, platform_account, platform_application_id) DO UPDATE SET status='unfit_archived', updated_at=?`,
					appID, accountScope, candidate.RecordID, candidate.CandidateID, jobID, projectID, nowIso, string(recordJson), nowIso, nowIso, nowIso)

				// 若已授权合规确认且为人选“待处理”状态，联动 51job 平台点击不合适
				if criteria.ComplianceConfirmed && statusIndex == 1 {
					go func(scope, pJobId, cId, rId, reason string) {
						_, _ = a.ExecuteStrictRejectCandidate(scope, pJobId, cId, rId, reason)
					}(accountScope, jobID, candidate.CandidateID, candidate.RecordID, reasonLabel)
				}
				// 排除！不加入常规入库 records 列表
				continue
			}
		}

		record := ApplicationRecord{
			Platform:            "51job",
			PlatformAccount:     accountScope,
			ApplicationID:       candidate.RecordID,
			PlatformCandidateID: candidate.CandidateID,
			PlatformJobID:       jobID,
			CandidateName:       candidateName,
			PlatformStatus:      []string{"", "待处理", "合适", "面试", "待定"}[statusIndex],
			DocumentsComplete:   &completed,
			DocumentStatus:      documentStatus,
			ResumeText:          resumeText,
		}
		records = append(records, record)
	}
	imported, err := a.ingestApplicationRecords("51job", records)
	result.ApplicationImportResult = imported
	result.InterceptedCount = interceptedCount
	result.InterceptReason = lastInterceptReason
	return result, err
}

// GetTalentManagementSyncTask 查询或自动初始化单岗位的断点任务
func (a *App) GetTalentManagementSyncTask(platform, accountScope, jobID string) (*TalentManagementSyncTask, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var task TalentManagementSyncTask
	var autoSync int
	err = db.QueryRow(`SELECT platform, platform_account, platform_job_id, project_id, status_index, page_index,
		total_candidates, imported_candidates, state, auto_sync_enabled, last_sync_at, last_error, updated_at
		FROM talent_management_sync_tasks WHERE platform=? AND platform_account=? AND platform_job_id=?`,
		platform, accountScope, jobID).Scan(
		&task.Platform, &task.PlatformAccount, &task.PlatformJobID, &task.ProjectID, &task.StatusIndex, &task.PageIndex,
		&task.TotalCandidates, &task.ImportedCandidates, &task.State, &autoSync, &task.LastSyncAt, &task.LastError, &task.UpdatedAt)
	if err == sql.ErrNoRows {
		var projectID string
		_ = db.QueryRow(`SELECT project_id FROM platform_job_mappings WHERE platform=? AND platform_account=? AND platform_job_id=?`,
			platform, accountScope, jobID).Scan(&projectID)
		if projectID == "" {
			return nil, fmt.Errorf("该职位尚未绑定本地项目")
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		task = TalentManagementSyncTask{
			Platform:        platform,
			PlatformAccount: accountScope,
			PlatformJobID:   jobID,
			ProjectID:       projectID,
			StatusIndex:     1,
			PageIndex:       1,
			State:           "idle",
			UpdatedAt:       now,
		}
		_, _ = db.Exec(`INSERT INTO talent_management_sync_tasks(platform, platform_account, platform_job_id, project_id, status_index, page_index, state, auto_sync_enabled, updated_at)
			VALUES(?,?,?,?,?,?,'idle',0,?)`, platform, accountScope, jobID, projectID, 1, 1, now)
		return &task, nil
	}
	if err != nil {
		return nil, err
	}
	task.AutoSyncEnabled = autoSync == 1
	return &task, nil
}

// ListTalentManagementSyncTasks 列出全部已记录的同步断点任务
func (a *App) ListTalentManagementSyncTasks() ([]TalentManagementSyncTask, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT platform, platform_account, platform_job_id, project_id, status_index, page_index,
		total_candidates, imported_candidates, state, auto_sync_enabled, last_sync_at, last_error, updated_at
		FROM talent_management_sync_tasks ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []TalentManagementSyncTask
	for rows.Next() {
		var task TalentManagementSyncTask
		var autoSync int
		if err := rows.Scan(&task.Platform, &task.PlatformAccount, &task.PlatformJobID, &task.ProjectID, &task.StatusIndex, &task.PageIndex,
			&task.TotalCandidates, &task.ImportedCandidates, &task.State, &autoSync, &task.LastSyncAt, &task.LastError, &task.UpdatedAt); err == nil {
			task.AutoSyncEnabled = autoSync == 1
			list = append(list, task)
		}
	}
	return list, nil
}

// SetTalentManagementAutoSync 开启或关闭某职位的每小时自动同步
func (a *App) SetTalentManagementAutoSync(platform, accountScope, jobID string, enabled bool) error {
	task, err := a.GetTalentManagementSyncTask(platform, accountScope, jobID)
	if err != nil {
		return err
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	autoVal := 0
	if enabled {
		autoVal = 1
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err = db.Exec(`UPDATE talent_management_sync_tasks SET auto_sync_enabled=?, updated_at=? WHERE platform=? AND platform_account=? AND platform_job_id=?`,
		autoVal, now, platform, accountScope, jobID)
	task.AutoSyncEnabled = enabled
	return err
}

// ResetTalentManagementSyncTask 重置断点回到第 1 状态第 1 页
func (a *App) ResetTalentManagementSyncTask(platform, accountScope, jobID string) error {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err = db.Exec(`UPDATE talent_management_sync_tasks SET status_index=1, page_index=1, state='idle', last_error='', updated_at=?
		WHERE platform=? AND platform_account=? AND platform_job_id=?`, now, platform, accountScope, jobID)
	return err
}

// RunTalentManagementNextBatch 从断点执行单批次导入（最多 50 人），可续跑、暂停与自动推进状态
func (a *App) RunTalentManagementNextBatch(accountScope, jobID string) (TalentManagementPageImport, error) {
	task, err := a.GetTalentManagementSyncTask("51job", accountScope, jobID)
	if err != nil {
		return TalentManagementPageImport{JobID: jobID}, err
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return TalentManagementPageImport{JobID: jobID}, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	_, _ = db.Exec(`UPDATE talent_management_sync_tasks SET state='running', last_error='', updated_at=? WHERE platform='51job' AND platform_account=? AND platform_job_id=?`,
		now, accountScope, jobID)

	importResult, importErr := a.Import51JobTalentManagementPage(accountScope, jobID, task.StatusIndex, task.PageIndex)
	if importErr != nil {
		_, _ = db.Exec(`UPDATE talent_management_sync_tasks SET state='error', last_error=?, updated_at=? WHERE platform='51job' AND platform_account=? AND platform_job_id=?`,
			importErr.Error(), time.Now().Format("2006-01-02 15:04:05"), accountScope, jobID)
		return importResult, importErr
	}

	nextStatusIndex := task.StatusIndex
	nextPageIndex := task.PageIndex
	nextState := "running"
	if importResult.HasMore {
		nextPageIndex++
	} else {
		if nextStatusIndex < 4 {
			nextStatusIndex++
			nextPageIndex = 1
		} else {
			nextStatusIndex = 1
			nextPageIndex = 1
			nextState = "completed"
		}
	}

	now = time.Now().Format("2006-01-02 15:04:05")
	_, _ = db.Exec(`UPDATE talent_management_sync_tasks SET status_index=?, page_index=?, total_candidates=?,
		imported_candidates=imported_candidates+?, state=?, last_sync_at=?, updated_at=?
		WHERE platform='51job' AND platform_account=? AND platform_job_id=?`,
		nextStatusIndex, nextPageIndex, importResult.Total, importResult.AddedCount, nextState, now, now,
		accountScope, jobID)

	return importResult, nil
}

func parseEducationRank(edu string) int {
	edu = strings.TrimSpace(edu)
	switch {
	case strings.Contains(edu, "博士"):
		return 5
	case strings.Contains(edu, "硕士") || strings.Contains(edu, "研究生") || strings.Contains(edu, "MBA") || strings.Contains(edu, "EMBA"):
		return 4
	case strings.Contains(edu, "本科") || strings.Contains(edu, "学士"):
		return 3
	case strings.Contains(edu, "大专") || strings.Contains(edu, "专科"):
		return 2
	case strings.Contains(edu, "中专") || strings.Contains(edu, "高中") || strings.Contains(edu, "中技") || strings.Contains(edu, "初中"):
		return 1
	default:
		return 0
	}
}

func parseWorkYears(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if strings.Contains(raw, "无经验") || strings.Contains(raw, "在校生") || strings.Contains(raw, "应届") {
		return 0, true
	}
	var y float64
	if _, err := fmt.Sscanf(raw, "%f年", &y); err == nil {
		return y, true
	}
	if _, err := fmt.Sscanf(raw, "%f", &y); err == nil {
		return y, true
	}
	return 0, false
}

// EvaluateCandidateStrictCriteria 仅在拥有确凿可定位证据时判定不符合硬性条件。
// 信息缺失、来源冲突、低分、年龄及性别绝不自动淘汰。
func EvaluateCandidateStrictCriteria(resumeText string, criteria TalentManagementStrictCriteria) (decision, ruleCode, reasonLabel, evidence string) {
	if strings.TrimSpace(resumeText) == "" {
		return "KEEP_UNKNOWN", "", "", "资料不完整，保持待人工复核"
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(resumeText), &data); err != nil {
		return "KEEP_UNKNOWN", "", "", "简历格式无法解析，保持待人工复核"
	}

	baseInfo, _ := data["base_info"].(map[string]interface{})
	if baseInfo == nil {
		return "KEEP_UNKNOWN", "", "", "缺少基本信息，保持待人工复核"
	}

	hasUnknown := false
	var unknownReason string

	// 1. 最低学历硬性条件
	minEduRank := parseEducationRank(criteria.MinEducation)
	if minEduRank > 0 {
		topDegreeVal, _ := baseInfo["top_degree_value"].(string)
		candRank := parseEducationRank(topDegreeVal)
		if candRank == 0 {
			hasUnknown = true
			unknownReason = "最高学历未填或无法解析，保持待人工复核"
		} else if candRank < minEduRank {
			school, _ := baseInfo["top_school_name"].(string)
			major, _ := baseInfo["top_major_value"].(string)
			evidence = fmt.Sprintf("最高学历为【%s】，未达到岗位最低硬性要求【%s】（毕业院校: %s, 专业: %s）", topDegreeVal, criteria.MinEducation, school, major)
			return "UNFIT", "EDU_BELOW_MIN", "学历不达标", evidence
		}
	}

	// 2. 最低工作年限硬性条件
	if criteria.MinWorkYears > 0 {
		workYearVal, _ := baseInfo["work_year_value"].(string)
		years, ok := parseWorkYears(workYearVal)
		if !ok {
			hasUnknown = true
			if unknownReason == "" {
				unknownReason = "工作年限信息缺失，保持待人工复核"
			}
		} else if years < criteria.MinWorkYears {
			evidence = fmt.Sprintf("工作年限为【%.1f年】（%s），未达到岗位最低硬性要求【%.1f年】", years, workYearVal, criteria.MinWorkYears)
			return "UNFIT", "EXP_BELOW_MIN", "年限不达标", evidence
		}
	}

	// 3. 目标城市工作意愿
	if criteria.RejectNonLocalWillingness && strings.TrimSpace(criteria.TargetCity) != "" {
		targetCity := strings.TrimSpace(criteria.TargetCity)
		optIntention, _ := data["optimal_job_intention"].(map[string]interface{})
		if optIntention != nil {
			intentArea, _ := optIntention["intention_area"].(string)
			intentArea = strings.TrimSpace(intentArea)
			if intentArea != "" && !strings.Contains(intentArea, targetCity) && !strings.Contains(targetCity, intentArea) {
				evidence = fmt.Sprintf("期望工作城市仅包含【%s】，明确表示不在目标城市【%s】工作", intentArea, targetCity)
				return "UNFIT", "INTENT_CITY_MISMATCH", "无目标城市工作意愿", evidence
			}
		}
	}

	if hasUnknown {
		return "KEEP_UNKNOWN", "", "", unknownReason
	}
	return "KEEP_MATCHED", "", "", "满足各项硬性要求"
}

// GetTalentManagementStrictCriteria 获取岗位专属硬性条件
func (a *App) GetTalentManagementStrictCriteria(projectID string) (*TalentManagementStrictCriteria, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var c TalentManagementStrictCriteria
	var rejectCity, compliance int
	err = db.QueryRow(`SELECT project_id, min_education, min_work_years, reject_non_local_willingness, target_city, compliance_confirmed, updated_at
		FROM talent_management_strict_criteria WHERE project_id=?`, projectID).Scan(
		&c.ProjectID, &c.MinEducation, &c.MinWorkYears, &rejectCity, &c.TargetCity, &compliance, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		p := a.GetProject(projectID)
		targetCity := ""
		minEdu := ""
		var minYears float64
		if p != nil {
			targetCity = p.JobConfig.City
			minEdu = p.JobConfig.EducationLevel
			minYears = float64(p.JobConfig.ExperienceYears)
		}
		c = TalentManagementStrictCriteria{
			ProjectID:                 projectID,
			MinEducation:              minEdu,
			MinWorkYears:              minYears,
			RejectNonLocalWillingness: false,
			TargetCity:                targetCity,
			ComplianceConfirmed:       false,
			UpdatedAt:                 time.Now().Format("2006-01-02 15:04:05"),
		}
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	c.RejectNonLocalWillingness = rejectCity == 1
	c.ComplianceConfirmed = compliance == 1
	return &c, nil
}

// SetTalentManagementStrictCriteria 保存岗位专属硬性条件
func (a *App) SetTalentManagementStrictCriteria(criteria TalentManagementStrictCriteria) error {
	if criteria.ProjectID == "" {
		return fmt.Errorf("必须指定本地职位分库 ID")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	rejectCity := 0
	if criteria.RejectNonLocalWillingness {
		rejectCity = 1
	}
	compliance := 0
	if criteria.ComplianceConfirmed {
		compliance = 1
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err = db.Exec(`INSERT INTO talent_management_strict_criteria(project_id, min_education, min_work_years, reject_non_local_willingness, target_city, compliance_confirmed, updated_at)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(project_id) DO UPDATE SET min_education=excluded.min_education, min_work_years=excluded.min_work_years,
		reject_non_local_willingness=excluded.reject_non_local_willingness, target_city=excluded.target_city,
		compliance_confirmed=excluded.compliance_confirmed, updated_at=excluded.updated_at`,
		criteria.ProjectID, criteria.MinEducation, criteria.MinWorkYears, rejectCity, criteria.TargetCity, compliance, now)
	if err != nil {
		return err
	}

	// 智能自愈：若 HR 调整了硬性条件（如将本科调整为大专或不限），自动将此前因该条件归档但现在符合条件的候选人恢复到人才库活跃状态
	rows, err := db.Query(`SELECT pc.id, COALESCE(rv.content, ''), COALESCE(rv.raw_json, '')
		FROM project_candidates pc
		LEFT JOIN project_candidate_resumes pcr ON pcr.project_candidate_id = pc.id
		LEFT JOIN resume_versions rv ON rv.id = pcr.resume_version_id
		WHERE pc.project_id = ? AND pc.archived_at <> ''`, criteria.ProjectID)
	if err == nil {
		nowIso := time.Now().UTC().Format(time.RFC3339Nano)
		for rows.Next() {
			var pcID, content, rawJson string
			if rows.Scan(&pcID, &content, &rawJson) == nil {
				resumeContent := content
				if rawJson != "" {
					var r Resume
					if json.Unmarshal([]byte(rawJson), &r) == nil && strings.TrimSpace(r.Content) != "" {
						resumeContent = r.Content
					}
				}
				if strings.TrimSpace(resumeContent) != "" {
					decision, _, _, _ := EvaluateCandidateStrictCriteria(resumeContent, criteria)
					if decision != "UNFIT" {
						// 恢复到人才库
						_, _ = db.Exec(`UPDATE project_candidates SET archived_at='', updated_at=? WHERE id=?`, nowIso, pcID)
					}
				}
			}
		}
		rows.Close()
	}

	return nil
}

// ListTalentManagementAuditLogs 查询某岗位的淘汰审计日志（按时间倒序）
func (a *App) ListTalentManagementAuditLogs(projectID string) ([]TalentManagementAuditLog, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id,
		candidate_name, rule_code, reason_label, evidence, platform_receipt, action_status, created_at
		FROM talent_management_audit_logs WHERE project_id=? ORDER BY created_at DESC LIMIT 100`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []TalentManagementAuditLog
	for rows.Next() {
		var l TalentManagementAuditLog
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Platform, &l.PlatformAccount, &l.PlatformJobID, &l.CandidateID, &l.RecordID,
			&l.CandidateName, &l.RuleCode, &l.ReasonLabel, &l.Evidence, &l.PlatformReceipt, &l.ActionStatus, &l.CreatedAt); err == nil {
			logs = append(logs, l)
		}
	}
	return logs, nil
}

// ExecuteStrictRejectCandidate 严格自动淘汰单条候选人，核验身份、状态、证据并在平台执行与留存审计
func (a *App) ExecuteStrictRejectCandidate(accountScope, jobID, candidateID, recordID, projectID string) (bool, error) {
	criteria, err := a.GetTalentManagementStrictCriteria(projectID)
	if err != nil {
		return false, err
	}
	if !criteria.ComplianceConfirmed {
		return false, fmt.Errorf("请先在岗位硬性条件中勾选 HR 合规授权确认")
	}

	out, err := runTalentManagementAdapter(45*time.Second, "--mode", "detail", "--job-id", jobID,
		"--candidate-id", candidateID, "--record-id", recordID, "--status-index", "1", "--page-index", "1")
	if err != nil {
		return false, fmt.Errorf("读取候选人详情失败: %w", err)
	}
	var detail talentManagementDetail
	if err := json.Unmarshal(out, &detail); err != nil || detail.Status != "ok" {
		return false, fmt.Errorf("平台候选人数据解析失败")
	}
	if !detail.DocumentsComplete {
		return false, fmt.Errorf("候选人资料尚未完整（附件待核验），受完整性保护禁止自动淘汰")
	}

	decision, ruleCode, reasonLabel, evidence := EvaluateCandidateStrictCriteria(detail.ResumeText, *criteria)
	if decision != "UNFIT" {
		return false, fmt.Errorf("候选人未违反硬性条件（%s），不能标记不合适", evidence)
	}

	platformReason := "其他原因"
	switch ruleCode {
	case "EDU_BELOW_MIN":
		platformReason = "学历不匹配"
	case "EXP_BELOW_MIN":
		platformReason = "工作年限不匹配"
	case "INTENT_CITY_MISMATCH":
		platformReason = "工作地点不匹配"
	}

	rejectOut, rejectErr := runTalentManagementAdapter(30*time.Second, "--mode", "reject", "--job-id", jobID,
		"--candidate-id", candidateID, "--record-id", recordID, "--reason-label", platformReason)
	receipt := ""
	actionStatus := "rejected"
	if rejectErr != nil {
		actionStatus = "error"
		receipt = "平台操作未完成: " + rejectErr.Error()
	} else {
		var resp struct {
			Receipt string `json:"receipt"`
		}
		_ = json.Unmarshal(rejectOut, &resp)
		receipt = resp.Receipt
		if receipt == "" {
			receipt = "已在 51job 标记不合适（" + platformReason + "）"
		}
	}

	db, _ := a.getRecruitmentDB()
	auditID := recruitmentID("tm_audit")
	now := time.Now().Format("2006-01-02 15:04:05")
	if db != nil {
		_, _ = db.Exec(`INSERT INTO talent_management_audit_logs(id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, rule_code, reason_label, evidence, platform_receipt, action_status, created_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			auditID, projectID, "51job", accountScope, jobID, candidateID, recordID, detail.Name, ruleCode, reasonLabel, evidence, receipt, actionStatus, now)
		_, _ = db.Exec(`UPDATE applications SET status='unfit_archived' WHERE platform='51job' AND platform_account=? AND platform_application_id=?`, accountScope, recordID)
	}

	if rejectErr != nil {
		return false, rejectErr
	}
	return true, nil
}

func (a *App) startTalentManagementScheduler() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.runScheduledTalentManagementSync()
			}
		}
	}()
}

func (a *App) runScheduledTalentManagementSync() {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return
	}
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM talent_management_sync_tasks WHERE auto_sync_enabled=1`).Scan(&count)
	if count == 0 {
		return
	}

	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9503/json/version")
	if err != nil || resp.StatusCode != 200 {
		return
	}
	_ = resp.Body.Close()

	catalog, err := a.Get51JobTalentManagementCatalog()
	if err != nil || catalog.Status != "ok" || catalog.AccountScope == "" {
		return
	}

	rows, err := db.Query(`SELECT platform_job_id FROM talent_management_sync_tasks WHERE auto_sync_enabled=1 AND platform='51job' AND platform_account=?`, catalog.AccountScope)
	if err != nil {
		return
	}
	var jobIDs []string
	for rows.Next() {
		var jid string
		if rows.Scan(&jid) == nil {
			jobIDs = append(jobIDs, jid)
		}
	}
	rows.Close()

	for _, jid := range jobIDs {
		_, _ = a.RunTalentManagementNextBatch(catalog.AccountScope, jid)
		time.Sleep(2 * time.Second)
	}
}

// OneClickFilterTalentPool 对指定职位分库执行“一键筛选”，根据岗位专属硬性条件自动排除不合格简历
func (a *App) OneClickFilterTalentPool(projectID string) (int, error) {
	if strings.TrimSpace(projectID) == "" {
		return 0, fmt.Errorf("请先选择具体的职位分库")
	}
	criteria, err := a.GetTalentManagementStrictCriteria(projectID)
	if err != nil || criteria == nil || (criteria.MinEducation == "" && criteria.MinWorkYears <= 0 && (!criteria.RejectNonLocalWillingness || criteria.TargetCity == "")) {
		return 0, fmt.Errorf("当前职位尚未配置硬性条件，请先在【51job 人才管理】中设置学历或年限要求")
	}

	db, err := a.getRecruitmentDB()
	if err != nil {
		return 0, err
	}

	// 查出该职位下所有当前在库（未归档）的候选人
	rows, err := db.Query(`SELECT pc.id, c.id, c.display_name, COALESCE(rv.content, ''), COALESCE(rv.raw_json, '')
		FROM project_candidates pc
		JOIN candidates c ON c.id = pc.candidate_id
		LEFT JOIN resume_versions rv ON rv.id = (
			SELECT pcr.resume_version_id FROM project_candidate_resumes pcr
			JOIN resume_versions rv2 ON rv2.id = pcr.resume_version_id
			WHERE pcr.project_candidate_id = pc.id ORDER BY rv2.created_at DESC, rv2.id DESC LIMIT 1
		)
		WHERE pc.project_id = ? AND pc.archived_at = ''`, projectID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type unfitCandidate struct {
		pcID                string
		candidateID         string
		name                string
		reasonLabel         string
		evidence            string
		sourcePlatform      string
		platformCandidateID string
	}

	var unfitness []unfitCandidate
	for rows.Next() {
		var pcID, cID, name, content, rawJson string
		if err := rows.Scan(&pcID, &cID, &name, &content, &rawJson); err != nil {
			return 0, err
		}
		platform := ""
		platformCandidateID := ""
		resumeContent := content
		if rawJson != "" {
			var resume Resume
			if json.Unmarshal([]byte(rawJson), &resume) == nil {
				platform = resume.SourcePlatform
				platformCandidateID = resume.PlatformCandidateID
				if strings.TrimSpace(resume.Content) != "" {
					resumeContent = resume.Content
				}
			}
		}
		if strings.TrimSpace(resumeContent) == "" {
			continue
		}
		decision, _, reasonLabel, evidence := EvaluateCandidateStrictCriteria(resumeContent, *criteria)
		if decision == "UNFIT" {
			unfitness = append(unfitness, unfitCandidate{
				pcID:                pcID,
				candidateID:         cID,
				name:                name,
				reasonLabel:         reasonLabel,
				evidence:            evidence,
				sourcePlatform:      platform,
				platformCandidateID: platformCandidateID,
			})
		}
	}
	rows.Close()

	if len(unfitness) == 0 {
		return 0, nil
	}

	pcIDs := make([]string, 0, len(unfitness))
	for _, u := range unfitness {
		pcIDs = append(pcIDs, u.pcID)
	}

	// 批量归档移出人才库
	archivedCount, err := a.ArchiveTalentCandidates(pcIDs)
	if err != nil {
		return 0, err
	}

	// 写入审计日志与平台联动
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	for _, u := range unfitness {
		accountID := ""
		platformJobID := ""
		var recordID, stage string
		_ = db.QueryRow(`SELECT platform_account, platform_job_id, platform_application_id, status FROM applications
			WHERE platform=? AND platform_candidate_id=? AND project_id=?`, u.sourcePlatform, u.platformCandidateID, projectID).Scan(&accountID, &platformJobID, &recordID, &stage)

		receipt := "人才库一键筛选识别不符合硬性条件已移入淘汰归档"
		auditID := recruitmentID("tmaudit")
		_, _ = db.Exec(`INSERT INTO talent_management_audit_logs
			(id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, rule_code, reason_label, evidence, platform_receipt, action_status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			auditID, projectID, u.sourcePlatform, accountID, platformJobID, u.candidateID, recordID, u.name, "EDU_BELOW_MIN", u.reasonLabel, u.evidence, receipt, "archived", nowStr)

		// 更新 applications 状态
		if recordID != "" {
			_, _ = db.Exec(`UPDATE applications SET status='unfit_archived', updated_at=? WHERE platform=? AND platform_application_id=?`,
				time.Now().UTC().Format(time.RFC3339Nano), u.sourcePlatform, recordID)
		}

		// 若已授权合规确认且属于待处理状态，联动平台执行淘汰
		if criteria.ComplianceConfirmed && u.sourcePlatform == "51job" && accountID != "" && platformJobID != "" && recordID != "" && (stage == "待处理" || stage == "") {
			go func(acc, pJob, cId, rId, reason string) {
				_, _ = a.ExecuteStrictRejectCandidate(acc, pJob, cId, rId, reason)
			}(accountID, platformJobID, u.platformCandidateID, recordID, u.reasonLabel)
		}
	}

	return archivedCount, nil
}
