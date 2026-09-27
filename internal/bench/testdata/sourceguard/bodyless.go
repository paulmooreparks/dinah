package bench

// want: code outside Go: function declared without a body

// plantAsm is implemented outside Go, as an assembly routine would be, so no
// graph of Go syntax can see what it reads.
func plantAsm(path string) ([]byte, error)

// plantedReadThroughAssembly reads the disk through the bodyless function.
func (b *Bench) plantedReadThroughAssembly() ([]byte, error) {
	return plantAsm(b.Root + "/workbench.md")
}
