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
  "profile": "dinah-core/0.7",
  "title": "Buffered",
  "instructions": "Standing text.\n",
  "columns": [
    { "id": "b00000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" },
    { "id": "b00000000003", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "b00000000004", "title": "Done", "slug": "done", "kind": "done" }
  ]
}`

// The identifiers of the buffered flow's four columns, which the tests below
// name directly.
const (
	bufferIntake = "b00000000001"
	bufferQueue  = "b00000000002"
	bufferDoing  = "b00000000003"
	bufferDone   = "b00000000004"
)

// The slugs of the same four columns, which is what a group drawn on the
// column axis carries as its value, because columnRef prefers a column's slug
// over its identifier.
const (
	bufferIntakeSlug = "intake"
	bufferQueueSlug  = "waiting"
	bufferDoingSlug  = "doing"
	bufferDoneSlug   = "done"
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
// answers in two names: a column declaring awaiting_outside says who the
// workbench is waiting on and keeps its own name, and a column that takes no
// work up by kind has nobody to name and answers dinah.takes-no-work.
func TestAClaimIsRefusedWhereNoWorkIsTakenUp(t *testing.T) {
	cases := []struct {
		name   string
		column string
	}{
		{name: "a buffer", column: bufferQueue},
		{name: "an intake column", column: bufferIntake},
		{name: "a done column", column: bufferDone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newBufferHarness(t)
			ref := h.add("standing where nobody works")
			h.at(ref, c.column)
			response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.TakesNoWork {
				t.Fatalf("wanted %s, got %s %s", contract.TakesNoWork, response.Outcome, response.Refusal)
			}
			if card := h.card(ref); card.State != contract.StateReady || card.Holder != "" {
				t.Errorf("the refused claim wrote something: state %q, holder %q", card.State, card.Holder)
			}
		})
	}

	t.Run("a column waiting on somebody outside keeps its own name", func(t *testing.T) {
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

// TestABlockedCardAtABufferStillAnswersBlocked is dinah-273 AC-6. The column row
// stands after the state row, so a card that is both blocked and standing
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
	for _, column := range []string{bufferQueue, bufferIntake, bufferDone} {
		h := newBufferHarness(t)
		ref := h.add("stopped")
		h.at(ref, column)
		h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "the supplier has not answered"})
		if card := h.card(ref); card.State != contract.StateBlocked {
			t.Errorf("a block at %s left the card %q", column, card.State)
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

	response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: "doing"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("pull out of a buffer: %s %s", response.Outcome, response.Refusal)
	}
	if response.Card == nil || response.Card.Ref != first {
		t.Fatalf("the pull took %+v, and the earliest arrival is %s", response.Card, first)
	}
	card := h.card(first)
	if card.Column != bufferDoing || card.State != contract.StateActive || card.Holder != "bo" {
		t.Errorf("the card reads column %q state %q holder %q", card.Column, card.State, card.Holder)
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

// TestNoQueueColumnIsEverABarePullsDestination is dinah-273 AC-9 and AC-31. A
// pull lands a card where somebody works it, so a bare pull whose only
// candidate takes no work up answers emptily and writes nothing. The done half
// is the behaviour change from before this card, where such a pull landed the
// card in the done column and claimed it there.
func TestNoQueueColumnIsEverABarePullsDestination(t *testing.T) {
	cases := []struct {
		name  string
		flow  string
		ready string
	}{
		{
			name: "the only candidate is a buffer",
			flow: `{
  "profile": "dinah-core/0.7",
  "title": "Buffer last",
  "columns": [
    { "id": "c00000000001", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "c00000000002", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" }
  ]
}`,
			ready: "c00000000001",
		},
		{
			name: "the only candidate is a done column",
			flow: `{
  "profile": "dinah-core/0.7",
  "title": "Done last",
  "columns": [
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
				t.Fatalf("the bare pull took %+v, and no column qualifies as its destination", response.Card)
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
		name   string
		column string
		slug   string
		from   string
	}{
		{name: "a buffer", column: bufferQueue, slug: "waiting", from: bufferIntake},
		{name: "a done column", column: bufferDone, slug: "done", from: bufferDoing},
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
				response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: d.slug, NoClaim: noClaim})
				h.reopen()
				if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.TakesNoWork {
					t.Fatalf("wanted %s, got %s %s", contract.TakesNoWork, response.Outcome, response.Refusal)
				}
			})
		}
	}

	t.Run("an intake column", func(t *testing.T) {
		// An intake column stands first, so a pull naming one answers
		// no-upstream before it reaches the destination row. The case that
		// reaches the row is an intake column standing elsewhere, which
		// dinah check reports and which still opens.
		h := harnessFromDefinition(t, "ni", `{
  "profile": "dinah-core/0.7",
  "title": "Intake second",
  "columns": [
    { "id": "d00000000001", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "d00000000002", "title": "Intake", "slug": "intake", "kind": "intake" }
  ]
}`)
		ref := h.add("upstream of the intake column")
		h.at(ref, "d00000000001")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: "intake"})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.TakesNoWork {
			t.Fatalf("wanted %s, got %s %s", contract.TakesNoWork, response.Outcome, response.Refusal)
		}
	})
}

