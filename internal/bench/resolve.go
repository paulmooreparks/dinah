package bench

import (
	"errors"
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
			// A card the reader refused is a card this workbench has, so
			// its own refusal travels rather than being rewritten into a
			// report that the workbench carries no such card. Only the
			// reader's own unknown-card, which is what an unreadable anchor
			// gives back, is restated here against the reference the caller
			// typed. Without this, the same card answered one way when it
			// was addressed by identifier and another way when it was
			// addressed by its human reference, because the reference route
			// reads the collection and lets the refusal through.
			var refusal *contract.Refusal
			if errors.As(err, &refusal) && refusal.Name != contract.UnknownCard {
				return nil, err
			}
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

// checklistSegments declares, once, how each of the three checklist kinds is
// addressed inside a path reference. Word is the spelling Dinah composes and
// prints. Short is an older spelling the resolver goes on accepting on input
// and never produces, so a reference written down before the words landed
// still opens what it named.
//
// Everything else a workbench holds is addressed by a word, and the three
// short forms were the exception: two of them were abbreviations a reader had
// to be told about, and one was a single letter naming a whole entity kind.
// The words follow comments and attachments instead.
//
// A segment is not a kind. The kind tokens open_question, acceptance_criterion
// and decision are what the format stores, what the wire carries and what the
// machine surface prints, and none of them changes because a reference spells
// the collection it narrows differently.
//
// The two directions of the mapping are both derived from this one
// declaration rather than written out a second time, so the spelling a
// reference resolves by and the spelling it is composed from cannot drift
// apart: an edit here teaches the resolver and the composer together.
var checklistSegments = []struct {
	Kind  string
	Word  string
	Short string
}{
	{Kind: "open_question", Word: "questions", Short: "oq"},
	{Kind: "acceptance_criterion", Word: "criteria", Short: "ac"},
	{Kind: "decision", Word: "decisions", Short: "d"},
}

// checklistKinds maps every segment a path reference may carry onto the
// checklist kind it selects. Both spellings of each kind resolve, which is
// what keeps the short forms working.
var checklistKinds = func() map[string]string {
	kinds := make(map[string]string, 2*len(checklistSegments))
	for _, segment := range checklistSegments {
		kinds[segment.Word] = segment.Kind
		kinds[segment.Short] = segment.Kind
	}
	return kinds
}()

// itemKindWord is checklistSegments read from the kind, and it answers with
// the word alone, because only the word is ever composed.
var itemKindWord = func() map[string]string {
	words := make(map[string]string, len(checklistSegments))
	for _, segment := range checklistSegments {
		words[segment.Kind] = segment.Word
	}
	return words
}()

// WordForItemKind returns the segment a checklist item's kind composes into a
// reference under, and reports whether the kind is one of the three the format
// declares. A kind outside those three composes no reference, since nothing
// would resolve one.
func WordForItemKind(kind string) (string, bool) {
	word, ok := itemKindWord[kind]
	return word, ok
}

// ResolvePath resolves a reference to an absolute path: the workbench itself,
// a column, a workstream, a card, or anything below any of the first three
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
		// The whole reference goes to the resolver rather than the
		// remainder, because that resolver strips the prefix itself so that
		// the workstream-taking commands accept either spelling. Passing the
		// remainder would strip a second time and admit a doubled prefix.
		workstream := b.WorkstreamByRef(strings.TrimSpace(ref))
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

// CollectionRef is what <head>/<collection> resolves to: the directory the
// collection lives in, the containment mount it is, and its live members in
// creation order. A collection is not an entity of the format, so it has no
// anchor and no identifier of its own.
type CollectionRef struct {
	// Ref is the reference as typed, trimmed.
	Ref string
	// Dir is the collection's directory, whether or not it exists yet.
	Dir string
	// Mount is the containment-table mount the last step named, which is
	// where the member kind and the member anchor filename come from.
	Mount Mount
	// Narrow is the item kind a checklist segment selected, empty for every
	// other collection and for the bare checklist segment.
	Narrow string
	// Members are the member directory identifiers, in creation order and
	// after any narrowing, which are the ids the positional selector counts.
	Members []string
	// Holder is the entity the collection hangs from.
	Holder *EntityRef
}

// FirstMember is the reference of the collection's first member, and the
// empty string when the collection holds none. A collection reference plus
// /1 always names its first member, because a position is counted in the
// same creation order this list is in.
func (c *CollectionRef) FirstMember() string {
	if len(c.Members) == 0 {
		return ""
	}
	return c.Ref + "/1"
}

// Refuse is the refusal a command that takes one entity raises when its
// reference names a whole collection.
//
// The named values are written out at each of the two raise sites rather than
// built into one map that is filled conditionally, because the profile guard
// holding a refusal's sentences against the values they carry reads the
// composite literal written at the raise site.
func (c *CollectionRef) Refuse() error {
	count := strconv.Itoa(len(c.Members))
	if len(c.Members) == 0 {
		return contract.RefuseWith(contract.IsACollection, c.Ref, map[string]string{"count": count})
	}
	return contract.RefuseWith(contract.IsACollection, c.Ref, map[string]string{"count": count, "member": c.FirstMember()})
}

// ResolveReference resolves any reference the grammar admits and answers
// which of the two things it names: one entity, or a whole collection.
// Exactly one of the two is non-nil whenever the error is nil.
//
// It accepts every reference ResolvePath accepts but one, so a reference a
// walk prints names the same thing to every command that takes one. The
// exception is an attachment's payload: that file carries no anchor, so it
// names no entity of the format, and ResolvePath answers it with a path where
// this refuses it. ResolveEntity is the reading of this that takes the entity
// and refuses the collection.
//
// An answer of kind card always carries the card, and an answer below a card
// always carries the card it belongs to. Callers read Card without asking, and
// the ones that ask read a nil as the entity belonging to no card at all: the
// event a write records goes to the bench journal and the lock it takes is the
// bench's. A half-filled answer therefore does not degrade, it misreports, so
// the last guard below refuses rather than returning one.
func (b *Bench) ResolveReference(ref string) (*EntityRef, *CollectionRef, error) {
	ref = strings.TrimSpace(ref)
	// The empty reference is this resolver's own case and IsWorkbenchRef does
	// not carry it, because ResolvePath refuses it. See IsWorkbenchRef.
	if ref == "" || IsWorkbenchRef(ref) {
		return &EntityRef{Kind: KindWorkbench, Dir: b.Root}, nil, nil
	}
	// A workstream names its own kind in the grammar, per WorkstreamRefPrefix,
	// so it is tried before the columns and the cards rather than falling
	// through to them: a bare workstream reference would otherwise be
	// shadowed by a column or a card sharing its name.
	if entity, named, err := b.resolveWorkstreamRef(ref); named {
		return entity, nil, err
	}
	head, rest, _ := strings.Cut(ref, "/")
	if rest == "" {
		if column := b.ColumnByRef(ref); column != nil {
			return &EntityRef{
				Kind: KindColumn,
				Dir:  b.ColumnDir(column.ID),
				ID:   column.ID,
				Ref:  column.Ref(),
			}, nil, nil
		}
		found, err := b.ResolveCard(head)
		if err != nil {
			return nil, nil, b.orAWorkstreamNamedBarely(ref, err)
		}
		return &EntityRef{Kind: KindCard, Dir: found.Card.Dir, ID: found.Card.ID, Ref: found.Card.Ref(b.Slug), Card: found.Card}, nil, nil
	}
	landed := &landing{}
	path, card, err := b.resolveBelowLanding(ref, landed)
	if err != nil {
		return nil, nil, err
	}
	if landed.collection {
		collection, err := b.collectionAt(ref, landed)
		if err != nil {
			return nil, nil, err
		}
		return nil, collection, nil
	}
	kind, named := KindOfAnchor(filepath.Base(path))
	if !named {
		return nil, nil, contract.Refuse(contract.UnknownPath, rest)
	}
	// No reference reaches this guard today, because descend refuses a
	// collection whose kind is addressed in its own right before anything
	// half-filled is built, so deleting it reddens no test. It stays because
	// the invariant belongs on this function rather than in the caller that
	// happens to enforce it, and a reader meeting it here is told what every
	// caller of ResolveEntity may assume.
	if kind == KindCard && card == nil {
		return nil, nil, contract.Refuse(contract.UnknownCard, ref)
	}
	dir := filepath.Dir(path)
	headKind, headRef, headDir := KindWorkbench, b.Slug, b.Root
	if card != nil {
		headKind, headRef, headDir = KindCard, card.Ref(b.Slug), card.Dir
	} else if !IsWorkbenchRef(head) && head != b.Slug {
		if column := b.ColumnByRef(head); column != nil {
			headKind, headRef = KindColumn, column.Ref()
			headDir = filepath.Join(b.Root, ColumnsDir, column.ID)
		}
	}
	return &EntityRef{
		Kind: kind,
		Dir:  dir,
		ID:   filepath.Base(dir),
		Ref:  b.refBelowHead(headKind, headRef, headDir, dir),
		Card: card,
	}, nil, nil
}

// orAWorkstreamNamedBarely carries the workstream a bare reference names onto
// an unknown-card refusal, so the sentence can send the reader to the spelling
// the grammar wants instead of to the card listing.
//
// A bare slug is what the retired kind-prefixed commands took, so it is the
// spelling a reader who learned the tool before those went away still types.
// Nothing about it is a card, and the card listing the usual next step points
// at cannot answer the question, so the refusal names the prefixed form and
// the guide that spells the grammar out.
//
// The workstream is looked for only after the card lookup has already failed,
// which is what stops a workstream shadowing a card that shares its name. The
// refusal name does not change: no card was found, which is what happened.
func (b *Bench) orAWorkstreamNamedBarely(ref string, err error) error {
	refusal, isRefusal := err.(*contract.Refusal)
	if !isRefusal || refusal.Name != contract.UnknownCard {
		return err
	}
	workstream := b.WorkstreamByRef(ref)
	if workstream == nil {
		return err
	}
	return contract.RefuseWith(contract.UnknownCard, refusal.Detail, map[string]string{"workstream": workstream.Ref()})
}

// collectionAt builds the answer for a reference the walk stopped on a
// collection with. The members are the walk's own list, narrowed the way the
// walk narrows before it counts a position, so the position this resolver
// answers and the position a screen prints are one number.
func (b *Bench) collectionAt(ref string, landed *landing) (*CollectionRef, error) {
	ref = strings.TrimSpace(ref)
	members := MemberIDs(landed.dir, landed.mount)
	if landed.narrow != "" {
		members = filterByKind(landed.dir, landed.mount.Anchor, members, landed.narrow)
	}
	holder, err := b.collectionHolder(ref)
	if err != nil {
		return nil, err
	}
	return &CollectionRef{
		Ref:     ref,
		Dir:     landed.dir,
		Mount:   landed.mount,
		Narrow:  landed.narrow,
		Members: members,
		Holder:  holder,
	}, nil
}

// collectionHolder is the entity a collection hangs from, which is the
// reference minus its last segment.
//
// ResolveEntity answers every spelling of that but one. A bare slug names no
// entity to it and is a legal head below the workbench, so that case is
// answered here with what ResolveEntity answers for the workbench itself,
// including the empty Ref: the workbench's own printed spelling is a question
// this resolver does not settle, and the two consumers that need one already
// carry their own rule for it.
func (b *Bench) collectionHolder(ref string) (*EntityRef, error) {
	holder := ref[:strings.LastIndex(ref, "/")]
	if b.Slug != "" && holder == b.Slug {
		return &EntityRef{Kind: KindWorkbench, Dir: b.Root}, nil
	}
	return b.ResolveEntity(holder)
}

// resolveBelow resolves a reference to the file it names, and to the card that
// file belongs to when it belongs to one. ResolvePath reads it, and answers a
// reference naming a whole collection where ResolveEntity refuses one.
//
// The head segment names where the walk starts and the rest descends through
// the containment grammar. A column is an entity of the workbench and the
// containment walk draws one, so the reference a walk prints for it opens the
// column the way every other reference opens what it names. The slug heads a
// path below the workbench without naming the workbench itself, which is the
// form the walk prints for a workbench attachment; whether the bare slug also
// opens the workbench is a separate question and this does not answer it.
func (b *Bench) resolveBelow(ref string) (string, *Card, error) {
	return b.resolveBelowLanding(ref, nil)
}

// resolveBelowLanding is resolveBelow with the walk's landing reported. A
// caller that needs to know whether the reference stopped on a collection
// passes one to write into; resolveBelow passes nil, which is every caller
// that only wants the path.
func (b *Bench) resolveBelowLanding(ref string, landed *landing) (string, *Card, error) {
	head, rest, _ := strings.Cut(strings.TrimSpace(ref), "/")
	if IsWorkbenchRef(head) || (rest != "" && b.Slug != "" && head == b.Slug) {
		if rest == "" {
			return filepath.Join(b.Root, WorkbenchAnchor), nil, nil
		}
		path, err := descend(b.Root, KindWorkbench, strings.Split(rest, "/"), nil, landed)
		return path, nil, err
	}
	if column := b.ColumnByRef(head); column != nil {
		if rest == "" {
			return b.ColumnAnchorPath(column.ID), nil, nil
		}
		path, err := descend(b.ColumnDir(column.ID), KindColumn, strings.Split(rest, "/"), nil, landed)
		return path, nil, err
	}
	found, err := b.ResolveCard(head)
	if err != nil {
		return "", nil, err
	}
	path, err := walkBelowCard(found.Card, rest, landed)
	return path, found.Card, err
}

// walkBelowCard resolves the segments below a card. An empty rest is the
// card's own anchor, which is what makes `path <card>` open the card.
func walkBelowCard(card *Card, rest string, landed *landing) (string, error) {
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
		return descend(card.Dir, KindCard, aliased, &kind, landed)
	}
	return descend(card.Dir, KindCard, segments, nil, landed)
}

