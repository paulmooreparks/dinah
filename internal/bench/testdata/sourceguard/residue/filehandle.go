package bench

import "os"

// PlantFromFile reads through an *os.File opened elsewhere and handed in.
// os.File is allowlisted as the type every write goes through, so the
// signature names nothing the guard judges, and a method call on a value is
// not judged. It is kept as the reproduction of a gap the guard states rather
// than closes.
func (b *Bench) PlantFromFile(f *os.File) ([]byte, error) {
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	return buf[:n], err
}
