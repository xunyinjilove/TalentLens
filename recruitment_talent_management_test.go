package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTalentManagementPendingReviewKeepsFailedRuleReason(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", &JobConfig{Title: "测试岗位"})
	if project == nil {
		t.Fatal("create project failed")
	}
	if err := app.SetPlatformJobMapping("51job", "co:1:hr:2", "job-1", project.ID); err != nil {
		t.Fatal(err)
	}
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	candidate := talentManagementCandidate{CandidateID: "candidate-1", RecordID: "record-1", Name: "测试人选"}
	reasons := []RuleCheckItem{{Rule: "MIN_EDUCATION", Label: "最低学历", Status: "fail", Evidence: "最高学历大专，要求本科"}}
	if err := saveTalentManagementPendingReview(db, project.ID, "co:1:hr:2", "job-1", candidate, "待处理", "unfit", reasons); err != nil {
		t.Fatal(err)
	}
	items, err := app.ListTalentManagementPendingReviews(project.ID)
	if err != nil || len(items) != 1 || len(items[0].Reasons) != 1 || items[0].Reasons[0].Evidence != reasons[0].Evidence {
		t.Fatalf("pending reasons: %+v, err=%v", items, err)
	}
}

func TestTalentManagementImportCanBeStopped(t *testing.T) {
	app := NewApp()
	app.Begin51JobManagementImport()
	ctx := app.managementImportContext()
	app.Stop51JobManagementImport()
	if ctx.Err() == nil {
		t.Fatal("stop did not cancel the active import")
	}
	app.End51JobManagementImport()
	if app.managementImportContext().Err() != nil {
		t.Fatal("cancelled context remained active")
	}
}

func TestTalentManagementPlatformStageMapping(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("阶段映射岗位", "", 1, "", "", &JobConfig{Title: "阶段映射岗位"})
	if project == nil {
		t.Fatal("create project failed")
	}
	const scope = "co:1:hr:2"
	if err := app.SetPlatformJobMapping("51job", scope, "job-1", project.ID); err != nil {
		t.Fatal(err)
	}
	complete := true
	for _, tc := range []struct{ status, expected string }{
		{"待处理", "new"}, {"合适", "pending_contact"}, {"面试", "first_interview"}, {"待定", "new"},
	} {
		record := ApplicationRecord{Platform: "51job", PlatformAccount: scope,
			ApplicationID: "record-" + tc.status, PlatformCandidateID: "candidate-" + tc.status,
			PlatformJobID: "job-1", CandidateName: tc.status + "人选", ResumeText: "完整简历正文",
			DocumentsComplete: &complete, PlatformStatus: tc.status}
		if result, err := app.ingestApplicationRecords("51job", []ApplicationRecord{record}); err != nil || result.AddedCount != 1 {
			t.Fatalf("import %s: %+v, %v", tc.status, result, err)
		}
		db, _ := app.getRecruitmentDB()
		if err := app.applyTalentManagementStage(db, project.ID, record); err != nil {
			t.Fatalf("stage %s: %v", tc.status, err)
		}
		var resume *Resume
		for _, candidateResume := range app.GetProjectResumes(project.ID) {
			if candidateResume.ID == applicationResumeID(record) {
				resume = candidateResume
				break
			}
		}
		if resume == nil {
			t.Fatalf("missing resume for %s", tc.status)
		}
		resume.Status = "done"
		app.syncRecruitmentResume(resume)
		var stage string
		if err := db.QueryRow(`SELECT pc.stage FROM project_candidates pc JOIN candidate_sources cs ON cs.candidate_id=pc.candidate_id
			WHERE pc.project_id=? AND cs.platform_candidate_id=?`, project.ID, record.PlatformCandidateID).Scan(&stage); err != nil || stage != tc.expected {
			t.Fatalf("%s stage=%q, want %q, err=%v", tc.status, stage, tc.expected, err)
		}
	}
}

