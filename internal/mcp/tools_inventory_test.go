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
// markers are held back from this head, so the row carries the injected
// properties every tool carries and nothing of check's own. Restoring the
// markers means deleting their rows from argumentExemptions and editing this
// row in the same commit.
//
// Every row gained harness, provider, model and server at dinah-496. They are
// injected properties rather than any command's parameters, and every tool
// consumes all four: the facts are stamped on whatever a call writes, and a
// read reports them through whoami.
var publishedProperties = map[string][]string{
	"claim":             {"actor", "basis", "card", "expires", "harness", "model", "provider", "server", "workbench"},
	"move":              {"actor", "basis", "card", "column", "harness", "model", "override", "provider", "server", "workbench"},
	"release":           {"actor", "basis", "card", "harness", "model", "provider", "server", "workbench"},
	"block":             {"actor", "basis", "card", "harness", "kind", "model", "provider", "reason", "server", "workbench"},
	"unblock":           {"actor", "basis", "card", "harness", "model", "provider", "server", "workbench"},
	"raise":             {"actor", "card", "harness", "model", "provider", "reason", "server", "tier", "workbench"},
	"join_workstream":   {"actor", "basis", "card", "harness", "model", "provider", "server", "workbench", "workstream"},
	"leave_workstream":  {"actor", "basis", "card", "harness", "model", "provider", "server", "workbench", "workstream"},
	"add_card":          {"actor", "column", "harness", "model", "priority", "provider", "route", "server", "severity", "title", "workbench"},
	"comment":           {"actor", "card", "harness", "model", "provider", "server", "text", "workbench"},
	"attach":            {"actor", "description", "file", "harness", "model", "provider", "ref", "replace", "server", "workbench"},
	"file_item":         {"actor", "card", "column", "harness", "kind", "model", "owner", "provider", "server", "text", "workbench"},
	"cite_item":         {"actor", "harness", "item", "model", "observed", "provider", "scheme", "server", "target", "workbench"},
	"resolve_item":      {"actor", "designation", "harness", "item", "model", "provider", "server", "text", "workbench"},
	"verify_item":       {"actor", "designation", "harness", "item", "model", "provider", "server", "text", "workbench"},
	"fail_item":         {"actor", "designation", "harness", "item", "model", "provider", "server", "text", "workbench"},
	"reopen_item":       {"actor", "harness", "item", "model", "provider", "reason", "server", "workbench"},
	"settle":            {"actor", "designation", "harness", "item", "model", "provider", "reason", "server", "state", "text", "workbench"},
	"link_card":         {"actor", "card", "harness", "kind", "model", "provider", "server", "to", "workbench"},
	"unlink_card":       {"actor", "card", "harness", "kind", "model", "provider", "server", "to", "workbench"},
	"archive":           {"actor", "harness", "model", "provider", "ref", "server", "workbench"},
	"restore":           {"actor", "archived", "harness", "model", "provider", "ref", "server", "workbench"},
	"delete":            {"actor", "force", "harness", "model", "provider", "ref", "server", "workbench", "yes"},
	"accept_divergence": {"actor", "comment", "harness", "model", "provider", "server", "workbench"},
	"rename":            {"actor", "harness", "model", "name", "provider", "ref", "server", "workbench"},
	"status":            {"actor", "harness", "max-depth", "model", "provider", "root", "server", "workbench"},
	"next_card":         {"actor", "column", "harness", "max-depth", "model", "provider", "root", "server", "workbench"},
	"pull":              {"actor", "basis", "column", "expires", "harness", "model", "no-claim", "override", "provider", "server", "workbench"},
	"query":             {"actor", "harness", "model", "provider", "query", "server", "workbench"},
	"search_cards":      {"actor", "archived", "harness", "max-depth", "model", "phrase", "provider", "query", "root", "server", "workbench"},
	"tree":              {"actor", "depth", "group-by", "harness", "max-depth", "model", "provider", "query", "root", "server", "workbench"},
	"show":              {"actor", "all", "archived", "card", "fields", "harness", "model", "provider", "server", "since", "unresolved", "workbench"},
	"changes":           {"actor", "card", "column", "harness", "max-depth", "model", "provider", "root", "server", "since", "workbench"},
	"list":              {"actor", "archived", "depth", "harness", "max-depth", "model", "provider", "ready", "ref", "root", "server", "since", "unresolved", "workbench"},
	"instructions":      {"actor", "card", "harness", "model", "provider", "server", "workbench"},
	"whoami":            {"actor", "harness", "model", "provider", "server", "workbench"},
	"prime":             {"actor", "brief", "full-pending", "harness", "model", "provider", "server", "workbench"},
	"workbench":         {"actor", "harness", "model", "provider", "server", "workbench"},
	"workstream":        {"action", "actor", "harness", "model", "provider", "server", "slug", "workbench", "workstream"},
	"get_field":         {"actor", "field", "harness", "model", "provider", "ref", "server", "workbench"},
	"set_field":         {"actor", "at", "expect-digest", "field", "harness", "model", "note", "provider", "ref", "server", "value", "workbench", "yes"},
	"new_column":        {"actor", "before", "capacity", "column", "harness", "kind", "model", "provider", "server", "slug", "tier", "workbench"},
	"version":           {"actor", "catalogs", "harness", "model", "provider", "server", "workbench"},
	"export":            {"actor", "harness", "model", "provider", "server", "workbench"},
	"check":             {"actor", "harness", "model", "provider", "server", "workbench"},
}

// TestThePublishedPropertyInventoryMatchesTheSurface asserts dinah-397 AC-5:
// what toolList serves is what the table above says it serves, tool for tool
// and name for name.
func TestThePublishedPropertyInventoryMatchesTheSurface(t *testing.T) {
	served := map[string][]string{}
	for _, entry := range toolList(ProfileAll) {
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
	if want := "actor, harness, model, provider, server, workbench"; strings.Join(served["check"], ", ") != want {
		t.Errorf("check publishes [%s], want [%s], which is the ruling of 2026-09-06 plus the four injected identity properties and no marker of check's own",
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

// TestOnlyTheTwoWritingCommandsPublishATierProperty asserts dinah-496's half
// of the retirement dinah-410 AC-11 used to cover. claim, next_card and pull
// took a declaration of what the caller is, and they take none now: a tier is
// resolved from the provider and the model against the workbench's own table,
// so a property saying it would be a declaration nothing reads.
//
// The two commands that still publish one are writing a requirement rather than
// declaring a claimant. raise writes what a card requires at the column it
// stands in, and new_column writes a column's own default, and naming both here
// says the survival is deliberate rather than an oversight.
func TestOnlyTheTwoWritingCommandsPublishATierProperty(t *testing.T) {
	served := map[string]bool{}
	for _, entry := range toolList(ProfileAll) {
		name, _ := entry["name"].(string)
		schema, ok := entry["inputSchema"].(map[string]any)
		if !ok {
			continue
		}
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			continue
		}
		if _, published := properties["tier"]; published {
			served[name] = true
		}
	}
	for _, name := range []string{"claim", "next_card", "pull"} {
		if served[name] {
			t.Errorf("%s publishes a tier property, and the three claiming commands take no declaration of what the caller is", name)
		}
	}
	for _, name := range []string{"raise", "new_column"} {
		if !served[name] {
			t.Errorf("%s publishes no tier property, and it writes what a card or a column requires", name)
		}
	}
	if len(served) != 2 {
		t.Errorf("%d tools publish a tier property, wanted the two that write a requirement", len(served))
	}
}
