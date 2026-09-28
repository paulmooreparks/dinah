package verb

import "dinah/internal/durable"

// want: reads dinah/internal/durable.ReadFile

// plantedDurableRead reads a card's anchor through durable, which every open
// outside durable goes through, rather than through the bench.
func (l *Library) plantedDurableRead(path string) ([]byte, error) {
	return durable.ReadFile(path)
}
