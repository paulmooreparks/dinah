package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/completion"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// completionDefinition is the flow the completion tests stand in. Spec and
// Test give the comma-list terms the specification names somewhere to land,
// and the declared levels, tiers and route give each value completer a set.
const completionDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Completion",
  "levels": { "severity": ["trivial", "minor", "major"], "priority": ["later", "now"], "tier": ["small", "large"] },
  "routes": { "short": ["c10000000001", "c10000000004"] },
  "columns": [
    { "id": "c10000000001", "title": "Intake", "kind": "intake" },
    { "id": "c10000000002", "title": "Spec", "kind": "work" },
    { "id": "c10000000003", "title": "Test", "kind": "work" },
    { "id": "c10000000004", "title": "Done", "kind": "done" }
  ]
}`

// quotedTitle is a card title carrying a literal tab and quotation marks,
// which is the title a description has to survive in every shell.
const quotedTitle = "Say \"hi\"\tto \"them\""

// completionBench builds the completion flow, declares a hinted level, a
// level and a field value that would need quoting, and files four cards: two
// plain ones, one titled quotedTitle, and one whose title runs past sixty
// display columns. It returns the directory the workbench stands in.
func completionBench(t *testing.T) string {
	t.Helper()
	root := newBenchFromDefinition(t, completionDefinition)
	anchor := filepath.Join(benchDir(t, root), bench.WorkbenchAnchor)
	editWorkbenchAnchor(t, anchor, "  severity: [trivial, minor, major]\n",
		"  severity:\n    - trivial\n    - minor: Something small is off.\n    - very high\n    - \"quoted\"level\n    - major\n")
	editWorkbenchAnchor(t, anchor, "routes:\n",
		"fields:\n  card.kind:\n    type: string\n    meaning: what kind of work this card is\n    values: [bug, \"big feature\", chore, café, thé, théâtre]\nroutes:\n")
	for _, title := range []string{"First card", "Second card", quotedTitle, strings.Repeat("A long title ", 8)} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %q: %d %s", title, got.code, got.errw)
		}
	}
	return root
}

// answer is one callback's output, read back into its header and its lines.
type answer struct {
	// code is the exit code.
	code int
	// mode is the header's mode, empty when there was no header.
	mode string
	// inserts and descriptions are each candidate line's two halves.
	inserts      []string
	descriptions []string
	// raw is the whole of stdout, and errw the whole of stderr.
	raw  string
	errw string
}

// callBack runs the callback in process from a directory, as a script would,
// and reads the answer back.
func callBack(t *testing.T, dir string, args ...string) answer {
	t.Helper()
	got := runCLI(t, dir, append([]string{"__complete", "1"}, args...)...)
	return readAnswer(t, got)
}

// readAnswer splits an invocation's stdout into its header and its lines,
// failing on any line that does not carry exactly one TAB.
func readAnswer(t *testing.T, got invocation) answer {
	t.Helper()
	a := answer{code: got.code, raw: got.out, errw: got.errw}
	if got.out == "" {
		return a
	}
	lines := strings.Split(strings.TrimSuffix(got.out, "\n"), "\n")
	header := strings.Fields(lines[0])
	if len(header) != 3 || header[0] != "dinah-complete" || header[1] != "1" {
		t.Fatalf("the header reads %q", lines[0])
	}
	a.mode = header[2]
	for _, line := range lines[1:] {
		if strings.Count(line, "\t") != 1 {
			t.Fatalf("the line %q carries %d TABs, wanted exactly one", line, strings.Count(line, "\t"))
		}
		insert, description, _ := strings.Cut(line, "\t")
		a.inserts = append(a.inserts, insert)
		a.descriptions = append(a.descriptions, description)
	}
	return a
}

// zsh completes the words the way the zsh script hands them over, the last
// being the current word.
func zsh(t *testing.T, dir string, words ...string) answer {
	t.Helper()
	return callBack(t, dir, append([]string{"zsh", "--"}, words...)...)
}

// bashLine completes a whole line the way the bash script hands it over, with
// bash's default word breaks.
func bashLine(t *testing.T, dir, line string) answer {
	t.Helper()
	return callBack(t, dir, "bash", " \t\n\"'><=;|&(:", line)
}

// TestTheCompletionVerbPrintsEachScript is dinah-601/criteria/1: each shell's
// script is printed with the protocol substituted and nothing else, the bytes
// are the same with and without a workbench, --json carries the object, and a
// missing or unknown shell is refused naming all four.
func TestTheCompletionVerbPrintsEachScript(t *testing.T) {
	root := completionBench(t)
	empty := t.TempDir()
	for _, shell := range completion.Shells {
		script, _ := completion.Script(shell)
		inside := runCLI(t, root, "completion", shell)
		if inside.code != 0 || inside.out != script || inside.errw != "" {
			t.Errorf("%s inside a workbench: exit %d, stderr %q, script printed as it is embedded %v", shell, inside.code, inside.errw, inside.out == script)
		}
		if strings.Contains(inside.out, "__DINAH_PROTOCOL__") {
			t.Errorf("%s: the protocol token was not substituted", shell)
		}
		t.Setenv("DINAH_HOME", filepath.Join(empty, "home"))
		outside := runCLI(t, empty, "completion", shell)
		if outside.code != 0 || outside.out != inside.out {
			t.Errorf("%s from a directory with no workbench: exit %d, and the bytes match those printed inside one %v", shell, outside.code, outside.out == inside.out)
		}
		machine := runCLI(t, empty, "completion", shell, "--json")
		var object map[string]any
		if err := json.Unmarshal([]byte(machine.out), &object); err != nil {
			t.Fatalf("%s --json: %v\n%s", shell, err, machine.out)
		}
		if len(object) != 3 || object["shell"] != shell || object["protocol"] != float64(1) || object["script"] != script {
			t.Errorf("%s --json carried %v", shell, object)
		}
	}
	for _, argv := range [][]string{{"completion"}, {"completion", "bogus"}} {
		got := runCLI(t, empty, argv...)
		if got.code != 2 || strings.Fields(got.errw)[0] != contract.UnknownShell {
			t.Errorf("%v: exit %d, stderr %q", argv, got.code, got.errw)
		}
		for _, shell := range completion.Shells {
			if !strings.Contains(got.errw, "\n  "+shell+"\n") {
				t.Errorf("%v: the refusal does not list %s:\n%s", argv, shell, got.errw)
			}
		}
	}
}

// TestTheCompletionVerbIsListedAndDocumented is dinah-601/criteria/2: the
// help block lists the verb in the serve group right after setup, its page
// carries the summary, the four profile lines and its one refusal row, and
// every catalog carries its keys.
func TestTheCompletionVerbIsListedAndDocumented(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(dir, "home"))
	block := runCLI(t, dir, "help").out
	setupAt := strings.Index(block, "\n  setup ")
	completionAt := strings.Index(block, "\n  completion <shell>")
	helpAt := strings.Index(block, "\n  help ")
	if setupAt < 0 || completionAt < setupAt {
		t.Fatalf("the help block does not list completion after setup:\n%s", block)
	}
	between := block[setupAt+1 : completionAt]
	for _, line := range strings.Split(between, "\n")[1:] {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.TrimSpace(line) != "" {
			t.Errorf("a command row stands between setup and completion: %q", line)
		}
	}
	if helpAt >= 0 && helpAt < completionAt {
		t.Error("completion is listed after the ungrouped help row rather than in the serve group")
	}
	page := runCLI(t, dir, "help", "completion").out
	english := msg.For(msg.Base)
	for _, want := range []string{
		english.T("cmd.completion.summary"),
		`eval "$(dinah completion bash)"`,
		`eval "$(dinah completion zsh)"`,
		"dinah completion fish | source",
		"dinah completion powershell | Out-String | Invoke-Expression",
		"(one of: powershell, bash, zsh, fish)",
		contract.UnknownShell,
	} {
		if !strings.Contains(strings.Join(strings.Fields(page), " "), strings.Join(strings.Fields(want), " ")) {
			t.Errorf("the completion page does not carry %q:\n%s", want, page)
		}
	}
	keys := []string{
		"cmd.completion.summary", "cmd.completion.note", "param.completion.shell.summary",
		verb.CheckKey("completion", 1), "refusal." + contract.UnknownShell, "refusal." + contract.UnknownShell + ".next",
	}
	checked := 0
	for _, tag := range msg.Tags() {
		for _, key := range keys {
			entry, ok := msg.CatalogEntry(tag, key)
			if !ok || entry.Text == "" {
				t.Errorf("the %s catalog carries no %s", tag, key)
			}
			checked++
		}
	}
	if checked != 8*len(keys) {
		t.Errorf("checked %d catalog entries, wanted %d across eight catalogs", checked, 8*len(keys))
	}
}

// TestTheCallbackIsHidden is dinah-601/criteria/3: no help page, catalog or
// tool names the callback, help refuses it as an unknown command, and every
// shape the protocol does not define exits 1 with both streams empty.
func TestTheCallbackIsHidden(t *testing.T) {
	root := completionBench(t)
	block := runCLI(t, root, "help").out
	if strings.Contains(block, completeCallback) {
		t.Errorf("the help block names %s", completeCallback)
	}
	for _, c := range commands {
		if c.name == completeCallback {
			t.Errorf("%s is a row of the command table", completeCallback)
		}
		if page := runCLI(t, root, "help", c.name).out; strings.Contains(page, completeCallback) {
			t.Errorf("the page of %s names %s", c.name, completeCallback)
		}
	}
	for _, tag := range msg.Tags() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "msg", "locales", tag+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte(completeCallback)) {
			t.Errorf("the %s catalog names %s", tag, completeCallback)
		}
	}
	asked := runCLI(t, root, "help", completeCallback)
	if asked.code != 2 || strings.Fields(asked.errw)[0] != contract.UnknownVerb {
		t.Errorf("help %s: exit %d, stderr %q", completeCallback, asked.code, asked.errw)
	}
	malformed := [][]string{
		{},
		{"2", "zsh", "--", "sh"},
		{"1", "cmd", "--", "sh"},
		{"1", "zsh", "sh"},
		{"1", "fish", "show", "x"},
		{"1", "zsh", "--"},
		{"1", "bash", "x"},
		{"1", "bash", "x", "y", "z"},
		{"1", "powershell", "extra"},
		{"--help"},
	}
	for _, args := range malformed {
		got := runCLI(t, root, append([]string{completeCallback}, args...)...)
		if got.code != 1 || got.out != "" || got.errw != "" {
			t.Errorf("__complete %q: exit %d, stdout %q, stderr %q", args, got.code, got.out, got.errw)
		}
	}
	for _, value := range []string{"", `[]`, `{"words":[],"replacing":""}`, `{"words":["a",1],"replacing":""}`, `{"words":["a"]}`, `{"words":["a"],"replacing":"","extra":1}`, `{"words":["show",null],"replacing":""}`, `{"words":"a","replacing":""}`, `not json`} {
		t.Setenv(completeWordsVariable, value)
		if value == "" {
			os.Unsetenv(completeWordsVariable)
		}
		got := runCLI(t, root, completeCallback, "1", "powershell")
		if got.code != 1 || got.out != "" || got.errw != "" {
			t.Errorf("powershell with %s=%q: exit %d, stdout %q, stderr %q", completeWordsVariable, value, got.code, got.out, got.errw)
		}
	}
}

// TestEveryArgumentDeclaresHowItCompletes is dinah-601/criteria/4's first
// guard: every non-marker parameter of every dispatched command resolves a
// completer, either by naming one of verb.Completers or through a vocabulary.
func TestEveryArgumentDeclaresHowItCompletes(t *testing.T) {
	known := map[string]bool{}
	for _, name := range verb.Completers {
		known[name] = true
	}
	checked := 0
	for _, c := range commands {
		for _, param := range verb.Params(c.name) {
			if param.Marker {
				continue
			}
			checked++
			if param.Complete != "" {
				if !known[param.Complete] {
					t.Errorf("%s.%s names the completer %q, which verb.Completers does not declare", c.name, param.Name, param.Complete)
				}
				continue
			}
			if _, ok := verb.VocabularyFor(c.name, param.Name); !ok {
				t.Errorf("%s.%s declares no completer and no vocabulary, so a shell cannot complete it", c.name, param.Name)
			}
		}
	}
	if checked < 100 {
		t.Errorf("checked %d parameters, which is fewer than the table declares", checked)
	}
	t.Logf("checked %d non-marker parameters across %d commands", checked, len(commands))
}

// TestEveryCompleterHasAResolver is dinah-601/criteria/4's second guard: the
// declared completers and the resolvers are the same set, in both directions,
// and every global flag carrying a value names a completer something resolves.
func TestEveryCompleterHasAResolver(t *testing.T) {
	for _, name := range verb.Completers {
		if _, ok := completers[name]; !ok {
			t.Errorf("verb.Completers declares %q and no resolver answers it", name)
		}
	}
	declared := map[string]bool{}
	for _, name := range verb.Completers {
		declared[name] = true
	}
	for name := range completers {
		if !declared[name] {
			t.Errorf("a resolver answers %q, which verb.Completers does not declare", name)
		}
	}
	if len(completers) != len(verb.Completers) || len(completers) != 22 {
		t.Errorf("checked %d resolvers against %d completers, wanted 22 of each", len(completers), len(verb.Completers))
	}
	for _, flag := range globalFlags {
		if flag.marker {
			continue
		}
		_, session := sessionCompleters[flag.complete]
		_, shared := completers[flag.complete]
		if !session && !shared {
			t.Errorf("the global flag --%s names the completer %q, which nothing resolves", flag.name, flag.complete)
		}
	}
}

// TestVerbsAndFlagsComeFromTheTables is dinah-601/criteria/5: the first word
// offers every command and each alias carrying no defect, -- offers exactly
// each command's declared flags and the eight global flags, and an alias
// completes as its expansion unless it carries a placeholder.
func TestVerbsAndFlagsComeFromTheTables(t *testing.T) {
	root := completionBench(t)
	for _, alias := range [][]string{{"alias.mv", "move"}, {"alias.take", "claim $1"}} {
		if got := runCLI(t, root, "config", "set", alias[0], alias[1]); got.code != 0 {
			t.Fatalf("config set %s: %d %s", alias[0], got.code, got.errw)
		}
	}
	first := zsh(t, root, "")
	if len(first.inserts) != len(commands)+2 {
		t.Errorf("the first word offered %d names, wanted %d commands and 2 aliases", len(first.inserts), len(commands))
	}
	for i, c := range commands {
		if i >= len(first.inserts) || first.inserts[i] != c.name {
			t.Fatalf("offer %d is not %s, in table order: %q", i, c.name, first.inserts)
		}
	}
	if first.inserts[len(commands)] != "mv" || first.inserts[len(commands)+1] != "take" {
		t.Errorf("the aliases were not offered after the commands, by name: %q", first.inserts[len(commands):])
	}
	commandsChecked := 0
	for _, c := range commands {
		got := zsh(t, root, c.name, "--")
		var want []string
		for _, param := range verb.Params(c.name) {
			if param.Flag {
				want = append(want, "--"+param.Spelling())
			}
		}
		for _, flag := range globalFlags {
			want = append(want, "--"+flag.name)
		}
		if strings.Join(got.inserts, " ") != strings.Join(want, " ") {
			t.Errorf("%s --: offered %q, wanted %q", c.name, got.inserts, want)
		}
		if len(got.inserts) != len(want) || len(want) < 8 {
			t.Errorf("%s --: offered %d flags, wanted %d including the eight global flags", c.name, len(got.inserts), len(want))
		}
		commandsChecked++
	}
	if commandsChecked != len(commands) {
		t.Errorf("checked the flags of %d commands, wanted %d", commandsChecked, len(commands))
	}
	moved := zsh(t, root, "mv", "fx-1", "")
	direct := zsh(t, root, "move", "fx-1", "")
	if strings.Join(moved.inserts, " ") != strings.Join(direct.inserts, " ") || len(direct.inserts) == 0 {
		t.Errorf("the alias mv offered %q where move offers %q", moved.inserts, direct.inserts)
	}
	if placeholder := zsh(t, root, "take", ""); placeholder.mode != completion.ModeWords || len(placeholder.inserts) != 0 {
		t.Errorf("an alias with a placeholder completed %q after itself", placeholder.inserts)
	}
}

// TestTheBashCallbackReplacesOnlyWhatReadlineReplaces is the callback half
// of dinah-601/criteria/6: with bash's default word breaks, a term after a
// colon inserts only what follows the colon, and a comma list keeps its
// earlier values.
func TestTheBashCallbackReplacesOnlyWhatReadlineReplaces(t *testing.T) {
	root := completionBench(t)
	cases := []struct {
		line, mode string
		want       []string
	}{
		{"dinah query colu", completion.ModeNospace, []string{"column:"}},
		{"dinah query column:sp", completion.ModeWords, []string{"spec"}},
		{"dinah query state:re", completion.ModeWords, []string{"ready"}},
		{"dinah query column:spec,te", completion.ModeWords, []string{"spec,test"}},
		{"dinah show x --fields=card,bo", completion.ModeWords, []string{"card,body"}},
		{"dinah show x --fields card,bo", completion.ModeWords, []string{"card,body"}},
	}
	for _, c := range cases {
		got := bashLine(t, root, c.line)
		if got.mode != c.mode || strings.Join(got.inserts, " ") != strings.Join(c.want, " ") {
			t.Errorf("%q: mode %s, inserts %q; wanted %s, %q", c.line, got.mode, got.inserts, c.mode, c.want)
		}
		for _, description := range got.descriptions {
			if description != "" {
				t.Errorf("%q: bash was sent the description %q", c.line, description)
			}
		}
	}
}

// TestATitleWithATabAndQuotesIsOneDescribedLine is the callback half of
// dinah-601/criteria/7: the title reaches zsh, fish and PowerShell as one
// line with one TAB, the reference unchanged and the description made one
// line with its quotation marks kept, and a long title is cut at sixty
// display columns.
func TestATitleWithATabAndQuotesIsOneDescribedLine(t *testing.T) {
	root := completionBench(t)
	t.Setenv(completeWordsVariable, `{"words":["show","fx-3"],"replacing":"fx-3"}`)
	answers := map[string]answer{
		"zsh":        zsh(t, root, "show", "fx-3"),
		"fish":       callBack(t, root, "fish", "--", "show", "fx-3"),
		"powershell": callBack(t, root, "powershell"),
	}
	for shell, got := range answers {
		if len(got.inserts) != 1 || got.inserts[0] != "fx-3" || got.descriptions[0] != `Say "hi" to "them"` {
			t.Errorf("%s: inserts %q, descriptions %q", shell, got.inserts, got.descriptions)
		}
	}
	long := zsh(t, root, "show", "fx-4")
	if len(long.descriptions) != 1 || !strings.HasSuffix(long.descriptions[0], "\u2026") || len([]rune(long.descriptions[0])) != 60 {
		t.Errorf("a long title was described as %q", long.descriptions)
	}
}

// TestTheCallbackAnswersWithNoWorkbench is dinah-601/criteria/8: from a
// directory no workbench is found from, and from one whose workbench anchor
// is malformed, the callback answers the header, offers every command for the
// first word and nothing for a card, and writes nothing to stderr. A field
// name still completes there, from the fields every kind carries, since the
// keys a workbench declares are the only half that needs one.
func TestTheCallbackAnswersWithNoWorkbench(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(empty, "home"))
	t.Setenv("DINAH_WORKBENCH", "")
	malformed := completionBench(t)
	anchor := filepath.Join(benchDir(t, malformed), bench.WorkbenchAnchor)
	if err := os.WriteFile(anchor, []byte("---\nformat: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DINAH_HOME", filepath.Join(empty, "home"))
	for name, dir := range map[string]string{"no workbench": empty, "a malformed workbench": malformed} {
		first := zsh(t, dir, "")
		if first.code != 0 || first.mode != completion.ModeWords || len(first.inserts) != len(commands) || first.errw != "" {
			t.Errorf("%s, first word: exit %d, mode %s, %d offers, stderr %q", name, first.code, first.mode, len(first.inserts), first.errw)
		}
		cards := zsh(t, dir, "show", "")
		if cards.code != 0 || cards.mode != completion.ModeWords || len(cards.inserts) != 0 || cards.errw != "" {
			t.Errorf("%s, show: exit %d, mode %s, offers %q, stderr %q", name, cards.code, cards.mode, cards.inserts, cards.errw)
		}
		fields := zsh(t, dir, "get", "x", "")
		if strings.Join(fields.inserts, " ") != strings.Join(bench.AllFields(), " ") || fields.errw != "" {
			t.Errorf("%s, get x: offered %q, wanted every kind's fields %q", name, fields.inserts, bench.AllFields())
		}
	}
}

// moveFixture is the flow dinah-601/criteria/9 stands in. The card waits at
// Work, which holds on the way out for an item the card carries, so the two
// forward rows are refused by the exit hold. Full is at its capacity and Gate
// holds on the way in for another of the card's items, so two of the
// backward rows are refused too, and only Intake passes.
const moveFixture = `{
  "profile": "dinah-core/0.12",
  "title": "Moves",
  "columns": [
    { "id": "d10000000001", "title": "Intake", "kind": "intake" },
    { "id": "d10000000002", "title": "Full", "kind": "work", "capacity": 1 },
    { "id": "d10000000003", "title": "Gate", "kind": "work", "gate_items": true },
    { "id": "d10000000004", "title": "Work", "kind": "work", "gate_items": "out" },
    { "id": "d10000000005", "title": "Later", "kind": "work" },
    { "id": "d10000000006", "title": "Done", "kind": "done" }
  ]
}`

// moveBench builds moveFixture with the card fx-2 standing at Work and fx-1
// filling Full.
func moveBench(t *testing.T) string {
	t.Helper()
	root := newBenchFromDefinition(t, moveFixture)
	steps := [][]string{
		{"add", "Filling the full column"},
		{"move", "fx-1", "full"},
		{"add", "The card being moved"},
		{"move", "fx-2", "work"},
		{"file", "--column", "gate", "fx-2", "open_question", "Settled at Gate."},
		{"file", "--column", "work", "fx-2", "open_question", "Settled at Work."},
	}
	for _, step := range steps {
		if got := runCLI(t, root, step...); got.code != 0 {
			t.Fatalf("%v: %d %s", step, got.code, got.errw)
		}
	}
	return root
}

// copyWorkbench copies a workbench directory to a fresh one, so a move can be
// tried there without touching the original.
func copyWorkbench(t *testing.T, from string) string {
	t.Helper()
	to := filepath.Join(t.TempDir(), "workbench")
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", from, err)
	}
	return to
}

// columnRefsBut lists every column of a workbench but the one a card stands
// in, which is the set of rows legalMoves answers for a card outside a done
// column.
func columnRefsBut(t *testing.T, root, standing string) []string {
	t.Helper()
	opened, err := bench.Open(benchDir(t, root))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	var refs []string
	for _, column := range opened.Columns {
		if column.Ref() != standing {
			refs = append(refs, column.Ref())
		}
	}
	return refs
}

// moveOracle tries dinah move card column for every row on a fresh copy of the
// workbench, with the environment the test has set, and answers the rows the
// move accepted and how many rows it tried.
func moveOracle(t *testing.T, root, card string, rows []string, extra ...string) (accepted []string, tried int) {
	t.Helper()
	for _, row := range rows {
		fresh := copyWorkbench(t, root)
		argv := append([]string{"move", card, row}, extra...)
		if got := runCLI(t, fresh, argv...); got.code == 0 {
			accepted = append(accepted, row)
		}
		tried++
	}
	return accepted, tried
}

// TestAMoveOffersOnlyTheDestinationThatPasses is dinah-601/criteria/9: of the
// five rows legalMoves answers for the card, capacity, the entry hold and the
// exit hold refuse four, and the callback offers the one left, which is
// exactly the set the move itself accepts row by row.
func TestAMoveOffersOnlyTheDestinationThatPasses(t *testing.T) {
	root := moveBench(t)
	rows := columnRefsBut(t, root, "work")
	offered := zsh(t, root, "move", "fx-2", "")
	accepted, tried := moveOracle(t, root, "fx-2", rows)
	if len(rows) != 5 || tried != len(rows) {
		t.Fatalf("the oracle tried %d of %d rows, wanted 5", tried, len(rows))
	}
	if strings.Join(offered.inserts, " ") != "intake" || strings.Join(accepted, " ") != "intake" {
		t.Errorf("offered %q, the move accepted %q, wanted intake alone from both", offered.inserts, accepted)
	}
}

// TestTheMoveFilterRefusesExactlyWhereTheMoveRefuses is dinah-601/criteria/10:
// in each case the destinations offered are exactly the rows a real move
// accepts on a fresh copy, and a lapsed claim is cleared in memory only.
func TestTheMoveFilterRefusesExactlyWhereTheMoveRefuses(t *testing.T) {
	type moveCase struct {
		name    string
		arrange func(t *testing.T, root string)
		extra   []string
		empty   bool
	}
	cases := []moveCase{
		{name: "no actor resolves", empty: true, arrange: func(t *testing.T, root string) {
			t.Setenv("DINAH_ACTOR", "")
			os.Unsetenv("DINAH_ACTOR")
		}},
		{name: "a malformed harness", empty: true, arrange: func(t *testing.T, root string) {
			t.Setenv("DINAH_HARNESS", "a b")
		}},
		{name: "no operator", empty: true, arrange: func(t *testing.T, root string) {
			editWorkbenchAnchor(t, filepath.Join(benchDir(t, root), bench.WorkbenchAnchor), "operator: alka\n", "")
		}},
		{name: "an override by the operator", extra: []string{"--override"}},
		{name: "a card held by another owner", empty: true, arrange: func(t *testing.T, root string) {
			if got := runCLI(t, root, "claim", "fx-2", "--actor", "bob"); got.code != 0 {
				t.Fatalf("claim as bob: %d %s", got.code, got.errw)
			}
		}},
		{name: "a blocked card", empty: true, arrange: func(t *testing.T, root string) {
			if got := runCLI(t, root, "block", "fx-2", "waiting"); got.code != 0 {
				t.Fatalf("block: %d %s", got.code, got.errw)
			}
		}},
		{name: "a lapsed claim by another owner", arrange: func(t *testing.T, root string) {
			if got := runCLI(t, root, "claim", "fx-2", "--actor", "bob", "--expires", "1h"); got.code != 0 {
				t.Fatalf("claim as bob: %d %s", got.code, got.errw)
			}
			anchor := anchorText(t, root, "fx-2")
			lapsed := regexpReplace(anchor, `claim_expires: .*`, "claim_expires: 2000-01-01T00:00:00Z")
			writeAnchor(t, root, "fx-2", lapsed)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newBenchFromDefinition(t, completionDefinition)
			for _, step := range [][]string{{"add", "The card being moved"}, {"add", "The card it moves beside"}, {"move", "fx-2", "spec"}} {
				if got := runCLI(t, root, step...); got.code != 0 {
					t.Fatalf("%v: %d %s", step, got.code, got.errw)
				}
			}
			if c.arrange != nil {
				c.arrange(t, root)
			}
			anchorBefore := anchorText(t, root, "fx-2")
			journalBefore := journalText(t, root, "fx-2")
			words := append([]string{"move", "fx-2"}, c.extra...)
			offered := zsh(t, root, append(words, "")...)
			if anchorText(t, root, "fx-2") != anchorBefore || journalText(t, root, "fx-2") != journalBefore {
				t.Error("completing the move changed the card's anchor or journal")
			}
			rows := columnRefsBut(t, root, "spec")
			accepted, tried := moveOracle(t, root, "fx-2", rows, c.extra...)
			if tried != len(rows) || tried == 0 {
				t.Fatalf("the oracle tried %d of %d rows", tried, len(rows))
			}
			if strings.Join(offered.inserts, " ") != strings.Join(accepted, " ") {
				t.Errorf("offered %q, the move accepted %q", offered.inserts, accepted)
			}
			if c.empty != (len(accepted) == 0) {
				t.Errorf("the move accepted %q, and this case expects it to accept none: %v", accepted, c.empty)
			}
			if c.name == "a malformed harness" {
				refused := runCLI(t, copyWorkbench(t, root), "move", "fx-2", "test")
				if name := strings.Fields(refused.errw); len(name) == 0 || name[0] != contract.MalformedHarness {
					t.Errorf("the move with a malformed harness was refused %q, wanted %s", refused.errw, contract.MalformedHarness)
				}
			}
		})
	}
}

// journalText reads a card's journal.
func journalText(t *testing.T, root, ref string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(anchorPath(t, root, ref)), bench.JournalName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the journal of %s: %v", ref, err)
	}
	return string(data)
}

// anchorPath is where a card's anchor stands, as dinah path reports it.
func anchorPath(t *testing.T, root, ref string) string {
	t.Helper()
	got := runCLI(t, root, "path", ref)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", ref, got.code, got.errw)
	}
	return strings.TrimSpace(got.out)
}

// writeAnchor replaces a card's anchor.
func writeAnchor(t *testing.T, root, ref, text string) {
	t.Helper()
	if err := os.WriteFile(anchorPath(t, root, ref), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// regexpReplace replaces every match of a pattern.
func regexpReplace(text, pattern, with string) string {
	return regexp.MustCompile(pattern).ReplaceAllString(text, with)
}

// TestQueryTermsComplete is dinah-601/criteria/17: field names end in a colon
// with no space, and after each field's operator the values come from the
// workbench definition alone.
func TestQueryTermsComplete(t *testing.T) {
	root := completionBench(t)
	if got := runCLI(t, root, "workstream", "new", "Release", "--slug", "release"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}
	names := zsh(t, root, "query", "")
	var want []string
	for _, field := range verb.QueryFields {
		if field != verb.FieldAt {
			want = append(want, field+":")
		}
	}
	want = append(want, "card.kind:")
	if names.mode != completion.ModeNospace || strings.Join(names.inserts, " ") != strings.Join(want, " ") {
		t.Errorf("query field names: mode %s, %q; wanted %q", names.mode, names.inserts, want)
	}
	columns := "column:intake column:spec column:test column:done"
	values := map[string]string{
		"column:":     columns,
		"entered:":    strings.ReplaceAll(columns, "column:", "entered:"),
		"left:":       strings.ReplaceAll(columns, "column:", "left:"),
		"state:":      "state:ready state:active state:blocked",
		"severity:":   "severity:trivial severity:minor severity:major",
		"priority:":   "priority:later priority:now",
		"workstream:": "workstream:release",
		"route:":      "route:short",
		"card.kind:":  "card.kind:bug card.kind:chore card.kind:café card.kind:thé card.kind:théâtre",
		"holder:":     "",
		"actor:":      "",
		"block_kind:": "",
		// The != operator is offered nothing, because section 4.3's list of
		// safe characters leaves ! out, so no candidate carrying it is written.
		"state!=": "",
	}
	for term, want := range values {
		got := zsh(t, root, "query", term)
		if got.mode != completion.ModeWords || strings.Join(got.inserts, " ") != want {
			t.Errorf("query %s: mode %s, %q; wanted %q", term, got.mode, got.inserts, want)
		}
	}
	events := zsh(t, root, "query", "event:")
	if len(events.inserts) != len(contract.Events) {
		t.Errorf("event: offered %d values, wanted the %d contract events", len(events.inserts), len(contract.Events))
	}
}

// TestOnlySafeWordsAreOffered is dinah-601/criteria/18: a declared level and
// a field value that would need quoting are not offered, and across every
// completer no candidate carries a character outside the safe set or begins
// with a dash unless it is a flag.
func TestOnlySafeWordsAreOffered(t *testing.T) {
	root := completionBench(t)
	levels := zsh(t, root, "add", "--severity", "")
	if strings.Join(levels.inserts, " ") != "trivial minor major" {
		t.Errorf("severity offered %q, wanted the three levels needing no quoting", levels.inserts)
	}
	kinds := zsh(t, root, "query", "card.kind:")
	if strings.Join(kinds.inserts, " ") != "card.kind:bug card.kind:chore card.kind:café card.kind:thé card.kind:théâtre" {
		t.Errorf("card.kind offered %q, wanted the five values needing no quoting", kinds.inserts)
	}
	swept := 0
	for _, words := range completerLines() {
		got := zsh(t, root, words...)
		flagPosition := strings.HasPrefix(words[len(words)-1], "-")
		for _, insert := range got.inserts {
			swept++
			if !completion.SafeWord(insert) {
				t.Errorf("%q offered %q, which needs quoting", words, insert)
			}
			if strings.HasPrefix(insert, "-") && !flagPosition {
				t.Errorf("%q offered %q, which begins with a dash outside the flag list", words, insert)
			}
		}
	}
	if swept < 50 {
		t.Errorf("swept %d candidates, which is too few to have reached every completer", swept)
	}
}

// completerLines are lines reaching every completer, the flag list and the
// first word, for the sweeps that have to touch all of them.
func completerLines() [][]string {
	return [][]string{
		{""}, {"--"}, {"show", "--"},
		{"add", "x", "--route", ""}, {"add", "x", "--priority", ""}, {"raise", "fx-1", ""},
		{"help", ""}, {"show", ""}, {"cite", ""}, {"instructions", ""}, {"list", ""},
		{"next", ""}, {"move", "fx-1", ""}, {"join", "fx-1", ""}, {"add", "x", "--severity", ""},
		{"setup", ""}, {"config", "get", ""}, {"config", ""}, {"get", "fx-1", ""},
		{"set", "fx-1", "severity", ""}, {"set", "fx-1", ""}, {"query", ""}, {"query", "column:"},
		{"attach", "fx-1", ""}, {"init", ""}, {"comment", "fx-1", ""}, {"guide", ""},
		{"completion", ""}, {"show", "x", "--fields", ""}, {"--lang", ""}, {"--format", ""},
		{"--workbench", ""}, {"--actor", ""},
	}
}

// TestFileAndDirectoryArgumentsHandOverToTheShell is the callback half of
// dinah-601/criteria/19.
func TestFileAndDirectoryArgumentsHandOverToTheShell(t *testing.T) {
	root := completionBench(t)
	cases := map[string][]string{
		completion.ModeFiles: {"attach", "fx-1", ""},
		completion.ModeDirs:  {"init", ""},
	}
	for mode, words := range cases {
		if got := zsh(t, root, words...); got.mode != mode || len(got.inserts) != 0 {
			t.Errorf("%q: mode %s with %d candidates, wanted %s and none", words, got.mode, len(got.inserts), mode)
		}
	}
	for _, words := range [][]string{{"init", "--from", ""}, {"init", "--from="}} {
		if got := zsh(t, root, words...); got.mode != completion.ModeFiles {
			t.Errorf("%q: mode %s, wanted files", words, got.mode)
		}
	}
	for _, words := range [][]string{{"--workbench", ""}, {"show", "--workbench", ""}} {
		if got := zsh(t, root, words...); got.mode != completion.ModeDirs {
			t.Errorf("%q: mode %s, wanted dirs", words, got.mode)
		}
	}
}

// TestAPowerShellWordThatDoesNotEndWithTheReplacementCompletesNothing pins
// the rule of section 3.4: when the current word does not end with the text
// PowerShell replaces, the callback cannot tell what the shell will replace
// and answers the header alone, and when it does, the insert is what follows.
func TestAPowerShellWordThatDoesNotEndWithTheReplacementCompletesNothing(t *testing.T) {
	root := completionBench(t)
	t.Setenv(completeWordsVariable, `{"words":["query","column:spec,te"],"replacing":"xx"}`)
	if got := callBack(t, root, "powershell"); got.mode != completion.ModeWords || len(got.inserts) != 0 {
		t.Errorf("a replacement the word does not end with completed %q", got.inserts)
	}
	t.Setenv(completeWordsVariable, `{"words":["query","column:spec,te"],"replacing":"te"}`)
	if got := callBack(t, root, "powershell"); strings.Join(got.inserts, " ") != "test" {
		t.Errorf("column:spec,te replacing te inserted %q, wanted test", got.inserts)
	}
	t.Setenv(completeWordsVariable, `{"words":["query","\"column:spec,te"],"replacing":"\"column:spec,te"}`)
	if got := callBack(t, root, "powershell"); len(got.inserts) != 0 {
		t.Errorf("a quoted current word completed %q", got.inserts)
	}
	// Two-byte letters ahead of the point PowerShell replaces from are where
	// a count in characters and a count in bytes disagree.
	accented := map[string]string{
		`{"words":["query","card.kind:café,th"],"replacing":"th"}`:      "thé théâtre",
		`{"words":["query","card.kind:théâtre,caf"],"replacing":"caf"}`: "café",
	}
	for words, want := range accented {
		t.Setenv(completeWordsVariable, words)
		if got := callBack(t, root, "powershell"); strings.Join(got.inserts, " ") != want {
			t.Errorf("%s inserted %q, wanted %q", words, got.inserts, want)
		}
	}
}

// TestTheCallbackAnswersTheSameWhoeverAsks is dinah-601/criteria/15: the
// identity variables change no completer's answer but the move filter's, a
// malformed harness empties the move alone, and a declared harness keeps the
// configured actor out of the move filter as the actor ladder decides.
func TestTheCallbackAnswersTheSameWhoeverAsks(t *testing.T) {
	root := completionBench(t)
	identity := []string{"DINAH_ACTOR", "DINAH_HARNESS", "DINAH_PROVIDER", "DINAH_MODEL", "DINAH_SERVER"}
	sweep := func() map[string]string {
		answers := map[string]string{}
		for _, words := range completerLines() {
			if words[0] == "move" {
				continue
			}
			answers[strings.Join(words, " ")] = zsh(t, root, words...).raw
		}
		return answers
	}
	for _, name := range identity {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	unset := sweep()
	if move := zsh(t, root, "move", "fx-1", ""); len(move.inserts) != 0 {
		t.Errorf("with no actor the move offered %q", move.inserts)
	}
	settings := [][]string{
		{"alka", "claude-code", "anthropic", "claude-opus-5-5", "https://example.test"},
		{"alka", "a b", "x y", "m\tn", "::"},
	}
	for _, values := range settings {
		for i, name := range identity {
			t.Setenv(name, values[i])
		}
		set := sweep()
		if len(set) != len(unset) {
			t.Fatalf("swept %d lines and %d lines", len(set), len(unset))
		}
		for line, text := range unset {
			if set[line] != text {
				t.Errorf("with %v the line %q answered differently:\n%s\nagainst\n%s", values, line, set[line], text)
			}
		}
		move := zsh(t, root, "move", "fx-1", "")
		malformed := values[1] == "a b"
		if malformed != (len(move.inserts) == 0) {
			t.Errorf("with the harness %q the move offered %q", values[1], move.inserts)
		}
	}
	for _, name := range identity {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	if got := runCLI(t, root, "config", "set", "actor", "alka"); got.code != 0 {
		t.Fatalf("config set actor: %d %s", got.code, got.errw)
	}
	if configured := zsh(t, root, "move", "fx-1", ""); len(configured.inserts) == 0 {
		t.Error("with no harness declared the configured actor was not used")
	}
	t.Setenv("DINAH_HARNESS", "claude-code")
	if harnessed := zsh(t, root, "move", "fx-1", ""); len(harnessed.inserts) != 0 {
		t.Errorf("with a harness declared and no DINAH_ACTOR the configured actor was used: %q", harnessed.inserts)
	}
}

// snapshot records every file under the directories given, with its bytes and
// its modification time.
func snapshot(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			info, statErr := entry.Info()
			data, readErr := os.ReadFile(path)
			if statErr != nil || readErr != nil {
				t.Fatalf("snapshot %s: %v %v", path, statErr, readErr)
			}
			files[path] = info.ModTime().String() + "\x00" + string(data)
			return nil
		})
	}
	return files
}

// TestTheCallbackWritesNothing is dinah-601/criteria/14: fifty callbacks
// across every completer leave every file under the workbench and the user
// base as it was, and on Linux a read-only copy answers identically.
func TestTheCallbackWritesNothing(t *testing.T) {
	root := completionBench(t)
	home := os.Getenv("DINAH_HOME")
	if got := runCLI(t, root, "move", "fx-2", "spec"); got.code != 0 {
		t.Fatalf("move: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "claim", "fx-2", "--actor", "bob", "--expires", "1h"); got.code != 0 {
		t.Fatalf("claim: %d %s", got.code, got.errw)
	}
	writeAnchor(t, root, "fx-2", regexpReplace(anchorText(t, root, "fx-2"), `claim_expires: .*`, "claim_expires: 2000-01-01T00:00:00Z"))
	lines := append(completerLines(), []string{"move", "fx-2", ""}, []string{"show", "fx-"})
	for len(lines) < 50 {
		lines = append(lines, lines[len(lines)%20])
	}
	lines = lines[:50]
	before := snapshot(t, root, home)
	writable := map[string]string{}
	for _, words := range lines {
		writable[strings.Join(words, " ")] = zsh(t, root, words...).raw
	}
	after := snapshot(t, root, home)
	if len(before) != len(after) {
		t.Errorf("the callbacks left %d files where there were %d", len(after), len(before))
	}
	for path, state := range before {
		if after[path] != state {
			t.Errorf("the callbacks changed %s", path)
		}
	}
	if runtime.GOOS != "linux" {
		t.Logf("ran %d callbacks; the read-only half runs on Linux", len(lines))
		return
	}
	readOnly(t, root, home)
	for _, words := range lines {
		got := zsh(t, root, words...)
		if got.raw != writable[strings.Join(words, " ")] || got.errw != "" {
			t.Errorf("%q on a read-only workbench answered %q, stderr %q", words, got.raw, got.errw)
		}
	}
}

// readOnly takes write permission away from every file and directory under
// the directories given, and gives it back when the test ends.
func readOnly(t *testing.T, dirs ...string) {
	t.Helper()
	var paths []string
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err == nil {
				paths = append(paths, path)
			}
			return nil
		})
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	for _, path := range paths {
		os.Chmod(path, 0o555)
	}
	t.Cleanup(func() {
		for i := len(paths) - 1; i >= 0; i-- {
			os.Chmod(paths[i], 0o755)
		}
	})
}

// largeFixture is the six-hundred-card workbench the budget, cap and expiry
// tests share. It is built once per test binary, because writing sixty
// megabytes of bodies is the slow part of every test that needs it.
var largeFixture struct {
	once sync.Once
	root string
	home string
	err  error
}

// largeCardsDefinition is the flow the large fixture stands in. Spec declares
// a capacity, so completing a move has to count every card.
const largeCardsDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Large",
  "columns": [
    { "id": "e10000000001", "title": "Intake", "kind": "intake" },
    { "id": "e10000000002", "title": "Spec", "kind": "work", "capacity": 10000 },
    { "id": "e10000000003", "title": "Done", "kind": "done" }
  ]
}`

