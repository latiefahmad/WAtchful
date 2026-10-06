package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPDFPreviewOwnsBlobURLLifecycle(t *testing.T) {
	script := getInitScript("test-agent")

	checks := []string{
		"origCreateObjectURL(previewBlob)",
		"URL.revokeObjectURL(ownedBlobUrl)",
		"isRecentPDFIntent()",
		"blob.type === 'application/octet-stream'",
		"findDocumentDownloadControl(el)",
		"e.stopImmediatePropagation()",
		"extractDocumentName(el)",
	}
	for _, want := range checks {
		if !strings.Contains(script, want) {
			t.Errorf("PDF preview script is missing %q", want)
		}
	}
}

func TestWebViewDoesNotAdvertiseMissingChromePDFPlugin(t *testing.T) {
	script := getInitScript("test-agent")
	for _, unsupported := range []string{
		"get: () => true,\n\t\t\t\tconfigurable: true\n\t\t\t});\n\n\t\t\tif (!navigator.mimeTypes",
		"Chrome PDF Viewer",
		"internal-pdf-viewer",
	} {
		if strings.Contains(script, unsupported) {
			t.Errorf("WKWebView must not advertise unsupported PDF capability %q", unsupported)
		}
	}
}

func TestPDFDownloadDiscoveryDoesNotDependOnLegacyViewerTestID(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"function findVisibleViewerDownloadControl()",
		"document.querySelectorAll(viewerDownloadSelector)",
		"rect.top < window.innerHeight * 0.3",
		"pendingViewerDownloadClick",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("PDF toolbar discovery is missing %q", want)
		}
	}
}

func TestDownloadInterceptorCoalescesDuplicateRequests(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"var activeDownloadKeys = Object.create(null)",
		"function downloadRequestKey(href, filename)",
		"activeDownloadKeys[requestKey] = { status: 'downloading' }",
		"markDownloadComplete(requestKey, savedPath)",
		"releaseDownloadRequest(requestKey)",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("download de-duplication is missing %q", want)
		}
	}
}

func TestOfficeDocumentPreviewSupport(t *testing.T) {
	script := getInitScript("test-agent")

	checks := []string{
		"readZipEntryText",
		"parseDocxToHtml",
		"parsePptxToHtml",
		"renderSpreadsheetPreview",
		"XLSX.read(rawXlsxB64",
		"XLSX.utils.sheet_to_html",
		"Open in Excel / Numbers",
		"Open in Word / Pages",
		"Open in PowerPoint / Keynote",
		"isDocumentFileName(foundName)",
	}
	for _, want := range checks {
		if !strings.Contains(script, want) {
			t.Errorf("Office document preview script is missing %q", want)
		}
	}
}

func TestSpreadsheetPreviewSupportsLegacyXLS(t *testing.T) {
	script := getInitScript("test-agent")

	// The spreadsheet branch must handle .xls (legacy binary Excel) in addition to
	// .xlsx and .csv, and must no longer rely on the old hand-rolled OOXML-only
	// parser, which never supported the legacy binary format at all.
	if !strings.Contains(script, "ext === 'csv' || ext === 'xlsx' || ext === 'xls'") {
		t.Errorf("spreadsheet preview branch does not unify csv/xlsx/xls handling")
	}
	if strings.Contains(script, "parseXlsxToHtml") || strings.Contains(script, "parseCsvToHtml") {
		t.Errorf("legacy hand-rolled spreadsheet parser should have been removed in favor of the bundled XLSX library")
	}

	// SheetJS is no longer inlined into the page script (that would force the page to
	// parse ~430 KB on every load, even when no spreadsheet is ever opened). It is
	// embedded in the binary and pulled in on demand through the native bridge, so the
	// script must reference the lazy loader and its consumers.
	for _, want := range []string{
		"function ensureXLSXLoaded()",
		"loadXLSXLibraryNative",
		"XLSX.read(rawXlsxB64",
		"XLSX.utils.sheet_to_html",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("spreadsheet preview script is missing %q", want)
		}
	}

	// xlsxLibJS is exactly what the native binding hands to the page, so the embedded
	// asset must be a real, complete SheetJS core build, not an empty placeholder.
	if len(xlsxLibJS) < 100000 {
		t.Fatalf("embedded SheetJS asset looks truncated (%d bytes)", len(xlsxLibJS))
	}
	for _, want := range []string{"make_xlsx_lib", "sheet_to_html"} {
		if !strings.Contains(xlsxLibJS, want) {
			t.Errorf("embedded SheetJS asset is missing %q", want)
		}
	}
}

// Every platform must expose the lazy SheetJS bridge; a missing binding would
// silently disable spreadsheet preview on that OS alone.
func TestAllPlatformsExposeXLSXBridge(t *testing.T) {
	for _, file := range []string{"app_darwin.go", "app_windows.go", "app_linux.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		content := string(source)
		if !strings.Contains(content, `Bind("loadXLSXLibraryNative"`) {
			t.Errorf("%s does not bind loadXLSXLibraryNative", file)
		}
		if !strings.Contains(content, "return xlsxLibJS") {
			t.Errorf("%s does not return the embedded SheetJS library", file)
		}
	}
}

func TestHiddenWindowRequestsNativeMemoryRelease(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{"visibilitychange", "releaseMemoryNative", "lastClickedDocName = ''"} {
		if !strings.Contains(script, want) {
			t.Errorf("memory lifecycle script is missing %q", want)
		}
	}
}

func TestBackgroundEnhancementsYieldDuringScrolling(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"waBackgroundWorkBusyUntil", "markBackgroundWorkBusy",
		"window.addEventListener('wheel', markBackgroundWorkBusy", "Date.now() < waBackgroundWorkBusyUntil",
		"pendingMediaRoots.length >= 24", "pendingSpellRoots.length < 12",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("performance guard is missing %q", want)
		}
	}
}

