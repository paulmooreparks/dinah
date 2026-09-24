package bench

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"dinah/internal/contract"
)

// StandingItemsKey is the column frontmatter key carrying the column's
// standing items: a nested mapping whose members are the entries, each keyed
// by the name the workbench chose and carrying the members StandingItem holds.
// The block is hand-written into the column's own anchor, the route
// require_fields, reject_to and loop_limit take, and CORE-JSON-14 blesses the
// member the interchange form carries it under.
const StandingItemsKey = "standing_items"

// The four members one standing entry carries.
const (
	standingKindMember     = "kind"
	standingTextMember     = "text"
	standingOwnerMember    = "owner"
	standingEvidenceMember = "evidence"
)

// StandingItem is one entry of a column's standing_items declaration: the
// checklist item every card arriving at the column receives an instance of.
//
// The key is the entry's identity. It is what makes a re-entry idempotent,
// because a card that has left the column and come back carries an instance
// whose standing key names this entry, and the arrival mints nothing for it.
// Every other member is copied onto the instance at minting and never read
// again for it, so editing an entry's text after cards carry the item
// rewrites nothing on any card.
type StandingItem struct {
	// Key is the mapping member's name, matching HarnessNameExpression.
	Key string
	// Kind is one of ItemKinds.
	Kind string
	// Text is the item's one line of prose, read to the end of its line
	// unparsed, the way a declared field's meaning is read.
	Text string
	// Owner is who the instance names as its answerer, empty where the entry
	// declares none. ItemOwnerOperator is the one value the tool enforces.
	Owner string
	// Evidence is the scheme the instance has to be settled against, empty
	// where the entry declares none.
	Evidence string
}

// readStandingItems reads a column's standing_items block, answering the
// entries it declares in declaration order and, for each entry it refused, the
// key or the offending line.
//
// The reader posture is readDeclaredFields's: a line it cannot read is skipped
// rather than raised over, so a hand-damaged block leaves the column openable
// and `dinah check` is what puts the damage in front of a reader. Four entry
// defects are refused and reported, and each leaves the entry undeclared: a
// key outside the one-segment grammar, a kind absent or outside the three, a
// text absent or blank, and a dashed line where a member line was expected. A
// duplicate key keeps its first occurrence, as a duplicate declared field does.
//
// The block's levels are told apart by indentation rather than by a second
// pattern, because an entry key and a member name are the same shape of line
// and only their depth separates them.
func readStandingItems(fm *Frontmatter) ([]StandingItem, []string) {
	block, malformed := standingBlock(fm)
	var entries []StandingItem
	for _, entry := range block {
		item := StandingItem{Key: entry.key}
		for _, member := range entry.members {
			switch member.name {
			case standingKindMember:
				item.Kind = member.value
			case standingTextMember:
				item.Text = member.value
			case standingOwnerMember:
				item.Owner = member.value
			case standingEvidenceMember:
				item.Evidence = member.value
			}
		}
		entries = append(entries, item)
	}
	declared := make([]StandingItem, 0, len(entries))
	for _, entry := range entries {
		if !HarnessName(entry.Key) || !KnownItemKind(entry.Kind) || strings.TrimSpace(entry.Text) == "" {
			malformed = append(malformed, entry.Key)
			continue
		}
		declared = append(declared, entry)
	}
	return declared, malformed
}

// standingEntry is one entry of a standing_items block as the file spells it:
// the key, and its member lines in the order written, each member's value read
// to the end of its line as text. It is the structural pass readStandingItems
// and standingItemsValue share, so the declaration reader and the export read
// one grammar: every member of an entry is a scalar, and a bare value opening
// with a bracket is the text it spells rather than the flow sequence the
// schema-free block reader would take it for.
type standingEntry struct {
	key     string
	members []standingMember
}

// standingMember is one member line of a standing entry, name to text.
type standingMember struct {
	name  string
	value string
}

// standingBlock reads a column's standing_items block into its entries in
// declaration order, and answers beside them every dashed line met anywhere
// in the block, trimmed, as the lines the grammar has no reading for.
//
// A duplicate key keeps its first occurrence and a duplicate member within an
// entry keeps its last, which is what the switch in readStandingItems always
// did. An unknown member name is kept rather than skipped, so the export
// carries it and a later build declaring it reads it.
func standingBlock(fm *Frontmatter) ([]standingEntry, []string) {
	var entries []standingEntry
	var dashed []string
	seen := map[string]bool{}
	entryIndent := -1
	current := -1
	for _, line := range fm.Raw(StandingItemsKey) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if fieldBlockEntry.MatchString(line) {
			// No member of a standing entry takes a dashed list, so a
			// dashed line anywhere in the block is a line the grammar has no
			// reading for. It is reported rather than skipped, because the
			// entry pattern is tried first and a key spelled with a leading
			// hyphen would otherwise vanish into whatever entry was open.
			dashed = append(dashed, strings.TrimSpace(line))
			continue
		}
		m := fieldBlockMember.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent, name, rest := len(m[1]), unquote(strings.TrimSpace(m[2])), m[3]
		if entryIndent < 0 {
			entryIndent = indent
		}
		if indent <= entryIndent {
			current = -1
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			entries = append(entries, standingEntry{key: name})
			current = len(entries) - 1
			continue
		}
		if current < 0 {
			continue
		}
		entries[current].members = append(entries[current].members, standingMember{name: name, value: unquote(strings.TrimSpace(rest))})
	}
	return entries, dashed
}

