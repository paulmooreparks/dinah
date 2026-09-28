package main

import (
	"strings"
	"testing"

	"dinah/internal/msg"
	"dinah/internal/verb"
)

// TestSpendCellsDrawWhatTheReportCarries pins the three cell composers the
// spend tables draw through, on the shapes the terminal test's fixture does
// not reach: a consumer known by one half, a column the line carries none
// of, and a note after figures.
func TestSpendCellsDrawWhatTheReportCarries(t *testing.T) {
	for _, row := range []struct{ provider, model, want string }{
		{"openai", "gpt-5", "openai/gpt-5"},
		{"openai", "", "openai"},
		{"", "gpt-5", "gpt-5"},
		{"", "", spendAbsent},
	} {
		if got := spendConsumer(row.provider, row.model); got != row.want {
			t.Errorf("spendConsumer(%q, %q) = %q, wanted %q", row.provider, row.model, got, row.want)
		}
	}
	if got := spendText(""); got != spendAbsent {
		t.Errorf("an empty text drew %q rather than the absent glyph", got)
	}
	if got := spendText("Intake"); got != "Intake" {
		t.Errorf("a text drew %q rather than itself", got)
	}
	in, out := 1200.0, 300.5
	figures := spendFigures(verb.SpendFigures{Input: &in, Output: &out}, false, 2, "a remark")
	if want := "input 1200, output 300.5, round 2; a remark"; figures != want {
		t.Errorf("the folded cell reads %q, wanted %q", figures, want)
	}
	if got := spendFigures(verb.SpendFigures{}, false, 0, ""); got != spendAbsent {
		t.Errorf("a row carrying no figure drew %q rather than the absent glyph", got)
	}
}

// TestSpendRecordsAtTheTerminalAndReports drives dinah-647's three shapes at
// the terminal: a record, the card's report and the workbench's, and the
// sentence each report prints where there is nothing to draw. The report's
// figures are asserted as cells, so a sum that printed a zero for an absent
// figure would be caught by the empty cell the control line expects.
func TestSpendRecordsAtTheTerminalAndReports(t *testing.T) {
	root := newBench(t)
	card := addCard(t, root, "A card that cost something")
	base := msg.For(msg.Base)

	none := runCLI(t, root, "spend", card)
	if none.code != 0 {
		t.Fatalf("spend %s on a fresh card: %d %s", card, none.code, none.errw)
	}
	if !strings.Contains(none.out, base.T("spend.none", "ref", card)) {
		t.Errorf("a card without a spend line does not say so: %q", none.out)
	}
	bare := runCLI(t, root, "spend")
	if bare.code != 0 || !strings.Contains(bare.out, base.T("spend.none.workbench")) {
		t.Errorf("a workbench without a spend line does not say so: %d %q %s", bare.code, bare.out, bare.errw)
	}

	if got := runCLI(t, root, "spend", card, "tokens", "--input", "1200", "--output", "300"); got.code != 0 {
		t.Fatalf("spend %s tokens: %d %s", card, got.code, got.errw)
	}
	if got := runCLI(t, root, "spend", card, "tokens", "--unreported", "--round", "2", "--note", "unpriced"); got.code != 0 {
		t.Fatalf("spend %s --unreported: %d %s", card, got.code, got.errw)
	}
	refused := runCLI(t, root, "spend", card, "tokens")
	if refused.code == 0 {
		t.Errorf("a record with no figure and no marker was written: %q", refused.out)
	}

	report := runCLI(t, root, "spend", card)
	if report.code != 0 {
		t.Fatalf("spend %s: %d %s", card, report.code, report.errw)
	}
	for _, want := range []string{base.T("spend.records"), base.T("spend.totals"), "input 1200", "output 300", "unreported, round 2; unpriced"} {
		if !strings.Contains(report.out, want) {
			t.Errorf("the card's report does not carry %q: %q", want, report.out)
		}
	}
	if strings.Contains(report.out, "1500") {
		t.Errorf("the report summed input and output into a total nobody reported: %q", report.out)
	}
	whole := runCLI(t, root, "spend")
	if whole.code != 0 {
		t.Fatalf("spend: %d %s", whole.code, whole.errw)
	}
	if !strings.Contains(whole.out, base.T("spend.totals")) || strings.Contains(whole.out, base.T("spend.records")) {
		t.Errorf("the workbench report should carry totals and no records: %q", whole.out)
	}
	if !strings.Contains(whole.out, "1200") || !strings.Contains(whole.out, "Intake") {
		t.Errorf("the workbench report does not sum the card's line under its column: %q", whole.out)
	}
	machine := runCLI(t, root, "spend", card, "--json")
	if machine.code != 0 || !strings.Contains(machine.out, `"records"`) || !strings.Contains(machine.out, `"unreported": true`) {
		t.Errorf("the machine form of the report is wrong: %d %q", machine.code, machine.out)
	}
}
