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
	for _, wanted := range []string{
		"fx-1",
		"the vendor has not answered",
		bench.WorkbenchKey,
		orphan,
	} {
		if !strings.Contains(got.out, wanted) {
			t.Errorf("the checkpoint does not draw %q:\n%s", wanted, got.out)
		}
	}
	if !strings.Contains(got.out, "cursor: ") {
		t.Errorf("the checkpoint prints no cursor, so the reader has nothing to ask with next:\n%s", got.out)
	}
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
