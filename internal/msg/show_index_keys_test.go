package msg

import (
	"testing"
)

// TestTheShowIndexKeysAreInTheEnglishCatalogue asserts that English carries
// the keys the show index card writes, each with a text and a context.
//
// The reworded key rides beside the four new ones. Its English changed with
// the default it describes, so a translation still carrying the old sentence
// is a translation of a surface that no longer exists, and the fingerprint
// TestATranslationTracksItsEnglishSource checks is what catches that.
func TestTheShowIndexKeysAreInTheEnglishCatalogue(t *testing.T) {
	keys := []string{
		"column.comments.subject",
		"column.comments.size",
		"param.show.since.summary",
		"param.show.unresolved.summary",
		"show.refilter",
		"param.show.fields.summary",
	}
	if len(keys) != 6 {
		t.Fatalf("the subject set holds %d keys and this card writes six", len(keys))
	}
	assertTheEnglishCarries(t, keys)
}
