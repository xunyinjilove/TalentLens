package main

import (
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
