package verb

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestPullAgreesWithNextWhenBothHaveAHead asserts that a pull and a next
// over the same state name the same card: both reach the head of the ready
// queue through headOfReady, and a test that drives pull on the back of
// next's offer is the same card next would have offered.
//
// Pull reaches into the destination's upstream, so the matching call to
// next asks about the upstream (intake, when the destination is doing)
// rather than the destination itself.
func TestPullAgreesWithNextWhenBothHaveAHead(t *testing.T) {
	h := newHarness(t)
	ref := h.add("agreeing")

	offers, err := h.library.Next(&Request{Verb: "next", State: "intake"})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if len(offers) != 1 || offers[0].Card == nil || offers[0].Card.Ref != ref {
		t.Fatalf("next: wanted to offer %s, got %+v", ref, offers)
	}

	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("pull: wanted ok, got %s %s", response.Outcome, response.Refusal)
	}
	if response.Card == nil || response.Card.Ref != ref {
		t.Fatalf("pull: wanted to claim %s, got %+v", ref, response.Card)
	}
}

// TestPullWritesClaimedThenMovedInOneTransaction asserts that a successful
// pull appends exactly two events to the card: a claimed event followed by
// a moved event, both stamped with the same time and the same actor, and
// neither succeeded without the other landing on disk. The journal is what
// the spec reads to prove the order.
func TestPullWritesClaimedThenMovedInOneTransaction(t *testing.T) {
	h := newHarness(t)
	ref := h.add("twin")
	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("pull: wanted ok, got %s %s", response.Outcome, response.Refusal)
	}
	events := h.events(ref)
	if len(events) < 2 {
		t.Fatalf("wanted at least claimed and moved, got %d events: %+v", len(events), events)
	}
	// Find the trailing two events, since a card's journal also carries its
	// created event from when add filed it.
	var claimed, moved int
	var claimedTS, movedTS string
	for _, ev := range events {
		switch ev.Event {
		case contract.EventClaimed:
			claimed++
			claimedTS = ev.TS
		case contract.EventMoved:
			moved++
			movedTS = ev.TS
		}
	}
	if claimed != 1 || moved != 1 {
		t.Errorf("wanted one claimed and one moved event, got %d / %d", claimed, moved)
	}
	if claimedTS != movedTS {
		t.Errorf("the claimed and moved events should share a timestamp, got %q and %q", claimedTS, movedTS)
	}
	if !orderThen(events, contract.EventClaimed, contract.EventMoved) {
		t.Errorf("claimed should precede moved in the journal: %+v", events)
	}
}

// orderThen reports whether, scanning events in order, the first occurrence of
// first precedes the first occurrence of second. Both events are assumed to
// appear exactly once each.
func orderThen(events []bench.Event, first, second string) bool {
	var firstAt, secondAt = -1, -1
	for i, ev := range events {
		if firstAt == -1 && ev.Event == first {
			firstAt = i
		}
		if secondAt == -1 && ev.Event == second {
			secondAt = i
		}
	}
	return firstAt != -1 && secondAt != -1 && firstAt < secondAt
}

// TestPullWithNoClaimLeavesTheCardReadyWithoutAHolder asserts that a pull
// naming --no-claim moves a card but does not stamp a claim: the card lands
// where the caller asked, but its substate stays ready, no holder is set,
// no expiry is recorded, and the journal carries only the moved event.
func TestPullWithNoClaimLeavesTheCardReadyWithoutAHolder(t *testing.T) {
	h := newHarness(t)
	ref := h.add("loaned")
	before := h.card(ref)
	if before.Substate != contract.SubstateReady || before.Holder != "" {
		t.Fatalf("preflight: wanted ready and unheld, got %s / %q", before.Substate, before.Holder)
	}

	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing", NoClaim: true})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("pull --no-claim: wanted ok, got %s %s", response.Outcome, response.Refusal)
	}
	card := h.card(ref)
	if card.State != doing {
		t.Errorf("state: wanted doing, got %s", card.State)
	}
	if card.Substate != contract.SubstateReady {
		t.Errorf("--no-claim should leave the card ready, got %s", card.Substate)
	}
	if card.Holder != "" || card.ClaimSince != "" || card.Expires != "" {
		t.Errorf("--no-claim should not stamp a holder, got holder=%q since=%q expires=%q",
			card.Holder, card.ClaimSince, card.Expires)
	}

	var claimed, moved int
	for _, ev := range h.events(ref) {
		switch ev.Event {
		case contract.EventClaimed:
			claimed++
		case contract.EventMoved:
			moved++
		}
	}
	if claimed != 0 || moved != 1 {
		t.Errorf("wanted no claimed and one moved event, got %d / %d", claimed, moved)
	}
}

