package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type JobManage51JobItem struct {
	Title      string `json:"title"`
	City       string `json:"city"`
	Location   string `json:"location"`
	Education  string `json:"education"`
	Experience string `json:"experience"`
	Salary     string `json:"salary"`
	Status     string `json:"status"`
}

type JobManage51JobResult struct {
	Status  string               `json:"status"`
	Count   int                  `json:"count"`
	Message string               `json:"message,omitempty"`
	Jobs    []JobManage51JobItem `json:"jobs"`
}

type SyncJobCitiesResult struct {
	Success      bool                 `json:"success"`
	UpdatedCount int                  `json:"updatedCount"`
	Message      string               `json:"message"`
	MatchedLogs  []string             `json:"matchedLogs"`
	Jobs         []JobManage51JobItem `json:"jobs"`
}

func find51JobManageScript() string {
	scriptCandidates := []string{
		filepath.Join("scripts", "51job_job_manage_sync.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "51job_job_manage_sync.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "51job_job_manage_sync.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "51job_job_manage_sync.js"),
	}
	for _, candidate := range scriptCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// Fetch51JobManagementJobs 抓取 51job [职位管理] 页面上的所有岗位城市与基本信息
func (a *App) Fetch51JobManagementJobs() JobManage51JobResult {
	scriptPath := find51JobManageScript()
	if scriptPath == "" {
		return JobManage51JobResult{
			Status:  "error",
			Message: "未找到 51job 职位管理抓取脚本 (51job_job_manage_sync.js)",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "node", scriptPath)
	hideConsoleWindow(cmd)

	nodePath := filepath.Join(filepath.Dir(scriptPath), "..", "node_modules")
	if abs, err := filepath.Abs(nodePath); err == nil {
		cmd.Env = append(os.Environ(), "NODE_PATH="+abs)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		var parsed JobManage51JobResult
		if json.Unmarshal(stdout.Bytes(), &parsed) == nil && parsed.Message != "" {
			return parsed
		}
		return JobManage51JobResult{
			Status:  "error",
			Message: fmt.Sprintf("执行抓取脚本失败: %s", errMsg),
		}
	}

	var result JobManage51JobResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return JobManage51JobResult{
			Status:  "error",
			Message: fmt.Sprintf("解析职位管理数据失败: %v", err),
		}
	}

	return result
}

// Sync51JobJobManagementCities 抓取 51job [职位管理] 岗位城市并批量同步至本地对应项目
func (a *App) Sync51JobJobManagementCities() SyncJobCitiesResult {
	fetchRes := a.Fetch51JobManagementJobs()
	if fetchRes.Status != "success" || len(fetchRes.Jobs) == 0 {
		msg := fetchRes.Message
		if msg == "" {
			msg = "51job [职位管理] 暂未发现可抓取的岗位，请确认页面已登录并打开职位管理列表"
		}
		return SyncJobCitiesResult{
			Success: false,
			Message: msg,
			Jobs:    fetchRes.Jobs,
		}
	}

	projects := a.GetProjects()
	updatedCount := 0
	var matchedLogs []string

	for _, p := range projects {
		pTitle := strings.TrimSpace(p.JobConfig.Title)
		if pTitle == "" {
			pTitle = strings.TrimSpace(p.Name)
		}
		if pTitle == "" {
			continue
		}

		cleanPTitle := strings.ToLower(pTitle)
		cleanPTitle = strings.ReplaceAll(cleanPTitle, "（", "(")
		cleanPTitle = strings.ReplaceAll(cleanPTitle, "）", ")")

		var matchedJob *JobManage51JobItem
		for _, job := range fetchRes.Jobs {
			jTitle := strings.TrimSpace(job.Title)
			cleanJTitle := strings.ToLower(jTitle)
			cleanJTitle = strings.ReplaceAll(cleanJTitle, "（", "(")
			cleanJTitle = strings.ReplaceAll(cleanJTitle, "）", ")")

			if cleanPTitle == cleanJTitle || strings.Contains(cleanJTitle, cleanPTitle) || strings.Contains(cleanPTitle, cleanJTitle) {
				matchedJob = &job
				break
			}
		}

		if matchedJob != nil && matchedJob.City != "" {
			changed := false
			if p.JobConfig.City != matchedJob.City {
				oldCity := p.JobConfig.City
				if oldCity == "" {
					oldCity = "未设置"
				}
				p.JobConfig.City = matchedJob.City
				changed = true
				matchedLogs = append(matchedLogs, fmt.Sprintf("【%s】城市：%s ➔ %s", pTitle, oldCity, matchedJob.City))
			}
			if matchedJob.Salary != "" && (p.JobConfig.SalaryRange == "" || p.JobConfig.SalaryRange == "25-40K") {
				p.JobConfig.SalaryRange = matchedJob.Salary
				changed = true
			}
			if changed {
				a.saveProject(p)
				updatedCount++
			}
		}
	}

	msg := fmt.Sprintf("已成功从 51job [职位管理] 抓取 %d 个岗位信息，更新了 %d 个招聘项目的工作城市", len(fetchRes.Jobs), updatedCount)
	if updatedCount == 0 && len(matchedLogs) == 0 {
		msg = fmt.Sprintf("已成功从 51job [职位管理] 抓取 %d 个岗位，本地岗位城市已处于最新匹配状态", len(fetchRes.Jobs))
	}

	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "projects:updated", map[string]interface{}{
			"updatedCount": updatedCount,
		})
	}

	return SyncJobCitiesResult{
		Success:      true,
		UpdatedCount: updatedCount,
		Message:      msg,
		MatchedLogs:  matchedLogs,
		Jobs:         fetchRes.Jobs,
	}
}
