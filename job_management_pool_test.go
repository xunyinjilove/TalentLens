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
