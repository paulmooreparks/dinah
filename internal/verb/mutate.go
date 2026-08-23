package verb

import (
	"strings"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Do runs one of the five contract verbs and returns the canonical response.
//
// The order of the checks is the profile's, and it is the whole of what
// CORE-OUT-6 makes observable: the two workbench-level checks first, then the
// card's existence, then the basis, then the verb's own list in the order
// section 6 declares it.
//
// The whole of that evaluation happens under the card's lock, which is what
// makes a mutation one transaction rather than a decision followed by a write.
// The reference is resolved first, because the lock lives inside the card's
// own directory and there is nothing to lock until the card is found; the
// card is then read again under the lock, so the revision the basis is
// compared against and the substate every precondition reads are the ones on
// disk at the moment of the write rather than a snapshot taken before it. Two
// processes reaching the same card therefore cannot both see it ready, since
// the second is refused the lock outright.
func (l *Library) Do(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return l.FromError(req, err)
	}
	lock, err := bench.Acquire(found.Card.Dir, req.Actor, bench.Stamp(l.Now()))
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	card, err := bench.LoadCard(l.Bench.CardsRoot(), found.Card.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	if err := l.lapse(card); err != nil {
		return l.FromError(req, err)
	}
	if l.Interleave != nil {
		l.Interleave()
	}
	if req.Basis != "" && req.Basis != card.Revision {
		response := &Response{
			Outcome:     contract.OutcomeStale,
			Verb:        req.Verb,
			Card:        l.view(card),
			Basis:       req.Basis,
			Affordances: l.affordances(card),
		}
		return response
	}
	response := l.evaluate(req, card)
	if found.StalePrefix != "" && response.Outcome == contract.OutcomeOK {
		response.Warning = "warn.stale-prefix"
		response.WarningDetail = found.StalePrefix
	}
	return response
}

// evaluate applies one verb's own precondition list and, where every check is
// satisfied, its effect.
//
// It dispatches more than the contract's five. join and leave write the card's
// membership list, which is a write to the card like any other, so they run
// inside the same transaction Do opens rather than opening a second one of
// their own.
func (l *Library) evaluate(req *Request, card *bench.Card) *Response {
	switch req.Verb {
	case Claim:
		return l.claim(req, card)
	case Move:
		return l.move(req, card)
	case Release:
		return l.release(req, card)
	case Block:
		return l.block(req, card)
	case Unblock:
		return l.unblock(req, card)
	case Join:
		return l.join(req, card)
	case Leave:
		return l.leave(req, card)
	}
	return l.refuse(req, card, contract.UnknownVerb, req.Verb)
}

// lapse applies an expired claim. Expiry is evaluated at the moment any verb
// or read touches the card, because a single-seat local tool runs no
// background process, and the lapse is journaled when it is noticed.
//
// It writes, so its caller holds the card's lock. A verb calls it inside its
// own transaction; a read calls it through lapseRead, which takes the lock
// itself.
func (l *Library) lapse(card *bench.Card) error {
	if !card.Lapsed(l.Now()) {
		return nil
	}
	holder := card.Holder
	ev := bench.Event{
		TS:      bench.Stamp(l.Now()),
		Event:   contract.EventExpired,
		Actor:   holder,
		Expires: card.Expires,
	}
	card.Substate = contract.SubstateReady
	card.Holder = ""
	card.ClaimSince = ""
	card.Expires = ""
	if err := card.Save(); err != nil {
		return err
	}
	return bench.AppendEvent(card.JournalPath(), ev)
}

// canClaim runs the precondition sequence CORE-CLAIM declares. It returns a
// refusal response when the card may not be claimed and nil when the card is
// ready to take. Pull reuses the same checks for the claimed event it bundles
// with its move, so both verbs run the same gates in the same order.
func (l *Library) canClaim(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	if req.Holder != "" && req.Holder != req.Actor {
		return l.refuse(req, card, contract.NotRequester, req.Holder)
	}
	return l.claimableSubstate(req, card)
}

// claimableSubstate runs the last two rows of the claim's list, blocked then
// held, against the substate the card carries. It stands apart from canClaim
// because pull evaluates these two rows at rows 9 and 10 of its own longer
// list, between the move's departure row and the move's destination rows, and
// a caller reaching them through canClaim would have to run the whole claim
// list at that point. Splitting the tail off changes neither list's order:
// canClaim still evaluates no-owner, not-requester, blocked, held.
//
// The held row here is the stricter of the tool's two: it refuses a card
// whose substate is active whoever holds it, where the move's row admits a
// card the owner asking already holds. A claim cannot take a card somebody is
// already working, its own asker included.
func (l *Library) claimableSubstate(req *Request, card *bench.Card) *Response {
	if card.Substate == contract.SubstateBlocked {
		return l.refuse(req, card, contract.Blocked, card.BlockReason)
	}
	if card.Substate == contract.SubstateActive {
		return l.refuse(req, card, contract.Held, card.Holder)
	}
	return nil
}

// claim takes up a waiting card. The list is CORE-CLAIM's: the card exists,
// the request names an owner, the owner named as holder is the owner asking,
// the card is not blocked, and the card is not already held.
func (l *Library) claim(req *Request, card *bench.Card) *Response {
	if refusal := l.canClaim(req, card); refusal != nil {
		return refusal
	}
	now := l.Now()
	card.Substate = contract.SubstateActive
	card.Holder = req.Actor
	card.ClaimSince = bench.Stamp(now)
	if req.Expires > 0 {
		card.Expires = bench.Stamp(now.Add(req.Expires))
	}
	ev := bench.Event{
		TS:      card.ClaimSince,
		Event:   contract.EventClaimed,
		Actor:   req.Actor,
		Expires: card.Expires,
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	response.Instructions = l.serve(card)
	response.LegalMoves = l.legalMoves(card)
	return response
}

// canMove runs the precondition sequence CORE-MOVE declares, in the order
// section 6.4 states it. It returns the destination and departure state when
// the move is permitted, so a caller can write the journal event without
// re-resolving either. The retiring check is intentionally the last of the
// destination checks, read under the card lock this transaction already
// holds. A move that reached the sibling first is one the retiring act's own
// scan cannot miss, and a move arriving later reads the sibling and stops
// itself.
//
// Pull reuses these gates, with two small differences: pull's own preflight
// already answers the NoOwner / NotOperator / Override checks before the
// transaction opens, so it calls into here knowing req.Actor is non-empty and
// the operator has been confirmed. The returned destination and departure
// are what pull writes into its moved event.
//
// The list is split across canRoute and canLand so that pull can run the
// claim's two substate rows between them, which is where pull's own table
// puts them. Reading the two halves in this order is reading CORE-MOVE's
// list in CORE-MOVE's order, and neither half is called anywhere in an order
// this function does not also produce.
func (l *Library) canMove(req *Request, card *bench.Card) (*bench.State, *bench.State, bool, *Response, error) {
	destination, departure, refusal := l.canRoute(req, card)
	if refusal != nil {
		return nil, nil, false, refusal, nil
	}
	override, refusal, err := l.canLand(req, card, destination, departure)
	if err != nil {
		return nil, nil, false, nil, err
	}
	if refusal != nil {
		return nil, nil, false, refusal, nil
	}
	return destination, departure, override, nil, nil
}

// canRoute runs the rows of CORE-MOVE's list that read the request and the
// two states, in that list's order: the request names an owner, the named
// destination is declared, an override marker is the operator's, and the
// departure is one the owner asking may move work out of. It returns the
// resolved destination and departure so its caller resolves neither twice.
//
// The departure can be nil, when the card stands in a state the workbench no
// longer declares, and the rows below carry that possibility rather than
// refusing it here, exactly as the single list did.
func (l *Library) canRoute(req *Request, card *bench.Card) (*bench.State, *bench.State, *Response) {
	if req.Actor == "" {
		return nil, nil, l.refuse(req, card, contract.NoOwner, "")
	}
	destination := l.Bench.StateByRef(req.State)
	if destination == nil {
		return nil, nil, l.refuse(req, card, contract.UnknownState, req.State)
	}
	operator := req.Actor == l.Bench.Operator
	if req.Override && !operator {
		return nil, nil, l.refuse(req, card, contract.NotOperator, req.Actor)
	}
	departure := l.Bench.State(card.State)
	if departure != nil && departure.OperatorOwned && !operator {
		return nil, nil, l.refuse(req, card, contract.NotOperator, req.Actor)
	}
	return destination, departure, nil
}

// canLand runs the rows of CORE-MOVE's list that read the card and the
// destination, in that list's order: the card is not blocked, the card is
// not held by somebody else, the move is not a forward move out of a done
// state, the destination stands below its capacity, and the destination is
// not being retired. It reports whether the capacity limit was reached and
// overridden, which is the flag the moved event carries.
func (l *Library) canLand(req *Request, card *bench.Card, destination, departure *bench.State) (bool, *Response, error) {
	if card.Substate == contract.SubstateBlocked {
		return false, l.refuse(req, card, contract.Blocked, card.BlockReason), nil
	}
	if card.Holder != "" && card.Holder != req.Actor {
		return false, l.refuse(req, card, contract.Held, card.Holder), nil
	}
	forward := departure != nil && destination.Position > departure.Position
	if forward && departure.Kind == contract.KindDone {
		return false, l.refuse(req, card, contract.Terminal, stateRef(departure)), nil
	}
	reached, err := l.atCapacity(destination)
	if err != nil {
		return false, nil, err
	}
	if reached && !req.Override {
		return false, l.refuse(req, card, contract.AtCapacity, stateRef(destination)), nil
	}
	if holder, retiring := l.retiring(destination.ID); retiring {
		return false, l.refuse(req, card, contract.Locked, holder), nil
	}
	return reached && req.Override, nil, nil
}

// move carries a card from one state to another. The list is CORE-MOVE's, in
// the order section 6.4 declares it.
func (l *Library) move(req *Request, card *bench.Card) *Response {
	destination, departure, override, refusal, err := l.canMove(req, card)
	if err != nil {
		return l.FromError(req, err)
	}
	if refusal != nil {
		return refusal
	}
	ev := bench.Event{
		TS:        bench.Stamp(l.Now()),
		Event:     contract.EventMoved,
		Actor:     req.Actor,
		From:      card.State,
		FromTitle: titleOf(departure),
		To:        destination.ID,
		ToTitle:   destination.Title,
		Override:  override,
	}
	card.State = destination.ID
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	response.Instructions = l.serve(card)
	response.LegalMoves = l.legalMoves(card)
	return response
}

// titleOf names a state that may be absent, which a card carrying a state the
// bench no longer declares makes possible.
func titleOf(state *bench.State) string {
	if state == nil {
		return ""
	}
	return state.Title
}

// atCapacity reports whether a state has reached its declared limit. The
// count is every live card in the state whatever its substate, because a
// blocked card still occupies the place.
func (l *Library) atCapacity(state *bench.State) (bool, error) {
	if state.Capacity <= 0 {
		return false, nil
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return false, err
	}
	count := 0
	for _, card := range cards {
		if card.State == state.ID {
			count++
		}
	}
	return count >= state.Capacity, nil
}

// release gives a card back. The list is CORE-RELEASE's.
func (l *Library) release(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	if card.Holder != req.Actor {
		return l.refuse(req, card, contract.NotHolder, card.Holder)
	}
	card.Substate = contract.SubstateReady
	card.Holder = ""
	card.ClaimSince = ""
	card.Expires = ""
	ev := bench.Event{
		TS:    bench.Stamp(l.Now()),
		Event: contract.EventReleased,
		Actor: req.Actor,
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	return response
}

// block raises an obstacle and frees the card. The list is CORE-BLOCK's.
func (l *Library) block(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	if req.Reason == "" {
		return l.refuse(req, card, contract.NoReason, "")
	}
	if card.Holder != "" && card.Holder != req.Actor {
		return l.refuse(req, card, contract.Held, card.Holder)
	}
	now := bench.Stamp(l.Now())
	card.Substate = contract.SubstateBlocked
	card.Holder = ""
	card.ClaimSince = ""
	card.Expires = ""
	card.BlockReason = req.Reason
	card.BlockKind = req.Kind
	card.BlockSince = now
	ev := bench.Event{
		TS:     now,
		Event:  contract.EventBlocked,
		Actor:  req.Actor,
		Reason: req.Reason,
		Kind:   req.Kind,
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	return response
}

// unblock lifts an obstacle, and is the operator's alone. The list is
// CORE-UNBLOCK's, with the operator check evaluated ahead of the substate
// check, so an owner who is not the operator is refused not-operator whatever
// the card's substate.
func (l *Library) unblock(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	if req.Actor != l.Bench.Operator {
		return l.refuse(req, card, contract.NotOperator, req.Actor)
	}
	if card.Substate != contract.SubstateBlocked {
		return l.refuse(req, card, contract.NotBlocked, card.Substate)
	}
	card.Substate = contract.SubstateReady
	card.BlockReason = ""
	card.BlockKind = ""
	card.BlockSince = ""
	ev := bench.Event{
		TS:    bench.Stamp(l.Now()),
		Event: contract.EventUnblocked,
		Actor: req.Actor,
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	return response
}

// join adds a workstream to a card's membership list. Its list is the card
// exists, the request names an owner, and the workstream resolves. Nobody is
// asked who holds the card, because membership is not a claim and the pull
// discipline is about claims; Comment already writes to a card somebody else
// holds on the same terms.
//
// Joining a workstream the card already belongs to succeeds and writes
// nothing, because membership is a set. The reference is still resolved
// first, so a typo is caught by dinah.unknown-workstream rather than passing
// as a silent success.
func (l *Library) join(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	workstream := l.Bench.WorkstreamByRef(req.Workstream)
	if workstream == nil {
		return l.refuse(req, card, contract.UnknownWorkstream, req.Workstream)
	}
	for _, joined := range card.Workstreams {
		if joined == workstream.ID {
			return l.ok(req, card)
		}
	}
	card.Workstreams = append(card.Workstreams, workstream.ID)
	ev := bench.Event{
		TS:         bench.Stamp(l.Now()),
		Event:      contract.EventWorkstreamJoined,
		Actor:      req.Actor,
		Workstream: workstream.ID,
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	return response
}

// leave removes exactly the workstream named from a card's membership list and
// leaves every other entry where it was. Its list is join's, and leaving a
// workstream the card never joined succeeds and writes nothing.
func (l *Library) leave(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	workstream := l.Bench.WorkstreamByRef(req.Workstream)
	if workstream == nil {
		return l.refuse(req, card, contract.UnknownWorkstream, req.Workstream)
	}
	kept := make([]string, 0, len(card.Workstreams))
	for _, joined := range card.Workstreams {
		if joined == workstream.ID {
			continue
		}
		kept = append(kept, joined)
	}
	if len(kept) == len(card.Workstreams) {
		return l.ok(req, card)
	}
	card.Workstreams = kept
	ev := bench.Event{
		TS:         bench.Stamp(l.Now()),
		Event:      contract.EventWorkstreamLeft,
		Actor:      req.Actor,
		Workstream: workstream.ID,
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	return response
}

// lapseRead applies an expired claim from a read path, taking the card's lock
// for the write and re-reading the card under it.
//
// A lock another process holds means somebody is mid-transaction on this
// card, and that transaction lapses the claim itself, so the read leaves the
// card alone rather than failing: a read has no business refusing because a
// write is in flight.
func (l *Library) lapseRead(card *bench.Card) error {
	if !card.Lapsed(l.Now()) {
		return nil
	}
	lock, err := bench.Acquire(card.Dir, "", bench.Stamp(l.Now()))
	if err != nil {
		return nil
	}
	defer lock.Release()
	fresh, err := bench.LoadCard(l.Bench.CardsRoot(), card.ID)
	if err != nil {
		return nil
	}
	if !fresh.Lapsed(l.Now()) {
		*card = *fresh
		return nil
	}
	if err := l.lapse(fresh); err != nil {
		return err
	}
	*card = *fresh
	return nil
}

// commit finishes a mutation inside the transaction its caller opened: the
// anchor write through a temporary and a rename, then the journal appends. The
// card's lock is already held by Do, which is what keeps the decision and the
// write on the same side of it.
//
// The variadic event list carries every journal entry the transaction
// produces, so pull can write a single claimed and a single moved event in
// one card.Save and one round of AppendEvent calls. A single-event caller
// passes one element, the common case; every event lands with the same
// timestamp the card's anchor stamp bears.
func (l *Library) commit(req *Request, card *bench.Card, events ...bench.Event) (*Response, error) {
	if err := card.Save(); err != nil {
		return nil, err
	}
	for _, ev := range events {
		if err := bench.AppendEvent(card.JournalPath(), ev); err != nil {
			return nil, err
		}
	}
	return l.ok(req, card), nil
}

// ParseDuration reads the duration a claim's expiry carries, accepting Go's
// own spellings and the bare day suffix a lease measured in days wants.
func ParseDuration(text string) (time.Duration, error) {
	if text == "" {
		return 0, nil
	}
	if days, ok := strings.CutSuffix(text, "d"); ok {
		hours, err := time.ParseDuration(days + "h")
		if err != nil {
			return 0, contract.Refuse(contract.Malformed, text)
		}
		return hours * 24, nil
	}
	parsed, err := time.ParseDuration(text)
	if err != nil {
		return 0, contract.Refuse(contract.Malformed, text)
	}
	return parsed, nil
}
