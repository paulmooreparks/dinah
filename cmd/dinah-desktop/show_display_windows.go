//go:build windows

package main

import (
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procMonitorFromWindow       = iconUser32.NewProc("MonitorFromWindow")
	procEnumDisplayMonitors     = iconUser32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW         = iconUser32.NewProc("GetMonitorInfoW")
	procSetWindowPos            = iconUser32.NewProc("SetWindowPos")
	procSetForegroundWindow     = iconUser32.NewProc("SetForegroundWindow")
	procSetThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")
)

type winRect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

// ownWindow is the top-level window of this process with the given class,
// found with FindWindowExW and GetWindowThreadProcessId, or 0.
// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-findwindowexw
func ownWindow(class string) uintptr {
	name, err := windows.UTF16PtrFromString(class)
	if err != nil || class == "" {
		return 0
	}
	pid := uint32(os.Getpid())
	var h uintptr
	for {
		h, _, _ = procFindWindowExW.Call(0, h, uintptr(unsafe.Pointer(name)), 0)
		if h == 0 {
			return 0
		}
		var owner uint32
		windows.GetWindowThreadProcessId(windows.HWND(h), &owner)
		if owner == pid {
			return h
		}
	}
}

// systemDisplays lists the displays with their rectangles, and marks the
// one the main window is on. MonitorFromWindow gives that display,
// EnumDisplayMonitors every display and GetMonitorInfoW each one's rectangle
// and whether it is the primary one.
// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-monitorfromwindow
// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-enumdisplaymonitors
// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-getmonitorinfow
func systemDisplays() []showDisplay {
	const monitorDefaultToNearest, monitorInfoPrimary = 2, 1
	var current uintptr
	if h := ownWindow(windowClass); h != 0 {
		current, _, _ = procMonitorFromWindow.Call(h, monitorDefaultToNearest)
	}
	var out []showDisplay
	each := windows.NewCallback(func(monitor, _, _, _ uintptr) uintptr {
		info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
			r := info.Monitor
			out = append(out, showDisplay{X: int(r.Left), Y: int(r.Top), Width: int(r.Right - r.Left), Height: int(r.Bottom - r.Top),
				Primary: info.Flags&monitorInfoPrimary != 0, Current: current != 0 && monitor == current})
		}
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, each, 0)
	return out
}

// moveShowWindow puts this process's window of the given class over a
// display's rectangle with SetWindowPos, without showing it or changing which
// window is in front.
// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-setwindowpos
func moveShowWindow(class string, d showDisplay) bool {
	const noZOrder, noActivate = 0x0004, 0x0010
	h := ownWindow(class)
	if h == 0 {
		return false
	}
	ok, _, _ := procSetWindowPos.Call(h, 0, uintptr(int32(d.X)), uintptr(int32(d.Y)), uintptr(int32(d.Width)), uintptr(int32(d.Height)), noZOrder|noActivate)
	return ok != 0
}

// focusShowWindow asks for the show's window to be the one the keyboard
// goes to. Windows grants it to a process the foreground process started,
// which the show's is, and otherwise flashes the window's taskbar button.
// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-setforegroundwindow
func focusShowWindow(class string) {
	if h := ownWindow(class); h != 0 {
		procSetForegroundWindow.Call(h)
	}
}

// keepDisplayAwake keeps the display on while this process runs. The request
// belongs to the thread that makes it and ends with that thread, so a thread
// is kept for it.
// https://learn.microsoft.com/windows/win32/api/winbase/nf-winbase-setthreadexecutionstate
func keepDisplayAwake() {
	go func() {
		runtime.LockOSThread()
		const continuous, displayRequired = 0x80000000, 0x00000002
		procSetThreadExecutionState.Call(continuous | displayRequired)
		select {}
	}()
}
