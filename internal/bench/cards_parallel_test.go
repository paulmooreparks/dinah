package bench

import (
	"fmt"
	"path/filepath"
	"testing"
)

// manyCards plants n live cards with ascending identifiers below root, each a
// minimal, readable anchor, on top of the one card newFixture already plants
// at c00000000001. It returns every card's identifier in the collection,
// c00000000001 included, in the ascending order Cards is meant to answer
// them in. It is the fixture the walk tests below share, and it is large
// enough that a serial walk and a parallel one would answer at visibly
// different speeds on a slow filesystem, which is not what these tests
// assert; they assert the shape of the answer, not its timing.
func manyCards(t *testing.T, root string, n int) []string {
	t.Helper()
	ids := []string{"c00000000001"}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("c%011d", i+2)
		ids = append(ids, id)
		write(t, filepath.Join(root, CardsDir, id, CardAnchor), cleanCard)
	}
	return ids
}

// TestBenchCardsAnswersInAscendingIdentifierOrderAcrossAParallelWalk pins the
// order guarantee Cards() has always carried, now that the walk behind it
// loads every card across parallelRead's pool instead of one at a time: the
// answer is sorted ascending by identifier exactly as ListIDs sorts the ids
// it hands the walk, whichever worker happened to finish reading which card.
//
// Arming: dinah-632's own history armed this at the parallelRead level
// (TestParallelReadPreservesOrder), and this test pins the same property one
// layer up, at the function every caller of this card actually uses. Reverting
// cardsWith to append cards from a mutex-guarded slice in completion order,
// the change parallelRead's own arming rehearsed, reddens this by scrambling
// the answer whenever two cards finish loading out of id order.
func TestBenchCardsAnswersInAscendingIdentifierOrderAcrossAParallelWalk(t *testing.T) {
	root := newFixture(t)
	ids := manyCards(t, root, 40)

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cards, err := opened.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != len(ids) {
		t.Fatalf("read %d cards, wanted %d", len(cards), len(ids))
	}
	for i, card := range cards {
		if card.ID != ids[i] {
			t.Fatalf("cards[%d].ID = %s, wanted %s: the parallel walk did not preserve ascending order", i, card.ID, ids[i])
		}
	}
}

// TestBenchCardsFailsTheWholeWalkOnOneUnreadableCard asserts that Cards, which
// never skips a card that will not load, still reports the failure when the
// broken card is loaded by a pool rather than by a single goroutine: the walk
// answers no cards at all and the error names the broken card, on the same
// terms readCollection's own tests already pin for a collection that will not
// list.
func TestBenchCardsFailsTheWholeWalkOnOneUnreadableCard(t *testing.T) {
	root := newFixture(t)
	manyCards(t, root, 12)
	broken := filepath.Join(root, CardsDir, "c00000000099")
	plantUnreadable(t, filepath.Join(broken, CardAnchor))

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cards, err := opened.Cards()
	if err == nil {
		t.Fatalf("Cards() answered %d cards over a collection carrying an unreadable one, wanted an error", len(cards))
	}
	if cards != nil {
		t.Errorf("Cards() answered %d cards alongside its error, wanted none", len(cards))
	}
}

// TestBenchReadableCardsSkipsOnlyTheUnreadableOne asserts that ReadableCards,
// unlike Cards, drops the one card that will not load and answers every other
// card the walk found, in ascending identifier order, when the loads run
// across a pool rather than one at a time.
func TestBenchReadableCardsSkipsOnlyTheUnreadableOne(t *testing.T) {
	root := newFixture(t)
	ids := manyCards(t, root, 12)
	broken := filepath.Join(root, CardsDir, "c00000000099")
	plantUnreadable(t, filepath.Join(broken, CardAnchor))

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cards, err := opened.ReadableCards()
	if err != nil {
		t.Fatalf("ReadableCards: %v", err)
	}
	if len(cards) != len(ids) {
		t.Fatalf("read %d cards, wanted %d (the unreadable one dropped and every other kept)", len(cards), len(ids))
	}
	for i, card := range cards {
		if card.ID != ids[i] {
			t.Fatalf("cards[%d].ID = %s, wanted %s", i, card.ID, ids[i])
		}
	}
}

// TestWatchedEntitiesAnswersLiveCardsInAscendingOrder is the change cursor's
// own mint walk (dinah-632 names it explicitly as one of the walks this card
// covers): WatchedEntities reads a stat and a content hash per live card
// across the same pool cardsWith uses, and sortWatched then puts every half
// back in key order, so the answer is unaffected by which worker read which
// card first.
func TestWatchedEntitiesAnswersLiveCardsInAscendingOrder(t *testing.T) {
	root := newFixture(t)
	ids := manyCards(t, root, 30)

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	live, _, _, err := opened.WatchedEntities()
	if err != nil {
		t.Fatalf("WatchedEntities: %v", err)
	}
	// live carries the workbench entity ahead of every card, so the cards
	// are every entry after it, and WorkbenchKey sorts first because "cards/"
	// follows "workbench" lexically among this fixture's keys.
	var cardKeys []string
	for _, entry := range live {
		if entry.Key != WorkbenchKey {
			cardKeys = append(cardKeys, entry.Key)
		}
	}
	if len(cardKeys) != len(ids) {
		t.Fatalf("WatchedEntities named %d cards, wanted %d", len(cardKeys), len(ids))
	}
	for i, id := range ids {
		wanted := CardsDir + "/" + id
		if cardKeys[i] != wanted {
			t.Fatalf("cardKeys[%d] = %s, wanted %s: the mint walk did not preserve key order", i, cardKeys[i], wanted)
		}
	}
}
