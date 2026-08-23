package mcp

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
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

	// The two heads are compared on an answer that carries something in every
	// collection the projection could drop. Compared on the empty answer this
	// test used to build, events, cards, gone and unreadable are absent from
	// both sides, and a head that dropped or reshaped any of the four agrees
	// with the library byte for byte anyway.
	library = benchWithSomethingInEveryCollection(t, library)
	carrying := allCoveringCursor(t, token)

	encoded, err := json.Marshal(carrying)
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
	direct, err := library.Changes(&verb.Request{Verb: "changes", Actor: "alka", Since: carrying})
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
	// The byte equality above means what its comment claims only if the value
	// compared had something in it, so the four collections are read back off
	// the decoded answer rather than assumed.
	if !toolSide.Changed {
		t.Fatal("the fixture moved the board and the answer reports it unchanged")
	}
	for name, count := range map[string]int{
		"events":     len(toolSide.Events),
		"cards":      len(toolSide.Cards),
		"gone":       len(toolSide.Gone),
		"unreadable": len(toolSide.Unreadable),
	} {
		if count == 0 {
			t.Errorf("the compared answer carries no %s, so the projection of that member is untested", name)
		}
	}
}

// benchWithSomethingInEveryCollection moves the fixture's board until one
// checkpoint has a reason to fill each of the four collections: a card that
// stays and carries a history, a card that is destroyed, and a card whose
// journal will not parse. It returns a library reopened over the result.
func benchWithSomethingInEveryCollection(t *testing.T, library *verb.Library) *verb.Library {
	t.Helper()
	filed := func(title string) *verb.CardView {
		response := library.Add(&verb.Request{Verb: "add", Actor: "alka", Title: title})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("add %q: %s %s", title, response.Outcome, response.Refusal)
		}
		return response.Card
	}
	filed("A card that stays")
	destroyed := filed("A card that is destroyed")
	torn := filed("A card whose history will not parse")

	root := library.Bench.Root
	library = newLibraryAt(t, root)
	if response := library.Delete(&verb.Request{Verb: "delete", Actor: "alka", Ref: destroyed.ID, Confirm: true}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("delete: %s %s", response.Outcome, response.Refusal)
	}

	// A malformed line with a good line written after it, which is the shape
	// bench.ReadJournal refuses outright rather than tolerating.
	journal := filepath.Join(library.Bench.CardsRoot(), torn.ID, bench.JournalName)
	handle, err := os.OpenFile(journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", journal, err)
	}
	if _, err := handle.WriteString("this line is not JSON\n"); err != nil {
		t.Fatalf("write %s: %v", journal, err)
	}
	if err := handle.Close(); err != nil {
		t.Fatalf("close %s: %v", journal, err)
	}
	if err := bench.AppendEvent(journal, bench.Event{TS: bench.Stamp(time.Now().UTC()), Event: contract.EventCommented, Actor: "alka"}); err != nil {
		t.Fatalf("append past the bad line: %v", err)
	}
	return newLibraryAt(t, root)
}

// allCoveringCursor takes a minted token and returns one that covers nothing,
// so every line on the board falls after it and the answer is a full one.
//
// A minted cursor names the end of the total order as it stands, and the clock
// this fixture writes by has second resolution, so handing the token straight
// back would make what the answer carried depend on which second the acts
// landed in. Dropping the position and spoiling one digest term makes the call
// report the whole board instead. The token is rebuilt from the one the
// library minted rather than typed out here, so its shape stays the library's.
func allCoveringCursor(t *testing.T, token string) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("decode the minted cursor: %v", err)
	}
	fields := map[string]any{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("read the minted cursor: %v", err)
	}
	delete(fields, "ts")
	delete(fields, "entity")
	delete(fields, "index")
	fields["live"] = "sha256:" + strings.Repeat("0", 64)
	rebuilt, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("rebuild the cursor: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(rebuilt)
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
