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
	// events are the journals read, by card identifier.
	events map[string][]bench.Event
	// earliest memoises earliestOf for a holder whose computation met no
	// card on the path, because such an answer depends on nothing above the
	// holder and is the same along every path.
	earliest map[string]earliestDay
}

// earliestDay is earliestOf's answer: the day, and whether there is one.
type earliestDay struct {
	day bench.Date
	ok  bool
}

// holdsOn is the request's hold index, built on first use from the live
// cards and Bench.Holds, and answered again on every later call on req. It
// is the one place a hold reads the disk, so it is the one place a hold can
// fail, and an unreadable journal fails the read that asked, as a read of
// the journal fails elsewhere.
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
func (l *Library) buildHolds(now time.Time) (*holdIndex, error) {
	settings, _ := l.Bench.Holds()
	if len(settings.Rules) == 0 {
		return nil, nil
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	index := &holdIndex{
		bench:    l.Bench,
		now:      now,
		edges:    bench.HoldEdges(cards, settings.Rules),
		byHeld:   map[string][]bench.HoldEdge{},
		cards:    map[string]*bench.Card{},
		events:   map[string][]bench.Event{},
		earliest: map[string]earliestDay{},
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
		events, _, err := bench.ReadJournal(path)
		if err != nil {
			return nil, err
		}
		index.events[holder.ID] = events
	}
	return index, nil
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
		ground, inForce := x.groundOf(edge, holder, today, map[string]bool{card.ID: true})
		if inForce {
			grounds = append(grounds, ground)
		}
	}
	return grounds
}

// groundOf reads one edge's ground on today, reporting whether it is in
// force. path is the cards from the one asked about down to the held card of
// this edge, which the not-before recursion reads a holder already on as no
// day.
func (x *holdIndex) groundOf(edge bench.HoldEdge, holder *bench.Card, today bench.Date, path map[string]bool) (linkHold, bool) {
	ground := linkHold{
		Holder:   holder,
		Kind:     edge.Rule.Kind,
		WaitsFor: edge.Rule.WaitsFor,
		FinishAt: edge.Rule.FinishAt,
		LagDays:  edge.Rule.LagDays,
	}
	if !x.bench.Reached(holder, edge.Rule, x.now) {
		if path[holder.ID] {
			// A card waiting on itself, or on a card already on the path,
			// reads that card as no day.
			return ground, true
		}
		if earliest, ok, _ := x.earliestOf(holder, today, path); ok {
			if notBefore := earliest.AddDays(edge.Rule.LagDays); notBefore.After(today) {
				ground.NotBefore = notBefore.String()
			}
		}
		return ground, true
	}
	if edge.Rule.LagDays == 0 {
		return ground, false
	}
	// A holder that reached its event on no readable day makes no lagging
	// ground, so its lag is treated as already run.
	reached, ok := bench.ParseDate(x.bench.DayReached(holder, edge.Rule, x.events[holder.ID], x.now))
	if !ok {
		return ground, false
	}
	until := reached.AddDays(edge.Rule.LagDays)
	if !until.After(today) {
		return ground, false
	}
	ground.Reached, ground.Until = reached.String(), until.String()
	return ground, true
}

// earliestOf is the earliest day holder could reach any event given its own
// dates: the latest of its own start_after where that parses, the until date
// of each of its own lagging grounds, and the not-before date of each of its
// own awaiting grounds, computed the same way. A holder that has started has
// passed its own start constraints and answers no day. A ground whose holder
// is already on path is read as no day, and nothing below it is read, which
// is what ends a cycle. The third answer reports whether the computation met
// a card on the path, which is what decides whether the answer may be
// memoised.
func (x *holdIndex) earliestOf(holder *bench.Card, today bench.Date, path map[string]bool) (bench.Date, bool, bool) {
	if known, ok := x.earliest[holder.ID]; ok {
		return known.day, known.ok, false
	}
	if x.bench.Started(holder, x.now) {
		x.earliest[holder.ID] = earliestDay{}
		return bench.Date{}, false, false
	}
	latest, found, metPath := bench.Date{}, false, false
	later := func(day bench.Date) {
		if !found || day.After(latest) {
			latest, found = day, true
		}
	}
	if startAfter, ok := holder.ScheduleDate(bench.StartAfterField); ok {
		later(startAfter)
	}
	path[holder.ID] = true
	for _, edge := range x.byHeld[holder.ID] {
		above := x.cards[edge.Holder]
		if above == nil {
			continue
		}
		if path[above.ID] {
			metPath = true
			continue
		}
		if !x.bench.Reached(above, edge.Rule, x.now) {
			day, ok, met := x.earliestOf(above, today, path)
			metPath = metPath || met
			if ok {
				later(day.AddDays(edge.Rule.LagDays))
			}
			continue
		}
		if ground, inForce := x.groundOf(edge, above, today, path); inForce {
			until, _ := bench.ParseDate(ground.Until)
			later(until)
		}
	}
	delete(path, holder.ID)
	if !metPath {
		x.earliest[holder.ID] = earliestDay{day: latest, ok: found}
	}
	return latest, found, metPath
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
	cards, err := l.Bench.Cards()
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
// at the request's one clock reading. A card whose header will not read
// fails the listing, which the card walk has already reported, so the cycles
// are then left unreported rather than failing the whole check.
func (l *Library) holdCycleFindings(req *Request) []bench.Finding {
	settings, _ := l.Bench.Holds()
	if len(settings.Rules) == 0 {
		return nil
	}
	cards, err := l.Bench.Cards()
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
