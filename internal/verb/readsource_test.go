package verb

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/seamguard"
)

// This file holds dinah-619's guard over package verb's reads, on the
// type-checked parse of internal/seamguard, which judges every use by the
// object go/types resolves it to and nothing by its spelling. A Library reads
// the workbench only through l.Bench, whose source is the disk for every head
// but dinah serve, where it may be a resident snapshot. Over verb's non-test
// files, taken flat (every declaration, signatures included), five rules
// apply, each outside the exemption table, and the outside-Go rule beside
// them:
//
//  1. The member rule: no use resolves to a read of a judged package.
//  2. The binding rule: no use resolves to a binding object of package bench,
//     an exported free function or variable from which a read or the bottom
//     is reachable in bench's own graph, or a method of Disk.
//  3. The bottom rule: no expression holds bench.Disk, and no identifier
//     resolves to the Disk type name or an alias of it, in any position.
//  4. The seam-opening rule: no use resolves to bench.OpenWith, so verb never
//     chooses a source.
//  5. The Source rule: no expression, object, use of the type name, type
//     argument or type-parameter constraint in verb has a type that holds a
//     Source-shaped type (sourceRule's comment defines it).
//
// What this guard cannot see, each item with the plant under
// testdata/readsource/residue that reproduces it; seamguard's header states
// the residue of both guards. The guard is asserted to pass each residue
// plant, so this list cannot claim a gap the guard has closed.
//
//  1. A write used as a read: residue/mkdirprobe.go asks whether a folder
//     exists through the error of os.Mkdir.
//  2. A read delegated to a package that is neither judged nor bench:
//     residue/delegated.go runs a program through os/exec.
//  3. A second read of the same member inside an exempt function:
//     residue/samepair.go, run against a table extended by one pair for its
//     own function.
//  4. A Source that reaches verb under a static type in which no
//     Source-shaped type appears, read by reflection: residue/reflectsource.go
//     takes one as any from its bench companion and calls ReadFile through
//     reflect.
//  5. A read through golang.org/x/sys/windows or golang.org/x/sys/unix, which
//     no plant reproduces for the reason seamguard's header gives.
//
// Verb and bench are judged together once for every distinct set of their
// files that a configuration in seamguard.Shipped builds, and the guard fails
// on any non-test Go file of either directory none of them builds. The
// planted files run under the host's configuration alone.

