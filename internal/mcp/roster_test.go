package mcp

import (
	"sort"
	"testing"

	"dinah/internal/verb"
)

// TestEveryLibraryCommandIsServedOrExempted asserts that this head's roster
// and its exemption map together account for the verb table exactly, in both
// directions.
//
// The first direction is the ordinary drift: a command lands in the library,
// one head grows a way to reach it, and the others never do. The second
// direction is the drift this card was raised over, where a head publishes a
// capability by reaching past the verb table into the library, so the tool
// exists and no command stands behind it. Reading the roster off tools and the
// table off verb.Commands, both live, is what keeps this from becoming a third
// hand-written list that rots alongside the two it watches.
func TestEveryLibraryCommandIsServedOrExempted(t *testing.T) {
	served := map[string]string{}
	for _, entry := range tools {
		if earlier, repeated := served[entry.command]; repeated {
			t.Errorf("the command %s is served by both %s and %s", entry.command, earlier, entry.name)
		}
		served[entry.command] = entry.name
	}
	declared := map[string]bool{}
	for _, name := range verb.Commands() {
		declared[name] = true
	}

	for _, name := range sorted(verb.Commands()) {
		_, dispatched := served[name]
		reason, exempted := toolExemptions[name]
		switch {
		case dispatched && exempted:
			t.Errorf("%s is served as the tool %s and is also exempted (%q); one of the two is wrong", name, served[name], reason)
		case !dispatched && !exempted:
			t.Errorf("the library defines %s and this head neither serves it nor names a reason it is absent", name)
		case exempted && reason == "":
			t.Errorf("%s is exempted with no reason, which is a gap nobody has argued for", name)
		}
	}

	for command, name := range served {
		if !declared[command] {
			t.Errorf("the tool %s dispatches %q, which the verb table does not define", name, command)
		}
	}
	for command := range toolExemptions {
		if !declared[command] {
			t.Errorf("%q is exempted and the verb table does not define it, so the exemption outlived its command", command)
		}
	}
	if len(declared) == 0 {
		t.Fatal("the verb table declared no commands, so this check read nothing")
	}
}

// TestEveryArgumentExemptionNamesSomethingThatExists asserts that
// argumentExemptions names a tool this head publishes and, under it,
// parameters that tool's command really declares.
//
// It is the sibling of the last two checks in the roster test above, which
// refuse a toolExemptions key the verb table no longer defines, and it exists
// for the same reason one rung down: an exemption is paperwork excusing a gap,
// and paperwork that outlives what it excused reads as an argued decision
// while standing for nothing. A parameter renamed in the verb table leaves the
// entry beneath it naming a parameter nobody declares, and the tool then
// publishes the new name with no exemption in front of it, silently, since
// every other check here reads the exemption only when a live parameter
// matches it.
//
// Nothing else catches either shape. TestTheRootScopedRosterIsTheParamsTableRootCarryingSet
// asks whether an exemption that fires really holds its argument back, and the
// schema check in mcp_test.go asks the same of the published schema; both are
// silent about an exemption that never fires at all. A dead tool key and a
// dead parameter key were both planted here and both suites stayed green.
func TestEveryArgumentExemptionNamesSomethingThatExists(t *testing.T) {
	if len(argumentExemptions) == 0 {
		t.Fatal("no argument exemption is declared, so this check read nothing")
	}
	for _, name := range sortedExemptedTools() {
		held := argumentExemptions[name]
		entry, served := toolsByName[name]
		if !served {
			t.Errorf("%q holds arguments back in argumentExemptions and this head publishes no tool by that name, so the exemption outlived its tool", name)
			continue
		}
		if len(held) == 0 {
			t.Errorf("%s carries an argument exemption naming no parameter, which is a gap nobody has argued for", name)
		}
		declared := map[string]bool{}
		for _, param := range verb.Params(entry.command) {
			declared[param.Name] = true
		}
		for _, param := range sortedNames(held) {
			if !declared[param] {
				t.Errorf("%s holds %q back and its command %s declares no such parameter, so the exemption outlived its argument", name, param, entry.command)
			}
			if held[param] == "" {
				t.Errorf("%s holds %s back with no reason, which is a gap nobody has argued for", name, param)
			}
		}
	}
}

// sortedExemptedTools returns the tool names argumentExemptions carries, in
// order, so a failing run reports the same tool first every time.
func sortedExemptedTools() []string {
	names := make([]string, 0, len(argumentExemptions))
	for name := range argumentExemptions {
		names = append(names, name)
	}
	return sorted(names)
}

// sortedNames returns a reason map's keys in order, for the same reason.
func sortedNames(reasons map[string]string) []string {
	names := make([]string, 0, len(reasons))
	for name := range reasons {
		names = append(names, name)
	}
	return sorted(names)
}

// TestTheAffordanceTranslationAgreesWithTheRoster asserts that the map which
// rewrites a library affordance into a tool name says what the roster says.
//
// commandTool carries only the commands whose two vocabularies differ, so it
// is a short hand-written list standing beside the generated one, and a tool
// renamed in tools would leave it pointing at a name this head no longer
// serves. That is the drift this card exists to catch, one layer in from the
// roster itself.
func TestTheAffordanceTranslationAgreesWithTheRoster(t *testing.T) {
	if len(commandTool) == 0 {
		t.Fatal("the affordance translation is empty, so this check read nothing")
	}
	for command, name := range commandTool {
		served := ToolNameFor(command)
		if served == "" {
			t.Errorf("the affordance translation rewrites %s and this head serves no tool for it", command)
			continue
		}
		if served != name {
			t.Errorf("the affordance translation rewrites %s to %s and the roster serves it as %s", command, name, served)
		}
	}
}

// sorted returns a copy of names in order, so a failing run reports the same
// command first every time rather than in whatever order the map ranged.
func sorted(names []string) []string {
	ordered := append([]string(nil), names...)
	sort.Strings(ordered)
	return ordered
}
