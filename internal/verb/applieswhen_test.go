package verb

import (
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The construction blocks of the dinah-590 specification, gating a level axis
// and a declared field on the same declared field, plus a second gated axis
// so that one gate write can orphan two values at once.
const conditionedLevels = `levels:
  severity:
    values: [cosmetic, functional, safety]
    applies_when:
      field: task.type
      is: [snag]
  priority:
    values: [later, now]
    applies_when:
      field: task.type
      is: [snag]
`

const conditionedFields = `fields:
  task.type:
    type: string
    meaning: whether the task is our own crew's work, subcontracted, or a snag found on inspection
    on: [card]
  task.trade:
    type: string
    meaning: the trade the subcontractor brings
    on: [card]
    applies_when:
      field: task.type
      is: [subcontracted]
`

// declareLevels writes a levels block into the workbench anchor, on the terms
// declareFields writes the fields block.
func (h *harness) declareLevels(block string) {
	h.t.Helper()
	path := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		h.t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.LevelsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		h.t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()
}

// conditionedHarness is a fixture declaring the two blocks above.
func conditionedHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.declareLevels(conditionedLevels)
	h.declareFields(conditionedFields)
	return h
}

// mustSet writes one field and fails the test where the write was refused.
func (h *harness) mustSet(ref, field, value string) *Response {
	h.t.Helper()
	response := h.set(ref, field, value)
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("set %s %s %q: %s %s", ref, field, value, response.Outcome, response.Refusal)
	}
	return response
}

// TestTheApplicabilityRefusalRunsAfterTheValueChecks asserts specification
// section 5.1's order: a level the workbench does not declare is refused for
// that on an inapplicable axis, and a declared one is refused for the axis
// not applying, so the two refusals never trade places.
func TestTheApplicabilityRefusalRunsAfterTheValueChecks(t *testing.T) {
	h := conditionedHarness(t)
	ref := h.add("Frame the extension")
	h.mustSet(ref, "task.type", "own")
	if response := h.set(ref, "severity", "nonsense"); response.Refusal != contract.UnknownLevel {
		t.Errorf("a malformed level on an inapplicable axis is refused %q, wanted %s", response.Refusal, contract.UnknownLevel)
	}
	refused := h.set(ref, "severity", "safety")
	if refused.Refusal != contract.InapplicableField || refused.Detail != "severity" {
		t.Fatalf("a declared level on an inapplicable axis is refused %q over %q, wanted %s over severity", refused.Refusal, refused.Detail, contract.InapplicableField)
	}
	if refused.Context["gate"] != "task.type" || refused.Context["admits"] != "snag" || refused.Context["value"] != "own" {
		t.Errorf("the refusal's context is %v", refused.Context)
	}
	// A malformed declared value is refused for being malformed first.
	if response := h.set(ref, "task.trade", "electrical\nplumbing"); response.Refusal != contract.Malformed {
		t.Errorf("a malformed value on an inapplicable field is refused %q, wanted %s", response.Refusal, contract.Malformed)
	}
	// The same write on a card the condition admits succeeds.
	snag := h.add("Cracked tile in the hall")
	h.mustSet(snag, "task.type", "snag")
	h.mustSet(snag, "severity", "safety")
}

// TestClearingAnOrphanedValueSucceedsAndTheFindingGoes asserts that a
// clearing write is never refused on applicability, and that check stops
// reporting the value once it is gone.
func TestClearingAnOrphanedValueSucceedsAndTheFindingGoes(t *testing.T) {
	h := conditionedHarness(t)
	ref := h.add("Garden lighting")
	h.mustSet(ref, "task.type", "subcontracted")
	h.mustSet(ref, "task.trade", "electrical")
	h.mustSet(ref, "task.type", "own")
	if findings := h.check(); len(findings) != 1 || findings[0].Key != bench.FindingInapplicableValue || findings[0].Detail != "task.trade electrical" {
		t.Fatalf("check reports %+v after the retype, wanted the one orphaned trade", findings)
	}
	h.mustSet(ref, "task.trade", "")
	if findings := h.check(); len(findings) != 0 {
		t.Errorf("check still reports %+v after the orphaned value was cleared", findings)
	}
	// An own-crew task is excluded from every conditioned slot, and the
	// cleared trade stays listed among them with nothing stored under it.
	if view := h.cardView(ref); fieldsOf(view.Inapplicable) != "severity,priority,task.trade" || view.Fields["task.trade"] != "" {
		t.Errorf("the view lists %+v as inapplicable after the clear, with fields %v", view.Inapplicable, view.Fields)
	}
}

