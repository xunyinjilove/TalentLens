package main

import (
	"testing"
)

func TestBatchJobSuggestionPrefersMappedIDAndNeverGuessesAmbiguousTitle(t *testing.T) {
	jobs := []JobCommunicationOption{{ID: "a", Title: "工程师"}, {ID: "b", Title: "工程师"}}
	batch := OutreachBatch{ProjectTitle: "工程师"}
	if suggestedOutreachJob(batch, jobs) != "" {
		t.Fatal("ambiguous titles must require HR selection")
	}
	batch.Items = []OutreachItem{{Platform: "51job", PlatformJobID: "b"}}
	if suggestedOutreachJob(batch, jobs) != "b" {
		t.Fatal("verified mapped ID should be preselected")
	}
	batch.Items = nil
	if suggestedOutreachJob(batch, jobs[:1]) != "a" {
		t.Fatal("unique exact title should be preselected")
	}
}

func TestOutreachBudgetRequiresExactJobAndBothCeilings(t *testing.T) {
	auth := &OutreachAuthorization{JobID: "job", JobTitle: "工程师", PerPersonLimit: 5, TotalLimit: 12, ReservedPoints: 8, Status: "active"}
	quote := pending51JobHiQuote{JobID: "job", JobTitle: "工程师", Points: 4}
	if err := checkOutreachBudget(auth, quote); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []pending51JobHiQuote{{JobID: "other", JobTitle: "工程师", Points: 1}, {JobID: "job", JobTitle: "不同职位", Points: 1}, {JobID: "job", JobTitle: "工程师", Points: 0}, {JobID: "job", JobTitle: "工程师", Points: 5}, {JobID: "job", JobTitle: "工程师", Points: 6}} {
		if checkOutreachBudget(auth, invalid) == nil {
			t.Fatalf("unauthorized quote accepted: %+v", invalid)
		}
	}
	auth.Status = "paused"
	if checkOutreachBudget(auth, quote) == nil {
		t.Fatal("paused authorization must not send")
	}
}

func TestBudgetReservationIsDurableAndCannotReplayAnItem(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	p := a.CreateProject("Synthetic project", "Test", 1, "HR", "", &JobConfig{Title: "工程师"})
	r := &Resume{ID: "budget_resume", ProjectID: p.ID, FileName: "synthetic.txt", Content: "synthetic experience", Status: "done", Analysis: &AnalysisResult{CandidateName: "Synthetic", OverallScore: 80}}
	if err := a.persistSearchCandidate(r); err != nil {
		t.Fatal(err)
	}
	batch, err := a.PreviewTopTenOutreach(p.ID)
	if err != nil || len(batch.Items) != 1 {
		t.Fatalf("batch: %+v %v", batch, err)
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO outreach_authorizations(batch_id,job_id,job_title,per_person_limit,total_limit,status,confirmed_at) VALUES(?,'job','工程师',5,20,'active','now')`, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	item := batch.Items[0]
	item.Status = "awaiting_confirmation"
	item.PlatformJobID = "job"
	if _, err = a.setOutreachItemStatus(item, item.Status, ""); err != nil {
		t.Fatal(err)
	}
	quote := pending51JobHiQuote{JobID: "job", JobTitle: "工程师", Points: 5}
	got, err := a.reserveOutreachPoints(batch.ID, item, quote)
	if err != nil || got.Status != "sending" {
		t.Fatalf("reserve: %+v %v", got, err)
	}
	if _, err = a.reserveOutreachPoints(batch.ID, item, quote); err == nil {
		t.Fatal("second reservation must fail")
	}
	auth, err := a.loadOutreachAuthorization(batch.ID)
	if err != nil || auth.ReservedPoints != 5 {
		t.Fatalf("budget spent twice: %+v %v", auth, err)
	}
	stored, err := a.loadOutreachItem(item.ID)
	if err != nil || stored.ReservedPoints != 5 || stored.Status != "sending" {
		t.Fatalf("lost in-flight marker: %+v %v", stored, err)
	}
	if err = a.RecordOutreachResult(item.ID, "failed", "verified platform receipt", "HR"); err != nil {
		t.Fatal(err)
	}
	auth, _ = a.loadOutreachAuthorization(batch.ID)
	if auth.ReservedPoints != 5 {
		t.Fatal("unknown cost must remain reserved after reconciliation")
	}
}

func TestVerifiedPlatformIDContactStateDoesNotSpreadByName(t *testing.T) {
	first := &Resume{URL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123", ContactStatus: "greeted", ContactUpdatedAt: "2026-10-16T10:00:00+08:00"}
	copy := &Resume{URL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123&source=other"}
	other := &Resume{URL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=124"}
	merge51JobContactStates([]*Resume{first, copy, other})
	if copy.ContactStatus != "greeted" || other.ContactStatus != "" {
		t.Fatal("only exact platform identity may share contact state")
	}
}
