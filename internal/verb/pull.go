package verb

import (
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Pull is the one-command route that combines a claim and a move. It fixes
// the destination column, takes the card at the head of that column's upstream
// ready queue, and writes the claim and the move in one card.Save and one
// pair of journal appends.
//
// Pull is not a contract verb and is not dispatched through Do, because the
// bare form carries no card reference and the named form's card is chosen
// from the destination's upstream queue rather than typed. Pull runs its own
// transaction. The workbench pair and the invocation's own refusals are
// raised here, before any column is considered and before any lock is taken,
// because each of them answers the same for every column on the workbench and
// folding them into the qualifying predicate would turn a refusal into an
// empty answer. The column-scoped refusals narrow the qualifying set instead,
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
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	if req.Actor == "" {
		return l.refuse(req, nil, contract.NoOwner, "")
	}
	// The unknown-column row belongs to the named form alone, since the bare
	// form names no column to be unknown, and it is settled ahead of the
	// override row because that is the order the table declares and the
	// order the generated help prints.
	var named *bench.Column
	if req.Column != "" {
		named = l.Bench.ColumnByRef(req.Column)
		if named == nil {
			return l.refuse(req, nil, contract.UnknownColumn, req.Column)
		}
	}
	if req.Override && req.Actor != l.Bench.Operator {
		return l.refuse(req, nil, contract.NotOperator, req.Actor)
	}
	destination, answer, err := l.pullDestination(req, named)
	if err != nil {
		return l.FromError(req, err)
	}
	if answer != nil {
		return answer
	}
	if destination == nil {
		return l.okEmpty(req, nil)
	}
	upstream := upstreamOf(destination, l.Bench.Columns)
	if upstream == nil {
		detail := columnRef(destination)
		return l.refuseWith(req, nil, contract.NoUpstream, detail, map[string]string{"column": detail})
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return l.FromError(req, err)
	}
	// The immediate upstream is tried first and on its own terms, so every
	// refusal it owes the caller under the lock is still raised: a done
	// upstream still answers terminal and one waiting on somebody outside
	// still answers its own name. What it now applies is the route filter, so
	// a card whose own road carries it somewhere else is left standing rather
	// than taken into a station its route does not reach. Only when it holds
	// no such card does the pull look further back, through the columns that
	// carry into this destination, nearest first.
	by := selectionAdmission(l.Bench, req)
	head, _, sawReady, _ := headOfReadyFor(l.Bench, upstream.ID, l.immediateLanding(upstream, destination), cards, by)
	if head == nil {
		// The further walk does not reconsider the immediate upstream, which
		// is what the source predicate excludes it for. The predicate carries
		// no reading of the caller's identity: the named form does not skip a
		// source the workbench reserves to its operator, so the card standing
		// there is selected and the lock answers not-operator, rather than the
		// caller being left with the empty answer for a card the board is
		// showing them.
		further, furtherReady := l.pullableCards(destination, cards, by, func(source *bench.Column) bool {
			return source.ID != upstream.ID
		})
		sawReady = sawReady || furtherReady
		if len(further) > 0 {
			head = further[0]
		}
	}
	if head == nil {
		// Finding nothing to take has two causes and they are different
		// answers. Either no source holds a ready card, or one does and the
		// caller's resolved tier is admitted for none of it, which is work
		// waiting on a more senior caller rather than an empty workbench.
		if sawReady {
			return l.okAboveTier(req, destination)
		}
		return l.okEmpty(req, destination)
	}
	req.Card = head.Ref(l.Bench.Slug)
	req.Column = columnRef(destination)
	// Pull fills in no basis of its own. The revision read during selection
	// is not a revision the caller read, and standing it in as one turns
	// losing the race for a card into `stale`, which tells a person the card
	// moved since they read it about a card they never read. Losing the race
	// is what rows 9 and 10 answer, under the lock, in the words claim uses.
	return l.pullTransaction(req, head)
}

