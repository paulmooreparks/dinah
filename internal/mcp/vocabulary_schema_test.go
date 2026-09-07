package mcp

import (
	"reflect"
	"sort"
	"testing"

	"dinah/internal/verb"
)

// TestTheSchemaPublishesEachVocabularyAndDurationKeyExactlyWhereTheTableDeclaresIt
// asserts dinah-420 AC1: over every tool this head serves, "enum" appears on
// exactly the properties whose parameter declares a vocabulary fixed in the
// source and takes one member as its whole value, "x-dinah-vocabulary-members"
// on exactly the properties whose parameter declares such a vocabulary and
// takes a comma-separated list of its members, "x-dinah-vocabulary-source" on
// exactly the properties whose parameter declares a vocabulary a head resolves
// when it runs, "x-dinah-value-list" on exactly the properties whose parameter
// declares the list placeholder, and "format": "duration" on exactly the
// properties whose parameter declares the duration placeholder.
//
// The list half is the one dinah-420's code review caught. show's fields
// argument is split on commas by the parser, so publishing its vocabulary as
// an enum narrowed a machine surface rather than describing it, and this test
// now fails when a list-valued parameter carries an enum.
//
// Both directions matter and the loop below checks both. A key missing where
// the table declares one costs a client the guidance the key exists to carry;
// a key present where the table declares none tells a client a set is closed
// when it is not, which is worse, because a strict client will then refuse to
// send a value the tool would have accepted.
//
// The expectation is derived from verb.Params and verb.VocabularyFor rather
// than written out per tool, which is deliberate and is not the code agreeing
// with itself: schemaFor could publish the right keys on the wrong properties,
// publish a stale copy of a set, or drop the keys on a tool whose parameters
// are read through a different path, and each of those is a disagreement
// between the two functions this test compares.
func TestTheSchemaPublishesEachVocabularyAndDurationKeyExactlyWhereTheTableDeclaresIt(t *testing.T) {
	if len(tools) == 0 {
		t.Fatal("the surface carries no tool, so this test proves nothing")
	}
	enums, members, sources, lists, durations := 0, 0, 0, 0, 0
	for _, served := range tools {
		schema := schemaFor(served)
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s: the schema carries no properties object", served.name)
		}
		declared := map[string]bool{}
		for _, param := range verb.Params(served.command) {
			if exemptArgument(served.name, param.Name) {
				continue
			}
			declared[param.Name] = true
			property, ok := properties[param.Name].(map[string]any)
			if !ok {
				t.Errorf("%s: the schema publishes no property for the parameter %s", served.name, param.Name)
				continue
			}
			set, hasVocabulary := verb.VocabularyFor(served.command, param.Name)
			wantList := param.Value == listPlaceholder
			wantEnum := hasVocabulary && set.Source == "" && !wantList
			wantMembers := hasVocabulary && set.Source == "" && wantList
			wantSource := hasVocabulary && set.Source != ""
			gotList, carriesList := property["x-dinah-value-list"]
			if wantList != carriesList {
				t.Errorf("%s.%s: x-dinah-value-list published %v, wanted %v", served.name, param.Name, carriesList, wantList)
			} else if wantList {
				lists++
				if gotList != true {
					t.Errorf("%s.%s: x-dinah-value-list is %v, wanted true", served.name, param.Name, gotList)
				}
			}
			gotMembers, carriesMembers := property["x-dinah-vocabulary-members"]
			if wantMembers != carriesMembers {
				t.Errorf("%s.%s: x-dinah-vocabulary-members published %v, wanted %v", served.name, param.Name, carriesMembers, wantMembers)
			} else if wantMembers {
				members++
				if !reflect.DeepEqual(gotMembers, set.Values) {
					t.Errorf("%s.%s: x-dinah-vocabulary-members is %v, wanted the declared set %v", served.name, param.Name, gotMembers, set.Values)
				}
			}
			gotEnum, carriesEnum := property["enum"]
			if wantEnum != carriesEnum {
				t.Errorf("%s.%s: enum published %v, wanted %v", served.name, param.Name, carriesEnum, wantEnum)
			} else if wantEnum {
				enums++
				if !reflect.DeepEqual(gotEnum, set.Values) {
					t.Errorf("%s.%s: enum is %v, wanted the declared set %v", served.name, param.Name, gotEnum, set.Values)
				}
			}
			gotSource, carriesSource := property["x-dinah-vocabulary-source"]
			if wantSource != carriesSource {
				t.Errorf("%s.%s: x-dinah-vocabulary-source published %v, wanted %v", served.name, param.Name, carriesSource, wantSource)
			} else if wantSource {
				sources++
				if gotSource != set.Source {
					t.Errorf("%s.%s: x-dinah-vocabulary-source is %v, wanted %q", served.name, param.Name, gotSource, set.Source)
				}
			}
			wantDuration := param.Value == durationPlaceholder
			gotFormat, carriesFormat := property["format"]
			if wantDuration != carriesFormat {
				t.Errorf("%s.%s: format published %v, wanted %v", served.name, param.Name, carriesFormat, wantDuration)
			} else if wantDuration {
				durations++
				if gotFormat != "duration" {
					t.Errorf("%s.%s: format is %v, wanted \"duration\"", served.name, param.Name, gotFormat)
				}
			}
		}
		// A property the parameter table does not name is one schemaFor
		// injects, and no injected property is a parameter, so none of the
		// five keys may reach one.
		for name, raw := range properties {
			if declared[name] {
				continue
			}
			property, ok := raw.(map[string]any)
			if !ok {
				t.Errorf("%s: the injected property %s is not an object", served.name, name)
				continue
			}
			for _, key := range []string{"enum", "x-dinah-vocabulary-members", "x-dinah-vocabulary-source", "x-dinah-value-list", "format"} {
				if _, carried := property[key]; carried {
					t.Errorf("%s.%s: an injected property carries %s, which only a parameter earns", served.name, name, key)
				}
			}
		}
	}
	// Five counters rather than one, because a rule checked over a surface
	// where nothing triggers it passes for the wrong reason, and the five
	// keys are triggered by five different declarations.
	if enums == 0 {
		t.Error("no served property declares a single-valued fixed vocabulary, so the enum half of this test read nothing")
	}
	if members == 0 {
		t.Error("no served property declares a list-valued fixed vocabulary, so the members half of this test read nothing")
	}
	if sources == 0 {
		t.Error("no served property declares a resolved vocabulary, so the source half of this test read nothing")
	}
	if lists == 0 {
		t.Error("no served property declares a list, so the list half of this test read nothing")
	}
	if durations == 0 {
		t.Error("no served property declares a duration, so the format half of this test read nothing")
	}
}

