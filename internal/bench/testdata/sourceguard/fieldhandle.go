package bench

import "io/fs"

// want: fs.FS

// plantedHolder keeps a filesystem in a field, where no signature shows it.
type plantedHolder struct {
	fsys fs.FS
}

var plantedHeld plantedHolder

// plantedReadThroughField opens a file through the field.
func (b *Bench) plantedReadThroughField() error {
	f, err := plantedHeld.fsys.Open("workbench.md")
	if err != nil {
		return err
	}
	return f.Close()
}
