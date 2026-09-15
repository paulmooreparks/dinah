package bench

import (
	"path/filepath"
	"strconv"
	"testing"
)

// TestTheWorkbenchRootedSweepVisitsBothHalvesAndSkipsTheUnstamped asserts the
// counting half of dinah-518/criteria/13.
//
// A sweep rooted at the wrong directory finds nothing and reads exactly like
// a clean workbench, so the collections it visits are counted rather than
// inferred from its findings, and the workbench's own half is counted apart
// from the per-column half. A single number would be held above zero by
// either half alone.
//
// The columns and cards collections are required to be absent from the swept
// set. Their anchors carry no ordinal, so a build that swept them would
// report every column and every card as missing one.
func TestTheWorkbenchRootedSweepVisitsBothHalvesAndSkipsTheUnstamped(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ColumnsDir, "b00000000002", ColumnAnchor), columnDefinition)
	// One comment under the first column, so the walk has a comment's own
	// attachments collection to descend into as well.
	writeColumnComment(t, root, "b00000000001", "e00000000001", 1)

	collections, err := ordinalCollections(root, KindWorkbench, map[string]bool{KindCard: true})
	if err != nil {
		t.Fatalf("ordinalCollections: %v", err)
	}

	rootHalf, columnHalf := 0, 0
	columnsRoot := filepath.Join(root, ColumnsDir)
	for _, collection := range collections {
		switch {
		case filepath.Dir(collection.dir) == root:
			rootHalf++
		case isBelow(collection.dir, columnsRoot):
			columnHalf++
		default:
			t.Errorf("the sweep visited %s, which is neither the workbench's own half nor a column's", collection.dir)
		}
		if collection.dir == filepath.Join(root, ColumnsDir) || collection.dir == filepath.Join(root, CardsDir) {
			t.Errorf("the sweep visited %s, whose members carry no ordinal at all", collection.dir)
		}
	}

	// The workbench's own half is its attachments collection and nothing
	// else, because the columns and cards mounts are not stamped.
	if rootHalf != 1 {
		t.Errorf("the sweep visited %d collections at the workbench root, wanted 1: %v", rootHalf, collections)
	}
	// Two columns contribute two collections each, and the one column comment
	// contributes its own attachments collection.
	if columnHalf != 5 {
		t.Errorf("the sweep visited %d collections below the columns, wanted 5: %v", columnHalf, collections)
	}

	// A card-rooted sweep answers exactly what it answered before this card,
	// because every one of a card's mounts is stamped and so is every mount
	// below them.
	cardDir := filepath.Join(root, CardsDir, "c00000000001")
	below, err := ordinalCollections(cardDir, KindCard, nil)
	if err != nil {
		t.Fatalf("the card-rooted sweep: %v", err)
	}
	if len(below) != 3 {
		t.Errorf("the card-rooted sweep visited %d collections, wanted the card's own three: %v", len(below), below)
	}
}

// isBelow reports whether a path sits inside a directory, compared segment by
// segment rather than as a text prefix.
func isBelow(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && len(rel) >= 2 && rel[:2] != ".."
}

// writeColumnComment puts a comment under a column by hand, which is what a
// fixture needs before the verb layer exists to write one.
func writeColumnComment(t *testing.T, root, columnID, commentID string, ordinal int) {
	t.Helper()
	fm := NewFrontmatter()
	fm.Set("ts", "2026-08-17T09:00:00Z")
	fm.Set("author", "alka")
	if ordinal > 0 {
		fm.Set(OrdinalField, strconv.Itoa(ordinal))
	}
	path := filepath.Join(root, ColumnsDir, columnID, CommentsDir, commentID, CommentAnchor)
	write(t, path, fm.Render("a station note\n"))
}
