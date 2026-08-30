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
// The caller already holds the card's lock; this takes none of its own.
func (b *Bench) WitnessDivergence(actor, now string, card *Card) (bool, error) {
	events, _, err := ReadJournal(card.JournalPath())
	if err != nil {
		return false, err
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
		Actor:     actor,
		From:      believed,
		FromTitle: fromTitle,
		To:        card.Column,
		ToTitle:   toTitle,
	}
	if err := AppendEvent(card.JournalPath(), ev); err != nil {
		return false, err
	}
	return true, nil
}

// WriteWitnesses witnesses every live card whose anchor and journal disagree,
// which is the batch form of the same repair the verb path performs on the
// next touch. It returns the identifiers of the cards it wrote a line for.
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
	for _, id := range ListIDs(b.CardsRoot()) {
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
		card, err := LoadCard(b.CardsRoot(), id)
		if err != nil {
			lock.Release()
			findings = append(findings, Finding{Path: dir, Key: unreadableCardFinding(err), Detail: id})
			continue
		}
		wrote, err := b.WitnessDivergence(actor, now, card)
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
