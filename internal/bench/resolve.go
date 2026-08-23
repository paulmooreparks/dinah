package bench

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// Resolved is a card found by reference, together with whatever the
// resolution wants to say about the reference itself.
type Resolved struct {
	// Card is the card the reference names.
	Card *Card
	// StalePrefix is the prefix a reference carried that names no current
	// slug. Within a bench a reference resolves on its number, so a prefix
	// left over from a rename identifies exactly one card and is accepted
	// with this warning rather than refused.
	StalePrefix string
}

// ResolveCard finds the card a reference names. The accepted forms are the
// 12-hex identifier and the dash-joined reference whose last segment is the
// card's number.
func (b *Bench) ResolveCard(ref string) (*Resolved, error) {
	return b.resolveCardIn(b.CardsRoot(), ref)
}

// ResolveArchivedCard finds a card in the archive mirror, by the same grammar
// ResolveCard accepts. Only a caller that has already failed to resolve a
// reference against the live half has any business here, because reading the
// mirror's anchors is work the live path never does.
func (b *Bench) ResolveArchivedCard(ref string) (*Resolved, error) {
	return b.resolveCardIn(b.ArchivedCardsRoot(), ref)
}

// resolveCardIn is the resolution both halves of the collection share. The
// grammar is identical either side; only the directory the numbers are read
// out of differs.
func (b *Bench) resolveCardIn(root, ref string) (*Resolved, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, contract.Refuse(contract.UnknownCard, ref)
	}
	if IsID(ref) {
		card, err := LoadCard(root, ref)
		if err != nil {
			return nil, contract.Refuse(contract.UnknownCard, ref)
		}
		return &Resolved{Card: card}, nil
	}
	prefix, number, ok := splitRef(ref)
	if !ok {
		return nil, contract.Refuse(contract.UnknownCard, ref)
	}
	cards, err := cardsIn(root)
	if err != nil {
		return nil, err
	}
	for _, card := range cards {
		if card.Number != number {
			continue
		}
		found := &Resolved{Card: card}
		if prefix != "" && prefix != b.Slug {
			found.StalePrefix = prefix
		}
		return found, nil
	}
	return nil, contract.Refuse(contract.UnknownCard, ref)
}

// splitRef reads a card reference of the form <anything>-<number> into its
// prefix and its number. The number is the durable half of the pair, so it is
// what the resolution keys on.
func splitRef(ref string) (string, int, bool) {
	cut := strings.LastIndex(ref, "-")
	if cut < 0 {
		number, err := strconv.Atoi(ref)
		if err != nil || number <= 0 {
			return "", 0, false
		}
		return "", number, true
	}
	number, err := strconv.Atoi(ref[cut+1:])
	if err != nil || number <= 0 {
		return "", 0, false
	}
	return ref[:cut], number, true
}

// checklistKinds maps the short segments a path reference may carry onto the
// checklist kind they select. The spellings are the ones the board vocabulary
// already uses for the three kinds.
var checklistKinds = map[string]string{
	"oq": "open_question",
	"ac": "acceptance_criterion",
	"d":  "decision",
}

