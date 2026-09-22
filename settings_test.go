package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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

	savedPath, existed, err := saveDownloadedFileToDir(tempDir, "sample_notes.txt", b64)
	if err != nil {
		t.Fatalf("saveDownloadedFileToDir failed: %v", err)
	}
	if existed {
		t.Error("fresh save must report alreadyExisted=false")
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

	first, firstExisted, err := saveDownloadedFileToDir(tempDir, "document.pdf", b64)
	if err != nil {
		t.Fatal(err)
	}
	if firstExisted {
		t.Error("first save must report alreadyExisted=false")
	}
	second, secondExisted, err := saveDownloadedFileToDir(tempDir, "document.pdf", b64)
	if err != nil {
		t.Fatal(err)
	}
	if !secondExisted {
		t.Error("identical re-save must report alreadyExisted=true")
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

	first, firstExisted, err := saveDownloadedFileToDir(tempDir, "report.txt", encode("first"))
	if err != nil {
		t.Fatal(err)
	}
	if firstExisted {
		t.Error("first save must report alreadyExisted=false")
	}
	second, secondExisted, err := saveDownloadedFileToDir(tempDir, "report.txt", encode("second"))
	if err != nil {
		t.Fatal(err)
	}
	if secondExisted {
		t.Error("different content must report alreadyExisted=false")
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

// Both notification switches default to on, persist across restarts, and
// round-trip through the settings file. Older settings.json files predate
// these keys, so a fresh file must still read back as enabled.
func TestNotificationSettingsPersistAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	if got := getNotificationsEnabled(); got != true {
		t.Fatalf("fresh notifications = %v, want true", got)
	}
	if got := getNotifyOnDownload(); got != true {
		t.Fatalf("fresh notify-on-download = %v, want true", got)
	}
	if got := setNotificationsEnabled(false); got != false {
		t.Fatalf("setNotificationsEnabled(false) = %v, want false", got)
	}
	if got := setNotifyOnDownload(false); got != false {
		t.Fatalf("setNotifyOnDownload(false) = %v, want false", got)
	}
	if got := loadSettings().NotificationsEnabled; got != false {
		t.Fatalf("reloaded notifications = %v, want false", got)
	}
	if got := loadSettings().NotifyOnDownload; got != false {
		t.Fatalf("reloaded notify-on-download = %v, want false", got)
	}
	if got := setNotificationsEnabled(true); got != true {
		t.Errorf("setNotificationsEnabled(true) = %v, want true", got)
	}
	if got := setNotifyOnDownload(true); got != true {
		t.Errorf("setNotifyOnDownload(true) = %v, want true", got)
	}
}

// The open-file bridge must stay jailed: only the download folder (with
// monthly subfolders) and the internal preview dir may be opened from page
// JavaScript — never arbitrary local files.
func TestIsAllowedOpenPathJailsArbitraryFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("APPDATA", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home)

	dl := t.TempDir()
	s := loadSettings()
	s.DownloadDir = dl
	if err := saveSettings(s); err != nil {
		t.Fatal(err)
	}

	inside := filepath.Join(dl, "report.pdf")
	if err := os.WriteFile(inside, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	monthly := filepath.Join(dl, "2026-09", "photo.jpg")
	if err := os.MkdirAll(filepath.Dir(monthly), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(monthly, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	preview := filepath.Join(os.TempDir(), "WAtchfulPreview", "doc.pdf")
	for _, ok := range []string{inside, monthly, preview} {
		if !isAllowedOpenPath(ok) {
			t.Errorf("isAllowedOpenPath(%q) = false, want true", ok)
		}
	}
	outside := filepath.Join(home, "secret.txt")
	if err := os.WriteFile(outside, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	escape := filepath.Join(dl, "..", "secret.txt")
	for _, bad := range []string{"", outside, escape, "/etc/passwd"} {
		if isAllowedOpenPath(bad) {
			t.Errorf("isAllowedOpenPath(%q) = true, want false", bad)
		}
	}
}

// Oversized attachments must be rejected from the encoded length alone,
// before base64 decoding can transiently multiply memory use.
func TestSaveDownloadedFileRejectsOversizedPayload(t *testing.T) {
	old := maxAttachmentBytes
	maxAttachmentBytes = 32
	defer func() { maxAttachmentBytes = old }()

	tempDir := t.TempDir()
	big := "data:application/octet-stream;base64," + strings.Repeat("A", 48) // 48*3/4=36 > 32
	if _, _, err := saveDownloadedFileToDir(tempDir, "big.bin", big); err == nil {
		t.Fatal("oversized attachment was accepted")
	}
	if entries, _ := os.ReadDir(tempDir); len(entries) != 0 {
		t.Fatalf("rejected save left %d files behind", len(entries))
	}
	small := "data:application/octet-stream;base64," + strings.Repeat("A", 16) // 12 <= 32
	if _, _, err := saveDownloadedFileToDir(tempDir, "small.bin", small); err != nil {
		t.Fatalf("small attachment was rejected: %v", err)
	}
}

// The download folder must never point at sensitive locations: empty,
// existing files, every blocklisted prefix, or a symlink (even dangling)
// escaping into one.
func TestValidateDownloadDirRejectsSensitiveLocations(t *testing.T) {
	tempDir := t.TempDir()
	if err := validateDownloadDir(tempDir); err != nil {
		t.Errorf("valid temp dir rejected: %v", err)
	}
	if err := validateDownloadDir(""); err == nil {
		t.Error("empty download dir accepted")
	}
	notDir := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(notDir, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateDownloadDir(notDir); err == nil {
		t.Error("existing file accepted as download dir")
	}
	prefixes := blockedDownloadDirPrefixes()
	if len(prefixes) == 0 {
		t.Fatal("blocklist is empty")
	}
	for _, p := range prefixes {
		if err := validateDownloadDir(p); err == nil {
			t.Errorf("blocklisted prefix accepted: %q", p)
		}
		if err := validateDownloadDir(filepath.Join(p, "sub")); err == nil {
			t.Errorf("subfolder of blocklisted prefix accepted: %q", filepath.Join(p, "sub"))
		}
	}
	// Symlink escape, dangling or not: link -> blocked must never validate.
	link := filepath.Join(tempDir, "wa-link")
	if err := os.Symlink(prefixes[0], link); err != nil {
		t.Skipf("cannot create symlink (Windows needs privileges): %v", err)
	}
	if err := validateDownloadDir(filepath.Join(link, "sub")); err == nil {
		t.Errorf("symlink escape into %q accepted", prefixes[0])
	}
}

// A hostile settings.json pointing the download folder at a sensitive
// location must fall back to the default instead of being honored.
func TestLoadSettingsFallsBackOnHostileDownloadDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	prefixes := blockedDownloadDirPrefixes()
	raw := `{"download_dir": ` + strconv.Quote(prefixes[0]) + `}`
	if err := os.MkdirAll(getSettingsBaseDir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(getSettingsFilePath(), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings().DownloadDir; got != getDefaultDownloadDir() {
		t.Fatalf("hostile download dir honored: %q", got)
	}
}

// Every platform shell must expose the notification bridges the Settings
// cards call; a missing binding leaves the toggle promise hanging forever.
func TestNotificationBindingsPresentOnAllPlatforms(t *testing.T) {
	for _, file := range []string{"app_windows.go", "app_darwin.go", "app_linux.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			`"getNotificationsEnabledNative"`,
			`"setNotificationsEnabledNative"`,
			`"getNotifyOnDownloadNative"`,
			`"setNotifyOnDownloadNative"`,
		} {
			if !strings.Contains(string(src), want) {
				t.Errorf("%s is missing binding %s", file, want)
			}
		}
	}
}
