package verb

import (
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
	// Seed one ready card in intake and one ready card in doing.
	h.add("ready in intake")
	ref := h.add("moves into doing")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: "doing"})
	h.mustDo(&Request{Verb: Release, Card: ref, Actor: "alka"})
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
		first := h.add("first")
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
		first := h.add("first")
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
		// The destination is finished, whose upstream is the fixture's
		// operator-owned review state, because the row reads the state the
		// card departs rather than the state it arrives in: a state is
		// operator-owned to say who may take work out of it, so pulling
		// into one is as legal for anybody as moving into one.
		ref := h.add("waiting in review")
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: review})
		response := h.library.Pull(&Request{Verb: Pull, Actor: "bob", State: finished})
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotOperator {
			t.Fatalf("wanted %s, got %s %s", contract.NotOperator, response.Outcome, response.Refusal)
		}
	})

	t.Run("the operator pulling out of an operator-owned upstream is admitted", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("waiting in review")
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: review})
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: finished})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("the operator should be admitted, got %s %s", response.Outcome, response.Refusal)
		}
		if response.Card == nil || response.Card.State != finished {
			t.Errorf("expected the card in finished, got %+v", response.Card)
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
		h.mustDo(&Request{Verb: Claim, Card: seeded, Actor: "alka"})
		h.mustDo(&Request{Verb: Move, Card: seeded, Actor: "alka", State: "doing"})
		h.mustDo(&Request{Verb: Release, Card: seeded, Actor: "alka"})
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
	// the done state) and then try to pull forward into aftercare, where
	// finished is the upstream and KindDone filters the destination out of
	// the qualifier, while the named form lands on terminal.
	ref := h.add("finished")
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: finished})

	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: aftercare})
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

// TestAPullWithHeldSiblingRefusesLocked asserts the race window the
// qualifier cannot catch. A sibling planted after the qualifier runs but
// before the inner pull takes its lock refuses locked, and the journal
// records no claim or move because the inner pull never reached its
// journal step.
//
// The sibling is planted by the Interleave hook so the qualifier has
// already run by the time it stands up, and the refused pull records
// neither claimed nor moved because the inner pull's journal step lives
// after the precondition checks.
func TestAPullWithHeldSiblingRefusesLocked(t *testing.T) {
	h := newHarness(t)
	ref := h.add("racy")
	card := h.card(ref)

	h.library.Interleave = func() {
		// Plant the sibling as if the other process had archived the card
		// out from under the pull. The race is structural: any Op that
		// plants a sibling where the card's lockfile expects none suffices.
		h.plant(bench.SiblingPath(card.Dir), bench.LockRecord{
			Actor: "bob",
			PID:   1234,
			TS:    bench.Stamp(h.clock),
			Op:    bench.OpArchive,
		})
	}
	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing"})
	h.library.Interleave = nil
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

// TestPullChecksAgainstTheFullThirteenRowTable asserts that the ordered
// precondition list Pull's help is generated from is the workbench pair
// followed by the eleven pull rows the spec owns, in the order the spec
// names them. This is the test the help renders against.
func TestPullChecksAgainstTheFullThirteenRowTable(t *testing.T) {
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
		{Refusal: contract.Locked, Key: "check.pull.11"},
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
	pulled := h.add("taken by pull")
	byHand := h.add("taken by hand")

	response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", State: "doing"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("pull: %s %s", response.Outcome, response.Refusal)
	}
	if response.Card == nil || response.Card.Ref != pulled {
		t.Fatalf("pull took %+v, wanted %s", response.Card, pulled)
	}
	// The fixture's doing state holds one card, so the pulled card is carried
	// on before the second card takes the same route. That carrying leaves a
	// fourth event on the pulled card, and the comparison reads the three
	// events both routes wrote rather than the whole journal.
	h.mustDo(&Request{Verb: Move, Card: pulled, Actor: "alka", State: review})
	h.mustDo(&Request{Verb: Claim, Card: byHand, Actor: "alka"})
	h.mustDo(&Request{Verb: Move, Card: byHand, Actor: "alka", State: "doing"})

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

// TestPullAndMoveRefuseOneCardInTheSameWords asserts the merged refusal order
// on the case that motivated it: a card standing in an operator-owned state,
// asked for by an owner who is not the operator. Both commands read the
// departure state before they read anything about the card, so both answer
// not-operator, and a lifted sequence that reordered either list would show up
// here as two different answers to one question.
//
// The Interleave hook blocks the card in the window the lock protects, which
// is the arrival the review was worried about. The card the transaction
// already re-read stays ready, so the blocked row cannot fire, and the point
// of the assertion is that neither command reaches that row in the first
// place: the departure decides it first.
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
	pulled := h.library.Pull(&Request{Verb: Pull, Actor: "bob", State: finished})
	h.library.Interleave = nil
	h.reopen()

	h.library.Interleave = blockDuringTheLock
	moved := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "bob", State: finished})
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
