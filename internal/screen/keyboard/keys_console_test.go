package keyboard

import (
	"testing"
)

// down builds a key-down record for a virtual key and the character it
// types, with the control state given.
func down(vk uint16, char uint16, state uint32) InputRecord {
	return InputRecord{EventType: KeyEvent, KeyDown: true, RepeatCount: 1, VirtualKeyCode: vk, UnicodeChar: char, ControlKeyState: state}
}

// typed is the records a terminal delivers for text: one key-down record per
// UTF-16 code unit, a carriage return carried on VK_RETURN, and the escape
// character on VK_ESCAPE.
func typed(s string) []InputRecord {
	var records []InputRecord
	for _, r := range s {
		vk := uint16(0)
		switch r {
		case '\r':
			vk = vkReturn
		case 0x1b:
			vk = 0x1b
		}
		if r > 0xffff {
			r -= 0x10000
			records = append(records, down(0, uint16(0xd800+(r>>10)), 0), down(0, uint16(0xdc00+(r&0x3ff)), 0))
			continue
		}
		records = append(records, down(vk, uint16(r), 0))
	}
	return records
}

// consoleCase is one list of records and the events it must deliver.
type consoleCase struct {
	name    string
	records []InputRecord
	want    []Event
	resized bool
}

// consoleCases are the cases section 11.3 names for the console decoder.
var consoleCases = []consoleCase{
	{name: "up", records: []InputRecord{down(vkUp, 0, 0)}, want: []Event{key(CodeUp)}},
	{name: "down", records: []InputRecord{down(vkDown, 0, 0)}, want: []Event{key(CodeDown)}},
	{name: "left", records: []InputRecord{down(vkLeft, 0, 0)}, want: []Event{key(CodeLeft)}},
	{name: "right", records: []InputRecord{down(vkRight, 0, 0)}, want: []Event{key(CodeRight)}},
	{name: "page up", records: []InputRecord{down(vkPrior, 0, 0)}, want: []Event{key(CodePgUp)}},
	{name: "page down", records: []InputRecord{down(vkNext, 0, 0)}, want: []Event{key(CodePgDown)}},
	{name: "home", records: []InputRecord{down(vkHome, 0, 0)}, want: []Event{key(CodeHome)}},
	{name: "end", records: []InputRecord{down(vkEnd, 0, 0)}, want: []Event{key(CodeEnd)}},
	{name: "enter", records: []InputRecord{down(vkReturn, '\r', 0)}, want: []Event{key(CodeEnter)}},
	{name: "backspace", records: []InputRecord{down(vkBack, 0x08, 0)}, want: []Event{key(CodeBackspace)}},
	{name: "delete", records: []InputRecord{down(vkDelete, 0, 0)}, want: []Event{key(CodeDelete)}},
	{name: "tab", records: []InputRecord{down(vkTab, '\t', 0)}, want: []Event{key(CodeTab)}},
	{name: "a key released", records: []InputRecord{{EventType: KeyEvent, KeyDown: false, RepeatCount: 1, VirtualKeyCode: 'Q', UnicodeChar: 'q'}}},
	{name: "Alt held", records: []InputRecord{down('Q', 'q', leftAltPressed)}},
	{name: "right Alt held with an arrow", records: []InputRecord{down(vkUp, 0, rightAltPressed)}},
	{name: "AltGr types text", records: []InputRecord{down('Q', '@', rightAltPressed|leftCtrlPressed)}, want: []Event{text('@')}},
	{name: "Ctrl with a letter", records: []InputRecord{down('C', 0x03, leftCtrlPressed)}, want: []Event{ctrl('c')}},
	{name: "right Ctrl with G", records: []InputRecord{down('G', 0x07, rightCtrlPressed)}, want: []Event{ctrl('g')}},
	{name: "a printable key", records: []InputRecord{down('K', 'k', 0)}, want: []Event{text('k')}},
	{name: "a surrogate pair", records: typed("😀"), want: []Event{text('😀')}},
	{name: "an unpaired low surrogate", records: []InputRecord{down(0, 0xdc00, 0)}},
	{name: "an unpaired high surrogate then a key", records: []InputRecord{down(0, 0xd83d, 0), down('K', 'k', 0)}, want: []Event{text('k')}},
	{name: "a repeat count of three", records: []InputRecord{{EventType: KeyEvent, KeyDown: true, RepeatCount: 3, VirtualKeyCode: vkDown}}, want: []Event{key(CodeDown), key(CodeDown), key(CodeDown)}},
	{name: "a lone Esc", records: []InputRecord{down(0x1b, 0x1b, 0)}},
	{name: "Esc then Ctrl+C", records: []InputRecord{down(0x1b, 0x1b, 0), down('C', 0x03, leftCtrlPressed)}, want: []Event{ctrl('c')}},
	{name: "Esc then q", records: []InputRecord{down(0x1b, 0x1b, 0), down('Q', 'q', 0)}, want: []Event{text('q')}},
	{name: "a buffer size record", records: []InputRecord{{EventType: WindowBufferSizeEvent}}, resized: true},
	{name: "a mouse record", records: []InputRecord{{EventType: 0x0002}}},
	{name: "a paste with a line break", records: typed("\x1b[200~looks fine\rbut\x1b[201~"), want: []Event{paste("looks fine\rbut")}},
	{name: "a paste with a near miss of its end marker", records: typed("\x1b[200~a\x1b[201x\x1b[201~"), want: []Event{paste("a[201x")}},
	{name: "a paste holding Ctrl+C abandons it", records: append(typed("\x1b[200~ab"), down('C', 0x03, leftCtrlPressed)), want: []Event{ctrl('c')}},
	{name: "an end marker outside a paste", records: typed("\x1b[201~")},
}

