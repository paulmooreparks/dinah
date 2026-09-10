package msg

import (
	"path/filepath"
	"testing"
)

// TestTheRestoreKeysAreCarriedInEveryCatalogue is dinah-461 AC-6.
//
// The catalogue directory is enumerated rather than listed, so a ninth
// catalogue arriving fails this test instead of being missed, and both the key
// slice and the enumeration carry a floor, because a sweep whose subject set
// can be empty passes for free.
//
// Every comparison is on the JSON values rather than on rendered output, so
// wrapping and terminal width cannot reach it. The de and hi arm asserts that
// each translation differs from its English as well as carrying the
// fingerprint of the English of the day, because a translated entry holding
// English is the failure a fingerprint cannot see.
func TestTheRestoreKeysAreCarriedInEveryCatalogue(t *testing.T) {
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

	files, err := filepath.Glob(filepath.Join("locales", "*.json"))
	if err != nil {
		t.Fatalf("glob the catalogues: %v", err)
	}
	if len(files) < 8 {
		t.Fatalf("the catalogue directory holds %d files and the tool carries eight: %v", len(files), files)
	}
	translated := map[string]bool{"de": true, "hi": true}

	entries := 0
	for _, file := range files {
		tag := filepath.Base(file)
		tag = tag[:len(tag)-len(".json")]
		for _, key := range keys {
			base, carried := BaseEntry(key)
			if !carried {
				t.Fatalf("English carries no entry for %s, so nothing below can be compared against it", key)
			}
			if base.Text == "" || base.Context == "" {
				t.Errorf("the base entry for %s carries an empty text or an empty context", key)
			}
			entry, held := CatalogEntry(tag, key)
			if !held {
				t.Errorf("%s carries no entry for %s", tag, key)
				continue
			}
			entries++
			switch {
			case tag == Base:
				if entry.Source != "" || entry.Skeleton {
					t.Errorf("the base entry for %s carries a source or a skeleton mark, and it is a translation of nothing", key)
				}
			case translated[tag]:
				if entry.Text == base.Text {
					t.Errorf("%s carries the English text for %s under its own tag", tag, key)
				}
				if entry.Skeleton {
					t.Errorf("%s marks %s a skeleton and it is translated", tag, key)
				}
				if want := Fingerprint(base.Text); entry.Source != want {
					t.Errorf("%s records source %q for %s and the English of the day fingerprints to %q", tag, entry.Source, key, want)
				}
			default:
				if !entry.Skeleton {
					t.Errorf("%s carries %s without the skeleton mark", tag, key)
				}
				if entry.Source != "" {
					t.Errorf("%s records a source for the skeleton entry %s, and a skeleton is a translation of nothing", tag, key)
				}
				if entry.Text != base.Text {
					t.Errorf("%s's skeleton for %s does not carry the English text", tag, key)
				}
			}
		}
	}
	t.Logf("%d catalogue files enumerated, %d entries read", len(files), entries)
	if want := len(files) * len(keys); entries != want {
		t.Fatalf("the sweep read %d entries and %d keys across %d catalogues is %d", entries, len(keys), len(files), want)
	}
}