func TestSettingsControlsRemainWired(t *testing.T) {
	script := getInitScript("test-agent")
	ids := []string{
		"wa-theme-btn-dark", "wa-theme-btn-light", "wa-theme-btn-system",
		"wa-action-toggle-priv", "wa-action-toggle-pin", "wa-action-toggle-mute", "wa-action-toggle-auto",
		"wa-action-toggle-notif", "wa-action-toggle-dlnotif",
		"wa-btn-change-folder", "wa-btn-open-folder", "wa-btn-reset-folder",
		"wa-btn-check-updates-modal", "wa-btn-reload-modal", "wa-btn-hardref-modal", "wa-btn-onboard-modal",
		"wa-action-open-directchat",
		"wa-qr-trigger", "wa-qr-text", "wa-qr-add",
		"wa-sched-phone", "wa-sched-text", "wa-sched-when", "wa-sched-repeat", "wa-sched-add",
		"wa-sched-whentext", "wa-sched-whenpreview", "wa-sched-kill",
		"wa-btn-run-diagnostics", "wa-btn-show-shortcuts",
		"wa-zoom-out", "wa-zoom-in", "wa-zoom-reset",
	}
	for _, id := range ids {
		if strings.Count(script, `id="`+id+`"`) != 1 {
			t.Errorf("control %s must be rendered exactly once", id)
		}
		if !strings.Contains(script, "getElementById('"+id+"')") {
			t.Errorf("control %s has no event or state binding", id)
		}
	}
}

func TestSettingsAlwaysHasAnAccessibleEntryPoint(t *testing.T) {
	script := getInitScript("test-agent")
	start := strings.Index(script, "function injectHeaderToolbarBtn()")
	end := strings.Index(script, "window.showSettingsModal = function()")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("settings entry-point implementation is incomplete")
	}
	entryPoints := script[start:end]
	if strings.Contains(entryPoints, "function injectHeaderToolbarBtn() {\n\t\t\t\tif (shouldPauseBackgroundWork())") {
		t.Fatal("essential Settings button must not be deferred by scroll throttling")
	}
	for _, want := range []string{
		"wa-toolbar-settings-btn",
		"header.lastElementChild || header",
		"setTimeout(injectHeaderToolbarBtn, 600)",
		"window.addEventListener('keydown', function(e)",
		"}, true);",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("settings must remain reachable on changing WhatsApp UI; missing %q", want)
		}
	}
	for _, gone := range []string{
		"wa-settings-fallback-btn",
		"ensureSettingsFallback",
		"syncRailSettingsBtnTheme",
	} {
		if strings.Contains(script, gone) {
			t.Errorf("floating fallback gear must stay removed; found %q", gone)
		}
	}
}

// Zoom must round-trip through the native settings bridge: shortcuts, the
// Ctrl+scroll wheel and the modal stepper share one persisted level that is
// restored on load. On Windows the level is applied natively (browser-style
// reflow) instead of by CSS-scaling the body, which left an uncovered
// background gap on WhatsApp Web.
func TestPageZoomWiringPersists(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"window.getZoomNative",
		"window.setZoomNative",
		"window.applyZoomNative",
		"window.setPageZoom = function",
		"window.stepPageZoom = function",
		"window.getPageZoom = function",
		"window.syncZoomLabel",
		"wa-zoom-label",
		"applyZoom(saved, false)",
		"applyZoom(1.0, true)",
		"ZOOM_LEVELS = [0.5, 0.67, 0.75, 0.8, 0.9, 1.0, 1.1, 1.25, 1.5, 1.75, 2.0]",
		"e.deltaY < 0 ? 1 : -1",
		"{ passive: false, capture: true });",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("persisted zoom wiring is missing %q", want)
		}
	}
	// The CSS body-zoom path must stay a fallback only: applying it on
	// Windows would reintroduce the transparent/gapped layout.
	if strings.Contains(script, "if (document.body) document.body.style.zoom") &&
		!strings.Contains(script, "} else if (document.body) {") {
		t.Error("CSS body zoom must only run when the native bridge is unavailable")
	}
	// 75% must be reachable (the level the user asked for).
	if !strings.Contains(script, "0.75") {
		t.Error("zoom ladder must include 75%")
	}
}

// The Settings panel is a two-level navigation now: sticky search + category
// pills filter a grid of rounded section cards (data-cat), toggle rows are
// switch-style buttons, and syncModalTheme repaints everything through CSS
// custom properties instead of per-card inline styles.
func TestSettingsPanelLayoutAndDiscovery(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"id=\"wa-settings-search\"", "id=\"wa-settings-tabs\"",
		"wa-settings-body", "wa-set-tools",
		"setAttribute('data-cat'",
		".wa-switch", "role=\"switch\"",
		".wa-segbtn",
		"function applyTab()", "function applySearch(",
		"--w-surface", "--w-accent", "modal.style.setProperty",
		"setNoResult",
		"toolsRow.style.top = header.offsetHeight",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("settings panel layout is missing %q", want)
		}
	}
	// The old per-row inline segment styling must stay gone (the Profiles
	// card keeps its legacy marker string, so only check the theme pills).
	if strings.Contains(script, "class=\"wa-theme-btn\"") {
		t.Error("unstyled theme segment buttons are back")
	}
}

func TestProfilesUIInjectsAfterSettingsModal(t *testing.T) {
	script := getInitScript("test-agent")
	if !strings.Contains(script, "showSettingsModalWithProfiles") {
		t.Fatal("profiles UI wrapper for the settings modal is not injected")
	}
	mainDef := strings.Index(script, "window.showSettingsModal = function()")
	wrapperDef := strings.Index(script, "function showSettingsModalWithProfiles()")
	if mainDef < 0 || wrapperDef < 0 || mainDef > wrapperDef {
		t.Fatal("profiles UI must be injected after the base settings modal definition, otherwise showSettingsModal stays empty and the Settings shortcut cannot work")
	}
}

