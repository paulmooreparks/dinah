package bench

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// childCountsFixture is a card carrying two comments, three checklist items
// and one attachment, which is the shape dinah-519 criteria/12 names: one
// mount holding more than one member, one holding a different number, and one
// holding a single member, so a sum that dropped a mount cannot agree with the
// sum that keeps it.
func childCountsFixture(t *testing.T) string {
	t.Helper()
	root := newFixture(t)
	writeComment(t, root, "f00000000001", "2026-09-15T09:00:00Z", 1, "The first thought")
	writeComment(t, root, "f00000000002", "2026-09-15T09:01:00Z", 2, "The second thought")
	writeItem(t, root, "d00000000001", 1)
	writeItem(t, root, "d00000000002", 2)
	writeItem(t, root, "d00000000003", 3)
	writeAttachment(t, root, "e00000000001", 1)
	return root
}

// TestChildCountsAnswersOneEntryPerMountAndSumsThem asserts dinah-519
// criteria/12's first half. The entry count is asserted beside the sum,
// because a walk that read no mount at all answers an empty map summing to
// zero, which is what a card holding nothing also answers.
func TestChildCountsAnswersOneEntryPerMountAndSumsThem(t *testing.T) {
	root := childCountsFixture(t)
	card := filepath.Join(root, CardsDir, "c00000000001")

	counts, err := ChildCounts(card, KindCard)
	if err != nil {
		t.Fatalf("child counts over the filled card: %v", err)
	}
	if len(counts) != 3 {
		t.Errorf("the filled card answers %d entries, wanted 3: %v", len(counts), counts)
	}
	if total := ChildTotal(counts); total != 6 {
		t.Errorf("the filled card sums to %d, wanted 6: %v", total, counts)
	}

	empty := newFixture(t)
	bare := filepath.Join(empty, CardsDir, "c00000000001")
	counts, err = ChildCounts(bare, KindCard)
	if err != nil {
		t.Fatalf("child counts over the empty card: %v", err)
	}
	if len(counts) != 3 {
		t.Errorf("the empty card answers %d entries, wanted 3: %v", len(counts), counts)
	}
	if total := ChildTotal(counts); total != 0 {
		t.Errorf("the empty card sums to %d, wanted 0: %v", total, counts)
	}
}

// TestChildCountsAnswersTheMountsTheGrammarDeclares asserts dinah-519
// criteria/12's second half, and it asserts it behaviourally rather than by
// reading any source.
//
// The key set is compared with what Contains answers for a card, asked inside
// the test rather than written down, and then the grammar is grown and the
// answer is asserted to grow with it. A walk written against a fixed set of
// mounts answers three in the second half and fails, wherever that fixed set
// is written: in ChildCounts's own body, in an unexported helper it calls, or
// in a slice of this package's directory constants. A guard over the source
// catches none of those, because each of them names no literal.
//
// The test writes a package var, so it declares itself non-parallel and puts
// the table back.
//
// Arming: rewriting ChildCounts to range over a fixed slice of the three
// mounts a card carries today leaves the build green and reddens the second
// half of this test by name, while the first half stays green.
func TestChildCountsAnswersTheMountsTheGrammarDeclares(t *testing.T) {
	root := childCountsFixture(t)
	card := filepath.Join(root, CardsDir, "c00000000001")

	counts, err := ChildCounts(card, KindCard)
	if err != nil {
		t.Fatalf("child counts: %v", err)
	}
	var declared []string
	for _, mount := range Contains(KindCard) {
		declared = append(declared, mount.Dir)
	}
	if len(declared) == 0 {
		t.Fatal("the grammar gives a card no mount at all, so this comparison reads nothing")
	}
	if got := keysOf(counts); !reflect.DeepEqual(got, sorted(declared)) {
		t.Errorf("ChildCounts answers the keys %v, and the grammar gives a card the mounts %v", got, sorted(declared))
	}

	grown := "sketches"
	restore := containment[KindCard]
	t.Cleanup(func() { containment[KindCard] = restore })
	containment[KindCard] = append(append([]Mount{}, restore...), Mount{
		Dir: grown, Kind: "sketch", Anchor: "sketch.md",
	})
	write(t, filepath.Join(card, grown, "a00000000007", "sketch.md"), "---\n---\nA sketch.\n")

	counts, err = ChildCounts(card, KindCard)
	if err != nil {
		t.Fatalf("child counts over the grown grammar: %v", err)
	}
	if len(counts) != 4 {
		t.Fatalf("the grown grammar answers %d entries, wanted 4: %v", len(counts), counts)
	}
	if counts[grown] != 1 {
		t.Errorf("the grown mount is counted %d, wanted 1: %v", counts[grown], counts)
	}
	if total := ChildTotal(counts); total != 7 {
		t.Errorf("the grown grammar sums to %d, wanted 7: %v", total, counts)
	}
}

// keysOf are a count map's keys in sorted order, so two key sets compare
// without either side depending on map iteration order.
func keysOf(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sorted is a copy of a slice in sorted order, leaving the caller's own order
// alone.
func sorted(values []string) []string {
	copied := append([]string{}, values...)
	sort.Strings(copied)
	return copied
}
