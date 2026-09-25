//go:build !windows && !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package keyboard

import (
	"errors"
	"os"

	"golang.org/x/term"

	"dinah/internal/screen"
)

// errNoKeyboard is the answer on a system the key reader has no documented
// terminal interface for.
var errNoKeyboard = errors.New("keyboard: this system has no key reader for the terminal head")

// Keyboard is never built on these systems.
type Keyboard struct{}

// EnterKeyboard refuses, because the head reads keys through termios and
// poll on the systems it supports and has no reader here.
func EnterKeyboard(in, out *os.File, entry *screen.Terminfo) (*Keyboard, error) {
	return nil, errNoKeyboard
}

// Leave does nothing.
func (k *Keyboard) Leave() error { return nil }

// NewReader refuses.
func (k *Keyboard) NewReader() (screen.Reader, error) { return nil, errNoKeyboard }

// WindowSize reports no size, so the head refuses before it enters.
func WindowSize(out *os.File) (width, height int, err error) { return 0, 0, errNoKeyboard }

// IsTerminal reports whether f is a terminal.
func IsTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }
