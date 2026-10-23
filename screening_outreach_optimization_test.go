package main

import (
	"strings"
	"testing"
)

func TestLightweightScreeningStructureAndUsage(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()

	jobCfg := &JobConfig{
		Title:          "高级测试开发工程师",
		RequiredSkills: []string{"自动化测试", "CI/CD"},
		CoreCriteria:   []string{"缺陷分析"},
		RedLines:       []string{"本科以上学历"},
		BonusPoints:    []string{"性能压测经验"},
	}

	resume := &Resume{
		ID:       "res_test_light",
		FileName: "张三简历.pdf",
		Content:  "张三，本科毕业。负责自动化测试与CI/CD流水线建设，精通缺陷分析。",
	}

	prompt := app.buildAnalysisPrompt(resume, jobCfg)
	// 验证提示词已经轻量化：不包含大篇幅强制 5 道题的 interview_qa 模板要求
	if strings.Contains(prompt, "interview_qa 必须严格生成恰好 5 个问题") {
		t.Fatalf("buildAnalysisPrompt 不应包含硬性 5 道题的 interview_qa 模板要求")
	}
	if strings.Contains(prompt, "必须生成 manager_pitch") {
		t.Fatalf("buildAnalysisPrompt 不应强制要求大模型返回 manager_pitch，已改由后端程序规范生成")
	}

	// 验证兜底推介卡与防伪雷达生成能力
	analysis := &AnalysisResult{
		CandidateName: "张三",
		OverallScore:  85,
		CoreMatch:     85,
		AbilityScore:  85,
	}
	analysis.WaterCheck = app.generateFallbackWaterCheck(analysis)
	if analysis.WaterCheck == nil || analysis.WaterCheck.RiskLevel != "low" {
		t.Fatalf("generateFallbackWaterCheck 结果异常: %+v", analysis.WaterCheck)
	}
	pitch := app.buildReviewedManagerPitch(analysis, resume)
	if !strings.Contains(pitch, "张三") || !strings.Contains(pitch, "85") {
		t.Fatalf("buildReviewedManagerPitch 产出异常: %s", pitch)
	}
}

func TestPreviewBatchOutreachSupportsUnanalyzedCandidatesAndZeroAITemplate(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()

	project := app.CreateProject("前端架构师", "技术部", 1, "HR", "", &JobConfig{
		Title:          "前端架构师",
		RequiredSkills: []string{"Vue3", "TypeScript"},
	})

	// 创建一个已分析通过人选，和一个待分析 (pending) 人选
	res1 := &Resume{
		ID:        "res_done_1",
		ProjectID: project.ID,
		FileName:  "李四.pdf",
		Status:    "done",
		SourcePlatform: "51job",
		URL:       "https://ehire.51job.com/candidate/detail?id=51_done_1",
		Analysis: &AnalysisResult{
			CandidateName: "李四",
			OverallScore:  88,
			AbilityScore:  88,
			RedLineStatus: "passed",
		},
	}
	res2 := &Resume{
		ID:        "res_pending_2",
		ProjectID: project.ID,
		FileName:  "王五.pdf",
		Status:    "pending",
		SourcePlatform: "51job",
		URL:       "https://ehire.51job.com/candidate/detail?id=51_pending_2",
		Analysis:  nil, // 待分析状态！
	}

	app.saveResume(res1)
	app.saveResume(res2)
	project.ResumeIDs = []string{res1.ID, res2.ID}
	app.saveProject(project)

	// 测试全量指定候选人批量预览（使用零 AI 消耗的企业标准模板）
	batch, err := app.PreviewBatchOutreach(project.ID, []string{res1.ID, res2.ID}, "template")
	if err != nil {
		t.Fatalf("PreviewBatchOutreach 失败: %v", err)
	}

	if len(batch.Items) != 2 {
		t.Fatalf("应包含 2 位候选人，实际: %d", len(batch.Items))
	}

	// 验证待分析人选成功被纳入，且文案为零 AI 消耗的高质量模板
	var pendingItem *OutreachItem
	for i := range batch.Items {
		if batch.Items[i].ResumeID == res2.ID {
			pendingItem = &batch.Items[i]
		}
	}
	if pendingItem == nil {
		t.Fatalf("待分析人选未能纳入批量打招呼名单")
	}
	if !strings.Contains(pendingItem.Message, "前端架构师") {
		t.Fatalf("模板招呼语未正确插值岗位名称: %s", pendingItem.Message)
	}
}

