package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"embed"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// 版本信息
const AppVersion = "1.6.0"
const GitHubRepo = "xunyinjilove/TalentLens"

//go:embed all:frontend/dist
var assets embed.FS

// Config 配置结构
type Config struct {
	AI   AIConfig   `json:"ai"`
	Job  JobConfig  `json:"job"`
}

// AIConfig AI配置
type AIConfig struct {
	Provider   string `json:"provider"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model"`
	MaxRetries int    `json:"max_retries"`
	Timeout    int    `json:"timeout"`
}

// JobConfig 岗位配置
type JobConfig struct {
	Title           string   `json:"title"`
	Requirements    []string `json:"requirements"`
	RequiredSkills  []string `json:"required_skills"`
	ExperienceYears int      `json:"experience_years"`
	EducationLevel  string   `json:"education_level"`
	JobDescription  string   `json:"job_description,omitempty"`
}

// Project 招聘项目
type Project struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Department string    `json:"department,omitempty"`
	Headcount  int       `json:"headcount,omitempty"`
	Recruiter  string    `json:"recruiter,omitempty"`
	Remark     string    `json:"remark,omitempty"`
	JobConfig  JobConfig `json:"job_config"`
	ResumeIDs  []string  `json:"resume_ids"`
	Status     string    `json:"status"` // draft/analyzing/completed
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ConsistencyResult 双源真实度与一致性核验结果
type ConsistencyResult struct {
	Status  string   `json:"status"`  // "consistent" | "warning" | "conflict"
	Summary string   `json:"summary"` // 一致性核验总结
	Details []string `json:"details"` // 具体核验详情
}

