package bench

import "sort"

// Field is one field of one kind: the name a reader types, where the value is
// stored, whether it may be cleared, and which guard a write to it runs.
type Field struct {
	// Name is what a reader types after the reference.
	Name string
	// Key is the frontmatter key the value is stored under, where that
	// differs from the name a reader types. It is empty on every field
	// stored under its own name, which is all but two of them, and Stored
	// resolves the two so no caller has to remember which.
	//
	// Two column fields differ today. The format stores a column's capacity
	// as wip_limit, which is the profile's own word for the limit, and a
	// reader types capacity, which is what `dinah column new` already calls
	// the flag that writes it. The format stores a column's hold as
	// gate_items, which is the profile's own word for the declaration, and a
	// reader types hold, which is the word a person reaches for when a step
	// is to wait until its questions are answered.
	Key string
	// Prose is true where the value is the anchor's body rather than a
	// frontmatter key. A prose field holds several lines; every other field
	// holds one.
	Prose bool
	// Clearable is true where a write with no value clears the field. A field
	// the entity may not be without declares false and refuses the clear.
	Clearable bool
	// Guard names the rule a write to this field runs beyond the kind's own
	// authority check, and is empty on a field whose write is a plain
	// rewrite. Every non-empty value is one of the Guard constants below.
	Guard string
}

// The guards a field write may declare. The set is closed, and
// TestEveryDeclaredGuardIsRouted holds it to the routing in internal/verb.
const (
	GuardSlug     = "slug"
	GuardLevel    = "level"
	GuardTier     = "tier"
	GuardState    = "state"
	GuardFilename = "filename"
	GuardKind     = "column-kind"
	GuardCapacity = "capacity"
	GuardHold     = "hold"
	// GuardColumnRef admits any spelling ColumnByRef resolves, which is a
	// column's identifier, its slug or its title, and the router resolves
	// the admitted value to that column's identifier before it is stored.
	// A field carrying this guard therefore holds an identifier or nothing,
	// which is what a gate reading it compares against.
	GuardColumnRef = "column-ref"
)

// Guards lists the closed set of guard names a field may declare, in the order
// the declaration above states them. A sweep asking whether every guard is
// routed reads this rather than writing the nine out again.
var Guards = []string{
	GuardSlug, GuardLevel, GuardTier, GuardState,
	GuardFilename, GuardKind, GuardCapacity, GuardHold,
	GuardColumnRef,
}

// The two write authorities a kind declares. The set is closed at two, and it
// is declared per kind rather than branched on inside the writer, so a kind
// that slipped in with no rule about who may write it fails a test instead of
// defaulting to whatever the router happens to do.
const (
	// AuthorityOwner is the rule the four card-level kinds keep: the request
	// names an owner and any owner will do, because a classification is not
	// a claim.
	AuthorityOwner = "owner"
	// AuthorityOperator is the rule the three workbench-level kinds keep:
	// the actor is the workbench's own operator or the write refuses.
	AuthorityOperator = "operator"
)

