package main

import (
	"strings"
	"testing"
	"time"
)

func TestCommunicationJobsCacheStorageAndList(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()

	jobs := []JobCommunicationOption{
		{ID: "job_1", Title: "临床试验项目经理", Company: "泰格医药", CompanyID: "co_1", WorkArea: "上海-浦东新区", Source: "我的职位", PublishStatus: "在招"},
		{ID: "job_2", Title: "数据统计师", Company: "泰格医药", CompanyID: "co_1", WorkArea: "上海-徐汇区", Source: "公司职位", PublishStatus: "在招"},
	}

	if err := a.SaveCommunicationJobs("51job", "", jobs); err != nil {
		t.Fatalf("SaveCommunicationJobs failed: %v", err)
	}

	cached, err := a.ListCachedCommunicationJobs("51job", "")
	if err != nil {
		t.Fatalf("ListCachedCommunicationJobs failed: %v", err)
	}
	if len(cached) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(cached))
	}

	foundJob1 := false
	for _, j := range cached {
		if j.ID == "job_1" {
			foundJob1 = true
			if j.Title != "临床试验项目经理" || j.CompanyID != "co_1" || j.Company != "泰格医药" || j.WorkArea != "上海-浦东新区" {
				t.Fatalf("job_1 fields mismatch: %+v", j)
			}
		}
	}
	if !foundJob1 {
		t.Fatal("job_1 not found in cached list")
	}

	// Upsert test: updating an existing job
	jobs[0].Title = "临床试验高级项目经理"
	if err := a.SaveCommunicationJobs("51job", "", jobs[:1]); err != nil {
		t.Fatalf("SaveCommunicationJobs upsert failed: %v", err)
	}
	cachedAfter, err := a.ListCachedCommunicationJobs("51job", "")
	if err != nil || len(cachedAfter) != 2 {
		t.Fatalf("expected 2 jobs after upsert, got %d: %v", len(cachedAfter), err)
	}
	for _, j := range cachedAfter {
		if j.ID == "job_1" && j.Title != "临床试验高级项目经理" {
			t.Fatalf("expected updated title, got %s", j.Title)
		}
	}
}

func TestProjectDefaultCommunicationJob(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()

	p := a.CreateProject("临床研发", "研发部", 1, "HR", "", &JobConfig{Title: "临床试验项目经理"})
	if err := a.SetProjectDefaultCommunicationJob(p.ID, "51job", "", "job_100", "临床试验项目经理", "co_1"); err != nil {
		t.Fatalf("SetProjectDefaultCommunicationJob failed: %v", err)
	}

	rec, err := a.GetProjectDefaultCommunicationJob(p.ID, "51job", "")
	if err != nil || rec == nil {
		t.Fatalf("GetProjectDefaultCommunicationJob failed: %v", err)
	}
	if rec.JobID != "job_100" || rec.JobTitle != "临床试验项目经理" || rec.CompanyID != "co_1" {
		t.Fatalf("unexpected record: %+v", rec)
	}
}

func TestGetCommunicationJobsImmediateAndDefaultPriority(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()

	p := a.CreateProject("临床研发", "研发部", 1, "HR", "", &JobConfig{Title: "临床数据统计师"})

	// 1. 无缓存时
	qEmpty := a.GetCommunicationJobs(p.ID, "51job")
	if qEmpty.Status != "empty" || len(qEmpty.Jobs) != 0 || !qEmpty.NeedsRefresh {
		t.Fatalf("expected empty and needsRefresh, got: %+v", qEmpty)
	}

	// 2. 存入缓存
	jobs := []JobCommunicationOption{
		{ID: "j1", Title: "临床试验项目经理", Company: "Tellgen", CompanyID: "c1"},
		{ID: "j2", Title: "临床数据统计师", Company: "Tellgen", CompanyID: "c1"},
	}
	if err := a.SaveCommunicationJobs("51job", "", jobs); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_ = a.SaveCommunicationJobSyncSnapshot(&CommunicationJobSyncSnapshot{
		ID:           "sync_test",
		Platform:     "51job",
		Status:       "complete",
		TotalJobs:    2,
		StartedAt:    now,
		FinishedAt:   now,
	})

	// 此时无项目默认职位，应按项目招聘岗位标题唯一精准匹配到 j2
	qMatched := a.GetCommunicationJobs(p.ID, "51job")
	if qMatched.Status != "ok" || len(qMatched.Jobs) != 2 {
		t.Fatalf("expected ok with 2 jobs, got: %+v", qMatched)
	}
	if qMatched.DefaultJobID != "j2" {
		t.Fatalf("expected defaultJobID j2 by title match, got %s", qMatched.DefaultJobID)
	}
	if qMatched.NeedsRefresh {
		t.Fatalf("fresh snapshot should not need refresh: %+v", qMatched)
	}

	// 3. HR 明确确认了该项目的默认职位为 j1（即使标题与招聘岗位不同，以 HR 确认为最高优先）
	_ = a.SetProjectDefaultCommunicationJob(p.ID, "51job", "", "j1", "临床试验项目经理", "c1")
	qHRDefault := a.GetCommunicationJobs(p.ID, "51job")
	if qHRDefault.DefaultJobID != "j1" {
		t.Fatalf("expected HR confirmed default j1, got %s", qHRDefault.DefaultJobID)
	}
}

