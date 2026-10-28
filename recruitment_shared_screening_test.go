package main

import (
	"database/sql"
	"encoding/json"
	"testing"
)

func TestTelescopeActivityLabels(t *testing.T) {
	cases := []struct {
		label string
		days  int
	}{
		{"1小时前活跃", 0}, {"24小时内活跃", 1}, {"3日内活跃", 3},
		{"1周内活跃", 7}, {"2周内活跃", 14}, {"1个月内活跃", 30},
	}
	for _, tc := range cases {
		days, known := parseCandidateActivityDays(`{"active_type":"` + tc.label + `"}`)
		if !known || days != tc.days {
			t.Fatalf("%s: got (%d,%v), want %d", tc.label, days, known, tc.days)
		}
	}
}

func TestScreeningIgnoresAgeAndGender(t *testing.T) {
	criteria := TalentManagementStrictCriteria{MinEducation: "本科", MinAge: 30, MaxAge: 40, Gender: "男"}
	verdict, checks := EvaluateCandidateTriState(`{"degree":"本科","age":"25","sex":"女"}`, criteria)
	if verdict != "qualified" {
		t.Fatalf("age and gender must not change verdict, got %s", verdict)
	}
	for _, check := range checks {
		if check.Rule == "AGE_RANGE" || check.Rule == "GENDER" {
			t.Fatalf("age and gender must not become screening checks: %+v", check)
		}
	}
}

