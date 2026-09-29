//go:build windows

package resident

import "golang.org/x/sys/windows"

// longPath resolves a path to its long form with GetLongPathNameW, documented
// as "Converts the specified path to its long form," which fails when the
// path does not exist. A change record may carry either of a file's two
// names, and the documentation leaves unspecified which, so the reconcile
// asks for the long form of a name it cannot place.
func longPath(path string) (string, error) {
	in, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetLongPathName(in, &buf[0], uint32(len(buf)))
		if err != nil {
			return "", err
		}
		if int(n) < len(buf) {
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n)
	}
}
