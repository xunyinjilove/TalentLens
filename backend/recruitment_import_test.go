package backend

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

func TestDeleted51JobResumeCanBeImportedAgain(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", &JobConfig{Title: "测试岗位"})
	if project == nil {
		t.Fatal("create project failed")
	}
	const scope = "co:1:hr:2"
	if err := app.SetPlatformJobMapping("51job", scope, "job-1", project.ID); err != nil {
		t.Fatal(err)
	}
	complete := true
	record := ApplicationRecord{Platform: "51job", PlatformAccount: scope, ApplicationID: "apply-1",
		PlatformCandidateID: "candidate-1", PlatformJobID: "job-1", CandidateName: "测试人选",
		ResumeText: "真实的完整简历内容", DocumentsComplete: &complete}
	first, err := app.ingestApplicationRecords("51job", []ApplicationRecord{record})
	if err != nil || first.AddedCount != 1 {
		t.Fatalf("first import: %+v, %v", first, err)
	}
	if err := app.DeleteResume(applicationResumeID(record)); err != nil {
		t.Fatal(err)
	}
	if got := len(app.GetProjectResumes(project.ID)); got != 0 {
		t.Fatalf("resumes after delete = %d", got)
	}
	second, err := app.ingestApplicationRecords("51job", []ApplicationRecord{record})
	if err != nil || second.AddedCount != 1 {
		t.Fatalf("reimport: %+v, %v", second, err)
	}
	if got := len(app.GetProjectResumes(project.ID)); got != 1 {
		t.Fatalf("resumes after reimport = %d", got)
	}
}

func TestRepairMisfiledProjectCandidateUsesExactPlatformJobID(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	ningbo := app.CreateProject("区域销售经理 宁波", "", 1, "", "", &JobConfig{Title: "区域销售经理", City: "宁波"})
	chengdu := app.CreateProject("区域销售经理 成都", "", 1, "", "", &JobConfig{Title: "区域销售经理", City: "成都"})
	if ningbo == nil || chengdu == nil {
		t.Fatal("create projects")
	}
	const account = "co:1:hr:2"
	if err := app.SetPlatformJobMapping("51job", account, "ningbo-id", ningbo.ID); err != nil {
		t.Fatal(err)
	}
	foreign := ApplicationRecord{Platform: "51job", PlatformAccount: account, ApplicationID: "apply-chengdu", PlatformCandidateID: "candidate-chengdu", PlatformJobID: "chengdu-id", CandidateName: "成都候选人", ResumeText: "成都区域销售经历"}
	correct := ApplicationRecord{Platform: "51job", PlatformAccount: account, ApplicationID: "apply-ningbo", PlatformCandidateID: "candidate-ningbo", PlatformJobID: "ningbo-id", CandidateName: "宁波候选人", ResumeText: "宁波区域销售经历"}
	// Reproduce the legacy error: an association lives in Ningbo while its
	// application row points to another project.
	for _, record := range []ApplicationRecord{foreign, correct} {
		if err := app.createApplicationResume(record, ningbo.ID); err != nil {
			t.Fatal(err)
		}
	}
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []ApplicationRecord{foreign, correct} {
		projectID := ningbo.ID
		if record.PlatformJobID == "chengdu-id" {
			projectID = chengdu.ID
		}
		_, err := db.Exec(`INSERT INTO applications(id,platform,platform_account,platform_application_id,platform_job_id,project_id,status,applied_at,raw_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			recruitmentID("application"), "51job", account, record.ApplicationID, record.PlatformJobID, projectID, "assigned", "", "{}", time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			t.Fatal(err)
		}
	}
	archived, err := repairMisfiled51JobProjectCandidates(db, account, "ningbo-id", ningbo.ID)
	if err != nil || archived != 1 {
		t.Fatalf("archived=%d err=%v", archived, err)
	}
	remaining := app.GetProjectResumes(ningbo.ID)
	if len(remaining) != 1 || remaining[0].PlatformCandidateID != "candidate-ningbo" {
		t.Fatalf("remaining=%+v", remaining)
	}
}

func TestExclusive51JobBindingArchivesOtherJobResumes(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("区域销售经理兰州", "", 1, "", "", &JobConfig{Title: "区域销售经理兰州"})
	const scope = "co:1:hr:2"
	for _, job := range []string{"regional", "international"} {
		if err := app.SetPlatformJobMapping("51job", scope, job, project.ID); err != nil {
			t.Fatal(err)
		}
	}
	complete := true
	for _, job := range []string{"regional", "international"} {
		record := ApplicationRecord{Platform: "51job", PlatformAccount: scope, PlatformJobID: job,
			ApplicationID: "apply-" + job, PlatformCandidateID: "candidate-" + job,
			CandidateName: job, ResumeText: `{"topdegree":"本科"}`, DocumentsComplete: &complete}
		if result, err := app.ingestApplicationRecords("51job", []ApplicationRecord{record}); err != nil || result.AddedCount != 1 {
			t.Fatalf("import %s: %+v %v", job, result, err)
		}
	}
	if got := len(app.GetProjectResumes(project.ID)); got != 2 {
		t.Fatalf("before rebind: %d", got)
	}
	if err := app.bindExclusive51JobProject(scope, "regional", project.ID); err != nil {
		t.Fatal(err)
	}
	resumes := app.GetProjectResumes(project.ID)
	if len(resumes) != 1 || resumes[0].PlatformCandidateID != "candidate-regional" {
		t.Fatalf("wrong resumes after rebind: %+v", resumes)
	}
	foreign := ApplicationRecord{Platform: "51job", PlatformAccount: scope, PlatformJobID: "international",
		ApplicationID: "apply-international", PlatformCandidateID: "candidate-international", CandidateName: "international",
		ResumeText: `{"topdegree":"本科"}`, DocumentsComplete: &complete}
	if result, err := app.ingestApplicationRecords("51job", []ApplicationRecord{foreign}); err != nil || result.AddedCount != 0 {
		t.Fatalf("stale job reimported: %+v %v", result, err)
	}
	if got := len(app.GetProjectResumes(project.ID)); got != 1 {
		t.Fatalf("stale job returned to library: %d", got)
	}
}
