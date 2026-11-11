package backend

import (
	"log"
	"time"
)

// Historical results keep their original explicit-failure decision, including
// older criteria versions. The action path still verifies the live account,
// job and current candidate list before it can click anything.
func (a *App) processHistoricalTelescopeUnfitOnce() {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return
	}
	rows, err := db.Query(`SELECT DISTINCT b.id FROM screening_batches b
		JOIN screening_candidate_results r ON r.batch_id=b.id
		WHERE b.source='telescope' AND b.scan_status='complete' AND r.verdict='unfit'
		AND r.action_status IN ('pending','receipt_pending','retrying','failed') ORDER BY b.started_at,b.id`)
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if len(ids) == 0 {
		return
	}
	if _, err := a.Get51JobTelescopeCatalog(); err != nil {
		return
	}
	for _, id := range ids {
		_, _ = a.run51JobTelescopeReject(id, true)
	}
}

// This scheduler runs only while the desktop process is alive. A missed time
// while closed is deliberately not replayed on the next startup.
func (a *App) startProjectTelescopeScheduler() {
	go func() {
		time.Sleep(8 * time.Second)
		// Startup backlog is a one-shot attempt. Login failure waits for a later
		// manual run or scheduled minute, rather than opening invalid pages again.
		a.processHistoricalManagementUnfitOnce()
		a.processHistoricalTelescopeUnfitOnce()
		seen := map[string]bool{}
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			a.runScheduledTelescopeMinute(seen)
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (a *App) runScheduledTelescopeMinute(seen map[string]bool) {
	now := time.Now()
	db, err := a.getRecruitmentDB()
	if err != nil {
		return
	}
	rows, err := db.Query(`SELECT project_id,schedule_time FROM talent_management_strict_criteria WHERE schedule_enabled=1`)
	if err != nil {
		return
	}
	type scheduled struct{ projectID, when string }
	var due []scheduled
	for rows.Next() {
		var task scheduled
		if rows.Scan(&task.projectID, &task.when) == nil && task.when == now.Format("15:04") {
			key := now.Format("2006-01-02") + "|" + task.projectID
			if !seen[key] {
				seen[key] = true
				due = append(due, task)
			}
		}
	}
	rows.Close()
	for _, task := range due {
		var account, jobID string
		err := db.QueryRow(`SELECT platform_account,platform_job_id FROM telescope_job_bindings WHERE project_id=?`, task.projectID).Scan(&account, &jobID)
		if err != nil {
			var count int
			err = db.QueryRow(`SELECT COUNT(*),MIN(platform_account),MIN(platform_job_id)
				FROM platform_job_mappings WHERE platform='51job' AND project_id=?`, task.projectID).Scan(&count, &account, &jobID)
			if err != nil || count != 1 {
				log.Printf("[TelescopeSchedule] 项目 %s 尚未绑定望远镜职位", task.projectID)
				continue
			}
		}
		if _, err := a.Get51JobTelescopeCatalog(); err != nil {
			log.Printf("[TelescopeSchedule] 望远镜未就绪: %v", err)
			continue
		}
		preview, err := a.Scan51JobTelescopeBatch(account, jobID, task.projectID)
		if err != nil {
			log.Printf("[TelescopeSchedule] 项目 %s 扫描失败: %v", task.projectID, err)
			continue
		}
		if preview.UnfitCount > 0 {
			if _, err := a.Run51JobTelescopeReject(preview.BatchID); err != nil {
				log.Printf("[TelescopeSchedule] 项目 %s 自动不感兴趣失败: %v", task.projectID, err)
			}
		}
	}
}
