package bench

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// Comment is one comment of a card: an entity like every other, ordered by
// the creation ordinal its anchor carries rather than by its directory name.
type Comment struct {
	// ID is the comment's 12-hex identifier.
	ID string
	// Dir is the comment's directory.
	Dir string
	// TS is when the comment was written.
	TS string
	// Ordinal is the comment's one-based position among the card's comments,
	// assigned when it was written.
	Ordinal int
	// Author is who wrote it.
	Author string
	// Body is the comment itself.
	Body string
}

// AddComment writes a comment entity under a card and returns it. The caller
// holds the card's lock, which is what makes the ordinal scan race-free.
func AddComment(cardDir, author, ts, body string) (*Comment, error) {
	collection := filepath.Join(cardDir, CommentsDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	ordinal := nextOrdinal(collection, CommentAnchor)
	dir := filepath.Join(collection, id)
	fm := NewFrontmatter()
	fm.Set("ts", ts)
	fm.Set("author", author)
	fm.Set(OrdinalField, strconv.Itoa(ordinal))
	if err := WriteText(filepath.Join(dir, CommentAnchor), fm.Render(body)); err != nil {
		return nil, err
	}
	comment := &Comment{
		ID:      id,
		Dir:     dir,
		TS:      ts,
		Ordinal: ordinal,
		Author:  author,
		Body:    body,
	}
	return comment, nil
}

// Comments reads a card's comments in creation order.
//
// The order is the ordinal's rather than the timestamp's, because a timestamp
// is wall-clock and two processes commenting inside one second record the same
// one, which leaves the reader's order to the directory listing.
func Comments(cardDir string) ([]*Comment, error) {
	collection := filepath.Join(cardDir, CommentsDir)
	var comments []*Comment
	for _, id := range SortByOrdinal(collection, CommentAnchor, ListIDs(collection)) {
		dir := filepath.Join(collection, id)
		text, err := ReadText(filepath.Join(dir, CommentAnchor))
		if err != nil {
			continue
		}
		fm, body := ParseAnchor(text)
		comment := &Comment{
			ID:      id,
			Dir:     dir,
			TS:      fm.Value("ts"),
			Ordinal: OrdinalOf(fm),
			Author:  fm.Value("author"),
			Body:    body,
		}
		comments = append(comments, comment)
	}
	return comments, nil
}

// Attachment is one attachment: the entity wrapping bytes the format never
// inspects, carrying the original filename, a description and provenance.
type Attachment struct {
	// ID is the attachment's 12-hex identifier.
	ID string
	// Dir is the attachment's directory.
	Dir string
	// Filename is the payload's original name.
	Filename string
	// Description is the optional prose describing the attachment.
	Description string
	// Provenance says where the bytes came from.
	Provenance string
	// Ordinal is the attachment's one-based position among the attachments of
	// the entity it hangs from, assigned when it was written.
	Ordinal int
}

// AddAttachment copies a file into a new attachment entity of the collection
// belonging to any entity directory: the bench, a state, a card or a comment.
// The caller holds the lock covering that collection, which is what makes the
// ordinal scan race-free.
func AddAttachment(ownerDir, source, description, provenance string) (*Attachment, error) {
	collection := filepath.Join(ownerDir, AttachmentsDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	ordinal := nextOrdinal(collection, AttachmentAnchor)
	dir := filepath.Join(collection, id)
	filename := filepath.Base(source)
	fm := NewFrontmatter()
	fm.Set("filename", filename)
	if description != "" {
		fm.Set("description", description)
	}
	fm.Set("provenance", provenance)
	fm.Set(OrdinalField, strconv.Itoa(ordinal))
	if err := WriteText(filepath.Join(dir, AttachmentAnchor), fm.Render("")); err != nil {
		return nil, err
	}
	if err := copyFile(source, filepath.Join(dir, PayloadDir, filename)); err != nil {
		return nil, err
	}
	attachment := &Attachment{
		ID:          id,
		Dir:         dir,
		Filename:    filename,
		Description: description,
		Provenance:  provenance,
		Ordinal:     ordinal,
	}
	return attachment, nil
}

// ReplaceAttachment swaps an attachment's payload for the bytes of another
// file, which is a journaled act rather than a quiet overwrite.
func ReplaceAttachment(dir, source string) (*Attachment, error) {
	attachment, err := LoadAttachment(dir)
	if err != nil {
		return nil, err
	}
	payload := filepath.Join(dir, PayloadDir)
	if err := os.RemoveAll(payload); err != nil {
		return nil, err
	}
	filename := filepath.Base(source)
	if err := copyFile(source, filepath.Join(payload, filename)); err != nil {
		return nil, err
	}
	fm, body := loadAnchor(filepath.Join(dir, AttachmentAnchor))
	fm.Set("filename", filename)
	if err := WriteText(filepath.Join(dir, AttachmentAnchor), fm.Render(body)); err != nil {
		return nil, err
	}
	attachment.Filename = filename
	return attachment, nil
}

// LoadAttachment reads an attachment entity from its directory.
func LoadAttachment(dir string) (*Attachment, error) {
	text, err := ReadText(filepath.Join(dir, AttachmentAnchor))
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, dir)
	}
	fm, _ := ParseAnchor(text)
	attachment := &Attachment{
		ID:          filepath.Base(dir),
		Dir:         dir,
		Filename:    fm.Value("filename"),
		Description: fm.Value("description"),
		Provenance:  fm.Value("provenance"),
		Ordinal:     OrdinalOf(fm),
	}
	return attachment, nil
}

