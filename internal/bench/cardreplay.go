package bench

import (
	"errors"
	"path/filepath"
	"strings"

	"dinah/internal/contract"
)

// ReplayCardFields composes card.md from a card's journal: its header and its
// body as the lines state them. It starts from the text of the last
// card_baseline, which carries card.md as the storage migration found it, or,
// on a journal carrying none, from an empty anchor where the journal's first
// line is the card's own created line, and applies every later line. ok is
// false for a journal that states neither start, which is a card whose
// history began before any line said what its anchor held.
//
// Each line sets what its writer sets on the card: created the title, the
// column, the body and the fields the filing named; moved, manual_correction,
// claimed, released, expired, blocked and unblocked the position, the claim
// and the block; card_updated one field, the body from its text; the
// retirement grant, the tier overrides, the links and the workstreams their
// own lines. A card_updated line a witness wrote carries a key's own raw lines
// where the key is structured, and sets them as they stand.
//
// The answer is compared with card.md by value, key by key and the body after
// newline normalisation, since a hand edit may reorder keys or change their
// spacing.
func ReplayCardFields(events []Event) (*Frontmatter, string, bool) {
	start := -1
	for i, ev := range events {
		if ev.Event == contract.EventCardBaseline {
			start = i
		}
	}
	var fm *Frontmatter
	var body string
	switch {
	case start >= 0:
		fm, body = ParseAnchor(events[start].Text)
	case len(events) > 0 && events[0].Event == contract.EventCreated:
		fm = NewFrontmatter()
	default:
		return nil, "", false
	}
	for _, ev := range events[start+1:] {
		body = applyCardLine(fm, body, ev)
	}
	return fm, body, true
}

// applyCardLine applies one journal line to a card's replayed header and
// answers the body after it.
func applyCardLine(fm *Frontmatter, body string, ev Event) string {
	if ev.Event == contract.EventCardUpdated {
		return applyCardUpdate(fm, body, ev)
	}
	card := cardOf(fm, body)
	switch ev.Event {
	case contract.EventCreated:
		card.Title, card.Column, card.State, card.Body = ev.Title, ev.To, contract.StateReady, ev.Text
		for key, value := range ev.Fields {
			setCardKey(fm, key, value)
		}
		card = cardOfKeeping(card, fm)
	case contract.EventMoved:
		card.Column = ev.To
		card.RetirementGrant = ""
	case contract.EventManualCorrection:
		card.Column = ev.To
	case contract.EventClaimed:
		card.State, card.Holder, card.ClaimSince, card.Expires = contract.StateActive, ev.Actor.Name, ev.TS, ev.Expires
	case contract.EventReleased, contract.EventExpired:
		card.State, card.Holder, card.ClaimSince, card.Expires = contract.StateReady, "", "", ""
	case contract.EventBlocked:
		card.State, card.Holder, card.ClaimSince, card.Expires = contract.StateBlocked, "", "", ""
		card.BlockReason, card.BlockKind, card.BlockSince = ev.Reason, ev.Kind, ev.TS
	case contract.EventUnblocked:
		card.State, card.BlockReason, card.BlockKind, card.BlockSince = contract.StateReady, "", "", ""
	case contract.EventRetirementGranted:
		card.RetirementGrant = ev.To
	case contract.EventRetirementRevoked:
		card.RetirementGrant = ""
	case contract.EventTierOverridden:
		card.ColumnTiers = overrideTier(card.ColumnTiers, spelledColumn(ev), ev.To)
	case contract.EventTierOverrideDropped:
		card.ColumnTiers = overrideTier(card.ColumnTiers, spelledColumn(ev), "")
	case contract.EventLinked:
		if indexOfCardLink(card.Links, ev.Kind, ev.To) < 0 {
			card.Links = append(card.Links, Link{Kind: ev.Kind, To: ev.To})
		}
	case contract.EventUnlinked:
		if at := indexOfCardLink(card.Links, ev.Kind, ev.To); at >= 0 {
			card.Links = append(card.Links[:at], card.Links[at+1:]...)
		}
	case contract.EventWorkstreamJoined:
		joined := false
		for _, id := range card.Workstreams {
			joined = joined || id == ev.Workstream
		}
		if !joined {
			card.Workstreams = append(card.Workstreams, ev.Workstream)
		}
	case contract.EventWorkstreamLeft:
		var kept []string
		for _, id := range card.Workstreams {
			if id != ev.Workstream {
				kept = append(kept, id)
			}
		}
		card.Workstreams = kept
	default:
		return body
	}
	card.applyFields()
	return card.Body
}

