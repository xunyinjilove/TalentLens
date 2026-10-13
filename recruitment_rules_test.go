package main

import (
	"testing"
	"time"
)

func TestJobRulesNeedExplicitApprovalAndPreserveOldEvaluation(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	project := &Project{ID: "proj_123", Name: "Go 工程师", JobConfig: JobConfig{Title: "高级Go开发工程师", RequiredSkills: []string{"Go"}}, ResumeIDs: []string{"resume_1"}, CreatedAt: now, UpdatedAt: now}
	resume := &Resume{ID: "resume_1", ProjectID: project.ID, FileName: "甲", Content: "Go 服务开发", Status: "done", Analysis: &AnalysisResult{CandidateName: "甲", AbilityScore: 75, RedLineStatus: "pending"}, CreatedAt: now}
	writeLegacyJSON(t, dir, "projects", project.ID, project)
	writeLegacyJSON(t, dir, "resumes", resume.ID, resume)
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	rules, err := app.SuggestJobRules(project.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) < 2 {
		t.Fatalf("expected template redline and core, got %+v", rules)
	}
	if got := app.GetProject(project.ID).JobConfig.RedLines; len(got) != 0 {
		t.Fatalf("draft changed active config: %v", got)
	}
	choices := make([]JobRuleApproval, 0, len(rules))
	for _, rule := range rules {
		choices = append(choices, JobRuleApproval{ID: rule.ID, Criterion: rule.Criterion, Approved: true})
	}
	version, err := app.ApproveJobRules(project.ID, "HR", choices)
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("version %d", version)
	}
	if got := app.GetProject(project.ID).JobConfig.RedLines; len(got) != 1 {
		t.Fatalf("approved redline missing: %v", got)
	}
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	var oldVersion int
	if err := db.QueryRow(`SELECT rule_version FROM evaluations WHERE id=?`, "legacy_eval_"+project.ID+"_"+resume.ID).Scan(&oldVersion); err != nil {
		t.Fatal(err)
	}
	if oldVersion != 0 {
		t.Fatalf("history was rewritten to version %d", oldVersion)
	}
}
