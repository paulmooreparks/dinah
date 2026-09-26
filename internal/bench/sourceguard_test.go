package bench

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/seamguard"
)

// This file holds the call-graph guard of dinah-619's read seam: from every
// method of the package's types, *Bench and *Positions first among them, from
// every init function, from every function literal whose value is kept and
// from every function or variable named as a value, nothing reachable reads
// the filesystem except through a Source; no function body anywhere names a
// read as a value; and no type declaration names a judged type. The parse is
// internal/seamguard's, shared with package verb's guard, and it judges a
// read by allowlist: every member of os, io/ioutil, io/fs, path/filepath and
// syscall is a read unless seamguard.Allowed names it.
//
// What this guard cannot see, each with its reproduction. A read delegated to
// another package is invisible, as with a method calling a helper in a
// package of its own that calls os.ReadFile, since the parse reads this
// package's files alone. A value that reads, built by code no root reaches
// and handed to a method through a variable or a field, passes when neither
// its construction as the walk sees it nor its type names a judged member:
// testdata/sourceguard/residue/diskheld.go stores a Disk that way, and
// residue/filehandle.go hands in an *os.File. A write used as a read passes,
// since writes are allowlisted: residue/mkdirprobe.go asks whether a folder
// exists through the error of os.Mkdir. The guard is asserted to pass each
// residue plant, so this list cannot claim a gap the guard has closed.
// seamguard's header states the same residue.

// sourceMethodNames are Source's own methods. A selector naming one is a
// leaf of the walk, because it is where a read goes through the seam.
var sourceMethodNames = map[string]bool{
	"ReadFile": true, "ReadHead": true, "ReadDir": true, "Stat": true, "Text": true, "Derive": true,
}

// seamExemptions are the reads the guard lets through, keyed on the function
// that makes one and then on the member it reads, each with its reason. The
// pair is the key and not the function, because a reason argues for one read,
// and an exemption keyed on a function lets any read added beside it through
// (dinah-619/comments/15 walked one past package verb's guard that way). The
// guard fails when a pair no longer occurs, so an exemption cannot outlive
// what it excuses.
var seamExemptions = map[string]map[string]string{
	"newlineFiles": {
		"filepath.WalkDir": "it lists files for dinah check and its newline repair; check is grounded, not routed, on the HTTP head " +
			"(internal/httphead/routes.go, GroundLaterCard), and the repair writes, so no workbench opened over a resident snapshot ever " +
			"reaches it; and WalkDir classifies the root with os.Lstat, which Source does not offer, so a conversion would " +
			"change which files a symbolic-link root yields on Disk",
	},
}

// outsideTheRead are the receiver types whose methods the walk does not
// start from, each with its reason. Every other method of the package is a
// root, because a read hands out cards, items and the rest, and a method of
// one of those that read the disk would bypass the seam as surely as a method
// of the workbench itself; Card.Arrival did, until the guard started from it.
var outsideTheRead = map[string]string{
	"Disk":   "it is the seam's bottom, whose methods read the filesystem by definition",
	"search": "it is discovery, which runs before any workbench is opened and finds one by reading the filesystem",
}

// isGuardRoot reports whether the walk starts from a declaration: every
// method whose receiver outsideTheRead does not name, and every init
// function, since an init runs before any method and can leave a read where a
// method will call it (dinah-619/comments/12 planted exactly that).
func isGuardRoot(n *seamguard.Node) bool {
	if n.Key == "func:init" {
		return true
	}
	if !strings.HasPrefix(n.Key, "method:") {
		return false
	}
	_, outside := outsideTheRead[n.Receiver]
	return !outside
}

