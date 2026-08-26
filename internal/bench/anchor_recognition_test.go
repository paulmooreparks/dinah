package bench

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// foreignAnchor is a workbench.md that carries none of the keys the
// recognition test looks for: no frontmatter fence at all, just a plain
// Markdown note. Millions of unrelated files on a real machine carry exactly
// this shape.
const foreignAnchor = "# Notes\n\nJust some notes, unrelated to any workbench.\n"

// TestAForeignAnchorDoesNotStopTheClimb asserts AC-1: a directory holding a
// workbench.md that carries none of profile, format or columns sitting below
// a real workbench no longer stops the ancestor walk. The command run from
// inside it resolves the ancestor workbench instead of refusing over the
// foreign file, and the foreign file is reported back as passed over.
func TestAForeignAnchorDoesNotStopTheClimb(t *testing.T) {
	outer := t.TempDir()
	write(t, filepath.Join(outer, WorkbenchAnchor), benchDefinition)
	notes := filepath.Join(outer, "notes")
	write(t, filepath.Join(notes, WorkbenchAnchor), foreignAnchor)

	found, passed, err := Discover(notes, "", "", "")
	if err != nil {
		t.Fatalf("a foreign anchor below a real workbench should not stop the climb, got %v", err)
	}
	if found != outer {
		t.Errorf("wanted the ancestor workbench %q, got %q", outer, found)
	}
	want := filepath.Join(notes, WorkbenchAnchor)
	if len(passed) != 1 || passed[0] != want {
		t.Errorf("wanted the foreign anchor reported as passed over, wanted [%q], got %v", want, passed)
	}
}

// TestARecognizedButDamagedAnchorStillStopsTheClimb asserts AC-2: a directory
// whose workbench.md carries format or columns but not profile is still
// recognized, so the climb stops there rather than passing it over, and Open
// refuses over it with the exact wording the format's malformed refusal has
// always carried.
func TestARecognizedButDamagedAnchorStillStopsTheClimb(t *testing.T) {
	outer := t.TempDir()
	write(t, filepath.Join(outer, WorkbenchAnchor), benchDefinition)
	damaged := strings.Replace(benchDefinition, "profile: dinah-core/0.7\n", "", 1)
	inner := filepath.Join(outer, "damaged")
	write(t, filepath.Join(inner, WorkbenchAnchor), damaged)

	found, passed, err := Discover(inner, "", "", "")
	if err != nil {
		t.Fatalf("a recognized anchor should stop the climb rather than refuse discovery, got %v", err)
	}
	if found != inner {
		t.Errorf("wanted the climb to stop at the damaged anchor %q, got %q", inner, found)
	}
	if len(passed) != 0 {
		t.Errorf("a recognized anchor is not passed over, got %v", passed)
	}

	_, err = Open(found)
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal opening the damaged workbench, got %v", err)
	}
	if refusal.Name != contract.Malformed || refusal.Detail != "profile" {
		t.Errorf("wanted malformed/profile, got %s/%s", refusal.Name, refusal.Detail)
	}
	if got := refusal.Extra["path"]; got != filepath.Join(inner, WorkbenchAnchor) {
		t.Errorf("wanted the refusal to name the file, got %q", got)
	}
}

// TestARecognizedAnchorStopsTheClimbOnProfileAlone asserts AC-3: an anchor
// carrying profile and nothing else that format or columns would have
// declared stops the climb exactly as before, since profile alone already
// claims the directory.
func TestARecognizedAnchorStopsTheClimbOnProfileAlone(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, WorkbenchAnchor), "---\nprofile: dinah-core/0.7\n---\n")

	found, passed, err := Discover(root, "", "", "")
	if err != nil {
		t.Fatalf("an anchor declaring profile should stop the climb, got %v", err)
	}
	if found != root {
		t.Errorf("wanted %q, got %q", root, found)
	}
	if len(passed) != 0 {
		t.Errorf("a recognized anchor is not passed over, got %v", passed)
	}
}

// TestARecognizedAnchorStopsTheClimbOnTheColumnSequenceAlone asserts
// dinah-287 AC-19: the key Recognized tests beside profile and format is
// columns, the name that sequence took at the vocabulary rename, so an anchor
// carrying only that sequence claims its directory.
//
// The other tests in this file all write a profile, which is the arm of
// Recognized that answers first, so none of them can tell which name the third
// arm reads. This one writes no profile at all, which is what makes it the
// test that fails when the arm names the retired key.
func TestARecognizedAnchorStopsTheClimbOnTheColumnSequenceAlone(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, WorkbenchAnchor), "---\ntitle: Sequence only\ncolumns:\n  - b00000000001\n---\n")

	found, passed, err := Discover(root, "", "", "")
	if err != nil {
		t.Fatalf("an anchor declaring a column sequence should stop the climb, got %v", err)
	}
	if found != root {
		t.Errorf("wanted %q, got %q", root, found)
	}
	if len(passed) != 0 {
		t.Errorf("a recognized anchor is not passed over, got %v", passed)
	}
}

