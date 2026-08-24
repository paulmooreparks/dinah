package verb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// bufferDefinition is a flow with a buffer standing between two stations, plus
// a second station past it so that a pull out of the buffer has somewhere to
// land and a pull out of that station has somewhere to go.
const bufferDefinition = `{
  "profile": "dinah-core/0.5",
  "title": "Buffered",
  "instructions": "Standing text.\n",
  "states": [
    { "id": "b00000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" },
    { "id": "b00000000003", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "b00000000004", "title": "Done", "slug": "done", "kind": "done" }
  ]
}`

// The identifiers of the buffered flow's four states, which the tests below
// name directly.
const (
	bufferIntake = "b00000000001"
	bufferQueue  = "b00000000002"
	bufferDoing  = "b00000000003"
	bufferDone   = "b00000000004"
)

// newBufferHarness builds a harness over the buffered flow above.
func newBufferHarness(t *testing.T) *harness {
	t.Helper()
	return harnessFromDefinition(t, "bf", bufferDefinition)
}

// harnessFromDefinition builds a harness over any interchange definition,
// which is what lets each test below describe the flow its own case turns on
// rather than bend the shared fixture around it.
func harnessFromDefinition(t *testing.T, slug, definition string) *harness {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	root := filepath.Join(base, "workbench")
	if err := os.MkdirAll(filepath.Join(home, bench.UserBaseName), 0o755); err != nil {
		t.Fatalf("user base: %v", err)
	}
	read, err := bench.ReadDefinition([]byte(definition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := bench.Instantiate(root, slug, "alka", read); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	h := &harness{home: home, root: root, clock: time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC), t: t}
	h.reopen()
	return h
}

// TestAClaimIsRefusedWhereNoWorkIsTakenUp is dinah-273 AC-4 and AC-5. One rule
// answers in two names: a state declaring awaiting_outside says who the
// workbench is waiting on and keeps its own name, and a state that takes no
// work up by kind has nobody to name and answers dinah.takes-no-work.
func TestAClaimIsRefusedWhereNoWorkIsTakenUp(t *testing.T) {
	cases := []struct {
		name  string
		state string
	}{
		{name: "a buffer", state: bufferQueue},
		{name: "an intake state", state: bufferIntake},
		{name: "a done state", state: bufferDone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newBufferHarness(t)
			ref := h.add("standing where nobody works")
			h.at(ref, c.state)
			response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.TakesNoWork {
				t.Fatalf("wanted %s, got %s %s", contract.TakesNoWork, response.Outcome, response.Refusal)
			}
			if card := h.card(ref); card.Substate != contract.SubstateReady || card.Holder != "" {
				t.Errorf("the refused claim wrote something: substate %q, holder %q", card.Substate, card.Holder)
			}
		})
	}

	t.Run("a state waiting on somebody outside keeps its own name", func(t *testing.T) {
		h := newBufferHarness(t)
		h.declare(bufferDoing, "awaiting_outside", "true")
		ref := h.add("waiting on the customer")
		h.at(ref, bufferDoing)
		response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
		if response.Refusal != contract.AwaitingOutside {
			t.Fatalf("wanted %s, got %s %s", contract.AwaitingOutside, response.Outcome, response.Refusal)
		}
	})
}

// TestABlockedCardAtABufferStillAnswersBlocked is dinah-273 AC-6. The state row
// stands after the substate row, so a card that is both blocked and standing
// where nobody works answers the profile's own name, which is what CORE-OUT-6
// makes observable.
func TestABlockedCardAtABufferStillAnswersBlocked(t *testing.T) {
	h := newBufferHarness(t)
	ref := h.add("blocked in the buffer")
	h.at(ref, bufferQueue)
	h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "waiting on a ruling"})
	response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	if response.Refusal != contract.Blocked {
		t.Fatalf("wanted %s ahead of %s, got %s %s", contract.Blocked, contract.TakesNoWork, response.Outcome, response.Refusal)
	}
}

