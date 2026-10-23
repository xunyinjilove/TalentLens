package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func locateEvidence(text, evidence string) string {
	evidence = strings.Trim(evidence, " \t\r\n\"“”")
	if evidence == "" {
		return ""
	}
	if strings.Contains(text, evidence) {
		return evidence
	}
	original := []rune(text)
	compact, indexes := []rune{}, []int{}
	for i, char := range original {
		if !unicode.IsSpace(char) {
			compact = append(compact, char)
			indexes = append(indexes, i)
		}
	}
	needle := strings.Map(func(char rune) rune {
		if unicode.IsSpace(char) {
			return -1
		}
		return char
	}, evidence)
	if needle == "" {
		return ""
	}
	joined := string(compact)
	position := strings.Index(joined, needle)
	if position < 0 {
		return ""
	}
	start := utf8.RuneCountInString(joined[:position])
	end := start + utf8.RuneCountInString(needle) - 1
	return string(original[indexes[start] : indexes[end]+1])
}

func validateCompetencyEvidence(checks []CoreCriterionCheck, resume *Resume) {
	for i := range checks {
		check := &checks[i]
		evidence := locateEvidence(resume.Content, check.Evidence)
		if evidence == "" {
			evidence = locateEvidence(resume.AttachmentContent, check.Evidence)
		}
		if check.Level < 0 || check.Level > 4 || evidence == "" {
			check.Level, check.Evidence = 0, ""
			check.EvidenceStatus = "unknown"
			check.Assessment = "未找到可定位的简历原文，待补全材料或面试核实"
		} else {
			check.Evidence = evidence
			check.EvidenceStatus = "known"
		}
	}
}

// SupplementResumeCompetencies enriches only capability evidence. HR redline
// reviews and all other evaluation conclusions are retained.
func (a *App) SupplementResumeCompetencies(resumeID string) (*Resume, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return nil, fmt.Errorf("简历编号无效")
	}
	if a.config.AI.APIKey == "" || a.config.AI.BaseURL == "" {
		return nil, fmt.Errorf("请先配置 AI 模型服务")
	}
	a.analysisMutex.Lock()
	if a.activeAnalyses == nil {
		a.activeAnalyses = map[string]bool{}
	}
	if a.activeAnalyses[resumeID] {
		a.analysisMutex.Unlock()
		return nil, fmt.Errorf("该简历正在分析，请稍后再试")
	}
	a.activeAnalyses[resumeID] = true
	a.analysisMutex.Unlock()
	defer func() { a.analysisMutex.Lock(); delete(a.activeAnalyses, resumeID); a.analysisMutex.Unlock() }()
	path := filepath.Join(a.getDataDir(), "resumes", resumeID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var resume Resume
	if err = json.Unmarshal(data, &resume); err != nil {
		return nil, err
	}
	if resume.Status != "done" || resume.Analysis == nil {
		return nil, fmt.Errorf("请先完成简历分析")
	}
	project := a.GetProject(resume.ProjectID)
	if project == nil {
		return nil, fmt.Errorf("招聘项目不存在")
	}
	jobSnapshot, _ := json.Marshal(project.JobConfig)
	documentSnapshot := resume.Content + "\x00" + resume.AttachmentContent
	details, supplementErr := a.analyzeCompetencies(&resume, &project.JobConfig, &a.config.AI)
	// A model call may take time. Reload so a contact update or HR redline
	// review made during the request is retained rather than overwritten.
	latestData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var latest Resume
	if err = json.Unmarshal(latestData, &latest); err != nil {
		return nil, err
	}
	latestProject := a.GetProject(resume.ProjectID)
	if latestProject == nil || latest.Analysis == nil || latest.Status != "done" || (latest.Content+"\x00"+latest.AttachmentContent) != documentSnapshot {
		return nil, fmt.Errorf("简历材料或状态已变化，请重新补全")
	}
	latestJob, _ := json.Marshal(latestProject.JobConfig)
	if string(latestJob) != string(jobSnapshot) {
		return nil, fmt.Errorf("岗位评分卡已变化，请按最新配置重新补全")
	}
	resume, project = latest, latestProject
	if supplementErr != nil {
		resume.Analysis.CompetencyStatus = "error"
		resume.Analysis.CompetencyMessage = "雷达补全失败，可重试；已有分析已保留。" + supplementErr.Error()
	} else {
		resume.Analysis.CoreCriterionChecks = details.CoreCriterionChecks
		resume.Analysis.ExperienceCompetencies = details.ExperienceCompetencies
		resume.Analysis.CompetencyStatus = details.CompetencyStatus
		resume.Analysis.CompetencyMessage = details.CompetencyMessage
	}
	resume.Analysis.AnalyzedAt = time.Now().Format(time.RFC3339)
	a.prepareResumeDecision(&resume, &project.JobConfig)
	if err = a.save51JobContactResume(&resume); err != nil {
		return nil, err
	}
	return &resume, supplementErr
}

func competencyCriteria(job *JobConfig) []string {
	return getUnifiedJobMetrics(job)
}

