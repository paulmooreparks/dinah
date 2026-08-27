package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/mcp"
	"dinah/internal/verb"
)

// rootToolFor pairs each root-scoped read with the tool this surface serves it
// as and the member that tool publishes its answer under. The tool name is read
// off the mcp head's own roster rather than typed here, so a tool renamed there
// fails this rather than quietly stopping being compared.
var rootToolFor = map[string]string{
	"tree":    "forest",
	"status":  "root_status",
	"ls":      "root_listing",
	"next":    "root_offers",
	"changes": "root_changes",
}

// serveForest drives one tool call against a server bounded by the forest root
// and returns the decoded payload with the transport envelope unwrapped.
func serveForest(t *testing.T, root, tool string, arguments map[string]any) map[string]any {
	t.Helper()
	// The server carries a default library the way a real one does, which is
	// the workbench discovery resolved at startup. The root-scoped call reaches
	// past it, and the home it carries is what each opened workbench inherits.
	first := firstWorkbenchUnder(t, root)
	opened, err := bench.Open(first)
	if err != nil {
		t.Fatalf("open %s: %v", first, err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	line, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": arguments},
	})
	if err != nil {
		t.Fatalf("marshal the call: %v", err)
	}
	out := &strings.Builder{}
	if err := mcp.Serve(root, library, map[string]*verb.Library{}, strings.NewReader(string(line)+"\n"), out); err != nil {
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
		t.Fatalf("%s over the protocol: %v (%s)", tool, err, out.String())
	}
	if len(answer.Error) > 0 {
		t.Fatalf("%s over the protocol answered an error: %s", tool, answer.Error)
	}
	if len(answer.Result.Content) != 1 {
		t.Fatalf("%s carried %d content members", tool, len(answer.Result.Content))
	}
	decoded, ok := decode(t, answer.Result.Content[0].Text).(map[string]any)
	if !ok {
		t.Fatalf("%s answered something that is not an object: %s", tool, answer.Result.Content[0].Text)
	}
	delete(decoded, "affordances")
	return decoded
}

// firstWorkbenchUnder returns one workbench beneath the root, which is what a
// server started there would have resolved at startup.
func firstWorkbenchUnder(t *testing.T, root string) string {
	t.Helper()
	listed, err := bench.EnumerateDeep(root, 0)
	if err != nil || len(listed) == 0 {
		t.Fatalf("no workbench beneath %s: %v", root, err)
	}
	return listed[0].Path
}

// TestBothHeadsAnswerARootScopedReadAlike asserts dinah-281 AC-7: for each of
// the five reads, one fixture root driven through the terminal under --json and
// through the matching tool's root argument answers the same payload, modulo
// the transport envelope.
//
// This is the check the argument-coverage guards cannot make. Those prove each
// head reads the parameters it advertises; two heads can both read a root and
// still answer differently, and a root-scoped read routes through one builder
// on purpose, so what is tested here is that the routing really is shared
// rather than two paths that happen to agree today.
func TestBothHeadsAnswerARootScopedReadAlike(t *testing.T) {
	// The fixture holds a sibling whose name sorts between a directory and what
	// lies inside it: two-extra sorts before two/three, and a depth-first walk
	// reaches it after. A listing ordered by the recursion and one ordered by
	// path therefore disagree here, which is what makes this fixture able to
	// catch two heads ordering one answer two ways.
	root := newForest(t, "alpha", "two/three", "two-extra")
	compared := 0
	for command, member := range rootToolFor {
		tool := mcp.ToolNameFor(command)
		if tool == "" {
			t.Errorf("the mcp head serves no tool for %s", command)
			continue
		}
		t.Run(command, func(t *testing.T) {
			terminal := forestJSON(t, root, command, "--root", root)
			served := serveForest(t, root, tool, map[string]any{"actor": "alka", "root": root})
			wrapped, named := served[member]
			if !named {
				t.Fatalf("the tool published no %s member: %v", member, served)
			}
			if !reflect.DeepEqual(terminal, wrapped) {
				t.Errorf("the two heads answer %s differently:\nterminal: %s\ntool:     %s",
					command, mustJSON(t, terminal), mustJSON(t, wrapped))
			}
		})
		compared++
	}
	if compared != 5 {
		t.Errorf("compared %d reads, wanted the five a refresh needs", compared)
	}
}

// TestBothHeadsAnswerTheWorkbenchesWalkAlike asserts dinah-281 AC-14: the
// terminal's positional path and the tool's path argument, given the same
// fixture and the same depth, answer the same listing.
func TestBothHeadsAnswerTheWorkbenchesWalkAlike(t *testing.T) {
	root := newForest(t, "one", "two/three", "two/four/five", "two-extra")
	for _, depth := range []string{"1", "2", "0"} {
		t.Run("at depth "+depth, func(t *testing.T) {
			got := runCLI(t, root, "workbenches", root, "--max-depth", depth, "--json")
			if got.code != 0 {
				t.Fatalf("the terminal: %d %s", got.code, got.errw)
			}
			served := serveForest(t, root, "workbenches", map[string]any{"path": root, "max-depth": depth})
			listed, named := served["workbenches"]
			if !named {
				t.Fatalf("the tool published no workbenches member: %v", served)
			}
			if !reflect.DeepEqual(decode(t, got.out), listed) {
				t.Errorf("the two heads answer the walk differently:\nterminal: %s\ntool:     %s",
					strings.TrimSpace(got.out), mustJSON(t, listed))
			}
		})
	}
}
