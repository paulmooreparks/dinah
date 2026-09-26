package verb

import (
	"io/fs"
	"path/filepath"
)

// plantedWalkDir walks the workbench tree.
func (l *Library) plantedWalkDir() int {
	n := 0
	_ = filepath.WalkDir(l.Bench.Root, func(string, fs.DirEntry, error) error {
		n++
		return nil
	})
	return n
}
