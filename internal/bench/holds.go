package bench

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"dinah/internal/contract"
)

// HoldsKey is the top-level frontmatter key the holds layer is declared
// under. It is read from the workbench's own workbench.md and from nowhere
// else, and no verb writes it.
const HoldsKey = contract.HoldsKey

// The members of the dinah.holds block and of each kind under it.
const (
	holdsStartAtMember  = "start_at"
	holdsKindsMember    = "kinds"
	holdHeldMember      = "held"
	holdWaitsForMember  = "waits_for"
	holdLagDaysMember   = "lag_days"
	holdFinishAtMember  = "finish_at"
	holdReadDone        = "a done column"
	holdReadFirst       = "the first column of the flow"
	holdReadStartOnly   = "applies only where waits_for is finish"
	holdReadBeforeStart = "before the commitment column"
)

// The four defects a dinah.holds block can carry, as machine tokens. None of
// them refuses anything: the member at fault takes its default, or the kind
// at fault holds nothing, and dinah check names it.
const (
	HoldsNotAMapping     = "not-a-mapping"
	HoldsKindNotAMapping = "kind-not-a-mapping"
	HoldsMalformedMember = "malformed-member"
	HoldsUnknownMember   = "unknown-member"
)

// HoldSettings is what the dinah.holds layer declares, with every member the
// declaration leaves out, or carries unreadably, at its default. A workbench
// declaring no block carries the default commitment column and no rule.
type HoldSettings struct {
	// StartAt is the identifier of the commitment column, declared or
	// defaulted. It is empty only on a workbench with no column at all.
	StartAt string
	// Rules are the kinds that hold, in the order the block declares them.
	Rules []HoldRule
}

// HoldRule is one kind under kinds, with every member the declaration leaves
// out, or carries unreadably, at its default. Only a kind whose held member
// reads is a rule at all.
type HoldRule struct {
	Kind     string
	Held     string // contract.HoldHeldNamed or contract.HoldHeldCarrier
	WaitsFor string // contract.HoldWaitsStart or contract.HoldWaitsFinish
	LagDays  int
	// FinishAt is the identifier of the column whose reaching counts as
	// finishing, empty for a column of kind done.
	FinishAt string
}

// HoldDefect names one thing the reader could not use. Kind is empty for a
// member of the block itself, and Member is empty for a whole kind entry.
type HoldDefect struct {
	Defect string // HoldsNotAMapping, HoldsKindNotAMapping, HoldsMalformedMember or HoldsUnknownMember
	Kind   string
	Member string
	Read   string
}

// DefaultCommitment is the commitment column a flow carries where the holds
// layer names none, or names one unusably: the first column after the flow's
// first at which work is taken up, and failing that the flow's first done
// column. The flow's first column is where work arrives, so it is never the
// default, because a commitment column there would leave nothing for a hold
// to withhold. It is empty only for a flow with no such column at all.
func DefaultCommitment(columns []*Column) string {
	for i, column := range columns {
		if i > 0 && column.TakesWorkUp() {
			return column.ID
		}
	}
	for _, column := range columns {
		if column.Terminal() {
			return column.ID
		}
	}
	return ""
}

// ReadHolds reads the dinah.holds key of whatever frontmatter it is handed,
// through blockValue and firstMembers as ReadSchedule reads its block.
// columns is the flow, from which the default commitment column is taken, and
// column resolves a reference a member names.
//
// A defect falls back rather than refusing, on ReadSchedule's reasoning: the
// rules reach every selection, so a refusal would stop all work over a typo,
// and dinah check names the defect. The commitment column is resolved before
// the kinds are read, so a finish_at is compared against the column the layer
// actually uses.
func ReadHolds(fm *Frontmatter, columns []*Column, column func(ref string) *Column) (HoldSettings, []HoldDefect) {
	settings := HoldSettings{StartAt: DefaultCommitment(columns)}
	if fm == nil || !fm.Has(HoldsKey) {
		return settings, nil
	}
	raw := blockValue(fm, HoldsKey)
	if sameJSON(raw, mustMarshal("")) {
		return settings, nil
	}
	members, mapping := firstMembers(raw)
	if !mapping {
		return settings, []HoldDefect{{Defect: HoldsNotAMapping}}
	}
	var defects []HoldDefect
	var kinds json.RawMessage
	for _, member := range members {
		switch member.name {
		case holdsStartAtMember:
			id, read := holdsStartAt(member.value, columns, column)
			if read != "" {
				defects = append(defects, HoldDefect{Defect: HoldsMalformedMember, Member: member.name, Read: read})
				continue
			}
			settings.StartAt = id
		case holdsKindsMember:
			kinds = member.value
		default:
			defects = append(defects, HoldDefect{Defect: HoldsUnknownMember, Member: member.name, Read: scheduleRead(member.value)})
		}
	}
	if kinds == nil {
		return settings, defects
	}
	entries, mapping := firstMembers(kinds)
	if !mapping {
		defects = append(defects, HoldDefect{Defect: HoldsMalformedMember, Member: holdsKindsMember, Read: scheduleRead(kinds)})
		return settings, defects
	}
	commitment := positionOf(columns, settings.StartAt)
	for _, entry := range entries {
		rule, ruleDefects, ok := readHoldRule(entry, columns, column, commitment)
		defects = append(defects, ruleDefects...)
		if ok {
			settings.Rules = append(settings.Rules, rule)
		}
	}
	return settings, defects
}

