package bench

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// Comment is one comment: an entity like every other, ordered by the creation
// ordinal it was written with. It hangs below a card, below one of that card's
// checklist items, or below a column. On a store in the card-unit layout it is
// a member of its nearest journal; below that format it is a directory of its
// own, and CardRecord reads both the same way.
type Comment struct {
	// ID is the comment's 12-hex identifier.
	ID string
	// Holder is what the comment hangs on: empty for the card, the item's
	// identifier for an item comment, and the column's identifier for a
	// column comment.
	Holder string
	// Home is the directory the comment's attachments collection hangs
	// from. It is not a file of the comment's own: in the card-unit layout
	// it is comments/<id> below the holding card or column and need not
	// exist, and below that format it is the comment's own directory.
	Home string
	// Archived reports whether the comment was archived in its own right.
	// Its half is also the archived one when the item it hangs on is
	// archived, which CardRecord answers.
	Archived bool
	// Redacted is who replaced the comment's text with its digest and when,
	// nil on a comment nobody redacted.
	Redacted *Redaction
	// TS is when the comment was written.
	TS string
	// Ordinal is the comment's one-based position among its holder's
	// comments, assigned when it was written.
	Ordinal int
	// Author is who wrote it. It is empty on a comment the note migration
	// could not attribute, which records the absence in
	// AuthorUnrecoverable rather than inventing a name.
	Author string
	// AuthorUnrecoverable is true where the store cannot say who wrote the
	// comment and says so. An author is plain text with nothing reserved,
	// so a word standing for the absence would collide with a person of
	// that name; absence itself collides with nothing.
	AuthorUnrecoverable bool
	// RecordedDigest is the hex-encoded SHA-256 the last verb to write this
	// comment recorded over its body, and is empty on a comment no verb has
	// written since dinah-525. The comment is diverged exactly when it
	// differs from CommentDigest of the body beside it.
	RecordedDigest string
	// Body is the comment itself.
	Body string

	// raw is the whole anchor file as the old layout stored it, which is
	// what a show of the comment prints there without reading the file
	// again; it is empty in the card-unit layout.
	raw string
}

// Attachments reads a card's attachments in creation order.
//
// The order is the ordinal's rather than the directory listing's, on the same
// terms Comments already orders its comments. An attachment carrying no
// ordinal sorts ahead of every stamped one, and SortByOrdinal recovers the
// order such attachments were attached in from the card's journal, which is
// the order check --migrate-ordinals will stamp them in.
func Attachments(cardDir string) ([]*Attachment, error) {
	return attachments(Disk{}, cardDir)
}

// Attachments is the free Attachments read through this bench's source.
func (b *Bench) Attachments(cardDir string) ([]*Attachment, error) {
	return attachments(b.source(), cardDir)
}

