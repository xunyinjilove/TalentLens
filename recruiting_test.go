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
		wantMax    float64
	}{
		{"model omitted check", nil, "pending", 69},
		{"unsupported claim", []RedLineCheck{{Criterion: job.RedLines[0], Status: "met", Evidence: "主导十年大分子临床"}}, "pending", 69},
		{"supported match", []RedLineCheck{{Criterion: job.RedLines[0], Status: "met", Evidence: "负责大分子临床I期项目三年"}}, "passed", 100},
		{"supported violation", []RedLineCheck{{Criterion: job.RedLines[0], Status: "violated", Evidence: "本科毕业"}}, "failed", 49},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := &AnalysisResult{CoreMatch: 90, BonusMatch: 80, RedLineChecks: tc.checks, RedLineViolations: []string{"unverified model text"}}
			a.applyJobDecision(result, job, resume)
			if result.RedLineStatus != tc.wantStatus || result.OverallScore > tc.wantMax {
				t.Fatalf("status=%s score=%v, wanted %s <=%v", result.RedLineStatus, result.OverallScore, tc.wantStatus, tc.wantMax)
			}
			if tc.wantStatus == "failed" && result.Recommendation != "not_recommend" {
				t.Fatal("violation must reject")
			}
			if tc.wantStatus == "pending" && result.Recommendation == "recommend" {
				t.Fatal("unknown red line must not recommend")
			}
			if tc.wantStatus != "failed" && len(result.RedLineViolations) != 0 {
				t.Fatal("unverified violation leaked into decision")
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
	if err != nil || resume.Analysis.RedLineStatus != "passed" || resume.Score != 86 {
		t.Fatalf("review pass: %+v %v", resume, err)
	}
	resume, err = a.SetReviewedRedLine("reviewed_resume", "必须有大分子临床经验", "violated", "经核实仅有小分子经验")
	if err != nil || resume.Analysis.RedLineStatus != "failed" || resume.Score > 49 {
		t.Fatalf("review fail: %+v %v", resume, err)
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
