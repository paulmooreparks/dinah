package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/setup"
	"dinah/internal/testenv"
	"dinah/internal/verb"
)

// setupFixture is one throwaway machine for dinah setup: a home directory
// that HOME, USERPROFILE and DINAH_HOME all name, and a project holding one
// workbench. Every variable a run reads is set here, so the tests carry their
// own identity and nothing reaches a real home.
type setupFixture struct {
	root, home, project, workbench string
}

// newSetupFixture builds a fixture and points the process's home variables
// at it for the test's lifetime.
func newSetupFixture(t *testing.T) *setupFixture {
	t.Helper()
	root := t.TempDir()
	f := &setupFixture{root: root, home: filepath.Join(root, "home"), project: filepath.Join(root, "proj")}
	for _, dir := range []string{f.home, f.project} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", f.home)
	t.Setenv("USERPROFILE", f.home)
	t.Setenv("DINAH_HOME", f.home)
	t.Setenv("DINAH_ACTOR", "")
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	if got := runCLI(t, f.project, "init", "--slug", "sx", "--operator", "ana"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	f.workbench = soleBenchDir(t, f.project)
	return f
}

// write puts files under a directory of the fixture.
func (f *setupFixture) write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// setupRecipe writes a recipe of one project-scope write-file step, with any
// extra steps appended, into a directory named for it, and returns the
// directory.
func setupRecipe(t *testing.T, parent, name string, extraSteps ...string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	steps := []string{`{"id": "rules", "kind": "write-file", "scope": "project", "path": "rules.md", "template": "rules.md"}`}
	steps = append(steps, extraSteps...)
	files := map[string]string{
		"recipe.json":    `{"format": 1, "name": "` + name + `", "title": "Trial", "provider": "acme", "agent": "helper", "tools": "all", "scopes": ["project"], "documentation": ["https://example.com/trial"]}`,
		"steps.json":     `{"steps": [` + strings.Join(steps, ", ") + `]}`,
		"files/rules.md": "Rules for {{agent}}.\n",
		"prompt.md":      "Finish the rest in {{base}}.\n",
		"remove.md":      "Undo the rest in {{base}}.\n",
	}
	for file, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runStep is a run step's JSON, naming a program and its arguments.
func runStep(t *testing.T, id, program string, args ...string) string {
	t.Helper()
	if args == nil {
		args = []string{}
	}
	step := map[string]any{"id": id, "kind": "run", "scope": "project", "program": program, "args": args}
	data, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// refusedWith fails unless an invocation was refused with a name.
func refusedWith(t *testing.T, label string, got invocation, name string) {
	t.Helper()
	if got.code != 2 || !strings.HasPrefix(got.errw, name+" ") && strings.TrimSpace(got.errw) != name {
		t.Errorf("%s: wanted %s, got exit %d\nstdout %s\nstderr %s", label, name, got.code, got.out, got.errw)
	}
}

// accepted fails unless an invocation exited 0.
func accepted(t *testing.T, label string, got invocation) {
	t.Helper()
	if got.code != 0 {
		t.Errorf("%s: wanted exit 0, got %d\nstdout %s\nstderr %s", label, got.code, got.out, got.errw)
	}
}

// setupCheckRow is one row of setup's precondition list as a test drives it:
// an invocation that fails that row alone, and one beside it that passes it.
type setupCheckRow struct {
	refusal string
	refuse  func(t *testing.T, f *setupFixture) invocation
	accept  func(t *testing.T, f *setupFixture) invocation
}

// setupCheckRows are the eighteen rows of specification section 3.2, in
// order. The refusal names are written out rather than read off the verb
// table, so a reordering there fails here.
func setupCheckRows() []setupCheckRow {
	return []setupCheckRow{
		{contract.Usage,
			func(t *testing.T, f *setupFixture) invocation { return runCLI(t, f.project, "setup") },
			func(t *testing.T, f *setupFixture) invocation { return runCLI(t, f.project, "setup", "--list") }},
		{contract.MalformedHarness,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "Claude_Code", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--dry-run")
			}},
		{contract.UnknownPath,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "--recipe", filepath.Join(f.root, "absent"), "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "--recipe", setupRecipe(t, f.root, "trial"), "--dry-run")
			}},
		{contract.UnknownRecipe,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "no-such-harness", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "codex", "--dry-run")
			}},
		{contract.MalformedRecipe,
			func(t *testing.T, f *setupFixture) invocation {
				dir := setupRecipe(t, filepath.Join(f.home, ".dinah", "recipes"), "broken")
				f.write(t, dir, map[string]string{"recipe.json": `{"format": 2}`})
				return runCLI(t, f.project, "setup", "broken", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				setupRecipe(t, filepath.Join(f.home, ".dinah", "recipes"), "whole")
				return runCLI(t, f.project, "setup", "whole", "--dry-run")
			}},
		{contract.UntrustedRecipe,
			func(t *testing.T, f *setupFixture) invocation {
				setupRecipe(t, filepath.Join(f.project, ".dinah", "recipes"), "local")
				return runCLI(t, f.project, "setup", "local", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				setupRecipe(t, filepath.Join(f.project, ".dinah", "recipes"), "local")
				return runCLI(t, f.project, "setup", "local", "--dry-run", "--trust-project-recipe")
			}},
		{contract.UnknownScope,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "codex", "--scope", "user", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "codex", "--scope", "project", "--dry-run")
			}},
		{contract.UnknownToolProfile,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--tools", "everything", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--tools", "station", "--dry-run")
			}},
		{contract.Usage,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--remove", "--agent", "other")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--remove")
			}},
		{contract.SetupRunNotAllowed,
			func(t *testing.T, f *setupFixture) invocation {
				dir := setupRecipe(t, f.root, "runs", runStep(t, "register", "some-program", "a"))
				return runCLI(t, f.project, "setup", "--recipe", dir)
			},
			func(t *testing.T, f *setupFixture) invocation {
				dir := setupRecipe(t, f.root, "runs", runStep(t, "register", "some-program", "a"))
				return runCLI(t, f.project, "setup", "--recipe", dir, "--dry-run")
			}},
		{contract.NoWorkbenchFound,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.root, "setup", "claude-code", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.root, "setup", "claude-code", "--scope", "user", "--dry-run")
			}},
		{contract.SetupRelocatedHome,
			func(t *testing.T, f *setupFixture) invocation {
				elsewhere := filepath.Join(f.root, "elsewhere")
				if err := os.MkdirAll(elsewhere, 0o755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("DINAH_HOME", elsewhere)
				defer t.Setenv("DINAH_HOME", f.home)
				return runCLI(t, f.project, "setup", "claude-code", "--scope", "user", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--scope", "user", "--dry-run")
			}},
		{contract.SetupNoTarget,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--target", f.home, "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--target", f.project, "--dry-run")
			}},
		{contract.Malformed,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--agent", "two words", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--agent", "one-word", "--dry-run")
			}},
		{contract.SetupAgentIsOperator,
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--agent", "ana", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				return runCLI(t, f.project, "setup", "claude-code", "--agent", "anna", "--dry-run")
			}},
		{contract.SetupOtherWorkbench,
			func(t *testing.T, f *setupFixture) invocation {
				second := secondWorkbench(t, f)
				accepted(t, "setup for the first workbench", runCLI(t, f.project, "--workbench", f.workbench, "setup", "claude-code"))
				return runCLI(t, f.project, "--workbench", second, "setup", "claude-code", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				second := secondWorkbench(t, f)
				return runCLI(t, f.project, "--workbench", second, "setup", "claude-code", "--dry-run")
			}},
		{contract.SetupUnreadableTarget,
			func(t *testing.T, f *setupFixture) invocation {
				f.write(t, f.project, map[string]string{".mcp.json": "not json\n"})
				return runCLI(t, f.project, "setup", "claude-code", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				f.write(t, f.project, map[string]string{".mcp.json": "{}\n"})
				return runCLI(t, f.project, "setup", "claude-code", "--dry-run")
			}},
		{contract.SetupConflict,
			func(t *testing.T, f *setupFixture) invocation {
				f.write(t, f.project, map[string]string{".mcp.json": `{"mcpServers": {"dinah": {"command": "elsewhere"}}}`})
				return runCLI(t, f.project, "setup", "claude-code", "--dry-run")
			},
			func(t *testing.T, f *setupFixture) invocation {
				f.write(t, f.project, map[string]string{".mcp.json": `{"mcpServers": {"other": {"command": "elsewhere"}}}`})
				return runCLI(t, f.project, "setup", "claude-code", "--dry-run")
			}},
	}
}

