package verb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file holds dinah-619's guard over package verb's reads. A Library
// reads the workbench only through l.Bench, whose source is the disk for
// every head but dinah serve, where it may be a resident snapshot. So no
// function here reads the filesystem itself, and none calls a free function
// of package bench that reads through Disk, except the functions the table
// below names, each with its reason.

// libraryReadExemptions are the functions of this package allowed to read the
// filesystem or to call a Disk-binding free function of package bench. The
// guard fails when one no longer does, so an exemption cannot outlive what it
// excuses.
var libraryReadExemptions = map[string]string{
	"Init": "checks that the directory a new workbench is created in is empty, which is a directory outside any " +
		"workbench; init is grounded on the HTTP head",
	"readSource": "reads the definition a new workbench is instantiated from, a file or a directory the caller " +
		"named outside any workbench",
	"readReshapeSource": "reads the definition reshape applies, a file or a directory the caller named outside any " +
		"workbench; reshape is grounded on the HTTP head",
	"matchingAttachment": "compares a payload a reshape is about to write with one already present, inside a " +
		"write, which reads the disk as every act does",
	"openCandidate": "opens each workbench a root-scoped read finds under a directory; the HTTP head holds root " +
		"and max-depth back (internal/httphead/routes.go, rootScoped), so no request on a resident reaches it",
	"migrateOneVocabulary": "opens a workbench written in the retired vocabulary to migrate it, which writes and " +
		"is reached from dinah migrate alone",
}

// seamImportPath is the import path of the package the seam lives in.
const seamImportPath = "dinah/internal/bench"

// verbReadsBelowTheSeam are the os and filepath members dinah-619 section
// 2.3 defines as reads, keyed by import path. os.OpenFile is judged by the
// syntax of its flag argument.
var verbReadsBelowTheSeam = map[string]map[string]bool{
	"os":            {"ReadFile": true, "ReadDir": true, "Stat": true, "Lstat": true, "Open": true, "DirFS": true},
	"path/filepath": {"Walk": true, "WalkDir": true, "Glob": true},
}

// diskBindingReaders answers the exported free functions of package bench
// whose bodies name Disk, parsed from the package's sources rather than
// listed, so a reader converted later joins the set with no edit here.
func diskBindingReaders(t *testing.T) map[string]bool {
	t.Helper()
	sources, err := filepath.Glob(filepath.Join("..", path.Base(seamImportPath), "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	binding := map[string]bool{}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, source, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", source, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || !ast.IsExported(fn.Name.Name) {
				continue
			}
			names := false
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if ident, ok := node.(*ast.Ident); ok && ident.Name == "Disk" {
					names = true
				}
				return !names
			})
			if names {
				binding[fn.Name.Name] = true
			}
		}
	}
	return binding
}

// libraryReadFinding is one read, or one Disk-binding call, in one function.
type libraryReadFinding struct {
	function, what string
}

