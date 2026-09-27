//go:build !windows

package resident

// WorkingDirectory answers the directory dinah serve moves its working
// directory to: "/", which lies beneath no directory.
func WorkingDirectory(string) (string, error) {
	return "/", nil
}
