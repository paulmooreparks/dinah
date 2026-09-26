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
// compared against and the state every precondition reads are the ones on
// disk at the moment of the write rather than a snapshot taken before it. Two
// processes reaching the same card therefore cannot both see it ready, since
// the second is refused the lock outright.
func (l *Library) Do(req *Request) *Response {
	found, refused := l.admit(req)
	if refused != nil {
		return refused
	}
	lock, err := l.Bench.Acquire(found.Card.Dir, req.Actor, bench.Stamp(l.Now()))
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	card, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), found.Card.ID)
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
		view, err := l.view(card, l.dayOf(req))
		if err != nil {
			return l.FromError(req, err)
		}
		response := &Response{
			Outcome:     contract.OutcomeStale,
			Verb:        req.Verb,
			Card:        view,
			Basis:       req.Basis,
			Affordances: l.affordances(card),
		}
		return response
	}
	if _, err := l.Bench.WitnessDivergence(req.Actor, bench.Stamp(l.Now()), card); err != nil {
		return l.FromError(req, err)
	}
	response := l.evaluate(req, card)
	if found.StalePrefix != "" && response.Outcome == contract.OutcomeOK {
		response.Warning = "warn.stale-prefix"
		response.WarningDetail = found.StalePrefix
	}
	return response
}

// admit runs the rows Do runs before it takes the card's lock, in Do's order:
// the workbench has an operator, the declared harness is well formed, the
// card resolves, and the request names an owner. It answers the card the
// reference found, or the refusal of the first row that fails. Do and
// MoveDestinations both call it, so a row added here reaches a real act and
// the completion of a move alike. A row that refuses a move belongs here or in
// canMove. Written anywhere else in Do, evaluate or move, it reaches the move
// alone, and no test fails because of where it was written; MoveDestinations'
// comment says what does protect the filter.
//
// The owner row sits here rather than only inside each verb's own function,
// because canClaim, canRoute, release, block, unblock, join and leave all run
// after the lock is acquired and after WitnessDivergence has already had the
// chance to write a journal line under an unnamed actor. This is the order
// every one of those functions' own check already keeps: card exists, then
// owner named. The per-verb checks stay; pull calls canRoute/canLand
// directly, on its own transaction, without going through Do at all, so those
// checks remain load-bearing for that caller.
func (l *Library) admit(req *Request) (*bench.Resolved, *Response) {
	if l.Bench.Operator == "" {
		return nil, l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return nil, refused
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	if req.Actor == "" {
		return nil, l.refuse(req, nil, contract.NoOwner, "")
	}
	return found, nil
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
		// The hold is read before the claim, while the card has not started:
		// once claimed, no link holds it, and the warning would say nothing.
		// It is read in a statement of its own, because the claim writes
		// the card it is handed, and Go evaluates a call's arguments left to
		// right, so a hold read as the second argument would read the card
		// the first had already claimed.
		hold, err := l.selectionHold(req)
		if err != nil {
			return l.FromError(req, err)
		}
		answer := hold(card)
		return l.warnWithheld(l.claim(req, card), answer)
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
	case GrantPermission:
		return l.grant(req, card)
	case RevokePermission:
		return l.revoke(req, card)
	}
	return l.refuse(req, card, contract.UnknownVerb, req.Verb)
}

// warnWithheld carries a warning on a successful claim of a card the start
// hold would have withheld from selection. The claim is not refused: the
// caller named the card, and selection withholding it is a statement about
// what the tool hands out rather than about what a person may choose. A
// response already carrying a warning keeps it, and Do's own stale-prefix
// warning, set after this runs, takes the slot where both apply.
//
// answer is the hold read before the claim. A card waiting on another card
// is warned with the cards it waited on, each once, since the claim starts it
// and nothing withholds it while the claim holds. A card held only by a date,
// its own start_after or the end of a lag, is warned with that date. The
// card's own start_after does not depend on whether it has started, so
// reading it before the claim answers what reading it after did.
func (l *Library) warnWithheld(response *Response, answer holdAnswer) *Response {
	if response == nil || response.Outcome != contract.OutcomeOK || response.Warning != "" {
		return response
	}
	if len(answer.Waiting) > 0 {
		response.Warning = "warn.waiting-on"
		response.WarningDetail = strings.Join(holderRefs(answer.Waiting, l.Bench.Slug), ", ")
		return response
	}
	if answer.Held {
		response.Warning = "warn.before-start-after"
		response.WarningDetail = answer.From
	}
	return response
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
		Actor:   bench.NamedActor(holder),
		Expires: card.Expires,
	}
	clearLapsedClaim(card)
	if err := card.Save(); err != nil {
		return err
	}
	return bench.AppendEvent(card.JournalPath(), ev)
}

