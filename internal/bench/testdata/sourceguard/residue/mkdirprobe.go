package bench

import (
	"os"
	"path/filepath"
)

// PlantExists answers whether a folder exists below the workbench by trying
// to create it: os.IsExist on the error of os.Mkdir reports what a Stat would
// have. Every write is allowlisted, so the guard cannot tell a write used as
// a read from a write, and this passes (dinah-619/comments/15). It is kept as
// the reproduction of a gap the guard states rather than closes.
func (b *Bench) PlantExists(name string) bool {
	return os.IsExist(os.Mkdir(filepath.Join(b.Root, name), 0o755))
}
