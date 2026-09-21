package verb

import (
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// lastEvent returns the most recent journal line the card's own journal
// carries, which is what the equivalence tests below compare between a
// direct verb and Settle landing an item at the same state.
func lastEvent(t *testing.T, h *harness, card string) bench.Event {
	t.Helper()
	resolved, err := h.library.Bench.ResolveCard(card)
	if err != nil {
		t.Fatalf("resolve %s: %v", card, err)
	}
	events, _, err := bench.ReadJournal(resolved.Card.JournalPath())
	if err != nil {
		t.Fatalf("journal %s: %v", card, err)
	}
	if len(events) == 0 {
		t.Fatalf("%s carries no journal event", card)
	}
	return events[len(events)-1]
}

// TestSettleIsEquivalentToResolve asserts settle/criteria/6's resolved half:
// settle with state=resolved on a decision produces the identical written
// item state and journal event resolve itself would, and settle refuses the
// same operator-owned-item guard resolve does for the same broken
// precondition.
func TestSettleIsEquivalentToResolve(t *testing.T) {
	direct := newHarness(t)
	card := direct.add("a card carrying a decision")
	ref := direct.file(card, "decision", "which library wins")
	if response := direct.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: ref, Text: "the incumbent, per the thread"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", response.Outcome, response.Refusal)
	}
	direct.reopen()
	directFM, _ := direct.itemAnchor(ref)
	directState := directFM.Value(bench.ItemStateField)
	directEvent := lastEvent(t, direct, card)

	settled := newHarness(t)
	sCard := settled.add("a card carrying a decision")
	sRef := settled.file(sCard, "decision", "which library wins")
	if response := settled.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: sRef, State: bench.ItemResolved, Text: "the incumbent, per the thread"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("settle: %s %s", response.Outcome, response.Refusal)
	}
	settled.reopen()
	settledFM, _ := settled.itemAnchor(sRef)
	settledState := settledFM.Value(bench.ItemStateField)
	settledEvent := lastEvent(t, settled, sCard)

	if settledState != directState {
		t.Errorf("settle landed the item at %q, resolve landed it at %q", settledState, directState)
	}
	if settledEvent.Event != directEvent.Event {
		t.Errorf("settle wrote the journal event %q, resolve wrote %q", settledEvent.Event, directEvent.Event)
	}

	// Arm: an operator-owned open question settled by a non-operator refuses
	// contract.NotOperator, the precondition unique to closeItem, the same
	// way a direct resolve does.
	owned := newHarness(t)
	ownedCard := owned.add("a card carrying the operator's own question")
	filed := owned.library.File(&Request{
		Verb: "file", Actor: "alka", Card: ownedCard, Kind: "open_question",
		Text: "does the deadline move?", Owner: bench.ItemOwnerOperator,
	})
	if filed.Outcome != contract.OutcomeOK {
		t.Fatalf("file: %s %s", filed.Outcome, filed.Refusal)
	}
	owned.reopen()
	ownedRef := ownedCard + "/questions/1"
	direct2 := owned.library.Resolve(&Request{Verb: "resolve", Actor: "bob", Ref: ownedRef, Text: "not mine to settle"})
	if direct2.Refusal != contract.NotOperator {
		t.Fatalf("a direct resolve by a non-operator: wanted %s, got %s %s", contract.NotOperator, direct2.Outcome, direct2.Refusal)
	}
	settle2 := owned.library.Settle(&Request{Verb: "settle", Actor: "bob", Ref: ownedRef, State: bench.ItemResolved, Text: "not mine to settle"})
	if settle2.Refusal != contract.NotOperator {
		t.Errorf("settle by a non-operator on an operator-owned item: wanted %s, got %s %s", contract.NotOperator, settle2.Outcome, settle2.Refusal)
	}
}