// clearLapsedClaim is what a lapse does to the card in memory: the card is
// ready again, and the holder and both claim fields are gone. lapse saves it
// and journals the expiry; MoveDestinations asks what a move would find and
// saves nothing, so both call this and neither restates the four fields.
func clearLapsedClaim(card *bench.Card) {
	card.State = contract.StateReady
	card.Holder = ""
	card.ClaimSince = ""
	card.Expires = ""
}

// canClaim runs the precondition sequence CORE-CLAIM declares, and Dinah's own
// tier row after it. It returns a refusal response when the card may not be
// claimed and nil when the card is ready to take. Pull reuses the same checks
// for the claimed event it bundles with its move, so both verbs run the same
// gates in the same order.
//
// The tier row is Dinah's own, and it is appended rather than inserted among
// the profile's seven, on the terms check.move.10 already keeps: inserting it
// would renumber rows the profile numbers, and dinah help claim heads its
// table Order and promises the rows in the order each is checked, so the
// published numbering decides the evaluation order rather than the code
// deciding what the page prints. A claim that is both below the card's tier
// and carrying an unresolved item is therefore refused unresolved-item.
func (l *Library) canClaim(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	if req.Holder != "" && req.Holder != req.Actor {
		return l.refuse(req, card, contract.NotRequester, req.Holder)
	}
	if refusal := l.claimableState(req, card); refusal != nil {
		return refusal
	}
	if refusal := l.claimableColumn(req, card); refusal != nil {
		return refusal
	}
	if refusal := l.claimableItems(req, card); refusal != nil {
		return refusal
	}
	return l.claimableTier(req, card, l.Bench.Column(card.Column))
}

// claimableItems carries the last row of the claim's list, CORE-CLAIM-10. It
// stands after claimableColumn because every row ahead of it asks about the
// card's state or the column the card stands in, where this one reads the
// card's own checklist and the columns the workbench declares, which neither
// of those touches.
//
// No column declares this and no workbench opts into it, so the refusal binds
// every claim on every workbench. What it refuses over is narrower than the
// card's whole checklist: an item naming a column the workbench declares is
// left to that column's own hold, and what reaches this row is the item no
// column can ever be asked about. The card's position is not read, on either
// side, so pull's own row answers the same way at the column the card leaves
// and at the column it lands in.
func (l *Library) claimableItems(req *Request, card *bench.Card) *Response {
	blocking, err := l.Bench.BlockingItems(card.Dir)
	if err != nil {
		return l.FromError(req, err)
	}
	if len(blocking) == 0 {
		return nil
	}
	return l.refuse(req, card, contract.UnresolvedItem, blocking[0].ID)
}

// claimableColumn carries the last two rows of the claim's list, which are the
// rows that read the column a card stands in rather than the card. A column
// where no owner takes work up refuses the claim whoever asks, the operator
// included, because taking work up is a fact about the column and not a
// permission.
//
// Two names answer that first rule. A column declaring awaiting_outside tells
// a reader who the workbench is waiting on, so that name wins wherever the
// flag is set. A column that takes no work up by kind has nobody to name, so
// it answers dinah.takes-no-work instead.
//
// The second row asks a different question of a column that does take work
// up. Such a column can still be reserved to the operator, and CORE-CLAIM-8
// refuses the claim there to every other owner under the name CORE-MOVE-6
// already reports for the departure side of the same reservation. The two
// rows are not one axis wearing two names. A column declaring
// awaiting_outside says nobody inside the workbench can act yet, an
// operator-owned column says only the operator may take the card up, and a
// column can be either, both or neither.
//
// It stands after claimableState rather than before it so that a card
// which is both blocked and standing at such a column answers the profile's
// own blocked, which is the answer a reader of the contract expects and the
// answer CORE-OUT-6 makes observable.
func (l *Library) claimableColumn(req *Request, card *bench.Card) *Response {
	column := l.Bench.Column(card.Column)
	if column != nil && !column.HoldsState(contract.StateActive) {
		return l.refuse(req, card, takesNoWorkName(column), columnRef(column))
	}
	if operatorReservesClaim(column, req.Actor, l.Bench.Operator) {
		return l.refuse(req, card, contract.NotOperator, req.Actor)
	}
	return nil
}