func TestListOutreachCommunicationJobsUsesCacheWithoutOnlineBrowser(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()

	p := a.CreateProject("项目A", "部门A", 1, "HR", "", &JobConfig{Title: "临床试验项目经理"})
	r := &Resume{
		ID:       "r_test_1",
		ProjectID: p.ID,
		FileName: "张三_临床试验.pdf",
		Content:  "多年临床试验经验",
		Status:   "done",
		Analysis: &AnalysisResult{CandidateName: "张三", OverallScore: 85},
	}
	if err := a.persistSearchCandidate(r); err != nil {
		t.Fatal(err)
	}
	batch, err := a.PreviewTopTenOutreach(p.ID)
	if err != nil || len(batch.Items) == 0 {
		t.Fatalf("PreviewTopTenOutreach failed: %v", err)
	}

	// 预先存入缓存
	cachedJobs := []JobCommunicationOption{
		{ID: "51_job_888", Title: "临床试验项目经理", Company: "泰格医药", CompanyID: "co_99", WorkArea: "上海"},
	}
	if err := a.SaveCommunicationJobs("51job", "", cachedJobs); err != nil {
		t.Fatal(err)
	}

	// 即使候选人详情页不具备真实平台登录或不可连接，也必须秒级返回缓存列表
	result := a.ListOutreachCommunicationJobs(batch.ID, "")
	if result.Status != "jobs" {
		t.Fatalf("expected jobs status from cache, got: %+v", result)
	}
	if len(result.Jobs) != 1 || result.Jobs[0].ID != "51_job_888" {
		t.Fatalf("expected cached job 51_job_888, got: %+v", result.Jobs)
	}
	if result.SuggestedJobID != "51_job_888" {
		t.Fatalf("expected suggested job 51_job_888, got: %s", result.SuggestedJobID)
	}
}

func TestAuthorizeOutreachBudgetValidatesFromCacheAndRemembersChoice(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()

	p := a.CreateProject("项目B", "部门B", 1, "HR", "", &JobConfig{Title: "临床试验项目经理"})
	r := &Resume{
		ID:       "r_test_2",
		ProjectID: p.ID,
		FileName: "李四_临床试验.pdf",
		Content:  "多年临床试验经验",
		Status:   "done",
		Analysis: &AnalysisResult{CandidateName: "李四", OverallScore: 90},
	}
	if err := a.persistSearchCandidate(r); err != nil {
		t.Fatal(err)
	}
	batch, err := a.PreviewTopTenOutreach(p.ID)
	if err != nil || len(batch.Items) == 0 {
		t.Fatalf("PreviewTopTenOutreach failed: %v", err)
	}

	// 确认名单
	batch, err = a.ConfirmOutreachBatch(batch.ID, []OutreachApproval{
		{ItemID: batch.Items[0].ID, Message: "你好！", Include: true, ConfirmPending: true},
	})
	if err != nil {
		t.Fatalf("ConfirmOutreachBatch failed: %v", err)
	}

	// 存入本地缓存
	cachedJobs := []JobCommunicationOption{
		{ID: "job_authorized_1", Title: "临床试验项目经理", Company: "泰格医药", CompanyID: "co_88", WorkArea: "上海"},
	}
	if err := a.SaveCommunicationJobs("51job", "", cachedJobs); err != nil {
		t.Fatal(err)
	}

	// 执行授权：直接读取本地缓存校验，不触发在线重拉
	authBatch, err := a.AuthorizeOutreachBudget(batch.ID, "job_authorized_1", "临床试验项目经理", 100, 100000)
	if err != nil {
		t.Fatalf("AuthorizeOutreachBudget failed: %v", err)
	}
	if authBatch.Authorization == nil || authBatch.Authorization.JobID != "job_authorized_1" {
		t.Fatalf("unexpected authorization: %+v", authBatch.Authorization)
	}

	// 验证项目默认职位是否已自动记住
	defaultRec, err := a.GetProjectDefaultCommunicationJob(p.ID, "51job", "")
	if err != nil || defaultRec == nil {
		t.Fatalf("GetProjectDefaultCommunicationJob failed: %v", err)
	}
	if defaultRec.JobID != "job_authorized_1" || defaultRec.CompanyID != "co_88" {
		t.Fatalf("project default job mismatch: %+v", defaultRec)
	}

	// 尝试授权不存在的职位应当被拒绝
	_, err = a.AuthorizeOutreachBudget(batch.ID, "non_existent_job", "不存在的职位", 100, 100000)
	if err == nil || !strings.Contains(err.Error(), "未在有效职位列表中") {
		t.Fatalf("expected error for non-existent job, got: %v", err)
	}
}
