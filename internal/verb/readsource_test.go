package verb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/seamguard"
)

// This file holds dinah-619's guard over package verb's reads. A Library
// reads the workbench only through l.Bench, whose source is the disk for
// every head but dinah serve, where it may be a resident snapshot. So no
// function here reads the filesystem itself, and none names an exported free
// function or variable of package bench from which a read is reachable,
// except the reads the table below names, each keyed on the function that
// makes it and the one member or bench name it reads, with its reason. The
// scan is flat, over every declaration of the package, signatures included,
// so a read stored as a value or in a literal is found where it is named. A
// read is judged by internal/seamguard's allowlist: every member of os,
// io/ioutil, io/fs, path/filepath and syscall is a read unless
// seamguard.Allowed names it.
//
// What this guard cannot see, with its reproduction. A read delegated to a
// package other than bench and the judged ones passes, as with a function
// here calling a helper package of its own that calls os.ReadFile, since the
// scan reads this package's files and bench's call graph alone. A write used
// as a read passes, as with os.IsExist(os.Mkdir(path, 0o755)) asking whether
// a folder exists, since writes are allowlisted. An *os.File handed in and
// read through its methods passes, since os.File is allowlisted as the type
// every write goes through. The bench graph is keyed by name, so a bench
// function reaching a method of a common name can join the binding set
// without reading; that errs toward refusing.

// libraryReadExemptions are the reads package verb may make below the seam,
// keyed on the function that makes one and then on what it reads or names, as
// "os.ReadFile" or "GlobalInstructions", each with its reason. The key is
// the pair and not the function, because a reason argues for one read: an
// exemption keyed on the function alone let any read added beside the excused
// one through, and a read of a workbench file planted in composeChain passed
// (dinah-619/comments/15). The guard fails when a pair no longer occurs, so an
// exemption cannot outlive what it excuses.
var libraryReadExemptions = map[string]map[string]string{
	"Init": {
		"AnchorRecognized": "asks whether the directory a new workbench is created in already holds an anchor, " +
			"a directory outside any workbench; init is grounded on the HTTP head",
		"os.ReadDir": "checks that the directory a new workbench is created in is empty, a directory outside any " +
			"workbench; init is grounded on the HTTP head",
		"Instantiate": "writes the new workbench, an act grounded on the HTTP head",
	},
	"readSource": {
		"Exists": "asks whether the definition a new workbench is instantiated from is a workbench directory, " +
			"a path the caller named outside any workbench",
		"OpenUncontained": "opens that definition when it is a workbench directory, outside any workbench",
		"os.ReadFile":     "reads that definition when it is a file, outside any workbench",
	},
	"readReshapeSource": {
		"Exists": "asks whether the definition reshape applies is a workbench directory, a path the caller " +
			"named outside any workbench; reshape is grounded on the HTTP head",
		"OpenUncontained": "opens that definition when it is a workbench directory, outside any workbench",
		"os.ReadFile":     "reads that definition when it is a file, outside any workbench",
	},
	"matchingAttachment": {
		"os.ReadFile": "compares a payload a reshape is about to write with one already present, inside a write, " +
			"which reads the disk as every act does",
	},
	"openCandidate": {
		"Open": "opens each workbench a root-scoped read finds under a directory; the HTTP head holds root " +
			"and max-depth back (internal/httphead/routes.go, rootScoped), so no request on a resident reaches it",
	},
	"migrateOneVocabulary": {
		"ClassifyVocabulary": "reads the version of a workbench found by dinah migrate's sweep, reached from " +
			"dinah migrate alone",
		"OpenPreVocabulary": "opens a workbench written in the retired vocabulary to migrate it, reached " +
			"from dinah migrate alone",
		"MigrateVocabulary": "rewrites that workbench in the current vocabulary, a write reached from dinah " +
			"migrate alone",
	},
	"vocabularyCandidates": {
		"ScanContainers": "walks a directory tree for workbenches written in the retired vocabulary, which is " +
			"dinah migrate's sweep and reached from nothing else",
	},
	"MigrateContainerTree": {
		"ScanContainers": "sweeps a directory tree for workbenches to move into containers, which writes and " +
			"is reached from dinah migrate alone",
	},
	"migrateOneContainer": {
		"MigrateContainer": "moves one workbench into its container for MigrateContainerTree, a write " +
			"reached from dinah migrate alone",
	},
	"RemintWorkbench": {
		"Remint": "gives one workbench directory a fresh identifier, a repair reached from dinah migrate alone",
	},
	"forestCandidates": {
		"EnumerateDeep": "enumerates the workbenches under a root for a root-scoped read, as openCandidate " +
			"opens them; the HTTP head holds root and max-depth back (internal/httphead/routes.go, rootScoped)",
	},
	"Attach": {
		"AddAttachment": "is the act itself, which reads the disk as every act does, and reads the file being " +
			"attached, which the caller named outside the workbench",
	},
	"rewriteKeptColumns": {
		"ColumnAnchorText": "compares a column's anchor before and after a step of reshape, which writes under " +
			"its own lock and reads the disk as every act does; reshape is grounded on the HTTP head",
	},
	"composeChain": {
		"GlobalInstructions": "reads the user-global instructions layer, a file under the Dinah home and " +
			"outside every workbench, which the resident does not hold and every serve reads from disk",
	},
	"primeInstructions": {
		"GlobalInstructions": "reads the user-global instructions layer, for the reason composeChain does",
	},
	"visibleViews": {
		"LoadUserViews": "reads the user's own views, a file under the Dinah home and outside every " +
			"workbench, which the resident does not hold",
	},
	"setting": {
		"ResolveWorkbenchSource": "reports where discovery would find a workbench, which climbs the " +
			"directories above any workbench before one is opened",
	},
}

