package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnalysisPromptShowsEscapedNewlinesInJSONExample(t *testing.T) {
	prompt := (&App{}).buildAnalysisPrompt(&Resume{FileName: "synthetic.txt", Content: "synthetic resume"}, &JobConfig{Title: "Synthetic role"})
	if !strings.Contains(prompt, `\n👤`) {
		t.Fatal("manager_pitch JSON example must escape its line breaks")
	}
	if strings.Contains(prompt, "）：\n👤") {
		t.Fatal("manager_pitch example contains a literal line break inside a JSON string")
	}
}

func TestParseAnalysisResultRepairsLiteralLineBreakInString(t *testing.T) {
	content := "{\"overall_score\":80,\"manager_pitch\":\"first line\nsecond line\",\"summary\":\"synthetic\"}"
	result, err := (&App{}).parseAnalysisResult(content)
	if err != nil {
		t.Fatalf("literal newline in model JSON string must be repaired: %v", err)
	}
	if result.ManagerPitch != "first line\nsecond line" {
		t.Fatalf("unexpected repaired pitch: %q", result.ManagerPitch)
	}
}

func TestParseAnalysisResultAcceptsWaterCheckGapsString(t *testing.T) {
	content := `{"overall_score":80,"water_check":{"water_score":10,"gaps":"No significant gap"}}`
	result, err := (&App{}).parseAnalysisResult(content)
	if err != nil {
		t.Fatalf("model string for water_check.gaps should be normalized: %v", err)
	}
	if len(result.WaterCheck.Gaps) != 1 || result.WaterCheck.Gaps[0] != "No significant gap" {
		t.Fatalf("unexpected gaps: %#v", result.WaterCheck.Gaps)
	}
}

func TestAnalysisRetriesMalformedModelJSONOnce(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		answer := `{"overall_score":80,bad}`
		if requests == 2 {
			answer = `{"overall_score":82,"summary":"valid"}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}})
	}))
	defer server.Close()
	result, err := (&App{}).analyzeModelWithFormatRetry(&AIConfig{APIKey: "synthetic", BaseURL: server.URL, Model: "test-model"}, "synthetic prompt")
	if err != nil || result.OverallScore != 82 || requests != 2 {
		t.Fatalf("expected one format retry: score=%v requests=%d err=%v", result, requests, err)
	}
}

func TestAutoRetryOnlyOldFormatFailuresOnce(t *testing.T) {
	resume := &Resume{Status: "error", ErrorMessage: "解析AI返回失败: JSON解析失败"}
	if !shouldAutoRetryFormatFailure(resume) {
		t.Fatal("old format failure should be retried")
	}
	resume.AnalysisAutoRetryVersion = analysisFormatRetryVersion
	if shouldAutoRetryFormatFailure(resume) {
		t.Fatal("already attempted record must not spend credits again")
	}
	resume.AnalysisAutoRetryVersion = ""
	resume.ErrorMessage = "AI 服务账户余额不足"
	if shouldAutoRetryFormatFailure(resume) {
		t.Fatal("provider billing errors must not be automatically retried")
	}
}
