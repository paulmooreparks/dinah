package bench

import (
	"io/fs"
	"path/filepath"
)

// want: filepath.WalkDir

// plantedWalkDir walks the tree below the root.
func (b *Bench) plantedWalkDir() int {
	n := 0
	_ = filepath.WalkDir(b.Root, func(string, fs.DirEntry, error) error {
		n++
		return nil
	})
	return n
}
