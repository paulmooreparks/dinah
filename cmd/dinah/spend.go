package main

import (
	"strconv"
	"strings"

	"dinah/internal/verb"
)

// runSpend records what a station spent on a card, or reports what was spent
// on a card or on the workbench. The library decides which by reading the
// request, so the two heads agree about what a call is.
func runSpend(s *session, parsed *arguments) int {
	words := parsed.rest()
	req := s.request("spend", parsed)
	req.Card = at(words, 0)
	req.Unit = at(words, 1)
	req.Input = parsed.value("input")
	req.Output = parsed.value("output")
	req.Cached = parsed.value("cached")
	req.Total = parsed.value("total")
	req.Unreported = parsed.has("unreported")
	req.By = parsed.value("by")
	req.Round = parsed.value("round")
	req.Column = parsed.value("column")
	req.Note = parsed.value("note")
	return s.withBench(func(l *verb.Library) int {
		switch answer := l.SpendCall(req).(type) {
		case *verb.Response:
			return s.emit(answer)
		case *verb.SpendReport:
			if s.format != formatHuman {
				return s.emitMachine(answer)
			}
			s.renderSpend(answer)
			return 0
		}
		return 0
	})
}

// renderSpend draws a spend report: the card's lines where the report is a
// card's, then the totals, and a sentence where there is nothing to draw.
//
// The figures of a row are folded into its last column rather than spread
// over one column each, because a provider reports any subset of the four
// and a table with a column per figure is mostly empty cells and wider than
// a window. The folded cell names each figure by the flag word it was typed
// under, which is machine vocabulary and needs no translation, and it wraps
// between words where the window is narrow. A consumer or a column the row
// carries none of draws a dash rather than an empty cell, because a row is
// read across its columns and an empty cell in the middle of one leaves a
// reader counting gutters.
func (s *session) renderSpend(report *verb.SpendReport) {
	if len(report.Totals) == 0 {
		if report.Card != "" {
			s.line(s.r.T("spend.none", "ref", report.Card))
		} else {
			s.line(s.r.T("spend.none.workbench"))
		}
		return
	}
	if len(report.Records) > 0 {
		s.line(s.r.T("spend.records"))
		records := table{indent: 2, columns: s.columns("spend", "when", "column", "by", "unit", "figures"), wrapTail: true}
		for _, record := range report.Records {
			fields := []string{
				record.TS, spendText(record.ColumnTitle), spendConsumer(record.Provider, record.Model), record.Unit,
				spendFigures(record.SpendFigures, record.Unreported, record.Round, record.Note),
			}
			records.rows = append(records.rows, tableRow{fields: fields})
		}
		s.table(records)
		s.line("")
	}
	s.line(s.r.T("spend.totals"))
	// The counts ride in the folded cell after the sums rather than in two
	// columns of their own, because a heading wider than every count under it
	// would widen the table past what the rows need and push the sums into a
	// wrap the rows themselves never asked for.
	totals := table{indent: 2, columns: s.columns("spend", "column", "by", "unit", "figures"), wrapTail: true}
	for _, total := range report.Totals {
		counted := s.r.T("spend.counted", "records", strconv.Itoa(total.Records), "unreported", strconv.Itoa(total.Unreported))
		fields := []string{
			spendText(total.ColumnTitle), spendConsumer(total.Provider, total.Model), total.Unit,
			spendFigures(total.SpendFigures, false, 0, "") + "; " + counted,
		}
		totals.rows = append(totals.rows, tableRow{fields: fields})
	}
	s.table(totals)
}

// spendAbsent is what a cell draws where the report carries no value for it.
// It is a glyph rather than a word, so it needs no translation, and it is
// never confused with a figure, since a figure is never a bare dash.
const spendAbsent = "-"

// spendText draws a text cell, or the absent glyph where the report carries
// none.
func spendText(text string) string {
	if text == "" {
		return spendAbsent
	}
	return text
}

// spendFigures folds a row's figures into one cell: each figure the row
// carries, named by its flag word, then the unreported marker and the round
// where the row carries them, all joined by commas, then the note after a
// semicolon. A row carrying no figure and no marker draws the absent glyph,
// which is what a total over lines that were all unreported comes to.
func spendFigures(figures verb.SpendFigures, unreported bool, round int, note string) string {
	var parts []string
	add := func(word string, figure *float64) {
		if figure != nil {
			parts = append(parts, word+" "+strconv.FormatFloat(*figure, 'f', -1, 64))
		}
	}
	add("input", figures.Input)
	add("output", figures.Output)
	add("cached", figures.Cached)
	add("total", figures.Total)
	if unreported {
		parts = append(parts, "unreported")
	}
	if round > 0 {
		parts = append(parts, "round "+strconv.Itoa(round))
	}
	text := strings.Join(parts, ", ")
	if text == "" {
		text = spendAbsent
	}
	if note != "" {
		text += "; " + note
	}
	return text
}

// spendConsumer draws a consumer as provider/model, which is how the by flag
// takes it, draws whichever half is known where one is absent, and draws the
// absent glyph where neither is.
func spendConsumer(provider, model string) string {
	switch {
	case provider != "" && model != "":
		return provider + "/" + model
	case provider != "":
		return provider
	case model != "":
		return model
	default:
		return spendAbsent
	}
}
