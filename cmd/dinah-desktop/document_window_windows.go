//go:build windows

package main

import (
	"strings"
	"unicode"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procSetAppUserModelID        = iconShell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	procAllowSetForegroundWindow = iconUser32.NewProc("AllowSetForegroundWindow")
)

// identityWord is a word as an application user model ID takes it: letters
// and digits only.
func identityWord(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// documentAppUserModelID is the taskbar identity of one stand-alone
// application: company, product and application, with full stops between,
// as the form Windows documents for it.
func documentAppUserModelID(application string) string {
	return identityWord(product.Company) + "." + identityWord(product.Name) + ".Document." + identityWord(application)
}

// setProcessIdentity gives this process the taskbar identity of the
// application its window is, so that the windows of one application group
// together on the taskbar and apart from the main window's and from the
// other applications'. It is set before the window is made.
// https://learn.microsoft.com/windows/win32/api/shobjidl_core/nf-shobjidl_core-setcurrentprocessexplicitappusermodelid
func setProcessIdentity(application string) {
	id, err := windows.UTF16PtrFromString(documentAppUserModelID(application))
	if err != nil {
		return
	}
	procSetAppUserModelID.Call(uintptr(unsafe.Pointer(id)))
}

// allowForeground lets another process bring its window to the front,
// which Windows permits a process to grant when it could do so itself, as
// one the person has just started or the one in front can.
// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-allowsetforegroundwindow
func allowForeground(pid int) {
	if pid > 0 {
		procAllowSetForegroundWindow.Call(uintptr(uint32(pid)))
	}
}

// focusOwnWindow asks for this process's window of a class to be the one
// in front. Windows grants it to a process that was allowed to, and
// otherwise flashes the window's taskbar button.
func focusOwnWindow(class string) { focusShowWindow(class) }
