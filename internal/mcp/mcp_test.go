package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// definition is the bench this head's tests are served over.
const definition = `{
  "profile": "dinah-core/0.7",
  "title": "Fixture",
  "instructions": "Standing text.\n",
  "columns": [
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
	// The card is carried to the doing station, because no owner takes work up
	// at an intake column and the claims below would be refused there.
	moved := library.Do(&verb.Request{Verb: verb.Move, Actor: "alka", Card: "fx-1", Column: "doing"})
	if moved.Outcome != contract.OutcomeOK {
		t.Fatalf("move: %s %s", moved.Outcome, moved.Refusal)
	}
	// A second card stands ready in the intake column, which is what the pull
	// tests take and what leaves the first card claimable where it stands.
	if response := library.Add(&verb.Request{Verb: "add", Actor: "alka", Title: "A waiting card"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("add the waiting card: %s %s", response.Outcome, response.Refusal)
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
	return askUnderRoot(t, library.Bench.Root, library, line)
}

// askUnderRoot is ask for a test that names the root the head is bounded by,
// rather than taking the workbench's own directory as the root. Both go
// through one Serve call, so a test naming a root reads its answer the way
// every other test reads one.
func askUnderRoot(t *testing.T, root string, library *verb.Library, line string) *response {
	t.Helper()
	out := &strings.Builder{}
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

// TestToolSurfaceIsTheProjection asserts that the head exposes every tool the
// surface declares and no other, that each input schema is generated from the
// same parameter list the cli head composes its syntax from, and that the
// commands bound to a shell and a filesystem get no tool.
//
// The count below is the suite's own record of how wide the surface is, and it
// moves whenever a card adds a command or takes one away. Anything outside
// this package that wants to know how many tools the head serves reads this
// number rather than carrying its own copy, because a copy freezes a set that
// grows and is stale the next time a command lands.
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
	if len(listed.Tools) != 34 {
		t.Errorf("wanted thirty-four tools, got %d", len(listed.Tools))
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
	for _, wanted := range []string{"claim", "move", "pull", "release", "block", "unblock", "add_card", "list_cards", "next_card", "query", "workbench", "workbenches", "workstream", "join_workstream", "leave_workstream"} {
		if !names[wanted] {
			t.Errorf("the surface is missing the tool %s", wanted)
		}
	}
	for absent := range toolExemptions {
		if names[absent] {
			t.Errorf("%s is exempted from this head and is served as a tool anyway", absent)
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
		`{"name":"columns","arguments":{"actor":"alka"}}`,
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

// TestTheInstructionsListAgreesWithWhereTheCardIsStanding is dinah-273 AC-42.
// The instructions tool is the one a caller reaches for precisely to learn
// what it may do where the card is standing, so a written-out list here sends
// the reader into the refusal it came to avoid. The list is read off the real
// tool answer and held against the act, for a card at a station, for a card at
// an intake column, and for the bare column itself.
func TestTheInstructionsListAgreesWithWhereTheCardIsStanding(t *testing.T) {
	cases := []struct {
		name      string
		arguments string
		wantClaim bool
		wantPull  bool
	}{
		{name: "a card at a station", arguments: `{"actor":"alka","card":"fx-1"}`, wantClaim: true},
		{name: "a card at an intake column", arguments: `{"actor":"alka","card":"fx-2"}`, wantPull: true},
		{name: "the intake column itself", arguments: `{"actor":"alka","card":"intake"}`, wantPull: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			library := newLibrary(t)
			answer := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"instructions","arguments":`+c.arguments+`}}`))
			offered := stringsOf(t, answer["affordances"])
			if got := slices.Contains(offered, verb.Claim); got != c.wantClaim {
				t.Errorf("the list %v offers claim: %t, wanted %t", offered, got, c.wantClaim)
			}
			if got := slices.Contains(offered, verb.Pull); got != c.wantPull {
				t.Errorf("the list %v offers pull: %t, wanted %t", offered, got, c.wantPull)
			}
			for _, name := range offered {
				if _, served := commandTool[name]; served {
					t.Errorf("the list %v names %s, which is a command here and not a tool", offered, name)
				}
			}
		})
	}
}

