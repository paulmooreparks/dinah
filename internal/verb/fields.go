package verb

import (
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// GetField reports one field of one entity, whatever kind the reference
// resolves to. Reading is open to anybody, so no owner is required and no
// operator is asked for, on the terms every other read in the tool is open.
//
// It evaluates rows 2 and 3 of the check list and no more, because a read
// validates nothing: an entity carrying a level the workbench no longer
// declares is reported rather than refused, and a field the entity has never
// been given reports the empty string.
func (l *Library) GetField(req *Request) (string, error) {
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return "", err
	}
	field, known := bench.FieldOf(entity.Kind, req.Field)
	if !known {
		return "", unknownEntityField(entity.Kind, req.Field)
	}
	return l.readField(entity, field)
}

// SetField writes one field of one entity, whatever kind the reference
// resolves to, and clears it where the field declares itself clearable and the
// request carries no value.
//
// It evaluates in the order the check table fixes, which
// internal/verb/checks.go draws as this command's own list: the workbench
// designates an operator, the reference resolves to one entity, the field is
// one the resolved kind records, the value is present where the field may not
// be cleared and is one line where the field is not prose, the field's own
// guard admits the value, the request names an owner, that owner is the
// operator where the kind's write authority is the operator's, and a write to
// a slug field carries the confirmation flag.
//
// Four guards route to the verb that already checks them rather than
// reimplementing it: a card's tier with --at goes to SetCardTierAt, an item's
// state goes to the checklist verb that lands that state, and an attachment's
// filename goes to Rename so the payload moves with the name. What those
// verbs check, refuse and journal is theirs and is not restated here.
func (l *Library) SetField(req *Request) *Response {
	if l.Bench.Operator == "" {
		return l.refuse(req, nil, contract.NoOperator, "")
	}
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return l.FromError(req, err)
	}
	field, known := bench.FieldOf(entity.Kind, req.Field)
	if !known {
		return l.FromError(req, unknownEntityField(entity.Kind, req.Field))
	}
	// The two flags are legal beside one field each, and every other use is a
	// usage error rather than a value quietly ignored. The check sits here
	// rather than on either head, because a caller reaching this library over
	// MCP has to meet the same refusal a caller at a terminal meets.
	if strings.TrimSpace(req.At) != "" && !(entity.Kind == bench.KindCard && field.Name == bench.TierField) {
		return l.refuse(req, entity.Card, contract.Usage, "--at")
	}
	if strings.TrimSpace(req.Note) != "" && field.Name != bench.ItemStateField {
		return l.refuse(req, entity.Card, contract.Usage, "--note")
	}
	value := req.Value
	if !field.Prose {
		value = strings.TrimSpace(value)
	}
	if value == "" && !field.Clearable {
		return l.refuse(req, entity.Card, contract.Malformed, field.Name)
	}
	if !field.Prose && strings.ContainsAny(value, "\r\n") {
		return l.refuseWith(req, entity.Card, contract.Malformed, field.Name, map[string]string{
			"oneLine": "1",
		})
	}
	if routed, response := l.routeGuardedWrite(req, entity, field, value); routed {
		return response
	}
	if value != "" {
		if refused := l.admitFieldValue(req, entity, field, value); refused != nil {
			return refused
		}
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if bench.WriteAuthorityOf(entity.Kind) == bench.AuthorityOperator && req.Actor != l.Bench.Operator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	if refused := l.admitOwnerWrite(req, entity, field, value); refused != nil {
		return refused
	}
	if field.Guard == bench.GuardSlug && !req.Confirm {
		// A workbench slug change renames every card of the workbench with
		// it, which a column's and a workstream's do not, so the sentence
		// splices that clause on this kind alone.
		var extra map[string]string
		if entity.Kind == bench.KindWorkbench {
			extra = map[string]string{"renamesCards": "1"}
		}
		return l.refuseWith(req, entity.Card, contract.Unconfirmed, value, extra)
	}
	// The value a reader types is any spelling of a column, and the value
	// stored is that column's identifier, because a gate reads the field by
	// identifier equality. admitFieldValue has already refused a spelling
	// that resolves to nothing, so the lookup here cannot come back empty,
	// and a clear carries no value to resolve.
	if field.Guard == bench.GuardColumnRef && value != "" {
		value = l.Bench.ColumnByRef(value).ID
	}
	if field.Guard == bench.GuardHold {
		return l.writeHold(req, entity, field, value)
	}
	return l.writeField(req, entity, field, value)
}

