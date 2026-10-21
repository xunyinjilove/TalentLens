package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/xuri/excelize/v2"
)

// 版本信息
const AppVersion = "1.8.44"
const analysisFormatRetryVersion = "model-json-repair-v1"
const GitHubRepo = "xunyinjilove/TalentLens"

// cleanJSONBytes 自动移除 UTF-8 BOM 头，防御 Windows/PowerShell 等工具引入的 \xef\xbb\xbf
func cleanJSONBytes(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
}

// readJSONFile 读取文件并自动剔除 UTF-8 BOM
func readJSONFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return cleanJSONBytes(data), nil
}

//go:embed all:frontend/dist
var assets embed.FS

// Config 配置结构
type Config struct {
	AI     AIConfig     `json:"ai"`
	Job    JobConfig    `json:"job"`
	Window WindowConfig `json:"window,omitempty"`
}

// WindowConfig 窗口持久化尺寸与状态
type WindowConfig struct {
	Width       int  `json:"width,omitempty"`
	Height      int  `json:"height,omitempty"`
	IsMaximized bool `json:"is_maximized,omitempty"`
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
	Title                string   `json:"title"`
	Requirements         []string `json:"requirements"`
	RequiredSkills       []string `json:"required_skills"`
	ExperienceYears      int      `json:"experience_years"`
	EducationLevel       string   `json:"education_level"`
	JobDescription       string   `json:"job_description,omitempty"`
	City                 string   `json:"city,omitempty"`
	EmploymentType       string   `json:"employment_type,omitempty"`
	SalaryRange          string   `json:"salary_range,omitempty"`
	ExperienceText       string   `json:"experience_text,omitempty"`       // 原始自定义经验要求，输入什么就是什么
	InterviewQuestions   []string `json:"interview_questions,omitempty"`   // 初试重点核查题纲与重点问题
	RedLines             []string `json:"red_lines,omitempty"`             // 兼容旧项目；已并入初试问题
	BonusPoints          []string `json:"bonus_points,omitempty"`          // 兼容旧项目；已并入初试问题
	EquivalentExperience []string `json:"equivalent_experience,omitempty"` // 旧项目兼容读取；不再参与配置或评估
	TrainableSkills      []string `json:"trainable_skills,omitempty"`      // 旧项目兼容读取；不再参与配置或评估
	CoreCriteria         []string `json:"core_criteria,omitempty"`         // 岗位能力评分卡；为空时使用 RequiredSkills
}

// GetInterviewQuestions 获取初试重点核查题纲（自动兼容平滑继承历史保存的红线与加分项内容）
func (j *JobConfig) GetInterviewQuestions() []string {
	if j == nil {
		return nil
	}
	var questions []string
	seen := make(map[string]bool)
	for _, q := range j.InterviewQuestions {
		q = strings.TrimSpace(q)
		if q != "" && !seen[q] {
			seen[q] = true
			questions = append(questions, q)
		}
	}
	// 兼容迁移：旧项目中保存的红线与加分项自动作为初试重点核查问题
	for _, rl := range j.RedLines {
		rl = strings.TrimSpace(rl)
		if rl != "" && !seen[rl] {
			seen[rl] = true
			questions = append(questions, rl)
		}
	}
	for _, bp := range j.BonusPoints {
		bp = strings.TrimSpace(bp)
		if bp != "" && !seen[bp] {
			seen[bp] = true
			questions = append(questions, bp)
		}
	}
	return questions
}

