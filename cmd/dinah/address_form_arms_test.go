package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/addressform"
)

// addressAnsweringResultTypes are the entity types a function answers when it
// answers a caller's string with a thing rather than with a path.
//
// The subject set is keyed on the result type rather than on an inference from
// a name, which is what lets it find a resolver nobody enumerated. Its stated
// cost is that a resolver answering a type outside this set is invisible to
// it, and five rostered functions are exactly that case: resolvePathBody,
// walkBelowCard, descend and pick answer a string, and ResolveLinkTarget
// answers a string and a refusal.
var addressAnsweringResultTypes = map[string]bool{
	"Column": true, "Workstream": true, "Card": true, "Resolved": true,
	"EntityRef": true, "CollectionRef": true, "Attachment": true,
	"Item": true, "Comment": true,
}

// benchFunction is one function declaration read out of internal/bench, with
// the two arm counts taken over its own body.
type benchFunction struct {
	file      string
	name      string
	returns   int
	accepting int
	subject   bool
}

// namesAnEntity reports whether a result type is one of the entity types,
// reached by pointer or by slice.
func namesAnEntity(node ast.Expr) bool {
	switch typed := node.(type) {
	case *ast.StarExpr:
		return namesAnEntity(typed.X)
	case *ast.ArrayType:
		return namesAnEntity(typed.Elt)
	case *ast.Ident:
		return addressAnsweringResultTypes[typed.Name]
	}
	return false
}

// namesAString reports whether a parameter's type is a plain string, which is
// what "takes a caller's spelling" means here.
func namesAString(node ast.Expr) bool {
	named, ok := node.(*ast.Ident)
	return ok && named.Name == "string"
}

// isNilLiteral reports whether an expression is the bare identifier nil.
func isNilLiteral(node ast.Expr) bool {
	named, ok := node.(*ast.Ident)
	return ok && named.Name == "nil"
}

// armCounts counts the two numbers over a function's own body, descending into
// no function literal, because an arm of a closure is not an arm of the
// function that declares it.
//
// Returns is every return statement. Accepting is every return whose first
// result is not the literal nil and whose last result, on a return of more
// than one value, is the literal nil.
func armCounts(body *ast.BlockStmt) (returns, accepting int) {
	ast.Inspect(body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			returns++
			if len(typed.Results) == 0 {
				return true
			}
			if isNilLiteral(typed.Results[0]) {
				return true
			}
			if len(typed.Results) > 1 && !isNilLiteral(typed.Results[len(typed.Results)-1]) {
				return true
			}
			accepting++
		}
		return true
	})
	return returns, accepting
}

// readBenchFunctions parses every non-test .go file in internal/bench and
// answers every function declared in them, with its arm counts and whether it
// stands in the subject set. It also says how many files it read, because a
// scan that read no file finds no function.
func readBenchFunctions(t *testing.T) ([]benchFunction, int) {
	t.Helper()
	dir := filepath.Join(repositoryRoot, filepath.FromSlash(addressform.PackageDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	fileset := token.NewFileSet()
	var functions []benchFunction
	files := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files++
		for _, decl := range parsed.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			read := benchFunction{file: name, name: function.Name.Name}
			if function.Body != nil {
				read.returns, read.accepting = armCounts(function.Body)
			}
			takesAString := false
			if function.Type.Params != nil {
				for _, param := range function.Type.Params.List {
					if namesAString(param.Type) {
						takesAString = true
					}
				}
			}
			answersAnEntity := false
			if function.Type.Results != nil {
				for _, result := range function.Type.Results.List {
					if namesAnEntity(result.Type) {
						answersAnEntity = true
					}
				}
			}
			read.subject = takesAString && answersAnEntity
			functions = append(functions, read)
		}
	}
	return functions, files
}

// TestEveryDeclaredResolverCarriesTheArmsItDeclares recomputes both arm counts
// for every rostered function and holds the roster to them.
//
// Two counts rather than one, because each catches what the other misses.
// Accepting is the meaningful number and it pairs with the Forms list, and it
// does not see an arm whose return delegates and passes an error through, of
// the shape `return path, err`. Returns sees every arm, because an arm that
// answers has a return, and it also moves when an unrelated refusal is added.
// Requiring both is what makes a new arm in ColumnByRef redden the run, which
// is the hole this guard exists to close: three of the rostered functions
// build neither of the two structs an earlier design was going to scan, and
// internal/verb calls them directly.
func TestEveryDeclaredResolverCarriesTheArmsItDeclares(t *testing.T) {
	roster := addressform.Resolvers()
	if len(roster) == 0 {
		t.Fatal("the resolver roster is empty, so this sweep read nothing")
	}
	functions, files := readBenchFunctions(t)
	if files == 0 {
		t.Fatalf("no non-test .go file stands in %s, so this scan read nothing", addressform.PackageDir)
	}
	if len(functions) == 0 {
		t.Fatalf("no function is declared in %s, so this scan read nothing", addressform.PackageDir)
	}

	read := map[string]benchFunction{}
	for _, function := range functions {
		read[function.file+" "+function.name] = function
	}

	checked := 0
	for _, resolver := range roster {
		key := resolver.File + " " + resolver.Function
		found, ok := read[key]
		if !ok {
			t.Errorf("the roster names %s in %s/%s and no function of that name is declared there", resolver.Function, addressform.PackageDir, resolver.File)
			continue
		}
		checked++
		if found.returns != resolver.Returns || found.accepting != resolver.Accepting {
			t.Errorf("%s %s carries %d returns and %d accepting arms, and the roster declares %d and %d: declare the form the new arm accepts, or correct the counts",
				resolver.File, resolver.Function, found.returns, found.accepting, resolver.Returns, resolver.Accepting)
			continue
		}
		t.Logf("%s %s returns=%d accepting=%d", resolver.File, resolver.Function, found.returns, found.accepting)
	}
	if checked != len(roster) {
		t.Errorf("%d of the %d rostered functions were found and counted, so this sweep read less than the roster it claims", checked, len(roster))
	}
	t.Logf("%d rostered functions counted over %d files of %s", checked, files, addressform.PackageDir)
}

