package profile

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// statementRef matches a statement identifier named in a test's source.
var statementRef = regexp.MustCompile(`\b(CORE-[A-Z]+-\d+)\b`)

// testFunction matches the declaration of a test, and testDoc the first line
// of the doc comment above one. A statement named in a test's doc comment is
// that test's, so both forms open a test's span.
var testFunction = regexp.MustCompile(`^func (Test\w+)\(`)
var testDoc = regexp.MustCompile(`^// (Test\w+) `)

// outOfReach lists the statements a single-seat local tool does not reach in
// v0, each with the reason it is out of reach. A statement absent from both
// this table and the tests fails the run, so nothing goes unaccounted for.
var outOfReach = map[string]string{
	"CORE-OUT-4":   "unreachable answers for a remote arbiter, and v0 serves one local workbench with nothing to be out of reach of",
	"CORE-STATE-6": "reordering the column list is an edit to workbench.md rather than an act the tool offers, so v0 exposes no verb that reorders a flow",
	"CORE-STATE-8": "a permission, satisfied by every move the fixture drives, with no observable behaviour of its own to assert",
	"CORE-CARD-2":  "retitling a card is an edit to card.md, and v0 offers no retitle verb",
	"CORE-LINK-1":  "links are card data and v0 ships no link write-sugar, so a card carrying one is read but never offered to the tool",
	"CORE-LINK-2":  "a malformed link is refused at a write surface v0 does not offer",
	"CORE-LINK-3":  "a link naming an absent card is refused at a write surface v0 does not offer; check reports the dangler instead",
	"CORE-LINK-4":  "no closed set is imposed because nothing in the tool reads a link at all",
	"CORE-LINK-5":  "no verb consults a link, so no refusal can follow from one",
	"CORE-LINK-6":  "nothing in the tool adds a link, so no link can be added as a consequence of another",
	"CORE-LAYER-1": "a workbench-declared extension kind is a definition surface v0 neither writes nor validates",
	"CORE-LAYER-2": "the same surface, read from the other side: an extension's content is preserved by the frontmatter rule rather than by a layer mechanism v0 implements",
	"CORE-LAYER-3": "the same surface again: v0 validates no layer declaration, so it can refuse none",
	"CORE-STATE-4": "a permission the fixture exercises by declaring an operator-owned column",
	"CORE-STATE-5": "a permission the fixture exercises by declaring a capacity limit",
	"CORE-CARD-8":  "a permission the interchange round trip exercises by carrying an undefined field",
	"CORE-JSON-8":  "a permission for a tool holding definitions in some other form; this one holds them in the shape the interchange form describes",
	"CORE-QUEUE-4": "a permission: v0 offers no order beside the fixed one, so there is nothing to keep available beside it",
	"CORE-TEXT-4":  "a permission the human rendering exercises by translating tokens where a person reads them",
	"CORE-BENCH-1": "a definition offered with no title is refused, and the offering surface is init --from, whose malformed refusal the interchange reader carries",
	"CORE-BENCH-2": "the same surface: an empty column list is refused by the interchange reader",
	"CORE-BENCH-3": "the same surface: a definition with no declared profile version is refused when the workbench is opened",
	"CORE-STATE-1": "a duplicate column identifier is refused when the workbench is opened, on a definition v0 writes rather than accepts from a caller",
	"CORE-STATE-2": "a column with no title is refused when the workbench is opened",
	"CORE-CARD-5":  "every card the tool reports carries one of the three states by construction, since the reader defaults an absent one to ready",
	"CORE-OWNER-1": "every act names its owner by construction, since the journal writer takes the actor from a request the ladder resolved",
	"CORE-TEXT-1":  "every text the tool writes is UTF-8 by construction, since Go strings are and nothing transcodes",
	"CORE-HIST-3":  "the journal is append-only by construction: nothing in the tool opens one for rewriting",
	"CORE-ITEM-1":  "a permission, and v0 stores no structured item on a card, so there is nothing for a card to be offered carrying",
	"CORE-ITEM-2":  "the tool reports no structured item, so it is never asked whether one is resolved",
	"CORE-ITEM-3":  "no verb consults a structured item, so no refusal can follow from one and no refusal name can be reported for one",
	"CORE-GATE-1":  "marking a column as requiring an item resolved is a definition surface v0 neither writes nor validates",
	"CORE-GATE-2":  "the same surface, read from the other side: v0 declares no such column, so no move can be refused for entering one",
}

