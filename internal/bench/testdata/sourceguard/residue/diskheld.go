package bench

// plantedSourceHolder keeps a source in a field a method reads through.
type plantedSourceHolder struct {
	src Disk
}

var plantedSourceHeld plantedSourceHolder

// PlantDisk is a free function no root reaches, which builds a Disk and stores
// it. Only this function names Disk, and the walk never starts from it, so
// the method below reads the disk with nothing the guard can see. It is kept
// as the reproduction of a gap the guard states rather than closes.
func PlantDisk() { plantedSourceHeld.src = Disk{} }

// plantedReadThroughHeldDisk reads through the held Disk.
func (b *Bench) plantedReadThroughHeldDisk() []byte {
	data, _ := plantedSourceHeld.src.ReadFile("workbench.md")
	return data
}
