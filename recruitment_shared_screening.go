package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// RuleCheckItem 逐项判定规则检查明细（三态：pass / fail / unknown）
type RuleCheckItem struct {
	Rule     string `json:"rule"`     // 规则代码，如 MIN_EDUCATION, MIN_WORK_YEARS, AGE, SALARY, GENDER, STATUS, MAJOR, CITY, ACTIVITY
	Label    string `json:"label"`    // 规则标签，如 "最低学历要求"
	Status   string `json:"status"`   // "pass" (合格) | "fail" (明确不符) | "unknown" (信息缺失/待核实)
	Evidence string `json:"evidence"` // 确凿证据描述
}

// ScreeningCandidateResult 扫描批次中的单条候选人判定结果
type ScreeningCandidateResult struct {
	ID              string          `json:"id"`
	BatchID         string          `json:"batchId"`
	ProjectID       string          `json:"projectId"`
	CandidateID     string          `json:"candidateId"`
	RecordID        string          `json:"recordId"`
	CandidateName   string          `json:"candidateName"`
	Verdict         string          `json:"verdict"` // "qualified" | "unfit" | "pending_verification"
	RuleEvidence    []RuleCheckItem `json:"ruleEvidence"`
	RawProfile      string          `json:"rawProfile"`
	DetailLoaded    bool            `json:"detailLoaded"`
	ActionStatus    string          `json:"actionStatus"` // "pending" | "executing" | "receipt_pending" | "retrying" | "completed" | "verified_on_platform" | "manual_required" | "failed" | "imported"
	ActionReceiptAt string          `json:"actionReceiptAt"`
	ActionError     string          `json:"actionError"`
	PlatformStatus  string          `json:"platformStatus"`
	CreatedAt       string          `json:"createdAt"`
}

// ScreeningBatchPreview 批次扫描预览对象（向 HR 展现合格、不合格、待核实三类名单）
type ScreeningBatchPreview struct {
	BatchID              string                     `json:"batchId"`
	ProjectID            string                     `json:"projectId"`
	Source               string                     `json:"source"` // "talent_management" | "telescope"
	Platform             string                     `json:"platform"`
	PlatformAccount      string                     `json:"platformAccount"`
	PlatformJobID        string                     `json:"platformJobId"`
	BusinessDate         string                     `json:"businessDate"`
	CriteriaVersion      string                     `json:"criteriaVersion"`
	TotalCount           int                        `json:"totalCount"`
	QualifiedCount       int                        `json:"qualifiedCount"`
	UnfitCount           int                        `json:"unfitCount"`
	PendingCount         int                        `json:"pendingCount"`
	QualifiedList        []ScreeningCandidateResult `json:"qualifiedList"`
	UnfitList            []ScreeningCandidateResult `json:"unfitList"`
	PendingList          []ScreeningCandidateResult `json:"pendingList"`
	Status               string                     `json:"status"` // "scanning" | "preview_ready" | "completed" | "failed"
	ScanStatus           string                     `json:"scanStatus"` // "pending" | "scanning" | "paused" | "manual_required" | "complete"
	CurrentPage          int                        `json:"currentPage"`
	NextPage             int                        `json:"nextPage"`
	LastSuccessPage      int                        `json:"lastSuccessPage"`
	TotalPagesEstimated  int                        `json:"totalPagesEstimated"`
	InitialPlatformCount int                        `json:"initialPlatformCount"`
	LastError            string                     `json:"lastError"`
	NextCheckAt          string                     `json:"nextCheckAt"`
}

// PlatformActionLedgerItem 平台动作台账记录
type PlatformActionLedgerItem struct {
	ID              string `json:"id"`
	ProjectID       string `json:"projectId"`
	Platform        string `json:"platform"`
	PlatformAccount string `json:"platformAccount"`
	PlatformJobID   string `json:"platformJobId"`
	CandidateID     string `json:"candidateId"`
	RecordID        string `json:"recordId"`
	CandidateName   string `json:"candidateName"`
	ActionType      string `json:"actionType"` // 'reject' | 'hi' | 'message' | 'import'
	ConfirmedBy     string `json:"confirmedBy"`
	RequestedAt     string `json:"requestedAt"`
	Receipt         string `json:"receipt"`
	Status          string `json:"status"` // 'success' | 'failed' | 'pending_retry'
	ErrorMessage    string `json:"errorMessage"`
	RetryCount      int    `json:"retryCount"`
}

// parseCandidateActivityDays 从简历文本或结构化数据解析候选人活跃天数
func parseCandidateActivityDays(text string) (days int, known bool) {
	// 优先解析结构化时间戳字段如 active_time, modifydate, refresh_date, updatetime, applydate
	var data map[string]interface{}
	if json.Unmarshal([]byte(text), &data) == nil {
		for _, key := range []string{"active_time", "modifydate", "refresh_date", "updatetime", "applydate"} {
			if val, ok := data[key].(string); ok && strings.TrimSpace(val) != "" {
				val = strings.TrimSpace(val)
				for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", "2006/01/02", "2006.01.02"} {
					if t, err := time.Parse(layout, val); err == nil {
						diff := int(time.Since(t).Hours() / 24)
						if diff < 0 {
							diff = 0
						}
						return diff, true
					}
				}
			}
		}
	}

	lower := strings.ToLower(text)
	if strings.Contains(lower, "刚刚活跃") || strings.Contains(lower, "今日活跃") || strings.Contains(lower, "今天活跃") || strings.Contains(lower, "小时前活跃") {
		return 0, true
	}
	if strings.Contains(lower, "24小时内活跃") || strings.Contains(lower, "1天内活跃") || strings.Contains(lower, "昨天活跃") {
		return 1, true
	}
	if strings.Contains(lower, "近3日活跃") || strings.Contains(lower, "3天内活跃") || strings.Contains(lower, "3日内活跃") || strings.Contains(lower, "近3天活跃") {
		return 3, true
	}
	if strings.Contains(lower, "近7天活跃") || strings.Contains(lower, "7天内活跃") || strings.Contains(lower, "7日内活跃") || strings.Contains(lower, "近7日活跃") || strings.Contains(lower, "本周活跃") || strings.Contains(lower, "1周内活跃") {
		return 7, true
	}
	if strings.Contains(lower, "2周内活跃") {
		return 14, true
	}
	if strings.Contains(lower, "近30天活跃") || strings.Contains(lower, "30天内活跃") || strings.Contains(lower, "30日内活跃") || strings.Contains(lower, "近30日活跃") || strings.Contains(lower, "本月活跃") || strings.Contains(lower, "1个月内活跃") || strings.Contains(lower, "近1个月活跃") {
		return 30, true
	}
	if strings.Contains(lower, "近60天活跃") || strings.Contains(lower, "60天内活跃") || strings.Contains(lower, "2个月内活跃") || strings.Contains(lower, "近2个月活跃") {
		return 60, true
	}
	if strings.Contains(lower, "近90天活跃") || strings.Contains(lower, "90天内活跃") || strings.Contains(lower, "3个月内活跃") || strings.Contains(lower, "近3个月活跃") || strings.Contains(lower, "1季度内活跃") {
		return 90, true
	}
	if strings.Contains(lower, "3个月前活跃") || strings.Contains(lower, "3个月以上活跃") || strings.Contains(lower, "半年前活跃") || strings.Contains(lower, "半年以前") || strings.Contains(lower, "1年前活跃") || strings.Contains(lower, "1年以上") {
		return 180, true
	}
	// “半年内活跃”属于模糊区间（0~180天），无法证明是否在90天内，按保守边界归入待核实
	if strings.Contains(lower, "半年内活跃") || strings.Contains(lower, "半年内") {
		return -1, false
	}

	return -1, false
}

