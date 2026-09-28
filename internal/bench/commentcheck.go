package bench

import (
	"strconv"
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
	// FindingDesignationMissing names a checklist item standing in a settled
	// state and carrying no answer of record. The designation conversion
	// leaves such an item behind wherever the card's history could not say
	// which comment the answer meant, and the operator ruled that it is left
	// unanswered rather than having somebody's best guess written down.
	//
	// It is reported for as long as the set exists, because a run's report
	// scrolls away and the set is what a person has to go back and answer.
	// Re-answering is the ordinary route with its ordinary authority: reopen
	// the item and settle it again with a designated comment.
	FindingDesignationMissing = "check.designation-missing"
	// FindingMemberUnknown names a line of a card-unit journal that names a
	// comment or an item the replay never established. The line contributes
	// nothing to any read, and the finding is a defect because such a line
	// is either damage or the residue of a hand edit that removed the line
	// creating the member. Detail is the journal and the one-based line.
	FindingMemberUnknown = "check.member-unknown"
	// FindingMemberIDCollision names an identifier two members of one card
	// carry, which the card's journal states and no read can tell apart, so
	// every read of the card is refused dinah.journal-unreadable until the
	// journal is repaired by hand. Detail is the identifier, a colon, and
	// the one-based numbers of the two lines that established a member
	// under it.
	FindingMemberIDCollision = "check.member-id-collision"
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

// checkComments applies the body invariants to every live comment below one
// card: the card's own comments and the comments of each of its live items.
//
// Each comment is read once from the card's record and answers both
// questions asked of it, whether its body still matches its recorded digest
// and whether its body is empty. Which item designates which comment is read
// from the items rather than from the comments, because a designation names a
// comment of the item that carries it, so one pass over the checklist names
// every designated comment on the card.
func (b *Bench) checkComments(card *Card, record *CardRecord) []Finding {
	items := record.ItemsIn(LiveHalf, "")
	designated := map[string]bool{}
	var findings []Finding
	for _, item := range items {
		if item.Resolution == "" {
			continue
		}
		if _, found := record.Designated(item); !found {
			findings = append(findings, Finding{
				Path:   record.FileOf(KindItem, item.ID),
				Key:    FindingDanglingResolution,
				Detail: item.Resolution,
			})
			continue
		}
		designated[item.Resolution] = true
	}
	// Each holder is carried with the reference its comments compose under,
	// because a finding names what a reader can type. The identifier alone
	// resolves to nothing: `dinah show f2fc8866dfda` answers unknown-card,
	// so a finding carrying one told a reader which file was wrong in a
	// spelling they could not use to open it, while the verb refusal over
	// the same comment printed the reference correctly and the two surfaces
	// disagreed about how to name one thing.
	type holder struct {
		id  string
		ref string
	}
	cardRef := card.Ref(b.Slug)
	holders := []holder{{ref: cardRef}}
	seen := map[string]int{}
	for _, item := range items {
		seen[item.Kind]++
		itemRef := cardRef + "/" + ChecklistSegment + "/" + strconv.Itoa(seen[item.Kind])
		if word, ok := WordForItemKind(item.Kind); ok {
			itemRef = cardRef + "/" + word + "/" + strconv.Itoa(seen[item.Kind])
		}
		holders = append(holders, holder{id: item.ID, ref: itemRef})
	}
	for _, held := range holders {
		for _, comment := range record.CommentsOf(held.id, LiveHalf) {
			reference := held.ref + "/" + CommentsDir + "/" + strconv.Itoa(comment.Ordinal)
			path := record.FileOf(KindComment, comment.ID)
			findings = append(findings, commentBodyFindings(comment, path, reference, designated[comment.ID])...)
		}
	}
	return findings
}

// commentBodyFindings judges one comment's body, given whether an item
// designates it.
//
// The two questions are independent, and a comment can answer both: a
// diverged body that is now empty is two facts about one file, and reporting
// one of them would leave an operator repairing half of what is wrong.
func commentBodyFindings(comment *Comment, path, reference string, designated bool) []Finding {
	var findings []Finding
	if comment.RecordedDigest != "" && comment.RecordedDigest != CommentDigest(comment.Body) {
		findings = append(findings, Finding{
			Path:     path,
			Key:      FindingCommentBodyDiverged,
			Detail:   reference,
			Severity: SeverityDefect,
		})
	}
	if comment.Body != "" || comment.Redacted != nil {
		return findings
	}
	if designated {
		return append(findings, Finding{
			Path:     path,
			Key:      FindingEmptyDesignatedComment,
			Detail:   reference,
			Severity: SeverityDefect,
		})
	}
	return append(findings, Finding{
		Path:     path,
		Key:      FindingEmptyComment,
		Detail:   reference,
		Severity: SeverityCleanup,
	})
}

// Designated answers the comment an item designates as its answer of record,
// in either half of that item's comments, and reports whether it names one at
// all.
//
// The stored value is the designated comment's own identifier since
// DesignationFormat, so the lookup is a comparison with nothing to resolve
// and nothing to go stale. Both halves are searched, because archiving a
// designated comment stays permitted and the item goes on citing it wherever
// it now lives. The comment has to hang on the item itself, which is what the
// write refuses anything else for.
func (r *CardRecord) Designated(item *Item) (*Comment, bool) {
	if item == nil || item.Resolution == "" {
		return nil, false
	}
	r.holdComments(item.ID)
	comment, ok := r.Comments[item.Resolution]
	if !ok || comment.Holder != item.ID {
		return nil, false
	}
	return comment, true
}

// checkMissingDesignations reports every checklist item of a card standing in
// a settled state and carrying no answer of record.
//
// The population it exists for is the one the designation conversion leaves
// behind. Where a card's history could not say which comment an answer meant,
// the operator ruled that the item is left unanswered rather than having
// somebody's best guess written down as a recorded ruling, and what such an
// item loses is its answer of record. It must not lose that quietly once the
// conversion's report has scrolled away, so the set stays visible on demand
// for as long as it exists.
//
// Both halves of the checklist are walked, because an archived item settled
// without an answer says the same thing a live one says and the conversion
// reaches both.
//
// It is a cleanup rather than a defect. Nothing depends on the key: every hold
// reads an item's state, its kind or its column, and none reads the answer, so
// an item carrying none holds exactly what it held before and releases exactly
// what it released before.
func (b *Bench) checkMissingDesignations(record *CardRecord) []Finding {
	var findings []Finding
	for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
		for _, item := range record.ItemsIn(half, "") {
			if !ItemOwesDesignation(item) {
				continue
			}
			findings = append(findings, Finding{
				Path:     record.FileOf(KindItem, item.ID),
				Key:      FindingDesignationMissing,
				Detail:   item.State,
				Severity: SeverityCleanup,
			})
		}
	}
	return findings
}

