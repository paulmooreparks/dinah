package verb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestRefusalOrderFollowsTheProfile asserts that a request failing several
// checks at once reports the first unsatisfied check in the profile's own
// order for that verb, which is CORE-OUT-6.
//
// Each case also drives the refusal it lands on, so the table covers
// CORE-VERB-1, CORE-VERB-2, CORE-CLAIM-1, CORE-CLAIM-2, CORE-CLAIM-7,
// CORE-MOVE-1, CORE-MOVE-2, CORE-MOVE-3, CORE-MOVE-6, CORE-MOVE-11,
// CORE-BLOCK-2, CORE-BLOCK-6 and CORE-UNBLOCK-2 as well.
func TestRefusalOrderFollowsTheProfile(t *testing.T) {
	cases := []struct {
		name    string
		build   func(*harness) *Request
		refusal string
		why     string
	}{
		{
			name: "claim reports the card before the owner",
			build: func(h *harness) *Request {
				return &Request{Verb: Claim, Card: "fx-99"}
			},
			refusal: contract.UnknownCard,
			why:     "the card's existence is checked ahead of the owner",
		},
		{
			name: "claim reports the owner before the holder it names",
			build: func(h *harness) *Request {
				ref := h.add("ordering")
				return &Request{Verb: Claim, Card: ref, Holder: "bob"}
			},
			refusal: contract.NoOwner,
			why:     "no-owner precedes not-requester",
		},
		{
			name: "claim reports the named holder before the block",
			build: func(h *harness) *Request {
				ref := h.add("ordering")
				h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "stopped"})
				return &Request{Verb: Claim, Card: ref, Actor: "alka", Holder: "bob"}
			},
			refusal: contract.NotRequester,
			why:     "not-requester precedes blocked",
		},
		{
			// Not an ordering case: a block clears the claim, so no card is
			// ever both blocked and active and the two checks cannot compete.
			// This pins the refusal itself.
			name: "claim on a blocked card reports blocked",
			build: func(h *harness) *Request {
				ref := h.ready("ordering")
				h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "bob"})
				h.mustDo(&Request{Verb: Block, Card: ref, Actor: "bob", Reason: "stopped"})
				return &Request{Verb: Claim, Card: ref, Actor: "alka"}
			},
			refusal: contract.Blocked,
			why:     "a blocked card is refused blocked whoever asks",
		},
		{
			name: "move reports the destination before the override marker",
			build: func(h *harness) *Request {
				ref := h.add("ordering")
				return &Request{Verb: Move, Card: ref, Actor: "bob", State: "nowhere", Override: true}
			},
			refusal: contract.UnknownState,
			why:     "unknown-state precedes the override check",
		},
		{
			name: "move reports the override marker before the block",
			build: func(h *harness) *Request {
				ref := h.add("ordering")
				h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "stopped"})
				return &Request{Verb: Move, Card: ref, Actor: "bob", State: doing, Override: true}
			},
			refusal: contract.NotOperator,
			why:     "the override check precedes blocked",
		},
		{
			name: "move reports the departure before the held card",
			build: func(h *harness) *Request {
				ref := h.add("ordering")
				h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: review})
				h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
				return &Request{Verb: Move, Card: ref, Actor: "bob", State: doing}
			},
			refusal: contract.NotOperator,
			why:     "the operator-owned departure precedes held",
		},
		{
			name: "move reports the block before the terminal state",
			build: func(h *harness) *Request {
				ref := h.add("ordering")
				h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: finished})
				h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "stopped"})
				return &Request{Verb: Move, Card: ref, Actor: "alka", State: finished}
			},
			refusal: contract.Blocked,
			why:     "blocked precedes terminal",
		},
		{
			name: "block reports the owner before the missing reason",
			build: func(h *harness) *Request {
				ref := h.add("ordering")
				return &Request{Verb: Block, Card: ref}
			},
			refusal: contract.NoOwner,
			why:     "no-owner precedes no-reason",
		},
		{
			name: "block reports the missing reason before the other holder",
			build: func(h *harness) *Request {
				ref := h.ready("ordering")
				h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "bob"})
				return &Request{Verb: Block, Card: ref, Actor: "alka"}
			},
			refusal: contract.NoReason,
			why:     "no-reason precedes held",
		},
		{
			name: "release reports the card before the holder",
			build: func(h *harness) *Request {
				return &Request{Verb: Release, Card: "fx-99", Actor: "bob"}
			},
			refusal: contract.UnknownCard,
			why:     "unknown-card precedes not-holder",
		},
		{
			name: "unblock reports the operator before the substate",
			build: func(h *harness) *Request {
				ref := h.add("ordering")
				return &Request{Verb: Unblock, Card: ref, Actor: "bob"}
			},
			refusal: contract.NotOperator,
			why:     "not-operator precedes not-blocked, per section 6.7",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			response := h.do(c.build(h))
			if response.Outcome != contract.OutcomeRefused {
				t.Fatalf("wanted refused, got %s", response.Outcome)
			}
			if response.Refusal != c.refusal {
				t.Errorf("wanted %s, got %s: %s", c.refusal, response.Refusal, c.why)
			}
		})
	}
}

// TestWorkbenchChecksPrecedeEveryVerb asserts that a bench designating no
// operator refuses every verb with no-operator ahead of the verb's own list,
// which is CORE-OWNER-3 read with CORE-OUT-6.
func TestWorkbenchChecksPrecedeEveryVerb(t *testing.T) {
	h := newHarness(t)
	h.add("orphan")
	h.library.Bench.FM.Delete("operator")
	h.library.Bench.Operator = ""
	for _, name := range ContractVerbs {
		response := h.library.Do(&Request{Verb: name, Card: "fx-99", Actor: "alka"})
		if response.Refusal != contract.NoOperator {
			t.Errorf("%s: wanted %s ahead of unknown-card, got %s", name, contract.NoOperator, response.Refusal)
		}
	}
}