// EvaluateCandidateTriState 逐项三态判定引擎（完全满足无序偏序中立：每一项产生 pass/fail/unknown，无短路副作用）
func EvaluateCandidateTriState(resumeText string, criteria TalentManagementStrictCriteria) (verdict string, checks []RuleCheckItem) {
	checks = make([]RuleCheckItem, 0, 9)

	if strings.TrimSpace(resumeText) == "" {
		checks = append(checks, RuleCheckItem{
			Rule:     "DOC_EMPTY",
			Label:    "简历正文完整性",
			Status:   "unknown",
			Evidence: "候选人简历内容为空或尚未取得完整资料，无法完成合规核验",
		})
		return "pending_verification", checks
	}

	prof := extractCandidateProfile(resumeText)

	// 1. 最低学历
	minEduRank := parseEducationRank(criteria.MinEducation)
	if minEduRank > 0 {
		candRank := parseEducationRank(prof.Degree)
		if candRank == 0 {
			checks = append(checks, RuleCheckItem{
				Rule:     "MIN_EDUCATION",
				Label:    "最低学历要求",
				Status:   "unknown",
				Evidence: "最高学历未填或无法解析",
			})
		} else if candRank < minEduRank {
			evidence := fmt.Sprintf("最高学历为【%s】，未达到要求的【%s及以上】", prof.Degree, criteria.MinEducation)
			if prof.School != "" || prof.Major != "" {
				evidence += fmt.Sprintf("（毕业院校: %s, 专业: %s）", prof.School, prof.Major)
			}
			checks = append(checks, RuleCheckItem{
				Rule:     "MIN_EDUCATION",
				Label:    "最低学历要求",
				Status:   "fail",
				Evidence: evidence,
			})
		} else {
			checks = append(checks, RuleCheckItem{
				Rule:     "MIN_EDUCATION",
				Label:    "最低学历要求",
				Status:   "pass",
				Evidence: fmt.Sprintf("最高学历【%s】达到【%s及以上】要求", prof.Degree, criteria.MinEducation),
			})
		}
	}

	// 2. 最低工作年限
	if criteria.MinWorkYears > 0 {
		if !prof.WorkYearsKnown {
			checks = append(checks, RuleCheckItem{
				Rule:     "MIN_WORK_YEARS",
				Label:    "工作年限要求",
				Status:   "unknown",
				Evidence: "工作年限信息缺失，暂无法核查实操经验",
			})
		} else if prof.WorkYears < criteria.MinWorkYears {
			checks = append(checks, RuleCheckItem{
				Rule:     "MIN_WORK_YEARS",
				Label:    "工作年限要求",
				Status:   "fail",
				Evidence: fmt.Sprintf("工作年限为【%.1f年】，低于最低要求的【%.1f年】", prof.WorkYears, criteria.MinWorkYears),
			})
		} else {
			checks = append(checks, RuleCheckItem{
				Rule:     "MIN_WORK_YEARS",
				Label:    "工作年限要求",
				Status:   "pass",
				Evidence: fmt.Sprintf("工作年限【%.1f年】满足最低【%.1f年】门槛", prof.WorkYears, criteria.MinWorkYears),
			})
		}
	}

	// Age and gender are display-only context and never affect the verdict.
	// 3. 期望薪资上限
	if criteria.MaxExpectedSalary > 0 {
		if !prof.SalaryKnown || prof.SalaryMaxK <= 0 {
			checks = append(checks, RuleCheckItem{
				Rule:     "MAX_EXPECTED_SALARY",
				Label:    "期望薪资限制",
				Status:   "unknown",
				Evidence: "期望薪资面议或未明确标明数字",
			})
		} else if prof.SalaryMaxK > float64(criteria.MaxExpectedSalary) {
			checks = append(checks, RuleCheckItem{
				Rule:     "MAX_EXPECTED_SALARY",
				Label:    "期望薪资限制",
				Status:   "fail",
				Evidence: fmt.Sprintf("期望薪资上限约为【%.1f千/月】，超出预算上限【%d千/月】", prof.SalaryMaxK, criteria.MaxExpectedSalary),
			})
		} else {
			checks = append(checks, RuleCheckItem{
				Rule:     "MAX_EXPECTED_SALARY",
				Label:    "期望薪资限制",
				Status:   "pass",
				Evidence: fmt.Sprintf("期望薪资【%.1f千/月】在预算上限【%d千/月】内", prof.SalaryMaxK, criteria.MaxExpectedSalary),
			})
		}
	}

	// 4. 求职状态
	if criteria.JobSeekingStatus != "" && criteria.JobSeekingStatus != "不限" {
		if prof.JobSeekingStatus == "" {
			checks = append(checks, RuleCheckItem{
				Rule:     "JOB_SEEKING_STATUS",
				Label:    "求职状态要求",
				Status:   "unknown",
				Evidence: "求职状态未填写",
			})
		} else if strings.Contains(criteria.JobSeekingStatus, "离职") && !strings.Contains(prof.JobSeekingStatus, "离职") {
			checks = append(checks, RuleCheckItem{
				Rule:     "JOB_SEEKING_STATUS",
				Label:    "求职状态要求",
				Status:   "fail",
				Evidence: fmt.Sprintf("当前状态为【%s】，非随时到岗离职状态", prof.JobSeekingStatus),
			})
		} else {
			checks = append(checks, RuleCheckItem{
				Rule:     "JOB_SEEKING_STATUS",
				Label:    "求职状态要求",
				Status:   "pass",
				Evidence: fmt.Sprintf("求职状态【%s】符合要求", prof.JobSeekingStatus),
			})
		}
	}

	// 7. 专业背景关键词
	if strings.TrimSpace(criteria.MajorKeywords) != "" {
		rawKws := strings.FieldsFunc(criteria.MajorKeywords, func(r rune) bool {
			return r == ',' || r == '，' || r == '、' || r == ' ' || r == ';' || r == '；'
		})
		if len(rawKws) > 0 {
			matched := ""
			fullTarget := prof.Major + " " + prof.Degree + " " + prof.School + " " + resumeText
			for _, kw := range rawKws {
				kw = strings.TrimSpace(kw)
				if kw != "" && strings.Contains(fullTarget, kw) {
					matched = kw
					break
				}
			}
			if matched != "" {
				checks = append(checks, RuleCheckItem{
					Rule:     "MAJOR_KEYWORDS",
					Label:    "专业背景匹配",
					Status:   "pass",
					Evidence: fmt.Sprintf("命中指定专业关键词【%s】", matched),
				})
			} else if strings.TrimSpace(prof.Major) == "" {
				checks = append(checks, RuleCheckItem{
					Rule:     "MAJOR_KEYWORDS",
					Label:    "专业背景匹配",
					Status:   "unknown",
					Evidence: "候选人专业信息未明确填写",
				})
			} else {
				checks = append(checks, RuleCheckItem{
					Rule:     "MAJOR_KEYWORDS",
					Label:    "专业背景匹配",
					Status:   "fail",
					Evidence: fmt.Sprintf("专业背景【%s】未包含任一指定关键词【%s】", prof.Major, criteria.MajorKeywords),
				})
			}
		}
	}

	// 8. 目标城市工作意愿
	if criteria.RejectNonLocalWillingness && strings.TrimSpace(criteria.TargetCity) != "" {
		targetCity := strings.TrimSpace(criteria.TargetCity)
		if len(prof.IntentCities) == 0 {
			checks = append(checks, RuleCheckItem{
				Rule:     "TARGET_CITY",
				Label:    "工作地点意向",
				Status:   "unknown",
				Evidence: "期望工作地点未列出，待核实",
			})
		} else {
			hasTarget := false
			for _, city := range prof.IntentCities {
				if strings.Contains(city, targetCity) || strings.Contains(targetCity, city) {
					hasTarget = true
					break
				}
			}
			if !hasTarget {
				checks = append(checks, RuleCheckItem{
					Rule:     "TARGET_CITY",
					Label:    "工作地点意向",
					Status:   "fail",
					Evidence: fmt.Sprintf("期望工作城市仅包含【%s】，未包含目标城市【%s】", strings.Join(prof.IntentCities, "、"), targetCity),
				})
			} else {
				checks = append(checks, RuleCheckItem{
					Rule:     "TARGET_CITY",
					Label:    "工作地点意向",
					Status:   "pass",
					Evidence: fmt.Sprintf("包含目标城市【%s】工作意向", targetCity),
				})
			}
		}
	}

	// 9. 活跃度天数
	if criteria.ActivityDays > 0 {
		days, known := parseCandidateActivityDays(resumeText)
		if !known {
			checks = append(checks, RuleCheckItem{
				Rule:     "ACTIVITY_DAYS",
				Label:    "人才活跃度",
				Status:   "unknown",
				Evidence: "平台活跃/刷新时间未标记或无法核验",
			})
		} else if days > criteria.ActivityDays {
			checks = append(checks, RuleCheckItem{
				Rule:     "ACTIVITY_DAYS",
				Label:    "人才活跃度",
				Status:   "fail",
				Evidence: fmt.Sprintf("活跃距今约【%d天】，超出要求的【近%d天内】", days, criteria.ActivityDays),
			})
		} else {
			checks = append(checks, RuleCheckItem{
				Rule:     "ACTIVITY_DAYS",
				Label:    "人才活跃度",
				Status:   "pass",
				Evidence: fmt.Sprintf("活跃距今约【%d天】，符合【近%d天内】活跃要求", days, criteria.ActivityDays),
			})
		}
	}

	// 最终三态聚合判定（契约严格定义：任一明确不符使结果为不合格；否则任一未知使结果为待核实；否则合格）
	hasFail := false
	hasUnknown := false
	for _, c := range checks {
		if c.Status == "fail" {
			hasFail = true
			break
		}
		if c.Status == "unknown" {
			hasUnknown = true
		}
	}

	if hasFail {
		return "unfit", checks
	}
	if hasUnknown {
		return "pending_verification", checks
	}
	return "qualified", checks
}

