# dinah-439: every shape this repository writes a directory read in

This companion carries the two programs section 8 of dinah-439's spec rests on,
together with what they printed at `65a80ad8b3b525e62af2262ad6d3203876ea6c7a`.
It exists because three rounds of design review on that card each falsified a
rule worded over a shape somebody had pictured, and the remedy was to derive the
shapes from the syntax trees instead. A spec that tells its implementer to
re-run a program owes them the program.

Neither program ships. They are held here as fenced source rather than as `.go`
files on purpose, so that `go build ./...` never compiles them and no guard over
the tree has to carve out an exemption for them. To run one, copy it into a
throwaway directory outside the repository, write a `go.mod` beside it, and pass
the worktree path as the single argument.

## Program 1: the enumeration

This one answers what shapes exist. It matches every call to a directory-reading
API and every call to the four collection readers, then classifies each site by
how the error result is taken and by the shape of the statement that tests it.

```go
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var fset = token.NewFileSet()

func src(n ast.Node) string {
	var b strings.Builder
	printer.Fprint(&b, fset, n)
	s := b.String()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " ..."
	}
	if len(s) > 110 {
		s = s[:110] + "..."
	}
	return s
}

func calleeName(c *ast.CallExpr) string {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if x, ok := f.X.(*ast.Ident); ok {
			return x.Name + "." + f.Sel.Name
		}
		return "?." + f.Sel.Name
	}
	return ""
}

var dirReadNames = map[string]bool{
	"os.ReadDir": true, "filepath.WalkDir": true, "fs.WalkDir": true,
	"filepath.Walk": true, "fs.ReadDir": true, "ioutil.ReadDir": true,
	"os.Open": true, "filepath.Glob": true, "os.DirFS": true,
}
var dirReadSels = map[string]bool{
	"ReadDir": true, "Readdir": true, "Readdirnames": true, "Glob": true, "WalkDir": true, "Walk": true,
}
var collectionReaders = map[string]bool{
	"readCollection": true, "ListIDs": true, "ListWorkbenchIDs": true, "heldLocks": true,
	"bench.ListIDs": true, "bench.ListWorkbenchIDs": true,
}

type site struct {
	pos      string
	callee   string
	fn       string
	binding  string
	errName  string
	guards   []string
	guardSrc []string
	term     []string
}

func condShape(cond ast.Expr, errName string) (string, bool) {
	mentions := false
	ast.Inspect(cond, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == errName {
			mentions = true
		}
		return true
	})
	if !mentions {
		return "", false
	}
	switch b := cond.(type) {
	case *ast.BinaryExpr:
		switch b.Op {
		case token.NEQ, token.EQL:
			if l, ok := b.X.(*ast.Ident); ok && l.Name == errName {
				if r, ok := b.Y.(*ast.Ident); ok && r.Name == "nil" {
					if b.Op == token.NEQ {
						return "simple err != nil", true
					}
					return "POSITIVE err == nil", true
				}
			}
			return "comparison other", true
		case token.LOR:
			return "COMPOUND || containing err test", true
		case token.LAND:
			return "COMPOUND && containing err test", true
		}
	case *ast.UnaryExpr:
		if b.Op == token.NOT {
			return "negated", true
		}
	case *ast.CallExpr:
		return "call on err", true
	}
	return "other shape", true
}

func termOf(body *ast.BlockStmt) []string {
	if body == nil || len(body.List) == 0 {
		return []string{"<empty>"}
	}
	var out []string
	last := body.List[len(body.List)-1]
	switch s := last.(type) {
	case *ast.ReturnStmt:
		out = append(out, "ends: "+src(s))
	case *ast.BranchStmt:
		out = append(out, "ends: "+s.Tok.String())
	default:
		out = append(out, fmt.Sprintf("ends: falls out of branch (%T)", last))
	}
	nested := 0
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.ReturnStmt); ok {
			nested++
		}
		return true
	})
	if nested > 1 {
		out = append(out, fmt.Sprintf("(%d returns inside branch)", nested))
	}
	return out
}

func main() {
	root := os.Args[1]
	var sites []site
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			b := filepath.Base(path)
			if b == ".git" || b == "node_modules" || b == "editors" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			fmt.Println("PARSE ERROR", path, perr)
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			walkBlock(fn.Body, fn.Name.Name, rel, &sites)
			return false
		})
		return nil
	})
	sort.Slice(sites, func(i, j int) bool { return sites[i].pos < sites[j].pos })

	fmt.Println("### PER-SITE")
	for _, s := range sites {
		fmt.Printf("%s  %s  in %s\n    binding: %s  err=%q\n", s.pos, s.callee, s.fn, s.binding, s.errName)
		for i, g := range s.guards {
			fmt.Printf("    guard: %s   | %s\n", g, s.guardSrc[i])
		}
		for _, t := range s.term {
			fmt.Printf("    term:  %s\n", t)
		}
	}
	fmt.Println("")
	fmt.Println("### SHAPE TABLE (binding x guard)")
	tab := map[string][]string{}
	for _, s := range sites {
		g := "<no guard found>"
		if len(s.guards) > 0 {
			g = strings.Join(s.guards, " + ")
		}
		k := s.binding + "  ||  " + g
		tab[k] = append(tab[k], s.pos+" "+s.callee)
	}
	keys := []string{}
	for k := range tab {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("\n[%d] %s\n", len(tab[k]), k)
		for _, v := range tab[k] {
			fmt.Println("      ", v)
		}
	}
	fmt.Println("")
	fmt.Println("TOTAL SITES:", len(sites))
}

func interesting(c *ast.CallExpr) (string, bool) {
	name := calleeName(c)
	if name == "" {
		return "", false
	}
	if dirReadNames[name] || collectionReaders[name] {
		return name, true
	}
	if sel, ok := c.Fun.(*ast.SelectorExpr); ok && dirReadSels[sel.Sel.Name] {
		return name, true
	}
	return "", false
}

func walkBlock(b *ast.BlockStmt, fnName, rel string, out *[]site) {
	if b == nil {
		return
	}
	for i, st := range b.List {
		collect(st, b.List[i+1:], fnName, rel, out)
	}
}

func collect(st ast.Stmt, rest []ast.Stmt, fnName, rel string, out *[]site) {
	record := func(c *ast.CallExpr, binding, errName string, guards, guardSrc, term []string) {
		name, ok := interesting(c)
		if !ok {
			return
		}
		*out = append(*out, site{
			pos:    fmt.Sprintf("%s:%d", rel, fset.Position(c.Pos()).Line),
			callee: name, fn: fnName, binding: binding, errName: errName,
			guards: guards, guardSrc: guardSrc, term: term,
		})
	}
	scanFollowing := func(errName string, rest []ast.Stmt) (g, gs, term []string) {
		if errName == "" || errName == "_" {
			return
		}
		for _, s := range rest {
			if ifs, ok := s.(*ast.IfStmt); ok {
				if shape, m := condShape(ifs.Cond, errName); m {
					g = append(g, shape)
					gs = append(gs, src(ifs.Cond))
					term = append(term, termOf(ifs.Body)...)
					continue
				}
			}
			if sw, ok := s.(*ast.SwitchStmt); ok && sw.Tag != nil {
				if _, m := condShape(sw.Tag, errName); m {
					g = append(g, "switch on err")
					gs = append(gs, src(sw.Tag))
				}
			}
		}
		return
	}

	switch s := st.(type) {
	case *ast.AssignStmt:
		for _, rhs := range s.Rhs {
			if c, ok := rhs.(*ast.CallExpr); ok {
				errName := ""
				binding := "assign"
				if len(s.Lhs) >= 1 {
					last := s.Lhs[len(s.Lhs)-1]
					if id, ok := last.(*ast.Ident); ok {
						errName = id.Name
						if id.Name == "_" {
							binding = "assign, error DISCARDED (_)"
						}
					} else {
						binding = "assign to non-ident"
					}
				}
				g, gs, term := scanFollowing(errName, rest)
				if len(g) == 0 && errName != "" && errName != "_" {
					binding += " [NO following err test in this block]"
				}
				record(c, binding, errName, g, gs, term)
			}
		}
	case *ast.ExprStmt:
		if c, ok := s.X.(*ast.CallExpr); ok {
			record(c, "bare call statement, result discarded", "", nil, nil, nil)
		}
	case *ast.ReturnStmt:
		for _, r := range s.Results {
			if c, ok := r.(*ast.CallExpr); ok {
				record(c, "returned directly as the enclosing function results", "", nil, nil, []string{"ends: " + src(s)})
			}
		}
	case *ast.IfStmt:
		if s.Init != nil {
			if as, ok := s.Init.(*ast.AssignStmt); ok {
				for _, rhs := range as.Rhs {
					if c, ok := rhs.(*ast.CallExpr); ok {
						errName := ""
						if len(as.Lhs) >= 1 {
							if id, ok := as.Lhs[len(as.Lhs)-1].(*ast.Ident); ok {
								errName = id.Name
							}
						}
						shape, _ := condShape(s.Cond, errName)
						if shape == "" {
							shape = "<cond does not mention err>"
						}
						var term []string
						term = append(term, termOf(s.Body)...)
						if s.Else == nil {
							term = append(term, "NO else branch")
						} else {
							term = append(term, "has else branch")
						}
						record(c, "if-init assign (read inside the if statement)", errName,
							[]string{shape}, []string{src(s.Cond)}, term)
					}
				}
			}
		}
		if c, ok := s.Cond.(*ast.CallExpr); ok {
			record(c, "call in if-condition", "", nil, nil, termOf(s.Body))
		}
		walkBlock(s.Body, fnName, rel, out)
		if eb, ok := s.Else.(*ast.BlockStmt); ok {
			walkBlock(eb, fnName, rel, out)
		} else if ei, ok := s.Else.(*ast.IfStmt); ok {
			collect(ei, nil, fnName, rel, out)
		}
	case *ast.ForStmt:
		walkBlock(s.Body, fnName, rel, out)
	case *ast.RangeStmt:
		if c, ok := s.X.(*ast.CallExpr); ok {
			record(c, "ranged over directly", "", nil, nil, nil)
		}
		walkBlock(s.Body, fnName, rel, out)
	case *ast.BlockStmt:
		walkBlock(s, fnName, rel, out)
	case *ast.SwitchStmt:
		for _, cc := range s.Body.List {
			if c, ok := cc.(*ast.CaseClause); ok {
				for i, in := range c.Body {
					collect(in, c.Body[i+1:], fnName, rel, out)
				}
			}
		}
	case *ast.TypeSwitchStmt:
		for _, cc := range s.Body.List {
			if c, ok := cc.(*ast.CaseClause); ok {
				for i, in := range c.Body {
					collect(in, c.Body[i+1:], fnName, rel, out)
				}
			}
		}
	case *ast.SelectStmt:
		walkBlock(s.Body, fnName, rel, out)
	case *ast.LabeledStmt:
		collect(s.Stmt, rest, fnName, rel, out)
	case *ast.DeferStmt:
		record(s.Call, "deferred call", "", nil, nil, nil)
	case *ast.GoStmt:
		record(s.Call, "go statement", "", nil, nil, nil)
	}

	ast.Inspect(st, func(n ast.Node) bool {
		fl, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}
		walkBlock(fl.Body, fnName+".func", rel, out)
		return false
	})
}
```

