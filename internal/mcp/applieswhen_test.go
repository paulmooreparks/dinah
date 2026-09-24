package mcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// weddingFields is the wedding block of the dinah-590 specification, section
// 1, verbatim.
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

// TestTheQueryToolFindsTheUnansweredCateringBooking is dinah-590/criteria/11:
// the wedding fixture of specification section 11.1 built up to its fourth
// step, and that step's query issued through the query tool, answering the
// one reference the CLI test asserts, spelled out here rather than read off
// the CLI's own answer.
func TestTheQueryToolFindsTheUnansweredCateringBooking(t *testing.T) {
	library := newLibrary(t)
	anchor := filepath.Join(library.Bench.Root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.FieldsKey, bench.SplitLines(strings.TrimSuffix(weddingFields, "\n")))
	if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
	reopen := func() {
		opened, err := bench.Open(library.Bench.Root)
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
		library.Bench = opened
	}
	reopen()

	file := func(title string) string {
		filed := library.Add(&verb.Request{Verb: "add", Actor: "alka", Title: title})
		if filed.Outcome != contract.OutcomeOK {
			t.Fatalf("add %q: %s %s", title, filed.Outcome, filed.Refusal)
		}
		reopen()
		return filed.Card.Ref
	}
	set := func(ref, field, value string) *verb.Response {
		response := library.SetField(&verb.Request{Verb: "set", Actor: "alka", Ref: ref, Field: field, Value: value})
		reopen()
		return response
	}
	mustSet := func(ref, field, value string) {
		if response := set(ref, field, value); response.Outcome != contract.OutcomeOK {
			t.Fatalf("set %s %s %s: %s %s", ref, field, value, response.Outcome, response.Refusal)
		}
	}
	c1 := file("Caterer for the reception")
	c2 := file("Wedding cake")
	v := file("Reception hall")
	f := file("Florist for the ceremony")
	u := file("Guest book")
	mustSet(c1, "event.category", "catering")
	mustSet(c2, "event.category", "catering")
	mustSet(v, "event.category", "venue")
	mustSet(f, "event.category", "florist")
	mustSet(c1, "vendor.deposit-required", "true")
	mustSet(v, "vendor.deposit-required", "true")
	for _, excluded := range []string{f, u} {
		if response := set(excluded, "vendor.deposit-required", "true"); response.Refusal != contract.InapplicableField {
			t.Fatalf("a deposit answer on %s is refused %q, wanted %s", excluded, response.Refusal, contract.InapplicableField)
		}
	}

	query, err := json.Marshal(`event.category:catering,venue vendor.deposit-required:""`)
	if err != nil {
		t.Fatalf("marshal the query: %v", err)
	}
	answer := ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"query":`+string(query)+`}}}`)
	carried := payload(t, answer)
	reencoded, err := json.Marshal(carried["matches"])
	if err != nil {
		t.Fatalf("marshal the tool's answer: %v", err)
	}
	matches := &verb.Matches{}
	if err := json.Unmarshal(reencoded, matches); err != nil {
		t.Fatalf("the tool's answer does not decode as Matches: %v", err)
	}
	refs := make([]string, 0, len(matches.Cards))
	for _, card := range matches.Cards {
		refs = append(refs, card.Ref)
	}
	want := []string{c2}
	if strings.Join(refs, ",") != strings.Join(want, ",") {
		t.Errorf("the query tool returned %v, wanted exactly %v", refs, want)
	}
	// The florist's view over the tool carries the member the CLI's does.
	shown := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"show","arguments":{"card":"`+f+`","fields":"card"}}}`))
	detail, ok := shown["detail"].(map[string]any)
	if !ok {
		t.Fatalf("show over the tool carries no detail: %v", shown)
	}
	card, ok := detail["card"].(map[string]any)
	if !ok {
		t.Fatalf("show over the tool carries no card: %v", shown)
	}
	excluded, ok := card["inapplicable"].([]any)
	if !ok || len(excluded) != 1 {
		t.Fatalf("the florist's view lists %v as inapplicable", card["inapplicable"])
	}
	entry := excluded[0].(map[string]any)
	if entry["field"] != "vendor.deposit-required" || entry["gate"] != "event.category" || entry["gate_value"] != "florist" {
		t.Errorf("the florist's view carries %v", entry)
	}
}

