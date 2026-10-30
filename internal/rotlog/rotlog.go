// Package rotlog is a buffered log file that rotates itself. Every write goes
// into a buffer in memory, never to the disk directly; the buffer is written
// to the file when it grows past a threshold, a moment after the last write,
// and on Flush and Close. Before the buffer is written, a file that would pass
// its size limit is renamed (name.log becomes name.1.log, the older ones move
// up, and the oldest beyond the number kept is removed), a new file is
// created, and the buffer is written to the new one. A log is never allowed to
// grow without limit, and a slow or full disk never blocks the writer: while
// the file cannot be written the buffer keeps what it can up to its own limit,
// and a line in the file says how much was dropped once writing works again.
//
// The file is open only while the buffer is written to it. On Windows an open
// file cannot be renamed or removed, so a writer that held its file open
// would stop the person, a pruning of old logs or a test's clean-up from
// removing it; and a program with many chats would hold many files open.
//
// A writer creates its file's own folder, but not the folders above it: a
// log whose folder tree was removed, such as when a test's temporary folder
// is cleaned up, does not bring it back.
package rotlog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Defaults for a writer that leaves them unset.
const (
	DefaultMaxSize   = 8 << 20
	DefaultKeep      = 4
	DefaultFlushSize = 64 << 10
	DefaultDelay     = time.Second
	DefaultMaxBuffer = 8 << 20
)

// Options configure a writer.
type Options struct {
	// MaxSize is the size a file may reach before it is rotated.
	MaxSize int64
	// Keep is how many files are kept, the current one included; at least 1.
	Keep int
	// FlushSize is how large the buffer may grow before it is written at once.
	// A FlushSize of 1 writes every write at once.
	FlushSize int
	// Delay is how long after a write the buffer is written when it has not
	// reached FlushSize.
	Delay time.Duration
	// MaxBuffer is how much the buffer keeps while the file cannot be
	// written; what does not fit is dropped and counted.
	MaxBuffer int
}

// Immediate, when set before writers are opened, makes every writer that
// leaves FlushSize unset write each write at once, with no timer. Tests set
// it so that nothing is written after they end.
var Immediate bool

// Writer is one rotating log file. It is safe for concurrent use.
type Writer struct {
	path string
	opt  Options

	mu      sync.Mutex
	buf     bytes.Buffer
	dropped int64
	timer   *time.Timer
	closed  bool
	lastErr error
}

// Open returns a writer for the file at path. Nothing is read or created
// until the first flush.
func Open(path string, opt Options) *Writer {
	if opt.MaxSize <= 0 {
		opt.MaxSize = DefaultMaxSize
	}
	if opt.Keep < 1 {
		opt.Keep = DefaultKeep
	}
	if opt.FlushSize <= 0 {
		opt.FlushSize = DefaultFlushSize
		if Immediate {
			opt.FlushSize = 1
		}
	}
	if opt.Delay <= 0 {
		opt.Delay = DefaultDelay
	}
	if opt.MaxBuffer <= 0 {
		opt.MaxBuffer = DefaultMaxBuffer
	}
	return &Writer{path: path, opt: opt}
}

// Path is the current file's path.
func (w *Writer) Path() string { return w.path }

// Write adds p to the buffer. It never fails and never waits for the disk,
// except that a buffer past FlushSize is written before Write returns.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return len(p), nil
	}
	if w.buf.Len()+len(p) > w.opt.MaxBuffer {
		w.dropped += int64(len(p))
		return len(p), nil
	}
	w.buf.Write(p)
	if w.buf.Len() >= w.opt.FlushSize {
		_ = w.flushLocked()
	} else if w.timer == nil {
		w.timer = time.AfterFunc(w.opt.Delay, func() { _ = w.Flush() })
	}
	return len(p), nil
}

// WriteString adds s to the buffer.
func (w *Writer) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

// Flush writes the buffer to the file now, rotating it first when needed.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

// Close writes what is buffered. Later writes are ignored.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	err := w.flushLocked()
	w.closed = true
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	return err
}

// Err is the last error writing the file met, or nil once a write succeeded.
func (w *Writer) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastErr
}

func (w *Writer) flushLocked() error {
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	if w.buf.Len() == 0 && w.dropped == 0 {
		return nil
	}
	if w.dropped > 0 {
		note := fmt.Appendf(nil, "[%s rotlog: %d bytes were dropped while the log could not be written]\n", time.Now().Format(time.RFC3339), w.dropped)
		w.buf = *bytes.NewBuffer(append(note, w.buf.Bytes()...))
		w.dropped = 0
	}
	err := w.writeLocked()
	if err != nil {
		w.lastErr = err
		if !w.closed && w.timer == nil {
			w.timer = time.AfterFunc(5*w.opt.Delay, func() { _ = w.Flush() })
		}
		return err
	}
	w.lastErr = nil
	return nil
}

// writeLocked writes the buffer to the file, rotating first when the file
// would pass its limit, and closes the file again.
func (w *Writer) writeLocked() error {
	dir := filepath.Dir(w.path)
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	var size int64
	if info, err := os.Stat(w.path); err == nil {
		size = info.Size()
	}
	if size > 0 && size+int64(w.buf.Len()) > w.opt.MaxSize {
		if err := w.rotate(); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	n, err := f.Write(w.buf.Bytes())
	w.buf.Next(n)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	w.buf.Reset()
	return nil
}

// Rotated is the name of the i-th older file of path: name.1.log for
// name.log, and so on; the 0th is path itself.
func Rotated(path string, i int) string {
	if i == 0 {
		return path
	}
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + fmt.Sprintf(".%d", i) + ext
}

func (w *Writer) rotate() error {
	if w.opt.Keep == 1 {
		if err := os.Remove(w.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	_ = os.Remove(Rotated(w.path, w.opt.Keep-1))
	for i := w.opt.Keep - 2; i >= 0; i-- {
		if err := os.Rename(Rotated(w.path, i), Rotated(w.path, i+1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