// TestClaimTakesUpACard asserts CORE-CLAIM-3 and CORE-CARD-6: a claim that
// succeeds sets the substate to active and records the holder and the time,
// which is also what makes an active card one that carries both.
func TestClaimTakesUpACard(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("claimable")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	card := h.card(ref)
	if card.Substate != contract.SubstateActive {
		t.Errorf("substate: wanted active, got %s", card.Substate)
	}
	if card.Holder != "alka" {
		t.Errorf("holder: wanted alka, got %q", card.Holder)
	}
	if card.ClaimSince == "" {
		t.Error("claim time: wanted one, got none")
	}
}

// TestExpiryLapsesTheClaim asserts CORE-CLAIM-5 and CORE-HIST-2: after a
// recorded expiry the card reads ready and the history carries the lapse
// attributed to the owner whose claim lapsed. The expiry a claim may carry is
// CORE-CLAIM-4.
func TestExpiryLapsesTheClaim(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("leased")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "bob", Expires: 2 * time.Hour})
	if card := h.card(ref); card.Expires == "" {
		t.Fatal("expiry: wanted one recorded, got none")
	}
	h.advance(3 * time.Hour)
	h.reopen()
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	card := h.card(ref)
	if card.Holder != "alka" {
		t.Errorf("holder after the lapse: wanted alka, got %q", card.Holder)
	}
	found := false
	for _, ev := range h.events(ref) {
		if ev.Event != contract.EventExpired {
			continue
		}
		found = true
		if ev.Actor != "bob" {
			t.Errorf("expiry actor: wanted bob, got %q", ev.Actor)
		}
	}
	if !found {
		t.Error("journal: wanted an expiry act, got none")
	}
}

// TestMoveHonoursItsStatements asserts the effects and refusals of CORE-MOVE,
// including CORE-MOVE-4, the capacity count of CORE-MOVE-5 over cards of
// mixed substate, and the override of CORE-MOVE-9 and CORE-MOVE-10.
func TestMoveHonoursItsStatements(t *testing.T) {
	h := newHarness(t)
	first := h.ready("first")
	second := h.add("second")

	h.mustDo(&Request{Verb: Claim, Card: first, Actor: "alka"})
	h.mustDo(&Request{Verb: Move, Card: first, Actor: "alka", State: doing})
	card := h.card(first)
	if card.Substate != contract.SubstateActive || card.Holder != "alka" {
		t.Errorf("CORE-MOVE-8: a move changed the substate or the holder: %s %q", card.Substate, card.Holder)
	}

	// Doing is limited to one card, and the card occupying it is blocked, so
	// the count of CORE-MOVE-5 still includes it.
	h.mustDo(&Request{Verb: Block, Card: first, Actor: "alka", Reason: "stopped"})
	refused := h.do(&Request{Verb: Move, Card: second, Actor: "alka", State: doing})
	if refused.Refusal != contract.AtCapacity {
		t.Fatalf("wanted at-capacity over a blocked occupant, got %s %s", refused.Outcome, refused.Refusal)
	}
	if refused.Detail != "doing" {
		// Named by the slug the caller just typed ("doing"), not by the raw
		// identifier behind it.
		t.Fatalf("at-capacity refusal: wanted the slug %q, got %q", "doing", refused.Detail)
	}

	// The operator carries one through with the marker, and the act is one
	// act marked an override.
	admitted := h.mustDo(&Request{Verb: Move, Card: second, Actor: "alka", State: doing, Override: true})
	if admitted.Card.State != doing {
		t.Errorf("override move: wanted state %s, got %s", doing, admitted.Card.State)
	}
	overrides := 0
	moves := 0
	for _, ev := range h.events(second) {
		if ev.Event != contract.EventMoved {
			continue
		}
		moves++
		if ev.Override {
			overrides++
		}
	}
	if moves != 1 || overrides != 1 {
		t.Errorf("CORE-MOVE-10: wanted one move marked an override, got %d moves and %d marks", moves, overrides)
	}

	// A forward move out of a done state is terminal; a backward one is not.
	third := h.add("third")
	h.mustDo(&Request{Verb: Move, Card: third, Actor: "alka", State: finished})
	if response := h.do(&Request{Verb: Move, Card: third, Actor: "alka", State: intake}); response.Outcome != contract.OutcomeOK {
		t.Errorf("a backward move out of a done state should be admitted, got %s", response.Refusal)
	}
}

// TestForwardMoveOutOfDoneIsTerminal asserts CORE-MOVE-7 on its own, since the
// combined test above cannot reach it once the card has left the done state.
func TestForwardMoveOutOfDoneIsTerminal(t *testing.T) {
	h := newHarness(t)
	ref := h.add("finished")
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: finished})
	// CORE-STATE-9: the same card is offered no forward move at all, so the
	// refusal below is not the only place the rule shows.
	for _, move := range h.library.legalMoves(h.card(ref)) {
		if move.Direction == Forward {
			t.Errorf("CORE-STATE-9: a card in a done state was offered the forward move %s", move.State)
		}
	}
	response := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", State: closed})
	if response.Refusal != contract.Terminal {
		t.Fatalf("wanted terminal on a forward move out of a done state, got %s %s", response.Outcome, response.Refusal)
	}
	if response.Detail != "finished" {
		// Named by the departure state's slug ("finished"), not by its raw
		// identifier: this refusal is the one dinah-29 cycle 2's own
		// counterexample entry was written about.
		t.Fatalf("terminal refusal: wanted the slug %q, got %q", "finished", response.Detail)
	}
}

