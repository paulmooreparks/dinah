//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package keyboard

import "golang.org/x/sys/unix"

// The requests golang.org/x/term itself uses on Darwin and the BSDs to read
// and write a terminal's settings, which are the implementations of
// tcgetattr() and tcsetattr().
const (
	termiosRead  = unix.TIOCGETA
	termiosWrite = unix.TIOCSETA
)

// flushRead is FREAD from <sys/fcntl.h>, the bit tty(4) documents for
// TIOCFLUSH as selecting the input queue. golang.org/x/sys/unix does not
// export it.
const flushRead = 0x0001

// flushInput discards the input the terminal has received and not yet been
// read. These systems have no TCFLSH request, and tty(4) documents TIOCFLUSH,
// given a pointer to an int holding FREAD, as flushing "all characters in the
// input queue".
func flushInput(fd int) error {
	return unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, flushRead)
}
