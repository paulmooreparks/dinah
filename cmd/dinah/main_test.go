package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
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

// TestMain arms two records for the whole run and reports one of them after
// it. It redirects this binary's temporary directory outside the
// developer's home before any test runs, so the ancestor walk this
// package's tests exercise through the CLI cannot climb out of its own
// synthetic fixture tree and reach the real workbenches sitting above it.
// See internal/testenv's package comment for what this does and does not
// cover. It also clears the variables isolatedEnv names for the whole run,
// so a shell that exports one does not reach a test that never asked to see
// it.
func TestMain(m *testing.M) {
	restoreTemp := testenv.IsolateTempDir()
	restoreIsolated := testenv.ClearVars(isolatedEnv...)
	tableSiteRecorder = recordReachedTableSite
	code := m.Run()
	tableSiteRecorder = nil
	for _, complaint := range unreachedTableSites() {
		fmt.Fprintln(os.Stderr, complaint)
		code = 1
	}
	restoreIsolated()
	restoreTemp()
	os.Exit(code)
}

// isolatedEnv names the variables production code reads straight from the
// environment and that no test in this binary asked to see. dinah-229.
//
// windowWidth (row.go) reads COLUMNS. ResolveEditorSource
// (internal/bench/config.go) reads DINAH_EDITOR, VISUAL and EDITOR, with
// DINAH_EDITOR sitting above the config rung a fixture can write, so a
// developer who exports it draws an editor row no expectation predicts.
// osLocale (internal/bench/config.go) reads LC_ALL, LC_MESSAGES and LANG and
// feeds the lang ladder's fourth rung; the sweep does not fail on those only
// because it always passes --lang, which is a fixture accident rather than
// isolation.
//
// resolveFormat (format.go) reads DINAH_FORMAT and is the reason this list is
// eight rather than seven. Before dinah-31 the variable was harmless in a
// shell, because main.go compared it against "json" and every other value fell
// through to the human rendering unremarked. It now selects the compact form
// and refuses a value it does not recognise, so a developer who exports it
// changes what three tests that have nothing to do with it see: a plausible
// DINAH_FORMAT=compact reddens the rendering-head coverage sweep, the
// attachment-history alignment test and the missing-slug listing test, and a
// mistyped one reddens a fixture at its own init with dinah.unknown-format.
// The audience for the compact form is a driver loop, which is exactly the
// caller that exports this variable, so the reader who meets those failures
// reads a working trunk as broken.
//
// The list deliberately stops short of DINAH_ACTOR and DINAH_LANG. Fixtures
// set both on purpose, and clearing them at the binary boundary would change
// what currently-passing tests start from.
//
// It is a named variable rather than a literal at the call site so
// TestIsolatedEnvNamesEveryVariableTheBinaryClears can read it back.
var isolatedEnv = []string{
	"DINAH_FORMAT",
	"COLUMNS", "DINAH_EDITOR", "VISUAL", "EDITOR",
	"LC_ALL", "LC_MESSAGES", "LANG",
}

// TestIsolatedEnvNamesEveryVariableTheBinaryClears guards the isolation this
// binary depends on, in the two halves that fail in different places.
// dinah-229.
//
// The first half compares isolatedEnv against a copy written out here. That is
// a golden list on purpose: reading the names off the list under test would
// assert only that a slice equals itself. It catches a name dropped from
// isolatedEnv later, and it fails on any machine, including a CI runner
// exporting none of the eight.
//
// The second half asserts that TestMain actually made the call, by reading
// each name back while tests run. That one can only fail where the name is
// exported, so it is the developer's machine that arms it and no CI leg. Both
// halves are needed: without the first, a dropped name is invisible on CI;
// without the second, a list nothing acts on passes.
func TestIsolatedEnvNamesEveryVariableTheBinaryClears(t *testing.T) {
	want := []string{
		"DINAH_FORMAT",
		"COLUMNS", "DINAH_EDITOR", "VISUAL", "EDITOR",
		"LC_ALL", "LC_MESSAGES", "LANG",
	}
	for _, name := range want {
		if !namesVariable(isolatedEnv, name) {
			t.Errorf("isolatedEnv no longer names %s, so this binary inherits it from whoever runs the tests", name)
		}
	}
	for _, name := range isolatedEnv {
		if !namesVariable(want, name) {
			t.Errorf("isolatedEnv names %s, which this test does not expect: add it here with the reason, or take it out of the list", name)
		}
	}
	if len(isolatedEnv) != len(want) {
		t.Errorf("isolatedEnv carries %d names, wanted %d: %v", len(isolatedEnv), len(want), isolatedEnv)
	}

	for _, name := range want {
		if value, set := os.LookupEnv(name); set {
			t.Errorf("%s is still set to %q while tests run, so TestMain did not clear it", name, value)
		}
	}
}

// namesVariable reports whether a list of environment variable names carries
// one, which is what lets the guard above name the variable that went missing
// rather than print two slices and leave the reader to diff them.
func namesVariable(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

// invocation is one run of the head with its streams captured.
type invocation struct {
	// code is the process exit code the run returned.
	code int
	// out and errw are what the run wrote to each stream.
	out  string
	errw string
}

// resolvedDir returns the path the head itself resolves as its own working
// directory once it has chdir'd into dir: this test's own os.Getwd() call,
// made from dir, which is the exact mechanism runCLI uses and main.go's
// session captures right after. A path built by joining onto the raw
// t.TempDir() value can spell the same directory differently than what a
// running head reports for it: macOS's default temporary directory sits
// behind a symlink, and a CI-provisioned Windows temp directory can already
// be handed out as an 8.3 short name that a generic symlink-resolution pass
// would "helpfully" expand back to its long form, which is itself a second
// mismatch of the same kind. Reproducing the head's own os.Chdir/os.Getwd
// sequence, rather than guessing at whichever platform quirk explains a
// given mismatch, is what keeps this correct on every platform without
// asserting anything about which quirk is in play. dir must already exist.
func resolvedDir(t *testing.T, dir string) string {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir into %q: %v", dir, err)
	}
	defer os.Chdir(previous)
	resolved, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return resolved
}

// runCLI runs the head in a directory, with the streams captured.
func runCLI(t *testing.T, dir string, argv ...string) invocation {
	t.Helper()
	return runCLIWithInput(t, dir, strings.NewReader(""), argv...)
}

// runCLIWithInput is runCLI for the one command that reads its argument from a
// pipe. Both go through here, so run is driven from one place and the output
// check below reads both streams of every invocation any test makes.
func runCLIWithInput(t *testing.T, dir string, in io.Reader, argv ...string) invocation {
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
	code := run(argv, in, out, errw)
	checkColumnsLineUp(t, "stdout", out.String())
	checkColumnsLineUp(t, "stderr", errw.String())
	checkRefusalShape(t, code, errw.String())
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

// carryToDoing moves a card to the doing station, which is where a test that
// goes on to claim it needs the card: no owner takes work up at an intake
// column, so a claim standing there is refused.
func carryToDoing(t *testing.T, root, card string) {
	t.Helper()
	if got := runCLI(t, root, "move", card, "doing"); got.code != 0 {
		t.Fatalf("move %s to doing: %d %s", card, got.code, got.errw)
	}
}

// soleBenchDir resolves the one workbench directory a container built by
// newBench actually holds, which sits under container's own .dinah rather
// than at container itself since dinah-76. Tests that hand a workbench path
// to --workbench or to `config set workbench`, which stat the directory
// itself rather than searching it, need this rather than the container.
func soleBenchDir(t *testing.T, container string) string {
	t.Helper()
	ids := bench.ListIDs(filepath.Join(container, bench.UserBaseName))
	if len(ids) != 1 {
		t.Fatalf("wanted one workbench in the container, got %v", ids)
	}
	return filepath.Join(container, bench.UserBaseName, ids[0])
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

	// The block lists forty-one commands, and every command the binary
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
		if !blockLists(string(fixture), verb.Usage(c.name)) {
			t.Errorf("the block does not list %s", c.name)
		}
	}
	if listed != 41 {
		t.Errorf("wanted forty-one listed commands, got %d", listed)
	}
}

