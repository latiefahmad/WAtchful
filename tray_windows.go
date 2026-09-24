//go:build windows

package main

// Tray icon for WAtchful on Windows (previously the app had none).
// Left-click shows the window; right-click opens a context menu with the
// profile switcher (each entry relaunches the app into that profile) plus a
// few common actions. The menu is rebuilt on every open so newly created
// profiles appear without a restart.

import (
	"syscall"
	"unsafe"
)

var (
	shell32Tray             = syscall.NewLazyDLL("shell32.dll")
	user32Tray              = syscall.NewLazyDLL("user32.dll")
	comctl32                = syscall.NewLazyDLL("comctl32.dll")
	procShellNotifyIcon     = shell32Tray.NewProc("Shell_NotifyIconW")
	procCreatePopupMenuW    = user32Tray.NewProc("CreatePopupMenu")
	procAppendMenuWW        = user32Tray.NewProc("AppendMenuW")
	procTrackPopupMenuW     = user32Tray.NewProc("TrackPopupMenu")
	procPostMessageW        = user32Tray.NewProc("PostMessageW")
	procDestroyMenuW        = user32Tray.NewProc("DestroyMenu")
	procSetForeground       = user32Tray.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32Tray.NewProc("GetCursorPos")
	procDefSubclassProc     = comctl32.NewProc("DefSubclassProc")
	procSetWindowSubclassCC = comctl32.NewProc("SetWindowSubclass")
	procLoadImage           = user32Tray.NewProc("LoadImageW")
)

const (
	nimAdd       = 0
	nimModify    = 1
	nimDelete    = 2
	nifMessage   = 1
	nifIcon      = 2
	nifTip       = 4
	nifShowTip   = 0x80
	nimCallback  = 0x0400 // WM_USER + 0: app-private window message range
	trayIconID   = 1
	trayMenuBase = 0x8100 // command IDs for tray menu items (menu-only range)

	tpmLeftButton  = 0x0000
	tpmRightButton = 0x0002
	tpmBottomAlign = 0x0020
	tpmRightAlign  = 0x0008
	tpmReturnCmd   = 0x0100
)

// MF_ menu flags (user32)
const (
	mfString    = 0x00000000
	mfSeparator = 0x00000800
	mfChecked   = 0x00000008
	mfDisabled  = 0x00000002
)

type notifyIconDataW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [128]uint16
	// Union { uVersion; szInfoTitle[128]; dwInfoFlags } shares one offset in
	// the C struct; modeling them as sequential fields would grow the struct
	// by 8 bytes and make Shell_NotifyIconW misread the layout.
	Union        [256]byte
	GuidItem     GUID
	HBalloonIcon uintptr
}

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type pointL struct {
	X, Y int32
}

var trayHwnd uintptr
var trayHicon uintptr
var trayAdded bool

// trayMenuRows snapshots the profile display names at the time the menu was
// built, so a click always maps to the row the user actually saw even if the
// registry changed in between.
var trayMenuRows []string

// showWindowFromTray restores/focuses the main window (left-click behavior).
func showWindowFromTray() {
	if trayHwnd == 0 {
		return
	}
	procShowNormal.Call(trayHwnd, 9) // SW_RESTORE
	procSetFgWindow.Call(trayHwnd)
}

// appendTrayMenuRow appends one MF_STRING entry; id 0 marks a disabled row.
func appendTrayMenuRow(menu uintptr, label string, id uintptr, flags uintptr) {
	mf := uintptr(mfString)
	if flags&mfDisabled != 0 {
		mf |= mfDisabled
	}
	if flags&mfChecked != 0 {
		mf |= mfChecked
	}
	labelPtr, _ := syscall.UTF16PtrFromString(label)
	procAppendMenuWW.Call(menu, mf, id, uintptr(unsafe.Pointer(labelPtr)))
}

func appendTrayMenuSep(menu uintptr) {
	procAppendMenuWW.Call(menu, uintptr(mfSeparator), 0, 0)
}

// buildTrayMenu composes the context menu fresh on every open: header, the
// profile switcher (current profile checkmarked/disabled), and quick actions.
func buildTrayMenu() uintptr {
	menu, _, _ := procCreatePopupMenuW.Call()
	if menu == 0 {
		return 0
	}

	appendTrayMenuRow(menu, windowTitleForProfile(), 0, mfDisabled)
	appendTrayMenuSep(menu)

	// Profile switcher: one entry per registered profile.
	joined, activeName := profileNamesForMenu()
	rows := splitProfileMenuRows(joined)
	trayMenuRows = rows
	id := uintptr(trayMenuBase)
	for _, name := range rows {
		flags := uintptr(0)
		label := name
		if name == activeName {
			flags |= mfChecked | mfDisabled
			label = name + "  ✓"
		}
		appendTrayMenuRow(menu, label, id, flags)
		id++
	}
	if len(rows) > 0 {
		appendTrayMenuSep(menu)
	}

	appendTrayMenuRow(menu, "Show Window", trayMenuBase+900, 0)
	appendTrayMenuRow(menu, "Settings", trayMenuBase+901, 0)
	appendTrayMenuRow(menu, "Direct Chat...", trayMenuBase+902, 0)
	return menu
}

