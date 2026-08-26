package msg

import (
	"regexp"
	"sort"
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
// leading separator runs into the sentence before it. The leading separator in
// the English source is what selects a spliced clause here, because the
// catalog names these clauses several ways and a check keyed on the ".next"
// spelling would miss the seven that are named something else.
//
// Both halves take the English entry as the whole expectation, and they read
// only its placeholders and its leading separator. Anything else a translation
// should carry goes unchecked here, whether it is content the English lists
// and the translation drops or content the English never named. German's
// help.environment omitted DINAH_MCP_ROOT, which the English lists, and this
// test said nothing about it; dinah-248 fixed the omission and added
// TestEveryUntranslatableIdentifierSurvivesTranslation, which reads the
// content of every entry declaring it untranslatable. A reader chasing a
// missing-content defect elsewhere in the catalog should still not expect this
// guard to have caught it.
func TestATranslationKeepsThePlaceholdersAndTheSplice(t *testing.T) {
	placeholder := regexp.MustCompile(`\{[a-zA-Z][a-zA-Z0-9_.-]*\}`)
	for _, key := range Keys() {
		entry, ok := BaseEntry(key)
		if !ok {
			continue
		}
		names := placeholder.FindAllString(entry.Text, -1)
		splice := strings.HasPrefix(entry.Text, "; ")
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

// TestATranslationTracksItsEnglishSource asserts that every translated entry
// records the English it was translated from, and that the English has not
// moved since. The entry stores a fingerprint of the base text under source,
// this test recomputes that fingerprint from the base catalog of the day, and
// the two disagree exactly when somebody edited English and left a translation
// behind. That is the failure no other guard in this file can see:
// TestATranslationKeepsThePlaceholdersAndTheSplice reads only the
// placeholders and the leading separator, and
// TestEveryUntranslatableIdentifierSurvivesTranslation reads only the entries
// that declare part of their content untranslatable, so an English sentence
// that gains, loses or rewords an ordinary clause passes both while the
// translation goes on saying the older thing.
//
// A subtest per language keeps a failure readable. Without one, a single
// English edit reports the same key once per catalog and a reader cannot tell
// how many languages are actually behind. Every catalog that ships earns a
// subtest, so a skeleton catalog joins the guard the moment somebody
// translates one of its entries and nothing here has to be edited for that to
// happen.
//
// The loop reads every shipped catalog rather than the Complete roster it once
// read, because the roster answers a different question. Complete is
// catalog-level: it says a language has no untranslated entry left, and
// dinah-287 took Hindi and German off it while both still carry hundreds of
// real translations. A guard keyed on that roster stopped checking those
// translations on the day the roster changed, and reported that it was
// asserting nothing rather than checking what was in front of it.
//
// What the change costs is worth stating in the numbers, because the first
// version of this comment claimed a strictly larger population and the
// opposite is true. The figures below are counted on origin/main for the
// before and on this branch for the after, which is the correction the second
// version of this comment needed: it counted both on the branch, where en has
// already gained this card's own keys, and so reported a before that no tree
// ever held.
//
// Before dinah-287, on origin/main, the roster was [en, hi, de], every catalog
// carried 631 entries and none of them was a skeleton, so the loop covered 631
// of German and 631 of Hindi, which is 1262. After it, en carries 647 and each
// of those two carries 647 of which 111 are skeletons, entries the rename left
// holding renamed English, and this loop skips a skeleton entry, so it covers
// 536 apiece and 1072 together. The five catalogs that joined the loop are
// skeletons throughout and contribute nothing at all. The population fell by
// 190 and gained none.
//
// The change is still the right one, on the merits rather than on the size of
// the set. A skeleton entry holds English rather than a translation and has no
// source it could have fallen behind, so checking one would assert nothing;
// every entry that does hold a translation is checked here, whichever catalog
// carries it and whatever roster that catalog is on. What the edit gives up is
// the empty-population alarm's ability to report a roster emptied by accident,
// which the workbench document "Translation staleness contract" names as its
// designed behaviour. That document is amended by this card to describe the
// guard the tree now has, and the roster change itself is the operator's to
// rule on rather than this test's to absorb.
//
// The base catalog and any skeleton entry are exempt, because neither is a
// translation of anything and so neither has a source to fall behind. A key
// missing from a catalog is left to TestEveryDeclaredLanguageShips, which
// already reports it, rather than reported twice.
func TestATranslationTracksItsEnglishSource(t *testing.T) {
	checked := 0
	for _, tag := range Tags() {
		if tag == Base {
			continue
		}
		t.Run(tag, func(t *testing.T) {
			catalog, shipped := loaded[tag]
			if !shipped {
				t.Fatalf("the catalog %s does not ship, so no entry of it can be checked", tag)
			}
			for _, key := range Keys() {
				base, ok := BaseEntry(key)
				if !ok {
					continue
				}
				entry, carried := catalog.Entries[key]
				if !carried || entry.Skeleton {
					continue
				}
				checked++
				want := Fingerprint(base.Text)
				switch {
				case entry.Source == "":
					t.Errorf("%s: carries no recorded source, so write one with Fingerprint of the English text before this entry ships", key)
				case entry.Source != want:
					t.Errorf("%s: the source is stale, so the English text changed after this entry was translated; git log -p internal/msg/locales/en.json shows what changed", key)
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no translated entry was checked, so this guard is asserting nothing")
	}
}

// TestEveryUntranslatableIdentifierSurvivesTranslation asserts that a
// translation carries through, unchanged, every machine identifier its English
// source names, for each entry whose context declares those identifiers are
// never translated. The declaration is the selector, so an entry earns this
// check by saying in its own context note that part of its text is machine
// vocabulary. A check keyed on the help.environment key name would cover the
// same ground today and would not notice the second such entry arriving, which
// is how German lost DINAH_MCP_ROOT in the first place; a check keyed on the
// identifier pattern alone would flag the four help.group headings, whose
// ALL-CAPS words are section titles a translator is supposed to render (READ
// becomes LESEN).
//
// Today the selector picks out help.environment alone, because it is the only
// entry that both declares its content untranslatable and spells that content
// in the identifier shape this guard reads. Twelve other entries carry a
// "never translated" declaration about text held in a placeholder, which
// TestATranslationKeepsThePlaceholdersAndTheSplice already guards.
//
// The comparison is between token sets rather than name by name, because
// membership by substring cannot enforce a name another name contains:
// DINAH_EDITOR satisfies a containment test for EDITOR, so a catalog dropping
// the bare EDITOR passed the earlier form of this guard.
func TestEveryUntranslatableIdentifierSurvivesTranslation(t *testing.T) {
	identifier := regexp.MustCompile(`\b[A-Z][A-Z0-9_]{2,}\b`)
	checked := 0
	for _, key := range Keys() {
		entry, ok := BaseEntry(key)
		if !ok {
			continue
		}
		if !strings.Contains(entry.Context, "never translated") {
			continue
		}
		wanted := identifiers(identifier, entry.Text)
		if wanted == "" {
			continue
		}
		checked++
		for _, tag := range Tags() {
			rendered := For(tag).T(key)
			if got := identifiers(identifier, rendered); got != wanted {
				t.Errorf("%s/%s: wanted the identifiers %s, got %s in %q", tag, key, wanted, got, rendered)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no English entry both declares its content untranslatable and names an identifier, so this guard checks nothing")
	}
}

// identifiers returns the identifiers pattern finds in text, sorted and joined,
// so that two texts naming the same identifiers compare equal whatever order
// each puts them in and however the sentence around them is worded.
func identifiers(pattern *regexp.Regexp, text string) string {
	found := pattern.FindAllString(text, -1)
	sort.Strings(found)
	return strings.Join(found, ", ")
}