// libraryReadExemptionCount is the number of pairs libraryReadExemptions
// carries, which the guard asserts so that a widened table is a visible edit.
const libraryReadExemptionCount = 25

// seamImportPath is the import path of the package the seam lives in.
const seamImportPath = "dinah/internal/bench"

// benchSources answers package bench's non-test files, and any extra files
// named, which a plant uses to add a declaration to bench for one run.
func benchSources(t *testing.T, extra ...string) []string {
	t.Helper()
	sources, err := filepath.Glob(filepath.Join("..", path.Base(seamImportPath), "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, source := range sources {
		if !strings.HasSuffix(source, "_test.go") {
			kept = append(kept, source)
		}
	}
	if len(kept) < 50 {
		t.Fatalf("found %d non-test sources in %s, and it has far more, so the glob is reading the wrong directory", len(kept), seamImportPath)
	}
	return append(kept, extra...)
}

// benchSourceMethods are bench.Source's own methods, which the bench graph
// treats as the seam: a selector naming one is a leaf.
var benchSourceMethods = map[string]bool{
	"ReadFile": true, "ReadHead": true, "ReadDir": true, "Stat": true, "Text": true, "Derive": true,
}

// diskBindingReaders answers the exported free functions and package
// variables of package bench from which a read is reachable, or the name
// Disk, computed over bench's own call graph rather than listed, so a reader
// added later joins the set with no edit here. The first form of this guard
// took only the functions whose bodies name Disk, and a free function reading
// with os.ReadFile itself walked past it (dinah-619/comments/12).
func diskBindingReaders(t *testing.T, sources []string) map[string]string {
	t.Helper()
	g, err := seamguard.Build(sources, seamguard.Options{Leaf: benchSourceMethods, Bottom: "Disk"})
	if err != nil {
		t.Fatal(err)
	}
	binding := map[string]string{}
	for key, n := range g.Nodes {
		if !strings.HasPrefix(key, "func:") && !strings.HasPrefix(key, "var:") {
			continue
		}
		if !ast.IsExported(n.Name()) {
			continue
		}
		if how, ok := g.Reaches(key, "method:source"); ok {
			binding[n.Name()] = how
		}
	}
	return binding
}

// libraryReadFinding is one read, or one Disk-binding call, in one function.
type libraryReadFinding struct {
	// name is what the function reads or names, which an exemption is keyed
	// on: a judged member with its package, as "os.ReadFile", and a name of
	// package bench bare, as "GlobalInstructions" or "Disk".
	function, name, what string
}

// libraryReads scans the named files and answers every read and every
// Disk-binding reference they make, by function. A read is judged by
// seamguard's allowlist.
func libraryReads(t *testing.T, files []string, binding map[string]string) []libraryReadFinding {
	t.Helper()
	fset := token.NewFileSet()
	var found []libraryReadFinding
	for _, name := range files {
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		imports, dotted := seamguard.Imports(file)
		for _, path := range dotted {
			found = append(found, libraryReadFinding{"import", "import " + path, "dot import of " + path + " in " + filepath.Base(name)})
		}
		for _, decl := range file.Decls {
			var body ast.Node
			function := ""
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				// The whole declaration, signature included: a parameter
				// of a judged type is a handle the body reads through.
				body, function = d, d.Name.Name
			case *ast.GenDecl:
				body, function = d, "package-level declaration"
			default:
				continue
			}
			for _, read := range seamguard.ScanReads(fset, imports, body).Reads {
				found = append(found, libraryReadFinding{function, read.Member, read.What})
			}
			ast.Inspect(body, func(node ast.Node) bool {
				x, ok := node.(*ast.SelectorExpr)
				if !ok || seamguard.ImportPathOf(imports, x) != seamImportPath {
					return true
				}
				at := fset.Position(x.Pos())
				where := filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
				switch how, binds := binding[x.Sel.Name]; {
				case x.Sel.Name == "Disk":
					found = append(found, libraryReadFinding{function, "Disk", "names Disk at " + where})
				case binds:
					found = append(found, libraryReadFinding{function, x.Sel.Name, "names " + x.Sel.Name + ", which reads below the seam (" + how + "), at " + where})
				}
				return true
			})
		}
	}
	return found
}

