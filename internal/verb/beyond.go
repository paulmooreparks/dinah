package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/template"
)

// Add files a new card. It enters the first column of the ordered list with
// state ready, its identifier is claimed by mkdir of the hex directory, its
// journal opens with the created event, and the number it answers to is
// appended to the registry under the workbench lock, one past the highest
// number already there. A workbench still carrying its numbers in card
// frontmatter is refused rather than half-migrated, because the registry is
// the half of the format that workbench has not reached.
//
// A named column honours that column's capacity limit while a filing into the
// first column never does, because work has to be able to enter the bench and
// the intake station is where unstarted work is meant to pile up.
//
// Bench.Open tolerates a workbench whose live columns list has been emptied
// by every id going stranded, so check can diagnose it. Add refuses with
// contract.AddNeedsAColumn instead of reading the first column off an empty
// list, before anything about the request past its title and actor is
// checked, so the refusal is a pure read-only bail-out with nothing to
// clean up.
func (l *Library) Add(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	if req.Actor == "" {
		return l.refuse(req, nil, contract.NoOwner, "")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return l.refuse(req, nil, contract.Malformed, "title")
	}
	if len(l.Bench.Columns) == 0 {
		anchor := filepath.Join(l.Bench.Root, bench.WorkbenchAnchor)
		return l.refuse(req, nil, contract.AddNeedsAColumn, anchor)
	}
	destination := l.Bench.Columns[0]
	if req.Column != "" {
		named := l.Bench.ColumnByRef(req.Column)
		if named == nil {
			return l.refuse(req, nil, contract.UnknownColumn, req.Column)
		}
		reached, err := l.atCapacity(req, named)
		if err != nil {
			return l.FromError(req, err)
		}
		if reached {
			return l.refuse(req, nil, contract.AtCapacity, named.Ref())
		}
		destination = named
	}
	// The levels are admitted before an identifier is claimed, so a refused
	// filing leaves no card directory behind.
	levels := map[string]string{
		bench.SeverityField: strings.TrimSpace(req.Severity),
		bench.PriorityField: strings.TrimSpace(req.Priority),
	}
	if refusal := l.admitLevels(levels); refusal != nil {
		return l.refuseWith(req, nil, refusal.Name, refusal.Detail, refusal.Extra)
	}
	// A card being filed stores no declared field value, so a level whose
	// axis carries a condition is refused with the gate unset, before any
	// identifier is claimed. Filing, then writing the gate, then writing the
	// level is the route.
	for _, axis := range bench.LevelAxes {
		if levels[axis] == "" {
			continue
		}
		if refusal := l.inapplicable(nil, axis); refusal != nil {
			return l.refuseWith(req, nil, refusal.Name, refusal.Detail, refusal.Extra)
		}
	}
	// A filing that names a route is refused where the route does not carry
	// the column the card would land in, which covers both halves of the
	// question with one refusal: a --column the route drops, and a bare filing
	// into a workbench's first column that the route drops.
	//
	// Refusing at creation while permitting a route change on a live card is a
	// deliberate asymmetry. A new card has no history and stands nowhere, so
	// refusing costs the caller one corrected flag and prevents a card that is
	// off its route from its first moment, where a refusal on a live card
	// would block the legitimate act of re-routing work that has started.
	route := strings.TrimSpace(req.Route)
	if route != "" {
		if !l.Bench.DeclaresRoute(route) {
			return l.unknownRoute(req, nil, route)
		}
		carried := bench.RouteColumnsIn(l.Bench.Routes[route], l.Bench.Columns)
		if bench.RouteIndexOf(carried, destination) < 0 {
			return l.refuseWith(req, nil, contract.RouteOffColumn, destination.Ref(), map[string]string{
				"route": route,
			})
		}
		// A filing is a placement, so the operator's ruling on
		// dinah-542/decisions/6 reaches it as it reaches a route write: no
		// road may carry a card around a column the workbench reserves to its
		// operator before the card has passed it. A card that does not exist
		// yet has passed nothing, so the rule is read from the column it is
		// about to stand in, which is where a route write immediately after
		// the filing would read it too.
		arriving := &bench.Card{Column: destination.ID}
		if skipped := l.Bench.RouteSkipsOperatorColumn(arriving, route); skipped != nil {
			return l.refuseWith(req, nil, contract.RouteSkipsOperatorColumn, route, map[string]string{
				"column": skipped.Ref(),
			})
		}
	}
	// The three dates are admitted before an identifier is claimed, on the
	// terms the levels are, so a malformed one refuses the whole filing and
	// leaves no card directory behind.
	dates := map[string]string{
		bench.StartAfterField: strings.TrimSpace(req.StartAfter),
		bench.StartByField:    strings.TrimSpace(req.StartBy),
		bench.DueField:        strings.TrimSpace(req.Due),
	}
	for _, field := range bench.ScheduleFields {
		if dates[field] == "" {
			continue
		}
		if _, ok := bench.ParseDate(dates[field]); !ok {
			return l.refuse(req, nil, contract.Malformed, field)
		}
	}
	now := bench.Stamp(l.Now())
	// The workbench lock alone guarantees nothing about the mark a caller reads
	// after taking it. It stops two filings from writing at once, but a caller
	// holding a stale in-memory snapshot from before the lock was ever
	// contested would still mint a number another process already took.
	lock, err := bench.Acquire(l.Bench.Root, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	// A workbench below the format the registry arrived at has no registry to
	// allocate from: its numbers still live in card frontmatter, and a filing
	// there would write a line into a format the workbench does not declare.
	// The migration carries the workbench across first, and the refusal names
	// the command that runs it.
	if l.Bench.Format < bench.RegistryFormat {
		return l.refuse(req, nil, contract.NeedsNumberMigration, l.Bench.Root)
	}
	// The mark is read fresh from disk under the lock just taken, rather than
	// from whatever snapshot this caller opened with, so a long-lived caller
	// sitting on a stale mark cannot mint a number another process already
	// claimed since. See ReloadNumbers in internal/bench/numbers.go.
	l.Bench.ReloadNumbers()
	number, err := l.Bench.NextNumber()
	if err != nil {
		return l.FromError(req, err)
	}
	id, err := bench.ClaimID(l.Bench.CardsRoot(), l.Bench.HasIdentifier)
	if err != nil {
		return l.FromError(req, err)
	}
	dir := filepath.Join(l.Bench.CardsRoot(), id)
	// A creation takes no card lock, so the destination's sibling is read once
	// mkdir has claimed the identifier and before the anchor lands. Giving
	// the identifier up means giving up the directory too, since an empty
	// hex directory makes every listing on the bench fail.
	if holder, retiring := l.retiring(destination.ID); retiring {
		os.RemoveAll(dir)
		return l.refuse(req, nil, contract.Locked, holder)
	}
	fm := bench.NewFrontmatter()
	fm.Set("title", title)
	fm.Set("column", destination.ID)
	fm.Set("state", contract.StateReady)
	// Set appends, and state is the last key written above, so the pair
	// lands under it in the order Card.Save places it. An axis the filing
	// named no level for is written not at all, so absence stays absence
	// rather than becoming an empty value.
	for _, axis := range bench.LevelAxes {
		if levels[axis] != "" {
			fm.Set(axis, levels[axis])
		}
	}
	// The route lands beside the levels and on the same terms: a filing that
	// named none writes the key not at all, so absence stays absence.
	if route != "" {
		fm.Set(bench.RouteField, route)
	}
	for _, field := range bench.ScheduleFields {
		bench.SetScheduleDate(fm, field, dates[field])
	}
	if err := bench.WriteText(filepath.Join(dir, bench.CardAnchor), fm.Render(req.Text)); err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:      now,
		Event:   contract.EventCreated,
		Actor:   req.Acting(),
		Title:   title,
		To:      destination.ID,
		ToTitle: destination.Title,
	}
	if err := bench.AppendEvent(filepath.Join(dir, bench.JournalName), ev); err != nil {
		return l.FromError(req, err)
	}
	// The registry line lands after the card it names, so a crash between the
	// two leaves a card with no line rather than a line naming a card that was
	// never created. The first is a state check reports and the migration
	// repairs; the second is a number nobody can give back. The write is an
	// append-open followed by Sync on AppendEvent's terms, so a crash mid-line
	// can tear the final line alone and never the ones already in the file.
	if err := bench.AppendNumberLine(filepath.Join(l.Bench.Root, bench.CardNumbersName), number, id); err != nil {
		return l.FromError(req, err)
	}
	// The registry is read back from the file rather than extended in memory,
	// so the bench and the disk cannot disagree about what the new card
	// answers to, and the response below stamps the number it was filed under.
	l.Bench.ReloadNumbers()
	card, err := l.Bench.LoadCardIn(l.Bench.CardsRoot(), id)
	if err != nil {
		return l.FromError(req, err)
	}
	// A filing is an arrival at the destination, so the column's standing
	// items are minted here, after the created line and the registry line.
	// No lock is needed: the card is new and nobody else can name it yet.
	if err := l.mintStandingItems(req, card, destination, now); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, card)
	scheduleOrderWarning(response, card)
	return response
}

