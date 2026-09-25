package mcp

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// updateGolden rewrites the MCP payload goldens from the head as it stands,
// then fails, so a run that rewrote them can never be read as a run that
// compared them. dinah-152.
var updateGolden = flag.Bool("update", false, "rewrite internal/mcp/testdata/golden from the head, then fail")

// goldenDir is where the MCP payload goldens live.
const goldenDir = "testdata/golden"

// goldenExclusions names every tool the golden sequence calls without
// recording its payload, with the reason. The key set is asserted to be
// exactly {version}, so a second exclusion is an edit a reviewer sees.
var goldenExclusions = map[string]string{
	"version": "its payload carries the path of the running test binary, which lies outside the fixture root, and a catalogs total that grows whenever a card adds a catalog key, so no masking of either would survive a different build",
}

// goldenDefinition is the workbench the golden sequence runs against. It is
// its own constant rather than the package's shared fixture, so an edit to
// that fixture for another test's sake does not rewrite every golden. It
// declares a tier axis because raise refuses on a workbench that declares
// none.
const goldenDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Golden",
  "instructions": "Standing text.\n",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake", "instructions": "Intake text.\n" },
    { "id": "c00000000002", "title": "Build", "kind": "work", "tier": "workhorse", "instructions": "Build text.\n" },
    { "id": "c00000000003", "title": "Review", "kind": "work", "instructions": "Review text.\n" },
    { "id": "c00000000004", "title": "Finished", "kind": "done" }
  ]
}`

// goldenCall is one call of the fixed sequence: the tool, the case word that
// tells two calls of one tool apart in the golden's file name, and the
// arguments.
type goldenCall struct {
	// tool is the tool the call names.
	tool string
	// kase tells two calls to one tool apart, and is empty on the first.
	kase string
	// arguments are the call's arguments.
	arguments map[string]any
}

// goldenAnswer is one call's answer as the sequence recorded it.
type goldenAnswer struct {
	// call is what was sent.
	call goldenCall
	// file is the golden's file name, empty for an excluded tool.
	file string
	// text is the raw text content the head answered with.
	text string
}

// goldenFixture is a fixture workbench and the spellings of its root.
type goldenFixture struct {
	// base is the directory the test created, which holds the workbench, the
	// user base and the file the attach call reads.
	base string
	// workbench is the workbench's own directory.
	workbench string
	// roots are every spelling of base the normaliser masks.
	roots []string
}

// newGoldenFixture builds the workbench, sets every DINAH_ variable the
// sequence's answers could reflect to a value the test chose, and clears
// every other one.
func newGoldenFixture(t *testing.T) *goldenFixture {
	t.Helper()
	base := t.TempDir()
	workbench := filepath.Join(base, bench.UserBaseName, fixtureWorkbenchID)
	read, err := bench.ReadDefinition([]byte(goldenDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := bench.Instantiate(workbench, "gd", "alka", read); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if err := os.WriteFile(filepath.Join(base, "evidence.md"), []byte("Evidence.\n"), 0o644); err != nil {
		t.Fatalf("write the attach source: %v", err)
	}
	chosen := map[string]string{
		"DINAH_ACTOR":     "alka",
		"DINAH_HOME":      filepath.Join(base, "home"),
		"DINAH_WORKBENCH": workbench,
		"DINAH_HARNESS":   "golden",
		"DINAH_PROVIDER":  "golden-provider",
		"DINAH_MODEL":     "golden-model",
		"DINAH_SERVER":    "golden.invalid",
	}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, "DINAH_") {
			continue
		}
		if _, kept := chosen[name]; kept {
			continue
		}
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	for name, value := range chosen {
		t.Setenv(name, value)
	}
	return &goldenFixture{base: base, workbench: workbench, roots: rootSpellings(t, base)}
}

// rootSpellings lists the directory, its absolute form and its form with
// symbolic links evaluated, deduplicated and longest first.
func rootSpellings(t *testing.T, dir string) []string {
	t.Helper()
	spellings := []string{dir}
	if abs, err := filepath.Abs(dir); err == nil {
		spellings = append(spellings, abs)
	}
	if evaluated, err := filepath.EvalSymlinks(dir); err == nil {
		spellings = append(spellings, evaluated)
	}
	seen := map[string]bool{}
	var unique []string
	for _, spelling := range spellings {
		if seen[spelling] {
			continue
		}
		seen[spelling] = true
		unique = append(unique, spelling)
	}
	sort.SliceStable(unique, func(i, j int) bool { return len(unique[i]) > len(unique[j]) })
	return unique
}

// goldenSequence is the fixed sequence of calls: one successful call to every
// tool the tools table serves, and the refusals section 14.1.1 of dinah-152's
// specification names. The order matters, because a later act reads what an
// earlier one wrote.
func goldenSequence(f *goldenFixture) []goldenCall {
	actor := func(arguments map[string]any) map[string]any {
		arguments["actor"] = "alka"
		return arguments
	}
	return []goldenCall{
		{tool: "add_card", arguments: actor(map[string]any{"title": "A card", "column": "build"})},
		{tool: "add_card", kase: "second", arguments: actor(map[string]any{"title": "A waiting card"})},
		{tool: "add_card", kase: "third", arguments: actor(map[string]any{"title": "A card to archive", "column": "build"})},
		{tool: "status", arguments: actor(map[string]any{})},
		{tool: "list", arguments: actor(map[string]any{"ref": "columns"})},
		{tool: "next_card", arguments: actor(map[string]any{})},
		{tool: "query", arguments: actor(map[string]any{"query": "column:build"})},
		{tool: "search_cards", arguments: actor(map[string]any{"phrase": "card"})},
		{tool: "tree", arguments: actor(map[string]any{})},
		{tool: "view", arguments: actor(map[string]any{})},
		{tool: "changes", arguments: actor(map[string]any{})},
		{tool: "whoami", arguments: actor(map[string]any{})},
		{tool: "prime", arguments: actor(map[string]any{})},
		{tool: "workbench", arguments: actor(map[string]any{})},
		{tool: "version", arguments: actor(map[string]any{})},
		{tool: "claim", arguments: actor(map[string]any{"card": "gd-1"})},
		{tool: "claim", kase: "unknown-card", arguments: actor(map[string]any{"card": "gd-99"})},
		{tool: "claim", kase: "held", arguments: map[string]any{"actor": "bryn", "card": "gd-1"}},
		{tool: "show", arguments: actor(map[string]any{"card": "gd-1"})},
		{tool: "instructions", arguments: actor(map[string]any{"card": "gd-1"})},
		{tool: "comment", arguments: actor(map[string]any{"card": "gd-1", "text": "A note."})},
		{tool: "accept_divergence", arguments: actor(map[string]any{"comment": "gd-1/comments/1"})},
		{tool: "attach", arguments: actor(map[string]any{"ref": "gd-1", "file": filepath.Join(f.base, "evidence.md")})},
		{tool: "rename", arguments: actor(map[string]any{"ref": "gd-1/attachments/1", "name": "renamed.md"})},
		{tool: "file_item", arguments: actor(map[string]any{"card": "gd-1", "kind": "acceptance_criterion", "text": "The first criterion."})},
		{tool: "file_item", kase: "second", arguments: actor(map[string]any{"card": "gd-1", "kind": "acceptance_criterion", "text": "The second criterion."})},
		{tool: "file_item", kase: "third", arguments: actor(map[string]any{"card": "gd-1", "kind": "acceptance_criterion", "text": "The third criterion."})},
		{tool: "file_item", kase: "fourth", arguments: actor(map[string]any{"card": "gd-1", "kind": "acceptance_criterion", "text": "The fourth criterion."})},
		{tool: "file_item", kase: "question", arguments: actor(map[string]any{"card": "gd-1", "kind": "open_question", "text": "A question?"})},
		{tool: "cite_item", arguments: actor(map[string]any{"item": "gd-1/criteria/1", "scheme": "note", "target": "evidence"})},
		{tool: "verify_item", arguments: actor(map[string]any{"item": "gd-1/criteria/1", "text": "Verified."})},
		{tool: "reopen_item", arguments: actor(map[string]any{"item": "gd-1/criteria/1", "reason": "Again."})},
		{tool: "settle", arguments: actor(map[string]any{"item": "gd-1/criteria/1", "state": "verified", "text": "Verified again."})},
		{tool: "fail_item", arguments: actor(map[string]any{"item": "gd-1/criteria/2", "text": "Failed."})},
		{tool: "waive_item", arguments: actor(map[string]any{"item": "gd-1/criteria/3", "text": "Waived."})},
		{tool: "withdraw_item", arguments: actor(map[string]any{"item": "gd-1/criteria/4", "text": "Withdrawn."})},
		{tool: "resolve_item", arguments: actor(map[string]any{"item": "gd-1/questions/1", "text": "Answered."})},
		{tool: "grant", arguments: actor(map[string]any{"card": "gd-1", "permission": verb.CriterionRetirement})},
		{tool: "revoke", arguments: actor(map[string]any{"card": "gd-1", "permission": verb.CriterionRetirement})},
		{tool: "workstream", arguments: actor(map[string]any{"action": "new", "workstream": "Golden stream", "slug": "golden"})},
		{tool: "join_workstream", arguments: actor(map[string]any{"card": "gd-1", "workstream": "golden"})},
		{tool: "leave_workstream", arguments: actor(map[string]any{"card": "gd-1", "workstream": "golden"})},
		{tool: "link_card", arguments: actor(map[string]any{"card": "gd-1", "kind": "relates_to", "to": "gd-2"})},
		{tool: "unlink_card", arguments: actor(map[string]any{"card": "gd-1", "kind": "relates_to", "to": "gd-2"})},
		{tool: "get_field", arguments: actor(map[string]any{"ref": "gd-1", "field": "title"})},
		{tool: "set_field", arguments: actor(map[string]any{"ref": "gd-1", "field": "title", "value": "A renamed card"})},
		{tool: "block", arguments: actor(map[string]any{"card": "gd-1", "reason": "An obstacle."})},
		{tool: "unblock", arguments: actor(map[string]any{"card": "gd-1", "reason": "Cleared."})},
		{tool: "claim", kase: "again", arguments: actor(map[string]any{"card": "gd-1"})},
		{tool: "raise", arguments: actor(map[string]any{"card": "gd-1", "tier": "apex", "reason": "It needs more."})},
		{tool: "claim", kase: "after-raise", arguments: actor(map[string]any{"card": "gd-1"})},
		{tool: "move", kase: "stale", arguments: actor(map[string]any{"card": "gd-1", "column": "review", "basis": "sha256:" + strings.Repeat("0", 64)})},
		{tool: "move", arguments: actor(map[string]any{"card": "gd-1", "column": "review"})},
		{tool: "release", arguments: actor(map[string]any{"card": "gd-1"})},
		{tool: "pull", arguments: actor(map[string]any{"column": "build"})},
		{tool: "new_column", arguments: actor(map[string]any{"column": "Parking", "kind": "work"})},
		{tool: "archive", arguments: actor(map[string]any{"ref": "gd-3"})},
		{tool: "restore", arguments: actor(map[string]any{"ref": "gd-3"})},
		{tool: "delete", arguments: actor(map[string]any{"ref": "gd-3", "yes": true, "force": true})},
		{tool: "changes", kase: "later", arguments: actor(map[string]any{})},
		{tool: "export", arguments: actor(map[string]any{})},
		{tool: "check", arguments: actor(map[string]any{})},
		{tool: "status", kase: "no-workbench", arguments: actor(map[string]any{"workbench": filepath.Join(f.base, "nowhere")})},
	}
}

// goldenSession is one MCP connection held open across the whole sequence,
// so the chain memory behaves as it does for a real client.
type goldenSession struct {
	// in feeds the head its request lines.
	in *io.PipeWriter
	// out reads the head's answer lines.
	out *bufio.Reader
	// done carries Serve's return once in closes.
	done chan error
	// next is the next request identifier.
	next int
}

// openGoldenSession starts the head in process on one connection.
func openGoldenSession(t *testing.T, f *goldenFixture) *goldenSession {
	t.Helper()
	opened, err := bench.Open(f.workbench)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	library := verb.New(opened, filepath.Join(f.base, "home"))
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	session := &goldenSession{in: inWriter, out: bufio.NewReader(outReader), done: make(chan error, 1), next: 1}
	go func() {
		err := Serve(f.base, library, map[string]*verb.Library{}, inReader, outWriter, ProfileAll)
		outWriter.Close()
		session.done <- err
	}()
	t.Cleanup(func() {
		inWriter.Close()
		<-session.done
	})
	return session
}

// call sends one tools/call and returns the text content of its answer.
func (s *goldenSession) call(t *testing.T, tool string, arguments map[string]any) string {
	t.Helper()
	line, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": s.next, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": arguments},
	})
	if err != nil {
		t.Fatalf("marshal the %s call: %v", tool, err)
	}
	s.next++
	if _, err := s.in.Write(append(line, '\n')); err != nil {
		t.Fatalf("send the %s call: %v", tool, err)
	}
	answerLine, err := s.out.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read the %s answer: %v", tool, err)
	}
	var answer struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	if err := json.Unmarshal(answerLine, &answer); err != nil {
		t.Fatalf("decode the %s answer %q: %v", tool, answerLine, err)
	}
	if answer.Error != nil {
		t.Fatalf("the %s call failed at the transport: %+v", tool, answer.Error)
	}
	if len(answer.Result.Content) != 1 {
		t.Fatalf("the %s call answered %d content blocks, wanted one", tool, len(answer.Result.Content))
	}
	return answer.Result.Content[0].Text
}

// runGoldenSequence makes every call of the sequence on one connection and
// returns the answers in call order.
func runGoldenSequence(t *testing.T, f *goldenFixture) []goldenAnswer {
	t.Helper()
	session := openGoldenSession(t, f)
	sequence := goldenSequence(f)
	answers := make([]goldenAnswer, 0, len(sequence))
	for index, call := range sequence {
		text := session.call(t, call.tool, call.arguments)
		answer := goldenAnswer{call: call, text: text}
		if _, excluded := goldenExclusions[call.tool]; !excluded {
			name := call.tool
			if call.kase != "" {
				name += "-" + call.kase
			}
			answer.file = fmt.Sprintf("%02d-%s.json", index+1, name)
		}
		answers = append(answers, answer)
	}
	return answers
}

// goldenRefusalCases are the calls in the sequence meant to be refused, each
// mapped to the outcome it must carry, so the goldens record the refusal the
// specification asks for and not an accident.
var goldenRefusalCases = map[string]string{
	"claim-unknown-card":  "refused",
	"claim-held":          "refused",
	"move-stale":          "stale",
	"status-no-workbench": "refused",
}

// goldenReportOutcomes are the calls whose successful answer is a report
// carrying an outcome of its own rather than ok. check reports the tier
// levels this fixture declares without a table, which is a finding and not a
// refusal.
var goldenReportOutcomes = map[string]string{
	"check": "findings",
}

// TestEveryToolAnswersItsGolden is dinah-152/criteria/7's byte-for-byte proof
// that moving the MCP head's answer code into a shared package changes no
// payload. The goldens were recorded on trunk's code before the move, and
// each call's text is compared with its golden after normaliseGolden masks
// the six classes that differ from run to run.
func TestEveryToolAnswersItsGolden(t *testing.T) {
	f := newGoldenFixture(t)
	answers := runGoldenSequence(t, f)

	keys := make([]string, 0, len(goldenExclusions))
	for key := range goldenExclusions {
		keys = append(keys, key)
	}
	if len(keys) != 1 || keys[0] != "version" {
		t.Fatalf("goldenExclusions holds %v; it must hold version alone, and a second exclusion needs its own argument", keys)
	}

	recorded := map[string]bool{}
	compared := 0
	if *updateGolden {
		if err := os.RemoveAll(goldenDir); err != nil {
			t.Fatalf("clear the goldens: %v", err)
		}
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatalf("make the golden directory: %v", err)
		}
	}
	for _, answer := range answers {
		name := answer.call.tool
		if answer.call.kase != "" {
			name += "-" + answer.call.kase
		}
		decoded := map[string]any{}
		if err := json.Unmarshal([]byte(answer.text), &decoded); err != nil {
			t.Errorf("%s answered text that is not a JSON object: %v\n%s", name, err, answer.text)
			continue
		}
		outcome, _ := decoded["outcome"].(string)
		if want, refusal := goldenRefusalCases[name]; refusal {
			if outcome != want {
				t.Errorf("%s answered outcome %q, and the sequence needs %q there:\n%s", name, outcome, want, answer.text)
			}
		} else if outcome != "" && outcome != "ok" && outcome != goldenReportOutcomes[name] {
			t.Errorf("%s answered outcome %q, and the sequence needs every call outside the refusal cases to succeed:\n%s", name, outcome, answer.text)
		}
		if answer.file == "" {
			continue
		}
		recorded[answer.call.tool] = true
		normalised := normaliseGolden(answer.text, f.roots)
		path := filepath.Join(goldenDir, answer.file)
		if *updateGolden {
			if err := os.WriteFile(path, []byte(normalised), 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read the golden %s: %v", path, err)
			continue
		}
		compared++
		if string(want) != normalised {
			t.Errorf("%s no longer answers its golden %s.\n%s", name, path, firstDifference(string(want), normalised))
		}
	}
	for _, entry := range tools {
		if recorded[entry.name] {
			continue
		}
		if _, excluded := goldenExclusions[entry.name]; excluded {
			continue
		}
		t.Errorf("the tool %s has neither a golden nor an entry in goldenExclusions; add a call to goldenSequence", entry.name)
	}
	if *updateGolden {
		t.Fatalf("rewrote the goldens under %s; run again without -update to compare", goldenDir)
	}
	if compared == 0 {
		t.Fatal("compared no golden, so this test read nothing")
	}
	t.Logf("compared %d goldens over %d tools, with %d excluded", compared, len(tools), len(goldenExclusions))
}

// firstDifference names the first line two texts disagree on, which is what
// a reader of a red run needs rather than two whole payloads.
func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return fmt.Sprintf("line %d:\n golden: %q\n    got: %q", i+1, w, g)
		}
	}
	return "the texts differ only in a way no line shows"
}

// The patterns normaliseGolden masks by.
var (
	// cursorMember matches a JSON member named cursor whose value is a string,
	// capturing the value between the quotes.
	cursorMember = regexp.MustCompile(`"cursor": "((?:[^"\\]|\\.)*)"`)
	// hexRun matches a maximal run of lowercase hex characters, which the
	// digest and identifier classes then choose among by length.
	hexRun = regexp.MustCompile(`[0-9a-f]+`)
	// rfc3339 matches an RFC 3339 timestamp.
	rfc3339 = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
)

