package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type JobConversationResult struct {
	Status   string                   `json:"status"`
	Message  string                   `json:"message"`
	FilePath string                   `json:"filePath,omitempty"`
	FileName string                   `json:"fileName,omitempty"`
	Baseline []string                 `json:"baseline,omitempty"`
	JobID    string                   `json:"jobId,omitempty"`
	JobTitle string                   `json:"jobTitle,omitempty"`
	TargetID string                   `json:"targetId,omitempty"`
	Points   int                      `json:"points,omitempty"`
	QuoteID  string                   `json:"quoteId,omitempty"`
	Jobs     []JobCommunicationOption `json:"jobs,omitempty"`
	HasMore  bool                     `json:"hasMore,omitempty"`
}

type JobCommunicationOption struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Company       string `json:"company,omitempty"`
	WorkArea      string `json:"workArea,omitempty"`
	Source        string `json:"source,omitempty"`
	PublishStatus string `json:"publishStatus,omitempty"`
}

type pending51JobHiQuote struct {
	ResumeID     string
	ProjectTitle string
	JobID        string
	JobTitle     string
	TargetID     string
	Points       int
	Expires      time.Time
}

func (a *App) load51JobContactResume(resumeID string) (*Resume, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return nil, fmt.Errorf("候选人编号无效")
	}
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
	if err != nil {
		return nil, fmt.Errorf("候选人档案不存在")
	}
	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return nil, fmt.Errorf("候选人档案无法读取：%w", err)
	}
	if !is51JobCandidateDetailURL(resume.URL) {
		return nil, fmt.Errorf("该候选人没有可核对的 51job 详情地址")
	}
	return &resume, nil
}

func (a *App) save51JobContactResume(resume *Resume) error {
	encoded, err := json.MarshalIndent(resume, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(a.getDataDir(), "resumes", resume.ID+".json"), encoded, 0600); err != nil {
		return err
	}
	a.syncRecruitmentResume(resume)
	return nil
}

func is51JobCandidateDetailURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "ehire.51job.com" ||
		parsed.Path != "/Revision/talent/resume/detail" {
		return false
	}
	id := parsed.Query().Get("resumeId")
	if id == "" {
		return false
	}
	for _, char := range id {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func conversationCandidateName(resume *Resume) string {
	name := regexp.MustCompile(`^【.*?】\s*`).ReplaceAllString(resume.FileName, "")
	name = strings.Split(name, "_")[0]
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	if resume.Analysis != nil {
		return strings.TrimSpace(resume.Analysis.CandidateName)
	}
	return ""
}

func (a *App) conversationJobTitle(resume *Resume) string {
	if resume.ProjectID == "" {
		return ""
	}
	project := a.GetProject(resume.ProjectID)
	if project == nil {
		return ""
	}
	return strings.TrimSpace(project.JobConfig.Title)
}

func (a *App) run51JobConversation(resume *Resume, action, message string, quote *pending51JobHiQuote) JobConversationResult {
	scriptCandidates := []string{
		filepath.Join("scripts", "51job_conversation.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "51job_conversation.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "51job_conversation.js"),
	}
	var script string
	for _, candidate := range scriptCandidates {
		if _, err := os.Stat(candidate); err == nil {
			script = candidate
			break
		}
	}
	if script == "" {
		return JobConversationResult{Status: "error", Message: "缺少 51job 沟通脚本，请检查 scripts 目录"}
	}
	timeout := 30 * time.Second
	if action == "preview_hi" || action == "list_jobs" {
		timeout = 45 * time.Second // 首次详情页加载、职位搜索和平台点数查询均包含在预览内。
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	dataDir := filepath.Join(a.getDataDir(), "candidates_multi")
	args := []string{script, "--action", action, "--candidate-url", resume.URL,
		"--candidate-name", conversationCandidateName(resume), "--resume-record", resume.ID,
		"--data-dir", dataDir}
	if action != "check_resume" {
		jobTitle := a.conversationJobTitle(resume)
		if quote != nil && quote.JobTitle != "" {
			jobTitle = quote.JobTitle
		}
		args = append(args, "--job-title", jobTitle)
	}
	if message != "" {
		args = append(args, "--message", message)
	}
	if quote != nil {
		args = append(args, "--expected-job-id", quote.JobID)
		if quote.Points > 0 {
			args = append(args, "--expected-points", strconv.Itoa(quote.Points))
		}
		if quote.TargetID != "" {
			args = append(args, "--target-id", quote.TargetID)
		}
	}
	if action == "check_resume" {
		baseline, _ := json.Marshal(resume.ContactAttachmentBaseline)
		args = append(args, "--known-links", string(baseline))
	}
	cmd := exec.CommandContext(ctx, "node", args...)
	hideConsoleWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return JobConversationResult{Status: "uncertain", Message: "51job 响应超时，请核对聊天记录后再决定是否重试"}
		}
		if action == "confirm_hi" {
			return JobConversationResult{Status: "uncertain", Message: "51job 打招呼进程中断，无法确认平台是否已发送；请查看聊天记录，暂勿重复操作"}
		}
		return JobConversationResult{Status: "error", Message: fmt.Sprintf("51job 沟通脚本未完成：%v", err)}
	}
	var result JobConversationResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(output))), &result); err != nil || result.Status == "" {
		return JobConversationResult{Status: "uncertain", Message: "无法确认 51job 返回状态，请先查看聊天记录"}
	}
	return result
}