// holdsStartAt reads a start_at member, answering the identifier of the
// column it names, or the text a defect reports where it cannot be used: the
// text read where it names no column, and a phrase naming the reason where it
// names a done column or the flow's first column.
func holdsStartAt(raw json.RawMessage, columns []*Column, column func(ref string) *Column) (string, string) {
	read := scheduleRead(raw)
	var ref string
	if err := json.Unmarshal(raw, &ref); err != nil || ref == "" {
		return "", nonEmptyRead(read)
	}
	named := column(ref)
	switch {
	case named == nil:
		return "", nonEmptyRead(read)
	case named.Terminal():
		return "", holdReadDone
	case len(columns) > 0 && named.ID == columns[0].ID:
		return "", holdReadFirst
	}
	return named.ID, ""
}

// nonEmptyRead is the text a defect reports, never empty, so a defect always
// reads as one.
func nonEmptyRead(read string) string {
	if read == "" {
		return `""`
	}
	return read
}

// positionOf is the position in the flow of the column with this identifier,
// and -1 where the flow carries none.
func positionOf(columns []*Column, id string) int {
	for i, column := range columns {
		if column.ID == id {
			return i
		}
	}
	return -1
}

// readHoldRule reads one kind entry, answering the rule, the defects it
// carries, and whether it is a rule at all, which it is only where it is a
// mapping whose held member reads.
func readHoldRule(entry jsonMember, columns []*Column, column func(ref string) *Column, commitment int) (HoldRule, []HoldDefect, bool) {
	rule := HoldRule{Kind: entry.name, WaitsFor: contract.HoldWaitsFinish}
	members, mapping := firstMembers(entry.value)
	if !mapping {
		return rule, []HoldDefect{{Defect: HoldsKindNotAMapping, Kind: entry.name, Read: scheduleRead(entry.value)}}, false
	}
	var defects []HoldDefect
	malformed := func(member string, read string) {
		defects = append(defects, HoldDefect{Defect: HoldsMalformedMember, Kind: entry.name, Member: member, Read: read})
	}
	held := false
	var finishAt json.RawMessage
	for _, member := range members {
		switch member.name {
		case holdHeldMember:
			value, ok := holdWord(member.value, contract.HoldHeldNamed, contract.HoldHeldCarrier)
			if !ok {
				malformed(member.name, nonEmptyRead(scheduleRead(member.value)))
				continue
			}
			rule.Held, held = value, true
		case holdWaitsForMember:
			value, ok := holdWord(member.value, contract.HoldWaitsStart, contract.HoldWaitsFinish)
			if !ok {
				malformed(member.name, nonEmptyRead(scheduleRead(member.value)))
				continue
			}
			rule.WaitsFor = value
		case holdLagDaysMember:
			// A lag takes the spelling a soon window takes: a whole decimal
			// from 0 to 365, where a quoted number is a string and refused.
			days, ok := soonDays(member.value)
			if !ok {
				malformed(member.name, strings.TrimSpace(string(member.value)))
				continue
			}
			rule.LagDays = days
		case holdFinishAtMember:
			finishAt = member.value
		default:
			defects = append(defects, HoldDefect{Defect: HoldsUnknownMember, Kind: entry.name, Member: member.name, Read: scheduleRead(member.value)})
		}
	}
	if !held {
		// A held member that was present and unreadable has already been
		// reported; one left out is reported here. Nothing can guess which
		// end is held, so the kind holds nothing either way.
		if !hasMember(members, holdHeldMember) {
			malformed(holdHeldMember, `""`)
		}
		return rule, defects, false
	}
	// finish_at is read once waits_for is known, because the two members
	// may come in either order and the first only means something beside
	// the second's finish.
	if finishAt != nil {
		var ref string
		read := nonEmptyRead(scheduleRead(finishAt))
		named := (*Column)(nil)
		if err := json.Unmarshal(finishAt, &ref); err == nil && ref != "" {
			named = column(ref)
		}
		switch {
		case rule.WaitsFor == contract.HoldWaitsStart:
			malformed(holdFinishAtMember, holdReadStartOnly)
		case named == nil:
			malformed(holdFinishAtMember, read)
		case commitment >= 0 && positionOf(columns, named.ID) < commitment:
			malformed(holdFinishAtMember, holdReadBeforeStart)
		default:
			rule.FinishAt = named.ID
		}
	}
	return rule, defects, true
}

