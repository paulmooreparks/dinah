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
		StateRef: stateRefSubject(entity),
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
		Dir:           entity.Dir,
		LockDir:       l.lockDirFor(entity),
		Op:            bench.OpDelete,
		Actor:         req.Actor,
		Now:           now,
		StateID:       stateSubject(entity),
		StateRef:      stateRefSubject(entity),
		WorkstreamID:  workstreamSubject(entity),
		WorkstreamRef: workstreamRefSubject(entity),
		Record:        func() error { return bench.AppendEvent(journal, ev) },
	}
	if err := l.Bench.Run(act); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, nil)
	response.Detail = entity.ID
	return response
}

// Rename carries an attachment's payload under a new filename and rewrites
// the anchor's filename field to match. Cards, states, comments, checklist
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
		Actor:      req.Actor,
		Attachment: after.ID,
		Filename:   after.Filename,
		From:       before.Filename,
	}
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

// Workbench reports the workbench's own fields. Reading one is open to
// anybody, so no owner is required and no operator is asked for; every other
// read in the tool is open the same way.
//
// A request whose action is get names one field, and a field outside the set
// refuses. The refusal travels as a contract.Refusal carrying no verb, so one
// catalog sentence serves this read, the write inside the library, and
// `dinah config get` alike, and none of the three can drift from the others.
func (l *Library) Workbench(req *Request) (*WorkbenchView, error) {
	if req.Action == "get" && !bench.KnownWorkbenchField(req.Field) {
		return nil, contract.Refuse(contract.UnknownKey, req.Field)
	}
	view := &WorkbenchView{
		Title:    l.Bench.Title,
		Slug:     l.Bench.Slug,
		Operator: l.Bench.Operator,
	}
	return view, nil
}

// SetWorkbench writes one of the workbench's own fields. It evaluates in the
// order the spec's check table fixes: the workbench designates an operator,
// the field is one this workbench records, the value is present and well
// formed, the request names an owner, that owner is the operator, and a slug
// rename carries the confirmation flag.
//
// The operator the actor is compared against is the one Open loaded, before
// the lock is taken, which is what every sibling does. So `workbench set
// operator <somebody-else>`, run by the operator, succeeds, and the caller
// ceases to be the operator for the next command rather than for this one.
//
// No field is ever cleared, which is where this grammar departs from `config
// set`. Open refuses a workbench whose title is empty, clearing the operator
// leaves nobody who can set it again through this command, and clearing the
// slug leaves every card reachable only by its identifier.
//
// The write reloads the anchor under the lock and sets the one field on the
// reloaded value, because Save rewrites the whole anchor from the frontmatter
// the Bench is holding and a stale copy would revert whatever landed after it
// was read.
func (l *Library) SetWorkbench(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	if !bench.KnownWorkbenchField(req.Field) {
		return l.refuse(req, nil, contract.UnknownKey, req.Field)
	}
	value := strings.TrimSpace(req.Value)
	if value == "" {
		return l.refuse(req, nil, contract.Malformed, req.Field)
	}
	if req.Field == "slug" && !bench.ValidSlug(value) {
		return l.refuse(req, nil, contract.Malformed, req.Field)
	}
	if req.Actor == "" {
		return l.refuse(req, nil, contract.NoOwner, "")
	}
	if req.Actor != l.Bench.Operator {
		return l.refuse(req, nil, contract.NotOperator, req.Actor)
	}
	if req.Field == "slug" && !req.Confirm {
		return l.refuse(req, nil, contract.Unconfirmed, value)
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
	reloaded, err := bench.Open(l.Bench.Root)
	if err != nil {
		return l.FromError(req, err)
	}
	was := reloaded.WorkbenchField(req.Field)
	reloaded.SetWorkbenchField(req.Field, value)
	if err := reloaded.Save(); err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:    now,
		Event: contract.EventWorkbenchUpdated,
		Actor: req.Actor,
		Field: req.Field,
		From:  was,
		To:    value,
	}
	if err := bench.AppendEvent(reloaded.JournalPath(), ev); err != nil {
		return l.FromError(req, err)
	}
	l.Bench.SetWorkbenchField(req.Field, value)
	response := l.ok(req, nil)
	response.Detail = value
	return response
}

