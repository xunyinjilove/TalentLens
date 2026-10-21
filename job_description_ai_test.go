package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateJobDescriptionDraftRequiresConfiguredModel(t *testing.T) {
	_, err := (&App{}).GenerateJobDescriptionDraft(JobDescriptionDraftRequest{Title: "测试工程师", Direction: "自动化测试"})
	if err == nil || !strings.Contains(err.Error(), "AI 设置") {
		t.Fatalf("expected clear AI configuration error, got %v", err)
	}
}

func TestGenerateJobDescriptionDraftAmbiguousDirection(t *testing.T) {
	a := &App{}
	res, err := a.GenerateJobDescriptionDraft(JobDescriptionDraftRequest{Title: "器械工程师"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "needs_direction" {
		t.Fatalf("expected needs_direction, got %s", res.Status)
	}
	if len(res.SuggestedDirections) < 3 {
		t.Fatalf("expected multiple direction suggestions, got %v", res.SuggestedDirections)
	}
}

func TestGenerateJobDescriptionDraftConflictIsolationAndGeneration(t *testing.T) {
	requests := 0
	var receivedPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(body.Messages) >= 2 {
			receivedPrompt = body.Messages[1].Content
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"content": "一、岗位职责\n1. 负责医疗器械机械结构设计与三维建模。\n2. 参与样品试制与可靠性测试。\n\n二、任职要求\n1. 熟练掌握 SolidWorks、ProE 等机械设计软件。\n2. 具备良好的力学与公差分析基础。"},
		}}})
	}))
	defer server.Close()

	a := &App{config: Config{AI: AIConfig{APIKey: "synthetic", BaseURL: server.URL, Model: "test-model"}}}

	// 测试：器械工程师，方向为机械结构设计，但传入了历史前端遗留技能 Vue3, TypeScript 以及本岗位技能 SolidWorks
	res, err := a.GenerateJobDescriptionDraft(JobDescriptionDraftRequest{
		Title:     "器械工程师",
		Direction: "机械结构与外观设计方向",
		Skills:    []string{"Vue3", "TypeScript", "SolidWorks"},
		Mode:      "fresh",
	})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if res.Status != "ready" {
		t.Fatalf("expected ready status, got %s", res.Status)
	}

	// 1. 冲突应被精准捕获
	if len(res.Conflicts) == 0 {
		t.Fatal("expected Vue3 and TypeScript to be flagged as conflicts")
	}
	hasVueConflict := false
	for _, c := range res.Conflicts {
		if strings.Contains(c, "Vue3") {
			hasVueConflict = true
		}
	}
	if !hasVueConflict {
		t.Fatalf("expected Vue3 conflict, got: %v", res.Conflicts)
	}

	// 2. 发送给大模型的 Prompt 中绝对不能包含被隔离的前端技能，但必须包含合法技能 SolidWorks
	if strings.Contains(receivedPrompt, "Vue3") || strings.Contains(receivedPrompt, "TypeScript") {
		t.Fatalf("prompt leaked isolated frontend skills: %s", receivedPrompt)
	}
	if !strings.Contains(receivedPrompt, "SolidWorks") {
		t.Fatalf("prompt missing legitimate mechanical skill SolidWorks: %s", receivedPrompt)
	}

	// 3. 校验输出包含结构化条目
	if len(res.Responsibilities) != 2 || len(res.Requirements) != 2 {
		t.Fatalf("unexpected parsed items: resp=%d req=%d", len(res.Responsibilities), len(res.Requirements))
	}
	if !strings.Contains(res.Draft, "SolidWorks") {
		t.Fatalf("draft missing expected content: %s", res.Draft)
	}
}

func TestGenerateJobDescriptionDraftFrontendJobAllowsFrontendSkills(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"content": "一、岗位职责\n1. 负责 Web 前端核心业务开发。\n\n二、任职要求\n1. 熟练掌握 Vue3 与 TypeScript。"},
		}}})
	}))
	defer server.Close()

	a := &App{config: Config{AI: AIConfig{APIKey: "synthetic", BaseURL: server.URL, Model: "test-model"}}}

	// 前端岗位传入 Vue3/TypeScript 不得被误伤为冲突
	res, err := a.GenerateJobDescriptionDraft(JobDescriptionDraftRequest{
		Title:  "Web 前端开发工程师",
		Skills: []string{"Vue3", "TypeScript", "性能优化"},
	})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("frontend job should not flag frontend skills as conflicts: %v", res.Conflicts)
	}
	if res.Status != "ready" {
		t.Fatalf("expected ready status, got %s", res.Status)
	}
}