// takesNoWorkName picks the refusal name for a column where no owner takes
// work up. Both the claim path and the move's destination row choose between
// the two names this way, so the choice is written once.
func takesNoWorkName(column *bench.Column) string {
	if column.AwaitingOutside {
		return contract.AwaitingOutside
	}
	return contract.TakesNoWork
}

// operatorReservesClaim reports whether an operator-owned column refuses a
// claim to this actor. CORE-CLAIM-8 and canLand's destination row both read
// it, so an operator-owned column means the same thing whichever act is
// taking a card up there: an ordinary claim in place, or a pull that lands
// holding it. A card standing at a column the workbench no longer declares
// reaches this with a nil column and is reserved to nobody.
func operatorReservesClaim(column *bench.Column, actor, operator string) bool {
	return column != nil && column.OperatorOwned && actor != operator
}

// claimableState runs the last two rows of the claim's list, blocked then
// held, against the state the card carries. It stands apart from canClaim
// because pull evaluates these two rows at rows 9 and 10 of its own longer
// list, between the move's departure row and the move's destination rows, and
// a caller reaching them through canClaim would have to run the whole claim
// list at that point. Splitting the tail off changes neither list's order:
// canClaim still evaluates no-owner, not-requester, blocked, held.
//
// The held row here is the stricter of the tool's two: it refuses a card
// whose state is active whoever holds it, where the move's row admits a
// card the owner asking already holds. A claim cannot take a card somebody is
// already working, its own asker included.
func (l *Library) claimableState(req *Request, card *bench.Card) *Response {
	if card.State == contract.StateBlocked {
		return l.refuse(req, card, contract.Blocked, card.BlockReason)
	}
	if card.State == contract.StateActive {
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
	card.State = contract.StateActive
	card.Holder = req.Actor
	card.ClaimSince = bench.Stamp(now)
	if req.Expires > 0 {
		card.Expires = bench.Stamp(now.Add(req.Expires))
	}
	ev := bench.Event{
		TS:      card.ClaimSince,
		Event:   contract.EventClaimed,
		Actor:   req.Acting(),
		Expires: card.Expires,
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	response.Instructions, response.ChainServed, err = l.serve(req, card)
	if err != nil {
		return l.FromError(req, err)
	}
	response.LegalMoves = l.legalMoves(card)
	loop, err := l.cardLoop(card)
	if err != nil {
		return l.FromError(req, err)
	}
	response.Loop = loop
	return response
}

// canMove runs the precondition sequence CORE-MOVE declares, in the order
// section 6.4 states it. It returns the destination and departure column when
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
// claim's two state rows between them, which is where pull's own table
// puts them. Reading the two halves in this order is reading CORE-MOVE's
// list in CORE-MOVE's order, and neither half is called anywhere in an order
// this function does not also produce.
func (l *Library) canMove(req *Request, card *bench.Card) (*bench.Column, *bench.Column, bool, *Response, error) {
	destination, departure, refusal := l.canRoute(req, card)
	if refusal != nil {
		return nil, nil, false, refusal, nil
	}
	override, refusal, err := l.canLand(req, card, destination, departure, false)
	if err != nil {
		return nil, nil, false, nil, err
	}
	if refusal != nil {
		return nil, nil, false, refusal, nil
	}
	return destination, departure, override, nil, nil
}

// canRoute runs the rows of CORE-MOVE's list that read the request and the
// two columns, in that list's order: the request names an owner, the named
// destination is declared, an override marker is the operator's, and the
// departure is one the owner asking may move work out of. It returns the
// resolved destination and departure so its caller resolves neither twice.
//
// The departure can be nil, when the card stands in a column the workbench no
// longer declares, and the rows below carry that possibility rather than
// refusing it here, exactly as the single list did.
func (l *Library) canRoute(req *Request, card *bench.Card) (*bench.Column, *bench.Column, *Response) {
	if req.Actor == "" {
		return nil, nil, l.refuse(req, card, contract.NoOwner, "")
	}
	destination := l.Bench.ColumnByRef(req.Column)
	if destination == nil {
		return nil, nil, l.refuse(req, card, contract.UnknownColumn, req.Column)
	}
	operator := req.Actor == l.Bench.Operator
	if req.Override && !operator {
		return nil, nil, l.refuse(req, card, contract.NotOperator, req.Actor)
	}
	departure := l.Bench.Column(card.Column)
	if departure != nil && departure.OperatorOwned && !operator {
		return nil, nil, l.refuse(req, card, contract.NotOperator, req.Actor)
	}
	return destination, departure, nil
}

// canLand runs the rows of CORE-MOVE's list that read the card and the
// destination, in that list's order: the card is not blocked, the card is
// not held by somebody else, the move is not a forward move out of a done
// column, the destination stands below its capacity, the destination holds no
// unresolved item of this card's that names it, the card carries a value for
// every field the destination requires, the departure has not reached its own
// declared loop_limit for this card, a forward departure or one into a done
// column holds no unresolved item of this card's that names it for departure,
// the destination does not wait on somebody outside the workbench, the
// destination does not reserve to the operator the claim an arriving act
// would take there, and the destination is not being retired. It reports
// whether a limit or a hold was reached and overridden, which is the flag the
// moved event carries.
//
// The loop row is Dinah's own, appended after the profile's own nine rather
// than inserted among them. dinah help move heads its table Order and
// promises the rows in the order each is checked, so a move failing the
// capacity row and any row below it answers at-capacity, and a regressive
// move into a held column that has also reached its departure's loop limit
// answers unresolved-item, which is the ninth row and the earlier of the two.
//
// regressive decides both the loop-limit row and the exit-hold row below it,
// and it decides them the same way RegressiveDepartures (dinah-542's own
// helper, in internal/bench/check.go) decides one replayed departure: a
// column standing earlier in the flow's own order, and not of kind done.
// Route position plays no part, on dinah-542/criteria/6's own rule for the
// loop limit, and a card walking a route is judged the same way here.
//
// Five of these rows are overridable: at-capacity, the entry hold, the
// missing-field row, the loop limit and the exit hold, in that order, every
// one of them gated on !req.Override and every one of them therefore a row
// only the operator can actually pass, since canRoute already refuses
// req.Override to anybody else before canLand is reached at all. Where the
// caller a row of this kind refuses is the operator, its refusal carries
// extra["operator"]="1", which is what selects the operator's own next-step
// fragment naming --override in each of the five shapes; a caller who is not
// the operator reads the refusal exactly as before, and cannot pass any of
// these rows regardless of what the message says.
//
// takesUp says whether the act this list is running for takes the card up
// where it lands, which a pull does and a move does not. The waiting row and
// the operator-owned row read it, and it is a parameter rather than a reading
// of req.Verb because it is a property of the act rather than of the word the
// caller typed.
func (l *Library) canLand(req *Request, card *bench.Card, destination, departure *bench.Column, takesUp bool) (bool, *Response, error) {
	if card.State == contract.StateBlocked {
		return false, l.refuse(req, card, contract.Blocked, card.BlockReason), nil
	}
	if card.Holder != "" && card.Holder != req.Actor {
		return false, l.refuse(req, card, contract.Held, card.Holder), nil
	}
	operator := req.Actor == l.Bench.Operator
	forward := departure != nil && destination.Position > departure.Position
	if forward && departure.Terminal() {
		return false, l.refuse(req, card, contract.Terminal, columnRef(departure)), nil
	}
	regressive := departure != nil && !destination.Terminal() && destination.Position < departure.Position
	reached, err := l.atCapacity(req, destination)
	if err != nil {
		return false, nil, err
	}
	if reached && !req.Override {
		var extra map[string]string
		if operator {
			extra = map[string]string{"operator": "1"}
		}
		return false, l.refuseWith(req, card, contract.AtCapacity, columnRef(destination), extra), nil
	}
	// CORE-GATE-3, the destination's own hold. The column declares only that
	// it holds, and which items hold there follows from which of the card's
	// items name it, so nothing below reads what kind of item it found. Every
	// kind an item can carry holds a card on the same terms, and a workbench
	// that wants one kind held at a column and not another says so by which
	// kinds it files against that column.
	//
	// The refusal names the first such item, on claimableItems's own
	// convention, because a count tells whoever is holding the card nothing
	// about what to go and settle.
	gateHeld := false
	if destination.HoldsOnEntry() {
		holding, err := l.Bench.GatingItems(card.Dir, destination.ID)
		if err != nil {
			return false, nil, err
		}
		if len(holding) > 0 {
			gateHeld = true
			if !req.Override {
				var extra map[string]string
				if operator {
					extra = map[string]string{"operator": "1"}
				}
				return false, l.refuseWith(req, card, contract.UnresolvedItem, holding[0].ID, extra), nil
			}
		}
	}
	// A column may require more of a card than room and more than a settled
	// item: it names declared field keys the card has to hold a value for
	// before it enters. The row reads the destination, so it holds on the way
	// in, which is the direction the entry hold above reads and the direction
	// an acceptance criterion already relies on. There is no exit form.
	//
	// The refusal names the first such key in the column's declaration order,
	// which is claimableItems's own convention: a count tells whoever is
	// holding the card nothing about what to go and set.
	requiredMissing := false
	if missing := l.missingRequiredField(card, destination); missing != "" {
		requiredMissing = true
		if !req.Override {
			extra := map[string]string{contract.ValueColumn: columnRef(destination)}
			if operator {
				extra["operator"] = "1"
			}
			return false, l.refuseWith(req, card, contract.MissingField, missing, extra), nil
		}
	}
	// The cap is absolute, on the operator's own ruling: an override carries
	// the one move it is passed on, the count goes on rising underneath it,
	// and the next regressive move out of the column is refused again for the
	// life of the card. Nothing here resets a count and nothing stores a
	// standing exemption.
	loopReached := false
	if departure != nil && departure.LoopLimit > 0 && regressive {
		events, _, err := l.Bench.ReadJournal(card.JournalPath())
		if err != nil {
			return false, nil, err
		}
		loopReached = l.Bench.RegressiveDepartures(events, departure.ID) >= departure.LoopLimit
		if loopReached && !req.Override {
			var extra map[string]string
			if operator {
				extra = map[string]string{"operator": "1"}
			}
			return false, l.refuseWith(req, card, contract.AtLoopLimit, columnRef(departure), extra), nil
		}
	}
	// CORE-GATE-6, Dinah's own: the departure's own exit hold, symmetric with
	// the entry row above and running immediately after the loop-limit row it
	// follows, before the three rows beneath it that ask about the
	// destination rather than about this card's own departure. It binds a
	// forward departure and one into a done column, on regressive's own
	// reading, and a regressive departure passes with the item riding along:
	// the card's items still name the station, and the station holds it
	// again on any later forward attempt, so nothing the exit hold protects
	// is escaped by sending the card back upstream.
	exitGateHeld := false
	if departure != nil && departure.HoldsOnExit() && !regressive {
		holding, err := l.Bench.GatingItems(card.Dir, departure.ID)
		if err != nil {
			return false, nil, err
		}
		if len(holding) > 0 {
			exitGateHeld = true
			if !req.Override {
				var extra map[string]string
				if operator {
					extra = map[string]string{"operator": "1"}
				}
				return false, l.refuseWith(req, card, contract.UnresolvedItemExit, holding[0].ID, extra), nil
			}
		}
	}
	// A column where no owner takes work up receives a card that arrives
	// unheld, which is the ordinary handoff, and refuses one that arrives
	// held, because a held card at such a column is work taken up where
	// nobody takes work up. The claim is not cleared here on the holder's
	// behalf: CORE-MOVE-8 says a move MUST NOT change a card's state or
	// its holder, so the holder releases and then moves. A pull is refused
	// whichever state the card it carries is in, because a pull takes the
	// card up where a move does not.
	//
	// The row stands after the capacity row and before the retiring one, so a
	// full such column answers at-capacity and a retiring one answers this
	// name. The retiring check stays last for the concurrency reason its own
	// comment gives.
	if !destination.TakesWorkUp() && (card.Holder != "" || takesUp) {
		return false, l.refuse(req, card, takesNoWorkName(destination), columnRef(destination)), nil
	}
	// An operator-owned destination reserves the claim taken there to the
	// operator, which is CORE-CLAIM-8 read at the far end of a pull rather
	// than in place. The row is gated on takesUp, so an ordinary handoff
	// stays legal for every owner: a move leaves the card unheld wherever it
	// lands, and CORE-MOVE-6 already reserves the departure a card is later
	// taken out of.
	//
	// Like the row above it, this one does not weaken for --no-claim. Pull
	// passes takesUp whichever way that option is set, because the option
	// changes what a pull writes rather than what a pull allows.
	if takesUp && operatorReservesClaim(destination, req.Actor, l.Bench.Operator) {
		return false, l.refuse(req, card, contract.NotOperator, req.Actor), nil
	}
	if holder, retiring := l.retiring(destination.ID); retiring {
		return false, l.refuse(req, card, contract.Locked, holder), nil
	}
	return (reached || loopReached || gateHeld || exitGateHeld || requiredMissing) && req.Override, nil, nil
}

// missingRequiredField names the first declared field key the destination
// requires that the card holds no value for, and the empty string where the
// card satisfies every one of them.
//
// A key the workbench does not declare is skipped rather than refused, because
// a key nothing can ever be written under would make the column unreachable;
// `dinah check` reports it under check.required-field-undeclared instead.
func (l *Library) missingRequiredField(card *bench.Card, destination *bench.Column) string {
	for _, key := range destination.RequireFields {
		if l.Bench.DeclaredFieldOf(key) == nil {
			continue
		}
		if bench.FieldValue(card.FM, key) == "" {
			return key
		}
	}
	return ""
}

// move carries a card from one column to another. The list is CORE-MOVE's, in
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
		Actor:     req.Acting(),
		From:      card.Column,
		FromTitle: titleOf(departure),
		To:        destination.ID,
		ToTitle:   destination.Title,
		Override:  override,
	}
	if departure != nil {
		if target := l.Bench.RejectTarget(departure); target != nil && target.ID == destination.ID {
			ev.Reject = true
		}
	}
	// A criterion-retirement grant is spent by the card's next move, whatever
	// the direction, and is cleared by the same act that records the move.
	// The narrowing and the tidying are one episode at one station, so the
	// departure that ends the episode ends the permission.
	//
	// No journal line of its own is written for it. The moved event above
	// records the departure that spent the grant, and a second line saying
	// the same thing in other words is a fact a reader has to reconcile
	// rather than one they gain.
	card.RetirementGrant = ""
	card.Column = destination.ID
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	// The destination's standing items are minted after the moved line,
	// under the lock this act already holds, and after every refusal above
	// has passed, so a move an override carried in still mints and a
	// refused move mints nothing.
	if err := l.mintStandingItems(req, card, destination, ev.TS); err != nil {
		return l.FromError(req, err)
	}
	response.Instructions, response.ChainServed, err = l.serve(req, card)
	if err != nil {
		return l.FromError(req, err)
	}
	response.LegalMoves = l.legalMoves(card)
	loop, err := l.cardLoop(card)
	if err != nil {
		return l.FromError(req, err)
	}
	response.Loop = loop
	return response
}