// stringsOf reads a decoded affordances member as the list of names it is.
func stringsOf(t *testing.T, member any) []string {
	t.Helper()
	raw, ok := member.([]any)
	if !ok {
		t.Fatalf("wanted a list of affordances, got %v", member)
	}
	names := make([]string, 0, len(raw))
	for _, entry := range raw {
		name, ok := entry.(string)
		if !ok {
			t.Fatalf("wanted an affordance name, got %v", entry)
		}
		names = append(names, name)
	}
	return names
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
	if instructions["column"] != "Doing text.\n" {
		t.Errorf("column layer: got %v", instructions["column"])
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
	for _, text := range []string{"", " ", "state:ready", "holder:nobody"} {
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
		want := []string{"status", "columns", "list_cards", "next_card"}
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
	want := []string{"status", "columns", "list_cards", "next_card"}
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

// TestEveryDeclaredParameterReachesItsDeclaredField asserts that every
// parameter a tool advertises in its schema lands on the exact verb.Request
// field the table declares for it, and that a parameter declaring no field
// lands nowhere at all.
//
// The schema is generated from verb.Params, and the assignment behind it is a
// pair of hand-written switches in this package. A parameter can therefore be
// offered to every agent reading the surface, be looked up by name on the way
// in, and then fall off the end of a switch that has no case for it. Nothing
// fails: the call succeeds and the argument is discarded, so an agent asking
// for one thing is silently given another. `no-claim` shipped that way and is
// the reason this check exists.
//
// The check used to ask only whether the request came back different from an
// empty one, which passed a parameter that landed in the wrong field. It now
// reads param.Field, drives a sentinel of that field's own type through
// request2Args, and requires that field to hold the sentinel and every other
// field to be untouched. Two defects are caught rather than one: a parameter
// dropped on the floor, and a parameter that collides with a field it was
// never meant to fill.
//
// A parameter whose Field is empty is checked in the other direction. The
// declaration says it reaches no field, so the check builds the request and
// requires it to stay at its zero value, which is what makes "nothing, on
// purpose" different from "something, lost on the way".
func TestEveryDeclaredParameterReachesItsDeclaredField(t *testing.T) {
	// A parameter this check found landing nowhere, which is a defect rather
	// than a decision and is tracked on its own card. An entry here does not
	// skip the parameter. The check still builds the request and requires the
	// defect to still be present, so repairing the defect reddens this test
	// and the entry has to be deleted along with it.
	knownDefect := map[string]string{
		"attach.description": "dinah-222: assignValue has no case for it, so an attachment made over this head is created with no description",
	}
	// An entry naming a parameter the table no longer declares is looked up by
	// nothing below, so it would sit here unread while reading as coverage.
	// The keys are struck off as they are met and whatever is left over is
	// reported, which is what stops a repaired or renamed parameter leaving a
	// tracked defect behind it.
	unmet := map[string]bool{}
	for named := range knownDefect {
		unmet[named] = true
	}
	checked := 0
	for _, entry := range tools {
		for _, param := range verb.Params(entry.command) {
			named := entry.command + "." + param.Name
			delete(unmet, named)
			empty := request2Args(entry.command, map[string]any{})
			argument, want := sentinelFor(t, entry.name, param, empty)
			if argument == nil {
				continue
			}
			built := request2Args(entry.command, map[string]any{param.Name: argument})
			if param.Field == "" {
				checked++
				if !reflect.DeepEqual(built, empty) {
					t.Errorf("%s: %q declares no request field and this head puts it on one anyway, so the declaration and the code disagree",
						entry.name, param.Name)
				}
				continue
			}
			if tracked, defective := knownDefect[named]; defective {
				if !reflect.DeepEqual(built, empty) {
					t.Errorf("%s: %q now reaches the request, so the exemption has outlived its defect and belongs deleted (%s)",
						entry.name, param.Name, tracked)
				}
				continue
			}
			checked++
			assertOnlyFieldChanged(t, entry.name, param, built, empty, want)
		}
	}
	stale := make([]string, 0, len(unmet))
	for named := range unmet {
		stale = append(stale, named)
	}
	for _, named := range sorted(stale) {
		t.Errorf("%q is tracked as a known defect and no tool declares a parameter by that name, so the entry outlived the parameter it excused (%s)",
			named, knownDefect[named])
	}
	if checked == 0 {
		t.Fatal("no tool declared a parameter, so this check read nothing")
	}
}

// sentinelFor returns the argument to send for one parameter and the value its
// declared field should hold afterwards, both derived from that field's own Go
// type rather than from the parameter's spelling. A parameter declaring no
// field gets an argument of the type its Marker mark implies and no expected
// value, since there is nothing for it to land in.
//
// A field type this check cannot drive fails the test rather than being
// skipped, because a silently skipped parameter is the outcome the whole check
// exists to prevent. Such a parameter comes back with a nil argument, which is
// the caller's signal to move on to the next one.
func sentinelFor(t *testing.T, tool string, param verb.Param, empty *verb.Request) (any, any) {
	t.Helper()
	if param.Field == "" {
		if param.Marker {
			return true, nil
		}
		return "sentinel-value", nil
	}
	field := reflect.ValueOf(empty).Elem().FieldByName(param.Field)
	if !field.IsValid() {
		t.Errorf("%s: %q declares the request field %s and verb.Request has no such field", tool, param.Name, param.Field)
		return nil, nil
	}
	switch {
	case field.Type() == reflect.TypeOf(time.Duration(0)):
		return "8h", 8 * time.Hour
	case field.Kind() == reflect.Bool:
		return true, true
	case field.Kind() == reflect.String:
		return "sentinel-value", "sentinel-value"
	}
	t.Errorf("%s: %q declares the request field %s, whose type %s this check does not know how to drive",
		tool, param.Name, param.Field, field.Type())
	return nil, nil
}

// assertOnlyFieldChanged requires the built request to carry the sentinel in
// the parameter's declared field and to match the empty request everywhere
// else, which is what tells a landing apart from a collision.
func assertOnlyFieldChanged(t *testing.T, tool string, param verb.Param, built, empty *verb.Request, want any) {
	t.Helper()
	builtValue := reflect.ValueOf(built).Elem()
	emptyValue := reflect.ValueOf(empty).Elem()
	for i := 0; i < builtValue.NumField(); i++ {
		name := builtValue.Type().Field(i).Name
		got := builtValue.Field(i).Interface()
		if name == param.Field {
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: the schema offers %q and request2Args left %s holding %v rather than the value sent, so an agent sending it is silently ignored",
					tool, param.Name, name, got)
			}
			continue
		}
		if !reflect.DeepEqual(got, emptyValue.Field(i).Interface()) {
			t.Errorf("%s: %q declares the request field %s and also changed %s, so it collides with a field it was never meant to fill",
				tool, param.Name, param.Field, name)
		}
	}
}

// TestEveryWorkbenchesParameterChangesTheAnswer asserts that each parameter the
// workbenches command declares actually reaches the one tool that is answered
// ahead of the table lookup.
//
// workbenches is dispatched by call before request2Args ever runs, so the
// check above cannot see it: the handler takes the root and nothing else. That
// is how a schema could come to advertise a path and a depth bound the handler
// never reads. The assertion is call-shaped rather than reflective, and it
// iterates verb.Params rather than a count typed out here, so it reads nothing
// today and arms itself the moment the command declares its first parameter.
func TestEveryWorkbenchesParameterChangesTheAnswer(t *testing.T) {
	declared := verb.Params("workbenches")
	if len(declared) == 0 {
		t.Skip("workbenches declares no parameters yet, so there is nothing for the handler to read")
	}
	library := newLibrary(t)
	bare := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbenches","arguments":{}}}`))
	plain, err := json.Marshal(bare)
	if err != nil {
		t.Fatalf("marshal the bare answer: %v", err)
	}
	for _, param := range declared {
		var argument any = "sentinel-value"
		if param.Marker {
			argument = true
		}
		encoded, err := json.Marshal(map[string]any{param.Name: argument})
		if err != nil {
			t.Fatalf("marshal the arguments for %q: %v", param.Name, err)
		}
		call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbenches","arguments":` + string(encoded) + `}}`
		named, err := json.Marshal(payload(t, ask(t, library, call)))
		if err != nil {
			t.Fatalf("marshal the answer to %q: %v", param.Name, err)
		}
		if string(named) == string(plain) {
			t.Errorf("workbenches: the schema offers %q and the handler answers a call naming it exactly as it answers a call naming nothing, so the argument is discarded",
				param.Name)
		}
	}
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
// The fixture's flow is Intake then Doing, with one ready card standing in
// Intake and one already carried to Doing, so a pull into Doing takes the
// waiting card and a pull into Intake has nothing before it to take from.
func TestPullIsDrivenThroughTheHead(t *testing.T) {
	t.Run("the named form takes the card and claims it", func(t *testing.T) {
		library := newLibrary(t)
		answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"pull","arguments":{"actor":"alka","column":"doing"}}}`)
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
		if card["state"] != contract.StateActive {
			t.Errorf("wanted the card active, got %v", card["state"])
		}
		if _, carried := decoded["legal_moves"]; !carried {
			t.Error("wanted the legal moves the canonical response carries")
		}
	})

	t.Run("the bare form chooses the one column that qualifies", func(t *testing.T) {
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
		// Naming the column is what ties the subtest to its title: the fixture
		// declares Intake and Doing, and only Doing qualifies, so a head that
		// pulled into whichever column it saw last would pass without it.
		if card["column_title"] != "Doing" {
			t.Errorf("the one column that qualifies is Doing, got %v", card["column_title"])
		}
	})

	t.Run("the no-claim marker crosses the boundary", func(t *testing.T) {
		library := newLibrary(t)
		answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"pull","arguments":{"actor":"alka","column":"doing","no-claim":true}}}`)
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
		if card["state"] != contract.StateReady {
			t.Errorf("wanted the card left ready, got %v", card["state"])
		}
	})

	t.Run("a refusal carries its name across", func(t *testing.T) {
		library := newLibrary(t)
		answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"pull","arguments":{"actor":"alka","column":"intake"}}}`)
		decoded := payload(t, answer)
		if decoded["outcome"] != contract.OutcomeRefused {
			t.Fatalf("a pull into the first column should refuse, got %v", decoded)
		}
		if decoded["refusal"] != contract.NoUpstream {
			t.Errorf("wanted %s, got %v", contract.NoUpstream, decoded["refusal"])
		}
	})
}

// TestARootRemovedSinceTheServerStartedRefusesOutsideRoot holds the end of
// AC-17's containment clauses at the layer where the refusal exists.
//
// bench.PathUnderRoot answers a failed containment walk with the error the
// filesystem gave it and composes no refusal, because internal/bench does not
// know which caller is asking and a package that renders for one caller stops
// being reusable by the next. resolveLibrary is the caller that turns that
// error into dinah.outside-root, and until now nothing drove the two together:
// the bench tests assert the error and the surface tests reach the refusal
// through a path that is merely outside the root rather than one the walk
// could not settle.
//
// Removing the root after the server started is the failure a long-running
// head actually meets, and it reaches resolveLibrary's error branch rather
// than its not-contained branch, so the two branches are held apart here the
// way they are in internal/bench.
func TestARootRemovedSinceTheServerStartedRefusesOutsideRoot(t *testing.T) {
	library := newLibrary(t)
	workbench := library.Bench.Root
	root := filepath.Dir(workbench)
	if err := os.RemoveAll(root); err != nil {
		t.Skipf("the platform would not remove the root under the open workbench: %v", err)
	}

	refused := payload(t, askUnderRoot(t, root, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbench","arguments":{"actor":"alka","workbench":"`+filepath.ToSlash(workbench)+`"}}}`))
	if refused["outcome"] != contract.OutcomeRefused {
		t.Fatalf("a workbench under a removed root should be refused, got %v", refused)
	}
	if refused["refusal"] != contract.OutsideRoot {
		t.Errorf("the refusal a failed containment walk composes: wanted %s, got %v", contract.OutsideRoot, refused["refusal"])
	}
	named, ok := refused["context"].(map[string]any)
	if !ok {
		t.Fatalf("an outside-root refusal carries no context member: %v", refused)
	}
	if named["root"] != root {
		t.Errorf("the refusal should name the root it was bounded by: wanted %q, got %v", root, named["root"])
	}
}

