package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HROutlineQuestion HR 初试题目定义与现场记录
type HROutlineQuestion struct {
	QuestionID      string   `json:"question_id"`               // e.g. "q1_motivation"
	Category        string   `json:"category"`                  // "intention" | "competence" | "confirmation"
	CategoryLabel   string   `json:"category_label"`            // "工作意向" | "工作能力" | "双向确认"
	Topic           string   `json:"topic"`                     // e.g. "求职动机与选择标准"
	Question        string   `json:"question"`                  // 口语化主问题
	ReasonToAsk     string   `json:"reason_to_ask"`             // 为什么问
	SourceRefs      string   `json:"source_refs,omitempty"`     // 简历线索，无则"本次沟通确认"
	FollowUpHints   []string `json:"follow_up_hints"`           // 条件追问（1-2条）
	HRObservePoints []string `json:"hr_observe_points"`         // 重点听什么（HR 观察点）
	RecordFields    string   `json:"record_fields"`             // 建议记录内容
	RelatedChecks   []string `json:"related_checks,omitempty"`  // 关联的岗位关注要求
	HRVerdict       string   `json:"hr_verdict,omitempty"`      // "pass" | "pending" | "gap" | "unanswered"
	HRNotes         string   `json:"hr_notes,omitempty"`        // HR 现场记录/候选人原话摘要
	SavedAt         string   `json:"saved_at,omitempty"`        // 记录保存时间
}

