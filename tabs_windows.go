//go:build windows

package main

// Tabbed shell (Windows): one process hosts every profile as a Chrome-like
// tab. Each tab owns a dedicated WebView2 controller (isolated user-data
// dir, so accounts never log each other out); only the active tab is
// visible, hidden tabs are suspended after a grace period (idle: no CPU).
//
// Shape of the code:
//   - chromiumView adapts edge.Chromium to the webview2.WebView interface,
//     so setupProfileBindings (and the updater) work per tab unchanged.
//   - tabShell owns the outer window, the owner-drawn tab strip, one message
//     pump, suspend timers, badges, and second-launch IPC.
//   - All controller calls happen on the pump thread. Script bridges run on
//     WebView2 COM threads and marshal UI work through callOnPump.

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

// ---------------------------------------------------------------------------
// Win32 plumbing (tab-prefixed to avoid clashing with app/tray procs).
// ---------------------------------------------------------------------------

var (
	tabUser32   = windows.NewLazySystemDLL("user32.dll")
	tabGdi32    = windows.NewLazySystemDLL("gdi32.dll")
	tabKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	tabRegisterClassEx = tabUser32.NewProc("RegisterClassExW")
	tabCreateWindowEx  = tabUser32.NewProc("CreateWindowExW")
	tabDefWindowProc   = tabUser32.NewProc("DefWindowProcW")
	tabShowWindow      = tabUser32.NewProc("ShowWindow")
	tabUpdateWindow    = tabUser32.NewProc("UpdateWindow")
	tabGetMessage      = tabUser32.NewProc("GetMessageW")
	tabTranslate       = tabUser32.NewProc("TranslateMessage")
	tabDispatchMsg     = tabUser32.NewProc("DispatchMessageW")
	tabPostQuit        = tabUser32.NewProc("PostQuitMessage")
	tabDestroyWindow   = tabUser32.NewProc("DestroyWindow")
	tabGetClientRect   = tabUser32.NewProc("GetClientRect")
	tabSetWindowPos    = tabUser32.NewProc("SetWindowPos")
	tabSetWindowText   = tabUser32.NewProc("SetWindowTextW")
	tabSetTimer        = tabUser32.NewProc("SetTimer")
	tabKillTimer       = tabUser32.NewProc("KillTimer")
	tabPostThreadMsg   = tabUser32.NewProc("PostThreadMessageW")
	tabGetThreadID     = tabKernel32.NewProc("GetCurrentThreadId")
	tabGetModuleHandle = tabKernel32.NewProc("GetModuleHandleW")
	tabFindWindow      = tabUser32.NewProc("FindWindowW")
	tabSendMessage     = tabUser32.NewProc("SendMessageW")
	tabInvalidateRect  = tabUser32.NewProc("InvalidateRect")
	tabBeginPaint      = tabUser32.NewProc("BeginPaint")
	tabEndPaint        = tabUser32.NewProc("EndPaint")
	tabTrackMouse      = tabUser32.NewProc("TrackMouseEvent")
	tabLoadCursor      = tabUser32.NewProc("LoadCursorW")
	tabGetSysMetrics   = tabUser32.NewProc("GetSystemMetrics")
	tabSetFgWindow     = tabUser32.NewProc("SetForegroundWindow")
	tabShowNormal      = tabUser32.NewProc("ShowWindow")
	tabEnumChildren    = tabUser32.NewProc("EnumChildWindows")
	tabEnumWindows     = tabUser32.NewProc("EnumWindows")
	tabIsVisible       = tabUser32.NewProc("IsWindowVisible")
	tabSetFocus        = tabUser32.NewProc("SetFocus")
	tabGetClassName    = tabUser32.NewProc("GetClassNameW")
	tabGetWindowRect   = tabUser32.NewProc("GetWindowRect")
	tabGetDC           = tabUser32.NewProc("GetDC")
	tabReleaseDC       = tabUser32.NewProc("ReleaseDC")
	// GetDpiForWindow exists from Windows 10 1607; Find() guards the older
	// fallback so a missing proc can never panic (see the DrawTextW lesson).
	tabGetDpiForWindow = tabUser32.NewProc("GetDpiForWindow")
	tabGetDeviceCaps   = tabGdi32.NewProc("GetDeviceCaps")

	tabCreateFont     = tabGdi32.NewProc("CreateFontW")
	tabCreateBrush    = tabGdi32.NewProc("CreateSolidBrush")
	tabSelectObject   = tabGdi32.NewProc("SelectObject")
	tabDeleteObject   = tabGdi32.NewProc("DeleteObject")
	tabSetBkMode      = tabGdi32.NewProc("SetBkMode")
	tabSetTextColor   = tabGdi32.NewProc("SetTextColor")
	tabDrawText       = tabUser32.NewProc("DrawTextW")
	tabRoundRect      = tabGdi32.NewProc("RoundRect")
	tabEllipse        = tabGdi32.NewProc("Ellipse")
	tabCreatePen      = tabGdi32.NewProc("CreatePen")
	tabGetStockObject = tabGdi32.NewProc("GetStockObject")
	tabFillRect       = tabUser32.NewProc("FillRect")
)

const (
	tabClassOuter = "WAtchfulShell"
	tabClassStrip = "WAtchfulTabStrip"

	tabStripHeight = 40

	tabWMApp        = 0x8000
	tabWMTabGoTo    = 0x8001 // wParam: profile index+1, 0 = focus only
	tabWMDPICHanged = 0x02E0 // per-monitor DPI changed; lParam = suggested RECT

	// LOGPIXELSX / base DPI: 96 is 100% scaling.
	tabLogPixelsX = 88
	tabBaseDPI    = 96

	tabTimerSweep = 5001

	tabSuspendGrace = 30 * time.Second
	tabSweepEvery   = 5 * time.Second

	// tabHibernateGrace is how long a tab may stay in the background before
	// its engine is closed to give the RAM back. Suspend (above) only freezes
	// the page — measured: it does not return memory — so hibernation is what
	// actually keeps a multi-account setup light. Waking reloads the page
	// (~1–3 s); the account stays logged in because the session lives on disk.
	// Kept short on purpose: this app is meant to stay light.
	tabHibernateGrace = 2 * time.Minute

	// tabRecycleAge is how long the ACTIVE tab's page may run before the
	// recycler rebuilds its renderer in place. WhatsApp Web's renderer
	// baseline climbs with usage (measured: ~0.7–1.0 GB after an hour of
	// active use on a single account — not a linear leak, just heap/DOM/
	// decoded-media the page never returns) and no in-page GC gives it back.
	// A fresh renderer resumes at ~100–150 MB. The rebuild reuses the
	// hibernate wake path: the session lives on disk, the login persists,
	// and the tab keeps its strip position; the page reload is invisible
	// unless you watch RAM. Kept at 90 min so a single-account setup (which
	// never hibernates) also gets its memory back without a manual refresh.
	tabRecycleAge = 90 * time.Minute

	// tabRecycleProbeEvery throttles the page-busy probe: while a rebuild is
	// pending, ask the page at most once per minute instead of once per
	// sweep tick.
	tabRecycleProbeEvery = time.Minute

	tabWSOverlappedWindow = 0xCF0000
	tabWSChild            = 0x40000000
	tabWSVisible          = 0x10000000
	tabSWShow             = 5
	tabSWRestore          = 9
	tabSWPNoZOrder        = 0x0004
	tabSWPNoActivate      = 0x0010
	tabSWPNoMove          = 0x0002
	tabSWPNoSize          = 0x0001

	tabWMClose      = 0x0010
	tabWMCommand    = 0x0111
	tabWMTimer      = 0x0113
	tabWMSize       = 0x0005
	tabWMMove       = 0x0003
	tabWMActivate   = 0x0006
	tabWMDestroy    = 0x0002
	tabWMPaint      = 0x000F
	tabWMEraseBk    = 0x0014
	tabWMLButton    = 0x0201
	tabWMMouseMove  = 0x0200
	tabWMMouseLeave = 0x02A3

	tabSizeMinimized = 1

	tabDTLeft     = 0x0000
	tabDTCenter   = 0x0001
	tabDTRight    = 0x0002
	tabDTNoPrefix = 0x0800
	tabDTEllipsis = 0x00008000
	tabDTSingle   = 0x0020
	tabDTVCenter  = 0x0004
)

type tabRect struct{ Left, Top, Right, Bottom int32 }

type tabWndClass struct {
	Size, Style         uint32
	WndProc             uintptr
	ClsExtra, WndExtra  int32
	Instance, Icon      uintptr
	Cursor, Background  uintptr
	MenuName, ClassName uintptr
	IconSm              uintptr
}

type tabMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

type tabPaint struct {
	Hdc       uintptr
	Erase     uint32
	RcPaint   tabRect
	Restore   uint32
	IncUpdate uint32
	Reserved  [32]byte
}

type tabTrackMouseEvt struct {
	Size      uint32
	Flags     uint32
	HwndTrack uintptr
	HoverTime uint32
}

// ---------------------------------------------------------------------------
// chromiumView: webview2.WebView over a raw edge.Chromium.
// ---------------------------------------------------------------------------

type chromiumView struct {
	chromium *edge.Chromium
	outer    uintptr
	shell    *tabShell
	mu       sync.Mutex
	bindings map[string]interface{}
}

var _ webview2.WebView = (*chromiumView)(nil)

func tabJSString(v interface{}) string { b, _ := json.Marshal(v); return string(b) }

