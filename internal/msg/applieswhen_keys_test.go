package msg

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// applicabilityKeys is the hand-written list of the fifteen keys dinah-590
// mints, in the order its specification's section 11.6 lists them.
var applicabilityKeys = []string{
	"refusal.dinah.inapplicable-field",
	"refusal.dinah.inapplicable-field.unset",
	"refusal.dinah.inapplicable-field.next",
	"warn.inapplicable-value",
	"card.inapplicable",
	"card.inapplicable.unset",
	"card.inapplicable.stored",
	"card.inapplicable.stored-unset",
	"queue.cell.inapplicable",
	"check.applies-when-malformed",
	"check.applies-when-value-unmatchable",
	"check.inapplicable-value",
	"check.applies-when-below-format",
	"check.required-field-conditioned",
	"check.notices",
}

// domainWords are the words no key of the applicability catalog may carry in
// its English text or its context, as whole words and whatever their case.
// Each names the one domain the operator's two tests exist to keep out of a
// general mechanism.
var domainWords = []string{
	"bug", "defect", "feature", "software", "code", "repository", "branch", "commit", "build", "deploy", "release",
}

// migrationSummaryKey is the one key carrying an applies-when shape that is
// not on the list: the written meaning of the --migrate-applies-when flag,
// which the parameter table mints under the flag's own name and which the
// definition guard requires to exist. The sweep below leaves this one key
// out for that reason and no other, and still holds it to the word sweep, so
// the exclusion buys nothing but the flag's own name. Nothing else under
// param. is left out: a second summary in one of the shapes fails here.
const migrationSummaryKey = "param.check.migrate-applies-when.summary"

// TestTheApplicabilityCatalogSpeaksNoDomain is dinah-590/criteria/12. The
// list above is held against a sweep of the English catalog for the key
// shapes the card mints, so a key minted in one of those shapes and left off
// the list fails here, which a comparison against a written-down number would
// not catch. Every listed key then has to resolve in every locale file, and
// none may speak the domain in its English text or its context.
//
// Arming, twice: planting the word defect into one listed entry's English
// text fails the word sweep, and adding an unlisted key
// card.inapplicable.extra to en.json fails the set comparison.
func TestTheApplicabilityCatalogSpeaksNoDomain(t *testing.T) {
	listed := map[string]bool{}
	for _, key := range applicabilityKeys {
		listed[key] = true
	}
	if len(listed) != 15 {
		t.Fatalf("the list carries %d distinct keys, and the specification mints fifteen", len(listed))
	}

	swept := map[string]bool{}
	sawSummary := false
	for _, key := range Keys() {
		if key == migrationSummaryKey {
			sawSummary = true
			continue
		}
		if strings.Contains(key, "inapplicable") || strings.Contains(key, "applies-when") || key == "check.required-field-conditioned" || key == "check.notices" {
			swept[key] = true
		}
	}
	if !sawSummary {
		t.Errorf("the English catalog carries no %s, so the exclusion below covers nothing", migrationSummaryKey)
	}
	if len(swept) == 0 {
		t.Fatal("the sweep of the English catalog found nothing, so this guard read nothing")
	}
	for key := range swept {
		if !listed[key] {
			t.Errorf("the English catalog carries %s, which has one of the shapes this card mints and is not on the list", key)
		}
	}
	for key := range listed {
		if !swept[key] {
			t.Errorf("the list names %s and the sweep of the English catalog did not find it", key)
		}
	}

	tags := Tags()
	if len(tags) < 2 {
		t.Fatalf("only %d catalogs ship, so the resolution check reads almost nothing", len(tags))
	}
	for _, key := range applicabilityKeys {
		for _, tag := range tags {
			if _, ok := CatalogEntry(tag, key); !ok {
				t.Errorf("the %s catalog does not carry %s", tag, key)
			}
		}
	}

	patterns := make([]*regexp.Regexp, 0, len(domainWords))
	for _, word := range domainWords {
		patterns = append(patterns, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(word)+`\b`))
	}
	checked := 0
	for _, key := range append(append([]string(nil), applicabilityKeys...), migrationSummaryKey) {
		entry, ok := BaseEntry(key)
		if !ok {
			continue
		}
		checked++
		for _, surface := range []struct{ name, text string }{{"text", entry.Text}, {"context", entry.Context}} {
			for i, pattern := range patterns {
				if pattern.MatchString(surface.text) {
					t.Errorf("%s speaks the domain in its %s, carrying %q: %q", key, surface.name, domainWords[i], surface.text)
				}
			}
		}
	}
	if checked != len(applicabilityKeys)+1 {
		t.Errorf("the word sweep read %d entries of %d", checked, len(applicabilityKeys)+1)
	}
	sorted := append([]string(nil), applicabilityKeys...)
	sort.Strings(sorted)
	t.Logf("swept %d keys: %s", len(sorted), strings.Join(sorted, ", "))
}