// secondWorkbench adds a workbench beside the fixture's own in the same
// container and returns its directory.
func secondWorkbench(t *testing.T, f *setupFixture) string {
	t.Helper()
	before := map[string]bool{}
	entries, _ := os.ReadDir(filepath.Join(f.project, ".dinah"))
	for _, entry := range entries {
		before[entry.Name()] = true
	}
	if got := runCLI(t, f.project, "init", "--slug", "sy", "--operator", "ana", "--here"); got.code != 0 {
		t.Fatalf("second init: %d %s", got.code, got.errw)
	}
	entries, _ = os.ReadDir(filepath.Join(f.project, ".dinah"))
	for _, entry := range entries {
		if entry.IsDir() && !before[entry.Name()] {
			return filepath.Join(f.project, ".dinah", entry.Name())
		}
	}
	t.Fatal("the second init added no workbench directory")
	return ""
}

// TestEverySetupCheckRefusesAloneBesideAnAcceptingCase drives each of the
// eighteen rows of setup's precondition list with an invocation that fails
// that row alone, and beside it one that passes, each in a fixture of its own.
func TestEverySetupCheckRefusesAloneBesideAnAcceptingCase(t *testing.T) {
	rows := setupCheckRows()
	if len(rows) != 18 {
		t.Fatalf("the table drives %d rows, and setup's list has eighteen", len(rows))
	}
	checks := verb.Checks("setup")
	if len(checks) != len(rows) {
		t.Fatalf("setup declares %d checks and the table drives %d", len(checks), len(rows))
	}
	for i, row := range rows {
		if checks[i].Refusal != row.refusal {
			t.Errorf("row %d: setup declares %s and the specification's row is %s", i+1, checks[i].Refusal, row.refusal)
		}
		refusing := newSetupFixture(t)
		refusedWith(t, "row "+itoaRow(i)+" refusing", row.refuse(t, refusing), row.refusal)
		accepting := newSetupFixture(t)
		accepted(t, "row "+itoaRow(i)+" accepting", row.accept(t, accepting))
	}
}

