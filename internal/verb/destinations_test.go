package verb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// refusalSources are the calls in this package that compose a refusal. A
// function raising one of them decides, on its own, that an act is refused.
var refusalSources = map[string]bool{"refuse": true, "refuseWith": true, "malformedHarness": true}

// TestNoMoveRefusalIsRaisedOutsideTheSharedChecks is what keeps the move
// filter from drifting from the move it predicts. MoveDestinations calls admit
// and canMove, and a real move is refused through exactly those two, so a
// check added to either reaches both. A check added anywhere else in Do or in
// move would reach the move alone and be missed by the filter, and no fixture
// could be relied on to exercise it, so this test reads the source instead:
// it fails when Do or move composes a refusal itself, and when any of the
// three stops calling the shared function it relies on.
func TestNoMoveRefusalIsRaisedOutsideTheSharedChecks(t *testing.T) {
	files := token.NewFileSet()
	bodies := map[string]*ast.BlockStmt{}
	for _, name := range []string{"mutate.go", "destinations.go"} {
		file, err := parser.ParseFile(files, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Body != nil {
				bodies[fn.Name.Name] = fn.Body
			}
		}
	}
	calls := func(name string) map[string]int {
		body, ok := bodies[name]
		if !ok {
			t.Fatalf("no method %s was found to read", name)
		}
		called := map[string]int{}
		ast.Inspect(body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				if receiver, ok := selector.X.(*ast.Ident); ok && receiver.Name == "l" {
					called[selector.Sel.Name]++
				}
			}
			return true
		})
		return called
	}
	for _, name := range []string{"Do", "move"} {
		for source := range refusalSources {
			if n := calls(name)[source]; n > 0 {
				t.Errorf("%s calls l.%s %d times, so it refuses a move on a row MoveDestinations never runs; put the row in admit or in canMove", name, source, n)
			}
		}
	}
	required := map[string][]string{
		"Do":               {"admit"},
		"move":             {"canMove"},
		"MoveDestinations": {"admit", "canMove"},
	}
	checked := 0
	for name, needs := range required {
		for _, need := range needs {
			checked++
			if calls(name)[need] == 0 {
				t.Errorf("%s no longer calls %s, so the filter and the move no longer share it", name, need)
			}
		}
	}
	if checked != 4 {
		t.Errorf("checked %d shared calls, wanted 4", checked)
	}
}