// TestABlockStillLandsWhereNoWorkIsTakenUp is dinah-273 AC-7 and the second
// half of D-1. A block is a statement about the card rather than about a
// worker, so it stays available wherever the card stands.
func TestABlockStillLandsWhereNoWorkIsTakenUp(t *testing.T) {
	for _, state := range []string{bufferQueue, bufferIntake, bufferDone} {
		h := newBufferHarness(t)
		ref := h.add("stopped")
		h.at(ref, state)
		h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "the supplier has not answered"})
		if card := h.card(ref); card.Substate != contract.SubstateBlocked {
			t.Errorf("a block at %s left the card %q", state, card.Substate)
		}
	}
}

// TestAPullTakesTheEarliestCardOutOfABuffer is dinah-273 AC-8. The claim lands
// at the destination rather than at the buffer, and the journal carries the two
// events a pull writes and no more.
func TestAPullTakesTheEarliestCardOutOfABuffer(t *testing.T) {
	h := newBufferHarness(t)
	first := h.add("waiting first")
	h.at(first, bufferQueue)
	h.advance(time.Hour)
	second := h.add("waiting second")
	h.at(second, bufferQueue)

	response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", State: "doing"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("pull out of a buffer: %s %s", response.Outcome, response.Refusal)
	}
	if response.Card == nil || response.Card.Ref != first {
		t.Fatalf("the pull took %+v, and the earliest arrival is %s", response.Card, first)
	}
	card := h.card(first)
	if card.State != bufferDoing || card.Substate != contract.SubstateActive || card.Holder != "bo" {
		t.Errorf("the card reads state %q substate %q holder %q", card.State, card.Substate, card.Holder)
	}
	claimed, moved := 0, 0
	for _, event := range h.events(first) {
		switch event.Event {
		case contract.EventClaimed:
			claimed++
		case contract.EventMoved:
			moved++
		}
	}
	// The card was carried into the buffer by a move of its own, so the
	// journal holds that one as well as the pull's.
	if claimed != 1 || moved != 2 {
		t.Errorf("the journal holds %d claimed and %d moved events, and the pull appends one of each", claimed, moved)
	}
}

// TestNoQueueStateIsEverABarePullsDestination is dinah-273 AC-9 and AC-31. A
// pull lands a card where somebody works it, so a bare pull whose only
// candidate takes no work up answers emptily and writes nothing. The done half
// is the behaviour change from before this card, where such a pull landed the
// card in the done state and claimed it there.
func TestNoQueueStateIsEverABarePullsDestination(t *testing.T) {
	cases := []struct {
		name  string
		flow  string
		ready string
	}{
		{
			name: "the only candidate is a buffer",
			flow: `{
  "profile": "dinah-core/0.5",
  "title": "Buffer last",
  "states": [
    { "id": "c00000000001", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "c00000000002", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" }
  ]
}`,
			ready: "c00000000001",
		},
		{
			name: "the only candidate is a done state",
			flow: `{
  "profile": "dinah-core/0.5",
  "title": "Done last",
  "states": [
    { "id": "c00000000001", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "c00000000002", "title": "Done", "slug": "done", "kind": "done" }
  ]
}`,
			ready: "c00000000001",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := harnessFromDefinition(t, "bp", c.flow)
			ref := h.add("standing ready")
			h.at(ref, c.ready)
			before := len(h.events(ref))

			response := h.library.Pull(&Request{Verb: Pull, Actor: "bo"})
			h.reopen()
			if response.Outcome != contract.OutcomeOK {
				t.Fatalf("wanted the empty answer, got %s %s", response.Outcome, response.Refusal)
			}
			if response.Card != nil {
				t.Fatalf("the bare pull took %+v, and no state qualifies as its destination", response.Card)
			}
			if got := len(h.events(ref)); got != before {
				t.Errorf("the empty answer wrote %d events", got-before)
			}
		})
	}
}

