package backend

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AttachmentSyncTask is independent of the talent-management list scan. The
// persisted queue lets a stopped or interrupted run resume without rechecking
// candidates already visited in that run.
type AttachmentSyncTask struct {
	ProjectID       string   `json:"projectId"`
	PlatformAccount string   `json:"platformAccount"`
	PlatformJobID   string   `json:"platformJobId"`
	State           string   `json:"state"`
	Queue           []string `json:"queue"`
	Checked         int      `json:"checked"`
	Downloaded      int      `json:"downloaded"`
	Skipped         int      `json:"skipped"`
	Failed          int      `json:"failed"`
	CurrentName     string   `json:"currentName"`
	LastError       string   `json:"lastError"`
	UpdatedAt       string   `json:"updatedAt"`
}

func (a *App) readAttachmentSyncTask(db *sql.DB, projectID string) (*AttachmentSyncTask, error) {
	var raw string
	err := db.QueryRow(`SELECT task_json FROM attachment_sync_tasks WHERE project_id=?`, projectID).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var task AttachmentSyncTask
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		return nil, fmt.Errorf("附件同步进度无法读取: %w", err)
	}
	if task.ProjectID != projectID || task.Checked < 0 || task.Checked > len(task.Queue) {
		return nil, fmt.Errorf("附件同步进度与当前分库不一致")
	}
	return &task, nil
}

func (a *App) saveAttachmentSyncTask(db *sql.DB, task *AttachmentSyncTask) error {
	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(task)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO attachment_sync_tasks(project_id,task_json,updated_at) VALUES(?,?,?)
		ON CONFLICT(project_id) DO UPDATE SET task_json=excluded.task_json,updated_at=excluded.updated_at`,
		task.ProjectID, string(raw), task.UpdatedAt)
	if err == nil && a.ctx != nil {
		runtime.EventsEmit(a.ctx, "attachment_sync_progress", task)
	}
	return err
}

func (a *App) Get51JobAttachmentSyncTask(projectID string) (*AttachmentSyncTask, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	task, err := a.readAttachmentSyncTask(db, projectID)
	if err != nil || task == nil || task.State != "running" {
		return task, err
	}
	a.attachmentSyncMu.Lock()
	_, active := a.attachmentSyncCancel[projectID]
	a.attachmentSyncMu.Unlock()
	if !active {
		task.State = "paused" // the desktop process stopped during an earlier run
		task.CurrentName = ""
		if err := a.saveAttachmentSyncTask(db, task); err != nil {
			return nil, err
		}
	}
	return task, nil
}

func (a *App) Start51JobAttachmentSync(projectID string) (*AttachmentSyncTask, error) {
	if strings.TrimSpace(projectID) == "" || a.GetProject(projectID) == nil {
		return nil, fmt.Errorf("请选择有效的职位专属库")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT platform_account,platform_job_id FROM platform_job_mappings WHERE platform='51job' AND project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	var account, jobID string
	count := 0
	for rows.Next() {
		count++
		if err := rows.Scan(&account, &jobID); err != nil {
			rows.Close()
			return nil, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if count != 1 || account == "" || jobID == "" {
		return nil, fmt.Errorf("当前分库需要唯一且有效的 51job 账号与职位绑定")
	}

	a.attachmentSyncMu.Lock()
	defer a.attachmentSyncMu.Unlock()
	if a.attachmentSyncCancel == nil {
		a.attachmentSyncCancel = make(map[string]context.CancelFunc)
	}
	if a.attachmentSyncCancel[projectID] != nil {
		return a.readAttachmentSyncTask(db, projectID)
	}
	task, err := a.readAttachmentSyncTask(db, projectID)
	if err != nil {
		return nil, err
	}
	if task == nil || task.State == "completed" || task.PlatformAccount != account || task.PlatformJobID != jobID || task.Checked >= len(task.Queue) {
		ids := make([]string, 0)
		for _, resume := range a.GetProjectResumes(projectID) {
			if eligible51JobAttachmentSync(resume) {
				ids = append(ids, resume.ID)
			}
		}
		sort.Strings(ids)
		task = &AttachmentSyncTask{ProjectID: projectID, PlatformAccount: account, PlatformJobID: jobID, Queue: ids}
	}
	if len(task.Queue) == 0 {
		task.State = "completed"
		return task, a.saveAttachmentSyncTask(db, task)
	}
	ctx, cancel := context.WithCancel(context.Background())
	task.State = "running"
	task.LastError = ""
	if err := a.saveAttachmentSyncTask(db, task); err != nil {
		cancel()
		return nil, err
	}
	a.attachmentSyncCancel[projectID] = cancel
	go a.run51JobAttachmentSync(ctx, *task)
	return task, nil
}

func eligible51JobAttachmentSync(resume *Resume) bool {
	return resume != nil && resume.SourcePlatform == "51job" &&
		(!resume.HasAttachment || resume.AttachmentPendingReason != "") &&
		is51JobCandidateDetailURL(resume.URL) &&
		(resume.PlatformCandidateID == "" || is51JobResumeURLForCandidate(resume.URL, resume.PlatformCandidateID))
}

func (a *App) Stop51JobAttachmentSync(projectID string) error {
	a.attachmentSyncMu.Lock()
	cancel := a.attachmentSyncCancel[projectID]
	a.attachmentSyncMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (a *App) run51JobAttachmentSync(ctx context.Context, task AttachmentSyncTask) {
	defer func() {
		a.attachmentSyncMu.Lock()
		delete(a.attachmentSyncCancel, task.ProjectID)
		a.attachmentSyncMu.Unlock()
	}()
	db, err := a.getRecruitmentDB()
	if err != nil {
		return
	}
	for task.Checked < len(task.Queue) {
		if ctx.Err() != nil {
			task.State = "paused"
			task.CurrentName = ""
			_ = a.saveAttachmentSyncTask(db, &task)
			return
		}
		id := task.Queue[task.Checked]
		var resume *Resume
		for _, candidate := range a.GetProjectResumes(task.ProjectID) {
			if candidate.ID == id {
				resume = candidate
				break
			}
		}
		if !eligible51JobAttachmentSync(resume) {
			task.Skipped++
			task.Checked++
			_ = a.saveAttachmentSyncTask(db, &task)
			continue
		}
		task.CurrentName = conversationCandidateName(resume)
		_ = a.saveAttachmentSyncTask(db, &task)
		probe := a.attachmentSyncProbe
		if probe == nil {
			probe = a.check51JobResumeReplyContext
		}
		result := probe(ctx, id)
		if ctx.Err() != nil {
			task.State = "paused"
			task.CurrentName = ""
			_ = a.saveAttachmentSyncTask(db, &task)
			return
		}
		switch result.Status {
		case "downloaded":
			task.Downloaded++
		case "already_current", "waiting", "reply":
			task.Skipped++
		case "needs_login":
			task.State = "paused"
			task.LastError = result.Message
			task.CurrentName = ""
			_ = a.saveAttachmentSyncTask(db, &task)
			return
		default:
			task.Failed++
			task.LastError = result.Message
		}
		task.Checked++
		task.CurrentName = ""
		if err := a.saveAttachmentSyncTask(db, &task); err != nil {
			return
		}
	}
	task.State = "completed"
	task.CurrentName = ""
	_ = a.saveAttachmentSyncTask(db, &task)
}
