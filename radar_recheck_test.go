package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRadarRecheckRequiresPreviewAndHRActivation(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		answer := `{"core_criterion_checks":[{"criterion":"客户需求","level":3,"evidence":"负责客户需求方案谈判回款协作"},{"criterion":"方案","level":3,"evidence":"负责客户需求方案谈判回款协作"},{"criterion":"谈判","level":3,"evidence":"负责客户需求方案谈判回款协作"},{"criterion":"回款","level":3,"evidence":"负责客户需求方案谈判回款协作"},{"criterion":"协作","level":3,"evidence":"负责客户需求方案谈判回款协作"}],"experience_competencies":[]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}})
	}))
	defer server.Close()
	a.config.AI = AIConfig{BaseURL: server.URL, APIKey: "synthetic", Model: "test"}
	job := &JobConfig{Title: "大客户销售经理", RequiredSkills: []string{"客户需求", "方案", "谈判", "回款", "协作"}}
	job.RadarMetrics = append([]string(nil), job.RequiredSkills...)
	job.RadarSourceHash = jobRadarSourceHash(job)
	job.RadarProfileHash = confirmedJobRadarHash(job.RadarMetrics, job.RadarSourceHash)
	job.RadarProfileVersion = 1
	p := a.CreateProject("大客户销售经理", "销售", 1, "HR", "", job)
	resume := &Resume{ID: "radar_recheck_resume", ProjectID: p.ID, Status: "done", Score: 88, Content: "负责客户需求方案谈判回款协作", Analysis: &AnalysisResult{AbilityScore: 88, OverallScore: 88, CoreCriterionChecks: []CoreCriterionCheck{{Criterion: "旧三维", Level: 0}}}}
	a.saveResume(resume)
	p.ResumeIDs = []string{resume.ID}
	a.saveProject(p)
	preview, err := a.PreviewResumeAIRecheck(resume.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.NewRankEligible || preview.NewScore != 75 || !preview.Significant || len(preview.Changes) != 5 {
		t.Fatalf("unexpected draft: %+v", preview)
	}
	before, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resume.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var untouched Resume
	if err := json.Unmarshal(before, &untouched); err != nil {
		t.Fatal(err)
	}
	if untouched.Score != 88 {
		t.Fatal("preview must not overwrite active evaluation")
	}
	confirmed, err := a.ConfirmResumeAIRecheck(resume.ID, preview.DraftID)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Score != 75 || confirmed.Analysis.RadarProfileHash != job.RadarProfileHash {
		t.Fatalf("confirmation did not activate evidence score: %+v", confirmed)
	}
	if _, err := os.Stat(filepath.Join(a.getDataDir(), "radar_recheck_history", preview.DraftID+"-before.json")); err != nil {
		t.Fatal("old evaluation must be archived", err)
	}
}
