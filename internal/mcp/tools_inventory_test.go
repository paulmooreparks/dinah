package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// publishedProperties is the whole surface written out: per tool, the exact
// property names tools/list offers a caller, in sorted order.
//
// The table is here so that a change to what a caller is offered reaches a
// reviewer as a diff rather than as silence. Generating the expectation from
// the same declaration the surface is built from would assert only that the
// code agrees with itself, and this table's job is to make somebody write down
// what changed.
//
// check's row is the one capability-reducing ruling on dinah-397, taken by the
// operator at Operator Design Review on 2026-09-06. The ten store-repair
// markers are held back from this head and the row reads exactly actor and
// workbench. Restoring them means deleting their rows from argumentExemptions
// and editing this row in the same commit.
var publishedProperties = map[string][]string{
	"claim":            {"actor", "basis", "card", "expires", "tier", "workbench"},
	"move":             {"actor", "basis", "card", "column", "override", "workbench"},
	"release":          {"actor", "basis", "card", "workbench"},
	"block":            {"actor", "basis", "card", "kind", "reason", "workbench"},
	"unblock":          {"actor", "basis", "card", "workbench"},
	"raise":            {"actor", "card", "reason", "tier", "workbench"},
	"join_workstream":  {"actor", "basis", "card", "workbench", "workstream"},
	"leave_workstream": {"actor", "basis", "card", "workbench", "workstream"},
	"add_card":         {"actor", "column", "priority", "severity", "title", "workbench"},
	"comment":          {"actor", "card", "text", "workbench"},
	"attach":           {"actor", "description", "file", "ref", "replace", "workbench"},
	"file_item":        {"actor", "card", "column", "kind", "owner", "text", "workbench"},
	"cite_item":        {"actor", "item", "observed", "scheme", "target", "workbench"},
	"resolve_item":     {"actor", "item", "note", "workbench"},
	"verify_item":      {"actor", "item", "note", "workbench"},
	"fail_item":        {"actor", "item", "note", "workbench"},
	"reopen_item":      {"actor", "item", "reason", "workbench"},
	"archive":          {"actor", "ref", "workbench"},
	"delete":           {"actor", "ref", "workbench", "yes"},
	"rename":           {"actor", "name", "ref", "workbench"},
	"status":           {"actor", "max-depth", "root", "workbench"},
	"columns":          {"actor", "workbench"},
	"list_cards":       {"actor", "column", "max-depth", "ready", "root", "workbench"},
	"next_card":        {"actor", "column", "max-depth", "root", "tier", "workbench"},
	"pull":             {"actor", "basis", "column", "expires", "no-claim", "override", "tier", "workbench"},
	"query":            {"actor", "query", "workbench"},
	"search_cards":     {"actor", "archived", "max-depth", "phrase", "query", "root", "workbench"},
	"tree":             {"actor", "depth", "group-by", "max-depth", "query", "root", "workbench"},
	"contents":         {"actor", "depth", "ref", "workbench"},
	"attachments":      {"actor", "ref", "workbench"},
	"show":             {"actor", "card", "fields", "workbench"},
	"log":              {"actor", "card", "workbench"},
	"changes":          {"actor", "card", "column", "max-depth", "root", "since", "workbench"},
	"instructions":     {"actor", "card", "workbench"},
	"whoami":           {"actor", "workbench"},
	"card":             {"action", "actor", "at", "card", "field", "value", "workbench"},
	"workbench":        {"action", "actor", "field", "value", "workbench", "yes"},
	"workstream":       {"action", "actor", "field", "slug", "value", "workbench", "workstream", "yes"},
	"new_column":       {"actor", "before", "capacity", "column", "kind", "slug", "tier", "workbench"},
	"version":          {"actor", "catalogs", "workbench"},
	"export":           {"actor", "workbench"},
	"check":            {"actor", "workbench"},
	"workbenches":      {"actor", "max-depth", "path"},
}

