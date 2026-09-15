package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dinah/internal/bench"
)

// holdFixtureDefinition is a workbench whose four columns declare the four
// things gate_items can say: nothing at all, the legacy entry spelling, the
// exit direction and both directions. The flow is otherwise the smallest one
// that opens, because nothing below reads a card.
const holdFixtureDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Holds",
  "instructions": "The standing text of this workbench.\n",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake",
      "instructions": "Intake instructions.\n" },
    { "id": "a00000000002", "title": "Entry", "kind": "work", "gate_items": true,
      "instructions": "Entry instructions.\n" },
    { "id": "a00000000003", "title": "Exit", "kind": "work", "gate_items": "out",
      "instructions": "Exit instructions.\n" },
    { "id": "a00000000004", "title": "Either", "kind": "done", "gate_items": "both",
      "instructions": "Either instructions.\n" }
  ]
}`

// TestTheStatusPayloadCarriesEachColumnsHoldInTheTypedSpelling drives
// dinah-506/criteria/2. The published hold is one word wide and a wrong answer
// is a well-formed string, so nothing downstream of it fails loudly: a client
// composes a confident sentence and a false one, and a reader of that sentence
// has no way to tell.
//
// The marshalled payload is the object compared rather than the struct the
// library returned, because omitempty is half of what the contract promises
// and a struct comparison cannot see it. The true row is the one that bites: it
// is the one declaration whose stored and typed spellings differ, so it is the
// row every wrong translation gets wrong, and routing this field through
// typedHold reddens it and the absent row together.
func TestTheStatusPayloadCarriesEachColumnsHoldInTheTypedSpelling(t *testing.T) {
	library := openHoldFixture(t)
	status, err := library.Status(&Request{Verb: "status", Actor: "alka"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal the status: %v", err)
	}
	var payload struct {
		Columns []map[string]any `json:"columns"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("read the status back: %v", err)
	}
	// The size of the sweep is asserted before its contents, so a fixture
	// that lost a column cannot pass by comparing an empty sweep with
	// itself.
	if len(payload.Columns) != 4 {
		t.Fatalf("the payload carries %d column views, wanted 4", len(payload.Columns))
	}

	for at, want := range []struct {
		title   string
		hold    string
		carried bool
	}{
		{title: "Intake", carried: false},
		{title: "Entry", hold: bench.HoldOn, carried: true},
		{title: "Exit", hold: bench.HoldOut, carried: true},
		{title: "Either", hold: bench.HoldBoth, carried: true},
	} {
		column := payload.Columns[at]
		if column["title"] != want.title {
			t.Fatalf("column %d is %v, wanted %s; the fixture's order decides which row is which", at, column["title"], want.title)
		}
		got, carried := column["hold"]
		if carried != want.carried {
			t.Errorf("%s publishes hold %v, wanted carried=%v", want.title, got, want.carried)
			continue
		}
		if want.carried && got != want.hold {
			t.Errorf("%s publishes hold %v, wanted %q", want.title, got, want.hold)
		}
	}
}

// openHoldFixture instantiates the four-column workbench above and opens a
// library over it. The shared harness builds one fixed flow and no column in
// it declares a hold, so this case plants its own rather than widening the
// definition every other test in this package reads.
func openHoldFixture(t *testing.T) *Library {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	root := filepath.Join(base, "workbench", bench.UserBaseName, harnessWorkbenchID)
	if err := os.MkdirAll(filepath.Join(home, bench.UserBaseName), 0o755); err != nil {
		t.Fatalf("user base: %v", err)
	}
	definition, err := bench.ReadDefinition([]byte(holdFixtureDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := bench.Instantiate(root, "hd", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	library := New(opened, home)
	library.Now = func() time.Time { return time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC) }
	return library
}
