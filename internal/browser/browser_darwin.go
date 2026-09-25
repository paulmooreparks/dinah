//go:build darwin

package browser

// open hands the URL to open(1) as its one argument. The open(1) manual page
// says: "The open command opens a file (or a directory or URL), just as if
// you had double-clicked the file's icon."
func open(address string) error {
	return startCommand("open", address)
}
