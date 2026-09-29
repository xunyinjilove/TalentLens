package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SetReviewedRedLine 允许 HR 在核对简历原文或初面回答后留下明确结论和依据。
func (a *App) SetReviewedRedLine(resumeID string, criterion string, status string, evidence string) (*Resume, error) {
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
	checks := resume.Analysis.RedLineChecks
	found := false
	for i := range checks {
		if checks[i].Criterion == criterion {
			checks[i].Status, checks[i].Evidence = status, evidence
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("岗位红线不存在")
	}

	analysis := resume.Analysis
	analysis.RedLineViolations = nil
	analysis.OverallScore = math.Round(analysis.CoreMatch*0.6 + analysis.BonusMatch*0.4)
	switch {
	case analysis.OverallScore >= 85:
		analysis.Recommendation = "strong_recommend"
	case analysis.OverallScore >= 70:
		analysis.Recommendation = "recommend"
	case analysis.OverallScore >= 55:
		analysis.Recommendation = "consider"
	default:
		analysis.Recommendation = "not_recommend"
	}
	missing, violated := false, false
	for _, check := range checks {
		if check.Status == "violated" {
			violated = true
			analysis.RedLineViolations = append(analysis.RedLineViolations, check.Criterion+"："+check.Evidence)
		} else if check.Status != "met" {
			missing = true
		}
	}
	if violated {
		analysis.RedLineStatus, analysis.Recommendation = "failed", "not_recommend"
		analysis.OverallScore = math.Min(analysis.OverallScore, 49)
	} else if missing {
		analysis.RedLineStatus = "pending"
		analysis.OverallScore = math.Min(analysis.OverallScore, 69)
		if analysis.Recommendation == "recommend" || analysis.Recommendation == "strong_recommend" {
			analysis.Recommendation = "consider"
		}
	} else {
		analysis.RedLineStatus = "passed"
	}
	analysis.ManagerPitch = a.generateFallbackManagerPitch(analysis, &resume)
	if analysis.RedLineStatus == "failed" {
		analysis.ManagerPitch = "【触碰岗位红线，需人工核验】\n" + analysis.ManagerPitch
	} else if analysis.RedLineStatus == "pending" {
		analysis.ManagerPitch = "【岗位红线待核实，暂勿作为通过人选推介】\n" + analysis.ManagerPitch
	}
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
