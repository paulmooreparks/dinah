package bench

import (
	"io/fs"
	"path/filepath"
)

// want: filepath.WalkDir

// plantedCopyWalk holds filepath.WalkDir, and plantedCopyWalker receives a
// copy of it from a free function no method calls. This is the shape the
// round-two header stated as the residue; naming a variable as a value now
// makes it a root of the walk.
var plantedCopyWalk = filepath.WalkDir

type plantedCopyWalker struct {
	walk func(string, fs.WalkDirFunc) error
}

var plantedCopyHolder plantedCopyWalker

// PlantCopyWalk stores the variable's value in the holder.
func PlantCopyWalk() { plantedCopyHolder.walk = plantedCopyWalk }

// plantedWalkThroughCopy walks through the holder.
func (b *Bench) plantedWalkThroughCopy() int {
	n := 0
	_ = plantedCopyHolder.walk(b.Root, func(string, fs.DirEntry, error) error {
		n++
		return nil
	})
	return n
}
