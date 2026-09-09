package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// TestTheShowToolCarriesTheChecklistTheCliCarries is dinah-435 AC-7. The MCP
// head wraps the library's own Detail rather than composing an answer of its
// own, and this holds that to its word for the member this card adds: the
// checklist an agent reads under detail.checklist is compared against the
// checklist the same library answers `dinah show --json` with, entry for
// entry, as JSON rather than as a Go value, since JSON is what both heads
// actually hand out.
//
// Comparing the two encodings rather than reading fields off one of them is
// the point. A head that dropped the member, or that renamed a key on its way
// through, or that carried an empty array where the other carried none, would
// pass every assertion made against a single side.
func TestTheShowToolCarriesTheChecklistTheCliCarries(t *testing.T) {
	library := newLibrary(t)
	plantItem(t, library, "fx-1", "b00000000001",
		"kind: open_question\nstate: pending\nowner: operator\nordinal: 1\n",
		"Which vendor do we cite for the SLA numbers?")
	plantItem(t, library, "fx-1", "b00000000002",
		"kind: acceptance_criterion\nstate: pending\nordinal: 2\n",
		"The endpoint returns 404 for an unknown id.")

	answer := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"show","arguments":{"actor":"alka","card":"fx-1"}}}`))
	detail, ok := answer["detail"].(map[string]any)
	if !ok {
		t.Fatalf("the show tool carried no detail: %v", answer)
	}
	throughTool, err := json.Marshal(detail["checklist"])
	if err != nil {
		t.Fatalf("marshal the tool's checklist: %v", err)
	}

	direct, _, _, err := library.Show(&verb.Request{Verb: "show", Actor: "alka", Card: "fx-1"})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	encoded, err := json.Marshal(direct)
	if err != nil {
		t.Fatalf("marshal the payload: %v", err)
	}
	var carried struct {
		Checklist json.RawMessage `json:"checklist"`
	}
	if err := json.Unmarshal(encoded, &carried); err != nil {
		t.Fatalf("decode the payload: %v\n%s", err, encoded)
	}
	if len(carried.Checklist) == 0 {
		t.Fatalf("the payload carries no checklist, so this test compares nothing: %s", encoded)
	}
	if !sameJSON(t, throughTool, carried.Checklist) {
		t.Errorf("the two heads disagree:\n tool %s\n  cli %s", throughTool, carried.Checklist)
	}
}

// sameJSON reports whether two encodings say the same thing, which is what a
// comparison across two heads has to ask: one side walks a decoded map on its
// way back out and the other does not, so the bytes differ over key order
// while the answer is the same.
func sameJSON(t *testing.T, left, right []byte) bool {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(left, &a); err != nil {
		t.Fatalf("decode %s: %v", left, err)
	}
	if err := json.Unmarshal(right, &b); err != nil {
		t.Fatalf("decode %s: %v", right, err)
	}
	one, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	two, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	return string(one) == string(two)
}

// plantItem writes a checklist item by hand, which is how one reaches a card:
// no verb writes one yet, and the format is a tree of plain files a person is
// meant to be able to edit.
func plantItem(t *testing.T, library *verb.Library, ref, id, frontmatter, text string) {
	t.Helper()
	found, err := library.Bench.ResolveCard(ref)
	if err != nil {
		t.Fatalf("resolve %s: %v", ref, err)
	}
	dir := filepath.Join(found.Card.Dir, bench.ChecklistDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	body := "---\n" + frontmatter + "---\n" + text + "\n"
	if err := os.WriteFile(filepath.Join(dir, bench.ItemAnchor), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", id, err)
	}
}
