package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// TestTheStatesToolCarriesTheWaitingFlag is the MCP half of dinah-201 AC-8. An
// agent orienting itself reads the states object, and the flag has to be there
// or the agent learns the station is unavailable only by being refused.
//
// The Owner half is the guard the criterion asks for: awaiting_outside and
// operator_owned answer different questions, and an implementation collapsing
// them would tell every agent that a waiting state is the operator's, which is
// the confusion this card removes.
func TestTheStatesToolCarriesTheWaitingFlag(t *testing.T) {
	library := newLibrary(t)
	declareWaiting(t, library, "a00000000002")

	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"states","arguments":{"actor":"alka"}}}`)
	decoded := payload(t, answer)
	raw, err := json.Marshal(decoded["states"])
	if err != nil {
		t.Fatalf("re-encode the states member: %v", err)
	}
	var states []verb.StateView
	if err := json.Unmarshal(raw, &states); err != nil {
		t.Fatalf("decode the states member: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("wanted the fixture's two states, got %d", len(states))
	}
	if states[0].AwaitingOutside {
		t.Error("the intake state came back carrying the flag")
	}
	if !states[1].AwaitingOutside {
		t.Error("the waiting state came back without the flag")
	}
	if states[1].OperatorOwned {
		t.Error("a waiting state is not thereby operator-owned")
	}
	if !strings.Contains(string(raw), `"awaiting_outside"`) {
		t.Errorf("the member is not spelled awaiting_outside on the wire: %s", raw)
	}
}

// declareWaiting writes the flag into a state's own anchor and reopens the
// library's workbench, which is how this test reaches a state property no tool
// of the surface sets.
func declareWaiting(t *testing.T, library *verb.Library, id string) {
	t.Helper()
	path := filepath.Join(library.Bench.Root, bench.StatesDir, id, bench.StateAnchor)
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the state anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(string(text))
	fm.Set("awaiting_outside", "true")
	if err := os.WriteFile(path, []byte(fm.Render(body)), 0o644); err != nil {
		t.Fatalf("write the state anchor: %v", err)
	}
	reopened, err := bench.Open(library.Bench.Root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	library.Bench = reopened
}
