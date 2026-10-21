package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type CommunicationJobRecord struct {
	ID            string   `json:"id"`
	Platform      string   `json:"platform"`
	ScopeAccount  string   `json:"scopeAccount"`
	JobID         string   `json:"jobId"`
	CompanyID     string   `json:"companyId"`
	CompanyName   string   `json:"companyName"`
	Title         string   `json:"title"`
	WorkArea      string   `json:"workArea"`
	Sources       []string `json:"sources"`
	PublishStatus string   `json:"publishStatus"`
	IsValid       bool     `json:"isValid"`
	LastSyncedAt  string   `json:"lastSyncedAt"`
	CreatedAt     string   `json:"createdAt"`
}

type ProjectCommunicationJobRecord struct {
	ProjectID    string `json:"projectId"`
	Platform     string `json:"platform"`
	ScopeAccount string `json:"scopeAccount"`
	JobID        string `json:"jobId"`
	JobTitle     string `json:"jobTitle"`
	CompanyID    string `json:"companyId"`
	ConfirmedAt  string `json:"confirmedAt"`
}

type CommunicationJobSyncSnapshot struct {
	ID           string `json:"id"`
	Platform     string `json:"platform"`
	ScopeAccount string `json:"scopeAccount"`
	Status       string `json:"status"` // "complete", "partial", "syncing", "failed", "not_started"
	TotalJobs    int    `json:"totalJobs"`
	ErrorMessage string `json:"errorMessage"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
}

type CommunicationJobQueryResult struct {
	Status          string                   `json:"status"` // "ok", "empty"
	Jobs            []JobCommunicationOption `json:"jobs"`
	DefaultJobID    string                   `json:"defaultJobId"`
	DefaultJobTitle string                   `json:"defaultJobTitle"`
	LastSyncedAt    string                   `json:"lastSyncedAt"`
	SyncStatus      string                   `json:"syncStatus"`
	SyncMessage     string                   `json:"syncMessage"`
	NeedsRefresh    bool                     `json:"needsRefresh"`
	TotalJobs       int                      `json:"totalJobs"`
}

func (a *App) SaveCommunicationJobs(platform, scopeAccount string, jobs []JobCommunicationOption) error {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO communication_jobs
		(id, platform, scope_account, job_id, company_id, company_name, title, work_area, sources, publish_status, is_valid, last_synced_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(platform, scope_account, job_id, company_id) DO UPDATE SET
			company_name=excluded.company_name,
			title=excluded.title,
			work_area=excluded.work_area,
			sources=excluded.sources,
			publish_status=excluded.publish_status,
			is_valid=1,
			last_synced_at=excluded.last_synced_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, job := range jobs {
		jobID := strings.TrimSpace(job.ID)
		if jobID == "" {
			continue
		}
		title := strings.TrimSpace(job.Title)
		companyID := strings.TrimSpace(job.CompanyID)
		companyName := strings.TrimSpace(job.Company)
		workArea := strings.TrimSpace(job.WorkArea)
		pubStatus := strings.TrimSpace(job.PublishStatus)
		sourceJSON := "[]"
		if strings.TrimSpace(job.Source) != "" {
			srcList := []string{strings.TrimSpace(job.Source)}
			if b, err := json.Marshal(srcList); err == nil {
				sourceJSON = string(b)
			}
		}
		id := fmt.Sprintf("%s_%s_%s_%s", platform, scopeAccount, jobID, companyID)
		_, err := stmt.Exec(id, platform, scopeAccount, jobID, companyID, companyName, title, workArea, sourceJSON, pubStatus, now, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a *App) ListCachedCommunicationJobs(platform, scopeAccount string) ([]JobCommunicationOption, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT job_id, title, company_name, company_id, work_area, sources, publish_status
		FROM communication_jobs
		WHERE platform=? AND scope_account=? AND is_valid=1
		ORDER BY publish_status DESC, title ASC`, platform, scopeAccount)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []JobCommunicationOption
	for rows.Next() {
		var job JobCommunicationOption
		var sourcesStr string
		if err := rows.Scan(&job.ID, &job.Title, &job.Company, &job.CompanyID, &job.WorkArea, &sourcesStr, &job.PublishStatus); err != nil {
			return nil, err
		}
		if sourcesStr != "" {
			var srcList []string
			if err := json.Unmarshal([]byte(sourcesStr), &srcList); err == nil && len(srcList) > 0 {
				job.Source = srcList[0]
			}
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (a *App) GetProjectDefaultCommunicationJob(projectID, platform, scopeAccount string) (*ProjectCommunicationJobRecord, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var rec ProjectCommunicationJobRecord
	err = db.QueryRow(`SELECT project_id, platform, scope_account, job_id, job_title, company_id, confirmed_at
		FROM project_communication_jobs
		WHERE project_id=? AND platform=? AND scope_account=?`, projectID, platform, scopeAccount).
		Scan(&rec.ProjectID, &rec.Platform, &rec.ScopeAccount, &rec.JobID, &rec.JobTitle, &rec.CompanyID, &rec.ConfirmedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &rec, err
}

func (a *App) SetProjectDefaultCommunicationJob(projectID, platform, scopeAccount, jobID, jobTitle, companyID string) error {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO project_communication_jobs (project_id, platform, scope_account, job_id, job_title, company_id, confirmed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, platform, scope_account) DO UPDATE SET
			job_id=excluded.job_id,
			job_title=excluded.job_title,
			company_id=excluded.company_id,
			confirmed_at=excluded.confirmed_at`,
		projectID, platform, scopeAccount, jobID, jobTitle, companyID, now)
	return err
}

func (a *App) GetLatestCommunicationJobSyncSnapshot(platform, scopeAccount string) (*CommunicationJobSyncSnapshot, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var snap CommunicationJobSyncSnapshot
	err = db.QueryRow(`SELECT id, platform, scope_account, status, total_jobs, error_message, started_at, finished_at
		FROM communication_job_sync_snapshots
		WHERE platform=? AND scope_account=?
		ORDER BY started_at DESC LIMIT 1`, platform, scopeAccount).
		Scan(&snap.ID, &snap.Platform, &snap.ScopeAccount, &snap.Status, &snap.TotalJobs, &snap.ErrorMessage, &snap.StartedAt, &snap.FinishedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &snap, err
}

func (a *App) SaveCommunicationJobSyncSnapshot(snapshot *CommunicationJobSyncSnapshot) error {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO communication_job_sync_snapshots
		(id, platform, scope_account, status, total_jobs, error_message, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status,
			total_jobs=excluded.total_jobs,
			error_message=excluded.error_message,
			finished_at=excluded.finished_at`,
		snapshot.ID, snapshot.Platform, snapshot.ScopeAccount, snapshot.Status,
		snapshot.TotalJobs, snapshot.ErrorMessage, snapshot.StartedAt, snapshot.FinishedAt)
	return err
}

// GetCommunicationJobs 立即返回本地已缓存的沟通职位及同步快照，耗时通常 <5ms，零浏览器与在线依赖。
func (a *App) GetCommunicationJobs(projectID, platform string) CommunicationJobQueryResult {
	if strings.TrimSpace(platform) == "" {
		platform = "51job"
	}
	cachedJobs, _ := a.ListCachedCommunicationJobs(platform, "")
	if cachedJobs == nil {
		cachedJobs = []JobCommunicationOption{}
	}

	res := CommunicationJobQueryResult{
		Status:    "ok",
		Jobs:      cachedJobs,
		TotalJobs: len(cachedJobs),
	}
	if len(cachedJobs) == 0 {
		res.Status = "empty"
	}

	// 1. 优先预选该项目 HR 曾确认过的默认职位
	if projectID != "" {
		if defRec, _ := a.GetProjectDefaultCommunicationJob(projectID, platform, ""); defRec != nil && defRec.JobID != "" {
			for _, job := range cachedJobs {
				if job.ID == defRec.JobID {
					res.DefaultJobID = job.ID
					res.DefaultJobTitle = job.Title
					break
				}
			}
		}
	}

	// 2. 若未匹配到项目默认职位，尝试用项目招聘岗位名称唯一精准匹配
	if res.DefaultJobID == "" && projectID != "" {
		project := a.GetProject(projectID)
		if project != nil && strings.TrimSpace(project.JobConfig.Title) != "" {
			targetTitle := strings.TrimSpace(project.JobConfig.Title)
			var matches []JobCommunicationOption
			for _, job := range cachedJobs {
				if strings.TrimSpace(job.Title) == targetTitle {
					matches = append(matches, job)
				}
			}
			if len(matches) == 1 {
				res.DefaultJobID = matches[0].ID
				res.DefaultJobTitle = matches[0].Title
			}
		}
	}

	// 3. 读取最近一次同步快照与状态
	snapshot, _ := a.GetLatestCommunicationJobSyncSnapshot(platform, "")
	if snapshot != nil {
		res.SyncStatus = snapshot.Status
		res.SyncMessage = snapshot.ErrorMessage
		res.LastSyncedAt = snapshot.FinishedAt
		if res.LastSyncedAt != "" {
			if t, err := time.Parse(time.RFC3339, res.LastSyncedAt); err == nil {
				if time.Since(t) > 24*time.Hour {
					res.NeedsRefresh = true
				}
			}
		}
	} else {
		res.SyncStatus = "not_started"
		res.SyncMessage = "尚未执行职位同步"
		res.NeedsRefresh = true
	}

	if len(cachedJobs) == 0 {
		res.NeedsRefresh = true
		if res.SyncMessage == "" {
			res.SyncMessage = "本地暂未缓存在招职位，请点击同步"
		}
	}

	return res
}

// SyncCommunicationJobs 触发 51job 沟通职位后台同步，自动读取并在本地落盘。
func (a *App) SyncCommunicationJobs(projectID, platform string) CommunicationJobQueryResult {
	a.jobSyncMu.Lock()
	defer a.jobSyncMu.Unlock()

	if strings.TrimSpace(platform) == "" {
		platform = "51job"
	}

	snapID := fmt.Sprintf("sync_%d", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339)
	_ = a.SaveCommunicationJobSyncSnapshot(&CommunicationJobSyncSnapshot{
		ID:           snapID,
		Platform:     platform,
		ScopeAccount: "",
		Status:       "syncing",
		TotalJobs:    0,
		StartedAt:    now,
	})

	// 寻找可用于打开 51job 选职窗口的候选人上下文
	resumeID := a.find51JobSyncCandidate(projectID)
	if resumeID == "" {
		_ = a.SaveCommunicationJobSyncSnapshot(&CommunicationJobSyncSnapshot{
			ID:           snapID,
			Platform:     platform,
			ScopeAccount: "",
			Status:       "failed",
			ErrorMessage: "未找到包含有效 51job 详情链接的候选人，无法打开选职窗口；请先导入或核验候选人",
			StartedAt:    now,
			FinishedAt:   time.Now().UTC().Format(time.RFC3339),
		})
		return a.GetCommunicationJobs(projectID, platform)
	}

	// 执行平台选职读取
	result := a.List51JobCommunicationJobs(resumeID, "")
	finishNow := time.Now().UTC().Format(time.RFC3339)
	if result.Status == "jobs" && len(result.Jobs) > 0 {
		_ = a.SaveCommunicationJobs(platform, "", result.Jobs)
		_ = a.SaveCommunicationJobSyncSnapshot(&CommunicationJobSyncSnapshot{
			ID:           snapID,
			Platform:     platform,
			ScopeAccount: "",
			Status:       "complete",
			TotalJobs:    len(result.Jobs),
			StartedAt:    now,
			FinishedAt:   finishNow,
		})
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "communication_jobs:synced", map[string]interface{}{
				"platform": platform,
				"count":    len(result.Jobs),
			})
		}
	} else {
		errMsg := result.Message
		if errMsg == "" {
			errMsg = "51job 职位列表读取未返回职位"
		}
		_ = a.SaveCommunicationJobSyncSnapshot(&CommunicationJobSyncSnapshot{
			ID:           snapID,
			Platform:     platform,
			ScopeAccount: "",
			Status:       "failed",
			ErrorMessage: errMsg,
			StartedAt:    now,
			FinishedAt:   finishNow,
		})
	}

	return a.GetCommunicationJobs(projectID, platform)
}

