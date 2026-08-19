package main

import (
	"encoding/json"
	"io"
	"strconv"

	"dinah/internal/bench"
	"dinah/internal/contract"
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
		io.WriteString(s.errw, response.Refusal+" "+s.sentence(response.Refusal, response.Verb, response.Detail)+"\n")
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