// largeArchived are the numbers of the twenty archived cards, ten of them
// among the highest numbers so the cap would reach them if the archive leaked
// in.
func largeArchived(number int) bool {
	return (number > 610 && number <= 620) || (number > 400 && number <= 410)
}

// buildLargeFixture writes six hundred live cards with 100 KiB bodies and
// twenty archived ones straight in the storage format, numbered 1 to 620 in
// the registry. The workbench is created by init, so its anchor and columns
// are exactly what the tool writes.
func buildLargeFixture(t *testing.T) (string, string) {
	t.Helper()
	largeFixture.once.Do(func() {
		base, err := os.MkdirTemp("", "dinah-completion-large-*")
		if err != nil {
			largeFixture.err = err
			return
		}
		root := filepath.Join(base, "workbench")
		home := filepath.Join(base, "home")
		source := filepath.Join(base, "definition.json")
		os.MkdirAll(root, 0o755)
		os.WriteFile(source, []byte(largeCardsDefinition), 0o644)
		t.Setenv("DINAH_HOME", home)
		got := runCLI(t, root, "init", "--from", source, "--slug", "dinah", "--operator", "alka")
		if got.code != 0 {
			largeFixture.err = fmt.Errorf("init: %d %s", got.code, got.errw)
			return
		}
		entries, _ := os.ReadDir(filepath.Join(root, ".dinah"))
		workbench := filepath.Join(root, ".dinah", entries[0].Name())
		body := strings.Repeat(strings.Repeat("x", 1023)+"\n", 100)
		var registry strings.Builder
		for number := 1; number <= 620; number++ {
			id := fmt.Sprintf("%012x", 0xa00000000000+number)
			collection := filepath.Join(workbench, bench.CardsDir)
			if largeArchived(number) {
				collection = filepath.Join(workbench, bench.ArchiveDir, bench.CardsDir)
			}
			dir := filepath.Join(collection, id)
			os.MkdirAll(dir, 0o755)
			column := "e10000000001"
			if number%3 == 0 {
				column = "e10000000002"
			}
			anchor := "---\ntitle: Card number " + strconv.Itoa(number) + "\ncolumn: " + column + "\nstate: ready\n---\n\n" + body
			os.WriteFile(filepath.Join(dir, bench.CardAnchor), []byte(anchor), 0o644)
			journal := `{"ts":"2026-09-01T00:00:00Z","event":"created","actor":{"name":"alka"},"title":"Card number ` + strconv.Itoa(number) + `","to":"` + column + `","to_title":"Intake"}` + "\n"
			os.WriteFile(filepath.Join(dir, bench.JournalName), []byte(journal), 0o644)
			registry.WriteString(strconv.Itoa(number) + " " + id + "\n")
		}
		largeFixture.err = os.WriteFile(filepath.Join(workbench, bench.CardNumbersName), []byte(registry.String()), 0o644)
		largeFixture.root = root
		largeFixture.home = home
	})
	if largeFixture.err != nil {
		t.Fatalf("build the six-hundred-card fixture: %v", largeFixture.err)
	}
	t.Setenv("DINAH_HOME", largeFixture.home)
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_WORKBENCH", "")
	return largeFixture.root, largeFixture.home
}

