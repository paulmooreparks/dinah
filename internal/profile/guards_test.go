package profile

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
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
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"dinah/internal/contract"
	"dinah/internal/msg"
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
// storage, the workbench this repository carries as data, the binary assets
// that hold no prose, and the four trees the VS Code extension's toolchain
// materialises.
//
// Those four are not this repository's prose and are not ours to rewrite.
// node_modules holds several hundred third-party packages, .vscode-test holds
// a whole downloaded VS Code including its third-party notices, and dist and
// out hold generated JavaScript. A dependency that ships a benchmark suite
// would otherwise fail this guard for a word written by somebody who has
// never heard of Dinah, which is the same hazard that took ci.yml's gofmt
// step from `gofmt -l .` to `gofmt -l cmd internal`.
var skippedTrees = map[string]bool{
	".git":         true,
	".dinah":       true,
	"logo":         true,
	"__pycache__":  true,
	"node_modules": true,
	".vscode-test": true,
	"dist":         true,
	"out":          true,
}

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

// TestInstallPS1VerifiesWithoutGetFileHash runs scripts/install.ps1 to a
// successful, checksum-verified finish under a PSModulePath that shadows
// Windows PowerShell's own Get-FileHash with a PowerShell 7 installation's
// module directory, the shape a colleague's machine takes on when both
// PowerShell editions are installed.
//
// Get-FileHash is exported by the Microsoft.PowerShell.Utility module
// (Microsoft Learn's own cmdlet reference names it), not one of the cmdlets
// Windows PowerShell's default session already carries, so Windows
// PowerShell reaches it only by autoloading that module off PSModulePath
// (documented in about_Modules and about_PSModulePath). PowerShell 7 ships
// its own copy of Microsoft.PowerShell.Utility built into pwsh.exe itself
// rather than as a discoverable module under its Modules directory, so a
// Windows PowerShell session handed a PSModulePath naming that directory
// finds no Get-FileHash there and dies with CommandNotFoundException, after a
// successful download. install.ps1 closes this by computing the digest with
// System.Security.Cryptography.SHA256 directly: a base class library type,
// not a module export, so it needs no autoload and does not depend on
// PSModulePath.
//
// This test proves the fix rather than assuming the mechanism: it first
// confirms, against the real powershell.exe and pwsh.exe on this machine,
// that the poisoned PSModulePath really does make Get-FileHash unresolvable,
// and skips rather than passing vacuously if it does not.
func TestInstallPS1VerifiesWithoutGetFileHash(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("scripts/install.ps1 targets Windows PowerShell")
	}
	psExe, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("this machine has no powershell.exe, which scripts/install.ps1 needs")
	}
	pwshExe, err := exec.LookPath("pwsh.exe")
	if err != nil {
		t.Skip("this machine has no pwsh.exe (PowerShell 7), so the two-edition shape this test reproduces cannot be built here")
	}

	// Ask PowerShell 7 for its own PSModulePath, then take the one entry
	// that lives under its own installation directory. That is the entry a
	// machine with both editions installed can hand to Windows PowerShell
	// when the two inherit the same variable, and it is the shape that
	// produced the original failure.
	out, err := exec.Command(pwshExe, "-NoProfile", "-NonInteractive", "-Command", "$env:PSModulePath").CombinedOutput()
	if err != nil {
		t.Fatalf("asking pwsh.exe for its PSModulePath: %v\n%s", err, out)
	}
	pwshDir := strings.ToLower(filepath.Dir(pwshExe))
	var poison string
	for _, entry := range strings.Split(strings.TrimSpace(string(out)), ";") {
		if strings.HasPrefix(strings.ToLower(entry), pwshDir) {
			poison = entry
			break
		}
	}
	if poison == "" {
		t.Skip("could not find a PowerShell 7 module directory under its own install path in $env:PSModulePath, so the two-edition shape cannot be built here")
	}

	// Precondition: this really does shadow Get-FileHash from Windows
	// PowerShell on this machine, or the test proves nothing.
	probe := exec.Command(psExe, "-NoProfile", "-NonInteractive", "-Command", `Get-FileHash C:\Windows\win.ini -Algorithm SHA256`)
	probe.Env = windowsPowerShellEnv("PSModulePath=" + poison)
	if probeOut, probeErr := probe.CombinedOutput(); probeErr == nil {
		t.Skipf("Get-FileHash resolved under PSModulePath=%s on this machine, so it does not reproduce the two-edition shadowing this test targets:\n%s", poison, probeOut)
	}

	root := filepath.Join("..", "..")
	source, err := os.ReadFile(filepath.Join(root, "scripts", "install.ps1"))
	if err != nil {
		t.Fatalf("reading scripts/install.ps1: %v", err)
	}

	full := []byte("stand-in dinah.exe payload for the PSModulePath test\n")
	sum := fmt.Sprintf("%x", sha256.Sum256(full))
	const binary = "dinah-windows-amd64.exe"

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
`, server.URL+"/releases/download/v0.1.0-dev.7/", binary, sum, len(full))
		case strings.HasSuffix(r.URL.Path, "/"+binary):
			w.Write(full)
		default:
			http.NotFound(w, r)
		}
	}))
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
		"DINAH_NO_PATH=1",
		"PSModulePath="+poison,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("scripts/install.ps1 failed under a PSModulePath that shadows Get-FileHash: %v\n%s", err, output)
	}
	if strings.Contains(string(output), "CommandNotFoundException") {
		t.Errorf("install.ps1 still depends on something PSModulePath can shadow:\n%s", output)
	}

	installed, err := os.ReadFile(filepath.Join(localAppData, "dinah", "bin", "dinah.exe"))
	if err != nil {
		t.Fatalf("no binary was installed: %v\n%s", err, output)
	}
	if !bytes.Equal(installed, full) {
		t.Errorf("installed binary does not match the published bytes\ngot:  %q\nwant: %q", installed, full)
	}
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
// PATH message across the four columns persisted PATH and this session's PATH
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

// theOneTable is the file every table is measured and laid out in. Choosing
// each column's width, drawing a heading and a rule over it, and handing each
// row to the one renderer all live there, and the four patterns this file adds
// are what keep them there.
const theOneTable = "cmd/dinah/table.go"

// streamWriters are the functions of the rendering head that may name an
// output stream. Everything a person reads reaches a stream through one of
// them, so a mention anywhere else is either a row being written outside the
// renderer or a second way to write, and pattern 11 refuses both.
//
// The first eight are the writers themselves, and composeRefusal is not among
// them: it returns lines rather than writing them, which is what lets the text
// path and the machine path share one composition. The last three hand a
// stream to something outside the head: the value config <key> writes, the
// child process an editor runs in, and the MCP server that serves on stdio.
//
// The guard asserts every name here still resolves to a function in the
// rendering head, so a renamed writer fails rather than leaving a stale name
// covering something else.
var streamWriters = []string{
	"write",
	"line",
	"errLine",
	"fail",
	"reportOutcome",
	"emitJSON",
	"emit",
	"reportError",
	"runPath",
	"runEdit",
	"runMCP",
}

// processStreamHolders are the two functions that may name the process's own
// streams: main, which hands them to run, and windowWidth, which asks the
// terminal behind stdout how wide it is.
var processStreamHolders = []string{"main", "windowWidth"}

// columnarCatalogKeys are the catalog entries that compose columns inside a
// translated string, where the translator owns the spacing and no column is
// declared. Every one of them is present in every shipped catalog, and the
// guard asserts that each still matches something, so a retired entry cannot
// leave a stale name covering another one.
//
// card.line.workstreams is card.line with a trailing field on the end, chosen
// by the same caller, so it is exempt for card.line's own reason.
// workstream.line is that line's counterpart for an act on a workstream.
var columnarCatalogKeys = []string{
	"card.line",
	"card.line.workstreams",
	"workstream.line",
	"status.workbench",
}

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
	patternCellLiteral
	patternRowCall
	patternMeasure
	patternStreamMention
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
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(theOneTable))); err != nil {
		t.Errorf("the one table is not where this guard exempts it: %v", err)
	}

	// Every name pattern 11 lets name a stream has to still be a function of
	// the rendering head. A renamed writer would otherwise leave its old name
	// on the list, covering whatever function takes the name next, which is
	// the same failure the catalog exemption above is checked for.
	declared, err := functionsOfTheRenderingHead(root)
	if err != nil {
		t.Fatalf("read the rendering head: %v", err)
	}
	for _, name := range append(append([]string{}, streamWriters...), processStreamHolders...) {
		if !declared[name] {
			t.Errorf("pattern 11 exempts %s and the rendering head declares no such function, so the name covers nothing and may be covering something else; drop it or correct it", name)
		}
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

	if name != theOneTable {
		ast.Inspect(file, func(node ast.Node) bool {
			switch shape := node.(type) {
			case *ast.CompositeLit:
				if namesTheCellType(shape.Type) {
					findings = append(findings, rowLayoutFinding{
						where:   at(shape.Pos()),
						pattern: patternCellLiteral,
						detail:  "builds a cell, which carries a width no call site chooses",
					})
				}
			case *ast.CallExpr:
				if callsTheRenderer(shape.Fun) {
					findings = append(findings, rowLayoutFinding{
						where:   at(shape.Pos()),
						pattern: patternRowCall,
						detail:  "lays a row out itself, which only the table does",
					})
				}
			}
			return true
		})
	}

	if !strings.HasPrefix(name, theRenderingHead) {
		return findings, nil
	}

	if name != theOneRenderer && name != theOneTable {
		findings = append(findings, measurementFindings(file, at)...)
		findings = append(findings, streamMentions(file, at)...)
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

// functionsOfTheRenderingHead reports the name of every function the head
// declares, methods included, read from its non-test sources.
func functionsOfTheRenderingHead(root string) (map[string]bool, error) {
	dir := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(theRenderingHead, "/")))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	declared := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		for _, node := range file.Decls {
			if function, ok := node.(*ast.FuncDecl); ok {
				declared[function.Name.Name] = true
			}
		}
	}
	return declared, nil
}

// namesTheCellType reports whether a composite literal is a cell or a slice of
// them. The element type of a []cell literal is what the guard reads, so the
// elements of such a literal, which carry no type of their own, are caught by
// the literal that holds them.
func namesTheCellType(expr ast.Expr) bool {
	if isBareIdent(expr, "cell") {
		return true
	}
	array, ok := expr.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return false
	}
	return isBareIdent(array.Elt, "cell")
}

// callsTheRenderer reports whether a call reaches formatRow, row or rowLine.
//
// The match is on the callee's own name and nothing else: a bare identifier,
// or a selector whose final name is one of the three whatever the receiver is
// spelled. It is deliberately not isSelectorCall, which keys on the receiver
// identifier, because a receiver renamed from s would walk straight past that.
func callsTheRenderer(expr ast.Expr) bool {
	named := ""
	switch callee := expr.(type) {
	case *ast.Ident:
		named = callee.Name
	case *ast.SelectorExpr:
		named = callee.Sel.Name
	default:
		return false
	}
	return named == "formatRow" || named == "row" || named == "rowLine"
}

// measurementFindings reports every place under the rendering head that
// measures how wide something draws. After the table, a call site has no
// reason to ask: the table measures its own rows and the renderer lays them
// out.
//
// This pattern does not stop a hand-rolled table measuring. It stops one
// measuring correctly, and a byte length is what a hand-rolled table reaches
// for instead.
func measurementFindings(file *ast.File, at func(token.Pos) string) []rowLayoutFinding {
	var findings []rowLayoutFinding
	for _, imported := range file.Imports {
		if imported.Path.Value == strconv.Quote("dinah/internal/textwidth") {
			findings = append(findings, rowLayoutFinding{
				where:   at(imported.Pos()),
				pattern: patternMeasure,
				detail:  "imports the display-width measure, which only the renderer and the table read",
			})
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if isBareIdent(call.Fun, "displayWidth") || (isSelector && isBareIdent(selector.X, "textwidth")) {
			findings = append(findings, rowLayoutFinding{
				where:   at(call.Pos()),
				pattern: patternMeasure,
				detail:  "measures how wide text draws, which only the renderer and the table do",
			})
		}
		return true
	})
	return findings
}

// streamMentions reports every mention of an output stream outside the writers
// that own it, which is the pattern that closes the routes a printing call can
// take. Writing at the stream, calling its write method, and wrapping it in a
// buffer are three of them, and each has to name the stream to reach it.
//
// The fmt printing family is refused wherever it appears under the head, since
// the head has no use for any of it: every line a person reads is composed
// through the catalog and written by one of the writers above.
func streamMentions(file *ast.File, at func(token.Pos) string) []rowLayoutFinding {
	writers := map[string]bool{}
	for _, name := range streamWriters {
		writers[name] = true
	}
	holders := map[string]bool{}
	for _, name := range processStreamHolders {
		holders[name] = true
	}
	var findings []rowLayoutFinding
	for _, declared := range file.Decls {
		function, ok := declared.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		within := function.Name.Name
		ast.Inspect(function.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if isBareIdent(selector.X, "fmt") && strings.HasPrefix(selector.Sel.Name, "Fprint") {
				findings = append(findings, rowLayoutFinding{
					where:   at(selector.Pos()),
					pattern: patternStreamMention,
					detail:  "calls fmt." + selector.Sel.Name + ", which the head never has a use for",
				})
				return true
			}
			if isBareIdent(selector.X, "s") && (selector.Sel.Name == "out" || selector.Sel.Name == "errw") {
				if writers[within] {
					return true
				}
				findings = append(findings, rowLayoutFinding{
					where:   at(selector.Pos()),
					pattern: patternStreamMention,
					detail:  "names the stream s." + selector.Sel.Name + " inside " + within + ", which is not one of the writers that own it",
				})
				return true
			}
			if isBareIdent(selector.X, "os") && (selector.Sel.Name == "Stdout" || selector.Sel.Name == "Stderr") {
				if holders[within] {
					return true
				}
				findings = append(findings, rowLayoutFinding{
					where:   at(selector.Pos()),
					pattern: patternStreamMention,
					detail:  "names os." + selector.Sel.Name + " inside " + within + ", which is neither main nor the window query",
				})
			}
			return true
		})
	}
	return findings
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
	{
		pattern: patternCellLiteral,
		source:  "package planted\n\nvar columns = []cell{{\"fx-1\", 14}, {\"ready\", 10}}\n",
	},
	{
		pattern: patternRowCall,
		source:  "package planted\n\nfunc listed(s *session, ref string) { s.row(row{indent: 2, tail: ref}) }\n",
	},
	{
		pattern: patternMeasure,
		source:  "package planted\n\nfunc drawn(text string) int { return displayWidth(text) }\n",
	},
	{
		pattern: patternStreamMention,
		source:  "package planted\n\nimport \"io\"\n\nfunc listed(s *session, line string) { io.WriteString(s.out, line) }\n",
	},
}

// widenHelper is the padding every planted table below shares: a loop that
// counts bytes, which is what somebody reaches for who has not read the
// renderer, and which trips none of the eleven patterns on its own. Spelling
// the indent and the gutter through it rather than as a literal holding two
// spaces is what walks a plant past pattern 5, and the spec says as much.
const widenHelper = "\nfunc widen(text string, n int) string {\n\tfor len(text) < n {\n\t\ttext += \" \"\n\t}\n\treturn text\n}\n"

// handRolledTables are the seven whole tables the spec tabulates as routes A
// through G, each drawing the rows of dinah ls and each padding its first
// column by counting bytes.
//
// Five of them are caught and two are not, and the fixture asserts both, since
// the claim that decides this card is that a scan of Go sources cannot see
// every route. Route C hands the stream to a helper that writes on its
// behalf, from inside a writer that is allowed to name the stream, and
// deciding whether that helper composes columns is a whole-program analysis
// rather than a pattern. Route D passes the composed row into a translated
// message, where the outermost expression a scan reads is the translator's own
// call, which every scan has to approve.
//
// A later relaxation cannot quietly move a route from caught to uncaught, and
// a later tightening cannot quietly claim one of the two, because both
// directions are asserted here.
var handRolledTables = []struct {
	// route is the letter the spec's table gives this shape.
	route string
	// caught says whether the source patterns report it at all.
	caught bool
	// why names, for a caught route, the pattern that reports it, and for an
	// uncaught one, what a scan of Go sources would have to do to see it.
	why string
	// source is a whole Go file, scanned as though it sat in the rendering
	// head.
	source string
}{
	{
		route: "A", caught: true, why: "pattern 11 refuses fmt.Fprintln and the mention of the stream",
		source: "package planted\n\nimport \"fmt\"\n\nfunc (s *session) renderListing(cards []string) {\n\tfor _, card := range cards {\n\t\tfmt.Fprintln(s.out, widen(card, 14)+\"ready\")\n\t}\n}\n" + widenHelper,
	},
	{
		route: "B", caught: true, why: "pattern 11 refuses the mention of the stream, whatever is called on it",
		source: "package planted\n\nfunc (s *session) renderListing(cards []string) {\n\tfor _, card := range cards {\n\t\ts.out.Write([]byte(widen(card, 14) + \"ready\"))\n\t}\n}\n" + widenHelper,
	},
	{
		route: "C", caught: false, why: "the stream is handed to a helper from a writer that owns it, and the helper writes through its own parameter",
		source: "package planted\n\nimport \"io\"\n\nfunc (s *session) write(cards []string) {\n\temitLines(s.out, listingLines(cards))\n}\n\nfunc emitLines(w io.Writer, lines []string) {\n\tfor _, line := range lines {\n\t\tio.WriteString(w, line+\"\\n\")\n\t}\n}\n\nfunc listingLines(cards []string) []string {\n\tlines := []string{}\n\tfor _, card := range cards {\n\t\tlines = append(lines, widen(\"\", 2)+widen(card, 14)+\"ready\")\n\t}\n\treturn lines\n}\n" + widenHelper,
	},
	{
		route: "D", caught: false, why: "the row rides into a translated message as a substitution value, behind the translator's own call",
		source: "package planted\n\nfunc (s *session) renderListing(cards []string) {\n\tfor _, card := range cards {\n\t\ts.line(s.r.T(\"outcome.unreachable\", \"detail\", widen(\"\", 2)+widen(card, 14)+\"ready\"))\n\t}\n}\n" + widenHelper,
	},
	{
		route: "E", caught: true, why: "pattern 11 refuses the mention of the stream the buffer wraps",
		source: "package planted\n\nimport \"bufio\"\n\nfunc (s *session) renderListing(cards []string) {\n\tbuffered := bufio.NewWriter(s.out)\n\tfor _, card := range cards {\n\t\tbuffered.WriteString(widen(card, 14) + \"ready\")\n\t}\n\tbuffered.Flush()\n}\n" + widenHelper,
	},
	{
		route: "F", caught: true, why: "pattern 11 refuses fmt.Fprintf and the mention of the stream, where pattern 3 is silent for want of a width verb",
		source: "package planted\n\nimport \"fmt\"\n\nfunc (s *session) renderListing(cards []string) {\n\tfor _, card := range cards {\n\t\tfmt.Fprintf(s.out, \"%s%s\\n\", widen(card, 14), \"ready\")\n\t}\n}\n" + widenHelper,
	},
	{
		route: "G", caught: true, why: "pattern 10 refuses the measure it pads by",
		source: "package planted\n\nimport \"strings\"\n\nfunc (s *session) renderListing(cards []string) {\n\tfor _, card := range cards {\n\t\tfields := []string{widen(card, 14-displayWidth(card)), \"ready\"}\n\t\ts.line(strings.Join(fields, widen(\"\", 2)))\n\t}\n}\n" + widenHelper,
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

	for _, planted := range handRolledTables {
		t.Run("route "+planted.route, func(t *testing.T) {
			path := filepath.Join(dir, "route"+planted.route+".go")
			if err := os.WriteFile(path, []byte(planted.source), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			found, err := scanGoForRowLayout(path, theRenderingHead+"route"+planted.route+".go")
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if planted.caught && len(found) == 0 {
				t.Errorf("route %s draws a whole table by hand and the guard reported nothing; it is meant to be caught, %s", planted.route, planted.why)
			}
			if !planted.caught && len(found) != 0 {
				t.Errorf("route %s is one of the two the spec says a scan of Go sources cannot see, %s, and the guard reported %v; the spec's own claim about what these patterns establish has to be corrected before this passes",
					planted.route, planted.why, found)
			}
		})
	}

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

// refusalCatalogDir is where the shipped catalogs live, read as data by the
// one check of the refusal guard that reads catalogs rather than Go sources.
const refusalCatalogDir = "internal/msg/locales"

// refusalPlaceholder matches a named slot inside a catalog entry, which is the
// only kind of placeholder internal/msg fills.
var refusalPlaceholder = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_-]*)\}`)

