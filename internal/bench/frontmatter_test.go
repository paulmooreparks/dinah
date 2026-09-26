package bench

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// cloneBase is the header every case of TestACloneIsTheCallersOwn starts from.
const cloneBase = "---\na: 1\nb: 2\nc: 3\n---\nbody"

// TestACloneIsTheCallersOwn is part of dinah-619/criteria/12. A resident
// snapshot hands one parsed header to many requests as clones, so a write
// through a clone must never reach the header it was cloned from, and a write
// through that header must never reach the clone. Each of the six writers is
// run on a clone, then the original is written through its own copy, and each
// Render is compared with the one expected.
//
// The second half parses frontmatter.go and asserts that every method writing
// to the receiver's keys or block, or to an element of either, calls own
// before the write, so a writer added later without it fails here.
//
// Arming: removing own from Delete makes the Delete case's clone write into
// the backing array it shares with the original, and the original renders
// with a key repeated.
func TestACloneIsTheCallersOwn(t *testing.T) {
	cases := []struct {
		name  string
		write func(*Frontmatter)
		want  string
	}{
		{"Set", func(f *Frontmatter) { f.Set("b", "9") }, "---\na: 1\nb: 9\nc: 3\n---\nbody"},
		{"SetAfter", func(f *Frontmatter) { f.SetAfter("n", "9", "a") }, "---\na: 1\nn: 9\nb: 2\nc: 3\n---\nbody"},
		{"SetSeq", func(f *Frontmatter) { f.SetSeq("s", []string{"x", "y"}) }, "---\na: 1\nb: 2\nc: 3\ns:\n  - x\n  - y\n---\nbody"},
		{"SetRaw", func(f *Frontmatter) { f.SetRaw("r", []string{"r: raw"}) }, "---\na: 1\nb: 2\nc: 3\nr: raw\n---\nbody"},
		{"Delete", func(f *Frontmatter) { f.Delete("b") }, "---\na: 1\nc: 3\n---\nbody"},
		{"Rename", func(f *Frontmatter) {
			if err := f.Rename("a", "z"); err != nil {
				t.Fatal(err)
			}
		}, "---\nz: 1\nb: 2\nc: 3\n---\nbody"},
	}
	const originalWrite = "---\na: 1\nb: 2\nc: 3\nw: 7\n---\nbody"
	for _, c := range cases {
		original, body := ParseAnchor(cloneBase)
		clone := original.Clone()
		c.write(clone)
		if got := clone.Render(body); got != c.want+"\n" && got != c.want {
			t.Errorf("%s: the clone renders %q, wanted %q", c.name, got, c.want)
		}
		if got := original.Render(body); got != cloneBase+"\n" && got != cloneBase {
			t.Errorf("%s: writing the clone changed the original, which renders %q", c.name, got)
		}
		original.Set("w", "7")
		if got := original.Render(body); got != originalWrite+"\n" && got != originalWrite {
			t.Errorf("%s: the original renders %q after its own write, wanted %q", c.name, got, originalWrite)
		}
		if got := clone.Render(body); got != c.want+"\n" && got != c.want {
			t.Errorf("%s: writing the original changed the clone, which renders %q", c.name, got)
		}
		// The reverse order: the original written first, then the clone.
		first, _ := ParseAnchor(cloneBase)
		second := first.Clone()
		c.write(first)
		if got := second.Render(body); got != cloneBase+"\n" && got != cloneBase {
			t.Errorf("%s: writing the original changed a clone taken before it, which renders %q", c.name, got)
		}
	}
	if len(cases) != 6 {
		t.Fatalf("ran %d writers, and the header has six", len(cases))
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "frontmatter.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	writers := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Body == nil || fn.Name.Name == "own" {
			continue
		}
		receiver := fn.Recv.List[0].Names
		if len(receiver) == 0 {
			continue
		}
		name := receiver[0].Name
		ownAt, writeAt := token.NoPos, token.NoPos
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			switch x := node.(type) {
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "own" && isIdent(sel.X, name) && ownAt == token.NoPos {
					ownAt = x.Pos()
				}
				if ident, ok := x.Fun.(*ast.Ident); ok && ident.Name == "delete" && len(x.Args) > 0 && namesReceiverField(x.Args[0], name) && writeAt == token.NoPos {
					writeAt = x.Pos()
				}
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					if namesReceiverField(lhs, name) && writeAt == token.NoPos {
						writeAt = x.Pos()
					}
				}
			}
			return true
		})
		if writeAt == token.NoPos {
			continue
		}
		writers++
		if fn.Name.Name == "Clone" || fn.Name.Name == "markShared" {
			continue
		}
		if ownAt == token.NoPos || ownAt > writeAt {
			t.Errorf("%s writes the header's keys or block at %s without calling own first", fn.Name.Name, fset.Position(writeAt))
		}
	}
	if writers < 6 {
		t.Fatalf("found %d methods writing the header's keys or block, and there are at least six", writers)
	}
	t.Logf("checked %d methods writing the header's keys or block", writers)
}

// namesReceiverField reports whether an expression is the receiver's keys or
// block, or an element of either.
func namesReceiverField(expr ast.Expr, receiver string) bool {
	if index, ok := expr.(*ast.IndexExpr); ok {
		expr = index.X
	}
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && isIdent(sel.X, receiver) && (sel.Sel.Name == "keys" || sel.Sel.Name == "block")
}

// isIdent reports whether an expression is the named identifier.
func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}
