package bench

import (
	"errors"
	"path/filepath"
	"testing"

	"dinah/internal/contract"
)

// loopBenchDefinition is a three-column workbench: an ordinary station, a
// second station ahead of it, and a done column at the end. The middle column
// is where every loop_limit below is declared, so a departure to the first
// column is regressive and a departure to the third is not.
const loopBenchDefinition = `---
format: 1
profile: dinah-core/0.7
title: Fixture
slug: fx
operator: alka
columns:
  - b00000000001
  - b00000000002
  - b00000000003
---
Standing text.
`

// loopCard stands at the middle column, which is the column every test below
// declares a limit on.
const loopCard = `---
title: A card
number: 1
column: b00000000002
state: ready
---
Framing.
`

// loopFixture writes the three-column workbench with the middle column
// carrying the frontmatter lines the caller asks for, and returns its root.
// The card's journal is left holding only its created event, so each test
// appends exactly the history it is about.
func loopFixture(t *testing.T, middleExtra string) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, WorkbenchAnchor), loopBenchDefinition)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
		"---\ntitle: Editing\nslug: editing\nkind: work\n---\nEditing text.\n")
	write(t, filepath.Join(root, ColumnsDir, "b00000000002", ColumnAnchor),
		"---\ntitle: Review\nslug: review\nkind: work\n"+middleExtra+"---\nReview text.\n")
	write(t, filepath.Join(root, ColumnsDir, "b00000000003", ColumnAnchor),
		"---\ntitle: Finished\nslug: finished\nkind: done\n---\nFinished text.\n")
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), loopCard)
	write(t, filepath.Join(root, CardsDir, "c00000000001", JournalName),
		`{"ts":"2026-08-17T09:00:00Z","event":"created","actor":"alka","title":"A card","to":"b00000000002","to_title":"Review"}`+"\n")
	return root
}

// TestAColumnDeclaresHowOftenACardMayComeBack is dinah-364 AC-1. The three
// cases are the three a declaration can be in, and the malformed one is read
// against wip_limit's own answer to the same shape rather than against a
// number written down here, so the two limits cannot drift into disagreeing
// about what a non-integer means.
func TestAColumnDeclaresHowOftenACardMayComeBack(t *testing.T) {
	t.Run("a declared limit parses into the field", func(t *testing.T) {
		opened, err := Open(loopFixture(t, "loop_limit: 3\n"))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if got := opened.Column("b00000000002").LoopLimit; got != 3 {
			t.Errorf("wanted the declared 3, got %d", got)
		}
	})

	t.Run("an absent declaration reads as unbounded", func(t *testing.T) {
		opened, err := Open(loopFixture(t, ""))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if got := opened.Column("b00000000002").LoopLimit; got != 0 {
			t.Errorf("wanted 0 for a column declaring nothing, got %d", got)
		}
	})

	t.Run("a non-integer value is refused malformed, as wip_limit's is", func(t *testing.T) {
		for _, key := range []string{"loop_limit", "wip_limit"} {
			_, err := Open(loopFixture(t, key+": three\n"))
			if err == nil {
				t.Fatalf("%s: wanted a refusal on open, got none", key)
			}
			var refusal *contract.Refusal
			if !errors.As(err, &refusal) {
				t.Fatalf("%s: wanted a refusal, got %v", key, err)
			}
			if refusal.Name != contract.Malformed {
				t.Errorf("%s: wanted %s, got %s", key, contract.Malformed, refusal.Name)
			}
		}
	})

	t.Run("a limit on an intake or a done column parses", func(t *testing.T) {
		// dinah-364 AC-9, the parse half. No card is ever held at either
		// kind, so the declaration is inert rather than refused, which is
		// how a wip_limit on an intake column already behaves.
		root := loopFixture(t, "")
		write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
			"---\ntitle: Editing\nslug: editing\nkind: intake\nloop_limit: 1\n---\nEditing text.\n")
		write(t, filepath.Join(root, ColumnsDir, "b00000000003", ColumnAnchor),
			"---\ntitle: Finished\nslug: finished\nkind: done\nloop_limit: 1\n---\nFinished text.\n")
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if got := opened.Column("b00000000001").LoopLimit; got != 1 {
			t.Errorf("intake: wanted 1, got %d", got)
		}
		if got := opened.Column("b00000000003").LoopLimit; got != 1 {
			t.Errorf("done: wanted 1, got %d", got)
		}
	})
}

