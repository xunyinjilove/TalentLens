package main

import "testing"

func TestArchivedTalentCandidateDisappearsFromProjectAssessment(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "研发", 1, "", "", nil)
	if ok, message := app.RegisterResumeToProject(project.ID, "resume_one", "候选人.txt", "", "txt", 10); !ok {
		t.Fatal(message)
	}
	if got := len(app.GetProjectResumes(project.ID)); got != 1 {
		t.Fatalf("archive 前测评列表 = %d，期望 1", got)
	}
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	var linkID string
	if err := db.QueryRow(`SELECT pc.id FROM project_candidates pc
		JOIN project_candidate_resumes pcr ON pcr.project_candidate_id=pc.id
		JOIN resume_versions rv ON rv.id=pcr.resume_version_id
		WHERE pc.project_id=? AND rv.legacy_resume_id=?`, project.ID, "resume_one").Scan(&linkID); err != nil {
		t.Fatal(err)
	}
	if count, err := app.ArchiveTalentCandidates([]string{linkID}); err != nil || count != 1 {
		t.Fatalf("archive: count=%d err=%v", count, err)
	}
	if got := len(app.GetProjectResumes(project.ID)); got != 0 {
		t.Fatalf("archive 后测评列表 = %d，期望 0", got)
	}
}
