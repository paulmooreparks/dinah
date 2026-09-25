package main

import (
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// renderRankedSection draws one section of a view ordered by urgency in the
// list layout: a table whose rows carry each card's rank, its score and the
// terms that moved it.
func (s *session) renderRankedSection(section verb.ViewSectionAnswer, actor string) {
	ranked := table{
		indent:          2,
		columns:         s.columns("view", "rank", "card", "column", "item", "holder", "urgency", "why", "title"),
		stackOnOverflow: true,
		cutTail:         true,
	}
	s.rankedSectionRows(&ranked, section, actor)
	s.table(ranked)
}

// rankedSectionRows fills one section's table on a view ordered by urgency.
// It keeps viewSectionRows' rules for the Item and Holder columns and draws
// no level columns, since the Why cell names both levels.
//
// The Rank and Urgency cells start under their headings like every other
// cell. The table has no right alignment, and padding a cell on the left
// would start it past its heading, which the head's column checks refuse
// wherever two figures differ in width. Every figure carries one decimal
// digit, so the decimal points line up wherever the whole parts agree in
// width.
func (s *session) rankedSectionRows(t *table, section verb.ViewSectionAnswer, actor string) {
	heldByOthers := false
	for _, card := range section.Cards {
		if card.Holder != "" && card.Holder != actor {
			heldByOthers = true
		}
	}
	for _, card := range section.Cards {
		answer := section.Urgency[card.Ref]
		holder := ""
		if heldByOthers {
			holder = card.Holder
		}
		fields := []string{
			strconv.Itoa(answer.Rank),
			card.Ref,
			card.ColumnTitle,
			witnessCell(card.Ref, section.Items[card.Ref]),
			holder,
			answer.Score.String(),
			s.whyCell(answer.Contributing),
			card.Title,
		}
		t.rows = append(t.rows, tableRow{fields: fields})
	}
}

// whyCell names every term that moved a card, in term order, joined by a
// comma and a space. A level term is named by the level itself, and the two
// counted terms say how many they counted.
func (s *session) whyCell(terms []verb.UrgencyTerm) string {
	labels := make([]string, 0, len(terms))
	for _, term := range terms {
		labels = append(labels, s.whyLabel(term))
	}
	return strings.Join(labels, ", ")
}

// whyLabel is the phrase the Why cell names one term by.
func (s *session) whyLabel(term verb.UrgencyTerm) string {
	switch term.Term {
	case bench.UrgencyPriority, bench.UrgencySeverity:
		return term.Basis["level"]
	case bench.UrgencyYourQuestion:
		return s.r.TN("view.urgency.why.your-question", basisCount(term, "counted"))
	case bench.UrgencyAge:
		return s.r.TN("view.urgency.why.age", basisCount(term, "days"))
	case bench.UrgencyWaitsOnYou:
		return s.r.T("view.urgency.why.waits-on-you")
	case bench.UrgencyBlocked:
		return s.r.T("view.urgency.why.blocked")
	case bench.UrgencyStaleClaim:
		return s.r.T("view.urgency.why.stale-claim")
	}
	return s.r.T("view.urgency.term." + term.Term)
}

// basisCount reads a whole-number basis value, zero where it does not read.
func basisCount(term verb.UrgencyTerm, key string) int {
	count, err := strconv.Atoi(term.Basis[key])
	if err != nil {
		return 0
	}
	return count
}

// renderExplainedCard draws --explain for one card: its reference and title,
// then every term behind its rank. A card appearing in several sections is
// explained once, from the first section that holds it, since its score does
// not depend on the section.
func (s *session) renderExplainedCard(body verb.ViewBody, reader explainReader) {
	for _, section := range body.Sections {
		for _, card := range section.Cards {
			answer, ranked := section.Urgency[card.Ref]
			if !ranked {
				continue
			}
			s.line(s.wrappedLine(0, s.r.T("view.urgency.explain.card", "card", card.Ref, "title", card.Title)))
			s.explainTerms(card, answer, 2, reader)
			return
		}
	}
}

// renderExplainedSection replaces a section's table with one block per card
// in rank order: the rank, the reference and the title on one line, then the
// card's terms indented beneath it, with a blank line between blocks.
func (s *session) renderExplainedSection(section verb.ViewSectionAnswer, reader explainReader) {
	for i, card := range section.Cards {
		if i > 0 {
			s.line("")
		}
		answer := section.Urgency[card.Ref]
		heading := s.r.T("view.urgency.explain.ranked", "rank", strconv.Itoa(answer.Rank), "card", card.Ref, "title", card.Title)
		s.line(s.wrappedLine(2, heading))
		s.explainTerms(card, answer, 4, reader)
	}
}

// explainTerms draws every term of one card as a table at the given indent:
// the term's label, the phrase that says what it read, and its signed
// figure, then a rule and the total. A term that contributed nothing still
// prints, so a reader can see that a value was read and found empty. The
// table draws no heading row, since the labels in its first column already
// say what each row is, and its columns keep their headings for the stacked
// form a narrow window draws.
func (s *session) explainTerms(card verb.CardView, answer verb.UrgencyAnswer, indent int, reader explainReader) {
	terms := table{
		indent:  indent,
		columns: s.columns("explain", "term", "reading", "points"),
		labels:  labelInTheStack,
	}
	for _, term := range answer.Terms {
		fields := []string{
			s.r.T("view.urgency.term." + term.Term),
			s.explainPhrase(card, term, reader),
			signed(term.Points.String()),
		}
		terms.rows = append(terms.rows, tableRow{fields: fields})
	}
	total := tableRow{
		fields:    []string{s.r.T("view.urgency.total"), s.r.T("view.urgency.explain.total"), answer.Score.String()},
		ruleAbove: true,
	}
	terms.rows = append(terms.rows, total)
	s.table(terms)
}

// signed writes a term's figure with its sign, adding a plus sign to every
// figure that is not negative. A total is printed without it.
func signed(figure string) string {
	if strings.HasPrefix(figure, "-") {
		return figure
	}
	return "+" + figure
}

// explainPhrase is what one term read, in the reader's language, composed
// from the term's basis and the card's column title.
func (s *session) explainPhrase(card verb.CardView, term verb.UrgencyTerm, reader explainReader) string {
	basis := term.Basis
	switch term.Term {
	case bench.UrgencyWaitsOnYou:
		return s.waitsOnYouPhrase(card, basis, reader)
	case bench.UrgencyYourQuestion:
		owned := basisCount(term, "owned")
		if owned == 0 {
			return s.r.T("view.urgency.explain.your-question.none")
		}
		return s.r.TN("view.urgency.explain.your-question", owned, "counted", basis["counted"], "per-item", basis["per-item"])
	case bench.UrgencyPriority, bench.UrgencySeverity:
		return s.levelPhrase(term)
	case bench.UrgencyBlocked:
		return s.blockedPhrase(basis)
	case bench.UrgencyBlocksOthers:
		if basis["read"] != "true" {
			return s.r.T("view.urgency.explain.blocks-others")
		}
		return s.r.TN("view.urgency.explain.blocks-others.counted", basisCount(term, "counted"))
	case bench.UrgencyAge:
		if basis["arrival"] == "" {
			return s.r.T("view.urgency.explain.age.unknown")
		}
		return s.r.TN("view.urgency.explain.age", basisCount(term, "days"), "column", card.ColumnTitle, "per-day", basis["per-day"])
	case bench.UrgencyStaleClaim:
		if basis["expired"] == "" {
			return s.r.T("view.urgency.explain.stale-claim.none")
		}
		return s.r.T("view.urgency.explain.stale-claim", "holder", basis["holder"], "expired", basis["expired"])
	}
	return ""
}

// waitsOnYouPhrase says why the waits-on-you term scored what it scored. The
// term never applies to anybody but the operator, so every other caller is
// told that rather than told a column is not theirs. For the operator, a card
// in a done column he owns scores nothing because the work there is finished,
// and the phrase says so rather than saying the column is not his.
func (s *session) waitsOnYouPhrase(card verb.CardView, basis map[string]string, reader explainReader) string {
	switch {
	case !reader.operator:
		return s.r.T("view.urgency.explain.waits-on-you.agent")
	case basis["owned"] == "true":
		return s.r.T("view.urgency.explain.waits-on-you.owned", "column", card.ColumnTitle)
	case reader.finished[card.Column]:
		return s.r.T("view.urgency.explain.waits-on-you.finished", "column", card.ColumnTitle)
	}
	return s.r.T("view.urgency.explain.waits-on-you.unowned", "column", card.ColumnTitle)
}

// explainReader is what the explained phrases need beyond a card's terms:
// whether the view was drawn for the operator, and which columns are done
// columns the operator owns, where a card earns nothing for waiting on him.
type explainReader struct {
	operator bool
	finished map[string]bool
}

// explainReaderFor reads who a view was drawn for and the workbench's done
// columns the operator owns.
func explainReaderFor(body verb.ViewBody, b *bench.Bench) explainReader {
	reader := explainReader{
		operator: body.Actor != "" && body.Actor == b.Operator,
		finished: map[string]bool{},
	}
	for _, column := range b.Columns {
		if column.OperatorOwned && column.Terminal() {
			reader.finished[column.ID] = true
		}
	}
	return reader
}

// levelPhrase says which level a priority or severity term read, or that it
// read none, or that the name the card carries is not declared.
func (s *session) levelPhrase(term verb.UrgencyTerm) string {
	basis := term.Basis
	switch {
	case basis["level"] == "":
		return s.r.T("view.urgency.explain." + term.Term + ".none")
	case basis["declared"] != "true":
		return s.r.T("view.urgency.explain.level.undeclared", "level", basis["level"])
	}
	return s.r.T("view.urgency.explain.level", "level", basis["level"], "rank", basis["rank"], "of", basis["of"])
}

// blockedPhrase says whether the card is blocked, and of what kind.
func (s *session) blockedPhrase(basis map[string]string) string {
	switch {
	case basis["blocked"] != "true":
		return s.r.T("view.urgency.explain.blocked.not")
	case basis["kind"] == "":
		return s.r.T("view.urgency.explain.blocked")
	}
	return s.r.T("view.urgency.explain.blocked.kind", "kind", basis["kind"])
}
