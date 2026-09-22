//go:build !windows

package setup

import "path/filepath"

// finalPath answers the path the filesystem finally reaches for an existing
// file or directory. Outside Windows a symbolic link is the only kind of link
// a path component can be, and filepath.EvalSymlinks is documented to return
// "the path name after the evaluation of any symbolic links", resolving every
// component. The Windows build resolves through the operating system instead,
// because there EvalSymlinks does not follow a junction.
func finalPath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
