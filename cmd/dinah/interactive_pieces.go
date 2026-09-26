//go:build tui

package main

import (
	"bytes"
	"io"
	"os"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/rivo/uniseg"

	"dinah/internal/consolewriter"
	"dinah/internal/screen"
)

// pieceBound is the most UTF-16 code units consoleFrames hands the console
// in one piece. WriteConsoleW counts what it writes in those units, and the
// console writer of internal/consolewriter submits at most 8,000 of them in
// one call, so each piece reaches the console as exactly one WriteConsoleW
// call. Microsoft documents no fixed ceiling for that call, and the number is
// the console writer's engineering margin rather than a claimed limit.
const pieceBound = 8000

// frameUnit is one unit framePieces reads: a whole escape sequence, or one
// extended grapheme cluster of text, as a span of the input and the number of
// UTF-16 code units it encodes to.
type frameUnit struct {
	start, end int
	units      int
	escape     bool
}

// escapeExtent measures an escape sequence starting at buf[0], an ESC, by
// ECMA-48's grammar as the key decoder uses it: ESC [ then any parameter
// bytes 0x30 to 0x3F, any intermediate bytes 0x20 to 0x2F and one final
// byte 0x40 to 0x7E; ESC ] then an OSC string ended by BEL or ESC \; and ESC
// with one more byte from 0x20 to 0x7E. It answers the length and whether
// the sequence is complete. A byte the grammar does not allow ends a
// malformed sequence where it stands, so the unit never swallows it.
func escapeExtent(buf []byte) (int, bool) {
	if len(buf) < 2 {
		return len(buf), false
	}
	switch next := buf[1]; {
	case next == '[':
		i := 2
		for i < len(buf) && buf[i] >= 0x30 && buf[i] <= 0x3f {
			i++
		}
		for i < len(buf) && buf[i] >= 0x20 && buf[i] <= 0x2f {
			i++
		}
		if i >= len(buf) {
			return len(buf), false
		}
		if buf[i] >= 0x40 && buf[i] <= 0x7e {
			return i + 1, true
		}
		return i, true
	case next == ']':
		for i := 2; i < len(buf); i++ {
			if buf[i] == 0x07 {
				return i + 1, true
			}
			if buf[i] == 0x1b && i+1 < len(buf) && buf[i+1] == '\\' {
				return i + 2, true
			}
		}
		return len(buf), false
	case next >= 0x20 && next <= 0x7e:
		return 2, true
	}
	return 1, true
}

// frameUnits reads buf as a run of units, and answers them with how many
// bytes at its end begin a unit whose end has not arrived: an escape
// sequence or a UTF-8 sequence that is incomplete.
func frameUnits(buf []byte) (units []frameUnit, held int) {
	state := -1
	for at := 0; at < len(buf); {
		if buf[at] == 0x1b {
			length, complete := escapeExtent(buf[at:])
			if !complete {
				return units, len(buf) - at
			}
			units = append(units, frameUnit{start: at, end: at + length, units: utf16Length(buf[at : at+length]), escape: true})
			at += length
			state = -1
			continue
		}
		if !utf8.FullRune(buf[at:]) {
			return units, len(buf) - at
		}
		text := buf[at:]
		if next := bytes.IndexByte(text, 0x1b); next >= 0 {
			text = text[:next]
		}
		cluster, _, _, newState := uniseg.FirstGraphemeCluster(text, state)
		state = newState
		units = append(units, frameUnit{start: at, end: at + len(cluster), units: utf16Length(cluster)})
		at += len(cluster)
	}
	return units, 0
}

// framePieces cuts buf into pieces of at most bound UTF-16 code units each,
// cutting only between whole units, so no cut falls inside an escape
// sequence, a UTF-8 sequence or a grapheme cluster. When the next unit would
// take a piece past the bound, the piece is closed before it. Where that unit
// is text, the cut moves back to just before the last escape sequence in the
// piece, so a run of text stays with the sequence that positioned it, unless
// that would leave the piece empty. A single unit longer than the bound is a
// piece of its own. buf is read whole: the caller holds back any incomplete
// tail.
func framePieces(buf []byte, bound int) [][]byte {
	units, _ := frameUnits(buf)
	var pieces [][]byte
	first, size, lastEscape := 0, 0, -1
	for i, unit := range units {
		if size > 0 && size+unit.units > bound {
			cut := i
			if !unit.escape && lastEscape > first && unitsOf(units[lastEscape:i])+unit.units <= bound {
				cut = lastEscape
			}
			pieces = append(pieces, buf[units[first].start:units[cut-1].end])
			first, size, lastEscape = cut, unitsOf(units[cut:i]), -1
		}
		if unit.escape && i > first {
			lastEscape = i
		}
		size += unit.units
	}
	if first < len(units) {
		pieces = append(pieces, buf[units[first].start:units[len(units)-1].end])
	}
	return pieces
}

