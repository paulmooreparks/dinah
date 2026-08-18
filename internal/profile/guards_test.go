package profile

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"dinah/internal/textwidth"
)

// lockAcquisition matches the exclusive-create primitive a lock is taken with.
var lockAcquisition = regexp.MustCompile(`\bO_EXCL\b`)

// lockWrite matches a line that both names a lock file and writes one, which
// is the other shape a hand-rolled acquisition takes.
var lockWrite = regexp.MustCompile(`\b(OpenFile|Create|WriteText|WriteFile)\b`)
var lockNamed = regexp.MustCompile(`\b(LockName|SiblingSuffix)\b`)

// theOneAcquirer is the file every lock in this codebase is created in.
const theOneAcquirer = "internal/bench/lock.go"

// TestNoLockIsCreatedOutsideTheOneAcquirer asserts that nothing in the shipped
// binary creates a lock file except the acquisition in internal/bench/lock.go.
//
// The sibling check that stops a writer from working inside a structural act's
// window lives inside that acquisition rather than at each call site, so a
// second place that took a lock would be a place the check could be forgotten.
// An earlier enumeration of the call sites missed one, which is why the rule
// is a guard rather than a comment.
//
// Test sources are outside the scan: a test that plants a lock by hand is
// standing in for another process rather than acquiring one, and no such line
// reaches a caller of this library.
func TestNoLockIsCreatedOutsideTheOneAcquirer(t *testing.T) {
	root := filepath.Join("..", "..")
	scanned := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		name := filepath.ToSlash(relative)
		if name == theOneAcquirer {
			return nil
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		scanned++
		for number, line := range strings.Split(string(source), "\n") {
			if lockAcquisition.MatchString(line) {
				t.Errorf("%s:%d takes a lock with the exclusive-create primitive: %s", name, number+1, strings.TrimSpace(line))
			}
			if lockNamed.MatchString(line) && lockWrite.MatchString(line) {
				t.Errorf("%s:%d writes a lock file: %s", name, number+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if scanned == 0 {
		t.Error("no source was scanned, so this guard proves nothing")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(theOneAcquirer))); err != nil {
		t.Errorf("the one acquirer is not where this guard exempts it: %v", err)
	}
}

// theShortWord matches the retired spelling of the product's own noun. The
// pattern is loose, since the scan removes every legitimate spelling from a
// line before reaching for it.
var theShortWord = regexp.MustCompile(`(?i)bench`) // retired spelling, named deliberately

// theDeliberateSpelling exempts the line it appears on. It belongs on this
// guard's own pattern, which has to spell what it looks for, and on the
// assertions proving the retired flag and the retired variable are gone,
// which have to type them to watch the tool refuse them. Nothing else has a
// claim on it. The marker sits on the line rather than in a list here, so a
// reader meets the reason where the spelling is instead of hunting for a
// file-wide exemption.
const theDeliberateSpelling = "// retired spelling, named deliberately"

// vocabularyExceptions are the tokens that still spell the short word and are
// meant to. Each is removed from a line before the line is scanned, so an
// exception covers its own token and widens nothing around it.
//
// Every surface carries a refusal name as the coordination contract spells
// it, the statement identifiers belong to the core profile, and the package
// path is where the Go identifiers live. Longer tokens come first so that
// removing a shorter one cannot strand the tail of a longer one.
var vocabularyExceptions = []string{
	"CORE-BENCH",
	"internal/bench",
}

// vocabularyReason is what a reader who trips this guard needs: why the word
// is refused here, and what to do instead of adding an exception.
const vocabularyReason = "the product's word is workbench, and the short word " +
	"survives only in Go identifiers, in the comments naming them, and in the " +
	"contract tokens this guard lists; rewrite the text rather than widening " +
	"the exceptions"

// foldStringConcat's known gaps: it folds only a +-chain of string literals
// inside one expression. It does not resolve a named constant or variable
// even when that identifier's own value is itself a literal chain defined
// elsewhere; it does not follow a value built across more than one
// statement, such as repeated strings.Builder.WriteString calls, a
// compound += assignment, fmt.Sprintf, or strings.Join; and it has no
// notion of a value crossing a function or package boundary. Closing any
// of those needs real dataflow analysis or a type-checked pass with
// go/types, and is out of scope here on the judgement that the narrow
// case (see dinah-72) is the one this guard has actually been asked to
// catch, and a broader attempt risks false confidence more than it buys
// coverage.

// scannedExtensions are the file kinds this guard reads whole, which are the
// ones carrying text a person meets: documents, message catalogs, the
// byte-enforced help fixture, and CI workflow files.
var scannedExtensions = map[string]bool{".md": true, ".json": true, ".txt": true, ".yml": true, ".yaml": true}

// TestTheProductSaysWorkbenchEverywhereItIsRead asserts that the short word
// reaches no surface a person reads. Help text, message catalogs, documents,
// guides, refusal sentences, and test fixtures all say workbench, and the
// retired flag and the retired variable are gone rather than aliased.
//
// The word survives in Go as identifiers, as the package path internal/bench,
// and in the doc comments that name them, so a Go file is scanned for its
// string literals alone. That is where a flag spelling, a catalog key or a
// message would be reintroduced, and it is the boundary between what a
// contributor reads in the source and what a user reads in the output.
//
// The guard exists because several cards edit these surfaces at once and a
// prose reviewer cannot hold the whole tree at a glance.
func TestTheProductSaysWorkbenchEverywhereItIsRead(t *testing.T) {
	root := filepath.Join("..", "..")
	files, literals := 0, 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skippedTrees[entry.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		name := filepath.ToSlash(relative)
		extension := filepath.Ext(name)
		if extension != ".go" && !scannedExtensions[extension] {
			return nil
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		lines := strings.Split(string(source), "\n")
		files++
		if extension != ".go" {
			report(t, name, 0, name, "")
			for number, line := range lines {
				report(t, name, number+1, line, line)
			}
			return nil
		}
		found, scanErr := goStringLiterals(path)
		if scanErr != nil {
			return scanErr
		}
		literals += len(found)
		for _, held := range found {
			report(t, name, held.line, held.text, lines[held.line-1])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if files == 0 || literals == 0 {
		t.Errorf("nothing was scanned, so this guard proves nothing: %d files, %d string literals", files, literals)
	}
}

// skippedTrees are the directories this guard does not walk: git's own
// storage, the workbench this repository carries as data, and the binary
// assets that hold no prose.
var skippedTrees = map[string]bool{".git": true, ".dinah": true, "logo": true, "__pycache__": true}

// report fails the test when a subject still carries the short word, naming
// the file and the line so that whoever trips the guard reads an answer. A
// line number of zero names the file's own path rather than a line inside it,
// which is how a document or a guide named for the short word is caught.
//
// The subject is what gets scanned and the source is the line it came from,
// and the two differ for a Go file, where the subject is one string literal
// and the source is the whole line carrying it. The marker is looked for in
// the source, and only honoured in a file whose name ends in _test.go: every
// legitimate use sits in a test, and a production Go string literal has no
// standing to silence this guard.
func report(t *testing.T, name string, number int, subject, source string) {
	t.Helper()
	if !carriesTheShortWord(name, subject, source) {
		return
	}
	where := name
	if number > 0 {
		where = name + ":" + strconv.Itoa(number)
	}
	t.Errorf("%s carries the short word: %s\n%s", where, strings.TrimSpace(subject), vocabularyReason)
}

// carriesTheShortWord is report's decision, factored out so it can be
// asserted on directly (TestGuardCatchesAConcatenatedRetiredWord) without
// having to fail a *testing.T to observe the answer.
func carriesTheShortWord(name, subject, source string) bool {
	if strings.HasSuffix(name, "_test.go") && strings.Contains(source, theDeliberateSpelling) {
		return false
	}
	stripped := subject
	for _, allowed := range vocabularyExceptions {
		stripped = strings.ReplaceAll(stripped, allowed, "")
	}
	stripped = theLongWord.ReplaceAllString(stripped, "")
	return theShortWord.MatchString(stripped)
}

// theLongWord matches the spelling the product ruled for, removed from a line
// before the short one is looked for.
var theLongWord = regexp.MustCompile(`(?i)workbench`)

// nonLatinEntry holds one script's known transliteration(s) of one watched
// word: the retired word or the product name today, and whichever this
// table is next asked to catch. The current word's own spelling is
// stripped from a line first, exactly as theLongWord is stripped from the
// Latin scan, so a transliteration embedded inside a legitimate spelling
// does not trip its own guard. carriesNonLatinSpelling's boundary check
// (below) is the second layer that still applies after the strip, and the
// only one available to an entry whose longWord cannot appear in the
// target script.
type nonLatinEntry struct {
	// subject names which watched word this entry catches a transliteration
	// of (retiredWordSubject or productNameSubject), so
	// TestTheRetiredWordHasNoNonLatinSpelling and
	// TestTheProductNameHasNoNonLatinSpelling can each read only the
	// entries that are theirs out of a shared per-locale slice, and report
	// a hit with the right reason.
	subject string
	// longWord is the spelling stripped from a line before a
	// transliteration is looked for: the current word's own rendering in
	// this locale's script for a retired-word entry, or the product name's
	// own Latin spelling for a product-name entry, since the product name
	// is never transliterated in any locale.
	longWord string
	// shortWords are known transliterations of the watched word into this
	// locale's script.
	shortWords []string
}

// retiredWordSubject and productNameSubject are the two watched-word
// identities a nonLatinEntry can carry.
const (
	retiredWordSubject = "the retired word"
	productNameSubject = "the product name"
)

// nonLatinRetiredSpellings maps a BCP-47 locale tag to the nonLatinEntry
// values for every non-Latin-script locale carrying a known
// transliteration of a watched word worth cataloging. A tag may carry more
// than one entry, one per subject; entryFor reads the one a caller wants.
var nonLatinRetiredSpellings = map[string][]nonLatinEntry{
	"hi": {
		{
			subject:    retiredWordSubject,
			longWord:   "वर्कबेंच",
			shortWords: []string{"बेंच"},
		},
		{
			subject:    productNameSubject,
			longWord:   "Dinah",
			shortWords: []string{"डिना"},
		},
	},
}

// entryFor returns tag's nonLatinEntry for subject out of
// nonLatinRetiredSpellings, and whether one exists. Both production tests
// and both fixture tests use it, rather than each re-walking the slice, so
// a locale carrying entries for more than one subject is read the same way
// everywhere.
func entryFor(tag, subject string) (nonLatinEntry, bool) {
	for _, entry := range nonLatinRetiredSpellings[tag] {
		if entry.subject == subject {
			return entry, true
		}
	}
	return nonLatinEntry{}, false
}

// nonLatinRetiredSpellings' known gaps: an entry lists only the
// transliteration(s) somebody thought to add, and transliteration has no
// single correct spelling, so a translator choosing a different but equally
// defensible rendering of the same sound passes unnoticed. Devanagari has
// more than one defensible way to render the retired word's sound, using a
// different vowel matra than बेंच does, and none of those alternate
// renderings appear anywhere in the shipped catalogs today; adding one here
// would be inventing a spelling rather than cataloging one, so the entry
// stays at the single spelling actually grounded in existing text. The
// table only ever grows by a person adding to it; nothing here derives a
// script's transliteration from first principles. Closing this gap needs a
// phonetic model of the target script, out of scope here on the same
// judgement foldStringConcat's own comment records: a curated table cheap
// enough to trust is worth more than a broader attempt that risks false
// confidence. TestEveryLocaleIsClassifiedForScript is the maintenance
// backstop below, and it is what keeps this table from going stale
// unnoticed.
//
// The product-name entry has the same gap for the same reason: Devanagari
// admits more than one defensible rendering of "Dinah," none of which are
// grounded in anything a contributor has actually written, and the table
// stops at डिना rather than inventing further variants, for the reason
// dinah-73's own OQ-1 already gives for the retired word.
// TestEveryLocaleIsClassifiedForScript is the same maintenance backstop for
// this entry as for the retired word's.
//
// The boundary check carriesNonLatinSpelling applies (below) stops a
// shortWord embedded inside a longer, legitimate word, but it cannot tell
// a genuine standalone Hindi homograph of a shortWord from a real hit,
// since the two are indistinguishable by shape alone; none is known to
// exist, per dinah-83's own investigation into whether डिना collides with
// any real Hindi word or word fragment.
//
// The boundary check treats the Devanagari danda, double danda, and
// abbreviation sign as boundaries, so a name at the end of a Hindi sentence
// is caught, but it still treats a Devanagari digit glued directly against
// a match, with no separator, as still part of a word, so a shortWord
// immediately against a Devanagari numeral with nothing between them would
// not be flagged. This is judged acceptable because a digit is not how a
// Hindi sentence ends or pauses, and it is named here rather than fixed.

// noKnownTransliterationLocales classifies a non-Latin-script locale for
// which nobody has found a transliteration of the retired word worth
// cataloging. The map is the honest alternative to an entry with an empty
// shortWords slice: an empty slice is indistinguishable from an entry
// nobody finished, while a key here demands a reason. The value is a
// human-readable justification, checked only for non-emptiness; its quality
// is a reviewer's call, not a machine's, the same way a code comment's
// honesty is a reviewer's call. No locale needs an entry here today; the
// map exists so the next non-Latin locale that genuinely carries no known
// risk can say so instead of shipping a blank line.
var noKnownTransliterationLocales = map[string]string{}

// latinScriptLocales are the shipped locales theShortWord already reads
// correctly, so they need no entry in nonLatinRetiredSpellings or
// noKnownTransliterationLocales.
var latinScriptLocales = map[string]bool{
	"af": true, "cs": true, "de": true, "en": true,
	"es": true, "fil": true, "id": true,
}

// TestEveryLocaleIsClassifiedForScript asserts that every locale file under
// internal/msg/locales has been classified into exactly one of
// latinScriptLocales, nonLatinRetiredSpellings, and
// noKnownTransliterationLocales, and that the classification does work
// rather than standing in for one: a nonLatinRetiredSpellings entry must
// list at least one transliteration, and a noKnownTransliterationLocales
// entry must carry a justification that is not blank once trimmed. A locale
// file cannot ship, translated or still a skeleton, without somebody having
// decided, and recorded, whether its script carries transliteration risk.
func TestEveryLocaleIsClassifiedForScript(t *testing.T) {
	root := filepath.Join("..", "..", "internal", "msg", "locales")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read locales dir: %v", err)
	}
	found := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		found++
		tag := strings.TrimSuffix(entry.Name(), ".json")
		buckets := 0
		if latinScriptLocales[tag] {
			buckets++
		}
		if entries, ok := nonLatinRetiredSpellings[tag]; ok {
			buckets++
			if len(entries) == 0 {
				t.Errorf("%s: nonLatinRetiredSpellings entry lists no transliteration entries; belongs in noKnownTransliterationLocales with a reason instead", tag)
			}
			for i, entry := range entries {
				if len(entry.shortWords) == 0 {
					t.Errorf("%s: nonLatinRetiredSpellings entry %d (%s) lists no transliterations; belongs in noKnownTransliterationLocales with a reason instead", tag, i, entry.subject)
				}
			}
		}
		if justification, ok := noKnownTransliterationLocales[tag]; ok {
			buckets++
			if strings.TrimSpace(justification) == "" {
				t.Errorf("%s: noKnownTransliterationLocales entry carries no justification", tag)
			}
		}
		switch buckets {
		case 0:
			t.Errorf("%s is not classified in latinScriptLocales, nonLatinRetiredSpellings, or noKnownTransliterationLocales", tag)
		case 1:
			// classified exactly once, as required
		default:
			t.Errorf("%s is classified in more than one of latinScriptLocales, nonLatinRetiredSpellings, and noKnownTransliterationLocales", tag)
		}
	}
	if found == 0 {
		t.Error("no locale files were found, so this guard proves nothing")
	}
}

// TestTheRetiredWordHasNoNonLatinSpelling asserts that no catalog named in
// nonLatinRetiredSpellings carries a standalone transliteration of the
// retired word. For every line, every occurrence of the entry's longWord is
// stripped first, exactly as theLongWord is stripped from the Latin scan,
// and what remains is checked against the entry's shortWords.
func TestTheRetiredWordHasNoNonLatinSpelling(t *testing.T) {
	root := filepath.Join("..", "..", "internal", "msg", "locales")
	checked := 0
	for tag := range nonLatinRetiredSpellings {
		entry, ok := entryFor(tag, retiredWordSubject)
		if !ok {
			continue
		}
		path := filepath.Join(root, tag+".json")
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		for number, line := range strings.Split(string(source), "\n") {
			if held, hit := carriesNonLatinSpelling(line, entry); hit {
				t.Errorf("%s.json:%d carries the short word: %s\n%s", tag, number+1, held, vocabularyReason)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Error("no non-Latin catalogs were checked, so this guard proves nothing")
	}
}

// productNameReason is what a reader who trips this guard on the product
// name needs: why it is refused, distinct from vocabularyReason so a hit
// on Dinah is never reported as though it were a hit on the retired word.
const productNameReason = "the product name Dinah is never translated or " +
	"transliterated; it stays in Latin script in every locale, so rewrite " +
	"the text rather than respelling the name"

// TestTheProductNameHasNoNonLatinSpelling asserts that no catalog carries a
// standalone transliteration of the product name into a non-Latin script,
// the same absence check TestTheRetiredWordHasNoNonLatinSpelling runs for
// the retired word, against the productNameSubject entries of the same
// per-locale table. It is a separate function from
// TestTheRetiredWordHasNoNonLatinSpelling rather than a merged one, because
// the two report a different reason and a merged failure message would blur
// which word was caught.
func TestTheProductNameHasNoNonLatinSpelling(t *testing.T) {
	root := filepath.Join("..", "..", "internal", "msg", "locales")
	checked := 0
	for tag := range nonLatinRetiredSpellings {
		entry, ok := entryFor(tag, productNameSubject)
		if !ok {
			continue
		}
		path := filepath.Join(root, tag+".json")
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		for number, line := range strings.Split(string(source), "\n") {
			if held, hit := carriesNonLatinSpelling(line, entry); hit {
				t.Errorf("%s.json:%d carries a transliteration of the product name: %s\n%s", tag, number+1, held, productNameReason)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Error("no non-Latin catalogs were checked, so this guard proves nothing")
	}
}

// carriesNonLatinSpelling reports whether line, with entry.longWord
// stripped out first, still holds a standalone occurrence of one of
// entry.shortWords: not immediately preceded or followed by another
// Devanagari letter or combining mark. The boundary check stands in for
// the strip when entry.longWord cannot appear in the target script at
// all (the product-name entry's longWord is the Latin spelling "Dinah",
// inert against Devanagari text): it stops a shortWord that sits inside
// a longer, legitimate word the same way the strip stops one that sits
// inside entry.longWord itself. It cannot tell a genuine standalone
// Hindi homograph of a shortWord from a real hit, since the two are
// indistinguishable by shape alone; none is known to exist (see the
// collision-question section of dinah-83's spec).
func carriesNonLatinSpelling(line string, entry nonLatinEntry) (string, bool) {
	stripped := strings.ReplaceAll(line, entry.longWord, "")
	for _, short := range entry.shortWords {
		for start := 0; ; {
			i := strings.Index(stripped[start:], short)
			if i < 0 {
				break
			}
			matchStart := start + i
			matchEnd := matchStart + len(short)
			if !adjacentToDevanagari(stripped, matchStart, matchEnd) {
				return short, true
			}
			start = matchStart + 1
		}
	}
	return "", false
}

// devanagariPunctuation lists the code points in the Devanagari Unicode
// block (U+0900-U+097F) that Go's unicode package classifies as
// punctuation (General Category Po) rather than as a letter, a combining
// mark, or a digit: DEVANAGARI DANDA, DEVANAGARI DOUBLE DANDA, and
// DEVANAGARI ABBREVIATION SIGN. This list was produced by running every
// code point in the block through unicode.IsLetter, unicode.IsMark,
// unicode.IsNumber, and unicode.IsPunct and reading off the category each
// one actually falls in, not by reading the Unicode block chart. These
// three are the only punctuation code points the block contains.
var devanagariPunctuation = map[rune]bool{
	0x0964: true, // DEVANAGARI DANDA "।", the plain sentence-ending mark
	0x0965: true, // DEVANAGARI DOUBLE DANDA "॥", the verse/emphatic mark
	0x0970: true, // DEVANAGARI ABBREVIATION SIGN "॰"
}

// adjacentToDevanagari reports whether the rune immediately before start
// or immediately after end in s is a Devanagari letter or combining mark
// (in the Unicode block U+0900-U+097F but not in devanagariPunctuation),
// meaning the [start:end) match sits inside a longer run of Devanagari
// word text rather than standing alone. A combining mark (a vowel sign,
// virama, nukta, anusvara, visarga, or candrabindu) counts as
// non-boundary because it modifies the letter it attaches to and stays
// part of the same word; a Devanagari punctuation mark does not, because
// it ends or pauses a sentence the same way a comma, a line end, or a
// Latin letter already does.
func adjacentToDevanagari(s string, start, end int) bool {
	if start > 0 {
		if r, _ := utf8.DecodeLastRuneInString(s[:start]); r >= 0x0900 && r <= 0x097F && !devanagariPunctuation[r] {
			return true
		}
	}
	if end < len(s) {
		if r, _ := utf8.DecodeRuneInString(s[end:]); r >= 0x0900 && r <= 0x097F && !devanagariPunctuation[r] {
			return true
		}
	}
	return false
}

// TestGuardCatchesANonLatinRetiredWord reproduces this card's own gap: a
// standalone Devanagari transliteration of the retired word with no
// surrounding legitimate spelling, in a synthetic file the production scan
// never walks. The fixture lives under t.TempDir() and is removed with the
// test, so the reproduction string never reaches a file the production scan
// would itself trip on.
func TestGuardCatchesANonLatinRetiredWord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "planted.json")
	source := `{"planted": "यह एक बेंच है"}` + "\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	planted, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	entry, ok := entryFor("hi", retiredWordSubject)
	if !ok {
		t.Fatal("hi carries no retiredWordSubject entry to test against")
	}
	tripped := false
	for _, line := range strings.Split(string(planted), "\n") {
		if _, hit := carriesNonLatinSpelling(line, entry); hit {
			tripped = true
		}
	}
	if !tripped {
		t.Error("the planted non-Latin retired-word fixture did not trip the guard")
	}
}

// TestGuardCatchesANonLatinProductName reproduces this card's own gap: a
// standalone Devanagari transliteration of the product name with no Latin
// spelling anywhere on the line, in a synthetic file the production scan
// never walks. The fixture lives under t.TempDir() and is removed with the
// test, so the reproduction string never reaches a file the production scan
// would itself trip on.
//
// It also plants two boundary cases on the same fixture: a डिना genuinely
// embedded inside a longer Devanagari word (AC-8), which must not trip the
// guard, and a डिना adjacent to a danda on each side with no space (AC-9),
// which must trip the guard even though the earlier, pre-D-8 boundary rule
// would have missed it, alongside a डिना embedded beside a danda with no
// other separator, which must not trip the guard.
func TestGuardCatchesANonLatinProductName(t *testing.T) {
	dir := t.TempDir()
	entry, ok := entryFor("hi", productNameSubject)
	if !ok {
		t.Fatal("hi carries no productNameSubject entry to test against")
	}

	t.Run("standalone occurrence trips the guard", func(t *testing.T) {
		path := filepath.Join(dir, "standalone.json")
		source := `{"planted": "यह डिना का काम है"}` + "\n"
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		planted, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		tripped := false
		for _, line := range strings.Split(string(planted), "\n") {
			if _, hit := carriesNonLatinSpelling(line, entry); hit {
				tripped = true
			}
		}
		if !tripped {
			t.Error("the planted non-Latin product-name fixture did not trip the guard")
		}
	})

	t.Run("embedded occurrence does not trip the guard, standalone elsewhere on the line still does (AC-8)", func(t *testing.T) {
		path := filepath.Join(dir, "embedded.json")
		// डिनाबिंदुडिनाकर embeds डिना twice inside a longer synthetic
		// Devanagari run with no boundary on either side; the separately
		// spaced डिना later on the line is the standalone control.
		source := `{"planted": "डिनाबिंदुडिनाकर और यह भी डिना है"}` + "\n"
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		planted, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		tripped := false
		for _, line := range strings.Split(string(planted), "\n") {
			if held, hit := carriesNonLatinSpelling(line, entry); hit {
				tripped = true
				if held != "डिना" {
					t.Errorf("unexpected match text %q", held)
				}
			}
		}
		if !tripped {
			t.Error("the standalone occurrence on the embedded-run line did not trip the guard")
		}
	})

	t.Run("standalone occurrences beside a danda trip the guard, embedded-beside-a-danda does not (AC-9, D-8)", func(t *testing.T) {
		path := filepath.Join(dir, "danda.json")
		source := `{"planted": "यह डिना।"}` + "\n"
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		planted, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		tripped := false
		for _, line := range strings.Split(string(planted), "\n") {
			if _, hit := carriesNonLatinSpelling(line, entry); hit {
				tripped = true
			}
		}
		if !tripped {
			t.Error("the standalone product-name occurrence immediately before a danda did not trip the guard")
		}

		precedingPath := filepath.Join(dir, "danda-preceding.json")
		precedingSource := `{"planted": "।डिना यह है"}` + "\n"
		if err := os.WriteFile(precedingPath, []byte(precedingSource), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		precedingPlanted, err := os.ReadFile(precedingPath)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		precedingTripped := false
		for _, line := range strings.Split(string(precedingPlanted), "\n") {
			if _, hit := carriesNonLatinSpelling(line, entry); hit {
				precedingTripped = true
			}
		}
		if !precedingTripped {
			t.Error("the standalone product-name occurrence immediately after a danda did not trip the guard")
		}

		embeddedPath := filepath.Join(dir, "danda-embedded.json")
		// मधुडिनाकर embeds डिना inside a longer word immediately before a
		// danda with no other separator; it must not be reported as a hit.
		embeddedSource := `{"planted": "मधुडिनाकर।"}` + "\n"
		if err := os.WriteFile(embeddedPath, []byte(embeddedSource), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		embeddedPlanted, err := os.ReadFile(embeddedPath)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		for _, line := range strings.Split(string(embeddedPlanted), "\n") {
			if held, hit := carriesNonLatinSpelling(line, entry); hit {
				t.Errorf("the embedded-beside-a-danda occurrence tripped the guard: %s", held)
			}
		}
	})
}

// TestFoldStringConcat asserts what the fold catches and what it declines,
// against expressions built with go/parser.ParseExpr so each case exercises
// the same AST shapes the guard walks rather than a hand-built stand-in.
func TestFoldStringConcat(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
		ok   bool
	}{
		{name: "two literals", expr: `"foo" + "bar"`, want: "foobar", ok: true},
		{name: "three literals nested left to right", expr: `"foo" + "bar" + "baz"`, want: "foobarbaz", ok: true},
		{name: "parenthesised sub-chain", expr: `"foo" + ("bar" + "baz")`, want: "foobarbaz", ok: true},
		{name: "one non-literal operand", expr: `"--" + name`, ok: false},
		{name: "bare single literal", expr: `"foo"`, want: "foo", ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := parser.ParseExpr(tt.expr)
			if err != nil {
				t.Fatalf("ParseExpr(%q): %v", tt.expr, err)
			}
			got, ok := foldStringConcat(expr)
			if ok != tt.ok {
				t.Fatalf("foldStringConcat(%q) ok = %v, want %v", tt.expr, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("foldStringConcat(%q) = %q, want %q", tt.expr, got, tt.want)
			}
		})
	}
}

// TestGuardCatchesAConcatenatedRetiredWord reproduces the card's own gap: a
// retired-word string assembled from more than one string literal joined
// with +, in a synthetic file the production guard never walks. The fixture
// lives under t.TempDir() and is removed with the test, so the reproduction
// string never reaches a file the production scan would trip on itself.
func TestGuardCatchesAConcatenatedRetiredWord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "planted.go")
	source := "package planted\n\nconst planted = \"the \" + \"b\" + \"ench is back\"\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	found, err := goStringLiterals(path)
	if err != nil {
		t.Fatalf("goStringLiterals: %v", err)
	}
	tripped := false
	for _, held := range found {
		if carriesTheShortWord("planted.go", held.text, "") {
			tripped = true
		}
	}
	if !tripped {
		t.Error("the concatenated retired-word fixture did not trip the guard")
	}
}

// literal is one Go string literal with the line it sits on.
type literal struct {
	// line is the one-based line the literal starts on.
	line int
	// text is the literal's source spelling, quotes included.
	text string
}

// goStringLiterals returns every string literal of a Go file, folding any
// chain of literals joined end to end with + within one expression into a
// single reported value. The parser is what separates a literal from a
// comment or an identifier, which a regular expression over the source
// could not do.
func goStringLiterals(path string) ([]literal, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var found []literal
	ast.Inspect(file, func(node ast.Node) bool {
		if binary, ok := node.(*ast.BinaryExpr); ok {
			if value, ok := foldStringConcat(binary); ok {
				found = append(found, literal{
					line: fset.Position(binary.Pos()).Line,
					text: strconv.Quote(value),
				})
				return false // the fold already covers every operand below this node
			}
			return true // some operand isn't a literal; let Inspect still find any pure-literal sub-chain or bare literal beneath
		}
		held, ok := node.(*ast.BasicLit)
		if !ok || held.Kind != token.STRING {
			return true
		}
		found = append(found, literal{line: fset.Position(held.Pos()).Line, text: held.Value})
		return true
	})
	return found, nil
}

// foldStringConcat evaluates expr as a compile-time string constant built
// from string literals joined with +, and reports whether it succeeded.
// A parenthesised sub-expression is unwrapped before folding. Any operand
// that is not itself a string literal or a further +-chain of them, an
// identifier, a call, a conversion, anything reading a value at runtime,
// stops the fold and returns false: the guard does not try to resolve what
// that operand holds.
func foldStringConcat(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return value, true
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		left, ok := foldStringConcat(e.X)
		if !ok {
			return "", false
		}
		right, ok := foldStringConcat(e.Y)
		if !ok {
			return "", false
		}
		return left + right, true
	case *ast.ParenExpr:
		return foldStringConcat(e.X)
	default:
		return "", false
	}
}

// theOneRenderer is the file every columnar row in this codebase is laid out
// in. Padding a field, breaking a row whose field outruns its column, and
// measuring how many columns a string draws all live there, and this guard is
// what keeps them there.
const theOneRenderer = "cmd/dinah/row.go"

// theRenderingHead is the package whose non-test sources are read for a row
// built out of bare spaces. Tree-wide that pattern is unusable, since several
// packages carry legitimate multi-space literals, and inside this package the
// conversion leaves exactly one, which the guard exempts by its shape in the
// AST rather than by its line number.
const theRenderingHead = "cmd/dinah/"

// columnarCatalogKeys are the two catalog entries that compose columns inside
// a translated string, where the translator owns the spacing and no column is
// declared. Both are present in every shipped catalog, and the guard asserts
// that each still matches something, so a retired entry cannot leave a stale
// name covering another one.
var columnarCatalogKeys = []string{"card.line", "status.workbench"}

// rowLayoutReason is what a reader who trips this guard needs: why the shape
// is refused here, and what to do instead of working around it.
const rowLayoutReason = "every columnar line is built as a row and laid out by " +
	"formatRow in " + theOneRenderer + "; padding a field anywhere else counts " +
	"characters where a terminal counts columns, and the row drifts the moment " +
	"the field carries text from a script that disagrees with a rune count"

// printfWidthVerb matches a printf verb carrying a width and taking a string,
// which is how a Go programmer pads a column with no import at all. The verb
// set stops at s, v, q and x, so a width on a number, such as the one in a
// zero-padded timestamp, is left alone.
var printfWidthVerb = regexp.MustCompile(`%[-+ #0]*(\[[0-9]+\])?([0-9]+|\*)(\.[0-9]+)?[svqx]`)

// spacedRun matches a run of two or more spaces, which is a row laid out by
// hand once it sits inside a Go string literal in the rendering head.
var spacedRun = regexp.MustCompile(`  +`)

// rowLayoutPattern names one shape a row laid out by hand takes. The numbers
// are the ones the spec and TestGuardCatchesAHandRolledRow both count in, so a
// failure names a pattern a reader can look up.
type rowLayoutPattern int

const (
	patternTabwriter rowLayoutPattern = iota + 1
	patternStringsRepeat
	patternPrintfWidth
	patternRuneCount
	patternSpacedLiteral
	patternColumnarCatalogEntry
	patternPadCall
)

// rowLayoutFinding is one place a row is laid out outside the one renderer.
type rowLayoutFinding struct {
	// where names the file and, for a Go source, the line.
	where string
	// pattern is which of the seven shapes was found.
	pattern rowLayoutPattern
	// detail is the text that tripped it.
	detail string
}

// TestNoRowIsLaidOutOutsideTheOneRenderer asserts that nothing in the shipped
// binary lays a columnar row out except formatRow in cmd/dinah/row.go, and
// that no message catalog composes columns inside an entry except the two
// entries meant to.
//
// The guard parses each Go file and asks the AST, which is how it tells a call
// from a comment and a literal from an identifier. Four of the seven patterns
// are AST shapes rather than text: an import, two callees, and a length over a
// conversion to runes.
//
// Test sources are outside the scan. A test that builds a row by hand is
// standing in for output another version of the code produced, and no such
// line reaches a person reading the tool.
func TestNoRowIsLaidOutOutsideTheOneRenderer(t *testing.T) {
	root := filepath.Join("..", "..")
	scanned := 0
	scannedRenderingHead := 0
	var findings []rowLayoutFinding
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skippedTrees[entry.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		name := filepath.ToSlash(relative)
		if name == theOneRenderer {
			return nil
		}
		found, scanErr := scanGoForRowLayout(path, name)
		if scanErr != nil {
			return scanErr
		}
		scanned++
		if strings.HasPrefix(name, theRenderingHead) {
			scannedRenderingHead++
		}
		findings = append(findings, found...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if scanned == 0 {
		t.Error("no source was scanned, so this guard proves nothing")
	}
	// scanned == 0 would still hold from internal/ alone if the walk ever
	// stopped reaching cmd/dinah, which is the one directory this guard was
	// filed to watch. Count what is actually on disk there and require the
	// walk to have matched it, the same shape the catalog check below uses
	// for its own directory, so a change that quietly excludes the rendering
	// head fails here rather than passing on unrelated coverage.
	wantRenderingHead := 0
	renderingHeadDir := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(theRenderingHead, "/")))
	renderingHeadEntries, err := os.ReadDir(renderingHeadDir)
	if err != nil {
		t.Fatalf("read %s: %v", renderingHeadDir, err)
	}
	for _, entry := range renderingHeadEntries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		if theRenderingHead+entry.Name() == theOneRenderer {
			continue
		}
		wantRenderingHead++
	}
	if scannedRenderingHead == 0 || scannedRenderingHead != wantRenderingHead {
		t.Errorf("the walk scanned %d of the %d non-test sources under %s, so this guard proves less than it claims about the rendering head it exists to watch", scannedRenderingHead, wantRenderingHead, theRenderingHead)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(theOneRenderer))); err != nil {
		t.Errorf("the one renderer is not where this guard exempts it: %v", err)
	}

	catalogs := filepath.Join(root, "internal", "msg", "locales")
	onDisk, err := os.ReadDir(catalogs)
	if err != nil {
		t.Fatalf("read locales dir: %v", err)
	}
	wanted := 0
	for _, entry := range onDisk {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			wanted++
		}
	}
	read, matched, catalogFindings, err := scanCatalogsForRowLayout(catalogs)
	if err != nil {
		t.Fatalf("scan catalogs: %v", err)
	}
	findings = append(findings, catalogFindings...)
	if read == 0 || read != wanted {
		t.Errorf("the catalog walk read %d of the %d catalogs on disk, so this guard proves less than it claims", read, wanted)
	}
	for _, key := range columnarCatalogKeys {
		if matched[key] == 0 {
			t.Errorf("the exemption names %s and no catalog entry matches it, so the name covers nothing and may be covering something else; drop it or correct the key", key)
		}
	}

	for _, finding := range findings {
		t.Errorf("%s lays out a row by hand (pattern %d): %s\n%s", finding.where, finding.pattern, finding.detail, rowLayoutReason)
	}
}

// scanGoForRowLayout reports every finding of the six patterns a Go source can
// carry in one file. name is the file's slash-separated path from the
// repository root, which is what decides whether the patterns confined to the
// rendering head apply to it.
func scanGoForRowLayout(path, name string) ([]rowLayoutFinding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	at := func(pos token.Pos) string {
		return name + ":" + strconv.Itoa(fset.Position(pos).Line)
	}
	var findings []rowLayoutFinding

	for _, imported := range file.Imports {
		if imported.Path.Value == strconv.Quote("text/tabwriter") {
			findings = append(findings, rowLayoutFinding{
				where:   at(imported.Pos()),
				pattern: patternTabwriter,
				detail:  "imports text/tabwriter, whose column writer measures bytes",
			})
		}
	}

	indents := map[token.Pos]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 3 || !isSelectorCall(call.Fun, "json", "MarshalIndent") {
			return true
		}
		if literal, ok := call.Args[2].(*ast.BasicLit); ok {
			indents[literal.Pos()] = true
		}
		return true
	})

	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch {
		case isSelectorCall(call.Fun, "strings", "Repeat"):
			findings = append(findings, rowLayoutFinding{
				where:   at(call.Pos()),
				pattern: patternStringsRepeat,
				detail:  "calls strings.Repeat, which is how a run of padding is built",
			})
		case isSelectorCall(call.Fun, "utf8", "RuneCountInString"), isSelectorCall(call.Fun, "utf8", "RuneCount"):
			findings = append(findings, rowLayoutFinding{
				where:   at(call.Pos()),
				pattern: patternRuneCount,
				detail:  "counts runes, which is not what a terminal counts",
			})
		case isBareIdent(call.Fun, "len") && len(call.Args) == 1 && isRuneConversion(call.Args[0]):
			findings = append(findings, rowLayoutFinding{
				where:   at(call.Pos()),
				pattern: patternRuneCount,
				detail:  "takes the length of a conversion to runes, which is not what a terminal counts",
			})
		case isBareIdent(call.Fun, "pad"):
			findings = append(findings, rowLayoutFinding{
				where:   at(call.Pos()),
				pattern: patternPadCall,
				detail:  "calls pad, which widens a field to a column outside the one renderer",
			})
		}
		return true
	})

	held, err := goStringLiterals(path)
	if err != nil {
		return nil, err
	}
	for _, literal := range held {
		if printfWidthVerb.MatchString(literal.text) {
			findings = append(findings, rowLayoutFinding{
				where:   name + ":" + strconv.Itoa(literal.line),
				pattern: patternPrintfWidth,
				detail:  "carries a printf width on a string verb: " + literal.text,
			})
		}
	}

	if !strings.HasPrefix(name, theRenderingHead) {
		return findings, nil
	}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING || indents[literal.Pos()] {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		if spacedRun.MatchString(value) {
			findings = append(findings, rowLayoutFinding{
				where:   at(literal.Pos()),
				pattern: patternSpacedLiteral,
				detail:  "carries a run of spaces wide enough to be a column: " + literal.Value,
			})
		}
		return true
	})
	return findings, nil
}

