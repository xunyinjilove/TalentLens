package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// InterviewSession 保存候选人的真实作答，与简历分析生成的参考答案分开。
type InterviewSession struct {
	ResumeID         string          `json:"resume_id"`
	CandidateName    string          `json:"candidate_name"`
	Status           string          `json:"status"` // in_progress | completed | reviewed
	Turns            []InterviewTurn `json:"turns"`
	OverallScore     int             `json:"overall_score"`
	AISummary        string          `json:"ai_summary,omitempty"`
	AIRecommendation string          `json:"ai_recommendation,omitempty"`
	HRDecision       string          `json:"hr_decision,omitempty"`
	HRNote           string          `json:"hr_note,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type InterviewTurn struct {
	Category       string `json:"category"`
	Standard       bool   `json:"standard,omitempty"` // 同岗位候选人共用题，便于横向比较
	RuleID         string `json:"rule_id,omitempty"`
	Criterion      string `json:"criterion,omitempty"`
	Question       string `json:"question"`
	Answer         string `json:"answer"`
	FollowUp       string `json:"follow_up,omitempty"`
	FollowUpAnswer string `json:"follow_up_answer,omitempty"`
	Score          int    `json:"score"`
	Assessment     string `json:"assessment,omitempty"`
	Evidence       string `json:"evidence,omitempty"`
	Evaluated      bool   `json:"evaluated"`
}

var interviewIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (a *App) interviewPath(resumeID string) (string, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return "", fmt.Errorf("简历 ID 无效")
	}
	return filepath.Join(a.getDataDir(), "interviews", resumeID+".json"), nil
}

func (a *App) loadInterview(resumeID string) (*InterviewSession, error) {
	path, err := a.interviewPath(resumeID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session InterviewSession
	if err = json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	if session.ResumeID != resumeID {
		return nil, fmt.Errorf("初面会话 ID 不匹配")
	}
	return &session, nil
}

func (a *App) saveInterview(session *InterviewSession) error {
	path, err := a.interviewPath(session.ResumeID)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	session.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (a *App) GetInterview(resumeID string) (*InterviewSession, error) {
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	return a.loadInterview(resumeID)
}

func (a *App) StartInterview(resumeID string) (*InterviewSession, error) {
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	if existing, err := a.loadInterview(resumeID); err == nil {
		return existing, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	path, err := a.interviewPath(resumeID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
	if err != nil {
		return nil, err
	}
	var resume Resume
	if err = json.Unmarshal(data, &resume); err != nil {
		return nil, err
	}
	if resume.Analysis == nil {
		return nil, fmt.Errorf("请先完成简历初筛，以生成初面题目")
	}
	turns := make([]InterviewTurn, 0, len(resume.Analysis.InterviewQA)+8)
	project := a.GetProject(resume.ProjectID)
	if project != nil {
		turns = append(turns,
			InterviewTurn{Category: "岗位统一题", Standard: true, Question: "请介绍一个与“" + project.JobConfig.Title + "”直接相关的完整项目：说明本人职责、关键行动、结果及可核实依据。"},
			InterviewTurn{Category: "岗位统一题", Standard: true, Question: "请介绍一次与该岗位核心工作有关的困难或失误：您如何定位原因、采取什么措施、结果如何？"},
		)
	}
	for _, question := range resume.Analysis.InterviewQA {
		if strings.TrimSpace(question.Question) != "" {
			turns = append(turns, InterviewTurn{Category: question.Category, Question: question.Question})
		}
	}
	// 1. 将有关岗位重点要求（包括原红线与加分项内容）直接纳入初试核查问题
	seenCriterion := make(map[string]bool)
	if project != nil {
		for _, q := range project.JobConfig.GetInterviewQuestions() {
			q = strings.TrimSpace(q)
			if q != "" && !seenCriterion[q] {
				seenCriterion[q] = true
				turns = append(turns, InterviewTurn{
					Category:  "初试重点核查",
					Criterion: q,
					Question:  fmt.Sprintf("针对岗位重点关注项【%s】，请详细说明您的实际经历、掌握程度与交付成果：", q),
				})
			}
		}
	}
	for _, check := range resume.Analysis.RedLineChecks {
		criterion := strings.TrimSpace(check.Criterion)
		if criterion != "" && !seenCriterion[criterion] {
			seenCriterion[criterion] = true
			ruleID := check.RuleID
			if ruleID == "" {
				ruleID = redLineRuleID(criterion)
			}
			turns = append(turns, InterviewTurn{
				Category:  "初试重点核查",
				RuleID:    ruleID,
				Criterion: criterion,
				Question:  fmt.Sprintf("针对岗位重点要求【%s】，请说明并举例证明您的实际经历：", criterion),
			})
		}
	}
	for _, check := range resume.Analysis.BonusChecks {
		criterion := strings.TrimSpace(check.Criterion)
		if criterion != "" && !seenCriterion[criterion] {
			seenCriterion[criterion] = true
			ruleID := check.RuleID
			if ruleID == "" {
				ruleID = redLineRuleID(criterion)
			}
			turns = append(turns, InterviewTurn{
				Category:  "初试重点核查",
				RuleID:    ruleID,
				Criterion: criterion,
				Question:  fmt.Sprintf("针对岗位关注的业务亮点【%s】，请介绍您相关的实战经验或成果：", criterion),
			})
		}
	}
	// 2. 核心能力实战核查（围绕岗位统一雷达指标）
	if len(resume.Analysis.InterviewQA) == 0 && project != nil {
		metrics := getUnifiedJobMetrics(&project.JobConfig)
		for i, m := range metrics {
			if i >= 3 || len(turns) >= 10 {
				break
			}
			turns = append(turns, InterviewTurn{Category: "核心能力实战", Question: fmt.Sprintf("请结合您最近的实战项目，说明您在【%s】方面的实际交付成果与独立承担范围？", m)})
		}
	}
	name := resume.FileName
	if resume.Analysis.CandidateName != "" {
		name = resume.Analysis.CandidateName
	}
	session := &InterviewSession{ResumeID: resumeID, CandidateName: name, Status: "in_progress", Turns: turns, CreatedAt: time.Now()}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err = a.saveInterview(session); err != nil {
		return nil, err
	}
	return session, nil
}

func interviewJSON(raw string, value interface{}) error {
	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return fmt.Errorf("AI 初面返回的内容不是 JSON")
	}
	return json.Unmarshal([]byte(raw[start:end+1]), value)
}

func (a *App) evaluateInterviewTurn(session *InterviewSession, index int, allowFollowUp bool) error {
	turn := &session.Turns[index]
	if a.config.AI.APIKey == "" {
		return fmt.Errorf("请先配置 AI API Key")
	}
	prompt := "你是结构化初面评估员。只依据候选人真实回答判断，不把参考答案当作候选人的回答。评分锚点：0 无回答、25 泛泛了解、50 参与过、75 独立完成并说明方法、100 主导并复盘结果。红线题只用于核实，不计入能力平均分。只返回 JSON: {\"score\":0到100的整数,\"assessment\":\"具体评价\",\"evidence\":\"回答中的原文依据\",\"follow_up_question\":\"如需核实则给一道简短追问，否则为空\"}。\n\n---\n\n" +
		fmt.Sprintf("问题：%s\n候选人回答：%s\n", turn.Question, turn.Answer)
	if turn.FollowUp != "" {
		prompt += fmt.Sprintf("追问：%s\n追问回答：%s\n", turn.FollowUp, turn.FollowUpAnswer)
	}
	if !allowFollowUp {
		prompt += "本题已经追问过，不要再追问；follow_up_question 必须为空。"
	}
	raw, err := a.callAI(&a.config.AI, prompt)
	if err != nil {
		return err
	}
	var result struct {
		Score      int    `json:"score"`
		Assessment string `json:"assessment"`
		Evidence   string `json:"evidence"`
		FollowUp   string `json:"follow_up_question"`
	}
	if err = interviewJSON(raw, &result); err != nil {
		return err
	}
	if strings.TrimSpace(result.Assessment) == "" {
		return fmt.Errorf("AI 初面未返回评价")
	}
	if allowFollowUp && strings.TrimSpace(result.FollowUp) != "" {
		turn.FollowUp = strings.TrimSpace(result.FollowUp)
		return nil
	}
	turn.Score = int(math.Max(0, math.Min(100, float64(result.Score))))
	turn.Assessment, turn.Evidence, turn.Evaluated = result.Assessment, result.Evidence, true
	return nil
}

func (a *App) SubmitInterviewAnswer(resumeID string, index int, answer string) (*InterviewSession, error) {
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	session, err := a.loadInterview(resumeID)
	if err != nil {
		return nil, err
	}
	if session.Status != "in_progress" || index < 0 || index >= len(session.Turns) {
		return nil, fmt.Errorf("初面阶段或题号无效")
	}
	for i := 0; i < index; i++ {
		if !session.Turns[i].Evaluated {
			return nil, fmt.Errorf("请按顺序完成题目")
		}
	}
	turn := &session.Turns[index]
	answer = strings.TrimSpace(answer)
	if answer == "" || len([]rune(answer)) > 4000 {
		return nil, fmt.Errorf("回答不能为空且不能超过 4000 字")
	}
	if turn.Answer != "" && turn.Answer != answer {
		return nil, fmt.Errorf("该题已提交，不能覆盖原始回答")
	}
	if turn.Evaluated || turn.FollowUp != "" {
		return nil, fmt.Errorf("该题已进入追问或完成阶段")
	}
	turn.Answer = answer
	if err = a.saveInterview(session); err != nil {
		return nil, err
	}
	if err = a.evaluateInterviewTurn(session, index, true); err != nil {
		return session, err
	}
	return session, a.saveInterview(session)
}

func (a *App) SubmitInterviewFollowUp(resumeID string, index int, answer string) (*InterviewSession, error) {
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	session, err := a.loadInterview(resumeID)
	if err != nil {
		return nil, err
	}
	if session.Status != "in_progress" || index < 0 || index >= len(session.Turns) {
		return nil, fmt.Errorf("初面阶段或题号无效")
	}
	turn := &session.Turns[index]
	answer = strings.TrimSpace(answer)
	if turn.FollowUp == "" || turn.Evaluated || answer == "" || len([]rune(answer)) > 4000 {
		return nil, fmt.Errorf("追问状态或回答无效")
	}
	if turn.FollowUpAnswer != "" && turn.FollowUpAnswer != answer {
		return nil, fmt.Errorf("追问回答已提交，不能覆盖")
	}
	turn.FollowUpAnswer = answer
	if err = a.saveInterview(session); err != nil {
		return nil, err
	}
	if err = a.evaluateInterviewTurn(session, index, false); err != nil {
		return session, err
	}
	return session, a.saveInterview(session)
}

func (a *App) CompleteInterview(resumeID string) (*InterviewSession, error) {
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	session, err := a.loadInterview(resumeID)
	if err != nil {
		return nil, err
	}
	if session.Status == "completed" || session.Status == "reviewed" {
		return session, nil
	}
	if len(session.Turns) == 0 {
		return nil, fmt.Errorf("初面题目为空")
	}
	var transcript strings.Builder
	for i := range session.Turns {
		turn := &session.Turns[i]
		if !turn.Evaluated && strings.TrimSpace(turn.Answer) != "" {
			_ = a.evaluateInterviewTurn(session, i, false)
		}
		if !turn.Evaluated && strings.TrimSpace(turn.Answer) == "" {
			turn.Score = 0
			turn.Assessment = "候选人未对该题作答"
			turn.Evaluated = true
		}
		fmt.Fprintf(&transcript, "问题：%s\n回答：%s\n追问：%s\n追问回答：%s\n单题评价：%s\n", turn.Question, turn.Answer, turn.FollowUp, turn.FollowUpAnswer, turn.Assessment)
	}
	session.OverallScore = scoreInterviewAbility(session)
	if a.config.AI.APIKey == "" {
		return nil, fmt.Errorf("请先配置 AI API Key")
	}
	prompt := "你是初面总结员。仅基于候选人真实回答给 HR 一份简短、可复核的结论。只返回 JSON: {\"summary\":\"优势、疑点及待人工核验点\",\"recommendation\":\"advance/review/decline\"}。不要根据推断直接作录用决定。\n\n---\n\n" + transcript.String()
	raw, err := a.callAI(&a.config.AI, prompt)
	if err != nil {
		return nil, err
	}
	var result struct {
		Summary        string `json:"summary"`
		Recommendation string `json:"recommendation"`
	}
	if err = interviewJSON(raw, &result); err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Summary) == "" {
		return nil, fmt.Errorf("AI 未返回初面总结")
	}
	session.AISummary, session.AIRecommendation, session.Status = result.Summary, result.Recommendation, "completed"
	if session.AIRecommendation != "advance" && session.AIRecommendation != "review" && session.AIRecommendation != "decline" {
		session.AIRecommendation = "review"
	}
	for _, turn := range session.Turns {
		if turn.RuleID != "" && session.AIRecommendation == "advance" {
			session.AIRecommendation = "review"
			break
		}
	}
	if data, readErr := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json")); readErr == nil {
		var resume Resume
		if json.Unmarshal(data, &resume) == nil && resume.Analysis != nil {
			if project := a.GetProject(resume.ProjectID); project != nil {
				a.prepareResumeDecision(&resume, &project.JobConfig)
			}
			if (resume.Analysis.RedLineStatus == "pending" || resume.Analysis.RedLineStatus == "failed") && session.AIRecommendation == "advance" {
				session.AIRecommendation = "review"
			}
		}
	}
	return session, a.saveInterview(session)
}

// scoreInterviewAbility 汇总初试题目得分
func scoreInterviewAbility(session *InterviewSession) int {
	if session == nil {
		return 0
	}
	standardSum, standardCount, dynamicSum, dynamicCount := 0, 0, 0, 0
	for _, turn := range session.Turns {
		if !turn.Evaluated {
			continue
		}
		if turn.Standard {
			standardSum += turn.Score
			standardCount++
		} else {
			dynamicSum += turn.Score
			dynamicCount++
		}
	}
	if standardCount == 0 && dynamicCount == 0 {
		return 0
	}
	if standardCount == 0 {
		return int(math.Round(float64(dynamicSum) / float64(dynamicCount)))
	}
	if dynamicCount == 0 {
		return int(math.Round(float64(standardSum) / float64(standardCount)))
	}
	return int(math.Round(float64(standardSum)/float64(standardCount)*0.5 + float64(dynamicSum)/float64(dynamicCount)*0.5))
}

// ReviewInterviewRedLine 将 HR 确认的初面原话关联到具体红线（兼容历史核验）；AI 单题分不参与裁决。
func (a *App) ReviewInterviewRedLine(resumeID string, ruleID string, status string, evidence string) (*Resume, error) {
	session, err := a.GetInterview(resumeID)
	if err != nil {
		return nil, err
	}
	if session.Status != "completed" && session.Status != "reviewed" {
		return nil, fmt.Errorf("请先完成初面")
	}
	evidence = strings.TrimSpace(evidence)
	for _, turn := range session.Turns {
		if turn.RuleID != ruleID || !turn.Evaluated {
			continue
		}
		if status != "unknown" && (evidence == "" ||
			(!strings.Contains(turn.Answer, evidence) && !strings.Contains(turn.FollowUpAnswer, evidence))) {
			return nil, fmt.Errorf("核实依据必须引用该题候选人的原话")
		}
		return a.setReviewedRedLine(resumeID, turn.Criterion, status, evidence, "interview_hr_review")
	}
	return nil, fmt.Errorf("初面中没有对应的红线问题")
}

func (a *App) ReviewInterview(resumeID string, decision string, note string) (*InterviewSession, error) {
	a.interviewMutex.Lock()
	defer a.interviewMutex.Unlock()
	if decision != "advance" && decision != "hold" && decision != "decline" {
		return nil, fmt.Errorf("人工结论无效")
	}
	session, err := a.loadInterview(resumeID)
	if err != nil {
		return nil, err
	}
	if session.Status != "completed" && session.Status != "reviewed" {
		return nil, fmt.Errorf("请先完成 AI 初面")
	}
	if decision == "advance" {
		data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
		if err != nil {
			return nil, err
		}
		var resume Resume
		if err := json.Unmarshal(data, &resume); err != nil {
			return nil, err
		}
		if project := a.GetProject(resume.ProjectID); project != nil {
			a.prepareResumeDecision(&resume, &project.JobConfig)
		}
		if resume.Analysis != nil && (resume.Analysis.RedLineStatus == "pending" || resume.Analysis.RedLineStatus == "failed") {
			return nil, fmt.Errorf("请先逐项核实岗位红线，再决定是否进入下一轮")
		}
	}
	session.HRDecision, session.HRNote, session.Status = decision, strings.TrimSpace(note), "reviewed"
	return session, a.saveInterview(session)
}
