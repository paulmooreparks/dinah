package mcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/verb"
)

// definition is the bench this head's tests are served over.
const definition = `{
  "profile": "dinah-core/1.0",
  "title": "Fixture",
  "instructions": "Standing text.\n",
  "states": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake", "instructions": "Intake text.\n" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "instructions": "Doing text.\n" }
  ]
}`

// newLibrary builds the bench and the library both heads project.
func newLibrary(t *testing.T) *verb.Library {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workbench")
	// The fixture instantiates the bench directly rather than through
	// verb.Init, which writes into a .dinah container under the directory it
	// is given; these tests want a bench at a path they name.
	read, err := bench.ReadDefinition([]byte(definition))
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
	if response := library.Add(&verb.Request{Verb: "add", Actor: "alka", Title: "A card"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("add: %s %s", response.Outcome, response.Refusal)
	}
	reopened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	library.Bench = reopened
	return library
}

// ask drives one JSON-RPC request through the head and returns the response.
func ask(t *testing.T, library *verb.Library, line string) *response {
	t.Helper()
	out := &strings.Builder{}
	if err := Serve(library, strings.NewReader(line+"\n"), out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	answer := &response{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), answer); err != nil {
		t.Fatalf("decode %q: %v", out.String(), err)
	}
	return answer
}

// payload decodes the canonical form a tool call carried back.
func payload(t *testing.T, answer *response) map[string]any {
	t.Helper()
	if answer.Error != nil {
		t.Fatalf("tool call failed at the transport: %+v", answer.Error)
	}
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatalf("result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("the tool call carried no content")
	}
	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
		t.Fatalf("payload %q: %v", result.Content[0].Text, err)
	}
	return decoded
}

// TestToolSurfaceIsTheProjection asserts that the head exposes the
// twenty-two tools the spec names, that each input schema is generated from
// the same parameter list the cli head composes its syntax from, and that the
// commands bound to a shell and a filesystem get no tool.
func TestToolSurfaceIsTheProjection(t *testing.T) {
	library := newLibrary(t)
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var listed struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(encoded, &listed); err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(listed.Tools) != 22 {
		t.Errorf("wanted twenty-two tools, got %d", len(listed.Tools))
	}
	names := map[string]bool{}
	for _, tool := range listed.Tools {
		names[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("%s carries no description", tool.Name)
		}
		properties, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s carries no input schema properties", tool.Name)
		}
		command := toolsByName[tool.Name].command
		for _, param := range verb.Params(command) {
			if _, carried := properties[param.Name]; !carried {
				t.Errorf("%s: the schema is missing the parameter %s the library defines", tool.Name, param.Name)
			}
		}
		if _, carried := properties["actor"]; !carried {
			t.Errorf("%s: every tool takes an actor", tool.Name)
		}
	}
	for _, wanted := range []string{"claim", "move", "release", "block", "unblock", "add_card", "list_cards", "next_card", "workbench"} {
		if !names[wanted] {
			t.Errorf("the surface is missing the tool %s", wanted)
		}
	}
	for _, absent := range []string{"path", "edit", "init", "extract", "config", "mcp", "guide"} {
		if names[absent] {
			t.Errorf("%s should not be a tool on this head", absent)
		}
	}
}

// TestInitializeCarriesTheWorkingAgreement asserts that the initialize
// response's instructions field carries section 8 of the profile and the
// orientation an agent needs before its first tool call.
func TestInitializeCarriesTheWorkingAgreement(t *testing.T) {
	library := newLibrary(t)
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		Instructions    string `json:"instructions"`
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if result.ProtocolVersion == "" {
		t.Error("initialize carried no protocol version")
	}
	for _, rule := range []string{"Claim a card before", "stopped working", "Treat the workbench as the authority", "operator-owned"} {
		if !strings.Contains(result.Instructions, rule) {
			t.Errorf("the working agreement is missing %q", rule)
		}
	}
	if !strings.Contains(result.Instructions, "affordances") {
		t.Error("the orientation should name the affordances member every response carries")
	}
}

// TestEveryToolResponseCarriesAffordances asserts that an agent never has to
// learn which responses answer the question of what it may do next.
func TestEveryToolResponseCarriesAffordances(t *testing.T) {
	library := newLibrary(t)
	calls := []string{
		`{"name":"status","arguments":{"actor":"alka"}}`,
		`{"name":"states","arguments":{"actor":"alka"}}`,
		`{"name":"list_cards","arguments":{"actor":"alka"}}`,
		`{"name":"next_card","arguments":{"actor":"alka"}}`,
		`{"name":"show","arguments":{"actor":"alka","card":"fx-1"}}`,
		`{"name":"log","arguments":{"actor":"alka","card":"fx-1"}}`,
		`{"name":"instructions","arguments":{"actor":"alka","card":"fx-1"}}`,
		`{"name":"whoami","arguments":{"actor":"alka"}}`,
		`{"name":"version","arguments":{}}`,
		`{"name":"export","arguments":{}}`,
		`{"name":"check","arguments":{}}`,
		`{"name":"claim","arguments":{"actor":"alka","card":"fx-99"}}`,
		`{"name":"claim","arguments":{"actor":"alka","card":"fx-1"}}`,
	}
	for _, call := range calls {
		t.Run(call, func(t *testing.T) {
			answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":`+call+`}`)
			decoded := payload(t, answer)
			if _, carried := decoded["affordances"]; !carried {
				t.Errorf("the response carries no affordances member: %v", decoded)
			}
		})
	}
}

