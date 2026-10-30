//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// windowClass is the main window's class, set through Wails' documented
// Windows option WindowClassName, so the window can be found by it. The
// profile gives it (product.WindowClass).
var windowClass = product.WindowClass

var (
	iconUser32           = windows.NewLazySystemDLL("user32.dll")
	iconShell32          = windows.NewLazySystemDLL("shell32.dll")
	procFindWindowExW    = iconUser32.NewProc("FindWindowExW")
	procSendMessageIconW = iconUser32.NewProc("SendMessageW")
	procExtractIconsExW  = iconShell32.NewProc("ExtractIconExW")
)

// setWindowIcon gives the main window the program's own icon, the frying
// pan, for its title bar and the taskbar. Wails loads its window icon from
// a resource ID that Dinah's resource file does not use, so the window had
// none and Windows showed its generic one (23 October 2026).
//
// The icon is the first in the executable, read with ExtractIconExW, and
// set with WM_SETICON. The window is the top-level window of this process
// with Dinah's class, found with FindWindowExW and GetWindowThreadProcessId.
// https://learn.microsoft.com/windows/win32/api/shellapi/nf-shellapi-extracticonexw
// https://learn.microsoft.com/windows/win32/winmsg/wm-seticon
// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-findwindowexw
func setWindowIcon() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	path, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return
	}
	var large, small windows.Handle
	if n, _, _ := procExtractIconsExW.Call(uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1); n == 0 {
		return
	}
	class, _ := windows.UTF16PtrFromString(windowClass)
	pid := uint32(os.Getpid())
	var h uintptr
	for {
		h, _, _ = procFindWindowExW.Call(0, h, uintptr(unsafe.Pointer(class)), 0)
		if h == 0 {
			return
		}
		var owner uint32
		windows.GetWindowThreadProcessId(windows.HWND(h), &owner)
		if owner == pid {
			break
		}
	}
	const wmSetIcon, iconSmall, iconBig = 0x0080, 0, 1
	// The window keeps the icons for its lifetime, so they are not destroyed.
	if large != 0 {
		procSendMessageIconW.Call(h, wmSetIcon, iconBig, uintptr(large))
	}
	if small != 0 {
		procSendMessageIconW.Call(h, wmSetIcon, iconSmall, uintptr(small))
	}
}
