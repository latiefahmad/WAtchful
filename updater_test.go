package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDarwinUpdaterSelectsOnlyMacArchive(t *testing.T) {
	release := &GitHubRelease{Assets: []GitHubAsset{
		{Name: "WAtchful-Windows-x64.zip", BrowserDownloadURL: "https://example.test/windows.zip"},
		{Name: "WAtchful-macOS-Universal.zip", BrowserDownloadURL: "https://example.test/macos.zip"},
	}}
	asset := findAssetForOS(release, "darwin")
	if asset == nil || asset.Name != "WAtchful-macOS-Universal.zip" {
		t.Fatalf("macOS self-update must select the macOS archive, got %#v", asset)
	}
}

func TestLinuxUpdaterSelectsOnlyLinuxArchive(t *testing.T) {
	release := &GitHubRelease{Assets: []GitHubAsset{
		{Name: "unrelated.tar.gz", BrowserDownloadURL: "https://example.test/unrelated.tar.gz"},
		{Name: "WAtchful-Linux-x64.tar.gz", BrowserDownloadURL: "https://example.test/linux.tar.gz"},
	}}
	asset := findAssetForOS(release, "linux")
	if asset == nil || asset.Name != "WAtchful-Linux-x64.tar.gz" {
		t.Fatalf("Linux self-update must select the Linux archive, got %#v", asset)
	}
}

func TestLinuxUpdaterSelectsPortableArchive(t *testing.T) {
	release := &GitHubRelease{Assets: []GitHubAsset{
		{Name: "WAtchful-Linux-amd64.deb", BrowserDownloadURL: "https://example.test/app.deb"},
		{Name: "WAtchful-Linux-x64.tar.gz", BrowserDownloadURL: "https://example.test/app.tar.gz"},
	}}
	asset := findAssetForOS(release, "linux")
	if asset == nil || asset.Name != "WAtchful-Linux-x64.tar.gz" {
		t.Fatalf("Linux self-update must select tar.gz, got %#v", asset)
	}
	if got := updateDownloadExtension(asset.BrowserDownloadURL); got != ".tar.gz" {
		t.Fatalf("download extension = %q, want .tar.gz", got)
	}
}

func TestLinuxArm64UpdaterNeverFallsBackToX64(t *testing.T) {
	release := &GitHubRelease{Assets: []GitHubAsset{
		{Name: "WAtchful-Linux-x64.tar.gz", BrowserDownloadURL: "https://example.test/x64.tar.gz"},
		{Name: "WAtchful-Linux-arm64.tar.gz", BrowserDownloadURL: "https://example.test/arm64.tar.gz"},
	}}
	asset := findAssetForPlatform(release, "linux", "arm64")
	if asset == nil || asset.Name != "WAtchful-Linux-arm64.tar.gz" {
		t.Fatalf("Linux arm64 updater must select arm64 archive, got %#v", asset)
	}

	missing := &GitHubRelease{Assets: []GitHubAsset{
		{Name: "WAtchful-Linux-x64.tar.gz", BrowserDownloadURL: "https://example.test/x64.tar.gz"},
	}}
	if asset := findAssetForPlatform(missing, "linux", "arm64"); asset != nil {
		t.Fatalf("Linux arm64 updater must not download x64 fallback, got %#v", asset)
	}
	if got := updateAssetForPlatform("linux", "arm64"); !strings.HasSuffix(got, "Linux-arm64.tar.gz") {
		t.Fatalf("Linux arm64 stable asset URL = %q", got)
	}
}

func TestWindowsUpdaterSelectsWAtchful(t *testing.T) {
	release := &GitHubRelease{Assets: []GitHubAsset{
		{Name: "WAtchful-Windows-x64.zip", BrowserDownloadURL: "https://example.test/app.zip"},
		{Name: "legacy-helper.exe", BrowserDownloadURL: "https://example.test/legacy.exe"},
		{Name: "WAtchful.exe", BrowserDownloadURL: "https://example.test/WAtchful.exe"},
	}}
	asset := findAssetForOS(release, "windows")
	if asset == nil || asset.Name != "WAtchful.exe" {
		t.Fatalf("Windows self-update must select WAtchful.exe, got %#v", asset)
	}
	if got := updateDownloadExtension(asset.BrowserDownloadURL); got != ".exe" {
		t.Fatalf("download extension = %q, want .exe", got)
	}
}

func TestWindowsUpdaterRecoversWhenExecutableReplacementFails(t *testing.T) {
	batch := windowsUpdateBatch(4321, `C:\Temp\update.exe`, `C:\Apps\WAtchful.exe`)
	for _, want := range []string{
		`:copy_loop`,
		`if not errorlevel 1 goto restart`,
		`goto copy_failed`,
		`:copy_failed`,
		`WAtchful-update.log`,
		`The existing version was restarted.`,
		`:restart`,
	} {
		if !strings.Contains(batch, want) {
			t.Errorf("Windows updater recovery script is missing %q", want)
		}
	}
	if strings.Index(batch, `:copy_failed`) > strings.Index(batch, `:restart`) {
		t.Fatal("failed-copy recovery must be defined before the normal restart path")
	}
}

