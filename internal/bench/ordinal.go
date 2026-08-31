package bench

import (
	"path/filepath"
	"sort"
	"strconv"

	"dinah/internal/contract"
)

// OrdinalField is the frontmatter key carrying an entity's creation ordinal:
// a one-based integer counting the entity's own arrival within the collection
// instance it was created in.
//
// The ordinal is a sort key, never an address: it orders the collection, and
// a positional reference such as `<card>/comment/2` selects whichever member
// stands second once the collection is sorted into that order, not the member
// whose stored ordinal happens to read 2. The directory listing cannot supply
// that order, because a hex identifier is random and the listing is therefore
// in no order anybody wrote in, and the comment timestamp cannot either,
// because it is wall-clock and two processes writing inside one second record
// the same value.
const OrdinalField = "ordinal"

// OrdinalOf reads the creation ordinal out of an anchor's frontmatter. A
// header carrying no ordinal, an unparseable one, or a value below one reads as
// zero, which is the value check reports as missing and the migration fills in.
//
// This is the one statement of that rule. Every reader of the field goes
// through it rather than repeating the parse, so no second call site can drift
// from what missing means.
func OrdinalOf(fm *Frontmatter) int {
	n, err := strconv.Atoi(fm.Value(OrdinalField))
	if err != nil || n < 1 {
		return 0
	}
	return n
}

// EntityOrdinal reads the creation ordinal of one entity of a collection.
func EntityOrdinal(collection, id, anchor string) int {
	fm, _ := loadAnchor(filepath.Join(collection, id, anchor))
	return OrdinalOf(fm)
}

// nextOrdinal returns the ordinal a new entity of a collection takes: one past
// the greater of the highest ordinal already in use there and the number of
// entities the collection already holds.
//
// Counting the members matters on a collection nobody has migrated yet, where
// every existing entity reads as zero. One past the highest in use would hand
// the new entity ordinal 1, and the migration would then step over that value
// and stamp the older entities 2 and 3, leaving the newest comment permanently
// first with nothing to report. Counting the members instead reserves the range
// the migration is going to need, so a write before the migration and the
// migration itself agree about write order.
//
// The count reads the members carrying an anchor rather than the directories
// ListIDs returns, because the caller has already claimed the new entity's own
// identifier and created its directory by the time this runs, and that
// directory has no anchor yet.
//
// The scan is safe without a mutex of its own because every caller holds the
// lock of the nearest enclosing journal-bearing entity before it adds to the
// collection, so no second writer can be between this scan and the write that
// follows it.
func nextOrdinal(collection, anchor string) int {
	highest, members := 0, 0
	for _, id := range ListIDs(collection) {
		if !Exists(filepath.Join(collection, id, anchor)) {
			continue
		}
		members++
		if n := EntityOrdinal(collection, id, anchor); n > highest {
			highest = n
		}
	}
	if members > highest {
		highest = members
	}
	return highest + 1
}

// SortByOrdinal returns a collection's identifiers in creation order. An
// entity carrying no ordinal sorts ahead of every stamped one, and the
// unstamped entities are ordered among themselves by fallbackRank, which is
// the order check --migrate-ordinals would stamp them in if it ran now.
//
// Ranking the unstamped group that way is what keeps a position naming the
// same entity on both sides of the migration. Ordering them by the directory
// listing instead, which is what this did before, read an unmigrated
// collection in hex-identifier order and the same collection in journal order
// the moment it was stamped, so a reference somebody had written down changed
// what it named while nobody was looking.
func SortByOrdinal(collection, anchor string, ids []string) []string {
	ordered := append([]string(nil), ids...)
	ordinals := make(map[string]int, len(ordered))
	var unstamped []string
	for _, id := range ordered {
		n := EntityOrdinal(collection, id, anchor)
		ordinals[id] = n
		if n == 0 {
			unstamped = append(unstamped, id)
		}
	}
	rank := fallbackRank(collection, unstamped)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if ordinals[a] != ordinals[b] {
			return ordinals[a] < ordinals[b]
		}
		return rank[a] < rank[b]
	})
	return ordered
}

