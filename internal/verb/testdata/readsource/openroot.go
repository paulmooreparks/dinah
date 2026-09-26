package verb

import "os"

// plantedOpenRoot reads a file through a Root, whose ReadFile is a method on
// a value and names no package member.
func (l *Library) plantedOpenRoot() []byte {
	root, err := os.OpenRoot(l.Bench.Root)
	if err != nil {
		return nil
	}
	defer root.Close()
	data, _ := root.ReadFile("card-numbers.txt")
	return data
}