### What it printed at `65a80ad`

49 sites, collapsing to the shape table below. The per-site half is omitted here;
re-run the program for it.

```
[2] assign  ||  COMPOUND || containing err test
       internal/bench/check.go:699 os.ReadDir
       internal/bench/resolve.go:768 os.ReadDir

[1] assign  ||  COMPOUND || containing err test + simple err != nil + simple err != nil
       internal/bench/entity.go:250 os.ReadDir

[13] assign  ||  simple err != nil
       internal/bench/bench.go:1039 os.ReadDir
       internal/bench/bench.go:1255 ListWorkbenchIDs
       internal/bench/bench.go:880 os.ReadDir
       internal/bench/container.go:186 os.ReadDir
       internal/bench/container.go:485 os.ReadDir
       internal/bench/container.go:541 ListWorkbenchIDs
       internal/bench/container.go:750 filepath.WalkDir
       internal/bench/finish.go:60 os.ReadDir
       internal/bench/storage.go:157 os.ReadDir
       internal/bench/storage.go:299 os.ReadDir
       internal/bench/vocabulary.go:328 os.ReadDir
       internal/guide/guide.go:43 guides.ReadDir
       internal/msg/msg.go:100 locales.ReadDir

[1] assign  ||  simple err != nil + COMPOUND && containing err test
       internal/verb/search.go:550 os.Open

[1] assign  ||  simple err != nil + simple err != nil
       internal/textwidth/gen_emoji_properties.go:90 os.Open

[1] assign  ||  simple err != nil + simple err != nil + simple err != nil + simple err != nil
       internal/bench/entity.go:317 os.Open

[1] bare call statement, result discarded  ||  <no guard found>
       internal/bench/container.go:705 filepath.WalkDir

[1] if-init assign (read inside the if statement)  ||  POSITIVE err == nil
       internal/bench/container.go:693 os.ReadDir

[3] if-init assign (read inside the if statement)  ||  other shape
       internal/bench/container.go:325 heldLocks
       internal/bench/container.go:371 heldLocks
       internal/bench/container.go:607 heldLocks

[24] ranged over directly  ||  <no guard found>
       ... 22 bare ListIDs sites inside internal/bench, plus
       internal/verb/search.go:168 bench.ListIDs
       internal/verb/tree.go:1204 bench.ListIDs

[1] returned directly as the enclosing function results  ||  <no guard found>
       internal/bench/container.go:799 filepath.WalkDir

TOTAL SITES: 49
```

