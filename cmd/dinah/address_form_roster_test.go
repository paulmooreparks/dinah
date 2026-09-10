package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"testing"

	"dinah/internal/addressform"
	"dinah/internal/verb"
)

// addressFormSource is the file the constants are declared in, read by the
// scan below rather than by a second hand-written list. A hand-written list
// would be the same declaration twice, and the failure it exists to catch,
// a constant minted with no declaration beside it, is exactly the failure
// two copies of one list cannot see.
const addressFormSource = "internal/addressform/addressform.go"

// declaredAddressFormConstants reads every constant of type AddressForm out of
// the package's own source, and says how many const specs it examined.
//
// The count is returned rather than folded into the caller for the reason
// testFunctionsUnder gives: a scan that read no file finds no constant, and
// "no constant of that type" and "no file read at all" are the same answer to
// a caller that cannot tell them apart.
func declaredAddressFormConstants(t *testing.T) ([]addressform.AddressForm, int) {
	t.Helper()
	path := filepath.Join(repositoryRoot, filepath.FromSlash(addressFormSource))
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	var declared []addressform.AddressForm
	specs := 0
	for _, decl := range parsed.Decls {
		general, ok := decl.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			specs++
			named, ok := value.Type.(*ast.Ident)
			if !ok || named.Name != "AddressForm" {
				continue
			}
			for _, expr := range value.Values {
				literal, ok := expr.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Errorf("a constant of type AddressForm in %s is not a string literal, so this scan cannot read its value", addressFormSource)
					continue
				}
				declared = append(declared, addressform.AddressForm(literal.Value[1:len(literal.Value)-1]))
			}
		}
	}
	return declared, specs
}

// TestEveryAddressFormConstantStandsInExactlyOneDeclaration holds the minted
// constants and the roster to each other in both directions.
//
// One direction catches a constant minted and never declared, which is the
// way this roster would rot: somebody adds a form to the package and stops
// before writing down which guide sentence teaches it. The other catches a
// declaration naming a constant that no longer exists, which the compiler
// already catches for a constant reference and does not catch for a value
// somebody typed by hand.
func TestEveryAddressFormConstantStandsInExactlyOneDeclaration(t *testing.T) {
	declared, specs := declaredAddressFormConstants(t)
	if specs == 0 {
		t.Fatalf("no const spec stands in %s, so this scan read nothing", addressFormSource)
	}
	if len(declared) == 0 {
		t.Fatalf("%s declares no constant of type AddressForm, so this sweep read nothing", addressFormSource)
	}

	roster := addressform.Declarations()
	if len(roster) == 0 {
		t.Fatal("the address-form roster is empty, so this sweep read nothing")
	}

	counted := map[addressform.AddressForm]int{}
	for _, entry := range roster {
		counted[entry.Form]++
	}
	for _, form := range declared {
		switch counted[form] {
		case 1:
		case 0:
			t.Errorf("%s mints the constant %q and no declaration names it, so the tool would accept a spelling nothing teaches", addressFormSource, form)
		default:
			t.Errorf("the roster names %q in %d declarations, and a form is declared once", form, counted[form])
		}
	}
	minted := map[addressform.AddressForm]bool{}
	for _, form := range declared {
		minted[form] = true
	}
	for _, entry := range roster {
		if !minted[entry.Form] {
			t.Errorf("the roster declares %q and %s mints no constant of that value", entry.Form, addressFormSource)
		}
	}
	t.Logf("%d constants of type AddressForm read from %d const specs, held against %d declarations", len(declared), specs, len(roster))
}

