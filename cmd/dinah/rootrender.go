package main

import (
	"dinah/internal/bench"
	"dinah/internal/verb"
)

// The root-scoped renderers draw what a read answered for every workbench
// beneath one directory. Each is the same two moves: a heading naming the
// workbench, then that workbench's own single-workbench rendering, or nothing
// further when the row carries a refusal in place of an answer.
//
// The inner rendering is the existing single-workbench renderer, called
// unchanged. That is the point of the wrapping design rather than an economy:
// what a caller reads under one heading is what a caller reading that one
// workbench on its own would have read, so the two forms cannot drift.

// rootHeading names the workbench whose answer comes next, so two workbenches'
// rows are never printed with nothing between them to say which is which. It
// reports whether the row carries a refusal, which is what tells the caller
// there is nothing further to draw for it.
func (s *session) rootHeading(candidate bench.Candidate) bool {
	if candidate.Refused != "" {
		s.line(s.r.T("root.workbench.refused",
			"refusal", s.refusedCell(candidate.Refused),
			"path", candidate.Path,
		))
		return true
	}
	s.line(s.r.T("root.workbench",
		"title", candidate.Title,
		"slug", s.slugCell(candidate.Slug),
		"path", candidate.Path,
	))
	return false
}

// A workbench's own rendering follows its heading unchanged and unindented.
//
// Indenting the block would mean prefixing lines this head had already laid
// out, and a columnar line is laid out in one place: the tables inside each
// rendering already sit in from the margin, and a hand-built prefix counts
// characters where a terminal counts columns. Leaving it alone costs a level
// of nesting and buys the stronger property, which is that what a reader sees
// under one heading is byte for byte what reading that workbench on its own
// prints.

// rootEmpty says a walk found no workbench at all, which is an ordinary answer
// rather than a refusal: a directory with nothing beneath it is a fact, and the
// command exits 0 having reported it.
func (s *session) rootEmpty(root string) {
	s.line(s.r.T("root.empty", "root", root))
}

// renderForest prints one tree per workbench beneath the root.
func (s *session) renderForest(forest *verb.Forest) {
	if len(forest.Workbenches) == 0 {
		s.rootEmpty(forest.Root)
		return
	}
	for i, member := range forest.Workbenches {
		if i > 0 {
			s.line("")
		}
		if s.rootHeading(member.Candidate) {
			continue
		}
		s.renderTree(member.Tree)
	}
}

// renderRootStatus prints where each workbench beneath the root stands.
func (s *session) renderRootStatus(answer *verb.RootStatus) {
	if len(answer.Workbenches) == 0 {
		s.rootEmpty(answer.Root)
		return
	}
	for i, member := range answer.Workbenches {
		if i > 0 {
			s.line("")
		}
		if s.rootHeading(member.Candidate) {
			continue
		}
		s.renderStatus(member.Status)
	}
}

// renderRootListing prints each workbench's own listing beneath the root.
func (s *session) renderRootListing(answer *verb.RootListing) {
	if len(answer.Workbenches) == 0 {
		s.rootEmpty(answer.Root)
		return
	}
	for i, member := range answer.Workbenches {
		if i > 0 {
			s.line("")
		}
		if s.rootHeading(member.Candidate) {
			continue
		}
		s.renderListing(member.Listing)
	}
}

// renderRootOffers prints what each workbench beneath the root offers next.
func (s *session) renderRootOffers(answer *verb.RootOffers) {
	if len(answer.Workbenches) == 0 {
		s.rootEmpty(answer.Root)
		return
	}
	for i, member := range answer.Workbenches {
		if i > 0 {
			s.line("")
		}
		if s.rootHeading(member.Candidate) {
			continue
		}
		s.renderOffers(member.Offers)
	}
}

// renderRootChanges prints what each workbench beneath the root answered, and
// then the one merged cursor to hand back at the next checkpoint.
//
// The cursor is printed once, at the end, rather than per workbench, because
// one merged token is what the next call takes: a reader handed twenty-five
// member tokens would have to know how they are assembled, which is exactly
// what the merged token exists to keep them from having to know.
//
// A minting call reports no events for any workbench by contract, so its rows
// are headings alone under the sentence that says so.
func (s *session) renderRootChanges(answer *verb.RootChangeSet) {
	if len(answer.Workbenches) == 0 {
		s.rootEmpty(answer.Root)
		s.line(s.r.T("changes.cursor", "cursor", answer.Cursor))
		return
	}
	for i, member := range answer.Workbenches {
		if i > 0 {
			s.line("")
		}
		if s.rootHeading(member.Candidate) {
			continue
		}
		if member.Changes == nil {
			continue
		}
		s.renderChangesBody(member.Changes)
	}
	s.line("")
	s.line(s.r.T("changes.cursor", "cursor", answer.Cursor))
}