// TestTheCheckToolCarriesTheNoticesTheCliCarries is the MCP half of
// dinah-590/criteria/14 and specification section 7.4: the check tool returns
// the report the library composed, so a workbench whose column requires a
// conditioned key answers over this head with the same notice the terminal's
// machine form carries, under outcome ok and empty findings.
//
// The tool's answer is compared against the library's own report as JSON,
// member for member, on the terms TestTheShowToolCarriesTheChecklistTheCliCarries
// sets, because a projection naming members one by one is exactly what
// dropped notices before this test existed, and a member added to the report
// later is caught the same way. The notice is then read off the tool's side
// on its own, so the comparison cannot pass over two answers that both carry
// nothing.
//
// Arming: rebuilding the answer from outcome and findings alone, as readCheck
// did before dinah-590, leaves the answer without a notices member and the
// first assertion fails.
func TestTheCheckToolCarriesTheNoticesTheCliCarries(t *testing.T) {
	library := newLibrary(t)
	anchor := filepath.Join(library.Bench.Root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.FieldsKey, bench.SplitLines(strings.TrimSuffix(weddingFields, "\n")))
	if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
	// The doing column requires the conditioned field, which is the
	// configuration the operator ruled legitimate and the one report with no
	// repair (dinah-590/questions/3).
	doing := library.Bench.ColumnByRef("doing")
	if doing == nil {
		t.Fatalf("the fixture declares no doing column")
	}
	columnAnchor := library.Bench.ColumnAnchorPath(doing.ID)
	text, err = bench.ReadText(columnAnchor)
	if err != nil {
		t.Fatalf("read the column anchor: %v", err)
	}
	fm, body = bench.ParseAnchor(text)
	fm.SetSeq(bench.RequireFieldsKey, []string{"vendor.deposit-required"})
	if err := bench.WriteText(columnAnchor, fm.Render(body)); err != nil {
		t.Fatalf("write the column anchor: %v", err)
	}
	opened, err := bench.Open(library.Bench.Root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	library.Bench = opened

	answer := payload(t, ask(t, library, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"check","arguments":{}}}`))
	notices, ok := answer["notices"].([]any)
	if !ok || len(notices) != 1 {
		t.Fatalf("the check tool's answer carries notices %v, wanted the one required-field notice: %v", answer["notices"], answer)
	}
	if notice, _ := notices[0].(map[string]any); notice["Key"] != bench.NoticeRequiredFieldConditioned {
		t.Errorf("the notice is %v, wanted %s", notices[0], bench.NoticeRequiredFieldConditioned)
	}
	if answer["outcome"] != contract.ReadOK {
		t.Errorf("a workbench whose only report is a notice answers outcome %v, wanted %q", answer["outcome"], contract.ReadOK)
	}
	if found, _ := answer["findings"].([]any); len(found) != 0 {
		t.Errorf("the notice is counted among the findings: %v", answer["findings"])
	}

	// Every member the terminal's machine form carries reaches the tool, and
	// the tool adds nothing but its affordances.
	delete(answer, "affordances")
	throughTool, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("marshal the tool's answer: %v", err)
	}
	direct, err := library.Check(&verb.Request{Verb: "check", Actor: "alka"})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	encoded, err := json.Marshal(direct)
	if err != nil {
		t.Fatalf("marshal the report: %v", err)
	}
	if !sameJSON(t, throughTool, encoded) {
		t.Errorf("the two heads disagree:\n tool %s\n  cli %s", throughTool, encoded)
	}
}
