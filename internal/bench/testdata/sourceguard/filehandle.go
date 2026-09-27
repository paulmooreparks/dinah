package bench

import "os"

// want: reads os.File.Read

// PlantFromFile reads through an *os.File opened elsewhere and handed in. The
// name-keyed guard stated this as a gap, because os.File is allowed as the
// type every write goes through; the member rule judges each method of a
// judged type by its own name, and (*os.File).Read is a read.
func (b *Bench) PlantFromFile(f *os.File) ([]byte, error) {
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	return buf[:n], err
}