func TestArchiveRejectedManagementCandidateRemovesFormalLibraryLink(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", &JobConfig{Title: "测试岗位"})
	if err := app.SetPlatformJobMapping("51job", "co:1:hr:2", "job-1", project.ID); err != nil {
		t.Fatal(err)
	}
	complete := true
	record := ApplicationRecord{Platform: "51job", PlatformAccount: "co:1:hr:2", PlatformJobID: "job-1", ApplicationID: "rec-1", PlatformCandidateID: "cand-1", CandidateName: "测试人选", PlatformStatus: "合适", ResumeText: "完整简历正文", DocumentsComplete: &complete}
	if _, err := app.ingestApplicationRecords("51job", []ApplicationRecord{record}); err != nil {
		t.Fatal(err)
	}
	db, _ := app.getRecruitmentDB()
	if err := archiveRejectedManagementCandidate(db, project.ID, "cand-1"); err != nil {
		t.Fatal(err)
	}
	if got := app.GetProjectResumes(project.ID); len(got) != 0 {
		t.Fatalf("archived candidate remains in formal library: %d", len(got))
	}
}

func TestReconcileUnfitManagementCandidatesNeverImportsRejectedResume(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", &JobConfig{Title: "测试岗位"})
	if err := app.SetPlatformJobMapping("51job", "co:1:hr:2", "job-1", project.ID); err != nil {
		t.Fatal(err)
	}
	complete := true
	record := ApplicationRecord{Platform: "51job", PlatformAccount: "co:1:hr:2", PlatformJobID: "job-1", ApplicationID: "rec-old", PlatformCandidateID: "cand-old", CandidateName: "旧简历", PlatformStatus: "面试", ResumeText: "完整简历", DocumentsComplete: &complete}
	if _, err := app.ingestApplicationRecords("51job", []ApplicationRecord{record}); err != nil {
		t.Fatal(err)
	}
	db, _ := app.getRecruitmentDB()
	ids := []talentManagementCandidate{
		{CandidateID: "cand-old", RecordID: "rec-old", JobID: "job-1"},
		{CandidateID: "cand-only-on-platform", RecordID: "rec-new", JobID: "job-1"},
	}
	if err := reconcileUnfitManagementCandidates(db, project.ID, "co:1:hr:2", "job-1", ids); err != nil {
		t.Fatal(err)
	}
	if got := app.GetProjectResumes(project.ID); len(got) != 0 {
		t.Fatalf("平台不合适对账后正式库仍有 %d 份简历", len(got))
	}
}

func TestStoredAssociateFromPendingStatusIsArchivedBeforeSync(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("区域销售经理", "", 1, "", "", &JobConfig{Title: "区域销售经理"})
	const scope = "co:1:hr:2"
	if err := app.SetPlatformJobMapping("51job", scope, "job-1", project.ID); err != nil {
		t.Fatal(err)
	}
	if err := app.SetTalentManagementStrictCriteria(TalentManagementStrictCriteria{ProjectID: project.ID, MinEducation: "本科", ActivityDays: 90}); err != nil {
		t.Fatal(err)
	}
	complete := true
	record := ApplicationRecord{Platform: "51job", PlatformAccount: scope, PlatformJobID: "job-1",
		ApplicationID: "rec-1", PlatformCandidateID: "cand-1", CandidateName: "大专人选", PlatformStatus: "待定",
		ResumeText: `{"degree":"大专","topdegree":"大专","activetimelabel":"3日内活跃"}`, DocumentsComplete: &complete}
	if result, err := app.ingestApplicationRecords("51job", []ApplicationRecord{record}); err != nil || result.AddedCount != 1 {
		t.Fatalf("old import: %+v %v", result, err)
	}
	if err := app.reconcileStored51JobHardCriteria(project.ID, scope, "job-1"); err != nil {
		t.Fatal(err)
	}
	if got := app.GetProjectResumes(project.ID); len(got) != 0 {
		t.Fatalf("大专旧简历仍在正式库: %d", len(got))
	}
	reviews, err := app.ListTalentManagementPendingReviews(project.ID)
	if err != nil || len(reviews) != 1 || reviews[0].Verdict != "unfit" {
		t.Fatalf("missing actionable reason: %+v %v", reviews, err)
	}
}

