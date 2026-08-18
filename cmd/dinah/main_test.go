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
// cover. It also clears COLUMNS for the whole run, so a shell that exports
// it does not reach a test that never asked to see it.
func TestMain(m *testing.M) {
	restoreTemp := testenv.IsolateTempDir()
	restoreColumns := isolateColumns()
	code := m.Run()
	restoreColumns()
	restoreTemp()
	os.Exit(code)
}

// isolateColumns clears COLUMNS for the whole test binary before any test
// runs, restoring whatever the environment held once every test has
// finished. windowWidth (row.go) reads COLUMNS straight from the
// environment, so an exported COLUMNS reaches every test in this package,
// not only the ones that set it on purpose. A handful of tests already call
// t.Setenv("COLUMNS", ...) to control the value they need, and that call
// keeps working exactly as before: t.Setenv overrides for the one test and
// restores automatically when it ends, so it composes with an unset starting
// point the same way it composes with any other. What clearing it here buys
// is every test that never mentions COLUMNS at all, present or future,
// which is where the hazard actually lives.
func isolateColumns() (restore func()) {
	prev, had := os.LookupEnv("COLUMNS")
	os.Unsetenv("COLUMNS")
	return func() {
		if had {
			os.Setenv("COLUMNS", prev)
			return
		}
		os.Unsetenv("COLUMNS")
	}
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
		if !blockLists(string(fixture), verb.Usage(c.name)) {
			t.Errorf("the block does not list %s", c.name)
		}
	}
	if listed != 29 {
		t.Errorf("wanted twenty-nine listed commands, got %d", listed)
	}
}

