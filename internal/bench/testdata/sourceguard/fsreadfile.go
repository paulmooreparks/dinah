package bench

import "io/fs"

// want: fs.ReadFile

// plantedFSRead reads a file through an fs.FS its caller hands it, so no
// member of os appears at all.
func (b *Bench) plantedFSRead(fsys fs.FS) []byte {
	data, _ := fs.ReadFile(fsys, "card.md")
	return data
}
