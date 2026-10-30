//go:build windows

package hostnotify

import (
	"errors"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"studio/internal/profile"
)

// On Windows the notifier uses these documented Win32 interfaces.
//
// A notification is a balloon notification of an icon in the notification
// area, shown with Shell_NotifyIcon. The icon is added when the first
// notification is shown and deleted when the notifier is closed. The remarks
// of Shell_NotifyIconW say that Windows 10 shows the balloon as a banner
// notification.
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shell_notifyiconw
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-notifyicondataw
// https://learn.microsoft.com/en-us/windows/win32/shell/notification-area
//
// The icon reports clicks to a message-only window this file makes with
// RegisterClassExW and CreateWindowExW, whose thread pumps its messages.
// Under NOTIFYICON_VERSION_4 the low word of lParam is the event.
// https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features#message-only-windows
//
// The icon is the program's own, the first icon in its executable, read with
// ExtractIconExW.
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-extracticonexw
//
// Wails v2 documents no way to get the handle of its main window, so the
// main window is found with EnumWindows and GetWindowThreadProcessId: the
// visible, unowned top-level window of this process, preferring the one
// titled with the app's name. A message-only window cannot be enumerated.
// https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enumwindows
// https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowthreadprocessid
//
// Attention flashes the main window's taskbar button with FlashWindowEx until
// the window comes to the foreground.
// https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-flashwindowex
//
// Focused compares the process of GetForegroundWindow with this one.
// https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getforegroundwindow
//
// A click on the notification or the icon restores the main window and asks
// for it with SetForegroundWindow, which Windows may refuse; when it does,
// it flashes the taskbar button instead.
// https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setforegroundwindow

var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procFlashWindowEx       = user32.NewProc("FlashWindowEx")
	procGetWindow           = user32.NewProc("GetWindow")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procIsIconic            = user32.NewProc("IsIconic")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procExtractIconExW      = shell32.NewProc("ExtractIconExW")
)

const (
	wmDestroy         = 0x0002
	wmLButtonUp       = 0x0202
	wmUser            = 0x0400
	wmApp             = 0x8000
	wmNotifyIcon      = wmApp + 1 // the icon's callback message
	wmCloseNotifier   = wmApp + 2 // asks the window's own thread to destroy it
	ninSelect         = wmUser + 0
	ninKeySelect      = wmUser + 1
	ninBalloonClick   = wmUser + 5
	nimAdd            = 0
	nimModify         = 1
	nimDelete         = 2
	nimSetVersion     = 4
	nifMessage        = 0x01
	nifIcon           = 0x02
	nifTip            = 0x04
	nifInfo           = 0x10
	nifShowTip        = 0x80
	niifInfo          = 0x01
	niifUser          = 0x04
	niifLargeIcon     = 0x20
	notifyIconVersion = 4
	flashwAll         = 0x03
	flashwTimerNoFG   = 0x0C
	gwOwner           = 4
	swRestore         = 9
	idiApplication    = 32512
	iconID            = 1
)

// hwndMessage is HWND_MESSAGE, ((HWND)-3), the parent of a message-only
// window.
var hwndMessage = ^uintptr(2)