// GetExperienceRequirement 返回精准且原汁原味的经验要求文本
func (j *JobConfig) GetExperienceRequirement() string {
	if strings.TrimSpace(j.ExperienceText) != "" {
		return strings.TrimSpace(j.ExperienceText)
	}
	if j.ExperienceYears <= 0 {
		return "经验不限"
	}
	return fmt.Sprintf("%d 年以上", j.ExperienceYears)
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

// WaterCheckResult 简历防伪注水雷达与断层侦测结果
type WaterCheckResult struct {
	WaterScore         int      `json:"water_score"`                    // 注水/破绽风险分 (0-100分，越低越真实安全)
	RiskLevel          string   `json:"risk_level"`                     // "low" | "medium" | "high"
	Gaps               []string `json:"gaps"`                           // 职场时间断层/空窗期侦测
	VagueClaims        []string `json:"vague_claims"`                   // 假大空/缺乏量化成果描述
	OutsourcingWarning string   `json:"outsourcing_warning,omitempty"`  // 外包驻场/挂靠疑点提示
	FrequentHopWarning string   `json:"frequent_hop_warning,omitempty"` // 频繁跳槽/履历动荡提示
	AdviseQuestions    []string `json:"advise_questions"`               // 初试防伪一针见血反问建议
}

// Resume 简历结构
type Resume struct {
	ID                        string          `json:"id"`
	ProjectID                 string          `json:"project_id"`
	FileName                  string          `json:"file_name"`
	FilePath                  string          `json:"file_path"`
	FileType                  string          `json:"file_type"`
	FileSize                  int64           `json:"file_size"`
	Content                   string          `json:"content"`
	URL                       string          `json:"url,omitempty"`
	SourcePlatform            string          `json:"source_platform,omitempty"`
	PlatformCandidateID       string          `json:"platform_candidate_id,omitempty"`
	SnapshotCopyIDs           []string        `json:"snapshot_copy_ids,omitempty"`
	Email                     string          `json:"email,omitempty"`
	Status                    string          `json:"status"`
	Score                     int             `json:"score"`
	InitialScore              int             `json:"initial_score,omitempty"`
	FinalScore                int             `json:"final_score,omitempty"`
	HasAttachment             bool            `json:"has_attachment,omitempty"`
	AttachmentPath            string          `json:"attachment_path,omitempty"`
	AttachmentFileName        string          `json:"attachment_file_name,omitempty"`
	AttachmentContent         string          `json:"attachment_content,omitempty"`
	IsMergedAnalysis          bool            `json:"is_merged_analysis,omitempty"`
	SourceKeyword             string          `json:"source_keyword,omitempty"`
	ActivityLabel             string          `json:"activity_label,omitempty"` // 平台原始活跃标签；未披露时留空
	ContactStatus             string          `json:"contact_status,omitempty"` // 51job: greeted | resume_requested | resume_received
	ContactUpdatedAt          string          `json:"contact_updated_at,omitempty"`
	ContactAttachmentBaseline []string        `json:"contact_attachment_baseline,omitempty"`
	ErrorMessage              string          `json:"error_message,omitempty"`
	AnalysisAutoRetryVersion  string          `json:"analysis_auto_retry_version,omitempty"`
	Analysis                  *AnalysisResult `json:"analysis,omitempty"`
	AnalysisRuleVersion       int             `json:"analysis_rule_version,omitempty"`
	CreatedAt                 time.Time       `json:"created_at"`
}

// JobSynonymItem 岗位同义词与拓扑维度词条
type JobSynonymItem struct {
	Keyword      string `json:"keyword"`
	Category     string `json:"category"`     // "standard" | "senior" | "derivative"
	CategoryName string `json:"categoryName"` // "标准称谓" | "高阶下探" | "业务衍生"
	Description  string `json:"description"`  // 词条应用背景与人群属性
	Ratio        int    `json:"ratio"`        // 推荐配额比例 (例如 40, 30, 30)
}

// AnalysisResult AI分析结果
type AnalysisResult struct {
	OverallScore    float64 `json:"overall_score"`
	SkillMatch      float64 `json:"skill_match"`
	ExperienceMatch float64 `json:"experience_match"`
	EducationMatch  float64 `json:"education_match"`
	Recommendation  string  `json:"recommendation"`

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

	// 业务主管极简推介卡（微信/钉钉一键转发）
	ManagerPitch string `json:"manager_pitch,omitempty"`

	// 简历防伪注水雷达与断层侦测
	WaterCheck *WaterCheckResult `json:"water_check,omitempty"`

	// 岗位画像核验：一票否决红线与优先加分项
	RedLineViolations      []string             `json:"red_line_violations,omitempty"` // 仅 HR 核实不满足的红线；与能力分分开
	RedLineChecks          []RedLineCheck       `json:"red_line_checks,omitempty"`
	RedLineStatus          string               `json:"red_line_status,omitempty"` // passed | failed | pending | not_configured
	CoreMatch              float64              `json:"core_match,omitempty"`
	BonusMatch             float64              `json:"bonus_match,omitempty"`
	AbilityScore           int                  `json:"ability_score,omitempty"` // 核心能力原始分，不受红线状态影响
	ScoreBasis             string               `json:"score_basis,omitempty"`   // rubric | provisional | legacy；证据不足时明确标记暂估
	BonusScore             int                  `json:"bonus_score,omitempty"`   // 旧字段，现固定为 0；确认项只用于同档排序
	BonusMatches           []string             `json:"bonus_matches,omitempty"` // 命中的核心加分项列表
	BonusChecks            []BonusCheck         `json:"bonus_checks,omitempty"`
	ConfirmedBonusCount    int                  `json:"confirmed_bonus_count,omitempty"`
	CoreCriterionChecks    []CoreCriterionCheck `json:"core_criterion_checks,omitempty"`
	ExperienceCompetencies []CoreCriterionCheck `json:"experience_competencies,omitempty"`
	CompetencyStatus       string               `json:"competency_status,omitempty"` // completed | insufficient | error
	CompetencyMessage      string               `json:"competency_message,omitempty"`

	// 契约化雷达覆盖率与全维度评分区间
	RadarCoverage   float64 `json:"radar_coverage,omitempty"`    // 已知维度权重覆盖率 (0.0~1.0)
	RankEligible    bool    `json:"rank_eligible,omitempty"`     // 是否达到 80% 覆盖率排名资格
	ScoreLowerBound float64 `json:"score_lower_bound,omitempty"` // 全维度下界 (未知项按0分估算)
	ScoreUpperBound float64 `json:"score_upper_bound,omitempty"` // 全维度上界 (未知项按4分满分估算)

	// 面试建议
	InterviewSuggestions []string `json:"interview_suggestions"`

	// AI 预面试提纲与参考回答（5组）
	InterviewQA []InterviewQuestion `json:"interview_qa,omitempty"`

	// 性能、成本与阶段记录
	TokenUsage    *AIUsage `json:"token_usage,omitempty"`
	AnalysisStage string   `json:"analysis_stage,omitempty"` // screening | deep_report

	AnalyzedAt string `json:"analyzed_at"`
}

type RedLineCheck struct {
	RuleID          string               `json:"rule_id,omitempty"`
	Criterion       string               `json:"criterion"`
	Status          string               `json:"status"`                     // met | violated | unknown；AI 只能产出 unknown
	SuggestedStatus string               `json:"suggested_status,omitempty"` // AI 初步判断，不是录用结论
	Evidence        string               `json:"evidence"`
	ReviewedAt      string               `json:"reviewed_at,omitempty"`
	ReviewHistory   []RedLineReviewEvent `json:"review_history,omitempty"`
}

type RedLineReviewEvent struct {
	Status     string `json:"status"`
	Evidence   string `json:"evidence"`
	Source     string `json:"source"` // hr_manual；当前本地软件没有个人身份认证
	ReviewedAt string `json:"reviewed_at"`
}

type BonusCheck struct {
	RuleID          string               `json:"rule_id,omitempty"`
	Criterion       string               `json:"criterion"`
	Status          string               `json:"status"` // unknown | met | violated；met 必须由 HR 保存
	SuggestedStatus string               `json:"suggested_status,omitempty"`
	Evidence        string               `json:"evidence"`
	ReviewedAt      string               `json:"reviewed_at,omitempty"`
	ReviewHistory   []RedLineReviewEvent `json:"review_history,omitempty"`
}

type CoreCriterionCheck struct {
	Criterion      string `json:"criterion"`
	Level          int    `json:"level"` // 0 未体现，1 了解，2 参与，3 独立完成，4 主导并可复盘
	Evidence       string `json:"evidence,omitempty"`
	Assessment     string `json:"assessment,omitempty"`
	EvidenceStatus string `json:"evidence_status,omitempty"` // known | unknown | conflict
}

// InterviewQuestion 结构化面试问题及基于简历的参考回答
type InterviewQuestion struct {
	Category        string `json:"category"`         // 技术深度 / 项目实战 / 真实性与短板
	Question        string `json:"question"`         // 针对性面试提问
	ReferenceAnswer string `json:"reference_answer"` // 基于简历的参考要点与依据
}

// OpenAI API 请求/响应结构
type ChatRequest struct {
	Model           string        `json:"model"`
	Messages        []ChatMessage `json:"messages"`
	Temperature     float64       `json:"temperature,omitempty"`
	MaxTokens       int           `json:"max_tokens,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
}

func isGLM53Flash(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "ZHIPU/GLM-5.3-Flash")
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AIUsage 记录单次大模型调用的 Token 消耗详情
type AIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens            int `json:"prompt_tokens"`
		CompletionTokens        int `json:"completion_tokens"`
		TotalTokens             int `json:"total_tokens"`
		CompletionTokensDetails *struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details,omitempty"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type App struct {
	ctx                context.Context
	config             Config
	activeProjectID    string // 当前活跃的项目ID（前端设置）
	bossCmd            *exec.Cmd
	bossStopFile       string // 多平台任务的协作停机标记；空值表示旧版任务
	activeSearchTaskID string
	searchStarting     bool
	bossMutex          sync.Mutex
	interviewMutex     sync.Mutex
	contactMutex       sync.Mutex
	translationMutex   sync.Mutex
	outreachMutex      sync.Mutex
	jobSyncMu          sync.Mutex
	contactPending     map[string]pending51JobHiQuote
	analysisMutex      sync.Mutex
	activeAnalyses     map[string]bool
	activeProjects     map[string]bool
	dataDirOverride    string // 测试使用的隔离数据目录；生产环境为空
	recruitmentDBMu    sync.Mutex
	recruitmentDB      *sql.DB
	recruitmentDir     string
}

func NewApp() *App {
	return &App{}
}

func main() {
	app := NewApp()
	app.loadConfig()

	initWidth := 1200
	initHeight := 800
	if app.config.Window.Width >= 900 {
		initWidth = app.config.Window.Width
	}
	if app.config.Window.Height >= 600 {
		initHeight = app.config.Window.Height
	}

	err := wails.Run(&options.App{
		Title:            "Tellgen",
		Width:            initWidth,
		Height:           initHeight,
		MinWidth:         900,
		MinHeight:        600,
		Frameless:        true,
		DisableResize:    false,
		StartHidden:      false,
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: func(context.Context) { app.closeRecruitmentDB() },
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
	if a.ctx != nil {
		runtime.WindowMinimise(a.ctx)
	}
}

// WindowMaximize 最大化/还原窗口
func (a *App) WindowMaximize() {
	if a.ctx != nil {
		runtime.WindowToggleMaximise(a.ctx)
		// 状态变更后异步保存
		go func() {
			time.Sleep(150 * time.Millisecond)
			if a.ctx != nil {
				w, h := runtime.WindowGetSize(a.ctx)
				isMax := runtime.WindowIsMaximised(a.ctx)
				_ = a.SaveWindowSettings(w, h, isMax)
			}
		}()
	}
}

// WindowClose 关闭窗口并保存最终尺寸与状态
func (a *App) WindowClose() {
	if a.ctx != nil {
		w, h := runtime.WindowGetSize(a.ctx)
		isMax := runtime.WindowIsMaximised(a.ctx)
		_ = a.SaveWindowSettings(w, h, isMax)
		runtime.Quit(a.ctx)
	}
}

// StartWindowDrag 启动原生平滑窗口拖动（传递当前真实光标绝对坐标）
func (a *App) StartWindowDrag() {
	go startWindowNCDown(2 /* HTCAPTION */)
}

// StartWindowResize 启动原生无死角八方窗口尺寸调整（传递当前真实光标绝对坐标）
func (a *App) StartWindowResize(edge string) {
	borderMap := map[string]uintptr{
		"n-resize":  12, // HTTOP
		"ne-resize": 14, // HTTOPRIGHT
		"e-resize":  11, // HTRIGHT
		"se-resize": 17, // HTBOTTOMRIGHT
		"s-resize":  15, // HTBOTTOM
		"sw-resize": 16, // HTBOTTOMLEFT
		"w-resize":  10, // HTLEFT
		"nw-resize": 13, // HTTOPLEFT
	}
	if border, ok := borderMap[edge]; ok {
		go startWindowNCDown(border)
	}
}

// GetWindowSettings 获取当前窗口记忆配置
func (a *App) GetWindowSettings() WindowConfig {
	return a.config.Window
}

// SaveWindowSettings 保存窗口尺寸与状态到本地配置文件
func (a *App) SaveWindowSettings(width, height int, isMaximized bool) error {
	if width < 900 {
		width = 900
	}
	if height < 600 {
		height = 600
	}
	a.config.Window = WindowConfig{
		Width:       width,
		Height:      height,
		IsMaximized: isMaximized,
	}
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.getConfigPath(), data, 0644)
}

// SetWindowSize 动态设置窗口尺寸并居中
func (a *App) SetWindowSize(width, height int) {
	if width < 900 {
		width = 900
	}
	if height < 600 {
		height = 600
	}
	if a.ctx != nil {
		runtime.WindowUnmaximise(a.ctx)
		runtime.WindowSetSize(a.ctx, width, height)
		runtime.WindowCenter(a.ctx)
		_ = a.SaveWindowSettings(width, height, false)
	}
}

// CenterWindow 将窗口在当前屏幕居中
func (a *App) CenterWindow() {
	if a.ctx != nil {
		runtime.WindowCenter(a.ctx)
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadConfig()
	if a.config.Window.IsMaximized {
		runtime.WindowMaximise(ctx)
	}
	if _, err := a.getRecruitmentDB(); err != nil {
		log.Printf("[RecruitmentStore] SQLite 迁移未完成，保留旧数据入口: %v", err)
	} else if err := a.interruptUnfinishedSourcingTasks(); err != nil {
		log.Printf("[RecruitmentStore] 检索任务恢复状态失败: %v", err)
	}

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
			data, _ := readJSONFile(resumePath)
			var resume Resume
			json.Unmarshal(data, &resume)
			runtime.EventsEmit(a.ctx, "resume:dropped", &resume)
			added++
		}
		log.Printf("[OnFileDrop] 成功添加 %d 个文件", added)
	})

	log.Println("TalentLens 已启动")
}

func shouldAutoRetryFormatFailure(resume *Resume) bool {
	return resume != nil && resume.Status == "error" &&
		strings.HasPrefix(resume.ErrorMessage, "解析AI返回失败") &&
		resume.AnalysisAutoRetryVersion != analysisFormatRetryVersion
}

// retryLegacyFormatFailuresOnce revisits only the old model-format failures.
// The marker is saved before the AI request so a crash or uncertain response
// cannot spend model credits again on every application start.
func (a *App) retryLegacyFormatFailuresOnce() {
	cfg := a.config.AI
	if cfg.APIKey == "" || cfg.BaseURL == "" {
		return
	}
	dir := filepath.Join(a.getDataDir(), "resumes")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := readJSONFile(path)
		if err != nil {
			continue
		}
		var resume Resume
		if json.Unmarshal(data, &resume) != nil || !shouldAutoRetryFormatFailure(&resume) {
			continue
		}
		project := a.GetProject(resume.ProjectID)
		if project == nil {
			continue
		}
		resume.AnalysisAutoRetryVersion = analysisFormatRetryVersion
		marked, err := json.MarshalIndent(&resume, "", "  ")
		if err != nil || os.WriteFile(path, marked, 0600) != nil {
			continue
		}
		a.syncRecruitmentResume(&resume)
		if _, err := a.AnalyzeResume(resume.ID, &cfg, &project.JobConfig); err != nil {
			log.Printf("[AnalysisAutoRetry] 旧格式失败记录自动重试未成功: %v", err)
		}
	}
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
		data, _ := readJSONFile(resumePath)
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
	if a.dataDirOverride != "" {
		return a.dataDirOverride
	}
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
	data, err := readJSONFile(a.getConfigPath())
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

	// 自动防护与纠正：清理测试临时端口污染或残留的虚拟 Key
	baseURL := strings.TrimSpace(a.config.AI.BaseURL)
	needSave := false
	if strings.Contains(baseURL, "127.0.0.1:") || strings.Contains(baseURL, "localhost:") {
		provider := strings.ToLower(strings.TrimSpace(a.config.AI.Provider))
		if provider == "deepseek" || provider == "" {
			log.Printf("[loadConfig] 自动纠正被污染的本地 BaseURL (%s) 为 DeepSeek 官方服务地址", baseURL)
			a.config.AI.BaseURL = "https://api.deepseek.com"
			needSave = true
		} else if provider == "openai" {
			a.config.AI.BaseURL = "https://api.openai.com/v1"
			needSave = true
		}
	}
	if a.config.AI.APIKey == "custom-client-key-123" {
		log.Printf("[loadConfig] 清理测试残留的虚拟 Key")
		a.config.AI.APIKey = ""
		needSave = true
	}
	if needSave {
		_ = a.SaveConfig(&a.config)
	}
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
	case ".docx":
		content, err := extractDOCXText(filePath)
		if err != nil {
			log.Printf("[extractText] DOCX 正文提取失败: %v", err)
			return ""
		}
		return a.truncateContent(content, 50000)
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
	a.syncRecruitmentResume(r)
}

// persistSearchCandidate 仅在简历与所属项目都落盘后确认采集成功。
func (a *App) persistSearchCandidate(r *Resume) error {
	if !interviewIDPattern.MatchString(r.ID) || !interviewIDPattern.MatchString(r.ProjectID) {
		return fmt.Errorf("候选人或项目 ID 无效")
	}
	p := a.GetProject(r.ProjectID)
	if p == nil || p.ID != r.ProjectID {
		return fmt.Errorf("招聘项目不存在")
	}
	dir := filepath.Join(a.getDataDir(), "resumes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	resumeData, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, r.ID+".json"), resumeData, 0644); err != nil {
		return err
	}
	for _, id := range p.ResumeIDs {
		if id == r.ID {
			return nil
		}
	}
	// 入库前去重检测：若项目中已存在相同候选人，更新已有档案的最新元数据，防止 ResumeIDs 膨胀与重复分析
	existingResumes := a.GetProjectResumes(p.ID)
	candKey := extractCandidateDeduplicationKey(r)
	if candKey != "" {
		for _, ext := range existingResumes {
			if extractCandidateDeduplicationKey(ext) == candKey {
				updated := false
				if r.PlatformCandidateID != "" && ext.PlatformCandidateID != r.PlatformCandidateID {
					ext.PlatformCandidateID = r.PlatformCandidateID
					updated = true
				}
				if r.URL != "" && ext.URL != r.URL {
					ext.URL = r.URL
					updated = true
				}
				if updated {
					a.saveResume(ext)
					a.syncRecruitmentResume(ext)
				}
				return nil
			}
		}
	}
	p.ResumeIDs = append(p.ResumeIDs, r.ID)
	p.UpdatedAt = time.Now()
	projectData, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(a.getProjectsDir(), p.ID+".json"), projectData, 0644); err != nil {
		return err
	}
	a.syncRecruitmentProject(p)
	a.syncRecruitmentResume(r)
	return nil
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

// SaveAIConfig 单独保存并持久化 AI 模型配置，避免误触或覆写岗位规则
func (a *App) SaveAIConfig(cfg *AIConfig) error {
	if cfg == nil {
		return fmt.Errorf("AI 配置不能为空")
	}
	a.config.AI = *cfg
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
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
		data, _ := readJSONFile(filepath.Join(dir, entry.Name()))
		var r Resume
		if json.Unmarshal(data, &r) != nil {
			continue
		}
		if p := a.GetProject(r.ProjectID); p != nil {
			a.prepareResumeDecision(&r, &p.JobConfig)
		}
		resumes = append(resumes, &r)
	}
	return resumes
}

func (a *App) DeleteResume(id string) error {
	_ = a.deleteRecruitmentResume(id)
	path := filepath.Join(a.getDataDir(), "resumes", id+".json")
	return os.Remove(path)
}

func (a *App) ReAnalyzeResume(id string) error {
	path := filepath.Join(a.getDataDir(), "resumes", id+".json")
	data, err := readJSONFile(path)
	if err != nil {
		return err
	}
	var r Resume
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	r.Status = "pending"
	r.ErrorMessage = ""
	data, err = json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	a.syncRecruitmentResume(&r)
	runtime.EventsEmit(a.ctx, "resume:updated", &r)
	return nil
}

// ClearProjectResumes 仅清空指定项目的简历记录；原始文件和导出报告不属于应用缓存，不删除。
func (a *App) ClearProjectResumes(projectID string) error {
	if !interviewIDPattern.MatchString(projectID) {
		return fmt.Errorf("项目 ID 无效")
	}

	// 与分析任务的注册共用一把锁：清空期间不能启动新的分析任务。
	a.analysisMutex.Lock()
	defer a.analysisMutex.Unlock()
	if a.activeProjects[projectID] {
		return fmt.Errorf("当前项目正在分析，完成后才能清空")
	}
	p := a.GetProject(projectID)
	if p == nil {
		return fmt.Errorf("招聘项目不存在")
	}

	// 项目引用可能来自旧版本数据。只删除确实属于当前项目的记录，避免串删。
	var ownedPaths []string
	for _, id := range p.ResumeIDs {
		if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) || filepath.Base(id) != id {
			continue
		}
		if a.activeAnalyses[id] {
			return fmt.Errorf("当前项目正在分析，完成后才能清空")
		}
		path := filepath.Join(a.getDataDir(), "resumes", id+".json")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			continue
		}
		var resume Resume
		if err := json.Unmarshal(data, &resume); err == nil {
			if resume.ProjectID == projectID {
				ownedPaths = append(ownedPaths, path)
			}
		}
	}

	p.ResumeIDs = []string{}
	p.Status = "draft"
	p.UpdatedAt = time.Now()
	projectData, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(a.getProjectsDir(), projectID+".json"), projectData, 0644); err != nil {
		return err
	}
	for _, path := range ownedPaths {
		_ = os.Remove(path)
	}

	// 事务级级联清理 SQLite 数据库中该项目的候选人与评估流转记录
	_ = a.clearRecruitmentProjectResumes(projectID)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "resume:updated", map[string]interface{}{"projectId": projectID})
		runtime.EventsEmit(a.ctx, "project:updated", p)
	}

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
	if resume.ContactStatus == "resume_requested" {
		resume.ContactStatus = "resume_received"
		resume.ContactUpdatedAt = time.Now().Format(time.RFC3339)
	}
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
	a.syncRecruitmentProject(p)
}

// CreateProject 创建招聘项目
var projectIDSeq atomic.Int64

func (a *App) CreateProject(name string, department string, headcount int, recruiter string, remark string, jobCfg *JobConfig) *Project {
	if headcount <= 0 {
		headcount = 1
	}
	if jobCfg == nil {
		jobCfg = &JobConfig{Title: name}
	} else if jobCfg.Title == "" {
		jobCfg.Title = name
	}
	seq := projectIDSeq.Add(1)
	p := &Project{
		ID:         fmt.Sprintf("proj_%d_%d", time.Now().UnixNano(), seq),
		Name:       name,
		Department: department,
		Headcount:  headcount,
		Recruiter:  recruiter,
		Remark:     remark,
		JobConfig:  *jobCfg,
		ResumeIDs:  []string{},
		Status:     "publishing",
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
		data, err := readJSONFile(filepath.Join(dir, entry.Name()))
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
	data, err := readJSONFile(filepath.Join(a.getProjectsDir(), id+".json"))
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
	_ = a.deleteRecruitmentProject(id)
	return os.Remove(filepath.Join(a.getProjectsDir(), id+".json"))
}

// cleanCandidateNameKey 提取干净规范的候选人姓名
func cleanCandidateNameKey(r *Resume) string {
	if r == nil {
		return ""
	}
	name := ""
	if r.Analysis != nil && strings.TrimSpace(r.Analysis.CandidateName) != "" {
		name = strings.TrimSpace(r.Analysis.CandidateName)
	}
	if name == "" {
		name = conversationCandidateName(r)
	}
	name = regexp.MustCompile(`^【.*?】\s*`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`^BOSS牛人_\s*`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`\s*\(\d+\)$`).ReplaceAllString(name, "")
	if strings.Contains(name, "_") {
		name = strings.Split(name, "_")[0]
	}
	return strings.TrimSpace(name)
}

// extractCandidateDeduplicationKey 提取候选人全局排重键，将同名+同经历/公司或同平台ID的副本归一
func extractCandidateDeduplicationKey(r *Resume) string {
	if r == nil {
		return ""
	}
	name := cleanCandidateNameKey(r)
	content := r.Content

	company := ""
	profile := ""

	lines := strings.Split(content, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "基本画像：") {
			p := strings.TrimPrefix(l, "基本画像：")
			p = regexp.MustCompile(`\d+[日天周月]内活跃\s*·?\s*`).ReplaceAllString(p, "")
			p = regexp.MustCompile(`电话沟通\s*·?\s*`).ReplaceAllString(p, "")
			profile = strings.TrimSpace(p)
		} else if company == "" && (strings.HasPrefix(l, "任职履历快照：") || strings.Contains(l, "工作经历")) {
			compReg := regexp.MustCompile(`([\p{Han}]{3,25}(?:科技|网络|集团|公司|信息|咨询|药房|软件|制药|服务))`)
			if m := compReg.FindStringSubmatch(l); len(m) > 1 {
				c := m[1]
				c = strings.TrimSuffix(c, "有限公司")
				c = strings.TrimSuffix(c, "股份有限公司")
				c = strings.TrimSuffix(c, "上海分公司")
				company = c
			}
		}
	}

	if name != "" && name != "候选人" {
		if company != "" {
			return fmt.Sprintf("cand:%s:%s", name, company)
		}
		if profile != "" {
			return fmt.Sprintf("cand:%s:%s", name, profile)
		}
	}

	platID := strings.TrimSpace(r.PlatformCandidateID)
	if platID == "" && strings.Contains(r.URL, "resumeId=") {
		if parsed, err := url.Parse(r.URL); err == nil {
			platID = parsed.Query().Get("resumeId")
		}
	}
	if platID != "" {
		plat := strings.TrimSpace(r.SourcePlatform)
		if plat == "" {
			plat = "51job"
		}
		return fmt.Sprintf("plat:%s:%s", plat, platID)
	}

	if name != "" && name != "候选人" {
		return fmt.Sprintf("name:%s", name)
	}
	return fmt.Sprintf("rec:%s", r.ID)
}

// pruneDuplicateProjectResumes 对项目下简历列表进行去重合并，优先保留已有详细分析结论的档案，
// 将多余副本ID移入 SnapshotCopyIDs，并持久化更新 project.ResumeIDs，恢复干净唯一候选人列表。
func (a *App) pruneDuplicateProjectResumes(p *Project, resumes []*Resume) []*Resume {
	if p == nil || len(resumes) <= 1 {
		return resumes
	}

	type candGroup struct {
		primary *Resume
		copies  []string
	}

	groups := make([]*candGroup, 0, len(resumes))
	indexMap := make(map[string]int)

	for _, r := range resumes {
		key := extractCandidateDeduplicationKey(r)
		idx, exists := indexMap[key]
		if !exists {
			indexMap[key] = len(groups)
			groups = append(groups, &candGroup{primary: r, copies: []string{}})
			continue
		}

		curr := groups[idx]
		prev := curr.primary

		// 判定是否用 r 替换 prev 作为主档案
		replace := false
		if r.Status == "done" && prev.Status != "done" {
			replace = true
		} else if r.Status == prev.Status {
			if r.PlatformCandidateID != "" && prev.PlatformCandidateID == "" {
				replace = true
			} else if (r.PlatformCandidateID != "") == (prev.PlatformCandidateID != "") {
				rTime := r.CreatedAt
				prevTime := prev.CreatedAt
				if r.Analysis != nil && r.Analysis.AnalyzedAt != "" {
					if t, err := time.Parse(time.RFC3339, r.Analysis.AnalyzedAt); err == nil {
						rTime = t
					}
				}
				if prev.Analysis != nil && prev.Analysis.AnalyzedAt != "" {
					if t, err := time.Parse(time.RFC3339, prev.Analysis.AnalyzedAt); err == nil {
						prevTime = t
					}
				}
				if rTime.After(prevTime) {
					replace = true
				}
			}
		}

		if replace {
			if r.PlatformCandidateID == "" && prev.PlatformCandidateID != "" {
				r.PlatformCandidateID = prev.PlatformCandidateID
			}
			if r.URL == "" && prev.URL != "" {
				r.URL = prev.URL
			}
			curr.copies = append(curr.copies, prev.ID)
			curr.copies = append(curr.copies, prev.SnapshotCopyIDs...)
			curr.primary = r
		} else {
			if prev.PlatformCandidateID == "" && r.PlatformCandidateID != "" {
				prev.PlatformCandidateID = r.PlatformCandidateID
			}
			if prev.URL == "" && r.URL != "" {
				prev.URL = r.URL
			}
			curr.copies = append(curr.copies, r.ID)
			curr.copies = append(curr.copies, r.SnapshotCopyIDs...)
		}
	}

	result := make([]*Resume, 0, len(groups))
	var newResumeIDs []string
	for _, g := range groups {
		copySet := make(map[string]bool)
		for _, cid := range g.primary.SnapshotCopyIDs {
			copySet[cid] = true
		}
		for _, cid := range g.copies {
			if cid != g.primary.ID {
				copySet[cid] = true
			}
		}
		var cleanCopies []string
		for cid := range copySet {
			cleanCopies = append(cleanCopies, cid)
		}
		g.primary.SnapshotCopyIDs = cleanCopies
		result = append(result, g.primary)
		newResumeIDs = append(newResumeIDs, g.primary.ID)
	}

	// 若检测到重复被合并，自动同步写盘自愈
	if len(newResumeIDs) != len(p.ResumeIDs) {
		log.Printf("[pruneDuplicateProjectResumes] 项目 %s 简历排重自愈: %d -> %d", p.ID, len(p.ResumeIDs), len(newResumeIDs))
		p.ResumeIDs = newResumeIDs
		p.UpdatedAt = time.Now()
		a.saveProject(p)
	}

	return result
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
		data, err := readJSONFile(path)
		if err != nil {
			continue
		}
		var r Resume
		if json.Unmarshal(data, &r) == nil {
			if r.Status == "analyzing" && (a.activeAnalyses == nil || !a.activeAnalyses[r.ID]) {
				r.Status = "pending"
			}
			a.prepareResumeDecision(&r, &p.JobConfig)
			resumes = append(resumes, &r)
		}
	}
	folded := foldIdenticalResumeSnapshots(resumes)
	return a.pruneDuplicateProjectResumes(p, folded)
}

// prepareResumeDecision 在读取旧数据时将“AI 已满足/不满足”还原为待核验，
// 并重新计算不含加分项的能力分。此步骤不改写历史文件；HR 保存结论时再落盘。
func (a *App) prepareResumeDecision(resume *Resume, job *JobConfig) {
	if resume == nil || resume.Analysis == nil || job == nil {
		return
	}
	a.applyJobDecision(resume.Analysis, job, resume)
	resume.Score = int(resume.Analysis.OverallScore)
	resume.Analysis.ManagerPitch = a.buildReviewedManagerPitch(resume.Analysis, resume)
}

// GetProjectRanking 获取项目排名（按分数降序，同分时按创建时间降序保持顺序稳定）
func (a *App) GetProjectRanking(projectID string) []*Resume {
	resumes := a.GetProjectResumes(projectID)
	sortResumesByDecision(resumes)
	return resumes
}

func sortResumesByDecision(resumes []*Resume) {
	ability := func(r *Resume) int {
		if r.Analysis != nil && r.Analysis.AbilityScore > 0 {
			return r.Analysis.AbilityScore
		}
		return r.Score
	}
	sort.SliceStable(resumes, func(i, j int) bool {
		left, right := resumes[i], resumes[j]
		leftScore, rightScore := ability(left), ability(right)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.ID < right.ID
	})
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
		a.syncRecruitmentResume(resume)
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
	a.analysisMutex.Lock()
	p := a.GetProject(projectID)
	if p == nil {
		a.analysisMutex.Unlock()
		return
	}
	if a.activeProjects == nil {
		a.activeProjects = make(map[string]bool)
	}
	if a.activeProjects[projectID] {
		a.analysisMutex.Unlock()
		return
	}
	a.activeProjects[projectID] = true
	a.analysisMutex.Unlock()

	go func() {
		defer func() {
			a.analysisMutex.Lock()
			delete(a.activeProjects, projectID)
			a.analysisMutex.Unlock()
		}()
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
		if total == 0 {
			p.Status = "completed"
			a.saveProject(p)
			runtime.EventsEmit(a.ctx, "batch:completed", map[string]interface{}{
				"total":     0,
				"projectId": projectID,
			})
			return
		}

		// 工业级 Worker Pool 并发控制：并发度设为 3，既避免触发大模型频控限制，又提速 3 倍
		const maxWorkers = 3
		sem := make(chan struct{}, maxWorkers)
		var wg sync.WaitGroup
		var processedCount int32

		for _, id := range pendingIDs {
			wg.Add(1)
			sem <- struct{}{} // 申请工作槽位（超出则阻塞等待）

			go func(resumeID string) {
				defer func() {
					<-sem // 释放工作槽位
					wg.Done()
				}()

				_, err := a.AnalyzeResume(resumeID, cfg, &p.JobConfig)
				if err != nil {
					log.Printf("分析简历 %s 失败: %v", resumeID, err)
				}

				curr := atomic.AddInt32(&processedCount, 1)
				runtime.EventsEmit(a.ctx, "batch:progress", map[string]interface{}{
					"current":   curr,
					"total":     total,
					"resumeId":  resumeID,
					"projectId": projectID,
				})
			}(id)
		}

		// 等待当前批次全部完成
		wg.Wait()

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

// JobDraftRequest 为 51job 草稿提供项目外仍需由 HR 确认的字段；薪资绝不从旧岗位猜测。
type JobDraftRequest struct {
	ProjectID       string           `json:"projectId"`
	JobType         string           `json:"jobType"`
	EmploymentType  string           `json:"employmentType"`
	Majors          []JobMajor       `json:"majors"`
	Action          string           `json:"action"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	FunctionPath    string           `json:"functionPath"`
	Keywords        []string         `json:"keywords"`
	MinSalary       int              `json:"minSalary"`
	MaxSalary       int              `json:"maxSalary"`
	SalaryMonths    int              `json:"salaryMonths"`
	Headcount       int              `json:"headcount"`
	ExperienceYears int              `json:"experienceYears"`
	Education       string           `json:"education"`
	Address         string           `json:"address"`
	Language        string           `json:"language"`
	LanguageLevel   string           `json:"languageLevel"`
	Certificates    []JobCertificate `json:"certificates"`
}

type JobCertificate struct {
	Category string `json:"category"`
	Name     string `json:"name"`
}

type JobMajor struct {
	Value string   `json:"value"`
	Path  []string `json:"path"`
}

type JobPostingOptionsResult struct {
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	Options        []string `json:"options"`
	Levels         []string `json:"levels"`
	Categories     []string `json:"categories"`
	CurrentAddress string   `json:"currentAddress"`
	CurrentCity    string   `json:"currentCity"`
}

type JobKeywordGroup struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}