// TestCardCompletionCapsAndMatchesOnTheRegistry is dinah-601/criteria/13: the
// cap keeps the two hundred highest-numbered live cards, a prefix keeps
// exactly the cards it names under either case, and under bash no card
// anchor is opened, which Linux proves by making every anchor unreadable.
func TestCardCompletionCapsAndMatchesOnTheRegistry(t *testing.T) {
	root, _ := buildLargeFixture(t)
	all := zsh(t, root, "show", "dinah-")
	if len(all.inserts) != 200 {
		t.Fatalf("show dinah- offered %d cards, wanted 200", len(all.inserts))
	}
	var want []string
	for number := 620; len(want) < 200; number-- {
		if !largeArchived(number) {
			want = append(want, "dinah-"+strconv.Itoa(number))
		}
	}
	if strings.Join(all.inserts, " ") != strings.Join(want, " ") {
		t.Errorf("the cap kept %q, wanted the 200 highest live cards %q", all.inserts[:5], want[:5])
	}
	prefix := zsh(t, root, "show", "dinah-59")
	folded := zsh(t, root, "show", "Dinah-59")
	if len(prefix.inserts) != 11 || strings.Join(folded.inserts, " ") != strings.Join(prefix.inserts, " ") {
		t.Errorf("dinah-59 offered %d cards and Dinah-59 %q, wanted the same 11", len(prefix.inserts), folded.inserts)
	}
	readable := []string{bashLine(t, root, "dinah show dinah-").raw, bashLine(t, root, "dinah show dinah-59").raw}
	if runtime.GOOS != "linux" {
		return
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".dinah"))
	cards := filepath.Join(root, ".dinah", entries[0].Name(), bench.CardsDir)
	ids, _ := os.ReadDir(cards)
	for _, id := range ids {
		anchor := filepath.Join(cards, id.Name(), bench.CardAnchor)
		os.Chmod(anchor, 0)
		defer os.Chmod(anchor, 0o644)
	}
	if len(ids) != 600 {
		t.Fatalf("made %d anchors unreadable, wanted 600", len(ids))
	}
	unreadable := []string{bashLine(t, root, "dinah show dinah-").raw, bashLine(t, root, "dinah show dinah-59").raw}
	if strings.Join(unreadable, "") != strings.Join(readable, "") {
		t.Error("bash answered differently once every card anchor was unreadable, so it opened one")
	}
}

