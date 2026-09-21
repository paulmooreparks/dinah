package verb

import (
	"encoding/json"
	"strconv"
	"testing"

	"dinah/internal/contract"
)

// TestBareShowServesTheNarrowShapeOnALargeCard drives dinah-543/criteria/1.
// A card holding hundreds of comments and checklist items, read bare, still
// carries only the four members every other test in this file's package
// already treats as the narrow default. The byte-size assertion is the one
// this criterion is written for: a leaked index would grow with the fixture,
// and a fixed card holding this many rows would fail the bound below within
// the first few dozen entries if either collection leaked in even as an
// index.
func TestBareShowServesTheNarrowShapeOnALargeCard(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("a card carrying hundreds of things below it")
	const count = 150
	for i := 0; i < count; i++ {
		h.comment(ref, "comment number "+strconv.Itoa(i))
	}
	for i := 0; i < count; i++ {
		if response := h.library.File(&Request{
			Verb: "file", Actor: "alka", Card: ref, Kind: "acceptance_criterion",
			Text: "checklist item number " + strconv.Itoa(i),
		}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("file item %d: %s %s", i, response.Outcome, response.Refusal)
		}
	}
	h.reopen()

	detail, _, _, text, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if text != "" {
		t.Fatalf("a card came back as text: %q", text)
	}
	if detail.Card.Ref != ref {
		t.Errorf("wanted the card member, got %+v", detail.Card)
	}
	if len(detail.Comments) != 0 || len(detail.Checklist) != 0 {
		t.Errorf("the bare answer carried a collection the narrow default leaves out: %d comments, %d checklist items",
			len(detail.Comments), len(detail.Checklist))
	}
	if want := []string{"comments", "checklist", "path"}; !stringSlicesEqual(detail.Withheld, want) {
		t.Errorf("wanted withheld %v, got %v", want, detail.Withheld)
	}

	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// 150 comments and 150 checklist items would run to tens of kilobytes even
	// carried as a stripped index (roughly 444 bytes per checklist row alone,
	// per docs/design/token-cost.md's own measurement on dinah-514's 65-item
	// checklist), so a bound of 8KB is well clear of the narrow answer's real
	// size and would be blown through immediately by a leak of either
	// collection.
	const bound = 8 * 1024
	if len(encoded) >= bound {
		t.Errorf("the bare answer is %d bytes, wanted under %d: a collection leaked into the default", len(encoded), bound)
	}
}

// stringSlicesEqual compares two string slices for this file's own
// assertions, since reflect.DeepEqual on a nil versus an empty slice is a
// distinction none of these criteria are about.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestAllServesEveryMember drives dinah-543/criteria/2: --all carries every
// member, comment bodies and checklist designated comments are non-empty,
// and Withheld is empty.
func TestAllServesEveryMember(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("a card carrying one of each thing")
	h.comment(ref, "a remark worth keeping")
	settled := h.item(ref, "b00000000001", "kind: decision\nstate: resolved\nordinal: 1\nresolution: "+ref+"/decisions/1/comments/1\n",
		"Whose contract the numbers come from.")
	h.plantComment(settled, "c00000000001", 1, "alka", "the operator ruled it")

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, All: true})
	if err != nil {
		t.Fatalf("show --all: %v", err)
	}
	if len(detail.Comments) != 1 || detail.Comments[0].Body == "" {
		t.Fatalf("wanted one comment with a body, got %+v", detail.Comments)
	}
	if len(detail.Checklist) != 1 || detail.Checklist[0].Designated == nil {
		t.Fatalf("wanted one checklist item carrying its designated comment, got %+v", detail.Checklist)
	}
	if detail.Checklist[0].Designated.Body == "" {
		t.Errorf("the designated comment carries no body")
	}
	if len(detail.Withheld) != 0 {
		t.Errorf("--all withheld %v, wanted nothing", detail.Withheld)
	}
}

// TestAllConflictsWithFields drives dinah-543/criteria/3: --all --fields
// card is refused dinah.usage, and beside it, --all alone and --fields card
// alone both still answer.
func TestAllConflictsWithFields(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("a card")

	_, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, All: true, Fields: "card"})
	if err == nil {
		t.Fatal("wanted a refusal, got none")
	}
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %T %v", err, err)
	}
	if refusal.Name != contract.Usage {
		t.Fatalf("wanted %s, got %s", contract.Usage, refusal.Name)
	}
	if want := "--all conflicts with --fields"; refusal.Detail != want {
		t.Errorf("wanted the detail %q, got %q", want, refusal.Detail)
	}

	if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, All: true}); err != nil {
		t.Errorf("--all alone was refused: %v", err)
	}
	if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, Fields: "card"}); err != nil {
		t.Errorf("--fields card alone was refused: %v", err)
	}
}