// writeHold performs a hold write in the storage spelling and answers in the
// typed one, so the two words a person types are the two words the answer
// carries and `gate_items` reaches nothing a person reads.
//
// The detail is rewritten only where writeField reports back the value it was
// given, which is the successful write and the write that found the value
// already there. A refusal composed further down carries its own detail, and
// swapping that one for on or off would put a word in a sentence about
// something else.
func (l *Library) writeHold(req *Request, entity *bench.EntityRef, field bench.Field, typed string) *Response {
	stored := storedHold(typed)
	response := l.writeField(req, entity, field, stored)
	if response.Outcome == contract.OutcomeOK && response.Detail == stored {
		response.Detail = typed
	}
	return response
}

// admitOwnerWrite refuses a non-operator's write to a checklist item's owner
// key where the operator owns the item already, or would own it once the write
// landed. Without it the refusal closeItem raises is a refusal any value
// satisfies: whoever wanted to settle an item the operator owns would take the
// item out of the operator's name first and then settle it, and the record the
// refusal reads would be the record they had just rewritten.
//
// It stands here rather than beside the field guards above because a clear
// runs no guard, and clearing the key removes the operator's name exactly as
// overwriting it does. Filing a fresh item in the operator's name stays open to
// everybody, because that is the ordinary act of routing a question to the
// operator and it is AddItem's path rather than this one.
func (l *Library) admitOwnerWrite(req *Request, entity *bench.EntityRef, field bench.Field, value string) *Response {
	if entity.Kind != bench.KindItem || field.Name != bench.ItemOwnerField {
		return nil
	}
	if req.Actor == l.Bench.Operator {
		return nil
	}
	if value == bench.ItemOwnerOperator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	fm, _, err := l.entityAnchor(entity)
	if err != nil {
		return l.FromError(req, err)
	}
	if fm.Value(bench.ItemOwnerField) == bench.ItemOwnerOperator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	return nil
}

// routeGuardedWrite hands a write to the verb that already performs it, and
// reports whether it did. Three guards route: a card's tier when the request
// names a column, an item's state, and an attachment's filename. Everything
// else is written here.
//
// A routed write runs the destination verb's own check list rather than this
// one's, which is the point of routing: the kind check an item's state write
// needs, the citation obligation an acceptance criterion carries, and the
// payload move a rename performs all come from the verb rather than from a
// second implementation of it.
func (l *Library) routeGuardedWrite(req *Request, entity *bench.EntityRef, field bench.Field, value string) (bool, *Response) {
	switch field.Guard {
	case bench.GuardTier:
		if strings.TrimSpace(req.At) == "" {
			return false, nil
		}
		routed := *req
		routed.Card = req.Ref
		return true, l.SetCardTierAt(&routed)
	case bench.GuardState:
		return true, l.setItemState(req, entity, value)
	case bench.GuardFilename:
		return true, l.Rename(req)
	}
	return false, nil
}

// setItemState routes an item's state write to the checklist verb that lands
// that state, so the kind check, the pending check, the citation obligation
// and the journal event all come from the verb. A value outside the four
// states is refused before any of them, because none of the four verbs would
// know what to do with it.
//
// The note --note carries fills whichever slot the destination verb reads: the
// resolution note on the three terminal verbs, and the reason on a reopen. So
// `dinah set <item> state verified --note "ran the suite"` and `dinah verify
// <item> "ran the suite"` are one act written two ways.
func (l *Library) setItemState(req *Request, entity *bench.EntityRef, value string) *Response {
	landing := map[string]func(*Request) *Response{
		bench.ItemResolved: l.Resolve,
		bench.ItemVerified: l.Verify,
		bench.ItemFailed:   l.Fail,
		bench.ItemPending:  l.Reopen,
	}
	land, legal := landing[value]
	if !legal {
		return l.refuseWith(req, entity.Card, contract.UnknownValue, value, map[string]string{
			"term":  bench.ItemStateField,
			"field": bench.ItemStateField,
			"legal": strings.Join(bench.ItemStates, ", "),
		})
	}
	routed := *req
	routed.Reason = req.Note
	return land(&routed)
}

