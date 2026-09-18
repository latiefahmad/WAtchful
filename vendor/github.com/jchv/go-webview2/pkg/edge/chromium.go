//go:build windows
// +build windows

package edge

import (
	"encoding/binary"
	"errors"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/internal/w32"
	"golang.org/x/sys/windows"
)

var (
	zoomThunkMu     sync.Mutex
	zoomThunkAddr   uintptr
	zoomThunkTarget uintptr

	zoomKernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procZoomVirtualAlloc = zoomKernel32.NewProc("VirtualAlloc")
)

// zoomFactorThunk builds (once) a 17-byte executable stub that adapts the
// Windows x64 ABI for ICoreWebView2Controller::put_ZoomFactor(this, double):
//
//	movq xmm1, rdx      ; 66 48 0F 6E CA
//	movabs rax, target  ; 48 B8 <imm64>
//	jmp rax             ; FF E0
//
// Go's syscall can only fill integer registers (RCX/RDX/R8/R9), so the raw
// double bits arrive in RDX; the stub moves them into XMM1 where the callee
// reads them, then tail-jumps to the real vtbl entry (same stack frame).
func zoomFactorThunk(target uintptr) (uintptr, error) {
	zoomThunkMu.Lock()
	defer zoomThunkMu.Unlock()
	if zoomThunkAddr != 0 {
		if zoomThunkTarget != target {
			return 0, errors.New("zoom thunk target changed")
		}
		return zoomThunkAddr, nil
	}
	if target == 0 {
		return 0, errors.New("zoom thunk: empty target")
	}
	code := []byte{
		0x66, 0x48, 0x0F, 0x6E, 0xCA, // movq xmm1, rdx
		0x48, 0xB8, // movabs rax, imm64
		0, 0, 0, 0, 0, 0, 0, 0,
		0xFF, 0xE0, // jmp rax
	}
	binary.LittleEndian.PutUint64(code[7:], uint64(target))
	addr, _, callErr := procZoomVirtualAlloc.Call(0, uintptr(len(code)), 0x3000, 0x40)
	if addr == 0 {
		return 0, errors.New("zoom thunk: VirtualAlloc failed: " + callErr.Error())
	}
	// Copy the stub byte-by-byte: unsafe.Slice needs a newer language level
	// than this module declares.
	for i, b := range code {
		*(*byte)(unsafe.Pointer(addr + uintptr(i))) = b
	}
	zoomThunkAddr = addr
	zoomThunkTarget = target
	return addr, nil
}

func putZoomFactorThunked(target, this uintptr, zoom float64) uintptr {
	thunk, err := zoomFactorThunk(target)
	if err != nil {
		return 0x80004005 // E_FAIL
	}
	r1, _, _ := syscall.SyscallN(thunk, this, uintptr(math.Float64bits(zoom)))
	return r1
}

type Chromium struct {
	hwnd                  uintptr
	focusOnInit           bool
	controller            *ICoreWebView2Controller
	webview               *ICoreWebView2
	inited                uintptr
	envCompleted          *iCoreWebView2CreateCoreWebView2EnvironmentCompletedHandler
	controllerCompleted   *iCoreWebView2CreateCoreWebView2ControllerCompletedHandler
	webMessageReceived    *iCoreWebView2WebMessageReceivedEventHandler
	permissionRequested   *iCoreWebView2PermissionRequestedEventHandler
	webResourceRequested  *iCoreWebView2WebResourceRequestedEventHandler
	acceleratorKeyPressed *ICoreWebView2AcceleratorKeyPressedEventHandler
	navigationCompleted   *ICoreWebView2NavigationCompletedEventHandler
	trySuspendCompleted   *ICoreWebView2TrySuspendCompletedHandler
	suspended             uintptr

	environment *ICoreWebView2Environment

	// Settings
	DataPath string

	// permissions
	permissions      map[CoreWebView2PermissionKind]CoreWebView2PermissionState
	globalPermission *CoreWebView2PermissionState

	// Callbacks
	MessageCallback              func(string)
	WebResourceRequestedCallback func(request *ICoreWebView2WebResourceRequest, args *ICoreWebView2WebResourceRequestedEventArgs)
	NavigationCompletedCallback  func(sender *ICoreWebView2, args *ICoreWebView2NavigationCompletedEventArgs)
	AcceleratorKeyCallback       func(uint) bool
}

