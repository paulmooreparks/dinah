package bench

// want: bottom rule: dinah/internal/bench.Exists

// plantedExists calls the free reader, which binds Disk.
func (b *Bench) plantedExists() bool {
	return Exists(b.Root)
}
