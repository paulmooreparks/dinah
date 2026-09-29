package bench

import "io"

// PlantFromReader reads through an io.Reader its caller handed it, which the
// caller built from an *os.File. package io is not judged, because
// converting a judged handle to an interface of another package is also how
// every write is made (fmt.Fprintf(f, ...)), so this passes. It is kept as
// the reproduction of a gap the guard states rather than closes.
func (b *Bench) PlantFromReader(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
