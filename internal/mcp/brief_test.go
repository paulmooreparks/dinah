package mcp

import (
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestTheShowToolServesTheBrief asserts that the brief argument reaches the
// library's own selection through this head, on the payload an agent reads:
// the answer carries the handoff and the checklist and leaves the comments
// out. The fixture card was moved once before anybody wrote on it, so a
// comment is written and the card moved again for a handoff to exist, and a
// verified item stands beside the pending one so the narrowing has something
// to drop.
func TestTheShowToolServesTheBrief(t *testing.T) {
	library := newLibrary(t)
	if response := library.Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: "fx-1", Text: "## HANDOFF\n\nRead this first.\n"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %s %s", response.Outcome, response.Refusal)
	}
	if moved := library.Do(&verb.Request{Verb: verb.Move, Actor: "alka", Card: "fx-1", Column: "intake"}); moved.Outcome != contract.OutcomeOK {
		t.Fatalf("move: %s %s", moved.Outcome, moved.Refusal)
	}
	plantItem(t, library, "fx-1", "b00000000001",
		"kind: acceptance_criterion\nstate: pending\nordinal: 1\n", "Still to verify.")
	plantItem(t, library, "fx-1", "b00000000002",
		"kind: acceptance_criterion\nstate: verified\nordinal: 2\n", "Already verified.")
	opened, err := bench.Open(library.Bench.Root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	library.Bench = opened

	answer := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"show","arguments":{"actor":"alka","card":"fx-1","brief":true}}}`))
	detail, ok := answer["detail"].(map[string]any)
	if !ok {
		t.Fatalf("the show tool carried no detail: %v", answer)
	}
	handoff, ok := detail["handoff"].([]any)
	if !ok || len(handoff) != 1 {
		t.Fatalf("wanted the one handoff comment, got %v", detail["handoff"])
	}
	entry, _ := handoff[0].(map[string]any)
	if entry["subject"] != "HANDOFF" || entry["body"] == "" {
		t.Errorf("wanted the handoff comment in full, got %v", entry)
	}
	if _, carried := detail["comments"]; carried {
		t.Errorf("the brief carries the comments, which a station asks for when it needs them: %v", detail["comments"])
	}
	checklist, ok := detail["checklist"].([]any)
	if !ok || len(checklist) != 1 {
		t.Fatalf("wanted the one pending item alone, got %v", detail["checklist"])
	}
	if item, _ := checklist[0].(map[string]any); item["state"] != "pending" {
		t.Errorf("wanted the pending item, got %v", item)
	}
}
