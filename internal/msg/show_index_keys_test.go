package msg

import (
	"path/filepath"
	"testing"
)

// TestTheShowIndexKeysReachEveryCatalogue enumerates the catalogue directory
// rather than listing the languages, so a ninth catalogue arriving fails this
// test instead of being missed, and it reports how many files and how many
// entries it read, so a run that walked nothing says so rather than passing.
//
// The reworded key rides beside the four new ones. Its English changed with
// the default it describes, so a translation still carrying the old sentence
// is a translation of a surface that no longer exists, and the fingerprint is
// what catches that.
func TestTheShowIndexKeysReachEveryCatalogue(t *testing.T) {
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
		tag := filepath.Base(file)
		tag = tag[:len(tag)-len(".json")]
		for _, key := range keys {
			base, carried := BaseEntry(key)
			if !carried {
				t.Fatalf("English carries no entry for %s, so nothing below can be compared against it", key)
			}
			if base.Text == "" || base.Context == "" {
				t.Errorf("the base entry for %s carries no text or no context", key)
			}
			entry, held := CatalogEntry(tag, key)
			if !held {
				t.Errorf("%s carries no entry for %s", tag, key)
				continue
			}
			entries++
			if entry.Text == "" || entry.Context == "" {
				t.Errorf("%s/%s carries no text or no context", tag, key)
			}
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
