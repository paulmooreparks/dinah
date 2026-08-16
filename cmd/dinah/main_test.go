package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
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
	}{
		{name: "an act that succeeded", argv: []string{"claim", "fx-1"}, code: 0, token: ""},
		{name: "a card the bench does not carry", argv: []string{"claim", "fx-99"}, code: 2, token: contract.UnknownCard},
		{name: "a card another owner holds", argv: []string{"claim", "fx-1", "--actor", "bob"}, code: 2, token: contract.Held},
		{name: "a state the bench does not declare", argv: []string{"move", "fx-1", "nowhere"}, code: 2, token: contract.UnknownState},
		{name: "a block carrying no reason", argv: []string{"block", "fx-1"}, code: 2, token: contract.NoReason},
		{name: "an unblock by another owner", argv: []string{"unblock", "fx-1", "--actor", "bob"}, code: 2, token: contract.NotOperator},
		{name: "a release by another owner", argv: []string{"release", "fx-1", "--actor", "bob"}, code: 2, token: contract.NotHolder},
		{name: "a command the binary does not offer", argv: []string{"frobnicate"}, code: 2, token: contract.UnknownVerb},
		{name: "a flag the binary does not accept", argv: []string{"--frobnicate", "status"}, code: 2, token: contract.Usage},
		{name: "a delete carrying no confirmation", argv: []string{"delete", "fx-1"}, code: 2, token: contract.Unconfirmed},
		{name: "a guide topic nothing answers to", argv: []string{"guide", "nothing"}, code: 2, token: contract.UnknownGuide},
		{name: "a setting the tool does not know", argv: []string{"config", "get", "colour"}, code: 2, token: contract.UnknownKey},
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
