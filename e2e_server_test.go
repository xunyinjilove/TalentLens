//go:build dev && e2e

package main

import (
	"archive/zip"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// This harness uses the real App and Wails bindings, but never the user's data.
// It is excluded from regular tests and production builds.
func TestMain(m *testing.M) {
	dir := os.Getenv("TALENTLENS_E2E_DATA_DIR")
	if dir == "" {
		os.Exit(m.Run())
	}
	if !filepath.IsAbs(dir) {
		log.Fatal("TALENTLENS_E2E_DATA_DIR must be an absolute isolated directory")
	}
	app := NewApp()
	app.dataDirOverride = dir
	// Synthetic resume: exercise the real DOCX extraction and import path.
	fixture, err := os.Create(filepath.Join(dir, "e2e-resume.docx"))
	if err != nil {
		log.Fatal(err)
	}
	archive := zip.NewWriter(fixture)
	for name, body := range map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"word/document.xml":   `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>E2E虚构候选人。测试工程师，三年工作经验，本科学历。主导测试设计方案，参与缺陷分析报告，负责性能测试项目。本文件仅用于自动化测试。</w:t></w:r></w:p></w:body></w:document>`,
	} {
		entry, err := archive.Create(name)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			log.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		log.Fatal(err)
	}
	if err := fixture.Close(); err != nil {
		log.Fatal(err)
	}
	if err := wails.Run(&options.App{
		Title:    "TalentLens isolated E2E",
		LogLevel: logger.WARNING,
		Width:    1200, Height: 800, StartHidden: true,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup,
		Bind:        []interface{}{app},
		Windows:     &windows.Options{WebviewUserDataPath: filepath.Join(dir, "webview")},
	}); err != nil {
		log.Fatal(err)
	}
}