// TestReleaseGivesTheCardBack asserts CORE-RELEASE-1 and CORE-RELEASE-2, and
// with them the half of CORE-CARD-7 that says a ready card carries no holder.
func TestReleaseGivesTheCardBack(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("releasable")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "bob"})
	if response := h.do(&Request{Verb: Release, Card: ref, Actor: "alka"}); response.Refusal != contract.NotHolder {
		t.Fatalf("wanted not-holder, got %s %s", response.Outcome, response.Refusal)
	}
	h.mustDo(&Request{Verb: Release, Card: ref, Actor: "bob"})
	card := h.card(ref)
	if card.Substate != contract.SubstateReady || card.Holder != "" {
		t.Errorf("after release: wanted ready with no holder, got %s %q", card.Substate, card.Holder)
	}
}

// TestBlockRaisesAnObstacle asserts CORE-BLOCK-1, CORE-BLOCK-3, CORE-BLOCK-4
// and CORE-BLOCK-5: the reason a blocked card carries, the effect, the
// optional kind, and the reason not being restricted to any closed set. The
// removed holder is the other half of CORE-CARD-7.
func TestBlockRaisesAnObstacle(t *testing.T) {
	h := newHarness(t)
	for _, reason := range []string{"the printer is on fire", "waiting on the caterer"} {
		ref := h.ready("blockable")
		h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
		h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: reason, Kind: "external"})
		card := h.card(ref)
		if card.Substate != contract.SubstateBlocked {
			t.Errorf("substate: wanted blocked, got %s", card.Substate)
		}
		if card.Holder != "" {
			t.Errorf("CORE-BLOCK-3: wanted the holder removed, got %q", card.Holder)
		}
		if card.BlockReason != reason {
			t.Errorf("reason: wanted %q, got %q", reason, card.BlockReason)
		}
		if card.BlockKind != "external" {
			t.Errorf("kind: wanted external, got %q", card.BlockKind)
		}
	}
}

// TestOnlyUnblockLeavesBlocked asserts CORE-UNBLOCK-1 to CORE-UNBLOCK-4: the
// operator lifts a block, nobody else does, an unblock of a card that is not
// blocked is refused, and no other verb moves a card out of blocked. The last
// of those also drives CORE-CLAIM-6, since none of the verbs it tries changes
// the card's holder.
func TestOnlyUnblockLeavesBlocked(t *testing.T) {
	h := newHarness(t)
	ref := h.add("blocked")
	h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "stopped"})
	others := []*Request{
		{Verb: Claim, Card: ref, Actor: "alka"},
		{Verb: Move, Card: ref, Actor: "alka", State: doing},
		{Verb: Release, Card: ref, Actor: "alka"},
		{Verb: Block, Card: ref, Actor: "alka", Reason: "still stopped"},
	}
	for _, req := range others {
		h.do(req)
		if card := h.card(ref); card.Substate != contract.SubstateBlocked {
			t.Fatalf("CORE-UNBLOCK-3: %s left the card at %s", req.Verb, card.Substate)
		}
	}
	h.mustDo(&Request{Verb: Unblock, Card: ref, Actor: "alka"})
	if card := h.card(ref); card.Substate != contract.SubstateReady {
		t.Errorf("after unblock: wanted ready, got %s", card.Substate)
	}
	if response := h.do(&Request{Verb: Unblock, Card: ref, Actor: "alka"}); response.Refusal != contract.NotBlocked {
		t.Errorf("wanted not-blocked, got %s %s", response.Outcome, response.Refusal)
	}
}

// TestInstructionsAreServedAsThreeLayers asserts CORE-INSTR-3, CORE-INSTR-4,
// CORE-INSTR-5, CORE-INSTR-6 and CORE-INSTR-7
// and the format's user-global layer, with CORE-INSTR-1 and CORE-INSTR-2
// exercised by the fixture carrying both kinds of prose: three separate
// layers in the order global, standing, state, with no layer written into another, the legal
// moves alongside, and an absent global file as an absent layer.
func TestInstructionsAreServedAsThreeLayers(t *testing.T) {
	h := newHarness(t)
	// The card is stood at the review station before the claim, because no
	// owner takes work up at an intake state and a claim there is refused.
	// Review rather than doing, so that the move below still has room to
	// land the card in doing, which holds one card at a time.
	ref := h.add("served")
	h.at(ref, review)

	served := h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	if served.Instructions == nil {
		t.Fatal("claim served no instructions")
	}
	if served.Instructions.Global != "" {
		t.Errorf("with no global file the layer should be absent, got %q", served.Instructions.Global)
	}
	if !strings.Contains(served.Instructions.Standing, "standing text") {
		t.Errorf("standing layer: got %q", served.Instructions.Standing)
	}
	if !strings.Contains(served.Instructions.State, "Review instructions") {
		t.Errorf("state layer: got %q", served.Instructions.State)
	}
	if len(served.LegalMoves) == 0 {
		t.Fatal("CORE-INSTR-7: wanted the legal moves alongside the instructions")
	}
	// CORE-STATE-7: the moves reported include one to the next state in the
	// declared list, which is the departure a flow exists to offer.
	next := false
	for _, move := range served.LegalMoves {
		if move.State == aftercare && move.Direction == Forward {
			next = true
		}
	}
	if !next {
		t.Errorf("CORE-STATE-7: the legal moves omit the next state: %+v", served.LegalMoves)
	}

	global := filepath.Join(h.home, bench.UserBaseName, bench.InstructionsName)
	if err := os.WriteFile(global, []byte("The global text.\n"), 0o644); err != nil {
		t.Fatalf("global layer: %v", err)
	}
	moved := h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: doing})
	if !strings.Contains(moved.Instructions.Global, "global text") {
		t.Errorf("an edit to the global layer did not reach the serve: %q", moved.Instructions.Global)
	}
	if !strings.Contains(moved.Instructions.State, "Doing instructions") {
		t.Errorf("CORE-INSTR-4: wanted the entered state's instructions, got %q", moved.Instructions.State)
	}

	// No layer carries another's text, on the serve or on disk.
	if strings.Contains(moved.Instructions.State, "standing text") {
		t.Error("CORE-INSTR-6: the state layer carries the standing text")
	}
	stored, err := bench.ReadText(filepath.Join(h.root, bench.StatesDir, doing, bench.StateAnchor))
	if err != nil {
		t.Fatalf("state anchor: %v", err)
	}
	if strings.Contains(stored, "standing text") || strings.Contains(stored, "global text") {
		t.Error("CORE-INSTR-6: a stored state anchor carries another layer's text")
	}
}

