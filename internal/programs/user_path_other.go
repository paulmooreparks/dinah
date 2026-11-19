//go:build !windows

package programs

import "errors"

// SetUserPath is not built for this system yet: a shell reads its PATH from
// files of the person's own, which an installer should not edit unasked.
func SetUserPath(old, dir string) (bool, error) {
	return false, errors.New("putting the programs on the PATH is not built for this system; add " + dir + " to your shell's PATH")
}
