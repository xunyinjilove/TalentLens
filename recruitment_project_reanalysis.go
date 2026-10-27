package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ProjectReanalysisStart struct {
	RunID     string         `json:"runId"`
	ProjectID string         `json:"projectId"`
	Total     int            `json:"total"`
	Resumed   bool           `json:"resumed"`
	Skipped   map[string]int `json:"skipped"`
}

type ProjectReanalysisStatus struct {
	RunID     string `json:"runId"`
	ProjectID string `json:"projectId"`
	Status    string `json:"status"`
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	LastError string `json:"lastError"`
}

func (a *App) GetProjectReanalysisStatus(runID string) (ProjectReanalysisStatus, error) {
	result := ProjectReanalysisStatus{}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return result, err
	}
	err = db.QueryRow(`SELECT id,project_id,status,total,completed,failed,last_error FROM project_reanalysis_runs WHERE id=?`, runID).
		Scan(&result.RunID, &result.ProjectID, &result.Status, &result.Total, &result.Completed, &result.Failed, &result.LastError)
	if err == sql.ErrNoRows {
		return result, fmt.Errorf("分析任务不存在")
	}
	if err != nil {
		return result, err
	}
	// 实时动态统计：若任务处于进行中，从各条目实时聚合已完成与失败数，防止前端轮询被主表延迟写入阻塞
	if result.Status == "running" {
		var liveCompleted, liveFailed int
		_ = db.QueryRow(`SELECT COUNT(*) FROM project_reanalysis_items WHERE run_id=? AND status='done'`, runID).Scan(&liveCompleted)
		_ = db.QueryRow(`SELECT COUNT(*) FROM project_reanalysis_items WHERE run_id=? AND status='failed'`, runID).Scan(&liveFailed)
		if liveCompleted > result.Completed {
			result.Completed = liveCompleted
		}
		if liveFailed > result.Failed {
			result.Failed = liveFailed
		}
	}
	return result, nil
}

// projectReanalysisTargets uses the same active project-candidate links as the
// talent library. Neither archived people nor incomplete documents are sent
// to the model.
func (a *App) projectReanalysisTargets(projectID string) ([]string, map[string]int, error) {
	candidates, err := a.ListTalentCandidates(projectID)
	if err != nil {
		return nil, nil, err
	}
	ids := make([]string, 0, len(candidates))
	skipped := map[string]int{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.ResumeID)
		if id == "" {
			skipped["未关联简历"]++
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		data, err := readJSONFile(filepath.Join(a.getDataDir(), "resumes", id+".json"))
		if err != nil {
			skipped["简历文件缺失"]++
			continue
		}
		var resume Resume
		if json.Unmarshal(data, &resume) != nil || resume.ID != id || resume.ProjectID != projectID {
			skipped["简历与职位关联不一致"]++
			continue
		}
		if resume.Status == "pending_document" || strings.TrimSpace(resume.Content) == "" {
			skipped["资料尚未补全"]++
			continue
		}
		if resume.HasAttachment && strings.TrimSpace(resume.AttachmentContent) == "" {
			path := strings.TrimSpace(resume.AttachmentPath)
			if path == "" {
				skipped["附件尚未取得"]++
				continue
			}
			if _, err := os.Stat(path); err != nil {
				skipped["附件尚未取得"]++
				continue
			}
		}
		ids = append(ids, id)
	}
	return ids, skipped, nil
}

