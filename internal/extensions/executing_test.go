package extensions

import (
	"testing"

	extension "github.com/paulmooreparks/dinah-extension-go"
)

// A read in progress does not hold Dinah's exit; an action that changes
// something, or declares no effect, does.
func TestExecutingLeavesOutReads(t *testing.T) {
	m := &Manager{scopes: map[string]scope{"watch": {effect: extension.EffectRead}}}
	if n := m.Executing(); n != 0 {
		t.Fatalf("a read counted as executing: %d", n)
	}
	m.scopes["stage"] = scope{effect: extension.EffectLocal}
	m.scopes["old"] = scope{}
	if n := m.Executing(); n != 2 {
		t.Fatalf("executing = %d, want 2", n)
	}
}