// refusalRaisers are the call names that raise a refusal. A first argument
// naming a contract constant at one of them is a raise site, whatever package
// it sits in.
var refusalRaisers = map[string]bool{
	"Refuse": true, "RefuseWith": true, "refuse": true, "refuseWith": true, "fail": true,
}

// theComposer is the one function that renders a refusal for a person. It
// names no stream, which is what lets the text path and the machine path share
// one composition.
const theComposer = "composeRefusal"

// refusalCatalog is one shipped locale read as data.
type refusalCatalog struct {
	tag     string
	entries map[string]string
}

// readRefusalCatalogs reads every shipped locale, so a check can hold one
// locale's entries against another's rather than against the renderer, which
// agrees with whatever entries it was handed.
func readRefusalCatalogs(root string) ([]refusalCatalog, error) {
	dir := filepath.Join(root, filepath.FromSlash(refusalCatalogDir))
	found, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var catalogs []refusalCatalog
	for _, entry := range found {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var read struct {
			Tag     string `json:"tag"`
			Entries map[string]struct {
				Text string `json:"text"`
			} `json:"entries"`
		}
		if err := json.Unmarshal(data, &read); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		catalog := refusalCatalog{tag: read.Tag, entries: map[string]string{}}
		for key, value := range read.Entries {
			catalog.entries[key] = value.Text
		}
		catalogs = append(catalogs, catalog)
	}
	sort.Slice(catalogs, func(i, j int) bool { return catalogs[i].tag < catalogs[j].tag })
	return catalogs, nil
}

