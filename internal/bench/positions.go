package bench

import (
	"path/filepath"
)

// Positions answers listings, anchor texts, collection order, member position
// and each card's record for one read composition, listing each collection at
// most once, reading each anchor at most once, and reading each card's members
// once. It is not safe to keep past the composition that made it, because it
// never re-lists a collection, re-reads an anchor or re-reads a card. It is
// not safe for concurrent use.
//
// The uncached readers it stands beside, ListIDs, SortByOrdinal, Attachments
// and LoadCardRecord, stay what every other caller uses. A composition that
// asks one question many times, which is show deriving the position of every
// member of a collection, asks it here instead, so the collection is sorted
// once rather than once per member.
type Positions struct {
	listed  map[string]listing     // collection path -> what ListIDs answered
	texts   map[string]anchorText  // anchor path -> what ReadText answered
	sorted  map[string][]string    // collection path -> ids in SortByOrdinal order
	records map[string]cardRecords // card directory -> what LoadCardRecord answered
}

// listing is what one ListIDs call answered, error included, so a collection
// that would not read answers the same failure every time it is asked for.
type listing struct {
	ids []string
	err error
}

// anchorText is what one ReadText call answered, error included, so an anchor
// that would not read is skipped by every reader in the composition alike.
type anchorText struct {
	text string
	err  error
}

// cardRecords is what one LoadCardRecord call answered, error included.
type cardRecords struct {
	record *CardRecord
	err    error
}

// NewPositions makes an empty memo for one composition.
func NewPositions() *Positions {
	return &Positions{
		listed:  map[string]listing{},
		texts:   map[string]anchorText{},
		sorted:  map[string][]string{},
		records: map[string]cardRecords{},
	}
}

// IDs is ListIDs(collection), called at most once per collection.
func (p *Positions) IDs(collection string) ([]string, error) {
	if kept, ok := p.listed[collection]; ok {
		return kept.ids, kept.err
	}
	ids, err := ListIDs(collection)
	p.listed[collection] = listing{ids: ids, err: err}
	return ids, err
}

// Text is ReadText(path), called at most once per path.
func (p *Positions) Text(path string) (string, error) {
	if kept, ok := p.texts[path]; ok {
		return kept.text, kept.err
	}
	text, err := ReadText(path)
	p.texts[path] = anchorText{text: text, err: err}
	return text, err
}

// Record is b.LoadCardRecord(card), called at most once per card.
func (p *Positions) Record(b *Bench, card *Card) (*CardRecord, error) {
	if kept, ok := p.records[card.Dir]; ok {
		return kept.record, kept.err
	}
	record, err := b.LoadCardRecord(card)
	p.records[card.Dir] = cardRecords{record: record, err: err}
	return record, err
}

// CardCounts is how many members sit in each collection a card mounts, keyed
// by the collection's segment, together with the card's live items: the live
// comments and the live checklist from the card's record, and the attachments
// from their directory. A collection that will not read is reported rather
// than counted as none, since a zero is what a card holding nothing answers.
func (p *Positions) CardCounts(b *Bench, card *Card) (map[string]int, []*Item, error) {
	record, err := p.Record(b, card)
	if err != nil {
		return nil, nil, err
	}
	counts := map[string]int{}
	for _, mount := range Contains(KindCard) {
		if mount.Journaled && mount.Kind == KindComment {
			count, err := record.HeldCount("", LiveHalf)
			if err != nil {
				return nil, nil, err
			}
			counts[mount.Dir] = count
			continue
		}
		if mount.Journaled {
			counts[mount.Dir] = len(record.MemberIDs(MemberCollection{Kind: mount.Kind}, LiveHalf, ""))
			continue
		}
		ids, err := p.IDs(filepath.Join(card.Dir, mount.Dir))
		if err != nil {
			return nil, nil, err
		}
		counts[mount.Dir] = len(ids)
	}
	return counts, record.ItemsIn(LiveHalf, ""), nil
}

// Sorted is IDs(collection) sorted as SortByOrdinal sorts it, with each
// member's ordinal read from Text. A member whose anchor will not read sorts
// at ordinal zero, which is what EntityOrdinal answers for it, and it stays in
// the answer, because a position counts the unfiltered collection.
func (p *Positions) Sorted(collection, anchor string) ([]string, error) {
	if kept, ok := p.sorted[collection]; ok {
		return kept, nil
	}
	ids, err := p.IDs(collection)
	if err != nil {
		return nil, err
	}
	ordered := sortByOrdinalWith(collection, ids, func(id string) int {
		text, err := p.Text(filepath.Join(collection, id, anchor))
		if err != nil {
			return 0
		}
		fm, _ := ParseAnchor(text)
		return OrdinalOf(fm)
	})
	p.sorted[collection] = ordered
	return ordered, nil
}

// Of is the one-based place a directory member holds in its unfiltered sorted
// collection, and zero when the directory is not a member of it.
func (p *Positions) Of(dir, anchor string) (int, error) {
	ordered, err := p.Sorted(filepath.Dir(dir), anchor)
	if err != nil {
		return 0, err
	}
	id := filepath.Base(dir)
	for n, member := range ordered {
		if member == id {
			return n + 1, nil
		}
	}
	return 0, nil
}

// Attachments is Attachments(dir), in Sorted order, each attachment built by
// attachmentFromText from Text. An attachment whose anchor will not read is
// skipped, as Attachments skips it.
func (p *Positions) Attachments(dir string) ([]*Attachment, error) {
	collection := filepath.Join(dir, AttachmentsDir)
	ordered, err := p.Sorted(collection, AttachmentAnchor)
	if err != nil {
		return nil, err
	}
	var attachments []*Attachment
	for _, id := range ordered {
		member := filepath.Join(collection, id)
		text, err := p.Text(filepath.Join(member, AttachmentAnchor))
		if err != nil {
			continue
		}
		attachments = append(attachments, attachmentFromText(member, id, text))
	}
	return attachments, nil
}
