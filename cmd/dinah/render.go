package main

import (
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/verb"
)

// emit reports a verb's canonical response: a machine form where one was
// asked for, the rendering otherwise, and on any non-zero outcome the
// outcome's own token leading stderr.
func (s *session) emit(response *verb.Response) int {
	if response.Outcome != contract.OutcomeOK {
		s.reportOutcome(response)
	}
	if s.format != formatHuman {
		// emitMachine carries the outcome's own exit code back.
		return s.emitMachine(response)
	}
	if response.Outcome != contract.OutcomeOK {
		return contract.ExitCode(response.Outcome)
	}
	if response.Warning != "" {
		io.WriteString(s.errw, s.r.T(response.Warning, "detail", response.WarningDetail)+"\n")
	}
	if response.Card == nil && response.Message != "" {
		values := flattenMessageValues(response.MessageValues)
		s.line(s.r.T(response.Message, values...))
		return 0
	}
	// A response about a workstream draws the workstream's own line, which is
	// what the workstream verbs have always printed. It rides here rather than
	// in a second emitter because `dinah set` reaches every kind through one
	// reference, so which line to draw is a fact about the answer rather than
	// about the command that asked.
	if response.Card == nil && response.Workstream != nil {
		s.renderWorkstreamLine(response.Workstream)
		return 0
	}
	s.renderCard(response.Card)
	if response.Instructions != nil && !s.quiet {
		s.renderInstructions(response.Instructions, response.LegalMoves, response.Loop)
	}
	return 0
}

// reportOutcome writes the leading token and the sentence a person reads.
// For a refusal the token is the refusal name; for the other two non-zero
// outcomes it is the outcome name itself.
//
// A refusal a verb answered with goes through nameTheWorkbench exactly as one
// the open raised does, because advice that needs a workbench named needs it
// whichever half of the head composed the sentence. Four of the six entries in
// that table are refused by a verb rather than by the open, so leaving this
// path out would have left their scoped advice declared and never rendered.
func (s *session) reportOutcome(response *verb.Response) {
	switch response.Outcome {
	case contract.OutcomeRefused:
		refused := contract.RefuseWith(response.Refusal, response.Detail, s.outcomeValues(response))
		for _, line := range s.composeRefusal(s.nameTheWorkbench(refused)) {
			io.WriteString(s.errw, line+"\n")
		}
	case contract.OutcomeStale:
		revision := ""
		if response.Card != nil {
			revision = response.Card.Revision
		}
		io.WriteString(s.errw, contract.OutcomeStale+" "+s.r.T("outcome.stale", "revision", revision)+"\n")
	default:
		io.WriteString(s.errw, response.Outcome+" "+s.r.T("outcome.unreachable", "detail", response.Detail)+"\n")
	}
}

// emitCanonical writes a value as the canonical machine form. The form carries
// canonical tokens only, so the same command under any language setting emits
// byte-identical JSON.
func (s *session) emitCanonical(value any) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		io.WriteString(s.errw, contract.OutcomeUnreachable+" "+err.Error()+"\n")
		return contract.ExitCode(contract.OutcomeUnreachable)
	}
	io.WriteString(s.out, string(data)+"\n")
	return 0
}

// renderCard prints the one line a person needs after an act: where the card
// is, what it is called, what column it is in, and which workstreams it belongs
// to.
//
// A card belonging to at least one workstream draws from a sibling key
// carrying the whole sentence with the trailing field in it, chosen here the
// way check.count.one and check.count.other are chosen by their caller, so a
// translator gets a whole sentence in each form rather than a fragment
// concatenated onto one. This is the single site every act prints its card
// line from, so the field appears after claim, move, release, block, unblock,
// add and show as well as after join and leave.
func (s *session) renderCard(card *verb.CardView) {
	if card == nil {
		return
	}
	values := []string{
		"ref", card.Ref,
		"title", card.Title,
		"column", card.ColumnTitle,
		"state", s.token(card.State),
	}
	key := "card.line"
	if len(card.Workstreams) > 0 {
		key = "card.line.workstreams"
		values = append(values, "workstreams", s.workstreamsCell(card.Workstreams))
	}
	s.line(s.r.T(key, values...))
	// A value kept on a slot that no longer applies to the card is drawn in
	// place of the slot's ordinary line, on every surface that draws card
	// lines, because the ordinary line would present the value as though the
	// question had been asked of this card.
	if card.Severity != "" {
		s.slotLine(card, bench.SeverityField, card.Severity, s.r.T("card.severity", "severity", card.Severity))
	}
	if card.Priority != "" {
		s.slotLine(card, bench.PriorityField, card.Priority, s.r.T("card.priority", "priority", card.Priority))
	}
	// The route stands with the levels and before the declared fields, drawn
	// only where the card carries one, because a card on the workbench's full
	// column list has no road worth naming.
	if card.Route != "" {
		s.line(s.r.T("card.route", "route", card.Route))
	}
	// A standing criterion-retirement grant stands with the route, drawn only
	// where the card carries one, because somebody deciding whether to tidy a
	// card has to learn that the permission exists before they try rather
	// than by meeting a refusal.
	if card.RetirementGrant != "" {
		s.line(s.r.T("card.retirement-grant", "column", card.RetirementGrant, "title", card.RetirementGrantTitle))
	}
	// The declared fields stand after the two levels and before the holder,
	// in the order the workbench declares them. A declared field the card does
	// not carry draws no line, and a key the card stores that the workbench
	// does not declare never reaches the view, because printing it would make
	// an undeclared key look like a supported one.
	for _, key := range s.declaredFieldOrder() {
		if value, carried := card.Fields[key]; carried {
			s.slotLine(card, key, value, s.r.T("card.field", "field", key, "value", value))
		}
	}
	if card.Holder != "" {
		s.line(s.r.T("card.holder", "holder", card.Holder))
	}
	if card.BlockReason != "" {
		s.line(s.r.T("card.blocked", "reason", card.BlockReason))
	}
}

// slotLine draws one stored value's line: the ordinary line where the slot
// applies to the card, and the kept-although-inapplicable sentence where the
// view lists the slot as not applying, naming the gate and what it stores.
func (s *session) slotLine(card *verb.CardView, slot, stored, ordinary string) {
	excluded := inapplicableSlot(card, slot)
	if excluded == nil {
		s.line(ordinary)
		return
	}
	if excluded.GateValue == "" {
		s.line(s.r.T("card.inapplicable.stored-unset", "field", slot, "stored", stored, "gate", excluded.Gate))
		return
	}
	s.line(s.r.T("card.inapplicable.stored", "field", slot, "stored", stored, "gate", excluded.Gate, "value", excluded.GateValue))
}

// inapplicableSlot reports the view's entry for one slot it lists as not
// applying to the card, and nil where the slot applies.
func inapplicableSlot(card *verb.CardView, slot string) *verb.InapplicableView {
	for i := range card.Inapplicable {
		if card.Inapplicable[i].Field == slot {
			return &card.Inapplicable[i]
		}
	}
	return nil
}

// storesSlot reports whether a view carries a value for one slot, which is a
// level off the view's own two members and a declared key off its fields.
func storesSlot(card *verb.CardView, slot string) bool {
	switch slot {
	case bench.SeverityField:
		return card.Severity != ""
	case bench.PriorityField:
		return card.Priority != ""
	}
	_, carried := card.Fields[slot]
	return carried
}

// renderInapplicable draws one line for each slot the view lists as not
// applying to the card and that the card stores nothing for. It is drawn by
// show alone: the card line after an act says what the card carries, and a
// line per slot that does not apply would repeat on every act for every card
// the condition excludes.
func (s *session) renderInapplicable(card *verb.CardView) {
	for _, excluded := range card.Inapplicable {
		if storesSlot(card, excluded.Field) {
			continue
		}
		if excluded.GateValue == "" {
			s.line(s.r.T("card.inapplicable.unset", "field", excluded.Field, "gate", excluded.Gate))
			continue
		}
		s.line(s.r.T("card.inapplicable", "field", excluded.Field, "gate", excluded.Gate, "value", excluded.GateValue))
	}
}

// declaredFieldOrder is the keys of the fields this workbench declares, in
// declaration order, which is the order an entity's own values print in.
//
// It reads the open workbench rather than the view, because declaration order
// is a property of the workbench and the view carries a mapping. A session
// with no workbench open draws no declared field, which is right: a response
// composed before a workbench opened carries no declared value either.
func (s *session) declaredFieldOrder() []string {
	if s.library == nil {
		return nil
	}
	declared := s.library.Bench.DeclaredFields()
	keys := make([]string, 0, len(declared))
	for _, field := range declared {
		keys = append(keys, field.Key)
	}
	return keys
}

// flattenMessageValues turns a Message's named values into the variadic pair
// list Renderer.T takes. The keys are sorted so that one map produces one
// argument list however the runtime happens to walk it.
func flattenMessageValues(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, 2*len(values))
	for _, k := range keys {
		out = append(out, k, values[k])
	}
	return out
}

// token renders a machine token for a person, which CORE-TEXT-4 permits where
// a person reads it and CORE-TEXT-3 forbids on the machine surface. A token
// with no rendering shows its canonical spelling.
func (s *session) token(name string) string {
	key := "token." + name
	if !s.r.Has(key) {
		return name
	}
	return s.r.T(key)
}

// renderInstructions prints the three layers as labelled blocks, in the order
// the chain serves them, then the listing of the column's attachments where
// the chain carries one, with the legal moves after and the loop standing
// last. The loop line is printed only where the column declares a loop_limit,
// so a reader at an ordinary station sees nothing new, and it is printed
// whether or not there are moves to print above it, because a card at its
// limit has fewer moves rather than more and the line is what says why.
func (s *session) renderInstructions(instructions *verb.Instructions, moves []verb.LegalMove, loop *verb.Loop) {
	layers := []struct {
		label string
		text  string
	}{
		{label: "instructions.global", text: instructions.Global},
		{label: "instructions.standing", text: instructions.Standing},
		{label: "instructions.column", text: instructions.Column},
	}
	for _, layer := range layers {
		if layer.text == "" {
			continue
		}
		s.line("")
		s.line(s.r.T(layer.label))
		s.write(layer.text)
	}
	if len(instructions.ColumnAttachments) > 0 {
		s.line("")
		s.line(s.r.T("instructions.column-attachments"))
		s.renderAttachments(instructions.ColumnAttachments)
		s.line(s.r.T("instructions.column-attachments.read"))
	}
	if len(moves) > 0 {
		s.line("")
		s.line(s.r.T("instructions.moves"))
		// On route stands between Direction and Reject, because it narrows
		// what Direction says and a reader meets the two together: the row
		// marked here is the forward move along this card's own road, where
		// Direction answers for the flow whatever road the card walks.
		t := table{indent: 2, columns: s.columns("moves", "column", "name", "direction", "route", "reject")}
		for _, move := range moves {
			fields := []string{move.Ref, move.Title, s.token(move.Direction), s.yesNo(move.OnRoute), s.yesNo(move.Reject)}
			t.rows = append(t.rows, tableRow{fields: fields})
		}
		s.table(t)
	}
	if loop == nil {
		return
	}
	s.line("")
	s.line(s.r.T("instructions.loop",
		"count", strconv.Itoa(loop.Count),
		"limit", strconv.Itoa(loop.Limit),
		"column", loop.Column))
}

