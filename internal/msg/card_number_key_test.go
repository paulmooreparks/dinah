package msg

import (
	"testing"
)

// TestTheCardNumberKeysAreInTheEnglishCatalogue holds the family of finding
// keys this card mints to what every other key on this project is held to:
// English carries each with a text and a context. A translation, where a
// catalogue carries one, is held by the guards in msg_test.go, and German and
// Hindi carrying all seven is checked at release.
func TestTheCardNumberKeysAreInTheEnglishCatalogue(t *testing.T) {
	keys := []string{
		"check.card-number-duplicate",
		"check.card-number-repeated",
		"check.card-number-missing",
		"check.card-number-stranded",
		"check.card-number-malformed",
		"check.card-number-in-frontmatter",
		"check.card-number-renumbered",
	}
	if len(keys) != 7 {
		t.Fatalf("the subject set holds %d keys and this card mints seven", len(keys))
	}
	assertTheEnglishCarries(t, keys)
}