var (
	projectScreeningLeaseMu sync.Mutex
	projectScreeningLeases  = make(map[string]time.Time)
)

func acquireScreeningLease(projectID string) bool {
	projectScreeningLeaseMu.Lock()
	defer projectScreeningLeaseMu.Unlock()
	expires, exists := projectScreeningLeases[projectID]
	if exists && time.Now().Before(expires) {
		return false
	}
	projectScreeningLeases[projectID] = time.Now().Add(5 * time.Minute)
	return true
}

func releaseScreeningLease(projectID string) {
	projectScreeningLeaseMu.Lock()
	defer projectScreeningLeaseMu.Unlock()
	delete(projectScreeningLeases, projectID)
}

func updateBatchCounts(db *sql.DB, batchID string) error {
	var qCount, uCount, pCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM screening_candidate_results WHERE batch_id=? AND verdict='qualified'`, batchID).Scan(&qCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM screening_candidate_results WHERE batch_id=? AND verdict='unfit'`, batchID).Scan(&uCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM screening_candidate_results WHERE batch_id=? AND verdict='pending_verification'`, batchID).Scan(&pCount)
	_, err := db.Exec(`UPDATE screening_batches SET qualified_count=?, unfit_count=?, pending_count=? WHERE id=?`, qCount, uCount, pCount, batchID)
	return err
}

// Scan51JobTalentManagementBatch 扫描 51job 待处理名单并生成预览（只读扫描，支持多页断点恢复）
func (a *App) Scan51JobTalentManagementBatch(accountScope, jobID, projectID string, maxPages int) (*ScreeningBatchPreview, error) {
	if !acquireScreeningLease(projectID) {
		return nil, fmt.Errorf("当前职位已有筛选或核验任务正在运行中，请稍候")
	}
	defer releaseScreeningLease(projectID)

	catalog, err := a.Get51JobTalentManagementCatalog()
	if err != nil {
		return nil, fmt.Errorf("读取 51job 在线职位失败: %w", err)
	}
	if catalog.AccountScope != accountScope {
		return nil, fmt.Errorf("51job 登录账号已改变（当前: %s, 期望: %s）", catalog.AccountScope, accountScope)
	}

	matchedJob := false
	for _, j := range catalog.Jobs {
		if j.JobID == jobID {
			matchedJob = true
			break
		}
	}
	if !matchedJob {
		return nil, fmt.Errorf("平台职位 ID (%s) 不属于当前已登录企业账号", jobID)
	}

	// 读取项目硬性条件快照（条件读取失败立即整批停止，绝不静默跳过）
	criteria, err := a.GetTalentManagementStrictCriteria(projectID)
	if err != nil {
		return nil, fmt.Errorf("读取项目专属硬性条件失败: %w", err)
	}
	if criteria == nil {
		return nil, fmt.Errorf("未能找到项目【%s】的硬性筛选配置", projectID)
	}
	criteriaJSON, _ := json.Marshal(criteria)

	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}

	businessDate := time.Now().Format("2006-01-02")
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	var batchID string
	var startPage int = 1
	var lastSuccessPage int = 0
	var totalPagesEst int = 1
	var initPlatCount int = 0
	var scanStatus string = "scanning"

	err = db.QueryRow(`SELECT id, next_page, last_success_page, total_pages_estimated, initial_platform_count, scan_status
		FROM screening_batches WHERE project_id=? AND platform_job_id=? AND business_date=? AND source='talent_management'
		ORDER BY started_at DESC LIMIT 1`, projectID, jobID, businessDate).
		Scan(&batchID, &startPage, &lastSuccessPage, &totalPagesEst, &initPlatCount, &scanStatus)

	if err == sql.ErrNoRows || scanStatus == "complete" {
		batchID = recruitmentID("scrbatch")
		startPage = 1
		lastSuccessPage = 0
		totalPagesEst = 1
		initPlatCount = 0
		scanStatus = "scanning"
		_, err = db.Exec(`INSERT INTO screening_batches(id, project_id, source, platform, platform_account, platform_job_id, business_date, criteria_version, criteria_snapshot, status, scan_status, current_page, next_page, last_success_page, total_pages_estimated, initial_platform_count, last_error, next_check_at, qualified_count, unfit_count, pending_count, started_at, completed_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			batchID, projectID, "talent_management", "51job", accountScope, jobID,
			businessDate, criteria.CriteriaVersion, string(criteriaJSON),
			"preview_ready", "scanning", 1, 1, 0, 1, 0, "", "", 0, 0, 0, nowStr, "")
		if err != nil {
			return nil, fmt.Errorf("创建扫描批次失败: %w", err)
		}
	} else {
		_, _ = db.Exec(`UPDATE screening_batches SET scan_status='scanning', last_error='', criteria_version=?, criteria_snapshot=? WHERE id=?`,
			criteria.CriteriaVersion, string(criteriaJSON), batchID)
	}

	if startPage < 1 {
		startPage = 1
	}

	pageLimit := 100
	if maxPages > 1 {
		pageLimit = maxPages
	}

	for page := startPage; page <= pageLimit; page++ {
		out, scanErr := runTalentManagementAdapter(90*time.Second, "--mode", "scan", "--job-id", jobID,
			"--status-index", "1", "--page-index", fmt.Sprint(page))
		if scanErr != nil {
			errStr := scanErr.Error()
			nextScanStatus := "paused"
			if strings.Contains(errStr, "登录") || strings.Contains(errStr, "账号") || strings.Contains(errStr, "9503") {
				nextScanStatus = "manual_required"
			}
			nextCheck := time.Now().Add(5 * time.Minute).Format("15:04:05")
			_, _ = db.Exec(`UPDATE screening_batches SET scan_status=?, last_error=?, next_check_at=?, current_page=? WHERE id=?`,
				nextScanStatus, errStr, nextCheck, page, batchID)
			_ = updateBatchCounts(db, batchID)
			return a.GetScreeningBatchPreview(batchID)
		}

		var pageData talentManagementPage
		if jsonErr := json.Unmarshal(out, &pageData); jsonErr != nil || pageData.Status != "ok" {
			msg := "解析平台名单失败"
			if pageData.Message != "" {
				msg = pageData.Message
			}
			nextCheck := time.Now().Add(5 * time.Minute).Format("15:04:05")
			_, _ = db.Exec(`UPDATE screening_batches SET scan_status='paused', last_error=?, next_check_at=?, current_page=? WHERE id=?`,
				msg, nextCheck, page, batchID)
			_ = updateBatchCounts(db, batchID)
			return a.GetScreeningBatchPreview(batchID)
		}

		pageSize := 50
		if pageData.Total > 0 {
			initPlatCount = pageData.Total
			totalPagesEst = (pageData.Total + pageSize - 1) / pageSize
			if totalPagesEst < 1 {
				totalPagesEst = 1
			}
		}

		tx, txErr := db.Begin()
		if txErr != nil {
			return nil, txErr
		}

		for _, cand := range pageData.Candidates {
			if cand.CandidateID == "" || cand.RecordID == "" {
				continue
			}

			var existingActionStatus string
			_ = tx.QueryRow(`SELECT status FROM platform_action_ledger WHERE project_id=? AND candidate_id=? AND record_id=? ORDER BY requested_at DESC LIMIT 1`,
				projectID, cand.CandidateID, cand.RecordID).Scan(&existingActionStatus)

			actionStatus := "pending"
			if existingActionStatus == "success" {
				actionStatus = "verified_on_platform"
			}

			resumeContent := cand.ResumeText
			detailLoaded := false

			verdict, checks := EvaluateCandidateTriState(resumeContent, *criteria)

			if verdict == "pending_verification" {
				detailOut, detailErr := runTalentManagementAdapter(45*time.Second, "--mode", "detail", "--job-id", jobID,
					"--candidate-id", cand.CandidateID, "--record-id", cand.RecordID, "--status-index", "1", "--page-index", fmt.Sprint(page))
				if detailErr == nil {
					var detailResp talentManagementDetail
					if json.Unmarshal(detailOut, &detailResp) == nil && detailResp.Status == "ok" && strings.TrimSpace(detailResp.ResumeText) != "" {
						resumeContent = detailResp.ResumeText
						detailLoaded = true
						verdict, checks = EvaluateCandidateTriState(resumeContent, *criteria)
					}
				}
			}

			resID := recruitmentID("scr")
			evidenceJSON, _ := json.Marshal(checks)

			_, _ = tx.Exec(`INSERT INTO screening_candidate_results(id, batch_id, project_id, candidate_id, record_id, candidate_name, verdict, rule_evidence_json, raw_profile_json, detail_loaded, action_status, action_receipt_at, action_error, created_at)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
				ON CONFLICT(batch_id, candidate_id, record_id) DO UPDATE SET verdict=excluded.verdict, rule_evidence_json=excluded.rule_evidence_json, raw_profile_json=excluded.raw_profile_json, detail_loaded=excluded.detail_loaded, action_status=excluded.action_status`,
				resID, batchID, projectID, cand.CandidateID, cand.RecordID, cand.Name, verdict, string(evidenceJSON),
				resumeContent, boolToInt(detailLoaded), actionStatus, "", "", nowStr)
		}

		lastSuccessPage = page
		nextPage := page + 1
		isComplete := !pageData.HasMore || page >= totalPagesEst

		finalScanStatus := "scanning"
		completedAtStr := ""
		if isComplete {
			finalScanStatus = "complete"
			completedAtStr = time.Now().Format("2006-01-02 15:04:05")
		}

		_, _ = tx.Exec(`UPDATE screening_batches SET current_page=?, next_page=?, last_success_page=?, total_pages_estimated=?, initial_platform_count=?, scan_status=?, completed_at=?, last_error='' WHERE id=?`,
			page, nextPage, lastSuccessPage, totalPagesEst, initPlatCount, finalScanStatus, completedAtStr, batchID)

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("保存第 %d 页检查点失败: %w", page, err)
		}

		if isComplete {
			break
		}
	}

	_ = updateBatchCounts(db, batchID)
	return a.GetScreeningBatchPreview(batchID)
}

