package acp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Connection diagnostics must retain the registered redaction policy when a
// future call site is added. Inspect syntax rather than matching source text.
func TestDiagnosticsCannotBypassConnectionRedaction(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sinks := 0
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		var allowed *ast.CallExpr
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "logf" || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			ptr, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			recv, ok := ptr.X.(*ast.Ident)
			if !ok || recv.Name != "Conn" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if ok && id.Name == "Logf" {
					allowed = call
					sinks++
				}
				return true
			})
		}
		ast.Inspect(f, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || id.Name != "Logf" {
				return true
			}
			// Only its declaration and the direct call inside Conn.logf are allowed.
			if id.Obj != nil && id.Obj.Pos() == id.Pos() {
				return true
			}
			if allowed != nil && allowed.Fun == id {
				return true
			}
			t.Errorf("%s bypasses the connection diagnostic sink", fset.Position(id.Pos()))
			return true
		})
	}
	if sinks != 1 {
		t.Fatalf("expected one redacting sink, got %d", sinks)
	}
}
