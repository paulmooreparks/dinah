package msg

import (
	"testing"
)

// TestTheCollectionKeysAreInTheEnglishCatalogue asserts that English carries
// every key the collection card mints, each with a text and a context. A
// translation, where a catalogue carries one, is held by the guards in
// msg_test.go, and German and Hindi carrying all of them is checked at
// release.
func TestTheCollectionKeysAreInTheEnglishCatalogue(t *testing.T) {
	keys := []string{
		"refusal.dinah.is-a-collection",
		"refusal.dinah.is-a-collection.next-member",
		"refusal.dinah.is-a-collection.empty",
		"show.collection.empty",
		"contents.header.collection",
		"contents.empty.collection",
	}
	if len(keys) != 6 {
		t.Fatalf("the subject set holds %d keys and this card mints six", len(keys))
	}
	assertTheEnglishCarries(t, keys)
}