// itoaRow is a row's one-based number for a message.
func itoaRow(i int) string {
	return string(rune('0'+(i+1)/10)) + string(rune('0'+(i+1)%10))
}

// TestSetupChecksTheOperatorAndTheConfiguredActor refuses an agent named for
// the workbench's operator and one named for the configured actor, each
// saying which of the two it matched.
func TestSetupChecksTheOperatorAndTheConfiguredActor(t *testing.T) {
	f := newSetupFixture(t)
	operator := runCLI(t, f.project, "setup", "claude-code", "--agent", "ana", "--dry-run")
	refusedWith(t, "the operator", operator, contract.SetupAgentIsOperator)
	if !strings.Contains(operator.errw, "ana ("+msg.For("en").T("setup.operator.workbench")+")") {
		t.Errorf("the refusal does not say the name is the workbench's operator: %s", operator.errw)
	}
	if got := runCLI(t, f.project, "config", "set", "actor", "bob"); got.code != 0 {
		t.Fatalf("config set actor: %s", got.errw)
	}
	configured := runCLI(t, f.project, "setup", "claude-code", "--agent", "bob", "--dry-run")
	refusedWith(t, "the configured actor", configured, contract.SetupAgentIsOperator)
	if !strings.Contains(configured.errw, "bob ("+msg.For("en").T("setup.operator.config")+")") {
		t.Errorf("the refusal does not say the name is the configured actor: %s", configured.errw)
	}
	accepted(t, "another name", runCLI(t, f.project, "setup", "claude-code", "--agent", "carol", "--dry-run"))
}

// TestSetupChecksTheProviderItWouldWrite holds the provider setup would write
// to being one word, from the command line, against the one shipped recipe
// that declares no provider of its own. Two accepting invocations stand
// beside the refusals, since a check that refuses everything is no check.
//
// A value that is one space is refused here too, and this test cannot read
// that refusal. The detail setup builds is the flag, a space, and the value,
// so a value of one space makes a run of two spaces on the refusal's first
// line, and runCLI's own shape check reads such a run as an empty fill and
// fails the test. The refusal is correct and its message is not, and the
// message belongs to every flag that row checks rather than to this card.
// TestTheProviderFlagIsStillHeldToOneWord in internal/setup drives the single
// space against the same code and is where that case is held.
func TestSetupChecksTheProviderItWouldWrite(t *testing.T) {
	f := newSetupFixture(t)
	refusedWith(t, "a provider of two words", runCLI(t, f.project, "setup", "devin", "--provider", "two words", "--dry-run"), contract.Malformed)
	refusedWith(t, "a provider carrying a no-break space", runCLI(t, f.project, "setup", "devin", "--provider", "a b", "--dry-run"), contract.Malformed)
	accepted(t, "a provider of one word", runCLI(t, f.project, "setup", "devin", "--provider", "acme", "--dry-run"))
	accepted(t, "no provider at all", runCLI(t, f.project, "setup", "devin", "--dry-run"))
}