// TestTheLibraryReadsOnlyThroughTheBench is dinah-619/criteria/13's verb half.
func TestTheLibraryReadsOnlyThroughTheBench(t *testing.T) {
	binding := diskBindingReaders(t, benchSources(t))
	t.Logf("%s declares %d exported free functions and variables from which a read is reachable", seamImportPath, len(binding))
	if len(binding) < 30 {
		t.Fatalf("found %d binding readers in %s, and the seam converted far more, so the parse is reading the wrong files", len(binding), seamImportPath)
	}
	for _, name := range []string{"ReadText", "ListIDs", "Exists", "ReadJournal", "Open", "AddAttachment", "LoadUserViews", "CountIn", "MemberPosition"} {
		if _, ok := binding[name]; !ok {
			t.Errorf("%s is not in the computed binding set, so the set is not what this guard thinks it is", name)
		}
	}
	pairs := 0
	for _, reads := range libraryReadExemptions {
		pairs += len(reads)
	}
	if pairs != libraryReadExemptionCount {
		t.Errorf("the exemption table holds %d pairs, wanted %d", pairs, libraryReadExemptionCount)
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
	for _, f := range unexcused(findings) {
		t.Errorf("%s: %s", f.function, f.what)
	}
	for _, pair := range staleExemptions(findings) {
		t.Errorf("%s is exempt but no longer occurs, so its exemption has outlived what it excused; remove the entry", pair)
	}

	planted, err := filepath.Glob(filepath.Join("testdata", "readsource", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(planted) != 9 {
		t.Fatalf("found %d planted files under testdata/readsource, wanted nine: the four dinah-619 section 12.2 names, the four shapes dinah-619/comments/12 walked past the first form of this guard, and the read dinah-619/comments/15 added inside an exempt function", len(planted))
	}
	for _, plant := range planted {
		plantBinding := binding
		if companion := benchCompanion(plant); companion != "" {
			if _, ok := binding[plantedBenchReader]; ok {
				t.Fatalf("%s is in the binding set without its planted file, so the plant proves nothing", plantedBenchReader)
			}
			plantBinding = diskBindingReaders(t, benchSources(t, companion))
			if _, ok := plantBinding[plantedBenchReader]; !ok {
				t.Errorf("the planted file %s of the seam's package reads with os.ReadFile and the binding set did not take it in", filepath.Base(companion))
			}
		}
		caught := unexcused(libraryReads(t, []string{plant}, plantBinding))
		if len(caught) == 0 {
			t.Errorf("the planted file %s reads below the seam and the guard did not catch it", filepath.Base(plant))
		}
		if filepath.Base(plant) == "composechain.go" && (len(caught) != 1 || caught[0].name != "ReadText") {
			t.Errorf("the planted composeChain makes one excused read and one that is not, and the guard answered %v, wanted ReadText alone", caught)
		}
	}

	// An exemption whose read goes away is reported, pair by pair: drop
	// composeChain's one excused read and the stale check has to name it.
	var without []libraryReadFinding
	for _, f := range findings {
		if f.function != "composeChain" {
			without = append(without, f)
		}
	}
	if stale := staleExemptions(without); len(stale) != 1 || stale[0] != "composeChain reads GlobalInstructions" {
		t.Errorf("with composeChain's read of GlobalInstructions removed, the stale check answered %v, wanted that pair alone", stale)
	}
}

// unexcused answers the findings no exemption pair covers.
func unexcused(findings []libraryReadFinding) []libraryReadFinding {
	var kept []libraryReadFinding
	for _, f := range findings {
		if _, ok := libraryReadExemptions[f.function][f.name]; !ok {
			kept = append(kept, f)
		}
	}
	return kept
}

// staleExemptions answers, as "function reads name", every exemption pair no
// finding occurs for.
func staleExemptions(findings []libraryReadFinding) []string {
	occurs := map[string]bool{}
	for _, f := range findings {
		occurs[f.function+" reads "+f.name] = true
	}
	var stale []string
	for function, reads := range libraryReadExemptions {
		for name := range reads {
			if !occurs[function+" reads "+name] {
				stale = append(stale, function+" reads "+name)
			}
		}
	}
	sort.Strings(stale)
	return stale
}

// plantedBenchReader is the exported free function the bench companion of a
// plant declares: it reads with os.ReadFile and never names Disk.
const plantedBenchReader = "PlantedReadWithoutDisk"

// benchCompanion answers the bench file a plant adds to package bench for its
// run, which sits beside it under testdata/readsource/seam with the same
// name, or "" when it has none.
func benchCompanion(plant string) string {
	companion := filepath.Join(filepath.Dir(plant), "seam", filepath.Base(plant))
	if _, err := os.Stat(companion); err != nil {
		return ""
	}
	return companion
}
