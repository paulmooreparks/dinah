package screen

import (
	"io"
	"sync"
	"sync/atomic"
)

// Sink receives what a reader decodes, on the reader's own goroutine.
type Sink interface {
	// Event is one key or one whole paste.
	Event(event Event)
	// Resized says the console reported a change of its buffer size, so
	// the window's size is worth reading again.
	Resized()
	// Flushed confirms a flush RequestFlush asked for, naming the
	// generation every later event carries.
	Flushed(gen uint64)
}

// Reader is one source of keys for the terminal head: a console, a POSIX
// terminal, or a stream standing in for either under test.
type Reader interface {
	// Run reads and decodes until Stop is called or reading fails, handing
	// everything it decodes to the sink.
	Run(sink Sink) error
	// Stop ends Run. It is safe to call more than once, and from any
	// goroutine.
	Stop()
	// RequestFlush asks Run to discard the input already waiting, to clear
	// the decoder's unfinished input, and to confirm through the sink.
	RequestFlush()
	// SawPaste reports whether a paste start marker has been read, which
	// shows the terminal honours the bracketed-paste request.
	SawPaste() bool
}

// streamReader feeds the POSIX decoder from a stream, which is how the test
// seam delivers keys on every GOOS. A flush discards the chunks already read
// and not yet decoded, and reports itself to the discard hook.
type streamReader struct {
	source   io.Reader
	decoder  *KeyDecoder
	discard  func()
	stop     chan struct{}
	flush    chan struct{}
	once     sync.Once
	sawPaste atomic.Bool
	gen      uint64
}

// NewStreamReader builds a reader decoding source with entry's key strings.
// discard, when not nil, is called on every flush, before the confirmation
// is sent.
func NewStreamReader(source io.Reader, entry *Terminfo, discard func()) Reader {
	return &streamReader{
		source:  source,
		decoder: NewKeyDecoder(entry),
		discard: discard,
		stop:    make(chan struct{}),
		flush:   make(chan struct{}, 1),
	}
}

// Run reads the stream on a goroutine of its own and decodes each chunk here.
// The end of the stream ends reading and nothing else: flushes are still
// served until Stop, so a model waiting on a confirmation always gets one.
func (r *streamReader) Run(sink Sink) error {
	chunks := make(chan []byte, 16)
	go func() {
		defer close(chunks)
		buf := make([]byte, 4096)
		for {
			n, err := r.source.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				select {
				case chunks <- chunk:
				case <-r.stop:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	for {
		select {
		case <-r.stop:
			return nil
		case <-r.flush:
			r.drain(chunks)
			if r.discard != nil {
				r.discard()
			}
			r.gen++
			r.decoder.Reset(r.gen)
			sink.Flushed(r.gen)
		case chunk, open := <-chunks:
			if !open {
				chunks = nil
				continue
			}
			events := r.decoder.Feed(chunk)
			if r.decoder.SawPaste() {
				r.sawPaste.Store(true)
			}
			for _, event := range events {
				sink.Event(event)
			}
		}
	}
}

// drain discards every chunk already read and not yet decoded.
func (r *streamReader) drain(chunks chan []byte) {
	for {
		select {
		case _, open := <-chunks:
			if !open {
				return
			}
		default:
			return
		}
	}
}

// Stop ends Run.
func (r *streamReader) Stop() {
	r.once.Do(func() { close(r.stop) })
}

// RequestFlush asks Run to flush. A request already waiting absorbs this one.
func (r *streamReader) RequestFlush() {
	select {
	case r.flush <- struct{}{}:
	default:
	}
}

// SawPaste reports whether a paste start marker has been read.
func (r *streamReader) SawPaste() bool {
	return r.sawPaste.Load()
}
