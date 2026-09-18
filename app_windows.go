//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"unsafe"

	"github.com/go-toast/toast"
	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

//go:embed icon.png
var embeddedIconPNG []byte

var (
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	user32                       = windows.NewLazySystemDLL("user32.dll")
	dwmapi                       = windows.NewLazySystemDLL("dwmapi.dll")
	procCreateMutex              = kernel32.NewProc("CreateMutexW")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procFreeConsole              = kernel32.NewProc("FreeConsole")
	procGetConsoleWindow         = kernel32.NewProc("GetConsoleWindow")
	procGetConsoleProcessList    = kernel32.NewProc("GetConsoleProcessList")
	procFindWindow               = user32.NewProc("FindWindowW")
	procSetFgWindow              = user32.NewProc("SetForegroundWindow")
	procShowNormal               = user32.NewProc("ShowWindow")
	procDwmSetAttr               = dwmapi.NewProc("DwmSetWindowAttribute")
	procGetWindowLong            = user32.NewProc("GetWindowLongW")
	procSetWindowLong            = user32.NewProc("SetWindowLongW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procMoveWindow               = user32.NewProc("MoveWindow")
	procMonitorFromPoint         = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfo           = user32.NewProc("GetMonitorInfoW")
	procCreateJobObject          = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procSetProcessWorkingSetSize = kernel32.NewProc("SetProcessWorkingSetSize")
	procGetCurrentProcess        = kernel32.NewProc("GetCurrentProcess")
	procCreateToolhelpSnapshot   = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = kernel32.NewProc("Process32FirstW")
	procProcess32Next            = kernel32.NewProc("Process32NextW")
	procOpenProcess              = kernel32.NewProc("OpenProcess")

	isAlwaysOnTopWin = false
)

const (
	mutexName       = "WAtchfulSingleInstanceMutex"
	legacyMutexName = "WhatsAppDesktopSingleInstanceMutex"
	userAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"

	// Win32 Window Styles for Dynamic Resizability
	GWL_STYLE        = 0xFFFFFFF0 // -16
	WS_THICKFRAME    = 0x00040000
	WS_MAXIMIZEBOX   = 0x00010000
	WS_MINIMIZEBOX   = 0x00020000
	SWP_FRAMECHANGED = 0x0020
	SWP_NOMOVE       = 0x0002
	SWP_NOSIZE       = 0x0001
	SWP_NOZORDER     = 0x0004

	// Window Z-Order constants for Always On Top
	HWND_TOPMOST   = ^uintptr(0) // -1
	HWND_NOTOPMOST = ^uintptr(1) // -2

	// DWM Window Attributes for Dark Theme
	DWMWA_USE_IMMERSIVE_DARK_MODE_BEFORE_20H1 = 19
	DWMWA_USE_IMMERSIVE_DARK_MODE             = 20
	DWMWA_CAPTION_COLOR                       = 35
	DWMWA_TEXT_COLOR                          = 36

	// Taskbar badge constants
	TBF_NOPROGRESS    = 0x00000000
	TBF_INDETERMINATE = 0x00000001
	TBF_NORMAL        = 0x00000002
	TBF_ERROR         = 0x00000004
	TBF_PAUSED        = 0x00000008
)

var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")

	taskbarList *ITaskbarList3
)

type ITaskbarList3 struct {
	vtbl *ITaskbarList3Vtbl
}

type ITaskbarList3Vtbl struct {
	QueryInterface       uintptr
	AddRef               uintptr
	Release              uintptr
	HrInit               uintptr
	AddTab               uintptr
	DeleteTab            uintptr
	ActivateTab          uintptr
	SetActiveAlt         uintptr
	MarkFullscreenWindow uintptr
	SetProgressValue     uintptr
	SetProgressState     uintptr
	RegisterTab          uintptr
	UnregisterTab        uintptr
	SetTabOrder          uintptr
	SetTabActivate       uintptr
	SetTabProperties     uintptr
}

func (t *ITaskbarList3) HrInit() error {
	ret, _, _ := syscall.Syscall(t.vtbl.HrInit, 1, uintptr(unsafe.Pointer(t)), 0, 0)
	if ret != 0 {
		return syscall.Errno(ret)
	}
	return nil
}

func (t *ITaskbarList3) SetProgressValue(hwnd uintptr, completed, total uint64) error {
	ret, _, _ := syscall.Syscall6(t.vtbl.SetProgressValue, 4, uintptr(unsafe.Pointer(t)), hwnd, uintptr(completed), uintptr(total), 0, 0)
	if ret != 0 {
		return syscall.Errno(ret)
	}
	return nil
}

