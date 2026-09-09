package msg

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTheCollectionKeysReachEveryCatalogue enumerates the catalogue directory
// rather than listing the languages, so a ninth catalogue arriving fails this
// test instead of being missed, and it asserts the entry count so a catalogue
// silently skipped reports a number rather than passing.
//
// The de and hi arm asserts that each translation differs from its English and
// carries the fingerprint of the English of the day. Difference is asserted
// rather than presence because a translated entry holding English is the
// failure a fingerprint cannot see, which is what the translation staleness
// contract says of it.
func TestTheCollectionKeysReachEveryCatalogue(t *testing.T) {
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

	files, err := filepath.Glob(filepath.Join("locales", "*.json"))
	if err != nil {
		t.Fatalf("glob the catalogues: %v", err)
	}
	if len(files) != 8 {
		t.Fatalf("the catalogue directory holds %d files and the tool carries eight: %v", len(files), files)
	}
	translated := map[string]bool{"de": true, "hi": true}

	entries := 0
	for _, file := range files {
		tag := file[len(file)-len(filepath.Base(file)):]
		tag = tag[:len(tag)-len(".json")]
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("stat %s: %v", file, err)
		}
		for _, key := range keys {
			base, carried := BaseEntry(key)
			if !carried {
				t.Fatalf("English carries no entry for %s, so nothing below can be compared against it", key)
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
	if entries != 48 {
		t.Fatalf("the sweep read %d entries and six keys across eight catalogues is forty-eight", entries)
	}
}
