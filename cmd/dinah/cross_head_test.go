package main

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/mcp"
	"dinah/internal/verb"
)

// crossHeadQueries are the query strings the declared readers are driven with.
// One matches every card, one matches a card's state, and one matches nothing,
// so the comparison sees a populated answer and an empty one.
var crossHeadQueries = []string{"", "state:ready", "holder:nobody"}

// crossHeadCase is one invocation a declared command is compared on: the free
// text its rest slot carries, and a value per named parameter.
type crossHeadCase struct {
	// text fills the command's rest slot, and is empty for a command that
	// has none.
	text string
	// values fills named parameters, keyed by the parameter's own name, and
	// is empty for a command driven bare.
	values map[string]string
}

// TestBothHeadsAnswerTheDeclaredReadsAlike asserts that every command declared
// cross-head identical answers a terminal invocation under --json and an mcp
// tool call with the same payload.
//
// Presence is not agreement. The two layers above prove a command reaches both
// heads and that each head reads the arguments it advertises, and a command
// could satisfy both and still answer differently on one head, which is drift
// neither layer can see. The design says the JSON contract is the frozen
// surface and each head is a thin mapping of it, so this tests the sentence
// rather than trusting it.
//
// The covered set is verb.CrossHeadIdentical and nothing else. Adding a
// command to that map is what widens this test, so there is no second list
// here to fall behind the first.
func TestBothHeadsAnswerTheDeclaredReadsAlike(t *testing.T) {
	declared := verb.CrossHeadIdentical()
	if len(declared) == 0 {
		t.Fatal("no command is declared cross-head identical, so this check read nothing")
	}
	names := make([]string, 0, len(declared))
	for name := range declared {
		names = append(names, name)
	}
	sort.Strings(names)

	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "Another card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	// The cursor is minted before the bench moves, so that the checkpoint the
	// changes leg reads back has journal lines to report. A cursor taken from
	// a bench that then sits still answers that nothing changed, and two heads
	// agreeing on that agree on a false flag and a token rather than on a
	// payload.
	cursor := mintCursor(t, root)
	if got := runCLI(t, root, "add", "A third card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")

	compared := 0
	for _, name := range names {
		tool := mcp.ToolNameFor(name)
		if tool == "" {
			t.Errorf("%s is declared cross-head identical and the mcp head serves no tool for it", name)
			continue
		}
		populated := false
		for _, sample := range crossHeadCases(name, cursor) {
			compared++
			terminal := terminalPayload(t, root, name, sample)
			protocol := toolPayload(t, root, tool, name, sample)
			// Whether an answer carried content is a question about the
			// fixture rather than about agreement, so it is read off the
			// terminal payload before the two are compared. Reading it after
			// a failed comparison would report an answer that carried plenty
			// as one that carried nothing.
			if carriesAList(terminal) {
				populated = true
			}
			if !reflect.DeepEqual(terminal, protocol) {
				t.Errorf("%s %s answers differently on the two heads:\n terminal: %s\n     tool: %s",
					name, sample.describe(), mustEncode(t, terminal), mustEncode(t, protocol))
			}
		}
		if !populated {
			t.Errorf("every %s invocation compared answered with no populated list, so the two heads were held against an answer carrying none of this command's own content", name)
		}
	}
	if compared == 0 {
		t.Fatal("no declared command was driven through either head, so this check read nothing")
	}
}

// mintCursor takes a checkpoint at the terminal and returns the cursor it
// answers with, which is the token a later call hands back to ask what moved.
func mintCursor(t *testing.T, root string) string {
	t.Helper()
	got := runCLI(t, root, "changes", "--json")
	if got.code != 0 {
		t.Fatalf("changes to mint a cursor: %d %s", got.code, got.errw)
	}
	object, ok := decode(t, got.out).(map[string]any)
	if !ok {
		t.Fatalf("changes answered %s, which is not an object to read a cursor out of", got.out)
	}
	cursor, ok := object["cursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("changes answered no cursor to check back against: %s", got.out)
	}
	return cursor
}

// carriesAList reports whether a payload holds a populated array anywhere
// inside it, which is this test's measure of an answer with content in it.
//
// Every array a read answers with is omitempty, so an answer that found
// nothing is one whose substance is absent from both sides, and an equality
// asserted on what is left compares a flag and a token. The measure is
// structural rather than a declared member per command, because a declared
// member would be one more list to keep in step with the readers.
func carriesAList(payload any) bool {
	switch value := payload.(type) {
	case []any:
		return len(value) > 0
	case map[string]any:
		for _, member := range value {
			if carriesAList(member) {
				return true
			}
		}
	}
	return false
}

// crossHeadCases returns the invocations one declared command is compared on.
//
// A command whose parameter list carries a rest slot is driven once per query
// string, so the comparison sees a populated answer and an empty one. changes
// carries no rest slot and answers a checkpoint rather than a search, so it is
// driven with the cursor minted before the fixture moved, which is what puts
// journal lines in the answer the two heads are held against.
//
// Nothing beyond those is filled. Supplying a value for every declared flag
// would need a legal value per vocabulary and is left to a later pass, so a
// command added to the declared map is compared on its bare invocation rather
// than not compared at all. An addition whose bare invocation answers nothing
// fails the populated check above, which is where somebody is told to give it
// a fixture of its own.
func crossHeadCases(command, cursor string) []crossHeadCase {
	if command == "changes" {
		return []crossHeadCase{{values: map[string]string{"since": cursor}}}
	}
	for _, param := range verb.Params(command) {
		if param.Rest {
			cases := make([]crossHeadCase, 0, len(crossHeadQueries))
			for _, text := range crossHeadQueries {
				cases = append(cases, crossHeadCase{text: text})
			}
			return cases
		}
	}
	return []crossHeadCase{{}}
}