// TestClaimCarriesStructuredInstructions asserts that a successful claim
// response carries the three instruction layers and the legal moves as
// separate structured members rather than as a prose blob.
func TestClaimCarriesStructuredInstructions(t *testing.T) {
	library := newLibrary(t)
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"claim","arguments":{"actor":"alka","card":"fx-1"}}}`)
	decoded := payload(t, answer)
	if decoded["outcome"] != contract.OutcomeOK {
		t.Fatalf("claim: %v", decoded)
	}
	instructions, ok := decoded["instructions"].(map[string]any)
	if !ok {
		t.Fatalf("wanted structured instructions, got %v", decoded["instructions"])
	}
	if instructions["standing"] != "Standing text.\n" {
		t.Errorf("standing layer: got %v", instructions["standing"])
	}
	if instructions["state"] != "Intake text.\n" {
		t.Errorf("state layer: got %v", instructions["state"])
	}
	if _, carried := decoded["legal_moves"]; !carried {
		t.Error("wanted the legal moves as a member of their own")
	}
}

// TestBothHeadsAnswerAlike asserts the one-implementation rule: the same
// request driven through the library and through this head produces the same
// canonical payload, so the two surfaces cannot drift.
func TestBothHeadsAnswerAlike(t *testing.T) {
	direct := newLibrary(t)
	throughHead := newLibrary(t)

	req := &verb.Request{Verb: verb.Claim, Actor: "alka", Card: "fx-1"}
	expected := direct.Do(req)
	answer := ask(t, throughHead, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"claim","arguments":{"actor":"alka","card":"fx-1"}}}`)
	got := payload(t, answer)

	// The identifiers and the revisions belong to two different benches, so
	// the comparison is over everything else.
	for _, member := range []string{"outcome", "verb", "affordances", "instructions", "legal_moves"} {
		wanted, err := json.Marshal(fieldOf(t, expected, member))
		if err != nil {
			t.Fatalf("marshal %s: %v", member, err)
		}
		found, err := json.Marshal(got[member])
		if err != nil {
			t.Fatalf("marshal %s: %v", member, err)
		}
		if string(wanted) != string(found) {
			t.Errorf("%s differs between the heads:\nlibrary: %s\nmcp:     %s", member, wanted, found)
		}
	}
}

// fieldOf reads one member out of a canonical response, by encoding it and
// reading the member back, which is what keeps the comparison honest about
// the JSON both heads actually carry.
func fieldOf(t *testing.T, response *verb.Response, member string) any {
	t.Helper()
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return decoded[member]
}

// TestGuidesAreResourcesAndMatchTheCLI asserts that the embedded guides are
// served as MCP resources rather than as a tool, and that the text this head
// serves is byte-identical to what the cli head prints for the same topic.
func TestGuidesAreResourcesAndMatchTheCLI(t *testing.T) {
	library := newLibrary(t)
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`)
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var listed struct {
		Resources []struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(encoded, &listed); err != nil {
		t.Fatalf("resources/list: %v", err)
	}
	topics := guide.Topics()
	if len(listed.Resources) != len(topics) {
		t.Fatalf("wanted one resource per guide, got %d for %d guides", len(listed.Resources), len(topics))
	}
	for _, resource := range listed.Resources {
		read := ask(t, library, `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"`+resource.URI+`"}}`)
		body, err := json.Marshal(read.Result)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var contents struct {
			Contents []struct {
				Text string `json:"text"`
			} `json:"contents"`
		}
		if err := json.Unmarshal(body, &contents); err != nil {
			t.Fatalf("resources/read: %v", err)
		}
		topic := strings.TrimPrefix(resource.URI, "dinah://guide/")
		wanted, err := guide.Text(topic)
		if err != nil {
			t.Fatalf("guide %s: %v", topic, err)
		}
		if len(contents.Contents) == 0 || contents.Contents[0].Text != wanted {
			t.Errorf("%s: the resource text differs from the guide the cli head prints", topic)
		}
	}
}

// TestNotificationsGetNoAnswer asserts the transport rule that a request
// carrying no identifier wants no response.
func TestNotificationsGetNoAnswer(t *testing.T) {
	library := newLibrary(t)
	out := &strings.Builder{}
	if err := Serve(library, strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"), out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if out.String() != "" {
		t.Errorf("a notification was answered: %q", out.String())
	}
}

// TestUnknownMethodIsATransportError asserts that a method the head does not
// implement comes back as a JSON-RPC error rather than as a refusal, since a
// refusal is a contract answer and this is not one.
func TestUnknownMethodIsATransportError(t *testing.T) {
	library := newLibrary(t)
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"nonesuch"}`)
	if answer.Error == nil || answer.Error.Code != codeMethodNotFound {
		t.Errorf("wanted a method-not-found error, got %+v", answer)
	}
}
