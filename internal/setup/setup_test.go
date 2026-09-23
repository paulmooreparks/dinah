package setup

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// helperOut names the environment variable that turns this test binary into
// the program a run step starts. The child writes its arguments, as a JSON
// array, to the file the variable names, and exits with the code
// helperExit names, zero when that is unset. It is the pattern os/exec's own
// tests use, and it runs before any test so the child never runs one.
const (
	helperOut  = "DINAH_SETUP_TEST_HELPER_OUT"
	helperExit = "DINAH_SETUP_TEST_HELPER_EXIT"
)

// updateGoldens rewrites the golden files from what setup produces, then
// fails, so no run in that mode can pass.
var updateGoldens = flag.Bool("update-setup-goldens", false, "rewrite internal/setup/testdata/golden from the recipes, then fail")

// TestMain starts the helper program when the environment asks for it, and
// the suite otherwise.
func TestMain(m *testing.M) {
	if out := os.Getenv(helperOut); out != "" {
		data, _ := json.Marshal(os.Args[1:])
		if err := os.WriteFile(out, data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		code, _ := strconv.Atoi(os.Getenv(helperExit))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// fixture is one throwaway machine: a home directory with a user base inside
// it, and a project holding a workbench in its .dinah container. Nothing in it
// is opened as a workbench, since setup reads only the directory's path.
type fixture struct {
	root, home, userBase, project, workbench string
}

// fixedWorkbench is the workbench identifier every fixture uses, so goldens
// are stable.
const fixedWorkbench = "0123456789abcdef0123456789abcdef"

// newFixture builds a fixture under the test's own temporary directory.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		root:    root,
		home:    filepath.Join(root, "home"),
		project: filepath.Join(root, "proj"),
	}
	f.userBase = filepath.Join(f.home, ".dinah")
	f.workbench = filepath.Join(f.project, ".dinah", fixedWorkbench)
	for _, dir := range []string{f.userBase, f.workbench} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// options is a run of a named recipe at project scope in the fixture.
func (f *fixture) options(harness string) Options {
	return Options{
		Harness:        harness,
		Home:           f.home,
		UserBase:       f.userBase,
		Workbench:      f.workbench,
		WorkbenchTitle: "Proving ground",
		Operator:       "ana",
	}
}

// ledgerPath is the fixture's ledger file.
func (f *fixture) ledgerPath() string {
	return filepath.Join(f.userBase, LedgerName)
}

// writeFiles writes a set of files under a directory, creating directories.
func writeFiles(t *testing.T, dir string, files map[string]string) {
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

// readFile reads a file the test expects to exist.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// tree reads every file under a directory, by its path relative to it.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return files
}

// sameTree fails the test when two readings of a directory differ.
func sameTree(t *testing.T, label string, before, after map[string]string) {
	t.Helper()
	for name, text := range before {
		got, present := after[name]
		if !present {
			t.Errorf("%s: %s was removed", label, name)
			continue
		}
		if got != text {
			t.Errorf("%s: %s changed:\nbefore %q\nafter  %q", label, name, text, got)
		}
	}
	for name := range after {
		if _, present := before[name]; !present {
			t.Errorf("%s: %s was created", label, name)
		}
	}
}

// refusalName is the name of the refusal an error carries, empty for none.
func refusalName(err error) string {
	var refusal *contract.Refusal
	if errors.As(err, &refusal) {
		return refusal.Name
	}
	return ""
}

// mustRun applies setup and fails the test on a refusal.
func mustRun(t *testing.T, opts Options) *Report {
	t.Helper()
	report, err := Run(opts)
	if err != nil {
		t.Fatalf("setup refused: %v", err)
	}
	return report
}

// changesOf lists a report's rows as file key change, for a failure message
// and a comparison.
func changesOf(report *Report) []string {
	var rows []string
	for _, c := range report.Changes {
		rows = append(rows, c.File+" "+c.Key+" "+c.Change)
	}
	return rows
}

// allChanges fails unless every row of a report carries one change token.
func allChanges(t *testing.T, report *Report, token string) {
	t.Helper()
	if len(report.Changes) == 0 {
		t.Fatal("the report carries no row, so this check reads nothing")
	}
	for _, c := range report.Changes {
		if c.Change != token {
			t.Errorf("wanted every row %s, got %v", token, changesOf(report))
			return
		}
	}
}

// trialRecipe is a recipe holding one step of each data kind, which the
// tests of the engine's promises apply.
var trialRecipe = map[string]string{
	"recipe.json": `{
  "format": 1,
  "name": "trial",
  "title": "Trial",
  "provider": "acme",
  "agent": "helper",
  "tools": "all",
  "scopes": ["project", "user"],
  "documentation": ["https://example.com/trial"]
}
`,
	"steps.json": `{
  "steps": [
    {"id": "settings", "kind": "json-merge", "scope": "project", "path": "conf/settings.json",
     "pointer": "/env", "value": {"AGENT": "{{agent}}", "MODEL": "{{model}}"}},
    {"id": "notes", "kind": "marked-section", "scope": "project", "path": "NOTES.md",
     "template": "notes.md", "comment": "hash"},
    {"id": "rules", "kind": "write-file", "scope": "project", "path": "conf/rules.md",
     "template": "rules.md"}
  ]
}
`,
	"files/notes.md": "Agent {{agent}} works on {{workbench_title}}.\nIt stays in {{base}}.\n",
	"files/rules.md": "Rules for {{agent}}.\n",
	"prompt.md":      "Finish the rest in {{base}}.\n",
	"remove.md":      "Undo the rest in {{base}}.\n",
}

// trialDir writes the trial recipe, changed by any overrides, into a
// directory of its own and returns it.
func trialDir(t *testing.T, f *fixture, overrides map[string]string) string {
	t.Helper()
	dir := filepath.Join(f.root, "recipes", "trial")
	files := map[string]string{}
	for name, text := range trialRecipe {
		files[name] = text
	}
	for name, text := range overrides {
		files[name] = text
	}
	writeFiles(t, dir, files)
	return dir
}

// trialOptions is a run of the trial recipe by path.
func (f *fixture) trialOptions(dir string) Options {
	opts := f.options("")
	opts.RecipeDir = dir
	return opts
}

// TestTheShippedListMatchesTheEmbeddedRecipes holds the shipped list and the
// embedded directories equal in both directions, and every shipped recipe to
// reading cleanly with no run step.
func TestTheShippedListMatchesTheEmbeddedRecipes(t *testing.T) {
	entries, err := fs.ReadDir(shippedRecipes, "recipes")
	if err != nil {
		t.Fatal(err)
	}
	embedded := map[string]bool{}
	for _, entry := range entries {
		embedded[entry.Name()] = true
	}
	if len(embedded) == 0 {
		t.Fatal("the binary embeds no recipe, so this test reads nothing")
	}
	for _, name := range shipped {
		if !embedded[name] {
			t.Errorf("the shipped list names %s and the binary embeds no such recipe", name)
		}
	}
	for name := range embedded {
		if !contains(shipped, name) {
			t.Errorf("internal/setup/recipes/%s is embedded and the shipped list does not name it", name)
		}
	}
	shippedPlace := places("", "", false)[0]
	checked := 0
	for _, name := range shipped {
		r, err := shippedPlace.open(name)
		if err != nil {
			t.Errorf("the shipped recipe %s does not read: %v", name, err)
			continue
		}
		checked++
		for _, step := range r.Steps {
			if step.Kind == KindRun {
				t.Errorf("the shipped recipe %s declares the run step %s, and a shipped recipe never runs a program", name, step.ID)
			}
		}
	}
	if checked != len(shipped) {
		t.Errorf("read %d shipped recipes of %d", checked, len(shipped))
	}
}

// TestASecondApplyChangesNothing applies a recipe holding one step of each
// data kind twice, and holds the second run to reporting every row unchanged
// and leaving every target file and the ledger byte for byte as the first run
// left them.
func TestASecondApplyChangesNothing(t *testing.T) {
	f := newFixture(t)
	opts := f.trialOptions(trialDir(t, f, nil))
	opts.Model = "m1"
	first := mustRun(t, opts)
	if len(first.Changes) != 4 {
		t.Fatalf("wanted four rows from the first run, got %v", changesOf(first))
	}
	project := tree(t, f.project)
	ledger := readFile(t, f.ledgerPath())
	second := mustRun(t, opts)
	allChanges(t, second, ChangeUnchanged)
	if len(second.Changes) != len(first.Changes) {
		t.Errorf("the second run reported %d rows and the first %d", len(second.Changes), len(first.Changes))
	}
	sameTree(t, "second run", project, tree(t, f.project))
	if got := readFile(t, f.ledgerPath()); got != ledger {
		t.Errorf("the second run changed the ledger:\nbefore %s\nafter  %s", ledger, got)
	}
}

// TestAMergeLeavesEveryUnownedByteAlone applies the claude-code recipe to a
// .mcp.json holding another server and an unowned key, written with tabs, a
// one-line nested object and CRLF line endings. The file still parses, every
// member setup does not own keeps its compacted value and its order, and the
// only change is one insertion: everything before it and after it is the
// original's bytes.
func TestAMergeLeavesEveryUnownedByteAlone(t *testing.T) {
	f := newFixture(t)
	original := "{\r\n\t\"mcpServers\": {\r\n\t\t\"other\": {\"command\": \"x\", \"args\": [\"a\", \"b\"]}\r\n\t},\r\n\t\"unowned\": {\"keep\": true}\r\n}\r\n"
	writeFiles(t, f.project, map[string]string{".mcp.json": original})
	mustRun(t, f.options("claude-code"))
	got := readFile(t, filepath.Join(f.project, ".mcp.json"))
	prefix := 0
	for prefix < len(original) && prefix < len(got) && original[prefix] == got[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(original)-prefix && suffix < len(got)-prefix && original[len(original)-1-suffix] == got[len(got)-1-suffix] {
		suffix++
	}
	if prefix+suffix != len(original) {
		t.Errorf("setup changed original bytes rather than only inserting: %d bytes kept before the change and %d after, of %d", prefix, suffix, len(original))
	}
	inserted := got[prefix : len(got)-suffix]
	if !strings.Contains(inserted, `"dinah": {`) {
		t.Errorf("the insertion does not hold the server entry: %q", inserted)
	}
	if strings.Contains(strings.ReplaceAll(inserted, "\r\n", ""), "\n") {
		t.Errorf("the insertion into a CRLF file carries a bare LF: %q", inserted)
	}
	var parsed struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
		Unowned    json.RawMessage            `json:"unowned"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("the merged file does not parse: %v", err)
	}
	if string(compactSpan(parsed.MCPServers["other"])) != `{"command":"x","args":["a","b"]}` {
		t.Errorf("the other server changed: %s", parsed.MCPServers["other"])
	}
	if string(compactSpan(parsed.Unowned)) != `{"keep":true}` {
		t.Errorf("the unowned key changed: %s", parsed.Unowned)
	}
	root, err := parseJSONFile([]byte(got))
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, m := range root.members {
		order = append(order, m.name)
	}
	if strings.Join(order, ",") != "mcpServers,unowned" {
		t.Errorf("the top-level members are %v", order)
	}
}

// TestASectionRerunChangesOnlyTheLinesBetweenItsMarkers appends a section to
// a file with text of its own, adds text after the section, and reruns with
// another agent. Only the lines strictly between the markers change.
func TestASectionRerunChangesOnlyTheLinesBetweenItsMarkers(t *testing.T) {
	f := newFixture(t)
	dir := trialDir(t, f, nil)
	writeFiles(t, f.project, map[string]string{"NOTES.md": "# Notes\n\nOur own text.\n"})
	opts := f.trialOptions(dir)
	mustRun(t, opts)
	notes := filepath.Join(f.project, "NOTES.md")
	appended := readFile(t, notes) + "\nText after the section.\n"
	writeFiles(t, f.project, map[string]string{"NOTES.md": appended})
	opts.Agent = "second"
	report := mustRun(t, opts)
	got := readFile(t, notes)
	before := strings.Split(appended, "\n")
	after := strings.Split(got, "\n")
	if len(before) != len(after) {
		t.Fatalf("the rerun changed the line count:\n%s", got)
	}
	begin, end := -1, -1
	for i, line := range before {
		switch line {
		case "# dinah-setup:begin trial/notes":
			begin = i
		case "# dinah-setup:end trial/notes":
			end = i
		}
	}
	if begin < 0 || end < 0 {
		t.Fatalf("the markers are not in the file:\n%s", appended)
	}
	changed := 0
	for i := range before {
		if before[i] == after[i] {
			continue
		}
		changed++
		if i <= begin || i >= end {
			t.Errorf("line %d outside the markers changed: %q became %q", i+1, before[i], after[i])
		}
	}
	if changed == 0 {
		t.Errorf("the rerun with another agent changed no line; rows %v", changesOf(report))
	}
	if !strings.HasPrefix(got, "# Notes\n\nOur own text.\n\n# dinah-setup:begin trial/notes\n") {
		t.Errorf("the section was not appended after the file's own text with one empty line:\n%s", got)
	}
}

// TestAHandWrittenEntryIsAConflictUnlessItMatches refuses a hand-written
// mcpServers.dinah that differs from the rendered entry, names it, and leaves
// every file and the ledger alone. Beside it, an entry equal to the rendered
// one is adopted and reported unchanged.
func TestAHandWrittenEntryIsAConflictUnlessItMatches(t *testing.T) {
	f := newFixture(t)
	writeFiles(t, f.project, map[string]string{".mcp.json": "{\n  \"mcpServers\": {\n    \"dinah\": {\"command\": \"dinah\", \"args\": [\"mcp\"]}\n  }\n}\n"})
	project := tree(t, f.project)
	_, err := Run(f.options("claude-code"))
	if refusalName(err) != contract.SetupConflict {
		t.Fatalf("wanted %s, got %v", contract.SetupConflict, err)
	}
	var refusal *contract.Refusal
	errors.As(err, &refusal)
	if !strings.Contains(refusal.Detail, ".mcp.json /mcpServers/dinah") {
		t.Errorf("the conflict does not name the entry: %q", refusal.Detail)
	}
	sameTree(t, "refused run", project, tree(t, f.project))
	if _, err := os.Stat(f.ledgerPath()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the refused run wrote a ledger")
	}

	accepting := newFixture(t)
	mustRun(t, accepting.options("claude-code"))
	if err := os.Remove(accepting.ledgerPath()); err != nil {
		t.Fatal(err)
	}
	adopted := mustRun(t, accepting.options("claude-code"))
	allChanges(t, adopted, ChangeUnchanged)
}

// TestAnEditedSectionIsAConflict changes one line inside a section setup
// wrote, and holds the rerun to refusing it by name.
func TestAnEditedSectionIsAConflict(t *testing.T) {
	f := newFixture(t)
	opts := f.trialOptions(trialDir(t, f, nil))
	mustRun(t, opts)
	notes := filepath.Join(f.project, "NOTES.md")
	edited := strings.Replace(readFile(t, notes), "works on", "sometimes works on", 1)
	writeFiles(t, f.project, map[string]string{"NOTES.md": edited})
	_, err := Run(opts)
	if refusalName(err) != contract.SetupConflict || !strings.Contains(err.Error(), "NOTES.md trial/notes") {
		t.Fatalf("wanted a conflict naming NOTES.md trial/notes, got %v", err)
	}
	if got := readFile(t, notes); got != edited {
		t.Errorf("the refused rerun changed the edited file")
	}
}

// TestADryRunWritesNothing builds a base where a run would create, add,
// update and remove, and holds a dry run to reporting all four with a before
// and after for each while leaving every file and the ledger byte for byte.
func TestADryRunWritesNothing(t *testing.T) {
	f := newFixture(t)
	opts := f.options("claude-code")
	opts.Agent = "first"
	opts.Model = "m1"
	mustRun(t, opts)
	if err := os.Remove(filepath.Join(f.project, ".claude", "rules", "dinah.md")); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(f.project, ".claude", "settings.local.json")
	text := readFile(t, settings)
	text = strings.Replace(text, "    \"DINAH_HARNESS\": \"claude-code\",\n", "", 1)
	writeFiles(t, f.project, map[string]string{".claude/settings.local.json": text})
	project := tree(t, f.project)
	ledger := readFile(t, f.ledgerPath())

	dry := f.options("claude-code")
	dry.Agent = "second"
	dry.DryRun = true
	report := mustRun(t, dry)
	seen := map[string]bool{}
	for _, c := range report.Changes {
		seen[c.Change] = true
		if c.Change == ChangeUnchanged {
			continue
		}
		if c.Before == "" && c.After == "" {
			t.Errorf("the row %s %s %s carries neither a before nor an after", c.File, c.Key, c.Change)
		}
	}
	for _, token := range []string{ChangeCreate, ChangeAdd, ChangeUpdate, ChangeRemove} {
		if !seen[token] {
			t.Errorf("the dry run reported no %s row, so the base does not exercise it: %v", token, changesOf(report))
		}
	}
	if !report.DryRun {
		t.Error("the report does not say it was a dry run")
	}
	sameTree(t, "dry run", project, tree(t, f.project))
	if got := readFile(t, f.ledgerPath()); got != ledger {
		t.Error("the dry run changed the ledger")
	}
}

// TestADryRunIntoAnEmptyBaseCreatesNothing holds a dry run to creating no
// file, the ledger included, where every change would be a creation.
func TestADryRunIntoAnEmptyBaseCreatesNothing(t *testing.T) {
	f := newFixture(t)
	opts := f.options("claude-code")
	opts.DryRun = true
	report := mustRun(t, opts)
	allChanges(t, report, ChangeCreate)
	if files := tree(t, f.project); len(files) != 0 {
		t.Errorf("the dry run created %v", files)
	}
	if _, err := os.Stat(f.ledgerPath()); !errors.Is(err, fs.ErrNotExist) {
		t.Error("the dry run created a ledger")
	}
}

// TestRemoveLeavesTheBaseAsItWas takes back an apply into an empty base, and
// one into a base holding content of its own, and holds each base to what it
// was before the apply. A second removal has nothing on record.
func TestRemoveLeavesTheBaseAsItWas(t *testing.T) {
	f := newFixture(t)
	opts := f.options("claude-code")
	opts.Model = "m1"
	mustRun(t, opts)
	removal := f.options("claude-code")
	removal.Remove = true
	report := mustRun(t, removal)
	if report.Nothing || len(report.Changes) == 0 {
		t.Fatalf("the removal reported nothing: %v", changesOf(report))
	}
	if files := tree(t, f.project); len(files) != 0 {
		t.Errorf("the removal left files behind: %v", files)
	}
	if report.Prompt == "" || !strings.Contains(report.Prompt, "claude mcp remove dinah --scope user") {
		t.Errorf("the removal does not carry remove.md: %q", report.Prompt)
	}
	again := mustRun(t, removal)
	if !again.Nothing {
		t.Errorf("a second removal found something to take back: %v", changesOf(again))
	}

	g := newFixture(t)
	own := map[string]string{
		".mcp.json":                   "{\n  \"mcpServers\": {\n    \"other\": {\"command\": \"x\"}\n  }\n}\n",
		".claude/settings.local.json": "{\r\n  \"permissions\": {\r\n    \"allow\": []\r\n  }\r\n}\r\n",
		"notes.txt":                   "left alone\n",
	}
	writeFiles(t, g.project, own)
	before := tree(t, g.project)
	mustRun(t, g.options("claude-code"))
	withContent := g.options("claude-code")
	withContent.Remove = true
	mustRun(t, withContent)
	sameTree(t, "removal from a base with content", before, tree(t, g.project))
}

// TestAnApplyWithoutAModelConverges applies with a model and then without
// one, and holds both JSON files to losing DINAH_MODEL.
func TestAnApplyWithoutAModelConverges(t *testing.T) {
	f := newFixture(t)
	opts := f.options("claude-code")
	opts.Model = "m1"
	mustRun(t, opts)
	for _, name := range []string{".mcp.json", ".claude/settings.local.json"} {
		if !strings.Contains(readFile(t, filepath.Join(f.project, name)), "DINAH_MODEL") {
			t.Fatalf("%s carries no DINAH_MODEL after an apply naming a model", name)
		}
	}
	opts.Model = ""
	report := mustRun(t, opts)
	for _, name := range []string{".mcp.json", ".claude/settings.local.json"} {
		text := readFile(t, filepath.Join(f.project, name))
		if strings.Contains(text, "DINAH_MODEL") {
			t.Errorf("%s still carries DINAH_MODEL:\n%s", name, text)
		}
		if _, err := parseJSONFile([]byte(text)); err != nil {
			t.Errorf("%s no longer parses: %v", name, err)
		}
	}
	if report.Count(ChangeRemove) != 1 || report.Count(ChangeUpdate) != 1 {
		t.Errorf("wanted one removal and one update, got %v", changesOf(report))
	}
}

// TestAUserRecipeOverridesTheShippedOne puts a claude-code recipe in the user
// base and holds setup to applying it and the listing to marking it used and
// the shipped one not. A trusted project recipe overrides the user's in turn,
// and one declaring a user step is malformed.
func TestAUserRecipeOverridesTheShippedOne(t *testing.T) {
	f := newFixture(t)
	fresh := List("", f.userBase)
	if len(fresh) != 3 || fresh[0].Name != "claude-code" || fresh[1].Name != "codex" || fresh[2].Name != "devin" {
		t.Fatalf("with no user or project recipe the listing is %+v", fresh)
	}
	for _, row := range fresh {
		if row.Source != SourceShipped || !row.Used || row.Path != "" || row.Malformed != "" {
			t.Errorf("a shipped row reads %+v", row)
		}
	}

	userRecipe := map[string]string{}
	shippedPlace := places("", "", false)[0]
	err := fs.WalkDir(shippedPlace.fsys, "claude-code", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(shippedPlace.fsys, path)
		userRecipe[strings.TrimPrefix(path, "claude-code/")] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	userRecipe["files/instructions-project.md"] = "The user's own rules for {{agent}}.\n"
	writeFiles(t, filepath.Join(f.userBase, "recipes", "claude-code"), userRecipe)
	report := mustRun(t, f.options("claude-code"))
	if report.Recipe.Source != SourceUser {
		t.Errorf("the run used the %s recipe", report.Recipe.Source)
	}
	if got := readFile(t, filepath.Join(f.project, ".claude", "rules", "dinah.md")); got != "The user's own rules for claude.\n" {
		t.Errorf("the rules file is %q", got)
	}
	listed := List(containerOf(f.workbench), f.userBase)
	usedBy := map[string]bool{}
	for _, row := range listed {
		if row.Name == "claude-code" {
			usedBy[row.Source] = row.Used
		}
	}
	if !usedBy[SourceUser] || usedBy[SourceShipped] {
		t.Errorf("the listing marks %v", usedBy)
	}

	projectRecipe := map[string]string{}
	for name, text := range userRecipe {
		projectRecipe[name] = text
	}
	projectRecipe["files/instructions-project.md"] = "The project's rules for {{agent}}.\n"
	projectRecipe["steps.json"] = readShippedStepsWithoutUser(t)
	projectRecipe["recipe.json"] = strings.Replace(userRecipe["recipe.json"], `"scopes": ["project", "user"]`, `"scopes": ["project"]`, 1)
	writeFiles(t, filepath.Join(f.project, ".dinah", "recipes", "claude-code"), projectRecipe)
	trusted := f.options("claude-code")
	trusted.TrustProjectRecipe = true
	if err := os.Remove(filepath.Join(f.project, ".claude", "rules", "dinah.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.ledgerPath()); err != nil {
		t.Fatal(err)
	}
	projectReport := mustRun(t, trusted)
	if projectReport.Recipe.Source != SourceProject {
		t.Errorf("the trusted run used the %s recipe", projectReport.Recipe.Source)
	}
	if got := readFile(t, filepath.Join(f.project, ".claude", "rules", "dinah.md")); got != "The project's rules for claude.\n" {
		t.Errorf("the rules file is %q", got)
	}

	projectRecipe["recipe.json"] = userRecipe["recipe.json"]
	projectRecipe["steps.json"] = readShippedSteps(t)
	writeFiles(t, filepath.Join(f.project, ".dinah", "recipes", "claude-code"), projectRecipe)
	_, err = Run(trusted)
	if refusalName(err) != contract.MalformedRecipe || !strings.Contains(err.Error(), "user step") {
		t.Errorf("a project recipe with a user step: wanted %s, got %v", contract.MalformedRecipe, err)
	}
}

// readShippedSteps is the shipped claude-code steps.json.
func readShippedSteps(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(shippedRecipes, "recipes/claude-code/steps.json")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// readShippedStepsWithoutUser is the shipped claude-code steps with its two
// user-scope steps cut out, which a project recipe may carry. The steps are
// cut as text so that every member keeps the order the shipped recipe gives
// it, which is the order setup writes.
func readShippedStepsWithoutUser(t *testing.T) string {
	t.Helper()
	steps := readShippedSteps(t)
	from := strings.Index(steps, ",\n    {\n      \"id\": \"environment-user\"")
	to := strings.LastIndex(steps, "\n  ]")
	if from < 0 || to < from {
		t.Fatal("the shipped steps no longer read the way this cut expects")
	}
	return steps[:from] + steps[to:]
}

// TestAProjectApplyGivesAnAGENTSFileNoCLAUDEFile holds a claude-code apply
// into a project that keeps an AGENTS.md to writing no CLAUDE.md anywhere,
// since Claude Code stops reading AGENTS.md beside one, and to writing its
// rules file instead.
func TestAProjectApplyGivesAnAGENTSFileNoCLAUDEFile(t *testing.T) {
	f := newFixture(t)
	writeFiles(t, f.project, map[string]string{"AGENTS.md": "# Agents\n"})
	mustRun(t, f.options("claude-code"))
	for _, name := range []string{"CLAUDE.md", ".claude/CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(f.project, filepath.FromSlash(name))); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("the apply wrote %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(f.project, ".claude", "rules", "dinah.md")); err != nil {
		t.Errorf("the apply wrote no rules file: %v", err)
	}
	if got := readFile(t, filepath.Join(f.project, "AGENTS.md")); got != "# Agents\n" {
		t.Errorf("the apply changed AGENTS.md: %q", got)
	}
}

// TestTheHomeIsNeverAProjectBase refuses a target naming the home directory
// or its parent, and the implicit target of a workbench kept in the user base,
// and applies with a target beside the home.
func TestTheHomeIsNeverAProjectBase(t *testing.T) {
	f := newFixture(t)
	for _, target := range []string{f.home, filepath.Dir(f.home)} {
		opts := f.options("claude-code")
		opts.Target = target
		if _, err := Run(opts); refusalName(err) != contract.SetupNoTarget {
			t.Errorf("a target of %s: wanted %s, got %v", target, contract.SetupNoTarget, err)
		}
	}
	inUserBase := f.options("claude-code")
	inUserBase.Workbench = filepath.Join(f.userBase, fixedWorkbench)
	if _, err := Run(inUserBase); refusalName(err) != contract.SetupNoTarget {
		t.Errorf("a workbench in the user base: wanted %s, got %v", contract.SetupNoTarget, err)
	}
	beside := filepath.Join(f.root, "elsewhere")
	if err := os.MkdirAll(beside, 0o755); err != nil {
		t.Fatal(err)
	}
	accepting := f.options("claude-code")
	accepting.Target = beside
	report := mustRun(t, accepting)
	if report.BaseDir != beside {
		t.Errorf("the run wrote into %s", report.BaseDir)
	}
	for name := range tree(t, f.home) {
		if name != ".dinah/"+LedgerName {
			t.Errorf("a project run wrote %s into the home", name)
		}
	}
}

// TestUserScopeRefusesARelocatedHome refuses a user-scope run while
// DINAH_HOME names another directory, writing nothing, and applies when it
// names the home itself.
func TestUserScopeRefusesARelocatedHome(t *testing.T) {
	f := newFixture(t)
	elsewhere := filepath.Join(f.root, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := f.options("claude-code")
	opts.Scope = ScopeUser
	opts.DinahHome = elsewhere
	if _, err := Run(opts); refusalName(err) != contract.SetupRelocatedHome {
		t.Fatalf("wanted %s, got %v", contract.SetupRelocatedHome, err)
	}
	if files := tree(t, f.home); len(files) != 0 {
		t.Errorf("the refused run wrote %v", files)
	}
	opts.DinahHome = f.home
	report := mustRun(t, opts)
	if report.Scope != ScopeUser || report.Workbench != "" {
		t.Errorf("the report reads scope %s and workbench %q", report.Scope, report.Workbench)
	}
	if !strings.Contains(readFile(t, filepath.Join(f.home, ".claude", "CLAUDE.md")), "<!-- dinah-setup:begin claude-code/instructions-user -->") {
		t.Error("the user-scope run wrote no section into the user's CLAUDE.md")
	}
}

// TestASecondWorkbenchInOneProjectIsRefused sets up the first of two
// workbenches sharing one container, refuses the second by naming the first
// with the files unchanged, and applies the second once the first is removed.
func TestASecondWorkbenchInOneProjectIsRefused(t *testing.T) {
	f := newFixture(t)
	mustRun(t, f.options("claude-code"))
	project := tree(t, f.project)
	second := f.options("claude-code")
	second.Workbench = filepath.Join(f.project, ".dinah", "fedcba9876543210fedcba9876543210")
	_, err := Run(second)
	if refusalName(err) != contract.SetupOtherWorkbench || !strings.Contains(err.Error(), f.workbench) {
		t.Fatalf("wanted %s naming %s, got %v", contract.SetupOtherWorkbench, f.workbench, err)
	}
	sameTree(t, "refused second workbench", project, tree(t, f.project))
	removal := f.options("claude-code")
	removal.Remove = true
	mustRun(t, removal)
	report := mustRun(t, second)
	if report.Workbench != slashPath(second.Workbench) {
		t.Errorf("the second apply was set up for %s", report.Workbench)
	}
}

// TestDevinHasNoUserScope refuses devin at user scope, because the Devin
// CLI's per-user directory is a different relative path on Windows from the
// one it is elsewhere and a step's path is one string. A project apply writes
// the two files the recipe declares and nothing else under the base.
func TestDevinHasNoUserScope(t *testing.T) {
	f := newFixture(t)
	opts := f.options("devin")
	opts.Scope = ScopeUser
	before := tree(t, f.project)
	_, err := Run(opts)
	if refusalName(err) != contract.UnknownScope {
		t.Fatalf("wanted %s, got %v", contract.UnknownScope, err)
	}
	if !strings.Contains(err.Error(), ScopeUser) {
		t.Errorf("the refusal does not name the scope it refused: %v", err)
	}
	sameTree(t, "the refused user-scope run", before, tree(t, f.project))
	if _, err := os.Stat(f.ledgerPath()); err == nil {
		t.Error("the refused user-scope run wrote a ledger")
	}
	applied := f.options("devin")
	applied.Model = "m1"
	mustRun(t, applied)
	files := tree(t, f.project)
	if len(files) != 2 || files["AGENTS.md"] == "" || files[".devin/mcp_config.json"] == "" {
		t.Errorf("the devin apply wrote %v", sortedKeys(files))
	}
}

// TestCodexHasNoUserScope refuses codex at user scope, and holds a project
// apply to writing AGENTS.md and nothing under .codex.
func TestCodexHasNoUserScope(t *testing.T) {
	f := newFixture(t)
	opts := f.options("codex")
	opts.Scope = ScopeUser
	if _, err := Run(opts); refusalName(err) != contract.UnknownScope {
		t.Fatalf("wanted %s, got %v", contract.UnknownScope, err)
	}
	mustRun(t, f.options("codex"))
	files := tree(t, f.project)
	if len(files) != 1 || files["AGENTS.md"] == "" {
		t.Errorf("the codex apply wrote %v", sortedKeys(files))
	}
	if _, err := os.Stat(f.ledgerPath()); err != nil {
		t.Errorf("the codex apply wrote no ledger: %v", err)
	}
}

// providerlessTrial writes the trial recipe with its provider member removed,
// which is how a recipe declares no provider default.
func providerlessTrial(t *testing.T, f *fixture) string {
	t.Helper()
	without := strings.Replace(trialRecipe["recipe.json"], "  \"provider\": \"acme\",\n", "", 1)
	if without == trialRecipe["recipe.json"] {
		t.Fatal("the trial recipe no longer carries a provider member, so this helper removes nothing")
	}
	return trialDir(t, f, map[string]string{"recipe.json": without})
}

// TestAManifestReadsProviderFiveWays holds recipe.json's provider member to
// the four readings that are the contract: absent means the recipe declares
// none, and an empty value, a whitespace-only value, a value that is not one
// word and a null are each the defect they have always been. The
// whitespace-only case is the one that separates absence from blankness, and
// a reading that treats a blank value as absent passes every other case here.
func TestAManifestReadsProviderFiveWays(t *testing.T) {
	f := newFixture(t)
	read, err := readRecipe(os.DirFS(providerlessTrial(t, f)), "trial", SourcePath, "")
	if err != nil {
		t.Fatalf("a recipe.json with no provider member does not read: %v", err)
	}
	if read.Provider != "" {
		t.Errorf("a recipe.json with no provider member reads the provider %q", read.Provider)
	}
	for _, written := range []string{`""`, `" "`, `"two words"`, `null`} {
		manifest := strings.Replace(trialRecipe["recipe.json"], `"provider": "acme"`, `"provider": `+written, 1)
		if manifest == trialRecipe["recipe.json"] {
			t.Fatalf("the provider member was not replaced by %s, so this case reads the accepting recipe", written)
		}
		dir := trialDir(t, f, map[string]string{"recipe.json": manifest})
		_, err := readRecipe(os.DirFS(dir), "trial", SourcePath, "")
		if err == nil {
			t.Errorf(`"provider": %s was read rather than refused`, written)
			continue
		}
		if want := manifestFile + ": carries no one-word provider"; err.Error() != want {
			t.Errorf(`"provider": %s was refused as %q, want %q`, written, err.Error(), want)
		}
	}
}

// TestARecipeWithNoProviderWritesNoProviderMember applies a recipe declaring
// no provider and holds the written JSON to carrying no DINAH_PROVIDER member
// at all rather than an empty one, then applies it again with --provider and
// holds the member to being written.
func TestARecipeWithNoProviderWritesNoProviderMember(t *testing.T) {
	f := newFixture(t)
	dir := providerlessTrial(t, f)
	opts := f.trialOptions(dir)
	opts.Model = "m1"
	mustRun(t, opts)
	settings := filepath.Join(f.project, "conf", "settings.json")
	if got := readFile(t, settings); strings.Contains(got, "PROVIDER") {
		t.Errorf("the apply wrote a provider member with no provider to write:\n%s", got)
	}

	g := newFixture(t)
	named := g.trialOptions(trialDir(t, g, map[string]string{
		"recipe.json": strings.Replace(trialRecipe["recipe.json"], "  \"provider\": \"acme\",\n", "", 1),
		"steps.json": `{
  "steps": [
    {"id": "settings", "kind": "json-merge", "scope": "project", "path": "conf/settings.json",
     "pointer": "/env", "value": {"PROVIDER": "{{provider}}", "AGENT": "{{agent}}"}}
  ]
}
`,
	}))
	named.Provider = "acme"
	mustRun(t, named)
	got := readFile(t, filepath.Join(g.project, "conf", "settings.json"))
	if !strings.Contains(got, `"PROVIDER": "acme"`) {
		t.Errorf("--provider acme wrote no provider member:\n%s", got)
	}
}

// TestTheProviderFlagIsStillHeldToOneWord holds the value setup would write
// as the provider to being one word, which is the check this card made
// conditional. The single space is the case a condition asking whether the
// value looks empty would let through, and the other values each trim to
// something, so all four are needed rather than one standing for the family.
// The accepting cases are pinned beside them, because a refusal every value
// satisfies is not a refusal.
func TestTheProviderFlagIsStillHeldToOneWord(t *testing.T) {
	f := newFixture(t)
	dir := providerlessTrial(t, f)
	values := []string{" ", "two words", `a"b`, "a b"}
	refused := 0
	for _, recipe := range []string{"trial", "devin"} {
		for _, value := range values {
			opts := f.trialOptions(dir)
			if recipe == "devin" {
				opts = f.options("devin")
			}
			opts.Model = "m1"
			opts.Provider = value
			_, err := Run(opts)
			if refusalName(err) != contract.Malformed {
				t.Errorf("%s with --provider %q was answered with %v, want %s", recipe, value, err, contract.Malformed)
				continue
			}
			if !strings.Contains(err.Error(), "--provider") {
				t.Errorf("%s with --provider %q was refused without naming the flag: %v", recipe, value, err)
			}
			refused++
		}
	}
	if refused != 2*len(values) {
		t.Errorf("%d of %d provider values were refused over the two provider-less recipes", refused, 2*len(values))
	}

	accepting := f.trialOptions(dir)
	accepting.Model = "m1"
	accepting.Provider = "acme"
	mustRun(t, accepting)

	g := newFixture(t)
	bare := g.trialOptions(providerlessTrial(t, g))
	bare.Model = "m1"
	mustRun(t, bare)

	// A recipe carrying its own provider default is under the same check,
	// which is why the condition reads the resolved fact and not the flag.
	h := newFixture(t)
	shipped := h.options("claude-code")
	shipped.Provider = "two words"
	if _, err := Run(shipped); refusalName(err) != contract.Malformed {
		t.Errorf("a two-word --provider against claude-code was answered with %v, want %s", err, contract.Malformed)
	}
}

// TestTheShippedRecipesDeclareTheProvidersTheyDeclare holds each shipped
// recipe to the provider default it is meant to carry, so that restoring a
// placeholder to devin, or dropping one from claude-code or codex, fails here
// rather than passing quietly.
func TestTheShippedRecipesDeclareTheProvidersTheyDeclare(t *testing.T) {
	want := map[string]string{"claude-code": "anthropic", "codex": "openai", "devin": ""}
	place := places("", "", false)[0]
	checked := 0
	for _, name := range shipped {
		r, err := place.open(name)
		if err != nil {
			t.Errorf("the shipped recipe %s does not read: %v", name, err)
			continue
		}
		declared, known := want[name]
		if !known {
			t.Errorf("the shipped recipe %s is not in this test's table, so nothing holds its provider", name)
			continue
		}
		if r.Provider != declared {
			t.Errorf("the shipped recipe %s declares the provider %q, want %q", name, r.Provider, declared)
		}
		checked++
	}
	if checked != len(want) {
		t.Errorf("checked %d shipped recipes of %d", checked, len(want))
	}
}
