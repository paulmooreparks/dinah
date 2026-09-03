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

// deepTree writes one workbench at each relative path beneath a fresh
// directory and returns that directory. A path is written as a slash-separated
// string, so a case reads as the shape it builds rather than as a pile of
// filepath.Join calls.
//
// Each workbench is planted in the named directory's own .dinah container
// rather than at the directory itself, because a workbench sitting anywhere
// else is not a workbench and the walk under test correctly reports none. The
// cases go on naming the containing directory, which is what a person calls
// the place a workbench lives, and deepPlace is the one function that knows
// how the two spellings relate.
func deepTree(t *testing.T, places ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, place := range places {
		writeWorkbench(t, deepPlace(root, place), "Fixture "+place)
	}
	return root
}

// deepPlace answers the workbench directory a fixture plants for one
// slash-separated place: the container beneath that place, under a fixture
// identifier wide enough for IsWorkbenchID to admit it.
func deepPlace(root, place string) string {
	parts := append([]string{root}, strings.Split(place, "/")...)
	return filepath.Join(filepath.Join(parts...), UserBaseName, fixtureWorkbenchID)
}

// fixtureWorkbenchID is one minted-looking workbench identifier these fixtures
// reuse, since each is the only workbench in its own container and no case
// here turns on two of them differing. It is written out rather than minted so
// that a failure names the same path twice running, and IsWorkbenchID admits
// it: 32 lowercase hex characters whose thirteenth is the version nibble 7 and
// whose seventeenth carries the variant bits 10.
const fixtureWorkbenchID = "0199a1b2c3d47abc8000000000000001"

