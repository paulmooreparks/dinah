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

// emit reports a verb's canonical response: the machine form under --json,
// the rendering otherwise, and on any non-zero outcome the outcome's own
// token leading stderr.
func (s *session) emit(response *verb.Response) int {
	if response.Outcome != contract.OutcomeOK {
		s.reportOutcome(response)
	}
	if s.json {
		s.emitJSON(response)
		return contract.ExitCode(response.Outcome)
	}
	if response.Outcome != contract.OutcomeOK {
		return contract.ExitCode(response.Outcome)
	}
	if response.Warning != "" {
		io.WriteString(s.errw, s.r.T(response.Warning, "detail", response.WarningDetail)+"\n")
	}
	s.renderCard(response.Card)
	if response.Instructions != nil && !s.quiet {
		s.renderInstructions(response.Instructions, response.LegalMoves)
	}
	return 0
}

// reportOutcome writes the leading token and the sentence a person reads.
// For a refusal the token is the refusal name; for the other two non-zero
// outcomes it is the outcome name itself.
func (s *session) reportOutcome(response *verb.Response) {
	switch response.Outcome {
	case contract.OutcomeRefused:
		for _, line := range s.composeRefusal(contract.RefuseWith(response.Refusal, response.Detail, s.outcomeValues(response))) {
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

// emitJSON writes a value as the canonical machine form. The form carries
// canonical tokens only, so the same command under any language setting emits
// byte-identical JSON.
func (s *session) emitJSON(value any) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		io.WriteString(s.errw, contract.OutcomeUnreachable+" "+err.Error()+"\n")
		return contract.ExitCode(contract.OutcomeUnreachable)
	}
	io.WriteString(s.out, string(data)+"\n")
	return 0
}

// renderCard prints the one line a person needs after an act: where the card
// is, what it is called and what state it is in.
func (s *session) renderCard(card *verb.CardView) {
	if card == nil {
		return
	}
	s.line(s.r.T("card.line",
		"ref", card.Ref,
		"title", card.Title,
		"state", card.StateTitle,
		"substate", s.token(card.Substate),
	))
	if card.Holder != "" {
		s.line(s.r.T("card.holder", "holder", card.Holder))
	}
	if card.BlockReason != "" {
		s.line(s.r.T("card.blocked", "reason", card.BlockReason))
	}
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
// the chain serves them, with the legal moves after.
func (s *session) renderInstructions(instructions *verb.Instructions, moves []verb.LegalMove) {
	layers := []struct {
		label string
		text  string
	}{
		{label: "instructions.global", text: instructions.Global},
		{label: "instructions.standing", text: instructions.Standing},
		{label: "instructions.state", text: instructions.State},
	}
	for _, layer := range layers {
		if layer.text == "" {
			continue
		}
		s.line("")
		s.line(s.r.T(layer.label))
		s.write(layer.text)
	}
	if len(moves) == 0 {
		return
	}
	s.line("")
	s.line(s.r.T("instructions.moves"))
	t := table{indent: 2, columns: s.columns("moves", "state", "name", "direction")}
	for _, move := range moves {
		fields := []string{move.Ref, move.Title, s.token(move.Direction)}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// renderStatus prints where the bench stands.
func (s *session) renderStatus(status *verb.Status) {
	s.line(s.r.T("status.workbench",
		"title", status.Bench,
		"root", status.Root,
		"source", s.token(status.WorkbenchSource),
	))
	s.line(s.r.T("status.actor", "actor", status.Actor, "operator", s.yesNo(status.IsOperator)))
	s.line("")
	s.renderStates(status.States)
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

// yesNo renders a boolean for a person.
func (s *session) yesNo(value bool) string {
	if value {
		return s.r.T("word.yes")
	}
	return s.r.T("word.no")
}

// renderStates prints the flow in order with each station's occupancy.
func (s *session) renderStates(states []verb.StateView) {
	t := table{indent: 2, columns: s.columns("states", "slug", "name", "kind", "cards", "owner")}
	for _, state := range states {
		count := strconv.Itoa(state.Count)
		if state.Capacity > 0 {
			count += "/" + strconv.Itoa(state.Capacity)
		}
		owner := s.r.T("states.moved-by.agent")
		if state.OperatorOwned {
			owner = s.r.T("states.moved-by.operator")
		}
		fields := []string{s.slugCell(state.Slug), state.Title, s.token(state.Kind), count, owner}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// renderListing prints a state's cards in queue order.
func (s *session) renderListing(listing *verb.Listing) {
	if len(listing.Cards) == 0 {
		s.line(s.r.T("ls.empty"))
		return
	}
	t := table{indent: 2, columns: s.columns("ls", "card", "standing", "title")}
	for _, card := range listing.Cards {
		t.rows = append(t.rows, tableRow{fields: []string{card.Ref, s.token(card.Substate), card.Title}})
	}
	s.table(t)
}

// renderMatches prints the cards a query selected. A query spans the whole
// workbench where ls lists one state at a time, so the reader needs a state
// column that ls has no use for, and it carries the state's title rather than
// its identifier.
func (s *session) renderMatches(matches *verb.Matches) {
	if len(matches.Cards) == 0 {
		s.line(s.r.T("query.empty"))
		return
	}
	t := table{indent: 2, columns: s.columns("query", "card", "state", "standing", "title")}
	for _, card := range matches.Cards {
		fields := []string{card.Ref, card.StateTitle, s.token(card.Substate), card.Title}
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
	if len(tree.Root.Children) == 0 {
		return
	}
	t := table{indent: 2, columns: s.columns("tree", "reference", "entity", "title", "count", "hidden")}
	s.treeRows(&t, tree, tree.Root.Children, nil)
	s.table(t)
}

// treeHeader is the sentence above the table. Under a filter it says what the
// workbench holds and how much of that matched, so the first number is the
// root's count added to what the filter removed and the second is the count
// alone.
func (s *session) treeHeader(tree *verb.Tree) string {
	root := tree.Root
	count := strconv.Itoa(root.Count)
	if tree.Producer == verb.ProducerContainment {
		if root.Count == 0 {
			return s.r.T("contents.empty", "title", root.Title, "ref", root.Ref)
		}
		return s.r.T("contents.header", "title", root.Title, "ref", root.Ref, "count", count)
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
func (s *session) renderWorkbenches(rows []bench.Candidate) {
	if len(rows) == 0 {
		s.line(s.r.T("workbenches.empty"))
		return
	}
	for _, row := range s.formatCandidateRows(rows) {
		s.line(row)
	}
}

// formatCandidateRows renders each candidate as the padded title, slug and
// path columns dinah workbenches prints, one row per string with its own
// two-space lead. dinah.ambiguous-workbench prints the same rows beneath its
// opening sentence, so this is the one place the column widths live; the two
// callers can never draw the same candidates in different columns.
func (s *session) formatCandidateRows(rows []bench.Candidate) []string {
	t := table{indent: 2, columns: s.columns("workbenches", "workbench", "slug", "path")}
	for _, candidate := range rows {
		fields := []string{candidate.Title, s.slugCell(candidate.Slug), candidate.Path}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	return s.tableLines(t)
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

// renderOffers prints what each state offers next.
func (s *session) renderOffers(offers []verb.Offer) {
	t := table{indent: 2, columns: s.columns("next", "state", "card", "title")}
	for _, offer := range offers {
		if offer.Card == nil {
			t.rows = append(t.rows, tableRow{fields: []string{offer.Title, s.r.T("next.none")}})
			continue
		}
		fields := []string{offer.Title, offer.Card.Ref, offer.Card.Title}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// renderDetail prints a card, its links and its comments.
func (s *session) renderDetail(detail *verb.Detail) {
	s.renderCard(&detail.Card)
	if detail.Body != "" {
		s.line("")
		s.write(detail.Body)
	}
	if len(detail.Links) > 0 {
		s.line("")
		s.line(s.r.T("show.links"))
		links := table{indent: 2, columns: s.columns("links", "link", "card")}
		for _, link := range detail.Links {
			links.rows = append(links.rows, tableRow{fields: []string{link.Kind, link.Ref}})
		}
		s.table(links)
	}
	if len(detail.Comments) > 0 {
		s.line("")
		s.line(s.r.T("show.comments"))
		comments := table{indent: 2, columns: s.columns("comments", "when", "who")}
		for _, comment := range detail.Comments {
			fields := []string{comment.TS, comment.Author}
			comments.rows = append(comments.rows, tableRow{fields: fields, note: comment.Body})
		}
		s.table(comments)
	}
}

// renderHistory prints a card's acts in the order they were recorded. An
// identifier carried in an act is never resolved against the bench as it now
// stands, so the titles printed are the ones the act itself carries.
func (s *session) renderHistory(events []bench.Event) {
	t := table{indent: 2, columns: s.columns("log", "when", "action", "actor", "detail")}
	for _, ev := range events {
		var tail string
		switch ev.Event {
		case contract.EventMoved:
			tail = s.r.T("log.moved", "from", ev.FromTitle, "to", ev.ToTitle)
			if ev.Override {
				tail += " " + s.r.T("log.override")
			}
		case contract.EventBlocked:
			tail = ev.Reason
		case contract.EventCreated:
			tail = ev.Title
		}
		fields := []string{ev.TS, s.token(ev.Event), ev.Actor, tail}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
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
	}
	if report.StampedOrdinals != nil {
		s.line(s.r.TN("check.ordinal-stamped", *report.StampedOrdinals))
	}
	if report.MigratedStates {
		s.line(s.r.TN("check.states-removed", len(report.RemovedStrandedStates)))
		removed := table{indent: 2, columns: listColumn()}
		for _, id := range report.RemovedStrandedStates {
			removed.rows = append(removed.rows, tableRow{fields: []string{id}})
		}
		s.table(removed)
	}
	return s.renderFindings(report.Findings)
}

// renderFindings prints what check found and returns the exit code: zero on a
// clean bench, the refused code when anything was found.
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
	return contract.ExitCode(contract.OutcomeRefused)
}

// renderIdentity prints the actor and whether it is the operator.
func (s *session) renderIdentity(identity *verb.Identity) {
	s.line(s.r.T("whoami.line", "actor", identity.Actor, "operator", s.yesNo(identity.IsOperator)))
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
// A refusal raised before the workbench opens carries no states, which costs
// nothing in practice because unknown-state is only ever raised once one is
// open.
var refusalListings = map[string]func(*session) []string{
	"states": func(s *session) []string {
		if s.library == nil {
			return nil
		}
		rows := make([]string, 0, len(s.library.Bench.States))
		for _, state := range s.library.Bench.States {
			rows = append(rows, state.Ref())
		}
		return rows
	},
	"guides":   func(s *session) []string { return guide.Topics() },
	"settings": func(s *session) []string { return bench.ConfigKeys },
}

// refusalBlocks are the listings that arrive already laid out, because their
// rows carry columns rather than bare names and one place already draws them.
var refusalBlocks = map[string]func(*session) []string{
	"workbenches": func(s *session) []string {
		rows, _ := bench.Reachable(s.cwd, s.benchFlag, s.home, s.nativeHome)
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
		key += ".unnamed"
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

// renderWorkbenchFields prints the workbench's own fields, one row per field.
//
// The field names travel untranslated, the way `config`'s keys do, because a
// field name is machine vocabulary a caller types back. The slug row is served
// through slugCell, so a workbench carrying none names its repair rather than
// standing blank, and this listing says what `dinah states` and `dinah
// workbenches` already say about a missing slug.
func (s *session) renderWorkbenchFields(fields *verb.WorkbenchView) {
	t := table{indent: 2, columns: s.columns("workbench", "field", "value")}
	for _, name := range bench.WorkbenchFields {
		value := fields.Field(name)
		if name == "slug" {
			value = s.slugCell(value)
		}
		t.rows = append(t.rows, tableRow{fields: []string{name, value}})
	}
	s.table(t)
}
