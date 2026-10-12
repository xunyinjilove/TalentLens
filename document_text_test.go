package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDOCXExtractsBodyTextInsteadOfZIPBytes(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "resume.docx")
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, err := z.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(`<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>中文候选人</w:t></w:r><w:r><w:t>：研发 &amp; 测试</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>项目经历</w:t><w:tab/><w:t>主导测试设计</w:t><w:br/><w:t>质量提升</w:t></w:r></w:p></w:tc></w:tr></w:tbl></w:body></w:document>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	got := NewApp().extractText(filePath)
	want := "中文候选人：研发 & 测试\n项目经历\t主导测试设计\n质量提升"
	if got != want {
		t.Fatalf("DOCX text mismatch: got %q, want %q", got, want)
	}
}

func TestBrokenDOCXNeverReturnsArchiveOrXMLBytes(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		filePath := filepath.Join(t.TempDir(), "broken.docx")
		if malformed {
			f, _ := os.Create(filePath)
			z := zip.NewWriter(f)
			w, _ := z.Create("word/document.xml")
			w.Write([]byte(`<document><t>incomplete`))
			z.Close()
			f.Close()
		} else {
			os.WriteFile(filePath, []byte("PK invalid archive"), 0600)
		}
		if got := NewApp().extractText(filePath); strings.TrimSpace(got) != "" {
			t.Fatalf("invalid DOCX must not become resume text: %q", got)
		}
	}
}