// TestAnEmptyUpstreamAnswersOkWithNoCard asserts that a pull whose
// destination has no upstream ready card exits zero with the named-form
// empty answer, and writes nothing to any card's journal.
func TestAnEmptyUpstreamAnswersOkWithNoCard(t *testing.T) {
	h := newHarness(t)
	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("wanted ok for an empty upstream, got %s %s", response.Outcome, response.Refusal)
	}
	if response.Card != nil {
		t.Errorf("an empty upstream should answer without a card, got %+v", response.Card)
	}
	if response.Message != "answer.pull.empty.named" {
		t.Errorf("wanted the named-form empty answer, got %q", response.Message)
	}
	if got := len(h.benchEvents()); got != 0 {
		t.Errorf("an empty pull should write no workbench events, got %d", got)
	}
}

// TestTheBareFormPicksTheOnlyReadyUpstream asserts that a bare-form pull
// chooses the only state whose upstream holds a ready card, journals the
// card's claimed and moved events, and answers ok with the chosen state.
func TestTheBareFormPicksTheOnlyReadyUpstream(t *testing.T) {
	h := newHarness(t)
	ref := h.add("chosen")

	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("wanted ok, got %s %s", response.Outcome, response.Refusal)
	}
	if response.Card == nil || response.Card.Ref != ref {
		t.Fatalf("the bare form should have chosen %s, got %+v", ref, response.Card)
	}
	if response.Card.State != doing {
		t.Errorf("the chosen card should be in doing, got %s", response.Card.State)
	}
}

// TestTheBareFormRefusesAmbiguousUpstreams asserts that a pull with no
// destination name refused on multiple qualifying upstream ready cards
// reports the names of every qualifying state, so a person can tell the
// caller which one to type. The fixture's doing has capacity 1 and a
// linear flow, which makes it structurally impossible for the bare form
// to find two qualifying destinations at once; this test stands up its
// own wider bench where intake feeds a doing with capacity 2 and doing
// feeds a review, then seeds one ready card in intake and one ready
// card in doing, so both doing and review qualify.
func TestTheBareFormRefusesAmbiguousUpstreams(t *testing.T) {
	h := newAmbiguousHarness(t)
	// After the harness stands up, intake holds one ready card and doing
	// holds one ready card. Now add another card to intake so doing has
	// two ready cards upstream of it; doing's capacity is 2 so it still
	// qualifies. The qualifier should see doing (intake has ready, doing
	// under cap) and review (doing has ready, review no cap) as two
	// distinct qualifying destinations.
	h.add("another in intake")

	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka"})
	h.reopen()
	if response.Outcome != contract.OutcomeRefused {
		t.Fatalf("wanted refused for an ambiguous bare pull, got %s", response.Outcome)
	}
	if response.Refusal != contract.AmbiguousState {
		t.Errorf("wanted %s, got %s", contract.AmbiguousState, response.Refusal)
	}
	if _, ok := response.Context["states"]; !ok {
		t.Errorf("the refusal should carry the qualifying states in context, got %+v", response.Context)
	}
}

// newAmbiguousHarness builds a bench wide enough for the bare form to see
// more than one qualifying destination. intake -> doing (capacity 2) ->
// review -> finished. The wider bench is what lets the bare form
// distinguish "two states both qualify" from "one state, one candidate".
func newAmbiguousHarness(t *testing.T) *harness {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	root := filepath.Join(base, "workbench")
	if err := os.MkdirAll(filepath.Join(home, bench.UserBaseName), 0o755); err != nil {
		t.Fatalf("user base: %v", err)
	}
	definition, err := bench.ReadDefinition([]byte(ambiguousDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := bench.Instantiate(root, "amb", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	clock := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	h := &harness{home: home, root: root, clock: clock, t: t}
	h.reopen()
	// Seed one ready card in intake and one ready card in doing. The second
	// is carried by a move rather than claimed and released, because no owner
	// takes work up at an intake state.
	h.add("ready in intake")
	ref := h.add("moves into doing")
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: "doing"})
	return h
}

const ambiguousDefinition = `{
  "profile": "dinah-core/1.0",
  "title": "Wide",
  "instructions": "Standing text.\n",
  "states": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 2 },
    { "id": "a00000000003", "title": "Review", "kind": "work" },
    { "id": "a00000000004", "title": "Finished", "kind": "done" }
  ]
}`

// TestTheBareFormAnswersEmptyWhenNothingQualifies asserts that a pull with
// no destination name on a bench with no upstream ready card answers the
// bare-form empty answer and writes nothing.
func TestTheBareFormAnswersEmptyWhenNothingQualifies(t *testing.T) {
	h := newHarness(t)
	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("wanted ok for a bare pull with no upstream, got %s %s", response.Outcome, response.Refusal)
	}
	if response.Card != nil {
		t.Errorf("the bare form should answer without a card, got %+v", response.Card)
	}
	if response.Message != "answer.pull.empty.bare" {
		t.Errorf("wanted the bare-form empty answer, got %q", response.Message)
	}
}

