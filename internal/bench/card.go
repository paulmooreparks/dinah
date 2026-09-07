package bench

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dinah/internal/contract"
)

// Link is one entry of a card's links sequence: a kind and the identifier of
// the card it names. A link is a declaration rather than an entity, and
// nothing in the tool reads one.
type Link struct {
	// Kind is an open value; no contract behaviour hangs on its members.
	Kind string
	// To is the identifier of the card the link names.
	To string
}

// ColumnTier is one entry of a card's tier_at sequence: the column the
// override is written for, and the tier a claim into that column must declare
// at or above.
//
// Column is a reference rather than an identifier, resolved through
// Bench.ColumnByRef exactly as a column's own reject_to declaration is, so a
// person writes the slug they read on the board. Tier is always an absolute
// member name of the declared tier set.
type ColumnTier struct {
	// Column is the reference the override was written against, exactly as
	// the anchor carries it.
	Column string
	// Tier is the resolved absolute member name the override requires.
	Tier string
}

// Card is one card of a bench: its identity, its position, its claim or block
// and the framing prose of its body.
type Card struct {
	// ID is the card's 12-hex identifier and the name of its directory.
	ID string
	// Dir is the card's directory, in whichever half of the collection it
	// currently sits.
	Dir string
	// Number is the durable half of the card's human reference.
	Number int
	// Title is what a person calls the card.
	Title string
	// Column is the identifier of the column the card occupies.
	Column string
	// State is one of ready, active and blocked.
	State string
	// Holder and ClaimSince are present exactly when the state is
	// active, which is the implication check enforces both ways.
	Holder     string
	ClaimSince string
	// Expires is the moment a lease lapses, empty when the claim carries no
	// expiry.
	Expires string
	// BlockReason, BlockKind and BlockSince are the block's own fields, and
	// the reason is present exactly when the state is blocked.
	BlockReason string
	BlockKind   string
	BlockSince  string
	// Severity, Priority and Tier are the levels the card records on the
	// three axes a workbench may declare. An empty value means the card
	// carries no level for that axis, which the format declares legal.
	//
	// Tier is the card's own baseline requirement: the tier a claim must
	// declare at or above, at every column the card stands in, unless an
	// entry of ColumnTiers names that column. It is what the card asks of
	// whoever takes it up rather than what any column asks.
	Severity string
	Priority string
	Tier     string
	// ColumnTiers are the card's per-column tier overrides, in the order the
	// anchor carries them. Each entry names a column the way reject_to names
	// one and carries an absolute member of the workbench's declared tier
	// set, never a relative expression, because a stored relative value would
	// repoint the day somebody inserts a rung into the middle of the set.
	ColumnTiers []ColumnTier
	// Links are what this card records about other cards.
	Links []Link
	// Workstreams are the identifiers of the workstreams the card belongs
	// to, membership being card-owned like position.
	Workstreams []string
	// Body is the card's framing prose.
	Body string
	// Revision is the content hash of the anchor as it was read, which is
	// what a basis names.
	Revision string
	// FM is the anchor's header, kept so a write preserves unknown keys.
	FM *Frontmatter
}

// LoadCard reads one card from a cards collection, refusing one that is not
// written in the vocabulary this build reads: a card carrying no column key at
// all, and a card carrying the column key beside the retired substate key.
func LoadCard(collection, id string) (*Card, error) {
	return loadCard(collection, id, true)
}

// loadRetiredCard reads a card written in the retired vocabulary without
// refusing it, which the lenient opener the migration reads through is the one
// caller of. A workbench that opener admits declares a pre-vocabulary revision
// on its own anchor, so its cards and its anchor agree with each other and the
// disagreement LoadCard refuses is not present. What such a card gives back is
// still read in the current vocabulary's meaning of the two keys, so nothing
// here should take its Column or its State for the card's real standing; the
// migration itself reads cards through ParseAnchor for exactly that reason.
func loadRetiredCard(collection, id string) (*Card, error) {
	return loadCard(collection, id, false)
}

