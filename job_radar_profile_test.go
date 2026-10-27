package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBatchRadarFixesExistingMetricsAndRanksOnlyWithEnoughEvidence(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	p := a.CreateProject("临床项目经理", "临床", 1, "HR", "", &JobConfig{
		Title:          "临床项目经理",
		RequiredSkills: []string{"临床试验方案设计", "GCP法规与合规", "项目全周期管理", "报批文件撰写"},
		CoreCriteria:   []string{"药监核查应对能力", "多方沟通与团队协同"},
	})
	fixed, err := a.ensureBatchJobRadar(p.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(fixed.JobConfig.RadarMetrics); got != 6 {
		t.Fatalf("expected six fixed job axes, got %d", got)
	}
	if fixed.JobConfig.RadarProfileHash == "" || fixed.JobConfig.RadarProfileVersion != 1 {
		t.Fatal("the batch must fix a version before analyzing resumes")
	}
	checks := make([]CoreCriterionCheck, 0, 6)
	for i, metric := range fixed.JobConfig.RadarMetrics {
		check := CoreCriterionCheck{Criterion: metric, Level: 3, EvidenceStatus: "known", Evidence: "可定位原文"}
		if i == 0 {
			check.EvidenceStatus, check.Evidence = "unknown", ""
		}
		checks = append(checks, check)
	}
	result := &AnalysisResult{CoreCriterionChecks: checks}
	a.applyJobDecision(result, &fixed.JobConfig, nil)
	if !result.RankEligible || result.ScoreBasis != "rubric" || result.RadarCoverage < 0.8 {
		t.Fatalf("five of six known axes should yield an official score: %+v", result)
	}
	checks[1].EvidenceStatus, checks[1].Evidence, checks[1].Level = "unknown", "", 1
	result = &AnalysisResult{CoreCriterionChecks: checks}
	a.applyJobDecision(result, &fixed.JobConfig, nil)
	if !result.RankEligible || result.ScoreBasis != "provisional" || result.RadarCoverage >= 0.8 {
		t.Fatalf("four of six known axes should remain rank-eligible with provisional basis: %+v", result)
	}
	again, err := a.ensureBatchJobRadar(p.ID, nil)
	if err != nil || again.JobConfig.RadarProfileVersion != 1 {
		t.Fatalf("same job should reuse fixed version: version=%d err=%v", again.JobConfig.RadarProfileVersion, err)
	}
}

func TestBatchRadarAllowsOneOrTwoExistingAxes(t *testing.T) {
	for _, metrics := range [][]string{{"客户沟通"}, {"客户沟通", "需求分析"}} {
		a := NewApp()
		a.dataDirOverride = t.TempDir()
		p := a.CreateProject("客户经理", "销售", 1, "HR", "", &JobConfig{Title: "客户经理", RequiredSkills: metrics})
		fixed, err := a.ensureBatchJobRadar(p.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(fixed.JobConfig.RadarMetrics) != len(metrics) {
			t.Fatalf("must retain %d axes without adding extras, got %v", len(metrics), fixed.JobConfig.RadarMetrics)
		}
		checks := make([]CoreCriterionCheck, 0, len(metrics))
		for _, metric := range metrics {
			checks = append(checks, CoreCriterionCheck{Criterion: metric, Level: 3, EvidenceStatus: "known", Evidence: "可定位原文"})
		}
		result := &AnalysisResult{CoreCriterionChecks: checks}
		a.applyJobDecision(result, &fixed.JobConfig, nil)
		if !result.RankEligible || result.AbilityScore != 75 {
			t.Fatalf("fully covered 1-2 axes should score directly: %+v", result)
		}
	}
}

func TestOneAxisInitialAnalysisDoesNotRequestUnnecessarySecondAIPass(t *testing.T) {
	a := NewApp()
	resume := &Resume{Content: "负责客户沟通与项目推进"}
	job := &JobConfig{Title: "客户经理", RequiredSkills: []string{"客户沟通"}, RadarMetrics: []string{"客户沟通"}}
	result := &AnalysisResult{CoreCriterionChecks: []CoreCriterionCheck{{Criterion: "客户沟通", Level: 3, Evidence: "客户沟通"}}}
	a.completeInitialCompetencies(result, resume, job, nil)
	if len(result.CoreCriterionChecks) != 1 || result.CoreCriterionChecks[0].EvidenceStatus != "known" {
		t.Fatalf("valid one-axis evidence should be retained without a second AI call: %+v", result.CoreCriterionChecks)
	}
}

func TestBatchRadarRejectsObviousCrossOccupationMetricWithoutChangingProject(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	p := a.CreateProject("器械工程师", "研发", 1, "HR", "", &JobConfig{Title: "器械工程师", RequiredSkills: []string{"Vue3 前端开发", "机械结构设计"}})
	if _, err := a.ensureBatchJobRadar(p.ID, nil); err == nil {
		t.Fatal("unrelated frontend metric must stop auto confirmation")
	}
	if updated := a.GetProject(p.ID); updated.JobConfig.RadarProfileHash != "" {
		t.Fatal("invalid job metrics must not be saved as a profile")
	}
}

func TestBatchRadarRequiresJobInformationWhenNoMetricsExist(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	p := a.CreateProject("未知职位", "", 1, "HR", "", &JobConfig{Title: "未知职位"})
	if _, err := a.ensureBatchJobRadar(p.ID, nil); err == nil {
		t.Fatal("without configured metrics or an AI result, batch must stop before spending resume tokens")
	}
}

func TestBatchRadarInfersMissingMetricsAndFreezesThemBeforeAnalysis(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected AI path: %s", r.URL.Path)
		}
		answer := `{"confident":true,"metrics":["临床试验进度管理","GCP合规","研究中心协调"]}`
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{map[string]interface{}{"message": map[string]string{"content": answer}}},
		})
	}))
	defer server.Close()
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	p := a.CreateProject("临床项目经理", "临床", 1, "HR", "", &JobConfig{Title: "临床项目经理", JobDescription: "负责临床试验项目交付"})
	fixed, err := a.ensureBatchJobRadar(p.ID, &AIConfig{APIKey: "test", BaseURL: server.URL, Model: "test", MaxRetries: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed.JobConfig.RadarMetrics) != 3 || fixed.JobConfig.RadarProfileVersion != 1 {
		t.Fatalf("AI-inferred job axes should be fixed before resumes are analyzed: %+v", fixed.JobConfig)
	}
}

