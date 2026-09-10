package bench

import (
	"os"
	"path/filepath"
	"testing"
)

// TestItemsReadsACardsChecklistInCreationOrder drives the loader dinah-435
// adds. The identifiers are planted out of ordinal order deliberately: a
// loader reading the directory listing rather than the ordinal would come back
// with these three in the other order and would still come back with all
// three, so the order is the assertion rather than the count.
func TestItemsReadsACardsChecklistInCreationOrder(t *testing.T) {
	card := t.TempDir()
	plantChecklistItem(t, card, "b00000000003", "kind: open_question\nstate: pending\nordinal: 1\n", "The first question.")
	plantChecklistItem(t, card, "b00000000001", "kind: decision\nstate: resolved\nordinal: 2\nnote: it is Acme\n", "Whose contract.")
	plantChecklistItem(t, card, "b00000000002", "kind: acceptance_criterion\nstate: pending\ncolumn: doing\nowner: holder\nordinal: 3\n", "The endpoint answers 404.")

	items, err := Items(card)
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("wanted three items, got %d", len(items))
	}
	wanted := []Item{
		{ID: "b00000000003", Kind: "open_question", State: "pending", Ordinal: 1, Text: "The first question."},
		{ID: "b00000000001", Kind: "decision", State: "resolved", Ordinal: 2, Note: "it is Acme", Text: "Whose contract."},
		{ID: "b00000000002", Kind: "acceptance_criterion", State: "pending", Ordinal: 3, Column: "doing", Owner: "holder", Text: "The endpoint answers 404."},
	}
	for i, want := range wanted {
		got := items[i]
		if got.ID != want.ID {
			t.Errorf("item %d is %s, wanted %s", i+1, got.ID, want.ID)
		}
		if got.Kind != want.Kind || got.State != want.State || got.Ordinal != want.Ordinal {
			t.Errorf("item %s: kind %q state %q ordinal %d, wanted %q %q %d",
				got.ID, got.Kind, got.State, got.Ordinal, want.Kind, want.State, want.Ordinal)
		}
		if got.Column != want.Column || got.Owner != want.Owner || got.Note != want.Note {
			t.Errorf("item %s: column %q owner %q note %q, wanted %q %q %q",
				got.ID, got.Column, got.Owner, got.Note, want.Column, want.Owner, want.Note)
		}
		// The trailing newline every text file ends in is trimmed, so the
		// comparison is against the sentence rather than against the file.
		if got.Text != want.Text {
			t.Errorf("item %s carries text %q, wanted %q", got.ID, got.Text, want.Text)
		}
	}
}

// TestItemsSkipsAnItemWhoseAnchorWillNotOpen holds the loader to the tolerance
// BlockingItems already has. An unreadable file is a defect dinah check
// reports rather than one a read discovers, so the readable items still come
// back and the read does not fail.
func TestItemsSkipsAnItemWhoseAnchorWillNotOpen(t *testing.T) {
	card := t.TempDir()
	plantChecklistItem(t, card, "b00000000001", "kind: open_question\nstate: pending\nordinal: 1\n", "The question.")
	if err := os.MkdirAll(filepath.Join(card, ChecklistDir, "b00000000002"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	items, err := Items(card)
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if len(items) != 1 || items[0].ID != "b00000000001" {
		t.Fatalf("wanted the one readable item, got %+v", items)
	}
}

// TestACardWithNoChecklistReadsAsNoItems asserts the emptiness a read reports
// as absence rather than as an empty collection.
func TestACardWithNoChecklistReadsAsNoItems(t *testing.T) {
	items, err := Items(t.TempDir())
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("wanted nothing, got %+v", items)
	}
}

// TestWordForItemKindAgreesWithWhatAReferenceResolvesBy asserts dinah-454
// AC-14: the segment a reference is composed from is a segment a reference
// resolves by, and it is the word rather than the short form. Both directions
// are derived from one declaration, and this is what would catch a second
// literal being introduced beside it.
//
// The last two arms carry the weight. Every segment the resolver knows has to
// come from that declaration, so a resolver quietly widened to accept a fourth
// spelling fails here rather than passing as a convenience, and a kind the
// format does not declare composes nothing rather than composing a reference
// that resolves to no item.
func TestWordForItemKindAgreesWithWhatAReferenceResolvesBy(t *testing.T) {
	declared := map[string]bool{}
	for _, segment := range checklistSegments {
		declared[segment.Word] = true
		declared[segment.Short] = true
		if segment.Word == segment.Short {
			t.Errorf("%s declares one spelling twice, so nothing distinguishes the word from the short form", segment.Kind)
		}
		for _, spelling := range []string{segment.Word, segment.Short} {
			if got := checklistKinds[spelling]; got != segment.Kind {
				t.Errorf("the segment %q resolves to %q, wanted %q", spelling, got, segment.Kind)
			}
		}
		got, ok := WordForItemKind(segment.Kind)
		if !ok {
			t.Errorf("the resolver reaches %s and nothing composes it back", segment.Kind)
			continue
		}
		if got != segment.Word {
			t.Errorf("%s composes to %q and its declared word is %q", segment.Kind, got, segment.Word)
		}
	}
	for spelling := range checklistKinds {
		if !declared[spelling] {
			t.Errorf("the resolver accepts the segment %q, which no entry of checklistSegments declares", spelling)
		}
	}
	if word, ok := WordForItemKind("risk"); ok {
		t.Errorf("a kind the format does not declare composed the segment %q", word)
	}
}

// plantChecklistItem writes one checklist item by hand, which is how a card in
// this package's tests comes to carry one. `dinah file` lives a layer up in
// internal/verb, which imports this package and cannot be called from it, and
// the format is a tree of plain files a person is meant to be able to edit, so
// writing the anchor is the whole act.
func plantChecklistItem(t *testing.T, cardDir, id, frontmatter, text string) {
	t.Helper()
	path := filepath.Join(cardDir, ChecklistDir, id, ItemAnchor)
	if err := WriteText(path, "---\n"+frontmatter+"---\n"+text+"\n"); err != nil {
		t.Fatalf("write %s: %v", id, err)
	}
}