// TestBasisReportsStale asserts CORE-BASIS-1, CORE-BASIS-2, CORE-BASIS-4 and
// CORE-BASIS-5:
// a request whose basis names a superseded revision reports stale carrying
// the current revision, and it does so even when a precondition other than
// the card's existence is also unsatisfied.
func TestBasisReportsStale(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("mutated")
	before := h.card(ref).Revision
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "bob"})
	current := h.card(ref).Revision

	response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka", Basis: before})
	if response.Outcome != contract.OutcomeStale {
		t.Fatalf("wanted stale, got %s %s", response.Outcome, response.Refusal)
	}
	if contract.ExitCode(response.Outcome) != 3 {
		t.Errorf("exit code: wanted 3, got %d", contract.ExitCode(response.Outcome))
	}
	if response.Card.Revision != current {
		t.Errorf("CORE-BASIS-4: wanted the current revision %s, got %s", current, response.Card.Revision)
	}
	if response.Refusal != "" {
		t.Errorf("CORE-BASIS-5: stale should win over the held precondition, got refusal %s", response.Refusal)
	}
	if h.card(ref).FM.Has("basis") {
		t.Error("CORE-BASIS-3: the card carries a basis")
	}
}

// TestQueueOrderIsArrivalThenOrdinal asserts CORE-QUEUE-3 over a fixture
// whose arrival order and creation order disagree.
func TestQueueOrderIsArrivalThenOrdinal(t *testing.T) {
	h := newHarness(t)
	first := h.add("earliest")
	h.advance(time.Hour)
	second := h.add("later")
	h.advance(time.Hour)
	third := h.add("latest")

	// Move the middle card away and back, so its arrival is the most recent
	// while its identifier order is unchanged.
	h.mustDo(&Request{Verb: Move, Card: second, Actor: "alka", State: doing})
	h.advance(time.Hour)
	h.mustDo(&Request{Verb: Move, Card: second, Actor: "alka", State: intake})

	listing, err := h.library.List(&Request{Verb: "ls", State: intake})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	wanted := []string{first, third, second}
	if len(listing.Cards) != len(wanted) {
		t.Fatalf("wanted %d cards, got %d", len(wanted), len(listing.Cards))
	}
	for i, ref := range wanted {
		if listing.Cards[i].Ref != ref {
			t.Errorf("position %d: wanted %s, got %s", i, ref, listing.Cards[i].Ref)
		}
	}

	// The ready filter narrows without disturbing the order. The card is
	// blocked rather than claimed, because an intake state takes no work up
	// and refuses a claim, where a block says something about the card and
	// stays available wherever the card stands.
	h.mustDo(&Request{Verb: Block, Card: first, Actor: "alka", Reason: "stopped"})
	ready, err := h.library.List(&Request{Verb: "ls", State: intake, ReadyOnly: true})
	if err != nil {
		t.Fatalf("list ready: %v", err)
	}
	if len(ready.Cards) != 2 || ready.Cards[0].Ref != third {
		t.Errorf("ready listing: got %d cards leading with %s", len(ready.Cards), ready.Cards[0].Ref)
	}
}

// TestTiesBreakByCreationOrdinal asserts the tie-break half of CORE-QUEUE-3
// over two cards that entered a state at the same instant.
//
// The numbers are rewritten so that the creation order runs against the
// identifier order, because the retired CORE-QUEUE-1 broke the same tie by
// ascending identifier and a fixture where the two orders agree passes under
// either rule.
func TestTiesBreakByCreationOrdinal(t *testing.T) {
	h := newHarness(t)
	h.add("one")
	h.add("two")

	cards, err := h.library.Bench.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("wanted 2 cards, got %d", len(cards))
	}
	// Cards reads in ascending identifier order, so numbering downwards from
	// the count puts the lower creation ordinal on the higher identifier.
	for i, card := range cards {
		h.renumber(card.ID, len(cards)-i)
	}
	h.reopen()

	listing, err := h.library.List(&Request{Verb: "ls", State: intake})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listing.Cards) != 2 {
		t.Fatalf("wanted 2 cards, got %d", len(listing.Cards))
	}
	first, second := h.card(listing.Cards[0].Ref), h.card(listing.Cards[1].Ref)
	if first.Number > second.Number {
		t.Errorf("wanted the lower creation ordinal first, got %d then %d", first.Number, second.Number)
	}
	if first.ID < second.ID {
		t.Errorf("the tie followed the identifier rather than the ordinal, got %s then %s", first.ID, second.ID)
	}
}

