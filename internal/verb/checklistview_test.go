package verb

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// TestShowCarriesEveryChecklistItemTheCardHolds drives dinah-435 AC-1, AC-2
// and AC-5. A card's items reach a reader in creation order, each carrying
// what it says as well as the fact that it exists, and the count the listing
// already published is left where it was.
//
// The reference is what makes the row worth printing, so it is asserted as a
// string rather than as "not empty": a reference composed from the overall
// ordinal rather than from the position within the kind would still be
// non-empty and would still name a different item.
func TestShowCarriesEveryChecklistItemTheCardHolds(t *testing.T) {
	const answer = "the operator confirmed it's Acme per the 2026-08 contract"
	h := newHarness(t)
	ref := h.ready("carrying a checklist")
	h.item(ref, "b00000000001", "kind: open_question\nstate: pending\nowner: operator\nordinal: 1\n",
		"Which vendor do we cite for the SLA numbers?")
	h.item(ref, "b00000000002", "kind: acceptance_criterion\nstate: pending\nordinal: 2\n",
		"The endpoint returns 404 for an unknown id.")
	settled := h.item(ref, "b00000000003", "kind: decision\nstate: resolved\nordinal: 3\nresolution: "+ref+"/decisions/1/comments/1\n",
		"Whose contract the numbers come from.")
	h.plantComment(settled, "c00000000001", 1, "alka", answer)

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, Fields: "card,checklist"})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	wanted := []ItemView{
		{
			ID: "b00000000001", Ordinal: 1, Ref: ref + "/questions/1", Kind: "open_question",
			State: "pending", Owner: "operator", Text: "Which vendor do we cite for the SLA numbers?",
		},
		{
			ID: "b00000000002", Ordinal: 2, Ref: ref + "/criteria/1", Kind: "acceptance_criterion",
			State: "pending", Text: "The endpoint returns 404 for an unknown id.",
		},
		{
			ID: "b00000000003", Ordinal: 3, Ref: ref + "/decisions/1", Kind: "decision",
			State: "resolved", Text: "Whose contract the numbers come from.",
			Resolution: ref + "/decisions/1/comments/1", CommentCount: 1,
		},
	}
	if len(detail.Checklist) != len(wanted) {
		t.Fatalf("wanted %d items, got %d: %+v", len(wanted), len(detail.Checklist), detail.Checklist)
	}
	for i, want := range wanted {
		if detail.Checklist[i] != want {
			t.Errorf("item %d:\n got %+v\nwant %+v", i+1, detail.Checklist[i], want)
		}
	}
	// The count the many-card reads publish is the half this card leaves
	// alone, and a criterion never blocks, so one of the three is the answer.
	if detail.Card.BlockingItems != 1 {
		t.Errorf("the card's blocking count is %d, wanted 1", detail.Card.BlockingItems)
	}

	// The designated comment is the one member the unshaped read stopped
	// carrying, so the same three items are read again in full and the
	// comment is what the second read is for. The reference is carried by
	// both reads, because it costs the item's own anchor and nothing
	// further; the comment itself costs a file open and is carried by the
	// full read alone. Asserting both here rather than in a file of their
	// own keeps the index and the full form beside each other, where a
	// reader comparing the two reads one fixture.
	whole, _, _, _, err := h.library.Show(&Request{
		Verb: "show", Actor: "alka", Card: ref, Fields: "checklist.full",
	})
	if err != nil {
		t.Fatalf("show in full: %v", err)
	}
	if len(whole.Checklist) != len(wanted) {
		t.Fatalf("wanted %d items in full, got %d", len(wanted), len(whole.Checklist))
	}
	designated := whole.Checklist[2].Designated
	if designated == nil {
		t.Fatalf("checklist.full carried no designated comment for the settled item")
	}
	if designated.Body != answer {
		t.Errorf("checklist.full carries the answer %q, wanted %q", designated.Body, answer)
	}
	if designated.Author != "alka" {
		t.Errorf("the designated comment's author is %q, wanted alka", designated.Author)
	}
	opened := 0
	for _, item := range whole.Checklist {
		if item.Designated != nil {
			opened++
		}
	}
	if opened != 1 {
		t.Errorf("one of the three items is settled and checklist.full opened %d comments", opened)
	}
	// The indexed read carried the same reference and opened nothing, which
	// is the property the two reads part company on.
	for _, item := range detail.Checklist {
		if item.Designated != nil {
			t.Errorf("the indexed read opened the designated comment of %s", item.Ref)
		}
	}
	for _, name := range whole.Withheld {
		if name == "checklist" || name == "checklist.full" {
			t.Errorf("checklist.full announced %s, and it carried every item and every answer: %v",
				name, whole.Withheld)
		}
	}
}

