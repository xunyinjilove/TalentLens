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
	if pendingItem.RedLineStatus != "pending" {
		t.Fatalf("待分析人选红线状态应为 pending，实际: %s", pendingItem.RedLineStatus)
	}
}

func TestInterviewQuestionAssemblyFromJobCriteria(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()

	project := app.CreateProject("质量总监", "质量部", 1, "HR", "", &JobConfig{
		Title:          "质量总监",
		CoreCriteria:   []string{"GMP认证合规", "CAPA持续改进"},
		RedLines:       []string{"10年以上质量管理经验"},
		BonusPoints:    []string{"FDA海外审计经验"},
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
			RedLineChecks: []RedLineCheck{
				{Criterion: "10年以上质量管理经验", Status: "unknown"},
			},
			BonusChecks: []BonusCheck{
				{Criterion: "FDA海外审计经验", Status: "unknown"},
			},
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

	// 验证问题中包含岗位统一题、核心能力题、红线题与加分项题
	hasRedLine := false
	hasBonus := false
	hasStandard := false
	for _, turn := range session.Turns {
		if turn.Category == "红线核实" {
			hasRedLine = true
		}
		if turn.Category == "加分核实" {
			hasBonus = true
		}
		if turn.Standard {
			hasStandard = true
		}
	}
	if !hasRedLine {
		t.Errorf("初面题目未成功融入红线核实题")
	}
	if !hasBonus {
		t.Errorf("初面题目未成功融入加分核实题")
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
