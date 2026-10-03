package verb

import (
	"errors"
	"os"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// handEditAnchor rewrites a card's card.md the way an editor would, outside the
// tool, leaving its journal saying what it already said.
func (h *harness) handEditAnchor(ref string, edit func(text string) string) {
	h.t.Helper()
	anchor := h.card(ref).AnchorPath()
	text, err := bench.ReadText(anchor)
	if err != nil {
		h.t.Fatalf("read %s: %v", anchor, err)
	}
	edited := edit(text)
	if edited == text {
		h.t.Fatalf("the hand edit of %s changed nothing", anchor)
	}
	if err := bench.WriteText(anchor, edited); err != nil {
		h.t.Fatalf("write %s: %v", anchor, err)
	}
	h.reopen()
}

// projectionDifferences answers the keys on which a card's card.md and the
// replay of its journal disagree, which is empty for a card whose journal
// states it.
func (h *harness) projectionDifferences(ref string) []string {
	h.t.Helper()
	card := h.card(ref)
	events, _, err := bench.ReadJournal(card.JournalPath())
	if err != nil {
		h.t.Fatalf("journal %s: %v", ref, err)
	}
	fm, body, ok := bench.ReplayCardFields(events)
	if !ok {
		h.t.Fatalf("the journal of %s states no start to replay from", ref)
	}
	text, err := bench.ReadText(card.AnchorPath())
	if err != nil {
		h.t.Fatalf("read %s: %v", card.AnchorPath(), err)
	}
	anchor, anchorBody := bench.ParseAnchor(text)
	return bench.ProjectionDifferences(anchor, anchorBody, fm, body)
}

// divergedFinding reports whether check names ref's card as diverged.
func (h *harness) divergedFinding(ref string) bool {
	h.t.Helper()
	id := h.card(ref).ID
	for _, f := range h.check() {
		if f.Key == bench.FindingCardProjectionDiverged && strings.Contains(f.Path, id) {
			return true
		}
	}
	return false
}

// TestAHandEditIsWitnessedByTheNextWrite drives dinah-637/criteria/9. Each of
// four hand edits to card.md is followed by a comment on the card, whose lock
// runs the witness first: a body edit gains a witnessed card_updated carrying
// the body in its text, a severity edit a witnessed card_updated for
// severity, a column edit a manual_correction, and an added key a witnessed
// card_updated naming it. check names the card as diverged before the write
// and not after it, and afterwards the replay of the journal value-equals
// card.md. A card nobody edited gains no witness line from the same write.
//
// Arming: dropping the witnessOnAcquire call from Bench.Acquire leaves every
// edit unwitnessed, so the wanted line is missing, check still reports the
// divergence after the write, and the replay disagrees with card.md.
func TestAHandEditIsWitnessedByTheNextWrite(t *testing.T) {
	h := newCardUnitHarness(t)
	ref := h.add("A card somebody edits by hand")
	untouched := h.add("A card nobody edits")

	cases := []struct {
		name  string
		edit  func(text string) string
		event string
		field string
		check func(ev bench.Event) string
	}{
		{
			name:  "body",
			edit:  func(text string) string { return text + "A paragraph typed in an editor.\n" },
			event: contract.EventCardUpdated, field: bench.BodyField,
			check: func(ev bench.Event) string {
				if !strings.Contains(ev.Text, "A paragraph typed in an editor.") {
					return "the line does not carry the edited body in its text: " + ev.Text
				}
				return ""
			},
		},
		{
			name:  "severity",
			edit:  func(text string) string { return strings.Replace(text, "\ntitle:", "\nseverity: major\ntitle:", 1) },
			event: contract.EventCardUpdated, field: bench.SeverityField,
			check: func(ev bench.Event) string {
				if ev.To != "major" {
					return "the line carries " + ev.To + " rather than the severity the edit wrote"
				}
				return ""
			},
		},
		{
			name:  "column",
			edit:  func(text string) string { return strings.Replace(text, "column: "+intake, "column: "+review, 1) },
			event: contract.EventManualCorrection,
			check: func(ev bench.Event) string {
				if ev.From != intake || ev.To != review {
					return "the correction runs from " + ev.From + " to " + ev.To
				}
				return ""
			},
		},
		{
			name: "unknown key",
			edit: func(text string) string {
				return strings.Replace(text, "\ntitle:", "\nhand_key: typed by hand\ntitle:", 1)
			},
			event: contract.EventCardUpdated, field: "hand_key",
			check: func(ev bench.Event) string {
				if ev.To != "typed by hand" {
					return "the line carries " + ev.To + " rather than the value the edit wrote"
				}
				return ""
			},
		},
	}
	for _, c := range cases {
		h.handEditAnchor(ref, c.edit)
		if !h.divergedFinding(ref) {
			t.Errorf("%s: check does not report the unwitnessed edit as %s", c.name, bench.FindingCardProjectionDiverged)
		}
		before := len(h.events(ref))
		h.comment(ref, "the write after the "+c.name+" edit")
		events := h.events(ref)
		added := events[before:]
		if len(added) != 2 {
			t.Fatalf("%s: the write appended %d lines, wanted the witness line and the comment's own: %+v", c.name, len(added), added)
		}
		witness := added[0]
		if witness.Event != c.event || witness.Field != c.field {
			t.Errorf("%s: the witness line is %s about %q, wanted %s about %q", c.name, witness.Event, witness.Field, c.event, c.field)
		}
		if c.event == contract.EventCardUpdated && !witness.Witnessed {
			t.Errorf("%s: the card_updated line is not marked witnessed", c.name)
		}
		if witness.Actor.Name != "alka" {
			t.Errorf("%s: the witness line is attributed to %q, wanted the writer alka", c.name, witness.Actor.Name)
		}
		if problem := c.check(witness); problem != "" {
			t.Errorf("%s: %s", c.name, problem)
		}
		if added[1].Event != contract.EventCommented {
			t.Errorf("%s: the write's own line is %s, wanted it after the witness", c.name, added[1].Event)
		}
		if differ := h.projectionDifferences(ref); len(differ) != 0 {
			t.Errorf("%s: after the write the replay still differs from card.md on %v", c.name, differ)
		}
		if h.divergedFinding(ref) {
			t.Errorf("%s: check still reports the card as diverged after the write witnessed it", c.name)
		}
	}

	before := len(h.events(untouched))
	h.comment(untouched, "a write to a card nobody edited")
	if added := h.events(untouched)[before:]; len(added) != 1 || added[0].Event != contract.EventCommented {
		t.Errorf("a card with no hand edit gained %d lines from one comment, wanted the comment's own: %+v", len(added), added)
	}
}

// TestADamagedCardMdIsRefusedAndRebuilt drives dinah-637/criteria/10. A card
// whose card.md is deleted, and separately one whose card.md carries the three
// conflict markers, is refused dinah.card-projection-unreadable naming the
// file on show; dinah check --rebuild writes a card.md value-equal to the one
// the damage replaced and appends card_rebuilt; and a card whose card.md reads
// is not rewritten.
//
// Arming: making projectionUnreadable answer nil lets the deleted card through
// to the anchor's own not-found refusal and the conflicted one through to
// show, so the refusal assertion fails in both cases, and the rebuild finds
// nothing to rebuild.
func TestADamagedCardMdIsRefusedAndRebuilt(t *testing.T) {
	damages := []struct {
		name   string
		damage func(t *testing.T, anchor, text string)
	}{
		{
			name: "deleted",
			damage: func(t *testing.T, anchor, _ string) {
				if err := os.Remove(anchor); err != nil {
					t.Fatalf("remove %s: %v", anchor, err)
				}
			},
		},
		{
			name: "conflicted",
			damage: func(t *testing.T, anchor, text string) {
				conflicted := text + "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> branch\n"
				if err := bench.WriteText(anchor, conflicted); err != nil {
					t.Fatalf("write %s: %v", anchor, err)
				}
			},
		},
	}
	for _, d := range damages {
		t.Run(d.name, func(t *testing.T) {
			h := newCardUnitHarness(t)
			ref := h.ready("A card whose anchor gets damaged")
			h.mustSet(ref, "body", "The body the journal states.\n")
			h.mustDo(&Request{Verb: "claim", Actor: "alka", Card: ref})
			whole := h.add("A card whose anchor reads")
			card := h.card(ref)
			anchor := card.AnchorPath()
			original, err := bench.ReadText(anchor)
			if err != nil {
				t.Fatalf("read %s: %v", anchor, err)
			}
			wholeAnchor := h.card(whole).AnchorPath()
			wholeBefore, err := os.ReadFile(wholeAnchor)
			if err != nil {
				t.Fatalf("read %s: %v", wholeAnchor, err)
			}

			d.damage(t, anchor, original)
			h.reopen()
			_, _, _, _, shown := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
			var refusal *contract.Refusal
			if !errors.As(shown, &refusal) || refusal.Name != contract.CardProjectionUnreadable {
				t.Fatalf("show on the damaged card answered %v, wanted %s", shown, contract.CardProjectionUnreadable)
			}
			if !strings.HasSuffix(refusal.Detail, bench.CardAnchor) || !strings.Contains(refusal.Detail, card.ID) {
				t.Errorf("the refusal names %q, wanted the card's own card.md", refusal.Detail)
			}

			report, err := h.library.Check(&Request{Verb: "check", Actor: "alka", Rebuild: true})
			if err != nil {
				t.Fatalf("check --rebuild: %v", err)
			}
			if len(report.RebuiltCards) != 1 || report.RebuiltCards[0] != card.ID {
				t.Errorf("the rebuild answered %v, wanted the damaged card %s alone", report.RebuiltCards, card.ID)
			}
			h.reopen()
			rebuilt, err := bench.ReadText(anchor)
			if err != nil {
				t.Fatalf("read the rebuilt %s: %v", anchor, err)
			}
			wantFM, wantBody := bench.ParseAnchor(original)
			gotFM, gotBody := bench.ParseAnchor(rebuilt)
			if differ := bench.ProjectionDifferences(wantFM, wantBody, gotFM, gotBody); len(differ) != 0 {
				t.Errorf("the rebuilt card.md differs from the original on %v:\n--- original\n%s\n--- rebuilt\n%s", differ, original, rebuilt)
			}
			events := h.events(ref)
			if last := events[len(events)-1]; last.Event != contract.EventCardRebuilt {
				t.Errorf("the journal's last line is %s, wanted %s", last.Event, contract.EventCardRebuilt)
			}
			wholeAfter, err := os.ReadFile(wholeAnchor)
			if err != nil {
				t.Fatalf("read %s: %v", wholeAnchor, err)
			}
			if string(wholeAfter) != string(wholeBefore) {
				t.Error("the rebuild rewrote a card whose card.md reads")
			}
			for _, ev := range h.events(whole) {
				if ev.Event == contract.EventCardRebuilt {
					t.Error("the card whose card.md reads gained a card_rebuilt line")
				}
			}
		})
	}
}

// TestACrashBetweenSaveAndAppendIsWitnessedByTheNextWrite drives
// dinah-637/criteria/29. A claim whose card.md save lands without its claimed
// line, planted through commitFailure, is followed by a release whose lock
// first appends witness lines attributed to the releaser and then the
// released line; the replay then value-equals card.md, and no claimed line
// appears anywhere in the journal.
//
// Arming: moving the commitFailure call ahead of Save leaves card.md as it
// was, so the card reads ready rather than the crashed claim's active and the
// test stops there.
func TestACrashBetweenSaveAndAppendIsWitnessedByTheNextWrite(t *testing.T) {
	h := newCardUnitHarness(t)
	ref := h.ready("A card whose claim crashes half way")
	planted := errors.New("the process died between the two writes")
	h.library.commitFailure = func(*bench.Card) error { return planted }
	crashed := h.library.Do(&Request{Verb: "claim", Actor: "bob", Card: ref})
	h.library.commitFailure = nil
	h.reopen()
	if crashed.Outcome == contract.OutcomeOK {
		t.Fatal("the planted failure did not stop the claim, so this test proves nothing")
	}
	if card := h.card(ref); card.State != contract.StateActive || card.Holder != "bob" {
		t.Fatalf("card.md reads %s held by %q, wanted the crashed claim's active and bob", card.State, card.Holder)
	}

	before := len(h.events(ref))
	h.mustDo(&Request{Verb: "release", Actor: "bob", Card: ref})
	added := h.events(ref)[before:]
	if len(added) < 2 {
		t.Fatalf("the release appended %d lines, wanted witness lines and then its own: %+v", len(added), added)
	}
	for _, ev := range added[:len(added)-1] {
		if !ev.Witnessed || ev.Event != contract.EventCardUpdated {
			t.Errorf("a line ahead of the release's own is %s (witnessed %v), wanted a witnessed card_updated", ev.Event, ev.Witnessed)
		}
		if ev.Actor.Name != "bob" {
			t.Errorf("a witness line is attributed to %q, wanted the releaser bob", ev.Actor.Name)
		}
	}
	if last := added[len(added)-1]; last.Event != contract.EventReleased {
		t.Errorf("the release's own line is %s, wanted it last", last.Event)
	}
	if differ := h.projectionDifferences(ref); len(differ) != 0 {
		t.Errorf("after the release the replay differs from card.md on %v", differ)
	}
	for _, ev := range h.events(ref) {
		if ev.Event == contract.EventClaimed {
			t.Error("the crashed claim's own line appears in the journal")
		}
	}
}
