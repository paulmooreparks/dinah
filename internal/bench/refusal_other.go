//go:build !windows

package bench

// transientRenameRefusal is false outside Windows, where an open handle
// refuses no rename.
func transientRenameRefusal(error) bool { return false }

// transientRemoveRefusal is false outside Windows, where an open handle
// refuses no removal.
func transientRemoveRefusal(error) bool { return false }