// contractConstants reads the refusal-name constants out of internal/contract,
// so a raise site naming contract.Malformed is resolved to the name it carries
// rather than to the identifier that spells it.
func contractConstants(root string) (map[string]string, error) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseDir(fileSet, filepath.Join(root, "internal", "contract"), func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, pkg := range parsed {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				general, ok := decl.(*ast.GenDecl)
				if !ok || general.Tok != token.CONST {
					continue
				}
				for _, spec := range general.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok || len(value.Names) != len(value.Values) {
						continue
					}
					for i, name := range value.Names {
						if text, ok := foldContractName(value.Values[i], values); ok {
							values[name.Name] = text
						}
					}
				}
			}
		}
	}
	return values, nil
}

// foldContractName evaluates a constant expression that is a string literal or
// a concatenation reaching one, which is the shape every refusal name in
// internal/contract takes: a literal, or LayerPrefix plus a literal.
func foldContractName(expr ast.Expr, known map[string]string) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(e.Value)
		return text, err == nil
	case *ast.Ident:
		text, ok := known[e.Name]
		return text, ok
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		left, ok := foldContractName(e.X, known)
		if !ok {
			return "", false
		}
		right, ok := foldContractName(e.Y, known)
		if !ok {
			return "", false
		}
		return left + right, true
	case *ast.ParenExpr:
		return foldContractName(e.X, known)
	}
	return "", false
}

