package verb

import (
	"reflect"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// spendOn records a spend on a card through the verb and fails the test unless
// it was written.
func (h *harness) spendOn(req Request) {
	h.t.Helper()
	req.Verb, req.Actor = "spend", "alka"
	response := h.library.Spend(&req)
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("spend on %s: %s %s %s", req.Card, response.Outcome, response.Refusal, response.Detail)
	}
	h.reopen()
}

// figure is a pointer to a figure, which is how a record carries one.
func figure(value float64) *float64 { return &value }

// TestSpendRecordsALineAndSumsIt is dinah-647's record and its card report on
// one card that carries every shape a line can take: two figures from the
// recorder's own model, one total from a consumer the dispatcher named on a
// second round in a column the card has left, and a line the harness could
// not price. The totals have to keep the three apart and count the fourth,
// because a sum that folded a consumer into another or an unreported line
// into a zero would pass a test with one line.
func TestSpendRecordsALineAndSumsIt(t *testing.T) {
	h := newHarness(t)
	card := h.ready("A card that cost something")
	h.spendOn(Request{Card: card, Unit: "tokens", Input: "1200", Output: "300", Provider: "anthropic", Model: "claude-fable-5-1"})
	h.spendOn(Request{Card: card, Unit: "tokens", Input: "800", Output: "100", Provider: "anthropic", Model: "claude-fable-5-1", Note: "the second pass"})
	h.spendOn(Request{Card: card, Unit: "tokens", Total: "5000", By: "openai/gpt-5", Round: "2", Column: "doing"})
	h.spendOn(Request{Card: card, Unit: "tokens", Unreported: true, Provider: "anthropic", Model: "claude-fable-5-1"})

	events, _, err := h.library.Bench.ReadJournal(h.card(card).JournalPath())
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	var lines []bench.Event
	for _, ev := range events {
		if ev.Event == contract.EventSpend {
			lines = append(lines, ev)
		}
	}
	if len(lines) != 4 {
		t.Fatalf("wanted four spend lines, got %d", len(lines))
	}
	first := lines[0]
	if first.Unit != "tokens" || first.Input == nil || *first.Input != 1200 || first.Output == nil || *first.Output != 300 ||
		first.Cached != nil || first.Total != nil || first.Unreported {
		t.Errorf("the first line carries the wrong figures: %+v", first)
	}
	if first.Column != aftercare || first.ColumnTitle != "Aftercare" {
		t.Errorf("the first line does not name the card's own column as of the write: %q %q", first.Column, first.ColumnTitle)
	}
	if first.Actor.Name != "alka" || first.Actor.Model != "claude-fable-5-1" || first.ConsumerModel != "" {
		t.Errorf("the first line's recorder and consumer are wrong: %+v", first.Actor)
	}
	third := lines[2]
	if third.ConsumerProvider != "openai" || third.ConsumerModel != "gpt-5" || third.Round != 2 || third.Column != doing || third.ColumnTitle != "Doing" {
		t.Errorf("the third line does not carry the consumer, the round and the named column: %+v", third)
	}
	if !lines[3].Unreported || lines[3].Input != nil || lines[3].Total != nil {
		t.Errorf("the fourth line is not the bare unreported fact: %+v", lines[3])
	}

	report, err := h.library.SpendReport(&Request{Verb: "spend", Actor: "alka", Card: card})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if report.Card != card || len(report.Records) != 4 {
		t.Fatalf("wanted the card's four records, got %+v", report)
	}
	if report.Records[1].Note != "the second pass" || report.Records[2].Provider != "openai" || report.Records[2].Model != "gpt-5" {
		t.Errorf("the records do not resolve the note and the consumer: %+v", report.Records)
	}
	// Doing stands before Aftercare in the fixture, so the consumer's total
	// sorts first however the lines were written.
	want := []SpendTotal{
		{Column: doing, ColumnTitle: "Doing", Provider: "openai", Model: "gpt-5", Unit: "tokens",
			SpendFigures: SpendFigures{Total: figure(5000)}, Records: 1},
		{Column: aftercare, ColumnTitle: "Aftercare", Provider: "anthropic", Model: "claude-fable-5-1", Unit: "tokens",
			SpendFigures: SpendFigures{Input: figure(2000), Output: figure(400)}, Records: 3, Unreported: 1},
	}
	if !reflect.DeepEqual(report.Totals, want) {
		t.Errorf("wanted the totals\n%s\ngot\n%s", spendTotalsText(want), spendTotalsText(report.Totals))
	}

	// The workbench report sums every live card, so a second card's line
	// lands in its own row and the first card's rows are unchanged.
	other := h.ready("Another card that cost something")
	h.spendOn(Request{Card: other, Unit: "minutes", Total: "12", Provider: "local", Model: "llama"})
	whole, err := h.library.SpendReport(&Request{Verb: "spend", Actor: "alka"})
	if err != nil {
		t.Fatalf("workbench report: %v", err)
	}
	if whole.Card != "" || len(whole.Records) != 0 {
		t.Errorf("the workbench report names a card or carries records: %+v", whole)
	}
	want = append(want, SpendTotal{Column: aftercare, ColumnTitle: "Aftercare", Provider: "local", Model: "llama", Unit: "minutes",
		SpendFigures: SpendFigures{Total: figure(12)}, Records: 1})
	if !reflect.DeepEqual(whole.Totals, want) {
		t.Errorf("wanted the workbench totals\n%s\ngot\n%s", spendTotalsText(want), spendTotalsText(whole.Totals))
	}

	// SpendCall tells a record from a report by what the request carries.
	if _, isReport := h.library.SpendCall(&Request{Verb: "spend", Actor: "alka", Card: card}).(*SpendReport); !isReport {
		t.Errorf("a call naming the card alone did not answer a report")
	}
	if _, isRecord := h.library.SpendCall(&Request{Verb: "spend", Actor: "alka", Card: card, Unit: "tokens", Total: "1"}).(*Response); !isRecord {
		t.Errorf("a call naming a unit did not record")
	}
}