// TestNoOwnerIsTheFirstRefusalInBothForms asserts that a pull naming no
// actor is refused no-owner regardless of whether it names a destination.
func TestNoOwnerIsTheFirstRefusalInBothForms(t *testing.T) {
	cases := []struct {
		name string
		req  *Request
	}{
		{"the bare form", &Request{Verb: Pull}},
		{"the named form", &Request{Verb: Pull, State: "doing"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			response := h.library.Pull(c.req)
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NoOwner {
				t.Fatalf("wanted %s, got %s %s", contract.NoOwner, response.Outcome, response.Refusal)
			}
		})
	}
}

// TestNotOperatorAdmitsTheOperatorCarryingOverrideOnly asserts that a
// non-operator carrying --override on a pull is refused not-operator, and
// that the operator carrying --override is admitted regardless.
func TestNotOperatorAdmitsTheOperatorCarryingOverrideOnly(t *testing.T) {
	cases := []struct {
		name    string
		req     *Request
		refusal string
	}{
		{"a non-operator carrying override in the bare form",
			&Request{Verb: Pull, Actor: "bob", Override: true},
			contract.NotOperator,
		},
		{"a non-operator carrying override in the named form",
			&Request{Verb: Pull, Actor: "bob", State: "doing", Override: true},
			contract.NotOperator,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.add("anything")
			response := h.library.Pull(c.req)
			if response.Outcome != contract.OutcomeRefused || response.Refusal != c.refusal {
				t.Fatalf("wanted %s, got %s %s", c.refusal, response.Outcome, response.Refusal)
			}
		})
	}

	// The operator carrying --override is admitted on a state that has
	// already reached its capacity (the workbench owns alka as operator).
	t.Run("the operator carrying override is admitted", func(t *testing.T) {
		h := newHarness(t)
		first := h.ready("first")
		h.mustDo(&Request{Verb: Claim, Card: first, Actor: "alka"})
		h.mustDo(&Request{Verb: Move, Card: first, Actor: "alka", State: "doing"})
		second := h.add("second")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing", Override: true})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("operator with override: wanted ok, got %s %s", response.Outcome, response.Refusal)
		}
		if response.Card == nil || response.Card.Ref != second {
			t.Fatalf("expected second, got %+v", response.Card)
		}
	})
}

// TestCapacityAndRetiringRefuseUnderEachForm asserts the two preconditions
// the operator overrides but a regular asker cannot: a destination already
// at capacity refuses at-capacity, and a retiring destination refuses
// locked. The capacity refusal covers both forms by reaching into the same
// state the pull is being asked to carry.
func TestCapacityAndRetiringRefuseUnderEachForm(t *testing.T) {
	t.Run("at-capacity refuses both forms", func(t *testing.T) {
		h := newHarness(t)
		first := h.ready("first")
		h.mustDo(&Request{Verb: Claim, Card: first, Actor: "alka"})
		h.mustDo(&Request{Verb: Move, Card: first, Actor: "alka", State: "doing"})
		h.add("second")
		// The bare form sees two qualifying destinations (doing) and refuses
		// ambiguous-state only when its predicate allows capacity through, so
		// we drive the named form to land on at-capacity.
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing"})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused {
			t.Fatalf("the named form: wanted refused, got %s", response.Outcome)
		}
		if response.Refusal != contract.AtCapacity {
			t.Errorf("wanted %s, got %s", contract.AtCapacity, response.Refusal)
		}
	})

	t.Run("a retiring state refuses locked", func(t *testing.T) {
		h := newHarness(t)
		// Plant the sibling beside the doing state directory so the bench
		// reads it as a retirement in flight. The state remains declared
		// and its directory is intact, which is the shape an archive
		// aborts in: the sibling stands, and the act has not yet removed
		// the identifier from workbench.md. A pull into that state refuses
		// locked for the same reason the qualifier leaves it out of the
		// bare form's set.
		//
		// The destination is doing rather than aftercare because the
		// refusal has to be reachable. Row 13 is evaluated under the
		// card's lock, so a card must be standing ready in the upstream
		// state for the pull to get that far, and aftercare's upstream is
		// the done state, which refuses terminal at row 11 first.
		dir := filepath.Join(h.root, bench.StatesDir, doing)
		h.plant(bench.SiblingPath(dir), bench.LockRecord{
			Actor: "alka",
			PID:   4321,
			TS:    bench.Stamp(h.clock),
			Op:    bench.OpArchive,
		})
		h.add("waiting in intake")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: doing})
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.Locked {
			t.Fatalf("the named form into a retiring state: wanted %s, got %s %s",
				contract.Locked, response.Outcome, response.Refusal)
		}
	})

	t.Run("a retiring state with nothing waiting answers empty", func(t *testing.T) {
		h := newHarness(t)
		// The same retiring destination, with its upstream left empty.
		// Rows 8 to 13 read the one card the selection chose, so with no
		// card to choose the pull never reaches them and answers the
		// empty answer rather than the refusal above. Both forms answer a
		// pull that finds nothing to take the same way, which is what
		// keeps one fact from having two vocabularies.
		dir := filepath.Join(h.root, bench.StatesDir, review)
		h.plant(bench.SiblingPath(dir), bench.LockRecord{
			Actor: "alka",
			PID:   4321,
			TS:    bench.Stamp(h.clock),
			Op:    bench.OpArchive,
		})
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: review})
		if response.Outcome != contract.OutcomeOK || response.Card != nil {
			t.Fatalf("wanted the empty answer, got %s %s %+v",
				response.Outcome, response.Refusal, response.Card)
		}
	})
}

