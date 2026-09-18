//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Suspend only freezes a page (measured: no RAM returned); hibernation is what
// gives memory back. The thresholds must stay ordered and wired.
func TestSuspendAndHibernateThresholds(t *testing.T) {
	if tabHibernateGrace <= tabSuspendGrace {
		t.Fatalf("hibernation (%v) must come after suspend (%v)", tabHibernateGrace, tabSuspendGrace)
	}
	if shouldSuspend(tabSuspendGrace - time.Second) {
		t.Error("a tab must not suspend before the grace period")
	}
	if !shouldSuspend(tabSuspendGrace) {
		t.Error("a tab must suspend once it passes the grace period")
	}
	if shouldHibernate(tabSuspendGrace) {
		t.Error("a freshly hidden tab must be suspended first, not hibernated")
	}
	if !shouldHibernate(tabHibernateGrace) {
		t.Error("a long-idle tab must hibernate to return its memory")
	}
}

// The hibernate/wake path: engine closed on idle, rebuilt on click, other
// views never dereferenced while hibernated.
func TestHibernationWiring(t *testing.T) {
	shell, err := os.ReadFile("tabs_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(shell)
	for _, want := range []string{
		"tabHibernateGrace = 2 * time.Minute",
		"func (m *tabShell) hibernateLocked",
		"func (m *tabShell) buildView",
		"func shouldSuspend(",
		"func shouldHibernate(",
		"if cur.hibernated {", // wake path in activateLocked
		"if err := m.buildView(cur); err != nil {",
		"entry.hibernated = true", // lazy startup for non-active profiles
		"hibernated bool",
		"hibernated []bool",  // strip marks them
		"if t.view == nil {", // nil guards on hibernated entries
	} {
		if !strings.Contains(src, want) {
			t.Errorf("hibernation wiring is missing %q", want)
		}
	}
	// Only the active profile may be loaded at startup now.
	if strings.Contains(src, "for _, p := range profiles {\n\t\tentry, err := m.createTabView(p)") {
		t.Error("startup must not load an engine for every registered profile")
	}
}

// Keep the strip painter's hibernated marker pinned so a repaint regression
// is caught too (the tab must look "asleep" before it reloads).
func TestHibernatedTabsAreMarkedInStrip(t *testing.T) {
	shell, err := os.ReadFile("tabs_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(shell)
	for _, want := range []string{
		"s.hibernated = append(s.hibernated, t.hibernated)",
		"hib := snap.hibernated[i]",
		"tabCreatePen.Call(0, 0, muted)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("strip must show hibernated tabs as asleep; missing %q", want)
		}
	}
}

// The tabbed shell must stay wired: page shortcuts -> native bridges ->
// manager ops, with one shared outer window and suspended hidden tabs.
func TestTabbedShellBridgesWired(t *testing.T) {
	shell, err := os.ReadFile("tabs_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("app_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	script := getInitScript("test-agent")

	for _, want := range []string{
		// Page side: shortcuts reach guarded bridges.
		"cycleTabNative", "activateTabNative",
		"window.addEventListener('keydown', function(e)",
		"+Tab / ", "+1 &hellip",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("tab shortcut wiring is missing %q", want)
		}
	}
	for _, want := range []string{
		`func tabManagerActivateIndex`, `func tabManagerCycle`,
		// One outer window; hidden views hide, idle views suspend.
		`SetBoundsRect(`, `func (m *tabShell) sweepHidden`,
		`suspendedByUs`, `tabWMTabGoTo`,
	} {
		if !strings.Contains(string(shell), want) {
			t.Errorf("tab shell is missing %q", want)
		}
	}
	for _, want := range []string{
		// Go side: every tab bridge is bound for each view.
		`Bind("cycleTabNative"`, `Bind("activateTabNative"`,
		`tabManagerActivateByName`, `tabManagerOpenProfile`,
		`tabManagerDeleteProfile`, `tabManagerRenameProfile`, `tabManagerResetProfile`,
		`tabManagerReportBadge`, `tabManagerSetTheme`,
	} {
		if !strings.Contains(string(app), want) {
			t.Errorf("profile bridges are not routed to the tab shell; missing %q", want)
		}
	}
	// Relaunch survives only as the no-shell fallback (e.g. picker flows).
	if strings.Count(string(app), "relaunchWithProfile(p.Name)") < 2 {
		t.Error("relaunch fallbacks must stay for the no-shell path")
	}
}

// The owner-drawn strip must scale with the monitor DPI (the page is scaled
// by WebView2, the strip is ours). Scaling is derived from one place so
// painting, hit-testing and layout cannot drift apart.
func TestTabMetricsScaleWithDPI(t *testing.T) {
	m := &tabShell{dpi: 96}
	base := m.metrics()
	if base.stripH != tabStripHeight || base.pad != 8 || base.gap != 6 {
		t.Fatalf("96 DPI must equal the design pixels, got %+v", base)
	}

	m.dpi = 144 // 150%
	if got := m.metrics().stripH; got != 60 {
		t.Errorf("150%% strip height = %d, want 60", got)
	}
	if got := m.metrics().dot; got != 33 {
		t.Errorf("150%% avatar dot = %d, want 33", got)
	}

	m.dpi = 192 // 200%
	if got := m.metrics().stripH; got != tabStripHeight*2 {
		t.Errorf("200%% strip height = %d, want %d", got, tabStripHeight*2)
	}
	if got := m.metrics().dot; got != 44 {
		t.Errorf("200%% avatar dot = %d, want 44", got)
	}

	// A missing/unreadable DPI must fall back to 100%, never zero.
	m.dpi = 0
	if got := m.metrics().stripH; got != tabStripHeight {
		t.Errorf("unknown DPI must fall back to 100%%, got %d", got)
	}
}

// Hit-testing must use the same scaled geometry as the painter, at any DPI.
// With 2 tabs and a wide strip the tabs are capped (220 px at 100%), so the
// area past the last tab must report "no tab" instead of a wrong index.
func TestTabHitTestFollowsScaledGeometry(t *testing.T) {
	m := &tabShell{dpi: 96}
	if got := m.indexAt(10, 1084, 2); got != 0 {
		t.Errorf("left edge = tab %d, want 0", got)
	}
	if got := m.indexAt(200, 1084, 2); got != 0 {
		t.Errorf("first tab body = tab %d, want 0", got)
	}
	if got := m.indexAt(240, 1084, 2); got != 1 {
		t.Errorf("second tab body = tab %d, want 1", got)
	}
	if got := m.indexAt(800, 1084, 2); got != -1 {
		t.Errorf("empty strip area = tab %d, want -1 (no tab)", got)
	}

	m.dpi = 192
	if got := m.indexAt(20, 2168, 2); got != 0 {
		t.Errorf("200%% left edge = tab %d, want 0", got)
	}
	if got := m.indexAt(300, 2168, 2); got != 0 {
		t.Errorf("200%% first tab body = tab %d, want 0", got)
	}
	// The same relative position as x=240 at 100%: still the second tab.
	if got := m.indexAt(480, 2168, 2); got != 1 {
		t.Errorf("200%% second tab body = tab %d, want 1", got)
	}
	if got := m.indexAt(1600, 2168, 2); got != -1 {
		t.Errorf("200%% empty strip area = tab %d, want -1", got)
	}
}

func TestDPIAwarenessWiring(t *testing.T) {
	shell, err := os.ReadFile("tabs_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(shell)
	for _, want := range []string{
		"tabWMDPICHanged = 0x02E0",
		"case tabWMDPICHanged:",
		"func windowDPI(hwnd uintptr) int",
		"tabGetDpiForWindow.Find()", // missing proc must not panic (DrawTextW lesson)
		"func (m *tabShell) metrics() tabMetrics",
		"func (m *tabShell) rebuildFonts()",
		"func (m *tabShell) sc(px int32) int32",
		"m.dpi = windowDPI(hwnd)",
		"m.rebuildFonts()",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("DPI wiring is missing %q", want)
		}
	}
	// The strip must not fall back to hardcoded pixel geometry: strip height,
	// tab width caps and the avatar dot all come from metrics().
	for _, banned := range []string{
		"h := int32(tabStripHeight - 8)",
		"x := int32(8) + int32(i)*(tabW+6)",
		"ar.Left, ar.Top, ar.Right, ar.Bottom = x+10, y+5, x+32, y+27",
	} {
		if strings.Contains(src, banned) {
			t.Errorf("unscaled strip geometry is back: %q", banned)
		}
	}
	// The manifest must keep declaring per-monitor awareness for this to work.
	manifest, err := os.ReadFile("app.manifest")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"PerMonitorV2", "true/pm"} {
		if !strings.Contains(string(manifest), want) {
			t.Errorf("app.manifest must keep %q", want)
		}
	}
}