func TestInterviewQuestionAssemblyFromJobCriteria(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()

	project := app.CreateProject("质量总监", "质量部", 1, "HR", "", &JobConfig{
		Title:              "质量总监",
		CoreCriteria:       []string{"GMP认证合规", "CAPA持续改进"},
		InterviewQuestions: []string{"10年以上质量管理经验", "FDA海外审计经验"},
	})

	// 轻量初筛结果：没有生成长文本 InterviewQA
	resume := &Resume{
		ID:        "res_qa_test",
		ProjectID: project.ID,
		FileName:  "赵六.pdf",
		Status:    "done",
		Analysis: &AnalysisResult{
			CandidateName: "赵六",
			OverallScore:  80,
			InterviewQA:   nil, // 空 InterviewQA！
		},
	}
	app.saveResume(resume)

	session, err := app.StartInterview(resume.ID)
	if err != nil {
		t.Fatalf("StartInterview 组装题目失败: %v", err)
	}

	if len(session.Turns) == 0 {
		t.Fatalf("初试题目不应为空")
	}

	// 验证问题中包含岗位统一题、重点要求核查题与核心能力实战题
	hasKeyRequirement := false
	hasAuditExperience := false
	hasStandard := false
	for _, turn := range session.Turns {
		if strings.Contains(turn.Question, "10年以上质量管理经验") {
			hasKeyRequirement = true
		}
		if strings.Contains(turn.Question, "FDA海外审计经验") {
			hasAuditExperience = true
		}
		if turn.Standard {
			hasStandard = true
		}
	}
	if !hasKeyRequirement {
		t.Errorf("初面题目未成功融入岗位重点核查题（10年以上质量管理经验）")
	}
	if !hasAuditExperience {
		t.Errorf("初面题目未成功融入岗位重点核查题（FDA海外审计经验）")
	}
	if !hasStandard {
		t.Errorf("初面题目未成功融入岗位统一标准题")
	}
}

func TestCompanyTalentStoreAttachmentSHA256Deduplication(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()

	content := []byte("%PDF-1.4 simulated resume content for testing SHA256 deduplication")

	att1, err := app.SaveCompanyTalentAttachment("default", "cand_123", "张三简历.pdf", content)
	if err != nil {
		t.Fatalf("保存附件失败: %v", err)
	}

	if att1.FileHash == "" || att1.LocalPath == "" {
		t.Fatalf("附件元数据不完整: %+v", att1)
	}

	// 相同字节内容再次保存，验证哈希幂等复用
	att2, err := app.SaveCompanyTalentAttachment("default", "cand_123", "张三简历_副本.pdf", content)
	if err != nil {
		t.Fatalf("重复保存附件失败: %v", err)
	}

	if att1.FileHash != att2.FileHash {
		t.Fatalf("相同文件哈希不一致: %s vs %s", att1.FileHash, att2.FileHash)
	}
	if att1.LocalPath != att2.LocalPath {
		t.Fatalf("相同文件本地存储路径未复用: %s vs %s", att1.LocalPath, att2.LocalPath)
	}

	// 批量导入记录
	batch := &CompanyTalentBatch{
		Scope:     "default",
		ProjectID: "proj_demo",
		Records: []CompanyTalentRecord{
			{
				Platform:            "51job",
				PlatformCandidateID: "51_9999",
				Name:                "孙七",
				CurrentRole:         "Java工程师",
				Attachments:         []CompanyTalentAttachment{*att1},
			},
		},
	}

	ingested, err := app.IngestCompanyTalentBatch(batch)
	if err != nil {
		t.Fatalf("批量导入公司人才库失败: %v", err)
	}

	if ingested.SuccessCount != 1 {
		t.Fatalf("成功导入数量应为 1，实际: %d", ingested.SuccessCount)
	}

	records, err := app.ListCompanyTalentRecords("default")
	if err != nil || len(records) != 1 {
		t.Fatalf("ListCompanyTalentRecords 读取异常: %d %v", len(records), err)
	}
	if records[0].Name != "孙七" || len(records[0].Attachments) != 1 {
		t.Fatalf("读取记录与导入不一致: %+v", records[0])
	}
}