// List51JobCommunicationJobs 返回平台当前可选择的沟通职位，不会发起Hi聊。
func (a *App) List51JobCommunicationJobs(resumeID, keyword string) JobConversationResult {
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	resume, err := a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	if resume.ContactStatus != "" {
		return JobConversationResult{Status: "error", Message: "此候选人已有沟通记录，请先查看 51job 聊天记录"}
	}
	keyword = strings.TrimSpace(keyword)
	if len([]rune(keyword)) > 100 {
		return JobConversationResult{Status: "error", Message: "职位搜索词不能超过 100 字"}
	}
	result := a.run51JobConversation(resume, "list_jobs", keyword, nil)
	if result.Status != "jobs" {
		return result
	}
	for _, job := range result.Jobs {
		if job.ID == "" || job.Title == "" {
			return JobConversationResult{Status: "error", Message: "51job 职位列表包含无法核对的职位，未发起Hi聊"}
		}
	}
	return result
}

func (a *App) store51JobHiQuote(resume *Resume, result JobConversationResult) JobConversationResult {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return JobConversationResult{Status: "error", Message: "无法生成本次操作凭据，未发起Hi聊"}
	}
	token := hex.EncodeToString(random)
	if a.contactPending == nil {
		a.contactPending = make(map[string]pending51JobHiQuote)
	}
	for key, pending := range a.contactPending {
		if pending.ResumeID == resume.ID || time.Now().After(pending.Expires) {
			delete(a.contactPending, key)
		}
	}
	a.contactPending[token] = pending51JobHiQuote{
		ResumeID: resume.ID, ProjectTitle: a.conversationJobTitle(resume),
		JobID: result.JobID, JobTitle: result.JobTitle,
		TargetID: result.TargetID, Points: result.Points, Expires: time.Now().Add(2 * time.Minute),
	}
	return JobConversationResult{Status: "needs_confirmation", QuoteID: token,
		JobTitle: result.JobTitle, Points: result.Points,
		Message: fmt.Sprintf("51job 显示本次预计消耗 %d 个Hi聊数", result.Points)}
}

