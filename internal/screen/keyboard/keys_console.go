package keyboard

import (
	"unicode/utf16"
)

// InputRecord is the part of one Windows console input record the console
// decoder reads. The fields are those Microsoft's INPUT_RECORD,
// KEY_EVENT_RECORD and WINDOW_BUFFER_SIZE_RECORD pages define, and only a key
// record fills the key fields.
type InputRecord struct {
	// EventType is the record's type, KeyEvent or WindowBufferSizeEvent
	// among the values INPUT_RECORD defines.
	EventType uint16
	// KeyDown is bKeyDown: true for a key pressed, false for one released.
	KeyDown bool
	// RepeatCount is wRepeatCount, how many times the key was delivered.
	RepeatCount uint16
	// VirtualKeyCode is wVirtualKeyCode.
	VirtualKeyCode uint16
	// UnicodeChar is uChar.UnicodeChar, one UTF-16 code unit.
	UnicodeChar uint16
	// ControlKeyState is dwControlKeyState.
	ControlKeyState uint32
}

// The record types INPUT_RECORD defines that the decoder reads.
const (
	KeyEvent              = 0x0001
	WindowBufferSizeEvent = 0x0004
)

// The dwControlKeyState flags KEY_EVENT_RECORD defines for the Alt and Ctrl
// keys.
const (
	rightAltPressed  = 0x0001
	leftAltPressed   = 0x0002
	rightCtrlPressed = 0x0004
	leftCtrlPressed  = 0x0008
)

// The virtual-key codes, from Microsoft's "Virtual-Key Codes" page, of the
// keys the decoder delivers by name, and of the letters Ctrl combines with.
const (
	vkBack   = 0x08
	vkTab    = 0x09
	vkReturn = 0x0d
	vkPrior  = 0x21
	vkNext   = 0x22
	vkEnd    = 0x23
	vkHome   = 0x24
	vkLeft   = 0x25
	vkUp     = 0x26
	vkRight  = 0x27
	vkDown   = 0x28
	vkDelete = 0x2e
	vkA      = 0x41
	vkZ      = 0x5a
)

// consoleNamedKeys are the virtual keys that deliver a key by name.
var consoleNamedKeys = map[uint16]Code{
	vkUp:     CodeUp,
	vkDown:   CodeDown,
	vkLeft:   CodeLeft,
	vkRight:  CodeRight,
	vkPrior:  CodePgUp,
	vkNext:   CodePgDown,
	vkHome:   CodeHome,
	vkEnd:    CodeEnd,
	vkReturn: CodeEnter,
	vkBack:   CodeBackspace,
	vkDelete: CodeDelete,
	vkTab:    CodeTab,
}

// ConsoleDecoder turns Windows console input records into keys and pastes.
// It keeps an unfinished paste marker, a paste in progress and a held high
// surrogate between calls to Feed.
type ConsoleDecoder struct {
	// match holds the characters of a paste marker matched so far.
	match []uint16
	// inPaste is true between a paste's start marker and its end marker, and
	// paste holds the content seen so far as UTF-16 code units.
	inPaste bool
	paste   []uint16
	// high is a high surrogate waiting for its low half, zero when none is.
	high uint16
	// sawPaste is true once a start marker has been decoded.
	sawPaste bool
	// gen is stamped on every event decoded.
	gen uint64
}

// NewConsoleDecoder builds a decoder with nothing held.
func NewConsoleDecoder() *ConsoleDecoder {
	return &ConsoleDecoder{}
}

// SawPaste reports whether a paste start marker has been decoded since the
// decoder was built.
func (d *ConsoleDecoder) SawPaste() bool {
	return d.sawPaste
}

// Reset discards a marker in progress, a paste in progress and a held
// surrogate, and stamps later events with a new generation.
func (d *ConsoleDecoder) Reset(gen uint64) {
	d.match = nil
	d.inPaste = false
	d.paste = nil
	d.high = 0
	d.gen = gen
}

// Feed decodes records, answering every event they complete and whether a
// buffer size record was among them.
func (d *ConsoleDecoder) Feed(records []InputRecord) (events []Event, resized bool) {
	for _, record := range records {
		if record.EventType == WindowBufferSizeEvent {
			resized = true
			continue
		}
		if record.EventType != KeyEvent || !record.KeyDown {
			continue
		}
		for _, event := range d.record(record) {
			event.Gen = d.gen
			events = append(events, event)
		}
	}
	return events, resized
}

// record decodes one key-down record.
func (d *ConsoleDecoder) record(record InputRecord) []Event {
	if d.inPaste {
		return d.pasteRecord(record)
	}
	return d.keyRecord(record)
}

// markerPrefix reports whether chars is a prefix of either paste marker, and
// which marker it completes, if any.
func markerPrefix(chars []uint16) (prefix bool, complete string) {
	text := string(utf16.Decode(chars))
	for _, marker := range []string{pasteStart, pasteEnd} {
		if len(text) <= len(marker) && marker[:len(text)] == text {
			prefix = true
			if text == marker {
				complete = marker
			}
		}
	}
	return prefix, complete
}