// libraryReads scans the named files and answers every read and every
// Disk-binding call they make, by function.
func libraryReads(t *testing.T, files []string, binding map[string]bool) []libraryReadFinding {
	t.Helper()
	fset := token.NewFileSet()
	var found []libraryReadFinding
	for _, name := range files {
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		imports := map[string]string{}
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			local := filepath.Base(path)
			if spec.Name != nil {
				local = spec.Name.Name
			}
			imports[local] = path
		}
		pathOf := func(sel *ast.SelectorExpr) string {
			qualifier, ok := sel.X.(*ast.Ident)
			if !ok || qualifier.Obj != nil {
				return ""
			}
			return imports[qualifier.Name]
		}
		writeFlag := func(expr ast.Expr) bool {
			wronly := false
			var constant func(ast.Expr) bool
			constant = func(e ast.Expr) bool {
				switch x := e.(type) {
				case *ast.ParenExpr:
					return constant(x.X)
				case *ast.BinaryExpr:
					return x.Op == token.OR && constant(x.X) && constant(x.Y)
				case *ast.SelectorExpr:
					if pathOf(x) != "os" || !strings.HasPrefix(x.Sel.Name, "O_") {
						return false
					}
					if x.Sel.Name == "O_WRONLY" {
						wronly = true
					}
					return true
				}
				return false
			}
			return constant(expr) && wronly
		}
		for _, decl := range file.Decls {
			var body ast.Node
			function := ""
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				body, function = d.Body, d.Name.Name
			case *ast.GenDecl:
				body, function = d, "package-level declaration"
			default:
				continue
			}
			writes := map[*ast.SelectorExpr]bool{}
			ast.Inspect(body, func(node ast.Node) bool {
				switch x := node.(type) {
				case *ast.CallExpr:
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok && pathOf(sel) == "os" && sel.Sel.Name == "OpenFile" && len(x.Args) >= 2 && writeFlag(x.Args[1]) {
						writes[sel] = true
					}
				case *ast.SelectorExpr:
					at := fset.Position(x.Pos())
					where := filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
					path := pathOf(x)
					switch {
					case path == "os" && x.Sel.Name == "OpenFile":
						if !writes[x] {
							found = append(found, libraryReadFinding{function, "os.OpenFile (not a constant write flag) at " + where})
						}
					case verbReadsBelowTheSeam[path][x.Sel.Name]:
						found = append(found, libraryReadFinding{function, filepath.Base(path) + "." + x.Sel.Name + " at " + where})
					case path == seamImportPath && x.Sel.Name == "Disk":
						found = append(found, libraryReadFinding{function, "names Disk at " + where})
					case path == seamImportPath && binding[x.Sel.Name]:
						found = append(found, libraryReadFinding{function, "names " + x.Sel.Name + ", which reads through Disk, at " + where})
					}
				}
				return true
			})
		}
	}
	return found
}

// TestTheLibraryReadsOnlyThroughTheBench is dinah-619/criteria/13's verb half.
func TestTheLibraryReadsOnlyThroughTheBench(t *testing.T) {
	binding := diskBindingReaders(t)
	t.Logf("%s declares %d exported free functions that read through Disk", seamImportPath, len(binding))
	if len(binding) < 30 {
		t.Fatalf("found %d Disk-binding readers in %s, and the seam converted far more, so the parse is reading the wrong files", len(binding), seamImportPath)
	}
	for _, name := range []string{"ReadText", "ListIDs", "Exists", "ReadJournal", "Open"} {
		if !binding[name] {
			t.Errorf("%s is not in the computed Disk-binding set, so the set is not what this guard thinks it is", name)
		}
	}
	if len(libraryReadExemptions) != 6 {
		t.Errorf("the exemption table holds %d entries, wanted 6", len(libraryReadExemptions))
	}

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var trunk []string
	for _, source := range sources {
		if !strings.HasSuffix(source, "_test.go") {
			trunk = append(trunk, source)
		}
	}
	if len(trunk) < 30 {
		t.Fatalf("found %d non-test sources, and this package has far more", len(trunk))
	}
	findings := libraryReads(t, trunk, binding)
	exempted := map[string]bool{}
	for _, f := range findings {
		if _, ok := libraryReadExemptions[f.function]; ok {
			exempted[f.function] = true
			continue
		}
		t.Errorf("%s: %s", f.function, f.what)
	}
	var stale []string
	for name := range libraryReadExemptions {
		if !exempted[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	for _, name := range stale {
		t.Errorf("%s is exempt but no longer reads the filesystem or calls a Disk-binding reader, so its exemption has outlived what it excused; remove the entry", name)
	}

	planted, err := filepath.Glob(filepath.Join("testdata", "readsource", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(planted) != 4 {
		t.Fatalf("found %d planted files under testdata/readsource, wanted the four dinah-619 section 12.2 names", len(planted))
	}
	for _, plant := range planted {
		caught := libraryReads(t, []string{plant}, binding)
		if len(caught) == 0 {
			t.Errorf("the planted file %s reads below the seam and the guard did not catch it", filepath.Base(plant))
		}
	}
}
