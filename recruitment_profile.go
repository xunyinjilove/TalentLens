package main

import (
	"database/sql"
	"fmt"
)

type CandidateSourceView struct {
	Platform            string `json:"platform"`
	PlatformCandidateID string `json:"platformCandidateId"`
	ProfileURL          string `json:"profileUrl"`
	SourceKind          string `json:"sourceKind"`
}

type CandidateResumeView struct {
	ID        string `json:"id"`
	FileName  string `json:"fileName"`
	CreatedAt string `json:"createdAt"`
}

type CandidateTimelineEvent struct {
	ID          string `json:"id"`
	ProjectName string `json:"projectName"`
	Type        string `json:"type"`
	Note        string `json:"note"`
	Actor       string `json:"actor"`
	CreatedAt   string `json:"createdAt"`
}

type CandidateProfile struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	VerifiedEmail string                   `json:"verifiedEmail"`
	VerifiedPhone string                   `json:"verifiedPhone"`
	Projects      []TalentCandidate        `json:"projects"`
	Sources       []CandidateSourceView    `json:"sources"`
	Resumes       []CandidateResumeView    `json:"resumes"`
	Timeline      []CandidateTimelineEvent `json:"timeline"`
}

// GetCandidateProfileByResume resolves the unified candidate record from the
// resume shown in the assessment page. Legacy resumes without an imported
// candidate record return an empty profile so their assessment stays usable.
func (a *App) GetCandidateProfileByResume(resumeID string) (CandidateProfile, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return CandidateProfile{}, fmt.Errorf("简历 ID 无效")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return CandidateProfile{}, err
	}
	var candidateID string
	err = db.QueryRow(`SELECT candidate_id FROM resume_versions WHERE legacy_resume_id=? ORDER BY created_at DESC LIMIT 1`, resumeID).Scan(&candidateID)
	if err == sql.ErrNoRows {
		return CandidateProfile{}, nil
	}
	if err != nil {
		return CandidateProfile{}, err
	}
	return a.GetCandidateProfile(candidateID)
}

func (a *App) GetCandidateProfile(candidateID string) (CandidateProfile, error) {
	profile := CandidateProfile{ID: candidateID, Projects: make([]TalentCandidate, 0), Sources: make([]CandidateSourceView, 0), Resumes: make([]CandidateResumeView, 0), Timeline: make([]CandidateTimelineEvent, 0)}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return profile, err
	}
	if err := db.QueryRow("SELECT display_name,verified_email,verified_phone FROM candidates WHERE id=?", candidateID).Scan(&profile.Name, &profile.VerifiedEmail, &profile.VerifiedPhone); err != nil {
		return profile, err
	}
	all, err := a.ListTalentCandidates("")
	if err != nil {
		return profile, err
	}
	for _, item := range all {
		if item.ID == candidateID {
			profile.Projects = append(profile.Projects, item)
		}
	}
	sources, err := db.Query("SELECT platform,platform_candidate_id,profile_url,source_kind FROM candidate_sources WHERE candidate_id=? ORDER BY first_seen_at,id", candidateID)
	if err != nil {
		return profile, err
	}
	for sources.Next() {
		var item CandidateSourceView
		if err := sources.Scan(&item.Platform, &item.PlatformCandidateID, &item.ProfileURL, &item.SourceKind); err != nil {
			sources.Close()
			return profile, err
		}
		profile.Sources = append(profile.Sources, item)
	}
	if err := sources.Err(); err != nil {
		sources.Close()
		return profile, err
	}
	sources.Close()
	resumes, err := db.Query("SELECT id,file_name,created_at FROM resume_versions WHERE candidate_id=? ORDER BY created_at DESC,id DESC", candidateID)
	if err != nil {
		return profile, err
	}
	for resumes.Next() {
		var item CandidateResumeView
		if err := resumes.Scan(&item.ID, &item.FileName, &item.CreatedAt); err != nil {
			resumes.Close()
			return profile, err
		}
		profile.Resumes = append(profile.Resumes, item)
	}
	if err := resumes.Err(); err != nil {
		resumes.Close()
		return profile, err
	}
	resumes.Close()
	events, err := db.Query(`SELECT f.id,p.name,f.event_type,f.note,f.actor,f.created_at FROM follow_up_events f
  JOIN project_candidates pc ON pc.id=f.project_candidate_id JOIN projects p ON p.id=pc.project_id
  WHERE pc.candidate_id=? ORDER BY f.created_at DESC,f.id DESC LIMIT 200`, candidateID)
	if err != nil {
		return profile, err
	}
	for events.Next() {
		var item CandidateTimelineEvent
		if err := events.Scan(&item.ID, &item.ProjectName, &item.Type, &item.Note, &item.Actor, &item.CreatedAt); err != nil {
			events.Close()
			return profile, err
		}
		profile.Timeline = append(profile.Timeline, item)
	}
	if err := events.Err(); err != nil {
		events.Close()
		return profile, err
	}
	events.Close()
	if len(profile.Projects) == 0 && len(profile.Sources) == 0 && len(profile.Resumes) == 0 {
		return profile, fmt.Errorf("候选人档案为空")
	}
	return profile, nil
}
