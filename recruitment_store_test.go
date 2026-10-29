package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeLegacyJSON(t *testing.T, dir, group, name string, value interface{}) string {
	t.Helper()
	path := filepath.Join(dir, group, name+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRecruitmentMigrationPreservesLegacyData(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	project := &Project{ID: "proj_123", Name: "临床项目经理", JobConfig: JobConfig{Title: "临床项目经理", RedLines: []string{"可核实的项目管理经历"}}, ResumeIDs: []string{"resume_1"}, CreatedAt: now, UpdatedAt: now}
	resume := &Resume{ID: "resume_1", ProjectID: project.ID, FileName: "候选人甲.pdf", Content: "临床项目管理五年", Status: "done", Analysis: &AnalysisResult{CandidateName: "候选人甲", AbilityScore: 80, RedLineStatus: "pending"}, CreatedAt: now}
	projectPath := writeLegacyJSON(t, dir, "projects", project.ID, project)
	resumePath := writeLegacyJSON(t, dir, "resumes", resume.ID, resume)
	projectOriginal, _ := os.ReadFile(projectPath)
	resumeOriginal, _ := os.ReadFile(resumePath)
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	var projectCount, candidateCount, linkCount, evalCount int
	for _, check := range []struct {
		query string
		value *int
	}{
		{"SELECT COUNT(*) FROM projects", &projectCount},
		{"SELECT COUNT(*) FROM candidates", &candidateCount},
		{"SELECT COUNT(*) FROM project_candidates", &linkCount},
		{"SELECT COUNT(*) FROM evaluations", &evalCount},
	} {
		if err := db.QueryRow(check.query).Scan(check.value); err != nil {
			t.Fatal(err)
		}
	}
	if projectCount != 1 || candidateCount != 1 || linkCount != 1 || evalCount != 1 {
		t.Fatalf("migration counts: projects=%d candidates=%d links=%d evaluations=%d", projectCount, candidateCount, linkCount, evalCount)
	}
	var status string
	if err := db.QueryRow("SELECT redline_status FROM evaluations LIMIT 1").Scan(&status); err != nil || status != "pending" {
		t.Fatalf("redline status: %q, %v", status, err)
	}
	if err := db.QueryRow("SELECT status FROM job_rules LIMIT 1").Scan(&status); err != nil || status != "source_unconfirmed" {
		t.Fatalf("legacy rule provenance: %q, %v", status, err)
	}
	if err := migrateLegacyRecruitmentData(db, dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(projectPath); string(got) != string(projectOriginal) {
		t.Fatal("project JSON changed during migration")
	}
	if got, _ := os.ReadFile(resumePath); string(got) != string(resumeOriginal) {
		t.Fatal("resume JSON changed during migration")
	}
	backups, err := filepath.Glob(filepath.Join(dir, "legacy-backup-*", "resumes", "resume_1.json"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("expected one resume backup, got %v, %v", backups, err)
	}
}

func TestRecruitmentReopenRepairsProjectCatalogAndAnalysisStage(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	if _, err := app.getRecruitmentDB(); err != nil {
		t.Fatal(err)
	}
	app.closeRecruitmentDB()
	project := &Project{ID: "proj_added_after_migration", Name: "测试职位", JobConfig: JobConfig{Title: "测试职位"}, CreatedAt: now, UpdatedAt: now}
	writeLegacyJSON(t, dir, "projects", project.ID, project)
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM projects WHERE id=?`, project.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("missing project was not repaired: count=%d err=%v", count, err)
	}
	if _, err := db.Exec(`INSERT INTO candidates(id,display_name,created_at,updated_at) VALUES(?,?,?,?)`, "cand_stage", "测试人选", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO resume_versions(id,candidate_id,content_sha256,created_at) VALUES(?,?,?,?)`, "rv_stage", "cand_stage", "stage-test-hash", now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO project_candidates(id,project_id,candidate_id,stage,created_at,updated_at) VALUES(?,?,?,?,?,?)`, "pc_stage", project.ID, "cand_stage", "pending_analysis", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO evaluations(id,project_candidate_id,resume_version_id,status,created_at) VALUES(?,?,?,?,?)`, "ev_stage", "pc_stage", "rv_stage", "done", now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	app.closeRecruitmentDB()
	db, err = app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	var stage string
	if err := db.QueryRow(`SELECT stage FROM project_candidates WHERE id='pc_stage'`).Scan(&stage); err != nil || stage != "pending_contact" {
		t.Fatalf("completed analysis stage = %q, err=%v", stage, err)
	}
}

func TestRecruitmentMigrationDoesNotSilentlyDropInvalidLegacyRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects", "broken.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	if _, err := app.getRecruitmentDB(); err == nil {
		t.Fatal("invalid legacy record must stop migration")
	}
	if got, _ := os.ReadFile(path); string(got) != "{not json" {
		t.Fatal("invalid source file changed")
	}
}

func TestVerifiedPlatformIdentityLinksOneCandidateAcrossProjects(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	url := "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123456"
	for _, id := range []string{"proj_a", "proj_b"} {
		resumeID := "resume_" + id
		writeLegacyJSON(t, dir, "projects", id, &Project{ID: id, Name: id, JobConfig: JobConfig{Title: "测试岗位"}, ResumeIDs: []string{resumeID}, CreatedAt: now, UpdatedAt: now})
		score := 70
		if id == "proj_b" {
			score = 90
		}
		writeLegacyJSON(t, dir, "resumes", resumeID, &Resume{ID: resumeID, ProjectID: id, FileName: "同一候选人", URL: url, SourcePlatform: "51job", PlatformCandidateID: "123456", Status: "done", Analysis: &AnalysisResult{CandidateName: "同一候选人", AbilityScore: score, RedLineStatus: "pending"}, CreatedAt: now})
	}
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	var candidates, sources, links int
	for _, check := range []struct {
		q      string
		target *int
	}{{"SELECT COUNT(*) FROM candidates", &candidates}, {"SELECT COUNT(*) FROM candidate_sources", &sources}, {"SELECT COUNT(*) FROM project_candidates", &links}} {
		if err := db.QueryRow(check.q).Scan(check.target); err != nil {
			t.Fatal(err)
		}
	}
	if candidates != 1 || sources != 1 || links != 2 {
		t.Fatalf("identity migration: candidates=%d sources=%d links=%d", candidates, sources, links)
	}
	view, err := app.ListTalentCandidates("")
	if err != nil {
		t.Fatal(err)
	}
	if len(view) != 2 || view[0].ID != view[1].ID || view[0].AbilityScore == view[1].AbilityScore {
		t.Fatalf("project evaluations must remain separate: %+v", view)
	}
	profile, err := app.GetCandidateProfile(view[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.Projects) != 2 || len(profile.Resumes) != 2 || len(profile.Sources) != 1 {
		t.Fatalf("unified profile lost project or source history: %+v", profile)
	}
}

func TestBatchUpdateCandidateStage(t *testing.T) {
	dir := t.TempDir()
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()

	now := time.Now()
	writeLegacyJSON(t, dir, "projects", "proj_batch", &Project{
		ID:        "proj_batch",
		Name:      "批量测试岗位",
		JobConfig: JobConfig{Title: "批量测试岗位"},
		ResumeIDs: []string{"res_1", "res_2"},
		CreatedAt: now,
		UpdatedAt: now,
	})
	writeLegacyJSON(t, dir, "resumes", "res_1", &Resume{
		ID:        "res_1",
		ProjectID: "proj_batch",
		FileName:  "候选人甲.pdf",
		Status:    "done",
		Analysis:  &AnalysisResult{CandidateName: "候选人甲", OverallScore: 85},
		CreatedAt: now,
	})
	writeLegacyJSON(t, dir, "resumes", "res_2", &Resume{
		ID:        "res_2",
		ProjectID: "proj_batch",
		FileName:  "候选人乙.pdf",
		Status:    "done",
		Analysis:  &AnalysisResult{CandidateName: "候选人乙", OverallScore: 80},
		CreatedAt: now,
	})

	list, err := app.ListTalentCandidates("proj_batch")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(list))
	}

	pcIDs := []string{list[0].ProjectCandidateID, list[1].ProjectCandidateID}
	if err := app.BatchUpdateCandidateStage(pcIDs, "first_interview", "批量进入初试", "HR"); err != nil {
		t.Fatalf("BatchUpdateCandidateStage failed: %v", err)
	}

	updatedList, err := app.ListTalentCandidates("proj_batch")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range updatedList {
		if c.Stage != "first_interview" {
			t.Fatalf("expected stage 'first_interview', got '%s' for candidate %s", c.Stage, c.Name)
		}
	}
}
