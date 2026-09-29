package bench

// want: bottom rule: (dinah/internal/bench.plantOther).source

// plantOther has a method named source, as the seam function is, answering a
// Disk. The seam is keyed on (*Bench).source's object, so this one is judged.
type plantOther struct{}

func (plantOther) source() Source { return Disk{} }

// plantedReadThroughOtherSource reads the disk through the other source.
func (b *Bench) plantedReadThroughOtherSource() ([]byte, error) {
	return plantOther{}.source().ReadFile(b.Root + "/workbench.md")
}