// normaliseGolden masks the six classes of dinah-152's specification section
// 14.1.1, in its order, and nothing else: cursor values, the fixture root,
// backslashes inside a root span, the 64 hex characters of a digest,
// twelve-hex identifiers, and RFC 3339 timestamps. Each distinct value of a
// numbered class is numbered in order of first appearance within the text.
func normaliseGolden(text string, roots []string) string {
	cursors := map[string]int{}
	text = cursorMember.ReplaceAllStringFunc(text, func(member string) string {
		value := cursorMember.FindStringSubmatch(member)[1]
		n, seen := cursors[value]
		if !seen {
			n = len(cursors) + 1
			cursors[value] = n
		}
		return fmt.Sprintf(`"cursor": "<cursor-%d>"`, n)
	})
	for _, root := range roots {
		escaped, _ := json.Marshal(root)
		text = strings.ReplaceAll(text, string(escaped[1:len(escaped)-1]), "<root>")
		text = strings.ReplaceAll(text, root, "<root>")
	}
	text = foldRootSpans(text)
	text = maskHexRuns(text, 64, "hex")
	text = maskHexRuns(text, 12, "id")
	text = rfc3339.ReplaceAllString(text, "<ts>")
	return text
}

// foldRootSpans replaces every JSON-escaped backslash with a forward slash
// inside each span that begins with <root> and ends at the next unescaped
// double quote, and touches nothing outside those spans.
func foldRootSpans(text string) string {
	const marker = "<root>"
	var b strings.Builder
	for {
		at := strings.Index(text, marker)
		if at < 0 {
			b.WriteString(text)
			return b.String()
		}
		b.WriteString(text[:at+len(marker)])
		text = text[at+len(marker):]
		i := 0
		for i < len(text) {
			if text[i] == '"' {
				break
			}
			if text[i] == '\\' && i+1 < len(text) {
				if text[i+1] == '\\' {
					b.WriteByte('/')
				} else {
					b.WriteString(text[i : i+2])
				}
				i += 2
				continue
			}
			b.WriteByte(text[i])
			i++
		}
		text = text[i:]
	}
}

