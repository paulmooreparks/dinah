package bench

import (
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// BranchHeading is the literal heading a card body carried the branch under
// before this field existed, matched on a line of its own once trailing space
// is trimmed. Nothing composes it out of parts, because the convention was a
// literal string in five column bodies and in every brief written against
// them.
const BranchHeading = "## Branch"

// BranchFieldKey is the declared field the migration lifts that heading into,
// and the key it declares on a workbench whose cards carried one.
const BranchFieldKey = "git.branch"

// BranchFieldMeaning is the one line of prose the declaration the migration
// writes carries, so a reader of the workbench meets the same sentence
// whoever declared the key.
const BranchFieldMeaning = "the branch the card's code lives on"

// The three conditions that make a card a conflict. Each is a token rather
// than a sentence, so the report names the condition and the catalog carries
// the words.
const (
	// BranchConflictAnchorDiffers is a card whose anchor already carries a
	// value for the key that differs from the value in its body.
	BranchConflictAnchorDiffers = "anchor-differs"
	// BranchConflictTwoHeadings is a card whose body carries more than one
	// heading.
	BranchConflictTwoHeadings = "two-headings"
	// BranchConflictUnreadable is a card whose anchor will not open.
	BranchConflictUnreadable = "unreadable"
)

// BranchConflict is one card the classification pass refused, with the
// condition it met. Every conflict the pass found is reported, rather than the
// first, because an operator repairing them one run at a time is the failure
// mode a first-only report creates.
type BranchConflict struct {
	// Card is the card's identifier.
	Card string `json:"card"`
	// Condition is one of the three tokens above.
	Condition string `json:"condition"`
}

// BranchLift is one card the run carried across, with the value it lifted.
type BranchLift struct {
	// Card is the card's identifier.
	Card string `json:"card"`
	// Value is the branch name the body carried, which is what the anchor
	// now holds under BranchFieldKey.
	Value string `json:"value"`
}

// BranchMigration is what one `dinah check --migrate-branches` run answers:
// what it classified, what it wrote, and what stopped it.
type BranchMigration struct {
	// Preview says the run wrote nothing because it carried no confirmation,
	// so Lifted and Emptied name what a confirmed run would rewrite.
	Preview bool `json:"preview,omitempty"`
	// Lifted are the cards whose heading became a value.
	Lifted []BranchLift `json:"lifted,omitempty"`
	// Emptied are the cards that carried a heading with no value beneath it,
	// which lose the heading and gain no key. They are named rather than
	// counted, so an operator sees which cards lost a heading instead of
	// gaining a value.
	Emptied []string `json:"emptied,omitempty"`
	// Untouched counts the cards carrying no heading at all, so a run that
	// classified nothing cannot be read as a run that found nothing wrong.
	Untouched int `json:"untouched"`
	// Conflicts are every card the classification refused. One of them stops
	// the run before any write.
	Conflicts []BranchConflict `json:"conflicts,omitempty"`
	// Declared says the run added the declaration for BranchFieldKey. A
	// workbench that already declared the key keeps its own declaration and a
	// workbench in which no card carried a heading gets none, so the flag is
	// what tells those two apart from a run that wrote one.
	Declared bool `json:"declared,omitempty"`
	// Stamped says the run wrote the format number onto the workbench anchor.
	Stamped bool `json:"stamped,omitempty"`
	// Written are the cards the write pass rewrote, by the reference every
	// other member of this report names a card by, in the order it wrote them.
	// A run that finished names every card it wrote; a run that failed
	// part-way names the ones that landed, which is the one case the format
	// cannot make atomic and which re-running is safe over.
	Written []string `json:"written,omitempty"`
}

// CarriesBranchHeading reports whether a body carries the retired heading on a
// line of its own. `dinah check` asks it of every card of a migrated
// workbench, and the classification pass asks it through headingLines below,
// so one rule answers both.
func CarriesBranchHeading(body string) bool {
	return len(headingLines(body)) > 0
}

// headingLines are the indices of every line of a body equal to the heading
// once trailing space is trimmed. Leading space is not trimmed, because an
// indented line is not a heading in the format Markdown reads and lifting one
// would rewrite a body that never carried the convention.
func headingLines(body string) []int {
	var at []int
	for i, line := range SplitLines(body) {
		if strings.TrimRight(line, " \t") == BranchHeading {
			at = append(at, i)
		}
	}
	return at
}

// branchMigrant is one card the classification pass cleared for the write
// pass: where it stands and what a reader calls it. The value and the
// rewritten body are deliberately not carried, because the write pass reads
// the anchor again under the card's own lock and recomputes both from what it
// finds there rather than from a copy read outside that lock.
//
// The reference is carried rather than the identifier, so that one report
// prints one spelling of a card wherever it names one.
type branchMigrant struct {
	dir string
	ref string
}

// MigrateBranches carries every card of one workbench across the retirement of
// the branch heading: the value in the body becomes the declared field's
// value, the heading leaves the body, the workbench declares the key, and the
// anchor is stamped with the format that says the retirement has happened.
//
// The run is all or nothing over one workbench, and it achieves that by
// classifying every card before it writes any card. A walk that wrote as it
// went would leave a workbench half migrated the first time it met a card it
// could not read, and the operator would then be holding a workbench in a
// state no format number describes.
//
// The classification pass writes nothing at all. It reads every live card,
// archived cards excluded on the rule MigrateNumbers keeps, and sorts each
// into one of four classes: a card carrying no heading is untouched, one
// carrying exactly one heading with a value beneath it is a lift, one carrying
// exactly one heading with no value is an empty heading, and one whose anchor
// already carries a different value, whose body carries two headings, or whose
// anchor will not open is a conflict. A single conflict reports every conflict
// found, writes nothing, stamps no format, declares no field, and answers a
// report the caller's exit code carries outward.
//
// A failure part-way through the write pass is the one case this does not make
// atomic, because the format holds no transaction across files and inventing
// one is not this repair's work. The report names the cards it had written
// when it stopped, and re-running is safe: a card already lifted classifies as
// untouched on the second run, since its heading is gone.
//
// apply is what separates a preview from a migration, on the two-phase shape
// --migrate-numbers already runs. A preview classifies exactly as a migration
// does and writes nothing.
func (b *Bench) MigrateBranches(actor, now string, apply bool) (*BranchMigration, error) {
	report := &BranchMigration{Preview: !apply}
	lock, err := b.Acquire(b.Root, actor, now)
	if err != nil {
		return report, err
	}
	defer lock.Release()
	ids, err := b.ListIDs(b.CardsRoot())
	if err != nil {
		return report, err
	}
	var migrants []branchMigrant
	for _, id := range ids {
		card, err := b.LoadCardIn(b.CardsRoot(), id)
		if err != nil {
			report.Conflicts = append(report.Conflicts, BranchConflict{Card: id, Condition: BranchConflictUnreadable})
			continue
		}
		headings := headingLines(card.Body)
		switch {
		case len(headings) == 0:
			report.Untouched++
			continue
		case len(headings) > 1:
			report.Conflicts = append(report.Conflicts, BranchConflict{Card: card.Ref(b.Slug), Condition: BranchConflictTwoHeadings})
			continue
		}
		value, _ := liftBranchHeading(card.Body, headings[0])
		stored := FieldValue(card.FM, BranchFieldKey)
		if stored != "" && stored != value {
			report.Conflicts = append(report.Conflicts, BranchConflict{Card: card.Ref(b.Slug), Condition: BranchConflictAnchorDiffers})
			continue
		}
		if value == "" {
			report.Emptied = append(report.Emptied, card.Ref(b.Slug))
		} else {
			report.Lifted = append(report.Lifted, BranchLift{Card: card.Ref(b.Slug), Value: value})
		}
		migrants = append(migrants, branchMigrant{dir: card.Dir, ref: card.Ref(b.Slug)})
	}
	// A single conflict stops the run here, before any write. The conflicts
	// travel in the report rather than as an error, because a caller that
	// discards the report on the error path would show an operator the word
	// malformed and none of the cards they have to repair, and the report is
	// the whole point of classifying before writing.
	if len(report.Conflicts) > 0 {
		return report, nil
	}
	if !apply {
		return report, nil
	}
	for _, migrant := range migrants {
		wrote, err := b.writeBranchMigrant(migrant, actor, now)
		if err != nil {
			return report, err
		}
		// A card whose heading has gone between the two passes is not
		// recorded, because the account this report gives of what it wrote is
		// what a re-run is decided from, and naming a card it did not touch
		// would put the operator one card out.
		if wrote {
			report.Written = append(report.Written, migrant.ref)
		}
	}
	if len(migrants) > 0 && b.DeclaredFieldOf(BranchFieldKey) == nil {
		declaration := append(b.DeclaredFields(), DeclaredField{
			Key:     BranchFieldKey,
			Type:    FieldTypeString,
			Meaning: BranchFieldMeaning,
			On:      []string{KindCard},
		})
		b.FM.SetRaw(FieldsKey, RenderFieldsBlock(declaration))
		b.declaredFields = declaration
		report.Declared = true
	}
	// At or above, rather than not equal to. The comparison was an equality
	// while FieldsFormat was the newest format there was, and the two stopped
	// being the same thing when later formats arrived: a store already past
	// it would have been stamped back down by a repair that only ever meant
	// to raise it, and since dinah-525 a store stamped down is a store no
	// ordinary read will open. Reading it as a floor says what was always
	// meant.
	if b.Format < FieldsFormat {
		b.FM.Set("format", strconv.Itoa(FieldsFormat))
		report.Stamped = true
	}
	if report.Declared || report.Stamped {
		if err := b.Save(); err != nil {
			return report, err
		}
	}
	// Raised, never lowered, for the reason the comparison above gives. The
	// stamp on disk is already conditional; this line was not, so a store
	// past this format kept its number in the file and carried the lower one
	// in memory, and the next thing to save the workbench for any reason at
	// all would have written that lower number down. The file and the value
	// held over it now say the same thing on every path.
	if b.Format < FieldsFormat {
		b.Format = FieldsFormat
	}
	return report, nil
}

// writeBranchMigrant performs one card's half of the write pass and reports
// whether it wrote: the value lands in the anchor under the declared key, the
// heading leaves the body, and one line is appended to the card's own journal
// so the write is as visible as any other field write.
//
// The anchor is read again under the card's own lock rather than carried from
// the classification pass, on the discipline MigrateNumbers keeps for its own
// strip: the pass stood outside the lock and the body could have changed
// since. A card whose heading has gone in between is already carried across,
// so it answers false and the caller leaves it out of the account.
func (b *Bench) writeBranchMigrant(migrant branchMigrant, actor, now string) (bool, error) {
	held, err := b.Acquire(migrant.dir, actor, now)
	if err != nil {
		return false, err
	}
	defer held.Release()
	anchor := filepath.Join(migrant.dir, CardAnchor)
	text, err := b.ReadText(anchor)
	if err != nil {
		return false, err
	}
	fm, body := ParseAnchor(text)
	headings := headingLines(body)
	if len(headings) != 1 {
		return false, nil
	}
	value, rewritten := liftBranchHeading(body, headings[0])
	was := FieldValue(fm, BranchFieldKey)
	if value != "" {
		SetFieldValue(fm, BranchFieldKey, value)
	}
	if err := WriteText(anchor, fm.Render(rewritten)); err != nil {
		return false, err
	}
	ev := Event{
		TS:    now,
		Event: contract.EventCardUpdated,
		Actor: NamedActor(actor),
		Field: BranchFieldKey,
		From:  was,
		To:    value,
	}
	return true, AppendEvent(filepath.Join(migrant.dir, JournalName), ev)
}

// liftBranchHeading reads the value one heading carries and answers the body
// with that heading, its value line and the blank lines belonging to them
// removed. A heading with no non-blank line before the next heading or the end
// of the body answers the empty value, which is what makes it an empty heading
// rather than a lift.
//
// The normalisation leaves one blank line separating what was above from what
// was below where both exist, and it is chosen so that no body comes out of
// the run carrying a leading blank line, a trailing blank line, or two blank
// lines together at the seam. That settles the three shapes no real card
// carries, which are a heading standing first in a body, one standing last,
// and one whose value line is followed immediately by another heading.
func liftBranchHeading(body string, at int) (string, string) {
	trailing := strings.HasSuffix(body, "\n")
	lines := SplitLines(strings.TrimSuffix(body, "\n"))
	if at >= len(lines) {
		return "", body
	}
	end := len(lines)
	for i := at + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	value := ""
	last := at
	for i := at + 1; i < end; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		value = strings.TrimSpace(lines[i])
		last = i
		break
	}
	// The blank lines immediately after the removed run go with it, so the
	// seam is composed rather than inherited from whichever side had more.
	for last+1 < end && strings.TrimSpace(lines[last+1]) == "" {
		last++
	}
	above := trimBlankTail(lines[:at])
	below := trimBlankHead(lines[last+1:])
	var kept []string
	kept = append(kept, above...)
	if len(above) > 0 && len(below) > 0 {
		kept = append(kept, "")
	}
	kept = append(kept, below...)
	rewritten := strings.Join(kept, "\n")
	if trailing && rewritten != "" {
		rewritten += "\n"
	}
	return value, rewritten
}

// trimBlankTail drops the blank lines standing at the end of a run.
func trimBlankTail(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[:end]
}

// trimBlankHead drops the blank lines standing at the start of a run.
func trimBlankHead(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	return lines[start:]
}

// Clean reports whether the run needs a person, which is the question the
// command's exit code answers. A conflict does: the migration wrote nothing
// and the cards it named have to be repaired before a second run completes.
// Everything else is clean, a preview included, because a preview is what an
// operator runs to read the blast radius before authorising the write.
func (r *BranchMigration) Clean() bool {
	return len(r.Conflicts) == 0
}