// TestThePublishedPropertyInventoryMatchesTheSurface asserts dinah-397 AC-5:
// what toolList serves is what the table above says it serves, tool for tool
// and name for name.
func TestThePublishedPropertyInventoryMatchesTheSurface(t *testing.T) {
	served := map[string][]string{}
	for _, entry := range toolList() {
		name, _ := entry["name"].(string)
		schema, ok := entry["inputSchema"].(map[string]any)
		if !ok {
			t.Fatalf("%s carries no input schema", name)
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s carries no input schema properties", name)
		}
		names := make([]string, 0, len(properties))
		for property := range properties {
			names = append(names, property)
		}
		sort.Strings(names)
		served[name] = names
	}
	for name, want := range publishedProperties {
		got, published := served[name]
		if !published {
			t.Errorf("the inventory names %s, which this head no longer serves", name)
			continue
		}
		if strings.Join(got, ", ") != strings.Join(want, ", ") {
			t.Errorf("%s publishes [%s], and the inventory says [%s]",
				name, strings.Join(got, ", "), strings.Join(want, ", "))
		}
	}
	for name := range served {
		if _, inventoried := publishedProperties[name]; !inventoried {
			t.Errorf("the surface serves %s, which the inventory does not name", name)
		}
	}
	// The one row the operator ruled on, asserted by itself so that a reversal
	// cannot ride in under a bulk edit of the table.
	if want := "actor, workbench"; strings.Join(served["check"], ", ") != want {
		t.Errorf("check publishes [%s], want [%s], which is the ruling of 2026-09-06",
			strings.Join(served["check"], ", "), want)
	}
}

// basisConsumers names the eight tools whose verb reads the request's basis,
// and basisProbeArguments carries what each of them needs to reach that read.
//
// The guard sits inside Library.Do and inside the pull transaction, ahead of
// evaluate in both, so a call reaching the guard answers stale whatever the
// card's own state is and whatever the rest of the arguments say.
var basisProbeArguments = map[string]map[string]any{
	"claim":            {"card": "fx-1"},
	"move":             {"card": "fx-1", "column": "doing"},
	"release":          {"card": "fx-1"},
	"block":            {"card": "fx-1", "kind": "external", "reason": "a probe"},
	"unblock":          {"card": "fx-1"},
	"join_workstream":  {"card": "fx-1", "workstream": "portfolio"},
	"leave_workstream": {"card": "fx-1", "workstream": "portfolio"},
	"pull":             {"column": "doing"},
}

// impossibleBasis is a value no card's revision can equal. A revision is a
// sha256 over the card's content, so a string carrying neither the length nor
// the alphabet of one cannot collide with a real card by accident.
const impossibleBasis = "not-a-revision"

// TestBasisIsPublishedExactlyWhereItIsConsumed asserts dinah-397 AC-4: an
// injected property reaches exactly the tools that read it.
//
// The probe fails in both directions, which is what makes it worth running. A
// tool that publishes basis and ignores it answers ok rather than stale and
// fails here; a tool that honours basis without publishing it is refused by
// checkArguments and fails here too.
func TestBasisIsPublishedExactlyWhereItIsConsumed(t *testing.T) {
	id := 0
	for _, entry := range tools {
		id++
		arguments, consumes := basisProbeArguments[entry.name]
		call := map[string]any{"actor": "alka", "basis": impossibleBasis}
		for name, value := range arguments {
			call[name] = value
		}
		// Each tool gets its own workbench, because the consuming tools that
		// reach past the guard on a later run would otherwise meet a card the
		// probe before them had moved.
		library := newLibrary(t)
		answer := ask(t, library, callLine(t, id, entry.name, call))
		if !consumes {
			if answer.Error == nil {
				t.Errorf("%s accepted a basis it never reads: %+v", entry.name, answer.Result)
				continue
			}
			message := answer.Error.Message
			if !strings.Contains(message, "basis") {
				t.Errorf("%s refused the call without naming basis: %q", entry.name, message)
			}
			if !strings.Contains(message, fmt.Sprintf("%q", entry.name)) {
				t.Errorf("%s refused the call without naming the tool: %q", entry.name, message)
			}
			continue
		}
		if answer.Error != nil {
			t.Errorf("%s refused a basis its verb reads: %+v", entry.name, answer.Error)
			continue
		}
		outcome, _ := payload(t, answer)["outcome"].(string)
		if outcome != "stale" {
			t.Errorf("%s answered %q on an impossible basis, want stale", entry.name, outcome)
		}
	}
}

