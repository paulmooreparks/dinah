package release

import (
	"strings"
	"testing"
)

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

// publishedTUIBinaries are the six filenames of dinah-tui, the terminal UI's
// program, which a release publishes beside publishedBinaries from dinah-603
// on, written out by hand for the same reason, in the same order.
var publishedTUIBinaries = []string{
	"dinah-tui-windows-amd64.exe",
	"dinah-tui-windows-arm64.exe",
	"dinah-tui-linux-amd64",
	"dinah-tui-linux-arm64",
	"dinah-tui-darwin-amd64",
	"dinah-tui-darwin-arm64",
}

// publishedFiles is every filename a release publishes: each platform's dinah
// and then its dinah-tui, in the order dinah-release targets --format names
// prints them.
func publishedFiles() []string {
	var files []string
	for at := range publishedBinaries {
		files = append(files, publishedBinaries[at], publishedTUIBinaries[at])
	}
	return files
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

// TestEveryTargetNamesItsTerminalUIProgram asserts that each entry of
// Targets names the dinah-tui file a release publishes for its platform, and
// that Names answers the two files dinah first, so the twelve names
// dinah-release targets --format names prints are exactly publishedFiles.
func TestEveryTargetNamesItsTerminalUIProgram(t *testing.T) {
	var names []string
	for at, target := range Targets {
		if got := target.TUIBinaryName(); got != publishedTUIBinaries[at] {
			t.Errorf("Targets[%d] names dinah-tui %s, and the release publishes %s", at, got, publishedTUIBinaries[at])
		}
		names = append(names, target.Names()...)
	}
	if got, want := strings.Join(names, " "), strings.Join(publishedFiles(), " "); got != want {
		t.Errorf("the targets name %s, and a release publishes %s", got, want)
	}
}
