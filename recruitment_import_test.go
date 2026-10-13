package main

import (
	"testing"
	"time"
)

func TestApplicationImportUsesPlatformJobIDAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	writeLegacyJSON(t, dir, "projects", "proj_a", &Project{ID: "proj_a", Name: "岗位 A", JobConfig: JobConfig{Title: "岗位 A"}, CreatedAt: now, UpdatedAt: now})
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	if err := app.SetPlatformJobMapping("51job", "", "job_1", "proj_a"); err != nil {
		t.Fatal(err)
	}
	records := []ApplicationRecord{
		{Platform: "51job", ApplicationID: "apply_1", PlatformCandidateID: "123456", PlatformJobID: "job_1", CandidateName: "候选人甲", ProfileURL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123456", ResumeText: "候选人甲拥有真实岗位经历，负责过项目计划、执行、沟通和交付。"},
		{Platform: "51job", ApplicationID: "apply_2", PlatformCandidateID: "567890", PlatformJobID: "unknown", CandidateName: "候选人乙", ProfileURL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=567890"},
	}
	first, err := app.ingestApplicationRecords("51job", records)
	if err != nil {
		t.Fatal(err)
	}
	if first.AddedCount != 2 || first.UnassignedCount != 1 || first.FailedCount != 0 {
		t.Fatalf("first import %+v", first)
	}
	second, err := app.ingestApplicationRecords("51job", records)
	if err != nil {
		t.Fatal(err)
	}
	if second.AddedCount != 0 || second.DuplicateCount != 2 {
		t.Fatalf("repeat import %+v", second)
	}
	assigned, err := app.ListImportedApplications("assigned")
	if err != nil {
		t.Fatal(err)
	}
	if len(assigned) != 1 || assigned[0].ProjectID != "proj_a" {
		t.Fatalf("assigned %+v", assigned)
	}
	unassigned, err := app.ListImportedApplications("unassigned")
	if err != nil {
		t.Fatal(err)
	}
	if len(unassigned) != 1 || unassigned[0].ProjectID != "" {
		t.Fatalf("unassigned %+v", unassigned)
	}
	if err := app.AssignApplication(unassigned[0].ID, "proj_a"); err != nil {
		t.Fatal(err)
	}
	if got := app.GetProjectResumes("proj_a"); len(got) != 2 {
		t.Fatalf("assigned resumes %d", len(got))
	}
}