// Persist the parent and every result atomically; foreign keys reject children
// inserted before screening_batches.
func persistScreeningBatch(db *sql.DB, preview *ScreeningBatchPreview, criteriaJSON, nowStr string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO screening_batches(id, project_id, source, platform, platform_account, platform_job_id, business_date, criteria_version, criteria_snapshot, status, qualified_count, unfit_count, pending_count, started_at, completed_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		preview.BatchID, preview.ProjectID, preview.Source, preview.Platform, preview.PlatformAccount,
		preview.PlatformJobID, preview.BusinessDate, preview.CriteriaVersion, criteriaJSON,
		preview.Status, preview.QualifiedCount, preview.UnfitCount, preview.PendingCount, nowStr, nowStr)
	if err != nil {
		return err
	}
	for _, list := range [][]ScreeningCandidateResult{preview.QualifiedList, preview.UnfitList, preview.PendingList} {
		for _, item := range list {
			evidenceJSON, err := json.Marshal(item.RuleEvidence)
			if err != nil {
				return err
			}
			_, err = tx.Exec(`INSERT INTO screening_candidate_results(id, batch_id, project_id, candidate_id, record_id, candidate_name, verdict, rule_evidence_json, raw_profile_json, detail_loaded, action_status, created_at)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, preview.BatchID, preview.ProjectID,
				item.CandidateID, item.RecordID, item.CandidateName, item.Verdict, string(evidenceJSON),
				item.RawProfile, boolToInt(item.DetailLoaded), item.ActionStatus, item.CreatedAt)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// ConfirmScreeningImportQualified HR 确认入库合格人选：同步入库至职位专属分库并记录台账
func (a *App) ConfirmScreeningImportQualified(batchID string) (int, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return 0, err
	}
	var batch ScreeningBatchPreview
	err = db.QueryRow(`SELECT id, project_id, platform, platform_account, platform_job_id, business_date, criteria_version FROM screening_batches WHERE id=?`, batchID).
		Scan(&batch.BatchID, &batch.ProjectID, &batch.Platform, &batch.PlatformAccount, &batch.PlatformJobID, &batch.BusinessDate, &batch.CriteriaVersion)
	if err != nil {
		return 0, fmt.Errorf("未找到指定批次: %w", err)
	}

	rows, err := db.Query(`SELECT id, candidate_id, record_id, candidate_name, raw_profile_json FROM screening_candidate_results WHERE batch_id=? AND verdict='qualified' AND action_status != 'imported'`, batchID)
	if err != nil {
		return 0, err
	}
	type qualifiedItem struct {
		scrID   string
		candID  string
		recID   string
		name    string
		rawText string
	}
	var items []qualifiedItem
	for rows.Next() {
		var it qualifiedItem
		if err := rows.Scan(&it.scrID, &it.candID, &it.recID, &it.name, &it.rawText); err == nil {
			items = append(items, it)
		}
	}
	rows.Close()

	importedCount := 0
	nowISO := time.Now().UTC().Format(time.RFC3339Nano)
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	for _, it := range items {
		// 登记到 applications 正常入库流程
		appID := fmt.Sprintf("51job_%s_%s", batch.PlatformAccount, it.recID)
		completed := true
		record := ApplicationRecord{
			Platform:            "51job",
			PlatformAccount:     batch.PlatformAccount,
			ApplicationID:       it.recID,
			PlatformCandidateID: it.candID,
			PlatformJobID:       batch.PlatformJobID,
			CandidateName:       it.name,
			PlatformStatus:      "待处理",
			DocumentsComplete:   &completed,
			DocumentStatus:      "三态合规核验合格入库",
			ResumeText:          it.rawText,
		}
		appRecordJSON, _ := json.Marshal(record)

		_, _ = db.Exec(`INSERT INTO applications (id, platform, platform_account, platform_application_id, platform_candidate_id, platform_job_id, project_id, status, applied_at, raw_json, created_at, updated_at)
			VALUES (?, '51job', ?, ?, ?, ?, ?, 'imported', ?, ?, ?, ?)
			ON CONFLICT(platform, platform_account, platform_application_id) DO UPDATE SET status='imported', updated_at=?`,
			appID, batch.PlatformAccount, it.recID, it.candID, batch.PlatformJobID, batch.ProjectID, nowISO, string(appRecordJSON), nowISO, nowISO, nowISO)

		// 登记或更新到 project_candidates
		candUUID := recruitmentID("cand")
		pcID := recruitmentID("pc")
		_, _ = db.Exec(`INSERT OR IGNORE INTO candidates (id, display_name, created_at, updated_at) VALUES (?, ?, ?, ?)`, candUUID, it.name, nowISO, nowISO)
		_ = db.QueryRow(`SELECT id FROM candidates WHERE display_name=? LIMIT 1`, it.name).Scan(&candUUID)

		_, _ = db.Exec(`INSERT INTO project_candidates (id, project_id, candidate_id, stage, created_at, updated_at)
			VALUES (?, ?, ?, 'new', ?, ?)
			ON CONFLICT(project_id, candidate_id) DO UPDATE SET stage='new', archived_at='', updated_at=?`,
			pcID, batch.ProjectID, candUUID, nowISO, nowISO, nowISO)

		// 记录动作台账
		ledgerID := recruitmentID("pal")
		_, _ = db.Exec(`INSERT INTO platform_action_ledger(id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, action_type, confirmed_by, requested_at, receipt, status, created_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(project_id, candidate_id, record_id, action_type) DO UPDATE SET status='success', receipt=excluded.receipt, requested_at=excluded.requested_at`,
			ledgerID, batch.ProjectID, "51job", batch.PlatformAccount, batch.PlatformJobID, it.candID, it.recID, it.name, "import", "HR", nowStr, "已入库至本地专属分库", "success", nowStr)

		_, _ = db.Exec(`UPDATE screening_candidate_results SET action_status='imported' WHERE id=?`, it.scrID)
		importedCount++
	}

	return importedCount, nil
}

// ConfirmScreeningRejectUnfit HR 确认对不合格人选批量执行平台标记不合适（逐人核验、原子落库、即时反馈）
func (a *App) ConfirmScreeningRejectUnfit(batchID string, candidateIDs []string) ([]PlatformActionLedgerItem, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var batch ScreeningBatchPreview
	err = db.QueryRow(`SELECT id, project_id, platform, platform_account, platform_job_id, business_date, criteria_version FROM screening_batches WHERE id=?`, batchID).
		Scan(&batch.BatchID, &batch.ProjectID, &batch.Platform, &batch.PlatformAccount, &batch.PlatformJobID, &batch.BusinessDate, &batch.CriteriaVersion)
	if err != nil {
		return nil, fmt.Errorf("未找到指定批次: %w", err)
	}

	criteria, _ := a.GetTalentManagementStrictCriteria(batch.ProjectID)
	if criteria == nil || !criteria.ComplianceConfirmed {
		return nil, fmt.Errorf("执行平台不感兴趣/不合适前，必须先在岗位硬性条件中勾选【HR 合规授权确认】")
	}

	results := make([]PlatformActionLedgerItem, 0)
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	for _, candID := range candidateIDs {
		var scrID, recID, name, evidenceJSON string
		err := db.QueryRow(`SELECT id, record_id, candidate_name, rule_evidence_json FROM screening_candidate_results WHERE batch_id=? AND candidate_id=?`, batchID, candID).
			Scan(&scrID, &recID, &name, &evidenceJSON)
		if err != nil {
			continue
		}

		// 幂等性防护：若该候选人针对同一动作已存在成功记录，直接返回已有回执并对齐状态
		var existingLedger PlatformActionLedgerItem
		err = db.QueryRow(`SELECT id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, action_type, confirmed_by, requested_at, receipt, status, error_message, retry_count
			FROM platform_action_ledger WHERE project_id=? AND candidate_id=? AND record_id=? AND action_type='reject'`,
			batch.ProjectID, candID, recID).Scan(&existingLedger.ID, &existingLedger.ProjectID, &existingLedger.Platform, &existingLedger.PlatformAccount,
			&existingLedger.PlatformJobID, &existingLedger.CandidateID, &existingLedger.RecordID, &existingLedger.CandidateName,
			&existingLedger.ActionType, &existingLedger.ConfirmedBy, &existingLedger.RequestedAt, &existingLedger.Receipt, &existingLedger.Status, &existingLedger.ErrorMessage, &existingLedger.RetryCount)
		if err == nil && existingLedger.Status == "success" {
			_, _ = db.Exec(`UPDATE screening_candidate_results SET action_status='verified_on_platform', action_receipt_at=?, action_error='' WHERE id=?`, existingLedger.RequestedAt, scrID)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "screening_candidate_updated", map[string]interface{}{
					"batchId": batchID, "candidateId": candID, "recordId": recID, "actionStatus": "verified_on_platform", "receipt": existingLedger.Receipt,
				})
			}
			results = append(results, existingLedger)
			continue
		}

		// 解析不合格的首要原因用于平台理由映射
		var checks []RuleCheckItem
		_ = json.Unmarshal([]byte(evidenceJSON), &checks)
		platformReason := "其他原因"
		firstEvidence := ""
		for _, c := range checks {
			if c.Status == "fail" {
				firstEvidence = c.Evidence
				switch c.Rule {
				case "MIN_EDUCATION":
					platformReason = "学历不匹配"
				case "MIN_WORK_YEARS":
					platformReason = "工作年限不匹配"
				case "TARGET_CITY":
					platformReason = "工作地点不匹配"
				}
				break
			}
		}

		// 联动平台适配器执行操作与双重核验
		rejectOut, rejectErr := runTalentManagementAdapter(45*time.Second, "--mode", "reject", "--job-id", batch.PlatformJobID,
			"--candidate-id", candID, "--record-id", recID, "--reason-label", platformReason)

		receipt := ""
		ledgerStatus := "success"
		actionStatus := "completed"
		errMsg := ""

		if rejectErr != nil {
			errMsg = rejectErr.Error()
			ledgerStatus = "failed"
			actionStatus = "failed"
			if strings.Contains(errMsg, "登录") || strings.Contains(errMsg, "账号") || strings.Contains(errMsg, "9503") {
				actionStatus = "manual_required"
			}
			receipt = "平台标记不合适失败: " + errMsg
		} else {
			var resp struct {
				Status           string `json:"status"`
				Verified         bool   `json:"verified"`
				AlreadyProcessed bool   `json:"alreadyProcessed"`
				Receipt          string `json:"receipt"`
			}
			_ = json.Unmarshal(rejectOut, &resp)
			receipt = resp.Receipt
			if resp.AlreadyProcessed {
				actionStatus = "verified_on_platform"
				ledgerStatus = "success"
				if receipt == "" {
					receipt = "经核验平台已处于不合适状态，已逐人核对确认（保留来源，不重复点击）"
				}
			} else if resp.Status == "receipt_pending" || !resp.Verified {
				actionStatus = "receipt_pending"
				ledgerStatus = "pending_retry"
				if receipt == "" {
					receipt = "已在平台提交不合适操作，但待处理列表尚未移除该卡片，待核实回执"
				}
			} else {
				actionStatus = "completed"
				ledgerStatus = "success"
				if receipt == "" {
					receipt = fmt.Sprintf("已在 51job 平台成功标记不合适并经回执核验（理由：%s）", platformReason)
				}
			}
		}

		ledgerID := recruitmentID("pal")
		ledgerItem := PlatformActionLedgerItem{
			ID:              ledgerID,
			ProjectID:       batch.ProjectID,
			Platform:        "51job",
			PlatformAccount: batch.PlatformAccount,
			PlatformJobID:   batch.PlatformJobID,
			CandidateID:     candID,
			RecordID:        recID,
			CandidateName:   name,
			ActionType:      "reject",
			ConfirmedBy:     "HR",
			RequestedAt:     nowStr,
			Receipt:         receipt,
			Status:          ledgerStatus,
			ErrorMessage:    errMsg,
			RetryCount:      0,
		}

		// 单人独立事务：保证每人操作立即落库，一人失败绝不影响他人
		tx, txErr := db.Begin()
		if txErr == nil {
			_, _ = tx.Exec(`INSERT INTO platform_action_ledger(id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, action_type, confirmed_by, requested_at, receipt, status, error_message, retry_count)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
				ON CONFLICT(project_id, candidate_id, record_id, action_type) DO UPDATE SET status=excluded.status, receipt=excluded.receipt, error_message=excluded.error_message, retry_count=retry_count+1, requested_at=excluded.requested_at`,
				ledgerID, batch.ProjectID, "51job", batch.PlatformAccount, batch.PlatformJobID, candID, recID, name, "reject", "HR", nowStr, receipt, ledgerStatus, errMsg, 0)

			auditID := recruitmentID("tmaudit")
			_, _ = tx.Exec(`INSERT INTO talent_management_audit_logs
				(id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, rule_code, reason_label, evidence, platform_receipt, action_status, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				auditID, batch.ProjectID, "51job", batch.PlatformAccount, batch.PlatformJobID, candID, recID, name, platformReason, platformReason, firstEvidence, receipt, ledgerStatus, nowStr)

			_, _ = tx.Exec(`UPDATE screening_candidate_results SET action_status=?, action_receipt_at=?, action_error=? WHERE id=?`, actionStatus, nowStr, errMsg, scrID)

			if actionStatus == "completed" || actionStatus == "verified_on_platform" {
				_, _ = tx.Exec(`UPDATE applications SET status='unfit_archived' WHERE platform='51job' AND platform_account=? AND platform_application_id=?`, batch.PlatformAccount, recID)
			}
			_ = tx.Commit()
		}

		// 实时向前端广播行级更新
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "screening_candidate_updated", map[string]interface{}{
				"batchId": batchID, "candidateId": candID, "recordId": recID, "actionStatus": actionStatus, "actionReceiptAt": nowStr, "actionError": errMsg, "receipt": receipt,
			})
		}

		results = append(results, ledgerItem)
	}

	return results, nil
}