// completeOnce runs the callback in process from a directory and answers how
// long runComplete took and what it wrote.
func completeOnce(t *testing.T, dir string, args ...string) (time.Duration, string) {
	t.Helper()
	previous, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(previous)
	home := bench.Home()
	cfg := bench.LoadConfig(home)
	var out bytes.Buffer
	started := time.Now()
	code := runComplete(append([]string{"1"}, args...), &out, home, cfg)
	took := time.Since(started)
	if code != 0 {
		t.Fatalf("the callback exited %d", code)
	}
	return took, out.String()
}

// percentile95 is the 95th percentile of twenty runs of a measurement.
func percentile95(runs int, measure func() time.Duration) time.Duration {
	taken := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		taken = append(taken, measure())
	}
	sort.Slice(taken, func(i, j int) bool { return taken[i] < taken[j] })
	return taken[(runs*95+99)/100-1]
}

// TestTheCallbackStaysInsideItsBudget is dinah-601/criteria/12: on six hundred
// cards with 100 KiB bodies, the four most expensive completions stay inside
// 100 ms in process, the show and query cases inside a quarter of reading
// every card, and the move case inside reading every header plus 30 ms, having
// read the six hundred headers exactly once.
func TestTheCallbackStaysInsideItsBudget(t *testing.T) {
	if os.Getenv(coverageChildMarker) != "" {
		t.Skip("the coverage run instruments every statement, so it measures the instrumentation rather than the budget")
	}
	root, _ := buildLargeFixture(t)
	opened, err := bench.Open(filepath.Join(root, ".dinah", soleName(t, filepath.Join(root, ".dinah"))))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cardsTime := percentile95(20, func() time.Duration {
		started := time.Now()
		if _, err := opened.Cards(); err != nil {
			t.Fatal(err)
		}
		return time.Since(started)
	})
	headersTime := percentile95(20, func() time.Duration {
		started := time.Now()
		if _, err := opened.LiveCardHeaders(); err != nil {
			t.Fatal(err)
		}
		return time.Since(started)
	})
	quarter := cardsTime / 4
	cases := []struct {
		name  string
		args  []string
		bound time.Duration
	}{
		{"show under zsh", []string{"zsh", "--", "show", "dinah-"}, quarter},
		{"show under bash", []string{"bash", " \t\n\"'><=;|&(:", "dinah show dinah-"}, quarter},
		{"query priority", []string{"zsh", "--", "query", "priority:"}, quarter},
		{"move", []string{"zsh", "--", "move", "dinah-1", ""}, headersTime + 30*time.Millisecond},
	}
	for _, c := range cases {
		took := percentile95(20, func() time.Duration {
			d, _ := completeOnce(t, root, c.args...)
			return d
		})
		t.Logf("%s: p95 %v; bound %v; reading every card p95 %v; reading every header p95 %v", c.name, took, c.bound, cardsTime, headersTime)
		if took > 100*time.Millisecond {
			t.Errorf("%s took %v at the 95th percentile, over the 100 ms budget", c.name, took)
		}
		if took > c.bound {
			t.Errorf("%s took %v at the 95th percentile, over its bound of %v", c.name, took, c.bound)
		}
	}
	completeOnce(t, root, "zsh", "--", "move", "dinah-1", "")
	if completeOpens != 600 {
		t.Errorf("completing a move opened %d files, wanted the 600 headers read once", completeOpens)
	}
}

