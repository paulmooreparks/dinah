package bench

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// waitingDefinition is a two-state workbench whose second station waits on
// somebody outside it, which is the smallest fixture that can show the member
// written on one state and left off the other.
const waitingDefinition = `{
  "profile": "dinah-core/1.0",
  "title": "Waiting",
  "states": [
    { "id": "d00000000001", "title": "Doing", "kind": "work",
      "instructions": "Doing instructions.\n" },
    { "id": "d00000000002", "title": "Customer approval", "kind": "work",
      "awaiting_outside": true, "instructions": "Approval instructions.\n" }
  ]
}`

// TestTheWaitingFlagIsParsedStrictly is dinah-201 AC-6 and D-8. The value is
// exactly true or false, following wip_limit rather than operator_owned, whose
// == "true" test reads yes as false and tells nobody.
func TestTheWaitingFlagIsParsedStrictly(t *testing.T) {
	t.Run("a value outside the two refuses the workbench", func(t *testing.T) {
		for _, value := range []string{"yes", "1", "True", "no", "maybe"} {
			root := newFixture(t)
			write(t, filepath.Join(root, StatesDir, "b00000000001", StateAnchor),
				"---\ntitle: Only\nslug: only\nkind: work\nawaiting_outside: "+value+"\n---\nState text.\n")
			_, err := Open(root)
			if err == nil {
				t.Fatalf("awaiting_outside: %s opened the workbench", value)
			}
			var refusal *contract.Refusal
			if !errors.As(err, &refusal) || refusal.Name != contract.Malformed {
				t.Fatalf("awaiting_outside: %s: wanted %s, got %v", value, contract.Malformed, err)
			}
			if !strings.Contains(refusal.Detail, "b00000000001") {
				t.Errorf("the refusal should name the state, got %q", refusal.Detail)
			}
		}
	})

	t.Run("false reads as a state that never carried the key", func(t *testing.T) {
		root := newFixture(t)
		write(t, filepath.Join(root, StatesDir, "b00000000001", StateAnchor),
			"---\ntitle: Only\nslug: only\nkind: work\nawaiting_outside: false\n---\nState text.\n")
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("awaiting_outside: false refused the workbench: %v", err)
		}
		if opened.States[0].AwaitingOutside {
			t.Error("awaiting_outside: false read as true")
		}
	})

	t.Run("a workbench with no such key opens unchanged", func(t *testing.T) {
		// newFixture writes the state anchor the rest of this package's
		// tests read, which carries no such key, and the committed
		// fixtures under testdata/compat are the same promise for every
		// workbench already on disk.
		root := newFixture(t)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if opened.States[0].AwaitingOutside {
			t.Error("a state that declares nothing read as waiting")
		}
		if opened.States[0].FM.Value("awaiting_outside") != "" {
			t.Error("opening a workbench invented the key")
		}
	})
}

// TestTheWaitingFlagRidesTheInterchange is dinah-201 AC-7. The round trip is
// what catches a field added to the parser and forgotten in exportState or in
// knownStateKeys, which is the half of the interchange nobody notices until a
// workbench is carried somewhere.
func TestTheWaitingFlagRidesTheInterchange(t *testing.T) {
	first := filepath.Join(t.TempDir(), "first")
	definition, err := ReadDefinition([]byte(waitingDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := Instantiate(first, "wt", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	opened, err := Open(first)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	exported, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	states := exportedStates(t, exported)
	if len(states) != 2 {
		t.Fatalf("wanted two states in the export, got %d", len(states))
	}
	if _, carried := states[0]["awaiting_outside"]; carried {
		t.Error("the export carried the member on a state that does not declare it")
	}
	var flag bool
	raw, carried := states[1]["awaiting_outside"]
	if !carried {
		t.Fatal("the export left the member off the state that declares it")
	}
	if err := json.Unmarshal(raw, &flag); err != nil || !flag {
		t.Errorf("the member exported as %s, wanted true", raw)
	}

	// init --from reads the export back, which is the import half, and an
	// export of the result matching the first byte for byte is what proves
	// nothing was dropped or invented in between.
	second := filepath.Join(t.TempDir(), "second")
	reread, err := ReadDefinition(exported)
	if err != nil {
		t.Fatalf("read the export back: %v", err)
	}
	if err := Instantiate(second, "wt", "alka", reread); err != nil {
		t.Fatalf("instantiate the import: %v", err)
	}
	imported, err := Open(second)
	if err != nil {
		t.Fatalf("open the import: %v", err)
	}
	if !imported.State("d00000000002").AwaitingOutside {
		t.Error("the import did not reproduce the flag on the state that declares it")
	}
	if imported.State("d00000000001").AwaitingOutside {
		t.Error("the import invented the flag on a state that does not declare it")
	}
	again, err := imported.Export()
	if err != nil {
		t.Fatalf("export the import: %v", err)
	}
	if string(again) != string(exported) {
		t.Errorf("the round trip is not byte for byte:\nfirst:\n%s\nsecond:\n%s", exported, again)
	}

	// The anchor on disk carries the key as the frontmatter it was declared
	// in, rather than as a preserved unknown member with a JSON value.
	text, err := os.ReadFile(filepath.Join(second, StatesDir, "d00000000002", StateAnchor))
	if err != nil {
		t.Fatalf("read the imported anchor: %v", err)
	}
	if !strings.Contains(string(text), "awaiting_outside: true") {
		t.Errorf("the imported anchor reads:\n%s", text)
	}
}

// exportedStates pulls the states array out of an export as raw objects.
func exportedStates(t *testing.T, exported []byte) []map[string]json.RawMessage {
	t.Helper()
	var object struct {
		States []map[string]json.RawMessage `json:"states"`
	}
	if err := json.Unmarshal(exported, &object); err != nil {
		t.Fatalf("unmarshal the export: %v", err)
	}
	return object.States
}
