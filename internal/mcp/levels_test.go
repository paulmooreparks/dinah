package mcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// levelledDefinition is the fixture definition with the two level sets
// declared, so the write path this file exercises has something to admit.
const levelledDefinition = `{
  "profile": "dinah-core/1.0",
  "title": "Fixture",
  "instructions": "Standing text.\n",
  "levels": { "severity": ["trivial", "minor", "major", "critical"], "priority": ["later", "soon", "next", "now"] },
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake", "instructions": "Intake text.\n" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "instructions": "Doing text.\n" }
  ]
}`

// newLevelledLibrary is newLibrary over a workbench that declares both axes.
func newLevelledLibrary(t *testing.T) *verb.Library {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workbench")
	read, err := bench.ReadDefinition([]byte(levelledDefinition))
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

// schemaOf reads one tool's generated input schema off tools/list.
func schemaOf(t *testing.T, library *verb.Library, want string) map[string]any {
	t.Helper()
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var listed struct {
		Tools []struct {
			Name        string         `json:"name"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(encoded, &listed); err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	for _, tool := range listed.Tools {
		if tool.Name == want {
			return tool.InputSchema
		}
	}
	t.Fatalf("the surface carries no %s tool", want)
	return nil
}

// propertyDescription reads one property's description out of a schema.
func propertyDescription(t *testing.T, schema map[string]any, name string) string {
	t.Helper()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("the schema carries no properties")
	}
	property, ok := properties[name].(map[string]any)
	if !ok {
		t.Fatalf("the schema carries no %s property", name)
	}
	described, _ := property["description"].(string)
	return described
}

// requiredOf reads a schema's required list.
func requiredOf(schema map[string]any) []string {
	raw, ok := schema["required"].([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(raw))
	for _, entry := range raw {
		if name, ok := entry.(string); ok {
			names = append(names, name)
		}
	}
	return names
}

// TestTheCardToolCarriesTheSameSentencesTheTerminalPrints asserts dinah-193
// AC-20: the surface serves a card tool whose schema carries the four
// arguments the parameter table declares, add_card carries the two new ones,
// and every property is described with the sentence the cli head prints beside
// the same argument.
func TestTheCardToolCarriesTheSameSentencesTheTerminalPrints(t *testing.T) {
	library := newLevelledLibrary(t)
	catalog := msg.For(msg.Base)
	card := schemaOf(t, library, "card")
	for _, param := range verb.Params("card") {
		wanted := catalog.T(param.SummaryKey("card"))
		if got := propertyDescription(t, card, param.Name); got != wanted {
			t.Errorf("the card tool describes %s as %q and the terminal prints %q", param.Name, got, wanted)
		}
	}
	added := schemaOf(t, library, "add_card")
	for _, name := range []string{bench.SeverityField, bench.PriorityField} {
		wanted := catalog.T("param.add." + name + ".summary")
		if got := propertyDescription(t, added, name); got != wanted {
			t.Errorf("add_card describes %s as %q and the terminal prints %q", name, got, wanted)
		}
	}
}

// TestTheCardToolMarksItsThreeSlotsRequired asserts dinah-193 AC-25. Required
// is read by Param.Token and by this schema generator and by nothing in the
// cli parser, so what the declaration buys is a call naming no action, no card
// or no field being refused before the tool runs at all.
func TestTheCardToolMarksItsThreeSlotsRequired(t *testing.T) {
	library := newLevelledLibrary(t)
	got := strings.Join(requiredOf(schemaOf(t, library, "card")), ",")
	if got != "action,card,field" {
		t.Errorf("the card tool's required list is %q, wanted action, card and field with value left out", got)
	}
}

// TestTheCardToolReadsAndWritesOneField asserts that the tool reaches the same
// two acts a person reaches from a terminal, and that the refusals a write
// raises travel to an agent under their own names.
func TestTheCardToolReadsAndWritesOneField(t *testing.T) {
	library := newLevelledLibrary(t)
	written := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"card","arguments":{"action":"set","card":"fx-1","field":"severity","value":"major","actor":"alka"}}}`))
	if outcome, _ := written["outcome"].(string); outcome != contract.OutcomeOK {
		t.Fatalf("the write answered %v", written)
	}
	read := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"card","arguments":{"action":"get","card":"fx-1","field":"severity","actor":"alka"}}}`))
	if value, _ := read["value"].(string); value != "major" {
		t.Errorf("the read answered %v", read)
	}
	refused := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"card","arguments":{"action":"set","card":"fx-1","field":"severity","value":"urgent","actor":"alka"}}}`))
	if name, _ := refused["refusal"].(string); name != contract.UnknownLevel {
		t.Errorf("the refused write answered %v", refused)
	}
}
