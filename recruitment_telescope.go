package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type telescopeCandidate struct {
	CandidateID   string `json:"candidateId"`
	RecordID      string `json:"recordId"`
	JobID         string `json:"jobId"`
	Name          string `json:"name"`
	ResumeText    string `json:"resumeText"`
	ActivityLabel string `json:"activityLabel"`
	AlreadyHi     bool   `json:"alreadyHi"`
}

type telescopePage struct {
	Status       string               `json:"status"`
	Message      string               `json:"message"`
	JobID        string               `json:"jobId"`
	JobTitle     string               `json:"jobTitle"`
	CompanyID    string               `json:"companyId"`
	AccountScope string               `json:"accountScope"`
	Total        int                  `json:"total"`
	Candidates   []telescopeCandidate `json:"candidates"`
}

type telescopeCatalogResult struct {
	Status         string                `json:"status"`
	Message        string                `json:"message"`
	AccountScope   string                `json:"accountScope"`
	ActiveJobID    string                `json:"activeJobId"`
	ActiveJobTitle string                `json:"activeJobTitle"`
	CompanyID      string                `json:"companyId"`
	Jobs           []TalentManagementJob `json:"jobs"`
}

// Open51JobTelescopeBrowser opens the platform's AI淘金/人才望远镜 page.
func (a *App) Open51JobTelescopeBrowser() error {
	a.EnsurePlatformBrowser("51job")
	return a.OpenURL("https://ehire.51job.com/Revision/talent/search-recommend")
}

