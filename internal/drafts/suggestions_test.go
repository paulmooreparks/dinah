package drafts

import (
	"errors"
	"strings"
	"testing"
)

func newDraft(t *testing.T, text string) (*Store, Draft) {
	t.Helper()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d, err := store.Create(Origin{Kind: OriginNew}, Baseline{}, "Plan", &text)
	if err != nil {
		t.Fatal(err)
	}
	return store, d
}

// A proposal becomes one suggestion per changed paragraph, which the person
// accepts or rejects; the text changes only on acceptance.
func TestProposeAcceptReject(t *testing.T) {
	base := "# Plan\n\nFirst paragraph.\n\nSecond paragraph.\n\nThird paragraph.\n"
	store, d := newDraft(t, base)
	proposed := "# Plan\n\nFirst paragraph, improved.\n\nSecond paragraph.\n\nThird paragraph.\n\nA new fourth.\n"
	d, added, err := store.Propose(d.ID, base, proposed, Proposal{Author: "run 1, rework", Reason: "the review asked for it"})
	if err != nil || len(added) != 2 || d.Text != base || len(d.Suggestions) != 2 {
		t.Fatalf("proposed %+v %v", added, err)
	}
	first, fourth := added[0], added[1]
	if first.Before != "First paragraph.\n" || first.After != "First paragraph, improved.\n" || d.Text[first.Start:first.End] != first.Before || first.Group != fourth.Group {
		t.Fatalf("first %+v", first)
	}
	d, err = store.Accept(d.ID, d.Revision, first.ID)
	if err != nil || !strings.Contains(d.Text, "First paragraph, improved.") || len(d.Suggestions) != 1 {
		t.Fatalf("accept %q %v", d.Text, err)
	}
	// The other suggestion was found again after the text moved.
	if s := d.Suggestions[0]; s.Stale || d.Text[s.Start:s.End] != s.Before {
		t.Fatalf("the remaining suggestion lost its place: %+v", s)
	}
	d, err = store.Reject(d.ID, fourth.ID)
	if err != nil || len(d.Suggestions) != 0 || strings.Contains(d.Text, "fourth") {
		t.Fatalf("reject %+v %v", d, err)
	}
	if _, err := store.Reject(d.ID, fourth.ID); !errors.Is(err, ErrSuggestionGone) {
		t.Fatalf("rejecting twice: %v", err)
	}
}

// The person edits what a suggestion proposes before deciding: the edited
// text is what acceptance puts in, the draft's text is untouched until then,
// and the suggestion says it was edited.
func TestReviseASuggestion(t *testing.T) {
	base := "# Plan\n\nFirst paragraph.\n\nSecond paragraph.\n"
	store, d := newDraft(t, base)
	d, added, err := store.Propose(d.ID, base, "# Plan\n\nFirst paragraph, improved.\n\nSecond paragraph.\n", Proposal{Author: "Draft", Reason: "clearer"})
	if err != nil || len(added) != 1 {
		t.Fatalf("proposed %+v %v", added, err)
	}
	d, err = store.Revise(d.ID, added[0].ID, "First paragraph, as I want it.\n")
	if err != nil || d.Text != base || len(d.Suggestions) != 1 {
		t.Fatalf("revise %+v %v", d, err)
	}
	if s := d.Suggestions[0]; s.After != "First paragraph, as I want it.\n" || !s.Edited || s.Author != "Draft" || s.Reason != "clearer" || s.Before != "First paragraph.\n" {
		t.Fatalf("revised suggestion %+v", s)
	}
	d, err = store.Accept(d.ID, d.Revision, added[0].ID)
	if err != nil || d.Text != "# Plan\n\nFirst paragraph, as I want it.\n\nSecond paragraph.\n" {
		t.Fatalf("accepted %q %v", d.Text, err)
	}
	if _, err := store.Revise(d.ID, added[0].ID, "x"); !errors.Is(err, ErrSuggestionGone) {
		t.Fatalf("revising a decided suggestion: %v", err)
	}
}