func TestProgressWriterFallback(t *testing.T) {
	var reported []int
	pw := &progressWriter{
		total: -1, // Unknown or chunked Content-Length
		onProgress: func(pct int) {
			reported = append(reported, pct)
		},
	}

	chunk := make([]byte, 1024*1024) // 1MB
	_, _ = pw.Write(chunk)
	if len(reported) == 0 {
		t.Errorf("expected progress reported on fallback, got empty")
	}
}

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		current string
		latest  string
		want    bool
	}{
		{"1.4.0", "1.4.0", false},
		{"1.4.0", "v1.4.0", false},
		{"1.4.0", "1.4.1", true},
		{"1.4.0", "v1.5.0", true},
		{"1.5.0", "v1.5.1", true},
		{"1.5.1", "v1.5.2", true},
		{"1.5.2", "v1.5.3", true},
		{"1.5.9", "1.5.9.1", true},
		{"2.0.0", "2.0.1", true},
		{"2.0.1", "2.0.0", false},
		{"2.0.0", "2.0.0", false},
		{"1.5.3", "1.5.3", false},
		{"1.5.2", "1.5.2", false},
		{"1.5.2", "1.5.1", false},
		{"1.5.1", "v1.5.1", false},
		{"1.5.1", "1.5.0", false},
		{"1.4.0", "2.0.0", true},
		{"1.4.0", "1.3.9", false},
		{"1.4.0", "1.4.0-beta", false},
	}

	for _, c := range cases {
		got := isNewerVersion(c.current, c.latest)
		if got != c.want {
			t.Errorf("isNewerVersion(%q, %q) = %v; want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestCheckForUpdateLive(t *testing.T) {
	// Version 1.0.0 should always detect an update on live repo
	infoOld, err := checkForUpdate("1.0.0")
	if err != nil {
		t.Skipf("skipping live network test: %v", err)
		return
	}
	if !infoOld.Available {
		t.Errorf("expected update for 1.0.0, got false")
	}
	if infoOld.LatestVersion == "" {
		t.Errorf("expected non-empty latest version")
	}
	if infoOld.DownloadURL == "" {
		t.Errorf("expected non-empty download URL")
	}
}

// The updater must only ever download release artifacts of this repository
// over HTTPS from github.com: a page script handing startUpdateNative an
// arbitrary URL must be refused before anything is fetched, let alone
// installed and restarted into.
func TestIsAllowedUpdateURL(t *testing.T) {
	allowed := []string{
		updateAssetForPlatform("windows", "amd64"),
		updateAssetForPlatform("darwin", "arm64"),
		updateAssetForPlatform("linux", "amd64"),
		updateAssetForPlatform("linux", "arm64"),
		"https://github.com/latiefahmad/WAtchful/releases/download/v2.0.7/WAtchful.exe",
		"https://github.com/latiefahmad/WAtchful/releases/latest/download/WAtchful-Windows-x64.zip",
	}
	for _, u := range allowed {
		if !isAllowedUpdateURL(u) {
			t.Errorf("isAllowedUpdateURL(%q) = false, want true", u)
		}
	}
	denied := []string{
		"",
		"not a url",
		"http://github.com/latiefahmad/WAtchful/releases/latest/download/WAtchful.exe",
		"https://objects.githubusercontent.com/evil/payload.exe",
		"https://github.com/attacker/WAtchful/releases/latest/download/WAtchful.exe",
		"https://github.com/latiefahmad/OtherApp/releases/latest/download/WAtchful.exe",
		"https://github.com/latiefahmad/WAtchful/issues/1",
		"https://github.com/latiefahmad/WAtchful",
		"https://evil-github.com/latiefahmad/WAtchful/releases/latest/download/WAtchful.exe",
		"file:///tmp/WAtchful.exe",
	}
	for _, u := range denied {
		if isAllowedUpdateURL(u) {
			t.Errorf("isAllowedUpdateURL(%q) = true, want false", u)
		}
	}
}

// Self-update downloads are capped: an oversized payload is refused and no
// partial file is left behind, while a small payload still downloads intact.
func TestDownloadFileWithProgressEnforcesCap(t *testing.T) {
	old := maxUpdateBytes
	maxUpdateBytes = 64
	defer func() { maxUpdateBytes = old }()

	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 200))
	}))
	defer big.Close()
	dest := filepath.Join(t.TempDir(), "u.bin")
	if err := downloadFileWithProgress(big.URL, dest, nil); err == nil {
		t.Fatal("oversized update was accepted")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("rejected update left a partial file behind")
	}

	small := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tiny-update"))
	}))
	defer small.Close()
	dest2 := filepath.Join(t.TempDir(), "u2.bin")
	if err := downloadFileWithProgress(small.URL, dest2, func(int) {}); err != nil {
		t.Fatalf("small update was rejected: %v", err)
	}
	data, err := os.ReadFile(dest2)
	if err != nil || string(data) != "tiny-update" {
		t.Fatalf("small update corrupted: %q, %v", data, err)
	}
}

// Archive entries must never propagate elevated mode bits to disk: setuid,
// setgid and sticky are stripped, plain executability maps to 0755/0644,
// directories are always 0755.
func TestSafeTarMode(t *testing.T) {
	cases := []struct {
		mode  int64
		isDir bool
		want  os.FileMode
	}{
		{0644, false, 0644},
		{0755, false, 0755},
		{0777, false, 0755},
		{0600, false, 0644},
		{04755, false, 0755},        // setuid executable
		{02755, false, 0755},        // setgid executable
		{0644 | 01000, false, 0644}, // sticky data file
		{0, true, 0755},
		{0777, true, 0755},
	}
	for _, c := range cases {
		if got := safeTarMode(c.mode, c.isDir); got != c.want {
			t.Errorf("safeTarMode(%#o, %v) = %#o, want %#o", c.mode, c.isDir, got, c.want)
		}
	}
}
