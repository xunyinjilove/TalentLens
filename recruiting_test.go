package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJobRedLineDecision(t *testing.T) {
	a := NewApp()
	job := &JobConfig{RedLines: []string{"必须有大分子临床经验"}}
	resume := &Resume{Content: "负责大分子临床I期项目三年；本科毕业"}
	cases := []struct {
		name       string
		checks     []RedLineCheck
		wantStatus string
	}{
		{"model omitted check", nil, "pending"},
		{"unsupported claim", []RedLineCheck{{Criterion: job.RedLines[0], Status: "met", Evidence: "主导十年大分子临床"}}, "pending"},
		{"relevant text still needs review", []RedLineCheck{{Criterion: job.RedLines[0], Status: "met", Evidence: "负责大分子临床I期项目三年"}}, "pending"},
		{"unrelated text must not reject", []RedLineCheck{{Criterion: job.RedLines[0], Status: "violated", Evidence: "本科毕业"}}, "pending"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := &AnalysisResult{CoreMatch: 90, BonusMatch: 80, RedLineChecks: tc.checks, RedLineViolations: []string{"unverified model text"}}
			a.applyJobDecision(result, job, resume)
			if result.RedLineStatus != tc.wantStatus || result.OverallScore != 90 {
				t.Fatalf("status=%s score=%v, wanted %s and unchanged ability score 90", result.RedLineStatus, result.OverallScore, tc.wantStatus)
			}
			if result.Recommendation == "recommend" || result.Recommendation == "strong_recommend" {
				t.Fatal("unknown red line must not recommend")
			}
			if len(result.RedLineViolations) != 0 {
				t.Fatal("unverified violation leaked into decision")
			}
		})
	}
}

func TestJobBonusDoesNotOverrideCoreAbility(t *testing.T) {
	a := NewApp()
	for _, tc := range []struct {
		name        string
		core, bonus float64
		bonusRules  []string
		wantScore   float64
		wantRec     string
	}{
		{"no bonus configured", 90, 0, nil, 90, "strong_recommend"},
		{"weak core cannot be rescued", 50, 100, []string{"项目加分"}, 50, "not_recommend"},
		{"strong core with bonus", 80, 100, []string{"项目加分"}, 90, "strong_recommend"},
		{"no bonus evidence", 80, 100, nil, 80, "recommend"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := &AnalysisResult{CoreMatch: tc.core, BonusMatch: tc.bonus}
			if tc.name != "no bonus evidence" && len(tc.bonusRules) > 0 {
				result.BonusMatches = []string{"项目成果有佐证"}
			}
			job := &JobConfig{BonusPoints: tc.bonusRules}
			if tc.name == "no bonus evidence" {
				job.BonusPoints = []string{"项目加分"}
			}
			a.applyJobDecision(result, job, &Resume{})
			if result.OverallScore != tc.wantScore || result.Recommendation != tc.wantRec {
				t.Fatalf("score=%v recommendation=%s, want %v %s", result.OverallScore, result.Recommendation, tc.wantScore, tc.wantRec)
			}
		})
	}
}

func TestInterviewJSON(t *testing.T) {
	var result struct {
		Score int `json:"score"`
	}
	if err := interviewJSON("```json\n{\"score\":85}\n```", &result); err != nil || result.Score != 85 {
		t.Fatalf("parse failed: %v, %+v", err, result)
	}
	if err := interviewJSON("not JSON", &result); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("bad result should fail: %v", err)
	}
}