// TestTheVerbSelectionFixtureNamesEveryPublishedTool asserts dinah-397 AC-7: a
// tool added to the surface without a scenario fails the build rather than
// going unexercised by the discoverability check.
//
// The check itself needs a credential and a live model, so it does not run in
// CI. This guard needs neither, and it is what keeps the fixture from decaying
// into a record of whatever the surface looked like on the day it was written.
func TestTheVerbSelectionFixtureNamesEveryPublishedTool(t *testing.T) {
	path := filepath.Join("..", "..", "scripts", "verb_selection_fixture.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the verb-selection fixture could not be read: %v", err)
	}
	var fixture struct {
		System    string `json:"system"`
		Scenarios []struct {
			Tool      string `json:"tool"`
			Statement string `json:"statement"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("the verb-selection fixture does not parse: %v", err)
	}
	if strings.TrimSpace(fixture.System) == "" {
		t.Error("the fixture carries no system prompt, and the check pins one")
	}
	covered := map[string]int{}
	for _, scenario := range fixture.Scenarios {
		if strings.TrimSpace(scenario.Statement) == "" {
			t.Errorf("the scenario for %s carries no task statement", scenario.Tool)
		}
		covered[scenario.Tool]++
	}
	for _, entry := range tools {
		if covered[entry.name] == 0 {
			t.Errorf("the surface publishes %s and the fixture names no scenario for it",
				entry.name)
		}
	}
	for name := range covered {
		if _, served := toolsByName[name]; !served {
			t.Errorf("the fixture names %s, which this head serves no tool for", name)
		}
	}
}

// tierTakingTools are the three tools that take a caller's own tier
// declaration. claim gates on it (dinah-408) and next_card and pull select on
// it (dinah-410), and all three read it from one parameter table, so a caller
// that has learned to declare a tier on one of them has learned it on all
// three.
var tierTakingTools = []string{"claim", "next_card", "pull"}

// TestTheTierParameterIsOneShapeAcrossEveryToolThatTakesIt asserts dinah-410
// AC-11: the tier property the schema publishes for next_card and for pull is
// claim's property, byte for byte, rather than a second way of saying the
// same thing.
//
// The comparison is over the property's whole serialized JSON rather than
// over its type alone, so a description that drifts, a default that appears
// on one tool, or an enumeration added to one schema fails here. A reader of
// this failure gets the two JSON objects and can see which field moved.
func TestTheTierParameterIsOneShapeAcrossEveryToolThatTakesIt(t *testing.T) {
	served := map[string]string{}
	for _, entry := range toolList() {
		name, _ := entry["name"].(string)
		schema, ok := entry["inputSchema"].(map[string]any)
		if !ok {
			continue
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			continue
		}
		property, published := properties["tier"]
		if !published {
			continue
		}
		encoded, err := json.Marshal(property)
		if err != nil {
			t.Fatalf("%s: encode the tier property: %v", name, err)
		}
		served[name] = string(encoded)
	}
	for _, name := range tierTakingTools {
		if _, published := served[name]; !published {
			t.Errorf("%s publishes no tier property, so a caller cannot declare one", name)
		}
	}
	if t.Failed() {
		return
	}
	want := served["claim"]
	for _, name := range tierTakingTools {
		if served[name] != want {
			t.Errorf("%s publishes tier as %s, and claim publishes it as %s", name, served[name], want)
		}
	}
	// new_column also carries a tier parameter, and it is a different act:
	// the column's own default rather than a declaration about the caller.
	// Naming it here says the exclusion is deliberate rather than an
	// oversight, and a reader comparing the two shapes is looking at the
	// wrong pair.
	if _, published := served["new_column"]; !published {
		t.Errorf("new_column publishes no tier property, so this exclusion is describing something that is not there")
	}
}
