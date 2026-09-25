package httphead

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The three packages whose names the composition guard tracks, by import
// path.
const (
	verbPath     = "dinah/internal/verb"
	answerPath   = "dinah/internal/answer"
	contractPath = "dinah/internal/contract"
)

// checkComposition parses every non-test Go file in dir and reports each
// place the file composes an answer of its own rather than taking one from
// package answer, one line per violation naming the file, the line and the
// rule:
//
//  1. a dot import or a blank import of verb, answer or contract, or an
//     import of reflect or unsafe, each of which lets a value be built out
//     of the walk's sight;
//  2. any selector verb.ComposeRefusal, called or taken as a value;
//  3. any selector FromError whose operand is not the name bound to package
//     answer, which covers a method call on a library, a method value and a
//     method expression;
//  4. any selector verb.Response whose parent is not a star, so a signature,
//     a field or a variable may hold *verb.Response and nothing may build or
//     hold a verb.Response itself;
//  5. any assignment, increment, append or copy whose target, and any
//     address-of whose operand, reduces to a selector named Affordances once
//     writeRoot has stripped its index, slice, parenthesis and dereference
//     layers.
//
// The guard reads one directory, no types, and no value's path from one
// name to another, so code can still rewrite or compose an answer past it.
// The shapes below are the ones known to pass, and a parse cannot close the
// list, each with a reproduction:
//
//   - A library act called directly in place of answer.Run:
//     payload := x.library.Do(req)
//   - A helper in another package returning a *verb.Response it built:
//     payload := elsewhere.Refuse(req) // elsewhere builds &verb.Response{...}
//   - A generic allocator whose type argument is inferred from a
//     *verb.Response variable:
//     func alloc[T any](_ *T) *T { return new(T) }; payload := alloc(existing)
//   - json.Unmarshal into a *verb.Response:
//     var payload *verb.Response; json.Unmarshal(bytes, &payload)
//   - The affordance list handed to a function that rewrites it in place:
//     sort.Strings(response.Affordances)
//     slices.Reverse(response.Affordances)
//   - The affordance list copied into a variable and written through it:
//     list := response.Affordances; list[0] = "status"
//
// TestEveryPublishedAffordanceHasARow catches part of this and no more. It
// fails when an answer publishes a name the affordance table has no row for,
// and when one of the head's own refusals it pins lacks next_card. It drives
// every act route into a refusal before a card is found, where the library
// puts its untranslated next, so a route answering through any of the first
// four shapes publishes next there and fails it. The last two fail it only when they introduce a name without a row
// or push next_card off a pinned refusal. A rewrite that drops, reorders or
// repeats names the table already carries passes this guard and that test.
func checkComposition(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", dir, err)}
	}
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		violations = append(violations, checkFile(fset, name, file)...)
	}
	return violations
}