// isSelectorCall reports whether expr names pkg.name.
func isSelectorCall(expr ast.Expr, pkg, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	return isBareIdent(selector.X, pkg)
}

// isBareIdent reports whether expr is the identifier name on its own.
func isBareIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

// isRuneConversion reports whether expr converts something to a slice of
// runes.
func isRuneConversion(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	array, ok := call.Fun.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return false
	}
	return isBareIdent(array.Elt, "rune")
}

// scanCatalogsForRowLayout reads every catalog in a directory and reports each
// entry composing columns inside its own text, which is a row no scan of Go
// sources can see. It returns how many catalogs it read, how many entries each
// exempted key matched, and the findings.
func scanCatalogsForRowLayout(dir string) (int, map[string]int, []rowLayoutFinding, error) {
	onDisk, err := os.ReadDir(dir)
	if err != nil {
		return 0, nil, nil, err
	}
	exempt := map[string]bool{}
	for _, key := range columnarCatalogKeys {
		exempt[key] = true
	}
	matched := map[string]int{}
	read := 0
	var findings []rowLayoutFinding
	for _, entry := range onDisk {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		source, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return read, matched, findings, err
		}
		var catalog struct {
			Entries map[string]struct {
				Text string `json:"text"`
			} `json:"entries"`
		}
		if err := json.Unmarshal(source, &catalog); err != nil {
			return read, matched, findings, err
		}
		read++
		for _, key := range sortedCatalogKeys(catalog.Entries) {
			text := catalog.Entries[key].Text
			if !composesColumns(text) {
				continue
			}
			if exempt[key] {
				matched[key]++
				continue
			}
			findings = append(findings, rowLayoutFinding{
				where:   entry.Name() + " " + key,
				pattern: patternColumnarCatalogEntry,
				detail:  "composes columns inside its own text: " + strconv.Quote(text),
			})
		}
	}
	return read, matched, findings, nil
}