func TestAIOutreachGreetingPersistenceAndReuse(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()

	project := app.CreateProject("高级Go开发", "技术部", 1, "HR", "", &JobConfig{
		Title:          "高级Go开发",
		RequiredSkills: []string{"Go", "微服务"},
	})

	res1 := &Resume{
		ID:             "res_ai_saved",
		ProjectID:      project.ID,
		FileName:       "陈八.pdf",
		Status:         "done",
		SourcePlatform: "51job",
		URL:            "https://ehire.51job.com/candidate/detail?id=51_ai_saved",
		Analysis: &AnalysisResult{
			CandidateName: "陈八",
			OverallScore:  91,
			AbilityScore:  91,
			RedLineStatus: "passed",
		},
	}
	res2 := &Resume{
		ID:             "res_ai_unsaved",
		ProjectID:      project.ID,
		FileName:       "周九.pdf",
		Status:         "done",
		SourcePlatform: "51job",
		URL:            "https://ehire.51job.com/candidate/detail?id=51_ai_unsaved",
		Analysis: &AnalysisResult{
			CandidateName: "周九",
			OverallScore:  86,
			AbilityScore:  86,
			RedLineStatus: "passed",
		},
	}
	app.saveResume(res1)
	app.saveResume(res2)
	project.ResumeIDs = []string{res1.ID, res2.ID}
	app.saveProject(project)

	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatalf("getRecruitmentDB 失败: %v", err)
	}
	customMsg := "您好陈八，看到您在Go微服务高并发架构上的实战成果非常契合我们的高级Go开发岗位，期待进一步交流！"
	saveCandidateAIOutreachMessage(db, project.ID, []string{res1.ID, res1.URL}, customMsg, "Go微服务高并发")

	// 1. 默认打开弹窗（templateMode="template"）时，已有 AI 定制记录的候选人自动优先恢复已存文案并标记 AICustomized=true
	batchDefault, err := app.PreviewBatchOutreach(project.ID, []string{res1.ID, res2.ID}, "template")
	if err != nil {
		t.Fatalf("PreviewBatchOutreach(template) 失败: %v", err)
	}
	var item1, item2 *OutreachItem
	for i := range batchDefault.Items {
		if batchDefault.Items[i].ResumeID == res1.ID {
			item1 = &batchDefault.Items[i]
		}
		if batchDefault.Items[i].ResumeID == res2.ID {
			item2 = &batchDefault.Items[i]
		}
	}
	if item1 == nil || !item1.AICustomized || item1.Message != customMsg {
		t.Fatalf("已保存 AI 定制的候选人未能自动复用持久化招呼语: %+v", item1)
	}
	if item2 == nil || item2.AICustomized {
		t.Fatalf("未定制候选人不应标记为 AICustomized: %+v", item2)
	}

	// 2. 主动点击切换到标准模板（force_template）时，临时展示模板文案，但不擦除数据库里的 AI 定制记录
	batchForceTpl, err := app.PreviewBatchOutreach(project.ID, []string{res1.ID}, "force_template")
	if err != nil {
		t.Fatalf("PreviewBatchOutreach(force_template) 失败: %v", err)
	}
	if len(batchForceTpl.Items) != 1 || batchForceTpl.Items[0].Message == customMsg {
		t.Fatalf("force_template 应返回标准模板文案，实际: %+v", batchForceTpl.Items)
	}

	// 3. 再次切换到 AI 深度定制（ai）时，已保存的候选人直接从持久化存储恢复，不重新生成覆盖
	batchAI, err := app.PreviewBatchOutreach(project.ID, []string{res1.ID}, "ai")
	if err != nil {
		t.Fatalf("PreviewBatchOutreach(ai) 失败: %v", err)
	}
	if len(batchAI.Items) != 1 || !batchAI.Items[0].AICustomized || batchAI.Items[0].Message != customMsg {
		t.Fatalf("再次进入 AI 模式未能复用之前保存的定制招呼语: %+v", batchAI.Items)
	}

	// 4. HR 手动编辑并保存单人招呼语后，下次进入依然持久化保留
	editedMsg := customMsg + "（本周可约线上初聊）"
	if err := app.SaveOutreachItemMessage(project.ID, batchAI.Items[0].ID, editedMsg); err != nil {
		t.Fatalf("SaveOutreachItemMessage 失败: %v", err)
	}
	savedMsg, _, ok := getSavedCandidateAIOutreachMessage(db, project.ID, res1.ID, res1.URL)
	if !ok || savedMsg != editedMsg {
		t.Fatalf("手动编辑后的招呼语未同步持久化: ok=%v msg=%s", ok, savedMsg)
	}
}

