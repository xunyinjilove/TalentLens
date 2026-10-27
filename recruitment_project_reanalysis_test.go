package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestProjectReanalysisRequiresConfiguredModelAndUsesCurrentLibrary(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	p := a.CreateProject("临床项目经理", "临床", 1, "HR", "", &JobConfig{
		Title: "临床项目经理", RequiredSkills: []string{"GCP合规"},
	})
	r := &Resume{ID: "batch_real_model", ProjectID: p.ID, FileName: "候选人.txt", Status: "pending",
		Content: "具有五年临床项目管理经验，负责GCP合规、中心协调与临床试验交付。", CreatedAt: time.Now()}
	if err := a.persistSearchCandidate(r); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartProjectBatchReanalysis(p.ID); err == nil {
		t.Fatal("未配置模型时必须直接报错")
	}

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		answer := `{"overall_score":82,"skill_match":82,"experience_match":82,"education_match":80,"summary":"具备相关经验","recommendation":"recommend"}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}}}})
	}))
	defer server.Close()
	a.config.AI = AIConfig{APIKey: "test", BaseURL: server.URL, Model: "test", MaxRetries: 1}
	start, err := a.StartProjectBatchReanalysis(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if start.Total != 1 {
		t.Fatalf("want 1 current-library resume, got %d", start.Total)
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		if err := db.QueryRow(`SELECT status FROM project_reanalysis_runs WHERE id=?`, start.RunID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status == "completed" || status == "partial" {
			if calls.Load() == 0 {
				t.Fatal("批量完成却没有真实调用模型 API")
			}
			if status != "completed" {
				t.Fatalf("批量分析未完成: %s", status)
			}
			data, err := readJSONFile(filepath.Join(a.getDataDir(), "resumes", r.ID+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var saved Resume
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Status != "done" || saved.Analysis == nil {
				t.Fatalf("模型结果未写入当前简历: %+v", saved)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("批量分析超时")
}

func TestAssessmentResumesUseActiveLibraryLinks(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	p := a.CreateProject("大客户销售经理", "销售", 1, "HR", "", nil)
	r := &Resume{ID: "assessment_active", ProjectID: p.ID, FileName: "销售简历.txt", Status: "pending",
		Content: "八年销售经验", CreatedAt: time.Now()}
	if err := a.persistSearchCandidate(r); err != nil {
		t.Fatal(err)
	}
	got, err := a.GetProjectAssessmentResumes(p.ID)
	if err != nil || len(got) != 1 || got[0].ID != r.ID {
		t.Fatalf("测评应读取人才库当前有效简历: %+v, %v", got, err)
	}
	db, err := a.getRecruitmentDB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE project_candidates SET archived_at='2026-01-01' WHERE project_id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	got, err = a.GetProjectAssessmentResumes(p.ID)
	if err != nil || len(got) != 0 {
		t.Fatalf("归档简历不应再出现在测评: %+v, %v", got, err)
	}
}

func TestProjectReanalysisWorkerPoolAndLiveProgress(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()

	p := a.CreateProject("算法专家", "技术", 3, "HR", "", &JobConfig{
		Title: "算法专家", RequiredSkills: []string{"Go", "Python"},
	})

	for i := 1; i <= 3; i++ {
		r := &Resume{
			ID:        fmt.Sprintf("cand_%d", i),
			ProjectID: p.ID,
			FileName:  fmt.Sprintf("候选人_%d.txt", i),
			Status:    "pending",
			Content:   fmt.Sprintf("具备大模型工程经验，精通Go并发与Python开发，候选人序号 %d", i),
			CreatedAt: time.Now(),
		}
		if err := a.persistSearchCandidate(r); err != nil {
			t.Fatal(err)
		}
	}

	var active atomic.Int32
	var maxActive atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		curr := active.Add(1)
		for {
			oldMax := maxActive.Load()
			if curr <= oldMax || maxActive.CompareAndSwap(oldMax, curr) {
				break
			}
		}
		time.Sleep(80 * time.Millisecond)
		active.Add(-1)

		answer := `{"overall_score":88,"skill_match":85,"experience_match":90,"education_match":85,"summary":"并发评测合格","recommendation":"recommend"}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}}}})
	}))
	defer server.Close()

	a.config.AI = AIConfig{APIKey: "test", BaseURL: server.URL, Model: "test", MaxRetries: 1}
	start, err := a.StartProjectBatchReanalysis(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if start.Total != 3 {
		t.Fatalf("expected 3 targets, got %d", start.Total)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := a.GetProjectReanalysisStatus(start.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if status.Status == "completed" {
			if status.Completed != 3 {
				t.Fatalf("expected 3 completed, got %d", status.Completed)
			}
			if maxActive.Load() <= 1 {
				t.Fatalf("expected concurrent worker pool (maxActive > 1), got %d", maxActive.Load())
			}
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("批量并发分析超时未完成")
}
