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
		// A read never refuses on the ground that a key is undeclared, which
		// is what makes CORE-LAYER-2's preservation visible: a key an
		// imported card carries, or one somebody wrote by hand, survives a
		// read and a write of its neighbours and can be read back. A name
		// with no full stop is not a declared field key at all, so it keeps
		// today's refusal, which lists what the resolved kind records.
		if !bench.DeclaredFieldKey(req.Field) {
			return "", unknownEntityField(entity.Kind, req.Field)
		}
		fm, _, err := l.entityAnchor(entity)
		if err != nil {
			return "", err
		}
		return bench.FieldValue(fm, req.Field), nil
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
		if bench.DeclaredFieldKey(req.Field) {
			return l.setDeclaredField(req, entity)
		}
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
	// The value is normalised here rather than only on the way to disk,
	// because writeField compares the parsed stored value against this one to
	// decide whether there is anything to write. The parsed value carries no
	// carriage return, so an unnormalised request value makes that comparison
	// see an inequality that is not there, takes the write branch, and appends
	// an updated journal line whose from and to are the same prose. A get, a
	// CRLF-ification and a set has to be able to journal nothing.
	value := bench.NormalizeNewlines(req.Value)
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
	if l.writeAuthorityOf(entity) == bench.AuthorityOperator && req.Actor != l.Bench.Operator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	if refused := l.admitOwnerWrite(req, entity, field, value); refused != nil {
		return refused
	}
	if refused := l.admitItemColumnWrite(req, entity, field); refused != nil {
		return refused
	}
	if refused := l.admitDesignationClear(req, entity, field, value); refused != nil {
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
	// The two route rows are last, because both read the card rather than the
	// value being written and neither is a guard. A clear is never refused on
	// either ground: the default route carries every column, so clearing can
	// strand no item and skip no station.
	if field.Guard == bench.GuardRoute && value != "" {
		if refused := l.admitRouteWrite(req, entity, value); refused != nil {
			return refused
		}
	}
	// Moving an item onto a column the card's road does not carry reads the
	// card too, so it runs here with the two rows above rather than inside
	// the guard, and it has a help row of its own for the same reason.
	if field.Guard == bench.GuardColumnRef && value != "" {
		if refused := l.admitItemColumnRoute(req, entity, value); refused != nil {
			return refused
		}
	}
	// The value a reader types is any spelling of a column, and the value
	// stored is that column's identifier, because a gate reads the field by
	// identifier equality. admitFieldValue has already refused a spelling
	// that resolves to nothing, so the lookup here cannot come back empty,
	// and a clear carries no value to resolve.
	if field.Guard == bench.GuardColumnRef && value != "" {
		value = l.Bench.ColumnByRef(value).ID
	}
	// The same rule applied to an item's answer of record. What a person
	// types is any spelling of a comment of that item, a position or the
	// comment's own identifier, and the value stored is the identifier,
	// because a position is not an identity: archiving an earlier comment
	// renumbers the survivors and the stored reference comes to name a
	// different comment. admitResolutionValue has already refused a spelling
	// that reaches anything but a comment of this item, so the resolution
	// here cannot come back empty, and a clear carries no value to resolve.
	if field.Guard == bench.GuardResolution && value != "" {
		found, err := l.Bench.ResolveEntity(designationSpelling(entity, value))
		if err != nil {
			return l.FromError(req, err)
		}
		value = found.ID
	}
	if field.Guard == bench.GuardHold {
		return l.writeHold(req, entity, field, value)
	}
	return l.writeField(req, entity, builtInTarget(field), value)
}

// fieldWrite is where one write puts its value and what the journal calls it.
// The built-in fields and the declared ones differ in exactly these answers,
// so one writer serves both rather than two writers drifting apart over the
// lock discipline, the no-op rule and the journal line.
type fieldWrite struct {
	// name is what the event's field slot carries and what the ok response
	// reports back, which is the name a reader typed.
	name string
	// key is the frontmatter key the value is stored under, unread on a prose
	// write, which has no key at all.
	key string
	// prose is true where the value is the anchor's body rather than a
	// frontmatter key.
	prose bool
	// nested is true where the value is stored inside the field_values block
	// rather than at the top level of the anchor. The top level of a
	// workbench definition is the namespace a layer declares itself in, and a
	// declared field key carries a full stop by construction, so a value
	// written there could not be told from a layer declaration.
	nested bool
}

