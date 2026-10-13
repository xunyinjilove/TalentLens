package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) RetryCandidateAnalysis(resumeID string) error {
	if strings.TrimSpace(a.config.AI.APIKey) == "" {
		return fmt.Errorf("AI 未配置，请先在设置中填写 API Key")
	}
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
	if err != nil {
		return err
	}
	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return err
	}
	if resume.Status != "error" && resume.Status != "pending" && resume.Status != "pending_document" {
		return fmt.Errorf("当前简历状态不支持重试")
	}
	if resume.Status == "pending_document" {
		return fmt.Errorf("请先补全可信简历内容")
	}
	project := a.GetProject(resume.ProjectID)
	if project == nil {
		return fmt.Errorf("招聘项目不存在")
	}
	if err := a.ReAnalyzeResume(resumeID); err != nil {
		return err
	}
	db := a.activeRecruitmentDB()
	var jobID string
	if db != nil {
		_ = db.QueryRow("SELECT aj.id FROM analysis_jobs aj JOIN resume_versions rv ON rv.id=aj.resume_version_id WHERE rv.legacy_resume_id=? ORDER BY aj.created_at DESC LIMIT 1", resumeID).Scan(&jobID)
		if jobID != "" {
			_, _ = db.Exec("UPDATE analysis_jobs SET status='queued',error='',updated_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), jobID)
		}
	}
	cfg, job := a.config.AI, project.JobConfig
	go func() {
		sourcingAnalysisSlots <- struct{}{}
		defer func() { <-sourcingAnalysisSlots }()
		if db != nil && jobID != "" {
			_, _ = db.Exec("UPDATE analysis_jobs SET status='running',attempts=attempts+1,updated_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), jobID)
		}
		_, analysisErr := a.AnalyzeResume(resumeID, &cfg, &job)
		if db != nil && jobID != "" {
			status, detail := "completed", ""
			if analysisErr != nil {
				status, detail = "failed", analysisErr.Error()
			}
			_, _ = db.Exec("UPDATE analysis_jobs SET status=?,error=?,updated_at=? WHERE id=?", status, detail, time.Now().UTC().Format(time.RFC3339Nano), jobID)
		}
	}()
	return nil
}
