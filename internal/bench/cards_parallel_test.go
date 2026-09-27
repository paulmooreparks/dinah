package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// archivedFixture plants n archived cards below root, each carrying a
// journal but no anchor, on the terms an archived card's watch entry always
// carries: archive[i] is CardsDir/<id>, its journal at JournalName, and no
// anchor. It answers the identifiers in ascending order.
func archivedFixture(t *testing.T, root string, n int) []string {
	t.Helper()
	var ids []string
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("f%011d", i+1)
		ids = append(ids, id)
		write(t, filepath.Join(root, ArchiveDir, CardsDir, id, JournalName), "{}\n")
	}
	return ids
}

// TestWatchedEntitiesCachedReadsOnlyNewlyArchivedJournals drives dinah-620:
// a poll loop that keeps the same cache map across calls stats an archived
// card's journal once, on the call that first sees it, and reuses that entry
// on every later call whose listing still carries the identifier, so a
// workbench holding many archived cards costs one stat per archived card
// over the whole loop rather than one stat per archived card per poll.
//
// Arming: passing nil in place of cache on the second and third calls, or
// dropping watchArchived's cache lookup so it always takes the parallelRead
// path, reddens this test with every archived journal statted on every call.
func TestWatchedEntitiesCachedReadsOnlyNewlyArchivedJournals(t *testing.T) {
	root := newFixture(t)
	ids := archivedFixture(t, root, 20)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	cache := map[string]Watched{}
	statted := map[string]int{}
	var mu sync.Mutex
	t.Cleanup(func() { JournalStatObserver = nil })
	archiveRoot := opened.ArchivedCardsRoot()
	JournalStatObserver = func(path string) {
		if !strings.HasPrefix(filepath.Clean(path), archiveRoot) {
			return // the live half's own journals, not this test's concern
		}
		mu.Lock()
		defer mu.Unlock()
		statted[filepath.Clean(path)]++
	}

	if _, _, _, err := opened.WatchedEntitiesCached(cache); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(statted) != len(ids) {
		t.Fatalf("the first call statted %d journals, wanted one per archived card (%d)", len(statted), len(ids))
	}
	for _, count := range statted {
		if count != 1 {
			t.Errorf("a journal was statted %d times on the priming call, wanted once", count)
		}
	}

	// Second and third calls over the same, unchanged listing: no new stat.
	for pass := 2; pass <= 3; pass++ {
		if _, _, _, err := opened.WatchedEntitiesCached(cache); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
	}
	total := 0
	for _, count := range statted {
		total += count
	}
	if len(statted) != len(ids) {
		t.Fatalf("after two more calls over an unchanged archive, %d journals were ever statted, wanted still %d: a poll over an unchanged archive should stat nothing new", len(statted), len(ids))
	}
	if total != len(ids) {
		t.Fatalf("the three calls together statted %d times, wanted %d (once each, none repeated on the second and third pass over an unchanged archive)", total, len(ids))
	}

	// A newly archived card is statted once, on the call that first lists it,
	// and the rest of the cache is left alone.
	extra := fmt.Sprintf("f%011d", len(ids)+1)
	write(t, filepath.Join(opened.ArchivedCardsRoot(), extra, JournalName), "{}\n")
	if _, _, _, err := opened.WatchedEntitiesCached(cache); err != nil {
		t.Fatalf("call after a new archive: %v", err)
	}
	total = 0
	for _, count := range statted {
		total += count
	}
	if len(statted) != len(ids)+1 || total != len(ids)+1 {
		t.Fatalf("after a newly archived card, %d distinct journals were ever statted (%d stats total), wanted %d of each", len(statted), total, len(ids)+1)
	}
	if n := statted[filepath.Clean(filepath.Join(opened.ArchivedCardsRoot(), extra, JournalName))]; n != 1 {
		t.Errorf("the newly archived card's journal was statted %d times, wanted once", n)
	}
}

// TestWatchedEntitiesCachedDropsARestoredIdentifier drives dinah-620: an
// identifier the cache holds that the archive no longer lists, because the
// card was restored, is dropped from the cache rather than kept, so a later
// re-archiving of the same identifier is read fresh instead of served the
// value from before the restore.
func TestWatchedEntitiesCachedDropsARestoredIdentifier(t *testing.T) {
	root := newFixture(t)
	ids := archivedFixture(t, root, 3)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cache := map[string]Watched{}
	if _, _, _, err := opened.WatchedEntitiesCached(cache); err != nil {
		t.Fatalf("prime: %v", err)
	}
	if len(cache) != len(ids) {
		t.Fatalf("the cache holds %d entries after priming, wanted %d", len(cache), len(ids))
	}

	restored := ids[0]
	if err := os.RemoveAll(filepath.Join(opened.ArchivedCardsRoot(), restored)); err != nil {
		t.Fatalf("remove the restored card: %v", err)
	}
	if _, _, _, err := opened.WatchedEntitiesCached(cache); err != nil {
		t.Fatalf("call after a restore: %v", err)
	}
	if _, held := cache[restored]; held {
		t.Errorf("the cache still holds %s after it left the archive, wanted it dropped", restored)
	}
	if len(cache) != len(ids)-1 {
		t.Errorf("the cache holds %d entries after a restore, wanted %d", len(cache), len(ids)-1)
	}

	statted := map[string]int{}
	var mu sync.Mutex
	t.Cleanup(func() { JournalStatObserver = nil })
	JournalStatObserver = func(path string) {
		mu.Lock()
		defer mu.Unlock()
		statted[filepath.Clean(path)]++
	}
	if err := os.MkdirAll(filepath.Join(opened.ArchivedCardsRoot(), restored), 0o755); err != nil {
		t.Fatalf("re-create the archived directory: %v", err)
	}
	write(t, filepath.Join(root, ArchiveDir, CardsDir, restored, JournalName), "{}\n{}\n")
	if _, _, _, err := opened.WatchedEntitiesCached(cache); err != nil {
		t.Fatalf("call after a re-archive: %v", err)
	}
	if n := statted[filepath.Clean(filepath.Join(root, ArchiveDir, CardsDir, restored, JournalName))]; n != 1 {
		t.Errorf("the re-archived card's journal was statted %d times, wanted once, read fresh rather than served a value from before the restore", n)
	}
}
