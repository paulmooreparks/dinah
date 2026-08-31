package consolewriter

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// fakeProbe stands in for a real console so both branches of Write run on a
// machine with no console attached. It records every slice writeUTF16 was
// handed, accepts at most perCall units per call when perCall is positive,
// and fails on the call whose index is failOn when that is positive.
type fakeProbe struct {
	console bool
	perCall int
	failOn  int
	failErr error
	calls   int
	chunks  [][]uint16
}

func (p *fakeProbe) isConsole(*os.File) bool { return p.console }

func (p *fakeProbe) writeUTF16(_ *os.File, u []uint16) (int, error) {
	p.calls++
	if p.failOn > 0 && p.calls == p.failOn {
		return 0, p.failErr
	}
	accepted := len(u)
	if p.perCall > 0 && p.perCall < accepted {
		accepted = p.perCall
	}
	p.chunks = append(p.chunks, append([]uint16(nil), u[:accepted]...))
	return accepted, nil
}

// received is everything the fake was handed across every call, decoded back
// to text, which is what a console would have displayed.
func (p *fakeProbe) received() string {
	var units []uint16
	for _, chunk := range p.chunks {
		units = append(units, chunk...)
	}
	return string(utf16.Decode(units))
}

// newTestWriter wraps a fresh file under the test's own directory and
// injects the fake, so no test here needs a console or touches a real
// stream.
func newTestWriter(t *testing.T, p *fakeProbe) (*Writer, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stream")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	t.Cleanup(func() { f.Close() })
	return &Writer{f: f, probe: p}, path
}

// fileBytes reads back what actually reached the underlying file.
func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return got
}

// TestWriteLeavesARedirectedStreamByteIdentical asserts that when the stream
// is not a console every byte handed to Write reaches the file unchanged,
// including bytes that are not valid UTF-8, and that the console path is not
// entered at all.
func TestWriteLeavesARedirectedStreamByteIdentical(t *testing.T) {
	p := &fakeProbe{console: false}
	w, path := newTestWriter(t, p)
	writes := [][]byte{
		[]byte("plain ascii\n"),
		[]byte("ein Bindestrich — und ein Emoji \U0001F600\n"),
		{0x41, 0xff, 0xfe, 0x80, 0xc3, 0x42},
		{},
		[]byte("éन"),
	}
	var wanted []byte
	for _, chunk := range writes {
		n, err := w.Write(chunk)
		if err != nil {
			t.Fatalf("write %q: %v", chunk, err)
		}
		if n != len(chunk) {
			t.Errorf("write %q reported %d bytes, wanted %d", chunk, n, len(chunk))
		}
		wanted = append(wanted, chunk...)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got := fileBytes(t, path); string(got) != string(wanted) {
		t.Errorf("the redirected stream received %q, wanted %q byte for byte", got, wanted)
	}
	if p.calls != 0 {
		t.Errorf("the console write was called %d times on a redirected stream, wanted none", p.calls)
	}
}

// TestWriteEncodesConsoleTextAsUTF16 asserts that when the stream is a
// console the text is re-encoded to UTF-16 and round-trips exactly, that a
// character outside the Basic Multilingual Plane arrives as a well-formed
// surrogate pair, and that nothing is written to the file directly.
func TestWriteEncodesConsoleTextAsUTF16(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"plain ascii", "a card and a column\n"},
		{"an em dash", "Ein Bindestrich — und weiter\n"},
		{"outside the BMP", "an emoji \U0001F600 and Hindi हिन्दी\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &fakeProbe{console: true}
			w, path := newTestWriter(t, p)
			if _, err := w.Write([]byte(c.text)); err != nil {
				t.Fatalf("write: %v", err)
			}
			if got := p.received(); got != c.text {
				t.Errorf("the console received %q, wanted %q", got, c.text)
			}
			if got := fileBytes(t, path); len(got) != 0 {
				t.Errorf("the file received %q, wanted nothing on the console path", got)
			}
		})
	}
	// The surrogate pair is checked as units rather than only through the
	// round trip, so a decoder that quietly repaired a malformed pair could
	// not hide it.
	p := &fakeProbe{console: true}
	w, _ := newTestWriter(t, p)
	if _, err := w.Write([]byte("\U0001F600")); err != nil {
		t.Fatalf("write: %v", err)
	}
	units := p.chunks[0]
	if len(units) != 2 {
		t.Fatalf("U+1F600 encoded to %d units, wanted the two of a surrogate pair", len(units))
	}
	if units[0] != 0xD83D || units[1] != 0xDE00 {
		t.Errorf("U+1F600 encoded to %04X %04X, wanted D83D DE00", units[0], units[1])
	}
}

