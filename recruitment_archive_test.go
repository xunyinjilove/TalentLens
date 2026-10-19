package main

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveColumnMigratesExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "recruitment.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE project_candidates (
		id TEXT PRIMARY KEY, project_id TEXT NOT NULL, candidate_id TEXT NOT NULL,
		stage TEXT NOT NULL DEFAULT 'new', owner TEXT NOT NULL DEFAULT '',
		next_action_at TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL, UNIQUE(project_id,candidate_id))`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	db, err = app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	var archiveColumn string
	if err := db.QueryRow("SELECT archived_at FROM project_candidates LIMIT 1").Scan(&archiveColumn); err != sql.ErrNoRows {
		t.Fatalf("archive column missing after upgrade: %v", err)
	}
}

func TestArchiveTalentCandidatesKeepsProfileAndTimeline(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	project := &Project{ID: "proj_archive", Name: "Synthetic role", JobConfig: JobConfig{Title: "Synthetic role"}, ResumeIDs: []string{"resume_archive"}, CreatedAt: now, UpdatedAt: now}
	resume := &Resume{ID: "resume_archive", ProjectID: project.ID, FileName: "synthetic.txt", Content: "synthetic resume", CreatedAt: now}
	writeLegacyJSON(t, dir, "projects", project.ID, project)
	writeLegacyJSON(t, dir, "resumes", resume.ID, resume)
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	items, err := app.ListTalentCandidates(project.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("initial pool: %d, %v", len(items), err)
	}
	removed, err := app.ArchiveTalentCandidates([]string{items[0].ProjectCandidateID})
	if err != nil || removed != 1 {
		t.Fatalf("archive: %d, %v", removed, err)
	}
	itemsAfter, err := app.ListTalentCandidates(project.ID)
	if err != nil || len(itemsAfter) != 0 {
		t.Fatalf("archived resume remains in pool: %d, %v", len(itemsAfter), err)
	}
	profile, err := app.GetCandidateProfile(items[0].ID)
	if err != nil || len(profile.Resumes) != 1 || len(profile.Timeline) != 1 {
		t.Fatalf("profile history lost: resumes=%d timeline=%d err=%v", len(profile.Resumes), len(profile.Timeline), err)
	}
	fromAssessment, err := app.GetCandidateProfileByResume(resume.ID)
	if err != nil || fromAssessment.ID != items[0].ID || len(fromAssessment.Timeline) != 1 {
		t.Fatalf("assessment must retain unified history: profile=%+v err=%v", fromAssessment, err)
	}
	if _, err := app.ArchiveTalentCandidates([]string{items[0].ProjectCandidateID}); err == nil {
		t.Fatal("archiving an already archived link must be rejected")
	}
}