func (t *ITaskbarList3) SetProgressState(hwnd uintptr, flags uint32) error {
	ret, _, _ := syscall.Syscall(t.vtbl.SetProgressState, 3, uintptr(unsafe.Pointer(t)), hwnd, uintptr(flags))
	if ret != 0 {
		return syscall.Errno(ret)
	}
	return nil
}

func initTaskbarList() error {
	if taskbarList != nil {
		return nil
	}
	// Initialize COM
	ret, _, _ := procCoInitializeEx.Call(0, 0) // COINIT_APARTMENTTHREADED
	if ret != 0 && ret != 0x80010106 {         // S_FALSE or RPC_E_CHANGED_MODE
		// Ignore if already initialized
	}

	// CLSID_TaskbarList = {56FDF344-FD6D-11d0-958A-006097C9A090}
	clsid := windows.GUID{Data1: 0x56FDF344, Data2: 0xFD6D, Data3: 0x11D0, Data4: [8]byte{0x95, 0x8A, 0x00, 0x60, 0x97, 0xC9, 0xA0, 0x90}}
	// IID_ITaskbarList3 = {EA1AFB91-9E28-4B86-90E9-9E9F8A5EEFAF}
	iid := windows.GUID{Data1: 0xEA1AFB91, Data2: 0x9E28, Data3: 0x4B86, Data4: [8]byte{0x90, 0xE9, 0x9E, 0x9F, 0x8A, 0x5E, 0xEF, 0xAF}}

	var unk unsafe.Pointer
	ret, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsid)),
		0,
		1, // CLSCTX_INPROC_SERVER
		uintptr(unsafe.Pointer(&iid)),
		uintptr(unsafe.Pointer(&unk)),
	)
	if ret != 0 {
		return fmt.Errorf("CoCreateInstance failed: 0x%x", ret)
	}

	taskbarList = (*ITaskbarList3)(unk)
	return taskbarList.HrInit()
}

func setTaskbarBadge(hwnd uintptr, count int) error {
	if err := initTaskbarList(); err != nil {
		return err
	}
	if count <= 0 {
		return taskbarList.SetProgressState(hwnd, TBF_NOPROGRESS)
	}
	if count > 99 {
		count = 99
	}
	// Set progress value to show count
	err := taskbarList.SetProgressValue(hwnd, uint64(count), 100)
	if err != nil {
		return err
	}
	return taskbarList.SetProgressState(hwnd, TBF_NORMAL)
}

func toggleAlwaysOnTop(hwnd uintptr) bool {
	isAlwaysOnTopWin = !isAlwaysOnTopWin
	target := uintptr(HWND_NOTOPMOST)
	if isAlwaysOnTopWin {
		target = uintptr(HWND_TOPMOST)
	}
	procSetWindowPos.Call(hwnd, target, 0, 0, 0, 0, uintptr(SWP_NOMOVE|SWP_NOSIZE))
	return isAlwaysOnTopWin
}

// hiddenCommand runs a console-subsystem helper (reg.exe, powershell, ...)
// without flashing a visible console window from this GUI process.
func hiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}
	return cmd
}

func toggleAutoStartWindows() bool {
	runKey := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	valName := "WAtchful"

	// Check if already configured
	err := hiddenCommand("reg", "query", runKey, "/v", valName).Run()
	if err == nil {
		// Key exists, remove it
		_ = hiddenCommand("reg", "delete", runKey, "/v", valName, "/f").Run()
		return false
	}

	// Key does not exist, add it
	execPath, err := os.Executable()
	if err != nil {
		return false
	}
	dataVal := fmt.Sprintf("\"%s\"", execPath)
	err = hiddenCommand("reg", "add", runKey, "/v", valName, "/t", "REG_SZ", "/d", dataVal, "/f").Run()
	return err == nil
}

const (
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE   = 0x00002000
	JOB_OBJECT_LIMIT_BREAKAWAY_OK        = 0x00000800
	JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK = 0x00001000
	JobObjectExtendedLimitInformation    = 9
)

