package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type OutreachItem struct {
	ID                  string `json:"id"`
	ProjectCandidateID  string `json:"projectCandidateId"`
	CandidateName       string `json:"candidateName"`
	Platform            string `json:"platform"`
	PlatformCandidateID string `json:"platformCandidateId"`
	PlatformJobID       string `json:"platformJobId"`
	ProfileURL          string `json:"profileUrl"`
	Message             string `json:"message"`
	Evidence            string `json:"evidence"`
	RedLineStatus       string `json:"redLineStatus"`
	AbilityScore        int    `json:"abilityScore"`
	Rank                int    `json:"rank"`
	Status              string `json:"status"`
	Result              string `json:"result"`
}

type OutreachBatch struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"projectId"`
	ProjectName string         `json:"projectName"`
	Status      string         `json:"status"`
	Items       []OutreachItem `json:"items"`
	CreatedAt   string         `json:"createdAt"`
	ConfirmedAt string         `json:"confirmedAt"`
}

type OutreachApproval struct {
	ItemID         string `json:"itemId"`
	Message        string `json:"message"`
	Include        bool   `json:"include"`
	ConfirmPending bool   `json:"confirmPending"`
}

func extractResumeHighlights(resume *Resume) (company string, recentRole string, keyProject string, techSkills string, excerpt string) {
	if resume == nil {
		return "", "", "", "", ""
	}
	content := resume.Content

	if resume.Analysis != nil {
		recentRole = resume.Analysis.CurrentRole
		if len(resume.Analysis.Strengths) > 0 {
			count := len(resume.Analysis.Strengths)
			if count > 3 {
				count = 3
			}
			techSkills = strings.Join(resume.Analysis.Strengths[:count], "、")
		}
	}

	lines := strings.Split(content, "\n")
	var projectLines []string
	inProjectSection := false
	inWorkSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "工作经历") || strings.Contains(line, "任职履历") {
			inWorkSection = true
			inProjectSection = false
			continue
		}
		if strings.Contains(line, "项目经验") || strings.Contains(line, "项目经历") {
			inProjectSection = true
			inWorkSection = false
			continue
		}
		if strings.Contains(line, "教育经历") || strings.Contains(line, "在校情况") || strings.Contains(line, "技能/语言") {
			inProjectSection = false
			inWorkSection = false
		}

		if inWorkSection && company == "" {
			for _, kw := range []string{"研究所", "研究院", "银行", "有限公司", "股份", "集团", "科技", "中心", "医院", "所"} {
				if strings.Contains(line, kw) && len([]rune(line)) >= 4 && len([]rune(line)) <= 35 && !strings.Contains(line, "任职") && !strings.Contains(line, "规模") {
					company = line
					break
				}
			}
		}

		if inProjectSection && len(projectLines) < 5 {
			if len([]rune(line)) >= 4 && len([]rune(line)) <= 60 && !strings.Contains(line, "项目背景") && !strings.Contains(line, "项目职责") {
				projectLines = append(projectLines, line)
				if keyProject == "" && len([]rune(line)) >= 4 && len([]rune(line)) <= 30 {
					keyProject = line
				}
			}
		}
	}

	if keyProject == "" && len(projectLines) > 0 {
		keyProject = projectLines[0]
	}
	if keyProject == "" && resume.Analysis != nil && len(resume.Analysis.Strengths) > 0 {
		keyProject = resume.Analysis.Strengths[0]
	}

	if len(projectLines) > 0 {
		excerpt = strings.Join(projectLines, "\n")
	} else if len([]rune(content)) > 400 {
		excerpt = string([]rune(content)[:400])
	} else {
		excerpt = content
	}

	return company, recentRole, keyProject, techSkills, excerpt
}

func outreachFact(resume *Resume) string {
	if resume == nil {
		return ""
	}
	for _, line := range strings.Split(resume.Content, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"任职履历快照：", "【核心专业技能】", "• "} {
			if strings.HasPrefix(line, prefix) {
				fact := strings.TrimSpace(strings.TrimPrefix(line, prefix))
				if len([]rune(fact)) >= 6 && len([]rune(fact)) <= 70 && !strings.Contains(fact, "详见") && !strings.Contains(fact, "平台默认") {
					return fact
				}
			}
		}
	}
	return ""
}

func (a *App) generateAIOutreachGreeting(resume *Resume, jobTitle string, projectName string) (greeting string, evidence string) {
	if resume == nil {
		return "", ""
	}
	name := conversationCandidateName(resume)
	if resume.Analysis != nil && resume.Analysis.CandidateName != "" {
		name = resume.Analysis.CandidateName
	}
	if jobTitle == "" {
		jobTitle = "相关专业岗位"
	}

	company, recentRole, keyProject, techSkills, excerpt := extractResumeHighlights(resume)

	if keyProject != "" && company != "" {
		evidence = fmt.Sprintf("过往履历：%s · 核心项目/专长：%s", company, keyProject)
	} else if keyProject != "" {
		evidence = fmt.Sprintf("核心项目/业务经历：%s", keyProject)
	} else if techSkills != "" {
		evidence = fmt.Sprintf("核心技能亮点：%s", techSkills)
	} else {
		evidence = outreachFact(resume)
	}

	cfg := &a.config.AI
	if cfg != nil && strings.TrimSpace(cfg.APIKey) != "" {
		workYears := "具备丰富实战经验"
		strengthsDesc := techSkills
		if resume.Analysis != nil {
			if resume.Analysis.WorkYears != "" {
				workYears = resume.Analysis.WorkYears
			}
			if len(resume.Analysis.Strengths) > 0 {
				strengthsDesc = strings.Join(resume.Analysis.Strengths, "；")
			}
		}

		prompt := fmt.Sprintf(`你是一名资深技术猎头与【透景生命】招聘专家。请根据以下候选人的真实任职单位、代表性项目与业务落地成果，为我们正在招聘的岗位【%s】撰写一段专业、真诚、有深度的初次平台打招呼语（用于BOSS直聘/前程无忧/猎聘）。

