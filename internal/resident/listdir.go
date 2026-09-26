package resident

import (
	"io/fs"
	"os"
	"path/filepath"
)

// listDirHeld, when set, runs inside listDir while the listing's handle is
// still open, after the enumeration and before the close. Only a test sets
// it, to rename and delete the directory at the moment the resident holds it,
// through the builder's own listing rather than through a helper the builder
// might stop calling.
var listDirHeld func(path string)

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