// loadCard is the body both readers share. refuseRetired is what separates
// them, and it is a parameter rather than a package flag so that the choice is
// made at the call site by a caller who knows which vocabulary the workbench
// it opened declares.
func loadCard(collection, id string, refuseRetired bool) (*Card, error) {
	dir := filepath.Join(collection, id)
	anchor := filepath.Join(dir, CardAnchor)
	text, err := ReadText(anchor)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownCard, id)
	}
	revision, err := Revision(anchor)
	if err != nil {
		return nil, err
	}
	fm, body := ParseAnchor(text)
	// Which vocabulary a card is written in is decided by the column key,
	// which is the discriminator this card's D-14 settled and the one the
	// migration's own skip-guard already asks about. No card written before
	// the rename carries it, because that vocabulary spells the flow position
	// "state", and every card this build writes carries it, because Card.Save
	// sets it on every write. The substate key answers a narrower question:
	// its presence beside the column key is a header holding half of each
	// vocabulary, and a card can carry the harm without carrying it at all.
	//
	// Reaching here means the workbench's own anchor declares the current
	// vocabulary, because the version gate turns a pre-vocabulary workbench
	// away before any card is opened. So a card that is not across the rename
	// disagrees with the anchor above it, its state key holds a column
	// identifier, and every field below would be filled from the wrong half of
	// the header.
	//
	// The gate cannot see this, and that is the whole reason for the check.
	// It reads the revision the anchor declares, so a workbench carried across
	// the rename at its anchor and not in its cards passes it, and dinah ls
	// then prints a column identifier under the heading that names the card's
	// condition and exits 0. This migration writes the anchor last and so
	// cannot produce that shape, but a hand edit or another tool can, and a
	// silent misread is exactly what the gate exists to prevent. Asking the
	// card itself is the only place the disagreement is visible.
	//
	// The two conditions carry different refusals because their sentences are
	// different sentences. A header carrying both vocabularies is genuinely
	// mixed and the reader is told to pick one. A card written wholly in the
	// retired vocabulary is internally consistent and disagrees with the
	// workbench around it, so telling its reader to remove a mixture would
	// describe a file that does not exist.
	// The two conditions are written as sibling ifs rather than as a switch
	// because a switch naming an anchor constant is a second copy of the
	// containment grammar, which TestTheContainmentGrammarIsDeclaredOnce
	// refuses wherever it is not the table that declares it.
	if refuseRetired && !fm.Has(columnKey) {
		return nil, contract.RefuseWith(contract.VocabularyRetired, filepath.Join(id, CardAnchor), map[string]string{"path": anchor})
	}
	if refuseRetired && fm.Has(preVocabularyStateKey) {
		return nil, contract.RefuseWith(contract.VocabularyMixed, filepath.Join(id, CardAnchor), map[string]string{"path": anchor})
	}
	card := &Card{
		ID:          id,
		Dir:         dir,
		Title:       fm.Value("title"),
		Column:      fm.Value("column"),
		State:       fm.Value("state"),
		Holder:      fm.Value("claim_holder"),
		ClaimSince:  fm.Value("claim_since"),
		Expires:     fm.Value("claim_expires"),
		BlockReason: fm.Value("block_reason"),
		BlockKind:   fm.Value("block_kind"),
		BlockSince:  fm.Value("block_since"),
		Severity:    fm.Value(SeverityField),
		Priority:    fm.Value(PriorityField),
		Tier:        fm.Value(TierField),
		Workstreams: fm.Seq("workstreams"),
		Body:        body,
		Revision:    revision,
		FM:          fm,
	}
	if number := fm.Value("number"); number != "" {
		card.Number, _ = strconv.Atoi(number)
	}
	card.Links = readLinks(fm)
	card.ColumnTiers = readColumnTiers(fm)
	if card.State == "" {
		card.State = contract.StateReady
	}
	return card, nil
}

