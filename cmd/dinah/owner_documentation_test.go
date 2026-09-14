package main

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// ownerEnforcers are the functions that enforce bench.ItemOwnerOperator
// against the actor. closeItem refuses resolve, verify and fail on an item
// recording that value to anybody but the operator, and SetField refuses a
// rewrite of the item's own owner key on one, so the record cannot be edited
// out from under the refusal.
//
// Every doc comment in ownerDeclarations has to name both. Naming only the
// first would leave a reader believing the record is what protects the
// refusal, which is the half of the rule dinah-484 added second.
var ownerEnforcers = []string{"closeItem", "SetField"}

// ownerDeclarations are the declarations of the tree that document the
// checklist item's owner field, spelled by the struct-qualified name rather
// than by the bare identifier. The first two are constants of internal/bench,
// the third a field of that package's Item, and the last two fields of
// internal/verb's Request and its item view.
//
// The qualification is not decoration. Three of these five are fields spelled
// Owner, in three structs across two packages, so a walk keyed on the bare
// identifier cannot say which declaration it resolved and the equality below
// would mean nothing. That cannot be shown by breaking this guard, because on
// today's tree both keyings resolve the identical five and the guard passes
// under either; what shows it is a plant on the tree, which dinah-495 AC-12
// records. Flip the argument to resolveOwnerDeclarations to perform it.
//
// The package is left off the key rather than written into it, because the
// product's word guard reads a Go string literal as text a person meets and
// refuses the short spelling of workbench in one. The walk records the
// package it resolved each declaration in and a failure prints it.
var ownerDeclarations = []string{
	"ItemOwnerField",
	"ItemOwnerOperator",
	"Item.Owner",
	"Request.Owner",
	"ItemView.Owner",
}

// ownerDeclaration is one declaration the walk resolved, under both of the
// names it can be keyed by, with the doc comment that hangs on it.
type ownerDeclaration struct {
	// qualified is the struct-qualified name, as ownerDeclarations spells
	// it, and pkg is the package it was resolved in, which a failure prints
	// beside it.
	qualified string
	pkg       string
	// bare is the declared identifier alone, which is what an unqualified
	// walk would key on.
	bare string
	// doc is the declaration's doc comment, flattened to single spaces, so
	// a claim broken across a line break is one string. Two of the comments
	// this guard was built to catch were broken exactly that way and
	// defeated a search for their own words.
	doc string
}

// resolveOwnerDeclarations walks every non-test file of the tree and returns
// the declarations named by ownerDeclarations, together with every function
// and method name the tree declares.
//
// When qualify is false it keys on the bare identifier instead, which is the
// keying dinah-495 AC-12's plant exists to discredit. Nothing in the suite
// calls it that way; the argument is there so that a reader can perform the
// plant rather than take the reasoning on trust.
func resolveOwnerDeclarations(t *testing.T, qualify bool) ([]ownerDeclaration, map[string]bool) {
	t.Helper()
	wanted := map[string]bool{}
	for _, name := range ownerDeclarations {
		if qualify {
			wanted[name] = true
			continue
		}
		at := strings.LastIndex(name, ".")
		wanted[name[at+1:]] = true
	}

	fileset := token.NewFileSet()
	sources := readTreeSources(t, fileset)
	functions := map[string]bool{}
	var found []ownerDeclaration
	keep := func(declaration ownerDeclaration) {
		key := declaration.qualified
		if !qualify {
			key = declaration.bare
		}
		if wanted[key] {
			found = append(found, declaration)
		}
	}
	for _, source := range sources {
		pkg := source.file.Name.Name
		for _, declared := range source.file.Decls {
			switch typed := declared.(type) {
			case *ast.FuncDecl:
				functions[typed.Name.Name] = true
			case *ast.GenDecl:
				for _, spec := range typed.Specs {
					switch specified := spec.(type) {
					case *ast.ValueSpec:
						doc := specified.Doc
						if doc == nil && len(typed.Specs) == 1 {
							doc = typed.Doc
						}
						for _, name := range specified.Names {
							keep(ownerDeclaration{
								qualified: name.Name,
								pkg:       pkg,
								bare:      name.Name,
								doc:       flattenWords(doc.Text()),
							})
						}
					case *ast.TypeSpec:
						structure, isStruct := specified.Type.(*ast.StructType)
						if !isStruct || structure.Fields == nil {
							continue
						}
						for _, field := range structure.Fields.List {
							for _, name := range field.Names {
								keep(ownerDeclaration{
									qualified: specified.Name.Name + "." + name.Name,
									pkg:       pkg,
									bare:      name.Name,
									doc:       flattenWords(field.Doc.Text()),
								})
							}
						}
					}
				}
			}
		}
	}
	return found, functions
}

