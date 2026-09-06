package release

import "testing"

// publishedBinaries is the set of filenames a CLI release has published since
// the six-platform matrix was introduced, written out by hand rather than
// derived from Targets. A test that built its expectation from the same
// declaration it is checking would pass whatever that declaration said, so the
// literal list is the whole point of this fixture: it is the record of what
// release.yml named before internal/release owned the list, and Targets has to
// go on agreeing with it.
var publishedBinaries = []string{
	"dinah-windows-amd64.exe",
	"dinah-windows-arm64.exe",
	"dinah-linux-amd64",
	"dinah-linux-arm64",
	"dinah-darwin-amd64",
	"dinah-darwin-arm64",
}

// TestTargetsNamesTheSixPublishedBinaries asserts that Targets holds six
// entries and that the names they produce are exactly the six a release
// publishes. It fails when an entry is added or dropped, and it fails when any
// entry's GOOS, GOARCH or Ext is misspelled, because a misspelling changes the
// name that entry contributes to the set.
func TestTargetsNamesTheSixPublishedBinaries(t *testing.T) {
	if len(Targets) != 6 {
		t.Fatalf("Targets holds %d entries, and a CLI release builds 6", len(Targets))
	}
	built := map[string]bool{}
	for _, target := range Targets {
		name := target.BinaryName()
		if built[name] {
			t.Errorf("two entries of Targets both produce %s, so one platform would overwrite the other in dist/", name)
		}
		built[name] = true
	}
	published := map[string]bool{}
	for _, name := range publishedBinaries {
		published[name] = true
		if !built[name] {
			t.Errorf("a release publishes %s, and no entry of Targets produces it", name)
		}
	}
	for name := range built {
		if !published[name] {
			t.Errorf("Targets produces %s, which is not one of the binaries a release publishes", name)
		}
	}
}

// TestTargetsKeepsTheOrderTheBuildMatrixReads asserts that Targets stays in
// the order release.yml's build matrix declared before it read the list from
// here. The matrix is fed this slice in order, and the asset check sorts what
// it compares, so nothing else in the workflow would notice a reordering; the
// order is worth holding because a run's job names come from it and a reader
// comparing two runs reads them side by side.
func TestTargetsKeepsTheOrderTheBuildMatrixReads(t *testing.T) {
	if len(Targets) != len(publishedBinaries) {
		t.Fatalf("Targets holds %d entries and %d binaries are published, so there is no order to compare", len(Targets), len(publishedBinaries))
	}
	for at, target := range Targets {
		if got := target.BinaryName(); got != publishedBinaries[at] {
			t.Errorf("Targets[%d] produces %s, and the build matrix's %d-th entry has always been %s", at, got, at, publishedBinaries[at])
		}
	}
}
