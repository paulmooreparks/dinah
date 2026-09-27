package bench

// want: bottom rule

// plantRaw is Disk under another name. A guard keyed on the spelling Disk
// reads neither this declaration nor the call below as naming it
// (dinah-619/comments/18).
type plantRaw = Disk

// plantedReadThroughAlias reads the disk through the alias.
func (b *Bench) plantedReadThroughAlias() ([]byte, error) {
	return plantRaw{}.ReadFile(b.Root + "/workbench.md")
}