type IO_COUNTERS struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type JOBOBJECT_BASIC_LIMIT_INFORMATION struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type JOBOBJECT_EXTENDED_LIMIT_INFORMATION struct {
	BasicLimitInformation JOBOBJECT_BASIC_LIMIT_INFORMATION
	IoInfo                IO_COUNTERS
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

func initWindowsProcessProtection() {
	// 1. Assign process to Job Object with KILL_ON_JOB_CLOSE and SILENT_BREAKAWAY_OK.
	// CRITICAL: JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK and JOB_OBJECT_LIMIT_BREAKAWAY_OK
	// are strictly required so that Chromium / WebView2 child processes (GPU, Renderer)
	// can create their own sandboxed Job Objects. Without breakaway, Chromium fails to
	// spawn renderer/GPU processes (ERROR_ACCESS_DENIED), resulting in a solid black screen.
	job, _, _ := procCreateJobObject.Call(0, 0)
	if job != 0 {
		var info JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK | JOB_OBJECT_LIMIT_BREAKAWAY_OK
		procSetInformationJobObject.Call(
			job,
			uintptr(JobObjectExtendedLimitInformation),
			uintptr(unsafe.Pointer(&info)),
			uintptr(unsafe.Sizeof(info)),
		)
		curProc, _, _ := procGetCurrentProcess.Call()
		procAssignProcessToJobObject.Call(job, curProc)
	}

	// 2. Configure safe WebView2 / Chromium engine arguments:
	// Use only reliable, well-tested flags. Keep --js-flags a single token
	// (no spaces/nested quotes) and do NOT disable window occlusion or GPU
	// shader cache, which cause black screen / compositor initialization
	// failures on Windows 10 & 11.
	browserArgs := []string{
		"--disable-features=Translate,MediaRouter,HardwareMediaKeyHandling,MediaSessionService",
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-component-extensions-with-background-pages",
		"--disable-domain-reliability",
		"--disable-sync",
		// Exposes window.gc() so the page's visibilitychange handler can
		// actually force a JS heap collection when hidden (without this
		// flag that call is a no-op and the heap only grows).
		"--js-flags=--expose-gc",
	}
	_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", strings.Join(browserArgs, " "))
}

// processEntry mirrors the Win32 PROCESSENTRY32W layout. Field offsets are
// resolved with unsafe.Offsetof at the call site, so the walk stays correct
// on both 32- and 64-bit builds.
type processEntry struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

// procIdentity is the syscall-free view of one process used for filtering.
type procIdentity struct {
	pid  uint32
	ppid uint32
	exe  string
}

// childWebView2PIDs returns the PIDs of live msedgewebview2.exe processes
// parented by selfPID. Pure (no syscalls) so it stays unit-testable; only
// direct children are ever matched, never unrelated Edge instances.
func childWebView2PIDs(procs []procIdentity, selfPID uint32) []uint32 {
	var out []uint32
	for _, p := range procs {
		if p.pid == 0 || p.pid == selfPID || p.ppid != selfPID {
			continue
		}
		if !strings.EqualFold(p.exe, "msedgewebview2.exe") {
			continue
		}
		out = append(out, p.pid)
	}
	return out
}

func listChildWebView2PIDs() []uint32 {
	const TH32CS_SNAPPROCESS = 0x00000002
	snap, _, _ := procCreateToolhelpSnapshot.Call(uintptr(TH32CS_SNAPPROCESS), 0)
	if snap == ^uintptr(0) { // INVALID_HANDLE_VALUE
		return nil
	}
	defer procCloseHandle.Call(snap)
	self := uint32(os.Getpid())
	var procs []procIdentity
	var entry processEntry
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		procs = append(procs, procIdentity{
			pid:  entry.ProcessID,
			ppid: entry.ParentProcessID,
			exe:  windows.UTF16ToString(entry.ExeFile[:]),
		})
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return childWebView2PIDs(procs, self)
}

// trimWebView2WorkingSets drops every WebView2 child process's working set to
// its minimum via SetProcessWorkingSetSize(-1, -1). Trimmed pages move to the
// standby list and fault back transparently on next use — no process is
// suspended, terminated, or otherwise disturbed. Every failure (snapshot,
// open, quota) is ignored: the worst case is a no-op. Returns how many
// children were actually trimmed, for debug logging.
func trimWebView2WorkingSets() int {
	const (
		PROCESS_SET_QUOTA                 = 0x00000100
		PROCESS_QUERY_LIMITED_INFORMATION = 0x00001000
	)
	trimmed := 0
	for _, pid := range listChildWebView2PIDs() {
		h, _, _ := procOpenProcess.Call(
			uintptr(PROCESS_SET_QUOTA|PROCESS_QUERY_LIMITED_INFORMATION), 0, uintptr(pid))
		if h == 0 {
			continue
		}
		r, _, _ := procSetProcessWorkingSetSize.Call(h, ^uintptr(0), ^uintptr(0))
		procCloseHandle.Call(h)
		if r != 0 {
			trimmed++
		}
	}
	return trimmed
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

func isWindowsSystemDarkTheme() bool {
	advapi32 := windows.NewLazySystemDLL("advapi32.dll")
	procRegOpenKeyExW := advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW := advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")

	const HKEY_CURRENT_USER = uintptr(0x80000001)
	const KEY_READ = uintptr(0x20019)

	subKey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	var hKey uintptr
	ret, _, _ := procRegOpenKeyExW.Call(HKEY_CURRENT_USER, uintptr(unsafe.Pointer(subKey)), 0, KEY_READ, uintptr(unsafe.Pointer(&hKey)))
	if ret != 0 {
		return true // default dark
	}
	defer procRegCloseKey.Call(hKey)

	valName, _ := syscall.UTF16PtrFromString("AppsUseLightTheme")
	var valType uint32
	var valData uint32
	valSize := uint32(unsafe.Sizeof(valData))

	ret, _, _ = procRegQueryValueExW.Call(hKey, uintptr(unsafe.Pointer(valName)), 0, uintptr(unsafe.Pointer(&valType)), uintptr(unsafe.Pointer(&valData)), uintptr(unsafe.Pointer(&valSize)))
	if ret != 0 {
		return true
	}
	return valData == 0 // 0 = dark, 1 = light
}

func applyNativeThemeWin(hwnd uintptr, theme string) {
	isDark := false
	if theme == "system" {
		isDark = isWindowsSystemDarkTheme()
	} else {
		isDark = (theme == "dark")
	}

	darkMode := int32(0)
	captionColor := uint32(0x00F5F2F0) // WhatsApp Light Header: RGB(240, 242, 245) -> 0x00BBGGRR = 0x00F5F2F0
	textColor := uint32(0x00211B11)    // WhatsApp Dark Text: RGB(17, 27, 33) -> 0x00BBGGRR = 0x00211B11

	if isDark {
		darkMode = 1
		captionColor = 0x00211B11 // WhatsApp Dark Header: RGB(17, 27, 33) -> 0x00BBGGRR = 0x00211B11
		textColor = 0x00FFFFFF    // White text
	}

	procDwmSetAttr.Call(
		hwnd,
		uintptr(DWMWA_USE_IMMERSIVE_DARK_MODE),
		uintptr(unsafe.Pointer(&darkMode)),
		unsafe.Sizeof(darkMode),
	)
	procDwmSetAttr.Call(
		hwnd,
		uintptr(DWMWA_USE_IMMERSIVE_DARK_MODE_BEFORE_20H1),
		uintptr(unsafe.Pointer(&darkMode)),
		unsafe.Sizeof(darkMode),
	)
	procDwmSetAttr.Call(
		hwnd,
		uintptr(DWMWA_CAPTION_COLOR),
		uintptr(unsafe.Pointer(&captionColor)),
		unsafe.Sizeof(captionColor),
	)
	procDwmSetAttr.Call(
		hwnd,
		uintptr(DWMWA_TEXT_COLOR),
		uintptr(unsafe.Pointer(&textColor)),
		unsafe.Sizeof(textColor),
	)
}

// hideTransientConsoleWindow detaches the app from a console window it
// inherited from a double-click launch of a console-subsystem build. It is a
// safety net for locally built binaries; the release build already links with
// -H windowsgui so no console is allocated in the first place.
//
// Safety rule: GetConsoleProcessList must report this process ALONE in the
// console (no cmd.exe parent, no go test runner), otherwise the call is a
// no-op — freeing a shared console would kill the user's shell or the test
// output.
func hideTransientConsoleWindow() {
	const ourProcessOnly = 1
	var buf [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == ourProcessOnly {
		procFreeConsole.Call()
	}
}

// focusExistingProfileWindow brings this profile's own window to the front.
// FindWindowW matches the exact title, so look for the composed per-profile
// title (the base title alone would miss "WAtchful — Work").
func focusExistingProfileWindow() {
	titlePtr, _ := syscall.UTF16PtrFromString(windowTitleForProfile())
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd != 0 {
		procShowNormal.Call(hwnd, 9) // SW_RESTORE
		procSetFgWindow.Call(hwnd)
	}
}

func getUserDataDir() string {
	// Multi-profile: return the active profile's own user-data folder so the
	// WebView2 session (cookies, IndexedDB, service worker) stays isolated
	// per account. The Default profile keeps the legacy <base>/UserData path.
	dir := profileDirFor(getActiveProfile())
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func ensureAppIconFile(dir string) string {
	iconPath := filepath.Join(dir, "app_icon.png")
	if _, err := os.Stat(iconPath); os.IsNotExist(err) && len(embeddedIconPNG) > 0 {
		_ = os.WriteFile(iconPath, embeddedIconPNG, 0644)
	}
	return iconPath
}

func showNativeNotification(title, message, iconPath, exePath string) {
	notification := toast.Notification{
		AppID:               "WAtchful",
		Title:               title,
		Message:             message,
		Icon:                iconPath,
		ActivationType:      "protocol",
		ActivationArguments: exePath,
	}
	_ = notification.Push()
}

func configureWindow(hwnd uintptr) {
	s := loadSettings()
	applyNativeThemeWin(hwnd, s.Theme)

	// Ensure sizing border and maximize/minimize buttons are enabled
	gwlStyle := uintptr(GWL_STYLE)
	style, _, _ := procGetWindowLong.Call(hwnd, gwlStyle)
	style |= WS_THICKFRAME | WS_MAXIMIZEBOX | WS_MINIMIZEBOX
	procSetWindowLong.Call(hwnd, gwlStyle, style)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, uintptr(SWP_NOMOVE|SWP_NOSIZE|SWP_NOZORDER|SWP_FRAMECHANGED))
}

// monitorInfo mirrors the Win32 MONITORINFOEXW layout (wide name at the end).
type monitorInfo struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
	SzDevice  [32]uint16
}

// windowMonitorKey identifies the monitor the window's top-left corner sits
// on. Windows assigns stable device names (\\.\DISPLAY1, ...) per adapter
// output, which is what we key per-monitor frames by.
func windowMonitorKey(hwnd uintptr) string {
	var r RECT
	if ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return ""
	}
	// POINT{x,y} packed as a single uintptr for the MonitorFromPoint call.
	pt := uintptr(uint32(r.Left)) | uintptr(uint32(uint32(r.Top)))<<32
	hmon, _, _ := procMonitorFromPoint.Call(pt, 2 /* MONITOR_DEFAULTTONEAREST */)
	if hmon == 0 {
		return ""
	}
	var mi monitorInfo
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if ret, _, _ := procGetMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&mi))); ret == 0 {
		return ""
	}
	name := windows.UTF16ToString(mi.SzDevice[:])
	if name == "" {
		return ""
	}
	return name
}

