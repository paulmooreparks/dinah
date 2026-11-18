package drafts

import (
	"errors"
	"testing"
)

// agentBatch is a draft with three suggestions of one proposal by the chat
// "harbor", and one by the chat "docks".
func agentBatch(t *testing.T) (*Store, Draft, []Suggestion, Suggestion) {
	t.Helper()
	base := "# Plan\n\nOne.\n\nTwo.\n\nThree.\n\nFour.\n"
	store, d := newDraft(t, base)
	d, mine, err := store.Propose(d.ID, base, "# Plan\n\nOne, better.\n\nTwo, better.\n\nThree, better.\n\nFour.\n", Proposal{Author: "Harbor", Owner: "harbor", Reason: "clearer"})
	if err != nil || len(mine) != 3 {
		t.Fatalf("proposed %+v %v", mine, err)
	}
	d, theirs, err := store.Propose(d.ID, base, "# Plan\n\nOne.\n\nTwo.\n\nThree.\n\nFour, theirs.\n", Proposal{Author: "Docks", Owner: "docks"})
	if err != nil || len(theirs) != 1 {
		t.Fatalf("proposed %+v %v", theirs, err)
	}
	return store, d, mine, theirs[0]
}

func ownedBy(owner string) func(Suggestion) bool {
	return func(s Suggestion) bool { return s.Owner == owner }
}

// An agent revises some of its own suggestions: their text and reason change,
// Edited stays false, and the rest of the batch and the draft's text are as
// they were.
func TestAnAgentRevisesItsOwnSuggestions(t *testing.T) {
	store, d, mine, theirs := agentBatch(t)
	text := d.Text
	reason := "clearer, and the second fixed"
	d, revised, err := store.ReviseOwn(d.ID, ownedBy("harbor"), []AgentRevision{{ID: mine[1].ID, After: "Two, fixed."}}, &reason)
	if err != nil || len(revised) != 1 {
		t.Fatalf("revised %+v %v", revised, err)
	}
	// The line break the paragraph ends with is kept although the agent left
	// it out.
	if r := revised[0]; r.After != "Two, fixed.\n" || r.Reason != reason || r.Edited || r.Before != mine[1].Before || r.Owner != "harbor" {
		t.Fatalf("revised suggestion %+v", r)
	}
	if d.Text != text || len(d.Suggestions) != 4 {
		t.Fatalf("draft after revising %q %d", d.Text, len(d.Suggestions))
	}
	for _, s := range d.Suggestions {
		switch s.ID {
		case mine[0].ID, mine[2].ID:
			if s.After != map[string]string{mine[0].ID: mine[0].After, mine[2].ID: mine[2].After}[s.ID] || s.Reason != "clearer" {
				t.Fatalf("an untouched suggestion changed: %+v", s)
			}
		case theirs.ID:
			if s.After != theirs.After {
				t.Fatalf("another chat's suggestion changed: %+v", s)
			}
		}
	}
	// Without a reason the reason stays.
	d, revised, err = store.ReviseOwn(d.ID, ownedBy("harbor"), []AgentRevision{{ID: mine[0].ID, After: "One, fixed.\n"}}, nil)
	if err != nil || revised[0].Reason != "clearer" || revised[0].After != "One, fixed.\n" {
		t.Fatalf("revised without a reason %+v %v", revised, err)
	}
	// Accepting the revised suggestion puts its new text in.
	d, err = store.Accept(d.ID, d.Revision, mine[1].ID)
	if err != nil || d.Text != "# Plan\n\nOne.\n\nTwo, fixed.\n\nThree.\n\nFour.\n" {
		t.Fatalf("accepted %q %v", d.Text, err)
	}
}

// An agent withdraws some of its own suggestions; the rest stay.
func TestAnAgentWithdrawsItsOwnSuggestions(t *testing.T) {
	store, d, mine, theirs := agentBatch(t)
	text := d.Text
	d, removed, err := store.WithdrawOwn(d.ID, ownedBy("harbor"), []string{mine[0].ID, mine[2].ID})
	if err != nil || removed != 2 || d.Text != text {
		t.Fatalf("withdrew %d %v", removed, err)
	}
	if len(d.Suggestions) != 2 || d.Suggestions[0].ID != mine[1].ID || d.Suggestions[1].ID != theirs.ID {
		t.Fatalf("left %+v", d.Suggestions)
	}
}

