package bench

// The entity kind names of the containment grammar. Every value is a token a
// reader meets on the machine surface, so the spellings here are the ones the
// tree projections and the entity resolver both report.
const (
	// KindWorkbench is the workbench itself, the one root that is not
	// contained by anything.
	KindWorkbench = "workbench"
	// KindColumn is one station of the flow.
	KindColumn = "column"
	// KindCard is one card.
	KindCard = "card"
	// KindComment is one comment. It hangs below a card, below one of that
	// card's checklist items, or below a column.
	KindComment = "comment"
	// KindItem is one checklist item below a card.
	KindItem = "item"
	// KindAttachment is one attachment, which five kinds may carry: the
	// four above it here, and the workstream declared below.
	KindAttachment = "attachment"
	// KindWorkstream is one workstream. It is a key of the containment table
	// below, mounting the attachments collection docs/design/format.md gives
	// it, and it is deliberately absent from the workbench's own mount list,
	// because a workstream is a membership rather than a container of cards:
	// cards join and leave one, and a card is not contained by one.
	KindWorkstream = "workstream"
)

// Mount is one collection a kind contains: the directory name it hangs from
// the parent under, the kind of entity it holds, and that kind's anchor.
type Mount struct {
	// Dir is the collection directory's name, relative to the parent
	// entity's own directory.
	Dir string
	// Kind is the entity kind the collection holds.
	Kind string
	// Anchor is the anchor filename each member of the collection carries.
	Anchor string
	// NameField is the anchor field a name selector reads, when the
	// collection declares one. Empty means the collection declares no name
	// and a name selector against it refuses unknown-path, the same way it
	// does against one this table does not list.
	NameField string
	// Stamped says whether this collection's members carry an ordinal, which
	// the writer sets at creation and a positional reference resolves through.
	// A collection of unstamped members is not swept for the ordinal
	// invariants, because every member of it would be reported as missing one.
	Stamped bool
}

// containment is the one statement of what contains what. Every reader of the
// grammar goes through Contains rather than repeating any part of this, so an
// extension kind reaching the tree cannot stay invisible to archiving, to the
// ordinal migration, or to the entity resolver.
//
// A kind mounting nothing is listed with no mounts rather than left out, so a
// caller can tell a leaf of the grammar from a kind the grammar does not have.
//
// The table is acyclic, and no kind reaches itself through any chain of
// mounts. Five recursive readers of Contains rest on that property for their
// termination argument, carrying neither a depth bound nor a visited set:
// containedCount and containedChildren in internal/verb/tree.go,
// ordinalCollections in ordinal.go, collectionsBelow in finish.go, and
// mountlessAttachmentsBelow in check.go. Building the folder kind that
// docs/design/format.md declares and defers retires the property, because a
// folder contains folders, and it obliges each of those five walks to carry an
// explicit bound. Adding any other self-reaching mount does the same.
var containment = map[string][]Mount{
	// The workbench mounts no comments collection, and that is a ruling the
	// operator gave on 2026-09-15 rather than anything the grammar requires.
	// Nothing above forbids it: a comments collection here reaches no kind
	// that reaches the workbench, so the acyclicity argument survives it
	// untouched, and the entry costs one line. He judged that a note about
	// the whole workbench has no reader, where a note about a column has an
	// obvious one. Reversing it is that line and the refusal's own list of
	// what is commentable.
	KindWorkbench: {
		{Dir: ColumnsDir, Kind: KindColumn, Anchor: ColumnAnchor},
		{Dir: CardsDir, Kind: KindCard, Anchor: CardAnchor},
		{Dir: AttachmentsDir, Kind: KindAttachment, Anchor: AttachmentAnchor, NameField: "filename", Stamped: true},
	},
	KindColumn: {
		{Dir: CommentsDir, Kind: KindComment, Anchor: CommentAnchor, Stamped: true},
		{Dir: AttachmentsDir, Kind: KindAttachment, Anchor: AttachmentAnchor, NameField: "filename", Stamped: true},
	},
	KindCard: {
		{Dir: CommentsDir, Kind: KindComment, Anchor: CommentAnchor, Stamped: true},
		{Dir: ChecklistDir, Kind: KindItem, Anchor: ItemAnchor, Stamped: true},
		{Dir: AttachmentsDir, Kind: KindAttachment, Anchor: AttachmentAnchor, NameField: "filename", Stamped: true},
	},
	KindComment: {
		{Dir: AttachmentsDir, Kind: KindAttachment, Anchor: AttachmentAnchor, NameField: "filename", Stamped: true},
	},
	KindItem: {
		{Dir: CommentsDir, Kind: KindComment, Anchor: CommentAnchor, Stamped: true},
	},
	// A workstream mounts attachments and nothing else. It is absent from the
	// workbench's own mount list above, which is what the comment on
	// KindWorkstream means and what EntityKinds and AnchorOf work around: no
	// walk rooted at the workbench reaches a workstream, because a card is not
	// contained by one. What a workstream does contain is the attachments
	// collection docs/design/format.md has always given it, and stating that
	// here is what puts the collection in front of every reader of the
	// grammar at once.
	KindWorkstream: {
		{Dir: AttachmentsDir, Kind: KindAttachment, Anchor: AttachmentAnchor, NameField: "filename", Stamped: true},
	},
	KindAttachment: {},
}

// Contains reports the collections a kind mounts, in the order a walk draws
// them. It is the one definition of the containment grammar, and every reader
// of that grammar goes through it.
//
// A kind the grammar does not name reports no mounts, which is what lets a
// walk treat an unrecognised kind as a leaf rather than refusing over it.
func Contains(kind string) []Mount {
	return containment[kind]
}

// MountOf reports the collection a kind mounts under a directory name, and
// whether it mounts one at all. It is what a reference resolver asks when a
// path segment names a collection and the resolver needs the anchor and the
// kind behind it.
func MountOf(kind, dir string) (Mount, bool) {
	for _, mount := range Contains(kind) {
		if mount.Dir == dir {
			return mount, true
		}
	}
	return Mount{}, false
}

// KindOfAnchor reports the entity kind an anchor filename belongs to, and
// whether the grammar names one. The answer is derived from the containment
// table rather than declared beside it, so the anchor of a kind is written
// down once.
//
// The card's own anchor is reachable through the cards collection the
// workbench mounts, so no caller needs a second statement of it.
func KindOfAnchor(anchor string) (string, bool) {
	for _, mounts := range containment {
		for _, mount := range mounts {
			if mount.Anchor == anchor {
				return mount.Kind, true
			}
		}
	}
	return "", false
}
