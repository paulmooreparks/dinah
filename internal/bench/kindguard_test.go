package bench

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// kindAnswer is one line that answers a question about a state's kind for
// itself. Path is relative to the root the scan was given.
type kindAnswer struct {
	Path string
	Line int
	Text string
}

// theFourKinds are the kind literals a line answers for itself by naming. The
// scan reads the spellings rather than the constants, because a file that
// compares a bare string never mentions a constant at all.
var theFourKinds = map[string]bool{
	"intake":       true,
	"work":         true,
	"done":         true,
	"dinah.buffer": true,
}

// theFourConstants are the constant selectors that name a state kind. A line
// referring to one of them is answering a question about a kind for itself
// whatever it then does with the value.
var theFourConstants = map[string]bool{
	"KindIntake": true,
	"KindWork":   true,
	"KindDone":   true,
	"KindBuffer": true,
}

// scanForKindAnswers parses every .go file under root that is not a _test.go
// file and returns each line that decides a state's kind question for itself.
// It takes the root so the guard can read the product's own source while the
// test that proves the guard red reads a planted file somewhere else.
//
// A line answers for itself when it names one of the four kind constants, when
// it compares a selector ending in .Kind against one of the four kind literals,
// or when it stands in a case clause of a switch whose tag is such a selector
// and names one of those literals. The third shape is the one a text match
// misses, which is why the scan parses.
func scanForKindAnswers(root string) ([]kindAnswer, error) {
	var found []kindAnswer
	fset := token.NewFileSet()
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		lines, err := readLines(path)
		if err != nil {
			return err
		}
		at := func(node ast.Node) kindAnswer {
			line := fset.Position(node.Pos()).Line
			text := ""
			if line-1 < len(lines) {
				text = strings.TrimSpace(lines[line-1])
			}
			return kindAnswer{Path: filepath.ToSlash(relative), Line: line, Text: text}
		}
		seen := map[int]bool{}
		record := func(node ast.Node) {
			answer := at(node)
			if seen[answer.Line] {
				return
			}
			seen[answer.Line] = true
			found = append(found, answer)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.Ident:
				// An identifier rather than a selector, so the declaration in
				// the contract package and every reference from outside it are
				// read by one rule.
				if theFourConstants[n.Name] {
					record(n)
				}
			case *ast.BinaryExpr:
				if n.Op != token.EQL && n.Op != token.NEQ {
					return true
				}
				if comparesKindToLiteral(n) {
					record(n)
				}
			case *ast.SwitchStmt:
				if !endsInKind(n.Tag) {
					return true
				}
				for _, statement := range n.Body.List {
					clause, ok := statement.(*ast.CaseClause)
					if !ok {
						continue
					}
					for _, value := range clause.List {
						if namesAKind(value) {
							record(clause)
						}
					}
				}
			}
			return true
		})
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		return nil, err
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].Path != found[j].Path {
			return found[i].Path < found[j].Path
		}
		return found[i].Line < found[j].Line
	})
	return found, nil
}

// readLines returns a file's lines, so a finding can carry the source it was
// found on rather than a position a reader has to go and look up.
func readLines(path string) ([]string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n"), nil
}

// endsInKind reports whether an expression is a selector whose final name is
// Kind, which is the shape every state-kind question is asked in.
func endsInKind(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Kind"
}

// namesAKind reports whether an expression is a string literal holding one of
// the four kinds.
func namesAKind(expr ast.Expr) bool {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}
	return theFourKinds[value]
}

// comparesKindToLiteral reports whether a comparison puts a selector ending in
// .Kind on one side and one of the four kind literals on the other.
func comparesKindToLiteral(expr *ast.BinaryExpr) bool {
	if endsInKind(expr.X) && namesAKind(expr.Y) {
		return true
	}
	return endsInKind(expr.Y) && namesAKind(expr.X)
}

// kindExemption is one file the guard admits, together with why it answers a
// kind question for itself and how many lines it may do so on.
//
// The count is what keeps a whole file from going quiet behind a one-line
// exemption. internal/verb/beyond.go runs past a thousand lines and answers for
// itself on three of them, so an entry naming the file alone would take the
// guard off every other line in it. The precedent for asserting the list in
// both directions is uncoveredAllowlist in cmd/dinah/coverage_test.go.
type kindExemption struct {
	path   string
	lines  int
	reason string
}

