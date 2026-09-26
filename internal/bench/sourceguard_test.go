package bench

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file holds the call-graph guard of dinah-619's read seam: from every
// method of the package's types, *Bench and *Positions first among them,
// nothing reachable reads the filesystem except through a Source. The graph is the name-keyed shape of
// internal/verb's packageCallGraph, refined in two ways the parser can afford
// without type information. A declaration is keyed by whether it is a
// function, a method or a package variable, so a free exported reader and the
// method of the same name that reads through the source are two nodes rather
// than one. A selector is an external reference when its qualifier names one
// of the file's imports and is not shadowed there, and a method reference
// otherwise.

// readsBelowTheSeam are the os and filepath members section 2.3 of dinah-619
// defines as reads, keyed by import path. os.OpenFile is judged separately,
// by the syntax of its flag argument.
var readsBelowTheSeam = map[string]map[string]bool{
	"os":            {"ReadFile": true, "ReadDir": true, "Stat": true, "Lstat": true, "Open": true, "DirFS": true},
	"path/filepath": {"Walk": true, "WalkDir": true, "Glob": true},
}

// sourceMethodNames are Source's own methods. A selector naming one is a
// leaf of the walk, because it is where a read goes through the seam.
var sourceMethodNames = map[string]bool{
	"ReadFile": true, "ReadHead": true, "ReadDir": true, "Stat": true, "Text": true, "Derive": true,
}

// seamExemptions are the functions the guard does not look inside, each with
// its reason. An exempt function is a leaf, and the guard fails when one no
// longer contains a read, so an exemption cannot outlive what it excuses.
var seamExemptions = map[string]string{
	"newlineFiles": "it lists files for dinah check and its newline repair; check is grounded, not routed, on the HTTP head " +
		"(internal/httphead/routes.go, GroundLaterCard), and the repair writes, so no workbench opened over a resident snapshot ever " +
		"reaches it; and WalkDir classifies the root with os.Lstat, which Source does not offer, so a conversion would " +
		"change which files a symbolic-link root yields on Disk",
}

// guardNode is one declaration of the graph.
type guardNode struct {
	key       string   // "func:Name", "method:Name" or "var:Name"
	file      string   // the file declaring it
	root      bool     // a method of a receiver outsideTheRead does not name
	edges     []string // keys this declaration references
	reads     []string // reads it makes itself, as "os.Stat at file:line"
	namesDisk bool     // its body names the identifier Disk
	diskAt    string
}

// guardGraph is every declaration of the parsed files, merged by key.
type guardGraph struct {
	nodes map[string]*guardNode
}

// buildGuardGraph parses the named files as one package and answers its graph.
func buildGuardGraph(t *testing.T, files []string) *guardGraph {
	t.Helper()
	fset := token.NewFileSet()
	type parsed struct {
		name string
		file *ast.File
	}
	var all []parsed
	declared := map[string]bool{}
	for _, name := range files {
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		all = append(all, parsed{name, file})
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				declared[declKey(d)] = true
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					for _, n := range spec.(*ast.ValueSpec).Names {
						declared["var:"+n.Name] = true
					}
				}
			}
		}
	}
	g := &guardGraph{nodes: map[string]*guardNode{}}
	node := func(key, file string) *guardNode {
		n, ok := g.nodes[key]
		if !ok {
			n = &guardNode{key: key, file: file}
			g.nodes[key] = n
		}
		return n
	}
	for _, p := range all {
		imports := map[string]string{}
		for _, spec := range p.file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			local := filepath.Base(path)
			if spec.Name != nil {
				local = spec.Name.Name
			}
			imports[local] = path
		}
		for _, decl := range p.file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				if receiverIs(d.Recv, "Disk") {
					// Disk is the seam's bottom: its methods read the
					// filesystem by definition, and every selector naming
					// one is already a leaf.
					continue
				}
				n := node(declKey(d), p.name)
				if d.Recv != nil && !notAWorkbenchRead(d.Recv) {
					n.root = true
				}
				scanGuardBody(fset, p.file, imports, declared, d.Body, n)
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					for _, name := range vs.Names {
						n := node("var:"+name.Name, p.name)
						for _, value := range vs.Values {
							scanGuardBody(fset, p.file, imports, declared, value, n)
						}
					}
				}
			}
		}
	}
	if len(g.nodes) == 0 {
		t.Fatal("the parse found no declaration, so the graph is empty and the guard proves nothing")
	}
	return g
}