// fallbackRank ranks a collection's unstamped entities the way
// backfillCollection would number them if check --migrate-ordinals ran right
// now: in the order the journal of the nearest enclosing card names their
// creation, with every entity that journal does not name trailing in the
// collection's own listing order.
//
// journalPathFor returning "" (no enclosing card, which covers every
// collection hanging off the workbench itself, off a column and off a
// workstream) and journalOrder naming no checklist item (no event records one
// being written) both fall straight through to that listing-order ranking,
// which is what SortByOrdinal did everywhere before this function existed.
//
// The journal is read only when the collection holds at least one unstamped
// entity, so a collection whose entities all carry an ordinal, which is every
// collection on a workbench somebody has run the migration over, costs this
// nothing beyond the anchor reads SortByOrdinal already made.
func fallbackRank(collection string, unstamped []string) map[string]int {
	rank := make(map[string]int, len(unstamped))
	if len(unstamped) == 0 {
		return rank
	}
	var order []string
	if journalPath := journalPathFor(filepath.Dir(collection)); journalPath != "" {
		if events, _, err := ReadJournal(journalPath); err == nil {
			order = journalOrder(events)
		}
	}
	recovered, _ := orderedByJournal(unstamped, order)
	for i, id := range recovered {
		rank[id] = i
	}
	return rank
}

