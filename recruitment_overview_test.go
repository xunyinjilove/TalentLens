package main

import (
	"fmt"
	"testing"
	"time"
)

func TestRecruitmentOverviewInterviewAndHireCounts(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stamp := now.UTC().Format(time.RFC3339Nano)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonth := monthStart.Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO projects(id,name,legacy_json,created_at,updated_at) VALUES('project','测试职位','{}',?,?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct{ id, stage string }{
		{"pending", "first_interview"}, {"scheduled", "second_interview"},
		{"hired_now", "hired"}, {"hired_before", "hired"},
	} {
		if _, err := db.Exec(`INSERT INTO candidates(id,display_name,created_at,updated_at) VALUES(?,?,?,?)`, entry.id, entry.id, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO project_candidates(id,project_id,candidate_id,stage,created_at,updated_at) VALUES(?,'project',?,?,?,?)`, entry.id, entry.id, entry.stage, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	for _, entry := range []struct{ candidateID, status string }{{"pending", "cancelled"}, {"scheduled", "scheduled"}} {
		if _, err := db.Exec(`INSERT INTO interviews(id,project_candidate_id,starts_at,ends_at,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
			fmt.Sprintf("interview_%s", entry.candidateID), entry.candidateID,
			now.Add(24*time.Hour).UTC().Format(time.RFC3339Nano), now.Add(25*time.Hour).UTC().Format(time.RFC3339Nano), entry.status, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	for _, entry := range []struct{ candidateID, at string }{{"hired_now", stamp}, {"hired_before", lastMonth}} {
		if _, err := db.Exec(`INSERT INTO follow_up_events(id,project_candidate_id,event_type,from_stage,to_stage,created_at) VALUES(?,?,'stage_change','offer','hired',?)`,
			fmt.Sprintf("event_%s", entry.candidateID), entry.candidateID, entry.at); err != nil {
			t.Fatal(err)
		}
	}
	dashboard, err := app.GetRecruitmentDashboard()
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.PendingInterviewArrangements != 1 || dashboard.HiresThisMonth != 1 {
		t.Fatalf("unexpected overview counts: pending=%d hires=%d", dashboard.PendingInterviewArrangements, dashboard.HiresThisMonth)
	}
	activities, err := app.ListRecentRecruitmentActivities(8)
	if err != nil {
		t.Fatal(err)
	}
	interviews, followUps := 0, 0
	for _, activity := range activities {
		if activity.Kind == "interview" {
			interviews++
		} else if activity.Kind == "followup" {
			followUps++
		}
	}
	if interviews != 2 || followUps != 2 {
		t.Fatalf("recent activities should include interviews and follow-ups: %+v", activities)
	}
}

func TestRecruitmentOverviewCountsCurrentPoolFirstEntries(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	old := time.Now().AddDate(0, 0, -15).UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"job1", "job2"} {
		if _, err := db.Exec(`INSERT INTO projects(id,name,legacy_json,created_at,updated_at) VALUES(?,?, '{}',?,?)`, id, id, now, now); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"new", "old", "archived", "unlinked"} {
		if _, err := db.Exec(`INSERT INTO candidates(id,display_name,created_at,updated_at) VALUES(?,?,?,?)`, id, id, now, now); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ id, job, candidate, created, archived string }{
		{"new1", "job1", "new", now, ""}, {"new2", "job2", "new", now, ""},
		{"old1", "job1", "old", old, ""}, {"old2", "job2", "old", now, ""},
		{"archived1", "job1", "archived", now, now},
	} {
		if _, err := db.Exec(`INSERT INTO project_candidates(id,project_id,candidate_id,stage,created_at,updated_at,archived_at) VALUES(?,?,?,'new',?,?,?)`, row.id, row.job, row.candidate, row.created, row.created, row.archived); err != nil {
			t.Fatal(err)
		}
	}
	dashboard, err := app.GetRecruitmentDashboard()
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.CandidatesThisWeek != 1 {
		t.Fatalf("recent unique current pool entrants = %d, want 1", dashboard.CandidatesThisWeek)
	}
}

func TestTodayTodosPersistAndToggle(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	item, err := app.AddTodayTodo("  预约候选人面试  ")
	if err != nil || item.Title != "预约候选人面试" {
		t.Fatalf("add todo: item=%+v err=%v", item, err)
	}
	if err := app.SetTodayTodoCompleted(item.ID, true); err != nil {
		t.Fatal(err)
	}
	items, err := app.ListTodayTodos()
	if err != nil || len(items) != 1 || !items[0].Completed {
		t.Fatalf("completed todo missing: items=%+v err=%v", items, err)
	}
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	yesterday := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec("UPDATE today_todos SET completed_at=? WHERE id=?", yesterday, item.ID); err != nil {
		t.Fatal(err)
	}
	items, err = app.ListTodayTodos()
	if err != nil || len(items) != 0 {
		t.Fatalf("older completed todo should be hidden: items=%+v err=%v", items, err)
	}
	if err := app.SetTodayTodoCompleted(item.ID, false); err != nil {
		t.Fatal(err)
	}
	items, err = app.ListTodayTodos()
	if err != nil || len(items) != 1 || items[0].Completed {
		t.Fatalf("unfinished todo missing: items=%+v err=%v", items, err)
	}
	if err := app.DeleteTodayTodo(item.ID); err != nil {
		t.Fatal(err)
	}
	items, err = app.ListTodayTodos()
	if err != nil || len(items) != 0 {
		t.Fatalf("deleted todo still visible: items=%+v err=%v", items, err)
	}
}
