package resident

import (
	"io/fs"
	"os"
	"path/filepath"
)

// listedEntry is one entry of a listing Windows' listDir reads record by
// record rather than through os.ReadDir.
type listedEntry struct {
	dir, name string
	typ       fs.FileMode
}

func (e *listedEntry) Name() string      { return e.name }
func (e *listedEntry) IsDir() bool       { return e.typ.IsDir() }
func (e *listedEntry) Type() fs.FileMode { return e.typ }

// Info answers the entry's own FileInfo, not following a link, as
// fs.DirEntry's Info is documented to.
func (e *listedEntry) Info() (fs.FileInfo, error) {
	return os.Lstat(filepath.Join(e.dir, e.name))
}
