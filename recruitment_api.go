package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TalentCandidate struct {
	ID                 string   `json:"id"`
	ProjectCandidateID string   `json:"projectCandidateId"`
	ProjectID          string   `json:"projectId"`
	ProjectName        string   `json:"projectName"`
	JobTitle           string   `json:"jobTitle"`
	Name               string   `json:"name"`
	Source             string   `json:"source"`
	ResumeID           string   `json:"resumeId"`
	ProfileURL         string   `json:"profileUrl"`
	AnalysisStatus     string   `json:"analysisStatus"`
	AbilityScore       int      `json:"abilityScore"`
	MatchScore         int      `json:"matchScore"`
	CurrentRole        string   `json:"currentRole"`
	Education          string   `json:"education"`
	WorkYears          string   `json:"workYears"`
	Skills             []string `json:"skills"`
	RedLineStatus      string   `json:"redLineStatus"`
	ConfirmedBonus     int      `json:"confirmedBonus"`
	Stage              string   `json:"stage"`
	Owner              string   `json:"owner"`
	NextActionAt       string   `json:"nextActionAt"`
	CreatedAt          string   `json:"createdAt"`
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
		COALESCE(ev.status,''),COALESCE(ev.ability_score,0),COALESCE(ev.analysis_json,''),
		COALESCE(ev.redline_status,''),COALESCE(ev.confirmed_bonus_count,0),
		pc.stage,pc.owner,pc.next_action_at,pc.created_at
		FROM project_candidates pc JOIN candidates c ON c.id=pc.candidate_id
		JOIN projects p ON p.id=pc.project_id
		LEFT JOIN resume_versions rv ON rv.id=(SELECT pcr.resume_version_id FROM project_candidate_resumes pcr
			JOIN resume_versions rv2 ON rv2.id=pcr.resume_version_id
			WHERE pcr.project_candidate_id=pc.id ORDER BY rv2.created_at DESC,rv2.id DESC LIMIT 1)
		LEFT JOIN evaluations ev ON ev.id=(SELECT id FROM evaluations
			WHERE project_candidate_id=pc.id ORDER BY created_at DESC,id DESC LIMIT 1)
		WHERE pc.archived_at='' AND (?='' OR p.id=?) ORDER BY pc.created_at DESC,pc.id DESC`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]TalentCandidate, 0)
	for rows.Next() {
		var item TalentCandidate
		var raw, analysisJSON string
		if err := rows.Scan(&item.ID, &item.ProjectCandidateID, &item.ProjectID,
			&item.ProjectName, &item.JobTitle, &item.Name, &item.ResumeID, &raw,
			&item.AnalysisStatus, &item.AbilityScore, &analysisJSON, &item.RedLineStatus,
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
				if resume.Analysis != nil {
					if item.CurrentRole == "" {
						item.CurrentRole = strings.TrimSpace(resume.Analysis.CurrentRole)
					}
					if item.Education == "" {
						item.Education = strings.TrimSpace(resume.Analysis.Education)
						if item.Education == "" {
							item.Education = strings.TrimSpace(resume.Analysis.EducationDetail)
						}
					}
					if item.WorkYears == "" {
						item.WorkYears = strings.TrimSpace(resume.Analysis.WorkYears)
					}
				}
			}
		}
		if analysisJSON != "" && item.AnalysisStatus == "done" {
			var analysis AnalysisResult
			if json.Unmarshal([]byte(analysisJSON), &analysis) == nil {
				item.MatchScore = holisticMatchScore(&analysis)
				if role := strings.TrimSpace(analysis.CurrentRole); role != "" {
					item.CurrentRole = role
				}
				if education := strings.TrimSpace(analysis.Education); education != "" {
					item.Education = education
				} else if detail := strings.TrimSpace(analysis.EducationDetail); detail != "" {
					item.Education = detail
				}
				if workYears := strings.TrimSpace(analysis.WorkYears); workYears != "" {
					item.WorkYears = workYears
				}
				seen := make(map[string]bool)
				for _, check := range append(analysis.CoreCriterionChecks, analysis.ExperienceCompetencies...) {
					label := strings.TrimSpace(check.Criterion)
					if check.Level <= 0 || strings.TrimSpace(check.Evidence) == "" || label == "" || seen[label] {
						continue
					}
					item.Skills = append(item.Skills, label)
					seen[label] = true
					if len(item.Skills) == 4 {
						break
					}
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
		{"SELECT COUNT(*) FROM project_candidates WHERE archived_at='' AND next_action_at<>'' AND next_action_at<=? AND stage NOT IN ('hired','rejected')", []interface{}{now}, &result.PendingFollowUps},
		{"SELECT COUNT(*) FROM interviews WHERE starts_at>=? AND status='scheduled'", []interface{}{now}, &result.UpcomingInterviews},
	}
	for _, q := range queries {
		if err := db.QueryRow(q.query, q.args...).Scan(q.into); err != nil {
			return result, err
		}
	}
	rows, err := db.Query("SELECT stage,COUNT(*) FROM project_candidates WHERE archived_at='' GROUP BY stage")
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

// ArchiveTalentCandidates removes project links from the active talent pool
// while retaining the shared profile, resume versions and communication history.
func (a *App) ArchiveTalentCandidates(projectCandidateIDs []string) (int, error) {
	if len(projectCandidateIDs) == 0 {
		return 0, fmt.Errorf("请选择要移出人才库的简历")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return 0, err
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	seen := make(map[string]bool, len(projectCandidateIDs))
	count := 0
	for _, rawID := range projectCandidateIDs {
		id := strings.TrimSpace(rawID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		result, err := tx.Exec("UPDATE project_candidates SET archived_at=?,updated_at=? WHERE id=? AND archived_at=''", now, now, id)
		if err != nil {
			return 0, err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if changed == 0 {
			return 0, fmt.Errorf("简历不存在或已移出人才库: %s", id)
		}
		if _, err := tx.Exec(`INSERT INTO follow_up_events(id,project_candidate_id,event_type,note,actor,created_at)
			VALUES(?,?,'archived','从人才库移除，保留统一档案及沟通记录','HR',?)`, recruitmentID("followup"), id, now); err != nil {
			return 0, err
		}
		count++
	}
	if count == 0 {
		return 0, fmt.Errorf("请选择要移出人才库的简历")
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
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

// JobStats 职位统计数据（投递与合适候选人计数）
type JobStats struct {
	TotalApplications int `json:"totalApplications"`
	SuitableCount     int `json:"suitableCount"`
}

// GetJobStatistics 返回所有职位的投递总数与合适人数统计
func (a *App) GetJobStatistics() (map[string]JobStats, error) {
	candidates, err := a.ListTalentCandidates("")
	if err != nil {
		return nil, err
	}
	result := make(map[string]JobStats)
	for _, c := range candidates {
		stats := result[c.ProjectID]
		stats.TotalApplications++
		// 判定“合适”条件：未被淘汰且（能力分>=60且无红线违规，或阶段已推进至沟通/面试/Offer/入职）
		isSuitable := false
		if c.Stage != "rejected" {
			if c.AbilityScore >= 60 && c.RedLineStatus != "failed" {
				isSuitable = true
			} else if c.Stage == "first_interview" || c.Stage == "second_interview" || c.Stage == "offer" || c.Stage == "hired" || c.Stage == "communicating" {
				isSuitable = true
			}
		}
		if isSuitable {
			stats.SuitableCount++
		}
		result[c.ProjectID] = stats
	}

	// 保证所有现有项目均有初始结构
	projects := a.GetProjects()
	for _, p := range projects {
		if _, ok := result[p.ID]; !ok {
			result[p.ID] = JobStats{TotalApplications: 0, SuitableCount: 0}
		}
	}
	return result, nil
}

// MoveCandidateToProject 将候选人从当前职位调配至指定职位库
func (a *App) MoveCandidateToProject(projectCandidateID, targetProjectID string) error {
	projectCandidateID = strings.TrimSpace(projectCandidateID)
	targetProjectID = strings.TrimSpace(targetProjectID)
	if projectCandidateID == "" || targetProjectID == "" {
		return fmt.Errorf("参数不完整")
	}

	targetProj := a.GetProject(targetProjectID)
	if targetProj == nil {
		return fmt.Errorf("目标职位不存在")
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

	var candidateID, currentProjectID, stage string
	err = tx.QueryRow(`SELECT candidate_id, project_id, stage FROM project_candidates WHERE id = ?`, projectCandidateID).Scan(&candidateID, &currentProjectID, &stage)
	if err != nil {
		return fmt.Errorf("未找到候选人档案: %w", err)
	}

	if currentProjectID == targetProjectID {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	// 检查目标项目中是否已存在该候选人
	var existingPCID string
	err = tx.QueryRow(`SELECT id FROM project_candidates WHERE project_id = ? AND candidate_id = ?`, targetProjectID, candidateID).Scan(&existingPCID)
	if err == nil && existingPCID != "" {
		// 目标项目已存在记录，转移简历版本关联并清理当前关联
		_, _ = tx.Exec(`INSERT OR IGNORE INTO project_candidate_resumes (project_candidate_id, resume_version_id, created_at)
			SELECT ?, resume_version_id, created_at FROM project_candidate_resumes WHERE project_candidate_id = ?`, existingPCID, projectCandidateID)
		_, _ = tx.Exec(`UPDATE evaluations SET project_candidate_id = ? WHERE project_candidate_id = ?`, existingPCID, projectCandidateID)
		_, _ = tx.Exec(`DELETE FROM project_candidates WHERE id = ?`, projectCandidateID)
		projectCandidateID = existingPCID
	} else {
		// 将当前关联直接变更为目标项目
		_, err = tx.Exec(`UPDATE project_candidates SET project_id = ?, updated_at = ? WHERE id = ?`, targetProjectID, now, projectCandidateID)
		if err != nil {
			return fmt.Errorf("更新候选人所属职位失败: %w", err)
		}
	}

	// 记录调配跟进事件
	_, _ = tx.Exec(`INSERT INTO follow_up_events (id, project_candidate_id, event_type, from_stage, to_stage, note, actor, created_at)
		VALUES (?, ?, 'transfer', ?, ?, ?, 'HR', ?)`,
		recruitmentID("followup"), projectCandidateID, stage, stage, "调配至职位库: "+targetProj.Name, now)

	// 查询对应的旧版 resume ID (如果有)
	var legacyResumeID string
	_ = tx.QueryRow(`SELECT COALESCE(rv.legacy_resume_id, '') FROM project_candidate_resumes pcr
		JOIN resume_versions rv ON rv.id = pcr.resume_version_id
		WHERE pcr.project_candidate_id = ? LIMIT 1`, projectCandidateID).Scan(&legacyResumeID)

	if err := tx.Commit(); err != nil {
		return err
	}

	// 同步维护 JSON 物理文件与项目简历 ID 列表
	if legacyResumeID != "" {
		resDir := filepath.Join(a.getDataDir(), "resumes")
		resumeData, rErr := os.ReadFile(filepath.Join(resDir, legacyResumeID+".json"))
		if rErr == nil {
			var r Resume
			if json.Unmarshal(resumeData, &r) == nil {
				r.ProjectID = targetProjectID
				a.saveResume(&r)
			}
		}

		// 从原项目 ResumeIDs 移除
		oldProj := a.GetProject(currentProjectID)
		if oldProj != nil {
			var updatedResumeIDs []string
			for _, rid := range oldProj.ResumeIDs {
				if rid != legacyResumeID {
					updatedResumeIDs = append(updatedResumeIDs, rid)
				}
			}
			oldProj.ResumeIDs = updatedResumeIDs
			a.saveProject(oldProj)
		}

		// 追加到目标项目 ResumeIDs
		hasResume := false
		for _, rid := range targetProj.ResumeIDs {
			if rid == legacyResumeID {
				hasResume = true
				break
			}
		}
		if !hasResume {
			targetProj.ResumeIDs = append(targetProj.ResumeIDs, legacyResumeID)
			a.saveProject(targetProj)
		}
	}

	return nil
}
