package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RadarRecheckChange struct {
	Criterion   string `json:"criterion"`
	Before      string `json:"before"`
	After       string `json:"after"`
	Evidence    string `json:"evidence"`
	Significant bool   `json:"significant"`
}

type RadarRecheckPreview struct {
	ResumeID        string               `json:"resumeId"`
	DraftID         string               `json:"draftId"`
	OldScore        int                  `json:"oldScore"`
	NewScore        int                  `json:"newScore"`
	OldCoverage     float64              `json:"oldCoverage"`
	NewCoverage     float64              `json:"newCoverage"`
	NewRankEligible bool                 `json:"newRankEligible"`
	Significant     bool                 `json:"significant"`
	Changes         []RadarRecheckChange `json:"changes"`
}

type radarRecheckSavedDraft struct {
	Preview     RadarRecheckPreview `json:"preview"`
	ResumeHash  string              `json:"resumeHash"`
	ProfileHash string              `json:"profileHash"`
	Analysis    AnalysisResult      `json:"analysis"`
}

func checkRadarLabel(check CoreCriterionCheck) string {
	if check.EvidenceStatus == "known" && strings.TrimSpace(check.Evidence) != "" {
		return fmt.Sprintf("%d/4", check.Level)
	}
	return "待核实"
}

// PreviewResumeAIRecheck performs one HR-triggered evidence request. It saves a
// separate proposal and leaves the active evaluation unchanged.
func (a *App) PreviewResumeAIRecheck(resumeID string) (RadarRecheckPreview, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return RadarRecheckPreview{}, fmt.Errorf("简历编号无效")
	}
	if a.config.AI.APIKey == "" || a.config.AI.BaseURL == "" {
		return RadarRecheckPreview{}, fmt.Errorf("请先配置 AI 模型服务")
	}
	path := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return RadarRecheckPreview{}, err
	}
	var resume Resume
	if err = json.Unmarshal(data, &resume); err != nil {
		return RadarRecheckPreview{}, err
	}
	if resume.Status != "done" || resume.Analysis == nil {
		return RadarRecheckPreview{}, fmt.Errorf("只有已完成的测评可以复核")
	}
	project := a.GetProject(resume.ProjectID)
	if project == nil {
		return RadarRecheckPreview{}, fmt.Errorf("招聘项目不存在")
	}
	metrics := getUnifiedJobMetrics(&project.JobConfig)
	if len(metrics) == 0 {
		return RadarRecheckPreview{}, fmt.Errorf("岗位尚无技能指标，请先确认雷达草稿")
	}
	if len(project.JobConfig.RadarMetrics) == 0 {
		return RadarRecheckPreview{}, fmt.Errorf("请先由 HR 确认岗位雷达指标")
	}
	if project.JobConfig.RadarSourceHash != "" && project.JobConfig.RadarSourceHash != jobRadarSourceHash(&project.JobConfig) {
		return RadarRecheckPreview{}, fmt.Errorf("岗位资料已经修改，请先确认新版雷达指标")
	}
	profileHash := activeJobRadarHash(&project.JobConfig)
	old := *resume.Analysis
	details, err := a.analyzeCompetencies(&resume, &project.JobConfig, &a.config.AI)
	if err != nil {
		return RadarRecheckPreview{}, err
	}
	proposed := old
	proposed.CoreCriterionChecks = details.CoreCriterionChecks
	proposed.ExperienceCompetencies = details.ExperienceCompetencies
	proposed.CompetencyStatus = details.CompetencyStatus
	proposed.CompetencyMessage = details.CompetencyMessage
	proposed.RadarProfileHash = profileHash
	proposed.RadarStale = false
	a.applyJobDecision(&proposed, &project.JobConfig, &resume)
	proposed.AnalyzedAt = time.Now().Format(time.RFC3339)
	preview := RadarRecheckPreview{ResumeID: resumeID, DraftID: fmt.Sprintf("radar_%d", time.Now().UnixNano()), OldScore: resume.Score, NewScore: proposed.AbilityScore, OldCoverage: old.RadarCoverage, NewCoverage: proposed.RadarCoverage, NewRankEligible: proposed.RankEligible, Changes: []RadarRecheckChange{}}
	oldChecks := map[string]CoreCriterionCheck{}
	for _, check := range old.CoreCriterionChecks {
		oldChecks[strings.TrimSpace(check.Criterion)] = check
	}
	for _, check := range proposed.CoreCriterionChecks {
		before, exists := oldChecks[check.Criterion]
		change := RadarRecheckChange{Criterion: check.Criterion, Before: "旧版未评估", After: checkRadarLabel(check), Evidence: check.Evidence}
		if exists {
			change.Before = checkRadarLabel(before)
		}
		change.Significant = exists && before.EvidenceStatus == "known" && check.EvidenceStatus == "known" && int(math.Abs(float64(before.Level-check.Level))) >= 2
		if change.Significant || change.Before != change.After {
			preview.Significant = true
		}
		preview.Changes = append(preview.Changes, change)
	}
	if math.Abs(float64(preview.NewScore-preview.OldScore)) > 3 || (old.RadarCoverage < .8) != (proposed.RadarCoverage < .8) || !analysisMatchesRadar(&old, metrics, profileHash) {
		preview.Significant = true
	}
	resumeHash := sha256.Sum256(data)
	saved := radarRecheckSavedDraft{Preview: preview, ResumeHash: fmt.Sprintf("%x", resumeHash[:]), ProfileHash: profileHash, Analysis: proposed}
	dir := filepath.Join(a.getDataDir(), "radar_recheck_drafts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return RadarRecheckPreview{}, err
	}
	payload, err := json.Marshal(saved)
	if err != nil {
		return RadarRecheckPreview{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, preview.DraftID+".json"), payload, 0600); err != nil {
		return RadarRecheckPreview{}, err
	}
	return preview, nil
}

// ConfirmResumeAIRecheck activates only the exact preview the HR inspected.
func (a *App) ConfirmResumeAIRecheck(resumeID, draftID string) (*Resume, error) {
	if !interviewIDPattern.MatchString(resumeID) || !interviewIDPattern.MatchString(draftID) {
		return nil, fmt.Errorf("复核编号无效")
	}
	draftPath := filepath.Join(a.getDataDir(), "radar_recheck_drafts", draftID+".json")
	draftData, err := os.ReadFile(draftPath)
	if err != nil {
		return nil, err
	}
	var saved radarRecheckSavedDraft
	if err := json.Unmarshal(draftData, &saved); err != nil {
		return nil, err
	}
	if saved.Preview.ResumeID != resumeID || saved.Preview.DraftID != draftID {
		return nil, fmt.Errorf("复核草稿与简历不匹配")
	}
	resumePath := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")
	data, err := os.ReadFile(resumePath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if saved.ResumeHash != fmt.Sprintf("%x", sum[:]) {
		return nil, fmt.Errorf("简历或 HR 结论已有变化，请重新复核")
	}
	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return nil, err
	}
	project := a.GetProject(resume.ProjectID)
	if project == nil || saved.ProfileHash != activeJobRadarHash(&project.JobConfig) {
		return nil, fmt.Errorf("岗位指标已有变化，请重新复核")
	}
	historyDir := filepath.Join(a.getDataDir(), "radar_recheck_history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(historyDir, draftID+"-before.json"), data, 0600); err != nil {
		return nil, err
	}
	resume.Analysis = &saved.Analysis
	resume.Score = saved.Analysis.AbilityScore
	if err := a.save51JobContactResume(&resume); err != nil {
		return nil, err
	}
	_ = os.Remove(draftPath)
	return &resume, nil
}
