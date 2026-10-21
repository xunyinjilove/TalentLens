package main

import (
	"fmt"
	"regexp"
	"strings"
)

// JobDescriptionDraftRequest contains HR-provided job facts and generation options.
type JobDescriptionDraftRequest struct {
	FormInstanceID   string   `json:"formInstanceId,omitempty"`
	ProjectID        string   `json:"projectId,omitempty"`
	RequestID        string   `json:"requestId,omitempty"`
	Title            string   `json:"title"`
	Direction        string   `json:"direction,omitempty"`        // HR 确认的岗位方向
	Department       string   `json:"department,omitempty"`
	City             string   `json:"city,omitempty"`
	ExperienceYears  int      `json:"experienceYears"`
	EducationLevel   string   `json:"educationLevel,omitempty"`
	Skills           []string `json:"skills"`
	CurrentText      string   `json:"currentText,omitempty"`
	Mode             string   `json:"mode,omitempty"`             // "fresh" (全新起草) 或 "refine" (基于原文润色)
	PromptSupplement string   `json:"promptSupplement,omitempty"`  // HR 本次补充说明
	ConfirmedSkills  []string `json:"confirmedSkills,omitempty"`  // HR 明确确认保留的跨领域技能
}

// JobDescriptionDraftResult contains the generated draft and structured validation info.
type JobDescriptionDraftResult struct {
	Status              string   `json:"status"` // "ready", "needs_direction", "error"
	Message             string   `json:"message"`
	Draft               string   `json:"draft"`
	Direction           string   `json:"direction"`
	SuggestedDirections []string `json:"suggestedDirections,omitempty"`
	Responsibilities    []string `json:"responsibilities,omitempty"`
	Requirements        []string `json:"requirements,omitempty"`
	Conflicts           []string `json:"conflicts,omitempty"`
	RequestID           string   `json:"requestId,omitempty"`
}

var frontendKeywords = []string{
	"vue", "vue3", "typescript", "ts", "react", "angular",
	"css", "css3", "html5", "webpack", "vite", "前端开发", "代码评审", "前端组件", "javascript",
}

func resolveJobDirection(title, currentDirection string) (string, []string, bool) {
	cleanTitle := strings.TrimSpace(title)
	cleanDir := strings.TrimSpace(currentDirection)
	if cleanDir != "" {
		return cleanDir, nil, false
	}

	lowerTitle := strings.ToLower(cleanTitle)

	// 1. 器械 / 医疗器械相关
	if strings.Contains(lowerTitle, "器械") {
		if strings.Contains(lowerTitle, "结构") {
			return "机械结构与外观设计方向", nil, false
		}
		if strings.Contains(lowerTitle, "软件") || strings.Contains(lowerTitle, "上位机") {
			return "器械配套上位机与嵌入式软件方向", nil, false
		}
		if strings.Contains(lowerTitle, "电气") || strings.Contains(lowerTitle, "硬件") {
			return "电气控制与硬件电路研发方向", nil, false
		}
		if strings.Contains(lowerTitle, "注册") || strings.Contains(lowerTitle, "法规") || strings.Contains(lowerTitle, "体系") {
			return "医疗器械注册申报与质量体系方向", nil, false
		}
		return "", []string{
			"机械结构与外观设计方向",
			"电气控制与嵌入式硬件方向",
			"医疗器械注册与体系法规方向",
			"器械配套上位机与软件开发方向",
			"售后技术支持与临床装机方向",
		}, true
	}

	// 2. 硬件工程师
	if cleanTitle == "硬件工程师" || cleanTitle == "硬件开发工程师" {
		return "", []string{
			"PCB 原理图与硬件电路设计",
			"单片机与嵌入式系统开发",
			"射频与信号完整性方向",
			"电源与模拟电路设计",
			"硬件测试与可靠性验证",
		}, true
	}

	// 3. 机械工程师
	if cleanTitle == "机械工程师" || cleanTitle == "机械设计工程师" {
		return "", []string{
			"机械结构与机构运动设计",
			"自动化非标设备研发",
			"模具与注塑工艺设计",
			"热力学与流体仿真分析",
			"机械加工与装配工艺",
		}, true
	}

	// 4. 工艺工程师
	if cleanTitle == "工艺工程师" || cleanTitle == "PE工程师" || cleanTitle == "IE工程师" {
		return "", []string{
			"制程工艺改善与良率提升",
			"工业工程(IE)与精益生产",
			"新产品导入(NPI)工艺开发",
			"设备治具设计与维护",
		}, true
	}

	// 其他明确职位，默认以职位名称为方向
	return cleanTitle, nil, false
}

