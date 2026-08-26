package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// newStrandedFixture writes a workbench declaring two columns, one backed by
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
	fm.SetSeq("columns", []string{"b00000000001", "b00000000002"})
	write(t, filepath.Join(root, WorkbenchAnchor), fm.Render("Standing text.\n"))
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnAnchor("One", "one", "intake"))
	// b00000000002 is named in the definition; its directory is never written.
	return root
}

// newSoleStrandedFixture writes a workbench declaring one column, its
// identifier the definition's only member, whose directory does not exist:
// the shape driving a workbench down to its last column and archiving it left
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
	fm.SetSeq("columns", []string{"b00000000001"})
	write(t, filepath.Join(root, WorkbenchAnchor), fm.Render("Standing text.\n"))
	return root
}

// TestReadColumnStillRefusesAMissingDirectoryDirectly reproduces today's
// defect at the boundary Open used to hand it to: called directly against a
// column whose directory is not there, readColumn still raises the
// id-specific malformed refusal, never the generic one that only appears
// when columns: itself is absent or empty. Open no longer reaches readColumn
// for this shape at all (see the next test), which is the fix; this test
// pins down what the defect looked like before Open stopped calling it here.
func TestReadColumnStillRefusesAMissingDirectoryDirectly(t *testing.T) {
	root := newStrandedFixture(t)
	_, err := readColumn(root, "b00000000002", 0)
	if !refusedMalformed(err) {
		t.Fatalf("wanted the malformed refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), "b00000000002") {
		t.Errorf("wanted the id-specific message, got %v", err)
	}
}

// TestOpenToleratesAColumnDirectoryThatIsNotThere asserts AC-4: a workbench
// naming a column id whose directory does not exist opens successfully
// instead of refusing the whole bench, and the stranded id is left off
// Bench.Columns. The same holds whether the stranded id is one of several
// columns or the workbench's only one.
func TestOpenToleratesAColumnDirectoryThatIsNotThere(t *testing.T) {
	opened, err := Open(newStrandedFixture(t))
	if err != nil {
		t.Fatalf("a workbench naming a stranded column should open: %v", err)
	}
	if len(opened.Columns) != 1 || opened.Columns[0].ID != "b00000000001" {
		t.Fatalf("wanted the one live column, got %+v", opened.Columns)
	}
	if len(opened.StrandedColumns) != 1 || opened.StrandedColumns[0] != "b00000000002" {
		t.Fatalf("wanted the stranded id recorded, got %v", opened.StrandedColumns)
	}

	sole, err := Open(newSoleStrandedFixture(t))
	if err != nil {
		t.Fatalf("a workbench whose sole column is stranded should open: %v", err)
	}
	if len(sole.Columns) != 0 {
		t.Fatalf("wanted no live columns, got %+v", sole.Columns)
	}
	if len(sole.StrandedColumns) != 1 || sole.StrandedColumns[0] != "b00000000001" {
		t.Fatalf("wanted the sole stranded id recorded, got %v", sole.StrandedColumns)
	}
}

// TestCheckReportsStrandedColumnsAndTheMigrationRepairsThem asserts AC-5 and
// AC-6: check reports one check.stranded-column finding per stranded id,
// naming the workbench anchor as its path, and dinah check --migrate-columns
// removes every stranded id from workbench.md's columns list in one run and
// reports what it removed, when at least one non-stranded column remains, with
// a second run over the already-repaired workbench reporting nothing and
// removing nothing. When the stranded id is the workbench's only column,
// removing it would leave the columns list with none at all, so the migration
// refuses instead (AC-1), asserted by the "the sole column" subtest below.
func TestCheckReportsStrandedColumnsAndTheMigrationRepairsThem(t *testing.T) {
	t.Run("one of several", func(t *testing.T) {
		root := newStrandedFixture(t)
		gone := "b00000000002"
		anchor := filepath.Join(root, WorkbenchAnchor)
		opened, err := Open(root)
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
		if findings[0].Key != FindingStrandedColumn || findings[0].Detail != gone || findings[0].Path != anchor {
			t.Fatalf("wanted a stranded-column finding naming %s at %s, got %+v", gone, anchor, findings[0])
		}

		removed, err := opened.RemoveStrandedColumns()
		if err != nil {
			t.Fatalf("migrate: %v", err)
		}
		if len(removed) != 1 || removed[0] != gone {
			t.Fatalf("wanted %s reported removed, got %v", gone, removed)
		}
		if len(opened.StrandedColumns) != 0 {
			t.Errorf("the migration left %v stranded on the open workbench", opened.StrandedColumns)
		}

		// A second run over the same already-repaired bench finds and
		// removes nothing, whatever the anchor now declares.
		removedAgain, err := opened.RemoveStrandedColumns()
		if err != nil {
			t.Fatalf("second migrate: %v", err)
		}
		if len(removedAgain) != 0 {
			t.Errorf("a second run should remove nothing, got %v", removedAgain)
		}
	})

	// "the sole column" asserts AC-1: removing the workbench's only remaining
	// (and stranded) column would leave the columns list with none at all, so
	// the migration refuses with contract.RepairWouldEmptyColumns instead of
	// writing, and leaves workbench.md and StrandedColumns exactly as they
	// were, so a following check still reports the same finding.
	t.Run("the sole column", func(t *testing.T) {
		root := newSoleStrandedFixture(t)
		gone := "b00000000001"
		anchor := filepath.Join(root, WorkbenchAnchor)
		opened, err := Open(root)
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
		if findings[0].Key != FindingStrandedColumn || findings[0].Detail != gone || findings[0].Path != anchor {
			t.Fatalf("wanted a stranded-column finding naming %s at %s, got %+v", gone, anchor, findings[0])
		}

		before, err := os.ReadFile(anchor)
		if err != nil {
			t.Fatalf("read anchor: %v", err)
		}

		removed, err := opened.RemoveStrandedColumns()
		if removed != nil {
			t.Errorf("wanted nothing removed on refusal, got %v", removed)
		}
		refusal, ok := err.(*contract.Refusal)
		if !ok || refusal.Name != contract.RepairWouldEmptyColumns {
			t.Fatalf("wanted the repair-would-empty-columns refusal, got %v", err)
		}
		if !strings.Contains(refusal.Detail, gone) {
			t.Errorf("wanted the refusal to name %s, got detail %q", gone, refusal.Detail)
		}
		if len(opened.StrandedColumns) != 1 || opened.StrandedColumns[0] != gone {
			t.Errorf("wanted the stranded id still reported on the open workbench, got %v", opened.StrandedColumns)
		}

		after, err := os.ReadFile(anchor)
		if err != nil {
			t.Fatalf("read anchor after refusal: %v", err)
		}
		if string(before) != string(after) {
			t.Errorf("wanted workbench.md unchanged by the refused migration")
		}

		reopened, err := Open(root)
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
		findingsAgain, err := reopened.Check()
		if err != nil {
			t.Fatalf("check again: %v", err)
		}
		if len(findingsAgain) != 1 || findingsAgain[0].Key != FindingStrandedColumn || findingsAgain[0].Detail != gone {
			t.Fatalf("wanted the same finding reported again after the refusal, got %+v", findingsAgain)
		}
	})
}
