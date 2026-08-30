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
	// still answers its own name. Only when it holds no ready card does the
	// pull look further back, through the columns that carry into this
	// destination, nearest first.
	head := headOfReady(upstream.ID, cards)
	if head == nil {
		head = headOfFurtherSource(destination, upstream, l.Bench.Columns, cards)
	}
	if head == nil {
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
// one column that qualifies, a refusal when more than one does, or a nil column
// when none does, which Pull turns into the empty answer.
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
	qualifying := l.pullCandidates(req, cards)
	if len(qualifying) > 1 {
		carried := map[string]string{"columns": strings.Join(qualifying, "\n")}
		return nil, l.refuseWith(req, nil, contract.AmbiguousColumn, "", carried), nil
	}
	if len(qualifying) == 1 {
		return l.Bench.ColumnByRef(qualifying[0]), nil, nil
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
// The predicate reads the workbench without holding any lock, so the set it
// returns is a prediction about what a pull would do. The authoritative
// sequence runs under the card's lock once the destination is fixed, and when
// the two disagree, because the workbench changed in between, the lock's
// answer is the one the caller is given.
func (l *Library) pullCandidates(req *Request, cards []*bench.Card) []string {
	operator := req.Actor == l.Bench.Operator
	var qualifying []string
	for _, column := range l.Bench.Columns {
		if !l.someSourceIsReady(column, cards, operator) {
			continue
		}
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
		qualifying = append(qualifying, columnRef(column))
	}
	return qualifying
}

// someSourceIsReady reports whether any column a pull into this destination
// could take a card from holds a ready card the asking owner may take.
func (l *Library) someSourceIsReady(destination *bench.Column, cards []*bench.Card, operator bool) bool {
	for _, source := range pullSources(destination, l.Bench.Columns) {
		if source.OperatorOwned && !operator {
			continue
		}
		if headOfReady(source.ID, cards) != nil {
			return true
		}
	}
	return false
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
	if _, err := l.Bench.WitnessDivergence(req.Actor, bench.Stamp(l.Now()), card); err != nil {
		return l.FromError(req, err)
	}
	return l.pull(req, card)
}

// pull is the inner write, reached once the card has been chosen and locked.
// It evaluates rows 8 to 14 of pull's table in the order the table declares,
// out of the same two functions move and claim run, so a pull refuses in the
// words a move and a claim already refuse in.
//
// canRoute is the move's own list up to and including the departure row,
// which is row 8. claimableState is the claim's own state pair, rows 9
// and 10, and it stands between the halves because row 10 takes claim's
// stricter test: a pull that claims cannot take a card already active
// whoever holds it, where the move's row admits a card the owner asking
// holds. canLand is the rest of the move's list, rows 11 to 14. Its own
// blocked and held rows are reached with the state already settled by the
// pair above, so neither can decide a pull.
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
			Actor:   req.Actor,
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

// headOfFurtherSource returns the card a pull takes when the destination's
// immediate upstream holds none: the head of the nearest column that carries
// into this destination and holds a ready card. It reads the caller not at
// all, exactly as the immediate-upstream step does, so a source the caller
// may not move a card out of answers not-operator under the lock rather than
// leaving the caller with the empty answer for a card the board shows them.
func headOfFurtherSource(destination, upstream *bench.Column, columns []*bench.Column, cards []*bench.Card) *bench.Card {
	for _, source := range pullSources(destination, columns) {
		if source == upstream {
			continue
		}
		if head := headOfReady(source.ID, cards); head != nil {
			return head
		}
	}
	return nil
}

// upstreamOf returns the column standing immediately before the given column in
// the declared flow, or nil when that column stands first and nothing precedes
// it. Position is a dense index over the live columns, so the predecessor is
// the row before it.
func upstreamOf(column *bench.Column, columns []*bench.Column) *bench.Column {
	if column == nil || column.Position == 0 {
		return nil
	}
	return columns[column.Position-1]
}

// downstreamOf returns the column after the given one in the flow, or nil
// when the given column stands last. Position is a dense index over the live
// columns, so the successor is the row after it.
func downstreamOf(column *bench.Column, columns []*bench.Column) *bench.Column {
	if column == nil || column.Position+1 >= len(columns) {
		return nil
	}
	return columns[column.Position+1]
}

// carriesInto returns the column a pull would carry a card standing at the
// given column into, or nil when no pull could carry it anywhere.
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
func carriesInto(column *bench.Column, columns []*bench.Column) *bench.Column {
	if column == nil || !column.PullCanTakeFrom() {
		return nil
	}
	beyond := downstreamOf(column, columns)
	if column.TakesWorkUp() {
		if beyond != nil && beyond.TakesWorkUp() {
			return beyond
		}
		return nil
	}
	for ; beyond != nil; beyond = downstreamOf(beyond, columns) {
		if beyond.TakesWorkUp() {
			return beyond
		}
		if !beyond.PullCanTakeFrom() || beyond.OperatorOwned {
			return nil
		}
	}
	return nil
}

// pullSources returns the columns a pull into the given destination may take a
// card from, nearest first. It states no rule of its own. It asks carriesInto
// about each column in the flow and keeps the ones that answer this
// destination, so carriesInto stays the only place the rule is written.
//
// Iterating the flow backward is what puts the nearest source first. The run
// it returns happens to be contiguous, nothing here relies on that, and a
// caller must not walk back from the destination on the strength of it,
// because the walk and this filter disagree about an operator-owned column.
func pullSources(destination *bench.Column, columns []*bench.Column) []*bench.Column {
	var sources []*bench.Column
	for i := len(columns) - 1; i >= 0; i-- {
		if carriesInto(columns[i], columns) == destination {
			sources = append(sources, columns[i])
		}
	}
	return sources
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