// TestEveryDeclarationOfTheItemOwnerNamesWhatEnforcesIt asserts that each
// declaration documenting the checklist item's owner field says what enforces
// the owner, and that the names it says resolve to functions the tree declares.
//
// dinah-484 made an item owned by the operator settleable by the operator
// alone, and three doc comments went on saying the field was recorded and
// never enforced. A reader who trusts one of them files a question in the
// operator's name believing it protects nothing, or builds on the belief that
// it protects everything, and each is wrong in a different direction.
//
// The guard keys on the declaration the sentence is about rather than on the
// file it sits in or the words it uses, which is what the guard dinah-484 left
// behind did not do. That one reads item.go by name and searches for a
// literal, so a copy one package away was outside its reach and spelled
// differently besides. Keying on the declaration survives a file rename, a
// file split, and a rewording that keeps the enforcing names.
//
// Two things this guard cannot see, said here because somebody meets it here.
//
// It holds the five declarations ownerDeclarations names and nothing else, so
// a sixth doc comment about the owner field on a sixth declaration, or the
// same claim written as ordinary prose rather than as a doc comment, leaves
// the suite green. That is an observed limit rather than a hypothesis: this
// guard was first specified over two declarations while five stood, and two of
// the three it had not been told about carried the false claim in a spelling
// no phrase search reached, which is how they were found.
//
// And it resolves each cited name to some declaration in the tree without
// checking that the declaration it resolves to is the one performing the
// enforcement. The tree declares SetField twice, at internal/verb/fields.go
// and internal/bench/workstream.go, so a comment citing the name resolves
// whichever of the two is renamed, and pointing one at an unrelated function
// of the right name leaves the suite green. Closing that would mean
// recognising enforcement rather than a string, which is the same problem one
// level down.
func TestEveryDeclarationOfTheItemOwnerNamesWhatEnforcesIt(t *testing.T) {
	found, functions := resolveOwnerDeclarations(t, true)
	if len(functions) == 0 {
		t.Fatal("the walk collected no function name, so the citation check below would pass over any name at all")
	}
	if len(found) != len(ownerDeclarations) {
		var resolved []string
		for _, declaration := range found {
			resolved = append(resolved, declaration.pkg+"."+declaration.qualified)
		}
		sort.Strings(resolved)
		t.Fatalf("the walk resolved %d declarations and dinah-495 names %d: resolved %v, named %v",
			len(found), len(ownerDeclarations), resolved, ownerDeclarations)
	}
	for _, enforcer := range ownerEnforcers {
		if !functions[enforcer] {
			t.Errorf("the comments cite %s as an enforcer of the item owner and the tree declares no such function, so every one of them names something that is not there", enforcer)
		}
	}
	for _, declaration := range found {
		if declaration.doc == "" {
			t.Errorf("%s.%s carries no doc comment, so nothing there says what enforces the owner", declaration.pkg, declaration.qualified)
			continue
		}
		for _, enforcer := range ownerEnforcers {
			if strings.Contains(declaration.doc, enforcer) {
				continue
			}
			t.Errorf("%s.%s's doc comment does not name %s, so it does not say what enforces the owner: %q",
				declaration.pkg, declaration.qualified, enforcer, declaration.doc)
		}
	}
}
