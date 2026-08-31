// Package consolewriter wraps a stream so that text reaching a Windows
// console prints correctly regardless of the console's active code page,
// while leaving output that is not attached to a console (a redirected
// file, a pipe) byte-identical to what was written. On every other GOOS
// it is a transparent pass-through, because the defect it exists to fix is
// specific to the Windows console (dinah-199): nothing else in this
// repository decodes bytes as a legacy code page.
package consolewriter

import (
	"errors"
	"os"
	"unicode/utf16"
	"unicode/utf8"
)

// consoleWriteChunk bounds how many UTF-16 code units Write submits to the
// probe in a single call. WriteConsole's own documented parameter section
// states nNumberOfCharsToWrite "fails with ERROR_NOT_ENOUGH_MEMORY" when
// the requested size "exceeds the available heap"; Microsoft documents no
// fixed numeric ceiling, so this value is not a claimed OS limit, it is a
// conservative engineering margin chosen to stay comfortably clear of that
// documented failure mode on any real console. Write always honors the
// probe's reported count and loops rather than assuming a chunk completes
// in full, so truncation cannot happen silently regardless of where a
// real boundary sits (D-7).
const consoleWriteChunk = 8000

// errShortConsoleWrite is returned when a probe reports zero characters
// written for a non-empty request without an error, which Write treats as
// a failure rather than looping forever.
var errShortConsoleWrite = errors.New("consolewriter: write reported zero characters written")

// probe is how Writer asks whether a stream is presently a real console and
// how it puts text on one. winProbe (consolewriter_windows.go) answers both
// with the two documented Win32 calls this package is built on. neverProbe
// (consolewriter_other.go) answers isConsole false unconditionally, which is
// what makes Writer a pass-through on every non-Windows GOOS. A test in this
// package supplies a third implementation so both branches of Write run
// without a console attached anywhere.
type probe interface {
	// isConsole reports whether f is presently a real console screen
	// buffer, as opposed to a redirected file or pipe.
	isConsole(f *os.File) bool
	// writeUTF16 puts as much of u on the console f is attached to as one
	// underlying console-write call accepts, and reports how many of the
	// leading elements of u were actually written, mirroring
	// WriteConsole's own documented lpNumberOfCharsWritten out-parameter.
	// Called only when isConsole(f) is true. A non-nil error may still
	// carry a non-zero count if some units were written before failure.
	writeUTF16(f *os.File, u []uint16) (int, error)
}

// Writer wraps a stream, normally os.Stdout or os.Stderr, so every Write
// call either passes bytes straight through unchanged (f is not a console)
// or is re-encoded to UTF-16 and written with the platform's documented
// console-write call, looping over consoleWriteChunk-sized pieces until
// every unit is written (f is a console).
//
// On the console branch, Write does not require its caller to hand it a
// complete, self-contained UTF-8 sequence in one call. If p ends with a
// truncated multi-byte sequence, one that a later Write call could still
// complete, Writer holds those trailing bytes back and prepends them to
// the next Write call's input before encoding, instead of encoding the
// truncated tail as a replacement character. This makes Write safe for
// every caller in this tree without auditing any of them, and safe for a
// caller added later without reading this package's contract first (D-8).
//
// The held-back bytes are internal state, not lost input: Write always
// reports len(p) consumed on success, whether or not a trailing piece of
// it was buffered rather than emitted immediately. Call Flush once, after
// the last Write to this stream and before the process exits, so a stream
// that legitimately ends mid-sequence does not silently drop its last few
// bytes (see Flush).
//
// The non-console branch never buffers: every byte handed to Write on
// that branch reaches the underlying file in the same call, unchanged,
// exactly as before.
//
// Every write is treated as one complete unit on success: Write reports
// len(p) written once every UTF-16 unit derived from this call and any
// previously held-back bytes has been accepted. On the console branch's
// own failure, Write reports 0 and the error, because a partial UTF-16
// write has no well-defined byte offset back into p once encoding has
// interleaved multi-unit runes (surrogate pairs, multi-byte code points)
// with single-unit ones. A failure does not lose the held-back bytes from
// a prior call; they were already folded into this call's input before
// the failure and are not held a second time.
type Writer struct {
	f       *os.File
	probe   probe
	pending []byte // trailing incomplete UTF-8 sequence held from a previous Write
}

// New wraps f. f is normally os.Stdout or os.Stderr.
func New(f *os.File) *Writer {
	return &Writer{f: f, probe: defaultProbe()}
}

