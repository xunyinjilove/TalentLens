package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ApplicationRecord struct {
	Platform            string `json:"platform"`
	PlatformAccount     string `json:"platformAccount"`
	ApplicationID       string `json:"applicationId"`
	PlatformCandidateID string `json:"platformCandidateId"`
	PlatformJobID       string `json:"platformJobId"`
	CandidateName       string `json:"candidateName"`
	ProfileURL          string `json:"profileUrl"`
	ResumeText          string `json:"resumeText"`
	AppliedAt           string `json:"appliedAt"`
	DocumentsComplete   *bool  `json:"documentsComplete,omitempty"`
	DocumentStatus      string `json:"documentStatus,omitempty"`
	PlatformStatus      string `json:"platformStatus,omitempty"`
}

type ApplicationImportResult struct {
	RunID           string `json:"runId"`
	Platform        string `json:"platform"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	AddedCount      int    `json:"addedCount"`
	DuplicateCount  int    `json:"duplicateCount"`
	UnassignedCount int    `json:"unassignedCount"`
	FailedCount     int    `json:"failedCount"`
	PendingDocumentCount int `json:"pendingDocumentCount"`
}

type ImportedApplication struct {
	ID                  string `json:"id"`
	Platform            string `json:"platform"`
	ApplicationID       string `json:"applicationId"`
	PlatformCandidateID string `json:"platformCandidateId"`
	PlatformJobID       string `json:"platformJobId"`
	CandidateName       string `json:"candidateName"`
	ProjectID           string `json:"projectId"`
	Status              string `json:"status"`
	AppliedAt           string `json:"appliedAt"`
}

type PlatformJobMapping struct {
	Platform        string `json:"platform"`
	PlatformAccount string `json:"platformAccount"`
	PlatformJobID   string `json:"platformJobId"`
	ProjectID       string `json:"projectId"`
}

func (a *App) ListPlatformJobMappings() ([]PlatformJobMapping, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT platform,platform_account,platform_job_id,project_id FROM platform_job_mappings ORDER BY platform,platform_job_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PlatformJobMapping, 0)
	for rows.Next() {
		var item PlatformJobMapping
		if err := rows.Scan(&item.Platform, &item.PlatformAccount, &item.PlatformJobID, &item.ProjectID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *App) SetPlatformJobMapping(platform, account, jobID, projectID string) error {
	if platform != "51job" || strings.TrimSpace(jobID) == "" {
		return fmt.Errorf("首版仅支持 51job 的明确职位 ID")
	}
	if a.GetProject(projectID) == nil {
		return fmt.Errorf("招聘项目不存在")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO platform_job_mappings(platform,platform_account,platform_job_id,project_id)
		VALUES(?,?,?,?) ON CONFLICT(platform,platform_account,platform_job_id) DO UPDATE SET project_id=excluded.project_id`, platform, account, jobID, projectID)
	return err
}

