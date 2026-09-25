//go:build !windows && !darwin

package browser

// open hands the URL to xdg-open(1) as its one argument. The xdg-open(1)
// manual page from freedesktop.org's xdg-utils says: "xdg-open opens a file or
// URL in the user's preferred application." A machine without xdg-utils
// answers an error from Start, which the caller reports.
func open(address string) error {
	return startCommand("xdg-open", address)
}
