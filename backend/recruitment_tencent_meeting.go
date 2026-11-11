package backend

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TencentMeetingConnection struct {
	Installed bool   `json:"installed"`
	LoggedIn  bool   `json:"loggedIn"`
	Account   string `json:"account"`
	Message   string `json:"message"`
}

func tmeetBinary() (string, error) {
	if p := strings.TrimSpace(os.Getenv("TALENTLENS_TMEET_BIN")); p != "" {
		if _, e := os.Stat(p); e == nil {
			return p, nil
		}
		return "", fmt.Errorf("腾讯会议 CLI 路径不存在")
	}
	for _, p := range []string{filepath.Join(filepath.Dir(os.Args[0]), "bin", "tmeet.exe"), filepath.Join(filepath.Dir(os.Args[0]), "tmeet.exe"), filepath.Join("bin", "tmeet.exe")} {
		if _, e := os.Stat(p); e == nil {
			return p, nil
		}
	}
	return exec.LookPath("tmeet")
}

func runTMeet(timeout time.Duration, args ...string) ([]byte, error) {
	p, e := tmeetBinary()
	if e != nil {
		return nil, fmt.Errorf("未找到腾讯会议 CLI")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p, args...)
	hideConsoleWindow(cmd)
	out, e := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if e != nil {
		return out, fmt.Errorf("腾讯会议命令失败: %w", e)
	}
	return out, nil
}

func tmeetString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch x := v.(type) {
			case string:
				return x
			case float64:
				return fmt.Sprintf("%.0f", x)
			}
		}
	}
	return ""
}
func tmeetMaps(v interface{}) []map[string]interface{} {
	out := []map[string]interface{}{}
	var walk func(interface{})
	walk = func(n interface{}) {
		switch x := n.(type) {
		case string:
			var nested interface{}
			if json.Unmarshal([]byte(x), &nested) == nil {
				walk(nested)
			}
		case map[string]interface{}:
			out = append(out, x)
			for _, c := range x {
				walk(c)
			}
		case []interface{}:
			for _, c := range x {
				walk(c)
			}
		}
	}
	walk(v)
	return out
}

type tmeetMeeting struct{ ID, Code, URL, Subject string }

func parseTMeetMeetings(out []byte) []tmeetMeeting {
	var raw interface{}
	if json.Unmarshal(out, &raw) != nil {
		return nil
	}
	result := []tmeetMeeting{}
	for _, m := range tmeetMaps(raw) {
		id := tmeetString(m, "meeting_id", "meetingId")
		if id != "" {
			result = append(result, tmeetMeeting{ID: id, Code: tmeetString(m, "meeting_code", "meetingCode", "meeting_number", "meetingNumber", "code"), URL: tmeetString(m, "join_url", "joinUrl", "meeting_url", "meetingUrl"), Subject: tmeetString(m, "subject")})
		}
	}
	return result
}

func (a *App) GetTencentMeetingConnection() TencentMeetingConnection {
	if _, e := tmeetBinary(); e != nil {
		return TencentMeetingConnection{Message: "未安装腾讯会议 CLI"}
	}
	state := TencentMeetingConnection{Installed: true}
	out, e := runTMeet(15*time.Second, "auth", "status")
	if e != nil || strings.Contains(strings.ToLower(string(out)), "not logged in") {
		state.Message = "未登录或授权已失效"
		return state
	}
	var raw interface{}
	if json.Unmarshal(out, &raw) == nil {
		for _, m := range tmeetMaps(raw) {
			if id := tmeetString(m, "open_id", "openid", "openId"); id != "" {
				state.Account = id
				break
			}
		}
	}
	state.LoggedIn = true
	state.Message = "已授权"
	return state
}
func (a *App) LoginTencentMeeting() (TencentMeetingConnection, error) {
	return connectTencentMeeting(a.GetTencentMeetingConnection, func() error {
		_, err := runTMeet(6*time.Minute, "auth", "login")
		return err
	})
}

