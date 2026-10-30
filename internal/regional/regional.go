// Package regional reads the person's regional format for dates and times
// from the operating system, so that the page can write them as the
// person's computer writes them (Paul, 26 October 2026).
//
// Go's standard library has no way to read the format, so each system's
// documented interface is used: on Windows the user's default locale name
// and its time format, on macOS the current CoreFoundation locale and its
// short time format, and elsewhere the POSIX locale settings. The result is
// a BCP 47 language tag, which the page gives to Intl.DateTimeFormat. A
// format the person customised beyond the locale's, such as a different
// separator, is not carried, as a language tag cannot express it.
package regional

import "strings"

// HostLocale returns the language tag of the person's regional format on
// this computer, such as en-SG-u-hc-h23. It returns "" when the format
// cannot be read, and the page then uses its own default.
func HostLocale() string { return hostLocale() }

// localeTag makes a BCP 47 tag of a locale name and the clock the person
// uses: hour24 is 1 for a 24-hour clock, 0 for 12-hour, and -1 when it is
// not known, which leaves the locale's own.
func localeTag(name string, hour24 int) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "_", "-"))
	if name == "" || strings.HasPrefix(name, "x-") || strings.ContainsAny(name, " .@") {
		return ""
	}
	switch hour24 {
	case 1:
		return name + "-u-hc-h23"
	case 0:
		return name + "-u-hc-h12"
	}
	return name
}

// posixLocale is the locale name of a POSIX locale setting, such as
// en_SG.UTF-8 or de_DE@euro; C and POSIX have none.
func posixLocale(v string) string {
	if i := strings.IndexAny(v, ".@"); i >= 0 {
		v = v[:i]
	}
	if v == "C" || v == "POSIX" {
		return ""
	}
	return v
}
