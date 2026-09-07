package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// The tier tests sit in a file of their own rather than in levels_test.go,
// which they borrow every helper from. Tier is two things, and only the first
// of them is the level axis that file is about: the second is the tier
// assignment, which is a column default, a per-card override map, a write-time
// resolution with provenance, and a claim gate. Folding thirteen assertions
// about that second thing into the file that guards severity and priority
// would make both harder to read than either is now.

// tierDefinition is the workbench most of these tests run against: a declared
// tier set of three members, a column carrying a default of the middle rung,
// and a column carrying none at all. Both columns take work up, so a claim is
// legal at either one and only the card decides whether it is admitted.
const tierDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Tiered",
  "levels": {
    "severity": ["trivial", "minor", "major", "critical"],
    "priority": ["later", "soon", "next", "now"],
    "tier": ["workhorse", "frontier", "apex"]
  },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Test", "slug": "test", "kind": "work", "tier": "frontier" },
    { "id": "c00000000003", "title": "Plain", "slug": "plain", "kind": "work" },
    { "id": "c00000000004", "title": "Done", "kind": "done" }
  ]
}`

// topTierDefinition gives a column the highest declared rung and gives no card
// anything, which is the shape AC-7 is written against: a column's own default
// creates no requirement however high it is set.
const topTierDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Top tier",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Strict", "slug": "strict", "kind": "work", "tier": "apex" },
    { "id": "c00000000003", "title": "Done", "kind": "done" }
  ]
}`

// twoRungDefinition declares a set the relative form can be walked off the end
// of, which is what AC-5 needs: the column's default sits at the top rung, so
// one step up lands outside the set.
const twoRungDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Two rungs",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Ceiling", "slug": "ceiling", "kind": "work", "tier": "apex" },
    { "id": "c00000000003", "title": "Done", "kind": "done" }
  ]
}`

// noTierDefinition declares severity and priority and no tier axis at all,
// which is the workbench every "absence changes nothing" assertion runs on.
const noTierDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "No tier",
  "levels": { "severity": ["trivial", "minor", "major", "critical"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Plain", "slug": "plain", "kind": "work" },
    { "id": "c00000000003", "title": "Done", "kind": "done" }
  ]
}`

// tierOverriddenEvents is the tier_overridden lines of a journal, which is
// where the provenance of every override write lands.
func tierOverriddenEvents(events []bench.Event) []bench.Event {
	var overrides []bench.Event
	for _, event := range events {
		if event.Event == contract.EventTierOverridden {
			overrides = append(overrides, event)
		}
	}
	return overrides
}

// TestWritingAndReadingACardsBaselineTier asserts AC-1: the tier axis is a
// level like the other two, so the write, the read and the journal line all
// come out of the machinery severity and priority already run through.
func TestWritingAndReadingACardsBaselineTier(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	if got := runCLI(t, root, "add", "a card to assess"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "card", "set", "fx-1", "tier", "apex"); got.code != 0 {
		t.Fatalf("card set tier: %d %s", got.code, got.errw)
	}
	read := runCLI(t, root, "card", "get", "fx-1", "tier")
	if read.code != 0 {
		t.Fatalf("card get tier: %d %s", read.code, read.errw)
	}
	if got := strings.TrimSpace(read.out); got != "apex" {
		t.Errorf("card get tier printed %q, wanted apex", got)
	}
	found := false
	for _, event := range updatesOf(cardEvents(t, root, "fx-1")) {
		if event.Field == bench.TierField && event.To == "apex" {
			found = true
		}
	}
	if !found {
		t.Errorf("no card_updated event carries field tier and to apex:\n%s", anchorText(t, root, "fx-1"))
	}
}