// declKey keys a function declaration by whether it is a method.
func declKey(d *ast.FuncDecl) string {
	if d.Recv != nil {
		return "method:" + d.Name.Name
	}
	return "func:" + d.Name.Name
}

// receiverIs reports whether a receiver is the named type, by value or by
// pointer.
func receiverIs(recv *ast.FieldList, name string) bool {
	if recv == nil || len(recv.List) == 0 {
		return false
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
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

// notAWorkbenchRead reports whether a receiver is one of outsideTheRead.
func notAWorkbenchRead(recv *ast.FieldList) bool {
	for name := range outsideTheRead {
		if receiverIs(recv, name) {
			return true
		}
	}
	return false
}

// scanGuardBody records into n what one body references: reads, the name
// Disk, and every declaration it names, called or not.
func scanGuardBody(fset *token.FileSet, file *ast.File, imports map[string]string, declared map[string]bool, body ast.Node, n *guardNode) {
	writeOpens := map[*ast.SelectorExpr]bool{}
	var visit func(ast.Node) bool
	visit = func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && importPathOf(file, imports, sel) == "os" && sel.Sel.Name == "OpenFile" {
				if len(x.Args) >= 2 && isWriteFlag(file, imports, x.Args[1]) {
					writeOpens[sel] = true
				}
			}
			return true
		case *ast.SelectorExpr:
			if path := importPathOf(file, imports, x); path != "" {
				name := x.Sel.Name
				at := fset.Position(x.Pos())
				where := filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
				switch {
				case path == "os" && name == "OpenFile":
					if !writeOpens[x] {
						n.reads = append(n.reads, "os.OpenFile (not a constant write flag) at "+where)
					}
				case readsBelowTheSeam[path][name]:
					n.reads = append(n.reads, filepath.Base(path)+"."+name+" at "+where)
				}
				return false
			}
			if !sourceMethodNames[x.Sel.Name] && declared["method:"+x.Sel.Name] {
				n.edges = append(n.edges, "method:"+x.Sel.Name)
			}
			ast.Inspect(x.X, visit)
			return false
		case *ast.KeyValueExpr:
			// A key that is a bare identifier names a struct field, which
			// is no reference to the declaration of the same name.
			if _, ok := x.Key.(*ast.Ident); !ok {
				ast.Inspect(x.Key, visit)
			}
			ast.Inspect(x.Value, visit)
			return false
		case *ast.Ident:
			if !packageLevel(file, x) {
				return true
			}
			if x.Name == "Disk" {
				n.namesDisk = true
				at := fset.Position(x.Pos())
				n.diskAt = filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
			}
			if declared["func:"+x.Name] {
				n.edges = append(n.edges, "func:"+x.Name)
			}
			if declared["var:"+x.Name] {
				n.edges = append(n.edges, "var:"+x.Name)
			}
		}
		return true
	}
	ast.Inspect(body, visit)
}

// packageLevel reports whether an identifier refers to a package-level name:
// one the parser left unresolved, which is a name declared in another file of
// the package or a predeclared one, or one resolved into this file's scope.
func packageLevel(file *ast.File, ident *ast.Ident) bool {
	if ident.Obj == nil {
		return true
	}
	return file.Scope.Lookup(ident.Name) == ident.Obj
}

// importPathOf answers the import path a selector's qualifier names, or ""
// when the qualifier is not an unshadowed import of the file.
func importPathOf(file *ast.File, imports map[string]string, sel *ast.SelectorExpr) string {
	qualifier, ok := sel.X.(*ast.Ident)
	if !ok || qualifier.Obj != nil {
		return ""
	}
	return imports[qualifier.Name]
}

