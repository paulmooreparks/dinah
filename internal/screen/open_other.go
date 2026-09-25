//go:build !windows

package screen

import (
	"io"
	"os"

	"golang.org/x/term"
)

// isTerminal reports whether f is a terminal, which golang.org/x/term's
// IsTerminal answers by asking the terminal driver for its attributes.
func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// openTerminal opens the terminfo layer on the terminal TERM names. XBD
// chapter 8 defines TERM as "the terminal type for which output is to be
// prepared", and a terminal whose description cannot be found is opened
// with none.
func openTerminal(f *os.File, text io.Writer) Screen {
	entry, err := LoadTerminfo(os.Getenv("TERM"), os.Getenv)
	if err != nil {
		entry = nil
	}
	return NewTerminfo(entry, text)
}
