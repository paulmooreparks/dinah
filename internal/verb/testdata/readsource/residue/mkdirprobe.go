package verb

import (
	"os"
	"path/filepath"
)

// plantExists answers whether a folder exists below the workbench by trying
// to create it: os.IsExist on the error of os.Mkdir reports what a Stat would
// have. Every write is allowed, so the guard cannot tell a write used as a
// read from a write, and this passes. It is kept as the reproduction of a gap
// the guard states rather than closes.
func (l *Library) plantExists(name string) bool {
	return os.IsExist(os.Mkdir(filepath.Join(l.Bench.Root, name), 0o755))
}
