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
		if check.Level < 0 || check.Level > 4 || (check.Level > 0 && evidence == "") {
			check.Level, check.Evidence = 0, ""
			check.Assessment = "未找到可定位的简历原文，待补全材料或面试核实"
		} else {
			check.Evidence = evidence
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
	criteria := project.JobConfig.CoreCriteria
	if len(criteria) == 0 {
		criteria = project.JobConfig.RequiredSkills
	}
	encodedCriteria, _ := json.Marshal(criteria)
	jobSnapshot, _ := json.Marshal(project.JobConfig)
	documentSnapshot := resume.Content + "\x00" + resume.AttachmentContent
	material := resume.Content + "\n" + resume.AttachmentContent
	if len([]rune(material)) > 28000 {
		material = string([]rune(material)[:28000])
	}
	prompt := fmt.Sprintf(`你是招聘能力证据分析助手。只根据简历原文分析工作、项目和学习经历，不替 HR 作准入结论。
输出合法 JSON，包含 core_criterion_checks 和 experience_competencies 两个数组。
core_criterion_checks 必须逐项对应岗位评分卡，criterion 原样复制。experience_competencies 从真实经历提炼 3–6 个补充能力维度；材料不足则少填或留空，不为凑数编造。
每项结构：{"criterion":"能力名称","level":1,"evidence":"原文中的连续引用","assessment":"基于该段经历的判断和待核实之处"}。
等级：0未体现、1有相关经历线索、2参与、3独立完成、4主导且有可复盘成果。仅有职位名称最多给1级；不能从算法工程师职位推断会Python、PyTorch等未出现的技能。不得用年限推断主导能力。无原文依据的岗位评分项给0并说明缺少什么。
evidence 必须逐字引用材料中的连续片段，不能自行总结、拼接或补写；不要输出代码块。

---

岗位：%s
岗位评分卡：%s
简历原文：
%s`, project.JobConfig.Title, encodedCriteria, material)
	details, err := a.analyzeModelWithFormatRetry(&a.config.AI, prompt)
	if err != nil {
		return nil, err
	}
	validateCompetencyEvidence(details.CoreCriterionChecks, &resume)
	validateCompetencyEvidence(details.ExperienceCompetencies, &resume)
	byCriterion := map[string]CoreCriterionCheck{}
	for _, check := range details.CoreCriterionChecks {
		byCriterion[check.Criterion] = check
	}
	checks := make([]CoreCriterionCheck, 0, len(criteria))
	for _, criterion := range criteria {
		check, found := byCriterion[criterion]
		if !found {
			check = CoreCriterionCheck{Criterion: criterion, Assessment: "简历未提供该项能力的可核对依据"}
		}
		checks = append(checks, check)
	}
	extra := make([]CoreCriterionCheck, 0, 6)
	for _, check := range details.ExperienceCompetencies {
		if check.Level > 0 && check.Evidence != "" && strings.TrimSpace(check.Criterion) != "" && len(extra) < 6 {
			extra = append(extra, check)
		}
	}
	if len(checks) == 0 && len(extra) == 0 {
		return nil, fmt.Errorf("简历信息不足，未取得可核对的能力证据")
	}
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
	resume.Analysis.CoreCriterionChecks = checks
	resume.Analysis.ExperienceCompetencies = extra
	resume.Analysis.AnalyzedAt = time.Now().Format(time.RFC3339)
	a.prepareResumeDecision(&resume, &project.JobConfig)
	if err = a.save51JobContactResume(&resume); err != nil {
		return nil, err
	}
	return &resume, nil
}
