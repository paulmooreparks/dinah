package bench

import (
	"path/filepath"

	"dinah/internal/contract"
)

// Positions answers listings, anchor texts, collection order and member
// position for one read composition, listing each collection at most once and
// reading each anchor at most once. It is not safe to keep past the
// composition that made it, because it never re-lists a collection or
// re-reads an anchor. It is not safe for concurrent use.
//
// The uncached readers it stands beside, ListIDs, SortByOrdinal,
// MemberPosition, Items, Comments and Attachments, stay what every other
// caller uses. A composition that asks one question many times, which is
// show deriving the position of every member of a collection, asks it here
// instead, so the collection is sorted once rather than once per member.
type Positions struct {
	src    Source                // where the composition reads
	listed map[string]listing    // collection path -> what ListIDs answered
	texts  map[string]anchorText // anchor path -> what ReadText answered
	sorted map[string][]string   // collection path -> ids in SortByOrdinal order
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
	text     string
	revision string
	err      error
}

// NewPositions makes an empty memo for one composition.
func NewPositions() *Positions {
	return newPositions(Disk{})
}

// NewPositions makes an empty memo for one composition that reads through
// this bench's source.
func (b *Bench) NewPositions() *Positions {
	return newPositions(b.source())
}

// newPositions makes an empty memo reading through src.
func newPositions(src Source) *Positions {
	return &Positions{
		src:    src,
		listed: map[string]listing{},
		texts:  map[string]anchorText{},
		sorted: map[string][]string{},
	}
}

// IDs is ListIDs(collection), called at most once per collection.
func (p *Positions) IDs(collection string) ([]string, error) {
	if kept, ok := p.listed[collection]; ok {
		return kept.ids, kept.err
	}
	ids, err := listIDs(p.src, collection)
	p.listed[collection] = listing{ids: ids, err: err}
	return ids, err
}

// Text is ReadText(path), called at most once per path.
func (p *Positions) Text(path string) (string, error) {
	kept := p.textAndRevision(path)
	return kept.text, kept.err
}

// textAndRevision is what one read of an anchor answered, read at most once
// per path.
func (p *Positions) textAndRevision(path string) anchorText {
	if kept, ok := p.texts[path]; ok {
		return kept
	}
	text, revision, err := readTextAndRevision(p.src, path)
	kept := anchorText{text: text, revision: revision, err: err}
	p.texts[path] = kept
	return kept
}

// derive answers Derive for one anchor. Over a source that rereads, the
// composition keeps the text itself and derives from it, so an anchor is read
// at most once however many readers of the composition ask for it; over a
// source that memoises, it asks the source, whose memo is shared beyond the
// composition.
func (p *Positions) derive(path string, kind DeriveKind, derive func(path, text, revision string) (any, error)) (any, error) {
	if _, rereads := p.src.(rereader); rereads {
		kept := p.textAndRevision(path)
		if kept.err != nil {
			return nil, kept.err
		}
		return derive(path, kept.text, kept.revision)
	}
	observeAnchor(path)
	return p.src.Derive(path, kind, derive)
}

// ChildIDs is IDs of each collection the containment grammar gives a kind,
// keyed by the collection's directory name, walking Contains(kind) as
// ChildCounts does. A collection that will not read is reported rather than
// answered as empty, on the terms ChildCounts states.
func (p *Positions) ChildIDs(dir, kind string) (map[string][]string, error) {
	listed := map[string][]string{}
	for _, mount := range Contains(kind) {
		ids, err := p.IDs(filepath.Join(dir, mount.Dir))
		if err != nil {
			return nil, err
		}
		listed[mount.Dir] = ids
	}
	return listed, nil
}

// Item is LoadItem(dir) answered from Text, refusing with the path LoadItem
// refuses with when the anchor will not read.
func (p *Positions) Item(dir string) (*Item, error) {
	value, err := p.derive(filepath.Join(dir, ItemAnchor), DeriveItem, deriveItem)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, dir)
	}
	return value.(*Item).Clone(), nil
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
	ordered := sortByOrdinalWith(p.src, collection, ids, func(id string) int {
		value, err := p.derive(filepath.Join(collection, id, anchor), DeriveAnchor, deriveAnchor)
		if err != nil {
			return 0
		}
		return OrdinalOf(value.(*parsedAnchor).fm)
	})
	p.sorted[collection] = ordered
	return ordered, nil
}

// Of is MemberPosition(dir, anchor) answered from Sorted: the one-based place
// the member holds in the unfiltered sorted collection, and zero when the
// directory is not a member of it.
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

// Items is Items(cardDir), in Sorted order, each item built by Item. An item
// whose anchor will not read is skipped, as Items skips it.
func (p *Positions) Items(cardDir string) ([]*Item, error) {
	collection := filepath.Join(cardDir, ChecklistDir)
	ordered, err := p.Sorted(collection, ItemAnchor)
	if err != nil {
		return nil, err
	}
	var items []*Item
	for _, id := range ordered {
		item, err := p.Item(filepath.Join(collection, id))
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// Comments is Comments(holderDir), in Sorted order, each comment built by
// commentFromText from Text. A comment whose anchor will not read is skipped,
// as Comments skips it.
func (p *Positions) Comments(holderDir string) ([]*Comment, error) {
	collection := filepath.Join(holderDir, CommentsDir)
	ordered, err := p.Sorted(collection, CommentAnchor)
	if err != nil {
		return nil, err
	}
	var comments []*Comment
	for _, id := range ordered {
		value, err := p.derive(filepath.Join(collection, id, CommentAnchor), DeriveComment, deriveComment)
		if err != nil {
			continue
		}
		comments = append(comments, value.(*Comment).Clone())
	}
	return comments, nil
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
		value, err := p.derive(filepath.Join(collection, id, AttachmentAnchor), DeriveAttachment, deriveAttachment)
		if err != nil {
			continue
		}
		attachment := value.(*Attachment).Clone()
		withPayload(p.src, attachment)
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// CountComments is CountComments(dir) answered from IDs.
func (p *Positions) CountComments(dir string) (int, error) {
	ids, err := p.IDs(filepath.Join(dir, CommentsDir))
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// DesignatedCommentDir is Bench.DesignatedCommentDir(item) answered from IDs
// and Text. It searches the same two holders in the same order, the item's own
// comments and then its archived ones, and a member matching item.Resolution
// counts only when its anchor reads, because Comments skips a member whose
// anchor will not read and the uncached lookup therefore cannot find one.
func (p *Positions) DesignatedCommentDir(item *Item) (string, bool) {
	if item == nil || item.Resolution == "" {
		return "", false
	}
	for _, holder := range []string{item.Dir, filepath.Join(item.Dir, ArchiveDir)} {
		collection := filepath.Join(holder, CommentsDir)
		ids, err := p.IDs(collection)
		if err != nil {
			continue
		}
		for _, id := range ids {
			if id != item.Resolution {
				continue
			}
			dir := filepath.Join(collection, id)
			if _, err := p.Text(filepath.Join(dir, CommentAnchor)); err == nil {
				return dir, true
			}
		}
	}
	return "", false
}
