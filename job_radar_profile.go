package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// JobRadarDraft is a proposal. Generating one never changes a project or an evaluation.
type JobRadarDraft struct {
	ProjectID  string   `json:"projectId"`
	SourceHash string   `json:"sourceHash"`
	Metrics    []string `json:"metrics"`
	Origin     string   `json:"origin"`
	Note       string   `json:"note"`
}

func jobRadarSourceHash(job *JobConfig) string {
	if job == nil {
		return ""
	}
	parts := []interface{}{strings.TrimSpace(job.Title), strings.TrimSpace(job.JobDescription), job.RequiredSkills, job.CoreCriteria}
	if direction := strings.TrimSpace(job.Direction); direction != "" {
		parts = append(parts, direction)
	}
	input, _ := json.Marshal(parts)
	sum := sha256.Sum256(input)
	return fmt.Sprintf("source-v1-%x", sum[:12])
}

func confirmedJobRadarHash(metrics []string, sourceHash string) string {
	sum := sha256.Sum256([]byte(strings.Join(uniqueJobMetrics(metrics), "\x00") + "\x01" + sourceHash))
	return fmt.Sprintf("radar-profile-v1-%x", sum[:12])
}

func activeJobRadarHash(job *JobConfig) string {
	if job != nil && job.RadarProfileHash != "" {
		return job.RadarProfileHash
	}
	return jobMetricsHash(getUnifiedJobMetrics(job))
}

// configuredJobRadarMetrics reads the current job requirements instead of a
// previously fixed radar. A changed job must not silently reuse old axes.
func configuredJobRadarMetrics(job *JobConfig) []string {
	if job == nil {
		return nil
	}
	items := append([]string(nil), job.RequiredSkills...)
	items = append(items, job.CoreCriteria...)
	return uniqueJobMetrics(items)
}

// ensureBatchJobRadar fixes one job-wide rubric before any resume worker starts.
// The batch then uses this exact version; one or two relevant axes are valid.
func (a *App) ensureBatchJobRadar(projectID string, cfg *AIConfig) (*Project, error) {
	p := a.GetProject(projectID)
	if p == nil {
		return nil, fmt.Errorf("招聘项目不存在")
	}
	job := &p.JobConfig
	sourceHash := jobRadarSourceHash(job)
	if len(job.RadarMetrics) > 0 && job.RadarProfileHash != "" && job.RadarSourceHash == sourceHash {
		return p, nil
	}
	metrics := configuredJobRadarMetrics(job)
	if len(metrics) == 0 {
		var err error
		metrics, err = a.inferBatchJobRadar(job, cfg)
		if err != nil {
			return nil, err
		}
	} else if err := a.checkBatchRadarRelevance(job, metrics, cfg); err != nil {
		return nil, err
	}
	// AI checking can take time. Never save a profile against a newer job edit.
	latest := a.GetProject(projectID)
	if latest == nil || jobRadarSourceHash(&latest.JobConfig) != sourceHash {
		return nil, fmt.Errorf("岗位资料在分析准备期间已变化，请重新点击分析")
	}
	newHash := confirmedJobRadarHash(metrics, sourceHash)
	if (latest.JobConfig.RadarProfileHash != "" && latest.JobConfig.RadarProfileHash != newHash) || latest.JobConfig.RadarProfileVersion == 0 {
		latest.JobConfig.RadarProfileVersion++
	}
	latest.JobConfig.RadarMetrics = metrics
	latest.JobConfig.RadarProfileHash = newHash
	latest.JobConfig.RadarSourceHash = sourceHash
	a.saveProject(latest)
	return latest, nil
}

func (a *App) inferBatchJobRadar(job *JobConfig, cfg *AIConfig) ([]string, error) {
	if strings.TrimSpace(job.Title) == "" {
		return nil, fmt.Errorf("岗位名称为空，请先补充岗位资料")
	}
	if cfg == nil || cfg.APIKey == "" || cfg.BaseURL == "" {
		return nil, fmt.Errorf("岗位尚无技能指标，请先配置 AI 服务或补充岗位技能")
	}
	prompt := fmt.Sprintf(`根据职位名称和职责，生成与工作任务直接相关、互不重复的岗位技能指标。职位：%s。职责：%s。只依据岗位资料，不引用候选人信息；若职位方向无法可靠判断，令 confident=false。最多8项，不凑数。只输出JSON对象：{"confident":true,"metrics":["指标"]}。`, job.Title, job.JobDescription)
	reply, err := a.callAI(cfg, prompt)
	if err != nil {
		return nil, fmt.Errorf("自动生成岗位指标失败：%w", err)
	}
	start, end := strings.Index(reply, "{"), strings.LastIndex(reply, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("AI 未返回有效岗位指标，请补充岗位职责后重试")
	}
	var parsed struct {
		Confident bool     `json:"confident"`
		Metrics   []string `json:"metrics"`
	}
	if err := json.Unmarshal([]byte(reply[start:end+1]), &parsed); err != nil || !parsed.Confident {
		return nil, fmt.Errorf("无法可靠判断岗位方向，请补充岗位职责或技能后重试")
	}
	metrics := uniqueJobMetrics(parsed.Metrics)
	if len(metrics) == 0 {
		return nil, fmt.Errorf("AI 未生成可靠岗位指标，请补充岗位职责后重试")
	}
	return metrics, nil
}