// TestEveryRefusalIsComposedTheOneWay holds the shape table, the catalogs and
// the raise sites against each other, so that a refusal cannot be raised with
// no declaration, declared with no sentence, or given a sentence nobody can
// reach.
//
// It reads sources and catalogs rather than running the binary, so it sees
// what no rendering can show it: a shape whose next step could fail to render,
// a placeholder nobody fills, and a catalog that kept a clause it handed to a
// fragment. What it cannot see is recorded in the card's spec and in the
// limits the header comment of cmd/dinah/output_check_test.go already records.
// The output-side half of the one-way rule is checkRefusalShape in that
// package, which reads rendered bytes.
func TestEveryRefusalIsComposedTheOneWay(t *testing.T) {
	root := filepath.Join("..", "..")
	catalogs, err := readRefusalCatalogs(root)
	if err != nil {
		t.Fatalf("read the catalogs: %v", err)
	}
	if len(catalogs) == 0 {
		t.Fatal("no catalog was read, so this guard proves nothing")
	}
	var base map[string]string
	for _, catalog := range catalogs {
		if catalog.tag == msg.Base {
			base = catalog.entries
		}
	}
	if base == nil {
		t.Fatalf("the base catalog %s does not ship, so nothing below can be read", msg.Base)
	}

	shapes := map[string]*contract.Shape{}
	for i := range contract.Shapes {
		shape := &contract.Shapes[i]
		if _, twice := shapes[shape.Name]; twice {
			t.Errorf("%s carries two shapes, and a refusal has one", shape.Name)
		}
		shapes[shape.Name] = shape
	}

	checkOneRefusalIsOneDeclaration(t, root, shapes, base)
	checkOnePlaceBuildsARefusal(t, root)
	checkEveryShapeSaysWhatToDoNext(t, shapes)
	checkNoPlaceholderIsStrayOrOrphaned(t, shapes, base)
	checkNoEnglishLiteralReachesAPlaceholder(t, root)
	checkNoEmptySubjectReachesASentence(t, shapes, base)
	checkNoCatalogKeepsAClauseItHandedOver(t, shapes, catalogs)
}

// checkOneRefusalIsOneDeclaration is check 1: the name, the sentence and the
// shape close over each other, so adding any one of the three without the
// others fails.
func checkOneRefusalIsOneDeclaration(t *testing.T, root string, shapes map[string]*contract.Shape, base map[string]string) {
	t.Helper()
	for _, name := range append(append([]string{}, contract.Declared...), contract.Introduced...) {
		if shapes[name] == nil {
			t.Errorf("%s is a refusal name and no shape declares it, so nothing says what it carries", name)
		}
	}
	for _, name := range sortedShapeNames(shapes) {
		shape := shapes[name]
		if _, ok := base[refusalKeyOf(name)]; !ok {
			t.Errorf("%s carries a shape and the base catalog carries no %s, so the shape declares a sentence nobody wrote", name, refusalKeyOf(name))
		}
		if len(shape.Variants) > 0 && shape.Subject != "" {
			t.Errorf("%s declares both a subject and per-command variants, and nothing states which of the two selects the base entry", name)
		}
		for _, command := range shape.Variants {
			if _, ok := base[shape.VariantKeyOf(command)]; !ok {
				t.Errorf("%s declares the variant %s and the base catalog carries no %s, so a command selects a sentence nobody wrote", name, command, shape.VariantKeyOf(command))
			}
		}
	}
	raised, err := raisedRefusalNames(root)
	if err != nil {
		t.Fatalf("walk the raise sites: %v", err)
	}
	if len(raised) == 0 {
		t.Error("the walk found no raise site, so this check proves nothing")
	}
	for name := range raised {
		if shapes[name] == nil {
			t.Errorf("%s is raised and no shape declares it", name)
		}
	}

	declared := map[string]bool{"refusal.unknown": true}
	for name, shape := range shapes {
		declared[refusalKeyOf(name)] = true
		if shape.Subject != "" {
			declared[refusalKeyOf(name)+".unnamed"] = true
		}
		for _, command := range shape.Variants {
			declared[shape.VariantKeyOf(command)] = true
		}
		for _, fragment := range shape.Fragments {
			declared[fragment.Key] = true
		}
	}
	for key := range base {
		if !strings.HasPrefix(key, "refusal.") || declared[key] {
			continue
		}
		t.Errorf("the base catalog carries %s and no shape names it as a base entry, a fragment or an unnamed sibling, so a sentence has drifted from its refusal", key)
	}
}

// refusalKeyOf is the base catalog key of a refusal name.
func refusalKeyOf(name string) string { return "refusal." + name }

// raisedRefusalNames reports every refusal name a raise site in the tree names
// through a contract constant.
//
// A name built by concatenation or held in a variable at the raise site is
// invisible to it, which is foldStringConcat's own documented gap and applies
// here unchanged.
func raisedRefusalNames(root string) (map[string]bool, error) {
	constants, err := contractConstants(root)
	if err != nil {
		return nil, err
	}
	legal := map[string]bool{}
	for _, name := range append(append([]string{}, contract.Declared...), contract.Introduced...) {
		legal[name] = true
	}
	raised := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
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
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !refusalRaisers[selector.Sel.Name] {
				return true
			}
			for _, arg := range call.Args {
				named, ok := arg.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				if value, known := constants[named.Sel.Name]; known && legal[value] {
					raised[value] = true
				}
			}
			return true
		})
		return nil
	})
	return raised, err
}