func resolveTelescopeScript() string {
	for _, candidate := range []string{
		filepath.Join("scripts", "51job_telescope.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "51job_telescope.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "51job_telescope.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "51job_telescope.js"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// Get51JobTelescopeCatalog reads online positions and account info directly from the opened Telescope page.
func (a *App) Get51JobTelescopeCatalog() (*TalentManagementCatalog, error) {
	script := resolveTelescopeScript()
	if script == "" {
		return nil, fmt.Errorf("缺少 51job 人才望远镜读取脚本")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script, "--mode", "catalog")
	hideConsoleWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("读取人才望远镜职位超时")
		}
		return nil, fmt.Errorf("读取人才望远镜职位失败: %w", err)
	}
	var res telescopeCatalogResult
	if jsonErr := json.Unmarshal(out, &res); jsonErr != nil {
		return nil, fmt.Errorf("人才望远镜目录回执无法解析: %w", jsonErr)
	}
	if res.Status != "ok" {
		return nil, fmt.Errorf("%s", res.Message)
	}
	return &TalentManagementCatalog{
		AccountScope: res.AccountScope,
		Jobs:         res.Jobs,
	}, nil
}

func readTelescopeCandidates(jobID string) (telescopePage, error) {
	result := telescopePage{}
	script := resolveTelescopeScript()
	if script == "" {
		return result, fmt.Errorf("缺少 51job 人才望远镜读取脚本")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script, "--job-id", jobID)
	hideConsoleWindow(cmd)
	out, err := cmd.Output()
	if jsonErr := json.Unmarshal(out, &result); jsonErr != nil {
		if ctx.Err() != nil {
			return result, fmt.Errorf("人才望远镜读取超时: %w", ctx.Err())
		}
		return result, fmt.Errorf("人才望远镜回执无法解析: %w", jsonErr)
	}
	if err != nil || result.Status != "ok" {
		if result.Message != "" {
			return result, fmt.Errorf("%s", result.Message)
		}
		return result, fmt.Errorf("人才望远镜读取失败: %w", err)
	}
	return result, nil
}

// Scan51JobTelescopeBatch reads the currently opened telescope page and uses
// the same project criteria as talent management. It never performs platform actions.
func (a *App) Scan51JobTelescopeBatch(accountScope, jobID, projectID string) (*ScreeningBatchPreview, error) {
	if jobID == "" || projectID == "" {
		return nil, fmt.Errorf("请先选择已绑定本地项目的 51job 职位")
	}

	page, err := readTelescopeCandidates(jobID)
	if err != nil {
		return nil, err
	}
	if page.JobID != jobID {
		return nil, fmt.Errorf("人才望远镜当前展示的职位与所选职位不一致，请在 51job 页面切换到对应职位")
	}
	if accountScope != "" && page.CompanyID != "" && !strings.HasPrefix(accountScope, "co:"+page.CompanyID+":") {
		return nil, fmt.Errorf("人才望远镜企业账号与当前选定账号不一致")
	}
	if accountScope == "" {
		if page.AccountScope != "" {
			accountScope = page.AccountScope
		} else if page.CompanyID != "" {
			accountScope = "co:" + page.CompanyID
		}
	}

	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}

	// 自动保全项目与职位映射
	var mappedProject string
	err = db.QueryRow(`SELECT project_id FROM platform_job_mappings WHERE platform='51job' AND platform_job_id=?`, jobID).Scan(&mappedProject)
	if err != nil || mappedProject != projectID {
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		_, _ = db.Exec(`INSERT INTO platform_job_mappings(platform, platform_account, platform_job_id, project_id, updated_at)
			VALUES('51job', ?, ?, ?, ?)
			ON CONFLICT(platform, platform_account, platform_job_id) DO UPDATE SET project_id=excluded.project_id, updated_at=excluded.updated_at`,
			accountScope, jobID, projectID, nowStr)
	}

	criteria, err := a.GetTalentManagementStrictCriteria(projectID)
	if err != nil {
		return nil, fmt.Errorf("读取项目筛选条件失败: %w", err)
	}
	if criteria == nil {
		return nil, fmt.Errorf("项目筛选条件尚未设置")
	}

	now := time.Now()
	preview := &ScreeningBatchPreview{
		BatchID: recruitmentID("scrbatch"), ProjectID: projectID, Source: "telescope",
		Platform: "51job", PlatformAccount: accountScope, PlatformJobID: jobID,
		BusinessDate: now.Format("2006-01-02"), CriteriaVersion: criteria.CriteriaVersion,
		Status: "preview_ready", QualifiedList: []ScreeningCandidateResult{},
		UnfitList: []ScreeningCandidateResult{}, PendingList: []ScreeningCandidateResult{},
	}
	for _, cand := range page.Candidates {
		if cand.CandidateID == "" || cand.RecordID == "" || cand.JobID != jobID || cand.AlreadyHi {
			continue
		}
		var existing int
		if err := db.QueryRow(`SELECT COUNT(*) FROM platform_action_ledger WHERE project_id=? AND candidate_id=? AND status='success' AND action_type IN ('hi','reject')`, projectID, cand.CandidateID).Scan(&existing); err != nil {
			return nil, err
		}
		if existing > 0 {
			continue
		}
		verdict, checks := EvaluateCandidateTriState(cand.ResumeText, *criteria)
		item := ScreeningCandidateResult{
			ID: recruitmentID("scr"), BatchID: preview.BatchID, ProjectID: projectID,
			CandidateID: cand.CandidateID, RecordID: cand.RecordID, CandidateName: cand.Name,
			Verdict: verdict, RuleEvidence: checks, RawProfile: cand.ResumeText,
			DetailLoaded: false, ActionStatus: "pending", PlatformStatus: "人才望远镜",
			CreatedAt: now.Format("2006-01-02 15:04:05"),
		}
		switch verdict {
		case "qualified":
			preview.QualifiedList = append(preview.QualifiedList, item)
		case "unfit":
			preview.UnfitList = append(preview.UnfitList, item)
		default:
			preview.PendingList = append(preview.PendingList, item)
		}
	}
	preview.QualifiedCount = len(preview.QualifiedList)
	preview.UnfitCount = len(preview.UnfitList)
	preview.PendingCount = len(preview.PendingList)
	preview.TotalCount = preview.QualifiedCount + preview.UnfitCount + preview.PendingCount
	criteriaJSON, err := json.Marshal(criteria)
	if err != nil {
		return nil, err
	}
	if err := persistScreeningBatch(db, preview, string(criteriaJSON), now.Format("2006-01-02 15:04:05")); err != nil {
		return nil, fmt.Errorf("保存望远镜预览失败: %w", err)
	}
	return preview, nil
}
