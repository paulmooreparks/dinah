package main

import (
	"strconv"
	"strings"
	"testing"

	"dinah/internal/msg"
)

// The checklist block draws two things, and the row sweep can only see one of
// them. A sweep entry harvests the cells of a row and pairs them against an
// expected row, so it holds the reference, the kind, the state and the owner
// in every locale. An item's own text is not a cell. It is prose printed
// beneath the row, and a resolution note is a second line of prose beneath
// that, so both fall outside everything the sweep collects: deleting every
// item's text from the render leaves the sweep green, which is how the shipped
// build reached review with its headline behaviour unguarded.
//
// This is the guard for that half. It runs one locale, because the prose is
// the item's own text rather than catalog wording and the sweep already holds
// the wording in eight. What it asserts is the association rather than the
// presence: each item's text has to sit under the row bearing that item's own
// reference, since text printed under the wrong row is exactly as wrong as
// text not printed at all and a whole-output search cannot tell them apart.
func TestAChecklistItemsTextPrintsUnderItsOwnRow(t *testing.T) {
	dir, ref := sweptChecklistTree(t, t.TempDir(), &sweptRecord{})
	got := runCLI(t, dir, "--lang", "en", "show", ref)
	if got.code != 0 {
		t.Fatalf("show %s: exit %d\n%s", ref, got.code, got.errw)
	}
	resolution := msg.For(msg.Base).T("show.checklist.resolution")
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
		// The block belonging to this row runs from the row to the next row,
		// so prose that landed under a neighbour is not counted here.
		end := len(lines)
		if i+1 < len(sweptChecklistItems) {
			end = at[i+1]
		}
		block := lines[at[i]+1 : end]

		text := -1
		for n, line := range block {
			if strings.TrimSpace(line) == item.text {
				text = n
				break
			}
		}
		if text < 0 {
			t.Errorf("the item %s prints no line carrying its text %q under the row %s, which drew:\n%s",
				item.id, item.text, refs[i], strings.Join(block, "\n"))
			continue
		}

		wanted := resolution + " " + item.note
		found := -1
		for n, line := range block {
			if strings.HasPrefix(strings.TrimSpace(line), resolution) {
				found = n
				break
			}
		}
		if item.note == "" {
			if found >= 0 {
				t.Errorf("the item %s is %s and carries no resolution note, but the row %s drew %q under it",
					item.id, item.state, refs[i], strings.TrimSpace(block[found]))
			}
			continue
		}
		if found < 0 {
			t.Errorf("the item %s records the resolution note %q and the row %s drew no %s line, only:\n%s",
				item.id, item.note, refs[i], resolution, strings.Join(block, "\n"))
			continue
		}
		if strings.TrimSpace(block[found]) != wanted {
			t.Errorf("the item %s draws its resolution as %q under the row %s, wanted %q",
				item.id, strings.TrimSpace(block[found]), refs[i], wanted)
		}
		if found < text {
			t.Errorf("the item %s draws its resolution above its own text under the row %s, which drew:\n%s",
				item.id, refs[i], strings.Join(block, "\n"))
		}
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
