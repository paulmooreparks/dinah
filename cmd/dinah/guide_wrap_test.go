package main

import (
	"strings"
	"testing"

	"dinah/internal/guide"
)

// guideTestWidths are the widths the pass-through fixtures are checked at:
// three windows and the unknown one.
var guideTestWidths = []int{20, 40, 80, 0}

// guideWrapWidths are the windows every marker-bearing fixture below is held
// across, rather than the one width the fixture happened to be drawn at. A
// rule checked at a single width proves the drawing and leaves the defect
// living at every width nobody drew: the room deduction wrappedLines makes is
// visible at 40 columns on one line of one guide and invisible at 80, so a
// suite pinned to 40 and 80 rests on the wording of that one line. Every
// fixture here is longer than the widest of these windows, so all of them
// wrap.
func guideWrapWidths() []int {
	var widths []int
	for width := 20; width <= 60; width++ {
		widths = append(widths, width)
	}
	return widths
}

// flattenWords reduces text to its words separated by single spaces, which is
// what a wrap is allowed to change nothing about.
func flattenWords(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// checkWrappedTo asserts that no line of text draws wider than width and that
// the words survived the wrap in their original order.
func checkWrappedTo(t *testing.T, source, wrapped string, width int) {
	t.Helper()
	for _, line := range strings.Split(wrapped, "\n") {
		if displayWidth(line) > width {
			t.Errorf("line draws %d columns at width %d: %q", displayWidth(line), width, line)
		}
	}
	if got, want := flattenWords(wrapped), flattenWords(source); got != want {
		t.Errorf("words changed across the wrap\n got: %q\nwant: %q", got, want)
	}
}

// TestAGuideWrittenToAnUnknownWindowIsUnchanged asserts that a width no
// documented source answered leaves the guide's own bytes alone, which is what
// keeps a redirected guide identical to its source.
func TestAGuideWrittenToAnUnknownWindowIsUnchanged(t *testing.T) {
	source := "# A heading\n\nA paragraph that runs on for a good while without any break in it at all.\n\n- an item\n"
	for _, width := range []int{0, -1, -80} {
		if got := wrapGuideText(source, width); got != source {
			t.Errorf("width %d changed the text\n got: %q\nwant: %q", width, got, source)
		}
	}
}

// TestAHardWrappedGuideParagraphRewrapsToTheWindow asserts that a paragraph
// the source already broke near 79 columns is rejoined and rewrapped, so the
// source's own choice of width stops reaching the reader.
func TestAHardWrappedGuideParagraphRewrapsToTheWindow(t *testing.T) {
	source := "Dinah keeps a workbench as a directory of plain-text files. You can read\n" +
		"everything Dinah tracks about your work there with an editor, search it with\n" +
		"grep, and put it under git alongside the project it belongs to.\n"
	checkWrappedTo(t, source, wrapGuideText(source, 40), 40)
}

// TestAnUnwrappedGuideParagraphRewrapsToTheWindow asserts the same of a
// paragraph the source wrote as one continuous line, which is the shape the
// terminal breaks mid-word today.
func TestAnUnwrappedGuideParagraphRewrapsToTheWindow(t *testing.T) {
	source := "If you let a state accept every card offered to it, you stop having a station and start having a pile. Cards standing in a state are inventory. None of them are finished, and the time they spend waiting is time you have already paid for. A card that sits does not announce what stopped it either, so whatever stalled it stays out of sight for as long as you tolerate the pile. Setting a limit on how much a state holds means you meet that cost on the day the queue grows, as a refusal, instead of working it out a month later.\n"
	checkWrappedTo(t, source, wrapGuideText(source, 40), 40)
}

// TestAFencedBlockIsReproducedWhole asserts that a fenced block, including a
// line inside it wider than the window, reaches the stream byte for byte.
func TestAFencedBlockIsReproducedWhole(t *testing.T) {
	source := "```\n" +
		"  Slug    Name    Kind    Cards  Owner\n" +
		"  intake  Intake  intake  1      agent\n" +
		"  a line inside the fence that is a good deal wider than any of the test widths\n" +
		"```\n"
	for _, width := range guideTestWidths {
		if got := wrapGuideText(source, width); got != source {
			t.Errorf("width %d re-flowed a fenced block\n got: %q\nwant: %q", width, got, source)
		}
	}
}

// TestAnIndentedCodeBlockIsReproducedWhole asserts the same of the fenceless
// indented blocks the query guide spells its examples with.
func TestAnIndentedCodeBlockIsReproducedWhole(t *testing.T) {
	source := "    dinah query \"actor:alka\"\n" +
		"    dinah query \"state:doing substate:ready\"\n" +
		"    dinah query \"entered:done at>=2026-08-01\"\n"
	for _, width := range guideTestWidths {
		if got := wrapGuideText(source, width); got != source {
			t.Errorf("width %d re-flowed an indented code block\n got: %q\nwant: %q", width, got, source)
		}
	}
}

// TestATableIsReproducedWhole asserts that the reference guide's support
// table keeps the padding its columns line up on.
func TestATableIsReproducedWhole(t *testing.T) {
	source := "| Command      | This workbench | A state | A card | Below a card |\n" +
		"|--------------|----------------|---------|--------|--------------|\n" +
		"| path         | yes            | yes     | yes    | yes          |\n" +
		"| rename       | no             | no      | no     | yes          |\n"
	for _, width := range guideTestWidths {
		if got := wrapGuideText(source, width); got != source {
			t.Errorf("width %d re-flowed a table\n got: %q\nwant: %q", width, got, source)
		}
	}
}

// TestALongHeadingWrapsUnderItsMarker asserts that a heading wider than the
// window keeps its marker on the first line, indents every line after it to
// the marker's own width, and reaches past no window it was wrapped for. The
// three claims are held at every window from 20 to 60 columns rather than at
// the one the fixture was written against.
func TestALongHeadingWrapsUnderItsMarker(t *testing.T) {
	source := "## A state says how much work it will hold and says so before you ask\n"
	for _, width := range guideWrapWidths() {
		wrapped := wrapGuideText(source, width)
		lines := strings.Split(strings.TrimRight(wrapped, "\n"), "\n")
		if len(lines) < 2 {
			t.Errorf("at %d columns the heading did not wrap: %q", width, wrapped)
			continue
		}
		if !strings.HasPrefix(lines[0], "## A state") {
			t.Errorf("at %d columns the first line lost its marker: %q", width, lines[0])
		}
		for _, line := range lines[1:] {
			if !strings.HasPrefix(line, "   ") || strings.HasPrefix(line, "    ") {
				t.Errorf("at %d columns a continuation is not indented to the marker's width: %q", width, line)
			}
		}
		checkWrappedTo(t, source, wrapped, width)
	}
}

// TestBothListItemSourceShapesWrapAlike asserts that an item the source
// hard-wrapped with a two-space continuation and the same item written as one
// long line reach the reader as the same lines, indented to the marker and
// inside the window. Held at every window from 20 to 60 columns, since the
// two shapes agreeing at one width says nothing about their agreeing at
// another, and neither does one width's continuation fitting.
func TestBothListItemSourceShapesWrapAlike(t *testing.T) {
	broken := "- `state` is the state the card is in. Give it a state's short name or its\n  identifier.\n"
	whole := "- `state` is the state the card is in. Give it a state's short name or its identifier.\n"
	for _, width := range guideWrapWidths() {
		fromBroken := wrapGuideText(broken, width)
		fromWhole := wrapGuideText(whole, width)
		if fromBroken != fromWhole {
			t.Errorf("at %d columns the two source shapes wrapped differently\nbroken: %q\n whole: %q", width, fromBroken, fromWhole)
		}
		lines := strings.Split(strings.TrimRight(fromBroken, "\n"), "\n")
		if len(lines) < 2 {
			t.Errorf("at %d columns the item did not wrap: %q", width, fromBroken)
			continue
		}
		if !strings.HasPrefix(lines[0], "- ") {
			t.Errorf("at %d columns the first line lost its marker: %q", width, lines[0])
		}
		for _, line := range lines[1:] {
			if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
				t.Errorf("at %d columns a continuation is not indented to the marker's width: %q", width, line)
			}
		}
		checkWrappedTo(t, broken, fromBroken, width)
	}
}