// cardOfKeeping reads the keys a line set straight into the header back into
// a card whose typed fields that line also set, keeping the typed ones.
func cardOfKeeping(card *Card, fm *Frontmatter) *Card {
	reread := cardOf(fm, card.Body)
	reread.Title, reread.Column, reread.State = card.Title, card.Column, card.State
	return reread
}

// spelledColumn is the reference a tier override line names its column by: the
// spelling the card's tier_at entry carries, or the column's identifier on a
// line written before the spelling was recorded.
func spelledColumn(ev Event) string {
	if ev.ColumnRef != "" {
		return ev.ColumnRef
	}
	return ev.Column
}

// overrideTier sets or, with an empty tier, removes the tier_at entry written
// under one reference, keeping every other entry where it stands.
func overrideTier(overrides []ColumnTier, column, tier string) []ColumnTier {
	for i, override := range overrides {
		if override.Column != column {
			continue
		}
		if tier == "" {
			return append(overrides[:i], overrides[i+1:]...)
		}
		overrides[i].Tier = tier
		return overrides
	}
	if tier == "" {
		return overrides
	}
	return append(overrides, ColumnTier{Column: column, Tier: tier})
}

// indexOfCardLink is where a card carries one link, and -1 where it carries
// none of that kind to that card.
func indexOfCardLink(links []Link, kind, to string) int {
	for i, link := range links {
		if link.Kind == kind && link.To == to {
			return i
		}
	}
	return -1
}

// applyCardUpdate applies a card_updated line: the body from its text, a field
// the card declares by its own rule, a key a witness recorded as the lines it
// found, and any other field as a declared field's value.
func applyCardUpdate(fm *Frontmatter, body string, ev Event) string {
	if ev.Field == BodyField {
		return ev.Text
	}
	if field, known := FieldOf(KindCard, ev.Field); known {
		key := field.Stored()
		switch {
		case field.Guard == GuardDate:
			SetScheduleDate(fm, key, ev.To)
		case ev.To == "":
			fm.Delete(key)
		default:
			fm.Set(key, ev.To)
		}
		return body
	}
	if ev.Witnessed {
		setWitnessedKey(fm, ev.Field, ev.To)
		return body
	}
	SetFieldValue(fm, ev.Field, ev.To)
	return body
}

// setCardKey sets one key a filing wrote, dates by their own rule.
func setCardKey(fm *Frontmatter, key, value string) {
	if field, known := FieldOf(KindCard, key); known && field.Guard == GuardDate {
		SetScheduleDate(fm, key, value)
		return
	}
	fm.Set(key, value)
}

// setWitnessedKey sets a key the way a witness line records it: a scalar
// value, the key's own raw lines joined by a newline for a structured key, or
// nothing, which removes it.
func setWitnessedKey(fm *Frontmatter, key, value string) {
	switch {
	case value == "":
		fm.Delete(key)
	case strings.Contains(value, "\n") || strings.HasPrefix(value, key+":"):
		fm.SetRaw(key, strings.Split(value, "\n"))
	default:
		fm.Set(key, value)
	}
}

