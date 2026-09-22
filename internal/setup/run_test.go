package setup

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// helperStep is a run step's JSON that starts this test binary as the helper
// program, with arguments.
func helperStep(t *testing.T, id string, args ...string) string {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
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

// runRecipe is the trial recipe's steps with run steps placed after its
// json-merge step, so a failing program leaves one data step applied and two
// after it unapplied.
func runRecipe(t *testing.T, f *fixture, runSteps ...string) string {
	t.Helper()
	steps := `{
  "steps": [
    {"id": "settings", "kind": "json-merge", "scope": "project", "path": "conf/settings.json",
     "pointer": "/env", "value": {"AGENT": "{{agent}}"}},
    ` + strings.Join(runSteps, ",\n    ") + `,
    {"id": "rules", "kind": "write-file", "scope": "project", "path": "conf/rules.md",
     "template": "rules.md"}
  ]
}
`
	return trialDir(t, f, map[string]string{"steps.json": steps})
}

// helperReceived is what the helper program was handed, and whether it ran.
func helperReceived(t *testing.T, out string) ([]string, bool) {
	t.Helper()
	data, err := os.ReadFile(out)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatalf("the helper wrote %q: %v", data, err)
	}
	return args, true
}

// TestARunStepNeedsAllowRunAndADryRunNeverStartsIt refuses a recipe with a
// run step unless --allow-run is given, applies it when it is, and holds a
// dry run to reporting the step as would-run without starting the program.
func TestARunStepNeedsAllowRunAndADryRunNeverStartsIt(t *testing.T) {
	f := newFixture(t)
	out := filepath.Join(f.root, "helper.json")
	t.Setenv(helperOut, out)
	opts := f.trialOptions(runRecipe(t, f, helperStep(t, "register", "one")))
	_, err := Run(opts)
	if refusalName(err) != contract.SetupRunNotAllowed || !strings.Contains(err.Error(), "register: ") {
		t.Fatalf("wanted %s listing the step, got %v", contract.SetupRunNotAllowed, err)
	}
	if _, ran := helperReceived(t, out); ran {
		t.Fatal("the refused run started the program")
	}
	if files := tree(t, f.project); len(files) != 0 {
		t.Errorf("the refused run wrote %v", files)
	}

	dry := opts
	dry.DryRun = true
	report := mustRun(t, dry)
	if _, ran := helperReceived(t, out); ran {
		t.Fatal("the dry run started the program")
	}
	if report.Count(ChangeWouldRun) != 1 {
		t.Errorf("the dry run reported %v", changesOf(report))
	}

	opts.AllowRun = true
	applied := mustRun(t, opts)
	if applied.Count(ChangeRan) != 1 {
		t.Errorf("the apply reported %v", changesOf(applied))
	}
	if args, ran := helperReceived(t, out); !ran || strings.Join(args, "|") != "one" {
		t.Errorf("the program received %v, ran %v", args, ran)
	}
}

// TestARunStepsArgumentsReachTheProgramWhole hands the program an argument
// holding a semicolon, a double ampersand, a space and a quote, and an empty
// one, and holds each to arriving as one element, since no shell stands
// between setup and the program. An argument that is nothing but an empty
// placeholder is left out.
func TestARunStepsArgumentsReachTheProgramWhole(t *testing.T) {
	f := newFixture(t)
	out := filepath.Join(f.root, "helper.json")
	t.Setenv(helperOut, out)
	awkward := `a; b && "c" d`
	opts := f.trialOptions(runRecipe(t, f, helperStep(t, "register", awkward, "{{model}}", "", "{{agent}}")))
	opts.AllowRun = true
	mustRun(t, opts)
	args, ran := helperReceived(t, out)
	if !ran {
		t.Fatal("the program did not run")
	}
	want := []string{awkward, "", "helper"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("the program received %q, want %q", args, want)
	}
	if got := CommandLine("p", []string{awkward, ""}); got != `p "a; b && \"c\" d" ""` {
		t.Errorf("the display form is %s", got)
	}
}

