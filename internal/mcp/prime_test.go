package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"dinah/internal/verb"
)

// TestPrimeToolListedUnderEveryProfile is dinah-573/criteria/16: prime is
// listed under tools/list when the connection opens with ProfileStation,
// ProfileOperator and ProfileAll alike.
func TestPrimeToolListedUnderEveryProfile(t *testing.T) {
	library := newLibrary(t)
	for _, profile := range []string{ProfileStation, ProfileOperator, ProfileAll} {
		answer := askUnderProfile(t, library.Bench.Root, profile, library, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		if answer.Error != nil {
			t.Fatalf("%s: tools/list failed: %+v", profile, answer.Error)
		}
		encoded, err := json.Marshal(answer.Result)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		if err := json.Unmarshal(encoded, &result); err != nil {
			t.Fatalf("decode tools/list: %v", err)
		}
		found := false
		for _, tool := range result.Tools {
			if tool.Name == "prime" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: tools/list does not carry prime", profile)
		}
	}
}

// primeInstructions reads the instructions member off a prime tool's payload.
func primeInstructions(t *testing.T, decoded map[string]any) verb.Instructions {
	t.Helper()
	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	var shape struct {
		Primer struct {
			Instructions verb.Instructions `json:"instructions"`
		} `json:"primer"`
	}
	if err := json.Unmarshal(encoded, &shape); err != nil {
		t.Fatalf("decode the primer: %v", err)
	}
	return shape.Primer.Instructions
}

// TestPrimeWithholdsOnASecondCallAndRecoversByColumn is dinah-573/criteria/12
// and dinah-573/criteria/13: a first prime call in a session serves Global
// and Standing in full and records both chain keys served; a second call
// with the actor still holding the same card withholds both, naming Reread
// as the held card's column; and an actor holding no card is never withheld
// from on any prime call, with Withheld and Reread always absent.
func TestPrimeWithholdsOnASecondCallAndRecoversByColumn(t *testing.T) {
	library := newLibrary(t)
	writeGlobal(t, library, "Global text.\n")

	// A connection whose actor holds nothing sees both calls come back in
	// full, since Prime's own narrowing of the withholding rule applies
	// whether or not a connection has sent the text before.
	empty := newChainSession(t, library, newChainMemory())
	first := primeInstructions(t, empty.call("prime", map[string]any{"actor": "alka"}))
	wantFull(t, "the first prime call for an actor holding nothing", first, "")
	second := primeInstructions(t, empty.call("prime", map[string]any{"actor": "alka"}))
	if len(second.Withheld) != 0 || second.Reread != "" {
		t.Errorf("an actor holding nothing was withheld from: %+v", second)
	}
	if !strings.Contains(second.Global, "Global text") || !strings.Contains(second.Standing, "Standing") {
		t.Errorf("an actor holding nothing was not served in full: %+v", second)
	}

	// alka claims fx-1 over a connection of its own, so the claim it makes
	// there records nothing in the session under test below.
	claiming := newChainSession(t, library, newChainMemory())
	claimed := claiming.call("claim", map[string]any{"card": "fx-1", "actor": "alka"})
	if _, ok := claimed["card"]; !ok {
		t.Fatalf("the claim did not answer a card: %+v", claimed)
	}

	// A fresh connection has sent alka nothing yet, so the first prime call
	// on it serves Global and Standing in full even though alka now holds a
	// card, and a second call at the same position withholds both, naming
	// the held card's own column as the recovery reference.
	holding := newChainSession(t, library, newChainMemory())
	third := primeInstructions(t, holding.call("prime", map[string]any{"actor": "alka"}))
	wantFull(t, "the first prime call on a fresh connection after claiming fx-1", third, "")

	fourth := primeInstructions(t, holding.call("prime", map[string]any{"actor": "alka"}))
	wantWithheld(t, "a second prime call at the same position", fourth, verb.LayerGlobal, verb.LayerStanding)
	if fourth.Reread != "doing" {
		t.Fatalf("wanted Reread to name fx-1's own column doing, got %q", fourth.Reread)
	}
}

// TestPrimeBriefForcesTheNarrowFormOnAFirstCall is dinah-573/criteria/26: on
// MCP, brief: true is accepted and honored on every call, forcing the
// narrow, pointer-only answer even on a first call in the session that
// would otherwise serve Global and Standing in full, and it does not
// record the withheld keys as served, so the next ordinary call still gets
// them in full.
func TestPrimeBriefForcesTheNarrowFormOnAFirstCall(t *testing.T) {
	library := newLibrary(t)
	writeGlobal(t, library, "Global text.\n")
	session := newChainSession(t, library, newChainMemory())

	brief := primeInstructions(t, session.call("prime", map[string]any{"actor": "alka", "brief": true}))
	if brief.Global != "" || brief.Standing != "" {
		t.Errorf("brief carried text: %+v", brief)
	}
	wantWithheld(t, "the brief call", brief, verb.LayerGlobal, verb.LayerStanding)

	ordinary := primeInstructions(t, session.call("prime", map[string]any{"actor": "alka"}))
	wantFull(t, "the ordinary call after a brief one", ordinary, "")
}
