package verb

import (
	"encoding/json"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// sendBack carries a card from aftercare to doing and straight back again, as
// the operator, which is one regressive departure from aftercare and the
// setup half of every test below rather than the act under test. Both legs
// are needed: the card has to be standing at aftercare again for the next act
// to depart from it.
func (h *harness) sendBack(ref string) {
	h.t.Helper()
	h.at(ref, doing)
	h.at(ref, aftercare)
}

// TestAClaimCarriesTheLoopStanding is dinah-364 AC-3 for the two mutations.
// The absent half is doing real work: an implementation serving the block
// everywhere would pass the present half on its own, and an agent reading a
// count of zero at a column that declares nothing would be told there is a
// loop to be bounded where there is none.
func TestAClaimCarriesTheLoopStanding(t *testing.T) {
	t.Run("a declaring column serves the count and the limit", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "loop_limit", "2")
		ref := h.ready("looping")
		h.sendBack(ref)

		response := h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
		if response.Loop == nil {
			t.Fatal("wanted a loop block on the claim, got none")
		}
		if response.Loop.Column != aftercareSlug {
			t.Errorf("column: wanted %s, got %s", aftercareSlug, response.Loop.Column)
		}
		if response.Loop.Limit != 2 {
			t.Errorf("limit: wanted 2, got %d", response.Loop.Limit)
		}
		if response.Loop.Count != 1 {
			t.Errorf("count: wanted 1, got %d", response.Loop.Count)
		}
		if response.Loop.AtLimit {
			t.Error("at_limit: wanted false one departure short of the limit")
		}
	})

	t.Run("a column declaring nothing serves no loop member at all", func(t *testing.T) {
		h := newHarness(t)
		ref := h.ready("looping")
		h.sendBack(ref)

		response := h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
		if response.Loop != nil {
			t.Fatalf("wanted no loop block, got %+v", response.Loop)
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(encoded), `"loop"`) {
			t.Errorf("the member survived omitempty on the wire: %s", encoded)
		}
	})

	t.Run("a move serves the same block the claim serves", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "loop_limit", "3")
		ref := h.ready("looping")
		h.sendBack(ref)

		// The block is served for the column the card lands in, not for the
		// one it left, so the move that departs the declaring column carries
		// none and the move that returns to it carries the count that
		// departure just raised.
		response := h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: doing})
		if response.Loop != nil {
			t.Fatalf("a move landing at a column declaring nothing wanted no block, got %+v", response.Loop)
		}
		back := h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: aftercare})
		if back.Loop == nil {
			t.Fatal("a move landing at the declaring column wanted a block, got none")
		}
		if back.Loop.Count != 2 || back.Loop.Limit != 3 || back.Loop.AtLimit {
			t.Errorf("wanted 2 of 3 and not at the limit, got %+v", back.Loop)
		}
	})
}

// TestInstructionsCarryTheLoopStanding is dinah-364 AC-3 for the read. The
// chain served for a column named on its own carries no block, because a
// count belongs to a card and a column named alone brings none.
func TestInstructionsCarryTheLoopStanding(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "loop_limit", "2")
	ref := h.ready("looping")
	h.sendBack(ref)

	served, err := h.library.Instructions(&Request{Verb: "instructions", Actor: "alka", Card: ref})
	if err != nil {
		t.Fatalf("instructions: %v", err)
	}
	if served.Loop == nil {
		t.Fatal("wanted a loop block on the served chain, got none")
	}
	if served.Loop.Count != 1 || served.Loop.Limit != 2 || served.Loop.Column != aftercareSlug {
		t.Errorf("wanted 1 of 2 at %s, got %+v", aftercareSlug, served.Loop)
	}

	byColumn, err := h.library.Instructions(&Request{Verb: "instructions", Actor: "alka", Card: aftercareSlug})
	if err != nil {
		t.Fatalf("instructions by column: %v", err)
	}
	if byColumn.Loop != nil {
		t.Errorf("a column named on its own wanted no block, got %+v", byColumn.Loop)
	}
}