// isWriteFlag reports whether an os.OpenFile flag argument is an expression
// of os.O_* identifiers joined by | that names os.O_WRONLY.
func isWriteFlag(file *ast.File, imports map[string]string, expr ast.Expr) bool {
	wronly := false
	var constant func(ast.Expr) bool
	constant = func(e ast.Expr) bool {
		switch x := e.(type) {
		case *ast.ParenExpr:
			return constant(x.X)
		case *ast.BinaryExpr:
			return x.Op == token.OR && constant(x.X) && constant(x.Y)
		case *ast.SelectorExpr:
			if importPathOf(file, imports, x) != "os" || !strings.HasPrefix(x.Sel.Name, "O_") {
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

// seamViolation is one reach of a read, or of the name Disk, from a root.
type seamViolation struct {
	root, via, what string
}

// violations walks from every root and answers what each reaches that it
// must not. An exempt declaration is a leaf and is not itself a root.
func (g *guardGraph) violations(exempt map[string]string) []seamViolation {
	var found []seamViolation
	seen := map[string]bool{}
	var roots []string
	for key, n := range g.nodes {
		if n.root {
			roots = append(roots, key)
		}
	}
	sort.Strings(roots)
	for _, root := range roots {
		if _, ok := exempt[strings.TrimPrefix(root, "method:")]; ok {
			continue
		}
		reached := map[string]string{root: root}
		queue := []string{root}
		for len(queue) > 0 {
			key := queue[0]
			queue = queue[1:]
			n := g.nodes[key]
			if n == nil {
				continue
			}
			name := key[strings.Index(key, ":")+1:]
			if _, ok := exempt[name]; ok && key != root {
				continue
			}
			for _, read := range n.reads {
				tag := key + " " + read
				if !seen[tag] {
					seen[tag] = true
					found = append(found, seamViolation{root: root, via: reached[key], what: key + " reads " + read})
				}
			}
			if n.namesDisk && key != "method:source" {
				tag := key + " Disk"
				if !seen[tag] {
					seen[tag] = true
					found = append(found, seamViolation{root: root, via: reached[key], what: key + " names Disk at " + n.diskAt})
				}
			}
			for _, next := range n.edges {
				if target := g.nodes[next]; target != nil && next != "method:source" && target.namesDisk {
					tag := key + " > " + next
					if !seen[tag] {
						seen[tag] = true
						found = append(found, seamViolation{root: root, via: reached[key], what: key + " references " + next + ", which names Disk"})
					}
				}
				if _, ok := reached[next]; ok {
					continue
				}
				reached[next] = reached[key] + " > " + next
				queue = append(queue, next)
			}
		}
	}
	return found
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
	if len(seamExemptions) != 1 {
		t.Errorf("the exemption table holds %d entries, and section 2.3 of dinah-619 names exactly one, newlineFiles", len(seamExemptions))
	}
	trunk := packageSources(t)
	g := buildGuardGraph(t, trunk)

	roots := 0
	for _, n := range g.nodes {
		if n.root {
			roots++
		}
	}
	if roots < 100 {
		t.Fatalf("found %d methods to walk from, and the package declares far more, so the guard proves nothing", roots)
	}
	t.Logf("walked from %d methods over %d declarations", roots, len(g.nodes))
	if len(outsideTheRead) != 2 {
		t.Errorf("the walk leaves out the methods of %d receiver types, wanted the two named", len(outsideTheRead))
	}

	for name := range seamExemptions {
		n := g.nodes["method:"+name]
		if n == nil {
			n = g.nodes["func:"+name]
		}
		if n == nil || len(n.reads) == 0 {
			t.Errorf("%s is exempt from the read seam but no longer reads anything itself, so its exemption has outlived the read it excused; remove the entry", name)
		}
	}

	for _, v := range g.violations(seamExemptions) {
		t.Errorf("%s: %s (via %s)", v.root, v.what, v.via)
	}

	planted, err := filepath.Glob(filepath.Join("testdata", "sourceguard", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(planted) != 7 {
		t.Fatalf("found %d planted files under testdata/sourceguard, wanted the seven section 12.1 of dinah-619 names", len(planted))
	}
	for _, plant := range planted {
		want := plantedWant(t, plant)
		pg := buildGuardGraph(t, append(append([]string(nil), trunk...), plant))
		caught := false
		for _, v := range pg.violations(seamExemptions) {
			if strings.HasPrefix(v.root, "method:planted") && strings.Contains(v.what, want) {
				caught = true
			}
		}
		if !caught {
			t.Errorf("the planted file %s carries a violation (%s) and the guard did not catch it", filepath.Base(plant), want)
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
