package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/msg"
	"dinah/internal/profile"
	"dinah/internal/verb"
)

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
	root := filepath.Join(base, "bench")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_BENCH", "")
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

	// The block lists twenty-eight commands, and every command the binary
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
	if listed != 28 {
		t.Errorf("wanted twenty-eight listed commands, got %d", listed)
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
		{name: "a card the bench does not carry", argv: []string{"claim", "fx-99"}, code: 2, token: contract.UnknownCard, sentence: "this workbench carries no card fx-99"},
		{name: "a card another owner holds", argv: []string{"claim", "fx-1", "--actor", "bob"}, code: 2, token: contract.Held},
		{name: "a state the bench does not declare", argv: []string{"move", "fx-1", "nowhere"}, code: 2, token: contract.UnknownState, sentence: "this workbench declares no state nowhere"},
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
		{name: "an init into a directory that already holds a bench", argv: []string{"init"}, code: 2, token: contract.Exists, sentence: "already holds a workbench"},
		{name: "an extract into a directory that already holds one", argv: []string{"extract", "."}, code: 2, token: contract.Exists},
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
// The three names discovery raises before a bench is open (dinah.no-bench,
// dinah.no-bench-found and dinah.ambiguous-bench) are swept by
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
			name: "a bench designating no operator",
			build: func(t *testing.T) (string, []string) {
				root := newLimitedBench(t)
				runCLI(t, root, "add", "First")
				editAnchor(t, root, "operator: alka\n", "")
				return root, []string{"claim", "lim-1"}
			},
			token:    contract.NoOperator,
			sentence: "this workbench designates no operator, so its reserved acts are dead",
		},
		{
			name: "a bench declaring a profile major this binary does not implement",
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
	root := filepath.Join(base, "bench")
	source := filepath.Join(base, "definition.json")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_BENCH", "")
	if err := os.WriteFile(source, []byte(limitedDefinition), 0o644); err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, root, "init", "--from", source, "--slug", "lim", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	return root
}

// editAnchor rewrites the workbench anchor, which is how the cases above build
// a bench that a hand edit has put outside what the tool will serve.
func editAnchor(t *testing.T, root, from, to string) {
	t.Helper()
	path := filepath.Join(root, "workbench.md")
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

// TestCheckDeclaresItsRepairFlagsOnEverySurface asserts that the two flags
// which repair rather than report are declared once and projected everywhere:
// the ratified help block's check line names them, the generated help for the
// command names them from the same definition, and the argument parser accepts
// them. One completes an interrupted structural act, the other stamps the
// creation ordinals a workbench written before the field carries none of.
//
// The change to the fixture's check line is a ratified one rather than drift.
// The MCP head's schema is generated from the same parameter list and is
// asserted against it by TestToolSurfaceIsTheProjection.
func TestCheckDeclaresItsRepairFlagsOnEverySurface(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "help.txt"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if !strings.Contains(string(fixture), "  check [--finish] [--migrate-ordinals] ") {
		t.Error("the ratified block's check line does not name both repair flags")
	}
	if got := verb.Usage("check"); got != "check [--finish] [--migrate-ordinals]" {
		t.Errorf("the one definition composes %q", got)
	}

	root := newBench(t)
	generated := runCLI(t, root, "help", "check")
	if generated.code != 0 {
		t.Fatalf("help check: %d %s", generated.code, generated.errw)
	}
	for _, flag := range []string{"--finish", "--migrate-ordinals"} {
		if !strings.Contains(generated.out, flag) {
			t.Errorf("the generated help does not name %s:\n%s", flag, generated.out)
		}
		if got := runCLI(t, root, "check", flag); got.code != 0 {
			t.Errorf("check %s on a clean bench: %d %s", flag, got.code, got.errw)
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
	t.Setenv("DINAH_BENCH", "")
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
	for _, rung := range []string{bench.SourceFlag, bench.SourceConfig, bench.SourceVisual, bench.SourceEnvironment} {
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
			source: bench.SourceFlag,
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
				filepath.Join("bench", "workbench.md"),
				"dinah check",
			},
			context: []string{"path"},
		},
		{
			name: "a --bench pointed at a directory holding no workbench",
			build: func(t *testing.T) (string, []string) {
				root := newBench(t)
				elsewhere := t.TempDir()
				return root, []string{"status", "--bench", elsewhere}
			},
			token:   contract.NoBench,
			carries: []string{"carries no workbench.md", "point --bench at a directory that does"},
		},
		{
			name: "no workbench reachable anywhere",
			build: func(t *testing.T) (string, []string) {
				return emptyTree(t), []string{"status"}
			},
			token:   contract.NoBenchFound,
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
					if err := os.MkdirAll(room, 0o755); err != nil {
						t.Fatalf("mkdir: %v", err)
					}
					if got := runCLI(t, room, "init", "--slug", slug, "--operator", "alka"); got.code != 0 {
						t.Fatalf("init: %d %s", got.code, got.errw)
					}
				}
				return tree, []string{"status"}
			},
			token:   contract.AmbiguousBench,
			carries: []string{"are all reachable from", "choose one with --bench"},
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
			if c.token == contract.NoBenchFound && leading == contract.AmbiguousBench {
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
	t.Setenv("DINAH_BENCH", "")
	return tree
}