// sumTabBadges aggregates unread counts; the taskbar shows the total.
func TestSumTabBadges(t *testing.T) {
	if got := sumTabBadges(map[string]int{}); got != 0 {
		t.Errorf("empty = %d, want 0", got)
	}
	if got := sumTabBadges(map[string]int{"a": 3, "b": 0, "c": 12}); got != 15 {
		t.Errorf("sum = %d, want 15", got)
	}
}

// Stability pins for the two reported hangs:
//  1. "Not responding": keyboard focus must never block the pump on a
//     waking renderer (MoveFocus stalls) — focus goes via the live child
//     HWND, with DOM sync best-effort off-pump.
//  2. Delete/reset "macet": only the fast UI half may run on the pump;
//     the slow wipe runs on a background thread, then repaints.
func TestTabStabilityPins(t *testing.T) {
	shell, err := os.ReadFile("tabs_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	ui, err := os.ReadFile("profiles_ui.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`func (m *tabShell) focusTabView`,
		`focusVisibleChildView(m.hwnd)`,
		`time.Sleep(500 * time.Millisecond)`,
		`removeProfileFromRegistry(id)`,
		`go func() {`,
		`wipeProfileDataDir(id)`,
		`window.__repaintProfiles = function()`,
		`m.active--`,
	} {
		here := string(shell)
		if strings.Contains(want, "repaintProfiles =") {
			here = string(ui)
		}
		if !strings.Contains(here, want) {
			t.Errorf("stability wiring is missing %q", want)
		}
	}
	// The pump must never call MoveFocus on a waking renderer: the resume
	// path goes through focusTabView. The only raw focus call is allowed for
	// a freshly rebuilt (hibernated) tab, which has no renderer to wait on.
	if got := strings.Count(string(shell), "cur.view.Focus()"); got != 1 {
		t.Errorf("cur.view.Focus() must exist only for the rebuilt tab, found %d", got)
	}
	if !strings.Contains(string(shell), "m.focusTabView(cur, wasSuspended)") {
		t.Error("the resume path must route focus through focusTabView")
	}
	if got := strings.Count(string(shell), "focusTabView("); got < 4 {
		t.Errorf("focusTabView has %d references, want def + 3 routed call sites", got)
	}
	// A second launch may name a registered profile with no live tab yet
	// (created elsewhere); the receiver must open it, not ignore it.
	if !strings.Contains(string(shell), "func (m *tabShell) activateOrOpenIndex") {
		t.Error("stale-tab recovery (activateOrOpenIndex) is missing")
	}
	// Display zoom must be native (controller-level reflow), applied to all
	// tabs, restored at creation, and re-applied after a wake.
	for _, want := range []string{
		"func (m *tabShell) applyZoomToAllTabs",
		"t.view.SetZoomFactor(z)",
		"zoom: getZoom()",
		"_ = c.SetZoomFactor(t.zoom)",
		"_ = cur.view.SetZoomFactor(cur.zoom)",
		"SetZoomFactor(float64) error",
	} {
		if !strings.Contains(string(shell), want) {
			t.Errorf("native zoom wiring is missing %q", want)
		}
	}
	if strings.Contains(string(shell), "document.body.style.zoom") {
		t.Error("Windows must not CSS-scale the page body; it leaves a background gap")
	}
}

