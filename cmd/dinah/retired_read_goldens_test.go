package main

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"dinah/internal/verb"
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
	// superseded names the card that changed what this invocation prints,
	// empty on a row still held to the byte-for-byte promise. See
	// TestTheCollapsedReadsPrintWhatTheRetiredOnesPrinted for what a
	// superseded row is held to instead.
	superseded string
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
		{file: "dinah-contents.txt", retired: []string{"contents", card}, after: []string{"list", card, "--depth", "entities"}, superseded: "dinah-536"},
		{file: "dinah-contents-workbench.txt", retired: []string{"contents", "workbench"}, after: []string{"list", "workbench", "--depth", "entities"}, superseded: "dinah-536"},
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

// assertSupersededRead holds a row whose bytes a later card changed on
// purpose to the part of the promise that survived.
//
// dinah-536 gave a card's checklist three published branches, and the operator
// ruled on dinah-536/questions/1 that the CLI is what publishes them. A card's
// rows therefore carry a branch row above its items, its items are indented
// below that row, and the reference column is wider for every row in the
// table because one reference in it grew. No filtering recovers the captured
// bytes from that, because the table is aligned as a whole.
//
// Regenerating the golden is what this file refuses, and rightly: the retired
// command is gone, so a regenerated golden would be the surviving command
// compared against itself. Rewriting it by hand is the same thing more slowly.
// So the captured bytes stay as they are, as the record of what the retired
// command printed, and what is asserted here is the part dinah-536 did not
// claim to change: every reference the retired command printed is still
// printed, in the order it printed them, and everything this invocation adds
// is a collection row. A row that disappeared, moved, or was replaced by
// something that is not a branch still reddens this test.
func assertSupersededRead(t *testing.T, row retiredRead, printed, captured string) {
	t.Helper()
	before := referenceColumn(captured, false)
	after := referenceColumn(printed, false)
	if len(before) == 0 {
		t.Fatalf("%s carries no reference rows, so %v is compared against nothing", row.file, row.after)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("%v prints the references\n  %s\nwhere %s printed\n  %s\n(superseded by %s, which may add collection rows and nothing else)",
			row.after, strings.Join(after, " | "), row.file, strings.Join(before, " | "), row.superseded)
	}
	added := len(referenceColumn(printed, true)) - len(after)
	if added == 0 {
		t.Errorf("%v draws no collection row, so %s is recorded as superseded by %s and nothing superseded it",
			row.after, row.file, row.superseded)
	}
}

// referenceColumn is the reference of every table row of a printed read, in
// the order the table drew them. Branch rows are kept only when asked for, so
// one call answers what the retired command printed and the other answers what
// this one prints on top of it.
//
// A row is recognised by its tree glyphs, which every row of these tables
// carries and no header or heading does, and the reference is the first word
// after them. The indentation a branch puts on its members is dropped with the
// glyphs, which is what lets a moved row compare equal to where it started.
func referenceColumn(out string, branches bool) []string {
	var refs []string
	for _, line := range strings.Split(out, "\n") {
		// The glyphs end at the first "-- ", and the reference begins after
		// it. Scanning for the last glyph character instead would land inside
		// the reference, which carries hyphens of its own.
		cut := strings.Index(line, "-- ")
		if cut < 0 {
			continue
		}
		// The table's header underline is a run of dashes and spaces and
		// carries "-- " like any row, so it reaches this line. It is not a
		// row, and it compared equal between the two sides only because the
		// words under it happened to be the same length. A row's reference
		// carries something that is not a dash; the underline does not.
		if strings.Trim(line, "- ") == "" {
			continue
		}
		fields := strings.Fields(line[cut+len("-- "):])
		if len(fields) < 1 {
			continue
		}
		reference, kind := fields[0], ""
		if len(fields) > 1 {
			kind = fields[1]
		}
		if kind == verb.KindCollection {
			if branches {
				refs = append(refs, reference)
			}
			continue
		}
		refs = append(refs, reference)
	}
	return refs
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
		printed, captured := stabilise(got.out, root), foldPathSeparators(string(want))
		if row.superseded != "" {
			assertSupersededRead(t, row, printed, captured)
			continue
		}
		if printed != captured {
			t.Errorf("%v prints\n%s\nwhere %s holds\n%s", row.after, printed, row.file, captured)
		}
	}
}
