//go:build tui

package main

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/rivo/uniseg"

	"dinah/internal/consolewriter"
)

// rendererShapedFrame is a frame in the shape Bubble Tea's renderer writes:
// 60 rows of 300 columns, each group of ten cells introduced by a cursor
// position and an SGR change, with text holding wide characters, a combining
// sequence and a character outside the Basic Multilingual Plane, which is two
// UTF-16 code units, so that its rune count is over 20,000 and its UTF-16
// length is greater still.
func rendererShapedFrame() []byte {
	var b bytes.Buffer
	cells := []string{"a", "b", "界", "c", "e\u0301", "d", "e", "語", "\U0001F600", "g"}
	for row := 1; row <= 60; row++ {
		for group := 0; group < 30; group++ {
			b.WriteString("\x1b[" + strconv.Itoa(row) + ";" + strconv.Itoa(group*10+1) + "H")
			b.WriteString("\x1b[" + []string{"0", "1;31", "7", "33", "22;34"}[group%5] + "m")
			for _, cell := range cells {
				b.WriteString(cell)
			}
		}
	}
	return b.Bytes()
}

// wholeSequencesOnly reports the first escape sequence a piece holds only
// part of, read on its own with ECMA-48's grammar, or the empty string.
func wholeSequencesOnly(piece []byte) string {
	for at := 0; at < len(piece); at++ {
		if piece[at] != 0x1b {
			continue
		}
		length, complete := escapeExtent(piece[at:])
		if !complete {
			return strconv.Quote(string(piece[at:]))
		}
		at += length - 1
	}
	return ""
}

// startsInsideASequence reports whether a piece opens with the tail of an
// escape sequence cut from the piece before it.
func startsInsideASequence(previous, piece []byte) bool {
	last := bytes.LastIndexByte(previous, 0x1b)
	if last < 0 {
		return false
	}
	_, complete := escapeExtent(previous[last:])
	return !complete && len(piece) > 0
}

// TestAFrameOverSixteenThousandCharactersIsNeverCutInASequence is the first
// half of dinah-603/criteria/48. A frame of over 20,000 runes in the
// renderer's shape passes through framePieces with pieceBound, and every
// piece holds at most pieceBound UTF-16 code units, and so at most pieceBound
// runes, the pieces joined are the input byte
// for byte, every piece holds only whole escape sequences, no piece begins or
// ends inside a UTF-8 sequence or a grapheme cluster, and there are at least
// three pieces.
func TestAFrameOverSixteenThousandCharactersIsNeverCutInASequence(t *testing.T) {
	frame := rendererShapedFrame()
	if runes := utf8.RuneCount(frame); runes <= 20000 {
		t.Fatalf("the frame holds %d runes, and the test wants over 20,000", runes)
	}
	pieces := framePieces(frame, pieceBound)
	t.Logf("%d bytes and %d runes cut into %d pieces", len(frame), utf8.RuneCount(frame), len(pieces))
	if len(pieces) < 3 {
		t.Fatalf("the frame was cut into %d pieces, and it has to cross the bound more than once", len(pieces))
	}
	if joined := bytes.Join(pieces, nil); !bytes.Equal(joined, frame) {
		t.Fatal("the pieces joined differ from the frame")
	}
	clusters := clusterBoundaries(frame)
	offset := 0
	for i, piece := range pieces {
		if units := len(utf16.Encode([]rune(string(piece)))); units > pieceBound {
			t.Errorf("piece %d holds %d UTF-16 code units, over the bound of %d", i, units, pieceBound)
		}
		if runes := utf8.RuneCount(piece); runes > pieceBound {
			t.Errorf("piece %d holds %d runes, over the bound of %d", i, runes, pieceBound)
		}
		if broken := wholeSequencesOnly(piece); broken != "" {
			t.Errorf("piece %d holds only part of the escape sequence %s", i, broken)
		}
		if i > 0 && startsInsideASequence(pieces[i-1], piece) {
			t.Errorf("piece %d begins inside an escape sequence the piece before it opened", i)
		}
		if !utf8.Valid(piece) {
			t.Errorf("piece %d begins or ends inside a UTF-8 sequence", i)
		}
		offset += len(piece)
		if offset < len(frame) && !clusters[offset] {
			t.Errorf("piece %d ends inside a grapheme cluster, at byte %d", i, offset)
		}
	}
}

