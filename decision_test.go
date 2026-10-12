package main

import "testing"

func TestBonusRequiresHRConfirmationAndNeverRaisesAbilityScore(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	job := &JobConfig{RequiredSkills: []string{"测试用例设计"}, BonusPoints: []string{"性能测试项目"}}
	result := &AnalysisResult{CoreMatch: 80, BonusMatch: 100, BonusMatches: []string{"性能测试项目"}}
	resume := &Resume{ID: "bonus_review", Content: "负责性能测试项目"}
	a.applyJobDecision(result, job, resume)
	if result.OverallScore != 80 || len(result.BonusChecks) != 1 || result.BonusChecks[0].Status != "unknown" {
		t.Fatalf("bonus must remain pending without raising ability score: %+v", result)
	}
	resume.Analysis = result
	resume.Score = 80
	a.saveResume(resume)
	if _, err := a.SetReviewedBonus(resume.ID, "性能测试项目", "met", ""); err == nil {
		t.Fatal("confirmed bonus needs HR evidence")
	}
	confirmed, err := a.SetReviewedBonus(resume.ID, "性能测试项目", "met", "候选人在初面中说明性能测试职责")
	if err != nil || confirmed.Analysis.ConfirmedBonusCount != 1 || confirmed.Score != 80 {
		t.Fatalf("bonus review must be saved without changing ability score: %+v %v", confirmed, err)
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

func TestLegacyAIVerdictsBecomePendingUntilHRReviews(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	project := a.CreateProject("测试工程师", "研发", 1, "HR", "", &JobConfig{
		RedLines: []string{"有自动化测试经验"}, BonusPoints: []string{"性能测试经验"},
	})
	resume := &Resume{ID: "legacy_review", ProjectID: project.ID, Content: "参与自动化测试", Analysis: &AnalysisResult{
		CoreMatch: 80, RedLineStatus: "passed", RedLineChecks: []RedLineCheck{{Criterion: "有自动化测试经验", Status: "met", Evidence: "参与自动化测试"}},
		BonusChecks: []BonusCheck{{Criterion: "性能测试经验", Status: "met", Evidence: "参与自动化测试"}}, ConfirmedBonusCount: 1,
	}}
	a.saveResume(resume)
	project.ResumeIDs = append(project.ResumeIDs, resume.ID)
	a.saveProject(project)
	loaded := a.GetProjectResumes(project.ID)
	if len(loaded) != 1 || loaded[0].Analysis.RedLineStatus != "pending" || loaded[0].Analysis.BonusChecks[0].Status != "unknown" || loaded[0].Analysis.ConfirmedBonusCount != 0 {
		t.Fatalf("old AI-only verdicts must be pending: %+v", loaded)
	}
	confirmed, err := a.SetReviewedRedLine(resume.ID, "有自动化测试经验", "met", "参与自动化测试")
	if err != nil || confirmed.Analysis.RedLineStatus != "passed" {
		t.Fatalf("HR should be able to review migrated redline: %+v %v", confirmed, err)
	}
	again := a.GetProjectResumes(project.ID)
	if again[0].Analysis.BonusChecks[0].Status != "unknown" || again[0].Analysis.BonusChecks[0].SuggestedStatus != "met" {
		t.Fatalf("unreviewed AI bonus hint should remain visible but pending: %+v", again[0].Analysis.BonusChecks)
	}
}

func TestRankingUsesEligibilityThenAbilityBandThenConfirmedBonus(t *testing.T) {
	passed := &Resume{ID: "passed", Score: 80, Analysis: &AnalysisResult{AbilityScore: 80, RedLineStatus: "passed", ConfirmedBonusCount: 1}}
	pending := &Resume{ID: "pending", Score: 95, Analysis: &AnalysisResult{AbilityScore: 95, RedLineStatus: "pending"}}
	failed := &Resume{ID: "failed", Score: 99, Analysis: &AnalysisResult{AbilityScore: 99, RedLineStatus: "failed"}}
	moreBonus := &Resume{ID: "more", Score: 81, Analysis: &AnalysisResult{AbilityScore: 81, RedLineStatus: "passed", ConfirmedBonusCount: 2}}
	resumes := []*Resume{failed, pending, passed, moreBonus}
	sortResumesByDecision(resumes)
	want := []string{"more", "passed", "pending", "failed"}
	for i, item := range resumes {
		if item.ID != want[i] {
			t.Fatalf("rank %d = %s, want %s", i, item.ID, want[i])
		}
	}
}

func TestInterviewScoreExcludesRedLineQuestions(t *testing.T) {
	session := &InterviewSession{Turns: []InterviewTurn{
		{Score: 90, Evaluated: true}, {Score: 10, Evaluated: true, RuleID: "redline"},
	}}
	if scoreInterviewAbility(session) != 90 {
		t.Fatalf("redline question affected competency score: %+v", session)
	}
	session = &InterviewSession{Turns: []InterviewTurn{
		{Score: 100, Evaluated: true, Standard: true},
		{Score: 50, Evaluated: true},
		{Score: 0, Evaluated: true, RuleID: "redline"},
	}}
	if scoreInterviewAbility(session) != 80 {
		t.Fatalf("standard and adaptive questions should use 60/40 weighting: %+v", session)
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
