package main

import "testing"

func TestValidatedAIJobRulesRequiresJobEvidenceAndCapsEachKind(t *testing.T) {
	job := JobConfig{Title: "软件测试工程师", JobDescription: "负责接口测试和自动化测试", RequiredSkills: []string{"Python", "接口测试"}}
	items := []aiJobRuleSuggestion{
		{Kind: "redline", Criterion: "能独立编写接口测试用例", Basis: "接口测试"},
		{Kind: "redline", Criterion: "具有 Python 自动化测试经验", Basis: "Python"},
		{Kind: "redline", Criterion: "能独立维护自动化测试脚本", Basis: "自动化测试"},
		{Kind: "redline", Criterion: "掌握数据分析", Basis: "数据分析"},
		{Kind: "redline", Criterion: "熟悉接口测试流程", Basis: "接口测试"},
		{Kind: "bonus", Criterion: "能优化 Python 测试工具", Basis: "Python"},
		{Kind: "bonus", Criterion: "有自动化测试框架实践", Basis: "自动化测试"},
		{Kind: "bonus", Criterion: "拥有名校学历", Basis: ""},
	}
	existing := []JobRule{{Kind: "redline", Criterion: "接口测试经验"}}
	got := validatedAIJobRules(items, job, existing)
	redlines, bonuses := 0, 0
	for _, rule := range got {
		if rule.Kind == "redline" {
			redlines++
		}
		if rule.Kind == "bonus" {
			bonuses++
		}
		if rule.Criterion == "掌握数据分析" || rule.Criterion == "拥有名校学历" {
			t.Fatalf("unsupported suggestion was accepted: %s", rule.Criterion)
		}
	}
	if redlines != 3 || bonuses != 2 {
		t.Fatalf("got redlines=%d bonus=%d, want 3 and 2", redlines, bonuses)
	}
}
