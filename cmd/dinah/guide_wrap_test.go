package main

import (
	"strings"
	"testing"

	"dinah/internal/guide"
)

// guideTestWidths are the widths the pass-through fixtures are checked at:
// three windows and the unknown one.
var guideTestWidths = []int{20, 40, 80, 0}

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
// window keeps its marker on the first line and indents every line after it
// to the marker's own width.
func TestALongHeadingWrapsUnderItsMarker(t *testing.T) {
	source := "## A state says how much work it will hold and says so before you ask\n"
	wrapped := wrapGuideText(source, 40)
	lines := strings.Split(strings.TrimRight(wrapped, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("heading did not wrap: %q", wrapped)
	}
	if !strings.HasPrefix(lines[0], "## A state") {
		t.Errorf("first line lost its marker: %q", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, "   ") || strings.HasPrefix(line, "    ") {
			t.Errorf("continuation is not indented to the marker's width: %q", line)
		}
	}
	checkWrappedTo(t, source, wrapped, 40)
}

// TestBothListItemSourceShapesWrapAlike asserts that an item the source
// hard-wrapped with a two-space continuation and the same item written as one
// long line reach the reader as the same lines.
func TestBothListItemSourceShapesWrapAlike(t *testing.T) {
	broken := "- `state` is the state the card is in. Give it a state's short name or its\n  identifier.\n"
	whole := "- `state` is the state the card is in. Give it a state's short name or its identifier.\n"
	fromBroken := wrapGuideText(broken, 40)
	fromWhole := wrapGuideText(whole, 40)
	if fromBroken != fromWhole {
		t.Errorf("the two source shapes wrapped differently\nbroken: %q\n whole: %q", fromBroken, fromWhole)
	}
	lines := strings.Split(strings.TrimRight(fromBroken, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("the item did not wrap: %q", fromBroken)
	}
	if !strings.HasPrefix(lines[0], "- ") {
		t.Errorf("first line lost its marker: %q", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
			t.Errorf("continuation is not indented to the marker's width: %q", line)
		}
	}
	checkWrappedTo(t, broken, fromBroken, 40)
}

// TestABlockQuoteKeepsItsMarkerOnEveryLine asserts that a quoted paragraph
// wraps with its marker reproduced on continuation lines as well as the
// first. No shipped guide carries one, so the fixture is built here.
func TestABlockQuoteKeepsItsMarkerOnEveryLine(t *testing.T) {
	source := "> The operator answers for the workbench and is the only owner who may\n" +
		"> lift a block, so a workbench with nobody in that seat has actions\n" +
		"> nobody can perform.\n"
	wrapped := wrapGuideText(source, 40)
	lines := strings.Split(strings.TrimRight(wrapped, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("the quote did not wrap: %q", wrapped)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "> ") {
			t.Errorf("a quoted line lost its marker: %q", line)
		}
		if displayWidth(line) > 40 {
			t.Errorf("a quoted line draws %d columns: %q", displayWidth(line), line)
		}
	}
	if got, want := flattenWords(strings.ReplaceAll(wrapped, ">", "")), flattenWords(strings.ReplaceAll(source, ">", "")); got != want {
		t.Errorf("words changed across the wrap\n got: %q\nwant: %q", got, want)
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
