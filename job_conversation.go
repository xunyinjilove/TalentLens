package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type JobConversationResult struct {
	Status          string                   `json:"status"`
	Message         string                   `json:"message"`
	FilePath        string                   `json:"filePath,omitempty"`
	FileName        string                   `json:"fileName,omitempty"`
	Baseline        []string                 `json:"baseline,omitempty"`
	JobID           string                   `json:"jobId,omitempty"`
	JobTitle        string                   `json:"jobTitle,omitempty"`
	TargetID        string                   `json:"targetId,omitempty"`
	Points          int                      `json:"points,omitempty"`
	QuoteID         string                   `json:"quoteId,omitempty"`
	Jobs            []JobCommunicationOption `json:"jobs,omitempty"`
	HasMore         bool                     `json:"hasMore,omitempty"`
	SuggestedJobID  string                   `json:"suggestedJobId,omitempty"`
	Replies         []string                 `json:"replies,omitempty"`
	AttachmentNames []string                 `json:"attachmentNames,omitempty"`
}

type JobCommunicationOption struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Company       string `json:"company,omitempty"`
	CompanyID     string `json:"companyId,omitempty"`
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
	if !is51JobCandidateDetailURL(resume.URL) || (resume.PlatformCandidateID != "" && !is51JobResumeURLForCandidate(resume.URL, resume.PlatformCandidateID)) {
		return nil, fmt.Errorf("该候选人没有可核对的 51job 详情地址")
	}
	// A verified platform identity may have several local document versions.
	// Reuse contact state only for that exact identity, never by display name.
	copies := []*Resume{&resume}
	if entries, err := os.ReadDir(filepath.Join(a.getDataDir(), "resumes")); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || entry.Name() == resumeID+".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", entry.Name()))
			var other Resume
			if err == nil && json.Unmarshal(data, &other) == nil {
				copies = append(copies, &other)
			}
		}
	}
	merge51JobContactStates(copies)
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
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "resume:contact_updated", map[string]string{"id": resume.ID, "projectId": resume.ProjectID})
	}
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

func is51JobResumeURLForCandidate(raw, candidateID string) bool {
	if !is51JobCandidateDetailURL(raw) || candidateID == "" {
		return false
	}
	parsed, _ := url.Parse(raw)
	return parsed.Query().Get("resumeId") == candidateID
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
	return a.run51JobConversationContext(context.Background(), resume, action, message, quote)
}