// spendTotalsText draws totals for a failure message, with each figure
// dereferenced, since a printed pointer says nothing.
func spendTotalsText(totals []SpendTotal) string {
	text := ""
	for _, total := range totals {
		cell := func(v *float64) string {
			if v == nil {
				return "-"
			}
			return string(rune('0'+int(*v)%10)) + "..."
		}
		text += total.ColumnTitle + " " + total.Provider + "/" + total.Model + " " + total.Unit +
			" in=" + cell(total.Input) + " out=" + cell(total.Output) + " total=" + cell(total.Total) +
			" records=" + string(rune('0'+total.Records)) + " unreported=" + string(rune('0'+total.Unreported)) + "\n"
	}
	return text
}

// TestSpendRefusesWhatItCannotSum asserts every refusal spend raises before
// it writes, each by its own detail, and that a refused call writes nothing:
// the card's journal carries no spend line after the whole table has run.
func TestSpendRefusesWhatItCannotSum(t *testing.T) {
	h := newHarness(t)
	card := h.ready("A card refused a spend")
	for _, row := range []struct {
		name   string
		req    Request
		name_  string
		detail string
	}{
		{"a unit that is not one lowercase word", Request{Unit: "Tokens", Total: "1"}, contract.Malformed, "unit"},
		{"a unit with a space", Request{Unit: "us dollars", Total: "1"}, contract.Malformed, "unit"},
		{"a negative figure", Request{Unit: "tokens", Input: "-1"}, contract.Malformed, "--input"},
		{"a figure that is not a number", Request{Unit: "tokens", Output: "many"}, contract.Malformed, "--output"},
		{"no figure and no marker", Request{Unit: "tokens"}, contract.Malformed, "figure"},
		{"a figure beside the marker", Request{Unit: "tokens", Total: "1", Unreported: true}, contract.Malformed, "--unreported"},
		{"a consumer without a model", Request{Unit: "tokens", Total: "1", By: "anthropic"}, contract.Malformed, "--by"},
		{"a round of zero", Request{Unit: "tokens", Total: "1", Round: "0"}, contract.Malformed, "--round"},
		{"a column the workbench does not declare", Request{Unit: "tokens", Total: "1", Column: "nowhere"}, contract.UnknownColumn, "nowhere"},
	} {
		t.Run(row.name, func(t *testing.T) {
			req := row.req
			req.Verb, req.Actor, req.Card = "spend", "alka", card
			response := h.library.Spend(&req)
			if response.Outcome != contract.OutcomeRefused || response.Refusal != row.name_ {
				t.Fatalf("wanted %s, got %s %s", row.name_, response.Outcome, response.Refusal)
			}
			if response.Detail != row.detail {
				t.Errorf("wanted the detail %q, got %q", row.detail, response.Detail)
			}
		})
	}
	unowned := h.library.Spend(&Request{Verb: "spend", Card: card, Unit: "tokens", Total: "1"})
	if unowned.Outcome != contract.OutcomeRefused || unowned.Refusal != contract.NoOwner {
		t.Errorf("a request naming no owner: wanted %s, got %s %s", contract.NoOwner, unowned.Outcome, unowned.Refusal)
	}
	unknown := h.library.Spend(&Request{Verb: "spend", Actor: "alka", Card: "fx-99", Unit: "tokens", Total: "1"})
	if unknown.Outcome != contract.OutcomeRefused || unknown.Refusal != contract.UnknownCard {
		t.Errorf("a card that does not exist: wanted %s, got %s %s", contract.UnknownCard, unknown.Outcome, unknown.Refusal)
	}
	h.reopen()
	report, err := h.library.SpendReport(&Request{Verb: "spend", Actor: "alka", Card: card})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if len(report.Records) != 0 {
		t.Errorf("a refused call wrote a line: %+v", report.Records)
	}
	// The control: the same card takes a well-formed line, so the refusals
	// above were refusals rather than a verb that writes nothing at all.
	h.spendOn(Request{Card: card, Unit: "tokens", Total: "1"})
	report, err = h.library.SpendReport(&Request{Verb: "spend", Actor: "alka", Card: card})
	if err != nil {
		t.Fatalf("report after the control: %v", err)
	}
	if len(report.Records) != 1 {
		t.Errorf("the control line was not written: %+v", report.Records)
	}
}
