package bench

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// writeItemComment puts a comment on a checklist item by hand, on the terms
// writeComment already puts one on a card. An ordinal of zero writes no
// ordinal field, standing for a comment written before the field existed or
// torn mid-write.
func writeItemComment(t *testing.T, root, itemID, commentID, ts string, ordinal int, body string) {
	t.Helper()
	fm := NewFrontmatter()
	fm.Set("ts", ts)
	fm.Set("author", "alka")
	if ordinal > 0 {
		fm.Set(OrdinalField, strconv.Itoa(ordinal))
	}
	path := filepath.Join(root, CardsDir, "c00000000001", ChecklistDir, itemID, CommentsDir, commentID, CommentAnchor)
	write(t, path, fm.Render(body+"\n"))
}

// preChangeOrdinalCollections is ordinalCollections exactly as it read before
// dinah-502: a card's own collections, descending only where the mount's kind
// is a comment. It is kept here, local to this test, as the counterfactual
// that arms TestOrdinalCheckAndMigrationReachAnItemsComments: the card's own
// walk still finds a card comment missing its ordinal, and an item's own
// comments collection, which the containment table now mounts, is invisible
// to it.
func preChangeOrdinalCollections(cardDir string) ([]ordinalCollection, error) {
	var collections []ordinalCollection
	for _, mount := range Contains(KindCard) {
		dir := filepath.Join(cardDir, mount.Dir)
		collections = append(collections, ordinalCollection{dir: dir, anchor: mount.Anchor})
		if mount.Kind != KindComment {
			continue
		}
		ids, err := ListIDs(dir)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			for _, below := range Contains(KindComment) {
				collections = append(collections, ordinalCollection{
					dir:    filepath.Join(dir, id, below.Dir),
					anchor: below.Anchor,
				})
			}
		}
	}
	return collections, nil
}

// missingOrdinals counts, over one set of collections, how many entities
// carry no creation ordinal, the same question checkOrdinals asks.
func missingOrdinals(t *testing.T, collections []ordinalCollection) int {
	t.Helper()
	missing := 0
	for _, collection := range collections {
		ids, err := ListIDs(collection.dir)
		if err != nil {
			t.Fatalf("list %s: %v", collection.dir, err)
		}
		for _, id := range ids {
			path := filepath.Join(collection.dir, id, collection.anchor)
			if !Exists(path) {
				continue
			}
			if OrdinalOf(mustLoadAnchor(t, path)) == 0 {
				missing++
			}
		}
	}
	return missing
}

func mustLoadAnchor(t *testing.T, path string) *Frontmatter {
	t.Helper()
	fm, _ := loadAnchor(Disk{}, path)
	return fm
}

