package bench

// want: bottom rule

// plantField holds a Disk in a named field and reads through it in a method
// of its own.
type plantField struct{ d Disk }

// ReadFile reads the disk through the held Disk.
func (w plantField) ReadFile(path string) ([]byte, error) {
	return w.d.ReadFile(path)
}

// plantedReadThroughWrapper reads the disk through the wrapper.
func (b *Bench) plantedReadThroughWrapper() ([]byte, error) {
	return plantField{}.ReadFile(b.Root + "/workbench.md")
}
