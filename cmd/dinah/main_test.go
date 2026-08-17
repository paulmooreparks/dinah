package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/msg"
	"dinah/internal/profile"
	"dinah/internal/testenv"
	"dinah/internal/verb"
)

// TestMain redirects this binary's temporary directory outside the
// developer's home before any test runs, so the ancestor walk this
// package's tests exercise through the CLI cannot climb out of its own
// synthetic fixture tree and reach the real workbenches sitting above it.
// See internal/testenv's package comment for what this does and does not
// cover.
func TestMain(m *testing.M) {
	restore := testenv.IsolateTempDir()
	code := m.Run()
	restore()
	os.Exit(code)
}

// invocation is one run of the head with its streams captured.
type invocation struct {
	// code is the process exit code the run returned.
	code int
	// out and errw are what the run wrote to each stream.
	out  string
	errw string
}

// runCLI runs the head in a directory, with the streams captured.
func runCLI(t *testing.T, dir string, argv ...string) invocation {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(previous)
	out := &bytes.Buffer{}
	errw := &bytes.Buffer{}
	code := run(argv, strings.NewReader(""), out, errw)
	return invocation{code: code, out: out.String(), errw: errw.String()}
}

// newBench builds a bench for the head's tests and returns its directory,
// with the user base pointed somewhere disposable.
func newBench(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workbench")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, root, "init", "--slug", "fx", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	return root
}

// TestHelpBlockIsTheRatifiedSurface asserts that `dinah` with no arguments
// prints the ratified help block byte for byte, and that the binary offers
// exactly the commands that block lists and no others.
func TestHelpBlockIsTheRatifiedSurface(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "help.txt"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	got := runCLI(t, t.TempDir())
	if got.code != 0 {
		t.Fatalf("exit code: wanted 0, got %d", got.code)
	}
	if got.out != string(fixture) {
		t.Errorf("the emitted block differs from the spec's section 2:\n%s", diffLines(string(fixture), got.out))
	}

	// The block lists twenty-nine commands, and every command the binary
	// offers is either one of them or `help`, which the block's own last
	// line names.
	listed := 0
	for _, c := range commands {
		if c.group == "" {
			if c.name != "help" {
				t.Errorf("the binary offers the unlisted command %s", c.name)
			}
			continue
		}
		listed++
		if !strings.Contains(string(fixture), "  "+verb.Usage(c.name)+" ") {
			t.Errorf("the block does not list %s", c.name)
		}
	}
	if listed != 29 {
		t.Errorf("wanted twenty-nine listed commands, got %d", listed)
	}
}

// diffLines reports the first line at which two blocks differ, which is what
// a reader fixing the help text needs rather than both blocks in full.
func diffLines(wanted, got string) string {
	a := strings.Split(wanted, "\n")
	b := strings.Split(got, "\n")
	for i := range a {
		if i >= len(b) {
			return "the emitted block ends at line " + strconv.Itoa(i+1)
		}
		if a[i] != b[i] {
			return "line " + strconv.Itoa(i+1) + "\nwanted: " + a[i] + "\ngot:    " + b[i]
		}
	}
	return "the emitted block is longer than the fixture"
}

// TestExitCodesAndTheLeadingToken asserts that each outcome carries its own
// exit code and that the first whitespace-delimited token on stderr is the
// refusal name or the outcome name, which is what a script reads with cut.
func TestExitCodesAndTheLeadingToken(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")

	cases := []struct {
		name  string
		argv  []string
		code  int
		token string
		// sentence is a fragment the refusal a person reads must carry, set
		// on the refusals whose wording names the workbench so that the
		// product's word is pinned where the binary prints it.
		sentence string
	}{
		{name: "an act that succeeded", argv: []string{"claim", "fx-1"}, code: 0, token: ""},
		{name: "a card the workbench does not carry", argv: []string{"claim", "fx-99"}, code: 2, token: contract.UnknownCard, sentence: "this workbench carries no card fx-99"},
		{name: "a card another owner holds", argv: []string{"claim", "fx-1", "--actor", "bob"}, code: 2, token: contract.Held},
		{name: "a state the workbench does not declare", argv: []string{"move", "fx-1", "nowhere"}, code: 2, token: contract.UnknownState, sentence: "this workbench declares no state nowhere"},
		{name: "a block carrying no reason", argv: []string{"block", "fx-1"}, code: 2, token: contract.NoReason},
		{name: "an unblock by another owner", argv: []string{"unblock", "fx-1", "--actor", "bob"}, code: 2, token: contract.NotOperator},
		{name: "a release by another owner", argv: []string{"release", "fx-1", "--actor", "bob"}, code: 2, token: contract.NotHolder},
		{name: "a command the binary does not offer", argv: []string{"frobnicate"}, code: 2, token: contract.UnknownVerb},
		{name: "a flag the binary does not accept", argv: []string{"--frobnicate", "status"}, code: 2, token: contract.Usage},
		{name: "a delete carrying no confirmation", argv: []string{"delete", "fx-1"}, code: 2, token: contract.Unconfirmed},
		{name: "a guide topic nothing answers to", argv: []string{"guide", "nothing"}, code: 2, token: contract.UnknownGuide},
		{name: "a setting the tool does not know", argv: []string{"config", "get", "colour"}, code: 2, token: contract.UnknownKey},
		{name: "a reference nothing below the card answers to", argv: []string{"path", "fx-1/nowhere"}, code: 2, token: contract.UnknownPath, sentence: "nothing in this workbench answers to"},
		{name: "an archive of a state cards occupy", argv: []string{"archive", "Intake"}, code: 2, token: contract.Occupied},
		{name: "an extract into a directory that already holds one", argv: []string{"extract", benchDir(t, root)}, code: 2, token: contract.Exists},
		{name: "a card offered with no title", argv: []string{"add"}, code: 2, token: contract.Malformed},
		// The explicit basis arrives with the remote arbiter, so this head
		// offers no way to write one and the flag is not understood.
		{name: "an explicit basis, which v0 does not offer", argv: []string{"claim", "fx-1", "--basis", "sha256:0000"}, code: 2, token: contract.Usage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := runCLI(t, root, c.argv...)
			if got.code != c.code {
				t.Errorf("exit code: wanted %d, got %d (%s)", c.code, got.code, got.errw)
			}
			if c.token == "" {
				if got.errw != "" {
					t.Errorf("a successful act wrote to stderr: %q", got.errw)
				}
				return
			}
			leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
			if leading != c.token {
				t.Errorf("leading token: wanted %s, got %q", c.token, got.errw)
			}
			if len(strings.TrimSpace(got.errw)) <= len(c.token) {
				t.Error("the refusal name should be followed by a sentence a person reads")
			}
			if c.sentence != "" && !strings.Contains(got.errw, c.sentence) {
				t.Errorf("the refusal sentence: wanted %q in %q", c.sentence, got.errw)
			}
		})
	}
}

// TestInitWritesIntoTheContainerAndSaysWhere asserts what `init` creates and
// what it reports: a workbench inside the .dinah container of the directory it
// was run in, named by a generated identifier, with nothing left bare at that
// directory, and a message naming the directory it actually wrote to. The slug
// and the title come from the directory rather than from the identifier.
func TestInitWritesIntoTheContainerAndSaysWhere(t *testing.T) {
	base := emptyTree(t)
	root := filepath.Join(base, "release-notes")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got := runCLI(t, root, "init", "--operator", "ana")
	if got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	ids := bench.ListIDs(filepath.Join(root, bench.UserBaseName))
	if len(ids) != 1 {
		t.Fatalf("the container should hold one workbench, got %v", ids)
	}
	written := filepath.Join(root, bench.UserBaseName, ids[0])
	reported := initReported(t, got)
	if !sameDirs(t, []string{reported}, []string{written}) {
		t.Errorf("the message should name the directory init wrote, wanted %s, got %s", written, reported)
	}
	if !bench.IsID(filepath.Base(reported)) || filepath.Base(filepath.Dir(reported)) != bench.UserBaseName {
		t.Errorf("the path should be a generated identifier inside the container, got %s", reported)
	}
	if !bench.Exists(filepath.Join(written, "workbench.md")) {
		t.Errorf("%s carries no workbench.md", written)
	}
	if bench.Exists(filepath.Join(root, "workbench.md")) {
		t.Error("init wrote a workbench bare at the directory it was run in")
	}
	opened, err := bench.Open(written)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if opened.Slug != bench.Slugify("release-notes") || opened.Title != "release-notes" {
		t.Errorf("the slug and the title should name the directory, got %q and %q", opened.Slug, opened.Title)
	}
	if bench.IsID(opened.Slug) || bench.IsID(opened.Title) {
		t.Error("the slug and the title should not come from the generated identifier")
	}
}

