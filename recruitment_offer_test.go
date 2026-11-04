package main

import (
	"testing"
	"time"
)

func TestOfferStatusControlsHiredStage(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", nil)
	resume := &Resume{ID: "offer_resume", ProjectID: project.ID, FileName: "测试简历.txt", Content: "synthetic", CreatedAt: time.Now()}
	if err := app.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	people, err := app.ListTalentCandidates(project.ID)
	if err != nil || len(people) != 1 {
		t.Fatalf("candidates=%d err=%v", len(people), err)
	}
	id := people[0].ProjectCandidateID
	if err := app.UpdateCandidateFollowUp(id, "offer", "HR", "", "", "HR"); err != nil {
		t.Fatal(err)
	}
	card := OfferCard{ProjectCandidateID: id, SalaryPackage: "月薪 20K", WorkLocation: "上海", PlannedStartDate: "2026-12-01", ValidUntil: "2026-11-20", Owner: "HR"}
	if err := app.SaveOfferCard(card, "HR"); err != nil {
		t.Fatal(err)
	}
	if err := app.UpdateCandidateFollowUp(id, "hired", "HR", "", "", "HR"); err == nil {
		t.Fatal("draft offer allowed hiring")
	}
	if err := app.SetOfferStatus(id, "confirmed", "HR"); err != nil {
		t.Fatal(err)
	}
	if err := app.SetOfferStatus(id, "accepted", "HR"); err != nil {
		t.Fatal(err)
	}
	if err := app.UpdateCandidateFollowUp(id, "hired", "HR", "", "", "HR"); err != nil {
		t.Fatal(err)
	}
	history, err := app.ListOfferHistory(id)
	if err != nil || len(history) != 3 {
		t.Fatalf("history=%d err=%v", len(history), err)
	}
}

func TestPendingInterviewCountMatchesList(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", nil)
	resume := &Resume{ID: "pending_interview_resume", ProjectID: project.ID, FileName: "测试简历.txt", Content: "synthetic", CreatedAt: time.Now()}
	if err := app.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	people, err := app.ListTalentCandidates(project.ID)
	if err != nil || len(people) != 1 {
		t.Fatalf("candidates=%d err=%v", len(people), err)
	}
	id := people[0].ProjectCandidateID
	if err := app.UpdateCandidateFollowUp(id, "first_interview", "HR", "", "", "HR"); err != nil {
		t.Fatal(err)
	}
	list, err := app.ListPendingInterviewCandidates("")
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%d err=%v", len(list), err)
	}
	dashboard, err := app.GetRecruitmentDashboard()
	if err != nil || dashboard.PendingInterviewArrangements != len(list) {
		t.Fatalf("dashboard=%d list=%d err=%v", dashboard.PendingInterviewArrangements, len(list), err)
	}
	start := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	interview := RecruitmentInterview{ProjectCandidateID: id, StartsAt: start.Format(time.RFC3339), EndsAt: start.Add(time.Hour).Format(time.RFC3339)}
	firstID, err := app.SaveRecruitmentInterview(interview)
	if err != nil {
		t.Fatal(err)
	}
	retryID, err := app.SaveRecruitmentInterview(interview)
	if err != nil || retryID != firstID {
		t.Fatalf("retry created another interview: first=%s retry=%s err=%v", firstID, retryID, err)
	}
	list, err = app.ListPendingInterviewCandidates("")
	if err != nil || len(list) != 0 {
		t.Fatalf("scheduled list=%d err=%v", len(list), err)
	}
	dashboard, err = app.GetRecruitmentDashboard()
	if err != nil || dashboard.PendingInterviewArrangements != 0 {
		t.Fatalf("scheduled dashboard=%d err=%v", dashboard.PendingInterviewArrangements, err)
	}
}

