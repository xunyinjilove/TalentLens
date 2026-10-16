package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) Bind51JobCandidateDetail(resumeID, detailURL string) JobConversationResult {
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	fail := func(err error) JobConversationResult {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	if !interviewIDPattern.MatchString(resumeID) || !is51JobCandidateDetailURL(detailURL) {
		return fail(fmt.Errorf("请输入该候选人的 51job 新版详情链接（包含 resumeId）"))
	}
	path := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fail(err)
	}
	var resume Resume
	if err = json.Unmarshal(data, &resume); err != nil {
		return fail(err)
	}
	if resume.SourcePlatform != "51job" && !strings.Contains(resume.FileName, "前程无忧") && !strings.HasPrefix(resume.URL, "https://ehire.51job.com/") {
		return fail(fmt.Errorf("该简历未标识为 51job 来源，请先核对来源"))
	}
	copyForVerification := resume
	copyForVerification.URL = detailURL
	copyForVerification.SourcePlatform, copyForVerification.PlatformCandidateID = "51job", ""
	platform, targetID := resumeSourceIdentity(&copyForVerification)
	if platform != "51job" {
		return fail(fmt.Errorf("无法核对详情 ID"))
	}
	if oldPlatform, oldID := resumeSourceIdentity(&resume); oldPlatform == "51job" && oldID != targetID {
		return fail(fmt.Errorf("新链接与档案已有的平台 ID 不一致，请先核对人选"))
	}
	verified := a.run51JobConversation(&copyForVerification, "verify_candidate", "", nil)
	if verified.Status != "verified" {
		return verified
	}
	if verified.TargetID != targetID {
		return fail(fmt.Errorf("平台返回的详情 ID 与输入链接不一致，未关联"))
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return fail(err)
	}
	tx, err := db.Begin()
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback()
	var candidateID string
	if err = tx.QueryRow(`SELECT candidate_id FROM resume_versions WHERE legacy_resume_id=? LIMIT 1`, resumeID).Scan(&candidateID); err != nil {
		return fail(fmt.Errorf("本地人才档案关联缺失，请刷新人才库后重试"))
	}
	var existingCandidate, sourceID string
	err = tx.QueryRow(`SELECT candidate_id,id FROM candidate_sources WHERE platform='51job' AND platform_account='' AND platform_candidate_id=?`, targetID).Scan(&existingCandidate, &sourceID)
	if err == nil && existingCandidate != candidateID {
		return fail(fmt.Errorf("该详情 ID 已属于另一份人才档案；请先人工核对重复档案，未自动合并"))
	}
	if err != nil && err != sql.ErrNoRows {
		return fail(err)
	}
	if err == sql.ErrNoRows {
		sourceID = recruitmentID("verified_source")
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err = tx.Exec(`INSERT INTO candidate_sources(id,candidate_id,platform,platform_candidate_id,profile_url,source_kind,first_seen_at,last_seen_at) VALUES(?,?,'51job',?,?,'verified_profile',?,?)`, sourceID, candidateID, targetID, detailURL, now, now); err != nil {
			return fail(err)
		}
	}
	if _, err = tx.Exec(`UPDATE resume_versions SET source_id=? WHERE legacy_resume_id=?`, sourceID, resumeID); err != nil {
		return fail(err)
	}
	if err = tx.Commit(); err != nil {
		return fail(err)
	}
	resume.URL, resume.SourcePlatform, resume.PlatformCandidateID = detailURL, "51job", targetID
	if err = a.save51JobContactResume(&resume); err != nil {
		return fail(err)
	}
	return JobConversationResult{Status: "verified", TargetID: targetID, Message: "已核对候选人详情页；可继续打招呼和索要简历"}
}
