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
		io.WriteString(s.errw, response.Refusal+" "+s.sentence(response.Refusal, response.Detail)+"\n")
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
	for _, move := range moves {
		s.row(row{
			indent: 2,
			cells:  []cell{{move.Ref, 14}, {move.Title, 32}},
			tail:   s.token(move.Direction),
		})
	}
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
		for _, card := range status.Holding {
			s.row(row{indent: 2, cells: []cell{{card.Ref, 14}}, tail: card.Title})
		}
	}
	if len(status.Blocked) > 0 {
		s.line("")
		s.line(s.r.T("status.blocked"))
		for _, card := range status.Blocked {
			s.row(row{indent: 2, cells: []cell{{card.Ref, 14}}, tail: card.BlockReason})
		}
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
	for _, state := range states {
		count := strconv.Itoa(state.Count)
		if state.Capacity > 0 {
			count += "/" + strconv.Itoa(state.Capacity)
		}
		tail := ""
		if state.OperatorOwned {
			tail = s.r.T("states.operator-owned")
		}
		s.row(row{
			indent: 2,
			cells: []cell{
				{state.ID, 14},
				{s.slugCell(state.Slug), 24},
				{state.Title, 32},
				{s.token(state.Kind), 10},
				{count, 8},
			},
			tail: tail,
		})
	}
}

// renderListing prints a state's cards in queue order.
func (s *session) renderListing(listing *verb.Listing) {
	if len(listing.Cards) == 0 {
		s.line(s.r.T("ls.empty"))
		return
	}
	for _, card := range listing.Cards {
		s.row(row{
			indent: 2,
			cells:  []cell{{card.Ref, 14}, {s.token(card.Substate), 10}},
			tail:   card.Title,
		})
	}
}

// renderSettings prints each setting with the value in force and the rung of
// its ladder that produced it. A setting no rung carried prints an empty
// value beside the source that says so, because the row itself is the answer
// to whether anybody has ever set the key.
func (s *session) renderSettings(settings []verb.SettingView) {
	for _, view := range settings {
		s.row(row{
			indent: 2,
			cells:  []cell{{view.Key, 12}, {view.Value, 24}},
			tail:   s.token(view.Source),
		})
	}
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
	lines := make([]string, 0, len(rows))
	for _, candidate := range rows {
		lines = append(lines, s.rowLine(row{
			indent: 2,
			cells:  []cell{{candidate.Title, 32}, {s.slugCell(candidate.Slug), 16}},
			tail:   candidate.Path,
		}))
	}
	return lines
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
	for _, offer := range offers {
		if offer.Card == nil {
			s.row(row{
				indent: 2,
				cells:  []cell{{offer.Title, 32}},
				tail:   s.r.T("next.none"),
			})
			continue
		}
		s.row(row{
			indent: 2,
			cells:  []cell{{offer.Title, 32}, {offer.Card.Ref, 14}},
			tail:   offer.Card.Title,
		})
	}
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
		for _, link := range detail.Links {
			s.row(row{indent: 2, cells: []cell{{link.Kind, 14}}, tail: link.Ref})
		}
	}
	if len(detail.Comments) > 0 {
		s.line("")
		s.line(s.r.T("show.comments"))
		for _, comment := range detail.Comments {
			s.row(row{indent: 2, cells: []cell{{comment.TS, 22}}, tail: comment.Author})
			s.write(comment.Body)
		}
	}
}

// renderHistory prints a card's acts in the order they were recorded. An
// identifier carried in an act is never resolved against the bench as it now
// stands, so the titles printed are the ones the act itself carries.
func (s *session) renderHistory(events []bench.Event) {
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
		s.row(row{
			indent: 2,
			cells: []cell{
				{ev.TS, 22},
				{s.token(ev.Event), 14},
				{ev.Actor, 16},
			},
			tail: tail,
		})
	}
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
		for _, assignment := range report.AssignedSlugs {
			s.row(row{
				indent: 2,
				cells:  []cell{{assignment.Slug, 24}},
				tail:   assignment.Title,
			})
		}
		if report.AssignedWorkbenchSlug != nil {
			s.line(s.r.T("check.workbench-slug-assigned", "slug", report.AssignedWorkbenchSlug.Slug))
		}
	}
	if report.StampedOrdinals != nil {
		s.line(s.r.TN("check.ordinal-stamped", *report.StampedOrdinals))
	}
	if report.MigratedStates {
		s.line(s.r.TN("check.states-removed", len(report.RemovedStrandedStates)))
		for _, id := range report.RemovedStrandedStates {
			s.row(row{indent: 2, tail: id})
		}
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
	for _, finding := range findings {
		s.row(row{indent: 2, tail: s.r.T(finding.Key, "detail", finding.Detail) + " (" + finding.Path + ")"})
	}
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
	for _, catalog := range release.Catalogs {
		coverage := strconv.Itoa(catalog.Translated) + "/" + strconv.Itoa(catalog.Total)
		s.row(row{indent: 2, cells: []cell{{catalog.Tag, 8}}, tail: coverage})
	}
}
