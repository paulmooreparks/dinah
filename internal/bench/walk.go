package bench

import (
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// walkAt is where a reference walk stands: one entity, the card the walk is
// inside when it is inside one, and the reference composed for the entity so
// far.
type walkAt struct {
	// dir is the entity's directory. A journaled member has none of its own
	// in the card-unit layout, so for an item it is the path its old
	// directory would have had, which nothing reads, and for a comment it is
	// the directory its attachments hang from.
	dir string
	// kind is the entity's kind.
	kind string
	// card is the card the walk is inside, nil above one.
	card *Card
	// holder is the identifier a journaled member below this entity hangs
	// on: the item's own for an item, the column's own for a column, and
	// empty for a card.
	holder string
	// ref is the reference composed for the entity: the head's own, then
	// one collection name and one position for each level down to it.
	ref string
	// record is the card's record, read once for the walk.
	record *CardRecord
}

// recordOf answers the record of the card the walk is inside, reading it the
// first time a step asks.
func (b *Bench) recordOf(at *walkAt) (*CardRecord, error) {
	if at.record != nil {
		return at.record, nil
	}
	record, err := b.LoadCardRecord(at.card)
	if err != nil {
		return nil, err
	}
	at.record = record
	return record, nil
}

// members answers one collection's members in one half, in the creation
// order a position counts in, narrowed to one item kind unless kind is empty.
// A collection of directories is listed; a journaled one is asked of the
// card's record, or of the workbench journal for a column's comments.
func (b *Bench) members(at *walkAt, mount Mount, collection string, half ResolutionHalf, kind string) ([]string, error) {
	if !mount.Journaled {
		ids, err := MemberIDs(collection, mount)
		if err != nil {
			return nil, err
		}
		if kind != "" {
			ids = filterByKind(collection, mount.Anchor, ids, kind)
		}
		return ids, nil
	}
	if at.kind == KindColumn {
		comments, err := b.ColumnComments(at.holder, half)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(comments))
		for _, comment := range comments {
			ids = append(ids, comment.ID)
		}
		return ids, nil
	}
	if at.card == nil {
		return nil, nil
	}
	record, err := b.recordOf(at)
	if err != nil {
		return nil, err
	}
	instance := MemberCollection{Kind: mount.Kind}
	if mount.Kind == KindComment && at.kind == KindItem {
		instance.Holder = at.holder
	}
	return record.MemberIDs(instance, half, kind), nil
}

// memberAt answers where the walk stands once it has selected one member of a
// collection.
func (b *Bench) memberAt(at *walkAt, mount Mount, collection, id, ref string, half ResolutionHalf) (*walkAt, error) {
	next := &walkAt{dir: filepath.Join(collection, id), kind: mount.Kind, card: at.card, ref: ref, record: at.record}
	if !mount.Journaled {
		return next, nil
	}
	switch mount.Kind {
	case KindItem:
		next.holder = id
	case KindComment:
		next.holder = at.holder
		if !b.CardUnit() {
			return next, nil
		}
		home, err := b.commentHome(at, id, half)
		if err != nil {
			return nil, err
		}
		next.dir = home
	}
	return next, nil
}

// commentHome answers the directory a comment's attachments hang from in the
// card-unit layout: comments/<id> below the card or column that holds it.
func (b *Bench) commentHome(at *walkAt, id string, half ResolutionHalf) (string, error) {
	if at.kind == KindColumn {
		return filepath.Join(at.dir, CommentsDir, id), nil
	}
	record, err := b.recordOf(at)
	if err != nil {
		return "", err
	}
	if comment, ok := record.Comment(id); ok {
		return comment.Home, nil
	}
	return filepath.Join(at.card.Dir, CommentsDir, id), nil
}

