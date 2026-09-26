package verb

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// UrgencyAnswer is one card's place in a section of a view ordered by
// urgency: its rank within the section, its score, and the terms behind the
// score when the draw was asked to explain them.
//
// Score and every term's Points are written from integer tenths with exactly
// one decimal digit, so a JSON reader meets the literal the table prints.
type UrgencyAnswer struct {
	// Rank is one-based, within the section.
	Rank int `json:"rank"`
	// Score is the card's total, the sum of its eight terms.
	Score json.Number `json:"score"`
	// Terms are all eight terms in the fixed order of bench.UrgencyTerms
	// when the draw was explained, and empty otherwise.
	Terms []UrgencyTerm `json:"terms"`
	// Contributing are the terms whose points are not zero, in the same
	// order, which is what the table's Why cell names. It is off the wire
	// for the reason Primer.ChainServed is: the JSON answer carries terms
	// only when it is explained, and the human table needs them anyway.
	Contributing []UrgencyTerm `json:"-"`
}

// UrgencyTerm is one term's contribution to a card's score, with the values
// the contribution was computed from. Every basis value is a string, and
// which keys a term carries is fixed per term.
type UrgencyTerm struct {
	Term   string            `json:"term"`
	Points json.Number       `json:"points"`
	Basis  map[string]string `json:"basis"`
}

// The basis keys the terms carry, by the names the JSON answer publishes.
const (
	basisColumn   = "column"
	basisOwned    = "owned"
	basisCounted  = "counted"
	basisPerItem  = "per-item"
	basisCap      = "cap"
	basisLevel    = "level"
	basisDeclared = "declared"
	basisRank     = "rank"
	basisOf       = "of"
	basisWeight   = "weight"
	basisBlocked  = "blocked"
	basisKind     = "kind"
	basisRead     = "read"
	basisArrival  = "arrival"
	basisDays     = "days"
	basisPerDay   = "per-day"
	basisExpired  = "expired"
	basisHolder   = "holder"
)

// urgencyTerm is one term as scoring computes it, its points held as an
// integer count of tenths.
type urgencyTerm struct {
	term   string
	points int64
	basis  map[string]string
}

// urgencyScore is a card's eight terms in the order of bench.UrgencyTerms.
// Its total is always the sum of those terms and is never stored apart from
// them, so the figure a table prints, the total an explanation prints, the
// JSON score and the sort key are one number by construction.
type urgencyScore struct {
	terms []urgencyTerm
}

// total is the score: the sum of the eight contributions, in tenths.
func (s urgencyScore) total() int64 {
	sum := int64(0)
	for _, term := range s.terms {
		sum += term.points
	}
	return sum
}

// answer publishes a score at a rank, carrying every term where the draw is
// explained and only the contributing ones off the wire otherwise.
func (s urgencyScore) answer(rank int, explained bool) UrgencyAnswer {
	answer := UrgencyAnswer{
		Rank:         rank,
		Score:        json.Number(bench.FormatTenths(s.total())),
		Terms:        []UrgencyTerm{},
		Contributing: []UrgencyTerm{},
	}
	for _, term := range s.terms {
		published := UrgencyTerm{Term: term.term, Points: json.Number(bench.FormatTenths(term.points)), Basis: term.basis}
		if explained {
			answer.Terms = append(answer.Terms, published)
		}
		if term.points != 0 {
			answer.Contributing = append(answer.Contributing, published)
		}
	}
	return answer
}

// cardHistory is what a draw read out of one card's journal: the moment the
// card entered the column it stands in, and the events themselves. Both come
// from one reading, so a move landing mid-draw cannot give one term an
// arrival from one reading and the sort a position from another.
type cardHistory struct {
	arrival time.Time
	events  []bench.Event
}

// viewDraw is what one draw of a view reads once and every section shares:
// the instant, the cards after the lapse pass, the caller's actionable set,
// and each ranked card's history and score. A card appearing in two sections
// is read and scored once, so it carries the same score in both.
type viewDraw struct {
	l   *Library
	req *Request
	// now is read once, after the lapse pass, and every term of every card
	// reads it.
	now      time.Time
	weights  bench.Urgency
	operator bool
	// cards are the live cards, in the order the store listed them, after
	// the lapse pass, and byID indexes them.
	cards []*bench.Card
	byID  map[string]*bench.Card
	// offered are the cards columnOffers offers the caller, and actionable
	// is the caller's whole actionable set, both by card identifier.
	offered    map[string]bool
	actionable map[string]bool
	items      map[string][]*bench.Item
	histories  map[string]cardHistory
	scores     map[string]urgencyScore
}