// pullDestination fixes the column a pull will land its card in. A named form
// resolves to the column the caller typed, which Pull has already found. A
// bare form runs the qualifying predicate over the flow and answers with the
// one column that qualifies, a refusal when more than one does, the
// above-tier answer when none qualifies and the predicate saw ready work the
// declared tier is admitted for nowhere, or a nil column and no answer when
// none qualifies for any other reason, which Pull turns into the empty
// answer.
//
// The retiring row is left to the inner pull for the named form, so that both
// forms reach it through canMove and neither carries a second copy of the
// test. The predicate reads it for the bare form because a retiring column is
// one a pull could not land in, so leaving it in the qualifying set would
// make the bare form ambiguous where a reader would say it plainly is not.
func (l *Library) pullDestination(req *Request, named *bench.Column) (*bench.Column, *Response, error) {
	if named != nil {
		return named, nil, nil
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, nil, err
	}
	qualifying, aboveTier := l.pullCandidates(req, cards)
	if len(qualifying) > 1 {
		carried := map[string]string{"columns": strings.Join(qualifying, "\n")}
		return nil, l.refuseWith(req, nil, contract.AmbiguousColumn, "", carried), nil
	}
	if len(qualifying) == 1 {
		return l.Bench.ColumnByRef(qualifying[0]), nil, nil
	}
	if aboveTier {
		return nil, l.okAboveTier(req, nil), nil
	}
	return nil, nil, nil
}

// pullCandidates returns the reference of every column this invocation could
// pull into, in flow order, so the sentence the ambiguous refusal prints
// reads as the workbench reads.
//
// The conditions are the column-scoped rows of the precondition list. A column
// qualifies when at least one column carries into it and holds a ready card,
// where carrying into it is carriesInto's answer and covers on its own the
// rows a hand-written list used to spell out: a pull may not take a card out
// of a done column or out of one waiting on somebody outside, and a pull never
// lands a card where no owner takes work up. The source is not
// operator-owned unless the owner asking is the operator, which narrows the
// set rather than refusing, because the bare form enumerates destinations and
// one the caller cannot reach makes a worse answer than a shorter list. The
// destination stands below its capacity limit, or the invocation carries the
// override marker, whose legality Pull has already settled. The destination is
// not operator-owned unless the owner asking is the operator, which is the
// same reservation read at the other end of the pull: a pull lands holding
// the card, so a destination reserved to the operator is one this caller
// cannot reach, and it is narrowed away for the same reason the source is.
// The destination is not being retired.
//
// One canLand row is deliberately absent here. canLand refuses a regressive
// departure out of a column that has reached its own declared loop_limit, and
// this predicate does not reproduce it, because pullSources only ever offers
// a destination standing ahead of the source, so a pull never lands a card
// backward and the row can never hold for one. A pull shape that does land a
// card backward makes this predicate wrong, and the fix then is to read the
// source card's regressive-departure count here rather than to leave the
// divergence to be found through a pull that predicts a destination the lock
// refuses.
//
// The predicate reads the workbench without holding any lock, so the set it
// returns is a prediction about what a pull would do. The authoritative
// sequence runs under the card's lock once the destination is fixed, and when
// the two disagree, because the workbench changed in between, the lock's
// answer is the one the caller is given.
func (l *Library) pullCandidates(req *Request, cards []*bench.Card) ([]string, bool) {
	operator := req.Actor == l.Bench.Operator
	by := selectionAdmission(l.Bench, req)
	var qualifying []string
	aboveTier := false
	for _, column := range l.Bench.Columns {
		// The bare form skips a source column the workbench reserves to its
		// operator for a caller who is not the operator, so it never nominates
		// a destination that caller could not have pulled into. The named form
		// holds the opposite policy, which is why the policy travels as the
		// caller's own predicate rather than living inside the helper.
		taken, gated := l.pullableCards(column, cards, by, func(source *bench.Column) bool {
			return operator || !source.OperatorOwned
		})
		ready := len(taken) > 0
		// The tier observation is kept only for a column clearing every other
		// row of the list, so the aggregate never reports tier-gated work
		// standing somewhere this caller could not have pulled into for a
		// reason that has nothing to do with tier. A column at its capacity
		// limit, one reserved to the operator, or one being retired is a
		// column the caller was never going to reach, and saying "above your
		// tier" about it would name the wrong obstacle.
		reached, err := l.atCapacity(column)
		if err == nil && reached && !req.Override {
			continue
		}
		if operatorReservesClaim(column, req.Actor, l.Bench.Operator) {
			continue
		}
		if _, retiring := l.retiring(column.ID); retiring {
			continue
		}
		if gated {
			aboveTier = true
		}
		if !ready {
			continue
		}
		qualifying = append(qualifying, columnRef(column))
	}
	return qualifying, aboveTier
}