// TestTheConsoleDecoderDeliversExactlyTheKeysSectionSixTwoLists feeds each
// case whole, and every case of more than one record again split into two
// calls at every record boundary, and holds the events and the resize flag
// to the case exactly. It counts what it ran.
func TestTheConsoleDecoderDeliversExactlyTheKeysSectionSixTwoLists(t *testing.T) {
	splits := 0
	for _, c := range consoleCases {
		got, resized := NewConsoleDecoder().Feed(c.records)
		if !sameEvents(got, c.want) || resized != c.resized {
			t.Errorf("%s: delivered %v resized %v, wanted %v resized %v", c.name, got, resized, c.want, c.resized)
		}
		for at := 1; at < len(c.records); at++ {
			d := NewConsoleDecoder()
			first, _ := d.Feed(c.records[:at])
			second, _ := d.Feed(c.records[at:])
			splits++
			if joined := append(first, second...); !sameEvents(joined, c.want) {
				t.Errorf("%s split at record %d: delivered %v, wanted %v", c.name, at, joined, c.want)
			}
		}
	}
	t.Logf("%d cases, %d two-call splits", len(consoleCases), splits)
	if len(consoleCases) < 30 || splits == 0 {
		t.Fatalf("ran %d cases and %d splits, which is fewer than the table carries", len(consoleCases), splits)
	}
}

// TestAConsolePasteSpelledByRecordsIsOnePasteAndNoKey is the console half of
// dinah-603/criteria/36: records spelling ESC [200~, text holding a
// VK_RETURN, and ESC [201~ deliver one paste and no key, and the decoder
// records that the terminal marks pastes.
func TestAConsolePasteSpelledByRecordsIsOnePasteAndNoKey(t *testing.T) {
	d := NewConsoleDecoder()
	got, _ := d.Feed(typed("\x1b[200~one\rtwo\x1b[201~"))
	if len(got) != 1 || !got[0].Paste || got[0].Text != "one\rtwo" {
		t.Fatalf("delivered %v, wanted one paste of one CR two", got)
	}
	if !d.SawPaste() {
		t.Error("the decoder did not record the start marker")
	}
}