// readLinks reads the links sequence. Each entry is a mapping, so the block
// is read line by line the way the rest of this codebase reads documents,
// rather than by introducing a YAML parser for one key.
func readLinks(fm *Frontmatter) []Link {
	var links []Link
	current := Link{}
	for _, line := range fm.Raw("links") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "links:" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		value = unquote(strings.TrimSpace(value))
		switch strings.TrimSpace(key) {
		case "kind":
			if current.Kind != "" || current.To != "" {
				links = append(links, current)
				current = Link{}
			}
			current.Kind = value
		case "to":
			current.To = value
		}
	}
	if current.Kind != "" || current.To != "" {
		links = append(links, current)
	}
	return links
}

// readColumnTiers reads the tier_at sequence the way readLinks reads links,
// line by line rather than through a YAML parser, since both keys carry the
// same shape: a block of dashed entries, each a mapping of two keys.
//
// An entry carrying neither key is not started, and a key outside the two is
// ignored, which is the reader posture the rest of this file keeps: what
// cannot be read is left alone rather than raised over.
func readColumnTiers(fm *Frontmatter) []ColumnTier {
	var overrides []ColumnTier
	current := ColumnTier{}
	for _, line := range fm.Raw(TierAtKey) {
		trimmed := strings.TrimSpace(line)
		if trimmed == TierAtKey+":" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		value = unquote(strings.TrimSpace(value))
		switch strings.TrimSpace(key) {
		case "column":
			if current.Column != "" || current.Tier != "" {
				overrides = append(overrides, current)
				current = ColumnTier{}
			}
			current.Column = value
		case TierField:
			current.Tier = value
		}
	}
	if current.Column != "" || current.Tier != "" {
		overrides = append(overrides, current)
	}
	return overrides
}

// renderColumnTiers is the tier_at block as the anchor carries it, which the
// frontmatter takes verbatim because no typed setter covers a sequence of
// mappings. The entries render in the order the card holds them, so a write
// that changes one leaves the rest where a reader last saw them.
func renderColumnTiers(overrides []ColumnTier) []string {
	lines := []string{TierAtKey + ":"}
	for _, override := range overrides {
		lines = append(lines, "  - column: "+quote(override.Column))
		lines = append(lines, "    "+TierField+": "+quote(override.Tier))
	}
	return lines
}

// RequiredTier is the tier a claim into one column must declare at or above,
// and the empty string when the card asks for nothing there.
//
// Two rules answer it, in this order. An entry of ColumnTiers whose column
// reference resolves to the column being claimed into governs that column. The
// card's own baseline governs everywhere else.
//
// There is no third rule. The column's own tier default is never read here,
// valid or stale, because a column default informs and never refuses: only a
// requirement the card itself carries can refuse a claim. The column argument
// is what an override is matched against and nothing else, which is why a card
// nobody has assessed answers empty at every column of every workbench.
func (c *Card) RequiredTier(b *Bench, column *Column) string {
	if b != nil && column != nil {
		for _, override := range c.ColumnTiers {
			if override.Tier == "" {
				continue
			}
			if target := b.ColumnByRef(override.Column); target != nil && target.ID == column.ID {
				return override.Tier
			}
		}
	}
	return c.Tier
}

// SetColumnTier writes one per-column override in memory, ready for Save,
// replacing the entry whose reference resolves to the same column and
// appending when none does. An empty tier removes the entry, so clearing an
// override leaves the card carrying no line for that column rather than a line
// carrying nothing.
//
// The stored reference is whatever the caller passed. A rewrite keeps the
// spelling already on disk, because the entry a person reads should not change
// its wording because somebody wrote the same column a second way.
func (c *Card) SetColumnTier(b *Bench, ref, tier string) {
	target := (*Column)(nil)
	if b != nil {
		target = b.ColumnByRef(ref)
	}
	for i, override := range c.ColumnTiers {
		if !sameColumnRef(b, target, override.Column, ref) {
			continue
		}
		if tier == "" {
			c.ColumnTiers = append(c.ColumnTiers[:i], c.ColumnTiers[i+1:]...)
			return
		}
		c.ColumnTiers[i].Tier = tier
		return
	}
	if tier == "" {
		return
	}
	c.ColumnTiers = append(c.ColumnTiers, ColumnTier{Column: ref, Tier: tier})
}

