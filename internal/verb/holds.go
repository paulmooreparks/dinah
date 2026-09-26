package verb

import (
	"strconv"
	"strings"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// linkHold is one ground in force on a card, as selection and the card view
// read it.
type linkHold struct {
	Holder    *bench.Card
	Kind      string
	WaitsFor  string // contract.HoldWaitsStart or contract.HoldWaitsFinish
	FinishAt  string // column identifier, empty for a done column
	LagDays   int
	Reached   string // YYYY-MM-DD, empty while awaiting
	Until     string // YYYY-MM-DD, present exactly on a lagging ground
	NotBefore string // YYYY-MM-DD, present only on an awaiting ground, and only where later than today
}

// awaiting reports whether the ground waits on its holder's event rather
// than on a date.
func (h linkHold) awaiting() bool {
	return h.Until == ""
}

// holdIndex is every holding edge among the live cards, by held card, with
// the journal of every holder of an edge whose rule carries a lag read once,
// when the index is built. It is nil on a workbench declaring no usable rule,
// and every reader treats nil as holding nothing.
type holdIndex struct {
	bench *bench.Bench
	// now is the request's one clock reading, which every question of
	// whether a card has started is asked at.
	now time.Time
	// edges are every edge among the live cards, in HoldEdges' order, and
	// byHeld indexes them by held card.
	edges  []bench.HoldEdge
	byHeld map[string][]bench.HoldEdge
	// cards are the live cards by identifier, which is what makes an
	// archived or missing holder hold nothing: it is not here.
	cards map[string]*bench.Card
	// events are the journals read, by card identifier. A journal that
	// would not read is here as nil, which reads as no day.
	events map[string][]bench.Event
	// component numbers the strongly connected set of the walked edges each
	// card stands in, for the cards at either end of a walked edge. Two
	// cards share a number exactly when each waits, through walked edges,
	// on the other, which is when they stand in one cycle.
	component map[string]int
	// members are each component's cards, by component number.
	members [][]string
	// own memoises ownOf by card, and floor memoises earliestOf by
	// component.
	own   map[string]earliestDay
	floor map[int]earliestDay
}

// earliestDay is a not-before computation's answer: the day, and whether
// there is one.
type earliestDay struct {
	day bench.Date
	ok  bool
}

// later is e moved to day where day is later, or where e has no day yet.
func (e earliestDay) later(day bench.Date) earliestDay {
	if !e.ok || day.After(e.day) {
		return earliestDay{day: day, ok: true}
	}
	return e
}

// holdsOn is the request's hold index, built on first use from the live
// cards and Bench.Holds, and answered again on every later call on req. It
// is the one place a hold reads the disk. A card whose anchor will not load
// and a journal that will not read are each left out of it rather than
// failing it, because every card view reads the index, and one damaged card
// must not take down the answer about every other card.
func (l *Library) holdsOn(req *Request) (*holdIndex, error) {
	return l.holdsAt(l.dayOf(req))
}

// holdsAt is holdsOn for a caller that already holds the request's day.
func (l *Library) holdsAt(day *requestDay) (*holdIndex, error) {
	if day.holdsRead {
		return day.holds, nil
	}
	index, err := l.buildHolds(day.now)
	if err != nil {
		return nil, err
	}
	day.holds, day.holdsRead = index, true
	return index, nil
}

// buildHolds reads the index at now. On a workbench declaring no usable rule
// it answers nil without listing a card or reading a journal, and a journal
// is read only for the holder of an edge whose rule carries a lag, because a
// day is needed only there.
//
// The cards are read through ReadableCards, so a card whose anchor will not
// load is absent, which makes it hold nothing and be held by nothing, as an
// archived card is; dinah check's card walk reports it. A holder whose
// journal will not read has no readable day, so it makes no lagging ground,
// on the rule that a day which cannot be read never withholds
// (dinah-608/decisions/19).
func (l *Library) buildHolds(now time.Time) (*holdIndex, error) {
	settings, _ := l.Bench.Holds()
	if len(settings.Rules) == 0 {
		return nil, nil
	}
	cards, err := l.Bench.ReadableCards()
	if err != nil {
		return nil, err
	}
	index := &holdIndex{
		bench:     l.Bench,
		now:       now,
		edges:     bench.HoldEdges(cards, settings.Rules),
		byHeld:    map[string][]bench.HoldEdge{},
		cards:     map[string]*bench.Card{},
		events:    map[string][]bench.Event{},
		component: map[string]int{},
		own:       map[string]earliestDay{},
		floor:     map[int]earliestDay{},
	}
	for _, card := range cards {
		index.cards[card.ID] = card
	}
	for _, edge := range index.edges {
		index.byHeld[edge.Held] = append(index.byHeld[edge.Held], edge)
		holder := index.cards[edge.Holder]
		if edge.Rule.LagDays == 0 || holder == nil {
			continue
		}
		if _, read := index.events[holder.ID]; read {
			continue
		}
		path := holder.JournalPath()
		l.observe(ObserveJournal, path)
		events, _, err := l.Bench.ReadJournal(path)
		if err != nil {
			events = nil
		}
		index.events[holder.ID] = events
	}
	index.partition()
	return index, nil
}

// walked reports whether the not-before computation follows edge from its
// held card to its holder: both are live, the held card has not started,
// and the holder has not reached the rule's event. An edge whose holder has
// reached its event is read for its lag and not followed, and a card that
// has started has passed its own start constraints, so nothing out of it is
// followed.
func (x *holdIndex) walked(edge bench.HoldEdge) bool {
	held, holder := x.cards[edge.Held], x.cards[edge.Holder]
	if held == nil || holder == nil {
		return false
	}
	return !x.bench.Started(held, x.now) && !x.bench.Reached(holder, edge.Rule, x.now)
}

// partition numbers the strongly connected sets of the walked edges.
func (x *holdIndex) partition() {
	next := map[string][]string{}
	var nodes []string
	seen := map[string]bool{}
	for _, edge := range x.edges {
		if !x.walked(edge) {
			continue
		}
		next[edge.Held] = append(next[edge.Held], edge.Holder)
		for _, id := range []string{edge.Held, edge.Holder} {
			if !seen[id] {
				seen[id] = true
				nodes = append(nodes, id)
			}
		}
	}
	for number, members := range bench.StronglyConnected(nodes, next) {
		for _, id := range members {
			x.component[id] = number
		}
		x.members = append(x.members, members)
	}
}

// together reports whether two cards stand in one cycle of walked edges, or
// are one card.
func (x *holdIndex) together(first, second string) bool {
	if first == second {
		return true
	}
	a, inA := x.component[first]
	b, inB := x.component[second]
	return inA && inB && a == b
}

// holdsOf is every ground in force on card today, awaiting and lagging, in
// edge order, and nil where card has started or is not live. It reads only
// the index. The card asked about is the one passed in, which a caller may
// hold fresher than the index's copy.
func (x *holdIndex) holdsOf(b *bench.Bench, card *bench.Card, today bench.Date) []linkHold {
	if x == nil || x.cards[card.ID] == nil {
		return nil
	}
	if b.Started(card, x.now) {
		return nil
	}
	var grounds []linkHold
	for _, edge := range x.byHeld[card.ID] {
		holder := x.cards[edge.Holder]
		if holder == nil {
			continue
		}
		ground, inForce := x.groundOf(edge, holder, today, card.ID)
		if inForce {
			grounds = append(grounds, ground)
		}
	}
	return grounds
}

// groundOf reads the ground one edge of the card asked about makes today,
// reporting whether it is in force.
func (x *holdIndex) groundOf(edge bench.HoldEdge, holder *bench.Card, today bench.Date, asked string) (linkHold, bool) {
	ground := linkHold{
		Holder:   holder,
		Kind:     edge.Rule.Kind,
		WaitsFor: edge.Rule.WaitsFor,
		FinishAt: edge.Rule.FinishAt,
		LagDays:  edge.Rule.LagDays,
	}
	if !x.bench.Reached(holder, edge.Rule, x.now) {
		if holder.ID == asked {
			// A card waiting on itself reads itself as no day.
			return ground, true
		}
		var earliest earliestDay
		if x.together(asked, holder.ID) {
			earliest = x.earliestAvoiding(holder, asked, today)
		} else {
			earliest = x.earliestOf(holder, today)
		}
		if earliest.ok {
			if notBefore := earliest.day.AddDays(edge.Rule.LagDays); notBefore.After(today) {
				ground.NotBefore = notBefore.String()
			}
		}
		return ground, true
	}
	reached, until, ok := x.lagging(edge, holder, today)
	if !ok {
		return ground, false
	}
	ground.Reached, ground.Until = reached.String(), until.String()
	return ground, true
}

// lagging reads the lagging ground of an edge whose holder has reached its
// event: the day it reached it and the day the lag runs out, and whether
// that day is still to come. A rule with no lag makes no lagging ground, and
// a holder that reached its event on no readable day makes none either, so
// its lag is treated as already run.
func (x *holdIndex) lagging(edge bench.HoldEdge, holder *bench.Card, today bench.Date) (bench.Date, bench.Date, bool) {
	if edge.Rule.LagDays == 0 {
		return bench.Date{}, bench.Date{}, false
	}
	reached, ok := bench.ParseDate(x.bench.DayReached(holder, edge.Rule, x.events[holder.ID], x.now))
	if !ok {
		return bench.Date{}, bench.Date{}, false
	}
	until := reached.AddDays(edge.Rule.LagDays)
	if !until.After(today) {
		return bench.Date{}, bench.Date{}, false
	}
	return reached, until, true
}

// The not-before computation.
//
// The specification defines a holder's earliest day by a walk that keeps its
// path: the latest of the holder's own start_after, the until date of each of
// its lagging grounds, and, for each awaiting ground, that ground's holder's
// earliest day plus the lag, where a holder already on the path is read as no
// day. That is the latest date over every simple path out of the holder,
// which on a cycle is a longest-simple-path question. A walk answering it
// exactly takes time exponential in the size of the cycle: a ladder of 16
// cards in one cycle took most of a second, and each added card multiplied
// the time by about 1.6.
//
// This computation reads the same dates in time linear in the walked edges
// for each ground read. It partitions the walked edges into strongly
// connected sets, which are the cycles, and computes in three parts.
//
//   - ownOf is a card's own contribution, which depends on no path: its
//     start_after, the until dates of its lagging grounds, and, for each
//     walked edge leaving its cycle, the earliest day of that edge's holder
//     plus the lag. A holder outside the card's cycle cannot reach back into
//     it, so no card on any path through the cycle is below that holder, and
//     its answer is the same along every path. ownOf is memoised per card.
//   - earliestOf is the earliest day of a holder read from outside its
//     cycle: the latest ownOf over every card of the cycle, since each is
//     reached from the holder by a simple path that meets no card of the
//     path above. It is memoised per cycle. On a card in no cycle it is
//     ownOf, so on an acyclic graph the answer is the specification's with
//     every lag counted, and the diamond reads X+9 because ownOf adds each
//     route's own lag to the one memoised answer below it.
//   - earliestAvoiding is a holder read by a card of its own cycle: the
//     latest ownOf over the cards the holder reaches inside the cycle
//     without passing through the card asked about, which is the set of
//     cards a simple path from the holder reaches with the asked card on the
//     path. The asked card's own dates therefore never return to it.
//
// What is given up is the sum of the lags along a path inside a cycle,
// beyond the lag of the ground being read, because that sum is the
// longest-path part. A ground inside a cycle is released by no date anyway,
// since the cycle holds every card in it until somebody starts one, so its
// not-before date is a floor, and leaving lags out of a floor keeps it a
// floor. Where no edge inside a cycle carries a lag, the answer is the
// specification's exactly. It is also exact on the specification's own cycle
// of two cards, lags or not, because a path through a cycle of two crosses
// no edge of it beyond the ground being read. dinah-608/decisions/18 records
// the choice.

// ownOf is card's own contribution to a not-before date, as described above,
// and no day where card has started.
func (x *holdIndex) ownOf(card *bench.Card, today bench.Date) earliestDay {
	if known, ok := x.own[card.ID]; ok {
		return known
	}
	var answer earliestDay
	if !x.bench.Started(card, x.now) {
		if startAfter, ok := card.ScheduleDate(bench.StartAfterField); ok {
			answer = answer.later(startAfter)
		}
		for _, edge := range x.byHeld[card.ID] {
			above := x.cards[edge.Holder]
			if above == nil {
				continue
			}
			if x.bench.Reached(above, edge.Rule, x.now) {
				if _, until, ok := x.lagging(edge, above, today); ok {
					answer = answer.later(until)
				}
				continue
			}
			if x.together(card.ID, above.ID) {
				continue
			}
			if earliest := x.earliestOf(above, today); earliest.ok {
				answer = answer.later(earliest.day.AddDays(edge.Rule.LagDays))
			}
		}
	}
	x.own[card.ID] = answer
	return answer
}

// earliestOf is the earliest day holder could reach any event given the
// dates of the cards it waits on, read from outside holder's cycle.
func (x *holdIndex) earliestOf(holder *bench.Card, today bench.Date) earliestDay {
	number, inCycle := x.component[holder.ID]
	if !inCycle {
		return x.ownOf(holder, today)
	}
	if known, ok := x.floor[number]; ok {
		return known
	}
	var answer earliestDay
	for _, id := range x.members[number] {
		if own := x.ownOf(x.cards[id], today); own.ok {
			answer = answer.later(own.day)
		}
	}
	x.floor[number] = answer
	return answer
}

// earliestAvoiding is the earliest day of a holder standing in one cycle with
// the card asked about, read without passing through that card.
func (x *holdIndex) earliestAvoiding(holder *bench.Card, asked string, today bench.Date) earliestDay {
	visited := map[string]bool{asked: true, holder.ID: true}
	queue := []string{holder.ID}
	var answer earliestDay
	for len(queue) > 0 {
		at := x.cards[queue[0]]
		queue = queue[1:]
		if own := x.ownOf(at, today); own.ok {
			answer = answer.later(own.day)
		}
		for _, edge := range x.byHeld[at.ID] {
			if visited[edge.Holder] || !x.walked(edge) || !x.together(at.ID, edge.Holder) {
				continue
			}
			visited[edge.Holder] = true
			queue = append(queue, edge.Holder)
		}
	}
	return answer
}

// awaitingOn counts the live cards standing outside a done column that wait
// on holder through an awaiting ground: the held card has not started and
// holder has not reached the rule's event. Each held card counts once,
// however many edges it waits on holder through.
func (x *holdIndex) awaitingOn(holder *bench.Card) int {
	if x == nil || x.cards[holder.ID] == nil {
		return 0
	}
	counted := map[string]bool{}
	for _, edge := range x.edges {
		if edge.Holder != holder.ID || counted[edge.Held] {
			continue
		}
		held := x.cards[edge.Held]
		if held == nil {
			continue
		}
		if column := x.bench.Column(held.Column); column != nil && column.Terminal() {
			continue
		}
		if x.bench.Started(held, x.now) || x.bench.Reached(holder, edge.Rule, x.now) {
			continue
		}
		counted[edge.Held] = true
	}
	return len(counted)
}

// holdAnswer is what the start hold says about one ready card.
type holdAnswer struct {
	// Held says selection may not hand the card out today.
	Held bool
	// From is the day, YYYY-MM-DD, from which time alone releases the card:
	// the latest of its own start_after and its lagging grounds' until dates.
	// It is empty where Waiting is non-empty, since no date releases it then.
	From string
	// Waiting are the card's awaiting grounds, in the order holdsOf lists them.
	Waiting []linkHold
}

// heldWork is what the start hold withheld during one scan, over the cards
// with a landing that it skipped.
type heldWork struct {
	// NotYetFrom is the earliest From among the skipped cards time alone
	// releases, empty where it skipped none of that sort.
	NotYetFrom string
	// WaitingOn are the references of the holders of the skipped cards'
	// awaiting grounds, each once, in the order first met.
	WaitingOn []string
}

// fold is h with o's work added: the earlier date and the union of holders,
// h's first.
func (h heldWork) fold(o heldWork) heldWork {
	h.NotYetFrom = earlierDate(h.NotYetFrom, o.NotYetFrom)
	h.WaitingOn = unionRefs(h.WaitingOn, o.WaitingOn)
	return h
}

// withheld is h with one skipped card's answer added, on fold's terms.
func (h heldWork) withheld(answer holdAnswer, slug string) heldWork {
	if len(answer.Waiting) == 0 {
		h.NotYetFrom = earlierDate(h.NotYetFrom, answer.From)
		return h
	}
	h.WaitingOn = unionRefs(h.WaitingOn, holderRefs(answer.Waiting, slug))
	return h
}

// unionRefs is first with every reference of second it does not already
// carry appended, in second's order.
func unionRefs(first, second []string) []string {
	for _, ref := range second {
		carried := false
		for _, have := range first {
			if have == ref {
				carried = true
				break
			}
		}
		if !carried {
			first = append(first, ref)
		}
	}
	return first
}

// holderRefs are the references of the grounds' holders, each once, in the
// order the grounds list them.
func holderRefs(grounds []linkHold, slug string) []string {
	var refs []string
	for _, ground := range grounds {
		refs = unionRefs(refs, []string{ground.Holder.Ref(slug)})
	}
	return refs
}

// WaitView is one ground in force on a card as a read reports it.
type WaitView struct {
	// ID and Ref are the holder's identifier and reference.
	ID  string `json:"id"`
	Ref string `json:"ref"`
	// Kind is the link kind the ground was declared under.
	Kind string `json:"kind"`
	// WaitsFor is start or finish.
	WaitsFor string `json:"waits_for"`
	// FinishAt is the reference of the column whose reaching counts as
	// finishing, where the rule names one, and FinishAtTitle is its title,
	// which a renderer prints and no payload carries.
	FinishAt      string `json:"finish_at,omitempty"`
	FinishAtTitle string `json:"-"`
	// LagDays is the rule's lag.
	LagDays int `json:"lag_days,omitempty"`
	// Reached is the day the holder reached its event, on a lagging ground.
	Reached string `json:"reached,omitempty"`
	// Until is the day time alone lifts a lagging ground.
	Until string `json:"until,omitempty"`
	// NotBefore is the earliest day an awaiting ground could lift, given
	// the holder's own dates, where that is later than today.
	NotBefore string `json:"not_before,omitempty"`
}

// waitViews are the grounds as a card view carries them, nil where there
// are none.
func (l *Library) waitViews(grounds []linkHold) []WaitView {
	var views []WaitView
	for _, ground := range grounds {
		view := WaitView{
			ID:        ground.Holder.ID,
			Ref:       ground.Holder.Ref(l.Bench.Slug),
			Kind:      ground.Kind,
			WaitsFor:  ground.WaitsFor,
			LagDays:   ground.LagDays,
			Reached:   ground.Reached,
			Until:     ground.Until,
			NotBefore: ground.NotBefore,
		}
		if column := l.Bench.Column(ground.FinishAt); column != nil {
			view.FinishAt, view.FinishAtTitle = columnRef(column), column.Title
		}
		views = append(views, view)
	}
	return views
}

// holdCycleWarning sets the cycle warning on a successful link whose new
// edge is awaiting and closes a cycle of awaiting edges among the live
// cards, naming the cycle's cards in edge order from the held card of the
// new link. The link is written either way.
func (l *Library) holdCycleWarning(req *Request, response *Response, carrier *bench.Card, kind, to string) error {
	if response == nil || response.Outcome != contract.OutcomeOK || response.Warning != "" {
		return nil
	}
	settings, _ := l.Bench.Holds()
	var rule *bench.HoldRule
	for i := range settings.Rules {
		if settings.Rules[i].Kind == kind {
			rule = &settings.Rules[i]
		}
	}
	if rule == nil {
		return nil
	}
	held, holder := to, carrier.ID
	if rule.Held == contract.HoldHeldCarrier {
		held, holder = carrier.ID, to
	}
	// A card whose anchor will not load is left out, as the hold index
	// leaves it out, so it closes no cycle and the link's answer stands.
	cards, err := l.Bench.ReadableCards()
	if err != nil {
		return err
	}
	edges := l.Bench.AwaitingEdges(cards, settings.Rules, l.dayOf(req).now)
	closes := false
	for _, edge := range edges {
		if edge.Held == held && edge.Holder == holder && edge.Rule.Kind == kind {
			closes = true
		}
	}
	if !closes {
		return nil
	}
	cycle := bench.HoldCycleThrough(edges, held, holder)
	if cycle == nil {
		return nil
	}
	byID := map[string]*bench.Card{}
	for _, card := range cards {
		byID[card.ID] = card
	}
	refs := make([]string, 0, len(cycle))
	for _, id := range cycle {
		refs = append(refs, byID[id].Ref(l.Bench.Slug))
	}
	response.Warning = "warn.hold-cycle"
	response.WarningDetail = strings.Join(refs, ", ")
	return nil
}

// holdCycleFindings are the check.hold-cycle findings among the live cards,
// at the request's one clock reading. A card whose header will not read is
// left out, because the card walk has already reported it, and the cycles
// among the other cards are still reported. A cards directory that will not
// list leaves the cycles unreported rather than failing the whole check.
func (l *Library) holdCycleFindings(req *Request) []bench.Finding {
	settings, _ := l.Bench.Holds()
	if len(settings.Rules) == 0 {
		return nil
	}
	cards, err := l.Bench.ReadableCards()
	if err != nil {
		return nil
	}
	// Check is also called with no request at all, and then the clock is
	// read here, once.
	now := l.Now()
	if req != nil {
		now = l.dayOf(req).now
	}
	var findings []bench.Finding
	for _, cycle := range l.Bench.HoldCycles(cards, settings.Rules, now) {
		refs := make([]string, 0, len(cycle))
		for _, card := range cycle {
			refs = append(refs, card.Ref(l.Bench.Slug))
		}
		findings = append(findings, bench.Finding{
			Path:   cycle[0].AnchorPath(),
			Key:    bench.FindingHoldCycle,
			Detail: strings.Join(refs, ", "),
		})
	}
	return findings
}

// waitingRefs is up to three references joined by ", ", and how many more
// there are beyond them, which is what the waiting sentences print.
func waitingRefs(refs []string) (string, string) {
	if len(refs) <= 3 {
		return strings.Join(refs, ", "), ""
	}
	return strings.Join(refs[:3], ", "), strconv.Itoa(len(refs) - 3)
}
