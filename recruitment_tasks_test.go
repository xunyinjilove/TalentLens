package main

import (
	"testing"
	"time"
)

func TestSourcingTaskPersistsProgressAndInterruptsOnRestart(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	project := &Project{ID: "proj_123", Name: "研发岗位", JobConfig: JobConfig{Title: "研发工程师"}, CreatedAt: now, UpdatedAt: now}
	writeLegacyJSON(t, dir, "projects", project.ID, project)
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	id, err := app.createSourcingTask(SourcingTaskConfig{ProjectID: project.ID, Keyword: "研发工程师", Count: 10, Platforms: []string{"boss", "51job"}})
	if err != nil {
		t.Fatal(err)
	}
	app.setSourcingTaskStatus(id, "running", "正在抓取")
	app.recordSourcingCandidate(id)
	if err := app.interruptUnfinishedSourcingTasks(); err != nil {
		t.Fatal(err)
	}
	tasks, err := app.ListSourcingTasks(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Status != "interrupted" || tasks[0].CapturedCount != 1 || tasks[0].TargetCount != 10 {
		t.Fatalf("unexpected restored task: %+v", tasks)
	}
	if tasks[0].Config.Keyword != "研发工程师" || len(tasks[0].Config.Platforms) != 2 {
		t.Fatalf("task configuration lost: %+v", tasks[0].Config)
	}
}