func TestParseEducationRank(t *testing.T) {
	cases := []struct {
		input    string
		expected int
	}{
		{"博士", 5},
		{"硕士研究生", 4},
		{"MBA", 4},
		{"本科", 3},
		{"全日制学士", 3},
		{"大专", 2},
		{"高职专科", 2},
		{"高中", 1},
		{"中专", 1},
		{"未知", 0},
		{"", 0},
	}
	for _, c := range cases {
		if r := parseEducationRank(c.input); r != c.expected {
			t.Errorf("parseEducationRank(%q) = %d, expected %d", c.input, r, c.expected)
		}
	}
}

func TestParseWorkYears(t *testing.T) {
	cases := []struct {
		input    string
		expected float64
		ok       bool
	}{
		{"无经验", 0, true},
		{"在校生/应届生", 0, true},
		{"35年", 35, true},
		{"3.5年", 3.5, true},
		{"1年", 1, true},
		{"", 0, false},
	}
	for _, c := range cases {
		years, ok := parseWorkYears(c.input)
		if ok != c.ok || years != c.expected {
			t.Errorf("parseWorkYears(%q) = (%f, %v), expected (%f, %v)", c.input, years, ok, c.expected, c.ok)
		}
	}
}

func TestEvaluateCandidateStrictCriteria(t *testing.T) {
	criteria := TalentManagementStrictCriteria{
		MinEducation:              "本科",
		MinWorkYears:              3.0,
		RejectNonLocalWillingness: true,
		TargetCity:                "上海",
	}

	// 1. 满足条件的人选（本科，35年，意向上海）
	cand1 := map[string]interface{}{
		"base_info": map[string]interface{}{
			"top_degree_value": "本科",
			"work_year_value":  "35年",
		},
		"optimal_job_intention": map[string]interface{}{
			"intention_area": "上海",
		},
	}
	cand1Raw, _ := json.Marshal(cand1)
	decision, rule, _, _ := EvaluateCandidateStrictCriteria(string(cand1Raw), criteria)
	if decision != "KEEP_MATCHED" {
		t.Errorf("cand1 decision = %s, expected KEEP_MATCHED", decision)
	}

	// 2. 学历低于硬性要求（大专 < 本科）
	cand2 := map[string]interface{}{
		"base_info": map[string]interface{}{
			"top_degree_value": "大专",
			"work_year_value":  "5年",
		},
		"optimal_job_intention": map[string]interface{}{
			"intention_area": "上海",
		},
	}
	cand2Raw, _ := json.Marshal(cand2)
	decision, rule, _, _ = EvaluateCandidateStrictCriteria(string(cand2Raw), criteria)
	if decision != "UNFIT" || rule != "EDU_BELOW_MIN" {
		t.Errorf("cand2 decision = (%s, %s), expected (UNFIT, EDU_BELOW_MIN)", decision, rule)
	}

	// 3. 工作年限低于硬性要求（1年 < 3年）
	cand3 := map[string]interface{}{
		"base_info": map[string]interface{}{
			"top_degree_value": "硕士",
			"work_year_value":  "1年",
		},
		"optimal_job_intention": map[string]interface{}{
			"intention_area": "上海",
		},
	}
	cand3Raw, _ := json.Marshal(cand3)
	decision, rule, _, _ = EvaluateCandidateStrictCriteria(string(cand3Raw), criteria)
	if decision != "UNFIT" || rule != "EXP_BELOW_MIN" {
		t.Errorf("cand3 decision = (%s, %s), expected (UNFIT, EXP_BELOW_MIN)", decision, rule)
	}

	// 4. 明确表示意向城市不包含目标城市
	cand4 := map[string]interface{}{
		"base_info": map[string]interface{}{
			"top_degree_value": "本科",
			"work_year_value":  "5年",
		},
		"optimal_job_intention": map[string]interface{}{
			"intention_area": "成都",
		},
	}
	cand4Raw, _ := json.Marshal(cand4)
	decision, rule, _, _ = EvaluateCandidateStrictCriteria(string(cand4Raw), criteria)
	if decision != "UNFIT" || rule != "INTENT_CITY_MISMATCH" {
		t.Errorf("cand4 decision = (%s, %s), expected (UNFIT, INTENT_CITY_MISMATCH)", decision, rule)
	}

	// 5. 资料缺失/未填：绝不自动淘汰，保持 KEEP_UNKNOWN
	cand5 := map[string]interface{}{
		"base_info": map[string]interface{}{},
	}
	cand5Raw, _ := json.Marshal(cand5)
	decision, _, _, _ = EvaluateCandidateStrictCriteria(string(cand5Raw), criteria)
	if decision != "KEEP_UNKNOWN" {
		t.Errorf("cand5 decision = %s, expected KEEP_UNKNOWN", decision)
	}
}

