package profile

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
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

// stubbedPlatform returns scripts/install.sh with a prologue spliced in after
// its shebang line: a shell function answering the platform questions the
// script asks, and, when onPath is true, a line putting the install directory
// on PATH before the script reads it. Nothing else in the script is touched.
//
// The stub is a shell function rather than an executable named uname planted
// in a directory prepended to PATH. A function outranks an external command of
// the same name in every POSIX shell, so it is reached however the shell works
// out PATH; a planted directory is reached only if the shell keeps the PATH it
// was handed, and Git for Windows' bin\sh.exe does not. That sh.exe is a
// wrapper which rewrites PATH to put /mingw64/bin and /usr/bin first, so the
// real uname shadowed the planted stub, answered MINGW64_NT-10.0-26100, and
// every shell-script test here failed on the Windows CI leg while passing on
// Linux and macOS. Which sh.exe answers is a property of the machine's PATH,
// not of anything this test can see, so the fix is to stop depending on it.
//
// The PATH entry is spliced in for the same reason, and for a second one: the
// script asks whether $HOME/.local/bin is on PATH, and only the shell knows
// what it calls that directory. A Go-built entry, joined with filepath.Join
// and separated with os.PathListSeparator, is a Windows path in a
// semicolon-separated list, which is not what the script compares against.
func stubbedPlatform(t *testing.T, script, systemName, machineName string, onPath bool) string {
	t.Helper()
	const shebang = "#!/bin/sh\n"
	if !strings.HasPrefix(script, shebang) {
		t.Fatal("scripts/install.sh no longer opens with a #!/bin/sh line, so the test prologue has nowhere to go")
	}
	var prologue strings.Builder
	fmt.Fprintf(&prologue, "uname() {\n\tcase \"$1\" in\n\t-s) echo %s ;;\n\t-m) echo %s ;;\n\t*) echo %s ;;\n\tesac\n}\n", systemName, machineName, systemName)
	if onPath {
		prologue.WriteString("PATH=\"$HOME/.local/bin:$PATH\"\nexport PATH\n")
	}
	return shebang + prologue.String() + strings.TrimPrefix(script, shebang)
}

// windowsPowerShellEnv returns the environment a powershell.exe child is run
// with: this process's own environment less PSModulePath, plus extra.
//
// Get-FileHash is a function of the Microsoft.PowerShell.Utility module rather
// than one of the cmdlets Windows PowerShell's default session already holds,
// so Windows PowerShell reaches it by autoloading that module off
// PSModulePath. A windows-latest step on GitHub Actions runs in PowerShell 7,
// whose PSModulePath names PowerShell 7's own module directory ahead of
// Windows PowerShell's; handed that value, Windows PowerShell autoloads
// PowerShell 7's Microsoft.PowerShell.Utility, which carries no Get-FileHash
// for it, and the install script died on CommandNotFoundException at the
// checksum step. Dropping the variable leaves Windows PowerShell to work out
// its own module path, which is what it does when the variable is absent.
//
// Every cmdlet the install script uses besides Get-FileHash is part of the
// default session and never needed the module, which is why the script got as
// far as downloading and length-checking before it failed.
func windowsPowerShellEnv(extra ...string) []string {
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+len(extra))
	for _, entry := range inherited {
		if name, _, ok := strings.Cut(entry, "="); ok && strings.EqualFold(name, "PSModulePath") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, extra...)
}

// releaseBinaries names every binary .github/workflows/release.yml builds, in
// the order its matrix declares them.
var releaseBinaries = []string{
	"dinah-windows-amd64.exe",
	"dinah-windows-arm64.exe",
	"dinah-linux-amd64",
	"dinah-linux-arm64",
	"dinah-darwin-amd64",
	"dinah-darwin-arm64",
}

// publishedRelease is a stand-in for a dev release: the six binaries, the
// SHA256SUMS.txt written over them, and the channel manifest, all assembled the
// way release.yml assembles them.
type publishedRelease struct {
	binaries map[string][]byte
	sums     []byte
	manifest []byte
}