type JobKeywordSuggestionResult struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Groups  []JobKeywordGroup `json:"groups"`
}

type JobFunctionOption struct {
	Name string `json:"name"`
	Leaf bool   `json:"leaf"`
}

type JobFunctionOptionsResult struct {
	Status  string              `json:"status"`
	Message string              `json:"message"`
	Options []JobFunctionOption `json:"options"`
}

type JobBrowserLoginResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type JobDraftResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
}

func (a *App) Save51JobDraft(req JobDraftRequest) JobDraftResult {
	req.Action = "draft"
	return a.submit51Job(req)
}

// Publish51Job 由用户明确点击“立即发布”后调用；平台未确认时不得报告发布成功。
func (a *App) Publish51Job(req JobDraftRequest) JobDraftResult {
	req.Action = "publish"
	return a.submit51Job(req)
}

func find51JobScript() string {
	scriptCandidates := []string{
		filepath.Join("scripts", "51job_job_publisher.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "51job_job_publisher.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "scripts", "51job_job_publisher.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "51job_job_publisher.js"),
	}
	for _, candidate := range scriptCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func run51JobScript(payload []byte, timeout time.Duration, flags ...string) ([]byte, error) {
	scriptPath := find51JobScript()
	if scriptPath == "" {
		return nil, fmt.Errorf("缺少 51job 职位脚本，请检查安装目录的 scripts 文件夹")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", append([]string{scriptPath}, flags...)...)
	hideConsoleWindow(cmd)
	cmd.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		log.Printf("[51job] node failure: %v: %s", err, stderr.String())
		return nil, fmt.Errorf("职位脚本执行失败，请确认 Node.js 和脚本依赖已安装")
	}
	return out, nil
}

// Get51JobFunctionOptions 按已选上级读取 51job 职能选择器的下一列，不提交职位。
func (a *App) Get51JobFunctionOptions(path []string) JobFunctionOptionsResult {
	if len(path) > 2 {
		return JobFunctionOptionsResult{Status: "error", Message: "职能层级无效"}
	}
	for _, part := range path {
		if strings.TrimSpace(part) == "" {
			return JobFunctionOptionsResult{Status: "error", Message: "职能层级不能为空"}
		}
	}
	payload, _ := json.Marshal(map[string][]string{"path": path})
	out, err := run51JobScript(payload, 40*time.Second, "--function-options")
	if err != nil {
		return JobFunctionOptionsResult{Status: "error", Message: err.Error()}
	}
	var result JobFunctionOptionsResult
	if json.Unmarshal(bytes.TrimSpace(out), &result) != nil || result.Status == "" {
		return JobFunctionOptionsResult{Status: "error", Message: "无法解析 51job 职能列表"}
	}
	return result
}

// Show51JobPage 显示已有的专用浏览器；关闭后用原登录资料重新打开并确认可见。
func (a *App) Show51JobPage() JobDraftResult {
	show := func() JobDraftResult {
		out, err := run51JobScript([]byte("{}"), 12*time.Second, "--show-page")
		if err != nil {
			return JobDraftResult{Status: "error", Message: err.Error()}
		}
		var result JobDraftResult
		if json.Unmarshal(bytes.TrimSpace(out), &result) != nil || result.Status == "" {
			return JobDraftResult{Status: "error", Message: "无法解析浏览器状态"}
		}
		return result
	}
	if result := show(); result.Status == "ready" {
		return result
	}
	a.OpenURL("https://ehire.51job.com/Revision/job?mark=new")
	for attempt := 0; attempt < 12; attempt++ {
		time.Sleep(time.Second)
		if result := show(); result.Status == "ready" {
			return result
		}
	}
	return JobDraftResult{Status: "error", Message: "51job 专用浏览器未能显示，请检查 Edge/Chrome 是否安装，然后重试"}
}

// Check51JobLogin 核实职位表单可用，但不改变窗口状态；供前端登录期间轮询。
func (a *App) Check51JobLogin() JobBrowserLoginResult {
	out, err := run51JobScript([]byte("{}"), 40*time.Second, "--login-status")
	if err != nil {
		return JobBrowserLoginResult{Status: "error", Message: err.Error()}
	}
	var result JobBrowserLoginResult
	if json.Unmarshal(bytes.TrimSpace(out), &result) != nil || result.Status == "" {
		return JobBrowserLoginResult{Status: "error", Message: "无法解析 51job 登录状态"}
	}
	return result
}

// Minimize51JobBrowser 在前端确认登录监测仍有效后，最小化专用浏览器。
func (a *App) Minimize51JobBrowser() JobBrowserLoginResult {
	out, err := run51JobScript([]byte("{}"), 12*time.Second, "--minimize-page")
	if err != nil {
		return JobBrowserLoginResult{Status: "error", Message: err.Error()}
	}
	var result JobBrowserLoginResult
	if json.Unmarshal(bytes.TrimSpace(out), &result) != nil || result.Status == "" {
		return JobBrowserLoginResult{Status: "error", Message: "无法解析 51job 窗口状态"}
	}
	return result
}

// Get51JobKeywordSuggestions 从已登录的 51job 职能关键词面板读取实际推荐词，不提交职位。
func (a *App) Get51JobKeywordSuggestions(functionPath string) JobKeywordSuggestionResult {
	if len(strings.Split(functionPath, ">")) < 2 {
		return JobKeywordSuggestionResult{Status: "error", Message: "请先填写完整的 51job 职能路径"}
	}
	payload, _ := json.Marshal(map[string]string{"functionPath": functionPath})
	out, err := run51JobScript(payload, 40*time.Second, "--suggest-keywords")
	if err != nil {
		return JobKeywordSuggestionResult{Status: "error", Message: err.Error()}
	}
	var result JobKeywordSuggestionResult
	if json.Unmarshal(bytes.TrimSpace(out), &result) != nil || result.Status == "" {
		return JobKeywordSuggestionResult{Status: "error", Message: "无法解析 51job 推荐关键词"}
	}
	return result
}

// Get51JobPostingOptions 从已登录的职位表单读取已有地址、语言或证书，不提交职位。
func (a *App) Get51JobPostingOptions(kind, query, category string) JobPostingOptionsResult {
	if kind != "address" && kind != "language" && kind != "certificate" {
		return JobPostingOptionsResult{Status: "error", Message: "职位选项类型无效"}
	}
	payload, _ := json.Marshal(map[string]string{"kind": kind, "query": query, "category": category})
	out, err := run51JobScript(payload, 40*time.Second, "--posting-options")
	if err != nil {
		return JobPostingOptionsResult{Status: "error", Message: err.Error()}
	}
	var result JobPostingOptionsResult
	if json.Unmarshal(bytes.TrimSpace(out), &result) != nil || result.Status == "" {
		return JobPostingOptionsResult{Status: "error", Message: "无法解析 51job 职位选项"}
	}
	return result
}

// submit51Job 在已登录的 51job 浏览器填写职位并执行草稿保存或正式发布。
func (a *App) submit51Job(req JobDraftRequest) JobDraftResult {
	if !interviewIDPattern.MatchString(req.ProjectID) || a.GetProject(req.ProjectID) == nil {
		return JobDraftResult{Status: "error", Message: "招聘项目不存在，请刷新项目列表"}
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Description) == "" || req.MinSalary <= 0 || req.MaxSalary < req.MinSalary {
		return JobDraftResult{Status: "error", Message: "请填写职位名称、描述和有效的月薪范围"}
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return JobDraftResult{Status: "error", Message: "无法编码职位信息"}
	}
	out, err := run51JobScript(payload, 90*time.Second)
	if err != nil {
		if err == context.DeadlineExceeded {
			return JobDraftResult{Status: "uncertain", Message: "等待 51job 响应超时，请先在平台职位管理核对结果，暂勿重复提交"}
		}
		return JobDraftResult{Status: "error", Message: err.Error()}
	}
	var result JobDraftResult
	if json.Unmarshal(bytes.TrimSpace(out), &result) != nil || result.Status == "" {
		return JobDraftResult{Status: "uncertain", Message: "51job 返回了无法识别的结果，请在平台职位管理核对，暂勿重复提交"}
	}
	return result
}

// TestPlatformLogin 独立测试指定平台的企业端账号登录与扫码鉴权
func (a *App) TestPlatformLogin(platform string) bool {
	a.bossMutex.Lock()
	if a.searchStarting || (a.bossCmd != nil && a.bossCmd.Process != nil) {
		a.bossMutex.Unlock()
		runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
			"type": "error", "message": "已有检索任务正在运行，请先停止或等待该任务完成",
		})
		return false
	}
	a.searchStarting = true
	a.bossMutex.Unlock()
	defer func() {
		a.bossMutex.Lock()
		a.searchStarting = false
		a.bossMutex.Unlock()
	}()

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
		hideConsoleWindow(cmd)
		stdout, err := cmd.StdoutPipe()
		if err == nil {
			if err := cmd.Start(); err == nil {
				a.bossMutex.Lock()
				a.bossCmd = cmd
				a.bossStopFile = ""
				a.bossMutex.Unlock()

				go func() {
					scanner := bufio.NewScanner(stdout)
					// 扩容缓冲区上限至 4MB，防止单行 JSON 超出默认 64KB 限制导致静默截断
					buf := make([]byte, 64*1024)
					scanner.Buffer(buf, 4*1024*1024)

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
							case "captcha":
								runtime.EventsEmit(a.ctx, "boss:captcha", evt)
								runtime.EventsEmit(a.ctx, "platform:captcha", evt)
							case "captcha_resolved":
								runtime.EventsEmit(a.ctx, "boss:captcha_resolved", evt)
								runtime.EventsEmit(a.ctx, "platform:captcha_resolved", evt)
							case "done":
								runtime.EventsEmit(a.ctx, "boss:done", evt)
								runtime.EventsEmit(a.ctx, "platform:done", evt)
							case "error":
								runtime.EventsEmit(a.ctx, "boss:error", evt)
								runtime.EventsEmit(a.ctx, "platform:error", evt)
							}
						}
					}
					if err := scanner.Err(); err != nil {
						log.Printf("[TestPlatformLogin Scanner Error] 子进程管道读取异常: %v", err)
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
		return map[string]interface{}{"type": "action_result", "success": false, "message": "缺少明确候选人姓名，操作未执行"}
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
		hideConsoleWindow(cmd)
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
		"success":       false,
		"action":        actionType,
		"candidateName": cleanName,
		"message":       fmt.Sprintf("未能确认候选人【%s】的「%s」操作已执行，请检查浏览器及脚本", cleanName, label),
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

// StartMultiPlatformSearch 启动 4合1 多平台（BOSS、智联、前程无忧、猎聘）矩阵式聚合搜寻任务（兼容原有调用）
func (a *App) StartMultiPlatformSearch(projectID string, keyword string, city string, expYears int, eduLevel string, count int, platforms []string) bool {
	return a.StartQuotaMatrixSearch(projectID, keyword, city, expYears, eduLevel, count, platforms, "")
}

// StartQuotaMatrixSearch 启动全网人才拓扑与渠道配额调度矩阵检索任务
func (a *App) StartQuotaMatrixSearch(projectID string, keyword string, city string, expYears int, eduLevel string, count int, platforms []string, quotaMatrixJSON string) bool {
	return a.startQuotaMatrixSearch(projectID, keyword, city, expYears, eduLevel, count, platforms, quotaMatrixJSON, true, 0, "balanced")
}

func (a *App) StartSearchWithOptions(projectID string, keyword string, city string, expYears int, eduLevel string, count int, platforms []string, quotaMatrixJSON string, autoAnalyze bool) bool {
	return a.startQuotaMatrixSearch(projectID, keyword, city, expYears, eduLevel, count, platforms, quotaMatrixJSON, autoAnalyze, 0, "balanced")
}

// StartSearchWithPolicy 为 HR 的活跃度和渠道策略选择提供明确参数；旧入口保持兼容。
func (a *App) StartSearchWithPolicy(projectID string, keyword string, city string, expYears int, eduLevel string, count int, platforms []string, quotaMatrixJSON string, autoAnalyze bool, activityDays int, channelStrategy string) bool {
	if activityDays != 0 && activityDays != 1 && activityDays != 3 {
		activityDays = 0
	}
	if channelStrategy != "priority" {
		channelStrategy = "balanced"
	}
	return a.startQuotaMatrixSearch(projectID, keyword, city, expYears, eduLevel, count, platforms, quotaMatrixJSON, autoAnalyze, activityDays, channelStrategy)
}

func (a *App) startQuotaMatrixSearch(projectID string, keyword string, city string, expYears int, eduLevel string, count int, platforms []string, quotaMatrixJSON string, autoAnalyze bool, activityDays int, channelStrategy string) bool {
	// Keep the public Wails signature compatible with older clients, but all
	// sourcing entrances now capture only. AI requires an explicit HR action.
	autoAnalyze = false
	if !interviewIDPattern.MatchString(projectID) || a.GetProject(projectID) == nil {
		runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
			"type": "error", "message": "招聘项目不存在，请重新选择项目后搜索",
		})
		return false
	}
	a.bossMutex.Lock()
	if a.searchStarting || (a.bossCmd != nil && a.bossCmd.Process != nil) {
		a.bossMutex.Unlock()
		runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
			"type": "error", "message": "已有检索任务正在运行，请先停止或等待该任务完成",
		})
		return false
	}
	a.searchStarting = true
	a.bossMutex.Unlock()
	defer func() {
		a.bossMutex.Lock()
		a.searchStarting = false
		a.bossMutex.Unlock()
	}()

	if len(platforms) == 0 {
		platforms = []string{"boss"}
	}
	if count <= 0 {
		count = 30
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

	// 导出已收录候选人的详情链接与卡片履历，避免继续检索时重复导入。
	existingResumes := a.GetProjectResumes(projectID)
	var excludedNames []string
	var excludedUrls []string
	var excludedCards []map[string]string
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
			card := map[string]string{"name": name}
			for _, line := range strings.Split(r.Content, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "基本画像：") {
					card["infoText"] = strings.TrimPrefix(line, "基本画像：")
				} else if strings.HasPrefix(line, "任职履历快照：") {
					card["workText"] = strings.TrimPrefix(line, "任职履历快照：")
				}
			}
			if card["infoText"] == "详见卡片信息" {
				delete(card, "infoText")
			}
			if card["workText"] == "详见卡片完整信息" {
				delete(card, "workText")
			}
			if card["infoText"] != "" || card["workText"] != "" {
				excludedCards = append(excludedCards, card)
			}
		}
		// 严密排重：仅将具体的候选人独立主页加入排除，绝对不把通用搜索页加入排除集
		if r.URL != "" && !strings.Contains(r.URL, "/talent/search") && !strings.Contains(r.URL, "/search") && !strings.Contains(r.URL, "/recommend") && !strings.Contains(r.URL, "/navigate") {
			excludedUrls = append(excludedUrls, r.URL)
		}
	}

	excludeFilePath := filepath.Join(dataDir, fmt.Sprintf("exclude_%s.json", projectID))
	// 同名或相似履历不构成可信身份。只用独立主页链接跨批次排重。
	_ = excludedNames
	_ = excludedCards
	var snapshotHashes []string
	for _, resume := range existingResumes {
		if !resume.HasAttachment {
			if hash := resumeSnapshotFingerprint(resume); hash != "" {
				snapshotHashes = append(snapshotHashes, hash)
			}
		}
	}
	excludeData := map[string]interface{}{"urls": excludedUrls, "snapshotHashes": snapshotHashes}
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
		"--auto-analyze", fmt.Sprintf("%t", autoAnalyze),
		"--active-days", fmt.Sprintf("%d", activityDays),
		"--channel-strategy", channelStrategy,
	}
	stopFile := filepath.Join(dataDir, fmt.Sprintf("stop_%d.flag", time.Now().UnixNano()))
	cmdArgs = append(cmdArgs, "--stop-file", stopFile)
	if strings.TrimSpace(quotaMatrixJSON) != "" {
		cmdArgs = append(cmdArgs, "--quota-matrix", strings.TrimSpace(quotaMatrixJSON))
	}

	cmd := exec.Command("node", cmdArgs...)
	hideConsoleWindow(cmd)
	taskID, taskErr := a.createSourcingTask(SourcingTaskConfig{
		ProjectID: projectID, Keyword: keyword, City: city, ExpYears: expYears,
		EduLevel: eduLevel, Count: count, Platforms: platforms,
		QuotaMatrixJSON: quotaMatrixJSON, AutoAnalyze: autoAnalyze,
		ActivityDays: activityDays, ChannelStrategy: channelStrategy,
	})
	if taskErr != nil {
		runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{"type": "error", "message": taskErr.Error()})
		return false
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.setSourcingTaskStatus(taskID, "failed", fmt.Sprintf("启动管道失败: %v", err))
		runtime.EventsEmit(a.ctx, "boss:error", map[string]interface{}{
			"type":    "error",
			"message": fmt.Sprintf("❌ 启动直连管道失败: %v", err),
		})
		return false
	}

	if err := cmd.Start(); err != nil {
		a.setSourcingTaskStatus(taskID, "failed", fmt.Sprintf("启动失败: %v", err))
		runtime.EventsEmit(a.ctx, "boss:error", map[string]interface{}{
			"type":    "error",
			"message": fmt.Sprintf("❌ 执行直连子进程失败: %v，请确认系统已安装 Node.js 环境。", err),
		})
		return false
	}

	a.bossMutex.Lock()
	a.bossCmd = cmd
	a.bossStopFile = stopFile
	a.activeSearchTaskID = taskID
	a.bossMutex.Unlock()
	a.setSourcingTaskStatus(taskID, "running", "正在检索")

	go func() {
		persistFailures := 0
		persistedCount := 0
		deduplicatedCount := 0
		lastEngineError := ""
		var pendingDone map[string]interface{}
		scanner := bufio.NewScanner(stdout)
		// 扩容缓冲区上限至 4MB，防止全量大简历 JSON 超出默认 64KB 限制导致静默截断
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 4*1024*1024)

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
				a.updateSourcingTaskEvent(taskID, evt)
				runtime.EventsEmit(a.ctx, "boss:status", evt)
				runtime.EventsEmit(a.ctx, "platform:status", evt)
			case "auth":
				runtime.EventsEmit(a.ctx, "boss:auth", evt)
				runtime.EventsEmit(a.ctx, "platform:auth", evt)
			case "captcha":
				a.setSourcingTaskStatus(taskID, "running", "等待平台安全验证")
				runtime.EventsEmit(a.ctx, "boss:captcha", evt)
				runtime.EventsEmit(a.ctx, "platform:captcha", evt)
			case "captcha_resolved":
				runtime.EventsEmit(a.ctx, "boss:captcha_resolved", evt)
				runtime.EventsEmit(a.ctx, "platform:captcha_resolved", evt)
			case "candidate":
				if candObj, ok := evt["candidate"].(map[string]interface{}); ok {
					candID, _ := candObj["id"].(string)
					candName, _ := candObj["fileName"].(string)
					candPath, _ := candObj["filePath"].(string)
					candContent, _ := candObj["content"].(string)
					candUrl, _ := candObj["url"].(string)
					candPlatform, _ := candObj["platform"].(string)
					candPlatformID, _ := candObj["platformCandidateId"].(string)
					candEmail, _ := candObj["email"].(string)
					candSourceKeyword, _ := candObj["sourceKeyword"].(string)
					candActivityLabel, _ := candObj["activityLabel"].(string)

					r := &Resume{
						ID:                  candID,
						ProjectID:           projectID,
						FileName:            candName,
						FilePath:            candPath,
						FileType:            ".txt",
						FileSize:            int64(len(candContent)),
						Content:             candContent,
						URL:                 candUrl,
						SourcePlatform:      candPlatform,
						PlatformCandidateID: candPlatformID,
						Email:               candEmail,
						SourceKeyword:       candSourceKeyword,
						ActivityLabel:       candActivityLabel,
						Status:              "pending",
						CreatedAt:           time.Now(),
					}
					known, identityErr := a.isKnownProjectSource(projectID, r)
					if identityErr != nil {
						persistFailures++
						runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{"type": "error", "message": fmt.Sprintf("候选人身份核对失败：%v", identityErr)})
						continue
					}
					if known {
						deduplicatedCount++
						continue
					}
					if err := a.persistSearchCandidate(r); err != nil {
						persistFailures++
						runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
							"type": "error", "message": fmt.Sprintf("候选人 %s 保存失败：%v", candName, err),
						})
						continue
					}
					persistedCount++
					a.recordSourcingCandidate(taskID)

					runtime.EventsEmit(a.ctx, "resume:dropped", r)
					runtime.EventsEmit(a.ctx, "boss:candidate_found", evt)
					runtime.EventsEmit(a.ctx, "platform:candidate_found", evt)
				}
			case "done":
				pendingDone = evt
			case "error":
				if message, ok := evt["message"].(string); ok && strings.TrimSpace(message) != "" {
					lastEngineError = strings.TrimSpace(message)
				}
				runtime.EventsEmit(a.ctx, "boss:error", evt)
				runtime.EventsEmit(a.ctx, "platform:error", evt)
			}
		}

		scanErr := scanner.Err()
		waitErr := cmd.Wait()
		a.bossMutex.Lock()
		wasCurrent := a.bossCmd == cmd
		if wasCurrent {
			a.bossCmd = nil
			a.bossStopFile = ""
			a.activeSearchTaskID = ""
		}
		a.bossMutex.Unlock()
		_ = os.Remove(stopFile)
		if !wasCurrent {
			return
		}
		if scanErr != nil || waitErr != nil {
			a.setSourcingTaskStatus(taskID, "failed", fmt.Sprintf("搜索进程异常结束：%v %v", scanErr, waitErr))
			runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
				"type": "error", "message": fmt.Sprintf("搜索进程异常结束：读取错误=%v，进程错误=%v", scanErr, waitErr),
			})
		} else if persistFailures > 0 {
			a.setSourcingTaskStatus(taskID, "partial", fmt.Sprintf("%d 份档案未能保存", persistFailures))
			runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
				"type": "error", "message": fmt.Sprintf("搜索结束，但 %d 份候选人档案未能保存；请检查存储空间后重试", persistFailures),
			})
		} else if pendingDone != nil {
			if !a.finishSourcingTaskFromDone(taskID, pendingDone, persistedCount, deduplicatedCount, count) {
				runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
					"type": "error", "message": fmt.Sprintf("搜索结果与已保存简历数量不一致（已保存 %d 人），请检查本地数据", persistedCount),
				})
			} else {
				runtime.EventsEmit(a.ctx, "boss:done", pendingDone)
				runtime.EventsEmit(a.ctx, "platform:done", pendingDone)
			}
		} else {
			failureMessage := lastEngineError
			if failureMessage == "" {
				failureMessage = "搜索任务未返回完成结果"
				runtime.EventsEmit(a.ctx, "platform:error", map[string]interface{}{
					"type": "error", "message": failureMessage,
				})
			}
			a.setSourcingTaskStatus(taskID, "failed", failureMessage)
		}
	}()

	return true
}