// checkReplayedMembers applies the invariants a card-unit journal can break
// and the old layout cannot: a line naming a member the replay never
// established, and two members of one collection carrying one ordinal, which
// two clones commenting from one base produce and git's union merge keeps.
// Below the card-unit format it reports nothing, since the ordinal sweep reads
// the directories there.
func (b *Bench) checkReplayedMembers(card *Card, record *CardRecord) []Finding {
	if !b.CardUnit() {
		return nil
	}
	var findings []Finding
	for _, line := range record.UnknownMemberLines() {
		findings = append(findings, Finding{
			Path:   card.JournalPath(),
			Key:    FindingMemberUnknown,
			Detail: card.JournalPath() + ":" + strconv.Itoa(line),
		})
	}
	seen := map[MemberCollection]map[int]bool{}
	claim := func(collection MemberCollection, ordinal int, id string) {
		if seen[collection] == nil {
			seen[collection] = map[int]bool{}
		}
		if seen[collection][ordinal] {
			findings = append(findings, Finding{Path: card.JournalPath(), Key: FindingOrdinalDuplicate, Detail: id})
			return
		}
		seen[collection][ordinal] = true
	}
	for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
		for _, item := range record.ItemsIn(half, "") {
			claim(MemberCollection{Kind: KindItem}, item.Ordinal, item.ID)
		}
	}
	for _, id := range sortedCommentIDs(record) {
		comment := record.Comments[id]
		claim(MemberCollection{Kind: KindComment, Holder: comment.Holder}, comment.Ordinal, id)
	}
	return findings
}

// sortedCommentIDs answers a record's comment identifiers in ordinal order
// within each holder, so a duplicate is reported against the later member.
func sortedCommentIDs(record *CardRecord) []string {
	record.holdEverything()
	var ids []string
	holders := map[string]bool{"": true}
	for _, comment := range record.Comments {
		holders[comment.Holder] = true
	}
	for holder := range holders {
		for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
			for _, comment := range record.CommentsOf(holder, half) {
				ids = append(ids, comment.ID)
			}
		}
	}
	return ids
}

// trailingOrdinal reads the number at the end of a reference, which on a
// designation written before DesignationFormat is the comment's position
// among the item's own comments. The designation conversion is the one reader
// left, and it reads a store the conversion has not reached yet.
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