// TestSoleBenchPassesOverAForeignContainerEntry asserts AC-4: the same
// three-way test runs inside a .dinah container. A container holding one
// recognized id directory and one foreign id directory resolves to the
// recognized one, and the foreign entry is reported as passed over rather
// than counted as a candidate.
func TestSoleBenchPassesOverAForeignContainerEntry(t *testing.T) {
	base := t.TempDir()
	writeWorkbench(t, filepath.Join(base, "d00000000001"), "The real one")
	write(t, filepath.Join(base, "d00000000002", WorkbenchAnchor), foreignAnchor)

	found, ambiguous, passed, err := soleBench(base)
	if err != nil {
		t.Fatalf("a foreign id directory should not stop the scan, got %v", err)
	}
	if len(ambiguous) != 0 {
		t.Errorf("a foreign entry should never make the base look ambiguous, got %v", ambiguous)
	}
	want := filepath.Join(base, "d00000000001")
	if found != want {
		t.Errorf("wanted the recognized entry %q, got %q", want, found)
	}
	wantPassed := filepath.Join(base, "d00000000002", WorkbenchAnchor)
	if len(passed) != 1 || passed[0] != wantPassed {
		t.Errorf("wanted the foreign entry reported passed over, wanted [%q], got %v", wantPassed, passed)
	}
}

// TestAnUnreadableAnchorStopsTheWalkWithARefusal asserts AC-5: a workbench.md
// that exists but returns an error on read, forced through the
// readAnchorContent test seam rather than through OS permission bits, stops
// the walk. Discover refuses with dinah.unreadable-workbench naming that
// file, rather than climbing past it or reporting it as absent.
func TestAnUnreadableAnchorStopsTheWalkWithARefusal(t *testing.T) {
	root := t.TempDir()
	broken := filepath.Join(root, "broken")
	write(t, filepath.Join(broken, WorkbenchAnchor), foreignAnchor)

	unreadable := errors.New("simulated unreadable file")
	previous := readAnchorContent
	readAnchorContent = func(path string) (string, error) {
		if path == filepath.Join(broken, WorkbenchAnchor) {
			return "", unreadable
		}
		return previous(path)
	}
	t.Cleanup(func() { readAnchorContent = previous })

	_, _, err := Discover(broken, "", "", "")
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if refusal.Name != contract.UnreadableBench {
		t.Errorf("refusal name: wanted %s, got %s", contract.UnreadableBench, refusal.Name)
	}
	if refusal.Detail != filepath.Join(broken, WorkbenchAnchor) {
		t.Errorf("wanted the refusal to name the unreadable file, got %q", refusal.Detail)
	}
}

// TestReadAnchorContentOnlyRunsWhereExistsFoundAFile asserts AC-6: Exists
// still runs at every rung of the climb, but readAnchorContent only runs at a
// rung where Exists found a file there. Several ancestor directories carrying
// no workbench.md at all must never reach the read.
func TestReadAnchorContentOnlyRunsWhereExistsFoundAFile(t *testing.T) {
	deep := filepath.Join(t.TempDir(), "a", "b", "c", "d")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	calls := 0
	previous := readAnchorContent
	readAnchorContent = func(path string) (string, error) {
		calls++
		return previous(path)
	}
	t.Cleanup(func() { readAnchorContent = previous })

	// Nothing anywhere in this tree carries a workbench.md, so the walk
	// exhausts and refuses; the read must never have run.
	root := filepath.VolumeName(deep) + string(filepath.Separator)
	if found, ambiguous, _, err := benchIn(root, false); found != "" || len(ambiguous) > 0 || err != nil {
		t.Skip("the volume root carries a workbench of its own")
	}
	if _, _, err := Discover(deep, "", "", root); err == nil {
		t.Fatalf("an exhausted walk over a tree with no anchors should refuse")
	}
	if calls != 0 {
		t.Errorf("readAnchorContent should never run where Exists found nothing, got %d calls", calls)
	}
}

// TestCheckReportsOneIgnoredAnchorFindingPerPassedFile asserts the bench-level
// half of AC-8: Check appends one check.ignored-anchor finding per entry of
// Passed, naming that file's path, and a bench carrying no Passed list (one
// Open reads directly by path, with no discovery walk behind it) reports
// none.
func TestCheckReportsOneIgnoredAnchorFindingPerPassedFile(t *testing.T) {
	root := newFixture(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		if finding.Key == FindingIgnoredAnchor {
			t.Fatalf("a workbench opened directly by path should report no ignored anchors, got %+v", finding)
		}
	}

	opened.Passed = []string{
		filepath.Join(root, "notes", WorkbenchAnchor),
		filepath.Join(root, UserBaseName, "d00000000099", WorkbenchAnchor),
	}
	findings, err = opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.Key == FindingIgnoredAnchor {
			seen[finding.Path] = true
		}
	}
	for _, path := range opened.Passed {
		if !seen[path] {
			t.Errorf("wanted a check.ignored-anchor finding naming %q, got %+v", path, findings)
		}
	}
	if len(seen) != len(opened.Passed) {
		t.Errorf("wanted exactly one finding per passed-over file, got %+v", findings)
	}
}