// StopBossSearch 停止搜寻
func (a *App) StopBossSearch() bool {
	a.bossMutex.Lock()
	defer a.bossMutex.Unlock()
	if a.bossCmd != nil && a.bossCmd.Process != nil {
		if a.bossStopFile != "" {
			if err := os.WriteFile(a.bossStopFile, []byte("stop"), 0600); err == nil {
				log.Println("[StopBossSearch] 已请求多平台任务在当前操作后保存进度并停止")
				return true
			}
		}
		_ = a.bossCmd.Process.Kill()
		taskID := a.activeSearchTaskID
		a.bossCmd = nil
		a.bossStopFile = ""
		a.activeSearchTaskID = ""
		go a.setSourcingTaskStatus(taskID, "stopped", "已强制停止；已抓取结果保留")
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

// resolveCandidateRadarScore 获取候选人在指定雷达技能指标上的得分 (0-4)
func resolveCandidateRadarScore(analysis *AnalysisResult, metric string) (int, bool) {
	if analysis == nil {
		return 0, false
	}
	cleanMetric := strings.TrimSpace(metric)
	if cleanMetric == "" {
		return 0, false
	}

	var allChecks []CoreCriterionCheck
	allChecks = append(allChecks, analysis.CoreCriterionChecks...)
	allChecks = append(allChecks, analysis.ExperienceCompetencies...)

	// 1. 优先精确逐字匹配（忽略大小写与首尾空白）
	for _, item := range allChecks {
		c := strings.TrimSpace(item.Criterion)
		if strings.EqualFold(c, cleanMetric) {
			lvl := item.Level
			if lvl < 0 {
				lvl = 0
			} else if lvl > 4 {
				lvl = 4
			}
			return lvl, true
		}
	}

	// 2. 模糊双向包含匹配（处理如“Vue3”匹配“Vue3前端框架”或反向包含）
	for _, item := range allChecks {
		c := strings.TrimSpace(item.Criterion)
		if c != "" && (strings.Contains(c, cleanMetric) || strings.Contains(cleanMetric, c)) {
			lvl := item.Level
			if lvl < 0 {
				lvl = 0
			} else if lvl > 4 {
				lvl = 4
			}
			return lvl, true
		}
	}

	// 候选人已完成分析但该项未体现/未提及，按雷达模型规范确定为 0 级
	return 0, true
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

	// 提取岗位统一雷达指标（技能要求 + 核心能力）
	radarMetrics := getUnifiedJobMetrics(&p.JobConfig)
	seenMetrics := make(map[string]bool)
	for _, m := range radarMetrics {
		seenMetrics[m] = true
	}
	// 兼容历史分析：如果历史简历中存在未在当前岗位配置中列出的核心指标，自动收集到表头后方
	for _, r := range resumes {
		if r.Analysis != nil {
			for _, check := range r.Analysis.CoreCriterionChecks {
				c := strings.TrimSpace(check.Criterion)
				if c != "" && !seenMetrics[c] {
					seenMetrics[c] = true
					radarMetrics = append(radarMetrics, c)
				}
			}
		}
	}

	// 表头
	headers := []string{"排名", "姓名", "文件名", "综合分", "技能匹配", "经验匹配", "学历匹配", "推荐等级"}
	for _, m := range radarMetrics {
		headers = append(headers, fmt.Sprintf("【技能雷达】%s (0-4分)", m))
	}
	headers = append(headers, "优势", "不足", "风险", "AI初试提纲及参考回答", "总结")

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// 表头样式：蓝底白字居中加粗
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"007AFF"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetRowStyle(sheet, 1, 1, headerStyle)
	f.SetRowHeight(sheet, 1, 26)

	// 居中单元格样式（用于排名、得分及雷达评分）
	centerStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	// 多行文本换行样式（用于优势、不足、风险、面试提纲与总结）
	wrapStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})

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
			0, 0, 0, rec,
		}
		if r.Analysis != nil {
			rowData[4] = r.Analysis.SkillMatch
			rowData[5] = r.Analysis.ExperienceMatch
			rowData[6] = r.Analysis.EducationMatch
		}

		// 追加各项雷达指标得分
		for _, m := range radarMetrics {
			if r.Analysis != nil {
				score, _ := resolveCandidateRadarScore(r.Analysis, m)
				rowData = append(rowData, score)
			} else {
				rowData = append(rowData, "")
			}
		}

		// 追加文本详情
		rowData = append(rowData, strengths, weaknesses, risks, interviewQAText, summary)

		for j, val := range rowData {
			cell, _ := excelize.CoordinatesToCellName(j+1, row)
			f.SetCellValue(sheet, cell, val)

			// 排名、综合分、匹配度、推荐等级及雷达得分列居中对齐
			isCenterCol := (j == 0) || (j >= 3 && j < 8+len(radarMetrics))
			if isCenterCol && centerStyle != 0 {
				_ = f.SetCellStyle(sheet, cell, cell, centerStyle)
			} else if j >= 8+len(radarMetrics) && wrapStyle != 0 {
				// 优势、不足、风险、提纲与总结列顶部对齐并自动换行
				_ = f.SetCellStyle(sheet, cell, cell, wrapStyle)
			}
		}
	}

	// 动态设置列宽
	for colIdx, h := range headers {
		colName, err := excelize.ColumnNumberToName(colIdx + 1)
		if err != nil {
			continue
		}
		var width float64
		switch h {
		case "排名":
			width = 6
		case "姓名":
			width = 12
		case "文件名":
			width = 25
		case "综合分":
			width = 8
		case "技能匹配", "经验匹配", "学历匹配":
			width = 10
		case "推荐等级":
			width = 12
		case "优势", "不足":
			width = 30
		case "风险":
			width = 25
		case "AI初试提纲及参考回答":
			width = 50
		case "总结":
			width = 40
		default:
			// 雷达指标列根据表头文字自适应宽度，确保表头不被截断
			width = float64(len([]rune(h)))*1.5 + 4
			if width < 16 {
				width = 16
			}
		}
		f.SetColWidth(sheet, colName, colName, width)
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
	if cfg == nil {
		return false, "请先填写 AI 配置"
	}
	if cfg.APIKey == "" {
		return false, "API Key 不能为空"
	}
	if cfg.BaseURL == "" {
		return false, "Base URL 不能为空"
	}

	// GLM-5.3-Flash 始终思考；10 个输出 Token 可能全部用于思考。
	maxTokens := 10
	reasoningEffort := ""
	if isGLM53Flash(cfg.Model) {
		maxTokens = 2048
		reasoningEffort = "low"
	}
	// 构造测试请求
	reqBody := ChatRequest{
		Model: cfg.Model,
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello, this is a connection test. Please respond with 'OK'."},
		},
		MaxTokens:       maxTokens,
		ReasoningEffort: reasoningEffort,
	}

	body, _ := json.Marshal(reqBody)
	url := strings.TrimSuffix(cfg.BaseURL, "/") + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Sprintf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	timeoutSec := cfg.Timeout
	if timeoutSec < 30 {
		timeoutSec = 30
	}
	if isGLM53Flash(cfg.Model) && timeoutSec < 90 {
		timeoutSec = 90
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("连接失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusPaymentRequired {
		if isGLM53Flash(cfg.Model) {
			return false, "AI 服务账户余额不足，请在阿里云百炼处理后重试"
		}
		return false, "AI 服务账户余额不足，请在模型服务商平台处理后重试"
	}

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
	if strings.TrimSpace(chatResp.Choices[0].Message.Content) == "" {
		return false, "模型未返回可显示的回答（可能只生成了思考过程）；请重试或检查模型输出限制"
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
	a.analysisMutex.Lock()
	if a.activeAnalyses == nil {
		a.activeAnalyses = make(map[string]bool)
	}
	if a.activeAnalyses[resumeID] {
		a.analysisMutex.Unlock()
		return nil, fmt.Errorf("简历正在分析")
	}
	a.activeAnalyses[resumeID] = true
	a.analysisMutex.Unlock()
	defer func() {
		a.analysisMutex.Lock()
		delete(a.activeAnalyses, resumeID)
		a.analysisMutex.Unlock()
	}()

	// 读取简历
	path := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")
	data, err := readJSONFile(path)
	if err != nil {
		return nil, fmt.Errorf("简历不存在: %v", err)
	}

	var resume Resume
	if err := json.Unmarshal(data, &resume); err != nil {
		return nil, fmt.Errorf("解析简历失败: %v", err)
	}
	if db := a.activeRecruitmentDB(); db != nil && resume.ProjectID != "" {
		_ = db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM job_rules WHERE project_id=? AND status='approved'`,
			resume.ProjectID).Scan(&resume.AnalysisRuleVersion)
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

	// 确保所属项目已完备统一技能与核心能力指标（若不足3项则静默生成并持久化）
	if resume.ProjectID != "" {
		if updatedP := a.EnsureProjectJobMetrics(resume.ProjectID); updatedP != nil {
			jobCfg = &updatedP.JobConfig
		}
	}

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
	analysis, err := a.analyzeModelWithFormatRetry(cfg, prompt)
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
	// 提炼履历能力雷达画像与岗位评分卡核验 - 进度 85%
	runtime.EventsEmit(a.ctx, "analysis:progress", map[string]interface{}{
		"id":       resumeID,
		"status":   "analyzing",
		"progress": 85,
		"phase":    "competencies",
	})
	a.completeInitialCompetencies(analysis, &resume, jobCfg, cfg)
	a.applyJobDecision(analysis, jobCfg, &resume)

	if analysis.WaterCheck == nil {
		analysis.WaterCheck = a.generateFallbackWaterCheck(analysis)
	}
	// 推介卡在人工核验结果产生后重建，不能直接转发 AI 可能写出的“已命中加分/通过红线”。
	analysis.ManagerPitch = a.buildReviewedManagerPitch(analysis, &resume)

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
		if resume.InitialScore == 0 {
			resume.InitialScore = scoreInt
		}
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

// analyzeModelWithFormatRetry repairs predictable schema deviations locally,
// then asks the model once more only if its JSON is still unusable.
func (a *App) analyzeModelWithFormatRetry(cfg *AIConfig, prompt string) (*AnalysisResult, error) {
	result, usage, err := a.callAIWithUsage(cfg, prompt)
	if err != nil {
		return nil, err
	}
	analysis, parseErr := a.parseAnalysisResult(result)
	if parseErr == nil {
		if usage != nil {
			analysis.TokenUsage = usage
		}
		analysis.AnalysisStage = "screening"
		return analysis, nil
	}
	strictPrompt := prompt + "\n\n上一次返回内容无法解析。请重新生成完整分析，只返回合法 JSON：对象键必须使用双引号，字符串内换行必须转义，gaps、vague_claims、advise_questions 等列表必须是字符串数组。"
	result, retryUsage, err := a.callAIWithUsage(cfg, strictPrompt)
	if err != nil {
		return nil, fmt.Errorf("解析AI返回失败；格式重试请求失败: %w", err)
	}
	analysis, err = a.parseAnalysisResult(result)
	if err != nil {
		return nil, fmt.Errorf("解析AI返回失败（已自动重试 1 次）: %w", err)
	}
	if retryUsage != nil {
		analysis.TokenUsage = retryUsage
	} else if usage != nil {
		analysis.TokenUsage = usage
	}
	analysis.AnalysisStage = "screening"
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
		if total == 0 {
			runtime.EventsEmit(a.ctx, "batch:completed", map[string]interface{}{
				"total": 0,
			})
			return
		}

		// 工业级 Worker Pool 并发控制：并发度设为 3
		const maxWorkers = 3
		sem := make(chan struct{}, maxWorkers)
		var wg sync.WaitGroup
		var processedCount int32

		for _, id := range resumeIDs {
			wg.Add(1)
			sem <- struct{}{} // 申请工作槽位

			go func(resumeID string) {
				defer func() {
					<-sem // 释放工作槽位
					wg.Done()
				}()

				_, err := a.AnalyzeResume(resumeID, cfg, jobCfg)
				if err != nil {
					log.Printf("分析简历 %s 失败: %v", resumeID, err)
				}

				curr := atomic.AddInt32(&processedCount, 1)
				runtime.EventsEmit(a.ctx, "batch:progress", map[string]interface{}{
					"current":  curr,
					"total":    total,
					"resumeId": resumeID,
				})
			}(id)
		}

		// 等待所有简历分析任务完成
		wg.Wait()

		runtime.EventsEmit(a.ctx, "batch:completed", map[string]interface{}{
			"total": total,
		})
	}()
}

// getUnifiedJobMetrics 获取当前岗位的统一技能指标列表（技能要求 + 核心能力），与前端展示完全一致
func getUnifiedJobMetrics(jobCfg *JobConfig) []string {
	if jobCfg == nil {
		return []string{"专业技能掌握度", "核心业务交付", "综合解决问题能力"}
	}
	var unifiedMetrics []string
	seenMetrics := make(map[string]bool)
	for _, s := range jobCfg.RequiredSkills {
		s = strings.TrimSpace(s)
		if s != "" && !seenMetrics[s] {
			seenMetrics[s] = true
			unifiedMetrics = append(unifiedMetrics, s)
		}
	}
	for _, c := range jobCfg.CoreCriteria {
		c = strings.TrimSpace(c)
		if c != "" && !seenMetrics[c] {
			seenMetrics[c] = true
			unifiedMetrics = append(unifiedMetrics, c)
		}
	}
	if len(unifiedMetrics) < 3 {
		fallbacks := []string{"专业技能掌握度", "核心业务交付", "复杂问题分析与排查", "工程化规范与落地", "团队沟通与自驱协同"}
		for _, fb := range fallbacks {
			if !seenMetrics[fb] {
				seenMetrics[fb] = true
				unifiedMetrics = append(unifiedMetrics, fb)
				if len(unifiedMetrics) >= 5 {
					break
				}
			}
		}
	}
	return unifiedMetrics
}

// buildAnalysisPrompt 构建分析提示词
func (a *App) buildAnalysisPrompt(resume *Resume, jobCfg *JobConfig) string {
	skills := strings.Join(jobCfg.RequiredSkills, "、")
	requirements := strings.Join(jobCfg.Requirements, "\n- ")
	jobDescBlock := ""
	if strings.TrimSpace(jobCfg.JobDescription) != "" {
		jobDescBlock = fmt.Sprintf("- 岗位职责与技能要求:\n```\n%s\n```\n", jobCfg.JobDescription)
	}
	interviewQBlock := ""
	interviewQuestions := jobCfg.GetInterviewQuestions()
	if len(interviewQuestions) > 0 {
		interviewQBlock = fmt.Sprintf("- 🎯 初试重点关注要求（作为初试核查与经验溯源重点）:\n  * %s\n", strings.Join(interviewQuestions, "\n  * "))
	}
	unifiedMetrics := getUnifiedJobMetrics(jobCfg)
	tierBlock := fmt.Sprintf("- 岗位统一能力评分卡与雷达指标（共 %d 项，每项按 0 未体现、1 了解/线索、2 熟练参与、3 精通独立、4 专家主导评分）：%s\n", len(unifiedMetrics), strings.Join(unifiedMetrics, "、"))

	var expCompExamples []string
	for _, m := range unifiedMetrics {
		expCompExamples = append(expCompExamples, fmt.Sprintf(`    {"criterion": %q, "level": 0, "evidence": "若简历有匹配的实战经历，请引用连续原文；若无证据则填空字符串", "assessment": "结合事实评估该能力等级(0-4)及掌握依据"}`, m))
	}
	expCompJsonBlock := "[\n" + strings.Join(expCompExamples, ",\n") + "\n  ]"

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
`
	} else {
		resumeBlock = fmt.Sprintf("文件名: %s\n```\n%s\n```", resume.FileName, a.truncateContent(resume.Content, 10000))
	}

	reAnalyzeAnchorBlock := ""
	baseAnchor := resume.InitialScore
	if baseAnchor == 0 {
		baseAnchor = resume.Score
	}
	if baseAnchor > 0 && !resume.IsMergedAnalysis {
		reAnalyzeAnchorBlock = fmt.Sprintf(
			"### 重新分析基准锚定规定（防打分大起大落）：\n"+
				"- 该候选人历史初筛评估基准分为 **%d 分**。\n"+
				"- 在岗位要求与候选人简历材料未发生重大变动的前提下，本次重新分析的各项匹配度及 overall_score 必须严格围绕该基准进行客观细化微调，保证评分严密一致与可信度。\n\n",
			baseAnchor,
		)
	}

	userPrompt := fmt.Sprintf(
		"## 招聘岗位信息\n"+
			"- 岗位名称: %s\n"+
			"- 经验要求: %s\n"+
			"- 最低学历: %s\n"+
			"- 核心必备技能: %s\n"+
			"%s"+
			"%s"+
			"%s"+
			"- 补充要求:\n- %s\n\n"+
			"## 候选人简历材料\n"+
			"%s\n\n"+
			"%s\n\n"+
			"## 分析任务\n\n"+
			"请对候选人简历进行全方位、深度的专业分析。\n\n"+
			"%s"+
			"### 评分标准与稳定性严格规定（必须严格执行，杜绝主观任意给分）\n\n"+
			"**核心原则**：评分必须客观严谨、具备极高的确定性与可复现性。严格根据简历中能核实的客观事实给出精准区间，禁止随意给出大起大落的估计分。\n\n"+
			"**技能匹配度 (skill_match)**:\n"+
			"- 90-100: 完全掌握所有核心必备技能，且有可定位的大型项目主导实战成果\n"+
			"- 75-89: 熟练掌握绝大多数核心必备技能（>=70%%），具备独立实操与交付经验\n"+
			"- 60-74: 掌握约半数核心技能（约 50%%），部分关键技能欠缺实战经历或仅停留在了解阶段\n"+
			"- 40-59: 仅涉及少数边缘技能，核心必备技能严重不足\n"+
			"- 0-39: 完全不具备岗位要求的核心技能\n\n"+
			"**经验匹配度 (experience_match)**:\n"+
			"- 90-100: 工作年限超过要求，业务场景与项目深度高度对口\n"+
			"- 75-89: 工作年限达到要求，有良好且直接相关的项目实战经历\n"+
			"- 60-74: 工作年限基本达标，但所属行业或业务场景有一定跨度\n"+
			"- 40-59: 工作年限明显不足（缺口1-2年），或项目复杂度过低\n"+
			"- 0-39: 无相关行业或岗位工作经验\n\n"+
			"**学历匹配度 (education_match)**:\n"+
			"- 90-100: 学历高于岗位最低要求，且专业完全对口\n"+
			"- 75-89: 满足最低学历要求，专业对口或高度相关\n"+
			"- 60-74: 满足最低学历要求，但专业有一定跨度\n"+
			"- 0-59: 未达到岗位最低学历门槛要求\n\n"+
			"**综合评分逻辑 (overall_score)**：\n"+
			"overall_score 必须在逻辑上严密契合各分项得分，基准公式为：skill_match * 0.55 + experience_match * 0.35 + education_match * 0.10（四舍五入取整）。不得脱离单项评分凭主观臆断给出突变分数。\n\n"+
			"**核心能力与岗位统一技能雷达指标 (experience_competencies 与 core_criterion_checks)**：\n"+
			"必须严格围绕上述【岗位统一能力评分卡与雷达指标】逐项评估！每一项的 criterion 必须与指标名称完全一致（逐字对应，不得自创其他名称）；\n"+
			"level 为 0-4 分整数（0 未体现、1 了解/线索、2 熟练参与、3 精通独立完成、4 专家主导复盘）；evidence 必须是简历中连续且真实的原文引用（无证据时填空字符串且 level 必须为 0）；assessment 给出专业评估与核实要点。最终岗位能力分由系统按各项等级等权计算。\n\n"+
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
			"  \"core_criterion_checks\": " + expCompJsonBlock + ",\n" +
			"  \"experience_competencies\": " + expCompJsonBlock + ",\n" +
			"  \"strengths\": [\n"+
			"    \"具体的优势1（必须引用简历中的事实依据）\",\n"+
			"    \"具体的优势2\"\n"+
			"  ],\n"+
			"  \"weaknesses\": [\n"+
			"    \"具体的不足1（必须基于岗位要求指出差距）\",\n"+
			"    \"具体的不足2\"\n"+
			"  ],\n"+
			"  \"risks\": [\n"+
			"    \"潜在风险或需关注事项，如频繁跳槽、职业路径不连贯等\"\n"+
			"  ],\n"+
			"  \"interview_suggestions\": [\n"+
			"    \"如果进入初试环节，建议重点核查的问题或方向\"\n"+
			"  ],\n"+
			"  \"summary\": \"2-3句话精炼初筛总结：核心亮点、主要短板与初试核实建议。\"\n"+
			"}\n"+
			"```\n\n"+
			"注意事项：\n"+
			"1. 所有分析必须基于简历中的客观内容，不得编造简历中不存在的信息\n"+
			"2. strengths 至少2条，weaknesses 至少1条，每条都要具体且有事实依据\n"+
			"3. summary 必须精炼（2-3句话），结合候选人的具体情况给出有价值的判断\n"+
			"4. 确保返回合法的JSON格式",
		jobCfg.Title,
		jobCfg.GetExperienceRequirement(),
		jobCfg.EducationLevel,
		skills,
		jobDescBlock,
		interviewQBlock,
		tierBlock,
		requirements,
		resumeBlock,
		mergedInstruction,
		reAnalyzeAnchorBlock,
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
	content, _, err := a.callAIWithUsage(cfg, prompt)
	return content, err
}

func (a *App) callAIWithUsage(cfg *AIConfig, prompt string) (string, *AIUsage, error) {
	// 拆分 system prompt 和 user prompt
	parts := strings.SplitN(prompt, "\n\n---\n\n", 2)
	systemMsg := parts[0]
	userMsg := prompt
	if len(parts) == 2 {
		userMsg = parts[1]
	}

	messages := []ChatMessage{
		{
			Role:    "system",
			Content: systemMsg,
		},
		{
			Role:    "user",
			Content: userMsg,
		},
	}

	return a.callAIChatMessagesDetailed(cfg, messages, 0.2)
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

	// 5. 修复模型在 JSON 字符串内部输出的字面换行与控制符
	jsonStr = repairJSONLiteralNewlines(jsonStr)
	jsonStr = normalizeAnalysisListFields(jsonStr)

	var result AnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("[parseAnalysisResult] 解析失败: %v", err)
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

	// 具体岗位的红线裁决在解析完成后由 applyJobDecision 统一执行。

	// 规范化与兜底防伪注水雷达
	if result.WaterCheck != nil {
		if result.WaterCheck.WaterScore < 0 {
			result.WaterCheck.WaterScore = 0
		} else if result.WaterCheck.WaterScore > 100 {
			result.WaterCheck.WaterScore = 100
		}
		if result.WaterCheck.RiskLevel == "" {
			if result.WaterCheck.WaterScore <= 25 {
				result.WaterCheck.RiskLevel = "low"
			} else if result.WaterCheck.WaterScore <= 55 {
				result.WaterCheck.RiskLevel = "medium"
			} else {
				result.WaterCheck.RiskLevel = "high"
			}
		}
	} else {
		result.WaterCheck = a.generateFallbackWaterCheck(&result)
	}

	result.AnalyzedAt = time.Now().Format(time.RFC3339)

	return &result, nil
}

// normalizeAnalysisListFields accepts a single string where the model should
// have returned a list, without rewriting any résumé facts or malformed JSON.
func normalizeAnalysisListFields(raw string) string {
	var root map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &root) != nil {
		return raw
	}
	changed := normalizeStringListFields(root, "strengths", "weaknesses", "risks", "red_line_violations")
	if waterRaw, ok := root["water_check"]; ok && len(waterRaw) > 0 && string(waterRaw) != "null" {
		var water map[string]json.RawMessage
		if json.Unmarshal(waterRaw, &water) == nil && normalizeStringListFields(water, "gaps", "vague_claims", "advise_questions") {
			root["water_check"], _ = json.Marshal(water)
			changed = true
		}
	}
	if !changed {
		return raw
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return raw
	}
	return string(encoded)
}

func normalizeStringListFields(fields map[string]json.RawMessage, names ...string) bool {
	changed := false
	for _, name := range names {
		value, ok := fields[name]
		if !ok || len(value) == 0 || value[0] != '"' {
			continue
		}
		var text string
		if json.Unmarshal(value, &text) != nil {
			continue
		}
		items := []string{}
		if strings.TrimSpace(text) != "" {
			items = append(items, text)
		}
		fields[name], _ = json.Marshal(items)
		changed = true
	}
	return changed
}

// repairJSONLiteralNewlines 修复模型输出在 JSON 字符串内部未转义的字面换行与制表符
func repairJSONLiteralNewlines(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 64)
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				b.WriteByte(ch)
				continue
			}
			if ch == '\\' {
				escaped = true
				b.WriteByte(ch)
				continue
			}
			if ch == '"' {
				inString = false
				b.WriteByte(ch)
				continue
			}
			if ch == '\n' {
				b.WriteString(`\n`)
				continue
			}
			if ch == '\r' {
				b.WriteString(`\r`)
				continue
			}
			if ch == '\t' {
				b.WriteString(`\t`)
				continue
			}
			b.WriteByte(ch)
		} else {
			if ch == '"' {
				inString = true
			}
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// redLineRuleID 为旧版自由文本规则生成稳定标识；修改规则文本会产生新的标识。
func redLineRuleID(criterion string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(criterion)))
	return fmt.Sprintf("%x", digest[:8])
}

