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
	if response.Card == nil && response.Message != "" {
		values := flattenMessageValues(response.MessageValues)
		s.line(s.r.T(response.Message, values...))
		return 0
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
// is, what it is called, what state it is in, and which workstreams it belongs
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
		"state", card.StateTitle,
		"substate", s.token(card.Substate),
	}
	key := "card.line"
	if len(card.Workstreams) > 0 {
		key = "card.line.workstreams"
		values = append(values, "workstreams", s.workstreamsCell(card.Workstreams))
	}
	s.line(s.r.T(key, values...))
	if card.Severity != "" {
		s.line(s.r.T("card.severity", "severity", card.Severity))
	}
	if card.Priority != "" {
		s.line(s.r.T("card.priority", "priority", card.Priority))
	}
	if card.Holder != "" {
		s.line(s.r.T("card.holder", "holder", card.Holder))
	}
	if card.BlockReason != "" {
		s.line(s.r.T("card.blocked", "reason", card.BlockReason))
	}
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
	t := table{indent: 2, columns: s.columns("moves", "state", "name", "direction", "reject")}
	for _, move := range moves {
		fields := []string{move.Ref, move.Title, s.token(move.Direction), s.yesNo(move.Reject)}
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
	t := table{indent: 2, columns: s.columns("states", "slug", "name", "kind", "cards", "work", "owner")}
	for _, state := range states {
		count := strconv.Itoa(state.Count)
		if state.Capacity > 0 {
			count += "/" + strconv.Itoa(state.Capacity)
		}
		owner := s.r.T("states.moved-by.agent")
		if state.OperatorOwned {
			owner = s.r.T("states.moved-by.operator")
		}
		// The Work cell answers whether work is taken up at the state and
		// the Owner cell answers who may move a card out of it. They are two
		// questions, so they are two cells, and a state where nobody takes
		// work up and nobody owner-owns still reads agent under Owner.
		//
		// The three values run most specific first. A state declaring the
		// flag reads waiting, because a reader is told the workbench is
		// waiting on somebody; a state where no owner takes work up for any
		// other reason reads none taken; every other state reads taken.
		work := s.r.T("states.work.taken")
		if !state.TakesWorkUp {
			work = s.r.T("states.work.none")
		}
		if state.AwaitingOutside {
			work = s.r.T("states.work.waiting")
		}
		fields := []string{s.slugCell(state.Slug), state.Title, s.token(state.Kind), count, work, owner}
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
	t := table{indent: 2, columns: s.columns("ls", "card", "standing", "severity", "priority", "title")}
	for _, card := range listing.Cards {
		t.rows = append(t.rows, tableRow{fields: []string{card.Ref, s.token(card.Substate), card.Severity, card.Priority, card.Title}})
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
	t := table{indent: 2, columns: s.columns("next", "state", "card", "title", "take")}
	for _, offer := range offers {
		if offer.Card == nil {
			// Three empty answers, most specific first. A state that waits on
			// somebody outside says who it waits on. A state where no act
			// could take a card up says so, which is a different fact from
			// nothing ready, because a done state holding four ready cards
			// offers none of them. Everything else has nothing ready.
			absent := s.r.T("next.none")
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
		// state beyond is what moves the card instead.
		take := s.r.T("next.take.claim")
		if offer.TakenByPull {
			take = s.r.T("next.take.pull")
		}
		fields := []string{offer.Title, offer.Card.Ref, offer.Card.Title, take}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// renderDetail prints a card, its links, its attachments and its comments.
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
	if len(detail.Attachments) > 0 {
		s.line("")
		s.line(s.r.T("show.attachments"))
		attachments := table{indent: 2, columns: s.columns("attachments", "position", "filename", "description")}
		for _, attachment := range detail.Attachments {
			attachments.rows = append(attachments.rows, tableRow{fields: []string{
				strconv.Itoa(attachment.Ordinal),
				attachment.Filename,
				attachment.Description,
			}})
		}
		s.table(attachments)
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
		fields := []string{ev.TS, s.token(ev.Event), ev.Actor, s.eventDetail(ev)}
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
	case contract.EventBlocked:
		return ev.Reason
	case contract.EventCreated:
		return ev.Title
	case contract.EventAttached, contract.EventAttachmentReplaced, contract.EventAttachmentRemoved:
		return ev.Filename
	case contract.EventAttachmentRenamed:
		return s.r.T("log.attachment-renamed", "from", ev.From, "to", ev.Filename)
	}
	return ""
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
	t := table{indent: 2, columns: s.columns("changes", "when", "card", "action", "actor", "detail")}
	for _, ev := range set.Events {
		// ChangeEvent embeds bench.Event, so ev.Event is the whole line and
		// ev.Event.Event is the act's own name. The two are spelled apart
		// here rather than aliased, since the shape is the one the machine
		// surface publishes.
		fields := []string{ev.TS, changeSubject(ev), s.token(ev.Event.Event), ev.Actor, s.eventDetail(ev.Event)}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
	s.line(s.r.T("changes.cursor", "cursor", set.Cursor))
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
	} else if shape.Carried != "" {
		carried := values[shape.Carried]
		if carried != "" {
			// The raise site joins references that are never empty, so
			// every field of the split is a row and none is skipped.
			t := table{indent: 2, columns: listColumn()}
			for _, member := range strings.Split(carried, "\n") {
				t.rows = append(t.rows, tableRow{fields: []string{member}})
			}
			rows = s.tableLines(t)
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

// renderWorkstreams prints every live workstream of the workbench, and the
// sentence that says so when the workbench carries none. The columns are the
// shape dinah states already draws, and a workstream carrying no slug prints
// through slugCell rather than as a blank.
func (s *session) renderWorkstreams(listing *verb.WorkstreamListing) {
	if len(listing.Workstreams) == 0 {
		s.line(s.r.T("workstreams.empty"))
		return
	}
	t := table{indent: 2, columns: s.columns("workstreams", "slug", "name", "status", "cards")}
	for _, workstream := range listing.Workstreams {
		fields := []string{
			s.slugCell(workstream.Slug),
			workstream.Title,
			workstream.Status,
			strconv.Itoa(workstream.Cards),
		}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
	s.table(t)
}

// renderWorkstreamDetail prints one workstream's own fields, its notes, and
// the live cards belonging to it.
//
// The field names travel untranslated, the way the workbench listing's do,
// because a field name is machine vocabulary a caller types back.
func (s *session) renderWorkstreamDetail(detail *verb.WorkstreamDetail) {
	workstream := detail.Workstream
	fields := table{indent: 2, columns: s.columns("workstream", "field", "value")}
	rows := [][]string{
		{"slug", s.slugCell(workstream.Slug)},
		{"id", workstream.ID},
		{"title", workstream.Title},
		{"status", workstream.Status},
		{"cards", strconv.Itoa(workstream.Cards)},
	}
	for _, row := range rows {
		fields.rows = append(fields.rows, tableRow{fields: row})
	}
	s.table(fields)
	if detail.Body != "" {
		s.line("")
		s.write(detail.Body)
	}
	if len(detail.Cards) == 0 {
		return
	}
	s.line("")
	members := table{indent: 2, columns: s.columns("workstream", "card", "title", "state")}
	for _, card := range detail.Cards {
		members.rows = append(members.rows, tableRow{fields: []string{card.Ref, card.Title, card.StateTitle}})
	}
	s.table(members)
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
// composer reads its states to list them.
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