// TestHistoryNeverResolvesAgainstTheBench asserts CORE-HIST-1 and CORE-HIST-4
// to CORE-HIST-6:
// the recorded acts keep their order and carry the titles as they stood, so a
// state renamed after a move still reads under its old title.
func TestHistoryNeverResolvesAgainstTheBench(t *testing.T) {
	h := newHarness(t)
	// The setup move stands the card where an owner takes work up, since a
	// claim at an intake state is refused, and it is the first of the two
	// moves the history below carries.
	ref := h.add("historic")
	h.at(ref, doing)
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: review})

	anchor := filepath.Join(h.root, bench.StatesDir, doing, bench.StateAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("state anchor: %v", err)
	}
	if err := os.WriteFile(anchor, []byte(strings.Replace(text, "title: Doing", "title: Renamed", 1)), 0o644); err != nil {
		t.Fatalf("rename: %v", err)
	}
	h.reopen()

	events, err := h.library.History(&Request{Verb: "log", Card: ref})
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	wanted := []string{contract.EventCreated, contract.EventMoved, contract.EventClaimed, contract.EventMoved}
	if len(events) != len(wanted) {
		t.Fatalf("wanted %d acts, got %d", len(wanted), len(events))
	}
	for i, name := range wanted {
		if events[i].Event != name {
			t.Errorf("CORE-HIST-5: position %d wanted %s, got %s", i, name, events[i].Event)
		}
	}
	move := events[1]
	if move.ToTitle != "Doing" {
		t.Errorf("CORE-HIST-6: wanted the title as of the move, got %q", move.ToTitle)
	}
	if move.From != intake || move.To != doing || move.FromTitle != "Intake" {
		t.Errorf("CORE-HIST-4: the move carries %s/%q to %s/%q", move.From, move.FromTitle, move.To, move.ToTitle)
	}
}

// TestNextOffersWithoutTaking asserts the pull discipline: next reports the
// card CORE-QUEUE-3 names and changes nothing about it.
func TestNextOffersWithoutTaking(t *testing.T) {
	h := newHarness(t)
	ref := h.add("offered")
	before := h.card(ref)
	offers, err := h.library.Next(&Request{Verb: "next", State: intake})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if len(offers) != 1 || offers[0].Card == nil || offers[0].Card.Ref != ref {
		t.Fatalf("wanted the intake station to offer %s, got %+v", ref, offers)
	}
	after := h.card(ref)
	if after.Substate != before.Substate || after.Holder != before.Holder || after.Revision != before.Revision {
		t.Error("next changed the card it offered")
	}

	// With no argument it reports every state in flow order.
	all, err := h.library.Next(&Request{Verb: "next"})
	if err != nil {
		t.Fatalf("next over the workbench: %v", err)
	}
	if len(all) != len(h.library.Bench.States) {
		t.Fatalf("wanted one offer per state, got %d", len(all))
	}
	for i, offer := range all {
		if offer.State != h.library.Bench.States[i].ID {
			t.Errorf("position %d: wanted %s, got %s", i, h.library.Bench.States[i].ID, offer.State)
		}
	}
}

// TestTheCardLockCoversTheWholeTransaction asserts that a mutation decides and
// writes on the same side of the card's lock.
//
// The Interleave hook drives a second library, over the same bench and so
// standing for a second process, into the middle of the first one's
// transaction: after the card has been read under its lock and before any
// precondition is evaluated. That is the window a lock taken only around the
// write would leave open, and a second claim landing in it would read the card
// as ready, be admitted, and then be overwritten by the first write, changing
// the holder by a path CORE-CLAIM-6 does not admit.
func TestTheCardLockCoversTheWholeTransaction(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("contested")

	second, err := bench.Open(h.root)
	if err != nil {
		t.Fatalf("open a second view of the workbench: %v", err)
	}
	other := New(second, h.home)
	other.Now = h.library.Now

	var intruder *Response
	h.library.Interleave = func() {
		intruder = other.Do(&Request{Verb: Claim, Card: ref, Actor: "bob"})
	}
	first := h.library.Do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	h.library.Interleave = nil
	h.reopen()

	if first.Outcome != contract.OutcomeOK {
		t.Fatalf("the first claim: wanted ok, got %s %s", first.Outcome, first.Refusal)
	}
	if intruder == nil {
		t.Fatal("the interleaved claim never ran, so this test proves nothing")
	}
	if intruder.Outcome != contract.OutcomeRefused || intruder.Refusal != contract.Locked {
		t.Errorf("the interleaved claim: wanted %s, got %s %s", contract.Locked, intruder.Outcome, intruder.Refusal)
	}
	card := h.card(ref)
	if card.Holder != "alka" {
		t.Errorf("holder: wanted alka, got %q", card.Holder)
	}
	claims := 0
	for _, ev := range h.events(ref) {
		if ev.Event == contract.EventClaimed {
			claims++
		}
	}
	if claims != 1 {
		t.Errorf("wanted one claim recorded, got %d", claims)
	}
}

