package main

import (
	"strings"
	"testing"
	"time"
)

func TestSnapshotIgnoresSearchMetadataButPreservesCandidateFacts(t *testing.T) {
	first := &Resume{Content: "【前程无忧 真实推荐牛人档案】\n姓名：合成人选\n分流同义词：测试开发\n目标城市：上海\n工作经历：真实项目"}
	next := &Resume{Content: strings.ReplaceAll(first.Content, "测试开发", "自动化测试")}
	if resumeSnapshotFingerprint(first) != resumeSnapshotFingerprint(next) {
		t.Fatal("search synonyms must not create new document snapshots")
	}
	next.Content = strings.ReplaceAll(first.Content, "真实项目", "不同项目")
	if resumeSnapshotFingerprint(first) == resumeSnapshotFingerprint(next) {
		t.Fatal("changed candidate facts must remain a different snapshot")
	}
}

func TestProjectListCollapsesIdenticalSnapshotsWithoutPickingHighestScore(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	project := &Project{ID: "project_snapshots", ResumeIDs: []string{"older", "newer"}}
	writeLegacyJSON(t, dir, "projects", project.ID, project)
	for i, id := range project.ResumeIDs {
		score := 90 - i*20
		writeLegacyJSON(t, dir, "resumes", id, &Resume{ID: id, ProjectID: project.ID, Content: "同一份完整的合成简历文档，包含相同的姓名、工作经历、教育经历、项目记录和技能。", Status: "done", Score: score, Analysis: &AnalysisResult{CoreMatch: float64(score)}, CreatedAt: now.Add(time.Duration(i) * time.Hour)})
	}
	app := NewApp()
	app.dataDirOverride = dir
	defer app.closeRecruitmentDB()
	got := app.GetProjectResumes(project.ID)
	if len(got) != 1 || got[0].ID != "newer" {
		t.Fatalf("identical snapshot should use latest evaluation, not highest score: count=%d", len(got))
	}
	if len(got[0].SnapshotCopyIDs) != 1 {
		t.Fatal("original copies must remain available as history")
	}
	history, err := app.GetResumeSnapshotHistory("newer")
	if err != nil || len(history) != 2 || history[0].Score != 70 || history[1].Score != 90 {
		t.Fatalf("original evaluation scores must remain reviewable: %+v %v", history, err)
	}
	known, err := app.isKnownProjectSource(project.ID, &Resume{ID: "new_capture", Content: got[0].Content})
	if err != nil || !known {
		t.Fatalf("identical anonymous capture should be skipped before AI analysis: %v", err)
	}
}

func TestDifferentVerifiedPeopleAndDifferentDocumentsStaySeparate(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	project := &Project{ID: "project_separate", ResumeIDs: []string{"first", "second", "third"}}
	writeLegacyJSON(t, app.getDataDir(), "projects", project.ID, project)
	for i, id := range project.ResumeIDs {
		r := &Resume{ID: id, ProjectID: project.ID, FileName: "同名.txt", Content: "相同的合成文档", SourcePlatform: "51job", PlatformCandidateID: "100"}
		if i == 1 {
			r.PlatformCandidateID = "200"
		}
		if i == 2 {
			r.Content = "不同的合成文档"
			r.PlatformCandidateID = ""
		}
		writeLegacyJSON(t, app.getDataDir(), "resumes", id, r)
	}
	if got := app.GetProjectResumes(project.ID); len(got) != 3 {
		t.Fatalf("name or conflicting platform IDs cannot collapse people: %d", len(got))
	}
}
