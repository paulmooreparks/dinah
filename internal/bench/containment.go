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
	// KindComment is one comment below a card.
	KindComment = "comment"
	// KindItem is one checklist item below a card.
	KindItem = "item"
	// KindAttachment is one attachment, which any of the four kinds above
	// may carry.
	KindAttachment = "attachment"
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
}

// containment is the one statement of what contains what. Every reader of the
// grammar goes through Contains rather than repeating any part of this, so an
// extension kind reaching the tree cannot stay invisible to archiving, to the
// ordinal migration, or to the entity resolver.
//
// A kind mounting nothing is listed with no mounts rather than left out, so a
// caller can tell a leaf of the grammar from a kind the grammar does not have.
var containment = map[string][]Mount{
	KindWorkbench: {
		{Dir: ColumnsDir, Kind: KindColumn, Anchor: ColumnAnchor},
		{Dir: CardsDir, Kind: KindCard, Anchor: CardAnchor},
		{Dir: AttachmentsDir, Kind: KindAttachment, Anchor: AttachmentAnchor, NameField: "filename"},
	},
	KindColumn: {
		{Dir: AttachmentsDir, Kind: KindAttachment, Anchor: AttachmentAnchor, NameField: "filename"},
	},
	KindCard: {
		{Dir: CommentsDir, Kind: KindComment, Anchor: CommentAnchor},
		{Dir: ChecklistDir, Kind: KindItem, Anchor: ItemAnchor},
		{Dir: AttachmentsDir, Kind: KindAttachment, Anchor: AttachmentAnchor, NameField: "filename"},
	},
	KindComment: {
		{Dir: AttachmentsDir, Kind: KindAttachment, Anchor: AttachmentAnchor, NameField: "filename"},
	},
	KindItem:       {},
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
