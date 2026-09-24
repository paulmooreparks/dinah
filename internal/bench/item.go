package bench

import (
	"encoding/json"
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
	// caller supplies one. GatingItems matches a move's destination against
	// it, and since dinah-484 a move's departure too, so the value is the
	// column's identifier rather than whatever the caller spelled, and an
	// item carrying no column holds nothing.
	ItemColumnField = "column"
	// ItemOwnerField is who the item is meant for. An item recording
	// ItemOwnerOperator is the operator's to settle: closeItem in
	// internal/verb refuses a terminal verb on one to anybody else, and
	// SetField beside it refuses a rewrite of this key on one, so the
	// record cannot be edited out from under the refusal. Every other
	// value is recorded and enforced against nobody, because only the
	// operator is a person the workbench can name.
	ItemOwnerField = "owner"
	// ItemOwnerOperator is the one owner value the tool enforces, and it
	// names the workbench's operator rather than any particular person.
	// What enforces it is closeItem in internal/verb, which refuses a
	// terminal verb on an item recording this value to anybody else, and
	// SetField beside it, which refuses a rewrite of the owner key on one.
	ItemOwnerOperator = "operator"
	// ItemResolutionField is the canonical reference of the comment a
	// terminal verb designated as this item's answer. It replaced the
	// free-text note key with dinah-525: a note could say who settled the
	// item only by implication, where a comment carries its own author and
	// the designation carries whoever chose it, so the record says both
	// rather than leaving one to be assumed.
	//
	// The reference names a comment of this item. Aiming it at another
	// item's answer or at a card comment is refused at the write, which is
	// what lets a reader open it without asking whose answer it is.
	ItemResolutionField = "resolution"
	// ItemNoteRetiredField is the key ItemResolutionField replaced. Nothing
	// writes it, and it is named here rather than spelled at its two
	// readers: dinah check, which reports a store the note migration has
	// not reached, and the migration itself, which carries the text into a
	// comment and removes the key.
	ItemNoteRetiredField = "note"
	// CitationsField is the sequence of citations an item carries.
	CitationsField = "citations"
	// ItemStandingField is the key of the standing entry that minted the
	// item, which is the item's identity for re-entry: an arrival at the
	// declaring column mints nothing for an entry whose key a live item of
	// the card already carries under this key. Minting alone writes it. It
	// is not a field of the item, so `dinah set <item> standing` and `dinah
	// get <item> standing` are both refused as an unknown field, and `dinah
	// show <item>` is its reading surface.
	ItemStandingField = "standing"
	// ItemEvidenceField is the evidence scheme the item has to be settled
	// against. closeItem in internal/verb refuses resolve, verify and fail
	// on an item carrying it while no citation of the item names that
	// scheme. It is a field of the item in its own right, so a hand-filed
	// item can carry the same demand, and its write follows the authority
	// the column key already has.
	ItemEvidenceField = "evidence"
)

// The six states a checklist item takes. The set is closed because method
// text travels between workbenches and a state has to mean one thing
// everywhere.
//
// ItemWaived records that the finding the item carries stands and that the
// workbench operator has decided the card may proceed regardless. Nothing
// about the finding is unsaid by the waiver: a waived acceptance criterion
// was not met, or was never checked, and the item goes on saying so.
//
// ItemWithdrawn records that the question the item carries stopped being a
// question, usually because the card changed underneath it. The item is not
// answered, not checked and not abandoned by whoever should have answered it,
// and a state saying either of those would be a different claim.
const (
	ItemPending   = "pending"
	ItemResolved  = "resolved"
	ItemVerified  = "verified"
	ItemFailed    = "failed"
	ItemWaived    = "waived"
	ItemWithdrawn = "withdrawn"
)

// ItemKinds are the three kinds an item takes, in the order the format
// declares them. A surface offering a caller the choice reads this rather
// than writing the three out again.
var ItemKinds = []string{"acceptance_criterion", "open_question", "decision"}

// ItemStates are the six states an item takes, in the order the constants
// above declare them. A surface offering a caller the choice reads this for
// ItemKinds' own reason rather than writing the six out again.
var ItemStates = []string{ItemPending, ItemResolved, ItemVerified, ItemFailed, ItemWaived, ItemWithdrawn}

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
	ordinal, err := nextOrdinal(collection, ItemAnchor)
	if err != nil {
		return nil, err
	}
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

// AddStandingItem writes one instance of a standing entry under a card, on
// AddItem's own terms: the caller holds the card's lock. It writes the entry's
// kind, the pending state, the declaring column, the owner where the entry
// declares one, the standing key, the evidence scheme where the entry declares
// one, the stamp and the ordinal, and the entry's text as the body, which is a
// copy taken at minting so one card's instance can be edited without touching
// every other card's.
func AddStandingItem(cardDir, columnID string, entry StandingItem, ts string) (*Item, error) {
	collection := filepath.Join(cardDir, ChecklistDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	ordinal, err := nextOrdinal(collection, ItemAnchor)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(collection, id)
	fm := NewFrontmatter()
	fm.Set(ItemKindField, entry.Kind)
	fm.Set(ItemStateField, ItemPending)
	fm.Set(ItemColumnField, columnID)
	if entry.Owner != "" {
		fm.Set(ItemOwnerField, entry.Owner)
	}
	fm.Set(ItemStandingField, entry.Key)
	if entry.Evidence != "" {
		fm.Set(ItemEvidenceField, entry.Evidence)
	}
	fm.Set("ts", ts)
	fm.Set(OrdinalField, strconv.Itoa(ordinal))
	if err := WriteText(filepath.Join(dir, ItemAnchor), fm.Render(entry.Text)); err != nil {
		return nil, err
	}
	return &Item{
		ID:       id,
		Dir:      dir,
		Kind:     entry.Kind,
		State:    ItemPending,
		Column:   columnID,
		Owner:    entry.Owner,
		Standing: entry.Key,
		Evidence: entry.Evidence,
		Text:     entry.Text,
	}, nil
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

// CitationSchemes answers the scheme each entry of an item's citations
// sequence names, in stored order, which is what the evidence demand on an
// item reads: whether at least one citation names the scheme the item has to
// be settled against. It reads the scheme member alone and nothing about the
// target, on the posture Cite takes toward a scheme it has never heard of.
//
// The entries are read through blockValue, the reader every structured
// frontmatter value is read by, so an entry carrying a member this build does
// not know still answers its scheme.
func CitationSchemes(fm *Frontmatter) []string {
	if !fm.Has(CitationsField) {
		return nil
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(blockValue(fm, CitationsField), &entries); err != nil {
		return nil
	}
	var schemes []string
	for _, entry := range entries {
		var scheme string
		if err := json.Unmarshal(entry[ItemCitationScheme], &scheme); err != nil {
			continue
		}
		schemes = append(schemes, scheme)
	}
	return schemes
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
