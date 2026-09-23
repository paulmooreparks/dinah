package bench

import (
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// The three ways one item's stored answer leaves this conversion, which the
// report groups by and which a reader compares two runs on.
const (
	// DesignationFromJournal is an item whose settling minted its own
	// comment, so the journal carries that comment's identifier on a
	// commented line immediately before the settling line. The identifier
	// was read rather than inferred.
	DesignationFromJournal = "journal"
	// DesignationUndisturbed is an item no removal has touched since it was
	// settled, so no position can have shifted and resolving the stored
	// reference by position reaches the comment that was meant. The
	// confidence is of a different kind from the route above, which is why
	// the report keeps the two apart.
	DesignationUndisturbed = "undisturbed"
	// DesignationUnrecoverable is an item the history cannot speak for. The
	// conversion does not guess: the resolution key is removed, the item's
	// state is left exactly as it stands, and a person answers it again.
	DesignationUnrecoverable = "unrecoverable"
)

// DesignationEntry is one item the conversion decided, in the report's own
// vocabulary.
type DesignationEntry struct {
	// Card is the card's reference and Item is the item's, both in the
	// spelling a person types.
	Card string `json:"card"`
	Item string `json:"item"`
	// Route is one of the three constants above.
	Route string `json:"route"`
	// Settling is the journal event that last settled the item, which is
	// what tells a reader which verb's answer is at stake.
	Settling string `json:"settling"`
	// State is the state the item stands at, which the conversion never
	// changes and which an unrecoverable item keeps.
	State string `json:"state"`
	// Identifier is the comment identifier the conversion wrote, empty on
	// an unrecoverable item.
	Identifier string `json:"identifier,omitempty"`
	// Author is who wrote the comment named in Identifier, or, on an
	// unrecoverable item, who wrote the comment the stored reference
	// reaches today. That second reading is the point of carrying it: it
	// shows the operator what a guessing conversion would have written
	// down.
	Author string `json:"author,omitempty"`
	// Stored is the positional reference the item carried before the run,
	// carried on an unrecoverable item so the operator can see what was
	// there.
	Stored string `json:"stored,omitempty"`
	// Reaches is the reference the stored value resolves to today, carried
	// on an unrecoverable item beside Author for the same reason.
	Reaches string `json:"reaches,omitempty"`
	// Archived reports whether this item's comments collection holds at
	// least one archived member, which is the population whose positions
	// may already have drifted before anybody ran this.
	Archived bool `json:"archived,omitempty"`
}

// DesignationMigration is what one run answers.
type DesignationMigration struct {
	// Entries are every item the run decided, in card order and then in
	// checklist order, whichever route each took.
	Entries []DesignationEntry `json:"entries"`
	// PassedClaims are the cards whose claim the run passed under the
	// operator's force, each with the owner who held it, and empty on every
	// other run. The holder travels with the reference because the operator
	// judging a claim dead is judging a session dead, and the report is where
	// he sees whose.
	PassedClaims []ClaimedCard `json:"passed_claims,omitempty"`
	// Forced reports that the run carried the operator's force past the
	// claimed-card refusal. It is carried beside PassedClaims rather than
	// inferred from it, because a forced run on a workbench where nothing
	// was claimed passes no claim and still has to say so: a flag that
	// prints nothing when it changed nothing is a flag nobody can tell they
	// typed.
	Forced bool `json:"forced,omitempty"`
	// Applied is false on a rehearsal, which decides everything by the same
	// rules and writes nothing.
	Applied bool `json:"applied"`
	// Stamped reports whether the run wrote the new format number onto the
	// workbench anchor.
	Stamped bool `json:"stamped,omitempty"`
}

// Count reports how many entries took one route, which is the number the
// report prints beside each group's heading.
func (m *DesignationMigration) Count(route string) int {
	if m == nil {
		return 0
	}
	n := 0
	for _, entry := range m.Entries {
		if entry.Route == route {
			n++
		}
	}
	return n
}

// Clean reports whether the run left anything a person has to act on that
// nothing else reports, which is what decides whether a check that ran this
// conversion still exits clean.
//
// An unrecoverable item is exactly that, and it is the number the operator
// acts on. It is reported by check.designation-missing afterwards as well, so
// this answers the run rather than the store.
func (m *DesignationMigration) Clean() bool {
	return m == nil || m.Count(DesignationUnrecoverable) == 0
}

// ClaimedCard is the first live card a conversion found claimed, which is what
// the workbench-in-use refusal names.
type ClaimedCard struct {
	Ref    string `json:"ref"`
	Holder string `json:"holder"`
}

// ClaimedCards reports every live card standing claimed, in card order. The
// conversion reads it to decide whether it may run at all.
//
// It reads live cards alone. An archived card carries no claim anybody can be
// working under, since taking a card up requires resolving it in the live
// half, so asking about archived claims would refuse the conversion over a
// state nobody can be in.
func (b *Bench) ClaimedCards() ([]ClaimedCard, error) {
	cards, err := b.Cards()
	if err != nil {
		return nil, err
	}
	var claimed []ClaimedCard
	for _, card := range cards {
		if card.Holder == "" {
			continue
		}
		claimed = append(claimed, ClaimedCard{Ref: card.Ref(b.Slug), Holder: card.Holder})
	}
	return claimed, nil
}

// MigrateDesignations converts every stored answer from a positional comment
// reference to the designated comment's own identifier, and stamps the
// workbench at DesignationFormat once it has.
//
// It walks both halves. A live-only walk would leave a positional designation
// sitting in a store that declares the new format, where the deletion guard
// compares the stored value against a directory name with no resolution step,
// so restoring the card later would make that guard fail open. That is the
// exact failure the format number was moved to prevent.
//
// It decides from each card's own journal rather than from the item's live
// comments collection, and that is the method rather than a fallback. A
// comment standing before the designated one can be deleted outright, which
// leaves the item's directory carrying no archive at all while the stored
// position now reaches somebody else's words; a detector that looked for
// archived comments would convert such an item silently and write the wrong
// author down as the operator's answer.
//
// Two routes recover an answer and everything else is left unanswered.
// decideDesignation carries each one's own reasoning, and both are written to
// decline rather than to guess: the point of reading the journal is that the
// conversion writes down what somebody recorded, so a case the record cannot
// speak for goes to a person rather than to an inference.
//
// apply is false on a rehearsal, which decides every item by these same rules,
// answers the identical report, and writes no anchor, no journal line and no
// format stamp.
func (b *Bench) MigrateDesignations(actor Actor, now string, apply bool) (*DesignationMigration, error) {
	run := &DesignationMigration{Applied: apply}
	roots := []string{b.CardsRoot(), b.ArchivedCardsRoot()}
	for _, root := range roots {
		ids, err := ListIDs(root)
		if err != nil {
			return run, err
		}
		for _, id := range ids {
			card, err := b.LoadCardIn(root, id)
			if err != nil {
				// A card the walk cannot open is dinah check's own
				// finding, reported by name there. Failing the whole
				// conversion over one damaged anchor would leave the
				// store half converted with no way to finish it.
				continue
			}
			if err := b.convertCard(run, card, actor, now, apply); err != nil {
				return run, err
			}
		}
	}
	if !apply {
		return run, nil
	}
	if err := b.stampFormat(DesignationFormat); err != nil {
		return run, err
	}
	run.Stamped = true
	return run, nil
}

// convertCard decides and writes every item of one card, under that card's own
// lock, which is the lock an ordinary item write takes. A concurrent restore of
// an archived card is serialised against the conversion rather than racing it.
func (b *Bench) convertCard(run *DesignationMigration, card *Card, actor Actor, now string, apply bool) error {
	pending, err := b.plannedDesignations(card)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	run.Entries = append(run.Entries, pending...)
	if !apply {
		return nil
	}
	lock, err := Acquire(card.Dir, actor.Name, now)
	if err != nil {
		return err
	}
	defer lock.Release()
	for _, entry := range pending {
		dir, found := b.itemDirOf(card, entry.Item)
		if !found {
			continue
		}
		fm, body, err := ReadItemAnchor(dir)
		if err != nil {
			return err
		}
		if entry.Identifier == "" {
			fm.Delete(ItemResolutionField)
		} else {
			fm.Set(ItemResolutionField, entry.Identifier)
		}
		if err := WriteItemAnchor(dir, fm, body); err != nil {
			return err
		}
	}
	return nil
}

// plannedDesignations decides every item of one card that still carries a
// positional answer, and answers nothing at all for a card already converted.
//
// An item whose stored value is already a bare identifier is passed over
// silently, which is what makes a second converting run write nothing, report
// nothing and leave the stamp where it is.
func (b *Bench) plannedDesignations(card *Card) ([]DesignationEntry, error) {
	items, err := b.everyItem(card)
	if err != nil {
		return nil, err
	}
	var planned []DesignationEntry
	var events []Event
	loaded := false
	cardRef := card.Ref(b.Slug)
	for _, item := range items {
		if item.Resolution == "" || IsID(item.Resolution) {
			continue
		}
		if !loaded {
			events, _, err = ReadJournal(card.JournalPath())
			if err != nil {
				return nil, err
			}
			loaded = true
		}
		planned = append(planned, b.decideDesignation(card, cardRef, item, events))
	}
	return planned, nil
}

// everyItem reads a card's checklist items from both halves, live first and
// archived after, so an archived item is converted exactly as a live one is.
func (b *Bench) everyItem(card *Card) ([]*Item, error) {
	items, err := Items(card.Dir)
	if err != nil {
		return nil, err
	}
	archived, err := Items(filepath.Join(card.Dir, ArchiveDir))
	if err != nil {
		return items, nil
	}
	return append(items, archived...), nil
}

// itemDirOf finds the directory of one item of one card by the reference the
// entry carries, looking in both halves for everyItem's reason.
func (b *Bench) itemDirOf(card *Card, ref string) (string, bool) {
	items, err := b.everyItem(card)
	if err != nil {
		return "", false
	}
	for _, item := range items {
		if b.itemRefOf(card, item) == ref {
			return item.Dir, true
		}
	}
	return "", false
}

// itemRefOf composes the reference the report names an item by, which is the
// item's own directory name under the card's reference. The conversion names
// items this way rather than by kind position, because an archived item has no
// position among the live ones and a report that renumbered them would name
// something a reader cannot type.
func (b *Bench) itemRefOf(card *Card, item *Item) string {
	return card.Ref(b.Slug) + "/checklist/" + item.ID
}

// decideDesignation answers one item, taking whichever of the two recovery
// routes the journal supports and leaving the item unanswered where neither
// does.
func (b *Bench) decideDesignation(card *Card, cardRef string, item *Item, events []Event) DesignationEntry {
	entry := DesignationEntry{
		Card:   cardRef,
		Item:   b.itemRefOf(card, item),
		Route:  DesignationUnrecoverable,
		State:  item.State,
		Stored: item.Resolution,
	}
	comments := b.commentsOfItem(item)
	for _, comment := range comments {
		if comment.archived {
			entry.Archived = true
			break
		}
	}
	at := lastSettling(events, item.ID)
	if at >= 0 {
		entry.Settling = events[at].Event
	}
	// What the stored reference reaches today, read before any decision, so
	// an unrecoverable entry can show the operator what a guessing
	// conversion would have written down.
	if reached, found := b.commentAtPosition(item, item.Resolution); found {
		entry.Reaches = entry.Stored
		entry.Author = reached.Author
	}
	if at < 0 {
		return entry
	}
	// Route 1, the minted-comment route. The --text form mints a comment and
	// designates it in one act, and it journals the commented line
	// immediately before the settling line carrying that comment's own
	// identifier. That identifier was recorded rather than inferred, so it
	// answers whatever has happened to the collection since.
	//
	// What selects that one act is the stamp as well as the adjacency, and
	// the stamp is what makes the route safe rather than merely likely. One
	// write stamps both lines from one reading of the clock, so the two
	// carry the same instant exactly when they are halves of one act.
	// Without that test the route also matches an ordinary comment written
	// just before a settling that named some other comment of the same item,
	// and it would then write the wrong comment's identifier down as the
	// answer, which is the harm this whole conversion exists to avoid.
	if at > 0 {
		prior := events[at-1]
		if prior.Event == contract.EventCommented && prior.Item == item.ID &&
			prior.Comment != "" && prior.TS == events[at].TS && sameActor(prior.Actor, events[at].Actor) {
			if comment, found := comments[prior.Comment]; found {
				entry.Route = DesignationFromJournal
				entry.Identifier = prior.Comment
				entry.Author = comment.Author
				return entry
			}
			// The journal names a comment that is no longer anywhere on
			// the card, which is a deletion outright. There is nothing to
			// point the answer at, so the item is left unanswered rather
			// than carrying an identifier that resolves to nothing.
			return entry
		}
	}
	// Route 2, the undisturbed route. Where nothing has been archived,
	// restored or deleted that could be a comment of this item since the
	// settling, no position can have shifted, so the stored reference still
	// reaches the comment that was meant.
	if b.disturbedSince(card, item, events, at) {
		return entry
	}
	reached, found := b.commentAtPosition(item, item.Resolution)
	if !found {
		return entry
	}
	entry.Route = DesignationUndisturbed
	entry.Identifier = reached.ID
	entry.Author = reached.Author
	entry.Reaches = ""
	return entry
}

// disturbedSince reports whether any act since the settling could have moved
// the positions of this item's comments.
//
// A removal event carries the identifier it removed and nothing saying which
// holder that identifier hung below, so attribution is done by looking the
// identifier up. One that names a comment of this item disturbs it. One that
// names a comment of some other holder on this card does not, which is what
// keeps an unrelated tidy from making every item on the card unrecoverable.
// One that names nothing anywhere is a deletion outright, and the journal
// cannot say whose comment it was, so the item is treated as disturbed. That
// is the conservative direction and it is the case the whole method exists
// for: a deleted comment leaves the item's directory carrying no archive at
// all while the stored position now reaches somebody else's words.
func (b *Bench) disturbedSince(card *Card, item *Item, events []Event, at int) bool {
	mine := b.commentsOfItem(item)
	elsewhere := b.commentsElsewhere(card, item)
	for _, ev := range events[at+1:] {
		switch ev.Event {
		case contract.EventArchived, contract.EventRestored, contract.EventDeleted:
		default:
			continue
		}
		id := strings.TrimSpace(ev.Note)
		if !IsID(id) {
			continue
		}
		if _, found := mine[id]; found {
			return true
		}
		if elsewhere[id] {
			continue
		}
		return true
	}
	return false
}

// designatedComment is one comment of one item as the conversion reads it: its
// identifier, its author, its position and which half it sits in.
type designatedComment struct {
	ID       string
	Author   string
	Ordinal  int
	archived bool
}

// commentsOfItem reads an item's comments from both halves, keyed by
// identifier, because the conversion asks whether an identifier belongs to
// this item rather than where it sits.
func (b *Bench) commentsOfItem(item *Item) map[string]designatedComment {
	found := map[string]designatedComment{}
	for _, half := range []struct {
		dir      string
		archived bool
	}{{item.Dir, false}, {filepath.Join(item.Dir, ArchiveDir), true}} {
		comments, err := Comments(half.dir)
		if err != nil {
			continue
		}
		for position, comment := range comments {
			found[comment.ID] = designatedComment{
				ID:       comment.ID,
				Author:   comment.Author,
				Ordinal:  position + 1,
				archived: half.archived,
			}
		}
	}
	return found
}

// commentsElsewhere is every comment identifier on this card that does not
// hang below the item being decided: the card's own comments and the comments
// of every other item, in both halves.
func (b *Bench) commentsElsewhere(card *Card, item *Item) map[string]bool {
	found := map[string]bool{}
	holders := []string{card.Dir, filepath.Join(card.Dir, ArchiveDir)}
	items, err := b.everyItem(card)
	if err == nil {
		for _, other := range items {
			if other.ID == item.ID {
				continue
			}
			holders = append(holders, other.Dir, filepath.Join(other.Dir, ArchiveDir))
		}
	}
	for _, holder := range holders {
		comments, err := Comments(holder)
		if err != nil {
			continue
		}
		for _, comment := range comments {
			found[comment.ID] = true
		}
	}
	return found
}

// commentAtPosition resolves a stored positional reference the way the build
// that wrote it resolved one: by the trailing ordinal, over the item's live
// comments in creation order.
func (b *Bench) commentAtPosition(item *Item, resolution string) (designatedComment, bool) {
	ordinal, ok := trailingOrdinal(resolution)
	if !ok {
		return designatedComment{}, false
	}
	comments, err := Comments(item.Dir)
	if err != nil || ordinal < 1 || ordinal > len(comments) {
		return designatedComment{}, false
	}
	comment := comments[ordinal-1]
	return designatedComment{ID: comment.ID, Author: comment.Author, Ordinal: ordinal}, true
}

// lastSettling finds the index of the final settling event for one item, and
// answers minus one where the journal carries none.
//
// The last rather than the first is what an answer recorded, reopened and
// recorded again needs: the designation on disk is the one the most recent
// settling wrote.
//
// It answers an index rather than the event itself so that the miss needs no
// zero Event to return. A zero event here would be one more bench.Event
// literal naming no actor, which the guard over every construction site in
// this package reads as a line somebody forgot to attribute.
func lastSettling(events []Event, itemID string) int {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Item != itemID {
			continue
		}
		switch events[i].Event {
		case contract.EventItemResolved, contract.EventItemVerified, contract.EventItemFailed,
			contract.EventItemWaived, contract.EventItemWithdrawn:
			return i
		}
	}
	return -1
}