// describe renders one case for a failing run's own report.
func (c crossHeadCase) describe() string {
	parts := []string{}
	if c.text != "" {
		parts = append(parts, strconv.Quote(c.text))
	}
	for _, name := range sortedKeys(c.values) {
		parts = append(parts, "--"+name+" "+strconv.Quote(c.values[name]))
	}
	if len(parts) == 0 {
		return "with no arguments"
	}
	return strings.Join(parts, " ")
}

// terminalPayload runs one command at the terminal under --json and returns
// its decoded answer.
func terminalPayload(t *testing.T, root, command string, sample crossHeadCase) any {
	t.Helper()
	argv := []string{command}
	if sample.text != "" {
		argv = append(argv, sample.text)
	}
	for _, name := range sortedKeys(sample.values) {
		argv = append(argv, "--"+name, sample.values[name])
	}
	argv = append(argv, "--json")
	got := runCLI(t, root, argv...)
	if got.code != 0 {
		t.Fatalf("%s %s at the terminal: %d %s", command, sample.describe(), got.code, got.errw)
	}
	return stripAffordances(decode(t, got.out))
}

// toolPayload drives the same read through the mcp head over a pipe and
// returns the decoded answer the tool call carried, with the transport's own
// content envelope unwrapped.
func toolPayload(t *testing.T, root, tool, command string, sample crossHeadCase) any {
	t.Helper()
	dir := soleBenchDir(t, root)
	opened, err := bench.Open(dir)
	if err != nil {
		t.Fatalf("open %q: %v", dir, err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	arguments := map[string]any{"actor": os.Getenv("DINAH_ACTOR")}
	for _, param := range verb.Params(command) {
		if param.Rest && sample.text != "" {
			arguments[param.Name] = sample.text
		}
	}
	for name, value := range sample.values {
		arguments[name] = value
	}
	call := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": arguments},
	}
	line, err := json.Marshal(call)
	if err != nil {
		t.Fatalf("marshal the call: %v", err)
	}
	out := &strings.Builder{}
	if err := mcp.Serve(dir, library, map[string]*verb.Library{}, strings.NewReader(string(line)+"\n"), out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var answer struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &answer); err != nil {
		t.Fatalf("%s %s over the protocol: %v (%s)", tool, sample.describe(), err, out.String())
	}
	if len(answer.Error) > 0 {
		t.Fatalf("%s %s over the protocol answered an error: %s", tool, sample.describe(), answer.Error)
	}
	if len(answer.Result.Content) != 1 {
		t.Fatalf("%s %s carried %d content members", tool, sample.describe(), len(answer.Result.Content))
	}
	payload := stripAffordances(decode(t, answer.Result.Content[0].Text))
	return unwrapDeclared(t, command, tool, payload)
}

// decode reads one payload as JSON.
func decode(t *testing.T, text string) any {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatalf("decode %q: %v", text, err)
	}
	return value
}

// stripAffordances removes the member carrying a head's own next-step hints,
// which tells a caller what to reach for next rather than answering what it
// asked. It is dropped on both sides, since the mcp head adds one around every
// wrapped read and the library puts one inside a ChangeSet.
func stripAffordances(payload any) any {
	object, ok := payload.(map[string]any)
	if !ok {
		return payload
	}
	stripped := make(map[string]any, len(object))
	for member, value := range object {
		if member == "affordances" {
			continue
		}
		stripped[member] = value
	}
	return stripped
}

// unwrapDeclared takes the mcp head's answer down to the object the terminal
// prints, and holds the member it was published under against the name the
// surface declares for that command.
//
// The member's name is inside the comparison rather than canonicalised out of
// it. An earlier form of this test unwrapped any single-member object on both
// sides, which put the name outside the comparison entirely: renaming the tree
// tool's wrapper to hierarchy left the whole suite green while an agent
// decoding the payload and looking up tree got nothing. The name is what a
// caller reads the answer by, so it is part of the payload rather than part of
// the envelope around it.
//
// A command the surface declares no wrapper for is returned as it stands, and
// that declaration is proved rather than trusted. A head that began wrapping
// such an answer would hand back an object the terminal never prints, and the
// comparison the caller makes fails on it.
func unwrapDeclared(t *testing.T, command, tool string, payload any) any {
	t.Helper()
	declared := mcp.WrapperMemberFor(command)
	if declared == "" {
		return payload
	}
	object, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("the %s tool declares the wrapping member %q and answered %s, which is no object to read that member out of", tool, declared, mustEncode(t, payload))
	}
	inner, ok := object[declared]
	if !ok {
		t.Fatalf("the %s tool publishes its answer under %q and this surface declares %q, so a caller reading the declared member gets nothing", tool, strings.Join(memberNames(object), `", "`), declared)
	}
	if len(object) != 1 {
		t.Fatalf("the %s tool declares the single wrapping member %q and answered the members %q beside its affordances", tool, declared, memberNames(object))
	}
	return inner
}

// memberNames lists an object's members in order, for a failing run to report.
func memberNames(object map[string]any) []string {
	names := make([]string, 0, len(object))
	for name := range object {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// mustEncode renders a payload for a failing run's own report.
func mustEncode(t *testing.T, payload any) string {
	t.Helper()
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal for the report: %v", err)
	}
	return string(encoded)
}
