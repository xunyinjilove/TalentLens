package main

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestResolveCandidateRadarScore(t *testing.T) {
	analysis := &AnalysisResult{
		CoreCriterionChecks: []CoreCriterionCheck{
			{Criterion: "Vue3", Level: 3, Evidence: "熟练使用Vue3"},
			{Criterion: "TypeScript", Level: 5, Evidence: "精通TS"}, // 非法等级不得进入正式证据
			{Criterion: "前端工程化与性能优化", Level: 2, Evidence: "负责Webpack构建"},
		},
		ExperienceCompetencies: []CoreCriterionCheck{
			{Criterion: "团队协作与沟通", Level: -1, Evidence: ""}, // 低于下限应被截断为 0
		},
	}

	// 1. 精确匹配
	lvl, ok := resolveCandidateRadarScore(analysis, "Vue3")
	if !ok || lvl != 3 {
		t.Fatalf("Vue3 精确匹配预期 3，实际: %d (ok=%v)", lvl, ok)
	}

	// 2. 大小写与空格不敏感
	lvl, ok = resolveCandidateRadarScore(analysis, "  vue3  ")
	if !ok || lvl != 3 {
		t.Fatalf("vue3 大小写模糊匹配预期 3，实际: %d", lvl)
	}

	// 3. 非法等级不得算作已知
	lvl, ok = resolveCandidateRadarScore(analysis, "TypeScript")
	if ok || lvl != 0 {
		t.Fatalf("TypeScript 非法等级应待核实，实际: %d", lvl)
	}

	// 4. 不能把相近指标的旧证据借给新指标
	lvl, ok = resolveCandidateRadarScore(analysis, "性能优化")
	if ok || lvl != 0 {
		t.Fatalf("性能优化不能模糊借用，实际: %d", lvl)
	}

	// 5. 没有证据的低等级是未知
	lvl, ok = resolveCandidateRadarScore(analysis, "团队协作与沟通")
	if ok || lvl != 0 {
		t.Fatalf("团队协作与沟通应待核实，实际: %d", lvl)
	}

	// 6. 未提及技能指标为未知
	lvl, ok = resolveCandidateRadarScore(analysis, "Go后端开发")
	if ok || lvl != 0 {
		t.Fatalf("未提及技能应待核实，实际: %d", lvl)
	}

	// 7. 空分析结果返回 false
	lvl, ok = resolveCandidateRadarScore(nil, "Vue3")
	if ok || lvl != 0 {
		t.Fatalf("nil 分析预期 false, 0，实际: %v, %d", ok, lvl)
	}
}

