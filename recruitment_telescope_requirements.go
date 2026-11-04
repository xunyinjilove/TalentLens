package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type TelescopeCondition struct {
	Text     string `json:"text"`
	Kind     string `json:"kind"` // skill | other
	Required bool   `json:"required"`
	Source   string `json:"source"` // job | custom
}

type TelescopeConditionConfig struct {
	ProjectID        string               `json:"projectId"`
	Conditions       []TelescopeCondition `json:"conditions"`
	Version          string               `json:"version"`
	JobSkillsChanged bool                 `json:"jobSkillsChanged"`
	JobSkills        []string             `json:"jobSkills"`
}

func telescopeHash(value interface{}) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16]
}

func (a *App) GetTelescopeConditionConfig(projectID string) (*TelescopeConditionConfig, error) {
	project := a.GetProject(projectID)
	if project == nil {
		return nil, fmt.Errorf("项目不存在")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	config := &TelescopeConditionConfig{ProjectID: projectID, Conditions: []TelescopeCondition{}, JobSkills: append([]string(nil), project.JobConfig.RequiredSkills...)}
	var raw, skillsHash string
	err = db.QueryRow(`SELECT conditions_json,job_skills_hash FROM telescope_conditions WHERE project_id=?`, projectID).Scan(&raw, &skillsHash)
	if err == sql.ErrNoRows {
		for _, skill := range config.JobSkills {
			if strings.TrimSpace(skill) != "" {
				config.Conditions = append(config.Conditions, TelescopeCondition{Text: strings.TrimSpace(skill), Kind: "skill", Required: true, Source: "job"})
			}
		}
	} else if err != nil {
		return nil, err
	} else if err = json.Unmarshal([]byte(raw), &config.Conditions); err != nil {
		return nil, err
	}
	config.Version = telescopeHash(config.Conditions)
	config.JobSkillsChanged = skillsHash != "" && skillsHash != telescopeHash(config.JobSkills)
	return config, nil
}

func (a *App) SaveTelescopeConditionConfig(config TelescopeConditionConfig) (*TelescopeConditionConfig, error) {
	return a.saveTelescopeConditionConfig(config, false)
}

func (a *App) saveTelescopeConditionConfig(config TelescopeConditionConfig, syncJobSkills bool) (*TelescopeConditionConfig, error) {
	project := a.GetProject(config.ProjectID)
	if project == nil {
		return nil, fmt.Errorf("项目不存在")
	}
	if len(config.Conditions) > 30 {
		return nil, fmt.Errorf("望远镜条件最多 30 项")
	}
	clean := make([]TelescopeCondition, 0, len(config.Conditions))
	seen := map[string]bool{}
	for _, item := range config.Conditions {
		item.Text = strings.TrimSpace(item.Text)
		if item.Text == "" {
			continue
		}
		if len([]rune(item.Text)) > 120 {
			return nil, fmt.Errorf("单项条件不能超过 120 字")
		}
		if item.Kind != "skill" && item.Kind != "other" {
			return nil, fmt.Errorf("条件类型无效")
		}
		if item.Source != "job" {
			item.Source = "custom"
		}
		key := item.Kind + ":" + strings.ToLower(item.Text)
		if seen[key] {
			continue
		}
		seen[key] = true
		clean = append(clean, item)
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	jobSkillsHash := telescopeHash(project.JobConfig.RequiredSkills)
	if !syncJobSkills {
		var savedHash string
		if err := db.QueryRow(`SELECT job_skills_hash FROM telescope_conditions WHERE project_id=?`, config.ProjectID).Scan(&savedHash); err == nil && savedHash != "" {
			jobSkillsHash = savedHash
		} else if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}
	data, _ := json.Marshal(clean)
	_, err = db.Exec(`INSERT INTO telescope_conditions(project_id,conditions_json,job_skills_hash,version,updated_at) VALUES(?,?,?,?,?)
		ON CONFLICT(project_id) DO UPDATE SET conditions_json=excluded.conditions_json,job_skills_hash=excluded.job_skills_hash,version=excluded.version,updated_at=excluded.updated_at`,
		config.ProjectID, string(data), jobSkillsHash, telescopeHash(clean), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	return a.GetTelescopeConditionConfig(config.ProjectID)
}

func (a *App) SyncTelescopeJobSkills(projectID string) (*TelescopeConditionConfig, error) {
	config, err := a.GetTelescopeConditionConfig(projectID)
	if err != nil {
		return nil, err
	}
	custom := make([]TelescopeCondition, 0, len(config.Conditions))
	for _, item := range config.Conditions {
		if item.Source != "job" {
			custom = append(custom, item)
		}
	}
	config.Conditions = custom
	for _, skill := range config.JobSkills {
		if strings.TrimSpace(skill) != "" {
			config.Conditions = append(config.Conditions, TelescopeCondition{Text: strings.TrimSpace(skill), Kind: "skill", Required: true, Source: "job"})
		}
	}
	return a.saveTelescopeConditionConfig(*config, true)
}

func telescopeProfileText(raw string) string {
	return telescopeFieldsText(raw, []string{"selfintro", "work", "workexperience", "project", "education", "itskill", "pskillslabel", "pskilldetail", "certification", "train", "others", "recentwork", "highestdegree"})
}

func telescopeSkillText(raw string) string {
	return telescopeFieldsText(raw, []string{"selfintro", "work", "workexperience", "project", "itskill", "pskillslabel", "pskilldetail", "certification", "train", "recentwork"})
}

func telescopeFieldsText(raw string, fields []string) string {
	var profile map[string]interface{}
	if json.Unmarshal([]byte(raw), &profile) != nil {
		return raw
	}
	parts := make([]string, 0, len(fields))
	for _, key := range fields {
		if value, ok := profile[key]; ok && value != nil {
			encoded, _ := json.Marshal(value)
			parts = append(parts, string(encoded))
		}
	}
	return strings.Join(parts, "\n")
}

func telescopeKeywordEvidence(text, term string) (string, bool) {
	if len([]rune(term)) < 2 {
		return "", false
	}
	lower, target := strings.ToLower(text), strings.ToLower(term)
	for from := 0; from < len(lower); {
		offset := strings.Index(lower[from:], target)
		if offset < 0 {
			break
		}
		index := from + offset
		from = index + len(target)
		if term[0] < 128 && isASCIILetter(term[0]) {
			before, after := index-1, index+len(term)
			if before >= 0 && isASCIILetter(text[before]) || after < len(text) && isASCIILetter(text[after]) {
				continue
			}
		}
		prefix := string([]rune(text[max(0, index-24):index]))
		if strings.HasSuffix(prefix, "没") || strings.HasSuffix(prefix, "没有") || strings.HasSuffix(prefix, "无") || strings.HasSuffix(prefix, "未") || strings.HasSuffix(prefix, "不") || strings.HasSuffix(prefix, "缺乏") {
			continue
		}
		start, end := max(0, index-32), min(len(text), index+len(term)+32)
		return text[start:end], true
	}
	return "", false
}
func isASCIILetter(b byte) bool { return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' }

func telescopeConditionKey(item TelescopeCondition) string {
	return telescopeHash([]interface{}{item.Kind, item.Text, item.Required})
}

func (a *App) evaluateTelescopeConditions(db *sql.DB, projectID, candidateID, raw string, config *TelescopeConditionConfig) []RuleCheckItem {
	profile := telescopeProfileText(raw)
	skillProfile := telescopeSkillText(raw)
	profileHash := telescopeHash(profile)
	checks := make([]RuleCheckItem, 0, len(config.Conditions))
	pending := []int{}
	cache := func(index int) {
		item, condition := checks[index], config.Conditions[index]
		_, _ = db.Exec(`INSERT INTO telescope_requirement_checks(project_id,candidate_id,profile_hash,criteria_version,condition_key,status,evidence,source,updated_at) VALUES(?,?,?,?,?,?,?,'automatic',?)
			ON CONFLICT(project_id,candidate_id,profile_hash,criteria_version,condition_key) DO NOTHING`,
			projectID, candidateID, profileHash, config.Version, telescopeConditionKey(condition), item.Status, item.Evidence, time.Now().UTC().Format(time.RFC3339Nano))
	}
	for _, condition := range config.Conditions {
		key := telescopeConditionKey(condition)
		item := RuleCheckItem{Rule: "TELESCOPE_MUST_" + key, Label: condition.Text, Status: "unknown", Evidence: "技能证据不足"}
		if !condition.Required {
			item.Rule = "TELESCOPE_REF_" + key
		}
		var source string
		if err := db.QueryRow(`SELECT status,evidence,source FROM telescope_requirement_checks WHERE project_id=? AND candidate_id=? AND profile_hash=? AND criteria_version=? AND condition_key=?`,
			projectID, candidateID, profileHash, config.Version, key).Scan(&item.Status, &item.Evidence, &source); err == nil {
			checks = append(checks, item)
			continue
		}
		keywordSource := profile
		if condition.Kind == "skill" {
			keywordSource = skillProfile
		}
		if evidence, found := telescopeKeywordEvidence(keywordSource, condition.Text); found {
			item.Status, item.Evidence = "pass", "关键词命中："+evidence
		}
		checks = append(checks, item)
		if item.Status == "pass" {
			cache(len(checks) - 1)
		} else {
			pending = append(pending, len(checks)-1)
		}
	}
	if len(pending) > 0 && a.config.AI.APIKey != "" {
		questions := make([]map[string]interface{}, 0, len(pending))
		for _, index := range pending {
			questions = append(questions, map[string]interface{}{"index": index, "condition": config.Conditions[index].Text, "kind": config.Conditions[index].Kind})
		}
		questionJSON, _ := json.Marshal(questions)
		prompt := fmt.Sprintf("你是招聘证据核验员。仅依据真实简历内容逐项判断。返回 JSON 数组，每项 {\"index\":整数,\"status\":\"pass|fail|unknown\",\"evidence\":\"简历原文短句\"}。原文明确证明满足才 pass；明确证明不满足才 fail；未提及或无法判断用 unknown。不得把缺失视为 fail。技能须有工作、项目或技能描述证据，仅有教育专业不代表掌握。项目：%s。简历：%s", questionJSON, profile)
		if response, err := a.callAI(&a.config.AI, prompt); err == nil {
			var results []struct {
				Index    int    `json:"index"`
				Status   string `json:"status"`
				Evidence string `json:"evidence"`
			}
			start, end := strings.Index(response, "["), strings.LastIndex(response, "]")
			if start >= 0 && end > start && json.Unmarshal([]byte(response[start:end+1]), &results) == nil {
				allowed := map[int]bool{}
				for _, index := range pending {
					allowed[index] = true
				}
				for _, result := range results {
					if !allowed[result.Index] {
						continue
					}
					item := &checks[result.Index]
					quote := strings.TrimSpace(result.Evidence)
					if result.Status == "unknown" {
						cache(result.Index)
						continue
					}
					if (result.Status == "pass" || result.Status == "fail") && quote != "" && strings.Contains(profile, quote) {
						item.Status, item.Evidence = result.Status, quote
						cache(result.Index)
					}
				}
			}
		}
	}
	return checks
}

func telescopeVerdict(base string, checks []RuleCheckItem) string {
	if base == "unfit" {
		return base
	}
	unknown := base != "qualified"
	for _, item := range checks {
		if strings.HasPrefix(item.Rule, "TELESCOPE_MUST_") {
			if item.Status == "fail" {
				return "unfit"
			}
			if item.Status == "unknown" {
				unknown = true
			}
		}
	}
	if unknown {
		return "pending_verification"
	}
	return "qualified"
}

func telescopeSortByReference(items []ScreeningCandidateResult) {
	hits := func(item ScreeningCandidateResult) int {
		n := 0
		for _, c := range item.RuleEvidence {
			if strings.HasPrefix(c.Rule, "TELESCOPE_REF_") && c.Status == "pass" {
				n++
			}
		}
		return n
	}
	sort.SliceStable(items, func(i, j int) bool { return hits(items[i]) > hits(items[j]) })
}

// ConfirmTelescopeCondition records a human conclusion for the exact candidate,
// detail version and condition version shown in the preview.
func (a *App) ConfirmTelescopeCondition(batchID, candidateID, rule, status string) (*ScreeningBatchPreview, error) {
	if status != "pass" && status != "fail" {
		return nil, fmt.Errorf("只能确认符合或不符")
	}
	if !strings.HasPrefix(rule, "TELESCOPE_MUST_") && !strings.HasPrefix(rule, "TELESCOPE_REF_") {
		return nil, fmt.Errorf("不是望远镜条件")
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		return nil, err
	}
	var projectID, version, source string
	if err = db.QueryRow(`SELECT project_id,criteria_version,source FROM screening_batches WHERE id=?`, batchID).Scan(&projectID, &version, &source); err != nil || source != "telescope" {
		return nil, fmt.Errorf("望远镜批次不存在")
	}
	config, err := a.GetTelescopeConditionConfig(projectID)
	if err != nil {
		return nil, err
	}
	criteria, err := a.GetTalentManagementStrictCriteria(projectID)
	if err != nil || criteria == nil || version != telescopeHash([]string{criteria.CriteriaVersion, config.Version}) {
		return nil, fmt.Errorf("条件已变化，请重新筛选")
	}
	var id, raw, evidenceJSON string
	var detailLoaded int
	if err = db.QueryRow(`SELECT id,raw_profile_json,rule_evidence_json,detail_loaded FROM screening_candidate_results WHERE batch_id=? AND candidate_id=?`, batchID, candidateID).Scan(&id, &raw, &evidenceJSON, &detailLoaded); err != nil || detailLoaded == 0 {
		return nil, fmt.Errorf("该人详情未核验，不能人工确认")
	}
	var condition *TelescopeCondition
	for i := range config.Conditions {
		item := &config.Conditions[i]
		prefix := "TELESCOPE_MUST_"
		if !item.Required {
			prefix = "TELESCOPE_REF_"
		}
		if prefix+telescopeConditionKey(*item) == rule {
			condition = item
			break
		}
	}
	if condition == nil {
		return nil, fmt.Errorf("条件与候选人记录不一致")
	}
	profileHash := telescopeHash(telescopeProfileText(raw))
	_, err = db.Exec(`INSERT INTO telescope_requirement_checks(project_id,candidate_id,profile_hash,criteria_version,condition_key,status,evidence,source,updated_at)
		VALUES(?,?,?,?,?,?,?,'manual',?) ON CONFLICT(project_id,candidate_id,profile_hash,criteria_version,condition_key)
		DO UPDATE SET status=excluded.status,evidence=excluded.evidence,source='manual',updated_at=excluded.updated_at`,
		projectID, candidateID, profileHash, config.Version, telescopeConditionKey(*condition), status, "HR 已逐项确认", time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	var checks []RuleCheckItem
	if err = json.Unmarshal([]byte(evidenceJSON), &checks); err != nil {
		return nil, err
	}
	base := "qualified"
	for i := range checks {
		if checks[i].Rule == rule {
			checks[i].Status, checks[i].Evidence = status, "HR 已逐项确认"
		}
		if !strings.HasPrefix(checks[i].Rule, "TELESCOPE_") {
			if checks[i].Status == "fail" {
				base = "unfit"
			} else if checks[i].Status == "unknown" && base != "unfit" {
				base = "pending_verification"
			}
		}
	}
	verdict := telescopeVerdict(base, checks)
	encoded, _ := json.Marshal(checks)
	if _, err = db.Exec(`UPDATE screening_candidate_results SET verdict=?,rule_evidence_json=? WHERE id=?`, verdict, string(encoded), id); err != nil {
		return nil, err
	}
	if err = updateBatchCounts(db, batchID); err != nil {
		return nil, err
	}
	if status == "fail" && condition.Required {
		result, rejectErr := a.Run51JobTelescopeReject(batchID)
		if rejectErr != nil {
			return nil, rejectErr
		}
		for _, item := range result.Items {
			if item.CandidateID == candidateID && item.Status == "rejected" {
				var account string
				_ = db.QueryRow(`SELECT platform_account FROM screening_batches WHERE id=?`, batchID).Scan(&account)
				_, _ = db.Exec(`UPDATE project_candidates SET stage='rejected',archived_at=?,updated_at=? WHERE project_id=? AND candidate_id IN (
					SELECT candidate_id FROM candidate_sources WHERE platform='51job' AND platform_account=? AND platform_candidate_id=?)`,
					time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), projectID, account, candidateID)
			}
		}
	}
	return a.GetScreeningBatchPreview(batchID)
}