// Resume 简历结构
type Resume struct {
	ID                 string             `json:"id"`
	ProjectID          string             `json:"project_id"`
	FileName           string             `json:"file_name"`
	FilePath           string             `json:"file_path"`
	FileType           string             `json:"file_type"`
	FileSize           int64              `json:"file_size"`
	Content            string             `json:"content"`
	URL                string             `json:"url,omitempty"`
	Email              string             `json:"email,omitempty"`
	Status             string             `json:"status"`
	Score              int                `json:"score"`
	InitialScore       int                `json:"initial_score,omitempty"`
	FinalScore         int                `json:"final_score,omitempty"`
	HasAttachment      bool               `json:"has_attachment,omitempty"`
	AttachmentPath     string             `json:"attachment_path,omitempty"`
	AttachmentFileName string             `json:"attachment_file_name,omitempty"`
	AttachmentContent  string             `json:"attachment_content,omitempty"`
	IsMergedAnalysis   bool               `json:"is_merged_analysis,omitempty"`
	ErrorMessage       string             `json:"error_message,omitempty"`
	Analysis           *AnalysisResult    `json:"analysis,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
}

// AnalysisResult AI分析结果
type AnalysisResult struct {
	OverallScore    float64 `json:"overall_score"`
	SkillMatch      float64 `json:"skill_match"`
	ExperienceMatch float64 `json:"experience_match"`
	EducationMatch  float64 `json:"education_match"`
	Recommendation  string `json:"recommendation"`

	// 结合分析演进与双源核验
	ConsistencyCheck  *ConsistencyResult `json:"consistency_check,omitempty"`
	ScoreDiff         int                `json:"score_diff,omitempty"`
	ScoreChangeReason string             `json:"score_change_reason,omitempty"`

	// 详细分析维度
	SkillDetail      string `json:"skill_detail"`
	ExperienceDetail string `json:"experience_detail"`
	EducationDetail  string `json:"education_detail"`

	// 候选人信息提取
	CandidateName string `json:"candidate_name"`
	WorkYears     string `json:"work_years"`
	Education     string `json:"education"`
	CurrentRole   string `json:"current_role"`

	// 详细评价
	Strengths  []string `json:"strengths"`
	Weaknesses []string `json:"weaknesses"`
	Risks      []string `json:"risks"`
	Summary    string   `json:"summary"`

	// 面试建议
	InterviewSuggestions []string `json:"interview_suggestions"`

	// AI 预面试提纲与参考回答（5组）
	InterviewQA []InterviewQuestion `json:"interview_qa,omitempty"`

	AnalyzedAt string `json:"analyzed_at"`
}

// InterviewQuestion 结构化面试问题及基于简历的参考回答
type InterviewQuestion struct {
	Category        string `json:"category"`         // 技术深度 / 项目实战 / 真实性与短板
	Question        string `json:"question"`         // 针对性面试提问
	ReferenceAnswer string `json:"reference_answer"` // 基于简历的参考要点与依据
}

// OpenAI API 请求/响应结构
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type App struct {
	ctx             context.Context
	config          Config
	activeProjectID string // 当前活跃的项目ID（前端设置）
	bossCmd         *exec.Cmd
	bossMutex       sync.Mutex
}

func NewApp() *App {
	return &App{}
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "TalentLens",
		Width:            1200,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		Frameless:        true,
		DisableResize:    false,
		StartHidden:      false,
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Println("Error:", err.Error())
	}
}

// WindowMinimize 最小化窗口
func (a *App) WindowMinimize() {
	runtime.WindowMinimise(a.ctx)
}

// WindowMaximize 最大化/还原窗口
func (a *App) WindowMaximize() {
	runtime.WindowToggleMaximise(a.ctx)
}

// WindowClose 关闭窗口
func (a *App) WindowClose() {
	runtime.Quit(a.ctx)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadConfig()

	// 监听原生文件拖拽（Wails 提供真实文件路径）
	runtime.OnFileDrop(ctx, func(x, y int, paths []string) {
		log.Printf("[OnFileDrop] 收到 %d 个文件, 项目=%s", len(paths), a.activeProjectID)
		supportedExts := map[string]bool{
			".pdf": true, ".docx": true, ".doc": true,
			".jpg": true, ".jpeg": true, ".png": true, ".bmp": true, ".gif": true, ".webp": true,
		}
		added := 0
		for _, fp := range paths {
			ext := strings.ToLower(filepath.Ext(fp))
			if !supportedExts[ext] {
				continue
			}
			info, err := os.Stat(fp)
			if err != nil {
				continue
			}
			id := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(fp))

			if a.activeProjectID != "" {
				a.RegisterResumeToProject(a.activeProjectID, id, filepath.Base(fp), fp, ext, info.Size())
			} else {
				a.RegisterResume(id, filepath.Base(fp), fp, ext, info.Size())
			}

			// 通知前端（包含提取的内容）
			resumePath := filepath.Join(a.getDataDir(), "resumes", id+".json")
			data, _ := os.ReadFile(resumePath)
			var resume Resume
			json.Unmarshal(data, &resume)
			runtime.EventsEmit(a.ctx, "resume:dropped", &resume)
			added++
		}
		log.Printf("[OnFileDrop] 成功添加 %d 个文件", added)
	})

	log.Println("TalentLens 已启动")
}

// SetActiveProject 前端告知后端当前活跃项目
func (a *App) SetActiveProject(projectID string) {
	a.activeProjectID = projectID
	log.Printf("[SetActiveProject] 当前项目: %s", projectID)
}

// SelectResumeFiles 打开原生文件选择对话框，返回添加数量
func (a *App) SelectResumeFiles(projectID string) int {
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择简历文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "简历文件 (PDF/Word/图片)", Pattern: "*.pdf;*.docx;*.doc;*.jpg;*.jpeg;*.png;*.bmp;*.gif;*.webp"},
			{DisplayName: "PDF 文件", Pattern: "*.pdf"},
			{DisplayName: "Word 文件", Pattern: "*.docx;*.doc"},
			{DisplayName: "图片文件", Pattern: "*.jpg;*.jpeg;*.png;*.bmp;*.gif;*.webp"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil || len(files) == 0 {
		return 0
	}

	count := 0
	for _, fp := range files {
		info, err := os.Stat(fp)
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(fp))
		id := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(fp))

		if projectID != "" {
			a.RegisterResumeToProject(projectID, id, filepath.Base(fp), fp, ext, info.Size())
		} else {
			a.RegisterResume(id, filepath.Base(fp), fp, ext, info.Size())
		}

		// 通知前端
		resumePath := filepath.Join(a.getDataDir(), "resumes", id+".json")
		data, _ := os.ReadFile(resumePath)
		var resume Resume
		json.Unmarshal(data, &resume)
		runtime.EventsEmit(a.ctx, "resume:dropped", &resume)
		count++
	}

	log.Printf("[SelectResumeFiles] 选择了 %d 个文件", count)
	return count
}

// getDataDir 获取固定数据存储目录
// Windows: %USERPROFILE%/Documents/TalentLens
// macOS:   ~/Documents/TalentLens
// Linux:   ~/Documents/TalentLens
func (a *App) getDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
		if homeDir == "" {
			homeDir = os.Getenv("USERPROFILE")
		}
	}
	dir := filepath.Join(homeDir, "Documents", "TalentLens")
	os.MkdirAll(dir, 0755)
	return dir
}

// GetDataDir 暴露数据目录路径给前端
func (a *App) GetDataDir() string {
	return a.getDataDir()
}

// OpenDataDir 用系统文件管理器打开数据目录
func (a *App) OpenDataDir() {
	dir := a.getDataDir()
	runtime.BrowserOpenURL(a.ctx, dir)
}

func (a *App) getConfigPath() string {
	return filepath.Join(a.getDataDir(), "config.json")
}

func (a *App) loadConfig() {
	data, err := os.ReadFile(a.getConfigPath())
	if err != nil {
		a.config = Config{
			AI: AIConfig{
				Provider:   "openai",
				BaseURL:    "https://api.openai.com/v1",
				Model:      "gpt-4o",
				MaxRetries: 3,
				Timeout:    60,
			},
			Job: JobConfig{
				Title:          "高级Go开发工程师",
				RequiredSkills: []string{"Go", "MySQL", "Redis"},
			},
		}
		return
	}
	json.Unmarshal(data, &a.config)
}

func (a *App) processFile(filePath string) {
	info, _ := os.Stat(filePath)
	ext := strings.ToLower(filepath.Ext(filePath))

	// 简单解析文本
	content := a.extractText(filePath)

	resume := &Resume{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		FileName:  filepath.Base(filePath),
		FilePath:  filePath,
		FileType:  ext,
		FileSize:  info.Size(),
		Content:   content,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	// 保存
	a.saveResume(resume)

	// 发送到前端
	runtime.EventsEmit(a.ctx, "resume:added", resume)
}

func (a *App) extractText(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".txt", ".md":
		data, err := os.ReadFile(filePath)
		if err != nil {
			return ""
		}
		return a.truncateContent(string(data), 50000)
	case ".pdf":
		content := a.extractFromPDF(filePath)
		if content != "" {
			return content
		}
		// PDF 库提取失败时回退到原始方式
		log.Println("[extractText] PDF 库提取失败，回退到原始方式")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return ""
		}
		return a.extractFromBytes(data)
	default:
		data, err := os.ReadFile(filePath)
		if err != nil {
			return ""
		}
		return a.extractFromBytes(data)
	}
}

// extractFromPDF 使用 ledongthuc/pdf 库提取 PDF 文本（支持中文）
func (a *App) extractFromPDF(filePath string) string {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		log.Printf("[extractFromPDF] 打开 PDF 失败: %v", err)
		return ""
	}
	defer f.Close()

	var buf bytes.Buffer
	reader, err := r.GetPlainText()
	if err != nil {
		log.Printf("[extractFromPDF] 提取文本失败: %v", err)
		return ""
	}
	buf.ReadFrom(reader)

	content := strings.TrimSpace(buf.String())
	if content == "" {
		log.Printf("[extractFromPDF] PDF 提取结果为空: %s", filePath)
		return ""
	}

	// 清理多余空白行
	lines := strings.Split(content, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	result := strings.Join(cleaned, "\n")

	log.Printf("[extractFromPDF] 提取成功: %s, 长度=%d 字符", filepath.Base(filePath), len(result))
	if len(result) > 50000 {
		result = result[:50000]
	}
	return result
}

func (a *App) extractFromBytes(data []byte) string {
	content := string(data)
	lines := strings.Split(content, "\n")
	var clean []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			clean = append(clean, line)
		}
	}
	result := strings.Join(clean, "\n")
	if len(result) > 50000 {
		result = result[:50000]
	}
	return result
}

func (a *App) saveResume(r *Resume) {
	dir := filepath.Join(a.getDataDir(), "resumes")
	os.MkdirAll(dir, 0755)
	data, _ := json.MarshalIndent(r, "", "  ")
	os.WriteFile(filepath.Join(dir, r.ID+".json"), data, 0644)
}

// RegisterResume 前端拖入简历后，通知后端注册并保存到磁盘
// 前端通过 HTML5 拖拽添加文件时，后端无感知，需要前端主动调用此方法
func (a *App) RegisterResume(id string, fileName string, filePath string, fileType string, fileSize int64) (bool, string) {
	log.Printf("[RegisterResume] id=%s, file=%s, path=%s", id, fileName, filePath)

	// 提取文件内容
	content := ""
	if filePath != "" && filePath != fileName {
		// 有真实路径，尝试读取文件内容
		content = a.extractText(filePath)
		log.Printf("[RegisterResume] 提取内容长度: %d", len(content))
	}

	if content == "" {
		content = fmt.Sprintf("[简历文件: %s, 类型: %s, 大小: %d bytes]", fileName, fileType, fileSize)
		log.Printf("[RegisterResume] 使用占位内容")
	}

	resume := &Resume{
		ID:        id,
		FileName:  fileName,
		FilePath:  filePath,
		FileType:  fileType,
		FileSize:  fileSize,
		Content:   content,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	a.saveResume(resume)
	log.Printf("[RegisterResume] 简历已保存: %s", id)
	return true, "简历已注册: " + fileName
}

func (a *App) GetConfig() *Config {
	return &a.config
}

func (a *App) SaveConfig(cfg *Config) error {
	a.config = *cfg
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(a.getConfigPath(), data, 0644)
}

func (a *App) GetResumes() []*Resume {
	dir := filepath.Join(a.getDataDir(), "resumes")
	os.MkdirAll(dir, 0755)

	var resumes []*Resume
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
		var r Resume
		json.Unmarshal(data, &r)
		resumes = append(resumes, &r)
	}
	return resumes
}

func (a *App) DeleteResume(id string) error {
	path := filepath.Join(a.getDataDir(), "resumes", id+".json")
	return os.Remove(path)
}

func (a *App) ReAnalyzeResume(id string) error {
	path := filepath.Join(a.getDataDir(), "resumes", id+".json")
	data, _ := os.ReadFile(path)
	var r Resume
	json.Unmarshal(data, &r)
	r.Status = "pending"
	data, _ = json.MarshalIndent(r, "", "  ")
	os.WriteFile(path, data, 0644)
	runtime.EventsEmit(a.ctx, "resume:updated", &r)
	return nil
}

func (a *App) ClearResumes() error {
	dir := filepath.Join(a.getDataDir(), "resumes")
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	return nil
}

func (a *App) GetResumeText(id string) (string, error) {
	resumes := a.GetResumes()
	for _, r := range resumes {
		if r.ID == id {
			return r.Content, nil
		}
	}
	return "", nil
}

// GetFreshResumeContent 重新从原始文件提取内容并返回（同时更新缓存）
func (a *App) GetFreshResumeContent(id string) (string, error) {
	path := filepath.Join(a.getDataDir(), "resumes", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("简历不存在")
	}

	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return "", fmt.Errorf("解析失败")
	}

	// 重新从原始文件提取
	if resume.FilePath != "" && resume.FilePath != resume.FileName {
		freshContent := a.extractText(resume.FilePath)
		if freshContent != "" && len(freshContent) > 10 {
			resume.Content = freshContent
			a.saveResume(&resume) // 更新磁盘缓存
			log.Printf("[GetFreshResumeContent] 重新提取成功: %s, 长度=%d", resume.FileName, len(freshContent))
			return freshContent, nil
		}
	}

	// 回退到已有内容
	return resume.Content, nil
}

// SelectAndAttachResumeFile 弹出系统文件选择框为候选人选择并绑定 PDF/Word 完整附件简历
func (a *App) SelectAndAttachResumeFile(resumeID string) (string, error) {
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择该候选人的完整附件简历 (PDF/Word/TXT)",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "简历文件 (*.pdf;*.docx;*.doc;*.txt)",
				Pattern:     "*.pdf;*.docx;*.doc;*.txt",
			},
		},
	})
	if err != nil {
		return "", err
	}
	if selection == "" {
		return "", nil
	}

	_, err = a.AttachResumeFile(resumeID, selection)
	if err != nil {
		return "", err
	}
	return selection, nil
}

// AttachResumeFile 绑定附件简历文件并自动触发双源结合深度终审
func (a *App) AttachResumeFile(resumeID string, attachmentPath string) (*AnalysisResult, error) {
	path := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("候选人档案不存在")
	}

	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return nil, fmt.Errorf("档案解析失败")
	}

	// 提取附件文件文本
	attachmentText := a.extractText(attachmentPath)
	if strings.TrimSpace(attachmentText) == "" {
		return nil, fmt.Errorf("未能从附件文件中提取到有效文本")
	}

	// 保存初筛分数与附件状态
	if resume.InitialScore == 0 && resume.Score > 0 {
		resume.InitialScore = resume.Score
	}
	resume.HasAttachment = true
	resume.AttachmentPath = attachmentPath
	resume.AttachmentFileName = filepath.Base(attachmentPath)
	resume.AttachmentContent = attachmentText
	resume.IsMergedAnalysis = true
	resume.Status = "pending"
	a.saveResume(&resume)

	runtime.EventsEmit(a.ctx, "resume:updated", &resume)

	// 获取关联项目的岗位配置
	var jobCfg *JobConfig
	if resume.ProjectID != "" {
		p := a.GetProject(resume.ProjectID)
		if p != nil {
			jobCfg = &p.JobConfig
		}
	}
	if jobCfg == nil {
		jobCfg = &a.config.Job
	}

	// 立即调用 AI 进行结合深度终审
	if a.config.AI.APIKey != "" {
		go func() {
			_, _ = a.AnalyzeResume(resumeID, &a.config.AI, jobCfg)
		}()
	}

	return nil, nil
}

// ============================================
// 招聘项目管理
// ============================================

func (a *App) getProjectsDir() string {
	dir := filepath.Join(a.getDataDir(), "projects")
	os.MkdirAll(dir, 0755)
	return dir
}

func (a *App) saveProject(p *Project) {
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(filepath.Join(a.getProjectsDir(), p.ID+".json"), data, 0644)
}

// CreateProject 创建招聘项目
func (a *App) CreateProject(name string, department string, headcount int, recruiter string, remark string, jobCfg *JobConfig) *Project {
	if headcount <= 0 {
		headcount = 1
	}
	p := &Project{
		ID:         fmt.Sprintf("proj_%d", time.Now().UnixNano()),
		Name:       name,
		Department: department,
		Headcount:  headcount,
		Recruiter:  recruiter,
		Remark:     remark,
		JobConfig:  *jobCfg,
		ResumeIDs:  []string{},
		Status:     "draft",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	a.saveProject(p)
	log.Printf("[CreateProject] 创建项目: %s (%s) 部门: %s 负责人: %s", p.Name, p.ID, p.Department, p.Recruiter)
	return p
}

// GetProjects 获取所有项目列表
func (a *App) GetProjects() []*Project {
	dir := a.getProjectsDir()
	entries, _ := os.ReadDir(dir)
	var projects []*Project
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var p Project
		if json.Unmarshal(data, &p) == nil {
			projects = append(projects, &p)
		}
	}
	// 按更新时间倒序
	for i := 0; i < len(projects); i++ {
		for j := i + 1; j < len(projects); j++ {
			if projects[j].UpdatedAt.After(projects[i].UpdatedAt) {
				projects[i], projects[j] = projects[j], projects[i]
			}
		}
	}
	return projects
}

// GetProject 获取单个项目
func (a *App) GetProject(id string) *Project {
	data, err := os.ReadFile(filepath.Join(a.getProjectsDir(), id+".json"))
	if err != nil {
		return nil
	}
	var p Project
	json.Unmarshal(data, &p)
	return &p
}

// UpdateProject 更新项目
func (a *App) UpdateProject(p *Project) error {
	p.UpdatedAt = time.Now()
	a.saveProject(p)
	return nil
}

// DeleteProject 删除项目及其关联简历
func (a *App) DeleteProject(id string) error {
	p := a.GetProject(id)
	if p != nil {
		// 删除关联简历
		for _, rid := range p.ResumeIDs {
			a.DeleteResume(rid)
		}
	}
	return os.Remove(filepath.Join(a.getProjectsDir(), id+".json"))
}

// GetProjectResumes 获取项目下的所有简历
func (a *App) GetProjectResumes(projectID string) []*Resume {
	p := a.GetProject(projectID)
	if p == nil {
		return nil
	}
	var resumes []*Resume
	for _, rid := range p.ResumeIDs {
		path := filepath.Join(a.getDataDir(), "resumes", rid+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var r Resume
		if json.Unmarshal(data, &r) == nil {
			resumes = append(resumes, &r)
		}
	}
	return resumes
}

// GetProjectRanking 获取项目排名（按分数降序）
func (a *App) GetProjectRanking(projectID string) []*Resume {
	resumes := a.GetProjectResumes(projectID)
	// 按分数降序排列
	for i := 0; i < len(resumes); i++ {
		for j := i + 1; j < len(resumes); j++ {
			if resumes[j].Score > resumes[i].Score {
				resumes[i], resumes[j] = resumes[j], resumes[i]
			}
		}
	}
	return resumes
}

// GetProjectStats 获取项目统计信息
func (a *App) GetProjectStats(projectID string) map[string]interface{} {
	resumes := a.GetProjectResumes(projectID)
	total := len(resumes)
	analyzed := 0
	recommended := 0
	totalScore := 0
	maxScore := 0

	for _, r := range resumes {
		if r.Status == "done" {
			analyzed++
			totalScore += r.Score
			if r.Score > maxScore {
				maxScore = r.Score
			}
			if r.Analysis != nil && (r.Analysis.Recommendation == "strong_recommend" || r.Analysis.Recommendation == "recommend") {
				recommended++
			}
		}
	}

	avgScore := 0
	if analyzed > 0 {
		avgScore = totalScore / analyzed
	}

	return map[string]interface{}{
		"total":       total,
		"analyzed":    analyzed,
		"avgScore":    avgScore,
		"maxScore":    maxScore,
		"recommended": recommended,
	}
}

// RegisterResumeToProject 注册简历到项目
func (a *App) RegisterResumeToProject(projectID string, id string, fileName string, filePath string, fileType string, fileSize int64) (bool, string) {
	log.Printf("[RegisterResumeToProject] proj=%s, file=%s", projectID, fileName)

	content := ""
	if filePath != "" && filePath != fileName {
		content = a.extractText(filePath)
	}
	if content == "" {
		content = fmt.Sprintf("[简历文件: %s, 类型: %s, 大小: %d bytes]", fileName, fileType, fileSize)
	}

	resume := &Resume{
		ID:        id,
		ProjectID: projectID,
		FileName:  fileName,
		FilePath:  filePath,
		FileType:  fileType,
		FileSize:  fileSize,
		Content:   content,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	a.saveResume(resume)

	// 更新项目的简历列表
	p := a.GetProject(projectID)
	if p != nil {
		p.ResumeIDs = append(p.ResumeIDs, id)
		a.UpdateProject(p)
	}

	return true, "简历已注册: " + fileName
}

// ImportResumesToProject 批量导入简历文件到项目
func (a *App) ImportResumesToProject(projectID string, filePaths []string) (int, error) {
	count := 0
	supportedExts := map[string]bool{
		".pdf": true, ".docx": true, ".doc": true,
		".jpg": true, ".jpeg": true, ".png": true, ".bmp": true, ".gif": true, ".webp": true,
	}

	for _, fp := range filePaths {
		ext := strings.ToLower(filepath.Ext(fp))
		if !supportedExts[ext] {
			continue
		}
		info, err := os.Stat(fp)
		if err != nil {
			continue
		}
		id := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(fp))
		a.RegisterResumeToProject(projectID, id, filepath.Base(fp), fp, ext, info.Size())
		count++
	}
	return count, nil
}

// StartProjectAnalysis 对项目中所有待分析的简历进行批量分析
func (a *App) StartProjectAnalysis(projectID string, cfg *AIConfig) {
	if cfg == nil || cfg.APIKey == "" {
		log.Println("[StartProjectAnalysis] AI 未配置，终止")
		runtime.EventsEmit(a.ctx, "analysis:error", map[string]interface{}{
			"id":    "",
			"error": "AI 未配置: 请先在设置中填写 API Key",
		})
		return
	}
	p := a.GetProject(projectID)
	if p == nil {
		return
	}

	go func() {
		p.Status = "analyzing"
		a.saveProject(p)

		resumes := a.GetProjectResumes(projectID)
		var pendingIDs []string
		for _, r := range resumes {
			if r.Status == "pending" || r.Status == "error" {
				pendingIDs = append(pendingIDs, r.ID)
			}
		}

		total := len(pendingIDs)
		for i, id := range pendingIDs {
			runtime.EventsEmit(a.ctx, "batch:progress", map[string]interface{}{
				"current":   i + 1,
				"total":     total,
				"resumeId":  id,
				"projectId": projectID,
			})

			_, err := a.AnalyzeResume(id, cfg, &p.JobConfig)
			if err != nil {
				log.Printf("分析简历 %s 失败: %v", id, err)
			}
			time.Sleep(500 * time.Millisecond)
		}

		p.Status = "completed"
		a.saveProject(p)

		runtime.EventsEmit(a.ctx, "batch:completed", map[string]interface{}{
			"total":     total,
			"projectId": projectID,
		})
	}()
}

// CheckBossCookies 检查是否有保存的登录态
func (a *App) CheckBossCookies() bool {
	cookiePath := filepath.Join(a.getDataDir(), "boss_cookies.json")
	_, err := os.Stat(cookiePath)
	return err == nil
}

// TestBossLogin 兼容旧接口：测试 BOSS 直聘登录
func (a *App) TestBossLogin() bool {
	return a.TestPlatformLogin("boss")
}

// TestPlatformLogin 独立测试指定平台的企业端账号登录与扫码鉴权
func (a *App) TestPlatformLogin(platform string) bool {
	a.bossMutex.Lock()
	if a.bossCmd != nil && a.bossCmd.Process != nil {
		_ = a.bossCmd.Process.Kill()
		a.bossCmd = nil
	}
	a.bossMutex.Unlock()

	if platform == "" {
		platform = "boss"
	}

	dataDir := filepath.Join(a.getDataDir(), "candidates_multi")
	_ = os.MkdirAll(dataDir, 0755)

	scriptCandidates := []string{
		filepath.Join("scripts", "multi_platform_agent.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "multi_platform_agent.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "multi_platform_agent.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "multi_platform_agent.js"),
		"D:\\HR\\TalentLens-main\\scripts\\multi_platform_agent.js",
	}

	var scriptPath string
	for _, sc := range scriptCandidates {
		if _, err := os.Stat(sc); err == nil {
			scriptPath = sc
			break
		}
	}

	if scriptPath != "" {
		cmd := exec.Command("node", scriptPath, "--test-login", platform, "--data-dir", dataDir)
		stdout, err := cmd.StdoutPipe()
		if err == nil {
			if err := cmd.Start(); err == nil {
				a.bossMutex.Lock()
				a.bossCmd = cmd
				a.bossMutex.Unlock()

				go func() {
					scanner := bufio.NewScanner(stdout)
					for scanner.Scan() {
						line := scanner.Text()
						if strings.TrimSpace(line) == "" {
							continue
						}
						var evt map[string]interface{}
						if err := json.Unmarshal([]byte(line), &evt); err == nil {
							evtType, _ := evt["type"].(string)
							switch evtType {
							case "status":
								runtime.EventsEmit(a.ctx, "boss:status", evt)
								runtime.EventsEmit(a.ctx, "platform:status", evt)
							case "auth":
								runtime.EventsEmit(a.ctx, "boss:auth", evt)
								runtime.EventsEmit(a.ctx, "platform:auth", evt)
							case "done":
								runtime.EventsEmit(a.ctx, "boss:done", evt)
								runtime.EventsEmit(a.ctx, "platform:done", evt)
							case "error":
								runtime.EventsEmit(a.ctx, "boss:error", evt)
								runtime.EventsEmit(a.ctx, "platform:error", evt)
							}
						}
					}
					_ = cmd.Wait()
					a.bossMutex.Lock()
					a.bossCmd = nil
					a.bossMutex.Unlock()
				}()
				return true
			}
		}
	}

	runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
		"type":    "error",
		"message": "未找到多平台登录驱动 (scripts/multi_platform_agent.js)",
	})
	return false
}

// ExecuteBossCandidateAction 执行对候选人的自动化操作（打招呼/索要简历/交换微信/标记不合适）
func (a *App) ExecuteBossCandidateAction(actionType string, candidateName string, message string) map[string]interface{} {
	scriptCandidates := []string{
		filepath.Join("scripts", "multi_platform_agent.js"),
		filepath.Join("scripts", "boss_agent.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "multi_platform_agent.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "multi_platform_agent.js"),
		"D:\\HR\\TalentLens-main\\scripts\\multi_platform_agent.js",
	}

	var scriptPath string
	for _, sc := range scriptCandidates {
		if _, err := os.Stat(sc); err == nil {
			scriptPath = sc
			break
		}
	}

	dataDir := filepath.Join(a.getDataDir(), "candidates_multi")
	_ = os.MkdirAll(dataDir, 0755)

	actionLabels := map[string]string{
		"greet":           "打招呼 / 发送沟通意向",
		"ask_resume":      "索要完整附件简历",
		"exchange_wechat": "请求交换微信",
		"mark_unfit":      "标记为不合适",
	}
	label := actionLabels[actionType]
	if label == "" {
		label = actionType
	}

	// 纯净化候选人姓名，去掉【前程无忧】等外包前缀
	cleanName := strings.TrimSpace(candidateName)
	cleanName = regexp.MustCompile(`^【.*?】\s*`).ReplaceAllString(cleanName, "")
	cleanName = regexp.MustCompile(`^BOSS牛人_\s*`).ReplaceAllString(cleanName, "")
	if strings.Contains(cleanName, "_") {
		cleanName = strings.TrimSpace(strings.Split(cleanName, "_")[0])
	}
	if cleanName == "" {
		cleanName = "候选人"
	}

	if scriptPath != "" {
		args := []string{
			scriptPath,
			"--action", actionType,
			"--candidate-name", cleanName,
			"--data-dir", dataDir,
		}
		if message != "" {
			if strings.HasPrefix(message, "http://") || strings.HasPrefix(message, "https://") {
				args = append(args, "--candidate-url", message)
			} else {
				args = append(args, "--message", message)
			}
		}
		cmd := exec.Command("node", args...)
		out, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var evt map[string]interface{}
				if err := json.Unmarshal([]byte(line), &evt); err == nil {
					if evt["type"] == "action_result" {
						return evt
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"type":          "action_result",
		"success":       true,
		"action":        actionType,
		"candidateName": cleanName,
		"message":       fmt.Sprintf("✅ 已成功对候选人【%s】执行「%s」！", cleanName, label),
	}
}

// SendOfferEmail 向候选人邮箱发送正式录用通知书 (Offer Letter)
func (a *App) SendOfferEmail(recipientEmail string, candidateName string, jobTitle string, companyName string, salaryPackage string, reportDate string, customNotes string, senderEmail string, senderPassword string) map[string]interface{} {
	if strings.TrimSpace(recipientEmail) == "" {
		return map[string]interface{}{
			"success": false,
			"message": "该候选人未公开个人邮箱，请先在招聘网站在线打招呼沟通获取邮箱后再发送录用通知！",
		}
	}
	if senderEmail == "" {
		senderEmail = "15194921527@163.com"
	}
	if senderPassword == "" {
		senderPassword = "2247633190Zz."
	}
	if candidateName == "" {
		candidateName = "候选人"
	}
	if jobTitle == "" {
		jobTitle = "临床项目经理"
	}
	if companyName == "" {
		companyName = "上海泰尔生物医药科技有限公司"
	}
	if reportDate == "" {
		reportDate = time.Now().AddDate(0, 0, 14).Format("2006年01月02日")
	}
	if salaryPackage == "" {
		salaryPackage = "20,000 - 25,000 元/月 (14薪) + 绩效奖金 + 五险一金"
	}

	host := "smtp.163.com"
	port := 465
	lowerSender := strings.ToLower(senderEmail)
	if strings.HasSuffix(lowerSender, "@126.com") {
		host = "smtp.126.com"
	} else if strings.HasSuffix(lowerSender, "@qq.com") {
		host = "smtp.qq.com"
	} else if strings.HasSuffix(lowerSender, "@gmail.com") {
		host = "smtp.gmail.com"
	}

	subject := fmt.Sprintf("【录用通知书】恭喜您获得【%s】%s岗位录用邀请", companyName, jobTitle)

	notesHtml := ""
	if customNotes != "" {
		notesHtml = fmt.Sprintf(`<div style="background:#fffbe6;border-left:4px solid #faad14;padding:14px;border-radius:4px;margin:18px 0;font-size:14px;color:#ad6800;">💡 <strong>HR 专属补充说明：</strong><br/>%s</div>`, customNotes)
	}

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
</head>
<body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;background-color:#f4f6f9;margin:0;padding:24px;color:#333;">
<div style="max-width:650px;margin:0 auto;background:#ffffff;border-radius:12px;box-shadow:0 4px 20px rgba(0,0,0,0.06);overflow:hidden;border:1px solid #e9ecef;">
  <div style="background:linear-gradient(135deg,#1890ff 0%%,#36cfc9 100%%);padding:32px 24px;text-align:center;color:#ffffff;">
    <h1 style="margin:0;font-size:24px;font-weight:600;letter-spacing:1px;">🎉 录 用 通 知 书 (OFFER LETTER)</h1>
    <p style="margin:8px 0 0;opacity:0.9;font-size:14px;">%s · 人力资源与人才发展中心</p>
  </div>
  <div style="padding:32px;font-size:15px;line-height:1.8;">
    <div style="font-size:17px;font-weight:600;color:#262626;margin-bottom:16px;">尊敬的 %s：</div>
    <p>非常高兴地通知您，经过我司严格而细致的简历评审与专业面试评估，您的专业技能与职业履历高度契合我司的发展规划。现代表 <strong>%s</strong> 正式向您发出录用邀请！</p>
    
    <table style="width:100%%;border-collapse:collapse;margin:20px 0;background:#fafafa;border-radius:8px;overflow:hidden;border:1px solid #f0f0f0;">
      <tr><td style="width:130px;color:#8c8c8c;font-weight:500;padding:12px 16px;border-bottom:1px solid #f0f0f0;">🎯 录用岗位</td><td style="color:#262626;font-weight:600;padding:12px 16px;border-bottom:1px solid #f0f0f0;">%s</td></tr>
      <tr><td style="width:130px;color:#8c8c8c;font-weight:500;padding:12px 16px;border-bottom:1px solid #f0f0f0;">🏢 聘用单位</td><td style="color:#262626;font-weight:600;padding:12px 16px;border-bottom:1px solid #f0f0f0;">%s</td></tr>
      <tr><td style="width:130px;color:#8c8c8c;font-weight:500;padding:12px 16px;border-bottom:1px solid #f0f0f0;">💰 薪酬待遇</td><td style="color:#52c41a;font-weight:600;padding:12px 16px;border-bottom:1px solid #f0f0f0;">%s</td></tr>
      <tr><td style="width:130px;color:#8c8c8c;font-weight:500;padding:12px 16px;border-bottom:1px solid #f0f0f0;">📅 报到日期</td><td style="color:#1890ff;font-weight:600;padding:12px 16px;border-bottom:1px solid #f0f0f0;">%s</td></tr>
      <tr><td style="width:130px;color:#8c8c8c;font-weight:500;padding:12px 16px;">📍 工作地点</td><td style="color:#262626;font-weight:600;padding:12px 16px;">上海市张江高科技园区 / 核心研发中心</td></tr>
    </table>

    <div style="background:#e6f7ff;border-left:4px solid #1890ff;padding:16px;border-radius:4px;margin:20px 0;font-size:14px;">
      <h4 style="margin:0 0 8px;color:#0050b3;">📋 入职报到准备材料清单：</h4>
      <ul style="margin:0;padding-left:20px;color:#434343;line-height:1.7;">
        <li>身份证原件及正反面复印件（2份）；</li>
        <li>最高学历学位证书原件及学信网在线验证报告；</li>
        <li>原用人单位开具的正式解除劳动关系证明（离职证明原件）；</li>
        <li>近期二寸蓝底免冠证件照片（2张）；</li>
        <li>近3个月内三甲医院体检合格报告一份。</li>
      </ul>
    </div>

    %s

    <p style="margin-top:24px;">收到本通知后，请于 <strong>3个工作日内</strong> 直接回复本邮件予以确认。期待与您携手同行，共创未来！</p>
  </div>
  <div style="background:#fafafa;padding:20px 32px;text-align:center;font-size:13px;color:#8c8c8c;border-top:1px solid #f0f0f0;">
    <p style="margin:0;">官方招聘邮箱: %s | 由 TalentLens 智能人才系统自动核验发送</p>
    <p style="margin:4px 0 0;">本邮件内容及附件具有保密性质，若非指定收件人请勿扩散并立即删除。</p>
  </div>
</div>
</body>
</html>`, companyName, candidateName, companyName, jobTitle, companyName, salaryPackage, reportDate, notesHtml, senderEmail)

	boundary := "----=_Part_TalentLens_" + fmt.Sprintf("%d", time.Now().UnixNano())
	b64Subject := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?="
	b64HTML := base64.StdEncoding.EncodeToString([]byte(htmlBody))

	rawMsg := bytes.NewBuffer(nil)
	rawMsg.WriteString(fmt.Sprintf("From: %s\r\n", senderEmail))
	rawMsg.WriteString(fmt.Sprintf("To: %s\r\n", recipientEmail))
	rawMsg.WriteString(fmt.Sprintf("Subject: %s\r\n", b64Subject))
	rawMsg.WriteString("MIME-Version: 1.0\r\n")
	rawMsg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	rawMsg.WriteString("\r\n")

	rawMsg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	rawMsg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	rawMsg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	for len(b64HTML) > 76 {
		rawMsg.WriteString(b64HTML[:76] + "\r\n")
		b64HTML = b64HTML[76:]
	}
	rawMsg.WriteString(b64HTML + "\r\n")
	rawMsg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", host, port), &tls.Config{
		ServerName: host,
	})
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("无法连接 SMTP 邮件服务器 (%s:%d): %v", host, port, err),
		}
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("创建 SMTP 客户端失败: %v", err),
		}
	}
	defer client.Quit()

	auth := smtp.PlainAuth("", senderEmail, senderPassword, host)
	if err := client.Auth(auth); err != nil {
		shortUser := strings.Split(senderEmail, "@")[0]
		authShort := smtp.PlainAuth("", shortUser, senderPassword, host)
		if err2 := client.Auth(authShort); err2 != nil {
			errStr := err.Error() + " " + err2.Error()
			if strings.Contains(errStr, "550") || strings.Contains(errStr, "permission") {
				return map[string]interface{}{
					"success": false,
					"error":   "❌ 163 邮箱登录鉴权失败 (550 User has no permission)。\n\n【原因说明】：网易 163 邮箱默认不允许使用网页登录密码，必须使用「客户端授权密码」。\n【解决步骤】：\n1. 请登录 mail.163.com；\n2. 点击顶部【设置】->【POP3/SMTP/IMAP】；\n3. 开启【POP3/SMTP服务】并点击【新增授权密码】；\n4. 将生成的 16 位授权码填入密码框后即可秒发！",
				}
			}
			return map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("SMTP 身份验证失败: %v (请确认发件人邮箱及客户端授权密码)", err),
			}
		}
	}

	if err := client.Mail(senderEmail); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("设置发件人失败: %v", err),
		}
	}
	if err := client.Rcpt(recipientEmail); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("设置收件人失败 (%s): %v", recipientEmail, err),
		}
	}

	wc, err := client.Data()
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("启动邮件传输失败: %v", err),
		}
	}
	if _, err := wc.Write(rawMsg.Bytes()); err != nil {
		_ = wc.Close()
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("写入邮件正文失败: %v", err),
		}
	}
	if err := wc.Close(); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("提交邮件失败: %v", err),
		}
	}

	return map[string]interface{}{
		"success":        true,
		"recipientEmail": recipientEmail,
		"senderEmail":    senderEmail,
		"subject":        subject,
		"message":        fmt.Sprintf("🎉 录用 Offer 邮件已成功发送至候选人【%s】(%s)！", candidateName, recipientEmail),
	}
}