func detectJobContextConflicts(title, direction string, skills []string, currentText string, confirmedSkills []string, promptSupplement string) ([]string, []string) {
	combinedScope := strings.ToLower(title + " " + direction + " " + promptSupplement)

	// 判断是否为软件/前端相关岗位
	isSoftwareRelated := strings.Contains(combinedScope, "前端") ||
		strings.Contains(combinedScope, "软件") ||
		strings.Contains(combinedScope, "web") ||
		strings.Contains(combinedScope, "上位机") ||
		(strings.Contains(combinedScope, "开发工程师") && !strings.Contains(combinedScope, "机械") && !strings.Contains(combinedScope, "结构") && !strings.Contains(combinedScope, "器械") && !strings.Contains(combinedScope, "硬件"))

	var conflicts []string
	var cleanSkills []string

	confirmedSet := make(map[string]bool)
	for _, cs := range confirmedSkills {
		confirmedSet[strings.ToLower(strings.TrimSpace(cs))] = true
	}

	for _, s := range skills {
		sClean := strings.TrimSpace(s)
		if sClean == "" {
			continue
		}
		sLower := strings.ToLower(sClean)

		isFrontendTech := false
		for _, kw := range frontendKeywords {
			if sLower == kw || strings.Contains(sLower, kw) {
				isFrontendTech = true
				break
			}
		}

		if isFrontendTech && !isSoftwareRelated && !confirmedSet[sLower] {
			conflicts = append(conflicts, fmt.Sprintf("技能【%s】属于 Web 前端技术，与当前【%s】岗位核心产出存在跨领域冲突，已自动隔离", sClean, title))
		} else {
			cleanSkills = append(cleanSkills, sClean)
		}
	}

	return conflicts, cleanSkills
}

func buildJobDescriptionPrompt(req JobDescriptionDraftRequest, direction string, cleanSkills []string) string {
	mode := req.Mode
	if mode == "" {
		mode = "fresh"
	}

	skillsText := "暂无特定限制，请根据岗位方向合理拟定"
	if len(cleanSkills) > 0 {
		skillsText = strings.Join(cleanSkills, "、")
	}

	prompt := fmt.Sprintf(`你是招聘岗位文案专家。请根据已知事实，为【%s】岗位起草严谨、具体、符合现代招聘标准的岗位说明。

【核心准则】：
1. 必须完全围绕当前职位【%s】及专业方向【%s】的工作职责与任职资格起草。
2. 严禁出现与该岗位方向无关的技术栈或工作内容（例如：在机械/硬件/医疗器械/临床/财务等非前端岗位中，严禁出现 Vue/React/Web前端开发/代码评审等内容）。
3. 只输出“一、岗位职责”和“二、任职要求”两个标准标题及其编号条目。
4. 不要主观编造公司规模、福利待遇、不合理的学历/年限门槛，保持专业、求实的中性表达。
5. 最终输出无需代码块标记，不要包含多余寒暄。

【岗位基本事实】：
- 职位名称：%s
- 岗位方向：%s
- 所属部门：%s
- 工作城市：%s
- 最低工作经验：%d 年
- 最低学历要求：%s
- 核心参考技能：%s`,
		req.Title, req.Title, direction,
		req.Title, direction,
		strings.TrimSpace(req.Department),
		strings.TrimSpace(req.City),
		req.ExperienceYears,
		strings.TrimSpace(req.EducationLevel),
		skillsText,
	)

	if strings.TrimSpace(req.PromptSupplement) != "" {
		prompt += fmt.Sprintf("\n- HR 补充要求：%s", strings.TrimSpace(req.PromptSupplement))
	}

	if mode == "refine" && strings.TrimSpace(req.CurrentText) != "" {
		prompt += fmt.Sprintf("\n\n【现有原描述参考（请在优化时剔除与当前岗位方向无关的内容）】：\n%s", strings.TrimSpace(req.CurrentText))
	}

	return prompt
}

