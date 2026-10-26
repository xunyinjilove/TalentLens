package main

import (
	"encoding/json"
	"testing"
	"time"
)

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
}
