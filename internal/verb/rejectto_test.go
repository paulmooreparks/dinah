package verb

import (
	"testing"
)

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
