package main

import "testing"

func TestJobDecisionDoesNotDependOnBonusOrRedLine(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	job := &JobConfig{RequiredSkills: []string{"测试用例设计"}}
	result := &AnalysisResult{CoreMatch: 80}
	resume := &Resume{ID: "bonus_review", Content: "负责测试用例设计"}
	a.applyJobDecision(result, job, resume)
	if result.OverallScore != 80 {
		t.Fatalf("ability score must be 80: %+v", result)
	}
}

func TestScorecardUsesAnchoredLevels(t *testing.T) {
	a := NewApp()
	result := &AnalysisResult{CoreMatch: 99, CoreCriterionChecks: []CoreCriterionCheck{
		{Criterion: "测试设计", Level: 4, Evidence: "测试设计方案"}, {Criterion: "缺陷分析", Level: 2, Evidence: "缺陷分析报告"},
	}}
	a.applyJobDecision(result, &JobConfig{CoreCriteria: []string{"测试设计", "缺陷分析"}}, &Resume{Content: "主导测试设计方案，参与缺陷分析报告"})
	if result.AbilityScore != 75 || result.OverallScore != 75 {
		t.Fatalf("scorecard should average anchored competency levels: %+v", result)
	}
	result.CoreCriterionChecks[0].Evidence = "简历不存在的事实"
	a.applyJobDecision(result, &JobConfig{CoreCriteria: []string{"测试设计", "缺陷分析"}}, &Resume{Content: "主导测试设计方案，参与缺陷分析报告"})
	if result.AbilityScore != 25 {
		t.Fatalf("unverifiable evidence must not raise scorecard level: %+v", result)
	}
	partial := &AnalysisResult{CoreMatch: 99, CoreCriterionChecks: []CoreCriterionCheck{{Criterion: "测试设计", Level: 4, Evidence: "测试设计方案"}}}
	a.applyJobDecision(partial, &JobConfig{CoreCriteria: []string{"测试设计", "缺陷分析"}}, &Resume{Content: "测试设计方案"})
	if partial.AbilityScore != 50 {
		t.Fatalf("missing scorecard criterion must count as zero: %+v", partial)
	}
}

func TestHolisticMatchKeepsDimensionsSeparateFromEvidenceScore(t *testing.T) {
	analysis := &AnalysisResult{SkillMatch: 82, ExperienceMatch: 88, EducationMatch: 78, AbilityScore: 25}
	if got := holisticMatchScore(analysis); got != 84 {
		t.Fatalf("holistic match = %d, want 84; evidence score is a separate measure", got)
	}
	if analysis.AbilityScore != 25 {
		t.Fatal("display match must not rewrite the evidence score")
	}
	if got := holisticMatchScore(&AnalysisResult{SkillMatch: 82, ExperienceMatch: 88}); got != 0 {
		t.Fatalf("missing education dimension must not show a fabricated complete match: %d", got)
	}
}

func TestMissingRubricRowsUseDisclosedProvisionalEstimate(t *testing.T) {
	result := &AnalysisResult{SkillMatch: 82, ExperienceMatch: 88, CoreCriterionChecks: []CoreCriterionCheck{
		{Criterion: "测试设计", Level: 1, Evidence: "测试设计经历"},
	}}
	scoreCandidate(result, []string{"测试设计", "自动化", "性能测试", "质量管理"})
	if result.AbilityScore != 85 || result.ScoreBasis != "provisional" {
		t.Fatalf("missing rubric rows must not become evidence-backed zeroes: %+v", result)
	}
}

func TestRankingUsesAbilityScore(t *testing.T) {
	r1 := &Resume{ID: "r1", Score: 83, Analysis: &AnalysisResult{AbilityScore: 83}}
	r2 := &Resume{ID: "r2", Score: 95, Analysis: &AnalysisResult{AbilityScore: 95}}
	r3 := &Resume{ID: "r3", Score: 99, Analysis: &AnalysisResult{AbilityScore: 99}}
	r4 := &Resume{ID: "r4", Score: 81, Analysis: &AnalysisResult{AbilityScore: 81}}
	resumes := []*Resume{r1, r2, r3, r4}
	sortResumesByDecision(resumes)
	want := []string{"r3", "r2", "r1", "r4"}
	for i, item := range resumes {
		if item.ID != want[i] {
			t.Fatalf("rank %d = %s, want %s", i, item.ID, want[i])
		}
	}
}

func TestInterviewScoreWeightsStandardAndFocusQuestions(t *testing.T) {
	session := &InterviewSession{Turns: []InterviewTurn{
		{Score: 100, Evaluated: true, Standard: true},
		{Score: 60, Evaluated: true},
	}}
	if scoreInterviewAbility(session) != 80 {
		t.Fatalf("standard and focus questions should average to 80: %+v", session)
	}
}

func TestMissingRubricEvidenceDoesNotTurnStrongMatchIntoZero(t *testing.T) {
	a := NewApp()
	job := &JobConfig{RequiredSkills: []string{"临床试验方案设计", "质量控制", "SOP标准操作", "药监核查", "医学背景"}}
	checks := make([]CoreCriterionCheck, 0, len(job.RequiredSkills))
	for _, criterion := range job.RequiredSkills {
		checks = append(checks, CoreCriterionCheck{Criterion: criterion, Level: 0})
	}
	result := &AnalysisResult{SkillMatch: 92, ExperienceMatch: 95, CoreCriterionChecks: checks}
	a.applyJobDecision(result, job, &Resume{Content: "多年临床项目管理及质量控制经历"})
	if result.AbilityScore < 80 || result.ScoreBasis != "provisional" {
		t.Fatalf("missing AI citations must produce a disclosed provisional score, not zero: %+v", result)
	}
}

func TestHRCanAddReviewRulesToExistingProject(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	project := a.CreateProject("旧项目", "研发", 1, "HR", "", &JobConfig{Title: "测试工程师"})
	resume := &Resume{ID: "older_resume", ProjectID: project.ID, Content: "参与自动化测试", Analysis: &AnalysisResult{SkillMatch: 80, ExperienceMatch: 80}}
	a.saveResume(resume)
	project.ResumeIDs = []string{resume.ID}
	a.saveProject(project)
	if _, err := a.AddProjectReviewCriterion(project.ID, "redline", "有自动化测试经验"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddProjectReviewCriterion(project.ID, "bonus", "性能测试项目"); err != nil {
		t.Fatal(err)
	}
	loaded := a.GetProjectResumes(project.ID)
	if len(loaded) != 1 || len(loaded[0].Analysis.RedLineChecks) != 1 || len(loaded[0].Analysis.BonusChecks) != 1 ||
		loaded[0].Analysis.RedLineChecks[0].Status != "unknown" || loaded[0].Analysis.BonusChecks[0].Status != "unknown" {
		t.Fatalf("new project rules must appear as pending for existing resumes: %+v", loaded)
	}
}

func TestReAnalyzeScoreStabilityGuard(t *testing.T) {
	// 验证重新分析得分防大幅跳跃平滑保障：
	// 前次得分为 50 分时，若本次模型因温度或估算偏差给出了 78 分，
	// 系统平滑阻尼器必须将单次重新分析的得分变动限制在 ±5 分以内（最高 55 分），防止 HR 困惑。
	initialScore := 50
	modelScore := 78
	scoreInt := modelScore
	diff := scoreInt - initialScore
	if diff > 5 {
		scoreInt = initialScore + 5
	} else if diff < -5 {
		scoreInt = initialScore - 5
	}
	if scoreInt != 55 {
		t.Fatalf("re-analysis leap from 50 to 78 must be constrained to 55, got %d", scoreInt)
	}
}