// holdWord reads a member that takes one of two words, spelled exactly.
func holdWord(raw json.RawMessage, first, second string) (string, bool) {
	var word string
	if err := json.Unmarshal(raw, &word); err != nil {
		return "", false
	}
	if word != first && word != second {
		return "", false
	}
	return word, true
}

// hasMember reports whether a block carries a member of this name.
func hasMember(members []jsonMember, name string) bool {
	for _, member := range members {
		if member.name == name {
			return true
		}
	}
	return false
}

// Holds answers the workbench's hold settings and every defect its block
// carries. The settings are always usable, because a defect has already
// fallen back to the member's default or left the kind out.
func (b *Bench) Holds() (HoldSettings, []HoldDefect) {
	settings := b.holds
	settings.Rules = append([]HoldRule(nil), b.holds.Rules...)
	return settings, append([]HoldDefect(nil), b.holdsDefects...)
}

// Committed reports whether a column lies in the committed set: the
// commitment column, every column after it in the flow, and every done
// column. The flow is the whole flow rather than any card's route, so a
// route that skips the commitment column still passes it. A nil column, which
// is how a reader passes an identifier the workbench no longer declares, lies
// outside the set.
func (b *Bench) Committed(column *Column) bool {
	if column == nil {
		return false
	}
	if column.Terminal() {
		return true
	}
	commitment := b.Column(b.holds.StartAt)
	return commitment != nil && column.Position >= commitment.Position
}

// FinishingPoint reports whether a column lies in a rule's finishing point:
// every done column, and where the rule names a finish_at, that column and
// every column the flow lists after it. A nil column lies outside it.
func (b *Bench) FinishingPoint(rule HoldRule, column *Column) bool {
	if column == nil {
		return false
	}
	if column.Terminal() {
		return true
	}
	if rule.FinishAt == "" {
		return false
	}
	finish := b.Column(rule.FinishAt)
	return finish != nil && column.Position >= finish.Position
}

// Started reports whether card has started, on the two clauses of the holds'
// definition: it stands in the committed set now, or somebody holds it now. A
// claim whose expiry has passed at now is no claim, whether or not a read has
// yet lapsed it on disk, so every reader asking with its request's one clock
// reading answers alike. It reads the card's column and state and no journal.
func (b *Bench) Started(card *Card, now time.Time) bool {
	if b.Committed(b.Column(card.Column)) {
		return true
	}
	return card.State == contract.StateActive && !card.Lapsed(now)
}

// StartDay is the day card became started as it is now, YYYY-MM-DD in the
// workbench's zone: the most recent crossing into the committed set for a
// card standing in it, the most recent claim for a card active outside it,
// and empty where neither is recorded or card has not started.
func (b *Bench) StartDay(card *Card, events []Event, now time.Time) string {
	if !b.Started(card, now) {
		return ""
	}
	if b.Committed(b.Column(card.Column)) {
		return b.crossingDay(events, b.Committed)
	}
	stamp := ""
	for _, event := range events {
		if event.Event == contract.EventClaimed {
			stamp = event.TS
		}
	}
	return b.dayText(stamp)
}

// Finished reports whether card stands in rule's finishing point now. It
// reads the card's column and no journal, so a card moved back out of the
// finishing point has not finished.
func (b *Bench) Finished(card *Card, rule HoldRule) bool {
	return b.FinishingPoint(rule, b.Column(card.Column))
}

// FinishDay is the day of the most recent crossing into rule's finishing
// point, empty where none is recorded.
func (b *Bench) FinishDay(card *Card, rule HoldRule, events []Event) string {
	return b.crossingDay(events, func(column *Column) bool {
		return b.FinishingPoint(rule, column)
	})
}

// Reached is Started where rule waits for a start and Finished where it
// waits for a finish.
func (b *Bench) Reached(holder *Card, rule HoldRule, now time.Time) bool {
	if rule.WaitsFor == contract.HoldWaitsStart {
		return b.Started(holder, now)
	}
	return b.Finished(holder, rule)
}