// builtInTarget is where a field of a kind's own declared set is written.
func builtInTarget(field bench.Field) fieldWrite {
	return fieldWrite{name: field.Name, key: field.Stored(), prose: field.Prose}
}

// declaredTarget is where a workbench-declared field is written, which is
// inside the entity's own field_values block under the key itself.
func declaredTarget(key string) fieldWrite {
	return fieldWrite{name: key, key: key, nested: true}
}

// setDeclaredField writes one workbench-declared field, in the order the
// contract fixes and the order that decides which name a caller meets: the
// workbench declares the key, the declaration reaches the kind the reference
// resolved to, the value satisfies the declared type, and then the write runs
// down SetField's own remaining list, which is the owner check, the kind's
// write authority and the journalled rewrite under the entity's lock.
//
// The write authority is the kind's and is unchanged here. A declared field on
// a card is any owner's to write, and one on a column or on the workbench is
// the operator's, because WriteAuthorityOf is read below exactly as it is read
// for a field of the kind's own set.
//
// A write to a comment, a checklist item or an attachment is refused whatever
// the key, and no branch here says so: a declaration reaches none of those
// three, so DeclaredFieldOf answers a field no declaration can reach and the
// first row below refuses it.
//
// It needs no line normalising the request's value, and the asymmetry with
// SetField above is a decision rather than an oversight. AdmitsFieldValue
// refuses any value containing a carriage return or a line feed, so a CRLF is
// refused before normalisation and the LF it would become is refused after, and
// no input exists for which such a line could change an outcome. A line no
// criterion can arm is a line nobody can prove right.
func (l *Library) setDeclaredField(req *Request, entity *bench.EntityRef) *Response {
	// Neither flag is legal beside a declared field, and the refusal is the
	// one a flag carried beside the wrong field of a kind's own set meets.
	if strings.TrimSpace(req.At) != "" {
		return l.refuse(req, entity.Card, contract.Usage, "--at")
	}
	if strings.TrimSpace(req.Note) != "" {
		return l.refuse(req, entity.Card, contract.Usage, "--note")
	}
	declared := l.Bench.DeclaredFieldOf(req.Field)
	if declared == nil || !declared.Declares(entity.Kind) {
		return l.FromError(req, undeclaredField(l.Bench, entity.Kind, req.Field, declared))
	}
	value := strings.TrimSpace(req.Value)
	// Every declared field is clearable, because what makes a field required
	// at a point in the flow is a column's own require_fields and not the
	// declaration. A write carrying no value deletes the key and runs no
	// guard, which is the rule a level write already keeps.
	if value != "" && !bench.AdmitsFieldValue(declared.Type, value) {
		return l.refuse(req, entity.Card, contract.Malformed, req.Field)
	}
	// The applicability guard runs after the value checks and before the
	// owner and operator checks, so a malformed value is still refused for
	// being malformed, and it runs only on a card because no condition
	// reaches any other kind. A clearing write never meets it, so an
	// orphaned value can always be removed.
	if value != "" && entity.Kind == bench.KindCard {
		if refusal := l.inapplicable(entity.Card, req.Field); refusal != nil {
			return l.refuseWith(req, entity.Card, refusal.Name, refusal.Detail, refusal.Extra)
		}
	}
	if req.Actor == "" {
		return l.refuse(req, entity.Card, contract.NoOwner, "")
	}
	if l.writeAuthorityOf(entity) == bench.AuthorityOperator && req.Actor != l.Bench.Operator {
		return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
	}
	// A write to a gate is never refused for what it does to the slots it
	// governs, and it says so when it leaves a value behind: the first slot
	// that was applicable before the write and is not after it is named in
	// the warning, and the value is kept. The comparison is against the
	// card as it stood, so a value already orphaned before the write draws
	// no second warning.
	var orphanedBefore map[string]bool
	if entity.Kind == bench.KindCard {
		orphanedBefore = map[string]bool{}
		for _, orphaned := range l.Bench.OrphanedValues(entity.Card) {
			orphanedBefore[orphaned.Slot] = true
		}
	}
	response := l.writeField(req, entity, declaredTarget(req.Field), value)
	if response.Outcome != contract.OutcomeOK || entity.Kind != bench.KindCard {
		return response
	}
	written, err := l.Bench.LoadCardIn(filepath.Dir(entity.Dir), entity.ID)
	if err != nil {
		return response
	}
	for _, orphaned := range l.Bench.OrphanedValues(written) {
		if orphanedBefore[orphaned.Slot] {
			continue
		}
		response.Warning = "warn.inapplicable-value"
		response.WarningDetail = orphaned.Slot
		break
	}
	return response
}

