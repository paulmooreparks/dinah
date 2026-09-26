package bench

import "os"

// want: os.ReadFile

// plantHook4 is called by a method; nothing but PlantConfigure4 fills it.
var plantHook4 func(string) ([]byte, error)

// plantMakeReader is called, not named as a value, and returns a literal that
// reads, which a caller no root reaches stores where a method calls it.
func plantMakeReader() func(string) ([]byte, error) {
	return func(p string) ([]byte, error) { return os.ReadFile(p) }
}

// PlantConfigure4 is an exported free function no root reaches.
func PlantConfigure4() { plantHook4 = plantMakeReader() }

// plantedReadThroughMadeReader calls the stored literal.
func (b *Bench) plantedReadThroughMadeReader() []byte {
	data, _ := plantHook4(b.Root)
	return data
}
