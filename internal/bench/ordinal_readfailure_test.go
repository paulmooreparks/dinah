package bench

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTheNumberAndOrdinalMintersRefuseACollectionTheyCannotRead covers the two
// sites where a wrong answer authorises a write.
//
// The archived case is the one that matters. internal/verb/beyond.go follows
// NextNumber with bench.ClaimID against the live cards root alone, so an
// unreadable live collection aborts the filing on ClaimID's own os.MkdirAll
// while an unreadable archive aborts nothing: NextNumber would silently omit
// every archived number from its maximum and the new card would be stamped
// with a number an archived card already carries.
//
// The succeeding cases run in the same test, so a build answering an error
// unconditionally fails here.
func TestTheNumberAndOrdinalMintersRefuseACollectionTheyCannotRead(t *testing.T) {
	t.Run("the live cards collection will not read", func(t *testing.T) {
		root := newFixture(t)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if err := os.RemoveAll(filepath.Join(root, CardsDir)); err != nil {
			t.Fatalf("clear the cards collection: %v", err)
		}
		plantUnreadable(t, filepath.Join(root, CardsDir))

		number, err := opened.NextNumber()
		if err == nil {
			t.Fatalf("NextNumber answered %d for a live collection it could not list", number)
		}
	})

	t.Run("the archived cards collection will not read", func(t *testing.T) {
		root := newFixture(t)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		// The live half is left readable, which is what makes this the route
		// that reaches a write: nothing downstream of NextNumber touches the
		// archive, so nothing else would refuse.
		plantUnreadable(t, filepath.Join(root, ArchiveDir, CardsDir))

		number, err := opened.NextNumber()
		if err == nil {
			t.Fatalf("NextNumber answered %d for an archived collection it could not list", number)
		}
	})

	t.Run("a comments collection will not read", func(t *testing.T) {
		root := newFixture(t)
		collection := filepath.Join(root, CardsDir, "c00000000001", CommentsDir)
		plantUnreadable(t, collection)

		ordinal, err := nextOrdinal(collection, CommentAnchor)
		if err == nil {
			t.Fatalf("nextOrdinal answered %d for a collection it could not list", ordinal)
		}
	})

	t.Run("both minters still answer over readable collections", func(t *testing.T) {
		root := newFixture(t)
		write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), numberedCard("7"))
		write(t, filepath.Join(root, ArchiveDir, CardsDir, "c00000000002", CardAnchor), numberedCard("9"))
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		// Ten rather than eight also proves NextNumber still reads the
		// archived half at all.
		number, err := opened.NextNumber()
		if err != nil {
			t.Fatalf("NextNumber over readable collections: %v", err)
		}
		if number != 10 {
			t.Errorf("NextNumber answered %d over a live 7 and an archived 9, wanted 10", number)
		}

		collection := filepath.Join(root, CardsDir, "c00000000001", CommentsDir)
		write(t, filepath.Join(collection, "e00000000001", CommentAnchor), "---\nauthor: alka\nordinal: 1\n---\nFirst.\n")
		write(t, filepath.Join(collection, "e00000000002", CommentAnchor), "---\nauthor: alka\nordinal: 2\n---\nSecond.\n")
		ordinal, err := nextOrdinal(collection, CommentAnchor)
		if err != nil {
			t.Fatalf("nextOrdinal over a readable collection: %v", err)
		}
		if ordinal != 3 {
			t.Errorf("nextOrdinal answered %d over a collection holding two stamped members, wanted 3", ordinal)
		}
	})
}
