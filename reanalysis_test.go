package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestArchivePriorAnalysisPreservesExactPreviousSnapshot(t *testing.T) {
	app := NewApp()
	app.dataDirOverride = t.TempDir()
	old := []byte(`{"status":"done","score":82,"analysis":{"summary":"旧测评"}}`)
	if err := app.archivePriorAnalysis("resume_1", old); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(filepath.Join(app.getDataDir(), "analysis_history"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected one archived evaluation, files=%d err=%v", len(files), err)
	}
	archived, err := os.ReadFile(filepath.Join(app.getDataDir(), "analysis_history", files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(archived, old) {
		t.Fatal("archived evaluation must match the previous saved result")
	}
}
