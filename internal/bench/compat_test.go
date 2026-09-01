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
//
// The published line the window admits runs from the rename dinah-287 raised
// the floor to up to the claim this build stamps, which dinah-358 moved to
// 0.9. Every revision below the floor is still published
// and still readable, through the vocabulary migration rather than through
// this window, and the third table below is the one that says so: a revision
// in that window meets needs-vocabulary-migration at the gate Open applies,
// not the unsupported-version this window gives a revision nothing reads.
func TestAdmitProfileReadsThePublishedLineAndRefusesTheRest(t *testing.T) {
	admitted := []string{
		"dinah-core/0.7",
		"dinah-core/0.8",
		"dinah-core/0.9",
	}
	refused := []string{
		"dinah-core/0.0",
		// The case CORE-BENCH-4's major-only text could not reach, and the one
		// CORE-BENCH-5 names: this build's own major, a minor above the
		// ceiling it implements. The example moved from 0.8 to 0.10 when
		// dinah-358 raised the claim to 0.9, because a revision this build now
		// implements cannot stand for one it does not.
		"dinah-core/0.10",
		"dinah-core/1.1",
		"dinah-core/2.0",
		"dinah-core/3.0",
		"dinah-core/4.0",
		"dinah-core/9.9",
	}
	migrates := []string{
		"dinah-core/0.1",
		"dinah-core/0.2",
		"dinah-core/0.3",
		"dinah-core/0.4",
		"dinah-core/0.5",
		"dinah-core/0.6",
		"dinah-core/1.0",
	}
	for _, declared := range migrates {
		_, _, err := admitProfileAfterVocabulary(declared)
		refusal := &contract.Refusal{}
		if !errors.As(err, &refusal) {
			t.Errorf("the gate returned %v for %q, wanted a refusal", err, declared)
			continue
		}
		if refusal.Name != contract.NeedsVocabularyMigration {
			t.Errorf("the gate refused %q with %s, wanted %s", declared, refusal.Name, contract.NeedsVocabularyMigration)
		}
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
	// The window is the pre-vocabulary one rather than this build's own,
	// because dinah-287 raised the live floor above the revision the alias
	// resolves to. What this test is about is the reading, not the window, and
	// the pre-vocabulary window is where that reading is still admitted;
	// TestARaisedFloorTellsAResolvedRevisionToMigrate below is what asserts
	// the live floor's own answer to the same string.
	floor := PreVocabularyFloor
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

// TestResolveProfileReportsTheReadingItUsed asserts the resolver's own
// contract: it reads a declared string as splitProfile does, applies the
// retired-name alias when the build's ceiling still calls for it, and says
// which of the two readings came back.
//
// The alias condition itself is not what this test stands on;
// TestTheRetiredSpellingResolvesOnlyWhileTheCeilingSitsBelowIt already holds
// that through admitProfileWithin. What is asserted here is that the flag
// agrees with the pair, so a caller picking a refusal from the flag picks the
// one the pair earned.
func TestResolveProfileReportsTheReadingItUsed(t *testing.T) {
	for _, ceiling := range [][2]int{{0, 6}, {ProfileMajor, ProfileMinor}} {
		major, minor, aliased, ok := resolveProfile("dinah-core/1.0", ceiling)
		if !ok {
			t.Fatalf("a ceiling of %v read dinah-core/1.0 as malformed", ceiling)
		}
		if [2]int{major, minor} != retiredProfileMeans || !aliased {
			t.Errorf("a ceiling of %v read dinah-core/1.0 as (%d, %d) aliased=%v, wanted %v aliased=true", ceiling, major, minor, aliased, retiredProfileMeans)
		}
	}
	for _, ceiling := range [][2]int{{1, 0}, {2, 0}} {
		major, minor, aliased, ok := resolveProfile("dinah-core/1.0", ceiling)
		if !ok {
			t.Fatalf("a ceiling of %v read dinah-core/1.0 as malformed", ceiling)
		}
		if [2]int{major, minor} != retiredProfileName || aliased {
			t.Errorf("a ceiling of %v read dinah-core/1.0 as (%d, %d) aliased=%v, wanted %v aliased=false", ceiling, major, minor, aliased, retiredProfileName)
		}
	}
	// The build's own ceiling is what checkColumnSlugsWithin passes, so this
	// is the reading the slug check computes for the workbench this
	// repository runs itself on.
	major, _, aliased, ok := resolveProfile("dinah-core/1.0", [2]int{ProfileMajor, ProfileMinor})
	if !ok || major != 0 || !aliased {
		t.Errorf("under this build's own ceiling dinah-core/1.0 resolved to major %d aliased=%v ok=%v, wanted major 0 aliased=true", major, aliased, ok)
	}
}

// TestResolveProfileReadsAMalformedStringAsASentinel asserts that a string
// that does not parse comes back as not-ok rather than as a pair, over the
// same inputs TestAMalformedProfileIsASentinelRatherThanARefusal drives
// through admitProfile.
func TestResolveProfileReadsAMalformedStringAsASentinel(t *testing.T) {
	for _, declared := range []string{"", "dinah/1.0", "dinah-core/1", "dinah-core/x.y"} {
		if _, _, _, ok := resolveProfile(declared, [2]int{ProfileMajor, ProfileMinor}); ok {
			t.Errorf("%q parsed, wanted the malformed sentinel", declared)
		}
	}
}

// TestARevisionTheAliasResolvedIsToldToMigrateRatherThanRefusedAsUnknown
// asserts the gate's third case: a revision the retired-name alias already
// resolved, which a floor raised past the alias's own output then rejects, is
// refused by a name that says a migration carries the workbench forward. A
// revision that never went through the alias keeps the older refusal, which is
// what the aliased flag exists to tell apart.
//
// No shipped build reaches either case, because ProfileFloorMinor is 1 and the
// alias resolves to 0.1. The floor is driven directly here for the same reason
// TestTheRetiredSpellingResolvesOnlyWhileTheCeilingSitsBelowIt drives a
// ceiling no shipped build carries.
func TestARevisionTheAliasResolvedIsToldToMigrateRatherThanRefusedAsUnknown(t *testing.T) {
	raised := [2]int{0, 7}
	if !sortsBelow(retiredProfileMeans, raised) {
		t.Fatalf("a floor of %v no longer sits above %v, so this test drives nothing", raised, retiredProfileMeans)
	}
	_, _, err := admitProfileWithin("dinah-core/1.0", raised, raised)
	var refusal *contract.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("a floor of %v admitted dinah-core/1.0 or refused it with %v, wanted a refusal", raised, err)
	}
	if refusal.Name != contract.NeedsVocabularyMigration {
		t.Errorf("dinah-core/1.0 below a raised floor was refused %s, wanted %s", refusal.Name, contract.NeedsVocabularyMigration)
	}

	_, _, err = admitProfileWithin("dinah-core/0.1", raised, raised)
	if !errors.As(err, &refusal) {
		t.Fatalf("a floor of %v admitted dinah-core/0.1 or refused it with %v, wanted a refusal", raised, err)
	}
	if refusal.Name != contract.UnsupportedVer {
		t.Errorf("dinah-core/0.1 below a raised floor was refused %s, wanted %s", refusal.Name, contract.UnsupportedVer)
	}
}

// openFixture opens one compat fixture through the opener its own declared
// revision calls for. A fixture inside the pre-vocabulary window is written in
// the retired state and substate vocabulary, which Open refuses by name and
// OpenPreVocabulary is the one reader of; every other fixture goes through
// Open exactly as before. The choice is made from the fixture's own anchor
// rather than from its directory name, so a fixture added later is sorted the
// same way with no edit here.
func openFixture(t *testing.T, fixture string) (*Bench, error) {
	t.Helper()
	root := filepath.Join(compatDir, fixture)
	major, minor, ok, err := ClassifyVocabulary(root)
	if err != nil {
		return nil, err
	}
	if ok && WithinPreVocabulary(major, minor) {
		return OpenPreVocabulary(root)
	}
	return Open(root)
}

// TestEveryCompatFixtureOpensAndReads is the open test: every fixture
// directory under testdata/compat opens under this build and gives up its
// columns and its cards. A fixture added later is picked up by the glob with no
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
		b, err := openFixture(t, fixture)
		if err != nil {
			t.Errorf("open %s: %v", fixture, err)
			continue
		}
		if len(b.Columns) == 0 {
			t.Errorf("%s opened with no columns", fixture)
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
//
// Each card is also compared against the column it stood in before the run,
// read off the fixture's own anchors, rather than merely counted. For a
// pre-vocabulary fixture that path runs through the migration, and a card
// whose column the migration destroyed still opens, still exports and still
// reads its definition back: this test used to ask only whether those three
// returned an error, and it passed against a migration that replaced every
// card's column with its condition. An existence assertion cannot tell a
// carried value from a lost one, so the value is what is asserted.
func TestEveryCompatFixtureSurvivesTheInterchangePath(t *testing.T) {
	for _, fixture := range compatFixtures(t) {
		source, stood := interchangeSource(t, fixture)
		b, err := Open(source)
		if err != nil {
			t.Errorf("open %s: %v", fixture, err)
			continue
		}
		cards, err := b.Cards()
		if err != nil {
			t.Errorf("list the cards of %s: %v", fixture, err)
			continue
		}
		if len(cards) == 0 {
			t.Errorf("%s opened with no cards, so nothing here was compared", fixture)
		}
		for _, card := range cards {
			want, recorded := stood[card.ID]
			if !recorded {
				t.Errorf("%s reports a card %s that its own anchors did not carry before the run", fixture, card.ID)
				continue
			}
			if card.Column != want {
				t.Errorf("a card of %s stands in column %q and stood in %q before the run", fixture, card.Column, want)
			}
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

// fixtureColumns reads the column identifier every live card of a workbench
// stands in, straight off the anchors and under whichever key that
// workbench's own vocabulary spells the column with. It opens nothing,
// because the pre-vocabulary shape is exactly what the ordinary opener
// refuses and reading these cards through LoadCard would take the flow
// position for the condition, which is the misread the migration exists to
// prevent.
func fixtureColumns(t *testing.T, root, key string) map[string]string {
	t.Helper()
	stood := map[string]string{}
	dir := filepath.Join(root, CardsDir)
	for _, id := range ListIDs(dir) {
		path := filepath.Join(dir, id, CardAnchor)
		if !Exists(path) {
			continue
		}
		text, err := ReadText(path)
		if err != nil {
			t.Fatalf("read the card %s: %v", id, err)
		}
		fm, _ := ParseAnchor(text)
		column := fm.Value(key)
		if column == "" {
			t.Fatalf("the card %s carries no %s key, so this fixture is not the shape it was classified as", id, key)
		}
		stood[id] = column
	}
	if len(stood) == 0 {
		t.Fatalf("%s holds no card, so nothing here can be compared across the run", root)
	}
	return stood
}

// interchangeSource answers the directory the interchange path should read a
// fixture from. A fixture this build opens ordinarily is read where it sits. A
// pre-vocabulary fixture is copied to a temporary directory and carried across
// the rename first, because the interchange path is reached through Open and
// the only route a workbench of that age has to Open is the migration. That
// makes this test the one that proves the migration's output survives the
// second admission site, which is worth more than skipping the old fixtures.
//
// It answers the columns its cards stood in as well as the directory, read
// off the fixture where it sits and before anything is migrated, so the
// caller compares what came out against a value nothing under test computed.
// Which key holds that value is decided by the classification rather than
// guessed at, since the retired vocabulary spells the column `state` and the
// current one spells it `column`.
func interchangeSource(t *testing.T, fixture string) (string, map[string]string) {
	t.Helper()
	root := filepath.Join(compatDir, fixture)
	major, minor, ok, err := ClassifyVocabulary(root)
	if err != nil {
		t.Fatalf("classify %s: %v", fixture, err)
	}
	if !ok || !WithinPreVocabulary(major, minor) {
		return root, fixtureColumns(t, root, columnKey)
	}
	stood := fixtureColumns(t, root, preVocabularyColumnKey)
	copied := filepath.Join(t.TempDir(), fixture)
	copyTree(t, root, copied)
	opened, err := OpenPreVocabulary(copied)
	if err != nil {
		t.Fatalf("open %s for migration: %v", fixture, err)
	}
	if _, err := MigrateVocabulary(opened); err != nil {
		t.Fatalf("migrate %s: %v", fixture, err)
	}
	return copied, stood
}

// copyTree copies a fixture into a directory a test may write to, since a
// migration rewrites what it is given and a fixture in the tree is evidence.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", from, err)
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
//
// A fixture the floor has passed is admitted here when the migration chain
// still reaches it, which today means the pre-vocabulary window dinah-287
// opened beneath the raised floor. That is not a hole in the alarm: the thing
// the alarm defends is that no committed fixture becomes unreadable, and a
// fixture the vocabulary migration carries forward is still read, by the
// reader that exists for it. A floor raise past a revision with no migration
// behind it fails here exactly as it did before.
func TestEveryFixtureDeclaresARevisionThisBuildAdmits(t *testing.T) {
	for _, fixture := range compatFixtures(t) {
		declared := declaredProfile(t, fixture)
		if _, _, err := admitProfile(declared); err == nil {
			continue
		}
		if _, _, err := admitPreVocabularyProfile(declared); err == nil {
			continue
		}
		t.Errorf("%s declares %s and neither this build's window nor its vocabulary migration reads it", fixture, declared)
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