// journalFor names the journal an event about an entity is recorded in, which
// is the nearest enclosing journal-bearing entity: a card's own journal for
// anything below a card, a workstream's own for the workstream, and the
// bench's for everything else.
func (l *Library) journalFor(entity *bench.EntityRef) string {
	if entity.Card != nil {
		return entity.Card.JournalPath()
	}
	if entity.Kind == "workstream" {
		return filepath.Join(entity.Dir, bench.JournalName)
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
	if entity.Kind == "workstream" {
		return entity.Dir
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

// stateRefSubject is StateRef's own reading of the same question stateSubject
// answers for StateID, so the two stay paired and StructuralAct.StateRef's
// documented invariant, empty exactly when StateID is, holds by construction
// rather than by every entity kind but state happening to carry no Ref today.
func stateRefSubject(entity *bench.EntityRef) string {
	if entity.Kind != "state" {
		return ""
	}
	return entity.Ref
}

// workstreamSubject names the workstream a deletion is removing, and is empty
// for an act on any other kind. It is what arms the membership scan the act
// runs once its own sibling exists. Archiving passes it nothing, because a
// workstream cards still belong to is the ordinary thing to archive.
func workstreamSubject(entity *bench.EntityRef) string {
	if entity.Kind != "workstream" {
		return ""
	}
	return entity.ID
}

// workstreamRefSubject is WorkstreamRef's own reading of the same question
// workstreamSubject answers for WorkstreamID, paired the same way
// stateRefSubject pairs with stateSubject, so StructuralAct.WorkstreamRef's
// documented invariant, empty exactly when WorkstreamID is, holds by
// construction rather than by every entity kind but workstream happening to
// carry no Ref today.
func workstreamRefSubject(entity *bench.EntityRef) string {
	if entity.Kind != "workstream" {
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
	// Deleting a card or a workstream destroys the journal inside it, so the
	// record goes to the bench's, carrying the identifier and the title as
	// of the event.
	if entity.Kind == "card" || entity.Kind == "workstream" {
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
	if entity.Kind == "workstream" {
		if workstream := l.Bench.Workstream(entity.ID); workstream != nil {
			return workstream.Title
		}
		return ""
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
		return nil, contract.With(contract.Refuse(contract.UnknownPath, source), "file", source)
	}
	definition, readErr := bench.ReadDefinition(data)
	return definition, contract.With(readErr, "file", source)
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

// WorkstreamView is a workstream as a response carries it.
type WorkstreamView struct {
	// ID is the workstream's 12-hex identifier.
	ID string `json:"id"`
	// Ref is what a person types to reach it: its slug where it carries one,
	// its identifier otherwise.
	Ref string `json:"ref,omitempty"`
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

// WorkstreamDetail is one workstream as a read reports it: the entity, its
// notes, the live cards belonging to it and the directory it lives in. It is
// the shape Show already uses for a card.
type WorkstreamDetail struct {
	// Workstream is the workstream itself.
	Workstream WorkstreamView `json:"workstream"`
	// Body is the workstream's long-form notes.
	Body string `json:"body"`
	// Cards are the live cards belonging to it, in queue order.
	Cards []CardView `json:"cards,omitempty"`
	// Path is the directory the workstream lives in.
	Path string `json:"path"`
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
	listing := &WorkstreamListing{Workstreams: []WorkstreamView{}}
	for _, workstream := range l.Bench.Workstreams() {
		listing.Workstreams = append(listing.Workstreams, workstreamView(workstream, counts))
	}
	return listing, nil
}

// Workstream reports one workstream, its notes, its path and the live cards
// belonging to it.
//
// A request whose action is get may name one field, and a field outside the
// set refuses. The workstream resolves before the field is read, because a
// reader who mistyped the reference is told about the reference rather than
// about a field of a workstream that does not exist.
func (l *Library) Workstream(req *Request) (*WorkstreamDetail, error) {
	workstream := l.Bench.WorkstreamByRef(req.Workstream)
	if workstream == nil {
		return nil, contract.Refuse(contract.UnknownWorkstream, req.Workstream)
	}
	if req.Field != "" && !bench.KnownWorkstreamField(req.Field) {
		return nil, contract.Refuse(contract.UnknownKey, req.Field)
	}
	counts, err := l.Bench.WorkstreamCounts()
	if err != nil {
		return nil, err
	}
	members, err := l.membersOf(workstream.ID)
	if err != nil {
		return nil, err
	}
	detail := &WorkstreamDetail{
		Workstream: workstreamView(workstream, counts),
		Body:       workstream.Notes,
		Cards:      members,
		Path:       workstream.Dir,
	}
	return detail, nil
}

// membersOf reads the live cards belonging to a workstream, in the order
// CORE-QUEUE-3 fixes, which is the order every other listing of cards takes.
func (l *Library) membersOf(id string) ([]CardView, error) {
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, err
	}
	var kept []*bench.Card
	for _, card := range cards {
		for _, joined := range card.Workstreams {
			if joined == id {
				kept = append(kept, card)
				break
			}
		}
	}
	sortByArrival(kept)
	var views []CardView
	for _, card := range kept {
		views = append(views, *l.view(card))
	}
	return views, nil
}

// NewWorkstream creates a workstream from a title and opens its journal with
// the created event.
//
// The title is required and nothing else is, which is Add's own list: filing a
// grouping is filing work, so it asks for an owner rather than for the
// operator. The workbench's own lock covers the write, because the identifier,
// the creation ordinal and the slug collision scan are all read against the
// collection as a whole.
func (l *Library) NewWorkstream(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	title := strings.TrimSpace(req.Workstream)
	if title == "" {
		return l.refuse(req, nil, contract.Malformed, "title")
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
	workstream, err := l.Bench.NewWorkstream(title)
	if err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:    now,
		Event: contract.EventCreated,
		Actor: req.Actor,
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

// SetWorkstream writes one of a workstream's own fields. It evaluates in the
// order SetWorkbench fixes, with the reference resolved first because this
// command names an entity where that one names the workbench it is already
// serving: the workbench designates an operator, the workstream resolves, the
// field is one this entity records, the value is present and well formed, the
// request names an owner, that owner is the operator, and a slug change
// carries the confirmation flag.
//
// No field is ever cleared, for SetWorkbench's own reason: an empty title
// leaves the entity unnameable, and an empty slug leaves it reachable only by
// its identifier.
//
// A slug another live workstream already carries is accepted, so this command
// can write a duplicate that NewWorkstream's own collision loop can never
// produce. Check is the whole answer to that state. It raises exactly one
// check.workstream-slug-duplicate finding over the pair, never two, and it
// names the later of the two by creation order, because checkWorkstreams and
// WorkstreamByRef walk the collection in the same order: the earlier workstream
// fills the seen set first and is the one a shared reference reaches, so the
// identifier printed is always the workstream whose slug has been shadowed. A
// person therefore meets the name that has stopped answering, together with the
// finding's own sentence saying another workstream of this workbench carries
// the same slug, and the repair is to rename that one or leave it reachable by
// its identifier alone.
//
// Refusing the write here instead would amend the evaluation order the operator
// ratified, inside a change he does not see, so whether this command grows that
// refusal is his call rather than this command's.
//
// The write reloads the anchor under the workstream's own lock and sets the
// one field on the reloaded value, because Save rewrites the whole anchor from
// the frontmatter it is holding and a stale copy would revert whatever landed
// after it was read.
func (l *Library) SetWorkstream(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	workstream := l.Bench.WorkstreamByRef(req.Workstream)
	if workstream == nil {
		return l.refuse(req, nil, contract.UnknownWorkstream, req.Workstream)
	}
	if !bench.KnownWorkstreamField(req.Field) {
		return l.refuse(req, nil, contract.UnknownKey, req.Field)
	}
	value := strings.TrimSpace(req.Value)
	if value == "" {
		return l.refuse(req, nil, contract.Malformed, req.Field)
	}
	if req.Field == bench.SlugField && !bench.ValidStateSlug(value) {
		return l.refuse(req, nil, contract.Malformed, req.Field)
	}
	if req.Actor == "" {
		return l.refuse(req, nil, contract.NoOwner, "")
	}
	if req.Actor != l.Bench.Operator {
		return l.refuse(req, nil, contract.NotOperator, req.Actor)
	}
	if req.Field == bench.SlugField && !req.Confirm {
		return l.refuse(req, nil, contract.Unconfirmed, value)
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(workstream.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	if l.Interleave != nil {
		l.Interleave()
	}
	reloaded, err := bench.LoadWorkstream(filepath.Dir(workstream.Dir), workstream.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	was := reloaded.Field(req.Field)
	reloaded.SetField(req.Field, value)
	if err := reloaded.Save(); err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:    now,
		Event: contract.EventWorkstreamUpdated,
		Actor: req.Actor,
		Field: req.Field,
		From:  was,
		To:    value,
	}
	if err := bench.AppendEvent(reloaded.JournalPath(), ev); err != nil {
		return l.FromError(req, err)
	}
	counts, err := l.Bench.WorkstreamCounts()
	if err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, nil)
	view := workstreamView(reloaded, counts)
	response.Workstream = &view
	response.Detail = value
	return response
}
