package verb

import (
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestChangesCarryATextAndNoMigrationLine drives the rest of
// dinah-637/criteria/20 in the card-unit layout. A changes answer spanning a
// comment write carries the comment's text on its commented line, and no
// answer carries an item_baseline, comment_baseline, card_baseline or
// storage_migrated line, even where such lines stand after the cursor.
//
// Arming: dropping the migrationLines test from eventsFrom lets the planted
// baselines through, which the second assertion catches.
func TestChangesCarryATextAndNoMigrationLine(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.add("A card whose history is watched")
	minted, err := h.library.Changes(&Request{Verb: "changes", Actor: "alka"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	h.comment(card, "The words the watcher should see.")
	h.file(card, "decision", "A decision the baselines restate.")

	// The three card baselines a migration would write for this card, each
	// restating what the store already holds, so the record reads as before.
	now := "2026-08-17T09:00:00Z"
	var baselines []bench.Event
	for _, ev := range h.events(card) {
		switch ev.Event {
		case contract.EventCommented:
			baselines = append(baselines, bench.Event{Event: contract.EventCommentBaseline, Comment: ev.Comment, Ordinal: ev.Ordinal, Text: ev.Text, Written: ev.TS, Author: ev.Actor.Name})
		case contract.EventItemFiled:
			baselines = append(baselines, bench.Event{Event: contract.EventItemBaseline, Item: ev.Item, Kind: ev.Kind, Ordinal: ev.Ordinal, State: bench.ItemPending, Text: ev.Text, Written: ev.TS})
		}
	}
	anchor, err := bench.ReadText(h.card(card).AnchorPath())
	if err != nil {
		t.Fatalf("read card.md: %v", err)
	}
	baselines = append(baselines, bench.Event{Event: contract.EventCardBaseline, Text: anchor})
	journal := h.card(card).JournalPath()
	lock, err := bench.Acquire(h.card(card).Dir, "alka", now)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	for _, line := range baselines {
		line.TS, line.Actor = now, bench.NamedActor("alka")
		if err := bench.AppendEvent(lock, journal, line); err != nil {
			t.Fatalf("plant %s: %v", line.Event, err)
		}
	}
	lock.Release()
	benchLock, err := bench.Acquire(h.root, "alka", now)
	if err != nil {
		t.Fatalf("lock the workbench: %v", err)
	}
	if err := bench.AppendEvent(benchLock, h.library.Bench.JournalPath(), bench.Event{TS: now, Event: contract.EventStorageMigrated, Actor: bench.NamedActor("alka")}); err != nil {
		t.Fatalf("plant %s: %v", contract.EventStorageMigrated, err)
	}
	benchLock.Release()
	h.reopen()

	set, err := h.library.Changes(&Request{Verb: "changes", Actor: "alka", Since: minted.Cursor})
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	carried := false
	for _, ev := range set.Events {
		if ev.Event.Event == contract.EventCommented && ev.Event.Text == "The words the watcher should see." {
			carried = true
		}
		if migrationLines[ev.Event.Event] {
			t.Errorf("the answer carries a %s line, which restates the store rather than recording an act", ev.Event.Event)
		}
	}
	if !carried {
		t.Errorf("the answer carries no commented line with the comment's text: %+v", set.Events)
	}
}
