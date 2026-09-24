package bench

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Condition is one applies_when member as the reader parsed it: the declared
// field that gates the declaration it sits on, and the values of that field
// which admit it. A declaration carrying a usable condition applies to a card
// only where the card's stored value for the gate equals one of the values.
type Condition struct {
	// Field is the gate, a declared field key.
	Field string
	// Is lists the values that admit the declaration, in declared order.
	Is []string
}

// Applicability is one slot's answer for one card, where a slot is a level
// axis or a declared field the card may carry a value for.
type Applicability struct {
	// Applies is true where the slot applies to the card.
	Applies bool
	// Gate is the gate's key, and empty where the slot carries no usable
	// condition.
	Gate string
	// GateValue is the gate's stored value on the card, and empty where the
	// card stores none.
	GateValue string
}

// SlotValue is one slot and the value a card stores under it, which is how an
// orphaned value is reported: a value kept on a slot that no longer applies to
// the card carrying it.
type SlotValue struct {
	// Slot is the level axis or the declared field key.
	Slot string
	// Value is what the card stores there.
	Value string
}

// MalformedCondition is one applies_when member the reader could not use,
// named by the slot it sits on and the reason it was refused. The condition
// is ignored, so the slot applies to every card, and dinah check is where a
// person meets the refusal.
type MalformedCondition struct {
	// Slot is the level axis or the declared field key carrying the member.
	Slot string
	// Reason is one of the six reason tokens below.
	Reason string
}

// UnmatchableValue is one value of a usable condition's is list that the
// gate's declared type can never hold, so no card can ever be admitted by it.
// The rest of the condition stands.
type UnmatchableValue struct {
	// Slot is the level axis or the declared field key carrying the member.
	Slot string
	// Value is the value as written.
	Value string
}

// The members a condition is written under. applies_when sits beside a
// declared field entry's type, meaning and on, and beside a mapping-form level
// axis's values; field and is are its two members.
const (
	appliesWhenMember    = "applies_when"
	conditionFieldMember = "field"
	conditionIsMember    = "is"
	levelValuesMember    = "values"
)

// The six reasons a condition cannot be used. Each is a machine token that
// travels in the finding's detail, so the rendered sentence reads the reason
// in parentheses after the slot.
const (
	// ConditionUnreadable is a member absent, an is member that is not a
	// flow sequence, an is member naming no value, or an applies_when member
	// carrying anything other than one block mapping.
	ConditionUnreadable = "unreadable"
	// ConditionOnTier is a condition on the tier axis, which drives the
	// claim gate and is not this mechanism's to narrow.
	ConditionOnTier = "tier"
	// ConditionReachesBeyondCards is a conditioned field entry that reaches
	// a kind other than card, which is any entry not saying on: [card].
	ConditionReachesBeyondCards = "reaches-beyond-cards"
	// ConditionGateUndeclared is a field member naming no key the workbench
	// declares.
	ConditionGateUndeclared = "gate-undeclared"
	// ConditionGateOffCards is a field member naming a declared key that
	// does not reach cards.
	ConditionGateOffCards = "gate-off-cards"
	// ConditionGateConditioned is a field member naming a key that itself
	// carries a condition, which includes a slot naming itself.
	ConditionGateConditioned = "gate-conditioned"
)

// conditionableAxes are the level axes a condition may sit on, in the order
// every listing of inapplicable slots reports them. Tier is left out on
// purpose: ConditionOnTier is what a condition written there draws.
var conditionableAxes = []string{SeverityField, PriorityField}

// rawCondition is one applies_when member as a block reader met it, before
// Open has decided whether it can be used. The readers record what they saw
// and nothing more, because a level condition's gate is a declared field and
// the two blocks are read one after the other.
type rawCondition struct {
	// field is the value of the field member, or empty where none was met.
	field string
	// is is the values the is member listed, and metIs says whether the
	// member was met at all, so an absent member and an empty one are told
	// apart for nothing: both read as unreadable.
	is    []string
	metIs bool
	// unreadable is set where the member's own shape was wrong: a value on
	// the applies_when line itself, an is member that was not a flow
	// sequence, or an is member written in the dashed form.
	unreadable bool
}