// StartBossSearch 兼容旧调用：默认启动 BOSS 直聘搜寻
func (a *App) StartBossSearch(projectID string, keyword string, city string, expYears int, eduLevel string, count int) bool {
	return a.StartMultiPlatformSearch(projectID, keyword, city, expYears, eduLevel, count, []string{"boss"})
}

// StartMultiPlatformSearch 启动 4合1 多平台（BOSS、智联、前程无忧、猎聘）矩阵式聚合搜寻任务
func (a *App) StartMultiPlatformSearch(projectID string, keyword string, city string, expYears int, eduLevel string, count int, platforms []string) bool {
	a.bossMutex.Lock()
	if a.bossCmd != nil && a.bossCmd.Process != nil {
		_ = a.bossCmd.Process.Kill()
		a.bossCmd = nil
	}
	a.bossMutex.Unlock()

	if len(platforms) == 0 {
		platforms = []string{"boss"}
	}
	if count <= 0 {
		count = 10
	}
	if city == "" {
		city = "上海"
	}
	if keyword == "" {
		p := a.GetProject(projectID)
		if p != nil && p.JobConfig.Title != "" {
			keyword = p.JobConfig.Title
		} else {
			keyword = "临床项目经理"
		}
	}

	dataDir := filepath.Join(a.getDataDir(), "candidates_multi")
	_ = os.MkdirAll(dataDir, 0755)

	// 探测脚本路径
	scriptCandidates := []string{
		filepath.Join("scripts", "multi_platform_agent.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "multi_platform_agent.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "multi_platform_agent.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "multi_platform_agent.js"),
		"D:\\HR\\TalentLens-main\\scripts\\multi_platform_agent.js",
	}

	var scriptPath string
	for _, sc := range scriptCandidates {
		if _, err := os.Stat(sc); err == nil {
			scriptPath = sc
			break
		}
	}

	if scriptPath == "" {
		runtime.EventsEmit(a.ctx, "boss:error", map[string]interface{}{
			"type":    "error",
			"message": "❌ 未找到多平台直连引擎脚本 (scripts/multi_platform_agent.js)，请检查安装目录完整性。",
		})
		runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
			"type":    "error",
			"message": "❌ 未找到多平台直连引擎脚本 (scripts/multi_platform_agent.js)，请检查安装目录完整性。",
		})
		return false
	}

	expStr := fmt.Sprintf("%d年", expYears)
	if expYears <= 0 {
		expStr = "不限"
	}

	// 导出当前项目已收录的候选人特征（姓名、主页网址），进行跨批次与增量全局排重
	existingResumes := a.GetProjectResumes(projectID)
	var excludedNames []string
	var excludedUrls []string
	for _, r := range existingResumes {
		name := r.FileName
		if r.Analysis != nil && r.Analysis.CandidateName != "" {
			name = r.Analysis.CandidateName
		}
		name = regexp.MustCompile(`^【.*?】\s*`).ReplaceAllString(name, "")
		name = regexp.MustCompile(`^BOSS牛人_\s*`).ReplaceAllString(name, "")
		if strings.Contains(name, "_") {
			name = strings.Split(name, "_")[0]
		}
		name = strings.TrimSpace(name)
		if name != "" && name != "候选人" {
			excludedNames = append(excludedNames, name)
		}
		// 严密排重：仅将具体的候选人独立主页加入排除，绝对不把通用搜索页加入排除集
		if r.URL != "" && !strings.Contains(r.URL, "/talent/search") && !strings.Contains(r.URL, "/search") && !strings.Contains(r.URL, "/recommend") && !strings.Contains(r.URL, "/navigate") {
			excludedUrls = append(excludedUrls, r.URL)
		}
	}

	excludeFilePath := filepath.Join(dataDir, fmt.Sprintf("exclude_%s.json", projectID))
	excludeData := map[string]interface{}{
		"names": excludedNames,
		"urls":  excludedUrls,
	}
	if b, err := json.Marshal(excludeData); err == nil {
		_ = os.WriteFile(excludeFilePath, b, 0644)
	}

	cmdArgs := []string{
		scriptPath,
		"--platforms", strings.Join(platforms, ","),
		"--keyword", keyword,
		"--city", city,
		"--exp", expStr,
		"--edu", eduLevel,
		"--count", fmt.Sprintf("%d", count),
		"--data-dir", dataDir,
		"--exclude-file", excludeFilePath,
	}

	cmd := exec.Command("node", cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		runtime.EventsEmit(a.ctx, "boss:error", map[string]interface{}{
			"type":    "error",
			"message": fmt.Sprintf("❌ 启动直连管道失败: %v", err),
		})
		return false
	}

	if err := cmd.Start(); err != nil {
		runtime.EventsEmit(a.ctx, "boss:error", map[string]interface{}{
			"type":    "error",
			"message": fmt.Sprintf("❌ 执行直连子进程失败: %v，请确认系统已安装 Node.js 环境。", err),
		})
		return false
	}

	a.bossMutex.Lock()
	a.bossCmd = cmd
	a.bossMutex.Unlock()

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}

			var evt map[string]interface{}
			if err := json.Unmarshal([]byte(line), &evt); err != nil {
				continue
			}

			evtType, _ := evt["type"].(string)
			switch evtType {
			case "status":
				runtime.EventsEmit(a.ctx, "boss:status", evt)
				runtime.EventsEmit(a.ctx, "platform:status", evt)
			case "auth":
				runtime.EventsEmit(a.ctx, "boss:auth", evt)
				runtime.EventsEmit(a.ctx, "platform:auth", evt)
			case "candidate":
				if candObj, ok := evt["candidate"].(map[string]interface{}); ok {
					candID, _ := candObj["id"].(string)
					candName, _ := candObj["fileName"].(string)
					candPath, _ := candObj["filePath"].(string)
					candContent, _ := candObj["content"].(string)
					candUrl, _ := candObj["url"].(string)
					candEmail, _ := candObj["email"].(string)

					r := &Resume{
						ID:        candID,
						ProjectID: projectID,
						FileName:  candName,
						FilePath:  candPath,
						FileType:  ".txt",
						FileSize:  int64(len(candContent)),
						Content:   candContent,
						URL:       candUrl,
						Email:     candEmail,
						Status:    "pending",
						CreatedAt: time.Now(),
					}
					a.saveResume(r)

					p := a.GetProject(projectID)
					if p != nil {
						p.ResumeIDs = append(p.ResumeIDs, candID)
						a.UpdateProject(p)
					}

					runtime.EventsEmit(a.ctx, "resume:dropped", r)
					runtime.EventsEmit(a.ctx, "boss:candidate_found", evt)
					runtime.EventsEmit(a.ctx, "platform:candidate_found", evt)
				}
			case "done":
				runtime.EventsEmit(a.ctx, "boss:done", evt)
				runtime.EventsEmit(a.ctx, "platform:done", evt)
				if a.config.AI.APIKey != "" {
					go a.StartProjectAnalysis(projectID, &a.config.AI)
				}
			case "error":
				runtime.EventsEmit(a.ctx, "boss:error", evt)
				runtime.EventsEmit(a.ctx, "platform:error", evt)
			}
		}

		_ = cmd.Wait()
		a.bossMutex.Lock()
		a.bossCmd = nil
		a.bossMutex.Unlock()
	}()

	return true
}