// InterviewOutline 独立持久化的 HR 初试提纲与记录
type InterviewOutline struct {
	ResumeID      string              `json:"resume_id"`
	ProjectID     string              `json:"project_id"`
	CandidateName string              `json:"candidate_name"`
	JobTitle      string              `json:"job_title"`
	PriorityFocus []string            `json:"priority_focus"` // 本次初试优先关注点（3-4项）
	Questions     []HROutlineQuestion `json:"questions"`
	Version       int                 `json:"version"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// outlinePath 获取指定简历的 HR 初试提纲持久化文件路径
func (a *App) outlinePath(resumeID string) (string, error) {
	if !interviewIDPattern.MatchString(resumeID) {
		return "", fmt.Errorf("简历 ID 无效")
	}
	return filepath.Join(a.getDataDir(), "interviews", resumeID+"_outline.json"), nil
}

// BuildDefaultHROutline 基于本方案标准 10 题与岗位背景组装提纲
func (a *App) BuildDefaultHROutline(candidateName string, jobCfg *JobConfig, resume *Resume) *InterviewOutline {
	jobTitle := "目标岗位"
	if jobCfg != nil && strings.TrimSpace(jobCfg.Title) != "" {
		jobTitle = jobCfg.Title
	}

	name := candidateName
	if strings.TrimSpace(name) == "" && resume != nil {
		if resume.Analysis != nil && strings.TrimSpace(resume.Analysis.CandidateName) != "" {
			name = resume.Analysis.CandidateName
		} else {
			name = strings.TrimSuffix(resume.FileName, filepath.Ext(resume.FileName))
		}
	}
	if strings.TrimSpace(name) == "" {
		name = "候选人"
	}

	// 提取简历线索
	currentRoleClue := "本次沟通现场确认"
	expClue := "本次沟通现场确认"
	if resume != nil && resume.Analysis != nil {
		if resume.Analysis.CurrentRole != "" {
			currentRoleClue = fmt.Sprintf("据简历现任：%s", resume.Analysis.CurrentRole)
		}
		if resume.Analysis.WorkYears != "" {
			expClue = fmt.Sprintf("据简历经验：%s", resume.Analysis.WorkYears)
		}
	}

	// 提取岗位关注项
	var relatedQ5, relatedQ6, relatedQ8 []string
	if jobCfg != nil {
		questions := jobCfg.GetInterviewQuestions()
		for i, q := range questions {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			if i%3 == 0 {
				relatedQ5 = append(relatedQ5, q)
			} else if i%3 == 1 {
				relatedQ6 = append(relatedQ6, q)
			} else {
				relatedQ8 = append(relatedQ8, q)
			}
		}
	}

	// 组装 10 个主问题
	questions := []HROutlineQuestion{
		// 3.1 工作意向（4 个主问题）
		{
			QuestionID:    "q1_motivation",
			Category:      "intention",
			CategoryLabel: "工作意向",
			Topic:         "求职动机与选择标准",
			Question:      "这次考虑新机会，您最希望下一份工作带来什么变化？",
			ReasonToAsk:   "了解候选人真实的求职/换工作动机，以及选择新机会时最在意的排序。",
			SourceRefs:    currentRoleClue,
			FollowUpHints: []string{
				"只说“寻求更好发展空间”时，追问希望增加哪一类具体职责或业务机会；",
				"如果提及多个原因，请其说明其中最重要的两项。",
			},
			HRObservePoints: []string{
				"关注离职/换工作真实原因，以及选择机会的优先级排序；",
				"希望改变的具体事项是否合理，本岗位能否真实回应其诉求；",
				"合理的薪资或发展诉求不作为能力不足的证据，重点在于双方期望能否对齐。",
			},
			RecordFields: "记录候选人原话、主要选择条件及尚未明确的部分",
			HRVerdict:    "unanswered",
		},
		{
			QuestionID:    "q2_role_preference",
			Category:      "intention",
			CategoryLabel: "工作意向",
			Topic:         "工作内容与发展方向",
			Question:      fmt.Sprintf("在“%s”岗位上，您最希望主要负责哪些事情？", jobTitle),
			ReasonToAsk:   "确认候选人期待承担的工作内容与岗位实际工作职责是否一致。",
			SourceRefs:    expClue,
			FollowUpHints: []string{
				"希望延续还是调整当前的工作方向；",
				"偏好独立执行、负责完整交付任务，还是更倾向带团队协作；",
				"与本岗位实际职责相比，有哪些需要进一步了解的地方。",
			},
			HRObservePoints: []string{
				"候选人想做的内容与岗位实际工作是否一致；",
				"是否有明确不希望承担的内容；",
				"先向候选人客观说明岗位真实职责再确认差异，不凭岗位名称相同就认定意向一致。",
			},
			RecordFields: "期待承担的工作、明确不希望承担的内容、对岗位的疑问",
			HRVerdict:    "unanswered",
		},
		{
			QuestionID:    "q3_conditions",
			Category:      "intention",
			CategoryLabel: "工作意向",
			Topic:         "薪资、地点与工作安排",
			Question:      "选择新机会时，薪资待遇、工作地点和工作安排中，哪些条件对您最重要？",
			ReasonToAsk:   "核实候选人核心诉求能否对齐岗位实际提供的条件边界与可协调空间。",
			SourceRefs:    "本次沟通现场确认",
			FollowUpHints: []string{
				"逐项确认尚未明确的薪资期望口径与工作地点接受度；",
				"岗位已有明确薪资/出差范围时，询问与预期是否存在差异及可协商空间。",
			},
			HRObservePoints: []string{
				"期望薪资的币种、周期与税前/税后口径是否清晰；",
				"工作地点、通勤方式及岗位确有要求的出差/加班安排能否接受；",
				"薪资口径未统一时先标待确认；岗位未提供的信息标待补充，不随意承诺或武断判定不匹配。",
			},
			RecordFields: "期望薪资口径、地点接受度、特殊工作安排及待协调项",
			HRVerdict:    "unanswered",
		},
		{
			QuestionID:    "q4_timeline",
			Category:      "intention",
			CategoryLabel: "工作意向",
			Topic:         "求职进度与到岗时间",
			Question:      "如果双方沟通合适，您大约何时可以到岗？目前是否有交接安排？",
			ReasonToAsk:   "确认可执行的入职推进时间与双方需要协调的事项节奏。",
			SourceRefs:    "本次沟通现场确认",
			FollowUpHints: []string{
				"目前是否还在职，是否有交接周期或其他离职手续安排；",
				"是否存在影响本次推进节奏的关键时间点，是否方便继续安排下一轮面试。",
			},
			HRObservePoints: []string{
				"最早到岗日期/范围、前置交接条件与推进节奏；",
				"到岗较晚不等于意愿低；“月底”、“一个月后”以本次沟通日期为基准确认，不随意猜测精确日期；",
				"候选人自愿说明其他机会进度时，只记录必要时间信息。",
			},
			RecordFields: "最早到岗时间范围、离职交接约束、下一步方便沟通的时间点",
			HRVerdict:    "unanswered",
		},

		// 3.2 工作能力（5 个主问题）
		{
			QuestionID:    "q5_responsibility",
			Category:      "competence",
			CategoryLabel: "工作能力",
			Topic:         "实际职责与独立负责范围",
			Question:      "请介绍最近一份工作中，您最主要负责的一项工作。",
			ReasonToAsk:   "核实候选人实际承担职责，区分参与协助、独立负责推进还是带领他人。",
			SourceRefs:    currentRoleClue,
			FollowUpHints: []string{
				"这项工作由谁发起；本人具体负责哪一段；哪些事情能独立决定；",
				"与团队其他人的分工界面是什么，最终交付物是什么。",
			},
			HRObservePoints: []string{
				"候选人能否讲清具体做过的事，清晰区分参与、独立负责与带领他人；",
				"不能把团队整体成果直接视为个人成果，也不要求每位候选人都有管理职责；",
				"提及的专业工具作为场景线索记录，专业深度留给下一轮技术/专业面试确认。",
			},
			RecordFields:  "任务场景、本人职责、协作对象与实际交付产出",
			RelatedChecks: relatedQ5,
			HRVerdict:     "unanswered",
		},
		{
			QuestionID:    "q6_delivery",
			Category:      "competence",
			CategoryLabel: "工作能力",
			Topic:         "计划安排与交付质量",
			Question:      "请举一项有明确截止时间的任务，说明您如何安排并完成交付。",
			ReasonToAsk:   "考察工作计划性、任务拆解、主动同步意识与保质交付的闭环能力。",
			SourceRefs:    "本次沟通现场确认",
			FollowUpHints: []string{
				"如何排优先级；如何确认进度正常；如何验收交付质量；",
				"发现可能延期或出现偏差时，本人采取了什么行动与补救措施。",
			},
			HRObservePoints: []string{
				"是否有清晰计划、主动同步节点与完成验收标准，能否把事情推进到闭环交付；",
				"销售、运营、行政等工作同样可用日常业务或客户事项作案例，不强制包装技术项目；",
				"关注结果是否可核实。",
			},
			RecordFields:  "时间要求、关键推进步骤、质量检查方式与实际交付结果",
			RelatedChecks: relatedQ6,
			HRVerdict:     "unanswered",
		},
		{
			QuestionID:    "q7_problem_solving",
			Category:      "competence",
			CategoryLabel: "工作能力",
			Topic:         "解决问题与承担责任",
			Question:      "请讲一次工作没有按预期推进的情况，您当时做了什么？",
			ReasonToAsk:   "考察面对阻碍与意外时的排查行动、复盘改进态度与主动担责意识。",
			SourceRefs:    "本次沟通现场确认",
			FollowUpHints: []string{
				"当时首先如何定位原因；本人采取了什么行动；",
				"何时寻求外部帮助或向上同步；最后有什么变化或复盘经验。",
			},
			HRObservePoints: []string{
				"如实说明失误不自动扣分，关注能否讲清问题根因、具体行动与责任边界；",
				"是否主动处理并及时反馈，而非被动等待或推卸责任；",
				"只谈客观困难或归咎他人时，追问本人具体做了什么，不先贴主观态度标签。",
			},
			RecordFields: "问题起因、排查行动、补救结果与事后改进复盘",
			HRVerdict:    "unanswered",
		},
		{
			QuestionID:    "q8_collaboration",
			Category:      "competence",
			CategoryLabel: "工作能力",
			Topic:         "沟通协作与分歧处理",
			Question:      "请举一次需要与其他同事或部门一起完成工作的经历。",
			ReasonToAsk:   "考察跨部门/跨团队协同、推进共识、表达沟通与化解分歧的能力。",
			SourceRefs:    "本次沟通现场确认",
			FollowUpHints: []string{
				"各方目标是否一致；遇到意见分歧或利益冲突时如何沟通；",
				"如何推动对方配合，什么时候需要升级向上协调。",
			},
			HRObservePoints: []string{
				"能否理解协作对象的诉求与痛点，清晰说明我方诉求并推进共识；",
				"表达流畅与工作协作能力分别观察，不能只凭口才下定论；",
				"不存在激烈冲突的日常案例也可以说明协作方法与推进节奏。",
			},
			RecordFields:  "协作背景、分歧或痛点、沟通推进过程与最终协同成果",
			RelatedChecks: relatedQ8,
			HRVerdict:     "unanswered",
		},
		{
			QuestionID:    "q9_learning",
			Category:      "competence",
			CategoryLabel: "工作能力",
			Topic:         "学习与适应变化",
			Question:      "最近一次接手不熟悉的任务时，您是怎样学会并完成它的？",
			ReasonToAsk:   "考察主动学习方法、任务拆解速度与适应新业务/新工具的能力。",
			SourceRefs:    "本次沟通现场确认",
			FollowUpHints: []string{
				"如何拆解新任务、查找资料或向专业人员请教；",
				"怎样判断自己已经掌握，是否实际应用到了业务交付中。",
			},
			HRObservePoints: []string{
				"是否有主动学习与验证方法，能否快速适应职责或业务环境的变化；",
				"允许使用工作、实习或其他实践经历作答；",
				"不要求候选人背诵工具底层原理或现场答技术考试题。",
			},
			RecordFields: "原有知识底子、新任务难点、学习验证行动与实际应用效果",
			HRVerdict:    "unanswered",
		},

		// 3.3 双向确认（1 个主问题）
		{
			QuestionID:    "q10_next_steps",
			Category:      "confirmation",
			CategoryLabel: "双向确认",
			Topic:         "岗位理解与下一步",
			Question:      fmt.Sprintf("了解目前介绍的“%s”职责和条件后，您最想进一步确认什么？", jobTitle),
			ReasonToAsk:   "双向解答候选人关切，建立对岗位的真实预期，并约定下一步推进动作。",
			SourceRefs:    "本次沟通现场确认",
			FollowUpHints: []string{
				"本岗位是否仍符合您的求职考虑方向；",
				"如果双方愿意继续，确认下一轮面试时间与需要补充的证明材料。",
			},
			HRObservePoints: []string{
				"双方信息是否充分对称，候选人最关心的核心关切点是什么；",
				"未当场确认下一轮不自动判为拒绝，招聘方未答清的问题应列入我方待办；",
				"明确约定下一步跟进动作与时间节点。",
			},
			RecordFields: "候选人疑问、HR 待答事项、继续沟通意向与下一步约定动作",
			HRVerdict:    "unanswered",
		},
	}

	// 组装优先关注点（3-4项）
	priority := []string{
		"期望职责方向、薪资待遇与到岗时间确认",
		"实际承担职责、独立负责范围与交付案例核验",
		"跨部门沟通协作经历与分歧化解案例",
	}
	if jobCfg != nil && len(jobCfg.GetInterviewQuestions()) > 0 {
		priority = append(priority, fmt.Sprintf("岗位重点关注要求：%s", strings.Join(jobCfg.GetInterviewQuestions()[:minInt(2, len(jobCfg.GetInterviewQuestions()))], "；")))
	}

	projectID := ""
	if resume != nil {
		projectID = resume.ProjectID
	}

	return &InterviewOutline{
		ResumeID:      "",
		ProjectID:     projectID,
		CandidateName: name,
		JobTitle:      jobTitle,
		PriorityFocus: priority,
		Questions:     questions,
		Version:       1,
		UpdatedAt:     time.Now(),
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetHROutline 获取候选人的 HR 初试提纲与记录（如无持久化文件则使用本地标准模板自动创建）
func (a *App) GetHROutline(resumeID string) (*InterviewOutline, error) {
	path, err := a.outlinePath(resumeID)
	if err != nil {
		return nil, err
	}

	// 如果存在已保存的提纲，直接读取返回
	if data, readErr := readJSONFile(path); readErr == nil {
		var outline InterviewOutline
		if jsonErr := json.Unmarshal(data, &outline); jsonErr == nil && len(outline.Questions) > 0 {
			return &outline, nil
		}
	}

	// 读取简历与项目上下文
	var resume *Resume
	resumeData, rErr := readJSONFile(filepath.Join(a.getDataDir(), "resumes", resumeID+".json"))
	if rErr == nil {
		var r Resume
		if json.Unmarshal(resumeData, &r) == nil {
			resume = &r
		}
	}

	var jobCfg *JobConfig
	candidateName := "候选人"
	if resume != nil {
		if resume.Analysis != nil && resume.Analysis.CandidateName != "" {
			candidateName = resume.Analysis.CandidateName
		} else {
			candidateName = strings.TrimSuffix(resume.FileName, filepath.Ext(resume.FileName))
		}
		if resume.ProjectID != "" {
			if p := a.GetProject(resume.ProjectID); p != nil {
				jobCfg = &p.JobConfig
			}
		}
	}

	outline := a.BuildDefaultHROutline(candidateName, jobCfg, resume)
	outline.ResumeID = resumeID
	if resume != nil {
		outline.ProjectID = resume.ProjectID
	}

	// 自动持久化落盘
	_ = a.SaveHROutline(outline)
	return outline, nil
}

// SaveHROutline 保存整份 HR 初试提纲与记录
func (a *App) SaveHROutline(outline *InterviewOutline) error {
	if outline == nil || strings.TrimSpace(outline.ResumeID) == "" {
		return fmt.Errorf("初试提纲简历 ID 不能为空")
	}
	path, err := a.outlinePath(outline.ResumeID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	outline.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// SaveHROutlineQuestion 保存单个题目的 HR 判断与记录
func (a *App) SaveHROutlineQuestion(resumeID string, questionID string, verdict string, notes string) (*InterviewOutline, error) {
	outline, err := a.GetHROutline(resumeID)
	if err != nil {
		return nil, err
	}

	found := false
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	for i := range outline.Questions {
		if outline.Questions[i].QuestionID == questionID {
			outline.Questions[i].HRVerdict = verdict
			outline.Questions[i].HRNotes = strings.TrimSpace(notes)
			outline.Questions[i].SavedAt = nowStr
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("未找到对应问题: %s", questionID)
	}

	if err := a.SaveHROutline(outline); err != nil {
		return nil, err
	}
	return outline, nil
}

// ResetHROutline 重置初试提纲为系统标准版本
func (a *App) ResetHROutline(resumeID string) (*InterviewOutline, error) {
	path, err := a.outlinePath(resumeID)
	if err == nil {
		_ = os.Remove(path)
	}
	return a.GetHROutline(resumeID)
}
