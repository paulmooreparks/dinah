package bench

import (
	"errors"
	"fmt"
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
		card, err := b.LoadCardIn(root, ref)
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
	claimants := b.Numbers.ByNumber[number]
	switch len(claimants) {
	case 0:
		return nil, contract.Refuse(contract.UnknownCard, ref)
	case 1:
		card, err := b.LoadCardIn(root, claimants[0])
		if err != nil {
			// A claimant whose directory is not in this root is the card
			// standing in the other half of the collection, and an anchor
			// that will not read is the reader's own refusal, so both are
			// answered on the identifier branch's terms: the reader's
			// refusal travels, and everything else says the reference
			// resolves to nothing here, which preserves the split between
			// ResolveCard and ResolveArchivedCard.
			var refusal *contract.Refusal
			if errors.As(err, &refusal) && refusal.Name != contract.UnknownCard {
				return nil, err
			}
			return nil, contract.Refuse(contract.UnknownCard, ref)
		}
		found := &Resolved{Card: card}
		if prefix != "" && prefix != b.Slug {
			found.StalePrefix = prefix
		}
		return found, nil
	}
	// More than one identifier claims the number, which is the state a
	// hand-edited registry or a union-merged clone produces, so the resolution
	// refuses rather than answering with the first claimant it met. The order
	// is the registry's own, which is file order at RegistryFormat and, below
	// it, the live half before the archived one with ascending identifiers
	// inside each. Every claimant is named and none is dropped, because a
	// truncation would hide the candidate the caller was reaching for.
	return nil, contract.RefuseWith(contract.AmbiguousCard, ref, map[string]string{
		"cards": strings.Join(claimants, "\n"),
	})
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
// what `show` walks for the composed form, and what ResolveEditTarget
// reaches, both for the empty reference it sends here ahead of anything
// else and for every reference the entity resolver does not answer.
//
// A workstream resolves to its directory rather than to its anchor, because
// the reference names the entity and `path` hands a shell a filesystem
// address to work from. The directory holds the anchor workstream.md, which
// carries the workstream's notes, and the machine-written journal.ndjson.
// ResolveEditTarget answers that anchor rather than this directory, because
// an editor is handed a file. A workstream is tried first, per
// WorkstreamRefPrefix and the reasoning on resolveWorkstreamRef, before the
// rest of the grammar gets a chance to shadow it.
func (b *Bench) ResolvePath(ref string) (string, error) {
	return b.ResolvePathIn(LiveHalf, ref)
}

// ResolvePathIn is ResolvePath reading the half a caller names. Under
// ArchivedHalf it refuses NotArchived for a reference the mirror does not
// hold, which is the refusal notArchivedFor decides.
//
// The refusal is asked for twice, once before the walk and once on the walk's
// failure, because neither walk fails on the workbench: resolveBelowLanding
// answers a bare workbench head with the live anchor and a success, so a
// refusal left to the failure path alone would never reach the one reference
// the archive can never hold.
func (b *Bench) ResolvePathIn(half ResolutionHalf, ref string) (string, error) {
	if half == ArchivedHalf {
		if refusal := b.notArchivedFor(ref, nil); refusal != nil {
			return "", refusal
		}
	}
	path, err := b.resolvePathBody(half, ref)
	if err != nil {
		if half == ArchivedHalf {
			err = b.notArchivedFor(ref, err)
		}
		return "", err
	}
	return filepath.Abs(path)
}

// resolvePathBody is the walk ResolvePathIn wraps, without the refusal and
// without the absolute-path step, so the two calls to notArchivedFor sit in
// one place rather than at each of this function's own returns.
func (b *Bench) resolvePathBody(half ResolutionHalf, ref string) (string, error) {
	if rest, named := strings.CutPrefix(strings.TrimSpace(ref), WorkstreamRefPrefix); named {
		// The whole reference goes to the resolver rather than the
		// remainder, because that resolver strips the prefix itself so that
		// the workstream-taking commands accept either spelling. Passing the
		// remainder would strip a second time and admit a doubled prefix.
		workstream, err := b.workstreamByRefIn(half, strings.TrimSpace(ref))
		if err != nil {
			return "", err
		}
		if workstream == nil {
			return "", contract.Refuse(contract.UnknownWorkstream, rest)
		}
		return workstream.Dir, nil
	}
	path, _, err := b.resolveBelowLanding(half, ref, nil)
	if err != nil {
		return "", err
	}
	return path, nil
}

// ResolveEditTarget is the file `edit` opens for a reference, and it is the
// whole of that command's reference policy in one place. An entity is
// answered with its anchor, which is what makes edit open a workstream's
// workstream.md rather than the directory holding it. A whole collection is
// refused with the refusal CollectionRef.Refuse composes: a collection
// directory is not a file an editor can open, and handing it over is what
// this command did with one until dinah-455. Everything else falls through
// to ResolvePath, which answers exactly two further references, both of them
// files: a card's journal and an attachment's payload.
//
// The empty reference is answered by ResolvePath rather than by the entity
// resolver, and that ordering is the whole of arm 1. ResolveReference reads
// an empty reference as the workbench, so asking it first would make a bare
// `dinah edit` open workbench.md; IsWorkbenchRef's own comment declares that
// edit refuses it, because edit declares its argument required and somebody
// who typed no argument forgot it rather than meaning the workbench. The
// refusal is delegated rather than restated, so the sentence a reader gets
// cannot drift from the one `dinah path` raises.
//
// The entity resolver's own error is discarded rather than reported, so
// every reference that refuses today refuses the same way: workstream/nosuch
// keeps its UnknownWorkstream refusal from ResolvePath rather than becoming
// an unknown card.
func (b *Bench) ResolveEditTarget(ref string) (string, error) {
	// The trim is what sends a space-only or tab-only argument down the same
	// arm as an argument nobody typed, since both resolvers trim inside
	// themselves rather than at the command.
	if strings.TrimSpace(ref) == "" {
		return b.ResolvePath(ref)
	}
	entity, collection, err := b.ResolveReference(ref)
	if err != nil {
		return b.ResolvePath(ref)
	}
	if collection != nil {
		return "", collection.Refuse()
	}
	anchor, declared := AnchorPathOf(entity)
	if !declared {
		// No reference a reader can type reaches this, because every kind
		// ResolveReference answers declares an anchor. It is a defect in
		// this build rather than a mistake by the reader, so it takes the
		// route reportError prints as unreachable rather than minting a
		// refusal name for a case nobody can produce.
		return "", fmt.Errorf("the kind %q declares no anchor, so %q names nothing to open", entity.Kind, ref)
	}
	return filepath.Abs(anchor)
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
	// Members are the member directory identifiers of the half this answer
	// was resolved in, in creation order and after any narrowing, which are
	// the ids the positional selector counts.
	Members []string
	// Holder is the entity the collection hangs from.
	Holder *EntityRef
	// Archived reports whether the members listed here came out of the
	// archive mirror rather than out of the live half.
	Archived bool
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
// It accepts every reference ResolvePath accepts but two, so a reference a
// walk prints names the same thing to every command that takes one. The
// exceptions are an attachment's payload and a card's journal: neither file
// carries an anchor, so neither names an entity of the format, and
// ResolvePath answers both with a path where this refuses them.
// ResolveEntity is the reading of this that takes the entity and refuses the
// collection.
//
// An answer of kind card always carries the card, and an answer below a card
// always carries the card it belongs to. Callers read Card without asking, and
// the ones that ask read a nil as the entity belonging to no card at all: the
// event a write records goes to the bench journal and the lock it takes is the
// bench's. A half-filled answer therefore does not degrade, it misreports, so
// the last guard below refuses rather than returning one.
func (b *Bench) ResolveReference(ref string) (*EntityRef, *CollectionRef, error) {
	return b.ResolveReferenceIn(LiveHalf, ref)
}

// ResolveReferenceIn is ResolveReference reading the half a caller names. It
// carries the same two calls to notArchivedFor ResolvePathIn carries, and for
// the same reason: the pre-walk call is what catches the workbench, which the
// branch below answers successfully rather than failing on.
//
// Two entry points raise this refusal rather than one, because neither of the
// two can be written in terms of the other. ResolvePath answers an
// attachment's payload, which this resolver refuses, and this resolver answers
// a collection separately, which ResolvePath answers as a directory path. The
// rule is in one place; what is in two places is the call to it.
func (b *Bench) ResolveReferenceIn(half ResolutionHalf, ref string) (*EntityRef, *CollectionRef, error) {
	if half == ArchivedHalf {
		if refusal := b.notArchivedFor(ref, nil); refusal != nil {
			return nil, nil, refusal
		}
	}
	entity, collection, err := b.resolveReferenceBody(half, ref)
	if err != nil {
		if half == ArchivedHalf {
			err = b.notArchivedFor(ref, err)
		}
		return nil, nil, err
	}
	return entity, collection, nil
}

// resolveReferenceBody is the walk ResolveReferenceIn wraps, without the
// refusal, so the two calls to notArchivedFor sit in one place rather than at
// each of this function's own returns.
func (b *Bench) resolveReferenceBody(half ResolutionHalf, ref string) (*EntityRef, *CollectionRef, error) {
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
	if entity, named, err := b.resolveWorkstreamRef(half, ref); named {
		return entity, nil, err
	}
	head, rest, _ := strings.Cut(ref, "/")
	// A bare head is always the reference's deepest collection step, so it
	// takes the caller's half straight through rather than asking headHalf.
	if rest == "" {
		column, err := b.columnByRefIn(half, ref)
		if err != nil {
			return nil, nil, err
		}
		if column != nil {
			return &EntityRef{
				Kind:     KindColumn,
				Dir:      b.columnDirIn(half, column.ID),
				ID:       column.ID,
				Ref:      column.Ref(),
				Archived: half == ArchivedHalf,
			}, nil, nil
		}
		found, err := b.resolveCardIn(b.cardsRootIn(half), head)
		if err != nil {
			return nil, nil, b.orAWorkstreamNamedBarely(ref, err)
		}
		return &EntityRef{Kind: KindCard, Dir: found.Card.Dir, ID: found.Card.ID, Ref: found.Card.Ref(b.Slug), Card: found.Card, Archived: half == ArchivedHalf}, nil, nil
	}
	landed := &landing{}
	path, card, err := b.resolveBelowLanding(half, ref, landed)
	if err != nil {
		return nil, nil, err
	}
	if landed.collection {
		collection, err := b.collectionAt(half, ref, landed)
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
		column, err := b.columnByRefIn(headHalf(half, rest), head)
		if err != nil {
			return nil, nil, err
		}
		if column != nil {
			headKind, headRef = KindColumn, column.Ref()
			headDir = b.columnDirIn(headHalf(half, rest), column.ID)
		}
	}
	// The composed reference is hoisted out of the composite literal because
	// a field of one takes a single value, and this call now answers two.
	below, err := b.refBelowHead(half, headKind, headRef, headDir, dir)
	if err != nil {
		return nil, nil, err
	}
	return &EntityRef{
		Kind:     kind,
		Dir:      dir,
		ID:       filepath.Base(dir),
		Ref:      below,
		Card:     card,
		Archived: half == ArchivedHalf,
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
	workstream, streamErr := b.WorkstreamByRef(ref)
	if streamErr != nil || workstream == nil {
		return err
	}
	return contract.RefuseWith(contract.UnknownCard, refusal.Detail, map[string]string{"workstream": workstream.Ref()})
}

// collectionAt builds the answer for a reference the walk stopped on a
// collection with. The members are the walk's own list, narrowed the way the
// walk narrows before it counts a position, so the position this resolver
// answers and the position a screen prints are one number.
func (b *Bench) collectionAt(half ResolutionHalf, ref string, landed *landing) (*CollectionRef, error) {
	ref = strings.TrimSpace(ref)
	members, err := MemberIDs(landed.dir, landed.mount)
	if err != nil {
		return nil, err
	}
	if landed.narrow != "" {
		members = filterByKind(landed.dir, landed.mount.Anchor, members, landed.narrow)
	}
	holder, err := b.collectionHolder(ref)
	if err != nil {
		return nil, err
	}
	return &CollectionRef{
		Ref:      ref,
		Dir:      landed.dir,
		Mount:    landed.mount,
		Narrow:   landed.narrow,
		Members:  members,
		Holder:   holder,
		Archived: half == ArchivedHalf,
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
	return b.resolveBelowLanding(LiveHalf, ref, nil)
}

// resolveBelowLanding is resolveBelow with the walk's landing reported. A
// caller that needs to know whether the reference stopped on a collection
// passes one to write into; resolveBelow passes nil, which is every caller
// that only wants the path.
func (b *Bench) resolveBelowLanding(half ResolutionHalf, ref string, landed *landing) (string, *Card, error) {
	head, rest, _ := strings.Cut(strings.TrimSpace(ref), "/")
	// The head is the reference's deepest collection step only when nothing
	// below it names a collection, so it asks headHalf rather than taking the
	// caller's half. The workbench is never one of the heads this walk
	// resolves under ArchivedHalf, because notArchivedFor refuses it ahead of
	// the walk.
	within := headHalf(half, rest)
	if IsWorkbenchRef(head) || (rest != "" && b.Slug != "" && head == b.Slug) {
		if rest == "" {
			return filepath.Join(b.Root, WorkbenchAnchor), nil, nil
		}
		path, err := descend(b.Root, KindWorkbench, strings.Split(rest, "/"), nil, landed, half)
		return path, nil, err
	}
	withinColumn, err := b.columnByRefIn(within, head)
	if err != nil {
		return "", nil, err
	}
	if column := withinColumn; column != nil {
		dir := b.columnDirIn(within, column.ID)
		if rest == "" {
			return filepath.Join(dir, ColumnAnchor), nil, nil
		}
		path, err := descend(dir, KindColumn, strings.Split(rest, "/"), nil, landed, half)
		return path, nil, err
	}
	found, err := b.resolveCardIn(b.cardsRootIn(within), head)
	if err != nil {
		return "", nil, err
	}
	path, err := walkBelowCard(found.Card, rest, landed, half)
	return path, found.Card, err
}

// walkBelowCard resolves the segments below a card. An empty rest is the
// card's own anchor, which is what makes `path <card>` open the card.
func walkBelowCard(card *Card, rest string, landed *landing, half ResolutionHalf) (string, error) {
	if rest == "" {
		return card.AnchorPath(), nil
	}
	segments := strings.Split(rest, "/")
	head := segments[0]
	// The card's own two files are named by segment rather than by
	// collection, so they are answered ahead of the grammar. Neither is an
	// entity of the containment table: the anchor is the card itself and the
	// journal is content.
	if cardOwnFileSegment(head) {
		if head == CardAnchor || head == KindCard {
			return card.AnchorPath(), nil
		}
		return card.JournalPath(), nil
	}
	if kind, ok := checklistKinds[head]; ok {
		items, ok := checklistMount()
		if !ok {
			return "", contract.Refuse(contract.UnknownPath, rest)
		}
		aliased := append([]string{items.Dir}, segments[1:]...)
		return descend(card.Dir, KindCard, aliased, &kind, landed, half)
	}
	return descend(card.Dir, KindCard, segments, nil, landed, half)
}

// cardOwnFileSegment reports whether a segment below a card names one of the
// card's own two files rather than a collection. walkBelowCard answers those
// segments ahead of the containment grammar, and the archived-half resolution
// asks the same question to decide whether the head is the reference's
// deepest collection step, so the set is declared once rather than written
// out in both places.
func cardOwnFileSegment(segment string) bool {
	return segment == CardAnchor || segment == KindCard || segment == "journal" || segment == JournalName
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
func MemberIDs(collection string, mount Mount) ([]string, error) {
	ids, err := ListIDs(collection)
	if err != nil {
		return nil, err
	}
	return SortByOrdinal(collection, mount.Anchor, ids), nil
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
// segment such as questions selects on. Position counts in creation order rather than
// in the listing's ascending-hex order, so `<card>/comment/2` names the second
// comment somebody wrote and keeps naming it however the identifiers happened
// to fall.
func descend(dir, kind string, segments []string, narrow *string, landed *landing, half ResolutionHalf) (string, error) {
	mount, ok := MountOf(kind, segments[0])
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
	// before the collection path is joined. Written in terms of tail and
	// below it would decide after the join has already chosen the live
	// directory and after the member listing has been read out of it, which
	// compiles and reads the live members in silence.
	deepest := len(segments) == 1 ||
		len(segments) == 2 ||
		(mount.Kind == KindAttachment && len(segments) > 2 && segments[2] == PayloadDir)
	collection := filepath.Join(dir, mount.Dir)
	if deepest && half == ArchivedHalf {
		collection = filepath.Join(dir, ArchiveDir, mount.Dir)
	}
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
	ids, err := MemberIDs(collection, mount)
	if err != nil {
		return "", err
	}
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
	return descend(member, mount.Kind, below, nil, landed, half)
}

// AddressedInItsOwnRight reports whether a kind is one a person names directly
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
func AddressedInItsOwnRight(kind string) bool {
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
	// An ambiguity is carried out of whichever half raised it rather than
	// being discarded with every other error. Without the two unwraps, a
	// number two live cards carry falls past the refusal and, where the
	// archive holds a single card on that number, records the link against
	// the archived card, which is the silent wrong answer this refusal
	// exists to stop.
	found, err := b.ResolveCard(raw)
	if err == nil {
		return found.Card.ID, nil
	}
	var refusal *contract.Refusal
	if errors.As(err, &refusal) && refusal.Name == contract.AmbiguousCard {
		return "", refusal
	}
	found, err = b.ResolveArchivedCard(raw)
	if err == nil {
		return found.Card.ID, nil
	}
	if errors.As(err, &refusal) && refusal.Name == contract.AmbiguousCard {
		return "", refusal
	}
	return "", contract.Refuse(contract.UnknownCard, raw)
}

// ResolutionHalf names which half of a collection a resolution reads at a
// reference's deepest collection step.
type ResolutionHalf int

const (
	// LiveHalf reads the live half, which is every resolution the tool
	// performed before restore existed.
	LiveHalf ResolutionHalf = iota
	// ArchivedHalf reads the archive mirror at the reference's deepest
	// collection step, and the live half at every step above it.
	ArchivedHalf
)

// headHalf is the half a reference's head segment resolves in. The head is
// the reference's deepest collection step exactly when nothing below it names
// a collection, which is a bare head and a head followed only by one of the
// card's own file segments. Every other reference resolves its head live and
// carries the archived half down to the step that uses it.
//
// The head is resolved twice over, once in ResolveReferenceIn's bare-head
// branch and once in resolveBelowLanding, and neither is written in terms of
// the other, so the question of which half it resolves in is one declared
// function rather than a condition written out in both places.
func headHalf(half ResolutionHalf, rest string) ResolutionHalf {
	if half == LiveHalf || rest == "" {
		return half
	}
	segment, below, _ := strings.Cut(rest, "/")
	if below == "" && cardOwnFileSegment(segment) {
		return ArchivedHalf
	}
	return LiveHalf
}

// cardsRootIn is the cards collection of one half.
func (b *Bench) cardsRootIn(half ResolutionHalf) string {
	if half == ArchivedHalf {
		return b.ArchivedCardsRoot()
	}
	return b.CardsRoot()
}

// columnsRootIn is the columns collection of one half.
func (b *Bench) columnsRootIn(half ResolutionHalf) string {
	if half == ArchivedHalf {
		return b.ArchivedColumnsRoot()
	}
	return filepath.Join(b.Root, ColumnsDir)
}

// columnDirIn is one column's directory in one half.
func (b *Bench) columnDirIn(half ResolutionHalf, id string) string {
	return filepath.Join(b.columnsRootIn(half), id)
}

// workstreamsRootIn is the workstreams collection of one half.
func (b *Bench) workstreamsRootIn(half ResolutionHalf) string {
	if half == ArchivedHalf {
		return b.ArchivedWorkstreamsRoot()
	}
	return b.WorkstreamsRoot()
}

// columnByRefIn is the column a reference names in one half. The live form is
// ColumnByRef, which reads the workbench's own ordered list, and that list
// cannot hold an archived column, so the archived form loads the anchors under
// the mirror instead.
func (b *Bench) columnByRefIn(half ResolutionHalf, ref string) (*Column, error) {
	if half == ArchivedHalf {
		return b.ArchivedColumnByRef(ref)
	}
	return b.ColumnByRef(ref), nil
}

// ArchivedColumnByRef finds a column in the archive mirror, by the grammar
// ColumnByRef accepts: the identifier first, then the slug, then the title,
// the last two compared without regard to ASCII case.
//
// The order is ColumnByRef's own, and the reason it records applies here
// unchanged: a reference matching one column's slug and another column's
// title resolves to the column whose slug it is.
func (b *Bench) ArchivedColumnByRef(ref string) (*Column, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil
	}
	root := filepath.Join(b.Root, ArchiveDir)
	ids, err := ListIDs(b.ArchivedColumnsRoot())
	if err != nil {
		return nil, err
	}
	var columns []*Column
	for n, id := range ids {
		// A directory the reader refuses is skipped rather than refused
		// over, which is what leaves the rest of the mirror reachable when
		// one anchor in it is damaged.
		column, err := readColumnIn(root, currentVocabulary, id, n+1)
		if err != nil {
			continue
		}
		columns = append(columns, column)
	}
	want := asciiLower(ref)
	for _, column := range columns {
		if column.ID == ref {
			return column, nil
		}
	}
	for _, column := range columns {
		if column.Slug != "" && asciiLower(column.Slug) == want {
			return column, nil
		}
	}
	for _, column := range columns {
		if asciiLower(column.Title) == want {
			return column, nil
		}
	}
	return nil, nil
}

// workstreamByRefIn is the workstream a reference names in one half. The live
// form is WorkstreamByRef, which spans both halves on purpose, because a
// card's membership has to resolve whichever half its workstream is in.
func (b *Bench) workstreamByRefIn(half ResolutionHalf, ref string) (*Workstream, error) {
	if half != ArchivedHalf {
		return b.WorkstreamByRef(ref)
	}
	handle := strings.TrimPrefix(strings.TrimSpace(ref), WorkstreamRefPrefix)
	if handle == "" {
		return nil, nil
	}
	archived, err := workstreamsIn(b.workstreamsRootIn(ArchivedHalf))
	if err != nil {
		return nil, err
	}
	for _, workstream := range archived {
		if workstream.ID == handle {
			return workstream, nil
		}
	}
	want := asciiLower(handle)
	for _, workstream := range archived {
		if workstream.Slug != "" && asciiLower(workstream.Slug) == want {
			return workstream, nil
		}
	}
	return nil, nil
}

// notArchivedFor is the only place NotArchived is raised. Both archived-half
// entry points call it twice: once before the walk, passing a nil failure,
// and once on the walk's failure, passing the error it failed with.
//
// The pre-walk call exists because neither walk fails on the workbench.
// ResolveReferenceIn answers the workbench at its own top and
// resolveBelowLanding answers a bare workbench head with the live anchor and
// a success, so a refusal left to the failure path would never be reached for
// the one reference the archive can never hold.
//
// A pre-walk call answers nil for every reference but the workbench's. A
// failure call answers either NotArchived or the failure unchanged, and never
// nil.
func (b *Bench) notArchivedFor(ref string, failure error) error {
	trimmed := strings.TrimSpace(ref)
	if failure == nil {
		head, rest, _ := strings.Cut(trimmed, "/")
		if trimmed != "" && !(IsWorkbenchRef(head) && rest == "") {
			return nil
		}
		detail := trimmed
		if detail == "" {
			detail = WorkbenchRef
		}
		return contract.RefuseWith(contract.NotArchived, detail, map[string]string{"slug": b.Slug})
	}
	holder, collection, found, probeErr := b.probe(trimmed)
	if probeErr != nil {
		return probeErr
	}
	if !found {
		// Nothing in either half answers to the reference, so the mirror's
		// own error travels unchanged and the reader goes on getting the
		// sentence they get today.
		return failure
	}
	values := map[string]string{}
	switch {
	case holder != "":
		values["holder"] = holder
	case collection != "":
		values["collection"] = collection
	}
	return contract.RefuseWith(contract.NotArchived, trimmed, values)
}

// probe is the refusal probe: one diagnostic walk over a reference the
// archived half failed on, reading the live half first and the archive mirror
// second at every collection step and taking the first that holds the member
// the segment names.
//
// It answers the reference truncated to the member selected at the first step
// it read out of the mirror, the reference of the collection whose member the
// deepest step selected, and whether it reached anything at all. It never
// answers an *EntityRef and notArchivedFor never answers anything but an
// error, so nothing this walk reaches can become an answer to a caller on
// either entry point. Section 3's answer rule still looks in exactly one
// place; a second look is affordable here because the whole output is which
// of two refusal names to print.
//
// Live first rather than mirror first is deliberate. This order decides only
// which of two true sentences a refused reader gets, and live first makes the
// live case win any tie, so the advice never tells a reader to restore
// something standing in front of them.
func (b *Bench) probe(ref string) (string, string, bool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", false, nil
	}
	if strings.HasPrefix(ref, WorkstreamRefPrefix) {
		// WorkstreamByRef spans both halves, and a bare workstream head is
		// its own deepest step, so an archived one resolved rather than
		// failing and only the live case can reach here.
		workstream, err := b.WorkstreamByRef(ref)
		if err != nil {
			return "", "", false, err
		}
		return "", "", workstream != nil, nil
	}
	head, rest, _ := strings.Cut(ref, "/")
	if IsWorkbenchRef(head) || (rest != "" && b.Slug != "" && head == b.Slug) {
		if rest == "" {
			return "", "", true, nil
		}
		segments := strings.Split(rest, "/")
		return b.probeBelow(b.Root, KindWorkbench, head, "", segments, segments, nil)
	}
	if column := b.ColumnByRef(head); column != nil {
		if rest == "" {
			return "", "", true, nil
		}
		segments := strings.Split(rest, "/")
		return b.probeBelow(b.ColumnDir(column.ID), KindColumn, head, "", segments, segments, nil)
	}
	archivedColumn, err := b.ArchivedColumnByRef(head)
	if err != nil {
		return "", "", false, err
	}
	if column := archivedColumn; column != nil {
		if rest == "" {
			return head, "", true, nil
		}
		segments := strings.Split(rest, "/")
		return b.probeBelow(b.columnDirIn(ArchivedHalf, column.ID), KindColumn, head, head, segments, segments, nil)
	}
	var card *Card
	holder := ""
	if found, err := b.resolveCardIn(b.CardsRoot(), head); err == nil {
		card = found.Card
	} else if found, err := b.resolveCardIn(b.ArchivedCardsRoot(), head); err == nil {
		card, holder = found.Card, head
	} else {
		return "", "", false, nil
	}
	if rest == "" {
		return holder, "", true, nil
	}
	segments := strings.Split(rest, "/")
	if cardOwnFileSegment(segments[0]) {
		return holder, "", true, nil
	}
	walk := segments
	var narrow *string
	if kind, ok := checklistKinds[segments[0]]; ok {
		items, ok := checklistMount()
		if !ok {
			return "", "", false, nil
		}
		walk = append([]string{items.Dir}, segments[1:]...)
		narrow = &kind
	}
	return b.probeBelow(card.Dir, KindCard, head, holder, segments, walk, narrow)
}

// probeBelow is the probe's walk below one entity. typed carries the segments
// the reader wrote, which is what the holder and collection references are
// composed from, and walk carries the same segments with a checklist word
// aliased onto the collection it narrows, which is what the containment
// grammar is walked with. The two are the same length, so one indexes the
// other.
func (b *Bench) probeBelow(dir, kind, ref, holder string, typed, walk []string, narrow *string) (string, string, bool, error) {
	mount, ok := MountOf(kind, walk[0])
	if !ok {
		return "", "", false, nil
	}
	if len(walk) == 1 {
		// The reference ends on the collection itself, which a walk answers
		// in either half whether or not anything was written into it.
		return holder, "", true, nil
	}
	for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
		collection := filepath.Join(dir, mount.Dir)
		if half == ArchivedHalf {
			collection = filepath.Join(dir, ArchiveDir, mount.Dir)
		}
		members, err := MemberIDs(collection, mount)
		if err != nil {
			return "", "", false, err
		}
		if narrow != nil {
			members = filterByKind(collection, mount.Anchor, members, *narrow)
		}
		id, err := pick(collection, mount, members, walk[1])
		if err != nil {
			continue
		}
		below := ref + "/" + typed[0] + "/" + typed[1]
		if half == ArchivedHalf && holder == "" {
			holder = below
		}
		if len(walk) == 2 {
			return holder, ref + "/" + typed[0], true, nil
		}
		member := filepath.Join(collection, id)
		if mount.Kind == KindAttachment && walk[2] == PayloadDir {
			if len(walk) > 3 {
				return "", "", false, nil
			}
			if _, err := payloadOf(member); err != nil {
				return "", "", false, nil
			}
			return holder, "", true, nil
		}
		return b.probeBelow(member, mount.Kind, below, holder, typed[2:], walk[2:], nil)
	}
	return "", "", false, nil
}