// TestSettleIsEquivalentToVerify asserts settle/criteria/6's verified half:
// settle with state=verified produces the identical written item state and
// journal event verify itself would, and settle refuses contract.NotPending
// for the same broken precondition verify does: an item that already closed.
func TestSettleIsEquivalentToVerify(t *testing.T) {
	direct := newHarness(t)
	card := direct.add("a card carrying a criterion")
	ref := direct.file(card, "acceptance_criterion", criterionText)
	if response := direct.library.Verify(&Request{Verb: "verify", Actor: "alka", Ref: ref, Text: "checked and it holds"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("verify: %s %s", response.Outcome, response.Refusal)
	}
	direct.reopen()
	directFM, _ := direct.itemAnchor(ref)
	directState := directFM.Value(bench.ItemStateField)
	directEvent := lastEvent(t, direct, card)

	settled := newHarness(t)
	sCard := settled.add("a card carrying a criterion")
	sRef := settled.file(sCard, "acceptance_criterion", criterionText)
	if response := settled.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: sRef, State: bench.ItemVerified, Text: "checked and it holds"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("settle: %s %s", response.Outcome, response.Refusal)
	}
	settled.reopen()
	settledFM, _ := settled.itemAnchor(sRef)
	settledState := settledFM.Value(bench.ItemStateField)
	settledEvent := lastEvent(t, settled, sCard)

	if settledState != directState {
		t.Errorf("settle landed the item at %q, verify landed it at %q", settledState, directState)
	}
	if settledEvent.Event != directEvent.Event {
		t.Errorf("settle wrote the journal event %q, verify wrote %q", settledEvent.Event, directEvent.Event)
	}

	// Arm: a second settle against the now-verified item refuses NotPending,
	// the same as a second direct verify.
	direct.reopen()
	direct2 := direct.library.Verify(&Request{Verb: "verify", Actor: "alka", Ref: ref, Text: "again"})
	if direct2.Refusal != contract.NotPending {
		t.Fatalf("a second direct verify: wanted %s, got %s %s", contract.NotPending, direct2.Outcome, direct2.Refusal)
	}
	settled.reopen()
	settle2 := settled.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: sRef, State: bench.ItemVerified, Text: "again"})
	if settle2.Refusal != contract.NotPending {
		t.Errorf("a second settle: wanted %s, got %s %s", contract.NotPending, settle2.Outcome, settle2.Refusal)
	}
}

