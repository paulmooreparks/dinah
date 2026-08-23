package mcp

import (
	"encoding/json"
	"path/filepath"
	"reflect"
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
// The root defaults to the test's bench root, which holds exactly the one
// workbench newLibrary opens; the libraries map is held by the test helper so
// each ask call reuses what an earlier ask opened.
func ask(t *testing.T, library *verb.Library, line string) *response {
	t.Helper()
	out := &strings.Builder{}
	root := library.Bench.Root
	if err := Serve(root, library, map[string]*verb.Library{}, strings.NewReader(line+"\n"), out); err != nil {
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
// twenty-six tools the spec names, that each input schema is generated from
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
	if len(listed.Tools) != 32 {
		t.Errorf("wanted thirty-two tools, got %d", len(listed.Tools))
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
	for _, wanted := range []string{"claim", "move", "pull", "release", "block", "unblock", "add_card", "list_cards", "next_card", "query", "workbench", "workstream", "join_workstream", "leave_workstream"} {
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
	for at, resource := range listed.Resources {
		// The position is asserted as well as the membership, because one
		// declared reading order governs every surface that offers the
		// guides. A count and a set of bytes hold while this head answers
		// in any arrangement, and the arrangement is the thing a reader
		// arriving with no guide open takes its recommendation from.
		if wanted := "dinah://guide/" + topics[at]; resource.URI != wanted {
			t.Errorf("resource %d is %s and the reading order places %s there", at, resource.URI, wanted)
		}
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
	if err := Serve(library.Bench.Root, library, map[string]*verb.Library{}, strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"), out); err != nil {
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

// TestTheQueryToolCarriesTheSameMatchesTheCliEmits asserts the parity the
// query card's whole design rests on: the tool takes the same string the
// command line takes, hands it to the one library call, and its result carries
// an object identical to the one the cli head emits under --json for that same
// string.
//
// The comparison is made against the library's own Matches marshalled to JSON
// rather than against a shape typed out here, because the cli head emits
// exactly that value and a copy typed here would agree with the head only until
// somebody changed one of them.
func TestTheQueryToolCarriesTheSameMatchesTheCliEmits(t *testing.T) {
	library := newLibrary(t)
	for _, text := range []string{"", " ", "substate:ready", "holder:nobody"} {
		encoded, err := json.Marshal(text)
		if err != nil {
			t.Fatalf("marshal %q: %v", text, err)
		}
		answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"query":`+string(encoded)+`}}}`)
		carried := payload(t, answer)
		if len(carried["affordances"].([]any)) == 0 {
			t.Errorf("query %q carried no affordances", text)
		}
		// payload decodes into maps, which lose member order, so the tool's
		// own answer is read back into the library's type and re-emitted. A
		// member the tool dropped, added or changed still fails below.
		reencoded, err := json.Marshal(carried["matches"])
		if err != nil {
			t.Fatalf("marshal the tool's answer to %q: %v", text, err)
		}
		toolSide := &verb.Matches{}
		if err := json.Unmarshal(reencoded, toolSide); err != nil {
			t.Fatalf("the tool's answer to %q does not decode as Matches: %v", text, err)
		}
		got, err := json.Marshal(toolSide)
		if err != nil {
			t.Fatalf("marshal the tool's answer to %q: %v", text, err)
		}
		direct, err := library.Query(&verb.Request{Verb: "query", Actor: "alka", Query: text})
		if err != nil {
			t.Fatalf("the library refused %q: %v", text, err)
		}
		want, err := json.Marshal(direct)
		if err != nil {
			t.Fatalf("marshal the library's own answer: %v", err)
		}
		if string(got) != string(want) {
			t.Errorf("query %q:\n tool: %s\n  cli: %s", text, got, want)
		}
	}
}

// TestTheWorkbenchToolReadsAndGuardsTheSameWayTheTerminalDoes asserts that the
// workbench tool answers a read with the same three fields the terminal
// listing prints, that a write by the operator lands, and that a write by
// somebody else refuses under the name the terminal raises, since both heads
// run one library and the operator check lives there rather than in either.
func TestTheWorkbenchToolReadsAndGuardsTheSameWayTheTerminalDoes(t *testing.T) {
	library := newLibrary(t)

	read := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbench","arguments":{"actor":"alka"}}}`))
	fields, ok := read["workbench"].(map[string]any)
	if !ok {
		t.Fatalf("the read carries no workbench member: %v", read)
	}
	if fields["slug"] != "fx" || fields["operator"] != "alka" || fields["title"] == "" {
		t.Errorf("the read answered %v, wanted the fixture's own three fields", fields)
	}

	refused := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbench","arguments":{"actor":"bob","action":"set","field":"title","value":"Renamed"}}}`))
	if refused["outcome"] != contract.OutcomeRefused || refused["refusal"] != contract.NotOperator {
		t.Errorf("a write by somebody other than the operator: wanted %s, got %v", contract.NotOperator, refused)
	}
	// A refusal names where to recover in the same tool vocabulary every
	// read answers in, so an agent that follows the affordances can actually
	// call what they name. The library spells these two reads as commands; the
	// head translates them to the surface's tool names before serving.
	if affordances, ok := refused["affordances"].([]any); ok {
		got := make([]string, len(affordances))
		for i, a := range affordances {
			got[i], _ = a.(string)
		}
		want := []string{"status", "states", "list_cards", "next_card"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("a refusal's affordances: got [%s], want surface tool names [%s]", strings.Join(got, ","), strings.Join(want, ","))
		}
	} else {
		t.Errorf("a refusal carries no affordances member: %v", refused)
	}

	written := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbench","arguments":{"actor":"alka","action":"set","field":"title","value":"Renamed"}}}`))
	if written["outcome"] != contract.OutcomeOK {
		t.Fatalf("a write by the operator: %v", written)
	}
	reread := payload(t, ask(t, newLibraryAt(t, library.Bench.Root), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbench","arguments":{"actor":"alka"}}}`))
	if reread["workbench"].(map[string]any)["title"] != "Renamed" {
		t.Errorf("the write did not reach the anchor: %v", reread["workbench"])
	}
}

// TestARefusalFromWorkbenchResolutionNamesRecoveryInToolNames asserts that the
// other refusal route, the one the library-resolution step raises before a
// tool runs, also serves its affordances as tool names rather than the
// library's command spellings. It travels through answerRefusal rather than
// through a tool's returned response, so it needs its own pin.
func TestARefusalFromWorkbenchResolutionNamesRecoveryInToolNames(t *testing.T) {
	library := newLibrary(t)
	outside := filepath.ToSlash(filepath.Join(library.Bench.Root, "..", "outside"))

	refused := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbench","arguments":{"workbench":"`+outside+`"}}}`))
	if refused["outcome"] != contract.OutcomeRefused || refused["refusal"] != contract.OutsideRoot {
		t.Fatalf("a workbench argument outside the root: wanted %s, got %v", contract.OutsideRoot, refused)
	}
	affordances, ok := refused["affordances"].([]any)
	if !ok {
		t.Fatalf("an outside-root refusal carries no affordances member: %v", refused)
	}
	got := make([]string, len(affordances))
	for i, a := range affordances {
		got[i], _ = a.(string)
	}
	want := []string{"status", "states", "list_cards", "next_card"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("an outside-root refusal's affordances: got [%s], want [%s]", strings.Join(got, ","), strings.Join(want, ","))
	}
}

// newLibraryAt opens a second library over a workbench that already exists,
// which is how a test reads back what a write through the head landed.
func newLibraryAt(t *testing.T, root string) *verb.Library {
	t.Helper()
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("open %s: %v", root, err)
	}
	return verb.New(opened, filepath.Join(root, "home"))
}

// TestTheWorkstreamToolsAnswerTheWayTheTerminalDoes asserts the three tools
// this card adds. The workstream tool creates, lists and reads through the one
// library the terminal runs, and the two membership tools write the card's own
// list, so an agent reaches the same four acts a person reaches and the
// answers are the canonical forms the cli head prints under --json.
func TestTheWorkstreamToolsAnswerTheWayTheTerminalDoes(t *testing.T) {
	library := newLibrary(t)

	created := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workstream","arguments":{"actor":"alka","action":"new","workstream":"Portfolio work"}}}`))
	if created["outcome"] != contract.OutcomeOK {
		t.Fatalf("creating a workstream: %v", created)
	}
	made, ok := created["workstream"].(map[string]any)
	if !ok {
		t.Fatalf("the answer carries no workstream member: %v", created)
	}
	if made["slug"] != "portfolio-work" || made["status"] != "active" {
		t.Errorf("the created workstream reads %v", made)
	}

	root := library.Bench.Root
	listed := payload(t, ask(t, newLibraryAt(t, root), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workstream","arguments":{"actor":"alka"}}}`))
	listing, ok := listed["listing"].(map[string]any)
	if !ok {
		t.Fatalf("the listing carries no listing member: %v", listed)
	}
	if rows, ok := listing["workstreams"].([]any); !ok || len(rows) != 1 {
		t.Errorf("the listing carries %v", listing["workstreams"])
	}

	card := payload(t, ask(t, newLibraryAt(t, root), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"add_card","arguments":{"actor":"alka","title":"a card to belong"}}}`))
	if card["outcome"] != contract.OutcomeOK {
		t.Fatalf("filing a card: %v", card)
	}
	ref := card["card"].(map[string]any)["ref"].(string)

	joined := payload(t, ask(t, newLibraryAt(t, root), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"join_workstream","arguments":{"actor":"alka","card":"`+ref+`","workstream":"portfolio-work"}}}`))
	if joined["outcome"] != contract.OutcomeOK {
		t.Fatalf("joining a workstream: %v", joined)
	}
	memberships, ok := joined["card"].(map[string]any)["workstreams"].([]any)
	if !ok || len(memberships) != 1 || memberships[0] != made["id"] {
		t.Errorf("the card carries %v, wanted the workstream's own identifier", joined["card"])
	}

	read := payload(t, ask(t, newLibraryAt(t, root), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workstream","arguments":{"actor":"alka","action":"get","workstream":"portfolio-work"}}}`))
	detail, ok := read["detail"].(map[string]any)
	if !ok {
		t.Fatalf("the read carries no detail member: %v", read)
	}
	if detail["workstream"].(map[string]any)["cards"].(float64) != 1 {
		t.Errorf("the read counts %v member cards, wanted one", detail["workstream"])
	}

	left := payload(t, ask(t, newLibraryAt(t, root), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"leave_workstream","arguments":{"actor":"alka","card":"`+ref+`","workstream":"portfolio-work"}}}`))
	if left["outcome"] != contract.OutcomeOK {
		t.Fatalf("leaving a workstream: %v", left)
	}
	if _, carried := left["card"].(map[string]any)["workstreams"]; carried {
		t.Errorf("a card belonging to no workstream still carries the member: %v", left["card"])
	}

	refused := payload(t, ask(t, newLibraryAt(t, root), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"join_workstream","arguments":{"actor":"alka","card":"`+ref+`","workstream":"nosuch"}}}`))
	if refused["outcome"] != contract.OutcomeRefused || refused["refusal"] != contract.UnknownWorkstream {
		t.Errorf("joining an unknown workstream: wanted %s, got %v", contract.UnknownWorkstream, refused)
	}
}

// TestEverySchemaPropertyIsDescribedAndNoneCarriesAnEnum asserts dinah-172
// AC-12: every property of every generated input schema carries a non-empty
// description, including the two schemaFor adds beyond any parameter table,
// and no property carries an enum.
//
// A description is additive and constrains no caller. An enum changes what a
// strict client will send, which is a change to a published machine interface,
// so its absence is asserted here rather than left to be noticed.
func TestEverySchemaPropertyIsDescribedAndNoneCarriesAnEnum(t *testing.T) {
	library := newLibrary(t)
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
	if len(listed.Tools) == 0 {
		t.Fatal("the surface carries no tool, so this test proves nothing")
	}
	described, beyond := 0, 0
	for _, tool := range listed.Tools {
		properties, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s carries no input schema properties", tool.Name)
		}
		for name, raw := range properties {
			property, ok := raw.(map[string]any)
			if !ok {
				t.Errorf("%s: the property %s is not an object", tool.Name, name)
				continue
			}
			description, _ := property["description"].(string)
			if strings.TrimSpace(description) == "" {
				t.Errorf("%s: the property %s carries no description", tool.Name, name)
				continue
			}
			if strings.HasPrefix(description, "{") {
				t.Errorf("%s: the property %s describes itself with the bare catalog key %s", tool.Name, name, description)
			}
			described++
			if name == "actor" || name == "basis" || name == "workbench" {
				beyond++
			}
			if _, carried := property["enum"]; carried {
				t.Errorf("%s: the property %s carries an enum, which changes what a strict client sends", tool.Name, name)
			}
		}
	}
	if described == 0 {
		t.Fatal("no property was read, so this test proves nothing")
	}
	// workbenches is the one tool that does not carry a workbench property,
	// so the injected count is three per tool minus the one exception.
	if beyond != 3*len(listed.Tools)-1 {
		t.Errorf("read %d injected properties across %d tools, want 3 per tool minus the workbenches exception", beyond, len(listed.Tools))
	}
}

// TestEveryDeclaredParameterReachesTheRequest asserts that every parameter a
// tool advertises in its schema is one this head can actually put on a
// request.
//
// The schema is generated from verb.Params, and request2Args reads the same
// list, but the two switch statements it dispatches into are written by hand.
// A parameter added to a command's table therefore appears in the schema, is
// offered to every agent reading the surface, is looked up by name on the way
// in, and then falls off the end of a switch that has no case for it. Nothing
// fails: the call succeeds and the argument is discarded, so an agent asking
// for one thing is silently given another. `no-claim` shipped that way and is
// the reason this check exists.
//
// The check drives a value through request2Args and asserts the request came
// back different from an empty one, which is what "reached the request"
// means. A parameter that lands in the wrong field still passes, so the check
// bounds one defect class and does not prove the assignment correct.
func TestEveryDeclaredParameterReachesTheRequest(t *testing.T) {
	// A parameter the head deliberately reads and discards is named here with
	// the reason, so that the deliberate case is visible rather than looking
	// like the accident this check exists to find. An entry here does not skip
	// the parameter either. The check requires the parameter to go on landing
	// nowhere, so an entry states what the head does today rather than excusing
	// it from being read, and wiring the parameter up reddens this test until
	// somebody says the decision has changed.
	carriedNowhereOnPurpose := map[string]string{
		"version.catalogs": "the version tool always reports catalog coverage, so the marker selects nothing",
	}
	// A parameter this check found landing nowhere, which is a defect rather
	// than a decision and is tracked on its own card. An entry here does not
	// skip the parameter. The check still builds the request and requires the
	// defect to still be present, so repairing the defect reddens this test
	// and the entry has to be deleted along with it.
	knownDefect := map[string]string{
		"attach.description": "dinah-222: assignValue has no case for it, so an attachment made over this head is created with no description",
	}
	checked := 0
	for _, entry := range tools {
		for _, param := range verb.Params(entry.command) {
			named := entry.command + "." + param.Name
			arguments := map[string]any{}
			if param.Marker {
				arguments[param.Name] = true
			} else {
				arguments[param.Name] = durationOrWord(param.Name)
			}
			built := request2Args(entry.command, arguments)
			empty := request2Args(entry.command, map[string]any{})
			reached := !reflect.DeepEqual(built, empty)
			if reason, deliberate := carriedNowhereOnPurpose[named]; deliberate {
				if reached {
					t.Errorf("%s: %q reaches the request, and this entry says it never does (%s); the decision has changed, so change the entry or take it out",
						entry.name, param.Name, reason)
				}
				continue
			}
			if tracked, defective := knownDefect[named]; defective {
				if reached {
					t.Errorf("%s: %q now reaches the request, so the exemption has outlived its defect and belongs deleted (%s)",
						entry.name, param.Name, tracked)
				}
				continue
			}
			checked++
			if !reached {
				t.Errorf("%s: the schema offers %q and request2Args puts it nowhere on the request, so an agent sending it is silently ignored",
					entry.name, param.Name)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no tool declared a parameter, so this check read nothing")
	}
}

// durationOrWord returns a value the assignment will accept for one named
// parameter. Most take any word; `expires` is parsed as a duration and a word
// it cannot parse would be dropped for a reason this check is not about.
func durationOrWord(name string) string {
	if name == "expires" {
		return "8h"
	}
	return "x"
}

// TestPullIsDrivenThroughTheHead asserts that the pull tool works over the
// protocol rather than merely appearing on the surface.
//
// Taking work is the thing an agent does over this head rather than at a
// terminal, so pull is the tool that most needs its arguments proved to cross
// the boundary. The schema projection holds the shape of the call and says
// nothing about what happens to it, which is how the `no-claim` marker came to
// be advertised, accepted, and thrown away.
//
// The fixture's flow is Intake then Doing, with one ready card in Intake, so a
// pull into Doing takes that card and a pull into Intake has nothing before it
// to take from.
func TestPullIsDrivenThroughTheHead(t *testing.T) {
	t.Run("the named form takes the card and claims it", func(t *testing.T) {
		library := newLibrary(t)
		answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"pull","arguments":{"actor":"alka","state":"doing"}}}`)
		decoded := payload(t, answer)
		if decoded["outcome"] != contract.OutcomeOK {
			t.Fatalf("pull: %v", decoded)
		}
		card, ok := decoded["card"].(map[string]any)
		if !ok {
			t.Fatalf("wanted a card on the response, got %v", decoded["card"])
		}
		if card["holder"] != "alka" {
			t.Errorf("a pull claims what it takes, got holder %v", card["holder"])
		}
		if card["substate"] != contract.SubstateActive {
			t.Errorf("wanted the card active, got %v", card["substate"])
		}
		if _, carried := decoded["legal_moves"]; !carried {
			t.Error("wanted the legal moves the canonical response carries")
		}
	})

	t.Run("the bare form chooses the one state that qualifies", func(t *testing.T) {
		library := newLibrary(t)
		answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"pull","arguments":{"actor":"alka"}}}`)
		decoded := payload(t, answer)
		if decoded["outcome"] != contract.OutcomeOK {
			t.Fatalf("bare pull: %v", decoded)
		}
		card, ok := decoded["card"].(map[string]any)
		if !ok {
			t.Fatalf("the bare form should have taken the one card standing ready, got %v", decoded["card"])
		}
		// Naming the state is what ties the subtest to its title: the fixture
		// declares Intake and Doing, and only Doing qualifies, so a head that
		// pulled into whichever state it saw last would pass without it.
		if card["state_title"] != "Doing" {
			t.Errorf("the one state that qualifies is Doing, got %v", card["state_title"])
		}
	})

	t.Run("the no-claim marker crosses the boundary", func(t *testing.T) {
		library := newLibrary(t)
		answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"pull","arguments":{"actor":"alka","state":"doing","no-claim":true}}}`)
		decoded := payload(t, answer)
		if decoded["outcome"] != contract.OutcomeOK {
			t.Fatalf("pull --no-claim: %v", decoded)
		}
		card, ok := decoded["card"].(map[string]any)
		if !ok {
			t.Fatalf("wanted a card on the response, got %v", decoded["card"])
		}
		// This is the assertion the marker was silently failing. Without the
		// case in assignMarker the request reaches the library with NoClaim
		// false, the pull claims the card, and the holder below is alka.
		if card["holder"] != nil && card["holder"] != "" {
			t.Errorf("--no-claim asked for no claim and the card came back held by %v", card["holder"])
		}
		if card["substate"] != contract.SubstateReady {
			t.Errorf("wanted the card left ready, got %v", card["substate"])
		}
	})

	t.Run("a refusal carries its name across", func(t *testing.T) {
		library := newLibrary(t)
		answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"pull","arguments":{"actor":"alka","state":"intake"}}}`)
		decoded := payload(t, answer)
		if decoded["outcome"] != contract.OutcomeRefused {
			t.Fatalf("a pull into the first state should refuse, got %v", decoded)
		}
		if decoded["refusal"] != contract.NoUpstream {
			t.Errorf("wanted %s, got %v", contract.NoUpstream, decoded["refusal"])
		}
	})
}