// admitFieldValue runs the guard a field declares over a value that is being
// written rather than cleared. A clear runs no guard, which is the rule a
// level write has always held: the checks are about the value, and a clear
// has none.
//
// Every guard here is the check the field's own create path already applies,
// so a value legal to file stays legal to correct.
func (l *Library) admitFieldValue(req *Request, entity *bench.EntityRef, field bench.Field, value string) *Response {
	switch field.Guard {
	case bench.GuardSlug:
		valid := bench.ValidColumnSlug
		if entity.Kind == bench.KindWorkbench {
			valid = bench.ValidSlug
		}
		if !valid(value) {
			return l.refuse(req, entity.Card, contract.Malformed, field.Name)
		}
	case bench.GuardLevel, bench.GuardTier:
		axis := field.Name
		if refusal := l.admitLevels(map[string]string{axis: value}); refusal != nil {
			return l.refuseWith(req, entity.Card, refusal.Name, refusal.Detail, refusal.Extra)
		}
	case bench.GuardKind:
		if !bench.ValidColumnKind(value) {
			return l.refuse(req, entity.Card, contract.Malformed, field.Name)
		}
	case bench.GuardCapacity:
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return l.refuse(req, entity.Card, contract.Malformed, field.Name)
		}
	case bench.GuardHold:
		switch value {
		case bench.HoldOn, bench.HoldOff, bench.HoldOut, bench.HoldBoth:
		default:
			return l.refuse(req, entity.Card, contract.Malformed, field.Name)
		}
	case bench.GuardColumnRef:
		if l.Bench.ColumnByRef(value) == nil {
			return l.refuse(req, entity.Card, contract.UnknownColumn, value)
		}
	}
	return nil
}

// readField reports what an entity stores under one field: the anchor's body
// where the field is prose, and the frontmatter key of the field's own name
// otherwise.
func (l *Library) readField(entity *bench.EntityRef, field bench.Field) (string, error) {
	fm, body, err := l.entityAnchor(entity)
	if err != nil {
		return "", err
	}
	if field.Prose {
		return body, nil
	}
	if field.Guard == bench.GuardHold {
		return typedHold(fm.Value(field.Stored())), nil
	}
	return fm.Value(field.Stored()), nil
}

// typedHold reports a column's hold in the words a person types, given what
// the anchor stores. The stored spelling is the profile's `gate_items`, whose
// value is exactly true, false, out, both, or absent, and bench.Open refuses a
// workbench carrying anything else, so the five readings this collapses to
// four typed words are the five that can reach it.
//
// The translation lives here and in storedHold below rather than in
// writeField, because the storage spelling is what every other reader of the
// anchor already expects and the typed spelling is what only this one field's
// two commands use.
func typedHold(stored string) string {
	switch stored {
	case "true":
		return bench.HoldOn
	case "out":
		return bench.HoldOut
	case "both":
		return bench.HoldBoth
	}
	return bench.HoldOff
}

// storedHold reports what the anchor carries for a hold a person has just
// typed. On stores true, which is the spelling the field was born with and
// which every workbench written before the direction existed already carries.
// Off clears the key rather than writing false, which is what writeField's own
// empty-value branch does with it, and the strict parser in internal/bench
// reads an absent key and an explicit false identically, so clearing never
// produces a second on-disk spelling of off.
func storedHold(typed string) string {
	switch typed {
	case bench.HoldOn:
		return "true"
	case bench.HoldOut:
		return bench.HoldOut
	case bench.HoldBoth:
		return bench.HoldBoth
	}
	return ""
}

// entityAnchor reads an entity's anchor into its header and its body. Reading the
// header rather than the loaded entity is what lets a write put back every key
// it did not touch, which is CORE-CARD-9's guarantee for a key this build has
// never heard of.
func (l *Library) entityAnchor(entity *bench.EntityRef) (*bench.Frontmatter, string, error) {
	path, declared := bench.AnchorPathOf(entity)
	if !declared {
		return nil, "", contract.Refuse(contract.UnknownPath, entity.Ref)
	}
	text, err := bench.ReadText(path)
	if err != nil {
		return nil, "", contract.Refuse(contract.UnknownPath, entity.Ref)
	}
	fm, body := bench.ParseAnchor(text)
	return fm, body, nil
}

