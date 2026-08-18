package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Add files a new card. It enters the first state of the ordered list with
// substate ready, its identifier is claimed by mkdir of the hex directory,
// and its journal opens with the created event.
//
// A named state honours that state's capacity limit while a filing into the
// first state never does, because work has to be able to enter the bench and
// the intake station is where unstarted work is meant to pile up.
//
// Bench.Open tolerates a workbench whose live states list has been emptied
// by every id going stranded, so check can diagnose it. Add refuses with
// contract.AddNeedsAState instead of reading the first state off an empty
// list, before anything about the request past its title and actor is
// checked, so the refusal is a pure read-only bail-out with nothing to
// clean up.
func (l *Library) Add(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if req.Actor == "" {
		return l.refuse(req, nil, contract.NoOwner, "")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return l.refuse(req, nil, contract.Malformed, "title")
	}
	if len(l.Bench.States) == 0 {
		anchor := filepath.Join(l.Bench.Root, bench.WorkbenchAnchor)
		return l.refuse(req, nil, contract.AddNeedsAState, anchor)
	}
	destination := l.Bench.States[0]
	if req.State != "" {
		named := l.Bench.StateByRef(req.State)
		if named == nil {
			return l.refuse(req, nil, contract.UnknownState, req.State)
		}
		reached, err := l.atCapacity(named)
		if err != nil {
			return l.FromError(req, err)
		}
		if reached {
			return l.refuse(req, nil, contract.AtCapacity, named.Ref())
		}
		destination = named
	}
	number := l.Bench.NextNumber()
	id, err := bench.ClaimID(l.Bench.CardsRoot(), l.Bench.HasIdentifier)
	if err != nil {
		return l.FromError(req, err)
	}
	dir := filepath.Join(l.Bench.CardsRoot(), id)
	// A creation takes no lock, so the destination's sibling is read once
	// mkdir has claimed the identifier and before the anchor lands. Giving
	// the identifier up means giving up the directory too, since an empty
	// hex directory makes every listing on the bench fail.
	if holder, retiring := l.retiring(destination.ID); retiring {
		os.RemoveAll(dir)
		return l.refuse(req, nil, contract.Locked, holder)
	}
	fm := bench.NewFrontmatter()
	fm.Set("title", title)
	fm.Set("number", strconv.Itoa(number))
	fm.Set("state", destination.ID)
	fm.Set("substate", contract.SubstateReady)
	if err := bench.WriteText(filepath.Join(dir, bench.CardAnchor), fm.Render(req.Text)); err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:      bench.Stamp(l.Now()),
		Event:   contract.EventCreated,
		Actor:   req.Actor,
		Title:   title,
		To:      destination.ID,
		ToTitle: destination.Title,
	}
	if err := bench.AppendEvent(filepath.Join(dir, bench.JournalName), ev); err != nil {
		return l.FromError(req, err)
	}
	card, err := bench.LoadCard(l.Bench.CardsRoot(), id)
	if err != nil {
		return l.FromError(req, err)
	}
	return l.ok(req, card)
}

// Comment records a comment on a card: an entity of its own carrying the
// timestamp and the author in frontmatter and the text as the body.
func (l *Library) Comment(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	found, err := l.Bench.ResolveCard(req.Card)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, found.Card, contract.NoOwner, "")
	}
	if strings.TrimSpace(req.Text) == "" {
		return l.refuse(req, found.Card, contract.Malformed, "text")
	}
	now := bench.Stamp(l.Now())
	// The comment is its own entity, so its identifier needs no lock, but the
	// event lands in the card's journal and that journal belongs to the card,
	// so the write happens under the card's own lock like any other.
	lock, err := bench.Acquire(found.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	comment, err := bench.AddComment(found.Card.Dir, req.Actor, now, req.Text)
	if err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:      now,
		Event:   contract.EventCommented,
		Actor:   req.Actor,
		Comment: comment.ID,
	}
	if err := bench.AppendEvent(found.Card.JournalPath(), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, found.Card)
	response.Detail = comment.ID
	return response
}

// Attach records a file against the bench, a state, a card or a comment. The
// entity carries the original filename, the description and the provenance,
// and the bytes alone sit in payload/ under their original name.
func (l *Library) Attach(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if !bench.Exists(req.File) {
		return l.refuse(req, entity.Card, contract.UnknownPath, req.File)
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
	ev := bench.Event{TS: now, Actor: req.Actor}
	if req.Replace && entity.Kind == "attachment" {
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
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if entity.Kind == "workbench" {
		return l.refuse(req, nil, contract.UnknownPath, req.Ref)
	}
	now := bench.Stamp(l.Now())
	journal := l.journalFor(entity)
	ev := bench.Event{TS: now, Event: contract.EventArchived, Actor: req.Actor, Note: entity.ID}
	act := &bench.StructuralAct{
		Dir:      entity.Dir,
		LockDir:  l.lockDirFor(entity),
		Op:       bench.OpArchive,
		Actor:    req.Actor,
		Now:      now,
		StateID:  stateSubject(entity),
		StateRef: entity.Ref,
		Record:   func() error { return bench.AppendEvent(journal, ev) },
	}
	if err := l.Bench.Run(act); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, nil)
	response.Detail = entity.ID
	return response
}

// Delete destroys an entity and the history inside it. The confirmation flag
// is required and there is no prompt, so the command behaves the same in a
// script and at a terminal.
//
// A card another card's link names is deleted without refusal, because a
// reference never refuses an act; the dangling `to:` is what check reports
// afterwards.
func (l *Library) Delete(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
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
	if entity.Kind == "workbench" {
		return l.refuse(req, nil, contract.UnknownPath, req.Ref)
	}
	now := bench.Stamp(l.Now())
	journal, ev := l.removalRecord(entity, req.Actor, now)
	act := &bench.StructuralAct{
		Dir:      entity.Dir,
		LockDir:  l.lockDirFor(entity),
		Op:       bench.OpDelete,
		Actor:    req.Actor,
		Now:      now,
		StateID:  stateSubject(entity),
		StateRef: entity.Ref,
		Record:   func() error { return bench.AppendEvent(journal, ev) },
	}
	if err := l.Bench.Run(act); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, nil)
	response.Detail = entity.ID
	return response
}