// checkFile applies the five rules to one parsed file.
func checkFile(fset *token.FileSet, name string, file *ast.File) []string {
	var violations []string
	report := func(at token.Pos, rule int, what string) {
		violations = append(violations, fmt.Sprintf("%s:%d: rule %d: %s", name, fset.Position(at).Line, rule, what))
	}
	bound := map[string]string{}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		switch path {
		case "reflect", "unsafe":
			report(spec.Pos(), 1, "imports "+path)
			continue
		case verbPath, answerPath, contractPath:
		default:
			continue
		}
		local := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			local = spec.Name.Name
		}
		if local == "." || local == "_" {
			report(spec.Pos(), 1, local+" import of "+path)
			continue
		}
		bound[path] = local
	}
	verbName, answerName := bound[verbPath], bound[answerPath]
	affordances := func(target ast.Expr) bool {
		selector, ok := writeRoot(target).(*ast.SelectorExpr)
		return ok && selector.Sel.Name == "Affordances"
	}
	var parents []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			parents = parents[:len(parents)-1]
			return true
		}
		var parent ast.Node
		if len(parents) > 0 {
			parent = parents[len(parents)-1]
		}
		parents = append(parents, n)
		switch node := n.(type) {
		case *ast.SelectorExpr:
			operand, _ := node.X.(*ast.Ident)
			onVerb := operand != nil && verbName != "" && operand.Name == verbName
			if onVerb && node.Sel.Name == "ComposeRefusal" {
				report(node.Pos(), 2, "verb.ComposeRefusal")
			}
			if node.Sel.Name == "FromError" && (operand == nil || answerName == "" || operand.Name != answerName) {
				report(node.Pos(), 3, "FromError on something other than package answer")
			}
			if onVerb && node.Sel.Name == "Response" {
				if _, star := parent.(*ast.StarExpr); !star {
					report(node.Pos(), 4, "verb.Response outside a pointer")
				}
			}
		case *ast.AssignStmt:
			for _, target := range node.Lhs {
				if affordances(target) {
					report(node.Pos(), 5, "assigns to Affordances")
				}
			}
		case *ast.IncDecStmt:
			if affordances(node.X) {
				report(node.Pos(), 5, "increments Affordances")
			}
		case *ast.CallExpr:
			if callee, ok := node.Fun.(*ast.Ident); ok && (callee.Name == "append" || callee.Name == "copy") && len(node.Args) > 0 && affordances(node.Args[0]) {
				report(node.Pos(), 5, callee.Name+" into Affordances")
			}
		case *ast.UnaryExpr:
			if node.Op == token.AND && affordances(node.X) {
				report(node.Pos(), 5, "takes the address of Affordances")
			}
		}
		return true
	})
	return violations
}

// writeRoot reduces an expression written to, or whose address is taken, to
// the expression that owns the storage: it strips index, slice, parenthesis
// and pointer-dereference layers until none is left, so
// (*x).Affordances[i], x.Affordances[:1] and (x.Affordances)[0] all reduce to
// a selector named Affordances.
func writeRoot(target ast.Expr) ast.Expr {
	for {
		switch layer := target.(type) {
		case *ast.IndexExpr:
			target = layer.X
		case *ast.SliceExpr:
			target = layer.X
		case *ast.ParenExpr:
			target = layer.X
		case *ast.StarExpr:
			target = layer.X
		default:
			return target
		}
	}
}

// plantedRule reads the rule a planted file was written for off its name,
// which begins rule<N>_.
var plantedRule = regexp.MustCompile(`^rule([0-9])_`)

// TestHeadComposesNoAnswer is the composition guard of dinah-152/criteria/16.
// It runs over this package and wants nothing, and over the planted files,
// each written as the code a rule stops, and wants each file caught by the
// rule it was written for and by no other, so the guard proves on every run
// that it catches what it forbids.
func TestHeadComposesNoAnswer(t *testing.T) {
	if violations := checkComposition("."); len(violations) > 0 {
		t.Errorf("the head composes answers of its own:\n%s", strings.Join(violations, "\n"))
	}
	planted := filepath.Join("testdata", "guard")
	entries, err := os.ReadDir(planted)
	if err != nil {
		t.Fatalf("read the planted files: %v", err)
	}
	byFile := map[string][]string{}
	for _, violation := range checkComposition(planted) {
		name, _, _ := strings.Cut(violation, ":")
		byFile[name] = append(byFile[name], violation)
	}
	perRule := map[string]int{}
	for _, entry := range entries {
		name := entry.Name()
		match := plantedRule.FindStringSubmatch(name)
		if match == nil {
			t.Errorf("%s is planted and its name says no rule", name)
			continue
		}
		perRule[match[1]]++
		found := byFile[name]
		if len(found) == 0 {
			t.Errorf("%s was written for rule %s and the guard found nothing in it", name, match[1])
		}
		for _, violation := range found {
			if !strings.Contains(violation, ": rule "+match[1]+": ") {
				t.Errorf("%s was written for rule %s and the guard reported %s", name, match[1], violation)
			}
		}
	}
	var rules []string
	for rule, count := range perRule {
		rules = append(rules, fmt.Sprintf("rule %s: %d", rule, count))
	}
	sort.Strings(rules)
	if len(perRule) != 5 {
		t.Errorf("the planted files cover %d rules, wanted all five: %v", len(perRule), rules)
	}
	t.Logf("%d planted files caught, %s", len(entries), strings.Join(rules, ", "))
}