// buildGuardGraph parses the named files as one package and answers its graph.
func buildGuardGraph(t *testing.T, files []string) *seamguard.Graph {
	t.Helper()
	g, err := seamguard.Build(files, seamguard.Options{Leaf: sourceMethodNames, Bottom: "Disk"})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// guardViolations walks g with this guard's roots and exemptions.
func guardViolations(g *seamguard.Graph) []seamguard.Violation {
	return g.Violations(seamguard.Walk{Root: isGuardRoot, Exempt: seamExemptions, Seam: "method:source"})
}

// packageSources are this package's non-test Go files.
func packageSources(t *testing.T) []string {
	t.Helper()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package's sources: %v", err)
	}
	var kept []string
	for _, source := range sources {
		if !strings.HasSuffix(source, "_test.go") {
			kept = append(kept, source)
		}
	}
	if len(kept) < 50 {
		t.Fatalf("found %d non-test sources, and this package has far more, so the glob is reading the wrong directory", len(kept))
	}
	return kept
}

// TestTheBenchReadsOnlyThroughItsSource is dinah-619/criteria/13's bench half.
func TestTheBenchReadsOnlyThroughItsSource(t *testing.T) {
	if len(seamExemptions) != 1 || len(seamExemptions["newlineFiles"]) != 1 {
		t.Errorf("the exemption table holds %v, and section 2.3 of dinah-619 names exactly one read, newlineFiles's filepath.WalkDir", seamExemptions)
	}
	allowed := 0
	for _, members := range seamguard.Allowed {
		allowed += len(members)
	}
	if allowed != seamguard.AllowedSize || len(seamguard.Allowed) != len(seamguard.Judged) {
		t.Errorf("the allowlist names %d members over %d packages, wanted %d over the %d judged; widen it only with a reason per member", allowed, len(seamguard.Allowed), seamguard.AllowedSize, len(seamguard.Judged))
	}
	trunk := packageSources(t)
	g := buildGuardGraph(t, trunk)

	roots := 0
	for _, n := range g.Nodes {
		if isGuardRoot(n) {
			roots++
		}
	}
	if roots < 100 {
		t.Fatalf("found %d declarations to walk from, and the package declares far more, so the guard proves nothing", roots)
	}
	t.Logf("walked from %d declarations over %d", roots, len(g.Nodes))
	if len(outsideTheRead) != 2 {
		t.Errorf("the walk leaves out the methods of %d receiver types, wanted the two named", len(outsideTheRead))
	}

	for name, members := range seamExemptions {
		n := g.Nodes["method:"+name]
		if n == nil {
			n = g.Nodes["func:"+name]
		}
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

	for _, v := range guardViolations(g) {
		t.Errorf("%s: %s (via %s)", v.Root, v.What, v.Via)
	}

	planted, err := filepath.Glob(filepath.Join("testdata", "sourceguard", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(planted) != 18 {
		t.Fatalf("found %d planted files under testdata/sourceguard, wanted eighteen: the seven section 12.1 of dinah-619 names, the four shapes dinah-619/comments/12 walked past the first form of this guard, and the seven dinah-619/comments/15 walked past the second", len(planted))
	}
	for _, plant := range planted {
		want := plantedWant(t, plant)
		pg := buildGuardGraph(t, append(append([]string(nil), trunk...), plant))
		caught := false
		for _, v := range guardViolations(pg) {
			if (v.File == plant || strings.Contains(v.Via, "planted")) && strings.Contains(v.What, want) {
				caught = true
			}
		}
		if !caught {
			t.Errorf("the planted file %s carries a violation (%s) and the guard did not catch it", filepath.Base(plant), want)
		}
	}

	// The residue this file's header states is planted too, and asserted to
	// pass, so the header cannot drift from what the guard does: a guard that
	// starts catching one has a header to narrow and a plant to move.
	residue, err := filepath.Glob(filepath.Join("testdata", "sourceguard", "residue", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(residue) != 3 {
		t.Fatalf("found %d planted files under testdata/sourceguard/residue, wanted three: a Disk stored by code no root reaches, an *os.File handed in, and a write used as a read", len(residue))
	}
	for _, plant := range residue {
		pg := buildGuardGraph(t, append(append([]string(nil), trunk...), plant))
		for _, v := range guardViolations(pg) {
			if v.File == plant || strings.Contains(strings.ToLower(v.Via), "plant") {
				t.Errorf("the residue plant %s is caught (%s), so the gap the header states is closed; move the plant among the caught ones and narrow the header", filepath.Base(plant), v.What)
			}
		}
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
