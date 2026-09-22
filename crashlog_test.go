package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Past the size cap the log rotates to a single backup, so a crash loop can
// never grow it without bound. Threshold is a var so the test does not need
// a real megabyte.
func TestCrashLogRotatesPastCap(t *testing.T) {
	old := crashLogMaxBytes
	crashLogMaxBytes = 200
	defer func() { crashLogMaxBytes = old }()

	dir := t.TempDir()
	path := filepath.Join(dir, "wa_crash.log")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 300)), 0644); err != nil {
		t.Fatal(err)
	}
	rotateCrashLogIfNeeded(path)
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected backup at %s.1: %v", path, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("original must be gone after rotation (fresh append recreates it)")
	}

	// Below the cap nothing moves.
	if err := os.WriteFile(path, []byte("small"), 0644); err != nil {
		t.Fatal(err)
	}
	rotateCrashLogIfNeeded(path)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("small log must be left alone: %v", err)
	}
}
