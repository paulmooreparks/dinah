package regional

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultLocaleName = kernel32.NewProc("GetUserDefaultLocaleName")
	procGetLocaleInfoEx          = kernel32.NewProc("GetLocaleInfoEx")
)

// localeSTimeFormat is LOCALE_STIMEFORMAT, the time format string.
const localeSTimeFormat = 0x1003

// hostLocale reads the person's regional format from Windows: the user's
// default locale name, which is Settings › Time & Language › Region's
// regional format (GetUserDefaultLocaleName), and whether its time format
// uses a 24-hour clock, which an H in it says (GetLocaleInfoEx with
// LOCALE_STIMEFORMAT). Both are documented Win32 functions.
func hostLocale() string {
	name := make([]uint16, 85) // LOCALE_NAME_MAX_LENGTH
	n, _, _ := procGetUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if n == 0 {
		return ""
	}
	locale := windows.UTF16ToString(name)
	hour24 := -1
	format := make([]uint16, 80)
	if m, _, _ := procGetLocaleInfoEx.Call(uintptr(unsafe.Pointer(&name[0])), localeSTimeFormat, uintptr(unsafe.Pointer(&format[0])), uintptr(len(format))); m != 0 {
		hour24 = 0
		if strings.Contains(windows.UTF16ToString(format), "H") {
			hour24 = 1
		}
	}
	return localeTag(locale, hour24)
}
