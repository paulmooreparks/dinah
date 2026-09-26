package main

import (
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
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

// TestTheScreenLayoutHelpersHoldTheirWidths holds the layout helpers of the
// terminal head that table.go carries for dinah-tui, in the untagged build
// whose coverage pass reads table.go: the heading keeps who is acting at the
// right edge where both parts fit and cuts the left part alone where they do
// not, an unselected list row carries no marker and no reverse video, the
// list pane takes two fifths of the width, the panes lay the detail beside
// the list or draw the list alone, colour spans past a cut are dropped, and
// the rule and the cut hold their widths.
func TestTheScreenLayoutHelpersHoldTheirWidths(t *testing.T) {
	heading := interactiveHeading("Workbench · Board", "acting as alka", 60, tailEllipsis)
	if displayWidth(heading.text) > 60 || !strings.HasSuffix(heading.text, "acting as alka") {
		t.Errorf("a heading with room for both parts drew %q", heading.text)
	}
	if len(heading.bold) != 1 || heading.text[heading.bold[0].start:heading.bold[0].end] != "Workbench · Board" {
		t.Errorf("the heading's bold range %v does not cover the left part alone", heading.bold)
	}
	narrow := interactiveHeading("Workbench · Board", "acting as alka", 12, tailEllipsis)
	if displayWidth(narrow.text) > 12 || strings.Contains(narrow.text, "alka") || len(narrow.bold) != 1 || narrow.bold[0].end != len(narrow.text) {
		t.Errorf("a heading without room for both parts drew %q bold over %v", narrow.text, narrow.bold)
	}

	card := boardCard{glyph: "o", number: "7", title: "A title"}
	row := interactiveListRow(card, false, 30, plainGlyphs, ">")
	if strings.HasPrefix(row.text, ">") || len(row.reverse) != 0 || displayWidth(row.text) != 30 {
		t.Errorf("an unselected row drew %q with reverse video %v", row.text, row.reverse)
	}
	if got := interactiveListWidth(100); got != 40 {
		t.Errorf("the list pane of a 100-column draw is %d columns, wanted 40", got)
	}

	list := []interactiveLine{
		{drawnLine: drawnLine{text: "> first", spans: []colourSpan{{start: 2, end: 7}, {start: 30, end: 40}}}, reverse: []byteRange{{start: 0, end: 7}}},
		{drawnLine: drawnLine{text: "  second"}},
	}
	alone := interactivePanes(list, nil, 100, 3, "│", tailEllipsis)
	if len(alone) != 3 || alone[0].text != "> first" || alone[2].text != "" {
		t.Errorf("the list pane alone drew %q", []string{alone[0].text, alone[1].text, alone[2].text})
	}
	beside := interactivePanes(list, []string{"fx-1  A title", strings.Repeat("d", 200)}, 100, 3, "│", tailEllipsis)
	for i, line := range beside {
		if displayWidth(line.text) > 100 || !strings.Contains(line.text, "│") {
			t.Errorf("row %d of the two panes drew %q", i, line.text)
		}
	}
	if !strings.HasSuffix(beside[0].text, "│fx-1  A title") || len(beside[0].reverse) != 1 || len(beside[0].spans) != 1 {
		t.Errorf("the first row of the two panes drew %q with reverse %v and spans %v", beside[0].text, beside[0].reverse, beside[0].spans)
	}
	if kept := clampSpans([]colourSpan{{start: 0, end: 4}, {start: 3, end: 9}}, 5); len(kept) != 1 || kept[0].end != 4 {
		t.Errorf("clamping spans to five bytes kept %v", kept)
	}
	if got := interactiveRule(plainGlyphs, 12); displayWidth(got) != 12 {
		t.Errorf("a 12-column rule drew %q", got)
	}
	if got := interactiveCut("a line longer than the room", 10, "..."); displayWidth(got) > 10 || !strings.HasSuffix(got, "...") {
		t.Errorf("a cut line drew %q", got)
	}
	if !interactiveFits("ten column", 10) || interactiveFits("eleven cols", 10) {
		t.Error("interactiveFits did not answer ten columns as fitting ten and eleven as not")
	}
	laid := interactiveColumns([][]string{{"fx-1/criteria/1", "pending", "It builds."}, {"fx-1/questions/12", "resolved", "Which?"}, {}, {"only"}})
	want := []string{"fx-1/criteria/1    pending   It builds.", "fx-1/questions/12  resolved  Which?", "", "only"}
	if strings.Join(laid, "\n") != strings.Join(want, "\n") {
		t.Errorf("interactiveColumns laid out\n%q\nand the columns should have been\n%q", laid, want)
	}
}

// TestAStaleOrUnreachableAnswerComposesItsOutcomeLine holds the two branches
// of outcomeLines that no untagged command reaches, which dinah-tui's message
// area draws: a stale answer names its outcome and the card's revision, and
// an unreachable one names its outcome and its detail.
func TestAStaleOrUnreachableAnswerComposesItsOutcomeLine(t *testing.T) {
	s := helpSession(80, "en")
	stale := s.outcomeLines(&verb.Response{Outcome: contract.OutcomeStale, Card: &verb.CardView{Revision: "r-42"}})
	if len(stale) != 1 || !strings.HasPrefix(stale[0], contract.OutcomeStale+" ") || !strings.Contains(stale[0], "r-42") {
		t.Errorf("a stale answer composed %q", stale)
	}
	bare := s.outcomeLines(&verb.Response{Outcome: contract.OutcomeStale})
	if len(bare) != 1 || !strings.HasPrefix(bare[0], contract.OutcomeStale+" ") {
		t.Errorf("a stale answer with no card composed %q", bare)
	}
	unreachable := s.outcomeLines(&verb.Response{Outcome: contract.OutcomeUnreachable, Detail: "the lock is held"})
	if len(unreachable) != 1 || !strings.HasPrefix(unreachable[0], contract.OutcomeUnreachable+" ") || !strings.Contains(unreachable[0], "the lock is held") {
		t.Errorf("an unreachable answer composed %q", unreachable)
	}
}
