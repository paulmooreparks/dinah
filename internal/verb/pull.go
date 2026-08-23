package verb

import (
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Pull is the one-command route that combines a claim and a move. It picks
// the destination state, takes a card from the head of that state's upstream
// ready queue, writes the claim stamp, writes the move, and journals both in
// one card.Save and one transaction. --no-claim makes the claim half a no-op
// the move half borrows the substate of, so a card lands where the reader
// asked without taking its lease.
//
// Pull is not a contract verb and is not dispatched through Do, because the
// bare form has no card reference and the named form's card is chosen from
// the destination's upstream queue rather than typed. Pull runs its own
// transaction: the workbench-level pair (rows 1 and 2) and the five
// invocation-level checks (rows 3 through 7) are evaluated before any lock
// is taken; rows 8 through 13 are evaluated under the card's lock by the
// inner pull method, which shares canClaim and canMove with claim and move.
//
// --no-claim weakens no precondition. Pull runs the whole table whether or
// not it claims, so a card that a claim would refuse is a card a pull
// refuses, and the option changes what pull writes rather than what pull
// allows.
func (l *Library) Pull(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if req.Actor == "" {
		return l.refuse(req, nil, contract.NoOwner, "")
	}
	// Row 4 belongs to the named form alone, since the bare form names no
	// state to be unknown, and it is settled ahead of row 5 because that is
	// the order the table declares and the order the help prints.
	var named *bench.State
	if req.State != "" {
		if named = l.Bench.StateByRef(req.State); named == nil {
			if stranded := l.Bench.StrandedStateByRef(req.State); stranded != "" {
				return l.refuse(req, nil, contract.Locked, stranded)
			}
			return l.refuse(req, nil, contract.UnknownState, req.State)
		}
	}
	if req.Override && req.Actor != l.Bench.Operator {
		return l.refuse(req, nil, contract.NotOperator, req.Actor)
	}
	l.lastActor = req.Actor
	l.lastOverride = req.Override
	destination, empty, refusal, err := l.resolvePullDestination(req, named)
	if err != nil {
		return l.FromError(req, err)
	}
	if refusal != nil {
		return refusal
	}
	if empty {
		return l.okEmpty(req, nil)
	}
	upstreamID := upstreamOf(destination, l.Bench.States)
	if upstreamID == "" {
		return l.refuseWith(req, nil, contract.NoUpstream, stateRef(destination),
			map[string]string{"state": stateRef(destination)})
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return l.FromError(req, err)
	}
	head := headOfReady(upstreamID, cards)
	if head == nil {
		return l.okEmpty(req, destination)
	}
	req.Card = head.Ref(l.Bench.Slug)
	req.State = stateRef(destination)
	if req.Basis == "" {
		req.Basis = head.Revision
	}
	return l.pullTransaction(req, head)
}

// resolvePullDestination picks the destination state for both forms. The
// named form's state has already been resolved by Pull, which settles row 4
// ahead of row 5, so all that remains here is row 13's retiring test. The
// bare form refuses AmbiguousState when more than one state qualifies.
//
// Whether the owner asking may take a card out of the upstream state is row
// 8, and it is canMove's departure test rather than a test of the
// destination: a state is operator-owned to say who may move work out of
// it, so a pull INTO an operator-owned state is as legal as a move into one.
// The qualifying predicate reads the upstream for the same reason, which is
// what keeps the bare form's answer and the named form's answer the same
// answer.
//
// Both forms return empty=true when nothing waits for a pull: the bare
// form when no state qualifies, and (after the destination is fixed) when
// the destination's upstream state holds no ready card.
func (l *Library) resolvePullDestination(req *Request, named *bench.State) (*bench.State, bool, *Response, error) {
	if named != nil {
		if holder, retiring := l.retiring(named.ID); retiring {
			return nil, false, l.refuse(req, nil, contract.Locked, holder), nil
		}
		return named, false, nil, nil
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, false, nil, err
	}
	qualifying := pullCandidates(cards, l.Bench.States, l)
	if len(qualifying) > 1 {
		return nil, false, l.refuseWith(req, nil, contract.AmbiguousState, "",
			map[string]string{"states": strings.Join(qualifying, "\n")}), nil
	}
	if len(qualifying) == 1 {
		return l.Bench.StateByRef(qualifying[0]), false, nil, nil
	}
	return nil, true, nil, nil
}

// okEmpty answers a pull that found nothing to do at exit 0 with no card.
// The named form answers the same way when its upstream state holds no
// ready card; the bare form answers it when nothing qualifies at all. The
// sentence reads from the catalog pass and is the one sentence the refusal
// table explicitly carves out as an answer rather than a refusal.
func (l *Library) okEmpty(req *Request, destination *bench.State) *Response {
	response := &Response{
		Outcome:     contract.OutcomeOK,
		Verb:        req.Verb,
		Affordances: l.affordances(nil),
	}
	if destination == nil {
		response.Message = "answer.pull.empty.bare"
	} else {
		response.Message = "answer.pull.empty.named"
		response.MessageValues = map[string]string{
			"upstream":     upstreamTitleOf(destination, l.Bench.States),
			"destination":  destination.Title,
		}
	}
	return response
}

// upstreamTitleOf returns the title of the state immediately ahead of
// destination in the flow, or "" when destination stands first. It is the
// word the named-form empty answer uses to name the upstream state that
// had no ready card.
func upstreamTitleOf(destination *bench.State, states []*bench.State) string {
	if destination == nil || destination.Position == 0 {
		return ""
	}
	return states[destination.Position-1].Title
}

// pullTransaction runs the under-lock half of a pull: take the card's lock,
// re-read it, lapse expired claims, fire the Interleave hook, compare the
// basis, and hand the fresh card to the inner pull for the precondition
// list and the journal write. It mirrors Do's shape for every verb that
// already runs through it, with the difference that the card was chosen by
// the caller rather than typed.
func (l *Library) pullTransaction(req *Request, head *bench.Card) *Response {
	lock, err := bench.Acquire(head.Dir, req.Actor, bench.Stamp(l.Now()))
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	card, err := bench.LoadCard(l.Bench.CardsRoot(), head.ID)
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
	return l.pull(req, card)
}

// pull is the inner write that the pull transaction reaches once the card
// has been resolved and locked. canClaim gates the claimed event for the
// claim half and canMove gates the moved event for the move half; together
// they run rows 8 through 13 of the spec's refusal table. The events are
// appended in the order the spec calls for: claimed, then moved.
//
// Pull's caller fixes req.State to the destination's reference on entry,
// so canMove resolves it the same way the named form's would.
func (l *Library) pull(req *Request, card *bench.Card) *Response {
	if path := bench.SiblingPath(card.Dir); path != "" {
		if record, present := bench.ReadLockRecord(path); present {
			return l.refuse(req, card, contract.Locked, record.Actor)
		}
	}
	if !req.NoClaim {
		if refusal := l.canClaim(req, card); refusal != nil {
			return refusal
		}
	}
	destination, departure, override, refusal, err := l.canMove(req, card)
	if err != nil {
		return l.FromError(req, err)
	}
	if refusal != nil {
		return refusal
	}
	now := l.Now()
	stamp := bench.Stamp(now)
	events := make([]bench.Event, 0, 2)
	if !req.NoClaim {
		card.Substate = contract.SubstateActive
		card.Holder = req.Actor
		card.ClaimSince = stamp
		if req.Expires > 0 {
			card.Expires = bench.Stamp(now.Add(req.Expires))
		}
		events = append(events, bench.Event{
			TS:      stamp,
			Event:   contract.EventClaimed,
			Actor:   req.Actor,
			Expires: card.Expires,
		})
	} else {
		// A --no-claim pull moves a card without claiming it. The card
		// lands where the caller asked, in the same ready substate a
		// plain move leaves; ownership stays with whoever held it
		// before, and the next claim takes it from there.
		card.Substate = contract.SubstateReady
		card.Holder = ""
		card.ClaimSince = ""
		card.Expires = ""
	}
	// The departure identifier is read before the card is carried, because
	// the moved event names where the card came from and the assignment
	// below overwrites that. move builds its event ahead of the same
	// assignment for the same reason.
	from := card.State
	card.State = destination.ID
	events = append(events, bench.Event{
		TS:        stamp,
		Event:     contract.EventMoved,
		Actor:     req.Actor,
		From:      from,
		FromTitle: titleOf(departure),
		To:        destination.ID,
		ToTitle:   destination.Title,
		Override:  override,
	})
	response, err := l.commit(req, card, events...)
	if err != nil {
		return l.FromError(req, err)
	}
	response.Instructions = l.serve(card)
	response.LegalMoves = l.legalMoves(card)
	return response
}

// pullCandidates returns the slugs of every state that has a ready card
// sitting in some upstream state of it and that this owner, with these flags,
// could pull into. The bare form reports one entry per qualifying state; the
// slugs are returned in flow order so the sentence the refusal prints reads
// as the workbench reads.
//
// The predicate runs the six conditions the spec resolves the qualifying
// set against: the state has an upstream, that upstream holds at least one
// ready card, the upstream's kind is not done, the upstream is not
// operator-owned for a non-operator asker, the destination stands below its
// capacity (or the asker is the operator carrying --override), and the
// destination is not being retired. The empty upstream test, the no-
// upstream test, the operator-owned test and the done test read state fields
// the candidate set enumerates from; the capacity and retiring tests call
// the same helpers canMove reaches for under the lock.
func pullCandidates(cards []*bench.Card, states []*bench.State, l *Library) []string {
	var out []string
	for _, state := range states {
		upstream := upstreamOf(state, states)
		if upstream == "" {
			continue
		}
		upstreamState := states[upstreamIndex(state, states)]
		if upstreamState != nil && upstreamState.Kind == contract.KindDone {
			continue
		}
		if upstreamState != nil && upstreamState.OperatorOwned && !l.isPullOperator() {
			continue
		}
		if headOfReady(upstream, cards) == nil {
			continue
		}
		if reached, err := l.atCapacity(state); err == nil && reached && !l.isPullOperatorWithOverride() {
			continue
		}
		if _, retiring := l.retiring(state.ID); retiring {
			continue
		}
		out = append(out, stateRef(state))
	}
	return out
}

// upstreamIndex returns the position of the state that stands immediately
// before the named state in the flow. It is the integer counterpart of
// upstreamOf, kept apart because the caller already has the upstream state's
// identifier from upstreamOf and only needs its row to read its fields.
func upstreamIndex(state *bench.State, states []*bench.State) int {
	if state == nil || state.Position == 0 {
		return -1
	}
	return state.Position - 1
}

// isPullOperator reports whether the actor Pull was asked by is the
// workbench's operator. It reads the field Pull set on entry, so the
// qualifying predicate sees the same asker Pull's own preflight did.
func (l *Library) isPullOperator() bool {
	return l.lastActor == l.Bench.Operator && l.lastActor != ""
}

// isPullOperatorWithOverride reports whether the actor Pull was asked by is
// the operator carrying --override. Override matters here because a
// destination at its capacity limit qualifies only for the operator with
// the marker; a destination in any other state qualifies for the same
// operator without it.
func (l *Library) isPullOperatorWithOverride() bool {
	return l.isPullOperator() && l.lastOverride
}

// upstreamOf returns the identifier of the state immediately ahead of state
// in the flow, or "" when state stands first. A state with no upstream
// cannot be pulled into, because nothing precedes it for a card to come
// from.
func upstreamOf(state *bench.State, states []*bench.State) string {
	if state == nil || state.Position == 0 {
		return ""
	}
	return states[state.Position-1].ID
}