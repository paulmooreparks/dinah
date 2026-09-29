//go:build !windows

package resident

// longPath answers a path unchanged: only Windows keeps short names.
func longPath(path string) (string, error) {
	return path, nil
}
