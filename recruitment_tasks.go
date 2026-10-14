package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type SourcingTaskConfig struct {
	ProjectID       string   `json:"projectId"`
	Keyword         string   `json:"keyword"`
	City            string   `json:"city"`
	ExpYears        int      `json:"expYears"`
	EduLevel        string   `json:"eduLevel"`
	Count           int      `json:"count"`
	Platforms       []string `json:"platforms"`
	QuotaMatrixJSON string   `json:"quotaMatrixJson"`
	AutoAnalyze     bool     `json:"autoAnalyze"`
	ActivityDays    int      `json:"activityDays"`
	ChannelStrategy string   `json:"channelStrategy"`
}

type SourcingTask struct {
	ID              string             `json:"id"`
	ProjectID       string             `json:"projectId"`
	ProjectName     string             `json:"projectName"`
	Config          SourcingTaskConfig `json:"config"`
	Status          string             `json:"status"`
	TargetCount     int                `json:"targetCount"`
	CapturedCount   int                `json:"capturedCount"`
	AnalyzedCount   int                `json:"analyzedCount"`
	FailedCount     int                `json:"failedCount"`
	CurrentPlatform string             `json:"currentPlatform"`
	Message         string             `json:"message"`
	CreatedAt       string             `json:"createdAt"`
	UpdatedAt       string             `json:"updatedAt"`
}

var sourcingAnalysisSlots = make(chan struct{}, 3)

func (a *App) createSourcingTask(config SourcingTaskConfig) (string, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	id := recruitmentID("search")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(`INSERT INTO search_tasks(id,project_id,config_json,status,target_count,created_at,updated_at)
		VALUES(?,?,?,'queued',?,?,?)`, id, config.ProjectID, string(raw), config.Count, now, now)
	if err != nil {
		return "", err
	}
	a.emitSourcingTask(id)
	return id, nil
}

func (a *App) interruptUnfinishedSourcingTasks() error {
	db := a.activeRecruitmentDB()
	if db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`UPDATE search_tasks SET status='interrupted',message='软件已关闭；已抓取结果保留，请手动继续',updated_at=?
		WHERE status IN ('queued','running')`, now); err != nil {
		return err
	}
	_, err := db.Exec(`UPDATE analysis_jobs SET status='interrupted',error='软件关闭时分析中断',updated_at=?
		WHERE status IN ('queued','running')`, now)
	return err
}

func (a *App) ListSourcingTasks(projectID string) ([]SourcingTask, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT t.id,t.project_id,p.name,t.config_json,t.status,t.target_count,
		t.captured_count,t.analyzed_count,t.failed_count,t.current_platform,t.message,t.created_at,t.updated_at
		FROM search_tasks t JOIN projects p ON p.id=t.project_id
		WHERE (?='' OR t.project_id=?) ORDER BY t.created_at DESC,t.id DESC`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SourcingTask, 0)
	for rows.Next() {
		var item SourcingTask
		var raw string
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.ProjectName, &raw, &item.Status,
			&item.TargetCount, &item.CapturedCount, &item.AnalyzedCount, &item.FailedCount,
			&item.CurrentPlatform, &item.Message, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &item.Config); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// DeleteSourcingTask removes only the progress/history row. Candidates and AI
// analysis jobs remain in their own tables.
func (a *App) DeleteSourcingTask(taskID string) (bool, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return false, err
	}
	result, err := db.Exec(`DELETE FROM search_tasks WHERE id=? AND status IN ('completed','partial','failed','stopped','interrupted')`, taskID)
	if err != nil {
		return false, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if deleted > 0 {
		return true, nil
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM search_tasks WHERE id=?`, taskID).Scan(&status); err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("寻才任务不存在")
		}
		return false, err
	}
	return false, fmt.Errorf("任务仍在运行（%s），请先停止并等待结果保存", status)
}

