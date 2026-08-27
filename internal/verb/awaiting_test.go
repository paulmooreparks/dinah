package verb

import (
	"path/filepath"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// declare writes a key into one column's own anchor frontmatter and reopens the
// bench, which is how a test reaches a column property no verb sets. An empty
// value clears the key, so a test can take a declaration away again and assert
// that the same fixture behaves as a workbench that never carried it.
func (h *harness) declare(id, key, value string) {
	h.t.Helper()
	path := filepath.Join(h.root, bench.ColumnsDir, id, bench.ColumnAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		h.t.Fatalf("read the anchor of %s: %v", id, err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set(key, value)
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		h.t.Fatalf("write the anchor of %s: %v", id, err)
	}
	h.reopen()
}

// retire plants the sibling lock a structural act leaves beside a column
// directory while it retires it, which is what makes the retiring row of the
// move's list reachable from a test.
func (h *harness) retire(id string) {
	h.t.Helper()
	h.plant(bench.SiblingPath(filepath.Join(h.root, bench.ColumnsDir, id)), bench.LockRecord{
		Actor: "alka",
		PID:   4321,
		TS:    bench.Stamp(h.clock),
		Op:    bench.OpArchive,
	})
}

// at moves a card to a column as the operator, which is the setup half of every
// test below and never the act under test.
func (h *harness) at(ref, column string) {
	h.t.Helper()
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: column})
}

// TestAWaitingColumnRefusesTheClaim is dinah-201 AC-1. The refusal is what makes
// the flag more than advisory: dinah claim <card> names the card directly, so a
// column merely left out of what next offers would leak straight back through
// the named form.
//
// The successful half is doing real work. An implementation that refused every
// claim would pass the first half alone, and the second is what tells the two
// apart.
func TestAWaitingColumnRefusesTheClaim(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "awaiting_outside", "true")
	waiting := h.add("waiting on the customer")
	h.at(waiting, aftercare)
	ordinary := h.add("ordinary work")
	h.at(ordinary, doing)

	refused := h.do(&Request{Verb: Claim, Card: waiting, Actor: "brin"})
	if refused.Outcome != contract.OutcomeRefused || refused.Refusal != contract.AwaitingOutside {
		t.Fatalf("the claim at a waiting column: wanted %s, got %s %s",
			contract.AwaitingOutside, refused.Outcome, refused.Refusal)
	}
	if refused.Detail != aftercareSlug {
		t.Errorf("the refusal should name the column by its reference, got %q", refused.Detail)
	}
	if card := h.card(waiting); card.State != contract.StateReady || card.Holder != "" {
		t.Errorf("the refused claim wrote to the card: state %q holder %q", card.State, card.Holder)
	}

	if ok := h.do(&Request{Verb: Claim, Card: ordinary, Actor: "brin"}); ok.Outcome != contract.OutcomeOK {
		t.Fatalf("the claim at an ordinary column: wanted ok, got %s %s", ok.Outcome, ok.Refusal)
	}

	// The operator is refused on the same terms, because the flag is a fact
	// about the station rather than a permission somebody can be above.
	h2 := newHarness(t)
	h2.declare(aftercare, "awaiting_outside", "true")
	operators := h2.add("waiting on the customer")
	h2.at(operators, aftercare)
	if refusal := h2.do(&Request{Verb: Claim, Card: operators, Actor: "alka"}); refusal.Refusal != contract.AwaitingOutside {
		t.Errorf("the operator's claim: wanted %s, got %s %s",
			contract.AwaitingOutside, refusal.Outcome, refusal.Refusal)
	}
}

// TestTheClaimAnswersBlockedAheadOfWaiting is dinah-201 OQ-4, case one. The new
// row is the last of the claim's list, so a card that is both blocked and
// standing at a waiting column answers the profile's own blocked. Nothing else
// in the suite goes red if that order is reversed, which is the shape this
// board has been bitten by twice.
func TestTheClaimAnswersBlockedAheadOfWaiting(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "awaiting_outside", "true")
	ref := h.add("blocked and waiting")
	h.at(ref, aftercare)
	h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "the supplier went quiet"})

	response := h.do(&Request{Verb: Claim, Card: ref, Actor: "brin"})
	if response.Refusal != contract.Blocked {
		t.Fatalf("a card both blocked and waiting: wanted %s, got %s %s",
			contract.Blocked, response.Outcome, response.Refusal)
	}
	if response.Detail != "the supplier went quiet" {
		t.Errorf("the blocked refusal should carry the block reason, got %q", response.Detail)
	}

	// With the block lifted and nothing else changed, the same claim reaches
	// the new row, which is what proves the ordering rather than the absence
	// of the row.
	h.mustDo(&Request{Verb: Unblock, Card: ref, Actor: "alka"})
	if response := h.do(&Request{Verb: Claim, Card: ref, Actor: "brin"}); response.Refusal != contract.AwaitingOutside {
		t.Fatalf("once unblocked: wanted %s, got %s %s",
			contract.AwaitingOutside, response.Outcome, response.Refusal)
	}
}

