package bench

// want: placement rule

// plantedHolder keeps a Disk in a field a method reads through.
type plantedHolder struct {
	src Disk
}

var plantedHeld plantedHolder

// PlantDisk is a free function no root reaches, which builds a Disk and
// stores it. The name-keyed guard stated this as a gap it could not close;
// the placement rule refuses the assignment wherever it stands.
func PlantDisk() { plantedHeld.src = Disk{} }

// plantedReadThroughHeldDisk reads through the held Disk.
func (b *Bench) plantedReadThroughHeldDisk() []byte {
	data, _ := plantedHeld.src.ReadFile("workbench.md")
	return data
}
