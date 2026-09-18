package edge

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type iCoreWebView2_3Vtbl struct {
	iCoreWebView2_2Vtbl
	TrySuspend                          ComProc
	Resume                              ComProc
	GetIsSuspended                      ComProc
	SetVirtualHostNameToFolderMapping   ComProc
	ClearVirtualHostNameToFolderMapping ComProc
}

type ICoreWebView2_3 struct {
	vtbl *iCoreWebView2_3Vtbl
}

type iCoreWebView2TrySuspendCompletedHandlerVtbl struct {
	_IUnknownVtbl
	Invoke ComProc
}

type ICoreWebView2TrySuspendCompletedHandler struct {
	vtbl *iCoreWebView2TrySuspendCompletedHandlerVtbl
	impl iCoreWebView2TrySuspendCompletedHandlerImpl
}

type iCoreWebView2TrySuspendCompletedHandlerImpl interface {
	_IUnknownImpl
	TrySuspendCompleted(errorCode uintptr, isSuccessful uintptr) uintptr
}

func trySuspendQueryInterface(this *ICoreWebView2TrySuspendCompletedHandler, refiid, object uintptr) uintptr {
	return this.impl.QueryInterface(refiid, object)
}

func trySuspendAddRef(this *ICoreWebView2TrySuspendCompletedHandler) uintptr {
	return this.impl.AddRef()
}

func trySuspendRelease(this *ICoreWebView2TrySuspendCompletedHandler) uintptr {
	return this.impl.Release()
}

func trySuspendInvoke(this *ICoreWebView2TrySuspendCompletedHandler, errorCode, isSuccessful uintptr) uintptr {
	return this.impl.TrySuspendCompleted(errorCode, isSuccessful)
}

var iCoreWebView2TrySuspendCompletedHandlerFn = iCoreWebView2TrySuspendCompletedHandlerVtbl{
	_IUnknownVtbl{
		NewComProc(trySuspendQueryInterface),
		NewComProc(trySuspendAddRef),
		NewComProc(trySuspendRelease),
	},
	NewComProc(trySuspendInvoke),
}

func newICoreWebView2TrySuspendCompletedHandler(impl iCoreWebView2TrySuspendCompletedHandlerImpl) *ICoreWebView2TrySuspendCompletedHandler {
	return &ICoreWebView2TrySuspendCompletedHandler{
		vtbl: &iCoreWebView2TrySuspendCompletedHandlerFn,
		impl: impl,
	}
}

func (i *ICoreWebView2_3) SetVirtualHostNameToFolderMapping(hostName, folderPath string, accessKind COREWEBVIEW2_HOST_RESOURCE_ACCESS_KIND) error {
	_hostName, err := windows.UTF16PtrFromString(hostName)
	if err != nil {
		return err
	}

	_folderPath, err := windows.UTF16PtrFromString(folderPath)
	if err != nil {
		return err
	}

	_, _, err = i.vtbl.SetVirtualHostNameToFolderMapping.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(_hostName)),
		uintptr(unsafe.Pointer(_folderPath)),
		uintptr(accessKind),
	)
	if err != windows.ERROR_SUCCESS {
		return err
	}

	return nil
}

func (i *ICoreWebView2) GetICoreWebView2_3() *ICoreWebView2_3 {
	var result *ICoreWebView2_3

	iidICoreWebView2_3 := NewGUID("{A0D6DF20-3B92-416D-AA0C-437A9C727857}")
	_, _, _ = i.vtbl.QueryInterface.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(iidICoreWebView2_3)),
		uintptr(unsafe.Pointer(&result)))

	return result
}

func (e *Chromium) GetICoreWebView2_3() *ICoreWebView2_3 {
	return e.webview.GetICoreWebView2_3()
}
