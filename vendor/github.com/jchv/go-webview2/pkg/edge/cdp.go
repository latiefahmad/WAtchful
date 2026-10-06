//go:build windows
// +build windows

package edge

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// cdpHandler implements ICoreWebView2CallDevToolsProtocolMethodCompletedHandler
// (IID 5c4889f0-5ef6-4c5a-952c-d8f1b92d0574) for CallCDP.
// WAtchful addition.
type cdpHandler struct {
	vtbl *cdpHandlerVtbl
	cb   func(hr uintptr, result string)
}

type cdpHandlerVtbl struct {
	QueryInterface ComProc
	AddRef         ComProc
	Release        ComProc
	Invoke         ComProc
}

// cdpLive roots every pending handler until WebView2 invokes it. Both the
// Bind path and CDP completions run on the single WebView2 UI thread, so
// no locking is required.
var cdpLive = map[*cdpHandler]struct{}{}

func cdpHandlerQueryInterface(this *cdpHandler, _refiid, object uintptr) uintptr {
	if object != 0 {
		*(*uintptr)(unsafe.Pointer(object)) = uintptr(unsafe.Pointer(this))
	}
	return 0
}

func cdpHandlerAddRef(this *cdpHandler) uintptr { return 2 }

func cdpHandlerRelease(this *cdpHandler) uintptr { return 1 }

func cdpHandlerInvoke(this *cdpHandler, hr uintptr, result *uint16) uintptr {
	s := ""
	if result != nil {
		s = windows.UTF16PtrToString(result)
	}
	delete(cdpLive, this)
	cb := this.cb
	this.cb = nil
	if cb != nil {
		cb(hr, s)
	}
	return 0
}

// CallCDP runs a Chrome DevTools Protocol method (for example
// "Input.insertText") on this WebView. cb receives the raw HRESULT and the
// CDP result JSON; it runs later, on the WebView2 UI thread. The trusted
// input events CDP dispatches are the only ones WhatsApp Web's Lexical
// composer accepts (synthetic DOM edits get reverted). WAtchful addition.
func (e *Chromium) CallCDP(method, params string, cb func(hr uintptr, result string)) error {
	if e == nil || e.webview == nil {
		return fmt.Errorf("webview2 controller not initialized")
	}
	_method, err := windows.UTF16PtrFromString(method)
	if err != nil {
		return err
	}
	_params, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return err
	}
	h := &cdpHandler{cb: cb}
	h.vtbl = &cdpHandlerVtbl{
		QueryInterface: NewComProc(cdpHandlerQueryInterface),
		AddRef:         NewComProc(cdpHandlerAddRef),
		Release:        NewComProc(cdpHandlerRelease),
		Invoke:         NewComProc(cdpHandlerInvoke),
	}
	cdpLive[h] = struct{}{}
	hr, _, _ := e.webview.vtbl.CallDevToolsProtocolMethod.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(_method)),
		uintptr(unsafe.Pointer(_params)),
		uintptr(unsafe.Pointer(h)),
	)
	if int32(hr) < 0 {
		delete(cdpLive, h)
		return fmt.Errorf("CallDevToolsProtocolMethod failed: 0x%08X", uint32(hr))
	}
	return nil
}
