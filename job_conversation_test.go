package main

import (
	"testing"
	"time"
)

func Test51JobContactUsesExactCandidateIdentityAndPreventsDuplicateGreeting(t *testing.T) {
	if !is51JobCandidateDetailURL("https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123") ||
		is51JobCandidateDetailURL("https://ehire.51job.com.evil.test/Revision/talent/resume/detail?resumeId=123") ||
		is51JobCandidateDetailURL("https://ehire.51job.com/Revision/talent/search?resumeId=123") {
		t.Fatal("51job URL validation accepted wrong page or rejected candidate detail")
	}
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	r := &Resume{ID: "resume_123", FileName: "【前程无忧】程先生_临床项目经理.txt",
		URL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123", ContactStatus: "greeted"}
	a.saveResume(r)
	if name := conversationCandidateName(r); name != "程先生" {
		t.Fatalf("candidate name = %q", name)
	}
	result := a.Start51JobConversation(r.ID, "greet", "您好")
	if result.Status != "error" {
		t.Fatalf("duplicate greeting must fail without opening browser: %+v", result)
	}
}

func Test51JobCommunicationUsesTheCandidateProjectJobTitle(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	project := a.CreateProject("招聘项目 - 2026秋", "研发", 1, "HR", "", &JobConfig{Title: "软件测试工程师"})
	resume := &Resume{ProjectID: project.ID}
	if got := a.conversationJobTitle(resume); got != "软件测试工程师" {
		t.Fatalf("communication job title = %q", got)
	}
	if got := a.conversationJobTitle(&Resume{ProjectID: "missing"}); got != "" {
		t.Fatalf("missing project must not reuse another job: %q", got)
	}
}

func Test51JobHiChatQuoteIsSingleUseAndCannotSwitchJobs(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	project := a.CreateProject("测试招聘", "研发", 1, "HR", "", &JobConfig{Title: "软件测试工程师"})
	resume := &Resume{ID: "resume_456", ProjectID: project.ID, FileName: "【前程无忧】测试候选人.txt",
		URL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=456"}
	a.saveResume(resume)
	a.contactPending = map[string]pending51JobHiQuote{
		"old":       {ResumeID: resume.ID, ProjectTitle: "软件测试工程师", JobTitle: "软件测试工程师", Expires: time.Now().Add(-time.Second)},
		"wrong-job": {ResumeID: resume.ID, ProjectTitle: "另一个岗位", JobTitle: "另一个岗位", Expires: time.Now().Add(time.Minute)},
	}
	if got := a.Confirm51JobHiChat(resume.ID, "old"); got.Status != "error" {
		t.Fatalf("expired quote must fail: %+v", got)
	}
	if got := a.Confirm51JobHiChat(resume.ID, "wrong-job"); got.Status != "error" {
		t.Fatalf("changed job must fail: %+v", got)
	}
	if _, exists := a.contactPending["wrong-job"]; exists {
		t.Fatal("confirmation token was not consumed")
	}
	if got := a.Confirm51JobHiChat(resume.ID, "wrong-job"); got.Status != "error" {
		t.Fatalf("replayed quote must fail: %+v", got)
	}
}

func Test51JobManualChoiceKeepsTheProjectJobSeparateFromThePlatformJob(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	project := a.CreateProject("测试招聘", "研发", 1, "HR", "", &JobConfig{Title: "软件测试工程师"})
	resume := &Resume{ID: "resume_999", ProjectID: project.ID}
	quote := a.store51JobHiQuote(resume, JobConversationResult{
		Status: "quote", JobID: "job-202", JobTitle: "测试工程师（医疗软件）", TargetID: "tab-1", Points: 2,
	})
	if quote.Status != "needs_confirmation" || quote.QuoteID == "" {
		t.Fatalf("manual job quote missing: %+v", quote)
	}
	pending := a.contactPending[quote.QuoteID]
	if pending.ProjectTitle != "软件测试工程师" || pending.JobTitle != "测试工程师（医疗软件）" || pending.JobID != "job-202" {
		t.Fatalf("project and platform positions were mixed: %+v", pending)
	}
}

func Test51JobHiChatDoesNotPreviewWithoutProjectTitle(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	resume := &Resume{ID: "resume_789", FileName: "【前程无忧】测试候选人.txt",
		URL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=789"}
	a.saveResume(resume)
	if got := a.Prepare51JobHiChat(resume.ID); got.Status != "needs_job_selection" {
		t.Fatalf("missing project title must stop before opening browser: %+v", got)
	}
}
