//go:build linux

package keyboard

import "golang.org/x/sys/unix"

// The requests golang.org/x/term itself uses on Linux to read and write a
// terminal's settings, which are the implementations of tcgetattr() and
// tcsetattr().
const (
	termiosRead  = unix.TCGETS
	termiosWrite = unix.TCSETS
)

// flushInput discards the input the terminal has received and not yet been
// read, which is tcflush with TCIFLUSH, the request Linux names TCFLSH.
// POSIX documents TCIFLUSH as discarding "data received but not read".
func flushInput(fd int) error {
	return unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIFLUSH)
}
