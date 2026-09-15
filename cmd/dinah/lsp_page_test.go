package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// desiredLSPPage is the page dinah-515 section 5.2 drafts, with the one
// correction that card's decision 27 settled: there is no refusal named
// dinah.malformed. contract.Malformed is spelled malformed with no layer
// prefix, its catalogue text fits a flag value, and this head raises it
// exactly as the rest of the tree already raises it over an argument that
// will not parse. The draft's refusal column is corrected here rather than a
// name being minted for one head.
const desiredLSPPage = `lsp [--root <dir>] [--annotate-prose] [--poll-seconds <n>]

Serve one workbench to an editor over LSP on stdio

What you may write:
  As you write it       What it is
  --------------------  --------------------------------------------------------
  [--root <dir>]        directory the workbench this server serves lies under;
                        the workbench discovered at startup when you name none
  [--annotate-prose]    draw the inline annotation in prose as well as in front
                        matter, for an editor that sends no settings
  [--poll-seconds <n>]  how often the server rereads the workbench for a change;
                        2 when you name none

What can go wrong, in the order each is checked:
  Order  What can go wrong                          Refusal
  -----  -----------------------------------------  ------------------
  1      --root names a directory that exists       dinah.unknown-root
  2      --poll-seconds is a positive whole number  malformed

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
`

// TestTheLSPPageMatchesTheDraft asserts dinah-515 criterion 12: the rendered
// help page for lsp is the text the specification drafts, word for word.
func TestTheLSPPageMatchesTheDraft(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "help", "lsp")
	if got.code != 0 {
		t.Fatalf("exit code: wanted 0, got %d: %s", got.code, got.errw)
	}
	if got.out != desiredLSPPage {
		t.Errorf("the rendered page differs from the draft:\n%s", diffLines(desiredLSPPage, got.out))
	}
}

// TestLSPRefusesABadPollInterval asserts the second row of that page against
// the running command: a --poll-seconds that is not a positive whole number
// refuses before any framing is written, and a value that is one is accepted.
//
// The accepting case beside the refusing one is what stops this passing
// against a build that refuses every value. The accepted invocation is given
// a closed stream, so the server starts, reads end of file and stops.
func TestLSPRefusesABadPollInterval(t *testing.T) {
	root := newBench(t)
	for _, value := range []string{"0", "-1", "two", "1.5"} {
		got := runCLI(t, root, "lsp", "--poll-seconds", value)
		if got.code != 2 {
			t.Errorf("--poll-seconds %s exited %d, wanted 2", value, got.code)
		}
		if !strings.HasPrefix(got.errw, "malformed") {
			t.Errorf("--poll-seconds %s refused %q, wanted a line opening with malformed", value, got.errw)
		}
	}
	if got := runCLI(t, root, "lsp", "--poll-seconds", "3"); got.code != 0 {
		t.Errorf("--poll-seconds 3 exited %d with %q, and three is a positive whole number", got.code, got.errw)
	}
}

// TestLSPRefusesARootThatIsNotThere asserts the first row of that page: a
// --root naming a directory that does not exist refuses by name, and one
// naming a directory that does is accepted.
func TestLSPRefusesARootThatIsNotThere(t *testing.T) {
	root := newBench(t)
	missing := filepath.Join(t.TempDir(), "nowhere")
	got := runCLI(t, root, "lsp", "--root", missing)
	if got.code != 2 {
		t.Errorf("a --root naming nothing exited %d, wanted 2", got.code)
	}
	if !strings.Contains(got.errw, "dinah.unknown-root") {
		t.Errorf("a --root naming nothing refused %q, wanted dinah.unknown-root", got.errw)
	}
	here := t.TempDir()
	if err := os.MkdirAll(here, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, root, "lsp", "--root", here); got.code != 0 {
		t.Errorf("a --root naming a directory that is there exited %d with %q", got.code, got.errw)
	}
}

// TestChangesCarriesTheColumnsMember asserts the caller half of dinah-515
// criterion 21 for the command-line head: dinah changes --json carries the
// new columns member when a column anchor moved and carries none when none
// did.
func TestChangesCarriesTheColumnsMember(t *testing.T) {
	root := newBench(t)
	minted := runCLI(t, root, "--json", "changes")
	if minted.code != 0 {
		t.Fatalf("mint: %d %s", minted.code, minted.errw)
	}
	var first struct {
		Cursor  string `json:"cursor"`
		Columns []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			HoldsOnEntry bool   `json:"holds_on_entry"`
			HoldsOnExit  bool   `json:"holds_on_exit"`
		} `json:"columns"`
	}
	if err := json.Unmarshal([]byte(minted.out), &first); err != nil {
		t.Fatalf("decode the minting answer: %v", err)
	}
	if len(first.Columns) != 0 {
		t.Errorf("a minting call reported %d columns, and it reports nothing", len(first.Columns))
	}

	// Nothing has moved, so the next call reports no column either.
	quiet := runCLI(t, root, "--json", "changes", "--since", first.Cursor)
	if quiet.code != 0 {
		t.Fatalf("quiet: %d %s", quiet.code, quiet.errw)
	}
	var second struct {
		Changed bool `json:"changed"`
		Columns []struct {
			ID string `json:"id"`
		} `json:"columns"`
	}
	if err := json.Unmarshal([]byte(quiet.out), &second); err != nil {
		t.Fatalf("decode the quiet answer: %v", err)
	}
	if second.Changed {
		t.Error("a workbench nobody touched reported a change")
	}
	if len(second.Columns) != 0 {
		t.Errorf("a workbench nobody touched reported %d columns", len(second.Columns))
	}

	// A column retitled by hand is what the widened checkpoint exists to see.
	columns := runCLI(t, root, "--json", "columns")
	if columns.code != 0 {
		t.Fatalf("columns: %d %s", columns.code, columns.errw)
	}
	var flow []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(columns.out), &flow); err != nil {
		t.Fatalf("decode the flow: %v", err)
	}
	if len(flow) == 0 {
		t.Fatal("the fixture workbench declares no column")
	}
	anchor := filepath.Join(soleBenchDir(t, root), "columns", flow[0].ID, "column.md")
	raw, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the column anchor: %v", err)
	}
	edited := strings.Replace(string(raw), "title: "+flow[0].Title, "title: "+flow[0].Title+"RENAMED", 1)
	if edited == string(raw) {
		t.Fatalf("the column anchor carries no title line to edit:\n%s", raw)
	}
	if err := os.WriteFile(anchor, []byte(edited), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	moved := runCLI(t, root, "--json", "changes", "--since", first.Cursor)
	if moved.code != 0 {
		t.Fatalf("moved: %d %s", moved.code, moved.errw)
	}
	var third struct {
		Changed bool `json:"changed"`
		Cards   []struct {
			ID string `json:"id"`
		} `json:"cards"`
		Columns []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			HoldsOnEntry bool   `json:"holds_on_entry"`
			HoldsOnExit  bool   `json:"holds_on_exit"`
		} `json:"columns"`
	}
	if err := json.Unmarshal([]byte(moved.out), &third); err != nil {
		t.Fatalf("decode the moved answer: %v", err)
	}
	if !third.Changed {
		t.Fatal("a hand-edited column anchor reported no change")
	}
	if len(third.Cards) != 0 {
		t.Errorf("a column edit resynced %d cards, and it must resync none", len(third.Cards))
	}
	named := false
	for _, reported := range third.Columns {
		if reported.ID == flow[0].ID && reported.Title == flow[0].Title+"RENAMED" {
			named = true
		}
	}
	if !named {
		t.Errorf("the answer names no column carrying the edited title: %+v", third.Columns)
	}
}