// TestASecondInitAddsAWorkbenchBesideTheFirst asserts that a directory whose
// container already holds a workbench takes another one, and that the search
// then reports the choice it cannot make rather than picking.
func TestASecondInitAddsAWorkbenchBesideTheFirst(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "init", "--slug", "second", "--operator", "alka")
	if got.code != 0 {
		t.Fatalf("the second init: %d %s", got.code, got.errw)
	}
	ids := bench.ListIDs(filepath.Join(root, bench.UserBaseName))
	if len(ids) != 2 {
		t.Fatalf("the container should hold two workbenches, got %v", ids)
	}
	reported := runCLI(t, root, "status")
	if reported.code != 2 {
		t.Fatalf("an unqualified status over two workbenches: wanted 2, got %d (%s)", reported.code, reported.out)
	}
	leading := strings.SplitN(strings.TrimSpace(reported.errw), " ", 2)[0]
	if leading != contract.AmbiguousWorkbench {
		t.Errorf("leading token: wanted %s, got %q", contract.AmbiguousWorkbench, reported.errw)
	}
	for _, id := range ids {
		if !strings.Contains(reported.errw, id) {
			t.Errorf("the refusal should name both workbenches, %s is missing from %q", id, reported.errw)
		}
	}
}

// TestInitRefusesADirectoryCarryingABareWorkbench asserts the one refusal
// creation keeps. A container written beside a bare workbench.md would sit
// where the climbing search never looks, so `init` refuses there and writes
// nothing.
func TestInitRefusesADirectoryCarryingABareWorkbench(t *testing.T) {
	base := emptyTree(t)
	root := filepath.Join(base, "workbench")
	definition, err := bench.ReadDefinition([]byte(fmt.Sprintf(baseDefinition, "Bare")))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := bench.Instantiate(root, "bare", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	got := runCLI(t, root, "init", "--slug", "other", "--operator", "alka")
	if got.code != 2 {
		t.Fatalf("exit code: wanted 2, got %d (%s)", got.code, got.out)
	}
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.Exists {
		t.Errorf("leading token: wanted %s, got %q", contract.Exists, got.errw)
	}
	if !strings.Contains(got.errw, "already holds a workbench") {
		t.Errorf("the refusal sentence: wanted %q in %q", "already holds a workbench", got.errw)
	}
	if bench.Exists(filepath.Join(root, bench.UserBaseName)) {
		t.Error("the refused init left a container behind")
	}
}

// TestInitHelpKeepsItsRefusalList asserts that the help a person reads before
// running `init` still summarises the command the same way and still names the
// refusal creation keeps, since writing into the container removed no check.
func TestInitHelpKeepsItsRefusalList(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "help", "init")
	if got.code != 0 {
		t.Fatalf("help init: %d %s", got.code, got.errw)
	}
	for _, carried := range []string{
		"create a workbench here, optionally from a template",
		"the directory holds no workbench already",
		contract.Exists,
		"the source definition carries what the profile requires",
	} {
		if !strings.Contains(got.out, carried) {
			t.Errorf("the help should carry %q, got %q", carried, got.out)
		}
	}
}

// TestJSONIsIdenticalUnderEveryLanguage asserts CORE-TEXT-3 through the head:
// the canonical machine form carries canonical tokens only, so the same
// command under English and under Hindi emits byte-identical JSON.
func TestJSONIsIdenticalUnderEveryLanguage(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	runCLI(t, root, "add", "Another card")
	runCLI(t, root, "claim", "fx-1")

	commands := [][]string{
		{"status"},
		{"states"},
		{"ls"},
		{"next"},
		{"show", "fx-1"},
		{"log", "fx-1"},
		{"instructions", "fx-1"},
		{"whoami"},
		{"version", "--catalogs"},
		{"claim", "fx-1", "--actor", "bob"},
		{"move", "fx-2", "nowhere"},
		{"release", "fx-1", "--actor", "bob"},
	}
	for _, argv := range commands {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			english := runCLI(t, root, append(append([]string{}, argv...), "--json", "--lang", "en")...)
			hindi := runCLI(t, root, append(append([]string{}, argv...), "--json", "--lang", "hi")...)
			if english.out != hindi.out {
				t.Errorf("the machine form differs by language:\nen: %s\nhi: %s", english.out, hindi.out)
			}
			if english.out == "" {
				t.Error("the machine form was empty")
			}
			if !strings.Contains(english.out, "\"") {
				t.Errorf("the machine form does not look like JSON: %s", english.out)
			}
		})
	}

	// The human rendering does change with the language, or the catalog is
	// doing nothing.
	english := runCLI(t, root, "ls", "--lang", "en")
	hindi := runCLI(t, root, "ls", "--lang", "hi")
	if english.out == hindi.out {
		t.Error("the human rendering should differ by language")
	}
}

// TestHindiRendersDevanagari asserts that the Hindi catalog reaches the reader
// as valid UTF-8 Devanagari with no replacement characters, while every
// machine token in the same output keeps its canonical spelling.
func TestHindiRendersDevanagari(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	runCLI(t, root, "claim", "fx-1", "--quiet")
	runCLI(t, root, "move", "fx-1", "Doing", "--quiet")
	runCLI(t, root, "block", "fx-1", "the printer is on fire")

	got := runCLI(t, root, "ls", "--lang", "hi")
	if !strings.Contains(got.out, "बाधित") {
		t.Errorf("wanted the Devanagari rendering of the blocked substate, got %q", got.out)
	}
	if strings.ContainsRune(got.out, '�') {
		t.Error("the output carries a replacement character")
	}
	refused := runCLI(t, root, "claim", "fx-1", "--lang", "hi")
	leading := strings.SplitN(strings.TrimSpace(refused.errw), " ", 2)[0]
	if leading != contract.Blocked {
		t.Errorf("the refusal name should keep its canonical spelling under any language, got %q", refused.errw)
	}
}

// TestPathCarriesThePlumbingGuarantee asserts that path writes the resolved
// absolute path alone to stdout, one line, whatever the language setting and
// whatever --json says, and that on refusal stdout is empty while the refusal
// name leads stderr.
func TestPathCarriesThePlumbingGuarantee(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")

	for _, argv := range [][]string{
		{"path", "fx-1"},
		{"path", "fx-1", "--json"},
		{"path", "fx-1", "--lang", "hi"},
		{"path", "fx-1", "--json", "--lang", "hi"},
	} {
		got := runCLI(t, root, argv...)
		if got.code != 0 {
			t.Fatalf("%v: exit %d %s", argv, got.code, got.errw)
		}
		lines := strings.Split(strings.TrimSuffix(got.out, "\n"), "\n")
		if len(lines) != 1 {
			t.Errorf("%v: wanted one line on stdout, got %d", argv, len(lines))
		}
		if !filepath.IsAbs(lines[0]) {
			t.Errorf("%v: wanted an absolute path, got %q", argv, lines[0])
		}
		if !strings.HasSuffix(lines[0], "card.md") {
			t.Errorf("%v: wanted the card's anchor, got %q", argv, lines[0])
		}
		if got.errw != "" {
			t.Errorf("%v: wrote to stderr: %q", argv, got.errw)
		}
	}

	refused := runCLI(t, root, "path", "fx-99")
	if refused.out != "" {
		t.Errorf("on refusal stdout should be empty, got %q", refused.out)
	}
	if !strings.HasPrefix(refused.errw, contract.UnknownCard+" ") {
		t.Errorf("the refusal name should lead stderr, got %q", refused.errw)
	}
	if refused.code != 2 {
		t.Errorf("exit code: wanted 2, got %d", refused.code)
	}

	// The composition below a card resolves too, which is what the operator's
	// own `code (dinah path ...)` composition rests on.
	deeper := runCLI(t, root, "path", "fx-1/journal")
	if deeper.code != 0 || !strings.HasSuffix(strings.TrimSpace(deeper.out), "journal.ndjson") {
		t.Errorf("a composed path should resolve, got %d %q %q", deeper.code, deeper.out, deeper.errw)
	}
}

// TestPerCommandHelpFollowsTheProfile asserts that the generated per-command
// help lists a verb's checks in the profile's own order with each check's
// refusal name beside it, prefixed by the two workbench-level checks.
//
// The comparison is against the ordered lists extracted from
// docs/spec/core-profile.md, so a reordering there that the code does not
// follow fails the build, which is what DOC-ORDER-1 asks of a tool.
func TestPerCommandHelpFollowsTheProfile(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "docs", "spec", "core-profile.md"))
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	lists := profile.Preconditions(string(text))
	workbench := lists[profile.WorkbenchChecks]
	if len(workbench) != 2 {
		t.Fatalf("wanted the two workbench-level checks, got %d", len(workbench))
	}
	catalog := msg.For(msg.Base)
	for _, name := range verb.ContractVerbs {
		wanted := append(append([]profile.Precondition{}, workbench...), lists[name]...)
		got := verb.Checks(name)
		if len(got) != len(wanted) {
			t.Errorf("%s: wanted %d rows, got %d", name, len(wanted), len(got))
			continue
		}
		for i, check := range got {
			if check.Refusal != wanted[i].Refusal {
				t.Errorf("%s row %d: wanted %s, got %s", name, i+1, wanted[i].Refusal, check.Refusal)
			}
			if rendered := catalog.T(check.Key); rendered != wanted[i].Check {
				t.Errorf("%s row %d: wanted %q, got %q", name, i+1, wanted[i].Check, rendered)
			}
		}
	}

	// The rendered help carries the rows in that order, with the refusal name
	// beside each one.
	root := newBench(t)
	got := runCLI(t, root, "help", "move")
	if got.code != 0 {
		t.Fatalf("help move: %d %s", got.code, got.errw)
	}
	rows := 0
	for _, line := range strings.Split(got.out, "\n") {
		if regexp.MustCompile(`^  \d+ `).MatchString(line) {
			rows++
		}
	}
	if rows != 10 {
		t.Errorf("wanted move's eight checks behind the two workbench-level ones, got %d rows", rows)
	}
	if !strings.Contains(got.out, contract.AtCapacity) {
		t.Error("the rendered help should carry each check's refusal name")
	}
}

