package main

import (
	"strconv"
	"strings"
	"testing"

	"dinah/internal/msg"
)

// The checklist block draws each item as one row of two columns: its
// reference, state and owner packed into the first, and its own text in the
// second. The row sweep pairs both columns in eight locales, and this file
// holds the same association in one locale from the other side, reading the
// text off the row by cutting at the state and owner rather than at a column
// the sweep derives from the ink.
//
// Two guards live here. The first is the association: an item's text has to
// sit on the row bearing that item's own reference, since text drawn against
// the wrong row is exactly as wrong as text not drawn at all and a
// whole-output search cannot tell the two apart. The second is the absence
// the operator ruled for: a resolution note belongs to the payload and to the
// item's own detail view, and no part of it reaches this block.
func TestAChecklistItemsTextPrintsOnItsOwnRow(t *testing.T) {
	dir, ref := sweptChecklistTree(t, t.TempDir(), &sweptRecord{})
	got := runCLI(t, dir, "--lang", "en", "show", ref)
	if got.code != 0 {
		t.Fatalf("show %s: exit %d\n%s", ref, got.code, got.errw)
	}
	lines := strings.Split(strings.TrimSuffix(got.out, "\n"), "\n")

	// The reference is composed here the way a person types one, out of the
	// card's reference, the kind's short alias and the item's position among
	// the items of its own kind, which is how the sweep's own expectation
	// builds it. Composing it from the view's own accessor would let a wrong
	// reference agree with itself.
	aliases := map[string]string{"open_question": "oq", "acceptance_criterion": "ac", "decision": "d"}
	within := map[string]int{}
	refs := make([]string, len(sweptChecklistItems))
	at := make([]int, len(sweptChecklistItems))
	for i, item := range sweptChecklistItems {
		within[item.kind]++
		refs[i] = ref + "/" + aliases[item.kind] + "/" + strconv.Itoa(within[item.kind])
		at[i] = -1
		for n, line := range lines {
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0] == refs[i] {
				at[i] = n
				break
			}
		}
		if at[i] < 0 {
			t.Fatalf("the item %s draws no row carrying its reference %s\n%s", item.id, refs[i], got.out)
		}
	}

	for i, item := range sweptChecklistItems {
		// The row runs to the next row, so text that landed against a
		// neighbour is not counted here. Every line after the first belongs
		// to one of the two columns' continuations, which the join below
		// folds back into the text.
		end := len(lines)
		if i+1 < len(sweptChecklistItems) {
			end = at[i+1]
		}

		// The text begins where the packed column ends, and the packed
		// column ends at its own last two parts. Cutting there reads the
		// text off the row without deriving a column position out of the
		// ink the render produced, which is the sweep's job and would agree
		// with a render that had drifted.
		packed := item.state + " " + item.owner
		head := lines[at[i]]
		cut := strings.Index(head, packed)
		if cut < 0 {
			t.Errorf("the item %s draws the row %q, which carries neither its state nor its owner beside its reference, wanted %q",
				item.id, head, packed)
			continue
		}
		drawn := []string{strings.TrimSpace(head[cut+len(packed):])}
		for _, line := range lines[at[i]+1 : end] {
			if strings.TrimSpace(line) == "" {
				break
			}
			drawn = append(drawn, strings.TrimSpace(line))
		}
		text := strings.TrimSpace(strings.Join(drawn, " "))
		if text != item.text {
			t.Errorf("the item %s draws %q on the row %s, wanted its own text %q",
				item.id, text, refs[i], item.text)
		}
	}

	// The resolution note leaves this block entirely. It stays on the wire
	// for a machine reader and a person reaches it through the item's own
	// reference, which `dinah show <card>/<kind>/<n>` answers with.
	//
	// The search runs over the output with its whitespace collapsed, since a
	// note the block drew would be wrapped at the window and a search for the
	// note as it was written would miss it across the break.
	flattened := strings.Join(strings.Fields(got.out), " ")
	for _, item := range sweptChecklistItems {
		if item.note == "" {
			continue
		}
		if strings.Contains(flattened, strings.Join(strings.Fields(item.note), " ")) {
			t.Errorf("the item %s records the resolution note %q and the checklist block drew it:\n%s",
				item.id, item.note, got.out)
		}
	}
}

// TestAResolvedItemStillCarriesItsNoteWhereItIsRead is the other half of the
// rule above. Dropping the note from the listing is only a tidy-up as long as
// a person can still reach it, so this holds the path the ruling rests on:
// `dinah show <card>/<kind>/<n>` answers with the item, and the note it
// records is in that answer. Without this, the assertion above could be
// satisfied by a build that had made a written resolution unreadable.
func TestAResolvedItemStillCarriesItsNoteWhereItIsRead(t *testing.T) {
	dir, ref := sweptChecklistTree(t, t.TempDir(), &sweptRecord{})
	aliases := map[string]string{"open_question": "oq", "acceptance_criterion": "ac", "decision": "d"}
	within := map[string]int{}
	checked := 0
	for _, item := range sweptChecklistItems {
		within[item.kind]++
		if item.note == "" {
			continue
		}
		checked++
		at := ref + "/" + aliases[item.kind] + "/" + strconv.Itoa(within[item.kind])
		got := runCLI(t, dir, "--lang", "en", "show", at)
		if got.code != 0 {
			t.Fatalf("show %s: exit %d\n%s", at, got.code, got.errw)
		}
		if !strings.Contains(got.out, item.note) {
			t.Errorf("the item %s records the resolution note %q and `show %s` does not carry it:\n%s",
				item.id, item.note, at, got.out)
		}
	}
	if checked == 0 {
		t.Fatal("no item of the fixture records a resolution note, so this guard asserts nothing")
	}
}

// TestACardWithNoChecklistDrawsNoHeading holds the human half of dinah-435
// AC-6, whose other half (the payload carrying no checklist key at all) is
// held in internal/verb. Nothing observed the heading before this: the sweep
// only visits the block on the fixture that has items, so a build that drew
// the label over an empty table would have passed everything.
func TestACardWithNoChecklistDrawsNoHeading(t *testing.T) {
	dir, _ := sweptChecklistTree(t, t.TempDir(), &sweptRecord{})
	sweptDo(t, dir, "add", "A card with nothing to answer")
	bare := "ck-2"
	got := runCLI(t, dir, "--lang", "en", "show", bare)
	if got.code != 0 {
		t.Fatalf("show %s: exit %d\n%s", bare, got.code, got.errw)
	}
	heading := msg.For(msg.Base).T("show.checklist")
	for _, line := range strings.Split(got.out, "\n") {
		if strings.TrimSpace(line) == heading {
			t.Fatalf("the card %s carries no checklist and the render drew %q:\n%s", bare, heading, got.out)
		}
	}
}
