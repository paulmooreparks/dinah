package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// The two domain blocks of the dinah-590 specification, section 1, verbatim.
// Neither knows anything about software, and both gate a declaration on a
// declared field's value in the same syntax.
const weddingFields = `fields:
  event.category:
    type: string
    meaning: what part of the wedding this booking is for
    on: [card]
  vendor.deposit-required:
    type: boolean
    meaning: whether the vendor asks for a deposit before confirming
    on: [card]
    applies_when:
      field: event.category
      is: [catering, venue]
`

const constructionLevelsBlock = `levels:
  severity:
    values: [cosmetic, functional, safety]
    applies_when:
      field: task.type
      is: [snag]
`

const constructionFieldsBlock = `fields:
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

// declareBlocksOn writes a levels block and a fields block into a workbench's
// own anchor, either of which may be empty. It is declareFieldsOn's shape,
// aimed at both blocks, because no verb declares a level set or a field and
// each is frontmatter a person edits.
func declareBlocksOn(t *testing.T, root, levels, fields string) {
	t.Helper()
	dir := soleBenchDir(t, root)
	path := filepath.Join(dir, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	if levels != "" {
		fm.SetRaw(bench.LevelsKey, bench.SplitLines(strings.TrimSuffix(levels, "\n")))
	}
	if fields != "" {
		fm.SetRaw(bench.FieldsKey, bench.SplitLines(strings.TrimSuffix(fields, "\n")))
	}
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// fileCard files a card and answers the reference add printed for it, which
// is what every later step of a domain test names the card by.
func fileCard(t *testing.T, root, title string, flags ...string) string {
	t.Helper()
	got := runCLI(t, root, append(append([]string{"add"}, flags...), title)...)
	if got.code != 0 {
		t.Fatalf("add %q: %d %s", title, got.code, got.errw)
	}
	return strings.Fields(got.out)[0]
}

// mustSet writes one field and fails the test where the write was refused.
func mustSet(t *testing.T, root, ref, field, value string) invocation {
	t.Helper()
	got := runCLI(t, root, "set", ref, field, value)
	if got.code != 0 {
		t.Fatalf("set %s %s %s: %d %s", ref, field, value, got.code, got.errw)
	}
	return got
}

// assertRefs fails unless a query returned exactly the references named, in
// order. Every caller names at least one, so a query returning nothing never
// passes here.
func assertRefs(t *testing.T, text string, got []string, want ...string) {
	t.Helper()
	if len(want) == 0 {
		t.Fatal("this assertion is written against a non-empty list and was handed none")
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("query %q returned %v, wanted exactly %v", text, got, want)
	}
}

// cardAnchorOf answers the path of a card's anchor through dinah path, which
// is what a finding's own path is compared against.
func cardAnchorOf(t *testing.T, root, ref string) string {
	t.Helper()
	got := runCLI(t, root, "path", ref)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", ref, got.code, got.errw)
	}
	return strings.TrimSpace(got.out)
}

// checkFindings runs check in the machine form and answers its findings.
func checkFindings(t *testing.T, root string) []bench.Finding {
	t.Helper()
	got := runCLI(t, root, "--json", "check")
	var report struct {
		Findings []bench.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(got.out), &report); err != nil {
		t.Fatalf("decode check's answer: %v\n%s", err, got.out)
	}
	return report.Findings
}

// TestAWeddingWorkbenchAsksForADepositOnlyOfCateringAndVenueBookings is the
// operator's wedding test, dinah-590/criteria/2: five bookings, of which only
// the catering and venue ones are asked whether a deposit is required, and a
// query that finds the one nobody has answered yet.
//
// Arming: making bench.Applicability answer true for every slot turns step
// 2's two refusals into successful writes, and the test fails there.
func TestAWeddingWorkbenchAsksForADepositOnlyOfCateringAndVenueBookings(t *testing.T) {
	root := newBench(t)
	declareBlocksOn(t, root, "", weddingFields)
	c1 := fileCard(t, root, "Caterer for the reception")
	c2 := fileCard(t, root, "Wedding cake")
	v := fileCard(t, root, "Reception hall")
	f := fileCard(t, root, "Florist for the ceremony")
	u := fileCard(t, root, "Guest book")
	for ref, category := range map[string]string{c1: "catering", c2: "catering", v: "venue", f: "florist"} {
		mustSet(t, root, ref, "event.category", category)
	}

	// Step 1: the bookings the condition admits take the answer.
	mustSet(t, root, c1, "vendor.deposit-required", "true")
	mustSet(t, root, v, "vendor.deposit-required", "true")

	// Step 2: the florist is refused for what its category is, and the
	// uncategorised booking for having no category at all.
	florist := mustRefuse(t, root, "set", f, "vendor.deposit-required", "true")
	assertRefusal(t, florist, contract.InapplicableField, "a deposit answer on the florist")
	if want := "because event.category is florist"; !strings.Contains(florist.errw, want) {
		t.Errorf("the florist's refusal does not say %q:\n%s", want, florist.errw)
	}
	unset := mustRefuse(t, root, "set", u, "vendor.deposit-required", "true")
	assertRefusal(t, unset, contract.InapplicableField, "a deposit answer on an uncategorised booking")
	if want := "because event.category carries no value"; !strings.Contains(unset.errw, want) {
		t.Errorf("the uncategorised booking's refusal does not say %q:\n%s", want, unset.errw)
	}

	// Step 3: the machine view says why the slot does not apply.
	shown := runCLI(t, root, "--json", "show", f, "--fields", "card")
	if shown.code != 0 {
		t.Fatalf("show %s: %d %s", f, shown.code, shown.errw)
	}
	var detail struct {
		Card struct {
			Inapplicable []struct {
				Field     string `json:"field"`
				Gate      string `json:"gate"`
				GateValue string `json:"gate_value"`
			} `json:"inapplicable"`
		} `json:"card"`
	}
	if err := json.Unmarshal([]byte(shown.out), &detail); err != nil {
		t.Fatalf("decode show's answer: %v\n%s", err, shown.out)
	}
	excluded := detail.Card.Inapplicable
	if len(excluded) != 1 || excluded[0].Field != "vendor.deposit-required" || excluded[0].Gate != "event.category" || excluded[0].GateValue != "florist" {
		t.Errorf("the florist's view lists %+v as inapplicable", excluded)
	}

	// Step 4: the query the planner actually wants. The florist and the
	// guest book carry no answer either, so a query ignoring the category
	// term would return them too, and the list below refuses that.
	const text = `event.category:catering,venue vendor.deposit-required:""`
	assertRefs(t, text, queryRefs(t, root, text), c2)

	// Step 5: recategorising the caterer keeps its answer and says so, and
	// check is where the kept answer is reported.
	retyped := mustSet(t, root, c1, "event.category", "music")
	if want := msg.For(msg.Base).T("warn.inapplicable-value", "detail", "vendor.deposit-required"); !strings.Contains(retyped.errw, want) {
		t.Errorf("recategorising the caterer did not warn %q:\n%s", want, retyped.errw)
	}
	if got := runCLI(t, root, "get", c1, "vendor.deposit-required"); strings.TrimSpace(got.out) != "true" {
		t.Errorf("the caterer's answer reads %q after the recategorisation, wanted true kept", got.out)
	}
	anchor := cardAnchorOf(t, root, c1)
	found := false
	for _, finding := range checkFindings(t, root) {
		if finding.Key == bench.FindingInapplicableValue && finding.Detail == "vendor.deposit-required true" && finding.Path == anchor {
			found = true
		}
	}
	if !found {
		t.Errorf("check does not report the kept answer on %s", c1)
	}
}

// TestAConstructionWorkbenchCarriesATradeOnlyOnSubcontractedTasks is the
// operator's construction test, dinah-590/criteria/3: the trade and the
// severity are gated on the task's type in one syntax, one task's trade is
// orphaned by retyping it, and a query needs both terms to find one task.
//
// Arming: making cardValues answer every dotted field with the empty string
// turns step 5's answer into no card at all, and the list assertion fails.
func TestAConstructionWorkbenchCarriesATradeOnlyOnSubcontractedTasks(t *testing.T) {
	root := newBench(t)
	declareBlocksOn(t, root, constructionLevelsBlock, constructionFieldsBlock)
	k1 := fileCard(t, root, "Rewire the kitchen")
	k2 := fileCard(t, root, "Replumb the bathroom")
	k3 := fileCard(t, root, "Frame the extension")
	k4 := fileCard(t, root, "Cracked tile in the hall")
	k5 := fileCard(t, root, "Garden lighting")
	for ref, kind := range map[string]string{k1: "subcontracted", k2: "subcontracted", k3: "own", k4: "snag", k5: "subcontracted"} {
		mustSet(t, root, ref, "task.type", kind)
	}
	mustSet(t, root, k1, "task.trade", "electrical")
	mustSet(t, root, k2, "task.trade", "plumbing")
	mustSet(t, root, k5, "task.trade", "electrical")
	retyped := mustSet(t, root, k5, "task.type", "own")
	if want := msg.For(msg.Base).T("warn.inapplicable-value", "detail", "task.trade"); !strings.Contains(retyped.errw, want) {
		t.Errorf("retyping the garden lighting did not warn %q:\n%s", want, retyped.errw)
	}
	if got := runCLI(t, root, "get", k5, "task.trade"); strings.TrimSpace(got.out) != "electrical" {
		t.Errorf("the garden lighting's trade reads %q after the retype, wanted electrical kept", got.out)
	}

	// Step 1: an own-crew task takes no trade.
	assertRefusal(t, mustRefuse(t, root, "set", k3, "task.trade", "electrical"), contract.InapplicableField, "a trade on an own-crew task")

	// Step 2: only a snag takes a severity, and the same word admits it on
	// the snag and refuses it on the subcontracted task.
	mustSet(t, root, k4, "severity", "safety")
	assertRefusal(t, mustRefuse(t, root, "set", k1, "severity", "safety"), contract.InapplicableField, "a severity on a subcontracted task")

	// Step 3: a filing stores no type, so a severity named at filing is
	// refused with the gate unset.
	filed := mustRefuse(t, root, "add", "--severity", "safety", "Loose handrail")
	assertRefusal(t, filed, contract.InapplicableField, "a severity named at filing")
	if want := "because task.type carries no value"; !strings.Contains(filed.errw, want) {
		t.Errorf("the filing's refusal does not say %q:\n%s", want, filed.errw)
	}

	// Step 4: the listing marks the axis as not applicable where it does not
	// apply and shows the snag's severity as stored.
	listed := runCLI(t, root, "list", "intake")
	if listed.code != 0 {
		t.Fatalf("list: %d %s", listed.code, listed.errw)
	}
	cells := map[string]string{}
	for _, line := range strings.Split(listed.out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 2 {
			cells[fields[0]] = fields[2]
		}
	}
	mark := msg.For(msg.Base).T("queue.cell.inapplicable")
	for _, ref := range []string{k1, k2} {
		if cells[ref] != mark {
			t.Errorf("%s's severity cell reads %q, wanted %q:\n%s", ref, cells[ref], mark, listed.out)
		}
	}
	if cells[k4] != "safety" {
		t.Errorf("%s's severity cell reads %q, wanted safety:\n%s", k4, cells[k4], listed.out)
	}

	// Step 5: both terms do work. Ignoring the trade term returns the
	// plumbing task as well, and ignoring the type term returns the garden
	// lighting, whose orphaned trade is still stored.
	const text = "task.type:subcontracted task.trade:electrical"
	assertRefs(t, text, queryRefs(t, root, text), k1)
}