// TestABlockQuoteKeepsItsMarkerOnEveryLine asserts that a quoted paragraph
// wraps with its marker reproduced on continuation lines as well as the
// first, and that the marker's own room deduction holds the whole quoted line
// inside the window. No shipped guide carries a block quote, so the fixture is
// built here, and it is held at every window from 20 to 60 columns: the
// deduction wrapQuote makes at its own call site is exact at some widths by
// arithmetic accident and only correct at all of them by rule.
func TestABlockQuoteKeepsItsMarkerOnEveryLine(t *testing.T) {
	source := "> The operator answers for the workbench and is the only owner who may\n" +
		"> lift a block, so a workbench with nobody in that seat has actions\n" +
		"> nobody can perform.\n"
	for _, width := range guideWrapWidths() {
		wrapped := wrapGuideText(source, width)
		lines := strings.Split(strings.TrimRight(wrapped, "\n"), "\n")
		if len(lines) < 2 {
			t.Errorf("at %d columns the quote did not wrap: %q", width, wrapped)
			continue
		}
		for _, line := range lines {
			if !strings.HasPrefix(line, "> ") {
				t.Errorf("at %d columns a quoted line lost its marker: %q", width, line)
			}
			if displayWidth(line) > width {
				t.Errorf("at %d columns a quoted line draws %d: %q", width, displayWidth(line), line)
			}
		}
		if got, want := flattenWords(strings.ReplaceAll(wrapped, ">", "")), flattenWords(strings.ReplaceAll(source, ">", "")); got != want {
			t.Errorf("at %d columns words changed across the wrap\n got: %q\nwant: %q", width, got, want)
		}
	}
}