// journalFor names the journal an event about an entity is recorded in, which
// is the nearest enclosing journal-bearing entity: a card's own journal for
// anything below a card, and the bench's for everything else.
func (l *Library) journalFor(entity *bench.EntityRef) string {
	if entity.Card != nil {
		return entity.Card.JournalPath()
	}
	return l.Bench.JournalPath()
}

// lockDirFor names the directory whose lock covers a write about an entity,
// which is the same nearest enclosing journal-bearing entity journalFor
// names, so the write and the event land on one side of one acquisition.
func (l *Library) lockDirFor(entity *bench.EntityRef) string {
	if entity.Card != nil {
		return entity.Card.Dir
	}
	return l.Bench.Root
}

// retiring names the actor retiring a state, when a structural act's sibling
// stands beside that state's directory. It is what a write storing a card's
// state reads before it stores one, so a card cannot enter a station whose
// retirement is already in flight.
func (l *Library) retiring(stateID string) (string, bool) {
	dir := filepath.Join(l.Bench.Root, bench.StatesDir, stateID)
	path := bench.SiblingPath(dir)
	if path == "" || !bench.Exists(path) {
		return "", false
	}
	return bench.LockHolder(path), true
}

// stateSubject names the state a structural act is retiring, and is empty for
// an act on any other kind. It is what arms the occupancy scan the act runs
// once its own sibling exists.
func stateSubject(entity *bench.EntityRef) string {
	if entity.Kind != "state" {
		return ""
	}
	return entity.ID
}

// removalRecord composes the event a deletion is recorded by and names the
// journal it goes to, which has to be one that survives the entity.
//
// Deleting a card destroys the journal inside it, so the record goes to the
// bench's, carrying the identifier and the title as of the event. A deleted
// attachment keeps the attachment event it has always carried.
func (l *Library) removalRecord(entity *bench.EntityRef, actor, now string) (string, bench.Event) {
	ev := bench.Event{TS: now, Actor: actor, Event: contract.EventDeleted, Note: entity.ID}
	if entity.Kind == "attachment" {
		ev.Event = contract.EventAttachmentRemoved
		ev.Attachment = entity.ID
		if attachment, err := bench.LoadAttachment(entity.Dir); err == nil {
			ev.Filename = attachment.Filename
		}
		return l.journalFor(entity), ev
	}
	ev.Title = l.titleOfEntity(entity)
	if entity.Kind == "card" {
		return l.Bench.JournalPath(), ev
	}
	return l.journalFor(entity), ev
}

// titleOfEntity is what a person called the entity at the moment it was
// deleted, so the bench's history reads without resolving anything.
func (l *Library) titleOfEntity(entity *bench.EntityRef) string {
	if entity.Kind == "card" && entity.Card != nil {
		return entity.Card.Title
	}
	if state := l.Bench.State(entity.ID); state != nil {
		return state.Title
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
func Init(root, slug, operator, source, override, overrideSource string) (string, error) {
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
	definition, err := readSource(root, source)
	if err != nil {
		return "", err
	}
	container := filepath.Join(root, bench.UserBaseName)
	id, err := bench.ClaimID(container, nil)
	if err != nil {
		return "", err
	}
	written := filepath.Join(container, id)
	if err := bench.Instantiate(written, slug, operator, definition); err != nil {
		return "", err
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
	if bench.Exists(filepath.Join(source, bench.WorkbenchAnchor)) {
		opened, err := bench.Open(source)
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
		return nil, contract.Refuse(contract.UnknownPath, source)
	}
	return bench.ReadDefinition(data)
}

// defaultDefinition is the flow a bench created from nothing carries: one
// intake station, one working station and one done station, which is the
// smallest flow the contract's own vocabulary can express.
func defaultDefinition(title string) *bench.Definition {
	states := []map[string]json.RawMessage{
		stateMember("intake", "Intake", contract.KindIntake),
		stateMember("doing", "Doing", contract.KindWork),
		stateMember("done", "Done", contract.KindDone),
	}
	return &bench.Definition{
		Object:  map[string]json.RawMessage{},
		Title:   title,
		Profile: bench.ProfileVersion,
		States:  states,
	}
}

// stateMember builds one element of the default flow's states array. The
// identifiers are minted at instantiation, since these names are not hex.
func stateMember(id, title, kind string) map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"id":    json.RawMessage(`"` + id + `"`),
		"title": json.RawMessage(`"` + title + `"`),
		"kind":  json.RawMessage(`"` + kind + `"`),
	}
}
