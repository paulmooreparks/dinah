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
	// State is the identifier of the state the card occupies.
	State string
	// Substate is one of ready, active and blocked.
	Substate string
	// Holder and ClaimSince are present exactly when the substate is
	// active, which is the implication fsck enforces both ways.
	Holder     string
	ClaimSince string
	// Expires is the moment a lease lapses, empty when the claim carries no
	// expiry.
	Expires string
	// BlockReason, BlockKind and BlockSince are the block's own fields, and
	// the reason is present exactly when the substate is blocked.
	BlockReason string
	BlockKind   string
	BlockSince  string
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

// LoadCard reads one card from a cards collection.
func LoadCard(collection, id string) (*Card, error) {
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
	card := &Card{
		ID:          id,
		Dir:         dir,
		Title:       fm.Value("title"),
		State:       fm.Value("state"),
		Substate:    fm.Value("substate"),
		Holder:      fm.Value("claim_holder"),
		ClaimSince:  fm.Value("claim_since"),
		Expires:     fm.Value("claim_expires"),
		BlockReason: fm.Value("block_reason"),
		BlockKind:   fm.Value("block_kind"),
		BlockSince:  fm.Value("block_since"),
		Workstreams: fm.Seq("workstreams"),
		Body:        body,
		Revision:    revision,
		FM:          fm,
	}
	if number := fm.Value("number"); number != "" {
		card.Number, _ = strconv.Atoi(number)
	}
	card.Links = readLinks(fm)
	if card.Substate == "" {
		card.Substate = contract.SubstateReady
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
	c.FM.Set("state", c.State)
	c.FM.Set("substate", c.Substate)
	setOrDelete(c.FM, "claim_holder", c.Holder)
	setOrDelete(c.FM, "claim_since", c.ClaimSince)
	setOrDelete(c.FM, "claim_expires", c.Expires)
	setOrDelete(c.FM, "block_reason", c.BlockReason)
	setOrDelete(c.FM, "block_kind", c.BlockKind)
	setOrDelete(c.FM, "block_since", c.BlockSince)
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

// Arrival returns when the card entered the state it now occupies, read from
// its journal rather than from a frontmatter field. The journal is
// authoritative for history, and the arrival of a card is a fact about its
// history, so nothing has to be kept in step with anything.
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
		case contract.EventMoved:
			if ev.To == c.State {
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
	if c.Substate != contract.SubstateActive || c.Expires == "" {
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
