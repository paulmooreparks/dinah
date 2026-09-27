package bench

import (
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/seamguard"
)

// This file holds the call-graph guard of dinah-619's read seam, on the
// type-checked parse of internal/seamguard, which judges everything by the
// object go/types resolves a use to and nothing by its spelling. From every
// method of the package's types but Disk's and search's, from every init
// function, from every stored function literal and from every function or
// variable a body names as a value, nothing reachable makes a read by the
// member rule, and nothing reachable holds an expression of the seam's
// bottom, Disk, or uses one of Disk's methods, except the one function
// allowed to, (*Bench).source. A call of a method of Source is a leaf of the
// walk, because it is where a read goes through the seam. Over every function
// of the package, reachable or not, an expression of Disk appears only inside
// (*Bench).source or as an argument to a Source parameter (the placement
// rule), and no file carries a //go:linkname directive or a function declared
// without a body (the outside-Go rule).
//
// What this guard cannot see, each item with the plant under
// testdata/sourceguard/residue that reproduces it; seamguard's header states
// the residue of both guards. The guard is asserted to pass each residue
// plant, so this list cannot claim a gap the guard has closed.
//
//  1. A write used as a read: residue/mkdirprobe.go asks whether a folder
//     exists through the error of os.Mkdir.
//  2. A read delegated to a package that is neither judged nor this one:
//     residue/delegated.go runs a program through os/exec.
//  3. A handle of a judged type converted to an interface of a package that is
//     not judged and read through it: residue/ioreader.go reads an io.Reader.
//  4. A Disk handed as an argument to a Source parameter of a function that
//     stores it, by code no root reaches: residue/handedsource.go.
//  5. A second read of the same member inside an exempt function:
//     residue/samepair.go, run against a table extended by one pair for its
//     own function.

// seamExemptions are the reads the guard lets through, keyed on the FullName
// of the function that makes one and then on the member it reads, each with
// its reason. The pair is the key and not the function, because a reason
// argues for one read. The guard fails when a pair no longer occurs, so an
// exemption cannot outlive what it excuses.
var seamExemptions = map[string]map[string]string{
	"(*dinah/internal/bench.Bench).newlineFiles": { // retired spelling, named deliberately
		"path/filepath.WalkDir": "it lists files for dinah check and its newline repair; check is grounded, not routed, on the HTTP head " +
			"(internal/httphead/routes.go, GroundLaterCard), and the repair writes, so no workbench opened over a resident snapshot ever " +
			"reaches it; and WalkDir classifies the root with os.Lstat, which Source does not offer, so a conversion would " +
			"change which files a symbolic-link root yields on Disk",
	},
}

// residueExemptions extend seamExemptions for one residue plant alone: a
// plant cannot add a statement to a function trunk declares, so the plant
// declares its own function and the pair that excuses its first read.
var residueExemptions = map[string]map[string]map[string]string{
	"samepair.go": {
		"(*dinah/internal/bench.Bench).plantSamePair": {"os.ReadFile": "the plant's own excused read"}, // retired spelling, named deliberately
	},
}

// outsideTheRead are the types whose methods the walk does not start from,
// keyed on the type's object by its full name, each with its reason. Every
// other method of the package is a root, because a read hands out cards,
// items and the rest, and a method of one of those that read the disk would
// bypass the seam as surely as a method of the workbench itself.
var outsideTheRead = map[string]string{
	"dinah/internal/bench.Disk":   "it is the seam's bottom, whose methods read the filesystem by definition",
	"dinah/internal/bench.search": "it is discovery, which runs before any workbench is opened and finds one by reading the filesystem",
}

// seamKey is the FullName of the one function allowed to hold a Disk.
const seamKey = "(*dinah/internal/bench.Bench).source" // retired spelling, named deliberately

// typeName answers the full name of a type's object.
func typeName(tn *types.TypeName) string {
	return tn.Pkg().Path() + "." + tn.Name()
}

// isGuardRoot reports whether the walk starts from a node: every method whose
// receiver outsideTheRead does not name, and every init function, since an
// init runs before any method and can leave a read where a method will call
// it (dinah-619/comments/12 planted exactly that).
func isGuardRoot(n *seamguard.Node) bool {
	if n.Key == "dinah/internal/bench.init" {
		return true
	}
	if n.Receiver == nil {
		return false
	}
	_, outside := outsideTheRead[typeName(n.Receiver)]
	return !outside
}

// guardRun is one type-checked run of the guard over the package, with or
// without a planted file.
type guardRun struct {
	pkg        *seamguard.Package
	graph      *seamguard.Graph
	violations []seamguard.Violation
}

// runGuard type-checks the package with extra files and applies every rule.
func runGuard(t *testing.T, importer *seamguard.Importer, exempt map[string]map[string]string, extra ...string) guardRun {
	t.Helper()
	pkg, err := seamguard.Load(".", extra, seamguard.LoadOptions{Importer: importer})
	if err != nil {
		t.Fatal(err)
	}
	scope := pkg.Types.Scope()
	disk, _ := scope.Lookup("Disk").(*types.TypeName)
	source, _ := scope.Lookup("Source").(*types.TypeName)
	if disk == nil || source == nil {
		t.Fatalf("this package declares no Disk or no Source, so the guard has no seam to judge")
	}
	g := seamguard.BuildGraph(pkg, seamguard.GraphOptions{Bottom: disk, Leaf: source})
	var found []seamguard.Violation
	found = append(found, g.Violations(seamguard.Walk{Root: isGuardRoot, Exempt: exempt, Seam: seamKey})...)
	found = append(found, seamguard.Placement(pkg, disk, source, seamKey)...)
	found = append(found, seamguard.OutsideGo(pkg)...)
	return guardRun{pkg: pkg, graph: g, violations: found}
}

