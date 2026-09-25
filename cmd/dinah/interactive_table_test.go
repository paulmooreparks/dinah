package main

import (
	"strconv"
	"strings"
	"testing"
)

// TestTheLaneBarKeepsTheFocusedLaneWholeAndMarksWhatItCut holds the lane
// bar's cutting rule: at every focus the bar fits the room, the focused lane
// is drawn whole between brackets and in reverse video, each end that lost a
// lane shows the ellipsis, a focused lane wider than the room is cut to it
// with its reverse video cut to match, and no lanes or a focus outside them
// draw nothing.
func TestTheLaneBarKeepsTheFocusedLaneWholeAndMarksWhatItCut(t *testing.T) {
	var lanes []interactiveLane
	for i := 0; i < 10; i++ {
		lanes = append(lanes, interactiveLane{label: "Lane " + strconv.Itoa(i), operator: i == 4, count: "(3)"})
	}
	for _, focus := range []int{0, 4, 9} {
		line := interactiveLaneBar(lanes, focus, 40, unicodeGlyphs)
		focused := "[Lane " + strconv.Itoa(focus)
		if got := displayWidth(line.text); got > 40 {
			t.Errorf("focus %d: the bar draws %d columns in a room of 40", focus, got)
		}
		if !strings.Contains(line.text, focused) {
			t.Errorf("focus %d: the bar %q does not draw the focused lane whole", focus, line.text)
		}
		if focus > 0 && !strings.HasPrefix(line.text, tailEllipsis) {
			t.Errorf("focus %d: the bar %q does not mark the lanes cut before it", focus, line.text)
		}
		if focus < 9 && !strings.HasSuffix(line.text, tailEllipsis) {
			t.Errorf("focus %d: the bar %q does not mark the lanes cut after it", focus, line.text)
		}
		if len(line.reverse) != 1 || !strings.HasPrefix(line.text[line.reverse[0].start:line.reverse[0].end], focused) {
			t.Errorf("focus %d: the reverse video %v does not cover the focused lane", focus, line.reverse)
		}
	}
	wide := []interactiveLane{{label: strings.Repeat("W", 30), count: "(1)"}}
	cut := interactiveLaneBar(wide, 0, 12, plainGlyphs)
	if displayWidth(cut.text) > 12 || !strings.HasSuffix(cut.text, "...") {
		t.Errorf("a focused lane wider than the room drew %q", cut.text)
	}
	if len(cut.reverse) != 1 || cut.reverse[0].end > len(cut.text) {
		t.Errorf("the reverse video %v outruns the cut bar %q", cut.reverse, cut.text)
	}
	if empty := interactiveLaneBar(nil, 0, 40, unicodeGlyphs); empty.text != "" {
		t.Errorf("no lanes drew %q", empty.text)
	}
	if outside := interactiveLaneBar(lanes, 12, 40, unicodeGlyphs); outside.text != "" {
		t.Errorf("a focus outside the lanes drew %q", outside.text)
	}
	if kept := clampRanges([]byteRange{{2, 9}, {12, 15}}, 5); len(kept) != 1 || kept[0] != (byteRange{2, 5}) {
		t.Errorf("clamping to five bytes kept %v", kept)
	}
}

// TestTheListRowFitsAPaneTooNarrowForItsTitle holds a list row in a pane
// narrower than the card's glyph, number and minimum title: it still fits.
func TestTheListRowFitsAPaneTooNarrowForItsTitle(t *testing.T) {
	card := boardCard{glyph: "o", number: "12345", holder: "somebody", priority: "now", title: "A title"}
	for _, width := range []int{6, 9, 14, 40} {
		row := interactiveListRow(card, true, width, plainGlyphs, ">")
		if got := displayWidth(row.text); got > width {
			t.Errorf("at %d the row %q draws %d columns", width, row.text, got)
		}
	}
}

// TestTheMenuScrollsToItsHighlight holds the move menu: a highlight below the
// rows that fit scrolls them so it is shown with the marker, and a menu given
// one row draws its title alone.
func TestTheMenuScrollsToItsHighlight(t *testing.T) {
	var rows []string
	for i := 0; i < 20; i++ {
		rows = append(rows, "Column "+strconv.Itoa(i))
	}
	lines := interactiveMenu("move fx-1 to:", rows, 15, 40, 5, plainGlyphs, ">")
	if len(lines) != 5 || !strings.HasPrefix(lines[4].text, "> Column 15") || len(lines[4].reverse) != 1 {
		var texts []string
		for _, line := range lines {
			texts = append(texts, line.text)
		}
		t.Errorf("the menu scrolled to %q, wanted Column 15 highlighted on its last row", texts)
	}
	if title := interactiveMenu("move fx-1 to:", rows, 0, 40, 1, plainGlyphs, ">"); len(title) != 1 {
		t.Errorf("a menu of one row drew %d rows", len(title))
	}
}

// TestTheMessageAreaMarksTheLinesItLeavesOut holds the message area: at most
// its limit of lines is shown, each fits the room, and the last one shown ends
// in the ellipsis when lines were left out after it, whether it fits the room
// or has to be cut.
func TestTheMessageAreaMarksTheLinesItLeavesOut(t *testing.T) {
	short := interactiveMessage([]string{"one", "two", "three", "four"}, 3, 20, "...")
	if len(short) != 3 || short[2] != "three..." {
		t.Errorf("four short lines at a limit of three showed %q", short)
	}
	long := interactiveMessage([]string{"one", "two", strings.Repeat("x", 30), "four"}, 3, 20, "...")
	if len(long) != 3 || displayWidth(long[2]) > 20 || !strings.HasSuffix(long[2], "...") {
		t.Errorf("a long third line with a fourth after it showed %q", long)
	}
	whole := interactiveMessage([]string{"one", "two"}, 3, 20, "...")
	if strings.Join(whole, "|") != "one|two" {
		t.Errorf("two lines under the limit showed %q", whole)
	}
}
