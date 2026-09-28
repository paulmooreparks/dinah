package bench

import (
	"errors"
	"io/fs"
	"os"
)

// want: os.OpenFile

// plantedOpenFile reads an optional file with a read-only OpenFile and treats
// its absence as nothing to read.
func (b *Bench) plantedOpenFile(p string) error {
	f, err := os.OpenFile(p, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return f.Close()
}