// TestWriteLoopsUntilEveryUnitIsAccepted asserts that a console write which
// accepts only part of what it is offered is retried for the remainder until
// nothing is left, and that a failure partway through is returned.
func TestWriteLoopsUntilEveryUnitIsAccepted(t *testing.T) {
	// The payload is built to need several calls at the fake's cap and to
	// carry surrogate pairs, so a loop that stopped early or that lost a
	// unit at a boundary would fail the round trip.
	block := "Ein Bindestrich — ein Emoji \U0001F600 und Hindi हिन्दी. "
	text := strings.Repeat(block, 800)
	perCall := 5000
	p := &fakeProbe{console: true, perCall: perCall}
	w, path := newTestWriter(t, p)
	n, err := w.Write([]byte(text))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != len(text) {
		t.Errorf("write reported %d bytes, wanted %d", n, len(text))
	}
	if p.calls < 4 {
		t.Errorf("the console write was called %d times, wanted at least 4 for a payload this size", p.calls)
	}
	if got := p.received(); got != text {
		t.Errorf("the console received %d characters, wanted the %d written", len([]rune(got)), len([]rune(text)))
	}
	if got := fileBytes(t, path); len(got) != 0 {
		t.Errorf("the file received %q, wanted nothing on the console path", got)
	}

	// The failure case reports the console's own error. No byte count is
	// asserted, because a partial UTF-16 write has no well-defined offset
	// back into the input.
	wanted := errors.New("the console refused")
	failing := &fakeProbe{console: true, perCall: perCall, failOn: 3, failErr: wanted}
	w2, _ := newTestWriter(t, failing)
	if _, err := w2.Write([]byte(text)); !errors.Is(err, wanted) {
		t.Errorf("a console failure returned %v, wanted %v", err, wanted)
	}
	if failing.calls != 3 {
		t.Errorf("the console write was called %d times, wanted 3 before the failure stopped the loop", failing.calls)
	}
}

// TestWriteRejoinsACharacterSplitAcrossTwoCalls asserts that a multi-byte
// character whose encoding is divided between two Write calls reaches the
// console whole, at every interior byte boundary of a 2-byte, a 3-byte and a
// 4-byte encoding.
func TestWriteRejoinsACharacterSplitAcrossTwoCalls(t *testing.T) {
	for _, char := range []string{"é", "—", "\U0001F600"} {
		encoded := []byte(char)
		for split := 1; split < len(encoded); split++ {
			p := &fakeProbe{console: true}
			w, path := newTestWriter(t, p)
			if _, err := w.Write(encoded[:split]); err != nil {
				t.Fatalf("%q split after %d, first write: %v", char, split, err)
			}
			if _, err := w.Write(encoded[split:]); err != nil {
				t.Fatalf("%q split after %d, second write: %v", char, split, err)
			}
			if got := p.received(); got != char {
				t.Errorf("%q split after byte %d arrived as %q", char, split, got)
			}
			if got := fileBytes(t, path); len(got) != 0 {
				t.Errorf("%q split after byte %d wrote %q to the file, wanted nothing", char, split, got)
			}
		}
	}
}

