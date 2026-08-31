//go:build !windows

package consolewriter

import "os"

func defaultProbe() probe { return neverProbe{} }

// neverProbe makes Writer a pass-through on every non-Windows GOOS. A POSIX
// terminal under a UTF-8 locale already renders this tool's output
// correctly; the mangling this package fixes is specific to the Windows
// console's code-page decoding, which no other platform does.
type neverProbe struct{}

func (neverProbe) isConsole(f *os.File) bool                      { return false }
func (neverProbe) writeUTF16(f *os.File, u []uint16) (int, error) { return len(u), nil }
