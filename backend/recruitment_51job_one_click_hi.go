package backend

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type OneClickHiItem struct {
	CandidateID string `json:"candidateId"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Points      int    `json:"points"`
	Reason      string `json:"reason"`
}

type OneClickHiResult struct {
	Total       int              `json:"total"`
	Sent        int              `json:"sent"`
	Skipped     int              `json:"skipped"`
	Uncertain   int              `json:"uncertain"`
	PointsUsed  int              `json:"pointsUsed"`
	PointsLimit int              `json:"pointsLimit"`
	Items       []OneClickHiItem `json:"items"`
}

// Run51JobOneClickHi sends only to formal library members. candidateIDs
// narrows a telescope batch; an empty slice means all eligible project resumes.
// Each attempted send is durably recorded before the platform click.
func (a *App) Run51JobOneClickHi(projectID, platformJobID string, candidateIDs []string, pointsLimit int) (OneClickHiResult, error) {
	result := OneClickHiResult{PointsLimit: pointsLimit, Items: []OneClickHiItem{}}
	if pointsLimit <= 0 || pointsLimit > 100000 {
		return result, fmt.Errorf("请设置 1 至 100000 的本批点数上限")
	}
	project := a.GetProject(projectID)
	if project == nil {
		return result, fmt.Errorf("项目不存在")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return result, err
	}
	criteria, _ := a.GetTalentManagementStrictCriteria(projectID)
	projectMessage := ""
	if criteria != nil {
		projectMessage = strings.TrimSpace(criteria.ProjectIntroMessage)
	}
	if len([]rune(projectMessage)) > 500 {
		return result, fmt.Errorf("项目文案超过 500 字，请先在岗位硬性条件中修改")
	}
	var account, jobID string
	if platformJobID == "" {
		rows, err := db.Query(`SELECT platform_account,platform_job_id FROM platform_job_mappings WHERE platform='51job' AND project_id=?`, projectID)
		if err != nil {
			return result, err
		}
		defer rows.Close()
		for rows.Next() {
			if err := rows.Scan(&account, &jobID); err != nil {
				return result, err
			}
			result.Total++
		}
		if result.Total != 1 {
			return result, fmt.Errorf("项目绑定了 %d 个 51job 职位，请先选定一个职位", result.Total)
		}
		result.Total = 0
	} else {
		jobID = platformJobID
		if err := db.QueryRow(`SELECT platform_account FROM platform_job_mappings WHERE platform='51job' AND project_id=? AND platform_job_id=?`, projectID, jobID).Scan(&account); err != nil {
			return result, fmt.Errorf("该望远镜职位未绑定当前项目")
		}
	}
	allowed := map[string]bool{}
	for _, id := range candidateIDs {
		if strings.TrimSpace(id) != "" {
			allowed[strings.TrimSpace(id)] = true
		}
	}
	seen := map[string]bool{}
	for _, resume := range a.GetProjectResumes(projectID) {
		if resume == nil || resume.SourcePlatform != "51job" || resume.PlatformCandidateID == "" || resume.ContactStatus != "" || strings.Contains(resume.PlatformStatus, "不合适") || strings.Contains(resume.PlatformStatus, "淘汰") {
			continue
		}
		if len(allowed) > 0 && !allowed[resume.PlatformCandidateID] {
			continue
		}
		if seen[resume.PlatformCandidateID] {
			continue
		}
		seen[resume.PlatformCandidateID] = true
		result.Total++
		item := OneClickHiItem{CandidateID: resume.PlatformCandidateID, Name: resume.FileName, Status: "skipped"}
		if reason := a.known51JobHardFailure(resume); reason != "" {
			item.Reason = "已知不符合硬性条件：" + reason
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		var prior string
		err := db.QueryRow(`SELECT status FROM platform_action_ledger WHERE project_id=? AND candidate_id=? AND action_type='hi' ORDER BY requested_at DESC LIMIT 1`, projectID, resume.PlatformCandidateID).Scan(&prior)
		if err != nil && err != sql.ErrNoRows {
			return result, err
		}
		if prior != "" {
			item.Reason = "已有发送或待核查记录，不重复操作"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		if !is51JobResumeURLForCandidate(resume.URL, resume.PlatformCandidateID) {
			linked := a.AutoLink51JobCandidateDetail(resume.ID)
			if linked.Status != "verified" {
				item.Reason = linked.Message
				result.Skipped++
				result.Items = append(result.Items, item)
				continue
			}
		}
		jobs := a.List51JobCommunicationJobs(resume.ID, "")
		if jobs.Status != "jobs" {
			item.Reason = jobs.Message
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		jobTitle := ""
		companyID := strings.TrimPrefix(strings.Split(account, ":hr:")[0], "co:")
		for _, option := range jobs.Jobs {
			if option.ID == jobID {
				if option.CompanyID != "" && option.CompanyID != companyID {
					continue
				}
				jobTitle = option.Title
				break
			}
		}
		if jobTitle == "" {
			searched := a.List51JobCommunicationJobs(resume.ID, project.JobConfig.Title)
			if searched.Status == "jobs" {
				for _, option := range searched.Jobs {
					if option.ID == jobID && (option.CompanyID == "" || option.CompanyID == companyID) {
						jobTitle = option.Title
						break
					}
				}
			}
		}
		if jobTitle == "" {
			item.Reason = "沟通职位列表中找不到已绑定职位"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		quote := a.Prepare51JobHiChatForJob(resume.ID, jobID, jobTitle)
		if quote.Status != "needs_confirmation" || quote.QuoteID == "" {
			item.Reason = quote.Message
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		item.Points = quote.Points
		if quote.Points <= 0 || quote.Points > 100 || result.PointsUsed+quote.Points > pointsLimit {
			item.Reason = "本次点数超过单人或本批限额"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		ledgerID := recruitmentID("pal")
		_, err = db.Exec(`INSERT INTO platform_action_ledger(id,project_id,platform,platform_account,platform_job_id,candidate_id,record_id,candidate_name,action_type,confirmed_by,requested_at,receipt,status,error_message)
			VALUES(?,?,?,?,?,?,?,?,?,'HR',?,'','executing','')`, ledgerID, projectID, "51job", account, jobID, resume.PlatformCandidateID, resume.PlatformCandidateID, item.Name, "hi", now)
		if err != nil {
			item.Reason = "无法保存操作记录：" + err.Error()
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}
		// Reserve the possible charge before clicking. An uncertain receipt may
		// already have consumed points and must count against the batch cap.
		result.PointsUsed += quote.Points
		sent := a.Confirm51JobHiChat(resume.ID, quote.QuoteID)
		if sent.Status == "sent" {
			item.Status, item.Reason = "sent", sent.Message
			result.Sent++
			_, _ = db.Exec(`UPDATE platform_action_ledger SET status='success',receipt=? WHERE id=?`, sent.Message, ledgerID)
			if projectMessage != "" {
				messageID := recruitmentID("pal")
				_, messageErr := db.Exec(`INSERT INTO platform_action_ledger(id,project_id,platform,platform_account,platform_job_id,candidate_id,record_id,candidate_name,action_type,confirmed_by,requested_at,receipt,status,error_message)
					VALUES(?,?,?,?,?,?,?,?,?,'HR',?,?,'executing','')`, messageID, projectID, "51job", account, jobID, resume.PlatformCandidateID, resume.PlatformCandidateID, item.Name, "project_message", now, projectMessage)
				if messageErr != nil {
					item.Reason += "；项目文案待重试：无法保存发送记录"
				} else {
					second := a.Send51JobOutreachMessage(resume.ID, projectMessage)
					if second.Status == "sent" {
						item.Reason += "；项目文案已发送"
						_, _ = db.Exec(`UPDATE platform_action_ledger SET status='success',receipt=? WHERE id=?`, second.Message, messageID)
					} else {
						item.Reason += "；项目文案待重试：" + second.Message
						_, _ = db.Exec(`UPDATE platform_action_ledger SET status='pending_retry',error_message=? WHERE id=?`, second.Message, messageID)
					}
				}
			}
		} else {
			item.Status, item.Reason = "uncertain", sent.Message
			result.Uncertain++
			_, _ = db.Exec(`UPDATE platform_action_ledger SET status='uncertain',error_message=? WHERE id=?`, sent.Message, ledgerID)
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// Retry51JobProjectMessage never repeats the first HiChat. The conversation
// adapter checks for the exact saved text in the platform chat before sending.
func (a *App) Retry51JobProjectMessage(projectID, candidateID string) JobConversationResult {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	var ledgerID, message, status string
	err = db.QueryRow(`SELECT id,receipt,status FROM platform_action_ledger WHERE project_id=? AND candidate_id=? AND action_type='project_message' ORDER BY requested_at DESC LIMIT 1`, projectID, candidateID).Scan(&ledgerID, &message, &status)
	if err != nil || status != "pending_retry" || strings.TrimSpace(message) == "" {
		return JobConversationResult{Status: "error", Message: "没有可安全重试的项目文案记录"}
	}
	var resumeID string
	for _, resume := range a.GetProjectResumes(projectID) {
		if resume != nil && resume.SourcePlatform == "51job" && resume.PlatformCandidateID == candidateID && resume.ContactStatus == "greeted" {
			resumeID = resume.ID
			break
		}
	}
	if resumeID == "" {
		return JobConversationResult{Status: "error", Message: "候选人未处于已联系状态，未重复发送首次招呼"}
	}
	result := a.Send51JobOutreachMessage(resumeID, message)
	if result.Status == "sent" || strings.Contains(result.Message, "聊天记录中已有相同内容") {
		_, _ = db.Exec(`UPDATE platform_action_ledger SET status='success',receipt=?,error_message='' WHERE id=?`, "项目文案已在平台聊天记录核实："+result.Message, ledgerID)
		return JobConversationResult{Status: "sent", Message: "项目文案已发送或已在平台聊天记录核实"}
	}
	_, _ = db.Exec(`UPDATE platform_action_ledger SET error_message=? WHERE id=?`, result.Message, ledgerID)
	return result
}
