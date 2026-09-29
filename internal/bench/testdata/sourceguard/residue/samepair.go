package bench

import "os"

// plantSamePair makes two reads of one member. The guard's run for this plant
// excuses the pair (plantSamePair, os.ReadFile) for the first, and an
// exemption is keyed on the pair, so the second passes with it. It is kept as
// the reproduction of a gap the guard states rather than closes.
func (b *Bench) plantSamePair() ([]byte, error) {
	if _, err := os.ReadFile(b.Root + "/definition.md"); err != nil {
		return nil, err
	}
	return os.ReadFile(b.Root + "/workbench.md")
}