// TestNoUpstreamRefusesNamedPullsToTheFirstState asserts the named form
// against a destination with no upstream: the qualifier answers empty in
// the bare form and refuses no-upstream in the named form, so the refusal
// is only reachable when the caller typed a destination by name.
func TestNoUpstreamRefusesNamedPullsToTheFirstState(t *testing.T) {
	h := newHarness(t)
	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: intake})
	h.reopen()
	if response.Outcome != contract.OutcomeRefused {
		t.Fatalf("named pull into the first state: wanted refused, got %s", response.Outcome)
	}
	if response.Refusal != contract.NoUpstream {
		t.Errorf("wanted %s, got %s", contract.NoUpstream, response.Refusal)
	}
	if response.Context["state"] != "intake" {
		t.Errorf("the no-upstream refusal should name the destination by its slug, got %+v", response.Context)
	}
}

// TestOperatorOwnedUpstreamFiltersAndAdmitsTheOperator asserts the
// operator-only precondition. A pull whose destination is operator-owned
// is admitted for the operator and refused not-operator for everyone else,
// in the named form. The bare form cannot produce the operator-owned
// refusal on its own, because the qualifier excludes operator-owned
// destinations from a non-operator's qualifying set: the pull either picks
// the only remaining state or refuses no upstream at all.
func TestOperatorOwnedUpstreamFiltersAndAdmitsTheOperator(t *testing.T) {
	t.Run("a non-operator pulling out of an operator-owned upstream is refused", func(t *testing.T) {
		h := newHarness(t)
		// The destination is aftercare, whose upstream is the fixture's
		// operator-owned review state, because this row reads the state the
		// card departs rather than the state it arrives in. The destination
		// side of the same reservation is a second row, which
		// TestANamedPullIntoAnOperatorOwnedDestinationRefusesUnlessOperator
		// covers, so a refusal here says nothing about where the card was
		// going.
		ref := h.add("waiting in review")
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: review})
		response := h.library.Pull(&Request{Verb: Pull, Actor: "bob", State: aftercareSlug})
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotOperator {
			t.Fatalf("wanted %s, got %s %s", contract.NotOperator, response.Outcome, response.Refusal)
		}
	})

	t.Run("the operator pulling out of an operator-owned upstream is admitted", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("waiting in review")
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: review})
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: aftercareSlug})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("the operator should be admitted, got %s %s", response.Outcome, response.Refusal)
		}
		if response.Card == nil || response.Card.State != aftercare {
			t.Errorf("expected the card in aftercare, got %+v", response.Card)
		}
	})

	t.Run("a non-operator's bare pull picks the only non-operator-owned candidate", func(t *testing.T) {
		h := newHarness(t)
		h.add("ordinary")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "bob"})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("wanted ok (a single candidate remains for bob), got %s %s", response.Outcome, response.Refusal)
		}
		if response.Card == nil || response.Card.State != doing {
			t.Errorf("the only qualifying destination for bob is doing, got %+v", response.Card)
		}
	})

	t.Run("the operator into an operator-owned state is admitted", func(t *testing.T) {
		h := newHarness(t)
		seeded := h.add("seeded into doing")
		h.mustDo(&Request{Verb: Move, Card: seeded, Actor: "alka", State: "doing"})
		h.add("review me")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: review})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("the operator should be admitted, got %s %s", response.Outcome, response.Refusal)
		}
		if response.Card == nil || response.Card.State != review {
			t.Errorf("expected a card in review, got %+v", response.Card)
		}
	})
}