// loadAnchor reads an anchor file, returning an empty header when it will not
// read, which is what keeps a caller mid-write from having to decide.
func loadAnchor(path string) (*Frontmatter, string) {
	text, err := ReadText(path)
	if err != nil {
		return NewFrontmatter(), ""
	}
	return ParseAnchor(text)
}

// copyFile copies bytes into a new file, creating the directories above it.
func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ArchiveTarget is where an entity directory goes when it is archived: the
// archive mirror at its own level, which is one pattern serving every kind at
// every depth.
func ArchiveTarget(dir string) string {
	collection := filepath.Dir(dir)
	parent := filepath.Dir(collection)
	return filepath.Join(parent, ArchiveDir, filepath.Base(collection), filepath.Base(dir))
}

// RestoreTarget is where an archived entity directory goes when it is
// restored: the live half of its own collection, which is the archive's
// mirror read in the other direction.
func RestoreTarget(dir string) string {
	collection := filepath.Dir(dir)
	archive := filepath.Dir(collection)
	parent := filepath.Dir(archive)
	return filepath.Join(parent, filepath.Base(collection), filepath.Base(dir))
}

// WorkstreamRefPrefix is the word a generic entity reference names a
// workstream's kind with, and the slash that separates it from the reference
// itself.
//
// A workstream is the one kind that names itself in that grammar. A bare
// reference is tried against the states before anything else, so a bare
// workstream reference would be shadowed by a state of the same name,
// silently, and only in the workbenches unlucky enough to have picked one.
// Naming the kind costs one word and lets a workstream and a state share a
// name.
const WorkstreamRefPrefix = "workstream/"

// resolveWorkstreamRef resolves a reference that names the workstream kind.
// The second return value reports whether the reference named that kind at all,
// which is what tells ResolveEntity to stop rather than fall through to the
// states and the cards: a caller who wrote workstream/ meant a workstream, so
// a name no workstream answers to is refused here rather than reported as an
// unknown card.
func (b *Bench) resolveWorkstreamRef(ref string) (*EntityRef, bool, error) {
	rest, named := strings.CutPrefix(ref, WorkstreamRefPrefix)
	if !named {
		return nil, false, nil
	}
	workstream := b.WorkstreamByRef(rest)
	if workstream == nil {
		return nil, true, contract.Refuse(contract.UnknownWorkstream, rest)
	}
	entity := &EntityRef{
		Kind: "workstream",
		Dir:  workstream.Dir,
		ID:   workstream.ID,
		Ref:  workstream.Ref(),
	}
	return entity, true, nil
}