// TestAFailingProgramStopsTheRun starts a program that exits 3, and holds the
// run to stopping there with the data step before it applied and recorded and
// the data step after it not applied. A rerun once the program succeeds
// reports the recorded step unchanged and runs the program again, and a
// removal instead takes back exactly the recorded step.
func TestAFailingProgramStopsTheRun(t *testing.T) {
	f := newFixture(t)
	out := filepath.Join(f.root, "helper.json")
	t.Setenv(helperOut, out)
	t.Setenv(helperExit, "3")
	opts := f.trialOptions(runRecipe(t, f, helperStep(t, "register", "one")))
	opts.AllowRun = true
	_, err := Run(opts)
	var failed *StepFailed
	if !errors.As(err, &failed) {
		t.Fatalf("wanted a failed step, got %v", err)
	}
	if failed.Refusal.Name != contract.SetupStepFailed || failed.Step != "register" || !strings.HasSuffix(failed.Refusal.Detail, " exited 3") || !strings.HasPrefix(failed.Refusal.Detail, "register: ") {
		t.Errorf("the failure reads %s %q", failed.Step, failed.Refusal.Detail)
	}
	if len(failed.Report.Changes) != 1 || failed.Report.Changes[0].Step != "settings" {
		t.Errorf("the failure's report holds %v, and only the settings step completed", changesOf(failed.Report))
	}
	if _, err := os.Stat(filepath.Join(f.project, "conf", "rules.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("the step after the failing program was applied")
	}
	ledger := readFile(t, f.ledgerPath())
	if !strings.Contains(ledger, `"/env/AGENT"`) || strings.Contains(ledger, "rules.md") {
		t.Errorf("the ledger does not hold exactly the completed step:\n%s", ledger)
	}
	if err := os.Remove(out); err != nil {
		t.Fatal(err)
	}

	t.Setenv(helperExit, "0")
	rerun := mustRun(t, opts)
	rows := changesOf(rerun)
	if len(rows) != 3 || rerun.Changes[0].Change != ChangeUnchanged || rerun.Changes[1].Change != ChangeRan || rerun.Changes[2].Change != ChangeCreate {
		t.Errorf("the rerun reported %v", rows)
	}
	if _, ran := helperReceived(t, out); !ran {
		t.Error("the rerun did not run the program again")
	}

	g := newFixture(t)
	t.Setenv(helperExit, "3")
	stopped := g.trialOptions(runRecipe(t, g, helperStep(t, "register")))
	stopped.AllowRun = true
	if _, err := Run(stopped); !errors.As(err, &failed) {
		t.Fatalf("wanted a failed step, got %v", err)
	}
	removal := g.trialOptions(stopped.RecipeDir)
	removal.Remove = true
	removed := mustRun(t, removal)
	if len(removed.Changes) != 1 || removed.Changes[0].Key != "/env/AGENT" || removed.Changes[0].Change != ChangeRemove {
		t.Errorf("the removal took back %v", changesOf(removed))
	}
	if !strings.Contains(removed.Prompt, "Undo the rest in") {
		t.Errorf("the removal did not carry remove.md: %q", removed.Prompt)
	}
}

// TestAProgramThatCannotBeFoundStopsTheRun names a program no directory
// carries, and holds the run to stopping with the step named.
func TestAProgramThatCannotBeFoundStopsTheRun(t *testing.T) {
	f := newFixture(t)
	missing := filepath.Join(f.root, "no-such-program")
	step := `{"id": "register", "kind": "run", "scope": "project", "program": ` + jsonString(missing) + `}`
	opts := f.trialOptions(runRecipe(t, f, step))
	opts.AllowRun = true
	_, err := Run(opts)
	var failed *StepFailed
	if !errors.As(err, &failed) || !strings.HasSuffix(failed.Refusal.Detail, " not found") {
		t.Fatalf("wanted a not-found failure, got %v", err)
	}
}

// TestARunStepLeavesNoLedgerEntry applies a recipe with a run step and holds
// the ledger to recording the data steps alone, and a removal to taking back
// only those.
func TestARunStepLeavesNoLedgerEntry(t *testing.T) {
	f := newFixture(t)
	out := filepath.Join(f.root, "helper.json")
	t.Setenv(helperOut, out)
	opts := f.trialOptions(runRecipe(t, f, helperStep(t, "register", "one")))
	opts.AllowRun = true
	mustRun(t, opts)
	l, err := readLedger(f.userBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) != 2 {
		t.Errorf("the ledger holds %d entries, and the recipe has two data steps", len(l.Entries))
	}
	for _, e := range l.Entries {
		if e.Kind != entryJSONMember && e.Kind != entryFile {
			t.Errorf("the ledger holds an entry of kind %s", e.Kind)
		}
	}
	if err := os.Remove(out); err != nil {
		t.Fatal(err)
	}
	removal := f.trialOptions(opts.RecipeDir)
	removal.Remove = true
	removed := mustRun(t, removal)
	if len(removed.Changes) != 2 {
		t.Errorf("the removal reported %v", changesOf(removed))
	}
	if _, ran := helperReceived(t, out); ran {
		t.Error("the removal ran the program")
	}
}

// TestARunStepStaysOutOfProjectRecipes refuses a project container's recipe
// declaring a run step, with and without --trust-project-recipe.
func TestARunStepStaysOutOfProjectRecipes(t *testing.T) {
	f := newFixture(t)
	steps := `{"steps": [{"id": "register", "kind": "run", "scope": "project", "program": "some-program"}]}`
	files := map[string]string{}
	for name, text := range trialRecipe {
		files[name] = text
	}
	files["recipe.json"] = strings.Replace(files["recipe.json"], `"scopes": ["project", "user"]`, `"scopes": ["project"]`, 1)
	files["steps.json"] = steps
	writeFiles(t, filepath.Join(f.project, ".dinah", "recipes", "trial"), files)
	for _, trust := range []bool{false, true} {
		opts := f.options("trial")
		opts.TrustProjectRecipe = trust
		opts.AllowRun = true
		_, err := Run(opts)
		if refusalName(err) != contract.MalformedRecipe || !strings.Contains(err.Error(), "run step") {
			t.Errorf("trust %v: wanted %s, got %v", trust, contract.MalformedRecipe, err)
		}
	}
}

// TestAPathLeavingTheBaseIsRefused points a step's path through a symbolic
// link, or a junction on Windows, at a directory outside the base, and holds
// setup to refusing it before any write, beside a link that stays inside.
func TestAPathLeavingTheBaseIsRefused(t *testing.T) {
	f := newFixture(t)
	outside := filepath.Join(f.root, "outside")
	inside := filepath.Join(f.project, "inside")
	for _, dir := range []string{outside, inside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if !link(t, outside, filepath.Join(f.project, "conf")) {
		t.Skip("this platform refused to create a symbolic link or a junction, so the escape cannot be built here")
	}
	opts := f.trialOptions(trialDir(t, f, nil))
	_, err := Run(opts)
	if refusalName(err) != contract.SetupUnreadableTarget || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("wanted %s, got %v", contract.SetupUnreadableTarget, err)
	}
	if files := tree(t, outside); len(files) != 0 {
		t.Errorf("the refused run wrote %v outside the base", files)
	}
	if _, err := os.Stat(filepath.Join(f.project, "NOTES.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Error("the refused run wrote a file inside the base")
	}

	g := newFixture(t)
	inner := filepath.Join(g.project, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if !link(t, inner, filepath.Join(g.project, "conf")) {
		t.Skip("this platform refused to create the second link")
	}
	mustRun(t, g.trialOptions(trialDir(t, g, nil)))
	if _, err := os.Stat(filepath.Join(inner, "rules.md")); err != nil {
		t.Errorf("a link that stays inside the base was not followed: %v", err)
	}
}

// link makes a directory link, a symbolic link where the platform allows one
// and a junction on Windows where it does not, and reports whether either
// was made.
func link(t *testing.T, target, name string) bool {
	t.Helper()
	if err := os.Symlink(target, name); err == nil {
		return true
	}
	if runtime.GOOS != "windows" {
		return false
	}
	cmd := exec.Command("cmd", "/c", "mklink", "/J", name, target)
	return cmd.Run() == nil
}

// TestARecipeIsValidatedBeforeAnythingIsRead refuses a recipe declaring what
// format 1 does not define, each as malformed and naming the file.
func TestARecipeIsValidatedBeforeAnythingIsRead(t *testing.T) {
	cases := map[string]map[string]string{
		"recipe.json: declares format 2":    {"recipe.json": strings.Replace(trialRecipe["recipe.json"], `"format": 1`, `"format": 2`, 1)},
		"recipe.json: does not parse":       {"recipe.json": strings.Replace(trialRecipe["recipe.json"], `"title"`, `"titel"`, 1)},
		"recipe.json: opens with a byte":    {"recipe.json": "\xef\xbb\xbf" + trialRecipe["recipe.json"]},
		"steps.json: step settings carries": {"steps.json": strings.Replace(trialRecipe["steps.json"], `"pointer": "/env"`, `"pointer": "/env", "template": "x"`, 1)},
		"steps.json: step notes writes":     {"steps.json": strings.Replace(trialRecipe["steps.json"], `"path": "NOTES.md"`, `"path": "conf/settings.json"`, 1)},
		"steps.json: step settings writes":  {"steps.json": strings.Replace(trialRecipe["steps.json"], `"conf/settings.json"`, `"../settings.json"`, 1)},
		"prompt.md: cannot be read":         {"prompt.md": ""},
	}
	for want, overrides := range cases {
		f := newFixture(t)
		dir := trialDir(t, f, overrides)
		if overrides["prompt.md"] == "" && strings.HasPrefix(want, "prompt.md") {
			if err := os.Remove(filepath.Join(dir, "prompt.md")); err != nil {
				t.Fatal(err)
			}
		}
		_, err := Run(f.trialOptions(dir))
		if refusalName(err) != contract.MalformedRecipe {
			t.Errorf("%s: wanted %s, got %v", want, contract.MalformedRecipe, err)
			continue
		}
		var refusal *contract.Refusal
		errors.As(err, &refusal)
		if !strings.HasPrefix(refusal.Detail, strings.Join(strings.Fields(want)[:2], " ")) {
			t.Errorf("%s: the detail reads %q", want, refusal.Detail)
		}
	}
}
