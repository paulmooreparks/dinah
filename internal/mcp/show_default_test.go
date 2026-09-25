package mcp

import (
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestTheShowToolDefaultsToTheNarrowShape asserts dinah-543 criterion 8: the
// show tool, called through the JSON-RPC surface with neither fields nor
// all, answers a detail carrying the same four members the CLI's bare call
// carries.
func TestTheShowToolDefaultsToTheNarrowShape(t *testing.T) {
	library := newLibrary(t)
	if response := library.Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: "fx-1", Text: "a remark worth keeping"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %s %s", response.Outcome, response.Refusal)
	}

	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"show","arguments":{"actor":"alka","card":"fx-1"}}}`)
	decoded := payload(t, answer)
	detail, ok := decoded["detail"].(map[string]any)
	if !ok {
		t.Fatalf("the answer carries no detail member: %v", decoded)
	}
	if _, carried := detail["comments"]; carried {
		t.Errorf("the bare call carried comments, wanted none: %v", detail)
	}
	if _, carried := detail["path"]; carried {
		t.Errorf("the bare call carried path, wanted none: %v", detail)
	}
	withheld := stringsOf(t, detail["withheld"])
	if want := []string{"comments", "path"}; len(withheld) != len(want) || withheld[0] != want[0] || withheld[1] != want[1] {
		t.Errorf("withheld is %v, wanted %v", withheld, want)
	}
}

// TestTheShowToolAllCarriesEveryMember asserts dinah-543 criteria 9 and 11:
// the show tool, called through the JSON-RPC surface with all: true, answers
// a detail whose comments carry non-empty bodies and leaves nothing
// withheld. It is what proves the "all" marker really reaches
// verb.Request.All through request2Args and answer.Build, rather than being
// dropped the way an unrecognised marker is (see assignMarker's doc comment
// in internal/answer/build.go). The accepting case beside it, the same tool and the same card
// with neither fields nor all, is the bare call criterion 8 above already
// covers; it is cited here rather than repeated.
func TestTheShowToolAllCarriesEveryMember(t *testing.T) {
	library := newLibrary(t)
	const remark = "a remark worth keeping"
	if response := library.Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: "fx-1", Text: remark}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %s %s", response.Outcome, response.Refusal)
	}

	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"show","arguments":{"actor":"alka","card":"fx-1","all":true}}}`)
	decoded := payload(t, answer)
	detail, ok := decoded["detail"].(map[string]any)
	if !ok {
		t.Fatalf("the answer carries no detail member: %v", decoded)
	}
	comments, ok := detail["comments"].([]any)
	if !ok || len(comments) != 1 {
		t.Fatalf("wanted the one comment the fixture holds, got %v", detail["comments"])
	}
	first, ok := comments[0].(map[string]any)
	if !ok {
		t.Fatalf("the comment entry is %T, wanted an object", comments[0])
	}
	if body, _ := first["body"].(string); body != remark {
		t.Errorf("all: true left the comment body %q, wanted %q", body, remark)
	}
	if _, withheld := decoded["withheld"]; withheld {
		t.Errorf("all: true announced withheld, wanted nothing left out: %v", decoded)
	}
	if _, carried := detail["withheld"]; carried {
		t.Errorf("all: true announced withheld, wanted nothing left out: %v", detail)
	}
}