// TestALapseNoticedByAReadTakesTheLock asserts that a read noticing an expired
// claim writes under the card's lock like any other mutation, and that a read
// finding the lock held leaves the card alone rather than failing, since the
// process holding it lapses the claim itself.
func TestALapseNoticedByAReadTakesTheLock(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("leased")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "bob", Expires: time.Hour})
	h.advance(2 * time.Hour)
	h.reopen()

	// With the card's lock held by somebody else, the read reports what it
	// found and writes nothing.
	held, err := bench.Acquire(h.card(ref).Dir, "someone", bench.Stamp(h.clock))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if _, err := h.library.List(&Request{Verb: "ls"}); err != nil {
		t.Fatalf("a read against a locked card should not fail: %v", err)
	}
	if card := h.card(ref); card.Substate != contract.SubstateActive {
		t.Errorf("the read wrote through another process's lock, leaving %s", card.Substate)
	}
	held.Release()

	// With the lock free, the same read applies the lapse and journals it.
	if _, err := h.library.List(&Request{Verb: "ls"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	card := h.card(ref)
	if card.Substate != contract.SubstateReady || card.Holder != "" {
		t.Errorf("after the lapse: wanted ready with no holder, got %s %q", card.Substate, card.Holder)
	}
	lapses := 0
	for _, ev := range h.events(ref) {
		if ev.Event == contract.EventExpired {
			lapses++
		}
	}
	if lapses != 1 {
		t.Errorf("wanted one expiry recorded, got %d", lapses)
	}
}

// TestAClaimDrivenIntoTheStepFiveWindowIsRefused asserts the race the sibling
// exists for. A claim driven into the window between the release of the card's
// own lock and the rename of its directory is refused naming the archiving
// actor, the card's journal gains no claimed line, and the archive lands.
//
// Without the sibling the claim would be admitted, its write would recreate
// cards/<id>/ underneath the rename through MkdirAll, and the identifier would
// then exist in both halves of the collection, which the format forbids
// outright.
//
// Arming: removing the sibling check from Acquire admits the claim, the card
// anchor is written back into a directory that has just moved, and the card is
// found in both halves.
func TestAClaimDrivenIntoTheStepFiveWindowIsRefused(t *testing.T) {
	h := newHarness(t)
	ref := h.add("archived under a claim")
	id := h.card(ref).ID
	other := h.second()

	var intruder *Response
	h.inWindow(func() {
		intruder = other.Do(&Request{Verb: Claim, Card: ref, Actor: "bob"})
	})
	archived := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
	h.library.Bench.Hooks = nil
	h.reopen()

	if archived.Outcome != contract.OutcomeOK {
		t.Fatalf("the archive: wanted ok, got %s %s", archived.Outcome, archived.Refusal)
	}
	if intruder == nil {
		t.Fatal("the interleaved claim never ran, so this test proves nothing")
	}
	if intruder.Refusal != contract.Locked {
		t.Fatalf("the interleaved claim: wanted %s, got %s %s", contract.Locked, intruder.Outcome, intruder.Refusal)
	}
	if intruder.Detail != "alka" {
		t.Errorf("the refusal should name the archiving actor, got %q", intruder.Detail)
	}
	if bench.Exists(filepath.Join(h.library.Bench.CardsRoot(), id)) {
		t.Error("the identifier is in both halves of the collection")
	}
	target := filepath.Join(h.library.Bench.ArchivedCardsRoot(), id)
	if !bench.Exists(filepath.Join(target, bench.CardAnchor)) {
		t.Fatalf("the archive did not land, wanted the card at %s", target)
	}
	events, _, err := bench.ReadJournal(filepath.Join(target, bench.JournalName))
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	for _, ev := range events {
		if ev.Event == contract.EventClaimed {
			t.Error("the refused claim reached the journal")
		}
	}
}

// TestALapseNoticedInsideTheWindowWritesNothing asserts that the fourth
// lock-acquiring writer, a read that notices an expired claim, is stopped by
// the sibling like every other. It acquires the card's lock from a pure read
// path and then writes, so a lapse noticed inside the window would rewrite
// card.md into a directory the rename is moving and resurrect it.
//
// The read reports what it found rather than failing, which is the existing
// skip-on-failure behaviour and is correct unchanged: a read has no business
// refusing because a write is in flight.
//
// Arming: exempting the read path from the sibling check lets its Save
// recreate the directory mid-rename, and the identifier ends up in both halves.
func TestALapseNoticedInsideTheWindowWritesNothing(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("leased")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "bob", Expires: time.Hour})
	h.advance(2 * time.Hour)
	h.reopen()
	card := h.card(ref)
	id := card.ID
	anchor, err := bench.ReadText(card.AnchorPath())
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	other := h.second()

	var readErr error
	h.inWindow(func() {
		_, readErr = other.List(&Request{Verb: "ls"})
	})
	archived := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
	h.library.Bench.Hooks = nil
	h.reopen()

	if archived.Outcome != contract.OutcomeOK {
		t.Fatalf("the archive: wanted ok, got %s %s", archived.Outcome, archived.Refusal)
	}
	if readErr != nil {
		t.Errorf("the read should report what it found rather than failing, got %v", readErr)
	}
	if bench.Exists(filepath.Join(h.library.Bench.CardsRoot(), id)) {
		t.Error("the read resurrected the card directory")
	}
	target := filepath.Join(h.library.Bench.ArchivedCardsRoot(), id)
	after, err := bench.ReadText(filepath.Join(target, bench.CardAnchor))
	if err != nil {
		t.Fatalf("read the archived anchor: %v", err)
	}
	if after != anchor {
		t.Error("the read rewrote card.md inside the window")
	}
	events, _, err := bench.ReadJournal(filepath.Join(target, bench.JournalName))
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	for _, ev := range events {
		if ev.Event == contract.EventExpired {
			t.Error("the read appended an expired event inside the window")
		}
	}
}

