package msg

import (
	"testing"
)

// TestTheRestoreKeysAreInTheEnglishCatalogue is dinah-461 AC-6 as the
// catalogues hold it now: English carries every key the restore card mints,
// each with a text and a context. A translation, where a catalogue carries
// one, is held by the guards in msg_test.go, and German and Hindi carrying all
// of them is checked at release.
func TestTheRestoreKeysAreInTheEnglishCatalogue(t *testing.T) {
	keys := []string{
		"cmd.restore.summary",
		"param.archived.summary",
		"check.restore.1",
		"check.restore.2",
		"check.restore.3",
		"refusal.dinah.not-archived",
		"refusal.dinah.not-archived.restore",
		"refusal.dinah.not-archived.next-holder",
		"refusal.dinah.not-archived.next-workbench",
		"refusal.dinah.not-archived.next-collection",
		"refusal.dinah.not-archived.restore.next",
		"refusal.dinah.not-archived.next",
		"refusal.dinah.exists.restore",
		"refusal.dinah.exists.restore.next",
		"contents.archived",
	}
	if len(keys) < 15 {
		t.Fatalf("the subject set holds %d keys and this card mints fifteen", len(keys))
	}
	assertTheEnglishCarries(t, keys)
}
