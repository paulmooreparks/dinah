//go:build !windows

package agents

import "path/filepath"

// ResolvePath resolves the links in a path.
func ResolvePath(path string) (string, error) { return filepath.EvalSymlinks(path) }
