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
