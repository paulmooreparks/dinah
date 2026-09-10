package mcp

import (
	"encoding/json"
	"errors"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestTheRestoreToolIsServedWithItsReference is dinah-461 AC-9.
//
// The schema assertions read the served JSON rather than the definition table,
// so a parameter declared and not published is caught rather than assumed.
func TestTheRestoreToolIsServedWithItsReference(t *testing.T) {
	library := newLibrary(t)
	served := toolsByName["restore"]
	if served.name == "" {
		t.Fatal("this head serves no tool called restore")
	}

	schema := publishedSchema(t, library, "restore")
	properties, held := schema["properties"].(map[string]any)
	if !held {
		t.Fatalf("the restore tool publishes no properties: %v", schema)
	}
	if _, declared := properties["ref"]; !declared {
		t.Errorf("the restore tool's schema declares no ref property: %v", properties)
	}
	required, listed := schema["required"].([]any)
	if !listed {
		t.Fatalf("the restore tool declares nothing required: %v", schema)
	}
	if !carriesString(required, "ref") {
		t.Errorf("the restore tool does not require ref: %v", required)
	}

	// The tool answers what the same reference answers at the library, which
	// is the one act both heads reach.
	archived := library.Archive(&verb.Request{Verb: "archive", Actor: "alka", Ref: "fx-2"})
	if archived.Outcome != contract.OutcomeOK {
		t.Fatalf("archive fx-2: %s %s", archived.Outcome, archived.Refusal)
	}
	answer := payload(t, ask(t, library,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"restore","arguments":{"actor":"alka","ref":"fx-2"}}}`))
	if answer["outcome"] != contract.OutcomeOK {
		t.Fatalf("the restore tool answered %v with %v", answer["outcome"], answer["refusal"])
	}
	if answer["detail"] != archived.Detail {
		t.Errorf("the restore tool reports %v and the archive reported %v, and one entity carries one identifier", answer["detail"], archived.Detail)
	}

	// A second restore of the same reference refuses what the CLI refuses for
	// it, which is that nothing in the archive answers to it any more.
	again := payload(t, ask(t, library,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"restore","arguments":{"actor":"alka","ref":"fx-2"}}}`))
	if again["refusal"] != contract.NotArchived {
		t.Errorf("restoring a live reference refused %v and it refuses %s", again["refusal"], contract.NotArchived)
	}
}

// TestTheArchivedFlagIsPublishedAndHonouredOnEveryCommandThatTakesIt is the
// second arm of dinah-461 AC-9.
//
// The path arm is here because runPath answers the workbench out of discovery
// before the bench is opened at all, so every other check in this card would
// pass while that one command silently ignored the flag.
func TestTheArchivedFlagIsPublishedAndHonouredOnEveryCommandThatTakesIt(t *testing.T) {
	library := newLibrary(t)
	for _, name := range []string{"show", "contents"} {
		schema := publishedSchema(t, library, name)
		properties, held := schema["properties"].(map[string]any)
		if !held {
			t.Errorf("the %s tool publishes no properties: %v", name, schema)
			continue
		}
		declared, published := properties["archived"].(map[string]any)
		if !published {
			t.Errorf("the %s tool's schema declares no archived property: %v", name, properties)
			continue
		}
		if declared["type"] != "boolean" {
			t.Errorf("the %s tool declares archived as %v and it is a boolean", name, declared["type"])
		}
	}

	// path is served by no tool, so its half of this arm is asserted against
	// the resolver runPath reaches once its own pre-bench shortcut is guarded
	// off. The shortcut itself is asserted at the terminal, where
	// cmd/dinah's TestEveryArchivedHalfCommandRefusesALiveReference runs
	// `dinah path --archived workbench` and reads its exit code.
	_, err := library.Bench.ResolvePathIn(bench.ArchivedHalf, "workbench")
	if err == nil {
		t.Fatal("the workbench resolves under the archived half, and the archive never holds it")
	}
	var refusal *contract.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("the workbench under the archived half fails with %v, which is no refusal", err)
	}
	if refusal.Name != contract.NotArchived {
		t.Errorf("the workbench under the archived half refuses %s and it refuses %s", refusal.Name, contract.NotArchived)
	}
}

// publishedSchema reads one tool's input schema off what tools/list actually
// serves, rather than off the definition the generator reads.
func publishedSchema(t *testing.T, library *verb.Library, name string) map[string]any {
	t.Helper()
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatalf("marshal tools/list: %v", err)
	}
	var listed struct {
		Tools []struct {
			Name   string         `json:"name"`
			Schema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(encoded, &listed); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	for _, tool := range listed.Tools {
		if tool.Name == name {
			return tool.Schema
		}
	}
	t.Fatalf("tools/list serves no tool called %s", name)
	return nil
}

// carriesString reports whether a JSON array of strings holds one.
func carriesString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