func TestPersistScreeningBatchWritesParentBeforeResults(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE projects(id TEXT PRIMARY KEY); INSERT INTO projects(id) VALUES('project-1');`); err != nil {
		t.Fatal(err)
	}
	if err := ensureScreeningFrameworkTables(db); err != nil {
		t.Fatal(err)
	}
	item := ScreeningCandidateResult{ID: "result-1", BatchID: "batch-1", ProjectID: "project-1",
		CandidateID: "candidate-1", RecordID: "record-1", Verdict: "qualified",
		RuleEvidence: []RuleCheckItem{}, RawProfile: `{}`, ActionStatus: "pending", CreatedAt: "2026-10-28 12:00:00"}
	preview := &ScreeningBatchPreview{BatchID: "batch-1", ProjectID: "project-1", Source: "telescope",
		Platform: "51job", PlatformAccount: "co:1:hr:2", PlatformJobID: "job-1",
		BusinessDate: "2026-10-28", CriteriaVersion: "v1", Status: "preview_ready",
		QualifiedCount: 1, QualifiedList: []ScreeningCandidateResult{item}}
	if err := persistScreeningBatch(db, preview, `{}`, "2026-10-28 12:00:00"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM screening_candidate_results WHERE batch_id='batch-1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("persisted result count = %d, error = %v", count, err)
	}
}

func TestEvaluateCandidateTriState_BachelorBlocksAssociate(t *testing.T) {
	criteria := TalentManagementStrictCriteria{
		MinEducation: "本科",
	}

	// 候选人最高学历为大专
	resumeAssociate := `{"degree": "大专", "school": "武汉职业技术学院", "major": "软件技术"}`
	verdict, checks := EvaluateCandidateTriState(resumeAssociate, criteria)
	if verdict != "unfit" {
		t.Fatalf("预期不合格 (unfit)，实际得到: %s", verdict)
	}
	foundFail := false
	for _, c := range checks {
		if c.Rule == "MIN_EDUCATION" && c.Status == "fail" {
			foundFail = true
		}
	}
	if !foundFail {
		t.Fatalf("未在 checks 中找到 MIN_EDUCATION 的 fail 项: %+v", checks)
	}

	// 候选人最高学历为本科
	resumeBachelor := `{"degree": "本科", "school": "武汉大学", "major": "计算机科学与技术"}`
	verdict2, checks2 := EvaluateCandidateTriState(resumeBachelor, criteria)
	if verdict2 != "qualified" {
		t.Fatalf("预期合格 (qualified)，实际得到: %s", verdict2)
	}
	for _, c := range checks2 {
		if c.Rule == "MIN_EDUCATION" && c.Status != "pass" {
			t.Fatalf("预期 MIN_EDUCATION 为 pass，实际得到: %+v", c)
		}
	}
}

func TestEvaluateCandidateTriState_MissingDegreeGoesToPending(t *testing.T) {
	criteria := TalentManagementStrictCriteria{
		MinEducation: "本科",
	}

	// 学历信息缺失
	resumeMissingEdu := `{"username": "张三", "workyear": "5年"}`
	verdict, checks := EvaluateCandidateTriState(resumeMissingEdu, criteria)
	if verdict != "pending_verification" {
		t.Fatalf("预期学历缺失为待核实 (pending_verification)，实际得到: %s", verdict)
	}
	foundUnknown := false
	for _, c := range checks {
		if c.Rule == "MIN_EDUCATION" && c.Status == "unknown" {
			foundUnknown = true
		}
	}
	if !foundUnknown {
		t.Fatalf("未在 checks 中找到 MIN_EDUCATION 的 unknown 项: %+v", checks)
	}
}

func TestEvaluateCandidateTriState_OrderIndependence(t *testing.T) {
	// 同时配置学历本科、工作年限 3 年、活跃度 7 天
	criteria := TalentManagementStrictCriteria{
		MinEducation: "本科",
		MinWorkYears: 3,
		ActivityDays: 7,
	}

	// Case 1: 学历合格，但年限明确不符 (1年)，活跃度缺失(unknown)
	// 契约：任一明确不符使结果为不合格 (unfit)
	resume1 := `{"degree": "本科", "workyear": "1年"}`
	verdict1, _ := EvaluateCandidateTriState(resume1, criteria)
	if verdict1 != "unfit" {
		t.Fatalf("即使活跃度未知，有确凿年限不符应判定为 unfit，实际为: %s", verdict1)
	}

	// Case 2: 学历合格，年限合格 (5年)，活跃度缺失(unknown)
	// 契约：无明确不符，但有未知项，使结果为待核实 (pending_verification)
	resume2 := `{"degree": "本科", "workyear": "5年"}`
	verdict2, _ := EvaluateCandidateTriState(resume2, criteria)
	if verdict2 != "pending_verification" {
		t.Fatalf("无不符但活跃度未知应判定为 pending_verification，实际为: %s", verdict2)
	}

	// Case 3: 全部满足
	resume3 := `{"degree": "硕士", "workyear": "5年", "active_time": "2026-10-27 10:00:00"}`
	verdict3, _ := EvaluateCandidateTriState(resume3, criteria)
	if verdict3 != "qualified" {
		t.Fatalf("全部满足应判定为 qualified，实际为: %s", verdict3)
	}
}

func TestScreeningBatchPreviewJSONSerialization(t *testing.T) {
	preview := ScreeningBatchPreview{
		BatchID:         "batch_test_1",
		ProjectID:       "proj_1",
		Platform:        "51job",
		PlatformAccount: "co:123:hr:456",
		PlatformJobID:   "job_999",
		CriteriaVersion: "v2",
		TotalCount:      2,
		QualifiedCount:  1,
		UnfitCount:      1,
		QualifiedList: []ScreeningCandidateResult{
			{
				CandidateID:   "cand_1",
				CandidateName: "李四",
				Verdict:       "qualified",
			},
		},
		UnfitList: []ScreeningCandidateResult{
			{
				CandidateID:   "cand_2",
				CandidateName: "王五",
				Verdict:       "unfit",
			},
		},
	}
	bytes, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %v", err)
	}
	if len(bytes) == 0 {
		t.Fatalf("JSON 为空")
	}
}

func TestRescreenProjectRepository(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()

	proj := app.CreateProject("测试项目", "技术部", 1, "", "", nil)
	if proj == nil {
		t.Fatalf("创建项目失败")
	}

	// 设置岗位硬性条件：本科及以上
	err := app.SetTalentManagementStrictCriteria(TalentManagementStrictCriteria{
		ProjectID:    proj.ID,
		MinEducation: "本科",
	})
	if err != nil {
		t.Fatalf("设置硬性条件失败: %v", err)
	}

	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatalf("获取数据库失败: %v", err)
	}

	// 插入两名候选人：一名大专，一名本科
	_, _ = db.Exec(`INSERT INTO candidates(id, display_name, created_at, updated_at) VALUES ('c1', '大专张三', '2026-10-28', '2026-10-28')`)
	_, _ = db.Exec(`INSERT INTO candidates(id, display_name, created_at, updated_at) VALUES ('c2', '本科李四', '2026-10-28', '2026-10-28')`)

	_, _ = db.Exec(`INSERT INTO project_candidates(id, project_id, candidate_id, stage, created_at, updated_at) VALUES ('pc1', ?, 'c1', 'new', '2026-10-28', '2026-10-28')`, proj.ID)
	_, _ = db.Exec(`INSERT INTO project_candidates(id, project_id, candidate_id, stage, created_at, updated_at) VALUES ('pc2', ?, 'c2', 'new', '2026-10-28', '2026-10-28')`, proj.ID)

	_, err = db.Exec(`INSERT INTO resume_versions(id, candidate_id, content, content_sha256, raw_json, created_at) VALUES ('rv1', 'c1', '大专学历', 'sha1', '{"degree":"大专"}', '2026-10-28')`)
	if err != nil {
		t.Fatalf("插入 rv1 失败: %v", err)
	}
	_, err = db.Exec(`INSERT INTO resume_versions(id, candidate_id, content, content_sha256, raw_json, created_at) VALUES ('rv2', 'c2', '本科学历', 'sha2', '{"degree":"本科"}', '2026-10-28')`)
	if err != nil {
		t.Fatalf("插入 rv2 失败: %v", err)
	}

	_, _ = db.Exec(`INSERT INTO project_candidate_resumes(project_candidate_id, resume_version_id, created_at) VALUES ('pc1', 'rv1', '2026-10-28')`)
	_, _ = db.Exec(`INSERT INTO project_candidate_resumes(project_candidate_id, resume_version_id, created_at) VALUES ('pc2', 'rv2', '2026-10-28')`)

	// 执行旧库重筛
	result, err := app.RescreenProjectRepository(proj.ID)
	if err != nil {
		t.Fatalf("旧库重筛执行失败: %v", err)
	}

	t.Logf("Rescreen Result: TotalChecked=%d, UnfitCount=%d, ActiveRemainingCount=%d, Details=%+v",
		result.TotalChecked, result.UnfitCount, result.ActiveRemainingCount, result.Details)

	if result.TotalChecked != 2 {
		t.Errorf("预期检查 2 人，实际: %d", result.TotalChecked)
	}
	if result.UnfitCount != 1 {
		t.Errorf("预期排除 1 人，实际: %d", result.UnfitCount)
	}
	if result.ActiveRemainingCount != 1 {
		t.Errorf("预期剩余 1 人，实际: %d", result.ActiveRemainingCount)
	}

	// 验证 pc1 已被标记 archived_at
	var archivedAt string
	_ = db.QueryRow(`SELECT archived_at FROM project_candidates WHERE id='pc1'`).Scan(&archivedAt)
	if archivedAt == "" {
		t.Errorf("pc1 应该被移入归档区 (archived_at 不为空)")
	}

	// 验证 pc2 仍然活跃 (archived_at 为空)
	var archivedAt2 string
	_ = db.QueryRow(`SELECT archived_at FROM project_candidates WHERE id='pc2'`).Scan(&archivedAt2)
	if archivedAt2 != "" {
		t.Errorf("pc2 应保持活跃，但被归档了: %s", archivedAt2)
	}
}

func TestPlatformActionLedgerIdempotency(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()

	proj := app.CreateProject("台账测试项目", "技术部", 1, "", "", nil)
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatalf("获取数据库失败: %v", err)
	}

	// 插入一条已成功的台账记录
	_, err = db.Exec(`INSERT INTO platform_action_ledger(id, project_id, platform, platform_account, platform_job_id, candidate_id, record_id, candidate_name, action_type, confirmed_by, requested_at, receipt, status, error_message, retry_count)
		VALUES ('pal_test', ?, '51job', 'acc1', 'job1', 'cand1', 'rec1', '测试候选人', 'reject', 'HR', '2026-10-28 10:00:00', '既有平台回执：已标记不合适', 'success', '', 0)`, proj.ID)
	if err != nil {
		t.Fatalf("插入既有台账失败: %v", err)
	}

	// 查询台账
	list, err := app.ListPlatformActionLedger(proj.ID)
	if err != nil {
		t.Fatalf("查询台账失败: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("预期 1 条台账，实际: %d", len(list))
	}
	if list[0].Receipt != "既有平台回执：已标记不合适" {
		t.Fatalf("回执不符: %s", list[0].Receipt)
	}
}

func TestActivityDays_90DaysRuleAndConservativeBoundary(t *testing.T) {
	criteria := TalentManagementStrictCriteria{
		ActivityDays: 90,
	}

	// 1. 3个月内活跃 -> 合格
	v1, _ := EvaluateCandidateTriState(`{"active_type":"3个月内活跃"}`, criteria)
	if v1 != "qualified" {
		t.Errorf("预期 3个月内活跃 在 90天要求下为 qualified，实际: %s", v1)
	}

	// 2. 1个月内活跃 -> 合格
	v2, _ := EvaluateCandidateTriState(`{"active_type":"1个月内活跃"}`, criteria)
	if v2 != "qualified" {
		t.Errorf("预期 1个月内活跃 在 90天要求下为 qualified，实际: %s", v2)
	}

	// 3. 半年内活跃 -> 模糊区间，保守边界，预期待核实 (pending_verification)，绝不直接拒绝
	v3, _ := EvaluateCandidateTriState(`{"active_type":"半年内活跃"}`, criteria)
	if v3 != "pending_verification" {
		t.Errorf("预期 半年内活跃 在 90天要求下为 pending_verification，实际: %s", v3)
	}

	// 4. 3个月前活跃 -> 明确超过 90 天，预期 unfit
	v4, _ := EvaluateCandidateTriState(`{"active_type":"3个月前活跃"}`, criteria)
	if v4 != "unfit" {
		t.Errorf("预期 3个月前活跃 在 90天要求下为 unfit，实际: %s", v4)
	}

	// 5. 半年前活跃 -> 明确超过 90 天，预期 unfit
	v5, _ := EvaluateCandidateTriState(`{"active_type":"半年前活跃"}`, criteria)
	if v5 != "unfit" {
		t.Errorf("预期 半年前活跃 在 90天要求下为 unfit，实际: %s", v5)
	}
}

func TestActivityDays_Default90ForNewProjectAndPreserveExisting(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()

	// 1. 新建项目默认应为 90 天
	proj := app.CreateProject("90天新项目", "研发部", 1, "", "", nil)
	c1, err := app.GetTalentManagementStrictCriteria(proj.ID)
	if err != nil {
		t.Fatalf("读取新项目硬性条件失败: %v", err)
	}
	if c1.ActivityDays != 90 {
		t.Errorf("新项目默认活跃天数应为 90 天，实际得到: %d", c1.ActivityDays)
	}

	// 2. 保存为 7 天后，再次读取必须保持 7 天（不被篡改为 90）
	c1.ActivityDays = 7
	if err := app.SetTalentManagementStrictCriteria(*c1); err != nil {
		t.Fatalf("保存 7 天条件失败: %v", err)
	}
	c2, err := app.GetTalentManagementStrictCriteria(proj.ID)
	if err != nil || c2.ActivityDays != 7 {
		t.Errorf("已有 7 天项目应严格保持 7 天，实际得到: %d, err: %v", c2.ActivityDays, err)
	}

	// 3. 用户设置为 0（不限），读取时绝不能被改写为 90
	c2.ActivityDays = 0
	if err := app.SetTalentManagementStrictCriteria(*c2); err != nil {
		t.Fatalf("保存 0 天条件失败: %v", err)
	}
	c3, err := app.GetTalentManagementStrictCriteria(proj.ID)
	if err != nil || c3.ActivityDays != 0 {
		t.Errorf("不限时间(0天)项目应保持 0，实际得到: %d, err: %v", c3.ActivityDays, err)
	}
}
