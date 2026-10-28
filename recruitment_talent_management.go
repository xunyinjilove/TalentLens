package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
	MinAge                    int     `json:"minAge"`                    // 最小年龄 (0 表示不限)
	MaxAge                    int     `json:"maxAge"`                    // 最大年龄 (0 表示不限)
	MaxExpectedSalary         int     `json:"maxExpectedSalary"`         // 期望薪资上限 (0 表示不限，单位：千元/月，如 15 代表 15k)
	Gender                    string  `json:"gender"`                    // 性别: "" (不限) / "男" / "女"
	JobSeekingStatus          string  `json:"jobSeekingStatus"`          // 求职状态: "" (不限) / "离职" 等
	MajorKeywords             string  `json:"majorKeywords"`             // 专业关键词 (逗号分隔，如 "医学,药学,市场营销")
	RejectNonLocalWillingness bool    `json:"rejectNonLocalWillingness"` // 明确拒绝在目标城市工作
	TargetCity                string  `json:"targetCity"`                // 目标工作城市
	ActivityDays              int     `json:"activityDays"`              // 活跃度天数: 0 (不限) / 3 / 7 / 30，默认 7
	CriteriaVersion           string  `json:"criteriaVersion"`           // 条件版本号
	ProjectIntroMessage       string  `json:"projectIntroMessage"`       // 项目专属打招呼/文案
	ScheduleTime              string  `json:"scheduleTime"`              // 每日计划时间 (如 "09:30")
	ScheduleEnabled           bool    `json:"scheduleEnabled"`           // 是否启用每日计划
	ComplianceConfirmed       bool    `json:"complianceConfirmed"`       // HR 确认合规授权
	AutoRejectPlatform        bool    `json:"autoRejectPlatform"`        // 是否联动 51job 自动标记不合适
	UpdatedAt                 string  `json:"updatedAt"`
}

type OneClickFilterResult struct {
	TotalChecked          int      `json:"totalChecked"`
	UnfitCount            int      `json:"unfitCount"`
	PlatformRejectedCount int      `json:"platformRejectedCount"`
	ActiveRemainingCount  int      `json:"activeRemainingCount"`
	Details               []string `json:"details"`
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

	// 快速探测 9503 端口是否在线，避免长时间超时卡死
	client := http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:9503/json/version")
	if err != nil || resp.StatusCode != 200 {
		return result, fmt.Errorf("51job 专用浏览器端口 (9503) 未连接，请先点击【打开 51job 浏览器】登录企业账号")
	}
	_ = resp.Body.Close()

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
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("读取 51job 职位超时，请先点击【打开 51job 浏览器】确认已登录并可查看人才管理")
		}
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

func parseCandidateAge(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var age int
	if _, err := fmt.Sscanf(raw, "%d岁", &age); err == nil && age > 0 {
		return age
	}
	if _, err := fmt.Sscanf(raw, "%d", &age); err == nil && age > 0 {
		return age
	}
	re := regexp.MustCompile(`(\d{2})\s*岁`)
	if m := re.FindStringSubmatch(raw); len(m) > 1 {
		if a, err := strconv.Atoi(m[1]); err == nil && a >= 16 && a <= 80 {
			return a
		}
	}
	return 0
}

func parseCandidateSalaryK(raw string) (maxK float64, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	reWan := regexp.MustCompile(`(?:[-~到至]|^)\s*(\d+(?:\.\d+)?)\s*万`)
	if m := reWan.FindStringSubmatch(raw); len(m) > 1 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v * 10, true
		}
	}
	reK := regexp.MustCompile(`(?:[-~到至]|^)\s*(\d+(?:\.\d+)?)\s*[kK千]`)
	if m := reK.FindStringSubmatch(raw); len(m) > 1 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v, true
		}
	}
	reYuan := regexp.MustCompile(`(?:[-~到至]|^)\s*(\d{4,6})`)
	if m := reYuan.FindStringSubmatch(raw); len(m) > 1 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v / 1000.0, true
		}
	}
	return 0, false
}

type candidateProfileSummary struct {
	Degree           string
	School           string
	Major            string
	WorkYears        float64
	WorkYearsKnown   bool
	Age              int
	SalaryMaxK       float64
	SalaryKnown      bool
	Gender           string
	JobSeekingStatus string
	IntentCities     []string
}

