package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAttachResumeFilePreservesVersionsAndSkipsIdenticalContent(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	resume := &Resume{ID: "attachment_test", FileName: "在线简历.txt", Content: "在线简历", CreatedAt: time.Now()}
	writeLegacyJSON(t, app.dataDirOverride, "resumes", resume.ID, resume)
	first := filepath.Join(app.dataDirOverride, "first.txt")
	second := filepath.Join(app.dataDirOverride, "second.txt")
	if err := os.WriteFile(first, []byte("第一份真实附件简历，包含工作经历、项目经验和教育经历。"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("第二份更新的真实附件简历，包含新的工作经历、项目经验和教育经历。"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AttachResumeFile(resume.ID, first); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AttachResumeFile(resume.ID, first); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AttachResumeFile(resume.ID, second); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(app.dataDirOverride, "resumes", resume.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved Resume
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.AttachmentPath != second || len(saved.AttachmentHistory) != 2 || saved.AttachmentHistory[0].Path != first {
		t.Fatalf("current=%s versions=%+v", saved.AttachmentPath, saved.AttachmentHistory)
	}
}

func TestLegacyDocIsRetainedWithoutImportingBinaryAsResume(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	defer app.closeRecruitmentDB()
	resume := &Resume{ID: "legacy_doc_test", FileName: "在线简历.txt", Content: "在线简历", CreatedAt: time.Now()}
	writeLegacyJSON(t, app.dataDirOverride, "resumes", resume.ID, resume)
	path := filepath.Join(app.dataDirOverride, "legacy.doc")
	if err := os.WriteFile(path, append([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, make([]byte, 128)...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AttachResumeFile(resume.ID, path); err == nil {
		t.Fatal("binary DOC imported as text")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("original attachment was lost", err)
	}
	data, err := os.ReadFile(filepath.Join(app.dataDirOverride, "resumes", resume.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved Resume
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.HasAttachment || saved.AttachmentContent != "" || saved.AttachmentPendingReason == "" {
		t.Fatalf("invalid attachment state: %+v", saved)
	}
}
