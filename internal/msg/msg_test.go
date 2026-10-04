package msg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEveryDeclaredLanguageShips asserts that the catalogs the language ruling
// calls for are exactly the ones that ship, each one as a file the binary
// embeds, and that Complete names only declared languages. A catalog may
// carry no entry at all; the language still ships, and its reader is served
// English one key at a time. Whether German and Hindi carry every key is a
// release question, which TestEveryReleaseLanguageIsComplete answers.
func TestEveryDeclaredLanguageShips(t *testing.T) {
	declared := map[string]bool{}
	for _, tag := range Declared {
		declared[tag] = true
	}
	for _, tag := range Complete {
		if !declared[tag] {
			t.Errorf("%s is on Complete and is not a declared language", tag)
		}
	}
	shipped := map[string]bool{}
	for _, tag := range Tags() {
		shipped[tag] = true
		if !declared[tag] {
			t.Errorf("%s ships a catalog and is not a declared language", tag)
		}
		if _, ok := loaded[tag]; !ok {
			t.Errorf("%s is listed and its catalog did not load", tag)
		}
	}
	for tag := range declared {
		if !shipped[tag] {
			t.Errorf("the declared catalog %s does not ship", tag)
		}
	}
	if Tags()[0] != Base {
		t.Errorf("the base language should lead the list, got %s", Tags()[0])
	}
	if translated, total := Coverage(Base); total == 0 || translated != total {
		t.Errorf("the base catalog reports %d of %d keys", translated, total)
	}
}

// assertTheEnglishCarries asserts that the base catalog carries every key
// named, each with a text and a context for a translator. It is what a card's
// own key test asks of its keys. A translation of one, where a catalog carries
// it, is held by the guards in this file that read every translation, and
// German and Hindi carrying all of them is the release gate's question, so a
// card's test names no language but English.
func assertTheEnglishCarries(t *testing.T, keys []string) {
	t.Helper()
	if len(keys) == 0 {
		t.Fatal("the subject set is empty, so this test reads nothing")
	}
	for _, key := range keys {
		entry, ok := BaseEntry(key)
		if !ok {
			t.Errorf("English carries no entry for %s", key)
			continue
		}
		if entry.Text == "" || entry.Context == "" {
			t.Errorf("the base entry for %s carries an empty text or an empty context", key)
		}
		if entry.Source != "" || entry.Verbatim {
			t.Errorf("the base entry for %s carries a source or a verbatim mark, and it is a translation of nothing", key)
		}
	}
}

// releaseCheck is the environment variable that turns on the checks a release
// needs and an ordinary change does not. The promotion workflow sets it.
const releaseCheck = "DINAH_RELEASE_CHECK"

// TestEveryReleaseLanguageIsComplete asserts that every language on Complete
// carries a translation for every key the base catalog carries. It is the
// release gate and nothing else, so it skips unless DINAH_RELEASE_CHECK is
// set: a change may add an English key without German or Hindi, and the
// release that would ship it may not.
//
// The comparison lives in missingTranslations so that the arming test below
// can drive it over a catalog written in the test, without the release
// switch, and so a gate that never fails cannot hide behind the skip.
func TestEveryReleaseLanguageIsComplete(t *testing.T) {
	if os.Getenv(releaseCheck) == "" {
		t.Skipf("set %s to run the release completeness check", releaseCheck)
	}
	base, ok := loaded[Base]
	if !ok {
		t.Fatalf("the base catalog %s does not ship", Base)
	}
	checked := 0
	for _, tag := range Complete {
		if tag == Base {
			continue
		}
		checked++
		catalog, shipped := loaded[tag]
		if !shipped {
			t.Errorf("%s is on Complete and ships no catalog", tag)
			continue
		}
		missing := missingTranslations(base.Entries, catalog.Entries)
		if len(missing) > 0 {
			t.Errorf("%s lacks a translation for %d of %d keys, and a release ships it complete: %s", tag, len(missing), len(base.Entries), strings.Join(missing, ", "))
		}
	}
	if checked == 0 {
		t.Fatal("Complete names no language but the base, so this gate checked nothing")
	}
}

