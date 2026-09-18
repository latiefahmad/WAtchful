//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// ebWebViewCacheBusy reports whether the WebView2 browser process holds any
// file inside the cache home open. The process probe must never flash a
// console window: it can run during startup, minimize, or close.
func ebWebViewCacheBusy() bool {
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command",
		`Get-Process msedgewebview2 -ErrorAction SilentlyContinue | Measure-Object | Select-Object -ExpandProperty Count`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		// Cannot tell: assume busy rather than risk a corrupting delete.
		return true
	}
	n, parseErr := strconv.Atoi(strings.TrimSpace(string(out)))
	return parseErr == nil && n > 0
}
