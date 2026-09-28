//go:build !linux

package durable

// syncDir does nothing outside Linux. On Windows a rename is made durable by
// MOVEFILE_WRITE_THROUGH, and on every other Unix, macOS and the BSDs among
// them, no manual page documents a directory flush that makes a rename
// durable. After a power loss there a destination holds the old bytes or the
// new ones, never a partial file, because the temporary was flushed before the
// rename.
func syncDir(string) error {
	return nil
}
