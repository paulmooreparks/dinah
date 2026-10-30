package layout

import (
	"path/filepath"
	"runtime"
	"strings"
)

// Inside reports whether p is root or a path below it.
func Inside(root, p string) bool {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}

// SamePath reports whether two paths name one place, ignoring case on
// Windows, whose file system does.
func SamePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