【招聘岗位】：%s
【候选人称呼】：%s
【候选人背景】：现任 %s | 工作年限 %s
【核心专业专长】：%s
【代表性单位/项目经历】：%s
【简历履历选段】：
%s

【严格撰写规范】：
1. 称呼候选人（如：%s），语气必须礼貌、真诚、尊重、专业。
2. 必须具体提及其简历中经历过的 1~2 个真实业务项目、技术落地成果或过往单位代表性业务（拒绝空泛套话，严禁出现只引用单个英文单词的情况），说明为什么该实战经历与透景生命当前【%s】岗位的业务方向高度契合。
3. 表达希望能与候选人深入交流岗位机会的诚意，并在末尾礼貌询问“方便了解一下吗？”。
4. 字数控制在 100~180 字之间。
5. 请直接输出打招呼文案内容，不要输出任何引言、标题、解释或前后缀！`,
			jobTitle, jobTitle, name, recentRole, workYears, strengthsDesc, evidence, excerpt, name, jobTitle)

		aiMsg, err := a.callAIChatMessages(cfg, []ChatMessage{
			{Role: "system", Content: "你是一名资深技术招聘顾问，擅长撰写打动优秀技术候选人的真诚、专业且高度定制的打招呼沟通语。直接输出招呼语，禁止输出任何无关解释。"},
			{Role: "user", Content: prompt},
		}, 0.5)

		if err == nil && strings.TrimSpace(aiMsg) != "" {
			cleanMsg := strings.TrimSpace(aiMsg)
			cleanMsg = strings.Trim(cleanMsg, "\"`'“”")
			if len([]rune(cleanMsg)) >= 30 {
				return cleanMsg, evidence
			}
		}
	}

	if company != "" && keyProject != "" {
		greeting = fmt.Sprintf("您好%s，看到您此前在%s担任%s期间，主导/参与了“%s”等核心方向的实战研发，技术落地经验非常扎实。透景生命目前正在招聘%s，您的业务沉淀与我们当前的研发方向非常契合，想与您交流一下岗位情况，方便了解一下吗？",
			name, company, recentRole, keyProject, jobTitle)
	} else if keyProject != "" {
		greeting = fmt.Sprintf("您好%s，关注到您在简历中关于“%s”等项目的深度实战经历，与透景生命目前招聘的【%s】岗位核心要求高度契合。我们非常认可您的专业沉淀，想与您深入交流一下岗位情况，方便了解一下吗？",
			name, keyProject, jobTitle)
	} else if techSkills != "" {
		greeting = fmt.Sprintf("您好%s，看到您在%s等领域具备深厚的技术功底与项目交付经验。透景生命目前正在招聘%s，想与您交流一下岗位情况，方便了解一下吗？",
			name, techSkills, jobTitle)
	} else {
		greeting = fmt.Sprintf("您好%s，透景生命目前正在招聘%s，关注到您的专业经历与该岗位高度契合，想与您交流一下岗位情况，方便了解一下吗？",
			name, jobTitle)
	}

	return greeting, evidence
}

