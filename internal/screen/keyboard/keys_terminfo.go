package keyboard

import (
	"bytes"
	"dinah/internal/screen"
	"unicode/utf8"
)

// keyCapabilities are the terminfo key strings the decoder delivers, each
// with the key it delivers. terminfo(5) defines them as what the keypad sends
// in transmit mode, which is why the head writes smkx on entry.
var keyCapabilities = []struct {
	index int
	code  Code
}{
	{screen.CapKcuu1, CodeUp},
	{screen.CapKcud1, CodeDown},
	{screen.CapKcub1, CodeLeft},
	{screen.CapKcuf1, CodeRight},
	{screen.CapKpp, CodePgUp},
	{screen.CapKnp, CodePgDown},
	{screen.CapKhome, CodeHome},
	{screen.CapKend, CodeEnd},
	{screen.CapKdch1, CodeDelete},
	{screen.CapKbs, CodeBackspace},
}

// requiredArrows are the capabilities without which the head refuses to
// start, in the order the refusal names the first one missing.
var requiredArrows = []struct {
	index int
	name  string
}{
	{screen.CapKcuu1, "kcuu1"},
	{screen.CapKcud1, "kcud1"},
	{screen.CapKcub1, "kcub1"},
	{screen.CapKcuf1, "kcuf1"},
}

// MissingArrow names the first arrow-key capability an entry lacks, of kcuu1,
// kcud1, kcub1 and kcuf1, and answers the empty string where it has all four.
func MissingArrow(entry *screen.Terminfo) string {
	for _, arrow := range requiredArrows {
		if value, ok := entry.String(arrow.index); !ok || value == "" {
			return arrow.name
		}
	}
	return ""
}

// KeypadTransmit answers the strings that switch the keypad into transmit
// mode and back, smkx and rmkx, each empty where the entry has none.
func KeypadTransmit(entry *screen.Terminfo) (on, off string) {
	on, _ = entry.String(screen.CapSmkx)
	off, _ = entry.String(screen.CapRmkx)
	return on, off
}

// KeyDecoder turns the bytes a POSIX terminal sends into keys and pastes,
// using the key strings of one terminfo entry. It keeps incomplete input, and
// any paste in progress, between calls to Feed, and decides everything from
// the bytes alone, so the timing of Esc never matters.
type KeyDecoder struct {
	// sequences maps each key string the entry declares to the key it is.
	sequences map[string]Code
	// pending is input whose end has not arrived yet.
	pending []byte
	// inPaste is true between a paste's start marker and its end marker, and
	// paste holds the content seen so far.
	inPaste bool
	paste   []byte
	// sawPaste is true once a start marker has been decoded.
	sawPaste bool
	// gen is stamped on every event decoded.
	gen uint64
}

// NewKeyDecoder builds a decoder for the key strings entry declares. A
// capability the entry lacks delivers no key.
func NewKeyDecoder(entry *screen.Terminfo) *KeyDecoder {
	d := &KeyDecoder{sequences: map[string]Code{}}
	if entry == nil {
		return d
	}
	for _, capability := range keyCapabilities {
		value, ok := entry.String(capability.index)
		if !ok || value == "" {
			continue
		}
		if _, taken := d.sequences[value]; !taken {
			d.sequences[value] = capability.code
		}
	}
	return d
}

// SawPaste reports whether a paste start marker has been decoded since the
// decoder was built.
func (d *KeyDecoder) SawPaste() bool {
	return d.sawPaste
}

// Reset discards every incomplete sequence and any paste in progress, and
// stamps later events with a new generation, which is what a flush does to
// the input the terminal had already delivered.
func (d *KeyDecoder) Reset(gen uint64) {
	d.pending = nil
	d.inPaste = false
	d.paste = nil
	d.gen = gen
}

// Feed decodes bytes, answering every event they complete. Bytes that begin
// something not yet complete are kept for the next call.
func (d *KeyDecoder) Feed(input []byte) []Event {
	buf := append(d.pending, input...)
	d.pending = nil
	var events []Event
	i := 0
	for i < len(buf) {
		if d.inPaste {
			used, event := d.pasteByte(buf[i:])
			if used == 0 {
				d.pending = append([]byte(nil), buf[i:]...)
				break
			}
			i += used
			if event != nil {
				events = append(events, d.stamped(*event))
			}
			continue
		}
		used, decoded := d.keyAt(buf[i:])
		if used == 0 {
			d.pending = append([]byte(nil), buf[i:]...)
			break
		}
		i += used
		for _, event := range decoded {
			events = append(events, d.stamped(event))
		}
	}
	return events
}

