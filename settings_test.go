package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetUniqueFilePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wa_test_unique")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	f1 := getUniqueFilePath(tempDir, "cat.png")
	if filepath.Base(f1) != "cat.png" {
		t.Errorf("expected cat.png, got %s", filepath.Base(f1))
	}
	_ = os.WriteFile(f1, []byte("cat1"), 0644)

	f2 := getUniqueFilePath(tempDir, "cat.png")
	if filepath.Base(f2) != "cat (1).png" {
		t.Errorf("expected cat (1).png, got %s", filepath.Base(f2))
	}
	_ = os.WriteFile(f2, []byte("cat2"), 0644)

	f3 := getUniqueFilePath(tempDir, "cat.png")
	if filepath.Base(f3) != "cat (2).png" {
		t.Errorf("expected cat (2).png, got %s", filepath.Base(f3))
	}
}

func TestSaveDownloadedFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wa_test_download")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testContent := "WAtchful Light Media Test"
	b64 := "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte(testContent))

	savedPath, err := saveDownloadedFileToDir(tempDir, "sample_notes.txt", b64)
	if err != nil {
		t.Fatalf("saveDownloadedFileToDir failed: %v", err)
	}

	if !strings.HasPrefix(savedPath, tempDir) {
		t.Errorf("expected file saved in tempDir %s, got %s", tempDir, savedPath)
	}

	content, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("failed reading saved file: %v", err)
	}
	if string(content) != testContent {
		t.Errorf("content mismatch: got %q, want %q", string(content), testContent)
	}
}

func TestSaveDownloadedFileReusesIdenticalDownload(t *testing.T) {
	tempDir := t.TempDir()
	content := []byte("same WhatsApp attachment")
	b64 := "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(content)

	first, err := saveDownloadedFileToDir(tempDir, "document.pdf", b64)
	if err != nil {
		t.Fatal(err)
	}
	second, err := saveDownloadedFileToDir(tempDir, "document.pdf", b64)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("identical download created a duplicate: first=%q second=%q", first, second)
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("identical download created %d files, want 1", len(entries))
	}
}

func TestSaveDownloadedFileKeepsDifferentContent(t *testing.T) {
	tempDir := t.TempDir()
	encode := func(value string) string {
		return "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte(value))
	}

	first, err := saveDownloadedFileToDir(tempDir, "report.txt", encode("first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := saveDownloadedFileToDir(tempDir, "report.txt", encode("second"))
	if err != nil {
		t.Fatal(err)
	}
	if second == first || filepath.Base(second) != "report (1).txt" {
		t.Fatalf("different content must be preserved separately, got %q", second)
	}
}

// Zoom must survive restarts: set 75%, reload, still 75%. Settings live in
// a shared file, so point the test at a temp data home on every OS.
func TestZoomPersistsAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	if got := getZoom(); got != 1.0 {
		t.Fatalf("fresh zoom = %v, want 1.0", got)
	}
	// 75% must survive both the write and the reload: a single-decimal
	// round would silently rewrite it to 80%.
	if got := setZoom(0.75); got != 0.75 {
		t.Fatalf("setZoom(0.75) = %v, want 0.75", got)
	}
	if got := loadSettings().Zoom; got != 0.75 {
		t.Fatalf("reloaded zoom = %v, want 0.75", got)
	}
	if got := setZoom(0.67); got != 0.67 {
		t.Errorf("setZoom(0.67) = %v, want 0.67", got)
	}
	if got := setZoom(5); got != 2.0 {
		t.Errorf("setZoom(5) = %v, want clamped 2.0", got)
	}
	if got := setZoom(0.1); got != 0.5 {
		t.Errorf("setZoom(0.1) = %v, want clamped 0.5", got)
	}
	if got := setZoom(0); got != 1.0 {
		t.Errorf("setZoom(0) = %v, want unset default 1.0", got)
	}
	if got := setZoom(1.9999999999999996); got != 2.0 {
		t.Errorf("setZoom(drifted 2.0) = %v, want rounded 2.0", got)
	}
	if got := setZoom(0.7500000000000001); got != 0.75 {
		t.Errorf("setZoom(drifted 0.75) = %v, want rounded 0.75", got)
	}
}

// The app was renamed from "WhatsAppDesk" to "WAtchful". Existing installs
// keep their data in the legacy folder: it must win over a fresh new-name
// folder so no profile, setting, or session is lost across the rename.
func TestGetSettingsBaseDirPrefersLegacyFolder(t *testing.T) {
	legacy := filepath.Join(t.TempDir(), legacyAppBaseDirNames[0])
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	dir := filepath.Dir(legacy)
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	if runtime.GOOS == "darwin" {
		base := filepath.Join(dir, "Library", "Application Support")
		legacy = filepath.Join(base, legacyAppBaseDirNames[0])
		if err := os.MkdirAll(legacy, 0755); err != nil {
			t.Fatalf("mkdir mac legacy: %v", err)
		}
	}

	// With the legacy folder present, it wins.
	if got := getSettingsBaseDir(); got != legacy {
		t.Fatalf("getSettingsBaseDir() = %q, want legacy %q", got, legacy)
	}

	// Without the legacy folder, the new name wins.
	cfgRoot := dir
	if runtime.GOOS == "darwin" {
		cfgRoot = filepath.Join(dir, "Library", "Application Support")
	}
	if err := os.RemoveAll(legacy); err != nil {
		t.Fatalf("remove legacy: %v", err)
	}
	fresh := filepath.Join(cfgRoot, appBaseDirName)
	if got := getSettingsBaseDir(); got != fresh {
		t.Fatalf("getSettingsBaseDir() = %q, want fresh %q", got, fresh)
	}
}