// GenerateCandidateAIOutreachMessage 单人调用接入的 AI 模型重新深度定制润色招呼文案
func (a *App) GenerateCandidateAIOutreachMessage(projectID string, itemID string) (string, error) {
	project := a.GetProject(projectID)
	if project == nil {
		return "", fmt.Errorf("招聘项目不存在")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return "", err
	}

	var resumeID string
	err = db.QueryRow(`SELECT rv.legacy_resume_id FROM outreach_items oi
		JOIN project_candidates pc ON pc.id = oi.project_candidate_id
		JOIN resume_versions rv ON rv.candidate_id = pc.candidate_id
		WHERE oi.id = ? LIMIT 1`, itemID).Scan(&resumeID)
	if err != nil || resumeID == "" {
		resumeID = itemID
	}

	resumes := a.GetProjectResumes(projectID)
	var targetResume *Resume
	for _, r := range resumes {
		if r.ID == resumeID {
			targetResume = r
			break
		}
	}
	if targetResume == nil {
		return "", fmt.Errorf("候选人简历记录不存在")
	}

	msg, _ := a.generateAIOutreachGreeting(targetResume, project.JobConfig.Title, project.Name)
	if msg == "" {
		return "", fmt.Errorf("生成打招呼文案失败")
	}

	_, _ = db.Exec(`UPDATE outreach_items SET message = ? WHERE id = ?`, msg, itemID)

	return msg, nil
}