// maskHexRuns replaces every maximal run of exactly length lowercase hex
// characters with <word-N>, numbering distinct values in order of first
// appearance. A maximal run is bounded by non-hex characters on both sides,
// so a 64-character digest never yields a twelve-character identifier.
func maskHexRuns(text string, length int, word string) string {
	seen := map[string]int{}
	return hexRun.ReplaceAllStringFunc(text, func(run string) string {
		if len(run) != length {
			return run
		}
		n, known := seen[run]
		if !known {
			n = len(seen) + 1
			seen[run] = n
		}
		return fmt.Sprintf("<%s-%d>", word, n)
	})
}

// goldenEdit is one change TestGoldenNormaliserStillSeesChanges makes to a
// raw text, which must survive normalisation.
type goldenEdit struct {
	// name says what the edit does.
	name string
	// apply makes the edit, and reports false when the text gives it no
	// occasion.
	apply func(string) (string, bool)
}

// scalarMemberLine matches an indented member line carrying a scalar value
// and a trailing comma, which is a member that can be removed or swapped
// without leaving the JSON unbalanced.
var scalarMemberLine = regexp.MustCompile(`(?m)^( +)"[a-z_]+": (?:"[^"\n]*"|true|false|null|-?\d+),$`)

// plainValueMembers are members whose string values no class masks, so an
// edit to one is an edit the normaliser must let through.
var plainValueMembers = []string{`"outcome": "`, `"verb": "`, `"title": "`}