// TestAChecklistPayloadOmitsWhatTheItemNeverCarried drives dinah-435 AC-2's
// second half and AC-6. A key present holding nothing is a claim about the
// item, and it is the claim this asserts against: an item filed with no owner
// and no note reaches the wire carrying neither name, and a card with no
// checklist at all carries no member rather than an empty one.
func TestAChecklistPayloadOmitsWhatTheItemNeverCarried(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("carrying one bare item")
	h.item(ref, "b00000000001", "kind: acceptance_criterion\nstate: pending\nordinal: 1\n",
		"The endpoint returns 404 for an unknown id.")
	bare := h.ready("carrying nothing at all")

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, Fields: "checklist"})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload struct {
		Checklist []map[string]json.RawMessage `json:"checklist"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, encoded)
	}
	if len(payload.Checklist) != 1 {
		t.Fatalf("wanted one item on the wire, got %d: %s", len(payload.Checklist), encoded)
	}
	for _, absent := range []string{"owner", "note", "column", "column_title"} {
		if raw, ok := payload.Checklist[0][absent]; ok {
			t.Errorf("the item carries no %s and the payload writes it as %s: %s", absent, raw, encoded)
		}
	}
	for _, present := range []string{"id", "ordinal", "ref", "kind", "state", "text"} {
		if _, ok := payload.Checklist[0][present]; !ok {
			t.Errorf("the payload drops %s, which every item carries: %s", present, encoded)
		}
	}

	empty, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: bare})
	if err != nil {
		t.Fatalf("show the bare card: %v", err)
	}
	encoded, err = json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal the bare card: %v", err)
	}
	var whole map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &whole); err != nil {
		t.Fatalf("decode the bare card: %v\n%s", err, encoded)
	}
	if raw, ok := whole["checklist"]; ok {
		t.Errorf("a card with no checklist directory reports checklist as %s: %s", raw, encoded)
	}
}

// TestAChecklistItemsReferenceResolvesToThatItem drives dinah-435 AC-4. The
// reference a row prints is composed here for the first time and resolved by
// machinery written before it, so the two are checked against each other
// rather than each against its own idea of the spelling: every reference the
// view composed is resolved, and the file it reaches has to be the one the
// same entry named by identifier.
func TestAChecklistItemsReferenceResolvesToThatItem(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("carrying one of each kind")
	planted := map[string]string{}
	for _, row := range []struct {
		id          string
		frontmatter string
		text        string
	}{
		{"b00000000001", "kind: decision\nstate: resolved\nordinal: 1\n", "The first decision."},
		{"b00000000002", "kind: open_question\nstate: resolved\nordinal: 2\n", "The first question."},
		{"b00000000003", "kind: acceptance_criterion\nstate: pending\nordinal: 3\n", "The first criterion."},
		{"b00000000004", "kind: open_question\nstate: resolved\nordinal: 4\n", "The second question."},
	} {
		planted[row.id] = h.item(ref, row.id, row.frontmatter, row.text)
	}

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, Fields: "checklist"})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if len(detail.Checklist) != len(planted) {
		t.Fatalf("wanted %d items, got %d", len(planted), len(detail.Checklist))
	}
	for _, item := range detail.Checklist {
		resolved, err := h.library.Bench.ResolvePath(item.Ref)
		if err != nil {
			t.Errorf("%s: the reference the view composed resolves to nothing: %v", item.Ref, err)
			continue
		}
		wanted := planted[item.ID]
		if got := filepath.Dir(resolved); got != wanted {
			t.Errorf("%s names the item %s and resolves to %s, wanted %s", item.Ref, item.ID, got, wanted)
		}
	}
}