// frameOnSomeMonitor verifies a saved frame still overlaps a connected
// monitor's area, so unplugging a display never strands the window.
func frameOnSomeMonitor(x, y, w, h float64) bool {
	left := int32(x)
	top := int32(y)
	right := int32(x + w)
	bottom := int32(y + h)
	// Probe the frame center; MONITOR_DEFAULTTONULL yields 0 when the point
	// is on no monitor at all (all displays unplugged/moved).
	cx := uintptr(uint32((left+right)/2)) | uintptr(uint32(uint32((top+bottom)/2)))<<32
	hmon, _, _ := procMonitorFromPoint.Call(cx, 0 /* MONITOR_DEFAULTTONULL */)
	if hmon == 0 {
		return false
	}
	var mi monitorInfo
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if ret, _, _ := procGetMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&mi))); ret == 0 {
		return false
	}
	interW := min32(right, mi.RcMonitor.Right) - max32(left, mi.RcMonitor.Left)
	interH := min32(bottom, mi.RcMonitor.Bottom) - max32(top, mi.RcMonitor.Top)
	return interW > 60 && interH > 40
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// loadWindowStateForMonitor returns the frame saved for the given monitor,
// falling back to the legacy single frame, rejecting off-screen frames.
func loadWindowStateForMonitor(dir, monitorKey string) *WindowState {
	path := filepath.Join(dir, "window_state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var state WindowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	var candidates []*WindowState
	if monitorKey != "" {
		if per, ok := state.Screens[monitorKey]; ok && per.Width >= 450 && per.Height >= 320 {
			c := per
			candidates = append(candidates, &c)
		}
	}
	if state.Width >= 450 && state.Height >= 320 {
		fallback := state
		fallback.Screens = nil
		candidates = append(candidates, &fallback)
	}
	for _, cand := range candidates {
		if frameOnSomeMonitor(cand.X, cand.Y, cand.Width, cand.Height) {
			return cand
		}
	}
	return nil
}

func saveWindowState(dir string, hwnd uintptr) {
	var r RECT
	ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return
	}
	w := r.Right - r.Left
	h := r.Bottom - r.Top
	if w < 450 || h < 320 {
		return
	}
	monitorKey := windowMonitorKey(hwnd)

	// Read-modify-write so other monitors' frames survive.
	state := WindowState{
		X: float64(r.Left), Y: float64(r.Top), Width: float64(w), Height: float64(h),
	}
	if data, err := os.ReadFile(filepath.Join(dir, "window_state.json")); err == nil {
		var existing WindowState
		if json.Unmarshal(data, &existing) == nil {
			state.Screens = existing.Screens
		}
	}
	if state.Screens == nil {
		state.Screens = map[string]WindowState{}
	}
	if monitorKey != "" {
		if prev, ok := state.Screens[monitorKey]; ok &&
			prev.X == state.X && prev.Y == state.Y &&
			prev.Width == state.Width && prev.Height == state.Height {
			return // unchanged on this monitor — skip the disk write
		}
		state.Screens[monitorKey] = WindowState{X: state.X, Y: state.Y, Width: state.Width, Height: state.Height}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Join(dir, "window_state.json"), data, 0644)
	}
}

