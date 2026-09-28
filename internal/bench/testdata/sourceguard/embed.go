package bench

// want: bottom rule

// plantWrap embeds Disk, so its ReadFile is Disk's, promoted.
type plantWrap struct{ Disk }

// plantedReadThroughEmbedding reads the disk through the promoted method.
func (b *Bench) plantedReadThroughEmbedding() ([]byte, error) {
	return plantWrap{}.ReadFile(b.Root + "/workbench.md")
}
