package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestZZSmokeDumpInitScriptHarness writes the exact runtime init script
// (getInitScript) to build/wa_init_script.js so scripts/cdp_smoke_test.mjs can
// load it into a real Chromium engine and verify the keyboard shortcut path
// end to end (real key events, not direct function calls).
//
// The leading "ZZ" keeps the test last alphabetically; it has no assertions
// beyond "the script is non-empty" and never fails a normal `go test ./...`
// run. Its only side effect is the gitignored build/ output directory.
func TestZZSmokeDumpInitScriptHarness(t *testing.T) {
	script := getInitScript("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36")
	if script == "" {
		t.Fatal("getInitScript returned an empty script")
	}
	if !strings.Contains(script, "showSettingsModal") {
		t.Fatal("init script does not define showSettingsModal; the settings modal wiring regressed")
	}
	if err := os.MkdirAll("build", 0o755); err != nil {
		t.Fatalf("cannot create build/: %v", err)
	}
	out := filepath.Join("build", "wa_init_script.js")
	if err := os.WriteFile(out, []byte(script), 0o644); err != nil {
		t.Fatalf("cannot write %s: %v", out, err)
	}
	t.Logf("init script written to %s (%d bytes)", out, len(script))
}
