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
	return jobMetricsHash(configuredJobRadarMetrics(job))
}

// configuredJobRadarMetrics reads the current job requirements instead of a
// previously fixed radar. A changed job must not silently reuse old axes.
func configuredJobRadarMetrics(job *JobConfig) []string {
	if job == nil {
		return nil
	}
	items := append([]string(nil), job.RequiredSkills...)
	return uniqueJobMetrics(items)
}

func equalJobMetrics(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func countUniqueSkills(items []string) int {
	seen := make(map[string]bool)
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			seen[item] = true
		}
	}
	return len(seen)
}

// ensureBatchJobRadar fixes one job-wide rubric before any resume worker starts.
// The batch then uses this exact version; one or two relevant axes are valid.
func (a *App) ensureBatchJobRadar(projectID string, cfg *AIConfig) (*Project, error) {
	p := a.GetProject(projectID)
	if p == nil {
		return nil, fmt.Errorf("招聘项目不存在")
	}
	job := &p.JobConfig
	if countUniqueSkills(job.RequiredSkills) > 8 {
		return nil, fmt.Errorf("必备关键技能最多填写 8 项，请删减后分析")
	}
	sourceHash := jobRadarSourceHash(job)
	metrics := configuredJobRadarMetrics(job)
	if len(metrics) == 0 {
		if len(job.RadarMetrics) > 0 || job.RadarProfileHash != "" || job.RadarSourceHash != "" {
			job.RadarMetrics = nil
			job.RadarProfileHash = ""
			job.RadarSourceHash = ""
			if err := a.saveProject(p); err != nil {
				return nil, err
			}
		}
		return p, nil
	}
	if len(job.RadarMetrics) == len(metrics) && job.RadarProfileHash == confirmedJobRadarHash(metrics, sourceHash) && job.RadarSourceHash == sourceHash {
		return p, nil
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
	current := configuredJobRadarMetrics(job)
	draft := JobRadarDraft{ProjectID: projectID, SourceHash: jobRadarSourceHash(job), Metrics: append([]string(nil), current...), Origin: "required_skills", Note: "仅使用已填写的必备关键技能；请在岗位配置中修改指标。"}
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
	configured := configuredJobRadarMetrics(&p.JobConfig)
	if !equalJobMetrics(metrics, configured) {
		return nil, fmt.Errorf("雷达指标必须与岗位必备关键技能一致，请在岗位配置中修改")
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