// The frontmatter keys an entity's own name and prose body are addressed by.
// Each is written here and read by name, so a write site and a read site
// cannot drift.
const (
	// TitleField is the title a person gave the entity.
	TitleField = "title"
	// OperatorField is the workbench's operator.
	OperatorField = "operator"
	// ColumnKindField is a column's kind. It is spelled apart from
	// ItemKindField because a column's kind is settable and an item's is not.
	ColumnKindField = "kind"
	// CapacityField is a column's declared limit, as a reader types it.
	CapacityField = "capacity"
	// WIPLimitKey is the frontmatter key that limit is stored under, which
	// is not the name a reader types for it.
	WIPLimitKey = "wip_limit"
	// HoldField is a column's hold, as a reader types it. It carries a
	// direction as well as a state: on where the column holds a card
	// entering it until an item naming the column is settled, out where it
	// holds a card leaving it on the same terms, both where it holds a card
	// either way, and off where it holds neither way.
	HoldField = "hold"
	// GateItemsKey is the frontmatter key that hold is stored under, which
	// is not the name a reader types for it. The stored spelling is the
	// profile's, under CORE-JSON-10, and it stays out of everything a person
	// types or reads.
	GateItemsKey = "gate_items"
	// HoldOn, HoldOff, HoldOut and HoldBoth are the four values a reader
	// types for a hold, and the set is closed. On is the sole spelling for
	// the entry direction, carried unchanged from the two-value vocabulary
	// this field was born with, so there is deliberately no fifth word in:
	// two spellings for one direction is the confusion this vocabulary is
	// shaped to avoid.
	HoldOn   = "on"
	HoldOff  = "off"
	HoldOut  = "out"
	HoldBoth = "both"
	// StatusField is a workstream's status.
	StatusField = "status"
	// FilenameField is an attachment's filename.
	FilenameField = "filename"
	// DescriptionField is an attachment's description.
	DescriptionField = "description"
	// InstructionsField is the name a workbench's and a column's prose body
	// is typed as. The body is not a frontmatter key, so this name addresses
	// it and never appears in a header.
	InstructionsField = "instructions"
	// BodyField is the name a card's and a comment's prose body is typed as.
	BodyField = "body"
	// TextField is the name an item's prose body is typed as.
	TextField = "text"
	// NotesField is the name a workstream's prose body is typed as.
	NotesField = "notes"
)

// HoldValues are the four values a column's hold takes, in the order off, on,
// out, both. It is the one statement of that set: the guard that admits a
// typed value reads it through KnownHold below rather than writing the four
// out again, and so does every surface offering a reader the choice. A fifth
// value added later moves this list and the documentation a test holds
// against it, and nothing else.
var HoldValues = []string{HoldOff, HoldOn, HoldOut, HoldBoth}

// KnownHold reports whether a value is one of the four HoldValues declares.
// It mirrors KnownItemKind, which answers the same question for an item's
// kind, so a closed vocabulary is read the same way wherever one is read.
func KnownHold(value string) bool {
	for _, known := range HoldValues {
		if known == value {
			return true
		}
	}
	return false
}

// fields is the one statement of what a kind's fields are. Every reader goes
// through FieldsOf, FieldOf, AllFields or WriteAuthorityOf rather than
// repeating any part of it, so a field added or dropped later moves this table
// and the round-trip sample beside it and nothing else.
//
// The order within a kind is the order a listing prints them, which is the
// order a reader meets them: the entity's name first, then what it is stored
// under, then the prose.
var fields = map[string][]Field{
	KindWorkbench: {
		{Name: TitleField},
		{Name: SlugField, Guard: GuardSlug},
		{Name: OperatorField},
		{Name: InstructionsField, Prose: true, Clearable: true},
	},
	KindColumn: {
		{Name: TitleField},
		{Name: SlugField, Guard: GuardSlug},
		{Name: ColumnKindField, Guard: GuardKind},
		{Name: TierField, Clearable: true, Guard: GuardLevel},
		{Name: CapacityField, Key: WIPLimitKey, Clearable: true, Guard: GuardCapacity},
		{Name: HoldField, Key: GateItemsKey, Guard: GuardHold},
		{Name: InstructionsField, Prose: true, Clearable: true},
	},
	KindCard: {
		{Name: TitleField},
		{Name: BodyField, Prose: true, Clearable: true},
		{Name: SeverityField, Clearable: true, Guard: GuardLevel},
		{Name: PriorityField, Clearable: true, Guard: GuardLevel},
		{Name: TierField, Clearable: true, Guard: GuardTier},
	},
	KindComment: {
		{Name: BodyField, Prose: true},
	},
	KindItem: {
		{Name: TextField, Prose: true},
		{Name: ItemStateField, Guard: GuardState},
		{Name: ItemNoteField, Clearable: true},
		{Name: ItemOwnerField, Clearable: true},
		{Name: ItemColumnField, Clearable: true, Guard: GuardColumnRef},
	},
	KindAttachment: {
		{Name: FilenameField, Guard: GuardFilename},
		{Name: DescriptionField, Clearable: true},
	},
	KindWorkstream: {
		{Name: TitleField},
		{Name: SlugField, Guard: GuardSlug},
		{Name: StatusField},
		{Name: NotesField, Prose: true, Clearable: true},
	},
}