// writeField performs the write every unrouted field takes: the anchor is
// reread under the lock of the nearest enclosing journal-bearing entity, the
// one field is rewritten, and one line is appended to that entity's journal.
//
// Rereading under the lock is what SetCardTierAt and the card's own level
// write both do, and for their reason: the anchor is rendered whole from the
// header being held, so a copy read before the lock would revert whatever
// landed after it was read.
//
// A write storing the value the entity already carries succeeds, writes
// nothing and journals nothing, on the terms join already returns ok for a
// workstream the card already belongs to.
func (l *Library) writeField(req *Request, entity *bench.EntityRef, field bench.Field, value string) *Response {
	now := bench.Stamp(l.Now())
	lock, err := bench.Acquire(l.lockDirFor(entity), req.Actor, now)
	if err != nil {
		return l.FromError(req, err)
	}
	defer lock.Release()
	if l.Interleave != nil {
		l.Interleave()
	}
	fm, body, err := l.entityAnchor(entity)
	if err != nil {
		return l.FromError(req, err)
	}
	was := fm.Value(field.Stored())
	if field.Prose {
		was = body
		body = value
	} else if value == "" {
		fm.Delete(field.Stored())
	} else {
		fm.Set(field.Stored(), value)
	}
	if was == value {
		response := l.ok(req, entity.Card)
		response.Detail = value
		return response
	}
	// entityAnchor above has already refused a kind declaring no anchor, so
	// this reads the same join rather than guarding it a second time; the
	// second answer is checked because swallowing it is what let a caller
	// join an empty filename and get a directory back.
	path, declared := bench.AnchorPathOf(entity)
	if !declared {
		return l.FromError(req, contract.Refuse(contract.UnknownPath, entity.Ref))
	}
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		return l.FromError(req, err)
	}
	ev := fieldEvent(entity, field, was, value)
	ev.TS = now
	ev.Actor = req.Actor
	if err := bench.AppendEvent(l.journalFor(entity), ev); err != nil {
		return l.FromError(req, err)
	}
	if entity.Kind == bench.KindWorkbench {
		l.Bench.SetWorkbenchField(field.Name, value)
	}
	return l.wroteField(req, entity, value)
}

// wroteField composes the ok response a write answers with, reading the
// written entity back off disk where the answer draws it.
//
// The reread is what stops the answer contradicting the write. A card's line
// carries the card's own title, and a workstream's carries its title and its
// status, so an answer built from the copy resolved before the write would
// print the value the caller has just replaced. Every other kind draws no line
// of its own, and the ok line carries the written value as its detail.
func (l *Library) wroteField(req *Request, entity *bench.EntityRef, value string) *Response {
	card := entity.Card
	if entity.Kind == bench.KindCard {
		if reloaded, err := bench.LoadCard(filepath.Dir(entity.Dir), entity.ID); err == nil {
			card = reloaded
		}
	}
	response := l.ok(req, card)
	response.Detail = value
	if entity.Kind != bench.KindWorkstream {
		return response
	}
	workstream := l.Bench.Workstream(entity.ID)
	if workstream == nil {
		return response
	}
	counts, err := l.Bench.WorkstreamCounts()
	if err != nil {
		return response
	}
	view := workstreamView(workstream, counts)
	response.Workstream = &view
	return response
}

// fieldEvent composes the journal line a field write appends: the written
// entity's own *_updated event, the field's name, and the entity's own
// identifier in Note where the event lands on somebody else's journal.
//
// A prose write carries neither From nor To. The journal records that an act
// happened and who did it; the prose itself lives in the anchor, which is the
// file that changed, and copying a whole instructions body into an append-only
// journal on every edit would grow the journal without bound and put a second
// copy of the text where nobody edits it.
func fieldEvent(entity *bench.EntityRef, field bench.Field, was, value string) bench.Event {
	ev := bench.Event{Event: updatedEvents[entity.Kind], Field: field.Name}
	if !field.Prose {
		ev.From = was
		ev.To = value
	}
	switch entity.Kind {
	case bench.KindColumn, bench.KindComment, bench.KindItem, bench.KindAttachment:
		ev.Note = entity.ID
	}
	return ev
}

// updatedEvents names the *_updated event of each written kind. The event is
// the written entity's own rather than the journal-holder's, so a query for
// the card's own field changing is still a question a reader can ask.
var updatedEvents = map[string]string{
	bench.KindWorkbench:  contract.EventWorkbenchUpdated,
	bench.KindColumn:     contract.EventColumnUpdated,
	bench.KindCard:       contract.EventCardUpdated,
	bench.KindComment:    contract.EventCommentUpdated,
	bench.KindItem:       contract.EventItemUpdated,
	bench.KindAttachment: contract.EventAttachmentUpdated,
	bench.KindWorkstream: contract.EventWorkstreamUpdated,
}

// unknownEntityField raises row 3 of the check list. It raises dinah.unknown-field
// rather than dinah.unknown-key because that shape carries its field set as a
// value the raise site fills, where unknown-key declares a Listing the head
// resolves to the config settings for every raise site of the name.
//
// The set is read off bench.FieldsOf for the kind that was resolved rather
// than out of a catalog sentence, so a reader who typed an item's field at a
// card is told what a card records rather than what any entity records, and a
// field added later reaches the sentence without a translator being asked for
// anything.
func unknownEntityField(kind, field string) error {
	return contract.RefuseWith(contract.UnknownField, field, map[string]string{
		"kind":   kind,
		"fields": strings.Join(bench.FieldsOf(kind), ", "),
	})
}
