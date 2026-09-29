package bench

import "os"

// want: os.ReadFile

// plantHook is called by a method; nothing but PlantConfigure fills it.
var plantHook func(string) ([]byte, error)

// plantReadIt reads a file. No method calls it; PlantConfigure names it as a
// value and stores it where a method calls it (dinah-619/comments/15).
func plantReadIt(p string) ([]byte, error) { return os.ReadFile(p) }

// PlantConfigure is an exported free function no root reaches.
func PlantConfigure() { plantHook = plantReadIt }

// plantedReadThroughHook calls the stored function, naming no read.
func (b *Bench) plantedReadThroughHook() []byte {
	data, _ := plantHook(b.Root)
	return data
}
