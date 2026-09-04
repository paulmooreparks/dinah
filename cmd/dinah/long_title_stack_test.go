package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// longTitleOverrunningItsHeading is a workbench title far wider than the
// "Workbench" heading it sits under, which is what takes the listing past the
// point where a title can share a line with the values after it.
//
// Its width is what the case rests on, so the constant is measured rather than
// trusted: longTitleStackedListing fails when this title stops clearing the
// heading by the margin the case needs.
const longTitleOverrunningItsHeading = "This is a deliberately very long workbench name that outruns the title column width by a lot"

// longTitleClearance is how many display columns the long title has to stand
// above its own heading before the case is exercising the fallback rather than
// an ordinary wide row. Twenty is wide enough that no window this suite runs at
// can seat the title beside its neighbours.
const longTitleClearance = 20

// TestALongTitleStacksTheWorkbenchListingRatherThanGluingTheNextColumnToIt
// pins the fallback a title too wide for its own column takes in `dinah
// workbenches`.
//
// The card this test lands under (dinah-94) reported the opposite: a title
// that outran its column with the slug glued straight onto its tail and no
// separator between the two. That failure was closed by the shared row
// renderer, and the reproduction that closed it observed the listing fall back
// to the labelled stacked form instead. Nothing pinned that observation, so
// this test does.
//
// It asserts the whole block, line for line, rather than asserting that output
// appeared. Three things are being held:
//
//  1. No heading row and no separator rule are drawn, which is what tells a
//     stack from a table.
//  2. The long title occupies its own line whole, with nothing after it. This
//     is the glue the card described, and it is the assertion that goes red
//     when the fallback stops firing.
//  3. Every value in the block starts at one display column, because each
//     label is padded to the widest heading in the listing.
//
// The listing is drawn by running the head, not by calling the renderer, so
// what is asserted is what a reader sees.
func TestALongTitleStacksTheWorkbenchListingRatherThanGluingTheNextColumnToIt(t *testing.T) {
	if got := displayWidth(longTitleOverrunningItsHeading) - displayWidth(longTitleHeading); got < longTitleClearance {
		t.Fatalf("the fixture title stands %d display columns above its heading, and the case needs at least %d", got, longTitleClearance)
	}

	container := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(container, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("COLUMNS", "")

	base := filepath.Join(container, bench.UserBaseName)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir the user base: %v", err)
	}
	rooms := populateBase(t, base, "long", "short")
	sweptRetitle(t, rooms[0], longTitleOverrunningItsHeading)
	sweptRetitle(t, rooms[1], ".")

	resolved := filepath.Join(resolvedDir(t, container), bench.UserBaseName)
	got := sweptRun(t, container, "en", "workbenches")

	wanted := strings.Join([]string{
		longTitleStackedRecord(longTitleOverrunningItsHeading, "long", filepath.Join(resolved, filepath.Base(rooms[0]))),
		longTitleStackedRecord(".", "short", filepath.Join(resolved, filepath.Base(rooms[1]))),
	}, "\n")
	if got != wanted {
		t.Fatalf("the listing did not fall back to the stacked form.\n got:\n%s\nwanted:\n%s", got, wanted)
	}
}

// The three headings the workbench listing declares, in the order it declares
// them, as the English catalog spells them. They are named here rather than
// read from the catalog because the case runs at one language on purpose: the
// sweep in row_sweep_test.go is what holds every catalog, and this test holds
// one shape.
const (
	longTitleHeading = "Workbench"
	longSlugHeading  = "Slug"
	longPathHeading  = "Path"
)

// longTitleStackedRecord draws the block a single workbench takes in the
// stacked form: one field to a line, the heading in front of the value as its
// label, and every label padded to the widest heading in the listing. One
// blank line separates one record from the next, which is why the caller joins
// the records rather than each record closing itself.
//
// The padding is computed from the headings rather than typed, so the
// expectation moves with the catalog instead of pinning a count of spaces that
// only holds while the words keep their present length.
func longTitleStackedRecord(title, slug, path string) string {
	label := displayWidth(longTitleHeading)
	if w := displayWidth(longSlugHeading); w > label {
		label = w
	}
	if w := displayWidth(longPathHeading); w > label {
		label = w
	}
	lines := make([]string, 0, 4)
	for _, field := range [][2]string{
		{longTitleHeading, title},
		{longSlugHeading, slug},
		{longPathHeading, path},
	} {
		pad := strings.Repeat(" ", label-displayWidth(field[0])+tableGutter)
		lines = append(lines, "  "+field[0]+pad+field[1])
	}
	return strings.Join(lines, "\n") + "\n"
}
