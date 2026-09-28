package bench

import "dinah/internal/durable"

// want: durable.ReadFile

// plantedDurableRead reads the workbench's anchor through durable, which
// every open outside durable goes through, rather than through the source.
func (b *Bench) plantedDurableRead() ([]byte, error) {
	return durable.ReadFile(b.Root)
}
