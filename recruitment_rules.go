package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type JobRule struct {
	ID         string `json:"id"`
	ProjectID  string `json:"projectId"`
	Kind       string `json:"kind"`
	Criterion  string `json:"criterion"`
	Source     string `json:"source"`
	Status     string `json:"status"`
	Version    int    `json:"version"`
	ApprovedBy string `json:"approvedBy"`
	ApprovedAt string `json:"approvedAt"`
	CreatedAt  string `json:"createdAt"`
}

type JobRuleApproval struct {
	ID        string `json:"id"`
	Criterion string `json:"criterion"`
	Approved  bool   `json:"approved"`
}

func (a *App) ListJobRules(projectID string) ([]JobRule, error) {
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id,project_id,kind,criterion,source,status,version,approved_by,approved_at,created_at
		FROM job_rules WHERE project_id=? ORDER BY version DESC,created_at,id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]JobRule, 0)
	for rows.Next() {
		var item JobRule
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Kind, &item.Criterion, &item.Source,
			&item.Status, &item.Version, &item.ApprovedBy, &item.ApprovedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func jobRuleTemplate(job JobConfig) []JobRule {
	result := make([]JobRule, 0)
	appendItems := func(kind string, items []string) {
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				result = append(result, JobRule{Kind: kind, Criterion: item, Source: "template", Status: "draft"})
			}
		}
	}
	redLines := job.RedLines
	if len(redLines) == 0 {
		title := strings.ToLower(job.Title)
		switch {
		case strings.Contains(title, "临床项目"):
			redLines = []string{"能提供可核实的临床项目整体计划、执行与问题处理经历"}
		case strings.Contains(title, "go开发") || strings.Contains(title, "go工程"):
			redLines = []string{"能提供可核实的 Go 生产服务开发与维护经历"}
		case strings.Contains(title, "前端"):
			redLines = []string{"能提供可核实的前端产品开发、上线与维护经历"}
		}
	}
	appendItems("redline", redLines)
	core := job.CoreCriteria
	if len(core) == 0 {
		core = job.RequiredSkills
	}
	appendItems("core", core)
	appendItems("bonus", job.BonusPoints)
	return result
}