// TestANamedPullIsRefusedAtAQueueDestination is dinah-273 AC-10 and D-4. The
// --no-claim form is refused on the same terms, because the option changes what
// a pull writes rather than what a pull allows.
func TestANamedPullIsRefusedAtAQueueDestination(t *testing.T) {
	destinations := []struct {
		name  string
		state string
		slug  string
		from  string
	}{
		{name: "a buffer", state: bufferQueue, slug: "waiting", from: bufferIntake},
		{name: "a done state", state: bufferDone, slug: "done", from: bufferDoing},
	}
	for _, d := range destinations {
		for _, noClaim := range []bool{false, true} {
			name := d.name
			if noClaim {
				name += ", carrying --no-claim"
			}
			t.Run(name, func(t *testing.T) {
				h := newBufferHarness(t)
				ref := h.add("upstream of the destination")
				h.at(ref, d.from)
				response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", State: d.slug, NoClaim: noClaim})
				h.reopen()
				if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.TakesNoWork {
					t.Fatalf("wanted %s, got %s %s", contract.TakesNoWork, response.Outcome, response.Refusal)
				}
			})
		}
	}

	t.Run("an intake state", func(t *testing.T) {
		// An intake state stands first, so a pull naming one answers
		// no-upstream before it reaches the destination row. The case that
		// reaches the row is an intake state standing elsewhere, which
		// dinah check reports and which still opens.
		h := harnessFromDefinition(t, "ni", `{
  "profile": "dinah-core/0.5",
  "title": "Intake second",
  "states": [
    { "id": "d00000000001", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "d00000000002", "title": "Intake", "slug": "intake", "kind": "intake" }
  ]
}`)
		ref := h.add("upstream of the intake state")
		h.at(ref, "d00000000001")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", State: "intake"})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.TakesNoWork {
			t.Fatalf("wanted %s, got %s %s", contract.TakesNoWork, response.Outcome, response.Refusal)
		}
	})
}

// TestAHolderReleasesBeforeMovingIntoAQueueState is dinah-273 AC-11 and AC-12.
// The refusal reaches a queue-state destination and no other, so the same move
// into a plain work state still lands with the card still held.
func TestAHolderReleasesBeforeMovingIntoAQueueState(t *testing.T) {
	for _, destination := range []string{"waiting", "intake", "done"} {
		t.Run("into "+destination, func(t *testing.T) {
			h := newBufferHarness(t)
			ref := h.add("held")
			h.at(ref, bufferDoing)
			h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})

			refused := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", State: destination})
			if refused.Refusal != contract.TakesNoWork {
				t.Fatalf("wanted %s, got %s %s", contract.TakesNoWork, refused.Outcome, refused.Refusal)
			}
			h.mustDo(&Request{Verb: Release, Card: ref, Actor: "alka"})
			h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: destination})
			card := h.card(ref)
			if card.Substate != contract.SubstateReady || card.Holder != "" {
				t.Errorf("after the release the card reads substate %q holder %q", card.Substate, card.Holder)
			}
		})
	}

	t.Run("into a plain work state, still held", func(t *testing.T) {
		h := newBufferHarness(t)
		ref := h.add("held")
		h.at(ref, bufferIntake)
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: "doing"})
		h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
		h.declare(bufferDone, "kind", contract.KindWork)
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: "done"})
		if card := h.card(ref); card.Substate != contract.SubstateActive || card.Holder != "alka" {
			t.Errorf("a move into a station changed the claim: substate %q holder %q", card.Substate, card.Holder)
		}
	})
}

