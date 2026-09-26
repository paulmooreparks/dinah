//go:build !windows

package resident

import (
	"io/fs"
	"os"
	"sort"
)

// listDir stats and lists one directory through a single handle, closed as
// soon as the listing ends. Outside Windows an open directory refuses no
// other process's rename or delete, so the one handle only saves the second
// open that os.Stat and os.ReadDir together would make.
func listDir(path string) (fs.FileInfo, []fs.DirEntry, error) {
	file, err := os.Open(path)
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
