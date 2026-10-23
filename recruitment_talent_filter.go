package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/xuri/excelize/v2"
)

// TalentManagementFilterConfig 51job「人才管理」临时筛选条件配置
type TalentManagementFilterConfig struct {
	JobTitle            string  `json:"jobTitle"`
	MinEducation        string  `json:"minEducation"`        // 大专 / 本科 / 硕士 / 博士 / ""
	MinWorkYears        float64 `json:"minWorkYears"`        // 最低工作年限，0 表示不限
	ExcludeStudents     bool    `json:"excludeStudents"`     // 排除在校生/未毕业人选
	AllowedResidences   string  `json:"allowedResidences"`   // 允许的现居地城市（逗号或空格分隔）
	AllowedIntentCities string  `json:"allowedIntentCities"` // 允许的求职意向城市
	AllowedMajors       string  `json:"allowedMajors"`       // 允许的专业关键词
	MaxSalaryK          float64 `json:"maxSalaryK"`          // 期望月薪上限（单位：K/月，仅候选人最低期望高于此值才淘汰）
	EnableAgeFilter     bool    `json:"enableAgeFilter"`     // 是否启用年龄筛选（合规受控）
	MinAge              int     `json:"minAge"`              // 最小年龄
	MaxAge              int     `json:"maxAge"`              // 最大年龄
	EnableGenderFilter  bool    `json:"enableGenderFilter"`  // 是否启用性别筛选（合规受控）
	AllowedGender       string  `json:"allowedGender"`       // 男 / 女 / ""
	ComplianceConfirmed bool    `json:"complianceConfirmed"` // HR 已勾选合规确认
	MaxCandidates       int     `json:"maxCandidates"`       // 单次扫描/处理上限（默认 200）
}

// TalentManagementFilterRuleHit 命中的单条规则或未知项说明
type TalentManagementFilterRuleHit struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// TalentManagementFilterCardItem 单张候选人卡片解析与筛选判定结果
type TalentManagementFilterCardItem struct {
	CandidateKey        string                          `json:"candidateKey"`
	ResumeID            string                          `json:"resumeId"`
	ProfileURL          string                          `json:"profileUrl"`
	CandidateName       string                          `json:"candidateName"`
	Gender              string                          `json:"gender,omitempty"`
	Age                 *int                            `json:"age,omitempty"`
	WorkYears           *float64                        `json:"workYears,omitempty"`
	IsStudent           *bool                           `json:"isStudent,omitempty"`
	Education           string                          `json:"education,omitempty"`
	EduRank             *int                            `json:"eduRank,omitempty"`
	Residence           string                          `json:"residence,omitempty"`
	IntentRole          string                          `json:"intentRole,omitempty"`
	IntentCity          string                          `json:"intentCity,omitempty"`
	IntentSalaryRaw     string                          `json:"intentSalaryRaw,omitempty"`
	SalaryMinK          *float64                        `json:"salaryMinK,omitempty"`
	SalaryMaxK          *float64                        `json:"salaryMaxK,omitempty"`
	School              string                          `json:"school,omitempty"`
	Major               string                          `json:"major,omitempty"`
	EduGradRange        string                          `json:"eduGradRange,omitempty"`
	RecentCompany       string                          `json:"recentCompany,omitempty"`
	RecentRole          string                          `json:"recentRole,omitempty"`
	Skills              []string                        `json:"skills,omitempty"`
	AppliedDate         string                          `json:"appliedDate,omitempty"`
	RawText             string                          `json:"rawText,omitempty"`
	Decision            string                          `json:"decision"` // UNFIT | KEEP_MATCHED | KEEP_UNKNOWN | WHITELIST_SKIP
	FailedRules         []TalentManagementFilterRuleHit `json:"failedRules,omitempty"`
	UnknownRules        []TalentManagementFilterRuleHit `json:"unknownRules,omitempty"`
	PrimaryReasonCode   string                          `json:"primaryReasonCode,omitempty"`
	ReasonLabel         string                          `json:"reasonLabel,omitempty"`
	ActionStatus        string                          `json:"actionStatus,omitempty"`
	ActionMessage       string                          `json:"actionMessage,omitempty"`
	ClickedReasonButton string                          `json:"clickedReasonButton,omitempty"`
	ImportedResumeID    string                          `json:"importedResumeId,omitempty"`
}