// buildPublishedRelease reproduces release.yml's "Write the checksums and the
// channel manifest" step. That step writes the manifest as compact one-line
// entries and then reformats the whole document through a JSON pretty-printer,
// which puts each binary's name and its sha256 on separate lines. A consumer
// that reads the checksum with a line-scoped match cannot see both at once, so
// a fixture written by hand in the compact shape tests something the workflow
// never publishes.
func buildPublishedRelease(downloadBase string) (publishedRelease, error) {
	release := publishedRelease{binaries: map[string][]byte{}}
	var sums bytes.Buffer
	var compact bytes.Buffer
	compact.WriteString("{\n")
	fmt.Fprintf(&compact, "  %q: %q,\n", "channel", "dev")
	fmt.Fprintf(&compact, "  %q: %q,\n", "version", "v0.1.0-dev.7")
	fmt.Fprintf(&compact, "  %q: %q,\n", "tag", "v0.1.0-dev.7")
	fmt.Fprintf(&compact, "  %q: %q,\n", "publishedAt", "2026-01-01T00:00:00Z")
	fmt.Fprintf(&compact, "  %q: %q,\n", "downloadBase", downloadBase)
	compact.WriteString("  \"binaries\": {\n")
	for i, name := range releaseBinaries {
		content := []byte("stand-in for " + name + "\n")
		release.binaries[name] = content
		sum := fmt.Sprintf("%x", sha256.Sum256(content))
		fmt.Fprintf(&sums, "%s  %s\n", sum, name)
		if i > 0 {
			compact.WriteString(",\n")
		}
		fmt.Fprintf(&compact, "    %q: { %q: %q, %q: %d }", name, "sha256", sum, "size", len(content))
	}
	compact.WriteString("\n  }\n}\n")

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, compact.Bytes(), "", "    "); err != nil {
		return publishedRelease{}, err
	}
	release.sums = sums.Bytes()
	release.manifest = pretty.Bytes()
	return release, nil
}

// TestInstallScriptReadsWhatTheWorkflowPublishes runs scripts/install.sh
// against a stand-in release assembled by release.yml's own steps, and asserts
// it installs a verified binary.
//
// The script reads the download location out of the manifest and the checksum
// out of SHA256SUMS.txt, which sha256sum writes one line per binary. Reading
// the checksum out of the manifest's JSON instead made the install depend on
// how the manifest happened to be laid out, and every Linux and macOS install
// failed the first time the layout changed. The fixture below is therefore
// built by the publisher's own steps rather than written by hand, and the test
// asserts the manifest really did come out in the expanded shape, so it cannot
// quietly revert to a shape no release carries.
func TestInstallScriptReadsWhatTheWorkflowPublishes(t *testing.T) {
	for _, tool := range []string{"sh", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("this machine has no %s, which scripts/install.sh needs", tool)
		}
	}
	if _, shaErr := exec.LookPath("sha256sum"); shaErr != nil {
		if _, sumErr := exec.LookPath("shasum"); sumErr != nil {
			t.Skip("this machine has neither sha256sum nor shasum, which scripts/install.sh needs")
		}
	}

	const wanted = "dinah-linux-amd64"
	var release publishedRelease
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/channels/dev.json"):
			w.Write(release.manifest)
		case strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt"):
			w.Write(release.sums)
		default:
			name := path.Base(r.URL.Path)
			content, ok := release.binaries[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(content)
		}
	}))
	defer server.Close()

	built, err := buildPublishedRelease(server.URL + "/releases/download/v0.1.0-dev.7/")
	if err != nil {
		t.Fatalf("assembling the stand-in release: %v", err)
	}
	release = built

	// The fixture has to carry the defect's precondition, or it proves nothing.
	for _, line := range strings.Split(string(release.manifest), "\n") {
		if strings.Contains(line, `"`+wanted+`"`) && strings.Contains(line, "sha256") {
			t.Fatalf("the fixture manifest puts %s and its sha256 on one line, which is not the shape release.yml publishes: %q", wanted, line)
		}
	}

	root := filepath.Join("..", "..")
	source, err := os.ReadFile(filepath.Join(root, "scripts", "install.sh"))
	if err != nil {
		t.Fatalf("reading scripts/install.sh: %v", err)
	}
	// Only the origin is rewritten. Everything the script does with what it
	// fetches is the shipped code.
	script := strings.ReplaceAll(string(source), "https://github.com/paulmooreparks/dinah", server.URL)
	if !strings.Contains(script, server.URL) {
		t.Fatal("the test server URL did not reach the script under test")
	}
	// The script asks uname which binary this machine needs. The stub answers
	// for a linux/amd64 machine, so the test asserts the same thing on every
	// platform it runs on.
	script = stubbedPlatform(t, script, "Linux", "x86_64", false)

	home := t.TempDir()
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "install.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("writing the script under test: %v", err)
	}

	command := exec.Command("sh", scriptPath)
	command.Env = append(os.Environ(), "HOME="+home)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("scripts/install.sh failed against what release.yml publishes: %v\n%s", err, output)
	}

	installed, err := os.ReadFile(filepath.Join(home, ".local", "bin", "dinah"))
	if err != nil {
		t.Fatalf("no binary was installed: %v\n%s", err, output)
	}
	if !bytes.Equal(installed, release.binaries[wanted]) {
		t.Errorf("installed binary is not the published %s\ngot:  %q\nwant: %q", wanted, installed, release.binaries[wanted])
	}
}

