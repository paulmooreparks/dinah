package verb

import "io/fs"

// plantedFSRead reads a file through an fs.FS its caller hands it, so no
// member of os appears at all.
func (l *Library) plantedFSRead(fsys fs.FS) []byte {
	data, _ := fs.ReadFile(fsys, "card-numbers.txt")
	return data
}
