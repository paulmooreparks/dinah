package main

import (
	"strconv"
	"strings"

	"dinah/internal/verb"
)

// renderReshape prints what a reshape did, or what it would do.
//
// One report serves both forms. A preview opens by saying that nothing was
// written, because a reader who has just watched a page of columns and card
// counts scroll past needs to know at the top that none of it happened, and an
// apply opens by saying that the new shape is on disk. Everything after that
// first line is the same page, and the counts a preview prints as what it
// would carry are the counts an apply prints as what it carried.
//
// It lives in a file of its own rather than at the foot of render.go, where
// its siblings are, because three fixtures in this package address a table by
// its own source line and a block added above one of them moves it.
func (s *session) renderReshape(report *verb.ReshapeReport) {
	if report.Applied {
		s.line(s.r.T("reshape.applied"))
	} else {
		s.line(s.r.T("reshape.preview"))
	}
	for _, column := range report.Columns {
		s.line(s.reshapeColumnLine(column))
	}
	for _, retirement := range report.Retirements {
		s.reshapeRetirementLines(retirement)
	}
	for _, id := range report.Updated {
		s.line(s.r.T("reshape.updated", "id", id))
	}
	if report.StrandedCards > 0 {
		s.line(s.r.T("reshape.stranded", "count", strconv.Itoa(report.StrandedCards)))
	}
	if !report.Applied {
		s.line(s.r.T("reshape.confirm"))
	}
}

// reshapeColumnLine is one column's row: what the run does to it, and what it
// is called once the run has finished.
//
// An adopted entry has a line of its own because it names no live column, so
// there is no title to print and nothing on disk the reader could open. What
// they have instead is the identifier their own cards still carry, which is
// what the sentence gives them.
func (s *session) reshapeColumnLine(column verb.ReshapeColumn) string {
	if column.Disposition == verb.ReshapeAdopted {
		return s.r.T("reshape.column.adopted", "id", column.ID)
	}
	return s.r.T("reshape.column."+column.Disposition, "id", column.ID, "title", column.Title)
}

// reshapeRetirementLines are the rows for one retirement: where its cards go
// and how many there are, and the cards among them carrying a block.
//
// A blocked card is named rather than counted, because a reshape carries a
// block through untouched and a reader who sees only a count has no way to
// tell which card is still waiting on whatever blocked it.
func (s *session) reshapeRetirementLines(retirement verb.ReshapeRetirement) {
	if retirement.Destination == "" {
		s.line(s.r.T("reshape.empty", "id", retirement.ID))
		return
	}
	s.line(s.r.T("reshape.carry",
		"id", retirement.ID,
		"destination", retirement.DestinationTitle,
		"count", strconv.Itoa(retirement.Cards),
	))
	if len(retirement.Blocked) > 0 {
		s.line(s.r.T("reshape.blocked", "cards", strings.Join(retirement.Blocked, ", ")))
	}
}