// TestADoneUpstreamRefusesTerminal asserts that pulling into a state whose
// upstream is itself a done state refuses terminal, since the qualifier
// filters the destination out and a named pull lands on terminal.
func TestADoneUpstreamRefusesTerminal(t *testing.T) {
	h := newHarness(t)
	h.add("anywhere")
	// Walk a card into finished (without claiming it, so it stays ready in
	// the done state) and then try to pull forward into closed, where
	// finished is the upstream and a done state carries no card into the
	// state beyond it, while the named form lands on terminal.
	ref := h.add("finished")
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: finished})

	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: closed})
	h.reopen()
	if response.Outcome != contract.OutcomeRefused {
		t.Fatalf("named pull into a state whose upstream is done: wanted refused, got %s", response.Outcome)
	}
	if response.Refusal != contract.Terminal {
		t.Errorf("wanted %s, got %s", contract.Terminal, response.Refusal)
	}
	if response.Detail != "finished" {
		t.Errorf("the terminal refusal should name the upstream done state by its slug, got %q", response.Detail)
	}
}

// TestAPullWithHeldSiblingRefusesLocked asserts that a structural act
// standing on the card refuses the pull, and that the refused pull writes
// neither claimed nor moved.
//
// The refusal is bench.Acquire's own: the acquisition reads for a sibling and
// gives the lock straight back when it finds one, which is why no card verb
// carries a second copy of that read and why pull no longer carries one
// either. The sibling therefore stands before the pull is asked for, which is
// also the state a real archive in flight leaves on disk.
func TestAPullWithHeldSiblingRefusesLocked(t *testing.T) {
	h := newHarness(t)
	ref := h.add("racy")
	card := h.card(ref)

	// Plant the sibling as if another process were archiving the card. The
	// obstruction is structural: any Op that plants a sibling beside the
	// card's directory suffices.
	h.plant(bench.SiblingPath(card.Dir), bench.LockRecord{
		Actor: "bob",
		PID:   1234,
		TS:    bench.Stamp(h.clock),
		Op:    bench.OpArchive,
	})
	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing"})
	h.reopen()

	if response.Outcome != contract.OutcomeRefused {
		t.Fatalf("pull with held sibling: wanted refused, got %s", response.Outcome)
	}
	if response.Refusal != contract.Locked {
		t.Errorf("wanted %s, got %s", contract.Locked, response.Refusal)
	}
	// The inner pull refuses before the journal step, so neither claim
	// nor move lands.
	for _, ev := range h.events(ref) {
		if ev.Event == contract.EventClaimed || ev.Event == contract.EventMoved {
			t.Errorf("the refused pull wrote %s", ev.Event)
		}
	}
	// The card is still where the qualifier saw it: intake, ready, unheld.
	card = h.card(ref)
	if card.State != intake {
		t.Errorf("the refused pull moved the card anyway: %s", card.State)
	}
}

// TestANamedPullIntoAnOperatorOwnedDestinationRefusesUnlessOperator drives
// CORE-CLAIM-8 at the far end of a pull. A pull lands its card holding it, so
// the destination reserves that claim exactly as the state a plain claim is
// asked at does, and a named pull is the form that reaches the row: the bare
// form's qualifier reads the source rather than the destination.
//
// The --no-claim form is refused on the same terms, because the option changes
// what a pull writes rather than what a pull allows, which is the rule
// TestANamedPullIsRefusedAtAQueueDestination already holds for the row above
// this one in canLand.
//
// The departure is doing, which is not itself operator-owned, so nothing here
// can be answered by the departure-side row
// TestOperatorOwnedUpstreamFiltersAndAdmitsTheOperator covers.
func TestANamedPullIntoAnOperatorOwnedDestinationRefusesUnlessOperator(t *testing.T) {
	t.Run("a non-operator claiming is refused", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("upstream of review")
		h.at(ref, doing)

		response := h.library.Pull(&Request{Verb: Pull, Actor: "bob", State: review})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotOperator {
			t.Fatalf("wanted %s, got %s %s", contract.NotOperator, response.Outcome, response.Refusal)
		}
		if card := h.card(ref); card.State != doing || card.Holder != "" {
			t.Errorf("the refused pull wrote to the card: state %q holder %q", card.State, card.Holder)
		}
	})

	t.Run("a non-operator carrying --no-claim is refused on the same terms", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("upstream of review")
		h.at(ref, doing)

		response := h.library.Pull(&Request{Verb: Pull, Actor: "bob", State: review, NoClaim: true})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotOperator {
			t.Fatalf("wanted %s, got %s %s", contract.NotOperator, response.Outcome, response.Refusal)
		}
		if card := h.card(ref); card.State != doing {
			t.Errorf("the refused pull moved the card anyway: %s", card.State)
		}
	})

	t.Run("the operator carrying --no-claim is admitted", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("upstream of review")
		h.at(ref, doing)

		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: review, NoClaim: true})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("the operator should be admitted, got %s %s", response.Outcome, response.Refusal)
		}
		card := h.card(ref)
		if card.State != review {
			t.Errorf("expected the card in review, got %s", card.State)
		}
		if card.Substate != contract.SubstateReady || card.Holder != "" {
			t.Errorf("a pull carrying --no-claim should leave the card ready and unheld: substate %q holder %q",
				card.Substate, card.Holder)
		}
	})
}

