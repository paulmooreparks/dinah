package bench

import (
	"io/fs"
	"path/filepath"
)

// want: filepath.WalkDir

// plantedWalk holds filepath.WalkDir, so a call through it names no read at
// the call site.
var plantedWalk = filepath.WalkDir

// plantedWalkThroughVariable walks the tree through the variable.
func (b *Bench) plantedWalkThroughVariable() int {
	n := 0
	_ = plantedWalk(b.Root, func(string, fs.DirEntry, error) error {
		n++
		return nil
	})
	return n
}