func NewChromium() *Chromium {
	e := &Chromium{}
	/*
	 All these handlers are passed to native code through syscalls with 'uintptr(unsafe.Pointer(handler))' and we know
	 that a pointer to those will be kept in the native code. Furthermore these handlers als contain pointer to other Go
	 structs like the vtable.
	 This violates the unsafe.Pointer rule '(4) Conversion of a Pointer to a uintptr when calling syscall.Syscall.' because
	 theres no guarantee that Go doesn't move these objects.
	 AFAIK currently the Go runtime doesn't move HEAP objects, so we should be safe with these handlers. But they don't
	 guarantee it, because in the future Go might use a compacting GC.
	 There's a proposal to add a runtime.Pin function, to prevent moving pinned objects, which would allow to easily fix
	 this issue by just pinning the handlers. The https://go-review.googlesource.com/c/go/+/367296/ should land in Go 1.19.
	*/
	e.envCompleted = newICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler(e)
	e.controllerCompleted = newICoreWebView2CreateCoreWebView2ControllerCompletedHandler(e)
	e.webMessageReceived = newICoreWebView2WebMessageReceivedEventHandler(e)
	e.permissionRequested = newICoreWebView2PermissionRequestedEventHandler(e)
	e.webResourceRequested = newICoreWebView2WebResourceRequestedEventHandler(e)
	e.acceleratorKeyPressed = newICoreWebView2AcceleratorKeyPressedEventHandler(e)
	e.navigationCompleted = newICoreWebView2NavigationCompletedEventHandler(e)
	e.trySuspendCompleted = newICoreWebView2TrySuspendCompletedHandler(e)
	e.permissions = make(map[CoreWebView2PermissionKind]CoreWebView2PermissionState)

	return e
}

func (e *Chromium) Embed(hwnd uintptr) bool {
	e.hwnd = hwnd

	dataPath := e.DataPath
	if dataPath == "" {
		currentExePath := make([]uint16, windows.MAX_PATH)
		_, err := windows.GetModuleFileName(windows.Handle(0), &currentExePath[0], windows.MAX_PATH)
		if err != nil {
			// What to do here?
			return false
		}
		currentExeName := filepath.Base(windows.UTF16ToString(currentExePath))
		dataPath = filepath.Join(os.Getenv("AppData"), currentExeName)
	}

	res, err := createCoreWebView2EnvironmentWithOptions(nil, windows.StringToUTF16Ptr(dataPath), 0, e.envCompleted)
	if err != nil {
		log.Printf("Error calling Webview2Loader: %v", err)
		return false
	} else if res != 0 {
		log.Printf("Result: %08x", res)
		return false
	}
	var msg w32.Msg
	for {
		if atomic.LoadUintptr(&e.inited) != 0 {
			break
		}
		r, _, _ := w32.User32GetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
		)
		if r == 0 {
			break
		}
		_, _, _ = w32.User32TranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = w32.User32DispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	e.Init("window.external={invoke:s=>window.chrome.webview.postMessage(s)}")
	return true
}

func (e *Chromium) Navigate(url string) {
	_, _, _ = e.webview.vtbl.Navigate.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(url))),
	)
}

func (e *Chromium) NavigateToString(htmlContent string) {
	_, _, _ = e.webview.vtbl.NavigateToString.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(htmlContent))),
	)
}

func (e *Chromium) Init(script string) {
	_, _, _ = e.webview.vtbl.AddScriptToExecuteOnDocumentCreated.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(script))),
		0,
	)
}

func (e *Chromium) Eval(script string) {
	_script, err := windows.UTF16PtrFromString(script)
	if err != nil {
		log.Fatal(err)
	}

	_, _, _ = e.webview.vtbl.ExecuteScript.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(_script)),
		0,
	)
}

func (e *Chromium) Show() error {
	return e.controller.PutIsVisible(true)
}

func (e *Chromium) Hide() error {
	return e.controller.PutIsVisible(false)
}

func (e *Chromium) Suspend() bool {
	if e.controller == nil || e.webview == nil {
		return false
	}
	if err := e.Hide(); err != nil {
		return false
	}
	webview3 := e.GetICoreWebView2_3()
	if webview3 == nil {
		_ = e.Show()
		return false
	}
	defer webview3.vtbl.Release.Call(uintptr(unsafe.Pointer(webview3)))
	atomic.StoreUintptr(&e.suspended, 1)
	hr, _, _ := webview3.vtbl.TrySuspend.Call(
		uintptr(unsafe.Pointer(webview3)),
		uintptr(unsafe.Pointer(e.trySuspendCompleted)),
	)
	if int32(hr) < 0 {
		atomic.StoreUintptr(&e.suspended, 0)
		_ = e.Show()
		return false
	}
	return true
}

func (e *Chromium) Resume() bool {
	if e.controller == nil || e.webview == nil {
		return false
	}
	if atomic.SwapUintptr(&e.suspended, 0) != 0 {
		webview3 := e.GetICoreWebView2_3()
		if webview3 != nil {
			defer webview3.vtbl.Release.Call(uintptr(unsafe.Pointer(webview3)))
			_, _, _ = webview3.vtbl.Resume.Call(uintptr(unsafe.Pointer(webview3)))
		}
	}
	_ = e.Show()
	return true
}

