package mcp

import (
	"testing"

	"dinah/internal/verb"
)

// TestShowsFiltersArePublishedOnShowAndNowhereElse walks the generated schema
// rather than the parameter table, because the table is what the walk is
// checking: a filter declared on show and published on a second tool would
// let an agent send it where nothing reads it.
//
// The since argument is the one that needs the walk. The word is spelled the
// same on show and on changes and the two do not mean the same thing, so the
// pair is asserted by name, by the value each publishes, and by the two
// sentences differing. A single shared sentence would tell an agent that a
// cursor and an ordinal are the same argument.
func TestShowsFiltersArePublishedOnShowAndNowhereElse(t *testing.T) {
	carrying := map[string][]string{"since": nil, "unresolved": nil}
	described := map[string]map[string]string{"since": {}, "unresolved": {}}
	walked := 0
	for _, served := range tools {
		properties, ok := schemaFor(served)["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s: the schema carries no properties object", served.name)
		}
		walked++
		for name := range carrying {
			property, published := properties[name].(map[string]any)
			if !published {
				continue
			}
			carrying[name] = append(carrying[name], served.command)
			sentence, _ := property["description"].(string)
			if sentence == "" {
				t.Errorf("%s publishes %s with no sentence beside it", served.name, name)
			}
			described[name][served.command] = sentence
		}
	}
	if walked == 0 {
		t.Fatal("no tool was walked, so this guard read nothing")
	}
	t.Logf("%d tools walked", walked)

	if got := carrying["unresolved"]; len(got) != 1 || got[0] != "show" {
		t.Errorf("unresolved is published on %v, wanted show alone", got)
	}
	if got := carrying["since"]; len(got) != 2 {
		t.Fatalf("since is published on %v, wanted show and changes", got)
	}
	on := map[string]bool{}
	for _, command := range carrying["since"] {
		on[command] = true
	}
	if !on["show"] || !on["changes"] {
		t.Errorf("since is published on %v, wanted show and changes", carrying["since"])
	}
	if described["since"]["show"] == described["since"]["changes"] {
		t.Errorf("both spellings of since carry one sentence, %q, and the two arguments differ",
			described["since"]["show"])
	}

	// The value name is the other half of the difference, and the terminal
	// is where it is published, so it is read off the declaration both heads
	// project rather than off the schema.
	for _, row := range []struct{ command, want string }{
		{command: "show", want: "ordinal"},
		{command: "changes", want: "cursor"},
	} {
		found := false
		for _, param := range verb.Params(row.command) {
			if param.Name != "since" {
				continue
			}
			found = true
			if param.Value != row.want {
				t.Errorf("%s publishes the value name %q for since, wanted %q", row.command, param.Value, row.want)
			}
			if param.Shared != "" {
				t.Errorf("%s declares since as shared with %q, and the two meanings differ", row.command, param.Shared)
			}
		}
		if !found {
			t.Errorf("%s declares no since argument", row.command)
		}
	}
}