// TestInstallScriptSaysWhetherDinahIsReadyToRun runs scripts/install.sh to a
// successful finish under four combinations of platform and PATH state, and
// asserts the final message matches what is actually true of that run: ready
// to run when the install directory is already on PATH, and the accurate,
// platform-specific explanation when it is not. Debian and Ubuntu add
// ~/.local/bin to PATH from the login profile only once the directory
// exists, so a first install is not picked up by the session that just
// created it; macOS never adds it. A script that prints the same advice
// regardless leaves a colleague on either path concluding the install
// failed.
func TestInstallScriptSaysWhetherDinahIsReadyToRun(t *testing.T) {
	for _, tool := range []string{"sh", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("this machine has no %s, which scripts/install.sh needs", tool)
		}
	}
	if _, shaErr := exec.LookPath("sha256sum"); shaErr != nil {
		if _, sumErr := exec.LookPath("shasum"); sumErr != nil {
			t.Skip("this machine has neither sha256sum nor shasum, which scripts/install.sh needs")
		}
	}

	root := filepath.Join("..", "..")
	source, err := os.ReadFile(filepath.Join(root, "scripts", "install.sh"))
	if err != nil {
		t.Fatalf("reading scripts/install.sh: %v", err)
	}

	cases := []struct {
		name        string
		unameS      string // what the uname -s stub reports
		unameM      string // what the uname -m stub reports
		binary      string
		onPath      bool
		wantSubstrs []string
		mustNotHave []string
	}{
		{
			name:        "linux, already on PATH",
			unameS:      "Linux",
			unameM:      "x86_64",
			binary:      "dinah-linux-amd64",
			onPath:      true,
			wantSubstrs: []string{"You can run dinah now."},
			mustNotHave: []string{"Debian and Ubuntu", "macOS does not add"},
		},
		{
			name:   "linux, first install, not yet on PATH",
			unameS: "Linux",
			unameM: "x86_64",
			binary: "dinah-linux-amd64",
			onPath: false,
			wantSubstrs: []string{
				"Debian and Ubuntu add ~/.local/bin to PATH automatically",
				`export PATH="$HOME/.local/bin:$PATH"`,
			},
			mustNotHave: []string{"You can run dinah now.", "macOS does not add"},
		},
		{
			name:        "darwin, already on PATH",
			unameS:      "Darwin",
			unameM:      "x86_64",
			binary:      "dinah-darwin-amd64",
			onPath:      true,
			wantSubstrs: []string{"You can run dinah now."},
			mustNotHave: []string{"Debian and Ubuntu", "macOS does not add"},
		},
		{
			name:   "darwin, not on PATH",
			unameS: "Darwin",
			unameM: "x86_64",
			binary: "dinah-darwin-amd64",
			onPath: false,
			wantSubstrs: []string{
				"macOS does not add ~/.local/bin to PATH by default.",
				`export PATH="$HOME/.local/bin:$PATH"`,
			},
			mustNotHave: []string{"You can run dinah now.", "Debian and Ubuntu"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var release publishedRelease
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/channels/dev.json"):
					w.Write(release.manifest)
				case strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt"):
					w.Write(release.sums)
				default:
					name := path.Base(r.URL.Path)
					content, ok := release.binaries[name]
					if !ok {
						http.NotFound(w, r)
						return
					}
					w.Write(content)
				}
			}))
			defer server.Close()

			built, err := buildPublishedRelease(server.URL + "/releases/download/v0.1.0-dev.7/")
			if err != nil {
				t.Fatalf("assembling the stand-in release: %v", err)
			}
			release = built

			script := strings.ReplaceAll(string(source), "https://github.com/paulmooreparks/dinah", server.URL)
			script = stubbedPlatform(t, script, tc.unameS, tc.unameM, tc.onPath)
			home := t.TempDir()
			scriptDir := t.TempDir()
			scriptPath := filepath.Join(scriptDir, "install.sh")
			if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
				t.Fatalf("writing the script under test: %v", err)
			}

			cmd := exec.Command("sh", scriptPath)
			cmd.Env = append(os.Environ(), "HOME="+home)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("scripts/install.sh failed: %v\n%s", err, output)
			}

			got := string(output)
			for _, want := range tc.wantSubstrs {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, got)
				}
			}
			for _, unwanted := range tc.mustNotHave {
				if strings.Contains(got, unwanted) {
					t.Errorf("expected output NOT to contain %q, got:\n%s", unwanted, got)
				}
			}

			installed, err := os.ReadFile(filepath.Join(home, ".local", "bin", "dinah"))
			if err != nil {
				t.Fatalf("no binary was installed: %v\n%s", err, got)
			}
			if !bytes.Equal(installed, release.binaries[tc.binary]) {
				t.Errorf("installed binary is not the published %s", tc.binary)
			}
		})
	}
}