func (e *Chromium) TrySuspendCompleted(errorCode uintptr, isSuccessful uintptr) uintptr {
	if int32(errorCode) < 0 || isSuccessful == 0 {
		atomic.StoreUintptr(&e.suspended, 0)
	}
	return 0
}

// SetBoundsRect positions this controller's view at (x, y) with the given
// size, in parent client coordinates. WAtchful tabbed-shell addition:
// every tab shares one outer window and sits below the tab strip, so the
// full-client Resize() is not sufficient.
func (e *Chromium) SetBoundsRect(x, y, w, h int32) error {
	if e.controller == nil {
		return errors.New("webview2 controller not initialized")
	}
	return e.controller.PutBounds(w32.Rect{Left: x, Top: y, Right: x + w, Bottom: y + h})
}

// CloseController closes the underlying WebView2 controller, releasing its
// renderer and user-data directory locks. WAtchful tabbed-shell addition:
// needed to delete or reset a profile's data while the app keeps running.
func (e *Chromium) CloseController() error {
	if e.controller == nil {
		return errors.New("webview2 controller not initialized")
	}
	hr, _, _ := e.controller.vtbl.Close.Call(uintptr(unsafe.Pointer(e.controller)))
	if int32(hr) < 0 {
		return errors.New("failed to close webview2 controller")
	}
	return nil
}

// HostWindow returns the parent window handle this controller was embedded
// into. WAtchful tabbed-shell addition: lets the adapter satisfy the
// webview2.WebView interface without uintptr/unsafe.Pointer conversions in
// vetted application code.
func (e *Chromium) HostWindow() unsafe.Pointer {
	return unsafe.Pointer(e.hwnd)
}

// SetZoomFactor sets the page zoom factor (1.0 = 100%). This is the same
// mechanism a browser uses for Ctrl+scroll zoom: the page reflows at the new
// scale instead of being painted larger, so no uncovered background shows
// through (which is what CSS `zoom` on the body causes on WhatsApp Web).
// WAtchful addition.
func (e *Chromium) SetZoomFactor(zoom float64) error {
	if e.controller == nil {
		return errors.New("webview2 controller not initialized")
	}
	return e.controller.PutZoomFactor(zoom)
}

// ZoomFactor returns the current page zoom factor (1.0 = 100%).
// WAtchful addition.
func (e *Chromium) ZoomFactor() (float64, error) {
	if e.controller == nil {
		return 1, errors.New("webview2 controller not initialized")
	}
	return e.controller.GetZoomFactor()
}

func (e *Chromium) QueryInterface(_, _ uintptr) uintptr {
	return 0
}

func (e *Chromium) AddRef() uintptr {
	return 1
}

func (e *Chromium) Release() uintptr {
	return 1
}

func (e *Chromium) EnvironmentCompleted(res uintptr, env *ICoreWebView2Environment) uintptr {
	if int64(res) < 0 {
		log.Fatalf("Creating environment failed with %08x", res)
	}
	_, _, _ = env.vtbl.AddRef.Call(uintptr(unsafe.Pointer(env)))
	e.environment = env

	_, _, _ = env.vtbl.CreateCoreWebView2Controller.Call(
		uintptr(unsafe.Pointer(env)),
		e.hwnd,
		uintptr(unsafe.Pointer(e.controllerCompleted)),
	)
	return 0
}

func (e *Chromium) CreateCoreWebView2ControllerCompleted(res uintptr, controller *ICoreWebView2Controller) uintptr {
	if int64(res) < 0 {
		log.Fatalf("Creating controller failed with %08x", res)
	}
	_, _, _ = controller.vtbl.AddRef.Call(uintptr(unsafe.Pointer(controller)))
	e.controller = controller

	var token _EventRegistrationToken
	_, _, _ = controller.vtbl.GetCoreWebView2.Call(
		uintptr(unsafe.Pointer(controller)),
		uintptr(unsafe.Pointer(&e.webview)),
	)
	_, _, _ = e.webview.vtbl.AddRef.Call(
		uintptr(unsafe.Pointer(e.webview)),
	)
	_, _, _ = e.webview.vtbl.AddWebMessageReceived.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(e.webMessageReceived)),
		uintptr(unsafe.Pointer(&token)),
	)
	_, _, _ = e.webview.vtbl.AddPermissionRequested.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(e.permissionRequested)),
		uintptr(unsafe.Pointer(&token)),
	)
	_, _, _ = e.webview.vtbl.AddWebResourceRequested.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(e.webResourceRequested)),
		uintptr(unsafe.Pointer(&token)),
	)
	_, _, _ = e.webview.vtbl.AddNavigationCompleted.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(e.navigationCompleted)),
		uintptr(unsafe.Pointer(&token)),
	)

	_ = e.controller.AddAcceleratorKeyPressed(e.acceleratorKeyPressed, &token)

	atomic.StoreUintptr(&e.inited, 1)

	if e.focusOnInit {
		e.Focus()
	}

	return 0
}