// profileViewContext carries everything the per-view script bridges need.
// Phase 0: exactly one view exists (runApp below). Phase 1 (tabbed shell)
// creates one context per profile tab and reuses setupProfileBindings
// unchanged for every tab.
type profileViewContext struct {
	profile        Profile
	userDataDir    string
	executablePath string
	iconFullPath   string
	hwnd           uintptr
}

// setupProfileBindings installs every per-view script bridge. It takes the
// view plus its context instead of closing over runApp locals, so Phase 1
// can call it once per profile tab with no changes here.
func setupProfileBindings(w webview2.WebView, ctx *profileViewContext) {
	_ = w.Bind("saveWindowStateNative", func(width, height int) {
		// Tabbed shell: one shared outer window, so the frame lives in the
		// global data home instead of a per-profile folder.
		saveWindowState(getSettingsBaseDir(), ctx.hwnd)
	})

	// Bind native notification bridge
	_ = w.Bind("sendNativeNotification", func(title, body string) {
		if len(listProfiles()) > 1 {
			title = ctx.profile.Name + " — " + title
		}
		go showNativeNotification(title, body, ctx.iconFullPath, ctx.executablePath)
	})

	_ = w.Bind("releaseMemoryNative", func() {
		// Note: do NOT call w.Suspend() here. The webview2 vendor library already
		// suspends/resumes the WebView2 renderer symmetrically on real minimize/restore
		// (see its WM_SIZE handling). Calling Suspend() manually just because the page
		// is merely hidden (e.g. briefly alt-tabbing without minimizing) has no matching
		// Resume() call anywhere, which previously left the WebView hidden/frozen. Just
		// trim reclaimable memory instead, which is always safe to call.
		debug.FreeOSMemory()
		curProc, _, _ := procGetCurrentProcess.Call()
		procSetProcessWorkingSetSize.Call(curProc, ^uintptr(0), ^uintptr(0))
		// The Go process is only ~2 MB; the gigabytes live in the WebView2
		// child processes (renderer/GPU/utility). Trim their working sets too:
		// pages freed by the page-side window.gc() leave the working set
		// immediately instead of waiting for OS paging. Non-destructive —
		// trimmed pages fault back from standby on next use.
		trimmed := trimWebView2WorkingSets()
		cacheDebugLog("release-memory: trimmed %d WebView2 child working sets", trimmed)
		debugLogProcessStats("release-memory")
		enforceDiskCacheCapFrom(
			[]string{filepath.Join(ctx.userDataDir, "EBWebView")},
			[]string{
				filepath.Join(ctx.userDataDir, "EBWebView", "Default", "Cache"),
				filepath.Join(ctx.userDataDir, "EBWebView", "Default", "GPUCache"),
				filepath.Join(ctx.userDataDir, "EBWebView", "Default", "Code Cache"),
				filepath.Join(ctx.userDataDir, "EBWebView", "Default", "Service Worker"),
			},
			"minimize",
		)
	})

	// Renderer recycler gate: the page reports downloads in flight and open
	// document previews so the tab shell never rebuilds the engine under
	// them (the download would die with the old renderer). Rebinding per
	// rebuilt view keeps the tab routed to the right profile.
	tabSetProfileBusyState = func(profileID, kind string, on bool) {
		if s := theShell; s != nil {
			s.setProfileBusyState(profileID, kind, on)
		}
	}
	_ = w.Bind("setBusyStateNative", func(profileID, kind string, on bool) {
		if tabSetProfileBusyState != nil {
			tabSetProfileBusyState(profileID, kind, on)
		}
	})

	// Bind external link handler to open links in default Windows browser
	_ = w.Bind("openExternalLink", func(rawURL string) {
		if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
			go func() {
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
			}()
		}
	})

	// Bind Always on Top toggle
	_ = w.Bind("toggleAlwaysOnTopNative", func() bool {
		return toggleAlwaysOnTop(ctx.hwnd)
	})

	// Bind Auto-Start toggle
	_ = w.Bind("toggleAutoStartNative", func() bool {
		return toggleAutoStartWindows()
	})

	// Bind in-app auto updater
	_ = w.Bind("checkForUpdateNative", func(manual bool) UpdateInfo {
		info, err := checkForUpdate(appVersion)
		if err != nil {
			return UpdateInfo{CurrentVersion: appVersion, CheckError: err.Error()}
		}
		return *info
	})

	_ = w.Bind("startUpdateNative", func(downloadURL string) {
		go func() {
			_ = executeUpdate(w, downloadURL)
		}()
	})

	// Bind download, preview, and settings handlers
	_ = w.Bind("saveDownloadedFileNative", func(filename, dataURI string) string {
		path, err := saveDownloadedFile(filename, dataURI)
		if err != nil {
			return ""
		}
		return path
	})

	_ = w.Bind("previewDocumentNative", func(filename, dataURI string) string {
		path, err := previewDocument(filename, dataURI)
		if err != nil {
			return ""
		}
		return path
	})

	_ = w.Bind("openFileNative", func(filePath string) bool {
		return openFileInDefaultApp(filePath)
	})

	// Lazy-load SheetJS library for spreadsheet preview
	_ = w.Bind("loadXLSXLibraryNative", func() string {
		return xlsxLibJS
	})

	_ = w.Bind("getDownloadDirNative", func() string {
		s := loadSettings()
		return s.DownloadDir
	})

	_ = w.Bind("getOrganizeByMonthNative", func() bool {
		return loadSettings().OrganizeByMonth
	})

	_ = w.Bind("setOrganizeByMonthNative", func(on bool) bool {
		return setOrganizeByMonth(on)
	})

	_ = w.Bind("checkFileExistsNative", func(filename string) bool {
		return fileExistsInDownloadDir(filename)
	})

	_ = w.Bind("chooseDownloadDirNative", func() string {
		selected, err := chooseFolderDialog()
		if err != nil || selected == "" {
			return ""
		}
		s := loadSettings()
		s.DownloadDir = selected
		_ = saveSettings(s)
		return selected
	})

	_ = w.Bind("openDownloadDirNative", func() bool {
		s := loadSettings()
		_ = openFolderInFileManager(s.DownloadDir)
		return true
	})

	_ = w.Bind("resetDownloadDirNative", func() string {
		s := loadSettings()
		s.DownloadDir = getDefaultDownloadDir()
		_ = saveSettings(s)
		return s.DownloadDir
	})

	_ = w.Bind("getAppThemeNative", func() string {
		s := loadSettings()
		return s.Theme
	})

	_ = w.Bind("setAppThemeNative", func(theme string) string {
		saved := saveTheme(theme)
		applyNativeThemeWin(ctx.hwnd, saved)
		tabManagerSetTheme(saved)
		return saved
	})

	_ = w.Bind("getZoomNative", func() float64 {
		return getZoom()
	})
	_ = w.Bind("setZoomNative", func(zoom float64) float64 {
		return setZoom(zoom)
	})
	// Native page zoom: applies at the WebView2 controller level so the page
	// reflows exactly like a browser's Ctrl+scroll zoom. The CSS body-zoom
	// fallback (other platforms) leaves an uncovered gap on WhatsApp Web,
	// which is why Windows routes through the controller instead. The level
	// is a global display setting, so every tab follows it.
	_ = w.Bind("applyZoomNative", func(zoom float64) bool {
		z := clampZoom(zoom)
		if m := theShell; m != nil {
			m.applyZoomToAllTabs(z)
			return true
		}
		if zs, ok := w.(zoomSetter); ok {
			return zs.SetZoomFactor(z) == nil
		}
		return false
	})

	// Spell check bindings
	_ = w.Bind("getSpellCheckEnabledNative", func() bool {
		return getSpellCheckEnabled()
	})
	_ = w.Bind("setSpellCheckEnabledNative", func(enabled bool) bool {
		return setSpellCheckEnabled(enabled)
	})
	_ = w.Bind("getSpellCheckLangNative", func() string {
		return getSpellCheckLang()
	})
	_ = w.Bind("setSpellCheckLangNative", func(lang string) string {
		return setSpellCheckLang(lang)
	})
	_ = w.Bind("getBlurAvatarsNative", func() bool {
		return getBlurAvatars()
	})
	_ = w.Bind("setBlurAvatarsNative", func(on bool) bool {
		return setBlurAvatars(on)
	})

	// Multi-profile bindings (tabbed shell: in-process tab operations;
	// relaunch fallbacks cover the no-manager case, e.g. picker flows).
	_ = w.Bind("listProfilesNative", func() []ProfileInfo {
		return profileInfos(listProfiles(), true)
	})
	_ = w.Bind("switchProfileNative", func(name string) {
		if tabShellActive() {
			tabManagerActivateByName(name)
			return
		}
		if p := findProfileByIDOrName(loadProfileRegistry(), name); p != nil {
			touchProfileLastUsed(p.ID)
			relaunchWithProfile(p.Name)
		}
	})
	_ = w.Bind("createProfileNative", func(name string) {
		p, err := createProfile(name)
		if err != nil {
			tabManagerToast("⚠️ " + err.Error())
			return
		}
		touchProfileLastUsed(p.ID)
		if tabShellActive() {
			tabManagerOpenProfile(p)
			return
		}
		relaunchWithProfile(p.Name)
	})
	_ = w.Bind("deleteProfileNative", func(id string) bool {
		if !tabShellActive() {
			return deleteProfile(id) == nil
		}
		return tabManagerDeleteProfile(id)
	})
	_ = w.Bind("renameProfileNative", func(id, name string) bool {
		p, err := renameProfile(id, name)
		if err != nil {
			return false
		}
		if tabShellActive() {
			tabManagerRenameProfile(p.ID)
		}
		return true
	})
	_ = w.Bind("resetProfileNative", func(id string) bool {
		if !tabShellActive() {
			return resetProfileData(id) == nil
		}
		return tabManagerResetProfile(id)
	})
	_ = w.Bind("getPendingCrashNative", func() string {
		return pendingCrashReport()
	})
	_ = w.Bind("markCrashNotifiedNative", func() bool {
		return markCrashNotified()
	})

	// Taskbar badge binding (tabbed shell: per-tab counts are aggregated).
	_ = w.Bind("updateDockBadge", func(badge string) {
		count := 0
		if badge != "" {
			fmt.Sscanf(badge, "%d", &count)
		}
		if tabShellActive() {
			tabManagerReportBadge(ctx.profile.ID, count)
			return
		}
		_ = setTaskbarBadge(ctx.hwnd, count)
	})

	// Tab navigation bridges (tabbed shell only; guarded page-side too).
	_ = w.Bind("cycleTabNative", func(dir int) {
		tabManagerCycle(dir)
	})
	_ = w.Bind("activateTabNative", func(index int) {
		tabManagerActivateIndex(index)
	})
}