// RescreenProjectRepository 提供一次性旧库重筛：排除平台已推进的人；旧库不合格者移到待处理区，保留恢复和审计所需数据
func (a *App) RescreenProjectRepository(projectID string) (*OneClickFilterResult, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	criteria, err := a.GetTalentManagementStrictCriteria(projectID)
	if err != nil {
		return nil, fmt.Errorf("读取岗位硬性条件失败: %w", err)
	}

	rows, err := db.Query(`SELECT pc.id, COALESCE(c.display_name, ''), COALESCE(rv.content, ''), COALESCE(rv.raw_json, ''), pc.stage
		FROM project_candidates pc
		LEFT JOIN candidates c ON c.id = pc.candidate_id
		LEFT JOIN project_candidate_resumes pcr ON pcr.project_candidate_id = pc.id
		LEFT JOIN resume_versions rv ON rv.id = pcr.resume_version_id
		WHERE pc.project_id = ? AND pc.archived_at = ''`, projectID)
	if err != nil {
		return nil, err
	}
	type rescreenItem struct {
		pcID    string
		name    string
		content string
		rawJSON string
		stage   string
	}
	var candList []rescreenItem
	for rows.Next() {
		var it rescreenItem
		if err := rows.Scan(&it.pcID, &it.name, &it.content, &it.rawJSON, &it.stage); err == nil {
			candList = append(candList, it)
		}
	}
	rows.Close()

	res := &OneClickFilterResult{
		Details: make([]string, 0),
	}
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	for _, cand := range candList {
		res.TotalChecked++

		// 规范：排除平台已推进（合适、面试、待定）的人选
		if cand.stage == "interview" || cand.stage == "offer" || cand.stage == "hired" {
			continue
		}

		resumeContent := cand.content
		if strings.TrimSpace(cand.rawJSON) != "" {
			resumeContent = cand.rawJSON
		}

		verdict, checks := EvaluateCandidateTriState(resumeContent, *criteria)
		if verdict == "unfit" {
			res.UnfitCount++
			firstEvidence := ""
			firstRule := "UNFIT"
			for _, c := range checks {
				if c.Status == "fail" {
					firstEvidence = c.Evidence
					firstRule = c.Rule
					break
				}
			}

			// 移入归档区（保留完整数据用于自愈与审计，任何平台动作必须由 HR 手动确认）
			_, _ = db.Exec(`UPDATE project_candidates SET archived_at=?, stage='unfit', updated_at=? WHERE id=?`, nowStr, nowStr, cand.pcID)

			// 记录审计日志
			auditID := recruitmentID("tmaudit")
			_, _ = db.Exec(`INSERT INTO talent_management_audit_logs
				(id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, rule_code, reason_label, evidence, platform_receipt, action_status, created_at)
				VALUES (?, ?, 'local', '', '', ?, ?, ?, ?, ?, ?, '旧库重筛识别不合规已移入归档待处理区', 'archived', ?)`,
				auditID, projectID, cand.pcID, cand.pcID, cand.name, firstRule, "岗位硬性条件不符", firstEvidence, nowStr)

			res.Details = append(res.Details, fmt.Sprintf("%s：%s", cand.name, firstEvidence))
		}
	}

	res.ActiveRemainingCount = res.TotalChecked - res.UnfitCount
	return res, nil
}

