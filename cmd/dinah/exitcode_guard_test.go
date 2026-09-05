package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// exitTwoAllowed is the complete list of functions this tree permits to
// compute exit code 2 by hand, named by enclosing function rather than by line
// so that an edit above one of them does not silently retarget the guard.
//
// writeRefusal puts a genuine *contract.Refusal on stderr and answers its exit
// code, which is where reportError's own hand-computed refusal moved when the
// stderr half was split out for reshape's half-applied run. runMCP holds the
// mcp server's three startup gates, each of which answers one question with
// one meaning. Every other hand-written exit 2 is the defect dinah-346
// removed, which is a command answering both "the invocation was refused" and
// "the read found something" with one number.
var exitTwoAllowed = map[string]bool{"writeRefusal": true, "runMCP": true}

// exitTwoShapes are the two searches dinah-346 AC-7 names. The first is the
// spelling this tree actually uses, and the second covers a bare literal
// written any of the four ways a patch is likeliest to reach for.
var exitTwoShapes = []*regexp.Regexp{
	regexp.MustCompile(`contract\.ExitCode\(contract\.OutcomeRefused\)`),
	regexp.MustCompile(`os\.Exit\(2\)`),
	regexp.MustCompile(`^\s*return 2\s*(//.*)?$`),
	regexp.MustCompile(`\bcode\s*:?=\s*2\b`),
}

// exitTwoSite is one hand-computed exit code the sweep found, carrying enough
// to name the offending function in a failure message.
type exitTwoSite struct {
	// file is the path relative to the repository root.
	file string
	// line is the one-based line number the match sits on.
	line int
	// function is the enclosing function's name, empty when the match sits
	// outside every function body.
	function string
	// text is the matching source line, trimmed.
	text string
}

// TestNoCommandComputesExitTwoByHand is dinah-346 AC-7. It runs both of that
// criterion's searches over this branch's own source rather than trusting the
// list a spec recorded, and holds every hit against exitTwoAllowed. A command
// that starts computing exit 2 for itself fails here until whoever added it
// either stops doing so or writes the new function into that list on purpose,
// which is the point: the list is a decision somebody makes, not a line count
// somebody maintains.
//
// The two searches are scoped differently and deliberately. The first runs
// over cmd/dinah and internal alike, because the constant travels. The second
// runs over cmd/dinah alone, because no internal package calls os.Exit or
// returns a value used as the process's own exit status, and internal/verb's
// tree.go carries two unrelated "return 2" statements, a containment depth and
// an entity rank, that a tree-wide literal search would wrongly name.
//
// What this guard does not catch, stated here rather than left for a later
// reader to discover the hard way: an exit code built through a further layer
// of indirection. A local constant equal to 2, an arithmetic expression that
// evaluates to 2, and a value threaded through an extra function call before
// it reaches os.Exit all pass this test. It catches the shapes present in the
// tree today and the shapes a patch is likeliest to write by hand, and a green
// run is not a proof that nothing anywhere computes exit 2 some third way.
func TestNoCommandComputesExitTwoByHand(t *testing.T) {
	root := filepath.Join("..", "..")
	head := filepath.Join(root, "cmd", "dinah")

	constantSites := sweepForExitTwo(t, []string{head, filepath.Join(root, "internal")}, exitTwoShapes[:1])
	literalSites := sweepForExitTwo(t, []string{head}, exitTwoShapes[1:])
	found := append(constantSites, literalSites...)

	if len(constantSites) == 0 {
		t.Fatal("the first search matched nothing at all, so this guard asserted nothing; the spelling it looks for has changed")
	}
	for _, site := range found {
		if exitTwoAllowed[site.function] {
			continue
		}
		where := site.function
		if where == "" {
			where = "no function"
		}
		t.Errorf("%s:%d computes exit 2 by hand inside %s, which exitTwoAllowed does not name: %s", site.file, site.line, where, site.text)
	}

	covered := map[string]bool{}
	for _, site := range found {
		covered[site.function] = true
	}
	var missing []string
	for name := range exitTwoAllowed {
		if !covered[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("exitTwoAllowed names %v, and the sweep found no hand-computed exit 2 in them, so the list has outlived the code it excuses", missing)
	}
}

// sweepForExitTwo reads every non-test Go file beneath the given directories
// and reports each line matching one of the shapes, resolved to the function
// that encloses it. The resolution goes through the parser rather than through
// a brace count, so a match inside a nested literal or a long switch is still
// named by the function it really sits in.
func sweepForExitTwo(t *testing.T, dirs []string, shapes []*regexp.Regexp) []exitTwoSite {
	t.Helper()
	var sites []exitTwoSite
	for _, dir := range dirs {
		walk := func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			sites = append(sites, exitTwoSitesIn(t, path, shapes)...)
			return nil
		}
		if err := filepath.WalkDir(dir, walk); err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	return sites
}

// exitTwoSitesIn reports the matching lines of one file, each carrying the
// name of the function it sits in.
func exitTwoSitesIn(t *testing.T, path string, shapes []*regexp.Regexp) []exitTwoSite {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, text, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	type span struct {
		name          string
		opens, closes int
	}
	var spans []span
	for _, declared := range parsed.Decls {
		function, ok := declared.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		spans = append(spans, span{
			name:   function.Name.Name,
			opens:  fset.Position(function.Pos()).Line,
			closes: fset.Position(function.End()).Line,
		})
	}
	var sites []exitTwoSite
	for index, line := range strings.Split(string(text), "\n") {
		matched := false
		for _, shape := range shapes {
			if shape.MatchString(line) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		number := index + 1
		site := exitTwoSite{file: filepath.ToSlash(path), line: number, text: strings.TrimSpace(line)}
		for _, holds := range spans {
			if number >= holds.opens && number <= holds.closes {
				site.function = holds.name
				break
			}
		}
		sites = append(sites, site)
	}
	return sites
}