func TestExportProjectReportWithRadarMetrics(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()

	jobCfg := &JobConfig{
		Title:          "前端开发专家",
		RequiredSkills: []string{"Vue3", "TypeScript"},
		CoreCriteria:   []string{"前端性能优化"},
	}
	p := a.CreateProject("前端招聘项目", "", 1, "", "", jobCfg)
	if p == nil {
		t.Fatal("创建项目失败")
	}

	r1 := &Resume{
		ID:        "resume_1",
		ProjectID: p.ID,
		FileName:  "zhangsan.pdf",
		Score:     92,
		Status:    "done",
		Content:   "张三的简历内容。熟练使用Vue3与Pinia进行企业级中后台开发。主导底层SDK的TypeScript类型架构与重构。参与首屏Lighthouse优化。",
		Analysis: &AnalysisResult{
			CandidateName:   "张三",
			OverallScore:    92,
			SkillMatch:      95,
			ExperienceMatch: 90,
			EducationMatch:  88,
			Recommendation:  "strong_recommend",
			Strengths:       []string{"Vue3生态深入掌握", "代码规范严谨"},
			Weaknesses:      []string{"无移动端开发经验"},
			Risks:           []string{"近期跳槽稍显频繁"},
			Summary:         "非常符合要求的前端高级开发工程师",
			CoreCriterionChecks: []CoreCriterionCheck{
				{Criterion: "Vue3", Level: 3, Evidence: "熟练使用Vue3与Pinia进行企业级中后台开发"},
				{Criterion: "TypeScript", Level: 4, Evidence: "主导底层SDK的TypeScript类型架构与重构"},
				{Criterion: "前端性能优化", Level: 2, Evidence: "参与首屏Lighthouse优化"},
			},
		},
	}

	r2 := &Resume{
		ID:        "resume_2",
		ProjectID: p.ID,
		FileName:  "lisi.pdf",
		Score:     75,
		Status:    "done",
		Content:   "李四的简历内容。在校学习过Vue3基础。",
		Analysis: &AnalysisResult{
			CandidateName:   "李四",
			OverallScore:    75,
			SkillMatch:      70,
			ExperienceMatch: 75,
			EducationMatch:  80,
			Recommendation:  "consider",
			CoreCriterionChecks: []CoreCriterionCheck{
				{Criterion: "Vue3", Level: 1, Evidence: "在校学习过Vue3基础"},
				// TypeScript 未提及
				{Criterion: "前端性能优化", Level: 0, Evidence: ""},
			},
		},
	}

	r3 := &Resume{
		ID:        "resume_3",
		ProjectID: p.ID,
		FileName:  "wangwu.pdf",
		Score:     0,
		Status:    "pending",
		Analysis:  nil, // 待分析简历
	}

	a.saveResume(r1)
	a.saveResume(r2)
	a.saveResume(r3)
	p.ResumeIDs = []string{r1.ID, r2.ID, r3.ID}
	a.saveProject(p)

	// 执行导出
	outPath, err := a.ExportProjectReport(p.ID)
	if err != nil {
		t.Fatalf("ExportProjectReport 失败: %v", err)
	}
	if !strings.HasSuffix(outPath, ".xlsx") {
		t.Fatalf("导出文件扩展名不是 .xlsx: %s", outPath)
	}

	// 用 excelize 打开并验证内容
	f, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("打开生成的 Excel 失败: %v", err)
	}
	defer f.Close()

	sheet := "候选人排名"
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("读取工作表失败: %v", err)
	}
	if len(rows) < 4 {
		t.Fatalf("导出的表格行数不足: 实际 %d 行", len(rows))
	}

	headerRow := rows[0]
	// 验证表头是否包含三个雷达指标
	expectedRadarHeaders := []string{
		"【技能雷达】Vue3 (0-4分)",
		"【技能雷达】TypeScript (0-4分)",
		"【技能雷达】前端性能优化 (0-4分)",
	}

	headerIndices := make(map[string]int)
	for idx, h := range headerRow {
		headerIndices[h] = idx
	}

	for _, exp := range expectedRadarHeaders {
		if _, exists := headerIndices[exp]; !exists {
			t.Fatalf("表头缺少预期的雷达指标: %q, 当前表头: %v", exp, headerRow)
		}
	}

	vueIdx := headerIndices["【技能雷达】Vue3 (0-4分)"]
	tsIdx := headerIndices["【技能雷达】TypeScript (0-4分)"]
	perfIdx := headerIndices["【技能雷达】前端性能优化 (0-4分)"]

	// 验证张三得分（第 2 行，rows[1]）
	rowZhang := rows[1]
	nameIdx := headerIndices["姓名"]
	if rowZhang[nameIdx] != "张三" {
		t.Fatalf("第 1 位候选人预期为张三，实际: %s", rowZhang[nameIdx])
	}
	if rowZhang[vueIdx] != "3" {
		t.Fatalf("张三 Vue3 得分预期 3，实际: %s", rowZhang[vueIdx])
	}
	if rowZhang[tsIdx] != "4" {
		t.Fatalf("张三 TypeScript 得分预期 4，实际: %s", rowZhang[tsIdx])
	}
	if rowZhang[perfIdx] != "2" {
		t.Fatalf("张三 前端性能优化 得分预期 2，实际: %s", rowZhang[perfIdx])
	}

	// 验证李四得分（第 3 行，rows[2]）
	rowLi := rows[2]
	if rowLi[nameIdx] != "李四" {
		t.Fatalf("第 2 位候选人预期为李四，实际: %s", rowLi[nameIdx])
	}
	if rowLi[vueIdx] != "1" {
		t.Fatalf("李四 Vue3 得分预期 1，实际: %s", rowLi[vueIdx])
	}
	// 未提及是未知，不是能力 0 分。
	if rowLi[tsIdx] != "待核实" {
		t.Fatalf("李四 TypeScript 应待核实，实际: %s", rowLi[tsIdx])
	}
	if rowLi[perfIdx] != "待核实" {
		t.Fatalf("李四 前端性能优化 应待核实，实际: %s", rowLi[perfIdx])
	}

	// 验证王五（未分析简历，第 4 行，rows[3]）
	rowWang := rows[3]
	if rowWang[nameIdx] != "wangwu.pdf" {
		t.Fatalf("第 3 位候选人预期为 wangwu.pdf，实际: %s", rowWang[nameIdx])
	}
	// 未分析简历的雷达列应当为空
	if vueIdx < len(rowWang) && rowWang[vueIdx] != "" {
		t.Fatalf("王五未分析简历 Vue3 得分预期为空，实际: %q", rowWang[vueIdx])
	}

	// 验证列宽
	colName, _ := excelize.ColumnNumberToName(vueIdx + 1)
	width, err := f.GetColWidth(sheet, colName)
	if err != nil || width < 16 {
		t.Fatalf("雷达列宽未正确设置: width=%v, err=%v", width, err)
	}
}