// hijackTruncated takes over an already-accepted HTTP request and writes a
// response whose declared Content-Length is the length of full, but whose
// body stops after sendLen bytes, then closes the connection cleanly (a FIN,
// never a RST). That is what a proxy or a CDN edge does when it terminates a
// transfer early: nothing raises, and a client that only watches for a
// transport error never learns the transfer was short. sendLen == len(full)
// serves the whole declared length but with the wrong bytes past corruptFrom,
// which is a different failure (the transfer completed, the content is
// wrong) that must be reported with a different message.
func hijackTruncated(t *testing.T, w http.ResponseWriter, full []byte, sendLen int) {
	t.Helper()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("test server does not support hijacking")
	}
	conn, buf, err := hijacker.Hijack()
	if err != nil {
		t.Fatalf("hijacking the connection: %v", err)
	}
	defer conn.Close()
	fmt.Fprintf(buf, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nContent-Type: application/octet-stream\r\nConnection: close\r\n\r\n", len(full))
	buf.Write(full[:sendLen])
	buf.Flush()
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.CloseWrite()
	}
}

// TestInstallScriptsReportATruncatedDownloadDistinctlyFromCorruption proves
// two things about both install scripts: a download a proxy or CDN edge cuts
// short, closing the connection cleanly, is reported as a short download
// (never as a checksum mismatch), and a download that completes but carries
// the wrong bytes is still reported as a checksum mismatch. The two messages
// have to stay distinct in both directions, or a person on a flaky link is
// told their download is corrupt when it was only cut short.
func TestInstallScriptsReportATruncatedDownloadDistinctlyFromCorruption(t *testing.T) {
	full := bytes.Repeat([]byte("dinah release payload for the truncation test "), 200)
	sum := fmt.Sprintf("%x", sha256.Sum256(full))
	corrupted := append([]byte(nil), full...)
	corrupted[0] ^= 0xff // still the declared length; the bytes are simply wrong

	root := filepath.Join("..", "..")

	newManifestServer := func(binary, mode string) *httptest.Server {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/channels/dev.json"):
				compact := fmt.Sprintf(`{
  "channel": "dev",
  "version": "v0.1.0-dev.7",
  "tag": "v0.1.0-dev.7",
  "publishedAt": "2026-01-01T00:00:00Z",
  "downloadBase": %q,
  "binaries": {
    %q: { "sha256": %q, "size": %d }
  }
}
`, server.URL+"/releases/download/v0.1.0-dev.7/", binary, sum, len(full))
				var pretty bytes.Buffer
				if err := json.Indent(&pretty, []byte(compact), "", "    "); err != nil {
					t.Fatalf("indenting the stand-in manifest: %v", err)
				}
				w.Write(pretty.Bytes())
			case strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt"):
				fmt.Fprintf(w, "%s  %s\n", sum, binary)
			case strings.HasSuffix(r.URL.Path, "/"+binary):
				switch mode {
				case "truncated":
					hijackTruncated(t, w, full, len(full)/2)
				case "corrupted":
					hijackTruncated(t, w, corrupted, len(corrupted))
				default:
					t.Fatalf("unknown mode %q", mode)
				}
			default:
				http.NotFound(w, r)
			}
		}))
		return server
	}

	t.Run("install.sh", func(t *testing.T) {
		for _, tool := range []string{"sh", "curl"} {
			if _, err := exec.LookPath(tool); err != nil {
				t.Skipf("this machine has no %s, which scripts/install.sh needs", tool)
			}
		}
		source, err := os.ReadFile(filepath.Join(root, "scripts", "install.sh"))
		if err != nil {
			t.Fatalf("reading scripts/install.sh: %v", err)
		}

		run := func(mode string) (string, error) {
			server := newManifestServer("dinah-linux-amd64", mode)
			defer server.Close()
			script := strings.ReplaceAll(string(source), "https://github.com/paulmooreparks/dinah", server.URL)
			script = stubbedPlatform(t, script, "Linux", "x86_64", false)
			home := t.TempDir()
			scriptDir := t.TempDir()
			scriptPath := filepath.Join(scriptDir, "install.sh")
			if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
				t.Fatalf("writing the script under test: %v", err)
			}
			cmd := exec.Command("sh", scriptPath)
			cmd.Env = append(os.Environ(), "HOME="+home)
			output, err := cmd.CombinedOutput()
			if _, statErr := os.Stat(filepath.Join(home, ".local", "bin", "dinah")); statErr == nil {
				t.Errorf("%s: a file was installed even though the download never verified", mode)
			}
			return string(output), err
		}

		truncatedOut, truncatedErr := run("truncated")
		if truncatedErr == nil {
			t.Fatalf("truncated download: expected install.sh to fail, it exited 0\n%s", truncatedOut)
		}
		if !strings.Contains(truncatedOut, "did not complete (network error)") {
			t.Errorf("truncated download: expected the network-error message, got:\n%s", truncatedOut)
		}
		if strings.Contains(truncatedOut, "checksum does not match") {
			t.Errorf("truncated download: reported as a checksum mismatch instead of a short download:\n%s", truncatedOut)
		}

		corruptedOut, corruptedErr := run("corrupted")
		if corruptedErr == nil {
			t.Fatalf("corrupted download: expected install.sh to fail, it exited 0\n%s", corruptedOut)
		}
		if !strings.Contains(corruptedOut, "checksum does not match") {
			t.Errorf("corrupted download: expected the checksum-mismatch message, got:\n%s", corruptedOut)
		}
		if strings.Contains(corruptedOut, "did not complete (network error)") {
			t.Errorf("corrupted download: reported as a network error instead of a checksum mismatch:\n%s", corruptedOut)
		}
	})

	t.Run("install.ps1", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("scripts/install.ps1 targets Windows PowerShell")
		}
		psExe, err := exec.LookPath("powershell.exe")
		if err != nil {
			t.Skip("this machine has no powershell.exe, which scripts/install.ps1 needs")
		}
		source, err := os.ReadFile(filepath.Join(root, "scripts", "install.ps1"))
		if err != nil {
			t.Fatalf("reading scripts/install.ps1: %v", err)
		}

		run := func(mode string) (string, error) {
			server := newManifestServer("dinah-windows-amd64.exe", mode)
			defer server.Close()
			script := strings.ReplaceAll(string(source), "https://github.com/paulmooreparks/dinah", server.URL)
			localAppData := t.TempDir()
			scriptDir := t.TempDir()
			scriptPath := filepath.Join(scriptDir, "install.ps1")
			if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
				t.Fatalf("writing the script under test: %v", err)
			}
			cmd := exec.Command(psExe, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
			cmd.Env = windowsPowerShellEnv(
				"LOCALAPPDATA="+localAppData,
				"PROCESSOR_ARCHITECTURE=AMD64",
				"DINAH_NO_PATH=1", // never touch the real PATH from a test
			)
			output, err := cmd.CombinedOutput()
			if _, statErr := os.Stat(filepath.Join(localAppData, "dinah", "bin", "dinah.exe")); statErr == nil {
				t.Errorf("%s: a file was installed even though the download never verified", mode)
			}
			return string(output), err
		}

		truncatedOut, truncatedErr := run("truncated")
		if truncatedErr == nil {
			t.Fatalf("truncated download: expected install.ps1 to fail, it exited 0\n%s", truncatedOut)
		}
		if !strings.Contains(truncatedOut, "is incomplete") {
			t.Errorf("truncated download: expected the incomplete-download message, got:\n%s", truncatedOut)
		}
		if strings.Contains(truncatedOut, "checksum does not match") {
			t.Errorf("truncated download: reported as a checksum mismatch instead of a short download:\n%s", truncatedOut)
		}

		corruptedOut, corruptedErr := run("corrupted")
		if corruptedErr == nil {
			t.Fatalf("corrupted download: expected install.ps1 to fail, it exited 0\n%s", corruptedOut)
		}
		if !strings.Contains(corruptedOut, "checksum does not match") {
			t.Errorf("corrupted download: expected the checksum-mismatch message, got:\n%s", corruptedOut)
		}
		if strings.Contains(corruptedOut, "is incomplete") {
			t.Errorf("corrupted download: reported as incomplete instead of a checksum mismatch:\n%s", corruptedOut)
		}
	})
}

