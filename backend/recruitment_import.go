package backend

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
	RunID                string `json:"runId"`
	Platform             string `json:"platform"`
	Status               string `json:"status"`
	Message              string `json:"message"`
	AddedCount           int    `json:"addedCount"`
	DuplicateCount       int    `json:"duplicateCount"`
	UnassignedCount      int    `json:"unassignedCount"`
	FailedCount          int    `json:"failedCount"`
	PendingDocumentCount int    `json:"pendingDocumentCount"`
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

// A position-specific library must have one current 51job position. Keep old
// applications for audit, while hiding resumes imported through stale bindings.
func (a *App) bindExclusive51JobProject(account, jobID, projectID string) error {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT platform_account,platform_application_id FROM applications
		WHERE platform='51job' AND project_id=? AND (platform_account<>? OR platform_job_id<>?)`, projectID, account, jobID)
	if err != nil {
		return err
	}
	var stale []ApplicationRecord
	for rows.Next() {
		var item ApplicationRecord
		item.Platform = "51job"
		if err := rows.Scan(&item.PlatformAccount, &item.ApplicationID); err != nil {
			rows.Close()
			return err
		}
		stale = append(stale, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, item := range stale {
		_, err = tx.Exec(`UPDATE project_candidates SET archived_at=?,updated_at=? WHERE project_id=? AND archived_at='' AND id IN (
			SELECT pcr.project_candidate_id FROM project_candidate_resumes pcr
			JOIN resume_versions rv ON rv.id=pcr.resume_version_id WHERE rv.legacy_resume_id=?)`,
			now, now, projectID, applicationResumeID(item))
		if err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`UPDATE applications SET status='wrong_job_archived' WHERE platform='51job' AND project_id=? AND (platform_account<>? OR platform_job_id<>?)`, projectID, account, jobID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE talent_management_sync_tasks SET state='paused',auto_sync_enabled=0,updated_at=? WHERE project_id=? AND (platform_account<>? OR platform_job_id<>?)`, now, projectID, account, jobID); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM platform_job_mappings WHERE platform='51job' AND project_id=? AND (platform_account<>? OR platform_job_id<>?)`, projectID, account, jobID); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO platform_job_mappings(platform,platform_account,platform_job_id,project_id)
		VALUES('51job',?,?,?) ON CONFLICT(platform,platform_account,platform_job_id) DO UPDATE SET project_id=excluded.project_id`, account, jobID, projectID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_, err = repairMisfiled51JobProjectCandidates(db, account, jobID, projectID)
	return err
}