// Prepare51JobHiChat 只读取本次沟通职位和平台显示的点数，不触发平台发送。
func (a *App) Prepare51JobHiChat(resumeID string) JobConversationResult {
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	resume, err := a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	if resume.ContactStatus != "" {
		return JobConversationResult{Status: "error", Message: "此候选人已有沟通记录，请先查看 51job 聊天记录"}
	}
	jobTitle := a.conversationJobTitle(resume)
	if jobTitle == "" {
		return JobConversationResult{Status: "needs_job_selection", Message: "招聘项目未设置岗位名称，无法自动选择沟通职位"}
	}
	result := a.run51JobConversation(resume, "preview_hi", "", nil)
	if result.Status != "quote" {
		return result
	}
	if result.JobTitle != jobTitle || result.JobID == "" || result.TargetID == "" || result.Points <= 0 {
		return JobConversationResult{Status: "error", Message: "51job 职位或点数无法核实，本次未发起Hi聊"}
	}
	return a.store51JobHiQuote(resume, result)
}

// Prepare51JobHiChatForJob 以 HR 指定的 51job 职位 ID 为准，脚本须在平台重新找到同一职位后才能报价。
func (a *App) Prepare51JobHiChatForJob(resumeID, jobID, jobTitle string) JobConversationResult {
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	resume, err := a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	if resume.ContactStatus != "" {
		return JobConversationResult{Status: "error", Message: "此候选人已有沟通记录，请先查看 51job 聊天记录"}
	}
	jobID, jobTitle = strings.TrimSpace(jobID), strings.TrimSpace(jobTitle)
	if jobID == "" || jobTitle == "" || len([]rune(jobID)) > 100 || len([]rune(jobTitle)) > 200 {
		return JobConversationResult{Status: "error", Message: "请选择 51job 返回的有效沟通职位"}
	}
	selected := &pending51JobHiQuote{JobID: jobID, JobTitle: jobTitle}
	result := a.run51JobConversation(resume, "preview_hi", "", selected)
	if result.Status != "quote" {
		return result
	}
	if result.JobID != jobID || result.JobTitle != jobTitle || result.TargetID == "" || result.Points <= 0 {
		return JobConversationResult{Status: "error", Message: "51job 选中职位或点数无法核实，本次未发起Hi聊"}
	}
	return a.store51JobHiQuote(resume, result)
}

// Confirm51JobHiChat 只接受刚刚预览过的一次性报价；脚本还会在点击前重新核对职位和点数。
func (a *App) Confirm51JobHiChat(resumeID, quoteID string) JobConversationResult {
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	quote, exists := a.contactPending[quoteID]
	if !exists || quote.ResumeID != resumeID || time.Now().After(quote.Expires) {
		return JobConversationResult{Status: "error", Message: "本次点数确认已过期，请重新预览；未点击发起Hi聊"}
	}
	delete(a.contactPending, quoteID) // 凭据先失效，防止双击和不确定结果时重放。
	resume, err := a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	if resume.ContactStatus != "" || a.conversationJobTitle(resume) != quote.ProjectTitle {
		return JobConversationResult{Status: "error", Message: "候选人沟通状态或岗位已变化，未点击发起Hi聊"}
	}
	result := a.run51JobConversation(resume, "confirm_hi", "", &quote)
	if result.Status != "sent" {
		return result
	}
	resume, err = a.load51JobContactResume(resumeID)
	if err != nil || resume.ContactStatus != "" {
		return JobConversationResult{Status: "uncertain", Message: "平台显示已发送，但本地档案状态无法确认；请查看聊天记录，暂勿重复操作"}
	}
	resume.ContactStatus = "greeted"
	resume.ContactUpdatedAt = time.Now().Format(time.RFC3339)
	if err := a.save51JobContactResume(resume); err != nil {
		return JobConversationResult{Status: "uncertain", Message: "平台显示已发送，但本地记录保存失败；请查看聊天记录，暂勿重复操作"}
	}
	return result
}

