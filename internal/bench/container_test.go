package bench

import (
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// currentBenchDefinition is benchDefinition raised to the storage format the
// containment rule arrived at. The cases below need both: the older one to
// show that a workbench predating the rule still opens where it always did,
// and this one to show that a workbench declaring the rule is held to it.
var currentBenchDefinition = strings.Replace(benchDefinition, "format: 1", "format: 2", 1)

// plantBench writes one workbench, anchor and single column, at the directory
// it is given, and answers that directory. The definition decides which
// storage format the workbench declares, which is the whole of what the
// containment gate reads.
func plantBench(t *testing.T, root, definition string) string {
	t.Helper()
	write(t, filepath.Join(root, WorkbenchAnchor), definition)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	return root
}

// TestAWorkbenchIdentifierIsAUUIDVersionSeven asserts dinah-285 AC-1. Every
// identifier NewWorkbenchID mints is 32 lowercase hex characters that
// IsWorkbenchID accepts and that decode to a UUID whose version field is 7 and
// whose variant field is 10, per RFC 9562, and the timestamps of a run of them
// never go backwards.
//
// A thousand values are decoded rather than one, because the version and
// variant fields are set by masking bytes that are otherwise random: a mask
// written wrongly leaves most values correct and a few not, and one sample
// would find that roughly never.
func TestAWorkbenchIdentifierIsAUUIDVersionSeven(t *testing.T) {
	previous := uint64(0)
	for i := 0; i < 1000; i++ {
		id, err := NewWorkbenchID()
		if err != nil {
			t.Fatalf("mint: %v", err)
		}
		if len(id) != WorkbenchIDLength {
			t.Fatalf("%s is %d characters, wanted %d", id, len(id), WorkbenchIDLength)
		}
		if id != strings.ToLower(id) {
			t.Fatalf("%s carries an upper-case character", id)
		}
		raw, err := hex.DecodeString(id)
		if err != nil {
			t.Fatalf("%s does not decode as hex: %v", id, err)
		}
		if version := raw[6] >> 4; version != 7 {
			t.Fatalf("%s carries version %d, wanted 7", id, version)
		}
		if variant := raw[8] >> 6; variant != 0b10 {
			t.Fatalf("%s carries variant bits %02b, wanted 10", id, variant)
		}
		if !IsWorkbenchID(id) {
			t.Fatalf("IsWorkbenchID refuses %s, which this build minted", id)
		}
		milliseconds := uint64(0)
		for _, b := range raw[:6] {
			milliseconds = milliseconds<<8 | uint64(b)
		}
		if milliseconds < previous {
			t.Fatalf("%s carries the timestamp %d after %d, so the identifiers are not time-ordered", id, milliseconds, previous)
		}
		previous = milliseconds
	}
}

// TestTheTwoIdentifierPredicatesAreDisjoint asserts dinah-285 AC-2. No string
// satisfies both IsID and IsWorkbenchID, in either direction, so a legacy
// workbench directory and a migrated one can never be mistaken for each other
// and a workbench identifier can never be read as a card, a comment or an
// attachment.
func TestTheTwoIdentifierPredicatesAreDisjoint(t *testing.T) {
	for i := 0; i < 200; i++ {
		narrow, err := NewID()
		if err != nil {
			t.Fatalf("mint an entity identifier: %v", err)
		}
		if !IsID(narrow) {
			t.Fatalf("IsID refuses %s, which this build minted", narrow)
		}
		if IsWorkbenchID(narrow) {
			t.Fatalf("IsWorkbenchID accepts the 12-hex entity identifier %s", narrow)
		}
		wide, err := NewWorkbenchID()
		if err != nil {
			t.Fatalf("mint a workbench identifier: %v", err)
		}
		if IsID(wide) {
			t.Fatalf("IsID accepts the workbench identifier %s", wide)
		}
	}
	// The hand-written cases the generated ones cannot reach: a wide string
	// that is hex but carries the wrong version or variant is refused too, so
	// the predicate is not merely a length test wearing a UUID's name.
	for _, id := range []string{
		"0199a1b2c3d46abc8000000000000001",
		"0199a1b2c3d47abc4000000000000001",
		"0199A1B2C3D47ABC8000000000000001",
		"0199a1b2c3d47abc800000000000000",
	} {
		if IsWorkbenchID(id) {
			t.Errorf("IsWorkbenchID accepts %s, which is not one this build would mint", id)
		}
	}
}

// TestAContainedWorkbenchIsTheOnlyOneTheRuleAdmits asserts dinah-285 AC-4 for
// the opener. A workbench declaring the storage format the containment rule
// arrived at opens inside a container and is refused outside one, by name; a
// workbench declaring the older format, or declaring none, opens outside one
// exactly as it always did.
//
// The older format is asserted first and deliberately. It is the behaviour
// that stood before this rule, over a fixture built the same way, so a change
// that refused every bare workbench rather than only the ones declaring the
// new format fails here rather than passing quietly.
func TestAContainedWorkbenchIsTheOnlyOneTheRuleAdmits(t *testing.T) {
	older := plantBench(t, filepath.Join(t.TempDir(), "workbench"), benchDefinition)
	if _, err := Open(older); err != nil {
		t.Fatalf("a bare workbench declaring the older format should open as it always did, got %v", err)
	}
	none := strings.Replace(benchDefinition, "format: 1\n", "", 1)
	silent := plantBench(t, filepath.Join(t.TempDir(), "workbench"), none)
	if _, err := Open(silent); err != nil {
		t.Fatalf("a bare workbench declaring no format at all should open, got %v", err)
	}

	bare := plantBench(t, filepath.Join(t.TempDir(), "workbench"), currentBenchDefinition)
	_, err := Open(bare)
	refusal, refused := err.(*contract.Refusal)
	if !refused {
		t.Fatalf("a bare workbench declaring the current format should be refused, got %v", err)
	}
	if refusal.Name != contract.NeedsContainerMigration {
		t.Errorf("refusal name: wanted %s, got %s", contract.NeedsContainerMigration, refusal.Name)
	}
	if refusal.Detail != bare {
		t.Errorf("the refusal should name the directory, wanted %q, got %q", bare, refusal.Detail)
	}
	if _, err := OpenUncontained(bare); err != nil {
		t.Errorf("the uncontained opener should read the same directory, got %v", err)
	}

	contained := plantBench(t, containedPath(t.TempDir()), currentBenchDefinition)
	opened, err := Open(contained)
	if err != nil {
		t.Fatalf("a contained workbench declaring the current format should open, got %v", err)
	}
	if opened.ID != filepath.Base(contained) {
		t.Errorf("the workbench identifier should be its own directory name, wanted %q, got %q", filepath.Base(contained), opened.ID)
	}
}

// TestDiscoveryNeverReturnsABareWorkbench asserts dinah-285 AC-5. A tree
// holding a bare recognized workbench, a contained legacy-width one, a
// contained wide-id one and a foreign anchor is walked from inside the bare
// one: nothing is found there, the bare directory is named in the refusal, and
// the foreign anchor is reported separately as passed over rather than as the
// same thing.
func TestDiscoveryNeverReturnsABareWorkbench(t *testing.T) {
	tree := t.TempDir()
	bare := plantBench(t, filepath.Join(tree, "bare"), benchDefinition)
	legacy := plantBench(t, filepath.Join(tree, "legacy", UserBaseName, "d00000000001"), benchDefinition)
	wide := plantBench(t, containedPath(filepath.Join(tree, "wide")), currentBenchDefinition)
	foreign := filepath.Join(tree, "foreign")
	write(t, filepath.Join(foreign, WorkbenchAnchor), foreignAnchor)

	_, _, err := Discover(bare, "", filepath.Join(tree, "home"), "")
	refusal, refused := err.(*contract.Refusal)
	if !refused {
		t.Fatalf("the walk from inside a bare workbench should refuse, got %v", err)
	}
	if refusal.Name != contract.NoWorkbenchFound {
		t.Errorf("refusal name: wanted %s, got %s", contract.NoWorkbenchFound, refusal.Name)
	}
	if !strings.Contains(refusal.Extra["bare"], bare) {
		t.Errorf("the refusal should name the bare workbench it walked past, got %q", refusal.Extra["bare"])
	}

	// The foreign anchor is reported on the other list, so a reader can tell a
	// document that was never Dinah's from a workbench that stopped being
	// found. A walk that put both on one list would tell them the same thing
	// about two different situations.
	_, passed, err := Discover(foreign, "", filepath.Join(tree, "home"), "")
	if err == nil {
		t.Fatal("a foreign anchor should not resolve a workbench")
	}
	foreignRefusal, refused := err.(*contract.Refusal)
	if !refused {
		t.Fatalf("wanted a refusal over the foreign anchor, got %v", err)
	}
	if foreignRefusal.Extra["bare"] != "" {
		t.Errorf("a foreign anchor was reported as a bare workbench: %q", foreignRefusal.Extra["bare"])
	}
	if len(passed) != 0 {
		t.Errorf("Discover returns no passed list beside a refusal, got %v", passed)
	}

	// Both contained shapes are still found, which is what says the rule
	// removed the bare case and nothing else.
	for _, where := range []string{legacy, wide} {
		found, _, err := Discover(filepath.Dir(filepath.Dir(where)), "", filepath.Join(tree, "home"), "")
		if err != nil {
			t.Errorf("the contained workbench at %s should be found, got %v", where, err)
			continue
		}
		if found != where {
			t.Errorf("wanted %q, got %q", where, found)
		}
	}
}

// TestSoleBenchStillFindsBothWidths asserts dinah-285 AC-6. A container's
// listing admits the legacy 12-hex width and the wide identifier alike, each
// describes as itself, and each opens by its own path. Candidate.ID is the
// directory name on the migrated one and empty on the one the migration has
// not reached, which is the signal the container repair acts on rather than a
// defect.
func TestSoleBenchStillFindsBothWidths(t *testing.T) {
	base := filepath.Join(t.TempDir(), UserBaseName)
	legacy := plantBench(t, filepath.Join(base, "d00000000001"), benchDefinition)
	wide := plantBench(t, filepath.Join(base, fixtureWorkbenchID), currentBenchDefinition)

	listed := ListWorkbenchIDs(base)
	sort.Strings(listed)
	want := []string{"0199a1b2c3d47abc8000000000000001", "d00000000001"}
	sort.Strings(want)
	if strings.Join(listed, " ") != strings.Join(want, " ") {
		t.Errorf("the container listing answered %v, wanted both widths", listed)
	}

	// Two workbenches in one container is the ordinary ambiguity, and it is
	// asserted here so that neither width is silently dropped: a listing that
	// saw only one of them would resolve rather than report a tie.
	found, ambiguous, err := soleBench(base)
	if err != nil {
		t.Fatalf("the sole-workbench probe: %v", err)
	}
	if found != "" {
		t.Errorf("a container holding two workbenches resolved to %q", found)
	}
	sort.Strings(ambiguous)
	if len(ambiguous) != 2 || ambiguous[0] != wide || ambiguous[1] != legacy {
		t.Errorf("the candidates are %v, wanted both %q and %q", ambiguous, wide, legacy)
	}

	if got := describe(legacy); got.ID != "" {
		t.Errorf("the unmigrated workbench carries the identifier %q, and its directory name is not one Dinah minted", got.ID)
	}
	if got := describe(wide); got.ID != filepath.Base(wide) {
		t.Errorf("the migrated workbench reports the identifier %q, wanted its own directory name %q", got.ID, filepath.Base(wide))
	}
	for _, where := range []string{legacy, wide} {
		if _, err := Open(where); err != nil {
			t.Errorf("%s should open by its own path, got %v", where, err)
		}
	}
}

// TestAStrayContainerEntryIsInvisibleToDiscoveryAndFoundByTheSweep asserts
// dinah-285 AC-7. A container subdirectory whose name is neither width carries
// a recognized workbench.md and is not a candidate for any listing in this
// package, exactly as it was not before this card, because the container
// listing filters on the name before an anchor is read. The container sweep
// finds it and names its shape, which is the only thing that ever has.
func TestAStrayContainerEntryIsInvisibleToDiscoveryAndFoundByTheSweep(t *testing.T) {
	tree := t.TempDir()
	base := filepath.Join(tree, UserBaseName)
	stray := plantBench(t, filepath.Join(base, "my-notes"), benchDefinition)

	if listed := ListWorkbenchIDs(base); len(listed) != 0 {
		t.Errorf("the container listing sees %v, and neither width admits my-notes", listed)
	}
	found, ambiguous, err := soleBench(base)
	if err != nil {
		t.Fatalf("the sole-workbench probe: %v", err)
	}
	if found != "" || len(ambiguous) != 0 {
		t.Errorf("the sole-workbench probe reported found=%q ambiguous=%v over a stray entry it cannot see", found, ambiguous)
	}

	candidates, _, err := ScanContainers(tree)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("the sweep found %v, wanted the stray entry alone", candidates)
	}
	if candidates[0].Path != stray || candidates[0].Shape != ShapeStray {
		t.Errorf("the sweep reported %+v, wanted %q as %s", candidates[0], stray, ShapeStray)
	}
}

// TestTheContainerSweepNamesEachShape asserts that the three shapes the
// migration repairs, and the one it leaves alone, are each classified as
// themselves over one tree holding all four.
func TestTheContainerSweepNamesEachShape(t *testing.T) {
	tree := t.TempDir()
	want := map[string]ContainerShape{
		plantBench(t, filepath.Join(tree, "a", UserBaseName, "d00000000001"), benchDefinition): ShapeLegacy,
		plantBench(t, filepath.Join(tree, "b"), benchDefinition):                               ShapeBare,
		plantBench(t, filepath.Join(tree, "c", UserBaseName, "notes"), benchDefinition):        ShapeStray,
		plantBench(t, containedPath(filepath.Join(tree, "d")), currentBenchDefinition):         ShapeContained,
	}
	candidates, _, err := ScanContainers(tree)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(candidates) != len(want) {
		t.Fatalf("the sweep found %d workbenches, wanted %d: %+v", len(candidates), len(want), candidates)
	}
	for _, candidate := range candidates {
		if want[candidate.Path] != candidate.Shape {
			t.Errorf("%s was classified %s, wanted %s", candidate.Path, candidate.Shape, want[candidate.Path])
		}
	}
}