// Older imports may have moved an application row to another project while
// leaving its project_candidate association in this library. Compare the
// immutable resume ID derived from the platform application with the exact
// account/job mapping; never infer a match from a similar job title.
func repairMisfiled51JobProjectCandidates(db *sql.DB, account, jobID, projectID string) (int, error) {
	if account == "" || jobID == "" || projectID == "" {
		return 0, nil
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT platform_account,platform_application_id,platform_job_id FROM applications WHERE platform='51job'`)
	if err != nil {
		return 0, err
	}
	correct, foreign := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var sourceAccount, applicationID, sourceJob string
		if err := rows.Scan(&sourceAccount, &applicationID, &sourceJob); err != nil {
			rows.Close()
			return 0, err
		}
		id := applicationResumeID(ApplicationRecord{Platform: "51job", PlatformAccount: sourceAccount, ApplicationID: applicationID})
		if sourceAccount == account && sourceJob == jobID {
			correct[id] = true
		} else {
			foreign[id] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	rows, err = tx.Query(`SELECT pc.id,rv.legacy_resume_id FROM project_candidates pc
		JOIN project_candidate_resumes pcr ON pcr.project_candidate_id=pc.id
		JOIN resume_versions rv ON rv.id=pcr.resume_version_id
		WHERE pc.project_id=? AND pc.archived_at='' AND rv.legacy_resume_id<>''`, projectID)
	if err != nil {
		return 0, err
	}
	type membership struct{ correct, foreign bool }
	members := map[string]membership{}
	for rows.Next() {
		var id, resumeID string
		if err := rows.Scan(&id, &resumeID); err != nil {
			rows.Close()
			return 0, err
		}
		m := members[id]
		m.correct = m.correct || correct[resumeID]
		m.foreign = m.foreign || foreign[resumeID]
		members[id] = m
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	archived := 0
	for id, m := range members {
		if !m.foreign || m.correct {
			continue
		}
		result, err := tx.Exec(`UPDATE project_candidates SET archived_at=?,updated_at=? WHERE id=? AND project_id=? AND archived_at=''`, now, now, id, projectID)
		if err != nil {
			return archived, err
		}
		if count, _ := result.RowsAffected(); count > 0 {
			archived++
		}
	}
	if err := tx.Commit(); err != nil {
		return archived, err
	}
	return archived, nil
}

func (a *App) repairAllMisfiled51JobProjectCandidates() error {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	rows, err := db.Query(`SELECT project_id,platform_account,platform_job_id FROM platform_job_mappings WHERE platform='51job'`)
	if err != nil {
		return err
	}
	type binding struct{ projectID, account, jobID string }
	byProject := map[string][]binding{}
	for rows.Next() {
		var b binding
		if err := rows.Scan(&b.projectID, &b.account, &b.jobID); err != nil {
			rows.Close()
			return err
		}
		byProject[b.projectID] = append(byProject[b.projectID], b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, bindings := range byProject {
		if len(bindings) != 1 {
			continue
		} // Ambiguous legacy bindings need an explicit choice.
		b := bindings[0]
		if _, err := repairMisfiled51JobProjectCandidates(db, b.account, b.jobID, b.projectID); err != nil {
			return err
		}
	}
	return nil
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
	rawProfile := ""
	if record.Platform == "51job" && strings.HasPrefix(content, "{") {
		formatted := format51JobResume(content)
		if formatted != content {
			rawProfile, content = content, formatted
		}
	}
	status := "pending"
	if content == "" || (record.DocumentsComplete != nil && !*record.DocumentsComplete) {
		status = "pending_document"
		if content == "" {
			content = "平台投递记录已导入，尚未取得可评估的完整简历。"
		}
	}
	resume := &Resume{ID: applicationResumeID(record), ProjectID: projectID, FileName: record.CandidateName,
		Content: content, FileType: ".txt", FileSize: int64(len(content)), URL: record.ProfileURL,
		RawProfileJSON: rawProfile,
		SourcePlatform: record.Platform, PlatformCandidateID: record.PlatformCandidateID, PlatformStatus: record.PlatformStatus, Status: status, CreatedAt: time.Now()}
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
			targetProj := ""
			if record.PlatformJobID != "" {
				_ = db.QueryRow(`SELECT project_id FROM platform_job_mappings WHERE platform=? AND platform_account=? AND platform_job_id=?`,
					platform, record.PlatformAccount, record.PlatformJobID).Scan(&targetProj)
			}
			resumeID := applicationResumeID(record)
			resumePath := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")

			// 检查是否已经在目标项目中真正建有人才库关联（检查 project_candidates）
			hasActiveProjectCandidate := false
			if targetProj != "" {
				var count int
				_ = db.QueryRow(`SELECT COUNT(*) FROM project_candidates pc
					JOIN project_candidate_resumes pcr ON pcr.project_candidate_id=pc.id
					JOIN resume_versions rv ON rv.id=pcr.resume_version_id
					WHERE pc.project_id=? AND rv.legacy_resume_id=? AND pc.archived_at=''`, targetProj, resumeID).Scan(&count)
				hasActiveProjectCandidate = count > 0
			}

			// 情况 A：若原记录之前被判定为 unfit_archived，或者之前尚未在当前目标专属库生成有效简历：
			// 既然该记录通过了当前岗位的硬性条件判定进入了 records，必须为其在目标专属库建档入库！
			if (!hasActiveProjectCandidate || appStatus == "unfit_archived") && targetProj != "" && strings.TrimSpace(record.ResumeText) != "" {
				if err := a.createApplicationResume(record, targetProj); err == nil {
					nowIso := time.Now().UTC().Format(time.RFC3339Nano)
					_, _ = db.Exec(`UPDATE applications SET project_id=?, status='assigned', updated_at=? WHERE id=?`, targetProj, nowIso, appID)
					// 若 project_candidates 之前存在被归档记录，解除归档恢复为活跃
					_, _ = db.Exec(`UPDATE project_candidates SET archived_at='', updated_at=?
						WHERE project_id=? AND id IN (
							SELECT pcr.project_candidate_id FROM project_candidate_resumes pcr
							JOIN resume_versions rv ON rv.id=pcr.resume_version_id
							WHERE rv.legacy_resume_id=?
						)`, nowIso, targetProj, resumeID)

					if record.DocumentsComplete == nil || *record.DocumentsComplete {
						a.queueImportedAnalysis(targetProj, resumeID)
					}
					result.AddedCount++
					continue
				}
			}

			// 情况 B：若原记录因缺少附件处于 pending_document，且本次同步已获得完整资料，补齐内容并入队分析
			if (record.DocumentsComplete == nil || *record.DocumentsComplete) && strings.TrimSpace(record.ResumeText) != "" {
				if rawResume, readErr := os.ReadFile(resumePath); readErr == nil {
					var existingResume Resume
					if json.Unmarshal(rawResume, &existingResume) == nil && existingResume.Status == "pending_document" {
						existingResume.Content = strings.TrimSpace(record.ResumeText)
						existingResume.FileSize = int64(len(existingResume.Content))
						existingResume.Status = "pending"
						a.saveResume(&existingResume)
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
