package main

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/mcp"
	"dinah/internal/verb"
)

// crossHeadQueries are the query strings the declared readers are driven with.
// One matches every card, one matches a substate, and one matches nothing, so
// the comparison sees a populated answer and an empty one.
var crossHeadQueries = []string{"", "substate:ready", "holder:nobody"}

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
	compared := 0
	for _, name := range names {
		tool := mcp.ToolNameFor(name)
		if tool == "" {
			t.Errorf("%s is declared cross-head identical and the mcp head serves no tool for it", name)
			continue
		}
		for _, text := range crossHeadArguments(name) {
			compared++
			terminal := terminalPayload(t, root, name, text)
			protocol := toolPayload(t, root, tool, name, text)
			if !reflect.DeepEqual(terminal, protocol) {
				t.Errorf("%s %q answers differently on the two heads:\n terminal: %s\n     tool: %s",
					name, text, mustEncode(t, terminal), mustEncode(t, protocol))
			}
		}
	}
	if compared == 0 {
		t.Fatal("no declared command was driven through either head, so this check read nothing")
	}
}

// crossHeadArguments returns the free-text values a command is driven with. A
// command whose parameter list carries a rest slot is driven once per query
// string; one that carries none is driven once with nothing, which is what
// changes takes.
//
// Only the rest slot is filled. Supplying a value for every declared flag would
// need a legal value per vocabulary and is left to a later pass, so a command
// added to the declared map is compared on its bare invocation rather than not
// compared at all.
func crossHeadArguments(command string) []string {
	for _, param := range verb.Params(command) {
		if param.Rest {
			return crossHeadQueries
		}
	}
	return []string{""}
}

// terminalPayload runs one command at the terminal under --json and returns
// its decoded answer.
func terminalPayload(t *testing.T, root, command, text string) any {
	t.Helper()
	argv := []string{command}
	if text != "" {
		argv = append(argv, text)
	}
	argv = append(argv, "--json")
	got := runCLI(t, root, argv...)
	if got.code != 0 {
		t.Fatalf("%s %q at the terminal: %d %s", command, text, got.code, got.errw)
	}
	return canonical(t, decode(t, got.out))
}

// toolPayload drives the same read through the mcp head over a pipe and
// returns the decoded answer the tool call carried, with the transport's own
// content envelope unwrapped.
func toolPayload(t *testing.T, root, tool, command, text string) any {
	t.Helper()
	dir := soleBenchDir(t, root)
	opened, err := bench.Open(dir)
	if err != nil {
		t.Fatalf("open %q: %v", dir, err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	arguments := map[string]any{"actor": os.Getenv("DINAH_ACTOR")}
	for _, param := range verb.Params(command) {
		if param.Rest && text != "" {
			arguments[param.Name] = text
		}
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
		t.Fatalf("%s %q over the protocol: %v (%s)", tool, text, err, out.String())
	}
	if len(answer.Error) > 0 {
		t.Fatalf("%s %q over the protocol answered an error: %s", tool, text, answer.Error)
	}
	if len(answer.Result.Content) != 1 {
		t.Fatalf("%s %q carried %d content members", tool, text, len(answer.Result.Content))
	}
	return canonical(t, decode(t, answer.Result.Content[0].Text))
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

// canonical strips the two members that belong to a head rather than to the
// answer, so what is left is the payload both heads claim to carry.
//
// The mcp head adds an affordances member to every tool response, which the
// terminal does not print, and it wraps a read's own object under a single
// member named for it. Neither is drift, and every other difference is. A
// payload that genuinely carried one member and no affordances would be
// unwrapped here by mistake, and the comparison would then fail loudly rather
// than pass quietly, which is the direction a guard should fail in.
func canonical(t *testing.T, payload any) any {
	t.Helper()
	object, ok := payload.(map[string]any)
	if !ok {
		return payload
	}
	stripped := map[string]any{}
	for member, value := range object {
		if member == "affordances" {
			continue
		}
		stripped[member] = value
	}
	if len(stripped) != 1 {
		return stripped
	}
	for _, only := range stripped {
		return only
	}
	return stripped
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