// TestEveryAddressAnsweringFunctionIsRosteredOrExempted sweeps the whole
// package for functions that answer a caller's string with an entity, and
// requires each to be rostered or exempted on a stated ground.
//
// This is what closes the hole the arm-count guard alone leaves. A nineteenth
// address-answering function added to internal/bench is found by the sweep,
// and it fails the run until somebody either rosters it with its arm counts
// and its forms, or exempts it on one of the four grounds.
//
// The reverse direction is asserted of the exemption roster alone. Five
// rostered functions answer a string rather than an entity and stand outside a
// subject set keyed on entity result types, so asserting it of the resolver
// roster would reject the roster this package declares. Those five are not
// left unchecked: the arm-count guard above fails when a roster row names a
// function that does not stand in the file the row names.
func TestEveryAddressAnsweringFunctionIsRosteredOrExempted(t *testing.T) {
	functions, files := readBenchFunctions(t)
	if files == 0 {
		t.Fatalf("no non-test .go file stands in %s, so this scan read nothing", addressform.PackageDir)
	}

	subject := map[string]bool{}
	for _, function := range functions {
		if function.subject {
			subject[function.name] = true
		}
	}
	if len(subject) == 0 {
		t.Fatalf("no function in %s takes a string and answers an entity, so this sweep read nothing", addressform.PackageDir)
	}

	rostered := map[string]bool{}
	for _, resolver := range addressform.Resolvers() {
		if rostered[resolver.Function] {
			t.Errorf("the resolver roster names %s twice", resolver.Function)
		}
		rostered[resolver.Function] = true
	}
	if len(rostered) == 0 {
		t.Fatal("the resolver roster is empty, so this sweep had nothing to compare against")
	}

	grounds := map[string]bool{}
	for _, ground := range addressform.Grounds() {
		grounds[ground] = true
	}
	if len(grounds) == 0 {
		t.Fatal("no exemption ground is declared, so every exemption would be unreadable")
	}

	exempted := map[string]bool{}
	for _, exemption := range addressform.Exemptions() {
		if !grounds[exemption.Ground] {
			t.Errorf("%s is exempted on the ground %q, which is outside the closed set %v", exemption.Function, exemption.Ground, addressform.Grounds())
		}
		if strings.TrimSpace(exemption.Reason) == "" {
			t.Errorf("%s is exempted with no reason, and a ground is an argument rather than a word", exemption.Function)
		}
		if exempted[exemption.Function] {
			t.Errorf("the exemption roster names %s twice", exemption.Function)
		}
		exempted[exemption.Function] = true
		if !subject[exemption.Function] {
			t.Errorf("%s is exempted from this sweep and the sweep of %s does not find it, so the exemption excuses nothing", exemption.Function, addressform.PackageDir)
		}
		if rostered[exemption.Function] {
			t.Errorf("%s stands in the resolver roster and in the exemption roster, and a function is one or the other", exemption.Function)
		}
	}
	if len(exempted) == 0 {
		t.Fatal("the exemption roster is empty, so this sweep read nothing of it")
	}

	var uncovered []string
	rosteredInSubject := 0
	for name := range subject {
		switch {
		case rostered[name]:
			rosteredInSubject++
		case exempted[name]:
		default:
			uncovered = append(uncovered, name)
		}
	}
	sort.Strings(uncovered)
	for _, name := range uncovered {
		t.Errorf("%s in %s takes a string and answers an entity, and neither roster names it: roster it with its arm counts and the forms it accepts, or exempt it on a stated ground", name, addressform.PackageDir)
	}

	if rosteredInSubject+len(exempted) != len(subject) {
		t.Errorf("%d rostered functions stand in the subject set and %d are exempted, which comes to %d against a subject set of %d, so the partition does not close",
			rosteredInSubject, len(exempted), rosteredInSubject+len(exempted), len(subject))
	}
	t.Logf("%d functions declared over %d files of %s; subject set %d, of which %d rostered and %d exempted",
		len(functions), files, addressform.PackageDir, len(subject), rosteredInSubject, len(exempted))
}
