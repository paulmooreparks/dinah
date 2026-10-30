//go:build !windows && !(darwin && cgo)

package regional

import "os"

// hostLocale reads the person's regional format for dates and times from
// the POSIX locale settings, in their documented order of precedence:
// LC_ALL, then LC_TIME, then LANG. They do not say which clock the person
// prefers, so the locale's own is used.
func hostLocale() string {
	for _, v := range []string{"LC_ALL", "LC_TIME", "LANG"} {
		if s := os.Getenv(v); s != "" {
			return localeTag(posixLocale(s), -1)
		}
	}
	return ""
}