// ListPlatformActionLedger 查询项目平台动作台账
func (a *App) ListPlatformActionLedger(projectID string) ([]PlatformActionLedgerItem, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, action_type, confirmed_by, requested_at, receipt, status, error_message, retry_count
		FROM platform_action_ledger WHERE project_id=? ORDER BY requested_at DESC LIMIT 150`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PlatformActionLedgerItem, 0)
	for rows.Next() {
		var it PlatformActionLedgerItem
		if err := rows.Scan(&it.ID, &it.ProjectID, &it.Platform, &it.PlatformAccount, &it.PlatformJobID,
			&it.CandidateID, &it.RecordID, &it.CandidateName, &it.ActionType, &it.ConfirmedBy,
			&it.RequestedAt, &it.Receipt, &it.Status, &it.ErrorMessage, &it.RetryCount); err == nil {
			items = append(items, it)
		}
	}
	return items, nil
}

// GetLatestProjectScreeningBatch 获取某项目的最新一次扫描批次及三类预览
func (a *App) GetLatestProjectScreeningBatch(projectID string) (*ScreeningBatchPreview, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var batchID string
	err = db.QueryRow(`SELECT id FROM screening_batches WHERE project_id=? ORDER BY started_at DESC LIMIT 1`, projectID).Scan(&batchID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a.GetScreeningBatchPreview(batchID)
}

// GetScreeningBatchPreview 根据批次 ID 检索完整的三类名单及明细（含状态对账与检查点信息）
func (a *App) GetScreeningBatchPreview(batchID string) (*ScreeningBatchPreview, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var preview ScreeningBatchPreview
	err = db.QueryRow(`SELECT id, project_id, source, platform, platform_account, platform_job_id, business_date, criteria_version, status,
		COALESCE(scan_status, 'complete'), COALESCE(current_page, 1), COALESCE(next_page, 1), COALESCE(last_success_page, 0),
		COALESCE(total_pages_estimated, 1), COALESCE(initial_platform_count, 0), COALESCE(last_error, ''), COALESCE(next_check_at, ''),
		qualified_count, unfit_count, pending_count
		FROM screening_batches WHERE id=?`, batchID).Scan(
		&preview.BatchID, &preview.ProjectID, &preview.Source, &preview.Platform,
		&preview.PlatformAccount, &preview.PlatformJobID, &preview.BusinessDate,
		&preview.CriteriaVersion, &preview.Status,
		&preview.ScanStatus, &preview.CurrentPage, &preview.NextPage, &preview.LastSuccessPage,
		&preview.TotalPagesEstimated, &preview.InitialPlatformCount, &preview.LastError, &preview.NextCheckAt,
		&preview.QualifiedCount, &preview.UnfitCount, &preview.PendingCount)
	if err != nil {
		return nil, err
	}
	preview.QualifiedList = make([]ScreeningCandidateResult, 0)
	preview.UnfitList = make([]ScreeningCandidateResult, 0)
	preview.PendingList = make([]ScreeningCandidateResult, 0)

	rows, err := db.Query(`SELECT id, batch_id, project_id, candidate_id, record_id, candidate_name, verdict, rule_evidence_json, raw_profile_json, detail_loaded, action_status, COALESCE(action_receipt_at, ''), COALESCE(action_error, ''), created_at
		FROM screening_candidate_results WHERE batch_id=?`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var it ScreeningCandidateResult
		var evidenceJSON string
		var detailLoaded int
		if err := rows.Scan(&it.ID, &it.BatchID, &it.ProjectID, &it.CandidateID, &it.RecordID, &it.CandidateName,
			&it.Verdict, &evidenceJSON, &it.RawProfile, &detailLoaded, &it.ActionStatus, &it.ActionReceiptAt, &it.ActionError, &it.CreatedAt); err == nil {
			it.DetailLoaded = detailLoaded == 1
			_ = json.Unmarshal([]byte(evidenceJSON), &it.RuleEvidence)

			// 状态对账：核验已存在的平台动作台账，对齐未反映的状态
			var ledgerStatus, ledgerReceipt string
			errL := db.QueryRow(`SELECT status, receipt FROM platform_action_ledger WHERE project_id=? AND candidate_id=? AND record_id=? ORDER BY requested_at DESC LIMIT 1`,
				it.ProjectID, it.CandidateID, it.RecordID).Scan(&ledgerStatus, &ledgerReceipt)
			if errL == nil {
				if ledgerStatus == "success" {
					if it.ActionStatus != "completed" && it.ActionStatus != "verified_on_platform" {
						it.ActionStatus = "verified_on_platform"
						it.ActionReceiptAt = ledgerReceipt
					}
				} else if ledgerStatus == "pending_retry" && it.ActionStatus == "pending" {
					it.ActionStatus = "receipt_pending"
					it.ActionReceiptAt = ledgerReceipt
				}
			}

			switch it.Verdict {
			case "qualified":
				preview.QualifiedList = append(preview.QualifiedList, it)
			case "unfit":
				preview.UnfitList = append(preview.UnfitList, it)
			default:
				preview.PendingList = append(preview.PendingList, it)
			}
		}
	}

	preview.QualifiedCount = len(preview.QualifiedList)
	preview.UnfitCount = len(preview.UnfitList)
	preview.PendingCount = len(preview.PendingList)
	preview.TotalCount = preview.QualifiedCount + preview.UnfitCount + preview.PendingCount
	return &preview, nil
}

// CheckAndResumeProjectScreening HR 手动点击或定时调度执行：检查并恢复未完成的筛选与动作核验
func (a *App) CheckAndResumeProjectScreening(projectID string) (*ScreeningBatchPreview, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var batchID, accountScope, jobID, scanStatus string
	err = db.QueryRow(`SELECT id, platform_account, platform_job_id, COALESCE(scan_status, 'complete')
		FROM screening_batches WHERE project_id=? AND source='talent_management'
		ORDER BY started_at DESC LIMIT 1`, projectID).Scan(&batchID, &accountScope, &jobID, &scanStatus)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("当前项目暂无筛选任务记录")
	}
	if err != nil {
		return nil, err
	}

	// 若扫描未完成，尝试断点续跑
	if scanStatus == "scanning" || scanStatus == "paused" || scanStatus == "manual_required" {
		return a.Scan51JobTalentManagementBatch(accountScope, jobID, projectID, 0)
	}

	// 否则刷新并返回当前批次预览（自动对账）
	return a.GetScreeningBatchPreview(batchID)
}

// resumeAllUnfinishedScreeningTasks 检查所有未完成的筛选任务并尝试续跑
func (a *App) resumeAllUnfinishedScreeningTasks() {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return
	}
	rows, err := db.Query(`SELECT id, project_id, platform_account, platform_job_id, scan_status
		FROM screening_batches WHERE scan_status IN ('scanning', 'paused') ORDER BY started_at DESC LIMIT 5`)
	if err != nil {
		return
	}
	type taskItem struct {
		batchID      string
		projectID    string
		accountScope string
		jobID        string
		scanStatus   string
	}
	var tasks []taskItem
	for rows.Next() {
		var t taskItem
		if err := rows.Scan(&t.batchID, &t.projectID, &t.accountScope, &t.jobID, &t.scanStatus); err == nil {
			tasks = append(tasks, t)
		}
	}
	rows.Close()

	if len(tasks) == 0 {
		return
	}

	// 检查 51job 专用浏览器是否在线
	catalog, err := a.Get51JobTalentManagementCatalog()
	if err != nil {
		for _, t := range tasks {
			_, _ = db.Exec(`UPDATE screening_batches SET scan_status='manual_required', last_error='51job 专属浏览器未连接，保留断点' WHERE id=?`, t.batchID)
		}
		return
	}

	for _, t := range tasks {
		if t.accountScope == catalog.AccountScope {
			_, _ = a.Scan51JobTalentManagementBatch(t.accountScope, t.jobID, t.projectID, 0)
		}
	}
}

// startScreeningFrameworkScheduler 启动后台 5 分钟定时任务与重启恢复调度器
func (a *App) startScreeningFrameworkScheduler() {
	go func() {
		time.Sleep(3 * time.Second)
		a.resumeAllUnfinishedScreeningTasks()

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			a.resumeAllUnfinishedScreeningTasks()
		}
	}()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
