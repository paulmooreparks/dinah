package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The goldens of the seven retired read commands.
//
// Six acceptance criteria of dinah-523 require the new invocation to print
// byte for byte what a retired one printed. After the retirement the retired
// command does not exist, so nothing is left to compare against unless the
// bytes were captured while both spellings still answered. They were, at the
// step of that card's implementation order where the new command had landed
// and nothing had yet been taken away.
//
// -update-read-goldens is how they were captured, and the flag follows the
// convention -update-quick-start already sets in this package: it rewrites
// every golden and then fails, so a green run can never regenerate one and a
// regenerated golden always costs a deliberate second run. It now refuses,
// naming the retired command it can no longer run, because regenerating a
// golden from the surviving command is the one way this comparison could be
// made vacuous.
var updateReadGoldens = flag.Bool("update-read-goldens", false, "rewrite the retired-read goldens from the retired spellings, then fail")

// goldenDir is where the captured bytes live.
const goldenDir = "testdata/retired-read-goldens"

// retiredRead is one captured pair: the invocation that was retired, the
// invocation that answers afterwards, and the file holding what the first one
// printed.
type retiredRead struct {
	// file is the golden's basename, which is the retired invocation with
	// its spaces replaced by dashes.
	file string
	// retired is the argv of the command that no longer exists. It is kept
	// so that the capture flag can name what it would have to run.
	retired []string
	// after is the argv that answers the same question now.
	after []string
}

// retiredReads is the whole set, one row per golden. seedRetiredBench builds
// the workbench all of them are run against, and both the capture and the
// comparison call it, so the two read one workbench rather than two.
//
// A bare `dinah ls` has no row, and its absence is the one deliberate gap. Its
// replacement prints a different table on purpose, under dinah-523's decision
// 3: the reader gains the column each card stands in and loses its severity
// and its priority, so a byte comparison there would be asserting that a
// decided change did not happen. What holds that row instead is
// TestTheCardsRosterWordAndABareQueryPrintOneTable below, which pins the
// replacement against the table it was moved onto.
func retiredReads(card, column, ref string) []retiredRead {
	return []retiredRead{
		{file: "dinah-ls-column.txt", retired: []string{"ls", column}, after: []string{"list", column}},
		{file: "dinah-ls-column-ready.txt", retired: []string{"ls", column, "--ready"}, after: []string{"list", column, "--ready"}},
		{file: "dinah-columns.txt", retired: []string{"columns"}, after: []string{"list", "columns"}},
		{file: "dinah-workstream.txt", retired: []string{"workstream"}, after: []string{"list", "workstreams"}},
		{file: "dinah-attachments.txt", retired: []string{"attachments", card}, after: []string{"list", card + "/attachments"}},
		{file: "dinah-attachments-bare.txt", retired: []string{"attachments"}, after: []string{"list", "attachments"}},
		{file: "dinah-log.txt", retired: []string{"log", card}, after: []string{"list", card + "/journal"}},
		{file: "dinah-contents.txt", retired: []string{"contents", card}, after: []string{"list", card, "--depth", "entities"}},
		{file: "dinah-contents-workbench.txt", retired: []string{"contents", "workbench"}, after: []string{"list", "workbench", "--depth", "entities"}},
		{file: "dinah-contents-column.txt", retired: []string{"contents", column}, after: []string{"list", column, "--depth", "entities"}},
		{file: "dinah-contents-below-card.txt", retired: []string{"contents", ref}, after: []string{"list", ref, "--depth", "entities"}},
		{file: "dinah-workbenches.txt", retired: []string{"workbenches", "..", "--max-depth", "4"}, after: []string{"list", "workbenches", "--root", "..", "--max-depth", "4"}},
	}
}

