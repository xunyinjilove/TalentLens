package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearProjectResumesKeepsOtherProjectsAndOriginalFiles(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	first := a.CreateProject("项目 A", "", 1, "", "", &JobConfig{})
	second := a.CreateProject("项目 B", "", 1, "", "", &JobConfig{})
	original := filepath.Join(a.getDataDir(), "candidate.pdf")
	if err := os.WriteFile(original, []byte("original file"), 0644); err != nil {
		t.Fatal(err)
	}
	export := filepath.Join(a.getDataDir(), "exports", "report.xlsx")
	if err := os.MkdirAll(filepath.Dir(export), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(export, []byte("existing report"), 0644); err != nil {
		t.Fatal(err)
	}
	a.saveResume(&Resume{ID: "resume_a", ProjectID: first.ID, FilePath: original, Status: "done", ContactStatus: "greeted", Analysis: &AnalysisResult{OverallScore: 90}})
	a.saveResume(&Resume{ID: "resume_b", ProjectID: second.ID, Status: "done"})
	first.ResumeIDs = []string{"resume_a", "resume_b", "missing_resume"} // 错误引用不能让 B 的简历被删除
	first.Status = "completed"
	second.ResumeIDs = []string{"resume_b"}
	a.saveProject(first)
	a.saveProject(second)

	if err := a.ClearProjectResumes(first.ID); err != nil {
		t.Fatal(err)
	}
	gotFirst := a.GetProject(first.ID)
	if gotFirst == nil || len(gotFirst.ResumeIDs) != 0 || gotFirst.Status != "draft" {
		t.Fatalf("项目 A 未重置: %+v", gotFirst)
	}
	if gotSecond := a.GetProject(second.ID); gotSecond == nil || len(gotSecond.ResumeIDs) != 1 {
		t.Fatalf("项目 B 被修改: %+v", gotSecond)
	}
	if _, err := os.Stat(filepath.Join(a.getDataDir(), "resumes", "resume_a.json")); !os.IsNotExist(err) {
		t.Fatalf("项目 A 的简历记录未删除: %v", err)
	}
	for _, path := range []string{original, export, filepath.Join(a.getDataDir(), "resumes", "resume_b.json")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("应保留的文件丢失 %s: %v", path, err)
		}
	}
}

func TestClearProjectResumesRejectsActiveAnalysis(t *testing.T) {
	a := NewApp()
	a.dataDirOverride = t.TempDir()
	defer a.closeRecruitmentDB()
	p := a.CreateProject("分析中", "", 1, "", "", &JobConfig{})
	a.saveResume(&Resume{ID: "resume_1", ProjectID: p.ID, Status: "analyzing"})
	p.ResumeIDs = []string{"resume_1"}
	a.saveProject(p)
	a.activeAnalyses = map[string]bool{"resume_1": true}

	if err := a.ClearProjectResumes(p.ID); err == nil {
		t.Fatal("分析中不应允许清空")
	}
	if got := a.GetProject(p.ID); got == nil || len(got.ResumeIDs) != 1 {
		t.Fatalf("拒绝清空后项目记录应保持不变: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(a.getDataDir(), "resumes", "resume_1.json")); err != nil {
		t.Fatalf("拒绝清空后简历记录应保留: %v", err)
	}
	delete(a.activeAnalyses, "resume_1")
	a.activeProjects = map[string]bool{p.ID: true}
	if err := a.ClearProjectResumes(p.ID); err == nil {
		t.Fatal("项目批量分析中不应允许清空")
	}
}