// witnessedValue is what a witness line carries for one key of an anchor: the
// scalar value where the key holds one on a single line, and otherwise the
// key's raw lines joined by a newline, so the replay can set them back.
func witnessedValue(fm *Frontmatter, key string) string {
	if !fm.Has(key) {
		return ""
	}
	raw := fm.Raw(key)
	if len(raw) == 1 {
		if value := fm.Value(key); value != "" && strings.TrimSpace(raw[0]) == key+": "+value {
			return value
		}
	}
	return strings.Join(raw, "\n")
}

// RebuildCards writes card.md back from the journal for every card, live and
// archived, in the card-unit layout whose card.md is absent, will not parse or
// carries conflict markers while its journal reads, and appends card_rebuilt
// to each. A card whose card.md reads is not touched. It answers the cards it
// rebuilt, by identifier, and a finding for each card whose lock another
// process holds, which is stepped over.
//
// template carries the timestamp and the actor of the lines it writes. It is
// open to any owner, as the witness is, since it writes only what the journal
// already records.
func (b *Bench) RebuildCards(template Event) ([]string, []Finding, error) {
	if !b.CardUnit() {
		return nil, nil, nil
	}
	var rebuilt []string
	var findings []Finding
	for _, root := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
		ids, err := b.ListIDs(root)
		if err != nil {
			return rebuilt, findings, err
		}
		for _, id := range ids {
			dir := filepath.Join(root, id)
			if b.Exists(b.SiblingPath(dir)) {
				continue
			}
			if _, err := b.LoadCardIn(root, id); !isProjectionUnreadable(err) {
				continue
			}
			lock, err := b.takeLock(dir, template.Actor.Name, template.TS)
			if err != nil {
				findings = append(findings, Finding{Path: dir, Key: FindingWitnessLocked, Detail: id})
				continue
			}
			err = rebuildCard(b.source(), lock, dir, template)
			lock.Release()
			if err != nil {
				return rebuilt, findings, err
			}
			rebuilt = append(rebuilt, id)
		}
	}
	return rebuilt, findings, nil
}

// isProjectionUnreadable reports the refusal a card whose card.md a rebuild
// writes back is read with.
func isProjectionUnreadable(err error) bool {
	var refusal *contract.Refusal
	return errors.As(err, &refusal) && refusal.Name == contract.CardProjectionUnreadable
}

// rebuildCard writes one card's card.md from its journal, rendered as Save
// renders it, and appends card_rebuilt, holding the card's lock.
func rebuildCard(src Source, lock *Lock, dir string, template Event) error {
	journal := filepath.Join(dir, JournalName)
	events, _, err := readJournal(src, journal)
	if err != nil {
		return err
	}
	fm, body, ok := ReplayCardFields(events)
	if !ok {
		return contract.Refuse(contract.CardProjectionUnreadable, filepath.Join(dir, CardAnchor))
	}
	card := cardOf(fm, body)
	if err := WriteText(filepath.Join(dir, CardAnchor), card.Rendered()); err != nil {
		return err
	}
	ev := template
	ev.Event = contract.EventCardRebuilt
	return AppendEvent(lock, journal, ev)
}

// ProjectionDifferences are the keys on which a card's replayed header and its
// card.md disagree, the body named as body, in card.md's own key order with
// the keys only the replay carries after them. Equal means ParseAnchor of each
// gives the same value for the key: the same scalar, or the same raw lines.
func ProjectionDifferences(anchor *Frontmatter, anchorBody string, replayed *Frontmatter, replayedBody string) []string {
	var differ []string
	seen := map[string]bool{}
	for _, key := range anchor.Keys() {
		seen[key] = true
		if witnessedValue(anchor, key) != witnessedValue(replayed, key) {
			differ = append(differ, key)
		}
	}
	for _, key := range replayed.Keys() {
		if !seen[key] {
			differ = append(differ, key)
		}
	}
	if NormalizeNewlines(anchorBody) != NormalizeNewlines(replayedBody) {
		differ = append(differ, BodyField)
	}
	return differ
}
