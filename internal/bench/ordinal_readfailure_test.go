package bench

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNextNumberReadsTheRegistryRatherThanTheCollections pins where the number
// minter's answer comes from now that the registry replaced the collection
// scan.
//
// The high-water mark is a fact about the file, so a collection nobody can
// list changes nothing, and the mark covers the live half and the archived
// half alike because both halves' claims are lines in one file. Ten rather
// than eight is what proves the archived line was read at all. A filing over
// an unreadable collection is still refused, because internal/verb/beyond.go
// follows NextNumber with bench.ClaimID against the live cards root alone and
// ClaimID's own os.MkdirAll refuses there, which is the door the minter's
// refusal used to hold.
func TestNextNumberReadsTheRegistryRatherThanTheCollections(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ArchiveDir, CardsDir, "c00000000002", CardAnchor), cleanCard)
	write(t, filepath.Join(root, CardNumbersName), "7 c00000000001\n9 c00000000002\n")
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// The collections are planted unreadable after the open, so the registry
	// was read while both halves stood readable and the answer below cannot
	// have come from walking either of them.
	if err := os.RemoveAll(filepath.Join(root, CardsDir)); err != nil {
		t.Fatalf("clear the cards collection: %v", err)
	}
	plantUnreadable(t, filepath.Join(root, CardsDir))
	if err := os.RemoveAll(filepath.Join(root, ArchiveDir, CardsDir)); err != nil {
		t.Fatalf("clear the archived cards collection: %v", err)
	}
	plantUnreadable(t, filepath.Join(root, ArchiveDir, CardsDir))
	number, err := opened.NextNumber()
	if err != nil {
		t.Fatalf("NextNumber over a workbench whose registry reads: %v", err)
	}
	if number != 10 {
		t.Errorf("NextNumber answered %d over a live 7 and an archived 9, wanted 10", number)
	}
}

// TestTheOrdinalMinterRefusesACollectionItCannotRead keeps the ordinal half of
// the minters' read-failure contract. nextOrdinal authorises a write with its
// answer, so a collection it cannot list must refuse rather than answer a
// number an existing member already holds. The number minter's half of that
// contract moved with the registry, which is one file rather than a walk, and
// the readable case runs beside the refusal so a build refusing
// unconditionally fails here.
func TestTheOrdinalMinterRefusesACollectionItCannotRead(t *testing.T) {
	t.Run("a readable collection still answers", func(t *testing.T) {
		root := newFixture(t)
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

	t.Run("a comments collection will not read", func(t *testing.T) {
		root := newFixture(t)
		collection := filepath.Join(root, CardsDir, "c00000000001", CommentsDir)
		plantUnreadable(t, collection)

		ordinal, err := nextOrdinal(collection, CommentAnchor)
		if err == nil {
			t.Fatalf("nextOrdinal answered %d for a collection it could not list", ordinal)
		}
	})
}
