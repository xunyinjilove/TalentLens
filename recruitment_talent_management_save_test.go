package main

import (
	"testing"
	"time"
)

func TestSaveStrictCriteriaReturnsWithArchivedCandidate(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", nil)
	resume := &Resume{ID: "strict_save_candidate", ProjectID: project.ID, FileName: "测试简历.txt",
		Content:  `{"base_info":{"top_degree_value":"本科","work_year_value":"5年"}}`,
		FileType: ".txt", Status: "pending", CreatedAt: time.Now()}
	if err := app.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE project_candidates SET archived_at='2026-01-01',stage='rejected' WHERE project_id=?`, project.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- app.SetTalentManagementStrictCriteria(TalentManagementStrictCriteria{ProjectID: project.ID, MinEducation: "大专"})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("保存岗位硬性条件在归档候选人恢复时卡住")
	}
	var archived string
	if err := db.QueryRow(`SELECT archived_at FROM project_candidates WHERE project_id=?`, project.ID).Scan(&archived); err != nil {
		t.Fatal(err)
	}
	if archived != "" {
		t.Fatalf("预期恢复符合新条件的候选人，archived_at=%q", archived)
	}
}