// renderPrime prints dinah prime's answer: the workbench line, who the
// caller is, what it holds, what is ready for it, what is pending for it,
// and the standing instructions, on the terms the caller asked for them.
//
// The three sections below draw one row per line rather than a table.table:
// each carries a shape of its own (the ready count and the checklist kind,
// both new to this surface) that a genuine table would need new headings
// for, and each row is still laid out through row/formatRow, the one place
// this head pads a field, rather than by hand.
func (s *session) renderPrime(primer *verb.Primer, brief bool) {
	s.line(s.workbenchLine(&verb.Status{Bench: primer.Bench, Root: primer.Root, WorkbenchSource: primer.WorkbenchSource}))
	s.line(s.r.T("prime.actor", "actor", primer.Identity.Actor, "operator", s.yesNo(primer.Identity.IsOperator)))
	s.line("")
	s.renderPrimeHolding(primer.Holding)
	s.line("")
	s.renderPrimeReady(primer.Ready)
	s.line("")
	s.renderPrimePending(primer)
	s.line("")
	s.renderPrimeInstructions(primer, brief)
}

// primeRow prints one row at table.go's own indentedLine, since these three
// sections vary in shape row to row rather than sharing declared columns
// and building a row.row directly belongs to table.go alone.
func (s *session) primeRow(text string) {
	s.line(s.indentedLine(text))
}

func (s *session) renderPrimeHolding(holding []verb.CardView) {
	if len(holding) == 0 {
		s.line(s.r.T("prime.holding.none"))
		return
	}
	s.line(s.r.T("prime.holding"))
	for _, card := range holding {
		s.primeRow(card.Ref + ": " + card.Title)
	}
}

func (s *session) renderPrimeReady(ready []verb.Offer) {
	if len(ready) == 0 {
		s.line(s.r.T("prime.ready.none"))
		return
	}
	s.line(s.r.T("prime.ready"))
	for _, offer := range ready {
		count := strconv.Itoa(offer.ReadyCount)
		if offer.Card != nil {
			s.primeRow(offer.Title + ": " + count + " ready, " + offer.Card.Ref + ": " + offer.Card.Title)
			continue
		}
		s.primeRow(offer.Title + ": " + s.r.T("next.above-tier"))
	}
}

func (s *session) renderPrimePending(primer *verb.Primer) {
	if len(primer.Pending) == 0 {
		s.line(s.r.T("prime.pending.none"))
		return
	}
	total := len(primer.Pending) + primer.PendingWithheld
	if primer.PendingWithheld > 0 {
		s.line(s.r.T("prime.pending.truncated",
			"shown", strconv.Itoa(len(primer.Pending)),
			"total", strconv.Itoa(total),
			"withheld", strconv.Itoa(primer.PendingWithheld)))
	} else {
		s.line(s.r.T("prime.pending"))
	}
	for _, item := range primer.Pending {
		s.primeRow(item.Ref + " (" + s.token(item.Kind) + "): " + item.Text)
	}
	if len(primer.PendingByColumn) > 0 {
		parts := make([]string, 0, len(primer.PendingByColumn))
		for _, column := range primer.PendingByColumn {
			parts = append(parts, column.ColumnTitle+" "+strconv.Itoa(column.Count))
		}
		s.primeRow(s.r.T("prime.pending.by-column", "columns", strings.Join(parts, ", ")))
	}
	if primer.PendingWithheld > 0 {
		s.primeRow(s.r.T("prime.pending.full-pending-hint", "total", strconv.Itoa(total)))
	}
}

// renderPrimeInstructions prints the standing instructions block: the
// Global and Standing layers in full on an ordinary call, or, where the
// caller asked for the narrow form, one line saying so and the command
// that recovers them.
//
// brief is the caller's own --brief flag rather than anything read off
// Withheld, because Withheld is empty both when the caller asked for the
// full form and got it and when the caller asked for the narrow form on a
// workbench carrying no text to withhold in the first place (layer's own
// empty-text skip), and only the caller's own flag tells those two apart.
func (s *session) renderPrimeInstructions(primer *verb.Primer, brief bool) {
	instructions := primer.Instructions
	if brief {
		s.line(s.r.T("prime.instructions.brief"))
		s.line(s.r.T("prime.instructions.brief.hint", "example", s.primeInstructionsExample(primer)))
		return
	}
	layers := []struct {
		label string
		text  string
	}{
		{label: "instructions.global", text: instructions.Global},
		{label: "instructions.standing", text: instructions.Standing},
	}
	drawn := false
	for _, layer := range layers {
		if layer.text == "" {
			continue
		}
		if drawn {
			s.line("")
		}
		s.line(s.r.T(layer.label))
		s.write(layer.text)
		drawn = true
	}
	s.line("")
	s.line(s.r.T("prime.instructions.hint"))
}

// primeInstructionsExample names a column reference for the recovery
// command's own example: the column of the caller's earliest-arrival held
// card where it holds one, and the workbench's first declared column
// otherwise.
//
// primer.Holding carries no arrival information of its own (it is built in
// Bench.Cards()' own directory-listing order, sorted by the card's random
// ID rather than by when it arrived, matching Status.Holding), so this
// re-reads the bench's cards and picks the earliest-arrival one by
// bench.ByArrival, the same rule Library.Prime's own Reread member uses.
func (s *session) primeInstructionsExample(primer *verb.Primer) string {
	if len(primer.Holding) > 0 && s.library != nil {
		if ref := s.earliestHeldColumnRef(primer.Identity.Actor); ref != "" {
			return ref
		}
	}
	if s.library != nil && len(s.library.Bench.Columns) > 0 {
		return s.library.Bench.Columns[0].Ref()
	}
	return ""
}

// earliestHeldColumnRef finds the column of the earliest-arrival card actor
// holds, by bench.ByArrival, empty where the bench cannot be read or actor
// holds nothing.
func (s *session) earliestHeldColumnRef(actor string) string {
	cards, err := s.library.Bench.Cards()
	if err != nil {
		return ""
	}
	var earliest *bench.Card
	for _, card := range cards {
		if card.Holder != actor {
			continue
		}
		if earliest == nil || bench.ByArrival(card, earliest) {
			earliest = card
		}
	}
	if earliest == nil {
		return ""
	}
	if column := s.library.Bench.Column(earliest.Column); column != nil {
		return column.Ref()
	}
	return ""
}

// renderStatus prints where the bench stands.
func (s *session) renderStatus(status *verb.Status) {
	s.line(s.workbenchLine(status))
	if status.Actor == "" {
		// A process with no actor resolved is not meaningfully "not the
		// operator" any more than it is the operator; it has named nobody.
		// Interpolating the empty string into status.actor would print
		// "acting as , operator: no", which is uninformative and reads as a
		// claim this line is not making, so an unresolved actor gets its
		// own line instead.
		s.line(s.r.T("status.actor.unnamed"))
	} else {
		s.line(s.r.T("status.actor", "actor", status.Actor, "operator", s.yesNo(status.IsOperator)))
	}
	s.line("")
	s.renderColumns(status.Columns)
	if len(status.Holding) > 0 {
		s.line("")
		s.line(s.r.T("status.holding"))
		held := table{indent: 2, columns: s.columns("holding", "card", "title")}
		for _, card := range status.Holding {
			held.rows = append(held.rows, tableRow{fields: []string{card.Ref, card.Title}})
		}
		s.table(held)
	}
	if len(status.Blocked) > 0 {
		s.line("")
		s.line(s.r.T("status.blocked"))
		blocked := table{indent: 2, columns: s.columns("blocked", "card", "reason")}
		for _, card := range status.Blocked {
			blocked.rows = append(blocked.rows, tableRow{fields: []string{card.Ref, card.BlockReason}})
		}
		s.table(blocked)
	}
}

// workbenchLine is the first line of a status: the workbench, where it was
// discovered, and the rung that resolved it.
//
// A root-scoped read resolves no single workbench by any rung, since the walk
// found every one of them, so its answer carries no source and this line leaves
// the bracket off rather than drawing an empty one. The two forms are separate
// catalog entries rather than one entry with a blank in it, because a bracket
// with nothing inside reads as a value that failed to render.
func (s *session) workbenchLine(status *verb.Status) string {
	if status.WorkbenchSource == "" {
		return s.r.T("status.workbench.unsourced",
			"title", status.Bench,
			"root", status.Root,
		)
	}
	return s.r.T("status.workbench",
		"title", status.Bench,
		"root", status.Root,
		"source", s.token(status.WorkbenchSource),
	)
}

// yesNo renders a boolean for a person.
func (s *session) yesNo(value bool) string {
	if value {
		return s.r.T("word.yes")
	}
	return s.r.T("word.no")
}