// descend resolves the segments below one entity by walking the containment
// grammar a collection at a time. A pair of segments names a collection and
// then a member of it, and the member's own kind decides what the pair after
// that may name, so a reference reaches as deep as the grammar goes.
//
// A collection holding a kind that is addressed in its own right is refused,
// so the workbench's cards and columns are reached by the address a person
// types for them and by nothing else. See AddressedInItsOwnRight.
//
// A segment the grammar does not know is refused rather than dropped. The
// resolver used to read the first collection and discard everything past the
// entity it found, which made `<card>/comments/1/attachments/1` open the
// comment: an address the containment walk prints and a different file behind
// it, with nothing said.
//
// A kind narrows the collection's members first, which is what a checklist
// segment such as questions selects on. Position counts in creation order
// rather than in the listing's ascending-hex order, so `<card>/comment/2`
// names the second comment somebody wrote and keeps naming it however the
// identifiers happened to fall.
//
// The walk composes the reference of what it reaches as it goes: the head's
// own, then one collection name and one position for each level, the position
// counted in the whole collection of the half that step read, so one entity
// has one spelling however the caller reached it. A journaled member answers
// no path of its own in the card-unit layout, and the landing says what was
// reached; below that format it answers the anchor file the old layout keeps
// it in.
func (b *Bench) descend(at *walkAt, segments []string, narrow *string, landed *landing, half ResolutionHalf) (string, error) {
	mount, ok := MountOf(at.kind, segments[0])
	if !ok {
		return "", contract.Refuse(contract.UnknownPath, segments[0])
	}
	if AddressedInItsOwnRight(mount.Kind) {
		// The segment names a collection this workbench plainly has, so a
		// refusal quoting the segment alone tells a reader that something
		// they can see does not exist. What is refused is the addressing
		// rather than the word, so the whole path below the head is quoted
		// and the next step says how the thing is named instead.
		return "", contract.RefuseWith(
			contract.UnknownPath,
			strings.Join(segments, "/"),
			map[string]string{"addressed": mount.Kind},
		)
	}
	// Whether this call is the reference's deepest collection step is decided
	// from the segment count and the mount kind, and it is decided here,
	// before the collection path is joined, so the half the members are read
	// in is the half the whole reference asked for at its deepest step and
	// the live one everywhere above it.
	deepest := len(segments) == 1 ||
		len(segments) == 2 ||
		(mount.Kind == KindAttachment && len(segments) > 2 && segments[2] == PayloadDir)
	within := LiveHalf
	collection := filepath.Join(at.dir, mount.Dir)
	if deepest && half == ArchivedHalf {
		within = ArchivedHalf
		collection = filepath.Join(at.dir, ArchiveDir, mount.Dir)
	}
	kind := ""
	if narrow != nil {
		kind = *narrow
	}
	tail := segments[1:]
	if len(tail) == 0 {
		// A collection the containment table declares for this kind is
		// there whether or not anything has been written into it, so the
		// walk answers with the directory a first member would be written
		// into rather than refusing over a directory nobody has made yet.
		if landed != nil {
			members, err := b.members(at, mount, collection, within, kind)
			if err != nil {
				return "", err
			}
			landed.collection = true
			landed.dir = collection
			landed.mount = mount
			landed.narrow = kind
			landed.members = members
		}
		return collection, nil
	}
	whole, err := b.members(at, mount, collection, within, "")
	if err != nil {
		return "", err
	}
	selectable := whole
	if kind != "" {
		selectable, err = b.members(at, mount, collection, within, kind)
		if err != nil {
			return "", err
		}
	}
	id, err := pick(collection, mount, selectable, tail[0])
	if err != nil {
		return "", err
	}
	position := 0
	for n, member := range whole {
		if member == id {
			position = n + 1
			break
		}
	}
	ref := at.ref + "/" + mount.Dir + "/" + strconv.Itoa(position)
	next, err := b.memberAt(at, mount, collection, id, ref, within)
	if err != nil {
		return "", err
	}
	below := tail[1:]
	if len(below) == 0 {
		if landed != nil {
			landed.kind = mount.Kind
			landed.id = id
			landed.entityDir = next.dir
			landed.holder = next.holder
			landed.ref = ref
			landed.journaled = mount.Journaled
		}
		if mount.Journaled {
			return b.memberFile(next, mount), nil
		}
		return filepath.Join(next.dir, mount.Anchor), nil
	}
	// An attachment wraps bytes rather than containing entities, so the one
	// segment that may follow one names the payload it wraps.
	if mount.Kind == KindAttachment && below[0] == PayloadDir {
		if len(below) > 1 {
			return "", contract.Refuse(contract.UnknownPath, below[1])
		}
		return payloadOf(next.dir)
	}
	return b.descend(next, below, nil, landed, half)
}

// memberFile answers the file a journaled member names: the anchor the old
// layout keeps it in, below the card-unit format, and nothing in that layout,
// where the member is lines of a journal rather than a file.
func (b *Bench) memberFile(at *walkAt, mount Mount) string {
	if b.CardUnit() {
		return ""
	}
	path, _ := legacyMemberFile(&EntityRef{Kind: mount.Kind, Dir: at.dir})
	return path
}
