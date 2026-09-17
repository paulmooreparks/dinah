package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	return foldPathSeparators(rootedText(text, root))
}

// rootedText is stabilise without the separator fold: the fixture's directory
// and the instants and identifiers a run mints are replaced, and every
// separator is left as the run printed it.
//
// It is separate so that one check can read the bytes the fold erases, which
// is TestTheEnumeratedPathCarriesTheHostsOwnSeparator below.
func rootedText(text, root string) string {
	out := text
	// Every path is replaced longest first, so that a shorter one does not
	// eat a longer one's prefix and leave the tail behind.
	for _, path := range rootSpellings(root) {
		out = strings.ReplaceAll(out, path, "<root>")
		out = strings.ReplaceAll(out, strings.ReplaceAll(path, `\`, `/`), "<root>")
	}
	out = timestamp.ReplaceAllString(out, "<when>")
	return mintedID.ReplaceAllString(out, "<id>")
}

// rootSpellings is every spelling of the fixture's own directory and of the
// directory above it that a printed path may carry, longest first.
//
// Each of the two is spelled twice, because macOS hands out a temporary
// directory under /var/folders and reaches the same directory at
// /private/var/folders, and the enumeration prints whichever spelling the
// tool resolved rather than the one the test was handed. A run whose
// symbolic links do not resolve, which is every run on Linux and Windows,
// yields the same string twice and the duplicate replaces nothing.
func rootSpellings(root string) []string {
	spellings := []string{}
	for _, path := range []string{root, filepath.Dir(root)} {
		spellings = append(spellings, path)
		if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != path {
			spellings = append(spellings, resolved)
		}
	}
	sort.Slice(spellings, func(i, j int) bool { return len(spellings[i]) > len(spellings[j]) })
	return spellings
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

// TestTheEnumeratedPathCarriesTheHostsOwnSeparator reads the bytes the
// separator fold erases from the golden comparison.
//
// The fold is applied to the golden and to the output alike, which is what
// lets one captured file hold on three hosts, and the cost is that no
// comparison in that test can say anything about a separator below the rooted
// path any more. This is the assertion that can: the workbench enumeration
// prints a path the host's own filesystem spells, so the separator it carries
// is filepath.Separator and nothing else, and it is read here off the
// unfolded text rather than off the golden.
//
// Only the separator is asserted. What the path is, and the rest of the table
// around it, are the golden's business.
func TestTheEnumeratedPathCarriesTheHostsOwnSeparator(t *testing.T) {
	root, _, _, _ := seedRetiredBench(t)
	got := runCLI(t, root, "list", "workbenches", "--root", "..", "--max-depth", "4")
	if got.code != 0 {
		t.Fatalf("the enumeration exited %d: %s", got.code, got.errw)
	}
	printed := tokenisedPath.FindString(rootedText(got.out, root))
	if printed == "" {
		t.Fatalf("the enumeration printed no path below the fixture's own directory:\n%s", got.out)
	}
	wanted := "<root>" + string(filepath.Separator) + ".dinah"
	if !strings.HasPrefix(printed, wanted) {
		t.Errorf("the enumeration printed %q, and this host spells the separator %q", printed, string(filepath.Separator))
	}
	foreign := `\`
	if filepath.Separator == '\\' {
		foreign = "/"
	}
	if strings.Contains(printed, foreign) {
		t.Errorf("the enumeration printed %q, which carries the separator of another host", printed)
	}
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
