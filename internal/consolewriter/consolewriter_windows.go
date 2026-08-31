//go:build windows

package consolewriter

import (
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

func defaultProbe() probe { return winProbe{} }

// winProbe is the production probe: GetConsoleMode (via term.IsTerminal)
// detects a real console, WriteConsole puts UTF-16 text on one. Neither
// call reads or changes console state that outlives this process.
type winProbe struct{}

func (winProbe) isConsole(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

func (winProbe) writeUTF16(f *os.File, u []uint16) (int, error) {
	var written uint32
	err := windows.WriteConsole(windows.Handle(f.Fd()), &u[0], uint32(len(u)), &written, nil)
	return int(written), err
}
