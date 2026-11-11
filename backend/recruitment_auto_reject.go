package backend

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// A platform page is shared by all projects in the dedicated browser. Serialize
// automatic actions so a scheduled job cannot switch the page mid-operation.
var platformAutoRejectMu sync.Mutex

type AutoRejectSummary struct {
	Total     int `json:"total"`
	Rejected  int `json:"rejected"`
	Skipped   int `json:"skipped"`
	Uncertain int `json:"uncertain"`
}

type managementRejectTarget struct {
	Candidate talentManagementCandidate
	Status    int
	Page      int
	Checks    []RuleCheckItem
}

// ProcessTalentManagementUnfitBacklog acts only on persisted, explicit failures.
// It first reads the current platform list and requires the original status and
// exact candidate/record IDs to still be present. Old criteria versions are kept.
func (a *App) ProcessTalentManagementUnfitBacklog(accountScope, jobID string) (AutoRejectSummary, error) {
	platformAutoRejectMu.Lock()
	defer platformAutoRejectMu.Unlock()
	result := AutoRejectSummary{}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return result, err
	}
	var projectID string
	if err := db.QueryRow(`SELECT project_id FROM platform_job_mappings WHERE platform='51job' AND platform_account=? AND platform_job_id=?`, accountScope, jobID).Scan(&projectID); err != nil {
		return result, fmt.Errorf("51job 账号与职位未绑定本地项目: %w", err)
	}
	rows, err := db.Query(`SELECT r.candidate_id,r.record_id,r.candidate_name,r.platform_status,r.reasons_json,COALESCE(l.status,'')
		FROM talent_management_pending_reviews r LEFT JOIN platform_action_ledger l
		ON l.project_id=r.project_id AND l.candidate_id=r.candidate_id AND l.record_id=r.record_id AND l.action_type='reject'
		WHERE r.project_id=? AND r.platform_account=? AND r.platform_job_id=?
		AND r.verdict='unfit' AND r.platform_status IN ('待处理','待定') ORDER BY r.updated_at,r.candidate_id`, projectID, accountScope, jobID)
	if err != nil {
		return result, err
	}
	targets := map[string]managementRejectTarget{}
	for rows.Next() {
		var c talentManagementCandidate
		var platformStatus, raw, ledgerStatus string
		if err := rows.Scan(&c.CandidateID, &c.RecordID, &c.Name, &platformStatus, &raw, &ledgerStatus); err != nil {
			rows.Close()
			return result, err
		}
		c.JobID = jobID
		status := 1
		if platformStatus == "待定" {
			status = 4
		}
		var checks []RuleCheckItem
		if json.Unmarshal([]byte(raw), &checks) != nil {
			continue
		}
		hasFailure := false
		for _, check := range checks {
			if check.Status == "fail" && strings.TrimSpace(check.Evidence) != "" {
				hasFailure = true
				break
			}
		}
		if hasFailure && ledgerStatus != "success" && ledgerStatus != "uncertain" && ledgerStatus != "executing" {
			targets[c.CandidateID+"|"+c.RecordID] = managementRejectTarget{Candidate: c, Status: status, Checks: checks}
		} else if hasFailure {
			result.Skipped++
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	result.Total = len(targets)
	if result.Total == 0 {
		return result, nil
	}
	catalog, err := a.Get51JobTalentManagementCatalog()
	if err != nil {
		return result, err
	}
	if catalog.AccountScope != accountScope {
		return result, fmt.Errorf("51job 登录账号已变化，未执行历史名单")
	}
	jobFound := false
	for _, job := range catalog.Jobs {
		if job.JobID == jobID {
			jobFound = true
			break
		}
	}
	if !jobFound {
		return result, fmt.Errorf("51job 原职位已不在当前账号，未执行历史名单")
	}
	ctx := a.managementImportContext()
	verified := make([]managementRejectTarget, 0, len(targets))
	for _, status := range []int{1, 4} {
		for pageIndex := 1; pageIndex <= 200; pageIndex++ {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			out, err := runTalentManagementAdapterContext(ctx, 90*time.Second, "--mode", "scan", "--job-id", jobID,
				"--status-index", fmt.Sprint(status), "--page-index", fmt.Sprint(pageIndex))
			if err != nil {
				return result, err
			}
			var page talentManagementPage
			if err := json.Unmarshal(out, &page); err != nil {
				return result, err
			}
			if page.Status != "ok" || page.AccountScope != accountScope || page.JobID != jobID || page.StatusIndex != status || page.PageIndex != pageIndex {
				return result, fmt.Errorf("51job 待拒绝名单的账号、职位或页码回执不一致")
			}
			for _, c := range page.Candidates {
				target, ok := targets[c.CandidateID+"|"+c.RecordID]
				if !ok || target.Status != status {
					continue
				}
				delete(targets, c.CandidateID+"|"+c.RecordID)
				target.Page = pageIndex
				verified = append(verified, target)
			}
			if !page.HasMore {
				break
			}
		}
	}
	// Remove from the last page backward, and within each page from the last
	// card backward, so platform pagination does not skip a shifted candidate.
	for status := 1; status <= 4; status += 3 {
		for i := len(verified) - 1; i >= 0; i-- {
			target := verified[i]
			if target.Status != status {
				continue
			}
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			c := target.Candidate
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "talent_management_import_progress", map[string]interface{}{
					"statusText": fmt.Sprintf("正在核验并标记不合适：%s · %d/%d", c.Name, result.Rejected+result.Uncertain+1, result.Total),
				})
			}
			action, _ := a.rejectUnfitManagementCandidate(ctx, db, projectID, accountScope, jobID, status, target.Page, c, target.Checks)
			if action == "success" {
				result.Rejected++
				if err := archiveRejectedManagementCandidate(db, projectID, c.CandidateID); err != nil {
					return result, err
				}
				if _, err := db.Exec(`DELETE FROM talent_management_pending_reviews WHERE project_id=? AND platform_account=? AND platform_job_id=? AND candidate_id=? AND record_id=?`,
					projectID, accountScope, jobID, c.CandidateID, c.RecordID); err != nil {
					return result, err
				}
			} else {
				result.Uncertain++
			}
		}
	}
	result.Skipped += len(targets)
	return result, nil
}

func (a *App) processHistoricalManagementUnfitOnce() {
	a.talentImportMu.Lock()
	importRunning := a.talentImportCtx != nil
	a.talentImportMu.Unlock()
	if importRunning {
		return
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return
	}
	rows, err := db.Query(`SELECT DISTINCT platform_account,platform_job_id FROM talent_management_pending_reviews
		WHERE verdict='unfit' AND platform_status IN ('待处理','待定')`)
	if err != nil {
		return
	}
	var pairs [][2]string
	for rows.Next() {
		var p [2]string
		if rows.Scan(&p[0], &p[1]) == nil {
			pairs = append(pairs, p)
		}
	}
	rows.Close()
	for _, p := range pairs {
		_, _ = a.ProcessTalentManagementUnfitBacklog(p[0], p[1])
	}
}