// TestARegressiveMovePastTheLoopLimitIsRefused is dinah-364 AC-4. The
// override half is what makes this the capacity shape rather than a closed
// door, and the two unaffected halves are what keep the limit from binding
// moves it was never about.
func TestARegressiveMovePastTheLoopLimitIsRefused(t *testing.T) {
	t.Run("the move past the limit is refused, naming the departure", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "loop_limit", "1")
		ref := h.ready("looping")
		h.sendBack(ref)

		response := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: doing})
		if response.Outcome != contract.OutcomeRefused {
			t.Fatalf("wanted refused, got %s", response.Outcome)
		}
		if response.Refusal != contract.AtLoopLimit {
			t.Fatalf("wanted %s, got %s", contract.AtLoopLimit, response.Refusal)
		}
		if response.Detail != aftercareSlug {
			t.Errorf("detail: wanted the departure %s, got %s", aftercareSlug, response.Detail)
		}
	})

	t.Run("the operator's override lands the card and is witnessed", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "loop_limit", "1")
		ref := h.ready("looping")
		h.sendBack(ref)

		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: doing, Override: true})
		if got := h.card(ref).Column; got != doing {
			t.Errorf("wanted the card landed at %s, got %s", doing, got)
		}
		events, _, err := bench.ReadJournal(h.card(ref).JournalPath())
		if err != nil {
			t.Fatalf("journal: %v", err)
		}
		last := events[len(events)-1]
		if last.Event != contract.EventMoved {
			t.Fatalf("wanted the move last in the journal, got %s", last.Event)
		}
		if !last.Override {
			t.Error("wanted override recorded on the moved event, got false")
		}
	})

	t.Run("the cap stays absolute after an override", func(t *testing.T) {
		// The operator's own ruling on this card: an override carries one
		// move and does not reset the count, so the next regressive move out
		// of the same column is refused again. The count climbing past the
		// limit is intended rather than an oversight.
		h := newHarness(t)
		h.declare(aftercare, "loop_limit", "1")
		ref := h.ready("looping")
		h.sendBack(ref)

		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: doing, Override: true})
		h.at(ref, aftercare)
		again := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: doing})
		if again.Refusal != contract.AtLoopLimit {
			t.Fatalf("wanted the next regressive move refused again, got %s %s", again.Outcome, again.Refusal)
		}
		response := h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
		if response.Loop == nil || response.Loop.Count != 2 || !response.Loop.AtLimit {
			t.Errorf("wanted the count risen to 2 and still at the limit, got %+v", response.Loop)
		}
	})

	t.Run("a forward move out of the column is unaffected", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "loop_limit", "1")
		ref := h.ready("looping")
		h.sendBack(ref)

		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
		if got := h.card(ref).Column; got != finished {
			t.Errorf("wanted the forward move to land, got %s", got)
		}
	})

	t.Run("a regressive move out of another column is unaffected", func(t *testing.T) {
		h := newHarness(t)
		// The limit is declared on doing this time, so the card can reach a
		// second column and depart it regressively without the declaring
		// column being the one it is leaving.
		h.declare(doing, "loop_limit", "1")
		ref := h.readyAt("looping", doing)
		h.at(ref, intake)
		h.at(ref, doing)
		if response := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: intake}); response.Refusal != contract.AtLoopLimit {
			t.Fatalf("the setup wanted doing standing at its limit, got %s %s", response.Outcome, response.Refusal)
		}

		// Aftercare declares nothing, so a regressive move out of it lands
		// whatever doing's own limit says.
		h.at(ref, aftercare)
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: review})
		if got := h.card(ref).Column; got != review {
			t.Errorf("wanted the move out of a column declaring nothing to land, got %s", got)
		}
	})
}

// TestALoopLimitOnAColumnTakingNoWorkUpNeverBinds is dinah-364 AC-9, the
// behavioural half. A fresh card travels through the intake column that
// declares the limit and meets no refusal, because no card is ever held there
// to depart regressively from in the first place.
func TestALoopLimitOnAColumnTakingNoWorkUpNeverBinds(t *testing.T) {
	h := newHarness(t)
	h.declare(intake, "loop_limit", "1")
	ref := h.add("passing through")

	for _, column := range []string{doing, intake, doing, aftercare} {
		response := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: column})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("move to %s: %s %s", column, response.Outcome, response.Refusal)
		}
	}
	if got := h.card(ref).Column; got != aftercare {
		t.Errorf("wanted the card carried through to %s, got %s", aftercare, got)
	}
}

// TestTheLoopLimitIsReportedAheadOfCapacity is dinah-364 AC-6. The two rows
// can both hold on one move, and the order is not arbitrary: the loop row
// reads the departure and the capacity row reads the destination, which is the
// order canRoute already runs its own rows in.
func TestTheLoopLimitIsReportedAheadOfCapacity(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "loop_limit", "1")
	ref := h.ready("looping")
	h.sendBack(ref)
	// The fixture's doing column has a capacity of one, so a second card
	// standing there fills it and the move below fails both rows at once.
	filler := h.add("filler")
	h.at(filler, doing)

	response := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: doing})
	if response.Outcome != contract.OutcomeRefused {
		t.Fatalf("wanted refused, got %s", response.Outcome)
	}
	if response.Refusal != contract.AtLoopLimit {
		t.Errorf("wanted %s ahead of %s, got %s", contract.AtLoopLimit, contract.AtCapacity, response.Refusal)
	}
	// The capacity row is still live on the same fixture, which is what says
	// the case above really did have both conditions to choose between.
	other := h.ready("other")
	full := h.do(&Request{Verb: Move, Card: other, Actor: "alka", Column: doing})
	if full.Refusal != contract.AtCapacity {
		t.Errorf("wanted the capacity row still reachable, got %s %s", full.Outcome, full.Refusal)
	}
}