// TestRegressiveDeparturesCountsOnlyTheDeparturesTheLimitIsAbout is dinah-364
// AC-2. Each excluded shape gets a case of its own, because a count that reads
// high or low turns the gate into a wrong answer rather than a missing one,
// and a single mixed slice would let one exclusion cover for another's
// absence.
//
// Every case runs against the same fixture and the same declaring column, so
// the only thing changing between them is the event under test.
func TestRegressiveDeparturesCountsOnlyTheDeparturesTheLimitIsAbout(t *testing.T) {
	opened, err := Open(loopFixture(t, "loop_limit: 2\n"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	const declaring = "b00000000002"

	cases := []struct {
		name   string
		events []Event
		want   int
		why    string
	}{
		{
			name: "a backward move out of the column counts",
			events: []Event{
				{Event: contract.EventMoved, From: declaring, To: "b00000000001"},
			},
			want: 1,
			why:  "the motivating case: a review station sending work back",
		},
		{
			name: "two backward moves count twice",
			events: []Event{
				{Event: contract.EventMoved, From: declaring, To: "b00000000001"},
				{Event: contract.EventMoved, From: "b00000000001", To: declaring},
				{Event: contract.EventMoved, From: declaring, To: "b00000000001"},
			},
			want: 2,
			why:  "the return leg is forward and only the two departures count",
		},
		{
			name: "a forward move out of the column does not count",
			events: []Event{
				{Event: contract.EventMoved, From: declaring, To: "b00000000003"},
			},
			want: 0,
			why:  "a card finishing is not a card coming back",
		},
		{
			name: "a move into a done column does not count",
			events: []Event{
				{Event: contract.EventMoved, From: declaring, To: "b00000000003"},
			},
			want: 0,
			why: "this fixture's done column also stands ahead, so the case " +
				"that isolates the kind from the position is " +
				"TestRegressiveDeparturesExcludesADoneColumnStandingEarlier",
		},
		{
			name: "a move out of a different column does not count",
			events: []Event{
				{Event: contract.EventMoved, From: "b00000000003", To: "b00000000001"},
			},
			want: 0,
			why:  "the count is per declaring column, not per workbench",
		},
		{
			name: "a manual_correction does not count",
			events: []Event{
				{Event: contract.EventManualCorrection, From: declaring, To: "b00000000001"},
			},
			want: 0,
			why:  "a repair is not a transition anybody chose",
		},
		{
			name: "a move whose destination is no longer declared does not count",
			events: []Event{
				{Event: contract.EventMoved, From: declaring, To: "b00000000099"},
			},
			want: 0,
			why:  "there is no current position left to compare against",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := opened.RegressiveDepartures(c.events, declaring); got != c.want {
				t.Errorf("wanted %d, got %d: %s", c.want, got, c.why)
			}
		})
	}
}

// TestRegressiveDeparturesExcludesADoneColumnStandingEarlier is the other half
// of the done-column exclusion, and it is the half a fixture whose done column
// stands last cannot reach. The column here stands before the declaring one,
// so position alone would count the departure and only the kind stops it.
func TestRegressiveDeparturesExcludesADoneColumnStandingEarlier(t *testing.T) {
	root := loopFixture(t, "loop_limit: 2\n")
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
		"---\ntitle: Editing\nslug: editing\nkind: done\n---\nEditing text.\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	events := []Event{{Event: contract.EventMoved, From: "b00000000002", To: "b00000000001"}}
	if got := opened.RegressiveDepartures(events, "b00000000002"); got != 0 {
		t.Errorf("wanted a done destination excluded on its kind, got %d", got)
	}
}

// TestCheckReportsACardAtItsColumnsLoopLimit is dinah-364 AC-5. The
// limit-minus-one case is the false-failure guard: an implementation
// reporting every card at a declaring column would pass the positive case on
// its own.
func TestCheckReportsACardAtItsColumnsLoopLimit(t *testing.T) {
	// One departure back to the first column, which is the history all three
	// cases below are read against.
	const oneDeparture = `{"ts":"2026-08-17T10:00:00Z","event":"moved","actor":"alka","from":"b00000000002","from_title":"Review","to":"b00000000001","to_title":"Editing"}
{"ts":"2026-08-17T11:00:00Z","event":"moved","actor":"alka","from":"b00000000001","from_title":"Editing","to":"b00000000002","to_title":"Review"}
`

	t.Run("a count that has reached the limit is reported", func(t *testing.T) {
		root := loopFixture(t, "loop_limit: 1\n")
		appendText(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), oneDeparture)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		assertFindingNames(t, opened, FindingAtLoopLimit, "review")
	})

	t.Run("a count one short of the limit is not reported", func(t *testing.T) {
		root := loopFixture(t, "loop_limit: 2\n")
		appendText(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), oneDeparture)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		assertNoLoopFinding(t, opened, "a card one departure short of its limit")
	})

	t.Run("a column declaring no limit reports nothing", func(t *testing.T) {
		root := loopFixture(t, "")
		appendText(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), oneDeparture)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		assertNoLoopFinding(t, opened, "a card at a column declaring nothing")
	})
}

// assertNoLoopFinding fails when check reports the loop finding at all.
func assertNoLoopFinding(t *testing.T, b *Bench, what string) {
	t.Helper()
	findings, err := b.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		if finding.Key == FindingAtLoopLimit {
			t.Errorf("%s was reported: %+v", what, finding)
		}
	}
}