// libraryReadExemptions are the reads package verb may make below the seam,
// keyed on the FullName of the function that makes one and then on what it
// reads: a judged member, as "os.ReadFile", or a package bench object's
// FullName, as "dinah/internal/bench.GlobalInstructions", each with its
// reason. The key is the pair and not the function, because a reason argues
// for one read. The guard fails when a pair no longer occurs, so an exemption
// cannot outlive what it excuses.
var libraryReadExemptions = map[string]map[string]string{
	"dinah/internal/verb.Init": {
		"dinah/internal/bench.AnchorRecognized": "asks whether the directory a new workbench is created in already holds an anchor, " +
			"a directory outside any workbench; init is grounded on the HTTP head",
		"os.ReadDir": "checks that the directory a new workbench is created in is empty, a directory outside any " +
			"workbench; init is grounded on the HTTP head",
		"dinah/internal/bench.Instantiate": "writes the new workbench, an act grounded on the HTTP head",
	},
	"dinah/internal/verb.readSource": {
		"dinah/internal/bench.Exists": "asks whether the definition a new workbench is instantiated from is a workbench directory, " +
			"a path the caller named outside any workbench",
		"dinah/internal/bench.OpenUncontained": "opens that definition when it is a workbench directory, outside any workbench",
		"dinah/internal/durable.ReadFile":      "reads that definition when it is a file, outside any workbench",
	},
	"dinah/internal/verb.readReshapeSource": {
		"dinah/internal/bench.Exists": "asks whether the definition reshape applies is a workbench directory, a path the caller " +
			"named outside any workbench; reshape is grounded on the HTTP head",
		"dinah/internal/bench.OpenUncontained": "opens that definition when it is a workbench directory, outside any workbench",
		"dinah/internal/durable.ReadFile":      "reads that definition when it is a file, outside any workbench",
	},
	"dinah/internal/verb.matchingAttachment": {
		"dinah/internal/durable.ReadFile": "compares a payload a reshape is about to write with one already present, inside a write, " +
			"which reads the disk as every act does",
	},
	"dinah/internal/verb.openCandidate": {
		"dinah/internal/bench.Open": "opens each workbench a root-scoped read finds under a directory; the HTTP head holds root " +
			"and max-depth back (internal/httphead/routes.go, rootScoped), so no request on a resident reaches it",
	},
	"dinah/internal/verb.migrateOneVocabulary": {
		"dinah/internal/bench.ClassifyVocabulary": "reads the version of a workbench found by dinah migrate's sweep, reached from " +
			"dinah migrate alone",
		"dinah/internal/bench.OpenPreVocabulary": "opens a workbench written in the retired vocabulary to migrate it, reached " +
			"from dinah migrate alone",
		"dinah/internal/bench.MigrateVocabulary": "rewrites that workbench in the current vocabulary, a write reached from dinah " +
			"migrate alone",
	},
	"dinah/internal/verb.vocabularyCandidates": {
		"dinah/internal/bench.ScanContainers": "walks a directory tree for workbenches written in the retired vocabulary, which is " +
			"dinah migrate's sweep and reached from nothing else",
	},
	"dinah/internal/verb.MigrateContainerTree": {
		"dinah/internal/bench.ScanContainers": "sweeps a directory tree for workbenches to move into containers, which writes and " +
			"is reached from dinah migrate alone",
	},
	"dinah/internal/verb.migrateOneContainer": {
		"dinah/internal/bench.MigrateContainer": "moves one workbench into its container for MigrateContainerTree, a write " +
			"reached from dinah migrate alone",
	},
	"dinah/internal/verb.RemintWorkbench": {
		"dinah/internal/bench.Remint": "gives one workbench directory a fresh identifier, a repair reached from dinah migrate alone",
	},
	"dinah/internal/verb.forestCandidates": {
		"dinah/internal/bench.EnumerateDeep": "enumerates the workbenches under a root for a root-scoped read, as openCandidate " +
			"opens them; the HTTP head holds root and max-depth back (internal/httphead/routes.go, rootScoped)",
	},
	"(*dinah/internal/verb.Library).Attach": {
		"dinah/internal/bench.AddAttachment": "is the act itself, which reads the disk as every act does, and reads the file being " +
			"attached, which the caller named outside the workbench",
	},
	"(*dinah/internal/verb.Library).rewriteKeptColumns": {
		"dinah/internal/bench.ColumnAnchorText": "compares a column's anchor before and after a step of reshape, which writes under " +
			"its own lock and reads the disk as every act does; reshape is grounded on the HTTP head",
	},
	"(*dinah/internal/verb.Library).composeChain": {
		"dinah/internal/bench.GlobalInstructions": "reads the user-global instructions layer, a file under the Dinah home and " +
			"outside every workbench, which the resident does not hold and every serve reads from disk",
	},
	"(*dinah/internal/verb.Library).primeInstructions": {
		"dinah/internal/bench.GlobalInstructions": "reads the user-global instructions layer, for the reason composeChain does",
	},
	"(*dinah/internal/verb.Library).visibleViews": {
		"dinah/internal/bench.LoadUserViews": "reads the user's own views, a file under the Dinah home and outside every " +
			"workbench, which the resident does not hold",
	},
	"dinah/internal/verb.setting": {
		"dinah/internal/bench.ResolveWorkbenchSource": "reports where discovery would find a workbench, which climbs the " +
			"directories above any workbench before one is opened",
	},
}

// libraryReadExemptionCount is the number of pairs libraryReadExemptions
// carries, which the guard asserts so that a widened table is a visible edit.
const libraryReadExemptionCount = 25

// plantExemptions extend libraryReadExemptions for one planted file alone: a
// plant cannot add a statement to a function trunk declares, so the plant
// declares its own function and the pair that excuses one of its reads.
var plantExemptions = map[string]map[string]map[string]string{
	"pairplant.go": {
		"(*dinah/internal/verb.Library).plantPair": {"dinah/internal/bench.GlobalInstructions": "the plant's own excused read"},
	},
	"samepair.go": {
		"(*dinah/internal/verb.Library).plantSamePair": {"os.ReadFile": "the plant's own excused read"},
	},
}

// benchPath is the import path of the package the seam lives in.
const benchPath = "dinah/internal/bench"

// benchSeamKey is the FullName of the one function allowed to hold a Disk.
const benchSeamKey = "(*dinah/internal/bench.Bench).source" // retired spelling, named deliberately

