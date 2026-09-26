package bench

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dinah/internal/contract"
)

// Link is one entry of a card's links sequence: a kind and the identifier of
// the card it names. A link is a declaration rather than an entity. Nothing in
// the core reads one, and only a kind the workbench declares under dinah.holds
// carries behaviour, which is Dinah's own: HoldEdges reads such links.
type Link struct {
	// Kind is an open value; no contract behaviour hangs on its members, and
	// a kind carries behaviour only where the workbench declares it under
	// dinah.holds.
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
	// Route is the name of the route this card walks, empty on a card walking
	// the workbench's full ordered column list. A name the workbench does not
	// declare is stored and read as written, and the card walks the default
	// route until somebody repairs it, on the posture tier_at already keeps
	// for a column reference that no longer resolves.
	Route string
	// StartAfter, StartBy and Due are the card's three scheduling dates, as
	// stored, each empty where the card carries none. A stored value that
	// does not parse is kept as written, read as absent by every condition
	// and by selection, and reported by dinah check. They hold only what the
	// card itself declares: a date derived from anything else is computed on
	// read and never written here.
	StartAfter string
	StartBy    string
	Due        string
	// RetirementGrant is the identifier of the column this card stood in
	// when the workbench operator gave it a criterion-retirement grant, and
	// empty on a card carrying no grant. Under a standing grant an actor who
	// is not the operator may withdraw an acceptance criterion of this card,
	// and nothing else: the grant reaches neither the owner guard, nor waive,
	// nor the item's own column key, nor archiving or deleting an item.
	//
	// It carries a column rather than a clock because the narrowing and the
	// tidying are one episode at one station, so binding the grant to that
	// station bounds it with a fact the journal already records. The card's
	// next move spends it, whatever the direction.
	RetirementGrant string
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
	// src is where the card was read, which its own history is read through
	// too; nil reads as Disk.
	src Source
}

// source is the Source this card was read through, or Disk for a card built
// any other way.
func (c *Card) source() Source {
	if c.src == nil {
		return Disk{}
	}
	return c.src
}

// LoadCard reads one card from a cards collection, refusing one that is not
// written in the vocabulary this build reads: a card carrying no column key at
// all, and a card carrying the column key beside the retired substate key.
func LoadCard(collection, id string) (*Card, error) {
	return loadCard(Disk{}, collection, id, true)
}

// loadRetiredCard reads a card written in the retired vocabulary without
// refusing it, which the lenient opener the migration reads through is the one
// caller of. A workbench that opener admits declares a pre-vocabulary revision
// on its own anchor, so its cards and its anchor agree with each other and the
// disagreement LoadCard refuses is not present. What such a card gives back is
// still read in the current vocabulary's meaning of the two keys, so nothing
// here should take its Column or its State for the card's real standing; the
// migration itself reads cards through ParseAnchor for exactly that reason.
func loadRetiredCard(src Source, collection, id string) (*Card, error) {
	return loadCard(src, collection, id, false)
}

// loadCard is the body both readers share. refuseRetired is what separates
// them, and it is a parameter rather than a package flag so that the choice is
// made at the call site by a caller who knows which vocabulary the workbench
// it opened declares.
func loadCard(src Source, collection, id string, refuseRetired bool) (*Card, error) {
	anchor := joinMember(collection, id, CardAnchor)
	// One read answers both the text and the revision, so the revision a card
	// carries is the revision of the very bytes its fields were parsed from.
	observeAnchor(anchor)
	kind, derive := DeriveCard, cardFromText
	if !refuseRetired {
		kind, derive = DeriveCardRetired, retiredCardFromText
	}
	value, err := src.Derive(anchor, kind, derive)
	if err != nil {
		var refusal *contract.Refusal
		if errors.As(err, &refusal) {
			return nil, err
		}
		return nil, contract.Refuse(contract.UnknownCard, id)
	}
	card := value.(*Card).Clone()
	card.src = src
	return card, nil
}

// cardFromText is DeriveCard's derive function: the card an anchor's text
// and revision describe, in the vocabulary this build reads.
func cardFromText(path, text, revision string) (any, error) {
	return parseCard(path, text, revision, true)
}

