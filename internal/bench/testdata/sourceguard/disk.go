package bench

// want: names Disk

// plantedDisk constructs Disk itself rather than reading through the bench's
// source.
func (b *Bench) plantedDisk() ([]byte, error) {
	return Disk{}.ReadFile(b.Root)
}
