package setup

import (
	"encoding/json"
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
	if len(shipped) != 3 || covered != 4 {
		t.Errorf("the goldens covered %d recipe scopes over %d shipped recipes, and the shipped list names claude-code at two scopes, codex at one and devin at one", covered, len(shipped))
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

	// A removal after the move takes back the block alone. The empty line
	// setup added before the block stays where the block first stood,
	// because section 5.8 takes that line only while it still stands
	// directly before the block.
	h := newFixture(t)
	hAgents := filepath.Join(h.project, "AGENTS.md")
	writeFiles(t, h.project, map[string]string{"AGENTS.md": codexFixtureAGENTS})
	mustRun(t, h.options("codex"))
	applied := readFile(t, hAgents)
	from := strings.Index(applied, "<!-- dinah-setup:begin")
	to := strings.Index(applied, endMarker) + len(endMarker)
	writeFiles(t, h.project, map[string]string{"AGENTS.md": applied[from:to] + applied[:from] + applied[to:]})
	removal := h.options("codex")
	removal.Remove = true
	removed := mustRun(t, removal)
	if len(removed.Changes) != 1 || removed.Changes[0].Change != ChangeRemove {
		t.Errorf("the removal after the move reported %v", changesOf(removed))
	}
	if got, want := readFile(t, hAgents), codexFixtureAGENTS+"\r\n\r\n"; got != want {
		t.Errorf("the removal after the move left %q, want the fixture with the added line break and the empty line %q", got, want)
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

// devinFixtureAGENTS is a hand-written AGENTS.md with headings, CRLF line
// endings, and a last line with no line break.
const devinFixtureAGENTS = "# House rules\r\n\r\nWrite the test first.\r\n\r\n## Review\r\n\r\nRead the diff before you push."

// devinFixtureMCP is a hand-written .devin/mcp_config.json holding a server
// the recipe does not own under /mcpServers, and a key outside /mcpServers
// that it does not own either.
const devinFixtureMCP = "{\r\n\t\"mcpServers\": {\r\n\t\t\"other\": {\"command\": \"x\", \"args\": [\"a\", \"b\"]}\r\n\t},\r\n\t\"unowned\": {\"keep\": true}\r\n}\r\n"

// TestTheDevinApplyWritesBothLocationsOverExistingContent applies devin into a
// project that already carries both of the files the recipe writes, which the
// goldens into an empty base cannot see. It holds the other server and the
// unowned key to their own bytes and their own order, the section to being
// appended after the hand-written text, a second apply to changing nothing,
// and a removal to giving both files back.
//
// The removal leaves AGENTS.md carrying one line break the fixture lacked,
// because setup ends a file that has no final line break before it appends a
// marked section, and section removal takes back only the block.
// TestTheCodexSectionSitsBesideAHandWrittenAGENTSFile pins the same byte, and
// TestADevinRemovalGivesBackAFileThatEndedInALineBreak holds a file that ends
// in a line break to exact restoration.
func TestTheDevinApplyWritesBothLocationsOverExistingContent(t *testing.T) {
	f := newFixture(t)
	agents := filepath.Join(f.project, "AGENTS.md")
	config := filepath.Join(f.project, ".devin", "mcp_config.json")
	writeFiles(t, f.project, map[string]string{
		"AGENTS.md":              devinFixtureAGENTS,
		".devin/mcp_config.json": devinFixtureMCP,
	})
	opts := f.options("devin")
	opts.Agent = "helper"
	opts.Model = "claude-opus-5"
	report := mustRun(t, opts)
	if len(report.Changes) != 2 {
		t.Fatalf("the apply reported %v, and the recipe declares two steps", changesOf(report))
	}

	merged := readFile(t, config)
	prefix := 0
	for prefix < len(devinFixtureMCP) && prefix < len(merged) && devinFixtureMCP[prefix] == merged[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(devinFixtureMCP)-prefix && suffix < len(merged)-prefix && devinFixtureMCP[len(devinFixtureMCP)-1-suffix] == merged[len(merged)-1-suffix] {
		suffix++
	}
	if prefix+suffix != len(devinFixtureMCP) {
		t.Errorf("the merge changed original bytes rather than only inserting: %d bytes kept before the change and %d after, of %d", prefix, suffix, len(devinFixtureMCP))
	}
	var parsed struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
		Unowned    json.RawMessage            `json:"unowned"`
	}
	if err := json.Unmarshal([]byte(merged), &parsed); err != nil {
		t.Fatalf("the merged file does not parse: %v", err)
	}
	if string(compactSpan(parsed.MCPServers["other"])) != `{"command":"x","args":["a","b"]}` {
		t.Errorf("the other server changed: %s", parsed.MCPServers["other"])
	}
	if string(compactSpan(parsed.Unowned)) != `{"keep":true}` {
		t.Errorf("the unowned key changed: %s", parsed.Unowned)
	}
	root, err := parseJSONFile([]byte(merged))
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, m := range root.members {
		order = append(order, m.name)
	}
	if strings.Join(order, ",") != "mcpServers,unowned" {
		t.Errorf("the top-level members are %v, and the unowned key moved", order)
	}
	if !strings.Contains(merged, `"dinah"`) {
		t.Errorf("the merge wrote no dinah entry:\n%s", merged)
	}

	written := readFile(t, agents)
	if !strings.HasPrefix(written, devinFixtureAGENTS+"\r\n\r\n<!-- dinah-setup:begin devin/instructions-project -->\r\n") {
		t.Errorf("the section is not appended after the hand-written text and one empty line:\n%q", written)
	}
	if strings.Contains(strings.ReplaceAll(written, "\r\n", ""), "\n") {
		t.Errorf("the apply left a bare LF in a CRLF file:\n%q", written)
	}

	project := tree(t, f.project)
	ledger := readFile(t, f.ledgerPath())
	second := mustRun(t, opts)
	allChanges(t, second, ChangeUnchanged)
	if len(second.Changes) != len(report.Changes) {
		t.Errorf("the second apply reported %d rows and the first %d", len(second.Changes), len(report.Changes))
	}
	sameTree(t, "second apply", project, tree(t, f.project))
	if got := readFile(t, f.ledgerPath()); got != ledger {
		t.Errorf("the second apply changed the ledger:\nbefore %s\nafter  %s", ledger, got)
	}

	removal := opts
	removal.Agent, removal.Model = "", ""
	removal.Remove = true
	mustRun(t, removal)
	if got := readFile(t, config); got != devinFixtureMCP {
		t.Errorf("the removal left .devin/mcp_config.json as %q, want the fixture %q", got, devinFixtureMCP)
	}
	if got, want := readFile(t, agents), devinFixtureAGENTS+"\r\n"; got != want {
		t.Errorf("the removal left AGENTS.md as %q, want the fixture with the one line break it lacked %q", got, want)
	}
}

// TestADevinRemovalGivesBackAFileThatEndedInALineBreak is the exact half of
// the removal criterion. Where the hand-written AGENTS.md already ends in a
// line break, setup adds no byte of its own, and the removal gives every file
// under the base back as it stood.
func TestADevinRemovalGivesBackAFileThatEndedInALineBreak(t *testing.T) {
	f := newFixture(t)
	writeFiles(t, f.project, map[string]string{
		"AGENTS.md":              devinFixtureAGENTS + "\r\n",
		".devin/mcp_config.json": devinFixtureMCP,
		"notes.txt":              "left alone\n",
	})
	before := tree(t, f.project)
	opts := f.options("devin")
	opts.Agent = "helper"
	opts.Model = "claude-opus-5"
	mustRun(t, opts)
	removal := opts
	removal.Agent, removal.Model = "", ""
	removal.Remove = true
	mustRun(t, removal)
	sameTree(t, "removal from a base with content", before, tree(t, f.project))
}

// devinGolden reads one of the rendered devin goldens.
func devinGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(goldenRoot, "devin", "project", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestTheDevinPromptAndRemovalSayWhatTheReaderMustDo holds the rendered devin
// prompt and removal text to the instructions nothing else in the tree would
// notice losing: the check against the build's own rules paths, the failure
// path for a build that does not name AGENTS.md, the two flags the reader
// supplies because this recipe names no provider, and the sentence about a
// copy setup cannot take back. The goldens render with no provider, since the
// golden fixture sets a model and never a provider.
func TestTheDevinPromptAndRemovalSayWhatTheReaderMustDo(t *testing.T) {
	prompt := devinGolden(t, "prompt.md")
	for _, want := range []string{
		"devin rules paths",
		"AGENTS.md",
		"copy the whole marked block, markers included, into a file the build does name",
		"--provider",
		"--model",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the devin prompt golden does not carry %q", want)
		}
	}
	read := 0
	for _, line := range strings.Split(prompt, "\n") {
		for _, name := range []string{"DINAH_ACTOR", "DINAH_HARNESS", "DINAH_PROVIDER", "DINAH_MODEL", "DINAH_SERVER"} {
			at := strings.Index(line, name+"=")
			if at < 0 {
				continue
			}
			read++
			rest := line[at+len(name)+1:]
			if rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\r' {
				t.Errorf("the devin prompt assigns %s nothing: %q", name, line)
			}
		}
	}
	if read == 0 {
		t.Error("the devin prompt carries no environment assignment at all, so the empty-assignment check read nothing")
	}
	removal := devinGolden(t, "remove.md")
	for _, want := range []string{"Three things remain", "delete that copy yourself"} {
		if !strings.Contains(removal, want) {
			t.Errorf("the devin removal golden does not carry %q", want)
		}
	}
}
