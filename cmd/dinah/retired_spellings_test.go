package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/mcp"
	"dinah/internal/verb"
)

// retiredSpellings are the six kind-prefixed field invocations this workstream
// retired. They are written out here rather than derived, because the whole
// point is that nothing in the tool declares them any more: a set derived from
// the command table would be empty and every check over it would pass for
// free.
var retiredSpellings = []string{
	"card get", "card set",
	"workbench get", "workbench set",
	"workstream get", "workstream set",
}

// retiredSpellingPattern matches one retired spelling as two whole words. The
// boundaries are what the corpus needs rather than a nicety: a plain
// containment test reads "the workbench setting" as `workbench set` and "the
// next reader gets" as `workbench get`, and the two of them together account
// for most of the corpus's occurrences. What survives the boundaries is the
// genuine word pair, of the shape "the card set", where set is a noun, and
// that is what the allowlist below is for.
var retiredSpellingPattern = func() []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(retiredSpellings))
	for _, spelling := range retiredSpellings {
		patterns = append(patterns, regexp.MustCompile(`\b`+regexp.QuoteMeta(spelling)+`\b`))
	}
	return patterns
}()

// TestTheSixRetiredSpellingsRefuseAndTheirReplacementsWork asserts, per
// spelling, the refusal a reader now meets and the generic invocation that
// replaces it, in one run.
//
// The replacement half is what stops this passing against a tree where get and
// set do not work at all: a build that refused everything would fail the
// second column of every row. Each refusal assertion reads the first
// whitespace-separated token of stderr, which is the refusal name, and never
// the prose after it, so a reworded sentence cannot redden it.
func TestTheSixRetiredSpellingsRefuseAndTheirReplacementsWork(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card to reach"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "workstream", "new", "Autumn release", "--slug", "autumn"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}

	rows := []struct {
		retired     []string
		wanted      string
		replacement []string
		prints      string
	}{
		{
			// card has no act left once its two are gone, so the command
			// itself is retired and main's own dispatch answers the name.
			retired:     []string{"card", "get", "fx-1", "title"},
			wanted:      contract.UnknownVerb,
			replacement: []string{"get", "fx-1", "title"},
			prints:      "a card to reach",
		},
		{
			retired:     []string{"card", "set", "fx-1", "title", "renamed"},
			wanted:      contract.UnknownVerb,
			replacement: []string{"set", "fx-1", "title", "renamed"},
			prints:      "",
		},
		{
			// workbench keeps its bare listing, so the command survives and
			// its unknown first word falls through to the usage refusal.
			retired:     []string{"workbench", "get", "slug"},
			wanted:      contract.Usage,
			replacement: []string{"get", "workbench", "slug"},
			prints:      "fx",
		},
		{
			// The replacement is the write, and the read that proves it
			// landed runs after the loop, because a write prints the ok
			// line rather than the value it stored.
			retired:     []string{"workbench", "set", "title", "Renamed"},
			wanted:      contract.Usage,
			replacement: []string{"set", "workbench", "title", "A renamed workbench"},
			prints:      "",
		},
		{
			// workstream keeps its listing and its creating verb, so it
			// reaches the same fall-through.
			retired:     []string{"workstream", "get", "autumn", "status"},
			wanted:      contract.Usage,
			replacement: []string{"get", "workstream/autumn", "status"},
			prints:      "active",
		},
		{
			// The second of the two replacements that write. It runs after
			// the row above, whose read wants the value this one changes.
			retired:     []string{"workstream", "set", "autumn", "status", "finished"},
			wanted:      contract.Usage,
			replacement: []string{"set", "workstream/autumn", "status", "finished"},
			prints:      "",
		},
	}
	if len(rows) != len(retiredSpellings) {
		t.Fatalf("this check runs %d rows and %d spellings were retired", len(rows), len(retiredSpellings))
	}

	for _, row := range rows {
		spelling := strings.Join(row.retired, " ")
		refused := runCLI(t, root, row.retired...)
		if refused.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Errorf("`dinah %s` exited %d, wanted the refused exit code", spelling, refused.code)
		}
		if name := refusalNameOf(refused.errw); name != row.wanted {
			t.Errorf("`dinah %s` refused %s, wanted %s", spelling, name, row.wanted)
		}

		replacement := strings.Join(row.replacement, " ")
		answered := runCLI(t, root, row.replacement...)
		if answered.code != 0 {
			t.Errorf("`dinah %s` exited %d: %s", replacement, answered.code, answered.errw)
			continue
		}
		if row.prints == "" {
			continue
		}
		if !strings.Contains(answered.out, row.prints) {
			t.Errorf("`dinah %s` printed %q, wanted it to carry %q", replacement, answered.out, row.prints)
		}
	}

	// The two replacements above that write rather than read are read back
	// here, so each row proves its write landed rather than only that it
	// exited zero. Two rows retire a write, so two rows replace one, and a
	// row given another row's replacement leaves the write it stands for
	// unexercised.
	title := runCLI(t, root, "get", "workbench", "title")
	if got := strings.TrimSuffix(title.out, "\n"); got != "A renamed workbench" {
		t.Errorf("the workbench title reads back %q after the generic write", got)
	}
	status := runCLI(t, root, "get", "workstream/autumn", "status")
	if got := strings.TrimSuffix(status.out, "\n"); got != "finished" {
		t.Errorf("the workstream status reads back %q after the generic write", got)
	}
}

