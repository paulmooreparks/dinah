// Package seamguard is the parse that dinah-619's two read-seam guards share:
// the bench guard (internal/bench, TestTheBenchReadsOnlyThroughItsSource) and
// the library guard (internal/verb, TestTheLibraryReadsOnlyThroughTheBench).
// Only those tests import it, and it imports nothing of Dinah's, so package
// bench can use it from its own tests without a cycle.
//
// The guards judge reads by allowlist. Every member of the packages Judged
// names is a read unless Allowed names it, with the reason it reads no file,
// so a member nobody thought about (ioutil.ReadFile, fs.ReadFile, os.OpenRoot
// and whatever a later Go release adds) is refused rather than let through.
// The first form of these guards listed the reads instead, and a reviewer
// walked four of them past it (dinah-619/comments/12). os.OpenFile is the one
// member judged by its arguments: a flag argument that is an expression of
// os.O_* identifiers joined by | and naming os.O_WRONLY is a write, and any
// other flag is a read.
//
// A function value can be stored by code no root reaches and called by a
// method that names no read, so the walk starts from more than the roots a
// guard declares. It also starts from every package function or variable a
// function body names other than as a call's function or a method call's
// receiver, from every function literal that is neither called in place nor
// handed straight to a function of another package, and it counts a judged
// type named in a signature, a variable's type or a type declaration as a
// read, since a handle of that type reads through methods no allowlist
// judges. dinah-619/comments/15 walked a function stored by name, a stored
// literal and a handed-in *os.Root and fs.FS past the second form of these
// guards; each is a planted file now.
//
// What the parse still cannot see is stated here, because a guard's blind
// spots belong in its header and not in a reviewer's notes, and each has a
// reproduction. A read delegated to a package Judged does not name (a helper
// package of Dinah's own, os/exec running another program, a third-party
// library) is invisible, since the parse reads one package's files: a method
// calling a helper package's function that calls os.ReadFile passes. A value
// that reads, built at run time by code no root reaches and handed to a
// method through a variable or a field, passes whenever neither its
// construction as the walk sees it nor its type names a judged member: a
// free function no method calls doing `holder.src = Disk{}`, with a method
// calling holder.src.ReadFile, passes, and so does an *os.File opened for
// reading there, because os.File is allowlisted as the type every write goes
// through. A write used as a read passes, because writes are allowlisted:
// `os.IsExist(os.Mkdir(path, 0o755))` answers what a Stat would.
// internal/bench keeps the held Disk, the handed-in *os.File and the Mkdir
// probe as planted files under testdata/sourceguard/residue, and its guard is
// asserted to pass each of them.
// The call graph is keyed by name without type information, so a method edge
// reaches every method of that name in the package, which errs toward
// reporting too much rather than too little.
package seamguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Judged are the import paths whose members are reads unless Allowed names
// them: the standard library's filesystem surface, including the raw system
// calls a handle can be opened through.
var Judged = []string{"os", "io/ioutil", "io/fs", "path/filepath", "syscall"}

// Allowed names, for each judged package, the members that read no file,
// each with its reason. Every name in it is one package bench or package verb
// uses today; a member neither uses stays out, so its first use is refused
// and has to be argued for here. io/ioutil allows nothing: it is deprecated,
// and each of its members that touches a file has an os counterpart.
var Allowed = map[string]map[string]string{
	"os": {
		"Create":          "creates or truncates a file for writing",
		"CreateTemp":      "creates a new file for writing",
		"Mkdir":           "creates a directory",
		"MkdirAll":        "creates directories",
		"Remove":          "removes a file or an empty directory",
		"RemoveAll":       "removes a tree",
		"Rename":          "renames a path",
		"WriteFile":       "writes a file",
		"IsExist":         "classifies an error already returned",
		"IsNotExist":      "classifies an error already returned",
		"IsPathSeparator": "classifies a byte",
		"SameFile":        "compares two FileInfo values already read",
		"ErrNotExist":     "an error value",
		"DirEntry":        "a type",
		"File":            "the handle type every write goes through; the open that makes one is judged where it is named",
		"FileInfo":        "a type",
		"LinkError":       "a type",
		"ModeSymlink":     "a mode bit",
		"Args":            "the process's own arguments",
		"Executable":      "the running program's own path, never a path below a workbench",
		"Getenv":          "the process environment",
		"Getpid":          "the process's own identifier",
		"UserHomeDir":     "the home directory's path, read from the environment",
	},
	"io/fs": {
		"DirEntry":    "a type",
		"FileInfo":    "a type",
		"ErrNotExist": "an error value",
	},
	"path/filepath": {
		"Abs":       "joins a path with the working directory without touching the file it names",
		"Base":      "lexical",
		"Clean":     "lexical",
		"Dir":       "lexical",
		"Ext":       "lexical",
		"Join":      "lexical",
		"Rel":       "lexical",
		"Separator": "a constant",
		"SkipDir":   "an error value a walk callback answers",
		"ToSlash":   "lexical",
	},
	"io/ioutil": {},
	"syscall": {
		"Errno":                      "an error number type",
		"ERROR_ACCESS_DENIED":        "an error number",
		"ERROR_DIR_NOT_EMPTY":        "an error number",
		"EXDEV":                      "an error number",
		"FILE_FLAG_BACKUP_SEMANTICS": "a flag constant",
		"FILE_SHARE_DELETE":          "a share-mode constant",
		"FILE_SHARE_READ":            "a share-mode constant",
		"FILE_SHARE_WRITE":           "a share-mode constant",
		"OPEN_EXISTING":              "a disposition constant",
	},
}

