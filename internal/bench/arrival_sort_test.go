package bench

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

	// SortByArrival reads the arrivals on parallelRead's workers, so the
	// observer counts under a lock.
	opens := map[string]int{}
	var mu sync.Mutex
	t.Cleanup(func() { AnchorReadObserver = nil })
	AnchorReadObserver = func(path string) {
		if filepath.Base(path) == JournalName {
			mu.Lock()
			opens[filepath.Dir(path)]++
			mu.Unlock()
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

// TestArrivalReadsTheLinesArrivalFromReads answers dinah-637/questions/26,
// which moved Arrival onto a narrow read of the journal: only a line carrying
// created, moved or manual_correction is decoded, and then into the three
// members an arrival needs. The narrow read has to answer what ArrivalFrom
// answers over the whole parse, for every column the card could stand in, on
// a journal that holds each kind of arrival line and lines that carry those
// words in text without being one: a comment whose text names moved, and a
// cited item whose target does. The comparison runs over each column the
// journal names and one it does not, so a line read wrongly moves some
// column's answer.
//
// Arming: leaving manual_correction out of the narrow read's events moves the
// answer for Review, which only a correction names at its last stamp.
func TestArrivalReadsTheLinesArrivalFromReads(t *testing.T) {
	root := newFixture(t)
	id := "c00000000002"
	lines := []string{
		`{"ts":"2026-08-17T09:00:00Z","event":"created","actor":{"name":"alka"},"title":"A card","to":"b00000000001","to_title":"Only"}`,
		`{"ts":"2026-08-17T09:01:00Z","event":"moved","actor":{"name":"alka"},"from":"b00000000001","to":"doing","to_title":"Doing"}`,
		`{"ts":"2026-08-17T09:02:00Z","event":"commented","actor":{"name":"alka"},"comment":"0123456789ab","ordinal":1,"text":"the card moved to review, \"event\":\"moved\",\"to\":\"review\""}`,
		`{"ts":"2026-08-17T09:03:00Z","event":"item_cited","actor":{"name":"alka"},"item":"0123456789ac","scheme":"test","target":"manual_correction moved created"}`,
		`{"ts":"2026-08-17T09:04:00Z","event":"moved","actor":{"name":"alka"},"from":"doing","to":"review","to_title":"Review"}`,
		`{"ts":"2026-08-17T09:05:00Z","event":"moved","actor":{"name":"alka"},"from":"review","to":"doing","to_title":"Doing"}`,
		`{"ts":"2026-08-17T09:06:00Z","event":"manual_correction","actor":{"name":"alka"},"from":"doing","to":"review","to_title":"Review"}`,
	}
	write(t, filepath.Join(root, CardsDir, id, CardAnchor), cleanCard)
	write(t, filepath.Join(root, CardsDir, id, JournalName), strings.Join(lines, "\n")+"\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	card, err := opened.LoadCardIn(opened.CardsRoot(), id)
	if err != nil {
		t.Fatalf("load %s: %v", id, err)
	}
	events, _, err := ReadJournal(card.JournalPath())
	if err != nil || len(events) != len(lines) {
		t.Fatalf("read the journal: %d events, %v", len(events), err)
	}
	for _, column := range []string{"b00000000001", "doing", "review", "nowhere"} {
		card.Column = column
		if got, want := card.Arrival(), ArrivalFrom(events, column); !got.Equal(want) {
			t.Errorf("standing in %s the card arrived at %s, and ArrivalFrom reads %s", column, got, want)
		}
	}
	card.Column = "review"
	if got := card.Arrival(); got.Format(time.RFC3339) != "2026-08-17T09:06:00Z" {
		t.Errorf("standing in review the card arrived at %s, wanted the correction's stamp", got.Format(time.RFC3339))
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