// DayReached is StartDay or FinishDay on the terms Reached reads.
func (b *Bench) DayReached(holder *Card, rule HoldRule, events []Event, now time.Time) string {
	if rule.WaitsFor == contract.HoldWaitsStart {
		return b.StartDay(holder, events, now)
	}
	return b.FinishDay(holder, rule, events)
}

// crossingDay is the day of the most recent journal event that placed the
// card inside a set of columns from outside it: a moved event whose from lies
// outside and whose to lies inside, or a created event whose to lies inside.
// A move between two columns that both lie inside is no crossing. It is empty
// where no crossing is recorded.
func (b *Bench) crossingDay(events []Event, inside func(*Column) bool) string {
	stamp := ""
	for _, event := range events {
		switch event.Event {
		case contract.EventMoved:
			if inside(b.Column(event.To)) && !inside(b.Column(event.From)) {
				stamp = event.TS
			}
		case contract.EventCreated:
			if inside(b.Column(event.To)) {
				stamp = event.TS
			}
		}
	}
	return b.dayText(stamp)
}

// dayText is a journal stamp's day as YYYY-MM-DD, empty where the stamp is
// empty or does not parse.
func (b *Bench) dayText(stamp string) string {
	day, ok := b.DayOf(stamp)
	if !ok {
		return ""
	}
	return day.String()
}

// DayOf reads an RFC 3339 journal stamp as the calendar day it falls on in
// the workbench's zone, answering false for a stamp that does not parse.
func (b *Bench) DayOf(stamp string) (Date, bool) {
	moment := ParseStamp(stamp)
	if moment.IsZero() {
		return Date{}, false
	}
	return b.Today(moment), true
}

// HoldEdge is one holding link between two cards: the card it holds, the card
// it waits on, and the rule that made it a hold.
type HoldEdge struct {
	Held   string // card identifier
	Holder string // card identifier
	Rule   HoldRule
}

// HoldEdges is every edge the declared rules make of the links the given
// cards carry, in the order the cards are given and, within a card, the
// order its links are stored. Every caller passes the live cards, so a link
// an archived card carries is part of that card's record and holds nothing.
// An edge whose two ends are one card is kept, and it is a cycle.
func HoldEdges(cards []*Card, rules []HoldRule) []HoldEdge {
	if len(rules) == 0 {
		return nil
	}
	byKind := map[string]HoldRule{}
	for _, rule := range rules {
		byKind[rule.Kind] = rule
	}
	var edges []HoldEdge
	for _, card := range cards {
		for _, link := range card.Links {
			rule, ok := byKind[link.Kind]
			if !ok {
				continue
			}
			edge := HoldEdge{Held: link.To, Holder: card.ID, Rule: rule}
			if rule.Held == contract.HoldHeldCarrier {
				edge.Held, edge.Holder = card.ID, link.To
			}
			edges = append(edges, edge)
		}
	}
	return edges
}

// AwaitingEdges is the edges among cards whose ground is awaiting now: both
// ends are among the cards, the held card is outside a done column and has
// not started, and the holder has not reached the rule's event. Both are
// readings of position and state, so it reads no journal and no date.
func (b *Bench) AwaitingEdges(cards []*Card, rules []HoldRule, now time.Time) []HoldEdge {
	byID := map[string]*Card{}
	for _, card := range cards {
		byID[card.ID] = card
	}
	var awaiting []HoldEdge
	for _, edge := range HoldEdges(cards, rules) {
		held, holder := byID[edge.Held], byID[edge.Holder]
		if held == nil || holder == nil {
			continue
		}
		if column := b.Column(held.Column); column != nil && column.Terminal() {
			continue
		}
		if b.Started(held, now) || b.Reached(holder, edge.Rule, now) {
			continue
		}
		awaiting = append(awaiting, edge)
	}
	return awaiting
}

// HoldCycles is every cycle of awaiting edges among cards: each strongly
// connected set of them holding more than one card, or one card with an edge
// to itself. Each is answered once, as its cards in edge order starting from
// the lowest-numbered, and the cycles are ordered by that first card's
// number.
func (b *Bench) HoldCycles(cards []*Card, rules []HoldRule, now time.Time) [][]*Card {
	edges := b.AwaitingEdges(cards, rules, now)
	if len(edges) == 0 {
		return nil
	}
	byID := map[string]*Card{}
	for _, card := range cards {
		byID[card.ID] = card
	}
	next := map[string][]string{}
	selfLoop := map[string]bool{}
	var nodes []string
	seen := map[string]bool{}
	for _, edge := range edges {
		next[edge.Held] = append(next[edge.Held], edge.Holder)
		if edge.Held == edge.Holder {
			selfLoop[edge.Held] = true
		}
		for _, id := range []string{edge.Held, edge.Holder} {
			if !seen[id] {
				seen[id] = true
				nodes = append(nodes, id)
			}
		}
	}
	var cycles [][]*Card
	for _, component := range stronglyConnected(nodes, next) {
		if len(component) == 1 && !selfLoop[component[0]] {
			continue
		}
		cycles = append(cycles, cycleOrder(component, next, byID))
	}
	sort.SliceStable(cycles, func(i, j int) bool {
		return cycles[i][0].Number < cycles[j][0].Number
	})
	return cycles
}

