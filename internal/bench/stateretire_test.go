package bench

import (
	"path/filepath"
	"strings"
	"testing"
)

// newStrandedFixture writes a workbench declaring two states, one backed by
// a real directory and one whose directory was never written at all: the
// shape retirement produces when its definition write is skipped, and the
// shape a hand-stranded id leaves behind.
func newStrandedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	fm := NewFrontmatter()
	fm.Set("format", "1")
	fm.Set("profile", "dinah-core/1.0")
	fm.Set("title", "Fixture")
	fm.Set("slug", "fx")
	fm.Set("operator", "alka")
	fm.SetSeq("states", []string{"b00000000001", "b00000000002"})
	write(t, filepath.Join(root, WorkbenchAnchor), fm.Render("Standing text.\n"))
	write(t, filepath.Join(root, StatesDir, "b00000000001", StateAnchor), stateAnchor("One", "one", "intake"))
	// b00000000002 is named in the definition; its directory is never written.
	return root
}

// newSoleStrandedFixture writes a workbench declaring one state, its
// identifier the definition's only member, whose directory does not exist:
// the shape driving a workbench down to its last state and archiving it left
// behind before this card's fix.
func newSoleStrandedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	fm := NewFrontmatter()
	fm.Set("format", "1")
	fm.Set("profile", "dinah-core/1.0")
	fm.Set("title", "Fixture")
	fm.Set("slug", "fx")
	fm.Set("operator", "alka")
	fm.SetSeq("states", []string{"b00000000001"})
	write(t, filepath.Join(root, WorkbenchAnchor), fm.Render("Standing text.\n"))
	return root
}

// TestReadStateStillRefusesAMissingDirectoryDirectly reproduces today's
// defect at the boundary Open used to hand it to: called directly against a
// state whose directory is not there, readState still raises the
// id-specific malformed refusal, never the generic one that only appears
// when states: itself is absent or empty. Open no longer reaches readState
// for this shape at all (see the next test), which is the fix; this test
// pins down what the defect looked like before Open stopped calling it here.
func TestReadStateStillRefusesAMissingDirectoryDirectly(t *testing.T) {
	root := newStrandedFixture(t)
	_, err := readState(root, "b00000000002", 0)
	if !refusedMalformed(err) {
		t.Fatalf("wanted the malformed refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), "b00000000002") {
		t.Errorf("wanted the id-specific message, got %v", err)
	}
}

// TestOpenToleratesAStateDirectoryThatIsNotThere asserts AC-4: a workbench
// naming a state id whose directory does not exist opens successfully
// instead of refusing the whole bench, and the stranded id is left off
// Bench.States. The same holds whether the stranded id is one of several
// states or the workbench's only one.
func TestOpenToleratesAStateDirectoryThatIsNotThere(t *testing.T) {
	opened, err := Open(newStrandedFixture(t))
	if err != nil {
		t.Fatalf("a workbench naming a stranded state should open: %v", err)
	}
	if len(opened.States) != 1 || opened.States[0].ID != "b00000000001" {
		t.Fatalf("wanted the one live state, got %+v", opened.States)
	}
	if len(opened.StrandedStates) != 1 || opened.StrandedStates[0] != "b00000000002" {
		t.Fatalf("wanted the stranded id recorded, got %v", opened.StrandedStates)
	}

	sole, err := Open(newSoleStrandedFixture(t))
	if err != nil {
		t.Fatalf("a workbench whose sole state is stranded should open: %v", err)
	}
	if len(sole.States) != 0 {
		t.Fatalf("wanted no live states, got %+v", sole.States)
	}
	if len(sole.StrandedStates) != 1 || sole.StrandedStates[0] != "b00000000001" {
		t.Fatalf("wanted the sole stranded id recorded, got %v", sole.StrandedStates)
	}
}

// TestCheckReportsStrandedStatesAndTheMigrationRepairsThem asserts AC-5 and
// AC-6: check reports one check.stranded-state finding per stranded id,
// naming the workbench anchor as its path, dinah check --migrate-states
// removes every stranded id from workbench.md's states list in one run and
// reports what it removed, and a second run over the already-repaired
// workbench reports nothing and removes nothing. The same shape and the
// same repair hold whether the stranded id was the workbench's sole state or
// one of several, with no hand edit to workbench.md at any point.
func TestCheckReportsStrandedStatesAndTheMigrationRepairsThem(t *testing.T) {
	cases := []struct {
		name string
		root string
		gone string
	}{
		{"one of several", newStrandedFixture(t), "b00000000002"},
		{"the sole state", newSoleStrandedFixture(t), "b00000000001"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			anchor := filepath.Join(c.root, WorkbenchAnchor)
			opened, err := Open(c.root)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			findings, err := opened.Check()
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("wanted one finding, got %+v", findings)
			}
			if findings[0].Key != FindingStrandedState || findings[0].Detail != c.gone || findings[0].Path != anchor {
				t.Fatalf("wanted a stranded-state finding naming %s at %s, got %+v", c.gone, anchor, findings[0])
			}

			removed, err := opened.RemoveStrandedStates()
			if err != nil {
				t.Fatalf("migrate: %v", err)
			}
			if len(removed) != 1 || removed[0] != c.gone {
				t.Fatalf("wanted %s reported removed, got %v", c.gone, removed)
			}
			if len(opened.StrandedStates) != 0 {
				t.Errorf("the migration left %v stranded on the open workbench", opened.StrandedStates)
			}

			// A second run over the same already-repaired bench finds and
			// removes nothing, whatever the anchor now declares.
			removedAgain, err := opened.RemoveStrandedStates()
			if err != nil {
				t.Fatalf("second migrate: %v", err)
			}
			if len(removedAgain) != 0 {
				t.Errorf("a second run should remove nothing, got %v", removedAgain)
			}
		})
	}
}