// clusterBoundaries marks every byte offset of a frame's text at which a
// grapheme cluster begins, and every offset at which an escape sequence
// begins or ends, which are the places a cut may fall.
func clusterBoundaries(frame []byte) map[int]bool {
	marks := map[int]bool{0: true, len(frame): true}
	for at := 0; at < len(frame); {
		if frame[at] == 0x1b {
			length, _ := escapeExtent(frame[at:])
			at += length
			marks[at] = true
			continue
		}
		text := frame[at:]
		if next := bytes.IndexByte(text, 0x1b); next >= 0 {
			text = text[:next]
		}
		state := -1
		for len(text) > 0 {
			var cluster []byte
			cluster, text, _, state = uniseg.FirstGraphemeCluster(text, state)
			at += len(cluster)
			marks[at] = true
		}
	}
	return marks
}

// TestConsoleFramesRewritesOnlyCRAndLF is the rewrite half of
// dinah-603/criteria/44: consoleFrames writes ESC [1G for every carriage
// return and ESC [B for every line feed, passes every other byte through, and
// holds an escape or UTF-8 sequence split across two writes until it is
// whole, at every split point of each case.
func TestConsoleFramesRewritesOnlyCRAndLF(t *testing.T) {
	cases := map[string]string{
		"a line feed":         "one\ntwo",
		"a carriage return":   "one\rtwo",
		"CR LF":               "one\r\ntwo",
		"escape sequences":    "\x1b[3;7H\x1b[1;31mred\x1b[m\x1b[K",
		"UTF-8 text":          "界語é and naïve",
		"all of them at once": "\x1b[2J\r\n界\x1b[7m語\x1b[m\r\n",
	}
	want := map[string]string{
		"a line feed":         "one\x1b[Btwo",
		"a carriage return":   "one\x1b[1Gtwo",
		"CR LF":               "one\x1b[1G\x1b[Btwo",
		"escape sequences":    "\x1b[3;7H\x1b[1;31mred\x1b[m\x1b[K",
		"UTF-8 text":          "界語é and naïve",
		"all of them at once": "\x1b[2J\x1b[1G\x1b[B界\x1b[7m語\x1b[m\x1b[1G\x1b[B",
	}
	splits := 0
	for name, input := range cases {
		for at := 0; at <= len(input); at++ {
			var out bytes.Buffer
			frames := newConsoleFrames(&out)
			frames.Write([]byte(input[:at]))
			frames.Write([]byte(input[at:]))
			frames.Flush()
			splits++
			if out.String() != want[name] {
				t.Errorf("%s split at %d: wrote %q, wanted %q", name, at, out.String(), want[name])
			}
			if strings.ContainsAny(out.String(), "\r\n") {
				t.Errorf("%s split at %d: a CR or LF reached the console", name, at)
			}
		}
	}
	t.Logf("%d cases, %d splits", len(cases), splits)
}

// TestConsoleFramesWritesEachPieceOnce is the last clause of
// dinah-603/criteria/44: behind consoleFrames stands the console writer of
// internal/consolewriter over a fake console that records every
// WriteConsoleW call, and for the renderer-shaped frame, with its rows broken
// by CR LF, the calls are exactly the pieces framePieces answers for the
// rewritten buffer, in order, one call to a piece.
func TestConsoleFramesWritesEachPieceOnce(t *testing.T) {
	frame := bytes.ReplaceAll(rendererShapedFrame(), []byte("\x1b[1;1H"), []byte("\r\n\x1b[1;1H"))
	rewritten := bytes.ReplaceAll(bytes.ReplaceAll(frame, []byte("\r"), []byte("\x1b[1G")), []byte("\n"), []byte("\x1b[B"))
	var calls []string
	console := consolewriter.NewConsole(func(u []uint16) (int, error) {
		calls = append(calls, string(utf16.Decode(u)))
		return len(u), nil
	})
	frames := newConsoleFrames(console)
	if _, err := frames.Write(frame); err != nil {
		t.Fatal(err)
	}
	want := framePieces(rewritten, pieceBound)
	if len(calls) != len(want) {
		t.Fatalf("the console received %d calls, and framePieces answers %d pieces", len(calls), len(want))
	}
	for i := range want {
		if calls[i] != string(want[i]) {
			t.Errorf("call %d differs from piece %d", i, i)
		}
	}
	t.Logf("%d calls, each one piece", len(calls))
}