// checkOnePlaceBuildsARefusal is check 2: internal/contract builds every
// refusal, and the composer names no stream.
func checkOnePlaceBuildsARefusal(t *testing.T, root string) {
	t.Helper()
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
		fileSet := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		insideContract := strings.HasPrefix(name, "internal/contract/")
		ast.Inspect(file, func(node ast.Node) bool {
			composite, ok := node.(*ast.CompositeLit)
			if !ok || insideContract {
				return true
			}
			if refusalLiteralType(composite.Type) {
				t.Errorf("%s:%d builds a contract.Refusal literal, and contract.Refuse and contract.RefuseWith are where a refusal is built", name, fileSet.Position(composite.Pos()).Line)
			}
			return true
		})
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Name.Name != theComposer {
				continue
			}
			ast.Inspect(function, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if selector.Sel.Name != "out" && selector.Sel.Name != "errw" {
					return true
				}
				t.Errorf("%s:%d names a stream inside %s, and the composer returns lines so that the text path and the machine path share one composition", name, fileSet.Position(selector.Pos()).Line, theComposer)
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// refusalLiteralType reports whether a composite literal's type is
// contract.Refusal, written either through the package qualifier or as a
// pointer to it.
func refusalLiteralType(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		pkg, ok := e.X.(*ast.Ident)
		return ok && pkg.Name == "contract" && e.Sel.Name == "Refusal"
	case *ast.StarExpr:
		return refusalLiteralType(e.X)
	}
	return false
}

// checkEveryShapeSaysWhatToDoNext is check 3: a shape carries a next step or a
// stated reason and never both, and an alternation that could render none or
// two fails here rather than in front of a reader.
func checkEveryShapeSaysWhatToDoNext(t *testing.T, shapes map[string]*contract.Shape) {
	t.Helper()
	for _, name := range sortedShapeNames(shapes) {
		shape := shapes[name]
		hasNext := len(shape.NextStep) > 0
		hasReason := shape.NoNext != ""
		if hasNext && hasReason {
			t.Errorf("%s sets both NextStep and NoNext, and a refusal either says what to do next or columns why it cannot", name)
		}
		if !hasNext && !hasReason {
			t.Errorf("%s sets neither NextStep nor NoNext, so nothing says whether this refusal offers a next step", name)
		}
		seen := map[string]bool{}
		for i, key := range shape.NextStep {
			if seen[key] {
				t.Errorf("%s names %s twice in NextStep, and an alternation renders one member", name, key)
			}
			seen[key] = true
			fragment := shape.Fragment(key)
			if fragment == nil {
				t.Errorf("%s names %s in NextStep and declares no such fragment", name, key)
				continue
			}
			if i < len(shape.NextStep)-1 {
				continue
			}
			if fragment.When != "" || fragment.Unless != "" || fragment.WhenCommand != "" {
				t.Errorf("%s ends its NextStep on %s, which carries a condition, so a reader whose values match none of the branches gets no next step at all", name, key)
			}
		}
		for _, command := range shape.Variants {
			if !nextStepReaches(shape, command) {
				t.Errorf("%s declares the variant %s and names no fragment of that command in NextStep, so the variant's reader ends on the next step written for another act", name, command)
			}
		}
	}
}

// nextStepReaches reports whether one of a shape's alternation members is
// switched on by a command, which is what gives a variant a next step of its
// own rather than the one the shape's other readers get.
func nextStepReaches(shape *contract.Shape, command string) bool {
	for _, key := range shape.NextStep {
		fragment := shape.Fragment(key)
		if fragment != nil && fragment.WhenCommand == command {
			return true
		}
	}
	return false
}

// sortedShapeNames orders the table so a failure reads the same way twice.
func sortedShapeNames(shapes map[string]*contract.Shape) []string {
	names := make([]string, 0, len(shapes))
	for name := range shapes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// shapeEntry is one entry a shape declares: its base sentence, its unnamed
// sibling, or one of its fragments, with the condition value that fragment
// switches on.
type shapeEntry struct {
	key       string
	condition string
}

// checkNoPlaceholderIsStrayOrOrphaned is check 4, read over every entry a
// shape declares rather than over its base entry alone, because two of the
// splits move a base entry's only placeholder into its fragment.
func checkNoPlaceholderIsStrayOrOrphaned(t *testing.T, shapes map[string]*contract.Shape, base map[string]string) {
	t.Helper()
	for _, name := range sortedShapeNames(shapes) {
		shape := shapes[name]
		declared := map[string]bool{"detail": true}
		if shape.Subject != "" {
			declared[shape.Subject] = true
		}
		for _, value := range shape.Values {
			declared[value] = true
		}
		entries := []shapeEntry{{key: refusalKeyOf(name)}}
		if shape.Subject != "" {
			entries = append(entries, shapeEntry{key: refusalKeyOf(name) + ".unnamed"})
		}
		for _, command := range shape.Variants {
			entries = append(entries, shapeEntry{key: shape.VariantKeyOf(command)})
		}
		for _, fragment := range shape.Fragments {
			condition := fragment.When
			if condition == "" {
				condition = fragment.Unless
			}
			entries = append(entries, shapeEntry{key: fragment.Key, condition: condition})
		}
		used := map[string]bool{}
		for _, entry := range entries {
			text, ok := base[entry.key]
			if !ok {
				continue
			}
			for _, match := range refusalPlaceholder.FindAllStringSubmatch(text, -1) {
				slot := match[1]
				used[slot] = true
				if declared[slot] || slot == entry.condition {
					continue
				}
				t.Errorf("%s carries the slot {%s} and %s declares neither it nor a fragment condition of that name, so nothing fills it", entry.key, slot, name)
			}
		}
		for _, value := range shape.Values {
			if !used[value] {
				t.Errorf("%s declares the value %s and no entry of its own carries {%s}, so the name outlived the entry that used it", name, value, value)
			}
		}
	}
}

// unfollowedRefusalValues names the third argument of a refusal that check 5
// cannot resolve to the strings inside it, keyed by the file the refusal is
// raised in and the argument as it is written, and carrying the reason the
// tree keeps that shape. An entry is the one way such a site stays quiet, so a
// construct nobody has argued for is a finding rather than a silence, and an
// entry naming a shape the tree no longer has is reported too.
//
// A key is a file and an expression rather than a line, so an entry excuses
// every site in that file handing a refusal an argument written that way. That
// is deliberate for the two entries below, whose expressions each occur once,
// and it is the granularity to tighten first if a file ever grows a second
// site spelled identically.
var unfollowedRefusalValues = map[string]string{
	"cmd/dinah/render.go:s.outcomeValues(response)": "outcomeValues composes a response's named values in cmd/dinah/render.go, and a map built inside a called function is not followed into that function. Nothing else in the tree composes its named values that way, and that is a fact this check keeps rather than a claim it makes, since a second such site would be reported here instead of read past.",
	"internal/contract/contract.go:extra":           "With copies an existing refusal's Extra map forward and adds one caller-supplied value, so its contents are a container this check reads at the raise site that built it rather than at this one, and the added value is a parameter. Following it would mean following a struct field across calls, which is dataflow this check does not do. Every caller in the tree passes a path variable there, which grep -rn 'contract.With(' confirms, so no English literal reaches a reader through this site today.",
}

// checkNoEnglishLiteralReachesAPlaceholder is check 5. A value a reader sees
// is a catalog key rather than a phrase, and a phrase is what a string literal
// carrying a space is.
//
// It is scoped to the named-value map and to freeText's label on purpose.
// Widening it to the detail field would fire on the sites internal/bench
// already has, which dinah-146 carries rather than this card.
//
// The named-value map is recognised by the grammar of the refusal that
// consumes it rather than by the shape of the literal or by the name of the
// variable holding it. The map is whatever a RefuseWith call is handed as its
// third argument, and the check resolves that argument to the strings it
// carries along four routes:
//
//   - a map[string]string composite literal written out at the raise site;
//   - an identifier the enclosing function assigns, read through every
//     composite literal assigned to that name and through every value written
//     into it by index;
//   - an identifier a package-level var declares, read through that var's
//     composite literal and through every index write into the name anywhere
//     in the package's non-test files;
//   - an identifier the enclosing function takes as a map[string]string
//     parameter, read through the composite literals its callers in the same
//     package hand it in that position.
//
// Reaching a container is not the same as reading it, and dinah-282's third
// review turned on that difference. A route answers that it read the map only
// when every value reaching the name is a string literal this check examined.
// A value of any other shape, a name handed to another function that may fill
// it, and an argument no route describes are all one answer, which is that the
// check did not read the map, and that answer reports the site. An empty
// literal assigned to a name is therefore the least evidence this check can
// hold rather than the most, so a map made empty and filled by a helper, and a
// map populated by copying from a table, are both reported rather than counted
// as read.
//
// An argument the check did not read is not read past. It is reported at the
// raise site, and it goes quiet only when unfollowedRefusalValues names it
// with a reason. The number of constructs this check reads past is therefore
// kept by the code rather than asserted in this comment, which matters because
// dinah-282 wrote that number down twice and was wrong both times. The first
// attempt recognised a literal in an argument list and a literal assigned to
// the name extra, and it missed the same map under any other name. The second
// followed a local identifier and missed a package-level one. The third
// followed a local identifier to a container it never opened. Each missed
// spelling reached the same placeholder through the same refusal, and a check
// whose reach is the list its author thought of is the defect this card exists
// to remove.
//
// Three totals rather than one say whether the check did any work, because one
// counter cannot tell a check that found no refusal to examine from a check
// that examined nothing but empty containers. sites counts the refusals
// carrying named values, resolved counts those the check followed to their
// values, and values counts the individual strings it examined. Each total
// carries its own assertion and its own sentence.
//
// The routes over-reach rather than under-reach, and a guard is better wrong
// in that direction. An index write is matched by name across the whole
// package, so a local sharing a package-level map's name is attributed to that
// map. A caller is matched by function name, so a method sharing a function's
// name is read as a call to it. A name handed to any call other than the
// refusal that consumes it is treated as filled there, even where the callee
// only reads it. Each costs at worst a finding that quotes a real phrase in a
// real map, or that names a real site somebody then exempts with a reason.
func checkNoEnglishLiteralReachesAPlaceholder(t *testing.T, root string) {
	t.Helper()
	fileSet := token.NewFileSet()
	packages := map[string][]*refusalSource{}
	err := filepath.WalkDir(root, func(where string, entry fs.DirEntry, err error) error {
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
		relative, relErr := filepath.Rel(root, where)
		if relErr != nil {
			return relErr
		}
		parsed, parseErr := parser.ParseFile(fileSet, where, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		name := filepath.ToSlash(relative)
		directory := path.Dir(name)
		packages[directory] = append(packages[directory], &refusalSource{name: name, file: parsed})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	reporter := refusalReporter{t: t, fileSet: fileSet, seen: map[string]bool{}}
	excused := map[string]bool{}
	tally := refusalTally{}
	for _, directory := range sortedCatalogKeys(packages) {
		scope := readRefusalScope(packages[directory])
		for _, source := range packages[directory] {
			tally.add(readRefusalPhrases(reporter, source, scope, excused))
		}
	}
	if tally.sites == 0 {
		t.Error("no refusal in the tree was found carrying named values, so check 5 read nothing")
	}
	if tally.sites > 0 && tally.resolved == 0 {
		t.Errorf("check 5 found %d refusals carrying named values and followed none of them to a value, so every site it saw was reported or excused and it validated nothing", tally.sites)
	}
	if tally.resolved > 0 && tally.values == 0 {
		t.Errorf("check 5 followed %d refusals to their named values and every container it reached was empty, so it examined no string at all and would report no phrase however many the tree carried", tally.resolved)
	}
	for _, key := range sortedCatalogKeys(unfollowedRefusalValues) {
		if strings.TrimSpace(unfollowedRefusalValues[key]) == "" {
			t.Errorf("%s is held outside check 5's reach with no reason, which is a gap nobody has argued for", key)
		}
		if !excused[key] {
			t.Errorf("%s is held outside check 5's reach and every refusal in the tree resolves without it, so the entry outlived the site it excused", key)
		}
	}
}

// refusalSource is one non-test file of a package, kept alongside the name a
// finding quotes it by.
type refusalSource struct {
	name string
	file *ast.File
}

// refusalTally is how much work check 5 did, kept as three totals because one
// total cannot distinguish the two ways the check can do none. A tree carrying
// no refusal with named values leaves sites at zero, a tree whose every such
// refusal is reported or excused leaves resolved at zero, and a tree whose
// followed refusals all carry empty containers leaves values at zero. Each is
// a different defect and each gets its own sentence.
type refusalTally struct {
	// sites is the RefuseWith calls found carrying a named-value argument.
	sites int
	// resolved is the sites the check followed to the values inside them.
	resolved int
	// values is the individual strings the check examined.
	values int
}

// add accumulates one file's totals into the run's.
func (r *refusalTally) add(other refusalTally) {
	r.sites += other.sites
	r.resolved += other.resolved
	r.values += other.values
}

// refusalReach is what one resolution route learned about the name a refusal
// was handed. Reading nothing and reading a container that turned out to be
// empty are different answers, and treating the second as the first is the
// fail-open dinah-282's third review found.
type refusalReach int

const (
	// reachNone says the route does not describe this name at all, so the
	// next route gets its turn.
	reachNone refusalReach = iota
	// reachOpaque says the route found the name and could not follow every
	// value that reaches it, which is the answer that reports the site.
	reachOpaque
	// reachRead says every value reaching the name was a string literal this
	// check examined.
	reachRead
)

// refusalLiteral is a composite literal and the file it was written in, which
// is not always the file of the refusal that reaches it: a package-level map
// is declared in one file of a package and handed to a refusal in another.
type refusalLiteral struct {
	file    string
	literal *ast.CompositeLit
}

// refusalValue is one value written into a map by index, with the file it was
// written in, for the same reason.
type refusalValue struct {
	file  string
	value ast.Expr
}

// refusalCall is one call's argument list, with the file the call sits in.
type refusalCall struct {
	file      string
	arguments []ast.Expr
}

// refusalScope is what a package puts in the string maps its own level
// declares, and what its callers hand each of its functions. A refusal given a
// package-level name reaches everything the package wrote into that name, and
// a refusal given a parameter reaches whatever its callers passed.
// A package-level name can also be filled by a function this check does not
// enter, so escapes records every name the package hands to a call that is not
// the refusal consuming it.
type refusalScope struct {
	literals map[string]refusalLiteral
	writes   map[string][]refusalValue
	calls    map[string][]refusalCall
	escapes  map[string]bool
}

// readRefusalScope reads one package's files into the scope above.
func readRefusalScope(sources []*refusalSource) *refusalScope {
	scope := &refusalScope{
		literals: map[string]refusalLiteral{},
		writes:   map[string][]refusalValue{},
		calls:    map[string][]refusalCall{},
		escapes:  map[string]bool{},
	}
	for _, source := range sources {
		for _, declaration := range source.file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				valued, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, named := range valued.Names {
					if i >= len(valued.Values) {
						continue
					}
					literal, isLiteral := valued.Values[i].(*ast.CompositeLit)
					if isLiteral && stringMapType(literal.Type) {
						scope.literals[named.Name] = refusalLiteral{file: source.name, literal: literal}
					}
				}
			}
		}
		ast.Inspect(source.file, func(node ast.Node) bool {
			switch found := node.(type) {
			case *ast.AssignStmt:
				for i, target := range found.Lhs {
					if i >= len(found.Rhs) {
						continue
					}
					indexed, ok := target.(*ast.IndexExpr)
					if !ok {
						continue
					}
					if named, ok := indexed.X.(*ast.Ident); ok {
						scope.writes[named.Name] = append(scope.writes[named.Name], refusalValue{file: source.name, value: found.Rhs[i]})
					}
				}
			case *ast.CallExpr:
				if named := calleeName(found); named != "" {
					scope.calls[named] = append(scope.calls[named], refusalCall{file: source.name, arguments: found.Args})
				}
				if !fillableCall(found) {
					return true
				}
				for _, argument := range found.Args {
					if passed, ok := argument.(*ast.Ident); ok {
						scope.escapes[passed.Name] = true
					}
				}
			}
			return true
		})
	}
	return scope
}

// calleeName is the simple name a call names, which is the identifier for a
// plain call and the selected name for a method call. It is the name a
// same-package definition is found by, and it is spelled the way
// cmd/dinah/args_coverage_test.go spells the same question, since the two
// packages cannot share an unexported helper.
func calleeName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// refusalReporter writes check 5's one finding sentence, naming the file the
// phrase was found in rather than the file the refusal was raised in.
//
// It reports each phrase once. A map handed to two refusals in the same
// function, and a caller's literal read for each raise site inside the
// function it feeds, would otherwise repeat one finding as many times as the
// map is consumed, and the reader has one place to go either way.
type refusalReporter struct {
	t       *testing.T
	fileSet *token.FileSet
	seen    map[string]bool
}

// phrase reports one English literal reaching a reader.
func (r refusalReporter) phrase(file string, pos token.Pos, where, text string) {
	found := fmt.Sprintf("%s:%d:%s", file, r.fileSet.Position(pos).Line, text)
	if r.seen[found] {
		return
	}
	r.seen[found] = true
	r.t.Errorf("%s:%d passes the phrase %q as %s, and a value a reader sees reaches them through a catalog key rather than as an English literal", file, r.fileSet.Position(pos).Line, text, where)
}

// mapLiteral reports every phrase a named-value map spells out, and answers
// how many string literals it examined. A literal written out in front of the
// check carries its whole content there, so every element of it is examined
// whether or not the element turns out to be a literal.
func (r refusalReporter) mapLiteral(file string, literal *ast.CompositeLit) int {
	read := 0
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if r.value(file, pair.Value) {
			read++
		}
	}
	return read
}

// written reports the phrases written into a named-value map by index, and
// answers in the same terms mapLiteral does.
func (r refusalReporter) written(values []refusalValue) int {
	read := 0
	for _, value := range values {
		if r.value(value.file, value.value) {
			read++
		}
	}
	return read
}

// value examines one string reaching a named-value map, reporting it when it
// is a phrase, and answers whether it was a string literal.
//
// An expression of any other shape is examined and found not to be a literal,
// which is a real answer rather than a gap: a path from filepath.Join, a count
// from strconv.Itoa and a catalog lookup carry data to a reader and carry no
// English written into this map. What the check must not miss is a literal
// somebody wrote, and the routes above are what make sure every write it could
// arrive through is in front of this method.
func (r refusalReporter) value(file string, expr ast.Expr) bool {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	if text, ok := phraseLiteral(expr); ok {
		r.phrase(file, expr.Pos(), "a named value", text)
	}
	return true
}

// readRefusalPhrases reports one file's raise sites and returns how much work
// they gave the check, which is what tells the caller whether the check read
// anything at all and whether what it read carried anything.
func readRefusalPhrases(reporter refusalReporter, source *refusalSource, scope *refusalScope, excused map[string]bool) refusalTally {
	// freeText's label is checked in the same pass over the file, since it
	// reaches a reader the same way a named value does.
	ast.Inspect(source.file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if isSelector && selector.Sel.Name == "freeText" && len(call.Args) == 3 {
			if text, ok := phraseLiteral(call.Args[2]); ok {
				reporter.phrase(source.name, call.Args[2].Pos(), "a freeText label", text)
			}
		}
		return true
	})
	tally := refusalTally{}
	for _, declaration := range source.file.Decls {
		function, _ := declaration.(*ast.FuncDecl)
		ast.Inspect(declaration, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			argument := refusalValues(call)
			if argument == nil {
				return true
			}
			tally.sites++
			reach, read := resolveRefusalValues(argument, function, source.name, scope, reporter)
			if reach == reachRead {
				tally.resolved++
				tally.values += read
				return true
			}
			written := types.ExprString(argument)
			key := source.name + ":" + written
			if _, held := unfollowedRefusalValues[key]; held {
				excused[key] = true
				return true
			}
			reporter.t.Errorf("%s:%d hands a refusal its named values as %s, which check 5 cannot follow to the strings inside it, so an English phrase reaching a reader from there would pass unseen", source.name, reporter.fileSet.Position(argument.Pos()).Line, written)
			return true
		})
	}
	return tally
}