func (a *App) checkBatchRadarRelevance(job *JobConfig, metrics []string, cfg *AIConfig) error {
	title := strings.ToLower(job.Title)
	medical := strings.Contains(title, "器械") || strings.Contains(title, "临床") || strings.Contains(title, "医疗") || strings.Contains(title, "医药")
	if medical && !strings.Contains(title, "软件") && !strings.Contains(title, "前端") {
		for _, metric := range metrics {
			lower := strings.ToLower(metric)
			if strings.Contains(lower, "vue3") || strings.Contains(lower, "react") || strings.Contains(lower, "typescript") {
				return fmt.Errorf("岗位指标“%s”与“%s”明显不符，请先在职位配置中清理后再分析", metric, job.Title)
			}
		}
	}
	if cfg == nil || cfg.APIKey == "" || cfg.BaseURL == "" {
		return nil
	}
	prompt := fmt.Sprintf(`核对岗位指标是否存在明确的跨职业冲突。岗位名称：%s。岗位职责：%s。指标：%s。仅列明显无关项；不确定时不要列。返回JSON：{"conflicts":["必须与输入指标逐字一致的名称"]}。`, job.Title, job.JobDescription, strings.Join(metrics, "；"))
	reply, err := a.callAI(cfg, prompt)
	if err != nil {
		// An unavailable relevance check must not discard explicitly configured axes.
		return nil
	}
	start, end := strings.Index(reply, "{"), strings.LastIndex(reply, "}")
	if start < 0 || end <= start {
		return nil
	}
	var parsed struct {
		Conflicts []string `json:"conflicts"`
	}
	if json.Unmarshal([]byte(reply[start:end+1]), &parsed) != nil {
		return nil
	}
	for _, conflict := range parsed.Conflicts {
		for _, metric := range metrics {
			if strings.TrimSpace(conflict) == metric {
				return fmt.Errorf("岗位指标“%s”可能与“%s”不符，请先在职位配置中核对后再分析", metric, job.Title)
			}
		}
	}
	return nil
}

func (a *App) PreviewProjectJobRadar(projectID string) (JobRadarDraft, error) {
	p := a.GetProject(projectID)
	if p == nil {
		return JobRadarDraft{}, fmt.Errorf("招聘项目不存在")
	}
	job := &p.JobConfig
	current := getUnifiedJobMetrics(job)
	draft := JobRadarDraft{ProjectID: projectID, SourceHash: jobRadarSourceHash(job), Metrics: append([]string(nil), current...), Origin: "job_requirements"}
	if len(current) >= 5 {
		draft.Note = "当前岗位指标已经达到建议数量；请确认是否保留这组固定指标。"
		return draft, nil
	}
	var suggested []string
	if a.config.AI.APIKey != "" && a.config.AI.BaseURL != "" {
		prompt := fmt.Sprintf(`只根据招聘岗位资料提出与工作任务直接相关、互不重复的岗位能力雷达指标。岗位：%s。职责：%s。已有指标：%s。优先补到总数5至6项，最多8项；证据不足不凑数。不要使用候选人资料，不要加入无关技术。仅输出JSON对象 {"metrics":["维度1"]}。`, job.Title, job.JobDescription, strings.Join(current, "、"))
		if reply, err := a.callAI(&a.config.AI, prompt); err == nil {
			start, end := strings.Index(reply, "{"), strings.LastIndex(reply, "}")
			if start >= 0 && end > start {
				var parsed struct {
					Metrics []string `json:"metrics"`
				}
				if json.Unmarshal([]byte(reply[start:end+1]), &parsed) == nil {
					suggested = parsed.Metrics
					draft.Origin = "ai_inferred"
				}
			}
		}
	}
	if len(suggested) == 0 {
		// Only a matching occupational template can contribute fallback metrics.
		title := strings.ToLower(job.Title)
		supported := strings.Contains(title, "销售") || strings.Contains(title, "客户经理") || strings.Contains(title, "测试") || strings.Contains(title, "前端") || strings.Contains(title, "器械")
		if supported {
			template := a.getFallbackJobMetrics(job.Title)
			suggested = append(template.RequiredSkills, template.CoreCriteria...)
			draft.Origin = "template_inferred"
		} else {
			draft.Note = "岗位资料不足，未找到可靠的职业模板，请补充职责后重试。"
		}
	}
	draft.Metrics = uniqueJobMetrics(append(draft.Metrics, suggested...))
	if len(draft.Metrics) > 6 && len(current) <= 6 {
		draft.Metrics = draft.Metrics[:6]
	}
	if draft.Note == "" {
		draft.Note = "AI/模板推定项只用于岗位雷达，须由 HR 核对；不会自动成为红线。"
	}
	return draft, nil
}

func (a *App) ApplyProjectJobRadar(draft JobRadarDraft) (*Project, error) {
	p := a.GetProject(draft.ProjectID)
	if p == nil {
		return nil, fmt.Errorf("招聘项目不存在")
	}
	if draft.SourceHash == "" || draft.SourceHash != jobRadarSourceHash(&p.JobConfig) {
		return nil, fmt.Errorf("岗位资料已变化，请重新生成雷达草稿")
	}
	metrics := uniqueJobMetrics(draft.Metrics)
	if len(metrics) < 1 || len(metrics) > 8 {
		return nil, fmt.Errorf("请确认 1 至 8 项相关、独立的岗位指标")
	}
	oldHash := p.JobConfig.RadarProfileHash
	newHash := confirmedJobRadarHash(metrics, draft.SourceHash)
	if oldHash != newHash || p.JobConfig.RadarProfileVersion == 0 {
		p.JobConfig.RadarProfileVersion++
	}
	p.JobConfig.RadarMetrics = metrics
	p.JobConfig.RadarProfileHash = newHash
	p.JobConfig.RadarSourceHash = draft.SourceHash
	a.saveProject(p)
	return p, nil
}
