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
			return l.refuse(req, nil, contract.AtCapacity, named.ID)
		}
		destination = named
	}
	number := l.Bench.NextNumber()
	id, err := bench.ClaimID(l.Bench.CardsRoot(), l.Bench.HasIdentifier)
	if err != nil {
		return l.FromError(req, err)
	}
	fm := bench.NewFrontmatter()
	fm.Set("title", title)
	fm.Set("number", strconv.Itoa(number))
	fm.Set("state", destination.ID)
	fm.Set("substate", contract.SubstateReady)
	dir := filepath.Join(l.Bench.CardsRoot(), id)
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
	if entity.Kind == "state" && l.Bench.StateOccupied(entity.ID) {
		return l.refuse(req, nil, contract.Occupied, entity.ID)
	}
	if entity.Kind == "bench" {
		return l.refuse(req, nil, contract.UnknownPath, req.Ref)
	}
	journal := l.journalFor(entity)
	ev := bench.Event{TS: bench.Stamp(l.Now()), Event: contract.EventArchived, Actor: req.Actor, Note: entity.ID}
	// An entity's own journal travels with its directory, so the event is
	// written before the move; a journal outside the directory is written
	// after it, so a failed move leaves no record of one that did not happen.
	travels := strings.HasPrefix(journal, entity.Dir+string(filepath.Separator))
	if travels {
		if err := bench.AppendEvent(journal, ev); err != nil {
			return l.FromError(req, err)
		}
	}
	if _, err := bench.ArchiveEntity(entity.Dir); err != nil {
		return l.FromError(req, err)
	}
	if !travels {
		if err := bench.AppendEvent(journal, ev); err != nil {
			return l.FromError(req, err)
		}
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
// reference never refuses an act; the dangling `to:` is what fsck reports
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
	if entity.Kind == "state" && l.Bench.StateOccupied(entity.ID) {
		return l.refuse(req, nil, contract.Occupied, entity.ID)
	}
	if entity.Kind == "bench" {
		return l.refuse(req, nil, contract.UnknownPath, req.Ref)
	}
	if entity.Kind == "attachment" {
		ev := bench.Event{
			TS:         bench.Stamp(l.Now()),
			Event:      contract.EventAttachmentRemoved,
			Actor:      req.Actor,
			Attachment: entity.ID,
		}
		if attachment, err := bench.LoadAttachment(entity.Dir); err == nil {
			ev.Filename = attachment.Filename
		}
		if err := bench.AppendEvent(l.journalFor(entity), ev); err != nil {
			return l.FromError(req, err)
		}
	}
	if err := bench.DeleteEntity(entity.Dir); err != nil {
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

// Init creates a bench, optionally from a template or another bench's
// interchange form.
func Init(root, slug, operator, source string) error {
	if bench.Exists(filepath.Join(root, bench.WorkbenchAnchor)) {
		return contract.Refuse(contract.Exists, root)
	}
	definition, err := readSource(root, source)
	if err != nil {
		return err
	}
	return bench.Instantiate(root, slug, operator, definition)
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
