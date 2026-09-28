package bench

import "io/fs"

// want: fs.FS

// PlantFromFS reads through an fs.FS opened elsewhere and handed in, the same
// shape as PlantFromRoot through the io/fs interface (dinah-619/comments/15).
func (b *Bench) PlantFromFS(fsys fs.FS) error {
	f, err := fsys.Open("workbench.md")
	if err != nil {
		return err
	}
	return f.Close()
}
