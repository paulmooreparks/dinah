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
	real := containedPath(outer)
	write(t, filepath.Join(real, WorkbenchAnchor), benchDefinition)
	notes := filepath.Join(outer, "notes")
	write(t, filepath.Join(notes, WorkbenchAnchor), foreignAnchor)

	found, passed, err := Discover(notes, "", "", "")
	if err != nil {
		t.Fatalf("a foreign anchor below a real workbench should not stop the climb, got %v", err)
	}
	if found != real {
		t.Errorf("wanted the ancestor workbench %q, got %q", real, found)
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
	write(t, filepath.Join(containedPath(outer), WorkbenchAnchor), benchDefinition)
	damaged := strings.Replace(benchDefinition, "profile: dinah-core/0.7\n", "", 1)
	below := filepath.Join(outer, "damaged")
	inner := containedPath(below)
	write(t, filepath.Join(inner, WorkbenchAnchor), damaged)

	found, passed, err := Discover(below, "", "", "")
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
	dir := t.TempDir()
	root := containedPath(dir)
	write(t, filepath.Join(root, WorkbenchAnchor), "---\nprofile: dinah-core/0.7\n---\n")

	found, passed, err := Discover(dir, "", "", "")
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
	dir := t.TempDir()
	root := containedPath(dir)
	write(t, filepath.Join(root, WorkbenchAnchor), "---\ntitle: Sequence only\ncolumns:\n  - b00000000001\n---\n")

	found, passed, err := Discover(dir, "", "", "")
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
	if found, ambiguous, _, _, err := benchIn(root, false); found != "" || len(ambiguous) > 0 || err != nil {
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

// damageShapes is every way a workbench.md that reads can fail recognition,
// and it is exhaustive rather than a sample. ParseAnchor extracts a header
// only when line one trimmed is exactly "---" and some later line trimmed is
// too, so an unrecognised file is either unfenced, or fenced and never
// closed, or fenced and closed while carrying none of the three keys
// Recognized tests. Anything else a hand edit does to the file lands in one
// of those three.
var damageShapes = []struct {
	name string
	text string
}{
	// The anchor emptied. Line one is the empty string, so there is no
	// fence, and this is the shape dinah-272's review produced by
	// truncating the file.
	{"emptied", ""},
	// Text pasted over the whole file. Line one is whatever the paste
	// begins with, so this is the same unfenced path as the emptied file
	// rather than a second one.
	{"pasted over", "# Notes\n\nSomething else entirely got pasted over this.\n"},
	// The fence opened and never closed, which is the likeliest way a hand
	// edit leaves the file: the opening delimiter survives and the closing
	// one is eaten.
	{"unterminated fence", "---\nformat: 1\nprofile: dinah-core/0.7\ntitle: Fixture\n"},
	// Both fences present and a real header extracted, carrying none of
	// profile, format or columns. A hand edit that removed a stray format
	// line and took its neighbours with it leaves exactly this.
	{"fenced but keyless", "---\ntitle: Fixture\nslug: fx\noperator: alka\n---\nStanding text.\n"},
}

// TestADamagedContainedAnchorRefusesRatherThanPassingOver asserts AC-1. Each
// of the four damage shapes, written at the one address the containment rule
// gives a workbench, stops the climb with dinah.damaged-workbench naming that
// workbench's own directory. None of them answers dinah.no-workbench-found,
// which is the sentence the tool printed before this card while the reader
// was standing inside the workbench it could not see.
//
// The fifth case asserts the opposite of the first four on identical bytes.
// The emptied anchor written bare, with no .dinah container above it, is
// still passed over and the climb still resolves the ancestor workbench, the
// way TestAForeignAnchorDoesNotStopTheClimb proves for a file that was never
// Dinah's. Content did not become readable at this card; position became
// admissible, and only inside a container.
func TestADamagedContainedAnchorRefusesRatherThanPassingOver(t *testing.T) {
	for _, shape := range damageShapes {
		t.Run(shape.name, func(t *testing.T) {
			workbench := containedPath(t.TempDir())
			write(t, filepath.Join(workbench, WorkbenchAnchor), shape.text)

			_, _, err := Discover(workbench, "", "", "")
			refusal, ok := err.(*contract.Refusal)
			if !ok {
				t.Fatalf("a damaged contained anchor should refuse, got err %v", err)
			}
			if refusal.Name != contract.DamagedBench {
				t.Errorf("refusal name: wanted %s, got %s", contract.DamagedBench, refusal.Name)
			}
			if refusal.Detail != workbench {
				t.Errorf("wanted the refusal to name the workbench directory %q, got %q", workbench, refusal.Detail)
			}
		})
	}

	t.Run("the same bytes written bare are still climbed past", func(t *testing.T) {
		outer := t.TempDir()
		real := containedPath(outer)
		write(t, filepath.Join(real, WorkbenchAnchor), benchDefinition)
		bare := filepath.Join(outer, "notes")
		write(t, filepath.Join(bare, WorkbenchAnchor), damageShapes[0].text)

		found, passed, err := Discover(bare, "", "", "")
		if err != nil {
			t.Fatalf("a bare unrecognised anchor should not stop the climb, got %v", err)
		}
		if found != real {
			t.Errorf("wanted the ancestor workbench %q, got %q", real, found)
		}
		want := filepath.Join(bare, WorkbenchAnchor)
		if len(passed) != 1 || passed[0] != want {
			t.Errorf("wanted the bare anchor reported as passed over, wanted [%q], got %v", want, passed)
		}
	})
}

// TestADamagedWorkbenchStopsTheClimbBeforeAnUnrelatedAncestor asserts AC-9,
// which is the defect design review found by running the code rather than by
// reading it. A damaged workbench sitting between the caller and a real
// ancestor workbench used to be climbed past in silence, so the command ran
// against a workbench the caller never named and did not know was there.
//
// It is built as its own tree rather than inferred from the sibling case,
// because the two exercise different rungs. The sibling case is one container
// answering its own query, and this is a climb that has to stop at a
// container with no healthy answer instead of carrying on to one further up.
func TestADamagedWorkbenchStopsTheClimbBeforeAnUnrelatedAncestor(t *testing.T) {
	outer := t.TempDir()
	ancestor := containedPath(outer)
	writeWorkbench(t, ancestor, "The unrelated ancestor")

	descendant := filepath.Join(outer, "customer", "project")
	damaged := containedPath(descendant)
	write(t, filepath.Join(damaged, WorkbenchAnchor), damageShapes[2].text)

	// Start below the damaged workbench's own directory, so the climb reaches
	// it as a rung on the way up rather than starting inside it.
	start := filepath.Join(descendant, "src")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	found, _, err := Discover(start, "", "", "")
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("the climb resolved to %q instead of stopping at the damaged workbench", found)
	}
	if refusal.Name != contract.DamagedBench {
		t.Errorf("refusal name: wanted %s, got %s", contract.DamagedBench, refusal.Name)
	}
	if refusal.Detail != damaged {
		t.Errorf("wanted the refusal to name the damaged workbench %q, got %q", damaged, refusal.Detail)
	}
	if found == ancestor {
		t.Errorf("the climb resolved to the unrelated ancestor %q, which is the defect this test exists for", ancestor)
	}
}

// TestSoleBenchStillResolvesAHealthySiblingBesideDamage asserts AC-8. A
// container holding one healthy, unambiguous workbench and one damaged entry
// resolves to the healthy one, because that container had already answered
// the question before the damaged entry was relevant to it.
//
// This is what forced soleBench to read the whole base before deciding. The
// first round of this card refused the moment the scan met damage, which made
// a workbench somebody was using unreachable as soon as a half-migrated or
// hand-damaged sibling landed in its container, and dinah-285's own migration
// leaves two entries coexisting in one container for the length of a real
// migration window.
//
// The two cases differ only in which identifier carries the damage, so one of
// them meets the damage first and the other meets it last. The identifiers
// are written out rather than minted, so which order the scan runs in is a
// property of the fixture rather than of whatever a random draw produced.
func TestSoleBenchStillResolvesAHealthySiblingBesideDamage(t *testing.T) {
	cases := []struct {
		name    string
		healthy string
		damaged string
	}{
		{"the scan meets the healthy entry first", "d00000000001", "d00000000002"},
		{"the scan meets the damage first", "d00000000002", "d00000000001"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			outer := t.TempDir()
			base := filepath.Join(outer, UserBaseName)
			healthy := filepath.Join(base, c.healthy)
			damaged := filepath.Join(base, c.damaged)
			writeWorkbench(t, healthy, "The real one")
			write(t, filepath.Join(damaged, WorkbenchAnchor), damageShapes[3].text)

			found, ambiguous, err := soleBench(base)
			if err != nil {
				t.Fatalf("a damaged sibling should not discard a healthy answer, got %v", err)
			}
			if len(ambiguous) != 0 {
				t.Errorf("a damaged entry should never make the base look ambiguous, got %v", ambiguous)
			}
			if found != healthy {
				t.Errorf("wanted the healthy entry %q, got %q", healthy, found)
			}

			// The same fixture through the walk the commands actually run,
			// because soleBench answering correctly and Discover answering
			// correctly are two claims and only the second one ships.
			resolved, passed, err := Discover(outer, "", "", "")
			if err != nil {
				t.Fatalf("Discover refused over a container that has a healthy answer: %v", err)
			}
			if resolved != healthy {
				t.Errorf("Discover wanted %q, got %q", healthy, resolved)
			}
			for _, path := range passed {
				if strings.Contains(path, damaged) {
					t.Errorf("the damaged sibling %q did not decide this base and must not be named in the outcome, got %v", damaged, passed)
				}
			}
		})
	}
}

