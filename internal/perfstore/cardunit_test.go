package perfstore_test

import (
	"io/fs"
	"path/filepath"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/perfstore"
)

// TestTheCardUnitLayoutGeneratesAStoreThatReadsBack drives the generator half
// of dinah-637/criteria/22. With the layout switched on, the small shape
// generated in the card-unit layout opens at the card-unit format, holds no
// comment.md and no item.md, checks clean apart from the cards the shape
// plants straight in Done, and carries in its cards' records every item and
// comment the shape names. The directories layout is refused while the
// layout is switched on, since it would stamp the store with the wrong
// format.
//
// It flips the process-global switch, so it does not run in parallel; the
// package's parallel tests wait until it has finished and put the switch
// back.
//
// Arming: dropping the card baseline from baselineCard leaves each card's
// replay starting from its created line, which states neither the card's
// levels nor its body, so check reports card.md as diverged from the journal.
func TestTheCardUnitLayoutGeneratesAStoreThatReadsBack(t *testing.T) {
	bench.EnableCardUnitForTest(t)
	if _, err := perfstore.GenerateLayout(t.TempDir(), perfstore.DefaultSeed, perfstore.SmallShape(), perfstore.LayoutDirectories); err == nil {
		t.Error("the directories layout was generated with the card-unit layout switched on")
	}
	shape := perfstore.SmallShape()
	store, err := perfstore.GenerateLayout(t.TempDir(), perfstore.DefaultSeed, shape, perfstore.LayoutCardUnit)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	b, err := bench.Open(store.Root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if b.Format != bench.CardUnitFormat {
		t.Fatalf("the store opened at format %d, wanted the card-unit format %d", b.Format, bench.CardUnitFormat)
	}
	members := 0
	err = filepath.WalkDir(store.Root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && (entry.Name() == "comment.md" || entry.Name() == "item.md") {
			members++
		}
		return err
	})
	if err != nil || members != 0 {
		t.Errorf("the card-unit store holds %d member files (%v), wanted none", members, err)
	}
	findings, err := b.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		if finding.Key == bench.FindingUnarchivedDone {
			continue
		}
		t.Errorf("finding %s %s at %s", finding.Key, finding.Detail, finding.Path)
	}
	cards, err := b.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	items, pending, comments := 0, 0, 0
	for _, card := range cards {
		record, err := b.LoadCardRecord(card)
		if err != nil {
			t.Fatalf("record of %s: %v", card.ID, err)
		}
		for _, item := range record.ItemsIn(bench.LiveHalf, "") {
			items++
			if item.State == bench.ItemPending {
				pending++
			}
			comments += len(record.CommentsOf(item.ID, bench.LiveHalf))
		}
		comments += len(record.CommentsOf("", bench.LiveHalf))
	}
	if items != shape.Items || pending != shape.PendingItems || comments != shape.ItemComments+shape.CardComments {
		t.Errorf("the records hold %d items, %d pending, and %d comments; the shape names %d, %d and %d",
			items, pending, comments, shape.Items, shape.PendingItems, shape.ItemComments+shape.CardComments)
	}
}