// Another chat's suggestion, one the person edited, an unknown one and one
// chosen twice are refused, and a request with any of them changes nothing.
func TestAnAgentsCorrectionIsRefusedAsAWhole(t *testing.T) {
	store, d, mine, theirs := agentBatch(t)
	d, err := store.Revise(d.ID, mine[2].ID, "Three, as the person wants it.\n")
	if err != nil {
		t.Fatal(err)
	}
	before := d.Suggestions
	cases := []struct {
		name string
		id   string
		want error
	}{
		{"another chat's", theirs.ID, ErrSuggestionNotOwned},
		{"edited by the person", mine[2].ID, ErrSuggestionEdited},
		{"unknown", "0000000000000000", ErrSuggestionGone},
		{"repeated", mine[0].ID, ErrSuggestionRepeated},
	}
	for _, c := range cases {
		ids := []string{mine[0].ID, c.id}
		changes := []AgentRevision{{ID: mine[0].ID, After: "One, changed.\n"}, {ID: c.id, After: "Changed.\n"}}
		var refused *RefusedError
		if _, _, err := store.ReviseOwn(d.ID, ownedBy("harbor"), changes, nil); !errors.Is(err, c.want) || !errors.As(err, &refused) || len(refused.Refused) != 1 || refused.Refused[0].ID != c.id {
			t.Fatalf("revising %s: %v", c.name, err)
		}
		if _, _, err := store.WithdrawOwn(d.ID, ownedBy("harbor"), ids); !errors.Is(err, c.want) {
			t.Fatalf("withdrawing %s: %v", c.name, err)
		}
		after, err := store.Get(d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(after.Suggestions) != len(before) {
			t.Fatalf("a refused request (%s) removed suggestions: %+v", c.name, after.Suggestions)
		}
		for i := range before {
			if after.Suggestions[i].After != before[i].After || after.Suggestions[i].Reason != before[i].Reason {
				t.Fatalf("a refused request (%s) changed %+v", c.name, after.Suggestions[i])
			}
		}
	}
}

// A stale suggestion cannot be revised, since the text it would replace is
// gone, but it can be withdrawn. A revision that proposes the old text is
// refused too.
func TestAStaleSuggestionIsWithdrawnNotRevised(t *testing.T) {
	store, d, mine, _ := agentBatch(t)
	d, err := store.Update(d.ID, d.Revision, "# Plan\n\nOne, rewritten by the person.\n\nTwo.\n\nThree.\n\nFour.\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	stale := -1
	for i, s := range d.Suggestions {
		if s.ID == mine[0].ID && s.Stale {
			stale = i
		}
	}
	if stale < 0 {
		t.Fatalf("the first suggestion is not stale: %+v", d.Suggestions)
	}
	if _, _, err := store.ReviseOwn(d.ID, ownedBy("harbor"), []AgentRevision{{ID: mine[0].ID, After: "One, again.\n"}}, nil); !errors.Is(err, ErrSuggestionStale) {
		t.Fatalf("revising a stale suggestion: %v", err)
	}
	if _, _, err := store.ReviseOwn(d.ID, ownedBy("harbor"), []AgentRevision{{ID: mine[1].ID, After: "Two."}}, nil); !errors.Is(err, ErrSuggestionUnchanged) {
		t.Fatalf("revising to the old text: %v", err)
	}
	if _, removed, err := store.WithdrawOwn(d.ID, ownedBy("harbor"), []string{mine[0].ID}); err != nil || removed != 1 {
		t.Fatalf("withdrawing a stale suggestion: %d %v", removed, err)
	}
}

// A revision written with Windows line endings is kept with the draft's.
func TestARevisionTakesTheDraftsLineEndings(t *testing.T) {
	store, d, mine, _ := agentBatch(t)
	_, revised, err := store.ReviseOwn(d.ID, ownedBy("harbor"), []AgentRevision{{ID: mine[0].ID, After: "One,\r\nin two lines.\r\n"}}, nil)
	if err != nil || revised[0].After != "One,\nin two lines.\n" {
		t.Fatalf("revised %+v %v", revised, err)
	}
}
