package verb

import (
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The six verbs in this file are the write side of a card's checklist. They
// follow Comment and Attach exactly: resolve the target, check the operator
// and the actor, take the card's own directory lock, read the item under that
// lock, decide, write the entity, append one event to the card's journal,
// answer.
//
// The order of those steps is load-bearing, and it is the order Library.Do
// documents in internal/verb/mutate.go. Everything an item verb decides is
// read after the lock is held, so a precondition is answered against what is
// on disk at the moment of the write. A verb that read the item first and
// wrote that snapshot back under the lock would let two writers silently lose
// each other's work: two cite calls would each read no citations and the
// second would drop the first's entry, and two verify calls would each read
// the item pending, so NotPending would never fire.
//
// None of them touches the claim system. A checklist item write is a
// comment-shaped act rather than a claim-shaped one, so filing, citing,
// resolving or reopening an item on a card somebody else holds succeeds
// exactly as leaving a comment on it does. Two writers reaching the same item
// are serialised by the card's lock. One is admitted and the other is refused
// the transient Locked, and where the first has already finished, the second
// reads what it wrote and answers against that, so a second verify is refused
// NotPending rather than overwriting the first.

// File creates a pending checklist item on a card, carrying the given text as
// its body.
//
// The column and the owner are written only when the caller names one, and a
// named column is resolved before it is written. A column gates a card by the
// identifier its items carry, so a reference stored as it was typed holds
// nothing at all: the gate compares an identifier against a slug, finds no
// match, and the card walks through the station the workbench meant to stop
// it at with nothing reporting it. Resolving here means an item can never
// exist in that state, which is why the refusal is a write-time one rather
// than a finding dinah check reports afterwards.
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
	// Add's own column block, one file over, resolves and refuses on these
	// terms, and the refusal it raises names the unresolved value back to
	// whoever typed it.
	column := req.Column
	if column != "" {
		named := l.Bench.ColumnByRef(column)
		if named == nil {
			return l.refuse(req, found.Card, contract.UnknownColumn, column)
		}
		column = named.ID
	}
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(found.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	item, err := bench.AddItem(found.Card.Dir, kind, column, req.Owner, now, req.Text)
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
	return l.withItem(req, func(entity *itemTarget) (*bench.Event, *Response) {
		scheme := strings.TrimSpace(req.Scheme)
		if scheme == "" {
			return nil, l.refuse(req, entity.card, contract.Malformed, bench.ItemCitationScheme)
		}
		target := strings.TrimSpace(req.CiteTarget)
		if target == "" {
			return nil, l.refuse(req, entity.card, contract.Malformed, bench.ItemCitationTarget)
		}
		before, after, read := bench.ParseObserved(req.Observed)
		if !read {
			return nil, l.refuse(req, entity.card, contract.Malformed, bench.ItemCitationObserved)
		}
		if before == "" && l.Bench.EvidenceObservedRequired(scheme) {
			return nil, l.refuse(req, entity.card, contract.ObservationRequired, scheme)
		}
		bench.AppendCitation(entity.fm, bench.Citation{Scheme: scheme, Target: target, Before: before, After: after})
		return &bench.Event{Event: contract.EventItemCited, Scheme: scheme, Target: target}, nil
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
	return l.withItem(req, func(entity *itemTarget) (*bench.Event, *Response) {
		if !kindClosedBy(state)[entity.item.Kind] {
			return nil, l.refuse(req, entity.card, contract.WrongItemKind, entity.item.Kind)
		}
		if entity.item.State != bench.ItemPending {
			return nil, l.refuse(req, entity.card, contract.NotPending, entity.item.State)
		}
		note, refused := l.admitNote(req, entity, req.Note)
		if refused != nil {
			return nil, refused
		}
		// The citation obligation is stated for an acceptance criterion alone,
		// so a decision or an open question closes with citations or without
		// them exactly as it does today.
		if entity.item.Kind == criterionKind && l.Bench.EvidenceDeclared() && bench.CountCitations(entity.fm) == 0 {
			return nil, l.refuse(req, entity.card, contract.Uncited, entity.ref)
		}
		prior := entity.item.State
		entity.fm.Set(bench.ItemStateField, state)
		entity.fm.Set(bench.ItemNoteField, note)
		return &bench.Event{Event: event, From: prior, To: state}, nil
	})
}

// Reopen returns a closed item to pending, for the case the card's own text
// names: a reviewer finds an item was closed wrongly.
//
// The prior note and the prior citations stay on disk. Reopening supersedes a
// resolution rather than erasing it, so the record of what was in force
// survives until a fresh resolve, verify, fail or cite replaces or adds to it.
func (l *Library) Reopen(req *Request) *Response {
	return l.withItem(req, func(entity *itemTarget) (*bench.Event, *Response) {
		if entity.item.State == bench.ItemPending {
			return nil, l.refuse(req, entity.card, contract.NotResolved, entity.item.State)
		}
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			return nil, l.refuse(req, entity.card, contract.Malformed, "reason")
		}
		prior := entity.item.State
		entity.fm.Set(bench.ItemStateField, bench.ItemPending)
		return &bench.Event{
			Event:  contract.EventItemReopened,
			From:   prior,
			To:     bench.ItemPending,
			Reason: reason,
		}, nil
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

// itemStepUnlocked names the one window an item write leaves open: after the
// reference has been resolved and before the card's lock is taken, where
// nothing has been read yet and so nothing can go stale. It is the step name
// Interpose is given, and a test uses it to run a whole second write in that
// window and then assert the first one sees it.
const itemStepUnlocked = "item-unlocked"

// withItem is the body the five item verbs share, and the order of its steps
// is the whole of what makes an item write one transaction.
//
// The reference is resolved first, because the lock lives inside the card's
// own directory and there is nothing to lock until the card is found. The
// card's lock is then taken, and the item is read under it, so the state every
// precondition reads and the frontmatter the write puts back are the ones on
// disk at the moment of the write rather than a snapshot taken before it. That
// is the order Library.Do documents and it is the order for the same reason:
// two processes reaching the same item cannot both see it pending, because the
// second is either refused the lock outright or reads what the first wrote.
//
// The work function decides and edits. It reads the item and the frontmatter
// off the target, sets whatever it changes, and answers with the event its
// change produced, or with a refusal and no event. Everything the five verbs
// share is filled in here: the lock, the stamp, the actor, the item's
// identifier, the anchor write and the journal line.
//
// dinah-30's batch verb composes on top of this: it takes the lock once and
// runs the work body N times under it, which is the same concurrency story
// this acquisition already tells for one write.
func (l *Library) withItem(req *Request, work func(*itemTarget) (*bench.Event, *Response)) *Response {
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
	if entity.Kind != bench.KindItem || entity.Card == nil {
		return l.refuse(req, entity.Card, contract.UnknownPath, req.Ref)
	}
	l.interpose(itemStepUnlocked)
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(entity.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	item, err := bench.LoadItem(entity.Dir)
	if err != nil {
		return l.FromError(req, err)
	}
	fm, body, err := bench.ReadItemAnchor(entity.Dir)
	if err != nil {
		return l.FromError(req, err)
	}
	target := &itemTarget{ref: req.Ref, dir: entity.Dir, card: entity.Card, item: item, fm: fm, body: body}
	ev, refused := work(target)
	if refused != nil {
		return refused
	}
	if err := bench.WriteItemAnchor(target.dir, target.fm, target.body); err != nil {
		return l.FromError(req, err)
	}
	ev.TS = now
	ev.Actor = req.Actor
	ev.Item = item.ID
	if err := bench.AppendEvent(entity.Card.JournalPath(), *ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, entity.Card)
	response.Detail = item.ID
	return response
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