// TestTheMachineSurfaceRetiredTheKindShapedFieldActs asserts the same
// retirement on the head a person cannot see. The two capabilities left under
// one rule, so an agent must meet the same answer a reader at a terminal does.
func TestTheMachineSurfaceRetiredTheKindShapedFieldActs(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card to reach"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	dir := soleBenchDir(t, root)
	served := servedToolNames(t, dir)
	if len(served) == 0 {
		t.Fatal("this head serves no tool, so this check read nothing")
	}
	if served["card"] {
		t.Error("this head still serves a card tool, and the command behind it is gone")
	}
	for _, name := range []string{"get_field", "set_field"} {
		if !served[name] {
			t.Errorf("this head serves no %s tool", name)
		}
	}

	// A workbench or workstream call naming a retired action is refused,
	// while the same read through get_field answers.
	for _, call := range []struct {
		tool      string
		arguments map[string]any
	}{
		{tool: "workbench", arguments: map[string]any{"action": "get", "field": "slug"}},
		{tool: "workstream", arguments: map[string]any{"action": "set", "workstream": "autumn", "field": "status", "value": "finished"}},
	} {
		if err := callRefused(t, dir, call.tool, call.arguments); err == "" {
			t.Errorf("the %s tool accepted the retired action %v", call.tool, call.arguments["action"])
		}
	}
	value := callPayload(t, dir, "get_field", map[string]any{
		"actor": "alka", "ref": bench.WorkbenchRef, "field": bench.SlugField,
	})
	if value["value"] != "fx" {
		t.Errorf("get_field answered %v, wanted the workbench's own slug", value)
	}
}

// servedToolNames reads the names this head serves off a live tools/list
// answer.
func servedToolNames(t *testing.T, dir string) map[string]bool {
	t.Helper()
	var answer struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	decodeServed(t, dir, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, &answer)
	names := map[string]bool{}
	for _, tool := range answer.Result.Tools {
		names[tool.Name] = true
	}
	return names
}

// callRefused runs one tool call and returns the message of the protocol error
// or the refusal name it answered with, empty when the call succeeded.
func callRefused(t *testing.T, dir, tool string, arguments map[string]any) string {
	t.Helper()
	arguments["actor"] = "alka"
	var answer struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeServed(t, dir, toolCallLine(t, tool, arguments), &answer)
	if answer.Error.Message != "" {
		return answer.Error.Message
	}
	if len(answer.Result.Content) != 1 {
		t.Fatalf("the %s call carried %d content members", tool, len(answer.Result.Content))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(answer.Result.Content[0].Text), &payload); err != nil {
		t.Fatalf("decode the %s payload: %v", tool, err)
	}
	if refusal, refused := payload["refusal"].(string); refused {
		return refusal
	}
	return ""
}