// TestSetupHoldsItsCheckOrderWhereTwoRowsFail runs setup where two rows fail
// at once and holds the earlier row to answering: the operator's name beside
// a conflicting entry is refused for the name, and an unreadable file beside
// a conflict is refused for the file.
func TestSetupHoldsItsCheckOrderWhereTwoRowsFail(t *testing.T) {
	f := newSetupFixture(t)
	f.write(t, f.project, map[string]string{".mcp.json": `{"mcpServers": {"dinah": {"command": "elsewhere"}}}`})
	refusedWith(t, "rows 15 and 18", runCLI(t, f.project, "setup", "claude-code", "--agent", "ana", "--dry-run"), contract.SetupAgentIsOperator)
	f.write(t, f.project, map[string]string{".claude/settings.local.json": "[1, 2]\n"})
	refusedWith(t, "rows 17 and 18", runCLI(t, f.project, "setup", "claude-code", "--dry-run"), contract.SetupUnreadableTarget)
}

// TestSetupHelpPrintsItsRowsInOrder holds dinah help setup to printing the
// eighteen refusals in the order the specification lists them.
func TestSetupHelpPrintsItsRowsInOrder(t *testing.T) {
	f := newSetupFixture(t)
	page := runCLI(t, f.project, "help", "setup")
	accepted(t, "help setup", page)
	at := 0
	for i, row := range setupCheckRows() {
		found := strings.Index(page.out[at:], " "+row.refusal+"\n")
		if found < 0 {
			t.Fatalf("row %d, %s, is not on the page after the rows before it:\n%s", i+1, row.refusal, page.out)
		}
		at += found + len(row.refusal)
	}
	if !strings.Contains(page.out, "dinah guide setup-recipes") {
		t.Errorf("the page does not point at the setup-recipes guide:\n%s", page.out)
	}
}

// TestSetupListShowsTheShippedRecipes holds a listing with no user or
// project recipe to exactly the shipped recipes, each used.
func TestSetupListShowsTheShippedRecipes(t *testing.T) {
	f := newSetupFixture(t)
	got := runCLI(t, f.project, "--json", "setup", "--list")
	accepted(t, "setup --list", got)
	var listing struct {
		Recipes []setup.Listing `json:"recipes"`
	}
	if err := json.Unmarshal([]byte(got.out), &listing); err != nil {
		t.Fatalf("the listing is not JSON: %v\n%s", err, got.out)
	}
	want := []string{"claude-code", "codex", "devin"}
	if len(listing.Recipes) != len(want) {
		t.Fatalf("the listing carries %d rows: %+v", len(listing.Recipes), listing.Recipes)
	}
	for i, row := range listing.Recipes {
		if row.Name != want[i] || row.Source != setup.SourceShipped || !row.Used || row.Path != "" || row.Malformed != "" {
			t.Errorf("row %d reads %+v", i+1, row)
		}
	}
	human := runCLI(t, f.project, "setup", "--list")
	if !strings.Contains(human.out, "claude-code") || !strings.Contains(human.out, "Claude Code") {
		t.Errorf("the listing does not name the shipped recipe:\n%s", human.out)
	}
	refusedWith(t, "a listing with a flag of its own", runCLI(t, f.project, "setup", "--list", "--dry-run"), contract.Usage)
	beside := runCLI(t, f.project, "setup", "codex", "--list")
	refusedWith(t, "a listing beside a harness", beside, contract.Usage)
	if !strings.Contains(beside.errw, msg.For("en").T("refusal.dinah.usage.setup", "detail", "--list")) {
		t.Errorf("setup's usage refusal says the flag was not understood rather than that it does not fit:\n%s", beside.errw)
	}
}

