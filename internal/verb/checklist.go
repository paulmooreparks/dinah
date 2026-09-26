package verb

import (
	"path/filepath"
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
	if refused := l.malformedHarness(req, nil); refused != nil {
		return refused
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
		// An item naming a column the card's own route never reaches is a
		// hold that never fires, which the workbench's own instructions call
		// the commonest way a stop somebody meant to create fails to exist.
		// An item filed with no column names no column to be off the route,
		// and a card carrying no route walks every column the workbench
		// declares, so neither reaches this row.
		if !l.Bench.RouteCarries(found.Card, named) {
			return l.refuseWith(req, found.Card, contract.ItemOffRoute, named.Ref(), map[string]string{
				"route": found.Card.Route,
			})
		}
		column = named.ID
	}
	now := bench.Stamp(l.Now())
	lock, err := l.Bench.Acquire(found.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	item, err := l.Bench.AddItem(found.Card.Dir, kind, column, req.Owner, now, req.Text)
	if err != nil {
		return l.FromError(req, err)
	}
	ev := bench.Event{
		TS:    now,
		Event: contract.EventItemFiled,
		Actor: req.Acting(),
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
		return &bench.Event{Actor: req.Acting(), Event: contract.EventItemCited, Scheme: scheme, Target: target}, nil
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

// Waive lands an item at waived, which records that the finding the item
// carries stands and that the workbench operator has decided the card may
// proceed regardless.
//
// It is refused to everybody but the operator, whatever the item's owner key
// says, and the guard never reads the owner. A waiver is permission to
// proceed past a real finding, and the move-level override that carries a
// card past such a finding today is already the operator's alone. An agent
// that could waive its own failed criterion would make the gate worth
// nothing.
//
// Two checks closeItem runs are deliberately absent. The kind check does not
// apply, because the gate reads no kind, so an item of any kind can hold a
// card and an item of any kind can need waiving. The citation obligation does
// not apply either: a waiver is precisely the case where no check was run or
// the check did not hold, so demanding evidence would make the state
// unreachable exactly where it is most needed.
func (l *Library) Waive(req *Request) *Response {
	return l.withItem(req, func(entity *itemTarget) (*bench.Event, *Response) {
		if req.Actor != l.Bench.Operator {
			return nil, l.refuse(req, entity.card, contract.NotOperator, req.Actor)
		}
		// A waiver lifts a hold, so it is legal only from a state that is
		// holding. Waiving from pending is legal rather than requiring a
		// fail first: the operator may decide a check need not be run on
		// this card at all, and forcing him to record a failure nobody
		// observed would put a false finding on the record to reach a true
		// permission.
		switch entity.item.State {
		case bench.ItemPending, bench.ItemFailed:
		default:
			return nil, l.refuse(req, entity.card, contract.NotWaivable, entity.item.State)
		}
		resolution, refused := l.admitDesignation(req, entity)
		if refused != nil {
			return nil, refused
		}
		prior := entity.item.State
		entity.fm.Set(bench.ItemStateField, bench.ItemWaived)
		entity.fm.Set(bench.ItemResolutionField, resolution)
		return &bench.Event{
			Actor: req.Acting(),
			Event: contract.EventItemWaived,
			From:  prior,
			To:    bench.ItemWaived,
		}, nil
	})
}

// Withdraw lands an item at withdrawn, which records that the question the
// item carries stopped being a question, usually because the card changed
// underneath it.
//
// Two guards run rather than one. The kind guard refuses a withdrawal of an
// acceptance criterion to everybody but the operator, whatever the item's
// state and whatever its owner key says, unless the card carries a standing
// criterion-retirement grant. Without it the card's own argument collapses:
// every acceptance criterion on a working workbench is stamped for its
// holder, so an agent holding a card whose criterion stood failed could
// withdraw it and walk the card through the gate with nobody having decided
// anything.
//
// The guard is on the kind rather than on the failed state alone, because a
// state guard leaks through reopen. Nothing about reopen changes an item's
// kind, so a kind guard cannot be composed around.
//
// The owner guard is the one the three terminal verbs already run, and the
// grant does not reach it: a criterion the operator owns stays his alone.
//
// Withdrawing an item that already carries a resolution overwrites that key
// with the withdrawal's own designation. The comment the item previously
// designated stays where it is and the journal carries the earlier settling,
// so nothing is destroyed; the key means the comment recording why the item
// is in the state it is in, and after a withdrawal the state is withdrawn.
func (l *Library) Withdraw(req *Request) *Response {
	return l.withItem(req, func(entity *itemTarget) (*bench.Event, *Response) {
		if entity.item.State == bench.ItemWithdrawn {
			return nil, l.refuse(req, entity.card, contract.AlreadyWithdrawn, entity.item.State)
		}
		granted := false
		if req.Actor != l.Bench.Operator {
			if entity.fm.Value(bench.ItemOwnerField) == bench.ItemOwnerOperator {
				return nil, l.refuse(req, entity.card, contract.NotOperator, req.Actor)
			}
			if entity.item.Kind == criterionKind {
				if entity.card.RetirementGrant == "" {
					return nil, l.refuse(req, entity.card, contract.NotOperator, req.Actor)
				}
				// The grant admits the retirement of a criterion nobody
				// has found anything wrong with. A finding that exists is
				// the operator's to retire, and the exclusion is keyed on
				// the two states a non-operator can neither reach nor
				// leave, which is what makes a state key durable here.
				switch entity.item.State {
				case bench.ItemFailed, bench.ItemWaived:
					return nil, l.refuse(req, entity.card, contract.GrantExcludesFinding, entity.item.State)
				}
				granted = true
			}
		}
		resolution, refused := l.admitDesignation(req, entity)
		if refused != nil {
			return nil, refused
		}
		prior := entity.item.State
		entity.fm.Set(bench.ItemStateField, bench.ItemWithdrawn)
		entity.fm.Set(bench.ItemResolutionField, resolution)
		return &bench.Event{
			Actor: req.Acting(),
			Event: contract.EventItemWithdrawn,
			From:  prior,
			To:    bench.ItemWithdrawn,
			Grant: granted,
		}, nil
	})
}

// closeItem is the body the three terminal verbs share. The landing state is
// fixed by the item's own kind rather than chosen by the caller, which is why
// there are three verbs rather than one taking a state.
func (l *Library) closeItem(req *Request, event, state string) *Response {
	return l.withItem(req, func(entity *itemTarget) (*bench.Event, *Response) {
		// An item the operator owns is the operator's to settle. The check
		// reads the item rather than the verb, so one statement of it covers
		// resolve, verify and fail, which all land here, and covers all three
		// item kinds, because whose item this is does not depend on what kind
		// of judgment it records. Reopen does not land here and is left open
		// deliberately: reopening returns an item to pending, which can only
		// re-impose a hold and never lift one.
		if entity.fm.Value(bench.ItemOwnerField) == bench.ItemOwnerOperator && req.Actor != l.Bench.Operator {
			return nil, l.refuse(req, entity.card, contract.NotOperator, req.Actor)
		}
		if !kindClosedBy(state)[entity.item.Kind] {
			return nil, l.refuse(req, entity.card, contract.WrongItemKind, entity.item.Kind)
		}
		if entity.item.State != bench.ItemPending {
			return nil, l.refuse(req, entity.card, contract.NotPending, entity.item.State)
		}
		resolution, refused := l.admitDesignation(req, entity)
		if refused != nil {
			return nil, refused
		}
		// The citation obligation is stated for an acceptance criterion alone,
		// so a decision or an open question closes with citations or without
		// them exactly as it does today.
		if entity.item.Kind == criterionKind && l.Bench.EvidenceDeclared() && bench.CountCitations(entity.fm) == 0 {
			return nil, l.refuse(req, entity.card, contract.Uncited, entity.ref)
		}
		// An item demanding a scheme is settled only against a citation
		// naming it. The row reads the citation's scheme alone and nothing
		// about its target, on the posture Cite takes toward a scheme it
		// has never heard of, and it runs after the citation obligation
		// above, which asks whether there is any citation at all before
		// this one asks whether one of them is the right one. waive and
		// withdraw do not land here and are untouched: neither claims the
		// evidence exists.
		if scheme := entity.fm.Value(bench.ItemEvidenceField); scheme != "" && !citesScheme(entity.fm, scheme) {
			return nil, l.refuseWith(req, entity.card, contract.EvidenceSchemeRequired, scheme, map[string]string{
				"item": entity.ref,
			})
		}
		prior := entity.item.State
		entity.fm.Set(bench.ItemStateField, state)
		entity.fm.Set(bench.ItemResolutionField, resolution)
		return &bench.Event{Actor: req.Acting(), Event: event, From: prior, To: state}, nil
	})
}

// citesScheme reports whether at least one citation of an item names the
// scheme the item demands, which is the whole of what the evidence demand
// reads.
func citesScheme(fm *bench.Frontmatter, scheme string) bool {
	for _, cited := range bench.CitationSchemes(fm) {
		if cited == scheme {
			return true
		}
	}
	return false
}

// Reopen returns a closed item to pending, for the case the card's own text
// names: a reviewer finds an item was closed wrongly.
//
// It takes a reason, which is free prose and stays free prose. The framing
// this card was filed under asked for a designation here too, and dinah-525's
// spec amends it: reopening an item so that its only comment may be deleted
// would need a reason that is a comment of that item, and the comment the
// operator is about to delete is the one that exists. A reason is also not an
// answer, since it says why an answer stopped standing and the thing it refers
// to is often being destroyed.
//
// The designation is cleared, because the item no longer has an answer of
// record and a resolution standing beside a pending state would assert one.
// The comment itself and the prior citations stay on disk, so the words
// survive and only the claim that they settle anything goes. Clearing it is
// also what frees a designated comment for deletion.
func (l *Library) Reopen(req *Request) *Response {
	return l.withItem(req, func(entity *itemTarget) (*bench.Event, *Response) {
		if entity.item.State == bench.ItemPending {
			return nil, l.refuse(req, entity.card, contract.NotResolved, entity.item.State)
		}
		// Reopen is refused to anybody but the operator in three cases, and
		// stays open to everybody in every other one. The reasoning this
		// verb was built on, that reopening can only re-impose a hold and
		// never lift one, was true when it was written and is false now, so
		// the guard follows the reasoning rather than the sentence.
		//
		// The two state cases are what make the criterion-retirement grant's
		// own exclusion durable. With reopen open to everybody, a
		// non-operator returned a criterion from failed to pending and the
		// grant then admitted the withdrawal, so a finding was released in
		// two commands.
		//
		// The owner case is the rule the three terminal verbs already keep,
		// applied to the one verb that was left out of it: reopening an
		// operator-owned question un-answers his ruling and clears the
		// designation recording it, which is not a hold being re-imposed.
		if req.Actor != l.Bench.Operator {
			switch entity.item.State {
			case bench.ItemFailed, bench.ItemWaived:
				return nil, l.refuse(req, entity.card, contract.NotOperator, req.Actor)
			}
			if entity.fm.Value(bench.ItemOwnerField) == bench.ItemOwnerOperator {
				return nil, l.refuse(req, entity.card, contract.NotOperator, req.Actor)
			}
		}
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			return nil, l.refuse(req, entity.card, contract.Malformed, "reason")
		}
		prior := entity.item.State
		entity.fm.Set(bench.ItemStateField, bench.ItemPending)
		entity.fm.Delete(bench.ItemResolutionField)
		return &bench.Event{
			Actor:  req.Acting(),
			Event:  contract.EventItemReopened,
			From:   prior,
			To:     bench.ItemPending,
			Reason: reason,
		}, nil
	})
}

// Settle lands an item at the state the caller names, becoming whichever of
// Resolve, Verify, Fail, Waive, Withdraw or Reopen that state selects and
// running exactly
// that verb's own checks in exactly that verb's own order from that point
// on. The state choice is the only thing this function decides for itself;
// every other precondition, including the operator-owned-item guard, the
// pending/closed precondition, the citation obligation and the designation
// rule, is the chosen verb's own and is untouched.
//
// A field the chosen verb does not read is silently unused rather than
// refused: a caller settling an item to resolved may still send reason,
// and Resolve simply never looks at it, exactly as a person typing `dinah
// resolve` alongside a stray --reason flag it does not declare would be
// refused by argument-checking rather than by this function, and a caller
// of settle is refused the same way, by declaredArgNames, before this
// function ever runs.
func (l *Library) Settle(req *Request) *Response {
	switch req.State {
	case bench.ItemResolved:
		return l.Resolve(req)
	case bench.ItemVerified:
		return l.Verify(req)
	case bench.ItemFailed:
		return l.Fail(req)
	case bench.ItemWaived:
		return l.Waive(req)
	case bench.ItemWithdrawn:
		return l.Withdraw(req)
	case bench.ItemPending:
		return l.Reopen(req)
	case "":
		return l.refuse(req, nil, contract.Malformed, "state")
	default:
		return l.refuse(req, nil, contract.UnknownItemState, req.State)
	}
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
	// now is the stamp the whole write carries, so a comment the work body
	// mints bears the same instant as the settling that designated it
	// rather than a second reading of the clock.
	now string
	// also are events the work body produced beside its own, appended to
	// the journal ahead of it. The one writer of it is the --text form of a
	// terminal verb, which mints a comment and so owes the journal a
	// commented line as well as the settling.
	also []bench.Event
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
	if entity.Kind != bench.KindItem || entity.Card == nil {
		return l.refuse(req, entity.Card, contract.UnknownPath, req.Ref)
	}
	l.interpose(itemStepUnlocked)
	now := bench.Stamp(l.Now())
	lock, err := l.Bench.Acquire(entity.Card.Dir, req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	item, err := l.Bench.LoadItem(entity.Dir)
	if err != nil {
		return l.FromError(req, err)
	}
	fm, body, err := l.Bench.ReadItemAnchor(entity.Dir)
	if err != nil {
		return l.FromError(req, err)
	}
	target := &itemTarget{ref: req.Ref, dir: entity.Dir, card: entity.Card, item: item, fm: fm, body: body, now: now}
	ev, refused := work(target)
	if refused != nil {
		return refused
	}
	if err := bench.WriteItemAnchor(target.dir, target.fm, target.body); err != nil {
		return l.FromError(req, err)
	}
	// A comment the work body minted is journalled before the act that
	// designated it, because the designation names a comment and a reader
	// walking the journal forward should meet the comment first.
	for _, extra := range target.also {
		extra.TS = now
		extra.Item = item.ID
		if err := bench.AppendEvent(entity.Card.JournalPath(), extra); err != nil {
			return l.FromError(req, err)
		}
	}
	ev.TS = now
	ev.Item = item.ID
	if err := bench.AppendEvent(entity.Card.JournalPath(), *ev); err != nil {
		return l.FromError(req, err)
	}
	response := l.ok(req, entity.Card)
	response.Detail = item.ID
	return response
}

// admitDesignation settles what a terminal verb records as the item's answer.
//
// Two forms reach it and they are mutually exclusive. A reference names a
// comment that already exists, which is how an operator endorses words
// somebody else wrote: the author stays whoever wrote them and the designator
// is whoever settled the item. The --text form mints a comment of the item
// authored by whoever ran the command and designates it in the same act, which
// is today's semantics made explicit rather than a concession to convenience.
// Under the retired note key the settler's words were recorded with no field
// saying they were the settler's; now the comment records the author and the
// designation records the designator, and on this path they happen to be one
// person.
//
// An invocation naming both has not said which act it means, so it is refused
// rather than resolved by precedence.
func (l *Library) admitDesignation(req *Request, entity *itemTarget) (string, *Response) {
	named := strings.TrimSpace(req.Note)
	text := strings.TrimSpace(req.Text)
	switch {
	case named != "" && text != "":
		return "", l.refuse(req, entity.card, contract.Usage, "--text")
	case text != "":
		return l.mintDesignation(req, entity)
	case named != "":
		return l.designationOf(req, entity, named)
	}
	return "", l.refuse(req, entity.card, contract.Malformed, bench.ItemResolutionField)
}

// mintDesignation writes the comment the --text form designates, under the
// card lock withItem already holds, and hands back its canonical reference.
//
// AddComment takes no lock of its own, which is what lets it run inside a
// write that already holds the card's, and the commented event it owes the
// journal rides on the target so that one journal append site serves both
// lines.
func (l *Library) mintDesignation(req *Request, entity *itemTarget) (string, *Response) {
	// The echo check the retired note field carried is kept and moved here
	// rather than dropped with the field. It catches the literal echo alone:
	// an answer restating the criterion in different words passes it while
	// saying nothing, and the help text says so rather than presenting the
	// check as stronger than it is. What it does catch is the reflex of
	// pasting the item's own text back as its answer, which records a
	// comment that adds nothing and reads as though somebody had judged.
	if strings.TrimSpace(req.Text) == strings.TrimSpace(entity.body) {
		return "", l.refuseWith(req, entity.card, contract.Malformed, bench.ItemResolutionField, map[string]string{
			"echo": "1",
		})
	}
	comment, err := l.Bench.AddComment(entity.dir, req.Actor, entity.now, req.Text)
	if err != nil {
		return "", l.FromError(req, err)
	}
	entity.also = append(entity.also, bench.Event{
		Actor:   req.Acting(),
		Event:   contract.EventCommented,
		Comment: comment.ID,
	})
	ref, err := l.designationRef(entity, comment.Dir)
	if err != nil {
		return "", l.FromError(req, err)
	}
	return ref, nil
}

// designationOf admits a reference naming a comment of the item being settled
// and refuses every other one.
//
// The holder check is the whole of what makes a designation openable without
// asking whose answer it is: a reference reaching another item's comment, a
// card comment or a column comment resolves perfectly well and would store a
// reference to somebody else's words as this item's answer. What is stored is
// the canonical reference rather than the caller's spelling, so two callers
// typing one comment two ways record one value.
func (l *Library) designationOf(req *Request, entity *itemTarget, named string) (string, *Response) {
	// A bare identifier is a member selector inside this item's own comments
	// rather than a whole reference, so it is composed under the item before
	// the resolver sees it. Both forms reach the same comment and both store
	// the same identifier.
	if bench.IsID(named) {
		named = entity.ref + "/" + bench.CommentsDir + "/" + named
	}
	found, err := l.Bench.ResolveEntity(named)
	if err != nil {
		return "", l.refuse(req, entity.card, contract.NotADesignation, named)
	}
	if found.Kind != bench.KindComment {
		return "", l.refuse(req, entity.card, contract.NotADesignation, named)
	}
	if !sameDir(filepath.Dir(filepath.Dir(found.Dir)), entity.dir) {
		return "", l.refuse(req, entity.card, contract.NotADesignation, named)
	}
	ref, err := l.designationRef(entity, found.Dir)
	if err != nil {
		return "", l.FromError(req, err)
	}
	return ref, nil
}

// sameDir compares two directory paths for the one question this file asks of
// them, which is whether a comment hangs below the item being settled. The
// comparison is Clean's rather than a byte one, because one path was composed
// from a resolved entity's own directory and the other by climbing out of a
// comment's, and the two spellings need not agree character for character.
func sameDir(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

// designationRef answers the value a settling stores as the item's answer of
// record, which is the designated comment's own 12-hex identifier.
//
// It used to compose a positional reference, and a position is not an
// identity. Archiving any earlier comment of the item renumbers the survivors,
// so the stored reference came to name a different comment and the item went
// on citing an answer nobody had written for it; deleting one reached the same
// place and left no archive behind to notice. The identifier is minted when
// the comment is written and is never rewritten by anything.
//
// The identifier is read out of the comment's own directory name, which is
// what it is, rather than resolved: the key sits on the item, a designation
// names a comment of that same item, and designationOf refuses anything else,
// so the item's own comments are the scope the identifier is read in and no
// path is needed to disambiguate it.
func (l *Library) designationRef(entity *itemTarget, commentDir string) (string, error) {
	return filepath.Base(commentDir), nil
}

// itemCanonicalRef composes the reference a person types to reach one item of
// one card: the card's own reference, the item's kind word and its position
// among the items of that kind.
//
// Two callers need it and each needs it for a reference that has to resolve
// rather than merely read well. A settling stores it as the designation, and a
// forced deletion hands it to Reopen, which resolves what it is given; an
// item's bare identifier does not resolve, so handing that over is how the
// forced form came back unknown-card.
func (l *Library) itemCanonicalRef(card *bench.Card, itemID string) (string, error) {
	items, err := l.Bench.Items(card.Dir)
	if err != nil {
		return "", err
	}
	cardRef := card.Ref(l.Bench.Slug)
	kindPosition := map[string]int{}
	for _, item := range items {
		kindPosition[item.Kind]++
		if item.ID != itemID {
			continue
		}
		position, err := l.memberPosition(item.Dir, bench.ItemAnchor)
		if err != nil {
			return "", err
		}
		return itemRef(cardRef, item.Kind, kindPosition[item.Kind], position), nil
	}
	return "", contract.Refuse(contract.UnknownPath, itemID)
}