func (a *App) ListImportedApplications(status string) ([]ImportedApplication, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT a.id,a.platform,a.platform_application_id,a.platform_job_id,COALESCE(a.project_id,''),a.status,a.applied_at,a.raw_json
		FROM applications a WHERE (?='' OR a.status=?) ORDER BY a.created_at DESC,a.id DESC`, status, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ImportedApplication, 0)
	for rows.Next() {
		var item ImportedApplication
		var raw string
		if err := rows.Scan(&item.ID, &item.Platform, &item.ApplicationID, &item.PlatformJobID, &item.ProjectID, &item.Status, &item.AppliedAt, &raw); err != nil {
			return nil, err
		}
		var record ApplicationRecord
		if json.Unmarshal([]byte(raw), &record) == nil {
			item.CandidateName = record.CandidateName
			item.PlatformCandidateID = record.PlatformCandidateID
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func applicationResumeID(record ApplicationRecord) string {
	h := sha256.Sum256([]byte(record.Platform + "\x00" + record.PlatformAccount + "\x00" + record.ApplicationID))
	return "application_" + hex.EncodeToString(h[:12])
}

func (a *App) createApplicationResume(record ApplicationRecord, projectID string) error {
	if projectID == "" {
		return fmt.Errorf("未指定岗位")
	}
	content := strings.TrimSpace(record.ResumeText)
	status := "pending"
	if content == "" || (record.DocumentsComplete != nil && !*record.DocumentsComplete) {
		status = "pending_document"
		if content == "" { content = "平台投递记录已导入，尚未取得可评估的完整简历。" }
	}
	resume := &Resume{ID: applicationResumeID(record), ProjectID: projectID, FileName: record.CandidateName,
		Content: content, FileType: ".txt", FileSize: int64(len(content)), URL: record.ProfileURL,
		SourcePlatform: record.Platform, PlatformCandidateID: record.PlatformCandidateID, Status: status, CreatedAt: time.Now()}
	return a.persistSearchCandidate(resume)
}

// The import contract is common to all future platform adapters. A record is
// classified only by the platform job ID selected by HR, never by title text.
func (a *App) ingestApplicationRecords(platform string, records []ApplicationRecord) (ApplicationImportResult, error) {
	result := ApplicationImportResult{RunID: recruitmentID("import"), Platform: platform, Status: "completed"}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return result, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = db.Exec(`INSERT INTO import_runs(id,platform,status,created_at,updated_at) VALUES(?,?,"running",?,?)`, result.RunID, platform, now, now); err != nil {
		return result, err
	}
	for _, record := range records {
		if strings.TrimSpace(record.CandidateName) == "" {
			record.CandidateName = "候选人"
			if record.PlatformCandidateID != "" {
				record.CandidateName = "候选人_" + record.PlatformCandidateID
			}
		}
		if record.Platform != platform || strings.TrimSpace(record.ApplicationID) == "" {
			result.FailedCount++
			continue
		}
		var appID string
		var assignedProject sql.NullString
		var appStatus string
		err = db.QueryRow(`SELECT id, project_id, status FROM applications WHERE platform=? AND platform_account=? AND platform_application_id=?`,
			platform, record.PlatformAccount, record.ApplicationID).Scan(&appID, &assignedProject, &appStatus)
		if err == nil {
			resumeID := applicationResumeID(record)
			resumePath := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")
			// 若原记录因缺少附件处于 pending_document，且本次同步已获得完整资料，补齐内容并入队分析
			if (record.DocumentsComplete == nil || *record.DocumentsComplete) && strings.TrimSpace(record.ResumeText) != "" {
				if rawResume, readErr := os.ReadFile(resumePath); readErr == nil {
					var existingResume Resume
					if json.Unmarshal(rawResume, &existingResume) == nil && existingResume.Status == "pending_document" {
						existingResume.Content = strings.TrimSpace(record.ResumeText)
						existingResume.FileSize = int64(len(existingResume.Content))
						existingResume.Status = "pending"
						a.saveResume(&existingResume)
						targetProj := assignedProject.String
						if targetProj == "" && record.PlatformJobID != "" {
							_ = db.QueryRow(`SELECT project_id FROM platform_job_mappings WHERE platform=? AND platform_account=? AND platform_job_id=?`,
								platform, record.PlatformAccount, record.PlatformJobID).Scan(&targetProj)
						}
						if targetProj != "" {
							_, _ = db.Exec(`UPDATE applications SET project_id=?, status='assigned' WHERE id=?`, targetProj, appID)
							a.queueImportedAnalysis(targetProj, resumeID)
						}
						result.AddedCount++
						continue
					}
				}
			}
			result.DuplicateCount++
			continue
		}
		if err != sql.ErrNoRows {
			result.FailedCount++
			continue
		}
		var projectID string
		if record.PlatformJobID != "" {
			err = db.QueryRow(`SELECT project_id FROM platform_job_mappings WHERE platform=? AND platform_account=? AND platform_job_id=?`,
				platform, record.PlatformAccount, record.PlatformJobID).Scan(&projectID)
			if err != nil && err != sql.ErrNoRows {
				result.FailedCount++
				continue
			}
		}
		status := "unassigned"
		if projectID != "" {
			if err := a.createApplicationResume(record, projectID); err != nil {
				result.FailedCount++
				continue
			}
			status = "assigned"
		} else {
			result.UnassignedCount++
		}
		raw, _ := json.Marshal(record)
		var project interface{}
		if projectID != "" {
			project = projectID
		}
		_, err = db.Exec(`INSERT INTO applications(id,platform,platform_account,platform_application_id,platform_job_id,project_id,status,applied_at,raw_json,created_at)
			VALUES(?,?,?,?,?,?,?,?,?,?)`, recruitmentID("application"), platform, record.PlatformAccount, record.ApplicationID,
			record.PlatformJobID, project, status, record.AppliedAt, string(raw), now)
		if err != nil {
			result.FailedCount++
			continue
		}
		result.AddedCount++
		if status == "assigned" && strings.TrimSpace(record.ResumeText) != "" {
			if record.DocumentsComplete == nil || *record.DocumentsComplete {
				a.queueImportedAnalysis(projectID, applicationResumeID(record))
			} else {
				result.PendingDocumentCount++
			}
		}
	}
	if result.FailedCount > 0 {
		result.Status = "partial"
	}
	_, err = db.Exec(`UPDATE import_runs SET status=?,added_count=?,duplicate_count=?,unassigned_count=?,failed_count=?,updated_at=? WHERE id=?`,
		result.Status, result.AddedCount, result.DuplicateCount, result.UnassignedCount, result.FailedCount, time.Now().UTC().Format(time.RFC3339Nano), result.RunID)
	return result, err
}

func (a *App) queueImportedAnalysis(projectID, resumeID string) {
	if strings.TrimSpace(a.config.AI.APIKey) == "" {
		return
	}
	project := a.GetProject(projectID)
	if project == nil {
		return
	}
	cfg, job := a.config.AI, project.JobConfig
	go func() {
		sourcingAnalysisSlots <- struct{}{}
		defer func() { <-sourcingAnalysisSlots }()
		_, _ = a.AnalyzeResume(resumeID, &cfg, &job)
	}()
}

func (a *App) AssignApplication(applicationID, projectID string) error {
	if a.GetProject(projectID) == nil {
		return fmt.Errorf("招聘项目不存在")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	var raw, status string
	if err = db.QueryRow(`SELECT raw_json,status FROM applications WHERE id=?`, applicationID).Scan(&raw, &status); err != nil {
		return err
	}
	if status != "unassigned" {
		return fmt.Errorf("投递已分配或不可操作")
	}
	var record ApplicationRecord
	if err = json.Unmarshal([]byte(raw), &record); err != nil {
		return err
	}
	if err = a.createApplicationResume(record, projectID); err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE applications SET project_id=?,status='assigned' WHERE id=? AND status='unassigned'`, projectID, applicationID)
	if err == nil && strings.TrimSpace(record.ResumeText) != "" && (record.DocumentsComplete == nil || *record.DocumentsComplete) {
		a.queueImportedAnalysis(projectID, applicationResumeID(record))
	}
	return err
}