// TestSetupListsAConflictApartFromItsNextStep holds the conflict refusal to
// printing its next step on a line of its own, so the step does not read as
// part of the last conflicting location.
func TestSetupListsAConflictApartFromItsNextStep(t *testing.T) {
	f := newSetupFixture(t)
	f.write(t, f.project, map[string]string{".mcp.json": `{"mcpServers": {"dinah": {"command": "elsewhere"}}}`})
	got := runCLI(t, f.project, "setup", "claude-code", "--dry-run")
	refusedWith(t, "a conflict", got, contract.SetupConflict)
	next := msg.For("en").T("refusal.dinah.setup-conflict.next")
	if !strings.Contains(got.errw, "\n  .mcp.json /mcpServers/dinah\n"+next+"\n") {
		t.Errorf("the conflict's location and its next step share a line:\n%q", got.errw)
	}
}

// TestSetupListMarksABrokenOverride lists a user recipe that does not read
// beside the shipped one of the same name, used and marked unusable.
func TestSetupListMarksABrokenOverride(t *testing.T) {
	f := newSetupFixture(t)
	dir := setupRecipe(t, filepath.Join(f.home, ".dinah", "recipes"), "codex")
	f.write(t, dir, map[string]string{"prompt.md": "{{nothing}}\n"})
	got := runCLI(t, f.project, "setup", "--list")
	accepted(t, "setup --list", got)
	if !strings.Contains(got.out, msg.For("en").T("setup.list.malformed")) || !strings.Contains(got.out, dir) {
		t.Errorf("the listing does not mark the broken recipe:\n%s", got.out)
	}
}

// TestAProjectRecipeNeedsTrustEvenForADryRunOrARemoval refuses a project's
// recipe without --trust-project-recipe on an apply, a dry run and a removal,
// and with it prints the recipe's text under the heading naming its
// directory.
func TestAProjectRecipeNeedsTrustEvenForADryRunOrARemoval(t *testing.T) {
	f := newSetupFixture(t)
	dir := setupRecipe(t, filepath.Join(f.project, ".dinah", "recipes"), "local")
	for _, extra := range [][]string{nil, {"--dry-run"}, {"--remove"}} {
		argv := append([]string{"setup", "local"}, extra...)
		refusedWith(t, strings.Join(argv, " "), runCLI(t, f.project, argv...), contract.UntrustedRecipe)
	}
	got := runCLI(t, f.project, "setup", "local", "--trust-project-recipe")
	accepted(t, "a trusted apply", got)
	// The head names the recipe by the directory discovery resolved from
	// the working directory, so the expected side reproduces that sequence
	// rather than joining onto the fixture's own spelling of it.
	resolved := filepath.Join(resolvedDir(t, f.project), ".dinah", "recipes", filepath.Base(dir))
	heading := msg.For("en").T("setup.prompt.heading.project", "detail", resolved)
	if !strings.Contains(got.out, heading) {
		t.Errorf("the prompt is not printed under the project heading %q:\n%s", heading, got.out)
	}
}

// TestSetupPrintsItsReportAndItsPrompt applies and removes the claude-code
// recipe at the terminal and holds the report to its heading, its table, and
// the recipe's prompt, and a second removal to saying it has nothing on
// record.
func TestSetupPrintsItsReportAndItsPrompt(t *testing.T) {
	f := newSetupFixture(t)
	en := msg.For("en")
	got := runCLI(t, f.project, "setup", "claude-code", "--model", "m1")
	accepted(t, "apply", got)
	for _, want := range []string{
		en.T("setup.prompt.heading"),
		en.T("setup.change.create"),
		"claude mcp add --env DINAH_ACTOR=claude",
	} {
		if !strings.Contains(got.out, want) {
			t.Errorf("the apply does not print %q:\n%s", want, got.out)
		}
	}
	removed := runCLI(t, f.project, "setup", "claude-code", "--remove")
	accepted(t, "remove", removed)
	if !strings.Contains(removed.out, "claude mcp remove dinah --scope user") {
		t.Errorf("the removal does not print remove.md:\n%s", removed.out)
	}
	again := runCLI(t, f.project, "setup", "claude-code", "--remove")
	accepted(t, "a second remove", again)
	if !strings.Contains(again.out, en.T("setup.remove.nothing")) {
		t.Errorf("a second removal does not say it has nothing on record:\n%s", again.out)
	}
	if !strings.Contains(en.T("setup.remove.nothing"), "no record") {
		t.Errorf("the sentence claims nothing is there rather than that setup has no record: %q", en.T("setup.remove.nothing"))
	}
}

