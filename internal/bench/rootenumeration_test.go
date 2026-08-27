package bench

import (
	"path/filepath"
	"sort"
	"testing"
)

// rootDefinition is the smallest workbench these tests need: one column, no
// flags, and a revision this build opens.
const rootDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Root",
  "columns": [
    { "id": "e00000000001", "title": "Doing", "kind": "work", "instructions": "Doing instructions.\n" }
  ]
}`

// plantInContainer writes one workbench into a directory's own .dinah
// container, under the identifier it is given, and answers the anchor
// directory it wrote. That is the layout every dinah init run produces, and
// it is the layout the enumeration used to be unable to see, so a fixture
// planting a bare workbench instead cannot exercise these tests at all.
//
// cmd/dinah/vocabulary_test.go carries a closure of the same name doing the
// same job for the tree fixture there. The two cannot be one helper: an
// unexported helper does not cross a package boundary, and that copy plants
// through a real dinah init run because the fixture it feeds is testing the
// command, where this one calls Instantiate directly because the package
// under test is this one. Each says where the other is so that a reader who
// changes one finds the other, since nothing in the build, the vet pass or
// the test run reports a duplicate written across two packages.
func plantInContainer(t *testing.T, dir, id, slug string) string {
	t.Helper()
	definition, err := ReadDefinition([]byte(rootDefinition))
	if err != nil {
		t.Fatalf("read the definition: %v", err)
	}
	anchor := filepath.Join(dir, UserBaseName, id)
	if err := Instantiate(anchor, slug, "alka", definition); err != nil {
		t.Fatalf("instantiate %s: %v", anchor, err)
	}
	return anchor
}

// candidatePaths answers the paths of a listing, sorted, so a test compares
// two listings as sets rather than asserting an order no caller relies on.
func candidatePaths(listed []Candidate) []string {
	paths := make([]string, 0, len(listed))
	for _, candidate := range listed {
		paths = append(paths, candidate.Path)
	}
	sort.Strings(paths)
	return paths
}

// TestEnumerateFindsTheWorkbenchInTheRootsOwnContainer is dinah-312 AC-1. A
// directory whose own .dinah holds one workbench is listed when the
// enumeration is pointed at that directory. The walk read a root's entries and
// tested each child, skipping every dotted name, so it never opened the root's
// own container and the layout dinah init always writes was the one layout the
// enumeration could not answer for.
//
// The assertion names the anchor directory rather than counting a non-empty
// listing, so a change that answers one candidate for some other reason does
// not pass this test by accident.
func TestEnumerateFindsTheWorkbenchInTheRootsOwnContainer(t *testing.T) {
	root := t.TempDir()
	anchor := plantInContainer(t, root, "e00000000101", "rt")

	listed, err := enumerate(root)
	if err != nil {
		t.Fatalf("enumerate %s: %v", root, err)
	}
	if len(listed) != 1 {
		t.Fatalf("the enumeration of %s answered %d candidates, wanted the one its own container holds: %+v", root, len(listed), listed)
	}
	if listed[0].Path != anchor {
		t.Errorf("the candidate names %q, wanted the anchor directory %q", listed[0].Path, anchor)
	}
}

// TestEnumerateFindsBothWorkbenchesInAnAmbiguousRoot is dinah-312 AC-2, and
// the question it answers is dinah-312 D-2: how a root whose own .dinah holds
// more than one recognized workbench is reported. It is reported the way an
// ambiguous .dinah further down the tree is already reported, with every
// candidate listed as its own row and none of them chosen.
//
// It stands as a test of its own rather than as a second case inside AC-1's,
// because the two mutations it guards against both leave AC-1 green: taking
// only the first ambiguous candidate, and dropping the ambiguous list
// altogether. Each looks like a tidy simplification of the root probe, and
// neither is visible to a fixture holding a single workbench.
func TestEnumerateFindsBothWorkbenchesInAnAmbiguousRoot(t *testing.T) {
	root := t.TempDir()
	first := plantInContainer(t, root, "e00000000201", "ra")
	second := plantInContainer(t, root, "e00000000202", "rb")

	listed, err := enumerate(root)
	if err != nil {
		t.Fatalf("enumerate %s: %v", root, err)
	}
	wanted := []string{first, second}
	sort.Strings(wanted)
	got := candidatePaths(listed)
	if len(got) != len(wanted) {
		t.Fatalf("the enumeration of %s answered %d candidates, wanted both of its container's two: %+v", root, len(got), listed)
	}
	for position, path := range got {
		if path != wanted[position] {
			t.Errorf("the enumeration answered %v, wanted %v", got, wanted)
			break
		}
	}
}
