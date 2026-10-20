package main

import (
	"testing"
	"time"
)

func TestExtractCandidateDeduplicationKey(t *testing.T) {
	// 1. 同名同公司候选人（不同微简历版本）
	r1 := &Resume{
		ID:       "res_1",
		FileName: "【前程无忧】朱女士_软件测试工程师.txt",
		Content:  "基本画像：3日内活跃 · 36岁 · 13年 · 本科 · 上海\n任职履历快照：2017.05-至今 | 上海驰骛信息科技有限公司 | 高级软件测试",
	}
	r2 := &Resume{
		ID:                  "res_2",
		FileName:            "【前程无忧】朱女士 (1)_软件测试工程师.txt",
		PlatformCandidateID: "403920688",
		Content:             "基本画像：1周内活跃 · 36岁 · 13年 · 本科 · 上海\n任职履历快照：工作经历 | 上海驰骛信息科技有限公司 | 高级软件测试工程师",
	}

	key1 := extractCandidateDeduplicationKey(r1)
	key2 := extractCandidateDeduplicationKey(r2)

	if key1 != key2 {
		t.Fatalf("同一候选人排重Key应一致: key1=%s, key2=%s", key1, key2)
	}

	// 2. 同名不同公司候选人
	r3 := &Resume{
		ID:                  "res_3",
		FileName:            "【前程无忧】朱女士_软件测试工程师.txt",
		PlatformCandidateID: "901044607",
		Content:             "基本画像：2周内活跃 · 38岁 · 17年 · 本科 · 上海\n任职履历快照：平安健康互联网股份有限公司上海分公司 | 软件测试",
	}
	key3 := extractCandidateDeduplicationKey(r3)
	if key1 == key3 {
		t.Fatalf("不同公司的同名候选人Key应不同: key1=%s, key3=%s", key1, key3)
	}
}

func TestPruneDuplicateProjectResumes(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()

	now := time.Now()
	p := &Project{
		ID:        "proj_dedup",
		Name:      "软件测试工程师",
		ResumeIDs: []string{"r1", "r2", "r3"},
	}

	// r1: 未分析旧版微简历
	r1 := &Resume{
		ID:        "r1",
		ProjectID: p.ID,
		FileName:  "【前程无忧】赵女士.txt",
		Status:    "pending",
		Content:   "基本画像：41岁 · 18年 · 本科\n任职履历快照：上海展湾信息科技有限公司 | 测试",
		CreatedAt: now.Add(-10 * time.Minute),
	}

	// r2: 已完成分析且带有 PlatformCandidateID 的新版
	r2 := &Resume{
		ID:                  "r2",
		ProjectID:           p.ID,
		FileName:            "【前程无忧】赵女士_软件测试.txt",
		PlatformCandidateID: "344882917",
		Status:              "done",
		Score:               85,
		Analysis: &AnalysisResult{
			CandidateName: "赵女士",
			OverallScore:  85,
			AnalyzedAt:    now.Format(time.RFC3339),
		},
		Content:   "基本画像：41岁 · 18年 · 本科\n任职履历快照：上海展湾信息科技有限公司 | 高级测试工程师",
		CreatedAt: now,
	}

	// r3: 另一位不同候选人
	r3 := &Resume{
		ID:        "r3",
		ProjectID: p.ID,
		FileName:  "【前程无忧】李先生.txt",
		Status:    "done",
		Score:     78,
		Content:   "基本画像：30岁 · 7年 · 硕士\n任职履历快照：上海龙旗科技股份有限公司 | 研发",
		CreatedAt: now,
	}

	writeLegacyJSON(t, app.getDataDir(), "projects", p.ID, p)
	writeLegacyJSON(t, app.getDataDir(), "resumes", r1.ID, r1)
	writeLegacyJSON(t, app.getDataDir(), "resumes", r2.ID, r2)
	writeLegacyJSON(t, app.getDataDir(), "resumes", r3.ID, r3)

	pruned := app.pruneDuplicateProjectResumes(p, []*Resume{r1, r2, r3})

	if len(pruned) != 2 {
		t.Fatalf("去重后应只有 2 位独立候选人，实际为: %d", len(pruned))
	}

	// 赵女士应保留已分析的 r2 作为 primary
	var zhao *Resume
	for _, r := range pruned {
		if cleanCandidateNameKey(r) == "赵女士" {
			zhao = r
		}
	}
	if zhao == nil || zhao.ID != "r2" || zhao.Score != 85 {
		t.Fatalf("应保留已分析的 r2，实际保留: %+v", zhao)
	}

	// 且 r1 的 ID 应该进入 r2 的 SnapshotCopyIDs
	foundCopy := false
	for _, cid := range zhao.SnapshotCopyIDs {
		if cid == "r1" {
			foundCopy = true
			break
		}
	}
	if !foundCopy {
		t.Fatalf("r1 应该被记录在 r2 的 SnapshotCopyIDs 中: %v", zhao.SnapshotCopyIDs)
	}

	// 项目持久化的 ResumeIDs 应该同步减少为 2
	storedProj := app.GetProject(p.ID)
	if len(storedProj.ResumeIDs) != 2 {
		t.Fatalf("项目落盘的 ResumeIDs 数量应更新为 2，实际为: %d", len(storedProj.ResumeIDs))
	}
}

