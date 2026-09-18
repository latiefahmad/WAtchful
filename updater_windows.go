//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func applyUpdateWindows(newExePath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	pid := os.Getpid()

	// Robust updater batch script:
	// 1. Waits for this specific PID to completely terminate and release all file locks and mutex.
	// 2. Copies newExePath over execPath with a retry loop (works across drives and handles file locks).
	// 3. Launches the updated executable cleanly.
	// 4. Cleans up temp download and deletes itself.
	// Delays use "ping -n 2 127.0.0.1" instead of "timeout /t 1": timeout.exe
	// refuses to run when stdin is not an interactive console, which turns every
	// wait into a hot spin loop.
	batContent := windowsUpdateBatch(pid, newExePath, execPath)

	batPath := filepath.Join(os.TempDir(), fmt.Sprintf("wa_update_%d.bat", pid))
	if err := os.WriteFile(batPath, []byte(batContent), 0755); err != nil {
		return err
	}

	cmd := exec.Command("cmd.exe", "/c", batPath)
	// CREATE_NO_WINDOW (not DETACHED_PROCESS): a detached cmd.exe owns no
	// console, so every console tool inside the batch (tasklist, findstr, ping)
	// would allocate its own visible window and flash during the update. With a
	// hidden console the children simply inherit it.
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}

	if err := cmd.Start(); err != nil {
		// Fallback without CREATE_BREAKAWAY_FROM_JOB if OS policy restricts the flag
		cmd = exec.Command("cmd.exe", "/c", batPath)
		cmd.SysProcAttr = &windows.SysProcAttr{
			CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
			HideWindow:    true,
		}
		if err := cmd.Start(); err != nil {
			return err
		}
	}

	os.Exit(0)
	return nil
}

func applyUpdate(downloadedFile string) error {
	return applyUpdateWindows(downloadedFile)
}

func cleanupOldWindowsBinary() {
	execPath, err := os.Executable()
	if err == nil {
		oldPath := execPath + ".old"
		if _, err := os.Stat(oldPath); err == nil {
			_ = os.Remove(oldPath)
		}
		// Rename leftover: a previous release binary kept next to the new
		// WAtchful.exe under a legacy name only confuses launches and pins.
		// Remove exact-name matches in our own folder, never ourselves.
		for _, legacy := range []string{
			"whatsapp-desktop.exe",
			"WhatsAppDesk.exe",
			"WhatsAppDesktopLight.exe",
			"WhatsApp.exe",
		} {
			p := filepath.Join(filepath.Dir(execPath), legacy)
			if !strings.EqualFold(p, execPath) {
				if _, err := os.Stat(p); err == nil {
					_ = os.Remove(p)
				}
			}
		}
	}
}