The `ranged over directly` bucket holds the `for _, id := range ListIDs(...)`
sites this card rewrites, and it carries no guard today because `ListIDs`
declares no error today. The three `heldLocks` sites read `other shape` because
their `if`-init binds `held` rather than an error, and their condition tests
`len(held) > 0`.

Two cautions for anyone re-running this. It walks statement lists rather than
resolving types, so it sees a call written as an identifier and does not know
what that identifier resolves to; a second declaration of one of the four
collection reader names elsewhere in the tree would be matched as if it were the
real one, which is why the spec checks separately that no such declaration
exists. And it scans for an error test only among the statements following the
read in the same block, so a test nested one level deeper would be reported as
absent. Neither case arises at `65a80ad`.

## Program 2: the corrected rule 5

This one answers whether the rule catches what the card claims. It implements
rule 5 as section 8 words it, with the failure path covering all four forms the
enumeration found and the question asked of every exit reachable on that path.

```go
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var fset = token.NewFileSet()

func src(n ast.Node) string {
	var b strings.Builder
	printer.Fprint(&b, fset, n)
	s := b.String()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " ..."
	}
	if len(s) > 90 {
		s = s[:90] + "..."
	}
	return s
}

func calleeName(c *ast.CallExpr) string {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if x, ok := f.X.(*ast.Ident); ok {
			return x.Name + "." + f.Sel.Name
		}
	}
	return ""
}

var reads = map[string]bool{"os.ReadDir": true, "filepath.WalkDir": true, "fs.WalkDir": true}

func mentions(e ast.Expr, name string) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return true
	})
	return found
}

// failureBranch reports whether cond is `name != nil` or a boolean combination
// containing it. This is the half a rule worded over the err != nil branch gets
// right, widened to cover || and && alike.
func failureBranch(cond ast.Expr, name string) bool {
	switch b := cond.(type) {
	case *ast.BinaryExpr:
		switch b.Op {
		case token.NEQ:
			if l, ok := b.X.(*ast.Ident); ok && l.Name == name {
				if r, ok := b.Y.(*ast.Ident); ok && r.Name == "nil" {
					return true
				}
			}
		case token.LOR, token.LAND:
			return failureBranch(b.X, name) || failureBranch(b.Y, name)
		}
	}
	return false
}

// successBranch recognises the positive form, whose failure path is whatever
// follows rather than a branch of its own.
func successBranch(cond ast.Expr, name string) bool {
	if b, ok := cond.(*ast.BinaryExpr); ok && b.Op == token.EQL {
		if l, ok := b.X.(*ast.Ident); ok && l.Name == name {
			if r, ok := b.Y.(*ast.Ident); ok && r.Name == "nil" {
				return true
			}
		}
	}
	return false
}

func returnAnswers(r *ast.ReturnStmt, fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return false
	}
	hasErr := false
	for _, f := range fn.Type.Results.List {
		if id, ok := f.Type.(*ast.Ident); ok && id.Name == "error" {
			hasErr = true
		}
	}
	if !hasErr {
		return false
	}
	if len(r.Results) == 0 {
		return false
	}
	last := r.Results[len(r.Results)-1]
	if id, ok := last.(*ast.Ident); ok && id.Name == "nil" {
		return false
	}
	if len(r.Results) == 1 {
		if _, ok := last.(*ast.CallExpr); ok {
			return true
		}
	}
	return true
}

type finding struct{ pos, fn, why string }

var findings []finding

func report(n ast.Node, rel, fn, why string) {
	findings = append(findings, finding{fmt.Sprintf("%s:%d", rel, fset.Position(n.Pos()).Line), fn, why})
}

// checkPath walks a statement list lying on the failure path and reports any
// exit that does not answer. tail reports whether falling off the end of this
// list continues into the enclosing function's ordinary return.
func checkPath(stmts []ast.Stmt, fn *ast.FuncDecl, rel, readPos string, tail bool) {
	sawExit := false
	var walk func(list []ast.Stmt)
	walk = func(list []ast.Stmt) {
		for _, s := range list {
			switch t := s.(type) {
			case *ast.ReturnStmt:
				sawExit = true
				if !returnAnswers(t, fn) {
					report(t, rel, fn.Name.Name, "return on the failure path does not set a non-nil error: "+src(t)+"  [read at "+readPos+"]")
				}
			case *ast.BranchStmt:
				if t.Tok == token.CONTINUE || t.Tok == token.BREAK {
					sawExit = true
					report(t, rel, fn.Name.Name, t.Tok.String()+" on the failure path leaves the failure unanswered  [read at "+readPos+"]")
				}
			case *ast.IfStmt:
				walk(t.Body.List)
				if eb, ok := t.Else.(*ast.BlockStmt); ok {
					walk(eb.List)
				}
			case *ast.BlockStmt:
				walk(t.List)
			case *ast.ForStmt:
				walk(t.Body.List)
			case *ast.RangeStmt:
				walk(t.Body.List)
			}
		}
	}
	walk(stmts)
	if tail && !sawExit {
		report(stmts[0], rel, fn.Name.Name, "the failure path falls through to the function's ordinary return, which answers success  [read at "+readPos+"]")
	}
}

func main() {
	root := os.Args[1]
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			b := filepath.Base(path)
			if b == ".git" || b == "node_modules" || b == "editors" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			inspectFn(fn, rel)
		}
		return nil
	})
	sort.Slice(findings, func(i, j int) bool { return findings[i].pos < findings[j].pos })
	for _, f := range findings {
		fmt.Printf("%-40s %-26s %s\n", f.pos, f.fn, f.why)
	}
	fmt.Println("")
	fmt.Println("RULE 5 FINDINGS:", len(findings))
}

func inspectFn(fn *ast.FuncDecl, rel string) {
	var scan func(list []ast.Stmt)
	scan = func(list []ast.Stmt) {
		for i, s := range list {
			switch t := s.(type) {
			case *ast.AssignStmt:
				for _, rhs := range t.Rhs {
					c, ok := rhs.(*ast.CallExpr)
					if !ok || !reads[calleeName(c)] {
						continue
					}
					name := ""
					if id, ok := t.Lhs[len(t.Lhs)-1].(*ast.Ident); ok {
						name = id.Name
					}
					if name == "" || name == "_" {
						continue // rule 1 owns this
					}
					readPos := fmt.Sprintf("%s:%d", rel, fset.Position(c.Pos()).Line)
					found := false
					for _, after := range list[i+1:] {
						ifs, ok := after.(*ast.IfStmt)
						if !ok || !mentions(ifs.Cond, name) {
							continue
						}
						if failureBranch(ifs.Cond, name) {
							found = true
							checkPath(ifs.Body.List, fn, rel, readPos, false)
						} else if successBranch(ifs.Cond, name) {
							found = true
							if eb, ok := ifs.Else.(*ast.BlockStmt); ok {
								checkPath(eb.List, fn, rel, readPos, false)
							} else {
								checkPath(list[i+1:], fn, rel, readPos, true)
							}
						}
					}
					if !found {
						report(c, rel, fn.Name.Name, "the read's error is bound but no branch tests it  [read at "+readPos+"]")
					}
				}
			case *ast.IfStmt:
				if as, ok := t.Init.(*ast.AssignStmt); ok {
					for _, rhs := range as.Rhs {
						c, ok := rhs.(*ast.CallExpr)
						if !ok || !reads[calleeName(c)] {
							continue
						}
						name := ""
						if id, ok := as.Lhs[len(as.Lhs)-1].(*ast.Ident); ok {
							name = id.Name
						}
						readPos := fmt.Sprintf("%s:%d", rel, fset.Position(c.Pos()).Line)
						if failureBranch(t.Cond, name) {
							checkPath(t.Body.List, fn, rel, readPos, false)
						} else if successBranch(t.Cond, name) {
							if eb, ok := t.Else.(*ast.BlockStmt); ok {
								checkPath(eb.List, fn, rel, readPos, false)
							} else if rest := list[i+1:]; len(rest) == 0 {
								report(t, rel, fn.Name.Name, "the failure path is the fallthrough off the end of the function, which answers success  [read at "+readPos+"]")
							} else {
								checkPath(rest, fn, rel, readPos, true)
							}
						} else {
							report(t, rel, fn.Name.Name, "the read sits in an if-init whose condition does not test its error  [read at "+readPos+"]")
						}
					}
				}
				scan(t.Body.List)
				if eb, ok := t.Else.(*ast.BlockStmt); ok {
					scan(eb.List)
				}
			case *ast.ForStmt:
				scan(t.Body.List)
			case *ast.RangeStmt:
				scan(t.Body.List)
			case *ast.BlockStmt:
				scan(t.List)
			}
		}
	}
	scan(fn.Body.List)
}
```