// MoveEntity carries an entity's whole directory to another path, history and
// all. A rename the filesystem refuses is reported as a refusal and never
// retried as a copy followed by a delete, which would trade one short
// non-atomic operation for a long one and multiply the states a crash leaves.
func MoveEntity(dir, target string) error {
	if Exists(target) {
		return contract.Refuse(contract.Exists, target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.Rename(dir, target)
}

// ArchiveEntity moves an entity's whole directory into the archive mirror,
// history and all. It is the move a structural act performs at its sixth
// step rather than the whole of the act.
func ArchiveEntity(dir string) (string, error) {
	target := ArchiveTarget(dir)
	if err := MoveEntity(dir, target); err != nil {
		return "", err
	}
	return target, nil
}

// DeleteEntity removes an entity's directory and the history inside it.
func DeleteEntity(dir string) error {
	return os.RemoveAll(dir)
}

// StateOccupied reports whether a state may be retired, which is what keeps a
// state from being archived or deleted underneath its cards. A nil answer
// means it may; anything else is the refusal to report, naming what was found.
//
// The scan runs after the retiring act's own sibling exists, never before, so
// a writer that reaches the destination's sibling first is one this walk
// cannot miss. It refuses on three conditions. A live card whose state is
// this one is the ordinary occupancy refusal. A card whose own lock is held
// is a write whose destination cannot be read yet, and a card directory that
// will not load is a creation whose destination cannot be read either; both
// refuse conservatively, since a refusal costs a retry while a guess costs a
// card pointing into the archive.
//
// Per card the lock is stated first and the anchor read second, never the
// reverse. A pass that loaded every anchor and stated locks afterwards would
// leave a gap a mover's whole critical section fits inside, since the read
// could take the old state, the mover could then write and release, and the
// later stat would find a free lock with nothing having fired. That two-pass
// shape is the natural one to reach for, because Cards loads every anchor in
// a single pass, so this is written as its own walk rather than as a call to
// it. What carries over from Cards is the treatment of a card that will not
// load, and not the loop.
//
// id is what the occupancy comparison runs against; ref is what the
// occupancy refusal names the state by, so a caller who typed a slug reads
// that same slug back rather than the raw identifier behind it.
func (b *Bench) StateOccupied(id, ref string) error {
	for _, cardID := range ListIDs(b.CardsRoot()) {
		dir := filepath.Join(b.CardsRoot(), cardID)
		locked := Exists(filepath.Join(dir, LockName))
		if b.Hooks != nil && b.Hooks.BeforeAnchorRead != nil {
			b.Hooks.BeforeAnchorRead(cardID)
		}
		card, err := LoadCard(b.CardsRoot(), cardID)
		if err != nil {
			return contract.Refuse(contract.Locked, cardID)
		}
		if card.State == id {
			return contract.Refuse(contract.Occupied, ref)
		}
		if locked {
			return contract.Refuse(contract.Locked, cardID)
		}
	}
	return nil
}

// StructuralAct is one act that moves or removes an entity directory:
// archiving, restoring or deleting. The directory the entity's own lock lives
// in is the directory that goes, so the lock cannot arbitrate its own
// disappearance and the act takes three locks rather than one.
type StructuralAct struct {
	// Dir is the entity directory the act moves or removes.
	Dir string
	// LockDir is the directory whose lock the act takes at its third step,
	// which is the nearest enclosing journal-bearing entity: the card's own
	// directory for a card and for anything below one. An act whose scope
	// is the bench leaves this empty, since the bench's own lock is the one
	// already taken at the first step.
	LockDir string
	// Op is one of OpArchive, OpRestore and OpDelete.
	Op string
	// Actor is the owner the act is attributed to.
	Actor string
	// Now is the timestamp every lock the act takes records.
	Now string
	// StateID is the identifier of the state being retired, empty for an
	// act on any other kind. A non-empty one arms the occupancy scan.
	StateID string
	// StateRef is what a person typed, or could type, to reach that same
	// state (its slug, falling back to its identifier). A refusal raised
	// over the state names it by StateRef, never by the bare StateID, so
	// a person who typed a slug is never told about an identifier they
	// never saw. Empty exactly when StateID is.
	StateRef string
	// WorkstreamID is the identifier of the workstream being deleted, empty
	// for an act on any other kind and for an archiving. A non-empty one
	// arms the membership scan, which archiving does not run: archiving a
	// finished effort while its cards sit in Done is the ordinary case, and
	// an archived workstream still resolves, so no card is left dangling.
	WorkstreamID string
	// WorkstreamRef is what a person typed, or could type, to reach that
	// same workstream, on the terms StateRef states. Empty exactly when
	// WorkstreamID is.
	WorkstreamRef string
	// Record appends the act's event, and is called at the fourth step. It
	// is the point of record: a failure before it unwinds everything, and a
	// failure after it leaves the sibling standing.
	Record func() error
}

// Target is where the act is taking the directory, empty for a removal.
func (a *StructuralAct) Target() string {
	switch a.Op {
	case OpArchive:
		return ArchiveTarget(a.Dir)
	case OpRestore:
		return RestoreTarget(a.Dir)
	}
	return ""
}

// siblingDir is the directory the act's sibling stands beside, which is
// always the live half of the collection whichever way the entity is
// travelling. One identifier then carries at most one act at a time across
// both halves, and every writer reads one path.
func (a *StructuralAct) siblingDir() string {
	if a.Op == OpRestore {
		return a.Target()
	}
	return a.Dir
}

// apply is the act's own change to the tree: the rename that archives or
// restores a directory, or the removal that deletes one.
func (a *StructuralAct) apply() error {
	if a.Op == OpDelete {
		return DeleteEntity(a.Dir)
	}
	return MoveEntity(a.Dir, a.Target())
}

// Run performs a structural act under the protocol the format's concurrency
// section fixes: the bench's own lock, then the sibling beside the directory
// that is about to move, then the entity's own lock, the event, the release of
// that lock, the move, and the two releases in reverse.
//
// Every acquisition is a try that refuses rather than a wait that blocks, so
// the fixed order is a deadlock rule timing cannot defeat. The entity's lock
// is given back before the move because a lock must never travel into an
// archive and must never be held open across a removal, and the sibling is
// what covers the window that opens there.
func (b *Bench) Run(act *StructuralAct) error {
	benchLock, err := Acquire(b.Root, act.Actor, act.Now)
	if err != nil {
		return err
	}
	if err := b.step(1); err != nil {
		return unwind(err, benchLock)
	}

	sibling, record, err := AcquireSibling(act.siblingDir(), act.Actor, act.Now, act.Op, act.Target())
	if err != nil {
		benchLock.Release()
		return err
	}
	if err := b.step(2); err != nil {
		return unwind(err, sibling, benchLock)
	}
	if target := act.Target(); target != "" && Exists(target) {
		return unwind(contract.Refuse(contract.Exists, target), sibling, benchLock)
	}
	if act.WorkstreamID != "" {
		if err := b.WorkstreamReferenced(act.WorkstreamID, act.WorkstreamRef); err != nil {
			return unwind(err, sibling, benchLock)
		}
	}
	if act.StateID != "" {
		if err := b.StateOccupied(act.StateID, act.StateRef); err != nil {
			return unwind(err, sibling, benchLock)
		}
		if act.Op != OpRestore && len(b.States) <= 1 {
			return unwind(contract.Refuse(contract.LastState, act.StateRef), sibling, benchLock)
		}
	}

	// An entity that vanished between the moment a caller resolved it and
	// the moment the act reached its lock is reported as the unknown entity
	// it has become, rather than as whatever error the filesystem raises.
	if !Exists(act.Dir) {
		refusal := contract.Refuse(contract.UnknownCard, filepath.Base(act.Dir))
		return unwind(refusal, sibling, benchLock)
	}
	entityLock, err := b.takeEntityLock(act, record)
	if err != nil {
		return unwind(err, sibling, benchLock)
	}
	if err := b.step(3); err != nil {
		return unwind(err, entityLock, sibling, benchLock)
	}

	if err := act.Record(); err != nil {
		return unwind(err, entityLock, sibling, benchLock)
	}
	if err := b.step(4); err != nil {
		return unwind(err, entityLock, sibling, benchLock)
	}

	// Past the point of record. The sibling now stays where it is on any
	// failure, so that a retry follows the path a crash leaves rather than
	// leaving an archived event beside a live card with nothing on disk
	// saying an act was in flight.
	entityLock.Release()
	if err := b.step(5); err != nil {
		return reportInterruption(err, act, benchLock)
	}
	if err := act.apply(); err != nil {
		return reportInterruption(err, act, benchLock)
	}
	if act.StateID != "" && act.Op != OpRestore {
		if err := b.RemoveStateID(act.StateID); err != nil {
			return reportInterruption(err, act, benchLock)
		}
	}
	if err := b.step(6); err != nil {
		return reportInterruption(err, act, benchLock)
	}

	sibling.Release()
	if err := b.step(7); err != nil {
		return unwind(err, benchLock)
	}
	benchLock.Release()
	return b.step(8)
}

// takeEntityLock performs the act's third acquisition, through the acquire
// that tolerates the one sibling the act itself wrote. An act whose scope is
// the bench takes nothing here, because the bench's own lock is what covers a
// write at that scope and the act has held it since its first step.
func (b *Bench) takeEntityLock(act *StructuralAct, record LockRecord) (*Lock, error) {
	if act.LockDir == "" || act.LockDir == b.Root {
		return nil, nil
	}
	return acquireTolerating(act.LockDir, act.Actor, act.Now, record)
}

// step runs the injected failure a test asks for at one numbered step of the
// protocol, and is a no-op on every bench nobody is testing.
func (b *Bench) step(n int) error {
	if b.Hooks == nil || b.Hooks.AfterStep == nil {
		return nil
	}
	return b.Hooks.AfterStep(n)
}

// unwind gives back what an act took, in the reverse of the order it took
// them. A failure standing for a process that died releases nothing, because
// a dead process releases nothing, and the bench is left as a crash leaves it.
func unwind(err error, locks ...*Lock) error {
	if err == ErrAborted {
		return err
	}
	for _, lock := range locks {
		lock.Release()
	}
	return err
}

// reportInterruption reports a failure the tool saw for itself after the point of
// record. The sibling is left standing as the record of what was in flight,
// and the bench lock is released so the finish is not deadlocked against its
// own predecessor.
func reportInterruption(err error, act *StructuralAct, benchLock *Lock) error {
	if err == ErrAborted {
		return err
	}
	benchLock.Release()
	return contract.Refuse(contract.Interrupted, filepath.Base(act.Dir))
}

// EntityRef is a reference resolved to an entity directory and the kind of
// thing that directory holds.
type EntityRef struct {
	// Kind is one of the containment grammar's kinds, as Contains names
	// them: workbench, state, card, comment, item and attachment, plus
	// workstream, which resolves through its own dedicated prefix rather
	// than through the containment grammar.
	Kind string
	// Dir is the entity's directory.
	Dir string
	// ID is the entity's identifier, empty for the bench itself.
	ID string
	// Ref is what a person typed, or could type, to reach this entity: a
	// state's or a workstream's slug (falling back to its identifier), and
	// for anything below a head, that head's own reference followed by the
	// path down to it. It is empty only for the workbench itself, whose own
	// spelling is a question this resolver does not settle. A refusal
	// raised over this entity names it by Ref rather than by the bare ID,
	// so a person who typed a slug is never told about a raw identifier
	// they never saw, and a command drawing a header from an answer has an
	// address to print in it.
	Ref string
	// Card is the card the entity belongs to, when one does.
	Card *Card
}

// ResolveEntity resolves the reference the entity-shaped commands take: the
// bench itself, a state, a workstream, a card, or any entity below one of
// those. It accepts the same references ResolvePath does, so a reference a
// walk prints names the same entity to every command that takes one.
//
// An answer of kind card always carries the card, and an answer below a card
// always carries the card it belongs to. Callers read Card without asking, and
// the ones that ask read a nil as the entity belonging to no card at all: the
// event a write records goes to the bench journal and the lock it takes is the
// bench's. A half-filled answer therefore does not degrade, it misreports, so
// the last guard below refuses rather than returning one.
func (b *Bench) ResolveEntity(ref string) (*EntityRef, error) {
	ref = strings.TrimSpace(ref)
	// The empty reference is this resolver's own case and IsWorkbenchRef does
	// not carry it, because ResolvePath refuses it. See IsWorkbenchRef.
	if ref == "" || IsWorkbenchRef(ref) {
		return &EntityRef{Kind: KindWorkbench, Dir: b.Root}, nil
	}
	// A workstream names its own kind in the grammar, per WorkstreamRefPrefix,
	// so it is tried before the states and the cards rather than falling
	// through to them: a bare workstream reference would otherwise be
	// shadowed by a state or a card sharing its name.
	if entity, named, err := b.resolveWorkstreamRef(ref); named {
		return entity, err
	}
	head, rest, _ := strings.Cut(ref, "/")
	if rest == "" {
		if state := b.StateByRef(ref); state != nil {
			return &EntityRef{
				Kind: KindState,
				Dir:  filepath.Join(b.Root, StatesDir, state.ID),
				ID:   state.ID,
				Ref:  state.Ref(),
			}, nil
		}
		found, err := b.ResolveCard(head)
		if err != nil {
			return nil, err
		}
		return &EntityRef{Kind: KindCard, Dir: found.Card.Dir, ID: found.Card.ID, Ref: found.Card.Ref(b.Slug), Card: found.Card}, nil
	}
	path, card, err := b.resolveBelow(ref)
	if err != nil {
		return nil, err
	}
	kind, named := KindOfAnchor(filepath.Base(path))
	if !named {
		return nil, contract.Refuse(contract.UnknownPath, rest)
	}
	// No reference reaches this guard today, because descend refuses a
	// collection whose kind is addressed in its own right before anything
	// half-filled is built, so deleting it reddens no test. It stays because
	// the invariant belongs on this function rather than in the caller that
	// happens to enforce it, and a reader meeting it here is told what every
	// caller of ResolveEntity may assume.
	if kind == KindCard && card == nil {
		return nil, contract.Refuse(contract.UnknownCard, ref)
	}
	dir := filepath.Dir(path)
	headKind, headRef, headDir := KindWorkbench, b.Slug, b.Root
	if card != nil {
		headKind, headRef, headDir = KindCard, card.Ref(b.Slug), card.Dir
	} else if !IsWorkbenchRef(head) && head != b.Slug {
		if state := b.StateByRef(head); state != nil {
			headKind, headRef = KindState, state.Ref()
			headDir = filepath.Join(b.Root, StatesDir, state.ID)
		}
	}
	return &EntityRef{
		Kind: kind,
		Dir:  dir,
		ID:   filepath.Base(dir),
		Ref:  b.refBelowHead(headKind, headRef, headDir, dir),
		Card: card,
	}, nil
}

// refBelowHead composes the reference of an entity sitting below a head: the
// head's own reference, then one collection name and one position for each
// level down to the entity. The head is whichever of the workbench, a state,
// or a card the reference was resolved through.
//
// A position is the entity's place in its collection's creation order, which
// is what a containment walk draws and what a person types, rather than the
// identifier its directory is named for. Composing it here is what gives one
// entity one spelling however the caller reached it, whether by an identifier,
// by a narrowed checklist alias, or by the position itself.
//
// An entity this composer cannot name comes back with no reference at all,
// because a reference naming the head instead would send a reader somewhere
// they did not ask for, and an absent answer is one a caller can see.
func (b *Bench) refBelowHead(headKind, headRef, headDir, dir string) string {
	below, err := filepath.Rel(headDir, dir)
	if err != nil {
		return ""
	}
	// The path below a head alternates a collection's directory with one
	// member's identifier, so every level is two segments and an odd count is
	// a path this composer was never meant to be given.
	segments := strings.Split(filepath.ToSlash(below), "/")
	if len(segments)%2 != 0 {
		return ""
	}
	ref, kind, at := headRef, headKind, headDir
	for i := 0; i < len(segments); i += 2 {
		mount, ok := MountOf(kind, segments[i])
		if !ok {
			return ""
		}
		collection := filepath.Join(at, mount.Dir)
		position := 0
		for n, id := range SortByOrdinal(collection, mount.Anchor, ListIDs(collection)) {
			if id == segments[i+1] {
				position = n + 1
				break
			}
		}
		if position == 0 {
			return ""
		}
		ref = ref + "/" + mount.Dir + "/" + strconv.Itoa(position)
		kind, at = mount.Kind, filepath.Join(collection, segments[i+1])
	}
	return ref
}
