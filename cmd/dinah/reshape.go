package main

import (
	"strconv"
	"strings"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// reportReshapeRefusal answers a reshape that failed, and it is the whole of
// how an operator learns that a run wrote before it stopped.
//
// A refusal raised during validation goes to reportError untouched, because
// nothing was written and there is nothing more to say. A refusal raised after
// the write phase began takes the other route: the report is printed or
// emitted first, saying what landed and how to finish it, and the refusal
// follows on stderr with its name leading the stream exactly as it always
// does. The machine form is the report rather than the bare refusal, and the
// report carries the refusal's own name in its refusal member, so a script
// reads one document naming both what stopped the run and what the run did.
func (s *session) reportReshapeRefusal(report *verb.ReshapeReport, err error) int {
	refusal, ok := err.(*contract.Refusal)
	if !ok || report == nil || !report.PartlyApplied() {
		return s.reportError(err)
	}
	if s.format != formatHuman {
		s.emitMachine(report)
	} else {
		s.renderReshape(report)
	}
	return s.writeRefusal(s.nameTheWorkbench(refusal))
}

// renderReshape prints what a reshape did, what it would do, or what it had
// done when it stopped.
//
// One report serves all three. A preview opens by saying that nothing was
// written, because a reader who has just watched a page of columns and card
// counts scroll past needs to know at the top that none of it happened, and an
// apply opens by saying that the new shape is on disk. Everything after that
// first line is the same page, and the counts a preview prints as what it
// would carry are the counts an apply prints as what it carried.
//
// A run that stopped part way through gets its own page instead, because the
// column table describes the shape the run was asked for and that is not the
// shape on disk. What it prints is what was written and what to do about it.
//
// It lives in a file of its own rather than at the foot of render.go, where
// its siblings are, because three fixtures in this package address a table by
// its own source line and a block added above one of them moves it.
func (s *session) renderReshape(report *verb.ReshapeReport) {
	if report.PartlyApplied() {
		s.renderReshapePartial(report)
		return
	}
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

// renderReshapePartial is the page for a run that wrote part of the new shape
// and then refused: the workbench is between two shapes, here is which parts
// landed, and here is what finishes it.
//
// It prints no column table. That table is the shape the run was asked for,
// and printing it under a run that stopped would tell the reader the flow now
// looks like something it does not. The steps below are the opposite reading:
// each names a thing that is on disk.
//
// Running dinah check instead of reading this is not the same answer, and a
// half-applied run has been made to prove it. A general checker describes the
// state it finds in its own vocabulary, so the added column, appended to the
// end of the sequence because the reorder never ran, reports as every done
// column standing where its kind does not allow. That names columns nobody
// touched, says nothing about a reshape, and is a worse lead than silence.
func (s *session) renderReshapePartial(report *verb.ReshapeReport) {
	s.line(s.r.T("reshape.partial"))
	for _, wrote := range report.Wrote {
		s.line(s.r.T("reshape.wrote."+wrote.Step, "count", strconv.Itoa(wrote.Count)))
	}
	s.line(s.r.T("reshape.retry"))
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