// TestSetupDryRunPrintsBeforeAndAfter holds a dry run to printing its heading
// line, and a before and after for every location it would change.
func TestSetupDryRunPrintsBeforeAndAfter(t *testing.T) {
	f := newSetupFixture(t)
	got := runCLI(t, f.project, "setup", "claude-code", "--dry-run")
	accepted(t, "dry run", got)
	if !strings.HasPrefix(got.out, msg.For("en").T("setup.dry-run")+"\n") {
		t.Errorf("the dry run does not open with its heading:\n%s", got.out)
	}
	for _, want := range []string{
		"--- .mcp.json /mcpServers/dinah (before)\n(absent)\n+++ .mcp.json /mcpServers/dinah (after)\n{",
		"--- .claude/rules/dinah.md (before)\n(absent)\n",
	} {
		if !strings.Contains(got.out, want) {
			t.Errorf("the dry run does not print %q:\n%s", want, got.out)
		}
	}
	if _, err := os.Stat(filepath.Join(f.project, ".mcp.json")); err == nil {
		t.Error("the dry run wrote .mcp.json")
	}
}

// TestSetupRunStepsAtTheTerminal holds the three things a run step prints: a
// dry run's would-run line and warning, an apply's warning once the program
// has run, and under --json a failure's completed changes and failing step.
func TestSetupRunStepsAtTheTerminal(t *testing.T) {
	f := newSetupFixture(t)
	en := msg.For("en")
	log := filepath.Join(f.root, "program.log")
	t.Setenv(testenv.EditorRecordVar, log)
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := setupRecipe(t, f.root, "runs", runStep(t, "register", program, "first arg"))
	dry := runCLI(t, f.project, "setup", "--recipe", dir, "--dry-run")
	accepted(t, "dry run", dry)
	if !strings.Contains(dry.out, "=== register would run: ") {
		t.Errorf("the dry run does not print the would-run line:\n%s", dry.out)
	}
	if !strings.Contains(dry.out, en.T("setup.run.unverified.dry-run", "count", "1")) {
		t.Errorf("the dry run does not warn about the program:\n%s", dry.out)
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("the dry run started the program")
	}
	applied := runCLI(t, f.project, "setup", "--recipe", dir, "--allow-run")
	accepted(t, "apply", applied)
	if !strings.Contains(applied.out, en.T("setup.run.unverified", "count", "1")) || !strings.Contains(applied.out, en.T("setup.change.ran")) {
		t.Errorf("the apply does not warn about the program it ran:\n%s", applied.out)
	}
	recorded, err := os.ReadFile(log)
	if err != nil || strings.TrimSpace(string(recorded)) != "first arg" {
		t.Errorf("the program recorded %q, %v", recorded, err)
	}

	g := newSetupFixture(t)
	failing := setupRecipe(t, g.root, "fails", runStep(t, "missing", filepath.Join(g.root, "no-such-program")))
	failed := runCLI(t, g.project, "--json", "setup", "--recipe", failing, "--allow-run")
	refusedWith(t, "a program that cannot be found", failed, contract.SetupStepFailed)
	var answer struct {
		Refusal    string         `json:"refusal"`
		FailedStep string         `json:"failed_step"`
		Changes    []setup.Change `json:"changes"`
	}
	if err := json.Unmarshal([]byte(failed.out), &answer); err != nil {
		t.Fatalf("the failure is not JSON: %v\n%s", err, failed.out)
	}
	if answer.Refusal != contract.SetupStepFailed || answer.FailedStep != "missing" {
		t.Errorf("the failure reads %+v", answer)
	}
	if len(answer.Changes) != 1 || answer.Changes[0].Step != "rules" || answer.Changes[0].Change != setup.ChangeCreate {
		t.Errorf("the failure's changes are %+v, and only the rules step completed", answer.Changes)
	}
	human := runCLI(t, g.project, "setup", "--recipe", failing, "--allow-run")
	refusedWith(t, "the failure at the terminal", human, contract.SetupStepFailed)
	if !strings.Contains(human.out, "rules.md") {
		t.Errorf("the terminal failure does not print what completed:\n%s", human.out)
	}
}

// expectSetupList is the listing with no user or project recipe, which is the
// shipped recipes, each used.
func expectSetupList(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	yes := msg.For(tag).T("word.yes")
	var rows [][]sweptCell
	for _, row := range setup.List("", "") {
		rows = append(rows, sweptTexts(row.Name, row.Title, row.Source, yes, "-"))
	}
	return sweptExpectation{rows: rows, source: "the recipes the binary ships"}
}

