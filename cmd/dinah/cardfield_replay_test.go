package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// TestTheJournalStatesEveryCardAfterEveryCommand drives dinah-637/criteria/8.
// The population sequence is replayed with the card-unit layout switched on,
// and after every command every card's journal is replayed twice: its fields
// by ReplayCardFields, which must value-equal its card.md, and its members by
// ReplayMembers, which must be exactly the comments and items show reports.
//
// The differential is how a writer that does not reproduce its own anchor
// write is found: one that sets a key no line of its records fails here at the
// command that wrote it.
//
// Arming: dropping the fields the created line carries reddens the fifth card
// from its creation, which is the sequence's one filing that names levels.
func TestTheJournalStatesEveryCardAfterEveryCommand(t *testing.T) {
	bench.EnableCardUnitForTest(t)
	commands, cards := 0, 0
	replayPopulationWith(t, func(line, root string) {
		commands++
		dir := benchDir(t, root)
		opened, err := bench.Open(dir)
		if err != nil {
			t.Fatalf("after %q: open: %v", line, err)
		}
		library := verb.New(opened, os.Getenv("DINAH_HOME"))
		for _, collection := range []string{opened.CardsRoot(), opened.ArchivedCardsRoot()} {
			ids, err := bench.ListIDs(collection)
			if err != nil {
				t.Fatalf("after %q: list %s: %v", line, collection, err)
			}
			for _, id := range ids {
				card, err := opened.LoadCardIn(collection, id)
				if err != nil {
					t.Fatalf("after %q: load %s: %v", line, id, err)
				}
				cards++
				events, _, err := bench.ReadJournal(card.JournalPath())
				if err != nil {
					t.Fatalf("after %q: journal of %s: %v", line, id, err)
				}
				fm, body, ok := bench.ReplayCardFields(events)
				if !ok {
					t.Errorf("after %q: the journal of %s states no start for its fields", line, card.Ref(opened.Slug))
					continue
				}
				text, err := os.ReadFile(filepath.Join(card.Dir, bench.CardAnchor))
				if err != nil {
					t.Fatalf("after %q: read %s: %v", line, id, err)
				}
				anchor, anchorBody := bench.ParseAnchor(bench.NormalizeNewlines(string(text)))
				if differ := bench.ProjectionDifferences(anchor, anchorBody, fm, body); len(differ) > 0 {
					t.Errorf("after %q: %s replays differently from its card.md on %v:\nreplayed %q\ncard.md  %q",
						line, card.Ref(opened.Slug), differ, fm.Render(body), string(text))
				}
				if collection != opened.CardsRoot() {
					continue
				}
				comments, items := bench.ReplayMembers(events)
				detail, _, _, _, err := library.Show(&verb.Request{Verb: "show", Actor: "sam", Card: card.Ref(opened.Slug), All: true})
				if err != nil || detail == nil {
					t.Fatalf("after %q: show %s: %v", line, card.Ref(opened.Slug), err)
				}
				var replayed, shown []string
				for id, comment := range comments {
					if comment.Holder == "" && !comment.Archived {
						replayed = append(replayed, "comment "+id)
					}
				}
				for id, item := range items {
					if !item.Archived {
						replayed = append(replayed, "item "+id)
					}
				}
				for _, comment := range detail.Comments {
					shown = append(shown, "comment "+comment.ID)
				}
				for _, item := range detail.Checklist {
					shown = append(shown, "item "+item.ID)
				}
				sort.Strings(replayed)
				sort.Strings(shown)
				if strings.Join(replayed, ",") != strings.Join(shown, ",") {
					t.Errorf("after %q: %s replays the members %v and show reports %v", line, card.Ref(opened.Slug), replayed, shown)
				}
			}
		}
	})
	t.Logf("%d commands replayed, %d card comparisons made", commands, cards)
	if commands < 80 || cards < commands {
		t.Errorf("the replay ran %d commands and made %d card comparisons, which is less than the sequence holds", commands, cards)
	}
}