// StartProjectBatchReanalysis starts or resumes one job-specific, persistent
// batch. A completed run is never reused: the next click recalculates all
// currently eligible resumes with the same AnalyzeResume model path as the
// assessment page.
func (a *App) StartProjectBatchReanalysis(projectID string) (ProjectReanalysisStart, error) {
	result := ProjectReanalysisStart{ProjectID: projectID, Skipped: map[string]int{}}
	project := a.GetProject(projectID)
	if project == nil {
		return result, fmt.Errorf("请先选择有效的职位专属分库")
	}
	cfg := a.config.AI
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.BaseURL) == "" {
		return result, fmt.Errorf("AI 未配置：请先在系统设置中填写 API Key 和 Base URL")
	}
	eligible, skipped, err := a.projectReanalysisTargets(projectID)
	if err != nil {
		return result, err
	}
	result.Skipped = skipped
	if len(eligible) == 0 {
		return result, fmt.Errorf("当前职位没有资料完整、可读取的有效简历；请先补齐资料")
	}

	a.analysisMutex.Lock()
	if a.activeProjects == nil {
		a.activeProjects = make(map[string]bool)
	}
	if a.activeProjects[projectID] {
		a.analysisMutex.Unlock()
		return result, fmt.Errorf("该职位已有分析任务正在运行")
	}
	a.activeProjects[projectID] = true
	a.analysisMutex.Unlock()
	started := false
	defer func() {
		if !started {
			a.analysisMutex.Lock()
			delete(a.activeProjects, projectID)
			a.analysisMutex.Unlock()
		}
	}()

	db, err := a.getRecruitmentDB()
	if err != nil {
		return result, err
	}
	var oldID, oldStatus string
	err = db.QueryRow(`SELECT id,status FROM project_reanalysis_runs WHERE project_id=? ORDER BY created_at DESC,id DESC LIMIT 1`, projectID).Scan(&oldID, &oldStatus)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}
	eligibleSet := make(map[string]bool, len(eligible))
	for _, id := range eligible {
		eligibleSet[id] = true
	}
	var targets []string
	var staleIDs []string
	if oldID != "" && (oldStatus == "running" || oldStatus == "partial") {
		rows, queryErr := db.Query(`SELECT resume_id FROM project_reanalysis_items WHERE run_id=? AND status NOT IN ('done','skipped') ORDER BY resume_id`, oldID)
		if queryErr != nil {
			return result, queryErr
		}
		for rows.Next() {
			var id string
			if scanErr := rows.Scan(&id); scanErr != nil {
				rows.Close()
				return result, scanErr
			}
			if eligibleSet[id] {
				targets = append(targets, id)
			} else {
				result.Skipped["重算期间资料失效"]++
				staleIDs = append(staleIDs, id)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return result, err
		}
		rows.Close()
		for _, id := range staleIDs {
			if _, err := db.Exec(`UPDATE project_reanalysis_items SET status='skipped',error='重算期间资料失效',updated_at=? WHERE run_id=? AND resume_id=?`,
				time.Now().UTC().Format(time.RFC3339Nano), oldID, id); err != nil {
				return result, err
			}
		}
	}
	if len(targets) > 0 {
		result.RunID, result.Resumed = oldID, true
		if _, err := db.Exec(`UPDATE project_reanalysis_runs SET status='running',updated_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), oldID); err != nil {
			return result, err
		}
	} else {
		targets = eligible
		result.RunID = recruitmentID("reanalysis")
		now := time.Now().UTC().Format(time.RFC3339Nano)
		skippedJSON, _ := json.Marshal(skipped)
		tx, err := db.Begin()
		if err != nil {
			return result, err
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`INSERT INTO project_reanalysis_runs(id,project_id,status,total,skipped_json,created_at,updated_at)
			VALUES(?,?,'running',?,?,?,?)`, result.RunID, projectID, len(targets), string(skippedJSON), now, now); err != nil {
			return result, err
		}
		for _, id := range targets {
			if _, err := tx.Exec(`INSERT INTO project_reanalysis_items(run_id,resume_id,status,updated_at) VALUES(?,?,'pending',?)`, result.RunID, id, now); err != nil {
				return result, err
			}
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
	}
	result.Total = len(targets)
	jobProject, err := a.ensureBatchJobRadar(projectID, &cfg)
	if err != nil {
		_, _ = db.Exec(`UPDATE project_reanalysis_runs SET status='partial',last_error=?,updated_at=? WHERE id=?`, err.Error(), time.Now().UTC().Format(time.RFC3339Nano), result.RunID)
		return result, fmt.Errorf("岗位指标准备失败，未开始分析: %w", err)
	}
	started = true
	go a.runProjectReanalysis(db, result, targets, cfg, jobProject.JobConfig)
	return result, nil
}

func (a *App) runProjectReanalysis(db *sql.DB, run ProjectReanalysisStart, targets []string, cfg AIConfig, job JobConfig) {
	defer func() {
		a.analysisMutex.Lock()
		delete(a.activeProjects, run.ProjectID)
		a.analysisMutex.Unlock()
	}()

	total := len(targets)
	if total == 0 {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, _ = db.Exec(`UPDATE project_reanalysis_runs SET status='completed',completed=0,failed=0,updated_at=? WHERE id=?`, now, run.RunID)
		a.emitEvent("project-reanalysis:completed", map[string]interface{}{
			"runId": run.RunID, "projectId": run.ProjectID, "total": 0,
			"failed": 0, "succeeded": 0, "skipped": run.Skipped,
			"lastError": "", "status": "completed",
		})
		return
	}

	// 工业级 Worker Pool 并发控制：并发度设为 3，既大幅缩短等待时间（从单线程 ~8 分钟降至 ~2.5 分钟），又保障 LLM API 速率安全
	const maxWorkers = 3
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var completedCount int32
	var failedCount int32
	var mu sync.Mutex
	var lastError string

	for _, id := range targets {
		wg.Add(1)
		sem <- struct{}{} // 获取执行槽位

		go func(resumeID string) {
			defer func() {
				<-sem // 释放执行槽位
				wg.Done()
			}()

			_, err := a.AnalyzeResume(resumeID, &cfg, &job)
			status := "done"
			errorText := ""
			if err != nil {
				status, errorText = "failed", err.Error()
				atomic.AddInt32(&failedCount, 1)
				mu.Lock()
				lastError = errorText
				mu.Unlock()
			} else {
				atomic.AddInt32(&completedCount, 1)
			}

			now := time.Now().UTC().Format(time.RFC3339Nano)
			_, _ = db.Exec(`UPDATE project_reanalysis_items SET status=?,error=?,updated_at=? WHERE run_id=? AND resume_id=?`,
				status, errorText, now, run.RunID, resumeID)

			done := int(atomic.LoadInt32(&completedCount))
			fail := int(atomic.LoadInt32(&failedCount))
			current := done + fail

			// 每完成/失败一份，立即更新主任务的进度与时间戳，让前端轮询可实时感知
			_, _ = db.Exec(`UPDATE project_reanalysis_runs SET completed=?,failed=?,last_error=?,updated_at=? WHERE id=?`,
				done, fail, errorText, now, run.RunID)

			// 实时广播事件通知前端视图
			a.emitEvent("project-reanalysis:progress", map[string]interface{}{
				"runId":     run.RunID,
				"projectId": run.ProjectID,
				"current":   current,
				"completed": done,
				"total":     total,
				"failed":    fail,
				"resumeId":  resumeID,
				"error":     errorText,
			})
			a.emitEvent("batch:progress", map[string]interface{}{
				"current":   current,
				"total":     total,
				"resumeId":  resumeID,
				"projectId": run.ProjectID,
			})
		}(id)
	}

	wg.Wait()

	finalDone := int(atomic.LoadInt32(&completedCount))
	finalFailed := int(atomic.LoadInt32(&failedCount))
	finalStatus := "completed"
	if finalFailed > 0 {
		finalStatus = "partial"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mu.Lock()
	errSummary := lastError
	mu.Unlock()

	_, _ = db.Exec(`UPDATE project_reanalysis_runs SET status=?,completed=?,failed=?,last_error=?,updated_at=? WHERE id=?`,
		finalStatus, finalDone, finalFailed, errSummary, now, run.RunID)

	a.emitEvent("project-reanalysis:completed", map[string]interface{}{
		"runId":     run.RunID,
		"projectId": run.ProjectID,
		"total":     total,
		"failed":    finalFailed,
		"succeeded": finalDone,
		"skipped":   run.Skipped,
		"lastError": errSummary,
		"status":    finalStatus,
	})
	a.emitEvent("batch:completed", map[string]interface{}{
		"total":     total,
		"succeeded": finalDone,
		"failed":    finalFailed,
		"projectId": run.ProjectID,
	})
}