// journalPathFor names the journal an unstamped entity's creation might be
// recovered from: the journal of the nearest ancestor directory carrying a
// card's own anchor, walking up from a collection's owner directory.
//
// The walk stops the moment it reaches the workbench's own root, so a
// collection hanging off the workbench or off a column, neither of which
// check --migrate-ordinals stamps, costs a couple of stat calls rather than a
// walk toward the filesystem root. An absent journal is not distinguished
// from an unreadable one here, because ReadJournal reads a file that is not
// there as an empty history and an empty history recovers nothing, which is
// the listing-order answer either way.
func journalPathFor(ownerDir string) string {
	dir := ownerDir
	for {
		if Exists(filepath.Join(dir, CardAnchor)) {
			return filepath.Join(dir, JournalName)
		}
		if Exists(filepath.Join(dir, WorkbenchAnchor)) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// stampOrdinal writes an ordinal onto an entity that carries none, preserving
// every other key of the anchor and its body.
func stampOrdinal(collection, id, anchor string, ordinal int) error {
	path := filepath.Join(collection, id, anchor)
	text, err := ReadText(path)
	if err != nil {
		return contract.Refuse(contract.UnknownPath, path)
	}
	fm, body := ParseAnchor(text)
	fm.Set(OrdinalField, strconv.Itoa(ordinal))
	return WriteText(path, fm.Render(body))
}

// journalOrder returns the identifiers whose creation a card's journal
// records, in the order the events stand in the file.
//
// The journal is the format's authority for history, so it is what the
// migration recovers write order from rather than the comment timestamp or the
// directory listing. Comments and attachments are collected into one sequence
// because an attachment event names the attachment and not the collection it
// was added to; the caller narrows the sequence to the collection it is
// stamping.
func journalOrder(events []Event) []string {
	var order []string
	for _, ev := range events {
		switch ev.Event {
		case contract.EventCommented:
			if ev.Comment != "" {
				order = append(order, ev.Comment)
			}
		case contract.EventAttached:
			if ev.Attachment != "" {
				order = append(order, ev.Attachment)
			}
		}
	}
	return order
}

// orderedByJournal returns a collection's identifiers in the order the journal
// records their creation, with every identifier the journal does not account
// for following in listing order, and names that trailing stretch separately.
//
// A hand-created entity, and every entity of a collection whose creation events
// a torn journal lost, is in the trailing stretch. Its write order is not
// recoverable from anything on disk, so listing order is the only answer left,
// and it is no worse there than what a positional reference resolved to before
// ordinals existed. The caller reports the identifiers this returns as guessed.
// This computation could be repeated at any later time from the same journal
// to name the same entities again; the migration reports them once, at the
// run that stamped them, rather than standing as a check finding forever,
// because a bench that keeps hand-created entities is otherwise stuck with a
// finding nobody can clear.
//
// The candidates are an identifier list rather than a collection directory
// because the two callers pass different members of one collection: the
// migration passes every member, and fallbackRank passes only the members
// carrying no ordinal. Both want the same recovery over whatever they pass,
// which is what makes a read agree with the migration that follows it.
func orderedByJournal(candidates []string, order []string) (ordered, guessed []string) {
	present := map[string]bool{}
	for _, id := range candidates {
		present[id] = true
	}
	seen := map[string]bool{}
	for _, id := range order {
		if !present[id] || seen[id] {
			continue
		}
		seen[id] = true
		ordered = append(ordered, id)
	}
	for _, id := range candidates {
		if !seen[id] {
			ordered = append(ordered, id)
			guessed = append(guessed, id)
		}
	}
	return ordered, guessed
}

// backfillCollection stamps an ordinal on every entity of one collection that
// carries none, in the order the journal says they were written, and reports
// each entity it could only place by the directory listing.
//
// An entity already carrying an ordinal keeps it, and the values those
// entities hold are stepped over rather than reused, so a collection stamped
// halfway through an interrupted run finishes without a duplicate and a
// collection stamped in full is left exactly as it stands. That is what makes
// a second run of the migration change nothing, and it is also why an entity
// the journal does not cover is reported only when this run is what stamped
// it: a value already on disk was somebody else's call, not this run's guess.
//
// An entity this run cannot write to (a bare directory an interrupted claim
// left with no anchor yet, or a file the filesystem refuses) is treated the
// way a locked card is: reported and stepped over, not an abort. Its ordinal
// stays taken by neither, so a later run can still fill it once the
// obstruction is cleared, and every other entity of the collection is stamped
// as if it had never been there.
//
// Which entities the filesystem refuses is the filesystem's call and not this
// walk's. The write goes through WriteText, so it is a temporary renamed over
// the anchor, and the platform decides whether that replacement is allowed:
// POSIX asks the containing directory, Windows asks the file. Both answers are
// honoured here as they come back, and neither is second-guessed by a
// permission check of the tool's own.
func (b *Bench) backfillCollection(collection, anchor string, order []string) (int, []Finding) {
	ordered, unrecovered := orderedByJournal(ListIDs(collection), order)
	guessed := map[string]bool{}
	for _, id := range unrecovered {
		guessed[id] = true
	}
	taken := map[int]bool{}
	for _, id := range ordered {
		if n := EntityOrdinal(collection, id, anchor); n > 0 {
			taken[n] = true
		}
	}
	stamped := 0
	next := 1
	var findings []Finding
	for _, id := range ordered {
		if EntityOrdinal(collection, id, anchor) > 0 {
			continue
		}
		for taken[next] {
			next++
		}
		path := filepath.Join(collection, id, anchor)
		if err := b.beforeOrdinalStamp(id); err != nil {
			findings = append(findings, Finding{Path: path, Key: FindingOrdinalUnwritable, Detail: id})
			continue
		}
		if err := stampOrdinal(collection, id, anchor, next); err != nil {
			findings = append(findings, Finding{Path: path, Key: FindingOrdinalUnwritable, Detail: id})
			continue
		}
		if guessed[id] {
			findings = append(findings, Finding{Path: path, Key: FindingOrdinalGuessed, Detail: id})
		}
		taken[next] = true
		stamped++
	}
	return stamped, findings
}

// beforeOrdinalStamp runs the write failure a test asks for at one entity of
// the migration's walk, and is a no-op on every bench nobody is testing.
func (b *Bench) beforeOrdinalStamp(id string) error {
	if b.Hooks == nil || b.Hooks.BeforeOrdinalStamp == nil {
		return nil
	}
	return b.Hooks.BeforeOrdinalStamp(id)
}

// BackfillOrdinals stamps a creation ordinal on every entity of every card
// that carries none. It reports how many it stamped, and returns a finding for
// every entity whose write order it could not recover and for every card a
// lock kept it out of.
//
// The walk covers the three collections a positional reference reaches below a
// card: the card's comments, the card's attachments and the card's checklist,
// plus the attachments of each comment. Collections hanging off the workbench
// itself and off a column are left alone, because no reference syntax selects a
// member of one by position and an ordinal there would order nothing.
//
// A locked card, and an entity this run cannot write to, are each reported and
// stepped over rather than ending the walk. A repair that abandons the
// workbench on the first obstruction leaves the operator with a bare refusal
// and no account of what it had already stamped, including the guesses it had
// already made, and both obstructions it meets this way are ordinary: another
// process holds a lock right now, or one file stands in the way, and the
// migration can be run again once either clears.
//
// This is a one-time repair run by hand, not a read path. A read of an
// unmigrated collection recovers the same order from the same journal, which
// is what holds a position on one entity across this run, but it derives no
// ordinal and stores none, so a workbench nobody migrated is still caught by
// check's missing-ordinal finding.
func (b *Bench) BackfillOrdinals(actor, now string) (int, []Finding, error) {
	stamped := 0
	var findings []Finding
	for _, id := range ListIDs(b.CardsRoot()) {
		dir := filepath.Join(b.CardsRoot(), id)
		if !Exists(filepath.Join(dir, CardAnchor)) {
			continue
		}
		lock, err := Acquire(dir, actor, now)
		if err != nil {
			findings = append(findings, Finding{Path: dir, Key: FindingOrdinalLocked, Detail: id})
			continue
		}
		count, reported, err := b.backfillCard(dir)
		lock.Release()
		stamped += count
		findings = append(findings, reported...)
		if err != nil {
			return stamped, findings, err
		}
	}
	return stamped, findings, nil
}

// backfillCard stamps the collections of one card, whose lock the caller
// holds.
func (b *Bench) backfillCard(dir string) (int, []Finding, error) {
	events, _, err := ReadJournal(filepath.Join(dir, JournalName))
	if err != nil {
		return 0, nil, err
	}
	order := journalOrder(events)
	stamped := 0
	var findings []Finding
	for _, collection := range ordinalCollections(dir) {
		count, reported := b.backfillCollection(collection.dir, collection.anchor, order)
		stamped += count
		findings = append(findings, reported...)
	}
	return stamped, findings, nil
}

// ordinalCollection is one collection an ordinal is assigned within, named
// together with the anchor its entities carry.
type ordinalCollection struct {
	// dir is the collection directory.
	dir string
	// anchor is the anchor filename the collection's entities carry.
	anchor string
}

// ordinalCollections lists the collections below one card that a positional
// reference selects a member of: everything a card mounts, and everything each
// of its comments mounts.
//
// The list is derived from Contains rather than written out here, so a kind
// gaining a collection reaches the ordinal migration without this function
// being edited.
func ordinalCollections(cardDir string) []ordinalCollection {
	var collections []ordinalCollection
	for _, mount := range Contains(KindCard) {
		dir := filepath.Join(cardDir, mount.Dir)
		collections = append(collections, ordinalCollection{dir: dir, anchor: mount.Anchor})
		if mount.Kind != KindComment {
			continue
		}
		for _, id := range ListIDs(dir) {
			collections = append(collections, belowComment(filepath.Join(dir, id))...)
		}
	}
	return collections
}

// belowComment lists the collections one comment mounts, which the card walk
// above reaches for each comment it finds.
func belowComment(commentDir string) []ordinalCollection {
	var collections []ordinalCollection
	for _, mount := range Contains(KindComment) {
		dir := filepath.Join(commentDir, mount.Dir)
		collections = append(collections, ordinalCollection{dir: dir, anchor: mount.Anchor})
	}
	return collections
}
