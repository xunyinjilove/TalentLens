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
	if resume.Analysis == nil || len(resume.Analysis.InterviewQA) == 0 {
		return nil, fmt.Errorf("请先完成简历分析，生成初面题目")
	}
	turns := make([]InterviewTurn, 0, len(resume.Analysis.InterviewQA)+len(resume.Analysis.RedLineChecks))
	for _, question := range resume.Analysis.InterviewQA {
		if strings.TrimSpace(question.Question) != "" {
			turns = append(turns, InterviewTurn{Category: question.Category, Question: question.Question})
		}
	}
	for _, check := range resume.Analysis.RedLineChecks {
		if check.Status == "unknown" {
			ruleID := check.RuleID
			if ruleID == "" {
				ruleID = redLineRuleID(check.Criterion)
			}
			turns = append(turns, InterviewTurn{Category: "红线核实", RuleID: ruleID, Criterion: check.Criterion, Question: "请说明并举例证明：" + check.Criterion})
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
	prompt := "你是结构化初面评估员。只依据候选人真实回答判断，不把参考答案当作候选人的回答。只返回 JSON: {\"score\":0到100的整数,\"assessment\":\"具体评价\",\"evidence\":\"回答中的原文依据\",\"follow_up_question\":\"如需核实则给一道简短追问，否则为空\"}。\n\n---\n\n" +
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
	sum := 0
	var transcript strings.Builder
	for _, turn := range session.Turns {
		if !turn.Evaluated {
			return nil, fmt.Errorf("仍有题目未完成")
		}
		sum += turn.Score
		fmt.Fprintf(&transcript, "问题：%s\n回答：%s\n追问：%s\n追问回答：%s\n单题评价：%s\n", turn.Question, turn.Answer, turn.FollowUp, turn.FollowUpAnswer, turn.Assessment)
	}
	session.OverallScore = int(math.Round(float64(sum) / float64(len(session.Turns))))
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
	for _, turn := range session.Turns {
		if turn.RuleID != "" && session.AIRecommendation == "advance" {
			session.AIRecommendation = "review"
			break
		}
	}
	return session, a.saveInterview(session)
}

// ReviewInterviewRedLine 将 HR 确认的初面原话关联到具体红线；AI 单题分不参与裁决。
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
		if resume.Analysis != nil && (resume.Analysis.RedLineStatus == "pending" || resume.Analysis.RedLineStatus == "failed") {
			return nil, fmt.Errorf("请先逐项核实岗位红线，再决定是否进入下一轮")
		}
	}
	session.HRDecision, session.HRNote, session.Status = decision, strings.TrimSpace(note), "reviewed"
	return session, a.saveInterview(session)
}