// TestOrdinalCheckAndMigrationReachAnItemsComments asserts dinah-502 AC-13.
// A checklist item's comments collection is reached by the ordinal check and
// by the migration exactly as a card's own comments collection is, which the
// recursive ordinalCollections buys and the pre-change, comment-only descent
// did not.
func TestOrdinalCheckAndMigrationReachAnItemsComments(t *testing.T) {
	root := newFixture(t)
	writeItem(t, root, "d00000000001", 1)
	// One comment on the item carries no ordinal, standing for a hand edit or
	// a torn write; the other is stamped, so the migration has exactly one
	// item comment to fill in.
	writeItemComment(t, root, "d00000000001", "e00000000001", "2026-08-17T09:01:00Z", 0, "the reasoning")
	writeItemComment(t, root, "d00000000001", "e00000000002", "2026-08-17T09:02:00Z", 2, "the answer")
	// A card comment carries no ordinal too, so the sweep's find on the card
	// side is not new and the item side is what this test is about.
	writeComment(t, root, "e00000000009", "2026-08-17T09:00:00Z", 0, "a card remark")

	cardDir := filepath.Join(root, CardsDir, "c00000000001")

	// The counterfactual: the walk this card deletes finds the card's own
	// missing ordinal and never descends into the item's comments at all.
	before, err := preChangeOrdinalCollections(cardDir)
	if err != nil {
		t.Fatalf("pre-change walk: %v", err)
	}
	if got := missingOrdinals(t, before); got != 1 {
		t.Fatalf("the pre-change walk found %d entities missing an ordinal, wanted 1 (the card comment alone)", got)
	}
	for _, collection := range before {
		if collection.dir == filepath.Join(cardDir, ChecklistDir, "d00000000001", CommentsDir) {
			t.Fatalf("the pre-change walk reached the item's comments collection, and its premise is that it does not")
		}
	}

	// The walk this card ships reaches both.
	after, err := ordinalCollections(Disk{}, cardDir, KindCard, nil)
	if err != nil {
		t.Fatalf("ordinalCollections: %v", err)
	}
	t.Logf("the recursive walk read %d collections", len(after))
	if got := missingOrdinals(t, after); got != 2 {
		t.Fatalf("the recursive walk found %d entities missing an ordinal, wanted 2 (the card comment and the item comment)", got)
	}

	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := checkOrdinals(Disk{}, cardDir)
	if err != nil {
		t.Fatalf("checkOrdinals: %v", err)
	}
	missing := map[string]bool{}
	for _, finding := range findings {
		if finding.Key == FindingOrdinalMissing {
			missing[finding.Path] = true
		}
	}
	if len(missing) != 2 {
		t.Fatalf("check reported %d ordinal-missing findings, wanted 2: %+v", len(missing), findings)
	}
	itemCommentAnchor := filepath.Join(cardDir, ChecklistDir, "d00000000001", CommentsDir, "e00000000001", CommentAnchor)
	cardCommentAnchor := filepath.Join(cardDir, CommentsDir, "e00000000009", CommentAnchor)
	if !missing[itemCommentAnchor] {
		t.Errorf("check did not name the item comment's anchor %s among %v", itemCommentAnchor, missing)
	}
	if !missing[cardCommentAnchor] {
		t.Errorf("check did not name the card comment's anchor %s among %v", cardCommentAnchor, missing)
	}

	stamped, _, err := opened.BackfillOrdinals("alka", "2026-08-17T10:00:00Z")
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if stamped != 2 {
		t.Fatalf("the migration stamped %d entities, wanted 2", stamped)
	}
	again, err := checkOrdinals(Disk{}, cardDir)
	if err != nil {
		t.Fatalf("checkOrdinals after backfill: %v", err)
	}
	for _, finding := range again {
		if finding.Key == FindingOrdinalMissing {
			t.Errorf("check still reports %s after the migration ran: %+v", FindingOrdinalMissing, finding)
		}
	}
}

// TestCountCommentsAnswersWithoutOpeningAnAnchor asserts dinah-502 Agent Code
// Review round 1's second minor finding: CountComments answers a checklist
// item's comment count from the directory listing alone, the way
// CountAttachments already answers an attachment count, rather than opening
// and parsing every comment's own anchor the way Comments does.
//
// The fixture plants a comment directory carrying no anchor file at all,
// which Comments reads past (an unreadable comment is a defect check
// reports rather than one a read discovers) and which a read-every-anchor
// implementation would therefore undercount by one. CountComments has to
// count it anyway, because it answers from ListIDs and never opens the file
// whose absence is what Comments skips on.
//
// Arming: writing CountComments in terms of len(Comments(...)) instead of
// len(ListIDs(...)) reddens this test, since the comment with no anchor
// would then be silently dropped from the count.
func TestCountCommentsAnswersWithoutOpeningAnAnchor(t *testing.T) {
	root := newFixture(t)
	itemDir := filepath.Join(root, CardsDir, "c00000000001", ChecklistDir, "d00000000001")
	writeItem(t, root, "d00000000001", 1)
	writeItemComment(t, root, "d00000000001", "f00000000001", "2026-08-17T09:01:00Z", 1, "a readable comment")

	// A comment directory with no anchor file: ListIDs still names it,
	// because ListIDs reads the directory rather than the file inside it,
	// and Comments skips it, because ReadText on the missing anchor fails.
	if err := os.MkdirAll(filepath.Join(itemDir, CommentsDir, "f00000000002"), 0o755); err != nil {
		t.Fatalf("mkdir the anchorless comment: %v", err)
	}

	loaded, err := Comments(itemDir)
	if err != nil {
		t.Fatalf("Comments: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("Comments loaded %d, wanted 1 (the anchorless directory is not a comment Comments can read)", len(loaded))
	}

	counted, err := CountComments(itemDir)
	if err != nil {
		t.Fatalf("CountComments: %v", err)
	}
	if counted != 2 {
		t.Errorf("CountComments answered %d, wanted 2 (the directory listing, not the loaded comments)", counted)
	}
}
