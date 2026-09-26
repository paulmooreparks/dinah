package keyboard

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Event is one thing a reader decoded: a key, or a whole paste.
type Event struct {
	Key   Key    // the key, when Paste is false
	Paste bool   // the event is a paste
	Text  string // a paste's content, when Paste is true
	Gen   uint64 // the reader's flush generation when it was decoded
}

// Key is one key a reader decoded.
type Key struct {
	Code Code   // the key's code; CodeRune for a printable key or a ctrl+ key
	Rune rune   // with CodeRune, the key's rune: the character typed, or the letter of a ctrl+ key
	Ctrl bool   // Ctrl was held
	Text string // the text a printable key types, empty otherwise
}

// Code names a key a reader decoded. The package declares its own codes, so
// neither it nor internal/screen depends on the terminal UI's library; the
// head maps them to that library's codes in one table.
type Code int

// The keys a reader can deliver besides printable text and ctrl+ a letter.
const (
	CodeRune Code = iota // a printable key or a ctrl+ key; Key.Rune holds it
	CodeEnter
	CodeBackspace
	CodeDelete
	CodeTab
	CodeUp
	CodeDown
	CodeLeft
	CodeRight
	CodePgUp
	CodePgDown
	CodeHome
	CodeEnd
)

// The bracketed-paste sequences xterm's "XTerm Control Sequences" documents
// under "Bracketed Paste Mode": the two a program writes to switch the mode
// on and off, and the two markers a terminal honouring it wraps a paste in.
const (
	BracketedPasteOn  = "\x1b[?2004h"
	BracketedPasteOff = "\x1b[?2004l"
	pasteStart        = "\x1b[200~"
	pasteEnd          = "\x1b[201~"
)

// ctrlC is the character Ctrl+C produces, which both readers treat as the one
// key that ends a paste whose end marker never arrived.
const ctrlC = 0x03

// namedKey is a key with no text, such as an arrow.
func namedKey(code Code) Event {
	return Event{Key: Key{Code: code}}
}

// ctrlKey is Ctrl held with a letter, a to z.
func ctrlKey(letter rune) Event {
	return Event{Key: Key{Code: CodeRune, Rune: letter, Ctrl: true}}
}

// textKey is a printable character, delivered as the text it types.
func textKey(r rune) Event {
	return Event{Key: Key{Code: CodeRune, Rune: r, Text: string(r)}}
}

// controlKey decodes one C0 control character that is a key of its own:
// Enter, Tab, Backspace, or Ctrl with a letter. It reports false for every
// other control character, which the readers drop.
func controlKey(r rune) (Event, bool) {
	switch {
	case r == '\r' || r == '\n':
		return namedKey(CodeEnter), true
	case r == '\t':
		return namedKey(CodeTab), true
	case r == 0x7f || r == 0x08:
		return namedKey(CodeBackspace), true
	case r >= 0x01 && r <= 0x1a:
		return ctrlKey('a' + r - 1), true
	}
	return Event{}, false
}

// pasteContent is a paste's text as the model receives it: every control
// character other than a tab, a carriage return and a line feed dropped, so
// nothing pasted can reach the terminal as a control sequence.
func pasteContent(text string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\r' || r == '\n' {
			return r
		}
		if unicode.Is(unicode.Cc, r) {
			return -1
		}
		return r
	}, text)
}

// validText is raw bytes decoded as UTF-8, each ill-formed sequence becoming
// U+FFFD.
func validText(raw []byte) string {
	return strings.ToValidUTF8(string(raw), string(utf8.RuneError))
}
