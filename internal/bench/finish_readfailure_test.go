package bench

import (
	"path/filepath"
	"testing"
)

// TestTheInterruptionSweepReportsACollectionItCannotRead asserts that the
// sweep behind dinah finish reports a collection it cannot read rather than
// sweeping past it and answering that there is nothing to finish, which is the
// sentence an operator reads while a rename sits half-done.
//
// The succeeding case is the sharper half. It asserts the exact count of
// interruptions rather than merely that some were found, so a build reporting
// an error for every collection and a build reporting no interruptions at all
// both fail.
func TestTheInterruptionSweepReportsACollectionItCannotRead(t *testing.T) {
	// standing writes one genuine sibling into the cards collection: the
	// record an interrupted archive leaves beside the card's directory.
	standing := func(t *testing.T, root string) {
		t.Helper()
		card := filepath.Join(root, CardsDir, "c00000000001")
		write(t, SiblingPath(card), "holder: alka\nop: "+OpArchive+"\n")
	}

	t.Run("every collection readable", func(t *testing.T) {
		root := newFixture(t)
		standing(t, root)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		found, err := opened.interruptions()
		if err != nil {
			t.Fatalf("the sweep answered an error over a readable workbench: %v", err)
		}
		if len(found) != 1 {
			t.Fatalf("the sweep reported %d interruptions over a workbench carrying exactly one", len(found))
		}
	})

	t.Run("one collection that will not read", func(t *testing.T) {
		root := newFixture(t)
		standing(t, root)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		// The plain file goes at the workbench's own attachments collection,
		// which the containment table names beside cards and columns, so the
		// sweep meets a readable collection carrying the sibling and an
		// unreadable one in the same walk.
		plantUnreadable(t, filepath.Join(root, AttachmentsDir))

		found, err := opened.interruptions()
		if err == nil {
			t.Fatalf("the sweep reported %d interruptions and no error while one collection was unreadable", len(found))
		}
	})
}