func TestInterviewInviteAcceptsDatetimeLocalFollowUp(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", nil)
	resume := &Resume{ID: "invite_local_time", ProjectID: project.ID, FileName: "测试简历.txt", Content: "synthetic", CreatedAt: time.Now()}
	if err := app.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	people, err := app.ListTalentCandidates(project.ID)
	if err != nil || len(people) != 1 {
		t.Fatalf("candidates=%d err=%v", len(people), err)
	}
	id := people[0].ProjectCandidateID
	start := time.Now().Add(24 * time.Hour).Truncate(time.Minute)
	if _, err := app.SaveRecruitmentInterview(RecruitmentInterview{ProjectCandidateID: id, StartsAt: start.Format(time.RFC3339), EndsAt: start.Add(time.Hour).Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if err := app.UpdateCandidateFollowUp(id, "first_interview", "HR", start.Format("2006-01-02T15:04"), "已安排面试", "HR"); err != nil {
		t.Fatal(err)
	}
	people, err = app.ListTalentCandidates(project.ID)
	if err != nil || len(people) != 1 || people[0].Stage != "first_interview" {
		t.Fatalf("stage update failed: people=%+v err=%v", people, err)
	}
}

func TestLegacyRecruitmentStagesMigrateWithoutDroppingHistory(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	project := app.CreateProject("测试岗位", "", 1, "", "", nil)
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, stage string }{{"old_communicating", "communicating"}, {"old_hold", "on_hold"}, {"old_rejected", "rejected"}} {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err := db.Exec(`INSERT INTO candidates(id,display_name,created_at,updated_at) VALUES(?,?,?,?)`, item.id, item.id, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO project_candidates(id,project_id,candidate_id,stage,created_at,updated_at) VALUES(?,?,?,?,?,?)`, item.id, project.ID, item.id, item.stage, now, now); err != nil {
			t.Fatal(err)
		}
	}
	app.closeRecruitmentDB()
	db, err = app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	defer app.closeRecruitmentDB()
	rows, err := db.Query(`SELECT stage,archived_at FROM project_candidates ORDER BY stage`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	stages := map[string]int{}
	for rows.Next() {
		var stage, archived string
		if err := rows.Scan(&stage, &archived); err != nil {
			t.Fatal(err)
		}
		stages[stage]++
		if stage == "rejected" && archived == "" {
			t.Fatal("rejected candidate remained active")
		}
	}
	if stages["pending_contact"] != 1 || stages["new"] != 1 || stages["rejected"] != 1 {
		t.Fatalf("unexpected migrated stages: %v", stages)
	}
}

func TestInterviewRescheduleReplacesOriginalAndDeleteRemovesRecord(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := app.CreateProject("测试岗位", "", 1, "", "", nil)
	resume := &Resume{ID: "reschedule_resume", ProjectID: project.ID, FileName: "测试简历.txt", Content: "synthetic", CreatedAt: time.Now()}
	if err := app.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	people, err := app.ListTalentCandidates(project.ID)
	if err != nil || len(people) != 1 {
		t.Fatalf("candidates=%d err=%v", len(people), err)
	}
	pcID := people[0].ProjectCandidateID

	day1 := time.Date(2026, 11, 4, 2, 0, 0, 0, time.UTC)
	firstID, err := app.SaveRecruitmentInterview(RecruitmentInterview{
		ProjectCandidateID: pcID,
		StartsAt:           day1.Format(time.RFC3339),
		EndsAt:             day1.Add(time.Hour).Format(time.RFC3339),
		Interviewer:        "kk",
		Method:             "腾讯会议自动生成",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Re-booking / rescheduling to a new day without passing ID should update in place and preserve interviewer/method.
	day2 := time.Date(2026, 11, 8, 2, 0, 0, 0, time.UTC)
	secondID, err := app.SaveRecruitmentInterview(RecruitmentInterview{
		ProjectCandidateID: pcID,
		StartsAt:           day2.Format(time.RFC3339),
		EndsAt:             day2.Add(time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	if secondID != firstID {
		t.Fatalf("expected reschedule to reuse interview ID %s, got %s", firstID, secondID)
	}

	list, err := app.ListRecruitmentInterviews(project.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 interview after reschedule, got %d (err=%v)", len(list), err)
	}
	if list[0].StartsAt != day2.Format(time.RFC3339) || list[0].Interviewer != "kk" || list[0].Method != "腾讯会议自动生成" {
		t.Fatalf("rescheduled interview lost original fields: %+v", list[0])
	}

	// Deleting the interview should remove it completely.
	if err := app.DeleteRecruitmentInterview(firstID); err != nil {
		t.Fatalf("DeleteRecruitmentInterview failed: %v", err)
	}
	list, err = app.ListRecruitmentInterviews(project.ID)
	if err != nil || len(list) != 0 {
		t.Fatalf("expected 0 interviews after delete, got %d (err=%v)", len(list), err)
	}
}