func TestDownloadDedupReliesOnContentHash(t *testing.T) {
	script := getInitScript("test-agent")
	// The page-side size-keyed dedup index is gone (redundant bookkeeping):
	// duplicate saves are refused natively by SHA-256 content hash, so the
	// page must not keep any per-size download state.
	for _, gone := range []string{
		"activeDownloadSizes",
		"sizeKeys.length",
		"markDownloadComplete(requestKey, savedPath, blob.size)",
	} {
		if strings.Contains(script, gone) {
			t.Errorf("size-based download dedup is back: %q", gone)
		}
	}
	if !strings.Contains(script, "refuses byte-identical") {
		t.Error("the native SHA-256 dedup backstop must stay documented in the download flow")
	}
	// The native saver reports whether the bytes already existed, so the
	// page can say "Already saved" instead of implying a fresh save.
	for _, want := range []string{
		"alreadyExisted",
		"Already saved: '",
		"File already saved: '",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("already-saved reporting is missing %q", want)
		}
	}
}

func TestPrivacyModeUsesSolidRedactionWithoutFuzzyTextShadow(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"background: rgba(134,150,160,.42)",
		"text-shadow: none !important",
		"border-radius: 3px",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("privacy redaction is missing %q", want)
		}
	}
	if strings.Contains(script, "text-shadow: 0 0 10px") {
		t.Fatal("privacy mode must not render fuzzy text shadows")
	}
}

// Link/URL text sits directly inside <a> with its own explicit color, so it
// never inherits the redacted span color. Both redaction and hover-restore
// must therefore cover anchors everywhere spans are covered, or URLs leak.
func TestPrivacyModeRedactsLinks(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		`[data-testid="msg-container"] a:not([data-wa-time])`,
		`[data-testid="msg-container"]:hover a`,
		`[role="row"] a:not([data-wa-time])`,
		`[role="row"]:hover a`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("privacy link redaction is missing %q", want)
		}
	}
}

func TestThemeReapplyIsBoundedAndAvoidsObserverFeedbackLoop(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"data-wa-desk-theme",
		"function scheduleThemeReapply()",
		"[0, 350, 1200, 2600]",
		"wa-desk-theme-style",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("stable theme application is missing %q", want)
		}
	}
	if strings.Contains(script, "themeObserver") {
		t.Fatal("theme class observer can enter a WhatsApp feedback loop")
	}
}

func TestSettingsHelpUsesLocalDiagnosticsAndDocumentsShortcuts(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"Help & diagnostics", "Run quick check", "View keyboard shortcuts",
		"Native bridge: ready", "Local settings storage: ready",
		"getDownloadDirNative", "checkForUpdateNative", "isMac ? 'Cmd' : 'Ctrl'",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("settings help is missing %q", want)
		}
	}
}

func TestThemeSwitchDoesNotOverrideNativeMediaQueriesOrLoseUserChoice(t *testing.T) {
	script := getInitScript("test-agent")
	start := strings.Index(script, "// Theme Manager")
	end := strings.Index(script, "// --- In-Flow Header Toolbar Button")
	if start < 0 || end <= start {
		t.Fatal("theme manager block not found")
	}
	theme := script[start:end]

	if strings.Contains(theme, "window.matchMedia = function") {
		t.Fatal("theme manager must not replace the browser's native MediaQueryList implementation")
	}
	for _, want := range []string{
		"var themeChoiceVersion = 0",
		"if (requestVersion !== themeChoiceVersion) return",
		"window.location.reload()",
	} {
		if !strings.Contains(theme, want) {
			t.Errorf("reliable cross-platform theme switching is missing %q", want)
		}
	}
	if strings.Count(theme, "getAppThemeNative()") != 1 {
		t.Fatal("saved theme must be requested once so stale async responses cannot overwrite a user click")
	}
}

func TestAllPlatformsApplySavedThemeToNativeWindow(t *testing.T) {
	cases := []struct {
		file   string
		marker string
	}{
		{"app_darwin.go", "setNativeWindowTheme"},
		{"app_windows.go", "applyNativeThemeWin"},
		{"app_linux.go", "applyNativeThemeLinux"},
	}
	for _, tc := range cases {
		source, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		content := string(source)
		if strings.Count(content, tc.marker) < 2 {
			t.Errorf("%s must apply saved theme both at startup and after a settings change", tc.file)
		}
	}
}

func TestOnboardingIsQuietAndReplayable(t *testing.T) {
	script := getOnboardingScript()
	for _, unwanted := range []string{"radial-gradient", "backdrop-filter", "wa-feat-card", "Hemat RAM ~90%"} {
		if strings.Contains(script, unwanted) {
			t.Errorf("onboarding still contains noisy pattern %q", unwanted)
		}
	}
	for _, want := range []string{"window.showOnboardingModal", "prefers-reduced-motion", "Skip guide"} {
		if !strings.Contains(script, want) {
			t.Errorf("onboarding is missing %q", want)
		}
	}
}

func TestInjectedJavaScriptParses(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not available")
	}
	cmd := exec.Command("node", "--check", "-")
	cmd.Stdin = strings.NewReader(getInitScript("test-agent"))
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "permission denied") {
			t.Skipf("skipping node execution in sandboxed environment: %v", err)
		}
		t.Fatalf("injected JavaScript does not parse: %v\n%s", err, output)
	}
}

