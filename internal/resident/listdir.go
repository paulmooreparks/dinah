package resident

import (
	"io/fs"
	"sort"

	"dinah/internal/durable"
)

// listDirHeld, when set, runs inside listDir while the listing's handle is
// still open, after the enumeration and before the close. Only a test sets
// it, to rename and delete the directory at the moment the resident holds it,
// through the builder's own listing rather than through a helper the builder
// might stop calling.
var listDirHeld func(path string)

// listDir stats and lists one directory through a single handle, and closes
// it the moment the listing ends, before any file in the directory is read.
// The handle is durable.OpenDir's, which on Windows shares read, write and
// delete, so a directory the resident is listing never refuses another
// process's rename or removal of it for the length of the listing. The one
// handle also answers the directory's own stat, where os.Stat would open a
// second one.
//
// The enumeration is (*os.File).ReadDir, whose documentation is the whole of
// the contract this rests on: "ReadDir reads the contents of the directory
// associated with the file f and returns a slice of DirEntry values in
// directory order." and "If n <= 0, ReadDir returns all the DirEntry records
// remaining in the directory. When it succeeds, it returns a nil error (not
// io.EOF)." Which codes the operating system answers at the end of a
// directory is the os package's to know, and nothing here reads one.
//
// The entries are sorted by name, as os.ReadDir sorts them.
func listDir(path string) (fs.FileInfo, []fs.DirEntry, error) {
	file, err := durable.OpenDir(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return info, nil, nil
	}
	entries, err := file.ReadDir(-1)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if listDirHeld != nil {
		listDirHeld(path)
	}
	return info, entries, nil
}
