package mcp

import (
	"testing"

	"dinah/internal/verb"
)

// TestTheSchemaMarksExactlyThePropertiesThisHeadInjects asserts dinah-420
// AC15: over every tool this head serves, "x-dinah-injected": true appears on
// exactly the properties schemaFor supplies itself, and on no property that
// comes off the parameter table.
//
// The key exists so a client can hold back the plumbing without naming actor,
// basis and workbench in its own source. A client that draws the line from a
// name list of its own goes stale the day a fourth injected property lands
// here, which is the failure the key removes and the reason both directions
// below are checked. A marked parameter is the worse of the two: a client
// obeying the mark would then stop prompting for an argument its caller has to
// supply, and the call would be refused for a missing value nobody was asked
// for.
//
// The expectation comes from injectedProperties and its consumer sets rather
// than from a list written out per tool, so a property added to that table, or
// a consumer set narrowed, is checked rather than described.
func TestTheSchemaMarksExactlyThePropertiesThisHeadInjects(t *testing.T) {
	if len(tools) == 0 {
		t.Fatal("the surface carries no tool, so this test proves nothing")
	}
	marked, unmarked := 0, 0
	for _, served := range tools {
		schema := schemaFor(served)
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s: the schema carries no properties object", served.name)
		}
		want := map[string]bool{}
		declared := declaredArgNames(served)
		for _, injected := range injectedProperties {
			if declared[injected.name] {
				want[injected.name] = true
			}
		}
		for _, param := range verb.Params(served.command) {
			if exemptArgument(served.name, param.Name) {
				continue
			}
			if want[param.Name] {
				t.Errorf("%s.%s: a parameter and an injected property answer to one name, so this test cannot tell them apart", served.name, param.Name)
			}
		}
		for name, raw := range properties {
			property, ok := raw.(map[string]any)
			if !ok {
				t.Errorf("%s: the property %s is not an object", served.name, name)
				continue
			}
			got, carried := property["x-dinah-injected"]
			if carried != want[name] {
				t.Errorf("%s.%s: x-dinah-injected published %v, wanted %v", served.name, name, carried, want[name])
				continue
			}
			if !carried {
				unmarked++
				continue
			}
			marked++
			if got != true {
				t.Errorf("%s.%s: x-dinah-injected is %v, wanted true", served.name, name, got)
			}
		}
	}
	// Two counters, because a rule checked over a surface where nothing
	// triggers it passes for the wrong reason. A head that marked nothing and
	// a head that marked everything each satisfy one half of the loop above
	// while reading nothing, and these two refuse both.
	if marked == 0 {
		t.Error("no served property carries x-dinah-injected, so the marking half of this test read nothing")
	}
	if unmarked == 0 {
		t.Error("every served property carries x-dinah-injected, so the parameter half of this test read nothing")
	}
}