// printLiteral matches a print call carrying a double-quoted literal. The
// no-literals check reads it to find prose that never passed through the
// catalog.
var printLiteral = regexp.MustCompile("(?:Fprint|Fprintf|Fprintln|line|write)\\([^\n]*?\"([^\"\n]{2,})\"")

// prose matches a literal carrying three consecutive letters and a space,
// which is what separates a sentence from a format string or a separator.
var prose = regexp.MustCompile(`[A-Za-z]{3}`)

// TestNoUserFacingStringIsALiteral asserts the catalog rule: every string a
// person reads reaches them through a key, so a printed sentence that
// resolves through no key fails the build.
func TestNoUserFacingStringIsALiteral(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the head's sources: %v", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		for _, match := range printLiteral.FindAllStringSubmatch(string(source), -1) {
			literal := match[1]
			if !strings.Contains(literal, " ") || !prose.MatchString(literal) {
				continue
			}
			t.Errorf("%s: the literal %q is printed without passing through the catalog", entry.Name(), literal)
		}
	}
}

// TestEveryCatalogKeyTheCodeNamesExists asserts the other half of the same
// rule: a key the code renders is a key the base catalog carries, so a
// message can never come out as a bare key in front of a reader.
func TestEveryCatalogKeyTheCodeNamesExists(t *testing.T) {
	known := map[string]bool{}
	for _, key := range msg.Keys() {
		known[key] = true
	}
	if len(known) == 0 {
		t.Fatal("the base catalog carries no keys")
	}
	// The keys the code composes rather than writes out, which the scan
	// below cannot see as literals.
	for _, c := range commands {
		if !known["cmd."+c.name+".summary"] {
			t.Errorf("no summary key for the command %s", c.name)
		}
	}
	for _, flag := range globalFlags {
		if !known["flag."+flag.name+".summary"] {
			t.Errorf("no summary key for the flag --%s", flag.name)
		}
	}
	for _, group := range groups {
		if !known["help.group."+group] {
			t.Errorf("no heading key for the group %s", group)
		}
	}
	for _, name := range append(append([]string{}, contract.Declared...), contract.Introduced...) {
		if !known["refusal."+name] {
			t.Errorf("no sentence key for the refusal name %s", name)
		}
	}
	for _, name := range verb.Commands() {
		for _, check := range verb.Checks(name) {
			if !known[check.Key] {
				t.Errorf("no key for the check %s", check.Key)
			}
		}
	}
	// Every literal key the sources name.
	literal := regexp.MustCompile(`\.T[N]?\("([a-z][a-zA-Z0-9._-]*)"`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the head's sources: %v", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		for _, match := range literal.FindAllStringSubmatch(string(source), -1) {
			key := match[1]
			if strings.HasSuffix(key, ".") {
				continue
			}
			if !known[key] && !known[key+".other"] {
				t.Errorf("%s: the code renders the key %q, which the base catalog does not carry", entry.Name(), key)
			}
		}
	}
}

// limitedDefinition is a flow with a capacity limit and a station past the
// done station, which is what the refusals below need and the default flow
// `init` writes does not carry.
const limitedDefinition = `{
  "profile": "dinah-core/1.0",
  "title": "Limited",
  "states": [
    { "id": "b00000000001", "title": "Intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Doing", "kind": "work", "capacity": 1 },
    { "id": "b00000000003", "title": "Finished", "kind": "done" },
    { "id": "b00000000004", "title": "Aftercare", "kind": "work" }
  ]
}`

// TestTheRemainingRefusalsLeadStderr sweeps the refusal names the first table
// cannot reach without a fixture of their own, so that between the two every
// name a CLI invocation can provoke is asserted through stderr.
//
// Four names are structurally unreachable here and are driven at the library
// level instead: not-requester, because the cli head's claim takes no holder
// argument and so can never name one other than the asker; layer-collision,
// because v0 validates no layer declaration; dinah.locked, which needs a
// second process holding the card mid-transaction; and dinah.no-editor, which
// needs an environment carrying no editor at any rung and no fallback binary
// on the path.
//
// The three names discovery raises before a bench is open (dinah.no-workbench,
// dinah.no-workbench-found and dinah.ambiguous-workbench) are swept by
// TestRefusalsSayWhereTheToolLookedAndWhatComesNext below, which needs a fixture per
// case and asserts the location and the next step as well as the name.
func TestTheRemainingRefusalsLeadStderr(t *testing.T) {
	cases := []struct {
		name  string
		build func(*testing.T) (string, []string)
		token string
		// sentence is a fragment the refusal a person reads must carry, set
		// on the refusals whose wording names the workbench.
		sentence string
	}{
		{
			name: "a destination that has reached its limit",
			build: func(t *testing.T) (string, []string) {
				root := newLimitedBench(t)
				runCLI(t, root, "add", "First")
				runCLI(t, root, "add", "Second")
				runCLI(t, root, "move", "lim-1", "Doing")
				return root, []string{"move", "lim-2", "Doing"}
			},
			token: contract.AtCapacity,
		},
		{
			name: "a forward move out of a done state",
			build: func(t *testing.T) (string, []string) {
				root := newLimitedBench(t)
				runCLI(t, root, "add", "First")
				runCLI(t, root, "move", "lim-1", "Finished")
				return root, []string{"move", "lim-1", "Aftercare"}
			},
			token: contract.Terminal,
		},
		{
			name: "an invocation resolving no owner at any rung",
			build: func(t *testing.T) (string, []string) {
				root := newLimitedBench(t)
				runCLI(t, root, "add", "First")
				t.Setenv("DINAH_ACTOR", "")
				return root, []string{"claim", "lim-1"}
			},
			token: contract.NoOwner,
		},
		{
			name: "a workbench designating no operator",
			build: func(t *testing.T) (string, []string) {
				root := newLimitedBench(t)
				runCLI(t, root, "add", "First")
				editAnchor(t, root, "operator: alka\n", "")
				return root, []string{"claim", "lim-1"}
			},
			token:    contract.NoOperator,
			sentence: "this workbench designates no operator, so its reserved actions are dead",
		},
		{
			name: "a workbench declaring a profile major this binary does not implement",
			build: func(t *testing.T) (string, []string) {
				root := newLimitedBench(t)
				editAnchor(t, root, "profile: dinah-core/1.0", "profile: dinah-core/9.0")
				return root, []string{"status"}
			},
			token: contract.UnsupportedVer,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, argv := c.build(t)
			got := runCLI(t, dir, argv...)
			if got.code != 2 {
				t.Errorf("exit code: wanted 2, got %d (%s)", got.code, got.errw)
			}
			leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
			if leading != c.token {
				t.Errorf("leading token: wanted %s, got %q", c.token, got.errw)
			}
			if c.sentence != "" && !strings.Contains(got.errw, c.sentence) {
				t.Errorf("the refusal sentence: wanted %q in %q", c.sentence, got.errw)
			}
		})
	}
}

// newLimitedBench builds a bench from limitedDefinition and returns its
// directory.
func newLimitedBench(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workbench")
	source := filepath.Join(base, "definition.json")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	if err := os.WriteFile(source, []byte(limitedDefinition), 0o644); err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got := runCLI(t, root, "init", "--from", source, "--slug", "lim", "--operator", "alka")
	if got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	// A workbench built from a template lands in the container like any
	// other, and the message names the directory it landed in, which is
	// where every later command in these tests reads it from.
	written := benchDir(t, root)
	if reported := initReported(t, got); !sameDirs(t, []string{reported}, []string{written}) {
		t.Fatalf("the message should name the directory init wrote, wanted %s, got %s", written, reported)
	}
	if !bench.Exists(filepath.Join(written, "workbench.md")) {
		t.Fatalf("%s carries no workbench.md", written)
	}
	return root
}

// initReported asserts the wording of the message `init` printed and returns
// the path that message named.
//
// The wording is asserted literally and the path is handed to sameDirs rather
// than compared as a string, because macOS reaches its temporary directory
// through a symlink and Windows hands out the short 8.3 form of a long user
// name, so the tool prints a spelling of the path the test did not build.
func initReported(t *testing.T, got invocation) string {
	t.Helper()
	const prefix = "Workbench created at "
	line := strings.TrimSuffix(got.out, "\n")
	if strings.Contains(line, "\n") || !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, ".") {
		t.Fatalf("the message: wanted one line reading %q<path>%q, got %q", prefix, ".", got.out)
	}
	return strings.TrimSuffix(strings.TrimPrefix(line, prefix), ".")
}

