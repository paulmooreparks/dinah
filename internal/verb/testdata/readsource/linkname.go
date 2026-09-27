package verb

import _ "unsafe"

// want: code outside Go: //go:linkname directive

// plantRead is package bench's readText under verb's name, bound by the
// linker, so every use of it resolves to verb's own object.
//
//go:linkname plantRead dinah/internal/bench.readText
func plantRead(p string) ([]byte, error)

func plantReadThroughLinkname(p string) ([]byte, error) { return plantRead(p) }