// AllowedSize is the number of names Allowed carries across every package,
// which each guard asserts so that a widened list is a visible edit.
const AllowedSize = 45

// IsJudged reports whether an import path is one of Judged.
func IsJudged(path string) bool {
	for _, judged := range Judged {
		if judged == path {
			return true
		}
	}
	return false
}

// IsRead reports whether a member of an import path is a read. os.OpenFile is
// a read here; a caller that has read its flag argument decides otherwise.
//
// Every os.O_ flag is a constant that reads nothing, so the family is allowed
// by its prefix rather than listed: a flag does nothing until an open uses it,
// and the open is what os.OpenFile's rule judges.
func IsRead(path, member string) bool {
	if path == "os" && strings.HasPrefix(member, "O_") {
		return false
	}
	allowed, judged := Allowed[path]
	if !judged {
		return false
	}
	_, ok := allowed[member]
	return !ok
}

// Imports answers a file's imports by local name, and the import paths of
// Judged packages the file imports with a dot, which puts their members out of
// the parse's reach and is refused outright.
func Imports(file *ast.File) (map[string]string, []string) {
	imports := map[string]string{}
	var dotted []string
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		local := filepath.Base(path)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		if local == "." && IsJudged(path) {
			dotted = append(dotted, path)
		}
		imports[local] = path
	}
	return imports, dotted
}

// ImportPathOf answers the import path a selector's qualifier names, or ""
// when the qualifier is not an unshadowed import of the file.
func ImportPathOf(imports map[string]string, sel *ast.SelectorExpr) string {
	qualifier, ok := sel.X.(*ast.Ident)
	if !ok || qualifier.Obj != nil {
		return ""
	}
	return imports[qualifier.Name]
}

// IsWriteFlag reports whether an os.OpenFile flag argument is an expression
// of os.O_* identifiers joined by | that names os.O_WRONLY.
func IsWriteFlag(imports map[string]string, expr ast.Expr) bool {
	wronly := false
	var constant func(ast.Expr) bool
	constant = func(e ast.Expr) bool {
		switch x := e.(type) {
		case *ast.ParenExpr:
			return constant(x.X)
		case *ast.BinaryExpr:
			return x.Op == token.OR && constant(x.X) && constant(x.Y)
		case *ast.SelectorExpr:
			if ImportPathOf(imports, x) != "os" || !strings.HasPrefix(x.Sel.Name, "O_") {
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

// Read is one read a body calls or names.
type Read struct {
	// Member is the judged member named, as "os.Stat", which is what an
	// exemption keyed on one read matches.
	Member string
	// What is the member and where it is named, as "os.Stat at file:line".
	What string
}

// Reads is what one body names below the seam.
type Reads struct {
	// Reads are the reads the body calls or names.
	Reads []Read
	// Escapes are the reads the body names other than as the function of a
	// call, which a call graph cannot follow to wherever the value is called.
	Escapes []Read
}

// ScanReads records the reads one body names. A read is a selector on a
// Judged import that IsRead reports, or os.OpenFile with a flag argument that
// is not a constant write flag.
func ScanReads(fset *token.FileSet, imports map[string]string, body ast.Node) Reads {
	var found Reads
	called := map[*ast.SelectorExpr]bool{}
	writes := map[*ast.SelectorExpr]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		called[sel] = true
		if ImportPathOf(imports, sel) == "os" && sel.Sel.Name == "OpenFile" && len(call.Args) >= 2 && IsWriteFlag(imports, call.Args[1]) {
			writes[sel] = true
		}
		return true
	})
	ast.Inspect(body, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		path := ImportPathOf(imports, sel)
		if path == "" || !IsRead(path, sel.Sel.Name) || writes[sel] {
			return true
		}
		at := fset.Position(sel.Pos())
		member := filepath.Base(path) + "." + sel.Sel.Name
		what := member
		if path == "os" && sel.Sel.Name == "OpenFile" {
			what = "os.OpenFile (not a constant write flag)"
		}
		read := Read{Member: member, What: what + " at " + filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)}
		found.Reads = append(found.Reads, read)
		if !called[sel] {
			found.Escapes = append(found.Escapes, read)
		}
		return true
	})
	return found
}