// benchDir returns the directory holding the workbench that a CLI run in root
// resolves to: root itself when a bare workbench sits there, and the sole
// entry of root's container otherwise, which is where `init` writes.
func benchDir(t *testing.T, root string) string {
	t.Helper()
	if bench.Exists(filepath.Join(root, "workbench.md")) {
		return root
	}
	base := filepath.Join(root, bench.UserBaseName)
	ids := bench.ListIDs(base)
	if len(ids) != 1 {
		t.Fatalf("%s holds %d workbenches, wanted one", base, len(ids))
	}
	return filepath.Join(base, ids[0])
}

// editAnchor rewrites the workbench anchor, which is how the cases above build
// a bench that a hand edit has put outside what the tool will serve.
func editAnchor(t *testing.T, root, from, to string) {
	t.Helper()
	path := filepath.Join(benchDir(t, root), "workbench.md")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(text), from) {
		t.Fatalf("the anchor carries no %q", from)
	}
	edited := strings.Replace(string(text), from, to, 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// guideInvocation matches a dinah command line inside a guide's fenced block.
var guideInvocation = regexp.MustCompile(`(?m)^dinah ([a-z_]+)((?: [^\n]*)?)$`)

// guideFlag matches a long flag inside such a line.
var guideFlag = regexp.MustCompile(`--([a-z-]+)`)

// TestTheGuidesTeachOnlyDeclaredFlags asserts the reference rule against the
// shipped guides: every command a guide teaches exists, and every flag it
// spells is one that command declares or one of the global flags.
//
// This is the guard the cycle-one review's third finding wanted. The guide
// taught `dinah init --slug proj` while the generated help named only
// `--from`, and nothing in the suite could see the disagreement, because the
// help fixture counts commands rather than flags.
func TestTheGuidesTeachOnlyDeclaredFlags(t *testing.T) {
	global := map[string]bool{}
	for _, flag := range globalFlags {
		global[flag.name] = true
	}
	checked := 0
	for _, topic := range guide.Topics() {
		text, err := guide.Text(topic)
		if err != nil {
			t.Fatalf("guide %s: %v", topic, err)
		}
		for _, invocation := range guideInvocation.FindAllStringSubmatch(text, -1) {
			name, rest := invocation[1], invocation[2]
			if _, ok := lookup(name); !ok {
				t.Errorf("%s: the guide teaches the command %q, which the binary does not offer", topic, name)
				continue
			}
			declared := map[string]bool{}
			for _, param := range verb.Params(name) {
				if param.Flag {
					declared[param.Name] = true
				}
			}
			for _, flag := range guideFlag.FindAllStringSubmatch(rest, -1) {
				checked++
				if declared[flag[1]] || global[flag[1]] {
					continue
				}
				t.Errorf("%s: the guide teaches `dinah %s --%s`, which %s does not declare", topic, name, flag[1], name)
			}
		}
	}
	if checked == 0 {
		t.Error("no flagged invocation was found in any guide, so this test proves nothing")
	}
}

// TestCheckDeclaresItsRepairFlagsOnEverySurface asserts that the three flags
// which repair rather than report are declared once and projected everywhere:
// the ratified help block's check line names them, the generated help for the
// command names them from the same definition, and the argument parser accepts
// them. One completes an interrupted structural act, one stamps the creation
// ordinals a workbench written before the field carries none of, and one
// derives the slugs of states written before that field existed.
//
// The change to the fixture's check line is a ratified one rather than drift.
// The MCP head's schema is generated from the same parameter list and is
// asserted against it by TestToolSurfaceIsTheProjection.
func TestCheckDeclaresItsRepairFlagsOnEverySurface(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "help.txt"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if !strings.Contains(string(fixture), "  check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states] ") {
		t.Error("the ratified block's check line does not name every repair flag")
	}
	if got := verb.Usage("check"); got != "check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states]" {
		t.Errorf("the one definition composes %q", got)
	}

	root := newBench(t)
	generated := runCLI(t, root, "help", "check")
	if generated.code != 0 {
		t.Fatalf("help check: %d %s", generated.code, generated.errw)
	}
	for _, flag := range []string{"--finish", "--migrate-ordinals", "--migrate-slugs", "--migrate-states"} {
		if !strings.Contains(generated.out, flag) {
			t.Errorf("the generated help does not name %s:\n%s", flag, generated.out)
		}
		if got := runCLI(t, root, "check", flag); got.code != 0 {
			t.Errorf("check %s on a clean workbench: %d %s", flag, got.code, got.errw)
		}
	}
}

// settingsHome points the user base at a directory of this test's own, with
// every variable the three ladders read cleared, and returns both the home and
// a directory to run from. Nothing here touches the real user base, because
// the settings commands write to whatever DINAH_HOME names.
func settingsHome(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	t.Setenv("DINAH_HOME", home)
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("DINAH_FORMAT", "")
	for _, name := range []string{"DINAH_ACTOR", "DINAH_LANG", "DINAH_EDITOR", "VISUAL", "EDITOR", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(name, "")
	}
	return home, base
}

// settingRows parses the machine form of the settings listing, which is what
// asserts the shape as well as the content.
func settingRows(t *testing.T, got invocation) map[string]verb.SettingView {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("config --json: %d %s", got.code, got.errw)
	}
	var rows []verb.SettingView
	if err := json.Unmarshal([]byte(got.out), &rows); err != nil {
		t.Fatalf("the machine form should be an array of rows: %v\n%s", err, got.out)
	}
	byKey := map[string]verb.SettingView{}
	for _, row := range rows {
		byKey[row.Key] = row
	}
	return byKey
}

// TestConfigListsEverySettingWithTheRungThatAnsweredIt asserts the listing the
// bare command prints: every key the tool knows appears whether or not anybody
// ever set it, each row carries the value the ladder resolves rather than the
// stored one, and the rung that answered is named.
//
// The listing is a command form of its own rather than a variation on an
// existing read, so it gets a test of its own; the ladders themselves are
// asserted in internal/bench, where they live.
func TestConfigListsEverySettingWithTheRungThatAnsweredIt(t *testing.T) {
	home, dir := settingsHome(t)

	// A home nobody has ever written to still lists every known key, in the
	// declared order, with the rung that answered.
	listed := runCLI(t, dir, "config")
	if listed.code != 0 {
		t.Fatalf("config: %d %s", listed.code, listed.errw)
	}
	for _, key := range bench.ConfigKeys {
		if !strings.Contains(listed.out, key) {
			t.Errorf("the listing drops %s on a home nobody has written to:\n%s", key, listed.out)
		}
	}
	rows := settingRows(t, runCLI(t, dir, "--json", "config"))
	if len(rows) != len(bench.ConfigKeys) {
		t.Errorf("wanted a row per known key, got %d", len(rows))
	}
	if rows["lang"].Value != "en" || rows["lang"].Source != bench.SourceDefault {
		t.Errorf("a language nobody chose: wanted en at %s, got %+v", bench.SourceDefault, rows["lang"])
	}
	if rows["actor"].Value != "" || rows["actor"].Source != bench.SourceUnset {
		t.Errorf("an owner nobody set: wanted an empty value at %s, got %+v", bench.SourceUnset, rows["actor"])
	}
	// The editor's own rungs are all unset here, so whatever answered came
	// from the platform fallback or from nowhere. Naming any higher rung
	// would be a rung that did not answer.
	for _, rung := range []string{bench.SourceEditorVar, bench.SourceConfig, bench.SourceVisual, bench.SourceEnvironment} {
		if rows["editor"].Source == rung {
			t.Errorf("with every editor variable cleared, the row should not name %s", rung)
		}
	}

	// A key the file carries and the tool does not know is reported rather
	// than dropped, and it carries the source that says so.
	if got := runCLI(t, dir, "config", "set", "lang", "fr"); got.code != 0 {
		t.Fatalf("config set: %d %s", got.code, got.errw)
	}
	path := filepath.Join(bench.UserBase(home), bench.ConfigName)
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the settings file: %v", err)
	}
	written := strings.Replace(string(stored), "lang: fr", "lang: fr\ncolour: green", 1)
	if written == string(stored) {
		t.Fatalf("the settings file does not carry the key that was just set:\n%s", stored)
	}
	if err := os.WriteFile(path, []byte(written), 0o644); err != nil {
		t.Fatalf("write the settings file: %v", err)
	}
	rows = settingRows(t, runCLI(t, dir, "--json", "config"))
	if rows["colour"].Value != "green" || rows["colour"].Source != bench.SourceUnknown {
		t.Errorf("a key the tool does not know: wanted green at %s, got %+v", bench.SourceUnknown, rows["colour"])
	}
	if rows["lang"].Value != "fr" || rows["lang"].Source != bench.SourceConfig {
		t.Errorf("a language the file carries: wanted fr at %s, got %+v", bench.SourceConfig, rows["lang"])
	}

	// The rungs above the file are named as themselves.
	t.Setenv("DINAH_LANG", "de")
	rows = settingRows(t, runCLI(t, dir, "--json", "config"))
	if rows["lang"].Value != "de" || rows["lang"].Source != bench.SourceEnvironment {
		t.Errorf("a language from the environment: wanted de at %s, got %+v", bench.SourceEnvironment, rows["lang"])
	}
	rows = settingRows(t, runCLI(t, dir, "--json", "--lang", "hi", "config"))
	if rows["lang"].Value != "hi" || rows["lang"].Source != bench.SourceFlag {
		t.Errorf("a language from the flag: wanted hi at %s, got %+v", bench.SourceFlag, rows["lang"])
	}
}