// TestAllIsRefusedOnEveryNonCardReference drives dinah-543/criteria/4: --all
// on the workbench reference, a workstream reference, a bare column
// reference and a composed reference (an attachment's own reference stands
// for the whole class) is refused dinah.usage, detail --all, on each; beside
// each, the same reference read bare still answers exactly as today.
func TestAllIsRefusedOnEveryNonCardReference(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("a card carrying an attachment")
	h.attach(ref, "note.txt", "bytes")
	if response := h.library.NewWorkstream(&Request{
		Verb: "workstream", Action: "new", Actor: "alka", Workstream: "Autumn release", Slug: "autumn",
	}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("new workstream: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	for _, row := range []struct {
		name string
		ref  string
	}{
		{name: "the workbench reference", ref: "workbench"},
		{name: "a workstream reference", ref: "workstream/autumn"},
		{name: "a bare column reference", ref: aftercare},
		{name: "a composed reference", ref: ref + "/attachments/1"},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: row.ref, All: true})
			if err == nil {
				t.Fatal("wanted a refusal, got none")
			}
			refusal, ok := err.(*contract.Refusal)
			if !ok {
				t.Fatalf("wanted a refusal, got %T %v", err, err)
			}
			if refusal.Name != contract.Usage {
				t.Fatalf("wanted %s, got %s", contract.Usage, refusal.Name)
			}
			if refusal.Detail != flagAll {
				t.Errorf("wanted the detail %q, got %q", flagAll, refusal.Detail)
			}

			if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: row.ref}); err != nil {
				t.Errorf("the same reference read bare was refused: %v", err)
			}
		})
	}
}

// TestSinceAloneWithNoFieldsIsRefused drives dinah-543/criteria/5 and
// decision 3: --since 1 with no --fields is refused dinah.usage, and beside
// it, --fields card,comments --since 1 answers with the bodies past the
// ordinal filled and the rest indexed.
func TestSinceAloneWithNoFieldsIsRefused(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("a card carrying two comments")
	h.comment(ref, "the first remark")
	h.comment(ref, "the second remark")

	_, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, SinceComment: "1"})
	if err == nil {
		t.Fatal("wanted a refusal, got none")
	}
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %T %v", err, err)
	}
	if refusal.Name != contract.Usage {
		t.Fatalf("wanted %s, got %s", contract.Usage, refusal.Name)
	}
	if refusal.Detail != flagSince {
		t.Errorf("wanted the detail %q, got %q", flagSince, refusal.Detail)
	}

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, Fields: "card,comments", SinceComment: "1"})
	if err != nil {
		t.Fatalf("the recovery call was refused: %v", err)
	}
	if len(detail.Comments) != 2 {
		t.Fatalf("wanted two comments, got %d", len(detail.Comments))
	}
	if detail.Comments[0].Body != "" {
		t.Errorf("comment 1 carries a body, and --since 1 serves the bodies past ordinal 1: %+v", detail.Comments[0])
	}
	if detail.Comments[1].Body == "" {
		t.Errorf("comment 2 carries no body, and --since 1 serves it")
	}
}

// TestUnresolvedAloneWithNoFieldsIsRefused drives dinah-543/criteria/6:
// --unresolved with no --fields is refused dinah.usage, and beside it,
// --fields card,checklist --unresolved answers with only the unresolved
// items.
func TestUnresolvedAloneWithNoFieldsIsRefused(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("a card carrying an open item and a settled one")
	h.item(ref, "b00000000001", "kind: open_question\nstate: pending\nordinal: 1\n", "still open")
	h.item(ref, "b00000000002", "kind: decision\nstate: resolved\nordinal: 2\n", "already settled")

	_, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, Unresolved: true})
	if err == nil {
		t.Fatal("wanted a refusal, got none")
	}
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %T %v", err, err)
	}
	if refusal.Name != contract.Usage {
		t.Fatalf("wanted %s, got %s", contract.Usage, refusal.Name)
	}
	if refusal.Detail != flagUnresolved {
		t.Errorf("wanted the detail %q, got %q", flagUnresolved, refusal.Detail)
	}

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, Fields: "card,checklist", Unresolved: true})
	if err != nil {
		t.Fatalf("the recovery call was refused: %v", err)
	}
	if len(detail.Checklist) != 1 || detail.Checklist[0].ID != "b00000000001" {
		t.Fatalf("wanted only the one unresolved item, got %+v", detail.Checklist)
	}
}

// TestAllBesideSinceIsRefusedAllBesideUnresolvedAnswers drives
// dinah-543/criteria/7: --all --since 1 is refused with the same composed
// detail read.go already uses for --fields comments.full --since; beside
// it, --all --unresolved answers, carrying every member with the checklist
// narrowed to the unresolved items and Withheld naming checklist alone.
func TestAllBesideSinceIsRefusedAllBesideUnresolvedAnswers(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("a card carrying comments and a mixed checklist")
	h.comment(ref, "a remark")
	h.item(ref, "b00000000001", "kind: open_question\nstate: pending\nordinal: 1\n", "still open")
	h.item(ref, "b00000000002", "kind: decision\nstate: resolved\nordinal: 2\n", "already settled")

	_, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, All: true, SinceComment: "1"})
	if err == nil {
		t.Fatal("wanted a refusal, got none")
	}
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %T %v", err, err)
	}
	if refusal.Name != contract.Usage {
		t.Fatalf("wanted %s, got %s", contract.Usage, refusal.Name)
	}
	if want := flagSince + " beside " + modifierFor("comments"); refusal.Detail != want {
		t.Errorf("wanted the detail %q, got %q", want, refusal.Detail)
	}

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, All: true, Unresolved: true})
	if err != nil {
		t.Fatalf("--all --unresolved was refused: %v", err)
	}
	if detail.Comments[0].Body == "" {
		t.Errorf("--all --unresolved carried no comment body, and --all names every member in full")
	}
	if len(detail.Checklist) != 1 || detail.Checklist[0].ID != "b00000000001" {
		t.Fatalf("wanted only the one unresolved item, got %+v", detail.Checklist)
	}
	if want := []string{"checklist"}; !stringSlicesEqual(detail.Withheld, want) {
		t.Errorf("wanted withheld %v, got %v", want, detail.Withheld)
	}
}