func TestContractRadarScoringFixtures(t *testing.T) {
	// 夹具 1: 6 项等权，[3, 3, 2, 4, 3, null]，未知1项 -> 覆盖率 83.33%，得分 75.00，合格，区间 62.50 ~ 79.17
	criteria1 := []string{"M1", "M2", "M3", "M4", "M5", "M6"}
	res1 := &AnalysisResult{
		CoreCriterionChecks: []CoreCriterionCheck{
			{Criterion: "M1", Level: 3, Evidence: "ev1", EvidenceStatus: "known"},
			{Criterion: "M2", Level: 3, Evidence: "ev2", EvidenceStatus: "known"},
			{Criterion: "M3", Level: 2, Evidence: "ev3", EvidenceStatus: "known"},
			{Criterion: "M4", Level: 4, Evidence: "ev4", EvidenceStatus: "known"},
			{Criterion: "M5", Level: 3, Evidence: "ev5", EvidenceStatus: "known"},
			{Criterion: "M6", Level: 0, EvidenceStatus: "unknown"},
		},
	}
	scoreCandidate(res1, criteria1)
	if res1.RadarCoverage < 0.83 || res1.RadarCoverage > 0.84 {
		t.Fatalf("夹具1覆盖率错误: %v", res1.RadarCoverage)
	}
	if !res1.RankEligible {
		t.Fatalf("夹具1应具备排名资格")
	}
	if res1.AbilityScore != 75 {
		t.Fatalf("夹具1得分应为 75，实际: %d", res1.AbilityScore)
	}
	if res1.ScoreLowerBound < 62.4 || res1.ScoreLowerBound > 62.6 {
		t.Fatalf("夹具1下界应为 62.50，实际: %f", res1.ScoreLowerBound)
	}

	// 夹具 2: 5 项等权，[3, 2, 1, null, null] -> 覆盖率 60%，得分 50.00，不合格 (<80%)，区间 30.00 ~ 70.00
	criteria2 := []string{"M1", "M2", "M3", "M4", "M5"}
	res2 := &AnalysisResult{
		CoreCriterionChecks: []CoreCriterionCheck{
			{Criterion: "M1", Level: 3, Evidence: "ev1", EvidenceStatus: "known"},
			{Criterion: "M2", Level: 2, Evidence: "ev2", EvidenceStatus: "known"},
			{Criterion: "M3", Level: 1, Evidence: "ev3", EvidenceStatus: "known"},
			{Criterion: "M4", Level: 0, EvidenceStatus: "unknown"},
			{Criterion: "M5", Level: 0, EvidenceStatus: "unknown"},
		},
	}
	scoreCandidate(res2, criteria2)
	if res2.RadarCoverage != 0.6 {
		t.Fatalf("夹具2覆盖率应为 60%%，实际: %v", res2.RadarCoverage)
	}
	if res2.RankEligible {
		t.Fatalf("夹具2覆盖率不足80%%，不应具备正式排名资格")
	}
	if res2.AbilityScore != 50 {
		t.Fatalf("夹具2得分应为 50，实际: %d", res2.AbilityScore)
	}

	// 夹具 3: 5 项等权，[0, 3, 3, 2, 2]，0 有明确依据 -> 覆盖率 100%，得分 50.00，合格，区间 50.00 ~ 50.00
	criteria3 := []string{"M1", "M2", "M3", "M4", "M5"}
	res3 := &AnalysisResult{
		CoreCriterionChecks: []CoreCriterionCheck{
			{Criterion: "M1", Level: 0, Evidence: "明确不具备该技能", EvidenceStatus: "known"},
			{Criterion: "M2", Level: 3, Evidence: "ev2", EvidenceStatus: "known"},
			{Criterion: "M3", Level: 3, Evidence: "ev3", EvidenceStatus: "known"},
			{Criterion: "M4", Level: 2, Evidence: "ev4", EvidenceStatus: "known"},
			{Criterion: "M5", Level: 2, Evidence: "ev5", EvidenceStatus: "known"},
		},
	}
	scoreCandidate(res3, criteria3)
	if res3.RadarCoverage != 1.0 {
		t.Fatalf("夹具3覆盖率应为 100%%，实际: %v", res3.RadarCoverage)
	}
	if !res3.RankEligible {
		t.Fatalf("夹具3应具备排名资格")
	}
	if res3.AbilityScore != 50 {
		t.Fatalf("夹具3得分应为 50，实际: %d", res3.AbilityScore)
	}
	if res3.ScoreLowerBound != 50.0 || res3.ScoreUpperBound != 50.0 {
		t.Fatalf("夹具3区间应为 [50, 50]，实际: [%f, %f]", res3.ScoreLowerBound, res3.ScoreUpperBound)
	}
}