// File returns the writer's own unwrapped stream. A caller handing the
// stream to a child process it does not control (cmd/dinah's runEdit is
// the one case in this tree) uses File instead of the Writer itself, so
// the child gets a real console handle rather than the pipe exec.Cmd would
// build around an io.Writer that is not an *os.File.
func (w *Writer) File() *os.File {
	return w.f
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	if !w.probe.isConsole(w.f) {
		return w.f.Write(p)
	}
	if len(p) == 0 && len(w.pending) == 0 {
		return 0, nil
	}
	buf := append(w.pending, p...)
	w.pending = nil
	complete, incomplete := splitIncompleteTail(buf)
	if len(incomplete) > 0 {
		w.pending = append([]byte(nil), incomplete...)
	}
	if len(complete) == 0 {
		return len(p), nil
	}
	if err := w.writeConsole(complete); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Flush writes any bytes Write has held back because they ended a call
// with an incomplete multi-byte UTF-8 sequence. Call it once, after all
// writing to this stream is done and before the process exits.
//
// No call site in this repository is known to write a partial sequence,
// and Writer no longer depends on that being true. Establishing it would
// take an audit of every caller and of the libraries they write through,
// which is the per-site precondition D-8 removed rather than proved: the
// buffer above holds a truncated tail whoever produced it, so Flush is
// what bounds the buffer at end of stream for callers nobody has audited
// and for callers not yet written.
//
// Flush does not close the underlying file. This type wraps os.Stdout and
// os.Stderr, and this package must never close either.
//
// On the non-console branch, Flush is always a no-op: Write never buffers
// anything on that branch, so there is never anything pending to flush.
//
// A sequence still pending at Flush time can never be completed, because
// nothing further is coming, and the design cannot tell at Flush time
// whether the stream ended mid-sequence because something upstream
// genuinely truncated it or because the process is exiting right after a
// last write that happened to end there. Flush writes the pending bytes
// through the same console-encode path an ordinary Write would use.
// unicode/utf8 documents that converting invalid UTF-8 to runes (which is
// what []rune(string(p)) does, and what this package's encoding step has
// always done) yields the replacement rune U+FFFD for the invalid bytes;
// a truncated trailing sequence is invalid UTF-8 by definition, so it
// flushes as one or more U+FFFD characters, which is what the console
// branch does with invalid UTF-8 anywhere else.
func (w *Writer) Flush() error {
	if len(w.pending) == 0 {
		return nil
	}
	pending := w.pending
	w.pending = nil
	if !w.probe.isConsole(w.f) {
		return nil
	}
	return w.writeConsole(pending)
}

// writeConsole re-encodes b as UTF-16 and submits it to the probe in
// pieces of at most consoleWriteChunk units, honoring the probe's
// reported count and looping until every unit is written. b must not end
// with a byte sequence Write or Flush would otherwise have held back;
// both callers arrange that before calling writeConsole.
//
// No piece ends between the two units of a surrogate pair, which is what
// chunkEnd is for. Write already rejoins a character a caller split
// across two of its own calls, and a fixed cut every consoleWriteChunk
// units would divide one back apart a layer further down.
func (w *Writer) writeConsole(b []byte) error {
	units := utf16.Encode([]rune(string(b)))
	if len(units) == 0 {
		return nil
	}
	offset := 0
	for offset < len(units) {
		end := chunkEnd(units, offset)
		n, err := w.probe.writeUTF16(w.f, units[offset:end])
		if err != nil {
			return err
		}
		if n == 0 {
			return errShortConsoleWrite
		}
		offset += n
	}
	return nil
}

// chunkEnd returns the index one past the last unit writeConsole submits
// in the call starting at offset. It is offset+consoleWriteChunk, clamped
// to the length of units, and then backed off by one unit when that cut
// would fall between the two halves of a surrogate pair.
//
// The boundary test asks the encoding rather than a numeric range.
// utf16.DecodeRune "returns the UTF-16 decoding of a surrogate pair", and
// "if the pair is not a valid UTF-16 surrogate pair, DecodeRune returns
// the Unicode replacement code point U+FFFD". So when the two units
// either side of a candidate cut decode to anything but U+FFFD, those two
// units are one character and the cut is inside it. U+FFFD is itself in
// the Basic Multilingual Plane and no surrogate pair encodes it, so a
// genuine pair never answers with the sentinel.
//
// Backing off cannot empty the chunk: consoleWriteChunk is well above 1,
// so a chunk shortened by one unit still carries units to write, and the
// unit given up opens the next call.
//
// Where the console consumes less than it was offered, the next call
// resumes at whatever count the console reported, which can itself fall
// inside a pair. That count is the console's to choose and this package
// only reports it; the cut at consoleWriteChunk is the one it picks.
func chunkEnd(units []uint16, offset int) int {
	end := offset + consoleWriteChunk
	if end >= len(units) {
		return len(units)
	}
	if utf16.DecodeRune(rune(units[end-1]), rune(units[end])) != utf8.RuneError {
		end--
	}
	return end
}

// splitIncompleteTail splits buf into a leading complete portion and a
// trailing incomplete portion. The trailing portion is non-empty only
// when buf ends with the truncated start of a multi-byte UTF-8 sequence
// that more bytes could still complete. It is never non-empty for a
// sequence that is simply invalid.
//
// The check relies on two documented unicode/utf8 guarantees and nothing
// else. RuneStart(b) "reports whether the byte could be the first byte of
// an encoded, possibly invalid rune"; a continuation byte never answers
// true, so scanning backward from the end of buf for the first byte
// RuneStart accepts finds the start of the last rune-or-attempt in buf.
// FullRune(p) "reports whether the bytes in p begin with a full UTF-8
// encoding of a rune", and its doc comment states plainly that "An
// invalid encoding is considered a full Rune since it will convert as a
// width-1 error rune", so FullRune returning false is specifically the
// truncated-valid-prefix case this function exists to catch, never the
// malformed case, which FullRune already reports as full.
func splitIncompleteTail(buf []byte) (complete, incomplete []byte) {
	n := len(buf)
	limit := utf8.UTFMax - 1
	if limit > n {
		limit = n
	}
	for i := 1; i <= limit; i++ {
		b := buf[n-i]
		if !utf8.RuneStart(b) {
			continue
		}
		if !utf8.FullRune(buf[n-i:]) {
			return buf[:n-i], buf[n-i:]
		}
		break
	}
	return buf, nil
}
