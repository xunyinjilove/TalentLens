package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompetencyEvidenceAllowsWrappedOriginalAndRejectsInventedQuote(t *testing.T) {
	r := &Resume{Content: "负责 Go\n服务开发。"}
	checks := []CoreCriterionCheck{{Criterion: "Go", Level: 2, Evidence: "负责 Go 服务开发。"}, {Criterion: "PyTorch", Level: 4, Evidence: "主导 PyTorch 项目"}}
	validateCompetencyEvidence(checks, r)
	if checks[0].Level != 2 || checks[0].Evidence != r.Content {
		t.Fatal("line-wrapped source quote should be retained")
	}
	if checks[1].Level != 0 || checks[1].Evidence != "" {
		t.Fatal("invented skill evidence must never appear as supported")
	}
}

func TestSupplementCapabilitiesPreservesConcurrentContactAndHRReview(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := &Project{ID: "project_skills", JobConfig: JobConfig{Title: "工程师", RequiredSkills: []string{"Go", "PyTorch"}, RedLines: []string{"证书"}}, ResumeIDs: []string{"resume_skills"}}
	resume := &Resume{ID: "resume_skills", ProjectID: project.ID, Status: "done", Content: "负责 Go 服务开发。持有证书。", Analysis: &AnalysisResult{}, CreatedAt: time.Now()}
	writeLegacyJSON(t, app.getDataDir(), "projects", project.ID, project)
	writeLegacyJSON(t, app.getDataDir(), "resumes", resume.ID, resume)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resume.ContactStatus = "greeted"
		resume.Analysis.RedLineChecks = []RedLineCheck{{RuleID: redLineRuleID("证书"), Criterion: "证书", Status: "met", Evidence: "持有证书", ReviewedAt: time.Now().Format(time.RFC3339)}}
		encoded, _ := json.Marshal(resume)
		if err := os.WriteFile(filepath.Join(app.getDataDir(), "resumes", resume.ID+".json"), encoded, 0600); err != nil {
			t.Error(err)
		}
		answer := `{"core_criterion_checks":[{"criterion":"Go","level":2,"evidence":"负责 Go 服务开发。"},{"criterion":"PyTorch","level":4,"evidence":"不存在的PyTorch项目"}],"experience_competencies":[{"criterion":"服务开发","level":2,"evidence":"负责 Go 服务开发。","assessment":"有参与经历"}]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}})
	}))
	defer server.Close()
	app.config.AI = AIConfig{BaseURL: server.URL, APIKey: "synthetic", Model: "test-model"}
	got, err := app.SupplementResumeCompetencies(resume.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContactStatus != "greeted" || got.Analysis.RedLineChecks[0].Status != "met" {
		t.Fatal("contact and HR review must survive the model request")
	}
	if len(got.Analysis.ExperienceCompetencies) != 1 || got.Analysis.CoreCriterionChecks[1].Level != 0 {
		t.Fatal("only grounded capabilities should be retained")
	}
}

func TestInitialAnalysisAutomaticallyCompletesMissingRadarWithoutChangingSummary(t *testing.T) {
	app := NewApp()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		answer := `{"core_criterion_checks":[{"criterion":"Go","level":2,"evidence":"负责 Go 服务开发。"}],"experience_competencies":[{"criterion":"服务开发","level":2,"evidence":"负责 Go 服务开发。"}]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}})
	}))
	defer server.Close()
	cfg := &AIConfig{BaseURL: server.URL, APIKey: "synthetic", Model: "test"}
	result := &AnalysisResult{Summary: "main evaluation", OverallScore: 80}
	app.completeInitialCompetencies(result, &Resume{Content: "负责 Go 服务开发。"}, &JobConfig{CoreCriteria: []string{"Go", "PyTorch"}}, cfg)
	if calls != 1 || len(result.CoreCriterionChecks) < 2 || result.CoreCriterionChecks[1].Level != 0 || len(result.ExperienceCompetencies) < 2 || result.CompetencyStatus != "completed" || result.Summary != "main evaluation" || result.OverallScore != 80 {
		t.Fatalf("initial enrichment: calls=%d result=%+v", calls, result)
	}
}

func TestOptionalRadarFailurePreservesMainEvaluationAndExistingRubric(t *testing.T) {
	app := NewApp()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"message":"insufficient balance"}}`))
	}))
	defer server.Close()
	result := &AnalysisResult{Summary: "retained", SkillMatch: 80, ExperienceMatch: 70}
	app.completeInitialCompetencies(result, &Resume{Content: "synthetic"}, &JobConfig{RequiredSkills: []string{"Go"}}, &AIConfig{BaseURL: server.URL, APIKey: "synthetic", Model: "test"})
	if result.CompetencyStatus != "completed" || result.Summary != "retained" || result.SkillMatch != 80 || len(result.CoreCriterionChecks) == 0 {
		t.Fatalf("optional failure destroyed main result: %+v", result)
	}
}
