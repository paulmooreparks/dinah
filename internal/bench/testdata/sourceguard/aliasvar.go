package bench

// want: dinah/internal/bench.plantSrc

// plantRaw2 is Disk under another name, held in a package variable as a
// Source, so the method below calls a method of Source, which is a leaf of
// the walk.
type plantRaw2 = Disk

var plantSrc Source = plantRaw2{}

// plantedReadThroughHeldSource reads the disk through the variable.
func (b *Bench) plantedReadThroughHeldSource() ([]byte, error) {
	return plantSrc.ReadFile(b.Root + "/workbench.md")
}