// expectSetupChanges is a dry run of the claude-code recipe into a project
// holding none of its files, naming no model, so every location is created.
func expectSetupChanges(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	created := msg.For(tag).T("setup.change.create")
	rows := [][]sweptCell{
		sweptTexts("mcp-server", ".mcp.json", "/mcpServers/dinah", created),
		sweptTexts("environment-project", ".claude/settings.local.json", "/env/DINAH_ACTOR", created),
		sweptTexts("environment-project", ".claude/settings.local.json", "/env/DINAH_HARNESS", created),
		sweptTexts("environment-project", ".claude/settings.local.json", "/env/DINAH_PROVIDER", created),
		sweptTexts("instructions-project", ".claude/rules/dinah.md", "-", created),
	}
	return sweptExpectation{rows: rows, source: "the claude-code recipe's project steps"}
}

// setupHeadingOf is the heading a dry run of the claude-code recipe draws its
// table under in the healthy workbench, which names the directory the head
// resolves for it.
func setupHeadingOf(tag string, w *sweptWorkbenches) string {
	previous, _ := os.Getwd()
	base := w.healthy
	if os.Chdir(w.healthy) == nil {
		base, _ = os.Getwd()
		os.Chdir(previous)
	}
	return msg.For(tag).T("setup.heading", "recipe", "claude-code", "source", setup.SourceShipped, "scope", setup.ScopeProject, "base", base)
}

// TestSetupReportsAFileItEmptiedAndRemoved applies the claude-code recipe and
// then takes it back, and holds the row dinah-577 adds to its shape at the
// terminal and in the machine form. The recipe creates .mcp.json and owns the
// one member in it, so the removal empties a file setup created and removes
// it. The row carries no step and no key, its change is the remove token every
// other take-back already uses, and the dry run prints its leftover content as
// the before and the absent marker as the after.
func TestSetupReportsAFileItEmptiedAndRemoved(t *testing.T) {
	f := newSetupFixture(t)
	accepted(t, "apply", runCLI(t, f.project, "setup", "claude-code", "--model", "m1"))
	if _, err := os.Stat(filepath.Join(f.project, ".mcp.json")); err != nil {
		t.Fatalf("the apply wrote no .mcp.json, so this test proves nothing: %v", err)
	}

	planned := runCLI(t, f.project, "setup", "claude-code", "--remove", "--dry-run")
	accepted(t, "a dry run of the removal", planned)
	for _, want := range []string{
		"--- .mcp.json (before)\n{",
		"+++ .mcp.json (after)\n(absent)\n",
	} {
		if !strings.Contains(planned.out, want) {
			t.Errorf("the dry run of the removal does not print %q:\n%s", want, planned.out)
		}
	}

	t.Setenv("DINAH_FORMAT", "")
	machine := runCLI(t, f.project, "--json", "setup", "claude-code", "--remove")
	accepted(t, "the removal in the machine form", machine)
	var report struct {
		Changes []setup.Change `json:"changes"`
	}
	if err := json.Unmarshal([]byte(machine.out), &report); err != nil {
		t.Fatalf("the machine form does not parse: %v\n%s", err, machine.out)
	}
	var emptied []setup.Change
	for _, c := range report.Changes {
		if c.Kind == setup.KindEmptiedFile {
			emptied = append(emptied, c)
		}
	}
	if len(emptied) != 2 {
		t.Fatalf("the machine form carries %d emptied-file rows and the removal empties two files: %+v", len(emptied), report.Changes)
	}
	if emptied[0].File != ".mcp.json" || emptied[1].File != ".claude/settings.local.json" {
		t.Errorf("the emptied-file rows name %q and %q, in the order the run first reached those files", emptied[0].File, emptied[1].File)
	}
	for _, row := range emptied {
		if row.Change != setup.ChangeRemove || row.Step != "" || row.Key != "" || row.After != "" {
			t.Errorf("the emptied-file row reads %+v", row)
		}
		if row.Before == "" {
			t.Errorf("the emptied-file row for %s carries no before, and the take-backs left a root object in it", row.File)
		}
	}
	for _, name := range []string{".mcp.json", ".claude/settings.local.json"} {
		if _, err := os.Stat(filepath.Join(f.project, filepath.FromSlash(name))); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("the removal left %s behind: %v", name, err)
		}
	}
}