func TestDarwinMenuBridgeUsesStableAppWindow(t *testing.T) {
	source, err := os.ReadFile("app_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	script := string(source)
	if !strings.Contains(script, "static NSWindow* appWindow(void)") {
		t.Fatal("native menu must resolve the app's stored window")
	}

	start := strings.Index(script, "@implementation MenuBridge")
	if start < 0 {
		t.Fatal("MenuBridge implementation not found")
	}
	endOffset := strings.Index(script[start:], "@end")
	if endOffset < 0 {
		t.Fatal("MenuBridge implementation has no end")
	}
	bridge := script[start : start+endOffset]
	if strings.Contains(bridge, "[NSApp keyWindow] ?: [NSApp mainWindow]") {
		t.Fatal("menu actions must not depend on the transient key/main window")
	}
	for _, action := range []string{
		"menuSettings:", "menuCheckUpdates:", "menuOpenDownloads:",
		"menuTogglePrivacy:", "menuToggleAlwaysOnTop:", "menuToggleMuteAudio:",
		"menuReloadChat:", "menuHardRefresh:", "menuShowApp:",
		"menuSetThemeDark:", "menuSetThemeLight:", "menuSetThemeSystem:",
	} {
		if !strings.Contains(bridge, action) {
			t.Errorf("native menu action %s is missing", action)
		}
	}
}

func TestDarwinPDFUsesNativePDFKitPreview(t *testing.T) {
	source, err := os.ReadFile("app_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"-framework PDFKit", "#import <PDFKit/PDFKit.h>",
		"showNativePDFPreview", "showPDFPreviewNative",
	} {
		if !strings.Contains(string(source), want) {
			t.Errorf("native PDF preview is missing %q", want)
		}
	}

	script := getInitScript("test-agent")
	previewStart := strings.Index(script, "function showInAppDocModal")
	overlayStart := strings.Index(script[previewStart:], "var overlay = document.createElement('div')")
	nativeStart := strings.Index(script[previewStart:], "window.showPDFPreviewNative(savedPath)")
	if previewStart < 0 || overlayStart < 0 || nativeStart < 0 || nativeStart > overlayStart {
		t.Fatal("macOS PDFKit preview must run before the unsupported WKWebView overlay is created")
	}
	if !strings.Contains(script, "e.key === 'Escape' && e.isTrusted") {
		t.Fatal("synthetic viewer-dismiss Escape events must not close the document preview")
	}
}

func TestClosingNativePDFReturnsToChat(t *testing.T) {
	darwinSource, err := os.ReadFile("app_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"PDFPreviewWindowDelegate",
		"windowWillClose:",
		"closeDocumentViewerAfterNativePreview",
		"setDelegate:g_pdfPreviewDelegate",
	} {
		if !strings.Contains(string(darwinSource), want) {
			t.Errorf("native PDF close bridge is missing %q", want)
		}
	}
	if !strings.Contains(getInitScript("test-agent"), "window.closeDocumentViewerAfterNativePreview = function()") {
		t.Fatal("webview has no command that returns from the document viewer to chat")
	}
}

func TestClosingNativePDFReleasesRenderedDocument(t *testing.T) {
	source, err := os.ReadFile("app_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(source)
	start := strings.Index(content, "- (void)windowWillClose:")
	if start < 0 {
		t.Fatal("PDF preview close handler not found")
	}
	end := strings.Index(content[start:], "\n}")
	if end < 0 {
		t.Fatal("PDF preview close handler is incomplete")
	}
	handler := content[start : start+end]
	for _, want := range []string{
		"[pdfView setDocument:nil]",
		"[window setContentView:nil]",
		"purgeWebKitMemory()",
	} {
		if !strings.Contains(handler, want) {
			t.Errorf("PDF close handler must release rendered memory via %q", want)
		}
	}
}

func TestNativePDFCanReopenAfterContentCleanup(t *testing.T) {
	source, err := os.ReadFile("app_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(source)
	for _, want := range []string{
		"NSRect pdfFrame = [[g_pdfPreviewWindow contentView] bounds]",
		"NSIsEmptyRect(pdfFrame)",
		"initWithFrame:pdfFrame",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("reusable native PDF viewer is missing %q", want)
		}
	}
}

func TestBackgroundDOMWorkPausesWhenHidden(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"function shouldPauseBackgroundWork()",
		"if (shouldPauseBackgroundWork()) return;",
		"mediaObserver",
		"viewerObserver",
		"injectHeaderToolbarBtn",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("background work pause guard is missing %q", want)
		}
	}
}

func TestWindowsMemoryReleaseDoesNotOrphanSuspendedWebView(t *testing.T) {
	source, err := os.ReadFile("app_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(source)
	start := strings.Index(content, `_ = w.Bind("releaseMemoryNative"`)
	if start < 0 {
		t.Fatal("Windows memory release binding not found")
	}
	end := strings.Index(content[start:], "\n\t})")
	if end < 0 {
		t.Fatal("Windows memory release binding is incomplete")
	}
	binding := content[start : start+end]
	for _, line := range strings.Split(binding, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "//") && strings.Contains(line, "w.Suspend()") {
			t.Fatal("visibility-triggered memory release must not suspend WebView2 without a matching resume")
		}
	}
}

