package bench

import "testing"

// TestChecklistKindsHandsBackAnAnswerNobodyCanWriteThrough asserts what this
// package promises about ChecklistKinds, which is not that the value it
// returns cannot be written to. Go has no immutable slice, and a promise a
// language cannot keep is worth nothing on a contract. What is promised is
// that writing to the answer reaches nothing: not the declaration, not a later
// caller's answer, and not a reference this package resolves.
//
// The wrong implementation this is armed against is the one-word change that
// returns the package's own slice instead of a copy of it. Under that
// implementation the sort below reorders the declaration itself, and both
// halves of this test fail: the second answer comes back in the sorted order,
// and every reference resolving through the declaration follows it.
func TestChecklistKindsHandsBackAnAnswerNobodyCanWriteThrough(t *testing.T) {
	first := ChecklistKinds()
	if len(first) < 3 {
		t.Fatalf("the declaration answers %d kinds, and a test of its order needs at least three", len(first))
	}
	before := make([]ChecklistKind, len(first))
	copy(before, first)

	// A caller doing the most ordinary thing a caller does with a slice.
	for i, j := 0, len(first)-1; i < j; i, j = i+1, j-1 {
		first[i], first[j] = first[j], first[i]
	}
	first[0].Word = "trampled"

	second := ChecklistKinds()
	if len(second) != len(before) {
		t.Fatalf("the second answer carries %d kinds where the first carried %d", len(second), len(before))
	}
	for i := range before {
		if second[i] != before[i] {
			t.Errorf("kind %d came back as %+v after a caller wrote to an earlier answer, and it was %+v",
				i, second[i], before[i])
		}
	}

	// The other half of the same promise. The declaration is what composes a
	// reference, so a caller who trampled its answer must not have moved the
	// word any item composes under.
	for _, kind := range before {
		word, ok := WordForItemKind(kind.Kind)
		if !ok {
			t.Errorf("%s composes no reference word after a caller wrote to an answer", kind.Kind)
			continue
		}
		if word != kind.Word {
			t.Errorf("%s composes under %q after a caller wrote to an answer, and it composed under %q",
				kind.Kind, word, kind.Word)
		}
	}
}

// TestChecklistKindsIsTheDeclarationItself asserts that the accessor reads the
// declaration rather than restating it, which is what makes an edit to
// checklistSegments reach the projection. A second list written out by hand
// here would be the drift this guards against, so the comparison is against
// checklistSegments itself.
func TestChecklistKindsIsTheDeclarationItself(t *testing.T) {
	kinds := ChecklistKinds()
	if len(kinds) != len(checklistSegments) {
		t.Fatalf("the accessor answers %d kinds and the declaration holds %d", len(kinds), len(checklistSegments))
	}
	for i, segment := range checklistSegments {
		if kinds[i].Kind != segment.Kind || kinds[i].Word != segment.Word {
			t.Errorf("kind %d is %+v and the declaration holds %s/%s",
				i, kinds[i], segment.Kind, segment.Word)
		}
	}
}