// ResolvePath resolves a reference to an absolute path: the workbench itself,
// a state, a workstream, a card, or anything below any of the first three
// composed by path. It is what the plumbing guarantee of `path` rests on,
// what `edit` walks, and what `show` walks for the composed form.
//
// A workstream resolves to its directory rather than to its anchor, because
// its notes, its journal and its attachments all sit inside it and the
// reference names the entity rather than one file of it. A workstream is
// tried first, per WorkstreamRefPrefix and the reasoning on resolveWorkstreamRef,
// before the rest of the grammar gets a chance to shadow it.
func (b *Bench) ResolvePath(ref string) (string, error) {
	if rest, named := strings.CutPrefix(strings.TrimSpace(ref), WorkstreamRefPrefix); named {
		workstream := b.WorkstreamByRef(rest)
		if workstream == nil {
			return "", contract.Refuse(contract.UnknownWorkstream, rest)
		}
		return filepath.Abs(workstream.Dir)
	}
	path, _, err := b.resolveBelow(ref)
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

// resolveBelow resolves a reference to the file it names, and to the card that
// file belongs to when it belongs to one. ResolvePath and ResolveEntity are
// two readings of that pair, so both accept the same references.
//
// The head segment names where the walk starts and the rest descends through
// the containment grammar. A state is an entity of the workbench and the
// containment walk draws one, so the reference a walk prints for it opens the
// state the way every other reference opens what it names. The slug heads a
// path below the workbench without naming the workbench itself, which is the
// form the walk prints for a workbench attachment; whether the bare slug also
// opens the workbench is a separate question and this does not answer it.
func (b *Bench) resolveBelow(ref string) (string, *Card, error) {
	head, rest, _ := strings.Cut(strings.TrimSpace(ref), "/")
	if IsWorkbenchRef(head) || (rest != "" && b.Slug != "" && head == b.Slug) {
		if rest == "" {
			return filepath.Join(b.Root, WorkbenchAnchor), nil, nil
		}
		path, err := descend(b.Root, KindWorkbench, strings.Split(rest, "/"), nil)
		return path, nil, err
	}
	if state := b.StateByRef(head); state != nil {
		if rest == "" {
			return b.StateAnchorPath(state.ID), nil, nil
		}
		dir := filepath.Join(b.Root, StatesDir, state.ID)
		path, err := descend(dir, KindState, strings.Split(rest, "/"), nil)
		return path, nil, err
	}
	found, err := b.ResolveCard(head)
	if err != nil {
		return "", nil, err
	}
	path, err := walkBelowCard(found.Card, rest)
	return path, found.Card, err
}

// walkBelowCard resolves the segments below a card. An empty rest is the
// card's own anchor, which is what makes `path <card>` open the card.
func walkBelowCard(card *Card, rest string) (string, error) {
	if rest == "" {
		return card.AnchorPath(), nil
	}
	segments := strings.Split(rest, "/")
	head := segments[0]
	// The card's own two files are named by segment rather than by
	// collection, so they are answered ahead of the grammar. Neither is an
	// entity of the containment table: the anchor is the card itself and the
	// journal is content.
	if head == CardAnchor || head == KindCard {
		return card.AnchorPath(), nil
	}
	if head == "journal" || head == JournalName {
		return card.JournalPath(), nil
	}
	if kind, ok := checklistKinds[head]; ok {
		items, ok := checklistMount()
		if !ok {
			return "", contract.Refuse(contract.UnknownPath, rest)
		}
		aliased := append([]string{items.Dir}, segments[1:]...)
		return descend(card.Dir, KindCard, aliased, &kind)
	}
	return descend(card.Dir, KindCard, segments, nil)
}

// checklistMount is the collection a checklist alias such as oq narrows, read
// off the containment grammar so the aliases follow the table rather than a
// second statement of where a checklist lives.
func checklistMount() (Mount, bool) {
	for _, mount := range Contains(KindCard) {
		if mount.Kind == KindItem {
			return mount, true
		}
	}
	return Mount{}, false
}

// descend resolves the segments below one entity by walking the containment
// grammar a collection at a time. A pair of segments names a collection and
// then a member of it, and the member's own kind decides what the pair after
// that may name, so a reference reaches as deep as the grammar goes.
//
// A collection holding a kind that is addressed in its own right is refused,
// so the workbench's cards and states are reached by the address a person
// types for them and by nothing else. See addressedInItsOwnRight.
//
// A segment the grammar does not know is refused rather than dropped. The
// resolver used to read the first collection and discard everything past the
// entity it found, which made `<card>/comments/1/attachments/1` open the
// comment: an address the containment walk prints and a different file behind
// it, with nothing said.
//
// A kind narrows the collection's members first, which is what a checklist
// alias such as oq selects on. Position counts in creation order rather than
// in the listing's ascending-hex order, so `<card>/comment/2` names the second
// comment somebody wrote and keeps naming it however the identifiers happened
// to fall.
func descend(dir, kind string, segments []string, narrow *string) (string, error) {
	mount, ok := MountOf(kind, segments[0])
	if !ok {
		return "", contract.Refuse(contract.UnknownPath, segments[0])
	}
	if addressedInItsOwnRight(mount.Kind) {
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
	collection := filepath.Join(dir, mount.Dir)
	tail := segments[1:]
	if len(tail) == 0 {
		if !Exists(collection) {
			return "", contract.Refuse(contract.UnknownPath, collection)
		}
		return collection, nil
	}
	ids := SortByOrdinal(collection, mount.Anchor, ListIDs(collection))
	if narrow != nil {
		ids = filterByKind(collection, mount.Anchor, ids, *narrow)
	}
	id, err := pick(collection, mount, ids, tail[0])
	if err != nil {
		return "", err
	}
	member := filepath.Join(collection, id)
	below := tail[1:]
	if len(below) == 0 {
		return filepath.Join(member, mount.Anchor), nil
	}
	// An attachment wraps bytes rather than containing entities, so the one
	// segment that may follow one names the payload it wraps.
	if mount.Kind == KindAttachment && below[0] == PayloadDir {
		if len(below) > 1 {
			return "", contract.Refuse(contract.UnknownPath, below[1])
		}
		return payloadOf(member)
	}
	return descend(member, mount.Kind, below, nil)
}

// addressedInItsOwnRight reports whether a kind is one a person names directly
// rather than by its position in the collection that holds it. A card is named
// by its reference and a state by its slug, and each of those addresses is the
// only one either kind has.
//
// The containment walk mounts both under the workbench and draws a row for
// each, and the reference it draws for that row is the direct address rather
// than a composed one, so nothing the tool prints needs the composed form. A
// resolver accepting it anyway gives one entity two spellings, and the two
// then have to be kept equal everywhere: the widened form reached the card
// collection while filling in no card, which crashed the containment walk and
// wrote a card's own history into the workbench journal under the workbench's
// lock.
func addressedInItsOwnRight(kind string) bool {
	return kind == KindCard || kind == KindState
}

// payloadOf is the file an attachment wraps, which is the one file its payload
// directory holds.
func payloadOf(dir string) (string, error) {
	payload := filepath.Join(dir, PayloadDir)
	entries, err := os.ReadDir(payload)
	if err != nil || len(entries) == 0 {
		return "", contract.Refuse(contract.UnknownPath, payload)
	}
	return filepath.Join(payload, entries[0].Name()), nil
}

// filterByKind narrows a collection to the entities whose anchor declares a
// kind, which is how the checklist aliases select one of the three.
func filterByKind(collection, anchor string, ids []string, kind string) []string {
	var kept []string
	for _, id := range ids {
		text, err := ReadText(filepath.Join(collection, id, anchor))
		if err != nil {
			continue
		}
		fm, _ := ParseAnchor(text)
		if fm.Value("kind") == kind {
			kept = append(kept, id)
		}
	}
	return kept
}

// pick selects an entity of a collection by identifier, by one-based position
// within it, or by the value the collection's name field carries in the
// entity's anchor. Precedence is identifier, then position, then name, so an
// existing reference the addition of a name arm cannot change meaning stays
// answered by the form it has always had.
//
// A name selector against a collection that declares no name field refuses
// unknown-path exactly as it does for any other unrecognised selector, which
// is what keeps comments and checklist items behaving as they do today.
//
// Two attachments may carry the same filename, since attach permits it now
// and this card does not narrow attach. A name selector matching more than
// one entity refuses ambiguous-name and carries the ordinal of every match
// alongside the selector, so the caller retries with attachments/<n>.
func pick(collection string, mount Mount, ids []string, selector string) (string, error) {
	if IsID(selector) {
		for _, id := range ids {
			if id == selector {
				return id, nil
			}
		}
		return "", contract.Refuse(contract.UnknownPath, selector)
	}
	position, err := strconv.Atoi(selector)
	if err == nil {
		if position >= 1 && position <= len(ids) {
			return ids[position-1], nil
		}
		return "", contract.Refuse(contract.UnknownPath, selector)
	}
	if mount.NameField != "" {
		matches := matchByName(collection, mount, ids, selector)
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			return "", contract.Refuse(contract.UnknownPath, selector)
		default:
			ordinals := make([]string, 0, len(matches))
			for _, id := range matches {
				ordinals = append(ordinals, strconv.Itoa(EntityOrdinal(collection, id, mount.Anchor)))
			}
			return "", contract.RefuseWith(contract.AmbiguousName, selector, map[string]string{
				"selector": selector,
				"ordinals": strings.Join(ordinals, ","),
			})
		}
	}
	return "", contract.Refuse(contract.UnknownPath, selector)
}

// matchByName returns the identifiers of the entities whose anchor declares
// the collection's name field with the selector as its value. Order matches
// ids, so a single match returns that match's id in its place.
func matchByName(collection string, mount Mount, ids []string, selector string) []string {
	var matches []string
	for _, id := range ids {
		fm, _ := loadAnchor(filepath.Join(collection, id, mount.Anchor))
		if fm.Value(mount.NameField) == selector {
			matches = append(matches, id)
		}
	}
	return matches
}