func (e *Chromium) MessageReceived(sender *ICoreWebView2, args *iCoreWebView2WebMessageReceivedEventArgs) uintptr {
	var message *uint16
	_, _, _ = args.vtbl.TryGetWebMessageAsString.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(unsafe.Pointer(&message)),
	)
	if e.MessageCallback != nil {
		e.MessageCallback(w32.Utf16PtrToString(message))
	}
	_, _, _ = sender.vtbl.PostWebMessageAsString.Call(
		uintptr(unsafe.Pointer(sender)),
		uintptr(unsafe.Pointer(message)),
	)
	windows.CoTaskMemFree(unsafe.Pointer(message))
	return 0
}

func (e *Chromium) SetPermission(kind CoreWebView2PermissionKind, state CoreWebView2PermissionState) {
	e.permissions[kind] = state
}

func (e *Chromium) SetGlobalPermission(state CoreWebView2PermissionState) {
	e.globalPermission = &state
}

func (e *Chromium) PermissionRequested(_ *ICoreWebView2, args *iCoreWebView2PermissionRequestedEventArgs) uintptr {
	var kind CoreWebView2PermissionKind
	_, _, _ = args.vtbl.GetPermissionKind.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(kind),
	)
	var result CoreWebView2PermissionState
	if e.globalPermission != nil {
		result = *e.globalPermission
	} else {
		var ok bool
		result, ok = e.permissions[kind]
		if !ok {
			result = CoreWebView2PermissionStateDefault
		}
	}
	_, _, _ = args.vtbl.PutState.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(result),
	)
	return 0
}

func (e *Chromium) WebResourceRequested(sender *ICoreWebView2, args *ICoreWebView2WebResourceRequestedEventArgs) uintptr {
	req, err := args.GetRequest()
	if err != nil {
		log.Fatal(err)
	}
	if e.WebResourceRequestedCallback != nil {
		e.WebResourceRequestedCallback(req, args)
	}
	return 0
}

func (e *Chromium) AddWebResourceRequestedFilter(filter string, ctx COREWEBVIEW2_WEB_RESOURCE_CONTEXT) {
	err := e.webview.AddWebResourceRequestedFilter(filter, ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func (e *Chromium) Environment() *ICoreWebView2Environment {
	return e.environment
}

// AcceleratorKeyPressed is called when an accelerator key is pressed.
// If the AcceleratorKeyCallback method has been set, it will defer handling of the keypress
// to the callback. That callback returns a bool indicating if the event was handled.
func (e *Chromium) AcceleratorKeyPressed(sender *ICoreWebView2Controller, args *ICoreWebView2AcceleratorKeyPressedEventArgs) uintptr {
	if e.AcceleratorKeyCallback == nil {
		return 0
	}
	eventKind, _ := args.GetKeyEventKind()
	if eventKind == COREWEBVIEW2_KEY_EVENT_KIND_KEY_DOWN ||
		eventKind == COREWEBVIEW2_KEY_EVENT_KIND_SYSTEM_KEY_DOWN {
		virtualKey, _ := args.GetVirtualKey()
		status, _ := args.GetPhysicalKeyStatus()
		if !status.WasKeyDown {
			_ = args.PutHandled(e.AcceleratorKeyCallback(virtualKey))
			return 0
		}
	}
	_ = args.PutHandled(false)
	return 0
}

func (e *Chromium) GetSettings() (*ICoreWebViewSettings, error) {
	return e.webview.GetSettings()
}

func (e *Chromium) GetController() *ICoreWebView2Controller {
	return e.controller
}

func boolToInt(input bool) int {
	if input {
		return 1
	}
	return 0
}

func (e *Chromium) NavigationCompleted(sender *ICoreWebView2, args *ICoreWebView2NavigationCompletedEventArgs) uintptr {
	if e.NavigationCompletedCallback != nil {
		e.NavigationCompletedCallback(sender, args)
	}
	return 0
}

func (e *Chromium) NotifyParentWindowPositionChanged() error {
	//It looks like the wndproc function is called before the controller initialization is complete.
	//Because of this the controller is nil
	if e.controller == nil {
		return nil
	}
	return e.controller.NotifyParentWindowPositionChanged()
}

func (e *Chromium) Focus() {
	if e.controller == nil {
		e.focusOnInit = true
		return
	}
	_ = e.controller.MoveFocus(COREWEBVIEW2_MOVE_FOCUS_REASON_PROGRAMMATIC)
}
