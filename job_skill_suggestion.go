package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type JobSkillSuggestionRequest struct {
	Title          string   `json:"title"`
	Direction      string   `json:"direction,omitempty"`
	Department     string   `json:"department,omitempty"`
	JobDescription string   `json:"jobDescription,omitempty"`
	ExistingSkills []string `json:"existingSkills,omitempty"`
}

type JobSkillSuggestionResult struct {
	Status              string   `json:"status"`
	Direction           string   `json:"direction,omitempty"`
	Skills              []string `json:"skills,omitempty"`
	SuggestedDirections []string `json:"suggestedDirections,omitempty"`
	Message             string   `json:"message,omitempty"`
}

func parseJobSkillSuggestions(output string) ([]string, error) {
	start, end := strings.Index(output, "{"), strings.LastIndex(output, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("AI 未返回可解析的技能列表")
	}
	var payload struct {
		Skills []string `json:"skills"`
	}
	if err := json.Unmarshal([]byte(output[start:end+1]), &payload); err != nil {
		return nil, fmt.Errorf("AI 技能列表格式错误：%w", err)
	}
	seen := make(map[string]bool)
	var skills []string
	for _, raw := range payload.Skills {
		item := strings.TrimSpace(raw)
		key := strings.ToLower(item)
		if item == "" || len([]rune(item)) > 50 || seen[key] {
			continue
		}
		seen[key] = true
		skills = append(skills, item)
		if len(skills) == 6 {
			break
		}
	}
	if len(skills) == 0 {
		return nil, fmt.Errorf("AI 未提供与职位相关的技能")
	}
	return skills, nil
}

// SuggestJobSkills generates an editable proposal. It never saves the position.
func (a *App) SuggestJobSkills(req JobSkillSuggestionRequest) (JobSkillSuggestionResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return JobSkillSuggestionResult{Status: "error", Message: "请先填写职位名称"}, fmt.Errorf("请先填写职位名称")
	}
	if len([]rune(title)) > 100 || len([]rune(req.JobDescription)) > 4000 {
		return JobSkillSuggestionResult{Status: "error", Message: "职位名称或职责过长"}, fmt.Errorf("职位名称或职责过长")
	}
	direction, suggested, ambiguous := resolveJobDirection(title, req.Direction)
	if ambiguous {
		return JobSkillSuggestionResult{Status: "needs_direction", SuggestedDirections: suggested, Message: "请先确认岗位方向"}, nil
	}
	if a.config.AI.APIKey == "" || a.config.AI.BaseURL == "" {
		return JobSkillSuggestionResult{Status: "error", Message: "请先配置 AI 模型服务"}, fmt.Errorf("请先配置 AI 模型服务")
	}
	prompt := fmt.Sprintf(`你是招聘岗位技能顾问。根据职位事实提出 3 至 6 条直接相关、可在履历中核对的专业技能；信息不足时少给，不得凑数。
职位名称：%s
岗位方向：%s
所属部门：%s
岗位职责与要求：%s
已有技能（请勿重复）：%s
严禁将其他岗位技术栈带入；对非软件岗位严禁建议 Vue、React、TypeScript 等前端技能。不要把年龄、性别、学历或工作年限当成技能。
只返回 JSON：{"skills":["技能一","技能二"]}。`, title, direction, strings.TrimSpace(req.Department), strings.TrimSpace(req.JobDescription), strings.Join(req.ExistingSkills, "、"))
	output, err := a.callAI(&a.config.AI, prompt)
	if err != nil {
		return JobSkillSuggestionResult{Status: "error", Message: err.Error()}, err
	}
	skills, err := parseJobSkillSuggestions(output)
	if err != nil {
		return JobSkillSuggestionResult{Status: "error", Message: err.Error()}, err
	}
	conflicts, clean := detectJobContextConflicts(title, direction, skills, "", nil, "")
	if len(clean) == 0 {
		return JobSkillSuggestionResult{Status: "error", Message: "AI 返回的技能与职位方向不符"}, fmt.Errorf("AI 返回的技能与职位方向不符：%s", strings.Join(conflicts, "；"))
	}
	existing := make(map[string]bool)
	for _, item := range req.ExistingSkills {
		existing[strings.ToLower(strings.TrimSpace(item))] = true
	}
	var novel []string
	for _, item := range clean {
		if !existing[strings.ToLower(item)] {
			novel = append(novel, item)
		}
	}
	return JobSkillSuggestionResult{Status: "ready", Direction: direction, Skills: novel, Message: "请预览并选择建议技能"}, nil
}
