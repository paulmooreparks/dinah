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

// TestTheStatesToolCarriesTakesWorkUp is dinah-273 AC-26, on the pattern the
// test above establishes. A member an agent parses is a member it depends on,
// so the spelling on the wire is asserted by name rather than through the Go
// field, and the false answer is the one that matters: an agent reading it is
// told to reach for a pull rather than meet a refusal it was not warned about.
func TestTheStatesToolCarriesTakesWorkUp(t *testing.T) {
	library := newLibrary(t)

	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"states","arguments":{"actor":"alka"}}}`)
	decoded := payload(t, answer)
	raw, err := json.Marshal(decoded["states"])
	if err != nil {
		t.Fatalf("re-encode the states member: %v", err)
	}
	if !strings.Contains(string(raw), `"takes_work_up"`) {
		t.Errorf("the member is not spelled takes_work_up on the wire: %s", raw)
	}
	var states []verb.StateView
	if err := json.Unmarshal(raw, &states); err != nil {
		t.Fatalf("decode the states member: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("wanted the fixture's two states, got %d", len(states))
	}
	if states[0].TakesWorkUp {
		t.Error("no owner takes work up at an intake state, and the intake state came back true")
	}
	if !states[1].TakesWorkUp {
		t.Error("an owner takes work up at a work state, and the work state came back false")
	}
	// The member carries no omitempty, so the false answer survives the
	// encoding rather than being dropped as a zero value.
	if !strings.Contains(string(raw), `"takes_work_up":false`) {
		t.Errorf("the false answer was dropped from the wire form: %s", raw)
	}
}

// TestTheNextCardToolOffersAPull is dinah-273 AC-37's affordance half. An agent
// offered a card from a state where no owner takes work up cannot claim it, so
// the tool that would take it has to be among the acts the answer names.
func TestTheNextCardToolOffersAPull(t *testing.T) {
	library := newLibrary(t)
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"next_card","arguments":{"actor":"alka"}}}`)
	decoded := payload(t, answer)
	raw, err := json.Marshal(decoded["affordances"])
	if err != nil {
		t.Fatalf("re-encode the affordances member: %v", err)
	}
	var affordances []string
	if err := json.Unmarshal(raw, &affordances); err != nil {
		t.Fatalf("decode the affordances member: %v", err)
	}
	for _, name := range []string{"claim", "pull", "show", "log"} {
		found := false
		for _, offered := range affordances {
			if offered == name {
				found = true
			}
		}
		if !found {
			t.Errorf("the affordances are %v and %s is not among them", affordances, name)
		}
	}
}