// TestPullChecksAgainstTheFullFourteenRowTable asserts that the ordered
// precondition list Pull's help is generated from is the workbench pair
// followed by the twelve pull rows the spec owns, in the order the spec
// names them. This is the test the help renders against.
func TestPullChecksAgainstTheFullFourteenRowTable(t *testing.T) {
	checks := Checks(Pull)
	want := []Check{
		{Refusal: contract.UnsupportedVer, Key: "check.workbench.1"},
		{Refusal: contract.NoOperator, Key: "check.workbench.2"},
		{Refusal: contract.NoOwner, Key: "check.pull.1"},
		{Refusal: contract.UnknownState, Key: "check.pull.2"},
		{Refusal: contract.NotOperator, Key: "check.pull.3"},
		{Refusal: contract.AmbiguousState, Key: "check.pull.4"},
		{Refusal: contract.NoUpstream, Key: "check.pull.5"},
		{Refusal: contract.NotOperator, Key: "check.pull.6"},
		{Refusal: contract.Blocked, Key: "check.pull.7"},
		{Refusal: contract.Held, Key: "check.pull.8"},
		{Refusal: contract.Terminal, Key: "check.pull.9"},
		{Refusal: contract.AtCapacity, Key: "check.pull.10"},
		{Refusal: contract.NotOperator, Key: "check.pull.11"},
		{Refusal: contract.Locked, Key: "check.pull.12"},
	}
	if len(checks) != len(want) {
		t.Fatalf("wanted %d rows, got %d", len(want), len(checks))
	}
	for i, row := range want {
		if checks[i].Refusal != row.Refusal || checks[i].Key != row.Key {
			t.Errorf("row %d: wanted %+v, got %+v", i, row, checks[i])
		}
	}
}

// TestClaimChecksAgainstTheFullSixRowTable is the claim's half of the same
// assertion, and it is what pins the new row's position: the claim's own list
// ends with the operator-owned reservation, behind the workbench pair every
// contract verb carries in front of its own rows.
func TestClaimChecksAgainstTheFullSixRowTable(t *testing.T) {
	own, found := ownChecks(Claim)
	if !found {
		t.Fatal("the claim declares no precondition list")
	}
	wantOwn := []Check{
		{Refusal: contract.UnknownCard, Key: "check.claim.1"},
		{Refusal: contract.NoOwner, Key: "check.claim.2"},
		{Refusal: contract.NotRequester, Key: "check.claim.3"},
		{Refusal: contract.Blocked, Key: "check.claim.4"},
		{Refusal: contract.Held, Key: "check.claim.5"},
		{Refusal: contract.NotOperator, Key: "check.claim.6"},
	}
	if len(own) != len(wantOwn) {
		t.Fatalf("wanted %d own rows, got %d", len(wantOwn), len(own))
	}
	for i, row := range wantOwn {
		if own[i].Refusal != row.Refusal || own[i].Key != row.Key {
			t.Errorf("own row %d: wanted %+v, got %+v", i, row, own[i])
		}
	}
	want := append(append([]Check{}, WorkbenchChecks...), wantOwn...)
	checks := Checks(Claim)
	if len(checks) != len(want) {
		t.Fatalf("wanted %d rows, got %d", len(want), len(checks))
	}
	for i, row := range want {
		if checks[i].Refusal != row.Refusal || checks[i].Key != row.Key {
			t.Errorf("row %d: wanted %+v, got %+v", i, row, checks[i])
		}
	}
}

// TestTheQuickStartReplayMentionsPull was retired. The quick-start replay
// guard lives at cmd/dinah/quickstart_test.go, not here; asserting it here
// would duplicate that test's contract.