// Comment writes one comment below its holder, which is a card, a column, or
// one of a card's checklist items: an entity of its own carrying the
// timestamp and the author in frontmatter and the text as the body.
//
// A call carrying no text mints the entity with an empty body and journals
// the same commented line. That is the form an editor calls: the comment
// exists from the first keystroke, so nothing has to decide when an author
// has finished composing one, and an author who says nothing after all
// deletes the comment. A call carrying text keeps its present behaviour, so
// the one-shot path stays one call.
//
// What an abandoned draft costs is an entity and a journal line where there
// used to be nothing, and that is the price of the shape rather than an
// oversight. dinah check names an empty comment nothing designates, at
// cleanup severity, and names dinah delete as the remedy.
func (l *Library) Comment(req *Request) *Response {
	entity, refused := l.canComment(req)
	if refused != nil {
		return refused
	}
	now := bench.Stamp(l.Now())
	// The comment is its own entity, so its identifier needs no lock, but the
	// event lands in the nearest enclosing journal-bearing entity's journal.
	// That is the card's journal for a card comment and for an item comment,
	// and the workbench's for a column comment, because a column bears no
	// journal of its own. The write happens under that entity's own lock like
	// any other.
	lock, err := bench.Acquire(l.lockDirFor(entity), req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	comment, err := bench.AddComment(entity.Dir, req.Actor, now, req.Text)
	if err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:      now,
		Event:   contract.EventCommented,
		Actor:   req.Acting(),
		Comment: comment.ID,
	}
	// A commented line carries one locator naming the holder, and a line
	// carrying none means the holder is the journal's own entity. An item
	// comment carries the item, a column comment carries the column and its
	// title as of the write, and a card comment carries neither, sitting in
	// the journal of the card it hangs on.
	if entity.Kind == bench.KindItem {
		ev.Item = entity.ID
	}
	if entity.Kind == bench.KindColumn {
		ev.Column = entity.ID
		if column := l.Bench.Column(entity.ID); column != nil {
			ev.ColumnTitle = column.Title
		}
	}
	if err := bench.AppendEvent(l.journalFor(entity), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, entity.Card)
	response.Detail = l.commentRefOf(entity, comment)
	return response
}