// checklistMount is the collection a checklist segment such as questions
// narrows, read off the containment grammar so the segments follow the table
// rather than a second statement of where a checklist lives.
func checklistMount() (Mount, bool) {
	for _, mount := range Contains(KindCard) {
		if mount.Kind == KindItem {
			return mount, true
		}
	}
	return Mount{}, false
}

// landing is what the walk reports about where it stopped, filled only by
// the branch that stops on a collection. A caller wanting the answer passes
// a landing to write into; every other caller passes nil.
type landing struct {
	collection bool
	dir        string
	mount      Mount
	narrow     string
}

// MemberIDs are one collection's live members, in the creation order a
// positional reference counts in.
//
// The resolver counts a position in this list and the containment walk draws
// its rows from it, so the two read one statement of the order rather than
// two statements that agree today.
func MemberIDs(collection string, mount Mount) []string {
	return SortByOrdinal(collection, mount.Anchor, ListIDs(collection))
}

// descend resolves the segments below one entity by walking the containment
// grammar a collection at a time. A pair of segments names a collection and
// then a member of it, and the member's own kind decides what the pair after
// that may name, so a reference reaches as deep as the grammar goes.
//
// A collection holding a kind that is addressed in its own right is refused,
// so the workbench's cards and columns are reached by the address a person
// types for them and by nothing else. See addressedInItsOwnRight.
//
// A segment the grammar does not know is refused rather than dropped. The
// resolver used to read the first collection and discard everything past the
// entity it found, which made `<card>/comments/1/attachments/1` open the
// comment: an address the containment walk prints and a different file behind
// it, with nothing said.
//
// A kind narrows the collection's members first, which is what a checklist
// segment such as questions selects on. Position counts in creation order rather than
// in the listing's ascending-hex order, so `<card>/comment/2` names the second
// comment somebody wrote and keeps naming it however the identifiers happened
// to fall.
func descend(dir, kind string, segments []string, narrow *string, landed *landing) (string, error) {
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
		// A collection the containment table declares for this kind is
		// there whether or not anything has been written into it, so the
		// walk answers with the directory a first member would be written
		// into rather than refusing over a directory nobody has made yet.
		// ListIDs reads an absent directory as an empty one, so a
		// positional selector below this point still refuses through pick.
		if landed != nil {
			landed.collection = true
			landed.dir = collection
			landed.mount = mount
			landed.narrow = ""
			if narrow != nil {
				landed.narrow = *narrow
			}
		}
		return collection, nil
	}
	ids := MemberIDs(collection, mount)
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
	return descend(member, mount.Kind, below, nil, landed)
}