// TestAMoveIntoARetiringStateCannotLand asserts the state closure from both
// sides of the interleaving. A mover that reached its card lock first is one
// the retiring act's own scan cannot miss, and a mover arriving after the
// sibling exists reads it and stops itself. Either way no card is left
// pointing at a station that resolves only in the archive.
//
// Arming: moving the scan back before the sibling is created lets the first
// order through, and removing the destination check from move lets the second
// through; each lands a card in a state that resolves only under
// archive/states/.
func TestAMoveIntoARetiringStateCannotLand(t *testing.T) {
	t.Run("the mover got there first", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("mover")
		card := h.card(ref)
		other := h.second()
		var blocked *Response
		h.library.Interleave = func() {
			blocked = other.Archive(&Request{Verb: "archive", Actor: "bob", Ref: aftercare})
		}
		moved := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", State: aftercare})
		h.library.Interleave = nil
		h.reopen()

		if moved.Outcome != contract.OutcomeOK {
			t.Fatalf("the move: wanted ok, got %s %s", moved.Outcome, moved.Refusal)
		}
		if blocked == nil {
			t.Fatal("the interleaved archive never ran, so this test proves nothing")
		}
		if blocked.Refusal != contract.Locked {
			t.Fatalf("the interleaved archive: wanted %s, got %s %s", contract.Locked, blocked.Outcome, blocked.Refusal)
		}
		if blocked.Detail != card.ID {
			t.Errorf("the refusal should name the card whose lock is held, got %q", blocked.Detail)
		}
		if !bench.Exists(filepath.Join(h.root, bench.StatesDir, aftercare, bench.StateAnchor)) {
			t.Error("the refused archive retired the state anyway")
		}
	})

	t.Run("the retirement got there first", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("mover")
		id := h.card(ref).ID
		other := h.second()
		var blocked *Response
		h.library.Bench.Hooks = &bench.Hooks{
			AfterStep: func(n int) error {
				if n == 2 {
					blocked = other.Do(&Request{Verb: Move, Card: ref, Actor: "alka", State: aftercare})
				}
				return nil
			},
		}
		// A retired state leaves the bench unopenable until its identifier
		// comes out of the states list, which is a gap of its own, so the
		// card is read from disk here rather than through a reopen.
		archived := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: aftercare})
		h.library.Bench.Hooks = nil

		if archived.Outcome != contract.OutcomeOK {
			t.Fatalf("the archive: wanted ok, got %s %s", archived.Outcome, archived.Refusal)
		}
		if blocked == nil {
			t.Fatal("the interleaved move never ran, so this test proves nothing")
		}
		if blocked.Refusal != contract.Locked {
			t.Fatalf("the interleaved move: wanted %s, got %s %s", contract.Locked, blocked.Outcome, blocked.Refusal)
		}
		if blocked.Detail != "alka" {
			t.Errorf("the refusal should name the retiring actor, got %q", blocked.Detail)
		}
		card, err := bench.LoadCard(h.library.Bench.CardsRoot(), id)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if card.State != intake {
			t.Errorf("the refused move stored a state anyway, card is in %s", card.State)
		}
	})
}

// TestACreationIntoARetiringStateCannotLand asserts the closure's other half,
// which needs a row of its own because a creation takes no lock at all and is
// therefore observed through a single fact.
//
// The refused creation gives up the directory mkdir claimed as well as the
// identifier, because an empty hex directory makes Cards return an error and
// every listing on the bench fail with it.
func TestACreationIntoARetiringStateCannotLand(t *testing.T) {
	t.Run("the creation is between mkdir and its anchor", func(t *testing.T) {
		h := newHarness(t)
		claimed := filepath.Join(h.library.Bench.CardsRoot(), "cccccccccccc")
		if err := os.MkdirAll(claimed, 0o755); err != nil {
			t.Fatalf("claim an identifier: %v", err)
		}
		response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: aftercare})
		if response.Refusal != contract.Locked {
			t.Fatalf("wanted %s, got %s %s", contract.Locked, response.Outcome, response.Refusal)
		}
		if response.Detail != "cccccccccccc" {
			t.Errorf("the refusal should name the card that will not load, got %q", response.Detail)
		}
	})

	t.Run("the retirement got there first", func(t *testing.T) {
		h := newHarness(t)
		h.add("an occupant of the intake station")
		before := strings.Join(bench.ListIDs(h.library.Bench.CardsRoot()), ",")
		other := h.second()
		var blocked *Response
		h.library.Bench.Hooks = &bench.Hooks{
			AfterStep: func(n int) error {
				if n == 2 {
					blocked = other.Add(&Request{Verb: "add", Actor: "alka", Title: "latecomer", State: aftercare})
				}
				return nil
			},
		}
		archived := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: aftercare})
		h.library.Bench.Hooks = nil

		if archived.Outcome != contract.OutcomeOK {
			t.Fatalf("the archive: wanted ok, got %s %s", archived.Outcome, archived.Refusal)
		}
		if blocked == nil {
			t.Fatal("the interleaved creation never ran, so this test proves nothing")
		}
		if blocked.Refusal != contract.Locked {
			t.Fatalf("the interleaved creation: wanted %s, got %s %s", contract.Locked, blocked.Outcome, blocked.Refusal)
		}
		if after := strings.Join(bench.ListIDs(h.library.Bench.CardsRoot()), ","); after != before {
			t.Errorf("the refused creation left a directory behind: %q, wanted %q", after, before)
		}
	})
}

