// Package pty runs a program in a pseudoconsole: the program sees a real
// terminal, and the caller reads what it draws and writes what is typed, as
// bytes of text and VT sequences. Dinah's Terminal runs shells this way.
package pty

import (
	"errors"
	"time"
)

// CloseGrace is how long Close lets a program end on its own, saving its
// history and exiting cleanly, before it ends the program itself.
const CloseGrace = 5 * time.Second

// ConsoleCloseLimit is the longest Close waits, once the program has ended, for
// the console to finish closing, which needs its output to be read.
const ConsoleCloseLimit = 30 * time.Second

// Command is a program to run, with its arguments, folder and environment.
// Args[0] is the program's own name, as os/exec's Cmd.Args has it.
type Command struct {
	Path string
	Args []string
	Dir  string
	Env  []string
}

// ErrUnsupported is returned where Dinah has no pseudoconsole yet.
var ErrUnsupported = errors.New("the Terminal's shells are not supported on this system yet")
