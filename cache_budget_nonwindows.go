//go:build !windows

package main

// Other platform cache engines do not use the Windows WebView2 process probe.
func ebWebViewCacheBusy() bool {
	return false
}