// TestNextCarriesNothingFromAWaitingColumn is dinah-201 AC-2. The column holds two
// ready cards throughout, so an empty offer here is the flag rather than an
// empty queue, and the flag-removed half proves the fixture itself is sound.
func TestNextCarriesNothingFromAWaitingColumn(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "awaiting_outside", "true")
	first := h.add("first in the queue")
	h.at(first, aftercare)
	second := h.add("second in the queue")
	h.at(second, aftercare)

	offers, err := h.library.Next(&Request{Verb: "next", Actor: "brin", Column: aftercareSlug})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if len(offers) != 1 {
		t.Fatalf("wanted one offer, got %d", len(offers))
	}
	if offers[0].Card != nil {
		t.Errorf("a waiting column offered %s", offers[0].Card.Ref)
	}
	if !offers[0].AwaitingOutside {
		t.Error("the offer should say the column waits on somebody outside, so a reader can tell it from nothing ready")
	}

	h.declare(aftercare, "awaiting_outside", "")
	offers, err = h.library.Next(&Request{Verb: "next", Actor: "brin", Column: aftercareSlug})
	if err != nil {
		t.Fatalf("next once the flag is gone: %v", err)
	}
	if len(offers) != 1 || offers[0].Card == nil {
		t.Fatalf("wanted the queue head once the flag is gone, got %+v", offers)
	}
	if offers[0].Card.Ref != first {
		t.Errorf("wanted the queue head %s, got %s", first, offers[0].Card.Ref)
	}
	if offers[0].AwaitingOutside {
		t.Error("an ordinary column should not say it waits on anybody")
	}
}

// TestPullWillNotTakeFromOrLandInAWaitingColumn is dinah-201 AC-3. A pull is a
// claim bundled with a move and the claim is the thing the flag forbids, so the
// rule is uniform across both directions and both forms.
//
// Each half runs for a plain owner and for the operator, because the flag is
// not a permission and an operator-passes implementation would otherwise ship
// green.
func TestPullWillNotTakeFromOrLandInAWaitingColumn(t *testing.T) {
	for _, actor := range []string{"brin", "alka"} {
		t.Run("named, the upstream waits, as "+actor, func(t *testing.T) {
			h := newHarness(t)
			h.declare(doing, "awaiting_outside", "true")
			ref := h.add("standing in the waiting column")
			h.at(ref, doing)
			response := h.library.Pull(&Request{Verb: Pull, Actor: actor, Column: review})
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.AwaitingOutside {
				t.Fatalf("wanted %s, got %s %s", contract.AwaitingOutside, response.Outcome, response.Refusal)
			}
			if card := h.card(ref); card.Column != doing || card.State != contract.StateReady {
				t.Errorf("the refused pull moved the card: column %q state %q", card.Column, card.State)
			}
		})

		t.Run("named, the destination waits, as "+actor, func(t *testing.T) {
			h := newHarness(t)
			h.declare(doing, "awaiting_outside", "true")
			ref := h.add("ready in intake")
			response := h.library.Pull(&Request{Verb: Pull, Actor: actor, Column: doing})
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.AwaitingOutside {
				t.Fatalf("wanted %s, got %s %s", contract.AwaitingOutside, response.Outcome, response.Refusal)
			}
			if card := h.card(ref); card.Column != intake || card.State != contract.StateReady {
				t.Errorf("the refused pull moved the card: column %q state %q", card.Column, card.State)
			}
		})

		t.Run("bare, the upstream waits, as "+actor, func(t *testing.T) {
			h := newHarness(t)
			h.declare(doing, "awaiting_outside", "true")
			ref := h.add("standing in the waiting column")
			h.at(ref, doing)
			// The ready card is asserted to be standing there, so the empty
			// answer below is the qualifying set skipping the column rather
			// than an empty queue answering for it.
			if card := h.card(ref); card.Column != doing || card.State != contract.StateReady {
				t.Fatalf("the fixture should hold a ready card in the waiting column, got column %q state %q", card.Column, card.State)
			}
			response := h.library.Pull(&Request{Verb: Pull, Actor: actor})
			if response.Outcome != contract.OutcomeOK || response.Card != nil {
				t.Fatalf("wanted the empty answer, got %s %s", response.Outcome, response.Refusal)
			}
		})

		t.Run("bare, the destination waits, as "+actor, func(t *testing.T) {
			h := newHarness(t)
			h.declare(review, "awaiting_outside", "true")
			ref := h.add("ready in the upstream")
			h.at(ref, doing)
			if card := h.card(ref); card.Column != doing || card.State != contract.StateReady {
				t.Fatalf("the fixture should hold a ready card in the upstream, got column %q state %q", card.Column, card.State)
			}
			response := h.library.Pull(&Request{Verb: Pull, Actor: actor})
			if response.Outcome != contract.OutcomeOK || response.Card != nil {
				t.Fatalf("wanted the empty answer, got %s %s %s", response.Outcome, response.Refusal, response.Detail)
			}
		})
	}
}