// TestConformanceReport maps every CORE statement the v0 surface reaches to
// the test that drives it, and fails on a statement that neither a test names
// nor the out-of-reach table accounts for.
//
// The map is produced here rather than maintained by hand: the sources of
// every test in the tree are read, and a statement identifier named in a test
// is taken as that test driving it. SUITE-CONF-1 puts the CORE statements and
// no others in scope.
func TestConformanceReport(t *testing.T) {
	document := readProfile(t)
	extracted, err := Extract(strings.NewReader(document))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	driven := drivenStatements(t)

	var reached, unreached []string
	for _, statement := range extracted.Statements {
		if statement.Class != "CORE" {
			continue
		}
		if tests := driven[statement.ID]; len(tests) > 0 {
			sort.Strings(tests)
			reached = append(reached, statement.ID+"  "+strings.Join(tests, ", "))
			continue
		}
		if reason, ok := outOfReach[statement.ID]; ok {
			unreached = append(unreached, statement.ID+"  out of reach: "+reason)
			continue
		}
		t.Errorf("%s is driven by no test and is not listed as out of reach", statement.ID)
	}

	t.Logf("statements reached by a test: %d", len(reached))
	for _, row := range reached {
		t.Log("  " + row)
	}
	t.Logf("statements out of reach in v0: %d", len(unreached))
	for _, row := range unreached {
		t.Log("  " + row)
	}

	for _, fault := range outOfReachDefects(outOfReach, extracted.Statements, driven) {
		t.Errorf("%s", fault)
	}
}

// outOfReachDefects compares the out-of-reach table against the CORE
// statements a profile document currently publishes and the tests that
// currently drive them. It returns one message per fault, in sorted order: an
// entry naming an identifier the document no longer publishes as a CORE
// statement, and an entry whose statement a test now drives. A healthy table
// returns nil.
//
// The second fault is the stale exclusion the report has always caught. The
// first is the drift nothing caught before, because the report's main loop
// walks the statements the document publishes, so an entry left behind when
// its statement is retired is visited by nothing at all.
func outOfReachDefects(outOfReach map[string]string, statements []Statement, driven map[string][]string) []string {
	published := map[string]bool{}
	for _, statement := range statements {
		if statement.Class != "CORE" {
			continue
		}
		published[statement.ID] = true
	}

	var defects []string
	for _, id := range sortedCatalogKeys(outOfReach) {
		if !published[id] {
			defects = append(defects, id+" is listed as out of reach and names no current CORE statement")
			continue
		}
		tests := driven[id]
		if len(tests) == 0 {
			continue
		}
		sorted := append([]string(nil), tests...)
		sort.Strings(sorted)
		defects = append(defects, id+" is listed as out of reach and is driven by "+strings.Join(sorted, ", "))
	}
	return defects
}