// find51JobSyncCandidate 在当前项目或全局候选人库中定位一个可用于打开沟通窗口的候选人档案
func (a *App) find51JobSyncCandidate(projectID string) string {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return ""
	}

	// 1. 优先在当前项目中寻找
	if projectID != "" {
		rows, err := db.Query(`SELECT rv.legacy_resume_id FROM project_candidates pc
			JOIN project_candidate_resumes pcr ON pcr.project_candidate_id=pc.id
			JOIN resume_versions rv ON rv.id=pcr.resume_version_id
			WHERE pc.project_id=? AND rv.legacy_resume_id<>''
			ORDER BY pc.created_at DESC LIMIT 10`, projectID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var legacyID string
				if err := rows.Scan(&legacyID); err == nil && legacyID != "" {
					if r, err := a.load51JobContactResume(legacyID); err == nil && r != nil {
						return legacyID
					}
				}
			}
		}
	}

	// 2. 全局最近的 51job 候选人
	rows, err := db.Query(`SELECT legacy_resume_id FROM resume_versions
		WHERE legacy_resume_id<>''
		ORDER BY created_at DESC LIMIT 30`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var legacyID string
			if err := rows.Scan(&legacyID); err == nil && legacyID != "" {
				if r, err := a.load51JobContactResume(legacyID); err == nil && r != nil {
					return legacyID
				}
			}
		}
	}

	return ""
}

func (a *App) suggestedProjectCommunicationJob(projectID string, batch OutreachBatch, jobs []JobCommunicationOption) string {
	// 1. 优先已记录的项目默认职位
	if projectID != "" {
		if rec, _ := a.GetProjectDefaultCommunicationJob(projectID, "51job", ""); rec != nil && rec.JobID != "" {
			for _, job := range jobs {
				if job.ID == rec.JobID {
					return job.ID
				}
			}
		}
	}
	// 2. 其次复用批次建议规则
	return suggestedOutreachJob(batch, jobs)
}