// TestAMoveIntoAWaitingColumnNeedsAnUnheldCard is dinah-201 AC-4 and D-4. The
// tool does not release the claim on the holder's behalf, because CORE-MOVE-8
// says a move MUST NOT change a card's state or its holder, so the assertion
// after the refusal is the one that matters: an implementation that helpfully
// released would pass a refusal-name-only test.
func TestAMoveIntoAWaitingColumnNeedsAnUnheldCard(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "awaiting_outside", "true")

	unheld := h.add("handed on to the customer")
	h.mustDo(&Request{Verb: Move, Card: unheld, Actor: "brin", Column: aftercareSlug})
	if card := h.card(unheld); card.Column != aftercare {
		t.Fatalf("the unheld move did not land, the card stands at %q", card.Column)
	}

	held := h.readyAt("still being worked", doing)
	h.mustDo(&Request{Verb: Claim, Card: held, Actor: "brin"})
	response := h.do(&Request{Verb: Move, Card: held, Actor: "brin", Column: aftercareSlug})
	if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.AwaitingOutside {
		t.Fatalf("the held move: wanted %s, got %s %s", contract.AwaitingOutside, response.Outcome, response.Refusal)
	}
	card := h.card(held)
	if card.State != contract.StateActive {
		t.Errorf("the refused move changed the state to %q, which CORE-MOVE-8 forbids", card.State)
	}
	if card.Holder != "brin" {
		t.Errorf("the refused move changed the holder to %q, which CORE-MOVE-8 forbids", card.Holder)
	}
	if card.Column == aftercare {
		t.Error("the refused move carried the card anyway")
	}

	// The way onward is the one D-4 names: release, then move.
	h.mustDo(&Request{Verb: Release, Card: held, Actor: "brin"})
	h.mustDo(&Request{Verb: Move, Card: held, Actor: "brin", Column: aftercareSlug})
}

// TestDepartureFromAWaitingColumnStaysOpenToAnyOwner is dinah-201 AC-5 and D-7,
// which is the whole of what the card is complaining about. The second half is
// the guard: an implementation that wired the flag into the departure check
// would pass every other criterion on this card while reproducing exactly the
// operator-only bottleneck the flag exists to remove.
func TestDepartureFromAWaitingColumnStaysOpenToAnyOwner(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "awaiting_outside", "true")
	ref := h.add("the customer answered")
	h.at(ref, aftercare)

	before := len(h.events(ref))
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "brin", Column: doing})
	if card := h.card(ref); card.Column != doing {
		t.Fatalf("a plain owner could not move the card out, it stands at %q", card.Column)
	}
	events := h.events(ref)
	if len(events) != before+1 {
		t.Fatalf("wanted one new event, got %d", len(events)-before)
	}
	last := events[len(events)-1]
	if last.Event != contract.EventMoved {
		t.Errorf("wanted an ordinary %s event, got %s", contract.EventMoved, last.Event)
	}
	if last.From != aftercare || last.FromTitle != "Aftercare" {
		t.Errorf("the moved event should name the departure, got %q %q", last.From, last.FromTitle)
	}
	if last.Actor != "brin" {
		t.Errorf("the moved event should name the owner who moved it, got %q", last.Actor)
	}

	// A column carrying both declarations waits AND reserves the departure,
	// which is how a board that wants both says so. The refusal is
	// not-operator, from the departure row, and never the waiting name.
	h.declare(aftercare, "operator_owned", "true")
	back := h.add("waiting on the operator too")
	h.at(back, aftercare)
	response := h.do(&Request{Verb: Move, Card: back, Actor: "brin", Column: doing})
	if response.Refusal != contract.NotOperator {
		t.Fatalf("out of a column that waits and is operator-owned: wanted %s, got %s %s",
			contract.NotOperator, response.Outcome, response.Refusal)
	}
}

