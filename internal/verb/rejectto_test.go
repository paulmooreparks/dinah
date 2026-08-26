package verb

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTheStateViewCarriesTheDeclaredRejectTarget is the publication half of
// the declaration. StateView is the one read that enumerates what a state
// declares, and the MCP status tool serves it, so a state carrying reject_to
// and a view silent about it would leave an agent to discover the destination
// from the legal moves of a card it has already moved.
//
// The reference is published as written. A declaration naming no state opens
// the workbench anyway, so the view reports the string rather than dropping
// what it cannot resolve, and the two subtests below pin both halves of that.
func TestTheStateViewCarriesTheDeclaredRejectTarget(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "reject_to", "doing")
	views, err := h.library.States()
	if err != nil {
		t.Fatalf("states: %v", err)
	}
	var declaring, ordinary *StateView
	for i := range views {
		switch views[i].ID {
		case aftercare:
			declaring = &views[i]
		case doing:
			ordinary = &views[i]
		}
	}
	if declaring == nil || ordinary == nil {
		t.Fatalf("the fixture's states did not all come back: %+v", views)
	}
	if declaring.RejectTo != "doing" {
		t.Errorf("the declaring state's view carries reject_to %q, the state declares \"doing\"", declaring.RejectTo)
	}
	if ordinary.RejectTo != "" {
		t.Errorf("a state declaring nothing carries reject_to %q", ordinary.RejectTo)
	}

	raw, err := json.Marshal(declaring)
	if err != nil {
		t.Fatalf("marshalling the view: %v", err)
	}
	if !strings.Contains(string(raw), `"reject_to":"doing"`) {
		t.Errorf("the member is not spelled reject_to on the wire: %s", raw)
	}
	bare, err := json.Marshal(ordinary)
	if err != nil {
		t.Fatalf("marshalling the view: %v", err)
	}
	if strings.Contains(string(bare), "reject_to") {
		t.Errorf("a state declaring nothing publishes the member anyway: %s", bare)
	}
}

// TestTheStateViewPublishesAnUnresolvableRejectTarget pins the decision to
// publish the reference verbatim. Resolving it here would make the view drop
// exactly the value an operator needs to see in order to repair the typo that
// dinah check reports under check.reject-target-unknown.
func TestTheStateViewPublishesAnUnresolvableRejectTarget(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "reject_to", "nowhere at all")
	views, err := h.library.States()
	if err != nil {
		t.Fatalf("states: %v", err)
	}
	for i := range views {
		if views[i].ID != aftercare {
			continue
		}
		if views[i].RejectTo != "nowhere at all" {
			t.Errorf("the view carries reject_to %q, the state declares \"nowhere at all\"", views[i].RejectTo)
		}
		return
	}
	t.Fatalf("the declaring state did not come back: %+v", views)
}

// TestLegalMovesMarksTheDeclaredRejectTarget is dinah-207 AC-6. The marking is
// the whole of what makes the declaration discoverable: the destination was
// always legal, and until now nothing said which of the legal ones the board
// meant when the work here is refused.
func TestLegalMovesMarksTheDeclaredRejectTarget(t *testing.T) {
	t.Run("a declared target marks exactly that row", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "reject_to", "doing")
		ref := h.add("rejected work")
		response := h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: aftercare})
		assertRejectRow(t, response.LegalMoves, doing)
	})

	t.Run("a declared target named by identifier marks the same row", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "reject_to", doing)
		ref := h.add("rejected work")
		response := h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: aftercare})
		assertRejectRow(t, response.LegalMoves, doing)
	})

	t.Run("a state declaring nothing marks no row", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("ordinary work")
		response := h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: aftercare})
		assertNoRejectRow(t, response.LegalMoves, "a state declaring nothing")
	})

	t.Run("a ref naming no state marks no row", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "reject_to", "nowhere")
		ref := h.add("ordinary work")
		response := h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: aftercare})
		assertNoRejectRow(t, response.LegalMoves, "an unresolvable declaration")
	})

	t.Run("a ref naming the declaring state marks no row", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "reject_to", aftercareSlug)
		ref := h.add("ordinary work")
		response := h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: aftercare})
		assertNoRejectRow(t, response.LegalMoves, "a self-naming declaration")
	})
}

// assertRejectRow fails unless exactly one legal move carries the mark and it
// names the state given. Counting the marked rows rather than looking one up
// is what catches an implementation that marks every row, which reading the
// one expected row alone would pass.
func assertRejectRow(t *testing.T, moves []LegalMove, want string) {
	t.Helper()
	if len(moves) == 0 {
		t.Fatal("the response carried no legal moves at all")
	}
	marked := 0
	for _, move := range moves {
		if move.Reject {
			marked++
			if move.State != want {
				t.Errorf("the marked row names %s, wanted %s", move.State, want)
			}
		}
	}
	if marked != 1 {
		t.Errorf("wanted exactly one marked row, got %d of %d", marked, len(moves))
	}
}

// assertNoRejectRow fails when any legal move carries the mark.
func assertNoRejectRow(t *testing.T, moves []LegalMove, what string) {
	t.Helper()
	if len(moves) == 0 {
		t.Fatal("the response carried no legal moves at all")
	}
	for _, move := range moves {
		if move.Reject {
			t.Errorf("%s marked the row naming %s", what, move.State)
		}
	}
}
