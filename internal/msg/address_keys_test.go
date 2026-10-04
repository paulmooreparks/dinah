package msg

import (
	"testing"
)

// TestTheAddressHeadingsAreInTheEnglishCatalogue asserts dinah-454 AC-10 as
// the catalogues hold it now: the three headings this card lands carry the
// English text it specifies, and the two keys it retires are gone from every
// catalogue. A translation of a heading, where a catalogue carries one, is
// held by the guards in msg_test.go that read every translation.
//
// The retirement arm is the one worth spelling out. A rename that adds the new
// key and leaves the old one behind passes every other per-key guard in this
// package; only a check that names the retired key, or
// TestATranslationCarriesNoKeyEnglishLacks for a catalogue other than English,
// can see it.
func TestTheAddressHeadingsAreInTheEnglishCatalogue(t *testing.T) {
	added := map[string]string{
		"column.comments.ref":          "Ref",
		"column.attachments.ref":       "Ref",
		"column.workstreams.reference": "Reference",
	}
	keys := make([]string, 0, len(added))
	for key, english := range added {
		keys = append(keys, key)
		if entry, ok := BaseEntry(key); ok && entry.Text != english {
			t.Errorf("en/%s reads %q, wanted %q", key, entry.Text, english)
		}
	}
	assertTheEnglishCarries(t, keys)

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