func TestInterviewRoundTrip(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls++
		var content string
		switch {
		case strings.Contains(string(body), "已经追问过"):
			content = `{"score":88,"assessment":"回答给出项目细节","evidence":"完成一期试验","follow_up_question":""}`
		case strings.Contains(string(body), "follow_up_question"):
			content = `{"score":70,"assessment":"需核实","evidence":"一期试验","follow_up_question":"请说明具体职责"}`
		default:
			content = `{"summary":"项目经历已作答，仍需 HR 核实证据","recommendation":"review"}`
		}
		payload, _ := json.Marshal(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]string{"content": content}}}})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(payload))
	}))
	defer server.Close()
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	a.config.AI = AIConfig{BaseURL: server.URL, APIKey: "test", Model: "test", MaxRetries: 1}
	resume := &Resume{ID: "test_resume", FileName: "测试候选人", Analysis: &AnalysisResult{CandidateName: "测试候选人", InterviewQA: []InterviewQuestion{{Category: "项目", Question: "你做了什么？", ReferenceAnswer: "HR专用答案"}}}}
	a.saveResume(resume)
	session, err := a.StartInterview(resume.ID)
	if err != nil || len(session.Turns) != 1 || strings.Contains(session.Turns[0].Question, "HR专用答案") {
		t.Fatalf("start: %+v %v", session, err)
	}
	session, err = a.SubmitInterviewAnswer(resume.ID, 0, "完成一期试验")
	if err != nil || session.Turns[0].FollowUp == "" {
		t.Fatalf("follow up: %+v %v", session, err)
	}
	session, err = a.SubmitInterviewFollowUp(resume.ID, 0, "负责入组和数据核查")
	if err != nil || !session.Turns[0].Evaluated {
		t.Fatalf("evaluate: %+v %v", session, err)
	}
	session, err = a.CompleteInterview(resume.ID)
	if err != nil || session.Status != "completed" || session.OverallScore != 88 {
		t.Fatalf("complete: %+v %v", session, err)
	}
	session, err = a.ReviewInterview(resume.ID, "advance", "已核对")
	if err != nil || session.HRDecision != "advance" {
		t.Fatalf("review: %+v %v", session, err)
	}
	stored, err := a.GetInterview(resume.ID)
	if err != nil || stored.Turns[0].Answer != "完成一期试验" || stored.Status != "reviewed" || calls != 3 {
		t.Fatalf("persist: %+v %v, calls=%d", stored, err, calls)
	}
}

func TestHRCanResolvePendingRedLine(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	a.saveResume(&Resume{ID: "reviewed_resume", Analysis: &AnalysisResult{
		CoreMatch: 90, BonusMatch: 80, OverallScore: 69, RedLineStatus: "pending",
		RedLineChecks: []RedLineCheck{{Criterion: "必须有大分子临床经验", Status: "unknown"}},
	}})
	resume, err := a.SetReviewedRedLine("reviewed_resume", "必须有大分子临床经验", "met", "初面说明完成一期大分子试验")
	if err != nil || resume.Analysis.RedLineStatus != "passed" || resume.Score != 90 {
		t.Fatalf("review pass: %+v %v", resume, err)
	}
	resume, err = a.SetReviewedRedLine("reviewed_resume", "必须有大分子临床经验", "violated", "经核实仅有小分子经验")
	if err != nil || resume.Analysis.RedLineStatus != "failed" || resume.Score != 90 || resume.Analysis.Recommendation != "not_recommend" {
		t.Fatalf("review fail: %+v %v", resume, err)
	}
	if len(resume.Analysis.RedLineChecks[0].ReviewHistory) != 2 || resume.Analysis.RedLineChecks[0].ReviewedAt == "" {
		t.Fatal("manual review history and timestamp must be retained")
	}
	checks, err := a.GetResumeRedLineChecks("reviewed_resume")
	if err != nil || len(checks) != 1 || checks[0].Status != "violated" {
		t.Fatalf("saved HR verdict was not restored: %+v %v", checks, err)
	}
	freshResult := &AnalysisResult{CoreMatch: 90, RedLineChecks: []RedLineCheck{{Criterion: "必须有大分子临床经验", Status: "met"}}}
	a.applyJobDecision(freshResult, &JobConfig{RedLines: []string{"必须有大分子临床经验"}}, resume)
	if freshResult.RedLineStatus != "failed" || len(freshResult.RedLineChecks[0].ReviewHistory) != 2 {
		t.Fatalf("reanalysis must preserve HR decision and audit history: %+v", freshResult.RedLineChecks)
	}
}

