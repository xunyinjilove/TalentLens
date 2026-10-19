package main

import (
	"testing"
	"time"
)

func TestSameNameStaysSeparateUntilVerifiedContact(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	for _, item := range []struct{ project, resume string }{{"proj_a", "resume_a"}, {"proj_b", "resume_b"}} {
		writeLegacyJSON(t, dir, "projects", item.project, &Project{ID: item.project, Name: item.project, JobConfig: JobConfig{Title: "工程师"}, ResumeIDs: []string{item.resume}, CreatedAt: now, UpdatedAt: now})
		writeLegacyJSON(t, dir, "resumes", item.resume, &Resume{ID: item.resume, ProjectID: item.project, FileName: "同名候选人", Status: "pending", CreatedAt: now})
	}
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	var candidates int
	if err := db.QueryRow("SELECT COUNT(*) FROM candidates").Scan(&candidates); err != nil {
		t.Fatal(err)
	}
	if candidates != 2 {
		t.Fatalf("same name was automatically merged: %d", candidates)
	}
	hints, err := app.ListSuspectedDuplicates()
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) != 1 {
		t.Fatalf("expected manual duplicate hint: %+v", hints)
	}
	if _, err := app.VerifyCandidateContact(hints[0].CandidateID, "email", "person@example.com", "HR"); err != nil {
		t.Fatal(err)
	}
	primary, err := app.VerifyCandidateContact(hints[0].OtherCandidateID, "email", "person@example.com", "HR")
	if err != nil {
		t.Fatal(err)
	}
	if primary != hints[0].CandidateID {
		t.Fatalf("unexpected primary %s", primary)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM candidates").Scan(&candidates); err != nil {
		t.Fatal(err)
	}
	if candidates != 1 {
		t.Fatalf("verified identity did not merge: %d", candidates)
	}
	var links int
	if err := db.QueryRow("SELECT COUNT(*) FROM project_candidates WHERE candidate_id=?", primary).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 2 {
		t.Fatalf("project links lost: %d", links)
	}
}

func TestDismissAndClearDuplicateHintsPreserveCandidates(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	db, err := a.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, name string }{{"a", "同名甲"}, {"b", "同名甲"}, {"c", "同名乙"}, {"d", "同名乙"}} {
		if _, err := db.Exec("INSERT INTO candidates(id,display_name,created_at,updated_at) VALUES(?,?,?,?)", item.id, item.name, "now", "now"); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.DismissSuspectedDuplicate("a", "b"); err != nil {
		t.Fatal(err)
	}
	hints, err := a.ListSuspectedDuplicates()
	if err != nil || len(hints) != 1 {
		t.Fatalf("one hint should remain: %+v %v", hints, err)
	}
	removed, err := a.ClearSuspectedDuplicates()
	if err != nil || removed != 1 {
		t.Fatalf("clear current hints: %d %v", removed, err)
	}
	hints, err = a.ListSuspectedDuplicates()
	if err != nil || len(hints) != 0 {
		t.Fatalf("hints should be empty: %+v %v", hints, err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM candidates").Scan(&count); err != nil || count != 4 {
		t.Fatalf("candidate records changed: %d %v", count, err)
	}
}