// TestAPulledCardsJournalMatchesTheThreeCommandRoute asserts that a pull
// leaves the history a person would have written by hand. The two events a
// pull appends carry the same names, actors and details, in the same order,
// as the events a claim and a move append separately, so a reader who has
// never used the command cannot tell which route a card took.
//
// Timestamps are excluded because the two routes are two acts at two moments
// on one route and one act on the other, which is the difference the card is
// about rather than a difference in what was recorded.
func TestAPulledCardsJournalMatchesTheThreeCommandRoute(t *testing.T) {
	h := newHarness(t)
	pulled := h.readyAt("taken by pull", review)
	byHand := h.readyAt("taken by hand", review)

	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: aftercareSlug})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("pull: %s %s", response.Outcome, response.Refusal)
	}
	if response.Card == nil || response.Card.Ref != pulled {
		t.Fatalf("pull took %+v, wanted %s", response.Card, pulled)
	}
	// Both cards start at the review station, because a claim is refused
	// where no owner takes work up and the hand route claims before it
	// moves. The comparison reads the events both routes wrote rather than
	// the whole journal.
	h.mustDo(&Request{Verb: Claim, Card: byHand, Actor: "alka"})
	h.mustDo(&Request{Verb: Move, Card: byHand, Actor: "alka", State: aftercareSlug})

	right := h.events(byHand)
	left := h.events(pulled)
	if len(left) < len(right) {
		t.Fatalf("the pulled card wrote %d events and the hand route wrote %d: %+v against %+v", len(left), len(right), left, right)
	}
	left = left[:len(right)]
	for i := range left {
		a, b := left[i], right[i]
		if a.Event != b.Event || a.Actor != b.Actor || a.From != b.From || a.To != b.To || a.Override != b.Override {
			t.Errorf("event %d differs between the routes: %+v against %+v", i+1, a, b)
		}
	}
}

// hashCardDirectory sums a card directory's contents by path and by bytes,
// skipping the lock file, which stands only for the length of the
// transaction and so differs between a reading taken under the lock and one
// taken after it was given back. What remains is the card itself: its anchor,
// its journal, and anything filed beneath it.
func hashCardDirectory(t *testing.T, dir string) string {
	t.Helper()
	sum := sha256.New()
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if entry.Name() == bench.LockName {
			return nil
		}
		relative, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		sum.Write([]byte(filepath.ToSlash(relative)))
		sum.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("hash %s: %v", dir, err)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// TestACardThatStopsBeingReadyUnderTheLockRefusesInTheClaimsWords asserts the
// race the refusal table describes: the selection reads a ready card, another
// process takes or blocks it before the lock closes, and the pull refuses
// held or blocked in the sentence claim raises for the same condition rather
// than choosing a different card or reporting a stale read.
//
// The Interleave hook stands for the other process. It fires after the lock
// is taken and before the card is read, which is the window the selection
// leaves open. The hook records what it left on disk, and the assertion holds
// the card directory against that reading afterwards, so the refusal is shown
// to have written nothing at all rather than merely to have written no event.
func TestACardThatStopsBeingReadyUnderTheLockRefusesInTheClaimsWords(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(card *bench.Card)
		refusal string
		detail  string
	}{
		{
			name: "taken by somebody else",
			mutate: func(card *bench.Card) {
				card.Substate = contract.SubstateActive
				card.Holder = "bob"
			},
			refusal: contract.Held,
			detail:  "bob",
		},
		{
			name: "blocked by somebody else",
			mutate: func(card *bench.Card) {
				card.Substate = contract.SubstateBlocked
				card.BlockReason = "the printer is on fire"
			},
			refusal: contract.Blocked,
			detail:  "the printer is on fire",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ref := h.add("overtaken")
			dir := h.card(ref).Dir

			var left string
			h.library.Interleave = func() {
				card, err := bench.LoadCard(h.library.Bench.CardsRoot(), h.card(ref).ID)
				if err != nil {
					t.Fatalf("load the card mid-transaction: %v", err)
				}
				tc.mutate(card)
				if err := card.Save(); err != nil {
					t.Fatalf("overtake the card mid-transaction: %v", err)
				}
				left = hashCardDirectory(t, dir)
			}
			response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing"})
			h.library.Interleave = nil
			h.reopen()

			if response.Outcome != contract.OutcomeRefused || response.Refusal != tc.refusal {
				t.Fatalf("wanted %s, got %s %s", tc.refusal, response.Outcome, response.Refusal)
			}
			if response.Detail != tc.detail {
				t.Errorf("the refusal should name %q, got %q", tc.detail, response.Detail)
			}
			if after := hashCardDirectory(t, dir); after != left {
				t.Errorf("the refused pull wrote to the card: %s became %s", left, after)
			}
			for _, ev := range h.events(ref) {
				if ev.Event == contract.EventClaimed || ev.Event == contract.EventMoved {
					t.Errorf("the refused pull wrote %s", ev.Event)
				}
			}
		})
	}
}

