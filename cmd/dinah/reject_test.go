package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/msg"
)

// declareRejectTo writes reject_to into one column's own anchor, found by the
// title the init flow gives it. Nothing in the tool sets the field, so a test
// reaching the rendered surface has to write the declaration the way a person
// editing a column.md would.
func declareRejectTo(t *testing.T, root, title, ref string) {
	t.Helper()
	columns := filepath.Join(soleBenchDir(t, root), bench.ColumnsDir)
	for _, id := range bench.ListIDs(columns) {
		path := filepath.Join(columns, id, bench.ColumnAnchor)
		text, err := bench.ReadText(path)
		if err != nil {
			t.Fatalf("read a column: %v", err)
		}
		fm, body := bench.ParseAnchor(text)
		if fm.Value("title") != title {
			continue
		}
		fm.Set("reject_to", ref)
		if err := os.WriteFile(path, []byte(fm.Render(body)), 0o644); err != nil {
			t.Fatalf("write a column: %v", err)
		}
		return
	}
	t.Fatalf("the fixture flow carries no column titled %s", title)
}

// movesRows returns the rows of the legal-moves table an invocation printed,
// which is every line after the heading and its rule up to the first blank one.
func movesRows(t *testing.T, out string) []string {
	t.Helper()
	lines := strings.Split(out, "\n")
	opening := msg.For("en").T("instructions.moves")
	for i, line := range lines {
		if strings.TrimSpace(line) != strings.TrimSpace(opening) {
			continue
		}
		var rows []string
		// The heading line and the rule under it are the two lines the
		// block opens with, and the rows run from there to the first blank.
		for _, row := range lines[i+3:] {
			if strings.TrimSpace(row) == "" {
				break
			}
			rows = append(rows, row)
		}
		return rows
	}
	t.Fatalf("the output carries no legal-moves block:\n%s", out)
	return nil
}

// TestBareMoveListsTheRejectColumn is dinah-207 AC-8, against the surface that
// actually draws the table. The criterion names `dinah move <card>` with no
// destination, and that form answers unknown-column with a bare list of column
// references rather than the legal-moves table; `dinah instructions <card>` is
// where renderInstructions draws it, and it is also what a successful move
// prints. See the card's own comment on the correction.
func TestBareMoveListsTheRejectColumn(t *testing.T) {
	root := newBench(t)
	declareRejectTo(t, root, "Doing", "intake")
	got := runCLI(t, root, "add", "Write the release notes")
	if got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")

	served := runCLI(t, root, "instructions", "fx-1")
	if served.code != 0 {
		t.Fatalf("instructions: %d %s", served.code, served.errw)
	}
	heading := msg.For("en").T("column.moves.reject")
	if !strings.Contains(served.out, heading) {
		t.Fatalf("the moves table carries no %s heading:\n%s", heading, served.out)
	}
	yes := msg.For("en").T("word.yes")
	no := msg.For("en").T("word.no")
	rows := movesRows(t, served.out)
	if len(rows) != 2 {
		t.Fatalf("wanted two rows, got %d:\n%s", len(rows), served.out)
	}
	for _, row := range rows {
		fields := strings.Fields(row)
		last := fields[len(fields)-1]
		switch fields[0] {
		case "intake":
			if last != yes {
				t.Errorf("the declared target's row reads %q, wanted %q:\n%s", last, yes, row)
			}
		case "done":
			if last != no {
				t.Errorf("an undeclared row reads %q, wanted %q:\n%s", last, no, row)
			}
		default:
			t.Errorf("an unexpected row: %s", row)
		}
	}
}

// TestLogMarksARejectMove is dinah-207 AC-9. The mark is what makes a rejection
// legible in a card's own history, which is the half a person reads rather than
// counts.
func TestLogMarksARejectMove(t *testing.T) {
	root := newBench(t)
	declareRejectTo(t, root, "Doing", "intake")
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")
	if got := runCLI(t, root, "move", "fx-1", "intake"); got.code != 0 {
		t.Fatalf("the rejecting move: %d %s", got.code, got.errw)
	}

	marker := msg.For("en").T("log.reject")
	logged := runCLI(t, root, "log", "fx-1")
	if logged.code != 0 {
		t.Fatalf("log: %d %s", logged.code, logged.errw)
	}
	marked := 0
	for _, line := range strings.Split(logged.out, "\n") {
		if strings.Contains(line, marker) {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("wanted the mark %s on exactly one line, got %d:\n%s", marker, marked, logged.out)
	}

	// The move into the doing station is a move out of a column declaring
	// nothing, so an implementation marking every move would put the mark on
	// two lines and the count above is what tells the two apart.
	if got := runCLI(t, root, "move", "fx-1", "done"); got.code != 0 {
		t.Fatalf("the ordinary move: %d %s", got.code, got.errw)
	}
	again := runCLI(t, root, "log", "fx-1")
	marked = 0
	for _, line := range strings.Split(again.out, "\n") {
		if strings.Contains(line, marker) {
			marked++
		}
	}
	if marked != 1 {
		t.Errorf("an ordinary move took the mark too, %d lines carry it:\n%s", marked, again.out)
	}
}