func TestOneClickFilterTalentPool(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()

	project := app.CreateProject("销售经理", "销售部", 1, "", "", nil)
	// 设置硬性条件：要求本科
	err := app.SetTalentManagementStrictCriteria(TalentManagementStrictCriteria{
		ProjectID:    project.ID,
		MinEducation: "本科",
		MinWorkYears: 1.0,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 插入候选人1：大专
	cand1 := map[string]interface{}{
		"base_info": map[string]interface{}{
			"resume_name":      "张大专",
			"top_degree_value": "大专",
			"work_year_value":  "2年",
		},
	}
	cand1Raw, _ := json.Marshal(cand1)
	resume1 := &Resume{
		ID:                  "resume_dazhuan",
		ProjectID:           project.ID,
		FileName:            "张大专",
		Content:             string(cand1Raw),
		FileType:            ".txt",
		FileSize:            int64(len(cand1Raw)),
		SourcePlatform:      "51job",
		PlatformCandidateID: "cand_dazhuan_1",
		Status:              "pending",
		CreatedAt:           time.Now(),
	}
	if err := app.persistSearchCandidate(resume1); err != nil {
		t.Fatal(err)
	}

	// 插入候选人2：本科
	cand2 := map[string]interface{}{
		"base_info": map[string]interface{}{
			"resume_name":      "李本科",
			"top_degree_value": "本科",
			"work_year_value":  "3年",
		},
	}
	cand2Raw, _ := json.Marshal(cand2)
	resume2 := &Resume{
		ID:                  "resume_benke",
		ProjectID:           project.ID,
		FileName:            "李本科",
		Content:             string(cand2Raw),
		FileType:            ".txt",
		FileSize:            int64(len(cand2Raw)),
		SourcePlatform:      "51job",
		PlatformCandidateID: "cand_benke_1",
		Status:              "pending",
		CreatedAt:           time.Now(),
	}
	if err := app.persistSearchCandidate(resume2); err != nil {
		t.Fatal(err)
	}

	// 初始状态下人才库应该有2人
	candidates, err := app.ListTalentCandidates(project.ID)
	if err != nil || len(candidates) != 2 {
		t.Fatalf("初始候选人数量 = %d, expected 2, err = %v", len(candidates), err)
	}

	// 执行一键筛选
	excludedCount, err := app.OneClickFilterTalentPool(project.ID)
	if err != nil {
		t.Fatalf("OneClickFilterTalentPool failed: %v", err)
	}
	if excludedCount != 1 {
		t.Fatalf("排除人数 = %d, expected 1", excludedCount)
	}

	// 筛选后人才库应该只剩1人（李本科）
	candidatesAfter, err := app.ListTalentCandidates(project.ID)
	if err != nil || len(candidatesAfter) != 1 {
		t.Fatalf("筛选后候选人数量 = %d, expected 1, err = %v", len(candidatesAfter), err)
	}
	if candidatesAfter[0].Name != "李本科" {
		t.Fatalf("剩下候选人期望为李本科，实际为 %s", candidatesAfter[0].Name)
	}

	// 审计日志应该有被排除的“张大专”
	auditLogs, err := app.ListTalentManagementAuditLogs(project.ID)
	if err != nil || len(auditLogs) != 1 {
		t.Fatalf("审计日志数量 = %d, expected 1", len(auditLogs))
	}
	if auditLogs[0].CandidateName != "张大专" {
		t.Fatalf("审计日志候选人期望为张大专，实际为 %s", auditLogs[0].CandidateName)
	}

	// 测试清空审计日志
	if err := app.ClearTalentManagementAuditLogs(project.ID); err != nil {
		t.Fatalf("ClearTalentManagementAuditLogs failed: %v", err)
	}
	clearedLogs, err := app.ListTalentManagementAuditLogs(project.ID)
	if err != nil || len(clearedLogs) != 0 {
		t.Fatalf("清空后审计日志数量 = %d, expected 0", len(clearedLogs))
	}
}

func TestEvaluateCandidateStrictCriteria_NewDimensions(t *testing.T) {
	criteria := TalentManagementStrictCriteria{
		MinEducation:      "本科",
		MinAge:            25,
		MaxAge:            35,
		MaxExpectedSalary: 15, // 15k/月
		Gender:            "女",
		JobSeekingStatus:  "离职",
		MajorKeywords:     "药学,医学",
	}

	// 1. 年龄超标 (38岁)
	c1 := map[string]interface{}{
		"topdegree":  "本科",
		"displayage": "38岁",
		"sex":        "女",
		"highestdegree": map[string]interface{}{
			"degreemajor": "临床药学",
		},
		"job_hunting_status": "离职-随时到岗",
		"jobintention": []interface{}{
			map[string]interface{}{"displayexpectsalary": "10-12千/月"},
		},
	}
	b1, _ := json.Marshal(c1)
	decision, rule, _, _ := EvaluateCandidateStrictCriteria(string(b1), criteria)
	if decision != "UNFIT" || rule != "AGE_MISMATCH" {
		t.Errorf("c1 expected AGE_MISMATCH, got %s, %s", decision, rule)
	}

	// 2. 薪资超标 (20k/月 > 15k)
	c2 := map[string]interface{}{
		"topdegree":  "本科",
		"displayage": "28岁",
		"sex":        "女",
		"highestdegree": map[string]interface{}{
			"degreemajor": "临床药学",
		},
		"job_hunting_status": "离职-随时到岗",
		"jobintention": []interface{}{
			map[string]interface{}{"displayexpectsalary": "18-20千/月"},
		},
	}
	b2, _ := json.Marshal(c2)
	decision, rule, _, _ = EvaluateCandidateStrictCriteria(string(b2), criteria)
	if decision != "UNFIT" || rule != "SALARY_MISMATCH" {
		t.Errorf("c2 expected SALARY_MISMATCH, got %s, %s", decision, rule)
	}

	// 3. 性别不匹配 (男)
	c3 := map[string]interface{}{
		"topdegree":  "本科",
		"displayage": "28岁",
		"sex":        "男",
		"highestdegree": map[string]interface{}{
			"degreemajor": "药学",
		},
		"job_hunting_status": "离职-随时到岗",
	}
	b3, _ := json.Marshal(c3)
	decision, rule, _, _ = EvaluateCandidateStrictCriteria(string(b3), criteria)
	if decision != "UNFIT" || rule != "GENDER_MISMATCH" {
		t.Errorf("c3 expected GENDER_MISMATCH, got %s, %s", decision, rule)
	}

	// 4. 专业不匹配 (计算机)
	c4 := map[string]interface{}{
		"topdegree":  "本科",
		"displayage": "28岁",
		"sex":        "女",
		"highestdegree": map[string]interface{}{
			"degreemajor": "计算机科学与技术",
		},
		"job_hunting_status": "离职-随时到岗",
	}
	b4, _ := json.Marshal(c4)
	decision, rule, _, _ = EvaluateCandidateStrictCriteria(string(b4), criteria)
	if decision != "UNFIT" || rule != "MAJOR_MISMATCH" {
		t.Errorf("c4 expected MAJOR_MISMATCH, got %s, %s", decision, rule)
	}

	// 5. 全部满足
	c5 := map[string]interface{}{
		"topdegree":  "本科",
		"displayage": "28岁",
		"sex":        "女",
		"highestdegree": map[string]interface{}{
			"degreemajor": "中药学",
		},
		"job_hunting_status": "离职-随时到岗",
		"jobintention": []interface{}{
			map[string]interface{}{"displayexpectsalary": "8-10千/月"},
		},
	}
	b5, _ := json.Marshal(c5)
	decision, _, _, _ = EvaluateCandidateStrictCriteria(string(b5), criteria)
	if decision != "KEEP_MATCHED" {
		t.Errorf("c5 expected KEEP_MATCHED, got %s", decision)
	}
}
