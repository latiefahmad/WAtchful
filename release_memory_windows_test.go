//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

// The working-set trim must only ever touch our own WebView2 children:
// direct msedgewebview2.exe descendants of this process. Anything else
// (unrelated Edge instances, our own PID, other executables) is off limits.
func TestChildWebView2PIDsFiltersCorrectly(t *testing.T) {
	const self = 1234
	procs := []procIdentity{
		{pid: 2001, ppid: self, exe: "msedgewebview2.exe"}, // renderer
		{pid: 2002, ppid: self, exe: "MSEdgeWebView2.EXE"}, // case-insensitive
		{pid: 2003, ppid: self, exe: "msedgewebview2.exe"}, // GPU / utility
		{pid: self, ppid: 999, exe: "msedgewebview2.exe"},  // our own PID, never
		{pid: 3001, ppid: 9999, exe: "msedgewebview2.exe"}, // another app's Edge
		{pid: 3002, ppid: self, exe: "chrome.exe"},         // wrong exe
		{pid: 3003, ppid: self, exe: "msedge.exe"},         // wrong exe
		{pid: 0, ppid: self, exe: "msedgewebview2.exe"},    // idle slot
		{pid: 3004, ppid: self, exe: ""},                   // empty name
	}
	got := childWebView2PIDs(procs, self)
	want := map[uint32]bool{2001: true, 2002: true, 2003: true}
	if len(got) != len(want) {
		t.Fatalf("childWebView2PIDs returned %v; want exactly %v", got, want)
	}
	for _, pid := range got {
		if !want[pid] {
			t.Errorf("childWebView2PIDs wrongly matched pid %d", pid)
		}
	}
}

func TestChildWebView2PIDsEmpty(t *testing.T) {
	if got := childWebView2PIDs(nil, 1234); len(got) != 0 {
		t.Fatalf("expected no PIDs for empty input, got %v", got)
	}
}

// Trimming every 5-second hide would thrash on each alt-tab (trim ~800 MB,
// then fault it all back on show: a Manager CPU + disk spike per switch).
// The child trim must stay throttled to a multi-minute interval and seeded
// at startup, so rapid window switching never triggers it.
func TestChildTrimThrottleWiring(t *testing.T) {
	src, err := os.ReadFile("app_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"childTrimInterval = 10 * time.Minute",
		"lastChildTrim = time.Now()",
		"maybeTrimWebView2Children()",
		"time.Since(lastChildTrim) < childTrimInterval",
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("child trim throttle is missing %q", want)
		}
	}
}
