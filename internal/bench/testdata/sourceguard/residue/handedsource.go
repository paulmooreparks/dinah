package bench

// plantedHolder2 keeps a Source in a field a method reads through.
type plantedHolder2 struct {
	src Source
}

var plantedHeld2 plantedHolder2

// plantKeep stores whatever Source it is handed.
func plantKeep(s Source) { plantedHeld2.src = s }

// PlantHandDisk is a free function no root reaches, which hands a Disk to a
// Source parameter. The placement rule allows exactly that, and the bottom
// rule judges only what a root reaches, so the method below reads the disk
// with nothing the guard can see. It is kept as the reproduction of a gap the
// guard states rather than closes.
func PlantHandDisk() { plantKeep(Disk{}) }

// plantedReadThroughHandedSource reads through the stored Source.
func (b *Bench) plantedReadThroughHandedSource() ([]byte, error) {
	return plantedHeld2.src.ReadFile(b.Root + "/workbench.md")
}