// seedRetiredBench builds the workbench every golden is read against and
// answers the card, the column and the below-card reference the rows name.
//
// Everything it files is fixed text, so the only parts of an answer that
// differ between two runs are the ones stabilise replaces.
func seedRetiredBench(t *testing.T) (root, card, column, ref string) {
	t.Helper()
	root = newBench(t)
	if got := runCLI(t, root, "add", "A card the goldens are read against"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	card = "fx-1"
	column = "intake"
	if got := runCLI(t, root, "comment", card, "A comment the walk draws"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", card, "decision", "A decision the walk draws"); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	payload := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(payload, []byte("attached bytes\n"), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if got := runCLI(t, root, "attach", card, payload, "--description", "A file the listing draws"); got.code != 0 {
		t.Fatalf("attach: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "workstream", "new", "A workstream the listing draws", "--slug", "goldens"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "join", card, "workstream/goldens"); got.code != 0 {
		t.Fatalf("join: %d %s", got.code, got.errw)
	}
	return root, card, column, card + "/decisions/1"
}

// timestamp is an RFC 3339 instant as this tool stamps one.
var timestamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)

// mintedID is an identifier as this tool mints one, twelve characters for an
// entity and thirty-two for a workbench directory, each of which is
// minted afresh in every run and appears inside the path a workbench
// enumeration prints.
var mintedID = regexp.MustCompile(`[0-9a-f]{12,32}`)

// stabilise replaces what one run cannot share with another.
//
// Two things differ between the run that captured a golden and the run that
// compares against it, and neither is anything the comparison is about. A
// journal stamps the instant each act was recorded, and a fixture workbench
// lives under a temporary directory whose name is minted per run. Both are
// replaced by a fixed token, and the column widths a table computes are
// preserved, because the token is written to the width of what it replaces
// wherever a table has already laid the line out.
func stabilise(text, root string) string {
	out := text
	// The workbench directory is replaced before the directory above it, so
	// that the longer path is matched first and the shorter one does not eat
	// its prefix.
	for _, path := range []string{root, filepath.Dir(root)} {
		out = strings.ReplaceAll(out, path, "<root>")
		out = strings.ReplaceAll(out, strings.ReplaceAll(path, `\`, `/`), "<root>")
	}
	out = timestamp.ReplaceAllString(out, "<when>")
	return foldPathSeparators(mintedID.ReplaceAllString(out, "<id>"))
}

// tokenisedPath is a path the stabiliser has already rooted: the token, then
// whatever the printed path carried below it, up to the first space.
var tokenisedPath = regexp.MustCompile(`<root>\S*`)

// foldPathSeparators writes every separator below the rooted path as a forward
// slash, on both sides of the comparison.
//
// The goldens were captured on Windows, where a workbench enumeration prints
// `<root>\.dinah\<id>`, and the same enumeration on Linux and macOS prints the
// same path with forward slashes. That difference is the host's and not the
// command's, so folding it is what lets one captured golden hold the bytes on
// every platform the checks run. Only the rooted path is touched, because the
// separator is the only thing being folded and a backslash anywhere else in a
// listing means something else.
func foldPathSeparators(text string) string {
	return tokenisedPath.ReplaceAllStringFunc(text, func(path string) string {
		return strings.ReplaceAll(path, `\`, `/`)
	})
}

// TestTheCollapsedReadsPrintWhatTheRetiredOnesPrinted asserts that each
// surviving invocation prints the bytes its retired spelling printed, read off
// the goldens captured while both still answered.
//
// The row count is asserted against the number of goldens on disk, so a golden
// somebody deleted reports rather than quietly leaving one less thing checked.
func TestTheCollapsedReadsPrintWhatTheRetiredOnesPrinted(t *testing.T) {
	if *updateReadGoldens {
		t.Fatal("the retired spellings no longer exist, so no golden can be regenerated from one; the captured bytes in testdata/retired-read-goldens are the record")
	}
	root, card, column, ref := seedRetiredBench(t)
	rows := retiredReads(card, column, ref)
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("read %s: %v", goldenDir, err)
	}
	if len(entries) != len(rows) {
		t.Fatalf("%s holds %d goldens against %d rows, so one of the two moved without the other", goldenDir, len(entries), len(rows))
	}
	for _, row := range rows {
		want, err := os.ReadFile(filepath.Join(goldenDir, row.file))
		if err != nil {
			t.Fatalf("read %s: %v", row.file, err)
		}
		got := runCLI(t, root, row.after...)
		if got.code != 0 {
			t.Fatalf("%v: exit %d, %s", row.after, got.code, got.errw)
		}
		// The golden goes through the separator fold as well, because it was
		// captured on Windows and holds that host's separators below the
		// rooted path.
		if stabilise(got.out, root) != foldPathSeparators(string(want)) {
			t.Errorf("%v prints\n%s\nwhere %s holds\n%s", row.after, stabilise(got.out, root), row.file, foldPathSeparators(string(want)))
		}
	}
}
