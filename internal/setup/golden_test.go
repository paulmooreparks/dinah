package setup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// goldenRoot is where the golden files live.
var goldenRoot = filepath.Join("testdata", "golden")

// placeholderFor replaces every spelling of a directory a written file can
// carry with a fixed token, so a golden reads the same on every platform and
// under every temporary directory: the directory as it stands, with forward
// slashes, and escaped as a JSON string and as a TOML string would carry it.
func placeholderFor(text, dir, token string) string {
	spellings := []string{
		strings.TrimSuffix(strings.TrimPrefix(tomlBasicString(dir), `"`), `"`),
		strings.TrimSuffix(strings.TrimPrefix(jsonString(dir), `"`), `"`),
		dir,
		filepath.ToSlash(dir),
	}
	for _, spelling := range spellings {
		text = strings.ReplaceAll(text, spelling, token)
	}
	return text
}

// golden compares a text with its golden file, or writes the file in update
// mode.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldenRoot, filepath.FromSlash(name))
	if *updateGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden %s: %v", name, err)
	}
	if string(want) != got {
		t.Errorf("%s differs from its golden:\nwant %q\ngot  %q", name, want, got)
	}
}

// goldenFiles lists the files a golden directory holds under files/.
func goldenFiles(t *testing.T, dir string) map[string]bool {
	t.Helper()
	listed := map[string]bool{}
	root := filepath.Join(goldenRoot, filepath.FromSlash(dir), "files")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		listed[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return listed
}

// TestEveryShippedRecipeMatchesItsGoldens applies every shipped recipe at
// every scope it declares into an empty base with fixed facts, and holds each
// written file, the rendered prompt.md and the rendered remove.md to the
// goldens under testdata/golden/<recipe>/<scope>/.
func TestEveryShippedRecipeMatchesItsGoldens(t *testing.T) {
	covered := 0
	for _, name := range shipped {
		recipe, err := places("", "", false)[0].open(name)
		if err != nil {
			t.Fatalf("read the shipped recipe %s: %v", name, err)
		}
		for _, scope := range recipe.Scopes {
			f := newFixture(t)
			opts := f.options(name)
			opts.Scope = scope
			opts.Agent = "helper"
			opts.Model = "claude-opus-5"
			base := f.project
			if scope == ScopeUser {
				base = f.home
			}
			report := mustRun(t, opts)
			dir := name + "/" + scope
			normalise := func(text string) string {
				text = placeholderFor(text, f.workbench, "<WORKBENCH>")
				return placeholderFor(text, base, "<BASE>")
			}
			golden(t, dir+"/prompt.md", normalise(report.Prompt))
			written := map[string]bool{}
			for rel, text := range tree(t, base) {
				if strings.HasPrefix(rel, ".dinah/") {
					continue
				}
				written[rel] = true
				golden(t, dir+"/files/"+rel, normalise(text))
			}
			if !*updateGoldens {
				want := goldenFiles(t, dir)
				for rel := range want {
					if !written[rel] {
						t.Errorf("%s: the golden holds %s and the apply wrote no such file", dir, rel)
					}
				}
				if len(written) != len(want) {
					t.Errorf("%s: the apply wrote %d files and the golden holds %d", dir, len(written), len(want))
				}
			}
			removal := opts
			removal.Agent, removal.Model = "", ""
			removal.Remove = true
			removed := mustRun(t, removal)
			golden(t, dir+"/remove.md", normalise(removed.Prompt))
			covered++
		}
		if covered == 0 {
			t.Errorf("the shipped recipe %s declares no scope", name)
		}
	}
	if len(shipped) != 2 || covered != 3 {
		t.Errorf("the goldens covered %d recipe scopes over %d shipped recipes, and the shipped list names claude-code at two scopes and codex at one", covered, len(shipped))
	}
	if *updateGoldens {
		t.Fatal("the goldens were rewritten; read the diff and commit it")
	}
}

// TestTheClaudeUserPromptPutsEveryEnvBeforeTheTransport holds the Claude Code
// prompt to the argument order its documentation requires: every --env pair
// before --transport stdio --scope user dinah, since a server name straight
// after --env is read as another pair.
func TestTheClaudeUserPromptPutsEveryEnvBeforeTheTransport(t *testing.T) {
	want := "claude mcp add --env DINAH_ACTOR=helper --env DINAH_HARNESS=claude-code --env DINAH_PROVIDER=anthropic --transport stdio --scope user dinah -- dinah mcp --tools all"
	data, err := os.ReadFile(filepath.Join(goldenRoot, "claude-code", "user", "prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), want) {
		t.Errorf("the user-scope prompt golden does not carry %q", want)
	}
}

// codexFixtureAGENTS is a hand-written AGENTS.md with headings, CRLF line
// endings, and a last line with no line break.
const codexFixtureAGENTS = "# Team notes\r\n\r\nUse tabs.\r\n\r\n## Build\r\n\r\nRun make before you push."

// TestTheCodexSectionSitsBesideAHandWrittenAGENTSFile appends the Codex
// section to a hand-written AGENTS.md and holds everything before it to the
// original bytes apart from the one line break the last line lacked, the
// section to CRLF, a rerun to unchanged, and a removal to the original with
// that one line break.
func TestTheCodexSectionSitsBesideAHandWrittenAGENTSFile(t *testing.T) {
	f := newFixture(t)
	agents := filepath.Join(f.project, "AGENTS.md")
	writeFiles(t, f.project, map[string]string{"AGENTS.md": codexFixtureAGENTS})
	report := mustRun(t, f.options("codex"))
	if len(report.Changes) != 1 || report.Changes[0].Change != ChangeAdd {
		t.Fatalf("the apply reported %v", changesOf(report))
	}
	got := readFile(t, agents)
	if !strings.HasPrefix(got, codexFixtureAGENTS+"\r\n\r\n<!-- dinah-setup:begin codex/instructions-project -->\r\n") {
		t.Errorf("the section is not appended after the original and one empty line:\n%q", got)
	}
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Errorf("the file carries a bare LF after the apply:\n%q", got)
	}
	golden(t, "codex/project-existing/AGENTS.md", placeholderFor(strings.ReplaceAll(got, "\r\n", "\n"), f.workbench, "<WORKBENCH>"))
	rerun := mustRun(t, f.options("codex"))
	allChanges(t, rerun, ChangeUnchanged)
	if readFile(t, agents) != got {
		t.Error("the rerun changed AGENTS.md")
	}
	removal := f.options("codex")
	removal.Remove = true
	mustRun(t, removal)
	if restored := readFile(t, agents); restored != codexFixtureAGENTS+"\r\n" {
		t.Errorf("the removal left %q", restored)
	}
	if *updateGoldens {
		t.Fatal("the golden was rewritten; read the diff and commit it")
	}
}

// TestTheCodexSectionCanBeMovedAndEditedAround moves the marked block to the
// top of the file, and edits a line inside it and lines outside it, holding
// setup to following the block wherever it stands, to refusing an edit inside
// it by name, and to leaving edits outside it alone.
func TestTheCodexSectionCanBeMovedAndEditedAround(t *testing.T) {
	f := newFixture(t)
	agents := filepath.Join(f.project, "AGENTS.md")
	writeFiles(t, f.project, map[string]string{"AGENTS.md": codexFixtureAGENTS})
	mustRun(t, f.options("codex"))
	text := readFile(t, agents)
	begin := strings.Index(text, "<!-- dinah-setup:begin")
	endMarker := "<!-- dinah-setup:end codex/instructions-project -->\r\n"
	end := strings.Index(text, endMarker) + len(endMarker)
	block := text[begin:end]
	moved := block + text[:begin] + text[end:]
	writeFiles(t, f.project, map[string]string{"AGENTS.md": moved})
	rerun := mustRun(t, f.options("codex"))
	allChanges(t, rerun, ChangeUnchanged)
	if readFile(t, agents) != moved {
		t.Error("the rerun moved or changed the block")
	}

	edited := strings.Replace(moved, "Run `dinah prime`", "Run `dinah status`", 1)
	if edited == moved {
		t.Fatal("the edit inside the block changed nothing, so the refusing case is not exercised")
	}
	writeFiles(t, f.project, map[string]string{"AGENTS.md": edited})
	project := tree(t, f.project)
	_, err := Run(f.options("codex"))
	if refusalName(err) != contract.SetupConflict || !strings.Contains(err.Error(), "AGENTS.md codex/instructions-project") {
		t.Errorf("an edit inside the block: wanted a conflict naming AGENTS.md codex/instructions-project, got %v", err)
	}
	sameTree(t, "refused rerun", project, tree(t, f.project))

	g := newFixture(t)
	writeFiles(t, g.project, map[string]string{"AGENTS.md": codexFixtureAGENTS})
	mustRun(t, g.options("codex"))
	outside := strings.Replace(readFile(t, filepath.Join(g.project, "AGENTS.md")), "Use tabs.", "Use spaces.", 1) + "A line added after the block.\r\n"
	writeFiles(t, g.project, map[string]string{"AGENTS.md": outside})
	accepting := mustRun(t, g.options("codex"))
	allChanges(t, accepting, ChangeUnchanged)
	if readFile(t, filepath.Join(g.project, "AGENTS.md")) != outside {
		t.Error("the rerun changed the edits outside the block")
	}
}

// TestTOMLValuesAreEscapedByTheOneHelper renders the Codex prompt with a
// workbench path carrying a backslash, a double quote and an apostrophe, and
// holds it to its golden, beside a plain path that renders with nothing
// escaped.
func TestTOMLValuesAreEscapedByTheOneHelper(t *testing.T) {
	recipe, err := places("", "", false)[0].open("codex")
	if err != nil {
		t.Fatal(err)
	}
	quoted := Facts{
		Harness: "codex", Agent: "codex", Tools: "all", Provider: "openai", Scope: ScopeProject,
		Base: `C:\work\o'neil "x"`, Workbench: `C:\work\o'neil "x"\.dinah\0123`, Recipe: "codex",
	}
	rendered, err := renderText(recipe.Prompt, quoted, true)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := `args = ["mcp", "--tools", "all", "--workbench", "C:\\work\\o'neil \"x\"\\.dinah\\0123"]`
	if !strings.Contains(rendered, wantArgs) {
		t.Errorf("the rendered prompt does not carry %s", wantArgs)
	}
	golden(t, "codex/project-quoted-path/prompt.md", rendered)

	plain := quoted
	plain.Base, plain.Workbench = "/srv/proj", "/srv/proj/.dinah/0123"
	rendered, err = renderText(recipe.Prompt, plain, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `"--workbench", "/srv/proj/.dinah/0123"]`) {
		t.Errorf("a plain path does not render as it stands in quotes:\n%s", rendered)
	}
	if *updateGoldens {
		t.Fatal("the golden was rewritten; read the diff and commit it")
	}
}

// TestTomlBasicStringEscapesWhatTOMLRequires holds the helper to the TOML
// 1.0.0 rule: a backslash, a double quote and every control character but tab
// escaped, and everything else, the apostrophe and the tab included, as it
// stands.
func TestTomlBasicStringEscapesWhatTOMLRequires(t *testing.T) {
	cases := map[string]string{
		"":                   `""`,
		"plain":              `"plain"`,
		`back\slash`:         `"back\\slash"`,
		`say "hi"`:           `"say \"hi\""`,
		"o'neil":             `"o'neil"`,
		"tab\there":          "\"tab\there\"",
		"nul\x00":            `"nul\u0000"`,
		"unit\x1f":           `"unit\u001F"`,
		"delete\x7f":         `"delete\u007F"`,
		"line\nbreak":        `"line\u000Abreak"`,
		"accented caf\u00e9": "\"accented caf\u00e9\"",
	}
	for in, want := range cases {
		if got := tomlBasicString(in); got != want {
			t.Errorf("tomlBasicString(%q) = %s, want %s", in, got, want)
		}
	}
}

// TestATOMLSuffixIsRefusedOutsideTheText refuses the TOML suffix in a step's
// path and any other suffix anywhere, beside a recipe whose suffix stands in
// its prompt and is accepted.
func TestATOMLSuffixIsRefusedOutsideTheText(t *testing.T) {
	refusing := map[string]map[string]string{
		"the suffix in a path":  {"steps.json": strings.Replace(trialRecipe["steps.json"], `"conf/rules.md"`, `"conf/{{agent|toml}}.md"`, 1)},
		"another suffix":        {"prompt.md": "Finish in {{base|upper}}.\n"},
		"an unknown name":       {"files/rules.md": "Rules for {{nobody}}.\n"},
		"an unclosed brace":     {"remove.md": "Undo {{base.\n"},
		"the suffix in a value": {"steps.json": strings.Replace(trialRecipe["steps.json"], `"{{agent}}"`, `"{{agent|toml}}"`, 1)},
	}
	for label, overrides := range refusing {
		f := newFixture(t)
		_, err := Run(f.trialOptions(trialDir(t, f, overrides)))
		if refusalName(err) != contract.MalformedRecipe {
			t.Errorf("%s: wanted %s, got %v", label, contract.MalformedRecipe, err)
		}
	}
	f := newFixture(t)
	report := mustRun(t, f.trialOptions(trialDir(t, f, map[string]string{"prompt.md": "Workbench {{workbench|toml}}.\n"})))
	if !strings.Contains(report.Prompt, tomlBasicString(f.workbench)) {
		t.Errorf("the accepted suffix did not render: %q", report.Prompt)
	}
}