// The preview is a ranking snapshot. No platform message is sent here.
func (a *App) PreviewTopTenOutreach(projectID string) (OutreachBatch, error) {
	batch := OutreachBatch{ProjectID: projectID, Status: "draft", Items: make([]OutreachItem, 0)}
	project := a.GetProject(projectID)
	if project == nil {
		return batch, fmt.Errorf("招聘项目不存在")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return batch, err
	}
	batch.ProjectName = project.Name
	batch.ID = recruitmentID("outreach")
	batch.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	resumes := a.GetProjectRanking(projectID)
	seenLinks := make(map[string]bool)

	type targetItem struct {
		resume         *Resume
		linkID         string
		sourcePlatform string
		sourceID       string
		jobID          string
		name           string
	}
	var targets []targetItem

	for _, resume := range resumes {
		if len(targets) >= 10 {
			break
		}
		if resume.Status != "done" || resume.Analysis == nil || resume.Analysis.RedLineStatus == "failed" {
			continue
		}
		var linkID, sourcePlatform, sourceID string
		err := db.QueryRow(`SELECT pc.id,cs.platform,cs.platform_candidate_id FROM project_candidates pc
			JOIN resume_versions rv ON rv.candidate_id=pc.candidate_id
			LEFT JOIN candidate_sources cs ON cs.id=rv.source_id
			WHERE pc.project_id=? AND pc.archived_at='' AND rv.legacy_resume_id=?`, projectID, resume.ID).Scan(&linkID, &sourcePlatform, &sourceID)
		if err != nil {
			continue
		}
		if seenLinks[linkID] {
			continue
		}
		seenLinks[linkID] = true
		if sourcePlatform == "legacy" {
			sourcePlatform = ""
			sourceID = ""
		}
		var jobID string
		_ = db.QueryRow(`SELECT platform_job_id FROM platform_job_mappings WHERE platform=? AND project_id=? LIMIT 1`, sourcePlatform, projectID).Scan(&jobID)
		name := conversationCandidateName(resume)
		if resume.Analysis.CandidateName != "" {
			name = resume.Analysis.CandidateName
		}
		targets = append(targets, targetItem{
			resume:         resume,
			linkID:         linkID,
			sourcePlatform: sourcePlatform,
			sourceID:       sourceID,
			jobID:          jobID,
			name:           name,
		})
	}
	if len(targets) == 0 {
		return batch, fmt.Errorf("当前项目没有已完成分析且红线未确认不满足的候选人")
	}

	// 并发为排名前十的候选人深度定制招呼文案（充分利用接入的 AI 大模型）
	items := make([]OutreachItem, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(idx int, target targetItem) {
			defer wg.Done()
			msg, evi := a.generateAIOutreachGreeting(target.resume, project.JobConfig.Title, project.Name)
			items[idx] = OutreachItem{
				ID:                  recruitmentID("outreach_item"),
				ProjectCandidateID:  target.linkID,
				CandidateName:       target.name,
				Platform:            target.sourcePlatform,
				PlatformCandidateID: target.sourceID,
				PlatformJobID:       target.jobID,
				ProfileURL:          target.resume.URL,
				Message:             msg,
				Evidence:            evi,
				RedLineStatus:       target.resume.Analysis.RedLineStatus,
				AbilityScore:        target.resume.Analysis.AbilityScore,
				Rank:                idx + 1,
				Status:              "draft",
			}
		}(i, t)
	}
	wg.Wait()
	batch.Items = items

	tx, err := db.Begin()
	if err != nil {
		return batch, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO outreach_batches(id,project_id,status,created_at) VALUES(?,?,'draft',?)`, batch.ID, projectID, batch.CreatedAt); err != nil {
		return batch, err
	}
	for _, item := range batch.Items {
		meta, _ := json.Marshal(item)
		if _, err = tx.Exec(`INSERT INTO outreach_items(id,batch_id,project_candidate_id,platform,platform_candidate_id,platform_job_id,message,status,result_json,updated_at)
			VALUES(?,?,?,?,?,?,?,'draft',?,?)`, item.ID, batch.ID, item.ProjectCandidateID, item.Platform, item.PlatformCandidateID, item.PlatformJobID, item.Message, string(meta), batch.CreatedAt); err != nil {
			return batch, err
		}
	}
	return batch, tx.Commit()
}

func (a *App) GetOutreachBatch(batchID string) (OutreachBatch, error) {
	batch := OutreachBatch{Items: make([]OutreachItem, 0)}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return batch, err
	}
	err = db.QueryRow(`SELECT b.id,b.project_id,p.name,b.status,b.created_at,b.confirmed_at FROM outreach_batches b JOIN projects p ON p.id=b.project_id WHERE b.id=?`, batchID).
		Scan(&batch.ID, &batch.ProjectID, &batch.ProjectName, &batch.Status, &batch.CreatedAt, &batch.ConfirmedAt)
	if err != nil {
		return batch, err
	}
	rows, err := db.Query(`SELECT id,message,status,result_json FROM outreach_items WHERE batch_id=? ORDER BY updated_at,id`, batchID)
	if err != nil {
		return batch, err
	}
	defer rows.Close()
	for rows.Next() {
		var item OutreachItem
		var raw string
		if err := rows.Scan(&item.ID, &item.Message, &item.Status, &raw); err != nil {
			return batch, err
		}
		var meta OutreachItem
		if json.Unmarshal([]byte(raw), &meta) == nil {
			meta.ID = item.ID
			meta.Message = item.Message
			meta.Status = item.Status
			item = meta
		}
		batch.Items = append(batch.Items, item)
	}
	sort.Slice(batch.Items, func(i, j int) bool { return batch.Items[i].Rank < batch.Items[j].Rank })
	return batch, rows.Err()
}

// All current platform first-message adapters lack verified personalized-send
// receipts. Confirmation therefore records each chosen item as requiring
// manual handling; it never converts a click or default greeting into success.
func (a *App) ConfirmOutreachBatch(batchID string, approvals []OutreachApproval) (OutreachBatch, error) {
	batch, err := a.GetOutreachBatch(batchID)
	if err != nil {
		return batch, err
	}
	if batch.Status != "draft" {
		return batch, fmt.Errorf("批次已确认，不可重复发送")
	}
	if len(approvals) != len(batch.Items) {
		return batch, fmt.Errorf("请逐人确认本批全部候选人")
	}
	byID := make(map[string]OutreachItem)
	for _, item := range batch.Items {
		byID[item.ID] = item
	}
	seen := make(map[string]bool)
	db, err := a.getRecruitmentDB()
	if err != nil {
		return batch, err
	}
	tx, err := db.Begin()
	if err != nil {
		return batch, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, approval := range approvals {
		item, ok := byID[approval.ItemID]
		if !ok || seen[approval.ItemID] {
			return batch, fmt.Errorf("批次候选人列表不匹配")
		}
		seen[approval.ItemID] = true
		status, result := "excluded", "HR 未选择发送"
		if approval.Include {
			var active int
			if err = tx.QueryRow("SELECT COUNT(*) FROM project_candidates WHERE id=? AND archived_at=''", item.ProjectCandidateID).Scan(&active); err != nil {
				return batch, err
			}
			if active == 0 {
				return batch, fmt.Errorf("候选人 %s 已从人才库删除，请重新生成预览", item.CandidateName)
			}
			if item.RedLineStatus == "pending" && !approval.ConfirmPending {
				return batch, fmt.Errorf("候选人 %s 的红线待核实，发送前须单独勾选", item.CandidateName)
			}
			if strings.TrimSpace(approval.Message) == "" {
				return batch, fmt.Errorf("候选人 %s 的招呼语为空", item.CandidateName)
			}
			var sent int
			err = tx.QueryRow(`SELECT COUNT(*) FROM outreach_items oi JOIN outreach_batches b ON b.id=oi.batch_id
				WHERE b.project_id=? AND oi.status='sent' AND
				(oi.project_candidate_id=? OR (?<>'' AND oi.platform=? AND oi.platform_candidate_id=? AND oi.platform_job_id=?))`,
				batch.ProjectID, item.ProjectCandidateID, item.PlatformCandidateID, item.Platform, item.PlatformCandidateID, item.PlatformJobID).Scan(&sent)
			if err != nil {
				return batch, err
			}
			if sent > 0 {
				return batch, fmt.Errorf("候选人 %s 已有成功沟通记录，请核对平台会话", item.CandidateName)
			}
			status, result = "needs_manual", "平台首次定制招呼缺少可核实的自动发送能力；请在平台逐人发送并登记回执"
			if item.PlatformCandidateID == "" {
				result = "缺少稳定的平台候选人 ID；请人工核对目标后发送"
			}
		}
		meta := item
		meta.Message = approval.Message
		meta.Status = status
		meta.Result = result
		raw, _ := json.Marshal(meta)
		if _, err = tx.Exec(`UPDATE outreach_items SET message=?,status=?,result_json=?,updated_at=? WHERE id=?`, approval.Message, status, string(raw), now, item.ID); err != nil {
			return batch, err
		}
	}
	if _, err = tx.Exec(`UPDATE outreach_batches SET status='confirmed',confirmed_at=? WHERE id=?`, now, batchID); err != nil {
		return batch, err
	}
	if err = tx.Commit(); err != nil {
		return batch, err
	}
	return a.GetOutreachBatch(batchID)
}

func (a *App) RecordOutreachResult(itemID, status, platformReceipt, actor string) error {
	if status != "sent" && status != "failed" && status != "uncertain" {
		return fmt.Errorf("结果状态无效")
	}
	if strings.TrimSpace(platformReceipt) == "" || strings.TrimSpace(actor) == "" {
		return fmt.Errorf("请提供平台回执和登记人")
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
	var raw, current, projectID string
	err = tx.QueryRow(`SELECT oi.result_json,oi.status,b.project_id FROM outreach_items oi JOIN outreach_batches b ON b.id=oi.batch_id WHERE oi.id=?`, itemID).Scan(&raw, &current, &projectID)
	if err != nil {
		return err
	}
	if current != "needs_manual" && current != "uncertain" {
		return fmt.Errorf("当前状态不可登记发送结果")
	}
	var item OutreachItem
	if err = json.Unmarshal([]byte(raw), &item); err != nil {
		return err
	}
	if status == "sent" {
		var prior int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM outreach_items oi JOIN outreach_batches b ON b.id=oi.batch_id
			WHERE b.project_id=? AND oi.id<>? AND oi.project_candidate_id=? AND oi.status='sent'`, projectID, itemID, item.ProjectCandidateID).Scan(&prior); err != nil {
			return err
		}
		if prior > 0 {
			return fmt.Errorf("该候选人已有成功沟通记录，请核对平台会话")
		}
	}
	item.Status = status
	item.Result = fmt.Sprintf("%s 登记平台回执：%s", strings.TrimSpace(actor), strings.TrimSpace(platformReceipt))
	encoded, _ := json.Marshal(item)
	result, err := tx.Exec(`UPDATE outreach_items SET status=?,result_json=?,updated_at=? WHERE id=? AND status IN ('needs_manual','uncertain')`,
		status, string(encoded), time.Now().UTC().Format(time.RFC3339Nano), itemID)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return sql.ErrNoRows
	}
	if _, err = tx.Exec(`INSERT INTO follow_up_events(id,project_candidate_id,event_type,note,actor,created_at)
		VALUES(?,?,?,?,?,?)`, recruitmentID("followup"), item.ProjectCandidateID, "communication:platform", item.Result, strings.TrimSpace(actor), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}
