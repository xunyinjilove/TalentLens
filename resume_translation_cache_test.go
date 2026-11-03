package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadCachedTranslationNeverStartsAIAndInvalidatesOnSourceChange(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	resume := &Resume{ID: "resume_translation_cache", Content: "Work Experience: Built services"}
	a.saveResume(resume)
	sections := []string{"【工作经历】\nWork Experience: Built services"}
	if got, err := a.GetCachedResumeTranslation(resume.ID, "zh", sections); err != nil || got != nil {
		t.Fatalf("missing cache should be empty: %v %v", got, err)
	}
	path := filepath.Join(a.getDataDir(), "translations", resume.ID+"_zh.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(resumeTranslationCache{SourceHash: resumeTranslationHash(resume.Content, "zh", sections), Language: "zh", Sections: []string{"【工作经历】\n工作经历：构建服务"}})
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := a.GetCachedResumeTranslation(resume.ID, "zh", sections); err != nil || len(got) != 1 {
		t.Fatalf("saved translation not returned: %v %v", got, err)
	}
	resume.Content += " updated"
	a.saveResume(resume)
	if got, err := a.GetCachedResumeTranslation(resume.ID, "zh", sections); err != nil || got != nil {
		t.Fatalf("stale translation must not be shown: %v %v", got, err)
	}
}