// ClearFinishedSourcingTasks clears the entire finished history.
func (a *App) ClearFinishedSourcingTasks() (int64, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return 0, err
	}
	result, err := db.Exec(`DELETE FROM search_tasks WHERE status IN ('completed','partial','failed','stopped','interrupted')`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// finishSourcingTaskFromDone is the persistence boundary for a normal engine
// completion. The engine's shortfall reason must survive the final status write.
func (a *App) finishSourcingTaskFromDone(taskID string, done map[string]interface{}, persistedCount, deduplicatedCount, targetCount int) bool {
	reportedTotal, ok := done["total"].(float64)
	if !ok || int(reportedTotal) != persistedCount+deduplicatedCount {
		a.setSourcingTaskStatus(taskID, "partial", "平台结果与本地保存数量不一致")
		return false
	}
	if stopped, _ := done["stopped"].(bool); stopped {
		a.setSourcingTaskStatus(taskID, "stopped", "已停止，已抓取结果保留")
	} else if persistedCount < targetCount {
		reason, _ := done["shortfallReason"].(string)
		reason = strings.TrimSpace(reason)
		if reason == "" {
			reason = "当前筛选条件下没有足够的新候选人"
		}
		message := "未达目标：" + reason
		if len([]rune(message)) > 400 {
			message = string([]rune(message)[:400])
		}
		a.setSourcingTaskStatus(taskID, "partial", message)
	} else {
		a.setSourcingTaskStatus(taskID, "completed", "检索已完成，分析可能仍在进行")
	}
	return true
}

func (a *App) emitSourcingTask(id string) {
	if a.ctx == nil {
		return
	}
	tasks, err := a.ListSourcingTasks("")
	if err != nil {
		return
	}
	for _, task := range tasks {
		if task.ID == id {
			runtime.EventsEmit(a.ctx, "sourcing:updated", task)
			return
		}
	}
}

func (a *App) setSourcingTaskStatus(id, status, message string) {
	db := a.activeRecruitmentDB()
	if db == nil || id == "" {
		return
	}
	_, err := db.Exec(`UPDATE search_tasks SET status=?,message=?,updated_at=? WHERE id=?`, status, message,
		time.Now().UTC().Format(time.RFC3339Nano), id)
	if err == nil {
		a.emitSourcingTask(id)
	}
}

func (a *App) updateSourcingTaskEvent(id string, event map[string]interface{}) {
	db := a.activeRecruitmentDB()
	if db == nil {
		return
	}
	message, _ := event["message"].(string)
	platform, _ := event["platform"].(string)
	if len(message) > 400 {
		message = message[:400]
	}
	_, err := db.Exec(`UPDATE search_tasks SET message=?,current_platform=CASE WHEN ?='' THEN current_platform ELSE ? END,
		updated_at=? WHERE id=?`, message, platform, platform, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err == nil {
		a.emitSourcingTask(id)
	}
}

func (a *App) recordSourcingCandidate(id string) {
	db := a.activeRecruitmentDB()
	if db == nil {
		return
	}
	_, err := db.Exec(`UPDATE search_tasks SET captured_count=captured_count+1,updated_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339Nano), id)
	if err == nil {
		a.emitSourcingTask(id)
	}
}

func (a *App) queueSourcingAnalysis(taskID, projectID, resumeID string) {
	db := a.activeRecruitmentDB()
	if db == nil {
		return
	}
	var linkID, versionID string
	err := db.QueryRow(`SELECT pc.id,rv.id FROM project_candidates pc
		JOIN resume_versions rv ON rv.candidate_id=pc.candidate_id
		WHERE pc.project_id=? AND rv.legacy_resume_id=?`, projectID, resumeID).Scan(&linkID, &versionID)
	if err != nil {
		a.recordSourcingAnalysisFailure(taskID, resumeID, fmt.Errorf("档案尚未写入分析队列: %w", err))
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	jobID := recruitmentID("analysis")
	var ruleVersion int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM job_rules WHERE project_id=? AND status='approved'`, projectID).Scan(&ruleVersion); err != nil {
		a.recordSourcingAnalysisFailure(taskID, resumeID, err)
		return
	}
	result, err := db.Exec(`INSERT OR IGNORE INTO analysis_jobs
		(id,project_candidate_id,resume_version_id,rule_version,status,created_at,updated_at)
		VALUES(?,?,?,?,'queued',?,?)`, jobID, linkID, versionID, ruleVersion, now, now)
	if err != nil {
		a.recordSourcingAnalysisFailure(taskID, resumeID, err)
		return
	}
	inserted, _ := result.RowsAffected()
	if inserted == 0 {
		return
	}
	config := a.config.AI
	project := a.GetProject(projectID)
	if project == nil {
		a.finishSourcingAnalysisJob(taskID, jobID, resumeID, fmt.Errorf("招聘项目已不存在"))
		return
	}
	jobConfig := project.JobConfig
	if strings.TrimSpace(config.APIKey) == "" {
		a.finishSourcingAnalysisJob(taskID, jobID, resumeID, fmt.Errorf("AI 未配置，请在设置中填写 API Key 后重试"))
		return
	}
	go func() {
		sourcingAnalysisSlots <- struct{}{}
		defer func() { <-sourcingAnalysisSlots }()
		_, _ = db.Exec(`UPDATE analysis_jobs SET status='running',attempts=attempts+1,updated_at=? WHERE id=?`,
			time.Now().UTC().Format(time.RFC3339Nano), jobID)
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "analysis:updated", map[string]string{"taskId": taskID, "resumeId": resumeID, "status": "running"})
		}
		_, analysisErr := a.AnalyzeResume(resumeID, &config, &jobConfig)
		a.finishSourcingAnalysisJob(taskID, jobID, resumeID, analysisErr)
	}()
}