// TestFlushEmitsATruncatedTailAndIsOtherwiseSilent asserts that a stream
// ending on an incomplete sequence still delivers its complete leading
// content, with the truncated tail standing as the replacement character,
// and that a Flush with nothing pending writes nothing at all.
func TestFlushEmitsATruncatedTailAndIsOtherwiseSilent(t *testing.T) {
	// The em dash encodes as three bytes; only its first is written, so
	// nothing that follows can complete it.
	leading := "a complete line and then "
	truncated := []byte(leading + "—")[:len(leading)+1]
	p := &fakeProbe{console: true}
	w, _ := newTestWriter(t, p)
	if _, err := w.Write(truncated); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := p.received(); got != leading {
		t.Errorf("before the flush the console had %q, wanted the complete leading content %q", got, leading)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	got := p.received()
	if !strings.HasPrefix(got, leading) {
		t.Fatalf("the flush left %q, wanted it to open with %q", got, leading)
	}
	tail := strings.TrimPrefix(got, leading)
	if tail == "" {
		t.Errorf("the flush emitted nothing for the truncated tail, wanted the replacement character")
	}
	if strings.Trim(tail, "�") != "" {
		t.Errorf("the truncated tail flushed as %q, wanted replacement characters alone", tail)
	}

	// A flush with nothing pending is silent, both on a writer nothing has
	// been written to and on one whose last write ended on a complete
	// sequence.
	untouched := &fakeProbe{console: true}
	fresh, _ := newTestWriter(t, untouched)
	if err := fresh.Flush(); err != nil {
		t.Fatalf("flush a fresh writer: %v", err)
	}
	if untouched.calls != 0 {
		t.Errorf("flushing a fresh writer called the console %d times, wanted none", untouched.calls)
	}

	settled := &fakeProbe{console: true}
	done, _ := newTestWriter(t, settled)
	if _, err := done.Write([]byte("a line that ends whole — like this\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	before := settled.calls
	if err := done.Flush(); err != nil {
		t.Fatalf("flush after a complete write: %v", err)
	}
	if settled.calls != before {
		t.Errorf("flushing after a complete write called the console %d more times, wanted none", settled.calls-before)
	}
}

// TestBytesThatCanNeverBeCompletedAreNotHeldBack asserts the distinction
// splitIncompleteTail rests on: a truncated sequence waits for more bytes,
// but bytes that are simply invalid go out at once rather than being held
// for a completion that can never arrive.
func TestBytesThatCanNeverBeCompletedAreNotHeldBack(t *testing.T) {
	p := &fakeProbe{console: true}
	w, _ := newTestWriter(t, p)
	// A continuation byte with no lead byte before it is malformed rather
	// than truncated, so it is written now.
	if _, err := w.Write([]byte{0x41, 0x80}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if p.calls == 0 {
		t.Fatalf("malformed bytes were held back, wanted them written at once")
	}
	if len(w.pending) != 0 {
		t.Errorf("%d bytes are pending after malformed input, wanted none", len(w.pending))
	}
	if got := p.received(); !strings.HasPrefix(got, "A") {
		t.Errorf("the console received %q, wanted it to open with the valid byte", got)
	}
}

// TestNoChunkBoundarySplitsASurrogatePair asserts that the boundary
// writeConsole chooses between one console call and the next never falls
// between the two units of a surrogate pair, so a character outside the
// Basic Multilingual Plane never reaches a console as two halves in two calls.
//
// The fake accepts everything it is offered here, unlike the loop test
// above, because the boundary under test is the one this package picks at
// consoleWriteChunk. A cap the fake imposes stands in for a console
// consuming less than it was handed, and where that count falls is the
// console's choice rather than ours.
//
// Two assertions carry the criterion, and they fail on different things.
// The first reads each pair of adjacent chunks as the console receives
// them, one call at a time: utf16.DecodeRune "returns the UTF-16 decoding
// of a surrogate pair" and "if the pair is not a valid UTF-16 surrogate
// pair, DecodeRune returns the Unicode replacement code point U+FFFD", so
// a boundary whose two neighbouring units decode to anything else is a
// boundary sitting inside one character. The second decodes every chunk
// separately and joins the results, which is what a console displays
// across several calls, and compares that against the text written.
func TestNoChunkBoundarySplitsASurrogatePair(t *testing.T) {
	block := "Ein Bindestrich — ein Emoji \U0001F600 und Hindi हिन्दी. "
	cases := []struct {
		name string
		text string
	}{
		// A pair placed so that its high half is the last unit of the
		// first chunk, which splits at every value consoleWriteChunk
		// could take.
		{"a pair straddling the first boundary", strings.Repeat("a", consoleWriteChunk-1) + "\U0001F600" + strings.Repeat("b", 32)},
		// A payload long enough to cross the boundary a dozen times,
		// where whether any given crossing lands inside a pair is
		// arithmetic rather than a guarantee.
		{"a long mixed payload", strings.Repeat(block, 2000)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &fakeProbe{console: true}
			w, _ := newTestWriter(t, p)
			if _, err := w.Write([]byte(c.text)); err != nil {
				t.Fatalf("write: %v", err)
			}
			if len(p.chunks) < 2 {
				t.Fatalf("the payload produced %d console calls, wanted at least 2 for a boundary to exist", len(p.chunks))
			}
			for i := 0; i+1 < len(p.chunks); i++ {
				last := p.chunks[i][len(p.chunks[i])-1]
				first := p.chunks[i+1][0]
				if r := utf16.DecodeRune(rune(last), rune(first)); r != utf8.RuneError {
					t.Errorf("call %d ends on %04X and call %d opens on %04X, which utf16.DecodeRune reads as the single character %q: the boundary sits inside one character", i, last, i+1, first, r)
				}
			}
			var perCall strings.Builder
			for _, chunk := range p.chunks {
				perCall.WriteString(string(utf16.Decode(chunk)))
			}
			if got := perCall.String(); got != c.text {
				t.Errorf("decoding each console call on its own and joining them gave %d characters, wanted the %d written", len([]rune(got)), len([]rune(c.text)))
			}
		})
	}
}
