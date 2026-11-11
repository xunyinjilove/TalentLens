package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalysisVersionKeyTracksSourceAndJobButNotDerivedRadar(t *testing.T) {
	resume := &Resume{ProjectID: "project_cache", Content: "Original resume"}
	job := &JobConfig{Title: "Engineer", RequiredSkills: []string{"Go"}}
	base := analysisVersionKey(resume, job)
	job.RadarMetrics = []string{"Go"}
	job.RadarProfileHash = "generated"
	job.RadarProfileVersion = 2
	if analysisVersionKey(resume, job) != base {
		t.Fatal("derived radar data must not reset the analysis allowance")
	}
	resume.Content += " changed"
	if analysisVersionKey(resume, job) == base {
		t.Fatal("changed resume content must reset the allowance")
	}
	resume.Content = "Original resume"
	job.RequiredSkills = []string{"SQL"}
	if analysisVersionKey(resume, job) == base {
		t.Fatal("changed job requirements must reset the allowance")
	}
}

func TestAnalyzeResumeLegacyResultGetsOneMoreRoundThenUsesCache(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	resume := &Resume{ID: "resume_cache", Content: "Candidate has engineering experience", Status: "done", Analysis: &AnalysisResult{Summary: "legacy"}}
	a.saveResume(resume)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `{"overall_score":80,"summary":"second result"}`}}}})
	}))
	defer server.Close()
	cfg := &AIConfig{APIKey: "test", BaseURL: server.URL, Model: "test", MaxRetries: 1}
	second, err := a.AnalyzeResume(resume.ID, cfg, &JobConfig{Title: "Engineer"})
	if err != nil || second.Summary != "second result" {
		t.Fatalf("second successful business round: result=%+v err=%v", second, err)
	}
	data, err := os.ReadFile(filepath.Join(a.getDataDir(), "resumes", resume.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved Resume
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.AnalysisSuccessfulRounds != 2 || saved.AnalysisCacheKey == "" {
		t.Fatalf("second result was not durably capped: %+v", saved)
	}
	if want := analysisVersionKey(&saved, &JobConfig{Title: "Engineer"}); saved.AnalysisCacheKey != want {
		t.Fatalf("saved key drifted: saved=%s expected=%s content=%q", saved.AnalysisCacheKey, want, saved.Content)
	}
	callsBeforeCache := calls
	cached, err := a.AnalyzeResume(resume.ID, nil, &JobConfig{Title: "Engineer"})
	if err != nil || cached.Summary != "second result" || calls != callsBeforeCache {
		t.Fatalf("third request must reuse saved result without AI: result=%+v calls=%d before=%d err=%v", cached, calls, callsBeforeCache, err)
	}
}
