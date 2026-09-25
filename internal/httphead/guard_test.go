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

// sealedUse is every name the head may select from package answer. Each
// either composes nothing or answers with an answer.Sealed. Encode is absent
// because it encodes any value: the head encodes an answer only through
// Sealed.Encode and its own table only through EncodeAffordanceTable, which
// takes rows and nothing else.
var sealedUse = map[string]bool{
	"AffordanceRow":         true,
	"Affordances":           true,
	"Build":                 true,
	"EncodeAffordanceTable": true,
	"FormRoute":             true,
	"FromErrorSealed":       true,
	"RefusalSealed":         true,
	// RefusalValues composes no answer: it collects the values a refusal's
	// sentence names, which the pages' error page and command log render.
	"RefusalValues": true,
	"RunSealed":     true,
	"Sealed":        true,
}

// checkComposition parses every non-test Go file in dir and reports each
// place the file composes an answer of its own, or takes one from package
// answer in a form it could rewrite, one line per violation naming the file,
// the line and the rule:
//
//  1. a dot import or a blank import of verb, answer or contract, or an
//     import of reflect or unsafe, each of which lets a value be built out
//     of the walk's sight;
//  2. any selector verb.ComposeRefusal, called or taken as a value;
//  3. any selector FromError whose operand is not the name bound to package
//     answer, which covers a method call on a library, a method value and
//     a method expression;
//  4. any selector verb.Response, so nothing in the head may build, hold or
//     name a response, even through a pointer;
//  5. any selector on package answer outside sealedUse, which keeps the
//     head off Run, Refusal and FromError, the three that hand back the
//     *verb.Response itself, and off Encode, which would encode a value the
//     head built from the copy Sealed.Affordances returns.
//
// The affordance list is protected by a type rather than by this guard. The
// head holds every answer as an answer.Sealed, whose fields are unexported,
// so the compiler refuses a write to the list in any form, and
// TestASealedAnswerHandsOutACopy in package answer holds its one accessor,
// Affordances, to returning a copy. Rules 4 and 5 keep the head on that type.
//
// The guard reads one directory, no types, and no value's path from one
// name to another, so code can still compose an answer past it, and what
// it composes that way it can also rewrite. The shapes below are the ones
// known to pass, and a parse cannot close the list, each with a
// reproduction:
//
//   - A library act called directly in place of answer.RunSealed:
//     payload := x.library.Do(req)
//   - A helper in another package returning a *verb.Response it built:
//     payload := elsewhere.Refuse(req) // elsewhere builds &verb.Response{...}
//
// A generic allocator or json.Unmarshal can reach a response only through a
// value one of those two shapes produced, since naming the type is rule 4.
//
// TestEveryPublishedAffordanceHasARow catches part of this and no more. It
// fails when an answer publishes a name the affordance table has no row for,
// and when one of the head's own refusals it pins lacks next_card. It drives
// every act route into a refusal before a card is found, where the library
// puts its untranslated next, so a route answering through either shape
// without the translation publishes next there and fails it. A rewrite of
// such an answer that drops, reorders or repeats names the table already
// carries passes this guard and that test.
//
// The seal covers the answer and not the bytes Sealed.Encode returns. Rule 5
// keeps answer.Encode out of the head, so the head cannot encode a map it
// built from the copy Sealed.Affordances returns through package answer
// (rule5_encode_a_rewritten_copy.go). It can still do so through
// encoding/json, which it imports to read request bodies, and a rewrite of
// the bytes names nothing any rule reads:
//
//	encoded, err := answered.Encode()
//	var members map[string]any
//	json.Unmarshal(encoded, &members) // then reorder members["affordances"]
//	encoded, err = json.MarshalIndent(members, "", "  ")
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
	ast.Inspect(file, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		operand, _ := selector.X.(*ast.Ident)
		onVerb := operand != nil && verbName != "" && operand.Name == verbName
		onAnswer := operand != nil && answerName != "" && operand.Name == answerName
		name := selector.Sel.Name
		if onVerb && name == "ComposeRefusal" {
			report(selector.Pos(), 2, "verb.ComposeRefusal")
		}
		if name == "FromError" && !onAnswer {
			report(selector.Pos(), 3, "FromError on something other than package answer")
		}
		if onVerb && name == "Response" {
			report(selector.Pos(), 4, "names verb.Response")
		}
		if onAnswer && !sealedUse[name] {
			report(selector.Pos(), 5, "answer."+name+" answers with an unsealed response")
		}
		return true
	})
	return violations
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