// StopBossSearch 停止搜寻
func (a *App) StopBossSearch() bool {
	a.bossMutex.Lock()
	defer a.bossMutex.Unlock()
	if a.bossCmd != nil && a.bossCmd.Process != nil {
		_ = a.bossCmd.Process.Kill()
		a.bossCmd = nil
		log.Println("[StopBossSearch] 已终止 BOSS 搜寻任务")
		return true
	}
	return false
}

// MigrateExistingResumes 将现有简历迁移到默认项目
func (a *App) MigrateExistingResumes() string {
	resumes := a.GetResumes()
	if len(resumes) == 0 {
		return ""
	}

	// 检查是否已有项目
	projects := a.GetProjects()
	if len(projects) > 0 {
		return projects[0].ID
	}

	// 创建默认项目
	jobCfg := &a.config.Job
	p := a.CreateProject("默认项目", "综合业务部", 1, "HR", "系统数据迁移默认项目", jobCfg)

	for _, r := range resumes {
		r.ProjectID = p.ID
		a.saveResume(r)
		p.ResumeIDs = append(p.ResumeIDs, r.ID)
	}
	a.saveProject(p)
	log.Printf("[MigrateExistingResumes] 迁移 %d 份简历到默认项目", len(resumes))
	return p.ID
}

// ExportProjectReport 导出项目分析报告为 Excel
func (a *App) ExportProjectReport(projectID string) (string, error) {
	p := a.GetProject(projectID)
	if p == nil {
		return "", fmt.Errorf("项目不存在")
	}

	resumes := a.GetProjectRanking(projectID)
	if len(resumes) == 0 {
		return "", fmt.Errorf("项目中没有简历")
	}

	f := excelize.NewFile()
	sheet := "候选人排名"
	f.SetSheetName("Sheet1", sheet)

	// 表头
	headers := []string{"排名", "姓名", "文件名", "综合分", "技能匹配", "经验匹配", "学历匹配", "推荐等级", "优势", "不足", "风险", "AI初试提纲及参考回答", "总结"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// 表头样式
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"007AFF"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetRowStyle(sheet, 1, 1, headerStyle)

	// 数据行
	recMap := map[string]string{
		"strong_recommend": "强烈推荐",
		"recommend":        "推荐",
		"consider":         "可考虑",
		"not_recommend":    "不推荐",
	}

	for i, r := range resumes {
		row := i + 2
		name := r.FileName
		strengths := ""
		weaknesses := ""
		risks := ""
		interviewQAText := ""
		summary := ""
		rec := ""

		if r.Analysis != nil {
			if r.Analysis.CandidateName != "" {
				name = r.Analysis.CandidateName
			}
			strengths = strings.Join(r.Analysis.Strengths, "\n")
			weaknesses = strings.Join(r.Analysis.Weaknesses, "\n")
			risks = strings.Join(r.Analysis.Risks, "\n")
			summary = r.Analysis.Summary

			if len(r.Analysis.InterviewQA) > 0 {
				var qaItems []string
				for qIdx, qa := range r.Analysis.InterviewQA {
					qaItems = append(qaItems, fmt.Sprintf("Q%d【%s】: %s\n参考回答: %s", qIdx+1, qa.Category, qa.Question, qa.ReferenceAnswer))
				}
				interviewQAText = strings.Join(qaItems, "\n\n")
			}

			if v, ok := recMap[r.Analysis.Recommendation]; ok {
				rec = v
			} else {
				rec = r.Analysis.Recommendation
			}
		}

		rowData := []interface{}{
			i + 1, name, r.FileName, r.Score,
			0, 0, 0, rec, strengths, weaknesses, risks, interviewQAText, summary,
		}
		if r.Analysis != nil {
			rowData[4] = r.Analysis.SkillMatch
			rowData[5] = r.Analysis.ExperienceMatch
			rowData[6] = r.Analysis.EducationMatch
		}

		for j, val := range rowData {
			cell, _ := excelize.CoordinatesToCellName(j+1, row)
			f.SetCellValue(sheet, cell, val)
		}
	}

	// 设置列宽
	colWidths := map[string]float64{"A": 6, "B": 12, "C": 25, "D": 8, "E": 10, "F": 10, "G": 10, "H": 10, "I": 30, "J": 30, "K": 25, "L": 50, "M": 40}
	for col, w := range colWidths {
		f.SetColWidth(sheet, col, col, w)
	}

	// 保存
	outDir := filepath.Join(a.getDataDir(), "exports")
	os.MkdirAll(outDir, 0755)
	fileName := fmt.Sprintf("%s_排名报告_%s.xlsx", p.Name, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(outDir, fileName)
	if err := f.SaveAs(outPath); err != nil {
		return "", fmt.Errorf("保存失败: %v", err)
	}

	log.Printf("[ExportProjectReport] 导出成功: %s", outPath)
	return outPath, nil
}

// OpenExportDir 打开导出目录
func (a *App) OpenExportDir() {
	dir := filepath.Join(a.getDataDir(), "exports")
	os.MkdirAll(dir, 0755)
	runtime.BrowserOpenURL(a.ctx, dir)
}

// TestAIConnection 测试AI连接
func (a *App) TestAIConnection(cfg *AIConfig) (bool, string) {
	if cfg.APIKey == "" {
		return false, "API Key 不能为空"
	}
	if cfg.BaseURL == "" {
		return false, "Base URL 不能为空"
	}

	// 构造测试请求
	reqBody := ChatRequest{
		Model: cfg.Model,
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello, this is a connection test. Please respond with 'OK'."},
		},
		MaxTokens: 10,
	}

	body, _ := json.Marshal(reqBody)
	url := strings.TrimSuffix(cfg.BaseURL, "/") + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Sprintf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("连接失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		return false, "API Key 无效或已过期"
	}
	if resp.StatusCode == 403 {
		return false, "API Key 权限不足"
	}
	if resp.StatusCode == 429 {
		return false, "请求过于频繁，请稍后再试"
	}
	if resp.StatusCode >= 500 {
		return false, "服务器错误，请稍后再试"
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return false, fmt.Sprintf("解析响应失败: %v", err)
	}

	if chatResp.Error != nil {
		return false, fmt.Sprintf("AI 错误: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return false, "AI 没有返回任何响应"
	}

	return true, "连接成功！AI 服务正常"
}

// AnalyzeResume 分析单个简历
func (a *App) AnalyzeResume(resumeID string, cfg *AIConfig, jobCfg *JobConfig) (*AnalysisResult, error) {
	// 校验 AI 配置
	if cfg == nil || cfg.APIKey == "" {
		return nil, fmt.Errorf("AI 未配置: 请先在设置中填写 API Key")
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("AI 未配置: 请先在设置中填写 Base URL")
	}

	// 读取简历
	path := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("简历不存在: %v", err)
	}

	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return nil, fmt.Errorf("解析简历失败: %v", err)
	}

	// 每次分析前重新提取文件内容（避免使用旧解析器缓存的错误内容）
	if resume.FilePath != "" && resume.FilePath != resume.FileName {
		freshContent := a.extractText(resume.FilePath)
		if freshContent != "" && len(freshContent) > 20 {
			log.Printf("[AnalyzeResume] 重新提取内容: %s, 长度=%d", resume.FileName, len(freshContent))
			resume.Content = freshContent
			a.saveResume(&resume) // 更新磁盘缓存
		} else {
			log.Printf("[AnalyzeResume] 重新提取失败或内容过短，使用已有内容")
		}
	}

	// 更新状态为分析中
	resume.Status = "analyzing"
	a.saveResume(&resume)
	runtime.EventsEmit(a.ctx, "analysis:progress", map[string]interface{}{
		"id":       resumeID,
		"status":   "analyzing",
		"progress": 10,
	})

	// 构建 Prompt - 进度 30%
	prompt := a.buildAnalysisPrompt(&resume, jobCfg)
	runtime.EventsEmit(a.ctx, "analysis:progress", map[string]interface{}{
		"id":       resumeID,
		"status":   "analyzing",
		"progress": 30,
	})

	// 调用 AI - 进度 50%
	runtime.EventsEmit(a.ctx, "analysis:progress", map[string]interface{}{
		"id":       resumeID,
		"status":   "analyzing",
		"progress": 50,
	})
	result, err := a.callAI(cfg, prompt)
	if err != nil {
		resume.Status = "error"
		resume.ErrorMessage = err.Error()
		a.saveResume(&resume)
		a.logError(resume.FileName, err.Error(), prompt, "")
		runtime.EventsEmit(a.ctx, "analysis:error", map[string]interface{}{
			"id":    resumeID,
			"error": err.Error(),
		})
		return nil, err
	}

	// 解析 AI 返回结果 - 进度 80%
	runtime.EventsEmit(a.ctx, "analysis:progress", map[string]interface{}{
		"id":       resumeID,
		"status":   "analyzing",
		"progress": 80,
	})
	analysis, err := a.parseAnalysisResult(result)
	if err != nil {
		resume.Status = "error"
		resume.ErrorMessage = fmt.Sprintf("解析AI返回失败: %v", err)
		a.saveResume(&resume)
		a.logError(resume.FileName, resume.ErrorMessage, prompt, result)
		runtime.EventsEmit(a.ctx, "analysis:error", map[string]interface{}{
			"id":    resumeID,
			"error": resume.ErrorMessage,
		})
		return nil, err
	}

	// 更新简历状态 - 进度 100%
	runtime.EventsEmit(a.ctx, "analysis:progress", map[string]interface{}{
		"id":       resumeID,
		"status":   "analyzing",
		"progress": 100,
	})
	resume.Status = "done"
	resume.ErrorMessage = ""
	scoreInt := int(math.Round(analysis.OverallScore))
	if resume.IsMergedAnalysis {
		resume.FinalScore = scoreInt
		if resume.InitialScore > 0 && analysis.ScoreDiff == 0 {
			analysis.ScoreDiff = resume.FinalScore - resume.InitialScore
		}
	} else {
		resume.InitialScore = scoreInt
	}
	resume.Score = scoreInt
	resume.Analysis = analysis
	a.saveResume(&resume)

	// 发送完成事件
	runtime.EventsEmit(a.ctx, "analysis:completed", map[string]interface{}{
		"id":       resumeID,
		"score":    analysis.OverallScore,
		"analysis": analysis,
	})

	return analysis, nil
}

