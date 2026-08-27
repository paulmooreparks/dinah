package main

import (
	"sort"
	"testing"

	"dinah/internal/verb"
)

// TestEveryLibraryCommandIsDispatchedOrExempted asserts that the terminal's
// command table and its exemption map account for the verb table exactly, in
// both directions.
//
// It is the twin of the check the mcp head runs over its own roster, written
// separately rather than shared because neither head can import the other and
// a check living in the library would have to. A fourth head arrives as a
// fourth file of this shape rather than as an edit to a list that has to
// remember a name nobody has written yet.
func TestEveryLibraryCommandIsDispatchedOrExempted(t *testing.T) {
	dispatched := map[string]bool{}
	for _, entry := range commands {
		if dispatched[entry.name] {
			t.Errorf("the command table carries %s twice", entry.name)
		}
		dispatched[entry.name] = true
	}
	declared := map[string]bool{}
	for _, name := range verb.Commands() {
		declared[name] = true
	}

	ordered := append([]string(nil), verb.Commands()...)
	sort.Strings(ordered)
	for _, name := range ordered {
		reason, exempted := commandExemptions[name]
		switch {
		case dispatched[name] && exempted:
			t.Errorf("%s is dispatched at the terminal and is also exempted (%q); one of the two is wrong", name, reason)
		case !dispatched[name] && !exempted:
			t.Errorf("the library defines %s and this head neither dispatches it nor names a reason it is absent", name)
		case exempted && reason == "":
			t.Errorf("%s is exempted with no reason, which is a gap nobody has argued for", name)
		}
	}

	for name := range dispatched {
		if !declared[name] {
			t.Errorf("the terminal dispatches %s, which the verb table does not define", name)
		}
	}
	for name := range commandExemptions {
		if !declared[name] {
			t.Errorf("%q is exempted and the verb table does not define it, so the exemption outlived its command", name)
		}
	}
	if len(declared) == 0 {
		t.Fatal("the verb table declared no commands, so this check read nothing")
	}
}
