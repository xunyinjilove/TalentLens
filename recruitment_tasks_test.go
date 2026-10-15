package main

import (
	"os"
	"path/filepath"
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

func TestSourcingTaskRetainsPlatformShortfallReason(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", &JobConfig{})
	id, err := app.createSourcingTask(SourcingTaskConfig{ProjectID: project.ID, Keyword: "工程师", Count: 30})
	if err != nil {
		t.Fatal(err)
	}
	app.recordSourcingCandidate(id)
	if !app.finishSourcingTaskFromDone(id, map[string]interface{}{
		"total": float64(1), "shortfallReason": "51job：列表没有更多候选人",
	}, 1, 0, 30) {
		t.Fatal("matching engine and saved counts should finalize")
	}
	tasks, err := app.ListSourcingTasks(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Status != "partial" || tasks[0].Message != "未达目标：51job：列表没有更多候选人" {
		t.Fatalf("platform stop reason was lost: %+v", tasks)
	}
}

func TestSourcingTaskExplainsDeduplicatedShortfall(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", &JobConfig{})
	id, err := app.createSourcingTask(SourcingTaskConfig{ProjectID: project.ID, Keyword: "工程师", Count: 30})
	if err != nil {
		t.Fatal(err)
	}
	if !app.finishSourcingTaskFromDone(id, map[string]interface{}{"total": float64(30)}, 6, 24, 30) {
		t.Fatal("matching engine and saved counts should finalize")
	}
	tasks, err := app.ListSourcingTasks(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Status != "partial" || tasks[0].Message != "未达目标：平台返回了 24 条已入库记录；已保留本次 6 位新候选人，可手动继续检索" {
		t.Fatalf("duplicate shortfall reason was lost: %+v", tasks)
	}
}

func TestSourcingTaskDeletionKeepsCandidatesAndSkipsActiveTasks(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", &JobConfig{})
	app.saveResume(&Resume{ID: "resume_keep", ProjectID: project.ID, FileName: "保留简历", Content: "真实履历", Status: "done", Analysis: &AnalysisResult{AbilityScore: 86}})
	project.ResumeIDs = append(project.ResumeIDs, "resume_keep")
	app.saveProject(project)
	resumePath := filepath.Join(app.getDataDir(), "resumes", "resume_keep.json")
	resumeBefore, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatal(err)
	}
	makeTask := func(status string) string {
		t.Helper()
		id, err := app.createSourcingTask(SourcingTaskConfig{ProjectID: project.ID, Keyword: "工程师", Count: 10})
		if err != nil {
			t.Fatal(err)
		}
		app.setSourcingTaskStatus(id, status, status)
		return id
	}
	completed := makeTask("completed")
	running := makeTask("running")
	queued := makeTask("queued")
	makeTask("interrupted")
	if _, err := app.DeleteSourcingTask(running); err == nil {
		t.Fatal("active search history must not be deleted")
	}
	if _, err := app.DeleteSourcingTask(queued); err == nil {
		t.Fatal("queued search history must not be deleted")
	}
	if ok, err := app.DeleteSourcingTask(completed); err != nil || !ok {
		t.Fatalf("delete completed task: ok=%v err=%v", ok, err)
	}
	if removed, err := app.ClearFinishedSourcingTasks(); err != nil || removed != 1 {
		t.Fatalf("clear finished history: removed=%d err=%v", removed, err)
	}
	tasks, err := app.ListSourcingTasks(project.ID)
	if err != nil || len(tasks) != 2 || tasks[0].ID != queued || tasks[1].ID != running {
		t.Fatalf("active task was lost or history remains: %+v err=%v", tasks, err)
	}
	resumes := app.GetProjectResumes(project.ID)
	if len(resumes) != 1 || resumes[0].ID != "resume_keep" || resumes[0].Analysis == nil {
		t.Fatalf("deleting task history removed a candidate or its analysis: %+v", resumes)
	}
	resumeAfter, err := os.ReadFile(resumePath)
	if err != nil || string(resumeBefore) != string(resumeAfter) {
		t.Fatalf("candidate's saved analysis changed: %v", err)
	}
}
