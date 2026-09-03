package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// declareLoopLimit writes loop_limit into a column's own anchor and reopens
// the library's workbench, on the pattern declareWaiting already establishes
// for the other column property no tool of this surface sets.
func declareLoopLimit(t *testing.T, library *verb.Library, id, limit string) {
	t.Helper()
	path := filepath.Join(library.Bench.Root, bench.ColumnsDir, id, bench.ColumnAnchor)
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the column anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(string(text))
	fm.Set("loop_limit", limit)
	if err := os.WriteFile(path, []byte(fm.Render(body)), 0o644); err != nil {
		t.Fatalf("write the column anchor: %v", err)
	}
	reopened, err := bench.Open(library.Bench.Root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	library.Bench = reopened
}

// sendCardAroundTheLoop carries fx-1 from the intake column to the doing
// column, back, and forward again, which is one regressive departure from
// doing and leaves the card standing there ready to be claimed.
func sendCardAroundTheLoop(t *testing.T, library *verb.Library) {
	t.Helper()
	for _, column := range []string{"a00000000002", "a00000000001", "a00000000002"} {
		response := library.Do(&verb.Request{Verb: verb.Move, Actor: "alka", Card: "fx-1", Column: column})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("move to %s: %s %s", column, response.Outcome, response.Refusal)
		}
	}
}

// TestTheClaimToolCarriesTheLoopBlock is dinah-364 AC-8. The head projects
// verb.Response rather than remapping it field by field, so the criterion is
// really asking whether that claim still holds for a member added to the
// canonical shape. The comparison is against the library's own answer for the
// identical fixture and the identical call, so a head that grew a hardcoded
// field list would fail here rather than quietly dropping the member.
func TestTheClaimToolCarriesTheLoopBlock(t *testing.T) {
	direct := newLibrary(t)
	declareLoopLimit(t, direct, "a00000000002", "2")
	sendCardAroundTheLoop(t, direct)

	throughHead := newLibrary(t)
	declareLoopLimit(t, throughHead, "a00000000002", "2")
	sendCardAroundTheLoop(t, throughHead)

	expected := direct.Do(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: "fx-1"})
	if expected.Loop == nil {
		t.Fatalf("the library itself served no loop block: %s %s", expected.Outcome, expected.Refusal)
	}
	if expected.Loop.Count != 1 || expected.Loop.Limit != 2 || expected.Loop.AtLimit {
		t.Fatalf("the fixture wanted 1 of 2 and not at the limit, got %+v", expected.Loop)
	}

	answer := ask(t, throughHead, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"claim","arguments":{"actor":"alka","card":"fx-1"}}}`)
	got := payload(t, answer)
	if got["outcome"] != contract.OutcomeOK {
		t.Fatalf("claim: %v", got)
	}
	if _, carried := got["loop"]; !carried {
		t.Fatalf("the member is not spelled loop on the wire: %v", got)
	}
	wanted, err := json.Marshal(fieldOf(t, expected, "loop"))
	if err != nil {
		t.Fatalf("marshal the library's loop: %v", err)
	}
	found, err := json.Marshal(got["loop"])
	if err != nil {
		t.Fatalf("marshal the head's loop: %v", err)
	}
	if string(wanted) != string(found) {
		t.Errorf("loop differs between the heads:\nlibrary: %s\nmcp:     %s", wanted, found)
	}
}

// TestTheMoveToolCarriesTheLoopBlock is the move half of AC-8, and the absent
// half is what says the member is served for the column the card is standing
// in rather than served always.
func TestTheMoveToolCarriesTheLoopBlock(t *testing.T) {
	library := newLibrary(t)
	declareLoopLimit(t, library, "a00000000002", "2")
	sendCardAroundTheLoop(t, library)

	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"move","arguments":{"actor":"alka","card":"fx-1","column":"a00000000001"}}}`)
	got := payload(t, answer)
	if got["outcome"] != contract.OutcomeOK {
		t.Fatalf("move: %v", got)
	}
	if _, carried := got["loop"]; carried {
		t.Errorf("a card landing at a column declaring nothing carried the member: %v", got["loop"])
	}

	back := ask(t, library, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"move","arguments":{"actor":"alka","card":"fx-1","column":"a00000000002"}}}`)
	landed := payload(t, back)
	if landed["outcome"] != contract.OutcomeOK {
		t.Fatalf("move back: %v", landed)
	}
	block, carried := landed["loop"].(map[string]any)
	if !carried {
		t.Fatalf("a card landing at the declaring column carried no loop member: %v", landed)
	}
	if block["count"] != float64(2) || block["limit"] != float64(2) || block["at_limit"] != true {
		t.Errorf("wanted 2 of 2 and at the limit, got %v", block)
	}
}
