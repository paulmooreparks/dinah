package programs

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// SetUserPath puts dir on the person's own PATH in place of the folder old,
// or at its end (EditPath), and returns whether the PATH changed. The person's
// PATH is the value Path under HKEY_CURRENT_USER\Environment, which needs no
// administrator, and it keeps its type, which is REG_EXPAND_SZ when it holds
// variables. Programs that are running are told that the environment changed
// with WM_SETTINGCHANGE, as the documentation of environment variables says,
// so a Terminal started afterwards from Explorer finds the programs.
// https://learn.microsoft.com/windows/win32/procthread/environment-variables
func SetUserPath(old, dir string) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	value, kind, err := k.GetStringValue("Path")
	if errors.Is(err, registry.ErrNotExist) {
		value, kind, err = "", registry.EXPAND_SZ, nil
	}
	if err != nil {
		return false, err
	}
	expand := func(s string) string {
		if out, err := registry.ExpandString(s); err == nil {
			return out
		}
		return s
	}
	next := EditPath(value, old, dir, expand)
	if next == value {
		return false, nil
	}
	if kind == registry.EXPAND_SZ {
		err = k.SetExpandStringValue("Path", next)
	} else {
		err = k.SetStringValue("Path", next)
	}
	if err != nil {
		return false, err
	}
	environmentChanged()
	return true, nil
}

var procSendMessageTimeout = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")

// environmentChanged broadcasts WM_SETTINGCHANGE with "Environment", and
// gives up on a window that does not answer within five seconds.
// https://learn.microsoft.com/windows/win32/winmsg/wm-settingchange
func environmentChanged() {
	const hwndBroadcast, wmSettingChange, smtoAbortIfHung = 0xffff, 0x001A, 0x0002
	text, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	procSendMessageTimeout.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(text)), smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&result)))
}