// TestEveryShippedGuideRoundTripsAtAnUnknownWidth asserts that the whole
// corpus survives an unmeasured window untouched, which is the promise the
// guide guards and a redirected guide both rest on.
func TestEveryShippedGuideRoundTripsAtAnUnknownWidth(t *testing.T) {
	topics := guide.Topics()
	if len(topics) == 0 {
		t.Fatal("no guides are embedded")
	}
	for _, topic := range topics {
		text, err := guide.Text(topic)
		if err != nil {
			t.Fatalf("read guide %q: %v", topic, err)
		}
		if got := wrapGuideText(text, 0); got != text {
			t.Errorf("guide %q changed at an unknown width", topic)
		}
	}
}

// guideWrapsNoFurther reports whether a line has nothing left to break: what
// it carries is one word, with or without the marker leading it. breakWords
// writes such a word whole and lets it overrun, which the spec states as the
// wrap's own boundary, so a line of that shape is not held to the window.
func guideWrapsNoFurther(line string) bool {
	body := strings.TrimLeft(line, " ")
	for _, marker := range []string{"> ", "- ", "* "} {
		body = strings.TrimPrefix(body, marker)
	}
	body = strings.TrimLeft(body, "#")
	return len(strings.Fields(body)) <= 1
}

// TestEveryShippedGuideFitsEveryWindowItIsWrappedFor sweeps the whole corpus
// across every window from 20 to 120 columns and requires that no line the
// wrap governs reaches past the window it was wrapped for.
//
// This is the guard the rest of the suite was missing. The width rule was
// held at 40 and 80 on one guide, so the only assertion standing between the
// renderer and a line one column past the edge was a single continuation of a
// single paragraph of principles.md, and rewording that paragraph would have
// removed it without a test going red. A rule the design has to satisfy at
// every width is asserted at every width; 20 is the floor windowWidth clamps
// to and 120 is comfortably past any window the guides were authored for.
//
// Four line shapes are excused, and they are the four the spec names as the
// wrap's boundary rather than a licence taken here: a line inside a fenced
// block, an indented code line, a table row, and a line carrying one
// unbreakable word.
func TestEveryShippedGuideFitsEveryWindowItIsWrappedFor(t *testing.T) {
	topics := guide.Topics()
	if len(topics) == 0 {
		t.Fatal("no guides are embedded")
	}
	for _, topic := range topics {
		text, err := guide.Text(topic)
		if err != nil {
			t.Fatalf("read guide %q: %v", topic, err)
		}
		for width := 20; width <= 120; width++ {
			inside := false
			for _, line := range strings.Split(wrapGuideText(text, width), "\n") {
				if guideOpensFence(line) {
					inside = !inside
					continue
				}
				if inside || guideIsIndentedCode(line) || guideIsTableRow(line) {
					continue
				}
				if guideWrapsNoFurther(line) {
					continue
				}
				if drawn := displayWidth(line); drawn > width {
					t.Errorf("guide %q at %d columns draws a line of %d: %q", topic, width, drawn, line)
				}
			}
		}
	}
}