// TestWritingATierOnAWorkbenchDeclaringNoneRefuses asserts AC-2: a tier write
// on a workbench declaring no tier set refuses under the name an undeclared
// axis already refuses under, naming the axis rather than the workbench.
func TestWritingATierOnAWorkbenchDeclaringNoneRefuses(t *testing.T) {
	root := newBenchFromDefinition(t, noTierDefinition)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	refused := runCLI(t, root, "card", "set", "fx-1", "tier", "apex")
	if refused.code != 2 {
		t.Fatalf("the write exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.NoLevels {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.NoLevels)
	}
	if !strings.Contains(refused.errw, "tier") {
		t.Errorf("the sentence does not name the axis:\n%s", refused.errw)
	}
	if strings.Contains(anchorText(t, root, "fx-1"), "tier:") {
		t.Errorf("the refused write reached the anchor:\n%s", anchorText(t, root, "fx-1"))
	}
}

// TestARelativeOverrideStoresTheAbsoluteAndJournalsWhatWasTyped asserts AC-3:
// the anchor carries the resolved member name and never the expression, and
// the journal carries the expression and the default it was resolved against.
func TestARelativeOverrideStoresTheAbsoluteAndJournalsWhatWasTyped(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	if got := runCLI(t, root, "add", "a card to assess"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "card", "set", "fx-1", "tier", "+1", "--at", "test"); got.code != 0 {
		t.Fatalf("card set tier +1 --at test: %d %s", got.code, got.errw)
	}
	anchor := anchorText(t, root, "fx-1")
	if !strings.Contains(anchor, "tier_at:\n  - column: test\n    tier: apex\n") {
		t.Errorf("the anchor does not carry the resolved override:\n%s", anchor)
	}
	if strings.Contains(anchor, "+1") {
		t.Errorf("the anchor stores the expression rather than what it resolved to:\n%s", anchor)
	}
	overrides := tierOverriddenEvents(cardEvents(t, root, "fx-1"))
	if len(overrides) != 1 {
		t.Fatalf("wanted one tier_overridden event, got %d", len(overrides))
	}
	event := overrides[0]
	if event.Column != "c00000000002" {
		t.Errorf("the event names column %q, wanted the resolved identifier of test", event.Column)
	}
	if event.Expr != "+1" {
		t.Errorf("the event carries expr %q, wanted +1", event.Expr)
	}
	if event.Against != "frontier" {
		t.Errorf("the event carries against %q, wanted the column's own default frontier", event.Against)
	}
	if event.To != "apex" {
		t.Errorf("the event carries to %q, wanted apex", event.To)
	}
}

// TestARelativeOverrideAgainstAColumnWithNoDefaultRefuses asserts AC-4: there
// is nothing to be relative to, the refusal says so under its own name, and
// the anchor is untouched.
func TestARelativeOverrideAgainstAColumnWithNoDefaultRefuses(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	if got := runCLI(t, root, "add", "a card to assess"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	before := anchorText(t, root, "fx-1")
	refused := runCLI(t, root, "card", "set", "fx-1", "tier", "+1", "--at", "plain")
	if refused.code != 2 {
		t.Fatalf("the write exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.NoTierDefault {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.NoTierDefault)
	}
	if !strings.Contains(refused.errw, "plain") {
		t.Errorf("the sentence does not name the column:\n%s", refused.errw)
	}
	if after := anchorText(t, root, "fx-1"); after != before {
		t.Errorf("the refused write reached the anchor:\n%s", after)
	}
}

// TestARelativeOverrideOffTheEndOfTheSetRefuses asserts AC-5: a step landing
// outside the declared set refuses under its own name and nothing clamps to
// the nearest rung.
func TestARelativeOverrideOffTheEndOfTheSetRefuses(t *testing.T) {
	root := newBenchFromDefinition(t, twoRungDefinition)
	if got := runCLI(t, root, "add", "a card to assess"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	before := anchorText(t, root, "fx-1")
	refused := runCLI(t, root, "card", "set", "fx-1", "tier", "+1", "--at", "ceiling")
	if refused.code != 2 {
		t.Fatalf("the write exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.TierOutOfRange {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.TierOutOfRange)
	}
	if !strings.Contains(refused.errw, "3") {
		t.Errorf("the sentence does not report the rung the step landed at:\n%s", refused.errw)
	}
	if !strings.Contains(refused.errw, "workhorse, frontier, apex") {
		t.Errorf("the sentence does not list the declared set:\n%s", refused.errw)
	}
	if after := anchorText(t, root, "fx-1"); after != before {
		t.Errorf("the refused write reached the anchor:\n%s", after)
	}
}

// TestACardsOwnBaselineIsAFloorOnEveryClaim asserts AC-6, which is the one
// refusal this feature ships: the requirement comes from the card, the gate is
// a floor rather than a match, and it applies at a column carrying no default
// of its own.
func TestACardsOwnBaselineIsAFloorOnEveryClaim(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	for _, title := range []string{"below", "level", "above"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	for _, ref := range []string{"fx-1", "fx-2", "fx-3"} {
		if got := runCLI(t, root, "card", "set", ref, "tier", "frontier"); got.code != 0 {
			t.Fatalf("card set tier on %s: %d %s", ref, got.code, got.errw)
		}
		if got := runCLI(t, root, "move", ref, "plain"); got.code != 0 {
			t.Fatalf("move %s: %d %s", ref, got.code, got.errw)
		}
	}
	refused := runCLI(t, root, "claim", "fx-1", "--tier", "workhorse")
	if refused.code != 2 {
		t.Fatalf("a claim below the card's baseline exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.BelowTier {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.BelowTier)
	}
	if !strings.Contains(refused.errw, "frontier") {
		t.Errorf("the sentence does not name the tier the card asks for:\n%s", refused.errw)
	}
	if !strings.Contains(refused.errw, "Plain") && !strings.Contains(refused.errw, "plain") {
		t.Errorf("the sentence does not name the column:\n%s", refused.errw)
	}
	if got := runCLI(t, root, "claim", "fx-2", "--tier", "frontier"); got.code != 0 {
		t.Errorf("a claim at the card's own baseline was refused: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "claim", "fx-3", "--tier", "apex"); got.code != 0 {
		t.Errorf("a claim above the card's own baseline was refused, so the gate is a match rather than a floor: %d %s", got.code, got.errw)
	}
}

// TestAColumnsOwnDefaultRefusesNothing asserts AC-7 and the operator's ruling
// behind it: a column carrying the highest declared rung admits a bare claim
// and a claim declaring the lowest rung, for a card that asks for nothing.
func TestAColumnsOwnDefaultRefusesNothing(t *testing.T) {
	root := newBenchFromDefinition(t, topTierDefinition)
	for _, title := range []string{"bare", "declared"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	for _, ref := range []string{"fx-1", "fx-2"} {
		if got := runCLI(t, root, "move", ref, "strict"); got.code != 0 {
			t.Fatalf("move %s: %d %s", ref, got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "claim", "fx-1"); got.code != 0 {
		t.Errorf("a bare claim into a column carrying the top tier default was refused: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "claim", "fx-2", "--tier", "workhorse"); got.code != 0 {
		t.Errorf("a claim declaring the lowest rung was refused at a column defaulting to the highest: %d %s", got.code, got.errw)
	}
}

// TestAnOverrideGovernsItsOwnColumnAndTheBaselineGovernsElsewhere asserts
// AC-8: one card, one run, two columns, and the two rules answering
// differently at each.
func TestAnOverrideGovernsItsOwnColumnAndTheBaselineGovernsElsewhere(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	for _, title := range []string{"at the override", "away from it"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	for _, ref := range []string{"fx-1", "fx-2"} {
		if got := runCLI(t, root, "card", "set", ref, "tier", "apex"); got.code != 0 {
			t.Fatalf("card set tier on %s: %d %s", ref, got.code, got.errw)
		}
		if got := runCLI(t, root, "card", "set", ref, "tier", "frontier", "--at", "test"); got.code != 0 {
			t.Fatalf("card set tier --at test on %s: %d %s", ref, got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "move", "fx-1", "test"); got.code != 0 {
		t.Fatalf("move fx-1: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-2", "plain"); got.code != 0 {
		t.Fatalf("move fx-2: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "claim", "fx-1", "--tier", "frontier"); got.code != 0 {
		t.Errorf("the override for this column did not win over the baseline: %d %s", got.code, got.errw)
	}
	refused := runCLI(t, root, "claim", "fx-2", "--tier", "frontier")
	if refused.code != 2 {
		t.Fatalf("the baseline did not govern a column the override was not written for: %d %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.BelowTier {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.BelowTier)
	}
	if !strings.Contains(refused.errw, "apex") {
		t.Errorf("the sentence names a tier other than the baseline:\n%s", refused.errw)
	}
}

// TestAClaimAtAColumnTheWorkbenchNoLongerDeclaresIsRefusedRatherThanPanicking
// guards the state the gate meets when the card carries a floor and the column
// carries nothing at all, which is the one input claimableTier cannot resolve.
//
// The state is built the way dinah check finds it rather than by handing the
// gate a nil: a card is given a baseline, moved to a column, and its anchor is
// then rewritten to an identifier no column declares, which is the stranded
// card the quick start's own damaged-workbench transcript produces. The test
// asserts that check reports that state before the claim runs, so a later edit
// that stops producing it fails here rather than passing vacuously.
//
// What the claim owes is a refusal, and the assertions say so. A panic is not
// a refusal and neither is a silent admission, so both the exit code and the
// refusal name are checked, and the sentence is read for the identifier the
// card still carries, because that is the value an operator repairs the
// workbench with.
func TestAClaimAtAColumnTheWorkbenchNoLongerDeclaresIsRefusedRatherThanPanicking(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	if got := runCLI(t, root, "add", "a card whose column went away"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "card", "set", "fx-1", "tier", "frontier"); got.code != 0 {
		t.Fatalf("card set tier: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "plain"); got.code != 0 {
		t.Fatalf("move: %d %s", got.code, got.errw)
	}
	stranded := "c00000000009"
	rewriteAnchor(t, root, "fx-1", "column: c00000000003", "column: "+stranded)

	checked := runCLI(t, root, "check")
	if checked.code == 4 {
		t.Fatalf("check could not open the workbench: %d %s", checked.code, checked.errw)
	}
	sentence := msg.For(msg.Base).T(bench.FindingUnknownColumn, "detail", stranded)
	if !strings.Contains(checked.out, sentence) {
		t.Fatalf("check does not report the stranded column, so this test no longer builds the state it names:\n%s", checked.out)
	}

	refused := runCLI(t, root, "claim", "fx-1", "--tier", "workhorse")
	if refused.code != 2 {
		t.Fatalf("a claim below the card's baseline at a stranded column exited %d, wanted 2: %s%s", refused.code, refused.out, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.BelowTier {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.BelowTier)
	}
	if !strings.Contains(refused.errw, "frontier") {
		t.Errorf("the sentence does not name the tier the card asks for:\n%s", refused.errw)
	}
	if !strings.Contains(refused.errw, stranded) {
		t.Errorf("the sentence does not name the identifier the card still carries, which is what repairs the workbench:\n%s", refused.errw)
	}

	if got := runCLI(t, root, "claim", "fx-1", "--tier", "frontier"); got.code != 0 {
		t.Errorf("a claim meeting the card's baseline at a stranded column was refused: %d %s", got.code, got.errw)
	}
}

// TestCheckReportsAnOverrideNamingNoColumn asserts AC-11: a stale column
// reference in tier_at is a finding under its own key and not a refused read.
func TestCheckReportsAnOverrideNamingNoColumn(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	if got := runCLI(t, root, "add", "a card to assess"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "card", "set", "fx-1", "tier", "apex", "--at", "test"); got.code != 0 {
		t.Fatalf("card set tier --at test: %d %s", got.code, got.errw)
	}
	rewriteAnchor(t, root, "fx-1", "  - column: test\n", "  - column: retired-long-ago\n")
	got := runCLI(t, root, "check")
	if got.code == 4 {
		t.Fatalf("check could not open the workbench: %d %s", got.code, got.errw)
	}
	if !strings.Contains(got.out, "fx-1") || !strings.Contains(got.out, "retired-long-ago") {
		t.Errorf("check does not report the card and the stale reference:\n%s", got.out)
	}
	if strings.Contains(got.out, "which this workbench does not declare") {
		t.Errorf("the stale tier column was reported as an unknown column rather than under its own finding:\n%s", got.out)
	}
}

// TestCheckReportsAColumnsOwnStaleTierDefault asserts AC-16: a column default
// naming no declared member is reported under the level finding, in the
// axis@column shape, and the workbench still opens.
func TestCheckReportsAColumnsOwnStaleTierDefault(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	rewriteColumnAnchor(t, root, "test", "tier: frontier", "tier: retired-rung")
	got := runCLI(t, root, "check")
	if got.code == 4 {
		t.Fatalf("check could not open the workbench: %d %s", got.code, got.errw)
	}
	if !strings.Contains(got.out, "tier@test retired-rung") {
		t.Errorf("check does not report the column's stale default in the axis@column shape:\n%s", got.out)
	}
}

// TestAStaleColumnDefaultRefusesTheRelativeWriteAndNothingElse asserts AC-15,
// which is the criterion holding the two halves of the ruling apart. A stale
// default is the resolver's problem and it is nobody else's: the claim gate
// never reads the column's value, so a card standing in a column carrying a
// stale default is claimed exactly as it is at a column carrying none.
func TestAStaleColumnDefaultRefusesTheRelativeWriteAndNothingElse(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	for _, title := range []string{"in the stale column", "in the plain column"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	rewriteColumnAnchor(t, root, "test", "tier: frontier", "tier: retired-rung")

	before := anchorText(t, root, "fx-1")
	refused := runCLI(t, root, "card", "set", "fx-1", "tier", "+1", "--at", "test")
	if refused.code != 2 {
		t.Fatalf("the relative write exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.UnknownLevel {
		t.Errorf("the refusal name is %s, wanted %s, since a stale default and a missing one are different repairs", name, contract.UnknownLevel)
	}
	if !strings.Contains(refused.errw, "retired-rung") {
		t.Errorf("the sentence does not name the column's stored value:\n%s", refused.errw)
	}
	if after := anchorText(t, root, "fx-1"); after != before {
		t.Errorf("the refused write reached the anchor:\n%s", after)
	}

	// The two claims are the halves that have to agree. fx-1 stands in the
	// column carrying the stale default and fx-2 in a column carrying none,
	// and neither card asks for anything, so both are admitted.
	if got := runCLI(t, root, "move", "fx-1", "test"); got.code != 0 {
		t.Fatalf("move fx-1: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-2", "plain"); got.code != 0 {
		t.Fatalf("move fx-2: %d %s", got.code, got.errw)
	}
	stale := runCLI(t, root, "claim", "fx-1")
	plain := runCLI(t, root, "claim", "fx-2")
	if stale.code != plain.code {
		t.Errorf("a claim in the column carrying a stale default exited %d and one in a column carrying no default exited %d, so the gate reads the column's own value", stale.code, plain.code)
	}
	if stale.code != 0 {
		t.Errorf("a claim in the column carrying a stale default was refused: %d %s", stale.code, stale.errw)
	}
}

// TestCreatingAColumnValidatesItsTierDefault asserts AC-14: the one path that
// writes a column's tier default validates it exactly as a card's own level
// write is validated, on both the undeclared-axis case and the undeclared-value
// case.
func TestCreatingAColumnValidatesItsTierDefault(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	refused := runCLI(t, root, "column", "new", "Review", "--tier", "consultant")
	if refused.code != 2 {
		t.Fatalf("a column named a tier the workbench does not declare and exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.UnknownLevel {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnknownLevel)
	}
	if got := runCLI(t, root, "columns"); strings.Contains(got.out, "Review") {
		t.Errorf("the refused column was created anyway:\n%s", got.out)
	}
	if got := runCLI(t, root, "column", "new", "Review", "--tier", "apex"); got.code != 0 {
		t.Fatalf("a column naming a declared tier was refused: %d %s", got.code, got.errw)
	}

	bare := newBenchFromDefinition(t, noTierDefinition)
	refusedAxis := runCLI(t, bare, "column", "new", "Review", "--tier", "apex")
	if refusedAxis.code != 2 {
		t.Fatalf("a column named a tier on a workbench declaring no tier axis and exited %d, wanted 2: %s", refusedAxis.code, refusedAxis.errw)
	}
	if name := refusalNameOf(refusedAxis.errw); name != contract.NoLevels {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.NoLevels)
	}
}

// TestAWorkbenchDeclaringNoTierBehavesExactlyAsItDidBefore asserts AC-12 at
// the surface a person meets: on a workbench declaring no tier axis, a card
// that has been filed, claimed, released, moved and had a level written gains
// no tier key, no tier_at key and no tier event.
func TestAWorkbenchDeclaringNoTierBehavesExactlyAsItDidBefore(t *testing.T) {
	root := newBenchFromDefinition(t, noTierDefinition)
	if got := runCLI(t, root, "add", "an ordinary card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for _, args := range [][]string{
		{"card", "set", "fx-1", "severity", "major"},
		{"move", "fx-1", "plain"},
		{"claim", "fx-1"},
		{"release", "fx-1"},
	} {
		if got := runCLI(t, root, args...); got.code != 0 {
			t.Fatalf("%v: %d %s", args, got.code, got.errw)
		}
	}
	anchor := anchorText(t, root, "fx-1")
	if strings.Contains(anchor, "tier") {
		t.Errorf("the anchor gained a tier key on a workbench declaring no tier axis:\n%s", anchor)
	}
	for _, event := range cardEvents(t, root, "fx-1") {
		if event.Event == contract.EventTierOverridden || event.Event == contract.EventTierOverrideDropped {
			t.Errorf("the journal gained a %s event on a workbench declaring no tier axis", event.Event)
		}
	}
	if got := runCLI(t, root, "check"); got.code != 0 {
		t.Errorf("check reported a defect on an ordinary workbench: %d\n%s", got.code, got.out)
	}
}

// rewriteAnchor edits a card's anchor by hand, which is how a test reaches the
// states no write path can produce: a reference that resolved when it was
// written and does not now, and a stored value the declared set has lost.
func rewriteAnchor(t *testing.T, root, ref, from, to string) {
	t.Helper()
	got := runCLI(t, root, "path", ref)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", ref, got.code, got.errw)
	}
	path := strings.TrimSpace(got.out)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	edited := strings.Replace(string(data), from, to, 1)
	if edited == string(data) {
		t.Fatalf("the anchor of %s carries no %q to rewrite:\n%s", ref, from, data)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write the anchor of %s: %v", ref, err)
	}
}

// rewriteColumnAnchor is the same hand edit against a column's own anchor,
// reached through the identifier the tool reports rather than by walking the
// tree.
func rewriteColumnAnchor(t *testing.T, root, slug, from, to string) {
	t.Helper()
	got := runCLI(t, root, "path", slug)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", slug, got.code, got.errw)
	}
	path := strings.TrimSpace(got.out)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", slug, err)
	}
	edited := strings.Replace(string(data), from, to, 1)
	if edited == string(data) {
		t.Fatalf("the anchor of %s carries no %q to rewrite:\n%s", slug, from, data)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write the anchor of %s: %v", slug, err)
	}
}

// TestTheClaimHelpAndTheFormatDocStateBothLimits is dinah-408 AC-18. The two
// limits are the ones a caller has to meet before relying on the gate, and a
// caller meets them where they read: the help text for each --tier flag, and
// the design document's own tier section.
//
// The assertions are over the clauses rather than over whole sentences,
// because the help wraps at the reader's window and the document is prose
// somebody will reword. What each one names is the claim that cannot go
// missing: a declared tier is taken on trust, and a column's default cannot
// make a station selective.
func TestTheClaimHelpAndTheFormatDocStateBothLimits(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	claim := flattenWords(runCLI(t, root, "help", "claim").out)
	for _, clause := range []string{
		"taken on trust and never verified",
		"only what the card itself asks for can refuse you, never a column's own default",
	} {
		if !strings.Contains(claim, clause) {
			t.Errorf("dinah help claim does not state %q:\n%s", clause, claim)
		}
	}
	column := flattenWords(runCLI(t, root, "help", "column").out)
	if !strings.Contains(column, "never refuses a claim, so it cannot make this column selective") {
		t.Errorf("dinah help column does not state that a tier default cannot make a column selective:\n%s", column)
	}

	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "design", "format.md"))
	if err != nil {
		t.Fatalf("read the format document: %v", err)
	}
	prose := flattenWords(string(doc))
	for _, clause := range []string{
		"Dinah cannot verify a declared tier",
		"a claim the tool takes on trust rather than a capability it checks",
		"A column cannot make itself selective by declaring a default",
		"Only a requirement the card itself carries can refuse a claim",
	} {
		if !strings.Contains(prose, clause) {
			t.Errorf("docs/design/format.md does not state %q", clause)
		}
	}
}

// The block below is dinah-410, which is selection rather than the gate: next
// and pull take the same --tier declaration claim takes and answer with a card
// that declaration is admitted for. It sits in this file because it reads the
// same fixtures and asks the same question of the same code, and splitting it
// off would leave a reader comparing the offer against the refusal across two
// files.

// selectionDefinition is a plain flow of two work columns, which is all most
// of the selection assertions need: a queue to stand cards in and a
// destination to pull them into.
const selectionDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Selection",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "d00000000001", "title": "Intake", "kind": "intake" },
    { "id": "d00000000002", "title": "Queue", "slug": "queue", "kind": "work" },
    { "id": "d00000000003", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "d00000000004", "title": "Done", "kind": "done" }
  ]
}`

// routeDefinition is long enough to put three stops between the column a card
// stands at and the column its override names, which is the distance the
// landing-column reading of "competent for" has to survive.
const routeDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Route",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "e00000000001", "title": "Intake", "kind": "intake" },
    { "id": "e00000000002", "title": "First", "slug": "first", "kind": "work" },
    { "id": "e00000000003", "title": "Second", "slug": "second", "kind": "work" },
    { "id": "e00000000004", "title": "Third", "slug": "third", "kind": "work" },
    { "id": "e00000000005", "title": "Fourth", "slug": "fourth", "kind": "work" },
    { "id": "e00000000006", "title": "Done", "kind": "done" }
  ]
}`

// bufferDefinition puts a column nobody takes work up at between the intake
// and the work, so a card standing in the buffer leaves by a pull into the
// column beyond rather than by a claim where it stands.
const bufferDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Buffered",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "f00000000001", "title": "Intake", "kind": "intake" },
    { "id": "f00000000002", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" },
    { "id": "f00000000003", "title": "Work", "slug": "work", "kind": "work" },
    { "id": "f00000000004", "title": "Done", "kind": "done" }
  ]
}`

// offerLine is one entry of what `dinah --json next` prints. It is spelled
// here rather than borrowed from verb.Offer because these assertions are
// about the wire shape a reader branches on, and a struct shared with the
// producer would agree with the producer whatever the JSON said.
type offerLine struct {
	Column string `json:"column"`
	Title  string `json:"title"`
	Card   *struct {
		Ref string `json:"ref"`
	} `json:"card"`
	AwaitingOutside bool `json:"awaiting_outside"`
	NoTaker         bool `json:"no_taker"`
	TakenByPull     bool `json:"taken_by_pull"`
	AboveTier       bool `json:"above_tier"`
}

// answerLine is the part of a pull's JSON answer these assertions read.
type answerLine struct {
	Outcome string `json:"outcome"`
	Message string `json:"message"`
	Card    *struct {
		Ref string `json:"ref"`
	} `json:"card"`
}

// offersFrom runs `dinah --json next` with the given arguments and decodes
// what it printed.
func offersFrom(t *testing.T, root string, argv ...string) []offerLine {
	t.Helper()
	got := runCLI(t, root, append([]string{"--json", "next"}, argv...)...)
	if got.code != 0 {
		t.Fatalf("next %v: %d %s", argv, got.code, got.errw)
	}
	var offers []offerLine
	if err := json.Unmarshal([]byte(got.out), &offers); err != nil {
		t.Fatalf("next %v: decode %v:\n%s", argv, err, got.out)
	}
	return offers
}

// soleOffer is offersFrom for the calls that name one column and so expect
// exactly one entry back.
func soleOffer(t *testing.T, root string, argv ...string) offerLine {
	t.Helper()
	offers := offersFrom(t, root, argv...)
	if len(offers) != 1 {
		t.Fatalf("next %v: wanted one offer, got %d:\n%+v", argv, len(offers), offers)
	}
	return offers[0]
}

// answerFrom runs `dinah --json pull` and decodes what it printed.
func answerFrom(t *testing.T, root string, argv ...string) answerLine {
	t.Helper()
	got := runCLI(t, root, append([]string{"--json", "pull"}, argv...)...)
	if got.code != 0 {
		t.Fatalf("pull %v: %d %s", argv, got.code, got.errw)
	}
	var answer answerLine
	if err := json.Unmarshal([]byte(got.out), &answer); err != nil {
		t.Fatalf("pull %v: decode %v:\n%s", argv, err, got.out)
	}
	return answer
}

// standCard files a card, gives it a baseline tier when one is named, and
// moves it to a column. The three selection fixtures all build their queues
// this way, and a card's arrival order is the order this is called in.
func standCard(t *testing.T, root, ref, title, tier, column string) {
	t.Helper()
	if got := runCLI(t, root, "add", title); got.code != 0 {
		t.Fatalf("add %s: %d %s", title, got.code, got.errw)
	}
	if tier != "" {
		if got := runCLI(t, root, "card", "set", ref, "tier", tier); got.code != 0 {
			t.Fatalf("card set tier on %s: %d %s", ref, got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "move", ref, column); got.code != 0 {
		t.Fatalf("move %s to %s: %d %s", ref, column, got.code, got.errw)
	}
}

// TestNextOffersTheFirstCardTheDeclaredTierAdmits asserts dinah-410 AC-1 and
// the decision behind it: selection filters the arrival order and adds no
// order of its own, so a workhorse caller is offered the first card it is
// admitted for and never the head of the queue it is not.
func TestNextOffersTheFirstCardTheDeclaredTierAdmits(t *testing.T) {
	root := newBenchFromDefinition(t, selectionDefinition)
	standCard(t, root, "fx-1", "first and above", "apex", "queue")
	standCard(t, root, "fx-2", "second and within", "workhorse", "queue")
	standCard(t, root, "fx-3", "third and above", "apex", "queue")

	offer := soleOffer(t, root, "--column", "queue", "--tier", "workhorse")
	if offer.Card == nil {
		t.Fatalf("a queue holding a card this caller may take offered nothing: %+v", offer)
	}
	if offer.Card.Ref != "fx-2" {
		t.Errorf("the offer names %s, wanted fx-2: the head is above this caller and the third card is behind an eligible one", offer.Card.Ref)
	}
	// The same queue, read by a caller admitted for everything in it, answers
	// with the head. Without this the assertion above would also pass on an
	// implementation that always skipped the first card.
	apex := soleOffer(t, root, "--column", "queue", "--tier", "apex")
	if apex.Card == nil || apex.Card.Ref != "fx-1" {
		t.Errorf("an apex caller was not offered the head of the queue: %+v", apex)
	}
}

// TestNextSeparatesWorkAboveTheTierFromNothingReady asserts dinah-410 AC-2:
// a column holding only work above the caller says so, a column holding
// nothing says nothing, and the two are different answers on the wire and on
// the terminal.
func TestNextSeparatesWorkAboveTheTierFromNothingReady(t *testing.T) {
	root := newBenchFromDefinition(t, selectionDefinition)
	standCard(t, root, "fx-1", "above every workhorse", "apex", "queue")

	gated := soleOffer(t, root, "--column", "queue", "--tier", "workhorse")
	if gated.Card != nil {
		t.Errorf("a card above the declared tier was offered anyway: %+v", gated.Card)
	}
	if !gated.AboveTier {
		t.Errorf("the offer does not carry above_tier, so a reader cannot tell gated work from an empty queue: %+v", gated)
	}
	if gated.NoTaker || gated.AwaitingOutside {
		t.Errorf("the gated offer raises another column's flag: %+v", gated)
	}

	// Doing holds nothing at all, and its offer must not borrow the flag.
	empty := soleOffer(t, root, "--column", "doing", "--tier", "workhorse")
	if empty.Card != nil {
		t.Errorf("an empty column offered a card: %+v", empty.Card)
	}
	if empty.AboveTier || empty.NoTaker || empty.AwaitingOutside || empty.TakenByPull {
		t.Errorf("an empty column raised a flag, so nothing-ready reads as something: %+v", empty)
	}

	// The terminal says the same thing in its own words, which is the surface
	// an operator reads when a queue stalls above every agent available.
	printed := runCLI(t, root, "next", "--column", "queue", "--tier", "workhorse")
	if printed.code != 0 {
		t.Fatalf("next: %d %s", printed.code, printed.errw)
	}
	if want := msg.For(msg.Base).T("next.above-tier"); !strings.Contains(printed.out, want) {
		t.Errorf("the table does not print %q:\n%s", want, printed.out)
	}

	// The compact form carries the flag as the off record's sixth value, in
	// the position that record's own doc comment names, so a reader of that
	// form is told what a reader of the canonical form is told.
	compact := runCLI(t, root, "--format", "compact", "next", "--column", "queue", "--tier", "workhorse")
	if compact.code != 0 {
		t.Fatalf("compact next: %d %s", compact.code, compact.errw)
	}
	found := false
	for _, line := range strings.Split(compact.out, "\n") {
		fields := strings.Split(line, "|")
		if fields[0] != "off" {
			continue
		}
		found = true
		if len(fields) != 7 {
			t.Fatalf("the off record carries %d fields, wanted the kind and six values: %q", len(fields), line)
		}
		if fields[6] != "1" {
			t.Errorf("the off record's above_tier value is %q, wanted 1: %q", fields[6], line)
		}
	}
	if !found {
		t.Fatalf("the compact answer carries no off record:\n%s", compact.out)
	}
}

// TestNextWithNoDeclarationWithholdsWhatABareClaimWouldRefuse asserts
// dinah-410 AC-3, which is the whole point of the card: what selection shows
// and what the gate admits are one answer, so a caller declaring nothing is
// shown nothing it would then be refused.
func TestNextWithNoDeclarationWithholdsWhatABareClaimWouldRefuse(t *testing.T) {
	root := newBenchFromDefinition(t, selectionDefinition)
	standCard(t, root, "fx-1", "assessed", "frontier", "queue")

	offer := soleOffer(t, root, "--column", "queue")
	if offer.Card != nil {
		t.Errorf("a caller declaring nothing was offered an assessed card: %+v", offer.Card)
	}
	if !offer.AboveTier {
		t.Errorf("the withheld offer does not say why it is empty: %+v", offer)
	}
	refused := runCLI(t, root, "claim", "fx-1")
	if refused.code != 2 {
		t.Fatalf("the bare claim this offer was compared against exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.BelowTier {
		t.Errorf("the bare claim refused under %s rather than %s, so the two surfaces are not being compared on the same rule", name, contract.BelowTier)
	}
}

// TestNamedPullTakesTheFirstCardTheDeclaredTierAdmits asserts dinah-410 AC-4:
// the named form claims and moves the card selection would have offered, and
// the card it stepped over is left where it stood.
func TestNamedPullTakesTheFirstCardTheDeclaredTierAdmits(t *testing.T) {
	root := newBenchFromDefinition(t, selectionDefinition)
	standCard(t, root, "fx-1", "first and above", "apex", "queue")
	standCard(t, root, "fx-2", "second and within", "workhorse", "queue")

	answer := answerFrom(t, root, "doing", "--tier", "workhorse")
	if answer.Card == nil {
		t.Fatalf("the pull took nothing: %+v", answer)
	}
	if answer.Card.Ref != "fx-2" {
		t.Errorf("the pull took %s, wanted fx-2", answer.Card.Ref)
	}
	listing := runCLI(t, root, "ls", "queue")
	if listing.code != 0 {
		t.Fatalf("ls: %d %s", listing.code, listing.errw)
	}
	if !strings.Contains(listing.out, "fx-1") {
		t.Errorf("the card the pull stepped over did not stay in the queue:\n%s", listing.out)
	}
	if !strings.Contains(listing.out, "ready") {
		t.Errorf("the card the pull stepped over is no longer ready:\n%s", listing.out)
	}
}

// TestNamedPullSeparatesWorkAboveTheTierFromNothingReady asserts dinah-410
// AC-5: the named form carries two messages, and a caller branching on the
// message alone can tell an empty upstream from an upstream it may not take
// from.
func TestNamedPullSeparatesWorkAboveTheTierFromNothingReady(t *testing.T) {
	root := newBenchFromDefinition(t, selectionDefinition)
	standCard(t, root, "fx-1", "above every workhorse", "apex", "queue")

	gated := answerFrom(t, root, "doing", "--tier", "workhorse")
	if gated.Card != nil {
		t.Errorf("the pull took a card above the declared tier: %+v", gated.Card)
	}
	if gated.Message != "answer.pull.above-tier.named" {
		t.Errorf("the answer is %q, wanted answer.pull.above-tier.named", gated.Message)
	}

	// The same call on a workbench whose upstream holds nothing at all keeps
	// the older message, so the two cases never print the same sentence.
	bare := newBenchFromDefinition(t, selectionDefinition)
	empty := answerFrom(t, bare, "doing", "--tier", "workhorse")
	if empty.Message != "answer.pull.empty.named" {
		t.Errorf("an empty upstream answered %q, wanted answer.pull.empty.named", empty.Message)
	}
}

// TestBarePullSaysWorkStandsAboveTheDeclaredTier asserts dinah-410 AC-6: the
// bare form does not name a column it would then refuse the caller at, and it
// says why it named none.
func TestBarePullSaysWorkStandsAboveTheDeclaredTier(t *testing.T) {
	root := newBenchFromDefinition(t, selectionDefinition)
	standCard(t, root, "fx-1", "above every workhorse", "apex", "queue")

	answer := answerFrom(t, root, "--tier", "workhorse")
	if answer.Card != nil {
		t.Errorf("the bare pull took a card above the declared tier: %+v", answer.Card)
	}
	if answer.Message != "answer.pull.above-tier.bare" {
		t.Errorf("the answer is %q, wanted answer.pull.above-tier.bare", answer.Message)
	}
	if answer.Outcome != "ok" {
		t.Errorf("the above-tier answer is an outcome of %q, wanted ok", answer.Outcome)
	}
	// An apex caller reaches the same workbench and is given the column, which
	// is what shows the bare form withheld it for tier rather than for one of
	// the other rows of its own list.
	taken := answerFrom(t, root, "--tier", "apex")
	if taken.Card == nil || taken.Card.Ref != "fx-1" {
		t.Errorf("an apex caller was not given the card the bare form withheld: %+v", taken)
	}
}

// TestACardAskingForNothingIsOfferedAndTakenWhateverIsDeclared asserts
// dinah-410 AC-7: a card nobody has assessed is unaffected by any of this,
// which is the majority of cards on any workbench.
func TestACardAskingForNothingIsOfferedAndTakenWhateverIsDeclared(t *testing.T) {
	for _, declared := range [][]string{nil, {"--tier", "workhorse"}, {"--tier", "apex"}} {
		name := "no declaration"
		if len(declared) == 2 {
			name = declared[1]
		}
		t.Run(name, func(t *testing.T) {
			root := newBenchFromDefinition(t, selectionDefinition)
			standCard(t, root, "fx-1", "unassessed", "", "queue")

			offer := soleOffer(t, root, append([]string{"--column", "queue"}, declared...)...)
			if offer.Card == nil || offer.Card.Ref != "fx-1" {
				t.Fatalf("an unassessed card was not offered: %+v", offer)
			}
			answer := answerFrom(t, root, append([]string{"doing"}, declared...)...)
			if answer.Card == nil || answer.Card.Ref != "fx-1" {
				t.Errorf("an unassessed card was not taken: %+v", answer)
			}
		})
	}
}

// TestARequirementFurtherDownTheRouteDoesNotWithholdWorkHere asserts
// dinah-410 AC-8 and D-1: competent-for is read at the column this call would
// land the card in, never at every column still ahead of it. The requirement
// three stops down is checked again, on its own terms, against whichever
// caller reaches that column.
func TestARequirementFurtherDownTheRouteDoesNotWithholdWorkHere(t *testing.T) {
	root := newBenchFromDefinition(t, routeDefinition)
	standCard(t, root, "fx-1", "assessed further down", "workhorse", "first")
	if got := runCLI(t, root, "card", "set", "fx-1", "tier", "apex", "--at", "fourth"); got.code != 0 {
		t.Fatalf("card set tier --at fourth: %d %s", got.code, got.errw)
	}

	offer := soleOffer(t, root, "--column", "first", "--tier", "workhorse")
	if offer.Card == nil || offer.Card.Ref != "fx-1" {
		t.Fatalf("a requirement declared for a column three stops down withheld the card here: %+v", offer)
	}
	answer := answerFrom(t, root, "second", "--tier", "workhorse")
	if answer.Card == nil || answer.Card.Ref != "fx-1" {
		t.Fatalf("the pull refused a card its own destination admits: %+v", answer)
	}
	// The override still governs its own column, so nothing above has quietly
	// dropped it. A workhorse caller reaching Fourth is refused there.
	if got := runCLI(t, root, "move", "fx-1", "fourth"); got.code != 0 {
		t.Fatalf("move to fourth: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "release", "fx-1"); got.code != 0 {
		t.Fatalf("release: %d %s", got.code, got.errw)
	}
	refused := runCLI(t, root, "claim", "fx-1", "--tier", "workhorse")
	if refused.code != 2 {
		t.Fatalf("the override at Fourth admitted a workhorse claim, so the assertion above proved nothing: %d %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.BelowTier {
		t.Errorf("the refusal at Fourth is %s, wanted %s", name, contract.BelowTier)
	}
}

// TestSelectionAtABufferReadsTheColumnTheCardWouldLandIn asserts dinah-410
// AC-9: where a card leaves by a pull into the column beyond, the tier read
// is that column's, because that is where the claim actually happens. Reading
// the buffer instead would offer work the pull then refuses.
func TestSelectionAtABufferReadsTheColumnTheCardWouldLandIn(t *testing.T) {
	root := newBenchFromDefinition(t, bufferDefinition)
	standCard(t, root, "fx-1", "waiting to be carried on", "", "waiting")
	if got := runCLI(t, root, "card", "set", "fx-1", "tier", "apex", "--at", "work"); got.code != 0 {
		t.Fatalf("card set tier --at work: %d %s", got.code, got.errw)
	}

	offer := soleOffer(t, root, "--column", "waiting", "--tier", "workhorse")
	if offer.Card != nil {
		t.Errorf("the buffer offered a card the column beyond would refuse: %+v", offer.Card)
	}
	if !offer.AboveTier {
		t.Errorf("the buffer's empty offer does not say why it is empty: %+v", offer)
	}
	// The buffer itself carries no requirement, and the card carries no
	// baseline, so an implementation reading the buffer rather than the
	// landing column offers this card. An apex caller is offered it and is
	// told the card leaves by a pull, which is what makes the landing column
	// the one that matters.
	admitted := soleOffer(t, root, "--column", "waiting", "--tier", "apex")
	if admitted.Card == nil || admitted.Card.Ref != "fx-1" {
		t.Fatalf("an apex caller was not offered the buffered card: %+v", admitted)
	}
	if !admitted.TakenByPull {
		t.Errorf("the offer does not say the card leaves by a pull, so this test is not standing at a buffer: %+v", admitted)
	}
}