// resolveRefusalValues reads a refusal's third argument to the strings it
// carries, reporting each phrase it finds, and answers how far it got and how
// many strings it examined. Anything short of reachRead makes the caller
// report the construct instead of accepting it.
func resolveRefusalValues(argument ast.Expr, function *ast.FuncDecl, file string, scope *refusalScope, reporter refusalReporter) (refusalReach, int) {
	named, isName := argument.(*ast.Ident)
	if !isName {
		literal, isLiteral := argument.(*ast.CompositeLit)
		if !isLiteral || !stringMapType(literal.Type) {
			return reachNone, 0
		}
		// A literal at the raise site carries its whole content in
		// front of the check, so there is nowhere else for a string of
		// its to have come from.
		return reachRead, reporter.mapLiteral(file, literal)
	}
	// A refusal carrying no named values at all carries no phrase either.
	if named.Name == "nil" {
		return reachRead, 0
	}
	reach, read := readLocalMap(named.Name, function, file, scope, reporter)
	if reach == reachNone {
		reach, read = readPackageMap(named.Name, scope, reporter)
	}
	if reach == reachNone {
		reach, read = readParameterMap(named.Name, function, scope, reporter)
	}
	// Whichever route described the name, a function that hands it to
	// another call may be handing it somewhere it gets filled, and this
	// check does not enter that function.
	if reach == reachRead && handsOverName(function, named.Name) {
		return reachOpaque, read
	}
	return reach, read
}

