package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"dinah/internal/verb"
)

// TestTheChangesToolIsTheProjectionOfTheOneLibraryCall covers dinah-120 AC-12:
// the tool stands in tools/list with a schema generated from the command's own
// parameter list, and it answers the ChangeSet the cli head emits under --json
// for the same arguments.
//
// The comparison is made against the library's own value rather than against a
// shape typed out here, because the cli head emits exactly that value and a
// copy typed here would agree with the head only until somebody changed one of
// them.
func TestTheChangesToolIsTheProjectionOfTheOneLibraryCall(t *testing.T) {
	library := newLibrary(t)

	listed := payloadOfToolsList(t, library)
	entry, ok := listed["changes"]
	if !ok {
		t.Fatalf("tools/list carries no changes tool: %v", keysOf(listed))
	}
	if entry.Description == "" {
		t.Error("the changes tool carries no description, so an agent reading the list is told nothing")
	}
	properties, ok := entry.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("the changes schema declares no properties: %v", entry.InputSchema)
	}
	for _, param := range verb.Params("changes") {
		property, named := properties[param.Name].(map[string]any)
		if !named {
			t.Errorf("the schema does not generate the %s argument the command declares", param.Name)
			continue
		}
		if property["description"] == "" {
			t.Errorf("the %s argument's schema carries no sentence", param.Name)
		}
	}
	for _, always := range []string{"actor", "basis", "workbench"} {
		if _, named := properties[always]; !named {
			t.Errorf("the changes schema drops the %s argument every tool carries", always)
		}
	}

	// A first call mints a cursor, and the tool and the library are handed the
	// same argument on the call that reports against it.
	minted := changesPayload(t, library, `{}`)
	token, ok := minted["cursor"].(string)
	if !ok || token == "" {
		t.Fatalf("the tool minted no cursor: %v", minted)
	}
	if minted["changed"] != false {
		t.Errorf("a first call over the tool reported a change: %v", minted["changed"])
	}

	encoded, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("marshal the cursor: %v", err)
	}
	carried := changesPayload(t, library, `{"since":`+string(encoded)+`}`)
	reencoded, err := json.Marshal(carried)
	if err != nil {
		t.Fatalf("marshal the tool's answer: %v", err)
	}
	toolSide := &verb.ChangeSet{}
	if err := json.Unmarshal(reencoded, toolSide); err != nil {
		t.Fatalf("the tool's answer does not decode as a ChangeSet: %v", err)
	}
	got, err := json.Marshal(toolSide)
	if err != nil {
		t.Fatalf("marshal the decoded answer: %v", err)
	}
	direct, err := library.Changes(&verb.Request{Verb: "changes", Actor: "alka", Since: token})
	if err != nil {
		t.Fatalf("the library refused the same cursor: %v", err)
	}
	want, err := json.Marshal(direct)
	if err != nil {
		t.Fatalf("marshal the library's own answer: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("the two heads answer differently:\n tool: %s\n  cli: %s", got, want)
	}
	if len(toolSide.Affordances) == 0 {
		t.Error("the answer carries no affordances, and every tool answer names what to do next")
	}
}

// TestTheChangesToolRefusesABadCursorAsARefusalRatherThanAnError asserts that
// the refusal reaches an agent as an answer, which is where a refusal lives on
// this surface.
func TestTheChangesToolRefusesABadCursorAsARefusalRatherThanAnError(t *testing.T) {
	library := newLibrary(t)
	carried := changesPayload(t, library, `{"since":"not-a-cursor"}`)
	if carried["outcome"] != "refused" {
		t.Fatalf("wanted a refused outcome, got %v", carried["outcome"])
	}
	if carried["refusal"] != "malformed" {
		t.Errorf("wanted the malformed refusal, got %v", carried["refusal"])
	}
	if carried["detail"] != "not-a-cursor" {
		t.Errorf("wanted the token as the detail, got %v", carried["detail"])
	}
}

// changesPayload calls the changes tool with one arguments object and returns
// the decoded answer.
func changesPayload(t *testing.T, library *verb.Library, arguments string) map[string]any {
	t.Helper()
	line := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"changes","arguments":` + arguments + `}}`
	return payload(t, ask(t, library, line))
}

// listedTool is one entry of tools/list, read back far enough to assert what
// the schema generator produced for it.
type listedTool struct {
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// payloadOfToolsList reads the surface back, keyed by tool name.
func payloadOfToolsList(t *testing.T, library *verb.Library) map[string]listedTool {
	t.Helper()
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatalf("marshal tools/list: %v", err)
	}
	var listed struct {
		Tools []struct {
			Name string `json:"name"`
			listedTool
		} `json:"tools"`
	}
	if err := json.Unmarshal(encoded, &listed); err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	surface := map[string]listedTool{}
	for _, entry := range listed.Tools {
		surface[entry.Name] = entry.listedTool
	}
	return surface
}

// keysOf names what a map carried, which is what a failure prints when the
// entry it wanted is absent.
func keysOf(surface map[string]listedTool) string {
	names := make([]string, 0, len(surface))
	for name := range surface {
		names = append(names, name)
	}
	return strings.Join(names, " ")
}
