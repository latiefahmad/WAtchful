//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
	"unsafe"
)

// The Go mirror of NOTIFYICONDATAW must match the C layout byte for byte:
// C sizeof(NOTIFYICONDATAW) is 840 on amd64 (the uVersion/szInfoTitle/
// dwInfoFlags union is a single region). A drift here makes
// Shell_NotifyIconW silently misparse the struct.
func TestNotifyIconDataSizeMatchesCLayout(t *testing.T) {
	const want = 840
	if got := unsafe.Sizeof(notifyIconDataW{}); got != want {
		t.Fatalf("sizeof(notifyIconDataW) = %d, want %d (C NOTIFYICONDATAW on amd64)", got, want)
	}
	if got := unsafe.Offsetof(notifyIconDataW{}.Union); got != 560 {
		t.Fatalf("union offset = %d, want 560", got)
	}
}

func TestSplitProfileMenuRows(t *testing.T) {
	rows := splitProfileMenuRows("Default\nTeam Work\n\nPersonal")
	if len(rows) != 3 || rows[0] != "Default" || rows[1] != "Team Work" || rows[2] != "Personal" {
		t.Fatalf("rows = %#v, want 3 clean entries", rows)
	}
	if got := splitProfileMenuRows(""); len(got) != 0 {
		t.Fatalf("empty joined string must yield no rows, got %#v", got)
	}
}

func TestSplitProfileMenuRowsStableOrder(t *testing.T) {
	rows := splitProfileMenuRows("A\nB\nC")
	// Tray clicks map by row index, so the order must be the joined order.
	if rows[0] != "A" || rows[2] != "C" {
		t.Fatalf("row order changed: %#v", rows)
	}
}

// Direct Chat sits in the tray menu beside Settings so it stays reachable
// without hunting for a header icon.
func TestTrayMenuOffersDirectChat(t *testing.T) {
	src, err := os.ReadFile("tray_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"Direct Chat..."`,
		"cmdDirectChat",
		"openDirectChatFromTray",
		"window.openDirectChatModal",
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("tray direct-chat entry is missing %q", want)
		}
	}
}