// All suggestions remain drafts until HR edits and approves them. Existing
// project criteria are never overwritten by this operation.
func (a *App) SuggestJobRules(projectID string, withAI bool) ([]JobRule, error) {
	project := a.GetProject(projectID)
	if project == nil {
		return nil, fmt.Errorf("招聘项目不存在")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	suggestions := jobRuleTemplate(project.JobConfig)
	if withAI {
		if strings.TrimSpace(a.config.AI.APIKey) == "" {
			return nil, fmt.Errorf("AI 未配置，请先在设置中填写 API Key")
		}
		prompt := fmt.Sprintf(`你是招聘岗位规则起草助手。根据下面的岗位资料，补充最多 2 条直接关联工作职责且可核实的红线建议、最多 4 条核心能力、最多 3 条附加优势。允许没有红线。不要以年龄、性别、学校名气、企业名气或外包身份作为规则。AI 建议只作为 HR 审批草稿，不能直接淘汰候选人。每条要具体可验证，不重复岗位资料中已有条目。只输出 JSON 数组，每项包含 kind（redline/core/bonus）和 criterion（中文）。
岗位：%s
职责：%s
已有红线：%s
核心能力：%s
已有加分：%s`, project.JobConfig.Title, project.JobConfig.JobDescription,
			strings.Join(project.JobConfig.RedLines, "；"), strings.Join(project.JobConfig.CoreCriteria, "；"), strings.Join(project.JobConfig.BonusPoints, "；"))
		response, err := a.callAI(&a.config.AI, prompt)
		if err != nil {
			return nil, fmt.Errorf("生成 AI 规则建议失败: %w", err)
		}
		start, end := strings.Index(response, "["), strings.LastIndex(response, "]")
		if start < 0 || end <= start {
			return nil, fmt.Errorf("AI 未返回有效规则列表")
		}
		var aiItems []JobRule
		if err := json.Unmarshal([]byte(response[start:end+1]), &aiItems); err != nil {
			return nil, fmt.Errorf("AI 规则格式无效: %w", err)
		}
		for _, item := range aiItems {
			if item.Kind != "redline" && item.Kind != "core" && item.Kind != "bonus" {
				continue
			}
			criterion := strings.TrimSpace(item.Criterion)
			if criterion == "" || len([]rune(criterion)) > 220 {
				continue
			}
			suggestions = append(suggestions, JobRule{Kind: item.Kind, Criterion: criterion, Source: "ai", Status: "draft"})
		}
	}
	existing, err := a.ListJobRules(projectID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, item := range existing {
		if item.Status == "draft" {
			seen[item.Kind+"\x00"+item.Criterion] = true
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, item := range suggestions {
		key := item.Kind + "\x00" + item.Criterion
		if seen[key] {
			continue
		}
		seen[key] = true
		item.ID = recruitmentID("rule")
		item.ProjectID = projectID
		item.CreatedAt = now
		_, err = db.Exec(`INSERT INTO job_rules(id,project_id,kind,criterion,source,status,version,created_at)
			VALUES(?,?,?,?,?,'draft',0,?)`, item.ID, item.ProjectID, item.Kind, item.Criterion, item.Source, now)
		if err != nil {
			return nil, err
		}
	}
	return a.ListJobRules(projectID)
}

// An approval is a complete, explicit set of rules for the new version. Old
// evaluations keep their prior rule_version and HR review data.
func (a *App) ApproveJobRules(projectID, actor string, choices []JobRuleApproval) (int, error) {
	if strings.TrimSpace(actor) == "" {
		return 0, fmt.Errorf("请填写审批人")
	}
	project := a.GetProject(projectID)
	if project == nil {
		return 0, fmt.Errorf("招聘项目不存在")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return 0, err
	}
	rules, err := a.ListJobRules(projectID)
	if err != nil {
		return 0, err
	}
	byID := make(map[string]JobRule, len(rules))
	for _, rule := range rules {
		if rule.Status == "draft" {
			byID[rule.ID] = rule
		}
	}
	if len(byID) == 0 {
		return 0, fmt.Errorf("当前没有待审批的岗位规则草稿")
	}
	seen := make(map[string]bool)
	red, core, bonus := []string{}, []string{}, []string{}
	for _, choice := range choices {
		if seen[choice.ID] {
			return 0, fmt.Errorf("规则重复提交")
		}
		seen[choice.ID] = true
		rule, ok := byID[choice.ID]
		if !ok {
			return 0, fmt.Errorf("规则不属于本项目的待审批草稿: %s", choice.ID)
		}
		if !choice.Approved {
			continue
		}
		criterion := strings.TrimSpace(choice.Criterion)
		if criterion == "" || len([]rune(criterion)) > 220 {
			return 0, fmt.Errorf("规则内容为空或过长")
		}
		switch rule.Kind {
		case "redline":
			red = append(red, criterion)
		case "core":
			core = append(core, criterion)
		case "bonus":
			bonus = append(bonus, criterion)
		}
	}
	if len(seen) != len(byID) {
		return 0, fmt.Errorf("请逐条确认或拒绝本次草稿")
	}
	var version int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version),0)+1 FROM job_rules WHERE project_id=? AND status='approved'`, projectID).Scan(&version); err != nil {
		return 0, err
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, choice := range choices {
		status := "rejected"
		if choice.Approved {
			status = "approved"
		}
		if _, err = tx.Exec(`UPDATE job_rules SET criterion=?,status=?,version=?,approved_by=?,approved_at=? WHERE id=?`,
			strings.TrimSpace(choice.Criterion), status, version, actor, now, choice.ID); err != nil {
			return 0, err
		}
	}
	project.JobConfig.RedLines = red
	project.JobConfig.CoreCriteria = core
	project.JobConfig.BonusPoints = bonus
	project.UpdatedAt = time.Now()
	raw, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return 0, err
	}
	path := filepath.Join(a.getProjectsDir(), project.ID+".json")
	tmp := path + ".rules-tmp"
	if err = os.WriteFile(tmp, raw, 0600); err != nil {
		return 0, err
	}
	if err = os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	a.syncRecruitmentProject(project)
	return version, nil
}
