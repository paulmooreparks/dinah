package verb

import "testing"

// TestCrossHeadIdenticalIsWellFormed asserts that every command declared to
// answer alike on every head is a command the table actually defines, and that
// each one carries the clause somebody wrote to justify it.
//
// The map is the single declared source of the cross-head payload check that
// runs at the terminal, so a key naming nothing and a reason nobody wrote are
// the two ways it could quietly cover less than it claims.
func TestCrossHeadIdenticalIsWellFormed(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range Commands() {
		declared[name] = true
	}
	if len(CrossHeadIdentical()) == 0 {
		t.Fatal("no command is declared cross-head identical, so the payload check reads nothing")
	}
	for name, reason := range CrossHeadIdentical() {
		if !declared[name] {
			t.Errorf("%q is declared cross-head identical and the table defines no such command", name)
		}
		if reason == "" {
			t.Errorf("%q is declared cross-head identical with no reason, so nobody has said why the guarantee holds", name)
		}
	}
}

// TestCrossHeadIdenticalCannotBeEditedThroughItsAccessor asserts that a caller
// writing into the returned map does not reach the declaration behind it, so
// the guard's own source cannot be rewritten by whoever reads it.
func TestCrossHeadIdenticalCannotBeEditedThroughItsAccessor(t *testing.T) {
	CrossHeadIdentical()["query"] = ""
	if CrossHeadIdentical()["query"] == "" {
		t.Error("writing into the returned map reached the declaration behind it")
	}
}
