//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package keyboard

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"dinah/internal/screen"
)

// rawInput answers the termios the interface reads keys under: raw input,
// with output processing on.
//
// Every flag it changes is one POSIX.1-2017 defines in the Base Definitions,
// chapter 11, "General Terminal Interface". It clears ICANON, so input is
// available byte by byte; ECHO, so keys are not drawn over the frame; ISIG,
// so Ctrl+C, Ctrl+\ and Ctrl+Z arrive as bytes; and IEXTEN, so no character
// is consumed by the terminal. It clears IXON, so Ctrl+S and Ctrl+Q arrive as
// bytes; ICRNL, INLCR and IGNCR, so CR and LF arrive as sent; ISTRIP, so
// UTF-8 arrives whole; and BRKINT, so a break is not turned into a signal. It
// sets VMIN to 1 and VTIME to 0, so a read returns once one byte is
// available with no timer. It sets OPOST and ONLCR, so each line feed Bubble
// Tea writes reaches the screen as CR LF, which is what keeps a frame from
// staircasing, because with its input disabled Bubble Tea writes a bare line
// feed between rows. It changes nothing else.
func rawInput(t unix.Termios) unix.Termios {
	t.Lflag &^= unix.ICANON | unix.ECHO | unix.ISIG | unix.IEXTEN
	t.Iflag &^= unix.IXON | unix.ICRNL | unix.INLCR | unix.IGNCR | unix.ISTRIP | unix.BRKINT
	t.Oflag |= unix.OPOST | unix.ONLCR
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
	return t
}

// Keyboard is the terminal's input side while the terminal head runs: the
// settings it found, which Leave puts back, and the keypad strings of its
// terminfo entry.
type Keyboard struct {
	in, out    *os.File
	saved      *unix.Termios
	entry      *screen.Terminfo
	keypadOff  string
	keypadWent bool
}

// EnterKeyboard takes the terminal's keyboard for the terminal head. It reads
// the terminal's settings, writes rawInput's answer back, writes smkx where
// the entry has one, and then writes the request for bracketed paste.
func EnterKeyboard(in, out *os.File, entry *screen.Terminfo) (*Keyboard, error) {
	fd := int(in.Fd())
	saved, err := unix.IoctlGetTermios(fd, termiosRead)
	if err != nil {
		return nil, err
	}
	raw := rawInput(*saved)
	if err := unix.IoctlSetTermios(fd, termiosWrite, &raw); err != nil {
		return nil, err
	}
	k := &Keyboard{in: in, out: out, saved: saved, entry: entry}
	on, off := KeypadTransmit(entry)
	if on != "" {
		if _, err := out.WriteString(on); err != nil {
			unix.IoctlSetTermios(fd, termiosWrite, saved)
			return nil, err
		}
		k.keypadOff = off
		k.keypadWent = true
	}
	if _, err := out.WriteString(BracketedPasteOn); err != nil {
		k.Leave()
		return nil, err
	}
	return k, nil
}

// Leave gives the keyboard back: it writes the bracketed-paste disable, then
// rmkx where smkx was written and the entry has it, then restores the
// settings it read, whatever an earlier step answered. The first error is
// reported.
func (k *Keyboard) Leave() error {
	var first error
	keep := func(err error) {
		if first == nil {
			first = err
		}
	}
	_, err := k.out.WriteString(BracketedPasteOff)
	keep(err)
	if k.keypadWent && k.keypadOff != "" {
		_, err := k.out.WriteString(k.keypadOff)
		keep(err)
	}
	keep(unix.IoctlSetTermios(int(k.in.Fd()), termiosWrite, k.saved))
	return first
}

// ttyReader reads a POSIX terminal with poll, over standard input and the
// read end of a pipe it owns. A byte on the pipe names what is asked of the
// loop: a flush or the end.
type ttyReader struct {
	in       int
	wake     [2]int
	decoder  *KeyDecoder
	gen      uint64
	sawPaste atomic.Bool
	// mu guards stopped, so no write reaches the pipe's descriptor after
	// Stop has closed it and the number could name another file.
	mu      sync.Mutex
	stopped bool
}

// The bytes written to the wake pipe.
const (
	wakeStop  = 's'
	wakeFlush = 'f'
)

// NewReader builds the reader for the terminal the keyboard was entered on.
func (k *Keyboard) NewReader() (Reader, error) {
	var wake [2]int
	if err := unix.Pipe(wake[:]); err != nil {
		return nil, err
	}
	return &ttyReader{in: int(k.in.Fd()), wake: wake, decoder: NewKeyDecoder(k.entry)}, nil
}

// Run polls standard input and the wake pipe together, reading what is
// waiting on standard input whenever it is readable. poll(2), read(2) and
// pipe(2) are POSIX.
func (r *ttyReader) Run(sink Sink) error {
	defer unix.Close(r.wake[0])
	buf := make([]byte, 4096)
	fds := []unix.PollFd{{Fd: int32(r.in), Events: unix.POLLIN}, {Fd: int32(r.wake[0]), Events: unix.POLLIN}}
	for {
		if _, err := unix.Poll(fds, -1); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return err
		}
		if fds[1].Revents&unix.POLLIN != 0 {
			var asked [1]byte
			if _, err := unix.Read(r.wake[0], asked[:]); err != nil {
				return err
			}
			if asked[0] == wakeStop {
				return nil
			}
			flushInput(r.in)
			r.gen++
			r.decoder.Reset(r.gen)
			sink.Flushed(r.gen)
			continue
		}
		if fds[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) == 0 {
			continue
		}
		n, err := unix.Read(r.in, buf)
		if err != nil {
			if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
				continue
			}
			return err
		}
		if n == 0 {
			return nil
		}
		events := r.decoder.Feed(buf[:n])
		if r.decoder.SawPaste() {
			r.sawPaste.Store(true)
		}
		for _, event := range events {
			sink.Event(event)
		}
	}
}

// Stop ends Run by writing to the wake pipe.
func (r *ttyReader) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	r.stopped = true
	unix.Write(r.wake[1], []byte{wakeStop})
	unix.Close(r.wake[1])
}

// RequestFlush asks Run to flush by writing to the wake pipe, and does
// nothing once Stop has run.
func (r *ttyReader) RequestFlush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	unix.Write(r.wake[1], []byte{wakeFlush})
}

// SawPaste reports whether a paste start marker has been read.
func (r *ttyReader) SawPaste() bool {
	return r.sawPaste.Load()
}

// WindowSize answers the width and height of the window out is shown in,
// which golang.org/x/term reads with TIOCGWINSZ.
func WindowSize(out *os.File) (width, height int, err error) {
	return term.GetSize(int(out.Fd()))
}

// IsTerminal reports whether f is a terminal.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