// paths returns the candidates' paths relative to root, sorted, which is what
// a case asserts against: the walk's own order is stable but it is not the
// property any of these tests is about.
func paths(t *testing.T, root string, found []Candidate) []string {
	t.Helper()
	var out []string
	suffix := "/" + UserBaseName + "/" + fixtureWorkbenchID
	for _, candidate := range found {
		rel, err := filepath.Rel(root, candidate.Path)
		if err != nil {
			t.Fatalf("relative path of %q: %v", candidate.Path, err)
		}
		out = append(out, strings.TrimSuffix(filepath.ToSlash(rel), suffix))
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

// TestEnumerateDeepOrdersItsRowsByPath asserts the order two heads depend on.
// The recursion meets directories depth-first over each directory's sorted
// names, which is not path order: two-extra sorts before two/three and the
// recursion reaches it after. Two callers publish this listing, so the order
// belongs to the walk rather than to whichever of them sorted last.
func TestEnumerateDeepOrdersItsRowsByPath(t *testing.T) {
	root := deepTree(t, "two/three", "two-extra", "alpha")
	found, err := EnumerateDeep(root, 0)
	if err != nil {
		t.Fatalf("the walk refused: %v", err)
	}
	var got []string
	for _, candidate := range found {
		got = append(got, candidate.Path)
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("the rows are not in path order: %v", got)
	}
	// Named rather than left to the sortedness check alone, because a walk that
	// reported these two in recursion order would also be sorted if the fixture
	// happened not to separate them.
	if strings.Join(paths(t, root, found), " ") != "alpha two-extra two/three" {
		t.Errorf("the walk answered %v, wanted the sibling before what lies inside the directory it sorts against", paths(t, root, found))
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
	broken := filepath.Join(deepPlace(root, "broken"), WorkbenchAnchor)
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
		// The refused row names the directory whose container could not be
		// read rather than the workbench inside it, because the read failed
		// before anything named that workbench.
		if candidate.Path == filepath.Join(root, "broken") {
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
	writeWorkbench(t, deepPlace(root, "second"), "Fixture second")

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

// TestADamagedAnchorIsAReportedRowRatherThanAFailedWalk asserts AC-5. It is
// the sibling of TestABadAnchorIsAReportedRowRatherThanAFailedWalk above, and
// the two differ only in why the anchor is unusable: that one cannot be read
// at all, and this one reads and no longer names itself as a workbench.
//
// The deep walk degrades per row, so the damaged workbench arrives as a row
// carrying the refusal name and nothing else, and every other workbench in
// the tree is still listed. The shallow listing fails its whole call on the
// same tree, which is its own documented contract for an unusable anchor and
// needs no code change to hold (D-4).
func TestADamagedAnchorIsAReportedRowRatherThanAFailedWalk(t *testing.T) {
	root := deepTree(t, "alpha", "broken", "omega")
	write(t, filepath.Join(deepPlace(root, "broken"), WorkbenchAnchor), damageShapes[2].text)

	found, err := EnumerateDeep(root, 0)
	if err != nil {
		t.Fatalf("one damaged anchor failed the whole walk: %v", err)
	}
	got := paths(t, root, found)
	if strings.Join(got, " ") != "alpha broken omega" {
		t.Errorf("the walk found %v, wanted the two good workbenches and the damaged directory", got)
	}
	var row Candidate
	for _, candidate := range found {
		// The refused row names the directory whose container held the
		// damage rather than the workbench inside it, because benchIn is
		// what refused and it was asked about the directory.
		if candidate.Path == filepath.Join(root, "broken") {
			row = candidate
		}
	}
	if row.Refused != contract.DamagedBench {
		t.Errorf("the damaged row carries the refusal %q, wanted %s", row.Refused, contract.DamagedBench)
	}
	if row.Title != "" || row.Slug != "" {
		t.Errorf("the damaged row carries the title %q and the slug %q, and nothing read them", row.Title, row.Slug)
	}
	// Enumerate's own whole-call-fails contract is what this walk was written
	// beside rather than on top of, so the same tree is asserted to still
	// fail it.
	if _, err := enumerate(root); err == nil {
		t.Error("Enumerate now survives a damaged anchor, so this walk is no longer the one that differs")
	}
}

// TestTheSweepReportsADamagedWorkbenchAndKeepsWalking asserts the bench half
// of AC-6. ScanContainers is a second, independent tree walk that never went
// through soleBench, so it carried the same recognition gap: a damaged,
// positioned workbench was skipped in silence by the one tool an operator
// reaches for to repair a container.
//
// It accumulates rather than refusing, which is the difference between this
// walk and the climb. A climb picks one workbench out of one container and
// has a healthy answer to protect; this sweep reports a whole tree, so a
// damaged directory has nothing to crowd out and must not cost the operator
// every workbench after it in the walk order.
func TestTheSweepReportsADamagedWorkbenchAndKeepsWalking(t *testing.T) {
	root := deepTree(t, "alpha", "broken", "omega")
	damaged := deepPlace(root, "broken")
	write(t, filepath.Join(damaged, WorkbenchAnchor), damageShapes[3].text)

	found, reported, err := ScanContainers(root)
	if err != nil {
		t.Fatalf("one damaged workbench ended the sweep: %v", err)
	}
	var healthy []string
	for _, candidate := range found {
		healthy = append(healthy, candidate.Path)
	}
	if len(healthy) != 2 {
		t.Errorf("the sweep found %v, wanted the two workbenches either side of the damaged one", healthy)
	}
	for _, path := range healthy {
		if path == damaged {
			t.Errorf("the damaged workbench %q was reported as a healthy candidate", damaged)
		}
	}
	if len(reported) != 1 || reported[0] != damaged {
		t.Errorf("the sweep reported the damaged workbenches %v, wanted [%q]", reported, damaged)
	}

	// A bare anchor and a stray-named container entry stay outside this,
	// because the position test is the whole of what makes a damaged
	// workbench distinguishable from somebody else's document (D-1).
	write(t, filepath.Join(root, "notes", WorkbenchAnchor), damageShapes[1].text)
	write(t, filepath.Join(root, "stray", UserBaseName, "my-notes", WorkbenchAnchor), damageShapes[1].text)
	_, reported, err = ScanContainers(root)
	if err != nil {
		t.Fatalf("the second sweep: %v", err)
	}
	if len(reported) != 1 || reported[0] != damaged {
		t.Errorf("the sweep reported %v, and neither a bare anchor nor an unminted name is a damaged workbench", reported)
	}
}