// TestAHolderReleasesBeforeMovingIntoAQueueColumn is dinah-273 AC-11 and AC-12.
// The refusal reaches a queue-column destination and no other, so the same move
// into a plain work column still lands with the card still held.
func TestAHolderReleasesBeforeMovingIntoAQueueColumn(t *testing.T) {
	for _, destination := range []string{"waiting", "intake", "done"} {
		t.Run("into "+destination, func(t *testing.T) {
			h := newBufferHarness(t)
			ref := h.add("held")
			h.at(ref, bufferDoing)
			h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})

			refused := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: destination})
			if refused.Refusal != contract.TakesNoWork {
				t.Fatalf("wanted %s, got %s %s", contract.TakesNoWork, refused.Outcome, refused.Refusal)
			}
			h.mustDo(&Request{Verb: Release, Card: ref, Actor: "alka"})
			h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: destination})
			card := h.card(ref)
			if card.State != contract.StateReady || card.Holder != "" {
				t.Errorf("after the release the card reads state %q holder %q", card.State, card.Holder)
			}
		})
	}

	t.Run("into a plain work column, still held", func(t *testing.T) {
		h := newBufferHarness(t)
		ref := h.add("held")
		h.at(ref, bufferIntake)
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: "doing"})
		h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
		h.declare(bufferDone, "kind", contract.KindWork)
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: "done"})
		if card := h.card(ref); card.State != contract.StateActive || card.Holder != "alka" {
			t.Errorf("a move into a station changed the claim: state %q holder %q", card.State, card.Holder)
		}
	})
}