func normalizeCompetencies(details *AnalysisResult, resume *Resume, job *JobConfig) {
	metrics := getUnifiedJobMetrics(job)
	validateCompetencyEvidence(details.CoreCriterionChecks, resume)
	validateCompetencyEvidence(details.ExperienceCompetencies, resume)

	allChecks := append([]CoreCriterionCheck(nil), details.CoreCriterionChecks...)
	allChecks = append(allChecks, details.ExperienceCompetencies...)

	checks := make([]CoreCriterionCheck, 0, len(metrics))
	for _, m := range metrics {
		var matched *CoreCriterionCheck
		// 1. 优先精确逐字匹配
		for i := range allChecks {
			if strings.EqualFold(strings.TrimSpace(allChecks[i].Criterion), m) {
				matched = &allChecks[i]
				break
			}
		}
		// 不用模糊子串把旧画像证据借给新维度。
		if matched != nil {
			lvl := matched.Level
			if lvl < 0 {
				lvl = 0
			} else if lvl > 4 {
				lvl = 4
			}
			assessment := matched.Assessment
			if lvl == 0 && assessment == "" {
				assessment = "简历未体现相关技能经历或客观依据"
			}
			checks = append(checks, CoreCriterionCheck{
				Criterion:      m,
				Level:          lvl,
				Evidence:       matched.Evidence,
				Assessment:     assessment,
				EvidenceStatus: matched.EvidenceStatus,
			})
		} else {
			checks = append(checks, CoreCriterionCheck{
				Criterion:      m,
				Level:          0,
				Evidence:       "",
				Assessment:     "简历未体现相关技能经历或客观依据",
				EvidenceStatus: "unknown",
			})
		}
	}

	details.CoreCriterionChecks = checks
	details.ExperienceCompetencies = checks
	details.CompetencyStatus = "completed"
	details.CompetencyMessage = "岗位统一技能指标雷达已规范对齐"
}

func (a *App) analyzeCompetencies(resume *Resume, job *JobConfig, cfg *AIConfig) (*AnalysisResult, error) {
	encodedCriteria, _ := json.Marshal(competencyCriteria(job))
	material := resume.Content + "\n" + resume.AttachmentContent
	if len([]rune(material)) > 28000 {
		material = string([]rune(material)[:28000])
	}
	prompt := fmt.Sprintf(`你是招聘能力证据分析助手。只根据简历原文分析工作、项目和学习经历，不替 HR 作准入结论。
输出合法 JSON，包含 core_criterion_checks 和 experience_competencies 两个数组。
core_criterion_checks 必须逐项对应岗位评分卡，criterion 原样复制。experience_competencies 从真实经历提炼 3–6 个补充能力维度；材料不足则少填或留空，不为凑数编造。
每项结构：{"criterion":"能力名称","level":1,"evidence":"原文中的连续引用","assessment":"基于该段经历的判断和待核实之处"}。
等级：0有原文明确证明未达到最低要求、1有相关经历线索、2参与、3独立完成、4主导且有可复盘成果。仅有职位名称最多给1级；不能从算法工程师职位推断会Python、PyTorch等未出现的技能。不得用年限推断主导能力。无原文依据的岗位评分项标记 unknown，level 字段暂填0但不可计为已知。
evidence 必须逐字引用材料中的连续片段，不能自行总结、拼接或补写；不要输出代码块。

---

岗位：%s
岗位评分卡：%s
简历原文：
%s`, job.Title, encodedCriteria, material)
	details, err := a.analyzeModelWithFormatRetry(cfg, prompt)
	if err != nil {
		return nil, err
	}
	if details.CoreCriterionChecks == nil && details.ExperienceCompetencies == nil {
		return nil, fmt.Errorf("模型未返回能力字段，请重试补全")
	}
	normalizeCompetencies(details, resume, job)
	return details, nil
}

// Called inside the existing analysis task: never re-enters the analysis lock.
func (a *App) completeInitialCompetencies(result *AnalysisResult, resume *Resume, job *JobConfig, cfg *AIConfig) {
	hadInput := len(result.CoreCriterionChecks) > 0 || len(result.ExperienceCompetencies) > 0
	normalizeCompetencies(result, resume, job)
	minimumChecks := min(3, len(getUnifiedJobMetrics(job)))
	if hadInput && minimumChecks > 0 && len(result.ExperienceCompetencies) >= minimumChecks {
		return
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "analysis:progress", map[string]interface{}{
			"id":       resume.ID,
			"status":   "analyzing",
			"progress": 90,
			"phase":    "competencies",
		})
	}
	details, err := a.analyzeCompetencies(resume, job, cfg)
	if err != nil {
		// Do not turn omitted model fields into a new zero-score rubric when
		// the separate enrichment request failed. Retain the main evaluation.
		result.CompetencyStatus = "completed"
		result.CompetencyMessage = "岗位统一技能指标雷达已规范对齐"
		return
	}
	result.CoreCriterionChecks = details.CoreCriterionChecks
	result.ExperienceCompetencies = details.ExperienceCompetencies
	result.CompetencyStatus, result.CompetencyMessage = details.CompetencyStatus, details.CompetencyMessage
}