// seam is package bench as one run of this guard sees it: the package, its
// Disk, Source and OpenWith objects, and its binding set.
type seam struct {
	pkg      *seamguard.Package
	disk     *types.TypeName
	source   *types.TypeName
	openWith types.Object
	// binding maps each binding object's FullName to how a read is reached
	// from it.
	binding map[string]string
}

// loadSeam type-checks package bench with any companion files, through base.
func loadSeam(t *testing.T, base *seamguard.Importer, extra ...string) *seam {
	t.Helper()
	pkg, err := seamguard.Load(filepath.FromSlash("../bench"), extra, seamguard.LoadOptions{Importer: base})
	if err != nil {
		t.Fatal(err)
	}
	scope := pkg.Types.Scope()
	s := &seam{pkg: pkg, binding: map[string]string{}}
	s.disk, _ = scope.Lookup("Disk").(*types.TypeName)
	s.source, _ = scope.Lookup("Source").(*types.TypeName)
	s.openWith = scope.Lookup("OpenWith")
	if s.disk == nil || s.source == nil || s.openWith == nil {
		t.Fatalf("the seam's package declares no Disk, Source or OpenWith, so the guard has no seam to judge")
	}
	g := seamguard.BuildGraph(pkg, seamguard.GraphOptions{Bottom: s.disk, Leaf: s.source})
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		var key string
		switch o := obj.(type) {
		case *types.Func:
			key = o.FullName()
		case *types.Var:
			key = "var " + benchPath + "." + o.Name()
		default:
			continue
		}
		if how, ok := g.Reaches(key, benchSeamKey); ok {
			s.binding[benchPath+"."+name] = how
		}
	}
	named := s.disk.Type().(*types.Named)
	for i := 0; i < named.NumMethods(); i++ {
		s.binding[named.Method(i).FullName()] = "a method of the seam's bottom"
	}
	return s
}

// libraryFinding is one violation in one function of verb.
type libraryFinding struct {
	// function is the FullName of the function it is in, or a package-level
	// declaration's kind and names, as "var plantS".
	function string
	// name is what an exemption is keyed on: a judged member, or a bench
	// object's FullName; a rule's own label for the rules no pair excuses.
	name string
	what string
	file string
}

// sourceRule judges rule 5. A type is Source-shaped when its underlying type
// is an interface whose method set (NumMethods and Method, which include the
// methods of embedded interfaces) contains at least one method with the name
// of a method of bench.Source and a signature types.Identical to that
// method's. That is the one definition, and no types.Implements call, in
// either direction, decides it. A type holds a Source-shaped type when it is
// one, or when one appears inside it at any depth: through a pointer, a
// slice, an array, a map's key or value, a channel, a function signature's
// parameters and results, a struct's fields verb can reach, and a named
// type's underlying type. The fields verb can reach are every field of a
// struct declared in verb, and every exported or embedded field of a struct
// declared elsewhere; an embedded field is kept whatever its own name's
// export status, because Go promotes its methods into the struct ("A field or
// method f of an embedded field in a struct x is called promoted if x.f is a
// legal selector that denotes that field or method f").
type sourceRule struct {
	methods []*types.Func
	verb    *types.Package
	memo    map[types.Type]bool
}

func newSourceRule(source *types.TypeName, verb *types.Package) *sourceRule {
	iface := source.Type().Underlying().(*types.Interface)
	r := &sourceRule{verb: verb, memo: map[types.Type]bool{}}
	for i := 0; i < iface.NumMethods(); i++ {
		r.methods = append(r.methods, iface.Method(i))
	}
	return r
}

// shaped reports whether a type is Source-shaped.
func (r *sourceRule) shaped(t types.Type) bool {
	iface, ok := types.Unalias(t).Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		for _, sm := range r.methods {
			if m.Name() == sm.Name() && types.Identical(m.Type(), sm.Type()) {
				return true
			}
		}
	}
	return false
}

// holds reports whether a type holds a Source-shaped type.
func (r *sourceRule) holds(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if done, ok := r.memo[t]; ok {
		return done
	}
	r.memo[t] = false
	result := r.shaped(t)
	if !result {
		switch u := t.(type) {
		case *types.Named:
			result = r.holds(u.Underlying())
		case *types.Pointer:
			result = r.holds(u.Elem())
		case *types.Slice:
			result = r.holds(u.Elem())
		case *types.Array:
			result = r.holds(u.Elem())
		case *types.Map:
			result = r.holds(u.Key()) || r.holds(u.Elem())
		case *types.Chan:
			result = r.holds(u.Elem())
		case *types.Signature:
			result = r.holds(u.Params()) || r.holds(u.Results())
		case *types.Tuple:
			for i := 0; i < u.Len() && !result; i++ {
				result = r.holds(u.At(i).Type())
			}
		case *types.Struct:
			for i := 0; i < u.NumFields() && !result; i++ {
				field := u.Field(i)
				if field.Pkg() == r.verb || field.Exported() || field.Embedded() {
					result = r.holds(field.Type())
				}
			}
		}
	}
	r.memo[t] = result
	return result
}

