package main

import (
	"strings"
	"testing"
)

func TestOutreachPreviewRequiresPendingRedlineSelectionAndRecordsManualOutcome(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("临床项目经理", "临床", 1, "HR", "", &JobConfig{Title: "临床项目经理", RedLines: []string{"有临床项目管理经历"}})
	for _, item := range []struct {
		id, name, fact string
		score          int
	}{{"candidate_a", "甲", "任职履历快照：主导多中心临床项目执行", 83}, {"candidate_b", "乙", "任职履历快照：负责临床试验预算和质量管理", 81}} {
		resume := &Resume{ID: item.id, ProjectID: project.ID, FileName: item.name, Content: item.fact, Status: "done", Score: item.score,
			Analysis: &AnalysisResult{CandidateName: item.name, AbilityScore: item.score, CoreMatch: float64(item.score), RedLineStatus: "pending", RedLineChecks: []RedLineCheck{{Criterion: "有临床项目管理经历", Status: "unknown"}}}}
		if err := app.persistSearchCandidate(resume); err != nil {
			t.Fatal(err)
		}
	}
	batch, err := app.PreviewTopTenOutreach(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Items) != 2 {
		t.Fatalf("items %+v", batch.Items)
	}
	if batch.Items[0].Message == batch.Items[1].Message || !strings.Contains(batch.Items[0].Message, "临床项目") {
		t.Fatalf("messages are not individualized: %+v", batch.Items)
	}
	choices := make([]OutreachApproval, 0, 2)
	for _, item := range batch.Items {
		choices = append(choices, OutreachApproval{ItemID: item.ID, Message: item.Message, Include: true})
	}
	if _, err = app.ConfirmOutreachBatch(batch.ID, choices); err == nil {
		t.Fatal("pending redline must require explicit selection")
	}
	for i := range choices {
		choices[i].ConfirmPending = true
	}
	confirmed, err := app.ConfirmOutreachBatch(batch.ID, choices)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range confirmed.Items {
		if item.Status != "needs_manual" {
			t.Fatalf("unverified platform send must stay manual: %+v", item)
		}
	}
	if err := app.RecordOutreachResult(confirmed.Items[0].ID, "sent", "平台会话 123 / 15:00", "HR"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ConfirmOutreachBatch(batch.ID, choices); err == nil {
		t.Fatal("confirmed batch should be one-shot")
	}
}

func TestArchivedCandidateCannotBeConfirmedFromOldOutreachPreview(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("Synthetic role", "Test", 1, "HR", "", &JobConfig{Title: "Synthetic role"})
	resume := &Resume{ID: "archive_outreach", ProjectID: project.ID, FileName: "synthetic.txt", Content: "synthetic experience", Status: "done", Score: 80,
		Analysis: &AnalysisResult{CandidateName: "Synthetic candidate", AbilityScore: 80, CoreMatch: 80, RedLineStatus: "passed"}}
	if err := app.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	batch, err := app.PreviewTopTenOutreach(project.ID)
	if err != nil || len(batch.Items) != 1 {
		t.Fatalf("preview: %d, %v", len(batch.Items), err)
	}
	if _, err := app.ArchiveTalentCandidates([]string{batch.Items[0].ProjectCandidateID}); err != nil {
		t.Fatal(err)
	}
	_, err = app.ConfirmOutreachBatch(batch.ID, []OutreachApproval{{ItemID: batch.Items[0].ID, Include: true, Message: "Synthetic greeting"}})
	if err == nil || !strings.Contains(err.Error(), "已从人才库删除") {
		t.Fatalf("archived candidate must not remain eligible: %v", err)
	}
}

func TestOutreachQueuesOnlyVerified51JobDetailIdentity(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("Synthetic role", "Test", 1, "HR", "", &JobConfig{Title: "Synthetic role"})
	for _, candidate := range []struct{ id, url string }{
		{"verified", "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123456"},
		{"legacy", "https://ehire.51job.com/Candidate/ResumeView.aspx?hidUserID=987654"},
	} {
		resume := &Resume{ID: candidate.id, ProjectID: project.ID, FileName: candidate.id, SourcePlatform: "51job", URL: candidate.url,
			Content: "Synthetic project experience", Status: "done", Score: 80,
			Analysis: &AnalysisResult{CandidateName: candidate.id, AbilityScore: 80, CoreMatch: 80, RedLineStatus: "passed"}}
		if err := app.persistSearchCandidate(resume); err != nil {
			t.Fatal(err)
		}
	}
	batch, err := app.PreviewTopTenOutreach(project.ID)
	if err != nil || len(batch.Items) != 2 {
		t.Fatalf("preview: items=%d err=%v", len(batch.Items), err)
	}
	approvals := make([]OutreachApproval, 0, len(batch.Items))
	for _, item := range batch.Items {
		approvals = append(approvals, OutreachApproval{ItemID: item.ID, Include: true, Message: item.Message})
	}
	confirmed, err := app.ConfirmOutreachBatch(batch.ID, approvals)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range confirmed.Items {
		if item.ResumeID == "verified" && (item.Status != "queued" || item.PlatformCandidateID != "123456") {
			t.Fatalf("verified detail must queue with exact ID: %+v", item)
		}
		if item.ResumeID == "legacy" && item.Status != "needs_manual" {
			t.Fatalf("legacy URL must not be guessed into a sendable ID: %+v", item)
		}
		if item.ResumeID == "legacy" {
			if _, err := app.VerifyOutreachCandidateDetail(item.ID, "https://example.com/Revision/talent/resume/detail?resumeId=987654"); err == nil {
				t.Fatal("untrusted detail URL must not be bound to a candidate")
			}
		}
	}
}