// TalentManagementFilterSummary 试算或执行汇总统计
type TalentManagementFilterSummary struct {
	TotalScanned       int `json:"totalScanned"`
	UnfitCount         int `json:"unfitCount"`
	KeepMatchedCount   int `json:"keepMatchedCount"`
	KeepUnknownCount   int `json:"keepUnknownCount"`
	WhitelistSkipCount int `json:"whitelistSkipCount"`
	ImportedCount      int `json:"importedCount"`
}

// TalentManagementFilterRun 一次完整的试算或全自动筛选记录
type TalentManagementFilterRun struct {
	ID           string                         `json:"id"`
	ProjectID    string                         `json:"projectId"`
	JobTitle     string                         `json:"jobTitle"`
	Mode         string                         `json:"mode"`   // dry_run | execute
	Status       string                         `json:"status"` // ok | halted | error | running
	FilterConfig TalentManagementFilterConfig   `json:"filterConfig"`
	Summary      TalentManagementFilterSummary  `json:"summary"`
	Items        []TalentManagementFilterCardItem `json:"items"`
	Message      string                         `json:"message"`
	CreatedAt    string                         `json:"createdAt"`
	UpdatedAt    string                         `json:"updatedAt"`
}

func resolveTalentManagementFilterScript() string {
	candidates := []string{
		filepath.Join("scripts", "51job_talent_management_filter.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "51job_talent_management_filter.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "51job_talent_management_filter.js"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// collectTalentManagementWhitelist 汇总当前项目与人才库中受保护的候选人标识（已入库合适/面试/录用）
func (a *App) collectTalentManagementWhitelist(projectID string) []string {
	seen := make(map[string]bool)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" {
			seen[v] = true
		}
	}

	// 1. 当前项目已有简历中已评为强烈推荐/推荐，或已跟进的候选人
	for _, r := range a.GetProjectResumes(projectID) {
		if r == nil {
			continue
		}
		if r.Score >= 75 || r.ContactStatus != "" {
			add(r.PlatformCandidateID)
			add("name:" + strings.TrimSpace(r.FileName))
			if r.Analysis != nil && r.Analysis.CandidateName != "" {
				add("name:" + strings.TrimSpace(r.Analysis.CandidateName))
			}
		}
	}

	// 2. SQLite 中处于 screening / interview / offer / hired 阶段的候选人
	if db, err := a.getRecruitmentDB(); err == nil && db != nil {
		rows, qErr := db.Query(`
			SELECT c.display_name, COALESCE(cs.platform_candidate_id, '')
			FROM project_candidates pc
			JOIN candidates c ON c.id = pc.candidate_id
			LEFT JOIN candidate_sources cs ON cs.candidate_id = c.id AND cs.platform = '51job'
			WHERE pc.archived_at = '' AND (
				pc.stage IN ('screening', 'interview', 'offer', 'hired', 'suitable', 'contacted')
				OR pc.project_id = ?
			)`, projectID)
		if qErr == nil {
			defer rows.Close()
			for rows.Next() {
				var name, platID string
				if rows.Scan(&name, &platID) == nil {
					if platID != "" {
						add(platID)
						add("51job_" + platID)
					}
					if name != "" {
						add("name:" + name)
					}
				}
			}
		}
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

func validateTalentFilterCompliance(cfg TalentManagementFilterConfig) error {
	if (cfg.EnableAgeFilter || cfg.EnableGenderFilter) && !cfg.ComplianceConfirmed {
		return fmt.Errorf("已开启年龄或性别筛选，请先勾选「已确认岗位有合法特殊理由」合规声明")
	}
	if cfg.EnableAgeFilter && cfg.MinAge > 0 && cfg.MaxAge > 0 && cfg.MinAge > cfg.MaxAge {
		return fmt.Errorf("最小年龄不能大于最大年龄")
	}
	return nil
}

// Preview51JobTalentManagementFilter Phase 1: 只读试算（不点任何不合适按钮）
func (a *App) Preview51JobTalentManagementFilter(projectID string, cfg TalentManagementFilterConfig) (TalentManagementFilterRun, error) {
	return a.execute51JobTalentManagementFilterInternal(projectID, cfg, "dry_run")
}

// Run51JobTalentManagementFilter Phase 2: 确认后全自动连续执行（点不合适 + 勾选原因 + 保留人选导入待分析）
func (a *App) Run51JobTalentManagementFilter(projectID string, cfg TalentManagementFilterConfig) (TalentManagementFilterRun, error) {
	return a.execute51JobTalentManagementFilterInternal(projectID, cfg, "execute")
}

// Stop51JobTalentManagementFilter 随时优雅停止当前正在执行的自动筛选任务
func (a *App) Stop51JobTalentManagementFilter() (bool, string) {
	a.talentFilterMu.Lock()
	stopFile := a.talentFilterStopFile
	a.talentFilterMu.Unlock()
	if stopFile == "" {
		return false, "当前没有正在运行的 51job 投递筛选任务"
	}
	if err := os.WriteFile(stopFile, []byte("stop"), 0644); err != nil {
		return false, "写入停止信号失败: " + err.Error()
	}
	return true, "已发送停止指令，将在当前候选人操作完成后立即安全停机"
}

func (a *App) execute51JobTalentManagementFilterInternal(projectID string, cfg TalentManagementFilterConfig, mode string) (TalentManagementFilterRun, error) {
	run := TalentManagementFilterRun{
		ID:           recruitmentID("tm_filter"),
		ProjectID:    projectID,
		Mode:         mode,
		Status:       "error",
		FilterConfig: cfg,
		Items:        make([]TalentManagementFilterCardItem, 0),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err := validateTalentFilterCompliance(cfg); err != nil {
		run.Message = err.Error()
		return run, err
	}

	proj := a.GetProject(projectID)
	if proj == nil {
		err := fmt.Errorf("请先选择有效的招聘职位项目")
		run.Message = err.Error()
		return run, err
	}
	if strings.TrimSpace(cfg.JobTitle) == "" {
		cfg.JobTitle = strings.TrimSpace(proj.JobConfig.Title)
		if cfg.JobTitle == "" {
			cfg.JobTitle = strings.TrimSpace(proj.Name)
		}
	}
	if cfg.MaxCandidates <= 0 || cfg.MaxCandidates > 500 {
		cfg.MaxCandidates = 200
	}
	run.JobTitle = cfg.JobTitle
	run.FilterConfig = cfg

	script := resolveTalentManagementFilterScript()
	if script == "" {
		err := fmt.Errorf("缺少 51job 人才管理自动筛选脚本 (scripts/51job_talent_management_filter.js)")
		run.Message = err.Error()
		return run, err
	}

	whitelist := a.collectTalentManagementWhitelist(projectID)
	filterBytes, _ := json.Marshal(cfg)
	whitelistBytes, _ := json.Marshal(whitelist)

	stopFile := filepath.Join(a.getDataDir(), fmt.Sprintf("tm_filter_stop_%d.flag", time.Now().UnixNano()))
	_ = os.Remove(stopFile)
	a.talentFilterMu.Lock()
	a.talentFilterStopFile = stopFile
	a.talentFilterMu.Unlock()
	defer func() {
		_ = os.Remove(stopFile)
		a.talentFilterMu.Lock()
		if a.talentFilterStopFile == stopFile {
			a.talentFilterStopFile = ""
		}
		a.talentFilterMu.Unlock()
	}()

	a.EnsurePlatformBrowser("51job")

	timeout := 3 * time.Minute
	if mode == "execute" {
		timeout = 25 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"node",
		script,
		"--mode", mode,
		"--job-title", cfg.JobTitle,
		"--max-candidates", fmt.Sprintf("%d", cfg.MaxCandidates),
		"--filter-json", string(filterBytes),
		"--whitelist-json", string(whitelistBytes),
		"--stop-file", stopFile,
	)
	hideConsoleWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		run.Message = "启动筛选进程失败: " + err.Error()
		return run, err
	}
	if err := cmd.Start(); err != nil {
		run.Message = "启动 Node 筛选引擎失败: " + err.Error()
		return run, err
	}

	var finalPayload struct {
		Type    string                           `json:"type"`
		Status  string                           `json:"status"`
		Mode    string                           `json:"mode"`
		Message string                           `json:"message"`
		Summary TalentManagementFilterSummary    `json:"summary"`
		Items   []TalentManagementFilterCardItem `json:"items"`
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var base struct {
			Type           string                         `json:"type"`
			ProcessedCount int                            `json:"processedCount"`
			Item           TalentManagementFilterCardItem `json:"item"`
		}
		if json.Unmarshal([]byte(line), &base) != nil {
			continue
		}
		if base.Type == "progress" {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "51job-talent-filter-progress", map[string]any{
					"runId":          run.ID,
					"projectId":      projectID,
					"processedCount": base.ProcessedCount,
					"item":           base.Item,
				})
			}
		} else if base.Type == "result" {
			_ = json.Unmarshal([]byte(line), &finalPayload)
		}
	}
	_ = cmd.Wait()

	if finalPayload.Status == "" {
		finalPayload.Status = "error"
		if finalPayload.Message == "" {
			finalPayload.Message = "未能从 51job 人才管理页面获取筛选结果，请确认已登录并打开人才管理页面"
		}
	}

	run.Status = finalPayload.Status
	run.Message = finalPayload.Message
	run.Summary = finalPayload.Summary
	run.Items = finalPayload.Items
	run.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	// 在 execute 模式下，将 KEEP_MATCHED 与 KEEP_UNKNOWN 候选人自动导入到当前项目作为待分析简历
	if mode == "execute" && len(run.Items) > 0 {
		importedCount := 0
		for i := range run.Items {
			item := &run.Items[i]
			if item.Decision != "KEEP_MATCHED" && item.Decision != "KEEP_UNKNOWN" {
				continue
			}
			resumeID, impErr := a.importKeptTalentManagementCandidate(projectID, *item)
			if impErr == nil && resumeID != "" {
				item.ImportedResumeID = resumeID
				importedCount++
			}
		}
		run.Summary.ImportedCount = importedCount
		if importedCount > 0 {
			run.Message = fmt.Sprintf("%s；已将 %d 位保留候选人同步导入当前职位待分析列表", run.Message, importedCount)
		}
	}

	_ = a.saveTalentManagementFilterRun(run)

	if run.Status == "needs_login" || run.Status == "needs_action" || run.Status == "error" {
		return run, fmt.Errorf("%s", run.Message)
	}
	return run, nil
}

// importKeptTalentManagementCandidate 将通过初筛或资料未知保留的候选人卡片写入项目简历列表
func (a *App) importKeptTalentManagementCandidate(projectID string, item TalentManagementFilterCardItem) (string, error) {
	if strings.TrimSpace(item.CandidateName) == "" {
		return "", fmt.Errorf("候选人姓名为空")
	}
	seed := fmt.Sprintf("51job_tm_%s_%s_%s", projectID, item.CandidateKey, item.CandidateName)
	sum := sha256.Sum256([]byte(seed))
	resumeID := "tm51_" + hex.EncodeToString(sum[:10])

	var b strings.Builder
	b.WriteString(fmt.Sprintf("【姓名】%s\n", item.CandidateName))
	b.WriteString("【来源渠道】前程无忧(51job) 人才管理-待处理投递\n")
	if item.Decision == "KEEP_UNKNOWN" {
		b.WriteString(fmt.Sprintf("【初筛标记】⚠️ %s\n", item.ReasonLabel))
	} else {
		b.WriteString("【初筛标记】✅ 硬性筛选条件全部符合\n")
	}
	if item.Gender != "" {
		b.WriteString(fmt.Sprintf("【性别】%s\n", item.Gender))
	}
	if item.Age != nil {
		b.WriteString(fmt.Sprintf("【年龄】%d岁\n", *item.Age))
	}
	if item.WorkYears != nil {
		b.WriteString(fmt.Sprintf("【工作年限】%.1f年\n", *item.WorkYears))
	}
	if item.Education != "" {
		b.WriteString(fmt.Sprintf("【最高学历】%s\n", item.Education))
	}
	if item.Residence != "" {
		b.WriteString(fmt.Sprintf("【现居地】%s\n", item.Residence))
	}
	if item.IntentRole != "" || item.IntentCity != "" || item.IntentSalaryRaw != "" {
		b.WriteString(fmt.Sprintf("【求职意向】%s | 城市：%s | 期望薪资：%s\n", item.IntentRole, item.IntentCity, item.IntentSalaryRaw))
	}
	if item.School != "" || item.Major != "" {
		b.WriteString(fmt.Sprintf("【教育经历】%s %s · %s · %s\n", item.EduGradRange, item.School, item.Major, item.Education))
	}
	if item.RecentCompany != "" || item.RecentRole != "" {
		b.WriteString(fmt.Sprintf("【最近履历】%s · %s\n", item.RecentCompany, item.RecentRole))
	}
	if len(item.Skills) > 0 {
		b.WriteString(fmt.Sprintf("【技能&经验】%s\n", strings.Join(item.Skills, "、")))
	}
	if strings.TrimSpace(item.RawText) != "" {
		b.WriteString("\n【平台卡片原文快照】\n")
		b.WriteString(strings.TrimSpace(item.RawText))
	}

	content := b.String()
	resume := &Resume{
		ID:                  resumeID,
		ProjectID:           projectID,
		FileName:            item.CandidateName,
		Content:             content,
		FileType:            ".txt",
		FileSize:            int64(len(content)),
		URL:                 item.ProfileURL,
		SourcePlatform:      "51job",
		PlatformCandidateID: item.ResumeID,
		SourceKeyword:       "51job人才管理投递",
		Status:              "pending",
		CreatedAt:           time.Now(),
	}
	if err := a.persistSearchCandidate(resume); err != nil {
		return "", err
	}
	return resumeID, nil
}

func (a *App) saveTalentManagementFilterRun(run TalentManagementFilterRun) error {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	if proj := a.GetProject(run.ProjectID); proj != nil {
		a.syncRecruitmentProject(proj)
	}
	filterRaw, _ := json.Marshal(run.FilterConfig)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO talent_management_filter_runs(
			id, project_id, job_title, mode, status, filter_json,
			total_scanned, unfit_count, keep_matched_count, keep_unknown_count,
			whitelist_skip_count, imported_count, message, created_at, updated_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status,
			total_scanned=excluded.total_scanned,
			unfit_count=excluded.unfit_count,
			keep_matched_count=excluded.keep_matched_count,
			keep_unknown_count=excluded.keep_unknown_count,
			whitelist_skip_count=excluded.whitelist_skip_count,
			imported_count=excluded.imported_count,
			message=excluded.message,
			updated_at=excluded.updated_at`,
		run.ID, run.ProjectID, run.JobTitle, run.Mode, run.Status, string(filterRaw),
		run.Summary.TotalScanned, run.Summary.UnfitCount, run.Summary.KeepMatchedCount,
		run.Summary.KeepUnknownCount, run.Summary.WhitelistSkipCount, run.Summary.ImportedCount,
		run.Message, run.CreatedAt, run.UpdatedAt,
	)
	if err != nil {
		return err
	}

	for idx, item := range run.Items {
		cardRaw, _ := json.Marshal(item)
		itemID := fmt.Sprintf("%s_item_%d", run.ID, idx+1)
		_, err = tx.Exec(`
			INSERT OR REPLACE INTO talent_management_filter_items(
				id, run_id, project_id, candidate_key, resume_id, candidate_name,
				decision, primary_reason_code, reason_label, clicked_reason_button,
				action_status, card_json, created_at
			) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			itemID, run.ID, run.ProjectID, item.CandidateKey, item.ResumeID, item.CandidateName,
			item.Decision, item.PrimaryReasonCode, item.ReasonLabel, item.ClickedReasonButton,
			item.ActionStatus, string(cardRaw), run.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// List51JobTalentFilterRuns 查询指定项目的历史筛选与淘汰审计批次列表
func (a *App) List51JobTalentFilterRuns(projectID string) ([]TalentManagementFilterRun, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`
		SELECT id, project_id, job_title, mode, status, filter_json,
		       total_scanned, unfit_count, keep_matched_count, keep_unknown_count,
		       whitelist_skip_count, imported_count, message, created_at, updated_at
		FROM talent_management_filter_runs
		WHERE (? = '' OR project_id = ?)
		ORDER BY created_at DESC LIMIT 30`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TalentManagementFilterRun, 0)
	for rows.Next() {
		var r TalentManagementFilterRun
		var filterRaw string
		err := rows.Scan(
			&r.ID, &r.ProjectID, &r.JobTitle, &r.Mode, &r.Status, &filterRaw,
			&r.Summary.TotalScanned, &r.Summary.UnfitCount, &r.Summary.KeepMatchedCount,
			&r.Summary.KeepUnknownCount, &r.Summary.WhitelistSkipCount, &r.Summary.ImportedCount,
			&r.Message, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(filterRaw), &r.FilterConfig)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Get51JobTalentFilterRunDetail 查询单次筛选批次及其候选人审计明细
func (a *App) Get51JobTalentFilterRunDetail(runID string) (TalentManagementFilterRun, error) {
	var run TalentManagementFilterRun
	db, err := a.getRecruitmentDB()
	if err != nil {
		return run, err
	}
	var filterRaw string
	err = db.QueryRow(`
		SELECT id, project_id, job_title, mode, status, filter_json,
		       total_scanned, unfit_count, keep_matched_count, keep_unknown_count,
		       whitelist_skip_count, imported_count, message, created_at, updated_at
		FROM talent_management_filter_runs WHERE id = ?`, runID).Scan(
		&run.ID, &run.ProjectID, &run.JobTitle, &run.Mode, &run.Status, &filterRaw,
		&run.Summary.TotalScanned, &run.Summary.UnfitCount, &run.Summary.KeepMatchedCount,
		&run.Summary.KeepUnknownCount, &run.Summary.WhitelistSkipCount, &run.Summary.ImportedCount,
		&run.Message, &run.CreatedAt, &run.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return run, fmt.Errorf("未找到对应的筛选批次记录")
		}
		return run, err
	}
	_ = json.Unmarshal([]byte(filterRaw), &run.FilterConfig)

	rows, err := db.Query(`SELECT card_json FROM talent_management_filter_items WHERE run_id = ? ORDER BY rowid ASC`, runID)
	if err != nil {
		return run, err
	}
	defer rows.Close()
	run.Items = make([]TalentManagementFilterCardItem, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return run, err
		}
		var item TalentManagementFilterCardItem
		if json.Unmarshal([]byte(raw), &item) == nil {
			run.Items = append(run.Items, item)
		}
	}
	return run, rows.Err()
}

// Export51JobTalentFilterReport 导出 51job 人才管理自动筛选与淘汰审计 Excel 报表
func (a *App) Export51JobTalentFilterReport(runID string) (string, error) {
	run, err := a.Get51JobTalentFilterRunDetail(runID)
	if err != nil {
		return "", err
	}

	f := excelize.NewFile()
	sheet := "51job投递筛选审计"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"序号", "候选人姓名", "判定分类", "命中规则 / 保留原因", "51job勾选不合适原因",
		"执行状态", "性别", "年龄", "工作年限", "学历", "毕业院校", "所学专业",
		"毕业时间段", "现居地", "求职意向职位", "意向城市", "期望月薪", "最近任职单位与岗位",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	decisionLabels := map[string]string{
		"UNFIT":          "不合适（淘汰）",
		"KEEP_MATCHED":   "合适保留（导入待分析）",
		"KEEP_UNKNOWN":   "资料未知保留（不误筛）",
		"WHITELIST_SKIP": "白名单保护跳过",
	}

	for idx, item := range run.Items {
		row := idx + 2
		ageStr := "未知"
		if item.Age != nil {
			ageStr = fmt.Sprintf("%d岁", *item.Age)
		}
		wyStr := "未知"
		if item.WorkYears != nil {
			if *item.WorkYears == 0 {
				wyStr = "在校/无经验"
			} else {
				wyStr = fmt.Sprintf("%.1f年", *item.WorkYears)
			}
		}
		recentWork := strings.Trim(strings.Join([]string{item.RecentCompany, item.RecentRole}, " · "), " ·")
		values := []any{
			idx + 1,
			item.CandidateName,
			decisionLabels[item.Decision],
			item.ReasonLabel,
			item.ClickedReasonButton,
			item.ActionMessage,
			item.Gender,
			ageStr,
			wyStr,
			item.Education,
			item.School,
			item.Major,
			item.EduGradRange,
			item.Residence,
			item.IntentRole,
			item.IntentCity,
			item.IntentSalaryRaw,
			recentWork,
		}
		for colIdx, val := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, row)
			_ = f.SetCellValue(sheet, cell, val)
		}
	}

	defaultName := fmt.Sprintf("51job人才管理筛选审计_%s_%s.xlsx",
		strings.ReplaceAll(run.JobTitle, "/", "-"),
		time.Now().Format("20060102_150405"),
	)

	var savePath string
	if a.ctx != nil {
		savePath, _ = runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			Title:           "导出 51job 人才管理自动筛选审计报告",
			DefaultFilename: defaultName,
			Filters: []runtime.FileFilter{
				{DisplayName: "Excel 工作簿 (*.xlsx)", Pattern: "*.xlsx"},
			},
		})
	} else {
		exportDir := filepath.Join(a.getDataDir(), "exports")
		_ = os.MkdirAll(exportDir, 0755)
		savePath = filepath.Join(exportDir, defaultName)
	}

	if strings.TrimSpace(savePath) == "" {
		return "", nil
	}
	if err := f.SaveAs(savePath); err != nil {
		return "", err
	}
	return savePath, nil
}