// blockLists reports whether the ratified help block carries a command's
// usage line under the block's own indent. A usage that fits its column is
// followed by the padding before its summary, and one that reaches the column
// takes the rest of its line and is followed by the line ending instead, so
// asking for a trailing space alone would read a wrapped entry as a missing
// one.
func blockLists(block, usage string) bool {
	return strings.Contains(block, "  "+usage+" ") || strings.Contains(block, "  "+usage+"\n")
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
		"create a workbench here, optionally from a template",
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
	if !blockLists(string(fixture), "check [--finish] [--migrate-ordinals] [--migrate-slugs] [--migrate-states]") {
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

// TestStatesRenderNamesTheRepairInsteadOfPaddingBlank asserts AC-3: a state
// with no slug prints a catalog-served placeholder naming the repair, not an
// equal-width run of spaces indistinguishable from a rendering glitch.
func TestStatesRenderNamesTheRepairInsteadOfPaddingBlank(t *testing.T) {
	root := newBench(t)
	stripSlugs(t, root)
	got := runCLI(t, root, "states")
	if got.code != 0 {
		t.Fatalf("states: %d %s", got.code, got.errw)
	}
	if !strings.Contains(got.out, "no slug") || !strings.Contains(got.out, "migrate-slugs") {
		t.Errorf("the listing should name the repair for a state with no slug:\n%s", got.out)
	}
	for _, title := range []string{"Intake", "Doing", "Done"} {
		if !strings.Contains(got.out, title) {
			t.Errorf("the listing should still carry state %s:\n%s", title, got.out)
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

// TestCheckMigrateStatesNamesWhatItRemovedOnTheTerminal asserts the terminal
// rendering of the stranded-state repair, not just the internal function it
// calls: a clean check and a real repair must not print the same line, and
// the repair must say which state identifier it took out of the list.
//
// dinah check --migrate-states edits workbench.md whether or not the caller
// is told, so this is the one place that confirms the edit is also reported;
// a change to the internal repair function alone would leave this test
// passing or failing on its own, with no dependency on the renderer at all.
func TestCheckMigrateStatesNamesWhatItRemovedOnTheTerminal(t *testing.T) {
	root := newBench(t)
	gone := strandState(t, root, 2)

	clean := runCLI(t, root, "check")
	if clean.code == 0 {
		t.Fatalf("a stranded state should be reported, not pass clean")
	}
	if !strings.Contains(clean.out, gone) {
		t.Errorf("the checker did not name the stranded state:\n%s", clean.out)
	}

	migrated := runCLI(t, root, "check", "--migrate-states")
	if migrated.code != 0 {
		t.Fatalf("check --migrate-states: %d %s", migrated.code, migrated.errw)
	}
	if !strings.Contains(migrated.out, "Removed 1 stranded state") {
		t.Errorf("the migration did not say how many states it removed:\n%s", migrated.out)
	}
	if !strings.Contains(migrated.out, gone) {
		t.Errorf("the migration did not name the state it removed:\n%s", migrated.out)
	}

	again := runCLI(t, root, "check", "--migrate-states")
	if again.code != 0 {
		t.Fatalf("a second run: %d %s", again.code, again.errw)
	}
	if strings.Contains(again.out, gone) {
		t.Errorf("a second run should have nothing left to name:\n%s", again.out)
	}
	if !strings.Contains(again.out, "Removed 0 stranded states") {
		t.Errorf("a second run did not say it removed nothing:\n%s", again.out)
	}
}

// strandState hand-strands one state of a workbench the way retirement's own
// pre-fix defect used to: it removes the state's directory without touching
// workbench.md's states list, and returns the identifier left dangling.
func strandState(t *testing.T, root string, position int) string {
	t.Helper()
	machine := runCLI(t, root, "--json", "states")
	var states []verb.StateView
	if err := json.Unmarshal([]byte(machine.out), &states); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	if position >= len(states) {
		t.Fatalf("the workbench carries %d states", len(states))
	}
	id := states[position].ID
	dir := filepath.Join(benchDir(t, root), bench.StatesDir, id)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove %s: %v", dir, err)
	}
	return id
}

// strandAllStates hand-strands every state a fresh newBench declares, one at
// a time: each call to strandState re-reads the live list, which shrinks by
// one member as its predecessor is stranded, so position 0 always names
// whichever state is still live. It returns the stranded identifiers in the
// order they were stranded, leaving workbench.md's raw states list
// unchanged and every id on it stranded, which is the shape a real
// workbench reaches after every other state was already retired and this
// last one was retired or removed under the pre-dinah-49 code.
func strandAllStates(t *testing.T, root string) []string {
	t.Helper()
	machine := runCLI(t, root, "--json", "states")
	var states []verb.StateView
	if err := json.Unmarshal([]byte(machine.out), &states); err != nil {
		t.Fatalf("decode: %v\n%s", err, machine.out)
	}
	gone := make([]string, 0, len(states))
	for range states {
		gone = append(gone, strandState(t, root, 0))
	}
	return gone
}

// TestCheckMigrateStatesRefusesRatherThanEmptyingTheDefinition asserts AC-2:
// dinah check --migrate-states against a workbench whose states list is
// entirely stranded ids exits 2 with the new refusal, leaves workbench.md
// unchanged, and a following plain dinah check reports the same
// check.stranded-state finding(s) it would have reported before the
// migration attempt.
func TestCheckMigrateStatesRefusesRatherThanEmptyingTheDefinition(t *testing.T) {
	root := newBench(t)
	anchor := filepath.Join(benchDir(t, root), bench.WorkbenchAnchor)
	gone := strandAllStates(t, root)

	before, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read anchor: %v", err)
	}

	migrated := runCLI(t, root, "check", "--migrate-states")
	if migrated.code != 2 {
		t.Fatalf("check --migrate-states: wanted exit 2, got %d\n%s", migrated.code, migrated.errw)
	}
	if !strings.Contains(migrated.errw, contract.RepairWouldEmptyStates) {
		t.Errorf("wanted the repair-would-empty-states refusal, got:\n%s", migrated.errw)
	}
	for _, id := range gone {
		if !strings.Contains(migrated.errw, id) {
			t.Errorf("the refusal did not name the stranded state %s:\n%s", id, migrated.errw)
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
		t.Fatalf("a following check should still report the stranded states")
	}
	for _, id := range gone {
		if !strings.Contains(plain.out, id) {
			t.Errorf("the following check did not name %s:\n%s", id, plain.out)
		}
	}
}

// TestAddRefusesWithNoLiveStates asserts AC-8's CLI-level half: dinah add
// against a workbench whose states list is entirely stranded prints and
// exits on the AddNeedsAState refusal, naming the workbench.md path and the
// dinah check / dinah add follow-up, and creates no card directory.
func TestAddRefusesWithNoLiveStates(t *testing.T) {
	root := newBench(t)
	dir := benchDir(t, root)
	anchor := filepath.Join(dir, bench.WorkbenchAnchor)
	strandAllStates(t, root)

	got := runCLI(t, root, "add", "stranded card")
	if got.code != 2 {
		t.Fatalf("add: wanted exit 2, got %d\n%s", got.code, got.errw)
	}
	if !strings.Contains(got.errw, contract.AddNeedsAState) {
		t.Errorf("wanted the add-needs-a-state refusal, got:\n%s", got.errw)
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
	at := startColumnOf(lines[0], msg.For(msg.Base).T("column.workbenches.path"))
	if at < 0 {
		t.Fatalf("the listing carries no path heading:\n%s", got.out)
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

// TestAForeignWorkbenchFileIsPassedOverByTheClimb asserts AC-7: a directory
// holding a workbench.md that carries none of profile, format or states,
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
	if got.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("a workbench carrying a finding exits refused, got %d %q", got.code, got.errw)
	}
	catalog := msg.For(msg.Base)
	if !strings.Contains(got.out, catalog.T(bench.FindingIgnoredAnchor)) || !strings.Contains(got.out, foreign) {
		t.Errorf("wanted a check.ignored-anchor finding naming %q, got %q", foreign, got.out)
	}
}

// TestTheOverrideSkipsRecognitionAndLeavesItToOpen asserts AC-9: --workbench
// and DINAH_WORKBENCH test only file presence, unchanged. A recognition
// problem in the pointed-at file (no frontmatter carrying profile, format or
// states at all) is reported by Open's existing malformed refusal rather than
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

// TestLegalMovesReportTheAliasNotTheBareStateIdentifier is armed by
// asserting the ref field of a legal move rather than its state identifier.
// The moves-this-card-may-make listing, printed after claim, move and show,
// used to print the destination's bare identifier while move itself already
// accepted the state's slug, so the one place the tool told a person what to
// type next showed them something they could not comfortably type.
func TestLegalMovesReportTheAliasNotTheBareStateIdentifier(t *testing.T) {
	root := newBench(t)
	first := addCard(t, root, "First")

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
			t.Fatalf("legal move to %s carries no ref: %+v", move.State, move)
		}
		if move.Ref == move.State {
			t.Errorf("legal move to %s: ref fell back to the bare identifier though the state carries a slug", move.State)
		}
		// The ref is what move actually accepts, proving it is not merely
		// displayed but usable.
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
	for _, name := range []string{"status", "states", "version", "workbenches", "export", "mcp", "check", "whoami"} {
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
// every command whose bounded slot is a card reference, a state name, or a
// guide/help topic refuses with dinah.usage naming the word when a
// single-dash word fills that slot, rather than today's unknown-card,
// unknown-state, unknown-command or unknown-guide refusal.
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
		{"move's state slot", []string{"move", "fx-1", "-w"}},
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

	piped := &bytes.Buffer{}
	piped.WriteString("piped comment text")
	out, errw := &bytes.Buffer{}, &bytes.Buffer{}
	previous, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	code := run([]string{"comment", "fx-1", "-"}, piped, out, errw)
	os.Chdir(previous)
	if code != 0 {
		t.Fatalf("comment fx-1 -: wanted exit 0, got %d (%s)", code, errw.String())
	}

	// A single-dash word given as an explicit flag's own value is unaffected
	// and still reaches the domain refusal, not dinah.usage.
	got = runCLI(t, root, "move", "fx-1", "nowhere", "--state", "-w")
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if leading != contract.UnknownState {
		t.Errorf("an explicit flag value: wanted %s, got %q", contract.UnknownState, got.errw)
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
	for _, name := range []string{"status", "states", "version", "workbenches", "export", "mcp", "check", "whoami"} {
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
// scan itself (config set's value is allowed to be empty, unlike comment's
// text, so it is the vehicle for that half of the case), and a second "--"
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
		t.Errorf("config get actor: wanted the empty value the marker left behind, got %d %q", got.code, got.out)
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

	got := runCLI(t, root, "add", "the rollout failed because of", "--state", "doing")
	if got.code != 0 {
		t.Fatalf("add trailing --state: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	added := showDetail(t, root, "fx-1")
	if added.Card.Title != "the rollout failed because of" {
		t.Errorf("title: wanted %q, got %q", "the rollout failed because of", added.Card.Title)
	}
	if added.Card.StateTitle != "Doing" {
		t.Errorf("state: wanted Doing, got %q", added.Card.StateTitle)
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

	got := runCLI(t, root, "add", "--state", "doing", "the rollout failed")
	if got.code != 0 {
		t.Fatalf("add with a leading --state: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	added := showDetail(t, root, "fx-1")
	if added.Card.Title != "the rollout failed" {
		t.Errorf("title: wanted %q, got %q", "the rollout failed", added.Card.Title)
	}
	if added.Card.StateTitle != "Doing" {
		t.Errorf("state: wanted Doing, got %q", added.Card.StateTitle)
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

	got := runCLI(t, root, "comment", "fx-1", "please --state deploy done")
	if got.code != 0 {
		t.Fatalf("comment: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	detail := showDetail(t, root, "fx-1")
	if len(detail.Comments) != 1 || detail.Comments[0].Body != "please --state deploy done" {
		t.Fatalf("wanted one comment %q, got %v", "please --state deploy done", detail.Comments)
	}

	_, dir := settingsHome(t)
	got = runCLI(t, dir, "config", "set", "actor", "please --state deploy done")
	if got.code != 0 {
		t.Fatalf("config set: wanted exit 0, got %d (%s)", got.code, got.errw)
	}
	got = runCLI(t, dir, "config", "get", "actor")
	if got.code != 0 || strings.TrimSpace(got.out) != "please --state deploy done" {
		t.Errorf("config get actor: wanted %q, got %d %q", "please --state deploy done", got.code, got.out)
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

	wantUsage(t, runCLI(t, root, "move", "fx-1", "doing", "--state"), "--state")
}

// TestANonTrailingFlagNowReachesValidationInsteadOfLiteralText asserts the
// reverse of dinah-96's AC-18/D-7, dinah-100 (D-3): add's own domain flag is
// read as a flag wherever it is typed relative to the one-word free text,
// leading or trailing, so a bogus value now reaches add's own downstream
// state check and refuses, in place of the literal text dinah-96 would have
// accepted for the same input.
func TestANonTrailingFlagNowReachesValidationInsteadOfLiteralText(t *testing.T) {
	root := newBench(t)

	got := runCLI(t, root, "add", "--state", "bogus", "the rollout failed")
	leading := strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0]
	if got.code == 0 || leading != contract.UnknownState {
		t.Errorf("add with a leading bogus --state: wanted %s, got %d (%s)", contract.UnknownState, got.code, got.errw)
	}
}
