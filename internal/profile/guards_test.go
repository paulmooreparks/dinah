package profile

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
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
	if strings.HasSuffix(name, "_test.go") && strings.Contains(source, theDeliberateSpelling) {
		return
	}
	stripped := subject
	for _, allowed := range vocabularyExceptions {
		stripped = strings.ReplaceAll(stripped, allowed, "")
	}
	stripped = theLongWord.ReplaceAllString(stripped, "")
	if !theShortWord.MatchString(stripped) {
		return
	}
	where := name
	if number > 0 {
		where = name + ":" + strconv.Itoa(number)
	}
	t.Errorf("%s carries the short word: %s\n%s", where, strings.TrimSpace(subject), vocabularyReason)
}

// theLongWord matches the spelling the product ruled for, removed from a line
// before the short one is looked for.
var theLongWord = regexp.MustCompile(`(?i)workbench`)

// literal is one Go string literal with the line it sits on.
type literal struct {
	// line is the one-based line the literal starts on.
	line int
	// text is the literal's source spelling, quotes included.
	text string
}

// goStringLiterals returns every string literal of a Go file. The parser is
// what separates a literal from a comment or an identifier, which a regular
// expression over the source could not do.
func goStringLiterals(path string) ([]literal, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var found []literal
	ast.Inspect(file, func(node ast.Node) bool {
		held, ok := node.(*ast.BasicLit)
		if !ok || held.Kind != token.STRING {
			return true
		}
		found = append(found, literal{line: fset.Position(held.Pos()).Line, text: held.Value})
		return true
	})
	return found, nil
}