// blockLists reports whether the ratified help block carries a command's
// usage line under the block's own indent. The syntax column is a ceiling
// (dinah-200): a usage that fits it is followed by the padding before its
// summary, and one wider than the ceiling wraps between words, so this
// reconstructs the same wrap the renderer draws and reads it back off the
// block rather than looking for the usage as one contiguous run of text,
// which a wrapped entry never is.
//
// Since dinah-220 a continuation line can carry the summary's own
// continuation after the syntax fragment, so each chunk is matched as a
// prefix of its line rather than as the whole of it, with the same
// space-or-end-of-line rule that keeps one usage from matching another's
// prefix.
func blockLists(block, usage string) bool {
	wrapIndent := 2 + ceilingContinuationIndent
	room := halfWindow(assumedWindow)
	chunks := strings.Split(firstChunk(usage, wrapIndent, room), "\n")
	lines := strings.Split(block, "\n")
	// carries reports that a line opens with a chunk and ends the chunk at a
	// space or at the line's end. The character right after the chunk has to
	// be a space (the padding before the summary, or before the summary's
	// own continuation) or nothing at all (a line the summary has run out
	// on), so a usage that is merely a prefix of a longer one is not read as
	// a match.
	carries := func(line, chunk string) bool {
		if !strings.HasPrefix(line, chunk) {
			return false
		}
		rest := line[len(chunk):]
		return rest == "" || rest[0] == ' '
	}
	for i, line := range lines {
		if !carries(line, "  "+chunks[0]) {
			continue
		}
		matched := true
		for j := 1; j < len(chunks); j++ {
			if i+j >= len(lines) || !carries(lines[i+j], chunks[j]) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
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
	// The second card keeps the intake column occupied, which is what the
	// archive case below refuses over, while the first is carried to a
	// station so the claim cases can take it up.
	runCLI(t, root, "add", "A second card")
	carryToDoing(t, root, "fx-1")

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
		{name: "a column the workbench does not declare", argv: []string{"move", "fx-1", "nowhere"}, code: 2, token: contract.UnknownColumn, sentence: "this workbench declares no column nowhere"},
		{name: "a block carrying no reason", argv: []string{"block", "fx-1"}, code: 2, token: contract.NoReason},
		{name: "an unblock by another owner", argv: []string{"unblock", "fx-1", "--actor", "bob"}, code: 2, token: contract.NotOperator},
		{name: "a release by another owner", argv: []string{"release", "fx-1", "--actor", "bob"}, code: 2, token: contract.NotHolder},
		{name: "a command the binary does not offer", argv: []string{"frobnicate"}, code: 2, token: contract.UnknownVerb},
		{name: "a flag the binary does not accept", argv: []string{"--frobnicate", "status"}, code: 2, token: contract.Usage},
		{name: "a delete carrying no confirmation", argv: []string{"delete", "fx-1"}, code: 2, token: contract.Unconfirmed},
		{name: "a guide topic nothing answers to", argv: []string{"guide", "nothing"}, code: 2, token: contract.UnknownGuide},
		{name: "a setting the tool does not know", argv: []string{"config", "get", "colour"}, code: 2, token: contract.UnknownKey},
		{name: "a reference nothing below the card answers to", argv: []string{"path", "fx-1/nowhere"}, code: 2, token: contract.UnknownPath, sentence: "nothing in this workbench answers to"},
		{name: "an archive of a column cards occupy", argv: []string{"archive", "Intake"}, code: 2, token: contract.Occupied},
		{name: "an archive of a card the workbench does not carry", argv: []string{"archive", "fx-99"}, code: 2, token: contract.UnknownCard, sentence: "this workbench carries no card fx-99"},
		{name: "an archive of a workstream the workbench does not carry", argv: []string{"archive", "workstream/nothing"}, code: 2, token: contract.UnknownWorkstream},
		{name: "an archive of a reference nothing below the card answers to", argv: []string{"archive", "fx-1/nowhere"}, code: 2, token: contract.UnknownPath, sentence: "nothing in this workbench answers to"},
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
	if !strings.Contains(got.errw, "already carries a workbench.md") {
		t.Errorf("the refusal sentence: wanted %q in %q", "already carries a workbench.md", got.errw)
	}
	if bench.Exists(filepath.Join(root, bench.UserBaseName)) {
		t.Error("the refused init left a container behind")
	}
}

// TestInitProceedsPastAForeignWorkbenchFile asserts dinah-84's AC-2: a
// directory holding a workbench.md that carries none of Dinah's frontmatter
// keys no longer stops `init`, since init writes into a fresh container
// beside that file and never touches it. The foreign file is left
// byte-for-byte unchanged and the new bench lands in the container.
func TestInitProceedsPastAForeignWorkbenchFile(t *testing.T) {
	base := emptyTree(t)
	root := filepath.Join(base, "workbench")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	foreign := filepath.Join(root, "workbench.md")
	before := []byte("just a note, not dinah's\n")
	if err := os.WriteFile(foreign, before, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := runCLI(t, root, "init", "--slug", "other", "--operator", "alka")
	if got.code != 0 {
		t.Fatalf("init past a foreign anchor: wanted 0, got %d (%s)", got.code, got.errw)
	}
	ids := bench.ListIDs(filepath.Join(root, bench.UserBaseName))
	if len(ids) != 1 {
		t.Fatalf("the container should hold one workbench, got %v", ids)
	}
	written := filepath.Join(root, bench.UserBaseName, ids[0])
	if _, err := bench.Open(written); err != nil {
		t.Fatalf("the written workbench should open cleanly: %v", err)
	}
	after, err := os.ReadFile(foreign)
	if err != nil {
		t.Fatalf("read foreign anchor: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the foreign file should be untouched, wanted %q, got %q", before, after)
	}
}

// TestInitRefusesTheWorkbenchFlag asserts dinah-86: init refuses --workbench
// and DINAH_WORKBENCH rather than writing wherever either one names, in
// every shape the flag can reach it in (before the verb, after it, together
// with a positional root that disagrees, or from the environment alone), and
// leaves no workbench anywhere a refused run could have written one. The
// refusal names whichever of the two actually supplied the value, never the
// other.
func TestInitRefusesTheWorkbenchFlag(t *testing.T) {
	assertRefused := func(t *testing.T, base, elsewhere string, got invocation, wantSpelling string) {
		t.Helper()
		if got.code != 2 {
			t.Fatalf("exit code: wanted 2, got %d (%s)", got.code, got.out)
		}
		leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
		if leading != contract.WorkbenchNotApplicable {
			t.Errorf("leading token: wanted %s, got %q", contract.WorkbenchNotApplicable, got.errw)
		}
		if !strings.Contains(got.errw, wantSpelling) {
			t.Errorf("the refusal should name %s, got %q", wantSpelling, got.errw)
		}
		other := "DINAH_WORKBENCH"
		if wantSpelling == "DINAH_WORKBENCH" {
			other = "--workbench"
		}
		if strings.Contains(got.errw, other) {
			t.Errorf("the refusal should not name %s, got %q", other, got.errw)
		}
		if bench.Exists(filepath.Join(base, bench.UserBaseName)) {
			t.Error("a refused init left a container at the current directory")
		}
		if bench.Exists(filepath.Join(elsewhere, bench.UserBaseName)) {
			t.Error("a refused init left a container at the flag's target")
		}
	}

	t.Run("the flag before the verb", func(t *testing.T) {
		base := emptyTree(t)
		elsewhere := filepath.Join(base, "elsewhere")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		got := runCLI(t, base, "--workbench", elsewhere, "init", "--slug", "sample", "--operator", "alka")
		assertRefused(t, base, elsewhere, got, "--workbench")
	})

	t.Run("the flag after the verb", func(t *testing.T) {
		base := emptyTree(t)
		elsewhere := filepath.Join(base, "elsewhere")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		got := runCLI(t, base, "init", "--workbench", elsewhere, "--slug", "sample", "--operator", "alka")
		assertRefused(t, base, elsewhere, got, "--workbench")
	})

	t.Run("the flag with an agreeing positional root", func(t *testing.T) {
		base := emptyTree(t)
		elsewhere := filepath.Join(base, "elsewhere")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		got := runCLI(t, base, "init", elsewhere, "--workbench", elsewhere, "--operator", "alka")
		assertRefused(t, base, elsewhere, got, "--workbench")
		if bench.Exists(filepath.Join(elsewhere, bench.UserBaseName)) {
			t.Error("a refused init left a container at the positional root")
		}
	})

	t.Run("the flag with a disagreeing positional root", func(t *testing.T) {
		base := emptyTree(t)
		elsewhere := filepath.Join(base, "elsewhere")
		positional := filepath.Join(base, "positional")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.MkdirAll(positional, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		got := runCLI(t, base, "init", positional, "--workbench", elsewhere, "--operator", "alka")
		assertRefused(t, base, elsewhere, got, "--workbench")
		if bench.Exists(filepath.Join(positional, bench.UserBaseName)) {
			t.Error("a refused init left a container at the disagreeing positional root")
		}
	})

	t.Run("the environment variable alone", func(t *testing.T) {
		base := emptyTree(t)
		elsewhere := filepath.Join(base, "elsewhere")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Setenv("DINAH_WORKBENCH", elsewhere)
		got := runCLI(t, base, "init", "--slug", "sample", "--operator", "alka")
		assertRefused(t, base, elsewhere, got, "DINAH_WORKBENCH")
	})

	t.Run("the environment variable and the flag together names the flag", func(t *testing.T) {
		base := emptyTree(t)
		flagTarget := filepath.Join(base, "flag-target")
		envTarget := filepath.Join(base, "env-target")
		if err := os.MkdirAll(flagTarget, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.MkdirAll(envTarget, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Setenv("DINAH_WORKBENCH", envTarget)
		got := runCLI(t, base, "--workbench", flagTarget, "init", "--slug", "sample", "--operator", "alka")
		assertRefused(t, base, flagTarget, got, "--workbench")
		if bench.Exists(filepath.Join(envTarget, bench.UserBaseName)) {
			t.Error("a refused init left a container at the environment's target")
		}
	})

	t.Run("neither set still creates at the working directory, unchanged", func(t *testing.T) {
		base := emptyTree(t)
		root := filepath.Join(base, "plain")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		got := runCLI(t, root, "init", "--operator", "alka")
		if got.code != 0 {
			t.Fatalf("init with neither set: wanted 0, got %d (%s)", got.code, got.errw)
		}
		ids := bench.ListIDs(filepath.Join(root, bench.UserBaseName))
		if len(ids) != 1 {
			t.Fatalf("the container should hold one workbench, got %v", ids)
		}
	})
}

// TestInitStillHonoursThePositionalRootAlone asserts dinah-86's AC-2: a
// positional root argument, with neither --workbench nor DINAH_WORKBENCH
// set, still creates the workbench at that positional root exactly as
// before this card, since the flag refusal above must not swallow the
// plain positional case it sits beside.
func TestInitStillHonoursThePositionalRootAlone(t *testing.T) {
	base := emptyTree(t)
	root := filepath.Join(base, "target")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got := runCLI(t, base, "init", root, "--operator", "alka")
	if got.code != 0 {
		t.Fatalf("init with a positional root alone: wanted 0, got %d (%s)", got.code, got.errw)
	}
	ids := bench.ListIDs(filepath.Join(root, bench.UserBaseName))
	if len(ids) != 1 {
		t.Fatalf("the container should hold one workbench at the positional root, got %v", ids)
	}
	if bench.Exists(filepath.Join(base, bench.UserBaseName)) {
		t.Error("init with a positional root should not also write a container at the working directory")
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
		"Create a workbench here, optionally from a template",
		"no workbench.md file sits at this exact path",
		contract.Exists,
		"the source definition carries what the profile requires",
	} {
		if !strings.Contains(got.out, carried) {
			t.Errorf("the help should carry %q, got %q", carried, got.out)
		}
	}
}

// TestExtractStillRefusesOnAForeignWorkbenchFileAndLeavesItAlone asserts
// dinah-84's AC-4: unlike init, extract keeps its bare existence check.
// Extract overwrites whatever sits at the target path, so a foreign
// workbench.md there is exactly as much at risk as a real one, and the
// refusal (and the file's survival) does not depend on frontmatter
// recognition. This is the regression guard for the "no silent loss"
// requirement: it must keep passing since Extract itself does not change.
func TestExtractStillRefusesOnAForeignWorkbenchFileAndLeavesItAlone(t *testing.T) {
	root := newBench(t)
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	foreign := filepath.Join(target, "workbench.md")
	before := []byte("an unrelated document, not dinah's\n")
	if err := os.WriteFile(foreign, before, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := runCLI(t, root, "extract", target)
	if got.code != 2 {
		t.Fatalf("exit code: wanted 2, got %d (%s)", got.code, got.out)
	}
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.Exists {
		t.Errorf("leading token: wanted %s, got %q", contract.Exists, got.errw)
	}
	after, err := os.ReadFile(foreign)
	if err != nil {
		t.Fatalf("read foreign anchor: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the foreign file should survive a refused extract, wanted %q, got %q", before, after)
	}
}

// TestInitRefusesOnAnUnreadableAnchor asserts dinah-84's AC-5: a
// workbench.md that exists at init's target root but cannot be read refuses
// with contract.UnreadableBench rather than contract.Exists. The read is
// forced to fail by making workbench.md a directory rather than a file,
// which every platform this tool runs on refuses to read as text, so the
// failure is provoked the same way on Windows as everywhere else without
// depending on permission bits (see readAnchorContent's own comment).
func TestInitRefusesOnAnUnreadableAnchor(t *testing.T) {
	base := emptyTree(t)
	root := filepath.Join(base, "workbench")
	if err := os.MkdirAll(filepath.Join(root, "workbench.md"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := runCLI(t, root, "init", "--slug", "other", "--operator", "alka")
	if got.code != 2 {
		t.Fatalf("exit code: wanted 2, got %d (%s)", got.code, got.out)
	}
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.UnreadableBench {
		t.Errorf("leading token: wanted %s, got %q", contract.UnreadableBench, got.errw)
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
		{"columns"},
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
		t.Errorf("wanted the Devanagari rendering of the blocked state, got %q", got.out)
	}
	if strings.ContainsRune(got.out, '�') {
		t.Error("the output carries a replacement character")
	}
	refused := runCLI(t, root, "claim", "fx-1", "--lang", "hi")
	leading := strings.SplitN(strings.TrimSpace(refused.errw), " ", 2)[0]
	if leading != contract.Blocked {
		t.Errorf("the refusal name should keep its canonical spelling under any language, got %q", refused.errw)
	}

	// The line init prints when it records an actor is a catalog entry like
	// any other, so a Hindi reader gets it in Devanagari with the product
	// name still in Latin script. This subtest builds a home of its own,
	// because newBench above resolves an actor and init records nothing
	// when one is already resolvable.
	t.Run("the line init prints when it records an actor", func(t *testing.T) {
		home, base := noActorHome(t)
		room := newRoom(t, base)

		created := runCLI(t, room, "--lang", "hi", "init", "--operator", "paul")
		if created.code != 0 {
			t.Fatalf("init: %d %s", created.code, created.errw)
		}
		lines := strings.Split(strings.TrimSuffix(created.out, "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("init should print the created line and the recorded line, got %q", created.out)
		}
		announcement := lines[1]
		if !strings.Contains(announcement, "दर्ज") {
			t.Errorf("wanted the Devanagari rendering of the recorded line, got %q", announcement)
		}
		if !strings.Contains(announcement, "Dinah") {
			t.Errorf("the product name should stay in Latin script, got %q", announcement)
		}
		if !strings.Contains(announcement, "paul") || !strings.Contains(announcement, configPath(home)) {
			t.Errorf("the recorded line should name the actor and the file, got %q", announcement)
		}
		if strings.ContainsRune(created.out, '�') {
			t.Error("the output carries a replacement character")
		}

		identity := runCLI(t, room, "--lang", "hi", "whoami")
		if identity.code != 0 {
			t.Fatalf("whoami: %d %s", identity.code, identity.errw)
		}
		if !strings.Contains(identity.out, "संचालक") {
			t.Errorf("wanted the Devanagari rendering of whoami, got %q", identity.out)
		}
	})
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
	// The per-command refusal sentences, whose keys refusalSentence composes
	// from a refusal name and the verb that raised it, so the literal scan
	// below cannot see them.
	//
	// A composed key is optional, since a command that adds none falls back to
	// the bare key, so what is asserted is the invariant that holds whether or
	// not one exists: a per-command sentence never stands alone. The bare key
	// is what every other command renders, and a per-command entry written
	// where none exists would leave all of them printing refusal.unknown.
	refusals := map[string]bool{}
	for _, name := range append(append([]string{}, contract.Declared...), contract.Introduced...) {
		refusals[name] = true
	}
	composed := 0
	for _, name := range verb.Commands() {
		for refusal := range refusals {
			key := "refusal." + refusal + "." + name
			if !known[key] {
				continue
			}
			composed++
			if !known["refusal."+refusal] {
				t.Errorf("the base catalog carries %s and no refusal.%s under it, so every other command raising %s renders a bare key", key, refusal, refusal)
			}
		}
	}
	if composed == 0 {
		t.Error("the base catalog carries no per-command refusal sentence, so this loop proves nothing")
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
  "profile": "dinah-core/0.7",
  "title": "Limited",
  "columns": [
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
			name: "a forward move out of a done column",
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
			token:    contract.NoOwner,
			sentence: "Dinah does not know who you are; run `dinah config set actor <name>` to say so",
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
				editAnchor(t, root, "profile: "+bench.ProfileVersion, "profile: dinah-core/9.0")
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

// guideInvocation matches a dinah command line wherever it stands.
//
// The pattern reaches inside a line rather than anchoring at the start of one,
// because the corpus now carries the quick start, whose command lines open
// `$ ` and whose prose quotes an invocation between backticks. Two blocks also
// reach a dinah invocation from inside another command, where the reader is
// shown an editor opened on the path dinah prints, and that is the only place
// the documentation teaches that reference grammar.
//
// What may stand in front of an invocation is written out rather than left
// open, and the narrowing is measured. A pattern reading the word anywhere
// takes "you installed dinah to /home/ana/.local/bin" as the command `to` and
// "run this now to use dinah in this shell" as the command `in`, both of them
// lines the installer prints. So an invocation opens a line, opens one after
// the `$ ` a transcript marks a command line with, or stands just after a
// backtick, a command substitution, or an opening parenthesis.
//
// The tail stops at a backtick, so a sentence quoting two invocations yields
// both and prose standing after a closing backtick is not read as arguments.
var guideInvocation = regexp.MustCompile("(?m)(?:^\\$ |^|`|\\$\\(|\\()dinah ([a-z_]+)([^\n`]*)")

// guideFlag matches a long flag inside such a line.
var guideFlag = regexp.MustCompile(`--([a-z-]+)`)

// TestTheGuidesTeachOnlyDeclaredFlags asserts the reference rule against every
// document that teaches the tool: every command it teaches exists, and every
// long flag it spells is one that command declares or one of the global flags.
//
// The corpus is the shipped guides and docs/quick-start.md. The quick start is
// where the drift this rule exists for actually landed, and it sat outside
// this test's reach until it was named here.
func TestTheGuidesTeachOnlyDeclaredFlags(t *testing.T) {
	global := map[string]bool{}
	for _, flag := range globalFlags {
		global[flag.name] = true
	}
	checked := 0
	for _, document := range guardedDocuments(t) {
		for _, invocation := range guideInvocation.FindAllStringSubmatch(document.text, -1) {
			name, rest := invocation[1], invocation[2]
			if _, ok := lookup(name); !ok {
				t.Errorf("%s: the document teaches the command %q, which the binary does not offer", document.name, name)
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
				t.Errorf("%s: the document teaches `dinah %s --%s`, which %s does not declare", document.name, name, flag[1], name)
			}
		}
	}
	if checked == 0 {
		t.Error("no flagged invocation was found in any guide or in the quick start, so this test proves nothing")
	}
}

// TestCheckDeclaresItsRepairFlagsOnEverySurface asserts that the three flags
// which repair rather than report are declared once and projected everywhere:
// the ratified help block's check line names them, the generated help for the
// command names them from the same definition, and the argument parser accepts
// them. One completes an interrupted structural act, one stamps the creation
// ordinals a workbench written before the field carries none of, one derives
// the slugs of columns and workstreams written before that field existed, one
// removes the stranded identifiers from the columns list, and one creates a
// workstream at every membership the live cards carry that names none.
//
// The change to the fixture's check line is a ratified one rather than drift.
// The MCP head's schema is generated from the same parameter list and is
// asserted against it by TestToolSurfaceIsTheProjection.
func TestCheckDeclaresItsRepairFlagsOnEverySurface(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "help.txt"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	const line = "check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-columns] [--migrate-vocabulary] [--migrate-workstreams] [--witness] [--yes]"
	if !blockLists(string(fixture), line) {
		t.Error("the ratified block's check line does not name every repair flag")
	}
	if got := verb.Usage("check"); got != line {
		t.Errorf("the one definition composes %q", got)
	}

	root := newBench(t)
	generated := runCLI(t, root, "help", "check")
	if generated.code != 0 {
		t.Fatalf("help check: %d %s", generated.code, generated.errw)
	}
	for _, flag := range []string{"--finish", "--migrate-ordinals", "--migrate-slugs", "--migrate-columns", "--migrate-vocabulary", "--migrate-workstreams", "--witness"} {
		if !strings.Contains(generated.out, flag) {
			t.Errorf("the generated help does not name %s:\n%s", flag, generated.out)
		}
		if got := runCLI(t, root, "check", flag); got.code != 0 {
			t.Errorf("check %s on a clean workbench: %d %s", flag, got.code, got.errw)
		}
		// dinah-346: every flag reaches the same exit-code site, so the
		// clean case is asserted for each of them on the machine head too,
		// where the outcome member says the same thing the code does.
		machine := runCLI(t, root, "--json", "check", flag)
		if machine.code != contract.ExitCodeForRead(contract.ReadOK) {
			t.Errorf("check --json %s on a clean workbench exited %d, wanted %d:\n%s", flag, machine.code, contract.ExitCodeForRead(contract.ReadOK), machine.out)
		}
		var carried struct {
			Outcome string `json:"outcome"`
		}
		if err := json.Unmarshal([]byte(machine.out), &carried); err != nil {
			t.Fatalf("decode check --json %s: %v\n%s", flag, err, machine.out)
		}
		if carried.Outcome != contract.ReadOK {
			t.Errorf("check --json %s on a clean workbench reports outcome %q, wanted %q", flag, carried.Outcome, contract.ReadOK)
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

// TestConfigSetWorkbenchStoresAnAbsolutePathAndClears asserts dinah-70's
// write-time behaviour: a relative path handed to `config set workbench`
// is stored as the absolute path it named from the directory the command
// ran in, and a bare `config set workbench` with no value clears it, the
// same as every other key.
func TestConfigSetWorkbenchStoresAnAbsolutePathAndClears(t *testing.T) {
	_, dir := settingsHome(t)
	target := filepath.Join(dir, "elsewhere")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Resolved after creation, and after the mkdir above, so it matches the
	// path the head resolves through its own working directory rather than
	// the raw value t.TempDir() handed this test (see resolvedDir).
	wantTarget := resolvedDir(t, target)

	if got := runCLI(t, dir, "config", "set", "workbench", "elsewhere"); got.code != 0 {
		t.Fatalf("config set: %d %s", got.code, got.errw)
	}
	stored := runCLI(t, dir, "config", "get", "workbench")
	if stored.code != 0 {
		t.Fatalf("config get: %d %s", stored.code, stored.errw)
	}
	if got := strings.TrimSpace(stored.out); got != wantTarget {
		t.Errorf("a relative path should be stored absolute, wanted %q, got %q", wantTarget, got)
	}

	if got := runCLI(t, dir, "config", "set", "workbench"); got.code != 0 {
		t.Fatalf("config set with no value: %d %s", got.code, got.errw)
	}
	cleared := runCLI(t, dir, "config", "get", "workbench")
	if cleared.code != 0 {
		t.Fatalf("config get: %d %s", cleared.code, cleared.errw)
	}
	if got := strings.TrimSpace(cleared.out); got != "" {
		t.Errorf("clearing the setting: wanted an empty value, got %q", got)
	}
}

// TestConfiguredWorkbenchAnswersOnlyWhereDiscoveryRefuses asserts dinah-70's
// placement end to end, through the head rather than the library: the
// configured default opens a workbench standing anywhere on the filesystem
// when nothing local is reachable, a local workbench always wins over it, an
// ambiguous base still refuses with the configured default sitting there
// unconsulted, and a configured path gone stale refuses by its own name
// instead of falling through to the generic exhausted-search refusal.
func TestConfiguredWorkbenchAnswersOnlyWhereDiscoveryRefuses(t *testing.T) {
	t.Run("opens the configured workbench when nothing local is reachable", func(t *testing.T) {
		here := emptyTree(t)
		elsewhere := filepath.Join(t.TempDir(), "elsewhere")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		definition, err := bench.ReadDefinition([]byte(fmt.Sprintf(baseDefinition, "Configured")))
		if err != nil {
			t.Fatalf("definition: %v", err)
		}
		if err := bench.Instantiate(elsewhere, "cf", "alka", definition); err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if got := runCLI(t, here, "config", "set", "workbench", elsewhere); got.code != 0 {
			t.Fatalf("config set: %d %s", got.code, got.errw)
		}
		reported := runCLI(t, here, "status")
		if reported.code != 0 {
			t.Fatalf("status: %d %s", reported.code, reported.errw)
		}
		if !strings.Contains(reported.out, "Configured") {
			t.Errorf("status should open the configured workbench, got:\n%s", reported.out)
		}
	})

	t.Run("a local workbench wins over a workbench configured elsewhere", func(t *testing.T) {
		root := newBench(t)
		elsewhere := filepath.Join(t.TempDir(), "elsewhere")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		definition, err := bench.ReadDefinition([]byte(fmt.Sprintf(baseDefinition, "Elsewhere")))
		if err != nil {
			t.Fatalf("definition: %v", err)
		}
		if err := bench.Instantiate(elsewhere, "el", "alka", definition); err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if got := runCLI(t, root, "config", "set", "workbench", elsewhere); got.code != 0 {
			t.Fatalf("config set: %d %s", got.code, got.errw)
		}
		reported := runCLI(t, root, "status")
		if reported.code != 0 {
			t.Fatalf("status: %d %s", reported.code, reported.errw)
		}
		if strings.Contains(reported.out, "Elsewhere") {
			t.Errorf("the local workbench should win over the configured default, got:\n%s", reported.out)
		}
	})

	t.Run("an ambiguous base still refuses with a workbench configured", func(t *testing.T) {
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
		elsewhere := filepath.Join(t.TempDir(), "elsewhere")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		definition, err := bench.ReadDefinition([]byte(fmt.Sprintf(baseDefinition, "Configured")))
		if err != nil {
			t.Fatalf("definition: %v", err)
		}
		if err := bench.Instantiate(elsewhere, "cf", "alka", definition); err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		if got := runCLI(t, tree, "config", "set", "workbench", elsewhere); got.code != 0 {
			t.Fatalf("config set: %d %s", got.code, got.errw)
		}
		reported := runCLI(t, tree, "status")
		if reported.code != 2 {
			t.Fatalf("an ambiguous base with a workbench configured: wanted 2, got %d (%s)", reported.code, reported.out)
		}
		leading := strings.SplitN(strings.TrimSpace(reported.errw), " ", 2)[0]
		if leading != contract.AmbiguousWorkbench {
			t.Errorf("leading token: wanted %s, got %q (the configured default should not break the tie)", contract.AmbiguousWorkbench, reported.errw)
		}
	})

	t.Run("a stale configured workbench refuses by its own name", func(t *testing.T) {
		here := emptyTree(t)
		gone := filepath.Join(t.TempDir(), "gone")
		if err := os.MkdirAll(gone, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if got := runCLI(t, here, "config", "set", "workbench", gone); got.code != 0 {
			t.Fatalf("config set: %d %s", got.code, got.errw)
		}
		reported := runCLI(t, here, "status")
		if reported.code != 2 {
			t.Fatalf("a stale configured default: wanted 2, got %d (%s)", reported.code, reported.out)
		}
		leading := strings.SplitN(strings.TrimSpace(reported.errw), " ", 2)[0]
		if leading != contract.NoConfiguredWorkbench {
			t.Errorf("leading token: wanted %s, got %q (a fall-through to %s would be the silent bug this guards against)", contract.NoConfiguredWorkbench, reported.errw, contract.NoWorkbenchFound)
		}
		if !strings.Contains(reported.errw, gone) {
			t.Errorf("the refusal should name the configured path, got %q", reported.errw)
		}
		if !strings.Contains(reported.errw, "config set workbench") {
			t.Errorf("the refusal should say how to fix it, got %q", reported.errw)
		}
	})
}

// TestStatusNamesTheRungThatAnswered asserts dinah-70's own open question:
// `dinah status` names which rung resolved the active workbench, on both the
// rendered line and the --json form, and the rendered word goes through the
// message catalog rather than a raw internal token.
func TestStatusNamesTheRungThatAnswered(t *testing.T) {
	t.Run("search", func(t *testing.T) {
		root := newBench(t)
		reported := runCLI(t, root, "status")
		if !strings.Contains(reported.out, "[search]") {
			t.Errorf("the rendered line should name the search rung, got:\n%s", reported.out)
		}
		machine := runCLI(t, root, "--json", "status")
		var status verb.Status
		if err := json.Unmarshal([]byte(machine.out), &status); err != nil {
			t.Fatalf("the machine form should parse: %v\n%s", err, machine.out)
		}
		if status.WorkbenchSource != bench.SourceSearch {
			t.Errorf("workbench_source: wanted %s, got %q", bench.SourceSearch, status.WorkbenchSource)
		}
		// The new rung goes through the message catalog like every other
		// one, rather than leaking a raw internal token when the reader's
		// language is not English.
		hindi := runCLI(t, root, "--lang", "hi", "status")
		if hindi.code != 0 {
			t.Fatalf("status --lang hi: %d %s", hindi.code, hindi.errw)
		}
		if !strings.Contains(hindi.out, "[खोज]") {
			t.Errorf("the search rung's Hindi rendering should reach the status line, got:\n%s", hindi.out)
		}
	})

	t.Run("flag", func(t *testing.T) {
		actual := soleBenchDir(t, newBench(t))
		reported := runCLI(t, t.TempDir(), "status", "--workbench", actual)
		if !strings.Contains(reported.out, "[flag]") {
			t.Errorf("the rendered line should name the flag rung, got:\n%s", reported.out)
		}
	})

	t.Run("config, and rendered in another language rather than the raw token", func(t *testing.T) {
		here := emptyTree(t)
		configured := soleBenchDir(t, newBench(t))
		if got := runCLI(t, here, "config", "set", "workbench", configured); got.code != 0 {
			t.Fatalf("config set: %d %s", got.code, got.errw)
		}
		reported := runCLI(t, here, "status")
		if !strings.Contains(reported.out, "[config]") {
			t.Errorf("the rendered line should name the config rung, got:\n%s", reported.out)
		}
		machine := runCLI(t, here, "--json", "status")
		var status verb.Status
		if err := json.Unmarshal([]byte(machine.out), &status); err != nil {
			t.Fatalf("the machine form should parse: %v\n%s", err, machine.out)
		}
		if status.WorkbenchSource != bench.SourceConfig {
			t.Errorf("workbench_source: wanted %s, got %q", bench.SourceConfig, status.WorkbenchSource)
		}

		hindi := runCLI(t, here, "--lang", "hi", "status")
		if hindi.code != 0 {
			t.Fatalf("status --lang hi: %d %s", hindi.code, hindi.errw)
		}
		if !strings.Contains(hindi.out, "[कॉन्फ़िग]") {
			t.Errorf("the config rung's Hindi rendering should reach the status line, got:\n%s", hindi.out)
		}
	})
}

// TestConfigListingReportsTheWorkbenchRow asserts AC-10's own shape: the
// listing's workbench row carries the same resolved value and rung a real
// status would open, and an empty value with the unset source when nothing
// answers, matching the actor row's existing convention.
func TestConfigListingReportsTheWorkbenchRow(t *testing.T) {
	t.Run("nothing answers", func(t *testing.T) {
		here := emptyTree(t)
		rows := settingRows(t, runCLI(t, here, "--json", "config"))
		row, ok := rows["workbench"]
		if !ok {
			t.Fatalf("the listing drops the workbench key")
		}
		if row.Value != "" || row.Source != bench.SourceUnset {
			t.Errorf("nothing reachable and nothing configured: wanted an empty value at %s, got %+v", bench.SourceUnset, row)
		}
	})

	t.Run("a configured default answers", func(t *testing.T) {
		here := emptyTree(t)
		target := soleBenchDir(t, newBench(t))
		if got := runCLI(t, here, "config", "set", "workbench", target); got.code != 0 {
			t.Fatalf("config set: %d %s", got.code, got.errw)
		}
		rows := settingRows(t, runCLI(t, here, "--json", "config"))
		row := rows["workbench"]
		if row.Value != target || row.Source != bench.SourceConfig {
			t.Errorf("wanted %q at %s, got %+v", target, bench.SourceConfig, row)
		}
	})

	t.Run("a local workbench answers ahead of a configured default", func(t *testing.T) {
		container := newBench(t)
		// Resolved to match what the head's own search reports for the same
		// directory once it has chdir'd into it (see resolvedDir).
		actual := resolvedDir(t, soleBenchDir(t, container))
		elsewhere := filepath.Join(t.TempDir(), "elsewhere")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if got := runCLI(t, container, "config", "set", "workbench", elsewhere); got.code != 0 {
			t.Fatalf("config set: %d %s", got.code, got.errw)
		}
		rows := settingRows(t, runCLI(t, container, "--json", "config"))
		row := rows["workbench"]
		if row.Value != actual || row.Source != bench.SourceSearch {
			t.Errorf("wanted the local workbench %q at %s, got %+v", actual, bench.SourceSearch, row)
		}
	})
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
	// dinah-346: the migration stamped the ordinal, so the checker that runs
	// after it finds nothing, and the guess is the whole report's only
	// finding. That makes this the case where an outcome computed before the
	// migration branch appended would call the run clean, so the exit code is
	// asserted here as well as the sentence.
	if migrated.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Errorf("a migration reporting a guess exited %d, wanted %d:\n%s", migrated.code, contract.ExitCodeForRead(contract.ReadFindings), migrated.out)
	}
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
	if machine.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Errorf("the JSON migration exited %d, wanted %d:\n%s", machine.code, contract.ExitCodeForRead(contract.ReadFindings), machine.out)
	}
	reported := verb.CheckReport{}
	if err := json.Unmarshal([]byte(machine.out), &reported); err != nil {
		t.Fatalf("read the machine form: %v\n%s", err, machine.out)
	}
	if reported.Outcome != contract.ReadFindings {
		t.Errorf("the machine form reports outcome %q over %d findings, wanted %q", reported.Outcome, len(reported.Findings), contract.ReadFindings)
	}
	if !strings.Contains(machine.out, bench.FindingOrdinalGuessed) || !strings.Contains(machine.out, "e00000000002") {
		t.Errorf("the machine form does not name the entity it guessed at:\n%s", machine.out)
	}
}

// TestColumnsCarryTheirSlugOnBothSurfaces asserts that the slug reaches every
// surface a caller reads a column through: the human listing prints it beside
// the identifier, the machine form carries it as a member of each column
// object, and a reference typed as the slug reaches the column without the
// quoting a spaced title needs.
func TestColumnsCarryTheirSlugOnBothSurfaces(t *testing.T) {
	root := newBench(t)

	human := runCLI(t, root, "columns")
	if human.code != 0 {
		t.Fatalf("columns: %d %s", human.code, human.errw)
	}
	for _, slug := range []string{"intake", "doing", "done"} {
		if !strings.Contains(human.out, slug) {
			t.Errorf("the listing does not print the slug %s:\n%s", slug, human.out)
		}
	}

	machine := runCLI(t, root, "--json", "columns")
	if machine.code != 0 {
		t.Fatalf("columns --json: %d %s", machine.code, machine.errw)
	}
	var columns []verb.ColumnView
	if err := json.Unmarshal([]byte(machine.out), &columns); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	if len(columns) != 3 {
		t.Fatalf("the machine form carries %d columns", len(columns))
	}
	for position, wanted := range []string{"intake", "doing", "done"} {
		if got := columns[position].Slug; got != wanted {
			t.Errorf("column %d carries slug %q, wanted %q", position+1, got, wanted)
		}
	}

	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	listed := runCLI(t, root, "ls", "--column", "intake")
	if listed.code != 0 {
		t.Fatalf("ls by slug: %d %s", listed.code, listed.errw)
	}
	if !strings.Contains(listed.out, "A card") {
		t.Errorf("a slug should name a column on the command line:\n%s", listed.out)
	}
}

// TestColumnsRenderNamesTheRepairInsteadOfPaddingBlank asserts AC-3: a column
// with no slug prints a catalog-served placeholder naming the repair, not an
// equal-width run of spaces indistinguishable from a rendering glitch.
func TestColumnsRenderNamesTheRepairInsteadOfPaddingBlank(t *testing.T) {
	root := newBench(t)
	stripSlugs(t, root)
	got := runCLI(t, root, "columns")
	if got.code != 0 {
		t.Fatalf("columns: %d %s", got.code, got.errw)
	}
	if !strings.Contains(got.out, "no slug") || !strings.Contains(got.out, "migrate-slugs") {
		t.Errorf("the listing should name the repair for a column with no slug:\n%s", got.out)
	}
	for _, title := range []string{"Intake", "Doing", "Done"} {
		if !strings.Contains(got.out, title) {
			t.Errorf("the listing should still carry column %s:\n%s", title, got.out)
		}
	}
}

// TestWorkbenchesRenderNamesTheRepairInsteadOfPaddingBlank asserts AC-4: a
// workbench with no slug prints the same placeholder convention in the human
// listing, once the workbench-slug migration exists to point at, and the
// machine form omits the key entirely rather than serving an empty string.
func TestWorkbenchesRenderNamesTheRepairInsteadOfPaddingBlank(t *testing.T) {
	root := newBench(t)
	editAnchor(t, root, "slug: fx\n", "")

	human := runCLI(t, root, "workbenches")
	if human.code != 0 {
		t.Fatalf("workbenches: %d %s", human.code, human.errw)
	}
	if !strings.Contains(human.out, "no slug") || !strings.Contains(human.out, "migrate-slugs") {
		t.Errorf("the listing should name the repair for a workbench with no slug:\n%s", human.out)
	}

	machine := runCLI(t, root, "--json", "workbenches")
	if machine.code != 0 {
		t.Fatalf("workbenches --json: %d %s", machine.code, machine.errw)
	}
	if strings.Contains(machine.out, `"slug"`) {
		t.Errorf("the machine form should omit the key for a workbench with none:\n%s", machine.out)
	}

	repaired := runCLI(t, root, "check", "--migrate-slugs")
	if repaired.code != 0 {
		t.Fatalf("check --migrate-slugs: %d %s", repaired.code, repaired.errw)
	}
	// The workbench's title is "workbench", the base name of the directory
	// newBench built it in, since the migration derives from the title
	// rather than remembering the --slug value the anchor lost.
	if !strings.Contains(repaired.out, "Assigned the workbench slug workbench.") {
		t.Errorf("the migration did not say what it derived for the workbench:\n%s", repaired.out)
	}
	again := runCLI(t, root, "workbenches")
	if again.code != 0 {
		t.Fatalf("workbenches after repair: %d %s", again.code, again.errw)
	}
	if strings.Contains(again.out, "no slug") {
		t.Errorf("the repaired workbench should print its own slug, not the placeholder:\n%s", again.out)
	}
	if !strings.Contains(again.out, "workbench") {
		t.Errorf("the repaired workbench should carry its derived slug:\n%s", again.out)
	}
}

// TestTheSlugMigrationRepairsAWorkbenchWrittenBeforeTheField asserts the
// one-time repair end to end: the checker names each column carrying no slug,
// the repair derives one from the title and says which column got which slug on
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
	if !strings.Contains(migrated.out, "Assigned 3 column slugs.") {
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

// TestCheckMigrateColumnsNamesWhatItRemovedOnTheTerminal asserts the terminal
// rendering of the stranded-column repair, not just the internal function it
// calls: a clean check and a real repair must not print the same line, and
// the repair must say which column identifier it took out of the list.
//
// dinah check --migrate-columns edits workbench.md whether or not the caller
// is told, so this is the one place that confirms the edit is also reported;
// a change to the internal repair function alone would leave this test
// passing or failing on its own, with no dependency on the renderer at all.
func TestCheckMigrateColumnsNamesWhatItRemovedOnTheTerminal(t *testing.T) {
	root := newBench(t)
	gone := strandColumn(t, root, 2)

	clean := runCLI(t, root, "check")
	if clean.code == 0 {
		t.Fatalf("a stranded column should be reported, not pass clean")
	}
	if !strings.Contains(clean.out, gone) {
		t.Errorf("the checker did not name the stranded column:\n%s", clean.out)
	}

	migrated := runCLI(t, root, "check", "--migrate-columns")
	if migrated.code != 0 {
		t.Fatalf("check --migrate-columns: %d %s", migrated.code, migrated.errw)
	}
	if !strings.Contains(migrated.out, "Removed 1 stranded column") {
		t.Errorf("the migration did not say how many columns it removed:\n%s", migrated.out)
	}
	if !strings.Contains(migrated.out, gone) {
		t.Errorf("the migration did not name the column it removed:\n%s", migrated.out)
	}

	again := runCLI(t, root, "check", "--migrate-columns")
	if again.code != 0 {
		t.Fatalf("a second run: %d %s", again.code, again.errw)
	}
	if strings.Contains(again.out, gone) {
		t.Errorf("a second run should have nothing left to name:\n%s", again.out)
	}
	if !strings.Contains(again.out, "Removed 0 stranded columns") {
		t.Errorf("a second run did not say it removed nothing:\n%s", again.out)
	}
}

// strandColumn hand-strands one column of a workbench the way retirement's own
// pre-fix defect used to: it removes the column's directory without touching
// workbench.md's columns list, and returns the identifier left dangling.
func strandColumn(t *testing.T, root string, position int) string {
	t.Helper()
	machine := runCLI(t, root, "--json", "columns")
	var columns []verb.ColumnView
	if err := json.Unmarshal([]byte(machine.out), &columns); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	if position >= len(columns) {
		t.Fatalf("the workbench carries %d columns", len(columns))
	}
	id := columns[position].ID
	dir := filepath.Join(benchDir(t, root), bench.ColumnsDir, id)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove %s: %v", dir, err)
	}
	return id
}

// strandAllColumns hand-strands every column a fresh newBench declares, one at
// a time: each call to strandColumn re-reads the live list, which shrinks by
// one member as its predecessor is stranded, so position 0 always names
// whichever column is still live. It returns the stranded identifiers in the
// order they were stranded, leaving workbench.md's raw columns list
// unchanged and every id on it stranded, which is the shape a real
// workbench reaches after every other column was already retired and this
// last one was retired or removed under the pre-dinah-49 code.
func strandAllColumns(t *testing.T, root string) []string {
	t.Helper()
	machine := runCLI(t, root, "--json", "columns")
	var columns []verb.ColumnView
	if err := json.Unmarshal([]byte(machine.out), &columns); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	gone := make([]string, 0, len(columns))
	for range columns {
		gone = append(gone, strandColumn(t, root, 0))
	}
	return gone
}

// TestCheckMigrateColumnsRefusesRatherThanEmptyingTheDefinition asserts AC-2:
// dinah check --migrate-columns against a workbench whose columns list is
// entirely stranded ids exits 2 with the new refusal, leaves workbench.md
// unchanged, and a following plain dinah check reports the same
// check.stranded-column finding(s) it would have reported before the
// migration attempt.
func TestCheckMigrateColumnsRefusesRatherThanEmptyingTheDefinition(t *testing.T) {
	root := newBench(t)
	anchor := filepath.Join(benchDir(t, root), bench.WorkbenchAnchor)
	gone := strandAllColumns(t, root)

	before, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read anchor: %v", err)
	}

	migrated := runCLI(t, root, "check", "--migrate-columns")
	if migrated.code != 2 {
		t.Fatalf("check --migrate-columns: wanted exit 2, got %d\n%s", migrated.code, migrated.errw)
	}
	if !strings.Contains(migrated.errw, contract.RepairWouldEmptyColumns) {
		t.Errorf("wanted the repair-would-empty-columns refusal, got:\n%s", migrated.errw)
	}
	for _, id := range gone {
		if !strings.Contains(migrated.errw, id) {
			t.Errorf("the refusal did not name the stranded column %s:\n%s", id, migrated.errw)
		}
	}

	after, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read anchor after refusal: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("the refused migration touched workbench.md")
	}

	plain := runCLI(t, root, "check")
	if plain.code == 0 {
		t.Fatalf("a following check should still report the stranded columns")
	}
	for _, id := range gone {
		if !strings.Contains(plain.out, id) {
			t.Errorf("the following check did not name %s:\n%s", id, plain.out)
		}
	}
}

// TestAddRefusesWithNoLiveColumns asserts AC-8's CLI-level half: dinah add
// against a workbench whose columns list is entirely stranded prints and
// exits on the AddNeedsAColumn refusal, naming the workbench.md path and the
// dinah check / dinah add follow-up, and creates no card directory.
func TestAddRefusesWithNoLiveColumns(t *testing.T) {
	root := newBench(t)
	dir := benchDir(t, root)
	anchor := filepath.Join(dir, bench.WorkbenchAnchor)
	strandAllColumns(t, root)

	got := runCLI(t, root, "add", "stranded card")
	if got.code != 2 {
		t.Fatalf("add: wanted exit 2, got %d\n%s", got.code, got.errw)
	}
	if !strings.Contains(got.errw, contract.AddNeedsAColumn) {
		t.Errorf("wanted the add-needs-a-column refusal, got:\n%s", got.errw)
	}
	if !strings.Contains(got.errw, anchor) {
		t.Errorf("the refusal did not name the workbench.md path:\n%s", got.errw)
	}
	if !strings.Contains(got.errw, "dinah check") || !strings.Contains(got.errw, "dinah add") {
		t.Errorf("the refusal did not name both follow-up commands:\n%s", got.errw)
	}

	entries, err := os.ReadDir(filepath.Join(dir, bench.CardsDir))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read cards dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("wanted no card directory created, got %v", entries)
	}
}

// TestAHandTypedSlugLeavesTheWorkbenchOpenable asserts the corner an operator
// has to be able to get out of with the tool: somebody types a slug into a
// column anchor by hand and gets it wrong, and the workbench goes on opening,
// the checker names the column and the file, and the repair that fills in the
// columns around it still runs.
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
		column, anchor := writeColumnSlug(t, root, 0, "Caf--Corner")

		listed := runCLI(t, root, "columns")
		if listed.code != 0 {
			t.Fatalf("the workbench should still open: %d %s", listed.code, listed.errw)
		}
		if !strings.Contains(listed.out, "Caf--Corner") {
			t.Errorf("the listing should carry the slug as it stands:\n%s", listed.out)
		}

		reported := runCLI(t, root, "check")
		for _, fragment := range []string{column, anchor, "is not a letter followed by"} {
			if !strings.Contains(reported.out, fragment) {
				t.Errorf("the checker should carry %q:\n%s", fragment, reported.out)
			}
		}

		migrated := runCLI(t, root, "check", "--migrate-slugs")
		if !strings.Contains(migrated.out, "Assigned 2 column slugs.") {
			t.Errorf("the repair did not reach the columns around the bad one:\n%s", migrated.out)
		}
		if !strings.Contains(migrated.out, column) || !strings.Contains(migrated.out, anchor) {
			t.Errorf("the repair stopped naming the column it left alone:\n%s", migrated.out)
		}

		reopened := runCLI(t, root, "columns")
		if reopened.code != 0 {
			t.Fatalf("the repaired workbench should open: %d %s", reopened.code, reopened.errw)
		}
		for _, slug := range []string{"doing", "done"} {
			if !strings.Contains(reopened.out, slug) {
				t.Errorf("the listing does not carry the repaired slug %s:\n%s", slug, reopened.out)
			}
		}
	})

	t.Run("a slug another column already carries", func(t *testing.T) {
		root := newBench(t)
		writeColumnSlug(t, root, 0, "done")

		listed := runCLI(t, root, "columns")
		if listed.code != 0 {
			t.Fatalf("the workbench should still open: %d %s", listed.code, listed.errw)
		}

		// The walk names the second column to carry the value, which is the
		// one whose reference has stopped answering for it alone.
		duplicate, anchor := writeColumnSlug(t, root, 2, "done")
		reported := runCLI(t, root, "check")
		for _, fragment := range []string{duplicate, anchor, "another column of this workbench also carries"} {
			if !strings.Contains(reported.out, fragment) {
				t.Errorf("the checker should carry %q:\n%s", fragment, reported.out)
			}
		}

		migrated := runCLI(t, root, "check", "--migrate-slugs")
		if !strings.Contains(migrated.out, "Assigned 0 column slugs.") {
			t.Errorf("the repair should run and find nothing to assign:\n%s", migrated.out)
		}
	})
}

// writeColumnSlug types a slug into one column anchor of a workbench the way a
// person editing the file by hand would, and returns the column's identifier
// and the anchor's path, which are the two things a report about it names.
func writeColumnSlug(t *testing.T, root string, position int, slug string) (string, string) {
	t.Helper()
	machine := runCLI(t, root, "--json", "columns")
	var columns []verb.ColumnView
	if err := json.Unmarshal([]byte(machine.out), &columns); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	if position >= len(columns) {
		t.Fatalf("the workbench carries %d columns", len(columns))
	}
	path := filepath.Join(benchDir(t, root), "columns", columns[position].ID, "column.md")
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
	return columns[position].ID, path
}

// stripSlugs removes the slug from every column anchor of a workbench, which is
// the shape a workbench written before the field has on disk.
func stripSlugs(t *testing.T, root string) {
	t.Helper()
	columns := filepath.Join(benchDir(t, root), "columns")
	entries, err := os.ReadDir(columns)
	if err != nil {
		t.Fatalf("read columns: %v", err)
	}
	for _, entry := range entries {
		path := filepath.Join(columns, entry.Name(), "column.md")
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
				editAnchor(t, root, "profile: "+bench.ProfileVersion+"\n", "")
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
			token: contract.AmbiguousWorkbench,
			carries: []string{
				"more than one workbench is reachable from",
				"choose one with --workbench",
				"d00000000001", "d00000000002", "one", "two",
			},
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
			if c.token == contract.AmbiguousWorkbench {
				if report["detail"] != nil {
					t.Errorf("detail: wanted absent, got %v", report["detail"])
				}
				workbenches, ok := report["workbenches"].([]any)
				if !ok || len(workbenches) != 2 {
					t.Fatalf("workbenches: wanted a two-element array, got %v", report["workbenches"])
				}
				titles := map[string]bool{}
				for _, row := range workbenches {
					fields, _ := row.(map[string]any)
					if title, ok := fields["title"].(string); ok {
						titles[title] = true
					}
				}
				for _, title := range []string{"d00000000001", "d00000000002"} {
					if !titles[title] {
						t.Errorf("workbenches: wanted a row titled %q, got %v", title, report["workbenches"])
					}
				}
			} else if report["detail"] == nil || report["detail"] == "" {
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

// TestWorkbenchFlagResolvesAnAmbiguousCandidate asserts that --workbench,
// pointed at one of several candidates a base holds, resolves to that
// candidate rather than refusing with dinah.ambiguous-workbench: the flag
// names an exact directory and never runs the walk that finds the ambiguity
// in the first place.
func TestWorkbenchFlagResolvesAnAmbiguousCandidate(t *testing.T) {
	tree, rooms := ambiguousTree(t)
	got := runCLI(t, tree, "status", "--workbench", rooms[0])
	if got.code != 0 {
		t.Fatalf("status --workbench <candidate>: wanted 0, got %d (%s)", got.code, got.errw)
	}
	if !strings.Contains(got.out, "[flag]") {
		t.Errorf("the rendered line should name the flag rung, got:\n%s", got.out)
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
  "profile": "dinah-core/0.7",
  "title": %q,
  "columns": [
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
//
// The path is read from the display column its own heading starts at rather
// than as the last whitespace-separated field of a line. A workbench whose
// title cannot be laid out inside the window takes a line of its own and the
// slug and path resume underneath it, so the last field of a line is the
// title on one line and the path on the next.
//
// A line is read only where the path column really begins a field on it: a
// continuation line resuming exactly there, or a line whose field before the
// column ended in the gutter. A field that ran through the column belongs to
// the column it started in, and its tail is not a path.
func listedRows(t *testing.T, got invocation) []string {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("a listing should exit 0, got %d (%s)", got.code, got.errw)
	}
	lines := indentedBlock(got.out, "")
	if len(lines) < 2 {
		return nil
	}
	heading := msg.For(msg.Base).T("column.workbenches.path")
	at := startColumnOf(lines[0], heading)
	if at < 0 {
		return stackedValues(splitLines(got.out), heading)
	}
	paths := make([]string, 0, 2)
	for _, line := range lines[2:] {
		if sweptLead(line) > at {
			continue
		}
		if sweptLead(line) < at && !sweptSpaceAt(line, at-1) {
			continue
		}
		value := strings.TrimSpace(sweptField(line, at, -1))
		if value == "" {
			continue
		}
		paths = append(paths, value)
	}
	return paths
}

// stackedValues reads one column's values out of a stacked block, which is the
// form the workbench listing takes whenever a temporary directory makes its
// path long enough to take both of the other columns down to their headings.
// A record draws one labelled line per field, so the value wanted is whatever
// follows the column's own heading on the line that carries it. The whole
// output is walked rather than one indented run of it, since the blank line
// between two records ends such a run.
func stackedValues(lines []string, heading string) []string {
	values := make([]string, 0, 2)
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if !strings.HasPrefix(text, heading) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(text, heading))
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

// jsonRows reads a listing's machine form, which is a bare array with no
// envelope, the shape `columns --json` and `config --json` already emit.
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
		if !strings.Contains(got.out, slug) || !strings.Contains(got.out, filepath.Base(rooms[i])) {
			t.Errorf("the listing should carry the title and the slug of each workbench, wanted %q and %q in:\n%s",
				slug, filepath.Base(rooms[i]), got.out)
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
	// The sentence gained its next step with dinah-102, which gave one to each
	// of the twenty-one refusals that offered none. The empty {detail} ahead
	// of it is what a bare show has always rendered, since nobody named a card
	// for the sentence to be about; the shape table declares no subject for
	// unknown-card, so that fill is unchanged here and is filed rather than
	// repaired.
	want := contract.UnknownCard + " this workbench carries no card " +
		msg.For(msg.Base).T("refusal.unknown-card.next") + "\n"
	if got.errw != want {
		t.Errorf("the refusal should be the one a single workbench has always raised, with its next step:\n got  %q\n want %q", got.errw, want)
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
	refusal := runCLI(t, tree, "columns")
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

// TestAForeignWorkbenchFileIsPassedOverByTheClimb asserts AC-7: a directory
// holding a workbench.md that carries none of profile, format or columns,
// sitting below a real workbench, no longer stops the search. `dinah
// workbenches`, run from inside the foreign-holding directory, lists the real
// ancestor workbench by title and does not list the foreign directory.
func TestAForeignWorkbenchFileIsPassedOverByTheClimb(t *testing.T) {
	root := newBench(t)
	notes := filepath.Join(root, "notes")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(notes, "workbench.md"), []byte("# Just some notes\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := runCLI(t, notes, "workbenches")
	if got.code != 0 {
		t.Fatalf("a foreign workbench.md should not stop the search, got %d %q", got.code, got.errw)
	}
	listed := listedRows(t, got)
	if !sameDirs(t, listed, []string{benchDir(t, root)}) {
		t.Errorf("wanted only the real ancestor workbench, got %v", listed)
	}
	if strings.Contains(got.out, "notes") {
		t.Errorf("the foreign directory should not be listed, got %q", got.out)
	}
}

// TestCheckReportsTheForeignAnchorsAWalkPassedOver asserts the CLI half of
// AC-8: `dinah check`, run against a bench resolved through a foreign
// workbench.md, reports a check.ignored-anchor finding naming that file's
// path.
func TestCheckReportsTheForeignAnchorsAWalkPassedOver(t *testing.T) {
	root := newBench(t)
	notes := filepath.Join(root, "notes")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	foreign := filepath.Join(notes, "workbench.md")
	if err := os.WriteFile(foreign, []byte("# Just some notes\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := runCLI(t, notes, "check")
	if got.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Fatalf("a workbench carrying a finding exits with the findings code, got %d %q", got.code, got.errw)
	}
	catalog := msg.For(msg.Base)
	if !strings.Contains(got.out, catalog.T(bench.FindingIgnoredAnchor)) || !strings.Contains(got.out, foreign) {
		t.Errorf("wanted a check.ignored-anchor finding naming %q, got %q", foreign, got.out)
	}
}

// TestTheOverrideSkipsRecognitionAndLeavesItToOpen asserts AC-9: --workbench
// and DINAH_WORKBENCH test only file presence, unchanged. A recognition
// problem in the pointed-at file (no frontmatter carrying profile, format or
// columns at all) is reported by Open's existing malformed refusal rather than
// by a refusal the walk's new recognition test would raise.
func TestTheOverrideSkipsRecognitionAndLeavesItToOpen(t *testing.T) {
	root := newBench(t)
	foreign := filepath.Join(root, "notes")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "workbench.md"), []byte("# Just some notes\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := runCLI(t, root, "show", "--workbench", foreign)
	if got.code != 2 {
		t.Fatalf("exit code: wanted 2, got %d (%s)", got.code, got.errw)
	}
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.Malformed {
		t.Errorf("leading token: wanted %s, got %q (%s)", contract.Malformed, leading, got.errw)
	}
}

// TestCardAliasResolvesAcrossTheDeclaredCommandSurface is the fixture-level
// regression AC-6 asks for: it pins every row of the spec's section 2 audit
// table already marked done, driving the human-readable {slug}-{number}
// alias through claim, move, release, block, unblock, comment, attach,
// archive, delete, status, ls, next, show, log, instructions and path, so a
// later change cannot silently reopen a card-alias gap this card's spec
// found already closed.
//
// add, init, export and extract are not reference-accepting commands and
// carry no row of their own to pin here; edit shares ResolvePath with path
// and is not driven directly, since it launches a real editor process. guide,
// config and whoami name no entity and mcp serves the same verbs this test
// already exercises through the CLI.
func TestCardAliasResolvesAcrossTheDeclaredCommandSurface(t *testing.T) {
	root := newBench(t)

	first := addCard(t, root, "Alias card")
	second := addCard(t, root, "Second card")
	if !strings.HasPrefix(first, "fx-") || !strings.HasPrefix(second, "fx-") {
		t.Fatalf("wanted both cards to carry the fx- alias, got %q and %q", first, second)
	}

	// ls and next show the alias, not the bare identifier.
	if listed := runCLI(t, root, "ls"); listed.code != 0 || !strings.Contains(listed.out, first) || !strings.Contains(listed.out, second) {
		t.Fatalf("ls did not show both aliases: %d %q", listed.code, listed.out)
	}
	if offered := runCLI(t, root, "next"); offered.code != 0 || !strings.Contains(offered.out, first) {
		t.Fatalf("next did not show the alias: %d %q", offered.code, offered.out)
	}

	// show and path resolve the alias.
	if shown := runCLI(t, root, "show", first); shown.code != 0 || !strings.Contains(shown.out, first) {
		t.Fatalf("show %s: %d %q", first, shown.code, shown.out)
	}
	if pathed := runCLI(t, root, "path", first); pathed.code != 0 || !strings.Contains(pathed.out, "cards") {
		t.Fatalf("path %s: %d %q", first, pathed.code, pathed.out)
	}

	// claim, status, instructions and comment resolve the alias.
	carryToDoing(t, root, first)
	if claimed := runCLI(t, root, "claim", first); claimed.code != 0 || !strings.Contains(claimed.out, first) {
		t.Fatalf("claim %s: %d %q", first, claimed.code, claimed.out)
	}
	if status := runCLI(t, root, "status"); status.code != 0 || !strings.Contains(status.out, first) {
		t.Fatalf("status did not show the held alias: %d %q", status.code, status.out)
	}
	if instructed := runCLI(t, root, "instructions", first); instructed.code != 0 {
		t.Fatalf("instructions %s: %d %q", first, instructed.code, instructed.errw)
	}
	if commented := runCLI(t, root, "comment", first, "a note"); commented.code != 0 || !strings.Contains(commented.out, first) {
		t.Fatalf("comment %s: %d %q", first, commented.code, commented.out)
	}

	// move resolves the alias and the resulting card line still carries it.
	if moved := runCLI(t, root, "move", first, "doing"); moved.code != 0 || !strings.Contains(moved.out, first) {
		t.Fatalf("move %s: %d %q", first, moved.code, moved.out)
	}
	if logged := runCLI(t, root, "log", first); logged.code != 0 || !strings.Contains(logged.out, "moved") {
		t.Fatalf("log %s: %d %q", first, logged.code, logged.out)
	}

	// block and unblock resolve the alias.
	if blocked := runCLI(t, root, "block", first, "waiting on something"); blocked.code != 0 || !strings.Contains(blocked.out, first) {
		t.Fatalf("block %s: %d %q", first, blocked.code, blocked.out)
	}
	if unblocked := runCLI(t, root, "unblock", first); unblocked.code != 0 || !strings.Contains(unblocked.out, first) {
		t.Fatalf("unblock %s: %d %q", first, unblocked.code, unblocked.out)
	}
	if claimed := runCLI(t, root, "claim", first); claimed.code != 0 {
		t.Fatalf("re-claim %s: %d %q", first, claimed.code, claimed.errw)
	}
	if released := runCLI(t, root, "release", first); released.code != 0 || !strings.Contains(released.out, first) {
		t.Fatalf("release %s: %d %q", first, released.code, released.out)
	}

	// attach resolves the alias.
	note := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(note, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	if attached := runCLI(t, root, "attach", first, note); attached.code != 0 {
		t.Fatalf("attach %s: %d %q", first, attached.code, attached.errw)
	}

	// archive and delete each resolve the alias, on two different cards
	// since an archived card no longer answers to a live reference.
	if archived := runCLI(t, root, "archive", first); archived.code != 0 {
		t.Fatalf("archive %s: %d %q", first, archived.code, archived.errw)
	}
	if deleted := runCLI(t, root, "delete", second, "--yes"); deleted.code != 0 {
		t.Fatalf("delete %s: %d %q", second, deleted.code, deleted.errw)
	}
}

// addCard files a card and returns the alias it was given.
func addCard(t *testing.T, root, title string) string {
	t.Helper()
	got := runCLI(t, root, "--json", "add", title)
	if got.code != 0 {
		t.Fatalf("add %s: %d %s", title, got.code, got.errw)
	}
	var response struct {
		Card *verb.CardView `json:"card"`
	}
	if err := json.Unmarshal([]byte(got.out), &response); err != nil {
		t.Fatalf("decode: %v\n%s", err, got.out)
	}
	if response.Card == nil || response.Card.Ref == "" {
		t.Fatalf("add did not return a ref: %s", got.out)
	}
	return response.Card.Ref
}

// cardID resolves a card's own bare identifier from its alias, by asking
// path for the card's file and reading the directory name that holds it.
func cardID(t *testing.T, root, ref string) string {
	t.Helper()
	pathed := runCLI(t, root, "path", ref)
	if pathed.code != 0 {
		t.Fatalf("path %s: %d %s", ref, pathed.code, pathed.errw)
	}
	return filepath.Base(filepath.Dir(strings.TrimSpace(pathed.out)))
}

// addLink hand-writes a links entry onto a card's own anchor, since no verb
// in the declared surface writes one (card.go's Link comment: "a declaration
// rather than an entity, and nothing in the tool reads one" for anything but
// display).
func addLink(t *testing.T, root, ref, kind, to string) {
	t.Helper()
	path := runCLI(t, root, "path", ref)
	if path.code != 0 {
		t.Fatalf("path %s: %d %s", ref, path.code, path.errw)
	}
	cardPath := strings.TrimSpace(path.out)
	text, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("read card: %v", err)
	}
	closing := strings.LastIndex(string(text), "\n---\n")
	if closing < 0 {
		t.Fatalf("card %s carries no closing frontmatter fence", cardPath)
	}
	block := "\nlinks:\n  - kind: " + kind + "\n    to: " + to
	edited := string(text[:closing]) + block + string(text[closing:])
	if err := os.WriteFile(cardPath, []byte(edited), 0o644); err != nil {
		t.Fatalf("write card: %v", err)
	}
}

// TestShowResolvesALinkToTheAliasNotTheBareIdentifier is armed by asserting
// the alias in show's output and JSON form rather than the target's raw
// identifier. A link records only an identifier, and the render used to
// print that identifier verbatim: a destination a person is expected to type
// back at show, path or move, printed as the one thing they could not read
// comfortably. show now resolves it fresh on every read.
func TestShowResolvesALinkToTheAliasNotTheBareIdentifier(t *testing.T) {
	root := newBench(t)
	first := addCard(t, root, "First")
	second := addCard(t, root, "Second")
	targetID := cardID(t, root, second)
	addLink(t, root, first, "relates_to", targetID)

	shown := runCLI(t, root, "show", first)
	if shown.code != 0 {
		t.Fatalf("show %s: %d %s", first, shown.code, shown.errw)
	}
	if !strings.Contains(shown.out, second) {
		t.Errorf("show did not print the link's alias %q, got %q", second, shown.out)
	}
	if strings.Contains(shown.out, targetID) {
		t.Errorf("show printed the link's bare identifier %q instead of its alias: %q", targetID, shown.out)
	}

	jsonShown := runCLI(t, root, "--json", "show", first)
	if jsonShown.code != 0 {
		t.Fatalf("--json show %s: %d %s", first, jsonShown.code, jsonShown.errw)
	}
	var detail struct {
		Links []verb.LinkView `json:"links"`
	}
	if err := json.Unmarshal([]byte(jsonShown.out), &detail); err != nil {
		t.Fatalf("decode: %v\n%s", err, jsonShown.out)
	}
	if len(detail.Links) != 1 {
		t.Fatalf("wanted one link, got %v", detail.Links)
	}
	if detail.Links[0].To != targetID {
		t.Errorf("the machine form's to should stay the stable identifier %q, got %q", targetID, detail.Links[0].To)
	}
	if detail.Links[0].Ref != second {
		t.Errorf("the machine form's ref should carry the alias %q, got %q", second, detail.Links[0].Ref)
	}
}

// TestLegalMovesReportTheAliasNotTheBareColumnIdentifier is armed by
// asserting the ref field of a legal move rather than its column identifier.
// The moves-this-card-may-make listing, printed after claim, move and show,
// used to print the destination's bare identifier while move itself already
// accepted the column's slug, so the one place the tool told a person what to
// type next showed them something they could not comfortably type.
func TestLegalMovesReportTheAliasNotTheBareColumnIdentifier(t *testing.T) {
	root := newBench(t)
	first := addCard(t, root, "First")
	carryToDoing(t, root, first)

	claimed := runCLI(t, root, "--json", "claim", first)
	if claimed.code != 0 {
		t.Fatalf("claim %s: %d %s", first, claimed.code, claimed.errw)
	}
	var response struct {
		LegalMoves []verb.LegalMove `json:"legal_moves"`
	}
	if err := json.Unmarshal([]byte(claimed.out), &response); err != nil {
		t.Fatalf("decode: %v\n%s", err, claimed.out)
	}
	if len(response.LegalMoves) == 0 {
		t.Fatal("wanted at least one legal move")
	}
	for _, move := range response.LegalMoves {
		if move.Ref == "" {
			t.Fatalf("legal move to %s carries no ref: %+v", move.Column, move)
		}
		if move.Ref == move.Column {
			t.Errorf("legal move to %s: ref fell back to the bare identifier though the column carries a slug", move.Column)
		}
		// The ref is what move actually accepts, proving it is not merely
		// displayed but usable. The card is released first, because every
		// move out of the doing station on this flow lands at a column where
		// no owner takes work up, and such a column takes an unheld card.
		if released := runCLI(t, root, "release", first); released.code != 0 {
			t.Fatalf("release %s: %d %s", first, released.code, released.errw)
		}
		moved := runCLI(t, root, "move", first, move.Ref)
		if moved.code != 0 {
			t.Fatalf("move %s %s: %d %s", first, move.Ref, moved.code, moved.errw)
		}
		break
	}
}

// TestPerCommandHelpBreaksAnOverrunningRefusalName asserts dinah-81's AC-3:
// dinah help move, dinah help archive, and dinah help claim each place a
// checks-column entry whose catalog key reaches the 52-rune column on its
// own indented continuation line, with the refusal name starting that line
// rather than sitting one space past an overrunning check sentence.
func TestPerCommandHelpBreaksAnOverrunningRefusalName(t *testing.T) {
	container := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(container, "home"))
	t.Setenv("DINAH_ACTOR", "alka")

	cases := []struct {
		name string
		key  string
	}{
		{"move", "check.move.7"},
		{"archive", "check.archive.1"},
		{"claim", "check.workbench.1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := runCLI(t, container, "help", c.name)
			if got.code != 0 {
				t.Fatalf("help %s: %d %s", c.name, got.code, got.errw)
			}
			entry, ok := msg.BaseEntry(c.key)
			if !ok || entry.Text == "" {
				t.Fatalf("catalog carries no %s", c.key)
			}
			sentence := entry.Text
			var refusal string
			for _, check := range verb.Checks(c.name) {
				if check.Key == c.key {
					refusal = check.Refusal
				}
			}
			if refusal == "" {
				t.Fatalf("%s carries no check named %s", c.name, c.key)
			}
			glued := sentence + " " + refusal
			if strings.Contains(got.out, glued) {
				t.Errorf("%s: the refusal name still sits one space after the check sentence:\n%s", c.key, got.out)
			}
			lines := strings.Split(got.out, "\n")
			found := false
			for i, line := range lines {
				if !strings.Contains(line, sentence) {
					continue
				}
				found = true
				if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) != refusal {
					t.Errorf("%s: wanted a continuation line reading exactly %q, got %q", c.key, refusal, lines[min(i+1, len(lines)-1)])
				}
			}
			if !found {
				t.Fatalf("%s: help output never showed the check sentence:\n%s", c.key, got.out)
			}
		})
	}
}

// TestAttachmentHistoryEventsAlignTheirActorColumn asserts dinah-81's AC-4
// against the measured layout dinah-115 ships: a card history carrying an
// attachment_replaced or attachment_removed event, whose 19- and 18-rune
// event tokens are the widest that column ever holds, starts the actor field
// at the same display column on those rows as on every other row and as the
// heading above it.
//
// The subject has not moved and the mechanism has. The column is now as wide
// as the widest token in it, so the two long tokens no longer reach a
// declared width and no longer take a continuation line; what they must not
// do, and what this test still refuses, is push the actor along behind
// them.
func TestAttachmentHistoryEventsAlignTheirActorColumn(t *testing.T) {
	container := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(container, "home"))
	t.Setenv("DINAH_ACTOR", "tester")

	root := filepath.Join(container, "workbench")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, root, "init", "--operator", "tester"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	benchRoot := soleBenchDir(t, root)

	ref := addCard(t, benchRoot, "carries an attachment")

	source := filepath.Join(container, "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if got := runCLI(t, benchRoot, "attach", ref, source); got.code != 0 {
		t.Fatalf("attach: %d %s", got.code, got.errw)
	}

	replacement := filepath.Join(container, "revised.txt")
	if err := os.WriteFile(replacement, []byte("other bytes"), 0o644); err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	if got := runCLI(t, benchRoot, "attach", ref+"/attachments/1", replacement, "--replace"); got.code != 0 {
		t.Fatalf("replace: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, benchRoot, "delete", ref+"/attachments/1", "--yes"); got.code != 0 {
		t.Fatalf("delete attachment: %d %s", got.code, got.errw)
	}

	got := runCLI(t, benchRoot, "log", ref)
	if got.code != 0 {
		t.Fatalf("log: %d %s", got.code, got.errw)
	}

	replacedEntry, replacedOK := msg.BaseEntry("token.attachment_replaced")
	removedEntry, removedOK := msg.BaseEntry("token.attachment_removed")
	if !replacedOK || !removedOK {
		t.Fatalf("catalog carries no token for an attachment history event")
	}
	lines := indentedBlock(got.out, "")
	if len(lines) < 3 {
		t.Fatalf("the history drew no rows under its heading:\n%s", got.out)
	}
	actorColumn := startColumnOf(lines[0], msg.For(msg.Base).T("column.log.actor"))
	if actorColumn < 0 {
		t.Fatalf("the history carries no actor heading:\n%s", got.out)
	}
	for _, token := range []string{replacedEntry.Text, removedEntry.Text} {
		found := false
		for _, line := range lines[2:] {
			if !strings.Contains(line, token) {
				continue
			}
			found = true
			if at := startColumnOf(line, "tester"); at != actorColumn {
				t.Errorf("%q: the actor begins at display column %d and its heading begins at %d:\n%q",
					token, at, actorColumn, line)
			}
		}
		if !found {
			t.Errorf("log never showed the event token %q:\n%s", token, got.out)
		}
	}
}

// wantUsage asserts an invocation refuses with dinah.usage and that the
// refusal names the exact word given, which is the shape every case in this
// card's tests wants.
func wantUsage(t *testing.T, got invocation, word string) {
	t.Helper()
	if got.code != 2 {
		t.Fatalf("exit code: wanted 2, got %d (%s)", got.code, got.errw)
	}
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.Usage {
		t.Errorf("leading token: wanted %s, got %q", contract.Usage, got.errw)
	}
	if !strings.Contains(got.errw, word) {
		t.Errorf("the refusal should name the word given, wanted %q in %q", word, got.errw)
	}
}

// TestMistypedSingleDashRefusesOnAZeroBoundedCommand asserts dinah-69's AC-1:
// every zero-bounded-slot command called with a trailing single-dash word
// refuses with dinah.usage naming the word, in place of today's silent exit
// 0, and that nothing about a successful run of the same command changes.
func TestMistypedSingleDashRefusesOnAZeroBoundedCommand(t *testing.T) {
	for _, name := range []string{"status", "columns", "version", "workbenches", "export", "mcp", "check", "whoami"} {
		t.Run(name, func(t *testing.T) {
			wantUsage(t, runCLI(t, t.TempDir(), name, "-w"), "-w")
		})
	}
}

// TestWorkbenchesListingSurvivesAMissingSlug is the fixture-level twin of
// TestAlignedRowBreaksOnAnOverrunningCell: it drives the actual
// `workbenches` render, not the helper in isolation, over a row with no
// slug at all, whose placeholder is longer than the column it prints into,
// and asserts the row's path is still present and legible rather than
// shifted into the previous column.
func TestWorkbenchesListingSurvivesAMissingSlug(t *testing.T) {
	container := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(container, "home"))
	t.Setenv("DINAH_ACTOR", "alka")

	noSlug := filepath.Join(container, "noslug")
	if err := os.MkdirAll(noSlug, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, noSlug, "init", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	noSlugRoot := soleBenchDir(t, noSlug)
	editWorkbenchAnchor(t, filepath.Join(noSlugRoot, "workbench.md"), "slug: noslug\n", "slug:\n")

	got := runCLI(t, container, "--workbench", noSlugRoot, "workbenches")
	if got.code != 0 {
		t.Fatalf("workbenches: %d %s", got.code, got.errw)
	}
	if !strings.Contains(got.out, noSlugRoot) {
		t.Errorf("the missing-slug row's own path is missing or shifted out of the line: %q", got.out)
	}
	if strings.Count(got.out, "\n") < 2 {
		t.Errorf("wanted the overrunning placeholder to break onto a continuation line, got %q", got.out)
	}
}

// editWorkbenchAnchor rewrites one line of a workbench anchor already on
// disk, the way editWorkbench does for the bench-package fixtures this test
// mirrors.
func editWorkbenchAnchor(t *testing.T, path, from, to string) {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(text), from) {
		t.Fatalf("the workbench anchor carries no %q", from)
	}
	edited := strings.Replace(string(text), from, to, 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestMistypedSingleDashRefusesBeforeTheDomainCheck asserts dinah-69's AC-2:
// every command whose bounded slot is a card reference, a column name, or a
// guide/help topic refuses with dinah.usage naming the word when a
// single-dash word fills that slot, rather than today's unknown-card,
// unknown-column, unknown-command or unknown-guide refusal.
func TestMistypedSingleDashRefusesBeforeTheDomainCheck(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	cases := []struct {
		name string
		argv []string
	}{
		{"claim", []string{"claim", "-w"}},
		{"release", []string{"release", "-w"}},
		{"unblock", []string{"unblock", "-w"}},
		{"archive", []string{"archive", "-w"}},
		{"delete", []string{"delete", "-w"}},
		{"log", []string{"log", "-w"}},
		{"instructions", []string{"instructions", "-w"}},
		{"show", []string{"show", "-w"}},
		{"ls", []string{"ls", "-w"}},
		{"next", []string{"next", "-w"}},
		{"guide", []string{"guide", "-w"}},
		{"help", []string{"help", "-w"}},
		{"move's card slot", []string{"move", "-w", "ready"}},
		{"move's column slot", []string{"move", "fx-1", "-w"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantUsage(t, runCLI(t, root, c.argv...), "-w")
		})
	}
}

// TestMistypedSingleDashRefusesOnAPathSlotInsteadOfCreatingSomething asserts
// dinah-69's AC-3 and is the card's flagship case: `init -w` no longer
// creates a directory named `-w` and initialises a workbench inside it, and
// the same refusal reaches attach's ref and file slots, extract's directory
// slot, and path's and edit's argument slot.
func TestMistypedSingleDashRefusesOnAPathSlotInsteadOfCreatingSomething(t *testing.T) {
	base := t.TempDir()
	wantUsage(t, runCLI(t, base, "init", "-w", "--operator", "tester"), "-w")
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a refused init should create nothing, found %v", entries)
	}

	root := newBench(t)
	runCLI(t, root, "add", "A card")
	for _, argv := range [][]string{
		{"attach", "-w", "somefile"},
		{"attach", "fx-1", "-w"},
		{"extract", "-w"},
		{"path", "-w"},
		{"edit", "-w"},
	} {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			wantUsage(t, runCLI(t, root, argv...), "-w")
		})
	}
	if bench.Exists(filepath.Join(root, "-w")) {
		t.Error("a refused attach/extract should leave no -w path behind")
	}

	// "./" still reaches a real dash-led filesystem name unchanged, for the
	// slots that resolve a filesystem path directly rather than a card
	// reference: init's root and extract's directory.
	dashDir := filepath.Join(t.TempDir(), "-realdir")
	if err := os.MkdirAll(dashDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got := runCLI(t, root, "extract", filepath.Join(dashDir, "notyetused"))
	if got.code != 0 {
		t.Fatalf("extract into a real dash-led directory: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
}

// TestOpenTailContinuesToAcceptALeadingDashWord asserts dinah-69's AC-5: the
// open-tail slots (add's title, block's reason, comment's text, config set's
// value) keep taking a leading-dash word exactly as before, the bare "-"
// still reads comment's text from stdin, a single-dash word given as an
// explicit flag's own value is unaffected, and the top-level command name is
// unaffected.
func TestOpenTailContinuesToAcceptALeadingDashWord(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")

	got := runCLI(t, root, "add", "-weirdtitle2")
	if got.code != 0 {
		t.Fatalf("add -weirdtitle2: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	if !strings.Contains(got.out, "-weirdtitle2") {
		t.Errorf("the title should carry the dash-led word verbatim, got %q", got.out)
	}

	got = runCLI(t, root, "block", "fx-1", "-not-a-flag the actual reason")
	if got.code != 0 {
		t.Fatalf("block with a dash-led reason: wanted exit 0, got %d (%s)", got.code, got.errw)
	}

	_, homeBase := settingsHome(t)
	if got := runCLI(t, homeBase, "config", "set", "actor", "-w"); got.code != 0 {
		t.Fatalf("config set actor -w: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	if got := runCLI(t, homeBase, "config", "get", "actor"); got.code != 0 || strings.TrimSpace(got.out) != "-w" {
		t.Errorf("config set should have stored -w verbatim, got %d %q", got.code, got.out)
	}

	piped := strings.NewReader("piped comment text")
	if got := runCLIWithInput(t, root, piped, "comment", "fx-1", "-"); got.code != 0 {
		t.Fatalf("comment fx-1 -: wanted exit 0, got %d (%s)", got.code, got.errw)
	}

	// A single-dash word given as an explicit flag's own value is unaffected
	// and still reaches the domain refusal, not dinah.usage.
	got = runCLI(t, root, "move", "fx-1", "nowhere", "--column", "-w")
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.UnknownColumn {
		t.Errorf("an explicit flag value: wanted %s, got %q", contract.UnknownColumn, got.errw)
	}

	// The top-level command name itself is unaffected.
	got = runCLI(t, root, "-w", "ls")
	leading = strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.UnknownVerb {
		t.Errorf("a mistyped command name: wanted %s, got %q", contract.UnknownVerb, got.errw)
	}
}

// TestConfigRefusesAMistypedSingleDashKey asserts dinah-69's AC-4: config get
// and config set refuse with dinah.usage naming the exact word when the key
// is a single-dash word, before the unknown-key check runs, and a bare
// `config <word>` that is neither get nor set refuses naming that word
// rather than the literal string "config".
func TestConfigRefusesAMistypedSingleDashKey(t *testing.T) {
	_, dir := settingsHome(t)

	wantUsage(t, runCLI(t, dir, "config", "-w"), "-w")
	wantUsage(t, runCLI(t, dir, "config", "get", "-w"), "-w")
	wantUsage(t, runCLI(t, dir, "config", "set", "-w", "value"), "-w")

	got := runCLI(t, dir, "config", "bogus")
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.Usage {
		t.Errorf("leading token: wanted %s, got %q", contract.Usage, got.errw)
	}
	if !strings.Contains(got.errw, "bogus") {
		t.Errorf("the default branch should name the word given rather than the literal \"config\", got %q", got.errw)
	}
}

// TestMistypedSingleDashBeyondACommandsOwnArityStillRefuses answers dinah-69's
// OQ-1: for a command with no open tail, the check walks every word in
// rest(), not merely the command's own declared bounded count, so a word
// beyond a one-bounded command's own arity refuses exactly as a dash word
// inside a declared slot does. dinah-90 widens the walk past bounded to any
// word, not only a dash-led one, so the plain word "extraword" is now the
// first offending word scanning left to right, and it is what the refusal
// names, ahead of the "-x" that follows it.
func TestMistypedSingleDashBeyondACommandsOwnArityStillRefuses(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	wantUsage(t, runCLI(t, root, "claim", "fx-1", "extraword", "-x"), "extraword")
}

// TestPlainWordBeyondAZeroBoundedCommandRefuses asserts dinah-90's AC-1: every
// zero-bounded, no-open-tail command called with one plain trailing word (no
// leading dash) refuses with dinah.usage naming that exact word, in place of
// today's silent exit 0.
func TestPlainWordBeyondAZeroBoundedCommandRefuses(t *testing.T) {
	for _, name := range []string{"status", "columns", "version", "export", "mcp", "check", "whoami"} {
		t.Run(name, func(t *testing.T) {
			wantUsage(t, runCLI(t, t.TempDir(), name, "somejunk"), "somejunk")
		})
	}
}

// TestPlainWordBeyondAOneBoundedCommandRefuses asserts dinah-90's AC-2: every
// one-bounded, no-open-tail command called with its one legitimate argument
// plus one plain trailing word refuses with dinah.usage naming the trailing
// word, while the same call with only its one legitimate argument is
// unaffected.
func TestPlainWordBeyondAOneBoundedCommandRefuses(t *testing.T) {
	cases := []struct {
		name string
		argv []string
	}{
		{"claim", []string{"claim", "fx-1"}},
		{"release", []string{"release", "fx-1"}},
		{"unblock", []string{"unblock", "fx-1"}},
		{"archive", []string{"archive", "fx-1"}},
		{"delete", []string{"delete", "fx-1"}},
		{"log", []string{"log", "fx-1"}},
		{"instructions", []string{"instructions", "fx-1"}},
		{"show", []string{"show", "fx-1"}},
		{"ls", []string{"ls", "ready"}},
		{"next", []string{"next", "ready"}},
		{"guide", []string{"guide", "claim"}},
		{"help", []string{"help", "claim"}},
		{"extract", []string{"extract", filepath.Join(t.TempDir(), "out")}},
		{"path", []string{"path", "fx-1"}},
		// workbenches joined this table on dinah-281, which gave it a
		// positional: the directory to walk downward from. Its own bounded
		// slot is a path rather than a card reference, so the baseline call
		// names a real directory.
		{"workbenches", []string{"workbenches", t.TempDir()}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A fresh workbench per case, so a stateful command earlier in
			// the table (archive, delete, claim) cannot leave the card in a
			// shape a later case's own baseline call did not expect.
			root := newBench(t)
			runCLI(t, root, "add", "A card")
			withExtra := append(append([]string{}, c.argv...), "extraword")
			wantUsage(t, runCLI(t, root, withExtra...), "extraword")

			root = newBench(t)
			runCLI(t, root, "add", "A card")
			got := runCLI(t, root, c.argv...)
			leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
			if got.code != 0 && leading == contract.Usage {
				t.Errorf("without the extra word: should not refuse with dinah.usage, got %d (%s)", got.code, got.errw)
			}
		})
	}

	// edit's own baseline call, without an extra word, would launch a real
	// editor process, so only the extra-word refusal is checked here; its
	// baseline case is exactly the shape TestMistypedSingleDashRefusesOnAPathSlotInsteadOfCreatingSomething
	// already exercises for the dash-led form, and this card widens the same
	// check ahead of the point where an editor would ever be launched.
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	wantUsage(t, runCLI(t, root, "edit", "fx-1", "extraword"), "extraword")

	// init reads its own root argument (its bounded slot) rather than a card
	// reference, so it is exercised on its own with a real target directory
	// standing in for the legitimate argument.
	base := t.TempDir()
	sub := filepath.Join(base, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wantUsage(t, runCLI(t, base, "init", sub, "extraword", "--operator", "tester"), "extraword")
	got := runCLI(t, base, "init", sub, "--operator", "tester")
	if got.code != 0 {
		t.Errorf("init with only its own argument: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
}

// TestPlainWordBeyondATwoBoundedCommandRefuses asserts dinah-90's AC-3: move
// and attach, the two-bounded no-open-tail commands, refuse dinah.usage
// naming a third plain trailing word, while both of their own two arguments
// are unaffected.
func TestPlainWordBeyondATwoBoundedCommandRefuses(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")

	wantUsage(t, runCLI(t, root, "move", "fx-1", "Doing", "thirdword"), "thirdword")
	got := runCLI(t, root, "move", "fx-1", "Doing")
	if got.code != 0 {
		t.Errorf("move with only its own two arguments: wanted exit 0, got %d (%s)", got.code, got.errw)
	}

	somefile := filepath.Join(t.TempDir(), "somefile")
	if err := os.WriteFile(somefile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	wantUsage(t, runCLI(t, root, "attach", "fx-1", somefile, "thirdword"), "thirdword")
	got = runCLI(t, root, "attach", "fx-1", somefile)
	if got.code != 0 {
		t.Errorf("attach with only its own two arguments: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
}

// TestConfigGetRefusesAThirdWord asserts dinah-90's AC-4: config get refuses a
// third word naming it, in place of today's silent success that drops it,
// while config set with a several-word value is unaffected and config's own
// existing checks are unaffected.
func TestConfigGetRefusesAThirdWord(t *testing.T) {
	_, dir := settingsHome(t)

	runCLI(t, dir, "config", "set", "actor", "somebody")
	wantUsage(t, runCLI(t, dir, "config", "get", "actor", "extra"), "extra")

	got := runCLI(t, dir, "config", "get", "actor")
	if got.code != 0 || strings.TrimSpace(got.out) != "somebody" {
		t.Errorf("config get with only its key: wanted exit 0 and \"somebody\", got %d %q", got.code, got.out)
	}

	got = runCLI(t, dir, "config", "set", "actor", "a whole name")
	if got.code != 0 {
		t.Fatalf("config set with a quoted multi-word value: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	if got := runCLI(t, dir, "config", "get", "actor"); got.code != 0 || strings.TrimSpace(got.out) != "a whole name" {
		t.Errorf("config set should have stored the quoted value verbatim, got %d %q", got.code, got.out)
	}

	// config's own existing checks are unaffected: a bare bogus word, a
	// dash-led first word, and a dash-led key still refuse as before.
	got = runCLI(t, dir, "config", "bogus")
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.Usage || !strings.Contains(got.errw, "bogus") {
		t.Errorf("config bogus: wanted dinah.usage naming bogus, got %q", got.errw)
	}
	wantUsage(t, runCLI(t, dir, "config", "-w"), "-w")
	wantUsage(t, runCLI(t, dir, "config", "get", "-w"), "-w")
}

// TestOpenTailKeepsAcceptingAnyNumberOfPlainWords asserts dinah-100's AC-2:
// every open-tail command keeps accepting a many-word phrase as content, so
// long as it arrives quoted into the one argument the slot now takes; a
// second, separate unquoted word is what dinah.multiple-words exists to
// catch instead (TestOpenTailKeepsAcceptingExactlyOneWord below).
func TestOpenTailKeepsAcceptingAnyNumberOfPlainWords(t *testing.T) {
	root := newBench(t)

	got := runCLI(t, root, "add", "A whole title")
	if got.code != 0 {
		t.Fatalf("add with a quoted many-word title: wanted exit 0, got %d (%s)", got.code, got.errw)
	}

	got = runCLI(t, root, "block", "fx-1", "a whole reason")
	if got.code != 0 {
		t.Fatalf("block with a quoted many-word reason: wanted exit 0, got %d (%s)", got.code, got.errw)
	}

	got = runCLI(t, root, "comment", "fx-1", "a whole comment")
	if got.code != 0 {
		t.Fatalf("comment with a quoted many-word comment: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
}

// TestOpenTailKeepsAcceptingExactlyOneWord asserts dinah-100's own contract
// directly: a second unquoted word past an open-tail command's fixed
// arguments refuses with dinah.multiple-words naming the word count and the
// slot, and rebuilds the command line with the free text quoted so the
// caller can copy the fix.
func TestOpenTailKeepsAcceptingExactlyOneWord(t *testing.T) {
	root := newBench(t)

	got := runCLI(t, root, "add", "Fix", "the", "login", "bug")
	wantMultipleWords := "dinah.multiple-words Dinah read 4 separate words for the title, and it only accepts one. Put quotation marks around the whole thing: dinah add \"Fix the login bug\"\n"
	if got.code != 2 || got.errw != wantMultipleWords {
		t.Errorf("add with four loose words:\n got  %d %q\n want 2 %q", got.code, got.errw, wantMultipleWords)
	}

	runCLI(t, root, "add", "A card")

	got = runCLI(t, root, "block", "fx-1", "the", "door", "jammed")
	wantMultipleWords = "dinah.multiple-words Dinah read 3 separate words for the reason, and it only accepts one. Put quotation marks around the whole thing: dinah block fx-1 \"the door jammed\"\n"
	if got.code != 2 || got.errw != wantMultipleWords {
		t.Errorf("block with three loose words:\n got  %d %q\n want 2 %q", got.code, got.errw, wantMultipleWords)
	}

	got = runCLI(t, root, "comment", "fx-1", "done", "and", "dusted")
	wantMultipleWords = "dinah.multiple-words Dinah read 3 separate words for the comment, and it only accepts one. Put quotation marks around the whole thing: dinah comment fx-1 \"done and dusted\"\n"
	if got.code != 2 || got.errw != wantMultipleWords {
		t.Errorf("comment with three loose words:\n got  %d %q\n want 2 %q", got.code, got.errw, wantMultipleWords)
	}

	_, dir := settingsHome(t)
	got = runCLI(t, dir, "config", "set", "actor", "Paul", "Parks")
	wantMultipleWords = "dinah.multiple-words Dinah read 2 separate words for the value, and it only accepts one. Put quotation marks around the whole thing: dinah config set actor \"Paul Parks\"\n"
	if got.code != 2 || got.errw != wantMultipleWords {
		t.Errorf("config set with two loose words:\n got  %d %q\n want 2 %q", got.code, got.errw, wantMultipleWords)
	}
}

// TestOpenTailAsksForQuotingItselfWhenTheTextHasAQuote guards the cycle-3
// review finding on dinah-100: which shell reads back a pasted line is
// undocumented, and bash, cmd.exe and PowerShell disagree on how to escape
// an embedded quotation mark (a backslash escape round-trips in bash and
// cmd.exe but is not an escape character inside a PowerShell double-quoted
// string, so it mangles the text there instead). Rather than hand back a
// rebuilt example that is wrong in whichever shell the caller is using, a
// free-text value that itself contains a `"` refuses with an instruction to
// quote it themselves and offers no example at all. A backslash alone, or a
// single quote alone, carries no such ambiguity and keeps the ordinary
// rebuilt-example refusal (TestOpenTailKeepsAcceptingExactlyOneWord).
func TestOpenTailAsksForQuotingItselfWhenTheTextHasAQuote(t *testing.T) {
	root := newBench(t)

	got := runCLI(t, root, "add", "say", `"hi"`, "there")
	want := "dinah.multiple-words Dinah read 3 separate words for the title, and it only accepts one." +
		" That text has a quotation mark in it, so put quotation marks around the whole thing yourself." +
		" No command line Dinah rebuilds pastes back correctly in every shell once the text itself contains one.\n"
	if got.code != 2 || got.errw != want {
		t.Errorf("add with an embedded quote character:\n got  %d %q\n want 2 %q", got.code, got.errw, want)
	}
	if strings.Contains(got.errw, "dinah add") {
		t.Errorf("a quote-in-text refusal should offer no rebuilt example, got %q", got.errw)
	}

	got = runCLI(t, root, "add", `C:\say"hi"`, "here")
	if got.code != 2 || got.errw != "dinah.multiple-words Dinah read 2 separate words for the title, and it only accepts one."+
		" That text has a quotation mark in it, so put quotation marks around the whole thing yourself."+
		" No command line Dinah rebuilds pastes back correctly in every shell once the text itself contains one.\n" {
		t.Errorf("add with a backslash and an embedded quote:\n got  %d %q", got.code, got.errw)
	}

	got = runCLI(t, root, "add", "it's", "only", "a", "single", "quote")
	wantSingleQuote := `dinah.multiple-words Dinah read 5 separate words for the title, and it only accepts one. Put quotation marks around the whole thing: dinah add "it's only a single quote"` + "\n"
	if got.code != 2 || got.errw != wantSingleQuote {
		t.Errorf("add with only a single (apostrophe) quote:\n got  %d %q\n want 2 %q", got.code, got.errw, wantSingleQuote)
	}

	got = runCLI(t, root, "add", `C:\path`, "here")
	wantBackslash := `dinah.multiple-words Dinah read 2 separate words for the title, and it only accepts one. Put quotation marks around the whole thing: dinah add "C:\path here"` + "\n"
	if got.code != 2 || got.errw != wantBackslash {
		t.Errorf("add with a backslash and no quote:\n got  %d %q\n want 2 %q", got.code, got.errw, wantBackslash)
	}
}

// showDetail runs `--json show` for a card and decodes the parts these tests
// read: the title, the block reason, and the comments in order.
func showDetail(t *testing.T, dir, ref string) verb.Detail {
	t.Helper()
	got := runCLI(t, dir, "--json", "show", ref)
	if got.code != 0 {
		t.Fatalf("--json show %s: %d %s", ref, got.code, got.errw)
	}
	var detail verb.Detail
	if err := json.Unmarshal([]byte(got.out), &detail); err != nil {
		t.Fatalf("decode: %v\n%s", err, got.out)
	}
	return detail
}

// TestMarkerAcceptsAWordThatLooksLikeAnOptionAtEveryPosition asserts
// dinah-100's AC-5: a bare "--" still ends the flag scan, so comment, block
// and add all store a quoted word beginning with -- verbatim, whether the
// -- prefix opens, sits in the middle of, or closes the one free-text word
// that follows the marker.
func TestMarkerAcceptsAWordThatLooksLikeAnOptionAtEveryPosition(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")

	got := runCLI(t, root, "comment", "fx-1", "--", "--verbose is a flag some tools take")
	if got.code != 0 {
		t.Fatalf("comment, word at start: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	got = runCLI(t, root, "comment", "fx-1", "--", "remember to pass --verbose next time")
	if got.code != 0 {
		t.Fatalf("comment, word in middle: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	got = runCLI(t, root, "comment", "fx-1", "--", "next time pass --verbose")
	if got.code != 0 {
		t.Fatalf("comment, word at end: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	detail := showDetail(t, root, "fx-1")
	if len(detail.Comments) != 3 {
		t.Fatalf("wanted 3 comments, got %d", len(detail.Comments))
	}
	wantComments := []string{
		"--verbose is a flag some tools take",
		"remember to pass --verbose next time",
		"next time pass --verbose",
	}
	for i, want := range wantComments {
		if detail.Comments[i].Body != want {
			t.Errorf("comment %d: wanted %q, got %q", i, want, detail.Comments[i].Body)
		}
	}

	got = runCLI(t, root, "block", "fx-1", "--", "--waiting on external dep")
	if got.code != 0 {
		t.Fatalf("block, word at start: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	blocked := showDetail(t, root, "fx-1")
	if blocked.Card.BlockReason != "--waiting on external dep" {
		t.Errorf("block reason: wanted %q, got %q", "--waiting on external dep", blocked.Card.BlockReason)
	}
	runCLI(t, root, "unblock", "fx-1")

	got = runCLI(t, root, "add", "--", "--urgent fix the thing")
	if got.code != 0 {
		t.Fatalf("add, word at start: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	added := showDetail(t, root, "fx-2")
	if added.Card.Title != "--urgent fix the thing" {
		t.Errorf("add title: wanted %q, got %q", "--urgent fix the thing", added.Card.Title)
	}
}

// TestMarkerAcceptsADashPrefixedConfigValue asserts dinah-100's AC-5: config
// set stores a quoted value beginning with -- verbatim once the marker
// precedes it.
func TestMarkerAcceptsADashPrefixedConfigValue(t *testing.T) {
	_, dir := settingsHome(t)

	got := runCLI(t, dir, "config", "set", "actor", "--", "--urgent tester")
	if got.code != 0 {
		t.Fatalf("config set past the marker: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	got = runCLI(t, dir, "config", "get", "actor")
	if got.code != 0 || strings.TrimSpace(got.out) != "--urgent tester" {
		t.Errorf("config get actor: wanted %q, got %d %q", "--urgent tester", got.code, got.out)
	}
}

// TestMarkerHandlesTheAwkwardShapes asserts dinah-92's AC-4: a bare trailing
// marker with nothing after it is consumed rather than refused by the flag
// scan itself (config set takes an optional value, unlike comment's required
// text, so it is the vehicle for that half of the case; an omitted value
// clears the key), and a second "--"
// already past the first one is ordinary text rather than a second marker.
func TestMarkerHandlesTheAwkwardShapes(t *testing.T) {
	_, dir := settingsHome(t)

	got := runCLI(t, dir, "config", "set", "actor", "--")
	if got.code != 0 {
		t.Fatalf("a bare trailing marker with nothing after it: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	if strings.Contains(got.errw, "was not understood") {
		t.Errorf("the bare marker should not be refused as an unrecognized flag: %q", got.errw)
	}
	got = runCLI(t, dir, "config", "get", "actor")
	if got.code != 0 || strings.TrimSpace(got.out) != "" {
		t.Errorf("config get actor: wanted nothing, since the bare marker cleared the key, got %d %q", got.code, got.out)
	}

	root := newBench(t)
	runCLI(t, root, "add", "A card")

	got = runCLI(t, root, "comment", "fx-1", "--", "--")
	if got.code != 0 {
		t.Fatalf("two markers: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	got = runCLI(t, root, "comment", "fx-1", "--", "please mention -- here")
	if got.code != 0 {
		t.Fatalf("a marker inside text already past a marker: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	detail := showDetail(t, root, "fx-1")
	if len(detail.Comments) != 2 {
		t.Fatalf("wanted 2 comments, got %d", len(detail.Comments))
	}
	wantComments := []string{
		"--",
		"please mention -- here",
	}
	for i, want := range wantComments {
		if detail.Comments[i].Body != want {
			t.Errorf("comment %d: wanted %q, got %q", i, want, detail.Comments[i].Body)
		}
	}
}

// TestMarkerFreesAWordASiblingCheckWouldOtherwiseRefuse asserts dinah-92's
// last awkward case: a word after the marker that looksLikeMistypedFlag
// would refuse as a single-dash word lands as literal text instead, because
// the marker frees it into free text before that check ever sees it.
func TestMarkerFreesAWordASiblingCheckWouldOtherwiseRefuse(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")

	got := runCLI(t, root, "comment", "fx-1", "--", "the option is -w not what you want")
	if got.code != 0 {
		t.Fatalf("a single-dash word past the marker: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	detail := showDetail(t, root, "fx-1")
	want := "the option is -w not what you want"
	if len(detail.Comments) != 1 || detail.Comments[0].Body != want {
		t.Fatalf("wanted one comment %q, got %v", want, detail.Comments)
	}

	// The sibling checks still refuse a single-dash word in a bounded slot
	// when no marker frees it: dinah-69's own case is unaffected.
	wantUsage(t, runCLI(t, root, "claim", "-w"), "-w")
}

// TestBareMarkerAloneNoLongerRefusesItself asserts the open question the
// spec settled: a bare "--" typed alone, with nothing after it, is consumed
// as the marker rather than refused as an unrecognized flag. comment's own
// domain still refuses the empty text that leaves behind, exactly as it
// refuses an empty comment with no marker involved at all; what changes is
// which refusal fires. Today's dinah.usage naming "--" itself is gone, and
// malformed naming "text" takes its place.
func TestBareMarkerAloneNoLongerRefusesItself(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	got := runCLI(t, root, "comment", "fx-1", "--")
	if got.code != 2 {
		t.Fatalf("a bare -- alone: wanted exit 2 (empty comment text, not the marker), got %d (%s)", got.code, got.errw)
	}
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.Malformed {
		t.Errorf("wanted the empty-text refusal (%s), got %q", contract.Malformed, got.errw)
	}
	if strings.Contains(got.errw, "was not understood") {
		t.Errorf("the marker itself should not be refused as an unrecognized flag: %q", got.errw)
	}
}

// TestKnownFlagBeforeAndAfterTheMarker asserts dinah-92's AC-5: a known
// global flag written before the marker still parses as a flag exactly as it
// does today, and the same flag word written after the marker is read as
// literal text instead of being silently dropped.
func TestKnownFlagBeforeAndAfterTheMarker(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")

	before := runCLI(t, root, "comment", "fx-1", "--json", "before the marker")
	if before.code != 0 {
		t.Fatalf("--json before the marker: wanted exit 0, got %d (%s)", before.code, before.errw)
	}
	var beforeReport struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal([]byte(before.out), &beforeReport); err != nil {
		t.Fatalf("--json before the marker should still parse as the flag and emit machine json: %v\n%s", err, before.out)
	}
	beforeDetail := showDetail(t, root, "fx-1")
	if beforeDetail.Comments[0].Body != "before the marker" {
		t.Errorf("comment: wanted %q, got %q", "before the marker", beforeDetail.Comments[0].Body)
	}

	after := runCLI(t, root, "comment", "fx-1", "--", "after the marker --json here")
	if after.code != 0 {
		t.Fatalf("--json after the marker: wanted exit 0, got %d (%s)", after.code, after.errw)
	}
	if strings.TrimSpace(after.out) != "" && json.Valid([]byte(after.out)) {
		t.Errorf("--json after the marker should be literal text, not the flag, so this run should not have emitted machine json: %q", after.out)
	}
	detail := showDetail(t, root, "fx-1")
	last := detail.Comments[len(detail.Comments)-1].Body
	if last != "after the marker --json here" {
		t.Errorf("comment: wanted %q, got %q (the flag word after the marker should be stored, not dropped)", "after the marker --json here", last)
	}
}

// TestUsageRefusalNamesTheMarker asserts dinah-92's AC-6: the refusal for an
// unrecognized --word and for a recognized valued flag missing its value
// both carry the dash hint, and it names the "--" marker.
func TestUsageRefusalNamesTheMarker(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")

	wantUnknown := "dinah.usage --bogus was not understood; run dinah help for the list of commands. Dinah reads a word starting with two dashes as an option. Write `--` first, and Dinah reads every word that follows as plain text, dashes included.\n"
	unknown := runCLI(t, root, "comment", "fx-1", "nice", "work", "--bogus", "here")
	if unknown.code != 2 {
		t.Fatalf("unrecognized flag: wanted exit 2, got %d (%s)", unknown.code, unknown.errw)
	}
	if unknown.errw != wantUnknown {
		t.Errorf("unrecognized flag refusal:\n got  %q\n want %q", unknown.errw, wantUnknown)
	}

	wantMissing := "dinah.usage --actor was not understood; run dinah help for the list of commands. Dinah reads a word starting with two dashes as an option. Write `--` first, and Dinah reads every word that follows as plain text, dashes included.\n"
	missing := runCLI(t, root, "comment", "fx-1", "nice", "work", "--actor")
	if missing.code != 2 {
		t.Fatalf("valued flag missing its value: wanted exit 2, got %d (%s)", missing.code, missing.errw)
	}
	if missing.errw != wantMissing {
		t.Errorf("missing-value refusal:\n got  %q\n want %q", missing.errw, wantMissing)
	}

	// A refusal not raised by parseArgs's two flag-scan sites carries no
	// hint: extract with no target names dinah.usage but is unrelated to a
	// dash-prefixed word.
	unrelated := runCLI(t, root, "extract")
	if strings.Contains(unrelated.errw, "Dinah reads a word starting with two dashes") {
		t.Errorf("extract's own usage refusal should not carry the dash hint, got %q", unrelated.errw)
	}
	unrelatedConfig := runCLI(t, root, "config", "bogus")
	if strings.Contains(unrelatedConfig.errw, "Dinah reads a word starting with two dashes") {
		t.Errorf("config's own usage refusal should not carry the dash hint, got %q", unrelatedConfig.errw)
	}
}

// TestTrailingDomainFlagStillApplies asserts dinah-100's AC-6: add's title
// and block's reason keep applying their own documented flag exactly as
// before when it trails the free text, now a single quoted word.
func TestTrailingDomainFlagStillApplies(t *testing.T) {
	root := newBench(t)

	got := runCLI(t, root, "add", "the rollout failed because of", "--column", "doing")
	if got.code != 0 {
		t.Fatalf("add trailing --column: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	added := showDetail(t, root, "fx-1")
	if added.Card.Title != "the rollout failed because of" {
		t.Errorf("title: wanted %q, got %q", "the rollout failed because of", added.Card.Title)
	}
	if added.Card.ColumnTitle != "Doing" {
		t.Errorf("column: wanted Doing, got %q", added.Card.ColumnTitle)
	}

	got = runCLI(t, root, "block", "fx-1", "the rollout failed", "--kind", "external")
	if got.code != 0 {
		t.Fatalf("block trailing --kind: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	blocked := showDetail(t, root, "fx-1")
	if blocked.Card.BlockReason != "the rollout failed" {
		t.Errorf("reason: wanted %q, got %q", "the rollout failed", blocked.Card.BlockReason)
	}
	if blocked.Card.BlockKind != "external" {
		t.Errorf("kind: wanted external, got %q", blocked.Card.BlockKind)
	}
}

// TestNonTrailingDomainFlagIsLiteral asserts dinah-100's AC-6, its leading
// half: a command's own domain flag typed before the free text applies
// exactly as the same flag typed after it does (TestTrailingDomainFlagStillApplies),
// with neither position swallowing the flag into the free text or the free
// text into the flag's value. dinah-96's own case (a flag-shaped word
// literal inside a multi-word free-text zone) no longer arises: the zone is
// gone now that the slot takes exactly one word, so any additional unquoted
// word, flag-shaped or not, is what dinah.multiple-words refuses instead
// (TestOpenTailKeepsAcceptingExactlyOneWord).
func TestNonTrailingDomainFlagIsLiteral(t *testing.T) {
	root := newBench(t)

	got := runCLI(t, root, "add", "--column", "doing", "the rollout failed")
	if got.code != 0 {
		t.Fatalf("add with a leading --column: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	added := showDetail(t, root, "fx-1")
	if added.Card.Title != "the rollout failed" {
		t.Errorf("title: wanted %q, got %q", "the rollout failed", added.Card.Title)
	}
	if added.Card.ColumnTitle != "Doing" {
		t.Errorf("column: wanted Doing, got %q", added.Card.ColumnTitle)
	}

	runCLI(t, root, "add", "a card to block")
	got = runCLI(t, root, "block", "fx-2", "--kind", "external_dep", "the door jammed")
	if got.code != 0 {
		t.Fatalf("block with a leading --kind: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	blocked := showDetail(t, root, "fx-2")
	if blocked.Card.BlockReason != "the door jammed" {
		t.Errorf("reason: wanted %q, got %q", "the door jammed", blocked.Card.BlockReason)
	}
	if blocked.Card.BlockKind != "external_dep" {
		t.Errorf("kind: wanted external_dep, got %q", blocked.Card.BlockKind)
	}
}

// TestCommentAndConfigSetDeclareNoFlagOfTheirOwn asserts dinah-100 leaves
// nothing for a global flag name to consume once it sits inside the one
// quoted free-text word: comment's text and config set's value store a
// --prefixed substring verbatim, because a quoted word is never split into
// several argv words for the flag scan to see, whatever it starts with
// internally. Neither command declares a domain flag of its own, so this is
// the only shape the old dinah-96 guarantee still has to hold in.
func TestCommentAndConfigSetDeclareNoFlagOfTheirOwn(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card")

	got := runCLI(t, root, "comment", "fx-1", "please --column deploy done")
	if got.code != 0 {
		t.Fatalf("comment: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	detail := showDetail(t, root, "fx-1")
	if len(detail.Comments) != 1 || detail.Comments[0].Body != "please --column deploy done" {
		t.Fatalf("wanted one comment %q, got %v", "please --column deploy done", detail.Comments)
	}

	_, dir := settingsHome(t)
	got = runCLI(t, dir, "config", "set", "actor", "please --column deploy done")
	if got.code != 0 {
		t.Fatalf("config set: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	got = runCLI(t, dir, "config", "get", "actor")
	if got.code != 0 || strings.TrimSpace(got.out) != "please --column deploy done" {
		t.Errorf("config get actor: wanted %q, got %d %q", "please --column deploy done", got.code, got.out)
	}
}

// TestFlagBeforeTheFreeTextBoundaryIsUnaffected asserts dinah-96's AC-13: a
// flag captured before an open-tail command's free-text boundary, meaning
// before the command name or between the command name and its bounded
// slots, keeps applying exactly as it does today.
func TestFlagBeforeTheFreeTextBoundaryIsUnaffected(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card")

	bench := soleBenchDir(t, root)
	got := runCLI(t, root, "--workbench", bench, "comment", "fx-1", "hello")
	if got.code != 0 {
		t.Fatalf("--workbench before the command name: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	if strings.Contains(got.errw, "was not understood") {
		t.Errorf("--workbench before the command name should apply, not refuse: %q", got.errw)
	}

	got = runCLI(t, root, "comment", "--json", "fx-1", "hello")
	if got.code != 0 {
		t.Fatalf("--json before the card slot: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	if !json.Valid([]byte(got.out)) {
		t.Errorf("--json before the card slot should still apply as a flag, got %q", got.out)
	}
}

// TestAValueStarvedFlagRefusesRatherThanFallingThrough asserts dinah-100's
// own D-3 shape for this case: a domain flag that expects a value, with
// nothing left in the free text to serve as that value, is no longer
// spliced back in as literal text (dinah-96's AC-14/D-5 behavior). Once
// resolveOpenTailFlags stops running for add, block and comment, the flag
// is recognized eagerly wherever it is typed, so a value-starved --kind
// consumes the flag and leaves the reason empty, surfacing as no-reason
// instead.
func TestAValueStarvedFlagRefusesRatherThanFallingThrough(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "a card to block")

	got := runCLI(t, root, "block", "fx-1", "--kind")
	if got.code != 2 {
		t.Fatalf("block with only a value-starved --kind: wanted exit 2, got %d (%s)", got.code, got.errw)
	}
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.NoReason {
		t.Errorf("wanted %s (no reason given, not the flag name landing in the reason), got %q", contract.NoReason, got.errw)
	}

	wantUsage(t, runCLI(t, root, "move", "fx-1", "doing", "--column"), "--column")
}

// TestANonTrailingFlagNowReachesValidationInsteadOfLiteralText asserts the
// reverse of dinah-96's AC-18/D-7, dinah-100 (D-3): add's own domain flag is
// read as a flag wherever it is typed relative to the one-word free text,
// leading or trailing, so a bogus value now reaches add's own downstream
// column check and refuses, in place of the literal text dinah-96 would have
// accepted for the same input.
func TestANonTrailingFlagNowReachesValidationInsteadOfLiteralText(t *testing.T) {
	root := newBench(t)

	got := runCLI(t, root, "add", "--column", "bogus", "the rollout failed")
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if got.code == 0 || leading != contract.UnknownColumn {
		t.Errorf("add with a leading bogus --column: wanted %s, got %d (%s)", contract.UnknownColumn, got.code, got.errw)
	}
}

// refusalResidue matches an unfilled named slot, which is what internal/msg
// leaves behind when nothing supplied a value the entry asked for.
var refusalResidue = regexp.MustCompile(`\{[A-Za-z][A-Za-z0-9_-]*\}`)

// refusalDoubleSpace matches a run of two or more spaces, which is what an
// empty fill leaves in the middle of a sentence.
var refusalDoubleSpace = regexp.MustCompile(`  +`)

// checkRefusalShape reads the error stream of every invocation the package
// makes and holds it to the shape a refusal has: the name, then a sentence, on
// a first line with no hole in it, over rows and a next-step line that carry
// no unfilled slot.
//
// It is the output-side half of the one-way rule and it is the load-bearing
// half. A source scan cannot see a helper that writes through its own stream
// parameter, and it cannot see composed text ridden into a translated message
// as a substitution value, which is the route the composer takes by
// construction. Reading the bytes holds whatever route drew them.
//
// It stands down on a zero exit, where an error stream carries a warning
// beside a successful act rather than a refusal.
//
// Two bounds are worth stating. The double-space rule is a proxy rather than
// the property itself, since a card title, a path or a column name a caller
// typed could carry a run of its own; it holds because the corpus supplies
// every input the package's tests use, and it is a rule about this corpus
// rather than about the tool. And the check sees only the refusals some test
// provokes: TestTheRemainingRefusalsLeadStderr names four that no test here
// can, and those four reach the source-side guard alone.
func checkRefusalShape(t *testing.T, code int, text string) {
	t.Helper()
	if code == 0 || strings.TrimSpace(text) == "" {
		return
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for _, finding := range refusalShapeFindings(lines) {
		t.Errorf("the error stream: %s\n%s", finding, strings.Join(lines, "\n"))
	}
}

// refusalShapeFindings reports every way a block fails the shape, so that the
// arming test can hand it a broken block and read what it says rather than
// having to provoke one through the tool.
func refusalShapeFindings(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	var found []string
	name, sentence, _ := strings.Cut(lines[0], " ")
	if !contract.NameIsLegal(name) && name != contract.OutcomeStale && name != contract.OutcomeUnreachable {
		found = append(found, "leads with "+strconv.Quote(name)+", which is neither a refusal name nor an outcome token")
	}
	if strings.TrimSpace(sentence) == "" {
		found = append(found, "carries the name and no sentence after it")
	}
	if strings.TrimSpace(sentence) == "{refusal."+name+"}" {
		found = append(found, "carries the unrendered catalog key rather than the sentence")
	}
	if refusalDoubleSpace.MatchString(lines[0]) {
		found = append(found, "carries a run of two or more spaces on its first line, which is what an empty fill leaves behind")
	}
	for _, line := range lines {
		if slot := refusalResidue.FindString(line); slot != "" {
			found = append(found, "carries the unfilled slot "+slot)
			break
		}
	}
	return found
}

// TestCheckRefusalShapeReportsABrokenBlock arms the check above. A check that
// passes proves nothing on its own, because it also passes when what it guards
// is absent, so this hands it a block with an empty fill and a block with an
// unfilled slot and requires it to report each.
func TestCheckRefusalShapeReportsABrokenBlock(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name:  "an empty fill",
			lines: []string{contract.NotHolder + " you do not hold this card;  does"},
			want:  "run of two or more spaces",
		},
		{
			name:  "an unfilled slot",
			lines: []string{contract.Malformed + " title is missing; write the command as `dinah {usage}`"},
			want:  "unfilled slot {usage}",
		},
		{
			name:  "a leading token that is no refusal",
			lines: []string{"whoops something went wrong"},
			want:  "neither a refusal name nor an outcome token",
		},
		{
			name:  "the name with no sentence",
			lines: []string{contract.Terminal},
			want:  "no sentence after it",
		},
		{
			name:  "the unrendered catalog key",
			lines: []string{contract.Terminal + " {refusal.terminal}"},
			want:  "unrendered catalog key",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			found := refusalShapeFindings(tt.lines)
			if len(found) == 0 {
				t.Fatalf("the check passed a block it should have reported: %q", tt.lines)
			}
			joined := strings.Join(found, "; ")
			if !strings.Contains(joined, tt.want) {
				t.Errorf("the report should name %q, got %q", tt.want, joined)
			}
		})
	}
	clean := []string{
		contract.NotHolder + " you do not hold this card; alka does; run `dinah whoami` to see who Dinah takes you to be, or ask alka to release it",
		"  intake",
		"run `dinah ls` with one of them",
	}
	if found := refusalShapeFindings(clean); len(found) != 0 {
		t.Errorf("the check reported a well-formed block: %v", found)
	}
}

// TestARefusalReachesItsReaderInTheirOwnLanguage asserts the two findings that
// leaked English into a translated sentence: a flag-parse refusal raised
// before a session exists, and the free-text slot name spliced in as a Go
// literal.
//
// The Hindi is compared against hi.json's own entries rather than against the
// absence of English, because the shipped catalog keeps the product name, the
// command spellings and the flag spellings in Latin script on purpose.
func TestARefusalReachesItsReaderInTheirOwnLanguage(t *testing.T) {
	root := newBench(t)
	hindi := msg.For("hi")
	english := msg.For(msg.Base)

	got := runCLI(t, root, "--lang", "hi", "add", "--nosuchflag", "Thing")
	if got.code != 2 {
		t.Fatalf("a mistyped flag exited %d, wanted 2", got.code)
	}
	want := contract.Usage + " " +
		hindi.T("refusal.dinah.usage", "detail", "--nosuchflag") +
		hindi.T("refusal.dinah.usage.next") +
		hindi.T("refusal.dinah.usage.dash-hint") + "\n"
	if got.errw != want {
		t.Errorf("the parse refusal should render from the Hindi catalog, in three pieces:\n got  %q\n want %q", got.errw, want)
	}
	inEnglish := runCLI(t, root, "add", "--nosuchflag", "Thing")
	if inEnglish.errw == got.errw {
		t.Errorf("the Hindi and English renderings of the same refusal are the same bytes: %q", got.errw)
	}
	// A catalog that gained the fragment and kept the clause in its base entry
	// would satisfy every comparison above character for character, and print
	// the clause twice.
	if n := strings.Count(got.errw, hindi.T("refusal.dinah.usage.next")); n != 1 {
		t.Errorf("the next step should appear once, got %d times in %q", n, got.errw)
	}
	if n := strings.Count(got.errw, "dinah help"); n != 1 {
		t.Errorf("`dinah help` should appear once, got %d times in %q", n, got.errw)
	}

	words := runCLI(t, root, "--lang", "hi", "add", "Test", "card", "here")
	if words.code != 2 {
		t.Fatalf("a multi-word title exited %d, wanted 2", words.code)
	}
	label := hindi.T("slot.title")
	if label == english.T("slot.title") {
		t.Fatalf("the fixture is not testing anything: hi and en render slot.title the same way")
	}
	if !strings.Contains(words.errw, hindi.T("refusal.dinah.multiple-words", "count", "3", "label", label)) {
		t.Errorf("the free-text slot name should come from the Hindi catalog, got %q", words.errw)
	}
	if strings.Contains(words.errw, english.T("slot.title")) {
		t.Errorf("the English slot name leaked into the Hindi sentence: %q", words.errw)
	}
}

// TestARefusalWithAnAbsentSubjectSaysSomethingElse asserts that a refusal
// whose subject is missing renders the sentence written for that case rather
// than the sentence with a hole in it, and that each branch gets the next step
// written for its own reader.
func TestARefusalWithAnAbsentSubjectSaysSomethingElse(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "Something"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")
	english := msg.For(msg.Base)

	unheld := runCLI(t, root, "release", "fx-1")
	if unheld.code != 2 {
		t.Fatalf("releasing a card nobody holds exited %d, wanted 2", unheld.code)
	}
	wantUnheld := contract.NotHolder + " " +
		english.T("refusal.not-holder.unnamed") +
		english.T("refusal.not-holder.next-unheld", "card", "fx-1") + "\n"
	if unheld.errw != wantUnheld {
		t.Errorf("a card nobody holds:\n got  %q\n want %q", unheld.errw, wantUnheld)
	}

	t.Setenv("DINAH_ACTOR", "alka")
	if got := runCLI(t, root, "claim", "fx-1"); got.code != 0 {
		t.Fatalf("claim: %d %s", got.code, got.errw)
	}
	t.Setenv("DINAH_ACTOR", "somebody-else")
	held := runCLI(t, root, "release", "fx-1")
	if held.code != 2 {
		t.Fatalf("releasing somebody else's card exited %d, wanted 2", held.code)
	}
	wantHeld := contract.NotHolder + " " +
		english.T("refusal.not-holder", "detail", "alka") +
		english.T("refusal.not-holder.next", "detail", "alka") + "\n"
	if held.errw != wantHeld {
		t.Errorf("a card somebody else holds:\n got  %q\n want %q", held.errw, wantHeld)
	}
	if strings.Contains(held.errw, english.T("refusal.not-holder.next-unheld", "card", "fx-1")) {
		t.Errorf("the unheld branch's advice reached a reader whose card somebody holds: %q", held.errw)
	}
	if strings.Contains(unheld.errw, english.T("refusal.not-holder.next", "detail", "alka")) {
		t.Errorf("the held branch's advice reached a reader whose card nobody holds: %q", unheld.errw)
	}
}

// TestANamedValueSurvivesTheVerbLayer asserts that a refusal raised below a
// verb over a file on disk still names that file by the time it reaches a
// reader, on the rendering and on the machine surface alike. The named values
// used to be dropped on the way up, because verb.Response had nowhere to put
// them.
func TestANamedValueSurvivesTheVerbLayer(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "Something"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	missing := filepath.Join(root, "nosuchfile.txt")

	got := runCLI(t, root, "attach", "fx-1", missing)
	if got.code != 2 {
		t.Fatalf("attaching a file that is not there exited %d, wanted 2", got.code)
	}
	if !strings.Contains(got.errw, contract.UnknownPath) {
		t.Fatalf("wanted %s, got %q", contract.UnknownPath, got.errw)
	}
	if !strings.Contains(got.errw, msg.For(msg.Base).T("refusal.dinah.unknown-path.next-file", "file", missing)) {
		t.Errorf("the reader who named a path should be sent to check that path, got %q", got.errw)
	}
	machine := runCLI(t, root, "--json", "attach", "fx-1", missing)
	if !strings.Contains(machine.out, `"context"`) || !strings.Contains(machine.out, `"file"`) {
		t.Errorf("the machine form should carry the named value the sentence used, got %q", machine.out)
	}
}

// TestMalformedAnswersEachOfItsThreeReaders asserts the three branches one
// refusal name covers: a workbench anchor the reader edits and confirms with
// dinah check, a definition file the reader edits with no workbench to confirm
// against, and a request argument nobody edits at all.
func TestMalformedAnswersEachOfItsThreeReaders(t *testing.T) {
	english := msg.For(msg.Base)

	t.Run("a workbench anchor", func(t *testing.T) {
		root := newBench(t)
		editAnchor(t, root, "profile: "+bench.ProfileVersion+"\n", "")
		got := runCLI(t, root, "whoami")
		if got.code != 2 {
			t.Fatalf("a broken anchor exited %d, wanted 2", got.code)
		}
		for _, want := range []string{", in ", english.T("refusal.malformed.fix")} {
			if !strings.Contains(got.errw, want) {
				t.Errorf("the anchor-side refusal should carry %q, got %q", want, got.errw)
			}
		}
		if strings.Contains(got.errw, english.T("refusal.malformed.next-file", "file", "x")) {
			t.Errorf("the anchor-side refusal took the definition-file repair: %q", got.errw)
		}
		if strings.Contains(got.errw, "write the command as") {
			t.Errorf("the anchor-side refusal carries two next steps: %q", got.errw)
		}
	})

	t.Run("a definition file", func(t *testing.T) {
		root := newBench(t)
		template := filepath.Join(root, "template.json")
		if err := os.WriteFile(template, []byte(`{"profile":"dinah/1.0","title":"a template","columns":[]}`), 0o644); err != nil {
			t.Fatalf("write the template: %v", err)
		}
		elsewhere := filepath.Join(t.TempDir(), "target")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		got := runCLI(t, elsewhere, "init", "--from", template)
		if got.code != 2 {
			t.Fatalf("init from a broken template exited %d, wanted 2", got.code)
		}
		want := english.T("refusal.malformed.in-file", "file", template) +
			english.T("refusal.malformed.next-file", "file", template)
		if !strings.Contains(got.errw, want) {
			t.Errorf("the definition-file refusal should name the file and its repair:\n got  %q\n want it to carry %q", got.errw, want)
		}
		if strings.Contains(got.errw, "dinah check") {
			t.Errorf("the definition-file repair points at a workbench init never made: %q", got.errw)
		}
	})

	t.Run("a request argument", func(t *testing.T) {
		root := newBench(t)
		if got := runCLI(t, root, "add", "Something"); got.code != 0 {
			t.Fatalf("add: %d %s", got.code, got.errw)
		}
		for _, tt := range []struct {
			argv    []string
			command string
		}{
			{argv: []string{"add"}, command: "add"},
			{argv: []string{"comment", "fx-1"}, command: "comment"},
		} {
			got := runCLI(t, root, tt.argv...)
			want := english.T("refusal.malformed.next", "usage", verb.Usage(tt.command))
			if !strings.Contains(got.errw, want) {
				t.Errorf("%v should be told how the command is spelled:\n got  %q\n want it to carry %q", tt.argv, got.errw, want)
			}
			if strings.Contains(got.errw, ", in ") {
				t.Errorf("%v named a file it has none of: %q", tt.argv, got.errw)
			}
		}
	})
}

// TestAMembershipRefusalPrintsWhatTheToolAccepts asserts that the three
// refusals testing membership against a set the tool can enumerate print that
// set, and that the two whose set is unbounded print none.
func TestAMembershipRefusalPrintsWhatTheToolAccepts(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "Something"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	english := msg.For(msg.Base)

	t.Run("the columns a workbench declares", func(t *testing.T) {
		got := runCLI(t, root, "ls", "nowhere")
		if got.code != 2 {
			t.Fatalf("listing an unknown column exited %d, wanted 2", got.code)
		}
		for _, want := range []string{"  intake", "  doing", "  done"} {
			if !strings.Contains(got.errw, want+"\n") {
				t.Errorf("the listing should carry %q, got %q", want, got.errw)
			}
		}
		if !strings.HasSuffix(got.errw, english.T("refusal.unknown-column.next", "command", "ls")+"\n") {
			t.Errorf("the next step should name the command the reader typed, got %q", got.errw)
		}
		moved := runCLI(t, root, "move", "fx-1", "nowhere")
		if !strings.HasSuffix(moved.errw, english.T("refusal.unknown-column.next", "command", "move")+"\n") {
			t.Errorf("the next step should name move from a move, got %q", moved.errw)
		}
	})

	t.Run("the settings this tool knows", func(t *testing.T) {
		get := runCLI(t, root, "config", "get", "nosuch")
		set := runCLI(t, root, "config", "set", "nosuch", "value")
		if get.errw != set.errw {
			t.Errorf("the same refusal reads differently under get and set:\n get %q\n set %q", get.errw, set.errw)
		}
		for _, key := range bench.ConfigKeys {
			if !strings.Contains(get.errw, "  "+key+"\n") {
				t.Errorf("the listing should carry %q, got %q", key, get.errw)
			}
		}
		if !strings.HasSuffix(get.errw, english.T("refusal.dinah.unknown-key.next")+"\n") {
			t.Errorf("the next step should follow the rows on its own line, got %q", get.errw)
		}
	})

	t.Run("the guides Dinah carries", func(t *testing.T) {
		got := runCLI(t, root, "guide", "nosuch")
		if got.code != 2 {
			t.Fatalf("an unknown guide exited %d, wanted 2", got.code)
		}
		// The rows are held in order as well as in membership. This refusal
		// is one of the surfaces the declared reading order governs, and a
		// per-topic containment check passes whatever order they stand in.
		var listed []string
		for _, line := range strings.Split(got.errw, "\n") {
			if trimmed, found := strings.CutPrefix(line, "  "); found {
				listed = append(listed, trimmed)
			}
		}
		// The count is asserted before the positions are, because indexing a
		// longer list than the reading order holds finds every topic in its
		// place and says nothing about the row that was added beside them.
		if len(listed) != len(guide.Topics()) {
			t.Errorf("the refusal indents %d rows and the reading order holds %d topics: %v", len(listed), len(guide.Topics()), listed)
		}
		for at, topic := range guide.Topics() {
			if !strings.Contains(got.errw, "  "+topic+"\n") {
				t.Errorf("the listing should carry %q, got %q", topic, got.errw)
				continue
			}
			if at >= len(listed) || listed[at] != topic {
				t.Errorf("the listing stands in %v and the reading order places %q at position %d", listed, topic, at)
			}
		}
		if !strings.HasSuffix(got.errw, english.T("refusal.dinah.unknown-guide.next")+"\n") {
			t.Errorf("the next step should follow the rows on its own line, got %q", got.errw)
		}
	})

	t.Run("the sets nobody prints", func(t *testing.T) {
		// A card set and a workbench path are both unbounded, so printing
		// either is noise rather than help. A mistyped command name prints
		// none either, which is the operator's ruling on this card: thirty
		// bare names beside the grouped listing dinah help already prints is
		// the same judgement.
		for _, tt := range []struct {
			argv []string
			next string
		}{
			{argv: []string{"show", "fx-99"}, next: "refusal.unknown-card.next"},
			{argv: []string{"show", "fx-1/nosuch"}, next: "refusal.dinah.unknown-path.next"},
			{argv: []string{"frobnicate"}, next: "refusal.dinah.unknown-command.next"},
		} {
			got := runCLI(t, root, tt.argv...)
			lines := strings.Split(strings.TrimRight(got.errw, "\n"), "\n")
			if len(lines) != 1 {
				t.Errorf("%v should refuse in one line and print no listing, got %q", tt.argv, got.errw)
			}
			if !strings.HasSuffix(got.errw, english.T(tt.next)+"\n") {
				t.Errorf("%v should still say what to do next, got %q", tt.argv, got.errw)
			}
		}
	})
}

// TestWorkbenchListsReadsAndWritesItsOwnFields asserts the command's whole
// grammar at the terminal: the bare listing carries the three field names with
// their stored values, the machine form of the same invocation is the object
// and no table, `get` prints one raw value under every rendering, and `set`
// round-trips a value while leaving everything else in the anchor alone.
func TestWorkbenchListsReadsAndWritesItsOwnFields(t *testing.T) {
	root := newBench(t)
	anchor := filepath.Join(benchDir(t, root), "workbench.md")

	listed := runCLI(t, root, "workbench")
	if listed.code != 0 {
		t.Fatalf("the listing: %d %s", listed.code, listed.errw)
	}
	for _, fragment := range []string{"title", "slug", "operator", "fx", "alka", "workbench"} {
		if !strings.Contains(listed.out, fragment) {
			t.Errorf("the listing does not carry %q:\n%s", fragment, listed.out)
		}
	}

	machine := runCLI(t, root, "--json", "workbench")
	if machine.code != 0 {
		t.Fatalf("the machine form: %d %s", machine.code, machine.errw)
	}
	var view verb.WorkbenchView
	if err := json.Unmarshal([]byte(machine.out), &view); err != nil {
		t.Fatalf("the machine form should be one object: %v\n%s", err, machine.out)
	}
	if view.Slug != "fx" || view.Operator != "alka" || view.Title == "" {
		t.Errorf("the machine form reads %+v", view)
	}
	if strings.Contains(machine.out, "----") {
		t.Errorf("the machine form drew a table:\n%s", machine.out)
	}

	// `get` prints the stored value alone, with no heading and no padding,
	// under the default rendering, under another language, and under --json.
	for _, argv := range [][]string{
		{"workbench", "get", "slug"},
		{"--lang", "hi", "workbench", "get", "slug"},
		{"--json", "workbench", "get", "slug"},
	} {
		got := runCLI(t, root, argv...)
		if got.code != 0 {
			t.Fatalf("%v: %d %s", argv, got.code, got.errw)
		}
		if got.out != "fx\n" {
			t.Errorf("%v printed %q, wanted the stored value alone", argv, got.out)
		}
	}

	before, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if wrote := runCLI(t, root, "workbench", "set", "title", "Dinah, the tool"); wrote.code != 0 {
		t.Fatalf("set title: %d %s", wrote.code, wrote.errw)
	}
	if got := runCLI(t, root, "workbench", "get", "title"); got.out != "Dinah, the tool\n" {
		t.Errorf("the title read back as %q", got.out)
	}
	after, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	for _, key := range []string{"profile:", "format:", "columns:"} {
		if !strings.Contains(string(after), key) {
			t.Errorf("the write dropped the %s key from the anchor:\n%s", key, after)
		}
	}
	if !strings.Contains(string(before), "title: Fixture") && !strings.Contains(string(before), "title:") {
		t.Errorf("the fixture anchor carried no title to rewrite:\n%s", before)
	}

	// An unquoted multi-word value refuses, and the line it offers reads back.
	multiple := runCLI(t, root, "workbench", "set", "title", "Dinah,", "the", "tool")
	if multiple.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Errorf("an unquoted value: wanted the refused exit code, got %d", multiple.code)
	}
	if !strings.Contains(multiple.errw, contract.MultipleWords) {
		t.Errorf("an unquoted value: wanted %s, got %q", contract.MultipleWords, multiple.errw)
	}
	if !strings.Contains(multiple.errw, `dinah workbench set title "Dinah, the tool"`) {
		t.Errorf("the rebuilt command line does not read back:\n%s", multiple.errw)
	}

	// An empty value refuses on each of the three fields and writes nothing.
	held, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	for _, field := range bench.WorkbenchFields {
		got := runCLI(t, root, "workbench", "set", field, "")
		if got.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Errorf("an empty %s: wanted the refused exit code, got %d", field, got.code)
		}
		if leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]; leading != contract.Malformed {
			t.Errorf("an empty %s: wanted %s, got %q", field, contract.Malformed, got.errw)
		}
		if !strings.Contains(got.errw, field) {
			t.Errorf("an empty %s: the refusal does not name the field: %q", field, got.errw)
		}
	}
	unchanged, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if string(unchanged) != string(held) {
		t.Error("a refused write reached the anchor")
	}
}

// TestWorkbenchListingNamesTheRepairForAMissingSlug asserts that a workbench
// written before the slug field existed draws its slug row through the helper
// the columns and workbenches listings already use, so all three say the same
// thing rather than one of them padding an empty string.
func TestWorkbenchListingNamesTheRepairForAMissingSlug(t *testing.T) {
	root := newBench(t)
	editAnchor(t, root, "slug: fx\n", "")
	listed := runCLI(t, root, "workbench")
	if listed.code != 0 {
		t.Fatalf("the listing: %d %s", listed.code, listed.errw)
	}
	placeholder := msg.For(msg.Base).T("slug.missing")
	if !strings.Contains(listed.out, placeholder) {
		t.Errorf("the slug row does not name the repair %q:\n%s", placeholder, listed.out)
	}
}

// TestWorkbenchRefusesAFieldItDoesNotRecord asserts that a field name outside
// the three refuses on the read and on the write alike, that the anchor is
// untouched either way, and that all three paths, including `config get`,
// render one sentence, which is what keeps them from drifting apart.
func TestWorkbenchRefusesAFieldItDoesNotRecord(t *testing.T) {
	root := newBench(t)
	anchor := filepath.Join(benchDir(t, root), "workbench.md")
	before, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	sentences := map[string]bool{}
	for _, argv := range [][]string{
		{"workbench", "get", "profile"},
		{"workbench", "set", "profile", "dinah-core/1.0"},
		{"config", "get", "profile"},
	} {
		got := runCLI(t, root, argv...)
		if got.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Errorf("%v: wanted the refused exit code, got %d", argv, got.code)
		}
		leading, sentence, _ := strings.Cut(strings.TrimSpace(got.errw), " ")
		if leading != contract.UnknownKey {
			t.Errorf("%v: wanted %s, got %q", argv, contract.UnknownKey, got.errw)
		}
		sentences[sentence] = true
	}
	if len(sentences) != 1 {
		t.Errorf("the three paths render %d different sentences, wanted one: %v", len(sentences), sentences)
	}
	after, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if string(after) != string(before) {
		t.Error("a refused write reached the anchor")
	}
}

// TestRenamingTheSlugAsksOnceAndLeavesTheOldReferenceResolving asserts the
// rename: the first attempt refuses under the name a script tests and says what
// the rename costs, the confirmed attempt renames every card at once, a
// reference carrying the old prefix still resolves, and `delete` keeps the
// sentence it has always printed under the same refusal name.
func TestRenamingTheSlugAsksOnceAndLeavesTheOldReferenceResolving(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "the first card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "the second card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}

	unconfirmed := runCLI(t, root, "workbench", "set", "slug", "fx-dev")
	if unconfirmed.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("the first attempt: wanted the refused exit code, got %d", unconfirmed.code)
	}
	leading, renameSentence, _ := strings.Cut(strings.TrimSpace(unconfirmed.errw), " ")
	if leading != contract.Unconfirmed {
		t.Errorf("the first attempt: wanted %s, got %q", contract.Unconfirmed, unconfirmed.errw)
	}
	if !strings.Contains(renameSentence, "fx-dev") {
		t.Errorf("the sentence does not name the new slug: %q", renameSentence)
	}
	if !strings.Contains(renameSentence, "--yes") {
		t.Errorf("the sentence does not say how to go on: %q", renameSentence)
	}
	if got := runCLI(t, root, "workbench", "get", "slug"); got.out != "fx\n" {
		t.Errorf("the refused rename wrote the slug anyway: %q", got.out)
	}

	// The same refusal name under delete keeps its own sentence, in every
	// shipped catalog, because delete adds no per-command key of its own.
	deleted := runCLI(t, root, "delete", "fx-1")
	_, deleteSentence, _ := strings.Cut(strings.TrimSpace(deleted.errw), " ")
	if deleteSentence == renameSentence {
		t.Error("delete and the rename render one sentence, so the per-command selection is not running")
	}
	tags := msg.Tags()
	if len(tags) == 0 {
		t.Fatal("no catalogs to check, so the per-catalog claim below proves nothing")
	}
	for _, tag := range tags {
		if !msg.For(tag).Has("refusal." + contract.Unconfirmed) {
			t.Errorf("%s carries no sentence for %s", tag, contract.Unconfirmed)
		}
		// What delete renders in this catalog is the bare sentence, and it is
		// the bare sentence only while no per-command key exists to displace
		// it: refusalSentence prefers refusal.<name>.<verb> wherever the
		// catalog carries one. Asserting the absence is what carries the
		// unchanged claim, since a non-empty bare entry survives a rewrite.
		if msg.For(tag).Has("refusal." + contract.Unconfirmed + ".delete") {
			t.Errorf("%s now carries a per-command sentence for delete, so delete no longer renders the sentence it always has", tag)
		}
	}

	// The flag is read as a flag whether it is typed before the value or
	// after it, and neither position swallows the other.
	if got := runCLI(t, root, "workbench", "set", "slug", "fx-dev", "--yes"); got.code != 0 {
		t.Fatalf("the flag after the value: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "workbench", "get", "slug"); got.out != "fx-dev\n" {
		t.Errorf("the slug read back as %q", got.out)
	}
	if got := runCLI(t, root, "workbench", "set", "slug", "--yes", "fx-later"); got.code != 0 {
		t.Fatalf("the flag before the value: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "workbench", "get", "slug"); got.out != "fx-later\n" {
		t.Errorf("the slug read back as %q, so a position swallowed the flag or the value", got.out)
	}

	listed := runCLI(t, root, "ls")
	for _, ref := range []string{"fx-later-1", "fx-later-2"} {
		if !strings.Contains(listed.out, ref) {
			t.Errorf("the listing does not report %s under the new prefix:\n%s", ref, listed.out)
		}
	}
	stale := runCLI(t, root, "show", "fx-1")
	if stale.code != 0 {
		t.Errorf("a reference carrying the old prefix stopped resolving: %d %s", stale.code, stale.errw)
	}
}

// TestPathAndEditReachTheWorkbenchAnchor asserts that the two spellings
// ResolveEntity has always accepted now reach the anchor through ResolvePath
// too, on one line and with nothing around it, and that the empty reference
// keeps refusing because both commands declare the argument required.
func TestPathAndEditReachTheWorkbenchAnchor(t *testing.T) {
	root := newBench(t)
	// The head resolves its root through its own working directory, so the
	// expectation is built from the directory the head itself reports rather
	// than from the raw value t.TempDir() handed this test (see resolvedDir).
	wanted := filepath.Join(resolvedDir(t, benchDir(t, root)), "workbench.md")
	for _, ref := range []string{"workbench", "."} {
		got := runCLI(t, root, "path", ref)
		if got.code != 0 {
			t.Fatalf("path %s: %d %s", ref, got.code, got.errw)
		}
		if got.out != wanted+"\n" {
			t.Errorf("path %s printed %q, wanted %q on one line", ref, got.out, wanted)
		}
	}
	// edit walks the same resolver, so it stops refusing the reference. The
	// editor is pointed at a name no machine carries, which lets the run reach
	// the launch and fail there rather than opening a window this suite would
	// then wait on: what is asserted is that the refusal is no longer the
	// resolver's.
	t.Setenv("DINAH_EDITOR", "dinah-no-such-editor")
	edited := runCLI(t, root, "edit", "workbench")
	if strings.Contains(edited.errw, contract.UnknownCard) {
		t.Errorf("edit still refuses the workbench reference: %s", edited.errw)
	}

	bare := runCLI(t, root, "path")
	if bare.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Errorf("a bare path: wanted the refused exit code, got %d", bare.code)
	}
	if leading := strings.SplitN(strings.TrimSpace(bare.errw), " ", 2)[0]; leading != contract.UnknownCard {
		t.Errorf("a bare path: wanted %s, got %q", contract.UnknownCard, bare.errw)
	}
}

// TestPathPrintsOneLineForACardWhoseTitleIsNotASCII asserts the guarantee
// runPath's own doc comment states, at the exactness a caller pasting the
// output into another command depends on: one line, the resolved path and
// nothing else, for a card whose stored text carries an em dash (dinah-199).
func TestPathPrintsOneLineForACardWhoseTitleIsNotASCII(t *testing.T) {
	root := newBench(t)
	title := "Ein Bindestrich — und Hindi हिन्दी"
	ref := addCard(t, root, title)
	got := runCLI(t, root, "path", ref)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", ref, got.code, got.errw)
	}
	line, found := strings.CutSuffix(got.out, "\n")
	if !found {
		t.Fatalf("path %s printed %q, wanted a single trailing newline", ref, got.out)
	}
	if strings.ContainsAny(line, "\r\n") || line != strings.TrimSpace(line) {
		t.Errorf("path %s printed %q, wanted the path alone with nothing around it", ref, got.out)
	}
	// The line names the card's own file, which is what makes the assertion
	// above a claim about the resolved path rather than about its shape.
	stored, err := os.ReadFile(line)
	if err != nil {
		t.Fatalf("path %s printed %q, which does not open: %v", ref, line, err)
	}
	if !strings.Contains(string(stored), title) {
		t.Errorf("the file at %q does not carry the title %q the card was filed under", line, title)
	}
	within := resolvedDir(t, benchDir(t, root))
	if !strings.HasPrefix(line, within) {
		t.Errorf("path %s printed %q, wanted a path inside the workbench at %q", ref, line, within)
	}
}

// TestEditHandsTheChildTheRawStreamsWhenItHasThem asserts that the command
// runEdit runs is given the session's unwrapped *os.File values when main
// built the session, so an editor receives a real console handle rather than
// a pipe, and that a session a test built by hand still gets its own streams
// (dinah-199).
func TestEditHandsTheChildTheRawStreamsWhenItHasThem(t *testing.T) {
	dir := t.TempDir()
	rawOut, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("create the stdout stand-in: %v", err)
	}
	defer rawOut.Close()
	rawErr, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatalf("create the stderr stand-in: %v", err)
	}
	defer rawErr.Close()

	wrapped := &session{out: &bytes.Buffer{}, errw: &bytes.Buffer{}, rawOut: rawOut, rawErr: rawErr}
	cmd := editCmd(wrapped, "an-editor", filepath.Join(dir, "card.md"))
	if cmd.Stdout != io.Writer(rawOut) {
		t.Errorf("stdout reached the child as %#v, wanted the raw file itself", cmd.Stdout)
	}
	if cmd.Stderr != io.Writer(rawErr) {
		t.Errorf("stderr reached the child as %#v, wanted the raw file itself", cmd.Stderr)
	}

	out := &bytes.Buffer{}
	errw := &bytes.Buffer{}
	plain := &session{out: out, errw: errw}
	bare := editCmd(plain, "an-editor", filepath.Join(dir, "card.md"))
	if bare.Stdout != io.Writer(out) {
		t.Errorf("stdout reached the child as %#v, wanted the session's own stream", bare.Stdout)
	}
	if bare.Stderr != io.Writer(errw) {
		t.Errorf("stderr reached the child as %#v, wanted the session's own stream", bare.Stderr)
	}
	// The command is built and never run, so nothing here launches an
	// editor or waits on a window.
	var _ *exec.Cmd = bare
}

// TestInitDerivesTheReadableSlugAndRefusesOneThatReadsAsACardReference asserts
// the derivation and the exclusion together: a directory name yielding a dashed
// slug is repaired into the readable form, a name that would yield a card
// reference is repaired instead of refused, a dashed slug typed by hand is
// accepted, and the one shape the grammar excludes is refused with the clause
// that names it.
func TestInitDerivesTheReadableSlugAndRefusesOneThatReadsAsACardReference(t *testing.T) {
	cases := []struct {
		directory string
		slug      string
		wanted    string
		refused   bool
		clause    bool
	}{
		{directory: "Dinah development", wanted: "dinah-development"},
		{directory: "Sprint 2", wanted: "sprint2"},
		{directory: "named", slug: "my-board", wanted: "my-board"},
		{directory: "named", slug: "release-2-candidate", wanted: "release-2-candidate"},
		{directory: "named", slug: "sprint2", wanted: "sprint2"},
		{directory: "named", slug: "sprint-2", refused: true, clause: true},
		{directory: "named", slug: "My Project", refused: true},
	}
	for _, c := range cases {
		name := c.directory
		if c.slug != "" {
			name += " --slug " + c.slug
		}
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
			t.Setenv("DINAH_ACTOR", "alka")
			t.Setenv("DINAH_LANG", "")
			t.Setenv("DINAH_FORMAT", "")
			t.Setenv("DINAH_WORKBENCH", "")
			root := filepath.Join(base, c.directory)
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			argv := []string{"init"}
			if c.slug != "" {
				argv = append(argv, "--slug", c.slug)
			}
			got := runCLI(t, root, argv...)
			if c.refused {
				if got.code != contract.ExitCode(contract.OutcomeRefused) {
					t.Fatalf("wanted the refused exit code, got %d %s", got.code, got.out)
				}
				if leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]; leading != contract.Malformed {
					t.Errorf("wanted %s, got %q", contract.Malformed, got.errw)
				}
				clause := msg.For(msg.Base).T("refusal.malformed.reads-as-a-card-reference", "cardRef", c.slug)
				carried := strings.Contains(got.errw, strings.TrimSpace(clause))
				if carried != c.clause {
					t.Errorf("the card-reference clause carried %v, wanted %v: %q", carried, c.clause, got.errw)
				}
				return
			}
			if got.code != 0 {
				t.Fatalf("init: %d %s", got.code, got.errw)
			}
			if read := runCLI(t, root, "workbench", "get", "slug"); read.out != c.wanted+"\n" {
				t.Errorf("the slug read back as %q, wanted %q", read.out, c.wanted)
			}
		})
	}
}

// TestADashedWorkbenchSlugResolvesEveryReference asserts that the widened
// grammar costs the reference vocabulary nothing: every command that takes a
// card reference resolves one under a dashed prefix, a stale prefix still
// resolves with its warning rather than a refusal, and a workbench whose slug
// carries no dash at all draws no new finding.
func TestADashedWorkbenchSlugResolvesEveryReference(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "check"); got.code != 0 {
		t.Errorf("a dash-free slug drew a finding: %d %s", got.code, got.out)
	}
	if got := runCLI(t, root, "workbench", "set", "slug", "fx-dev", "--yes"); got.code != 0 {
		t.Fatalf("the rename: %d %s", got.code, got.errw)
	}
	for _, argv := range [][]string{
		{"ls"},
		{"show", "fx-dev-1"},
		{"path", "fx-dev-1"},
		{"move", "fx-dev-1", "doing"},
		{"claim", "fx-dev-1"},
		{"release", "fx-dev-1"},
	} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Errorf("%v under a dashed slug: %d %s", argv, got.code, got.errw)
		}
	}
	stale := runCLI(t, root, "move", "fx-1", "done")
	if stale.code != 0 {
		t.Errorf("a stale prefix refused rather than warning: %d %s", stale.code, stale.errw)
	}
	if !strings.Contains(stale.errw, "fx") {
		t.Errorf("a stale prefix carried no warning: %q", stale.errw)
	}
	if got := runCLI(t, root, "check"); got.code != 0 {
		t.Errorf("a dashed slug drew a finding: %d %s", got.code, got.out)
	}
}

// TestAStoredWorkbenchSlugOutsideTheGrammarIsReportedAndStillOpens asserts the
// finding this card adds. The write path refuses such a slug, so the only way
// one reaches disk is a hand edit, and the checker is what says so: the
// workbench keeps opening, every command keeps working, and check reports the
// slug under its own key.
func TestAStoredWorkbenchSlugOutsideTheGrammarIsReportedAndStillOpens(t *testing.T) {
	root := newBench(t)
	editAnchor(t, root, "slug: fx\n", "slug: sprint-2\n")
	anchor := filepath.Join(benchDir(t, root), "workbench.md")

	if got := runCLI(t, root, "workbench", "get", "slug"); got.code != 0 || got.out != "sprint-2\n" {
		t.Fatalf("the workbench should still open: %d %q %s", got.code, got.out, got.errw)
	}
	reported := runCLI(t, root, "check")
	if reported.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Fatalf("check should report the slug: %d %s", reported.code, reported.out)
	}
	wanted := msg.For(msg.Base).T(bench.FindingWorkbenchSlugMalformed, "detail", "sprint-2")
	if !strings.Contains(reported.out, wanted) {
		t.Errorf("check does not report the finding %q:\n%s", wanted, reported.out)
	}
	if !strings.Contains(reported.out, anchor) {
		t.Errorf("the finding does not name the file to edit:\n%s", reported.out)
	}
}

// ratifiedWorkbenchHelp is the block the operator approved, drawn in section 7
// of docs/specs/dinah-141-workbench-fields-ux-sketch.md and grown by the
// sketch dinah-172 carries, which he approved and which adds the arguments
// section to every per-command page. It is quoted here rather than read from
// either sketch because a sketch is a design document that ships once and this
// is the surface, and the eighty-column layout both were drawn at is what an
// unbounded run measures against.
//
// The field row carries this workbench's own three fields, which the block
// reads from bench.WorkbenchFields rather than from the workbench under test,
// so it says the same thing wherever the command is run.
const ratifiedWorkbenchHelp = `workbench [get|set] [field] [value] [--yes]

Read this workbench's own fields, or write one

What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  [get|set]        read one field or write one; every field with its value when
                   you name none
  [field]          which field you are reading or writing (one of: title, slug,
                   operator)
  [value]          what to store in it, on a set
  [--yes]          confirm the act, which Dinah does not carry out without it

What can go wrong, in the order each is checked:
  Order  What can go wrong                            Refusal
  -----  -------------------------------------------  -----------------
  1      the field is one this workbench records      dinah.unknown-key
  2      the value is present and well formed         malformed
  3      on a write, the request names an owner       no-owner
  4      that owner is the operator                   not-operator
  5      a slug rename carries the confirmation flag  dinah.unconfirmed

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
`

// TestWorkbenchHelpIsTheBlockTheOperatorApproved asserts the per-command help
// against the block drawn in the card's UX sketch, byte for byte, including
// the column widths and the rule lengths. The five rows are the command's own
// checks in the order the runtime evaluates them, and the workbench-level
// operator check that runs ahead of all five is deliberately absent, since no
// command outside the five the profile specifies lists one.
func TestWorkbenchHelpIsTheBlockTheOperatorApproved(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "80")
	got := runCLI(t, root, "help", "workbench")
	if got.code != 0 {
		t.Fatalf("help workbench: %d %s", got.code, got.errw)
	}
	if got.out != ratifiedWorkbenchHelp {
		t.Errorf("the emitted block differs from the one the operator approved:\n%s", diffLines(ratifiedWorkbenchHelp, got.out))
	}
	if strings.Contains(got.out, contract.NoOperator) {
		t.Error("the block lists the workbench-level operator check, which no beyond-contract command lists")
	}
}

// configPath is the configuration file a user base holds, which is the file
// `init` records an actor in.
func configPath(home string) string {
	return filepath.Join(home, bench.UserBaseName, bench.ConfigName)
}

// configBytes reads the configuration file whole, returning nil when it cannot
// be read at all, which on these paths means it is not there.
//
// The nil does not by itself separate an absent file from an empty one, since
// bytes.Equal reads nil and no bytes alike, so a caller that cares whether the
// file exists asks bench.Exists as well. The suppression cases below do both.
func configBytes(t *testing.T, home string) []byte {
	t.Helper()
	data, err := os.ReadFile(configPath(home))
	if err != nil {
		return nil
	}
	return data
}

// noActorHome points the user base at a directory of this test's own with
// every rung of the actor ladder empty, and proves the emptiness rather than
// trusting it.
//
// The proof carries the weight. newBench and emptyTree both export
// DINAH_ACTOR, so a test that reached for either would resolve an actor, take
// the branch where `init` records nothing, and pass without exercising the
// recording at all. Returning only once the listing reports the actor unset
// and the user base carries no configuration file means a later change to the
// shared setup fails these tests loudly instead of hollowing them out.
func noActorHome(t *testing.T) (string, string) {
	t.Helper()
	home, base := settingsHome(t)
	rows := settingRows(t, runCLI(t, base, "--json", "config"))
	if rows["actor"].Value != "" || rows["actor"].Source != bench.SourceUnset {
		t.Fatalf("no rung should carry an actor here, got %q from the %s rung", rows["actor"].Value, rows["actor"].Source)
	}
	if bench.Exists(configPath(home)) {
		t.Fatalf("the user base should carry no configuration file yet, %s exists", configPath(home))
	}
	return home, base
}

// newRoom makes an empty directory to create a workbench in, so that discovery
// from it reaches the workbench `init` writes and nothing else.
func newRoom(t *testing.T, base string) string {
	t.Helper()
	root := filepath.Join(base, "qs")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return root
}

// TestInitRecordsTheActorWhenNothingElseNamesOne asserts dinah-137: creating a
// workbench on a machine that knows nobody records the operator as the actor
// in the person's own configuration, says so, and leaves the person able to
// file a card with no command in between. It asserts the whole rule, including
// the three ways an actor already resolved suppresses the write and the
// ordering that keeps a refused creation from recording anything.
func TestInitRecordsTheActorWhenNothingElseNamesOne(t *testing.T) {
	t.Run("a machine with no configuration files a card straight after init", func(t *testing.T) {
		home, base := noActorHome(t)
		root := newRoom(t, base)

		created := runCLI(t, root, "init", "--operator", "paul")
		if created.code != 0 {
			t.Fatalf("init: %d %s", created.code, created.errw)
		}
		lines := strings.Split(strings.TrimSuffix(created.out, "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("init should print the created line and the recorded line, got %q", created.out)
		}
		announcement := lines[1]
		if !strings.Contains(announcement, "paul") {
			t.Errorf("the announcement should name the actor it recorded, got %q", announcement)
		}
		if !strings.Contains(announcement, configPath(home)) {
			t.Errorf("the announcement should name %s, got %q", configPath(home), announcement)
		}
		// The removal command is the bare form with no name after it,
		// which is the spelling that takes the value out rather than
		// replacing it. The subtest below runs it and checks it does.
		if !strings.Contains(announcement, "`dinah config set actor`") {
			t.Errorf("the announcement should name the command that removes the value, got %q", announcement)
		}

		filed := runCLI(t, root, "add", "Write the release notes")
		if filed.code != 0 {
			t.Fatalf("add straight after init: %d %s", filed.code, filed.errw)
		}

		stored, err := os.ReadFile(configPath(home))
		if err != nil {
			t.Fatalf("the configuration should exist: %v", err)
		}
		if !strings.Contains(string(stored), "actor: paul") {
			t.Errorf("the configuration should carry actor: paul, got %q", stored)
		}
		rows := settingRows(t, runCLI(t, root, "--json", "config"))
		if rows["actor"].Value != "paul" || rows["actor"].Source != bench.SourceConfig {
			t.Errorf("the listing should report paul from the config rung, got %q from %q", rows["actor"].Value, rows["actor"].Source)
		}
	})

	t.Run("the command the announcement names removes the recorded value", func(t *testing.T) {
		home, base := noActorHome(t)
		root := newRoom(t, base)
		if got := runCLI(t, root, "init", "--operator", "paul"); got.code != 0 {
			t.Fatalf("init: %d %s", got.code, got.errw)
		}

		if got := runCLI(t, root, "config", "set", "actor"); got.code != 0 {
			t.Fatalf("config set actor with no name: %d %s", got.code, got.errw)
		}
		stored, err := os.ReadFile(configPath(home))
		if err != nil {
			t.Fatalf("the configuration should still exist: %v", err)
		}
		if strings.Contains(string(stored), "actor") {
			t.Errorf("the key should be gone from the file rather than emptied, got %q", stored)
		}
		rows := settingRows(t, runCLI(t, root, "--json", "config"))
		if rows["actor"].Value != "" || rows["actor"].Source != bench.SourceUnset {
			t.Errorf("the listing should report the actor unset, got %q from %q", rows["actor"].Value, rows["actor"].Source)
		}
	})

	suppression := []struct {
		name    string
		prepare func(t *testing.T, root string)
		argv    []string
	}{
		{
			name:    "the flag",
			prepare: func(t *testing.T, root string) {},
			argv:    []string{"init", "--operator", "paul", "--actor", "bo"},
		},
		{
			name:    "the environment",
			prepare: func(t *testing.T, root string) { t.Setenv("DINAH_ACTOR", "bo") },
			argv:    []string{"init", "--operator", "paul"},
		},
		{
			name: "the configuration",
			prepare: func(t *testing.T, root string) {
				if got := runCLI(t, root, "config", "set", "actor", "bo"); got.code != 0 {
					t.Fatalf("config set: %d %s", got.code, got.errw)
				}
			},
			argv: []string{"init", "--operator", "paul"},
		},
	}
	for _, c := range suppression {
		t.Run("an actor at "+c.name+" suppresses the write and the line", func(t *testing.T) {
			home, base := noActorHome(t)
			root := newRoom(t, base)
			c.prepare(t, root)
			before := configBytes(t, home)
			existedBefore := bench.Exists(configPath(home))

			got := runCLI(t, root, c.argv...)
			if got.code != 0 {
				t.Fatalf("init: %d %s", got.code, got.errw)
			}
			// initReported fails unless stdout is the created line
			// alone, which is the assertion that no second line was
			// printed.
			initReported(t, got)
			if after := configBytes(t, home); !bytes.Equal(before, after) {
				t.Errorf("the configuration should be untouched, before %q and after %q", before, after)
			}
			// The bytes alone would not catch a zero-byte file
			// created where none existed, since bytes.Equal reads
			// that as the nil configBytes returns for an absent
			// one, so the existence is asserted on its own.
			if existedAfter := bench.Exists(configPath(home)); existedAfter != existedBefore {
				t.Errorf("the configuration file's existence changed, before %v and after %v", existedBefore, existedAfter)
			}
		})
	}

	t.Run("a refused init records nothing", func(t *testing.T) {
		home, base := noActorHome(t)
		root := filepath.Join(base, "workbench")
		definition, err := bench.ReadDefinition([]byte(fmt.Sprintf(baseDefinition, "Bare")))
		if err != nil {
			t.Fatalf("definition: %v", err)
		}
		if err := bench.Instantiate(root, "bare", "ana", definition); err != nil {
			t.Fatalf("instantiate: %v", err)
		}

		got := runCLI(t, root, "init", "--slug", "other", "--operator", "paul")
		if got.code != 2 {
			t.Fatalf("exit code: wanted 2, got %d (%s)", got.code, got.out)
		}
		if leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]; leading != contract.Exists {
			t.Errorf("leading token: wanted %s, got %q", contract.Exists, got.errw)
		}
		if bench.Exists(configPath(home)) {
			t.Errorf("the refused init wrote %s", configPath(home))
		}
	})

	t.Run("a configuration Dinah cannot write is reported without a refusal name", func(t *testing.T) {
		home, base := noActorHome(t)
		root := newRoom(t, base)
		// A regular file where the user base directory belongs makes
		// the MkdirAll inside WriteText fail on Windows and on Linux
		// alike, so the arm runs against a write that really failed
		// rather than against a stand-in for one.
		if err := os.MkdirAll(home, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		blocked := filepath.Join(home, bench.UserBaseName)
		if err := os.WriteFile(blocked, []byte("a file where the user base belongs\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		got := runCLI(t, root, "init", "--operator", "paul")
		if got.code != 0 {
			t.Fatalf("init: wanted 0 because the workbench exists, got %d (%s)", got.code, got.errw)
		}
		// The created line still stands alone on stdout, and the
		// announcement of a write that did not happen is absent.
		initReported(t, got)

		report := strings.TrimSuffix(got.errw, "\n")
		if strings.Contains(report, "\n") {
			t.Fatalf("the failure should reach stderr as one line, got %q", got.errw)
		}
		leading := strings.SplitN(report, " ", 2)[0]
		for _, token := range []string{contract.OutcomeUnreachable, contract.NoOwner} {
			if leading == token {
				t.Errorf("the report leads with %s, which reads as a failure on a run that exited 0: %q", token, report)
			}
		}
		if !strings.Contains(report, configPath(home)) {
			t.Errorf("the report should name %s, got %q", configPath(home), report)
		}
		if !strings.Contains(report, "`dinah config set actor <name>`") {
			t.Errorf("the report should name the command that recovers, got %q", report)
		}
	})

	t.Run("a workbench somebody else operates leaves the actor alone", func(t *testing.T) {
		home, base := noActorHome(t)
		root := newRoom(t, base)
		t.Setenv("DINAH_ACTOR", "bo")

		got := runCLI(t, root, "init", "--operator", "ana")
		if got.code != 0 {
			t.Fatalf("init: %d %s", got.code, got.errw)
		}
		initReported(t, got)
		if bench.Exists(configPath(home)) {
			t.Errorf("init wrote %s for somebody whose actor was already resolvable", configPath(home))
		}
		opened, err := bench.Open(benchDir(t, root))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if opened.Operator != "ana" {
			t.Errorf("the workbench's operator: wanted ana, got %q", opened.Operator)
		}
		identity := runCLI(t, root, "whoami")
		if identity.code != 0 {
			t.Fatalf("whoami: %d %s", identity.code, identity.errw)
		}
		if strings.TrimSpace(identity.out) != "bo, operator: no" {
			t.Errorf("whoami: wanted %q, got %q", "bo, operator: no", identity.out)
		}
	})

	t.Run("the no-owner refusal names a command that fixes it", func(t *testing.T) {
		_, base := noActorHome(t)
		root := newRoom(t, base)
		if got := runCLI(t, root, "init", "--actor", "bo", "--operator", "ana"); got.code != 0 {
			t.Fatalf("init: %d %s", got.code, got.errw)
		}

		refused := runCLI(t, root, "whoami")
		if refused.code != 2 {
			t.Fatalf("whoami with no actor: wanted 2, got %d (%s)", refused.code, refused.out)
		}
		if leading := strings.SplitN(strings.TrimSpace(refused.errw), " ", 2)[0]; leading != contract.NoOwner {
			t.Errorf("leading token: wanted %s, got %q", contract.NoOwner, refused.errw)
		}
		if !strings.Contains(refused.errw, "`dinah config set actor <name>`") {
			t.Errorf("the refusal should name the command that fixes it, got %q", refused.errw)
		}

		if got := runCLI(t, root, "config", "set", "actor", "bo"); got.code != 0 {
			t.Fatalf("the named command: %d %s", got.code, got.errw)
		}
		repaired := runCLI(t, root, "whoami")
		if repaired.code != 0 {
			t.Fatalf("whoami after the named command: %d %s", repaired.code, repaired.errw)
		}
		if strings.TrimSpace(repaired.out) != "bo, operator: no" {
			t.Errorf("whoami: wanted %q, got %q", "bo, operator: no", repaired.out)
		}
	})
}

// workstreamBench builds a workbench carrying one workstream and returns the
// container it sits in, which is what the workstream cases below start from.
func workstreamBench(t *testing.T) string {
	t.Helper()
	root := newBench(t)
	if got := runCLI(t, root, "workstream", "new", "Portfolio work"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}
	return root
}

// TestAWorkstreamIsCreatedListedAndReadFromATerminal asserts the four shapes
// the command draws: the sentence a workbench carrying none prints, the line
// creation prints, the listing, and one field printed alone with no heading
// and no padding.
func TestAWorkstreamIsCreatedListedAndReadFromATerminal(t *testing.T) {
	root := newBench(t)
	empty := runCLI(t, root, "workstream")
	if empty.code != 0 || empty.out != msg.For(msg.Base).T("workstreams.empty")+"\n" {
		t.Errorf("a workbench carrying no workstream printed %d %q", empty.code, empty.out)
	}

	created := runCLI(t, root, "workstream", "new", "Portfolio work")
	if created.code != 0 {
		t.Fatalf("workstream new: %d %s", created.code, created.errw)
	}
	if created.out != "portfolio-work  Portfolio work  [active]\n" {
		t.Errorf("creation printed %q", created.out)
	}

	listing := runCLI(t, root, "workstream")
	for _, want := range []string{"portfolio-work", "Portfolio work", "active", "0"} {
		if !strings.Contains(listing.out, want) {
			t.Errorf("the listing does not carry %q:\n%s", want, listing.out)
		}
	}

	field := runCLI(t, root, "workstream", "get", "portfolio-work", "status")
	if field.code != 0 || field.out != "active\n" {
		t.Errorf("one field alone printed %d %q, wanted the value and nothing else", field.code, field.out)
	}

	unknown := runCLI(t, root, "workstream", "get", "nosuch")
	if unknown.code != 2 || !strings.HasPrefix(unknown.errw, contract.UnknownWorkstream+" ") {
		t.Errorf("an unknown workstream: %d %q", unknown.code, unknown.errw)
	}
	if !strings.Contains(unknown.errw, "nosuch") {
		t.Errorf("the refusal does not name what the caller typed: %q", unknown.errw)
	}
}

// TestAWorkstreamSlugChangeNeedsTheConfirmationFlag asserts that the rename
// every reference already written down depends on is refused without --yes,
// writes nothing when it is refused, and moves the reference when it is given.
func TestAWorkstreamSlugChangeNeedsTheConfirmationFlag(t *testing.T) {
	root := workstreamBench(t)
	refused := runCLI(t, root, "workstream", "set", "portfolio-work", "slug", "folio")
	if refused.code != 2 || !strings.HasPrefix(refused.errw, contract.Unconfirmed+" ") {
		t.Fatalf("a slug change without the flag: %d %q", refused.code, refused.errw)
	}
	if got := runCLI(t, root, "workstream", "get", "portfolio-work", "slug"); got.out != "portfolio-work\n" {
		t.Errorf("the refused change wrote something: %q", got.out)
	}
	if got := runCLI(t, root, "workstream", "set", "portfolio-work", "slug", "folio", "--yes"); got.code != 0 {
		t.Fatalf("a slug change with the flag: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "workstream", "get", "portfolio-work"); got.code != 2 || !strings.HasPrefix(got.errw, contract.UnknownWorkstream+" ") {
		t.Errorf("the old slug still resolves: %d %q", got.code, got.errw)
	}
	if got := runCLI(t, root, "workstream", "get", "folio", "slug"); got.out != "folio\n" {
		t.Errorf("the new slug does not resolve: %q", got.out)
	}
}

// TestTheCardLineCarriesTheWorkstreamsACardBelongsTo asserts the trailing
// field: a card belonging to none draws the plain line, a card belonging to
// one or more draws the sibling key with the memberships on the end, the two
// forms differ by that field alone, and both carry real text in Hindi as well
// as in English.
//
// A membership naming no workstream prints the identifier the card stores,
// since that is the value a reader has to go and repair.
func TestTheCardLineCarriesTheWorkstreamsACardBelongsTo(t *testing.T) {
	root := workstreamBench(t)
	if got := runCLI(t, root, "add", "a card to belong"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	plain := runCLI(t, root, "show", "fx-1")
	if strings.Contains(plain.out, "portfolio-work") {
		t.Errorf("a card belonging to no workstream drew a trailing field: %q", plain.out)
	}
	joined := runCLI(t, root, "join", "fx-1", "portfolio-work")
	if joined.code != 0 {
		t.Fatalf("join: %d %s", joined.code, joined.errw)
	}
	if joined.out != strings.TrimSuffix(plain.out, "\n")+"  portfolio-work\n" {
		t.Errorf("the two forms differ by more than the trailing field:\n%q\n%q", plain.out, joined.out)
	}
	carryToDoing(t, root, "fx-1")
	for _, command := range [][]string{{"show", "fx-1"}, {"claim", "fx-1"}, {"release", "fx-1"}} {
		got := runCLI(t, root, command...)
		if got.code != 0 {
			t.Fatalf("%v: %d %s", command, got.code, got.errw)
		}
		if !strings.Contains(got.out, "portfolio-work") {
			t.Errorf("%v drew no trailing field, and one renderer draws every card line: %q", command, got.out)
		}
	}
	hindi := runCLI(t, root, "show", "fx-1", "--lang", "hi")
	if !strings.Contains(hindi.out, "portfolio-work") {
		t.Errorf("the Hindi card line drew no trailing field: %q", hindi.out)
	}

	anchor := filepath.Join(soleBenchDir(t, root), bench.CardsDir)
	ids := bench.ListIDs(anchor)
	if len(ids) != 1 {
		t.Fatalf("wanted one card, got %v", ids)
	}
	path := filepath.Join(anchor, ids[0], bench.CardAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the card: %v", err)
	}
	replaced := strings.Replace(text, "workstreams:\n", "workstreams:\n  - f00000000009\n", 1)
	if replaced == text {
		t.Fatalf("the fixture card carries no membership list to add to:\n%s", text)
	}
	if err := os.WriteFile(path, []byte(replaced), 0o644); err != nil {
		t.Fatalf("write the card: %v", err)
	}
	dangling := runCLI(t, root, "show", "fx-1")
	if !strings.Contains(dangling.out, "f00000000009") {
		t.Errorf("a membership naming no workstream did not print the identifier the card carries: %q", dangling.out)
	}
}

// TestAWorkstreamAndAColumnMayShareAName asserts the one asymmetry in the
// reference grammar. A workstream names its kind wherever the generic entity
// commands take a reference, so a column of the same name shadows neither it
// nor itself.
func TestAWorkstreamAndAColumnMayShareAName(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "workstream", "new", "review"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}
	columns := filepath.Join(soleBenchDir(t, root), bench.ColumnsDir)
	renamed := false
	for _, id := range bench.ListIDs(columns) {
		path := filepath.Join(columns, id, bench.ColumnAnchor)
		text, err := bench.ReadText(path)
		if err != nil {
			t.Fatalf("read a column: %v", err)
		}
		if !strings.Contains(text, "title: Doing") {
			continue
		}
		if err := os.WriteFile(path, []byte(strings.Replace(text, "slug: doing", "slug: review", 1)), 0o644); err != nil {
			t.Fatalf("write a column: %v", err)
		}
		renamed = true
	}
	if !renamed {
		t.Fatal("the fixture flow carries no column to rename")
	}
	if got := runCLI(t, root, "workstream", "get", "review", "title"); got.out != "review\n" {
		t.Errorf("the bare reference inside the workstream command read %q", got.out)
	}
	if got := runCLI(t, root, "archive", "workstream/review"); got.code != 0 {
		t.Fatalf("archiving the workstream: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "columns"); !strings.Contains(got.out, "review") {
		t.Errorf("archiving the workstream took the column with it:\n%s", got.out)
	}
	if got := runCLI(t, root, "archive", "review"); got.code != 0 {
		t.Fatalf("archiving the column: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "columns"); strings.Contains(got.out, "review") {
		t.Errorf("the column survived its own archiving:\n%s", got.out)
	}
}

// TestCheckReportsAndAdoptsAMembershipNamingNothing asserts the finding a
// workbench written before this card draws, the repair that answers it, and
// the promise the repair keeps: no card anchor is touched.
func TestCheckReportsAndAdoptsAMembershipNamingNothing(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card that belongs to something"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	cards := filepath.Join(soleBenchDir(t, root), bench.CardsDir)
	ids := bench.ListIDs(cards)
	path := filepath.Join(cards, ids[0], bench.CardAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the card: %v", err)
	}
	planted := strings.Replace(text, "state: ready", "state: ready\nworkstreams:\n  - f00000000009\nunknown_key: kept", 1)
	if err := os.WriteFile(path, []byte(planted), 0o644); err != nil {
		t.Fatalf("write the card: %v", err)
	}

	reported := runCLI(t, root, "check")
	if reported.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Fatalf("check on a workbench carrying a dangling membership: %d", reported.code)
	}
	if !strings.Contains(reported.out, "f00000000009") || !strings.Contains(reported.out, path) {
		t.Errorf("the finding names neither the identifier nor the card's anchor:\n%s", reported.out)
	}
	machine := runCLI(t, root, "--json", "check")
	// dinah-346: the JSON head and the human head are separate exit-code
	// sites, so the machine run is held to the same code the human one above
	// is, and to the outcome member a caller reads instead of the code.
	if machine.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Errorf("the machine form exited %d, wanted %d:\n%s", machine.code, contract.ExitCodeForRead(contract.ReadFindings), machine.out)
	}
	var report verb.CheckReport
	if err := json.Unmarshal([]byte(machine.out), &report); err != nil {
		t.Fatalf("decode the machine form: %v\n%s", err, machine.out)
	}
	if report.Outcome != contract.ReadFindings {
		t.Errorf("the machine form reports outcome %q over %d findings, wanted %q", report.Outcome, len(report.Findings), contract.ReadFindings)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Key == bench.FindingDanglingWorkstream && finding.Detail == "f00000000009" {
			found = true
		}
	}
	if !found {
		t.Errorf("the machine form carries no dangling-workstream finding: %+v", report.Findings)
	}

	adopted := runCLI(t, root, "check", "--migrate-workstreams")
	if !strings.Contains(adopted.out, msg.For(msg.Base).TN("check.workstream-adopted", 1)) {
		t.Errorf("the repair reported %q", adopted.out)
	}
	after, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the card again: %v", err)
	}
	if after != planted {
		t.Errorf("the repair rewrote the card anchor:\n%q\n%q", planted, after)
	}
	if got := runCLI(t, root, "workstream", "get", "f00000000009", "status"); got.out != "active\n" {
		t.Errorf("the adopted workstream reads %q, wanted an active status", got.out)
	}
	repaired := runCLI(t, root, "check")
	if strings.Contains(repaired.out, bench.FindingDanglingWorkstream) || strings.Contains(repaired.out, "resolves in neither half") {
		t.Errorf("the membership is still reported as dangling after the repair:\n%s", repaired.out)
	}
	if !strings.Contains(repaired.out, "carries no slug") {
		t.Errorf("the adopted workstream is not reported as unnamed:\n%s", repaired.out)
	}
}

// TestEveryMachineSurfaceCarriesAWorkstream asserts the two shapes the machine
// surface gained: a card carries the memberships its frontmatter stores, and
// the workstream command answers in the shape show already uses for a card.
func TestEveryMachineSurfaceCarriesAWorkstream(t *testing.T) {
	root := workstreamBench(t)
	if got := runCLI(t, root, "add", "a card to belong"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "join", "fx-1", "portfolio-work"); got.code != 0 {
		t.Fatalf("join: %d %s", got.code, got.errw)
	}

	shown := runCLI(t, root, "--json", "show", "fx-1")
	var detail verb.Detail
	if err := json.Unmarshal([]byte(shown.out), &detail); err != nil {
		t.Fatalf("decode show: %v\n%s", err, shown.out)
	}
	if len(detail.Card.Workstreams) != 1 || !bench.IsID(detail.Card.Workstreams[0]) {
		t.Errorf("the card carries %v, wanted the one identifier its frontmatter stores", detail.Card.Workstreams)
	}
	id := detail.Card.Workstreams[0]

	listed := runCLI(t, root, "--json", "ls")
	var listing verb.Listing
	if err := json.Unmarshal([]byte(listed.out), &listing); err != nil {
		t.Fatalf("decode ls: %v\n%s", err, listed.out)
	}
	if len(listing.Cards) != 1 || len(listing.Cards[0].Workstreams) != 1 {
		t.Errorf("the listing carries %+v, wanted the card with its membership", listing.Cards)
	}

	workstreams := runCLI(t, root, "--json", "workstream")
	var all verb.WorkstreamListing
	if err := json.Unmarshal([]byte(workstreams.out), &all); err != nil {
		t.Fatalf("decode the workstream listing: %v\n%s", err, workstreams.out)
	}
	if len(all.Workstreams) != 1 {
		t.Fatalf("the machine listing carries %+v", all.Workstreams)
	}
	entry := all.Workstreams[0]
	if entry.ID != id || entry.Ref != "portfolio-work" || entry.Slug != "portfolio-work" || entry.Title != "Portfolio work" || entry.Status != "active" || entry.Cards != 1 {
		t.Errorf("the machine listing reads %+v", entry)
	}

	one := runCLI(t, root, "--json", "workstream", "get", "portfolio-work")
	var got verb.WorkstreamDetail
	if err := json.Unmarshal([]byte(one.out), &got); err != nil {
		t.Fatalf("decode the workstream: %v\n%s", err, one.out)
	}
	if got.Workstream.ID != id || got.Path == "" {
		t.Errorf("the machine form reads %+v", got)
	}
	if len(got.Cards) != 1 || got.Cards[0].Ref != "fx-1" {
		t.Errorf("the machine form carries %+v, wanted the one member card", got.Cards)
	}
}

// TestAHandWrittenWorkstreamDirectoryIsSkippedRatherThanRefused asserts what
// the tool does with a directory it did not write. A name that is not an
// identifier is invisible, a directory carrying no anchor is reported and
// stepped over, and neither takes the listing away.
func TestAHandWrittenWorkstreamDirectoryIsSkippedRatherThanRefused(t *testing.T) {
	root := workstreamBench(t)
	workstreams := filepath.Join(soleBenchDir(t, root), bench.WorkstreamsDir)
	if err := os.MkdirAll(filepath.Join(workstreams, "notahex"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	anchor := filepath.Join(workstreams, "notahex", bench.WorkstreamAnchor)
	if err := os.WriteFile(anchor, []byte("---\ntitle: Bogus\n---\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workstreams, "f00000000001"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	listing := runCLI(t, root, "workstream")
	if listing.code != 0 {
		t.Fatalf("the listing refused over a directory it did not write: %d %s", listing.code, listing.errw)
	}
	if strings.Contains(listing.out, "notahex") || strings.Contains(listing.out, "f00000000001") {
		t.Errorf("the listing drew a directory carrying no readable workstream:\n%s", listing.out)
	}
	if !strings.Contains(listing.out, "portfolio-work") {
		t.Errorf("the listing lost the workstream Dinah wrote:\n%s", listing.out)
	}

	reported := runCLI(t, root, "check")
	if reported.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Fatalf("check on a workbench carrying a directory with no anchor: %d", reported.code)
	}
	if !strings.Contains(reported.out, "f00000000001") {
		t.Errorf("check does not name the directory carrying no anchor:\n%s", reported.out)
	}
	if strings.Contains(reported.out, "notahex") {
		t.Errorf("check named a directory whose name is not an identifier:\n%s", reported.out)
	}
}

// TestAWorkstreamsNotesAndItsEmptyMembershipBothDraw asserts the two branches
// of the read that a workbench of one workstream and no cards reaches: the
// notes print under the fields, and a workstream nobody has joined draws no
// member table at all.
func TestAWorkstreamsNotesAndItsEmptyMembershipBothDraw(t *testing.T) {
	root := workstreamBench(t)
	bare := runCLI(t, root, "workstream", "get", "portfolio-work")
	if bare.code != 0 {
		t.Fatalf("workstream get: %d %s", bare.code, bare.errw)
	}
	if strings.Contains(bare.out, msg.For(msg.Base).T("column.workstream.card")+"  ") {
		t.Errorf("a workstream nobody has joined drew a member table:\n%s", bare.out)
	}

	workstreams := filepath.Join(soleBenchDir(t, root), bench.WorkstreamsDir)
	ids := bench.ListIDs(workstreams)
	if len(ids) != 1 {
		t.Fatalf("wanted one workstream, got %v", ids)
	}
	path := filepath.Join(workstreams, ids[0], bench.WorkstreamAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(path, []byte(text+"The long-form notes.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	noted := runCLI(t, root, "workstream", "get", "portfolio-work")
	if !strings.Contains(noted.out, "The long-form notes.") {
		t.Errorf("the notes did not print:\n%s", noted.out)
	}
}

// TestQueryRendersATableAndSaysSoWhenNothingMatched asserts the two human
// renderings of the query command: the table it draws through the one renderer,
// and the single line it prints instead when nothing matched.
//
// The column column is what separates this rendering from the one ls draws. A
// query spans the whole workbench, so the reader is shown which column each card
// is in, and it carries the column's title rather than its identifier.
func TestQueryRendersATableAndSaysSoWhenNothingMatched(t *testing.T) {
	root := newBench(t)
	for _, title := range []string{"first", "second"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	carryToDoing(t, root, "fx-1")
	if got := runCLI(t, root, "claim", "fx-1"); got.code != 0 {
		t.Fatalf("claim: %d %s", got.code, got.errw)
	}

	drawn := runCLI(t, root, "query")
	if drawn.code != 0 {
		t.Fatalf("query: %d %s", drawn.code, drawn.errw)
	}
	catalog := msg.For(msg.Base)
	for _, heading := range []string{"column.query.card", "column.query.column", "column.query.standing", "column.query.title"} {
		if !strings.Contains(drawn.out, catalog.T(heading)) {
			t.Errorf("the query table carries no %s heading:\n%s", heading, drawn.out)
		}
	}
	if !strings.Contains(drawn.out, "Intake") {
		t.Errorf("the query table names no column title:\n%s", drawn.out)
	}
	for _, ref := range []string{"fx-1", "fx-2"} {
		if !strings.Contains(drawn.out, ref) {
			t.Errorf("the query table does not list %s:\n%s", ref, drawn.out)
		}
	}

	empty := runCLI(t, root, "query", "holder:nobody")
	if empty.code != 0 {
		t.Fatalf("a query matching nothing exited %d: %s", empty.code, empty.errw)
	}
	if strings.TrimSpace(empty.out) != catalog.T("query.empty") {
		t.Errorf("a query matching nothing printed %q rather than the empty line alone", empty.out)
	}
}

// TestQueryEmitsTheDocumentTheEscapeHatchReads asserts the machine form: one
// object carrying the query as it was received, the matched cards nested under
// cards, and a count. The nesting is what the guide's downstream reader unnests,
// and column_title is the member it groups by, so a card view that stopped
// carrying one would break the guide's example while every other test passed.
func TestQueryEmitsTheDocumentTheEscapeHatchReads(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	var document struct {
		Query string `json:"query"`
		Cards []struct {
			Ref         string `json:"ref"`
			ColumnTitle string `json:"column_title"`
		} `json:"cards"`
		Count int `json:"count"`
	}
	emitted := runCLI(t, root, "query", "--json")
	if emitted.code != 0 {
		t.Fatalf("query --json: %d %s", emitted.code, emitted.errw)
	}
	if err := json.Unmarshal([]byte(emitted.out), &document); err != nil {
		t.Fatalf("the emitted document does not decode: %v\n%s", err, emitted.out)
	}
	if len(document.Cards) != 1 || document.Count != 1 {
		t.Fatalf("the emitted document carries %d cards and a count of %d", len(document.Cards), document.Count)
	}
	if document.Cards[0].ColumnTitle == "" {
		t.Error("the card the document carries has no column_title, which is the member the escape hatch groups by")
	}

	// The echo is the argument as received, so a caller comparing a stored
	// result against what it sent finds its own string rather than a trimmed
	// one it never wrote.
	for _, text := range []string{"", " ", " holder:\"\" "} {
		argv := []string{"query", "--json"}
		if text != "" {
			argv = []string{"query", text, "--json"}
		}
		got := runCLI(t, root, argv...)
		if got.code != 0 {
			t.Fatalf("query %q: %d %s", text, got.code, got.errw)
		}
		if err := json.Unmarshal([]byte(got.out), &document); err != nil {
			t.Fatalf("query %q: %v", text, err)
		}
		if document.Query != text {
			t.Errorf("query %q echoed %q", text, document.Query)
		}
		if document.Count != 1 {
			t.Errorf("query %q selected %d cards, and every one of these selects the one card", text, document.Count)
		}
	}

	refused := runCLI(t, root, "query", "state:reday", "--json")
	if refused.code != 2 {
		t.Errorf("a query naming a value outside a closed vocabulary exited %d, want 2", refused.code)
	}
}

// TestQueryTakesItsTermsAsOneQuotedArgument asserts that the query reaches the
// command through the free-text slot rather than through a flag, so a caller who
// forgets the quotation marks meets dinah-100's own refusal with the line
// rebuilt for them instead of a second refusal saying the same thing.
func TestQueryTakesItsTermsAsOneQuotedArgument(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "query", "column:doing", "holder:alka")
	if got.code != 2 {
		t.Fatalf("an unquoted query exited %d, want 2\n%s", got.code, got.out)
	}
	if !strings.HasPrefix(got.errw, contract.MultipleWords+" ") {
		t.Errorf("an unquoted query refused with %q", got.errw)
	}
	if !strings.Contains(got.errw, `dinah query "column:doing holder:alka"`) {
		t.Errorf("the refusal did not rebuild the quoted line:\n%s", got.errw)
	}
}

// TestQueryHelpIsGeneratedFromTheCheckList asserts that the per-command help
// block is derived from the library's own ordered check list rather than
// written out beside it, so a check added, reordered or renamed changes the help
// text and the behaviour together.
func TestQueryHelpIsGeneratedFromTheCheckList(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "help", "query")
	if got.code != 0 {
		t.Fatalf("help query: %d %s", got.code, got.errw)
	}
	checks := verb.Checks("query")
	if len(checks) != 7 {
		t.Fatalf("the query command declares %d checks, and the spec's section 10 fixes seven", len(checks))
	}
	catalog := msg.For(msg.Base)
	at := 0
	for i, check := range checks {
		row := catalog.T(check.Key)
		found := strings.Index(got.out[at:], row)
		if found < 0 {
			t.Fatalf("the help block does not carry check %d, %q:\n%s", i+1, row, got.out)
		}
		at += found + len(row)
		if !strings.Contains(got.out[at:], check.Refusal) {
			t.Errorf("check %d, %q, is not followed by its refusal name %s:\n%s", i+1, row, check.Refusal, got.out)
		}
	}
}

// TestTheArgumentsTableSpellsEveryArgumentTheSyntaxLineWay asserts the half of
// the one-spelling claim that a reader can see: the left column of the
// rendered arguments table is verb.Tokens in order, so the table and the
// syntax line above it cannot spell one argument two ways.
//
// It reads the drawn page rather than the token list twice. The sweep of every
// command is what makes it a claim about the tool rather than about attach.
func TestTheArgumentsTableSpellsEveryArgumentTheSyntaxLineWay(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "80")
	swept := 0
	for _, name := range verb.Commands() {
		tokens := verb.Tokens(name)
		if len(tokens) == 0 {
			continue
		}
		got := runCLI(t, root, "help", name)
		if got.code != 0 {
			t.Fatalf("help %s: %d %s", name, got.code, got.errw)
		}
		drawn := argumentColumn(t, name, got.out)
		if len(drawn) != len(tokens) {
			t.Errorf("help %s draws %d argument rows and the command declares %d: %v against %v", name, len(drawn), len(tokens), drawn, tokens)
			continue
		}
		for i, token := range tokens {
			if drawn[i] != token {
				t.Errorf("help %s row %d spells the argument %q and the syntax line spells it %q", name, i+1, drawn[i], token)
			}
		}
		swept++
	}
	if swept == 0 {
		t.Fatal("no command drew an arguments table, so this test proves nothing")
	}
}

// argumentColumn reads the left column of a page's arguments table, one entry
// per row. It measures the column off the rule under the heading rather than
// splitting on whitespace, since a valued flag's token carries a space, and it
// skips the continuation lines a wrapped meaning draws.
func argumentColumn(t *testing.T, command, page string) []string {
	t.Helper()
	heading := msg.For(msg.Base).T("help.arguments")
	lines := strings.Split(page, "\n")
	at := -1
	for i, line := range lines {
		if line == heading {
			at = i + 1
			break
		}
	}
	if at < 0 || at+2 >= len(lines) {
		t.Fatalf("help %s draws no arguments section", command)
	}
	rule := strings.Fields(lines[at+1])
	if len(rule) != 2 {
		t.Fatalf("help %s draws %d rules under its arguments table, want two", command, len(rule))
	}
	width := len(rule[0])
	var drawn []string
	for _, line := range lines[at+2:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		if len(line) <= sweptIndent+width || strings.TrimSpace(line[:sweptIndent+width]) == "" {
			continue
		}
		drawn = append(drawn, strings.TrimSpace(line[:sweptIndent+width]))
	}
	return drawn
}

// TestEveryPageSaysWhatEachArgumentIs asserts the content this card writes,
// page by page, against the pages the binary draws. Each case names a page and
// the phrases that page has to carry, so a sentence dropped from the catalog
// or a row dropped from the table fails here rather than in front of a reader.
func TestEveryPageSaysWhatEachArgumentIs(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "80")
	cases := []struct {
		command string
		carries []string
	}{
		{command: "check", carries: []string{
			"[--finish]", "complete or roll back a structural act that was interrupted",
			"[--migrate-ordinals]", "stamp a creation ordinal on every entity that carries none",
			"[--migrate-slugs]", "derive a slug for every column of this workbench that carries none",
			"[--migrate-columns]", "remove stranded identifiers from this workbench's own list of columns",
			"Dinah exits 2 when it finds a defect",
		}},
		{command: "attach", carries: []string{
			"attach <ref> <file> [--description <text>] [--replace]",
			"[--description <text>]", "a line describing the attachment, stored beside it",
			"what the file hangs off: this workbench, a column, a card, or a comment or an attachment below a card",
			"with --replace, the attachment whose bytes you are replacing",
			"For more, run `dinah guide references`.",
		}},
		{command: "init", carries: []string{
			"init [dir] [--from <source>]",
			"[dir]", "the directory you are in when you name none",
			"a directory holding a workbench, or a single file written by `dinah export` or `dinah extract`",
		}},
		{command: "claim", carries: []string{
			"[--expires <duration>]", "written as a number and a unit: 30m, 2h, 7d",
		}},
		{command: "block", carries: []string{
			"[--kind <kind>]", "Dinah stores whatever you write and checks it against no set",
		}},
		{command: "query", carries: []string{"For more, run `dinah guide query`."}},
		{command: "path", carries: []string{
			"path <ref>", "this workbench written as `workbench` or `.`",
			"For more, run `dinah guide references`.",
		}},
		{command: "edit", carries: []string{
			"edit <ref>", "this workbench written as `workbench` or `.`",
			"For more, run `dinah guide references`.",
		}},
		{command: "show", carries: []string{
			"show <ref>", "show does not take this workbench",
			"For more, run `dinah guide references`.",
		}},
		{command: "instructions", carries: []string{
			"instructions <card|column>",
			"instructions takes neither this workbench nor anything below a card",
			"For more, run `dinah guide references`.",
		}},
		{command: "archive", carries: []string{
			"a column, a card, or something below a card such as wb-1/comments/1; not this workbench",
			"For more, run `dinah guide references`.",
		}},
		{command: "delete", carries: []string{
			"--yes", "confirm the act, which Dinah does not carry out without it",
			"For more, run `dinah guide references`.",
		}},
	}
	for _, c := range cases {
		got := runCLI(t, root, "help", c.command)
		if got.code != 0 {
			t.Fatalf("help %s: %d %s", c.command, got.code, got.errw)
		}
		flat := strings.Join(strings.Fields(got.out), " ")
		for _, phrase := range c.carries {
			if strings.Contains(flat, strings.Join(strings.Fields(phrase), " ")) {
				continue
			}
			t.Errorf("help %s does not carry %q:\n%s", c.command, phrase, got.out)
		}
	}

	// claim takes a card rather than a reference, so its page points at no
	// guide at all.
	claim := runCLI(t, root, "help", "claim")
	if strings.Contains(claim.out, "dinah guide references") {
		t.Errorf("help claim points at the references guide:\n%s", claim.out)
	}
}

// TestTheColumnVocabularyAnswersInsideAWorkbenchAndIsSilentOutside asserts
// dinah-172 AC-3: a vocabulary living in the reader's own workbench is printed
// where one opens and left out where none does, and the page answers either
// way.
//
// The outside case is established by pointing DINAH_HOME and the working
// directory at directories carrying no workbench, rather than by relying on
// where the test happens to run. This repository carries a discoverable
// workbench of its own and discovery walks the ancestor chain, so a test that
// says nothing about the environment exercises the inside case twice.
//
// Two separate refusals to answer hold the outside case, and this test cannot
// tell them apart. vocabularyValues declines to resolve a columns vocabulary
// when no workbench opens, and refusalListings["columns"] returns nothing when
// the session carries no library. Removing either one alone leaves the outside
// half of this test green, so a reader must not take a pass here as proof that
// the guard in vocabularyValues is doing the work.
func TestTheColumnVocabularyAnswersInsideAWorkbenchAndIsSilentOutside(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	inside := runCLI(t, newBench(t), "help", "ls")
	if inside.code != 0 {
		t.Fatalf("help ls inside a workbench: %d %s", inside.code, inside.errw)
	}
	flat := strings.Join(strings.Fields(inside.out), " ")
	if !strings.Contains(flat, "(one of: intake, doing, done)") {
		t.Errorf("the column row does not name the workbench's own columns:\n%s", inside.out)
	}
	if !strings.Contains(flat, "also written --column <column>") {
		t.Errorf("the column row does not say the argument is also written --column:\n%s", inside.out)
	}

	tree := emptyTree(t)
	if got := runCLI(t, tree, "ls"); got.code == 0 {
		t.Fatalf("the tree should carry no workbench, and ls answered: %s", got.out)
	}
	outside := runCLI(t, tree, "help", "ls")
	if outside.code != 0 {
		t.Fatalf("help ls outside a workbench: %d %s", outside.code, outside.errw)
	}
	if strings.Contains(outside.out, "one of:") {
		t.Errorf("the page names a set it cannot read from here:\n%s", outside.out)
	}
	bare := strings.Join(strings.Fields(outside.out), " ")
	if !strings.Contains(bare, "also written --column <column>") {
		t.Errorf("the page dropped the rest of the row along with the set:\n%s", outside.out)
	}

	// The page answers where a stated workbench is not there either. All six
	// ways s.open can fail reach the arguments section as an error return and
	// are swallowed in the one place, so these two exercise the swallow; the
	// other four need a fixture apiece and are driven against the commands
	// that raise them elsewhere in this suite.
	named := runCLI(t, tree, "help", "ls", "--workbench", filepath.Join(tree, "nowhere"))
	if named.code != 0 {
		t.Fatalf("help ls with a workbench flag naming nothing: %d %s", named.code, named.errw)
	}
	if strings.Contains(named.out, "one of:") {
		t.Errorf("the page names a set the stated workbench cannot answer for:\n%s", named.out)
	}
}

// TestEveryHelpSpellingReachesTheSamePage asserts dinah-213: each spelling in
// askedFor prints help rather than refusing, whether it is written alone,
// before a command or after one, and the page it prints is byte for byte the
// page `dinah help` and `dinah help <command>` already print.
//
// The comparison is against the existing command rather than against a fixture
// because the point of the card is that the flag is a second door onto one
// room. A fixture would let the two drift apart and still pass.
func TestEveryHelpSpellingReachesTheSamePage(t *testing.T) {
	tree := t.TempDir()
	surface := runCLI(t, tree, "help")
	if surface.code != 0 {
		t.Fatalf("dinah help: %d %s", surface.code, surface.errw)
	}
	page := runCLI(t, tree, "help", "ls")
	if page.code != 0 {
		t.Fatalf("dinah help ls: %d %s", page.code, page.errw)
	}
	version := runCLI(t, tree, "version")
	if version.code != 0 {
		t.Fatalf("dinah version: %d %s", version.code, version.errw)
	}
	for spelling, asked := range askedFor {
		want := surface
		if asked == "version" {
			want = version
		}
		// Alone, the spelling reaches the whole surface (or the version
		// report, for the version family).
		if got := runCLI(t, tree, spelling); got.code != 0 || got.out != want.out {
			t.Errorf("dinah %s: wanted exit 0 and the %s output, got %d\n%s", spelling, asked, got.code, got.out)
		}
		if asked != "help" {
			continue
		}
		// After a command and before one, the spelling reaches that
		// command's own page. Both orders are asserted because a caller who
		// has already typed the command name adds the flag on the end, and
		// one who has not writes it first.
		after := runCLI(t, tree, "ls", spelling)
		if after.code != 0 || after.out != page.out {
			t.Errorf("dinah ls %s: wanted ls's page, got %d\n%s", spelling, after.code, after.out)
		}
		before := runCLI(t, tree, spelling, "ls")
		if before.code != 0 || before.out != page.out {
			t.Errorf("dinah %s ls: wanted ls's page, got %d\n%s", spelling, before.code, before.out)
		}
		// A word that is not a command still refuses, so the flag opens no
		// route around the unknown-command check.
		unknown := runCLI(t, tree, spelling, "bogus")
		if unknown.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Errorf("dinah %s bogus: wanted the unknown-command refusal, got %d", spelling, unknown.code)
		}
		if !strings.HasPrefix(unknown.errw, contract.UnknownVerb+" ") {
			t.Errorf("dinah %s bogus: wanted %s to lead stderr, got %q", spelling, contract.UnknownVerb, unknown.errw)
		}
	}
	// A command that would otherwise refuse for want of its arguments prints
	// its page instead, which is the case the card was filed for: a caller
	// asking what a command takes must not be told they used it wrongly.
	move := runCLI(t, tree, "move", "--help")
	if move.code != 0 {
		t.Errorf("dinah move --help: wanted 0, got %d %s", move.code, move.errw)
	}
	if want := runCLI(t, tree, "help", "move"); move.out != want.out {
		t.Errorf("dinah move --help differs from dinah help move:\n%s", move.out)
	}
	// The POSIX marker still shields every spelling as literal text, so a
	// caller who means the word writes it after `--`.
	shielded := runCLI(t, tree, "--", "-h")
	if shielded.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Errorf("dinah -- -h: wanted the unknown-command refusal, got %d %s", shielded.code, shielded.out)
	}
	// Help outranks version when a caller wrote both, in either order.
	for _, argv := range [][]string{{"--help", "--version"}, {"--version", "--help"}} {
		got := runCLI(t, tree, argv...)
		if got.code != 0 || got.out != surface.out {
			t.Errorf("dinah %s: wanted exit 0 and the surface, got %d\n%s", strings.Join(argv, " "), got.code, got.out)
		}
	}
	// Both flags refuse a first word naming no command, so neither opens a
	// route around the check the other closes. `dinah bogus --version` used
	// to print the version while `dinah --help bogus` refused.
	for _, argv := range [][]string{{"bogus", "--version"}, {"--version", "bogus"}} {
		got := runCLI(t, tree, argv...)
		if got.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Errorf("dinah %s: wanted the unknown-command refusal, got %d\n%s", strings.Join(argv, " "), got.code, got.out)
		}
		if !strings.HasPrefix(got.errw, contract.UnknownVerb+" ") {
			t.Errorf("dinah %s: wanted %s to lead stderr, got %q", strings.Join(argv, " "), contract.UnknownVerb, got.errw)
		}
	}
	// A command name in front of the version flag is read and still prints
	// the version, since no command carries a version page of its own.
	if got := runCLI(t, tree, "ls", "--version"); got.code != 0 || got.out != version.out {
		t.Errorf("dinah ls --version: wanted the version report, got %d\n%s", got.code, got.out)
	}
}

// TestTheFlagSetsTheParserAcceptsAreDerivedFromTheParameterTable asserts
// dinah-172 AC-13: the sets args.go derives equal the sets it used to carry by
// hand, named literally here so the derivation is checked against something
// rather than against itself, and the two flags the derivation was written for
// still behave.
func TestTheFlagSetsTheParserAcceptsAreDerivedFromTheParameterTable(t *testing.T) {
	wantValued := []string{
		"actor", "card", "column", "depth", "description", "expires",
		"format", "from", "group-by", "kind", "lang", "max-depth",
		"operator", "priority", "root",
		"severity", "since", "slug", "workbench",
	}
	wantMarkers := []string{
		"catalogs", "finish", "help", "json", "migrate-columns",
		"migrate-ordinals", "migrate-slugs", "migrate-vocabulary",
		"migrate-workstreams", "no-claim", "override", "quiet", "ready",
		"replace", "version", "witness", "yes",
	}
	if got := strings.Join(valuedFlags, " "); got != strings.Join(wantValued, " ") {
		t.Errorf("the derived valued flags are %q and the parser accepted %q", got, strings.Join(wantValued, " "))
	}
	if got := strings.Join(markerFlags, " "); got != strings.Join(wantMarkers, " ") {
		t.Errorf("the derived marker flags are %q and the parser accepted %q", got, strings.Join(wantMarkers, " "))
	}
	for _, flag := range globalFlags {
		if flag.marker == (flag.value != "") {
			t.Errorf("--%s declares marker=%v and the value placeholder %q, which disagree", flag.name, flag.marker, flag.value)
		}
	}

	root := newBench(t)
	file := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "attach", "fx-1", file, "--description", "x"); got.code != 0 {
		t.Errorf("attach with a description: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "ls", "--nonsense"); got.code == 0 {
		t.Errorf("an unknown flag was accepted: %s", got.out)
	}
}

// TestTheReferencesGuideSaysWhichCommandTakesWhat asserts dinah-172 AC-8: the
// new guide teaches every form of a reference, says what each command taking
// one accepts, and is listed among the topics. dinah-151 adds the eighth row,
// for contents, which the sentence over the table counts.
func TestTheReferencesGuideSaysWhichCommandTakesWhat(t *testing.T) {
	root := newBench(t)
	listing := runCLI(t, root, "guide")
	if listing.code != 0 {
		t.Fatalf("guide: %d %s", listing.code, listing.errw)
	}
	if !strings.Contains(listing.out, "references") {
		t.Errorf("the topic listing does not carry the references guide:\n%s", listing.out)
	}
	got := runCLI(t, root, "guide", "references")
	if got.code != 0 {
		t.Fatalf("guide references: %d %s", got.code, got.errw)
	}
	for _, form := range []string{
		"dinah path workbench", "dinah path .", "dinah show wb-1", "dinah attach doing",
		"wb-1/card", "wb-1/journal", "wb-1/comments", "wb-1/comments/1",
		"wb-1/checklist", "wb-1/checklist/1", "wb-1/attachments", "wb-1/attachments/1",
		"wb-1/oq", "wb-1/ac", "wb-1/d",
		"in the order the entities were created",
		"nothing answers to the reference rather than telling you the collection is empty",
	} {
		if !strings.Contains(got.out, form) {
			t.Errorf("the references guide does not teach %q", form)
		}
	}
	// The table, row by row, in the shape the guide draws it. Every cell was
	// provoked against a build rather than read off the resolvers.
	for _, row := range []string{
		"| path         | yes            | yes     | yes    | yes          |",
		"| edit         | yes            | yes     | yes    | yes          |",
		"| show         | no             | yes     | yes    | yes          |",
		"| instructions | no             | yes     | yes    | no           |",
		"| attach       | yes            | yes     | yes    | yes          |",
		"| archive      | no             | yes     | yes    | yes          |",
		"| delete       | no             | yes     | yes    | yes          |",
		"| contents     | yes            | yes     | yes    | yes          |",
		"| attachments  | yes            | yes     | yes    | yes          |",
	} {
		if !strings.Contains(got.out, row) {
			t.Errorf("the references guide does not carry the row %q", row)
		}
	}
	assertTheGuideCountsItsOwnTable(t, got.out)
}

// assertTheGuideCountsItsOwnTable holds the sentence introducing the table in
// the references guide to the table underneath it. The sentence says how many
// commands take a reference and how many different sets of things they accept
// between them, and both numbers are read off the drawn rows rather than
// written down, so a row that gains or loses a cell reddens the sentence that
// describes it.
//
// The guard exists because the shipped sentence claimed that no two commands
// take the same set while the table showed two identical pairs, which a reader
// catches in seconds and no test did.
func assertTheGuideCountsItsOwnTable(t *testing.T, guide string) {
	t.Helper()
	commands := 0
	sets := map[string]bool{}
	for _, line := range strings.Split(guide, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 5 {
			continue
		}
		name := strings.TrimSpace(cells[0])
		if name == "Command" || strings.Trim(name, "-") == "" {
			continue
		}
		var accepts []string
		for _, cell := range cells[1:] {
			accepts = append(accepts, strings.TrimSpace(cell))
		}
		commands++
		sets[strings.Join(accepts, ",")] = true
	}
	if commands == 0 {
		t.Fatal("the references guide draws no command row, so this assertion proves nothing")
	}
	words := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	if commands >= len(words) || len(sets) >= len(words) {
		t.Fatalf("the table draws %d commands over %d sets, past what this assertion spells", commands, len(sets))
	}
	// The sentence is read with its line breaks folded away and its case
	// dropped, since the guide is hard-wrapped and the sentence opens one of
	// its paragraphs, so neither the wrap point nor the capital says anything
	// about whether the claim is true.
	flat := strings.ToLower(strings.Join(strings.Fields(guide), " "))
	takes := words[commands] + " commands take a reference"
	if !strings.Contains(flat, takes) {
		t.Errorf("the table draws %d command rows and the guide does not say %q", commands, takes)
	}
	accepts := "they accept " + words[len(sets)] + " different sets of things"
	if !strings.Contains(flat, accepts) {
		t.Errorf("the table draws %d distinct sets and the guide does not say %q", len(sets), accepts)
	}
}

// TestExpiresTakesTheDaySuffixAndRefusesTheWeek asserts the behaviour the
// claim page now states: the duration is Go's syntax with a day suffix Go does
// not have, so 7d is accepted and 1w is refused as malformed.
func TestExpiresTakesTheDaySuffixAndRefusesTheWeek(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")
	if got := runCLI(t, root, "claim", "fx-1", "--expires", "7d"); got.code != 0 {
		t.Errorf("--expires 7d: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "release", "fx-1"); got.code != 0 {
		t.Fatalf("release: %d %s", got.code, got.errw)
	}
	got := runCLI(t, root, "claim", "fx-1", "--expires", "1w")
	if got.code == 0 {
		t.Fatalf("--expires 1w was accepted: %s", got.out)
	}
	if !strings.HasPrefix(got.errw, contract.Malformed+" ") {
		t.Errorf("--expires 1w refuses with %q, want malformed", got.errw)
	}
}

// TestEveryCatalogIsReportedAgainstItsOwnRoster asserts dinah-172 AC-14: every
// key a card adds reaches every catalog, so `dinah version --catalogs` reports
// each one against the roster it is on and none of them short of a key.
//
// The rosters are read rather than inferred from the coverage numbers. Until
// dinah-287 every catalog was either fully translated or a generated skeleton,
// and reading "not N/N" as "must be 0/N" was true by accident. Hindi and German
// now carry hundreds of real translations and a run of entries the vocabulary
// rename left in English, so they are on neither roster and the numbers in
// between are the honest report rather than a defect.
func TestEveryCatalogIsReportedAgainstItsOwnRoster(t *testing.T) {
	total := len(msg.Keys())
	if total == 0 {
		t.Fatal("the base catalog carries no keys")
	}
	isComplete := map[string]bool{}
	for _, tag := range msg.Complete {
		isComplete[tag] = true
	}
	isSkeleton := map[string]bool{}
	for _, tag := range msg.Skeleton {
		isSkeleton[tag] = true
	}
	complete := 0
	for _, tag := range msg.Tags() {
		translated, present, count := msg.Coverage(tag)
		if count != total {
			t.Errorf("%s is measured against %d keys and the base catalog carries %d", tag, count, total)
		}
		if present != total {
			t.Errorf("%s carries %d of the base catalog's %d keys", tag, present, total)
		}
		if translated == total {
			complete++
		}
		if isComplete[tag] && translated != total {
			t.Errorf("%s ships complete and reports %d of %d keys translated", tag, translated, total)
		}
		if isSkeleton[tag] && translated != 0 {
			t.Errorf("%s ships as a skeleton and reports %d keys translated", tag, translated)
		}
	}
	if complete != len(msg.Complete) {
		t.Errorf("%d catalogs report every key translated, want the %d the roster names", complete, len(msg.Complete))
	}
}

// placeholderGroup matches one angle-bracketed group of a catalog string.
var placeholderGroup = regexp.MustCompile(`<([^<>]*)>`)

// placeholderWord matches the shape a replaceable word takes: letters, digits,
// hyphens and spaces, carrying at least one letter. A group outside that shape
// is not a placeholder at all, which is how the comparison operators in the
// query refusal (`>=, <=, > and <`) are passed over rather than read as words a
// reader replaces. A hand-spelled placeholder such as `<no slug>` is inside the
// shape and is checked.
var placeholderWord = regexp.MustCompile(`^[A-Za-z0-9 -]*[A-Za-z][A-Za-z0-9 -]*$`)

// placeholdersOutsideTheParameterTable are the replaceable words a catalog
// string may carry that no command declares and no global flag names. Each is
// listed with the string that carries it, so the list is read as a set of
// findings rather than as a way around the rule.
var placeholdersOutsideTheParameterTable = map[string]string{
	// The path of a column's own file, in the repair for a workbench whose
	// columns list names a column it carries no directory for. The identifier
	// belongs to the workbench's own storage rather than to any argument.
	"id": "refusal.dinah.add-needs-a-column.next",
	// The position of one attachment among several matching a name selector,
	// in the form rename's caller is told to spell. The argument rename
	// declares is a reference, not an ordinal.
	"ordinal": "refusal.dinah.ambiguous-name.next",
	// The reference rename expects, in the form the call has to take to
	// reach one. The argument rename declares is a reference, not the
	// reference's card segment.
	"card": "refusal.dinah.not-renamable.next",
}

// TestEveryPlaceholderNamesSomethingDeclared asserts dinah-172 AC-18: a word a
// reader replaces is marked one way across every surface, so every
// angle-bracketed placeholder in every catalog string names an argument some
// command declares, the value placeholder of a global flag, or one of the few
// words named above.
//
// A hand-spelled placeholder fails here, which is what stopped three refusals
// from offering `--workbench <path>` where the flag's own value is spelled
// `<dir>`.
func TestEveryPlaceholderNamesSomethingDeclared(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range verb.Commands() {
		for _, param := range verb.Params(name) {
			declared[param.Name] = true
			if param.Value != "" {
				declared[param.Value] = true
			}
		}
	}
	for _, flag := range globalFlags {
		declared[flag.name] = true
		if flag.value != "" {
			declared[flag.value] = true
		}
	}
	checked, skipped := 0, 0
	for _, key := range msg.Keys() {
		entry, ok := msg.BaseEntry(key)
		if !ok {
			t.Fatalf("%s: the base catalog reports it and does not carry it", key)
		}
		for _, group := range placeholderGroup.FindAllStringSubmatch(entry.Text, -1) {
			word := group[1]
			if !placeholderWord.MatchString(word) {
				skipped++
				continue
			}
			checked++
			if declared[word] {
				continue
			}
			if _, named := placeholdersOutsideTheParameterTable[word]; named {
				continue
			}
			t.Errorf("%s spells the placeholder <%s>, which no command declares, no global flag names, and no entry of placeholdersOutsideTheParameterTable covers: %q", key, word, entry.Text)
		}
	}
	if checked == 0 {
		t.Error("no catalog string carries a placeholder, so this test proves nothing")
	}
	if skipped == 0 {
		t.Error("no group was passed over as something other than a placeholder, so the case the comparison operators fall into is not exercised")
	}
	for word, key := range placeholdersOutsideTheParameterTable {
		entry, ok := msg.BaseEntry(key)
		if !ok || !strings.Contains(entry.Text, "<"+word+">") {
			t.Errorf("placeholdersOutsideTheParameterTable names <%s> as carried by %s and that string does not carry it, so the entry is stale", word, key)
		}
	}
}

// ratifiedGlobalFlagTable and ratifiedMoveRefusalTable are two tables the wrap
// this card adds must leave alone, held here as bytes. The arguments table is
// the only table that asks for its last column to be broken, and these are
// what shows that the opt-in is really an opt-in: one table with a summary
// column and one with a refusal column, both drawn at the window the arguments
// table wraps at.
const ratifiedGlobalFlagTable = `  Option             What it does
  -----------------  -----------------------------------------------------------
  --workbench <dir>  Use this workbench instead of the one discovered from here
  --json             Emit the canonical machine form
  --format <name>    Select json or compact for the machine form
  --quiet            Suppress served instructions on claim and move
  --lang <tag>       Render in this language; run ` + "`dinah version --catalogs`" + ` for the tags
  --actor <name>     Act as this owner`

const ratifiedMoveRefusalTable = `  Order  What can go wrong                                   Refusal
  -----  --------------------------------------------------  -------------------
  1      the workbench declares a profile version the tool implements
                                                            unsupported-version
  2      the workbench designates an operator                no-operator
  3      the card exists                                     unknown-card
  4      the destination is a column the workbench declares  unknown-column
  5      an override marker, if carried, is the operator's   not-operator
  6      the departure is legal for whoever asks             not-operator
  7      the card's state is not ` + "`" + `blocked` + "`" + `                   blocked
  8      the card is unheld or held by whoever asks          held
  9      the move is not a forward move out of a ` + "`" + `done` + "`" + ` column
                                                            terminal
  10     the destination is below its capacity limit         at-capacity`

// TestTheArgumentsTableWrapsAndNoOtherTableMoved asserts dinah-172 AC-17: at an
// eighty-column window the arguments table breaks its last column between
// words and indents each continuation under the column, so no line of a
// per-command page reaches past eighty columns, and the two tables held above
// still draw exactly as they did.
func TestTheArgumentsTableWrapsAndNoOtherTableMoved(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "80")
	pages := 0
	for _, name := range verb.Commands() {
		got := runCLI(t, root, "help", name)
		if got.code != 0 {
			t.Fatalf("help %s: %d %s", name, got.code, got.errw)
		}
		pages++
		for _, line := range strings.Split(got.out, "\n") {
			if displayWidth(line) <= 80 {
				continue
			}
			t.Errorf("help %s draws a line %d columns wide:\n%q", name, displayWidth(line), line)
		}
	}
	if pages == 0 {
		t.Fatal("no page was drawn, so this test proves nothing")
	}

	// A wrapped continuation begins under the column its own row's last field
	// begins at, rather than at the left margin.
	attach := runCLI(t, root, "help", "attach")
	if !strings.Contains(attach.out, "\n                          card, or a comment or an attachment below a card; with\n") {
		t.Errorf("the wrapped meaning does not indent under its column:\n%s", attach.out)
	}

	block := runCLI(t, root)
	if !strings.Contains(block.out, ratifiedGlobalFlagTable) {
		t.Errorf("the global flag table moved:\n%s", block.out)
	}
	move := runCLI(t, root, "help", "move")
	if !strings.Contains(move.out, ratifiedMoveRefusalTable) {
		t.Errorf("the refusal table of move moved:\n%s", move.out)
	}
}

// TestEveryVocabularySourceHasAListingThatAnswersIt asserts what the argument
// table's own help page rests on: every source a declared vocabulary names is
// one refusalListings can resolve, so the branch in vocabularyValues that
// gives up on an unresolvable source is unreachable rather than merely
// unreached. That branch is named in testdata/uncovered.txt for this reason,
// and this test is what makes the reason true.
func TestEveryVocabularySourceHasAListingThatAnswersIt(t *testing.T) {
	sources := verb.VocabularySources()
	if len(sources) == 0 {
		t.Fatal("no vocabulary names a source, so this test proves nothing")
	}
	for _, source := range sources {
		if _, ok := refusalListings[source]; !ok {
			t.Errorf("a vocabulary names the source %q and refusalListings carries no listing for it", source)
		}
	}
}

// TestPullOnTheCommandLine asserts the head's half of the pull command: the
// named form takes the card at the head of the upstream queue and claims it,
// both forms answer at exit 0 with a sentence when nothing is waiting, and a
// bare form with more than one qualifying column refuses and prints the columns
// it could not choose between.
//
// The refusal's rows are the reason this lives here rather than in
// internal/verb. The qualifying set is computed at the raise site and carried
// on the response, so the library test can assert the set and only the head
// can assert that a reader sees it drawn.
func TestPullOnTheCommandLine(t *testing.T) {
	t.Run("nothing waiting answers at exit 0", func(t *testing.T) {
		root := newBench(t)
		bare := runCLI(t, root, "pull")
		if bare.code != 0 {
			t.Fatalf("a bare pull with nothing waiting: wanted exit 0, got %d %s", bare.code, bare.errw)
		}
		if strings.TrimSpace(bare.out) == "" {
			t.Error("the bare form should print a sentence saying it found nothing to pull")
		}
		named := runCLI(t, root, "pull", "doing")
		if named.code != 0 {
			t.Fatalf("a named pull with nothing waiting: wanted exit 0, got %d %s", named.code, named.errw)
		}
		if strings.TrimSpace(named.out) == "" {
			t.Error("the named form should print a sentence naming the upstream column it found empty")
		}
		if named.out == bare.out {
			t.Error("the two forms answer different questions and should not print the same sentence")
		}
	})

	t.Run("the named form takes the head of the upstream queue", func(t *testing.T) {
		root := newBench(t)
		runCLI(t, root, "add", "First in")
		runCLI(t, root, "add", "Second in")
		offered := runCLI(t, root, "next", "intake")
		got := runCLI(t, root, "pull", "doing", "--quiet")
		if got.code != 0 {
			t.Fatalf("pull: %d %s", got.code, got.errw)
		}
		if !strings.Contains(got.out, "First in") {
			t.Errorf("pull should take the head of the queue, got %q", got.out)
		}
		if !strings.Contains(offered.out, "First in") {
			t.Errorf("next should have offered the same card, got %q", offered.out)
		}
		if !strings.Contains(got.out, "held by") {
			t.Errorf("a pull claims the card it takes, got %q", got.out)
		}
	})

	t.Run("a bare form with two qualifying columns refuses and lists them", func(t *testing.T) {
		// The flow runs intake, doing, review and done. Intake holds a ready
		// card, so doing qualifies, and doing holds a ready card of its own,
		// so review qualifies too. The done column never qualifies, because a
		// pull lands no card where no owner takes work up, which is why the
		// ambiguity needs a flow one column wider than the default one.
		root := newBenchFromDefinition(t, ambiguousFlow)
		runCLI(t, root, "add", "Waiting in intake")
		runCLI(t, root, "add", "Waiting in doing", "--column", "doing")
		got := runCLI(t, root, "pull")
		if got.code != 2 {
			t.Fatalf("an ambiguous bare pull: wanted exit 2, got %d %q %q", got.code, got.out, got.errw)
		}
		if !strings.HasPrefix(got.errw, contract.AmbiguousColumn+" ") {
			t.Errorf("wanted the refusal name first on stderr, got %q", got.errw)
		}
		for _, slug := range []string{"doing", "review"} {
			if !regexp.MustCompile(`(?m)^\s+` + slug + `\s*$`).MatchString(got.errw) {
				t.Errorf("the refusal should draw %s as a row of its own, got %q", slug, got.errw)
			}
		}
	})

	t.Run("help pull prints the arguments and the thirteen checks in order", func(t *testing.T) {
		root := newBench(t)
		got := runCLI(t, root, "help", "pull")
		if got.code != 0 {
			t.Fatalf("help pull: %d %s", got.code, got.errw)
		}
		for _, argument := range []string{"column", "no-claim", "expires", "override"} {
			if !strings.Contains(got.out, argument) {
				t.Errorf("the help should name the %s argument, got %q", argument, got.out)
			}
		}
		rows := regexp.MustCompile(`(?m)^  (\d+) `).FindAllStringSubmatch(got.out, -1)
		if len(rows) != len(verb.Checks(verb.Pull)) {
			t.Fatalf("wanted %d numbered check rows, got %d", len(verb.Checks(verb.Pull)), len(rows))
		}
		// The refusal names are read off the page in one forward walk, so
		// the assertion is about their order and not only about their
		// presence. A name that wraps onto a continuation line still lands
		// after the row above it, which is why the walk reads the page
		// rather than the numbered row it started on.
		rest := got.out
		for i, check := range verb.Checks(verb.Pull) {
			at := strings.Index(rest, check.Refusal)
			if at < 0 {
				t.Fatalf("row %d: the help does not carry %s after the row above it; the page from there:\n%s", i+1, check.Refusal, rest)
			}
			rest = rest[at+len(check.Refusal):]
		}
	})
}

// guideLinesOutsideFences returns the lines of a rendered guide that sit
// outside its fenced blocks, which are the lines this card's wrap governs.
// A fenced block is reproduced whole and is the terminal's problem, so it is
// dropped here rather than measured.
func guideLinesOutsideFences(text string) []string {
	var prose []string
	inside := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inside = !inside
			continue
		}
		if inside {
			continue
		}
		prose = append(prose, line)
	}
	return prose
}

// TestAGuideIsWrappedToTheWindowItIsReadIn asserts dinah-239 AC-11: the body
// of a guide reaches the stream laid out for the window rather than raw, so
// different widths produce different pages and none of them reaches past the
// window it was asked for. Only the prose is measured; a fenced block is
// reproduced whole by design.
//
// The widths are a spread reaching down to the 20 windowWidth clamps to,
// rather than the 40 and 80 this test first carried. Wrapping that overruns
// by a marker's width is invisible at a roomy window and shows at a narrow
// one, so a pair of widths either side of the fault line proves the pair and
// not the rule. The whole-corpus form of this sweep lives in
// TestEveryShippedGuideFitsEveryWindowItIsWrappedFor, which measures
// wrapGuideText directly; this test's own job is the wiring, that runGuide
// reaches the wrap at all.
//
// The measure excuses the same shapes that sweep excuses, and it has to: at
// 20 columns principles.md carries the single 23-column token
// `columns/<id>/column.md`, which breakWords writes whole because it has
// nowhere to break, exactly as the spec's overflow rule says it will.
func TestAGuideIsWrappedToTheWindowItIsReadIn(t *testing.T) {
	root := newBench(t)
	widths := []int{20, 24, 32, 40, 56, 80}
	pages := map[int]string{}
	for _, width := range widths {
		t.Setenv("COLUMNS", strconv.Itoa(width))
		got := runCLI(t, root, "guide", "principles")
		if got.code != 0 {
			t.Fatalf("guide principles at %d columns: %d %s", width, got.code, got.errw)
		}
		for _, line := range guideLinesOutsideFences(got.out) {
			if guideIsIndentedCode(line) || guideIsTableRow(line) || guideWrapsNoFurther(line) {
				continue
			}
			if displayWidth(line) > width {
				t.Errorf("at %d columns a line draws %d: %q", width, displayWidth(line), line)
			}
		}
		pages[width] = got.out
	}
	for i := 1; i < len(widths); i++ {
		if pages[widths[i-1]] == pages[widths[i]] {
			t.Errorf("the guide reads the same at %d columns as at %d, so nothing wrapped it", widths[i-1], widths[i])
		}
	}
}

// TestAGuideTableSurvivesTheWindowItIsReadIn asserts dinah-239 AC-12: the
// reference guide's support table is reproduced rather than re-flowed, so the
// columns still line up under the check runCLI runs over every stream. The
// alignment is confirmed by running that check here rather than argued from
// the table's own authored padding.
func TestAGuideTableSurvivesTheWindowItIsReadIn(t *testing.T) {
	root := newBench(t)
	t.Setenv("COLUMNS", "40")
	got := runCLI(t, root, "guide", "references")
	if got.code != 0 {
		t.Fatalf("guide references at 40 columns: %d %s", got.code, got.errw)
	}
	for _, row := range []string{
		"| Command      | This workbench | A column | A card | Below a card |",
		"|--------------|----------------|---------|--------|--------------|",
		"| path         | yes            | yes     | yes    | yes          |",
		"| rename       | no             | no      | no     | yes          |",
	} {
		if !strings.Contains(got.out, row) {
			t.Errorf("the table lost the row %q at 40 columns", row)
		}
	}
}

// TestTheGuideListingIsUnchangedByTheBodyWrap asserts dinah-239 AC-13's first
// half: the topic listing is served by the branch this card did not touch, so
// the body wrap never reaches it and every topic still draws its whole title
// on one line, however narrow the window is. A listing routed through the
// body wrap would break those titles onto continuation lines at 40 columns,
// which is what this refuses.
func TestTheGuideListingIsUnchangedByTheBodyWrap(t *testing.T) {
	root := newBench(t)
	for _, width := range []string{"80", "40"} {
		t.Setenv("COLUMNS", width)
		got := runCLI(t, root, "guide")
		if got.code != 0 {
			t.Fatalf("guide at %s columns: %d %s", width, got.code, got.errw)
		}
		for _, topic := range guide.Topics() {
			row := topic + "  "
			at := strings.Index(got.out, row)
			if at < 0 {
				t.Fatalf("at %s columns the listing has no row for %q:\n%s", width, topic, got.out)
			}
			rest := got.out[at:]
			line := rest[:strings.IndexByte(rest, '\n')]
			if !strings.HasSuffix(line, guide.Title(topic)) {
				t.Errorf("at %s columns the listing wrapped the title of %q: %q", width, topic, line)
			}
		}
	}
}

// TestAnAttachmentCarryingNoOrdinalStillNumbersFromOne asserts dinah-186 AC-1,
// AC-2 and AC-21 on the branch that shipped broken: a workbench written before
// the ordinal field existed carries attachments whose anchor declares none, and
// both read surfaces numbered every one of them 0. A position column reading 0
// twice tells a reader the collection has no first member, and the ref built
// from that number answers to nothing, so the fallback has to run everywhere
// the number is printed rather than only where the ref is composed.
//
// The rows are checked as a set rather than in a fixed order. What the test
// holds is that the positions are 1 and 2, that they name different
// attachments, and that each printed ref resolves to the attachment whose row
// printed it. Which filename lands on which position is the separate question
// TestAPositionNamesTheSameAttachmentAcrossTheMigration answers.
func TestAnAttachmentCarryingNoOrdinalStillNumbersFromOne(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	sources := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		file := filepath.Join(sources, name)
		if err := os.WriteFile(file, []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if got := runCLI(t, root, "attach", "fx-1", file); got.code != 0 {
			t.Fatalf("attach %s: %d %s", name, got.code, got.errw)
		}
	}
	located := runCLI(t, root, "path", "fx-1")
	if located.code != 0 {
		t.Fatalf("path: %d %s", located.code, located.errw)
	}
	stripOrdinals(t, filepath.Join(filepath.Dir(strings.TrimSpace(located.out)), "attachments"))

	machine := runCLI(t, root, "--json", "show", "fx-1")
	if machine.code != 0 {
		t.Fatalf("show --json: %d %s", machine.code, machine.errw)
	}
	var detail struct {
		Attachments []verb.AttachmentView `json:"attachments"`
	}
	if err := json.Unmarshal([]byte(machine.out), &detail); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	if len(detail.Attachments) != 2 {
		t.Fatalf("wanted both attachments, got %d:\n%s", len(detail.Attachments), machine.out)
	}
	seen := map[int]string{}
	for _, attachment := range detail.Attachments {
		if attachment.Ordinal < 1 {
			t.Errorf("the attachment %s carries the position %d, and a position counts from one", attachment.Filename, attachment.Ordinal)
			continue
		}
		if held, taken := seen[attachment.Ordinal]; taken {
			t.Errorf("the position %d names both %s and %s", attachment.Ordinal, held, attachment.Filename)
		}
		seen[attachment.Ordinal] = attachment.Filename
		resolved := runCLI(t, root, "path", attachment.Ref)
		if resolved.code != 0 {
			t.Errorf("the printed ref %s resolves to nothing: %d %s", attachment.Ref, resolved.code, resolved.errw)
			continue
		}
		payload := runCLI(t, root, "path", attachment.Ref+"/payload")
		if payload.code != 0 {
			t.Errorf("the payload of %s resolves to nothing: %d %s", attachment.Ref, payload.code, payload.errw)
			continue
		}
		if got := filepath.Base(strings.TrimSpace(payload.out)); got != attachment.Filename {
			t.Errorf("the ref %s reaches the payload %s, and its row printed %s", attachment.Ref, got, attachment.Filename)
		}
	}
	if seen[1] == "" || seen[2] == "" {
		t.Errorf("wanted the positions 1 and 2, got %v", seen)
	}

	human := runCLI(t, root, "show", "fx-1")
	if human.code != 0 {
		t.Fatalf("show: %d %s", human.code, human.errw)
	}
	for position, filename := range seen {
		row := strconv.Itoa(position) + "  " + filename
		if !strings.Contains(human.out, row) {
			t.Errorf("the attachments block draws no row %q:\n%s", row, human.out)
		}
	}
	if strings.Contains(human.out, "0  ") {
		t.Errorf("the attachments block still numbers from zero:\n%s", human.out)
	}
}

// TestAPositionNamesTheSameAttachmentAcrossTheMigration asserts the ruling
// this card carries: `<card>/attachments/2` names the same file before and
// after check --migrate-ordinals runs on a workbench that predates the ordinal
// field.
//
// The attachments are made in a known order and then stripped of their
// ordinals, which is the shape a legacy anchor has on disk. The read path
// recovers that order from the card's journal, which is the same source the
// migration stamps from, so the migration writes down what the read already
// said instead of contradicting it.
func TestAPositionNamesTheSameAttachmentAcrossTheMigration(t *testing.T) {
	root, collection := legacyAttachmentsAgainstTheListing(t)

	before := shownAttachments(t, root)
	if len(before) != 2 {
		t.Fatalf("wanted both attachments, got %d", len(before))
	}
	if before[0].Filename != "a.txt" || before[1].Filename != "b.txt" {
		t.Fatalf("an unmigrated card reads its attachments out of the order they were attached: %s then %s",
			before[0].Filename, before[1].Filename)
	}
	refs := map[string]string{}
	for _, attachment := range before {
		refs[attachment.Filename] = attachment.Ref
	}

	if got := runCLI(t, root, "check", "--migrate-ordinals"); got.code != 0 {
		t.Fatalf("migrate: %d %s", got.code, got.errw)
	}
	wanted := map[string]int{"a.txt": 1, "b.txt": 2}
	for _, id := range bench.ListIDs(collection) {
		filename := attachedFilename(t, collection, id)
		if got := bench.EntityOrdinal(collection, id, bench.AttachmentAnchor); got != wanted[filename] {
			t.Errorf("the migration stamped %s with ordinal %d, wanted %d", filename, got, wanted[filename])
		}
	}

	after := shownAttachments(t, root)
	if len(after) != 2 {
		t.Fatalf("wanted both attachments after the migration, got %d", len(after))
	}
	for _, attachment := range after {
		if got := refs[attachment.Filename]; got != attachment.Ref {
			t.Errorf("%s was named %s before the migration and %s after it", attachment.Filename, got, attachment.Ref)
		}
		resolved := runCLI(t, root, "path", attachment.Ref+"/payload")
		if resolved.code != 0 {
			t.Errorf("the ref %s resolves to nothing: %d %s", attachment.Ref, resolved.code, resolved.errw)
			continue
		}
		if got := filepath.Base(strings.TrimSpace(resolved.out)); got != attachment.Filename {
			t.Errorf("the ref %s reaches the payload %s, and its row printed %s", attachment.Ref, got, attachment.Filename)
		}
	}
}

// TestAnUnstampedAttachmentStillReadsWithoutAJournal asserts that the read
// path degrades rather than refusing when the history it now consults is gone
// or unreadable.
//
// A journal is not a precondition of a read. The order it would have supplied
// is a repair of the listing order, so a card whose journal was deleted, and a
// card whose last line a crash tore, both read exactly the way every card read
// before this card landed: every attachment shown, every printed ref
// resolving.
func TestAnUnstampedAttachmentStillReadsWithoutAJournal(t *testing.T) {
	cases := []struct {
		name    string
		breakIt func(*testing.T, string)
	}{
		{
			name: "a deleted journal",
			breakIt: func(t *testing.T, journal string) {
				if err := os.Remove(journal); err != nil {
					t.Fatalf("remove journal: %v", err)
				}
			},
		},
		{
			name: "a torn journal tail",
			breakIt: func(t *testing.T, journal string) {
				f, err := os.OpenFile(journal, os.O_APPEND|os.O_WRONLY, 0o644)
				if err != nil {
					t.Fatalf("open journal: %v", err)
				}
				defer f.Close()
				if _, err := f.WriteString(`{"ts":"2026-08-1`); err != nil {
					t.Fatalf("tear journal: %v", err)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, collection := legacyAttachmentsAgainstTheListing(t)
			c.breakIt(t, filepath.Join(filepath.Dir(collection), bench.JournalName))

			human := runCLI(t, root, "show", "fx-1")
			if human.code != 0 {
				t.Fatalf("show: %d %s", human.code, human.errw)
			}
			shown := shownAttachments(t, root)
			if len(shown) != 2 {
				t.Fatalf("wanted both attachments, got %d", len(shown))
			}
			seen := map[int]string{}
			for _, attachment := range shown {
				if attachment.Ordinal < 1 {
					t.Errorf("%s carries the position %d, and a position counts from one", attachment.Filename, attachment.Ordinal)
					continue
				}
				if held, taken := seen[attachment.Ordinal]; taken {
					t.Errorf("the position %d names both %s and %s", attachment.Ordinal, held, attachment.Filename)
				}
				seen[attachment.Ordinal] = attachment.Filename
				if located := runCLI(t, root, "path", attachment.Ref); located.code != 0 {
					t.Errorf("the printed ref %s resolves to nothing: %d %s", attachment.Ref, located.code, located.errw)
					continue
				}
				payload := runCLI(t, root, "path", attachment.Ref+"/payload")
				if payload.code != 0 {
					t.Errorf("the printed ref %s resolves to nothing: %d %s", attachment.Ref, payload.code, payload.errw)
					continue
				}
				if got := filepath.Base(strings.TrimSpace(payload.out)); got != attachment.Filename {
					t.Errorf("the ref %s reaches the payload %s, and its row printed %s", attachment.Ref, got, attachment.Filename)
				}
			}
			if seen[1] == "" || seen[2] == "" {
				t.Errorf("wanted the positions 1 and 2, got %v", seen)
			}
		})
	}
}

// legacyAttachmentsAgainstTheListing builds a card carrying a.txt and b.txt,
// attached in that order, whose identifiers put b.txt first in the directory
// listing, and strips the ordinal from both anchors. It returns the workbench
// root and the attachments collection.
//
// The disagreement has to be built rather than assumed. An identifier is six
// random bytes, so the listing agrees with the attach order about half the
// time, and a fixture that took whichever it was handed would let the defect
// this card fixes through on every other run. Each attempt is a fresh
// workbench, and twenty-four of them miss the disagreement about one time in
// sixteen million.
func legacyAttachmentsAgainstTheListing(t *testing.T) (string, string) {
	t.Helper()
	for attempt := 0; attempt < 24; attempt++ {
		root := newBench(t)
		if got := runCLI(t, root, "add", "A card"); got.code != 0 {
			t.Fatalf("add: %d %s", got.code, got.errw)
		}
		sources := t.TempDir()
		for _, name := range []string{"a.txt", "b.txt"} {
			file := filepath.Join(sources, name)
			if err := os.WriteFile(file, []byte(name), 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
			if got := runCLI(t, root, "attach", "fx-1", file); got.code != 0 {
				t.Fatalf("attach %s: %d %s", name, got.code, got.errw)
			}
		}
		collection := attachmentsCollection(t, root)
		listed := bench.ListIDs(collection)
		if len(listed) != 2 {
			t.Fatalf("the card carries %d attachments, wanted two", len(listed))
		}
		if attachedFilename(t, collection, listed[0]) != "b.txt" {
			continue
		}
		stripOrdinals(t, collection)
		return root, collection
	}
	t.Fatal("twenty-four workbenches all listed a.txt first, which no longer looks like chance")
	return "", ""
}

// attachedFilename reads the filename one attachment's anchor carries.
func attachedFilename(t *testing.T, collection, id string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(collection, id, bench.AttachmentAnchor))
	if err != nil {
		t.Fatalf("read %s: %v", id, err)
	}
	fm, _ := bench.ParseAnchor(string(text))
	return fm.Value("filename")
}

// stripOrdinals removes the ordinal line from every anchor of an attachments
// collection, which is the shape a workbench written before the field existed
// has on disk. The rest of each anchor is left byte-identical, so what the
// read path meets is the legacy anchor rather than a rewritten one.
func stripOrdinals(t *testing.T, collection string) {
	t.Helper()
	for _, id := range bench.ListIDs(collection) {
		anchor := filepath.Join(collection, id, bench.AttachmentAnchor)
		text, err := os.ReadFile(anchor)
		if err != nil {
			t.Fatalf("read %s: %v", anchor, err)
		}
		var kept []string
		for _, line := range strings.Split(string(text), "\n") {
			if strings.HasPrefix(line, bench.OrdinalField+":") {
				continue
			}
			kept = append(kept, line)
		}
		if err := os.WriteFile(anchor, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
			t.Fatalf("write %s: %v", anchor, err)
		}
	}
}

// attachOne writes a file of the given name under its own directory and
// attaches it to fx-1, so that two attachments can carry one filename without
// the second write clobbering the first.
func attachOne(t *testing.T, root, name string, nth int) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), strconv.Itoa(nth))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, []byte(name), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	if got := runCLI(t, root, "attach", "fx-1", file); got.code != 0 {
		t.Fatalf("attach %s: %d %s", name, got.code, got.errw)
	}
}

// attachmentsCollection is the directory holding fx-1's attachments.
func attachmentsCollection(t *testing.T, root string) string {
	t.Helper()
	located := runCLI(t, root, "path", "fx-1")
	if located.code != 0 {
		t.Fatalf("path: %d %s", located.code, located.errw)
	}
	return filepath.Join(filepath.Dir(strings.TrimSpace(located.out)), "attachments")
}

// shownAttachments decodes the attachments block of show --json for fx-1.
func shownAttachments(t *testing.T, root string) []verb.AttachmentView {
	t.Helper()
	machine := runCLI(t, root, "--json", "show", "fx-1")
	if machine.code != 0 {
		t.Fatalf("show --json: %d %s", machine.code, machine.errw)
	}
	var detail struct {
		Attachments []verb.AttachmentView `json:"attachments"`
	}
	if err := json.Unmarshal([]byte(machine.out), &detail); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	return detail.Attachments
}

// TestAGappedAttachmentCollectionAgreesAcrossEveryReadSurface asserts dinah-186
// AC-1 and AC-2 on the collection the earlier rounds never built: one a delete
// has left with a hole in its stored ordinals.
//
// The two tests that came before this one attach and never delete, so their
// stored ordinals run 1, 2, 3 with no gaps, and on such a collection a stored
// ordinal and a position happen to be the same number. NextOrdinal hands out
// highest-plus-one, so the hole a delete leaves is permanent, and from then on
// the two disagree for every member after it. A display reading the stored
// field then labels a row with a number the resolver answers to some other
// file, or to no file, which is how following show into rename renamed the
// wrong attachment.
//
// So the collection here is gapped on purpose, and what the test holds is that
// show, contents and path name one file per position: the positions run 1 and
// 2 with no hole in them, each ref show prints reaches the payload its own row
// named, and contents draws the same refs show does.
func TestAGappedAttachmentCollectionAgreesAcrossEveryReadSurface(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for nth, name := range []string{"one.txt", "two.txt", "three.txt"} {
		attachOne(t, root, name, nth)
	}
	if got := runCLI(t, root, "delete", "fx-1/attachments/1", "--yes"); got.code != 0 {
		t.Fatalf("delete: %d %s", got.code, got.errw)
	}

	shown := shownAttachments(t, root)
	if len(shown) != 2 {
		t.Fatalf("wanted the two surviving attachments, got %d", len(shown))
	}
	seen := map[int]string{}
	for _, attachment := range shown {
		if held, taken := seen[attachment.Ordinal]; taken {
			t.Errorf("the position %d names both %s and %s", attachment.Ordinal, held, attachment.Filename)
		}
		seen[attachment.Ordinal] = attachment.Filename
		if attachment.Ref != "fx-1/attachments/"+strconv.Itoa(attachment.Ordinal) {
			t.Errorf("the row at position %d prints the ref %s", attachment.Ordinal, attachment.Ref)
		}
		payload := runCLI(t, root, "path", attachment.Ref+"/payload")
		if payload.code != 0 {
			t.Errorf("the printed ref %s reaches no payload: %d %s", attachment.Ref, payload.code, payload.errw)
			continue
		}
		if got := filepath.Base(strings.TrimSpace(payload.out)); got != attachment.Filename {
			t.Errorf("the ref %s reaches the payload %s, and its row printed %s", attachment.Ref, got, attachment.Filename)
		}
	}
	if seen[1] == "" || seen[2] == "" {
		t.Errorf("a delete left a hole in the positions: wanted 1 and 2, got %v", seen)
	}
	if gone := runCLI(t, root, "path", "fx-1/attachments/3"); gone.code == 0 {
		t.Errorf("the collection holds two attachments and answers to a third position: %s", gone.out)
	}

	human := runCLI(t, root, "show", "fx-1")
	if human.code != 0 {
		t.Fatalf("show: %d %s", human.code, human.errw)
	}
	listed := runCLI(t, root, "contents", "fx-1")
	if listed.code != 0 {
		t.Fatalf("contents: %d %s", listed.code, listed.errw)
	}
	for position, filename := range seen {
		row := strconv.Itoa(position) + "  " + filename
		if !strings.Contains(human.out, row) {
			t.Errorf("the attachments block draws no row %q:\n%s", row, human.out)
		}
		ref := "fx-1/attachments/" + strconv.Itoa(position)
		if !strings.Contains(listed.out, ref) {
			t.Errorf("contents draws no %s, which is the ref show printed for %s:\n%s", ref, filename, listed.out)
		}
	}
	if strings.Contains(listed.out, "fx-1/attachments/3") {
		t.Errorf("contents draws a third position the collection has no member at:\n%s", listed.out)
	}
}

// TestTheAmbiguousNameRefusalNamesPositionsThatResolve asserts dinah-186 AC-5
// on the two collections where a stored ordinal is not a position: the
// unstamped one this card exists to repair, and the gapped one a delete makes.
//
// The refusal tells the caller to retry as attachments/<n>, so the numbers it
// prints have to be numbers that arm answers. Reading them off the anchor gave
// "0,0" on an unstamped collection, where attachments/0 reaches nothing at all,
// and gave a number past the end of a gapped one. Both boards drew a table
// saying 1 and 2 in the same session as a sentence saying something else.
func TestTheAmbiguousNameRefusalNamesPositionsThatResolve(t *testing.T) {
	for _, shape := range []struct {
		name  string
		build func(t *testing.T, root string)
	}{
		{
			name: "an unstamped collection",
			build: func(t *testing.T, root string) {
				for nth, name := range []string{"dup.txt", "dup.txt"} {
					attachOne(t, root, name, nth)
				}
				stripOrdinals(t, attachmentsCollection(t, root))
			},
		},
		{
			name: "a collection a delete has gapped",
			build: func(t *testing.T, root string) {
				for nth, name := range []string{"first.txt", "dup.txt", "dup.txt"} {
					attachOne(t, root, name, nth)
				}
				if got := runCLI(t, root, "delete", "fx-1/attachments/1", "--yes"); got.code != 0 {
					t.Fatalf("delete: %d %s", got.code, got.errw)
				}
			},
		},
	} {
		t.Run(shape.name, func(t *testing.T) {
			root := newBench(t)
			if got := runCLI(t, root, "add", "A card"); got.code != 0 {
				t.Fatalf("add: %d %s", got.code, got.errw)
			}
			shape.build(t, root)

			var wanted []string
			for _, attachment := range shownAttachments(t, root) {
				if attachment.Filename == "dup.txt" {
					wanted = append(wanted, strconv.Itoa(attachment.Ordinal))
				}
			}
			sort.Strings(wanted)
			if len(wanted) != 2 {
				t.Fatalf("wanted two rows named dup.txt, got %v", wanted)
			}

			refused := runCLI(t, root, "path", "fx-1/attachments/dup.txt")
			if refused.code == 0 {
				t.Fatalf("two attachments answer to dup.txt and the path resolved: %s", refused.out)
			}
			if !strings.Contains(refused.errw, contract.AmbiguousName) {
				t.Fatalf("wanted %s, got: %s", contract.AmbiguousName, refused.errw)
			}
			if joined := strings.Join(wanted, ","); !strings.Contains(refused.errw, joined) {
				t.Errorf("the table draws the positions %s and the refusal says something else: %s", joined, refused.errw)
			}
			for _, position := range wanted {
				ref := "fx-1/attachments/" + position
				resolved := runCLI(t, root, "path", ref+"/payload")
				if resolved.code != 0 {
					t.Errorf("the refusal names %s and it reaches nothing: %d %s", ref, resolved.code, resolved.errw)
					continue
				}
				if got := filepath.Base(strings.TrimSpace(resolved.out)); got != "dup.txt" {
					t.Errorf("the refusal names %s and it reaches %s rather than a dup.txt", ref, got)
				}
			}
		})
	}
}

// TestEverySplicedFragmentCarriesItsOwnSeparator asserts dinah-186 AC-5's
// rule at the catalog rather than at one refusal: a fragment that renders
// onto the end of the refusal's own sentence has to begin with the
// punctuation that joins it there, because renderRefusal concatenates the two
// with nothing between them.
//
// The shipped ambiguous-name refusal read "sketch.txtname one as
// attachments/<ordinal>", and the catalog test that holds a translation to
// its base entry's splice could not catch it, since the base entry was the
// one at fault. A shape drawing a listing or a carried set is exempt: its
// fragment renders as a line of its own, where a leading separator would be
// the mistake.
func TestEverySplicedFragmentCarriesItsOwnSeparator(t *testing.T) {
	catalog := msg.For(msg.Base)
	for _, shape := range contract.Shapes {
		if shape.Listing != "" || shape.Carried != "" {
			continue
		}
		for _, fragment := range shape.Fragments {
			text := catalog.T(fragment.Key)
			if text == "" {
				continue
			}
			if !strings.ContainsRune(";,. ", rune(text[0])) {
				t.Errorf("%s begins %q, and it is spliced onto the end of a sentence with nothing between them", fragment.Key, text)
			}
		}
	}
}

// usageRefusalIn composes the stderr a parse-time dinah.usage refusal prints
// in one language, which is the refusal name, the sentence naming the word,
// the next step, and the two-dash hint. The tests below compare against this
// rather than against the absence of English, because the catalogs keep the
// product name and the flag spellings in Latin script on purpose.
func usageRefusalIn(r *msg.Renderer, detail string) string {
	return contract.Usage + " " +
		r.T("refusal.dinah.usage", "detail", detail) +
		r.T("refusal.dinah.usage.next") +
		r.T("refusal.dinah.usage.dash-hint") + "\n"
}

// TestLangFlagIsHonouredWhateverItsPosition asserts dinah-97's AC-1, AC-3,
// AC-4 and the rendering half of its AC-8. A --lang written after the word
// that fails to parse reaches the reader exactly as one written before it
// does, so the same mistake typed in either order is answered in the same
// language. DINAH_LANG is cleared and the fixture configures no lang, so the
// German and Hindi answers below can only have come from the flag.
func TestLangFlagIsHonouredWhateverItsPosition(t *testing.T) {
	root := newBench(t)
	t.Setenv("DINAH_LANG", "")
	german := msg.For("de")
	hindi := msg.For("hi")
	english := msg.For(msg.Base)
	if usageRefusalIn(german, "--nosuchflag") == usageRefusalIn(english, "--nosuchflag") {
		t.Fatalf("the fixture is not testing anything: de and en render this refusal the same way")
	}

	// The control for the absence assertion in the loop below. Without it,
	// stderr that never names a flag at all would satisfy "no second
	// complaint about --lang" word for word, and that reading is the more
	// likely of the two. dinah --lang, whose only word is a session flag
	// with no value left, is the invocation whose refusal does name --lang.
	if named := runCLI(t, root, "--lang"); !strings.Contains(named.errw, "--lang") {
		t.Fatalf("no refusal names --lang at all, so the absence checks below prove nothing: %q", named.errw)
	}

	// The example of "some other flag that takes a value" is read out of the
	// declared flag tables rather than spelled here, so that renaming a flag
	// cannot leave this case naming a word the parser no longer recognizes.
	// Such a case goes on passing while the value-slot scenario it exists to
	// exercise has stopped happening, since an unrecognized word claims no
	// value slot for the --lang behind it to sit in.
	valueSlotFlag, _ := exampleValuedFlags(t)
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "the flag ahead of the word that fails to parse",
			argv: []string{"--lang", "de", "--nosuchflag"},
			want: usageRefusalIn(german, "--nosuchflag"),
		},
		{
			name: "the flag behind the word that fails to parse",
			argv: []string{"--nosuchflag", "--lang", "de"},
			want: usageRefusalIn(german, "--nosuchflag"),
		},
		{
			name: "an incomplete flag behind it falls through to English",
			argv: []string{"--nosuchflag", "--lang"},
			want: usageRefusalIn(english, "--nosuchflag"),
		},
		{
			name: "the last complete flag on the line wins",
			argv: []string{"--lang", "de", "--nosuchflag", "--lang", "hi"},
			want: usageRefusalIn(hindi, "--nosuchflag"),
		},
		{
			name: "a flag in " + valueSlotFlag + "'s value slot behind it is not a language choice",
			argv: []string{"--nosuchflag", "--" + valueSlotFlag, "--lang", "de"},
			want: usageRefusalIn(english, "--nosuchflag"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := runCLI(t, root, c.argv...)
			if got.code != 2 {
				t.Fatalf("dinah %s exited %d, wanted 2", strings.Join(c.argv, " "), got.code)
			}
			if got.errw != c.want {
				t.Errorf("dinah %s printed the wrong refusal:\n got  %q\n want %q", strings.Join(c.argv, " "), got.errw, c.want)
			}
			if strings.Count(got.errw, "--lang") != 0 {
				t.Errorf("dinah %s raised a second complaint about --lang: %q", strings.Join(c.argv, " "), got.errw)
			}
		})
	}
}

// TestLangPastTheEndOfOptionsMarkerIsLiteralText asserts dinah-97's AC-5: the
// scan that reads --lang stops at the POSIX marker exactly as the parse does,
// so `dinah add -- --lang de` hands both words to add as its title and is
// answered in English. Two words in that slot is itself a refusal under
// dinah-100's one-word rule, which is what this reads back.
func TestLangPastTheEndOfOptionsMarkerIsLiteralText(t *testing.T) {
	root := newBench(t)
	t.Setenv("DINAH_LANG", "")
	german := msg.For("de")
	english := msg.For(msg.Base)
	if german.T("slot.title") == english.T("slot.title") {
		t.Fatalf("the fixture is not testing anything: de and en render slot.title the same way")
	}

	got := runCLI(t, root, "add", "--", "--lang", "de")
	if got.code != 2 {
		t.Fatalf("dinah add -- --lang de exited %d, wanted 2", got.code)
	}
	wantSentence := english.T("refusal.dinah.multiple-words", "count", "2", "label", english.T("slot.title"))
	if !strings.Contains(got.errw, wantSentence) {
		t.Errorf("the refusal should be the English multiple-words sentence:\n got      %q\n wanted a %q", got.errw, wantSentence)
	}
	if strings.Contains(got.errw, german.T("slot.title")) {
		t.Errorf("a --lang past the marker set the language anyway: %q", got.errw)
	}
}