// TestTheEditorRowNamesWhichVariableWon asserts the distinction the card exists
// for: an editor that came from VISUAL and one that came from EDITOR are
// different answers, and the listing never collapses the five rungs of that
// ladder into one generic environment source.
func TestTheEditorRowNamesWhichVariableWon(t *testing.T) {
	_, dir := settingsHome(t)

	cases := []struct {
		name   string
		set    map[string]string
		config string
		wanted string
		source string
	}{
		{name: "EDITOR", set: map[string]string{"EDITOR": "ed"}, wanted: "ed", source: bench.SourceEnvironment},
		{
			name:   "VISUAL over EDITOR",
			set:    map[string]string{"EDITOR": "ed", "VISUAL": "vim"},
			wanted: "vim",
			source: bench.SourceVisual,
		},
		{
			name:   "the settings file over both",
			set:    map[string]string{"EDITOR": "ed", "VISUAL": "vim"},
			config: "kak",
			wanted: "kak",
			source: bench.SourceConfig,
		},
		{
			name:   "DINAH_EDITOR over everything",
			set:    map[string]string{"EDITOR": "ed", "VISUAL": "vim", "DINAH_EDITOR": "helix"},
			config: "kak",
			wanted: "helix",
			source: bench.SourceEditorVar,
		},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, name := range []string{"DINAH_EDITOR", "VISUAL", "EDITOR"} {
				t.Setenv(name, c.set[name])
			}
			if c.config != "" {
				if got := runCLI(t, dir, "config", "set", "editor", c.config); got.code != 0 {
					t.Fatalf("config set: %d %s", got.code, got.errw)
				}
			}
			rows := settingRows(t, runCLI(t, dir, "--json", "config"))
			if rows["editor"].Value != c.wanted || rows["editor"].Source != c.source {
				t.Errorf("wanted %s at %s, got %+v", c.wanted, c.source, rows["editor"])
			}
			seen[rows["editor"].Source] = true
		})
	}
	if len(seen) != len(cases) {
		t.Errorf("the four rungs exercised here should report four distinct sources, got %d", len(seen))
	}
}

// TestConfigGetAndSetAreUnchanged asserts that giving the bare command a
// listing left the two verbs where they were: the same round trip, the same
// refusal name on a key the tool does not know, and the same exit codes.
func TestConfigGetAndSetAreUnchanged(t *testing.T) {
	_, dir := settingsHome(t)

	if got := runCLI(t, dir, "config", "set", "actor", "alka"); got.code != 0 {
		t.Fatalf("config set: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, dir, "config", "get", "actor"); got.code != 0 || strings.TrimSpace(got.out) != "alka" {
		t.Errorf("the round trip: %d %q %s", got.code, got.out, got.errw)
	}
	cases := [][]string{
		{"config", "get", "colour"},
		{"config", "set", "colour", "green"},
		{"config", "get"},
		{"config", "set"},
	}
	for _, argv := range cases {
		got := runCLI(t, dir, argv...)
		if got.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Errorf("%v: wanted the refused exit code, got %d", argv, got.code)
		}
		if leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]; leading != contract.UnknownKey {
			t.Errorf("%v: wanted the leading token %s, got %q", argv, contract.UnknownKey, got.errw)
		}
	}
	// A word that is neither verb is still a usage refusal rather than a
	// listing, so the bare form is the only new spelling.
	stray := runCLI(t, dir, "config", "bogus")
	if stray.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Errorf("a stray word: wanted the refused exit code, got %d", stray.code)
	}
	if leading := strings.SplitN(strings.TrimSpace(stray.errw), " ", 2)[0]; leading != contract.Usage {
		t.Errorf("a stray word: wanted the leading token %s, got %q", contract.Usage, stray.errw)
	}
}