// addressedInItsOwnRight reports whether a kind is one a person names directly
// rather than by its position in the collection that holds it. A card is named
// by its reference and a column by its slug, and each of those addresses is the
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
	return kind == KindCard || kind == KindColumn
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
// kind, which is how the checklist segments select one of the three.
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
// one entity refuses ambiguous-name and carries the position of every match
// alongside the selector, so the caller retries with attachments/<n> and the
// number it retries with is one this same function's position arm answers.
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
			return ids[matches[0]], nil
		case 0:
			return "", contract.Refuse(contract.UnknownPath, selector)
		default:
			ordinals := make([]string, 0, len(matches))
			for _, index := range matches {
				ordinals = append(ordinals, strconv.Itoa(index+1))
			}
			return "", contract.RefuseWith(contract.AmbiguousName, selector, map[string]string{
				"selector": selector,
				"ordinals": strings.Join(ordinals, ","),
			})
		}
	}
	return "", contract.Refuse(contract.UnknownPath, selector)
}

// matchByName returns the positions within ids of the entities whose anchor
// declares the collection's name field with the selector as its value.
//
// It answers positions rather than identifiers because a position is what the
// refusal has to report: the sentence tells the caller to retry as
// attachments/<n>, and the arm that answers that retry is the one above, which
// indexes this same ids sequence. Reading the anchor's stored ordinal instead
// names a number the retry cannot resolve, since an unstamped anchor carries
// no ordinal at all and a gapped collection carries ordinals that have drifted
// off their positions. The caller reaches the identifier of a single match as
// ids[position].
func matchByName(collection string, mount Mount, ids []string, selector string) []int {
	var matches []int
	for index, id := range ids {
		fm, _ := loadAnchor(filepath.Join(collection, id, mount.Anchor))
		if fm.Value(mount.NameField) == selector {
			matches = append(matches, index)
		}
	}
	return matches
}