// titleOf names a column that may be absent, which a card carrying a column the
// bench no longer declares makes possible.
func titleOf(column *bench.Column) string {
	if column == nil {
		return ""
	}
	return column.Title
}

// atCapacity reports whether a column has reached its declared limit. The
// count is every live card in the column whatever its state, because a
// blocked card still occupies the place.
//
// A request carrying an occupancy answers from it, which is how
// MoveDestinations asks the question of every destination while reading the
// cards once. Every other request carries none, and the count is taken here.
// Either way the count is compared against the declared capacity once, on
// the last line.
func (l *Library) atCapacity(req *Request, column *bench.Column) (bool, error) {
	if column.Capacity <= 0 {
		return false, nil
	}
	count := 0
	if req != nil && req.occupancy != nil {
		count = req.occupancy[column.ID]
	} else {
		cards, err := l.Bench.Cards()
		if err != nil {
			return false, err
		}
		for _, card := range cards {
			if card.Column == column.ID {
				count++
			}
		}
	}
	return count >= column.Capacity, nil
}

// release gives a card back. The list is CORE-RELEASE's, and it lives in
// canRelease so that OfferActs asks the same rows the act runs.
func (l *Library) release(req *Request, card *bench.Card) *Response {
	if refusal := l.canRelease(req, card); refusal != nil {
		return refusal
	}
	card.State = contract.StateReady
	card.Holder = ""
	card.ClaimSince = ""
	card.Expires = ""
	ev := bench.Event{
		TS:    bench.Stamp(l.Now()),
		Event: contract.EventReleased,
		Actor: req.Acting(),
	}
	response, err := l.commit(req, card, ev)
	if err != nil {
		return l.FromError(req, err)
	}
	return response
}

