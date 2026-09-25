package screen

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// xtermKeys are the key strings the compiled xterm fixture declares, which
// the terminal head's tests in cmd/dinah send as bytes.
var xtermKeys = map[int]string{
	strKcuu1: "\x1bOA",
	strKcud1: "\x1bOB",
	strKcuf1: "\x1bOC",
	strKcub1: "\x1bOD",
	strKhome: "\x1bOH",
	strKend:  "\x1bOF",
	strKpp:   "\x1b[5~",
	strKnp:   "\x1b[6~",
	strKdch1: "\x1b[3~",
}

// TestTheXtermFixtureDeclaresTheKeysTheTestsSend holds the key strings the
// head's tests send to the compiled xterm entry they are decoded with, so a
// test sending ESC O A is sending what the entry calls the up arrow.
func TestTheXtermFixtureDeclaresTheKeysTheTestsSend(t *testing.T) {
	entry := readEntry(t, "78", "xterm")
	for index, want := range xtermKeys {
		got, ok := entry.String(index)
		if !ok || got != want {
			t.Errorf("capability %d is %q (present %v), and the tests send %q", index, got, ok, want)
		}
	}
	if on, off := KeypadTransmit(entry); on == "" || off == "" {
		t.Errorf("the entry declares smkx %q and rmkx %q, and both are expected", on, off)
	}
	if missing := MissingArrow(entry); missing != "" {
		t.Errorf("the entry lacks %s", missing)
	}
	if missing := MissingArrow(readEntry(t, "64", "dinah-no-cup")); missing != "kcuu1" {
		t.Errorf("an entry with no key strings names %q as the first missing arrow, wanted kcuu1", missing)
	}
}

// key, ctrl and text are the events the decoders deliver, spelled short.
func key(code rune) Event    { return Event{Key: Key{Code: code}} }
func ctrl(letter rune) Event { return Event{Key: Key{Code: letter, Ctrl: true}} }
func text(r rune) Event      { return Event{Key: Key{Code: r, Text: string(r)}} }
func paste(s string) Event   { return Event{Paste: true, Text: s} }

// decodeCase is one byte string and the events it must deliver.
type decodeCase struct {
	name  string
	input string
	want  []Event
}

// decodeCases are the cases section 11.3 of the specification names for the
// POSIX decoder, decoded with the xterm entry, with one change the design
// review's fourth round asked for: Ctrl+C inside a paste abandons the paste
// and is delivered, so a paste whose end marker never arrives cannot keep the
// person from quitting.
var decodeCases = []decodeCase{
	{"up", "\x1bOA", []Event{key(tea.KeyUp)}},
	{"down", "\x1bOB", []Event{key(tea.KeyDown)}},
	{"left", "\x1bOD", []Event{key(tea.KeyLeft)}},
	{"right", "\x1bOC", []Event{key(tea.KeyRight)}},
	{"page up", "\x1b[5~", []Event{key(tea.KeyPgUp)}},
	{"page down", "\x1b[6~", []Event{key(tea.KeyPgDown)}},
	{"home", "\x1bOH", []Event{key(tea.KeyHome)}},
	{"end", "\x1bOF", []Event{key(tea.KeyEnd)}},
	{"delete", "\x1b[3~", []Event{key(tea.KeyDelete)}},
	{"a lone ESC then q", "\x1bq", nil},
	{"ESC b", "\x1bb", nil},
	{"ESC then Ctrl+C", "\x1b\x03", []Event{ctrl('c')}},
	{"ESC then Ctrl+G", "\x1b\x07", []Event{ctrl('g')}},
	{"ESC ESC then an up arrow", "\x1b\x1bOA", []Event{key(tea.KeyUp)}},
	{"an unknown sequence with parameters", "\x1b[1;5A", nil},
	{"ESC [ with a final byte a, then the rest as text", "\x1b[a bq", []Event{text(' '), text('b'), text('q')}},
	{"ESC [ with an intermediate and a final byte, then q", "\x1b[ bq", []Event{text('q')}},
	{"ESC [ cut short by Ctrl+C", "\x1b[1\x03", []Event{ctrl('c')}},
	{"enter as CR", "\r", []Event{key(tea.KeyEnter)}},
	{"enter as LF", "\n", []Event{key(tea.KeyEnter)}},
	{"tab", "\t", []Event{key(tea.KeyTab)}},
	{"backspace as DEL", "\x7f", []Event{key(tea.KeyBackspace)}},
	{"backspace as BS", "\x08", []Event{key(tea.KeyBackspace)}},
	{"Ctrl+A", "\x01", []Event{ctrl('a')}},
	{"Ctrl+C", "\x03", []Event{ctrl('c')}},
	{"Ctrl+D", "\x04", []Event{ctrl('d')}},
	{"Ctrl+G", "\x07", []Event{ctrl('g')}},
	{"Ctrl+Z", "\x1a", []Event{ctrl('z')}},
	{"a NUL and the file separator are dropped", "\x00\x1c", nil},
	{"a printable key", "k", []Event{text('k')}},
	{"a two-byte character", "é", []Event{text('é')}},
	{"an ill-formed byte", "\xff", []Event{text('�')}},
	{"a paste holding a line break", "\x1b[200~a\r\nb\x1b[201~", []Event{paste("a\r\nb")}},
	{"a paste holding Ctrl+C abandons it", "\x1b[200~a\x03b\x1b[201~", []Event{ctrl('c'), text('b')}},
	{"a paste holding a near miss of its end marker", "\x1b[200~\x1b[201x\x1b[201~", []Event{paste("[201x")}},
	{"a paste holding an escape sequence keeps its text", "\x1b[200~\x1b[31mred\x1b[201~", []Event{paste("[31mred")}},
	{"an end marker outside a paste", "\x1b[201~", nil},
}