func extractCandidateProfile(resumeText string) candidateProfileSummary {
	var summary candidateProfileSummary
	trimmed := strings.TrimSpace(resumeText)
	if trimmed == "" {
		return summary
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &data); err == nil && data != nil {
		baseInfo, _ := data["base_info"].(map[string]interface{})
		if baseInfo != nil {
			if deg, ok := baseInfo["top_degree_value"].(string); ok {
				summary.Degree = deg
			}
			if sch, ok := baseInfo["top_school_name"].(string); ok {
				summary.School = sch
			}
			if maj, ok := baseInfo["top_major_value"].(string); ok {
				summary.Major = maj
			}
			if wy, ok := baseInfo["work_year_value"].(string); ok {
				if y, ok2 := parseWorkYears(wy); ok2 {
					summary.WorkYears = y
					summary.WorkYearsKnown = true
				}
			}
			if ageStr, ok := baseInfo["age"].(string); ok {
				summary.Age = parseCandidateAge(ageStr)
			}
			if gen, ok := baseInfo["sex"].(string); ok {
				summary.Gender = strings.TrimSpace(gen)
			}
			if sit, ok := baseInfo["current_situation"].(string); ok {
				summary.JobSeekingStatus = strings.TrimSpace(sit)
			}
		}

		if summary.Degree == "" {
			if deg, ok := data["degree"].(string); ok {
				summary.Degree = deg
			} else if edu, ok := data["education"].(string); ok {
				summary.Degree = edu
			} else if deg, ok := data["topdegree"].(string); ok {
				summary.Degree = deg
			} else if degObj, ok := data["highestdegree"].(map[string]interface{}); ok {
				if d, ok2 := degObj["degree"].(string); ok2 {
					summary.Degree = d
				}
				if m, ok2 := degObj["degreemajor"].(string); ok2 && summary.Major == "" {
					summary.Major = m
				}
				if s, ok2 := degObj["degreeschool"].(string); ok2 && summary.School == "" {
					summary.School = s
				}
			}
		}

		if summary.School == "" {
			if sch, ok := data["school"].(string); ok {
				summary.School = sch
			}
		}
		if summary.Major == "" {
			if maj, ok := data["major"].(string); ok {
				summary.Major = maj
			}
		}

		if !summary.WorkYearsKnown {
			if wy, ok := data["workyear"].(string); ok {
				if y, ok2 := parseWorkYears(wy); ok2 {
					summary.WorkYears = y
					summary.WorkYearsKnown = true
				}
			} else if wyNum, ok := data["workyear"].(float64); ok {
				summary.WorkYears = wyNum
				summary.WorkYearsKnown = true
			} else if wy2, ok := data["work_years"].(string); ok {
				if y, ok2 := parseWorkYears(wy2); ok2 {
					summary.WorkYears = y
					summary.WorkYearsKnown = true
				}
			} else if wyNum2, ok := data["work_years"].(float64); ok {
				summary.WorkYears = wyNum2
				summary.WorkYearsKnown = true
			}
		}

		if summary.Age == 0 {
			if aStr, ok := data["displayage"].(string); ok {
				summary.Age = parseCandidateAge(aStr)
			} else if aNum, ok := data["displayage"].(float64); ok {
				summary.Age = int(aNum)
			}
		}

		if summary.Gender == "" {
			if g, ok := data["sex"].(string); ok {
				summary.Gender = strings.TrimSpace(g)
			}
		}

		if summary.JobSeekingStatus == "" {
			if s, ok := data["job_hunting_status"].(string); ok {
				summary.JobSeekingStatus = strings.TrimSpace(s)
			} else if s2, ok := data["currentsituation"].(string); ok {
				summary.JobSeekingStatus = strings.TrimSpace(s2)
			}
		}

		if optIntention, ok := data["optimal_job_intention"].(map[string]interface{}); ok && optIntention != nil {
			if area, ok2 := optIntention["intention_area"].(string); ok2 && strings.TrimSpace(area) != "" {
				summary.IntentCities = append(summary.IntentCities, strings.TrimSpace(area))
			}
		}

		if intentions, ok := data["jobintention"].([]interface{}); ok && len(intentions) > 0 {
			for _, item := range intentions {
				if m, ok2 := item.(map[string]interface{}); ok2 {
					if sal, ok3 := m["displayexpectsalary"].(string); ok3 {
						if maxK, ok4 := parseCandidateSalaryK(sal); ok4 {
							if !summary.SalaryKnown || maxK > summary.SalaryMaxK {
								summary.SalaryMaxK = maxK
								summary.SalaryKnown = true
							}
						}
					}
					if areas, ok3 := m["expectarea"].([]interface{}); ok3 {
						for _, a := range areas {
							if aObj, ok4 := a.(map[string]interface{}); ok4 {
								if pc, ok5 := aObj["provincecity"].(string); ok5 && strings.TrimSpace(pc) != "" {
									summary.IntentCities = append(summary.IntentCities, strings.TrimSpace(pc))
								}
							}
						}
					}
				}
			}
		}

		if summary.Major == "" {
			if edus, ok := data["education"].([]interface{}); ok {
				for _, e := range edus {
					if em, ok2 := e.(map[string]interface{}); ok2 {
						if maj, ok3 := em["major"].(string); ok3 && strings.TrimSpace(maj) != "" {
							summary.Major = strings.TrimSpace(maj)
							break
						}
					}
				}
			}
		}
	} else {
		lines := strings.Split(trimmed, "\n")
		for _, line := range lines {
			if summary.Age == 0 {
				summary.Age = parseCandidateAge(line)
			}
			if !summary.WorkYearsKnown {
				if y, ok := parseWorkYears(line); ok {
					summary.WorkYears = y
					summary.WorkYearsKnown = true
				}
			}
			if summary.Degree == "" {
				for _, deg := range []string{"博士", "硕士", "研究生", "本科", "大专", "专科", "中专", "高中"} {
					if strings.Contains(line, deg) {
						summary.Degree = deg
						break
					}
				}
			}
			if summary.Gender == "" {
				if strings.Contains(line, "男") {
					summary.Gender = "男"
				} else if strings.Contains(line, "女") {
					summary.Gender = "女"
				}
			}
			if !summary.SalaryKnown {
				if maxK, ok := parseCandidateSalaryK(line); ok {
					summary.SalaryMaxK = maxK
					summary.SalaryKnown = true
				}
			}
		}
	}

	return summary
}