// TestNextOffersACardWhereverAnActCouldTakeIt is dinah-273 AC-13. The offer is
// about the card rather than about the kind, so an intake state and a buffer
// offer their head card marked as a pull, a done state and a state waiting on
// somebody outside offer nothing, and a plain work state offers as it always
// has.
func TestNextOffersACardWhereverAnActCouldTakeIt(t *testing.T) {
	h := newBufferHarness(t)
	h.declare(bufferDoing, "awaiting_outside", "")
	for _, state := range []string{bufferIntake, bufferQueue, bufferDoing, bufferDone} {
		ref := h.add("standing at " + state)
		h.at(ref, state)
	}

	cases := []struct {
		state           string
		card            bool
		takenByPull     bool
		noTaker         bool
		awaitingOutside bool
	}{
		{state: "intake", card: true, takenByPull: true},
		{state: "waiting", card: true, takenByPull: true},
		{state: "doing", card: true},
		{state: "done", noTaker: true},
	}
	for _, c := range cases {
		t.Run(c.state, func(t *testing.T) {
			offer := onlyOffer(t, h, c.state)
			if (offer.Card != nil) != c.card {
				t.Fatalf("the offer carries card %+v and the case wants a card: %v", offer.Card, c.card)
			}
			if offer.TakenByPull != c.takenByPull {
				t.Errorf("taken_by_pull is %v, wanted %v", offer.TakenByPull, c.takenByPull)
			}
			if offer.NoTaker != c.noTaker {
				t.Errorf("no_taker is %v, wanted %v", offer.NoTaker, c.noTaker)
			}
			if offer.AwaitingOutside != c.awaitingOutside {
				t.Errorf("awaiting_outside is %v, wanted %v", offer.AwaitingOutside, c.awaitingOutside)
			}
		})
	}

	t.Run("a state waiting on somebody outside carries both members", func(t *testing.T) {
		h.declare(bufferDoing, "awaiting_outside", "true")
		offer := onlyOffer(t, h, "doing")
		if offer.Card != nil {
			t.Fatalf("a state waiting on somebody outside offered %+v", offer.Card)
		}
		if !offer.NoTaker || !offer.AwaitingOutside {
			t.Errorf("wanted both members, got no_taker %v and awaiting_outside %v", offer.NoTaker, offer.AwaitingOutside)
		}
	})
}

