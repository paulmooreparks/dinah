package bench

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestSortByArrivalReadsEachCardsJournalOnce drives dinah-630. A sort over n
// cards makes O(n log n) comparisons; a sort that reads each comparison's
// arrival fresh, the way sorting directly against ByArrival does, opens a
// card's journal on that order rather than once. SortByArrival is the fix:
// it reads each card's arrival once before it sorts, so a composition
// sorting n cards opens at most one journal per card.
//
// Arming: replacing SortByArrival's body with
// sort.SliceStable(cards, func(i, j int) bool { return ByArrival(cards[i], cards[j]) })
// reddens this test. It reported opening several cards' journals 3 times
// apiece over 8 cards (an O(n log n) comparison count re-reading both sides
// of the comparator each time), where this test wants at most once, before
// the fix was restored.
func TestSortByArrivalReadsEachCardsJournalOnce(t *testing.T) {
	root := newFixture(t)
	const n = 8
	for i := 2; i <= n; i++ {
		id := fmt.Sprintf("c%011d", i)
		write(t, filepath.Join(root, CardsDir, id, CardAnchor), cleanCard)
		journal := fmt.Sprintf(
			`{"ts":"2026-08-17T09:00:%02dZ","event":"created","actor":"alka","title":"A card","to":"b00000000001","to_title":"Only"}`+"\n",
			i)
		write(t, filepath.Join(root, CardsDir, id, JournalName), journal)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cards, err := opened.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != n {
		t.Fatalf("read %d cards, wanted %d", len(cards), n)
	}

	opens := map[string]int{}
	t.Cleanup(func() { AnchorReadObserver = nil })
	AnchorReadObserver = func(path string) {
		if filepath.Base(path) == JournalName {
			opens[filepath.Dir(path)]++
		}
	}
	SortByArrival(cards)
	AnchorReadObserver = nil

	if len(opens) != n {
		t.Fatalf("opened %d cards' journals, wanted all %d", len(opens), n)
	}
	for dir, count := range opens {
		if count > 1 {
			t.Errorf("%s: the sort opened the journal %d times, wanted at most once", dir, count)
		}
	}
	for i := 0; i+1 < len(cards); i++ {
		if cards[i].Arrival().After(cards[i+1].Arrival()) {
			t.Fatalf("cards[%d] arrived after cards[%d]: the sort did not order by arrival", i, i+1)
		}
	}
}

// TestEarliestArrivalReadsEachCardsJournalOnce drives dinah-630's second
// shared entry point. Library.primeInstructions and the CLI's own
// next-example both scan a caller's held cards for the earliest arrival by
// comparing every candidate against a running earliest; comparing through
// ByArrival re-reads the running earliest's journal on every comparison it
// stays ahead in. EarliestArrival reads each card's arrival once regardless
// of how many candidates it is compared against.
//
// Arming: replacing EarliestArrival's loop with one that calls
// ByArrival(c, earliest) instead of comparing cached arrivals reddens this
// test, since the earliest candidate's journal is then opened once per
// remaining card it is compared against rather than once overall.
func TestEarliestArrivalReadsEachCardsJournalOnce(t *testing.T) {
	root := newFixture(t)
	const n = 6
	for i := 2; i <= n; i++ {
		id := fmt.Sprintf("c%011d", i)
		write(t, filepath.Join(root, CardsDir, id, CardAnchor), cleanCard)
		journal := fmt.Sprintf(
			`{"ts":"2026-08-17T09:00:%02dZ","event":"created","actor":"alka","title":"A card","to":"b00000000001","to_title":"Only"}`+"\n",
			i)
		write(t, filepath.Join(root, CardsDir, id, JournalName), journal)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cards, err := opened.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != n {
		t.Fatalf("read %d cards, wanted %d", len(cards), n)
	}

	opens := map[string]int{}
	t.Cleanup(func() { AnchorReadObserver = nil })
	AnchorReadObserver = func(path string) {
		if filepath.Base(path) == JournalName {
			opens[filepath.Dir(path)]++
		}
	}
	earliest := EarliestArrival(cards)
	AnchorReadObserver = nil

	if earliest == nil || earliest.ID != "c00000000001" {
		t.Fatalf("EarliestArrival answered %v, wanted the card created first", earliest)
	}
	if len(opens) != n {
		t.Fatalf("opened %d cards' journals, wanted all %d", len(opens), n)
	}
	for dir, count := range opens {
		if count > 1 {
			t.Errorf("%s: EarliestArrival opened the journal %d times, wanted at most once", dir, count)
		}
	}
}
