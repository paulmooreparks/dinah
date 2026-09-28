package bench

import (
	"reflect"
	"testing"
)

// TestEntityClonesShareNothingMutable is part of dinah-619/criteria/12. Each
// entity type is filled by reflection, so every slice, map and pointer it
// carries is non-empty, and cloned. Every slice element, map entry and
// pointed-to value of the clone is then overwritten, and the original must
// still equal a copy taken before the clone. A field added later that is a
// slice, a map or a pointer is filled and overwritten by the same walk, so it
// fails here until Clone covers it.
//
// A header is overwritten whole rather than element by element: its keys and
// block are copy-on-write, shared on purpose until a writer calls own, and
// TestACloneIsTheCallersOwn holds that half.
//
// Arming: removing the Links copy from (*Card).Clone reddens the card case.
func TestEntityClonesShareNothingMutable(t *testing.T) {
	cases := []struct {
		name  string
		make  func() any
		clone func(any) any
	}{
		{"Card", func() any { return &Card{} }, func(v any) any { return v.(*Card).Clone() }},
		{"Item", func() any { return &Item{} }, func(v any) any { return v.(*Item).Clone() }},
		{"Comment", func() any { return &Comment{} }, func(v any) any { return v.(*Comment).Clone() }},
		{"Attachment", func() any { return &Attachment{} }, func(v any) any { return v.(*Attachment).Clone() }},
		{"events", func() any { events := make([]Event, 0); return &events }, func(v any) any { events := cloneEvents(*v.(*[]Event)); return &events }},
	}
	overwritten := 0
	for _, c := range cases {
		original := c.make()
		fillAll(reflect.ValueOf(original).Elem(), 0)
		before := c.make()
		fillAll(reflect.ValueOf(before).Elem(), 0)
		if !reflect.DeepEqual(original, before) {
			t.Fatalf("%s: two fills of the same type differ, so the comparison below proves nothing", c.name)
		}
		clone := c.clone(original)
		n := overwriteAll(reflect.ValueOf(clone).Elem())
		if mutable := mutableMembers(reflect.TypeOf(original).Elem()); (mutable > 0) != (n > 0) {
			t.Errorf("%s: the type carries %d slices, maps and pointers and the walk overwrote %d members of the clone", c.name, mutable, n)
		}
		overwritten += n
		if !reflect.DeepEqual(original, before) {
			t.Errorf("%s: overwriting the clone changed the original, so the clone shares something mutable with it", c.name)
		}
	}
	if len(cases) != 5 {
		t.Fatalf("ran %d types, wanted the five dinah-619 section 12.1 names", len(cases))
	}
	t.Logf("overwrote %d slice elements, map entries and pointed-to values across %d types", overwritten, len(cases))
}

// fillAll gives every exported field of v a value that is not its zero: a
// string, a number, true, a slice or a map of two filled members, and a
// pointer to a filled value. A header is parsed from a fixed text.
func fillAll(v reflect.Value, depth int) {
	if depth > 6 {
		return
	}
	if v.Type() == reflect.TypeOf(Frontmatter{}) {
		fm, _ := ParseAnchor("---\na: 1\nb: 2\n---\n")
		// Marked shared as a memoised header is, so Clone has no reason to
		// write the original's mark and the comparison sees only sharing.
		fm.markShared()
		v.Set(reflect.ValueOf(*fm))
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString("filled")
	case reflect.Int, reflect.Int64, reflect.Int32:
		v.SetInt(7)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fillAll(v.Field(i), depth+1)
			}
		}
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 2, 2)
		for i := 0; i < 2; i++ {
			fillAll(s.Index(i), depth+1)
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		for i := 0; i < 2; i++ {
			key := reflect.New(v.Type().Key()).Elem()
			fillAll(key, depth+1)
			if key.Kind() == reflect.String {
				key.SetString(key.String() + string(rune('a'+i)))
			}
			value := reflect.New(v.Type().Elem()).Elem()
			fillAll(value, depth+1)
			m.SetMapIndex(key, value)
		}
		v.Set(m)
	case reflect.Pointer:
		p := reflect.New(v.Type().Elem())
		fillAll(p.Elem(), depth+1)
		v.Set(p)
	}
}

// overwriteAll overwrites every slice element, map entry and pointed-to value
// reachable from v's exported fields, and answers how many it overwrote. A
// slice element and a pointed-to value are walked before they are
// overwritten, so a slice inside an element (an event's Cards) is written
// in place first.
func overwriteAll(v reflect.Value) int {
	n := 0
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				n += overwriteAll(v.Field(i))
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			n += overwriteAll(v.Index(i))
			v.Index(i).Set(reflect.Zero(v.Type().Elem()))
			n++
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			v.SetMapIndex(key, reflect.Zero(v.Type().Elem()))
			n++
		}
	case reflect.Pointer:
		if !v.IsNil() {
			n += overwriteAll(v.Elem())
			v.Elem().Set(reflect.Zero(v.Type().Elem()))
			n++
		}
	}
	return n
}

// mutableMembers counts the exported slice, map and pointer members a type
// carries, itself included when it is one.
func mutableMembers(typ reflect.Type) int {
	switch typ.Kind() {
	case reflect.Slice, reflect.Map, reflect.Pointer:
		return 1
	case reflect.Struct:
		n := 0
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).IsExported() {
				n += mutableMembers(typ.Field(i).Type)
			}
		}
		return n
	}
	return 0
}
