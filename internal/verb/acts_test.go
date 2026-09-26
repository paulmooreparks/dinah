package verb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// actFunctions are the functions an act runs outside its shared checks, which
// the tripwire below reads.
var actFunctions = []string{"Do", "evaluate", "claim", "release", "move", "Comment"}

// TestNoActRefusesOutsideItsSharedChecks is a tripwire for one shape only: a
// direct call to refuse or refuseWith written inside Do, evaluate, claim,
// release, move or Comment in mutate.go and beyond.go, other than the call
// evaluate makes for contract.UnknownVerb. Such a call would refuse the act
// and not the offer, since OfferActs asks only the shared check functions.
//
// It does not protect the offer, and it cannot see these shapes, each of
// which reaches the act and not the offer just the same:
//
//   - a refusal raised in a helper those functions call, in the same file or
//     another file of the package;
//   - a refusal raised through a method value, such as r := l.refuse; return
//     r(...);
//   - a &Response{Outcome: contract.OutcomeRefused, ...} literal written by
//     hand;
//   - an error returned by a lower layer that FromError turns into a refusal.
//
// What protects the offer is that the rows live in the shared check
// functions the acts themselves call, and TestTheOfferMatchesEveryAct in
// cmd/dinah, which compares the offer with real acts state by state.
func TestNoActRefusesOutsideItsSharedChecks(t *testing.T) {
	found := map[string]bool{}
	for _, name := range []string{"mutate.go", "beyond.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Body == nil || function.Recv == nil || !slices.Contains(actFunctions, function.Name.Name) {
				continue
			}
			found[function.Name.Name] = true
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (selector.Sel.Name != "refuse" && selector.Sel.Name != "refuseWith") {
					return true
				}
				if function.Name.Name == "evaluate" && namesUnknownVerb(call) {
					return true
				}
				t.Errorf("%s: %s calls %s directly, so the act refuses where the offer cannot see it; move the row into the act's shared check function",
					name, function.Name.Name, selector.Sel.Name)
				return true
			})
		}
	}
	for _, name := range actFunctions {
		if !found[name] {
			t.Errorf("the tripwire did not find %s in mutate.go or beyond.go, so it reads nothing there", name)
		}
	}
	t.Logf("read %d of the %d act functions", len(found), len(actFunctions))
}

// namesUnknownVerb reports whether a refuse call's refusal argument is
// contract.UnknownVerb.
func namesUnknownVerb(call *ast.CallExpr) bool {
	for _, argument := range call.Args {
		selector, ok := argument.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "UnknownVerb" {
			return true
		}
	}
	return false
}