func (a *App) recordSourcingAnalysisFailure(taskID, resumeID string, err error) {
	db := a.activeRecruitmentDB()
	if db == nil {
		return
	}
	_, _ = db.Exec(`UPDATE search_tasks SET failed_count=failed_count+1,updated_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339Nano), taskID)
	a.emitSourcingTask(taskID)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "analysis:updated", map[string]string{"taskId": taskID, "resumeId": resumeID, "status": "failed", "error": err.Error()})
	}
}

func (a *App) finishSourcingAnalysisJob(taskID, jobID, resumeID string, analysisErr error) {
	db := a.activeRecruitmentDB()
	if db == nil {
		return
	}
	status, message, column := "completed", "", "analyzed_count"
	if analysisErr != nil {
		status, message, column = "failed", analysisErr.Error(), "failed_count"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = db.Exec(`UPDATE analysis_jobs SET status=?,error=?,updated_at=? WHERE id=?`, status, message, now, jobID)
	_, _ = db.Exec(`UPDATE search_tasks SET `+column+`=`+column+`+1,updated_at=? WHERE id=?`, now, taskID)
	a.emitSourcingTask(taskID)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "analysis:updated", map[string]string{"taskId": taskID, "resumeId": resumeID, "status": status, "error": message})
	}
}

// Continue only after explicit HR action. Existing independent profile URLs are
// excluded by the search engine; an interrupted task's saved rows remain intact.
func (a *App) ResumeSourcingTask(taskID string) (bool, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return false, err
	}
	var raw, status string
	var target, captured int
	err = db.QueryRow(`SELECT config_json,status,target_count,captured_count FROM search_tasks WHERE id=?`, taskID).
		Scan(&raw, &status, &target, &captured)
	if err == sql.ErrNoRows {
		return false, fmt.Errorf("检索任务不存在")
	}
	if err != nil {
		return false, err
	}
	if status != "interrupted" && status != "stopped" && status != "partial" && status != "failed" {
		return false, fmt.Errorf("当前任务状态不支持继续: %s", status)
	}
	remaining := target - captured
	if remaining <= 0 {
		return false, fmt.Errorf("目标人数已抓取完成")
	}
	var config SourcingTaskConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return false, err
	}
	config.Count = remaining
	started := a.startQuotaMatrixSearch(config.ProjectID, config.Keyword, config.City, config.ExpYears,
		config.EduLevel, config.Count, config.Platforms, config.QuotaMatrixJSON, config.AutoAnalyze,
		config.ActivityDays, config.ChannelStrategy)
	if started {
		a.setSourcingTaskStatus(taskID, "stopped", "已手动继续；新批次只搜索剩余目标")
	}
	return started, nil
}