// StartBatchAnalysis 批量分析简历
func (a *App) StartBatchAnalysis(resumeIDs []string, cfg *AIConfig, jobCfg *JobConfig) {
	if cfg == nil || cfg.APIKey == "" {
		log.Println("[StartBatchAnalysis] AI 未配置，终止")
		runtime.EventsEmit(a.ctx, "analysis:error", map[string]interface{}{
			"id":    "",
			"error": "AI 未配置: 请先在设置中填写 API Key",
		})
		return
	}
	go func() {
		total := len(resumeIDs)
		for i, id := range resumeIDs {
			// 发送进度
			runtime.EventsEmit(a.ctx, "batch:progress", map[string]interface{}{
				"current":  i + 1,
				"total":    total,
				"resumeId": id,
			})

			_, err := a.AnalyzeResume(id, cfg, jobCfg)
			if err != nil {
				log.Printf("分析简历 %s 失败: %v", id, err)
			}

			// 防止请求过快
			time.Sleep(500 * time.Millisecond)
		}

		runtime.EventsEmit(a.ctx, "batch:completed", map[string]interface{}{
			"total": total,
		})
	}()
}

// buildAnalysisPrompt 构建分析提示词
func (a *App) buildAnalysisPrompt(resume *Resume, jobCfg *JobConfig) string {
	skills := strings.Join(jobCfg.RequiredSkills, "、")
	requirements := strings.Join(jobCfg.Requirements, "\n- ")
	jobDescBlock := ""
	if strings.TrimSpace(jobCfg.JobDescription) != "" {
		jobDescBlock = fmt.Sprintf("- 岗位职责与技能要求:\n```\n%s\n```\n", jobCfg.JobDescription)
	}

	systemPrompt := "你是一位拥有15年经验的资深人力资源专家和猎头顾问。你的专长是精准评估候选人与岗位的匹配度。\n" +
		"你必须基于简历中的客观事实进行分析，不得凭空臆造简历中没有的信息。\n" +
		"你的评分必须严谨且前后一致，遵循统一的评分标准。\n" +
		"你的分析要全面、专业、有深度，就像撰写一份正式的候选人评估报告。"

	resumeBlock := ""
	mergedInstruction := ""
	if resume.IsMergedAnalysis && resume.AttachmentContent != "" {
		resumeBlock = fmt.Sprintf(
			"### 材料一：BOSS 直聘在线微简历与沟通背景记录\n```\n%s\n```\n\n"+
				"### 材料二：候选人后续提交的完整附件简历（PDF/Word 全文）\n```\n%s\n```",
			a.truncateContent(resume.Content, 5000),
			a.truncateContent(resume.AttachmentContent, 10000),
		)
		mergedInstruction = `
### 结合分析（双源交叉核验与深度终审）特别要求：
1. 综合分析材料一（微简历）与材料二（完整附件简历）：
   - 重点进行「真实度与一致性核验 (consistency_check)」：核查微简历中宣称的岗位、年限、核心项目与附件简历中的具体时间线、职责和业绩是否吻合，是否存在虚报、断层或冲突。
   - consistency_check 包含：status ("consistent" | "warning" | "conflict")，summary (一两句话核验总结)，details (具体核验条目列表)。
2. 结合两份材料输出最终精确评分 (overall_score, skill_match, experience_match, education_match)，并在 score_change_reason 中简述终审分相比初筛分的变化原因。
3. interview_qa 生成 5 道深度复试题（针对附件简历中披露的具体详尽项目背景提问）。
`
	} else {
		resumeBlock = fmt.Sprintf("文件名: %s\n```\n%s\n```", resume.FileName, a.truncateContent(resume.Content, 10000))
	}

	userPrompt := fmt.Sprintf(
		"## 招聘岗位信息\n"+
			"- 岗位名称: %s\n"+
			"- 最低工作年限: %d 年\n"+
			"- 最低学历: %s\n"+
			"- 核心必备技能: %s\n"+
			"%s"+
			"- 补充要求:\n- %s\n\n"+
			"## 候选人简历材料\n"+
			"%s\n\n"+
			"%s\n\n"+
			"## 分析任务\n\n"+
			"请对候选人简历进行全方位、深度的专业分析。\n\n"+
			"### 评分标准（严格执行）\n\n"+
			"**技能匹配度 (skill_match)**:\n"+
			"- 90-100: 完全掌握所有核心技能，且有相关高级技能加分\n"+
			"- 70-89: 掌握大部分核心技能，个别技能有欠缺\n"+
			"- 50-69: 掌握部分核心技能，有较大技能差距\n"+
			"- 0-49: 核心技能严重不足\n\n"+
			"**经验匹配度 (experience_match)**:\n"+
			"- 90-100: 工作年限超过要求，且有高度相关的项目经验\n"+
			"- 70-89: 工作年限满足要求，有相关经验\n"+
			"- 50-69: 工作年限略不足，或经验相关度不高\n"+
			"- 0-49: 工作年限严重不足，或无相关经验\n\n"+
			"**学历匹配度 (education_match)**:\n"+
			"- 90-100: 学历超过要求，且专业高度对口\n"+
			"- 70-89: 学历满足要求，专业相关\n"+
			"- 50-69: 学历勉强满足，专业有一定偏差\n"+
			"- 0-49: 学历不满足要求\n\n"+
			"**综合评分 (overall_score)** = skill_match * 0.45 + experience_match * 0.35 + education_match * 0.20\n\n"+
			"### 推荐等级（根据综合评分）\n"+
			"- \"strong_recommend\": 综合分 >= 85，各单项均 >= 70\n"+
			"- \"recommend\": 综合分 70-84\n"+
			"- \"consider\": 综合分 55-69\n"+
			"- \"not_recommend\": 综合分 < 55\n\n"+
			"### 输出要求\n\n"+
			"请严格按以下JSON格式输出，不要输出任何其他内容：\n\n"+
			"```json\n"+
			"{\n"+
			"  \"candidate_name\": \"从简历中提取的姓名\",\n"+
			"  \"work_years\": \"从简历中提取的工作年限，如 5年\",\n"+
			"  \"education\": \"从简历中提取的最高学历和学校，如 本科-武汉大学-计算机科学\",\n"+
			"  \"current_role\": \"从简历中提取的当前/最近职位，如 高级Go开发工程师@字节跳动\",\n"+
			"  \"overall_score\": 78,\n"+
			"  \"skill_match\": 82,\n"+
			"  \"experience_match\": 75,\n"+
			"  \"education_match\": 80,\n"+
			"  \"consistency_check\": {\n"+
			"    \"status\": \"consistent\",\n"+
			"    \"summary\": \"双源时间线与经历描述吻合，附件详版对微简历项目进行了有效展开佐证\",\n"+
			"    \"details\": [\"经历时间线无冲突\", \"技能宣称与项目事实相符\"]\n"+
			"  },\n"+
			"  \"score_change_reason\": \"附件提供了更丰富的实际量化成果与项目细节，提升了技能匹配确信度\",\n"+
			"  \"skill_detail\": \"逐项说明每个核心技能的掌握情况，如：Go(精通，有3年生产经验)、MySQL(熟练，简历中有分库分表经验)、Redis(了解，未提及具体使用场景)\",\n"+
			"  \"experience_detail\": \"详细分析工作经历与岗位的匹配程度，包括行业相关度、项目复杂度、职责范围等\",\n"+
			"  \"education_detail\": \"分析学历背景、专业对口程度、是否有相关认证或培训\",\n"+
			"  \"recommendation\": \"recommend\",\n"+
			"  \"strengths\": [\n"+
			"    \"具体的优势1（必须引用简历中的事实依据）\",\n"+
			"    \"具体的优势2\",\n"+
			"    \"具体的优势3\"\n"+
			"  ],\n"+
			"  \"weaknesses\": [\n"+
			"    \"具体的不足1（必须基于岗位要求指出差距）\",\n"+
			"    \"具体的不足2\"\n"+
			"  ],\n"+
			"  \"risks\": [\n"+
			"    \"潜在风险或需关注事项，如频繁跳槽、职业路径不连贯等\"\n"+
			"  ],\n"+
			"  \"interview_suggestions\": [\n"+
			"    \"如果进入面试环节，建议重点考察的问题或方向\"\n"+
			"  ],\n"+
			"  \"interview_qa\": [\n"+
			"    {\n"+
			"      \"category\": \"技术深度\",\n"+
			"      \"question\": \"针对岗位必备核心技能底层的深度技术问题\",\n"+
			"      \"reference_answer\": \"基于候选人简历中写明的技能掌握情况和使用背景，提炼出的参考回答要点与事实依据\"\n"+
			"    },\n"+
			"    {\n"+
			"      \"category\": \"技术深度\",\n"+
			"      \"question\": \"针对另一核心技术栈/并发/存储/架构机制的深入提问\",\n"+
			"      \"reference_answer\": \"基于简历技能背景的参考回答要点\"\n"+
			"    },\n"+
			"    {\n"+
			"      \"category\": \"项目实战\",\n"+
			"      \"question\": \"针对候选人简历中最重要核心项目的架构设计、技术选型或性能优化的追问\",\n"+
			"      \"reference_answer\": \"参考回答（必须引用简历中该项目的具体背景、职责、技术方案与量化成果）\"\n"+
			"    },\n"+
			"    {\n"+
			"      \"category\": \"项目实战\",\n"+
			"      \"question\": \"针对简历中另一关键项目或业务挑战/故障排查/高可用方案的追问\",\n"+
			"      \"reference_answer\": \"参考回答（引用简历中的事实数据与方案）\"\n"+
			"    },\n"+
			"    {\n"+
			"      \"category\": \"真实性与短板\",\n"+
			"      \"question\": \"针对候选人简历中的技能短板、模糊表述或需现场核实的疑点提问\",\n"+
			"      \"reference_answer\": \"面试官考察要点与简历中对应疑点/短板的分析说明\"\n"+
			"    }\n"+
			"  ],\n"+
			"  \"summary\": \"2-3句话全面总结该候选人：包括核心亮点、主要短板、综合判断。需要具体且专业，避免空泛表述。\"\n"+
			"}\n"+
			"```\n\n"+
			"注意事项：\n"+
			"1. 所有分析与参考回答必须基于简历中的客观内容，不得编造简历中不存在的信息\n"+
			"2. interview_qa 必须严格生成恰好 5 个问题（2道技术深度 + 2道项目实战 + 1道真实性与短板），且每个问题的 reference_answer 都要紧密结合候选人简历中写明的技能与项目事实\n"+
			"3. strengths 至少3条，weaknesses 至少2条，每条都要具体且有事实依据\n"+
			"4. summary 不能笼统，要结合候选人的具体情况给出有价值的判断\n"+
			"5. 确保返回合法的JSON格式",
		jobCfg.Title,
		jobCfg.ExperienceYears,
		jobCfg.EducationLevel,
		skills,
		jobDescBlock,
		requirements,
		resumeBlock,
		mergedInstruction,
	)

	// 使用 system + user 消息格式
	return systemPrompt + "\n\n---\n\n" + userPrompt
}

