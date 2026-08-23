package msg

import (
	"regexp"
	"strings"
	"testing"
)

// TestEveryDeclaredLanguageShips asserts that the catalogs the language ruling
// calls for are all present: English, Hindi and German complete, and the
// format's five remaining declared languages as generated skeletons carrying
// every key. The roster itself lives once, as Complete and Skeleton, so this
// test and internal/verb's TestVersionCarriesTheConformanceClaim read the
// same declaration rather than each carrying its own copy.
func TestEveryDeclaredLanguageShips(t *testing.T) {
	complete := map[string]bool{}
	for _, tag := range Complete {
		complete[tag] = true
	}
	skeletons := map[string]bool{}
	for _, tag := range Skeleton {
		skeletons[tag] = true
	}
	seen := map[string]bool{}
	for _, tag := range Tags() {
		seen[tag] = true
		translated, present, total := Coverage(tag)
		if present != total {
			t.Errorf("%s: wanted every key present, got %d of %d", tag, present, total)
		}
		if complete[tag] && translated != total {
			t.Errorf("%s ships complete, got %d of %d translated", tag, translated, total)
		}
		if skeletons[tag] && translated != 0 {
			t.Errorf("%s ships as a skeleton, got %d translated", tag, translated)
		}
	}
	for tag := range complete {
		if !seen[tag] {
			t.Errorf("the complete catalog %s does not ship", tag)
		}
	}
	for tag := range skeletons {
		if !seen[tag] {
			t.Errorf("the skeleton catalog %s does not ship", tag)
		}
	}
	if Tags()[0] != Base {
		t.Errorf("the base language should lead the list, got %s", Tags()[0])
	}
}

// TestEveryKeyCarriesAContext asserts that a translator gets told what each
// message is for, which a generated skeleton carries as well.
func TestEveryKeyCarriesAContext(t *testing.T) {
	keys := Keys()
	if len(keys) == 0 {
		t.Fatal("the base catalog carries no keys")
	}
	for _, key := range keys {
		entry, ok := BaseEntry(key)
		if !ok {
			t.Fatalf("%s: the base catalog reports it and does not carry it", key)
		}
		if entry.Text == "" {
			t.Errorf("%s carries no text", key)
		}
		if entry.Context == "" {
			t.Errorf("%s carries no context for a translator", key)
		}
	}
}

// TestPlaceholdersAreNamed asserts that a message's parameters are named
// rather than positional, so a translator may reorder a sentence around them.
func TestPlaceholdersAreNamed(t *testing.T) {
	for _, key := range Keys() {
		entry, _ := BaseEntry(key)
		if strings.Contains(entry.Text, "%s") || strings.Contains(entry.Text, "%d") {
			t.Errorf("%s carries a positional placeholder: %q", key, entry.Text)
		}
	}
	rendered := For(Base).T("refusal.at-capacity", "detail", "a00000000002")
	if !strings.Contains(rendered, "a00000000002") {
		t.Errorf("a named placeholder did not fill: %q", rendered)
	}
	if strings.Contains(rendered, "{detail}") {
		t.Errorf("the placeholder survived the fill: %q", rendered)
	}
}

// TestMissingKeysFallBackPerKey asserts that an incomplete catalog degrades
// one string at a time rather than failing the command, which is what the
// language ruling asks of a skeleton.
func TestMissingKeysFallBackPerKey(t *testing.T) {
	hindi := For("hi")
	if got := hindi.T("word.yes"); got == For(Base).T("word.yes") {
		t.Error("a translated key should not render as English")
	}
	czech := For("cs")
	if got := czech.T("word.yes"); got != For(Base).T("word.yes") {
		t.Errorf("a skeleton key should fall back to English, got %q", got)
	}
	unknown := For("qq")
	if got := unknown.T("word.yes"); got != For(Base).T("word.yes") {
		t.Errorf("a language no catalog answers to should render in English, got %q", got)
	}
	if got := For(Base).T("no.such.key"); got != "{no.such.key}" {
		t.Errorf("a key no catalog carries should be visible, got %q", got)
	}
}

// TestTheProductNameStaysLatinInEveryLocale asserts that a translated message
// naming the product carries the Latin spelling Dinah rather than a
// transliteration into the target script. A key untranslated in a given
// catalog falls back to the English text, which already carries the name, so
// this only fails where a translated string dropped or respelled it.
func TestTheProductNameStaysLatinInEveryLocale(t *testing.T) {
	for _, key := range Keys() {
		entry, ok := BaseEntry(key)
		if !ok || !strings.Contains(entry.Text, "Dinah") {
			continue
		}
		for _, tag := range Tags() {
			rendered := For(tag).T(key)
			if !strings.Contains(rendered, "Dinah") {
				t.Errorf("%s/%s: wanted the Latin spelling Dinah, got %q", tag, key, rendered)
			}
		}
	}
}

// TestRegionalTagsWalkTheHierarchy asserts the BCP 47 lookup: a regional tag
// falls back to its base language rather than to English.
func TestRegionalTagsWalkTheHierarchy(t *testing.T) {
	regional := For("hi-IN")
	if regional.T("word.yes") != For("hi").T("word.yes") {
		t.Error("hi-IN should render through the hi catalog")
	}
}

// TestPluralsFollowTheCategories asserts that a count chooses its message by
// CLDR category, with a language carrying only other getting it for every
// count.
func TestPluralsFollowTheCategories(t *testing.T) {
	one := For(Base).TN("check.count", 1)
	many := For(Base).TN("check.count", 4)
	if one == many {
		t.Errorf("wanted one form per category, got %q for both", one)
	}
	if !strings.Contains(one, "1") || !strings.Contains(many, "4") {
		t.Errorf("the count did not fill: %q and %q", one, many)
	}
}

// TestATranslationKeepsThePlaceholdersAndTheSplice asserts that a translated
// string carries every placeholder its English source names, and that a
// next-step clause still opens with the separator that splices it onto the
// refusal it follows. A translator may move a placeholder within the sentence
// and may not drop or respell one, and a next-step clause that loses its
// leading separator runs into the sentence before it.
func TestATranslationKeepsThePlaceholdersAndTheSplice(t *testing.T) {
	placeholder := regexp.MustCompile(`\{[a-zA-Z][a-zA-Z0-9_.-]*\}`)
	for _, key := range Keys() {
		entry, ok := BaseEntry(key)
		if !ok {
			continue
		}
		names := placeholder.FindAllString(entry.Text, -1)
		splice := strings.HasSuffix(key, ".next") && strings.HasPrefix(entry.Text, "; ")
		if len(names) == 0 && !splice {
			continue
		}
		for _, tag := range Complete {
			rendered := For(tag).T(key)
			for _, name := range names {
				if !strings.Contains(rendered, name) {
					t.Errorf("%s/%s: wanted the placeholder %s, got %q", tag, key, name, rendered)
				}
			}
			if splice && !strings.HasPrefix(rendered, "; ") {
				t.Errorf("%s/%s: wanted the leading separator that splices it onto the refusal, got %q", tag, key, rendered)
			}
		}
	}
}