// libraryFindings applies every rule to verb and answers what it finds.
func libraryFindings(pkg *seamguard.Package, s *seam) []libraryFinding {
	info := pkg.Info
	rule := newSourceRule(s.source, pkg.Types)
	var found []libraryFinding
	seen := map[string]bool{}
	add := func(f libraryFinding) {
		key := f.function + "|" + f.name + "|" + f.what
		if !seen[key] {
			seen[key] = true
			found = append(found, f)
		}
	}
	for i, file := range pkg.Files {
		fileName := pkg.Names[i]
		for _, decl := range file.Decls {
			function := declName(info, decl)
			ast.Inspect(decl, func(node ast.Node) bool {
				if node == nil {
					return true
				}
				at := pkg.At(node.Pos())
				if e, ok := node.(ast.Expr); ok {
					if tv, ok := info.Types[e]; ok && tv.Type != nil {
						if seamguard.HoldsTheBottom(tv.Type, s.disk) {
							add(libraryFinding{function, "bottom rule", "bottom rule: " + function + " holds an expression of type " + types.TypeString(tv.Type, nil) + " at " + at, fileName})
						}
						if rule.holds(tv.Type) {
							add(libraryFinding{function, "Source rule", "Source rule: " + function + " holds an expression of type " + types.TypeString(tv.Type, nil) + " at " + at, fileName})
						}
					}
				}
				id, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				if def := info.Defs[id]; def != nil && rule.holds(def.Type()) {
					add(libraryFinding{function, "Source rule", "Source rule: " + function + " declares " + id.Name + " of type " + types.TypeString(def.Type(), nil) + " at " + at, fileName})
				}
				obj := info.Uses[id]
				if obj == nil {
					return true
				}
				if member, read := seamguard.Classify(obj); read {
					add(libraryFinding{function, member, "reads " + member + " at " + at, fileName})
				}
				if full := fullName(obj); full != "" {
					if how, binds := s.binding[full]; binds {
						add(libraryFinding{function, full, "names " + full + ", which reads below the seam (" + how + "), at " + at, fileName})
					}
				}
				if tn, ok := obj.(*types.TypeName); ok && types.Identical(tn.Type(), s.disk.Type()) {
					add(libraryFinding{function, "bottom rule", "bottom rule: " + function + " names the type " + benchPath + ".Disk at " + at, fileName})
				}
				if obj == s.openWith {
					add(libraryFinding{function, "seam-opening rule", "seam-opening rule: " + function + " names " + benchPath + ".OpenWith at " + at, fileName})
				}
				if obj == types.Object(s.source) {
					add(libraryFinding{function, "Source rule", "Source rule: " + function + " names the type " + benchPath + ".Source at " + at, fileName})
				}
				if obj.Type() != nil && rule.holds(obj.Type()) {
					add(libraryFinding{function, "Source rule", "Source rule: " + function + " uses " + id.Name + " of type " + types.TypeString(obj.Type(), nil) + " at " + at, fileName})
				}
				return true
			})
		}
	}
	for _, v := range seamguard.OutsideGo(pkg) {
		add(libraryFinding{"outside Go", "outside Go", v.What, v.File})
	}
	return found
}

// fullName answers the name an exemption or the binding set keys an object
// of package bench on: a function's or method's FullName, or a package
// variable's path and name.
func fullName(obj types.Object) string {
	if obj.Pkg() == nil || obj.Pkg().Path() != benchPath {
		return ""
	}
	switch o := obj.(type) {
	case *types.Func:
		return o.Origin().FullName()
	case *types.Var:
		if o.Parent() == o.Pkg().Scope() {
			return benchPath + "." + o.Name()
		}
	}
	return ""
}

