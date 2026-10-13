package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type TalentCandidate struct {
	ID                 string `json:"id"`
	ProjectCandidateID string `json:"projectCandidateId"`
	ProjectID          string `json:"projectId"`
	ProjectName        string `json:"projectName"`
	JobTitle           string `json:"jobTitle"`
	Name               string `json:"name"`
	Source             string `json:"source"`
	ResumeID           string `json:"resumeId"`
	ProfileURL         string `json:"profileUrl"`
	AnalysisStatus     string `json:"analysisStatus"`
	AbilityScore       int    `json:"abilityScore"`
	RedLineStatus      string `json:"redLineStatus"`
	ConfirmedBonus     int    `json:"confirmedBonus"`
	Stage              string `json:"stage"`
	Owner              string `json:"owner"`
	NextActionAt       string `json:"nextActionAt"`
	CreatedAt          string `json:"createdAt"`
}

type FollowUpRecord struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	FromStage string `json:"fromStage"`
	ToStage   string `json:"toStage"`
	Note      string `json:"note"`
	Actor     string `json:"actor"`
	CreatedAt string `json:"createdAt"`
}

type RecruitmentInterview struct {
	ID                 string `json:"id"`
	ProjectCandidateID string `json:"projectCandidateId"`
	CandidateName      string `json:"candidateName"`
	ProjectName        string `json:"projectName"`
	StartsAt           string `json:"startsAt"`
	EndsAt             string `json:"endsAt"`
	Interviewer        string `json:"interviewer"`
	Method             string `json:"method"`
	Location           string `json:"location"`
	Note               string `json:"note"`
	Status             string `json:"status"`
}

type RecruitmentDashboard struct {
	OpenProjects       int            `json:"openProjects"`
	CandidatesThisWeek int            `json:"candidatesThisWeek"`
	PendingFollowUps   int            `json:"pendingFollowUps"`
	UpcomingInterviews int            `json:"upcomingInterviews"`
	StageCounts        map[string]int `json:"stageCounts"`
}

var recruitmentStages = map[string]bool{
	"new": true, "pending_analysis": true, "pending_contact": true,
	"communicating": true, "first_interview": true, "second_interview": true,
	"offer": true, "hired": true, "on_hold": true, "rejected": true,
}

func recruitmentID(prefix string) string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(data[:])
}

