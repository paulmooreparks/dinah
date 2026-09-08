package msg

import (
	"testing"
)

// TestTheAddressHeadingsShipInEveryCatalogue asserts dinah-454 AC-10: the
// three headings this card lands reach all eight catalogues in the shape the
// translation staleness contract requires, and the two keys it retires are
// gone from every one of them.
//
// The retirement arm is the one worth spelling out. A rename that adds the new
// key and leaves the old one behind passes every other guard in this package,
// because the catalogues stay in step with each other and the counts still
// match; only a check that names the retired key can see it.
func TestTheAddressHeadingsShipInEveryCatalogue(t *testing.T) {
	added := map[string]string{
		"column.comments.ref":          "Ref",
		"column.attachments.ref":       "Ref",
		"column.workstreams.reference": "Reference",
	}
	translated := map[string]bool{"de": true, "hi": true}

	for key, english := range added {
		for _, tag := range Tags() {
			catalog, shipped := loaded[tag]
			if !shipped {
				t.Errorf("%s ships no catalog at all", tag)
				continue
			}
			entry, carried := catalog.Entries[key]
			if !carried {
				t.Errorf("%s carries no %s, so one head draws a heading the catalog cannot answer", tag, key)
				continue
			}
			switch {
			case tag == Base:
				if entry.Text != english {
					t.Errorf("en/%s reads %q, wanted %q", key, entry.Text, english)
				}
				if entry.Context == "" {
					t.Errorf("en/%s carries no context, so a translator is told nothing about what the heading sits over", key)
				}
			case translated[tag]:
				if entry.Skeleton {
					t.Errorf("%s/%s ships as a skeleton, and that language ships complete", tag, key)
				}
				if want := Fingerprint(english); entry.Source != want {
					t.Errorf("%s/%s records the source %q, wanted %q, which is the fingerprint of the English it was read against", tag, key, entry.Source, want)
				}
			default:
				if entry.Text != english {
					t.Errorf("%s/%s reads %q, and a skeleton entry carries the English text unchanged", tag, key, entry.Text)
				}
				if !entry.Skeleton {
					t.Errorf("%s/%s is not marked as a skeleton, so the coverage count reads it as translated", tag, key)
				}
				if entry.Source != "" {
					t.Errorf("%s/%s records the source %q, and a skeleton was read against nothing", tag, key, entry.Source)
				}
			}
		}
	}

	for _, retired := range []string{"column.attachments.position", "column.workstreams.slug"} {
		for _, tag := range Tags() {
			catalog, shipped := loaded[tag]
			if !shipped {
				continue
			}
			if _, carried := catalog.Entries[retired]; carried {
				t.Errorf("%s still carries %s, which nothing draws once the column it headed is gone", tag, retired)
			}
		}
	}
}