// TestTheOrdinalMigrationSaysWhatItGuessed asserts that the repair reports
// itself on both the surfaces a caller reads: the human form prints what it
// stamped and names the entity it could only place by the directory listing,
// and the machine form carries the same count and the same finding.
//
// A repair that stamps in silence leaves a guess and a recovered fact looking
// alike on disk forever, and the run is the last moment anybody can tell them
// apart.
func TestTheOrdinalMigrationSaysWhatItGuessed(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	located := runCLI(t, root, "path", "fx-1")
	if located.code != 0 {
		t.Fatalf("path: %d %s", located.code, located.errw)
	}
	cardDir := filepath.Dir(strings.TrimSpace(located.out))

	// A comment nobody journalled, carrying no ordinal, which is the shape
	// every hand-created entity on a live workbench has.
	anchor := filepath.Join(cardDir, "comments", "e00000000001", "comment.md")
	if err := os.MkdirAll(filepath.Dir(anchor), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "---\nts: 2026-08-17T09:00:00Z\nauthor: alka\n---\nBy hand.\n"
	if err := os.WriteFile(anchor, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	migrated := runCLI(t, root, "check", "--migrate-ordinals")
	if !strings.Contains(migrated.out, "Stamped 1 creation ordinal.") {
		t.Errorf("the migration did not say what it stamped:\n%s", migrated.out)
	}
	if !strings.Contains(migrated.out, "e00000000001") || !strings.Contains(migrated.out, "guess") {
		t.Errorf("the migration did not name the entity it guessed at:\n%s", migrated.out)
	}

	// A second hand-created comment, so the machine form has a stamp and a
	// guess of its own to report rather than the empty answer a second run
	// over the same workbench gives.
	second := filepath.Join(cardDir, "comments", "e00000000002", "comment.md")
	if err := os.MkdirAll(filepath.Dir(second), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(second, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	machine := runCLI(t, root, "--json", "check", "--migrate-ordinals")
	flattened := strings.Join(strings.Fields(machine.out), "")
	if !strings.Contains(flattened, `"stamped_ordinals":1`) {
		t.Errorf("the machine form carries no stamped count:\n%s", machine.out)
	}
	if !strings.Contains(machine.out, bench.FindingOrdinalGuessed) || !strings.Contains(machine.out, "e00000000002") {
		t.Errorf("the machine form does not name the entity it guessed at:\n%s", machine.out)
	}
}

// TestStatesCarryTheirSlugOnBothSurfaces asserts that the slug reaches every
// surface a caller reads a state through: the human listing prints it beside
// the identifier, the machine form carries it as a member of each state
// object, and a reference typed as the slug reaches the state without the
// quoting a spaced title needs.
func TestStatesCarryTheirSlugOnBothSurfaces(t *testing.T) {
	root := newBench(t)

	human := runCLI(t, root, "states")
	if human.code != 0 {
		t.Fatalf("states: %d %s", human.code, human.errw)
	}
	for _, slug := range []string{"intake", "doing", "done"} {
		if !strings.Contains(human.out, slug) {
			t.Errorf("the listing does not print the slug %s:\n%s", slug, human.out)
		}
	}

	machine := runCLI(t, root, "--json", "states")
	if machine.code != 0 {
		t.Fatalf("states --json: %d %s", machine.code, machine.errw)
	}
	var states []verb.StateView
	if err := json.Unmarshal([]byte(machine.out), &states); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	if len(states) != 3 {
		t.Fatalf("the machine form carries %d states", len(states))
	}
	for position, wanted := range []string{"intake", "doing", "done"} {
		if got := states[position].Slug; got != wanted {
			t.Errorf("state %d carries slug %q, wanted %q", position+1, got, wanted)
		}
	}

	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	listed := runCLI(t, root, "ls", "--state", "intake")
	if listed.code != 0 {
		t.Fatalf("ls by slug: %d %s", listed.code, listed.errw)
	}
	if !strings.Contains(listed.out, "A card") {
		t.Errorf("a slug should name a state on the command line:\n%s", listed.out)
	}
}

// TestTheSlugMigrationRepairsAWorkbenchWrittenBeforeTheField asserts the
// one-time repair end to end: the checker names each state carrying no slug,
// the repair derives one from the title and says which state got which slug on
// both surfaces, and the workbench checks clean afterwards.
func TestTheSlugMigrationRepairsAWorkbenchWrittenBeforeTheField(t *testing.T) {
	root := newBench(t)
	stripSlugs(t, root)

	reported := runCLI(t, root, "check")
	if !strings.Contains(reported.out, "carries no slug") {
		t.Errorf("the checker did not report the missing slugs:\n%s", reported.out)
	}

	migrated := runCLI(t, root, "check", "--migrate-slugs")
	if migrated.code != 0 {
		t.Fatalf("check --migrate-slugs: %d %s", migrated.code, migrated.errw)
	}
	if !strings.Contains(migrated.out, "Assigned 3 state slugs.") {
		t.Errorf("the migration did not say what it assigned:\n%s", migrated.out)
	}
	for _, slug := range []string{"intake", "doing", "done"} {
		if !strings.Contains(migrated.out, slug) {
			t.Errorf("the migration did not name the slug %s:\n%s", slug, migrated.out)
		}
	}

	again := runCLI(t, root, "--json", "check", "--migrate-slugs")
	if again.code != 0 {
		t.Fatalf("a second run: %d %s", again.code, again.errw)
	}
	if strings.Contains(again.out, `"assigned_slugs"`) {
		t.Errorf("a second run assigned something:\n%s", again.out)
	}
	if !strings.Contains(again.out, `"migrated_slugs"`) {
		t.Errorf("a second run did not say the migration ran:\n%s", again.out)
	}
}

// TestAHandTypedSlugLeavesTheWorkbenchOpenable asserts the corner an operator
// has to be able to get out of with the tool: somebody types a slug into a
// state anchor by hand and gets it wrong, and the workbench goes on opening,
// the checker names the state and the file, and the repair that fills in the
// states around it still runs.
//
// Every command opens the workbench before it can do anything with it, so a
// reader refusing a stored slug would take the whole workbench away over one
// mistyped line, including the check that reports the mistake and the
// migration that would have finished the job. The workbench declares a major
// below the one CORE-STATE-10 arrives at, which is where a stored slug binds
// nothing yet.
func TestAHandTypedSlugLeavesTheWorkbenchOpenable(t *testing.T) {
	t.Run("a slug outside the grammar", func(t *testing.T) {
		root := newBench(t)
		stripSlugs(t, root)
		state, anchor := writeStateSlug(t, root, 0, "Caf--Corner")

		listed := runCLI(t, root, "states")
		if listed.code != 0 {
			t.Fatalf("the workbench should still open: %d %s", listed.code, listed.errw)
		}
		if !strings.Contains(listed.out, "Caf--Corner") {
			t.Errorf("the listing should carry the slug as it stands:\n%s", listed.out)
		}

		reported := runCLI(t, root, "check")
		for _, fragment := range []string{state, anchor, "is not a letter followed by"} {
			if !strings.Contains(reported.out, fragment) {
				t.Errorf("the checker should carry %q:\n%s", fragment, reported.out)
			}
		}

		migrated := runCLI(t, root, "check", "--migrate-slugs")
		if !strings.Contains(migrated.out, "Assigned 2 state slugs.") {
			t.Errorf("the repair did not reach the states around the bad one:\n%s", migrated.out)
		}
		if !strings.Contains(migrated.out, state) || !strings.Contains(migrated.out, anchor) {
			t.Errorf("the repair stopped naming the state it left alone:\n%s", migrated.out)
		}

		reopened := runCLI(t, root, "states")
		if reopened.code != 0 {
			t.Fatalf("the repaired workbench should open: %d %s", reopened.code, reopened.errw)
		}
		for _, slug := range []string{"doing", "done"} {
			if !strings.Contains(reopened.out, slug) {
				t.Errorf("the listing does not carry the repaired slug %s:\n%s", slug, reopened.out)
			}
		}
	})

	t.Run("a slug another state already carries", func(t *testing.T) {
		root := newBench(t)
		writeStateSlug(t, root, 0, "done")

		listed := runCLI(t, root, "states")
		if listed.code != 0 {
			t.Fatalf("the workbench should still open: %d %s", listed.code, listed.errw)
		}

		// The walk names the second state to carry the value, which is the
		// one whose reference has stopped answering for it alone.
		duplicate, anchor := writeStateSlug(t, root, 2, "done")
		reported := runCLI(t, root, "check")
		for _, fragment := range []string{duplicate, anchor, "another state of this workbench also carries"} {
			if !strings.Contains(reported.out, fragment) {
				t.Errorf("the checker should carry %q:\n%s", fragment, reported.out)
			}
		}

		migrated := runCLI(t, root, "check", "--migrate-slugs")
		if !strings.Contains(migrated.out, "Assigned 0 state slugs.") {
			t.Errorf("the repair should run and find nothing to assign:\n%s", migrated.out)
		}
	})
}

// writeStateSlug types a slug into one state anchor of a workbench the way a
// person editing the file by hand would, and returns the state's identifier
// and the anchor's path, which are the two things a report about it names.
func writeStateSlug(t *testing.T, root string, position int, slug string) (string, string) {
	t.Helper()
	machine := runCLI(t, root, "--json", "states")
	var states []verb.StateView
	if err := json.Unmarshal([]byte(machine.out), &states); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	if position >= len(states) {
		t.Fatalf("the workbench carries %d states", len(states))
	}
	path := filepath.Join(benchDir(t, root), "states", states[position].ID, "state.md")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var kept []string
	for _, line := range strings.Split(string(text), "\n") {
		if strings.HasPrefix(line, "slug: ") {
			continue
		}
		kept = append(kept, line)
		if strings.HasPrefix(line, "title: ") {
			kept = append(kept, "slug: "+slug)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return states[position].ID, path
}

// stripSlugs removes the slug from every state anchor of a workbench, which is
// the shape a workbench written before the field has on disk.
func stripSlugs(t *testing.T, root string) {
	t.Helper()
	states := filepath.Join(benchDir(t, root), "states")
	entries, err := os.ReadDir(states)
	if err != nil {
		t.Fatalf("read states: %v", err)
	}
	for _, entry := range entries {
		path := filepath.Join(states, entry.Name(), "state.md")
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var kept []string
		for _, line := range strings.Split(string(text), "\n") {
			if strings.HasPrefix(line, "slug: ") {
				continue
			}
			kept = append(kept, line)
		}
		if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

// TestRefusalsSayWhereTheToolLookedAndWhatComesNext asserts the standard this
// card was raised over, against the three refusals discovery can reach and the
// malformed refusal a workbench predating the profile line raises. Each names
// where the tool looked, what it wanted there, and what the reader does next.
//
// The last case is the other half of the same name. A malformed refusal over a
// request argument has no file to name, so it carries neither fragment, and
// the sentence a person reads is the bare one it has always been.
//
// Each case is asserted twice, once for the person reading stderr and once for
// the script reading --json, because the machine form of a discovery refusal
// went nowhere at all before this card.
func TestRefusalsSayWhereTheToolLookedAndWhatComesNext(t *testing.T) {
	cases := []struct {
		name  string
		build func(*testing.T) (string, []string)
		token string
		// carries are fragments the sentence a person reads must hold: the
		// location, and the next step the reader takes.
		carries []string
		// context names the members the machine form carries beyond the
		// refusal name and its detail.
		context []string
	}{
		{
			name: "a workbench predating the profile line",
			build: func(t *testing.T) (string, []string) {
				root := newBench(t)
				editAnchor(t, root, "profile: dinah-core/1.0\n", "")
				return root, []string{"whoami"}
			},
			token: contract.Malformed,
			carries: []string{
				"profile is missing, empty, or will not parse",
				// `init` writes into the container, so the anchor the
				// refusal names sits under it rather than at the directory
				// the command was run in.
				filepath.Join("workbench", bench.UserBaseName),
				"workbench.md",
				"dinah check",
			},
			context: []string{"path"},
		},
		{
			name: "a --workbench pointed at a directory holding no workbench",
			build: func(t *testing.T) (string, []string) {
				root := newBench(t)
				elsewhere := t.TempDir()
				return root, []string{"status", "--workbench", elsewhere}
			},
			token:   contract.NoWorkbench,
			carries: []string{"carries no workbench.md", "point --workbench at a directory that does"},
		},
		{
			name: "no workbench reachable anywhere",
			build: func(t *testing.T) (string, []string) {
				return emptyTree(t), []string{"status"}
			},
			token:   contract.NoWorkbenchFound,
			carries: []string{"no workbench was found walking up from", "user base at", "dinah init"},
			context: []string{"home"},
		},
		{
			name: "a base holding two workbenches and nothing closer",
			build: func(t *testing.T) (string, []string) {
				tree := emptyTree(t)
				rooms := map[string]string{"d00000000001": "one", "d00000000002": "two"}
				for id, slug := range rooms {
					room := filepath.Join(tree, ".dinah", id)
					definition, err := bench.ReadDefinition([]byte(fmt.Sprintf(baseDefinition, id)))
					if err != nil {
						t.Fatalf("definition: %v", err)
					}
					if err := bench.Instantiate(room, slug, "alka", definition); err != nil {
						t.Fatalf("instantiate: %v", err)
					}
				}
				return tree, []string{"status"}
			},
			token:   contract.AmbiguousWorkbench,
			carries: []string{"are all reachable from", "choose one with --workbench"},
			context: []string{"base"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, argv := c.build(t)
			got := runCLI(t, dir, argv...)
			if got.code != 2 {
				t.Fatalf("exit code: wanted 2, got %d (%s)", got.code, got.errw)
			}
			leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
			// The walk climbs to the volume root, so on a machine whose own
			// home directory carries a .dinah holding several workbenches the
			// search meets that ambiguity before it can exhaust. The refusal
			// is then honestly a different one, and the case is skipped rather
			// than asserted against whatever the machine happens to hold.
			if c.token == contract.NoWorkbenchFound && leading == contract.AmbiguousWorkbench {
				t.Skip("a directory above the temporary tree holds several workbenches of its own")
			}
			if leading != c.token {
				t.Errorf("leading token: wanted %s, got %q", c.token, got.errw)
			}
			for _, fragment := range c.carries {
				if !strings.Contains(got.errw, fragment) {
					t.Errorf("the refusal should carry %q, got %q", fragment, got.errw)
				}
			}

			machine := runCLI(t, dir, append([]string{"--json"}, argv...)...)
			report := map[string]any{}
			if err := json.Unmarshal([]byte(machine.out), &report); err != nil {
				t.Fatalf("--json wrote nothing a caller can parse: %q (%v)", machine.out, err)
			}
			if report["outcome"] != contract.OutcomeRefused {
				t.Errorf("outcome: wanted %s, got %v", contract.OutcomeRefused, report["outcome"])
			}
			if report["refusal"] != c.token {
				t.Errorf("refusal: wanted %s, got %v", c.token, report["refusal"])
			}
			if report["detail"] == nil || report["detail"] == "" {
				t.Error("the machine form should name what the refusal was about")
			}
			carried, _ := report["context"].(map[string]any)
			for _, member := range c.context {
				if value, ok := carried[member].(string); !ok || value == "" {
					t.Errorf("the context should carry %s, got %v", member, report["context"])
				}
			}
		})
	}
}

// TestMalformedOverAnArgumentCarriesNoLocation asserts that the two fragments
// the file-backed cases splice on are absent where there is no file: `dinah
// add` with no title names the argument and sends nobody to edit anything.
func TestMalformedOverAnArgumentCarriesNoLocation(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "add")
	if !strings.Contains(got.errw, "title is missing, empty, or will not parse") {
		t.Errorf("the sentence should name the argument, got %q", got.errw)
	}
	for _, fragment := range []string{", in ", "hand-edit"} {
		if strings.Contains(got.errw, fragment) {
			t.Errorf("a refusal over an argument should not carry %q, got %q", fragment, got.errw)
		}
	}
}

// emptyTree returns a directory holding no workbench, with the user base
// pointed at an empty directory of its own, which is the starting point of a
// search that finds nothing.
func emptyTree(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(tree, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	return tree
}

// baseDefinition is the flow populateBase writes each workbench from, taking
// the title as its one argument. It stands in for the flow `init` builds when
// no source is named.
const baseDefinition = `{
  "profile": "dinah-core/1.0",
  "title": %q,
  "states": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Doing", "kind": "work" },
    { "id": "c00000000003", "title": "Done", "kind": "done" }
  ]
}`

// populateBase writes one workbench per slug into a base directory, and
// returns the directory each one landed in, in the order the slugs were given.
//
// Each workbench is instantiated at the directory named for it rather than
// created through `init`, which writes into a .dinah container under the
// directory it is given. The discovery these tests exercise reads a base whose
// entries are workbenches, so they need the deterministic path directly.
func populateBase(t *testing.T, base string, slugs ...string) []string {
	t.Helper()
	rooms := make([]string, 0, len(slugs))
	for i, slug := range slugs {
		name := fmt.Sprintf("d0000000000%d", i+1)
		room := filepath.Join(base, name)
		definition, err := bench.ReadDefinition([]byte(fmt.Sprintf(baseDefinition, name)))
		if err != nil {
			t.Fatalf("definition: %v", err)
		}
		if err := bench.Instantiate(room, slug, "alka", definition); err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		rooms = append(rooms, room)
	}
	return rooms
}

// sameDirs reports whether two lists name the same directories in the same
// order, asking the filesystem for identity instead of comparing spellings.
// One directory has two names on both platforms the matrix runs beyond Linux:
// macOS reaches its temporary directory through a symlink, and Windows hands
// out the short 8.3 form of a long user name, so a test that compared the path
// it built against the path the tool printed would fail over the spelling
// while the tool was right.
func sameDirs(t *testing.T, got, wanted []string) bool {
	t.Helper()
	if len(got) != len(wanted) {
		return false
	}
	for i := range got {
		mine, err := os.Stat(wanted[i])
		if err != nil {
			t.Fatalf("stat %s: %v", wanted[i], err)
		}
		theirs, err := os.Stat(got[i])
		if err != nil {
			return false
		}
		if !os.SameFile(mine, theirs) {
			return false
		}
	}
	return true
}

// ambiguousTree returns a tree whose own base holds two workbenches and whose
// user base holds none, which is the search that resolves to a choice rather
// than to a workbench.
func ambiguousTree(t *testing.T) (string, []string) {
	t.Helper()
	tree := emptyTree(t)
	return tree, populateBase(t, filepath.Join(tree, bench.UserBaseName), "one", "two")
}

// listedRows reads a listing's human form back into the paths it named, which
// is the member of a row that identifies which workbench it stands for.
func listedRows(t *testing.T, got invocation) []string {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("a listing should exit 0, got %d (%s)", got.code, got.errw)
	}
	paths := make([]string, 0, 2)
	for _, line := range strings.Split(strings.TrimRight(got.out, "\n"), "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		fields := strings.Fields(line)
		paths = append(paths, fields[len(fields)-1])
	}
	return paths
}

// jsonRows reads a listing's machine form, which is a bare array with no
// envelope, the shape `states --json` and `config --json` already emit.
func jsonRows(t *testing.T, got invocation) []bench.Candidate {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("a listing should exit 0, got %d (%s)", got.code, got.errw)
	}
	var rows []bench.Candidate
	if err := json.Unmarshal([]byte(got.out), &rows); err != nil {
		t.Fatalf("--json wrote nothing a caller can parse: %q (%v)", got.out, err)
	}
	return rows
}

// TestBareShowListsTheChoiceItCannotMake asserts the one branch of `show` this
// card changes. Several workbenches reachable and no card named leaves nothing
// to read and a choice to offer, so the choices are listed on both surfaces
// and the run succeeds.
func TestBareShowListsTheChoiceItCannotMake(t *testing.T) {
	tree, rooms := ambiguousTree(t)

	got := runCLI(t, tree, "show")
	if listed := listedRows(t, got); !sameDirs(t, listed, rooms) {
		t.Errorf("the listing should name each reachable workbench, wanted %v, got %q", rooms, got.out)
	}
	for i, slug := range []string{"one", "two"} {
		row := strings.Split(strings.TrimRight(got.out, "\n"), "\n")[i]
		if !strings.Contains(row, slug) || !strings.Contains(row, filepath.Base(rooms[i])) {
			t.Errorf("a row should carry the title and the slug, wanted %q in %q", slug, row)
		}
	}
	if got.errw != "" {
		t.Errorf("a listing should refuse nothing, got %q", got.errw)
	}

	rows := jsonRows(t, runCLI(t, tree, "--json", "show"))
	paths := make([]string, 0, len(rows))
	for i, row := range rows {
		if row.Title == "" || row.Slug == "" {
			t.Errorf("row %d should carry a title and a slug, got %+v", i+1, row)
		}
		paths = append(paths, row.Path)
	}
	if !sameDirs(t, paths, rooms) {
		t.Errorf("the machine form should carry one row per workbench, wanted %v, got %v", rooms, paths)
	}
}

// TestBareShowStillRefusesWhereThereIsNoChoice asserts the two branches this
// card leaves alone. One workbench reachable has nothing to disambiguate and
// still refuses over the card reference nobody gave, and a search that reaches
// none still refuses over the search itself.
func TestBareShowStillRefusesWhereThereIsNoChoice(t *testing.T) {
	sole := newBench(t)
	got := runCLI(t, sole, "show")
	if got.code != 2 {
		t.Fatalf("exit code: wanted 2, got %d (%s)", got.code, got.errw)
	}
	if got.errw != contract.UnknownCard+" this workbench carries no card \n" {
		t.Errorf("the refusal should be the one a single workbench has always raised, got %q", got.errw)
	}
	machine := runCLI(t, sole, "--json", "show")
	if !strings.Contains(machine.out, `"refusal": "`+contract.UnknownCard+`"`) {
		t.Errorf("the machine form should carry the same refusal, got %q", machine.out)
	}

	empty := runCLI(t, emptyTree(t), "show")
	// The walk climbs to the volume root, so on a machine whose own home
	// directory carries workbenches the search meets them before it can
	// exhaust, and the answer is honestly a different one. That case is
	// skipped rather than asserted against whatever the machine holds, which
	// is what the discovery sweep above does for the same reason.
	if empty.code == 0 {
		t.Skip("a directory above the temporary tree holds several workbenches of its own")
	}
	leading := strings.SplitN(strings.TrimSpace(empty.errw), " ", 2)[0]
	if leading == contract.AmbiguousWorkbench {
		t.Skip("a directory above the temporary tree holds several workbenches of its own")
	}
	if empty.code != 2 || leading != contract.NoWorkbenchFound {
		t.Errorf("a search that reaches nothing should still refuse, got %d %q", empty.code, empty.errw)
	}
}

// TestWorkbenchesReportsWhateverTheSearchFinds asserts that the listing
// command answers on every search: several reachable, one reachable, and none
// reachable, each exiting 0 on both surfaces. The empty answer is a line
// saying so, because a question about what is reachable is answered as
// truthfully by no rows as by two.
func TestWorkbenchesReportsWhateverTheSearchFinds(t *testing.T) {
	t.Run("several reachable", func(t *testing.T) {
		tree, rooms := ambiguousTree(t)
		if listed := listedRows(t, runCLI(t, tree, "workbenches")); !sameDirs(t, listed, rooms) {
			t.Errorf("wanted %v, got %v", rooms, listed)
		}
		if rows := jsonRows(t, runCLI(t, tree, "--json", "workbenches")); len(rows) != 2 {
			t.Errorf("wanted two rows, got %d", len(rows))
		}
	})

	t.Run("one reachable", func(t *testing.T) {
		sole := newBench(t)
		if listed := listedRows(t, runCLI(t, sole, "workbenches")); !sameDirs(t, listed, []string{benchDir(t, sole)}) {
			t.Errorf("wanted the one workbench, got %v", listed)
		}
		rows := jsonRows(t, runCLI(t, sole, "--json", "workbenches"))
		if len(rows) != 1 || rows[0].Slug != "fx" {
			t.Errorf("wanted the one workbench with its slug, got %+v", rows)
		}
	})

	t.Run("none reachable", func(t *testing.T) {
		tree := emptyTree(t)
		got := runCLI(t, tree, "workbenches")
		if got.code != 0 {
			t.Fatalf("the listing should never refuse, got %d (%s)", got.code, got.errw)
		}
		if len(listedRows(t, got)) > 0 {
			t.Skip("a directory above the temporary tree holds workbenches of its own")
		}
		if strings.TrimSpace(got.out) != msg.For(msg.Base).T("workbenches.empty") {
			t.Errorf("wanted the line that says nothing is reachable, got %q", got.out)
		}
		if rows := jsonRows(t, runCLI(t, tree, "--json", "workbenches")); len(rows) != 0 {
			t.Errorf("wanted an empty array, got %+v", rows)
		}
	})
}

// TestTheListingAndTheRefusalNameTheSameCandidates asserts that the two
// surfaces describing one ambiguity agree on membership and on order. The
// listing and the refusal read the same descriptions, so a reader choosing
// from one and a reader reading the other see the same workbenches in the same
// sequence.
func TestTheListingAndTheRefusalNameTheSameCandidates(t *testing.T) {
	tree, rooms := ambiguousTree(t)
	listing := listedRows(t, runCLI(t, tree, "workbenches"))
	shown := listedRows(t, runCLI(t, tree, "show"))
	if !reflect.DeepEqual(listing, shown) {
		t.Errorf("the two listings disagree: %v against %v", listing, shown)
	}
	refusal := runCLI(t, tree, "states")
	if refusal.code != 2 {
		t.Fatalf("a command needing one workbench should still refuse, got %d", refusal.code)
	}
	at := -1
	for _, room := range rooms {
		// The refusal names each candidate by the path the tool resolved, so
		// the directory's own name is what a test compares against a path it
		// built itself.
		found := strings.Index(refusal.errw, filepath.Base(room))
		if found < 0 {
			t.Fatalf("the refusal should name %q, got %q", room, refusal.errw)
		}
		if found < at {
			t.Errorf("the refusal orders the candidates differently from the listing, got %q", refusal.errw)
		}
		at = found
	}
	if !sameDirs(t, listing, rooms) {
		t.Errorf("the listing should carry every candidate the refusal names, wanted %v, got %v", rooms, listing)
	}
}

// TestTheListingReportsOnlyTheClosestAmbiguity asserts that the listing walks
// exactly as far as discovery walks and stops where discovery stops. A tree
// ambiguous at two levels reports the inner pair alone, because that is what a
// command run from there would have had to choose between.
func TestTheListingReportsOnlyTheClosestAmbiguity(t *testing.T) {
	tree := emptyTree(t)
	inner := filepath.Join(tree, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	populateBase(t, filepath.Join(tree, bench.UserBaseName), "farone", "fartwo")
	near := populateBase(t, filepath.Join(inner, bench.UserBaseName), "nearone", "neartwo")

	for _, argv := range [][]string{{"workbenches"}, {"show"}} {
		got := runCLI(t, inner, argv...)
		if listed := listedRows(t, got); !sameDirs(t, listed, near) {
			t.Errorf("%v: wanted the inner pair %v, got %v", argv, near, listed)
		}
		if strings.Contains(got.out, "far") {
			t.Errorf("%v: the further ambiguity should not be reported, got %q", argv, got.out)
		}
	}
}

// TestWorkbenchesHelpCarriesNoRefusals asserts that the per-command help of a
// command that never refuses prints its summary and its exit codes with no
// precondition table between them.
func TestWorkbenchesHelpCarriesNoRefusals(t *testing.T) {
	got := runCLI(t, newBench(t), "help", "workbenches")
	if got.code != 0 {
		t.Fatalf("help workbenches: %d %s", got.code, got.errw)
	}
	catalog := msg.For(msg.Base)
	for _, wanted := range []string{"workbenches", catalog.T("cmd.workbenches.summary"), catalog.T("help.exitcodes")} {
		if !strings.Contains(got.out, wanted) {
			t.Errorf("the help should carry %q, got %q", wanted, got.out)
		}
	}
	if strings.Contains(got.out, catalog.T("help.refusals")) {
		t.Errorf("a command that never refuses should list no refusals, got %q", got.out)
	}
}

// TestTheOverrideIsSpelledInFull asserts that the listing softens no caller
// mistake. An explicit --workbench naming a directory holding no workbench is
// refused exactly as every other command refuses it, and never reported as an
// empty search.
//
// The test also pins the override's spelling on both surfaces a person uses.
// --workbench and DINAH_WORKBENCH are the only names the tool answers to, and
// the retired --bench and DINAH_BENCH are gone rather than aliased, so the
// flag is refused as an unknown one and the variable is not read at all. A
// silently ignored override would point a reader at the wrong workbench and
// tell them nothing, which is the failure this asserts against.
func TestTheOverrideIsSpelledInFull(t *testing.T) {
	tree, rooms := ambiguousTree(t)

	pointed := runCLI(t, tree, "workbenches", "--workbench", rooms[1])
	if listed := listedRows(t, pointed); !sameDirs(t, listed, []string{rooms[1]}) {
		t.Errorf("an override should report the workbench it names, wanted %v, got %v", rooms[1:], listed)
	}
	wrong := runCLI(t, tree, "workbenches", "--workbench", filepath.Join(tree, "nowhere"))
	if wrong.code != 2 {
		t.Fatalf("exit code: wanted 2, got %d (%s)", wrong.code, wrong.errw)
	}
	if leading := strings.SplitN(strings.TrimSpace(wrong.errw), " ", 2)[0]; leading != contract.NoWorkbench {
		t.Errorf("leading token: wanted %s, got %q", contract.NoWorkbench, wrong.errw)
	}

	retiredFlag := "--bench" // retired spelling, named deliberately
	retired := runCLI(t, tree, "workbenches", retiredFlag, rooms[1])
	if retired.code != 2 {
		t.Fatalf("the retired flag should be refused as an unknown one, got %d (%s)", retired.code, retired.out)
	}
	if leading := strings.SplitN(strings.TrimSpace(retired.errw), " ", 2)[0]; leading != contract.Usage {
		t.Errorf("leading token: wanted %s, got %q", contract.Usage, retired.errw)
	}
	if !strings.Contains(retired.errw, retiredFlag) {
		t.Errorf("the refusal should name the flag the caller typed, got %q", retired.errw)
	}

	t.Setenv("DINAH_WORKBENCH", rooms[1])
	named := runCLI(t, tree, "workbenches")
	if listed := listedRows(t, named); !sameDirs(t, listed, []string{rooms[1]}) {
		t.Errorf("DINAH_WORKBENCH should select a workbench, wanted %v, got %v", rooms[1:], listed)
	}

	retiredVariable := "DINAH_BENCH" // retired spelling, named deliberately
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv(retiredVariable, rooms[1])
	ignored := runCLI(t, tree, "workbenches")
	if listed := listedRows(t, ignored); sameDirs(t, listed, []string{rooms[1]}) {
		t.Error("the retired variable should select nothing, and it selected a workbench")
	}
}