// TestTheStateScanRefusesOnAllThreeConditionsInOrder asserts what the scan
// refuses on, that it says which condition fired, that it runs only once the
// retiring act's own sibling exists, and that per card it stats the lock
// before it reads the anchor.
//
// The last of those is the whole of the closure's second branch. Read the
// other way round the scan learns nothing from either observation about a move
// that landed between them, and both of the other armings pass a scan with the
// per-card order reversed, so it gets a case of its own.
func TestTheStateScanRefusesOnAllThreeConditionsInOrder(t *testing.T) {
	t.Run("a card sits in the state", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("occupant")
		h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: aftercare})
		response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: aftercare})
		if response.Refusal != contract.Occupied || response.Detail != aftercareSlug {
			t.Fatalf("wanted %s naming the state by its slug %q, got %s %q", contract.Occupied, aftercareSlug, response.Refusal, response.Detail)
		}
	})

	t.Run("a card elsewhere holds its own lock", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("busy")
		card := h.card(ref)
		held := h.hold(card.Dir, "someone")
		defer held.Release()
		sibling := bench.SiblingPath(filepath.Join(h.root, bench.StatesDir, aftercare))
		stood := false
		h.library.Bench.Hooks = &bench.Hooks{
			BeforeAnchorRead: func(string) { stood = bench.Exists(sibling) },
		}
		response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: aftercare})
		h.library.Bench.Hooks = nil
		if response.Refusal != contract.Locked || response.Detail != card.ID {
			t.Fatalf("wanted %s naming the card, got %s %q", contract.Locked, response.Refusal, response.Detail)
		}
		if !stood {
			t.Error("the scan ran before the sibling was created, so a writer could slip past it")
		}
	})

	t.Run("a card directory will not load", func(t *testing.T) {
		h := newHarness(t)
		claimed := filepath.Join(h.library.Bench.CardsRoot(), "dddddddddddd")
		if err := os.MkdirAll(claimed, 0o755); err != nil {
			t.Fatalf("claim an identifier: %v", err)
		}
		response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: aftercare})
		if response.Refusal != contract.Locked || response.Detail != "dddddddddddd" {
			t.Fatalf("wanted %s naming the card, got %s %q", contract.Locked, response.Refusal, response.Detail)
		}
	})

	t.Run("a move commits between the stat and the anchor read", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("mover")
		card := h.card(ref)
		committed := false
		h.library.Bench.Hooks = &bench.Hooks{
			BeforeAnchorRead: func(id string) {
				if id != card.ID || committed {
					return
				}
				committed = true
				// The mover's whole critical section, landing in the gap:
				// it holds no lock by the time the stat has run, and its
				// write is on disk before the anchor is read.
				fresh, err := bench.LoadCard(h.library.Bench.CardsRoot(), id)
				if err != nil {
					t.Errorf("load: %v", err)
					return
				}
				fresh.State = aftercare
				if err := fresh.Save(); err != nil {
					t.Errorf("save: %v", err)
				}
			},
		}
		response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: aftercare})
		h.library.Bench.Hooks = nil
		if !committed {
			t.Fatal("the move never landed in the gap, so this test proves nothing")
		}
		if response.Refusal != contract.Occupied || response.Detail != aftercareSlug {
			t.Fatalf("wanted %s naming the state by its slug %q, got %s %q", contract.Occupied, aftercareSlug, response.Refusal, response.Detail)
		}
	})
}

// TestTheSafeInterleavingsAreCleanRefusals asserts the two orders that end in
// a refusal nobody should read as a defect. A writer and a structural act that
// each got to one lock first stop each other, and both are cleanly retryable
// once the other stands down. A structural act whose entity vanished under it
// reports the unknown entity it has become rather than surfacing whatever the
// filesystem raised, and takes its own sibling off on the way out.
func TestTheSafeInterleavingsAreCleanRefusals(t *testing.T) {
	t.Run("each party stops the other", func(t *testing.T) {
		h := newHarness(t)
		ref := h.ready("contested")
		card := h.card(ref)

		// The writer got to the card's lock before the sibling existed, so
		// the act is refused at its third acquisition.
		held := h.hold(card.Dir, "bob")
		refused := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
		if refused.Refusal != contract.Locked || refused.Detail != "bob" {
			t.Fatalf("the act: wanted %s naming bob, got %s %q", contract.Locked, refused.Refusal, refused.Detail)
		}
		held.Release()

		// The same configuration read from the other side: the writer's own
		// look for a sibling lands after one was created, so it refuses too.
		record := bench.LockRecord{Actor: "alka", PID: 4242, TS: bench.Stamp(h.clock), Op: bench.OpArchive}
		h.plant(bench.SiblingPath(card.Dir), record)
		claimed := h.do(&Request{Verb: Claim, Card: ref, Actor: "bob"})
		if claimed.Refusal != contract.Locked || claimed.Detail != "alka" {
			t.Fatalf("the claim: wanted %s naming alka, got %s %q", contract.Locked, claimed.Refusal, claimed.Detail)
		}
		for _, ev := range h.events(ref) {
			if ev.Event == contract.EventClaimed || ev.Event == contract.EventArchived {
				t.Error("a refused party wrote something")
			}
		}

		// Both retry cleanly once the other has stood down.
		if err := os.Remove(bench.SiblingPath(card.Dir)); err != nil {
			t.Fatalf("stand down: %v", err)
		}
		h.reopen()
		if response := h.do(&Request{Verb: Claim, Card: ref, Actor: "bob"}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("the claim's retry: %s %s", response.Outcome, response.Refusal)
		}
		if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("the act's retry: %s %s", response.Outcome, response.Refusal)
		}
	})

	t.Run("the entity vanished under the loser", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("doomed")
		card := h.card(ref)
		h.library.Bench.Hooks = &bench.Hooks{
			AfterStep: func(n int) error {
				if n == 2 {
					// The winner finished while the loser held nothing but
					// a reference it resolved before the race began.
					return os.RemoveAll(card.Dir)
				}
				return nil
			},
		}
		response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: ref, Confirm: true})
		h.library.Bench.Hooks = nil
		if response.Refusal != contract.UnknownCard {
			t.Fatalf("wanted %s, got %s %s", contract.UnknownCard, response.Outcome, response.Refusal)
		}
		if response.Detail != card.ID {
			t.Errorf("the refusal should name the entity, got %q", response.Detail)
		}
		if got := strings.Join(h.locks(), ", "); got != "" {
			t.Errorf("the loser left %q behind", got)
		}
	})
}