// canRelease runs CORE-RELEASE's two rows, in its order: the request names an
// owner, and the owner asking holds the card. It answers the refusal of the
// first row that fails and nil when the card may be released. release and
// OfferActs both call it, so a row added here reaches the act and the offer
// together.
func (l *Library) canRelease(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	if card.Holder != req.Actor {
		return l.refuse(req, card, contract.NotHolder, card.Holder)
	}
	return nil
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
	card.State = contract.StateBlocked
	card.Holder = ""
	card.ClaimSince = ""
	card.Expires = ""
	card.BlockReason = req.Reason
	card.BlockKind = req.Kind
	card.BlockSince = now
	ev := bench.Event{
		TS:     now,
		Event:  contract.EventBlocked,
		Actor:  req.Acting(),
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
// CORE-UNBLOCK's, with the operator check evaluated ahead of the state
// check, so an owner who is not the operator is refused not-operator whatever
// the card's state.
func (l *Library) unblock(req *Request, card *bench.Card) *Response {
	if req.Actor == "" {
		return l.refuse(req, card, contract.NoOwner, "")
	}
	if req.Actor != l.Bench.Operator {
		return l.refuse(req, card, contract.NotOperator, req.Actor)
	}
	if card.State != contract.StateBlocked {
		return l.refuse(req, card, contract.NotBlocked, card.State)
	}
	now := bench.Stamp(l.Now())
	card.State = contract.StateReady
	card.BlockReason = ""
	card.BlockKind = ""
	card.BlockSince = ""
	ev := bench.Event{
		TS:    now,
		Event: contract.EventUnblocked,
		Actor: req.Acting(),
	}
	// A reason is optional (CORE-UNBLOCK-5), and one that is given is written
	// twice under the lock Do already holds: as a comment on the card, whose
	// author is the actor of the lift, and as the reason of the unblocked
	// line, which also names the comment. The trimmed text is what both
	// stores carry, and whitespace alone is the bare lift. The four writes
	// run in a fixed order, comment anchor, card anchor, commented line,
	// unblocked line, and nothing makes them one transaction, so the order is
	// what fixes which partial states an interruption can leave.
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		response, err := l.commit(req, card, ev)
		if err != nil {
			return l.FromError(req, err)
		}
		return response
	}
	comment, err := l.Bench.AddComment(card.Dir, req.Actor, now, reason)
	if err != nil {
		return l.FromError(req, err)
	}
	commented := bench.Event{
		TS:      now,
		Event:   contract.EventCommented,
		Actor:   req.Acting(),
		Comment: comment.ID,
	}
	ev.Reason = reason
	ev.Comment = comment.ID
	response, err := l.commit(req, card, commented, ev)
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
	workstream, err := l.Bench.WorkstreamByRef(req.Workstream)
	if err != nil {
		return l.FromError(req, err)
	}
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
		Actor:      req.Acting(),
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
	workstream, err := l.Bench.WorkstreamByRef(req.Workstream)
	if err != nil {
		return l.FromError(req, err)
	}
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
		Actor:      req.Acting(),
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
//
// The witness runs on the fresh copy before the lapse is reconsidered, so a
// hand-edited position is recorded whether or not the reread still finds the
// claim lapsed. The actor is whoever ran the read, which is the same
// whoever-noticed-it basis the write verbs use; the lock keeps its own empty
// actor, since that is an advisory lock rather than a claim.
func (l *Library) lapseRead(card *bench.Card, actor string) error {
	if !card.Lapsed(l.Now()) {
		return nil
	}
	if l.ReadOnly {
		return ErrReadOnly
	}
	lock, err := l.Bench.Acquire(card.Dir, "", bench.Stamp(l.Now()))
	if err != nil {
		return nil
	}
	defer lock.Release()
	fresh, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), card.ID)
	if err != nil {
		return nil
	}
	if _, err := l.Bench.WitnessDivergence(actor, bench.Stamp(l.Now()), fresh); err != nil {
		return err
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
