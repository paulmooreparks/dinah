package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// second opens a second library over the same bench, which is what a test
// uses to stand for another process reaching the same entity.
func (h *harness) second() *Library {
	h.t.Helper()
	opened, err := bench.Open(h.root)
	if err != nil {
		h.t.Fatalf("open a second view of the bench: %v", err)
	}
	other := New(opened, h.home)
	other.Now = h.library.Now
	return other
}

// hold takes an entity's own lock from outside the library, standing for a
// process that is already mid-transaction on it.
func (h *harness) hold(dir, actor string) *bench.Lock {
	h.t.Helper()
	lock, err := bench.Acquire(dir, actor, bench.Stamp(h.clock))
	if err != nil {
		h.t.Fatalf("hold %s: %v", dir, err)
	}
	return lock
}

// plant writes a lock file by hand at a path of the test's choosing, which is
// how a bench lock, a sibling or the leftovers of a crash are set up.
func (h *harness) plant(path string, record bench.LockRecord) {
	h.t.Helper()
	line, err := json.Marshal(record)
	if err != nil {
		h.t.Fatalf("marshal: %v", err)
	}
	if err := bench.WriteText(path, string(line)+"\n"); err != nil {
		h.t.Fatalf("plant %s: %v", path, err)
	}
}

// locks lists every lock file standing anywhere on the bench, by path
// relative to the bench root, which is what proves an unwind gave back
// exactly what it took.
func (h *harness) locks() []string {
	h.t.Helper()
	var found []string
	err := filepath.WalkDir(h.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name := entry.Name()
		if name != bench.LockName && !strings.HasSuffix(name, bench.SiblingSuffix) {
			return nil
		}
		relative, relErr := filepath.Rel(h.root, path)
		if relErr != nil {
			return relErr
		}
		found = append(found, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		h.t.Fatalf("walk: %v", err)
	}
	sort.Strings(found)
	return found
}

// abortAt arms the bench so the structural protocol stops at one numbered
// step the way a process that died there would, releasing nothing and
// unwinding nothing.
func (h *harness) abortAt(step int) {
	h.library.Bench.Hooks = &bench.Hooks{
		AfterStep: func(n int) error {
			if n == step {
				return bench.ErrAborted
			}
			return nil
		},
	}
}

// inWindow arms the bench so f runs between the release of the entity's own
// lock and the move of its directory, which is the window the sibling covers.
func (h *harness) inWindow(f func()) {
	h.library.Bench.Hooks = &bench.Hooks{
		AfterStep: func(n int) error {
			if n == 5 {
				f()
			}
			return nil
		},
	}
}

// clearBenchLock removes the bench lock a dead act left behind. It is the one
// step the design deliberately leaves to a person: nothing auto-breaks a lock,
// so the sequence after a crash is that a human removes the named bench lock
// and then runs the finish.
func (h *harness) clearBenchLock() {
	h.t.Helper()
	if err := os.Remove(filepath.Join(h.root, bench.LockName)); err != nil && !os.IsNotExist(err) {
		h.t.Fatalf("clear the bench lock: %v", err)
	}
}

// finish runs the checker with the finish marker set, which completes or rolls
// back the interrupted acts it reports.
func (h *harness) finish() ([]bench.Finding, error) {
	h.reopen()
	return h.library.Check(&Request{Verb: "check", Actor: "alka", Finish: true})
}

// check runs the checker over the bench as it now stands.
func (h *harness) check() []bench.Finding {
	h.t.Helper()
	findings, err := h.library.Check(&Request{Verb: "check", Actor: "alka"})
	if err != nil {
		h.t.Fatalf("check: %v", err)
	}
	return findings
}

// finding reports whether a catalog key appears among some findings, and what
// detail it carried.
func finding(findings []bench.Finding, key string) (string, bool) {
	for _, f := range findings {
		if f.Key == key {
			return f.Detail, true
		}
	}
	return "", false
}

// benchEvents reads the bench's own journal.
func (h *harness) benchEvents() []bench.Event {
	h.t.Helper()
	list, _, err := bench.ReadJournal(h.library.Bench.JournalPath())
	if err != nil {
		h.t.Fatalf("bench journal: %v", err)
	}
	return list
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