// ResolveLinkTarget turns what a caller typed for a link's target into the
// 12-hex identifier the anchor stores, and refuses unknown-card when the
// reference names no card either half of the collection carries.
//
// It is the one write-time resolution that reads both halves. Every other
// write verb resolves a reference that has to be live, since nothing comments
// on, attaches to or claims an archived card, but a link may legally name a
// card that has since been archived: the format fixes the identifier space as
// spanning both halves, which is the same scope HasIdentifier and the
// dangling-link check already read.
//
// An identifier is checked for presence rather than resolved, so a link may
// name an archived card by its identifier without the archived anchors being
// read at all. A human reference has to be resolved, so the live half is
// tried first and the archive only after it fails, which is the order
// ResolveArchivedCard's own comment fixes.
func (b *Bench) ResolveLinkTarget(raw string) (string, *contract.Refusal) {
	raw = strings.TrimSpace(raw)
	if IsID(raw) {
		if !b.HasIdentifier(raw) {
			return "", contract.Refuse(contract.UnknownCard, raw)
		}
		return raw, nil
	}
	if found, err := b.ResolveCard(raw); err == nil {
		return found.Card.ID, nil
	}
	if found, err := b.ResolveArchivedCard(raw); err == nil {
		return found.Card.ID, nil
	}
	return "", contract.Refuse(contract.UnknownCard, raw)
}