// TestNoListValuedParameterPublishesAnEnum asserts dinah-420 AC1 from the side
// the code review found it on, and states the rule against the parser rather
// than against the parameter table.
//
// The test above derives its expectation from verb.Params, so it holds
// schemaFor and the table to each other and both could be wrong together.
// This one names the rule the far end enforces instead. A parameter declared
// with the list placeholder is split on commas before any member is checked,
// so no such parameter may publish an enum, and the members it does publish
// have to be the set its own parser validates against, which for show's
// fields argument is verb.DetailFields rather than a copy of it.
func TestNoListValuedParameterPublishesAnEnum(t *testing.T) {
	checked := 0
	for _, served := range tools {
		properties, ok := schemaFor(served)["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s: the schema carries no properties object", served.name)
		}
		for _, param := range verb.Params(served.command) {
			if exemptArgument(served.name, param.Name) || param.Value != listPlaceholder {
				continue
			}
			property, ok := properties[param.Name].(map[string]any)
			if !ok {
				t.Errorf("%s: the schema publishes no property for the parameter %s", served.name, param.Name)
				continue
			}
			checked++
			if _, carried := property["enum"]; carried {
				t.Errorf("%s.%s takes a comma-separated list, so an enum makes a legal answer naming two members invalid on this surface", served.name, param.Name)
			}
			published, carried := property["x-dinah-vocabulary-members"]
			if !carried {
				t.Errorf("%s.%s publishes no members, so a client is left with a bare string where a bounded set exists", served.name, param.Name)
				continue
			}
			if !reflect.DeepEqual(published, verb.DetailFields) {
				t.Errorf("%s.%s publishes the members %v, wanted the set its parser checks against, %v", served.name, param.Name, published, verb.DetailFields)
			}
		}
	}
	if checked == 0 {
		t.Error("no served property declares the list placeholder, so this test read nothing")
	}
}

// TestEveryVocabularySourceAServedToolPublishesIsTheOneColumnsSource asserts
// dinah-420 AC2: the distinct Vocabulary.Source values reachable from a served
// tool's published parameters are exactly {"columns"}.
//
// The VS Code extension's command palette resolves a source-bearing argument
// by calling a tool of its own, and it carries a table of the sources it knows
// how to resolve. That table has one entry, and this test is what stands
// between it and a source it has never heard of: a card that serves guide or
// config as a tool, or that gives a served tool a parameter declaring a new
// source, fails here and is told to extend the table in the same diff.
//
// The second half is what keeps the first from passing vacuously. dinah
// declares two resolved sources today and serves tools for the commands
// naming one of them, so the set below is narrower than verb's own set for a
// reason, and that reason is checked rather than described.
func TestEveryVocabularySourceAServedToolPublishesIsTheOneColumnsSource(t *testing.T) {
	reachable := map[string]bool{}
	for _, served := range tools {
		for _, param := range verb.Params(served.command) {
			if exemptArgument(served.name, param.Name) {
				continue
			}
			set, declared := verb.VocabularyFor(served.command, param.Name)
			if declared && set.Source != "" {
				reachable[set.Source] = true
			}
		}
	}
	got := make([]string, 0, len(reachable))
	for source := range reachable {
		got = append(got, source)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, []string{"columns"}) {
		t.Errorf("served tools reach the vocabulary sources %v, wanted exactly [columns]; a client resolving these carries a table with one entry, so extend that table in the same diff", got)
	}

	declared := verb.VocabularySources()
	if !reflect.DeepEqual(declared, []string{"columns", "guides"}) {
		t.Errorf("the library declares the vocabulary sources %v, wanted [columns guides]; the set above is narrower than this one and this test says why", declared)
	}
	if _, exempt := toolExemptions["guide"]; !exempt {
		t.Error("guide is served as a tool, so the guides source is now reachable from the surface and the set above should have caught it")
	}
}
