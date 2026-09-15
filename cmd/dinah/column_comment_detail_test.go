package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// TestChangesNamesTheColumnAColumnCommentWasLeftOn asserts
// dinah-518/criteria/14.
//
// Three lines are read in one run. A column comment written by the verb draws
// the column's title in the detail column. A hand-written line carrying a
// column and no title falls back to the identifier, which is the rule
// Event.ColumnTitle's own doc comment already states for the tier events. A
// card comment draws an empty detail, unchanged from trunk, which is what
// keeps every commented line already written rendering as it did.
//
// The assertion is on the title's exact bytes. Each row's detail column is
// read as a field and compared whole, so a build whose arm composes a
// sentence around the title fails: the composed sentence is not the title,
// however much of the title it contains.
//
// The column is renamed to a title carrying a space before anything is
// commented on, and the fixture's own titles are all one word, which is why
// the rename is here rather than left to the flow a fresh bench carries.
// Against a one-word title a whole-field read and a read of the first token
// answer alike, so an arm drawing only the first word of the title passed this
// check while truncating every column on a real workbench whose title carries
// a space, which is most of them.
func TestChangesNamesTheColumnAColumnCommentWasLeftOn(t *testing.T) {
	root := newBench(t)
	dir := benchDir(t, root)

	if got := runCLI(t, root, "set", "intake", "title", "Intake Queue"); got.code != 0 {
		t.Fatalf("rename the column: %d %s", got.code, got.errw)
	}

	card := addCard(t, root, "a card")
	cursor := mintedCursor(t, root)
	if got := runCLI(t, root, "comment", card, "a card remark"); got.code != 0 {
		t.Fatalf("comment on the card: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "comment", "intake", "a station note"); got.code != 0 {
		t.Fatalf("comment on the column: %d %s", got.code, got.errw)
	}

	opened, err := bench.Open(dir)
	if err != nil {
		t.Fatalf("open %s: %v", dir, err)
	}
	column := opened.Columns[0]
	if column.Title == "" {
		t.Fatal("the fixture's first column carries no title, so the title half would assert nothing")
	}
	// A title of one word cannot tell a whole-field read from a read of its
	// first token, so the check would admit a build that truncates.
	if !strings.Contains(column.Title, " ") {
		t.Fatalf("the fixture's column title %q carries no space, so a build drawing only its first word would pass this check", column.Title)
	}
	// A detail is read back off a padded row, so a title carrying a run of
	// two spaces could not be told from the padding, and the comparison
	// below would be against a value the renderer never drew.
	if strings.Join(strings.Fields(column.Title), " ") != column.Title {
		t.Fatalf("the fixture's column title %q does not survive being read back off a drawn row", column.Title)
	}

	// A line carrying a column and no title, which no verb writes and which a
	// journal from before this card could carry. Its timestamp is the one the
	// verb just wrote, because a line stamped before the cursor is delivered
	// to nobody and this run would then assert nothing about the fallback.
	written, _, err := bench.ReadJournal(filepath.Join(dir, bench.JournalName))
	if err != nil {
		t.Fatalf("read the workbench journal: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("the workbench journal carries no line, so the column comment reached it nowhere")
	}
	plant := bench.Event{
		TS:      written[len(written)-1].TS,
		Event:   "commented",
		Actor:   bench.Actor{Name: "alka"},
		Comment: "e00000000099",
		Column:  column.ID,
	}
	encoded, err := json.Marshal(plant)
	if err != nil {
		t.Fatalf("encode the planted line: %v", err)
	}
	journal := filepath.Join(dir, bench.JournalName)
	existing, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the workbench journal: %v", err)
	}
	if err := os.WriteFile(journal, append(existing, append(encoded, '\n')...), 0o644); err != nil {
		t.Fatalf("write the workbench journal: %v", err)
	}

	changes := runCLI(t, root, "changes", "--since", cursor)
	if changes.code != 0 {
		t.Fatalf("changes: %d %s", changes.code, changes.errw)
	}
	// Each detail is read off its own row's detail column and compared
	// whole, which is what the criterion says and what the old containment
	// test did not do. A row whose detail composed a sentence around the
	// title still contained the title, so the assertion passed against
	// exactly the build it was written to catch.
	titled, bare, empty := 0, 0, 0
	for _, row := range changeRows(t, changes.out) {
		if !strings.Contains(row.line, "commented") {
			continue
		}
		switch row.detail {
		case column.Title:
			titled++
		case column.ID:
			bare++
		case "":
			empty++
		default:
			t.Errorf("a commented row's detail column reads %q, which is neither the column's title, nor its identifier, nor empty:\n%s", row.detail, changes.out)
		}
	}
	if titled != 1 {
		t.Errorf("%d commented rows draw the column's title and nothing else, wanted 1:\n%s", titled, changes.out)
	}
	if bare != 1 {
		t.Errorf("%d commented rows fall back to the column's identifier and nothing else, wanted 1:\n%s", bare, changes.out)
	}
	if empty != 1 {
		t.Errorf("%d commented rows draw an empty detail, wanted 1 for the card comment:\n%s", empty, changes.out)
	}

	// The card comment's own line is in the card's journal, and it renders
	// with an empty detail exactly as it does on trunk.
	log := runCLI(t, root, "log", card)
	if log.code != 0 {
		t.Fatalf("log: %d %s", log.code, log.errw)
	}
	for _, line := range strings.Split(log.out, "\n") {
		if !strings.Contains(line, "commented") {
			continue
		}
		if strings.Contains(line, column.Title) || strings.Contains(line, column.ID) {
			t.Errorf("a card journal's commented row names a column: %q", line)
		}
	}
}