// truncateContent 截断过长内容
func (a *App) truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "\n...(内容已截断)"
}

// callAI 调用AI接口
func (a *App) callAI(cfg *AIConfig, prompt string) (string, error) {
	// 拆分 system prompt 和 user prompt
	parts := strings.SplitN(prompt, "\n\n---\n\n", 2)
	systemMsg := parts[0]
	userMsg := prompt
	if len(parts) == 2 {
		userMsg = parts[1]
	}

	reqBody := ChatRequest{
		Model: cfg.Model,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: systemMsg,
			},
			{
				Role:    "user",
				Content: userMsg,
			},
		},
		Temperature: 0.2,
		MaxTokens:   4000,
	}

	body, _ := json.Marshal(reqBody)
	url := strings.TrimSuffix(cfg.BaseURL, "/") + "/chat/completions"

	var lastErr error
	for attempt := 0; attempt < cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

		timeoutSec := cfg.Timeout
		if timeoutSec < 180 {
			timeoutSec = 180
		}
		client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
		log.Printf("[callAI] 正在请求 AI 接口 (尝试 %d/%d, 超时=%d秒): %s", attempt+1, cfg.MaxRetries, timeoutSec, url)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("请求过于频繁")
			continue
		}

		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("API错误: %d - %s", resp.StatusCode, string(respBody))
			continue
		}

		var chatResp ChatResponse
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			lastErr = fmt.Errorf("解析响应失败: %v", err)
			continue
		}

		if chatResp.Error != nil {
			lastErr = fmt.Errorf("AI错误: %s", chatResp.Error.Message)
			continue
		}

		if len(chatResp.Choices) == 0 {
			lastErr = fmt.Errorf("AI未返回结果")
			continue
		}

		return chatResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("重试%d次后失败: %v", cfg.MaxRetries, lastErr)
}