// newViewDraw reads what a draw needs once: every live card, the lapse pass
// over them, the instant, and the caller's actionable set.
func (l *Library) newViewDraw(req *Request, weights bench.Urgency) (*viewDraw, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	for _, card := range cards {
		if err := l.lapseRead(card, req.Actor); err != nil {
			return nil, err
		}
	}
	d := &viewDraw{
		l:         l,
		req:       req,
		now:       l.Now(),
		weights:   weights,
		operator:  req.Actor != "" && req.Actor == l.Bench.Operator,
		cards:     cards,
		byID:      map[string]*bench.Card{},
		items:     map[string][]*bench.Item{},
		histories: map[string]cardHistory{},
		scores:    map[string]urgencyScore{},
	}
	for _, card := range cards {
		d.byID[card.ID] = card
	}
	d.offered, d.actionable, err = l.actionable(req, d)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// actionable is the set of cards the caller of req can act on, and the part
// of it columnOffers offers, both by card identifier.
//
// Every caller is offered what next would offer: each column's offered card,
// computed by the one function Next and Prime call, and an offer carrying
// AboveTier contributes nothing. The operator's set adds three arms to that:
// every card standing in a column he owns that is not terminal, every card
// carrying an item that waits on him, in any column, and every blocked card.
// A caller who is not the operator is never given those three arms.
func (l *Library) actionable(req *Request, d *viewDraw) (offered, set map[string]bool, err error) {
	offers, err := l.columnOffers(req, d.cards, l.Bench.Columns)
	if err != nil {
		return nil, nil, err
	}
	offered = map[string]bool{}
	set = map[string]bool{}
	for _, offer := range offers {
		if offer.Card == nil {
			continue
		}
		offered[offer.Card.ID] = true
		set[offer.Card.ID] = true
	}
	if !d.operator {
		return offered, set, nil
	}
	for _, card := range d.cards {
		if d.standsWhereTheOperatorActs(card) || card.State == contract.StateBlocked {
			set[card.ID] = true
			continue
		}
		items, err := d.itemsOf(card)
		if err != nil {
			return nil, nil, err
		}
		for _, item := range items {
			if bench.ItemAwaitsOperator(item) {
				set[card.ID] = true
				break
			}
		}
	}
	return offered, set, nil
}

// standsWhereTheOperatorActs reports whether a card stands in a column the
// operator owns that is not terminal. A finished card standing in an
// operator-owned done column is work already done, so it earns neither the
// operator's first arm nor the waits-on-you term.
func (d *viewDraw) standsWhereTheOperatorActs(card *bench.Card) bool {
	column := d.l.Bench.Column(card.Column)
	return column != nil && column.OperatorOwned && !column.Terminal()
}

// actionableCards is the caller's actionable set as cards, in the order the
// store listed them.
func (d *viewDraw) actionableCards() []*bench.Card {
	var cards []*bench.Card
	for _, card := range d.cards {
		if d.actionable[card.ID] {
			cards = append(cards, card)
		}
	}
	return cards
}

// current is the draw's own copy of a card, which a section whose query read
// the store again maps its cards onto so that every section scores one
// snapshot. A card the draw did not read is its own copy.
func (d *viewDraw) current(card *bench.Card) *bench.Card {
	if mine, ok := d.byID[card.ID]; ok {
		return mine
	}
	return card
}

// itemsOf is a card's live checklist items, read once per draw.
func (d *viewDraw) itemsOf(card *bench.Card) ([]*bench.Item, error) {
	if items, ok := d.items[card.ID]; ok {
		return items, nil
	}
	items, err := d.l.Bench.Items(card.Dir)
	if err != nil {
		return nil, err
	}
	d.items[card.ID] = items
	return items, nil
}

// history is a card's arrival and journal, read once per draw. A journal
// that cannot be read gives the zero arrival and no events, which every term
// reads as nothing known.
func (d *viewDraw) history(card *bench.Card) cardHistory {
	card = d.current(card)
	if known, ok := d.histories[card.ID]; ok {
		return known
	}
	path := card.JournalPath()
	d.l.observe(ObserveJournal, path)
	events, _, err := d.l.Bench.ReadJournal(path)
	known := cardHistory{}
	if err == nil {
		known = cardHistory{arrival: bench.ArrivalFrom(events, card.Column), events: events}
	}
	d.histories[card.ID] = known
	return known
}

// rank orders one section's cards by urgency: score highest first, then
// arrival in the current column earliest first, then card number lowest
// first. It is the queue's own tie-break applied to the per-draw arrival
// rather than through bench.ByArrival, which reads the journal twice per
// comparison, and it answers each card's score in the order it sorted them.
func (d *viewDraw) rank(cards []*bench.Card) ([]urgencyScore, error) {
	scores := make(map[string]urgencyScore, len(cards))
	for _, card := range cards {
		score, err := d.urgencyOf(card)
		if err != nil {
			return nil, err
		}
		scores[card.ID] = score
	}
	sort.SliceStable(cards, func(i, j int) bool {
		first, second := scores[cards[i].ID].total(), scores[cards[j].ID].total()
		if first != second {
			return first > second
		}
		early, late := d.history(cards[i]).arrival, d.history(cards[j]).arrival
		if !early.Equal(late) {
			return early.Before(late)
		}
		return cards[i].Number < cards[j].Number
	})
	ordered := make([]urgencyScore, 0, len(cards))
	for _, card := range cards {
		ordered = append(ordered, scores[card.ID])
	}
	return ordered, nil
}

// urgencyOf is a card's score for the caller at the draw's instant, computed
// once per draw. Every figure the view prints about the card reads the
// answer this returns.
func (d *viewDraw) urgencyOf(card *bench.Card) (urgencyScore, error) {
	card = d.current(card)
	if known, ok := d.scores[card.ID]; ok {
		return known, nil
	}
	question, err := d.yourQuestion(card)
	if err != nil {
		return urgencyScore{}, err
	}
	blocking, err := d.blocksOthers(card)
	if err != nil {
		return urgencyScore{}, err
	}
	score := urgencyScore{terms: []urgencyTerm{
		d.waitsOnYou(card),
		question,
		d.level(card, bench.PriorityField, card.Priority, d.weights.Priority),
		d.level(card, bench.SeverityField, card.Severity, d.weights.Severity),
		d.blocked(card),
		blocking,
		d.age(card),
		d.staleClaim(card),
	}}
	d.scores[card.ID] = score
	return score, nil
}

// waitsOnYou applies to the operator alone, at a column he owns that is not
// terminal. The only ownership a column declares is the operator's, so for
// every other caller it never applies.
func (d *viewDraw) waitsOnYou(card *bench.Card) urgencyTerm {
	column := ""
	if found := d.l.Bench.Column(card.Column); found != nil {
		column = columnRef(found)
	}
	owned := d.operator && d.standsWhereTheOperatorActs(card)
	points := int64(0)
	if owned {
		points = d.weights.WaitsOnYou
	}
	return urgencyTerm{
		term:   bench.UrgencyWaitsOnYou,
		points: points,
		basis:  map[string]string{basisColumn: column, basisOwned: strconv.FormatBool(owned)},
	}
}

// yourQuestion counts the pending open questions and decisions on a card that
// are the caller's, and contributes the per-item weight for each up to the
// cap. An item is the caller's when the caller is the operator and it waits
// on him; when its owner is holder and the caller holds the card, or would
// hold it by taking the card a column offers; or when its owner names the
// caller exactly and is neither holder nor operator.
func (d *viewDraw) yourQuestion(card *bench.Card) (urgencyTerm, error) {
	items, err := d.itemsOf(card)
	if err != nil {
		return urgencyTerm{}, err
	}
	actor := d.req.Actor
	takes := isHolder(card, actor) || d.offered[card.ID]
	owned := int64(0)
	for _, item := range items {
		if item.State != bench.ItemPending || (item.Kind != "open_question" && item.Kind != "decision") {
			continue
		}
		mine := false
		switch {
		case d.operator && bench.ItemAwaitsOperator(item):
			mine = true
		case item.Owner == "holder":
			mine = takes
		case item.Owner != bench.ItemOwnerOperator && item.Owner == actor:
			mine = true
		}
		if mine {
			owned++
		}
	}
	counted := owned
	if counted > d.weights.ItemCap {
		counted = d.weights.ItemCap
	}
	return urgencyTerm{
		term:   bench.UrgencyYourQuestion,
		points: d.weights.PerItem * counted,
		basis: map[string]string{
			basisOwned:   strconv.FormatInt(owned, 10),
			basisCounted: strconv.FormatInt(counted, 10),
			basisPerItem: bench.FormatTenths(d.weights.PerItem),
			basisCap:     strconv.FormatInt(d.weights.ItemCap, 10),
		},
	}, nil
}

// level is the priority or the severity term. The weight list aligns at the
// top of the declared set, so the highest declared level always takes the
// last weight and a list shorter than the set gives its first weight to the
// ranks below it. A card carrying no level, or a name the axis does not
// declare, contributes nothing, and so does every card on a workbench
// declaring no set on the axis.
func (d *viewDraw) level(card *bench.Card, axis, name string, weights []int64) urgencyTerm {
	levels := d.l.Bench.Levels(axis)
	term := urgencyTerm{
		term: axis,
		basis: map[string]string{
			basisLevel:    name,
			basisDeclared: "false",
			basisRank:     "",
			basisOf:       strconv.Itoa(len(levels)),
			basisWeight:   bench.FormatTenths(0),
		},
	}
	declared := d.l.Bench.Level(axis, name)
	if name == "" || declared == nil || len(weights) == 0 {
		return term
	}
	i := declared.Rank + len(weights) - len(levels)
	if i < 0 {
		i = 0
	}
	term.points = weights[i]
	term.basis[basisDeclared] = "true"
	term.basis[basisRank] = strconv.Itoa(declared.Rank + 1)
	term.basis[basisWeight] = bench.FormatTenths(weights[i])
	return term
}

// blocked contributes its weight when the card is blocked.
func (d *viewDraw) blocked(card *bench.Card) urgencyTerm {
	blocked := card.State == contract.StateBlocked
	points := int64(0)
	kind := ""
	if blocked {
		points = d.weights.Blocked
		kind = card.BlockKind
	}
	return urgencyTerm{
		term:   bench.UrgencyBlocked,
		points: points,
		basis:  map[string]string{basisBlocked: strconv.FormatBool(blocked), basisKind: kind},
	}
}

// blocksOthers contributes its weight once for each live card, standing
// outside a done column, that waits on this card through an awaiting ground:
// a link of a kind the workbench declares under dinah.holds, whose held card
// has not started and whose holder, this card, has not reached the rule's
// event. A lagging ground does not count, because the holder has done its
// part. A link carries no direction of its own, so on a workbench declaring
// no usable rule nothing is counted, the term contributes nothing and its
// basis says the links were not read.
func (d *viewDraw) blocksOthers(card *bench.Card) (urgencyTerm, error) {
	holds, err := d.l.holdsOn(d.req)
	if err != nil {
		return urgencyTerm{}, err
	}
	if holds == nil {
		return urgencyTerm{
			term:  bench.UrgencyBlocksOthers,
			basis: map[string]string{basisRead: "false"},
		}, nil
	}
	counted := int64(holds.awaitingOn(card))
	return urgencyTerm{
		term:   bench.UrgencyBlocksOthers,
		points: d.weights.BlocksOthers * counted,
		basis: map[string]string{
			basisRead:    "true",
			basisCounted: strconv.FormatInt(counted, 10),
		},
	}, nil
}

// age counts whole days since the card entered its current column, capped,
// and contributes the per-day weight for each. Clock skew between writers
// can stamp an arrival after now, and an unreadable journal gives no arrival
// at all; both count as no days.
func (d *viewDraw) age(card *bench.Card) urgencyTerm {
	arrival := d.history(card).arrival
	days := int64(0)
	stamp := ""
	if !arrival.IsZero() {
		stamp = bench.Stamp(arrival)
		if elapsed := d.now.Sub(arrival); elapsed > 0 {
			days = int64(elapsed / (24 * time.Hour))
		}
	}
	if days > d.weights.DayCap {
		days = d.weights.DayCap
	}
	return urgencyTerm{
		term:   bench.UrgencyAge,
		points: d.weights.PerDay * days,
		basis: map[string]string{
			basisArrival: stamp,
			basisDays:    strconv.FormatInt(days, 10),
			basisPerDay:  bench.FormatTenths(d.weights.PerDay),
			basisCap:     strconv.FormatInt(d.weights.DayCap, 10),
		},
	}
}

// staleClaim contributes its weight on a ready card whose last claimed or
// expired event since it entered its current column is an expiry. A claim
// past its expiry is never observable, because the draw's lapse pass has
// already lapsed it, so a lapsed claim is the only form the term can read.
// A later claim, a move and a release after a fresh claim all clear it.
func (d *viewDraw) staleClaim(card *bench.Card) urgencyTerm {
	term := urgencyTerm{
		term:  bench.UrgencyStaleClaim,
		basis: map[string]string{basisExpired: "", basisHolder: ""},
	}
	if card.State != contract.StateReady {
		return term
	}
	history := d.history(card)
	var last *bench.Event
	for i := range history.events {
		event := &history.events[i]
		if event.Event != contract.EventClaimed && event.Event != contract.EventExpired {
			continue
		}
		if bench.ParseStamp(event.TS).Before(history.arrival) {
			continue
		}
		last = event
	}
	if last == nil || last.Event != contract.EventExpired {
		return term
	}
	term.points = d.weights.StaleClaim
	term.basis[basisExpired] = last.TS
	term.basis[basisHolder] = last.Actor.Name
	return term
}
