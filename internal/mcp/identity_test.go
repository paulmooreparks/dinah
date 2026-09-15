package mcp

import (
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// TestAPerCallDeclarationOutranksTheProcessEnvironmentMemberByMember drives
// dinah-496's MCP resolution criterion.
//
// An orchestrator and the subagent it dispatches share one server process, so
// the process environment is the connection's default and the call is where a
// particular model says what it is. The resolution is per member rather than
// all-or-nothing: a call naming only model is stamped with that model and with
// the process environment's harness, provider and server, and a call naming
// none of the four is stamped with all four from the environment.
func TestAPerCallDeclarationOutranksTheProcessEnvironmentMemberByMember(t *testing.T) {
	t.Setenv("DINAH_HARNESS", "server-harness")
	t.Setenv("DINAH_PROVIDER", "server-provider")
	t.Setenv("DINAH_MODEL", "server-model")
	t.Setenv("DINAH_SERVER", "server.example")

	cases := []struct {
		name      string
		arguments map[string]any
		want      bench.Actor
	}{
		{
			name:      "the call names nothing",
			arguments: map[string]any{"actor": "alka"},
			want: bench.Actor{
				Name: "alka", Harness: "server-harness", Provider: "server-provider",
				Model: "server-model", Server: "server.example",
			},
		},
		{
			name:      "the call names only the model",
			arguments: map[string]any{"actor": "alka", "model": "call-model"},
			want: bench.Actor{
				Name: "alka", Harness: "server-harness", Provider: "server-provider",
				Model: "call-model", Server: "server.example",
			},
		},
		{
			name: "the call names all four",
			arguments: map[string]any{
				"actor": "alka", "harness": "call-harness", "provider": "call-provider",
				"model": "call-model", "server": "call.example",
			},
			want: bench.Actor{
				Name: "alka", Harness: "call-harness", Provider: "call-provider",
				Model: "call-model", Server: "call.example",
			},
		},
	}
	for at, want := range cases {
		t.Run(want.name, func(t *testing.T) {
			library := newLibrary(t)
			answer := ask(t, library, callLine(t, at+1, "claim", withCard(t, library, want.arguments)))
			if answer.Error != nil {
				t.Fatalf("claim: %+v", answer.Error)
			}
			if outcome, _ := payload(t, answer)["outcome"].(string); outcome != "ok" {
				t.Fatalf("claim answered %q, wanted ok: %+v", outcome, payload(t, answer))
			}
			events := claimedEvents(t, library)
			if len(events) != 1 {
				t.Fatalf("the card carries %d claimed lines, wanted 1", len(events))
			}
			if events[0].Actor != want.want {
				t.Errorf("the line carries %+v, wanted %+v", events[0].Actor, want.want)
			}
		})
	}
}

// withCard copies a call's arguments and adds the card the fixture stands
// ready, so each case names one card without repeating its reference.
func withCard(t *testing.T, library *verb.Library, arguments map[string]any) map[string]any {
	t.Helper()
	call := map[string]any{"card": "fx-1"}
	for name, value := range arguments {
		call[name] = value
	}
	return call
}

// claimedEvents reads the claimed lines off the fixture's one card.
func claimedEvents(t *testing.T, library *verb.Library) []bench.Event {
	t.Helper()
	found, err := library.Bench.ResolveCard("fx-1")
	if err != nil {
		t.Fatalf("resolve the card: %v", err)
	}
	events, _, err := bench.ReadJournal(found.Card.JournalPath())
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	var claimed []bench.Event
	for _, event := range events {
		if event.Event == "claimed" {
			claimed = append(claimed, event)
		}
	}
	return claimed
}