// read takes one member line beneath an open applies_when member.
func (c *rawCondition) read(name, value string) {
	switch name {
	case conditionFieldMember:
		c.field = value
	case conditionIsMember:
		c.metIs = true
		if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
			c.unreadable = true
			return
		}
		c.is = flowMembers(value)
	}
}

// resolveConditions turns the raw conditions the two block readers met into
// the usable ones the model holds, and records every refused one for dinah
// check. It runs once at Open, after both blocks are read, because whether a
// level axis's gate is declared is a question only the fields block answers.
//
// The order the slots are resolved in is the order every report lists them:
// the level axes first, then the declared fields in declaration order.
func (b *Bench) resolveConditions(levels map[string]*rawCondition, fields map[string]*rawCondition) {
	b.levelConditions = map[string]*Condition{}
	for _, axis := range LevelAxes {
		raw, carried := levels[axis]
		if !carried {
			continue
		}
		if reason := b.conditionReason(axis, raw, true, nil, fields); reason != "" {
			b.malformedConditions = append(b.malformedConditions, MalformedCondition{Slot: axis, Reason: reason})
			continue
		}
		b.levelConditions[axis] = b.admitCondition(axis, raw)
	}
	for at := range b.declaredFields {
		entry := &b.declaredFields[at]
		raw, carried := fields[entry.Key]
		if !carried {
			continue
		}
		if reason := b.conditionReason(entry.Key, raw, false, entry, fields); reason != "" {
			b.malformedConditions = append(b.malformedConditions, MalformedCondition{Slot: entry.Key, Reason: reason})
			continue
		}
		entry.AppliesWhen = b.admitCondition(entry.Key, raw)
	}
}

// conditionReason answers why one raw condition cannot be used, or the empty
// string where it can. The reasons are tried in the order the table in
// docs/design/format.md states them, so a condition wrong in two ways is
// reported for the earlier one.
func (b *Bench) conditionReason(slot string, raw *rawCondition, isLevel bool, entry *DeclaredField, fields map[string]*rawCondition) string {
	if raw.unreadable || raw.field == "" || !raw.metIs || len(raw.is) == 0 {
		return ConditionUnreadable
	}
	if isLevel && slot == TierField {
		return ConditionOnTier
	}
	if entry != nil && !reachesCardsAlone(*entry) {
		return ConditionReachesBeyondCards
	}
	gate := b.DeclaredFieldOf(raw.field)
	if gate == nil {
		return ConditionGateUndeclared
	}
	if !gate.Declares(KindCard) {
		return ConditionGateOffCards
	}
	if _, conditioned := fields[raw.field]; conditioned {
		return ConditionGateConditioned
	}
	return ""
}

// reachesCardsAlone reports whether a field entry says on: [card] and nothing
// wider, which is the one reach a conditioned entry may have.
func reachesCardsAlone(entry DeclaredField) bool {
	if entry.EveryKind || len(entry.On) == 0 {
		return false
	}
	for _, kind := range entry.On {
		if kind != KindCard {
			return false
		}
	}
	return true
}

// admitCondition builds the usable condition and records each is value the
// gate's type can never hold. The gate is known to be declared by the time
// this runs, since conditionReason answered nothing.
func (b *Bench) admitCondition(slot string, raw *rawCondition) *Condition {
	gate := b.DeclaredFieldOf(raw.field)
	for _, value := range raw.is {
		if !AdmitsFieldValue(gate.Type, value) {
			b.unmatchableValues = append(b.unmatchableValues, UnmatchableValue{Slot: slot, Value: value})
		}
	}
	return &Condition{Field: raw.field, Is: append([]string(nil), raw.is...)}
}