// psQuote wraps a string in double quotes for interpolation into a
// PowerShell command line. Go's %q escapes backslashes for Go source syntax,
// which corrupts a Windows registry path (HKCU:\Software\...) built with it;
// none of the strings this test passes through contain a double quote, so
// plain wrapping is enough.
func psQuote(s string) string {
	return `"` + s + `"`
}

// runPS runs a short PowerShell command and fails the test if it errors.
func runPS(t *testing.T, psExe, command string) error {
	t.Helper()
	cmd := exec.Command(psExe, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command)
	cmd.Env = windowsPowerShellEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\n%s", err, output)
	}
	return nil
}

// readRegistryPath reads the PATH value under a registry key, unexpanded, the
// same way scripts/install.ps1 reads it. An absent value reads back as "".
func readRegistryPath(t *testing.T, psExe, key string) (string, error) {
	t.Helper()
	cmd := exec.Command(psExe, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command",
		fmt.Sprintf("(Get-Item -Path %s -ErrorAction SilentlyContinue).GetValue('PATH','',[Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)", psQuote(key)))
	cmd.Env = windowsPowerShellEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v\n%s", err, output)
	}
	return strings.TrimRight(string(output), "\r\n"), nil
}

// TestInstallPS1SaysWhetherDinahIsReadyToRun exercises scripts/install.ps1's
// PATH message across the four states persisted PATH and this session's PATH
// can combine into: registry and session can each independently already have
// the install directory or not. Writing the registry does not change what an
// already-running PowerShell process can see, so "already on your PATH" is
// only true of a session that started after that write landed; a session
// that started before it needs a different, still accurate, message.
//
// The registry key the script reads and writes is redirected to a throwaway
// key created and torn down by this test, never HKCU:\Environment on the
// machine running it, per the standing rule against touching real PATH. The
// real key's value is read before the test and confirmed unchanged after.
func TestInstallPS1SaysWhetherDinahIsReadyToRun(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("scripts/install.ps1 targets Windows PowerShell")
	}
	psExe, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("this machine has no powershell.exe, which scripts/install.ps1 needs")
	}

	root := filepath.Join("..", "..")
	source, err := os.ReadFile(filepath.Join(root, "scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("reading scripts/install.ps1: %v", err)
	}

	const realKey = `HKCU:\Environment`
	realBefore, err := readRegistryPath(t, psExe, realKey)
	if err != nil {
		t.Fatalf("reading the real %s before the test: %v", realKey, err)
	}
	t.Cleanup(func() {
		realAfter, err := readRegistryPath(t, psExe, realKey)
		if err != nil {
			t.Fatalf("reading the real %s after the test: %v", realKey, err)
		}
		if realAfter != realBefore {
			t.Fatalf("this test modified the real Windows user PATH: before %q, after %q", realBefore, realAfter)
		}
	})

	throwawayKey := fmt.Sprintf(`HKCU:\Software\DinahInstallTest%d`, os.Getpid())
	t.Cleanup(func() {
		runPS(t, psExe, fmt.Sprintf("Remove-Item -Path %s -Recurse -Force -ErrorAction SilentlyContinue", psQuote(`HKCU:\Software\DinahInstallTest`+fmt.Sprint(os.Getpid()))))
	})

	script := strings.ReplaceAll(string(source), `HKCU:\Environment`, throwawayKey)
	if !strings.Contains(script, throwawayKey) {
		t.Fatal("the throwaway registry key did not reach the script under test")
	}

	full := []byte("stand-in dinah.exe payload\n")
	sum := fmt.Sprintf("%x", sha256.Sum256(full))
	const binary = "dinah-windows-amd64.exe"

	cases := []struct {
		name          string
		registryHasIt bool
		sessionHasIt  bool
		wantSubstrs   []string
		mustNotHave   []string
	}{
		{
			name:          "already configured, this session sees it",
			registryHasIt: true,
			sessionHasIt:  true,
			wantSubstrs:   []string{"Run: dinah version"},
			mustNotHave:   []string{"Open a new"},
		},
		{
			name:          "already configured, this session predates it",
			registryHasIt: true,
			sessionHasIt:  false,
			wantSubstrs:   []string{"this session started before that took effect", `$env:Path = "`},
			mustNotHave:   []string{"Run: dinah version"},
		},
		{
			name:          "freshly added, session already sees the directory",
			registryHasIt: false,
			sessionHasIt:  true,
			wantSubstrs:   []string{"already on this session's PATH, so you can run dinah now"},
			mustNotHave:   []string{"Open a new shell"},
		},
		{
			name:          "freshly added, ordinary first install",
			registryHasIt: false,
			sessionHasIt:  false,
			wantSubstrs:   []string{"Open a new shell to pick it up, or run this to use dinah now", `$env:Path = "`},
			mustNotHave:   []string{"Run: dinah version"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var real *httptest.Server
			real = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/channels/dev.json"):
					fmt.Fprintf(w, `{
  "channel": "dev",
  "version": "v0.1.0-dev.7",
  "tag": "v0.1.0-dev.7",
  "publishedAt": "2026-01-01T00:00:00Z",
  "downloadBase": %q,
  "binaries": {
    %q: { "sha256": %q, "size": %d }
  }
}
`, real.URL+"/releases/download/v0.1.0-dev.7/", binary, sum, len(full))
				case strings.HasSuffix(r.URL.Path, "/"+binary):
					w.Write(full)
				default:
					http.NotFound(w, r)
				}
			}))
			defer real.Close()

			runScript := strings.ReplaceAll(script, "https://github.com/paulmooreparks/dinah", real.URL)

			localAppData := t.TempDir()
			installDir := filepath.Join(localAppData, "dinah", "bin")

			if err := runPS(t, psExe, fmt.Sprintf("Remove-Item -Path %s -Force -ErrorAction SilentlyContinue; New-Item -Path %s -Force | Out-Null", psQuote(throwawayKey), psQuote(throwawayKey))); err != nil {
				t.Fatalf("resetting the throwaway registry key: %v", err)
			}
			if tc.registryHasIt {
				if err := runPS(t, psExe, fmt.Sprintf("Set-ItemProperty -Path %s -Name PATH -Value %s -Type ExpandString", psQuote(throwawayKey), psQuote(installDir))); err != nil {
					t.Fatalf("seeding the throwaway registry PATH: %v", err)
				}
			}

			scriptDir := t.TempDir()
			scriptPath := filepath.Join(scriptDir, "install.ps1")
			if err := os.WriteFile(scriptPath, []byte(runScript), 0o644); err != nil {
				t.Fatalf("writing the script under test: %v", err)
			}

			envPath := os.Getenv("PATH")
			if tc.sessionHasIt {
				envPath = installDir + string(os.PathListSeparator) + envPath
			}

			cmd := exec.Command(psExe, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
			cmd.Env = windowsPowerShellEnv(
				"LOCALAPPDATA="+localAppData,
				"PROCESSOR_ARCHITECTURE=AMD64",
				"PATH="+envPath,
			)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("scripts/install.ps1 failed: %v\n%s", err, output)
			}

			got := string(output)
			for _, want := range tc.wantSubstrs {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, got)
				}
			}
			for _, unwanted := range tc.mustNotHave {
				if strings.Contains(got, unwanted) {
					t.Errorf("expected output NOT to contain %q, got:\n%s", unwanted, got)
				}
			}
		})
	}
}