func TestMacRoutineMemoryPurgeKeepsDiskCache(t *testing.T) {
	source, err := os.ReadFile("app_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(source)
	start := strings.Index(content, "static void purgeWebKitMemory(void)")
	if start < 0 {
		t.Fatal("macOS WebKit memory purge routine not found")
	}
	end := strings.Index(content[start:], "\n}")
	if end < 0 {
		t.Fatal("macOS WebKit memory purge routine is incomplete")
	}
	purge := content[start : start+end]
	if strings.Contains(purge, "WKWebsiteDataTypeDiskCache") {
		t.Fatal("routine RAM cleanup must retain the disk cache")
	}
}

func TestMediaObserverScansOnlyAddedSubtrees(t *testing.T) {
	script := getInitScript("test-agent")
	start := strings.Index(script, "function scanForUnpreparedMedia()")
	if start < 0 {
		t.Fatal("media scan routine not found")
	}
	end := strings.Index(script[start:], "var mediaObserver")
	if end < 0 {
		t.Fatal("media scan routine is incomplete")
	}
	scan := script[start : start+end]
	if strings.Contains(scan, "document.body") || strings.Contains(scan, "document.documentElement") {
		t.Fatal("each mutation-frame must not rescan the whole document for media")
	}
	if !strings.Contains(script, "pendingMediaRoots") {
		t.Fatal("media observer must queue only newly-added subtrees")
	}
}

func TestLinuxDoesNotForceContinuousCompositingOrPeriodicGC(t *testing.T) {
	source, err := os.ReadFile("app_linux.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(source)
	if strings.Contains(content, `Setenv("WEBKIT_FORCE_COMPOSITING_MODE", "1")`) {
		t.Fatal("Linux must let WebKitGTK choose compositing mode")
	}
	if strings.Contains(content, "time.NewTicker(60 * time.Second)") {
		t.Fatal("Linux must not force full Go GC every minute")
	}
	if !strings.Contains(content, `_ = w.Bind("releaseMemoryNative"`) {
		t.Fatal("Linux must release Go memory when the shared visibility lifecycle requests it")
	}
}

func TestDesktopWindowStateUsesResizeEventsNotPolling(t *testing.T) {
	for _, file := range []string{"app_darwin.go", "app_windows.go", "app_linux.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		content := string(source)
		if !strings.Contains(content, `_ = w.Bind("saveWindowStateNative"`) {
			t.Errorf("%s must bind resize-driven window persistence", file)
		}
		if strings.Contains(content, "time.NewTicker(10 * time.Second)") {
			t.Errorf("%s must not poll window state every 10 seconds", file)
		}
	}
}

func TestMediaDoesNotEagerlyBufferEveryAttachment(t *testing.T) {
	script := getInitScript("test-agent")
	if strings.Contains(script, "setAttribute('preload', 'auto')") {
		t.Fatal("chat media must not preload full files before playback")
	}
	if !strings.Contains(script, "setAttribute('preload', 'metadata')") {
		t.Fatal("chat media should load metadata without buffering full files")
	}
}

func TestWindowsProcessProtectionPermitsChildBreakaway(t *testing.T) {
	source, err := os.ReadFile("app_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(source)

	// Must permit child breakaway so Chromium sandbox job objects don't fail
	if !strings.Contains(content, "JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK") {
		t.Error("Windows Job Object must include JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK")
	}

	// Must not kill own child processes at startup
	if strings.Contains(content, "Stop-Process") {
		t.Error("Windows startup must not invoke PowerShell Stop-Process which kills active webview instances")
	}

	// Must not pass dangerous flags that trigger black screen on Windows 10/11
	for _, dangerous := range []string{"--disable-gpu-shader-disk-cache", "CalculateNativeWinOcclusion", `--js-flags="`} {
		if strings.Contains(content, dangerous) {
			t.Errorf("Windows browser args contain dangerous flag %q known to cause black screen", dangerous)
		}
	}
}

func TestWindowsCacheProbeNeverShowsTerminal(t *testing.T) {
	source, err := os.ReadFile("cache_budget_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(source)
	for _, want := range []string{
		`powershell.exe`,
		`"-NonInteractive"`,
		`"-WindowStyle", "Hidden"`,
		`syscall.SysProcAttr{HideWindow: true}`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("Windows cache probe must prevent console flash; missing %q", want)
		}
	}
}

func TestMediaViewerCloseButtonNotIntercepted(t *testing.T) {
	script := getInitScript("test-agent")

	checks := []string{
		`el.closest('[data-testid="media-viewer"]')`,
		`el.closest('#wa-doc-modal-overlay')`,
		`target.closest('[data-testid="media-viewer"]')`,
		`button[data-testid="x-viewer"]`,
		`[data-icon="x-viewer"]`,
		`Escape`,
		`window.dismissStuckViewer()`,
	}

	for _, want := range checks {
		if !strings.Contains(script, want) {
			t.Errorf("script is missing media viewer safeguard %q", want)
		}
	}
}

// The Ctrl+, shortcut must stay invocable on any keyboard layout: match the
// Comma key by glyph, physical code and legacy keyCode, and never throw when
// the modal bridge is momentarily unavailable.
func TestShortcutCtrlCommaOpensSettings(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"e.code === 'Comma'",
		"keyCode === 188",
		"typeof window.showSettingsModal === 'function'",
		"window.showSettingsModal();",
		"}, true);",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("Ctrl+, shortcut wiring is missing %q", want)
		}
	}
}

// Init scripts run before the parser creates <head>/<body>, so an unguarded
// observer.observe(null) throws and aborts every enhancement defined after
// it (Settings modal, Profiles card, shortcut listener). Observers must
// defer instead — this once left Ctrl+, completely dead.
func TestInitScriptSurvivesDocumentStart(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"function watchHead()",
		"function initSpellCheckObserver()",
		"document.addEventListener('DOMContentLoaded', initSpellCheckObserver",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("document-start-safe observer setup is missing %q", want)
		}
	}
	if strings.Contains(script, "observer.observe(document.body,") {
		t.Error("unguarded observer call will abort the init script at document start")
	}
}

// The Settings panel and the Profiles card must be opaque by construction
// and the card must sit inside the panel as its first section — never
// appended to the overlay flex row (rendered detached beside the panel)
// and never transparent (page bleeding through).
func TestSettingsModalAndProfilesCardPlacement(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"'background:' + (isDark ? '#111b21' : '#ffffff')",
		"document.getElementById('wa-settings-container')",
		"container.querySelector('#wa-modal-header')",
		"container.insertBefore(card, head.nextSibling)",
		"padding:12px 0;background:#111b21;border-color:#2a3942",
		"window.syncModalTheme(darkNow)",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("settings/profiles panel wiring is missing %q", want)
		}
	}
	if strings.Contains(script, "modal.appendChild(card)") {
		t.Error("profiles card must not be appended to the overlay; it detaches beside the panel")
	}
}

