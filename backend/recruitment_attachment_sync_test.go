package backend

import (
	"context"
	"os"
	"testing"
	"time"
)

func attachmentSyncFixture(t *testing.T) (*App, string) {
	t.Helper()
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	project := app.CreateProject("附件测试岗位", "", 1, "", "", nil)
	resume := &Resume{ID: "attachment_sync_123", ProjectID: project.ID, FileName: "张先生.txt",
		SourcePlatform: "51job", PlatformCandidateID: "123", Content: "在线简历",
		URL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123", CreatedAt: time.Now()}
	if err := app.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO platform_job_mappings(platform,platform_account,platform_job_id,project_id) VALUES('51job','account-test','job-test',?)`, project.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.closeRecruitmentDB)
	return app, project.ID
}

func waitAttachmentTask(t *testing.T, app *App, projectID, state string) *AttachmentSyncTask {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		task, err := app.Get51JobAttachmentSyncTask(projectID)
		if err != nil {
			t.Fatal(err)
		}
		app.attachmentSyncMu.Lock()
		active := app.attachmentSyncCancel[projectID] != nil
		app.attachmentSyncMu.Unlock()
		if task != nil && task.State == state && !active {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("attachment task did not reach %s", state)
	return nil
}

func TestAttachmentSyncChecksChatEvenWhenPlatformDidNotFlagAttachment(t *testing.T) {
	app, projectID := attachmentSyncFixture(t)
	app.attachmentSyncProbe = func(_ context.Context, id string) JobConversationResult {
		if id != "attachment_sync_123" {
			t.Errorf("wrong candidate %s", id)
		}
		return JobConversationResult{Status: "waiting"}
	}
	started, err := app.Start51JobAttachmentSync(projectID)
	if err != nil || len(started.Queue) != 1 {
		t.Fatalf("start=%+v err=%v", started, err)
	}
	finished := waitAttachmentTask(t, app, projectID, "completed")
	if finished.Checked != 1 || finished.Skipped != 1 || finished.Failed != 0 || finished.Downloaded != 0 {
		t.Fatalf("no-attachment chat was misclassified: %+v", finished)
	}
	// A fresh sync checks it again, because the candidate may send a file later.
	restarted, err := app.Start51JobAttachmentSync(projectID)
	if err != nil || len(restarted.Queue) != 1 {
		t.Fatalf("restart=%+v err=%v", restarted, err)
	}
	waitAttachmentTask(t, app, projectID, "completed")
}

func TestAttachmentSyncStopResumesUnfinishedCandidate(t *testing.T) {
	app, projectID := attachmentSyncFixture(t)
	entered := make(chan struct{})
	app.attachmentSyncProbe = func(ctx context.Context, _ string) JobConversationResult {
		close(entered)
		<-ctx.Done()
		return JobConversationResult{Status: "interrupted"}
	}
	if _, err := app.Start51JobAttachmentSync(projectID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(4 * time.Second):
		t.Fatal("attachment check did not start")
	}
	if err := app.Stop51JobAttachmentSync(projectID); err != nil {
		t.Fatal(err)
	}
	paused := waitAttachmentTask(t, app, projectID, "paused")
	if paused.Checked != 0 {
		t.Fatalf("interrupted candidate was consumed: %+v", paused)
	}
	app.attachmentSyncProbe = func(_ context.Context, _ string) JobConversationResult {
		return JobConversationResult{Status: "waiting"}
	}
	if _, err := app.Start51JobAttachmentSync(projectID); err != nil {
		t.Fatal(err)
	}
	finished := waitAttachmentTask(t, app, projectID, "completed")
	if finished.Checked != 1 || finished.Skipped != 1 {
		t.Fatalf("resume did not continue unfinished candidate: %+v", finished)
	}
}

// Opt-in acceptance test against an already logged-in 51job browser. It uses
// an isolated data directory and never sends a platform message.
func TestLive51JobAttachmentBackgroundSync(t *testing.T) {
	url := os.Getenv("TALENTLENS_LIVE_URL")
	name := os.Getenv("TALENTLENS_LIVE_NAME")
	id := os.Getenv("TALENTLENS_LIVE_CANDIDATE_ID")
	if url == "" || name == "" || id == "" {
		t.Skip("requires an explicit live 51job fixture")
	}
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("隔离附件读取测试", "", 1, "", "", nil)
	resume := &Resume{ID: "live_attachment_probe", ProjectID: project.ID, FileName: name + ".txt",
		SourcePlatform: "51job", PlatformCandidateID: id, Content: "在线简历快照", URL: url, CreatedAt: time.Now()}
	if err := app.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO platform_job_mappings(platform,platform_account,platform_job_id,project_id) VALUES('51job','isolated-test','isolated-job',?)`, project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Start51JobAttachmentSync(project.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		task, err := app.Get51JobAttachmentSyncTask(project.ID)
		if err != nil {
			t.Fatal(err)
		}
		if task != nil && task.State == "completed" {
			if task.Downloaded != 1 || task.Checked != 1 {
				t.Fatalf("attachment not imported: %+v", task)
			}
			stored, err := app.load51JobContactResume(resume.ID)
			if err != nil || stored == nil || !stored.HasAttachment || len(stored.AttachmentContent) < 100 {
				t.Fatalf("saved attachment missing: err=%v", err)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("attachment background sync timed out")
}
