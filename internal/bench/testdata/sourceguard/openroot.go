package bench

import "os"

// want: os.OpenRoot

// plantedOpenRoot reads a file through a Root, whose own ReadFile is a method
// on a value and names no package member.
func (b *Bench) plantedOpenRoot() []byte {
	root, err := os.OpenRoot(b.Root)
	if err != nil {
		return nil
	}
	defer root.Close()
	data, _ := root.ReadFile("card.md")
	return data
}
