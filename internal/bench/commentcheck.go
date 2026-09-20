package bench

import (
	"path/filepath"
)

// The findings dinah-525 added, which are about comment bodies rather than
// about the structure every other finding names.
const (
	// FindingCommentBodyDiverged names a comment whose body no longer
	// matches the digest the tool last recorded over it, which means it was
	// edited by something other than a verb. The finding says nothing about
	// who edited it or when, because it knows neither, and it blocks
	// nothing: a hand edit is a fact about the record rather than a reason
	// to stop work.
	//
	// A comment carrying no digest is not reported. Every comment written
	// before dinah-525 is in that state, and one gains a digest the first
	// time a verb writes it, so absence is silence rather than an
	// accusation.
	FindingCommentBodyDiverged = "check.comment-body-diverged"
	// FindingEmptyComment names a comment whose body is empty and which
	// nothing designates. It is the residue of an abandoned draft, which is
	// what the empty creation form costs, and it is clutter rather than
	// damage: cleanup severity, nothing blocked, and dinah delete named as
	// the remedy.
	FindingEmptyComment = "check.empty-comment"
	// FindingEmptyDesignatedComment names a comment whose body is empty and
	// which a checklist item designates as its answer. That one is a defect
	// rather than clutter, because an item's answer of record cannot be
	// nothing.
	FindingEmptyDesignatedComment = "check.empty-designated-comment"
	// FindingItemCarriesRetiredNote names a checklist item still carrying
	// the note key dinah-525 retired, on a workbench whose declared format
	// says the note migration has run. It is the residue of an interrupted
	// run, which the migration itself is idempotent over: running it again
	// finishes the item.
	FindingItemCarriesRetiredNote = "check.item-carries-retired-note"
	// FindingDanglingResolution names a checklist item whose resolution
	// names a comment that no longer resolves. A delete cannot produce it,
	// since deleting a designated comment is refused and the forced form
	// clears the designation, so this is the residue of a hand edit or of a
	// removal outside the tool.
	FindingDanglingResolution = "check.dangling-resolution"
)

// The two severities a finding carries. The set is closed, and an empty
// severity reads as SeverityDefect, which is what every finding written before
// dinah-525 carries and what every structural invariant means.
const (
	// SeverityDefect is something wrong with the store: an invariant the
	// format states is not holding.
	SeverityDefect = "defect"
	// SeverityCleanup is something the store would be tidier without, which
	// nothing depends on and nothing is blocked by. It is reported so an
	// operator can decide, not so the tool can.
	SeverityCleanup = "cleanup"
)

// SeverityOf reports the severity a finding carries, which is the value it
// declares or SeverityDefect where it declares none. Reading it through one
// function rather than off the field is what lets every finding written before
// the severities existed keep meaning what it meant.
func SeverityOf(finding Finding) string {
	if finding.Severity == "" {
		return SeverityDefect
	}
	return finding.Severity
}

// checkComments applies the body invariants to every comment below one card:
// the card's own comments and the comments of each of its checklist items.
//
// The walk is the card's own directory rather than a call per comment, on the
// terms the rest of checkCard already reads the card it was given. Each
// comment's anchor is opened once and answers both questions asked of it,
// whether its body still matches its digest and whether its body is empty.
//
// Which item designates which comment is read from the items rather than from
// the comments, because a designation names a comment of the item that carries
// it, so one pass over the checklist names every designated comment on the
// card.
func (b *Bench) checkComments(card *Card) ([]Finding, error) {
	items, err := Items(card.Dir)
	if err != nil {
		return nil, err
	}
	designated := map[string]bool{}
	var findings []Finding
	for _, item := range items {
		if item.Resolution == "" {
			continue
		}
		dir, found := b.commentDirOf(item, item.Resolution)
		if !found {
			findings = append(findings, Finding{
				Path:   filepath.Join(item.Dir, ItemAnchor),
				Key:    FindingDanglingResolution,
				Detail: item.Resolution,
			})
			continue
		}
		designated[filepath.Clean(dir)] = true
	}
	holders := []string{card.Dir}
	for _, item := range items {
		holders = append(holders, item.Dir)
	}
	for _, holder := range holders {
		comments, err := Comments(holder)
		if err != nil {
			continue
		}
		for _, comment := range comments {
			findings = append(findings, commentBodyFindings(comment, designated[filepath.Clean(comment.Dir)])...)
		}
	}
	return findings, nil
}

