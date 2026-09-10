package bench

import (
	"sort"
	"strings"
	"testing"
)

// TestEveryKindDeclaresItsFieldsAndItsAuthority holds the field declaration to
// the grammar in both directions, and holds every kind to a write authority
// drawn from the closed pair.
//
// The authority arm is the one that stops a kind slipping in with no rule
// about who may write it, which would otherwise default to whatever the router
// happens to do. It compares two independent declarations, the containment
// grammar and the authority table, rather than computing either from the
// other.
func TestEveryKindDeclaresItsFieldsAndItsAuthority(t *testing.T) {
	kinds := EntityKinds()
	if len(kinds) == 0 {
		t.Fatal("the grammar names no kind, so this check read nothing")
	}
	// The six the containment table names, plus the workstream, which the
	// table deliberately leaves out because a workstream is a membership
	// rather than a container.
	want := []string{
		KindAttachment, KindCard, KindColumn, KindComment,
		KindItem, KindWorkbench, KindWorkstream,
	}
	sort.Strings(want)
	if got := strings.Join(kinds, " "); got != strings.Join(want, " ") {
		t.Errorf("EntityKinds reports\n  %s\nand the grammar names\n  %s", got, strings.Join(want, " "))
	}

	authorities := map[string]bool{AuthorityOwner: true, AuthorityOperator: true}
	for _, kind := range kinds {
		names := FieldsOf(kind)
		if len(names) == 0 {
			t.Errorf("%s carries no field, so nothing about it is readable or writable", kind)
		}
		for _, name := range names {
			field, known := FieldOf(kind, name)
			if !known {
				t.Errorf("FieldsOf(%q) reports %s and FieldOf does not carry it", kind, name)
				continue
			}
			if field.Name != name {
				t.Errorf("FieldOf(%q, %q) answers the field named %q", kind, name, field.Name)
			}
			if field.Guard == "" {
				continue
			}
			declared := false
			for _, guard := range Guards {
				if guard == field.Guard {
					declared = true
				}
			}
			if !declared {
				t.Errorf("%s/%s declares the guard %q, which is not one of %v", kind, name, field.Guard, Guards)
			}
		}
		authority := WriteAuthorityOf(kind)
		if authority == "" {
			t.Errorf("%s declares no write authority, so nothing says who may write its fields", kind)
			continue
		}
		if !authorities[authority] {
			t.Errorf("%s declares the write authority %q, which is neither %s nor %s", kind, authority, AuthorityOwner, AuthorityOperator)
		}
	}

	// A kind the grammar does not carry reports no fields, is not in the
	// list, and answers no authority, which is what lets a sweep tell a kind
	// with no rule from a kind whose rule is the looser of the two.
	const invented = "frobnicate"
	if names := FieldsOf(invented); len(names) != 0 {
		t.Errorf("a kind the grammar does not name reports the fields %v", names)
	}
	if _, known := FieldOf(invented, TitleField); known {
		t.Errorf("FieldOf answers for a kind the grammar does not name")
	}
	if authority := WriteAuthorityOf(invented); authority != "" {
		t.Errorf("a kind the grammar does not name reports the write authority %q", authority)
	}
	for _, kind := range kinds {
		if kind == invented {
			t.Errorf("EntityKinds carries %s, which the grammar does not name", invented)
		}
	}
}

// TestAllFieldsIsTheSortedUnionOfEveryKind holds the vocabulary the set
// command publishes to the per-kind sets it is drawn from, in both directions.
// The union is what a tool schema declares, because a schema is fixed before a
// reference is known, so a name in it that no kind carries would offer a
// caller a value nothing accepts.
func TestAllFieldsIsTheSortedUnionOfEveryKind(t *testing.T) {
	union := AllFields()
	if len(union) == 0 {
		t.Fatal("the union carries no name, so this check read nothing")
	}
	if !sort.StringsAreSorted(union) {
		t.Errorf("the union is not sorted: %v", union)
	}
	carried := map[string]bool{}
	for _, name := range union {
		if carried[name] {
			t.Errorf("the union carries %s twice", name)
		}
		carried[name] = true
	}
	for _, kind := range EntityKinds() {
		for _, name := range FieldsOf(kind) {
			if !carried[name] {
				t.Errorf("%s carries the field %s and the union does not", kind, name)
			}
		}
	}
	for _, name := range union {
		reached := false
		for _, kind := range EntityKinds() {
			if _, known := FieldOf(kind, name); known {
				reached = true
			}
		}
		if !reached {
			t.Errorf("the union carries %s and no kind records it", name)
		}
	}
}

// TestEveryKindsAnchorIsNamed asserts that the anchor of every kind the field
// declaration reaches is one this package can name, since a write reads and
// rewrites that file and a kind with no anchor would be a field set nothing
// can store.
func TestEveryKindsAnchorIsNamed(t *testing.T) {
	kinds := EntityKinds()
	if len(kinds) == 0 {
		t.Fatal("the grammar names no kind, so this check read nothing")
	}
	for _, kind := range kinds {
		anchor := AnchorOf(kind)
		if anchor == "" {
			t.Errorf("%s carries fields and this package names no anchor for it", kind)
			continue
		}
		if !strings.HasSuffix(anchor, ".md") {
			t.Errorf("%s names the anchor %q, which is not an anchor file", kind, anchor)
		}
	}
	if got := AnchorOf("frobnicate"); got != "" {
		t.Errorf("a kind the grammar does not name reports the anchor %q", got)
	}
}