func TestSearchExtraKeywordsAndActivityLabelMapping(t *testing.T) {
	// 验证职位名称 + 附加关键词组合且自动去重
	combined, extra := buildEffectiveSearchKeyword("分子诊断试剂研发", "", "PCR 体外诊断, 分子诊断试剂研发  NGS")
	if combined != "分子诊断试剂研发 PCR 体外诊断 NGS" || extra != "PCR 体外诊断 NGS" {
		t.Fatalf("buildEffectiveSearchKeyword 结果不符: combined=%s extra=%s", combined, extra)
	}
	if eff, ext := buildEffectiveSearchKeyword("Java开发工程师", "Java开发工程师", ""); eff != "Java开发工程师" || ext != "" {
		t.Fatalf("空附加关键词时应保持原职位名: eff=%s ext=%s", eff, ext)
	}

	// 验证 51job 7 档活跃度双向映射
	expectedPairs := []struct {
		days  int
		label string
	}{
		{0, ""},
		{7, "近1周"},
		{14, "近2周"},
		{30, "近1个月"},
		{60, "近2个月"},
		{180, "近6个月"},
		{365, "近1年"},
		{366, "1年及以上"},
	}
	for _, pair := range expectedPairs {
		if got := normalizeActivityDays(pair.days); got != pair.days {
			t.Fatalf("normalizeActivityDays(%d) = %d", pair.days, got)
		}
		if gotLabel := activityDaysTo51JobLabel(pair.days); gotLabel != pair.label {
			t.Fatalf("activityDaysTo51JobLabel(%d) = %q, want %q", pair.days, gotLabel, pair.label)
		}
		if pair.label != "" {
			if gotDays := activityLabelToDays(pair.label); gotDays != pair.days {
				t.Fatalf("activityLabelToDays(%q) = %d, want %d", pair.label, gotDays, pair.days)
			}
		}
	}
}

func TestPreviewBatchOutreachForeignKeySelfHealing(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()

	// 1. 先初始化 SQLite（此时 migration_meta 已标记 legacy_json_v1=complete），然后关闭连接模拟重启应用后尚未触发 SQLite 的状态
	if _, err := app.getRecruitmentDB(); err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	app.closeRecruitmentDB()

	// 2. 在 recruitmentDB == nil 时创建新项目与简历（仅落盘到 JSON，SQLite 中尚无 projects 和 project_candidates 记录）
	project := app.CreateProject("大客户销售经理", "销售部", 1, "HR", "", &JobConfig{
		Title:          "大客户销售经理",
		RequiredSkills: []string{"体外诊断", "直销"},
	})
	res1 := &Resume{
		ID:                  "51job_1792725305405_0_0",
		ProjectID:           project.ID,
		FileName:            "【前程无忧】高先生_大客户销售经理.txt",
		Status:              "pending",
		SourcePlatform:      "51job",
		PlatformCandidateID: "905922586",
		URL:                 "https://ehire.51job.com/Revision/talent/resume/detail?contentType=1&resumeId=905922586",
		Content:             "姓名 / 称谓：高先生\n工作经历：负责市场开发与大客户维护",
	}
	app.saveResume(res1)
	project.ResumeIDs = []string{res1.ID}
	app.saveProject(project)

	// 3. 调用 PreviewBatchOutreach，验证自动自愈补齐 projects 与 project_candidates 外键，绝不触发 FOREIGN KEY constraint failed (787)
	batch, err := app.PreviewBatchOutreach(project.ID, nil, "template")
	if err != nil {
		t.Fatalf("PreviewBatchOutreach 不应触发外键约束错误: %v", err)
	}
	if len(batch.Items) != 1 {
		t.Fatalf("应包含 1 位候选人，实际: %d", len(batch.Items))
	}
	if batch.Items[0].ProjectCandidateID == res1.ID {
		t.Fatalf("ProjectCandidateID 应为有效的 project_candidates.id，而非裸 resume.ID: %s", batch.Items[0].ProjectCandidateID)
	}
}