// callPayload runs one tool call and returns the payload object it answered
// with, failing when the call was refused.
func callPayload(t *testing.T, dir, tool string, arguments map[string]any) map[string]any {
	t.Helper()
	var answer struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeServed(t, dir, toolCallLine(t, tool, arguments), &answer)
	if answer.Error.Message != "" {
		t.Fatalf("the %s call answered an error: %s", tool, answer.Error.Message)
	}
	if len(answer.Result.Content) != 1 {
		t.Fatalf("the %s call carried %d content members", tool, len(answer.Result.Content))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(answer.Result.Content[0].Text), &payload); err != nil {
		t.Fatalf("decode the %s payload: %v", tool, err)
	}
	return payload
}

// toolCallLine composes one tools/call request.
func toolCallLine(t *testing.T, tool string, arguments map[string]any) string {
	t.Helper()
	line, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": arguments},
	})
	if err != nil {
		t.Fatalf("marshal the call: %v", err)
	}
	return string(line)
}

// decodeServed runs one request through the machine head and decodes the
// answer into whatever shape the caller wants.
func decodeServed(t *testing.T, dir, line string, into any) {
	t.Helper()
	opened, err := bench.Open(dir)
	if err != nil {
		t.Fatalf("open %q: %v", dir, err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	out := &strings.Builder{}
	if err := mcp.Serve(dir, library, map[string]*verb.Library{}, strings.NewReader(line+"\n"), out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), into); err != nil {
		t.Fatalf("decode the answer: %v (%s)", err, out.String())
	}
}

// retiredSpellingExcuse is one phrase a retired spelling legitimately survives
// inside, the file it is expected in, and the reason it is legitimate.
//
// The excuse is keyed to the phrase rather than to the file, and the
// difference is the whole point of it. An excuse keyed to a file excuses every
// hit in that file, so a genuine `dinah card set` written into one of the two
// files below would have passed in silence, and the doc comment's promise that
// each occurrence is argued for would have held everywhere except inside the
// files it names.
type retiredSpellingExcuse struct {
	File   string
	Phrase string
	Reason string
}

// retiredSpellingAllowlist is every phrase argued for. Each is an unrelated
// word pair rather than a command anybody could type, which is what the reason
// has to say: a fifth phrase is an argument somebody makes rather than a silent
// match.
var retiredSpellingAllowlist = []retiredSpellingExcuse{
	{File: filepath.Join("internal", "verb", "library.go"), Phrase: "narrows the card set", Reason: "set is the noun rather than the verb"},
	{File: filepath.Join("internal", "verb", "library.go"), Phrase: "the no-card set", Reason: "set is the noun rather than the verb"},
	{File: filepath.Join("internal", "verb", "tree.go"), Phrase: "over the card set", Reason: "set is the noun rather than the verb"},
	{File: filepath.Join("internal", "verb", "tree.go"), Phrase: "partitions a card set", Reason: "set is the noun rather than the verb"},
}

// retiredSpellingRoots are the document trees this sweep walks, which is
// everything the binary ships or the project publishes to a reader.
var retiredSpellingRoots = []string{
	filepath.Join("..", "..", "internal", "guide", "guides"),
	filepath.Join("..", "..", "internal", "msg", "locales"),
	filepath.Join("..", "..", "docs", "quick-start.md"),
	filepath.Join("..", "..", "docs", "design"),
}

// retiredSpellingGoRoots are the source trees this sweep walks. Test files
// stay outside it deliberately: they invoke the surviving generic commands by
// the hundred and would drown the signal, and the counted call sites are how
// the sweep reaches them instead.
var retiredSpellingGoRoots = []string{
	filepath.Join("..", "..", "internal"),
	filepath.Join("..", "..", "cmd"),
}

// TestNoRetiredSpellingSurvivesInTheShippedCorpus walks everything the binary
// ships or the project publishes, plus every non-test Go file, and fails on
// any occurrence of a retired spelling outside the declared allowlist.
//
// The two file counts are separate on purpose, and the separation is
// load-bearing rather than tidy. A single total is satisfied by the documents
// alone, so a Go walk whose root had moved would read nothing while the
// combined figure stayed comfortably non-zero and the run stayed green. Each
// half is therefore fatal on its own zero.
func TestNoRetiredSpellingSurvivesInTheShippedCorpus(t *testing.T) {
	if len(retiredSpellings) == 0 {
		t.Fatal("no spelling is named as retired, so this sweep read nothing")
	}

	found := map[string]int{}
	documents := 0
	for _, root := range retiredSpellingRoots {
		documents += sweepRetired(t, root, func(path string) bool {
			return true
		}, found)
	}
	if documents == 0 {
		t.Fatal("the document walk read no file, so its roots have moved")
	}

	goFiles := 0
	for _, root := range retiredSpellingGoRoots {
		goFiles += sweepRetired(t, root, func(path string) bool {
			return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
		}, found)
	}
	if goFiles == 0 {
		t.Fatal("the Go walk read no file, so its roots have moved")
	}

	// An excuse whose phrase is no longer in the file is an argument for
	// something nobody is writing any more, and it is the way this list rots
	// into a file-wide exemption again.
	for _, excuse := range retiredSpellingAllowlist {
		if found[excuse.Phrase] == 0 {
			t.Errorf("the allowlist argues for %q in %s, and no file carries that phrase, so the entry has outlived its hit", excuse.Phrase, excuse.File)
		}
	}

	// The embedded help text is not a file on disk under either root, so it
	// is read off the binary's own rendering rather than walked to.
	block := tableSession(80).helpBlock()
	for at, spelling := range retiredSpellings {
		if retiredSpellingPattern[at].MatchString(block) {
			t.Errorf("the embedded help block carries the retired spelling %q", spelling)
		}
	}

	t.Logf("the sweep read %d documents and %d Go files", documents, goFiles)
}

// sweepRetired walks one root, reads every file the predicate admits, and
// reports how many it read. Every excused phrase is cut out of a file's text
// before that text is scanned, so what the patterns then see is whatever the
// argued-for phrases do not account for, and a genuine retired spelling in an
// excused file is caught like any other.
//
// A phrase is counted where it is found, and the caller checks the counts, so
// an excuse that has outlived the phrase it argues for is reported rather than
// left standing.
func sweepRetired(t *testing.T, root string, admits func(string) bool, found map[string]int) int {
	t.Helper()
	read := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !admits(path) {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		read++
		relative := relativeToRepo(path)
		text := string(body)
		for _, excuse := range retiredSpellingAllowlist {
			if excuse.File != relative {
				continue
			}
			if hits := strings.Count(text, excuse.Phrase); hits > 0 {
				found[excuse.Phrase] += hits
				text = strings.ReplaceAll(text, excuse.Phrase, "")
			}
		}
		for at, spelling := range retiredSpellings {
			if retiredSpellingPattern[at].MatchString(text) {
				t.Errorf("%s carries the retired spelling %q, and no allowlist entry argues that it is legitimate", relative, spelling)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return read
}

// relativeToRepo spells a walked path the way the allowlist names it, which is
// relative to the repository root rather than to this package. Every root this
// sweep walks is written as two steps up, so the two steps come off again.
func relativeToRepo(path string) string {
	up := ".." + string(filepath.Separator) + ".." + string(filepath.Separator)
	return strings.TrimPrefix(filepath.Clean(path), up)
}