// readLocalMap reads a name the enclosing function assigns, through the
// composite literals assigned to it and the values written into it by index.
//
// A map the function makes empty is reached, not read, and the difference is
// what dinah-282's third review turned on. The route answers reachRead only
// when every write that can put a string in the map is one it walked over, so
// an empty literal or a make with nothing else in the function is an empty map
// rather than an unexamined one, and a container filled from somewhere the
// walk does not go is reported. Two such places exist and each is closed
// below: a value the function assigns from an expression that is neither a
// string-map literal nor make, and a loop copying another container's contents
// in wholesale.
func readLocalMap(name string, function *ast.FuncDecl, file string, scope *refusalScope, reporter refusalReporter) (refusalReach, int) {
	if function == nil || function.Body == nil {
		return reachNone, 0
	}
	touched := false
	whole := true
	read := 0
	ast.Inspect(function.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, target := range assignment.Lhs {
			if i >= len(assignment.Rhs) {
				continue
			}
			switch addressed := target.(type) {
			case *ast.Ident:
				if addressed.Name != name {
					continue
				}
				touched = true
				if literal, ok := assignment.Rhs[i].(*ast.CompositeLit); ok && stringMapType(literal.Type) {
					read += reporter.mapLiteral(file, literal)
					continue
				}
				// make produces an empty map and puts no string
				// in it, so it leaves the reading to the writes.
				// A right-hand side of any other shape arrived
				// carrying contents this check never saw.
				if !makesStringMap(assignment.Rhs[i]) {
					whole = false
				}
			case *ast.IndexExpr:
				indexed, ok := addressed.X.(*ast.Ident)
				if !ok || indexed.Name != name {
					continue
				}
				touched = true
				read += reporter.written([]refusalValue{{file: file, value: assignment.Rhs[i]}})
			}
		}
		return true
	})
	for _, source := range copiedInto(function, name) {
		touched = true
		copied, isName := source.(*ast.Ident)
		if !isName {
			whole = false
			continue
		}
		reach, count := readPackageMap(copied.Name, scope, reporter)
		if reach != reachRead {
			whole = false
			continue
		}
		read += count
	}
	if !touched {
		return reachNone, 0
	}
	if !whole {
		return reachOpaque, read
	}
	return reachRead, read
}

// copiedInto returns what a loop copies wholesale into the named map. A write
// of a string literal is that literal and nothing more, however it is spelled,
// so a loop writing literals is not a copy and does not appear here. A write of
// anything else inside a range is a value taken from the ranged container, and
// that container's strings are the ones at stake, so the caller has to read it
// or report the site.
func copiedInto(function *ast.FuncDecl, name string) []ast.Expr {
	sources := []ast.Expr{}
	ast.Inspect(function.Body, func(node ast.Node) bool {
		loop, ok := node.(*ast.RangeStmt)
		if !ok || loop.Body == nil {
			return true
		}
		if copiesNonLiteral(loop.Body, name) {
			sources = append(sources, loop.X)
		}
		return true
	})
	return sources
}

// copiesNonLiteral reports whether a block writes something other than a string
// literal into the named map by index.
func copiesNonLiteral(block *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(block, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, target := range assignment.Lhs {
			if i >= len(assignment.Rhs) {
				continue
			}
			indexed, ok := target.(*ast.IndexExpr)
			if !ok {
				continue
			}
			addressed, isName := indexed.X.(*ast.Ident)
			if !isName || addressed.Name != name {
				continue
			}
			literal, isLiteral := assignment.Rhs[i].(*ast.BasicLit)
			if !isLiteral || literal.Kind != token.STRING {
				found = true
			}
		}
		return true
	})
	return found
}

// readPackageMap reads a name the package declares at its own level, which is
// the spelling a walk bounded by a function body cannot see. A package-level
// name the package hands to any other call can be filled there, so escapes
// leaves it unread for the same reason a local one is.
func readPackageMap(name string, scope *refusalScope, reporter refusalReporter) (refusalReach, int) {
	declared, ok := scope.literals[name]
	if !ok {
		return reachNone, 0
	}
	read := reporter.mapLiteral(declared.file, declared.literal)
	read += reporter.written(scope.writes[name])
	if scope.escapes[name] {
		return reachOpaque, read
	}
	return reachRead, read
}

// readParameterMap reads a name the enclosing function takes as a parameter,
// through the composite literals its callers in the same package hand it. One
// caller passing something this check cannot read leaves the parameter
// unresolved, since a map is only as read as its least readable source.
func readParameterMap(name string, function *ast.FuncDecl, scope *refusalScope, reporter refusalReporter) (refusalReach, int) {
	if function == nil || function.Type == nil || function.Type.Params == nil {
		return reachNone, 0
	}
	position, ok := parameterPosition(function.Type.Params, name)
	if !ok {
		return reachNone, 0
	}
	calls := scope.calls[function.Name.Name]
	if len(calls) == 0 {
		return reachOpaque, 0
	}
	read := 0
	whole := true
	for _, call := range calls {
		if position >= len(call.arguments) {
			whole = false
			continue
		}
		literal, ok := call.arguments[position].(*ast.CompositeLit)
		if !ok || !stringMapType(literal.Type) {
			whole = false
			continue
		}
		read += reporter.mapLiteral(call.file, literal)
	}
	if !whole {
		return reachOpaque, read
	}
	return reachRead, read
}

// handsOverName reports whether a function gives the named map to a call other
// than the refusal that consumes it. The callee may fill the map, and this
// check does not enter it, so the name stops being readable at that call.
func handsOverName(function *ast.FuncDecl, name string) bool {
	if function == nil || function.Body == nil {
		return false
	}
	handed := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !fillableCall(call) {
			return true
		}
		for _, argument := range call.Args {
			if passed, ok := argument.(*ast.Ident); ok && passed.Name == name {
				handed = true
			}
		}
		return true
	})
	return handed
}