// TestPullReadsTheCardRowsBeforeTheDestinationRows asserts the half of the
// merged order the departure case cannot reach: row 10 reads the card, rows
// 11 to 13 read the destination, and the card decides first.
//
// The card is claimed under the lock by the owner asking for the pull, and
// the destination stands at its capacity. That pair is the only one that
// separates the two orders. The move's own held row admits a card its asker
// holds, so a pull running the move's list whole before the claim's substate
// pair reaches the capacity row and answers at-capacity. The table says the
// answer is held, because row 10 takes the claim's stricter test and a claim
// cannot take a card that is already active.
func TestPullReadsTheCardRowsBeforeTheDestinationRows(t *testing.T) {
	h := newHarness(t)
	// The fixture's doing state holds one card, so one card standing in it
	// puts it at its limit.
	occupying := h.add("occupying doing")
	h.mustDo(&Request{Verb: Move, Card: occupying, Actor: "alka", State: "doing"})
	ref := h.add("claimed under the lock")

	h.library.Interleave = func() {
		card, err := bench.LoadCard(h.library.Bench.CardsRoot(), h.card(ref).ID)
		if err != nil {
			t.Fatalf("load the card mid-transaction: %v", err)
		}
		card.Substate = contract.SubstateActive
		card.Holder = "alka"
		if err := card.Save(); err != nil {
			t.Fatalf("claim the card mid-transaction: %v", err)
		}
	}
	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing"})
	h.library.Interleave = nil
	h.reopen()

	if response.Refusal != contract.Held {
		t.Fatalf("wanted %s, got %s %s", contract.Held, response.Outcome, response.Refusal)
	}
	if response.Detail != "alka" {
		t.Errorf("the refusal should name the holder alka, got %q", response.Detail)
	}
}

// TestPullAndMoveRefuseOneCardInTheSameWords asserts the merged refusal order
// on the case that motivated it: a card standing in an operator-owned state,
// asked for by an owner who is not the operator. Both commands read the
// departure state before they read anything about the card, so both answer
// not-operator, and a lifted sequence that reordered either list would show up
// here as two different answers to one question.
//
// The Interleave hook blocks the card in the window the lock protects, and
// the card each command reads under its lock is therefore blocked. Row 9
// would answer blocked and row 8 answers not-operator, so the assertion is a
// statement about which of the two each command reaches first. Run pull's
// rows 9 and 10 ahead of row 8, as calling the claim's list before the
// move's does, and the pull half of this test answers blocked and fails.
func TestPullAndMoveRefuseOneCardInTheSameWords(t *testing.T) {
	h := newHarness(t)
	ref := h.add("standing in review")
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: review})

	blockDuringTheLock := func() {
		card := h.card(ref)
		card.Substate = contract.SubstateBlocked
		card.BlockReason = "raised by another process"
		if err := card.Save(); err != nil {
			t.Fatalf("block the card mid-transaction: %v", err)
		}
	}

	h.library.Interleave = blockDuringTheLock
	pulled := h.library.Pull(&Request{Verb: Pull, Actor: "bob", State: aftercareSlug})
	h.library.Interleave = nil
	h.reopen()

	h.library.Interleave = blockDuringTheLock
	moved := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "bob", State: aftercareSlug})
	h.library.Interleave = nil
	h.reopen()

	if pulled.Refusal != contract.NotOperator {
		t.Errorf("pull answered %s %s, wanted %s", pulled.Outcome, pulled.Refusal, contract.NotOperator)
	}
	if moved.Refusal != contract.NotOperator {
		t.Errorf("move answered %s %s, wanted %s", moved.Outcome, moved.Refusal, contract.NotOperator)
	}
	if pulled.Refusal != moved.Refusal {
		t.Errorf("one card, two answers: pull said %s and move said %s", pulled.Refusal, moved.Refusal)
	}
}

// TestANamedPullRefusesAStateTheWorkbenchDoesNotDeclare asserts the row that
// belongs to the argument rather than to any state: a destination the
// workbench does not declare is refused in the words move refuses it, and the
// refusal names back what the caller typed.
//
// The bare form cannot reach this row, because it names no state to be
// unknown, which is why the assertion drives the named form alone.
func TestANamedPullRefusesAStateTheWorkbenchDoesNotDeclare(t *testing.T) {
	h := newHarness(t)
	h.add("waiting")
	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "nowhere"})
	if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.UnknownState {
		t.Fatalf("wanted %s, got %s %s", contract.UnknownState, response.Outcome, response.Refusal)
	}
	if response.Detail != "nowhere" {
		t.Errorf("the refusal should name what the caller typed, got %q", response.Detail)
	}
}