// retiredCardFromText is DeriveCardRetired's derive function, which reads
// the card without refusing the retired vocabulary.
func retiredCardFromText(path, text, revision string) (any, error) {
	return parseCard(path, text, revision, false)
}

// parseCard is loadCard's body after its read. The card's directory and
// identifier come from the anchor's path, so the answer is a pure function of
// the path and the bytes, which is what lets a resident snapshot memoise it.
func parseCard(anchor, text, revision string, refuseRetired bool) (*Card, error) {
	dir := filepath.Dir(anchor)
	id := filepath.Base(dir)
	fm, body := ParseAnchor(text)
	fm.markShared()
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
	// the rename at its anchor and not in its cards passes it, and dinah list
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
		Route:       fm.Value(RouteField),
		StartAfter:  fm.Value(StartAfterField),
		StartBy:     fm.Value(StartByField),
		Due:         fm.Value(DueField),

		RetirementGrant: fm.Value(RetirementGrantKey),
		Workstreams:     fm.Seq("workstreams"),
		Body:            body,
		Revision:        revision,
		FM:              fm,
	}
	card.Links = readLinks(fm)
	card.ColumnTiers = readColumnTiers(fm)
	if card.State == "" {
		card.State = contract.StateReady
	}
	return card, nil
}

// LoadCardIn loads a card and stamps the number the workbench allocates it.
// The free LoadCard reads a card's own file and can know nothing about a
// number that no longer lives there, so every caller that goes on to read
// Number or to call Ref comes through here.
func (b *Bench) LoadCardIn(root, id string) (*Card, error) {
	card, err := loadCard(b.source(), root, id, true)
	if err != nil {
		return nil, err
	}
	b.stamp(card)
	return card, nil
}

// loadRetiredCardIn is LoadCardIn for a bench written in the retired
// vocabulary, which the lenient opener is the only source of. The fallback
// lives in stamp rather than in LoadCardIn alone so that the retired route
// reaches it too, which is the whole reason this method exists.
func (b *Bench) loadRetiredCardIn(root, id string) (*Card, error) {
	card, err := loadRetiredCard(b.source(), root, id)
	if err != nil {
		return nil, err
	}
	b.stamp(card)
	return card, nil
}

// stamp writes the number the workbench holds for this card onto the card it
// is given. A workbench at RegistryFormat or above reads the registry, where
// the first line in file order claiming the card is the one that counts and a
// card no line claims keeps zero, which is what Ref reads as no number. A
// workbench below it reads the number key the card's own frontmatter still
// carries, which is the whole of the legacy read path.
func (b *Bench) stamp(c *Card) {
	c.Number = b.numberOf(c.ID, c.FM)
}

