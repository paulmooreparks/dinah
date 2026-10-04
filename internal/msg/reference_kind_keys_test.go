package msg

import (
	"testing"
)

// TestTheReferenceKindKeysAreInTheEnglishCatalogue asserts that English
// carries every key the reference-kind card mints, each with a text and a
// context. A translation, where a catalogue carries one, is held by the guards
// in msg_test.go, and German and Hindi carrying all of them is checked at
// release.
func TestTheReferenceKindKeysAreInTheEnglishCatalogue(t *testing.T) {
	keys := []string{
		"reference.kind.workbench",
		"reference.kind.workstream",
		"reference.kind.column",
		"reference.kind.card",
		"reference.kind.below-card",
		"reference.kind.collection",
		"help.reference-kinds",
	}
	if len(keys) != 7 {
		t.Fatalf("the subject set holds %d keys and this card mints seven", len(keys))
	}
	assertTheEnglishCarries(t, keys)
}
