package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
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

// raisingDefinition is the workbench the raise tests run against: three
// declared rungs, a column defaulting to the lowest of them, and a column
// carrying no default at all, so a relative expression can be measured at one
// and refused at the other.
const raisingDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Raising",
  "levels": { "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Build", "slug": "build", "kind": "work", "tier": "workhorse" },
    { "id": "c00000000003", "title": "Plain", "slug": "plain", "kind": "work" },
    { "id": "c00000000004", "title": "Done", "kind": "done" }
  ]
}`

// heldAt files a card, carries it to a column and takes it up, which is the
// state every raise starts from.
func heldAt(t *testing.T, root, title, column string) {
	t.Helper()
	if got := runCLI(t, root, "add", title); got.code != 0 {
		t.Fatalf("add %s: %d %s", title, got.code, got.errw)
	}
	ref := lastCardRef(t, root)
	if got := runCLI(t, root, "move", ref, column); got.code != 0 {
		t.Fatalf("move %s: %d %s", ref, got.code, got.errw)
	}
	if got := runCLI(t, root, "claim", ref); got.code != 0 {
		t.Fatalf("claim %s: %d %s", ref, got.code, got.errw)
	}
}

// lastCardRef is the reference of the card filed most recently, read off the
// tool rather than composed here, so a change to how references are minted
// does not need this file edited.
func lastCardRef(t *testing.T, root string) string {
	t.Helper()
	got := runCLI(t, root, "ls", "--json")
	if got.code != 0 {
		t.Fatalf("ls: %d %s", got.code, got.errw)
	}
	var answer struct {
		Cards []struct {
			Ref string `json:"ref"`
		} `json:"cards"`
	}
	if err := json.Unmarshal([]byte(got.out), &answer); err != nil {
		t.Fatalf("decode the listing: %v\n%s", err, got.out)
	}
	if len(answer.Cards) == 0 {
		t.Fatal("the workbench carries no cards")
	}
	return answer.Cards[len(answer.Cards)-1].Ref
}

// TestARaiseWritesTheOverrideAndHandsTheCardBack asserts AC-1: one act writes
// the requirement, frees the claim, and leaves two adjacent journal lines
// sharing a timestamp, the first of them carrying the reason.
func TestARaiseWritesTheOverrideAndHandsTheCardBack(t *testing.T) {
	root := newBenchFromDefinition(t, raisingDefinition)
	heldAt(t, root, "a card that turns out to be harder", "build")

	const reason = "found deeper coupling than the ticket suggested"
	if got := runCLI(t, root, "raise", "fx-1", "apex", reason); got.code != 0 {
		t.Fatalf("raise: %d %s", got.code, got.errw)
	}

	anchor := anchorText(t, root, "fx-1")
	if !strings.Contains(anchor, "tier_at:\n  - column: build\n    tier: apex\n") {
		t.Errorf("the anchor does not carry the raised requirement:\n%s", anchor)
	}
	if !strings.Contains(anchor, "state: ready") {
		t.Errorf("the card was not handed back:\n%s", anchor)
	}
	if strings.Contains(anchor, "claim_holder:") {
		t.Errorf("the card is still held:\n%s", anchor)
	}

	events := cardEvents(t, root, "fx-1")
	if len(events) < 2 {
		t.Fatalf("wanted at least the raise's own two lines, got %d", len(events))
	}
	raised := events[len(events)-2]
	freed := events[len(events)-1]
	if raised.Event != contract.EventTierOverridden || freed.Event != contract.EventReleased {
		t.Fatalf("wanted %s then %s as the last two lines, got %s then %s",
			contract.EventTierOverridden, contract.EventReleased, raised.Event, freed.Event)
	}
	if raised.TS != freed.TS {
		t.Errorf("the pair carries %q and %q, and one act writes one timestamp", raised.TS, freed.TS)
	}
	if raised.Reason != reason {
		t.Errorf("the raise carries reason %q, wanted %q", raised.Reason, reason)
	}
	if raised.Column != "c00000000002" || raised.ColumnTitle != "Build" {
		t.Errorf("the raise names column %q titled %q, wanted the build column and its title",
			raised.Column, raised.ColumnTitle)
	}
	if raised.From != "" || raised.To != "apex" || raised.Expr != "apex" || raised.Against != "" {
		t.Errorf("the raise carries from %q to %q expr %q against %q, wanted an absolute write over no requirement",
			raised.From, raised.To, raised.Expr, raised.Against)
	}
	if freed.Actor != raised.Actor {
		t.Errorf("the two lines name %q and %q, and one act has one actor", raised.Actor, freed.Actor)
	}
}

// TestARaiseResolvingAtOrBelowWhatTheCardAsksIsRefused asserts AC-2 and AC-3:
// the comparison is against what the card itself already requires, so a
// relative step measured from the column's own lower default is refused, and
// so is an absolute value that is plainly lower.
func TestARaiseResolvingAtOrBelowWhatTheCardAsksIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name      string
		set       []string
		expr      string
		attempted string
	}{
		{
			name:      "a relative step measured from the column default",
			set:       []string{"card", "set", "fx-1", "tier", "apex", "--at", "build"},
			expr:      "+1",
			attempted: "frontier",
		},
		{
			name:      "an absolute value below the baseline",
			set:       []string{"card", "set", "fx-1", "tier", "apex"},
			expr:      "workhorse",
			attempted: "workhorse",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newBenchFromDefinition(t, raisingDefinition)
			heldAt(t, root, "a card assessed at the top already", "build")
			if got := runCLI(t, root, tc.set...); got.code != 0 {
				t.Fatalf("%v: %d %s", tc.set, got.code, got.errw)
			}
			anchor := anchorText(t, root, "fx-1")
			events := len(cardEvents(t, root, "fx-1"))

			refused := runCLI(t, root, "raise", "fx-1", tc.expr, "the ticket understated it")
			if refused.code != 2 {
				t.Fatalf("the raise exited %d, wanted 2: %s", refused.code, refused.errw)
			}
			if name := refusalNameOf(refused.errw); name != contract.TierNotHigher {
				t.Errorf("the refusal name is %s, wanted %s", name, contract.TierNotHigher)
			}
			english := msg.For(msg.Base)
			want := contract.TierNotHigher + " " +
				english.T("refusal.dinah.tier-not-higher", "current", "apex", "attempted", tc.attempted) +
				english.T("refusal.dinah.tier-not-higher.next", "current", "apex") + "\n"
			if refused.errw != want {
				t.Errorf("the sentence a caller reads:\n got  %q\n want %q", refused.errw, want)
			}
			after := anchorText(t, root, "fx-1")
			if after != anchor {
				t.Errorf("the refused raise reached the anchor:\n%s", after)
			}
			if got := len(cardEvents(t, root, "fx-1")); got != events {
				t.Errorf("the refused raise wrote %d journal lines", got-events)
			}
			if !strings.Contains(after, "state: active") {
				t.Errorf("the refused raise handed the card back anyway:\n%s", after)
			}
		})
	}
}

// TestARaiseByAnyoneButTheHolderIsRefused asserts AC-4: an unclaimed card and
// a card somebody else holds answer under one name, and neither is written to.
func TestARaiseByAnyoneButTheHolderIsRefused(t *testing.T) {
	root := newBenchFromDefinition(t, raisingDefinition)
	if got := runCLI(t, root, "add", "nobody holds it"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "build"); got.code != 0 {
		t.Fatalf("move: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "somebody else holds it"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-2", "build"); got.code != 0 {
		t.Fatalf("move fx-2: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "claim", "fx-2", "--actor", "bo"); got.code != 0 {
		t.Fatalf("claim fx-2 as bo: %d %s", got.code, got.errw)
	}

	for _, tc := range []struct{ name, ref, holder string }{
		{name: "unclaimed", ref: "fx-1", holder: ""},
		{name: "held by another", ref: "fx-2", holder: "bo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			anchor := anchorText(t, root, tc.ref)
			events := len(cardEvents(t, root, tc.ref))
			refused := runCLI(t, root, "raise", tc.ref, "apex", "the ticket understated it")
			if refused.code != 2 {
				t.Fatalf("the raise exited %d, wanted 2: %s", refused.code, refused.errw)
			}
			if name := refusalNameOf(refused.errw); name != contract.NotHolder {
				t.Errorf("the refusal name is %s, wanted %s", name, contract.NotHolder)
			}
			english := msg.For(msg.Base)
			want := contract.NotHolder + " " +
				english.T("refusal.not-holder.unnamed") +
				english.T("refusal.not-holder.next-unheld", "card", tc.ref) + "\n"
			if tc.holder != "" {
				want = contract.NotHolder + " " +
					english.T("refusal.not-holder", "detail", tc.holder) +
					english.T("refusal.not-holder.next", "detail", tc.holder) + "\n"
			}
			if refused.errw != want {
				t.Errorf("the sentence a caller reads:\n got  %q\n want %q", refused.errw, want)
			}
			if after := anchorText(t, root, tc.ref); after != anchor {
				t.Errorf("the refused raise reached the anchor:\n%s", after)
			}
			if got := len(cardEvents(t, root, tc.ref)); got != events {
				t.Errorf("the refused raise wrote %d journal lines", got-events)
			}
		})
	}
}

// TestARaiseWithNoReasonIsRefusedBeforeTheTierIsResolved asserts AC-5, which
// has two halves. The reason is checked ahead of the expression, so an
// invocation that would fail both fails for the missing reason, which is the
// order the printed check table promises. And the sentence the caller reads is
// raise's own: the whole of stderr is compared against the variant pair, so a
// raise that fell back on block's entries would fail here even though it
// refused under the right name. The name half alone cannot see that, which is
// how this card once shipped a refusal telling a caller to run `dinah block`.
func TestARaiseWithNoReasonIsRefusedBeforeTheTierIsResolved(t *testing.T) {
	root := newBenchFromDefinition(t, raisingDefinition)
	heldAt(t, root, "a card with nothing said about it", "build")
	anchor := anchorText(t, root, "fx-1")
	english := msg.For(msg.Base)

	refused := runCLI(t, root, "raise", "fx-1", "+5")
	if refused.code != 2 {
		t.Fatalf("the raise exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.NoReason {
		t.Errorf("the refusal name is %s, wanted %s, so the tier was resolved before the reason was asked for", name, contract.NoReason)
	}
	want := contract.NoReason + " " +
		english.T("refusal.no-reason.raise") +
		english.T("refusal.no-reason.raise.next", "card", "fx-1") + "\n"
	if refused.errw != want {
		t.Errorf("the sentence a caller reads:\n got  %q\n want %q", refused.errw, want)
	}
	if strings.Contains(refused.errw, english.T("refusal.no-reason.next", "card", "fx-1")) {
		t.Errorf("block's next step reached a reader who typed raise, so following it runs a different verb against the card: %q", refused.errw)
	}
	if strings.Contains(refused.errw, english.T("refusal.no-reason")) {
		t.Errorf("block's own sentence reached a reader who typed raise: %q", refused.errw)
	}
	if after := anchorText(t, root, "fx-1"); after != anchor {
		t.Errorf("the refused raise reached the anchor:\n%s", after)
	}
}

// TestARaiseReusesTheResolversOwnRefusals asserts AC-6 and AC-7: raise calls
// the same resolver the ordinary per-column write calls, so a column carrying
// no default and a step off the end of the set are refused under the names
// that write already uses, in sentences of the same shape.
func TestARaiseReusesTheResolversOwnRefusals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		column  string
		step    string
		refusal string
	}{
		{name: "no default to be relative to", column: "plain", step: "+1", refusal: contract.NoTierDefault},
		{name: "a step off the end of the set", column: "build", step: "+9", refusal: contract.TierOutOfRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newBenchFromDefinition(t, raisingDefinition)
			heldAt(t, root, "a card at a station that cannot answer", tc.column)
			anchor := anchorText(t, root, "fx-1")

			refused := runCLI(t, root, "raise", "fx-1", tc.step, "the ticket understated it")
			if refused.code != 2 {
				t.Fatalf("the raise exited %d, wanted 2: %s", refused.code, refused.errw)
			}
			if name := refusalNameOf(refused.errw); name != tc.refusal {
				t.Errorf("the refusal name is %s, wanted %s", name, tc.refusal)
			}
			if after := anchorText(t, root, "fx-1"); after != anchor {
				t.Errorf("the refused raise reached the anchor:\n%s", after)
			}

			// The same condition reached through the ordinary write prints
			// the same sentence, which is what reusing the name buys. The two
			// invocations differ only in the verb that raised it, so a
			// message naming the verb would show up here as a difference.
			write := runCLI(t, root, "card", "set", "fx-1", "tier", tc.step, "--at", tc.column)
			if got, want := flattenWords(write.errw), flattenWords(refused.errw); got != want {
				t.Errorf("the raise prints\n%s\nand the ordinary write prints\n%s", want, got)
			}
		})
	}
}

// TestTheLogPrintsARaisesColumnTitleReasonAndRanks asserts AC-14: the one
// surface a person reads carries the whole of what the raise decided, with the
// column named by the title the event itself captured.
func TestTheLogPrintsARaisesColumnTitleReasonAndRanks(t *testing.T) {
	root := newBenchFromDefinition(t, raisingDefinition)
	heldAt(t, root, "a card that turns out to be harder", "build")
	const reason = "found deeper coupling than the ticket suggested"
	if got := runCLI(t, root, "raise", "fx-1", "apex", reason); got.code != 0 {
		t.Fatalf("raise: %d %s", got.code, got.errw)
	}

	got := runCLI(t, root, "log", "fx-1")
	if got.code != 0 {
		t.Fatalf("log: %d %s", got.code, got.errw)
	}
	want := "Build: no requirement to apex (" + reason + ")"
	if !strings.Contains(flattenWords(got.out), want) {
		t.Errorf("the log does not carry %q:\n%s", want, got.out)
	}
	if strings.Contains(got.out, "c00000000002") {
		t.Errorf("the log prints the raw column identifier where the event captured a title:\n%s", got.out)
	}
}

// TestTheLogFallsBackToTheStoredColumnIdentifier asserts AC-13, which is the
// sibling line and the one a reader meets most often: an ordinary per-column
// tier write captures no column title and carries no reason, so its own line
// degrades to the stored identifier rather than to a blank detail, and prints
// no empty parenthesis where a raise prints its justification.
func TestTheLogFallsBackToTheStoredColumnIdentifier(t *testing.T) {
	root := newBenchFromDefinition(t, raisingDefinition)
	if got := runCLI(t, root, "add", "a card triage assessed"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "card", "set", "fx-1", "tier", "apex", "--at", "build"); got.code != 0 {
		t.Fatalf("card set tier --at build: %d %s", got.code, got.errw)
	}

	overrides := tierOverriddenEvents(cardEvents(t, root, "fx-1"))
	if len(overrides) != 1 {
		t.Fatalf("wanted one tier_overridden event, got %d", len(overrides))
	}
	if overrides[0].ColumnTitle != "" {
		t.Fatalf("the ordinary write captured a column title, so this test no longer builds the state it names: %+v", overrides[0])
	}

	got := runCLI(t, root, "log", "fx-1")
	if got.code != 0 {
		t.Fatalf("log: %d %s", got.code, got.errw)
	}
	flat := flattenWords(got.out)
	if !strings.Contains(flat, "c00000000002: no requirement to apex") {
		t.Errorf("the log does not fall back to the stored column identifier:\n%s", got.out)
	}
	if strings.Contains(flat, "()") {
		t.Errorf("the log prints an empty parenthesis where the write carried no reason:\n%s", got.out)
	}
}

// TestTheRaiseHelpTableMatchesTheEvaluationOrder asserts AC-12: `dinah help
// raise` prints the ten rows in the order the library evaluates them, with the
// operator row first because raise checks the operator itself, and both the
// page and the format document state that the reason is required and never
// verified.
func TestTheRaiseHelpTableMatchesTheEvaluationOrder(t *testing.T) {
	wanted := []string{
		contract.NoOperator,
		contract.UnknownCard,
		contract.NoOwner,
		contract.NotHolder,
		contract.NoReason,
		contract.UnknownColumn,
		contract.NoTierDefault,
		contract.UnknownLevel,
		contract.TierOutOfRange,
		contract.TierNotHigher,
	}
	checks := verb.Checks(verb.Raise)
	if len(checks) != len(wanted) {
		t.Fatalf("wanted %d rows, got %d, so raise is picking up the workbench pair it does not run", len(wanted), len(checks))
	}
	if verb.IsContractVerb(verb.Raise) {
		t.Error("raise reads as a contract verb, which would prefix the workbench pair onto its own table")
	}
	for i, want := range wanted {
		if checks[i].Refusal != want {
			t.Errorf("row %d reports %s, wanted %s", i+1, checks[i].Refusal, want)
		}
		if got, key := checks[i].Key, "check.raise."+strconv.Itoa(i+1); got != key {
			t.Errorf("row %d carries the key %s, wanted %s", i+1, got, key)
		}
	}

	root := newBenchFromDefinition(t, raisingDefinition)
	page := runCLI(t, root, "help", "raise")
	if page.code != 0 {
		t.Fatalf("help raise: %d %s", page.code, page.errw)
	}
	flat := flattenWords(page.out)
	previous := -1
	for i := range wanted {
		row := flattenWords(msg.For(msg.Base).T("check.raise." + strconv.Itoa(i+1)))
		at := strings.Index(flat, row)
		if at < 0 {
			t.Errorf("the page does not print row %d (%q):\n%s", i+1, row, page.out)
			continue
		}
		if at < previous {
			t.Errorf("row %d is printed before the row above it, so the page and the code disagree:\n%s", i+1, page.out)
		}
		previous = at
	}
	if !strings.Contains(flat, "records that you wrote one and never checks that it is true") {
		t.Errorf("dinah help raise does not state that the reason is required and never verified:\n%s", page.out)
	}

	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "design", "format.md"))
	if err != nil {
		t.Fatalf("read the format document: %v", err)
	}
	prose := flattenWords(string(doc))
	for _, clause := range []string{
		"A raise's reason is trusted prose",
		"It does not, and structurally cannot, check that the reason is true",
	} {
		if !strings.Contains(prose, clause) {
			t.Errorf("docs/design/format.md does not state %q", clause)
		}
	}
}
