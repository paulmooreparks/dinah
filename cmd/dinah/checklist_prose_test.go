package main

import (
	"strconv"
	"strings"
	"testing"

	"dinah/internal/msg"
)

// The checklist block draws each item as one row of four columns: its
// reference, its state, whoever answers it, and its own text last. The row
// sweep pairs all four in eight locales, and this file holds the same
// association in one locale from the other side, walking the row's own values
// in order rather than reading a column position off the ink.
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
	// card's reference, the kind's word and the item's position among
	// the items of its own kind, which is how the sweep's own expectation
	// builds it. Composing it from the view's own accessor would let a wrong
	// reference agree with itself.
	words := map[string]string{"open_question": "questions", "acceptance_criterion": "criteria", "decision": "decisions"}
	within := map[string]int{}
	refs := make([]string, len(sweptChecklistItems))
	at := make([]int, len(sweptChecklistItems))
	for i, item := range sweptChecklistItems {
		within[item.kind]++
		refs[i] = ref + "/" + words[item.kind] + "/" + strconv.Itoa(within[item.kind])
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

		// The row's own values are walked in order, each found after the
		// one before it with nothing but padding between them, and the text
		// is whatever the row has left after the owner. Walking the values
		// reads the text off the row without deriving a column position out
		// of the ink the render produced, which is the sweep's job and would
		// agree with a render that had drifted. A render packing the three
		// leading values back into one field still passes here, and fails in
		// the sweep, which is what holds the columns to lining up.
		head := lines[at[i]]
		cut, ok := afterTheRowsOwnValues(head, refs[i], item.state, item.owner)
		if !ok {
			t.Errorf("the item %s draws the row %q, which does not carry its reference, its state %q and its owner %q in that order separated by padding",
				item.id, head, item.state, item.owner)
			continue
		}
		drawn := []string{strings.TrimSpace(head[cut:])}
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

// afterTheRowsOwnValues reports where a row's last column begins: the byte
// after the last of the values given, each of which has to follow the one
// before it with nothing but spaces between them. It reports false when any
// value is missing or arrives out of order, which is what a row drawing its
// cells in the wrong places does.
//
// The values are given rather than read off the line, so a row that drew a
// neighbour's state or dropped its owner fails here rather than being cut at
// whatever it did draw.
func afterTheRowsOwnValues(line string, values ...string) (int, bool) {
	cursor := 0
	for _, value := range values {
		at := strings.Index(line[cursor:], value)
		if at < 0 {
			return 0, false
		}
		if strings.TrimLeft(line[cursor:cursor+at], " ") != "" {
			return 0, false
		}
		cursor += at + len(value)
	}
	return cursor, true
}

// TestAResolvedItemStillCarriesItsNoteWhereItIsRead is the other half of the
// rule above. Dropping the note from the listing is only a tidy-up as long as
// a person can still reach it, so this holds the path the ruling rests on:
// `dinah show <card>/<kind>/<n>` answers with the item, and the note it
// records is in that answer. Without this, the assertion above could be
// satisfied by a build that had made a written resolution unreadable.
func TestAResolvedItemStillCarriesItsNoteWhereItIsRead(t *testing.T) {
	dir, ref := sweptChecklistTree(t, t.TempDir(), &sweptRecord{})
	words := map[string]string{"open_question": "questions", "acceptance_criterion": "criteria", "decision": "decisions"}
	within := map[string]int{}
	checked := 0
	for _, item := range sweptChecklistItems {
		within[item.kind]++
		if item.note == "" {
			continue
		}
		checked++
		at := ref + "/" + words[item.kind] + "/" + strconv.Itoa(within[item.kind])
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
