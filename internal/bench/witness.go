package bench

import (
	"path/filepath"

	"dinah/internal/contract"
)

// WitnessDivergence appends a manual_correction event to a card's journal when
// the journal's believed position disagrees with what the anchor currently
// records, reconciling a hand edit the way the format's "Manual edits are
// witnessed, not prevented" section promises. It reports whether it wrote a
// line.
//
// The anchor is the present and the journal is history, so the direction is
// fixed: from is always the position the journal replays to, to is always the
// column the anchor names, never the reverse. A journal that says nothing
// about position at all is not a divergence and is left alone.
//
// The caller holds the card's lock and hands it over as held; this takes
// none of its own.
func (b *Bench) WitnessDivergence(held *Lock, actor, now string, card *Card) (bool, error) {
	events, _, err := ReadJournal(card.JournalPath())
	if err != nil {
		return false, err
	}
	if b.CardUnit() {
		if fm, body, ok := ReplayCardFields(events); ok {
			return b.witnessProjection(held, actor, now, card, fm, body)
		}
	}
	believed := ReplayPosition(events)
	if believed == "" || believed == card.Column {
		return false, nil
	}
	fromTitle, toTitle := "", ""
	if from := b.Column(believed); from != nil {
		fromTitle = from.Title
	}
	if to := b.Column(card.Column); to != nil {
		toTitle = to.Title
	}
	ev := Event{
		TS:        now,
		Event:     contract.EventManualCorrection,
		Actor:     NamedActor(actor),
		From:      believed,
		FromTitle: fromTitle,
		To:        card.Column,
		ToTitle:   toTitle,
	}
	if err := AppendEvent(held, card.JournalPath(), ev); err != nil {
		return false, err
	}
	return true, nil
}

// witnessProjection is the witness of the card-unit layout, where the journal
// states the whole of card.md: every key and the body on which card.md and the
// replay of its journal disagree gets one line making the journal agree with
// the anchor, which is never changed. A differing column is witnessed as a
// manual_correction, the body as a witnessed card_updated carrying the
// anchor's body, and any other key as a witnessed card_updated carrying the
// replayed and the anchor's value, the raw lines of a structured key joined
// by a newline. The actor is whoever's touch found the difference.
func (b *Bench) witnessProjection(held *Lock, actor, now string, card *Card, fm *Frontmatter, body string) (bool, error) {
	text, err := ReadText(card.AnchorPath())
	if err != nil {
		return false, err
	}
	anchor, anchorBody := ParseAnchor(text)
	differ := ProjectionDifferences(anchor, anchorBody, fm, body)
	if len(differ) == 0 {
		return false, nil
	}
	var lines []Event
	for _, key := range differ {
		switch key {
		case "column":
			from, to := fm.Value("column"), anchor.Value("column")
			ev := Event{
				TS:        now,
				Event:     contract.EventManualCorrection,
				Actor:     NamedActor(actor),
				From:      from,
				FromTitle: b.columnTitleAnyHalf(from),
				To:        to,
				ToTitle:   b.columnTitleAnyHalf(to),
			}
			if from == "" || to == "" {
				ev.Event = contract.EventCardUpdated
				ev.Field, ev.From, ev.To, ev.FromTitle, ev.ToTitle, ev.Witnessed = key, from, to, "", "", true
			}
			lines = append(lines, ev)
		case BodyField:
			lines = append(lines, Event{
				TS:        now,
				Event:     contract.EventCardUpdated,
				Actor:     NamedActor(actor),
				Field:     BodyField,
				Text:      anchorBody,
				Witnessed: true,
			})
		default:
			lines = append(lines, Event{
				TS:        now,
				Event:     contract.EventCardUpdated,
				Actor:     NamedActor(actor),
				Field:     key,
				From:      witnessedValue(fm, key),
				To:        witnessedValue(anchor, key),
				Witnessed: true,
			})
		}
	}
	if err := AppendEvents(held, card.JournalPath(), lines); err != nil {
		return false, err
	}
	return true, nil
}

// WriteWitnesses witnesses every live card whose anchor and journal disagree,
// which is the batch form of the same repair the verb path performs on the
// next touch, and repairs a journal ending in a torn tail on the writer's own
// two cases while it holds the card's lock. It returns the identifiers of the
// cards it wrote to.
//
// The walk mirrors Check's own: a card a structural act is in the middle of, a
// card with no anchor, and a card the reader refuses are each stepped over, and
// only live cards are visited, because a witness is useful for a card check
// still evaluates and nothing brings an archived card back into that walk.
//
// A card another process holds the lock on is reported and stepped over rather
// than ending the walk, on the terms BackfillOrdinals already reports one: the
// obstruction is ordinary and the repair can be run again once it clears.
func (b *Bench) WriteWitnesses(actor, now string) ([]string, []Finding, error) {
	var witnessed []string
	var findings []Finding
	cardIDs, err := ListIDs(b.CardsRoot())
	if err != nil {
		return nil, nil, err
	}
	for _, id := range cardIDs {
		dir := filepath.Join(b.CardsRoot(), id)
		if Exists(SiblingPath(dir)) {
			continue
		}
		if !Exists(filepath.Join(dir, CardAnchor)) {
			continue
		}
		lock, err := Acquire(dir, actor, now)
		if err != nil {
			findings = append(findings, Finding{Path: dir, Key: FindingWitnessLocked, Detail: id})
			continue
		}
		card, err := b.LoadCardIn(b.CardsRoot(), id)
		if err != nil {
			lock.Release()
			findings = append(findings, Finding{Path: dir, Key: unreadableCardFinding(err), Detail: id})
			continue
		}
		wrote, err := b.WitnessDivergence(lock, actor, now, card)
		if err == nil {
			var repaired bool
			repaired, err = RepairJournalTail(lock, card.JournalPath(), actor, now)
			wrote = wrote || repaired
		}
		lock.Release()
		if err != nil {
			return witnessed, findings, err
		}
		if wrote {
			witnessed = append(witnessed, id)
		}
	}
	return witnessed, findings, nil
}