// kindAllowlist holds the four files that declare or validate a state's kind,
// and no others. A fifth file wanting an entry here is a second answer to a
// question State.TakesWorkUp already answers, which is the defect this guard
// exists to catch; add the reader to the predicate rather than the file to this
// list.
// Only one of the four entries composes its path. The product's word is
// workbench, and a separate guard fails any Go string literal carrying the
// short one, so the entry naming this package's principal file cannot spell
// that file out. The other three can: the vocabulary guard strips this
// package's directory from a line before it reads it, which leaves check.go's
// literal carrying no hit at all.
func kindAllowlist(t *testing.T, root string) []kindExemption {
	t.Helper()
	return []kindExemption{
		{path: "internal/contract/contract.go", lines: 5, reason: "declares the four constants and the list of the kinds Dinah mints"},
		{path: ownPrincipalFile(t, root), lines: 3, reason: "holds the predicates every reader asks, and holds readState's vocabulary switch"},
		{path: "internal/bench/check.go", lines: 5, reason: "asks where a kind stands in the flow, which is a different question from whether work is taken up"},
		{path: "internal/verb/beyond.go", lines: 3, reason: "declares the kinds of the three states dinah init writes"},
	}
}

// ownPrincipalFile is the path, relative to the repository root, of the file
// in this package that holds the predicates. It is composed from two facts the
// test already has: the root, and the working directory, which go test sets to
// the package's own directory. Neither the module path nor the package name is
// written out, so a module rename and a package move both leave it correct.
//
// The composition does assume the principal file is named after its directory,
// which is true today and written down nowhere else. The stat below is what
// keeps a rename that breaks the assumption honest: it fails naming the file
// that is missing, rather than letting the guard report a stale entry and send
// the next reader after the wrong problem.
func ownPrincipalFile(t *testing.T, root string) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	relative, err := filepath.Rel(root, working)
	if err != nil {
		t.Fatalf("locate the package under %s: %v", root, err)
	}
	directory := filepath.ToSlash(relative)
	composed := directory + "/" + path.Base(directory) + ".go"
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(composed))); err != nil {
		t.Fatalf("the allow-list composed %s and no such file exists, so this package's principal file is no longer named after its directory: %v", composed, err)
	}
	return composed
}

// TestNoSecondAnswerAboutStateKind asserts that nothing outside the allow-list
// decides for itself what a state's kind means. One predicate answers that
// question and every reader asks it, and a copy of the rule is one line of
// ordinary-looking code that a review cannot be relied on to catch.
//
// The scan reads cmd/ and internal/ by name rather than walking the repository
// root, so an untracked worktree inside the tree never reaches it.
func TestNoSecondAnswerAboutStateKind(t *testing.T) {
	root := repositoryRoot(t)
	allowed := kindAllowlist(t, root)
	counted := map[string]int{}
	for _, directory := range []string{"cmd", "internal"} {
		found, err := scanForKindAnswers(filepath.Join(root, directory))
		if err != nil {
			t.Fatalf("scan %s: %v", directory, err)
		}
		for _, answer := range found {
			path := directory + "/" + answer.Path
			if exemptionFor(allowed, path) == nil {
				t.Errorf("%s:%d answers a question about a state's kind for itself, and no entry of kindAllowlist names it: %s",
					path, answer.Line, answer.Text)
				continue
			}
			counted[path]++
		}
	}
	for _, entry := range allowed {
		if counted[entry.path] > entry.lines {
			t.Errorf("%s carries %d kind-answering lines and its entry allows %d, so the file has grown a second answer behind its exemption",
				entry.path, counted[entry.path], entry.lines)
		}
		if counted[entry.path] < entry.lines {
			t.Errorf("%s carries %d kind-answering lines and its entry allows %d, so the entry is stale and has to be tightened",
				entry.path, counted[entry.path], entry.lines)
		}
	}
}

// exemptionFor returns the allow-list entry naming a path, or nil for a path no
// entry names.
func exemptionFor(allowed []kindExemption, path string) *kindExemption {
	for i := range allowed {
		if allowed[i].path == path {
			return &allowed[i]
		}
	}
	return nil
}

// TestTheKindGuardGoesRed proves the scan above can fail, by planting each
// shape it reads into a directory of its own and requiring the planted line
// back. A guard nobody has watched fail is a guard nobody knows works, and the
// fixture never lands in the tree, so the reproduction cannot trip the
// production scan.
func TestTheKindGuardGoesRed(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "a line naming a kind constant",
			source: `package planted

import "dinah/internal/contract"

func terminal(kind string) bool {
	return kind == contract.KindDone
}
`,
			want: "return kind == contract.KindDone",
		},
		{
			name: "a case clause of a switch on a bare kind string",
			source: `package planted

type state struct{ Kind string }

func takesWorkUp(s *state) bool {
	switch s.Kind {
	case "intake", "done":
		return false
	}
	return true
}
`,
			want: `case "intake", "done":`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "planted.go"), []byte(c.source), 0o644); err != nil {
				t.Fatalf("plant: %v", err)
			}
			found, err := scanForKindAnswers(dir)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			var texts []string
			for _, answer := range found {
				texts = append(texts, answer.Text)
			}
			for _, text := range texts {
				if text == c.want {
					return
				}
			}
			t.Errorf("the scan returned %v and the planted line %q is not among them", texts, c.want)
		})
	}
}

// repositoryRoot climbs from this package to the directory holding go.mod,
// which is what lets the guard name cmd and internal rather than walk whatever
// directory a test happens to run in.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if Exists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}