// The active-tab renderer recycler: WhatsApp Web's renderer baseline climbs
// to ~0.7–1.0 GB after an hour of use and the page never returns it, so the
// shell rebuilds the engine in place after a long calm period. The rebuild
// must stay gated on page activity (downloads, open document preview) and
// reuse the hibernate wake path.
func TestActiveTabRecyclerWiring(t *testing.T) {
	shell, err := os.ReadFile("tabs_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(shell)
	for _, want := range []string{
		"tabRecycleAge = 90 * time.Minute",
		"func (m *tabShell) recycleActiveLocked",
		"m.recycleActiveLocked(now)", // wired into sweepHidden
		"func (m *tabShell) rebuildActiveTabLocked",
		"m.destroyView(t)",
		"m.buildView(t)",
		"func (m *tabShell) setProfileBusyState",
		"func (m *tabShell) recycleActiveProbeDone",
		"if busy.downloads > 0",
		"if busy.docmodal > 0",
		"window.__waBusyProbe", // staleness probe into the page
		"var tabSetProfileBusyState func(profileID, kind string, on bool)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("active-tab recycler is missing %q", want)
		}
	}

	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	page := string(main)
	for _, want := range []string{
		"window.__waReportBusy('download', true)",      // gate on in-flight downloads
		"window.__waReportBusy('download', false)",     // ...and clear on every exit
		"window.setBusyStateNative('docmodal', true)",  // gate while preview open
		"window.setBusyStateNative('docmodal', false)", // ...and clear on close
		"window.__waBusyProbe = function",              // answer the staleness probe
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page-side recycler gates are missing %q", want)
		}
	}

	app, err := os.ReadFile("app_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	bindings := string(app)
	for _, want := range []string{
		`_ = w.Bind("setBusyStateNative"`, // routes page reports to the shell
		"tabSetProfileBusyState = func(profileID, kind string, on bool)",
	} {
		if !strings.Contains(bindings, want) {
			t.Errorf("busy-state binding is missing %q", want)
		}
	}
}