// goldenEdits are the ten edits of section 14.1.1, item 4.
var goldenEdits = []goldenEdit{
	{name: "a member renamed", apply: func(text string) (string, bool) {
		at := strings.Index(text, `"affordances":`)
		if at < 0 {
			return text, false
		}
		return text[:at] + `"affordancez":` + text[at+len(`"affordances":`):], true
	}},
	{name: "a member removed", apply: func(text string) (string, bool) {
		loc := scalarMemberLine.FindStringIndex(text)
		if loc == nil {
			return text, false
		}
		return text[:loc[0]] + strings.TrimPrefix(text[loc[1]:], "\n"), true
	}},
	{name: "two members swapped", apply: func(text string) (string, bool) {
		lines := strings.Split(text, "\n")
		for i := 0; i+1 < len(lines); i++ {
			if scalarMemberLine.MatchString(lines[i]) && scalarMemberLine.MatchString(lines[i+1]) {
				lines[i], lines[i+1] = lines[i+1], lines[i]
				return strings.Join(lines, "\n"), true
			}
		}
		return text, false
	}},
	{name: "one byte of indentation added", apply: func(text string) (string, bool) {
		at := strings.Index(text, "\n ")
		if at < 0 {
			return text, false
		}
		return text[:at+1] + " " + text[at+1:], true
	}},
	{name: "a non-masked string value changed by one character", apply: func(text string) (string, bool) {
		for _, member := range plainValueMembers {
			at := strings.Index(text, member)
			if at < 0 {
				continue
			}
			value := at + len(member)
			if value >= len(text) || text[value] == '"' {
				continue
			}
			swapped := byte('Q')
			if text[value] == 'Q' {
				swapped = 'R'
			}
			return text[:value] + string(swapped) + text[value+1:], true
		}
		return text, false
	}},
	{name: "a backslash inserted outside a root span", apply: func(text string) (string, bool) {
		for _, member := range plainValueMembers {
			at := strings.Index(text, member)
			if at < 0 {
				continue
			}
			value := at + len(member)
			return text[:value] + `\\` + text[value:], true
		}
		return text, false
	}},
	{name: "two distinct identifiers made equal", apply: func(text string) (string, bool) {
		return equaliseFirstTwo(text, runsOfLength(text, 12))
	}},
	{name: "two distinct cursors made equal", apply: func(text string) (string, bool) {
		var values []string
		for _, match := range cursorMember.FindAllStringSubmatch(text, -1) {
			values = append(values, match[1])
		}
		return equaliseFirstTwo(text, values)
	}},
	{name: "an affordance name changed", apply: func(text string) (string, bool) {
		at := strings.Index(text, `"affordances": [`)
		if at < 0 {
			return text, false
		}
		open := strings.Index(text[at:], `[`) + at
		quote := strings.Index(text[open:], `"`)
		if quote < 0 {
			return text, false
		}
		start := open + quote + 1
		return text[:start] + "x" + text[start:], true
	}},
	{name: "a digest's sha256: prefix removed", apply: func(text string) (string, bool) {
		at := strings.Index(text, "sha256:")
		if at < 0 {
			return text, false
		}
		return text[:at] + text[at+len("sha256:"):], true
	}},
}