// EvaluateCandidateStrictCriteria 仅在拥有确凿可定位证据时判定不符合硬性条件。
func EvaluateCandidateStrictCriteria(resumeText string, criteria TalentManagementStrictCriteria) (decision, ruleCode, reasonLabel, evidence string) {
	if strings.TrimSpace(resumeText) == "" {
		return "KEEP_UNKNOWN", "", "", "简历内容为空，保持待人工复核"
	}

	prof := extractCandidateProfile(resumeText)
	hasUnknown := false
	var unknownReason string

	// 1. 最低学历硬性条件
	minEduRank := parseEducationRank(criteria.MinEducation)
	if minEduRank > 0 {
		candRank := parseEducationRank(prof.Degree)
		if candRank == 0 {
			hasUnknown = true
			unknownReason = "最高学历未填或无法解析，保持待人工复核"
		} else if candRank < minEduRank {
			evidence = fmt.Sprintf("最高学历为【%s】，未达到岗位最低硬性要求【%s】", prof.Degree, criteria.MinEducation)
			if prof.School != "" || prof.Major != "" {
				evidence += fmt.Sprintf("（毕业院校: %s, 专业: %s）", prof.School, prof.Major)
			}
			return "UNFIT", "EDU_BELOW_MIN", "学历不达标", evidence
		}
	}

	// 2. 最低工作年限硬性条件
	if criteria.MinWorkYears > 0 {
		if !prof.WorkYearsKnown {
			hasUnknown = true
			if unknownReason == "" {
				unknownReason = "工作年限信息缺失，保持待人工复核"
			}
		} else if prof.WorkYears < criteria.MinWorkYears {
			evidence = fmt.Sprintf("工作年限为【%.1f年】，未达到岗位最低硬性要求【%.1f年】", prof.WorkYears, criteria.MinWorkYears)
			return "UNFIT", "EXP_BELOW_MIN", "年限不达标", evidence
		}
	}

	// 3. 年龄区间硬性限制
	if criteria.MinAge > 0 && prof.Age > 0 && prof.Age < criteria.MinAge {
		evidence = fmt.Sprintf("年龄为【%d岁】，低于岗位要求的最低年龄【%d岁】", prof.Age, criteria.MinAge)
		return "UNFIT", "AGE_MISMATCH", "年龄偏小", evidence
	}
	if criteria.MaxAge > 0 && prof.Age > 0 && prof.Age > criteria.MaxAge {
		evidence = fmt.Sprintf("年龄为【%d岁】，超过岗位要求的最高年龄【%d岁】", prof.Age, criteria.MaxAge)
		return "UNFIT", "AGE_MISMATCH", "年龄超出上限", evidence
	}

	// 4. 期望薪资上限
	if criteria.MaxExpectedSalary > 0 && prof.SalaryKnown && prof.SalaryMaxK > 0 {
		if prof.SalaryMaxK > float64(criteria.MaxExpectedSalary) {
			evidence = fmt.Sprintf("期望薪资约为【%.1f千/月】，超出岗位期望薪资上限【%d千/月】", prof.SalaryMaxK, criteria.MaxExpectedSalary)
			return "UNFIT", "SALARY_MISMATCH", "期望薪资超出预算", evidence
		}
	}

	// 5. 性别要求限制
	if criteria.Gender != "" && criteria.Gender != "不限" && prof.Gender != "" {
		if prof.Gender != criteria.Gender {
			evidence = fmt.Sprintf("候选人性别为【%s】，岗位要求为【%s】", prof.Gender, criteria.Gender)
			return "UNFIT", "GENDER_MISMATCH", "性别不匹配", evidence
		}
	}

	// 6. 求职状态限制
	if criteria.JobSeekingStatus != "" && criteria.JobSeekingStatus != "不限" && prof.JobSeekingStatus != "" {
		if strings.Contains(criteria.JobSeekingStatus, "离职") && !strings.Contains(prof.JobSeekingStatus, "离职") {
			evidence = fmt.Sprintf("当前求职状态为【%s】，未达到【离职-随时到岗】要求", prof.JobSeekingStatus)
			return "UNFIT", "STATUS_MISMATCH", "求职状态不符合", evidence
		}
	}

	// 7. 专业背景/关键词匹配
	if strings.TrimSpace(criteria.MajorKeywords) != "" {
		rawKws := strings.FieldsFunc(criteria.MajorKeywords, func(r rune) bool {
			return r == ',' || r == '，' || r == '、' || r == ' ' || r == ';' || r == '；'
		})
		if len(rawKws) > 0 {
			matched := false
			fullTarget := prof.Major + " " + prof.Degree + " " + prof.School + " " + resumeText
			for _, kw := range rawKws {
				kw = strings.TrimSpace(kw)
				if kw != "" && strings.Contains(fullTarget, kw) {
					matched = true
					break
				}
			}
			if !matched {
				evidence = fmt.Sprintf("专业背景中未找到指定关键词【%s】（候选人专业: %s）", criteria.MajorKeywords, prof.Major)
				return "UNFIT", "MAJOR_MISMATCH", "专业背景不符合", evidence
			}
		}
	}

	// 8. 目标城市工作意愿
	if criteria.RejectNonLocalWillingness && strings.TrimSpace(criteria.TargetCity) != "" {
		targetCity := strings.TrimSpace(criteria.TargetCity)
		if len(prof.IntentCities) > 0 {
			hasTarget := false
			for _, city := range prof.IntentCities {
				if strings.Contains(city, targetCity) || strings.Contains(targetCity, city) {
					hasTarget = true
					break
				}
			}
			if !hasTarget {
				evidence = fmt.Sprintf("期望工作城市仅包含【%s】，明确表示不愿在目标城市【%s】工作", strings.Join(prof.IntentCities, "、"), targetCity)
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
	var rejectCity, compliance, autoReject, schedEnabled int
	err = db.QueryRow(`SELECT project_id, min_education, min_work_years, min_age, max_age, max_expected_salary,
		gender, job_seeking_status, major_keywords, reject_non_local_willingness, target_city, compliance_confirmed, auto_reject_platform,
		COALESCE(activity_days, 90), COALESCE(criteria_version, 'v1'), COALESCE(project_intro_message, ''), COALESCE(schedule_time, '09:30'), COALESCE(schedule_enabled, 0), updated_at
		FROM talent_management_strict_criteria WHERE project_id=?`, projectID).Scan(
		&c.ProjectID, &c.MinEducation, &c.MinWorkYears, &c.MinAge, &c.MaxAge, &c.MaxExpectedSalary,
		&c.Gender, &c.JobSeekingStatus, &c.MajorKeywords, &rejectCity, &c.TargetCity, &compliance, &autoReject,
		&c.ActivityDays, &c.CriteriaVersion, &c.ProjectIntroMessage, &c.ScheduleTime, &schedEnabled, &c.UpdatedAt)
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
			ActivityDays:              90,
			CriteriaVersion:           "v1",
			ProjectIntroMessage:       "",
			ScheduleTime:              "09:30",
			ScheduleEnabled:           false,
			ComplianceConfirmed:       false,
			AutoRejectPlatform:        true,
			UpdatedAt:                 time.Now().Format("2006-01-02 15:04:05"),
		}
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	c.RejectNonLocalWillingness = rejectCity == 1
	c.ComplianceConfirmed = compliance == 1
	c.AutoRejectPlatform = autoReject == 1
	c.ScheduleEnabled = schedEnabled == 1
	// 注意：0 表示“不限活跃时间”，切勿覆盖篡改为默认值
	if c.CriteriaVersion == "" {
		c.CriteriaVersion = "v1"
	}
	if c.ScheduleTime == "" {
		c.ScheduleTime = "09:30"
	}
	return &c, nil
}

// SetTalentManagementStrictCriteria 保存岗位专属硬性条件
func (a *App) SetTalentManagementStrictCriteria(criteria TalentManagementStrictCriteria) error {
	if criteria.ProjectID == "" {
		return fmt.Errorf("必须指定本地职位分库 ID")
	}
	validDays := map[int]bool{0: true, 3: true, 7: true, 30: true, 90: true}
	if !validDays[criteria.ActivityDays] {
		return fmt.Errorf("活跃天数仅支持 0(不限), 3, 7, 30, 90 天，当前值: %d", criteria.ActivityDays)
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
	autoReject := 0
	if criteria.AutoRejectPlatform {
		autoReject = 1
	}
	schedEnabled := 0
	if criteria.ScheduleEnabled {
		schedEnabled = 1
	}
	if criteria.ScheduleTime == "" {
		criteria.ScheduleTime = "09:30"
	}

	// 自动生成递增条件版本号
	versionNum := 1
	var existingVer string
	_ = db.QueryRow(`SELECT criteria_version FROM talent_management_strict_criteria WHERE project_id=?`, criteria.ProjectID).Scan(&existingVer)
	if strings.HasPrefix(existingVer, "v") {
		if num, err := strconv.Atoi(strings.TrimPrefix(existingVer, "v")); err == nil {
			versionNum = num + 1
		}
	}
	criteria.CriteriaVersion = fmt.Sprintf("v%d", versionNum)

	now := time.Now().Format("2006-01-02 15:04:05")
	_, err = db.Exec(`INSERT INTO talent_management_strict_criteria(project_id, min_education, min_work_years, min_age, max_age, max_expected_salary,
		gender, job_seeking_status, major_keywords, reject_non_local_willingness, target_city, compliance_confirmed, auto_reject_platform,
		activity_days, criteria_version, project_intro_message, schedule_time, schedule_enabled, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(project_id) DO UPDATE SET min_education=excluded.min_education, min_work_years=excluded.min_work_years,
		min_age=excluded.min_age, max_age=excluded.max_age, max_expected_salary=excluded.max_expected_salary,
		gender=excluded.gender, job_seeking_status=excluded.job_seeking_status, major_keywords=excluded.major_keywords,
		reject_non_local_willingness=excluded.reject_non_local_willingness, target_city=excluded.target_city,
		compliance_confirmed=excluded.compliance_confirmed, auto_reject_platform=excluded.auto_reject_platform,
		activity_days=excluded.activity_days, criteria_version=excluded.criteria_version,
		project_intro_message=excluded.project_intro_message, schedule_time=excluded.schedule_time,
		schedule_enabled=excluded.schedule_enabled, updated_at=excluded.updated_at`,
		criteria.ProjectID, criteria.MinEducation, criteria.MinWorkYears, criteria.MinAge, criteria.MaxAge, criteria.MaxExpectedSalary,
		criteria.Gender, criteria.JobSeekingStatus, criteria.MajorKeywords, rejectCity, criteria.TargetCity, compliance, autoReject,
		criteria.ActivityDays, criteria.CriteriaVersion, criteria.ProjectIntroMessage, criteria.ScheduleTime, schedEnabled, now)
	if err != nil {
		return err
	}

	// 智能自愈：若 HR 调整了硬性条件（如将本科调整为大专或不限），自动将此前因该条件归档但现在符合条件的候选人恢复到人才库活跃状态
	rows, err := db.Query(`SELECT pc.id, COALESCE(rv.content, ''), COALESCE(rv.raw_json, '')
		FROM project_candidates pc
		LEFT JOIN project_candidate_resumes pcr ON pcr.project_candidate_id = pc.id
		LEFT JOIN resume_versions rv ON rv.id = pcr.resume_version_id
		WHERE pc.project_id = ? AND pc.archived_at <> ''`, criteria.ProjectID)
	if err != nil {
		return fmt.Errorf("条件已写入，但核对归档候选人失败: %w", err)
	}
	var restoreIDs []string
	for rows.Next() {
		var pcID, content, rawJSON string
		if err := rows.Scan(&pcID, &content, &rawJSON); err != nil {
			rows.Close()
			return fmt.Errorf("条件已写入，但读取归档候选人失败: %w", err)
		}
		resumeContent := content
		if rawJSON != "" {
			var r Resume
			if json.Unmarshal([]byte(rawJSON), &r) == nil && strings.TrimSpace(r.Content) != "" {
				resumeContent = r.Content
			}
		}
		if strings.TrimSpace(resumeContent) != "" {
			decision, _, _, _ := EvaluateCandidateStrictCriteria(resumeContent, criteria)
			if decision != "UNFIT" {
				restoreIDs = append(restoreIDs, pcID)
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("条件已写入，但扫描归档候选人失败: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("条件已写入，但关闭归档查询失败: %w", err)
	}
	nowISO := time.Now().UTC().Format(time.RFC3339Nano)
	for _, pcID := range restoreIDs {
		if _, err := db.Exec(`UPDATE project_candidates SET archived_at='', stage='new', updated_at=? WHERE id=?`, nowISO, pcID); err != nil {
			return fmt.Errorf("条件已写入，但恢复候选人失败: %w", err)
		}
	}

	return nil
}

// ClearTalentManagementAuditLogs 清空指定岗位的淘汰审计日志（若 projectID 为空则清空全部）
func (a *App) ClearTalentManagementAuditLogs(projectID string) error {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	if strings.TrimSpace(projectID) == "" {
		_, err = db.Exec(`DELETE FROM talent_management_audit_logs`)
	} else {
		_, err = db.Exec(`DELETE FROM talent_management_audit_logs WHERE project_id=?`, projectID)
	}
	return err
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

// OneClickFilterTalentPoolWithConfig 执行专属分库一键筛选，支持实时参数与双向联动
func (a *App) OneClickFilterTalentPoolWithConfig(projectID string, criteria TalentManagementStrictCriteria, saveAsDefault bool) (*OneClickFilterResult, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("请先选择具体的职位分库")
	}

	if saveAsDefault {
		_ = a.SetTalentManagementStrictCriteria(criteria)
	}

	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
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
		return nil, err
	}
	defer rows.Close()

	type candidateRecord struct {
		pcID                string
		candidateID         string
		name                string
		resumeContent       string
		platform            string
		platformCandidateID string
	}

	var allCandidates []candidateRecord
	for rows.Next() {
		var pcID, cID, name, content, rawJson string
		if err := rows.Scan(&pcID, &cID, &name, &content, &rawJson); err != nil {
			return nil, err
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
		allCandidates = append(allCandidates, candidateRecord{
			pcID:                pcID,
			candidateID:         cID,
			name:                name,
			resumeContent:       resumeContent,
			platform:            platform,
			platformCandidateID: platformCandidateID,
		})
	}
	rows.Close()

	result := &OneClickFilterResult{
		TotalChecked: len(allCandidates),
		Details:      make([]string, 0),
	}

	if len(allCandidates) == 0 {
		return result, nil
	}

	type unfitItem struct {
		rec         candidateRecord
		ruleCode    string
		reasonLabel string
		evidence    string
	}
	var unfitness []unfitItem

	for _, cand := range allCandidates {
		if strings.TrimSpace(cand.resumeContent) == "" {
			continue
		}
		decision, ruleCode, reasonLabel, evidence := EvaluateCandidateStrictCriteria(cand.resumeContent, criteria)
		if decision == "UNFIT" {
			unfitness = append(unfitness, unfitItem{
				rec:         cand,
				ruleCode:    ruleCode,
				reasonLabel: reasonLabel,
				evidence:    evidence,
			})
		}
	}

	result.UnfitCount = len(unfitness)
	result.ActiveRemainingCount = result.TotalChecked - result.UnfitCount

	if len(unfitness) == 0 {
		return result, nil
	}

	pcIDs := make([]string, 0, len(unfitness))
	for _, u := range unfitness {
		pcIDs = append(pcIDs, u.rec.pcID)
	}

	// 1. 本地批量归档移出人才库
	_, _ = a.ArchiveTalentCandidates(pcIDs)

	nowIso := time.Now().UTC().Format(time.RFC3339Nano)
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	for _, u := range unfitness {
		// 更新本地候选人阶段为 rejected
		_, _ = db.Exec(`UPDATE project_candidates SET stage='rejected', updated_at=? WHERE id=?`, nowIso, u.rec.pcID)

		accountID := ""
		platformJobID := ""
		var recordID string
		_ = db.QueryRow(`SELECT platform_account, platform_job_id, platform_application_id FROM applications
			WHERE platform=? AND (candidate_id=? OR platform_candidate_id=?) AND project_id=?`,
			u.rec.platform, u.rec.candidateID, u.rec.platformCandidateID, projectID).Scan(&accountID, &platformJobID, &recordID)

		if recordID != "" {
			_, _ = db.Exec(`UPDATE applications SET status='unfit_archived', updated_at=? WHERE platform=? AND platform_application_id=?`,
				nowIso, u.rec.platform, recordID)
		}

		receipt := "已移入本地淘汰归档"
		platformSuccess := false

		// 2. 联动 51job 平台点击不合适
		if (criteria.AutoRejectPlatform || criteria.ComplianceConfirmed) && u.rec.platform == "51job" && accountID != "" && platformJobID != "" && recordID != "" {
			platformReason := "其他原因"
			switch u.ruleCode {
			case "EDU_BELOW_MIN":
				platformReason = "学历不匹配"
			case "EXP_BELOW_MIN":
				platformReason = "工作年限不匹配"
			case "AGE_MISMATCH":
				platformReason = "其他原因"
			case "SALARY_MISMATCH":
				platformReason = "其他原因"
			case "INTENT_CITY_MISMATCH":
				platformReason = "工作地点不匹配"
			}

			rejectOut, rejectErr := runTalentManagementAdapter(30*time.Second, "--mode", "reject", "--job-id", platformJobID,
				"--candidate-id", u.rec.platformCandidateID, "--record-id", recordID, "--reason-label", platformReason)
			if rejectErr == nil {
				platformSuccess = true
				result.PlatformRejectedCount++
				var resp struct {
					Receipt string `json:"receipt"`
				}
				_ = json.Unmarshal(rejectOut, &resp)
				receipt = resp.Receipt
				if receipt == "" {
					receipt = "已在 51job 标记不合适（" + platformReason + "）"
				}
			} else {
				receipt = "本地已淘汰；51job 平台点击未完成: " + rejectErr.Error()
			}
		}

		// 3. 记录淘汰审计日志
		auditID := recruitmentID("tmaudit")
		actionStatus := "archived"
		if platformSuccess {
			actionStatus = "rejected"
		}
		_, _ = db.Exec(`INSERT INTO talent_management_audit_logs
			(id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, rule_code, reason_label, evidence, platform_receipt, action_status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			auditID, projectID, u.rec.platform, accountID, platformJobID, u.rec.candidateID, recordID, u.rec.name, u.ruleCode, u.reasonLabel, u.evidence, receipt, actionStatus, nowStr)

		result.Details = append(result.Details, fmt.Sprintf("%s: %s (%s)", u.rec.name, u.reasonLabel, receipt))
	}

	return result, nil
}

// OneClickFilterTalentPool 兼容旧接口
func (a *App) OneClickFilterTalentPool(projectID string) (int, error) {
	criteria, err := a.GetTalentManagementStrictCriteria(projectID)
	if err != nil || criteria == nil {
		return 0, fmt.Errorf("未能获取到职位硬性条件")
	}
	res, err := a.OneClickFilterTalentPoolWithConfig(projectID, *criteria, false)
	if err != nil {
		return 0, err
	}
	return res.UnfitCount, nil
}