// TestTheKeyDecoderDeliversExactlyTheKeysSectionSixThreeLists feeds each case
// whole, and again split into two calls at every byte boundary, and holds the
// events to the case's list exactly. It counts the cases and the splits it
// ran, and none of them may deliver Esc or a key with Alt held, which the
// decoder cannot represent at all.
func TestTheKeyDecoderDeliversExactlyTheKeysSectionSixThreeLists(t *testing.T) {
	entry := readEntry(t, "78", "xterm")
	splits := 0
	for _, c := range decodeCases {
		whole := NewKeyDecoder(entry).Feed([]byte(c.input))
		if !sameEvents(whole, c.want) {
			t.Errorf("%s: %q delivered %v, wanted %v", c.name, c.input, whole, c.want)
		}
		for at := 1; at < len(c.input); at++ {
			d := NewKeyDecoder(entry)
			got := append(d.Feed([]byte(c.input[:at])), d.Feed([]byte(c.input[at:]))...)
			splits++
			if !sameEvents(got, c.want) {
				t.Errorf("%s split at %d: delivered %v, wanted %v", c.name, at, got, c.want)
			}
		}
	}
	t.Logf("%d cases, %d two-call splits", len(decodeCases), splits)
	if len(decodeCases) < 30 || splits == 0 {
		t.Fatalf("ran %d cases and %d splits, which is fewer than the table carries", len(decodeCases), splits)
	}
}

// TestAResetDiscardsWhatTheDecoderHeld is the decoder's half of the flush: an
// incomplete sequence and an open paste are both gone after Reset, and later
// events carry the new generation.
func TestAResetDiscardsWhatTheDecoderHeld(t *testing.T) {
	d := NewKeyDecoder(readEntry(t, "78", "xterm"))
	if got := d.Feed([]byte("\x1bO")); got != nil {
		t.Fatalf("an incomplete sequence delivered %v", got)
	}
	d.Reset(3)
	got := d.Feed([]byte("A"))
	if len(got) != 1 || got[0].Key.Text != "A" || got[0].Gen != 3 {
		t.Errorf("after a reset, A delivered %v, wanted the text A at generation 3", got)
	}
	d.Feed([]byte("\x1b[200~half"))
	if !d.SawPaste() {
		t.Error("the decoder did not record the start marker it read")
	}
	d.Reset(4)
	if got := d.Feed([]byte("q")); len(got) != 1 || got[0].Key.Text != "q" {
		t.Errorf("after a reset inside a paste, q delivered %v, wanted the key q", got)
	}
}

// sameEvents compares two event lists, ignoring generations.
func sameEvents(got, want []Event) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		g := got[i]
		g.Gen = 0
		if !reflect.DeepEqual(g, want[i]) {
			return false
		}
	}
	return true
}