// canComment runs every row Comment runs before it takes a lock, in Comment's
// order: the workbench has an operator, the declared harness is well formed,
// the reference is not blank, it resolves, the request names an owner, and
// the entity it reached mounts comments. It answers the entity or the refusal
// of the first row that fails. Comment and OfferActs both call it, so a row
// added here reaches the act and the offer together.
func (l *Library) canComment(req *Request) (*bench.EntityRef, *Response) {
	if l.Bench.Operator == "" {
		return nil, l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return nil, refused
	}
	// A blank reference resolves to the workbench under ResolveEntity, which
	// is right for attach and wrong here: this parameter names a card, a
	// column or a checklist item, never the workbench by omission, so the
	// check this verb has always run first, that the card exists, is run
	// before the reference is handed to the general resolver.
	if strings.TrimSpace(req.Card) == "" {
		return nil, l.refuse(req, nil, contract.UnknownCard, "")
	}
	entity, err := l.Bench.ResolveEntity(req.Card)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	if req.Actor == "" {
		return nil, l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	// Three kinds mount comments, a card, a column and a checklist item, and
	// every other kind is refused: an attachment, a comment, a workstream and
	// the workbench. It is the same question Attach already asks about
	// attachments. The kind is carried beside the reference so the caller
	// reads what the reference reached.
	if _, mounts := bench.MountOf(entity.Kind, bench.CommentsDir); !mounts {
		return nil, l.refuseWith(req, entity.Card, contract.NotCommentable, entity.Ref,
			map[string]string{"kind": entity.Kind, entity.Kind: entity.Ref})
	}
	return entity, nil
}

// commentRefOf composes what a person types to reach a comment that was just
// written, which is what the answer carries back.
//
// A reference rather than the identifier, because the identifier of a comment
// does not resolve: a comment is reached through the thing it hangs below, so
// `dinah path <12-hex>` answers unknown-card and every caller that took the
// answer and asked a second question of it was refused. The empty creation
// form exists so that something can open the file it just made, and it can
// only do that if what it is handed is an address.
//
// The holder's own reference is the resolver's, except below a card, where
// itemCanonicalRef composes the kind-narrowed spelling `dinah show` prints.
// Both resolve; the second is the one a person recognises.
//
// A position that cannot be counted leaves the identifier standing. It is not
// an address, but it names the entity that was made, and answering nothing at
// all would be worse.
func (l *Library) commentRefOf(entity *bench.EntityRef, comment *bench.Comment) string {
	holder := entity.Ref
	if entity.Kind == bench.KindItem && entity.Card != nil {
		if named, err := l.itemCanonicalRef(entity.Card, entity.ID); err == nil {
			holder = named
		}
	}
	if holder == "" {
		return comment.ID
	}
	ordinal, err := memberPosition(comment.Dir, bench.CommentAnchor)
	if err != nil || ordinal == 0 {
		return comment.ID
	}
	return commentRef(holder, ordinal)
}

// Attach records a file against the bench, a workstream, a column, a card or
// a comment. The entity carries the original filename, the description and the
// provenance, and the bytes alone sit in payload/ under their original name.
func (l *Library) Attach(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	// Replacing an attachment's bytes writes nothing below it and stays legal,
	// so one expression decides both the refusal and the branch that writes,
	// and the two cannot drift apart.
	replacing := req.Replace && entity.Kind == bench.KindAttachment
	if _, mounts := bench.MountOf(entity.Kind, bench.AttachmentsDir); !mounts && !replacing {
		return l.refuseWith(req, entity.Card, contract.NotAttachable, entity.Ref,
			map[string]string{"kind": entity.Kind, entity.Kind: entity.Ref})
	}
	if l.definitionAttachmentWrite(entity) && req.Actor != l.Bench.Operator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	if !bench.Exists(req.File) {
		return l.refuseWith(req, entity.Card, contract.UnknownPath, req.File, map[string]string{"file": req.File})
	}
	now := bench.Stamp(l.Now())
	// The new entity is written inside the target's directory and the event
	// is appended to the journal of the nearest enclosing journal-bearing
	// entity, so one acquisition covers both writes and nothing lands
	// before it is taken.
	lock, err := bench.Acquire(l.lockDirFor(entity), req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	if l.Interleave != nil {
		l.Interleave()
	}
	ev := bench.Event{TS: now, Actor: req.Acting()}
	if replacing {
		locateColumnAttachment(&ev, l.attachmentColumn(entity))
	} else if entity.Kind == bench.KindColumn {
		locateColumnAttachment(&ev, l.Bench.Column(entity.ID))
	}
	if replacing {
		attachment, err := bench.ReplaceAttachment(entity.Dir, req.File)
		if err != nil {
			return l.FromError(req, err)
		}
		ev.Event = contract.EventAttachmentReplaced
		ev.Attachment = attachment.ID
		ev.Filename = attachment.Filename
	} else {
		attachment, err := bench.AddAttachment(entity.Dir, req.File, req.Description, req.Actor)
		if err != nil {
			return l.FromError(req, err)
		}
		ev.Event = contract.EventAttached
		ev.Attachment = attachment.ID
		ev.Filename = attachment.Filename
	}
	if err := bench.AppendEvent(l.journalFor(entity), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, entity.Card)
	response.Detail = ev.Attachment
	return response
}

// Archive moves an entity's whole directory into the archive mirror at its
// own level, history and all. Listings, next and the capacity count ignore
// the archive by construction, so an archived card is out of the flow while
// its identifier still resolves for a link.
func (l *Library) Archive(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if entity.Kind == bench.KindWorkbench {
		return l.refuse(req, nil, contract.UnknownPath, req.Ref)
	}
	if l.operatorOnlyRemoval(entity) && req.Actor != l.Bench.Operator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	now := bench.Stamp(l.Now())
	journal := l.journalFor(entity)
	ev := bench.Event{TS: now, Event: contract.EventArchived, Actor: req.Acting(), Note: entity.ID}
	locateColumnAttachment(&ev, l.attachmentColumn(entity))
	act := &bench.StructuralAct{
		Dir:       entity.Dir,
		LockDir:   l.lockDirFor(entity),
		Op:        bench.OpArchive,
		Actor:     req.Actor,
		Now:       now,
		ColumnID:  columnSubject(entity),
		ColumnRef: columnRefSubject(entity),
		Record:    func() error { return bench.AppendEvent(journal, ev) },
	}
	if err := l.Bench.Run(act); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, nil)
	response.Detail = entity.ID
	return response
}

// Restore moves an entity's whole directory out of the archive mirror and
// back into the live half of the collection it was archived from, history
// and all. It is Archive read in the other direction, and it runs the same
// structural protocol under bench.OpRestore, which Bench.Run has carried
// since the format's concurrency section was written.
//
// The workbench guard Archive carries is not repeated. The resolver refuses
// the workbench under bench.ArchivedHalf before this verb sees it, and a
// reference naming a whole collection is refused by ResolveEntityIn itself.
//
// journalFor and lockDirFor read the entity's own directory, which under a
// restore is the mirror path, and that is where the entity's journal still
// is. The event is written at the source and arrives at the destination
// inside the directory that carries it.
func (l *Library) Restore(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	entity, err := l.Bench.ResolveEntityIn(bench.ArchivedHalf, req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if l.operatorOnlyTarget(entity) && req.Actor != l.Bench.Operator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	now := bench.Stamp(l.Now())
	journal := l.journalFor(entity)
	ev := bench.Event{TS: now, Event: contract.EventRestored, Actor: req.Acting(), Note: entity.ID}
	locateColumnAttachment(&ev, l.attachmentColumn(entity))
	act := &bench.StructuralAct{
		Dir:       entity.Dir,
		LockDir:   l.lockDirFor(entity),
		Op:        bench.OpRestore,
		Actor:     req.Actor,
		Now:       now,
		ColumnID:  columnSubject(entity),
		ColumnRef: columnRefSubject(entity),
		Record:    func() error { return bench.AppendEvent(journal, ev) },
	}
	if err := l.Bench.Run(act); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, nil)
	response.Detail = entity.ID
	return response
}

// halfFor is the resolution half a request names, which is the archive mirror
// when the request carries the archived flag and the live half otherwise. The
// four commands that take the flag ask this rather than each writing the
// condition out.
func halfFor(req *Request) bench.ResolutionHalf {
	if req.Archived {
		return bench.ArchivedHalf
	}
	return bench.LiveHalf
}

// Delete destroys an entity and the history inside it. The confirmation flag
// is required and there is no prompt, so the command behaves the same in a
// script and at a terminal.
//
// A deleted card's registry line is rewritten to the tombstone rather than
// removed, so the number stays allocated and the next filing cannot hand out
// a number a deleted card once answered to.
//
// A card another card's link names is deleted without refusal, because a
// reference never refuses an act; the dangling `to:` is what check reports
// afterwards.
func (l *Library) Delete(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if !req.Confirm {
		return l.refuse(req, entity.Card, contract.Unconfirmed, req.Ref)
	}
	if entity.Kind == bench.KindWorkbench {
		return l.refuse(req, nil, contract.UnknownPath, req.Ref)
	}
	if l.operatorOnlyRemoval(entity) && req.Actor != l.Bench.Operator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	designator, refused := l.admitCommentDeletion(req, entity)
	if refused != nil {
		return refused
	}
	now := bench.Stamp(l.Now())
	journal, ev := l.removalRecord(req, entity, now)
	act := &bench.StructuralAct{
		Dir:           entity.Dir,
		LockDir:       l.lockDirFor(entity),
		Op:            bench.OpDelete,
		Actor:         req.Actor,
		Now:           now,
		ColumnID:      columnSubject(entity),
		ColumnRef:     columnRefSubject(entity),
		WorkstreamID:  workstreamSubject(entity),
		WorkstreamRef: workstreamRefSubject(entity),
		Record: func() error {
			if err := bench.AppendEvent(journal, ev); err != nil {
				return err
			}
			if entity.Kind != bench.KindCard {
				return nil
			}
			return l.tombstoneNumber(entity.ID)
		},
	}
	if err := l.Bench.Run(act); err != nil {
		return l.FromError(req, err)
	}
	if entity.Kind == bench.KindCard {
		l.Bench.ReloadNumbers()
	}
	// The reopen lands after the deletion rather than before it, so a run
	// that fails to remove the directory leaves the item settled and its
	// answer standing, which is the state the refusal above protects. The
	// other order would unsettle an item whose answer is still on disk.
	if designator != nil {
		if refused := l.reopenForDeletion(req, designator); refused != nil {
			return refused
		}
	}
	response := l.ok(req, nil)
	response.Detail = entity.ID
	return response
}

// admitCommentDeletion answers the one question deleting a comment raises:
// whether a checklist item designates it as its answer of record. It reports
// the designating item where the deletion is to go ahead and reopen it, and a
// refusal where it is not.
//
// A designation names a comment of the very item that carries it, so the only
// item that can designate a comment is the one it hangs below, and the check
// is that item's own anchor rather than a walk of the card. Nothing else
// designates anything, so every other kind and every comment hanging below a
// card or a column passes straight through.
//
// Without --force the answer is a refusal naming the item, because an answer
// of record cannot be destroyed while it is still the answer. With --force the
// deletion goes ahead and reopens the item as part of the same act, and the
// authority is the reopen's: on an operator-owned item the forced form is the
// operator's alone, on the terms closeItem already refuses a terminal verb
// there. A force that did not respect that would be a way to unsettle an
// operator's ruling without being the operator.
func (l *Library) admitCommentDeletion(req *Request, entity *bench.EntityRef) (*designatedBy, *Response) {
	if entity.Kind != bench.KindComment || entity.Card == nil {
		return nil, nil
	}
	holder := filepath.Dir(filepath.Dir(entity.Dir))
	item, err := bench.LoadItem(holder)
	if err != nil || item.Resolution == "" {
		return nil, nil
	}
	// The stored value is the comment's own identifier, so the comparison is
	// against this entity's own directory name with no resolution step and
	// nothing to go stale. It used to resolve the stored reference and
	// compare directories, which is what a positional designation obliged it
	// to do.
	if item.Resolution != entity.ID {
		return nil, nil
	}
	// Composed before the refusal rather than after it, because the refusal
	// names the item too. It used to fill its item slot with the designation,
	// so the sentence read "is the answer of record for item
	// <a comment reference>", naming a comment where it said item, while the
	// value it wanted was computed a dozen lines further down.
	//
	// The reference is composed rather than taken from the item's own
	// identifier, because Reopen resolves what it is handed and a bare
	// identifier resolves to nothing.
	named, err := l.itemCanonicalRef(entity.Card, item.ID)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	if !req.Force {
		return nil, l.refuseWith(req, entity.Card, contract.NotDesignatable, entity.Ref, map[string]string{
			"item": named,
		})
	}
	if item.Owner == bench.ItemOwnerOperator && req.Actor != l.Bench.Operator {
		return nil, l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	// The reference is composed under the item's own canonical spelling
	// rather than taken from the resolver's, so the reason a reader meets in
	// the journal is the address that item's comments answer to.
	ordinal, err := memberPosition(entity.Dir, bench.CommentAnchor)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	return &designatedBy{item: named, designation: commentRef(named, ordinal)}, nil
}

// designatedBy is the item a comment being deleted is the answer of record
// for, and the reference that item carries for it.
//
// The designation is kept beside the item rather than recomposed, because it
// is the canonical spelling the settling stored and it is the only part of the
// deleted comment that survives the act. A reference the resolver happened to
// answer with would reach the same comment and read as a different address.
type designatedBy struct {
	// item is the reference Reopen is handed, which resolves.
	item string
	// designation is the comment's canonical reference, which the composed
	// reason names. It is composed for the refusal rather than read off the
	// item, because the item stores the comment's identifier and a reason
	// naming a bare identifier tells a later reader nothing they can type.
	designation string
}

// reopenForDeletion runs the reopen a forced deletion is, composing the reason
// rather than demanding one.
//
// A reopen takes a reason, which is prose, and the journal keeps it. The
// forced form composes one rather than demanding one. The sentence itself is
// the head's, because prose belongs to the layer that holds a catalog and this
// one holds none; what this layer guarantees is that a reason exists, so a
// caller reaching the library directly still lands a reopen the journal can
// read. What that fallback carries is the deleted comment's reference, which
// is the fact worth keeping either way. That reference is the only part of the deleted comment
// that survives, and it is prose in a prose field, so nothing has to tell two
// kinds of value apart.
func (l *Library) reopenForDeletion(req *Request, designator *designatedBy) *Response {
	routed := *req
	routed.Ref = designator.item
	routed.Reason = req.Reason
	if strings.TrimSpace(routed.Reason) == "" {
		routed.Reason = designator.designation
	}
	response := l.Reopen(&routed)
	if response.Outcome != contract.OutcomeOK {
		return response
	}
	return nil
}

// tombstoneNumber rewrites the first registry line claiming the identifier to
// the tombstone, which keeps the number allocated: NextNumber answers one
// past the registry's high-water mark, so removing the line instead would let
// the next filing hand out a number a deleted card once answered to. The
// rewrite goes through WriteNumberLines because it is a modification rather
// than an append, and it runs inside the structural act's Record callback,
// under the workbench lock the act holds first, so a filing appending a line
// at the same moment cannot have its line lost to the rewrite.
//
// A card no line claims is left alone, which is every card on a workbench
// below the registry's format: there is nothing to tombstone, and the number
// such a workbench reads from frontmatter is reissuable exactly as it is
// today. A later line claiming the same identifier stands, because a repeated
// identifier is the state check.card-number-repeated reports and an operator,
// not a deletion, is who resolves it.
func (l *Library) tombstoneNumber(id string) error {
	rewritten := make([]string, 0, len(l.Bench.Numbers.Lines))
	tombstoned := false
	for _, line := range l.Bench.Numbers.Lines {
		if !tombstoned && line.ID == id {
			rewritten = append(rewritten, strconv.Itoa(line.Number)+" -")
			tombstoned = true
			continue
		}
		rewritten = append(rewritten, line.Raw)
	}
	if !tombstoned {
		return nil
	}
	return bench.WriteNumberLines(filepath.Join(l.Bench.Root, bench.CardNumbersName), rewritten)
}

// Rename carries an attachment's payload under a new filename and rewrites
// the anchor's filename field to match. Cards, columns, comments, checklist
// items, and the workbench itself sit outside this verb, since their names
// travel by other acts.
//
// The reference resolves ahead of the precondition list, so a name a reader
// cannot find is met with the unknown-path refusal rather than something
// invented on the call's behalf. The card the attachment hangs from, when
// one does, carries the event, since an attachment's journal is not the
// journal of a verb to record history under.
func (l *Library) Rename(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	if entity.Kind != bench.KindAttachment {
		return l.refuseWith(req, entity.Card, contract.NotRenamable, entity.Ref, map[string]string{"kind": entity.Kind})
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if l.definitionAttachmentWrite(entity) && req.Actor != l.Bench.Operator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	if !bench.ValidAttachmentName(req.Value) {
		return l.refuse(req, entity.Card, contract.Malformed, "name")
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(l.lockDirFor(entity), req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	if l.Interleave != nil {
		l.Interleave()
	}
	before, after, err := bench.RenameAttachment(entity.Dir, req.Value)
	if err != nil {
		return l.FromError(req, err)
	}
	if before.Filename == after.Filename {
		response := l.ok(req, entity.Card)
		response.Detail = entity.ID
		return response
	}
	ev := bench.Event{
		TS:         now,
		Event:      contract.EventAttachmentRenamed,
		Actor:      req.Acting(),
		Attachment: after.ID,
		Filename:   after.Filename,
		From:       before.Filename,
	}
	locateColumnAttachment(&ev, l.attachmentColumn(entity))
	if err := bench.AppendEvent(l.journalFor(entity), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, entity.Card)
	response.Detail = entity.ID
	return response
}

// WorkbenchView is the workbench's own fields as a read reports them: the
// three a person wrote when the workbench was created, and nothing structural.
type WorkbenchView struct {
	// Title is the workbench's title.
	Title string `json:"title"`
	// Slug is the prefix every card reference in the workbench carries.
	Slug string `json:"slug"`
	// Operator is the owner reserved acts belong to.
	Operator string `json:"operator"`
}

// Field reads one field of the view by name, and answers the empty string for
// a name the view does not carry, which Workbench has already refused over.
func (v *WorkbenchView) Field(name string) string {
	switch name {
	case "title":
		return v.Title
	case "slug":
		return v.Slug
	case "operator":
		return v.Operator
	}
	return ""
}

// Workbench reports the workbench's own fields, which is what the bare
// `dinah workbench` listing prints. Reading is open to anybody, so no owner is
// required and no operator is asked for; every other read in the tool is open
// the same way.
//
// It answers the listing alone. One field of the workbench is read through
// GetField, which reaches every field of every kind through one grammar.
func (l *Library) Workbench(req *Request) (*WorkbenchView, error) {
	view := &WorkbenchView{
		Title:    l.Bench.Title,
		Slug:     l.Bench.Slug,
		Operator: l.Bench.Operator,
	}
	return view, nil
}

// admitLevels runs the two level checks over the axes a write named, in the
// order the check lists fix: every named axis is asked whether this workbench
// declares a set for it, and only then is every named value asked whether that
// set carries it. It answers nil when every named axis passes both.
//
// Each question is about the named axis alone and never about whether the
// workbench declares any levels at all. On a workbench declaring severity and
// no priority a severity write passes and a priority write refuses
// dinah.no-levels naming priority, which is what keeps the two axes
// independent as the format declares them.
func (l *Library) admitLevels(named map[string]string) *contract.Refusal {
	for _, axis := range bench.LevelAxes {
		if named[axis] == "" {
			continue
		}
		if l.Bench.Levels(axis) != nil {
			continue
		}
		return contract.RefuseWith(contract.NoLevels, axis, map[string]string{
			"axis":   axis,
			"anchor": filepath.Join(l.Bench.Root, bench.WorkbenchAnchor),
		})
	}
	for _, axis := range bench.LevelAxes {
		value := named[axis]
		if value == "" || l.Bench.Level(axis, value) != nil {
			continue
		}
		return contract.RefuseWith(contract.UnknownLevel, value, map[string]string{
			"axis":   axis,
			"levels": strings.Join(bench.LevelNames(l.Bench.Levels(axis)), ", "),
		})
	}
	return nil
}

// journalFor names the journal an event about an entity is recorded in, which
// is the nearest enclosing journal-bearing entity: a card's own journal for
// anything below a card, a workstream's own for the workstream and for an
// attachment hanging on one, and the bench's for everything else.
func (l *Library) journalFor(entity *bench.EntityRef) string {
	if entity.Card != nil {
		return entity.Card.JournalPath()
	}
	if entity.Kind == bench.KindWorkstream {
		return filepath.Join(entity.Dir, bench.JournalName)
	}
	// An attachment hanging on a workstream records in that workstream's
	// journal rather than in the workbench's, so one attachment's life is
	// read in one place. Without this arm attach lands on the workstream's
	// journal, because the entity it is given is the workstream, and rename,
	// archive, restore, delete and --replace land on the workbench's, because
	// the entity those are given is the attachment.
	if holder := l.attachmentWorkstream(entity); holder != nil {
		return filepath.Join(holder.Dir, bench.JournalName)
	}
	return l.Bench.JournalPath()
}

// attachmentHolderDir is the directory an attachment hangs from: two levels up
// from its own directory, past the archive segment an archived attachment sits
// under.
func attachmentHolderDir(dir string) string {
	holder := filepath.Dir(filepath.Dir(dir))
	if filepath.Base(holder) == bench.ArchiveDir {
		holder = filepath.Dir(holder)
	}
	return holder
}

// attachmentColumn reports the column an attachment hangs on, and nil for an
// attachment hanging anywhere else. It reads the attachment's directory: the
// holder is two levels up, and it is a column when that directory's parent is
// the workbench's columns root.
func (l *Library) attachmentColumn(entity *bench.EntityRef) *bench.Column {
	if entity.Kind != bench.KindAttachment {
		return nil
	}
	holder := attachmentHolderDir(entity.Dir)
	if !sameDir(filepath.Dir(holder), filepath.Join(l.Bench.Root, bench.ColumnsDir)) {
		return nil
	}
	return l.Bench.Column(filepath.Base(holder))
}

// attachmentWorkstream reports the workstream an attachment hangs on, and nil
// for an attachment hanging anywhere else. It reads the attachment's directory
// the way attachmentColumn reads it: the holder is two levels up, and it is a
// workstream when that directory's parent is the workbench's workstreams root,
// live or archived. Both roots are asked, because an archived workstream still
// holds its attachments and an event about one still belongs to it.
func (l *Library) attachmentWorkstream(entity *bench.EntityRef) *bench.Workstream {
	if entity.Kind != bench.KindAttachment {
		return nil
	}
	holder := attachmentHolderDir(entity.Dir)
	parent := filepath.Dir(holder)
	if !sameDir(parent, l.Bench.WorkstreamsRoot()) && !sameDir(parent, l.Bench.ArchivedWorkstreamsRoot()) {
		return nil
	}
	return l.Bench.Workstream(filepath.Base(holder))
}

// locateColumnAttachment writes the column locator onto a journal line about
// an attachment hanging on a column: the column's identifier and its title as
// of the write, the pair a comment left on a column already carries. A line
// about any other attachment is left as it is, because it sits in the journal
// of the card or the workstream it belongs to, or hangs on the workbench
// itself; in each of those the holder is the journal's own entity, so the
// journal the line sits in is what names it.
func locateColumnAttachment(ev *bench.Event, column *bench.Column) {
	if column == nil {
		return
	}
	ev.Column = column.ID
	ev.ColumnTitle = column.Title
}

// definitionAttachmentWrite reports whether a write to an entity's
// attachments, or to the attachment the entity is, is the operator's alone.
// An attachment takes the write authority of what it hangs on, so the answer
// is yes where that is a column or the workbench itself, whose own fields are
// the operator's, and no where it is a card, a comment or a workstream. An entity that is
// not an attachment is asked about as the target of a new one.
func (l *Library) definitionAttachmentWrite(entity *bench.EntityRef) bool {
	if entity.Kind != bench.KindAttachment {
		return entity.Kind == bench.KindColumn || entity.Kind == bench.KindWorkbench
	}
	holder := attachmentHolderDir(entity.Dir)
	if sameDir(holder, l.Bench.Root) {
		return true
	}
	// A column that is itself archived still holds its attachments, under
	// the archive mirror's columns root, and they are no less the operator's
	// there.
	parent := filepath.Dir(holder)
	live := filepath.Join(l.Bench.Root, bench.ColumnsDir)
	return sameDir(parent, live) || sameDir(parent, l.Bench.ArchivedColumnsRoot())
}

// operatorOnlyTarget reports whether restoring an entity is the operator's
// alone: a column, or an attachment hanging on a column or on the workbench.
//
// Restore alone consults it now. Archiving and deleting go through
// operatorOnlyRemoval below, which is this plus the three clauses that
// protect a checklist item.
func (l *Library) operatorOnlyTarget(entity *bench.EntityRef) bool {
	switch entity.Kind {
	case bench.KindColumn:
		return true
	case bench.KindAttachment:
		return l.definitionAttachmentWrite(entity)
	}
	return false
}

// operatorOnlyRemoval reports whether archiving or deleting an entity is the
// operator's alone. It is operatorOnlyTarget plus the protected checklist
// item, which is an item any of three things is true of.
//
// Everything this card built protected an item from being rewritten and
// nothing protected it from being removed. Agent Design Review walked a card
// into a done column past an acceptance criterion standing at failed by
// archiving the item, and emptied the operator's own queue twice over, once by
// archiving a question stamped for him and once by deleting one. Archiving
// changes no state at all, which is why the sweeps that asked what can change
// an item's state all missed it: it takes the item away, and an item outside
// the live set has no state for any rule to read.
//
// The three clauses. An acceptance criterion is protected whatever its state
// and whatever its owner, which is Withdraw's kind guard applied to these two
// acts. An item whose owner key reads operator is protected whatever its kind,
// which makes that stamp survive removal as well as rewriting. An item
// standing at waived is protected whatever its kind, because a waiver is a
// permission the operator granted and removing the item destroys the record of
// it. A failed item needs no clause of its own, since fail closes acceptance
// criteria alone and the first clause already covers them.
//
// A withdrawn item is left removable by anybody, and that asymmetry is
// deliberate. A waiver is what let a card past a finding, so destroying it
// destroys the justification for work that has already travelled; a withdrawal
// records that a question stopped applying, which is ordinary bookkeeping
// anybody was entitled to perform in the first place, and reserving its
// removal would reserve the tidying of exactly the items dinah-472 was filed
// to make tidyable. Where a withdrawn item is also an acceptance criterion, which
// is the case where the record is load-bearing, the first clause covers it.
//
// Restore is not reserved, because restoring an item puts it back into the
// live set and can only re-impose a hold. That is the reasoning Reopen was
// built on, and it is sound here for the reason it stopped being sound there:
// nothing composes with a restore to reach a removal.
//
// A standing criterion-retirement grant admits neither act. Tidying under a
// grant means retiring an item with a recorded reason, so that the item, its
// text and the reason a person gave all stay on the card and stay readable;
// removing an item answers a different question and the operator answers that
// one himself.
func (l *Library) operatorOnlyRemoval(entity *bench.EntityRef) bool {
	if l.operatorOnlyTarget(entity) {
		return true
	}
	if entity.Kind != bench.KindItem {
		return false
	}
	item, err := bench.LoadItem(entity.Dir)
	if err != nil {
		// An item whose anchor will not open cannot be shown to be
		// unprotected, and the safe direction here is the reserved one: a
		// damaged file is dinah check's finding rather than a licence to
		// remove the item nobody can read.
		return true
	}
	return item.Kind == criterionKind ||
		item.Owner == bench.ItemOwnerOperator ||
		item.State == bench.ItemWaived
}

// lockDirFor names the directory whose lock covers a write about an entity,
// which is the same nearest enclosing journal-bearing entity journalFor
// names, so the write and the event land on one side of one acquisition.
func (l *Library) lockDirFor(entity *bench.EntityRef) string {
	if entity.Card != nil {
		return entity.Card.Dir
	}
	if entity.Kind == bench.KindWorkstream {
		return entity.Dir
	}
	if holder := l.attachmentWorkstream(entity); holder != nil {
		return holder.Dir
	}
	return l.Bench.Root
}

// retiring names the actor retiring a column, when a structural act's sibling
// stands beside that column's directory. It is what a write storing a card's
// column reads before it stores one, so a card cannot enter a station whose
// retirement is already in flight.
func (l *Library) retiring(columnID string) (string, bool) {
	dir := filepath.Join(l.Bench.Root, bench.ColumnsDir, columnID)
	path := bench.SiblingPath(dir)
	if path == "" || !bench.Exists(path) {
		return "", false
	}
	return bench.LockHolder(path), true
}

// columnSubject names the column a structural act is retiring, and is empty for
// an act on any other kind. It is what arms the occupancy scan the act runs
// once its own sibling exists.
func columnSubject(entity *bench.EntityRef) string {
	if entity.Kind != bench.KindColumn {
		return ""
	}
	return entity.ID
}

// columnRefSubject is ColumnRef's own reading of the same question columnSubject
// answers for ColumnID, so the two stay paired and StructuralAct.ColumnRef's
// documented invariant, empty exactly when ColumnID is, holds by construction
// rather than by every entity kind but column happening to carry no Ref today.
func columnRefSubject(entity *bench.EntityRef) string {
	if entity.Kind != bench.KindColumn {
		return ""
	}
	return entity.Ref
}

// workstreamSubject names the workstream a deletion is removing, and is empty
// for an act on any other kind. It is what arms the membership scan the act
// runs once its own sibling exists. Archiving passes it nothing, because a
// workstream cards still belong to is the ordinary thing to archive.
func workstreamSubject(entity *bench.EntityRef) string {
	if entity.Kind != bench.KindWorkstream {
		return ""
	}
	return entity.ID
}

// workstreamRefSubject is WorkstreamRef's own reading of the same question
// workstreamSubject answers for WorkstreamID, paired the same way
// columnRefSubject pairs with columnSubject, so StructuralAct.WorkstreamRef's
// documented invariant, empty exactly when WorkstreamID is, holds by
// construction rather than by every entity kind but workstream happening to
// carry no Ref today.
func workstreamRefSubject(entity *bench.EntityRef) string {
	if entity.Kind != bench.KindWorkstream {
		return ""
	}
	return entity.Ref
}

// removalRecord composes the event a deletion is recorded by and names the
// journal it goes to, which has to be one that survives the entity.
//
// Deleting a card destroys the journal inside it, so the record goes to the
// bench's, carrying the identifier and the title as of the event. A deleted
// attachment keeps the attachment event it has always carried.
func (l *Library) removalRecord(req *Request, entity *bench.EntityRef, now string) (string, bench.Event) {
	ev := bench.Event{TS: now, Actor: req.Acting(), Event: contract.EventDeleted, Note: entity.ID}
	if entity.Kind == bench.KindAttachment {
		ev.Event = contract.EventAttachmentRemoved
		ev.Attachment = entity.ID
		if attachment, err := bench.LoadAttachment(entity.Dir); err == nil {
			ev.Filename = attachment.Filename
		}
		locateColumnAttachment(&ev, l.attachmentColumn(entity))
		return l.journalFor(entity), ev
	}
	ev.Title = l.titleOfEntity(entity)
	// A deletion is the one removal that destroys what it removed, so the
	// line has to say what went away rather than only that something with an
	// identifier did. An item's text is what it required and its kind is
	// which rules it was judged under, and a reader of the journal meeting
	// this line has no other source for either.
	if entity.Kind == bench.KindItem {
		if item, err := bench.LoadItem(entity.Dir); err == nil {
			ev.Kind = item.Kind
		}
	}
	// Deleting a card or a workstream destroys the journal inside it, so the
	// record goes to the bench's, carrying the identifier and the title as
	// of the event.
	if entity.Kind == bench.KindCard || entity.Kind == bench.KindWorkstream {
		return l.Bench.JournalPath(), ev
	}
	return l.journalFor(entity), ev
}

// titleOfEntity is what a person called the entity at the moment it was
// deleted, so the bench's history reads without resolving anything.
func (l *Library) titleOfEntity(entity *bench.EntityRef) string {
	if entity.Kind == bench.KindCard && entity.Card != nil {
		return entity.Card.Title
	}
	// An item is what a person called it too: the judgement it records is its
	// text, and a journal line saying that something with an identifier went
	// away says nothing about what that thing required.
	if entity.Kind == bench.KindItem {
		item, err := bench.LoadItem(entity.Dir)
		if err != nil {
			return ""
		}
		return item.Text
	}
	if entity.Kind == bench.KindWorkstream {
		if workstream := l.Bench.Workstream(entity.ID); workstream != nil {
			return workstream.Title
		}
		return ""
	}
	if column := l.Bench.Column(entity.ID); column != nil {
		return column.Title
	}
	return ""
}

// Init creates a bench, optionally from a template or another bench's
// interchange form, inside a fresh directory of the .dinah container under
// root. It returns the directory it wrote to.
//
// override is --workbench or DINAH_WORKBENCH as the session resolved it, and
// overrideSource is which of the two answered, passed through so Init can
// refuse it by name rather than silently discard it. Every other verb reads
// --workbench as the path to an existing workbench to open; Init has no
// existing workbench to open, so honouring the flag here would give it a
// second, contradictory meaning depending on which command sits next to it.
// Init refuses instead, whatever the flag's value and whether or not a root
// argument was also given.
//
// The refusal stays aimed at a bare workbench.md at root rather than at the
// container, because benchIn resolves a recognized one before it ever looks
// at that directory's container, so a bench written into the container
// beside it would sit where the climbing search can never reach it. It
// fires only when that bare file is a recognized Dinah workbench
// (bench.AnchorRecognized): a file sharing the name but carrying none of
// Dinah's frontmatter keys is passed over by the discovery walk exactly as
// it is everywhere else, so a container written beside it stays reachable
// and init proceeds rather than refusing over a file it never writes to.
// here is --here as the caller passed it. Its absence is what makes Init
// refuse a directory that already exists and already holds something: a
// directory init did not create, the way an existing, unrelated project's
// checkout does. A directory that does not yet exist, or exists and is
// empty, is unaffected either way, since os.MkdirAll inside
// bench.Instantiate creates it regardless and there is nothing in it to
// clobber.
func Init(root, slug, operator, source, override, overrideSource string, here bool) (string, error) {
	if override != "" {
		spelling := "--workbench"
		if overrideSource == bench.SourceEnvironment {
			spelling = "DINAH_WORKBENCH"
		}
		return "", contract.RefuseWith(contract.WorkbenchNotApplicable, override, map[string]string{"source": spelling})
	}
	anchor := filepath.Join(root, bench.WorkbenchAnchor)
	recognized, err := bench.AnchorRecognized(anchor)
	if err != nil {
		return "", contract.Refuse(contract.UnreadableBench, anchor)
	}
	if recognized {
		return "", contract.Refuse(contract.Exists, root)
	}
	if !here {
		entries, err := os.ReadDir(root)
		if err == nil && len(entries) > 0 {
			return "", contract.Refuse(contract.DirectoryNotEmpty, root)
		}
	}
	definition, err := readSource(root, source)
	if err != nil {
		return "", err
	}
	container := filepath.Join(root, bench.UserBaseName)
	id, err := bench.ClaimWorkbenchID(container)
	if err != nil {
		return "", err
	}
	written := filepath.Join(container, id)
	if err := bench.Instantiate(written, slug, operator, definition); err != nil {
		return "", contract.With(err, "file", source)
	}
	return written, nil
}

// readSource reads the definition a new bench is instantiated from: an
// interchange file, another bench's directory, or the default flow when the
// caller named no source.
func readSource(root, source string) (*bench.Definition, error) {
	if source == "" {
		return defaultDefinition(filepath.Base(root)), nil
	}
	if template.Known(source) {
		return template.Definition(source)
	}
	if bench.Exists(filepath.Join(source, bench.WorkbenchAnchor)) {
		// A template is read through the uncontained opener, because a
		// template is a definition rather than a workbench: Extract writes one
		// to whatever path a caller names, it holds no cards and no journal,
		// and nothing ever serves it. Holding it to the rule that says where a
		// workbench lives would refuse every template this tool has ever
		// written.
		opened, err := bench.OpenUncontained(source)
		if err != nil {
			return nil, err
		}
		data, err := opened.Export()
		if err != nil {
			return nil, err
		}
		return bench.ReadDefinition(data)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, contract.With(contract.Refuse(contract.UnknownPath, source), "file", source)
	}
	definition, readErr := bench.ReadDefinition(data)
	return definition, contract.With(readErr, "file", source)
}

// defaultDefinition is the flow a bench created from nothing carries: one
// intake station, one working station and one done station, which is the
// smallest flow the contract's own vocabulary can express.
func defaultDefinition(title string) *bench.Definition {
	columns := []map[string]json.RawMessage{
		columnMember("intake", "Intake", contract.KindIntake),
		columnMember("doing", "Doing", contract.KindWork),
		columnMember("done", "Done", contract.KindDone),
	}
	return &bench.Definition{
		Object:  map[string]json.RawMessage{},
		Title:   title,
		Profile: bench.ProfileVersion,
		Columns: columns,
	}
}

// columnMember builds one element of the default flow's columns array. The
// identifiers are minted at instantiation, since these names are not hex.
func columnMember(id, title, kind string) map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"id":    json.RawMessage(`"` + id + `"`),
		"title": json.RawMessage(`"` + title + `"`),
		"kind":  json.RawMessage(`"` + kind + `"`),
	}
}

// WorkstreamView is a workstream as a response carries it.
type WorkstreamView struct {
	// ID is the workstream's 12-hex identifier.
	ID string `json:"id"`
	// Ref is what a person types to reach the workstream: the kind's own
	// prefix, then the slug where the workstream carries one and the
	// identifier otherwise. It is never empty, because Workstream.Ref falls
	// back to the identifier.
	Ref string `json:"ref"`
	// Slug is the short handle, absent on a workstream carrying none.
	Slug string `json:"slug,omitempty"`
	// Title is what a person calls it, absent on one the adoption repair
	// created and nobody has named.
	Title string `json:"title,omitempty"`
	// Status is the open value a person reads, which Dinah never acts on.
	Status string `json:"status,omitempty"`
	// Cards is how many live cards belong to it, derived by walking them.
	Cards int `json:"cards"`
}

// Field reads one field of the view by name, and answers the empty string for
// a name the view does not carry, which Workstream has already refused over.
func (v *WorkstreamView) Field(name string) string {
	switch name {
	case "title":
		return v.Title
	case "slug":
		return v.Slug
	case "status":
		return v.Status
	}
	return ""
}

// WorkstreamListing is every live workstream of the workbench.
type WorkstreamListing struct {
	// Workstreams are the workstreams in creation order.
	Workstreams []WorkstreamView `json:"workstreams"`
}

// workstreamView renders a workstream for a response, with the member count
// read off a map the caller derived once for the whole listing.
func workstreamView(workstream *bench.Workstream, counts map[string]int) WorkstreamView {
	view := WorkstreamView{
		ID:     workstream.ID,
		Ref:    workstream.Ref(),
		Slug:   workstream.Slug,
		Title:  workstream.Title,
		Status: workstream.Status,
		Cards:  counts[workstream.ID],
	}
	return view
}

// Workstreams reports every live workstream of the workbench with the number
// of live cards belonging to each. Reading is open to anybody, so no owner is
// required and no operator is asked for, the way every other read is.
func (l *Library) Workstreams() (*WorkstreamListing, error) {
	counts, err := l.Bench.WorkstreamCounts()
	if err != nil {
		return nil, err
	}
	workstreams, err := l.Bench.Workstreams()
	if err != nil {
		return nil, err
	}
	listing := &WorkstreamListing{Workstreams: []WorkstreamView{}}
	for _, workstream := range workstreams {
		listing.Workstreams = append(listing.Workstreams, workstreamView(workstream, counts))
	}
	return listing, nil
}

// NewWorkstream creates a workstream from a title, and from the slug the
// caller may name alongside it, and opens its journal with the created event.
//
// The title is required and nothing else is, which is Add's own list: filing a
// grouping is filing work, so it asks for an owner rather than for the
// operator. The workbench's own lock covers the write, because the identifier,
// the creation ordinal and the slug collision scan are all read against the
// collection as a whole.
//
// The slug is optional and is what lets a caller finish provisioning the
// entity it is creating in one act rather than two. The row was added when a
// workstream field write was the operator's alone, so a caller that was not
// the operator could not correct a derived slug afterwards and was left
// holding a half-provisioned entity it was not permitted to finish. That
// restriction went on dinah-582, and the argument from it is history; the
// slug stays because provisioning in one act is still better than two.
//
// The grammar check sits between the title and the owner, which is where
// SetField puts "the value is present and well formed" relative to the entity
// the reference names. It refuses a malformed slug
// naming the field rather than the offending value, because the field is what
// a caller can act on. A slug a live workstream already carries is
// NewWorkstream's own row, raised there because the collection scan that
// answers it is that function's.
func (l *Library) NewWorkstream(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	title := strings.TrimSpace(req.Workstream)
	if title == "" {
		return l.refuse(req, nil, contract.Malformed, "title")
	}
	slug := strings.TrimSpace(req.Slug)
	if slug != "" && !bench.ValidColumnSlug(slug) {
		return l.refuse(req, nil, contract.Malformed, bench.SlugField)
	}
	if req.Actor == "" {
		return l.refuse(req, nil, contract.NoOwner, "")
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(l.Bench.Root, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	if l.Interleave != nil {
		l.Interleave()
	}
	workstream, err := l.Bench.NewWorkstream(title, slug)
	if err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:    now,
		Event: contract.EventCreated,
		Actor: req.Acting(),
		Title: title,
	}
	if err := bench.AppendEvent(workstream.JournalPath(), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, nil)
	view := workstreamView(workstream, nil)
	response.Workstream = &view
	response.Detail = workstream.ID
	return response
}

// AcceptDivergence ratifies a comment body somebody edited outside the tool.
//
// It takes no text, and that is the whole of what distinguishes it from a
// write. The two acts mean different things: ratifying says the body somebody
// typed is now the record, and writing says here is the record instead. An
// operator who wants to do both does two acts, and the journal records them as
// two.
//
// What it does is re-stamp the digest from the body as it stands and journal
// that a divergence was accepted and by whom. Once ratified the comment is no
// longer diverged, so an ordinary write works again. A comment that is not
// diverged is accepted all the same and answers ok, on the terms a field write
// storing the value already there answers ok: the caller asked for a state and
// the state is what they get.
func (l *Library) AcceptDivergence(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if entity.Kind != bench.KindComment {
		return l.refuse(req, entity.Card, contract.UnknownPath, req.Ref)
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(l.lockDirFor(entity), req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	fm, body, err := bench.ReadCommentAnchor(entity.Dir)
	if err != nil {
		return l.FromError(req, err)
	}
	if err := bench.WriteCommentAnchor(entity.Dir, fm, body); err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:      now,
		Event:   contract.EventDivergenceAccepted,
		Actor:   req.Acting(),
		Comment: entity.ID,
		Note:    entity.ID,
	}
	if err := bench.AppendEvent(l.journalFor(entity), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, entity.Card)
	response.Detail = entity.ID
	return response
}

// RecordCommentEdit is the return half of dinah edit on a comment: the head
// opens the file in the author's editor, and this decides what, if anything,
// that edit means.
//
// It claims nothing about when the editor returned, and that is the point.
// runEdit calls Run, and a GUI editor that hands the file to an
// already-running instance returns at once, so an implementation asserting the
// author had finished would attribute an edit that had not happened yet. What
// the tool has is the body before and the body after, and it says only what
// those two support.
//
// Three outcomes, decided against the digest the anchor carries:
//
//   - The body agrees with the recorded digest. Nothing is written and nothing
//     is journalled. This is the answer both when the author changed nothing
//     and when the editor returned before the author started, and the tool
//     cannot tell those apart, which is exactly why it must do the same thing
//     in both. It is also the answer when an author restored a diverged body
//     by hand, which is the remedy that needs no command at all.
//   - The body disagrees with the recorded digest and already disagreed before
//     the editor opened it. Somebody else's edit is standing in the file, and
//     this author's own change cannot be told from it, so the write is refused
//     rather than attributed. dinah accept-divergence settles what the record
//     is, and then an ordinary edit works again.
//   - The body disagrees and did not before. The author finished before the
//     editor returned, which is now a fact rather than an assumption. The new
//     digest is recorded and the edit is journalled as theirs.
//
// An author still typing when the editor returns falls into the first case,
// and their edit is caught later by dinah check like any other hand edit.
// Nothing is lost and nothing is asserted that was not observed.
func (l *Library) RecordCommentEdit(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
	}
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if entity.Kind != bench.KindComment {
		return l.refuse(req, entity.Card, contract.UnknownPath, req.Ref)
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(l.lockDirFor(entity), req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	fm, body, err := bench.ReadCommentAnchor(entity.Dir)
	if err != nil {
		return l.FromError(req, err)
	}
	// Whether this author changed anything is asked first, and a no ends the
	// run whatever the header says. The spec's first bullet is unconditional:
	// an unchanged body does nothing and journals nothing. Asking about a
	// divergence ahead of it made `dinah edit` on a diverged comment answer a
	// refusal for an edit that never happened, which told the reader nothing
	// they could act on and nothing dinah check would not have told them.
	stored := fm.Value(bench.CommentDigestField)
	standing := bench.CommentDigest(body)
	if standing == req.PriorDigest || (stored != "" && stored == standing) {
		response := l.ok(req, entity.Card)
		response.Detail = entity.ID
		return response
	}
	// The body did change under this author's hand, so there is an edit to
	// attribute. A comment that was already diverged when edit opened it is
	// where that attribution would be wrong, because this author's change
	// cannot be told from the one already standing in the file.
	if stored != "" && stored != req.PriorDigest {
		return l.refuse(req, entity.Card, contract.CommentBodyDiverged, entity.Ref)
	}
	if err := bench.WriteCommentAnchor(entity.Dir, fm, body); err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:    now,
		Event: contract.EventCommentUpdated,
		Actor: req.Acting(),
		Field: bench.BodyField,
		Note:  entity.ID,
	}
	if err := bench.AppendEvent(l.journalFor(entity), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, entity.Card)
	response.Detail = entity.ID
	return response
}

// CommentBodyDigest reports the digest of a comment's body as it stands, for
// a caller that has to observe it before handing the file to something else.
// It is dinah edit's own need and nobody else's, which is why it reads rather
// than resolving: the caller has already resolved the entity.
func CommentBodyDigest(dir string) (string, error) {
	_, body, err := bench.ReadCommentAnchor(dir)
	if err != nil {
		return "", err
	}
	return bench.CommentDigest(body), nil
}