// TestNextOffersACardWhereverAnActCouldTakeIt is dinah-273 AC-13. The offer is
// about the card rather than about the kind, so an intake column and a buffer
// offer their head card marked as a pull, a done column and a column waiting on
// somebody outside offer nothing, and a plain work column offers as it always
// has.
func TestNextOffersACardWhereverAnActCouldTakeIt(t *testing.T) {
	h := newBufferHarness(t)
	h.declare(bufferDoing, "awaiting_outside", "")
	for _, column := range []string{bufferIntake, bufferQueue, bufferDoing, bufferDone} {
		ref := h.add("standing at " + column)
		h.at(ref, column)
	}

	cases := []struct {
		column          string
		card            bool
		takenByPull     bool
		noTaker         bool
		awaitingOutside bool
	}{
		{column: "intake", card: true, takenByPull: true},
		{column: "waiting", card: true, takenByPull: true},
		{column: "doing", card: true},
		{column: "done", noTaker: true},
	}
	for _, c := range cases {
		t.Run(c.column, func(t *testing.T) {
			offer := onlyOffer(t, h, c.column)
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

	t.Run("a column waiting on somebody outside carries both members", func(t *testing.T) {
		h.declare(bufferDoing, "awaiting_outside", "true")
		offer := onlyOffer(t, h, "doing")
		if offer.Card != nil {
			t.Fatalf("a column waiting on somebody outside offered %+v", offer.Card)
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
  "profile": "dinah-core/0.7",
  "title": "Buffer last",
  "columns": [
    { "id": "e00000000001", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "e00000000002", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" }
  ]
}`,
		},
		{
			name: "a buffer whose downstream waits on somebody outside",
			flow: `{
  "profile": "dinah-core/0.7",
  "title": "Buffer then outside",
  "columns": [
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

// TestNextOffersEveryColumnOfABufferedFlow is dinah-273 AC-29, which reads the
// whole flow in one call rather than one column at a time, so the Take column a
// reader sees is asserted over the rows that stand beside each other.
func TestNextOffersEveryColumnOfABufferedFlow(t *testing.T) {
	h := newBufferHarness(t)
	for _, column := range []string{bufferIntake, bufferQueue, bufferDoing, bufferDone} {
		ref := h.add("standing at " + column)
		h.at(ref, column)
	}
	offers, err := h.library.Next(&Request{Verb: "next", Actor: "bo"})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if len(offers) != 4 {
		t.Fatalf("wanted one offer per column, got %d", len(offers))
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
		t.Error("the done column offers nothing and should say nothing is taken from it")
	}
}

// TestTheColumnsListingSaysWhereWorkIsTakenUp is dinah-273 AC-14 read at the
// library, which is where the value the renderer prints is decided.
func TestTheColumnsListingSaysWhereWorkIsTakenUp(t *testing.T) {
	h := newBufferHarness(t)
	h.declare(bufferDoing, "awaiting_outside", "true")
	views, err := h.library.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	wanted := map[string]bool{"intake": false, "waiting": false, "doing": false, "done": false}
	for _, view := range views {
		want, ok := wanted[view.Slug]
		if !ok {
			t.Fatalf("the listing carries an unexpected column %q", view.Slug)
		}
		if view.TakesWorkUp != want {
			t.Errorf("%s reads takes_work_up %v, wanted %v", view.Slug, view.TakesWorkUp, want)
		}
	}

	h.declare(bufferDoing, "awaiting_outside", "")
	views, err = h.library.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	for _, view := range views {
		if view.Slug == "doing" && !view.TakesWorkUp {
			t.Error("a plain work column reads takes_work_up false")
		}
	}
}

// TestACardHeldWhereNoWorkIsTakenUpIsReported is dinah-273 AC-15 and D-7. A
// board that already carries such a claim opens, dinah status reads it, and
// dinah check names the card's own anchor.
func TestACardHeldWhereNoWorkIsTakenUpIsReported(t *testing.T) {
	h := newBufferHarness(t)
	ref := h.add("held in a done column")
	h.at(ref, bufferDoing)
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	// The claim is written first and the card carried afterwards by hand,
	// because every act that would put it there is refused from this build on.
	card := h.card(ref)
	card.Column = bufferDone
	if err := card.Save(); err != nil {
		t.Fatalf("carry the held card into the done column: %v", err)
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
// offers the card, which is the work-column reading the statement requires.
func TestAKindThisBuildDoesNotImplementIsWorkedLikeAnyOther(t *testing.T) {
	h := harnessFromDefinition(t, "uk", `{
  "profile": "dinah-core/0.7",
  "title": "Unknown kind",
  "columns": [
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
		t.Errorf("next offered %+v at a column read as a work column", offer.Card)
	}
}

// onlyOffer reads the one offer a named column makes and fails unless exactly
// one came back.
func onlyOffer(t *testing.T, h *harness, column string) Offer {
	t.Helper()
	h.reopen()
	offers, err := h.library.Next(&Request{Verb: "next", Actor: "bo", Column: column})
	if err != nil {
		t.Fatalf("next %s: %v", column, err)
	}
	if len(offers) != 1 {
		t.Fatalf("next %s returned %d offers", column, len(offers))
	}
	return offers[0]
}

// TestTheAffordancesAgreeWithWhatTheActsPermit is dinah-273 AC-40. Every
// card-shaped response carries a list of what a caller may do next, and a list
// naming an act the tool refuses is worse than no list at all, because the
// reader most likely to act on it is an agent that cannot see the board.
//
// The kind guard cannot catch this class and no source scan can. A list holding
// the literal "claim" compares nothing and names no kind; it fails by not
// asking, and an absence is invisible to a scan. What catches it is running the
// act: for a card standing at each kind of column, what the list offers and what
// the act does are held against each other here.
//
// Both acts are run on every case, and that is the point rather than
// thoroughness. An affordance list is only ever wrong by offering too much, so
// a case that runs one act and stops as soon as the list offered it is silent
// in the one direction where the defect lives. An earlier shape of this test
// returned as soon as the list offered a pull, which is the intake column and
// the buffer, and those are the two columns the defect it was written for stood
// at: the claim could be put back beside the pull and the suite stayed green.
func TestTheAffordancesAgreeWithWhatTheActsPermit(t *testing.T) {
	cases := []struct {
		name    string
		column  string
		waiting bool
	}{
		{name: "an intake column", column: bufferIntake},
		{name: "a buffer", column: bufferQueue},
		{name: "a working station", column: bufferDoing},
		{name: "a done column", column: bufferDone},
		{name: "a column waiting on somebody outside", column: bufferDoing, waiting: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newBufferHarness(t)
			if c.waiting {
				h.declare(c.column, "awaiting_outside", "true")
			}
			ref := h.add("standing where the list is read")
			h.at(ref, c.column)
			offered := h.affordances(ref)

			// The claim runs first and it runs on every case. A refused claim
			// consumes nothing, so at the columns where the pull is the act
			// this costs the pull below nothing, and it is what makes the
			// intake column and the buffer answer for their claim at all.
			claim := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
			claimable := claim.Outcome == contract.OutcomeOK
			if contains(offered, Claim) != claimable {
				t.Fatalf("the list %v offers claim: %t, and the claim answered %s %s",
					offered, contains(offered, Claim), claim.Outcome, claim.Refusal)
			}
			if claimable {
				// A claim that succeeded took the card up, so hand it back.
				// Otherwise the pull below would be answering what the claim
				// did rather than what the list said.
				h.mustDo(&Request{Verb: Release, Card: ref, Actor: "alka"})
			}

			pull := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: "doing"})
			h.reopen()
			took := pull.Outcome == contract.OutcomeOK && pull.Card != nil && pull.Card.Ref == ref
			if contains(offered, Pull) != took {
				t.Fatalf("the list %v offers pull: %t, and a pull into Doing answered %s %s carrying %+v",
					offered, contains(offered, Pull), pull.Outcome, pull.Refusal, pull.Card)
			}
		})
	}
}

// TestTheAffordancesOfferThePullWhereverAPullCouldTakeTheCard is dinah-273
// AC-41. It is the other half of the agreement above, which proves the list
// never offers what the acts refuse but would also pass if the list offered
// nothing anywhere. A column that takes no work up loses the claim, so a reader
// left with neither act has been told the card is stuck when it is not.
func TestTheAffordancesOfferThePullWhereverAPullCouldTakeTheCard(t *testing.T) {
	for _, column := range []string{bufferIntake, bufferQueue} {
		h := newBufferHarness(t)
		ref := h.add("waiting for somebody downstream")
		h.at(ref, column)
		if offered := h.affordances(ref); !contains(offered, Pull) {
			t.Errorf("a card at %s is taken by a pull into Doing and its list reads %v", column, offered)
		}
	}
}