func connectTencentMeeting(status func() TencentMeetingConnection, login func() error) (TencentMeetingConnection, error) {
	current := status()
	if current.LoggedIn {
		return current, nil
	}
	err := login()
	current = status()
	if current.LoggedIn {
		return current, nil
	}
	if err != nil {
		return current, err
	}
	return current, fmt.Errorf("腾讯会议授权尚未完成，请检查浏览器授权结果")
}
func (a *App) LogoutTencentMeeting() (TencentMeetingConnection, error) {
	_, e := runTMeet(20*time.Second, "auth", "logout")
	return a.GetTencentMeetingConnection(), e
}

func localMeetingTime(value string) string {
	t, e := time.Parse(time.RFC3339, value)
	if e != nil {
		return value
	}
	return t.In(time.Local).Format(time.RFC3339)
}
func meetingTitle(db *sql.DB, item RecruitmentInterview) string {
	if s := strings.TrimSpace(item.MeetingSubject); s != "" {
		return s
	}
	var name, job, stage string
	_ = db.QueryRow(`SELECT c.display_name,p.job_title,pc.stage FROM project_candidates pc JOIN candidates c ON c.id=pc.candidate_id JOIN projects p ON p.id=pc.project_id WHERE pc.id=?`, item.ProjectCandidateID).Scan(&name, &job, &stage)
	if name == "" {
		name = "候选人"
	}
	if job == "" {
		job = "面试岗位"
	}
	round := "初面"
	if stage == "second_interview" {
		round = "复面"
	}
	return name + "－" + job + "－" + round
}
func meetingState(db *sql.DB, id, state, message string) error {
	_, e := db.Exec(`UPDATE interview_meetings SET sync_status=?,last_error=?,updated_at=? WHERE interview_id=?`, state, message, time.Now().UTC().Format(time.RFC3339Nano), id)
	return e
}
func saveMeetingReceipt(db *sql.DB, id string, m tmeetMeeting, host, start, end string) error {
	_, e := db.Exec(`UPDATE interview_meetings SET meeting_id=?,meeting_code=?,join_url=?,host_account=?,remote_starts_at=?,remote_ends_at=?,sync_status='synced',last_error='',updated_at=? WHERE interview_id=?`, m.ID, m.Code, m.URL, host, start, end, time.Now().UTC().Format(time.RFC3339Nano), id)
	return e
}

func (a *App) reconcileMeetingCreate(db *sql.DB, item RecruitmentInterview, subject string) error {
	if !a.GetTencentMeetingConnection().LoggedIn {
		return nil
	}
	start, e := time.Parse(time.RFC3339, item.StartsAt)
	if e != nil {
		return e
	}
	out, e := runTMeet(25*time.Second, "meeting", "search", "--query", subject, "--query-field", "subject", "--start", localMeetingTime(start.Add(-time.Minute).Format(time.RFC3339)), "--end", localMeetingTime(start.Add(time.Minute).Format(time.RFC3339)))
	if e != nil {
		return nil
	}
	matches := []tmeetMeeting{}
	for _, m := range parseTMeetMeetings(out) {
		if m.Subject == subject && m.URL != "" {
			matches = append(matches, m)
		}
	}
	if len(matches) == 1 {
		return saveMeetingReceipt(db, item.ID, matches[0], a.GetTencentMeetingConnection().Account, item.StartsAt, item.EndsAt)
	}
	return nil // An ambiguous create outcome never triggers a second create.
}