// belongs reports whether a violation belongs to a planted file: its File or
// its RootFile is that file's path.
func belongs(v seamguard.Violation, plant string) bool {
	abs, err := filepath.Abs(plant)
	if err != nil {
		return false
	}
	return filepath.Clean(v.File) == abs || filepath.Clean(v.RootFile) == abs
}

// TestTheBenchReadsOnlyThroughItsSource is dinah-619/criteria/13's bench half.
func TestTheBenchReadsOnlyThroughItsSource(t *testing.T) {
	if len(seamExemptions) != 1 || len(seamExemptions["(*dinah/internal/bench.Bench).newlineFiles"]) != 1 { // retired spelling, named deliberately
		t.Errorf("the exemption table holds %v, and section 2.3 of dinah-619 names exactly one read, newlineFiles's filepath.WalkDir", seamExemptions)
	}
	allowed := 0
	for _, members := range seamguard.Allowed {
		allowed += len(members)
	}
	if allowed != seamguard.AllowedSize || len(seamguard.Allowed) != len(seamguard.Judged) {
		t.Errorf("the allowlist names %d members over %d packages, wanted %d over the %d judged; widen it only with a reason per member", allowed, len(seamguard.Allowed), seamguard.AllowedSize, len(seamguard.Judged))
	}
	if len(outsideTheRead) != 2 {
		t.Errorf("the walk leaves out the methods of %d types, wanted the two named", len(outsideTheRead))
	}
	importer := seamguard.SourceImporter(token.NewFileSet())
	trunk := runGuard(t, importer, seamExemptions)

	roots := 0
	for _, n := range trunk.graph.Nodes {
		if isGuardRoot(n) {
			roots++
		}
	}
	if roots < 100 {
		t.Fatalf("found %d declarations to walk from, and the package declares far more, so the guard proves nothing", roots)
	}
	t.Logf("walked from %d declarations over %d nodes", roots, len(trunk.graph.Nodes))

	for name, members := range seamExemptions {
		n := trunk.graph.Nodes[name]
		for member := range members {
			occurs := false
			if n != nil {
				for _, read := range n.Reads {
					occurs = occurs || read.Member == member
				}
			}
			if !occurs {
				t.Errorf("%s is exempt to read %s but no longer does, so its exemption has outlived the read it excused; remove the entry", name, member)
			}
		}
	}
	for _, v := range trunk.violations {
		t.Errorf("%s: %s (via %s)", v.Root, v.What, v.Via)
	}

	planted, err := filepath.Glob(filepath.Join("testdata", "sourceguard", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(planted) != 27 {
		t.Fatalf("found %d planted files under testdata/sourceguard, wanted twenty-seven: the eighteen of the name-keyed guard and the nine section 12.1 of dinah-619 adds", len(planted))
	}
	// The residue this file's header states is planted too, and asserted to
	// pass, so the header cannot drift from what the guard does: a guard that
	// starts catching one has a header to narrow and a plant to move.
	residue, err := filepath.Glob(filepath.Join("testdata", "sourceguard", "residue", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(residue) != 5 {
		t.Fatalf("found %d planted files under testdata/sourceguard/residue, wanted five: a write used as a read, a delegated read, a read through an interface of another package, a Disk handed to a storing Source parameter, and a second read of an exempt pair", len(residue))
	}

	// Each plant type-checks the package afresh, so the plants run in
	// parallel, each through its own view over the one base importer.
	for _, plant := range planted {
		t.Run(filepath.Base(plant), func(t *testing.T) {
			t.Parallel()
			want := plantedWant(t, plant)
			run := runGuard(t, seamguard.NewImporter(importer, nil), seamExemptions, plant)
			var got []string
			caught := false
			for _, v := range run.violations {
				if belongs(v, plant) {
					got = append(got, v.What)
					caught = caught || strings.Contains(v.What, want)
				}
			}
			if !caught {
				t.Errorf("the planted file %s carries a violation (%s) and the guard did not catch it; it answered %q", filepath.Base(plant), want, got)
			}
		})
	}
	for _, plant := range residue {
		t.Run("residue/"+filepath.Base(plant), func(t *testing.T) {
			t.Parallel()
			exempt := seamExemptions
			if extra, ok := residueExemptions[filepath.Base(plant)]; ok {
				exempt = map[string]map[string]string{}
				for k, v := range seamExemptions {
					exempt[k] = v
				}
				for k, v := range extra {
					exempt[k] = v
				}
			}
			run := runGuard(t, seamguard.NewImporter(importer, nil), exempt, plant)
			for _, v := range run.violations {
				if belongs(v, plant) {
					t.Errorf("the residue plant %s is caught (%s), so the gap the header states is closed; move the plant among the caught ones and narrow the header", filepath.Base(plant), v.What)
				}
			}
		})
	}
}

// plantedWant reads the "// want: " line a planted file names its violation
// with.
func plantedWant(t *testing.T, path string) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse the planted file %s: %v", path, err)
	}
	for _, group := range file.Comments {
		for _, c := range group.List {
			if rest, ok := strings.CutPrefix(c.Text, "// want: "); ok {
				return rest
			}
		}
	}
	t.Fatalf("the planted file %s names no violation on a // want: line", path)
	return ""
}
