//go:build linux

package durable

import "os"

// syncDir flushes a directory's entries to the disk. The Linux fsync(2) page
// says that calling fsync on a file "does not necessarily ensure that the entry
// in the directory containing the file has also reached disk", and that "an
// explicit fsync() on a file descriptor for the directory is also needed".
func syncDir(dir string) error {
	observe("syncdir", dir, 0)
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}
