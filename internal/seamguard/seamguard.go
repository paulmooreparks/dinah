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
// What the parse cannot see is stated here, because a guard's blind spots
// belong in its header and not in a reviewer's notes. A read delegated to a
// package Judged does not name (a helper package of Dinah's own, os/exec
// running another program, a third-party library) is invisible, since the
// parse reads one package's files. The call graph is keyed by name without
// type information, so a method edge reaches every method of that name in the
// package, which errs toward reporting too much rather than too little. And a
// read held in a package variable, then copied into some other place by code
// no root reaches and called from there, escapes: the variable's initialiser
// is followed only through references to the variable, and the copy is a
// reference the walk never starts from. The reproduction is
// `var w = filepath.WalkDir` beside a free function that no method calls
// doing `holder.walk = w`, and a method calling holder.walk. Naming a read
// member as a value inside any function body is refused wherever it sits,
// which closes the direct form of that shape; the form through a variable is
// the residue.
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
		"File":            "a type",
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
		"FILE_FLAG_BACKUP_SEMANTICS": "a flag constant",
		"FILE_SHARE_DELETE":          "a share-mode constant",
		"FILE_SHARE_READ":            "a share-mode constant",
		"FILE_SHARE_WRITE":           "a share-mode constant",
		"OPEN_EXISTING":              "a disposition constant",
	},
}

// AllowedSize is the number of names Allowed carries across every package,
// which each guard asserts so that a widened list is a visible edit.
const AllowedSize = 44

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

// Reads is what one body names below the seam.
type Reads struct {
	// Reads are the reads the body calls or names, as "os.Stat at file:line".
	Reads []string
	// Escapes are the reads the body names other than as the function of a
	// call, which a call graph cannot follow to wherever the value is called.
	Escapes []string
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
		what := filepath.Base(path) + "." + sel.Sel.Name
		if path == "os" && sel.Sel.Name == "OpenFile" {
			what = "os.OpenFile (not a constant write flag)"
		}
		what += " at " + filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
		found.Reads = append(found.Reads, what)
		if !called[sel] {
			found.Escapes = append(found.Escapes, what)
		}
		return true
	})
	return found
}

// Node is one declaration of the graph.
type Node struct {
	Key       string   // "func:Name", "method:Name" or "var:Name"
	File      string   // the file declaring it
	Receiver  string   // a method's receiver type, "" otherwise
	Edges     []string // keys this declaration references
	Reads     []string // reads it makes itself, as "os.Stat at file:line"
	Escapes   []string // reads a function body names as a value
	NamesDisk bool     // its body names the identifier Disk
	DiskAt    string
}

// Name answers the declared name without its kind.
func (n *Node) Name() string {
	return n.Key[strings.Index(n.Key, ":")+1:]
}

// Graph is every declaration of the parsed files, merged by key, so a name
// declared once per platform file is one node carrying both bodies.
type Graph struct {
	Nodes map[string]*Node
	// Dotted are the Judged packages a file imports with a dot, as
	// "file: path".
	Dotted []string
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
				reads := ScanReads(fset, imports, d.Body)
				n.Reads = append(n.Reads, reads.Reads...)
				n.Escapes = append(n.Escapes, reads.Escapes...)
				scanEdges(fset, p.file, imports, declared, opts.Leaf, d.Body, n)
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					for _, name := range vs.Names {
						n := node("var:"+name.Name, p.name)
						for _, value := range vs.Values {
							n.Reads = append(n.Reads, ScanReads(fset, imports, value).Reads...)
							scanEdges(fset, p.file, imports, declared, opts.Leaf, value, n)
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

// scanEdges records into n the name Disk and every declaration its body
// names, called or not.
func scanEdges(fset *token.FileSet, file *ast.File, imports map[string]string, declared, leaf map[string]bool, body ast.Node, n *Node) {
	var visit func(ast.Node) bool
	visit = func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.SelectorExpr:
			if ImportPathOf(imports, x) != "" {
				return false
			}
			if !leaf[x.Sel.Name] && declared["method:"+x.Sel.Name] {
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
			if !packageLevel(file, x) {
				return true
			}
			if x.Name == "Disk" {
				n.NamesDisk = true
				at := fset.Position(x.Pos())
				n.DiskAt = filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
			}
			if declared["func:"+x.Name] {
				n.Edges = append(n.Edges, "func:"+x.Name)
			}
			if declared["var:"+x.Name] {
				n.Edges = append(n.Edges, "var:"+x.Name)
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
	// Root reports whether a node is a root of the walk.
	Root func(*Node) bool
	// Exempt are the declarations, by name, the walk does not look inside.
	Exempt map[string]string
	// Seam is the key of the one declaration allowed to name Disk, the
	// source accessor; "" allows none.
	Seam string
}

// Violations walks from every root and answers what each reaches that it
// must not, and every read a function body names as a value, whether or not a
// root reaches it. An exempt declaration is a leaf, is not itself a root, and
// is not held to the value rule.
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
		if _, ok := w.Exempt[n.Name()]; ok {
			continue
		}
		for _, escape := range n.Escapes {
			found = append(found, Violation{Root: key, Via: key, File: n.File,
				What: key + " names " + escape + " as a value, which no call graph can follow to where it is called"})
		}
	}
	for _, dotted := range g.Dotted {
		found = append(found, Violation{Root: "import", Via: "import", What: "dot import of a judged package, " + dotted})
	}
	for _, root := range keys {
		n := g.Nodes[root]
		if !w.Root(n) {
			continue
		}
		if _, ok := w.Exempt[n.Name()]; ok {
			continue
		}
		reached := map[string]string{root: root}
		queue := []string{root}
		for len(queue) > 0 {
			key := queue[0]
			queue = queue[1:]
			n := g.Nodes[key]
			if n == nil {
				continue
			}
			if _, ok := w.Exempt[n.Name()]; ok && key != root {
				continue
			}
			for _, read := range n.Reads {
				tag := key + " " + read
				if !seen[tag] {
					seen[tag] = true
					found = append(found, Violation{Root: root, Via: reached[key], File: n.File, What: key + " reads " + read})
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
			return reached[at] + " reads " + n.Reads[0], true
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