// pullableCards returns the ready cards a pull into this destination may take,
// nearest source first and in arrival order within a source, and reports
// whether any source held ready work this caller's declared tier is admitted
// for nowhere.
//
// A card qualifies when carriesInto, read against that card's own route,
// answers this destination. The set is keyed on the card rather than on the
// column because two cards standing in one column walk two routes and carry
// into two destinations, which is the one thing about a pull that routes
// change. That is also why the flow-derived source set it replaces could not
// survive: a route can make a column carry into a destination it does not carry
// into on the full list, so a filter over the old set would silently drop
// cards.
//
// Nearest is measured by the source column's Position in the flow, descending.
// A route is a subsequence of the flow, so flow order and route order agree for
// any one card, and measuring against the flow gives one total order across
// cards whose routes differ.
//
// fromSource narrows which columns are walked at all, and nil walks every
// column. It exists because the two forms of the verb hold opposite policies
// about a source column the workbench reserves to its operator, and the policy
// belongs to the caller that wants it rather than to this function. Pull, the
// named form, passes a predicate that excludes the immediate upstream it has
// already tried and reads the caller's identity not at all; pullCandidates, the
// bare form, passes one refusing an operator-owned column to a caller who is
// not the operator, because a destination that caller could not have pulled
// into is one it must not nominate.
//
// The predicate runs before the above-tier observation is taken, so a
// tier-gated card in a column the predicate excluded does not report work
// standing above the caller, which is the order the walk it replaces ran its
// two tests in.
func (l *Library) pullableCards(destination *bench.Column, cards []*bench.Card, by admission, fromSource func(*bench.Column) bool) ([]*bench.Card, bool) {
	landing := func(card *bench.Card) *bench.Column {
		if carriesInto(l.Bench.Column(card.Column), l.Bench.RouteOf(card)) == destination {
			return destination
		}
		return nil
	}
	var taken []*bench.Card
	aboveTier := false
	// Iterating the flow backward is what puts the nearest source first.
	for i := len(l.Bench.Columns) - 1; i >= 0; i-- {
		source := l.Bench.Columns[i]
		if fromSource != nil && !fromSource(source) {
			continue
		}
		head, _, sawReady, _ := headOfReadyFor(l.Bench, source.ID, landing, cards, by)
		if head != nil {
			taken = append(taken, head)
			continue
		}
		// The walk carries on past a source holding only tier-gated work, so a
		// nearer gated column cannot hide an eligible card further back.
		if sawReady {
			aboveTier = true
		}
	}
	return taken, aboveTier
}

