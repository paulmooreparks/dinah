package msg

import (
	"fmt"
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
// read. Complete is [Base, "hi", "de"], so the old loop covered two languages
// and compared English against itself, and the five skeleton catalogs went
// unread by the guard over the very thing a skeleton will stop carrying the
// day somebody translates it. A skeleton is byte-identical English today, so
// widening the roster costs nothing and asserts the same claim over every
// catalog that ships, which is the shape TestATranslationTracksItsEnglishSource
// in this file already uses. Widening a walk without a floor is the vacuous
// check dinah-406 is about, so the pairs it compared are counted and a run that
// compared none is fatal.
func TestATranslationKeepsThePlaceholdersAndTheSplice(t *testing.T) {
	pairs := 0
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
			pairs++
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
	if pairs == 0 {
		t.Fatal("no key pair was compared, so this guard is asserting nothing")
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
// language. Each subtest counts two populations of its own and fails on either
// at zero: the key pairs it compared, and the placeholder names it read off
// that tag's translations. Two counters rather than one, because a placeholder
// pattern that stopped matching, or a translation side that went empty, leaves
// the pairs count healthy while the check reads nothing.
func TestATranslationInventsNoPlaceholder(t *testing.T) {
	base, ok := loaded[Base]
	if !ok {
		t.Fatalf("the base catalog %s does not ship, so nothing can be compared against it", Base)
	}
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
			if report.pairs == 0 {
				t.Fatalf("%s shares no key with the base catalog, so this guard compared nothing", tag)
			}
			if report.placeholders == 0 {
				t.Fatalf("%s carries no placeholder at all, so this guard read nothing", tag)
			}
			for _, finding := range report.invented {
				t.Errorf("%s: the translation names a value the English does not, and nobody fills it, so a reader meets the name itself", finding)
			}
		})
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
// subtest, so a skeleton catalog joins the guard the moment somebody
// translates one of its entries and nothing here has to be edited for that to
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
// the set. A skeleton entry holds English rather than a translation and has no
// source it could have fallen behind, so checking one would assert nothing;
// every entry that does hold a translation is checked here, whichever catalog
// carries it and whatever roster that catalog is on. What the edit gives up is
// the empty-population alarm's ability to report a roster emptied by accident,
// which the workbench document names as its designed behaviour, and the
// checked == 0 fatal at the foot of this test is what remains of it.
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
				if !carried || entry.Skeleton {
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
// to skeletons, which would have taken both languages off the list.
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
// is on. An entry that is not a skeleton must differ from its English source,
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
				if !carried || entry.Skeleton {
					continue
				}
				checked++
				switch {
				case entry.Text == base.Text && !entry.Verbatim:
					t.Errorf("%s: carries the English text and is marked neither skeleton nor verbatim, so English is standing where a translation should be", key)
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

// TestASkeletonEntryReallyCarriesTheEnglishText is the other half of the rule
// above, and it is what stops a catalog answering the guard by marking
// everything a skeleton. A skeleton entry is defined as the English text
// standing in for a translation nobody has written, so an entry marked
// skeleton whose text is not the English text is misdescribing itself: either
// somebody translated it and left the flag on, in which case the reader is
// told the language has an untranslated entry it does not have, or the entry
// holds text that came from nowhere this project can name.
func TestASkeletonEntryReallyCarriesTheEnglishText(t *testing.T) {
	checked := 0
	for _, tag := range Tags() {
		if tag == Base {
			continue
		}
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
			if !carried || !entry.Skeleton {
				continue
			}
			checked++
			if entry.Text != base.Text {
				t.Errorf("%s/%s: is marked as a skeleton and does not carry the English text", tag, key)
			}
			if entry.Verbatim {
				t.Errorf("%s/%s: is marked both skeleton and verbatim, and the two say different things about the same entry", tag, key)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no skeleton entry was checked, so this guard is asserting nothing")
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
// The subject is the English catalog alone. Every other catalog either
// carries the English text unchanged, which this guard has already read
// through the base entry, or carries a translation nobody on this project can
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
