package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench/compattest"
	"dinah/internal/contract"
)

// compatDir is where the compatibility fixtures live, relative to this
// package.
const compatDir = "testdata/compat"

// compatRepoPrefix is the same directory named from the repository root. The
// manifest digests a fixture's files under their repository-relative paths, so
// the digest is the same in every checkout and does not depend on where the
// repository sits on a machine.
const compatRepoPrefix = "internal/bench/testdata/compat"

// manifestName is the file listing each fixture with the digest of its
// contents.
const manifestName = "manifest.json"

// compatFixtures lists the fixture directory names under testdata/compat, in
// sorted order. A test globs rather than naming them, so a fixture added later
// is exercised without any test code changing.
func compatFixtures(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(compatDir)
	if err != nil {
		t.Fatalf("read %s: %v", compatDir, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		t.Fatalf("%s carries no fixture directory", compatDir)
	}
	sort.Strings(names)
	return names
}

// declaredProfile reads the profile string a fixture's anchor declares,
// without opening the workbench, because the admission of that string is what
// several of these tests are asserting. Reads through this package's own
// ReadText and ParseAnchor directly rather than through compattest: compattest
// is imported by this file (for the fixture-manifest shape below), and this
// file is itself part of package bench's own test binary, so a shared helper
// that also imported bench back out of compattest would close an import
// cycle. See the doc comment on compattest for the same reasoning from the
// other side.
func declaredProfile(t *testing.T, fixture string) string {
	t.Helper()
	text, err := ReadText(filepath.Join(compatDir, fixture, WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", fixture, err)
	}
	fm, _ := ParseAnchor(text)
	return fm.Value("profile")
}

// TestAdmitProfileReadsThePublishedLineAndRefusesTheRest asserts the window
// over the whole accept and refuse table, including that the two retired
// spellings no build ever stamped are read literally and refused.
func TestAdmitProfileReadsThePublishedLineAndRefusesTheRest(t *testing.T) {
	admitted := []string{
		"dinah-core/0.1",
		"dinah-core/0.2",
		"dinah-core/0.3",
		"dinah-core/0.4",
		"dinah-core/0.5",
		"dinah-core/0.6",
		"dinah-core/1.0",
	}
	refused := []string{
		"dinah-core/0.0",
		"dinah-core/0.7",
		"dinah-core/1.1",
		"dinah-core/2.0",
		"dinah-core/3.0",
		"dinah-core/4.0",
		"dinah-core/9.9",
	}
	for _, declared := range admitted {
		if _, _, err := admitProfile(declared); err != nil {
			t.Errorf("admitProfile(%q) refused it: %v", declared, err)
		}
	}
	for _, declared := range refused {
		_, _, err := admitProfile(declared)
		refusal := &contract.Refusal{}
		if !errors.As(err, &refusal) {
			t.Errorf("admitProfile(%q) returned %v, wanted a refusal", declared, err)
			continue
		}
		if refusal.Name != contract.UnsupportedVer {
			t.Errorf("admitProfile(%q) refused %s, wanted %s", declared, refusal.Name, contract.UnsupportedVer)
		}
		if refusal.Extra["floor"] == "" || refusal.Extra["ceiling"] == "" {
			t.Errorf("admitProfile(%q) refused without naming the window: %v", declared, refusal.Extra)
		}
	}
}

// TestAMalformedProfileIsASentinelRatherThanARefusal asserts that a string
// that will not parse comes back as errProfileMalformed, which is what lets
// each call site keep the malformed sentence it raises today.
func TestAMalformedProfileIsASentinelRatherThanARefusal(t *testing.T) {
	for _, declared := range []string{"", "dinah/1.0", "dinah-core/1", "dinah-core/x.y"} {
		_, _, err := admitProfile(declared)
		if !errors.Is(err, errProfileMalformed) {
			t.Errorf("admitProfile(%q) returned %v, wanted errProfileMalformed", declared, err)
		}
	}
}

// TestTheRetiredSpellingResolvesOnlyWhileTheCeilingSitsBelowIt asserts both
// readings of dinah-core/1.0: this build resolves it to the revision the
// profile's 0.4 changelog entry renamed it to, and a build whose ceiling has
// reached 1.0 reads it literally and admits it anyway.
func TestTheRetiredSpellingResolvesOnlyWhileTheCeilingSitsBelowIt(t *testing.T) {
	floor := [2]int{ProfileFloorMajor, ProfileFloorMinor}
	ceiling := [2]int{ProfileMajor, ProfileMinor}
	if !sortsBelow(ceiling, retiredProfileName) {
		t.Fatalf("this build's ceiling %v no longer sorts below %v, so the alias is off and this test needs rewriting", ceiling, retiredProfileName)
	}
	major, minor, err := admitProfileWithin("dinah-core/1.0", floor, ceiling)
	if err != nil {
		t.Fatalf("this build refused dinah-core/1.0: %v", err)
	}
	if [2]int{major, minor} != retiredProfileMeans {
		t.Errorf("this build read dinah-core/1.0 as (%d, %d), wanted %v", major, minor, retiredProfileMeans)
	}
	for _, raised := range [][2]int{{1, 0}, {1, 4}, {2, 0}} {
		major, minor, err := admitProfileWithin("dinah-core/1.0", floor, raised)
		if err != nil {
			t.Errorf("a build with ceiling %v refused dinah-core/1.0: %v", raised, err)
			continue
		}
		if [2]int{major, minor} != retiredProfileName {
			t.Errorf("a build with ceiling %v read dinah-core/1.0 as (%d, %d), wanted %v", raised, major, minor, retiredProfileName)
		}
	}
}

// TestEveryCompatFixtureOpensAndReads is the open test: every fixture
// directory under testdata/compat opens under this build and gives up its
// states and its cards. A fixture added later is picked up by the glob with no
// change here.
//
// What this test does not prove (spec section 6.5): it shows the binary
// accepts a committed tree and reads it, and it settles nothing about
// completeness. A hand-built tree carrying only the path names and keys this
// test happens to touch would open and read here as readily as a genuine
// capture; the coverage claim belongs to the sample and coverage alarms
// above, not to this test.
func TestEveryCompatFixtureOpensAndReads(t *testing.T) {
	for _, fixture := range compatFixtures(t) {
		b, err := Open(filepath.Join(compatDir, fixture))
		if err != nil {
			t.Errorf("open %s: %v", fixture, err)
			continue
		}
		if len(b.States) == 0 {
			t.Errorf("%s opened with no states", fixture)
		}
		cards, err := b.Cards()
		if err != nil {
			t.Errorf("list the cards of %s: %v", fixture, err)
			continue
		}
		if len(cards) == 0 {
			t.Errorf("%s opened with no cards", fixture)
		}
	}
}

// TestEveryCompatFixtureSurvivesTheInterchangePath drives the second admission
// site for every fixture, which is the path dinah init --from takes through
// Open, Export and ReadDefinition.
func TestEveryCompatFixtureSurvivesTheInterchangePath(t *testing.T) {
	for _, fixture := range compatFixtures(t) {
		b, err := Open(filepath.Join(compatDir, fixture))
		if err != nil {
			t.Errorf("open %s: %v", fixture, err)
			continue
		}
		exported, err := b.Export()
		if err != nil {
			t.Errorf("export %s: %v", fixture, err)
			continue
		}
		if _, err := ReadDefinition(exported); err != nil {
			t.Errorf("read the exported definition of %s back: %v", fixture, err)
		}
	}
}

// TestSomeFixtureDeclaresTheRevisionThisBuildStamps is the bump alarm. Moving
// ProfileMajor or ProfileMinor fails the build in the commit that moves it, and
// the way to make it pass is to capture a fixture in the outgoing shape.
func TestSomeFixtureDeclaresTheRevisionThisBuildStamps(t *testing.T) {
	for _, fixture := range compatFixtures(t) {
		if declaredProfile(t, fixture) == ProfileVersion {
			return
		}
	}
	t.Fatalf("no fixture under %s declares %s. Replay populate.txt through this build, commit the capture as %s/%s, and mark its manifest row sample: true", compatDir, ProfileVersion, compatDir, strings.ReplaceAll(ProfileVersion, "/", "-"))
}

// TestSomeFixtureDeclaresTheFloor keeps the oldest revision this build opens
// from losing its own sample, which is the one part of the deletion route an
// in-tree assertion can close.
//
// The declared string is resolved before it is compared, rather than matched
// against ProfileFloorVersion as text, because the floor revision reaches a
// workbench on disk under the spelling the profile's 0.4 changelog entry
// retired. A fixture declaring dinah-core/1.0 is a sample of the floor while
// this build resolves that spelling.
func TestSomeFixtureDeclaresTheFloor(t *testing.T) {
	floor := [2]int{ProfileFloorMajor, ProfileFloorMinor}
	for _, fixture := range compatFixtures(t) {
		major, minor, err := admitProfile(declaredProfile(t, fixture))
		if err != nil {
			continue
		}
		if [2]int{major, minor} == floor {
			return
		}
	}
	t.Fatalf("no fixture under %s samples the floor revision, which this build reads as %s and which reaches disk spelled %s", compatDir, ProfileFloorVersion, ProfileName+"/1.0")
}

// TestEveryFixtureDeclaresARevisionThisBuildAdmits is the floor alarm. Raising
// ProfileFloorMinor past a committed fixture turns this red rather than quietly
// narrowing what the tool opens.
func TestEveryFixtureDeclaresARevisionThisBuildAdmits(t *testing.T) {
	for _, fixture := range compatFixtures(t) {
		declared := declaredProfile(t, fixture)
		if _, _, err := admitProfile(declared); err != nil {
			t.Errorf("%s declares %s and this build's window refuses it: %v", fixture, declared, err)
		}
	}
}

// TestFloorHasNotMovedSincePromiseBound asserts nothing while PromiseBoundAt is
// empty, because the operator ruled that the never-refuse promise starts
// binding at the first stable release rather than at a dev build. The release
// card that cuts that build sets all three constants together, and from then on
// this test refuses a floor that has moved.
func TestFloorHasNotMovedSincePromiseBound(t *testing.T) {
	if PromiseBoundAt == "" {
		return
	}
	recorded := [2]int{PromiseFloorMajor, PromiseFloorMinor}
	current := [2]int{ProfileFloorMajor, ProfileFloorMinor}
	if current != recorded {
		t.Fatalf("the promise bound at %s with the floor at %v and the floor now reads %v; a release must never refuse a workbench an earlier release wrote", PromiseBoundAt, recorded, current)
	}
}

// TestTheFixtureManifestMatchesWhatIsCommitted is the edit alarm. It cannot
// make a fixture unwritable, and what it does instead is keep the build red
// until somebody re-blesses the digest, so an edit to an already-published
// shape carries its own line in the same diff.
func TestTheFixtureManifestMatchesWhatIsCommitted(t *testing.T) {
	manifest := readManifest(t)
	rows := map[string]compattest.FixtureRow{}
	for _, row := range manifest.Fixtures {
		rows[row.Directory] = row
	}
	fixtures := map[string]bool{}
	for _, fixture := range compatFixtures(t) {
		fixtures[fixture] = true
		row, ok := rows[fixture]
		if !ok {
			t.Errorf("%s has no row in %s", fixture, manifestName)
			continue
		}
		digest := digestFixture(t, fixture)
		if digest != row.Digest {
			t.Errorf("%s digests to %s and its row in %s carries %s", fixture, digest, manifestName, row.Digest)
		}
	}
	for _, row := range manifest.Fixtures {
		if !fixtures[row.Directory] {
			t.Errorf("%s carries a row for %s and no such fixture directory exists", manifestName, row.Directory)
		}
	}
}

// TestExactlyOneFixtureIsMarkedTheSampleForThisRevision asserts the mark the
// shape comparison in cmd/dinah reads. The comparison runs against one fixture
// rather than the union of every fixture declaring the revision, because a
// union lets a second fixture cover the first one's gaps.
func TestExactlyOneFixtureIsMarkedTheSampleForThisRevision(t *testing.T) {
	manifest := readManifest(t)
	var marked []string
	for _, row := range manifest.Fixtures {
		if !row.Sample {
			continue
		}
		if declaredProfile(t, row.Directory) != ProfileVersion {
			continue
		}
		marked = append(marked, row.Directory)
	}
	if len(marked) != 1 {
		t.Fatalf("%d fixtures declaring %s carry sample: true in %s (%v), wanted exactly one", len(marked), ProfileVersion, manifestName, marked)
	}
}

// readManifest reads the fixture manifest.
func readManifest(t *testing.T) compattest.FixtureManifest {
	t.Helper()
	manifest, err := compattest.ReadFixtureManifest(compatDir)
	if err != nil {
		t.Fatalf("read %s: %v", manifestName, err)
	}
	return manifest
}

// digestFixture hashes a fixture's files in sorted repository-relative path
// order, feeding each path and then its bytes into the hash, so a renamed file
// changes the digest as surely as an edited one does.
func digestFixture(t *testing.T, fixture string) string {
	t.Helper()
	root := filepath.Join(compatDir, fixture)
	var paths []string
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walk %s: %v", fixture, err)
	}
	sort.Strings(paths)
	sum := sha256.New()
	for _, relative := range paths {
		sum.Write([]byte(compatRepoPrefix + "/" + fixture + "/" + relative))
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s of %s: %v", relative, fixture, err)
		}
		sum.Write(data)
	}
	return hex.EncodeToString(sum.Sum(nil))
}
