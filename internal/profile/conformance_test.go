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
	"CORE-OUT-4":   "unreachable answers for a remote arbiter, and v0 serves one local bench with nothing to be out of reach of",
	"CORE-STATE-6": "reordering the state list is an edit to workbench.md rather than an act the tool offers, so v0 exposes no verb that reorders a flow",
	"CORE-STATE-8": "a permission, satisfied by every move the fixture drives, with no observable behaviour of its own to assert",
	"CORE-CARD-2":  "retitling a card is an edit to card.md, and v0 offers no retitle verb",
	"CORE-LINK-1":  "links are card data and v0 ships no link write-sugar, so a card carrying one is read but never offered to the tool",
	"CORE-LINK-2":  "a malformed link is refused at a write surface v0 does not offer",
	"CORE-LINK-3":  "a link naming an absent card is refused at a write surface v0 does not offer; fsck reports the dangler instead",
	"CORE-LINK-4":  "no closed set is imposed because nothing in the tool reads a link at all",
	"CORE-LINK-5":  "no verb consults a link, so no refusal can follow from one",
	"CORE-LINK-6":  "nothing in the tool adds a link, so no link can be added as a consequence of another",
	"CORE-LAYER-1": "a bench-declared extension kind is a definition surface v0 neither writes nor validates",
	"CORE-LAYER-2": "the same surface, read from the other side: an extension's content is preserved by the frontmatter rule rather than by a layer mechanism v0 implements",
	"CORE-LAYER-3": "the same surface again: v0 validates no layer declaration, so it can refuse none",
	"CORE-BLOCK-1": "an obligation on a blocked card rather than on a verb, asserted through the block effect and the fsck invariant",
	"CORE-STATE-4": "a permission the fixture exercises by declaring an operator-owned state",
	"CORE-STATE-5": "a permission the fixture exercises by declaring a capacity limit",
	"CORE-CARD-8":  "a permission the interchange round trip exercises by carrying an undefined field",
	"CORE-JSON-6":  "a permission the fixture exercises by carrying instructions, an operator flag and a capacity on its states",
	"CORE-JSON-8":  "a permission for a tool holding definitions in some other form; this one holds them in the shape the interchange form describes",
	"CORE-QUEUE-2": "a permission: v0 offers no order beside the fixed one, so there is nothing to keep available beside it",
	"CORE-TEXT-4":  "a permission the human rendering exercises by translating tokens where a person reads them",
	"CORE-BENCH-1": "a definition offered with no title is refused, and the offering surface is init --from, whose malformed refusal the interchange reader carries",
	"CORE-BENCH-2": "the same surface: an empty state list is refused by the interchange reader",
	"CORE-BENCH-3": "the same surface: a definition with no declared profile version is refused when the bench is opened",
	"CORE-STATE-1": "a duplicate state identifier is refused when the bench is opened, on a definition v0 writes rather than accepts from a caller",
	"CORE-STATE-2": "a state with no title is refused when the bench is opened",
	"CORE-STATE-3": "a state whose kind is outside the three is refused when the bench is opened",
	"CORE-STATE-7": "the legal moves a claim and a move carry include the next state, which the instruction tests assert as a whole rather than statement by statement",
	"CORE-STATE-9": "the same list carries no forward move out of a done state, asserted through the terminal refusal instead",
	"CORE-CARD-5":  "every card the tool reports carries one of the three substates by construction, since the reader defaults an absent one to ready",
	"CORE-CARD-6":  "asserted through the claim effect rather than as a reporting rule of its own",
	"CORE-CARD-7":  "asserted through the release and block effects, and through the fsck invariant that catches the converse",
	"CORE-OWNER-1": "every act names its owner by construction, since the journal writer takes the actor from a request the ladder resolved",
	"CORE-TEXT-1":  "every text the tool writes is UTF-8 by construction, since Go strings are and nothing transcodes",
	"CORE-HIST-1":  "asserted through the history test, which reads the acts the five verbs record rather than naming this statement per verb",
	"CORE-HIST-3":  "the journal is append-only by construction: nothing in the tool opens one for rewriting",
	"CORE-MOVE-11": "asserted through the ordering table, whose override case is refused not-operator",
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

	// A statement listed as out of reach that a test does drive is a stale
	// exclusion, which the report reports rather than tolerates.
	for id := range outOfReach {
		if len(driven[id]) > 0 {
			t.Errorf("%s is listed as out of reach and is driven by %v", id, driven[id])
		}
	}
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
		current := ""
		for _, line := range strings.Split(string(source), "\n") {
			line = strings.TrimRight(line, "\r")
			if m := testFunction.FindStringSubmatch(line); m != nil {
				current = m[1]
			}
			if m := testDoc.FindStringSubmatch(line); m != nil {
				current = m[1]
			}
			if current == "" {
				continue
			}
			for _, ref := range statementRef.FindAllStringSubmatch(line, -1) {
				id := ref[1]
				if !named(driven[id], current) {
					driven[id] = append(driven[id], current)
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

// named reports whether a test is already recorded against a statement.
func named(tests []string, want string) bool {
	for _, test := range tests {
		if test == want {
			return true
		}
	}
	return false
}