// fillableCall reports whether a call could write into a map it is handed. The
// refusal consuming the map is the one call that reads it by construction, and
// len and cap cannot write, so those three are what a name survives being
// passed to.
func fillableCall(call *ast.CallExpr) bool {
	if refusalValues(call) != nil {
		return false
	}
	named, ok := call.Fun.(*ast.Ident)
	if ok && (named.Name == "len" || named.Name == "cap") {
		return false
	}
	return true
}

// parameterPosition is where a map[string]string parameter sits in a
// function's argument list, counting each name separately so that a grouped
// declaration lands on the right one. A parameter of any other type answers
// false, which leaves the refusal reported rather than read.
func parameterPosition(params *ast.FieldList, name string) (int, bool) {
	position := 0
	for _, field := range params.List {
		if len(field.Names) == 0 {
			position++
			continue
		}
		for _, declared := range field.Names {
			if declared.Name == name {
				return position, stringMapType(field.Type)
			}
			position++
		}
	}
	return 0, false
}

// makesStringMap reports whether an expression is make(map[string]string, ...),
// which is how a raise site builds a named-value map it then fills by index.
func makesStringMap(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	named, ok := call.Fun.(*ast.Ident)
	if !ok || named.Name != "make" {
		return false
	}
	return stringMapType(call.Args[0])
}

// refusalValues returns the argument a RefuseWith call is handed as its named
// values, and nil for every other call. That third argument is the one route a
// value reaches a placeholder by, so it is the whole of what check 5 follows.
//
// The name is matched both qualified and bare, since internal/contract raises
// its own refusals without the package prefix that every other package writes.
func refusalValues(call *ast.CallExpr) ast.Expr {
	if len(call.Args) != 3 {
		return nil
	}
	switch called := call.Fun.(type) {
	case *ast.SelectorExpr:
		if called.Sel.Name != "RefuseWith" {
			return nil
		}
	case *ast.Ident:
		if called.Name != "RefuseWith" {
			return nil
		}
	default:
		return nil
	}
	return call.Args[2]
}

// stringMapType reports whether a composite literal is a map[string]string,
// which is the shape of every named-value map a raise site builds.
func stringMapType(expr ast.Expr) bool {
	mapped, ok := expr.(*ast.MapType)
	if !ok {
		return false
	}
	key, keyOK := mapped.Key.(*ast.Ident)
	value, valueOK := mapped.Value.(*ast.Ident)
	return keyOK && valueOK && key.Name == "string" && value.Name == "string"
}

// phraseLiteral reports a string literal carrying a space, which is a phrase a
// person reads rather than a token or a key.
func phraseLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	text, err := strconv.Unquote(literal.Value)
	if err != nil || !strings.Contains(text, " ") {
		return "", false
	}
	return text, true
}

// checkNoEmptySubjectReachesASentence is check 6: a shape declaring a subject
// carries the sentence written for the case where that subject is absent.
func checkNoEmptySubjectReachesASentence(t *testing.T, shapes map[string]*contract.Shape, base map[string]string) {
	t.Helper()
	for _, name := range sortedShapeNames(shapes) {
		shape := shapes[name]
		if shape.Subject == "" {
			continue
		}
		if _, ok := base[refusalKeyOf(name)+".unnamed"]; !ok {
			t.Errorf("%s declares the subject %s and the base catalog carries no %s, so a card nobody holds would render a sentence with a hole in it", name, shape.Subject, refusalKeyOf(name)+".unnamed")
		}
	}
}

// baseKeysOf is every entry a shape's sentence can start from: its own base
// entry, its unnamed sibling, and one entry per per-command variant.
func baseKeysOf(shape *contract.Shape) []string {
	keys := []string{refusalKeyOf(shape.Name), refusalKeyOf(shape.Name) + ".unnamed"}
	for _, command := range shape.Variants {
		keys = append(keys, shape.VariantKeyOf(command))
	}
	return keys
}

// checkNoCatalogKeepsAClauseItHandedOver is check 7, and it is the one check
// here that reads catalogs rather than Go sources.
//
// It belongs beside the source checks rather than beside the rendering ones
// because a rendering assembled out of the entries agrees with them whether a
// clause appears once or twice. The comparison trims leading punctuation and
// whitespace from the fragment, since a fragment splices onto the sentence and
// carries the semicolon or comma that joins it.
//
// It reads every fragment of every shape rather than the fourteen the split
// touched, so a later card that moves a clause out of a base entry is held to
// the same rule without editing a list here.
func checkNoCatalogKeepsAClauseItHandedOver(t *testing.T, shapes map[string]*contract.Shape, catalogs []refusalCatalog) {
	t.Helper()
	for _, name := range sortedShapeNames(shapes) {
		shape := shapes[name]
		for _, fragment := range shape.Fragments {
			for _, catalog := range catalogs {
				clause := strings.TrimLeft(catalog.entries[fragment.Key], " ;,.")
				if clause == "" {
					continue
				}
				for _, key := range baseKeysOf(shape) {
					text, ok := catalog.entries[key]
					if !ok || !strings.Contains(text, clause) {
						continue
					}
					t.Errorf("%s: %s still carries the clause %s was given, so the reader sees it twice; the clause moves rather than being copied", catalog.tag, key, fragment.Key)
				}
			}
		}
	}
}

// theOneContainmentTable is the file the containment grammar is declared in.
const theOneContainmentTable = "internal/bench/containment.go"

// anchorConstants are the identifiers naming an entity kind's anchor file.
// The grammar is what says which kind mounts which collection, so a switch or
// a map keyed on one of these names is a second copy of the grammar.
var anchorConstants = map[string]bool{
	"ColumnAnchor":     true,
	"CardAnchor":       true,
	"CommentAnchor":    true,
	"AttachmentAnchor": true,
	"ItemAnchor":       true,
}

// TestTheContainmentGrammarIsDeclaredOnce asserts that no anchor constant
// appears inside a switch statement or a map literal anywhere but the table
// that declares the grammar.
//
// Four places carried a kind-to-anchor literal before the table existed: the
// walk below a card, the anchor switch at the foot of the entity resolver, the
// collections an ordinal is assigned within, and the collections a structural
// act can leave a sibling in. A fifth copy is what would make a declared
// extension kind reachable in one of them and invisible to the other four, so
// the rule is a guard rather than a comment.
//
// Test sources are outside the scan, since a test naming an anchor is reading
// what the tool wrote rather than deciding what contains what.
func TestTheContainmentGrammarIsDeclaredOnce(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(theOneContainmentTable))); err != nil {
		t.Fatalf("the one containment table is not where this guard exempts it: %v", err)
	}
	scanned := 0
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
		if filepath.ToSlash(relative) == theOneContainmentTable {
			return nil
		}
		scanned++
		for _, finding := range anchorsInDecisions(t, path) {
			t.Errorf("%s: %s, and %s is where the containment grammar is declared", filepath.ToSlash(relative), finding, theOneContainmentTable)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if scanned == 0 {
		t.Error("the walk read no source at all, so this guard proves nothing")
	}
}

// anchorsInDecisions reports every anchor constant one source names inside a
// switch statement or a map literal, with the line it stands on.
func anchorsInDecisions(t *testing.T, path string) []string {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.SwitchStmt:
			found = append(found, anchorsNamed(set, typed, "a switch statement")...)
		case *ast.TypeSwitchStmt:
			found = append(found, anchorsNamed(set, typed, "a switch statement")...)
		case *ast.CompositeLit:
			if _, isMap := typed.Type.(*ast.MapType); isMap {
				found = append(found, anchorsNamed(set, typed, "a map literal")...)
			}
		}
		return true
	})
	sort.Strings(found)
	return found
}

// anchorsNamed reports the anchor constants one node names, wherever they sit
// inside it, together with the line each stands on.
func anchorsNamed(set *token.FileSet, node ast.Node, what string) []string {
	var found []string
	ast.Inspect(node, func(inner ast.Node) bool {
		name := ""
		switch typed := inner.(type) {
		case *ast.SelectorExpr:
			name = typed.Sel.Name
		case *ast.Ident:
			name = typed.Name
		default:
			return true
		}
		if !anchorConstants[name] {
			return true
		}
		found = append(found, fmt.Sprintf("line %d names %s inside %s", set.Position(inner.Pos()).Line, name, what))
		return true
	})
	return found
}