// immediateLanding is the landing function the named form's first step reads at
// the destination's immediate flow upstream, and its doc comment is where the
// whole rule for which card a named pull may take is written down.
//
// The rule. A named pull into a column D may take a ready card C standing at a
// column S when either of two things holds:
//
//  1. C's own road carries it from S into D: carriesInto(S, road of C) is D.
//     This is the only way a card at a further source is taken, and it is the
//     walk pullableCards runs.
//  2. S is D's immediate upstream in the flow, and C's road gives the same
//     answer at S that the whole flow gives there: carriesInto(S, road of C)
//     equals carriesInto(S, flow). When both answer D this is case 1 again.
//     When both answer nothing, S is a done column, a column waiting on
//     somebody outside, or a station whose flow successor D takes no work up,
//     and the card is taken so that the lock refuses it by name, which is what
//     every workbench did before routes existed.
//
// Nothing else is taken. A road that answers at S differently from the flow
// is a road that does not send C into D the way the flow would, whether it
// names another station, runs through a queue the flow does not have there,
// or does not carry S at all because C stands off its road; in every such
// position the card is left standing. A card walking the default route has
// the flow for its road, so both answers are always the same and the first
// step selects exactly as it did before routes existed.
//
// Section 4.4 of dinah-542's specification compared twenty-one behaviours and
// missed two positions this rule covers: a card at S whose road puts a queue
// after S, and a card standing off its road at S. Code review found both.
func (l *Library) immediateLanding(upstream, destination *bench.Column) landingFor {
	flow := carriesInto(upstream, l.Bench.Columns)
	return func(card *bench.Card) *bench.Column {
		if carriesInto(upstream, l.Bench.RouteOf(card)) != flow {
			return nil
		}
		return destination
	}
}

// okEmpty answers a pull that found nothing to take, at exit 0 with no card
// and nothing written to any journal. The named form answers this way when
// the destination's upstream column holds no ready card, and the bare form
// answers it when no column qualifies at all, because the two are one
// condition and answering them differently would be two vocabularies for one
// fact.
func (l *Library) okEmpty(req *Request, destination *bench.Column) *Response {
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
		"upstream":    upstreamTitle(destination, l.Bench.Columns),
		"destination": destination.Title,
	}
	return response
}

// okAboveTier answers a pull that found ready work it could not take because
// the declared tier is admitted for none of it, at exit 0 with no card and
// nothing written to any journal, exactly as okEmpty answers nothing ready at
// all. The two carry separate messages rather than one message and a flag,
// because a caller reading Message alone, which is what an MCP client and a
// --json caller already branch on for okEmpty, has to be able to tell that
// there is genuinely nothing from that there is something and it is not for
// them, without inspecting anything else.
//
// What the declared tier is remains self-reported and unverified, here as on
// a claim. This answer reports what the caller said about itself and settles
// nothing about what the caller is.
func (l *Library) okAboveTier(req *Request, destination *bench.Column) *Response {
	response := &Response{
		Outcome:     contract.OutcomeOK,
		Verb:        req.Verb,
		Affordances: l.affordances(nil),
	}
	if destination == nil {
		response.Message = "answer.pull.above-tier.bare"
		return response
	}
	response.Message = "answer.pull.above-tier.named"
	response.MessageValues = map[string]string{
		"upstream":    upstreamTitle(destination, l.Bench.Columns),
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
	card, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), head.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	if err := l.lapse(card); err != nil {
		return l.FromError(req, err)
	}
	if req.Basis != "" && req.Basis != card.Revision {
		view, err := l.view(card)
		if err != nil {
			return l.FromError(req, err)
		}
		return &Response{
			Outcome:     contract.OutcomeStale,
			Verb:        req.Verb,
			Card:        view,
			Basis:       req.Basis,
			Affordances: l.affordances(card),
		}
	}
	if _, err := l.Bench.WitnessDivergence(req.Actor, bench.Stamp(l.Now()), card); err != nil {
		return l.FromError(req, err)
	}
	return l.pull(req, card)
}