// inapplicable raises the refusal a non-empty write to a slot the card's own
// condition does not admit meets, and answers nil where the slot applies. The
// slot is a level axis or a declared field key, and a nil card is one being
// filed, which stores nothing.
//
// The context carries the gate, the values that admit the slot, and the
// gate's stored value where it stores one. The value is the sentence's
// subject, so a gate storing nothing selects the unset sibling of the
// sentence rather than leaving a hole in it.
func (l *Library) inapplicable(card *bench.Card, slot string) *contract.Refusal {
	answer := l.Bench.Applicability(card, slot)
	if answer.Applies {
		return nil
	}
	condition := l.Bench.ConditionOn(slot)
	context := map[string]string{
		"gate":   answer.Gate,
		"admits": strings.Join(condition.Is, ", "),
	}
	if answer.GateValue != "" {
		context["value"] = answer.GateValue
	}
	return contract.RefuseWith(contract.InapplicableField, slot, context)
}

// undeclaredField raises the refusal a write naming a key the workbench does
// not declare on the resolved kind meets. The rows are the keys the workbench
// does declare on that kind, drawn from the declaration rather than from a
// sentence, so a workbench that declares a key later reaches the message with
// nobody editing a catalog.
//
// A key the workbench declares on some other kind fills the kinds value too,
// which is what switches on the clause telling the reader the key exists
// somewhere rather than nowhere.
func undeclaredField(b *bench.Bench, kind, key string, declared *bench.DeclaredField) error {
	context := map[string]string{
		"kind":     kind,
		"declared": strings.Join(b.DeclaredFieldKeysOn(kind), "\n"),
	}
	if declared != nil {
		context["kinds"] = strings.Join(declared.Kinds(), ", ")
	}
	return contract.RefuseWith(contract.UndeclaredField, key, context)
}

