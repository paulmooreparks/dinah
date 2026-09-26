package verb

import "os"

// plantedOpen opens a file below the workbench itself.
func (l *Library) plantedOpen(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}
