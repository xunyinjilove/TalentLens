package main

import (
	"encoding/json"
	"testing"
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