// writeHold performs a hold write in the storage spelling and answers in the
// typed one, so the word a person types is the word the answer carries and
// `gate_items` reaches nothing a person reads.
//
// The detail is rewritten only where writeField reports back the value it was
// given, which is the successful write and the write that found the value
// already there. A refusal composed further down carries its own detail, and
// swapping that one for a typed hold value would put a word in a sentence
// about something else.
func (l *Library) writeHold(req *Request, entity *bench.EntityRef, field bench.Field, typed string) *Response {
	stored := storedHold(typed)
	response := l.writeField(req, entity, builtInTarget(field), stored)
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

// admitItemColumnWrite refuses a non-operator's write to, or clear of, a
// checklist item's column key on an item the gate is protecting.
//
// The item's column key names the station that settles it, and until this card
// anybody could write it and anybody could clear it with no validation on the
// clear at all. That is the door beside the gate. Agent Design Review walked a
// card into a done column past an acceptance criterion standing at failed, in
// three commands, by clearing the key the refusal read: the move was refused,
// the clear was accepted, and the same move then succeeded with the criterion
// still failed and no override marker anywhere.
//
// The rule is the kind guard of Withdraw applied to the field instead of to
// the verb. An acceptance criterion's station is the operator's, whatever the
// item's state and whatever its owner key says, and an operator-owned item's
// station is his whatever its kind. Every other item stays repairable by
// anybody, which is what the workbench instruction about repairing a misfiled
// item depends on.
//
// It stands here beside admitOwnerWrite rather than among the value guards for
// exactly admitOwnerWrite's reason: a clear runs no value guard, and
// admitFieldValue is reached only when the value is non-empty, so a rule
// written as a value guard would refuse the write and admit the erasure.
//
// Re-pointing a criterion at a different station is never the repair. Filing
// is open to everybody, so the wrong item is withdrawn and a replacement is
// filed, which leaves a complete record where a silent re-pointing leaves
// none.
func (l *Library) admitItemColumnWrite(req *Request, entity *bench.EntityRef, field bench.Field) *Response {
	if entity.Kind != bench.KindItem || field.Name != bench.ItemColumnField {
		return nil
	}
	if req.Actor == l.Bench.Operator {
		return nil
	}
	fm, _, err := l.entityAnchor(entity)
	if err != nil {
		return l.FromError(req, err)
	}
	if fm.Value(bench.ItemKindField) != criterionKind &&
		fm.Value(bench.ItemOwnerField) != bench.ItemOwnerOperator {
		return nil
	}
	return l.refuse(req, entity.Card, contract.NotOperator, req.Actor)
}

// admitDesignationClear refuses a clear of an item's answer of record on an
// item that is not pending.
//
// An erasable reason is not a requirement. Both the waiver and the withdrawal
// are made to record why, and the key they record it in was settable and
// clearable by anybody, so a waiver's recorded reason could be cleared off a
// waived item and leave the item asserting that somebody decided something
// while carrying no record of who or why.
//
// Rewriting the key to another comment of the same item stays open under the
// guard it already has, because that is a correction rather than an erasure.
// Reopen clears the key inside the verb rather than through this path, so the
// one legitimate erasure is untouched, and it is the one the refusal's next
// step names.
func (l *Library) admitDesignationClear(req *Request, entity *bench.EntityRef, field bench.Field, value string) *Response {
	if entity.Kind != bench.KindItem || field.Name != bench.ItemResolutionField || value != "" {
		return nil
	}
	item, err := bench.LoadItem(entity.Dir)
	if err != nil {
		return l.FromError(req, err)
	}
	if item.State == bench.ItemPending {
		return nil
	}
	return l.refuse(req, entity.Card, contract.DesignationRequired, item.State)
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
// and the journal event all come from the verb. A value outside the six
// states is refused before any of them, because none of the six verbs would
// know what to do with it.
//
// The value --note carries fills whichever slot the destination verb reads:
// the designation on the three terminal verbs, and the reason on a reopen. So
// `dinah set <item> state verified --note <comment>` and `dinah verify <item>
// <comment>` are one act written two ways.
//
// The two slots take different kinds of value, and that asymmetry is the
// shape's rather than this router's. A terminal verb's answer is a reference
// to a comment of the item, because a comment carries its own author and the
// designation carries whoever chose it; a reopen's reason is prose, because it
// says why an answer stopped standing and the thing it refers to is often
// being destroyed. One flag reaches both, and which it means is decided by the
// state being written rather than by anything the caller has to spell.
//
// The one-command form has no spelling here. `dinah verify <item> --text
// "ran the suite"` mints the comment and designates it, and a caller who
// wants that writes the verb rather than the field.
func (l *Library) setItemState(req *Request, entity *bench.EntityRef, value string) *Response {
	landing := map[string]func(*Request) *Response{
		bench.ItemResolved:  l.Resolve,
		bench.ItemVerified:  l.Verify,
		bench.ItemFailed:    l.Fail,
		bench.ItemWaived:    l.Waive,
		bench.ItemWithdrawn: l.Withdraw,
		bench.ItemPending:   l.Reopen,
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
		// The applicability guard follows the level checks, so a level the
		// workbench does not declare is refused for that before anything
		// asks whether the axis applies to this card.
		if refusal := l.inapplicable(entity.Card, axis); refusal != nil {
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
		if !bench.KnownHold(value) {
			return l.refuse(req, entity.Card, contract.Malformed, field.Name)
		}
	case bench.GuardColumnRef:
		if l.Bench.ColumnByRef(value) == nil {
			return l.refuse(req, entity.Card, contract.UnknownColumn, value)
		}
	case bench.GuardRoute:
		// The guard admits a name the workbench declares and refuses every
		// other, resolving nothing, because a route name is stored as typed.
		if !l.Bench.DeclaresRoute(value) {
			return l.unknownRoute(req, entity.Card, value)
		}
	case bench.GuardResolution:
		// A designation names a comment of the very item being written, and
		// the check is the one the terminal verbs run, so a resolution
		// written by hand cannot reach a state a settling could not have
		// produced.
		if refused := l.admitResolutionValue(req, entity, value); refused != nil {
			return refused
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
func (l *Library) writeField(req *Request, entity *bench.EntityRef, target fieldWrite, value string) *Response {
	if refused := l.malformedHarness(req, entity.Card); refused != nil {
		return refused
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
	fm, body, err := l.entityAnchor(entity)
	if err != nil {
		return l.FromError(req, err)
	}
	if entity.Kind == bench.KindComment {
		if refused := l.admitCommentWrite(req, entity, fm, body); refused != nil {
			return refused
		}
	}
	var was string
	switch {
	case target.prose:
		was = body
		body = value
	case target.nested:
		was = bench.FieldValue(fm, target.key)
		bench.SetFieldValue(fm, target.key, value)
	case value == "":
		was = fm.Value(target.key)
		fm.Delete(target.key)
	default:
		was = fm.Value(target.key)
		fm.Set(target.key, value)
	}
	if was == value && !restampsComment(entity, fm, value) {
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
	// A comment's anchor is written through bench.WriteCommentAnchor and
	// through nothing else, which is what makes "the digest is recomputed by
	// every verb that writes the anchor" a property of one function rather
	// than a rule each call site has to remember. This site used to stamp
	// the digest and then write the file itself, which held the property by
	// remembering it here.
	write := func() error { return bench.WriteText(path, fm.Render(body)) }
	if entity.Kind == bench.KindComment {
		write = func() error { return bench.WriteCommentAnchor(entity.Dir, fm, body) }
	}
	if err := write(); err != nil {
		return l.FromError(req, err)
	}
	ev := fieldEvent(req, entity, target, was, value)
	locateColumnAttachment(&ev, l.attachmentColumn(entity))
	ev.TS = now
	if err := bench.AppendEvent(l.journalFor(entity), ev); err != nil {
		return l.FromError(req, err)
	}
	if entity.Kind == bench.KindWorkbench {
		if target.nested {
			// The bench's own header is the copy every later read in this
			// process draws from, and the write above rendered the file from
			// a header read afresh under the lock, so the two are reconciled
			// here the way a workbench field of the kind's own set already
			// is.
			bench.SetFieldValue(l.Bench.FM, target.key, value)
		} else {
			l.Bench.SetWorkbenchField(target.name, value)
		}
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
		if reloaded, err := l.Bench.LoadCardIn(filepath.Dir(entity.Dir), entity.ID); err == nil {
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

// writeAuthorityOf reports who may write a field of one entity. It is the
// kind's own rule for every kind but an attachment, which takes the authority
// of what it hangs on, so an attachment on a column or on the workbench is
// the operator's to write and one on a card or a comment is any owner's.
func (l *Library) writeAuthorityOf(entity *bench.EntityRef) string {
	if entity.Kind == bench.KindAttachment && l.definitionAttachmentWrite(entity) {
		return bench.AuthorityOperator
	}
	return bench.WriteAuthorityOf(entity.Kind)
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
func fieldEvent(req *Request, entity *bench.EntityRef, target fieldWrite, was, value string) bench.Event {
	ev := bench.Event{Actor: req.Acting(), Event: updatedEvents[entity.Kind], Field: target.name}
	if !target.prose {
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

// admitResolutionValue runs the resolution guard over a value being written
// directly to an item's resolution key, which is the one path to that key that
// does not come through resolve, verify or fail.
//
// It asks the two questions designationOf asks, in the same order and against
// the same store: the reference names a comment, and that comment hangs below
// this item. What it does not do is rewrite the value to the canonical
// spelling, because a field write stores what it admitted; a caller who wants
// the canonical reference settles the item with a verb.
func (l *Library) admitResolutionValue(req *Request, entity *bench.EntityRef, value string) *Response {
	found, err := l.Bench.ResolveEntity(designationSpelling(entity, value))
	if err != nil {
		return l.refuse(req, entity.Card, contract.NotADesignation, value)
	}
	if found.Kind != bench.KindComment {
		return l.refuse(req, entity.Card, contract.NotADesignation, value)
	}
	if !sameDir(filepath.Dir(filepath.Dir(found.Dir)), entity.Dir) {
		return l.refuse(req, entity.Card, contract.NotADesignation, value)
	}
	return nil
}

// designationSpelling is the reference the resolver is handed for a value
// naming a comment of one item, whichever of the two forms a caller typed.
//
// A position is already a whole reference and passes through. A bare
// identifier is not: it resolves to nothing on its own, and the item being
// written is the collection it is a member of, so it is composed under that
// item before the resolver sees it. That is what lets a caller name the
// comment the way the stored value spells it, which is the form dinah get
// answers with and the form a machine reader holds.
func designationSpelling(entity *bench.EntityRef, value string) string {
	if !bench.IsID(value) {
		return value
	}
	return entity.Ref + "/" + bench.CommentsDir + "/" + value
}

// admitCommentWrite decides whether a write of one comment's anchor may go
// ahead, and it asks a different question depending on what the caller was
// able to observe before it called.
//
// A caller naming no expected digest is one that has not been watching the
// comment, so the question is the plain one: does the digest on the header
// still describe the body standing beside it? A disagreement means something
// other than a verb wrote that body, and writing over it would recompute the
// digest from the tampered text and take the evidence with it, so the write is
// refused and the operator clears it deliberately, by restoring the body or by
// ratifying what is there with dinah accept-divergence.
//
// A caller naming an expected digest is one that had the comment open while
// its author typed, and for it the body comparison is not merely unhelpful but
// impossible: an editor writes the file on save, so by the time the caller can
// act the body it would compare against is gone. What survives the editor's
// write is the header, so the question becomes a compare-and-swap on the
// digest key. Still the value the caller last saw recorded, and nobody wrote
// this comment through a verb while the author was typing, so the change on
// disk is that author's own; the write takes the body as it stands. Moved, and
// somebody else's write landed in the middle of the session, so the author is
// about to overwrite work they never saw and the write is refused.
//
// This does not weaken the refusal. A hand edit made outside a session the
// caller opened is caught by the first question, because such a caller names
// no expected digest and has none to name.
func (l *Library) admitCommentWrite(req *Request, entity *bench.EntityRef, fm *bench.Frontmatter, body string) *Response {
	if refused := l.admitDesignatedCommentWrite(req, entity); refused != nil {
		return refused
	}
	if expected := strings.TrimSpace(req.ExpectedDigest); expected != "" {
		if fm.Value(bench.CommentDigestField) != expected {
			return l.refuse(req, entity.Card, contract.CommentBodyDiverged, entity.Ref)
		}
		return nil
	}
	if bench.CommentDiverged(fm, body) {
		return l.refuse(req, entity.Card, contract.CommentBodyDiverged, entity.Ref)
	}
	return nil
}

// admitDesignatedCommentWrite refuses a write to the body of a comment an item
// designates as its answer of record.
//
// Keying the answer on the comment's identifier fixes which comment the answer
// names and does nothing about what that comment says. A comment's fields
// belong to any owner and the divergence check asks only whether the body has
// been edited outside the tool, so anybody could rewrite the operator's
// designated answer in place, under his name, with his digest restamped. The
// designation went on naming the right comment and the comment said something
// else.
//
// There is no forced form. `dinah set` carries no force flag, and inventing
// one here would add a second way past a record this card exists to make
// durable. Correcting an answer means reopening the item, which clears the
// designation, and then settling it again.
//
// The refusal is the one a deletion of the same comment already raises, with
// the item in the same slot, because the two acts meet one rule: an answer of
// record cannot be changed or destroyed while it is still the answer.
func (l *Library) admitDesignatedCommentWrite(req *Request, entity *bench.EntityRef) *Response {
	if entity.Kind != bench.KindComment || entity.Card == nil {
		return nil
	}
	holder := filepath.Dir(filepath.Dir(entity.Dir))
	item, err := bench.LoadItem(holder)
	if err != nil || item.Resolution != entity.ID {
		return nil
	}
	named, err := l.itemCanonicalRef(entity.Card, item.ID)
	if err != nil {
		return l.FromError(req, err)
	}
	return l.refuseWith(req, entity.Card, contract.NotDesignatable, entity.Ref, map[string]string{
		"item": named,
	})
}

// restampsComment reports whether a write storing the value a comment already
// carries still has work to do, which is the case where the digest on the
// header does not describe that value.
//
// A write storing what is already there succeeds, writes nothing and journals
// nothing, and that rule holds for every other field. On a comment it has one
// exception, and the extension's save is entirely made of it: the editor puts
// the author's text on disk when they save, so by the time the verb reads the
// anchor the body is already the value being written and the comparison above
// finds nothing to do. The digest, meanwhile, still describes whatever the
// body was before the author typed. Taking the no-op there leaves a comment
// whose header disagrees with its own text, which dinah check reports as a
// hand edit on the tool's own most ordinary path.
//
// So the question the no-op asks on a comment is not "is the body already
// this" but "is the record already this", and the record is the body and the
// digest together.
func restampsComment(entity *bench.EntityRef, fm *bench.Frontmatter, value string) bool {
	if entity.Kind != bench.KindComment {
		return false
	}
	return fm.Value(bench.CommentDigestField) != bench.CommentDigest(value)
}