func TestBatchRadarRebuildsAfterRequirementsChange(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	p := a.CreateProject("客户经理", "销售", 1, "HR", "", &JobConfig{Title: "客户经理", RequiredSkills: []string{"客户沟通"}})
	first, err := a.ensureBatchJobRadar(p.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	first.JobConfig.RequiredSkills = []string{"需求分析", "合同谈判"}
	if err := a.UpdateProject(first); err != nil {
		t.Fatal(err)
	}
	second, err := a.ensureBatchJobRadar(p.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.JobConfig.RadarProfileVersion != 2 || len(second.JobConfig.RadarMetrics) != 2 || second.JobConfig.RadarMetrics[0] != "需求分析" {
		t.Fatalf("changed requirements must replace old job axes with a new version: %+v", second.JobConfig)
	}
}

func TestJobRadarDraftNeedsHRConfirmationAndFreezesVersion(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	p := a.CreateProject("大客户销售经理", "销售", 1, "HR", "", &JobConfig{
		Title:          "大客户销售经理",
		RequiredSkills: []string{"大项目全流程把控", "回款与履约风险管理", "跨部门资源协同"},
		CoreCriteria:   []string{"大项目全流程把控", "回款与履约风险管理", "跨部门资源协同"},
	})
	before := a.GetProject(p.ID)
	if len(getUnifiedJobMetrics(&before.JobConfig)) != 3 {
		t.Fatal("fixture must start with three metrics")
	}
	draft, err := a.PreviewProjectJobRadar(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Metrics) < 5 || len(draft.Metrics) > 6 {
		t.Fatalf("expected relevant supplementary proposal, got %+v", draft)
	}
	if a.GetProject(p.ID).JobConfig.RadarProfileVersion != 0 {
		t.Fatal("preview must not mutate project")
	}
	updated, err := a.ApplyProjectJobRadar(draft)
	if err != nil {
		t.Fatal(err)
	}
	if updated.JobConfig.RadarProfileVersion != 1 || updated.JobConfig.RadarProfileHash == "" {
		t.Fatalf("profile was not fixed: %+v", updated.JobConfig)
	}
	if _, err := a.ApplyProjectJobRadar(JobRadarDraft{ProjectID: p.ID, SourceHash: "outdated", Metrics: draft.Metrics}); err == nil {
		t.Fatal("stale draft must be rejected")
	}
}

func TestSuggestJobSkillsGeneratesDeduplicatedDraftWithoutSaving(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		answer := `{"skills":["临床试验方案设计","GCP合规管理","受试者招募","中心监查","数据管理"]}`
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []interface{}{map[string]interface{}{"message": map[string]string{"content": answer}}},
		})
	}))
	defer server.Close()
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	a.config.AI = AIConfig{APIKey: "test", BaseURL: server.URL, Model: "test", MaxRetries: 1}
	res, err := a.SuggestJobSkills(JobSkillSuggestionRequest{
		Title:          "临床项目经理",
		Department:     "临床医学部",
		JobDescription: "负责医疗器械临床试验全流程管理与GCP合规把控",
		ExistingSkills: []string{"GCP合规管理"},
	})
	if err != nil || res.Status != "ready" {
		t.Fatalf("expected ready skill suggestion draft, got res=%+v err=%v", res, err)
	}
	if len(res.Skills) != 4 {
		t.Fatalf("expected 4 deduplicated novel skills, got %v", res.Skills)
	}
}