func validateJobDescriptionDraft(draft string, title, direction string, cleanSkills []string) (string, []string, []string, error) {
	d := strings.TrimSpace(draft)
	d = strings.TrimPrefix(d, "```text")
	d = strings.TrimPrefix(d, "```markdown")
	d = strings.TrimPrefix(d, "```")
	d = strings.TrimSuffix(strings.TrimSpace(d), "```")
	d = strings.TrimSpace(d)

	if !strings.Contains(d, "岗位职责") || !strings.Contains(d, "任职要求") {
		return "", nil, nil, fmt.Errorf("AI 未返回完整的“岗位职责”与“任职要求”，请重试")
	}
	if len([]rune(d)) > 4000 {
		return "", nil, nil, fmt.Errorf("AI 返回内容过长，请精简输入后重试")
	}

	// 提取结构化条目并检查跨领域错岗漏斗
	combinedScope := strings.ToLower(title + " " + direction)
	isNonSoftware := strings.Contains(combinedScope, "器械") ||
		strings.Contains(combinedScope, "机械") ||
		strings.Contains(combinedScope, "临床") ||
		strings.Contains(combinedScope, "硬件")
	if strings.Contains(combinedScope, "软件") || strings.Contains(combinedScope, "上位机") || strings.Contains(combinedScope, "前端") {
		isNonSoftware = false
	}

	lines := strings.Split(d, "\n")
	var validLines []string
	var responsibilities []string
	var requirements []string
	currentSection := "" // "resp" or "req"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			validLines = append(validLines, "")
			continue
		}
		if strings.Contains(trimmed, "岗位职责") {
			currentSection = "resp"
			validLines = append(validLines, trimmed)
			continue
		}
		if strings.Contains(trimmed, "任职要求") {
			currentSection = "req"
			validLines = append(validLines, trimmed)
			continue
		}

		// 如果非软件岗位，检查当前行是否包含严重错岗关键词（如 Vue、TypeScript 等）
		if isNonSoftware {
			lineLower := strings.ToLower(trimmed)
			hasFrontendPollution := false
			for _, kw := range []string{"vue", "typescript", "react", "前端开发", "代码评审"} {
				if strings.Contains(lineLower, kw) {
					hasFrontendPollution = true
					break
				}
			}
			if hasFrontendPollution {
				// 丢弃这一被污染的行
				continue
			}
		}

		validLines = append(validLines, trimmed)
		cleanItem := regexp.MustCompile(`^[\d\.\-\*、\s]+`).ReplaceAllString(trimmed, "")
		cleanItem = strings.TrimSpace(cleanItem)
		if cleanItem != "" {
			if currentSection == "resp" {
				responsibilities = append(responsibilities, cleanItem)
			} else if currentSection == "req" {
				requirements = append(requirements, cleanItem)
			}
		}
	}

	cleanedDraft := strings.Join(validLines, "\n")
	return cleanedDraft, responsibilities, requirements, nil
}

// GenerateJobDescriptionDraft 生成与岗位完全对齐的职责与要求草稿，杜绝跨领域模板污染。
func (a *App) GenerateJobDescriptionDraft(req JobDescriptionDraftRequest) (JobDescriptionDraftResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return JobDescriptionDraftResult{Status: "error", Message: "请先填写职位名称", RequestID: req.RequestID}, fmt.Errorf("请先填写职位名称")
	}
	if len([]rune(title)) > 100 || len([]rune(req.CurrentText)) > 4000 {
		return JobDescriptionDraftResult{Status: "error", Message: "职位名称或现有描述过长，请先精简", RequestID: req.RequestID}, fmt.Errorf("职位名称或现有描述过长，请先精简")
	}

	// 1. 确认岗位方向
	direction, suggested, isAmbiguous := resolveJobDirection(title, req.Direction)
	if isAmbiguous {
		return JobDescriptionDraftResult{
			Status:              "needs_direction",
			Message:             fmt.Sprintf("职位【%s】包含多个专业方向，请选择具体的岗位方向以便精准生成", title),
			Direction:           "",
			SuggestedDirections: suggested,
			RequestID:           req.RequestID,
		}, nil
	}

	// 2. 检查大模型配置
	if a.config.AI.APIKey == "" || a.config.AI.BaseURL == "" {
		return JobDescriptionDraftResult{Status: "error", Message: "请先在 AI 设置中配置可用的模型服务", RequestID: req.RequestID}, fmt.Errorf("请先在 AI 设置中配置可用的模型服务")
	}

	if req.ExperienceYears < 0 {
		req.ExperienceYears = 0
	}
	if req.ExperienceYears > 30 {
		req.ExperienceYears = 30
	}

	// 3. 冲突检测与历史技能隔离
	conflicts, cleanSkills := detectJobContextConflicts(title, direction, req.Skills, req.CurrentText, req.ConfirmedSkills, req.PromptSupplement)

	// 4. 构建提示词
	prompt := buildJobDescriptionPrompt(req, direction, cleanSkills)

	// 5. 调用大模型
	output, err := a.callAI(&a.config.AI, prompt)
	if err != nil {
		return JobDescriptionDraftResult{Status: "error", Message: err.Error(), RequestID: req.RequestID}, err
	}

	// 6. 校验与结果清洗
	draft, resps, reqs, err := validateJobDescriptionDraft(output, title, direction, cleanSkills)
	if err != nil {
		return JobDescriptionDraftResult{Status: "error", Message: err.Error(), RequestID: req.RequestID}, err
	}

	return JobDescriptionDraftResult{
		Status:           "ready",
		Message:          "生成成功",
		Draft:            draft,
		Direction:        direction,
		Responsibilities: resps,
		Requirements:     reqs,
		Conflicts:        conflicts,
		RequestID:        req.RequestID,
	}, nil
}