// keyRecord decodes one key-down record outside a paste. A record that
// breaks a marker in progress drops the characters matched so far and is
// decoded afresh, so Esc then Ctrl+C delivers ctrl+c and a lone Esc is
// dropped.
func (d *ConsoleDecoder) keyRecord(record InputRecord) []Event {
	alt := record.ControlKeyState&(leftAltPressed|rightAltPressed) != 0
	ctrl := record.ControlKeyState&(leftCtrlPressed|rightCtrlPressed) != 0
	if alt && !ctrl {
		d.match = nil
		d.high = 0
		return nil
	}
	if len(d.match) > 0 {
		candidate := append(append([]uint16(nil), d.match...), record.UnicodeChar)
		prefix, complete := markerPrefix(candidate)
		if prefix {
			d.match = candidate
			if complete == pasteStart {
				d.match = nil
				d.inPaste = true
				d.sawPaste = true
				d.paste = nil
			}
			if complete == pasteEnd {
				d.match = nil
			}
			return nil
		}
		d.match = nil
	}
	if record.UnicodeChar == 0x1b {
		d.high = 0
		d.match = []uint16{0x1b}
		return nil
	}
	event, ok := d.keyOf(record, ctrl, alt)
	if !ok {
		return nil
	}
	repeat := int(record.RepeatCount)
	if repeat < 1 {
		repeat = 1
	}
	events := make([]Event, 0, repeat)
	for range repeat {
		events = append(events, event)
	}
	return events
}

// keyOf decodes the key one record delivers: a named virtual key, Ctrl with
// a letter, or the text it types, joining a surrogate pair across two
// records and dropping an unpaired half.
func (d *ConsoleDecoder) keyOf(record InputRecord, ctrl, alt bool) (Event, bool) {
	if code, named := consoleNamedKeys[record.VirtualKeyCode]; named {
		d.high = 0
		return namedKey(code), true
	}
	if ctrl && !alt && record.VirtualKeyCode >= vkA && record.VirtualKeyCode <= vkZ {
		d.high = 0
		return ctrlKey(rune('a' + record.VirtualKeyCode - vkA)), true
	}
	char := record.UnicodeChar
	if char == 0 {
		return Event{}, false
	}
	if utf16.IsSurrogate(rune(char)) {
		if char < 0xdc00 {
			d.high = char
			return Event{}, false
		}
		if d.high == 0 {
			return Event{}, false
		}
		r := utf16.DecodeRune(rune(d.high), rune(char))
		d.high = 0
		return textKey(r), true
	}
	d.high = 0
	if char < 0x20 || char == 0x7f {
		return Event{}, false
	}
	return textKey(rune(char)), true
}

// pasteRecord reads one key-down record inside an open paste. Every non-zero
// character is content, a record carrying a line break included, until the
// end marker, which delivers the paste with the marker excluded. A record
// that breaks the end marker in progress gives the characters matched so far
// to the content and is read afresh as content. Ctrl+C abandons the paste and
// is delivered as ctrl+c.
func (d *ConsoleDecoder) pasteRecord(record InputRecord) []Event {
	char := record.UnicodeChar
	if char == 0 {
		return nil
	}
	if char == ctrlC {
		d.inPaste = false
		d.paste = nil
		d.match = nil
		return []Event{ctrlKey('c')}
	}
	repeat := int(record.RepeatCount)
	if repeat < 1 {
		repeat = 1
	}
	for range repeat {
		if event, done := d.pasteChar(char); done {
			return []Event{event}
		}
	}
	return nil
}

// pasteChar adds one character to an open paste, answering the paste when
// the character completes its end marker.
func (d *ConsoleDecoder) pasteChar(char uint16) (Event, bool) {
	if len(d.match) > 0 {
		candidate := append(append([]uint16(nil), d.match...), char)
		if isEndPrefix(candidate) {
			d.match = candidate
			if len(candidate) == len(pasteEnd) {
				text := pasteContent(string(utf16.Decode(d.paste)))
				d.match = nil
				d.inPaste = false
				d.paste = nil
				return Event{Paste: true, Text: text}, true
			}
			return Event{}, false
		}
		d.paste = append(d.paste, d.match...)
		d.match = nil
	}
	if char == 0x1b {
		d.match = []uint16{0x1b}
		return Event{}, false
	}
	d.paste = append(d.paste, char)
	return Event{}, false
}

// isEndPrefix reports whether chars is a prefix of the end marker, which is
// the only marker a paste already open can meet.
func isEndPrefix(chars []uint16) bool {
	text := string(utf16.Decode(chars))
	return len(text) <= len(pasteEnd) && pasteEnd[:len(text)] == text
}