// ConditionOn reports the usable condition one slot carries, and nil where it
// carries none or where the one it carries could not be used.
func (b *Bench) ConditionOn(slot string) *Condition {
	if KnownLevelAxis(slot) {
		return b.levelConditions[slot]
	}
	if field := b.DeclaredFieldOf(slot); field != nil {
		return field.AppliesWhen
	}
	return nil
}

// MalformedConditions are the applies_when members the reader refused, in the
// order the slots are declared, which dinah check reports and nothing else
// reads.
func (b *Bench) MalformedConditions() []MalformedCondition {
	return append([]MalformedCondition(nil), b.malformedConditions...)
}

// UnmatchableConditionValues are the is values no card can ever store, in the
// order the slots are declared, which dinah check reports and nothing else
// reads.
func (b *Bench) UnmatchableConditionValues() []UnmatchableValue {
	return append([]UnmatchableValue(nil), b.unmatchableValues...)
}

// UsesAppliesWhen reports whether the definition carries any applies_when
// member or any level axis in mapping form, usable or not. It is what the
// format check reads: a workbench declaring a format below AppliesWhenFormat
// and carrying either is one an older build misreads.
func (b *Bench) UsesAppliesWhen() bool {
	return b.usesAppliesWhen
}

// Applicability answers for one card and one slot, where slot is a level axis
// name or a declared field key.
//
// A slot with no usable condition applies. Otherwise the gate's stored value
// on the card decides: no value makes the slot inapplicable, and a value makes
// it applicable exactly where it equals one of the condition's values, byte
// for byte. Nothing about the answer is stored, so it is computed on every
// read from the declaration and the card as they stand. A nil card, which is
// a card being filed, stores nothing.
func (b *Bench) Applicability(card *Card, slot string) Applicability {
	condition := b.ConditionOn(slot)
	if condition == nil {
		return Applicability{Applies: true}
	}
	answer := Applicability{Gate: condition.Field}
	if card != nil && card.FM != nil {
		answer.GateValue = FieldValue(card.FM, condition.Field)
	}
	if answer.GateValue == "" {
		return answer
	}
	for _, admitted := range condition.Is {
		if admitted == answer.GateValue {
			answer.Applies = true
			return answer
		}
	}
	return answer
}

// conditionedSlots are the slots that may be inapplicable to a card, in the
// one order every report lists them: severity, then priority, then declared
// field keys in declaration order. Only a slot carrying a usable condition is
// listed, since no other can be inapplicable.
func (b *Bench) conditionedSlots() []string {
	var slots []string
	for _, axis := range conditionableAxes {
		if b.levelConditions[axis] != nil {
			slots = append(slots, axis)
		}
	}
	for _, field := range b.declaredFields {
		if field.AppliesWhen != nil {
			slots = append(slots, field.Key)
		}
	}
	return slots
}

// InapplicableSlots lists the slots inapplicable to a card: severity, then
// priority, then declared field keys in declaration order.
func (b *Bench) InapplicableSlots(card *Card) []string {
	var inapplicable []string
	for _, slot := range b.conditionedSlots() {
		if !b.Applicability(card, slot).Applies {
			inapplicable = append(inapplicable, slot)
		}
	}
	return inapplicable
}

// OrphanedValues lists the values a card keeps on slots inapplicable to it, in
// the order InapplicableSlots reports the slots. An orphaned value is kept
// rather than cleared, and this is how every reader that reports it finds it.
func (b *Bench) OrphanedValues(card *Card) []SlotValue {
	var orphaned []SlotValue
	for _, slot := range b.InapplicableSlots(card) {
		stored := b.slotValue(card, slot)
		if stored == "" {
			continue
		}
		orphaned = append(orphaned, SlotValue{Slot: slot, Value: stored})
	}
	return orphaned
}

