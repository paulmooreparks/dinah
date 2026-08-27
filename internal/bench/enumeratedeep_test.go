package bench

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// deepTree writes one workbench anchor at each relative path beneath a fresh
// directory and returns that directory. A path is written as a slash-separated
// string, so a case reads as the shape it builds rather than as a pile of
// filepath.Join calls.
func deepTree(t *testing.T, places ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, place := range places {
		parts := append([]string{root}, strings.Split(place, "/")...)
		writeWorkbench(t, filepath.Join(parts...), "Fixture "+place)
	}
	return root
}

// paths returns the candidates' paths relative to root, sorted, which is what
// a case asserts against: the walk's own order is stable but it is not the
// property any of these tests is about.
func paths(t *testing.T, root string, found []Candidate) []string {
	t.Helper()
	var out []string
	for _, candidate := range found {
		rel, err := filepath.Rel(root, candidate.Path)
		if err != nil {
			t.Fatalf("relative path of %q: %v", candidate.Path, err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out
}

// TestEnumerateDeepFindsEveryWorkbenchBeneathARoot asserts that the downward
// walk reports a workbench at every rung, including one nested inside another,
// which is the recursion Enumerate already runs and this walk keeps.
func TestEnumerateDeepFindsEveryWorkbenchBeneathARoot(t *testing.T) {
	root := deepTree(t, "alpha", "customer/beta", "customer/deep/gamma", "customer/beta/nested")
	found, err := EnumerateDeep(root, 0)
	if err != nil {
		t.Fatalf("the walk refused: %v", err)
	}
	want := []string{"alpha", "customer/beta", "customer/beta/nested", "customer/deep/gamma"}
	got := paths(t, root, found)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the walk found %v, wanted %v", got, want)
	}
	for _, candidate := range found {
		if candidate.Title == "" {
			t.Errorf("%s carries no title, so the row was described by nothing", candidate.Path)
		}
		if candidate.Refused != "" {
			t.Errorf("%s carries the refusal %s and nothing about it is broken", candidate.Path, candidate.Refused)
		}
	}
}

// TestEnumerateDeepBoundsTheWalkAtTheDepthAsked asserts the maxDepth contract
// at each of its three shapes: a bound reports the rungs at or above it and
// says nothing at all about what lies past it, zero descends without a bound,
// and the bound counts rungs below the root rather than path separators.
//
// The workbench past the bound is asserted absent rather than merely
// unreported as a workbench: a bound that reported it as a refusal would be
// describing a directory the walk never examined.
func TestEnumerateDeepBoundsTheWalkAtTheDepthAsked(t *testing.T) {
	root := deepTree(t, "one", "two/three", "two/four/five")
	cases := []struct {
		depth int
		want  []string
	}{
		{depth: 1, want: []string{"one"}},
		{depth: 2, want: []string{"one", "two/three"}},
		{depth: 3, want: []string{"one", "two/four/five", "two/three"}},
		{depth: 0, want: []string{"one", "two/four/five", "two/three"}},
	}
	for _, c := range cases {
		found, err := EnumerateDeep(root, c.depth)
		if err != nil {
			t.Fatalf("depth %d: the walk refused: %v", c.depth, err)
		}
		got := paths(t, root, found)
		if strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("depth %d found %v, wanted %v", c.depth, got, c.want)
		}
	}
}

// TestDefaultEnumerateDepthIsTheBoundTheSurfacesApply asserts the constant is
// the documented 8 rather than whatever a later edit leaves behind, since two
// heads default to it by name and a silent change to it would widen every
// unbounded-looking call on the refresh path.
func TestDefaultEnumerateDepthIsTheBoundTheSurfacesApply(t *testing.T) {
	if DefaultEnumerateDepth != 8 {
		t.Errorf("the default bound is %d, wanted 8", DefaultEnumerateDepth)
	}
	deep := make([]string, 0, 1)
	deep = append(deep, strings.Repeat("down/", DefaultEnumerateDepth)+"buried")
	root := deepTree(t, append(deep, "shallow")...)
	found, err := EnumerateDeep(root, DefaultEnumerateDepth)
	if err != nil {
		t.Fatalf("the walk refused: %v", err)
	}
	got := paths(t, root, found)
	if strings.Join(got, " ") != "shallow" {
		t.Errorf("the default bound reported %v, wanted the shallow workbench alone", got)
	}
	unbounded, err := EnumerateDeep(root, 0)
	if err != nil {
		t.Fatalf("the unbounded walk refused: %v", err)
	}
	if len(unbounded) != 2 {
		t.Errorf("the unbounded walk found %d workbenches, wanted both", len(unbounded))
	}
}

// TestABadAnchorIsAReportedRowRatherThanAFailedWalk asserts the difference
// between this walk and Enumerate's: a workbench.md that will not read costs
// its own directory a described row and costs the walk nothing else. Every
// sibling is still reported, the call does not fail, and the bad directory is
// present carrying the refusal name and neither a title nor a slug.
//
// The read is broken through the readAnchorContent seam rather than through
// permission bits, which is how TestAnUnreadableAnchorStopsTheWalkWithARefusal
// already does it, because a file mode that denies a read on one platform does
// not deny it on another.
func TestABadAnchorIsAReportedRowRatherThanAFailedWalk(t *testing.T) {
	root := deepTree(t, "alpha", "broken", "omega")
	broken := filepath.Join(root, "broken", WorkbenchAnchor)
	unreadable := errors.New("simulated unreadable file")
	previous := readAnchorContent
	readAnchorContent = func(path string) (string, error) {
		if path == broken {
			return "", unreadable
		}
		return previous(path)
	}
	t.Cleanup(func() { readAnchorContent = previous })

	found, err := EnumerateDeep(root, 0)
	if err != nil {
		t.Fatalf("one bad anchor failed the whole walk: %v", err)
	}
	got := paths(t, root, found)
	if strings.Join(got, " ") != "alpha broken omega" {
		t.Errorf("the walk found %v, wanted the two good workbenches and the bad directory", got)
	}
	var row Candidate
	for _, candidate := range found {
		if filepath.Base(candidate.Path) == "broken" {
			row = candidate
		}
	}
	if row.Refused != contract.UnreadableBench {
		t.Errorf("the bad row carries the refusal %q, wanted %s", row.Refused, contract.UnreadableBench)
	}
	if row.Title != "" || row.Slug != "" {
		t.Errorf("the bad row carries the title %q and the slug %q, and nothing read them", row.Title, row.Slug)
	}
	// Enumerate's own contract is the one this walk was written beside rather
	// than on top of, so the same tree is asserted to still fail it.
	if _, err := enumerate(root); err == nil {
		t.Error("Enumerate now survives a bad anchor, so this walk is no longer the one that differs")
	}
}

// TestEnumerateDeepRefusesTheRootItselfAndReportsAnEmptyDirectory asserts the
// one case that stays a whole-call refusal and the one that does not. A root
// that is missing, is not a directory, or is empty of a path refuses
// dinah.unknown-root naming it; a real directory holding nothing recognisable
// answers with no rows and no refusal, because finding no workbench beneath a
// path is an ordinary fact rather than a failure.
func TestEnumerateDeepRefusesTheRootItselfAndReportsAnEmptyDirectory(t *testing.T) {
	tree := t.TempDir()
	missing := filepath.Join(tree, "nothing-here")
	file := filepath.Join(tree, "a-file")
	write(t, file, "not a directory\n")

	for _, root := range []string{missing, file} {
		_, err := EnumerateDeep(root, 0)
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Fatalf("%s: wanted a refusal, got %v", root, err)
		}
		if refusal.Name != contract.UnknownRoot {
			t.Errorf("%s: refusal name %s, wanted %s", root, refusal.Name, contract.UnknownRoot)
		}
		if refusal.Detail != root {
			t.Errorf("%s: the refusal names %q rather than the path asked about", root, refusal.Detail)
		}
	}
	if _, err := EnumerateDeep("", 0); err == nil {
		t.Error("an empty root should refuse rather than walking somewhere")
	}

	empty := filepath.Join(tree, "empty")
	if err := os.MkdirAll(filepath.Join(empty, "just", "directories"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	found, err := EnumerateDeep(empty, 0)
	if err != nil {
		t.Fatalf("an empty directory refused: %v", err)
	}
	if found == nil {
		t.Error("the empty answer is nil, so it marshals as null rather than as an empty array")
	}
	if len(found) != 0 {
		t.Errorf("the empty directory reported %d workbenches", len(found))
	}
}

// TestEnumerateDeepReadsTheDiskEveryCall asserts the cache decision: a
// workbench created after a first call appears on the second, which is the
// whole reason this walk exists beside the cached one. A poller is the caller
// this walk was written for, and a cache here would hide the exact thing a
// poll runs to notice.
//
// Enumerate is asserted to behave the other way over the same directory, so
// the test says the two walks differ rather than merely that this one works.
func TestEnumerateDeepReadsTheDiskEveryCall(t *testing.T) {
	root := deepTree(t, "first")
	if found, err := EnumerateDeep(root, 0); err != nil || len(found) != 1 {
		t.Fatalf("the first walk found %d workbenches: %v", len(found), err)
	}
	if cached, err := Enumerate(root); err != nil || len(cached) != 1 {
		t.Fatalf("the first cached walk found %d workbenches: %v", len(cached), err)
	}
	writeWorkbench(t, filepath.Join(root, "second"), "Fixture second")

	found, err := EnumerateDeep(root, 0)
	if err != nil {
		t.Fatalf("the second walk refused: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("the second walk found %d workbenches, wanted the one created since the first", len(found))
	}
	cached, err := Enumerate(root)
	if err != nil {
		t.Fatalf("the second cached walk refused: %v", err)
	}
	if len(cached) != 1 {
		t.Errorf("Enumerate found %d workbenches, and its cache is what keeps it at one; this walk is no longer the one that differs", len(cached))
	}
}
