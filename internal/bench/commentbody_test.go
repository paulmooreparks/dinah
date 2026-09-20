package bench

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOnlyWriteCommentAnchorWritesACommentAnchor holds the property the digest
// rests on: it is recomputed by every verb that writes a comment's anchor,
// which is true of one function rather than of a rule each call site has to
// remember.
//
// The first shape of this card held it by remembering. internal/verb's field
// write stamped the digest and then wrote the file itself with WriteText, so
// the property was carried by two call sites agreeing, and a third writer
// added later would have carried it by agreeing too or not at all.
//
// The check reads the source rather than the behaviour, because what it is
// about is how many places can do the thing rather than what they do. A writer
// that stamped correctly would pass a behavioural check and still leave the
// rule one edit from being broken.
func TestOnlyWriteCommentAnchorWritesACommentAnchor(t *testing.T) {
	roots := []string{".", filepath.Join("..", "verb"), filepath.Join("..", "..", "cmd")}
	var writers []string
	walked := 0
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			walked++
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				// The digest is stamped in one place, and that place is
				// inside WriteCommentAnchor. Counting the stamp rather than
				// the write is what makes the check independent of how a
				// writer spells its destination: the earlier form read the
				// first argument for the anchor constant and went quiet the
				// moment a writer put that constant in a variable first,
				// which is the one-spelling defect this repository's own
				// counterexample corpus is about.
				if strings.HasSuffix(render(call.Fun), "StampCommentDigest") {
					writers = append(writers, fset.Position(call.Pos()).String())
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if walked == 0 {
		t.Fatal("no source file was parsed, so this check proved nothing")
	}
	if len(writers) != 1 {
		t.Errorf("%d call sites write a comment's anchor, wanted the one inside WriteCommentAnchor:\n%s",
			len(writers), strings.Join(writers, "\n"))
	}
}

// render prints one expression back as source, which is what lets the walk
// above ask about a selector without resolving it.
func render(node ast.Expr) string {
	var out strings.Builder
	switch typed := node.(type) {
	case *ast.Ident:
		out.WriteString(typed.Name)
	case *ast.SelectorExpr:
		out.WriteString(render(typed.X))
		out.WriteString(".")
		out.WriteString(typed.Sel.Name)
	case *ast.CallExpr:
		out.WriteString(render(typed.Fun))
		out.WriteString("(")
		for i, arg := range typed.Args {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(render(arg))
		}
		out.WriteString(")")
	}
	return out.String()
}
