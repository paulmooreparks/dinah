package verb

import (
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Pull is the one-command route that combines a claim and a move. It fixes
// the destination state, takes the card at the head of that state's upstream
// ready queue, and writes the claim and the move in one card.Save and one
// pair of journal appends.
//
// Pull is not a contract verb and is not dispatched through Do, because the
// bare form carries no card reference and the named form's card is chosen
// from the destination's upstream queue rather than typed. Pull runs its own
// transaction. The workbench pair and the invocation's own refusals are
// raised here, before any state is considered and before any lock is taken,
// because each of them answers the same for every state on the workbench and
// folding them into the qualifying predicate would turn a refusal into an
// empty answer. The state-scoped refusals narrow the qualifying set instead,
// and the card-scoped ones are evaluated under the card's lock by the inner
// pull method, which shares canClaim and canMove with claim and move.
//
// --no-claim weakens no precondition. Pull runs the whole precondition list
// whether or not it claims, so a card that a claim would refuse is a card a
// pull refuses, and the option changes what pull writes rather than what pull
// allows.
func (l *Library) Pull(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if req.Actor == "" {
		return l.refuse(req, nil, contract.NoOwner, "")
	}
	// The unknown-state row belongs to the named form alone, since the bare
	// form names no state to be unknown, and it is settled ahead of the
	// override row because that is the order the table declares and the
	// order the generated help prints.
	var named *bench.State
	if req.State != "" {
		named = l.Bench.StateByRef(req.State)
		if named == nil {
			return l.refuse(req, nil, contract.UnknownState, req.State)
		}
	}
	if req.Override && req.Actor != l.Bench.Operator {
		return l.refuse(req, nil, contract.NotOperator, req.Actor)
	}
	destination, refusal, err := l.pullDestination(req, named)
	if err != nil {
		return l.FromError(req, err)
	}
	if refusal != nil {
		return refusal
	}
	if destination == nil {
		return l.okEmpty(req, nil)
	}
	upstream := upstreamOf(destination, l.Bench.States)
	if upstream == nil {
		detail := stateRef(destination)
		return l.refuseWith(req, nil, contract.NoUpstream, detail, map[string]string{"state": detail})
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return l.FromError(req, err)
	}
	head := headOfReady(upstream.ID, cards)
	if head == nil {
		return l.okEmpty(req, destination)
	}
	req.Card = head.Ref(l.Bench.Slug)
	req.State = stateRef(destination)
	// Pull fills in no basis of its own. The revision read during selection
	// is not a revision the caller read, and standing it in as one turns
	// losing the race for a card into `stale`, which tells a person the card
	// moved since they read it about a card they never read. Losing the race
	// is what rows 9 and 10 answer, under the lock, in the words claim uses.
	return l.pullTransaction(req, head)
}

// pullDestination fixes the state a pull will land its card in. A named form
// resolves to the state the caller typed, which Pull has already found. A
// bare form runs the qualifying predicate over the flow and answers with the
// one state that qualifies, a refusal when more than one does, or a nil state
// when none does, which Pull turns into the empty answer.
//
// The retiring row is left to the inner pull for the named form, so that both
// forms reach it through canMove and neither carries a second copy of the
// test. The predicate reads it for the bare form because a retiring state is
// one a pull could not land in, so leaving it in the qualifying set would
// make the bare form ambiguous where a reader would say it plainly is not.
func (l *Library) pullDestination(req *Request, named *bench.State) (*bench.State, *Response, error) {
	if named != nil {
		return named, nil, nil
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, nil, err
	}
	qualifying := l.pullCandidates(req, cards)
	if len(qualifying) > 1 {
		carried := map[string]string{"states": strings.Join(qualifying, "\n")}
		return nil, l.refuseWith(req, nil, contract.AmbiguousState, "", carried), nil
	}
	if len(qualifying) == 1 {
		return l.Bench.StateByRef(qualifying[0]), nil, nil
	}
	return nil, nil, nil
}

// pullCandidates returns the reference of every state this invocation could
// pull into, in flow order, so the sentence the ambiguous refusal prints
// reads as the workbench reads.
//
// The six conditions are the state-scoped rows of the precondition list, and
// each one names the row it stands for. The state has an upstream state, or
// it stands first in the flow and nothing precedes it. That upstream holds at
// least one ready card, which is the one condition with no row behind it,
// because nothing waiting is an answer rather than a refusal. The upstream's
// kind is not done, since a forward move out of a done state refuses. The
// upstream is not operator-owned unless the owner asking is the operator. The
// destination stands below its capacity limit, or the invocation carries the
// override marker, whose legality Pull has already settled. The destination is
// not being retired.
//
// The predicate reads the workbench without holding any lock, so the set it
// returns is a prediction about what a pull would do. The authoritative
// sequence runs under the card's lock once the destination is fixed, and when
// the two disagree, because the workbench changed in between, the lock's
// answer is the one the caller is given.
func (l *Library) pullCandidates(req *Request, cards []*bench.Card) []string {
	operator := req.Actor == l.Bench.Operator
	var qualifying []string
	for _, state := range l.Bench.States {
		upstream := upstreamOf(state, l.Bench.States)
		if upstream == nil {
			continue
		}
		if upstream.Kind == contract.KindDone {
			continue
		}
		if upstream.OperatorOwned && !operator {
			continue
		}
		if headOfReady(upstream.ID, cards) == nil {
			continue
		}
		reached, err := l.atCapacity(state)
		if err == nil && reached && !req.Override {
			continue
		}
		if _, retiring := l.retiring(state.ID); retiring {
			continue
		}
		qualifying = append(qualifying, stateRef(state))
	}
	return qualifying
}

// okEmpty answers a pull that found nothing to take, at exit 0 with no card
// and nothing written to any journal. The named form answers this way when
// the destination's upstream state holds no ready card, and the bare form
// answers it when no state qualifies at all, because the two are one
// condition and answering them differently would be two vocabularies for one
// fact.
func (l *Library) okEmpty(req *Request, destination *bench.State) *Response {
	response := &Response{
		Outcome:     contract.OutcomeOK,
		Verb:        req.Verb,
		Affordances: l.affordances(nil),
	}
	if destination == nil {
		response.Message = "answer.pull.empty.bare"
		return response
	}
	response.Message = "answer.pull.empty.named"
	response.MessageValues = map[string]string{
		"upstream":    upstreamTitle(destination, l.Bench.States),
		"destination": destination.Title,
	}
	return response
}

// pullTransaction runs the under-lock half of a pull: take the card's lock,
// fire the Interleave hook, re-read the card under the lock, lapse an expired
// claim, compare a basis the caller supplied, and hand the fresh card to the
// inner pull. It is Do's shape, with the one difference that the caller chose
// the card rather than the request naming it.
//
// The hook fires before the card is read rather than after, because the
// window it stands in for is the window between the selection and the lock,
// and a card mutated after the read is a mutation this transaction has
// already looked past. Firing it here is what makes the race the refusal
// table describes reachable from a test.
func (l *Library) pullTransaction(req *Request, head *bench.Card) *Response {
	lock, err := bench.Acquire(head.Dir, req.Actor, bench.Stamp(l.Now()))
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	if l.Interleave != nil {
		l.Interleave()
	}
	card, err := bench.LoadCard(l.Bench.CardsRoot(), head.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	if err := l.lapse(card); err != nil {
		return l.FromError(req, err)
	}
	if req.Basis != "" && req.Basis != card.Revision {
		return &Response{
			Outcome:     contract.OutcomeStale,
			Verb:        req.Verb,
			Card:        l.view(card),
			Basis:       req.Basis,
			Affordances: l.affordances(card),
		}
	}
	return l.pull(req, card)
}

// pull is the inner write, reached once the card has been chosen and locked.
// It evaluates rows 8 to 13 of pull's table in the order the table declares,
// out of the same two functions move and claim run, so a pull refuses in the
// words a move and a claim already refuse in.
//
// canRoute is the move's own list up to and including the departure row,
// which is row 8. claimableSubstate is the claim's own substate pair, rows 9
// and 10, and it stands between the halves because row 10 takes claim's
// stricter test: a pull that claims cannot take a card already active
// whoever holds it, where the move's row admits a card the owner asking
// holds. canLand is the rest of the move's list, rows 11 to 13. Its own
// blocked and held rows are reached with the substate already settled by the
// pair above, so neither can decide a pull.
//
// claimableSubstate runs whether or not the caller passed --no-claim, because
// the option changes what pull writes and not what pull allows.
//
// Pull's caller has already fixed req.State to the destination's reference,
// so canRoute resolves the destination exactly as the named form of a move
// would. Rows 3 to 5 run again here, harmlessly, because they are the front
// of the move's list and answering them twice cannot change an answer.
func (l *Library) pull(req *Request, card *bench.Card) *Response {
	destination, departure, refusal := l.canRoute(req, card)
	if refusal != nil {
		return refusal
	}
	if refusal := l.claimableSubstate(req, card); refusal != nil {
		return refusal
	}
	override, refusal, err := l.canLand(req, card, destination, departure)
	if err != nil {
		return l.FromError(req, err)
	}
	if refusal != nil {
		return refusal
	}
	now := l.Now()
	stamp := bench.Stamp(now)
	events := make([]bench.Event, 0, 2)
	if req.NoClaim {
		// A pull that does not claim leaves the card in the ready substate
		// an ordinary move leaves, so the next owner to claim or to pull it
		// onward takes it from there.
		card.Substate = contract.SubstateReady
		card.Holder = ""
		card.ClaimSince = ""
		card.Expires = ""
	} else {
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
	}
	// The departure identifier is read before the card is carried, because
	// the moved event names where the card came from and the assignment
	// below overwrites it. move reads it ahead of the same assignment for
	// the same reason.
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

// upstreamOf returns the state standing immediately before the given state in
// the declared flow, or nil when that state stands first and nothing precedes
// it. Position is a dense index over the live states, so the predecessor is
// the row before it.
func upstreamOf(state *bench.State, states []*bench.State) *bench.State {
	if state == nil || state.Position == 0 {
		return nil
	}
	return states[state.Position-1]
}

// upstreamTitle names the upstream state for the sentence the named form's
// empty answer prints, and is empty when the state stands first in the flow.
func upstreamTitle(destination *bench.State, states []*bench.State) string {
	upstream := upstreamOf(destination, states)
	if upstream == nil {
		return ""
	}
	return upstream.Title
}
