package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTheChecklistTablesBlankCommentCellLinesUp asserts dinah-502 AC-6's
// rendered half at the command surface rather than only at the library:
// dinah show <card> draws a blank cell, not a zero, against an item carrying
// no comments, and that blank cell still lines up with the counted one above
// it.
//
// Both fixtures that already draw the checklist block, in
// row_sweep_test.go and checklist_prose_test.go, give every item a nonzero
// count on purpose (dinah-502's own Agent Code Review round 1 major finding
// explains why: a blank cell corrupts deriveHeadinglessColumns's own
// column-boundary search when only some rows carry it). This test builds a
// fixture of its own, through dinah file and dinah comment rather than by
// hand, specifically to carry the case those two deliberately do not: one
// item with comments beside one item with none.
func TestTheChecklistTablesBlankCommentCellLinesUp(t *testing.T) {
	root := newBench(t)
	mustRun(t, root, "add", "a card carrying two questions")
	mustRun(t, root, "file", "fx-1", "open_question", "the first question")
	mustRun(t, root, "file", "fx-1", "open_question", "the second question")
	mustRun(t, root, "comment", "fx-1/questions/1", "a first thought")
	mustRun(t, root, "comment", "fx-1/questions/1", "a second thought")

	t.Setenv("COLUMNS", "80")
	got := runCLI(t, root, "show", "fx-1")
	if got.code != 0 {
		t.Fatalf("show fx-1: %d %s", got.code, got.errw)
	}
	var rows []string
	for _, line := range strings.Split(got.out, "\n") {
		if strings.Contains(line, "fx-1/questions/") {
			rows = append(rows, line)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("wanted two checklist rows, got %d:\n%s", len(rows), got.out)
	}
	if !strings.Contains(rows[0], "questions/1") || !strings.Contains(rows[1], "questions/2") {
		t.Fatalf("the rows are not in creation order:\n%s\n%s", rows[0], rows[1])
	}
	countedRowText := "the first question"
	blankRowText := "the second question"
	countedAt := strings.Index(rows[0], countedRowText)
	if countedAt < 0 {
		t.Fatalf("the counted row does not carry its own text: %q", rows[0])
	}
	countedCell := strings.Index(rows[0], "2")
	if countedCell < 0 || countedCell >= countedAt {
		t.Fatalf("the counted row carries no 2 ahead of its own text: %q", rows[0])
	}
	// The blank row's own text has to start at the same display column the
	// counted row's does, which is only true if the blank cell was drawn at
	// its full width (the comments column's own width, empty) rather than
	// collapsed to nothing while a neighbouring row still carries a count.
	textAt := strings.Index(rows[1], blankRowText)
	if textAt < 0 {
		t.Fatalf("the blank row does not carry its own text: %q", rows[1])
	}
	if textAt != countedAt {
		t.Errorf("the blank row's text starts at column %d and the counted row's own text starts at %d, so the comments cell is not blank at its own width:\n%s\n%s",
			textAt, countedAt, rows[0], rows[1])
	}
	// The cell itself, not only its width, is asserted: the gap between the
	// state field and the row's own text carries nothing but padding, so a
	// render that drew "0" at the count's own width would pass the position
	// check above and fail this one.
	stateAt := strings.Index(rows[1], "pending")
	if stateAt < 0 {
		t.Fatalf("the blank row does not carry its own state: %q", rows[1])
	}
	gap := rows[1][stateAt+len("pending") : textAt]
	if strings.TrimSpace(gap) != "" {
		t.Errorf("the blank row's comments cell carries %q rather than blank padding: %q", strings.TrimSpace(gap), rows[1])
	}

	machine := runCLI(t, root, "--json", "show", "fx-1")
	if machine.code != 0 {
		t.Fatalf("show fx-1 --json: %d %s", machine.code, machine.errw)
	}
	var payload struct {
		Checklist []map[string]json.RawMessage `json:"checklist"`
	}
	if err := json.Unmarshal([]byte(machine.out), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	if len(payload.Checklist) != 2 {
		t.Fatalf("wanted two checklist items in the payload, got %d", len(payload.Checklist))
	}
	if _, carried := payload.Checklist[0]["comment_count"]; !carried {
		t.Errorf("the first item's payload carries no comment_count: %s", machine.out)
	}
	if _, carried := payload.Checklist[1]["comment_count"]; carried {
		t.Errorf("the second item's payload carries comment_count though it holds no comments: %s", machine.out)
	}
}