// writeAuthority declares who may write a field of each kind. The three
// workbench-level kinds are the operator's, because a column is a station of
// the flow and renaming one or changing its kind puts the flow in the hands of
// whoever happens to hold a card. The four card-level kinds are any owner's,
// which is the rule a card's own severity write already keeps.
var writeAuthority = map[string]string{
	KindWorkbench:  AuthorityOperator,
	KindColumn:     AuthorityOperator,
	KindWorkstream: AuthorityOperator,
	KindCard:       AuthorityOwner,
	KindComment:    AuthorityOwner,
	KindItem:       AuthorityOwner,
	KindAttachment: AuthorityOwner,
}

// FieldsOf reports the fields of a kind a person wrote and may rewrite, in the
// order a listing prints them. A kind the grammar does not name reports no
// fields.
func FieldsOf(kind string) []string {
	declared := fields[kind]
	names := make([]string, 0, len(declared))
	for _, field := range declared {
		names = append(names, field.Name)
	}
	return names
}

// Stored reports the frontmatter key this field's value is written under,
// which is the field's own name wherever the two agree.
func (f Field) Stored() string {
	if f.Key != "" {
		return f.Key
	}
	return f.Name
}

// FieldOf reports one field's declaration, and whether the kind carries a
// field of that name at all.
func FieldOf(kind, name string) (Field, bool) {
	for _, field := range fields[kind] {
		if field.Name == name {
			return field, true
		}
	}
	return Field{}, false
}

// EntityKinds lists every kind the containment table names, plus the
// workstream, sorted. It is what a sweep over the kinds iterates.
//
// The workstream is added rather than read off the table for the reason
// containment.go gives: a workstream is a membership rather than a container,
// so it is deliberately absent from the grammar's own table while still being
// an entity a reference names and a field write reaches.
func EntityKinds() []string {
	seen := map[string]bool{KindWorkbench: true, KindWorkstream: true}
	for kind, mounts := range containment {
		seen[kind] = true
		for _, mount := range mounts {
			seen[mount.Kind] = true
		}
	}
	kinds := make([]string, 0, len(seen))
	for kind := range seen {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// AllFields is the sorted union of every kind's field names, which is the
// closed set the set command's field argument declares.
//
// It is a union rather than one kind's set because a tool schema is fixed
// before a reference is known. Every value the argument accepts is in it, and
// which of them the resolved kind actually carries is what the refusal
// completes.
func AllFields() []string {
	seen := map[string]bool{}
	for kind := range fields {
		for _, field := range fields[kind] {
			seen[field.Name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// WriteAuthorityOf reports who may write a field of this kind: AuthorityOwner,
// where any owner may, or AuthorityOperator, where the actor must be the
// workbench's operator. A kind the grammar does not name reports the empty
// string, which is what lets a sweep tell a kind with no rule from a kind
// whose rule is the looser of the two.
func WriteAuthorityOf(kind string) string {
	return writeAuthority[kind]
}

// AnchorOf reports the anchor filename a kind's entity carries, and the empty
// string for a kind the grammar does not name.
//
// The six mounted kinds are read off the containment table rather than listed
// again here, so the anchor of a kind stays written down once. The workbench
// and the workstream are named directly, because neither is mounted: the
// workbench is the root nothing contains, and a workstream is a membership
// rather than a container.
func AnchorOf(kind string) string {
	switch kind {
	case KindWorkbench:
		return WorkbenchAnchor
	case KindWorkstream:
		return WorkstreamAnchor
	}
	for _, mounts := range containment {
		for _, mount := range mounts {
			if mount.Kind == kind {
				return mount.Anchor
			}
		}
	}
	return ""
}
