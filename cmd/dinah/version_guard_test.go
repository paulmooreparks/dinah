package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// toolComparisonAllowed is the complete list of functions this tree permits to
// compare the tool's own release number, named by enclosing function rather
// than by line so that an edit above one of them does not silently retarget
// the guard.
//
// TestVersionCarriesTheConformanceClaim in internal/verb/beyond_test.go holds
// the two comparisons the rule allows. One asserts that the release number and
// the conformance claim are not the same string, which is the rule against
// conflating them rather than a gate on either. The other stamps a release
// number into the variable a release build overwrites and reads it back out of
// the report, which proves the plumbing carries it. Neither decides what the
// tool will do, and every other comparison of this value would.
var toolComparisonAllowed = map[string]bool{"TestVersionCarriesTheConformanceClaim": true}

// toolComparisonSite is one comparison of the tool's own release number that
// the sweep found, carrying enough to name the offending function in a failure
// message.
type toolComparisonSite struct {
	// file is the path relative to the repository root.
	file string
	// line is the one-based line number the comparison sits on.
	line int
	// function is the enclosing function's name, empty when the comparison
	// sits outside every function body.
	function string
	// text is the operator the comparison uses, which is what makes a failure
	// message readable without opening the file.
	text string
}

// TestNothingComparesTheToolsOwnReleaseNumber holds the rule three source
// comments already state and nothing enforced: the release number this binary
// reports is displayed and never compared. verb.ToolRelease reads "0.1.0" in
// every build from source and is overwritten only by a release build, so a
// gate reading it refuses every contributor's own binary while passing every
// released one, which is the failure a client reaches for it to avoid. The
// conformance claim and the storage format are what a client compares, and
// they carry that meaning for a second implementation as well as for this one.
//
// The sweep parses every Go file under cmd/dinah and internal, walks each
// binary expression whose operator compares two values, and reports one where
// an operand is the identifier ToolRelease or a selector ending in Tool. Test
// sources are inside the sweep rather than outside it, because a gate written
// into a test is a gate somebody will lift into the head, and because the one
// legitimate comparison in this tree lives in a test and is named in
// toolComparisonAllowed on purpose.
//
// What this guard does not catch, stated here rather than left for a later
// reader to discover the hard way. It does not see a value copied into an
// intermediate variable before the comparison, so `t := ToolRelease; if t <
// floor` passes. It does not see a comparison expressed through a call rather
// than an operator, so strings.Compare, a semantic-version helper or a
// membership test against a set of allowed release strings all pass. On the Go
// side it does not see a comparison reached through a further layer of
// indirection, such as a value returned from a function that closes over the
// field or forwards it. This is the class of limitation dinah-343 already
// tracks for guards that recognise a shape rather than a meaning, and a green
// run here is not a proof that nothing anywhere compares the release number
// some third way.
func TestNothingComparesTheToolsOwnReleaseNumber(t *testing.T) {
	root := filepath.Join("..", "..")
	dirs := []string{filepath.Join(root, "cmd", "dinah"), filepath.Join(root, "internal")}
	found := sweepForToolComparisons(t, dirs)

	if len(found) == 0 {
		t.Fatal("the sweep matched nothing at all, so this guard asserted nothing; the shape it looks for has changed")
	}
	for _, site := range found {
		if toolComparisonAllowed[site.function] {
			continue
		}
		where := site.function
		if where == "" {
			where = "no function"
		}
		t.Errorf("%s:%d compares the tool's own release number with %s inside %s, which toolComparisonAllowed does not name; the release number is displayed and never compared", site.file, site.line, site.text, where)
	}

	covered := map[string]bool{}
	for _, site := range found {
		covered[site.function] = true
	}
	var missing []string
	for name := range toolComparisonAllowed {
		if !covered[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("toolComparisonAllowed names %v, and the sweep found no comparison of the release number in them, so the list has outlived the source it excuses", missing)
	}
}

// sweepForToolComparisons reads every Go file beneath the given directories
// and reports each comparison of the tool's own release number, resolved to
// the function that encloses it.
func sweepForToolComparisons(t *testing.T, dirs []string) []toolComparisonSite {
	t.Helper()
	var sites []toolComparisonSite
	for _, dir := range dirs {
		walk := func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			sites = append(sites, toolComparisonsIn(t, path)...)
			return nil
		}
		if err := filepath.WalkDir(dir, walk); err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	return sites
}

// comparisonOperators are the six operators that compare two values. An
// assignment, an arithmetic expression and a logical connective are all
// binary expressions too, and none of them decides anything about a version.
var comparisonOperators = map[token.Token]bool{
	token.EQL: true,
	token.NEQ: true,
	token.LSS: true,
	token.LEQ: true,
	token.GTR: true,
	token.GEQ: true,
}

// toolComparisonsIn reports the comparisons of one file, each carrying the
// name of the function it sits in. The resolution goes through the parser
// rather than through a brace count, so a comparison inside a nested literal
// or a closure is still named by the function it really sits in.
func toolComparisonsIn(t *testing.T, path string) []toolComparisonSite {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var sites []toolComparisonSite
	for _, declared := range parsed.Decls {
		function, ok := declared.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		sites = append(sites, toolComparisonsUnder(fset, function.Body, filepath.ToSlash(path), function.Name.Name)...)
	}
	// A comparison outside every function body sits in a package-level
	// declaration, which the walk above does not reach.
	for _, declared := range parsed.Decls {
		if _, ok := declared.(*ast.FuncDecl); ok {
			continue
		}
		sites = append(sites, toolComparisonsUnder(fset, declared, filepath.ToSlash(path), "")...)
	}
	return sites
}

// toolComparisonsUnder walks one node and reports every comparison of the
// tool's own release number beneath it, attributed to the named function.
func toolComparisonsUnder(fset *token.FileSet, node ast.Node, path, function string) []toolComparisonSite {
	var sites []toolComparisonSite
	ast.Inspect(node, func(n ast.Node) bool {
		binary, ok := n.(*ast.BinaryExpr)
		if !ok || !comparisonOperators[binary.Op] {
			return true
		}
		if !readsToolRelease(binary.X) && !readsToolRelease(binary.Y) {
			return true
		}
		site := toolComparisonSite{
			file:     path,
			line:     fset.Position(binary.Pos()).Line,
			function: function,
			text:     binary.Op.String(),
		}
		sites = append(sites, site)
		return true
	})
	return sites
}

// readsToolRelease reports whether an operand reads the tool's own release
// number. Two shapes count: the identifier ToolRelease, which is the variable
// itself and the spelling a package-local read uses, and a selector whose
// final name is Tool, which covers verb.ToolRelease and every read of a
// version report's own member however the value holding it is named.
func readsToolRelease(operand ast.Expr) bool {
	switch expression := operand.(type) {
	case *ast.Ident:
		return expression.Name == "ToolRelease"
	case *ast.SelectorExpr:
		return expression.Sel.Name == "ToolRelease" || expression.Sel.Name == "Tool"
	}
	return false
}