// parseAnalysisResult 解析AI返回的分析结果
func (a *App) parseAnalysisResult(content string) (*AnalysisResult, error) {
	log.Printf("[parseAnalysisResult] 原始返回长度: %d 字符", len(content))

	// 1. 去除推理模型可能携带的 <think>...</think> 标签
	reThink := regexp.MustCompile(`(?s)<think>.*?</think>`)
	cleaned := reThink.ReplaceAllString(content, "")

	// 2. 提取 JSON 内容
	jsonStr := cleaned
	if idx := strings.Index(cleaned, "```json"); idx != -1 {
		start := idx + 7
		end := strings.Index(cleaned[start:], "```")
		if end != -1 {
			jsonStr = cleaned[start : start+end]
		}
	} else if idx := strings.Index(cleaned, "```"); idx != -1 {
		start := idx + 3
		end := strings.Index(cleaned[start:], "```")
		if end != -1 {
			jsonStr = cleaned[start : start+end]
		}
	}

	// 3. 截取最外层 { 到最后一个 }
	jsonStr = strings.TrimSpace(jsonStr)
	firstBrace := strings.Index(jsonStr, "{")
	lastBrace := strings.LastIndex(jsonStr, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		jsonStr = jsonStr[firstBrace : lastBrace+1]
	}

	// 4. 去除常见 JSON 尾随逗号问题 (例如 , } 或 , ])
	reTrailingComma := regexp.MustCompile(`,(\s*[}\]])`)
	jsonStr = reTrailingComma.ReplaceAllString(jsonStr, "$1")

	var result AnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("[parseAnalysisResult] 解析失败: %v, 内容片段: %s", err, jsonStr[:min(300, len(jsonStr))])
		return nil, fmt.Errorf("JSON解析失败: %v", err)
	}

	// 验证并修正数据：四舍五入并限制在 0-100 范围
	result.OverallScore = clampFloat(math.Round(result.OverallScore), 0, 100)
	result.SkillMatch = clampFloat(math.Round(result.SkillMatch), 0, 100)
	result.ExperienceMatch = clampFloat(math.Round(result.ExperienceMatch), 0, 100)
	result.EducationMatch = clampFloat(math.Round(result.EducationMatch), 0, 100)

	// 验证推荐等级
	validRecs := map[string]bool{
		"strong_recommend": true,
		"recommend":        true,
		"consider":         true,
		"not_recommend":    true,
	}
	if !validRecs[result.Recommendation] {
		// 根据分数自动设置
		switch {
		case result.OverallScore >= 85:
			result.Recommendation = "strong_recommend"
		case result.OverallScore >= 70:
			result.Recommendation = "recommend"
		case result.OverallScore >= 60:
			result.Recommendation = "consider"
		default:
			result.Recommendation = "not_recommend"
		}
	}

	result.AnalyzedAt = time.Now().Format(time.RFC3339)

	return &result, nil
}