// Filenames and release titles in the preview modal and update banner come
// from chat content or network metadata, so they must be HTML-escaped: a
// name like '"><img src=x onerror=...>x.pdf' would otherwise execute in the
// privileged page context that can reach every native bridge.
func TestPreviewModalEscapesAttackerStrings(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"function escapeHtml(s)",
		`title="' + escapeHtml(filename) + '">' + escapeHtml(filename)`,
		`' + escapeHtml(filename) + '</h2>'`,
		`' + escapeHtml(displayPath) + '</div>'`,
		`title="' + escapeHtml(filename) + '"></iframe>'`,
		`' + escapeHtml(titleText) + '</strong>'`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("preview escaping is missing %q", want)
		}
	}
}

// Choosing Download explicitly (viewer toolbar button, context-menu item,
// download anchor) means save only: the in-app document preview is reserved
// for clicking the document itself, and the blob hook must stand down while
// an explicit download is recent so one action never saves+previews twice.
func TestExplicitDownloadSuppressesPreview(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"lastExplicitDownloadAt = Date.now()",
		"function isRecentExplicitDownload()",
		"isDocBlob && !isRecentExplicitDownload()",
		"captureDownload(href, name, false)",
		"function isExplicitDownloadMenuItem(target)",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("explicit-download wiring is missing %q", want)
		}
	}
	if strings.Count(script, "captureDownload(href, name, false)") != 2 {
		t.Error("both anchor interceptions (prototype override + click capture) must save without preview")
	}
}

// Spreadsheet cells are attacker-controlled: sheet_to_html must be
// sanitized (inert template, banned elements removed, only structural
// attributes kept) before the markup reaches the privileged page.
func TestSpreadsheetPreviewSanitizesCellMarkup(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"function sanitizeSheetHtml(html)",
		"document.createElement('template')",
		"sanitizeSheetHtml(XLSX.utils.sheet_to_html",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("spreadsheet sanitizer is missing %q", want)
		}
	}
}

// A drop ending outside the chat zone (dialog, empty file list) must still
// clear the highlight — the stuck class disables pointer events app-wide —
// via the global drop/dragend/blur net; file injection probes over several
// rounds and only fires into an input WhatsApp left empty.
func TestDragDropRecoveryAndProbing(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"function resetDragState()",
		"document.addEventListener('drop', resetDragState, true)",
		"document.addEventListener('dragend', resetDragState, true)",
		"window.addEventListener('blur', resetDragState)",
		"rounds >= 8",
		"fileInput.files.length === 0",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("drag-drop recovery is missing %q", want)
		}
	}
}

// Direct Chat: toolbar button beside the gear, modal with number input +
// Send-From profile dropdown, client-side validation, and handoff to the
// native bridge that navigates the profile's tab to /send?phone=.
func TestDirectChatModalWiring(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"wa-action-open-directchat",
		"window.openDirectChatModal = function()",
		"wa-directchat-overlay",
		"wa-directchat-number",
		"wa-directchat-profile",
		"wa-directchat-cancel",
		"wa-directchat-start",
		"window.startDirectChatNative(profileKey, raw)",
		"/^[0-9]{8,15}$/",
		"Opening chat with +",
		"e.key === 'c'",
		"Direct chat (new number)",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("direct-chat wiring is missing %q", want)
		}
	}
}

// A dismissed WhatsApp viewer re-creates the attachment blob, which used to
// re-enter the preview interceptor and reopen the modal in an infinite
// loop. The same document is auto-previewed only once per window (8s
// cooldown); a fresh click on the document clears the guard and previews
// again.
func TestDocumentPreviewLoopGuard(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"var lastDocPreviewName = ''",
		"var docPreviewCooldownMs = 8000",
		"name === lastDocPreviewName && (Date.now() - lastDocPreviewAt) < docPreviewCooldownMs",
		"lastDocPreviewName = name;",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("preview loop guard is missing %q", want)
		}
	}
	// The guard must be reset by fresh intent: the document click handler
	// AND the hidden->visible window transition both clear it.
	if got := strings.Count(script, "lastDocPreviewName = '';"); got < 2 {
		t.Errorf("guard reset must exist in click + visibility handlers, found %d", got)
	}
}

// dismissStuckViewer must stay bounded and stop early: the old version
// hammered WhatsApp's viewer for 2.4s (30 rounds of close clicks + synthetic
// Escapes), which fought the page and fed the preview loop.
func TestDismissStuckViewerIsBounded(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"var maxAttempts = 8;",
		"if (attempts > 3) clearInterval(dismissTimer);",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("bounded dismissal is missing %q", want)
		}
	}
	if strings.Contains(script, "if (attempts > 30)") {
		t.Error("the old 30-round hammer is back")
	}
}

// The document preview overlay must scroll smoothly: a full-viewport
// backdrop blur forces the compositor to re-render the page behind the
// modal on every scrolled frame, and uncontained scroll surfaces repaint
// the whole overlay. Flat dim + containment + a layer-promoted card.
func TestDocumentPreviewScrollsSmoothly(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"transform:translateZ(0);",
		"contain:strict;",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("preview scroll performance is missing %q", want)
		}
	}
	if strings.Contains(script, "wa-doc-modal-overlay';backdrop-filter") ||
		strings.Contains(script, "0.85);backdrop-filter") {
		t.Error("the compositor-heavy preview backdrop blur is back")
	}
}