// ListTalentCandidates is the shared data source for the talent pool and
// pipeline. The same person appears once per linked project so evaluations and
// follow-up stages cannot leak across positions.
func (a *App) ListTalentCandidates(projectID string) ([]TalentCandidate, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT c.id,pc.id,p.id,p.name,p.job_title,c.display_name,
		COALESCE(rv.legacy_resume_id,''),COALESCE(rv.raw_json,''),
		COALESCE(ev.status,''),COALESCE(ev.ability_score,0),
		COALESCE(ev.redline_status,''),COALESCE(ev.confirmed_bonus_count,0),
		pc.stage,pc.owner,pc.next_action_at,pc.created_at
		FROM project_candidates pc JOIN candidates c ON c.id=pc.candidate_id
		JOIN projects p ON p.id=pc.project_id
		LEFT JOIN resume_versions rv ON rv.id=(SELECT pcr.resume_version_id FROM project_candidate_resumes pcr
			JOIN resume_versions rv2 ON rv2.id=pcr.resume_version_id
			WHERE pcr.project_candidate_id=pc.id ORDER BY rv2.created_at DESC,rv2.id DESC LIMIT 1)
		LEFT JOIN evaluations ev ON ev.id=(SELECT id FROM evaluations
			WHERE project_candidate_id=pc.id ORDER BY created_at DESC,id DESC LIMIT 1)
		WHERE (?='' OR p.id=?) ORDER BY pc.created_at DESC,pc.id DESC`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]TalentCandidate, 0)
	for rows.Next() {
		var item TalentCandidate
		var raw string
		if err := rows.Scan(&item.ID, &item.ProjectCandidateID, &item.ProjectID,
			&item.ProjectName, &item.JobTitle, &item.Name, &item.ResumeID, &raw,
			&item.AnalysisStatus, &item.AbilityScore, &item.RedLineStatus,
			&item.ConfirmedBonus, &item.Stage, &item.Owner, &item.NextActionAt,
			&item.CreatedAt); err != nil {
			return nil, err
		}
		if raw != "" {
			var resume Resume
			if json.Unmarshal([]byte(raw), &resume) == nil {
				item.ProfileURL = resume.URL
				item.Source = recruitmentSourceFromURL(resume.URL)
				if item.AnalysisStatus == "" {
					item.AnalysisStatus = resume.Status
				}
			}
		}
		if item.Source == "" {
			item.Source = "本地简历"
		}
		if item.AnalysisStatus == "" {
			item.AnalysisStatus = "pending"
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func recruitmentSourceFromURL(url string) string {
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, "51job.com"):
		return "51job"
	case strings.Contains(lower, "zhipin.com"):
		return "BOSS直聘"
	case strings.Contains(lower, "zhaopin.com"):
		return "智联招聘"
	case strings.Contains(lower, "liepin.com"):
		return "猎聘"
	default:
		return ""
	}
}

func (a *App) GetRecruitmentDashboard() (RecruitmentDashboard, error) {
	db, err := a.getRecruitmentDB()
	result := RecruitmentDashboard{StageCounts: make(map[string]int)}
	if err != nil {
		return result, err
	}
	weekStart := time.Now().AddDate(0, 0, -7).Format(time.RFC3339Nano)
	now := time.Now().Format(time.RFC3339Nano)
	queries := []struct {
		query string
		args  []interface{}
		into  *int
	}{
		{"SELECT COUNT(*) FROM projects", nil, &result.OpenProjects},
		{"SELECT COUNT(*) FROM candidates WHERE created_at>=?", []interface{}{weekStart}, &result.CandidatesThisWeek},
		{"SELECT COUNT(*) FROM project_candidates WHERE next_action_at<>'' AND next_action_at<=? AND stage NOT IN ('hired','rejected')", []interface{}{now}, &result.PendingFollowUps},
		{"SELECT COUNT(*) FROM interviews WHERE starts_at>=? AND status='scheduled'", []interface{}{now}, &result.UpcomingInterviews},
	}
	for _, q := range queries {
		if err := db.QueryRow(q.query, q.args...).Scan(q.into); err != nil {
			return result, err
		}
	}
	rows, err := db.Query("SELECT stage,COUNT(*) FROM project_candidates GROUP BY stage")
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var stage string
		var count int
		if err := rows.Scan(&stage, &count); err != nil {
			return result, err
		}
		result.StageCounts[stage] = count
	}
	return result, rows.Err()
}

func (a *App) UpdateCandidateFollowUp(projectCandidateID, stage, owner, nextActionAt, note, actor string) error {
	if !recruitmentStages[stage] {
		return fmt.Errorf("未知招聘阶段: %s", stage)
	}
	if nextActionAt != "" {
		if _, err := time.Parse(time.RFC3339, nextActionAt); err != nil {
			return fmt.Errorf("下一步日期格式无效: %w", err)
		}
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var oldStage string
	if err := tx.QueryRow("SELECT stage FROM project_candidates WHERE id=?", projectCandidateID).Scan(&oldStage); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.Exec(`UPDATE project_candidates SET stage=?,owner=?,next_action_at=?,updated_at=? WHERE id=?`,
		stage, strings.TrimSpace(owner), nextActionAt, now, projectCandidateID); err != nil {
		return err
	}
	if oldStage != stage || strings.TrimSpace(note) != "" {
		eventType := "note"
		if oldStage != stage {
			eventType = "stage_change"
		}
		if _, err := tx.Exec(`INSERT INTO follow_up_events
			(id,project_candidate_id,event_type,from_stage,to_stage,note,actor,created_at)
			VALUES(?,?,?,?,?,?,?,?)`, recruitmentID("followup"), projectCandidateID,
			eventType, oldStage, stage, strings.TrimSpace(note), strings.TrimSpace(actor), now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a *App) AddCandidateCommunication(projectCandidateID, channel, content, actor string) error {
	if strings.TrimSpace(content) == "" || strings.TrimSpace(actor) == "" {
		return fmt.Errorf("请填写沟通内容和记录人")
	}
	if channel != "phone" && channel != "platform" && channel != "wechat" && channel != "email" && channel != "other" {
		return fmt.Errorf("沟通渠道无效")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	var stage string
	if err := db.QueryRow("SELECT stage FROM project_candidates WHERE id=?", projectCandidateID).Scan(&stage); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO follow_up_events(id,project_candidate_id,event_type,from_stage,to_stage,note,actor,created_at)
		VALUES(?,?,?,?,?,?,?,?)`, recruitmentID("followup"), projectCandidateID, "communication:"+channel, stage, stage, strings.TrimSpace(content), strings.TrimSpace(actor), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (a *App) ListFollowUpEvents(projectCandidateID string) ([]FollowUpRecord, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id,event_type,from_stage,to_stage,note,actor,created_at
		FROM follow_up_events WHERE project_candidate_id=? ORDER BY created_at DESC,id DESC`, projectCandidateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]FollowUpRecord, 0)
	for rows.Next() {
		var item FollowUpRecord
		if err := rows.Scan(&item.ID, &item.Type, &item.FromStage, &item.ToStage,
			&item.Note, &item.Actor, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (a *App) ListRecruitmentInterviews(projectID string) ([]RecruitmentInterview, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT i.id,i.project_candidate_id,c.display_name,p.name,i.starts_at,
		i.ends_at,i.interviewer,i.method,i.location,i.note,i.status
		FROM interviews i JOIN project_candidates pc ON pc.id=i.project_candidate_id
		JOIN candidates c ON c.id=pc.candidate_id JOIN projects p ON p.id=pc.project_id
		WHERE (?='' OR p.id=?) ORDER BY i.starts_at,i.id`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]RecruitmentInterview, 0)
	for rows.Next() {
		var item RecruitmentInterview
		if err := rows.Scan(&item.ID, &item.ProjectCandidateID, &item.CandidateName,
			&item.ProjectName, &item.StartsAt, &item.EndsAt, &item.Interviewer,
			&item.Method, &item.Location, &item.Note, &item.Status); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (a *App) SaveRecruitmentInterview(item RecruitmentInterview) (string, error) {
	start, err := time.Parse(time.RFC3339, item.StartsAt)
	if err != nil {
		return "", fmt.Errorf("面试开始时间无效: %w", err)
	}
	end, err := time.Parse(time.RFC3339, item.EndsAt)
	if err != nil || !end.After(start) {
		return "", fmt.Errorf("面试结束时间必须晚于开始时间")
	}
	if strings.TrimSpace(item.ProjectCandidateID) == "" {
		return "", fmt.Errorf("请选择候选人与项目")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return "", err
	}
	if item.ID == "" {
		item.ID = recruitmentID("interview")
	} else {
		var ownerLink string
		err := db.QueryRow("SELECT project_candidate_id FROM interviews WHERE id=?", item.ID).Scan(&ownerLink)
		if err != nil && err != sql.ErrNoRows {
			return "", err
		}
		if err == nil && ownerLink != item.ProjectCandidateID {
			return "", fmt.Errorf("面试不能改挂到另一位候选人")
		}
	}
	if item.Status == "" {
		item.Status = "scheduled"
	}
	if item.Status != "scheduled" && item.Status != "cancelled" && item.Status != "completed" {
		return "", fmt.Errorf("面试状态无效")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(`INSERT INTO interviews
		(id,project_candidate_id,starts_at,ends_at,interviewer,method,location,note,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		starts_at=excluded.starts_at,ends_at=excluded.ends_at,interviewer=excluded.interviewer,
		method=excluded.method,location=excluded.location,note=excluded.note,
		status=excluded.status,updated_at=excluded.updated_at`,
		item.ID, item.ProjectCandidateID, item.StartsAt, item.EndsAt, item.Interviewer,
		item.Method, item.Location, item.Note, item.Status, now, now)
	return item.ID, err
}

func (a *App) CancelRecruitmentInterview(id string) error {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return err
	}
	result, err := db.Exec("UPDATE interviews SET status='cancelled',updated_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}
