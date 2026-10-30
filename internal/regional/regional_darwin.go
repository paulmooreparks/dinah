//go:build darwin && cgo

package regional

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

// readHostLocale writes the current locale's identifier and its short time
// format, both as UTF-8; it returns 0 when there is no current locale.
static int readHostLocale(char *name, int nameLen, char *timeFormat, int formatLen) {
	CFLocaleRef locale = CFLocaleCopyCurrent();
	if (locale == NULL) return 0;
	int ok = 0;
	CFStringRef id = CFLocaleGetIdentifier(locale);
	if (id != NULL) ok = CFStringGetCString(id, name, nameLen, kCFStringEncodingUTF8);
	timeFormat[0] = 0;
	CFDateFormatterRef f = CFDateFormatterCreate(NULL, locale, kCFDateFormatterNoStyle, kCFDateFormatterShortStyle);
	if (f != NULL) {
		CFStringRef s = CFDateFormatterGetFormat(f);
		if (s != NULL) CFStringGetCString(s, timeFormat, formatLen, kCFStringEncodingUTF8);
		CFRelease(f);
	}
	CFRelease(locale);
	return ok;
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

// hostLocale reads the person's regional format from macOS through
// CoreFoundation, which documents both calls: CFLocaleCopyCurrent is the
// locale of System Settings › General › Language & Region, and the short
// time format of a date formatter for it uses H when the person has chosen
// a 24-hour clock.
func hostLocale() string {
	name := make([]byte, 128)
	format := make([]byte, 128)
	if C.readHostLocale((*C.char)(unsafe.Pointer(&name[0])), C.int(len(name)), (*C.char)(unsafe.Pointer(&format[0])), C.int(len(format))) == 0 {
		return ""
	}
	cstr := func(b []byte) string {
		if i := strings.IndexByte(string(b), 0); i >= 0 {
			return string(b[:i])
		}
		return string(b)
	}
	hour24 := -1
	if f := cstr(format); f != "" {
		hour24 = 0
		if strings.ContainsAny(f, "Hk") {
			hour24 = 1
		}
	}
	return localeTag(posixLocale(cstr(name)), hour24)
}
