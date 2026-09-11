package verb

import (
	"testing"

	"dinah/internal/msg"
)

// TestEveryPullCheckKeyIsCarriedByEveryCatalog is dinah-484 AC-11's second
// half. The card renumbers pull's own rows, and renumbering is a rename of
// every key from the insertion point down, applied by hand in eight files. A
// file left un-renumbered carries a key nothing reads and is missing one
// something does, and the help for that language then prints the key name
// where the sentence should be.
//
// The keys come off pullChecks itself rather than off a list written here, so
// a row added later joins this guard without anybody remembering to.
func TestEveryPullCheckKeyIsCarriedByEveryCatalog(t *testing.T) {
	checked := 0
	for _, row := range Checks(Pull) {
		if _, ok := msg.BaseEntry(row.Key); !ok {
			t.Errorf("%s is a row of pull's list and the base catalog carries no such key", row.Key)
			continue
		}
		for _, tag := range msg.Tags() {
			checked++
			if _, carried := msg.CatalogEntry(tag, row.Key); !carried {
				t.Errorf("%s/%s: the row is declared and this catalog carries no sentence for it", tag, row.Key)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no row was checked against any catalog, so this guard is asserting nothing")
	}
}

// TestEveryMoveCheckKeyIsCarriedByEveryCatalog is the same guard over the
// move's own list, which this card appends a row to. An appended row cannot
// renumber anything, so the failure it catches is narrower: a row declared in
// code and left out of the catalogs entirely.
func TestEveryMoveCheckKeyIsCarriedByEveryCatalog(t *testing.T) {
	checked := 0
	for _, row := range Checks(Move) {
		if _, ok := msg.BaseEntry(row.Key); !ok {
			t.Errorf("%s is a row of the move's list and the base catalog carries no such key", row.Key)
			continue
		}
		for _, tag := range msg.Tags() {
			checked++
			if _, carried := msg.CatalogEntry(tag, row.Key); !carried {
				t.Errorf("%s/%s: the row is declared and this catalog carries no sentence for it", tag, row.Key)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no row was checked against any catalog, so this guard is asserting nothing")
	}
}