type notifyIconData struct {
	Size            uint32
	Wnd             windows.HWND
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            windows.Handle
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32 // a union with uTimeout
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUIDItem        windows.GUID
	BalloonIcon     windows.Handle
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type winMsg struct {
	Wnd     windows.HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
	Private uint32
}

type flashWInfo struct {
	Size    uint32
	Wnd     windows.HWND
	Flags   uint32
	Count   uint32
	Timeout uint32
}

// A callback made with windows.NewCallback is never released, so each is
// made once for the process, and the window procedure finds the notifier
// through current.
var (
	callbacks sync.Once
	wndProcCB uintptr
	enumCB    uintptr
	current   atomic.Pointer[winNotifier]

	enumMu    sync.Mutex
	enumFound []windows.HWND
)

func makeCallbacks() {
	callbacks.Do(func() {
		wndProcCB = windows.NewCallback(wndProc)
		enumCB = windows.NewCallback(enumProc)
	})
}

type winNotifier struct {
	appName  string
	activate func()

	mu      sync.Mutex
	started bool
	err     error
	hwnd    windows.HWND // the message-only window
	added   bool
	small   windows.Handle
	large   windows.Handle
	ownIcon bool // the icons came from ExtractIconExW and are destroyed on close
}

func newNotifier(appName string, activate func()) Notifier {
	n := &winNotifier{appName: appName, activate: activate}
	current.Store(n)
	return n
}

// start makes the message-only window on a thread of its own, once.
func (n *winNotifier) start() error {
	if n.started {
		return n.err
	}
	n.started = true
	makeCallbacks()
	ready := make(chan error, 1)
	go n.pump(ready)
	n.err = <-ready
	return n.err
}

// pump makes the window and dispatches its messages until it is destroyed.
// A window belongs to the thread that made it, so the goroutine stays on
// its thread.
func (n *winNotifier) pump(ready chan<- error) {
	runtime.LockOSThread()
	var inst windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &inst); err != nil {
		ready <- err
		return
	}
	class, _ := windows.UTF16PtrFromString(profile.Current().NotificationClass)
	wc := wndClassEx{WndProc: wndProcCB, Instance: inst, ClassName: class}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 && !errors.Is(err, windows.ERROR_CLASS_ALREADY_EXISTS) {
		ready <- err
		return
	}
	h, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, uintptr(inst), 0)
	if h == 0 {
		ready <- err
		return
	}
	n.hwnd = windows.HWND(h)
	ready <- nil
	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmNotifyIcon:
		switch lParam & 0xFFFF {
		case ninSelect, ninKeySelect, ninBalloonClick, wmLButtonUp:
			if n := current.Load(); n != nil {
				go n.bringForward()
			}
		}
		return 0
	case wmCloseNotifier:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

// enumProc collects the visible, unowned top-level windows of this process.
func enumProc(h windows.HWND, _ uintptr) uintptr {
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(h, &pid); err == nil && pid == uint32(os.Getpid()) && windows.IsWindowVisible(h) {
		if owner, _, _ := procGetWindow.Call(uintptr(h), gwOwner); owner == 0 {
			enumFound = append(enumFound, h)
		}
	}
	return 1
}

// mainWindow is the app's main window, or 0 when none is found.
func (n *winNotifier) mainWindow() windows.HWND {
	makeCallbacks()
	enumMu.Lock()
	defer enumMu.Unlock()
	enumFound = nil
	_ = windows.EnumWindows(enumCB, nil)
	found := enumFound
	enumFound = nil
	for _, h := range found {
		var buf [256]uint16
		procGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if windows.UTF16ToString(buf[:]) == n.appName {
			return h
		}
	}
	if len(found) > 0 {
		return found[0]
	}
	return 0
}

func (n *winNotifier) bringForward() {
	if n.activate != nil {
		n.activate()
	}
	h := n.mainWindow()
	if h == 0 {
		return
	}
	if r, _, _ := procIsIconic.Call(uintptr(h)); r != 0 {
		procShowWindow.Call(uintptr(h), swRestore)
	}
	procSetForegroundWindow.Call(uintptr(h))
}

func (n *winNotifier) Focused() (bool, bool) {
	fg := windows.GetForegroundWindow()
	if fg == 0 {
		return false, true
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(fg, &pid); err != nil {
		return false, false
	}
	return pid == uint32(os.Getpid()), true
}

func (n *winNotifier) Attention() error {
	h := n.mainWindow()
	if h == 0 {
		return nil
	}
	fi := flashWInfo{Wnd: h, Flags: flashwAll | flashwTimerNoFG}
	fi.Size = uint32(unsafe.Sizeof(fi))
	// The return value is the window's state before the call, not success.
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&fi)))
	return nil
}

