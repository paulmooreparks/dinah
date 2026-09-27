//go:build !windows

package resident

import (
	"io/fs"
	"os"
)

// isLink judges an entry a build, a re-list or a reconcile is about to read:
// a symbolic link, which io/fs documents as fs.ModeSymlink. No resident runs
// here in production, but the portable tests drive one through
// residenttest.Manual on every platform, and this is how the link rule is
// tested where creating a symbolic link needs no privilege.
func isLink(_ string, entry fs.DirEntry) bool {
	return entry.Type()&fs.ModeSymlink != 0
}

// isLinkPath is isLink for a path a reconcile names.
func isLinkPath(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}