// TestTheWaitingRowSitsAfterCapacityAndBeforeRetiring is dinah-201 OQ-4, cases
// two and three. CORE-OUT-6 makes the order observable, and the spec argues for
// this slot in prose alone, so these are the assertions that would go red if an
// implementer put the row anywhere else in canLand's list.
func TestTheWaitingRowSitsAfterCapacityAndBeforeRetiring(t *testing.T) {
	t.Run("a full waiting column answers at-capacity", func(t *testing.T) {
		h := newHarness(t)
		// Doing declares a limit of one, so the occupant below fills it.
		h.declare(doing, "awaiting_outside", "true")
		occupant := h.add("already there")
		h.at(occupant, doing)
		arriving := h.readyAt("arriving held", aftercare)
		h.mustDo(&Request{Verb: Claim, Card: arriving, Actor: "brin"})

		response := h.do(&Request{Verb: Move, Card: arriving, Actor: "brin", Column: doing})
		if response.Refusal != contract.AtCapacity {
			t.Fatalf("wanted %s ahead of %s, got %s %s",
				contract.AtCapacity, contract.AwaitingOutside, response.Outcome, response.Refusal)
		}
	})

	t.Run("a waiting column below its limit answers the waiting name", func(t *testing.T) {
		// The control for the case above: with the capacity row satisfied
		// and nothing else changed, the same move reaches the new row, so
		// the assertion above is about the order rather than about the row
		// being absent.
		h := newHarness(t)
		h.declare(doing, "awaiting_outside", "true")
		arriving := h.readyAt("arriving held", aftercare)
		h.mustDo(&Request{Verb: Claim, Card: arriving, Actor: "brin"})
		response := h.do(&Request{Verb: Move, Card: arriving, Actor: "brin", Column: doing})
		if response.Refusal != contract.AwaitingOutside {
			t.Fatalf("wanted %s, got %s %s", contract.AwaitingOutside, response.Outcome, response.Refusal)
		}
	})

	t.Run("a retiring waiting column answers the waiting name", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "awaiting_outside", "true")
		h.retire(aftercare)
		arriving := h.readyAt("arriving held", doing)
		h.mustDo(&Request{Verb: Claim, Card: arriving, Actor: "brin"})

		response := h.do(&Request{Verb: Move, Card: arriving, Actor: "brin", Column: aftercareSlug})
		if response.Refusal != contract.AwaitingOutside {
			t.Fatalf("wanted %s ahead of %s, got %s %s",
				contract.AwaitingOutside, contract.Locked, response.Outcome, response.Refusal)
		}
	})

	t.Run("a retiring ordinary column still answers locked", func(t *testing.T) {
		// The control for the case above, which is what keeps it from
		// passing because the retiring row stopped working.
		h := newHarness(t)
		h.retire(aftercare)
		arriving := h.readyAt("arriving held", doing)
		h.mustDo(&Request{Verb: Claim, Card: arriving, Actor: "brin"})
		response := h.do(&Request{Verb: Move, Card: arriving, Actor: "brin", Column: aftercareSlug})
		if response.Refusal != contract.Locked {
			t.Fatalf("wanted %s, got %s %s", contract.Locked, response.Outcome, response.Refusal)
		}
	})
}

// TestTheColumnViewCarriesTheFlag is the library half of dinah-201 AC-8: the
// property reaches the MCP columns tool through ColumnView, and it is a separate
// field from OperatorOwned, which goes on answering who may move a card out.
func TestTheColumnViewCarriesTheFlag(t *testing.T) {
	h := newHarness(t)
	h.declare(aftercare, "awaiting_outside", "true")
	views, err := h.library.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	var waiting, ordinary, owned *ColumnView
	for i := range views {
		switch views[i].ID {
		case aftercare:
			waiting = &views[i]
		case doing:
			ordinary = &views[i]
		case review:
			owned = &views[i]
		}
	}
	if waiting == nil || ordinary == nil || owned == nil {
		t.Fatalf("the fixture's columns did not all come back: %+v", views)
	}
	if !waiting.AwaitingOutside {
		t.Error("the waiting column's view does not carry the flag")
	}
	if waiting.OperatorOwned {
		t.Error("a waiting column is not thereby operator-owned, and collapsing the two would tell every reader it is")
	}
	if ordinary.AwaitingOutside {
		t.Error("an ordinary column's view carries the flag")
	}
	if !owned.OperatorOwned || owned.AwaitingOutside {
		t.Error("an operator-owned column is not thereby a waiting one")
	}
}