// askUnboundedStream is askUnderRoot for a test that has to see the server
// answer more than once, which is what proves Serve's read loop survived a
// refusal rather than stopping on it. One Serve call takes every line and the
// answers come back in order, so a caller reads the second answer the same way
// every other test reads its only one.
func askUnboundedStream(t *testing.T, root string, library *verb.Library, lines ...string) []*response {
	t.Helper()
	out := &strings.Builder{}
	if err := Serve(root, library, map[string]*verb.Library{}, strings.NewReader(strings.Join(lines, "\n")+"\n"), out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var answers []*response
	for _, encoded := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(encoded) == "" {
			continue
		}
		answer := &response{}
		if err := json.Unmarshal([]byte(encoded), answer); err != nil {
			t.Fatalf("decode %q: %v", encoded, err)
		}
		answers = append(answers, answer)
	}
	return answers
}

// instructionsOf reads the working agreement out of an initialize answer, so
// the tests that hold that text apart read it from one place.
func instructionsOf(t *testing.T, answer *response) string {
	t.Helper()
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var result struct {
		Instructions string `json:"instructions"`
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return result.Instructions
}

// TestResolveLibraryAdmitsAnyPathWhenRootIsEmpty is dinah-307 AC-2. A server
// given no root carries no boundary, so a call naming a real workbench by
// absolute path resolves it rather than being refused outside-root.
//
// This is the call site the card exists for. Before dinah-307 the containment
// predicate answered false for an empty root, so a server that started
// unbounded would have refused every workbench named to it, naming an empty
// root in the refusal's own detail.
func TestResolveLibraryAdmitsAnyPathWhenRootIsEmpty(t *testing.T) {
	elsewhere := newLibrary(t).Bench.Root

	library, refusal := resolveLibrary("", nil, nil, &verb.Request{Verb: "status", Actor: "alka"}, elsewhere)
	if refusal != nil {
		t.Fatalf("an unbounded server refused a workbench named by absolute path: %s %s", refusal.Name, refusal.Detail)
	}
	if library == nil {
		t.Fatal("the resolution carried neither a library nor a refusal")
	}
	if library.Bench.Root != elsewhere {
		t.Errorf("the resolution opened %q, wanted the workbench the call named, %q", library.Bench.Root, elsewhere)
	}
}

// TestResolveLibraryRefusesNoWorkbenchFoundWithNoRootAndNoDefault is dinah-307
// AC-3. An empty root widens what a named candidate may be; it changes nothing
// about a call that names nothing and has no default to fall back on.
func TestResolveLibraryRefusesNoWorkbenchFoundWithNoRootAndNoDefault(t *testing.T) {
	library, refusal := resolveLibrary("", nil, nil, &verb.Request{Verb: "status", Actor: "alka"}, "")
	if library != nil {
		t.Fatalf("a call naming no workbench against a server carrying no default resolved %q", library.Bench.Root)
	}
	if refusal == nil {
		t.Fatal("the resolution carried neither a library nor a refusal")
	}
	if refusal.Name != contract.NoWorkbenchFound {
		t.Errorf("the refusal: wanted %s, got %s", contract.NoWorkbenchFound, refusal.Name)
	}
}

// TestWorkingAgreementNamesNoBoundaryWhenUnbounded is dinah-307 AC-6. The
// working agreement an unbounded, default-less server serves has to say it
// carries no boundary. The old key spliced the root into the sentence, so an
// empty root rendered "under ;", which is not a sentence and names nothing.
func TestWorkingAgreementNamesNoBoundaryWhenUnbounded(t *testing.T) {
	instructions := instructionsOf(t, askUnderRoot(t, "", nil, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))

	if !strings.Contains(instructions, msg.For(msg.Base).T("mcp.reach.nodefault.unbounded")) {
		t.Errorf("the working agreement of an unbounded server carries no unbounded reach paragraph: %q", instructions)
	}
	if strings.Contains(instructions, "under ;") {
		t.Errorf("the working agreement spliced an empty root into a sentence: %q", instructions)
	}
}

// TestWorkingAgreementNamesNoBoundaryWhenUnboundedWithADefault is dinah-307
// AC-9, the combination that did not exist before this card: a server with a
// default workbench and no root at all. It must name the default and still
// claim no boundary, and it must not go on promising that workbenches lists
// what sits under the root, because with no root that tool answers nothing.
func TestWorkingAgreementNamesNoBoundaryWhenUnboundedWithADefault(t *testing.T) {
	library := newLibrary(t)
	instructions := instructionsOf(t, askUnderRoot(t, "", library, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))

	want := msg.For(msg.Base).T("mcp.reach.unbounded", "title", library.Bench.Title)
	if !strings.Contains(instructions, want) {
		t.Errorf("the working agreement of an unbounded server carrying a default: wanted %q in %q", want, instructions)
	}
	if !strings.Contains(instructions, library.Bench.Title) {
		t.Errorf("the working agreement does not name the default workbench it serves: %q", instructions)
	}
	if strings.Contains(instructions, "under ;") {
		t.Errorf("the working agreement spliced an empty root into a sentence: %q", instructions)
	}
	if strings.Contains(instructions, "The workbenches tool lists the workbenches under the root") {
		t.Errorf("the working agreement still promises an enumeration an unbounded server cannot answer: %q", instructions)
	}
}

// TestWorkbenchesToolRefusesCleanlyWhenUnbounded is dinah-307 AC-7. The one
// tool that takes no workbench needs a root to search, so an unbounded server
// refuses it by name rather than answering an empty list, which would assert
// that no workbench exists anywhere. The second request holds the other half:
// the refusal travels on the response and does not end the session.
func TestWorkbenchesToolRefusesCleanlyWhenUnbounded(t *testing.T) {
	answers := askUnboundedStream(t, "", nil,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbenches","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping","params":{}}`)

	if len(answers) != 2 {
		t.Fatalf("wanted an answer to each of the two requests, got %d", len(answers))
	}
	if answers[0].Error == nil {
		t.Fatalf("workbenches against an unbounded server answered rather than refused: %+v", answers[0].Result)
	}
	if !strings.HasPrefix(answers[0].Error.Message, contract.NoWorkbenchFound) {
		t.Errorf("the refusal message: wanted one leading with %s, got %q", contract.NoWorkbenchFound, answers[0].Error.Message)
	}
	if answers[1].Error != nil {
		t.Errorf("the server stopped answering after the refusal: %+v", answers[1].Error)
	}
}

// TestWorkbenchesToolRefusesEvenWithADefaultWhenUnbounded is dinah-307 AC-10.
// A default library is what answers a call naming no workbench; it is not a
// directory to search. So the enumeration refuses for the same reason whether
// or not the server carries one, rather than quietly listing the default.
func TestWorkbenchesToolRefusesEvenWithADefaultWhenUnbounded(t *testing.T) {
	library := newLibrary(t)
	answers := askUnboundedStream(t, "", library,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbenches","arguments":{}}}`)

	if len(answers) != 1 {
		t.Fatalf("wanted one answer, got %d", len(answers))
	}
	if answers[0].Error == nil {
		t.Fatalf("workbenches against an unbounded server carrying a default answered rather than refused: %+v", answers[0].Result)
	}
	if !strings.HasPrefix(answers[0].Error.Message, contract.NoWorkbenchFound) {
		t.Errorf("the refusal message: wanted one leading with %s, got %q", contract.NoWorkbenchFound, answers[0].Error.Message)
	}
}

// TestWorkbenchesListsTheWorkbenchInTheRootsOwnContainer is dinah-312 AC-5.
// The workbenches tool answers off bench.Enumerate, and the enumeration used
// to test a root's children and never the root itself, so a DINAH_MCP_ROOT
// pointing at the directory whose own .dinah holds the boards listed nothing
// at all. An empty list from that call says no workbench exists anywhere under
// the root, which is the answer a caller acts on.
//
// The workbench here is written by verb.Init rather than by the bare
// bench.Instantiate call newLibrary makes, because Init writes the .dinah
// container and a bare anchor is the one shape this defect never hid in.
func TestWorkbenchesListsTheWorkbenchInTheRootsOwnContainer(t *testing.T) {
	root := t.TempDir()
	written, err := verb.Init(root, "rt", "alka", "", "", "")
	if err != nil {
		t.Fatalf("init a workbench under %s: %v", root, err)
	}

	answer := askUnderRoot(t, root, newLibrary(t),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbenches","arguments":{}}}`)
	decoded := payload(t, answer)
	rows, ok := decoded["workbenches"].([]any)
	if !ok {
		t.Fatalf("the answer carries no workbenches array: %+v", decoded)
	}
	var listed []string
	for _, row := range rows {
		entry, ok := row.(map[string]any)
		if !ok {
			t.Fatalf("a workbenches row is not an object: %+v", row)
		}
		path, _ := entry["path"].(string)
		listed = append(listed, path)
	}
	if len(listed) != 1 || listed[0] != written {
		t.Errorf("workbenches under %s listed %v, wanted the one anchor directory %q", root, listed, written)
	}
}

// TestWorkbenchesRefusesARootWithAnUnreadableAnchor is dinah-312 AC-8's other
// half: the workbenches tool answers bench.Enumerate's contract.UnreadableBench
// refusal, served by askUnderRoot exactly as the recognized-container case
// above is, when DINAH_MCP_ROOT points at a directory whose own workbench.md
// exists and cannot be read.
//
// The unreadable anchor is a directory sitting at the workbench.md path
// rather than a regular file. bench.Exists uses os.Stat, which reports a
// directory as present, and os.ReadFile (bench.ReadText's implementation)
// refuses to read a directory as text on every platform this suite runs on,
// confirmed on this machine: "read ...workbench.md: Incorrect function." on
// Windows. That makes the anchor genuinely unreadable without depending on a
// permission bit, which does not reliably block a file's own owner from
// reading it on Windows. internal/mcp cannot reach internal/bench's unexported
// readAnchorContent test seam across the package boundary, so this is the
// mechanism available at this layer; internal/bench's own AC-8 test
// (TestEnumerateRefusesARootWithAnUnreadableAnchor) uses the seam instead,
// since it is in the same package.
//
// answerWorkbenches returns bench.Enumerate's error unwrapped, and mcp.call's
// dispatch turns that into a JSON-RPC transport error (codeInvalidParams)
// rather than a refusal payload, because the workbenches tool is served ahead
// of the resolveLibrary step that composes answerRefusal's payload shape for
// every other tool. So this assertion reads answer.Error directly instead of
// going through payload(), which fatals on a non-nil answer.Error.
func TestWorkbenchesRefusesARootWithAnUnreadableAnchor(t *testing.T) {
	root := t.TempDir()
	anchorPath := filepath.Join(root, bench.WorkbenchAnchor)
	if err := os.MkdirAll(anchorPath, 0o755); err != nil {
		t.Fatalf("make %s a directory: %v", anchorPath, err)
	}

	answer := askUnderRoot(t, root, newLibrary(t),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbenches","arguments":{}}}`)
	if answer.Error == nil {
		t.Fatalf("workbenches under %s: wanted a refusal, got a clean answer", root)
	}
	if answer.Error.Code != codeInvalidParams {
		t.Errorf("refusal transport code: wanted %d, got %d (%s)", codeInvalidParams, answer.Error.Code, answer.Error.Message)
	}
	if !strings.Contains(answer.Error.Message, contract.UnreadableBench) {
		t.Errorf("refusal message: wanted it to name %s, got %q", contract.UnreadableBench, answer.Error.Message)
	}
	if !strings.Contains(answer.Error.Message, anchorPath) {
		t.Errorf("refusal message: wanted it to name the unreadable anchor %q, got %q", anchorPath, answer.Error.Message)
	}
}

// TestTheCheckToolSaysWhetherItFoundAnything is dinah-346 AC-6. MCP carries no
// exit code, so before this member a caller had to guess from whether the
// findings array came back empty, null or absent, which is the guesswork
// dinah-281 already ruled out once for a value of the same shape. The clean
// case and the dirty one are asserted in one test because the member is only
// worth anything if it distinguishes them.
func TestTheCheckToolSaysWhetherItFoundAnything(t *testing.T) {
	library := newLibrary(t)
	clean := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"check","arguments":{}}}`))
	outcome, carried := clean["outcome"]
	if !carried {
		t.Fatalf("the check tool's answer carries no outcome member: %v", clean)
	}
	if outcome != contract.ReadOK {
		t.Errorf("a clean workbench answers outcome %v, wanted %q, over findings %v", outcome, contract.ReadOK, clean["findings"])
	}

	// A directory under the cards root with no anchor file in it, which is
	// the simplest defect the checker names and one no repair here touches.
	stray := filepath.Join(library.Bench.CardsRoot(), "f00000000001")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dirty := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"check","arguments":{}}}`))
	if dirty["outcome"] != contract.ReadFindings {
		t.Errorf("a workbench carrying a defect answers outcome %v, wanted %q, over findings %v", dirty["outcome"], contract.ReadFindings, dirty["findings"])
	}
	found, ok := dirty["findings"].([]any)
	if !ok || len(found) == 0 {
		t.Errorf("the answer reports findings %v, and the outcome member is worth nothing unless the array agrees with it", dirty["findings"])
	}
}