// sortedCatalogKeys returns a catalog's keys in order, so two runs report the
// same findings in the same order.
func sortedCatalogKeys[T any](entries map[string]T) []string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// composesColumns reports whether text lays out columns: a run of space
// characters separating two non-space characters and measuring two or more
// columns on screen.
//
// The run is measured in columns rather than counted in characters, which is
// what catches a single IDEOGRAPHIC SPACE, the one a translator working in a
// CJK locale reaches for by habit and the one an ASCII-space pattern reads as
// ordinary text. A pair of no-break spaces is caught the same way. A run at
// the start or the end of an entry indents or trails it rather than separating
// two fields, so the run has to sit between two non-space characters.
func composesColumns(text string) bool {
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if !unicode.Is(unicode.Zs, runes[i]) {
			continue
		}
		start := i
		for i < len(runes) && unicode.Is(unicode.Zs, runes[i]) {
			i++
		}
		if start == 0 || i >= len(runes) {
			continue
		}
		if textwidth.Columns(string(runes[start:i])) >= 2 {
			return true
		}
	}
	return false
}

// handRolledFixtures are one planted row per pattern the Go scan carries, each
// written the way somebody would write it who had not read the renderer.
// Removing any one pattern from the guard makes the fixture beside it go
// unreported, which is what TestGuardCatchesAHandRolledRow fails on.
//
// Fixture 7 is the one a six-pattern guard passed. Its indent is a single
// space, so it carries no multi-space literal, no repeat, no printf width, and
// no rune count, and nothing but the callee itself sees it.
var handRolledFixtures = []struct {
	// pattern is the shape this fixture is planted to trip.
	pattern rowLayoutPattern
	// source is a whole Go file.
	source string
}{
	{
		pattern: patternTabwriter,
		source:  "package planted\n\nimport \"text/tabwriter\"\n\nvar writer *tabwriter.Writer\n",
	},
	{
		pattern: patternStringsRepeat,
		source:  "package planted\n\nimport \"strings\"\n\nconst gap = \" \"\n\nfunc padded(text string, n int) string { return text + strings.Repeat(gap, n) }\n",
	},
	{
		pattern: patternPrintfWidth,
		source:  "package planted\n\nimport \"fmt\"\n\nfunc padded(ref string) string { return fmt.Sprintf(\"%-14s\", ref) }\n",
	},
	{
		pattern: patternRuneCount,
		source:  "package planted\n\nfunc drawn(text string) int { return len([]rune(text)) }\n",
	},
	{
		pattern: patternSpacedLiteral,
		source:  "package planted\n\nfunc header(ts, author string) string { return \"  \" + ts + \"  \" + author }\n",
	},
	{
		pattern: patternPadCall,
		source:  "package planted\n\nfunc pad(text string, width int) string { return text }\n\nfunc held(ref, title string) string { return \" \" + pad(ref, 14) + title }\n",
	},
}