// declName names a top-level declaration: a function's FullName, or a
// package-level declaration's keyword and names, as "var plantS".
func declName(info *types.Info, decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if fn, ok := info.Defs[d.Name].(*types.Func); ok {
			return fn.FullName()
		}
		return d.Name.Name
	case *ast.GenDecl:
		var names []string
		for _, spec := range d.Specs {
			switch sp := spec.(type) {
			case *ast.ValueSpec:
				for _, n := range sp.Names {
					names = append(names, n.Name)
				}
			case *ast.TypeSpec:
				names = append(names, sp.Name.Name)
			case *ast.ImportSpec:
				names = append(names, sp.Path.Value)
			}
		}
		return d.Tok.String() + " " + strings.Join(names, ", ")
	}
	return "declaration"
}

// unexcused answers the findings no exemption pair covers.
func unexcused(findings []libraryFinding, exempt map[string]map[string]string) []libraryFinding {
	var kept []libraryFinding
	for _, f := range findings {
		if _, ok := exempt[f.function][f.name]; !ok {
			kept = append(kept, f)
		}
	}
	return kept
}

// staleExemptions answers, as "function reads name", every exemption pair no
// libraryFinding occurs for.
func staleExemptions(findings []libraryFinding) []string {
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

// withPlant answers the exemption table extended by a plant's own pairs.
func withPlant(plant string) map[string]map[string]string {
	extra, ok := plantExemptions[filepath.Base(plant)]
	if !ok {
		return libraryReadExemptions
	}
	merged := map[string]map[string]string{}
	for k, v := range libraryReadExemptions {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	return merged
}

// companionOf answers the bench file a plant adds to package bench for its
// run, which sits beside it under seam/ with the same name, or "" when it has
// none.
func companionOf(plant string) string {
	companion := filepath.Join(filepath.Dir(plant), "seam", filepath.Base(plant))
	if _, err := os.Stat(companion); err != nil {
		return ""
	}
	return companion
}

// plantRun type-checks verb with one plant, against package bench with the
// plant's companion when it has one, and answers the unexcused findings that
// belong to the plant.
func plantRun(t *testing.T, base *seamguard.Importer, trunkSeam *seam, plant string) []libraryFinding {
	t.Helper()
	s := trunkSeam
	if companion := companionOf(plant); companion != "" {
		s = loadSeam(t, seamguard.NewImporter(base, nil), companion)
	}
	pkg, err := seamguard.Load(".", []string{plant}, seamguard.LoadOptions{Importer: seamguard.NewImporter(base, map[string]*types.Package{benchPath: s.pkg.Types})})
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(plant)
	if err != nil {
		t.Fatal(err)
	}
	var mine []libraryFinding
	for _, f := range unexcused(libraryFindings(pkg, s), withPlant(plant)) {
		if filepath.Clean(f.file) == abs {
			mine = append(mine, f)
		}
	}
	return mine
}

// TestTheLibraryReadsOnlyThroughTheBench is dinah-619/criteria/13's verb half.
func TestTheLibraryReadsOnlyThroughTheBench(t *testing.T) {
	base := seamguard.SourceImporter(token.NewFileSet())
	trunkSeam := loadSeam(t, base)
	t.Logf("%s declares %d binding objects", benchPath, len(trunkSeam.binding))
	if len(trunkSeam.binding) < 30 {
		t.Fatalf("found %d binding objects in %s, and the seam converted far more, so the parse is reading the wrong files", len(trunkSeam.binding), benchPath)
	}
	for _, name := range []string{"ReadText", "ListIDs", "Exists", "ReadJournal", "Open", "AddAttachment", "LoadUserViews", "CountIn", "MemberPosition"} {
		if _, ok := trunkSeam.binding[benchPath+"."+name]; !ok {
			t.Errorf("%s.%s is not in the computed binding set, so the set is not what this guard thinks it is", benchPath, name)
		}
	}
	pairs := 0
	for _, reads := range libraryReadExemptions {
		pairs += len(reads)
	}
	if pairs != libraryReadExemptionCount {
		t.Errorf("the exemption table holds %d pairs, wanted %d", pairs, libraryReadExemptionCount)
	}
	allowed := 0
	for _, members := range seamguard.Allowed {
		allowed += len(members)
	}
	if allowed != seamguard.AllowedSize {
		t.Errorf("the allowlist names %d members, wanted %d; widen it only with a reason per member", allowed, seamguard.AllowedSize)
	}

	// Package verb and the package bench it is judged against are loaded
	// together once for every distinct set of their files a shipped
	// configuration builds, so a file a build constraint keeps out of this
	// platform's build is still judged under one that builds it, and the
	// binding set is bench's own under that configuration.
	configs, loaded, unselected, err := seamguard.Distinct(seamguard.Shipped, ".", filepath.FromSlash("../bench"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range unselected {
		t.Errorf("no shipped configuration builds %s, so the guard judges it under none", name)
	}
	if len(configs) < 3 {
		t.Fatalf("the shipped configurations select %d distinct file sets from verb and %s, and the Windows, Linux and other Unix files of %s make at least three", len(configs), benchPath, benchPath)
	}
	t.Logf("judging %d files of verb and %s under %d configurations: %v", loaded, benchPath, len(configs), configs)
	var findings []libraryFinding
	reported := map[string]bool{}
	for _, config := range configs {
		configBase := seamguard.SourceImporterFor(token.NewFileSet(), config)
		configSeam := loadSeam(t, configBase)
		trunk, err := seamguard.Load(".", nil, seamguard.LoadOptions{Importer: seamguard.NewImporter(configBase, map[string]*types.Package{benchPath: configSeam.pkg.Types})})
		if err != nil {
			t.Fatal(err)
		}
		if len(trunk.Files) < 30 {
			t.Fatalf("%s: found %d non-test sources, and this package has far more", config, len(trunk.Files))
		}
		found := libraryFindings(trunk, configSeam)
		findings = append(findings, found...)
		for _, f := range unexcused(found, libraryReadExemptions) {
			if !reported[f.function+"|"+f.what] {
				reported[f.function+"|"+f.what] = true
				t.Errorf("%s: %s: %s", config, f.function, f.what)
			}
		}
	}
	for _, pair := range staleExemptions(findings) {
		t.Errorf("%s is exempt but no longer occurs under any shipped configuration, so its exemption has outlived what it excused; remove the entry", pair)
	}

	planted, err := filepath.Glob(filepath.Join("testdata", "readsource", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(planted) != 24 {
		t.Fatalf("found %d planted files under testdata/readsource, wanted twenty-four: the twenty-three section 12.2 of dinah-619 names, with durableread.go in ioutilread.go's place, and statplant.go, which dinah-619/questions/3 added", len(planted))
	}
	residue, err := filepath.Glob(filepath.Join("testdata", "readsource", "residue", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(residue) != 4 {
		t.Fatalf("found %d planted files under testdata/readsource/residue, wanted four: a write used as a read, a delegated read, a second read of an exempt pair, and a Source read by reflection", len(residue))
	}

	// An exemption whose read goes away is reported, pair by pair: drop
	// composeChain's one excused read and the stale check has to name it.
	var without []libraryFinding
	for _, f := range findings {
		if f.function != "(*dinah/internal/verb.Library).composeChain" {
			without = append(without, f)
		}
	}
	if stale := staleExemptions(without); len(stale) != 1 || stale[0] != "(*dinah/internal/verb.Library).composeChain reads dinah/internal/bench.GlobalInstructions" {
		t.Errorf("with composeChain's read of GlobalInstructions removed, the stale check answered %v, wanted that pair alone", stale)
	}

	// Each plant type-checks verb afresh, so the plants run in parallel, each
	// through its own view over the one base importer.
	for _, plant := range planted {
		t.Run(filepath.Base(plant), func(t *testing.T) {
			t.Parallel()
			want := plantedWant(t, plant)
			caught := plantRun(t, base, trunkSeam, plant)
			hit := false
			var got []string
			for _, f := range caught {
				got = append(got, f.what)
				hit = hit || strings.Contains(f.what, want)
			}
			if !hit {
				t.Errorf("the planted file %s carries a violation (%s) and the guard did not catch it; it answered %q", filepath.Base(plant), want, got)
			}
			if filepath.Base(plant) == "pairplant.go" && (len(caught) != 1 || caught[0].name != benchPath+".ReadText") {
				t.Errorf("the planted plantPair makes one excused read and one that is not, and the guard answered %q, wanted the read of ReadText alone", got)
			}
		})
	}
	for _, plant := range residue {
		t.Run("residue/"+filepath.Base(plant), func(t *testing.T) {
			t.Parallel()
			for _, f := range plantRun(t, base, trunkSeam, plant) {
				t.Errorf("the residue plant %s is caught (%s), so the gap the header states is closed; move the plant among the caught ones and narrow the header", filepath.Base(plant), f.what)
			}
		})
	}
}

// plantedWant reads the "// want: " line a planted file names its violation
// with.
func plantedWant(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "// want: "); ok {
			return rest
		}
	}
	t.Fatalf("the planted file %s names no violation on a // want: line", path)
	return ""
}