func scoreCandidate(result *AnalysisResult, criteria []string, initialScores ...int) {
	if result == nil {
		return
	}
	initialScore := 0
	if len(initialScores) > 0 {
		initialScore = initialScores[0]
	}

	if len(criteria) == 0 {
		if result.CoreMatch == 0 {
			result.CoreMatch = result.SkillMatch*0.55 + result.ExperienceMatch*0.45
		}
		if initialScore > 0 && result.CoreMatch < float64(initialScore) {
			result.CoreMatch = float64(initialScore)
		}
		result.AbilityScore = int(math.Round(result.CoreMatch))
		result.OverallScore = float64(result.AbilityScore)
		refreshCandidateRecommendation(result)
		return
	}

	if len(result.CoreCriterionChecks) == 0 {
		if result.CoreMatch == 0 {
			result.CoreMatch = result.SkillMatch*0.55 + result.ExperienceMatch*0.45
		}
		if initialScore > 0 && result.CoreMatch < float64(initialScore) {
			result.CoreMatch = float64(initialScore)
		}
		result.AbilityScore = int(math.Round(result.CoreMatch))
		result.OverallScore = float64(result.AbilityScore)
		result.ScoreBasis = "provisional"
		refreshCandidateRecommendation(result)
		return
	}

	wAll := len(criteria)
	wKnown := 0
	pKnown := 0

	byCrit := make(map[string]CoreCriterionCheck)
	for _, c := range result.CoreCriterionChecks {
		byCrit[strings.TrimSpace(c.Criterion)] = c
	}

	coveredCount := 0
	for _, metric := range criteria {
		m := strings.TrimSpace(metric)
		check, exists := byCrit[m]
		if !exists {
			continue
		}
		coveredCount++

		// 契约：known 必须有明确可核对依据，包括明确的低等级或否定信息，整数 0-4
		// unknown: 没有足够的原文信息或所给引用不能支持该维度
		isKnown := check.EvidenceStatus == "known"
		if check.EvidenceStatus == "" {
			if strings.TrimSpace(check.Evidence) != "" {
				isKnown = true
			} else if check.Level == 0 && strings.TrimSpace(check.Assessment) != "" && !strings.Contains(check.Assessment, "未提供") && !strings.Contains(check.Assessment, "未提及") && !strings.Contains(check.Assessment, "材料不足") {
				isKnown = true
			}
		}

		if isKnown && check.Level >= 0 && check.Level <= 4 {
			wKnown++
			pKnown += check.Level
		}
	}

	coverage := 0.0
	knownScore := 0.0
	lowerBound := 0.0
	upperBound := 100.0

	if wAll > 0 {
		coverage = float64(wKnown) / float64(wAll)
		if wKnown > 0 {
			knownScore = float64(100*pKnown) / float64(4*wKnown)
		}
		lowerBound = float64(100*pKnown) / float64(4*wAll)
		upperBound = lowerBound + float64(100*(wAll-wKnown))/float64(wAll)
	}

	rankEligible := (wKnown*100 >= wAll*80)
	result.RadarCoverage = coverage
	result.RankEligible = rankEligible
	result.ScoreLowerBound = lowerBound
	result.ScoreUpperBound = upperBound

	macroScore := 0.0
	if result.SkillMatch > 0 || result.ExperienceMatch > 0 {
		macroScore = clampFloat(result.SkillMatch*0.55+result.ExperienceMatch*0.45, 0, 100)
	}
	if initialScore > 0 && macroScore < float64(initialScore) {
		macroScore = float64(initialScore)
	}

	if macroScore > 0 {
		result.CoreMatch = math.Round(macroScore)
		if rankEligible {
			result.ScoreBasis = "rubric"
		} else {
			result.ScoreBasis = "provisional"
		}
	} else if rankEligible {
		result.CoreMatch = math.Round(knownScore)
		result.ScoreBasis = "rubric"
	} else if wKnown > 0 {
		if coveredCount < len(criteria) {
			result.CoreMatch = math.Round(lowerBound)
		} else {
			result.CoreMatch = math.Round(knownScore)
		}
		result.ScoreBasis = "provisional"
	} else {
		result.CoreMatch = 0
		result.ScoreBasis = "provisional"
	}

	if initialScore > 0 && result.CoreMatch < float64(initialScore) {
		result.CoreMatch = float64(initialScore)
	}

	result.AbilityScore = int(math.Round(result.CoreMatch))
	result.BonusScore = 0
	refreshCandidateRecommendation(result)
}

