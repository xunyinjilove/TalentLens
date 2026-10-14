package main

import (
	"testing"
	"time"
)

func TestJobStatisticsAndMoveCandidate(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	projA := &Project{
		ID:         "proj_a",
		Name:       "高级前端工程师",
		Department: "研发中心",
		Headcount:  2,
		JobConfig: JobConfig{
			Title:          "高级前端工程师",
			City:           "上海",
			EmploymentType: "全职",
			SalaryRange:    "25-40K",
		},
		Status:    "publishing",
		ResumeIDs: []string{"res_1", "res_2"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	projB := &Project{
		ID:         "proj_b",
		Name:       "运维工程师",
		Department: "基础架构",
		Headcount:  1,
		JobConfig: JobConfig{
			Title:          "运维工程师",
			City:           "深圳",
			EmploymentType: "全职",
			SalaryRange:    "20-35K",
		},
		Status:    "publishing",
		ResumeIDs: []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	res1 := &Resume{
		ID:        "res_1",
		ProjectID: projA.ID,
		FileName:  "张三.pdf",
		Content:   "前端开发5年精通Vue3",
		Status:    "done",
		Analysis: &AnalysisResult{
			CandidateName: "张三",
			AbilityScore:  85,
			RedLineStatus: "passed",
		},
		CreatedAt: now,
	}

	res2 := &Resume{
		ID:        "res_2",
		ProjectID: projA.ID,
		FileName:  "李四.pdf",
		Content:   "初级前端，有违规红线",
		Status:    "done",
		Analysis: &AnalysisResult{
			CandidateName: "李四",
			AbilityScore:  45,
			RedLineStatus: "failed",
		},
		CreatedAt: now,
	}

	writeLegacyJSON(t, dir, "projects", projA.ID, projA)
	writeLegacyJSON(t, dir, "projects", projB.ID, projB)
	writeLegacyJSON(t, dir, "resumes", res1.ID, res1)
	writeLegacyJSON(t, dir, "resumes", res2.ID, res2)

	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()

	// 1. 测试统计接口 GetJobStatistics
	stats, err := app.GetJobStatistics()
	if err != nil {
		t.Fatalf("GetJobStatistics failed: %v", err)
	}

	statA, okA := stats[projA.ID]
	if !okA || statA.TotalApplications != 2 || statA.SuitableCount != 1 {
		t.Fatalf("projA stats mismatch: %+v (expected Total:2, Suitable:1)", statA)
	}

	statB, okB := stats[projB.ID]
	if !okB || statB.TotalApplications != 0 || statB.SuitableCount != 0 {
		t.Fatalf("projB stats mismatch: %+v (expected Total:0, Suitable:0)", statB)
	}

	// 2. 查询 projA 的候选人列表
	candidatesA, err := app.ListTalentCandidates(projA.ID)
	if err != nil {
		t.Fatalf("ListTalentCandidates failed: %v", err)
	}
	if len(candidatesA) != 2 {
		t.Fatalf("expected 2 candidates in projA, got %d", len(candidatesA))
	}

	// 3. 测试候选人跨库调配 MoveCandidateToProject
	targetCandidate := candidatesA[0]
	err = app.MoveCandidateToProject(targetCandidate.ProjectCandidateID, projB.ID)
	if err != nil {
		t.Fatalf("MoveCandidateToProject failed: %v", err)
	}

	// 4. 再次验证分库人选
	candidatesAfterA, _ := app.ListTalentCandidates(projA.ID)
	candidatesAfterB, _ := app.ListTalentCandidates(projB.ID)

	if len(candidatesAfterA) != 1 {
		t.Fatalf("expected 1 candidate left in projA, got %d", len(candidatesAfterA))
	}
	if len(candidatesAfterB) != 1 {
		t.Fatalf("expected 1 candidate moved to projB, got %d", len(candidatesAfterB))
	}
	if candidatesAfterB[0].Name != targetCandidate.Name {
		t.Fatalf("expected moved candidate name %s, got %s", targetCandidate.Name, candidatesAfterB[0].Name)
	}
}

func TestEndToEndRecruitmentFlow(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()

	// 1. 创建「体外诊断IVD研发总监」职位
	jobCfg := &JobConfig{
		Title:           "体外诊断IVD研发总监",
		City:            "上海",
		EmploymentType:  "全职",
		SalaryRange:     "35-50K",
		ExperienceYears: 8,
		EducationLevel:  "硕士",
		RequiredSkills:  []string{"IVD", "化学发光", "试剂研发", "团队管理"},
		RedLines:        []string{"有独立IVD试剂开发申报经验"},
		BonusPoints:     []string{"主导过三类注册证获批"},
		CoreCriteria:    []string{"IVD", "化学发光"},
	}
	proj := app.CreateProject("体外诊断IVD研发总监", "研发中心", 2, "林", "IVD骨干", jobCfg)
	if proj == nil || proj.ID == "" {
		t.Fatalf("CreateProject failed: %+v", proj)
	}

	// 2. 检查 SQLite 中的项目数据
	db, err := app.getRecruitmentDB()
	if err != nil {
		t.Fatalf("getRecruitmentDB failed: %v", err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM projects WHERE id = ?", proj.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("project not synced to SQLite: count=%d, err=%v", count, err)
	}

	// 3. 更新职位信息（调整薪资与状态）
	proj.JobConfig.SalaryRange = "40-60K"
	proj.Status = "publishing"
	if err := app.UpdateProject(proj); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	// 4. 为该岗位导入一位候选人简历
	res := &Resume{
		ID:        "resume_ivd_001",
		ProjectID: proj.ID,
		FileName:  "陈博士_化学发光IVD专家_8年经验.pdf",
		Content:   "具有三类化学发光检测试剂盒独立研发与申报经验",
		Status:    "done",
		Analysis: &AnalysisResult{
			CandidateName: "陈博士",
			AbilityScore:  92,
			RedLineStatus: "passed",
		},
		CreatedAt: now,
	}
	app.saveResume(res)
	proj.ResumeIDs = append(proj.ResumeIDs, res.ID)
	app.saveProject(proj)

	// 5. 查询分库人选
	candidates, err := app.ListTalentCandidates(proj.ID)
	if err != nil {
		t.Fatalf("ListTalentCandidates failed: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Name != "陈博士" {
		t.Fatalf("expected 1 candidate '陈博士', got %+v", candidates)
	}

	// 6. 候选人跟进阶段流转至「初面」
	cand := candidates[0]
	err = app.UpdateCandidateFollowUp(cand.ProjectCandidateID, "first_interview", "林HR", "2026-10-20T14:00:00Z", "AI初筛通过，预约技术总监初面", "HR")
	if err != nil {
		t.Fatalf("UpdateCandidateFollowUp failed: %v", err)
	}

	// 7. 安排面试记录
	interview := RecruitmentInterview{
		ID:                 "interview_001",
		ProjectCandidateID: cand.ProjectCandidateID,
		CandidateName:      cand.Name,
		ProjectName:        proj.Name,
		StartsAt:           "2026-10-20T14:00:00Z",
		EndsAt:             "2026-10-20T15:00:00Z",
		Interviewer:        "张总监",
		Method:             "现场面试",
		Location:           "张江研发大楼301会议室",
		Status:             "scheduled",
	}
	if _, err := app.SaveRecruitmentInterview(interview); err != nil {
		t.Fatalf("SaveRecruitmentInterview failed: %v", err)
	}
	interviews, err := app.ListRecruitmentInterviews(proj.ID)
	if err != nil || len(interviews) != 1 {
		t.Fatalf("ListRecruitmentInterviews failed: %v, count=%d", err, len(interviews))
	}

	// 8. 验证统计指标
	stats, err := app.GetJobStatistics()
	if err != nil {
		t.Fatalf("GetJobStatistics failed: %v", err)
	}
	st, ok := stats[proj.ID]
	if !ok || st.TotalApplications != 1 || st.SuitableCount != 1 {
		t.Fatalf("job stats mismatch: %+v (expected Total:1, Suitable:1)", st)
	}

	// 9. 测试删除职位与级联清理
	if err := app.DeleteProject(proj.ID); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
	// 验证 SQLite 中无残留
	if err := db.QueryRow("SELECT COUNT(*) FROM projects WHERE id = ?", proj.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("deleted project still exists in SQLite: count=%d, err=%v", count, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM project_candidates WHERE project_id = ?", proj.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("deleted project candidate relations still exist in SQLite: count=%d, err=%v", count, err)
	}
}