### What it printed at `65a80ad`

```
internal/bench/check.go:701       checkAttachmentFilename  continue on the failure path leaves the failure unanswered  [read at internal/bench/check.go:699]
internal/bench/container.go:487   resumableLift            return on the failure path does not set a non-nil error: return "", nil  [read at internal/bench/container.go:485]
internal/bench/container.go:717   heldLocks                return on the failure path does not set a non-nil error: return held  [read at internal/bench/container.go:693]
internal/bench/finish.go:62       interruptions            continue on the failure path leaves the failure unanswered  [read at internal/bench/finish.go:60]
internal/bench/storage.go:159     ListIDs                  return on the failure path does not set a non-nil error: return nil  [read at internal/bench/storage.go:157]
internal/bench/storage.go:302     ListWorkbenchIDs         return on the failure path does not set a non-nil error: return nil, nil  [read at internal/bench/storage.go:299]
internal/bench/vocabulary.go:331  listIdentifiers          return on the failure path does not set a non-nil error: return nil, nil  [read at internal/bench/vocabulary.go:328]

RULE 5 FINDINGS: 7
```

Seven findings, and they are the seven reads dinah-439 rewrites. Six of them are
FIX rows in the spec's survey table. The seventh, `ListWorkbenchIDs`, is the row
the survey marks "correct since dinah-433, it is the model", and it fires here
because its `return nil, nil` on an absent path is the existence discrimination
this card moves into `readCollection`. After the card lands, `readCollection`
carries that shape alone and holds the guard's single exemption entry.

Nothing fires on `bench.go:880`, `bench.go:1039`, `container.go:186`,
`entity.go:250` or `resolve.go:768`, which are the five reads the survey blesses
as already honest. The false-positive set across 93 non-test files is empty.

Two earlier wordings of rule 5 fail against this same tree, which is why the
wording matters more than it looks. A rule worded over the branch taken when the
error is non-nil never fires on `container.go:693`, because that read has no such
branch, nor on `check.go:699`, whose condition merely contains the test. A rule
asking only about the branch's terminating statement never fires on
`storage.go:299` or `vocabulary.go:328`, whose branches end `return nil, err`
and swallow in a nested `return nil, nil` earlier in the same branch. Each of
those omissions blesses a site this card exists to fix.