func splitProfileMenuRows(joined string) []string {
	rows := []string{}
	cur := []rune{}
	for _, r := range joined {
		if r == '\n' {
			if len(cur) > 0 {
				rows = append(rows, string(cur))
			}
			cur = nil
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		rows = append(rows, string(cur))
	}
	return rows
}

// handleTrayMenuCommand maps a chosen menu ID to an action. Profile entries
// relaunch the app (which exits this process); the other actions run inline.
func handleTrayMenuCommand(id uintptr) {
	const (
		cmdShow       = trayMenuBase + 900
		cmdSettings   = trayMenuBase + 901
		cmdDirectChat = trayMenuBase + 902
	)
	if id >= trayMenuBase && id < trayMenuBase+900 {
		idx := int(id - trayMenuBase)
		if idx < len(trayMenuRows) {
			switchToProfileByName(trayMenuRows[idx])
		}
		return
	}
	switch id {
	case cmdShow:
		showWindowFromTray()
	case cmdSettings:
		openSettingsFromTray()
	case cmdDirectChat:
		openDirectChatFromTray()
	}
}

// openSettingsFromTray mirrors the macOS menuSettings bridge: surface the
// in-page Control Center modal and make sure the window is visible first.
func openSettingsFromTray() {
	showWindowFromTray()
	evalOnMainWebView(`if (window.showSettingsModal) { window.showSettingsModal(); }`)
}

// openDirectChatFromTray opens the Direct Chat modal the same way.
func openDirectChatFromTray() {
	showWindowFromTray()
	evalOnMainWebView(`if (window.openDirectChatModal) { window.openDirectChatModal(); }`)
}

var trayEvalDispatch func(string)

// evalOnMainWebView runs JS on the active tab via the dispatch hook
// installed by the tab shell on every tab switch.
func evalOnMainWebView(script string) {
	if trayEvalDispatch != nil {
		trayEvalDispatch(script)
	}
}

// trayWndProc handles the tray callback message and the tracked menu result.
// NOTE: the SUBCLASSPROC signature carries six arguments (hwnd, msg, wParam,
// lParam, uIdSubclass, dwRefData). The Go wrapper must declare all six —
// syscall.NewCallback pops the stack per the declared count, and a mismatch
// corrupts it on stdcall.
func trayWndProc(hwnd, msg, wParam, lParam, idSubclass, refData uintptr) uintptr {
	if msg == nimCallback {
		switch lParam & 0xFFFF {
		case 0x0202: // WM_LBUTTONUP
			showWindowFromTray()
		case 0x0205: // WM_RBUTTONUP
			showTrayContextMenu()
		}
	}
	if msg == trayMenuBase+0x0300 { // tray command after TrackPopupMenu
		handleTrayMenuCommand(wParam)
	}
	r, _, _ := procDefSubclassProc.Call(hwnd, msg, wParam, lParam, idSubclass, refData)
	return r
}

// showTrayContextMenu rebuilds and tracks the menu, then routes the chosen ID
// through PostMessage so the handler runs outside the menu's message loop.
// TPM_RETURNCMD makes TrackPopupMenu return the selected command ID directly
// (0 when dismissed), which is what handleTrayMenuCommand expects.
func showTrayContextMenu() {
	menu := buildTrayMenu()
	if menu == 0 {
		return
	}
	defer procDestroyMenuW.Call(menu)
	var pt pointL
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForeground.Call(trayHwnd)
	const tpmFlags = tpmRightButton | tpmBottomAlign | tpmRightAlign | tpmReturnCmd
	ret, _, _ := procTrackPopupMenuW.Call(menu, uintptr(tpmFlags), uintptr(pt.X), uintptr(pt.Y), 0, trayHwnd, 0)
	if ret == 0 {
		return
	}
	procPostMessageW.Call(trayHwnd, trayMenuBase+0x0300, ret, 0)
}

// installTrayIcon registers the notification icon and its window hook.
func installTrayIcon(hwnd uintptr) {
	trayHwnd = hwnd
	// LoadImageW(hinst=NULL, MAKEINTRESOURCE(2), IMAGE_ICON, 0, 0, LR_SHARED):
	// icon resource id 2 is the same one the webview window uses.
	if iconPtr, _, _ := procLoadImage.Call(0, uintptr(2), uintptr(1), 0, 0, uintptr(0x00000080)); iconPtr != 0 {
		trayHicon = iconPtr
	}

	// comctl32.SetWindowSubclass: keeps webview.go's own WndProc working
	// while letting us observe the tray callback message.
	if procSetWindowSubclassCC.Find() == nil {
		subclassCB := syscall.NewCallback(trayWndProc)
		procSetWindowSubclassCC.Call(hwnd, subclassCB, 1, 0)
	} else {
		return // cannot observe messages; tray would be a dead icon
	}

	nid := notifyIconDataW{
		CbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		HWnd:             hwnd,
		UID:              trayIconID,
		UFlags:           nifMessage | nifIcon | nifTip | nifShowTip,
		UCallbackMessage: nimCallback,
		HIcon:            trayHicon,
	}
	tip := syscall.StringToUTF16(windowTitleForProfile())
	copy(nid.SzTip[:], tip)
	procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	trayAdded = true
}

// removeTrayIcon unregisters the icon at shutdown.
func removeTrayIcon() {
	if !trayAdded || trayHwnd == 0 {
		return
	}
	nid := notifyIconDataW{
		CbSize: uint32(unsafe.Sizeof(notifyIconDataW{})),
		HWnd:   trayHwnd,
		UID:    trayIconID,
	}
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	trayAdded = false
}

// updateTrayTooltip keeps the tray hover text aligned with the profile.
func updateTrayTooltip(title string) {
	if !trayAdded || trayHwnd == 0 {
		return
	}
	nid := notifyIconDataW{
		CbSize: uint32(unsafe.Sizeof(notifyIconDataW{})),
		HWnd:   trayHwnd,
		UID:    trayIconID,
		UFlags: nifTip | nifShowTip,
	}
	tip := syscall.StringToUTF16(title)
	copy(nid.SzTip[:], tip)
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}
