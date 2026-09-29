package bench

import "os"

// want: os.ReadFile

// plantHook3 is called by a method; nothing but PlantConfigure3 fills it.
var plantHook3 func(string) ([]byte, error)

// PlantConfigure3 is an exported free function no root reaches, which stores
// a function literal that reads (dinah-619/comments/15).
func PlantConfigure3() {
	plantHook3 = func(p string) ([]byte, error) { return os.ReadFile(p) }
}

// plantedReadThroughLiteral calls the stored literal, naming no read.
func (b *Bench) plantedReadThroughLiteral() []byte {
	data, _ := plantHook3(b.Root)
	return data
}
