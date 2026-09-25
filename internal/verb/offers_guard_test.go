package verb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// offerSelectors walks every declaration of every non-test Go file in a
// directory, package-level declarations included, and answers the name of the
// enclosing function of every selector naming offerFor, called or not, with
// the empty string for one outside any function. It also answers which
// functions call columnOffers.
func offerSelectors(t *testing.T, dir string) (sites []string, callers map[string]bool) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	callers = map[string]bool{}
	files := 0
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files++
		for _, decl := range file.Decls {
			enclosing := ""
			if fn, ok := decl.(*ast.FuncDecl); ok {
				enclosing = fn.Name.Name
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch selector.Sel.Name {
				case "offerFor":
					sites = append(sites, enclosing)
				case "columnOffers":
					callers[enclosing] = true
				}
				return true
			})
		}
	}
	if files == 0 {
		t.Fatalf("no source file was read in %s, so the guard counted nothing", dir)
	}
	return sites, callers
}

// TestOneFunctionComputesWhatEveryColumnOffers is dinah-602/criteria/28. Next,
// Prime and the actionable scope each call columnOffers, and across every
// declaration of every non-test file of this package exactly one selector
// names offerFor, called or not, and it stands inside columnOffers.
//
// Counting every selector rather than every call is what catches the four
// easy ways round the rule: a second direct call, a method expression
// (*Library).offerFor, a method value l.offerFor assigned to a local and
// called, and a package-level variable holding the method expression. Each
// of those names offerFor where the source can see it.
//
// What this guard cannot see is a copy that reimplements the scan through
// headOfReadyFor without naming offerFor at all; such a copy passes here.
// TestTheAgendaHoldsWhatNextAndPrimeOffer is what holds the three answers
// equal over a fixture built to separate them, and it is the protection
// against that copy.
func TestOneFunctionComputesWhatEveryColumnOffers(t *testing.T) {
	sites, callers := offerSelectors(t, ".")
	if len(sites) != 1 || sites[0] != "columnOffers" {
		t.Errorf("offerFor is named at %d sites, in %v; want exactly one, inside columnOffers", len(sites), sites)
	}
	for _, caller := range []string{"Next", "Prime", "actionable"} {
		if !callers[caller] {
			t.Errorf("%s does not call columnOffers", caller)
		}
	}
}

// TestTheOfferGuardSeesEachNamingDodge arms the guard above against the four
// plants dinah-602/criteria/28 names, each written into a copy of this
// package's sources as the extra file it would be, and requires each to push
// the count past one. The copy is read from a directory of its own, so the
// package that compiles is never touched.
func TestTheOfferGuardSeesEachNamingDodge(t *testing.T) {
	plants := map[string]string{
		"a second direct call":               "func (l *Library) plantedDirect() {\n\t_, _ = l.offerFor(nil, nil, admission{})\n}",
		"a method expression":                "func plantedExpression(l *Library) {\n\t_, _ = (*Library).offerFor(l, nil, nil, admission{})\n}",
		"a method value assigned to a local": "func (l *Library) plantedValue() {\n\tscan := l.offerFor\n\t_, _ = scan(nil, nil, admission{})\n}",
		"a package-level variable":           "var plantedVariable = (*Library).offerFor",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package: %v", err)
	}
	for name, plant := range plants {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			copied := 0
			for _, entry := range entries {
				source := entry.Name()
				if entry.IsDir() || !strings.HasSuffix(source, ".go") || strings.HasSuffix(source, "_test.go") {
					continue
				}
				data, err := os.ReadFile(source)
				if err != nil {
					t.Fatalf("read %s: %v", source, err)
				}
				if err := os.WriteFile(filepath.Join(dir, source), data, 0o644); err != nil {
					t.Fatalf("copy %s: %v", source, err)
				}
				copied++
			}
			planted := "package verb\n\n" + plant + "\n"
			if err := os.WriteFile(filepath.Join(dir, "planted.go"), []byte(planted), 0o644); err != nil {
				t.Fatalf("write the plant: %v", err)
			}
			sites, _ := offerSelectors(t, dir)
			if copied == 0 || len(sites) < 2 {
				t.Errorf("the guard counted %d sites %v over %d copied files with %s planted, and should have counted two", len(sites), sites, copied, name)
			}
		})
	}
}