func TestInterviewRedLineMustBeReviewedBeforeAdvance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		content := `{"score":80,"assessment":"已回答","evidence":"参与项目","follow_up_question":""}`
		if !strings.Contains(string(body), "follow_up_question") {
			content = `{"summary":"红线仍需人工核实","recommendation":"advance"}`
		}
		payload, _ := json.Marshal(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]string{"content": content}}}})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(payload))
	}))
	defer server.Close()
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	a.config.AI = AIConfig{BaseURL: server.URL, APIKey: "test", Model: "test", MaxRetries: 1}
	criterion := "必须有大分子临床经验"
	a.saveResume(&Resume{ID: "interview_redline", Analysis: &AnalysisResult{
		CoreMatch: 80, AbilityScore: 80, OverallScore: 80, RedLineStatus: "pending",
		RedLineChecks: []RedLineCheck{{RuleID: redLineRuleID(criterion), Criterion: criterion, Status: "unknown"}},
		InterviewQA:   []InterviewQuestion{{Category: "项目", Question: "介绍项目经历"}},
	}})
	session, err := a.StartInterview("interview_redline")
	if err != nil || len(session.Turns) != 2 || session.Turns[1].RuleID != redLineRuleID(criterion) {
		t.Fatalf("redline question not linked: %+v %v", session, err)
	}
	if _, err = a.SubmitInterviewAnswer("interview_redline", 0, "参与项目"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.SubmitInterviewAnswer("interview_redline", 1, "负责大分子临床一期试验"); err != nil {
		t.Fatal(err)
	}
	session, err = a.CompleteInterview("interview_redline")
	if err != nil || session.AIRecommendation != "review" {
		t.Fatalf("AI must not advance pending redline: %+v %v", session, err)
	}
	if _, err = a.ReviewInterview("interview_redline", "advance", ""); err == nil {
		t.Fatal("HR advance must wait for redline review")
	}
	if _, err = a.ReviewInterviewRedLine("interview_redline", redLineRuleID(criterion), "met", "无关内容"); err == nil {
		t.Fatal("evidence not in candidate answer must be rejected")
	}
	resume, err := a.ReviewInterviewRedLine("interview_redline", redLineRuleID(criterion), "met", "负责大分子临床一期试验")
	if err != nil || resume.Analysis.RedLineStatus != "passed" || resume.Analysis.RedLineChecks[0].ReviewHistory[0].Source != "interview_hr_review" {
		t.Fatalf("review not persisted: %+v %v", resume, err)
	}
	if _, err = a.ReviewInterview("interview_redline", "advance", "已核实原话"); err != nil {
		t.Fatal(err)
	}
}

func TestPersistSearchCandidateReportsWriteFailure(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	a.saveProject(&Project{ID: "proj_test"})
	resume := &Resume{ID: "boss_test_1", ProjectID: "proj_test", FileName: "候选人"}
	if err := os.WriteFile(filepath.Join(a.getDataDir(), "resumes"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.persistSearchCandidate(resume); err == nil {
		t.Fatal("write failure must not be reported as a saved candidate")
	}
	if err := os.Remove(filepath.Join(a.getDataDir(), "resumes")); err != nil {
		t.Fatal(err)
	}
	if err := a.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	if stored := a.GetProject("proj_test"); stored == nil || len(stored.ResumeIDs) != 1 || stored.ResumeIDs[0] != resume.ID {
		t.Fatalf("candidate missing from project: %+v", stored)
	}
}

func TestErrorLogDoesNotPersistCandidateMaterial(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	a.logError("张三的简历.pdf", "电话 13800138000", "候选人原文", "模型返回中的个人资料")
	files, err := filepath.Glob(filepath.Join(a.getDataDir(), "logs", "error_*.log"))
	if err != nil || len(files) != 1 {
		t.Fatalf("missing diagnostic log: %v %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, sensitive := range []string{"张三", "13800138000", "候选人原文", "模型返回中的个人资料"} {
		if strings.Contains(string(data), sensitive) {
			t.Fatalf("diagnostic log contains candidate material: %s", sensitive)
		}
	}
}
