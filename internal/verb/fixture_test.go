package verb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The identifiers of the fixture's five states, which the tests name
// directly. They are fixed rather than minted so that a test can assert an
// ordering without first reading the bench back.
const (
	intake    = "a00000000001"
	doing     = "a00000000002"
	review    = "a00000000003"
	finished  = "a00000000004"
	aftercare = "a00000000005"
)

// fixtureDefinition is the interchange form of the bench every test starts
// from: an intake station, a working station limited to one card, an
// operator-owned review station, a done station, and one station past the
// done station, so that a forward move out of a done state is reachable.
const fixtureDefinition = `{
  "profile": "dinah-core/1.0",
  "title": "Fixture",
  "instructions": "The standing text of this bench.\n",
  "states": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake",
      "instructions": "Intake instructions.\n" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "capacity": 1,
      "instructions": "Doing instructions.\n" },
    { "id": "a00000000003", "title": "Review", "kind": "work",
      "operator_owned": true, "instructions": "Review instructions.\n" },
    { "id": "a00000000004", "title": "Finished", "kind": "done",
      "instructions": "Finished instructions.\n" },
    { "id": "a00000000005", "title": "Aftercare", "kind": "work",
      "instructions": "Aftercare instructions.\n" }
  ]
}`

// harness is one test's bench, its library and the clock the library reads.
type harness struct {
	// library is the verb layer under test.
	library *Library
	// home is the user base this test's ladders and layers read from.
	home string
	// root is the bench directory.
	root string
	// clock is what Now returns, advanced by the tests that need it.
	clock time.Time
	// t is the test, so a helper can fail without returning an error.
	t *testing.T
}

// newHarness builds a bench from the fixture definition and opens it.
func newHarness(t *testing.T) *harness {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	root := filepath.Join(base, "bench")
	if err := os.MkdirAll(filepath.Join(home, bench.UserBaseName), 0o755); err != nil {
		t.Fatalf("user base: %v", err)
	}
	source := filepath.Join(base, "definition.json")
	if err := os.WriteFile(source, []byte(fixtureDefinition), 0o644); err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := Init(root, "fx", "alka", source); err != nil {
		t.Fatalf("init: %v", err)
	}
	h := &harness{home: home, root: root, clock: time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC), t: t}
	h.reopen()
	return h
}

// reopen reads the bench from disk again, which every test doing two acts in
// a row needs, since the library holds the definition it opened.
func (h *harness) reopen() {
	h.t.Helper()
	opened, err := bench.Open(h.root)
	if err != nil {
		h.t.Fatalf("open: %v", err)
	}
	h.library = New(opened, h.home)
	h.library.Now = func() time.Time { return h.clock }
}

// advance moves the clock forward, which is how a test reaches past a
// recorded expiry without waiting for it.
func (h *harness) advance(d time.Duration) {
	h.clock = h.clock.Add(d)
}

// add files a card and returns its reference.
func (h *harness) add(title string) string {
	h.t.Helper()
	response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: title})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("add %q: %s %s", title, response.Outcome, response.Refusal)
	}
	h.reopen()
	return response.Card.Ref
}

// do runs one contract verb and returns its response, reopening the bench
// afterwards so the next act reads what this one wrote.
func (h *harness) do(req *Request) *Response {
	h.t.Helper()
	response := h.library.Do(req)
	h.reopen()
	return response
}

// mustDo runs one contract verb and fails the test unless it succeeded.
func (h *harness) mustDo(req *Request) *Response {
	h.t.Helper()
	response := h.do(req)
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("%s: wanted ok, got %s %s", req.Verb, response.Outcome, response.Refusal)
	}
	return response
}

// card reads a card back from disk, which is how a test asserts what was
// written rather than what was returned.
func (h *harness) card(ref string) *bench.Card {
	h.t.Helper()
	found, err := h.library.Bench.ResolveCard(ref)
	if err != nil {
		h.t.Fatalf("resolve %s: %v", ref, err)
	}
	return found.Card
}

// events reads a card's journal.
func (h *harness) events(ref string) []bench.Event {
	h.t.Helper()
	list, _, err := bench.ReadJournal(h.card(ref).JournalPath())
	if err != nil {
		h.t.Fatalf("journal %s: %v", ref, err)
	}
	return list
}