// standingItemsValue is the column's standing_items block as the JSON value
// the interchange form carries: an object whose members are the entries in
// declaration order, each an object of its members in the order written, and
// every member's value a JSON string. It is written from the same pass the
// declaration reader takes, so a text the reader accepted is the text the
// export carries, whatever a schema-free reading of its bare spelling would
// have made of it. A malformed entry travels as written, on the terms the
// block reader carries it, so nothing the author declared is lost between
// the source and the clone, and a dashed line has no JSON spelling and is
// dropped as the block reader drops it. A declared block with no entry, or
// holding only dashed lines, is the empty object, which is the member's
// shape with nothing in it, and it round-trips: the import writes it as the
// bare line `standing_items: {}`, which the block reader reads as a block of
// no entries.
func standingItemsValue(fm *Frontmatter) json.RawMessage {
	block, _ := standingBlock(fm)
	if len(block) == 0 {
		return json.RawMessage("{}")
	}
	entries := make([]jsonMember, 0, len(block))
	for _, entry := range block {
		members := make([]jsonMember, 0, len(entry.members))
		seen := map[string]bool{}
		for i := len(entry.members) - 1; i >= 0; i-- {
			// The last spelling of a duplicated member is the one the
			// declaration reader keeps, so it is the one that travels.
			member := entry.members[i]
			if seen[member.name] {
				continue
			}
			seen[member.name] = true
			members = append([]jsonMember{{name: member.name, value: mustMarshal(member.value)}}, members...)
		}
		entries = append(entries, jsonMember{name: entry.key, value: jsonObject(members)})
	}
	return jsonObject(entries)
}

// MissingStandingItems answers the entries of a column's declaration the card
// carries no live instance of, in declaration order.
//
// An instance is present when a live item's column is the declaring column's
// identifier and its standing key is the entry's key, whatever the item's
// state: a resolved, verified, failed, waived or withdrawn instance is a record
// of a judgement the card keeps, and `dinah reopen` is the way to re-impose a
// hold. The archive is not read, because an archived instance was taken out of
// the live set by somebody entitled to, and a card arriving again meets the
// declaration afresh. An item whose anchor will not open is passed over, as
// itemsWhere already passes over one.
func (b *Bench) MissingStandingItems(card *Card, column *Column) ([]StandingItem, error) {
	if len(column.StandingItems) == 0 {
		return nil, nil
	}
	instances, err := itemsWhere(card.Dir, func(item *Item) bool {
		return item.Column == column.ID && item.Standing != ""
	})
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	for _, instance := range instances {
		present[instance.Standing] = true
	}
	var missing []StandingItem
	for _, entry := range column.StandingItems {
		if present[entry.Key] {
			continue
		}
		missing = append(missing, entry)
	}
	return missing, nil
}

// PendingStandingInstances answers the live items of a card that a standing
// declaration minted for one column and that still stand pending: the
// instances a reshape retiring that column withdraws. An instance in any other
// state is a record of a judgement taken and is left as it stands, and a
// hand-filed item naming the column carries no standing key and is not one.
func PendingStandingInstances(cardDir, columnID string) ([]*Item, error) {
	return itemsWhere(cardDir, func(item *Item) bool {
		return item.Column == columnID && item.Standing != "" && item.State == ItemPending
	})
}

// StandingInstancesOwedAWithdrawal answers the instances PendingStandingInstances
// answers together with the withdrawn instances of the same column whose
// withdrawal the card's journal never recorded: an anchor reading withdrawn
// and designating a comment, with no item_withdrawn line naming the item. That
// is the shape a reshape stopped between an instance's anchor and its journal
// lines leaves behind, and the re-run completes the record rather than passing
// over an anchor nothing accounts for. A withdrawn instance whose line stands
// is not answered, so nothing is written twice.
func StandingInstancesOwedAWithdrawal(cardDir, columnID string) ([]*Item, error) {
	candidates, err := itemsWhere(cardDir, func(item *Item) bool {
		if item.Column != columnID || item.Standing == "" {
			return false
		}
		return item.State == ItemPending || (item.State == ItemWithdrawn && item.Resolution != "")
	})
	if err != nil {
		return nil, err
	}
	var owed []*Item
	var recorded map[string]bool
	for _, item := range candidates {
		if item.State == ItemPending {
			owed = append(owed, item)
			continue
		}
		if recorded == nil {
			recorded = map[string]bool{}
			events, _, err := ReadJournal(filepath.Join(cardDir, JournalName))
			if err != nil {
				return nil, err
			}
			for _, ev := range events {
				if ev.Event == contract.EventItemWithdrawn {
					recorded[ev.Item] = true
				}
			}
		}
		if !recorded[item.ID] {
			owed = append(owed, item)
		}
	}
	return owed, nil
}
