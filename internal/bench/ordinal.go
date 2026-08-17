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
// The ordinal is what a positional reference such as `<card>/comment/2`
// selects on. The directory listing cannot serve that reference, because a
// hex identifier is random and the listing is therefore in no order anybody
// wrote in, and the comment timestamp cannot either, because it is wall-clock
// and two processes writing inside one second record the same value.
const OrdinalField = "ordinal"

// EntityOrdinal reads the creation ordinal of one entity of a collection. An
// entity carrying no ordinal, an unparseable one, or a value below one reads
// as zero, which is the value fsck reports as missing and the migration fills
// in.
func EntityOrdinal(collection, id, anchor string) int {
	fm, _ := loadAnchor(filepath.Join(collection, id, anchor))
	n, err := strconv.Atoi(fm.Value(OrdinalField))
	if err != nil || n < 1 {
		return 0
	}
	return n
}

// nextOrdinal returns the ordinal a new entity of a collection takes: one past
// the highest already in use there.
//
// The scan is safe without a mutex of its own because every caller holds the
// lock of the nearest enclosing journal-bearing entity before it adds to the
// collection, so no second writer can be between this scan and the write that
// follows it.
func nextOrdinal(collection, anchor string) int {
	highest := 0
	for _, id := range ListIDs(collection) {
		if n := EntityOrdinal(collection, id, anchor); n > highest {
			highest = n
		}
	}
	return highest + 1
}

// SortByOrdinal returns a collection's identifiers in creation order. An
// entity carrying no ordinal sorts ahead of every stamped one and keeps its
// place in the listing order relative to its unstamped neighbours, which is
// what an unmigrated workbench falls back to until fsck's missing-ordinal
// finding is acted on.
func SortByOrdinal(collection, anchor string, ids []string) []string {
	ordered := append([]string(nil), ids...)
	ordinals := make(map[string]int, len(ordered))
	for _, id := range ordered {
		ordinals[id] = EntityOrdinal(collection, id, anchor)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordinals[ordered[i]] < ordinals[ordered[j]]
	})
	return ordered
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
// for following in listing order.
//
// A hand-created entity, and every entity of a collection whose creation
// events a torn journal lost, is in that trailing stretch. Listing order is no
// worse there than what a positional reference resolved to before ordinals
// existed, and fsck reports the torn journal separately.
func orderedByJournal(collection string, order []string) []string {
	present := map[string]bool{}
	for _, id := range ListIDs(collection) {
		present[id] = true
	}
	var ordered []string
	seen := map[string]bool{}
	for _, id := range order {
		if !present[id] || seen[id] {
			continue
		}
		seen[id] = true
		ordered = append(ordered, id)
	}
	for _, id := range ListIDs(collection) {
		if !seen[id] {
			ordered = append(ordered, id)
		}
	}
	return ordered
}

// backfillCollection stamps an ordinal on every entity of one collection that
// carries none, in the order the journal says they were written.
//
// An entity already carrying an ordinal keeps it, and the values those
// entities hold are stepped over rather than reused, so a collection stamped
// halfway through an interrupted run finishes without a duplicate and a
// collection stamped in full is left exactly as it stands. That is what makes
// a second run of the migration change nothing.
func backfillCollection(collection, anchor string, order []string) (int, error) {
	ordered := orderedByJournal(collection, order)
	taken := map[int]bool{}
	for _, id := range ordered {
		if n := EntityOrdinal(collection, id, anchor); n > 0 {
			taken[n] = true
		}
	}
	stamped := 0
	next := 1
	for _, id := range ordered {
		if EntityOrdinal(collection, id, anchor) > 0 {
			continue
		}
		for taken[next] {
			next++
		}
		if err := stampOrdinal(collection, id, anchor, next); err != nil {
			return stamped, err
		}
		taken[next] = true
		stamped++
	}
	return stamped, nil
}

// BackfillOrdinals stamps a creation ordinal on every entity of every card
// that carries none, and reports how many it stamped.
//
// The walk covers the three collections a positional reference reaches below a
// card: the card's comments, the card's attachments and the card's checklist,
// plus the attachments of each comment. Collections hanging off the workbench
// itself and off a state are left alone, because no reference syntax selects a
// member of one by position and an ordinal there would order nothing.
//
// This is a one-time repair run by hand, not a read path. Nothing re-derives
// an ordinal on a later read, so a workbench nobody migrated is caught by
// fsck's missing-ordinal finding rather than quietly ordered by its directory
// listing forever.
func (b *Bench) BackfillOrdinals(actor, now string) (int, error) {
	stamped := 0
	for _, id := range ListIDs(b.CardsRoot()) {
		dir := filepath.Join(b.CardsRoot(), id)
		if !Exists(filepath.Join(dir, CardAnchor)) {
			continue
		}
		lock, err := Acquire(dir, actor, now)
		if err != nil {
			return stamped, err
		}
		count, err := b.backfillCard(dir)
		lock.Release()
		stamped += count
		if err != nil {
			return stamped, err
		}
	}
	return stamped, nil
}

// backfillCard stamps the collections of one card, whose lock the caller
// holds.
func (b *Bench) backfillCard(dir string) (int, error) {
	events, _, err := ReadJournal(filepath.Join(dir, JournalName))
	if err != nil {
		return 0, err
	}
	order := journalOrder(events)
	stamped := 0
	for _, collection := range ordinalCollections(dir) {
		count, err := backfillCollection(collection.dir, collection.anchor, order)
		stamped += count
		if err != nil {
			return stamped, err
		}
	}
	return stamped, nil
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
// reference selects a member of: the card's comments, attachments and
// checklist, and the attachments of each comment.
func ordinalCollections(cardDir string) []ordinalCollection {
	comments := filepath.Join(cardDir, CommentsDir)
	collections := []ordinalCollection{
		{dir: comments, anchor: CommentAnchor},
		{dir: filepath.Join(cardDir, AttachmentsDir), anchor: AttachmentAnchor},
		{dir: filepath.Join(cardDir, ChecklistDir), anchor: ItemAnchor},
	}
	for _, id := range ListIDs(comments) {
		attachments := filepath.Join(comments, id, AttachmentsDir)
		collections = append(collections, ordinalCollection{dir: attachments, anchor: AttachmentAnchor})
	}
	return collections
}
