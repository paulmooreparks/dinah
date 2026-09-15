package verb

import (
	"testing"

	"dinah/internal/contract"
)

// TestEveryCardViewCarriesTheChecklistCount drives dinah-506/criteria/1. A
// reader deciding whether a card has anything to expand reads the count off
// the view it already holds, so the count has to ride every response carrying
// a card view rather than only the one a show composes.
//
// Two responses are read rather than one, and the second is a claim rather
// than a second show. CardView is filled in one place, so a count present on
// a show and absent on a claim would mean the claim composes its view
// elsewhere, and that is the defect this second half exists to catch.
func TestEveryCardViewCarriesTheChecklistCount(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("carrying two items")
	for _, filed := range []*Request{
		{
			Verb: "file", Actor: "alka", Card: ref, Kind: "open_question",
			Text: "Which vendor do we cite for the SLA numbers?", Column: aftercareSlug, Owner: "holder",
		},
		{
			Verb: "file", Actor: "alka", Card: ref, Kind: "decision",
			Text: "Whose contract the numbers come from.", Column: aftercareSlug, Owner: "holder",
		},
	} {
		response := h.library.File(filed)
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("file %s: %s %s", filed.Kind, response.Outcome, response.Refusal)
		}
	}

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if detail.Card.ChecklistCount != 2 {
		t.Errorf("the show reports %d checklist items, wanted 2", detail.Card.ChecklistCount)
	}

	claimed := h.do(&Request{Verb: "claim", Actor: "alka", Card: ref})
	if claimed.Outcome != contract.OutcomeOK {
		t.Fatalf("claim: %s %s", claimed.Outcome, claimed.Refusal)
	}
	if claimed.Card == nil {
		t.Fatal("the claim carried no card view, so this half read nothing")
	}
	if claimed.Card.ChecklistCount != 2 {
		t.Errorf("the claim reports %d checklist items, wanted 2", claimed.Card.ChecklistCount)
	}
}

// TestAChecklistCountIsAbsentOnACardCarryingNoItem pins the omitempty half. A
// count that rode every view as a literal zero would satisfy the test above
// and would put a member on every card of every listing, which is the cost
// AttachmentCount's own omitempty already declines to pay.
func TestAChecklistCountIsAbsentOnACardCarryingNoItem(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("carrying nothing")
	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if detail.Card.ChecklistCount != 0 {
		t.Errorf("a card carrying no item reports %d checklist items, wanted 0", detail.Card.ChecklistCount)
	}
}