// unitsOf is the number of UTF-16 code units a run of units encodes to.
func unitsOf(units []frameUnit) int {
	total := 0
	for _, unit := range units {
		total += unit.units
	}
	return total
}

// utf16Length is the number of UTF-16 code units text encodes to, which is
// how WriteConsoleW counts it.
func utf16Length(text []byte) int {
	total := 0
	for _, r := range string(text) {
		total += utf16.RuneLen(r)
	}
	return total
}

// pieceWriter is where consoleFrames writes each piece. WriteAll writes the
// whole piece and reports whether the console wrote fewer characters than one
// of its calls was given, as consolewriter.Writer.WriteAll does.
type pieceWriter interface {
	WriteAll(p []byte) (short bool, err error)
}

// plainPieces writes each piece to a writer that is not a console, such as a
// test's buffer, with one Write, and never reports a short write.
type plainPieces struct {
	w io.Writer
}

// WriteAll writes piece with one call.
func (p plainPieces) WriteAll(piece []byte) (bool, error) {
	_, err := p.w.Write(piece)
	return false, err
}

// newConsoleFrames answers the frames writer around output. A file, which is
// the console the head draws on, is written through the console writer of
// internal/consolewriter, which calls WriteConsoleW itself and reads the
// count it reports. A writer that already writes pieces is used as it is, and
// any other writer takes each piece in one Write.
func newConsoleFrames(output io.Writer) *consoleFrames {
	switch w := output.(type) {
	case pieceWriter:
		return &consoleFrames{w: w}
	case *os.File:
		return &consoleFrames{w: consolewriter.New(w)}
	}
	return &consoleFrames{w: plainPieces{w: output}}
}

// consoleFrames is the writer Bubble Tea draws through on Windows. It is not
// a terminal file, so Bubble Tea treats the output as no terminal: it does
// not turn on hard tabs or backspace moves and leaves the console mode alone.
// It rewrites every line feed as ESC [B and every carriage return as ESC
// [1G, the cursor movements the renderer means by them, which Microsoft's
// "Console Virtual Terminal Sequences" page lists, and it hands the console
// each frame in pieces of at most pieceBound UTF-16 code units that
// framePieces cuts, each written with one WriteConsoleW call. An escape or
// UTF-8 sequence incomplete at the end of a Write is held and joined to the
// next.
//
// Microsoft's WriteConsole page reports how many characters a call wrote and
// does not say when that can be fewer than it was given. Where it is, the
// console writer writes the rest of the piece in a further call, and a
// sequence may then have reached the console in two parts. consoleFrames
// calls repaint when that happens, and the head answers by repainting the
// whole screen on the next frame, so a frame the split drew wrongly is
// replaced at the next one.
type consoleFrames struct {
	w    pieceWriter
	held []byte
	// repaint is called, from the renderer's goroutine, when a piece was
	// written short. It must not block.
	repaint func()
}

// lineMoves are the two control characters the renderer writes to move the
// cursor, and the sequences that move it the same way.
var lineMoves = map[byte][]byte{
	'\n': []byte(screen.ConsoleCursorDown),
	'\r': []byte(screen.ConsoleCursorColumnOne),
}

// Write rewrites p, cuts it into pieces and writes each with one call.
func (f *consoleFrames) Write(p []byte) (int, error) {
	buf := append(f.held, p...)
	var rewritten bytes.Buffer
	for _, b := range buf {
		if move, ok := lineMoves[b]; ok {
			rewritten.Write(move)
			continue
		}
		rewritten.WriteByte(b)
	}
	data := rewritten.Bytes()
	_, held := frameUnits(data)
	complete := data[:len(data)-held]
	f.held = append([]byte(nil), data[len(complete):]...)
	for _, piece := range framePieces(complete, pieceBound) {
		if err := f.writePiece(piece); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// writePiece writes one piece and asks for a repaint when it was written
// short.
func (f *consoleFrames) writePiece(piece []byte) error {
	short, err := f.w.WriteAll(piece)
	if short && f.repaint != nil {
		f.repaint()
	}
	return err
}

// Flush writes whatever is held, whole or not, which the head does as it
// leaves, just before the output mode is restored.
func (f *consoleFrames) Flush() error {
	if len(f.held) == 0 {
		return nil
	}
	held := f.held
	f.held = nil
	if err := f.writePiece(held); err != nil {
		return err
	}
	if flusher, ok := f.w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}