// TestSettleIsEquivalentToFail asserts settle/criteria/6's failed half:
// settle with state=failed produces the identical written item state and
// journal event fail itself would, and settle refuses contract.Uncited for
// the same broken precondition fail does: an acceptance criterion left
// pending with no citation on a workbench declaring evidence.
func TestSettleIsEquivalentToFail(t *testing.T) {
	direct := newHarness(t)
	direct.declareEvidence(evidenceBlock)
	card := direct.add("a card carrying a criterion")
	ref := direct.file(card, "acceptance_criterion", criterionText)
	target := "internal/verb/query_test.go#TestQueryNamesSeverity"
	if response := direct.library.Cite(&Request{Verb: "cite", Actor: "alka", Ref: ref, Scheme: "test", CiteTarget: target, Observed: "pass:fail"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("cite: %s %s", response.Outcome, response.Refusal)
	}
	direct.reopen()
	if response := direct.library.Fail(&Request{Verb: "fail", Actor: "alka", Ref: ref, Text: "regressed"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("fail: %s %s", response.Outcome, response.Refusal)
	}
	direct.reopen()
	directFM, _ := direct.itemAnchor(ref)
	directState := directFM.Value(bench.ItemStateField)
	directEvent := lastEvent(t, direct, card)

	settled := newHarness(t)
	settled.declareEvidence(evidenceBlock)
	sCard := settled.add("a card carrying a criterion")
	sRef := settled.file(sCard, "acceptance_criterion", criterionText)
	if response := settled.library.Cite(&Request{Verb: "cite", Actor: "alka", Ref: sRef, Scheme: "test", CiteTarget: target, Observed: "pass:fail"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("cite: %s %s", response.Outcome, response.Refusal)
	}
	settled.reopen()
	if response := settled.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: sRef, State: bench.ItemFailed, Text: "regressed"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("settle: %s %s", response.Outcome, response.Refusal)
	}
	settled.reopen()
	settledFM, _ := settled.itemAnchor(sRef)
	settledState := settledFM.Value(bench.ItemStateField)
	settledEvent := lastEvent(t, settled, sCard)

	if settledState != directState {
		t.Errorf("settle landed the item at %q, fail landed it at %q", settledState, directState)
	}
	if settledEvent.Event != directEvent.Event {
		t.Errorf("settle wrote the journal event %q, fail wrote %q", settledEvent.Event, directEvent.Event)
	}

	// Arm: an uncited criterion on an evidence-declaring workbench refuses
	// contract.Uncited, the same for a direct fail and for settle.
	uncitedDirect := newHarness(t)
	uncitedDirect.declareEvidence(evidenceBlock)
	uc := uncitedDirect.add("a card carrying an uncited criterion")
	ucRef := uncitedDirect.file(uc, "acceptance_criterion", criterionText)
	direct2 := uncitedDirect.library.Fail(&Request{Verb: "fail", Actor: "alka", Ref: ucRef, Text: "regressed"})
	if direct2.Refusal != contract.Uncited {
		t.Fatalf("a direct fail with no citation: wanted %s, got %s %s", contract.Uncited, direct2.Outcome, direct2.Refusal)
	}
	uncitedSettle := newHarness(t)
	uncitedSettle.declareEvidence(evidenceBlock)
	us := uncitedSettle.add("a card carrying an uncited criterion")
	usRef := uncitedSettle.file(us, "acceptance_criterion", criterionText)
	settle2 := uncitedSettle.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: usRef, State: bench.ItemFailed, Text: "regressed"})
	if settle2.Refusal != contract.Uncited {
		t.Errorf("a settle with no citation: wanted %s, got %s %s", contract.Uncited, settle2.Outcome, settle2.Refusal)
	}
}

// TestSettleIsEquivalentToReopen asserts settle/criteria/7: settle with
// state=pending produces the identical written item state and journal event
// reopen itself would, including the case of an empty reason refusing
// contract.Malformed.
func TestSettleIsEquivalentToReopen(t *testing.T) {
	direct := newHarness(t)
	card := direct.add("a card carrying a decision")
	ref := direct.file(card, "decision", "which library wins")
	if response := direct.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: ref, Text: "the incumbent"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", response.Outcome, response.Refusal)
	}
	direct.reopen()
	if response := direct.library.Reopen(&Request{Verb: "reopen", Actor: "alka", Ref: ref, Reason: "the incumbent changed its license"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("reopen: %s %s", response.Outcome, response.Refusal)
	}
	direct.reopen()
	directFM, _ := direct.itemAnchor(ref)
	directState := directFM.Value(bench.ItemStateField)
	directEvent := lastEvent(t, direct, card)

	settled := newHarness(t)
	sCard := settled.add("a card carrying a decision")
	sRef := settled.file(sCard, "decision", "which library wins")
	if response := settled.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: sRef, Text: "the incumbent"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", response.Outcome, response.Refusal)
	}
	settled.reopen()
	if response := settled.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: sRef, State: bench.ItemPending, Reason: "the incumbent changed its license"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("settle: %s %s", response.Outcome, response.Refusal)
	}
	settled.reopen()
	settledFM, _ := settled.itemAnchor(sRef)
	settledState := settledFM.Value(bench.ItemStateField)
	settledEvent := lastEvent(t, settled, sCard)

	if settledState != directState {
		t.Errorf("settle landed the item at %q, reopen landed it at %q", settledState, directState)
	}
	if settledEvent.Event != directEvent.Event {
		t.Errorf("settle wrote the journal event %q, reopen wrote %q", settledEvent.Event, directEvent.Event)
	}

	// Arm: an empty reason refuses contract.Malformed naming "reason", the
	// same for a direct reopen and for settle with state=pending.
	pendingDirect := newHarness(t)
	pc := pendingDirect.add("a card carrying a decision")
	pRef := pendingDirect.file(pc, "decision", "which library wins")
	if response := pendingDirect.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: pRef, Text: "the incumbent"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", response.Outcome, response.Refusal)
	}
	pendingDirect.reopen()
	direct2 := pendingDirect.library.Reopen(&Request{Verb: "reopen", Actor: "alka", Ref: pRef, Reason: ""})
	if direct2.Refusal != contract.Malformed || direct2.Detail != "reason" {
		t.Fatalf("a direct reopen with an empty reason: wanted malformed on reason, got %s %s %q", direct2.Outcome, direct2.Refusal, direct2.Detail)
	}
	pendingSettle := newHarness(t)
	ps := pendingSettle.add("a card carrying a decision")
	psRef := pendingSettle.file(ps, "decision", "which library wins")
	if response := pendingSettle.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: psRef, Text: "the incumbent"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", response.Outcome, response.Refusal)
	}
	pendingSettle.reopen()
	settle2 := pendingSettle.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: psRef, State: bench.ItemPending, Reason: ""})
	if settle2.Refusal != contract.Malformed || settle2.Detail != "reason" {
		t.Errorf("a settle with an empty reason: wanted malformed on reason, got %s %s %q", settle2.Outcome, settle2.Refusal, settle2.Detail)
	}
}