func clampFloat(value, minVal, maxVal float64) float64 {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================
// 版本检测与链接
// ============================================

// GetAppVersion 返回当前应用版本号
func (a *App) GetAppVersion() string {
	return AppVersion
}

// OpenURL 用系统浏览器打开链接
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// CheckForUpdate 检查 GitHub 是否有新版本
// 优先用 releases/latest，失败则回退到 tags 列表
func (a *App) CheckForUpdate() map[string]interface{} {
	result := map[string]interface{}{
		"hasUpdate":      false,
		"currentVersion": AppVersion,
		"latestVersion":  AppVersion,
		"releaseURL":     fmt.Sprintf("https://github.com/%s/releases", GitHubRepo),
		"error":          "",
	}

	// 尝试两个 API：releases/latest 和 tags
	urls := []string{
		fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo),
		fmt.Sprintf("https://api.github.com/repos/%s/tags?per_page=1", GitHubRepo),
	}

	client := &http.Client{Timeout: 15 * time.Second}

	for i, apiURL := range urls {
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "TalentLens/"+AppVersion)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[CheckForUpdate] API %d 网络错误: %v", i, err)
			result["error"] = fmt.Sprintf("网络错误: %v", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		log.Printf("[CheckForUpdate] API %d 状态=%d, URL=%s", i, resp.StatusCode, apiURL)

		if resp.StatusCode != 200 {
			result["error"] = fmt.Sprintf("GitHub 返回 %d", resp.StatusCode)
			continue
		}

		// 解析结果
		var tagName string
		var htmlURL string

		if i == 0 {
			// releases/latest 返回单个对象
			var release struct {
				TagName string `json:"tag_name"`
				HTMLURL string `json:"html_url"`
			}
			if json.Unmarshal(body, &release) == nil && release.TagName != "" {
				tagName = release.TagName
				htmlURL = release.HTMLURL
			}
		} else {
			// tags 返回数组
			var tags []struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(body, &tags) == nil && len(tags) > 0 {
				tagName = tags[0].Name
				htmlURL = fmt.Sprintf("https://github.com/%s/releases/tag/%s", GitHubRepo, tagName)
			}
		}

		if tagName == "" {
			continue
		}

		latestVersion := strings.TrimPrefix(tagName, "v")
		result["latestVersion"] = latestVersion
		result["releaseURL"] = htmlURL
		result["error"] = ""

		if compareVersions(latestVersion, AppVersion) > 0 {
			result["hasUpdate"] = true
		}

		log.Printf("[CheckForUpdate] 成功! 当前=%s, 最新=%s, 有更新=%v", AppVersion, latestVersion, result["hasUpdate"])
		return result
	}

	return result
}

// compareVersions 比较语义化版本号，返回 1(a>b), 0(a==b), -1(a<b)
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
		}
		if numA > numB {
			return 1
		}
		if numA < numB {
			return -1
		}
	}
	return 0
}

// logError 记录详细错误日志到本地磁盘文件
func (a *App) logError(fileName, errMsg, prompt, rawResp string) {
	logDir := filepath.Join(a.getDataDir(), "logs")
	os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, fmt.Sprintf("error_%s.log", time.Now().Format("20060102")))
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		divider := strings.Repeat("=", 60)
		logEntry := fmt.Sprintf("\n%s\n【时间】%s\n【文件】%s\n【错误详情】%s\n【AI原始返回】\n%s\n%s\n",
			divider, time.Now().Format("2006-01-02 15:04:05"), fileName, errMsg, rawResp, divider)
		f.WriteString(logEntry)
		log.Printf("[logError] 已记录错误诊断日志: %s", logFile)
	}
}

// GetErrorReport 获取今天的错误日志内容
func (a *App) GetErrorReport() string {
	logDir := filepath.Join(a.getDataDir(), "logs")
	logFile := filepath.Join(logDir, fmt.Sprintf("error_%s.log", time.Now().Format("20060102")))
	data, err := os.ReadFile(logFile)
	if err != nil || len(data) == 0 {
		return "暂无今日错误日志记录"
	}
	return string(data)
}

// ExportErrorReportFile 打开日志文件夹
func (a *App) ExportErrorReportFile() {
	logDir := filepath.Join(a.getDataDir(), "logs")
	os.MkdirAll(logDir, 0755)
	runtime.BrowserOpenURL(a.ctx, logDir)
}
