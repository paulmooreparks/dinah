package bench

import "os"

// want: reads os.Stat

// plantedStat reads the disk directly from a method of Bench.
func (b *Bench) plantedStat() bool {
	_, err := os.Stat(b.Root)
	return err == nil
}