// ColumnTierFor reads the override written for one column reference, and the
// empty string when the card carries none.
func (c *Card) ColumnTierFor(b *Bench, ref string) string {
	target := (*Column)(nil)
	if b != nil {
		target = b.ColumnByRef(ref)
	}
	for _, override := range c.ColumnTiers {
		if sameColumnRef(b, target, override.Column, ref) {
			return override.Tier
		}
	}
	return ""
}

// sameColumnRef reports whether a stored reference and a written one name one
// column. Two references naming a column this workbench carries are compared
// by that column's identifier, so the slug and the title of one column are one
// key. Two references naming no column are compared as text, which is what
// keeps an override for a retired column rewritable rather than duplicated.
func sameColumnRef(b *Bench, target *Column, stored, ref string) bool {
	if b != nil && target != nil {
		if found := b.ColumnByRef(stored); found != nil {
			return found.ID == target.ID
		}
		return false
	}
	return stored == ref
}

// AnchorPath is the card's anchor file.
func (c *Card) AnchorPath() string {
	return filepath.Join(c.Dir, CardAnchor)
}

// JournalPath is the card's journal, the one non-anchor file a card is never
// without, because birth writes the created event.
func (c *Card) JournalPath() string {
	return filepath.Join(c.Dir, JournalName)
}

// Ref renders the card's human reference, the dash-joined slug and number.
func (c *Card) Ref(slug string) string {
	if slug == "" || c.Number == 0 {
		return c.ID
	}
	return slug + "-" + strconv.Itoa(c.Number)
}

// Save writes the card anchor back and refreshes the revision. Every field
// the tool owns is written, and every field it does not own is preserved by
// the frontmatter holding raw lines.
func (c *Card) Save() error {
	c.FM.Set("title", c.Title)
	if c.Number > 0 {
		c.FM.Set("number", strconv.Itoa(c.Number))
	}
	c.FM.Set("column", c.Column)
	c.FM.Set("state", c.State)
	setOrDelete(c.FM, "claim_holder", c.Holder)
	setOrDelete(c.FM, "claim_since", c.ClaimSince)
	setOrDelete(c.FM, "claim_expires", c.Expires)
	setOrDelete(c.FM, "block_reason", c.BlockReason)
	setOrDelete(c.FM, "block_kind", c.BlockKind)
	setOrDelete(c.FM, "block_since", c.BlockSince)
	// SetAfter inserts directly after its anchor and leaves an existing key
	// where it already sits, so a call made earlier is pushed outward by
	// every call made after it against the same anchor. All three level
	// fields anchor on state, and writing tier, then priority, then severity
	// lands severity, then priority, then tier, whichever of the three are
	// present, and a key somebody placed by hand stays put.
	setAfterOrDelete(c.FM, TierField, c.Tier, "state")
	setAfterOrDelete(c.FM, PriorityField, c.Priority, "state")
	setAfterOrDelete(c.FM, SeverityField, c.Severity, "state")
	if len(c.ColumnTiers) == 0 {
		c.FM.Delete(TierAtKey)
	} else {
		c.FM.SetRaw(TierAtKey, renderColumnTiers(c.ColumnTiers))
	}
	c.FM.SetSeq("workstreams", c.Workstreams)
	if err := WriteText(c.AnchorPath(), c.FM.Render(c.Body)); err != nil {
		return err
	}
	revision, err := Revision(c.AnchorPath())
	if err != nil {
		return err
	}
	c.Revision = revision
	return nil
}

// setOrDelete writes a value or removes the key when the value is empty, so
// an absent field is absent rather than present and blank.
func setOrDelete(fm *Frontmatter, key, value string) {
	if value == "" {
		fm.Delete(key)
		return
	}
	fm.Set(key, value)
}