func TestSavingUpdatedSkillsMarksOldEvaluationsStaleAndPreservesOldConclusion(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	p := a.CreateProject("临床项目经理", "临床", 1, "HR", "", &JobConfig{
		Title:          "临床项目经理",
		RequiredSkills: []string{"临床试验方案设计", "GCP合规管理"},
	})
	r := &Resume{
		ID:        "resume_stale_check",
		ProjectID: p.ID,
		Status:    "done",
		Score:     88,
		Content:   "主导临床试验方案设计并负责GCP合规管理",
		Analysis: &AnalysisResult{
			OverallScore:   88,
			AbilityScore:   88,
			Recommendation: "strong_recommend",
			CoreCriterionChecks: []CoreCriterionCheck{
				{Criterion: "临床试验方案设计", Level: 4, Evidence: "主导临床试验方案设计", EvidenceStatus: "known"},
				{Criterion: "GCP合规管理", Level: 3, Evidence: "负责GCP合规管理", EvidenceStatus: "known"},
			},
		},
	}
	a.saveResume(r)
	p.ResumeIDs = []string{r.ID}
	a.saveProject(p)

	// 追加新技能并保存职位
	p.JobConfig.RequiredSkills = []string{"临床试验方案设计", "GCP合规管理", "药监注册核查"}
	if err := a.UpdateProject(p); err != nil {
		t.Fatal(err)
	}
	loaded := a.GetProjectResumes(p.ID)
	if len(loaded) != 1 {
		t.Fatalf("expected 1 resume, got %d", len(loaded))
	}
	got := loaded[0]
	if !got.Analysis.RadarStale || got.Analysis.RankEligible {
		t.Fatalf("old evaluation must be marked stale and ineligible for ranking after skill update: %+v", got.Analysis)
	}
	if got.Score != 88 || got.Analysis.AbilityScore != 88 || got.Analysis.Recommendation != "strong_recommend" {
		t.Fatalf("old score and conclusion must be preserved while pending review: score=%d analysis=%+v", got.Score, got.Analysis)
	}
}

func TestStartBatchAnalysisSupportsCrossProjectCandidates(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()

	p1 := a.CreateProject("销售经理", "销售", 1, "HR", "", &JobConfig{
		Title:          "销售经理",
		RequiredSkills: []string{"大客户销售", "商务谈判"},
	})
	p2 := a.CreateProject("Go工程师", "技术", 1, "HR", "", &JobConfig{
		Title:          "Go工程师",
		RequiredSkills: []string{"Go并发编程", "微服务架构"},
	})

	r1 := &Resume{
		ID:        "resume_test_1",
		ProjectID: p1.ID,
		FileName:  "candidate1.pdf",
		Content:   "5年大客户销售与商务谈判经验",
	}
	r2 := &Resume{
		ID:        "resume_test_2",
		ProjectID: p2.ID,
		FileName:  "candidate2.pdf",
		Content:   "3年Go高并发开发与微服务落地经验",
	}
	a.saveResume(r1)
	a.saveResume(r2)

	cfg := &AIConfig{
		APIKey:  "test_key_12345",
		BaseURL: "http://127.0.0.1:9999",
	}

	// 验证批量分析支持跨岗位多项目 ID 集合，绝不抛出“一批简历只能属于同一个职位专属分库”错误
	err := a.StartBatchAnalysis([]string{r1.ID, r2.ID}, cfg, nil)
	if err != nil {
		t.Fatalf("cross project batch analysis must succeed without single-project error, got: %v", err)
	}
}