func (v *chromiumView) Bind(name string, f interface{}) error {
	rv := reflect.ValueOf(f)
	if rv.Kind() != reflect.Func {
		return errors.New("only functions can be bound")
	}
	if n := rv.Type().NumOut(); n > 2 {
		return errors.New("function may only return a value or a value+error")
	}
	v.mu.Lock()
	if v.bindings == nil {
		v.bindings = map[string]interface{}{}
	}
	v.bindings[name] = f
	v.mu.Unlock()
	v.Init("(function() { var name = " + tabJSString(name) + ";" + `
		var RPC = window._rpc = (window._rpc || {nextSeq: 1});
		window[name] = function() {
		  var seq = RPC.nextSeq++;
		  var promise = new Promise(function(resolve, reject) {
			RPC[seq] = {
			  resolve: resolve,
			  reject: reject,
			};
		  });
		  window.external.invoke(JSON.stringify({
			id: seq,
			method: name,
			params: Array.prototype.slice.call(arguments),
		  }));
		  return promise;
		}
	})()`)
	return nil
}

type tabRPCMessage struct {
	ID     int               `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func (v *chromiumView) routeMessage(payload string) {
	var d tabRPCMessage
	if err := json.Unmarshal([]byte(payload), &d); err != nil {
		log.Printf("tab: invalid RPC message: %v", err)
		return
	}
	id := strconv.Itoa(d.ID)
	v.mu.Lock()
	f, ok := v.bindings[d.Method]
	v.mu.Unlock()
	if !ok {
		return
	}
	rv := reflect.ValueOf(f)
	isVariadic := rv.Type().IsVariadic()
	numIn := rv.Type().NumIn()
	if (isVariadic && len(d.Params) < numIn-1) || (!isVariadic && len(d.Params) != numIn) {
		v.shell.dispatch(func() {
			v.chromium.Eval("window._rpc[" + id + "].reject(" + tabJSString("function arguments mismatch") + ")")
		})
		return
	}
	args := []reflect.Value{}
	for i := range d.Params {
		var arg reflect.Value
		if isVariadic && i >= numIn-1 {
			arg = reflect.New(rv.Type().In(numIn - 1).Elem())
		} else {
			arg = reflect.New(rv.Type().In(i))
		}
		if err := json.Unmarshal(d.Params[i], arg.Interface()); err != nil {
			v.shell.dispatch(func() {
				v.chromium.Eval("window._rpc[" + id + "].reject(" + tabJSString(err.Error()) + ")")
			})
			return
		}
		args = append(args, arg.Elem())
	}
	res := rv.Call(args)
	errType := reflect.TypeOf((*error)(nil)).Elem()
	var out interface{}
	var callErr error
	switch len(res) {
	case 0:
	case 1:
		if res[0].Type().Implements(errType) {
			if res[0].Interface() != nil {
				callErr = res[0].Interface().(error)
			}
		} else {
			out = res[0].Interface()
		}
	case 2:
		if !res[1].Type().Implements(errType) {
			callErr = errors.New("second return value must be an error")
		} else if res[1].Interface() == nil {
			out = res[0].Interface()
		} else {
			callErr = res[1].Interface().(error)
		}
	default:
		callErr = errors.New("unexpected number of return values")
	}
	if callErr != nil {
		v.shell.dispatch(func() {
			v.chromium.Eval("window._rpc[" + id + "].reject(" + tabJSString(callErr.Error()) + "); window._rpc[" + id + "] = undefined")
		})
		return
	}
	b, err := json.Marshal(out)
	if err != nil {
		v.shell.dispatch(func() {
			v.chromium.Eval("window._rpc[" + id + "].reject(" + tabJSString(err.Error()) + "); window._rpc[" + id + "] = undefined")
		})
		return
	}
	v.shell.dispatch(func() {
		v.chromium.Eval("window._rpc[" + id + "].resolve(" + string(b) + "); window._rpc[" + id + "] = undefined")
	})
}

func (v *chromiumView) Init(js string)         { v.chromium.Init(js) }
func (v *chromiumView) Eval(js string)         { v.chromium.Eval(js) }
func (v *chromiumView) Navigate(url string)    { v.chromium.Navigate(url) }
func (v *chromiumView) SetHtml(html string)    { v.chromium.NavigateToString(html) }
func (v *chromiumView) Suspend() bool          { return v.chromium.Suspend() }
func (v *chromiumView) Resume() bool           { return v.chromium.Resume() }
func (v *chromiumView) Window() unsafe.Pointer { return v.chromium.HostWindow() }

// applyZoomToAllTabs sets the display zoom for every tab (it is one global
// setting, so tabs must not disagree). Views that refuse while hidden or
// suspended keep the level in their entry and get it re-applied on wake.
func (m *tabShell) applyZoomToAllTabs(z float64) {
	tabCallOnPump(m, func() struct{} {
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, t := range m.tabs {
			t.zoom = z
			if t.view == nil {
				continue // hibernated: buildView re-applies t.zoom on wake
			}
			_ = t.view.SetZoomFactor(z)
		}
		return struct{}{}
	})
}

// zoomSetter is implemented by views that can apply a native page zoom
// (browser-style reflow instead of CSS scaling).
type zoomSetter interface {
	SetZoomFactor(float64) error
}

// View-visibility helpers used by the tab manager. Bounds always go through
// the manager (layoutViews) so views sit below the tab strip.
func (v *chromiumView) Show() error { return v.chromium.Show() }
func (v *chromiumView) Hide() error { return v.chromium.Hide() }
func (v *chromiumView) Focus()      { v.chromium.Focus() }

// SetZoomFactor applies the WebView2 page zoom factor (1.0 = 100%).
func (v *chromiumView) SetZoomFactor(zoom float64) error {
	return v.chromium.SetZoomFactor(zoom)
}

func (v *chromiumView) Dispatch(f func()) { v.shell.dispatch(f) }

func (v *chromiumView) SetTitle(title string) {
	t, _ := windows.UTF16PtrFromString(title)
	tabSetWindowText.Call(v.outer, uintptr(unsafe.Pointer(t)))
}

func (v *chromiumView) SetSize(w int, h int, hint webview2.Hint) {
	// Minimum-size hints are owned by the outer window (configureWindow);
	// per-view minimums were stored but never applied anywhere.
	if hint == webview2.HintNone && w > 0 && h > 0 {
		tabSetWindowPos.Call(v.outer, 0, 0, 0, uintptr(w), uintptr(h),
			uintptr(tabSWPNoZOrder|tabSWPNoActivate|tabSWPNoMove))
	}
}

func (v *chromiumView) Run() {
	var m tabMsg
	for {
		r, _, _ := tabGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if m.Message == tabWMApp {
			v.shell.drainPump()
			continue
		}
		tabTranslate.Call(uintptr(unsafe.Pointer(&m)))
		tabDispatchMsg.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (v *chromiumView) Terminate() { tabPostQuit.Call(0) }

func (v *chromiumView) Destroy() { _ = v.chromium.Hide() }

func (v *chromiumView) SetBrowserAcceleratorKeysEnabled(enabled bool) error {
	s, err := v.chromium.GetSettings()
	if err != nil {
		return err
	}
	return s.PutAreBrowserAcceleratorKeysEnabled(enabled)
}

// ---------------------------------------------------------------------------
// tabShell: one outer window, N profile views, one pump.
// ---------------------------------------------------------------------------

type tabEntry struct {
	profile       Profile
	ctx           *profileViewContext
	view          *chromiumView
	booted        bool
	hiddenSince   time.Time
	suspendedByUs bool
	// hibernated means the WebView2 controller was closed to free its memory
	// (~400 MB per account) after a long idle period; the tab keeps its place
	// in the strip and is rebuilt on the next click. The login survives on
	// disk, so waking only costs a page reload.
	hibernated bool
	// zoom is this tab's display zoom (1.0 = 100%). Kept on the entry so it
	// survives both suspend/resume and a hibernate/rebuild cycle.
	zoom float64

	// activeSince is when the current page finished loading (recycler age
	// reference). recycleProbedAt throttles the recycler's busy probes.
	activeSince     time.Time
	recycleProbedAt time.Time
}

// tabBusyState counts in-page activity that must block a recycler rebuild:
// downloads in flight and an open in-app document preview.
type tabBusyState struct {
	downloads int
	docmodal  int
}

// tabSetProfileBusyState is installed by the tab shell for the page to
// report activity that must block a recycler rebuild. Nil when no shell
// exists (non-tab builds).
var tabSetProfileBusyState func(profileID, kind string, on bool)

type tabShell struct {
	hwnd       uintptr
	strip      uintptr
	pumpThread uint32

	pumpMu sync.Mutex
	pumpQ  []func()

	mu     sync.Mutex // guards tabs/active/badges below
	tabs   []*tabEntry
	active int
	badges map[string]int

	// busyMu guards the page-side busy signals (recycler gates).
	busyMu sync.Mutex
	busy   map[string]tabBusyState

	isDark   bool
	hoverTab int
	// dpi is the window's current DPI (96 = 100%). The strip is owner-drawn,
	// so every pixel of it must be scaled by hand; the page itself is scaled
	// by WebView2. Written and read on the pump thread only.
	dpi int

	fontText uintptr
	fontBold uintptr

	executablePath string
	iconFullPath   string
}

var theShell *tabShell

func tabShellActive() bool { return theShell != nil }

// dispatch queues f on the pump thread (async, safe from any thread).
func (m *tabShell) dispatch(f func()) {
	m.pumpMu.Lock()
	m.pumpQ = append(m.pumpQ, f)
	m.pumpMu.Unlock()
	tabPostThreadMsg.Call(uintptr(m.pumpThread), tabWMApp, 0, 0)
}

func (m *tabShell) drainPump() {
	m.pumpMu.Lock()
	q := m.pumpQ
	m.pumpQ = nil
	m.pumpMu.Unlock()
	for _, f := range q {
		f()
	}
}

func (m *tabShell) onPumpThread() bool {
	id, _, _ := tabGetThreadID.Call()
	return uint32(id) == m.pumpThread
}

// callOnPump runs fn on the pump thread and waits for its result. Safe from
// COM threads; callers already on the pump run inline to avoid self-deadlock.
func tabCallOnPump[T any](m *tabShell, fn func() T) T {
	if m.onPumpThread() {
		return fn()
	}
	done := make(chan T, 1)
	m.dispatch(func() { done <- fn() })
	return <-done
}

func (m *tabShell) activeEntry() *tabEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active < 0 || m.active >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.active]
}

func (m *tabShell) evalActive(script string) {
	t := m.activeEntry()
	if t == nil || t.view == nil {
		return
	}
	v := t.view
	m.dispatch(func() { v.Eval(script) })
}

func tabEscapeJS(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (m *tabShell) toastActive(msg string) {
	m.evalActive("if (window.showFloatingToast) { window.showFloatingToast(" + tabEscapeJS(msg) + "); }")
}

// ---------------------------------------------------------------------------
// Tab operations (pump-thread bodies; public wrappers marshal as needed).
// ---------------------------------------------------------------------------

func (m *tabShell) titleFor(p Profile) string {
	if p.ID == defaultProfileID || strings.TrimSpace(p.Name) == "" {
		return windowTitle
	}
	return windowTitle + " — " + p.Name
}

func (m *tabShell) layoutViews() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.layoutViewsLocked()
}

func (m *tabShell) layoutViewsLocked() {
	var r tabRect
	tabGetClientRect.Call(m.hwnd, uintptr(unsafe.Pointer(&r)))
	y := m.metrics().stripH
	w := r.Right - r.Left
	h := r.Bottom - r.Top - y
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	for _, t := range m.tabs {
		if t.view == nil {
			continue // hibernated: nothing to position until it wakes
		}
		_ = t.view.chromium.SetBoundsRect(0, y, w, h)
	}
}

func (m *tabShell) activateLocked(i int) {
	if i < 0 || i >= len(m.tabs) {
		return
	}
	prev := -1
	if m.active >= 0 && m.active < len(m.tabs) {
		prev = m.active
	}
	if prev == i {
		m.focusTabView(m.tabs[i], m.tabs[i].suspendedByUs)
		return
	}
	if prev >= 0 {
		p := m.tabs[prev]
		if p.view != nil {
			_ = p.view.Hide()
		}
		p.hiddenSince = time.Now()
		p.suspendedByUs = false
	}
	m.active = i
	cur := m.tabs[i]
	cur.hiddenSince = time.Time{}
	cur.activeSince = time.Now()
	cur.recycleProbedAt = time.Time{}
	setActiveProfile(cur.profile)

	if cur.hibernated {
		// Waking a hibernated tab rebuilds its engine (page reload). Stay put
		// if that fails, so the shell always keeps one working tab.
		if err := m.buildView(cur); err != nil {
			m.active = prev
			m.toastActiveLocked("⚠️ Could not open \"" + cur.profile.Name + "\". Click it again to retry.")
			return
		}
		m.layoutViewsLocked()
		cur.view.Focus()
		m.refreshChromeLocked()
		return
	}

	wasSuspended := cur.suspendedByUs
	_ = cur.view.Resume()
	cur.suspendedByUs = false
	m.layoutViewsLocked()
	if wasSuspended && cur.zoom != 0 {
		// A suspend/resume cycle can drop the controller zoom; re-apply it
		// so the level survives switching away and back.
		_ = cur.view.SetZoomFactor(cur.zoom)
	}
	m.focusTabView(cur, wasSuspended)
	m.refreshChromeLocked()
}

// focusTabView moves keyboard focus into t's page without ever blocking the
// pump on the renderer: MoveFocus stalls while the renderer still wakes from
// suspend, which Windows reports as "Not responding". A waking tab gets pure
// Win32 focus immediately plus a best-effort DOM sync off-pump.
func (m *tabShell) focusTabView(t *tabEntry, wasSuspended bool) {
	if t.view == nil {
		return // hibernated: focus follows the rebuild on wake
	}
	if !wasSuspended {
		t.view.Focus()
		return
	}
	focusVisibleChildView(m.hwnd)
	v := t.view
	go func() {
		time.Sleep(500 * time.Millisecond)
		// The tab may have been hibernated again in the meantime.
		m.mu.Lock()
		stale := t.view != v
		m.mu.Unlock()
		if !stale {
			v.Focus()
		}
	}()
}

// focusVisibleChildView moves keyboard focus into the visible WebView2
// child with pure Win32 calls. Unlike controller MoveFocus it never waits
// on the renderer, so it is safe right after waking a suspended tab.
func focusVisibleChildView(outer uintptr) {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		vis, _, _ := tabIsVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		var cls [64]uint16
		n, _, _ := tabGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&cls[0])), 64)
		if n <= 0 {
			return 1
		}
		if windows.UTF16ToString(cls[:n]) == "Chrome_WidgetWin_0" {
			found = hwnd
			return 0
		}
		return 1
	})
	tabEnumChildren.Call(outer, cb, 0)
	if found != 0 {
		tabSetFocus.Call(found)
	}
}

func (m *tabShell) refreshChromeLocked() {
	if m.active < 0 || m.active >= len(m.tabs) {
		return
	}
	t := m.tabs[m.active]
	title := m.titleFor(t.profile)
	name, _ := windows.UTF16PtrFromString(title)
	tabSetWindowText.Call(m.hwnd, uintptr(unsafe.Pointer(name)))
	trayEvalDispatch = func(script string) {
		m.evalActive(script)
	}
	tabInvalidateRect.Call(m.strip, 0, 1)
}

// activateOrOpenIndex activates tabs[i], opening it first when the tab
// list is stale (e.g. a second launch registered a profile we haven't
// built a controller for yet). Bounds-checked; out-of-range is a no-op.
func (m *tabShell) activateOrOpenIndex(i int) {
	m.mu.Lock()
	if i >= 0 && i < len(m.tabs) {
		m.activateLocked(i)
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	reg := listProfiles()
	if i < 0 || i >= len(reg) {
		return
	}
	m.openTab(reg[i])
}

// activateTab switches to tabs[i]. Safe from any thread.
func (m *tabShell) activateTab(i int) {
	tabCallOnPump(m, func() struct{} {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.activateLocked(i)
		return struct{}{}
	})
}

func (m *tabShell) activateByName(name string) bool {
	p := findProfileByIDOrName(loadProfileRegistry(), name)
	if p == nil {
		return false
	}
	touchProfileLastUsed(p.ID)
	return tabCallOnPump(m, func() bool {
		m.mu.Lock()
		for i, t := range m.tabs {
			if t.profile.ID == p.ID {
				m.activateLocked(i)
				m.mu.Unlock()
				tabShowWindow.Call(m.hwnd, tabSWRestore)
				tabSetFgWindow.Call(m.hwnd)
				return true
			}
		}
		m.mu.Unlock()
		// Registered but tabless (created elsewhere while we run): open it.
		// Embed runs without holding m.mu (nested pump safe).
		entry, err := m.createTabView(*p)
		if err != nil {
			m.toastActive("⚠️ Could not open profile \"" + p.Name + "\".")
			return false
		}
		m.mu.Lock()
		m.tabs = append(m.tabs, entry)
		m.activateLocked(len(m.tabs) - 1)
		m.mu.Unlock()
		tabShowWindow.Call(m.hwnd, tabSWRestore)
		tabSetFgWindow.Call(m.hwnd)
		return true
	})
}

func (m *tabShell) cycle(dir int) {
	tabCallOnPump(m, func() struct{} {
		m.mu.Lock()
		defer m.mu.Unlock()
		if len(m.tabs) == 0 {
			return struct{}{}
		}
		m.activateLocked((m.active + dir + len(m.tabs)) % len(m.tabs))
		return struct{}{}
	})
}

func (m *tabShell) activateIndex(i int) {
	tabCallOnPump(m, func() struct{} {
		m.mu.Lock()
		defer m.mu.Unlock()
		if len(m.tabs) == 0 {
			return struct{}{}
		}
		if i < 0 {
			return struct{}{}
		}
		if i >= len(m.tabs) {
			i = len(m.tabs) - 1 // Ctrl+9 jumps to the last tab, like Chrome
		}
		m.activateLocked(i)
		return struct{}{}
	})
}

// createTabView builds one controller for p. MUST run on the pump thread:
// WebView2 controller creation is apartment-threaded.
// buildView creates t's WebView2 controller and loads WhatsApp into it.
// MUST run on the pump thread (controller creation is apartment-threaded).
// It fills the existing entry, so it serves both a brand-new tab and waking
// a hibernated one (which keeps its strip position, badge and zoom).
func (m *tabShell) buildView(t *tabEntry) error {
	p := t.profile
	dir := profileDirFor(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	c := edge.NewChromium()
	c.DataPath = dir
	v := &chromiumView{chromium: c, outer: m.hwnd, shell: m}
	c.MessageCallback = v.routeMessage
	if !c.Embed(m.hwnd) {
		return errors.New("failed to embed webview2 for profile " + p.Name)
	}
	c.SetPermission(edge.CoreWebView2PermissionKindClipboardRead, edge.CoreWebView2PermissionStateAllow)
	if s, err := c.GetSettings(); err == nil {
		_ = s.PutAreDefaultContextMenusEnabled(false)
		_ = s.PutAreDevToolsEnabled(false)
		_ = s.PutAreBrowserAcceleratorKeysEnabled(false)
	}
	if t.zoom == 0 {
		t.zoom = getZoom()
	}
	// Apply the display zoom before the first paint so the page never flashes
	// at 100%. Native zoom reflows (browser-like), unlike the CSS body zoom
	// this replaced, which left a background gap on WhatsApp.
	if t.zoom != 1.0 {
		_ = c.SetZoomFactor(t.zoom)
	}
	t.booted = false
	t.suspendedByUs = false
	t.hibernated = false
	c.NavigationCompletedCallback = func(_ *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
		t.booted = true
		t.activeSince = time.Now()
	}
	t.view = v
	t.ctx = &profileViewContext{
		profile:        p,
		userDataDir:    dir,
		executablePath: m.executablePath,
		iconFullPath:   m.iconFullPath,
		hwnd:           m.hwnd,
	}
	setupProfileBindings(v, t.ctx)
	v.Init(getInitScript(userAgent))
	v.Navigate(appURL)
	return nil
}

func (m *tabShell) createTabView(p Profile) (*tabEntry, error) {
	t := &tabEntry{profile: p, zoom: getZoom()}
	if err := m.buildView(t); err != nil {
		return nil, err
	}
	return t, nil
}

// hibernateLocked closes t's controller to actually release its memory. The
// tab stays in the strip (dimmed) and is rebuilt on the next click. Callers
// hold m.mu.
func (m *tabShell) hibernateLocked(t *tabEntry) {
	if t.view == nil || t.hibernated {
		return
	}
	m.destroyView(t)
	t.suspendedByUs = false
}

// shouldSuspend / shouldHibernate decide what happens to a tab that has been
// hidden for d: suspend first (freezes the page, saves CPU/battery, keeps
// memory), then hibernate (closes the engine, frees ~400 MB, reloads on wake).
func shouldSuspend(d time.Duration) bool   { return d >= tabSuspendGrace }
func shouldHibernate(d time.Duration) bool { return d >= tabHibernateGrace }

// openTab creates the tab for p if missing and activates it.
func (m *tabShell) openTab(p Profile) {
	tabCallOnPump(m, func() struct{} {
		m.mu.Lock()
		for i, t := range m.tabs {
			if t.profile.ID == p.ID {
				m.activateLocked(i)
				m.mu.Unlock()
				return struct{}{}
			}
		}
		m.mu.Unlock()
		entry, err := m.createTabView(p)
		if err != nil {
			m.toastActive("⚠️ Could not open profile \"" + p.Name + "\".")
			return struct{}{}
		}
		m.mu.Lock()
		m.tabs = append(m.tabs, entry)
		m.activateLocked(len(m.tabs) - 1)
		m.mu.Unlock()
		return struct{}{}
	})
}

func (m *tabShell) destroyView(t *tabEntry) {
	if t.view == nil {
		return // already hibernated
	}
	_ = t.view.Hide()
	_ = t.view.chromium.CloseController()
	t.view = nil
	t.ctx = nil
	t.hibernated = true
	t.booted = false
	t.activeSince = time.Time{}
	// A rebuilt page re-reports its own busy state; drop stale counters so
	// the recycler never stays blocked by a page that no longer exists.
	m.busyMu.Lock()
	delete(m.busy, t.profile.ID)
	m.busyMu.Unlock()
}

func (m *tabShell) removeTabLocked(id string) {
	for i, t := range m.tabs {
		if t.profile.ID == id {
			m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
			// Keep m.active pointing at the same tab: removal before it
			// shifts everything down; removal of the active tab itself
			// leaves -1 so callers must activate explicitly.
			switch {
			case m.active == i:
				m.active = -1
			case m.active > i:
				m.active--
			}
			return
		}
	}
}

func tabRetryOnLock(tries int, pause time.Duration, fn func() error) error {
	var err error
	for i := 0; i < tries; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(pause)
	}
	return err
}

// tabManagerDeleteProfile removes a profile without ever blocking the pump
// on the slow parts: tab UI + registry update happen synchronously (snappy),
// while the possibly-locked, possibly-large data folder is wiped on a
// background thread with generous retries. The page already toasts + repaints
// on success, so the background completion only toasts on failure.
func tabManagerDeleteProfile(id string) bool {
	m := theShell
	if m == nil {
		return false
	}
	proceed := tabCallOnPump(m, func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		t := m.byIDLocked(id)
		if t == nil {
			return false
		}
		if id == defaultProfileID {
			m.toastActiveLocked("⚠️ The Default profile cannot be deleted.")
			return false
		}
		if len(m.tabs) == 1 {
			m.toastActiveLocked("⚠️ Create another profile before deleting this one.")
			return false
		}
		if m.activeTabLocked().profile.ID == id && !m.activateNeighborLocked(id) {
			return false
		}
		m.destroyView(t)
		if err := removeProfileFromRegistry(id); err != nil {
			// Registry untouched: reboot the view to stay consistent.
			if _, rerr := m.createTabViewLocked(t.profile); rerr == nil {
				m.refreshChromeLocked()
			}
			m.toastActiveLocked("⚠️ Could not delete profile: " + err.Error())
			return false
		}
		m.removeTabLocked(id)
		m.refreshChromeLocked()
		return true
	})
	if !proceed {
		return false
	}
	go func() {
		err := tabRetryOnLock(30, 500*time.Millisecond, func() error {
			return wipeProfileDataDir(id)
		})
		tabCallOnPump(m, func() struct{} {
			if err != nil {
				m.toastActive("⚠️ Removed from the list, but its data folder is still locked and will retry on restart.")
			}
			m.repaintProfiles()
			return struct{}{}
		})
	}()
	return true
}

// activateNeighborLocked parks the active tab on the first tab that is not
// exceptID. Callers hold m.mu.
func (m *tabShell) activateNeighborLocked(exceptID string) bool {
	for i, cand := range m.tabs {
		if cand.profile.ID != exceptID {
			m.activateLocked(i)
			return true
		}
	}
	return false
}

// repaintProfiles re-renders the Settings Profiles card after async native
// ops complete (safe from any thread).
func (m *tabShell) repaintProfiles() {
	m.evalActive(`if (typeof window.__repaintProfiles === 'function') { try { window.__repaintProfiles(); } catch (e) {} }`)
}

func (m *tabShell) byIDLocked(id string) *tabEntry {
	for _, t := range m.tabs {
		if t.profile.ID == id {
			return t
		}
	}
	return nil
}

func (m *tabShell) activeTabLocked() *tabEntry {
	if m.active < 0 || m.active >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.active]
}

func (m *tabShell) toastActiveLocked(msg string) {
	m.evalActive("if (window.showFloatingToast) { window.showFloatingToast(" + tabEscapeJS(msg) + "); }")
}

func (m *tabShell) createTabViewLocked(p Profile) (*tabEntry, error) {
	entry, err := m.createTabView(p)
	if err != nil {
		return nil, err
	}
	m.tabs = append(m.tabs, entry)
	return entry, nil
}

// tabManagerRenameProfile refreshes tab chrome after a rename.
func tabManagerRenameProfile(id string) {
	m := theShell
	if m == nil {
		return
	}
	tabCallOnPump(m, func() struct{} {
		m.mu.Lock()
		defer m.mu.Unlock()
		if t := m.byIDLocked(id); t != nil {
			if p := findProfileByIDOrName(loadProfileRegistry(), id); p != nil {
				t.profile = *p
				if t.ctx != nil {
					t.ctx.profile = *p
				}
			}
		}
		m.refreshChromeLocked()
		return struct{}{}
	})
}

// tabManagerResetProfile wipes a tab's session and reboots its controller.
// Only the fast UI half runs on the pump (destroy, drop tab); the wipe runs
// on a background thread with generous retries, then the tab is recreated
// at its original position. The page already toasts + repaints on success,
// so the background completion only toasts on failure.
func tabManagerResetProfile(id string) bool {
	m := theShell
	if m == nil {
		return false
	}
	type resetPlan struct {
		ok        bool
		profile   Profile
		index     int
		wasActive bool
	}
	plan := tabCallOnPump(m, func() resetPlan {
		m.mu.Lock()
		defer m.mu.Unlock()
		idx := -1
		for i, cand := range m.tabs {
			if cand.profile.ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			return resetPlan{}
		}
		wasActive := idx == m.active
		if wasActive {
			m.activateNeighborLocked(id) // no-op when sole tab
		}
		t := m.byIDLocked(id)
		if t == nil {
			return resetPlan{}
		}
		m.destroyView(t)
		m.removeTabLocked(id)
		m.refreshChromeLocked()
		return resetPlan{ok: true, profile: t.profile, index: idx, wasActive: wasActive}
	})
	if !plan.ok {
		return false
	}
	go func() {
		err := tabRetryOnLock(30, 500*time.Millisecond, func() error {
			if werr := wipeProfileDataDir(id); werr != nil {
				return werr
			}
			return os.MkdirAll(profileDirFor(Profile{ID: id}), 0755)
		})
		tabCallOnPump(m, func() struct{} {
			m.mu.Lock()
			defer m.mu.Unlock()
			if err == nil {
				p := plan.profile
				if fresh := findProfileByIDOrName(loadProfileRegistry(), id); fresh != nil {
					p = *fresh
				}
				entry, rerr := m.createTabViewLocked(p)
				if rerr == nil {
					// Keep the original tab position.
					last := len(m.tabs) - 1
					if plan.index < last {
						e := m.tabs[last]
						copy(m.tabs[plan.index+1:], m.tabs[plan.index:])
						m.tabs[plan.index] = e
						if m.active >= plan.index {
							m.active++
						}
					}
					if plan.wasActive {
						m.activateLocked(plan.index)
					} else {
						_ = entry.view.Hide()
						entry.hiddenSince = time.Now()
					}
				} else {
					m.toastActiveLocked("⚠️ Could not reopen profile \"" + p.Name + "\". Use its Switch button to retry.")
				}
			} else {
				m.toastActiveLocked("⚠️ Could not reset profile: " + err.Error())
			}
			m.refreshChromeLocked()
			return struct{}{}
		})
		// The page handler repaints itself; hook again for late registry reads.
		m.repaintProfiles()
	}()
	return true
}

// sumTabBadges aggregates per-tab unread counts for the taskbar.
func sumTabBadges(badges map[string]int) int {
	total := 0
	for _, c := range badges {
		total += c
	}
	return total
}

// tabManagerReportBadge aggregates per-tab unread counts onto the taskbar.
func tabManagerReportBadge(id string, count int) {
	m := theShell
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.badges == nil {
		m.badges = map[string]int{}
	}
	if count <= 0 {
		delete(m.badges, id)
	} else {
		m.badges[id] = count
	}
	total := sumTabBadges(m.badges)
	m.mu.Unlock()
	_ = setTaskbarBadge(m.hwnd, total)
	tabInvalidateRect.Call(m.strip, 0, 1)
}

// tabManagerSetTheme repaints the native strip on theme changes.
func tabManagerSetTheme(theme string) {
	m := theShell
	if m == nil {
		return
	}
	isDark := theme == "dark"
	if theme == "system" {
		isDark = isWindowsSystemDarkTheme()
	}
	m.mu.Lock()
	m.isDark = isDark
	m.mu.Unlock()
	tabInvalidateRect.Call(m.strip, 0, 1)
}

// tabManagerToast surfaces a message on the active tab (no-op pre-shell).
func tabManagerToast(msg string) {
	m := theShell
	if m == nil {
		return
	}
	m.toastActive(msg)
}

// tabManagerActivateByName / Cycle / ActivateIndex: thin wrappers used by
// bridges, the tray hook and IPC.
func tabManagerOpenProfile(p Profile) {
	if m := theShell; m != nil {
		m.openTab(p)
	}
}

func tabManagerActivateByName(name string) bool {
	m := theShell
	if m == nil {
		return false
	}
	return m.activateByName(name)
}

func tabManagerCycle(dir int) {
	if m := theShell; m != nil {
		m.cycle(dir)
	}
}

func tabManagerActivateIndex(i int) {
	if m := theShell; m != nil {
		m.activateIndex(i)
	}
}

// ---------------------------------------------------------------------------
// Shell bootstrap, window procedures, tab strip, single instance.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// DPI awareness (per-monitor v2). WebView2 scales the page itself; the
// owner-drawn tab strip is scaled here so it matches the rest of the UI on
// 125–200% displays instead of staying tiny.
// ---------------------------------------------------------------------------

// windowDPI returns the DPI of the monitor hwnd sits on (96 = 100%). Falls
// back to the system DPI on Windows builds without GetDpiForWindow.
func windowDPI(hwnd uintptr) int {
	if hwnd != 0 {
		if err := tabGetDpiForWindow.Find(); err == nil {
			if dpi, _, _ := tabGetDpiForWindow.Call(hwnd); dpi >= 48 && dpi <= 480 {
				return int(dpi)
			}
		}
	}
	dc, _, _ := tabGetDC.Call(0)
	if dc == 0 {
		return tabBaseDPI
	}
	dpi, _, _ := tabGetDeviceCaps.Call(dc, tabLogPixelsX)
	tabReleaseDC.Call(0, dc)
	if dpi < 48 || dpi > 480 {
		return tabBaseDPI
	}
	return int(dpi)
}

// sc scales a 96-DPI pixel value to the window's current DPI.
func (m *tabShell) sc(px int32) int32 {
	dpi := m.dpi
	if dpi < 48 {
		dpi = tabBaseDPI
	}
	return int32(math.Round(float64(px) * float64(dpi) / float64(tabBaseDPI)))
}

// tabMetrics is the strip's DPI-scaled geometry, derived in one place so
// painting, hit-testing and layout can never disagree.
type tabMetrics struct {
	stripH int32
	pad    int32
	gap    int32
	top    int32
	radius int32
	dot    int32
	nameX  int32
	minW   int32
	maxW   int32
	barIn  int32
	barH   int32
	badgeW int32
	badgeH int32
	badgeY int32
	// verW reserves the right-edge slot for the muted build-version tag
	// ("v" + appVersion) so bug-report screenshots always show the running build.
	verW int32
}

func (m *tabShell) metrics() tabMetrics {
	return tabMetrics{
		stripH: m.sc(tabStripHeight),
		pad:    m.sc(8),
		gap:    m.sc(6),
		top:    m.sc(4),
		radius: m.sc(12),
		dot:    m.sc(22),
		nameX:  m.sc(38),
		minW:   m.sc(60),
		maxW:   m.sc(220),
		barIn:  m.sc(10),
		barH:   m.sc(3),
		badgeW: m.sc(28),
		badgeH: m.sc(16),
		badgeY: m.sc(8),
		verW:   m.sc(64),
	}
}

// rebuildFonts recreates the strip fonts for the current DPI.
func (m *tabShell) rebuildFonts() {
	if m.fontText != 0 {
		tabDeleteObject.Call(m.fontText)
		m.fontText = 0
	}
	if m.fontBold != 0 {
		tabDeleteObject.Call(m.fontBold)
		m.fontBold = 0
	}
	h := -m.sc(13)
	m.fontText = tabMakeFont(400, h)
	m.fontBold = tabMakeFont(700, h)
}

func ebCacheArgs(dir string) (roots, subs []string) {
	roots = []string{filepath.Join(dir, "EBWebView")}
	subs = []string{
		filepath.Join(dir, "EBWebView", "Default", "Cache"),
		filepath.Join(dir, "EBWebView", "Default", "GPUCache"),
		filepath.Join(dir, "EBWebView", "Default", "Code Cache"),
		filepath.Join(dir, "EBWebView", "Default", "Service Worker"),
	}
	return roots, subs
}

func tabRGB(r, g, b int32) uintptr {
	return uintptr((b << 16) | (g << 8) | r)
}

func (m *tabShell) stripColors() (bg, tabActive, tabHover, text, muted, accent, avatarText uintptr) {
	isDark := m.isDark
	if isDark {
		return tabRGB(0x11, 0x1b, 0x21), tabRGB(0x20, 0x2c, 0x33), tabRGB(0x1a, 0x25, 0x2b),
			tabRGB(0xe9, 0xed, 0xef), tabRGB(0x86, 0x96, 0xa0), tabRGB(0x00, 0xa8, 0x84),
			tabRGB(0x11, 0x1b, 0x21)
	}
	return tabRGB(0xf0, 0xf2, 0xf5), tabRGB(0xff, 0xff, 0xff), tabRGB(0xe4, 0xe7, 0xea),
		tabRGB(0x11, 0x1b, 0x21), tabRGB(0x66, 0x77, 0x81), tabRGB(0x00, 0x80, 0x69),
		tabRGB(0xff, 0xff, 0xff)
}

type tabPaintSnapshot struct {
	ids        []string
	names      []string
	badges     []int
	hibernated []bool
	active     int
	hover      int
	isDark     bool
}

func (m *tabShell) paintSnapshot() tabPaintSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := tabPaintSnapshot{active: m.active, hover: m.hoverTab, isDark: m.isDark}
	for _, t := range m.tabs {
		s.ids = append(s.ids, t.profile.ID)
		s.names = append(s.names, t.profile.Name)
		s.badges = append(s.badges, m.badges[t.profile.ID])
		s.hibernated = append(s.hibernated, t.hibernated)
	}
	return s
}

// indexAt maps a strip x coordinate to a tab index, using the same scaled
// geometry as the painter (-1 when the point is past the last tab).
func (m *tabShell) indexAt(x, clientW int32, n int) int {
	if n <= 0 {
		return -1
	}
	mt := m.metrics()
	w := (clientW - 2*mt.pad) / int32(n)
	if w > mt.maxW {
		w = mt.maxW
	}
	if w < mt.minW {
		w = mt.minW
	}
	w += mt.gap
	idx := (x - mt.pad) / w
	if idx < 0 || idx >= int32(n) {
		return -1
	}
	return int(idx)
}

func tabInitial(name string) string {
	for _, r := range strings.TrimSpace(name) {
		return strings.ToUpper(string(r))
	}
	return "?"
}

func (m *tabShell) paintStrip() {
	var ps tabPaint
	hdc, _, _ := tabBeginPaint.Call(m.strip, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	var area tabRect
	tabGetClientRect.Call(m.strip, uintptr(unsafe.Pointer(&area)))
	snap := m.paintSnapshot()

	bg, tabActive, tabHover, text, muted, accent, avatarText := m.stripColors()
	brush, _, _ := tabCreateBrush.Call(bg)
	var rc tabRect = area
	tabFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), brush)
	tabDeleteObject.Call(brush)

	n := len(snap.ids)
	mt := m.metrics()
	tabW := m.sc(190)
	if n > 0 {
		tabW = (area.Right - area.Left - 2*mt.pad) / int32(n)
		if tabW > mt.maxW {
			tabW = mt.maxW
		}
		if tabW < mt.minW {
			tabW = mt.minW
		}
	}
	tabSelectObject.Call(hdc, m.fontText)
	// Hibernated tabs are drawn "asleep": hollow avatar ring, muted name.
	// Created once per paint and released at the end.
	hibPen, _, _ := tabCreatePen.Call(0, 0, muted) // PS_SOLID, 1px, muted
	hollowBrush, _, _ := tabGetStockObject.Call(5) // HOLLOW_BRUSH
	for i := 0; i < n; i++ {
		hib := snap.hibernated[i]
		x := mt.pad + int32(i)*(tabW+mt.gap)
		y := mt.top
		w := tabW
		h := mt.stripH - 2*mt.top
		fill := tabActive
		if i != snap.active {
			fill = bg
			if i == snap.hover {
				fill = tabHover
			}
		}
		b, _, _ := tabCreateBrush.Call(fill)
		old, _, _ := tabSelectObject.Call(hdc, b)
		tabRoundRect.Call(hdc, uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+h),
			uintptr(mt.radius), uintptr(mt.radius))
		tabSelectObject.Call(hdc, old)
		tabDeleteObject.Call(b)
		if i == snap.active {
			ab, _, _ := tabCreateBrush.Call(accent)
			var bar tabRect
			bar.Left, bar.Top, bar.Right, bar.Bottom = x+mt.barIn, y, x+w-mt.barIn, y+mt.barH
			tabFillRect.Call(hdc, uintptr(unsafe.Pointer(&bar)), ab)
			tabDeleteObject.Call(ab)
		}
		// avatar dot with initial
		dotTop := y + mt.top
		tabSetBkMode.Call(hdc, 1)
		if hib {
			oldPen, _, _ := tabSelectObject.Call(hdc, hibPen)
			oldBr, _, _ := tabSelectObject.Call(hdc, hollowBrush)
			tabEllipse.Call(hdc, uintptr(x+mt.barIn), uintptr(dotTop),
				uintptr(x+mt.barIn+mt.dot), uintptr(dotTop+mt.dot))
			tabSelectObject.Call(hdc, oldPen)
			tabSelectObject.Call(hdc, oldBr)
			tabSetTextColor.Call(hdc, muted)
		} else {
			ab2, _, _ := tabCreateBrush.Call(accent)
			old2, _, _ := tabSelectObject.Call(hdc, ab2)
			tabEllipse.Call(hdc, uintptr(x+mt.barIn), uintptr(dotTop),
				uintptr(x+mt.barIn+mt.dot), uintptr(dotTop+mt.dot))
			tabSelectObject.Call(hdc, old2)
			tabDeleteObject.Call(ab2)
			tabSetTextColor.Call(hdc, avatarText)
		}
		tabSelectObject.Call(hdc, m.fontBold)
		init := tabInitial(snap.names[i])
		ip, _ := windows.UTF16PtrFromString(init)
		var ar tabRect
		ar.Left, ar.Top, ar.Right, ar.Bottom = x+mt.barIn, dotTop, x+mt.barIn+mt.dot, dotTop+mt.dot
		tabDrawText.Call(hdc, uintptr(unsafe.Pointer(ip)), uintptr(^uintptr(0)),
			uintptr(unsafe.Pointer(&ar)), uintptr(tabDTCenter|tabDTSingle|tabDTVCenter|tabDTNoPrefix))
		// name
		tabSetTextColor.Call(hdc, text)
		if i == snap.active {
			tabSelectObject.Call(hdc, m.fontBold)
		} else {
			tabSelectObject.Call(hdc, m.fontText)
		}
		if hib {
			tabSetTextColor.Call(hdc, muted)
		}
		np, _ := windows.UTF16PtrFromString(snap.names[i])
		var nr tabRect
		nr.Left, nr.Top, nr.Right, nr.Bottom = x+mt.nameX, y, x+w-mt.barIn, y+h
		tabDrawText.Call(hdc, uintptr(unsafe.Pointer(np)), uintptr(^uintptr(0)),
			uintptr(unsafe.Pointer(&nr)), uintptr(tabDTLeft|tabDTSingle|tabDTVCenter|tabDTNoPrefix|tabDTEllipsis))
		// unread pill
		if snap.badges[i] > 0 {
			label := strconv.Itoa(snap.badges[i])
			if snap.badges[i] > 99 {
				label = "99+"
			}
			bx := x + w - mt.badgeW - mt.gap
			by := y + mt.badgeY
			pb, _, _ := tabCreateBrush.Call(accent)
			old3, _, _ := tabSelectObject.Call(hdc, pb)
			tabRoundRect.Call(hdc, uintptr(bx), uintptr(by), uintptr(bx+mt.badgeW), uintptr(by+mt.badgeH),
				uintptr(mt.radius/2), uintptr(mt.radius/2))
			tabSelectObject.Call(hdc, old3)
			tabDeleteObject.Call(pb)
			tabSetTextColor.Call(hdc, avatarText)
			tabSelectObject.Call(hdc, m.fontText)
			lp, _ := windows.UTF16PtrFromString(label)
			var lr tabRect
			lr.Left, lr.Top, lr.Right, lr.Bottom = bx, by, bx+mt.badgeW, by+mt.badgeH
			tabDrawText.Call(hdc, uintptr(unsafe.Pointer(lp)), uintptr(^uintptr(0)),
				uintptr(unsafe.Pointer(&lr)), uintptr(tabDTCenter|tabDTSingle|tabDTVCenter|tabDTNoPrefix))
		}
	}
	// Build-version tag at the strip's right edge ("v" + appVersion + ", muted"): every
	// screenshot or bug report then shows which build is running. Tab
	// geometry is deliberately untouched (hit-testing keeps working exactly
	// as before), so the tag only paints into leftover empty space and is
	// skipped when tabs fill the strip.
	tabEnd := mt.pad + int32(n)*(tabW+mt.gap)
	if area.Right-area.Left-tabEnd >= mt.verW+mt.pad {
		vp, _ := windows.UTF16PtrFromString("v" + appVersion)
		var vr tabRect
		vr.Left, vr.Top, vr.Right, vr.Bottom = area.Right-mt.pad-mt.verW, 0, area.Right-mt.pad, mt.stripH
		tabSetTextColor.Call(hdc, muted)
		tabSelectObject.Call(hdc, m.fontText)
		tabDrawText.Call(hdc, uintptr(unsafe.Pointer(vp)), uintptr(^uintptr(0)),
			uintptr(unsafe.Pointer(&vr)), uintptr(tabDTRight|tabDTSingle|tabDTVCenter|tabDTNoPrefix))
	}
	if hibPen != 0 {
		tabDeleteObject.Call(hibPen)
	}
	tabEndPaint.Call(m.strip, uintptr(unsafe.Pointer(&ps)))
}

func stripWndProc(hwnd, m_, wp, lp uintptr) uintptr {
	m := theShell
	if m == nil {
		r, _, _ := tabDefWindowProc.Call(hwnd, m_, wp, lp)
		return r
	}
	switch m_ {
	case tabWMPaint:
		m.paintStrip()
		return 0
	case tabWMEraseBk:
		return 1
	case tabWMLButton:
		lx := int32(lp & 0xFFFF)
		var r tabRect
		tabGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		m.mu.Lock()
		n := len(m.tabs)
		m.mu.Unlock()
		if idx := m.indexAt(lx, r.Right-r.Left, n); idx >= 0 {
			m.activateTab(idx)
		}
		return 0
	case tabWMMouseMove:
		lx := int32(lp & 0xFFFF)
		var r tabRect
		tabGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		m.mu.Lock()
		n := len(m.tabs)
		m.mu.Unlock()
		hover := m.indexAt(lx, r.Right-r.Left, n)
		changed := false
		m.mu.Lock()
		if hover != m.hoverTab {
			m.hoverTab = hover
			changed = true
		}
		m.mu.Unlock()
		if changed {
			tabInvalidateRect.Call(hwnd, 0, 0)
		}
		var tm tabTrackMouseEvt
		tm.Size = uint32(unsafe.Sizeof(tm))
		tm.Flags = 2 // TME_LEAVE
		tm.HwndTrack = hwnd
		tabTrackMouse.Call(uintptr(unsafe.Pointer(&tm)))
		return 0
	case tabWMMouseLeave:
		m.mu.Lock()
		if m.hoverTab != -1 {
			m.hoverTab = -1
			m.mu.Unlock()
			tabInvalidateRect.Call(hwnd, 0, 0)
		} else {
			m.mu.Unlock()
		}
		return 0
	}
	r, _, _ := tabDefWindowProc.Call(hwnd, m_, wp, lp)
	return r
}

// sweepHidden ages background tabs: first suspend them (freeze the page,
// save CPU/battery), then hibernate them (close the engine and give the
// memory back). The active tab is never touched.
func (m *tabShell) sweepHidden() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, t := range m.tabs {
		if i == m.active || t.hibernated || t.view == nil {
			continue
		}
		if t.hiddenSince.IsZero() {
			t.hiddenSince = now
			continue
		}
		idle := now.Sub(t.hiddenSince)
		if shouldHibernate(idle) && t.booted {
			// Only a fully loaded tab is worth keeping warm; once it has been
			// idle this long, trading a reload for ~400 MB back is the point.
			m.hibernateLocked(t)
			continue
		}
		if !t.booted || t.suspendedByUs {
			continue
		}
		if shouldSuspend(idle) {
			if t.view.Suspend() {
				t.suspendedByUs = true
			}
		}
	}
	m.recycleActiveLocked(now)
}

// setProfileBusyState receives the page's activity reports (download in
// flight, document preview open, recycler probe answers). The page pushes
// state changes only, so this is cheap. Installed via
// tabSetProfileBusyState by buildView.
func (m *tabShell) setProfileBusyState(profileID, kind string, on bool) {
	m.busyMu.Lock()
	defer m.busyMu.Unlock()
	switch kind {
	case "download", "docmodal":
		st := m.busy[profileID]
		if kind == "download" {
			if on {
				st.downloads++
			} else {
				st.downloads--
			}
		} else {
			if on {
				st.docmodal++
			} else {
				st.docmodal--
			}
		}
		if st.downloads < 0 {
			st.downloads = 0
		}
		if st.docmodal < 0 {
			st.docmodal = 0
		}
		m.busy[profileID] = st
	case "query-docmodal":
		// Answer to a recycler busy probe: on = a document preview is
		// genuinely open (keep waiting); off = the counter was stale, clear
		// it so later cycles don't keep re-probing.
		if !on {
			st := m.busy[profileID]
			st.docmodal = 0
			m.busy[profileID] = st
		}
		m.busyMu.Unlock()
		// Off-pump: the answer arrives inside the old controller's message
		// callback, so the rebuild must run from the next loop iteration,
		// never from that callback frame.
		go m.recycleActiveProbeDone(on)
		m.busyMu.Lock()
	}
}

// recycleActiveLocked rebuilds the active tab's engine in place once its
// page has run long enough for WhatsApp Web's renderer baseline to bloat
// (~0.7–1.0 GB; see tabRecycleAge). The rebuild is the same controller
// close/reopen the hibernate wake path uses: session on disk, login kept,
// strip position unchanged. Deferred while the page reports downloads or
// an open document preview — a pending rebuild is retried on later sweeps.
func (m *tabShell) recycleActiveLocked(now time.Time) {
	if tabRecycleAge <= 0 || m.active < 0 || m.active >= len(m.tabs) {
		return
	}
	t := m.tabs[m.active]
	if t == nil || t.hibernated || t.view == nil || !t.booted {
		return
	}
	if t.activeSince.IsZero() {
		// Pre-recycler entry or a rebuild that never reached
		// NavigationCompleted: seed the reference point and decide later.
		t.activeSince = now
		return
	}
	age := now.Sub(t.activeSince)
	if age < tabRecycleAge {
		return
	}
	if !t.recycleProbedAt.IsZero() && now.Sub(t.recycleProbedAt) < tabRecycleProbeEvery {
		return
	}
	t.recycleProbedAt = now
	m.busyMu.Lock()
	busy := m.busy[t.profile.ID]
	m.busyMu.Unlock()
	if busy.downloads > 0 {
		return
	}
	if busy.docmodal > 0 {
		// The counters may be stale (modal closed without a report — e.g.
		// the page reloaded mid-preview). Ask the page; off-pump eval is the
		// same best-effort pattern focusTabView uses for a waking tab. A
		// "modal gone" answer clears the counter and rebuilds via
		// recycleActiveProbeDone; no answer just retries on a later sweep.
		v := t.view
		go v.Eval("if (window.__waBusyProbe) { window.__waBusyProbe('docmodal'); }")
		return
	}
	m.rebuildActiveTabLocked(t)
}

// recycleActiveProbeDone applies the page's answer to a busy probe.
func (m *tabShell) recycleActiveProbeDone(blocked bool) {
	tabCallOnPump(m, func() struct{} {
		m.mu.Lock()
		defer m.mu.Unlock()
		if blocked || m.active < 0 || m.active >= len(m.tabs) {
			return struct{}{}
		}
		t := m.tabs[m.active]
		if t == nil || t.hibernated || t.view == nil || !t.booted {
			return struct{}{}
		}
		m.rebuildActiveTabLocked(t)
		return struct{}{}
	})
} // rebuildActiveTabLocked swaps the active tab's engine for a fresh one,
// keeping the tab entry, zoom and strip position. Caller holds m.mu.
func (m *tabShell) rebuildActiveTabLocked(t *tabEntry) {
	p := t.profile
	z := t.zoom
	age := time.Since(t.activeSince)
	m.destroyView(t)
	t.profile = p
	t.zoom = z
	if err := m.buildView(t); err != nil {
		// Leave the entry intact; the wake path (activateLocked) rebuilds on
		// the next click, exactly like a failed wake from hibernation.
		m.toastActiveLocked("⚠️ Could not refresh \"" + p.Name + "\". Click the tab to retry.")
		return
	}
	m.layoutViewsLocked()
	if t.view != nil {
		t.view.Focus()
	}
	m.refreshChromeLocked()
	cacheDebugLog("recycler: rebuilt active renderer for %s (age %.0f min)", p.ID, age.Minutes())
}

func shellWndProc(hwnd, m_, wp, lp uintptr) uintptr {
	m := theShell
	if m == nil {
		r, _, _ := tabDefWindowProc.Call(hwnd, m_, wp, lp)
		return r
	}
	switch m_ {
	case tabWMApp:
		m.drainPump()
		return 0
	case tabWMSize:
		if wp == tabSizeMinimized {
			if t := m.activeEntry(); t != nil && t.view != nil {
				if t.view.Suspend() {
					m.mu.Lock()
					t.suspendedByUs = true
					m.mu.Unlock()
				}
			}
			return 0
		}
		if t := m.activeEntry(); t != nil && t.view != nil {
			_ = t.view.Resume()
			m.mu.Lock()
			t.suspendedByUs = false
			m.mu.Unlock()
		}
		m.layoutOuter()
		m.layoutViews()
		return 0
	case tabWMMove:
		if t := m.activeEntry(); t != nil && t.view != nil {
			_ = t.view.chromium.NotifyParentWindowPositionChanged()
		}
		return 0
	case tabWMActivate:
		if wp != 0 {
			if t := m.activeEntry(); t != nil {
				m.mu.Lock()
				susp := t.suspendedByUs
				m.mu.Unlock()
				m.focusTabView(t, susp)
			}
		}
		return 0
	case tabWMDPICHanged:
		// PerMonitorV2: Windows reports the new DPI and suggests a frame that
		// keeps the window's physical size. We scale the frame by the same
		// ratio ourselves rather than dereferencing the suggested RECT pointer.
		newDPI := int(wp & 0xFFFF)
		if newDPI < 48 || newDPI > 480 {
			newDPI = tabBaseDPI
		}
		old := m.dpi
		if old < 48 {
			old = tabBaseDPI
		}
		m.dpi = newDPI
		m.rebuildFonts()
		if newDPI != old {
			var r tabRect
			if ret, _, _ := tabGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret != 0 {
				ratio := float64(newDPI) / float64(old)
				w := int32(math.Round(float64(r.Right-r.Left) * ratio))
				h := int32(math.Round(float64(r.Bottom-r.Top) * ratio))
				tabSetWindowPos.Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(w), uintptr(h),
					uintptr(tabSWPNoZOrder|tabSWPNoActivate))
			}
		}
		m.layoutOuter()
		m.layoutViews()
		tabInvalidateRect.Call(m.strip, 0, 1)
		return 0
	case tabWMClose:
		tabDestroyWindow.Call(hwnd)
		return 0
	case tabWMDestroy:
		tabKillTimer.Call(hwnd, tabTimerSweep)
		saveWindowState(getSettingsBaseDir(), hwnd)
		removeTrayIcon()
		tabPostQuit.Call(0)
		return 0
	case tabWMTimer:
		if wp == tabTimerSweep {
			m.sweepHidden()
		}
		return 0
	case tabWMTabGoTo:
		// Second launch: wParam carries the profile index+1 (0 = focus only).
		// Both processes read the same registry, so the order agrees. Tabs
		// missing locally are opened on demand.
		if wp > 0 {
			m.activateOrOpenIndex(int(wp) - 1)
		}
		tabShowWindow.Call(hwnd, tabSWRestore)
		tabSetFgWindow.Call(hwnd)
		return 1
	}
	r, _, _ := tabDefWindowProc.Call(hwnd, m_, wp, lp)
	return r
}

func (m *tabShell) layoutOuter() {
	var r tabRect
	tabGetClientRect.Call(m.hwnd, uintptr(unsafe.Pointer(&r)))
	tabSetWindowPos.Call(m.strip, 0, 0, 0, uintptr(r.Right-r.Left), uintptr(m.metrics().stripH),
		uintptr(tabSWPNoZOrder|tabSWPNoActivate))
}

// ---------------------------------------------------------------------------
// Bootstrap.
// ---------------------------------------------------------------------------

func tabMakeFont(weight int32, height int32) uintptr {
	name, _ := windows.UTF16PtrFromString("Segoe UI")
	h, _, _ := tabCreateFont.Call(
		uintptr(height), 0, 0, 0, uintptr(weight),
		0, 0, 0, 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(name)))
	return h
}

func tabLoadAppIcon(hinst uintptr) uintptr {
	// Mirror the vendor window creation: resource icon id 2, shared.
	icon, _, _ := procLoadImage.Call(hinst, uintptr(2), uintptr(1), 0, 0, uintptr(0x0040|0x8000))
	return icon
}

func runTabbedShell() {
	profiles := listProfiles()
	for _, p := range profiles {
		dir := profileDirFor(p)
		cacheDebugLog("startup: pid=%d profile=%s", os.Getpid(), dir)
		roots, subs := ebCacheArgs(dir)
		enforceDiskCacheCapSync(roots, subs, "startup")
	}

	exe, _ := os.Executable()
	m := &tabShell{
		active:         -1,
		badges:         map[string]int{},
		busy:           map[string]tabBusyState{},
		hoverTab:       -1,
		executablePath: exe,
		iconFullPath:   ensureAppIconFile(getSettingsBaseDir()),
		isDark:         true,
	}
	tid, _, _ := tabGetThreadID.Call()
	m.pumpThread = uint32(tid)
	theShell = m
	tabSwitchHook = m.activateByName

	s := loadSettings()
	if s.Theme == "light" {
		m.isDark = false
	} else if s.Theme == "system" {
		m.isDark = isWindowsSystemDarkTheme()
	}

	hinst, _, _ := tabGetModuleHandle.Call(0)
	icon := tabLoadAppIcon(hinst)
	cursor, _, _ := tabLoadCursor.Call(0, 32512)

	regClass := func(name, bgMode string, proc uintptr) {
		_ = bgMode
		cn, _ := windows.UTF16PtrFromString(name)
		wc := tabWndClass{
			Style:     3,
			WndProc:   proc,
			Instance:  hinst,
			Icon:      icon,
			Cursor:    cursor,
			ClassName: uintptr(unsafe.Pointer(cn)),
			IconSm:    icon,
		}
		wc.Size = uint32(unsafe.Sizeof(wc))
		tabRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	}
	regClass(tabClassOuter, "", windows.NewCallback(shellWndProc))
	regClass(tabClassStrip, "", windows.NewCallback(stripWndProc))

	cn, _ := windows.UTF16PtrFromString(tabClassOuter)
	ttl, _ := windows.UTF16PtrFromString(windowTitle)
	// Size the first window for the monitor's DPI: with PerMonitorV2 the
	// manifest stops Windows from bitmap-stretching us, so 1100x750 would look
	// cramped on a 150%+ display.
	m.dpi = windowDPI(0)
	winW := int(m.sc(windowWidth))
	winH := int(m.sc(windowHeight))
	sw, _, _ := tabGetSysMetrics.Call(0)
	sh, _, _ := tabGetSysMetrics.Call(1)
	px := (int(sw) - winW) / 2
	py := (int(sh) - winH) / 2
	if px < 0 {
		px = 0
	}
	if py < 0 {
		py = 0
	}
	hwnd, _, _ := tabCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(cn)),
		uintptr(unsafe.Pointer(ttl)),
		uintptr(tabWSOverlappedWindow),
		uintptr(px), uintptr(py), uintptr(winW), uintptr(winH),
		0, 0, hinst, 0)
	if hwnd == 0 {
		log.Fatalln("Gagal membuat window utama tab")
	}
	m.hwnd = hwnd
	m.dpi = windowDPI(hwnd)
	configureWindow(hwnd)

	sn, _ := windows.UTF16PtrFromString(tabClassStrip)
	strip, _, _ := tabCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(sn)),
		0,
		uintptr(tabWSChild|tabWSVisible),
		0, 0, 100, uintptr(m.metrics().stripH), hwnd, 0, hinst, 0)
	if strip == 0 {
		log.Fatalln("Gagal membuat strip tab")
	}
	m.strip = strip

	m.rebuildFonts()

	if state := loadWindowStateForMonitor(getSettingsBaseDir(), windowMonitorKey(hwnd)); state != nil {
		procMoveWindow.Call(hwnd, uintptr(int32(state.X)), uintptr(int32(state.Y)), uintptr(int32(state.Width)), uintptr(int32(state.Height)), 1)
	}

	initialID := getActiveProfile().ID
	initialIdx := 0
	for _, p := range profiles {
		entry := &tabEntry{profile: p, zoom: getZoom()}
		if p.ID == initialID {
			// Load only the account we are starting on. The other tabs stay
			// hibernated and wake on first click (same code path), so having
			// N accounts registered no longer costs N engines at startup.
			if err := m.buildView(entry); err != nil {
				log.Fatalf("Gagal membuka profil %s: %v", p.Name, err)
			}
			initialIdx = len(m.tabs)
		} else {
			entry.hibernated = true
			entry.hiddenSince = time.Now()
		}
		m.tabs = append(m.tabs, entry)
	}
	m.mu.Lock()
	m.activateLocked(initialIdx)
	m.mu.Unlock()

	installTrayIcon(hwnd)
	trayEvalDispatch = func(script string) {
		m.evalActive(script)
	}

	go guardGoroutine("update-ticker", func() {
		checkAndNotifyUpdate := func() {
			info, err := checkForUpdate(appVersion)
			if err == nil && info != nil && info.Available {
				m.dispatch(func() {
					t := m.activeEntry()
					if t == nil || t.view == nil {
						return
					}
					script := fmt.Sprintf("if (window.showUpdateBanner) { window.showUpdateBanner(%q, %q, %q); }",
						info.LatestVersion, info.ReleaseTitle, info.DownloadURL)
					t.view.Eval(script)
				})
			}
		}
		time.Sleep(5 * time.Second)
		checkAndNotifyUpdate()
		ticker := time.NewTicker(4 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			checkAndNotifyUpdate()
		}
	})

	m.layoutOuter()
	m.layoutViews()
	tabShowWindow.Call(hwnd, tabSWShow)
	tabUpdateWindow.Call(hwnd, 0)
	if t := m.activeEntry(); t != nil && t.view != nil {
		t.view.Focus()
	}
	tabSetTimer.Call(hwnd, tabTimerSweep, uintptr(uint32(tabSweepEvery/time.Millisecond)), 0)

	var msg tabMsg
	for {
		r, _, _ := tabGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if msg.Message == tabWMApp {
			m.drainPump()
			continue
		}
		tabTranslate.Call(uintptr(unsafe.Pointer(&msg)))
		tabDispatchMsg.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// ---------------------------------------------------------------------------
// Single instance: one global mutex; second launches route via WM_COPYDATA.
// ---------------------------------------------------------------------------

func claimTabbedSingleInstance() bool {
	namePtr, _ := syscall.UTF16PtrFromString(mutexName)
	handle, _, err := procCreateMutex.Call(0, 1, uintptr(unsafe.Pointer(namePtr)))
	if err == windows.ERROR_ALREADY_EXISTS {
		focusTabbedShell(profileFlagPending)
		return false
	}
	// Legacy bridge: a pre-tabs instance may hold this profile's own mutex.
	// Defer to its window instead of running two copies of one profile.
	me := getActiveProfile().ID
	for _, legacy := range []string{mutexName + "-" + me, legacyMutexName + "-" + me} {
		lp, _ := syscall.UTF16PtrFromString(legacy)
		lh, _, lerr := procCreateMutex.Call(0, 1, uintptr(unsafe.Pointer(lp)))
		if lerr == windows.ERROR_ALREADY_EXISTS {
			procCloseHandle.Call(lh)
			focusExistingProfileWindow()
			return false
		}
		procCloseHandle.Call(lh)
	}
	_ = handle // held for the lifetime of the process
	return true
}

// findShellByEnum locates the shell outer window by enumerating top-level
// windows instead of trusting FindWindowW.
func findShellByEnum() uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var cls [64]uint16
		n, _, _ := tabGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&cls[0])), 64)
		if n > 0 && windows.UTF16ToString(cls[:n]) == tabClassOuter {
			found = hwnd
			return 0
		}
		return 1
	})
	tabEnumWindows.Call(cb, 0)
	return found
}

// profile travels as a registry index (both processes read the same file,
// so the order agrees), then the window is restored. Pointer-free by
// design — no shared-memory parsing on either side.
func focusTabbedShell(profileName string) {
	cn, _ := syscall.UTF16PtrFromString(tabClassOuter)
	var hwnd uintptr
	for i := 0; i < 100; i++ {
		h, _, _ := tabFindWindow.Call(0, uintptr(unsafe.Pointer(cn)))
		if h != 0 {
			hwnd = h
			break
		}
		// FindWindowW can miss top-level windows in some session states;
		// fall back to enumerating (proven reliable here).
		hwnd = findShellByEnum()
		if hwnd != 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if hwnd == 0 {
		return
	}
	var idx int
	if strings.TrimSpace(profileName) != "" {
		idx = -1
		if p := findProfileByIDOrName(loadProfileRegistry(), profileName); p != nil {
			for i, q := range listProfiles() {
				if q.ID == p.ID {
					idx = i
					break
				}
			}
		}
	}
	// wParam 0 = focus only (unknown profile or none requested).
	tabSendMessage.Call(hwnd, tabWMTabGoTo, uintptr(idx+1), 0)
	tabShowWindow.Call(hwnd, tabSWRestore)
	tabSetFgWindow.Call(hwnd)
}