// TestSettleRefusesAnUnknownState asserts settle/criteria/8: settle with
// state empty is refused contract.Malformed naming "state", and settle with
// a non-empty state outside the four legal ones is refused
// contract.UnknownItemState, both checked ahead of item resolution (no card
// or item need exist for either to fire) and ahead of the operator check
// (dinah-544's own claim that the state-selection check runs before
// anything else Settle does, including the guard every one of the four
// verbs it dispatches to opens with).
func TestSettleRefusesAnUnknownState(t *testing.T) {
	h := newHarness(t)

	empty := h.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: "fx-1/decisions/1", State: ""})
	if empty.Refusal != contract.Malformed || empty.Detail != "state" {
		t.Errorf("an empty state: wanted malformed on state, got %s %s %q", empty.Outcome, empty.Refusal, empty.Detail)
	}

	unknown := h.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: "fx-1/decisions/1", State: "verifiedx"})
	if unknown.Refusal != contract.UnknownItemState || unknown.Detail != "verifiedx" {
		t.Errorf("an unknown state: wanted unknown-item-state naming it, got %s %s %q", unknown.Outcome, unknown.Refusal, unknown.Detail)
	}

	// Neither call above named a card that exists, so a refusal reading
	// contract.UnknownCard or contract.UnknownPath instead of the two above
	// would mean the state check ran after resolving the item rather than
	// before it.
	if empty.Refusal == contract.UnknownCard || empty.Refusal == contract.UnknownPath {
		t.Errorf("the empty-state call resolved the item before checking the state: %s", empty.Refusal)
	}

	// The state check also runs ahead of the operator check: on a bench
	// designating no operator, the same two calls still refuse on the state
	// rather than on contract.NoOperator.
	h.library.Bench.FM.Delete("operator")
	h.library.Bench.Operator = ""
	orphanEmpty := h.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: "fx-1/decisions/1", State: ""})
	if orphanEmpty.Refusal != contract.Malformed || orphanEmpty.Detail != "state" {
		t.Errorf("an empty state on a bench with no operator: wanted malformed on state ahead of no-operator, got %s %s", orphanEmpty.Outcome, orphanEmpty.Refusal)
	}
	orphanUnknown := h.library.Settle(&Request{Verb: "settle", Actor: "alka", Ref: "fx-1/decisions/1", State: "verifiedx"})
	if orphanUnknown.Refusal != contract.UnknownItemState {
		t.Errorf("an unknown state on a bench with no operator: wanted unknown-item-state ahead of no-operator, got %s %s", orphanUnknown.Outcome, orphanUnknown.Refusal)
	}
}