// soleName is the one entry of a directory.
func soleName(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("%s holds %d entries: %v", dir, len(entries), err)
	}
	return entries[0].Name()
}

// TestRunningOutOfTimePrintsOnlyTheHeader is dinah-601/criteria/16: an expiry
// partway through the title read and partway through the occupancy read each
// answer the header alone, the counter proving where it fired, and a panic
// inside a completer exits 1 with both streams empty.
func TestRunningOutOfTimePrintsOnlyTheHeader(t *testing.T) {
	root, _ := buildLargeFixture(t)
	defer func() { completeExpireAfterOpens = -1 }()
	cases := []struct {
		after int
		args  []string
	}{
		{50, []string{"zsh", "--", "show", "dinah-"}},
		{300, []string{"zsh", "--", "move", "dinah-1", ""}},
	}
	for _, c := range cases {
		completeExpireAfterOpens = c.after
		_, out := completeOnce(t, root, c.args...)
		if out != "dinah-complete 1 words\n" {
			t.Errorf("%q expiring after %d opens wrote %q", c.args, c.after, firstLines(out, 3))
		}
		if completeOpens != c.after {
			t.Errorf("%q: the expiry fired after %d opens, wanted %d", c.args, completeOpens, c.after)
		}
	}
	completeExpireAfterOpens = -1
	saved := completers[verb.CompleteCard]
	completers[verb.CompleteCard] = func(*completionCall, verb.Param, string) (string, []completion.Candidate, error) {
		panic("a completer failed")
	}
	defer func() { completers[verb.CompleteCard] = saved }()
	got := runCLI(t, root, completeCallback, "1", "zsh", "--", "claim", "")
	if got.code != 1 || got.out != "" || got.errw != "" {
		t.Errorf("a panicking completer: exit %d, stdout %q, stderr %q", got.code, got.out, got.errw)
	}
}

// firstLines is the first n lines of a text, for a failure message.
func firstLines(text string, n int) string {
	lines := strings.SplitN(text, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
