//go:build !windows

package resident

import "os"

// openShared is os.Open: only Windows lets an open file refuse another
// process's delete or rename.
func openShared(path string) (*os.File, error) {
	return os.Open(path)
}
