package verb

import (
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// NewColumn creates a column carrying a title and, optionally, a kind, a
// capacity, a slug and a placement ahead of an existing column.
//
// It evaluates in the order SetWorkbench and NewWorkstream both fix: the
// workbench designates an operator, each supplied value is present and well
// formed, the reference --before names resolves, the request names an owner.
// No row compares the owner against the operator. Authorization here turns on
// what the placement would do to a column already carrying a card rather than
// on who is asking, and deleting a column, a larger act on the same entity, is
// already open to any named owner.
//
// The placement-safety check runs last and inside the lock, against a view of
// the workbench read after the lock was taken. It is the one check here whose
// subject can move while the call is being evaluated: a card arriving in the
// affected column between an earlier read and the write would otherwise have
// its automatic routing changed underneath it with nothing said, which is the
// silent violation of the very invariant the check exists to provide. The
// other checks read values the caller supplied or a name whose resolution
// costs a refusal and a retry when it goes stale, which is the window every
// creation call on this workbench already accepts.
func (l *Library) NewColumn(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	title := strings.TrimSpace(req.Column)
	if title == "" {
		return l.refuse(req, nil, contract.Malformed, "title")
	}
	kind := strings.TrimSpace(req.Kind)
	if kind != "" && !bench.ValidColumnKind(kind) {
		return l.refuse(req, nil, contract.Malformed, "kind")
	}
	var capacity int
	if text := strings.TrimSpace(req.Capacity); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || n <= 0 {
			return l.refuse(req, nil, contract.Malformed, "capacity")
		}
		capacity = n
	}
	slug := strings.TrimSpace(req.Slug)
	if slug != "" && !bench.ValidColumnSlug(slug) {
		return l.refuse(req, nil, contract.Malformed, bench.SlugField)
	}
	// A title no slug can be derived from is refused rather than written,
	// which is what SlugifyDashed asks of every caller: a column carrying no
	// slug is one dinah check reports and one nobody can name without its
	// identifier. The refusal names the title, because the title is the value
	// the caller would have to change.
	if slug == "" && bench.SlugifyDashed(title) == "" {
		return l.refuse(req, nil, contract.Malformed, "title")
	}
	before := strings.TrimSpace(req.Before)
	if before != "" && l.Bench.ColumnByRef(before) == nil {
		return l.refuse(req, nil, contract.UnknownColumn, before)
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
	// The write runs against a view opened under the lock rather than against
	// the one the library has been holding, so the flow the placement is
	// judged against and the flow the placement is spliced into are the same
	// flow. SetWorkstream reloads its own anchor under its own lock for the
	// same reason.
	fresh, err := bench.Open(l.Bench.Root)
	if err != nil {
		return l.FromError(req, err)
	}
	insertAt := len(fresh.Columns)
	if before != "" {
		target := fresh.ColumnByRef(before)
		if target == nil {
			return l.refuse(req, nil, contract.UnknownColumn, before)
		}
		insertAt = target.Position
	}
	effective := kind
	if effective == "" {
		effective = bench.DefaultColumnKind
	}
	cards, err := fresh.Cards()
	if err != nil {
		return l.FromError(req, err)
	}
	if disrupted := placementDisrupts(fresh.Columns, insertAt, effective, cards); disrupted != nil {
		return l.refuse(req, nil, contract.ColumnRoutingDisrupted, disrupted.Ref())
	}
	column, err := fresh.NewColumn(title, kind, slug, capacity, before)
	if err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{TS: now, Event: contract.EventCreated, Actor: req.Actor, Title: title, Note: column.ID}
	if err := bench.AppendEvent(fresh.JournalPath(), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, nil)
	view := ColumnView{
		ID:          column.ID,
		Slug:        column.Slug,
		Title:       column.Title,
		Kind:        column.Kind,
		TakesWorkUp: column.TakesWorkUp(),
		Capacity:    column.Capacity,
	}
	response.Column = &view
	response.Detail = column.ID
	return response
}