// TestNextOffersNothingWhereNoPullCouldReach is dinah-273 AC-30. An offer is
// made only where an act could take the card, so a buffer with nothing beyond
// it that takes work up offers nothing and says so.
func TestNextOffersNothingWhereNoPullCouldReach(t *testing.T) {
	cases := []struct {
		name string
		flow string
	}{
		{
			name: "a buffer standing last",
			flow: `{
  "profile": "dinah-core/0.5",
  "title": "Buffer last",
  "states": [
    { "id": "e00000000001", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "e00000000002", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" }
  ]
}`,
		},
		{
			name: "a buffer whose downstream waits on somebody outside",
			flow: `{
  "profile": "dinah-core/0.5",
  "title": "Buffer then outside",
  "states": [
    { "id": "e00000000001", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" },
    { "id": "e00000000002", "title": "Outside", "slug": "outside", "kind": "work", "awaiting_outside": true }
  ]
}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := harnessFromDefinition(t, "nb", c.flow)
			ref := h.add("standing in the buffer")
			h.at(ref, "waiting")
			offer := onlyOffer(t, h, "waiting")
			if offer.Card != nil {
				t.Fatalf("the buffer offered %+v and no pull could take it", offer.Card)
			}
			if !offer.NoTaker {
				t.Error("the offer carries no no_taker member, so a reader cannot tell it from nothing ready")
			}
		})
	}
}

// TestNextOffersEveryStateOfABufferedFlow is dinah-273 AC-29, which reads the
// whole flow in one call rather than one state at a time, so the Take column a
// reader sees is asserted over the rows that stand beside each other.
func TestNextOffersEveryStateOfABufferedFlow(t *testing.T) {
	h := newBufferHarness(t)
	for _, state := range []string{bufferIntake, bufferQueue, bufferDoing, bufferDone} {
		ref := h.add("standing at " + state)
		h.at(ref, state)
	}
	offers, err := h.library.Next(&Request{Verb: "next", Actor: "bo"})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if len(offers) != 4 {
		t.Fatalf("wanted one offer per state, got %d", len(offers))
	}
	wanted := []struct {
		card        bool
		takenByPull bool
	}{
		{card: true, takenByPull: true},
		{card: true, takenByPull: true},
		{card: true},
		{},
	}
	for at, want := range wanted {
		if (offers[at].Card != nil) != want.card {
			t.Errorf("offer %d carries card %+v, wanted a card: %v", at, offers[at].Card, want.card)
		}
		if offers[at].TakenByPull != want.takenByPull {
			t.Errorf("offer %d reads taken_by_pull %v, wanted %v", at, offers[at].TakenByPull, want.takenByPull)
		}
	}
	if !offers[3].NoTaker {
		t.Error("the done state offers nothing and should say nothing is taken from it")
	}
}

// TestTheStatesListingSaysWhereWorkIsTakenUp is dinah-273 AC-14 read at the
// library, which is where the value the renderer prints is decided.
func TestTheStatesListingSaysWhereWorkIsTakenUp(t *testing.T) {
	h := newBufferHarness(t)
	h.declare(bufferDoing, "awaiting_outside", "true")
	views, err := h.library.States()
	if err != nil {
		t.Fatalf("states: %v", err)
	}
	wanted := map[string]bool{"intake": false, "waiting": false, "doing": false, "done": false}
	for _, view := range views {
		want, ok := wanted[view.Slug]
		if !ok {
			t.Fatalf("the listing carries an unexpected state %q", view.Slug)
		}
		if view.TakesWorkUp != want {
			t.Errorf("%s reads takes_work_up %v, wanted %v", view.Slug, view.TakesWorkUp, want)
		}
	}

	h.declare(bufferDoing, "awaiting_outside", "")
	views, err = h.library.States()
	if err != nil {
		t.Fatalf("states: %v", err)
	}
	for _, view := range views {
		if view.Slug == "doing" && !view.TakesWorkUp {
			t.Error("a plain work state reads takes_work_up false")
		}
	}
}

// TestACardHeldWhereNoWorkIsTakenUpIsReported is dinah-273 AC-15 and D-7. A
// board that already carries such a claim opens, dinah status reads it, and
// dinah check names the card's own anchor.
func TestACardHeldWhereNoWorkIsTakenUpIsReported(t *testing.T) {
	h := newBufferHarness(t)
	ref := h.add("held in a done state")
	h.at(ref, bufferDoing)
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	// The claim is written first and the card carried afterwards by hand,
	// because every act that would put it there is refused from this build on.
	card := h.card(ref)
	card.State = bufferDone
	if err := card.Save(); err != nil {
		t.Fatalf("carry the held card into the done state: %v", err)
	}
	h.reopen()

	if _, err := h.library.Status(&Request{Verb: "status", Actor: "alka"}); err != nil {
		t.Fatalf("a workbench carrying such a claim should still be readable: %v", err)
	}
	findings, err := h.library.Bench.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	anchor := h.card(ref).AnchorPath()
	for _, finding := range findings {
		if finding.Key == bench.FindingClaimWhereNoWorkIsTaken && finding.Path == anchor {
			return
		}
	}
	t.Errorf("check reported %+v, and none of them names %s at %s", findings, bench.FindingClaimWhereNoWorkIsTaken, anchor)
}

// TestAKindThisBuildDoesNotImplementIsWorkedLikeAnyOther is dinah-273 AC-17 and
// drives CORE-STATE-12 at the verb layer: a claim succeeds there and next
// offers the card, which is the work-state reading the statement requires.
func TestAKindThisBuildDoesNotImplementIsWorkedLikeAnyOther(t *testing.T) {
	h := harnessFromDefinition(t, "uk", `{
  "profile": "dinah-core/0.5",
  "title": "Unknown kind",
  "states": [
    { "id": "f00000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "f00000000002", "title": "Odd", "slug": "odd", "kind": "other.thing" },
    { "id": "f00000000003", "title": "Done", "slug": "done", "kind": "done" }
  ]
}`)
	ref := h.add("worked at a kind this build does not implement")
	h.at(ref, "f00000000002")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	h.mustDo(&Request{Verb: Release, Card: ref, Actor: "alka"})
	offer := onlyOffer(t, h, "odd")
	if offer.Card == nil || offer.Card.Ref != ref {
		t.Errorf("next offered %+v at a state read as a work state", offer.Card)
	}
}

// onlyOffer reads the one offer a named state makes and fails unless exactly
// one came back.
func onlyOffer(t *testing.T, h *harness, state string) Offer {
	t.Helper()
	h.reopen()
	offers, err := h.library.Next(&Request{Verb: "next", Actor: "bo", State: state})
	if err != nil {
		t.Fatalf("next %s: %v", state, err)
	}
	if len(offers) != 1 {
		t.Fatalf("next %s returned %d offers", state, len(offers))
	}
	return offers[0]
}
