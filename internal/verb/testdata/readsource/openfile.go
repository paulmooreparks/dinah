package verb

import (
	"errors"
	"io/fs"
	"os"
)

// plantedOpenFile reads an optional file with a read-only OpenFile and treats
// its absence as nothing to read.
func (l *Library) plantedOpenFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return f.Close()
}