// stamped is an event carrying the decoder's generation.
func (d *KeyDecoder) stamped(event Event) Event {
	event.Gen = d.gen
	return event
}

// pasteByte reads inside an open paste. It answers how many bytes it used,
// zero where the input is a proper prefix of the end marker and more is
// needed, and the event completed, if any. The end marker delivers the paste
// with both markers excluded. Ctrl+C abandons the paste and is delivered as
// ctrl+c, so a paste whose end marker never arrives cannot keep the person
// from quitting. Any other byte, a near miss of the end marker included, is
// content.
func (d *KeyDecoder) pasteByte(buf []byte) (int, *Event) {
	if buf[0] == ctrlC {
		d.inPaste = false
		d.paste = nil
		event := ctrlKey('c')
		return 1, &event
	}
	if buf[0] == 0x1b {
		if bytes.HasPrefix(buf, []byte(pasteEnd)) {
			text := pasteContent(validText(d.paste))
			d.inPaste = false
			d.paste = nil
			return len(pasteEnd), &Event{Paste: true, Text: text}
		}
		if bytes.HasPrefix([]byte(pasteEnd), buf) {
			return 0, nil
		}
	}
	d.paste = append(d.paste, buf[0])
	return 1, nil
}

// keyAt decodes the input at the start of buf outside a paste. It answers
// how many bytes it used, zero where more input is needed, and the events
// those bytes delivered, which may be none.
func (d *KeyDecoder) keyAt(buf []byte) (int, []Event) {
	b := buf[0]
	if b == 0x1b {
		return d.escapeAt(buf)
	}
	if b < 0x20 || b == 0x7f {
		if event, ok := controlKey(rune(b)); ok {
			return 1, []Event{event}
		}
		return 1, nil
	}
	if !utf8.FullRune(buf) {
		return 0, nil
	}
	r, size := utf8.DecodeRune(buf)
	return size, []Event{textKey(r)}
}

// escapeAt decodes a sequence opened by ESC, taking its extent from ECMA-48
// section 5.4. A sequence equal to one of the entry's key strings delivers
// that key, the paste start marker opens a paste, and every other complete
// sequence, an Alt combination included, is discarded whole. An ESC followed
// by a byte that can begin no sequence is dropped alone, and that byte is
// decoded afresh, so Esc then Ctrl+C still delivers ctrl+c.
func (d *KeyDecoder) escapeAt(buf []byte) (int, []Event) {
	if len(buf) < 2 {
		return 0, nil
	}
	next := buf[1]
	var extent int
	switch {
	case next == '[':
		extent = controlSequenceExtent(buf)
	case next == 'O':
		extent = shiftThreeExtent(buf)
	case next >= 0x20 && next <= 0x7e:
		extent = 2
	default:
		return 1, nil
	}
	if extent == 0 {
		return 0, nil
	}
	if extent < 0 {
		// A byte outside the grammar cut the sequence short. What was read
		// before it is discarded, and that byte is decoded afresh.
		return -extent, nil
	}
	sequence := string(buf[:extent])
	if sequence == pasteStart {
		d.inPaste = true
		d.sawPaste = true
		d.paste = nil
		return extent, nil
	}
	if code, ok := d.sequences[sequence]; ok {
		return extent, []Event{namedKey(code)}
	}
	return extent, nil
}

// controlSequenceExtent measures a control sequence starting ESC [: any
// parameter bytes 0x30 to 0x3F, then any intermediate bytes 0x20 to 0x2F,
// then one final byte 0x40 to 0x7E. It answers the length including the
// final byte, zero where the input ends first, and minus the number of bytes
// read before a byte the grammar does not allow there.
func controlSequenceExtent(buf []byte) int {
	i := 2
	for i < len(buf) && buf[i] >= 0x30 && buf[i] <= 0x3f {
		i++
	}
	for i < len(buf) && buf[i] >= 0x20 && buf[i] <= 0x2f {
		i++
	}
	if i >= len(buf) {
		return 0
	}
	if buf[i] >= 0x40 && buf[i] <= 0x7e {
		return i + 1
	}
	return -i
}

// shiftThreeExtent measures a sequence starting ESC O, which is one more
// byte. A byte after it that is not printable ends the sequence short, as a
// byte outside the grammar does in controlSequenceExtent.
func shiftThreeExtent(buf []byte) int {
	if len(buf) < 3 {
		return 0
	}
	if buf[2] >= 0x20 && buf[2] <= 0x7e {
		return 3
	}
	return -2
}