// TestGuardCatchesAHandRolledRow plants one row per pattern and asserts each is
// reported, so that removing a pattern from the guard fails this test rather
// than quietly narrowing what the guard sees. It plants the two shapes the
// guard has to let through as well, since a guard that reports everything
// protects nothing either.
//
// Every fixture lives under t.TempDir() and is removed with the test, so no
// planted row reaches a file the production scan walks.
func TestGuardCatchesAHandRolledRow(t *testing.T) {
	dir := t.TempDir()
	for _, fixture := range handRolledFixtures {
		name := "pattern" + strconv.Itoa(int(fixture.pattern))
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".go")
			if err := os.WriteFile(path, []byte(fixture.source), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			found, err := scanGoForRowLayout(path, theRenderingHead+name+".go")
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			for _, finding := range found {
				if finding.pattern == fixture.pattern {
					return
				}
			}
			t.Errorf("the planted row was not reported by pattern %d; the guard reported %v", fixture.pattern, found)
		})
	}

	t.Run("the indent of a machine form is not a column", func(t *testing.T) {
		path := filepath.Join(dir, "marshal.go")
		source := "package planted\n\nimport \"encoding/json\"\n\nfunc form(value any) ([]byte, error) { return json.MarshalIndent(value, \"\", \"  \") }\n"
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		found, err := scanGoForRowLayout(path, theRenderingHead+"marshal.go")
		if err != nil {
			t.Fatalf("scan fixture: %v", err)
		}
		if len(found) != 0 {
			t.Errorf("the indent argument of a machine form was reported as a column: %v", found)
		}
	})

	t.Run("a run of spaces outside the rendering head is not a column", func(t *testing.T) {
		path := filepath.Join(dir, "elsewhere.go")
		source := "package planted\n\nconst prompt = \"a  b\"\n"
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		found, err := scanGoForRowLayout(path, "internal/planted/elsewhere.go")
		if err != nil {
			t.Fatalf("scan fixture: %v", err)
		}
		if len(found) != 0 {
			t.Errorf("a multi-space literal outside the rendering head was reported: %v", found)
		}
	})
}