// renderColumns prints the flow in order with each station's occupancy.
func (s *session) renderColumns(columns []verb.ColumnView) {
	t := table{indent: 2, columns: s.columns("columns", "slug", "name", "kind", "cards", "work", "owner")}
	for _, column := range columns {
		count := strconv.Itoa(column.Count)
		if column.Capacity > 0 {
			count += "/" + strconv.Itoa(column.Capacity)
		}
		owner := s.r.T("columns.moved-by.agent")
		if column.OperatorOwned {
			owner = s.r.T("columns.moved-by.operator")
		}
		// The Work cell answers whether work is taken up at the column and
		// the Owner cell answers who may move a card out of it. They are two
		// questions, so they are two cells, and a column where nobody takes
		// work up and nobody owner-owns still reads agent under Owner.
		//
		// The three values run most specific first. A column declaring the
		// flag reads waiting, because a reader is told the workbench is
		// waiting on somebody; a column where no owner takes work up for any
		// other reason reads none taken; every other column reads taken.
		work := s.r.T("columns.work.taken")
		if !column.TakesWorkUp {
			work = s.r.T("columns.work.none")
		}
		if column.AwaitingOutside {
			work = s.r.T("columns.work.waiting")
		}
		fields := []string{s.slugCell(column.Slug), column.Title, s.token(column.Kind), count, work, owner}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// renderRosters prints the workbench's top-level collections, each with the
// count of what it holds. The Reference cell is the word a reader types back,
// which is what makes the advice a refusal gives true: a reader told to run
// `dinah list` meets, in that answer, every word the next question needs.
func (s *session) renderRosters(listing *verb.RosterListing) {
	t := table{indent: 2, columns: s.columns("roster", "reference", "holds", "count")}
	for _, roster := range listing.Rosters {
		t.rows = append(t.rows, tableRow{fields: []string{
			roster.Reference,
			s.token(roster.Holds),
			strconv.Itoa(roster.Count),
		}})
	}
	s.table(t)
}

// renderListing prints a column's cards in queue order.
func (s *session) renderListing(listing *verb.Listing) {
	if len(listing.Cards) == 0 {
		s.line(s.r.T("queue.empty"))
		return
	}
	t := table{indent: 2, columns: s.columns("queue", "card", "standing", "severity", "priority", "title")}
	for _, card := range listing.Cards {
		severity := s.levelCell(&card, bench.SeverityField, card.Severity)
		priority := s.levelCell(&card, bench.PriorityField, card.Priority)
		t.rows = append(t.rows, tableRow{fields: []string{card.Ref, s.token(card.State), severity, priority, card.Title}})
	}
	s.table(t)
}

// levelCell is what a listing's level column shows for one card: the stored
// value as stored, which is how a level the workbench stopped declaring
// already prints, and the not-applicable mark where the axis does not apply to
// the card and it stores nothing there.
func (s *session) levelCell(card *verb.CardView, axis, stored string) string {
	if stored == "" && inapplicableSlot(card, axis) != nil {
		return s.r.T("queue.cell.inapplicable")
	}
	return stored
}

// renderMatches prints the cards a query selected. A query spans the whole
// workbench where ls lists one column at a time, so the reader needs a column
// column that ls has no use for, and it carries the column's title rather than
// its identifier.
func (s *session) renderMatches(matches *verb.Matches) {
	if len(matches.Cards) == 0 {
		s.line(s.r.T("query.empty"))
		return
	}
	t := table{indent: 2, columns: s.columns("query", "card", "column", "standing", "title")}
	for _, card := range matches.Cards {
		fields := []string{card.Ref, card.ColumnTitle, s.token(card.State), card.Title}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// renderTree prints a projected tree: one sentence naming the root, and then a
// table starting at the root's children.
//
// The root is not a row. It is a given for the whole command rather than a
// finding, so a row for it would indent every other row by one level to say
// something the caller already typed, and the count it carries reads better in
// words than as a bare number beside a title.
func (s *session) renderTree(tree *verb.Tree) {
	s.line(s.treeHeader(tree))
	// The rows below an archived root carry the addresses the children will
	// have once the root is restored, and none of them resolves while it is
	// archived. Saying that once here is what keeps a screen of addresses
	// from quietly not working.
	if tree.Archived {
		s.line(s.r.T("contents.archived", "ref", tree.Root.Ref))
	}
	if len(tree.Root.Children) == 0 {
		return
	}
	t := table{indent: 2, columns: s.columns("tree", "reference", "entity", "title", "count", "hidden")}
	s.treeRows(&t, tree, tree.Root.Children, nil)
	s.table(t)
}

// withoutEmptyTitle applies the empty-title rule of the entity sentences: a
// title that is empty draws no leading space. The renderer omits the title
// and the space that follows it together rather than interpolating an empty
// string and keeping the space. Every locale template for the pair opens with
// `{title} ({ref})`, so an empty title leaves exactly one leading space, and
// dropping that one space is the whole of the omission.
func withoutEmptyTitle(sentence, title string) string {
	if title == "" {
		return strings.TrimPrefix(sentence, " ")
	}
	return sentence
}

// treeHeader is the sentence above the table. Under a filter it says what the
// workbench holds and how much of that matched, so the first number is the
// root's count added to what the filter removed and the second is the count
// alone.
func (s *session) treeHeader(tree *verb.Tree) string {
	root := tree.Root
	count := strconv.Itoa(root.Count)
	if tree.Producer == verb.ProducerContainment {
		// A collection has no title of its own, and both entity sentences
		// open with one, so the two collection sentences name the root by
		// its reference alone.
		if root.Kind == verb.KindCollection {
			if root.Count == 0 {
				return s.r.T("contents.empty.collection", "ref", root.Ref)
			}
			return s.r.T("contents.header.collection", "ref", root.Ref, "count", count)
		}
		if root.Count == 0 {
			return withoutEmptyTitle(s.r.T("contents.empty", "title", root.Title, "ref", root.Ref), root.Title)
		}
		return withoutEmptyTitle(s.r.T("contents.header", "title", root.Title, "ref", root.Ref, "count", count), root.Title)
	}
	if root.Hidden == nil || root.Hidden.Filtered == 0 {
		return s.r.T("tree.header", "title", root.Title, "ref", root.Ref, "count", count)
	}
	held := strconv.Itoa(root.Count + root.Hidden.Filtered)
	return s.r.T("tree.header.filtered", "title", root.Title, "ref", root.Ref, "held", held, "matched", count)
}

// treeRows appends one row per node, depth first, carrying the guides that
// place each row in the tree. The guides describe every level below the top
// level the table draws, so the root contributes none.
func (s *session) treeRows(t *table, tree *verb.Tree, nodes []verb.TreeNode, above []bool) {
	for i, node := range nodes {
		guides := append(append([]bool{}, above...), i == len(nodes)-1)
		t.rows = append(t.rows, tableRow{fields: s.treeFields(tree, node), guides: guides})
		s.treeRows(t, tree, node.Children, guides)
	}
}

// treeFields is one node's cells, in column order.
//
// The Reference column carries two things and the Entity column beside it is
// what says which: a row reading a value under Reference and an axis under
// Entity is a group rather than an entity, so the cell to its left is a value
// rather than an address. The Count cell is blank on a card under the grouped
// producer, because a card is one card and the number would tell the reader
// nothing.
func (s *session) treeFields(tree *verb.Tree, node verb.TreeNode) []string {
	if node.Kind == verb.NodeGroup {
		value := node.Value
		if value == "" {
			value = s.r.T("tree.unset")
		}
		return []string{value, node.Axis, node.Title, strconv.Itoa(node.Count), s.hiddenCell(node.Hidden)}
	}
	count := strconv.Itoa(node.Count)
	if tree.Subject == verb.SubjectCard {
		count = ""
	}
	return []string{node.Ref, node.Kind, node.Title, count, s.hiddenCell(node.Hidden)}
}

// hiddenCell renders what a node is not showing. The depth sentence prints the
// node's own direct children the depth did not draw, which is not the same
// number as the subjects those children hold, and the Count column beside it
// already carries the subjects.
func (s *session) hiddenCell(hidden *verb.Hidden) string {
	if hidden == nil {
		return ""
	}
	var parts []string
	for _, reason := range hidden.Reason {
		switch reason {
		case verb.ReasonDepth:
			parts = append(parts, s.r.T("tree.hidden.depth", "count", strconv.Itoa(hidden.Children)))
		case verb.ReasonFilter:
			parts = append(parts, s.r.T("tree.hidden.filter", "count", strconv.Itoa(hidden.Filtered)))
		}
	}
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	return s.r.T("tree.hidden.join", "first", parts[0], "second", parts[1])
}

// renderSettings prints each setting with the value in force and the rung of
// its ladder that produced it. A setting no rung carried prints an empty
// value beside the source that says so, because the row itself is the answer
// to whether anybody has ever set the key.
func (s *session) renderSettings(settings []verb.SettingView) {
	t := table{indent: 2, columns: s.columns("config", "setting", "value", "source")}
	for _, view := range settings {
		t.rows = append(t.rows, tableRow{fields: []string{view.Key, view.Value, s.token(view.Source)}})
	}
	s.table(t)
}

// renderWorkbenches prints one row per reachable workbench, and the line that
// says so when none is reachable. The row carries what a reader needs to
// recognise a workbench and to select it, so the path it ends on is the one
// --workbench takes.
func (s *session) renderWorkbenches(rows []bench.Candidate, root string) {
	if len(rows) == 0 {
		if root != "" {
			s.line(s.r.T("root.empty", "root", root))
			return
		}
		s.line(s.r.T("workbenches.empty"))
		return
	}
	for _, row := range s.formatCandidateRows(rows) {
		s.line(row)
	}
}

// formatCandidateRows renders each candidate as the padded title, slug and
// path columns the workbenches listing prints, one row per string with its own
// two-space lead. dinah.ambiguous-workbench prints the same rows beneath its
// opening sentence, so this is the one place the column widths live; the two
// callers can never draw the same candidates in different columns.
// A row the walk could not describe carries its refusal name in the workbench
// cell, in place of the title it has none of, and leaves the slug cell empty
// rather than printing the missing-slug repair, because a workbench nothing
// could read is a workbench whose slug is not worth deriving. A walk that
// refused before it described anything is drawn the same way, as one row
// naming that refusal with no path beside it, so the reason a listing is
// missing reaches a person in the place the listing would have stood.
func (s *session) formatCandidateRows(rows []bench.Candidate) []string {
	t := table{indent: 2, columns: s.columns("workbenches", "workbench", "slug", "path")}
	for _, candidate := range rows {
		fields := []string{candidate.Title, s.slugCell(candidate.Slug), candidate.Path}
		if candidate.Refused != "" {
			fields = []string{s.refusedCell(candidate.Refused), "", candidate.Path}
		}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	return s.tableLines(t)
}

// refusedCell renders the refusal name a row could not be described past. The
// angle brackets are the convention that tells a reader the cell holds the
// reason a value is missing rather than the value itself, and one function
// composes it so the listing and the root-scoped headings cannot spell it two
// ways.
func (s *session) refusedCell(name string) string {
	return s.r.T("workbenches.refused", "refusal", name)
}

// slugCell renders a slug column's value: the slug itself when the entity has
// one, and a catalog-served placeholder naming the repair when it does not.
// A blank column gives a reader nothing to act on, indistinguishable from a
// rendering glitch, so a missing slug says so instead of padding an empty
// string.
func (s *session) slugCell(slug string) string {
	if slug == "" {
		return s.r.T("slug.missing")
	}
	return slug
}

// renderOffers prints what each column offers next.
func (s *session) renderOffers(offers []verb.Offer) {
	t := table{indent: 2, columns: s.columns("next", "column", "card", "title", "take")}
	for _, offer := range offers {
		if offer.Card == nil {
			// Four empty answers, most specific last, so the narrowest fact a
			// column can report is the one printed. A column holding ready
			// work that stands above the tier the caller declared says so,
			// which tells a reader to send a more senior caller rather than
			// to wait for work to arrive. A column where no act could take a
			// card up says that instead, which is a different fact again,
			// because a done column holding four ready cards offers none of
			// them. A column waiting on somebody outside says who it waits
			// on. Everything else has nothing ready.
			absent := s.r.T("next.none")
			if offer.AboveTier {
				absent = s.r.T("next.above-tier")
			}
			if offer.NoTaker {
				absent = s.r.T("next.no-taker")
			}
			if offer.AwaitingOutside {
				absent = s.r.T("next.awaiting-outside")
			}
			t.rows = append(t.rows, tableRow{fields: []string{offer.Title, absent}})
			continue
		}
		// The Take cell names the act that takes the offered card, since a
		// claim is refused where nobody takes work up and a pull into the
		// column beyond is what moves the card instead.
		take := s.r.T("next.take.claim")
		if offer.TakenByPull {
			take = s.r.T("next.take.pull")
		}
		fields := []string{offer.Title, offer.Card.Ref, offer.Card.Title, take}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// renderDetail prints a card, its links, its attachments, and its comments, and
// then what a shaped answer held back.
//
// The announcement is drawn last because it is about the answer rather than
// about the card, so a reader who asked for one member reads that member first
// and reads what it cost afterwards.
//
// Every member is drawn only where the answer carries it. The three below this
// one are slices and a string, so a member the answer left out holds nothing
// and the emptiness check already covers it, but the card view is a struct and
// its zero value draws a header of empty cells. The answer is asked directly
// here rather than read for emptiness, so a reader at the terminal is shown
// what the payload of the same call carries and nothing besides.
//
// The blank line between two blocks is drawn by gap rather than written at the
// head of each block, because a block is no longer sure of having anything
// above it. An answer whose field list leaves the card out opens on the first
// member it does carry, and one that carries a single member draws no blank
// line at all.
func (s *session) renderDetail(detail *verb.Detail) {
	drawn := false
	gap := func() {
		if drawn {
			s.line("")
		}
		drawn = true
	}
	if detail.Carries("card") {
		s.renderCard(&detail.Card)
		s.renderInapplicable(&detail.Card)
		drawn = true
	}
	if detail.Body != "" {
		gap()
		s.write(detail.Body)
	}
	if len(detail.Links) > 0 {
		gap()
		s.line(s.r.T("show.links"))
		links := table{indent: 2, columns: s.columns("links", "link", "card")}
		for _, link := range detail.Links {
			links.rows = append(links.rows, tableRow{fields: []string{link.Kind, link.Ref}})
		}
		s.table(links)
	}
	if len(detail.Attachments) > 0 {
		gap()
		s.line(s.r.T("show.attachments"))
		s.renderAttachments(detail.Attachments)
	}
	if len(detail.Comments) > 0 {
		gap()
		s.line(s.r.T("show.comments"))
		s.renderComments(detail.Comments)
	}
	if len(detail.Checklist) > 0 {
		gap()
		s.line(s.r.T("show.checklist"))
		// The block draws as a plain multi-column table with no heading row
		// and no rule over it: the item's reference, its state and whoever
		// answers it in three columns of their own, and the item's own text
		// last. A kind column would say a second time what the
		// questions/criteria/decisions segment of the reference already says.
		//
		// The three leading columns are separate rather than packed into one
		// value, which is what lets a reader run an eye down the states. Only
		// the last field of a row goes unpadded, so a value that has to line
		// up with the value below it has to be a column of its own, and
		// wrapTail then breaks the text between words at the column the text
		// itself starts at, so a wrapped line hangs under its own first line
		// rather than under the reference.
		//
		// The answer a settled item designates is drawn in two places, and
		// which one a reader gets is the whole of what the two checklist
		// reads differ by. Every answer carries the designated comment's
		// reference in a column of its own, which costs the item's own
		// anchor and nothing further, so a reader of the indexed checklist
		// learns that an answer exists and what to type to reach it without
		// a single comment being opened. An answer that asked for the
		// checklist in full carries the comment itself, and it is drawn
		// under the row it belongs to, which is where the retired
		// resolution note was drawn.
		checklist := table{indent: 2, columns: s.columns("checklist", "ref", "state", "owner", "comments", "resolution", "description"),
			labels: labelInTheStack, wrapTail: true}
		for _, item := range detail.Checklist {
			count := ""
			if item.CommentCount > 0 {
				count = strconv.Itoa(item.CommentCount)
			}
			fields := []string{item.Ref, item.State, item.Owner, count, item.Resolution, item.Text}
			checklist.rows = append(checklist.rows, tableRow{fields: fields, note: s.designationNote(item.Designated)})
		}
		s.table(checklist)
	}
	if len(detail.Withheld) > 0 {
		gap()
		s.line(s.r.T("show.withheld", "members", strings.Join(detail.Withheld, ", ")))
		// Which recovery the reader is offered depends on what held the
		// answer back. A member the field list left out is served by naming
		// it, and a member a filter narrowed is served by dropping the flag,
		// because the caller of `--unresolved --fields checklist.full` has
		// already named every member the announcement could ask them for.
		// One sentence names both levers where a filter ran, since such a
		// call can be short of a member for either reason at once.
		if flags := detail.NarrowedBy(); len(flags) > 0 {
			s.line(s.r.T("show.refilter", "reread", detail.Reread, "flags", strings.Join(flags, ", ")))
		} else {
			s.line(s.r.T("show.reread", "reread", detail.Reread))
		}
	}
}

// renderComments draws one comments block: each comment's reference, when it
// was written, who wrote it, what its first line says and how many bytes the
// body runs to, with the body carried as the row's note where the answer
// carries one.
//
// renderDetail and renderItemDetail both draw this, so a card's comments and
// an item's comments print in one shape. A card's own read serves an index
// and an item's serves every body, and the row carries whichever the answer
// holds rather than the block being split in two.
func (s *session) renderComments(comments []verb.CommentView) {
	block := table{indent: 2, columns: s.columns("comments", "ref", "when", "who", "subject", "size")}
	for _, comment := range comments {
		size := strconv.Itoa(comment.Size)
		fields := []string{comment.Ref, comment.TS, comment.Author, comment.Subject, size}
		block.rows = append(block.rows, tableRow{fields: fields, note: comment.Body})
	}
	s.table(block)
}

// renderItemDetail prints the item show answers for the item's own reference:
// the item's anchor, unchanged from what show printed for it before item
// comments existed, then the item's comments where it carries any.
func (s *session) renderItemDetail(item *verb.ItemDetail) {
	s.write(item.Text)
	if len(item.Comments) > 0 {
		s.line("")
		s.line(s.r.T("show.comments"))
		s.renderComments(item.Comments)
	}
}

// renderRecord prints the fields of an entity whose answer is a record rather
// than a body, which is the workbench and the workstream.
//
// It draws the two-column table the bare `dinah workbench` listing already
// draws, so one kind of answer reads one way wherever it is asked for. The
// field names travel untranslated, the way that listing's do and the way
// config's keys do, because a field name is machine vocabulary a caller types
// back. The slug row is served through slugCell, so an entity carrying none
// names its repair rather than standing blank.
func (s *session) renderRecord(record *verb.Record) {
	t := table{indent: 2, columns: s.columns("workbench", "field", "value")}
	for _, field := range record.Fields {
		value := field.Value
		if field.Name == "slug" {
			value = s.slugCell(value)
		}
		t.rows = append(t.rows, tableRow{fields: []string{field.Name, value}})
	}
	s.table(t)
}

// renderAttachmentListing prints the attachments of whatever entity was asked
// about, under a sentence naming that entity, and says so plainly when the
// entity carries none. An entity with nothing attached is an answer rather
// than a mistake, on the same terms `dinah list` already draws an entity
// that contains nothing.
func (s *session) renderAttachmentListing(listing *verb.AttachmentListing) {
	if len(listing.Attachments) == 0 {
		s.line(s.r.T("attachments.empty", "ref", listing.Ref))
		return
	}
	s.line(s.r.T("attachments.header", "ref", listing.Ref, "count", strconv.Itoa(len(listing.Attachments))))
	s.renderAttachments(listing.Attachments)
}

// renderCommentListing prints a card's comments under a sentence naming the
// collection, and says so plainly when the card carries none. The columns are
// the same five the show index draws, plus Who in place of Author for the
// shorter table heading.
func (s *session) renderCommentListing(listing *verb.CommentListing) {
	if len(listing.Members) == 0 {
		s.line(s.r.T("listing-comments.empty", "ref", listing.Ref))
		return
	}
	s.line(s.r.T("listing-comments.header", "ref", listing.Ref, "count", strconv.Itoa(len(listing.Members))))
	block := table{indent: 2, columns: s.columns("comments", "ref", "when", "who", "subject", "size"), stackOnOverflow: true}
	for _, comment := range listing.Members {
		block.rows = append(block.rows, tableRow{fields: []string{
			comment.Ref, comment.TS, comment.Author, comment.Subject, strconv.Itoa(comment.Size),
		}})
	}
	s.table(block)
}

// renderItemListing prints a card's checklist items under a sentence naming the
// collection, and says so plainly when the card carries none. The columns are
// the item's reference, its kind, its state, the column it names for gating,
// who answers it, its text, and how many comments it carries.
func (s *session) renderItemListing(listing *verb.ItemListing) {
	if len(listing.Members) == 0 {
		s.line(s.r.T("listing-items.empty", "ref", listing.Ref))
		return
	}
	s.line(s.r.T("listing-items.header", "ref", listing.Ref, "count", strconv.Itoa(len(listing.Members))))
	block := table{indent: 2, columns: s.columns("listing-items", "ref", "kind", "state", "column", "owner", "text", "comment-count"), stackOnOverflow: true}
	for _, item := range listing.Members {
		commentCount := ""
		if item.CommentCount > 0 {
			commentCount = strconv.Itoa(item.CommentCount)
		}
		block.rows = append(block.rows, tableRow{fields: []string{
			item.Ref, item.Kind, item.State, item.ColumnTitle, item.Owner, item.Text, commentCount,
		}})
	}
	s.table(block)
}

// renderAttachments draws the attachments table every read that reports
// attachments prints, so a card's own list and the list of any other entity
// cannot come out under different headings or in a different order.
func (s *session) renderAttachments(views []verb.AttachmentView) {
	attachments := table{indent: 2, columns: s.columns("attachments", "ref", "filename", "description")}
	for _, attachment := range views {
		attachments.rows = append(attachments.rows, tableRow{fields: []string{
			attachment.Ref,
			attachment.Filename,
			attachment.Description,
		}})
	}
	s.table(attachments)
}

// renderHistory prints a card's acts in the order they were recorded. An
// identifier carried in an act is never resolved against the bench as it now
// stands, so the titles printed are the ones the act itself carries.
func (s *session) renderHistory(events []bench.Event) {
	t := table{indent: 2, columns: s.columns("journal", "when", "action", "actor", "detail")}
	for _, ev := range events {
		fields := []string{ev.TS, s.token(ev.Event), ev.Actor.Name, s.eventDetail(ev)}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// eventDetail composes what an act carried, which is what the detail column of
// a journal line reads. Both blocks that draw journal lines read it, so one
// act cannot say one thing under log and another under changes.
func (s *session) eventDetail(ev bench.Event) string {
	switch ev.Event {
	case contract.EventMoved:
		tail := s.r.T("log.moved", "from", ev.FromTitle, "to", ev.ToTitle)
		if ev.Override {
			tail += " " + s.r.T("log.override")
		}
		if ev.Reject {
			tail += " " + s.r.T("log.reject")
		}
		return tail
	case contract.EventBlocked, contract.EventUnblocked:
		// An unblocked line carries a reason only when the lift said why,
		// and reads empty otherwise, which is what every such row read
		// before a lift could say anything.
		return ev.Reason
	case contract.EventCreated:
		return ev.Title
	case contract.EventAttached, contract.EventAttachmentReplaced, contract.EventAttachmentRemoved:
		return ev.Filename
	case contract.EventAttachmentRenamed:
		return s.r.T("log.attachment-renamed", "from", ev.From, "to", ev.Filename)
	case contract.EventManualCorrection:
		return s.r.T("log.manual-correction", "from", ev.FromTitle, "to", ev.ToTitle)
	case contract.EventRenumbered:
		return s.r.T("log.renumbered", "from", ev.From, "to", ev.To)
	case contract.EventTierOverridden:
		return s.tierOverriddenDetail(ev)
	case contract.EventCommented:
		// A column comment names its column, which is the locator the verb
		// writes and which a reader of the workbench journal otherwise has
		// no way to recover. Every other commented line carries no column
		// and draws the empty detail it has always drawn.
		if ev.ColumnTitle != "" {
			return ev.ColumnTitle
		}
		return ev.Column
	}
	return ""
}

// tierOverriddenDetail composes what a tier override carried: the column it
// concerns, the requirement on either side of the write, and the reason where
// one was given.
//
// The column is the title the event captured at write time, exactly as a moved
// line reads its own FromTitle and ToTitle, and nothing here resolves anything
// against the workbench as it now stands. A line carrying no title degrades to
// the stored identifier rather than to nothing: an ordinary per-column tier
// write captures no title, and neither did any line written before the field
// existed, so that fallback is what most of these lines read as.
//
// The reason is appended only where the event carries one, since only a raise
// is required to supply one and a hollow pair of brackets would say less than
// leaving them out.
func (s *session) tierOverriddenDetail(ev bench.Event) string {
	column := ev.ColumnTitle
	if column == "" {
		column = ev.Column
	}
	from := ev.From
	if from == "" {
		from = s.r.T("log.tier-overridden.no-requirement")
	}
	detail := s.r.T("log.tier-overridden", "column", column, "from", from, "to", ev.To)
	if ev.Reason == "" {
		return detail
	}
	return detail + " " + s.r.T("log.tier-overridden.reason", "reason", ev.Reason)
}

// renderChanges prints what one checkpoint answered with: the journal lines
// after the caller's cursor, in the order the merged walk imposes, and then
// the cursor to hand back next time.
//
// The cursor is printed on every answer, including one reporting nothing, so
// a reader always has the value the next call wants and never has to go and
// find the previous run. The columns are log's, with the entity each line was
// read from added, since a merged stream cannot say otherwise.
func (s *session) renderChanges(set *verb.ChangeSet) {
	s.renderChangesBody(set)
	s.line(s.r.T("changes.cursor", "cursor", set.Cursor))
}

// renderChangesBody is the journal half of a checkpoint's answer, without the
// cursor line beneath it. It is split out for the root-scoped form, where one
// merged cursor is printed once at the end and a member's own token is not the
// value the next call takes, so printing it under every workbench would offer
// a reader twenty-five tokens none of which is the one they need.
func (s *session) renderChangesBody(set *verb.ChangeSet) {
	t := table{indent: 2, columns: s.columns("changes", "when", "card", "action", "actor", "detail")}
	for _, ev := range set.Events {
		// ChangeEvent embeds bench.Event, so ev.Event is the whole line and
		// ev.Event.Event is the act's own name. The two are spelled apart
		// here rather than aliased, since the shape is the one the machine
		// surface publishes.
		fields := []string{ev.TS, changeSubject(ev), s.token(ev.Event.Event), ev.Actor.Name, s.eventDetail(ev.Event)}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// changeSubject is what the card column of a checkpoint reads: the reference
// of the entity the line came from where one could be composed, the bare
// identifier where the anchor that would name it is gone, and the scope word
// for the workbench, which is what a person types to name it.
func changeSubject(ev verb.ChangeEvent) string {
	if ev.Ref != "" {
		return ev.Ref
	}
	if ev.ID != "" {
		return ev.ID
	}
	return ev.Scope
}

// renderCheck prints what a check answered with: the account of the repair it
// was asked to make first, then the findings.
//
// The stamped count is printed even when it is zero, because a migration that
// found nothing to do and a migration that never ran are different answers to
// the operator's question and he asked for one of them.
func (s *session) renderCheck(report *verb.CheckReport) int {
	if report.MigratedSlugs {
		s.line(s.r.TN("check.slug-assigned", len(report.AssignedSlugs)))
		assigned := table{indent: 2, columns: s.columns("slugs", "slug", "title")}
		for _, assignment := range report.AssignedSlugs {
			assigned.rows = append(assigned.rows, tableRow{fields: []string{assignment.Slug, assignment.Title}})
		}
		s.table(assigned)
		if report.AssignedWorkbenchSlug != nil {
			s.line(s.r.T("check.workbench-slug-assigned", "slug", report.AssignedWorkbenchSlug.Slug))
		}
		// The workstream slugs are the third report of the one repair, so
		// they stay with the other two rather than reading as a separate
		// answer further down.
		s.line(s.r.TN("check.workstream-slug-assigned", len(report.AssignedWorkstreamSlugs)))
		workstreams := table{indent: 2, columns: s.columns("slugs", "slug", "title")}
		for _, assignment := range report.AssignedWorkstreamSlugs {
			workstreams.rows = append(workstreams.rows, tableRow{fields: []string{assignment.Slug, assignment.Title}})
		}
		s.table(workstreams)
	}
	if report.StampedOrdinals != nil {
		s.line(s.r.TN("check.ordinal-stamped", *report.StampedOrdinals))
	}
	// The adopted identifiers are counted and not listed, because every
	// workstream this repair creates carries no slug and so draws a finding
	// naming that same identifier immediately below.
	if report.MigratedWorkstreams {
		s.line(s.r.TN("check.workstream-adopted", len(report.AdoptedWorkstreams)))
	}
	if report.MigratedColumns {
		s.line(s.r.TN("check.columns-removed", len(report.RemovedStrandedColumns)))
		removed := table{indent: 2, columns: listColumn()}
		for _, id := range report.RemovedStrandedColumns {
			removed.rows = append(removed.rows, tableRow{fields: []string{id}})
		}
		s.table(removed)
	}
	if report.MigratedWitness {
		s.line(s.r.TN("check.witnessed", len(report.WitnessedCards)))
		witnessed := table{indent: 2, columns: listColumn()}
		for _, id := range report.WitnessedCards {
			witnessed.rows = append(witnessed.rows, tableRow{fields: []string{id}})
		}
		s.table(witnessed)
	}
	if report.MigratedBranches != nil {
		s.renderBranchMigration(report.MigratedBranches)
	}
	if report.MigratedNewlines != nil {
		s.renderNewlineMigration(report.MigratedNewlines)
	}
	if report.MigratedDesignations != nil {
		s.renderDesignationMigration(report.MigratedDesignations)
	}
	if report.MigratedAppliesWhen != nil {
		s.renderAppliesWhenMigration(report.MigratedAppliesWhen)
	}
	if report.MigratedNumbers {
		s.line(s.r.TN("check.card-numbers-written", *report.RegistryLines))
	}
	// The moved cards are counted and not listed, because each draws a
	// check.card-number-renumbered finding naming it immediately below.
	if report.MigratedNumbers || report.RenumberedNumbers {
		s.line(s.r.TN("check.cards-renumbered", len(report.RenumberedCards)))
	}
	code := s.renderFindings(report.Findings)
	s.renderNotices(report.Notices)
	// A repair can need a person while the checker finds nothing, which is
	// the branch migration meeting a conflict: it wrote nothing, it named the
	// cards to repair, and the defect it met is in no finding. The report's
	// own outcome already carries that, so the exit code is taken from there
	// rather than from the finding count alone.
	if code == 0 && report.Outcome == contract.ReadFindings {
		code = contract.ExitCodeForRead(report.Outcome)
	}
	return code
}

// renderNotices prints what check reports without counting it: a heading
// and then one row per notice, drawn as a finding row is drawn. It prints
// nothing where there are none, and it returns nothing because a notice never
// reaches the exit code.
func (s *session) renderNotices(notices []bench.Finding) {
	if len(notices) == 0 {
		return
	}
	s.line(s.r.T("check.notices"))
	t := table{indent: 2, columns: listColumn()}
	for _, notice := range notices {
		reported := s.r.T(notice.Key, "detail", notice.Detail) + " (" + notice.Path + ")"
		t.rows = append(t.rows, tableRow{fields: []string{reported}})
	}
	s.table(t)
}

// renderAppliesWhenMigration prints the one line the format stamp answers
// with: that it wrote the number, that it would write it, or that the store
// already declares it.
func (s *session) renderAppliesWhenMigration(report *bench.AppliesWhenMigration) {
	target := strconv.Itoa(bench.AppliesWhenFormat)
	switch {
	case report.Stamped:
		s.line(s.r.T("check.format-stamped", "format", target))
	case report.From >= bench.AppliesWhenFormat:
		s.line(s.r.T("check.format-current", "format", strconv.Itoa(report.From)))
	default:
		s.line(s.r.T("check.format-would-stamp", "from", strconv.Itoa(report.From), "format", target))
	}
}

// renderDesignationMigration prints the designation conversion's own account,
// which is the same report on a rehearsal and on a converting run.
//
// The claims the operator passed come first, ahead of every group, so he reads
// what he overrode at the moment he overrides it rather than afterwards.
//
// The three conversion groups are drawn in the order the contract states them
// and each carries its own count, because a group with no entries is a fact
// worth printing: a run reporting nothing at all under a heading nobody drew
// reads exactly like a run that never looked.
//
// The unanswered group is drawn last and its count is repeated on the run's
// own last line, because it is the number the operator acts on and the groups
// above it may have scrolled past by then.
//
// Each entry is one sentence rather than one row of a table, on the reasoning
// renderNewlineMigration states for its own account: a migration's report is
// read once by a person deciding what to do next, and a table here would owe
// the row-layout sweep a fixture and a language pass for a block nobody scans.
func (s *session) renderDesignationMigration(run *bench.DesignationMigration) {
	if run.Forced || len(run.PassedClaims) > 0 {
		s.line(s.r.TN("check.designations-claims-passed", len(run.PassedClaims)))
		for _, claim := range run.PassedClaims {
			s.line(s.r.T("check.designations-claim", "card", claim.Ref, "owner", claim.Holder))
		}
	}
	s.renderDesignationGroup(run, bench.DesignationFromJournal, "check.designations-journal")
	// The journal group is exactly the population whose answer was inferred,
	// so the group itself is the caution and it carries one. Route 1 is
	// reached only where something removed since the settling may have moved
	// the positions, and it matches only where a comment of the same item
	// landed in the settling's own second, so every entry above is an item
	// whose position moved and whose settling shares a second with another
	// comment. That is the residue the conversion cannot close, and the
	// operator reads it here rather than discovering it afterwards.
	//
	// The caution stands over the group rather than beside the archived
	// listing below, and the difference is the whole of this finding. The
	// archived listing can only see a comment that survives in the archive,
	// and the removal this residue most often turns on is a deletion, which
	// leaves nothing anywhere to find. That blind spot is the one dinah-472
	// already paid for once: an earlier draft detected drift by looking for
	// archived comments and was defeated by deleting instead. A caution
	// inheriting that blind spot would miss the one removal the card knows it
	// cannot see.
	if run.Count(bench.DesignationFromJournal) > 0 {
		s.line(s.r.T("check.designations-journal-inferred"))
	}
	s.renderDesignationGroup(run, bench.DesignationUndisturbed, "check.designations-undisturbed")
	drifted := 0
	for _, entry := range run.Entries {
		if entry.Archived && entry.Identifier != "" {
			drifted++
		}
	}
	s.line(s.r.TN("check.designations-archived", drifted))
	for _, entry := range run.Entries {
		if entry.Archived && entry.Identifier != "" {
			s.line(s.r.T("check.designations-entry", "item", entry.Item,
				"settling", entry.Settling, "comment", entry.Identifier, "author", entry.Author))
		}
	}
	unanswered := 0
	for _, entry := range run.Entries {
		if entry.Route == bench.DesignationUnrecoverable {
			unanswered++
		}
	}
	s.line(s.r.TN("check.designations-unanswered", unanswered))
	for _, entry := range run.Entries {
		if entry.Route != bench.DesignationUnrecoverable {
			continue
		}
		// Two shapes of absence reach this line and each is named rather
		// than left blank. An item whose journal records no settling takes
		// the unsettled form, because the ordinary one names the settling
		// verb and an empty slot leaves a comma hanging in the one group the
		// operator is told to act on. A stored reference that reaches no
		// comment of the item takes the second clause below, for the same
		// reason read one slot along.
		reaches := s.r.T("check.designations-reaches-nothing")
		if entry.Author != "" {
			reaches = s.r.T("check.designations-reaches", "author", entry.Author)
		}
		key := "check.designations-unanswered-entry"
		if entry.Settling == "" {
			key = "check.designations-unanswered-entry-unsettled"
		}
		s.line(s.r.T(key, "item", entry.Item,
			"settling", entry.Settling, "state", entry.State, "stored", entry.Stored, "reaches", reaches))
	}
	if unanswered > 0 {
		s.line(s.r.T("check.designations-keeps-state"))
	}
	if !run.Applied {
		s.line(s.r.T("check.designations-rehearsed"))
	} else if run.Stamped {
		s.line(s.r.T("check.designations-stamped", "detail", strconv.Itoa(bench.DesignationFormat)))
	}
	if unanswered > 0 {
		s.line(s.r.TN("check.designations-unanswered-total", unanswered))
	}
}

// renderDesignationGroup prints one conversion group's count and then one
// sentence per entry, each naming the item, the verb that settled it, the
// comment identifier the run wrote and that comment's author.
func (s *session) renderDesignationGroup(run *bench.DesignationMigration, route, key string) {
	s.line(s.r.TN(key, run.Count(route)))
	for _, entry := range run.Entries {
		if entry.Route != route {
			continue
		}
		s.line(s.r.T("check.designations-entry", "item", entry.Item,
			"settling", entry.Settling, "comment", entry.Identifier, "author", entry.Author))
	}
}

// branchConflictKeys names the catalog entry each conflict condition prints,
// so the tokens the migration reports stay tokens and the words stay in the
// catalog where a translator can reach them.
var branchConflictKeys = map[string]string{
	bench.BranchConflictAnchorDiffers: "check.branch-conflict.anchor-differs",
	bench.BranchConflictTwoHeadings:   "check.branch-conflict.two-headings",
	bench.BranchConflictUnreadable:    "check.branch-conflict.unreadable",
}

// renderBranchMigration prints the branch migration's own account: what the
// classification found, and then either the conflicts that stopped the run or
// what the write pass did.
//
// The conflicts are drawn instead of the write rather than beside it, because a
// run that met one wrote nothing at all, and a count of lifted cards printed
// under them would describe work that did not happen.
//
// The preview draws the same cards under sentences of its own. One sentence
// serving both phases is true of one of them, and this is the output an
// operator reads before authorising a repair that has no undo, so a line
// saying a card now carries a value under a line saying nothing was written
// is the worst place in the tool for that class of defect.
//
// Each card is one sentence rather than one row of a table, because a
// migration's account is read once by a person deciding whether to run it
// again, and a table here would owe the row-layout sweep a fixture and a
// language pass for a block nobody scans.
func (s *session) renderBranchMigration(report *bench.BranchMigration) {
	if report.Preview {
		s.line(s.r.T("check.branches-preview"))
	}
	s.line(s.r.TN("check.branches-lifted", len(report.Lifted)))
	s.line(s.r.TN("check.branches-untouched", report.Untouched))
	if len(report.Conflicts) > 0 {
		s.line(s.r.TN("check.branches-conflicted", len(report.Conflicts)))
		for _, conflict := range report.Conflicts {
			if key, named := branchConflictKeys[conflict.Condition]; named {
				s.line(s.r.T(key, "card", conflict.Card))
			}
		}
		return
	}
	lift, emptiedHeading, emptied := "check.branch-lift", "check.branches-emptied", "check.branch-emptied"
	if report.Preview {
		lift, emptiedHeading, emptied = "check.branch-would-lift", "check.branches-would-empty", "check.branch-would-empty"
	}
	for _, carried := range report.Lifted {
		s.line(s.r.T(lift, "card", carried.Card, "branch", carried.Value))
	}
	if len(report.Emptied) > 0 {
		s.line(s.r.TN(emptiedHeading, len(report.Emptied)))
		for _, ref := range report.Emptied {
			s.line(s.r.T(emptied, "card", ref))
		}
	}
	if report.Declared {
		s.line(s.r.T("check.branches-declared", "field", bench.BranchFieldKey))
	}
	if report.Stamped {
		s.line(s.r.T("check.branches-stamped", "format", strconv.Itoa(bench.FieldsFormat)))
	}
}

// renderFindings prints what check found and returns the read's own exit
// code, which is contract.ExitCodeForRead's table and never ExitCode's: zero
// on a clean bench, and the findings code when anything was found. A refusal
// does not come through here at all, because a read that could not run is
// reported before this function is reached.
func (s *session) renderFindings(findings []bench.Finding) int {
	if len(findings) == 0 {
		s.line(s.r.T("check.clean"))
		return 0
	}
	t := table{indent: 2, columns: listColumn()}
	for _, finding := range findings {
		reported := s.r.T(finding.Key, "detail", finding.Detail) + " (" + finding.Path + ")"
		t.rows = append(t.rows, tableRow{fields: []string{reported}})
	}
	s.table(t)
	s.line(s.r.TN("check.count", len(findings)))
	return contract.ExitCodeForRead(contract.ReadFindings)
}

// renderIdentity prints the actor and whether it is the operator, and then,
// where the caller declared anything about what is performing the act, the
// declared facts and the tier the workbench resolved from them.
//
// A person who declared nothing sees the one line they see today and nothing
// further, so they can tell that the tool read nothing rather than that it read
// something empty. The harness line says the value is malformed rather than
// dropping it, because a person repairing a mistyped variable needs to see what
// it says, and no read is ever refused over it.
func (s *session) renderIdentity(identity *verb.Identity) {
	s.line(s.r.T("whoami.line", "actor", identity.Actor, "operator", s.yesNo(identity.IsOperator)))
	if identity.Harness != "" {
		key := "whoami.harness"
		if identity.MalformedHarness {
			key = "whoami.harness.malformed"
		}
		s.line(s.r.T(key, "harness", identity.Harness))
	}
	if identity.Provider != "" {
		s.line(s.r.T("whoami.provider", "provider", identity.Provider))
	}
	if identity.Model != "" {
		s.line(s.r.T("whoami.model", "model", identity.Model))
	}
	if identity.Server != "" {
		s.line(s.r.T("whoami.server", "server", identity.Server))
	}
	if identity.Tier != "" {
		s.line(s.r.T("whoami.tier", "tier", identity.Tier))
	}
}

// renderVersion prints what this binary is and what it conforms to.
func (s *session) renderVersion(release *verb.VersionReport) {
	s.line(s.r.T("version.tool", "release", release.Tool))
	s.line(s.r.T("version.profile", "profile", release.Profile))
	s.line(s.r.T("version.format", "format", strconv.Itoa(release.Format)))
	if len(release.Catalogs) == 0 {
		return
	}
	s.line("")
	s.line(s.r.T("version.catalogs"))
	t := table{indent: 2, columns: s.columns("catalogs", "language", "translated")}
	for _, catalog := range release.Catalogs {
		coverage := strconv.Itoa(catalog.Translated) + "/" + strconv.Itoa(catalog.Total)
		t.rows = append(t.rows, tableRow{fields: []string{catalog.Tag, coverage}})
	}
	s.table(t)
}

// outcomeValues is what a verb's response carries for the composer: the named
// values the raise site attached, and the card reference the head knows and
// the raise site could not name without being edited.
func (s *session) outcomeValues(response *verb.Response) map[string]string {
	values := make(map[string]string, len(response.Context)+1)
	for name, carried := range response.Context {
		values[name] = carried
	}
	if response.Card != nil && response.Card.Ref != "" {
		values[contract.ValueCard] = response.Card.Ref
	}
	return values
}

// refusalListings resolves a shape's Listing name to the members that listing
// prints, one per row. The enumerable sets live in three different places, so
// this map is what keeps the composer from knowing about any of them.
//
// A refusal raised before the workbench opens carries no columns, which costs
// nothing in practice because unknown-column is only ever raised once one is
// open.
var refusalListings = map[string]func(*session) []string{
	"columns": func(s *session) []string {
		if s.library == nil {
			return nil
		}
		rows := make([]string, 0, len(s.library.Bench.Columns))
		for _, column := range s.library.Bench.Columns {
			rows = append(rows, column.Ref())
		}
		return rows
	},
	"guides":   func(s *session) []string { return guide.Topics() },
	"settings": func(s *session) []string { return bench.ConfigKeys },
	// The fields a `dinah set` may name are the union of every kind's own
	// fields and every key the open workbench declares, because the argument
	// reaches both and a caller offered only the first half would never learn
	// the second exists. The declared keys follow the built-in names in
	// declaration order rather than being sorted in among them, so a reader
	// can see which half a name came from.
	fieldsVocabulary: func(s *session) []string {
		names := bench.AllFields()
		if s.library == nil {
			return names
		}
		for _, field := range s.library.Bench.DeclaredFields() {
			names = append(names, field.Key)
		}
		return names
	},
}

// refusalBlocks are the listings that arrive already laid out, because their
// rows carry columns rather than bare names and one place already draws them.
var refusalBlocks = map[string]func(*session) []string{
	"workbenches": func(s *session) []string {
		rows, err := bench.Reachable(s.cwd, s.benchFlag, s.home, s.nativeHome)
		if walked := walkRefusalOf(err); walked != nil {
			// The walk that would have named the candidates refused, so the
			// reason stands where the rows would have. Drawing nothing here
			// tells a person the directory holds several workbenches and
			// then shows them none, which is the confident empty answer the
			// machine form no longer gives.
			return s.formatCandidateRows([]bench.Candidate{{Refused: walked.Name}})
		}
		return s.formatCandidateRows(rows)
	},
}

// composeRefusal renders a refusal for a person: the name and the sentence,
// the enumerated set where the refusal declares one, and the declared
// fragments in the order the shape states them.
//
// It returns lines rather than writing them, so the machine path and the text
// path share one composition, a test reads the result without a buffer, and
// the function names no stream at all. Every rule it applies comes off the
// shape, so no refusal name is tested anywhere inside it.
//
// One refusal name can answer two different acts, and the sentence then
// depends on which command raised it, so a shape declaring the raising command
// as a variant renders that command's own base entry. A command declaring none
// renders exactly what it rendered before, translations included.
//
// A fragment splices onto the sentence where the refusal prints no listing,
// which is what the tool has always done and is why each fragment carries
// whatever leading punctuation its position needs. Where a listing is printed
// the fragments form a line of their own beneath the rows, because a sentence
// cannot continue across a table.
func (s *session) composeRefusal(r *contract.Refusal) []string {
	shape := contract.ShapeOf(r.Name)
	if shape == nil {
		return []string{r.Name + " " + s.r.T("refusal.unknown", "name", r.Name, "detail", r.Detail)}
	}
	values := s.refusalValues(r)
	pairs := make([]string, 0, 2*len(values))
	for _, name := range sortedKeys(values) {
		pairs = append(pairs, name, values[name])
	}
	key := "refusal." + shape.Name
	if shape.Variant(values[contract.ValueCommand]) {
		key = shape.VariantKeyOf(values[contract.ValueCommand])
	}
	if shape.Subject != "" && values[shape.Subject] == "" {
		key = shape.AbsentKeyOf(key)
	}
	lines := []string{r.Name + " " + s.r.T(key, pairs...)}

	var rows []string
	if block, ok := refusalBlocks[shape.Listing]; ok {
		rows = block(s)
	} else if members, ok := refusalListings[shape.Listing]; ok {
		t := table{indent: 2, columns: listColumn()}
		for _, member := range members(s) {
			t.rows = append(t.rows, tableRow{fields: []string{member}})
		}
		rows = s.tableLines(t)
	} else if shape.Carried != "" {
		carried := values[shape.Carried]
		if carried != "" {
			// The raise site joins references that are never empty, so
			// every field of the split is a row and none is skipped.
			//
			// The local is named for the branch it belongs to rather than
			// t, which the listing branch above already binds to a table
			// of the same columns. Two same-named, same-columned calls in
			// one function are indistinguishable to the registry that
			// names call sites, so the two would trade entries silently
			// if the branches were ever reordered.
			carriedTable := table{indent: 2, columns: listColumn()}
			for _, member := range strings.Split(carried, "\n") {
				carriedTable.rows = append(carriedTable.rows, tableRow{fields: []string{member}})
			}
			rows = s.tableLines(carriedTable)
		}
	}
	lines = append(lines, rows...)

	next := s.nextStepOf(shape, values)
	var spliced []string
	for _, fragment := range shape.Fragments {
		if shape.NamedInNextStep(fragment.Key) {
			if fragment.Key == next {
				spliced = append(spliced, s.r.T(fragment.Key, pairs...))
			}
			continue
		}
		if holds(fragment, values) {
			spliced = append(spliced, s.r.T(fragment.Key, pairs...))
		}
	}
	if len(spliced) == 0 {
		return lines
	}
	joined := strings.Join(spliced, "")
	if len(rows) == 0 {
		lines[0] += joined
		return lines
	}
	return append(lines, joined)
}

// nextStepOf reads the alternation and returns the key of the one fragment
// that renders: the first whose condition holds, and never more than one. The
// last member carries no condition, so a shape the guard has passed always
// answers with a key.
//
// The winner renders at the position its fragment holds in the declared list
// rather than after every other fragment, because a clause split out of a base
// entry sat where the sentence put it. dinah.usage is where that is visible:
// its next step was written ahead of the dash hint, so it is declared ahead of
// it and it renders ahead of it.
func (s *session) nextStepOf(shape *contract.Shape, values map[string]string) string {
	for _, named := range shape.NextStep {
		fragment := shape.Fragment(named)
		if fragment != nil && holds(*fragment, values) {
			return named
		}
	}
	return ""
}

// holds reports whether a fragment's condition is satisfied: a When names a
// value that is present and non-empty, an Unless names one that is not, a
// WhenCommand names the command the reader typed, and a fragment carrying none
// of the three always renders.
func holds(fragment contract.Fragment, values map[string]string) bool {
	if fragment.When != "" {
		return values[fragment.When] != ""
	}
	if fragment.Unless != "" {
		return values[fragment.Unless] == ""
	}
	if fragment.WhenCommand != "" {
		return values[contract.ValueCommand] == fragment.WhenCommand
	}
	return true
}

// refusalValues collects everything a refusal's sentence may name: the detail,
// the two values only this invocation knows, and the named values the raise
// site carried. The raise site wins a collision, since a value it attached is
// about the refusal rather than about the invocation.
func (s *session) refusalValues(r *contract.Refusal) map[string]string {
	values := map[string]string{"detail": r.Detail}
	if s.command != "" {
		values[contract.ValueCommand] = s.command
		values[contract.ValueUsage] = verb.Usage(s.command)
	}
	for name, carried := range r.Extra {
		values[name] = carried
	}
	return values
}

// sortedKeys returns a map's keys in order, so that one refusal renders the
// same way twice however the map was built.
func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// renderWorkbenchFields prints the three fields the listing carries, one row
// per field. The workbench's instructions are a field of it too and are
// deliberately not here, because a listing that printed a whole instruction
// body would stop being a listing; `dinah get . instructions` reads them.
//
// The field names travel untranslated, the way `config`'s keys do, because a
// field name is machine vocabulary a caller types back. The slug row is served
// through slugCell, so a workbench carrying none names its repair rather than
// standing blank, and this listing says what `dinah list columns` and `dinah list
// workbenches` already say about a missing slug.
func (s *session) renderWorkbenchFields(fields *verb.WorkbenchView) {
	t := table{indent: 2, columns: s.columns("workbench", "field", "value")}
	for _, name := range bench.WorkbenchListingFields {
		value := fields.Field(name)
		if name == "slug" {
			value = s.slugCell(value)
		}
		t.rows = append(t.rows, tableRow{fields: []string{name, value}})
	}
	s.table(t)
}

// renderWorkstreams prints every live workstream of the workbench, and the
// sentence that says so when the workbench carries none. The columns are the
// shape the columns listing already draws, and the first cell is the reference a
// reader types rather than the bare slug, so it needs no slugCell: Ref falls
// back to the identifier and is never empty.
func (s *session) renderWorkstreams(listing *verb.WorkstreamListing) {
	if len(listing.Workstreams) == 0 {
		s.line(s.r.T("workstreams.empty"))
		return
	}
	t := table{indent: 2, columns: s.columns("workstreams", "reference", "name", "status", "cards")}
	for _, workstream := range listing.Workstreams {
		fields := []string{
			workstream.Ref,
			workstream.Title,
			workstream.Status,
			strconv.Itoa(workstream.Cards),
		}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// renderRoutes prints the routes a workbench declares: each route's name, how
// many live columns it carries, and the live columns it omits.
//
// The skipped columns carry a mark on the ones the workbench reserves to its
// operator, because that is the one question an operator asks of a road
// somebody drew and a reader who had to cross-reference the column listing to
// answer it would be deriving what the row states.
func (s *session) renderRoutes(listing *verb.RouteListing) {
	if len(listing.Routes) == 0 {
		s.line(s.r.T("routes.empty"))
		return
	}
	t := table{indent: 2, columns: s.columns("routes", "route", "columns", "skips")}
	for _, route := range listing.Routes {
		owned := map[string]bool{}
		for _, ref := range route.OperatorOwnedSkips {
			owned[ref] = true
		}
		skips := make([]string, 0, len(route.Skips))
		for _, ref := range route.Skips {
			if owned[ref] {
				ref = s.r.T("routes.skip.operator-owned", "column", ref)
			}
			skips = append(skips, ref)
		}
		t.rows = append(t.rows, tableRow{fields: []string{
			route.Name,
			strconv.Itoa(route.Columns),
			strings.Join(skips, ", "),
		}})
	}
	s.table(t)
}

// renderWorkstreamLine prints the one line a person needs after creating a
// workstream or writing one of its fields, which reads the way the card line
// reads after an act on a card.
func (s *session) renderWorkstreamLine(workstream *verb.WorkstreamView) {
	if workstream == nil {
		return
	}
	s.line(s.r.T("workstream.line",
		"ref", workstream.Ref,
		"title", workstream.Title,
		"status", workstream.Status,
	))
}

// workstreamsCell renders a card's memberships for the trailing field of the
// card line: each identifier as what a person could type to reach it, which is
// the workstream's slug where it carries one and the identifier where it does
// not, joined by the catalog's own separator.
//
// The resolution happens here rather than in the library because the machine
// surface carries the identifiers the card's frontmatter stores, deliberately,
// and a reader of the JSON resolves them the way a reader of a link's to
// already does. The head reads the open workbench for the same reason the
// composer reads its columns to list them.
func (s *session) workstreamsCell(ids []string) string {
	refs := make([]string, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, s.workstreamRef(id))
	}
	return strings.Join(refs, s.r.T("card.line.workstreams.separator"))
}

// workstreamRef is what a person types to reach one workstream a card lists.
// A membership naming nothing keeps the identifier the card carries, so the
// line still shows the value a reader has to go and repair.
func (s *session) workstreamRef(id string) string {
	if s.library == nil {
		return id
	}
	if workstream := s.library.Bench.Workstream(id); workstream != nil {
		return workstream.Ref()
	}
	return id
}

// renderVocabulary prints what a tree-wide vocabulary migration did, one
// section per outcome, and prints only the sections that hold something. A
// section nobody needs is a line a reader has to skip, and the counts alone
// already say that nothing was silently dropped.
//
// The rows are printed as whole lines rather than drawn as a table, and they
// carry no indent. A row is one filesystem path, sometimes with a reason after
// it, and a path is both the longest thing this head prints and the one thing a
// reader most often copies: measuring it into a column would wrap it or pad
// every other row out to it, and neither helps. The indent goes with the table,
// for the reason TestNoRowIsLaidOutOutsideTheOneRenderer gives: a run of spaces
// written here counts characters where a terminal counts columns, and one
// renderer owns that arithmetic.
func (s *session) renderVocabulary(report *verb.TreeVocabularyReport) {
	if report.Preview {
		s.renderVocabularyPreview(report)
	} else {
		s.line(s.r.TN("check.vocabulary-migrated", len(report.Migrated)))
		for _, entry := range report.Migrated {
			s.line(s.r.TN("check.vocabulary-cards", entry.Cards, "path", entry.Path))
		}
	}
	s.vocabularySection("check.vocabulary-already-current", report.AlreadyCurrent)
	s.vocabularySection("check.vocabulary-malformed", report.Malformed)
	unsupported := make([]string, 0, len(report.Unsupported))
	for _, entry := range report.Unsupported {
		unsupported = append(unsupported, entry.Path+" ("+entry.Revision+")")
	}
	s.vocabularySection("check.vocabulary-unsupported", unsupported)
	failed := make([]string, 0, len(report.Failed))
	for _, entry := range report.Failed {
		failed = append(failed, entry.Path+": "+entry.Reason)
	}
	s.vocabularySection("check.vocabulary-failed", failed)
	if report.Preview {
		s.line(s.r.T("check.vocabulary-confirm"))
	}
}

// renderVocabularyPreview prints what a run carrying the confirmation would
// rewrite, and the line telling the reader how to authorize it. The heading
// and the closing line are both printed even when the walk found nothing,
// because a reader who ran the command wants to be told that his tree holds
// nothing to carry forward rather than to be shown an empty page.
func (s *session) renderVocabularyPreview(report *verb.TreeVocabularyReport) {
	s.line(s.r.TN("check.vocabulary-would-migrate", len(report.Migrated)))
	for _, entry := range report.Migrated {
		s.line(entry.Path + " (" + entry.Revision + ")")
	}
}

// vocabularySection prints one heading and its rows, or nothing at all when
// the outcome caught no workbench.
func (s *session) vocabularySection(key string, rows []string) {
	if len(rows) == 0 {
		return
	}
	s.line(s.r.TN(key, len(rows)))
	for _, row := range rows {
		s.line(row)
	}
}

// renderContainer prints what a tree-wide container migration did, or would
// do. It follows renderVocabulary line for line, because the two commands
// answer the same question about different repairs and a reader who has
// learned one should not have to learn a second.
func (s *session) renderContainer(report *verb.TreeContainerReport) {
	if report.Preview {
		s.line(s.r.TN("check.container-would-migrate", len(report.Migrated)))
		for _, entry := range report.Migrated {
			s.line(s.containerPreviewRow(report, entry))
		}
	} else {
		bare, alreadyNamed := splitByShape(report.Migrated)
		s.migratedSection("check.container-migrated-bare", bare)
		s.migratedSection("check.container-migrated-legacy", alreadyNamed)
	}
	s.vocabularySection("check.container-already", report.AlreadyContained)
	s.vocabularySection("check.container-duplicate", s.findingRows(report.DuplicateFindings()))
	s.vocabularySection("check.container-damaged", s.findingRows(report.DamagedWorkbenchFindings()))
	unfinished := make([]string, 0, len(report.Failed))
	failed := make([]string, 0, len(report.Failed))
	for _, entry := range report.Failed {
		if entry.To != "" {
			unfinished = append(unfinished, entry.Path+" -> "+entry.To+": "+entry.Reason)
			continue
		}
		failed = append(failed, entry.Path+": "+entry.Reason)
	}
	s.vocabularySection("check.container-moved-unfinished", unfinished)
	s.vocabularySection("check.container-failed", failed)
	if report.Preview {
		s.line(s.r.T("check.container-confirm"))
	}
}

// splitByShape divides what a container migration moved into the two groups
// its applied report speaks about: the bare workbenches, whose repair created
// a container and emptied a directory, and the ones that already sat inside a
// container under a name the rule does not mint.
//
// The second group holds two shapes rather than one. A legacy name is one
// Dinah minted under an older width and a stray name is one a person typed,
// and both take the identical repair, so an operator counting what happened to
// his tree learns nothing from having them apart. What he does need to be told
// is which workbenches moved between directories, and that is the split this
// makes.
func splitByShape(migrated []verb.ContainerEntry) (bare, alreadyNamed []verb.ContainerEntry) {
	for _, entry := range migrated {
		if entry.Shape == string(bench.ShapeBare) {
			bare = append(bare, entry)
			continue
		}
		alreadyNamed = append(alreadyNamed, entry)
	}
	return bare, alreadyNamed
}

// migratedSection prints one heading of an applied container migration and the
// workbenches it counts, or nothing at all when that group caught none.
//
// A group with no members prints no heading, which is why an applied run that
// moved nothing prints no count line. The old single heading printed one over
// every shape at once and said every workbench had been carried into a
// container, which was untrue of the tree the operator ran it on: all thirteen
// of his workbenches were already in containers and were renamed in place.
func (s *session) migratedSection(key string, entries []verb.ContainerEntry) {
	if len(entries) == 0 {
		return
	}
	s.line(s.r.TN(key, len(entries)))
	for _, entry := range entries {
		s.line(entry.Path + " -> " + entry.To)
	}
}

// containerPreviewRow is the line a preview prints for one workbench it would
// carry forward.
//
// A bare workbench prints its own finding sentence rather than a shape word,
// because it is the one shape whose repair creates a directory and empties
// another, and a reader deciding whether to authorize that needs to be told
// which directory is created and that only the workbench's own files move.
// Every other shape is a rename inside a container the workbench already sits
// in, where the shape word is the whole of what happens.
func (s *session) containerPreviewRow(report *verb.TreeContainerReport, entry verb.ContainerEntry) string {
	if entry.Shape == string(bench.ShapeBare) {
		for _, finding := range report.BareWorkbenchFindings() {
			if finding.Detail == entry.Path {
				return s.r.T(finding.Key, "detail", finding.Detail)
			}
		}
	}
	return entry.Path + " (" + entry.Shape + ")"
}

// findingRows renders each finding as its own catalog sentence, which is how
// the rows of a container sweep reach a reader in his own language rather than
// as a detail string a caller assembled.
func (s *session) findingRows(findings []bench.Finding) []string {
	rows := make([]string, 0, len(findings))
	for _, finding := range findings {
		rows = append(rows, s.r.T(finding.Key, "detail", finding.Detail))
	}
	return rows
}

// renderRemint prints the one rename a remint performed.
func (s *session) renderRemint(report *verb.RemintReport) {
	s.line(s.r.T("check.reminted", "from", report.From, "to", report.To))
}

// renderColumnLine prints the one line a person needs after creating a column,
// which reads the way the workstream line reads after an act on a workstream.
//
// The capacity cell is a sentence of its own rather than a value spliced into
// the line, because a column with no limit has no number to print and a bare
// blank there would leave a reader wondering what was missing.
//
// The reference printed is the slug rather than Column.Ref's own fallback to
// the identifier, because a column this command created always carries one:
// creation refuses a title no slug can be derived from rather than writing a
// column reachable only by its identifier.
func (s *session) renderColumnLine(column *verb.ColumnView) {
	if column == nil {
		return
	}
	capacity := s.r.T("columns.new.unlimited")
	if column.Capacity > 0 {
		capacity = s.r.T("columns.new.capacity", "count", strconv.Itoa(column.Capacity))
	}
	s.line(s.r.T("columns.new.line",
		"ref", column.Slug,
		"title", column.Title,
		"kind", s.token(column.Kind),
		"capacity", capacity,
	))
}

// renderSearch prints what a search found: one row per hit, ranked, with a
// snippet showing why the hit surfaced.
//
// The score itself is not a column. A person reads rank by reading down the
// page, the way they read `dinah query`'s own table, and a number beside every
// row would only invite them to compare two hits by arithmetic the table has
// already done for them.
//
// This site sits at the foot of the file on purpose: three fixtures in this
// package address a table by its own source line, so a block added above one
// of them moves it.
func (s *session) renderSearch(results *verb.SearchResults) {
	if len(results.Hits) == 0 {
		s.line(s.r.T("search.empty"))
		return
	}
	t := table{indent: 2, columns: s.columns("search", "kind", "reference", "title", "matched", "snippet")}
	for _, hit := range results.Hits {
		fields := []string{s.token(hit.Kind), hit.Ref, hit.Title, s.token(hit.MatchedIn), hit.Snippet}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// newlineConflictKeys names the catalog entry each conflict condition prints,
// so the tokens the migration reports stay tokens and the words stay in the
// catalog where a translator can reach them.
var newlineConflictKeys = map[string]string{
	bench.NewlineConflictUnreadable:  "check.newline-conflict.unreadable",
	bench.NewlineConflictUnwritable:  "check.newline-conflict.unwritable",
	bench.NewlineConflictUnsupported: "check.newline-conflict.unsupported",
	bench.NewlineConflictLocked:      "check.newline-conflict.locked",
}

// renderNewlineMigration prints the newline repair's own account.
//
// The preview's heading says that each destination was rewritten with its own
// bytes and that its modification time is now, rather than saying that nothing
// was written, because the preview does write and a reader meeting that
// sentence afterwards has already paid for it. The rehearsal is the only way to
// establish that the real write will be permitted.
//
// A refused plan is drawn instead of the rewrites rather than beside them,
// because a run that met an unreadable, unwritable or unsupported destination
// wrote nothing at all. A busy file is the exception and is drawn beside them,
// with the sentence saying that running the command again picks it up.
func (s *session) renderNewlineMigration(report *bench.NewlineMigration) {
	if report.Applied {
		s.line(s.r.T("check.newlines-heading"))
	} else {
		s.line(s.r.T("check.newlines-preview"))
	}
	s.line(s.r.TN("check.newlines-examined", report.Examined))
	var busy, refused []bench.NewlineConflict
	for _, conflict := range report.Conflicts {
		if conflict.Condition == bench.NewlineConflictLocked {
			busy = append(busy, conflict)
			continue
		}
		refused = append(refused, conflict)
	}
	if len(refused) > 0 {
		s.line(s.r.TN("check.newlines-refused", len(refused)))
		s.drawNewlineConflicts(refused)
		return
	}
	if len(report.Rewrites) == 0 {
		s.line(s.r.T("check.newlines-nothing"))
	} else {
		heading := "check.newlines-would-rewrite"
		if report.Applied {
			heading = "check.newlines-rewritten"
		}
		s.line(s.r.TN(heading, len(report.Rewrites)))
		// Each destination is one sentence rather than one row of a table, on
		// the reasoning renderBranchMigration states for its own account: a
		// migration's report is read once by a person deciding whether to run
		// the command again, and a table here would owe the row-layout sweep a
		// fixture and a language pass for a block nobody scans.
		//
		// The tense is taken from the file rather than from the run, because a
		// confirmed run can carry a destination it did not reach: a file whose
		// lock was busy, or one a partial failure stopped short of. Saying
		// "repaired" over either would describe work that did not happen, and
		// saying "to repair" over a file whose bytes are already gone was the
		// other half of the same defect.
		//
		// The counts are pluralised, which needs one key per count, because a
		// plural category is chosen from a number and this line carries two.
		// The loose clause is drawn only where there is one, so the ordinary
		// destination reads as one sentence.
		for _, rewrite := range report.Rewrites {
			line := "check.newline-would-rewrite-file"
			if rewrite.Written {
				line = "check.newline-rewrote-file"
			}
			s.line(s.r.TN(line, rewrite.Returns, "path", rewrite.Path))
			if rewrite.Loose > 0 {
				s.line(s.r.TN("check.newline-rewrite-loose", rewrite.Loose))
			}
		}
	}
	if len(busy) > 0 {
		s.line(s.r.TN("check.newlines-busy", len(busy)))
		s.drawNewlineConflicts(busy)
	}
	if !report.Applied && len(report.Rewrites) > 0 {
		s.line(s.r.T("check.newlines-confirm"))
	}
}

// drawNewlineConflicts prints one line per refused destination, naming the
// condition it met and whatever the condition carries: the frontmatter key, the
// record number, or the lock's holder.
func (s *session) drawNewlineConflicts(conflicts []bench.NewlineConflict) {
	for _, conflict := range conflicts {
		key, named := newlineConflictKeys[conflict.Condition]
		if !named {
			continue
		}
		s.line(s.r.T(key, "path", conflict.Path, "detail", conflict.Detail))
	}
}

// designationNote is the line a full checklist read draws under a settled
// item: who wrote the designated comment, when, and what it says.
//
// A comment the store cannot attribute is drawn as one the store cannot name
// rather than as a blank or as an invented author, which is the whole of what
// recording the absence buys. The sentence comes from the catalog either way,
// so a reader meets it in their own language and the two cases are two
// sentences rather than one with a hole in it.
func (s *session) designationNote(designated *verb.DesignatedComment) string {
	if designated == nil {
		return ""
	}
	if designated.Author == "" {
		return s.r.T("show.designation.unattributed", "ts", designated.TS, "body", designated.Body)
	}
	return s.r.T("show.designation", "author", designated.Author, "ts", designated.TS, "body", designated.Body)
}