// attachments is Attachments's body, reading through src.
func attachments(src Source, cardDir string) ([]*Attachment, error) {
	collection := filepath.Join(cardDir, AttachmentsDir)
	ids, err := listIDs(src, collection)
	if err != nil {
		return nil, err
	}
	var attachments []*Attachment
	for _, id := range sortByOrdinal(src, collection, AttachmentAnchor, ids) {
		attachment, err := attachmentAt(src, filepath.Join(collection, id))
		if err != nil {
			continue
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// attachmentFromText builds an attachment from the text of its anchor, which
// is the build Attachments and Positions.Attachments share so the two cannot
// drift apart. It leaves Path empty: the payload's name is read from the
// payload directory by withPayload, because the anchor does not carry it.
func attachmentFromText(dir, id, text string) *Attachment {
	fm, _ := ParseAnchor(text)
	return &Attachment{
		ID:          id,
		Dir:         dir,
		Filename:    fm.Value("filename"),
		Description: fm.Value("description"),
		Provenance:  fm.Value("provenance"),
		Ordinal:     OrdinalOf(fm),
	}
}

// CountAttachments reports how many attachments a directory's own collection
// holds, and it opens no attachment's anchor to do it. It is the cheap half of
// Attachments, for a caller that wants the number rather than the list: one
// directory read instead of one file read per attachment.
//
// A listing that carried every attachment of every card it holds would cost
// what the listing is long, so the many-entity reads carry this count and the
// single-entity reads carry the list.
//
// A collection that will not read is reported rather than counted as none,
// because a count of zero is what an entity holding no attachments answers
// and a caller cannot tell the two apart.
func CountAttachments(dir string) (int, error) {
	return countAttachments(Disk{}, dir)
}

// CountAttachments is the free CountAttachments read through this bench's source.
func (b *Bench) CountAttachments(dir string) (int, error) {
	return countAttachments(b.source(), dir)
}

// countAttachments is CountAttachments's body, reading through src.
func countAttachments(src Source, dir string) (int, error) {
	ids, err := listIDs(src, filepath.Join(dir, AttachmentsDir))
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// ChildCounts is how many members sit in each collection the containment
// grammar gives a kind, keyed by the collection's directory name. It is the
// one place a caller can learn what an entity holds without naming the
// collections, so a kind that gains a mount is counted here with no edit.
//
// One level only: the card's own live comments and checklist from its record,
// and one directory listing for its attachments.
//
// A collection that will not read is reported rather than counted as none, on
// the terms CountAttachments already states: a zero is what an entity holding
// nothing answers, and a caller cannot tell the two apart.
func (b *Bench) ChildCounts(card *Card) (map[string]int, error) {
	counts, _, err := b.NewPositions().CardCounts(b, card)
	return counts, err
}

// ChildTotal sums what ChildCounts answered.
func ChildTotal(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
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
	// Path is the absolute path to the one payload file the attachment
	// wraps, and it is empty when the payload directory is missing or holds
	// nothing. That emptiness is an integrity defect `dinah check` already
	// reports under its own finding, so a read degrades here rather than
	// refusing the whole listing.
	//
	// The field is a Dinah-local convenience rather than a core member of
	// the shared contract. Dinah holds its workbenches on local disk and
	// always publishes it, an implementation that does not hold them there
	// omits it and stays conformant, and every client has to work when it
	// is absent.
	Path string
}

// AddAttachment copies a file into a new attachment entity of the collection
// belonging to any entity directory: the bench, a workstream, a column, a card
// or a comment.
// The caller holds the lock covering that collection, which is what makes the
// ordinal scan race-free.
func AddAttachment(ownerDir, source, description, provenance string) (*Attachment, error) {
	payload, err := durable.ReadFile(source)
	if err != nil {
		return nil, err
	}
	return AddAttachmentBytes(ownerDir, filepath.Base(source), payload, description, provenance)
}

// AddAttachmentBytes writes a new attachment entity into ownerDir's
// attachments collection from bytes already in hand, stamping the next
// ordinal. AddAttachment reads its source file and calls this, and a
// definition carrying a column's attachments writes them through it, so the
// two paths cannot lay an attachment out differently. The payload is written
// byte for byte, never through WriteText, which would normalise its newlines.
func AddAttachmentBytes(ownerDir, filename string, payload []byte, description, provenance string) (*Attachment, error) {
	return addAttachmentBytes(Disk{}, ownerDir, filename, payload, description, provenance)
}

// AddAttachmentBytes is the free AddAttachmentBytes read through this bench's source.
func (b *Bench) AddAttachmentBytes(ownerDir, filename string, payload []byte, description, provenance string) (*Attachment, error) {
	return addAttachmentBytes(b.source(), ownerDir, filename, payload, description, provenance)
}

// addAttachmentBytes is AddAttachmentBytes's body, reading through src.
func addAttachmentBytes(src Source, ownerDir, filename string, payload []byte, description, provenance string) (*Attachment, error) {
	collection := filepath.Join(ownerDir, AttachmentsDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	ordinal, err := nextOrdinal(src, collection, AttachmentAnchor)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(collection, id)
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
	target := filepath.Join(dir, PayloadDir, filename)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	if err := durable.WriteFile(target, payload, 0o644); err != nil {
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
//
// The new bytes are written over the payload's current name with
// durable.WriteFile, which replaces them atomically and flushes them first.
// When the new file carries a different name, RenameAttachment then carries the
// payload to it and rewrites the anchor, so a crash between the two leaves the
// disagreement check.attachment-filename-drift already reports.
func ReplaceAttachment(dir, source string) (*Attachment, error) {
	return replaceAttachment(Disk{}, dir, source)
}

// ReplaceAttachment is the free ReplaceAttachment read through this bench's source.
func (b *Bench) ReplaceAttachment(dir, source string) (*Attachment, error) {
	return replaceAttachment(b.source(), dir, source)
}

// replaceAttachment is ReplaceAttachment's body, reading through src.
func replaceAttachment(src Source, dir, source string) (*Attachment, error) {
	attachment, err := loadAttachment(src, dir)
	if err != nil {
		return nil, err
	}
	data, err := src.ReadFile(source)
	if err != nil {
		return nil, err
	}
	filename := filepath.Base(source)
	current := filename
	if attachment.Path != "" {
		current = filepath.Base(attachment.Path)
	}
	payload := filepath.Join(dir, PayloadDir)
	if err := os.MkdirAll(payload, 0o755); err != nil {
		return nil, err
	}
	if err := durable.WriteFile(filepath.Join(payload, current), data, 0o644); err != nil {
		return nil, err
	}
	if current == filename && attachment.Filename == filename {
		return attachment, nil
	}
	_, renamed, err := renameAttachment(src, dir, filename)
	if err != nil {
		return nil, err
	}
	return renamed, nil
}

// RenameAttachment carries an attachment's payload under a new filename and
// rewrites the anchor's filename field to match. It is called under the
// nearest enclosing journal-bearing entity's lock, which is what keeps a
// crash between the two writes detectable by check.
//
// The two writes happen in the order the spec fixes: the file on disk first,
// the anchor second. A crash between them leaves the anchor carrying the old
// name and the payload file carrying the new one, which is the disagreement
// check reports.
//
// A name equal to the current name is the no-op branch, and the caller checks
// for it before this runs so the journal records nothing. Renaming to a name
// some other attachment already carries is allowed, since the name selector
// already refuses the reference that would guess at it.
func RenameAttachment(dir, name string) (*Attachment, *Attachment, error) {
	return renameAttachment(Disk{}, dir, name)
}

// RenameAttachment is the free RenameAttachment read through this bench's source.
func (b *Bench) RenameAttachment(dir, name string) (*Attachment, *Attachment, error) {
	return renameAttachment(b.source(), dir, name)
}

// renameAttachment is RenameAttachment's body, reading through src.
func renameAttachment(src Source, dir, name string) (*Attachment, *Attachment, error) {
	attachment, err := loadAttachment(src, dir)
	if err != nil {
		return nil, nil, err
	}
	// The name is normalised above the no-op comparison, so the anchor's
	// filename key and the payload file's own name on disk take one value.
	// Normalising only inside quote would clean the anchor's copy and leave it
	// naming a file that is not there.
	name = NormalizeNewlines(name)
	if attachment.Filename == name {
		return attachment, attachment, nil
	}
	payload := filepath.Join(dir, PayloadDir)
	entries, err := src.ReadDir(payload)
	if err != nil || len(entries) == 0 {
		return nil, nil, contract.Refuse(contract.UnknownPath, payload)
	}
	from := entries[0].Name()
	if from != name {
		if err := durable.Replace(filepath.Join(payload, from), filepath.Join(payload, name)); err != nil {
			return nil, nil, err
		}
	}
	fm, body := loadAnchor(src, filepath.Join(dir, AttachmentAnchor))
	fm.Set("filename", name)
	if err := WriteText(filepath.Join(dir, AttachmentAnchor), fm.Render(body)); err != nil {
		return nil, nil, err
	}
	before := &Attachment{Filename: from, ID: attachment.ID, Dir: attachment.Dir}
	attachment.Filename = name
	return before, attachment, nil
}

// ValidAttachmentName reports whether a name is one rename will accept. Empty
// after trimming, a name carrying a slash, a back slash, or one that is `.` or
// `..` is refused malformed; every other name passes through.
func ValidAttachmentName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return false
	}
	if strings.ContainsAny(trimmed, `/\`) {
		return false
	}
	return true
}

// LoadAttachment reads an attachment entity from its directory.
func LoadAttachment(dir string) (*Attachment, error) {
	return loadAttachment(Disk{}, dir)
}

// LoadAttachment is the free LoadAttachment read through this bench's source.
func (b *Bench) LoadAttachment(dir string) (*Attachment, error) {
	return loadAttachment(b.source(), dir)
}

// loadAttachment is LoadAttachment's body, reading through src.
func loadAttachment(src Source, dir string) (*Attachment, error) {
	attachment, err := attachmentAt(src, dir)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, dir)
	}
	return attachment, nil
}

// loadAnchor reads an anchor file, returning an empty header when it will not
// read, which is what keeps a caller mid-write from having to decide.
func loadAnchor(src Source, path string) (*Frontmatter, string) {
	fm, body, err := anchorOf(src, path)
	if err != nil {
		return NewFrontmatter(), ""
	}
	return fm, body
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
// reference is tried against the columns before anything else, so a bare
// workstream reference would be shadowed by a column of the same name,
// silently, and only in the workbenches unlucky enough to have picked one.
// Naming the kind costs one word and lets a workstream and a column share a
// name.
const WorkstreamRefPrefix = "workstream/"

// resolveWorkstreamRef resolves a reference that names the workstream kind,
// either the workstream itself or something the workstream contains. The third
// return value reports whether the reference named that kind at all, which is
// what tells ResolveEntity to stop rather than fall through to the columns and
// the cards: a caller who wrote workstream/ meant a workstream, so a name no
// workstream answers to is refused here rather than reported as an unknown
// card.
//
// The collection is returned alongside the entity because a reference stopping
// on workstream/<slug>/attachments names a whole collection rather than an
// entity, and this function is the only place that landing can be built for a
// workstream head.
func (b *Bench) resolveWorkstreamRef(half ResolutionHalf, ref string) (*EntityRef, *CollectionRef, bool, error) {
	handle, below, named := WorkstreamHandle(ref)
	if !named {
		return nil, nil, false, nil
	}
	// The half is headHalf's answer rather than the caller's, which is the
	// rule every other head in this resolver already follows: the head is the
	// reference's deepest collection step only when nothing below it names a
	// collection, so workstream/<slug> under --archived reads the archived
	// workstreams root while workstream/<slug>/attachments resolves the
	// workstream live and reads the mirror at the collection step.
	workstream, err := b.workstreamByRefIn(headHalf(half, below), handle)
	if err != nil {
		return nil, nil, true, err
	}
	if workstream == nil {
		// The handle alone is named, whatever follows it, because a caller
		// who wrote workstream/ meant a workstream and the tail is not what
		// went wrong.
		return nil, nil, true, contract.Refuse(contract.UnknownWorkstream, strings.TrimPrefix(handle, WorkstreamRefPrefix))
	}
	if below == "" {
		entity := &EntityRef{
			Kind:     KindWorkstream,
			Dir:      workstream.Dir,
			ID:       workstream.ID,
			Ref:      workstream.Ref(),
			Archived: half == ArchivedHalf,
		}
		return entity, nil, true, nil
	}
	landed := &landing{}
	at := &walkAt{dir: workstream.Dir, kind: KindWorkstream, ref: workstream.Ref()}
	if _, err := b.descend(at, strings.Split(below, "/"), nil, landed, half); err != nil {
		return nil, nil, true, err
	}
	if landed.collection {
		collection, err := b.collectionAt(half, ref, landed)
		if err != nil {
			return nil, nil, true, err
		}
		return nil, collection, true, nil
	}
	if landed.kind == "" {
		return nil, nil, true, contract.Refuse(contract.UnknownPath, below)
	}
	return b.landedEntity(landed, nil, half), nil, true, nil
}

// MoveEntity carries an entity's whole directory to another path, history and
// all. A rename the filesystem refuses is reported as a refusal and never
// retried as a copy followed by a delete, which would trade one short
// non-atomic operation for a long one and multiply the columns a crash leaves.
// A transient refusal, a reader's handle open below the directory, is retried
// as the same rename for a bounded time (durable.MoveDir).
func MoveEntity(dir, target string) error {
	return moveEntity(Disk{}, dir, target)
}

// MoveEntity is the free MoveEntity read through this bench's source.
func (b *Bench) MoveEntity(dir, target string) error {
	return moveEntity(b.source(), dir, target)
}

// moveEntity is MoveEntity's body, reading through src.
func moveEntity(src Source, dir, target string) error {
	if exists(src, target) {
		return contract.Refuse(contract.Exists, target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return durable.MoveDir(dir, target)
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

// DeleteEntity removes an entity's directory and the history inside it,
// retrying a transient refusal as durable.RemoveAll does.
func DeleteEntity(dir string) error {
	return durable.RemoveAll(dir)
}

// ColumnOccupied reports whether a column may be retired, which is what keeps a
// column from being archived or deleted underneath its cards. A nil answer
// means it may; anything else is the refusal to report, naming what was found.
//
// The scan runs after the retiring act's own sibling exists, never before, so
// a writer that reaches the destination's sibling first is one this walk
// cannot miss. It refuses on three conditions. A live card whose column is
// this one is the ordinary occupancy refusal. A card whose own lock is held
// is a write whose destination cannot be read yet, and a card directory that
// will not load is a creation whose destination cannot be read either; both
// refuse conservatively, since a refusal costs a retry while a guess costs a
// card pointing into the archive.
//
// Per card the lock is stated first and the anchor read second, never the
// reverse. A pass that loaded every anchor and stated locks afterwards would
// leave a gap a mover's whole critical section fits inside, since the read
// could take the old column, the mover could then write and release, and the
// later stat would find a free lock with nothing having fired. That two-pass
// shape is the natural one to reach for, because Cards loads every anchor in
// a single pass, so this is written as its own walk rather than as a call to
// it. What carries over from Cards is the treatment of a card that will not
// load, and not the loop.
//
// id is what the occupancy comparison runs against; ref is what the
// occupancy refusal names the column by, so a caller who typed a slug reads
// that same slug back rather than the raw identifier behind it.
func (b *Bench) ColumnOccupied(id, ref string) error {
	cardIDs, err := b.ListIDs(b.CardsRoot())
	if err != nil {
		return err
	}
	for _, cardID := range cardIDs {
		dir := filepath.Join(b.CardsRoot(), cardID)
		locked := b.Exists(filepath.Join(dir, LockName))
		if b.Hooks != nil && b.Hooks.BeforeAnchorRead != nil {
			b.Hooks.BeforeAnchorRead(cardID)
		}
		card, err := b.LoadCardIn(b.CardsRoot(), cardID)
		if err != nil {
			return contract.Refuse(contract.Locked, cardID)
		}
		if card.Column == id {
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
	// ColumnID is the identifier of the column being retired, empty for an
	// act on any other kind. A non-empty one arms the occupancy scan.
	ColumnID string
	// ColumnRef is what a person typed, or could type, to reach that same
	// column (its slug, falling back to its identifier). A refusal raised
	// over the column names it by ColumnRef, never by the bare ColumnID, so
	// a person who typed a slug is never told about an identifier they
	// never saw. Empty exactly when ColumnID is.
	ColumnRef string
	// WorkstreamID is the identifier of the workstream being deleted, empty
	// for an act on any other kind and for an archiving. A non-empty one
	// arms the membership scan, which archiving does not run: archiving a
	// finished effort while its cards sit in Done is the ordinary case, and
	// an archived workstream still resolves, so no card is left dangling.
	WorkstreamID string
	// WorkstreamRef is what a person typed, or could type, to reach that
	// same workstream, on the terms ColumnRef states. Empty exactly when
	// WorkstreamID is.
	WorkstreamRef string
	// Record appends the act's event, and is called at the fourth step. It
	// is the point of record: a failure before it unwinds everything, and a
	// failure after it leaves the sibling standing. locks are the locks the
	// act holds when Record runs, and For answers the one an append to a
	// given journal is made under.
	Record func(locks ActLocks) error
	// Verify, when set, runs immediately after the entity lock is taken and
	// before Record, so a caller can re-confirm a precondition it read
	// before this act's own lock existed rather than trusting that earlier
	// snapshot. A non-nil error unwinds the act exactly as a Record failure
	// would, except that no event is appended: the act simply does not
	// happen, and the error travels to Run's own caller unchanged. Nil for
	// an act that needs no such recheck, which is every caller but
	// archiveOnDone today.
	Verify func() error
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
func (a *StructuralAct) apply(src Source) error {
	if a.Op == OpDelete {
		return DeleteEntity(a.Dir)
	}
	return moveEntity(src, a.Dir, a.Target())
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
	benchLock, err := b.Acquire(b.Root, act.Actor, act.Now)
	if err != nil {
		return err
	}
	if err := b.step(1); err != nil {
		return unwind(err, benchLock)
	}

	sibling, record, err := b.AcquireSibling(act.siblingDir(), act.Actor, act.Now, act.Op, act.Target())
	if err != nil {
		benchLock.Release()
		return err
	}
	if err := b.step(2); err != nil {
		return unwind(err, sibling, benchLock)
	}
	if target := act.Target(); target != "" && b.Exists(target) {
		return unwind(contract.Refuse(contract.Exists, target), sibling, benchLock)
	}
	if act.WorkstreamID != "" {
		if err := b.WorkstreamReferenced(act.WorkstreamID, act.WorkstreamRef); err != nil {
			return unwind(err, sibling, benchLock)
		}
	}
	// A restore runs neither of these. The occupancy scan is what stops an
	// archive stranding a live card that names the column, and on a restore
	// it is backwards: a live card naming a column the workbench does not
	// list is the stranded state check reports, and restoring the column is
	// the repair. The last-column check is about the workbench keeping one
	// column, and a restore adds a column rather than removing one.
	if act.ColumnID != "" && act.Op != OpRestore {
		if err := b.ColumnOccupied(act.ColumnID, act.ColumnRef); err != nil {
			return unwind(err, sibling, benchLock)
		}
		if len(b.Columns) <= 1 {
			return unwind(contract.Refuse(contract.LastColumn, act.ColumnRef), sibling, benchLock)
		}
	}

	// An entity that vanished between the moment a caller resolved it and
	// the moment the act reached its lock is reported as the unknown entity
	// it has become, rather than as whatever error the filesystem raises.
	if !b.Exists(act.Dir) {
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
	if act.Verify != nil {
		if err := act.Verify(); err != nil {
			return unwind(err, entityLock, sibling, benchLock)
		}
	}
	if err := refuseCarriedLock(b.source(), act); err != nil {
		return unwind(err, entityLock, sibling, benchLock)
	}

	if err := act.Record(ActLocks{bench: benchLock, entity: entityLock}); err != nil {
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
	if err := act.apply(b.source()); err != nil {
		return reportInterruption(err, act, benchLock)
	}
	if err := removeTravelledLock(act.Target()); err != nil {
		return reportInterruption(err, act, benchLock)
	}
	if act.ColumnID != "" {
		// The workbench anchor's columns sequence is the single authority
		// for order, so a column leaving it and a column returning to it are
		// both written here, under the bench lock this act still holds, and
		// the anchor never names a column whose directory is not there.
		write := b.RemoveColumnID
		if act.Op == OpRestore {
			write = b.AddColumnID
		}
		if err := write(act.ColumnID); err != nil {
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

// refuseCarriedLock refuses a restore of an entity whose archived directory
// holds a lock the act did not take itself, naming the holder that lock
// records. An earlier build could carry a lock into the archive with the
// directory, and a restore that moved it back would hand the live half a lock
// nobody holds. dinah check reports such a lock and dinah check --finish
// clears it once its holder is proven dead. A card's or a workstream's own
// archived lock is the one the act's third step already took, so only an
// entity whose lock directory is not its own directory, a column, is read
// here.
func refuseCarriedLock(src Source, act *StructuralAct) error {
	if act.Op != OpRestore || act.LockDir == act.Dir {
		return nil
	}
	record, present := readLockRecord(src, filepath.Join(act.Dir, LockName))
	if !present {
		return nil
	}
	return contract.Refuse(contract.Locked, record.Actor)
}

// removeTravelledLock removes the entity lock standing in a directory an
// archive or a restore has just moved, treating an absent one as success. A
// writer that created the entity's lock in the gap between the act's release
// of it and the move reads the sibling, refuses and releases, but the move may
// carry the file with the directory first. Every acquisition refuses while the
// sibling stands, so any lock found here once the move has run is such an
// orphan by construction. A removal has no target and nothing to remove.
func removeTravelledLock(target string) error {
	if target == "" {
		return nil
	}
	err := durable.RemoveLock(filepath.Join(target, LockName))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// takeEntityLock performs the act's third acquisition, through the acquire
// that tolerates the one sibling the act itself wrote. An act whose scope is
// the bench takes nothing here, because the bench's own lock is what covers a
// write at that scope and the act has held it since its first step.
func (b *Bench) takeEntityLock(act *StructuralAct, record LockRecord) (*Lock, error) {
	if act.LockDir == "" || act.LockDir == b.Root {
		return nil, nil
	}
	return acquireTolerating(b.source(), act.LockDir, act.Actor, act.Now, record)
}

// ActLocks are the locks a structural act holds when its Record runs: the
// bench lock its first step took, and the entity lock its third step took,
// which is nil for an act whose scope is the bench.
type ActLocks struct {
	bench  *Lock
	entity *Lock
}

// For answers the lock an append to journal is made under: whichever of the
// act's locks guards the journal's own directory, and nil when neither does,
// which AppendEvent then refuses. A card's deletion is recorded on the bench
// journal and its archiving on the card's own, and this is what lets one
// Record serve both without naming a lock of its own.
func (a ActLocks) For(journal string) *Lock {
	dir := filepath.Dir(journal)
	if a.entity.guards(dir) {
		return a.entity
	}
	if a.bench.guards(dir) {
		return a.bench
	}
	return nil
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
// a dead process releases nothing, and the bench is left as a crash leaves it:
// every lock file stays, and only what the process's own death takes with it,
// its handles and its registry entries, goes.
func unwind(err error, locks ...*Lock) error {
	if err == ErrAborted {
		for _, lock := range locks {
			lock.abandon()
		}
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
		benchLock.abandon()
		return err
	}
	benchLock.Release()
	return contract.Refuse(contract.Interrupted, filepath.Base(act.Dir))
}

// EntityRef is a reference resolved to an entity directory and the kind of
// thing that directory holds.
type EntityRef struct {
	// Kind is one of the containment grammar's kinds: workbench, column,
	// card, comment, item, attachment and workstream. A workstream resolves
	// through its own dedicated prefix rather than by being walked into from
	// above, because nothing contains one, and what hangs below that prefix
	// is walked through the containment grammar like anything else.
	Kind string
	// Dir is the entity's directory. A checklist item in the card-unit layout
	// has none and it is empty; a comment in that layout has none of its own
	// either, and it names the directory the comment's attachments would live
	// in, which need not exist.
	Dir string
	// ID is the entity's identifier, empty for the bench itself.
	ID string
	// Ref is what a person typed, or could type, to reach this entity: a
	// column's or a workstream's slug (falling back to its identifier), and
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
	// Holder is, for a comment, what it hangs on: the item's identifier, the
	// column's identifier, or empty for a comment on the card itself. It is
	// empty for every other kind.
	Holder string
	// Archived reports whether this answer came out of the archive mirror,
	// which is what a renderer reads to mark a listing and what the machine
	// views carry.
	Archived bool
	// Journaled reports a comment or an item of a card-unit store, which is
	// lines of a journal and has no file of its own to open.
	Journaled bool
}

// AnchorPathOf is the path of the file that IS an entity: the entity's own
// directory joined with the anchor filename its kind declares. The second
// answer reports whether the kind declares one at all, which is false for a
// kind outside the grammar and for a journaled member of a card-unit store,
// which has no file of its own. It is reported rather than swallowed
// because AnchorOf answers such a kind with the empty string, and a caller
// joining that gets the entity's directory back, which is a directory where
// it asked for a file. That is the defect dinah-467 fixed for the one kind
// that had it.
func AnchorPathOf(entity *EntityRef) (string, bool) {
	anchor := AnchorOf(entity.Kind)
	if anchor == "" || entity.Journaled {
		return "", false
	}
	return filepath.Join(entity.Dir, anchor), true
}

// ResolveEntity resolves the reference the entity-shaped commands take: the
// bench itself, a column, a workstream, a card, or any entity below one of
// those. It accepts every reference ResolvePath accepts but three, so a
// reference a walk prints names the same entity to every command that takes
// one. It refuses an attachment's payload and a card's journal, neither of
// which carries an anchor, and it refuses a reference naming a whole
// collection, which is not an entity of the format and has no anchor either.
// ResolvePath answers all three with a path.
//
// An answer of kind card always carries the card, and an answer below a card
// always carries the card it belongs to. Callers read Card without asking, and
// the ones that ask read a nil as the entity belonging to no card at all: the
// event a write records goes to the bench journal and the lock it takes is the
// bench's. A half-filled answer therefore does not degrade, it misreports, so
// the last guard below refuses rather than returning one.
func (b *Bench) ResolveEntity(ref string) (*EntityRef, error) {
	return b.ResolveEntityIn(LiveHalf, ref)
}

// ResolveEntityIn is ResolveEntity reading the half a caller names. It is
// ResolveReferenceIn plus the collection refusal, and it needs no call to
// notArchivedFor of its own because ResolveReferenceIn has already made both.
func (b *Bench) ResolveEntityIn(half ResolutionHalf, ref string) (*EntityRef, error) {
	entity, collection, err := b.ResolveReferenceIn(half, ref)
	if err != nil {
		return nil, err
	}
	// A reference naming a whole collection is refused here rather than in
	// each caller, because every caller of this resolver takes one entity
	// and a caller added later would otherwise have to remember the check.
	if collection != nil {
		return nil, collection.Refuse()
	}
	return entity, nil
}

// Item is one checklist item: a card's own recorded judgement, per
// docs/design/format.md's "Checklist items" section. The fields read here are
// the ones a claim decides on and the ones a read reports; a field the format
// names without settling a key for, timestamps among them, is added when the
// card that settles the key arrives.
type Item struct {
	// ID is the item's 12-hex identifier.
	ID string
	// Archived reports whether the item stands in the archived half.
	Archived bool
	// TS is when the item was filed, empty on an item whose anchor carried
	// no stamp.
	TS string
	// Citations are the item's citations in stored order.
	Citations []Citation
	// Redacted is who replaced the item's text with its digest and when,
	// nil on an item nobody redacted.
	Redacted *Redaction
	// Kind is one of acceptance_criterion, open_question and decision.
	Kind string
	// State is one of pending, resolved, verified and failed, and it is
	// whatever the file says rather than a value read into the closed set,
	// so a caller decides for itself what an unrecognized one means.
	State string
	// Ordinal is the position the item's own anchor records, which is what
	// SortByOrdinal reads a collection into creation order by.
	Ordinal int
	// Column is the column the item names for gating, empty when the item
	// was filed without one.
	Column string
	// Owner is who the item names as its answerer. ItemOwnerOperator is the
	// one value enforced against the actor: closeItem in internal/verb
	// refuses a terminal verb on such an item to anybody but the operator,
	// and SetField refuses a rewrite of this key on one. Every other value
	// is enforced against nobody.
	Owner string
	// Resolution is the canonical reference of the comment this item's
	// settling designated as its answer, empty until somebody settles it.
	// It names a comment of this item and never another item's answer or a
	// card comment, which is what resolve, verify and fail refuse at the
	// write. It replaced the free-text note key with dinah-525.
	Resolution string
	// Standing is the key of the standing entry that minted this item, and
	// empty on every hand-filed item. The pair (Column, Standing) is the
	// item's identity for re-entry, which MissingStandingItems reads.
	Standing string
	// Evidence is the scheme this item has to be settled against, empty
	// where nothing demands one. closeItem in internal/verb refuses
	// resolve, verify and fail while no citation names it.
	Evidence string
	// Text is the item's own body, the judgement it was filed under, with
	// the newline every text file ends in trimmed off the end of it. A card
	// body and a comment body are both carried verbatim, and an item's is
	// not, because an item's text is a single judgement rather than prose:
	// `dinah file` writes one and a person editing the tree writes another,
	// and either would otherwise report a trailing newline no reader asked
	// for, in a field a row of a table and a payload of one line both print.
	Text string

	// body is the text exactly as the item's anchor carried it, trailing
	// newline and all, which RenderItemAnchor composes the anchor's body
	// from.
	body string
	// dir is the item's own directory on the old layout, which that
	// layout's own readers and migrations use and nothing else; it is empty
	// in the card-unit layout.
	dir string
	// raw is the whole anchor file as the old layout stored it, which is
	// what a show of the item prints there without reading the file again;
	// it is empty in the card-unit layout.
	raw string
}

// ItemBlocksClaim reports whether an item is one CORE-CLAIM-10 refuses a claim
// over.
//
// Three clauses, and only the third reads the workbench. An acceptance
// criterion never blocks: it is verified after the work rather than before
// it, so blocking a claim on one would refuse the card the very work that
// lets anybody verify it. That ruling reaches a criterion naming no column as
// well as one naming a column, which is why the kind test stays here rather
// than being absorbed into the column test below. A blocking-kind item whose
// state is absent, empty or outside the closed set is read as pending,
// because reading a damaged file as resolved lets through exactly the
// unanswered question the refusal exists to catch, where reading it as
// pending costs a claim until somebody repairs the file.
//
// The third clause is CORE-CLAIM-10's own. An item naming a column the
// workbench declares is left to that column's hold, wherever the card is
// standing and whichever way that column holds, because the column the item
// names is what decides where the card stops. What is left for the claim to
// refuse over is an item naming no column at all and an item whose column
// field carries an identifier this workbench does not declare, neither of
// which any column's hold can ever reach.
//
// No hold is read here, so HoldsOnEntry and HoldsOnExit are not consulted and
// Dinah's exit hold stays off the claim path. The profile can say that a
// column is declared and cannot say which way a workbench holds at one, so a
// tool refusing more than the document states would answer differently from a
// second tool reading the same workbench.
//
// The lookup is Column rather than ColumnByRef, because an item's column
// field carries an identifier and a reference read would admit an item whose
// field happens to match some column's slug or title.
//
// Which columns count as declared is narrower here than the profile's phrase
// sounds, and it is worth saying so where somebody meets a refusal they did
// not expect. Column reads Columns, which is the flow the workbench anchor's
// own sequence resolves to directories. A column the anchor names whose
// directory is gone lands in StrandedColumns instead, and an archived column
// is read from a separate root and never enters Columns at all, so an item
// naming either is refused even though a reader can still see the stranded
// one in the anchor. Both readings are fail-closed and deliberate: the
// refusal is a misfiling alarm, and a column no flow carries is a column no
// hold can ever act on, whichever way the identifier came to be there.
// dinah check is what reports the stranded column itself.
func (b *Bench) ItemBlocksClaim(item *Item) bool {
	if item.Kind != "open_question" && item.Kind != "decision" {
		return false
	}
	if ItemIsResolved(item) {
		return false
	}
	return b.Column(item.Column) == nil
}

// ItemIsResolved reports whether an item has been settled, which is the
// question CORE-ITEM-2 puts to a tool and the one the claim refusal above
// turns on.
//
// It reads the state alone, and no kind is named here or read, because the
// claim refusal above exempts one kind on a ruling of its own and wants that
// exemption at the caller.
//
// A state that is absent, empty or outside the closed set reads as
// unresolved, on ItemBlocksClaim's own reasoning: reading a damaged file as
// settled lets through the very thing a hold exists to catch.
func ItemIsResolved(item *Item) bool {
	switch item.State {
	case ItemResolved, ItemVerified, ItemFailed, ItemWaived, ItemWithdrawn:
		return true
	default:
		return false
	}
}

// ItemLiftsColumnHold reports whether an item's state releases a hold a column
// declaring gate_items puts on a card, whichever way that column holds. The
// declaration carries a direction since dinah-484 and this reading does not:
// the state that settles an item settles it for a card arriving at the column
// and for one leaving it alike.
//
// It answers differently from ItemIsResolved above, on one state and on the
// operator's ruling of 2026-09-10 recorded as dinah-450 OQ-5. A failed item
// records that somebody checked the work and it did not hold, so it releases
// nothing: were failed to settle the hold here, a column gated on an
// acceptance criterion would admit the card on the one state saying the work
// is wrong. Whether the work passed and whether the card may proceed anyway
// are separate questions, and the state carrying the second answer is not
// built. Until it is, the operator's move-level override marker is what
// carries a card past a criterion that genuinely failed, which CORE-GATE-4
// already permits.
//
// The claim refusal keeps ItemIsResolved rather than this. It exempts
// acceptance criteria outright, so the ruling above does not reach it, and
// narrowing the state set underneath it would move behaviour nobody ruled on.
//
// ItemWaived and ItemWithdrawn both release, and they release for different
// reasons. A waiver is the operator's decision that this card may proceed
// past a finding that stands, which is the second of the two jobs the failed
// state used to do at once; a withdrawal says the question stopped applying,
// so there is nothing left for a hold to be waiting on. Neither is reachable
// by anybody but the operator on an item a gate is protecting, which is what
// internal/verb/checklist.go enforces and what makes releasing here safe.
//
// A state that is absent, empty or outside the closed set releases nothing,
// for ItemIsResolved's own reason.
func ItemLiftsColumnHold(item *Item) bool {
	switch item.State {
	case ItemResolved, ItemVerified, ItemWaived, ItemWithdrawn:
		return true
	default:
		return false
	}
}

// ItemAwaitsOperator reports whether an item is in the operator's queue: a
// pending open question or decision that names the operator as its owner or
// names no owner at all.
//
// This is the rule primePending's rule 2 in internal/verb/read.go applies to
// build the operator's queue, extracted here so that the queue and a card
// view's own count of what is waiting on him cannot disagree about which
// items are his. An acceptance criterion never qualifies, because Test
// verifies a criterion rather than the operator, and a state that is absent,
// empty or outside the closed set reads as not pending, on the same reading
// ItemIsResolved gives a damaged file.
func ItemAwaitsOperator(item *Item) bool {
	if item.State != ItemPending {
		return false
	}
	if item.Kind != "open_question" && item.Kind != "decision" {
		return false
	}
	return item.Owner == ItemOwnerOperator || item.Owner == ""
}

// ItemTally is what one walk of a card's checklist items counts.
type ItemTally struct {
	// Blocking is how many items would refuse a claim right now, by
	// ItemBlocksClaim.
	Blocking int
	// AwaitingOperator is how many items ItemAwaitsOperator admits.
	AwaitingOperator int
}