// holisticMatchScore is a display-only summary of the model's three match
// dimensions. The evidence-based ability score remains the hiring score.
func holisticMatchScore(result *AnalysisResult) int {
	if result == nil || result.SkillMatch <= 0 || result.ExperienceMatch <= 0 || result.EducationMatch <= 0 {
		return 0
	}
	return int(math.Round(clampFloat(result.SkillMatch, 0, 100)*0.4 +
		clampFloat(result.ExperienceMatch, 0, 100)*0.4 +
		clampFloat(result.EducationMatch, 0, 100)*0.2))
}

func refreshCandidateRecommendation(result *AnalysisResult) {
	result.OverallScore = float64(result.AbilityScore)
	switch {
	case result.AbilityScore < 55:
		result.Recommendation = "not_recommend"
	case result.AbilityScore < 70:
		result.Recommendation = "consider"
	case result.OverallScore >= 85:
		result.Recommendation = "strong_recommend"
	default:
		result.Recommendation = "recommend"
	}
}

func applyRedLineVerdict(result *AnalysisResult) {
	result.RedLineViolations = nil
	if len(result.RedLineChecks) == 0 {
		result.RedLineStatus = "not_configured"
		return
	}
	unknown, violated := false, false
	for _, check := range result.RedLineChecks {
		switch check.Status {
		case "violated":
			violated = true
			result.RedLineViolations = append(result.RedLineViolations, check.Criterion+"："+check.Evidence)
		case "met":
		default:
			unknown = true
		}
	}
	if violated {
		result.RedLineStatus, result.Recommendation = "failed", "not_recommend"
	} else if unknown {
		result.RedLineStatus = "pending"
		if result.Recommendation == "recommend" || result.Recommendation == "strong_recommend" {
			result.Recommendation = "consider"
		}
	} else {
		result.RedLineStatus = "passed"
	}
}

// applyJobDecision 只保存 AI 提议和原文证据；自由文本红线必须经 HR 核实后才会通过或拒绝。
func (a *App) applyJobDecision(result *AnalysisResult, job *JobConfig, resume *Resume) {
	if result == nil || job == nil {
		return
	}
	criteria := getUnifiedJobMetrics(job)
	if len(job.CoreCriteria) > 0 && len(job.RequiredSkills) == 0 {
		criteria = job.CoreCriteria
	} else if len(job.RequiredSkills) > 0 && len(job.CoreCriteria) == 0 {
		criteria = job.RequiredSkills
	}
	// 评分卡只接受简历材料中能逐字定位的依据；没有证据的高等级回落为“未体现”。
	if resume != nil {
		validateCompetencyEvidence(result.CoreCriterionChecks, resume)
		validateCompetencyEvidence(result.ExperienceCompetencies, resume)
		extra := make([]CoreCriterionCheck, 0, 6)
		for _, check := range result.ExperienceCompetencies {
			if check.Level > 0 && check.Evidence != "" && strings.TrimSpace(check.Criterion) != "" && len(extra) < 6 {
				extra = append(extra, check)
			}
		}
		result.ExperienceCompetencies = extra
	}
	initialScore := 0
	if resume != nil && resume.InitialScore > 0 {
		initialScore = resume.InitialScore
	}
	scoreCandidate(result, criteria, initialScore)
	if len(job.RedLines) > 0 || len(job.BonusPoints) > 0 {
		proposedBonus := result.BonusChecks
		checks := make([]RedLineCheck, 0, len(job.RedLines))
		for _, criterion := range job.RedLines {
			criterion = strings.TrimSpace(criterion)
			if criterion == "" {
				continue
			}
			check := RedLineCheck{RuleID: redLineRuleID(criterion), Criterion: criterion, Status: "unknown"}
			if resume != nil && resume.Analysis != nil {
				for _, previous := range resume.Analysis.RedLineChecks {
					if previous.Criterion == criterion && previous.ReviewedAt != "" {
						check = previous
						check.RuleID = redLineRuleID(criterion)
						break
					}
				}
			}
			if check.ReviewedAt != "" {
				checks = append(checks, check)
				continue
			}
			for _, proposed := range result.RedLineChecks {
				if strings.TrimSpace(proposed.Criterion) != criterion {
					continue
				}
				evidence := strings.Trim(proposed.Evidence, " \t\r\n\"“”")
				suggestion := proposed.Status
				if suggestion == "unknown" {
					suggestion = proposed.SuggestedStatus
				}
				if resume != nil && (suggestion == "met" || suggestion == "violated") && evidence != "" &&
					(strings.Contains(resume.Content, evidence) || strings.Contains(resume.AttachmentContent, evidence)) {
					check.SuggestedStatus, check.Evidence = suggestion, evidence
				}
				break
			}
			checks = append(checks, check)
		}
		result.RedLineChecks = checks
		bonusChecks := make([]BonusCheck, 0, len(job.BonusPoints))
		for _, criterion := range job.BonusPoints {
			criterion = strings.TrimSpace(criterion)
			if criterion == "" {
				continue
			}
			check := BonusCheck{RuleID: redLineRuleID(criterion), Criterion: criterion, Status: "unknown"}
			if resume != nil && resume.Analysis != nil {
				for _, previous := range resume.Analysis.BonusChecks {
					if previous.Criterion == criterion && previous.ReviewedAt != "" {
						check = previous
						break
					}
				}
			}
			if check.ReviewedAt == "" {
				for _, proposed := range proposedBonus {
					if strings.TrimSpace(proposed.Criterion) != criterion {
						continue
					}
					evidence := strings.Trim(proposed.Evidence, " \t\r\n\"“”")
					suggestion := proposed.Status
					if suggestion == "unknown" {
						suggestion = proposed.SuggestedStatus
					}
					if resume != nil && (suggestion == "met" || suggestion == "violated") && evidence != "" &&
						(strings.Contains(resume.Content, evidence) || strings.Contains(resume.AttachmentContent, evidence)) {
						check.SuggestedStatus, check.Evidence = suggestion, evidence
					}
					break
				}
			}
			bonusChecks = append(bonusChecks, check)
		}
		result.BonusChecks = bonusChecks
		refreshBonusVerdict(result)
		applyRedLineVerdict(result)
	} else {
		result.RedLineStatus = "not_configured"
		result.RedLineViolations = nil
		result.RedLineChecks = nil
		result.BonusChecks = nil
		result.ConfirmedBonusCount = 0
	}
}

func refreshBonusVerdict(result *AnalysisResult) {
	result.ConfirmedBonusCount = 0
	for _, check := range result.BonusChecks {
		if check.Status == "met" && check.ReviewedAt != "" {
			result.ConfirmedBonusCount++
		}
	}
}

// generateFallbackManagerPitch 当大模型未返回推介卡时的容错生成器
func (a *App) buildReviewedManagerPitch(res *AnalysisResult, resume *Resume) string {
	pitch := a.generateFallbackManagerPitch(res, resume)
	if res.RedLineStatus == "failed" {
		return "【HR 已核实未满足岗位必备条件】\n" + pitch
	}
	return pitch
}

func (a *App) generateFallbackManagerPitch(res *AnalysisResult, resume *Resume) string {
	candidateName := res.CandidateName
	if strings.TrimSpace(candidateName) == "" && resume != nil {
		candidateName = strings.TrimSuffix(resume.FileName, filepath.Ext(resume.FileName))
	}
	if strings.TrimSpace(candidateName) == "" {
		candidateName = "候选人"
	}

	role := res.CurrentRole
	if role == "" {
		role = "专业技术人才"
	}
	exp := res.WorkYears
	if exp == "" {
		exp = "具备工作经验"
	}
	edu := res.Education
	if edu == "" {
		edu = "学历符合"
	}

	recText := "推荐"
	switch res.Recommendation {
	case "strong_recommend":
		recText = "强力推荐"
	case "recommend":
		recText = "推荐"
	case "consider":
		recText = "建议斟酌"
	case "not_recommend":
		recText = "不推荐"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【候选人极简推介卡】\n"))
	if len(res.RedLineViolations) > 0 {
		sb.WriteString(fmt.Sprintf("🚨 触碰用人红线：%s\n", strings.Join(res.RedLineViolations, "；")))
	}
	var confirmedBonus []string
	for _, check := range res.BonusChecks {
		if check.Status == "met" && check.ReviewedAt != "" {
			confirmedBonus = append(confirmedBonus, check.Criterion)
		}
	}
	if len(confirmedBonus) > 0 {
		sb.WriteString(fmt.Sprintf("⭐ HR 已确认附加优势：%s\n", strings.Join(confirmedBonus, "；")))
	}
	sb.WriteString(fmt.Sprintf("👤 候选人：%s | 现任：%s\n", candidateName, role))
	sb.WriteString(fmt.Sprintf("📌 背景画像：%s | %s\n", exp, edu))
	sb.WriteString(fmt.Sprintf("🎯 核心能力：%d分（%s）\n", int(res.OverallScore), recText))
	sb.WriteString("✨ 核心亮点：\n")
	if len(res.Strengths) > 0 {
		for i, s := range res.Strengths {
			if i >= 3 {
				break
			}
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s))
		}
	} else {
		sb.WriteString("  - 具备岗位所需的核心技能底子与实战经验\n")
	}

	sb.WriteString("⚠️ 关注提示：\n")
	if len(res.Risks) > 0 {
		sb.WriteString(fmt.Sprintf("  - %s\n", res.Risks[0]))
	} else if len(res.Weaknesses) > 0 {
		sb.WriteString(fmt.Sprintf("  - %s\n", res.Weaknesses[0]))
	} else {
		sb.WriteString("  - 建议初试深入核实项目真实职责与实操深度\n")
	}

	if resume != nil && strings.TrimSpace(resume.URL) != "" {
		sb.WriteString(fmt.Sprintf("🔗 在线主页：%s\n", resume.URL))
	}

	return sb.String()
}