func runApp() {
	hideTransientConsoleWindow()
	cleanupOldWindowsBinary()
	initWindowsProcessProtection()

	// The picker's whole job is switching between running instances, so it
	// must open even while sessions are alive: it runs before the
	// single-instance check and gets its own tiny data dir
	// (profile folders may be locked by live WebView2 sessions).
	if isProfilePickerRequested() {
		runProfilePickerWindows(profilePickerDataDir())
		return
	}

	if !claimTabbedSingleInstance() {
		os.Exit(0)
	}
	runTabbedShell()
}

// runProfilePickerWindows hosts the standalone profile picker in a small
// WebView2 window. Every action either relaunches the app on the chosen
// profile or quits; the picker page itself never becomes a chat session.
func runProfilePickerWindows(userDataDir string) {
	opts := webview2.WebViewOptions{
		Window:    nil,
		Debug:     false,
		DataPath:  userDataDir, // picker only; chat sessions get their own dir
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle + " — Profiles",
			Width:  520,
			Height: 640,
			IconId: 2,
			Center: true,
		},
	}
	w := webview2.NewWithOptions(opts)
	if w == nil {
		log.Fatalln("Gagal inisialisasi WebView2 (profile picker)")
	}
	defer w.Destroy()

	pickerBindings(func(name string, f interface{}) error {
		return w.Bind(name, f)
	})

	w.SetHtml(pickerPageHTML())
	w.Run()
}
