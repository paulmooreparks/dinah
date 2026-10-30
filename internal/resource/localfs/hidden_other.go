//go:build !windows

package localfs

import "os"

// PathHidden reports whether the entry at path is hidden by an attribute.
// Other platforms mark hidden entries with a leading dot, which CleanPath
// already refuses.
func PathHidden(string) (bool, error) { return false, nil }

// FileHidden reports whether an open file is hidden by an attribute; see
// PathHidden.
func FileHidden(*os.File) (bool, error) { return false, nil }