func (a *App) run51JobConversationContext(parent context.Context, resume *Resume, action, message string, quote *pending51JobHiQuote) JobConversationResult {
	if err := parent.Err(); err != nil {
		return JobConversationResult{Status: "interrupted", Message: "附件同步已中止"}
	}
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
	// 确保 51job 专用浏览器已启动
	a.EnsurePlatformBrowser("51job")

	timeout := 30 * time.Second
	if action == "preview_hi" || action == "list_jobs" || action == "list_account_jobs" || action == "check_resume" || action == "check_replies" {
		timeout = 75 * time.Second // 包括候选人入口就绪重试与职位、点数读取。
	}
	if action == "check_resume" || action == "check_replies" {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	dataDir := filepath.Join(a.getDataDir(), "candidates_multi")
	args := []string{script, "--action", action, "--data-dir", dataDir}
	if resume != nil {
		args = append(args, "--candidate-url", resume.URL, "--candidate-name", conversationCandidateName(resume), "--resume-record", resume.ID)
	} else if action != "list_account_jobs" {
		return JobConversationResult{Status: "error", Message: "缺少需要核验的候选人，未执行沟通操作"}
	}
	if resume != nil && action != "check_resume" && action != "check_replies" {
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
	// Read every inbound attachment, including one sent before the request.
	// File hashes below prevent duplicate imports.
	cmd := exec.CommandContext(ctx, "node", args...)
	hideConsoleWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			if action == "list_jobs" || action == "list_account_jobs" {
				return JobConversationResult{Status: "error", Message: "51job 职位目录读取超时，未发起Hi聊；请确认浏览器已登录后重试"}
			}
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

// List51JobAccountCommunicationJobs reads the account's actual communication
// directory without requiring a fresh candidate or opening a Hi-chat window.
func (a *App) List51JobAccountCommunicationJobs(keyword string) JobConversationResult {
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	keyword = strings.TrimSpace(keyword)
	if len([]rune(keyword)) > 100 {
		return JobConversationResult{Status: "error", Message: "职位搜索词不能超过 100 字"}
	}
	return a.run51JobConversation(nil, "list_account_jobs", keyword, nil)
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
	if reason := a.known51JobHardFailure(resume); reason != "" {
		return JobConversationResult{Status: "error", Message: "候选人已知不符合项目硬性条件，已跳过打招呼：" + reason}
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
	if reason := a.known51JobHardFailure(resume); reason != "" {
		return JobConversationResult{Status: "error", Message: "候选人已知不符合项目硬性条件，已跳过打招呼：" + reason}
	}
	if quote.Points > 100 {
		return JobConversationResult{Status: "error", Message: "本次超过 100 点限额，未发起Hi聊"}
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

// Start51JobConversation is kept only for old desktop bindings. Resume requests
// are retired; attachment synchronization is a read-only operation.
func (a *App) Start51JobConversation(resumeID, action, message string) JobConversationResult {
	return JobConversationResult{Status: "error", Message: "51job 索要简历已停用；请使用同步附件简历读取已收到的文件"}
}

// Send51JobOutreachMessage sends the HR-approved second message only after the
// platform's default first HiChat has been verified. The script checks the
// exact candidate chat and a new success marker for this text.
func (a *App) Send51JobOutreachMessage(resumeID, message string) JobConversationResult {
	if len([]rune(strings.TrimSpace(message))) == 0 || len([]rune(message)) > 500 {
		return JobConversationResult{Status: "error", Message: "定制招呼语需为 1–500 字"}
	}
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	resume, err := a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	if resume.ContactStatus != "greeted" {
		return JobConversationResult{Status: "error", Message: "尚未核实平台默认 Hi聊已发送，定制招呼语未发送"}
	}
	result := a.run51JobConversation(resume, "send_outreach", message, nil)
	if result.Status != "sent" {
		return result
	}
	resume.ContactUpdatedAt = time.Now().Format(time.RFC3339)
	if err := a.save51JobContactResume(resume); err != nil {
		return JobConversationResult{Status: "uncertain", Message: "平台显示定制文案已发出，但本地记录保存失败；请核对聊天记录"}
	}
	return result
}

// Check51JobResumeReply 只导入该候选人聊天记录里的完整文件；未收到时保持等待，不虚报成功。
func (a *App) Check51JobResumeReply(resumeID string) JobConversationResult {
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	return a.check51JobResumeReplyLocked(resumeID)
}

func (a *App) check51JobResumeReplyLocked(resumeID string) JobConversationResult {
	return a.check51JobResumeReplyLockedContext(context.Background(), resumeID)
}

func (a *App) check51JobResumeReplyContext(ctx context.Context, resumeID string) JobConversationResult {
	a.contactMutex.Lock()
	defer a.contactMutex.Unlock()
	return a.check51JobResumeReplyLockedContext(ctx, resumeID)
}

func (a *App) check51JobResumeReplyLockedContext(ctx context.Context, resumeID string) JobConversationResult {
	resume, err := a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: err.Error()}
	}
	result := a.run51JobConversationContext(ctx, resume, "check_resume", "", nil)
	if result.Status != "downloaded" {
		if result.Status == "attachment_pending" {
			resume.AttachmentPendingReason = result.Message
			_ = a.save51JobContactResume(resume)
		} else if (result.Status == "waiting" || result.Status == "reply") && resume.AttachmentPendingReason != "" {
			resume.AttachmentPendingReason = ""
			_ = a.save51JobContactResume(resume)
		}
		return result
	}
	resume, err = a.load51JobContactResume(resumeID)
	if err != nil {
		return JobConversationResult{Status: "error", Message: "附件已下载，但本地候选人档案无法读取"}
	}
	root := filepath.Join(a.getDataDir(), "candidates_multi", "51job_received", resume.ID)
	absRoot, _ := filepath.Abs(root)
	absFile, pathErr := filepath.Abs(result.FilePath)
	if pathErr != nil || !strings.HasPrefix(strings.ToLower(absFile), strings.ToLower(absRoot)+string(os.PathSeparator)) {
		return JobConversationResult{Status: "error", Message: "下载文件不在候选人专用目录，已拒绝导入"}
	}
	if resume.AttachmentPath != "" {
		newBytes, newErr := os.ReadFile(absFile)
		oldBytes, oldErr := os.ReadFile(resume.AttachmentPath)
		if newErr == nil && oldErr == nil && sha256.Sum256(newBytes) == sha256.Sum256(oldBytes) {
			if resume.AttachmentPendingReason != "" {
				resume.AttachmentPendingReason = ""
				_ = a.save51JobContactResume(resume)
			}
			result.Status = "already_current"
			result.Message = "已收到并保存这份附件简历，无需再次索要"
			return result
		}
	}
	if newBytes, readErr := os.ReadFile(absFile); readErr == nil {
		digest := fmt.Sprintf("%x", sha256.Sum256(newBytes))
		for _, old := range resume.AttachmentHistory {
			if old.SHA256 == digest {
				if resume.AttachmentPendingReason != "" {
					resume.AttachmentPendingReason = ""
					_ = a.save51JobContactResume(resume)
				}
				result.Status = "already_current"
				result.Message = "这份附件此前已导入，保留较新的当前版本，无需重复索要"
				return result
			}
		}
	}
	previousStatus := resume.ContactStatus
	resume.ContactStatus = "resume_received"
	resume.ContactUpdatedAt = time.Now().Format(time.RFC3339)
	if err := a.save51JobContactResume(resume); err != nil {
		return JobConversationResult{Status: "error", Message: "附件已下载，但本地沟通状态保存失败，未启动分析"}
	}
	if _, err := a.AttachResumeFile(resume.ID, absFile); err != nil {
		resume.ContactStatus = previousStatus
		resume.AttachmentPendingReason = fmt.Sprintf("附件已下载但待解析：%v", err)
		_ = a.save51JobContactResume(resume)
		return JobConversationResult{Status: "attachment_pending", Message: resume.AttachmentPendingReason, FilePath: absFile, FileName: result.FileName}
	}
	verified, err := a.load51JobContactResume(resumeID)
	if err != nil || !verified.HasAttachment || verified.AttachmentPath != absFile {
		return JobConversationResult{Status: "error", Message: "附件分析流程未能确认保存成功，请检查本地档案"}
	}
	verified.AttachmentFileName = result.FileName
	if n := len(verified.AttachmentHistory); n > 0 {
		verified.AttachmentHistory[n-1].FileName = result.FileName
	}
	_ = a.save51JobContactResume(verified)
	if a.config.AI.APIKey == "" {
		result.Message = "已导入该候选人回复的完整简历；请先配置 AI，再启动结合分析"
	} else {
		result.Message = "已导入该候选人回复的完整简历，正在结合初筛材料分析"
	}
	return result
}
