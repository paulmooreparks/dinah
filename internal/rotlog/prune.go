package rotlog

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Prune removes the files under dir last written more than maxAge ago, then
// the oldest of the rest until what remains is at most maxTotal bytes. A file
// that cannot be removed, such as one open on Windows, is passed over. It
// returns how many files were removed. A zero maxAge or maxTotal does not
// limit by that measure.
func Prune(dir string, maxAge time.Duration, maxTotal int64) int {
	type file struct {
		path string
		size int64
		mod  time.Time
	}
	var files []file
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, file{p, info.Size(), info.ModTime()})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	removed := 0
	var total int64
	for _, f := range files {
		total += f.size
	}
	cutoff := time.Now().Add(-maxAge)
	for _, f := range files {
		old := maxAge > 0 && f.mod.Before(cutoff)
		big := maxTotal > 0 && total > maxTotal
		if !old && !big {
			continue
		}
		if os.Remove(f.path) == nil {
			removed++
			total -= f.size
		}
	}
	return removed
}