// generateFallbackWaterCheck 当大模型未返回注水雷达时的容错生成器
func (a *App) generateFallbackWaterCheck(res *AnalysisResult) *WaterCheckResult {
	score := 15
	level := "low"
	if len(res.Risks) > 0 {
		score = 35
		level = "medium"
	}
	advise := make([]string, 0)
	if len(res.Weaknesses) > 0 {
		advise = append(advise, fmt.Sprintf("请针对简历中提及的【%s】展开追问其实际解决方案与实操细节", res.Weaknesses[0]))
	}
	return &WaterCheckResult{
		WaterScore:         score,
		RiskLevel:          level,
		Gaps:               []string{"履历时间线连贯，无显著离职断层"},
		VagueClaims:        []string{"项目职责与技术方案描述详实，具备量化成果支撑"},
		OutsourcingWarning: "无外包驻场迹象",
		FrequentHopWarning: "跳槽频率在健康合理区间",
		AdviseQuestions:    advise,
	}
}

// GenerateJobSynonyms 依据岗位名称与职责描述，通过大模型（或行业内嵌知识库）智能派生同义词、英文简称及高阶/衍生拓扑词
func (a *App) GenerateJobSynonyms(jobTitle string, jobDescription string) []JobSynonymItem {
	cleanTitle := strings.TrimSpace(jobTitle)
	if cleanTitle == "" {
		return []JobSynonymItem{
			{Keyword: "开发工程师", Category: "standard", CategoryName: "标准称谓", Description: "大盘通用开发标准称谓", Ratio: 40},
			{Keyword: "技术专家", Category: "senior", CategoryName: "高阶下探", Description: "具备深厚业务落地沉淀的技术专家", Ratio: 30},
			{Keyword: "系统负责人", Category: "senior", CategoryName: "业务总揽", Description: "具备统筹管理与结果交付责任人", Ratio: 30},
		}
	}

	// 1. 若配置了 AI APIKey，优先调用大模型做高精度多维同义词派生
	if a.config.AI.APIKey != "" {
		descSample := jobDescription
		if len(descSample) > 500 {
			descSample = descSample[:500] + "..."
		}

		prompt := fmt.Sprintf(`你是一位拥有15年经验的招聘猎头专家与人才情报分析师。
在招聘网站搜索人才时，因候选人履历用词习惯不同（如外企缩写、行业不同职级称谓、职能衍生），若仅搜单一词条极易陷入“信息茧房”，漏掉市场上60%%以上的隐蔽顶尖人才。
请针对目标岗位【%s】，分析候选人在简历中常用的行业同义词、近义词、中英文简称及上下游衍生职级。
请严格输出 3~5 个高价值且在招聘平台（BOSS直聘/智联招聘/前程无忧/猎聘）实际能搜出人才的关键词，涵盖三个维度：
1. standard (标准称谓/外企简称，如: CPM, 临床项目经理)
2. senior (高职级/管理下探，如: 临床试验项目总监, 临床运营经理)
3. derivative (业务/职能衍生，如: 注册临床研究主管, 临床监查经理)

岗位职责参考（若有）：
%s

请直接返回合法 JSON 数组（不要输出 markdown 代码块以外的额外文字，各词条 ratio 比例之和应为 100）：
[
  {
    "keyword": "临床项目经理",
    "category": "standard",
    "categoryName": "标准称谓",
    "description": "国内主流药企及大盘通用称谓",
    "ratio": 40
  },
  {
    "keyword": "CPM",
    "category": "standard",
    "categoryName": "外企简称",
    "description": "跨国药企/外资CRO常用英文简称",
    "ratio": 30
  },
  {
    "keyword": "临床试验负责人",
    "category": "senior",
    "categoryName": "高阶下探",
    "description": "具备多中心临床统筹与拿证经验",
    "ratio": 30
  }
]`, cleanTitle, descSample)

		respText, err := a.callAI(&a.config.AI, prompt)
		if err == nil && respText != "" {
			// 解析 JSON 数组
			reJson := regexp.MustCompile(`\[\s*\{.*\}\s*\]`)
			matches := reJson.FindString(respText)
			targetJson := respText
			if matches != "" {
				targetJson = matches
			} else {
				start := strings.Index(respText, "[")
				end := strings.LastIndex(respText, "]")
				if start != -1 && end != -1 && end > start {
					targetJson = respText[start : end+1]
				}
			}

			var items []JobSynonymItem
			if jsonErr := json.Unmarshal([]byte(targetJson), &items); jsonErr == nil && len(items) > 0 {
				log.Printf("[GenerateJobSynonyms] AI 智能拓词成功派生 %d 个词条", len(items))
				return items
			}
		}
		log.Printf("[GenerateJobSynonyms] AI 接口调用或解析未命中，无缝回退至行业专家词库规则")
	}

	// 2. Fallback: 本地内置行业专家拓扑词库
	return a.getFallbackJobSynonyms(cleanTitle)
}

// getFallbackJobSynonyms 本地内置行业专家同义词与拓扑维度矩阵
func (a *App) getFallbackJobSynonyms(title string) []JobSynonymItem {
	t := strings.ToLower(title)

	// 临床 / 医疗 / IVD
	if strings.Contains(t, "临床项目经理") || strings.Contains(t, "cpm") {
		return []JobSynonymItem{
			{Keyword: "临床项目经理", Category: "standard", CategoryName: "标准称谓", Description: "国内主流药企及大盘通用标准称谓", Ratio: 40},
			{Keyword: "CPM", Category: "standard", CategoryName: "外企简称", Description: "跨国药企/外资CRO常用英文简称", Ratio: 30},
			{Keyword: "临床运营主管", Category: "derivative", CategoryName: "业务衍生", Description: "侧重临床试验现场运营与质控交付", Ratio: 15},
			{Keyword: "临床试验负责人", Category: "senior", CategoryName: "高阶下探", Description: "具备大型临床试验全流程申报把控经验", Ratio: 15},
		}
	}

	if strings.Contains(t, "cra") || strings.Contains(t, "临床监查") {
		return []JobSynonymItem{
			{Keyword: "CRA", Category: "standard", CategoryName: "英文缩写", Description: "临床监查员行业常用缩写", Ratio: 40},
			{Keyword: "临床监查员", Category: "standard", CategoryName: "标准称谓", Description: "主流药企与CRO通用全称", Ratio: 30},
			{Keyword: "SCRA", Category: "senior", CategoryName: "高阶探针", Description: "高级/资深临床监查员", Ratio: 15},
			{Keyword: "临床研究协调员", Category: "derivative", CategoryName: "业务衍生", Description: "CRC及临床协作执行人才", Ratio: 15},
		}
	}

	if strings.Contains(t, "ivd") || strings.Contains(t, "体外诊断") || strings.Contains(t, "试剂") {
		return []JobSynonymItem{
			{Keyword: "体外诊断研发", Category: "standard", CategoryName: "标准称谓", Description: "IVD试剂与仪器研发大盘通用词", Ratio: 40},
			{Keyword: "IVD研发工程师", Category: "standard", CategoryName: "行业惯称", Description: "知名IVD上市企业标准岗位", Ratio: 30},
			{Keyword: "化学发光研发", Category: "derivative", CategoryName: "技术细分", Description: "主流免疫诊断主流发光技术专家", Ratio: 15},
			{Keyword: "体外诊断试剂技术负责人", Category: "senior", CategoryName: "高阶下探", Description: "主导过三类注册证报批的技术负责人", Ratio: 15},
		}
	}

	// Go / Golang
	if strings.Contains(t, "go") || strings.Contains(t, "golang") {
		return []JobSynonymItem{
			{Keyword: "Go开发工程师", Category: "standard", CategoryName: "标准称谓", Description: "主流Go后端研发大盘求职者", Ratio: 40},
			{Keyword: "Golang后端开发", Category: "standard", CategoryName: "极客称谓", Description: "互联网科技公司与大厂常用词", Ratio: 30},
			{Keyword: "后端架构师", Category: "senior", CategoryName: "高阶下探", Description: "具备高并发分布式中台架构经验", Ratio: 15},
			{Keyword: "分布式系统研发专家", Category: "derivative", CategoryName: "业务衍生", Description: "深入底层网络、存储与微服务架构", Ratio: 15},
		}
	}

	// Java
	if strings.Contains(t, "java") {
		return []JobSynonymItem{
			{Keyword: "Java开发工程师", Category: "standard", CategoryName: "标准称谓", Description: "主流企业Java服务端开发人员", Ratio: 40},
			{Keyword: "Java架构师", Category: "senior", CategoryName: "高阶下探", Description: "具备微服务分布式大型项目经验", Ratio: 30},
			{Keyword: "后端技术专家", Category: "senior", CategoryName: "深度探针", Description: "一线大厂及独角兽常用职级", Ratio: 15},
			{Keyword: "全栈开发工程师", Category: "derivative", CategoryName: "业务衍生", Description: "具备前后端全链路交付能力", Ratio: 15},
		}
	}

	// 前端
	if strings.Contains(t, "前端") || strings.Contains(t, "web") || strings.Contains(t, "vue") || strings.Contains(t, "react") {
		return []JobSynonymItem{
			{Keyword: "前端开发工程师", Category: "standard", CategoryName: "标准称谓", Description: "Web与现代跨端开发通用称谓", Ratio: 40},
			{Keyword: "Web前端专家", Category: "standard", CategoryName: "资深称谓", Description: "精通Vue/React工程化框架演进", Ratio: 30},
			{Keyword: "前端架构师", Category: "senior", CategoryName: "高阶下探", Description: "统筹前端技术栈与微前端体系", Ratio: 15},
			{Keyword: "全栈工程师", Category: "derivative", CategoryName: "业务衍生", Description: "兼备Node.js与服务端全栈交付能力", Ratio: 15},
		}
	}

	// 算法 / AI
	if strings.Contains(t, "算法") || strings.Contains(t, "ai") || strings.Contains(t, "大模型") {
		return []JobSynonymItem{
			{Keyword: "算法工程师", Category: "standard", CategoryName: "标准称谓", Description: "机器学习与深度学习通用称谓", Ratio: 40},
			{Keyword: "大模型算法研究员", Category: "senior", CategoryName: "前沿探针", Description: "专注于LLM微调、Agent与RAG工程", Ratio: 30},
			{Keyword: "AI应用开发工程师", Category: "derivative", CategoryName: "业务衍生", Description: "侧重大模型商业化应用场景落地", Ratio: 15},
			{Keyword: "AI技术负责人", Category: "senior", CategoryName: "高阶下探", Description: "具备算法团队管理与业务赋能经验", Ratio: 15},
		}
	}

	// 产品经理
	if strings.Contains(t, "产品") || strings.Contains(t, "pm") {
		return []JobSynonymItem{
			{Keyword: "产品经理", Category: "standard", CategoryName: "标准称谓", Description: "通用互联网与软件产品经理", Ratio: 40},
			{Keyword: "高级产品专家", Category: "senior", CategoryName: "高阶下探", Description: "主导业务线0到1或大型商业化产品", Ratio: 30},
			{Keyword: "产品负责人", Category: "senior", CategoryName: "业务总揽", Description: "统筹整条产品线规划与交付", Ratio: 15},
			{Keyword: "业务分析师", Category: "derivative", CategoryName: "业务衍生", Description: "侧重业务需求深度拆解与数据驱动", Ratio: 15},
		}
	}

	// 通用兜底
	return []JobSynonymItem{
		{Keyword: title, Category: "standard", CategoryName: "标准称谓", Description: "当前岗位大盘基础称谓", Ratio: 40},
		{Keyword: title + "专家", Category: "senior", CategoryName: "高阶下探", Description: "具备深厚业务落地沉淀的技术业务专家", Ratio: 30},
		{Keyword: title + "负责人", Category: "senior", CategoryName: "业务总揽", Description: "具备统筹管理与结果交付责任人", Ratio: 15},
		{Keyword: "资深" + title, Category: "derivative", CategoryName: "业务衍生", Description: "具备多年一线攻坚与实战经验", Ratio: 15},
	}
}

type JobMetricsDraft struct {
	RequiredSkills []string `json:"required_skills"`
	CoreCriteria   []string `json:"core_criteria"`
}

// EnsureProjectJobMetrics 检查并确保项目拥有充足的岗位技能与核心能力指标（若不足3项则自动调用AI或行业知识库补齐并持久化）
func (a *App) EnsureProjectJobMetrics(projectID string) *Project {
	p := a.GetProject(projectID)
	if p == nil {
		return nil
	}
	if p.JobConfig.Title == "" {
		p.JobConfig.Title = p.Name
	}

	// 统计现有去重指标数
	seen := make(map[string]bool)
	for _, s := range p.JobConfig.RequiredSkills {
		s = strings.TrimSpace(s)
		if s != "" {
			seen[s] = true
		}
	}
	for _, c := range p.JobConfig.CoreCriteria {
		c = strings.TrimSpace(c)
		if c != "" {
			seen[c] = true
		}
	}

	// 如果已有 3 项以上，完全尊重既有设置，无需再生成
	if len(seen) >= 3 {
		return p
	}

	jobTitle := strings.TrimSpace(p.JobConfig.Title)
	if jobTitle == "" {
		jobTitle = strings.TrimSpace(p.Name)
	}
	if jobTitle == "" {
		jobTitle = "通用专业岗位"
	}

	jobDesc := strings.TrimSpace(p.JobConfig.JobDescription)
	log.Printf("[EnsureProjectJobMetrics] 项目 %s (%s) 技能指标不足(当前%d项)，开始由AI自动推导生成...", p.ID, jobTitle, len(seen))

	var generated JobMetricsDraft
	aiSuccess := false

	// 1. 若配置了 AI APIKey，优先请求大模型生成定制化画像
	if a.config.AI.APIKey != "" {
		descSnippet := jobDesc
		runes := []rune(descSnippet)
		if len(runes) > 600 {
			descSnippet = string(runes[:600]) + "..."
		}

		prompt := fmt.Sprintf(`你是一位拥有15年招聘与人才测评经验的资深人力资源架构专家。
当前正在评估目标岗位【%s】的候选人简历，但该岗位尚未配置详细的专业技能与核心能力要求。
为了确保所有候选人能在一套科学严谨、统一公允的标尺（多边形能力雷达图）上进行横向对比，请你为该岗位提炼出专属的统一技能与能力维度。

岗位职责参考（若有）：
%s

要求：
1. required_skills: 提取 3~4 项该岗位最关键的专业硬技能、核心技术工具或业务实操能力（名称简练，每个2~8个字，如“Go并发架构”、“MySQL/SQL调优”）；
2. core_criteria: 提取 2~3 项该岗位核心业务交付、复杂问题排查或团队协同综合素养（名称简练，每个4~10个字，如“业务建模与系统抽象”、“高可用容灾与性能排障”）；
3. 严格输出合法 JSON 对象，严禁包裹 markdown 代码块外的其他文字：
{
  "required_skills": ["技能1", "技能2", "技能3", "技能4"],
  "core_criteria": ["核心能力1", "核心能力2", "核心能力3"]
}`, jobTitle, descSnippet)

		respText, err := a.callAI(&a.config.AI, prompt)
		if err == nil && respText != "" {
			reJson := regexp.MustCompile(`\{\s*"required_skills"[\s\S]*\}`)
			matches := reJson.FindString(respText)
			targetJson := respText
			if matches != "" {
				targetJson = matches
			} else {
				start := strings.Index(respText, "{")
				end := strings.LastIndex(respText, "}")
				if start != -1 && end != -1 && end > start {
					targetJson = respText[start : end+1]
				}
			}

			var draft JobMetricsDraft
			if jsonErr := json.Unmarshal([]byte(targetJson), &draft); jsonErr == nil {
				if len(draft.RequiredSkills)+len(draft.CoreCriteria) >= 3 {
					generated = draft
					aiSuccess = true
					log.Printf("[EnsureProjectJobMetrics] AI 成功为岗位【%s】派生 %d 项硬技能与 %d 项核心能力", jobTitle, len(draft.RequiredSkills), len(draft.CoreCriteria))
				}
			}
		}
	}

	// 2. 若 AI 未能生成或离线异常，无缝启用行业标准专家画像库兜底
	if !aiSuccess {
		generated = a.getFallbackJobMetrics(jobTitle)
		log.Printf("[EnsureProjectJobMetrics] 采用行业标准画像为岗位【%s】兜底补齐 %d 项指标", jobTitle, len(generated.RequiredSkills)+len(generated.CoreCriteria))
	}

	// 3. 将生成的指标无缝融入当前项目岗位配置（去重合并）
	for _, s := range generated.RequiredSkills {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			p.JobConfig.RequiredSkills = append(p.JobConfig.RequiredSkills, s)
		}
	}
	for _, c := range generated.CoreCriteria {
		c = strings.TrimSpace(c)
		if c != "" && !seen[c] {
			seen[c] = true
			p.JobConfig.CoreCriteria = append(p.JobConfig.CoreCriteria, c)
		}
	}

	// 持久化保存到项目 JSON
	a.saveProject(p)
	log.Printf("[EnsureProjectJobMetrics] 项目 %s (%s) 技能指标持久化保存成功，合计 %d 项统一指标", p.ID, jobTitle, len(seen))
	return p
}

