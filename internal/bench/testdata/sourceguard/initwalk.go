package bench

import (
	"io/fs"
	"path/filepath"
)

// want: filepath.WalkDir

// plantedWalker holds a walk a method calls through a field.
type plantedWalker struct {
	walk func(string, fs.WalkDirFunc) error
}

var plantedWalkerHeld plantedWalker

// init stores the walk before any method runs, so the method's call names no
// read, and the read sits in a function no method reaches.
func init() {
	plantedWalkerHeld.walk = filepath.WalkDir
}

// plantedWalkThroughInit walks the tree through the field init filled.
func (b *Bench) plantedWalkThroughInit() int {
	n := 0
	_ = plantedWalkerHeld.walk(b.Root, func(string, fs.DirEntry, error) error {
		n++
		return nil
	})
	return n
}
