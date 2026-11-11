package backend

import (
	"errors"
	"testing"
	"time"
)

func TestConnectTencentMeetingDoesNotReloginAuthorizedAccount(t *testing.T) {
	loginCalls := 0
	current := func() TencentMeetingConnection {
		return TencentMeetingConnection{Installed: true, LoggedIn: true, Message: "已授权"}
	}
	result, err := connectTencentMeeting(current, func() error {
		loginCalls++
		return errors.New("user has been login")
	})
	if err != nil || !result.LoggedIn || loginCalls != 0 {
		t.Fatalf("authorized account was relogged: state=%+v, calls=%d, err=%v", result, loginCalls, err)
	}
}

func TestConnectTencentMeetingChecksAuthorizationAfterLogin(t *testing.T) {
	checks := 0
	status := func() TencentMeetingConnection {
		checks++
		return TencentMeetingConnection{Installed: true, LoggedIn: checks > 1}
	}
	result, err := connectTencentMeeting(status, func() error { return errors.New("login process exited after authorization") })
	if err != nil || !result.LoggedIn || checks != 2 {
		t.Fatalf("completed authorization was reported as failure: state=%+v, checks=%d, err=%v", result, checks, err)
	}
}

func TestConnectTencentMeetingReportsUnfinishedAuthorization(t *testing.T) {
	status := func() TencentMeetingConnection { return TencentMeetingConnection{Installed: true} }
	_, err := connectTencentMeeting(status, func() error { return nil })
	if err == nil {
		t.Fatal("login without an authorized status must not be reported as connected")
	}
}

func TestTelescopeMissingMustSkillNeverBecomesUnfit(t *testing.T) {
	checks := []RuleCheckItem{{Rule: "TELESCOPE_MUST_test", Status: "unknown"}}
	if got := telescopeVerdict("qualified", checks); got != "pending_verification" {
		t.Fatalf("missing evidence became %s", got)
	}
	checks[0].Rule = "TELESCOPE_REF_test"
	if got := telescopeVerdict("qualified", checks); got != "qualified" {
		t.Fatalf("reference affected verdict: %s", got)
	}
	checks[0].Status = "fail"
	if got := telescopeVerdict("qualified", checks); got != "qualified" {
		t.Fatalf("reference failure affected verdict: %s", got)
	}
}

func TestTelescopeKeywordAvoidsNegativeAndSubstring(t *testing.T) {
	for _, raw := range []string{"没有医院渠道开发经历", "logo design"} {
		term := "医院渠道开发经历"
		if raw == "logo design" {
			term = "Go"
		}
		if evidence, ok := telescopeKeywordEvidence(raw, term); ok {
			t.Fatalf("false keyword pass: %q", evidence)
		}
	}
	if _, ok := telescopeKeywordEvidence("负责 Go 微服务研发", "Go"); !ok {
		t.Fatal("standalone skill was not detected")
	}
}

func TestTelescopeConditionsPersistPerProject(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	project := a.CreateProject("测试岗位", "", 1, "", "", &JobConfig{Title: "测试岗位", RequiredSkills: []string{"Go"}})
	initial, err := a.GetTelescopeConditionConfig(project.ID)
	if err != nil || len(initial.Conditions) != 1 || initial.Conditions[0].Text != "Go" {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	initial.Conditions = append(initial.Conditions, TelescopeCondition{Text: "医院渠道开发", Kind: "other", Required: false, Source: "custom"})
	saved, err := a.SaveTelescopeConditionConfig(*initial)
	if err != nil || len(saved.Conditions) != 2 || saved.Version == initial.Version {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	loaded, err := a.GetTelescopeConditionConfig(project.ID)
	if err != nil || len(loaded.Conditions) != 2 || loaded.Version != saved.Version {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
	project.JobConfig.RequiredSkills = []string{"Rust"}
	if err := a.UpdateProject(project); err != nil {
		t.Fatal(err)
	}
	changed, err := a.GetTelescopeConditionConfig(project.ID)
	if err != nil || !changed.JobSkillsChanged {
		t.Fatalf("changed=%+v err=%v", changed, err)
	}
	changed.Conditions = append(changed.Conditions, TelescopeCondition{Text: "渠道管理", Kind: "other", Required: true, Source: "custom"})
	stillChanged, err := a.SaveTelescopeConditionConfig(*changed)
	if err != nil || !stillChanged.JobSkillsChanged {
		t.Fatalf("saving custom conditions silently acknowledged job skill change: %+v %v", stillChanged, err)
	}
	synced, err := a.SyncTelescopeJobSkills(project.ID)
	if err != nil || synced.JobSkillsChanged || len(synced.Conditions) != 3 {
		t.Fatalf("synced=%+v err=%v", synced, err)
	}
}

func TestTencentMeetingWithoutLoginKeepsPendingLocalSchedule(t *testing.T) {
	t.Setenv("TALENTLENS_TMEET_BIN", t.TempDir()+"/missing-tmeet.exe")
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	project := a.CreateProject("测试岗位", "", 1, "", "", nil)
	resume := &Resume{ID: "meeting_resume", ProjectID: project.ID, FileName: "简历.txt", Content: "synthetic", CreatedAt: time.Now()}
	if err := a.persistSearchCandidate(resume); err != nil {
		t.Fatal(err)
	}
	people, err := a.ListTalentCandidates(project.ID)
	if err != nil || len(people) != 1 {
		t.Fatalf("people=%d err=%v", len(people), err)
	}
	start := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	item := RecruitmentInterview{ProjectCandidateID: people[0].ProjectCandidateID, StartsAt: start.Format(time.RFC3339), EndsAt: start.Add(time.Hour).Format(time.RFC3339), Method: "腾讯会议自动生成"}
	id, err := a.SaveRecruitmentInterview(item)
	if err != nil {
		t.Fatal(err)
	}
	item.ID = id
	if _, err = a.SaveRecruitmentInterview(item); err != nil {
		t.Fatal(err)
	}
	list, err := a.ListRecruitmentInterviews(project.ID)
	if err != nil || len(list) != 1 || list[0].MeetingSyncStatus != "pending_create" || list[0].MeetingURL != "" {
		t.Fatalf("meeting=%+v err=%v", list, err)
	}
}

func TestTencentCLIReceiptRequiresRealLink(t *testing.T) {
	parsed := parseTMeetMeetings([]byte(`{"data":{"meeting_info_list":[{"meeting_id":"123","meeting_code":"456","join_url":"https://meeting.tencent.com/dm/example"}]}}`))
	if len(parsed) != 1 || parsed[0].ID != "123" || parsed[0].URL == "" {
		t.Fatalf("parsed=%+v", parsed)
	}
	if len(parseTMeetMeetings([]byte(`{"data":{"meeting_info_list":[]}}`))) != 0 {
		t.Fatal("empty receipt must not be treated as created")
	}
}