// getFallbackJobMetrics 行业标准岗位画像兜底矩阵（涵盖主流技术、产品、运营、医药、职能等领域）
func (a *App) getFallbackJobMetrics(jobTitle string) JobMetricsDraft {
	t := strings.ToLower(jobTitle)

	// 测试 / QA / 质量工程 / 测试开发（优先于通用开发匹配，避免测试开发工程师被误判）
	if strings.Contains(t, "测试") || strings.Contains(t, "qa") || strings.Contains(t, "质检") || strings.Contains(t, "测开") || strings.Contains(t, "test") {
		return JobMetricsDraft{
			RequiredSkills: []string{"功能与业务测试用例设计", "接口测试与抓包排查", "自动化测试脚本编写", "性能与压力测试评估"},
			CoreCriteria:   []string{"缺陷敏锐定位与复盘", "质量风险阻断与把控", "研测流程协同推进"},
		}
	}

	// 前端 / Web / 客户端
	if strings.Contains(t, "前端") || strings.Contains(t, "web") || strings.Contains(t, "javascript") || strings.Contains(t, "vue") || strings.Contains(t, "react") || strings.Contains(t, "ios") || strings.Contains(t, "android") {
		return JobMetricsDraft{
			RequiredSkills: []string{"JavaScript/TypeScript", "主流前端框架(Vue/React)", "组件化与工程化构建", "页面性能与跨端适配"},
			CoreCriteria:   []string{"复杂交互业务落地", "代码规范与调优", "前后端协同与交付"},
		}
	}

	// 后端 / Go / Java / Python / C++ / PHP
	if strings.Contains(t, "后端") || strings.Contains(t, "开发") || strings.Contains(t, "架构") || strings.Contains(t, "java") || strings.Contains(t, "go") || strings.Contains(t, "golang") || strings.Contains(t, "python") || strings.Contains(t, "c++") || strings.Contains(t, "c#") || strings.Contains(t, "php") {
		return JobMetricsDraft{
			RequiredSkills: []string{"核心编程语言与框架", "关系型数据库与SQL调优", "Redis/中间件与缓存", "高并发与分布式微服务"},
			CoreCriteria:   []string{"业务抽象与系统建模", "生产故障排障与高可用", "工程化规范与交付结果"},
		}
	}

	// 运维 / DevOps / 云原生
	if strings.Contains(t, "运维") || strings.Contains(t, "devops") || strings.Contains(t, "sre") || strings.Contains(t, "云原生") || strings.Contains(t, "网络") {
		return JobMetricsDraft{
			RequiredSkills: []string{"Linux系统与脚本运维", "Docker与K8s容器化编排", "CI/CD持续集成流水线", "监控报警与链路排查"},
			CoreCriteria:   []string{"生产突发故障快速响应", "系统容灾与高可用保障", "资源成本优化与安全合规"},
		}
	}

	// 数据 / 算法 / AI
	if strings.Contains(t, "算法") || strings.Contains(t, "ai") || strings.Contains(t, "数据") || strings.Contains(t, "模型") || strings.Contains(t, "nlp") || strings.Contains(t, "cv") {
		return JobMetricsDraft{
			RequiredSkills: []string{"Python/SQL数据处理", "机器学习/深度学习框架", "特征工程与算法建模", "模型工程化部署与调优"},
			CoreCriteria:   []string{"业务问题算法转化", "指标评测与持续迭代", "跨业务协同赋能"},
		}
	}

	// 临床 / 医疗 / IVD / CRA / CPM
	if strings.Contains(t, "临床") || strings.Contains(t, "cra") || strings.Contains(t, "cpm") || strings.Contains(t, "crc") || strings.Contains(t, "医学") || strings.Contains(t, "药") || strings.Contains(t, "ivd") || strings.Contains(t, "试剂") {
		return JobMetricsDraft{
			RequiredSkills: []string{"临床试验方案与SOP执行", "GCP法规与合规质控", "试验中心监查与进度把控", "临床数据核查与闭环管理"},
			CoreCriteria:   []string{"研究中心沟通协调", "风险预警与应急处置", "多中心里程碑高质量交付"},
		}
	}

	// 产品经理 / PM
	if strings.Contains(t, "产品") || strings.Contains(t, "pm") || strings.Contains(t, "需求") {
		return JobMetricsDraft{
			RequiredSkills: []string{"业务需求分析与PRD撰写", "用户调研与原型设计", "数据分析与业务指标拆解", "业务流程与功能规划"},
			CoreCriteria:   []string{"商业逻辑抽象与洞察", "跨部门资源推动与协调", "产品全生命周期高质量迭代"},
		}
	}

	// 运营 / 市场 / 策划
	if strings.Contains(t, "运营") || strings.Contains(t, "市场") || strings.Contains(t, "策划") || strings.Contains(t, "增长") || strings.Contains(t, "媒介") {
		return JobMetricsDraft{
			RequiredSkills: []string{"活动策划与文案撰写", "用户增长与转化漏斗分析", "多渠道分发与传播推广", "数据复盘与精细化运营"},
			CoreCriteria:   []string{"市场敏锐度与创意洞察", "跨职能资源协同调度", "核心ROI与运营目标交付"},
		}
	}

	// 销售 / 商务 / BD
	if strings.Contains(t, "销售") || strings.Contains(t, "商务") || strings.Contains(t, "bd") || strings.Contains(t, "招商") || strings.Contains(t, "客户经理") {
		return JobMetricsDraft{
			RequiredSkills: []string{"客户画像挖掘与线索开拓", "商务谈判与合同促成", "客户关系维护与复购挖掘", "行业市场分析与竞品研究"},
			CoreCriteria:   []string{"商机嗅觉与抗压破局", "客户信任建立与维系", "业绩指标达成与兑现"},
		}
	}

	// 人事 / 行政 / 财务 / 职能
	if strings.Contains(t, "人事") || strings.Contains(t, "hr") || strings.Contains(t, "招聘") || strings.Contains(t, "财务") || strings.Contains(t, "会计") || strings.Contains(t, "行政") {
		return JobMetricsDraft{
			RequiredSkills: []string{"专业业务实操与标准规范", "合规风控与制度落地", "数据核算与报表汇总分析", "办公协同系统高效应用"},
			CoreCriteria:   []string{"严谨细致与职业素养", "跨部门沟通与组织服务", "流程优化与降本增效"},
		}
	}

	// 通用岗位兜底
	return JobMetricsDraft{
		RequiredSkills: []string{"岗位核心专业技能", "专业工具与系统应用", "行业专业知识与实操规范", "业务流程熟练执行"},
		CoreCriteria:   []string{"业务分析与问题解决", "高质量交付与自驱推进", "团队协同与顺畅沟通"},
	}
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

// findBrowserExecutable 查找系统 Edge 或 Chrome 可执行文件路径
func (a *App) findBrowserExecutable() string {
	candidates := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Edge SxS\Application\msedge.exe`),
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
	}
	for _, p := range candidates {
		if p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// OpenURL 优先复用已登录的招聘专用浏览器；HR 主动查看候选人时让窗口保持可见。
func (a *App) OpenURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("候选人主页网址为空")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return fmt.Errorf("只能打开有效的 HTTP/HTTPS 网址")
	}
	host := strings.ToLower(parsed.Hostname())
	isHost := func(domain string) bool { return host == domain || strings.HasSuffix(host, "."+domain) }

	// 1. 根据 URL 域名精准映射到各平台调试端口与数据目录
	port := 0
	platformKey := ""
	profileDirName := ""
	if isHost("51job.com") {
		port = 9503
		platformKey = "51job"
		profileDirName = "51job_isolated_profile"
	} else if isHost("zhipin.com") {
		port = 9501
		platformKey = "boss"
		profileDirName = "boss_isolated_profile"
	} else if isHost("zhaopin.com") {
		port = 9502
		platformKey = "zhaopin"
		profileDirName = "zhaopin_isolated_profile"
	} else if isHost("liepin.com") {
		port = 9504
		platformKey = "liepin"
		profileDirName = "liepin_isolated_profile"
	}

	// 2. 若属于招聘平台，优先尝试通过 CDP 协议在已有已登录的专用浏览器（图二）中新开 Tab
	if port > 0 {
		markerDir := filepath.Join(a.getDataDir(), "candidates_multi")
		if err := os.MkdirAll(markerDir, 0755); err != nil {
			return fmt.Errorf("无法准备浏览器查看状态：%w", err)
		}
		if err := os.WriteFile(filepath.Join(markerDir, "manual_view_"+platformKey+".flag"), []byte("visible"), 0600); err != nil {
			return fmt.Errorf("无法记录浏览器查看状态：%w", err)
		}
		client := &http.Client{Timeout: 2 * time.Second}
		newTabURL := fmt.Sprintf("http://127.0.0.1:%d/json/new?%s", port, url.QueryEscape(rawURL))
		req, err := http.NewRequest(http.MethodPut, newTabURL, nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
			}
			if err == nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated) {
				var tabInfo struct {
					ID string `json:"id"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&tabInfo); err == nil && tabInfo.ID != "" {
					// /json/activate 只切换标签；最小化的窗口还需 Browser.setWindowBounds 恢复。
					activateURL := fmt.Sprintf("http://127.0.0.1:%d/json/activate/%s", port, tabInfo.ID)
					if activated, activateErr := client.Get(activateURL); activateErr == nil {
						_ = activated.Body.Close()
					}
					scriptCandidates := []string{
						filepath.Join("scripts", "show_candidate_window.js"),
						filepath.Join(filepath.Dir(os.Args[0]), "scripts", "show_candidate_window.js"),
						filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "show_candidate_window.js"),
					}
					for _, scriptPath := range scriptCandidates {
						if _, statErr := os.Stat(scriptPath); statErr != nil {
							continue
						}
						ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
						cmd := exec.CommandContext(ctx, "node", scriptPath, "--port", fmt.Sprintf("%d", port), "--target", tabInfo.ID)
						hideConsoleWindow(cmd)
						output, restoreErr := cmd.CombinedOutput()
						cancel()
						if restoreErr != nil {
							return fmt.Errorf("候选人标签页已打开，但恢复专用浏览器窗口失败：%s", strings.TrimSpace(string(output)))
						}
						log.Printf("[OpenURL] 已恢复 %s 专用浏览器窗口", platformKey)
						return nil
					}
					return fmt.Errorf("缺少浏览器窗口恢复脚本，请检查 scripts 目录")
				}
			}
		}

		// 3. 若当前未在运行（端口未连通），使用该平台的专属 profile 目录拉起专用 Edge/Chrome 浏览器（保留已登录 Cookie）
		profileDir := filepath.Join(a.getDataDir(), "candidates_multi", profileDirName)
		browserPath := a.findBrowserExecutable()
		if browserPath != "" && profileDirName != "" {
			_ = os.MkdirAll(profileDir, 0755)
			cmd := exec.Command(browserPath,
				fmt.Sprintf("--user-data-dir=%s", profileDir),
				fmt.Sprintf("--remote-debugging-port=%d", port),
				"--no-first-run",
				"--no-default-browser-check",
				rawURL,
			)
			hideConsoleWindow(cmd)
			if err := cmd.Start(); err == nil {
				log.Printf("[OpenURL] 已启动 %s 专用浏览器窗口", platformKey)
				return nil
			}
		}
		return fmt.Errorf("无法打开 %s 专用浏览器，请检查 Edge/Chrome 与登录窗口", platformKey)
	}

	// 4. 普通链接（如更新检测、外链）或保底兜底：调用系统默认浏览器打开
	runtime.BrowserOpenURL(a.ctx, rawURL)
	return nil
}

// ActivatePlatformBrowser 唤醒指定平台的 Edge 浏览器窗口至前台（处理验证码或查看页面）
func (a *App) ActivatePlatformBrowser(platform string) bool {
	portMap := map[string]int{
		"boss":    9501,
		"zhaopin": 9502,
		"51job":   9503,
		"liepin":  9504,
	}
	port, ok := portMap[platform]
	if !ok {
		return false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	listURL := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	resp, err := client.Get(listURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var tabs []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tabs); err == nil {
		for _, tab := range tabs {
			if (tab.Type == "page" || tab.Type == "") && tab.ID != "" {
				activateURL := fmt.Sprintf("http://127.0.0.1:%d/json/activate/%s", port, tab.ID)
				_, _ = client.Get(activateURL)
				return true
			}
		}
	}
	return false
}

// EnsurePlatformBrowser 确保指定平台（51job, boss, zhaopin, liepin）的专用独立浏览器已启动；若新启动则自动最小化
func (a *App) EnsurePlatformBrowser(platform string) bool {
	platform = strings.ToLower(strings.TrimSpace(platform))
	portMap := map[string]int{
		"boss":    9501,
		"zhaopin": 9502,
		"51job":   9503,
		"liepin":  9504,
	}
	urlMap := map[string]string{
		"boss":    "https://www.zhipin.com/web/geek/job",
		"zhaopin": "https://rd6.zhaopin.com",
		"51job":   "https://ehire.51job.com",
		"liepin":  "https://lpt.liepin.com",
	}
	profileMap := map[string]string{
		"boss":    "boss_isolated_profile",
		"zhaopin": "zhaopin_isolated_profile",
		"51job":   "51job_isolated_profile",
		"liepin":  "liepin_isolated_profile",
	}

	port, ok := portMap[platform]
	if !ok {
		return false
	}
	targetURL := urlMap[platform]
	profileDirName := profileMap[platform]

	// 1. 检查端口是否已经在运行
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err == nil {
		_ = resp.Body.Close()
		// 已经在运行中，直接复用，不重新激活或弹窗打扰用户
		return true
	}

	// 2. 未在运行，使用专用 profile 启动独立 Edge/Chrome，加入 --start-minimized
	profileDir := filepath.Join(a.getDataDir(), "candidates_multi", profileDirName)
	browserPath := a.findBrowserExecutable()
	if browserPath == "" {
		return false
	}
	_ = os.MkdirAll(profileDir, 0755)
	cmd := exec.Command(browserPath,
		fmt.Sprintf("--user-data-dir=%s", profileDir),
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-first-run",
		"--no-default-browser-check",
		"--start-minimized",
		targetURL,
	)
	hideConsoleWindow(cmd)
	if err := cmd.Start(); err != nil {
		log.Printf("[EnsurePlatformBrowser] 启动 %s 专用浏览器失败: %v", platform, err)
		return false
	}

	// 3. 轮询等待端口就绪（最多等待 3 秒），就绪后立刻自动执行窗口最小化
	for i := 0; i < 15; i++ {
		time.Sleep(200 * time.Millisecond)
		checkResp, checkErr := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
		if checkErr == nil {
			_ = checkResp.Body.Close()
			// 打开后自动再最小化
			a.MinimizePlatformBrowser(platform)
			return true
		}
	}
	return true
}

// MinimizePlatformBrowser 将指定平台（51job, boss, zhaopin, liepin）的专用独立浏览器窗口最小化
func (a *App) MinimizePlatformBrowser(platform string) bool {
	platform = strings.ToLower(strings.TrimSpace(platform))
	portMap := map[string]int{
		"boss":    9501,
		"zhaopin": 9502,
		"51job":   9503,
		"liepin":  9504,
	}
	port, ok := portMap[platform]
	if !ok {
		return false
	}

	scriptCandidates := []string{
		filepath.Join("scripts", "show_candidate_window.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "scripts", "show_candidate_window.js"),
		filepath.Join(filepath.Dir(os.Args[0]), "..", "..", "scripts", "show_candidate_window.js"),
	}
	for _, scriptPath := range scriptCandidates {
		if _, statErr := os.Stat(scriptPath); statErr != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "node", scriptPath, "--port", fmt.Sprintf("%d", port), "--state", "minimized")
		hideConsoleWindow(cmd)
		_ = cmd.Run()
		cancel()
		log.Printf("[MinimizePlatformBrowser] 已请求最小化 %s 专用浏览器 (端口 %d)", platform, port)
		return true
	}
	return false
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
func (a *App) logError(_ string, _ string, _ string, _ string) {
	logDir := filepath.Join(a.getDataDir(), "logs")
	os.MkdirAll(logDir, 0700)
	logFile := filepath.Join(logDir, fmt.Sprintf("error_%s.log", time.Now().Format("20060102")))
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		defer f.Close()
		_ = os.Chmod(logFile, 0600)
		logEntry := fmt.Sprintf("【时间】%s 【AI处理】失败；候选人资料与模型原文未写入日志\n",
			time.Now().Format("2006-01-02 15:04:05"))
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