// slotValue reports what a card stores under one slot, reading a level off the
// card and a declared field off its value block.
func (b *Bench) slotValue(card *Card, slot string) string {
	if card == nil {
		return ""
	}
	if KnownLevelAxis(slot) {
		return card.LevelOf(slot)
	}
	if card.FM == nil {
		return ""
	}
	return FieldValue(card.FM, slot)
}

// checkConditions reports every condition the reader refused, every is value
// no card can store, and a workbench carrying a condition while declaring a
// format an older build would misread it under.
func (b *Bench) checkConditions() []Finding {
	anchor := filepath.Join(b.Root, WorkbenchAnchor)
	var findings []Finding
	for _, malformed := range b.malformedConditions {
		findings = append(findings, Finding{
			Path:   anchor,
			Key:    FindingAppliesWhenMalformed,
			Detail: malformed.Slot + " (" + malformed.Reason + ")",
		})
	}
	for _, unmatchable := range b.unmatchableValues {
		findings = append(findings, Finding{
			Path:   anchor,
			Key:    FindingAppliesWhenValueUnmatchable,
			Detail: unmatchable.Slot + " (" + unmatchable.Value + ")",
		})
	}
	if b.usesAppliesWhen && b.Format < AppliesWhenFormat {
		findings = append(findings, Finding{
			Path:   anchor,
			Key:    FindingAppliesWhenBelowFormat,
			Detail: strconv.Itoa(b.Format),
		})
	}
	return findings
}

// Notices reports what dinah check prints beside its findings without counting
// it as one: a report with no repair, which never changes the outcome or the
// exit code. Each notice is a Finding carrying SeverityCleanup, so SeverityOf
// answers truly for a caller that reads one.
//
// One notice exists. A column whose require_fields names a key carrying a
// usable condition refuses entry to a card the condition excludes, since such
// a card can never store the value, and only the operator's override lets it
// in. The operator ruled the configuration legitimate, so the report has
// nothing to repair and is a notice rather than a finding.
func (b *Bench) Notices() []Finding {
	var notices []Finding
	for _, column := range b.Columns {
		for _, key := range column.RequireFields {
			field := b.DeclaredFieldOf(key)
			if field == nil || field.AppliesWhen == nil {
				continue
			}
			notices = append(notices, Finding{
				Path:     filepath.Join(b.Root, ColumnsDir, column.ID, ColumnAnchor),
				Key:      NoticeRequiredFieldConditioned,
				Detail:   column.Ref() + " (" + key + ")",
				Severity: SeverityCleanup,
			})
		}
	}
	return notices
}

// AppliesWhenMigration is the account dinah check --migrate-applies-when
// answers with.
type AppliesWhenMigration struct {
	// From is the format the workbench declared before the run.
	From int `json:"from"`
	// Stamped is true where the anchor was written with AppliesWhenFormat.
	Stamped bool `json:"stamped"`
	// Preview is true where the run carried no confirmation, so it wrote
	// nothing whatever the format was.
	Preview bool `json:"preview,omitempty"`
}

// MigrateAppliesWhen stamps AppliesWhenFormat on a workbench declaring a lower
// format, and does nothing to one already at or above it. Without apply it
// classifies and writes nothing, which is why it needs no rehearsal flag. The
// stamp is one write to the workbench anchor, through the writer the other
// migrations stamp their format with, and no card is read or written.
//
// Any lower number is stamped, a number below DesignationFormat included. A
// workbench that has never had its designations converted therefore comes
// out declaring a format that says they have been, and no card is read to
// find out. Run `dinah check --migrate-designations` first on such a store;
// this stamp does not do that conversion and does not refuse for want of it.
func (b *Bench) MigrateAppliesWhen(apply bool) (*AppliesWhenMigration, error) {
	report := &AppliesWhenMigration{From: b.Format, Preview: !apply}
	if !apply || b.Format >= AppliesWhenFormat {
		return report, nil
	}
	if err := b.stampFormat(AppliesWhenFormat); err != nil {
		return nil, err
	}
	report.Stamped = true
	return report, nil
}