// TestSoleBenchNamesTheFirstDamagedEntryDeterministically asserts AC-10 and
// D-6. A container holding only damaged entries refuses, and the entry it
// names is the first in ListWorkbenchIDs' own order, which os.ReadDir sorts
// by name.
//
// The fixture writes the higher identifier first, so construction order and
// sorted order disagree. A refusal naming whichever entry the test happened
// to create first would pass a version of this test that built them in sorted
// order, and two runs over one tree could then report different directories.
func TestSoleBenchNamesTheFirstDamagedEntryDeterministically(t *testing.T) {
	outer := t.TempDir()
	base := filepath.Join(outer, UserBaseName)
	later := filepath.Join(base, "d00000000002")
	earlier := filepath.Join(base, "d00000000001")
	write(t, filepath.Join(later, WorkbenchAnchor), damageShapes[1].text)
	write(t, filepath.Join(earlier, WorkbenchAnchor), damageShapes[2].text)

	found, ambiguous, err := soleBench(base)
	if found != "" || len(ambiguous) != 0 {
		t.Fatalf("a base holding only damaged entries has no candidate, got found %q and ambiguous %v", found, ambiguous)
	}
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if refusal.Name != contract.DamagedBench {
		t.Errorf("refusal name: wanted %s, got %s", contract.DamagedBench, refusal.Name)
	}
	if refusal.Detail != earlier {
		t.Errorf("wanted the alphabetically first damaged entry %q, got %q", earlier, refusal.Detail)
	}
}