// Node is one declaration of the graph, or one function literal.
type Node struct {
	Key      string // "func:Name", "method:Name", "var:Name" or "lit:file:line"
	File     string // the file declaring it
	Receiver string // a method's receiver type, "" otherwise
	// Enclosing is the declaration a function literal sits in, by name, which
	// an exemption of that declaration covers; "" for a declaration.
	Enclosing string
	Edges     []string // keys this declaration references
	// Values are the keys of the functions and variables this declaration
	// names other than as the function of a call, which the walk cannot follow
	// to wherever the value is called.
	Values    []string
	Reads     []Read // reads it makes or names itself, its signature included
	Escapes   []Read // reads a function body names as a value
	NamesDisk bool   // its body names the identifier Disk
	DiskAt    string
}

// Name answers the declared name without its kind, or for a function literal
// the declaration it sits in.
func (n *Node) Name() string {
	if n.Enclosing != "" {
		return n.Enclosing
	}
	return n.Key[strings.Index(n.Key, ":")+1:]
}

// Graph is every declaration of the parsed files, merged by key, so a name
// declared once per platform file is one node carrying both bodies.
type Graph struct {
	Nodes map[string]*Node
	// Dotted are the Judged packages a file imports with a dot, as
	// "file: path".
	Dotted []string
	// TypeReads are the reads a type declaration names, as a field or an
	// element type: a handle a method would read through, held where no
	// signature shows it.
	TypeReads []TypeRead
}

// TypeRead is one read a type declaration names, with the file declaring it.
type TypeRead struct {
	Read
	File string
}

// Options shape a graph's parse.
type Options struct {
	// Leaf are method names the walk treats as the seam itself: a selector
	// naming one is where a read goes through the seam, so it is no edge.
	Leaf map[string]bool
	// Bottom is the receiver type whose methods are the seam's bottom. They
	// read the filesystem by definition and are left out of the graph, so a
	// method edge keyed by name never reaches them.
	Bottom string
}

// Build parses the named files as one package and answers its graph.
func Build(files []string, opts Options) (*Graph, error) {
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
			return nil, fmt.Errorf("parse %s: %v", name, err)
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
	g := &Graph{Nodes: map[string]*Node{}}
	node := func(key, file string) *Node {
		n, ok := g.Nodes[key]
		if !ok {
			n = &Node{Key: key, File: file}
			g.Nodes[key] = n
		}
		return n
	}
	for _, p := range all {
		imports, dotted := Imports(p.file)
		for _, path := range dotted {
			g.Dotted = append(g.Dotted, filepath.Base(p.name)+": "+path)
		}
		s := scan{fset: fset, file: p.file, imports: imports, declared: declared, leaf: opts.Leaf}
		for _, decl := range p.file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				receiver := receiverName(d.Recv)
				if opts.Bottom != "" && receiver == opts.Bottom {
					continue
				}
				n := node(declKey(d), p.name)
				n.Receiver = receiver
				// A signature naming a judged type is a handle the body
				// reads through with methods no allowlist judges, so the
				// type is the read.
				n.Reads = append(n.Reads, ScanReads(fset, imports, d.Type).Reads...)
				reads := ScanReads(fset, imports, d.Body)
				n.Reads = append(n.Reads, reads.Reads...)
				n.Escapes = append(n.Escapes, reads.Escapes...)
				s.edges(d.Body, n)
				for _, lit := range storedLiterals(imports, d.Body) {
					at := fset.Position(lit.Pos())
					ln := node("lit:"+filepath.Base(at.Filename)+":"+strconv.Itoa(at.Line)+":"+strconv.Itoa(at.Column), p.name)
					ln.Enclosing = d.Name.Name
					ln.Reads = append(ln.Reads, ScanReads(fset, imports, lit).Reads...)
					s.edges(lit.Body, ln)
				}
			case *ast.GenDecl:
				switch d.Tok {
				case token.TYPE:
					for _, read := range ScanReads(fset, imports, d).Reads {
						g.TypeReads = append(g.TypeReads, TypeRead{read, p.name})
					}
				case token.VAR:
					for _, spec := range d.Specs {
						vs := spec.(*ast.ValueSpec)
						for _, name := range vs.Names {
							n := node("var:"+name.Name, p.name)
							if vs.Type != nil {
								n.Reads = append(n.Reads, ScanReads(fset, imports, vs.Type).Reads...)
							}
							for _, value := range vs.Values {
								n.Reads = append(n.Reads, ScanReads(fset, imports, value).Reads...)
								s.edges(value, n)
							}
						}
					}
				}
			}
		}
	}
	if len(g.Nodes) == 0 {
		return nil, fmt.Errorf("the parse found no declaration, so the graph is empty and a guard over it proves nothing")
	}
	return g, nil
}

