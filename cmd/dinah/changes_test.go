package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestTheCheckpointPrintsItsEventsAndItsCursor asserts what a person reads at
// a terminal: a table of the lines that landed since the cursor they held, and
// the cursor to ask with next time.
//
// Three shapes of the card column are drawn on one walk, because each is what
// the column falls back to when the one in front of it cannot be composed: a
// card that still has an anchor draws its reference, a card directory carrying
// a history and no anchor draws its bare identifier, and a workbench-scoped
// line draws the word a person types to name the workbench.
func TestTheCheckpointPrintsItsEventsAndItsCursor(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card that runs into an obstacle"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	cursor := mintedCursor(t, root)

	if got := runCLI(t, root, "block", "fx-1", "the vendor has not answered"); got.code != 0 {
		t.Fatalf("block: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "workbench", "set", "title", "A renamed workbench"); got.code != 0 {
		t.Fatalf("workbench set title: %d %s", got.code, got.errw)
	}
	orphan := writeAnchorlessCard(t, root)

	got := runCLI(t, root, "changes", "--since", cursor)
	if got.code != 0 {
		t.Fatalf("changes: %d %s", got.code, got.errw)
	}
	// Each subject is read off its own row's card column rather than searched
	// for in the whole block. Searching the block proves the string was drawn
	// somewhere, which is the existence assertion standing in for an identity
	// assertion that the counterexamples corpus names, and here it would pass
	// on a renderer that drew every subject in the wrong column.
	rows := changeRows(t, got.out)
	if len(rows) != 3 {
		t.Fatalf("wanted the three lines the fixture wrote, got %d:\n%s", len(rows), got.out)
	}
	remaining := rows
	for _, wanted := range []struct{ detail, subject string }{
		{detail: "the vendor has not answered", subject: "fx-1"},
		{detail: "A card with no anchor", subject: orphan},
	} {
		row, rest := rowCarrying(remaining, wanted.detail)
		if row == nil {
			t.Fatalf("no row carries %q:\n%s", wanted.detail, got.out)
		}
		if row.subject != wanted.subject {
			t.Errorf("the card column of the %q row reads %q, wanted %q", wanted.detail, row.subject, wanted.subject)
		}
		remaining = rest
	}
	// The workbench line is what is left: it is the one act of the three whose
	// detail column is empty, so it cannot be found by a marker of its own.
	if len(remaining) != 1 {
		t.Fatalf("wanted the workbench row left over, got %+v", remaining)
	}
	if remaining[0].subject != bench.WorkbenchKey {
		t.Errorf("the card column of the workbench row reads %q, wanted %q", remaining[0].subject, bench.WorkbenchKey)
	}
	if !strings.Contains(got.out, "cursor: ") {
		t.Errorf("the checkpoint prints no cursor, so the reader has nothing to ask with next:\n%s", got.out)
	}
}

// changeRow is one drawn line of a checkpoint, split far enough to read the
// card column off it.
type changeRow struct {
	subject string
	line    string
}

// changeRows reads the drawn table back, one entry per row rather than per
// printed line, because a narrow terminal wraps one row over several lines.
//
// A timestamp in the first column opens a row and every line under it belongs
// to that row until the next one does, which is what separates the rows from
// the heading above them and the cursor below them. Joined back up, the second
// whitespace-separated field is the card column, since the columns in front of
// it never hold a space.
func changeRows(t *testing.T, out string) []changeRow {
	t.Helper()
	var rows []changeRow
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(strings.TrimSpace(line), "cursor:") {
			continue
		}
		if !bench.ParseStamp(fields[0]).IsZero() {
			rows = append(rows, changeRow{line: line})
			continue
		}
		if len(rows) == 0 {
			continue
		}
		rows[len(rows)-1].line += " " + strings.TrimSpace(line)
	}
	for i := range rows {
		fields := strings.Fields(rows[i].line)
		if len(fields) < 2 {
			t.Fatalf("a checkpoint row carries no card column: %q", rows[i].line)
		}
		rows[i].subject = fields[1]
	}
	return rows
}

// rowCarrying picks the one row whose detail column carries a marker, and
// returns the rest, so each marker is matched once and the leftover row is the
// one no marker names.
func rowCarrying(rows []changeRow, marker string) (*changeRow, []changeRow) {
	for i, row := range rows {
		if !strings.Contains(row.line, marker) {
			continue
		}
		rest := append(append([]changeRow{}, rows[:i]...), rows[i+1:]...)
		return &rows[i], rest
	}
	return nil, rows
}

// TestTheCheckpointPrintsACursorEvenWhenNothingMoved asserts the answer a
// person gets on the call that reports nothing, which is the common one: the
// table is empty and the cursor is still there to carry forward.
func TestTheCheckpointPrintsACursorEvenWhenNothingMoved(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	cursor := mintedCursor(t, root)

	got := runCLI(t, root, "changes", "--since", cursor)
	if got.code != 0 {
		t.Fatalf("changes: %d %s", got.code, got.errw)
	}
	if !strings.Contains(got.out, "cursor: "+cursor) {
		t.Errorf("an unchanged workbench did not print the cursor back unchanged:\n%s", got.out)
	}
}

// TestTheCheckpointRefusesACursorItDidNotIssue asserts the refusal reaches the
// terminal under its own name, which is what a script reads with cut.
func TestTheCheckpointRefusesACursorItDidNotIssue(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "changes", "--since", "not-a-cursor")
	if got.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("wanted the refused exit code, got %d\n%s", got.code, got.out)
	}
	if !strings.HasPrefix(got.errw, contract.Malformed+" ") {
		t.Errorf("wanted %s to lead stderr, got %q", contract.Malformed, got.errw)
	}
}

// mintedCursor takes a cursor off the machine form, which is where a caller
// reads one when a script is holding it.
func mintedCursor(t *testing.T, root string) string {
	t.Helper()
	got := runCLI(t, root, "--json", "changes")
	if got.code != 0 {
		t.Fatalf("mint a cursor: %d %s", got.code, got.errw)
	}
	var minted struct {
		Cursor string `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(got.out), &minted); err != nil {
		t.Fatalf("read the minted cursor: %v\n%s", err, got.out)
	}
	if minted.Cursor == "" {
		t.Fatal("the machine form carried no cursor")
	}
	return minted.Cursor
}

// writeAnchorlessCard builds a card directory carrying a history and no
// anchor, which is what a crash between the directory and the anchor leaves
// and what dinah check reports. The walk keys on the directory, so the line
// is delivered and no reference can be composed for it.
func writeAnchorlessCard(t *testing.T, container string) string {
	t.Helper()
	id := "0f8d904a8b4c"
	dir := filepath.Join(soleBenchDir(t, container), bench.CardsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	event := bench.Event{TS: bench.Stamp(bench.ParseStamp("2099-01-01T00:00:00Z")), Event: contract.EventCreated, Actor: "alka", Title: "A card with no anchor"}
	if err := bench.AppendEvent(filepath.Join(dir, bench.JournalName), event); err != nil {
		t.Fatalf("write the history: %v", err)
	}
	return id
}
