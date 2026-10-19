package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTranslateResumeSectionsCachesWithoutChangingResume(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": "Translated section"}}}})
	}))
	defer server.Close()
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	a.config.AI = AIConfig{APIKey: "test", BaseURL: server.URL, Model: "test", MaxRetries: 1}
	resume := Resume{ID: "resume_1", Content: "【工作经历】\n公司：测试科技\n职位：测试工程师"}
	writeLegacyJSON(t, a.getDataDir(), "resumes", resume.ID, &resume)
	sections := []string{"工作经历\n公司：测试科技\n职位：测试工程师"}
	for i := 0; i < 2; i++ {
		result, err := a.TranslateResumeSections(resume.ID, "en", false, sections)
		if err != nil || len(result) != 1 || result[0] != "Translated section" {
			t.Fatalf("translation %d: result=%v error=%v", i, result, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("cached call should not invoke model again; got %d calls", calls.Load())
	}
	if _, err := a.TranslateResumeSections(resume.ID, "en", true, sections); err != nil || calls.Load() != 2 {
		t.Fatalf("forced refresh should invoke model: calls=%d err=%v", calls.Load(), err)
	}
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resume.ID+".json"))
	if err != nil || !strings.Contains(string(data), "测试工程师") || strings.Contains(string(data), "Translated section") {
		t.Fatalf("translation modified resume source: %v", err)
	}
}

func TestSplitTranslationTextPreservesLongContent(t *testing.T) {
	source := strings.Repeat("任职经历。", 700)
	parts := splitTranslationText(source, 120)
	if len(parts) < 2 || strings.Join(parts, "") != source {
		t.Fatalf("chunks lost source text")
	}
	for _, part := range parts {
		if len([]rune(part)) > 120 {
			t.Fatalf("chunk exceeded limit")
		}
	}
}
