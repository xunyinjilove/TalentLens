package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AddProjectReviewCriterion 允许 HR 为既有招聘项目补录规则；所有已入库候选人在下次读取时显示待核实。
func (a *App) AddProjectReviewCriterion(projectID, kind, criterion string) (*Project, error) {
	if !interviewIDPattern.MatchString(projectID) {
		return nil, fmt.Errorf("招聘项目 ID 无效")
	}
	criterion = strings.TrimSpace(criterion)
	if criterion == "" || len([]rune(criterion)) > 120 {
		return nil, fmt.Errorf("规则内容需为 1–120 字")
	}
	if kind != "redline" && kind != "bonus" {
		return nil, fmt.Errorf("规则类型无效")
	}
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	project := a.GetProject(projectID)
	if project == nil {
		return nil, fmt.Errorf("招聘项目不存在")
	}
	var rules *[]string
	if kind == "redline" {
		rules = &project.JobConfig.RedLines
	} else {
		rules = &project.JobConfig.BonusPoints
	}
	for _, existing := range *rules {
		if strings.EqualFold(strings.TrimSpace(existing), criterion) {
			return nil, fmt.Errorf("该项目已存在相同规则")
		}
	}
	if len(*rules) >= 20 {
		return nil, fmt.Errorf("每类规则最多 20 条，请先整理岗位要求")
	}
	*rules = append(*rules, criterion)
	project.UpdatedAt = time.Now()
	encoded, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(a.getProjectsDir(), project.ID+".json"), encoded, 0600); err != nil {
		return nil, err
	}
	return project, nil
}

// SetReviewedRedLine 允许 HR 在核对简历原文或初面回答后留下明确结论和依据。
func (a *App) SetReviewedRedLine(resumeID string, criterion string, status string, evidence string) (*Resume, error) {
	return a.setReviewedRedLine(resumeID, criterion, status, evidence, "hr_manual")
}

// GetResumeRedLineChecks 仅读取指定候选人的红线核实状态，供初面窗口恢复 HR 已保存的结论。
func (a *App) GetResumeRedLineChecks(resumeID string) ([]RedLineCheck, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return nil, fmt.Errorf("简历 ID 无效")
	}
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
	if err != nil {
		return nil, err
	}
	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return nil, err
	}
	if resume.Analysis == nil {
		return nil, fmt.Errorf("简历尚未分析")
	}
	if project := a.GetProject(resume.ProjectID); project != nil {
		a.prepareResumeDecision(&resume, &project.JobConfig)
	}
	return resume.Analysis.RedLineChecks, nil
}

// GetResumeBonusChecks 读取已保存的加分项核实记录。
func (a *App) GetResumeBonusChecks(resumeID string) ([]BonusCheck, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return nil, fmt.Errorf("简历 ID 无效")
	}
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
	if err != nil {
		return nil, err
	}
	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return nil, err
	}
	if resume.Analysis == nil {
		return nil, fmt.Errorf("简历尚未分析")
	}
	if project := a.GetProject(resume.ProjectID); project != nil {
		a.prepareResumeDecision(&resume, &project.JobConfig)
	}
	return resume.Analysis.BonusChecks, nil
}

// SetReviewedBonus 只记录 HR 对指定加分项的确认；AI 建议不能自行加分或改变排名。
func (a *App) SetReviewedBonus(resumeID, criterion, status, evidence string) (*Resume, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return nil, fmt.Errorf("简历 ID 无效")
	}
	if status != "met" && status != "violated" && status != "unknown" {
		return nil, fmt.Errorf("加分项结论无效")
	}
	evidence = strings.TrimSpace(evidence)
	if status != "unknown" && evidence == "" {
		return nil, fmt.Errorf("请填写人工核实依据")
	}
	if len([]rune(evidence)) > 1000 {
		return nil, fmt.Errorf("核实依据不能超过 1000 字")
	}
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	path := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return nil, err
	}
	if resume.Analysis == nil {
		return nil, fmt.Errorf("简历尚未分析")
	}
	if project := a.GetProject(resume.ProjectID); project != nil {
		a.prepareResumeDecision(&resume, &project.JobConfig)
	}
	found := false
	for i := range resume.Analysis.BonusChecks {
		check := &resume.Analysis.BonusChecks[i]
		if check.Criterion != criterion {
			continue
		}
		check.Status, check.Evidence = status, evidence
		check.RuleID = redLineRuleID(criterion)
		check.ReviewedAt = time.Now().Format(time.RFC3339)
		check.ReviewHistory = append(check.ReviewHistory, RedLineReviewEvent{Status: status, Evidence: evidence, Source: "hr_manual", ReviewedAt: check.ReviewedAt})
		found = true
		break
	}
	if !found {
		return nil, fmt.Errorf("岗位加分项不存在")
	}
	refreshBonusVerdict(resume.Analysis)
	resume.Analysis.ManagerPitch = a.buildReviewedManagerPitch(resume.Analysis, &resume)
	updated, err := json.MarshalIndent(&resume, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, updated, 0600); err != nil {
		return nil, err
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "analysis:completed", map[string]interface{}{"id": resumeID, "score": resume.Analysis.OverallScore, "analysis": resume.Analysis})
	}
	return &resume, nil
}

func (a *App) setReviewedRedLine(resumeID string, criterion string, status string, evidence string, source string) (*Resume, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return nil, fmt.Errorf("简历 ID 无效")
	}
	if status != "met" && status != "violated" && status != "unknown" {
		return nil, fmt.Errorf("红线结论无效")
	}
	evidence = strings.TrimSpace(evidence)
	if status != "unknown" && evidence == "" {
		return nil, fmt.Errorf("请填写核实依据")
	}
	if len([]rune(evidence)) > 1000 {
		return nil, fmt.Errorf("核实依据不能超过 1000 字")
	}

	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
	if err != nil {
		return nil, err
	}
	var resume Resume
	if err = json.Unmarshal(data, &resume); err != nil {
		return nil, err
	}
	if resume.Analysis == nil {
		return nil, fmt.Errorf("简历尚未分析")
	}
	if project := a.GetProject(resume.ProjectID); project != nil {
		a.prepareResumeDecision(&resume, &project.JobConfig)
	}
	checks := resume.Analysis.RedLineChecks
	found := false
	for i := range checks {
		if checks[i].Criterion == criterion {
			checks[i].Status, checks[i].Evidence = status, evidence
			checks[i].RuleID = redLineRuleID(criterion)
			checks[i].ReviewedAt = time.Now().Format(time.RFC3339)
			checks[i].ReviewHistory = append(checks[i].ReviewHistory, RedLineReviewEvent{
				Status: status, Evidence: evidence, Source: source, ReviewedAt: checks[i].ReviewedAt,
			})
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("岗位红线不存在")
	}

	analysis := resume.Analysis
	if analysis.AbilityScore == 0 && analysis.CoreMatch > 0 {
		analysis.AbilityScore = int(math.Round(analysis.CoreMatch))
	}
	refreshCandidateRecommendation(analysis)
	applyRedLineVerdict(analysis)
	analysis.ManagerPitch = a.buildReviewedManagerPitch(analysis, &resume)
	resume.Score = int(analysis.OverallScore)
	if resume.IsMergedAnalysis {
		resume.FinalScore = resume.Score
	} else {
		resume.InitialScore = resume.Score
	}
	updated, err := json.MarshalIndent(&resume, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"), updated, 0600); err != nil {
		return nil, err
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "analysis:completed", map[string]interface{}{"id": resumeID, "score": analysis.OverallScore, "analysis": analysis})
	}
	return &resume, nil
}
