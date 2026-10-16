package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateJobDescriptionDraftRequiresConfiguredModel(t *testing.T) {
	_, err := (&App{}).GenerateJobDescriptionDraft(JobDescriptionDraftRequest{Title: "测试工程师"})
	if err == nil || !strings.Contains(err.Error(), "AI 设置") {
		t.Fatalf("expected clear AI configuration error, got %v", err)
	}
}

func TestGenerateJobDescriptionDraftReturnsEditablePreview(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(body.Messages) < 2 || !strings.Contains(body.Messages[1].Content, "测试工程师") {
			t.Error("job facts were not included in the draft request")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"content": "岗位职责\n1. 编写测试用例。\n\n任职要求\n1. 熟悉自动化测试。"},
		}}})
	}))
	defer server.Close()
	a := &App{config: Config{AI: AIConfig{APIKey: "synthetic", BaseURL: server.URL, Model: "test-model"}}}
	draft, err := a.GenerateJobDescriptionDraft(JobDescriptionDraftRequest{Title: "测试工程师", Skills: []string{"自动化测试"}})
	if err != nil || requests != 1 || !strings.Contains(draft, "任职要求") {
		t.Fatalf("draft=%q requests=%d error=%v", draft, requests, err)
	}
}
