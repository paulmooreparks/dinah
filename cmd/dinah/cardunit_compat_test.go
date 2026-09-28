package main

import (
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// cardUnitWantedEvents is the shape-reach table of the card-unit layout: for
// every event spec section 3.3 of dinah-637 lists, the members that section
// lists as always present, which a replay of the population sequence against
// a store at the card-unit format has to write. wantedEvents holds the older
// layout's union, and this table holds what the journal carries once it is
// the store of the comments and items: the text, the ordinal and the
// locators the replay reads.
var cardUnitWantedEvents = map[string][]string{
	contract.EventCommented:          {"comment", "ordinal"},
	contract.EventCommentUpdated:     {"note", "field"},
	contract.EventDivergenceAccepted: {"comment", "note"},
	contract.EventArchived:           {"note", "comment", "item"},
	contract.EventRestored:           {"note", "comment", "item"},
	contract.EventDeleted:            {"note", "comment", "item", "kind", "title"},
	contract.EventItemFiled:          {"item", "kind", "ordinal", "text"},
	contract.EventItemUpdated:        {"note", "field", "text"},
	contract.EventItemCited:          {"item", "scheme", "target"},
	contract.EventItemResolved:       {"item", "from", "resolution"},
	contract.EventItemVerified:       {"item", "from", "resolution"},
	contract.EventItemFailed:         {"item", "from", "resolution"},
	contract.EventItemWaived:         {"item", "from", "to", "resolution"},
	contract.EventItemWithdrawn:      {"item", "from", "to", "resolution"},
	contract.EventItemReopened:       {"item", "from"},
	contract.EventRedacted:           {"kind", "lines", "comment"},
}

// TestReplayingThePopulationSequenceInTheCardUnitLayout drives
// dinah-637/criteria/5's switch-on half. With the layout switched on the
// population sequence is replayed against a store created at the card-unit
// format, then two acts it holds no line for are run beside it: the deletion
// of an item, whose line the frozen sample of the older layout cannot carry,
// and one dinah redact --yes, which refuses the older layout. The journals
// then carry every event of the table above with every member it names, and
// a line carrying redacted.
//
// The switch-off half is the sample alarm and the shape-reach test beside it,
// which replay the same sequence with the layout switched off and pass
// unchanged.
//
// Arming: dropping the ordinal CompleteMemberLine sets on a commented line
// leaves the commented row short of ordinal, which the table catches.
func TestReplayingThePopulationSequenceInTheCardUnitLayout(t *testing.T) {
	bench.EnableCardUnitForTest(t)
	root := replayPopulation(t)
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("open the replayed store: %v", err)
	}
	if !opened.CardUnit() {
		t.Fatalf("the replayed store declares format %d, wanted the card-unit format %d", opened.Format, bench.CardUnitFormat)
	}
	for _, argv := range [][]string{
		{"delete", "sample-5/d/1", "--yes"},
		{"redact", "sample-3/comments/1", "--yes"},
	} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Fatalf("%v: exit %d\n%s%s", argv, got.code, got.out, got.errw)
		}
	}
	reached := readShape(t, root).members
	for event, members := range cardUnitWantedEvents {
		if reached[event] == nil {
			t.Errorf("the card-unit replay wrote no %s event", event)
			continue
		}
		for _, member := range members {
			if !reached[event][member] {
				t.Errorf("the card-unit replay wrote no %s member on the %s event", member, event)
			}
		}
	}
	marked := false
	for _, members := range reached {
		marked = marked || members["redacted"]
	}
	if !marked {
		t.Error("the card-unit replay rewrote no line marked redacted")
	}
}