// storedLiterals answers the function literals of a body whose value is kept
// rather than run where it stands: every literal that is neither called in
// place nor handed as an argument to a function of another package. One
// assigned, returned, put in a composite or handed to a function of this
// package can be called from anywhere later, so the walk starts from it.
func storedLiterals(imports map[string]string, body ast.Node) []*ast.FuncLit {
	inPlace := map[*ast.FuncLit]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if lit, ok := unparen(call.Fun).(*ast.FuncLit); ok {
			inPlace[lit] = true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && ImportPathOf(imports, sel) != "" {
			for _, arg := range call.Args {
				if lit, ok := unparen(arg).(*ast.FuncLit); ok {
					inPlace[lit] = true
				}
			}
		}
		return true
	})
	var stored []*ast.FuncLit
	ast.Inspect(body, func(node ast.Node) bool {
		if lit, ok := node.(*ast.FuncLit); ok && !inPlace[lit] {
			stored = append(stored, lit)
		}
		return true
	})
	return stored
}

// unparen strips parentheses from an expression.
func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// declKey keys a function declaration by whether it is a method.
func declKey(d *ast.FuncDecl) string {
	if d.Recv != nil {
		return "method:" + d.Name.Name
	}
	return "func:" + d.Name.Name
}

// receiverName answers a receiver's type name, by value or by pointer.
func receiverName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if index, ok := expr.(*ast.IndexExpr); ok {
		expr = index.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// scan is one file's context for recording a body's edges.
type scan struct {
	fset     *token.FileSet
	file     *ast.File
	imports  map[string]string
	declared map[string]bool
	leaf     map[string]bool
}

// edges records into n the name Disk, every declaration its body names,
// called or not, and among those the functions and variables it names other
// than as the function of a call or the receiver of a method call.
func (s scan) edges(body ast.Node, n *Node) {
	called := map[*ast.Ident]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		// A name called, or the receiver of a method called, is used where
		// it stands; neither hands the value on.
		switch fun := unparen(call.Fun).(type) {
		case *ast.Ident:
			called[fun] = true
		case *ast.SelectorExpr:
			if ident, ok := unparen(fun.X).(*ast.Ident); ok {
				called[ident] = true
			}
		}
		return true
	})
	var visit func(ast.Node) bool
	visit = func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.SelectorExpr:
			if ImportPathOf(s.imports, x) != "" {
				return false
			}
			if !s.leaf[x.Sel.Name] && s.declared["method:"+x.Sel.Name] {
				n.Edges = append(n.Edges, "method:"+x.Sel.Name)
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
			if !packageLevel(s.file, x) {
				return true
			}
			if x.Name == "Disk" {
				n.NamesDisk = true
				at := s.fset.Position(x.Pos())
				n.DiskAt = filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
			}
			for _, key := range []string{"func:" + x.Name, "var:" + x.Name} {
				if !s.declared[key] {
					continue
				}
				n.Edges = append(n.Edges, key)
				if !called[x] {
					n.Values = append(n.Values, key)
				}
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

// Violation is one reach of a read, or of the name Disk, from a root, or one
// read named as a value anywhere.
type Violation struct {
	Root, Via, What string
	// File is the file declaring the node that reads or names Disk.
	File string
}

// Walk describes one guard's walk over a graph.
type Walk struct {
	// Root reports whether a declaration is a root of the walk. The walk
	// also starts from every function literal whose value is kept, and from
	// every function and variable some declaration names as a value.
	Root func(*Node) bool
	// Exempt are the reads the walk lets through, keyed on the declaration
	// by name and then on the member read, as "filepath.WalkDir". The pair
	// is the key, so a read beside the excused one is still refused.
	Exempt map[string]map[string]string
	// Seam is the key of the one declaration allowed to name Disk, the
	// source accessor; "" allows none.
	Seam string
}

// exempt reports whether a node's read is one its exemption names.
func (w Walk) exempt(n *Node, r Read) bool {
	_, ok := w.Exempt[n.Name()][r.Member]
	return ok
}

// Roots answers the keys the walk starts from: every node Root reports,
// every stored function literal, and every function and variable a function
// names as a value, which code no other root reaches may have stored where a
// method calls it. A value a variable's initialiser names is held by that
// variable, whose every use is an edge or is itself a value named, so the
// walk reaches it through the variable and does not start from it.
func (g *Graph) Roots(w Walk) []string {
	roots := map[string]bool{}
	for key, n := range g.Nodes {
		if w.Root(n) || strings.HasPrefix(key, "lit:") {
			roots[key] = true
		}
		if strings.HasPrefix(key, "var:") {
			continue
		}
		for _, value := range n.Values {
			if g.Nodes[value] != nil {
				roots[value] = true
			}
		}
	}
	var keys []string
	for key := range roots {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Violations walks from every root and answers what each reaches that it
// must not, every read a function body names as a value whether or not a
// root reaches it, and every read a type declaration names.
func (g *Graph) Violations(w Walk) []Violation {
	var found []Violation
	seen := map[string]bool{}
	var keys []string
	for key := range g.Nodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		n := g.Nodes[key]
		for _, escape := range n.Escapes {
			if w.exempt(n, escape) {
				continue
			}
			found = append(found, Violation{Root: key, Via: key, File: n.File,
				What: key + " names " + escape.What + " as a value, which no call graph can follow to where it is called"})
		}
	}
	for _, dotted := range g.Dotted {
		found = append(found, Violation{Root: "import", Via: "import", What: "dot import of a judged package, " + dotted})
	}
	for _, read := range g.TypeReads {
		found = append(found, Violation{Root: "type", Via: "type", File: read.File,
			What: "a type declaration names " + read.What + ", a handle a method would read through"})
	}
	for _, root := range g.Roots(w) {
		reached := map[string]string{root: root}
		queue := []string{root}
		for len(queue) > 0 {
			key := queue[0]
			queue = queue[1:]
			n := g.Nodes[key]
			if n == nil {
				continue
			}
			for _, read := range n.Reads {
				if w.exempt(n, read) {
					continue
				}
				tag := key + " " + read.What
				if !seen[tag] {
					seen[tag] = true
					found = append(found, Violation{Root: root, Via: reached[key], File: n.File, What: key + " reads " + read.What})
				}
			}
			if n.NamesDisk && key != w.Seam {
				tag := key + " Disk"
				if !seen[tag] {
					seen[tag] = true
					found = append(found, Violation{Root: root, Via: reached[key], File: n.File, What: key + " names Disk at " + n.DiskAt})
				}
			}
			for _, next := range n.Edges {
				if next == w.Seam {
					continue
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

// Reaches reports whether anything reachable from a key reads, or names Disk
// anywhere but the seam, and answers the first such finding, walking every
// edge with no exemption.
func (g *Graph) Reaches(key, seam string) (string, bool) {
	reached := map[string]string{key: key}
	queue := []string{key}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		n := g.Nodes[at]
		if n == nil {
			continue
		}
		if len(n.Reads) > 0 {
			return reached[at] + " reads " + n.Reads[0].What, true
		}
		if n.NamesDisk && at != seam {
			return reached[at] + " names Disk at " + n.DiskAt, true
		}
		for _, next := range n.Edges {
			if next == seam {
				continue
			}
			if _, ok := reached[next]; ok {
				continue
			}
			reached[next] = reached[at] + " > " + next
			queue = append(queue, next)
		}
	}
	return "", false
}
