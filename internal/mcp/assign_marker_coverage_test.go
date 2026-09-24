package mcp

import (
	"reflect"
	"sort"
	"testing"

	"dinah/internal/verb"
)

// TestAssignMarkerCoversEveryPublishedMarker asserts that every Marker
// parameter this head's own tool table publishes reaches its own field of
// the request through assignMarker, catching the shape dinah-543's own
// "all" flag almost shipped in: a marker added to a published tool's schema
// and never wired into dispatch, which assignMarker's switch drops silently
// rather than refusing (mcp.go:1000-1043, no default branch).
//
// The walk is table-driven over the live parameter declarations rather than
// a fixed list of names, per the specification's own reasoning: it does not
// need editing when a new marker is declared correctly, only when one is
// declared and forgotten in assignMarker.
//
// A marker name is swept once even where several tools publish it (archived
// on show, list and search, for instance), because assignMarker dispatches
// on the name alone and one case answers for all of them. The count is
// asserted so a sweep that walked zero tools, or a change to the tool table
// that silently dropped a marker from what is published, fails here rather
// than passing vacuously: twenty markers were already wired before this
// card, and all is the twenty-first. Twenty-two as of dinah-546: wait is
// wired into assignMarker for the same defense-in-depth reason the
// pre-existing migrate-* markers already are, even though it is held back
// from the changes tool's own published schema.
func TestAssignMarkerCoversEveryPublishedMarker(t *testing.T) {
	type marker struct {
		name  string
		field string
	}
	byName := map[string]marker{}
	walked := 0
	for _, served := range tools {
		walked++
		for _, param := range verb.Params(served.command) {
			if !param.Marker || param.Field == "" {
				continue
			}
			if existing, seen := byName[param.Name]; seen && existing.field != param.Field {
				t.Fatalf("%s's marker %q names the field %q, and another tool already named %q for the same marker",
					served.name, param.Name, param.Field, existing.field)
			}
			byName[param.Name] = marker{name: param.Name, field: param.Field}
		}
	}
	if walked == 0 {
		t.Fatal("no tool was walked, so this guard read nothing")
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	t.Logf("%d tools walked, %d distinct markers swept: %v", walked, len(names), names)
	if len(names) != 28 {
		t.Fatalf("swept %d markers, wanted 28 (twenty-four already wired, plus migrate-designations, rehearse and force-claims from dinah-472, plus migrate-applies-when from dinah-590)", len(names))
	}

	// Each marker is exercised for real, through assignMarker itself, rather
	// than by reading its source: the field the parameter names is read back
	// by reflection after the call, so a case that is missing, that writes
	// the wrong field, or that assignMarker's fallthrough silently drops (the
	// defect this card's own "all" case closed) is caught the same way a
	// dropped case would be caught tomorrow.
	for _, name := range names {
		m := byName[name]
		t.Run(name, func(t *testing.T) {
			req := &verb.Request{}
			assignMarker(req, m.name, true)
			value := reflect.ValueOf(req).Elem().FieldByName(m.field)
			if !value.IsValid() {
				t.Fatalf("the marker %q names the field %q, and Request declares no such field", m.name, m.field)
			}
			if value.Kind() != reflect.Bool {
				t.Fatalf("the marker %q names the field %q, which is not a bool", m.name, m.field)
			}
			if !value.Bool() {
				t.Errorf("assignMarker(%q, true) left %s false, so this marker reaches no case", m.name, m.field)
			}
		})
	}
}