func (a *App) syncInterviewMeeting(db *sql.DB, item RecruitmentInterview) error {
	var meetingID, state, oldStart, oldEnd, subject string
	e := db.QueryRow(`SELECT meeting_id,sync_status,remote_starts_at,remote_ends_at,subject FROM interview_meetings WHERE interview_id=?`, item.ID).Scan(&meetingID, &state, &oldStart, &oldEnd, &subject)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if item.Method != "腾讯会议自动生成" || item.Status == "cancelled" {
		if meetingID != "" && state != "cancelled" {
			return a.cancelInterviewMeeting(db, item.ID)
		}
		return nil
	}
	if errors.Is(e, sql.ErrNoRows) {
		subject = meetingTitle(db, item)
		_, e = db.Exec(`INSERT INTO interview_meetings(interview_id,subject,sync_status,updated_at) VALUES(?,?,'pending_create',?)`, item.ID, subject, time.Now().UTC().Format(time.RFC3339Nano))
		if e != nil {
			return e
		}
		state = "pending_create"
	}
	oldSubject := subject
	if s := strings.TrimSpace(item.MeetingSubject); s != "" && s != subject {
		subject = s
		_, _ = db.Exec(`UPDATE interview_meetings SET subject=? WHERE interview_id=?`, subject, item.ID)
	}
	conn := a.GetTencentMeetingConnection()
	if meetingID == "" {
		if state == "creating_unknown" {
			return a.reconcileMeetingCreate(db, item, subject)
		}
		if !conn.LoggedIn {
			return meetingState(db, item.ID, "pending_create", "腾讯会议未登录，已保存本地安排")
		}
		if e = meetingState(db, item.ID, "creating_unknown", "建会请求已发出，等待回执"); e != nil {
			return e
		}
		out, runErr := runTMeet(45*time.Second, "meeting", "create", "--subject", subject, "--start", localMeetingTime(item.StartsAt), "--end", localMeetingTime(item.EndsAt))
		matches := parseTMeetMeetings(out)
		if runErr == nil && len(matches) == 1 && matches[0].URL != "" {
			return saveMeetingReceipt(db, item.ID, matches[0], conn.Account, item.StartsAt, item.EndsAt)
		}
		_ = meetingState(db, item.ID, "creating_unknown", "建会回执不明，须先对账；不会重复建会")
		return a.reconcileMeetingCreate(db, item, subject)
	}
	if oldStart == item.StartsAt && oldEnd == item.EndsAt && state == "synced" && oldSubject == subject {
		return nil
	}
	if !conn.LoggedIn {
		return meetingState(db, item.ID, "pending_update", "腾讯会议未登录，远端仍按原时间有效")
	}
	if _, e = runTMeet(45*time.Second, "meeting", "update", "--meeting-id", meetingID, "--subject", subject, "--start", localMeetingTime(item.StartsAt), "--end", localMeetingTime(item.EndsAt)); e != nil {
		return meetingState(db, item.ID, "pending_update", "远端改期未确认，原会议仍可能有效")
	}
	_, e = db.Exec(`UPDATE interview_meetings SET remote_starts_at=?,remote_ends_at=?,sync_status='synced',last_error='',updated_at=? WHERE interview_id=?`, item.StartsAt, item.EndsAt, time.Now().UTC().Format(time.RFC3339Nano), item.ID)
	return e
}

func (a *App) cancelInterviewMeeting(db *sql.DB, id string) error {
	var meetingID, state string
	e := db.QueryRow(`SELECT meeting_id,sync_status FROM interview_meetings WHERE interview_id=?`, id).Scan(&meetingID, &state)
	if errors.Is(e, sql.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	if meetingID == "" || state == "cancelled" {
		return nil
	}
	if !a.GetTencentMeetingConnection().LoggedIn {
		return meetingState(db, id, "pending_cancel", "腾讯会议未登录，原会议仍有效")
	}
	if _, e = runTMeet(45*time.Second, "meeting", "cancel", "--meeting-id", meetingID); e != nil {
		return meetingState(db, id, "pending_cancel", "远端取消未确认，原会议仍可能有效")
	}
	return meetingState(db, id, "cancelled", "")
}

func (a *App) RetryTencentMeetingSync(id string) error {
	db, e := a.getRecruitmentDB()
	if e != nil {
		return e
	}
	var item RecruitmentInterview
	e = db.QueryRow(`SELECT id,project_candidate_id,starts_at,ends_at,method,status FROM interviews WHERE id=?`, id).Scan(&item.ID, &item.ProjectCandidateID, &item.StartsAt, &item.EndsAt, &item.Method, &item.Status)
	if e != nil {
		return e
	}
	return a.syncInterviewMeeting(db, item)
}
