package main

import (
	"fmt"
	"strings"
)

// JobDescriptionDraftRequest contains only HR-provided job facts. The AI
// response is a draft and never changes a project until HR saves the form.
type JobDescriptionDraftRequest struct {
	Title           string   `json:"title"`
	Department      string   `json:"department"`
	City            string   `json:"city"`
	ExperienceYears int      `json:"experienceYears"`
	EducationLevel  string   `json:"educationLevel"`
	Skills          []string `json:"skills"`
	CurrentText     string   `json:"currentText"`
}

func (a *App) GenerateJobDescriptionDraft(req JobDescriptionDraftRequest) (string, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return "", fmt.Errorf("请先填写职位名称")
	}
	if len([]rune(title)) > 100 || len([]rune(req.CurrentText)) > 4000 {
		return "", fmt.Errorf("职位名称或现有描述过长，请先精简")
	}
	if a.config.AI.APIKey == "" || a.config.AI.BaseURL == "" {
		return "", fmt.Errorf("请先在 AI 设置中配置可用的模型服务")
	}
	if req.ExperienceYears < 0 {
		req.ExperienceYears = 0
	}
	if req.ExperienceYears > 30 {
		req.ExperienceYears = 30
	}
	skills := make([]string, 0, 12)
	for _, skill := range req.Skills {
		if clean := strings.TrimSpace(skill); clean != "" && len(skills) < 12 {
			skills = append(skills, clean)
		}
	}
	prompt := fmt.Sprintf(`你是招聘岗位文案编辑。根据 HR 提供的已知信息，起草清晰、具体、可编辑的中文岗位说明。
只写“岗位职责”和“任职要求”两个标题及其编号条目。不要编造公司业务、待遇、证书、年龄、性别、工作年限或学历门槛；对未提供的事实使用中性表述。现有描述与其他字段不一致时，以现有描述中的明确事实为参考，不自行提高门槛。不要输出 Markdown 代码块或解释。

---

职位名称：%s
部门：%s
城市：%s
最低工作经验：%d 年
最低学历：%s
已确认技能：%s
HR 当前描述：%s`, title, strings.TrimSpace(req.Department), strings.TrimSpace(req.City), req.ExperienceYears, strings.TrimSpace(req.EducationLevel), strings.Join(skills, "、"), strings.TrimSpace(req.CurrentText))
	output, err := a.callAI(&a.config.AI, prompt)
	if err != nil {
		return "", err
	}
	draft := strings.TrimSpace(output)
	draft = strings.TrimPrefix(draft, "```text")
	draft = strings.TrimPrefix(draft, "```markdown")
	draft = strings.TrimPrefix(draft, "```")
	draft = strings.TrimSuffix(strings.TrimSpace(draft), "```")
	draft = strings.TrimSpace(draft)
	if !strings.Contains(draft, "岗位职责") || !strings.Contains(draft, "任职要求") {
		return "", fmt.Errorf("AI 未返回完整的岗位职责与任职要求，请重试")
	}
	if len([]rune(draft)) > 4000 {
		return "", fmt.Errorf("AI 返回内容过长，请重试")
	}
	return draft, nil
}