// setAfterOrDelete writes a value after a named key or removes the key when
// the value is empty, which is setOrDelete over SetAfter rather than over Set:
// a field the tool added to an anchor reads where a reader expects it when
// every writer puts it in the same place.
func setAfterOrDelete(fm *Frontmatter, key, value, after string) {
	if value == "" {
		fm.Delete(key)
		return
	}
	fm.SetAfter(key, value, after)
}

// The frontmatter keys carrying a card's three levels, which are also the
// names of the three axes a workbench declares them under.
const (
	SeverityField = "severity"
	PriorityField = "priority"
	TierField     = "tier"
)

// TierAtKey is the frontmatter key carrying a card's per-column tier
// overrides. It is a key of its own rather than a level, because what it holds
// is a sequence of mappings and not a value.
const TierAtKey = "tier_at"

// CardFields are the fields a card records that a person writes and may
// rewrite, in the order a reader meets them. It is a variable in the source
// rather than a sentence in a catalog, so a fourth card field reaches the
// refusal that lists them without a translator being asked for anything.
var CardFields = []string{SeverityField, PriorityField, TierField}

// KnownCardField reports whether a name is one of the card's own fields,
// which is what both a read of one and a write to one ask first.
func KnownCardField(name string) bool {
	for _, known := range CardFields {
		if known == name {
			return true
		}
	}
	return false
}

// LevelOf reads the level a card records on one axis, and answers the empty
// string for a name outside the set. The caller refuses over the name; this
// reports what is stored under it.
func (c *Card) LevelOf(field string) string {
	switch field {
	case SeverityField:
		return c.Severity
	case PriorityField:
		return c.Priority
	case TierField:
		return c.Tier
	}
	return ""
}

// SetLevel writes one of the card's own levels in memory, ready for Save. A
// name outside the set writes nothing, since the caller has already refused
// over it.
func (c *Card) SetLevel(field, value string) {
	switch field {
	case SeverityField:
		c.Severity = value
	case PriorityField:
		c.Priority = value
	case TierField:
		c.Tier = value
	}
}

// Arrival returns when the card entered the column it now occupies, read from
// its journal rather than from a frontmatter field. The journal is
// authoritative for history, and the arrival of a card is a fact about its
// history, so nothing has to be kept in step with anything.
//
// A witnessed correction counts as an arrival alongside a move, because a card
// whose position was reconciled rather than moved carries no move naming the
// column it stands in, and reading only moves would report the zero time and
// sort it ahead of every card that arrived by one.
func (c *Card) Arrival() time.Time {
	events, _, err := ReadJournal(c.JournalPath())
	if err != nil {
		return time.Time{}
	}
	arrival := time.Time{}
	for _, ev := range events {
		switch ev.Event {
		case contract.EventCreated:
			arrival = ParseStamp(ev.TS)
		case contract.EventMoved, contract.EventManualCorrection:
			if ev.To == c.Column {
				arrival = ParseStamp(ev.TS)
			}
		}
	}
	return arrival
}

// Lapsed reports whether a claim's expiry has passed as of the given moment.
// Expiry is evaluated lazily, at the moment any verb or read touches the
// card, because a single-seat local tool runs no background process and
// CORE-CLAIM-5 requires no daemon to notice the instant it happens.
func (c *Card) Lapsed(now time.Time) bool {
	if c.State != contract.StateActive || c.Expires == "" {
		return false
	}
	expiry := ParseStamp(c.Expires)
	if expiry.IsZero() {
		return false
	}
	return !now.Before(expiry)
}

// ByArrival orders cards the way CORE-QUEUE-3 fixes: the earliest arrival
// first, ties broken by ascending creation ordinal. It reports whether a comes
// before b, which is what sort.Slice wants.
//
// The ordinal is the card's own number, set at birth and never reused, so the
// tie-break is stable across every tool that reads the workbench. The
// identifier was what the retired CORE-QUEUE-1 named, and a random hex string
// makes the order total without making it meaningful.
func ByArrival(a, b *Card) bool {
	first, second := a.Arrival(), b.Arrival()
	if !first.Equal(second) {
		return first.Before(second)
	}
	return a.Number < b.Number
}