// The Archived navigation control ("Archived"/"Diarsipkan"/...) is UI
// guidance, not private chat data: it must stay readable in privacy mode
// while archived rows themselves stay redacted. Tagged by text match with a
// length guard (a real chat row holds far more text), exempted by CSS that
// wins over the row rules.
func TestPrivacyModeKeepsArchivedNavigationReadable(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"data-wa-archived-nav",
		"function tagArchivedNavIn(root)",
		"WA_ARCHIVED_RE",
		"Diarsipkan",
		"t.length > 24",
		"tagArchivedNavIn(document.getElementById('pane-side'))",
		"tagArchiveInfoIn(document.getElementById('pane-side'))",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("archived navigation handling is missing %q", want)
		}
	}
}

// The update banner is a fixed overlay: without yielding layout space it
// covers WhatsApp's own top header (Archived row, back button). The banner
// must push #app down by its live height (progress row grows it) and clean
// up the observer + CSS hooks on dismiss.
func TestUpdateBannerReservesLayoutSpace(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"html.wa-update-visible #app",
		"--wa-update-banner-height",
		"bannerResizeObserver = new ResizeObserver(syncBannerLayout)",
		"bannerResizeObserver.observe(banner)",
		"bannerResizeObserver.disconnect()",
		"classList.remove('wa-update-visible')",
		"removeProperty('--wa-update-banner-height')",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("update banner layout is missing %q", want)
		}
	}
}

// Quick Replies: /trigger + Space expands a saved template in the chat
// composer (Enter only for a known trigger, otherwise the message sends as
// usual); CRUD lives in Settings with user strings on textContent only.
func TestQuickRepliesWiring(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"wa_desk_quick_replies",
		"window.getQuickReplies",
		"window.addQuickReply",
		"window.deleteQuickReply",
		"expandQrVariables",
		"{date}",
		"{time}",
		"{name}",
		"wa-qr-list",
		"wa-qr-add",
		"document.execCommand('insertText'",
		"wa-settings-overlay, #wa-directchat-overlay, #wa-doc-modal-overlay",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("quick-reply wiring is missing %q", want)
		}
	}
}

// Scheduler: per-profile timed messages to phone numbers. Store +
// recurrence in-page; fire navigates to /send?phone= with a persisted
// pending payload that the boot handler completes (composer -> send ->
// verify); armed tabs report busy('scheduled') so the shell keeps them
// alive instead of suspending/hibernating under a waiting send.
func TestSchedulerWiring(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"wa_desk_schedules",
		"wa_desk_pending_send",
		"window.getSchedules",
		"window.addSchedule",
		"window.deleteSchedule",
		"window.toggleSchedule",
		"window.getScheduleLog",
		"s.repeat === 'monthly'",
		"s.repeat === 'yearly'",
		"setMonth(dm.getMonth() + 1)",
		"setFullYear(dy.getFullYear() + 1)",
		"window.__waAdvanceSchedule",
		"repeatDay",
		"item.dom",
		"item.yday",
		"wa-sched-repday",
		"seedRepDay",
		"__waReportBusy('scheduled'",
		"data-icon=\"send\"",
		"wa-sched-list",
		"wa-sched-add",
		"wa-sched-log",
		"send?phone=",
		"window.parseSchedWhen",
		"wa-sched-whentext",
		"wa-sched-whenpreview",
		"refreshSchedWhenPreview",
		"wa_desk_last_sent",
		"wa_desk_send_times",
		"noteBreakerTripped",
		"CLAIM_TTL",
		"pending.claimedAt",
		"failed (interrupted)",
		"wa_desk_sched_boots",
		"recordBootWatched",
		"schedFrozen()",
		"reboot loop",
		"lastFiredAt",
		"cancelled (schedule removed)",
		"seenPayloads",
		"payloadKey",
		"wa_desk_sched_killed",
		"setSchedulerKilled",
		"schedKilled()",
		"wa-sched-kill",
		"bootId",
		"attemptId",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("scheduler wiring is missing %q", want)
		}
	}
}

// AFK auto-reply: while armed, one automatic reply per chat per cooldown
// on new unread; the mode ends the moment the user sends a message
// themselves. Safety mirrors the scheduler: kill switch, cooldown claimed
// before any click, burst breaker, in-flight boot recovery, identity check
// against the opened chat header, and it yields the composer while a
// scheduler payload is queued. Armed tabs report busy('scheduled').
func TestAFKWiring(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"wa_desk_afk_cfg",
		"wa_desk_afk_last",
		"wa_desk_afk_send_times",
		"wa_desk_afk_log",
		"wa_desk_afk_killed",
		"wa_desk_afk_inflight",
		"window.getAfkConfig",
		"window.setAfkConfig",
		"window.__waAfkInWindow",
		"window.__waAfkStatus",
		"window.__waAfkProbe",
		"wa-afk-probe",
		"wa-afk-probeout",
		"Diagnose scan",
		"unreadEls",
		"rowDetail",
		"cell-frame-title",
		"icon-unread-count",
		"gridcell",
		"__waAfkNoComposerReason",
		"afkNoComposerReason",
		"skipped (channel, no reply box)",
		"mainEditables",
		"activateAfkRow",
		"__waActivateAfkRow",
		"triedFallback",
		"cloneNode(true)",
		"appShell",
		"appDeep",
		"titleHTML",
		"__waAfkRowName",
		"afkRowName",
		"span[title]",
		"mainHeaderAll",
		"mainHeaderText",
		"mainHeaderHTML",
		"header.cloneNode(true)",
		"__waAfkTrustedClick",
		"afkTrustedClick",
		"dispatchMouseEvent",
		"stillKey",
		"lastUserActivityAt",
		"e.isTrusted",
		"settings panel is open",
		"resumes when idle",
		"__waAfkAllowed",
		"afkAllowed",
		"allowOnly",
		"wa-afk-allowonly",
		"wa-afk-allow",
		"skipped (not in list)",
		"not in the reply list",
		"wa-afk-status",
		"afkStatusTimer",
		"cooldownSkips",
		"no unread chats right now",
		"afkInWindow",
		"useWindow",
		"startMin",
		"endMin",
		"window.__waAfkMinsToHM",
		"window.__waAfkHMToMins",
		"window.setAFKKilled",
		"window.getAfkLog",
		"window.__afkTick",
		"wa-afk-enable",
		"wa-afk-text",
		"wa-afk-cooldown",
		"wa-afk-groups",
		"wa-afk-window",
		"wa-afk-start",
		"wa-afk-end",
		"wa-afk-kill",
		"wa-afk-log",
		"data-testid=\"unread-pill\"",
		"findUnreadCandidates",
		"startAfkReply",
		"endAfkByUser",
		"chat mismatch: saw",
		"__waHeaderContainsKey",
		"headerContainsKey",
		"displayName",
		"failed (unverified)",
		"skipped (group)",
		"off (you sent a message)",
		"paused: burst limit",
		"failed (interrupted)",
		"SCHED_PENDING_KEY",
		"recountAfkBusy",
		"namesMatch",
		"looksLikeGroup",
		"bootId",
		"attemptId",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("AFK wiring is missing %q", want)
		}
	}
}

