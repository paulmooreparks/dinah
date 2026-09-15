package mcp

import (
	"sort"
	"strings"
	"testing"

	"dinah/internal/verb"
)

// TestEveryPublishedAffordanceNamesAServedTool asserts that every name in
// every affordance list this surface publishes is a tool this surface serves.
//
// The mcp guide promises that following the affordances cannot dead-end, and
// nothing held the surface to it. The next_card answer named log among the
// four things a caller may do next, log retired on dinah-523, and no test
// would have said so: the affordance lists are written out at their raise
// sites and the tool table is written somewhere else.
//
// Both sides are read off the same tools table tools/list is built from, so a
// rename moves them together or fails here.
func TestEveryPublishedAffordanceNamesAServedTool(t *testing.T) {
	served := map[string]bool{}
	for _, entry := range tools {
		served[entry.name] = true
	}
	if len(served) == 0 {
		t.Fatal("this head serves no tool, so the comparison below reads nothing")
	}
	library := newLibrary(t)
	published := map[string][]string{
		"readAffordances":          readAffordances,
		"the next_card answer":     affordancesOf(t, library, "next_card", map[string]any{"actor": "alka"}),
		"the show answer":          affordancesOf(t, library, "show", map[string]any{"actor": "alka", "card": "fx-1"}),
		"the list answer":          affordancesOf(t, library, "list", map[string]any{"actor": "alka", "ref": "columns"}),
		"a refusal from a tool":    affordancesOf(t, library, "show", map[string]any{"actor": "alka", "card": "fx-9999"}),
		"the workbench tool":       affordancesOf(t, library, "workbench", map[string]any{"actor": "alka"}),
		"the instructions tool":    affordancesOf(t, library, "instructions", map[string]any{"actor": "alka", "card": "fx-1"}),
		"the status tool":          affordancesOf(t, library, "status", map[string]any{"actor": "alka"}),
		"the claim tool's refusal": affordancesOf(t, library, "claim", map[string]any{"actor": "alka", "card": "fx-9999"}),
	}
	read := 0
	for where, names := range published {
		if len(names) == 0 {
			t.Errorf("%s publishes no affordance at all, so it asserts nothing", where)
			continue
		}
		for _, name := range names {
			read++
			if !served[name] {
				t.Errorf("%s names the affordance %q, and this head serves no tool by that name; it serves %s",
					where, name, strings.Join(servedToolNames(served), ", "))
			}
		}
	}
	if read == 0 {
		t.Fatal("no affordance was read, so this guard is asserting nothing")
	}
	t.Logf("%d affordance names read across %d published lists", read, len(published))
}

// affordancesOf runs one tool call and reads the affordances member off the
// payload, which is where every answer of this surface carries it.
func affordancesOf(t *testing.T, library *verb.Library, tool string, arguments map[string]any) []string {
	t.Helper()
	payload := payload(t, ask(t, library, callLine(t, 1, tool, arguments)))
	listed, carried := payload["affordances"].([]any)
	if !carried {
		t.Fatalf("the %s answer carries no affordances member: %v", tool, payload)
	}
	names := make([]string, 0, len(listed))
	for _, one := range listed {
		name, _ := one.(string)
		names = append(names, name)
	}
	return names
}

// servedToolNames spells a served set for a failure message.
func servedToolNames(served map[string]bool) []string {
	names := make([]string, 0, len(served))
	for name := range served {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