// joinedChangeSets joins the raw texts of every changes call in the sequence,
// in order, one to a line.
func joinedChangeSets(answers []goldenAnswer) string {
	var texts []string
	for _, answer := range answers {
		if answer.call.tool == "changes" {
			texts = append(texts, answer.text)
		}
	}
	return strings.Join(texts, "\n")
}

// runsOfLength lists the maximal hex runs of one length, in order.
func runsOfLength(text string, length int) []string {
	var runs []string
	for _, run := range hexRun.FindAllString(text, -1) {
		if len(run) == length {
			runs = append(runs, run)
		}
	}
	return runs
}

// equaliseFirstTwo rewrites every occurrence of the second distinct value
// among values to the first, and reports false when there are not two.
func equaliseFirstTwo(text string, values []string) (string, bool) {
	if len(values) == 0 {
		return text, false
	}
	first := values[0]
	for _, value := range values[1:] {
		if value != first {
			return strings.ReplaceAll(text, value, first), true
		}
	}
	return text, false
}

// TestGoldenNormaliserStillSeesChanges is section 14.1.1, item 4: it proves
// the masking is not so wide that a real change hides behind it. It makes the
// golden sequence's calls, takes the raw text of one read, one act and one
// refusal, and applies each of the ten edits to each; an edit a chosen text
// gives no occasion for runs instead on the first text in the sequence that
// has one. Every edited text must normalise differently from its golden, and
// every unedited one exactly to it.
func TestGoldenNormaliserStillSeesChanges(t *testing.T) {
	f := newGoldenFixture(t)
	answers := runGoldenSequence(t, f)
	chosenNames := []string{"show", "claim", "claim-unknown-card"}
	var chosen []goldenAnswer
	for _, name := range chosenNames {
		for _, answer := range answers {
			label := answer.call.tool
			if answer.call.kase != "" {
				label += "-" + answer.call.kase
			}
			if label == name {
				chosen = append(chosen, answer)
				break
			}
		}
	}
	if len(chosen) != len(chosenNames) {
		t.Fatalf("found %d of the %d chosen texts in the sequence", len(chosen), len(chosenNames))
	}
	golden := func(answer goldenAnswer) string {
		t.Helper()
		want, err := os.ReadFile(filepath.Join(goldenDir, answer.file))
		if err != nil {
			t.Fatalf("read the golden %s: %v", answer.file, err)
		}
		return string(want)
	}
	for _, answer := range chosen {
		if got := normaliseGolden(answer.text, f.roots); got != golden(answer) {
			t.Errorf("the unedited %s does not normalise to its golden.\n%s", answer.file, firstDifference(golden(answer), got))
		}
	}
	checked := 0
	for _, edit := range goldenEdits {
		applied := false
		for _, answer := range chosen {
			edited, ok := edit.apply(answer.text)
			if !ok {
				continue
			}
			applied = true
			checked++
			if normaliseGolden(edited, f.roots) == golden(answer) {
				t.Errorf("%s in %s normalises to the unedited golden, so the masking hides it", edit.name, answer.file)
			}
		}
		if applied {
			continue
		}
		for _, answer := range answers {
			if answer.file == "" {
				continue
			}
			edited, ok := edit.apply(answer.text)
			if !ok {
				continue
			}
			applied = true
			checked++
			if normaliseGolden(edited, f.roots) == golden(answer) {
				t.Errorf("%s in %s normalises to the unedited golden, so the masking hides it", edit.name, answer.file)
			}
			break
		}
		if applied {
			continue
		}
		// No payload the head answers carries two cursors: a change set
		// carries one, and a root-scoped change set carries one for its whole
		// forest. The edit that makes two distinct cursors equal therefore
		// runs on the two change-set texts of the sequence joined into one,
		// held against that joined text's own normalisation, which is what
		// its golden would be. That still proves what the edit is for: the
		// numbering of the cursor class tells two cursors from one.
		joined := joinedChangeSets(answers)
		if edited, ok := edit.apply(joined); ok {
			applied = true
			checked++
			if normaliseGolden(edited, f.roots) == normaliseGolden(joined, f.roots) {
				t.Errorf("%s in the joined change sets normalises to the unedited text, so the masking hides it", edit.name)
			}
		}
		if !applied {
			t.Errorf("no text in the sequence gives %s an occasion, so that edit was never checked", edit.name)
		}
	}
	if checked < len(goldenEdits) {
		t.Fatalf("checked %d edits, fewer than the %d kinds", checked, len(goldenEdits))
	}
	t.Logf("checked %d edits of %d kinds", checked, len(goldenEdits))
}
