package main

import "testing"

func TestProfileBindingRejectsConflictingStableIDBeforeBrowserAction(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	writeLegacyJSON(t, app.getDataDir(), "resumes", "candidate_id", &Resume{ID: "candidate_id", SourcePlatform: "51job", PlatformCandidateID: "123", URL: "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=123"})
	got := app.Bind51JobCandidateDetail("candidate_id", "https://ehire.51job.com/Revision/talent/resume/detail?resumeId=456")
	if got.Status != "error" {
		t.Fatalf("must reject mismatched identity before opening browser: %+v", got)
	}
}
