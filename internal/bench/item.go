package bench

import (
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// The frontmatter keys a checklist item carries, per docs/design/format.md's
// "Checklist items" section. Each is written here and read by name rather than
// spelled at a call site, so the write side and the read side cannot drift.
const (
	// ItemKindField is the item's kind, one of ItemKinds.
	ItemKindField = "kind"
	// ItemStateField is the item's state, one of ItemStates.
	ItemStateField = "state"
	// ItemColumnField is the column an item names, written only when a
	// caller supplies one. Nothing reads it for enforcement today, so the
	// write side commits to no default for it.
	ItemColumnField = "column"
	// ItemOwnerField is who the item is meant for, recorded and never
	// enforced against the actor calling a terminal verb.
	ItemOwnerField = "owner"
	// ItemNoteField is the resolution note a terminal verb requires, kept
	// apart from the item's body, which is the text the item was filed with
	// and never changes.
	ItemNoteField = "note"
	// CitationsField is the sequence of citations an item carries.
	CitationsField = "citations"
)

// The four states a checklist item takes. The set is closed because method
// text travels between workbenches and a state has to mean one thing
// everywhere.
const (
	ItemPending  = "pending"
	ItemResolved = "resolved"
	ItemVerified = "verified"
	ItemFailed   = "failed"
)

// ItemKinds are the three kinds an item takes, in the order the format
// declares them. A surface offering a caller the choice reads this rather
// than writing the three out again.
var ItemKinds = []string{"acceptance_criterion", "open_question", "decision"}

// ItemStates are the four states an item takes, in the order the constants
// above declare them. A surface offering a caller the choice reads this for
// ItemKinds' own reason rather than writing the four out again.
var ItemStates = []string{ItemPending, ItemResolved, ItemVerified, ItemFailed}

// KnownItemKind reports whether a name is one of the three the format
// declares.
func KnownItemKind(kind string) bool {
	for _, known := range ItemKinds {
		if known == kind {
			return true
		}
	}
	return false
}

// Citation is one entry of an item's citations sequence: the scheme it draws
// on, the target that scheme names, and what the check showed before the work
// and after it where the scheme demands an observation.
type Citation struct {
	// Scheme is the evidence scheme the entry names, as the caller typed it.
	Scheme string
	// Target is what that scheme points at, as the caller typed it.
	Target string
	// Before and After are the observation, each either fail or pass, and
	// both empty where the entry records none.
	Before string
	After  string
}

// AddItem writes a checklist item under a card and returns it. The caller
// holds the card's lock, which is what makes the ordinal scan race-free, and
// the sequence mirrors AddComment's exactly.
//
// The column and the owner are written only when the caller supplies one.
// Absence is legal for both, on the terms severity and priority already
// follow. A column that is supplied arrives resolved to its identifier,
// because GatingItems matches on the identifier and a caller's own spelling
// would match nothing there; File is where that resolution happens, and this
// writer takes the value it is given.
func AddItem(cardDir, kind, column, owner, ts, text string) (*Item, error) {
	collection := filepath.Join(cardDir, ChecklistDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	ordinal := nextOrdinal(collection, ItemAnchor)
	dir := filepath.Join(collection, id)
	fm := NewFrontmatter()
	fm.Set(ItemKindField, kind)
	fm.Set(ItemStateField, ItemPending)
	if column != "" {
		fm.Set(ItemColumnField, column)
	}
	if owner != "" {
		fm.Set(ItemOwnerField, owner)
	}
	fm.Set("ts", ts)
	fm.Set(OrdinalField, strconv.Itoa(ordinal))
	if err := WriteText(filepath.Join(dir, ItemAnchor), fm.Render(text)); err != nil {
		return nil, err
	}
	return &Item{ID: id, Dir: dir, Kind: kind, State: ItemPending}, nil
}

// ReadItemAnchor opens an item's anchor for a write, returning its whole
// header and its body rather than the two fields LoadItem reads. A write has
// to put back every key it did not touch, which is what reading the header
// rather than the entity gives it.
func ReadItemAnchor(dir string) (*Frontmatter, string, error) {
	text, err := ReadText(filepath.Join(dir, ItemAnchor))
	if err != nil {
		return nil, "", contract.Refuse(contract.UnknownPath, dir)
	}
	fm, body := ParseAnchor(text)
	return fm, body, nil
}

// WriteItemAnchor rewrites an item's anchor from a header and a body.
func WriteItemAnchor(dir string, fm *Frontmatter, body string) error {
	return WriteText(filepath.Join(dir, ItemAnchor), fm.Render(body))
}

// CountCitations reports how many entries an item's citations sequence
// carries, which is the one fact the citation obligation turns on.
//
// The count reads the dashed entries standing at the shallowest indent of the
// block, through readChildren, which is the same reader the interchange
// applies to any nested value. A deeper line belongs to the entry above it
// and is not an entry of its own.
func CountCitations(fm *Frontmatter) int {
	lines := fm.Raw(CitationsField)
	if len(lines) < 2 {
		return 0
	}
	children := readChildren(lines[1:])
	if len(children) == 0 {
		return 0
	}
	shallowest := children[0].indent
	for _, child := range children {
		if child.indent < shallowest {
			shallowest = child.indent
		}
	}
	entries := 0
	for _, child := range children {
		if child.indent == shallowest && blockEntry.MatchString(child.text) {
			entries++
		}
	}
	return entries
}

// AppendCitation adds one entry to the end of an item's citations sequence,
// leaving every line already there exactly as it stands.
//
// The existing lines are carried rather than re-rendered from a parsed model,
// so an entry carrying a member this build does not know survives an append
// beside it. That is the posture the format asks a reader to take about a key
// it has no field for, applied to a writer.
func AppendCitation(fm *Frontmatter, citation Citation) {
	lines := fm.Raw(CitationsField)
	if len(lines) == 0 {
		lines = []string{CitationsField + ":"}
	}
	lines = append(lines,
		"  - "+ItemCitationScheme+": "+quote(citation.Scheme),
		"    "+ItemCitationTarget+": "+quote(citation.Target),
	)
	if citation.Before != "" || citation.After != "" {
		lines = append(lines,
			"    "+ItemCitationObserved+":",
			"      before: "+quote(citation.Before),
			"      after: "+quote(citation.After),
		)
	}
	fm.SetRaw(CitationsField, lines)
}

// The three member names one citation entry carries.
const (
	ItemCitationScheme   = "scheme"
	ItemCitationTarget   = "target"
	ItemCitationObserved = "observed"
)

// The two values an observation records on either side of the work. The pair
// is closed, so a caller naming anything else is refused rather than having
// its word written down.
const (
	ObservedFail = "fail"
	ObservedPass = "pass"
)

// ParseObserved reads the before:after pair a citation may carry, and reports
// whether it read as the closed pair at all. An empty text is no observation
// and reads as two empty halves with a true report, which is what lets a
// caller ask one question of a flag that may be absent.
func ParseObserved(text string) (string, string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", "", true
	}
	before, after, split := strings.Cut(trimmed, ":")
	if !split {
		return "", "", false
	}
	if !knownObservation(before) || !knownObservation(after) {
		return "", "", false
	}
	return before, after, true
}

// knownObservation reports whether one half of an observation is one of the
// two values the format fixes.
func knownObservation(half string) bool {
	return half == ObservedFail || half == ObservedPass
}
