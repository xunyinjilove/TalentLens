package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCommunicationJobSyncContinuesAfterUnusableCandidate(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	p := a.CreateProject("职位同步回归", "研发", 1, "HR", "", &JobConfig{Title: "测试工程师"})
	for i := 1; i <= 2; i++ {
		resume := &Resume{ID: fmt.Sprintf("sync_context_%d", i), ProjectID: p.ID,
			FileName: fmt.Sprintf("测试人选%d.txt", i), Status: "pending", Content: "用于职位同步的测试记录",
			URL: fmt.Sprintf("https://ehire.51job.com/Revision/talent/resume/detail?resumeId=%d", i)}
		if err := a.persistSearchCandidate(resume); err != nil {
			t.Fatal(err)
		}
	}
	var calls []string
	result := a.syncCommunicationJobs(p.ID, "51job", nil, func(id, keyword string) JobConversationResult {
		calls = append(calls, id)
		if len(calls) == 1 {
			return JobConversationResult{Status: "error", Message: "候选人页面沟通入口未就绪，已自动重试；本次未发送"}
		}
		return JobConversationResult{Status: "jobs", Jobs: []JobCommunicationOption{{ID: "job_sync_1", Title: "测试工程师", CompanyID: "company_sync"}}}
	})
	if result.SyncStatus != "complete" || len(result.Jobs) != 1 || result.Jobs[0].ID != "job_sync_1" {
		t.Fatalf("单个候选人没有入口不能使职位同步永久失败：calls=%v, status=%s, error=%s, jobs=%d", calls, result.SyncStatus, result.SyncMessage, len(result.Jobs))
	}
	if len(calls) != 2 || calls[0] == calls[1] {
		t.Fatalf("应只尝试不同读取上下文：%v", calls)
	}
}

func TestCommunicationJobSyncDoesNotRequireCandidates(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	accountCalls := 0
	result := a.syncCommunicationJobs("", "51job", func(keyword string) JobConversationResult {
		accountCalls++
		return JobConversationResult{Status: "jobs", Jobs: []JobCommunicationOption{{ID: "account_job_1", Title: "测试工程师", CompanyID: "company_1"}}}
	}, func(id, keyword string) JobConversationResult {
		t.Fatal("账号职位目录可读时，不得再打开候选人或Hi聊入口")
		return JobConversationResult{}
	})
	if accountCalls != 1 || result.SyncStatus != "complete" || len(result.Jobs) != 1 {
		t.Fatalf("无候选人也应能完成职位同步：calls=%d, result=%+v", accountCalls, result)
	}
}

func TestCommunicationJobSyncPreservesPartialDirectory(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	result := a.syncCommunicationJobs("", "51job", func(string) JobConversationResult {
		return JobConversationResult{Status: "jobs", HasMore: true, Jobs: []JobCommunicationOption{{ID: "partial_job", Title: "测试工程师"}}}
	}, nil)
	if result.SyncStatus != "partial" || len(result.Jobs) != 1 || !result.NeedsRefresh {
		t.Fatalf("未完成分页不能标为全量同步成功：%+v", result)
	}
}

func TestFailedCommunicationJobRefreshKeepsCacheAndLastSuccessfulTime(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	if err := a.SaveCommunicationJobs("51job", "", []JobCommunicationOption{{ID: "old_job", Title: "已有职位"}}); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	if err := a.SaveCommunicationJobSyncSnapshot(&CommunicationJobSyncSnapshot{ID: "old_complete", Platform: "51job", Status: "complete", StartedAt: oldTime, FinishedAt: oldTime}); err != nil {
		t.Fatal(err)
	}
	result := a.syncCommunicationJobs("", "51job", func(string) JobConversationResult {
		return JobConversationResult{Status: "needs_login", Message: "请登录企业账号"}
	}, func(string, string) JobConversationResult {
		t.Fatal("登录失效时不应继续轮换候选人读取")
		return JobConversationResult{}
	})
	if result.SyncStatus != "failed" || len(result.Jobs) != 1 || result.LastSyncedAt != oldTime || !result.NeedsRefresh {
		t.Fatalf("失败不能清空缓存或把旧目录变新：%+v", result)
	}
}

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
		ID:         "sync_test",
		Platform:   "51job",
		Status:     "complete",
		TotalJobs:  2,
		StartedAt:  now,
		FinishedAt: now,
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
		ID:        "r_test_1",
		ProjectID: p.ID,
		FileName:  "张三_临床试验.pdf",
		Content:   "多年临床试验经验",
		Status:    "done",
		Analysis:  &AnalysisResult{CandidateName: "张三", OverallScore: 85},
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
		ID:        "r_test_2",
		ProjectID: p.ID,
		FileName:  "李四_临床试验.pdf",
		Content:   "多年临床试验经验",
		Status:    "done",
		Analysis:  &AnalysisResult{CandidateName: "李四", OverallScore: 90},
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