func (a *App) ImportApplications(platform string) (ApplicationImportResult, error) {
	result := ApplicationImportResult{Platform: platform, Status: "failed"}
	if platform != "51job" {
		return result, fmt.Errorf("首版仅支持 51job 主动投递")
	}
	candidates := []string{filepath.Join("scripts", "51job_application_import.js"), filepath.Join(filepath.Dir(os.Args[0]), "scripts", "51job_application_import.js"), filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "51job_application_import.js")}
	var script string
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			script = path
			break
		}
	}
	if script == "" {
		return result, fmt.Errorf("缺少 51job 主动投递适配器")
	}
	a.EnsurePlatformBrowser("51job")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script)
	hideConsoleWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return result, fmt.Errorf("读取 51job 主动投递失败: %w", err)
	}
	var payload struct {
		Status        string              `json:"status"`
		Message       string              `json:"message"`
		RejectedCount int                 `json:"rejectedCount"`
		Records       []ApplicationRecord `json:"records"`
	}
	if err = json.Unmarshal(out, &payload); err != nil {
		return result, fmt.Errorf("51job 返回内容无法解析: %w", err)
	}
	if payload.Status != "ok" {
		return result, fmt.Errorf("51job 投递读取未完成: %s", payload.Message)
	}
	result, err = a.ingestApplicationRecords(platform, payload.Records)
	result.Message = payload.Message
	if err == nil && payload.RejectedCount > 0 {
		result.FailedCount += payload.RejectedCount
		result.Status = "partial"
		if db, dbErr := a.getRecruitmentDB(); dbErr == nil {
			_, _ = db.Exec(`UPDATE import_runs SET status='partial',failed_count=? WHERE id=?`, result.FailedCount, result.RunID)
		}
	}
	return result, err
}
