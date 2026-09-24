package mcp

import (
	"path/filepath"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// enumeratedDefinition declares one string field carrying a `values` list, so
// the MCP write path this file exercises has something to refuse against.
const enumeratedDefinition = `{
  "profile": "dinah-core/0.18",
  "title": "Fixture",
  "instructions": "Standing text.\n",
  "fields": { "card.kind": { "type": "string", "meaning": "what kind of work this card is", "values": ["bug", "feature", "chore"] } },
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake", "instructions": "Intake text.\n" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "instructions": "Doing text.\n" }
  ]
}`

// newEnumeratedLibrary is newLevelledLibrary's shape, aimed at a workbench
// declaring an enumerated field instead of the two level axes.
func newEnumeratedLibrary(t *testing.T) *verb.Library {
	t.Helper()
	base := t.TempDir()
	root := containedPath(filepath.Join(base, "workbench"))
	read, err := bench.ReadDefinition([]byte(enumeratedDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := bench.Instantiate(root, "fx", "alka", read); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	library := verb.New(opened, filepath.Join(base, "home"))
	filed := library.Add(&verb.Request{Verb: "add", Actor: "alka", Title: "A card"})
	if filed.Outcome != contract.OutcomeOK {
		t.Fatalf("add: %s %s", filed.Outcome, filed.Refusal)
	}
	reopened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	library.Bench = reopened
	return library
}

// TestSetFieldRefusesAValueOutsideTheDeclaredList drives CORE-FIELD-13 on the
// MCP surface, mirroring the CLI-level assertion in
// internal/verb/declaredfields_test.go: a legal value succeeds, and an
// illegal one is refused malformed with the legal list under legalValues in
// the refusal's context, the same shape the CLI write path produces.
func TestSetFieldRefusesAValueOutsideTheDeclaredList(t *testing.T) {
	library := newEnumeratedLibrary(t)
	written := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"set_field","arguments":{"ref":"fx-1","field":"card.kind","value":"bug","actor":"alka"}}}`))
	if outcome, _ := written["outcome"].(string); outcome != contract.OutcomeOK {
		t.Fatalf("the legal write answered %v", written)
	}
	refused := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"set_field","arguments":{"ref":"fx-1","field":"card.kind","value":"Bug","actor":"alka"}}}`))
	if outcome, _ := refused["outcome"].(string); outcome != contract.OutcomeRefused {
		t.Fatalf("the illegal write answered %v", refused)
	}
	if name, _ := refused["refusal"].(string); name != contract.Malformed {
		t.Errorf("the refusal names %v, wanted malformed", refused["refusal"])
	}
	context, _ := refused["context"].(map[string]any)
	if context == nil {
		t.Fatalf("the refusal carries no context: %v", refused)
	}
	if got, _ := context["legalValues"].(string); got != "bug, feature, chore" {
		t.Errorf("the refusal's legalValues context is %q, wanted %q", got, "bug, feature, chore")
	}
	read := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_field","arguments":{"ref":"fx-1","field":"card.kind","actor":"alka"}}}`))
	if value, _ := read["value"].(string); value != "bug" {
		t.Errorf("the refused write changed the stored value: %v", read)
	}
}
