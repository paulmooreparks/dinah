package main

import (
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// runView lists the views the caller can see, or draws one by name.
//
// Nothing reaches standard output before the library has answered in full, so
// a refused view prints no partial answer. A whole-view refusal a section met
// carries the view's name and the section's position, and the human form
// writes one line naming them to standard error after the refusal itself,
// because the refusal's name leads standard error on every refusal the tool
// reports and a script reading that first token must not meet anything else.
//
// --watch is refused before the workbench is opened when it is asked for in
// a machine form or with no view to draw, in that order, and before anything
// reaches the terminal when the terminal cannot be redrawn in place.
func runView(s *session, parsed *arguments) int {
	req := s.request("view", parsed)
	req.View = at(parsed.rest(), 0)
	req.Card = at(parsed.rest(), 1)
	req.Explain = parsed.has("explain")
	req.Lang = s.r.Tag
	req.All = parsed.has("all")
	req.ViewPlain = parsed.has("plain")
	req.ViewWatch = parsed.has("watch")
	if req.ViewWatch && s.format != formatHuman {
		return s.reportError(contract.Refuse(contract.Malformed, "--watch --json"))
	}
	if req.ViewWatch && req.View == "" {
		return s.reportError(contract.Refuse(contract.Malformed, "--watch"))
	}
	return s.withBench(func(l *verb.Library) int {
		if req.View == "" {
			listing, err := l.ListViews(req)
			if err != nil {
				return s.reportError(err)
			}
			return s.emitViewList(listing)
		}
		answer, err := l.DrawView(req)
		if err != nil {
			code := s.reportError(err)
			s.reportSectionRefused(l, req, err)
			return code
		}
		if s.format != formatHuman {
			return s.emitMachine(answer)
		}
		if req.ViewWatch {
			return s.watch(l, req)
		}
		return s.drawView(answer, l.Bench, req)
	})
}

// drawView draws a view once. The layout decides the drawing: a columns view
// is drawn as a board, and every other view in the list layout. --explain is
// the exception, since its blocks lay out no columns of their own, so a
// view asked to explain its ranks draws them whatever its layout, and
// --explain on one card draws that card's blocks alone.
func (s *session) drawView(answer *verb.ViewAnswer, b *bench.Bench, req *verb.Request) int {
	body := answer.View
	switch {
	case body.Explained && req.Card != "":
		s.renderExplainedCard(body, explainReaderFor(body, b))
		return 0
	case body.Layout == bench.ViewLayoutColumns && !body.Explained:
		glyphs := s.boardGlyphSet(req.ViewPlain)
		lines := s.columnsView(answer, b, glyphs, boardWindow(s.rawWidth), req.All, req.Card != "")
		return s.drawOnce(lines)
	}
	s.renderView(answer, b)
	return 0
}

// reportSectionRefused writes the line that says which view and which section
// a whole-view refusal came from, in the human form only. The machine form
// carries the same two values in the refusal's context map.
func (s *session) reportSectionRefused(l *verb.Library, req *verb.Request, err error) {
	refusal, ok := err.(*contract.Refusal)
	if !ok || s.format != formatHuman {
		return
	}
	name, section := refusal.Extra["view"], refusal.Extra["section"]
	if name == "" || section == "" {
		return
	}
	position, convErr := strconv.Atoi(section)
	if convErr != nil {
		return
	}
	title := l.SectionHeading(req, position)
	s.errLine(s.r.T("view.section-refused", "view", name, "section", section, "title", title))
}

// emitViewList answers dinah view with no name in whichever form was asked
// for. It mirrors dinah setup --list: one row per declaration, the shadowed
// ones included, with the first row of each name marked used.
func (s *session) emitViewList(listing *verb.ViewListing) int {
	if s.format != formatHuman {
		return s.emitMachine(listing)
	}
	views := table{indent: 2, columns: s.columns("view-list", "view", "title", "layout", "from", "used"), stackOnOverflow: true}
	for _, row := range listing.Views {
		title := row.Title
		if row.Malformed != "" {
			title = s.r.T("view.list.malformed", "defect", row.Malformed)
		}
		layout := row.Layout
		if layout == "" {
			layout = bench.ViewLayoutList
		}
		views.rows = append(views.rows, tableRow{fields: []string{row.Name, title, layout, row.Source, s.yesNo(row.Used)}})
	}
	s.table(views)
	return 0
}

// renderView draws a view in the list layout: a heading naming the view and
// who it was asked as, then each section's heading and its cards, one table
// per section because two sections can carry different columns.
func (s *session) renderView(answer *verb.ViewAnswer, b *bench.Bench) {
	operator := b.Operator
	body := answer.View
	heading := body.Title
	if body.Actor != "" {
		acting := s.r.T("view.acting", "actor", body.Actor)
		if operator != "" && body.Actor == operator {
			acting = s.r.T("view.acting.operator", "actor", body.Actor)
		}
		heading = s.rightAligned(body.Title, acting)
	}
	s.line(heading)
	s.line("")
	for i, section := range body.Sections {
		if i > 0 {
			s.line("")
		}
		s.line(s.r.T("view.section.heading", "title", section.Title, "count", strconv.Itoa(section.Count)))
		switch {
		case section.Refused != "":
			s.renderRefusedSection(section)
		case section.Count == 0:
			s.line(s.wrappedLine(2, s.r.T("view.section.empty")))
		case body.Order == bench.ViewOrderUrgency && body.Explained:
			s.renderExplainedSection(section, explainReaderFor(body, b))
		case body.Order == bench.ViewOrderUrgency:
			s.renderRankedSection(section, body.Actor)
		default:
			cards := table{
				indent:          2,
				columns:         s.columns("view", "card", "column", "item", "holder", "priority", "severity", "title"),
				stackOnOverflow: true,
				cutTail:         true,
			}
			viewSectionRows(&cards, section, body.Actor)
			s.table(cards)
		}
	}
}

// renderRefusedSection writes the lines a section this workbench could not
// ask is drawn with, each laid out at its indent.
func (s *session) renderRefusedSection(section verb.ViewSectionAnswer) {
	for _, refused := range s.refusedSectionLines(section) {
		s.line(s.wrappedLine(refused.indent, refused.text))
	}
}

// indentedText is a line of prose and the indent it is laid out at, before
// any width breaks it.
type indentedText struct {
	indent int
	text   string
}

// refusedSectionLines composes what a section this workbench could not ask
// is drawn with. The refusal is rebuilt from the section's own members and
// composed by the refusal composer with every fragment but the next step, so
// it names the legal values wherever the refusal lists them, and any rows the
// refusal lists are drawn beneath the line one step further in.
//
// The composer leads a refusal with its name and a space, which is what a
// reader of standard error scans for. Inside a sentence that already says the
// question was not asked, the name reads as a label, so it takes a colon.
func (s *session) refusedSectionLines(section verb.ViewSectionAnswer) []indentedText {
	refusal := contract.RefuseWith(section.Refused, section.RefusedDetail, section.RefusedContext)
	lines := s.composeRefusalWithout(refusal)
	if len(lines) == 0 {
		return nil
	}
	first := lines[0]
	if rest, named := strings.CutPrefix(first, refusal.Name+" "); named {
		first = refusal.Name + ": " + rest
	}
	composed := []indentedText{{indent: 2, text: s.r.T("view.section.refused", "refusal", first)}}
	for _, line := range lines[1:] {
		composed = append(composed, indentedText{indent: 4, text: strings.TrimLeft(line, " ")})
	}
	return composed
}

// viewSectionRows fills one section's table with its cards. The Item column
// is filled only on a section whose query names an item field, since only
// there does a card carry a witness; the Holder column only where some card
// is held by an owner other than the caller; and the Pri and Sev columns only
// where some card carries that level. The table drops a column no row fills,
// so each of those rules is a matter of leaving the cells empty.
func viewSectionRows(t *table, section verb.ViewSectionAnswer, actor string) {
	heldByOthers := false
	for _, card := range section.Cards {
		if card.Holder != "" && card.Holder != actor {
			heldByOthers = true
		}
	}
	for _, card := range section.Cards {
		holder := ""
		if heldByOthers {
			holder = card.Holder
		}
		fields := []string{card.Ref, card.ColumnTitle, witnessCell(card.Ref, section.Items[card.Ref]), holder, card.Priority, card.Severity, card.Title}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
}

// witnessCell names the first item that witnessed a card's selection, relative
// to the card, and how many others did, as in `questions/1 +1`.
func witnessCell(cardRef string, witnesses []string) string {
	if len(witnesses) == 0 {
		return ""
	}
	cell := strings.TrimPrefix(witnesses[0], cardRef+"/")
	if others := len(witnesses) - 1; others > 0 {
		cell += " +" + strconv.Itoa(others)
	}
	return cell
}
