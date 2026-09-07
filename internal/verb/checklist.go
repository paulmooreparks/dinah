package verb

import (
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The six verbs in this file are the write side of a card's checklist. They
// follow Comment and Attach exactly: resolve the target, check the operator
// and the actor, take the card's own directory lock, write the entity, append
// one event to the card's journal, answer.
//
// None of them touches the claim system. A checklist item write is a
// comment-shaped act rather than a claim-shaped one, so filing, citing,
// resolving or reopening an item on a card somebody else holds succeeds
// exactly as leaving a comment on it does, and the only refusal two concurrent
// writers can produce is the transient Locked.

// File creates a pending checklist item on a card, carrying the given text as
// its body.
//
// The column and the owner are written only when the caller names one. Nothing
// in this build reads an item's column for enforcement, so absence is the one
// default that cannot be wrong once a later card decides what the field means.
func (l *Library) File(req *Request) *Response {
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
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		return l.refuse(req, found.Card, contract.Malformed, bench.ItemKindField)
	}
	if !bench.KnownItemKind(kind) {
		return l.refuseWith(req, found.Card, contract.UnknownItemKind, kind, map[string]string{
			"kinds": strings.Join(bench.ItemKinds, ", "),
		})
	}
	if strings.TrimSpace(req.Text) == "" {
		return l.refuse(req, found.Card, contract.Malformed, "text")
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(found.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	item, err := bench.AddItem(found.Card.Dir, kind, req.Column, req.Owner, now, req.Text)
	if err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:    now,
		Event: contract.EventItemFiled,
		Actor: req.Actor,
		Item:  item.ID,
		Kind:  kind,
	}
	if err := bench.AppendEvent(found.Card.JournalPath(), ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, found.Card)
	response.Detail = item.ID
	return response
}

// Cite appends one citation to an item. Gathering evidence is not a state
// transition, so a citation may be added any number of times and in any state
// the item is standing in, exactly as an attachment may be added to anything
// mounting a collection for one.
//
// Two things this deliberately does not check: that the scheme is one the
// workbench declares, and that the target resolves. Both are dinah check's own
// findings, check.unknown-scheme and check.dangling-citation, and the format
// already commits to taking a citation at its word at write time. The one
// refusal here is structural rather than semantic, since a scheme demanding an
// observation and a call supplying none writes a citation no terminal verb
// could ever close against.
func (l *Library) Cite(req *Request) *Response {
	entity, refused := l.resolveItem(req)
	if refused != nil {
		return refused
	}
	scheme := strings.TrimSpace(req.Scheme)
	if scheme == "" {
		return l.refuse(req, entity.card, contract.Malformed, bench.ItemCitationScheme)
	}
	target := strings.TrimSpace(req.CiteTarget)
	if target == "" {
		return l.refuse(req, entity.card, contract.Malformed, bench.ItemCitationTarget)
	}
	before, after, read := bench.ParseObserved(req.Observed)
	if !read {
		return l.refuse(req, entity.card, contract.Malformed, bench.ItemCitationObserved)
	}
	if before == "" && l.Bench.EvidenceObservedRequired(scheme) {
		return l.refuse(req, entity.card, contract.ObservationRequired, scheme)
	}
	return l.writeItem(req, entity, func(fm *bench.Frontmatter, body string) (*bench.Event, string) {
		bench.AppendCitation(fm, bench.Citation{Scheme: scheme, Target: target, Before: before, After: after})
		return &bench.Event{Event: contract.EventItemCited, Scheme: scheme, Target: target}, body
	})
}

// Resolve lands an open question or a decision at resolved.
func (l *Library) Resolve(req *Request) *Response {
	return l.closeItem(req, contract.EventItemResolved, bench.ItemResolved)
}

// Verify lands an acceptance criterion at verified.
func (l *Library) Verify(req *Request) *Response {
	return l.closeItem(req, contract.EventItemVerified, bench.ItemVerified)
}

// Fail lands an acceptance criterion at failed. It runs every check Verify
// runs, the citation obligation included, because that obligation is about an
// acceptance criterion leaving pending at all rather than about which of the
// two terminal states it lands at.
func (l *Library) Fail(req *Request) *Response {
	return l.closeItem(req, contract.EventItemFailed, bench.ItemFailed)
}

// closeItem is the body the three terminal verbs share. The landing state is
// fixed by the item's own kind rather than chosen by the caller, which is why
// there are three verbs rather than one taking a state.
func (l *Library) closeItem(req *Request, event, state string) *Response {
	entity, refused := l.resolveItem(req)
	if refused != nil {
		return refused
	}
	if !kindClosedBy(state)[entity.item.Kind] {
		return l.refuse(req, entity.card, contract.WrongItemKind, entity.item.Kind)
	}
	if entity.item.State != bench.ItemPending {
		return l.refuse(req, entity.card, contract.NotPending, entity.item.State)
	}
	note, refused := l.admitNote(req, entity, req.Note)
	if refused != nil {
		return refused
	}
	// The citation obligation is stated for an acceptance criterion alone, so
	// a decision or an open question closes with citations or without them
	// exactly as it does today.
	if entity.item.Kind == criterionKind && l.Bench.EvidenceDeclared() && bench.CountCitations(entity.fm) == 0 {
		return l.refuse(req, entity.card, contract.Uncited, entity.ref)
	}
	prior := entity.item.State
	return l.writeItem(req, entity, func(fm *bench.Frontmatter, body string) (*bench.Event, string) {
		fm.Set(bench.ItemStateField, state)
		fm.Set(bench.ItemNoteField, note)
		return &bench.Event{Event: event, From: prior, To: state}, body
	})
}

// Reopen returns a closed item to pending, for the case the card's own text
// names: a reviewer finds an item was closed wrongly.
//
// The prior note and the prior citations stay on disk. Reopening supersedes a
// resolution rather than erasing it, so the record of what was in force
// survives until a fresh resolve, verify, fail or cite replaces or adds to it.
func (l *Library) Reopen(req *Request) *Response {
	entity, refused := l.resolveItem(req)
	if refused != nil {
		return refused
	}
	if entity.item.State == bench.ItemPending {
		return l.refuse(req, entity.card, contract.NotResolved, entity.item.State)
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return l.refuse(req, entity.card, contract.Malformed, "reason")
	}
	prior := entity.item.State
	return l.writeItem(req, entity, func(fm *bench.Frontmatter, body string) (*bench.Event, string) {
		fm.Set(bench.ItemStateField, bench.ItemPending)
		return &bench.Event{
			Event:  contract.EventItemReopened,
			From:   prior,
			To:     bench.ItemPending,
			Reason: reason,
		}, body
	})
}

// criterionKind is the one item kind the citation obligation is stated for.
const criterionKind = "acceptance_criterion"

// kindClosedBy names the item kinds one landing state is legal for. It is the
// one statement of which verb closes which kind, so the refusal and the write
// cannot disagree about it.
func kindClosedBy(state string) map[string]bool {
	if state == bench.ItemResolved {
		return map[string]bool{"open_question": true, "decision": true}
	}
	return map[string]bool{criterionKind: true}
}

// itemTarget is one resolved checklist item and everything a write about it
// needs: the card whose lock and journal cover the write, the item's own two
// read fields, and the header and body a rewrite has to put back.
type itemTarget struct {
	ref  string
	dir  string
	card *bench.Card
	item *bench.Item
	fm   *bench.Frontmatter
	body string
}

// resolveItem resolves the reference the five item verbs take and reads the
// item's anchor, refusing UnknownPath for a reference naming anything that is
// not a checklist item. What it returns on a refusal is a whole response, so a
// caller writes one branch rather than three.
func (l *Library) resolveItem(req *Request) (*itemTarget, *Response) {
	if l.Bench.Operator == "" {
		return nil, l.refuse(req, nil, contract.NoOperator, "")
	}
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	if req.Actor == "" {
		return nil, l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if entity.Kind != bench.KindItem || entity.Card == nil {
		return nil, l.refuse(req, entity.Card, contract.UnknownPath, req.Ref)
	}
	item, err := bench.LoadItem(entity.Dir)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	fm, body, err := bench.ReadItemAnchor(entity.Dir)
	if err != nil {
		return nil, l.FromError(req, err)
	}
	return &itemTarget{ref: req.Ref, dir: entity.Dir, card: entity.Card, item: item, fm: fm, body: body}, nil
}

// admitNote checks the resolution note the three terminal verbs require. A
// note has to be there, and it has to say something the item's own text does
// not already say.
//
// The second check catches the literal echo alone. A note restating the
// criterion in different words passes it while saying nothing, and the help
// text says so rather than presenting the check as stronger than it is.
func (l *Library) admitNote(req *Request, entity *itemTarget, note string) (string, *Response) {
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return "", l.refuse(req, entity.card, contract.Malformed, bench.ItemNoteField)
	}
	if trimmed == strings.TrimSpace(entity.body) {
		return "", l.refuseWith(req, entity.card, contract.Malformed, bench.ItemNoteField, map[string]string{
			"echo": "1",
		})
	}
	return trimmed, nil
}

// writeItem takes the card's lock, applies a write to the item's anchor, and
// appends the event that write produced to the card's journal. The lock, the
// stamp, the actor and the item's identifier are filled in here, so each verb
// above states its own change and nothing else.
//
// dinah-30's batch verb composes on top of this: it takes the lock once and
// calls the write body N times under it, which is the same concurrency story
// this acquisition already tells for one write.
func (l *Library) writeItem(req *Request, entity *itemTarget, apply func(*bench.Frontmatter, string) (*bench.Event, string)) *Response {
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(entity.card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	ev, body := apply(entity.fm, entity.body)
	if err := bench.WriteItemAnchor(entity.dir, entity.fm, body); err != nil {
		return l.FromError(req, err)
	}
	ev.TS = now
	ev.Actor = req.Actor
	ev.Item = entity.item.ID
	if err := bench.AppendEvent(entity.card.JournalPath(), *ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, entity.card)
	response.Detail = entity.item.ID
	return response
}
