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
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, contract.Refuse(contract.UnknownCard, ref)
	}
	if IsID(ref) {
		card, err := LoadCard(b.CardsRoot(), ref)
		if err != nil {
			return nil, contract.Refuse(contract.UnknownCard, ref)
		}
		return &Resolved{Card: card}, nil
	}
	prefix, number, ok := splitRef(ref)
	if !ok {
		return nil, contract.Refuse(contract.UnknownCard, ref)
	}
	cards, err := b.Cards()
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
// a card, or anything below a card composed by path. It is what the plumbing
// guarantee of `path` rests on, what `edit` walks, and what `show` walks for
// the composed form below a card.
func (b *Bench) ResolvePath(ref string) (string, error) {
	if IsWorkbenchRef(ref) {
		return filepath.Abs(filepath.Join(b.Root, WorkbenchAnchor))
	}
	// A state is an entity of the workbench and the containment walk draws
	// one, so the reference a walk prints for it opens the state the way
	// every other reference opens what it names.
	if state := b.StateByRef(strings.TrimSpace(ref)); state != nil {
		return filepath.Abs(b.StateAnchorPath(state.ID))
	}
	head, rest, _ := strings.Cut(strings.TrimSpace(ref), "/")
	found, err := b.ResolveCard(head)
	if err != nil {
		return "", err
	}
	path, err := walkBelowCard(found.Card, rest)
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

// walkBelowCard resolves the segments below a card. An empty rest is the
// card's own anchor, which is what makes `path <card>` open the card.
func walkBelowCard(card *Card, rest string) (string, error) {
	if rest == "" {
		return card.AnchorPath(), nil
	}
	segments := strings.Split(rest, "/")
	head := segments[0]
	tail := segments[1:]
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
	if mount, ok := MountOf(KindCard, head); ok {
		if mount.Kind == KindAttachment {
			return attachmentBelow(filepath.Join(card.Dir, mount.Dir), tail)
		}
		return entityBelow(filepath.Join(card.Dir, mount.Dir), mount.Anchor, tail, nil)
	}
	kind, ok := checklistKinds[head]
	if !ok {
		return "", contract.Refuse(contract.UnknownPath, rest)
	}
	items, ok := checklistMount()
	if !ok {
		return "", contract.Refuse(contract.UnknownPath, rest)
	}
	return entityBelow(filepath.Join(card.Dir, items.Dir), items.Anchor, tail, &kind)
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

// entityBelow resolves a collection, or one entity of it named by identifier
// or by one-based position. A kind narrows the collection first, which is
// what a checklist alias such as oq selects on.
//
// Position counts in creation order rather than in the listing's ascending-hex
// order, so `<card>/comment/2` names the second comment somebody wrote and
// keeps naming it however the identifiers happened to fall.
func entityBelow(collection, anchor string, tail []string, kind *string) (string, error) {
	ids := SortByOrdinal(collection, anchor, ListIDs(collection))
	if kind != nil {
		ids = filterByKind(collection, anchor, ids, *kind)
	}
	if len(tail) == 0 {
		if !Exists(collection) {
			return "", contract.Refuse(contract.UnknownPath, collection)
		}
		return collection, nil
	}
	id, err := pick(ids, tail[0])
	if err != nil {
		return "", err
	}
	return filepath.Join(collection, id, anchor), nil
}

// attachmentBelow resolves an attachment, and its payload file when the
// reference reaches past the entity into the bytes it wraps.
func attachmentBelow(collection string, tail []string) (string, error) {
	path, err := entityBelow(collection, AttachmentAnchor, tail, nil)
	if err != nil {
		return "", err
	}
	if len(tail) < 2 || tail[1] != PayloadDir {
		return path, nil
	}
	payload := filepath.Join(filepath.Dir(path), PayloadDir)
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

// pick selects an entity of a collection by identifier or by one-based
// position within it.
func pick(ids []string, selector string) (string, error) {
	if IsID(selector) {
		for _, id := range ids {
			if id == selector {
				return id, nil
			}
		}
		return "", contract.Refuse(contract.UnknownPath, selector)
	}
	position, err := strconv.Atoi(selector)
	if err != nil || position < 1 || position > len(ids) {
		return "", contract.Refuse(contract.UnknownPath, selector)
	}
	return ids[position-1], nil
}