// numberOf is the number the workbench holds for a card, read from the
// registry at RegistryFormat and above and from the card's own header below
// it, and zero where neither holds one. stamp and LiveCardHeaders both ask it,
// so a card and its header cannot be numbered two ways.
func (b *Bench) numberOf(id string, fm *Frontmatter) int {
	if b.Format >= RegistryFormat {
		return b.Numbers.ByID[id]
	}
	claimed := fm.Value("number")
	if claimed == "" {
		return 0
	}
	number, _ := strconv.Atoi(claimed)
	return number
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

// renderLinks is the links block as the anchor carries it, which the
// frontmatter takes verbatim because no typed setter covers a sequence of
// mappings. It mirrors renderColumnTiers: one dashed entry per link, in the
// order the slice holds them, so a write that adds or removes one leaves the
// rest where a reader last saw them, and both values go through quote for the
// reason renderColumnTiers quotes.
func renderLinks(links []Link) []string {
	lines := []string{"links:"}
	for _, link := range links {
		lines = append(lines, "  - kind: "+quote(link.Kind))
		lines = append(lines, "    to: "+quote(link.To))
	}
	return lines
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
//
// The number is not written here, on either side of the registry: at
// RegistryFormat it lives in card-numbers.txt and nowhere else, and below
// that a card still carrying a number key keeps it across an edit because
// this write preserves the raw lines, which is what the legacy read path in
// stamp depends on.
func (c *Card) Save() error {
	c.FM.Set("title", c.Title)
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
	// The route lands beside the three level fields and on their own terms:
	// SetAfter anchors it on state, so it comes out under them in the order
	// the calls run, and a key somebody placed by hand stays put.
	setOrDelete(c.FM, RetirementGrantKey, c.RetirementGrant)
	setAfterOrDelete(c.FM, RouteField, c.Route, "state")
	setAfterOrDelete(c.FM, TierField, c.Tier, "state")
	setAfterOrDelete(c.FM, PriorityField, c.Priority, "state")
	setAfterOrDelete(c.FM, SeverityField, c.Severity, "state")
	// The three dates land after the route and the levels, in their own
	// order, and a key somebody placed by hand stays put.
	SetScheduleDate(c.FM, StartAfterField, c.StartAfter)
	SetScheduleDate(c.FM, StartByField, c.StartBy)
	SetScheduleDate(c.FM, DueField, c.Due)
	if len(c.ColumnTiers) == 0 {
		c.FM.Delete(TierAtKey)
	} else {
		c.FM.SetRaw(TierAtKey, renderColumnTiers(c.ColumnTiers))
	}
	if len(c.Links) == 0 {
		c.FM.Delete("links")
	} else {
		c.FM.SetRaw("links", renderLinks(c.Links))
	}
	c.FM.SetSeq("workstreams", c.Workstreams)
	rendered := c.FM.Render(c.Body)
	if err := WriteText(c.AnchorPath(), rendered); err != nil {
		return err
	}
	// The revision is computed over the bytes WriteText stored rather than
	// read back from the file, which is the same value under the card lock
	// every caller holds and which reads nothing, so a card cannot take its
	// revision from anywhere but the write it just made.
	c.Revision = TextRevision(NormalizeNewlines(rendered))
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

// The frontmatter keys carrying a card's three scheduling dates. Each is a
// calendar date written YYYY-MM-DD, and each is optional and independent of
// the other two.
const (
	// StartAfterField is the first day selection may hand the card out.
	StartAfterField = "start_after"
	// StartByField is the last day somebody should have taken the card up.
	StartByField = "start_by"
	// DueField is the last day for the card to reach a done column.
	DueField = "due"
)

// RetirementGrantKey is the frontmatter key carrying a card's
// criterion-retirement grant, whose value is the identifier of the column the
// card stood in when the grant was given.
//
// It is a key of its own rather than a declared field, because a field is
// something a person writes with dinah set and this one is written by two
// verbs of its own that are refused to anybody but the workbench operator.
const RetirementGrantKey = "retirement_grant"

// TierAtKey is the frontmatter key carrying a card's per-column tier
// overrides. It is a key of its own rather than a level, because what it holds
// is a sequence of mappings and not a value.
const TierAtKey = "tier_at"

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
	events, err := readJournalShared(c.source(), c.JournalPath())
	if err != nil {
		return time.Time{}
	}
	return ArrivalFrom(events, c.Column)
}

// ArrivalFrom is the moment a card standing in column entered it, read out of
// the card's journal events by the rule Arrival states: the last created
// event, or the last moved or manual_correction event naming the column,
// whichever comes later. A reader that already holds the events reads the
// arrival through this rather than asking Arrival to read the journal again.
func ArrivalFrom(events []Event, column string) time.Time {
	arrival := time.Time{}
	for _, ev := range events {
		switch ev.Event {
		case contract.EventCreated:
			arrival = ParseStamp(ev.TS)
		case contract.EventMoved, contract.EventManualCorrection:
			if ev.To == column {
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
	return ArrivedBefore(a, a.Arrival(), b, b.Arrival())
}

// ArrivedBefore is ByArrival over arrivals a caller has already read, so a
// sort reads each card's history once rather than once per comparison.
func ArrivedBefore(a *Card, first time.Time, b *Card, second time.Time) bool {
	if !first.Equal(second) {
		return first.Before(second)
	}
	return a.Number < b.Number
}

// cardHeaderLimit is the most of an anchor ReadCardHeader reads looking for
// the fence that closes the frontmatter.
const cardHeaderLimit = 64 * 1024

// ReadCardHeader reads a card anchor's frontmatter and stops at the line that
// closes it, so the body is never read however long it is. It reads at most
// 64 KiB, strips a carriage return from each line it reads, and computes no
// revision. An anchor that opens no frontmatter, or does not close it inside
// that limit, is an error.
func ReadCardHeader(anchor string) (*Frontmatter, error) {
	return readCardHeader(Disk{}, anchor)
}

// readCardHeader is ReadCardHeader's body, reading the anchor's head through
// src.
func readCardHeader(src Source, anchor string) (*Frontmatter, error) {
	head, err := src.ReadHead(anchor, cardHeaderProbe)
	if err != nil {
		return nil, err
	}
	if fm, err, decided := headerIn(anchor, head, len(head) < cardHeaderProbe); decided {
		return fm, err
	}
	head, err = src.ReadHead(anchor, cardHeaderLimit)
	if err != nil {
		return nil, err
	}
	fm, err, _ := headerIn(anchor, head, true)
	return fm, err
}

// cardHeaderProbe is how much of an anchor readCardHeader reads first. Most
// headers close well inside it, so reading every header on a keystroke reads
// a few kilobytes of each anchor rather than up to cardHeaderLimit; a header
// that does not close inside it is read again up to the limit.
const cardHeaderProbe = 4096

// headerIn reads a card anchor's frontmatter out of the head of its bytes,
// line by line, stopping at the line that closes it. complete says the head
// is all the reader will get: the whole file, or cardHeaderLimit of it. A
// head that is not complete decides nothing about a last line it cut short,
// and answers decided false when it holds no closing fence, so the caller
// reads further.
func headerIn(anchor string, head []byte, complete bool) (*Frontmatter, error, bool) {
	reader := bufio.NewReader(bytes.NewReader(head))
	var text strings.Builder
	for lineNumber := 0; ; lineNumber++ {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !complete {
			return nil, nil, false
		}
		text.WriteString(line)
		fence := strings.TrimSpace(line) == "---"
		if lineNumber == 0 && !fence {
			return nil, fmt.Errorf("%s opens no frontmatter", anchor), true
		}
		if lineNumber > 0 && fence {
			fm, _ := ParseAnchor(text.String())
			return fm, nil, true
		}
		if readErr != nil {
			return nil, fmt.Errorf("%s closes no frontmatter within %d bytes", anchor, cardHeaderLimit), true
		}
	}
}

// CardHeader is what a card's frontmatter says about it, read without its
// body. It carries no revision and cannot be saved.
type CardHeader struct {
	// ID is the card's identifier, which is its directory's name.
	ID string
	// Number is the number the workbench's registry holds for the card, and
	// zero where the registry holds none.
	Number int
	// Title is what a person calls the card.
	Title string
	// Column is the identifier of the column the card stands in.
	Column string
	// State is ready, active or blocked, and ready where the header names
	// none, which is how LoadCard reads the same header.
	State string
	// Holder is the owner holding the card, empty when nobody does.
	Holder string
}

// LiveCardHeaders reads the header of every live card, skipping any whose
// header will not read. It reads no body and computes no revision, which is
// what makes asking how many cards stand in each column cheap enough to do on
// a keystroke. BeforeHeaderRead, when set, is asked before each anchor opens.
func (b *Bench) LiveCardHeaders() ([]CardHeader, error) {
	ids, err := b.ListIDs(b.CardsRoot())
	if err != nil {
		return nil, err
	}
	headers := make([]CardHeader, 0, len(ids))
	for _, id := range ids {
		if b.BeforeHeaderRead != nil {
			if err := b.BeforeHeaderRead(); err != nil {
				return nil, err
			}
		}
		anchor := filepath.Join(b.CardsRoot(), id, CardAnchor)
		fm, err := readCardHeader(b.source(), anchor)
		if err != nil {
			continue
		}
		header := CardHeader{
			ID:     id,
			Title:  fm.Value("title"),
			Column: fm.Value("column"),
			State:  fm.Value("state"),
			Holder: fm.Value("claim_holder"),
		}
		if header.State == "" {
			header.State = contract.StateReady
		}
		header.Number = b.numberOf(id, fm)
		headers = append(headers, header)
	}
	return headers, nil
}