// Every intended composer send — scheduler and AFK — must pass the shared
// choke-point window.__waGuardSend: owner kill first, then a persisted
// minimum gap, then a per-page budget; a repeat pace violation or an
// exhausted budget trips the emergency (both kill switches + loud log).
// Kill withdraws queued payloads immediately, switching a schedule off
// cancels its pending send, and both engines expose their in-flight flags
// so they can yield to (and never mistake) each other's sends.
func TestSendGuardWiring(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"window.__waGuardSend",
		"wa_send_budget",
		"wa_last_send_click",
		"wa_send_violations",
		"emergency stop: send ",
		"tripped [",
		"cancelled (emergency stop)",
		"cancelled (switched off)",
		"cancelled (pending withdrawn)",
		"blocked (send ",
		"window.__afkInFlight",
		"window.__schedInFlight",
		"wa_desk_afk_killed",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("send guard wiring is missing %q", want)
		}
	}
	// The polling-stage latch must exist in BOTH engines — a check
	// without the set was how an early build could re-run its routine.
	if n := strings.Count(script, "pollDone = true;"); n < 2 {
		t.Errorf("poll stage latch set count = %d, want >= 2 (scheduler + AFK)", n)
	}
}

// The composer pipeline that turns "text visible in the draft" into an
// actually delivered message: insert via execCommand → beforeinput →
// direct replace WITH an input event (state sync — otherwise MIC never
// becomes SEND), then a send stage that waits out the async MIC→SEND
// re-render before clicking, with a keyCode-13 Enter as last resort.
// Both engines must use the shared helpers.
func TestComposerSendPipeline(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"window.__waInsertText",
		"window.__waSendWithWait",
		"window.__waClickSend",
		"window.__waEnterSend",
		"window.__waComposerText",
		"window.__waFooterSnapshot",
		"execCommand('insertText'",
		"#main footer [data-icon=\"send\"]",
		"aria-label=\"Send\"",
		"data-testid=\"send\"",
		"Kirim",
		"keyCode",
		"return 13",
		"new Event('input'",
		"ClipboardEvent('paste'",
		"insertFromPaste",
		"clipboardData",
		"dom-plain",
		"wa-sched-logcopy",
		"Copy log",
		"window.__waInsertAsync",
		"window.__waTrustedInsert",
		"window.__waCdpResult",
		"window.__waCdpCb",
		"'Input.insertText'",
		"waCdpNative",
		"diag: insert path=",
		"send clicked after ",
		"enter fired at ",
		"no send trigger by ",
		"send-wait start; footer=",
		"checks >= 16",
		"checks >= 32",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("composer send pipeline is missing %q", want)
		}
	}
	if n := strings.Count(script, "__waSendWithWait(composer"); n < 2 {
		t.Errorf("send-wait call sites = %d, want >= 2 (scheduler + AFK)", n)
	}
	if n := strings.Count(script, "__waInsertText(composer,"); n < 2 {
		t.Errorf("insert call sites = %d, want >= 2 (scheduler + AFK)", n)
	}
	if n := strings.Count(script, "__waInsertAsync(composer"); n < 2 {
		t.Errorf("async insert call sites = %d, want >= 2 (scheduler + AFK)", n)
	}
}

// TestCDPBridgeWiring pins the trusted-input bridge end to end: the page
// asks the host to run CDP Input.insertText, the Windows shell binds
// waCdpNative, and the vendored WebView2 edge layer performs the
// CallDevToolsProtocolMethod call. WhatsApp reverts every synthetic edit
// (field: insert path=none len=0), so this channel is what actually
// delivers scheduled messages.
func TestCDPBridgeWiring(t *testing.T) {
	script := getInitScript("test-agent")
	for _, want := range []string{
		"window.__waInsertAsync",
		"window.__waTrustedInsert",
		"window.__waCdp",
		"window.__waCdpResult",
		"'Input.insertText'",
		"typeof window.waCdpNative === 'function'",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("page CDP bridge is missing %q", want)
		}
	}
	checkFile := func(path string, wants ...string) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		s := string(b)
		for _, want := range wants {
			if !strings.Contains(s, want) {
				t.Errorf("%s is missing %q", path, want)
			}
		}
	}
	checkFile("app_windows.go", `Bind("waCdpNative"`, "callCDP(", "*chromiumView", "__waCdpResult(")
	checkFile("tabs_windows.go", "func (v *chromiumView) callCDP(", "CallCDP(method, params, cb)")
	checkFile("vendor/github.com/jchv/go-webview2/pkg/edge/cdp.go",
		"CallDevToolsProtocolMethod", "Input.insertText", "cdpHandlerInvoke")
}