// pull is the inner write, reached once the card has been chosen and locked.
// It evaluates rows 8 to 16 of pull's table in the order the table declares,
// out of the same two functions move and claim run, so a pull refuses in the
// words a move and a claim already refuse in.
//
// canRoute is the move's own list up to and including the departure row,
// which is row 8. claimableState is the claim's own state pair, rows 9
// and 10, and it stands between the halves because row 10 takes claim's
// stricter test: a pull that claims cannot take a card already active
// whoever holds it, where the move's row admits a card the owner asking
// holds. canLand is the rest of the move's list, rows 11 to 14, which now
// include the departure's own exit hold between the entry gate and the
// operator-owned reservation. Its own blocked and held rows are reached with
// the state already settled by the pair above, so neither can decide a pull.
//
// claimableState runs whether or not the caller passed --no-claim, because
// the option changes what pull writes and not what pull allows, and
// pullableDeparture behind it reads the column the card is leaving for the
// same reason.
//
// canLand is told this act takes the card up, which is what refuses a pull
// landing a card where no owner takes work up and what refuses a pull landing
// a card at a column that reserves the claim to the operator. The claim a pull
// writes is taken at the destination, so the departure is asked the narrower
// question pullableDeparture asks: whether a pull may take a card out of it
// at all.
//
// Pull's caller has already fixed req.Column to the destination's reference,
// so canRoute resolves the destination exactly as the named form of a move
// would. Rows 3 to 5 run again here, harmlessly, because they are the front
// of the move's list and answering them twice cannot change an answer.
func (l *Library) pull(req *Request, card *bench.Card) *Response {
	destination, departure, refusal := l.canRoute(req, card)
	if refusal != nil {
		return refusal
	}
	if refusal := l.claimableState(req, card); refusal != nil {
		return refusal
	}
	if refusal := l.pullableDeparture(req, card, departure); refusal != nil {
		return refusal
	}
	override, refusal, err := l.canLand(req, card, destination, departure, true)
	if err != nil {
		return l.FromError(req, err)
	}
	if refusal != nil {
		return refusal
	}
	// A pull that takes the card up is a claim, so it answers CORE-CLAIM-10
	// as a plain claim does. The rule reads the card's items against the
	// columns the workbench declares and never the card's position, so asking
	// it here, where the card has left one column and not yet landed at
	// another, gets the same answer either column would have given. A
	// --no-claim pull leaves the card ready and takes nothing up, so nothing
	// about an unresolved item refuses it.
	if !req.NoClaim {
		if refusal := l.claimableItems(req, card); refusal != nil {
			return refusal
		}
		// Row 16, Dinah's own, is asked about the destination rather than the
		// column the card is leaving, because that is where the claim is
		// taken. A --no-claim pull takes nothing up, so no requirement the
		// card carries can refuse it, exactly as no unresolved item can.
		if refusal := l.claimableTier(req, card, destination); refusal != nil {
			return refusal
		}
	}
	now := l.Now()
	stamp := bench.Stamp(now)
	events := make([]bench.Event, 0, 2)
	if req.NoClaim {
		// A pull that does not claim leaves the card in the ready state
		// an ordinary move leaves, so the next owner to claim or to pull it
		// onward takes it from there.
		card.State = contract.StateReady
		card.Holder = ""
		card.ClaimSince = ""
		card.Expires = ""
	} else {
		card.State = contract.StateActive
		card.Holder = req.Actor
		card.ClaimSince = stamp
		if req.Expires > 0 {
			card.Expires = bench.Stamp(now.Add(req.Expires))
		}
		events = append(events, bench.Event{
			TS:      stamp,
			Event:   contract.EventClaimed,
			Actor:   req.Acting(),
			Expires: card.Expires,
		})
	}
	// The departure identifier is read before the card is carried, because
	// the moved event names where the card came from and the assignment
	// below overwrites it. move reads it ahead of the same assignment for
	// the same reason.
	from := card.Column
	card.Column = destination.ID
	events = append(events, bench.Event{
		TS:        stamp,
		Event:     contract.EventMoved,
		Actor:     req.Acting(),
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
	response.Instructions, response.ChainServed, err = l.serve(req, card)
	if err != nil {
		return l.FromError(req, err)
	}
	response.LegalMoves = l.legalMoves(card)
	return response
}

// pullableDeparture is the row that reads the column a pull is taking the card
// out of. A pull may not take a card out of a column waiting on somebody
// outside the workbench, because the card leaves there when that person
// answers rather than when somebody pulls, and the refusal names the flag so
// the sentence can say who is being waited on.
//
// A departure where no owner takes work up by kind is not refused here. That
// is what a buffer and an intake column are for: nobody works the card where
// it stands, so a pull is the act that carries it on.
//
// The row decides on PullCanTakeFrom and reads the flags only to pick the
// name, on the pattern takesNoWorkName sets for the claim path. Reading
// AwaitingOutside to decide would leave any later reason for a pull not to
// take from a column sitting in the predicate and never reaching this row,
// which is the second-answer defect this card exists to close.
func (l *Library) pullableDeparture(req *Request, card *bench.Card, departure *bench.Column) *Response {
	if departure == nil || departure.PullCanTakeFrom() {
		return nil
	}
	name := pullDepartureName(departure)
	if name == "" {
		return nil
	}
	return l.refuse(req, card, name, columnRef(departure))
}

// pullDepartureName picks the refusal name for a column a pull may not take a
// card out of, and returns the empty string for a departure another row of the
// pull's list answers.
//
// A terminal departure is one of those: canLand's terminal row refuses the
// forward move a pull makes and names it after CORE-STATE-9, so repeating that
// refusal here would put the same rule in two places and could answer it by
// the wrong name. Every other reason names the flag, which is what lets the
// sentence say who is being waited on.
func pullDepartureName(column *bench.Column) string {
	if column.Terminal() {
		return ""
	}
	return contract.AwaitingOutside
}

// upstreamOf returns the column standing immediately before the given column
// along a road, or nil where that column stands first on it and nothing
// precedes it, and nil where the road does not carry the column at all.
//
// The road is a card's own route, and the workbench's whole ordered column list
// is the road a card walking the default route travels, so a caller asking
// about the flow passes it. The index comes from RouteIndexOf rather than from
// Column.Position: Position is a dense index over the flow, a route is a
// subsequence of it, and indexing a route slice by Position reads the wrong
// column wherever the route has dropped one.
func upstreamOf(column *bench.Column, route []*bench.Column) *bench.Column {
	at := bench.RouteIndexOf(route, column)
	if at <= 0 {
		return nil
	}
	return route[at-1]
}

// downstreamOf returns the column after the given one along a road, or nil
// where the given column stands last on it or the road does not carry it. It
// indexes the road exactly as upstreamOf does and for the same reason.
func downstreamOf(column *bench.Column, route []*bench.Column) *bench.Column {
	at := bench.RouteIndexOf(route, column)
	if at < 0 || at+1 >= len(route) {
		return nil
	}
	return route[at+1]
}

// carriesInto returns the column a pull would carry a card standing at the
// given column into, along the road that card walks, or nil when no pull could
// carry it anywhere.
//
// The road is the card's own route, and the whole ordered column list is the
// road of a card walking the default route. A column the road does not carry at
// all answers nil, because the road has no column beyond it to name.
//
// A card standing at a station is taken from where it stands, so the column
// beyond it is where a pull puts it or there is no pull to make. A card
// standing where nobody takes work up is carried on instead, because a
// queue is a place to wait rather than a place to arrive, and the walk ends
// at the first column where an owner takes the card up.
//
// Two columns end the walk without answering. A pull may not take a card out
// of a done column or out of one waiting on somebody outside, so neither
// carries a card through either. An operator-owned queue in the middle of a
// run ends it too, whoever is asking, because carrying a card past that
// column without its owner acting is what the column exists to prevent.
func carriesInto(column *bench.Column, route []*bench.Column) *bench.Column {
	if column == nil || !column.PullCanTakeFrom() {
		return nil
	}
	beyond := downstreamOf(column, route)
	if column.TakesWorkUp() {
		if beyond != nil && beyond.TakesWorkUp() {
			return beyond
		}
		return nil
	}
	for ; beyond != nil; beyond = downstreamOf(beyond, route) {
		if beyond.TakesWorkUp() {
			return beyond
		}
		if !beyond.PullCanTakeFrom() || beyond.OperatorOwned {
			return nil
		}
	}
	return nil
}

// placementDisrupts returns the column, first in flow order, of the first live
// card whose own pull destination a column-new placement would change, or nil
// when the placement changes no live card's destination.
//
// It iterates the live cards rather than the occupied columns, because two
// cards standing in one column walk two routes and carry into two
// destinations, so the question is a card's and not a column's. A placement
// changing the answer for a card on one route and not for a card on another is
// a disruption, because one live card is enough.
//
// columns is the flow as it stands, read under the workbench lock by the
// caller. insertAt is where the new column would land, which is len(columns)
// for a bare append and an existing column's Position for --before. kind is
// the new column's own kind, already defaulted by the caller exactly as
// bench.NewColumn defaults it, because an unresolved default would make this
// answer disagree with what is about to be written. cards is every live card
// in the workbench, read once by the caller and passed in.
//
// The comparison never mutates columns. It builds a second slice carrying
// every column the placement leaves undisturbed at its old Position, a
// placeholder at insertAt standing in for the column to be, and a clone,
// never the original pointer, of every column the placement pushes later,
// each clone's Position advanced by one to match its new index. downstreamOf
// indexes the slice by each column's own Position, so a shifted column
// compared against a slice still carrying its old Position would silently
// ask the wrong question, and the clone is what keeps the comparison honest.
// placementID stands for the column a placement would create, in the
// comparison placementDisrupts runs before that column exists. A column's own
// identifier is twelve hexadecimal characters, so this value can never name a
// column the workbench carries, and an answer of nil has to be told apart from
// an answer of the column-to-be rather than sharing an empty identifier with
// it.
const placementID = "the placement"

func placementDisrupts(b *bench.Bench, insertAt int, kind string, cards []*bench.Card) *bench.Column {
	columns := b.Columns
	placeholder := &bench.Column{ID: placementID, Kind: kind, Position: insertAt}
	after := make([]*bench.Column, 0, len(columns)+1)
	after = append(after, columns[:insertAt]...)
	after = append(after, placeholder)
	for i := insertAt; i < len(columns); i++ {
		clone := *columns[i]
		clone.Position = i + 1
		after = append(after, &clone)
	}
	moved := make(map[string]*bench.Column, len(after))
	for _, column := range after {
		moved[column.ID] = column
	}
	standing := make(map[string][]*bench.Card, len(columns))
	for _, card := range cards {
		standing[card.Column] = append(standing[card.Column], card)
	}
	for _, existing := range columns {
		for _, card := range standing[existing.ID] {
			// Each card's road is resolved twice, once against the flow as it
			// stands and once against the flow the placement would produce, so
			// a route naming a column the placement pushes later is read in
			// the order each comparison is about.
			was := routeThrough(b, card, columns)
			becomes := routeThrough(b, card, after)
			want, got := carriesInto(existing, was), carriesInto(moved[existing.ID], becomes)
			wantID, gotID := "", ""
			if want != nil {
				wantID = want.ID
			}
			if got != nil {
				gotID = got.ID
			}
			if wantID != gotID {
				return existing
			}
		}
	}
	return nil
}

// routeThrough resolves one card's road against an explicit ordered column
// list, which is what lets the placement comparison ask the route question of a
// flow that does not exist yet. A card walking the default route walks the list
// it is given, whatever that list is.
func routeThrough(b *bench.Bench, card *bench.Card, columns []*bench.Column) []*bench.Column {
	if card == nil || card.Route == "" {
		return columns
	}
	ids, declared := b.Routes[card.Route]
	if !declared {
		return columns
	}
	return bench.RouteColumnsIn(ids, columns)
}

// upstreamTitle names the upstream column for the sentence the named form's
// empty answer prints, and is empty when the column stands first in the flow.
func upstreamTitle(destination *bench.Column, columns []*bench.Column) string {
	upstream := upstreamOf(destination, columns)
	if upstream == nil {
		return ""
	}
	return upstream.Title
}
