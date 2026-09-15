package verb

import (
	"encoding/json"
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
//
// What is read is the marshalled bytes and not the Go field. The field is zero
// on a card carrying nothing whether the tag says omitempty or not, so a test
// reading the field reports on the count and reports nothing at all on the tag
// it says it pins. The first form of this test did exactly that: dropping
// omitempty left it green, which is how it was caught.
//
// AttachmentCount is read beside it as the control. It carries the same tag,
// so a marshaller that stopped honouring omitempty at all would show up here
// as both keys appearing rather than as one, and a reader meeting a failure
// can tell a tag that was edited from a marshaller that changed under it.
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
	encoded, err := json.Marshal(detail.Card)
	if err != nil {
		t.Fatalf("marshal the card view: %v", err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &members); err != nil {
		t.Fatalf("read the card view back: %v", err)
	}
	if _, present := members["ref"]; !present {
		t.Fatal("the marshalled view carries no ref, so this test read the wrong object")
	}
	if _, present := members["checklist_count"]; present {
		t.Errorf("a card carrying no item publishes checklist_count: %s", encoded)
	}
	if _, present := members["attachment_count"]; present {
		t.Errorf("a card carrying no attachment publishes attachment_count: %s", encoded)
	}
}