// sameActor reports whether two journal actors are the same owner. Route 1
// compares the commented line's actor against the settling line's, because the
// one-command form is one person doing both in one act, and a comment somebody
// else happened to write in the same instant is not that person's answer.
func sameActor(a, b Actor) bool {
	return a.Name != "" && a.Name == b.Name
}

// stampFormat writes the storage format number onto the workbench anchor,
// which is the last thing a converting run does: the number says the
// conversion has happened, so writing it before the items were converted would
// lock every older build out of a store still carrying positional answers.
func (b *Bench) stampFormat(format int) error {
	anchor := filepath.Join(b.Root, WorkbenchAnchor)
	text, err := ReadText(anchor)
	if err != nil {
		return err
	}
	fm, body := ParseAnchor(text)
	fm.Set("format", strconv.Itoa(format))
	if err := WriteText(anchor, fm.Render(body)); err != nil {
		return err
	}
	b.Format = format
	return nil
}

// SettledStates are the five states an item carries an answer of record in,
// which is every state but pending. check.designation-missing reads it, and so
// does anything else asking whether an item owes an answer.
var SettledStates = []string{ItemResolved, ItemVerified, ItemFailed, ItemWaived, ItemWithdrawn}

// ItemOwesDesignation reports whether an item stands in a state that asserts
// somebody decided something while carrying no record of who or why.
//
// A state outside the six the format declares answers false. Such an item
// asserts nothing this tool can read, so demanding an answer of record for it
// would report a second defect over the first, and an unrecognised state is
// check.unknown-state's own finding.
func ItemOwesDesignation(item *Item) bool {
	if item.Resolution != "" {
		return false
	}
	for _, settled := range SettledStates {
		if item.State == settled {
			return true
		}
	}
	return false
}

// DesignationRefusalWorkbenchInUse composes the refusal a conversion raises
// over a claimed card, naming the card and carrying its holder.
func DesignationRefusalWorkbenchInUse(claimed ClaimedCard) error {
	return contract.RefuseWith(contract.WorkbenchInUse, claimed.Ref, map[string]string{
		"owner": claimed.Holder,
	})
}