// A proposer that writes its texts with Windows line endings, as PowerShell
// does, still has its changes placed in a draft with LF line endings, and
// the change it adds has the draft's line endings.
func TestProposeWithOtherLineEndings(t *testing.T) {
	text := "# One\n\nFirst.\n\n# Two\n\nSecond.\n"
	store, d := newDraft(t, text)
	crlf := func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
	d, added, err := store.Propose(d.ID, crlf(text), crlf("# One\n\nFirst, changed.\n\n# Two\n\nSecond.\n"), Proposal{Author: "writer"})
	if err != nil || len(added) != 1 || added[0].Stale {
		t.Fatalf("proposed %+v %v", added, err)
	}
	d, err = store.Accept(d.ID, d.Revision, added[0].ID)
	if err != nil || d.Text != "# One\n\nFirst, changed.\n\n# Two\n\nSecond.\n" {
		t.Fatalf("accepted %q %v", d.Text, err)
	}
}

// The same proposal made twice adds its changes once, and a placed change
// takes over from the same change kept earlier because it was not placed.
func TestProposeTwiceAddsOnce(t *testing.T) {
	text := "# One\n\nFirst.\n\n# Two\n\nSecond.\n"
	after := "# One\n\nFirst, changed.\n\n# Two\n\nSecond, changed.\n"
	store, d := newDraft(t, text)
	crlf := func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
	// What an earlier build kept from texts with Windows line endings.
	d, err := store.changeSuggestions(d.ID, func(d *Draft) error {
		d.Suggestions = append(d.Suggestions, Suggestion{ID: "old", Group: "g0", Stale: true, Before: crlf("First.\n"), After: crlf("First, changed.\n")})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	d, first, err := store.Propose(d.ID, text, after, Proposal{Author: "writer"})
	if err != nil || len(first) != 2 {
		t.Fatalf("first proposal %+v %v", first, err)
	}
	d, again, err := store.Propose(d.ID, text, after, Proposal{Author: "writer"})
	if err != nil || len(again) != 0 || len(d.Suggestions) != 2 {
		t.Fatalf("the second proposal added %d, and the draft has %d: %+v %v", len(again), len(d.Suggestions), d.Suggestions, err)
	}
	for _, s := range d.Suggestions {
		if s.Stale || s.ID == "old" {
			t.Fatalf("the unplaced earlier copy is still there: %+v", d.Suggestions)
		}
	}
}

// A proposal for a passage that ends with line breaks may leave them off,
// and a passage that starts at its heading is placed in its own section
// even when its body is text that other sections repeat.
func TestProposePassageOfASection(t *testing.T) {
	text := "# One\n\nTBC\n\n# Two\n\nTBC\n\n# Three\n\nTBC\n"
	store, d := newDraft(t, text)
	d, added, err := store.Propose(d.ID, "# Two\n\nTBC\n\n", "# Two\n\nThe second section.", Proposal{Author: "writer"})
	if err != nil || len(added) != 1 || added[0].Stale {
		t.Fatalf("proposed %+v %v", added, err)
	}
	d, err = store.Accept(d.ID, d.Revision, added[0].ID)
	if err != nil || d.Text != "# One\n\nTBC\n\n# Two\n\nThe second section.\n\n# Three\n\nTBC\n" {
		t.Fatalf("accepted %q %v", d.Text, err)
	}
}

// A whole proposal is accepted or rejected at once.
func TestProposalGroups(t *testing.T) {
	base := "# Plan\n\nOne.\n\nTwo.\n\nThree.\n"
	store, d := newDraft(t, base)
	proposed := "# Plan, revised\n\nOne.\n\nTwo, revised.\n\nThree.\n\nFour.\n"
	d, added, _ := store.Propose(d.ID, base, proposed, Proposal{Author: "agent"})
	if len(added) != 3 {
		t.Fatalf("suggestions %+v", added)
	}
	other := "# Plan\n\nOne, other.\n\nTwo.\n\nThree.\n"
	d, kept, _ := store.Propose(d.ID, base, other, Proposal{Author: "someone else"})
	spans := d.Spans()
	if len(spans) != 4 {
		t.Fatalf("spans %v", spans)
	}
	d, err := store.AcceptGroup(d.ID, d.Revision, added[0].Group)
	if err != nil || d.Text != proposed || len(d.Suggestions) != 1 || d.Suggestions[0].ID != kept[0].ID {
		t.Fatalf("accept the proposal %q %+v %v", d.Text, d.Suggestions, err)
	}
	if s := d.Suggestions[0]; s.Stale || d.Text[s.Start:s.End] != "One.\n" {
		t.Fatalf("the other proposal lost its place: %+v", s)
	}
	d, err = store.RejectGroup(d.ID, kept[0].Group)
	if err != nil || len(d.Suggestions) != 0 || d.Text != proposed {
		t.Fatalf("reject the other %+v %v", d, err)
	}
}

// Spans count as a JavaScript editor counts.
func TestSpansCountUTF16(t *testing.T) {
	base := "# Plan 😀\n\nÄrger.\n"
	store, d := newDraft(t, base)
	d, added, _ := store.Propose(d.ID, base, "# Plan 😀\n\nÄrger, less.\n", Proposal{Author: "agent"})
	if span := d.Spans()[added[0].ID]; span != [2]int{11, 18} {
		t.Fatalf("span %v", span)
	}
}

// Typing elsewhere keeps a suggestion in place; typing over its text makes
// it stale, so it is shown but cannot be accepted.
func TestSuggestionsFollowTyping(t *testing.T) {
	base := "# Plan\n\nAlpha.\n\nBeta.\n\nGamma.\n"
	store, d := newDraft(t, base)
	d, added, _ := store.Propose(d.ID, base, "# Plan\n\nAlpha.\n\nBeta, better.\n\nGamma.\n", Proposal{Author: "agent"})
	s := added[0]
	d, err := store.Update(d.ID, d.Revision, "# The Plan\n\nAn introduction.\n\nAlpha.\n\nBeta.\n\nGamma.\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	moved := d.Suggestions[0]
	if moved.Stale || d.Text[moved.Start:moved.End] != "Beta.\n" || moved.Start == s.Start {
		t.Fatalf("the suggestion did not follow the text: %+v", moved)
	}
	d, _ = store.Update(d.ID, d.Revision, "# The Plan\n\nAn introduction.\n\nAlpha.\n\nBeta was rewritten by hand.\n\nGamma.\n", nil)
	if !d.Suggestions[0].Stale {
		t.Fatal("a suggestion survived its text being changed")
	}
	if _, err := store.Accept(d.ID, d.Revision, s.ID); !errors.Is(err, ErrSuggestionStale) {
		t.Fatalf("a stale suggestion was accepted: %v", err)
	}
}

// A proposal made against text the draft has moved on from is placed in the
// draft as it is now.
func TestProposeAgainstAnOlderText(t *testing.T) {
	base := "# Plan\n\nOne.\n\nTwo.\n"
	store, d := newDraft(t, base)
	d, _ = store.Update(d.ID, d.Revision, "# Plan\n\nNew opening.\n\nOne.\n\nTwo.\n", nil)
	d, added, err := store.Propose(d.ID, base, "# Plan\n\nOne.\n\nTwo, revised.\n", Proposal{Author: "agent"})
	if err != nil || len(added) != 1 || added[0].Stale || d.Text[added[0].Start:added[0].End] != "Two.\n" {
		t.Fatalf("proposal %+v %v", added, err)
	}
	// Accepting both of two overlapping suggestions is impossible: the
	// second becomes stale when the first is accepted.
	d, more, _ := store.Propose(d.ID, d.Text, strings.Replace(d.Text, "Two.", "Two, otherwise.", 1), Proposal{Author: "agent 2"})
	d, err = store.Accept(d.ID, d.Revision, added[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range d.Suggestions {
		if s.ID == more[0].ID && !s.Stale {
			t.Fatal("an overlapping suggestion stayed acceptable")
		}
	}
}