// TestGuardCatchesAColumnarCatalogEntry plants a catalog carrying the three
// shapes a translator lays columns out with and asserts each is reported. The
// ideographic space and the pair of no-break spaces are the two an ASCII-space
// pattern reads as ordinary text, and a translator working in a CJK locale
// reaches for the first of them by habit.
//
// The same planted catalog carries neither exempted key, so the run also
// asserts that an exemption matching nothing is reported rather than passing
// as though it had covered something.
func TestGuardCatchesAColumnarCatalogEntry(t *testing.T) {
	dir := t.TempDir()
	planted := `{"tag":"xx","entries":{` +
		`"planted.ascii":{"text":"{a}  {b}"},` +
		`"planted.ideographic":{"text":"{a}　{b}"},` +
		`"planted.nobreak":{"text":"{a}  {b}"},` +
		`"planted.indented":{"text":"  {a} {b}"},` +
		`"planted.plain":{"text":"{a} {b}"}}}`
	if err := os.WriteFile(filepath.Join(dir, "xx.json"), []byte(planted), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	read, matched, findings, err := scanCatalogsForRowLayout(dir)
	if err != nil {
		t.Fatalf("scan catalogs: %v", err)
	}
	if read != 1 {
		t.Fatalf("read %d catalogs, want 1", read)
	}
	reported := map[string]bool{}
	for _, finding := range findings {
		reported[finding.where] = true
	}
	for _, key := range []string{"planted.ascii", "planted.ideographic", "planted.nobreak"} {
		if !reported["xx.json "+key] {
			t.Errorf("the planted columnar entry %s was not reported; the guard reported %v", key, findings)
		}
	}
	for _, key := range []string{"planted.indented", "planted.plain"} {
		if reported["xx.json "+key] {
			t.Errorf("%s lays out no columns and was reported anyway", key)
		}
	}
	for _, key := range columnarCatalogKeys {
		if matched[key] != 0 {
			t.Errorf("the exemption for %s matched %d entries in a catalog that carries none", key, matched[key])
		}
	}
}
