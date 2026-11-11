package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AutoLink51JobCandidateDetail locates the platform-provided URL by stable
// candidate ID inside the bound job. A name match is never sufficient.
func (a *App) AutoLink51JobCandidateDetail(resumeID string) JobConversationResult {
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
	if err != nil {
		return JobConversationResult{Status: "error", Message: "本地简历不存在"}
	}
	var resume Resume
	if json.Unmarshal(data, &resume) != nil || resume.SourcePlatform != "51job" || resume.PlatformCandidateID == "" {
		return JobConversationResult{Status: "error", Message: "缺少可核对的 51job 人才 ID"}
	}
	if is51JobResumeURLForCandidate(resume.URL, resume.PlatformCandidateID) {
		return JobConversationResult{Status: "verified", Message: "候选人详情链接已关联"}
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	rows, err := db.Query(`SELECT platform_account,platform_job_id FROM platform_job_mappings
		WHERE platform='51job' AND project_id=?`, resume.ProjectID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	type jobScope struct{ account, job string }
	var jobs []jobScope
	for rows.Next() {
		var j jobScope
		if rows.Scan(&j.account, &j.job) == nil {
			jobs = append(jobs, j)
		}
	}
	rows.Close()
	if len(jobs) == 0 {
		return JobConversationResult{Status: "error", Message: "项目尚未绑定 51job 职位"}
	}
	catalog, err := a.Get51JobTalentManagementCatalog()
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for _, job := range jobs {
		if job.account != catalog.AccountScope {
			continue
		}
		for status := 1; status <= 4; status++ {
			for pageIndex := 1; pageIndex <= 20; pageIndex++ {
				if ctx.Err() != nil {
					return JobConversationResult{Status: "error", Message: "自动查找详情链接超时"}
				}
				out, scanErr := runTalentManagementAdapterContext(ctx, 25*time.Second, "--mode", "scan", "--job-id", job.job,
					"--status-index", fmt.Sprint(status), "--page-index", fmt.Sprint(pageIndex))
				if scanErr != nil {
					break
				}
				var listing talentManagementPage
				if json.Unmarshal(out, &listing) != nil || listing.Status != "ok" || listing.AccountScope != job.account || listing.JobID != job.job {
					break
				}
				for _, candidate := range listing.Candidates {
					if candidate.CandidateID != resume.PlatformCandidateID || candidate.RecordID == "" {
						continue
					}
					body, detailErr := runTalentManagementAdapterContext(ctx, 15*time.Second, "--mode", "detail", "--job-id", job.job,
						"--candidate-id", candidate.CandidateID, "--record-id", candidate.RecordID, "--status-index", fmt.Sprint(status), "--page-index", fmt.Sprint(pageIndex))
					if detailErr != nil {
						return JobConversationResult{Status: "error", Message: detailErr.Error()}
					}
					var detail talentManagementDetail
					if json.Unmarshal(body, &detail) != nil || detail.AccountScope != job.account || detail.JobID != job.job ||
						detail.CandidateID != resume.PlatformCandidateID || detail.RecordID != candidate.RecordID ||
						!is51JobResumeURLForCandidate(detail.ProfileURL, resume.PlatformCandidateID) {
						return JobConversationResult{Status: "error", Message: "平台详情链接或人才 ID 无法核对"}
					}
					resume.URL = strings.TrimSpace(detail.ProfileURL)
					if err := a.save51JobContactResume(&resume); err != nil {
						return JobConversationResult{Status: "error", Message: err.Error()}
					}
					return JobConversationResult{Status: "verified", Message: "已自动关联 51job 详情链接"}
				}
				if !listing.HasMore {
					break
				}
			}
		}
	}
	return JobConversationResult{Status: "error", Message: "在已绑定职位的四类名单中未找到同一人才 ID，已跳过"}
}