// TestTheWarningNamesTheFirstOrphanedSlotInTheFixedOrder asserts specification
// section 5.2: one gate write orphaning a severity and a priority warns once,
// naming severity, which comes first in the order every report keeps.
func TestTheWarningNamesTheFirstOrphanedSlotInTheFixedOrder(t *testing.T) {
	h := conditionedHarness(t)
	ref := h.add("Cracked tile in the hall")
	h.mustSet(ref, "task.type", "snag")
	h.mustSet(ref, "severity", "safety")
	h.mustSet(ref, "priority", "now")
	retyped := h.mustSet(ref, "task.type", "own")
	if retyped.Warning != "warn.inapplicable-value" || retyped.WarningDetail != "severity" {
		t.Errorf("the retype warns %q over %q, wanted warn.inapplicable-value over severity", retyped.Warning, retyped.WarningDetail)
	}
	card := h.card(ref)
	if card.Severity != "safety" || card.Priority != "now" {
		t.Errorf("the retype cleared a kept value: severity %q, priority %q", card.Severity, card.Priority)
	}
	// Writing the gate again, to a value that still excludes the card, warns
	// nothing: no value was applicable before this write.
	if again := h.mustSet(ref, "task.type", "subcontracted"); again.Warning != "" {
		t.Errorf("a second gate write warns %q over values already orphaned", again.Warning)
	}
}

// TestTheViewOmitsInapplicableWhereEverythingApplies asserts specification
// section 6.1: the member is absent on a card every slot applies to, and
// gate_value is absent where the gate stores nothing.
func TestTheViewOmitsInapplicableWhereEverythingApplies(t *testing.T) {
	h := conditionedHarness(t)
	snag := h.add("Cracked tile in the hall")
	h.mustSet(snag, "task.type", "snag")
	// A snag takes both levels and is not a subcontracted task, so the
	// trade alone is inapplicable.
	if view := h.cardView(snag); len(view.Inapplicable) != 1 || view.Inapplicable[0].Field != "task.trade" || view.Inapplicable[0].GateValue != "snag" {
		t.Errorf("the snag's view lists %+v", view.Inapplicable)
	}
	// A card nobody has typed yet has every conditioned slot inapplicable,
	// and none carries a gate value.
	untyped := h.add("Loose handrail")
	view := h.cardView(untyped)
	if got := fieldsOf(view.Inapplicable); got != "severity,priority,task.trade" {
		t.Errorf("the untyped card's view lists %q, wanted severity, priority, then the declared key", got)
	}
	for _, excluded := range view.Inapplicable {
		if excluded.GateValue != "" || excluded.Gate != "task.type" {
			t.Errorf("the untyped card's view carries %+v", excluded)
		}
	}
	// A workbench declaring no condition never carries the member.
	plain := declaringHarness(t)
	if view := plain.cardView(plain.add("A card")); view.Inapplicable != nil {
		t.Errorf("a card on a workbench without conditions lists %+v", view.Inapplicable)
	}
}

// fieldsOf joins the slots an inapplicable listing names.
func fieldsOf(excluded []InapplicableView) string {
	names := make([]string, 0, len(excluded))
	for _, entry := range excluded {
		names = append(names, entry.Field)
	}
	return strings.Join(names, ",")
}
