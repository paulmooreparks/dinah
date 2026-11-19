package judge

import (
	"testing"
	"time"
)

// What became of a decision is kept beside its record: the first outcome
// stands, an outcome for a record that is not there is not kept, the
// outcomes of a kind are counted over a time with those that have none, and
// an outcome goes when its record is pruned.
func TestOutcomesAreKeptCountedAndPrunedWithTheirRecords(t *testing.T) {
	s, clock := openTest(t)
	at := *clock
	taken := add(t, s, at, Record{Kind: "reply-suggestion"})
	dropped := add(t, s, at, Record{Kind: "reply-suggestion"})
	add(t, s, at, Record{Kind: "reply-suggestion"})
	add(t, s, at, Record{Kind: "reply-suggestion", Used: UsedRule})
	other := add(t, s, at, Record{Kind: "chat-title"})
	for id, word := range map[int64]string{taken: "taken", dropped: "dropped", other: "taken"} {
		if err := s.SetOutcome(id, word); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetOutcome(taken, "dropped"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Outcome(taken); got != "taken" {
		t.Fatalf("the first outcome did not stand: %q", got)
	}
	if err := s.SetOutcome(99999, "taken"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Outcome(99999); got != "" {
		t.Fatalf("an outcome was kept for a record that is not there: %q", got)
	}
	if err := s.SetOutcome(0, "taken"); err == nil {
		t.Fatal("an outcome with no record was accepted")
	}
	counts, err := s.Outcomes("reply-suggestion", at.Add(-time.Hour), at.Add(time.Hour))
	if err != nil || counts["taken"] != 1 || counts["dropped"] != 1 || counts[""] != 1 || len(counts) != 3 {
		t.Fatalf("the counts are %v %v", counts, err)
	}
	// Past the retention the records go, and their outcomes with them.
	*clock = at.Add(DefaultRetention + time.Hour)
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM outcomes`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("%d outcomes outlived their records (%v)", left, err)
	}
}