// loadIcons reads the executable's first icon, large and small, or falls
// back on the system's application icon.
func (n *winNotifier) loadIcons() {
	if n.small != 0 {
		return
	}
	if exe, err := os.Executable(); err == nil {
		if path, err := windows.UTF16PtrFromString(exe); err == nil {
			var large, small windows.Handle
			r, _, _ := procExtractIconExW.Call(uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&large)), uintptr(unsafe.Pointer(&small)), 1)
			if r != 0 && small != 0 {
				n.small, n.large, n.ownIcon = small, large, true
				return
			}
			destroyIcon(large)
			destroyIcon(small)
		}
	}
	h, _, _ := procLoadIconW.Call(0, idiApplication)
	n.small, n.large, n.ownIcon = windows.Handle(h), 0, false
}

func destroyIcon(h windows.Handle) {
	if h != 0 {
		procDestroyIcon.Call(uintptr(h))
	}
}

func (n *winNotifier) base() notifyIconData {
	d := notifyIconData{Wnd: n.hwnd, ID: iconID}
	d.Size = uint32(unsafe.Sizeof(d))
	return d
}

func shellNotifyIcon(message uint32, d *notifyIconData) bool {
	r, _, _ := procShellNotifyIconW.Call(uintptr(message), uintptr(unsafe.Pointer(d)))
	return r != 0
}

// add puts the icon in the notification area and selects version 4, which
// must be selected every time the icon is added.
func (n *winNotifier) add() error {
	d := n.base()
	d.Flags = nifMessage | nifIcon | nifTip | nifShowTip
	d.CallbackMessage = wmNotifyIcon
	d.Icon = n.small
	copyUTF16(d.Tip[:], n.appName)
	if !shellNotifyIcon(nimAdd, &d) {
		return errors.New("the notification area did not accept the icon")
	}
	d.Version = notifyIconVersion
	shellNotifyIcon(nimSetVersion, &d)
	n.added = true
	return nil
}

func (n *winNotifier) Notify(title, body string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := n.start(); err != nil {
		return err
	}
	n.loadIcons()
	if !n.added {
		if err := n.add(); err != nil {
			return err
		}
	}
	d := n.base()
	d.Flags = nifInfo
	copyUTF16(d.InfoTitle[:], title)
	if body == "" {
		body = " " // an empty szInfo removes the balloon rather than showing one
	}
	copyUTF16(d.Info[:], body)
	d.InfoFlags = niifInfo
	if n.large != 0 {
		d.InfoFlags, d.BalloonIcon = niifUser|niifLargeIcon, n.large
	}
	if shellNotifyIcon(nimModify, &d) {
		return nil
	}
	// The icon is gone when Explorer has restarted since it was added; add
	// it again and show the notification once more.
	n.added = false
	if err := n.add(); err != nil {
		return err
	}
	if !shellNotifyIcon(nimModify, &d) {
		return errors.New("the notification area did not show the notification")
	}
	return nil
}

func (n *winNotifier) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.added {
		d := n.base()
		shellNotifyIcon(nimDelete, &d)
		n.added = false
	}
	if n.hwnd != 0 {
		procPostMessageW.Call(uintptr(n.hwnd), wmCloseNotifier, 0, 0)
		n.hwnd = 0
	}
	if n.ownIcon {
		destroyIcon(n.small)
		destroyIcon(n.large)
	}
	n.small, n.large, n.ownIcon = 0, 0, false
	n.started, n.err = true, errors.New("the notifier is closed")
	current.CompareAndSwap(n, nil)
	return nil
}

// copyUTF16 copies s into a fixed, null-terminated buffer, shortening it
// without splitting a surrogate pair.
func copyUTF16(dst []uint16, s string) {
	u := utf16.Encode([]rune(s))
	limit := len(dst) - 1
	if len(u) > limit {
		u = u[:limit]
		if utf16.IsSurrogate(rune(u[limit-1])) && u[limit-1] < 0xDC00 {
			u = u[:limit-1]
		}
	}
	n := copy(dst, u)
	dst[n] = 0
}