// TestAnExplicitPointerAtADamagedWorkbenchStillResolves asserts AC-4. The
// override branch tests only that a workbench.md exists where the caller
// pointed, and never runs recognition, so --workbench and DINAH_WORKBENCH
// still reach a damaged workbench exactly as they did before this card. That
// is what lets a reader act on the refusal's own next step: the ambient climb
// refuses and names the directory, and passing that directory back gets them
// past discovery to whatever Open can say about the content.
func TestAnExplicitPointerAtADamagedWorkbenchStillResolves(t *testing.T) {
	workbench := containedPath(t.TempDir())
	write(t, filepath.Join(workbench, WorkbenchAnchor), damageShapes[3].text)

	// The ambient climb over the same directory refuses, which is what makes
	// the override's silence a difference between two call shapes rather
	// than a fixture that was never damaged.
	climbed, _, err := Discover(workbench, "", "", "")
	refusal, ok := err.(*contract.Refusal)
	if !ok || refusal.Name != contract.DamagedBench {
		t.Fatalf("the ambient climb over this fixture answered %q and %v rather than refusing %s, so the override case below proves nothing", climbed, err, contract.DamagedBench)
	}

	found, source, passed, err := DiscoverSource(workbench, workbench, SourceFlag, "", "", "")
	if err != nil {
		t.Fatalf("an explicit pointer at a damaged workbench should resolve, got %v", err)
	}
	if found != workbench {
		t.Errorf("wanted the pointed-at directory %q, got %q", workbench, found)
	}
	if source != SourceFlag {
		t.Errorf("wanted the flag named as the source, got %q", source)
	}
	if len(passed) != 0 {
		t.Errorf("the override branch runs no walk and passes nothing over, got %v", passed)
	}
}