// Start51JobConversation 发送 HR 确认过的索要简历文字；首次 Hi聊走点数预览和二次确认。
func (a *App) Start51JobConversation(resumeID, action, message string) JobConversationResult {
	if action == "greet" {
		return JobConversationResult{Status: "error", Message: "请使用一键打招呼，先核对本次Hi聊点数"}
	}
	if action != "ask_resume" {
		return JobConversationResult{Status: "error", Message: "不支持的沟通步骤"}
	}
	if len([]rune(strings.TrimSpace(message))) == 0 || len([]rune(message)) > 500 {
		return JobConversationResult{Status: "error", Message: "沟通内容需为 1–500 字"}
	}
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	resume, err := a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	if resume.ContactStatus == "resume_requested" || resume.ContactStatus == "resume_received" {
		return JobConversationResult{Status: "error", Message: "已向此候选人索要过简历，请检查回复，避免重复发送"}
	}
	if resume.ContactStatus != "greeted" {
		return JobConversationResult{Status: "error", Message: "请先完成并核实 51job 打招呼，再索要完整简历"}
	}
	result := a.run51JobConversation(resume, action, message, nil)
	if result.Status != "sent" {
		return result
	}
	resume, err = a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "uncertain", Message: "消息已发送，但本地档案状态无法保存；请核对聊天记录，暂勿重复发送"}
	}
	resume.ContactStatus = "resume_requested"
	resume.ContactAttachmentBaseline = result.Baseline
	resume.ContactUpdatedAt = time.Now().Format(time.RFC3339)
	if err := a.save51JobContactResume(resume); err != nil {
		return JobConversationResult{Status: "uncertain", Message: "消息已发送，但本地沟通状态保存失败；请核对聊天记录，暂勿重复发送"}
	}
	return result
}

// Check51JobResumeReply 只导入该候选人聊天记录里的完整文件；未收到时保持等待，不虚报成功。
func (a *App) Check51JobResumeReply(resumeID string) JobConversationResult {
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	resume, err := a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	if resume.ContactStatus != "resume_requested" {
		return JobConversationResult{Status: "error", Message: "请先向这位候选人索要完整简历"}
	}
	result := a.run51JobConversation(resume, "check_resume", "", nil)
	if result.Status != "downloaded" {
		return result
	}
	resume, err = a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: "附件已下载，但本地候选人档案无法读取"}
	}
	if resume.ContactStatus != "resume_requested" || resume.HasAttachment {
		return JobConversationResult{Status: "error", Message: "候选人档案已更新，未覆盖已有完整简历；请核对后再操作"}
	}
	root := filepath.Join(a.getDataDir(), "candidates_multi", "51job_received", resume.ID)
	absRoot, _ := filepath.Abs(root)
	absFile, pathErr := filepath.Abs(result.FilePath)
	if pathErr != nil || !strings.HasPrefix(strings.ToLower(absFile), strings.ToLower(absRoot)+string(os.PathSeparator)) {
		return JobConversationResult{Status: "error", Message: "下载文件不在候选人专用目录，已拒绝导入"}
	}
	resume.ContactStatus = "resume_received"
	resume.ContactUpdatedAt = time.Now().Format(time.RFC3339)
	if err := a.save51JobContactResume(resume); err != nil {
		return JobConversationResult{Status: "error", Message: "附件已下载，但本地沟通状态保存失败，未启动分析"}
	}
	if _, err := a.AttachResumeFile(resume.ID, absFile); err != nil {
		resume.ContactStatus = "resume_requested"
		_ = a.save51JobContactResume(resume)
		return JobConversationResult{Status: "error", Message: fmt.Sprintf("附件已下载但未能解析：%v", err)}
	}
	verified, err := a.load51JobContactResume(resumeID)
	if err != nil || !verified.HasAttachment || verified.AttachmentPath != absFile {
		return JobConversationResult{Status: "error", Message: "附件分析流程未能确认保存成功，请检查本地档案"}
	}
	if a.config.AI.APIKey == "" {
		result.Message = "已导入该候选人回复的完整简历；请先配置 AI，再启动结合分析"
	} else {
		result.Message = "已导入该候选人回复的完整简历，正在结合初筛材料分析"
	}
	return result
}