// missingTranslations returns, sorted, every key the base carries that other
// does not carry with a text.
func missingTranslations(base, other map[string]Entry) []string {
	var missing []string
	for key := range base {
		if entry, carried := other[key]; !carried || entry.Text == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

// TestTheReleaseGateReportsAMissingKey arms the release gate without the
// release switch: a catalog missing one key and carrying another empty
// reports both of them and nothing else.
func TestTheReleaseGateReportsAMissingKey(t *testing.T) {
	base := map[string]Entry{
		"fixture.present": {Text: "Here"},
		"fixture.absent":  {Text: "Gone"},
		"fixture.empty":   {Text: "Blank"},
	}
	other := map[string]Entry{
		"fixture.present": {Text: "Hier"},
		"fixture.empty":   {Text: ""},
	}
	if got := strings.Join(missingTranslations(base, other), ", "); got != "fixture.absent, fixture.empty" {
		t.Errorf("wanted the absent and the empty key, got %q", got)
	}
}

// TestATranslationCarriesNoKeyEnglishLacks asserts that no catalog carries a
// key the base catalog has retired. Nothing renders such an entry, so it is
// dead weight a translator would go on maintaining, and a rename that adds
// the new key and leaves the old one behind is the way it arrives.
func TestATranslationCarriesNoKeyEnglishLacks(t *testing.T) {
	checked := 0
	for _, tag := range Tags() {
		if tag == Base {
			continue
		}
		checked++
		for key := range loaded[tag].Entries {
			if _, ok := BaseEntry(key); !ok {
				t.Errorf("%s/%s: the base catalog carries no such key, so nothing renders this entry", tag, key)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no catalog but the base ships, so this guard read nothing")
	}
}

// TestEveryKeyCarriesAContext asserts that a translator gets told what each
// message is for.
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
// language ruling asks of a catalog that carries no entry for a key.
func TestMissingKeysFallBackPerKey(t *testing.T) {
	hindi := For("hi")
	if got := hindi.T("word.yes"); got == For(Base).T("word.yes") {
		t.Error("a translated key should not render as English")
	}
	lacking := ""
	for _, key := range Keys() {
		if _, carried := CatalogEntry("cs", key); !carried {
			lacking = key
			break
		}
	}
	if lacking == "" {
		t.Fatal("Czech carries every key, so pick a shipped language that lacks one for the fallback arm")
	}
	if got := For("cs").T(lacking); got != For(Base).T(lacking) {
		t.Errorf("%s: a key the catalog lacks should fall back to English, got %q", lacking, got)
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

// placeholderPattern is the spelling of a placeholder both guards below read,
// compiled once so that the two of them cannot drift apart. A guard whose idea
// of a placeholder differs from the renderer's checks a different thing from
// the one that ships, and the divergence would show up as a name the guard
// thinks is safe and fill leaves standing at a reader.
var placeholderPattern = regexp.MustCompile(`\{[a-zA-Z][a-zA-Z0-9_.-]*\}`)

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
//
// The loop reads every shipped catalog rather than the Complete roster it once
// read, so a language joins the guard the day somebody translates its first
// entry, which is the shape TestATranslationTracksItsEnglishSource in this
// file already uses. A key a catalog does not carry renders as English and
// passes trivially. Widening a walk without a floor is the vacuous check
// dinah-406 is about, so the pairs it compared are counted and a run that
// compared none is fatal.
//
// The two halves are counted apart, one floor each. A single counter here would
// be fed by two populations at once, because an entry enters this loop either
// by carrying a placeholder or by opening with the splice, and the two are
// mostly different entries. The base catalog carries 48 keys that open with the
// splice and name no placeholder, which is enough pairs to hold a combined
// counter above zero on their own while the placeholder half reads nothing at
// all. That is the shape this card exists to close, and D-6 and AC-8 are where
// the same seam is split on the extension side, so it is split here too.
func TestATranslationKeepsThePlaceholdersAndTheSplice(t *testing.T) {
	placeholderPairs := 0
	splicePairs := 0
	for _, key := range Keys() {
		entry, ok := BaseEntry(key)
		if !ok {
			continue
		}
		names := placeholderPattern.FindAllString(entry.Text, -1)
		splice := strings.HasPrefix(entry.Text, "; ")
		if len(names) == 0 && !splice {
			continue
		}
		for _, tag := range Tags() {
			if tag == Base {
				continue
			}
			if len(names) > 0 {
				placeholderPairs++
			}
			if splice {
				splicePairs++
			}
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
	if placeholderPairs == 0 {
		t.Error("no pair whose English names a placeholder was compared, so the placeholder half of this guard is asserting nothing")
	}
	if splicePairs == 0 {
		t.Error("no pair whose English opens with the splice was compared, so the splice half of this guard is asserting nothing")
	}
}

// placeholderReport is what one catalog's comparison found, and how much of it
// there was to find.
type placeholderReport struct {
	// pairs is the key pairs compared.
	pairs int
	// placeholders is the placeholder names read off the translation side,
	// counted per pair. It is the translation side rather than the English
	// side because that is the side the invention guard reads, and a pairs
	// count stays high while the side being read goes empty.
	placeholders int
	// dropped holds one entry per name the English carries and the
	// translation does not.
	dropped []string
	// invented holds one entry per name the translation carries and the
	// English does not.
	invented []string
}

// placeholderNames returns the placeholder names text carries, as a set.
// Comparison is by set rather than by multiset: a translator may name a value
// once where the English names it twice, and word order is theirs, so a count
// comparison would fire on correct German in a language nobody on this project
// reads. A name present on one side and absent on the other is the defect.
func placeholderNames(text string) map[string]bool {
	names := map[string]bool{}
	for _, name := range placeholderPattern.FindAllString(text, -1) {
		names[name] = true
	}
	return names
}

// comparePlaceholders reports what one catalog does with the placeholder names
// its English carries, in both directions.
//
// The two catalogs are arguments rather than reads of the package's own loaded
// map, so a fixture can drive this over a catalog that really does drop a name
// and really does invent one. No shipped catalog has to be allowed to carry a
// defect in order for the guard to have an armed path.
//
// A key the other catalog does not carry is skipped rather than counted or
// reported. A reader of that language meets the English for it, which carries
// the English placeholders by construction, and whether a release language
// carries every key is TestEveryReleaseLanguageIsComplete's question.
func comparePlaceholders(base, other map[string]Entry, tag string) placeholderReport {
	report := placeholderReport{}
	for key, english := range base {
		entry, carried := other[key]
		if !carried {
			continue
		}
		report.pairs++
		wanted := placeholderNames(english.Text)
		held := placeholderNames(entry.Text)
		report.placeholders += len(held)
		for name := range wanted {
			if !held[name] {
				report.dropped = append(report.dropped, fmt.Sprintf("%s/%s: %s", tag, key, name))
			}
		}
		for name := range held {
			if !wanted[name] {
				report.invented = append(report.invented, fmt.Sprintf("%s/%s: %s", tag, key, name))
			}
		}
	}
	sort.Strings(report.dropped)
	sort.Strings(report.invented)
	return report
}

// TestATranslationInventsNoPlaceholder asserts that a translation names no
// value its English does not, which is the direction neither this package nor
// the extension caught before dinah-406.
//
// It sits beside TestATranslationKeepsThePlaceholdersAndTheSplice rather than
// folded into it, because the two report different defects and a reader meeting
// a failure should not have to work out which half tripped. A dropped name
// leaves a hole in a sentence. An invented one renders its own characters at a
// reader, because fill leaves a name nobody passed alone, which is a defect
// that looks like a translation.
//
// One subtest per tag over every shipped catalog but the base, which is the
// shape TestATranslationTracksItsEnglishSource uses, so a failure names the
// language. A catalog carrying no entry yet has nothing to compare and passes
// its subtest. The floors are taken over the whole run instead, and there are
// two of them: the key pairs compared, and the placeholder names read off the
// translation side. Two counters rather than one, because a placeholder
// pattern that stopped matching, or a translation side that went empty, leaves
// the pairs count healthy while the check reads nothing.
func TestATranslationInventsNoPlaceholder(t *testing.T) {
	base, ok := loaded[Base]
	if !ok {
		t.Fatalf("the base catalog %s does not ship, so nothing can be compared against it", Base)
	}
	pairs, placeholders := 0, 0
	for _, tag := range Tags() {
		if tag == Base {
			continue
		}
		t.Run(tag, func(t *testing.T) {
			catalog, shipped := loaded[tag]
			if !shipped {
				t.Fatalf("the catalog %s does not ship, so no entry of it can be checked", tag)
			}
			report := comparePlaceholders(base.Entries, catalog.Entries, tag)
			pairs += report.pairs
			placeholders += report.placeholders
			for _, finding := range report.invented {
				t.Errorf("%s: the translation names a value the English does not, and nobody fills it, so a reader meets the name itself", finding)
			}
		})
	}
	if pairs == 0 {
		t.Fatal("no catalog shares a key with the base catalog, so this guard compared nothing")
	}
	if placeholders == 0 {
		t.Fatal("no translation carries a placeholder at all, so this guard read nothing")
	}
}

// TestThePlaceholderComparisonReportsBothDirections drives the comparison over
// two catalogs written here, and asserts both lists by content.
//
// No shipped catalog carries either defect, so nothing above proves the
// translation side is read at all: a comparison that reported every pair and a
// comparison that reported the right pair are both green against the tree as it
// stands. The third key is the clean case, carrying the same two names in a
// different order, and it is what a comparison reporting everything would fail.
func TestThePlaceholderComparisonReportsBothDirections(t *testing.T) {
	base := map[string]Entry{
		"fixture.dropped":   {Text: "Copied {ref}"},
		"fixture.invented":  {Text: "Moved {card}"},
		"fixture.reordered": {Text: "{from} became {to}, and {from} is gone"},
	}
	other := map[string]Entry{
		"fixture.dropped":   {Text: " kopiert"},
		"fixture.invented":  {Text: "{card} verschoben nach {ziel}"},
		"fixture.reordered": {Text: "{to} kommt von {from}"},
	}

	report := comparePlaceholders(base, other, "xx")

	if report.pairs != 3 {
		t.Errorf("wanted 3 pairs compared, got %d", report.pairs)
	}
	if report.placeholders != 4 {
		t.Errorf("wanted 4 placeholder names read off the translation side, got %d", report.placeholders)
	}
	if got := strings.Join(report.dropped, ", "); got != "xx/fixture.dropped: {ref}" {
		t.Errorf("wanted the dropped name and nothing else, got %q", got)
	}
	if got := strings.Join(report.invented, ", "); got != "xx/fixture.invented: {ziel}" {
		t.Errorf("wanted the invented name and nothing else, got %q", got)
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
// subtest, so a catalog with no entries yet joins the guard the moment
// somebody translates one and nothing here has to be edited for that to
// happen.
//
// The loop reads every shipped catalog rather than the Complete roster it once
// read, because the roster answers a different question. Complete is
// catalog-level and says a language has no untranslated entry left, which is
// not the same as saying the language carries translations worth checking.
// dinah-287 nearly took Hindi and German off that roster while both still
// carried hundreds of real translations, and a guard keyed on the roster would
// have stopped checking those translations on the day it changed. The operator
// ruled that the two languages be retranslated instead, so the roster came
// through the card unchanged, and this loop no longer depends on that.
//
// What the change costs is a smaller population, and this comment no longer
// writes down how much smaller. Three consecutive rounds of review each
// corrected that figure and each left it wrong somewhere, which is the signal
// that a count is the wrong kind of thing to maintain by hand. A number
// written into a comment is taken on some tree on some day and is stale by the
// next commit, and this file's copy of it disagreed with a transcript in
// docs/quick-start.md that the same commit regenerated.
//
// So the counts live in one place, and that place computes them: `dinah
// version --catalogs` prints every shipped catalog with its translated count
// over its total, read off the catalogs themselves. The population this loop
// covers is the sum of the translated column over every catalog but English,
// and the entries a retranslation would face in a language is the difference
// between its two columns. The workbench document "Translation staleness
// contract" carries that command and its output at a named commit, and
// nothing else in the tree writes the figures down.
//
// The change is still the right one, on the merits rather than on the size of
// the set. Every entry a catalog carries is a translation, so every one of
// them is checked here, whichever catalog carries it and whatever roster that
// catalog is on. What the edit gives up is the empty-population alarm's
// ability to report a roster emptied by accident, which the workbench
// document names as its designed behaviour, and the checked == 0 fatal at the
// foot of this test is what remains of it.
//
// The base catalog is exempt, because it is a translation of nothing and so
// has no source to fall behind. A key missing from a catalog is not stale and
// is not reported here; for German and Hindi it is the release gate's
// business, TestEveryReleaseLanguageIsComplete.
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
				if !carried {
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

// TestCatalogEntryReadsWithoutFallback asserts that CatalogEntry answers out
// of one catalog alone: it finds a German entry, refuses a tag no catalog
// answers to, and refuses a key the catalog does not carry, rather than
// falling back to English the way a Renderer does. A guard outside this
// package reads a translation's own text through it, so a silent fallback
// here would make that guard pass on an entry the catalog never carried.
func TestCatalogEntryReadsWithoutFallback(t *testing.T) {
	if entry, ok := CatalogEntry("de", "word.yes"); !ok || entry.Text == "" {
		t.Fatalf("wanted the German entry, got %+v, %v", entry, ok)
	}
	if _, ok := CatalogEntry("qq", "word.yes"); ok {
		t.Error("wanted false for a language no catalog answers to")
	}
	if _, ok := CatalogEntry("de", "no.such.key"); ok {
		t.Error("wanted false for a key the catalog does not carry")
	}
}

// machineSpans are the four shapes the catalog spells machine vocabulary in.
// A glossary trigger reads the base English as prose, so these come out of
// the text before it is matched: each names a thing rather than saying a
// word about it, and a trigger firing on one demands a translated word where
// a correct translation carries none. card.line's "[{state} / {substate}]"
// is a placeholder line with no prose in it at all, and
// refusal.dinah.ambiguous-state.next's "name one as `dinah pull <state>`, or
// pass --state <state>" names the flag three times and the concept never.
var machineSpans = []*regexp.Regexp{
	regexp.MustCompile(`\{[^}]*\}`),
	regexp.MustCompile("`[^`]*`"),
	regexp.MustCompile(`<[^>]*>`),
	regexp.MustCompile(`--[A-Za-z0-9-]+`),
}

// prose returns text with every machine-vocabulary span removed, which is
// what a glossary trigger matches against.
func prose(text string) string {
	for _, span := range machineSpans {
		text = span.ReplaceAllString(text, "")
	}
	return text
}

// TestATranslationUsesTheDeclaredWord asserts that a translation renders each
// recurring concept glossary.json declares with one of that language's
// declared words for it. It is the guard for the failure a fingerprint cannot
// see: an entry current with the English it was translated from, and carrying
// the wrong word for a term the rest of the catalog is consistent about.
// German said Eigentuemer, Eigner and Akteur for one owner, and dropped the
// word for a level on one row of a pair whose other row carries it.
//
// The trigger is the English phrase, matched case-insensitively and on word
// boundaries against the base text with its machine-vocabulary spans removed.
// The match is containment of a declared form in the translated text, because
// a language inflects its own words and spells them inside compounds, so
// German's Zielzustand carries Zustand and is not a wrong word. A form the
// corpus turns out to need is added to glossary.json with the evidence for
// it; neither the trigger nor the match is loosened to make a failure go away.
func TestATranslationUsesTheDeclaredWord(t *testing.T) {
	if len(glossary) == 0 {
		t.Fatal("the glossary carries no terms, so this guard is asserting nothing")
	}
	trigger := make(map[string]*regexp.Regexp, len(glossary))
	for _, term := range glossary {
		pattern := `(?i)\b` + regexp.QuoteMeta(term.EN) + `\b`
		trigger[term.EN] = regexp.MustCompile(pattern)
	}
	checked := 0
	for _, tag := range Tags() {
		if tag == Base {
			continue
		}
		t.Run(tag, func(t *testing.T) {
			for _, key := range Keys() {
				base, ok := BaseEntry(key)
				if !ok {
					continue
				}
				if strings.Contains(base.Context, "never translated") {
					continue
				}
				entry, carried := CatalogEntry(tag, key)
				if !carried {
					continue
				}
				plain := prose(base.Text)
				for _, term := range glossary {
					if !trigger[term.EN].MatchString(plain) {
						continue
					}
					forms, declared := term.Forms[tag]
					if !declared {
						continue
					}
					checked++
					if carriesAForm(entry.Text, forms) {
						continue
					}
					t.Errorf("%s: wanted the glossary word for %q (one of %v), got %q", key, term.EN, forms, entry.Text)
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no entry triggered a glossary term, so this guard is asserting nothing")
	}
}

// carriesAForm reports whether text spells the term in any of the renderings
// the language declares for it.
func carriesAForm(text string, forms []string) bool {
	for _, form := range forms {
		if strings.Contains(text, form) {
			return true
		}
	}
	return false
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

// TestATranslationIsNotEnglishUnderAnotherTag asserts that an entry a catalog
// presents as translated is genuinely in that catalog's language, and it is
// the check that survives a language leaving the Complete roster.
//
// Completeness and correctness are different properties, and dinah-287 made
// the difference matter. Complete says a catalog has no untranslated entry
// left, and that card's rename sent a block of German and Hindi entries back
// to English copies marked as skeletons, a shape catalogs no longer carry,
// which would have taken both languages off the list.
// TestEveryDeclaredLanguageShips only ever asserted anything about a language
// on one of the two rosters, so on the branch where those two had left it,
// nothing checked their contents at all: replacing all of German's remaining
// translations with the English text left the package green. The operator
// ruled that the entries be retranslated and both languages stayed on the
// list, so that branch is not the tree you are reading, but the hole it
// exposed was real and this guard is what closed it. A language off the roster
// is not expected to be complete. It is still expected that what it does carry
// is genuinely translated, and that an entry holding English says so.
//
// So the rule here is keyed on the entry rather than on the roster its catalog
// is on. Every entry a catalog carries must differ from its English source,
// unless it carries Verbatim, which is a translator saying the answer really
// is the English word. Both arms of that are load-bearing: without the first,
// English refilling a catalog passes; without the second, every German table
// heading reading "Name" fails and the guard gets switched off.
//
// The stale-flag arm keeps Verbatim honest. An entry marked verbatim whose
// text no longer matches English is either a translation somebody wrote
// without clearing the flag or an English source that moved underneath it, and
// either way the claim the flag makes is no longer true.
func TestATranslationIsNotEnglishUnderAnotherTag(t *testing.T) {
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
				if !carried {
					continue
				}
				checked++
				switch {
				case entry.Text == base.Text && !entry.Verbatim:
					t.Errorf("%s: carries the English text and is not marked verbatim, so English is standing where a translation should be; delete the entry and the reader gets the English anyway", key)
				case entry.Text != base.Text && entry.Verbatim:
					t.Errorf("%s: is marked verbatim and no longer matches the English text, so the flag says something that is no longer true", key)
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no entry a catalog presents as translated was checked, so this guard is asserting nothing")
	}
}

// TestEveryCatalogFileDecodesStrictly asserts that every embedded catalog file
// carries only the members Catalog and Entry declare. The loader ignores an
// unknown member, so a misspelt "verbatim" or "source" would otherwise vanish
// on load and leave the guards above reading an entry that does not say what
// its author wrote. The retired "skeleton" member is the case worth naming:
// catalogs once carried a copy of the English under that mark for every key,
// and they carry only real translations now, so a tool that regenerates the
// old shape fails here by name.
func TestEveryCatalogFileDecodesStrictly(t *testing.T) {
	files, err := locales.ReadDir("locales")
	if err != nil {
		t.Fatalf("read the embedded catalogs: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no catalog file is embedded, so this guard read nothing")
	}
	for _, file := range files {
		data, err := locales.ReadFile(path.Join("locales", file.Name()))
		if err != nil {
			t.Errorf("%s: %v", file.Name(), err)
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var catalog Catalog
		if err := decoder.Decode(&catalog); err != nil {
			t.Errorf("%s does not decode strictly, and a catalog carries real translations under the declared members only: %v", file.Name(), err)
		}
	}
}

// theIndefiniteArticle matches an English indefinite article standing
// immediately before a placeholder, which is the shape this guard refuses.
// The article is captured so the failure can quote what was written rather
// than only where.
var theIndefiniteArticle = regexp.MustCompile(`(?i)\b(an?)\s+(\{[a-zA-Z]+\})`)

// TestNoEnglishSentenceTakesAnArticleBeforeAPlaceholder asserts that no
// English catalog entry writes "a" or "an" in front of a placeholder.
//
// A placeholder carries a value the sentence cannot see, and the English
// indefinite article has to agree with the sound the word after it starts
// with, so the choice between the two forms is made before the value is
// known and is right only by luck. dinah.unknown-field's sentence was
// written when the kind it named was always a card, so "a card" agreed by
// construction; generalising it to any kind made it render "a item" and "a
// attachment". The repair is not a better article, because there is no
// article that fits every value: it is a sentence that names the value
// without one.
//
// That makes the rule cheap to hold and independent of how many kinds there
// are, which is the point. An eighth kind, or a ninth, changes nothing here,
// because no sentence this guard admits depends on which values exist.
//
// The subject is the English catalog alone. A key another catalog lacks
// renders the English, which this guard has already read through the base
// entry, and a key it carries is a translation nobody on this project can
// grade; what a translation is held to instead is the rule written into each
// affected entry's context, and the recorded source fingerprint, which sends
// the entry back to a translator whenever the English moves.
func TestNoEnglishSentenceTakesAnArticleBeforeAPlaceholder(t *testing.T) {
	checked := 0
	for _, key := range Keys() {
		entry, ok := BaseEntry(key)
		if !ok || !strings.Contains(entry.Text, "{") {
			continue
		}
		checked++
		for _, match := range theIndefiniteArticle.FindAllStringSubmatch(entry.Text, -1) {
			t.Errorf("%s: writes %q before %s, and no article agrees with every value that placeholder carries; name the value without one", key, match[1], match[2])
		}
	}
	if checked == 0 {
		t.Fatal("no English entry carries a placeholder, so this guard read nothing")
	}
	t.Logf("the guard read %d English entries carrying a placeholder", checked)
}