// drivenStatements reads every test in the tree and returns the statements
// each one names, keyed by statement identifier.
func drivenStatements(t *testing.T) map[string][]string {
	t.Helper()
	driven := map[string][]string{}
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		// This file carries the out-of-reach table, whose own identifiers
		// would otherwise read as statements a test drives.
		if entry.Name() == "conformance_test.go" {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for id, tests := range statementsNamedIn(source) {
			for _, test := range tests {
				if !named(driven[id], test) {
					driven[id] = append(driven[id], test)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return driven
}

// statementsNamedIn returns the statement identifiers a single test source
// file's tests are taken to drive, keyed by identifier, each value the
// sorted, deduplicated list of test names. An identifier counts only when
// it appears either in the contiguous doc-comment block that begins with a
// "// TestX ..." line, or on a non-comment line within that test's span. An
// identifier named only in some other comment inside a test's body does not
// count, because a comment can narrate a statement without the test
// asserting anything about it.
func statementsNamedIn(source []byte) map[string][]string {
	drives := map[string][]string{}
	current := ""
	inDocHeader := false
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		isComment := strings.HasPrefix(trimmed, "//")
		if m := testFunction.FindStringSubmatch(line); m != nil {
			current = m[1]
			inDocHeader = false
		}
		if m := testDoc.FindStringSubmatch(line); m != nil {
			current = m[1]
			inDocHeader = true
		} else if !isComment && trimmed != "" {
			// A line of code closes the doc-comment header, so every comment
			// after it narrates the test's body rather than declaring its
			// purpose.
			inDocHeader = false
		}
		if current == "" {
			continue
		}
		if isComment && !inDocHeader {
			continue
		}
		for _, ref := range statementRef.FindAllStringSubmatch(line, -1) {
			id := ref[1]
			if !named(drives[id], current) {
				drives[id] = append(drives[id], current)
			}
		}
	}
	for id := range drives {
		sort.Strings(drives[id])
	}
	return drives
}

// named reports whether a test is already recorded against a statement.
func named(tests []string, want string) bool {
	for _, test := range tests {
		if test == want {
			return true
		}
	}
	return false
}

// TestOutOfReachDefectsCatchesAnEntryTheDocumentNoLongerPublishes asserts that
// a table entry naming an identifier the profile no longer publishes as a CORE
// statement is reported, which is the drift the report's main loop cannot see.
func TestOutOfReachDefectsCatchesAnEntryTheDocumentNoLongerPublishes(t *testing.T) {
	table := map[string]string{"CORE-GONE-1": "retired from the document while its entry stayed behind"}
	statements := []Statement{{ID: "CORE-LIVE-1", Class: "CORE"}}
	driven := map[string][]string{}

	defects := outOfReachDefects(table, statements, driven)
	if len(defects) != 1 {
		t.Fatalf("defects = %v, want exactly one", defects)
	}
	want := "CORE-GONE-1 is listed as out of reach and names no current CORE statement"
	if defects[0] != want {
		t.Errorf("defect = %q, want %q", defects[0], want)
	}
}

// TestOutOfReachDefectsCatchesAnEntryATestNowDrives asserts that a table entry
// whose statement the document still publishes and a test now drives is
// reported as a stale exclusion, naming the tests that drive it.
func TestOutOfReachDefectsCatchesAnEntryATestNowDrives(t *testing.T) {
	table := map[string]string{"CORE-LIVE-1": "no verb reaches this in v0"}
	statements := []Statement{{ID: "CORE-LIVE-1", Class: "CORE"}}
	driven := map[string][]string{"CORE-LIVE-1": {"TestSecond", "TestFirst"}}

	defects := outOfReachDefects(table, statements, driven)
	if len(defects) != 1 {
		t.Fatalf("defects = %v, want exactly one", defects)
	}
	want := "CORE-LIVE-1 is listed as out of reach and is driven by TestFirst, TestSecond"
	if defects[0] != want {
		t.Errorf("defect = %q, want %q", defects[0], want)
	}
}

// TestOutOfReachDefectsPassesAHealthyEntry asserts that an entry whose
// statement the document publishes and no test drives reports nothing, so the
// two failing cases above are not passing because every table faults.
func TestOutOfReachDefectsPassesAHealthyEntry(t *testing.T) {
	table := map[string]string{"CORE-LIVE-1": "no verb reaches this in v0"}
	statements := []Statement{{ID: "CORE-LIVE-1", Class: "CORE"}}
	driven := map[string][]string{"CORE-OTHER-1": {"TestElsewhere"}}

	defects := outOfReachDefects(table, statements, driven)
	if len(defects) != 0 {
		t.Errorf("defects = %v, want none", defects)
	}
}

// TestADocCommentHeaderNamesTheStatementItsTestDrives asserts that an
// identifier appearing only in the doc-comment block that begins with a
// "// TestX ..." line is taken as that test's, since the header is where a
// test declares its purpose.
func TestADocCommentHeaderNamesTheStatementItsTestDrives(t *testing.T) {
	source := []byte(`// TestHeaderOnly drives one statement.
//
// The identifier CORE-HEADER-1 is named here and nowhere else in the file.
func TestHeaderOnly(t *testing.T) {
    t.Log("the assertion itself names nothing")
}
`)

	drives := statementsNamedIn(source)
	if got := drives["CORE-HEADER-1"]; len(got) != 1 || got[0] != "TestHeaderOnly" {
		t.Errorf("CORE-HEADER-1 drives = %v, want [TestHeaderOnly]", got)
	}
}

// TestACodeLineNamesTheStatementItsTestDrives asserts that an identifier
// appearing only on a non-comment line inside a test's body is taken as that
// test's, since an executable line naming a statement is the test working on
// it.
func TestACodeLineNamesTheStatementItsTestDrives(t *testing.T) {
	source := []byte(`// TestCodeOnly drives one statement.
func TestCodeOnly(t *testing.T) {
    t.Errorf("CORE-CODE-1 was not honoured")
}
`)

	drives := statementsNamedIn(source)
	if got := drives["CORE-CODE-1"]; len(got) != 1 || got[0] != "TestCodeOnly" {
		t.Errorf("CORE-CODE-1 drives = %v, want [TestCodeOnly]", got)
	}
}

// TestAStrayBodyCommentDoesNotNameAStatement asserts that an identifier
// appearing only in a comment inside a test's body, which is neither the
// doc-comment header nor a code line, is absent from the result, because a
// comment can narrate a statement without the test asserting anything about
// it. The same source names a second identifier on a code line, so a fixture
// the scanner failed to read at all would not pass this test.
func TestAStrayBodyCommentDoesNotNameAStatement(t *testing.T) {
	source := []byte(`// TestNarratedOnly drives one statement.
func TestNarratedOnly(t *testing.T) {
    // CORE-NARRATED-1 is mentioned here for the reader, and asserted nowhere.
    t.Errorf("CORE-CODE-1 was not honoured")
}
`)

	drives := statementsNamedIn(source)
	if got, ok := drives["CORE-NARRATED-1"]; ok {
		t.Errorf("CORE-NARRATED-1 drives = %v, want it absent from the result", got)
	}
	if got := drives["CORE-CODE-1"]; len(got) != 1 || got[0] != "TestNarratedOnly" {
		t.Fatalf("CORE-CODE-1 drives = %v, want [TestNarratedOnly]: the fixture was not read", got)
	}
}

// TestACodeLineClosesADocCommentHeader asserts that a line of code ends the
// doc-comment block a "// TestX ..." line opened, so a comment written after
// that code line narrates the body rather than declaring the test's purpose.
// The header opens above a declaration that is not the matching test function,
// which is the case the three sibling scanner tests never reach: without the
// closing rule the header would stay open for the rest of the file and every
// later comment would count. The same source names a second identifier inside
// the header itself, so a fixture the scanner failed to read at all would not
// pass this test.
func TestACodeLineClosesADocCommentHeader(t *testing.T) {
	source := []byte(`// TestQueueOrder drives one statement.
//
// CORE-QUEUE-1 is named in the declared purpose.
var queueFixture = 1

// CORE-STRAY-1 is narrated here, after a code line closed the header.
`)

	drives := statementsNamedIn(source)
	if got, ok := drives["CORE-STRAY-1"]; ok {
		t.Errorf("CORE-STRAY-1 drives = %v, want it absent from the result", got)
	}
	if got := drives["CORE-QUEUE-1"]; len(got) != 1 || got[0] != "TestQueueOrder" {
		t.Fatalf("CORE-QUEUE-1 drives = %v, want [TestQueueOrder]: the fixture was not read", got)
	}
}