// TestEveryHeadFormNamesADeclaredReferenceKind asserts the kinds in both
// directions against a declared set, and asserts that a selector form carries
// none.
//
// The empty kind on a selector is a stated property rather than an omission.
// verb.ReferenceKind is the vocabulary of what a command's reference may name,
// and a selector names a step within a reference rather than a thing a
// reference names, so no value of it is honest for one. Below-card and
// collection are named by no head form because a reference reaches both by
// composing a head form with segments, and asserting the set in both
// directions is what records that rather than a comment.
func TestEveryHeadFormNamesADeclaredReferenceKind(t *testing.T) {
	roster := addressform.Declarations()
	if len(roster) == 0 {
		t.Fatal("the address-form roster is empty, so this sweep read nothing")
	}

	want := map[verb.ReferenceKind]bool{
		verb.ReferenceKindWorkbench:  true,
		verb.ReferenceKindWorkstream: true,
		verb.ReferenceKindColumn:     true,
		verb.ReferenceKindCard:       true,
	}
	named := map[verb.ReferenceKind]bool{}
	heads, selectors := 0, 0
	for _, entry := range roster {
		if entry.Kind == "" {
			selectors++
			continue
		}
		heads++
		if !want[entry.Kind] {
			t.Errorf("the head form %q names the reference kind %q, which no head form names: a head form names the workbench, a workstream, a column or a card", entry.Form, entry.Kind)
		}
		named[entry.Kind] = true
	}
	for kind := range want {
		if !named[kind] {
			t.Errorf("no head form names the reference kind %q, so this sweep proves nothing about how that kind is addressed", kind)
		}
	}

	// The selector forms are named rather than counted, so a roster that
	// quietly gave one a kind leaves a failure saying which.
	for _, selector := range []addressform.AddressForm{
		addressform.MemberPosition, addressform.MemberIdentifier, addressform.MemberName,
	} {
		for _, entry := range roster {
			if entry.Form == selector && entry.Kind != "" {
				t.Errorf("the selector form %q carries the reference kind %q, and a selector names a step within a reference rather than a thing a reference names, so no kind is honest for it", selector, entry.Kind)
			}
		}
	}
	if heads == 0 || selectors == 0 {
		t.Fatalf("the roster holds %d head forms and %d selector forms, and a half that read nothing proves nothing", heads, selectors)
	}
	t.Logf("%d head forms naming %d reference kinds, %d selector forms carrying none", heads, len(named), selectors)
}

// TestEveryDeclaredResolverFormIsADeclaredForm holds the resolver roster's
// Forms lists to the declared constants. A resolver row naming a form nobody
// declares is the roster disagreeing with itself, and it would leave the
// running guard with nothing to probe for that name.
func TestEveryDeclaredResolverFormIsADeclaredForm(t *testing.T) {
	roster := addressform.Resolvers()
	if len(roster) == 0 {
		t.Fatal("the resolver roster is empty, so this sweep read nothing")
	}
	declared := map[addressform.AddressForm]bool{}
	for _, entry := range addressform.Declarations() {
		declared[entry.Form] = true
	}
	if len(declared) == 0 {
		t.Fatal("the address-form roster is empty, so this sweep had nothing to compare against")
	}

	named, carrying := 0, 0
	for _, resolver := range roster {
		if len(resolver.Forms) > 0 {
			carrying++
		}
		for _, form := range resolver.Forms {
			named++
			if !declared[form] {
				t.Errorf("%s %s declares the form %q and no declaration names it", resolver.File, resolver.Function, form)
			}
		}
		if resolver.Why == "" {
			t.Errorf("%s %s carries no sentence saying what it resolves", resolver.File, resolver.Function)
		}
	}
	if named == 0 {
		t.Fatal("no resolver row names a form, so this sweep read nothing")
	}

	// The rows carrying no form are named rather than counted, because that
	// emptiness is a claim: those arms match a declared segment vocabulary or
	// a step of the containment grammar rather than a property of an entity.
	var formless []string
	for _, resolver := range roster {
		if len(resolver.Forms) == 0 {
			formless = append(formless, resolver.Function)
		}
	}
	sort.Strings(formless)
	t.Logf("%d resolver rows, %d of them naming %d form references; the rows naming none are %v", len(roster), carrying, named, formless)
}
