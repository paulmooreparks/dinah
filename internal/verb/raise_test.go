package verb

import (
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The identifiers of the raise fixture's columns, named directly so a test can
// assert against one without reading the bench back first.
const (
	raiseIntake = "b00000000001"
	raiseBuild  = "b00000000002"
	raiseReview = "b00000000003"

	// raiseBuildSlug is what a person types for the build column, and what
	// columnRef answers for it, since a slug is preferred over an identifier.
	raiseBuildSlug = "build"
)

// raiseDefinition is a flow declaring a tier axis, which the shared fixture
// deliberately does not: every act this file exercises resolves a tier, and a
// workbench declaring no tier set refuses all of them before the questions
// here are reached.
const raiseDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Raising",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "b00000000001", "title": "Intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Build", "kind": "work", "tier": "workhorse" },
    { "id": "b00000000003", "title": "Review", "kind": "work" },
    { "id": "b00000000004", "title": "Finished", "kind": "done" }
  ]
}`

// retiresBuild is the same flow with the build station gone, which is what a
// reshape carrying a raise-authored override off its own column runs against.
const retiresBuild = `{
  "profile": "dinah-core/0.7",
  "title": "Raising",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "b00000000001", "title": "Intake", "kind": "intake" },
    { "id": "b00000000003", "title": "Review", "kind": "work" },
    { "id": "b00000000004", "title": "Finished", "kind": "done" }
  ]
}`

// raised takes a card up at the build station and raises it to apex, which is
// the act every test in this file starts from, and returns the card's
// reference.
func raised(t *testing.T, h *harness, title, reason string) string {
	t.Helper()
	ref := h.readyAt(title, raiseBuild)
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	response := h.library.Raise(&Request{
		Verb: Raise, Card: ref, Actor: "alka", Tier: "apex", Reason: reason,
	})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("raise: wanted ok, got %s %s", response.Outcome, response.Refusal)
	}
	return ref
}

// TestARaiseAndItsHandBackCannotBeSeparated is AC-8. The raise's two journal
// lines are written inside one per-card lock, so a second process reaching the
// same card in the middle of the act is refused rather than interleaved, and
// the pair comes back off the journal adjacent and sharing one timestamp.
//
// The Interleave hook drives a second library, over the same bench and so
// standing for a second process, into the window between the lock being taken
// and the two appends. That is the only window in which a foreign line could
// land between them, and the second call is one that would write to this very
// journal if it were admitted.
func TestARaiseAndItsHandBackCannotBeSeparated(t *testing.T) {
	h := harnessFromDefinition(t, "rz", raiseDefinition)
	ref := h.readyAt("contested", raiseBuild)
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})

	other := h.second()
	var intruder *Response
	h.library.Interleave = func() {
		intruder = other.Do(&Request{
			Verb: Block, Card: ref, Actor: "bob", Reason: "a second process writing to the same journal",
		})
	}
	response := h.library.Raise(&Request{
		Verb: Raise, Card: ref, Actor: "alka", Tier: "apex", Reason: "found deeper coupling than the ticket suggested",
	})
	h.library.Interleave = nil
	h.reopen()

	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("the raise: wanted ok, got %s %s", response.Outcome, response.Refusal)
	}
	if intruder == nil {
		t.Fatal("the interleaved write never ran, so this test proves nothing")
	}
	if intruder.Outcome != contract.OutcomeRefused || intruder.Refusal != contract.Locked {
		t.Errorf("the interleaved write: wanted %s, got %s %s", contract.Locked, intruder.Outcome, intruder.Refusal)
	}

	events := h.events(ref)
	if len(events) < 2 {
		t.Fatalf("wanted at least the raise's own two lines, got %d", len(events))
	}
	raise := events[len(events)-2]
	freed := events[len(events)-1]
	if raise.Event != contract.EventTierOverridden || freed.Event != contract.EventReleased {
		t.Fatalf("wanted %s then %s as the last two lines, got %s then %s",
			contract.EventTierOverridden, contract.EventReleased, raise.Event, freed.Event)
	}
	if raise.TS != freed.TS {
		t.Errorf("the pair carries %q and %q, and one act writes one timestamp", raise.TS, freed.TS)
	}
	for _, event := range events {
		if event.Event == contract.EventBlocked {
			t.Errorf("the interleaved write reached the journal: %+v", event)
		}
	}
}

// TestAReshapeDropsARaiseAuthoredOverrideLikeAnyOther is AC-11. A tier_at
// entry a raise wrote is byte-identical on disk to one an ordinary per-column
// write produced, so retiring its column drops it through the hook dinah-408
// already shipped, with no change to reshape.
func TestAReshapeDropsARaiseAuthoredOverrideLikeAnyOther(t *testing.T) {
	h := harnessFromDefinition(t, "rz", raiseDefinition)
	ref := raised(t, h, "carried", "found deeper coupling than the ticket suggested")

	before := h.card(ref).ColumnTiers
	if len(before) != 1 || before[0].Tier != "apex" {
		t.Fatalf("wanted the raise's own override before the reshape, got %+v", before)
	}

	if _, err := h.reshape(h.source(retiresBuild), true, raiseBuild+"="+raiseReview); err != nil {
		t.Fatalf("reshape: %v", err)
	}

	if after := h.card(ref).ColumnTiers; len(after) != 0 {
		t.Errorf("the raise-authored override survived the retirement of its column: %+v", after)
	}
	drops := 0
	var dropped bench.Event
	for _, event := range h.events(ref) {
		if event.Event == contract.EventTierOverrideDropped {
			drops++
			dropped = event
		}
		if event.Event == contract.EventTierOverrideDropped && event.Reason != "" {
			t.Errorf("the drop carries the raise's reason, which belongs to the write that happened: %q", event.Reason)
		}
	}
	if drops != 1 {
		t.Fatalf("wanted exactly one tier_override_dropped event, got %d", drops)
	}
	if dropped.Column != raiseBuild {
		t.Errorf("the drop names column %q, wanted the retired column %s", dropped.Column, raiseBuild)
	}
	if dropped.From != "apex" {
		t.Errorf("the drop carries from %q, wanted the dropped value apex", dropped.From)
	}
}

// TestARaiseIsRefusedWhereTheCardIsNotHeld is the library's half of AC-4: the
// two states a bystander can be in answer under one name, and neither reaches
// the anchor or the journal.
func TestARaiseIsRefusedWhereTheCardIsNotHeld(t *testing.T) {
	h := harnessFromDefinition(t, "rz", raiseDefinition)
	unheld := h.readyAt("nobody holds it", raiseBuild)
	held := h.readyAt("somebody else holds it", raiseBuild)
	h.mustDo(&Request{Verb: Claim, Card: held, Actor: "bob"})

	for _, tc := range []struct {
		name   string
		ref    string
		detail string
	}{
		{name: "unheld", ref: unheld, detail: ""},
		{name: "held by another", ref: held, detail: "bob"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			digest := h.digest()
			response := h.library.Raise(&Request{
				Verb: Raise, Card: tc.ref, Actor: "alka", Tier: "apex", Reason: "the work is harder than it looked",
			})
			h.reopen()
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotHolder {
				t.Fatalf("wanted %s, got %s %s", contract.NotHolder, response.Outcome, response.Refusal)
			}
			if response.Detail != tc.detail {
				t.Errorf("the refusal names %q as the holder, wanted %q", response.Detail, tc.detail)
			}
			if h.digest() != digest {
				t.Error("the refused raise wrote to the workbench")
			}
		})
	}
}

// TestARaiseSkipsTheComparisonAgainstARequirementTheWorkbenchHasLost covers a
// card asking for a tier the declared set no longer carries: there is no rank
// to compare against, so the raise proceeds rather than measuring against a
// fabricated rank of zero. It lives at the library because that is the only
// layer where the state can be built.
//
// No acceptance criterion on dinah-409 names this case. An earlier round of
// this comment cited AC-14, which is the log-rendering criterion and was never
// about it, and the citation was written before AC-13 and AC-14 were
// repurposed. Cite nothing rather than the wrong thing.
func TestARaiseSkipsTheComparisonAgainstARequirementTheWorkbenchHasLost(t *testing.T) {
	h := harnessFromDefinition(t, "rz", raiseDefinition)
	ref := h.readyAt("assessed against a retired rung", raiseBuild)
	card := h.card(ref)
	card.Tier = "consultant"
	if err := card.Save(); err != nil {
		t.Fatalf("save %s: %v", ref, err)
	}
	h.reopen()
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})

	response := h.library.Raise(&Request{
		Verb: Raise, Card: ref, Actor: "alka", Tier: "workhorse",
		Reason: "the stored requirement names a rung this workbench no longer declares",
	})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("a raise to the lowest declared rung against an unresolvable requirement: wanted ok, got %s %s",
			response.Outcome, response.Refusal)
	}
	if got := h.card(ref).ColumnTierFor(h.library.Bench, raiseBuildSlug); got != "workhorse" {
		t.Errorf("the override reads %q, wanted workhorse", got)
	}
}
