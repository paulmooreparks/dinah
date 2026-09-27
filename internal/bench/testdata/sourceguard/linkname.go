package bench

import _ "unsafe"

// want: code outside Go: //go:linkname directive

// plantRead is os.ReadFile under another name, bound by the linker. Every use
// of it resolves to this package's own object, so no graph of Go syntax can
// see the read behind it.
//
//go:linkname plantRead os.ReadFile
func plantRead(name string) ([]byte, error)

// plantedReadThroughLinkname reads the disk through the pulled symbol.
func (b *Bench) plantedReadThroughLinkname() ([]byte, error) {
	return plantRead(b.Root + "/workbench.md")
}