// commentBodyFindings judges one comment's body, given whether an item
// designates it.
//
// The two questions are independent, and a comment can answer both: a
// diverged body that is now empty is two facts about one file, and reporting
// one of them would leave an operator repairing half of what is wrong.
func commentBodyFindings(comment *Comment, designated bool) []Finding {
	path := filepath.Join(comment.Dir, CommentAnchor)
	var findings []Finding
	if comment.Digest != "" && comment.Digest != CommentDigest(comment.Body) {
		findings = append(findings, Finding{
			Path:     path,
			Key:      FindingCommentBodyDiverged,
			Detail:   comment.ID,
			Severity: SeverityDefect,
		})
	}
	if comment.Body != "" {
		return findings
	}
	if designated {
		return append(findings, Finding{
			Path:     path,
			Key:      FindingEmptyDesignatedComment,
			Detail:   comment.ID,
			Severity: SeverityDefect,
		})
	}
	return append(findings, Finding{
		Path:     path,
		Key:      FindingEmptyComment,
		Detail:   comment.ID,
		Severity: SeverityCleanup,
	})
}

// commentDirOf resolves one item's resolution to the directory of the comment
// it names, and reports whether it named one at all.
//
// It resolves against the item's own comments rather than through the general
// resolver, which is what keeps the check independent of the reference grammar
// the write side used: a designation names a comment of the item that carries
// it, so the ordinal at the end of the reference is a position in this one
// collection and the rest of the reference is the item the check already has.
func (b *Bench) commentDirOf(item *Item, resolution string) (string, bool) {
	ordinal, ok := trailingOrdinal(resolution)
	if !ok {
		return "", false
	}
	comments, err := Comments(item.Dir)
	if err != nil {
		return "", false
	}
	for _, comment := range comments {
		if comment.Ordinal == ordinal {
			return comment.Dir, true
		}
	}
	return "", false
}

// trailingOrdinal reads the number at the end of a reference, which on a
// designation is the comment's position among the item's own comments.
func trailingOrdinal(ref string) (int, bool) {
	last := ref
	if i := lastSlash(ref); i >= 0 {
		last = ref[i+1:]
	}
	n := 0
	if last == "" {
		return 0, false
	}
	for _, r := range last {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// lastSlash reports the index of the last forward slash in a reference, or -1
// where it carries none. References are written with forward slashes whatever
// the platform's own separator is, so this asks about the reference grammar
// rather than about a path.
func lastSlash(ref string) int {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '/' {
			return i
		}
	}
	return -1
}

// checkRetiredNotes reports every checklist item still carrying the note key
// dinah-525 retired, on a workbench whose declared format says the migration
// has run.
//
// A workbench below that format is not reported at all, and it is not reported
// because it is refused: a read cannot open such a store, so nothing reaches
// this walk. What this finding covers is the store the migration ran over and
// did not finish, which is the state a run interrupted between its two writes
// leaves one item in.
func (b *Bench) checkRetiredNotes(card *Card) ([]Finding, error) {
	if b.Format < ResolutionFormat {
		return nil, nil
	}
	collection := filepath.Join(card.Dir, ChecklistDir)
	ids, err := ListIDs(collection)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, id := range ids {
		dir := filepath.Join(collection, id)
		fm, _, err := ReadItemAnchor(dir)
		if err != nil {
			continue
		}
		if fm.Value(ItemNoteRetiredField) == "" {
			continue
		}
		findings = append(findings, Finding{
			Path:     filepath.Join(dir, ItemAnchor),
			Key:      FindingItemCarriesRetiredNote,
			Detail:   id,
			Severity: SeverityDefect,
		})
	}
	return findings, nil
}
