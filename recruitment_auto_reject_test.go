package main

import (
	"testing"
)

// Missing evidence must never cause a platform operation, even when an old
// review row says unfit. The function returns before opening a browser.
func TestManagementAutoRejectRequiresExplicitEvidence(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试职位", "", 1, "", "", &JobConfig{Title: "测试职位"})
	if project == nil {
		t.Fatal("create project failed")
	}
	const scope, job = "co:test:hr:test", "job-test"
	if err := app.SetPlatformJobMapping("51job", scope, job, project.ID); err != nil {
		t.Fatal(err)
	}
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO talent_management_pending_reviews
		(project_id,platform_account,platform_job_id,candidate_id,record_id,candidate_name,platform_status,verdict,reasons_json,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, project.ID, scope, job, "candidate", "record", "测试候选人", "待定", "unfit",
		`[{"rule":"MIN_EDUCATION","status":"unknown","evidence":"学历未标明"}]`, "2026-11-03")
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.ProcessTalentManagementUnfitBacklog(scope, job)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 || result.Rejected != 0 {
		t.Fatalf("unexpected auto rejection: %+v", result)
	}
}