// HoldCycleThrough is the cycle of awaiting edges the edge from held to
// holder closes, as its cards in edge order starting from held, and nil where
// it closes none. An edge from a card to itself is a cycle of that card
// alone. The path back from holder to held is the shortest one, so the
// warning a link carries names the fewest cards that make the cycle.
func HoldCycleThrough(edges []HoldEdge, held, holder string) []string {
	if held == holder {
		return []string{held}
	}
	next := map[string][]string{}
	for _, edge := range edges {
		next[edge.Held] = append(next[edge.Held], edge.Holder)
	}
	previous := map[string]string{holder: ""}
	queue := []string{holder}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		for _, to := range next[at] {
			if _, visited := previous[to]; visited {
				continue
			}
			previous[to] = at
			if to == held {
				var back []string
				for step := at; step != ""; step = previous[step] {
					back = append(back, step)
				}
				path := []string{held}
				for i := len(back) - 1; i >= 0; i-- {
					path = append(path, back[i])
				}
				return path
			}
			queue = append(queue, to)
		}
	}
	return nil
}

// stronglyConnected is Tarjan's partition of the nodes into strongly
// connected sets, each set in the order the walk met its members.
func stronglyConnected(nodes []string, next map[string][]string) [][]string {
	index := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var components [][]string
	counter := 0
	var visit func(string)
	visit = func(node string) {
		index[node], low[node] = counter, counter
		counter++
		stack = append(stack, node)
		onStack[node] = true
		for _, to := range next[node] {
			if _, visited := index[to]; !visited {
				visit(to)
				if low[to] < low[node] {
					low[node] = low[to]
				}
			} else if onStack[to] && index[to] < low[node] {
				low[node] = index[to]
			}
		}
		if low[node] != index[node] {
			return
		}
		var component []string
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			component = append(component, top)
			if top == node {
				break
			}
		}
		components = append(components, component)
	}
	for _, node := range nodes {
		if _, visited := index[node]; !visited {
			visit(node)
		}
	}
	return components
}

// cycleOrder lists a strongly connected set's cards in edge order: from its
// lowest-numbered card, each card once, in the order a walk along the edges
// that stay inside the set first meets it.
func cycleOrder(component []string, next map[string][]string, byID map[string]*Card) []*Card {
	inside := map[string]bool{}
	start := component[0]
	for _, id := range component {
		inside[id] = true
		if byID[id].Number < byID[start].Number {
			start = id
		}
	}
	var ordered []*Card
	visited := map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		visited[id] = true
		ordered = append(ordered, byID[id])
		for _, to := range next[id] {
			if inside[to] && !visited[to] {
				walk(to)
			}
		}
	}
	walk(start)
	return ordered
}

// HoldsMigration is the account dinah check --migrate-holds answers with.
type HoldsMigration struct {
	// From is the format the workbench declared before the run.
	From int `json:"from"`
	// Stamped is true where the anchor was written with HoldsFormat.
	Stamped bool `json:"stamped"`
	// Preview is true where the run carried no confirmation, so it wrote
	// nothing whatever the format was.
	Preview bool `json:"preview,omitempty"`
}

// MigrateHolds stamps HoldsFormat on a workbench declaring a lower format,
// and does nothing to one already at or above it, so the stamp is a floor and
// never a downgrade. Without apply it classifies and writes nothing. It
// follows MigrateSchedule in every respect: one write to the workbench
// anchor, no card read or written, because a link written below the format
// reads the same above it, and the same refusal below DesignationFormat.
func (b *Bench) MigrateHolds(apply bool) (*HoldsMigration, error) {
	if b.Format < DesignationFormat {
		return nil, contract.Refuse(contract.StoreAwaitingMigration, b.Root)
	}
	report := &HoldsMigration{From: b.Format, Preview: !apply}
	if !apply || b.Format >= HoldsFormat {
		return report, nil
	}
	if err := b.stampFormat(HoldsFormat); err != nil {
		return nil, err
	}
	report.Stamped = true
	return report, nil
}
