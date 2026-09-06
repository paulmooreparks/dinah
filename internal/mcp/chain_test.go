package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// chainSession drives several tool calls down one connection and reads each
// answer back before writing the next. Every other test in this package makes
// one Serve call per request, which is one connection per request, and a
// connection that has sent nothing withholds nothing. The rule under test is
// about what a connection remembers across acts, so it needs a session that
// outlives a single call.
//
// The two pipes are what make the interleaving real: the head is reading and
// answering while the test is still deciding what to send, which is also what
// lets a test edit an instruction file between two acts of one session.
type chainSession struct {
	t      *testing.T
	in     *io.PipeWriter
	out    *bufio.Scanner
	done   chan error
	closed sync.Once
	// memory is the connection's own record, held here so a test can inject a
	// clock and read the counter rather than sleeping.
	memory *chainMemory
	id     int
}

// newChainSession starts one head over one pair of pipes and returns the
// session that drives it. The memory is built here rather than inside Serve, so
// the clock a test injects is the clock the head expires on.
func newChainSession(t *testing.T, library *verb.Library, memory *chainMemory) *chainSession {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := serveWith(library.Bench.Root, library, map[string]*verb.Library{}, inR, outW, memory)
		outW.Close()
		done <- err
	}()
	session := &chainSession{t: t, in: inW, out: bufio.NewScanner(outR), memory: memory, done: done}
	session.out.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	t.Cleanup(session.close)
	return session
}

// close ends the connection and waits for the head to return, so a test that
// ends leaves no goroutine writing into a closed pipe.
func (s *chainSession) close() {
	s.closed.Do(func() {
		s.in.Close()
		if err := <-s.done; err != nil {
			s.t.Errorf("serve: %v", err)
		}
	})
}

// call sends one tools/call down the connection and returns its payload.
func (s *chainSession) call(name string, arguments map[string]any) map[string]any {
	s.t.Helper()
	s.id++
	line, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      s.id,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": arguments},
	})
	if err != nil {
		s.t.Fatalf("encode the call: %v", err)
	}
	if _, err := s.in.Write(append(line, '\n')); err != nil {
		s.t.Fatalf("write the call: %v", err)
	}
	if !s.out.Scan() {
		s.t.Fatalf("the head answered nothing for %s: %v", name, s.out.Err())
	}
	answer := &response{}
	if err := json.Unmarshal(s.out.Bytes(), answer); err != nil {
		s.t.Fatalf("decode %q: %v", s.out.Text(), err)
	}
	return payload(s.t, answer)
}

// chain reads the instruction chain off whichever member the answering tool
// published it under, and fails the test where the answer carried none. A
// coordination act carries it at the top level and the instructions tool
// carries it under served, which is the surface's own published shape.
func chain(t *testing.T, answer map[string]any) verb.Instructions {
	t.Helper()
	encoded, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("re-encode the answer: %v", err)
	}
	var shape struct {
		Instructions *verb.Instructions `json:"instructions"`
		Served       *struct {
			Instructions verb.Instructions `json:"instructions"`
		} `json:"served"`
	}
	if err := json.Unmarshal(encoded, &shape); err != nil {
		t.Fatalf("decode the chain from %s: %v", encoded, err)
	}
	switch {
	case shape.Instructions != nil:
		return *shape.Instructions
	case shape.Served != nil:
		return shape.Served.Instructions
	}
	t.Fatalf("the answer carried no instruction chain: %s", encoded)
	return verb.Instructions{}
}

// wantFull fails unless every layer of the fixture's chain came back in full
// and nothing was named withheld.
func wantFull(t *testing.T, what string, served verb.Instructions, column string) {
	t.Helper()
	if !strings.Contains(served.Global, "Global text") {
		t.Errorf("%s: the global layer was not served in full: %q", what, served.Global)
	}
	if !strings.Contains(served.Standing, "Standing text") {
		t.Errorf("%s: the standing layer was not served in full: %q", what, served.Standing)
	}
	if !strings.Contains(served.Column, column) {
		t.Errorf("%s: the column layer was not served in full: %q", what, served.Column)
	}
	if len(served.Withheld) != 0 {
		t.Errorf("%s: a full serve named withheld layers: %v", what, served.Withheld)
	}
	if served.Reread != "" {
		t.Errorf("%s: a full serve carried a reread reference: %q", what, served.Reread)
	}
}

// wantWithheld fails unless exactly the named layers were withheld, in the
// order given, with no text carried for any of them.
func wantWithheld(t *testing.T, what string, served verb.Instructions, names ...string) {
	t.Helper()
	if strings.Join(served.Withheld, ",") != strings.Join(names, ",") {
		t.Errorf("%s: wanted the layers %v withheld, got %v", what, names, served.Withheld)
	}
	for _, name := range names {
		var carried string
		switch name {
		case verb.LayerGlobal:
			carried = served.Global
		case verb.LayerStanding:
			carried = served.Standing
		case verb.LayerColumn:
			carried = served.Column
		}
		if carried != "" {
			t.Errorf("%s: the %s layer was named withheld and carried anyway: %q", what, name, carried)
		}
	}
}

// writeGlobal writes the user-global instruction layer under a library's home,
// which is the one layer bench.GlobalInstructions reads from disk on each
// serve.
func writeGlobal(t *testing.T, library *verb.Library, text string) {
	t.Helper()
	dir := bench.UserBase(library.Home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("the user base: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, bench.InstructionsName), []byte(text), 0o644); err != nil {
		t.Fatalf("the global layer: %v", err)
	}
}

// TestTheChainIsWithheldAndRecovered asserts CORE-INSTR-8, CORE-INSTR-9 and
// CORE-INSTR-10 over one connection: the first claim serves all three layers,
// a second act at the same position names them withheld in the order global,
// standing, column and carries the column's own ref to fetch them back, and
// the request that ref names answers with all three in full.
//
// The withheld response is also checked for the word held, which this surface
// already publishes as the refusal name reported when another owner holds a
// card. A withheld layer must not land a second sense of that word on the
// answer to a claim that just succeeded.
func TestTheChainIsWithheldAndRecovered(t *testing.T) {
	library := newLibrary(t)
	writeGlobal(t, library, "Global text.\n")
	session := newChainSession(t, library, newChainMemory())

	claimed := session.call("claim", map[string]any{"card": "fx-1", "actor": "alka"})
	wantFull(t, "the first claim", chain(t, claimed), "Doing text")

	asked := session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"})
	served := chain(t, asked)
	wantWithheld(t, "a second act at the same position", served, verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn)
	if served.Reread != "doing" {
		t.Fatalf("wanted the column's own ref to read the chain back, got %q", served.Reread)
	}
	encoded, err := json.Marshal(asked)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if strings.Contains(string(encoded), `"held"`) {
		t.Errorf("a withholding answer publishes the member name held: %s", encoded)
	}

	recovered := session.call("instructions", map[string]any{"card": served.Reread, "actor": "alka"})
	wantFull(t, "the request the marker named", chain(t, recovered), "Doing text")
}

// TestTheColumnShapedRequestNeverWithholds asserts the other half of
// CORE-INSTR-10: the recovery route is unconditional, so asking twice running
// answers in full twice running.
func TestTheColumnShapedRequestNeverWithholds(t *testing.T) {
	library := newLibrary(t)
	writeGlobal(t, library, "Global text.\n")
	session := newChainSession(t, library, newChainMemory())

	wantFull(t, "the first column-shaped request", chain(t, session.call("instructions", map[string]any{"card": "doing", "actor": "alka"})), "Doing text")
	wantFull(t, "the second column-shaped request", chain(t, session.call("instructions", map[string]any{"card": "doing", "actor": "alka"})), "Doing text")
}

// TestTheFingerprintTracksTheTextTheHeadLoaded asserts what the record is
// keyed on and what a running session sees of an edit. Only the user-global
// layer is read from disk on each serve; this head opens the library once
// before it starts serving, so the standing text and every column's text are
// frozen for the life of the process. That is the behaviour today rather than
// something this rule introduced, and the test asserts it so that a head which
// starts caching the global layer too fails here.
func TestTheFingerprintTracksTheTextTheHeadLoaded(t *testing.T) {
	library := newLibrary(t)
	writeGlobal(t, library, "Global text.\n")
	session := newChainSession(t, library, newChainMemory())

	wantFull(t, "the claim", chain(t, session.call("claim", map[string]any{"card": "fx-1", "actor": "alka"})), "Doing text")
	wantWithheld(t, "a later act at the same position", chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"})),
		verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn)

	// The user-global layer is live. An edit changes the text, so it changes
	// the record's key, so the layer is absent from the set and is served in
	// full at the next act. The other two are still withheld.
	writeGlobal(t, library, "Global text, edited.\n")
	edited := chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"}))
	if !strings.Contains(edited.Global, "edited") {
		t.Errorf("an edit to the user-global layer did not reach the session: %q", edited.Global)
	}
	wantWithheld(t, "after the global edit", edited, verb.LayerStanding, verb.LayerColumn)

	// The standing and column texts are not live. Editing them on disk changes
	// nothing the head has loaded, so both layers keep the fingerprint they
	// were served under and stay withheld.
	rewriteWorkbench(t, library)
	still := chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"}))
	wantWithheld(t, "after the workbench edit", still, verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn)
}

// TestTheRecordIsKeyedOnTheTextNotThePosition asserts that a second card
// worked at the same column withholds the column layer, which is the repeat
// that falls across a card boundary. The record names the text, so the second
// card's arrival at a column this connection has already met serves nothing.
func TestTheRecordIsKeyedOnTheTextNotThePosition(t *testing.T) {
	library := newLibrary(t)
	writeGlobal(t, library, "Global text.\n")
	session := newChainSession(t, library, newChainMemory())

	wantFull(t, "the first card's claim", chain(t, session.call("claim", map[string]any{"card": "fx-1", "actor": "alka"})), "Doing text")
	arrived := chain(t, session.call("move", map[string]any{"card": "fx-2", "column": "doing", "actor": "alka"}))
	wantWithheld(t, "a second card arriving at the same column", arrived, verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn)
}

// TestTheHoldExpiresOnElapsedTime asserts the first limb of the bound. The
// head's clock is injected rather than slept on, so the test drives the
// comparison the head actually makes.
func TestTheHoldExpiresOnElapsedTime(t *testing.T) {
	library := newLibrary(t)
	writeGlobal(t, library, "Global text.\n")
	memory := newChainMemory()
	at := time.Date(2026, 9, 6, 9, 0, 0, 0, time.UTC)
	memory.now = func() time.Time { return at }
	session := newChainSession(t, library, memory)

	wantFull(t, "the claim", chain(t, session.call("claim", map[string]any{"card": "fx-1", "actor": "alka"})), "Doing text")
	wantWithheld(t, "the act after the claim", chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"})),
		verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn)

	// One second short of the bound is inside it, so the hold still holds.
	at = at.Add(HoldTTL - time.Second)
	wantWithheld(t, "one second short of the bound", chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"})),
		verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn)

	// At the bound the record is treated as absent, so the chain is served in
	// full again, unasked.
	at = at.Add(time.Second)
	wantFull(t, "at the bound", chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"})), "Doing text")

	// The serve records the layers again, so the window starts over.
	wantWithheld(t, "the act after the re-serve", chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"})),
		verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn)
}

// TestTheHoldExpiresOnCallsAnswered asserts the second limb. The counter counts
// every tool call the head answers for an owner and not only the acts that
// consult the chain, because the exposure being bounded is the agent's context
// churn, so the filler calls below are a read that touches no chain at all.
func TestTheHoldExpiresOnCallsAnswered(t *testing.T) {
	library := newLibrary(t)
	writeGlobal(t, library, "Global text.\n")
	session := newChainSession(t, library, newChainMemory())

	// The claim is the first call this owner makes on the connection, and it
	// is the call the record is stamped against.
	wantFull(t, "the claim", chain(t, session.call("claim", map[string]any{"card": "fx-1", "actor": "alka"})), "Doing text")

	// Filler calls up to one short of the ceiling. Counting the probe that
	// follows them, the head has answered exactly HoldActCeiling calls since
	// the serve, which is inside the bound.
	for answered := 0; answered < HoldActCeiling-1; answered++ {
		session.call("whoami", map[string]any{"actor": "alka"})
	}
	wantWithheld(t, "at the ceiling", chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"})),
		verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn)

	// One call past the ceiling the record is treated as absent.
	wantFull(t, "one call past the ceiling", chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"})), "Doing text")

	// The counter is per owner, so another owner's calls neither advance this
	// owner's ceiling nor read this owner's records.
	other := chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "bo"}))
	wantFull(t, "a second owner on the same connection", other, "Doing text")
	wantWithheld(t, "the first owner after the second owner acted", chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"})),
		verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn)
}

// rewriteWorkbench edits the workbench's standing text and the doing column's
// text on disk, under a running head, which is what the frozen-text assertion
// needs to have happened.
func rewriteWorkbench(t *testing.T, library *verb.Library) {
	t.Helper()
	anchor := filepath.Join(library.Bench.Root, bench.WorkbenchAnchor)
	replaceInFile(t, anchor, "Standing text.", "Standing text, edited.")
	column := filepath.Join(library.Bench.Root, bench.ColumnsDir, "a00000000002", bench.ColumnAnchor)
	replaceInFile(t, column, "Doing text.", "Doing text, edited.")
}

// replaceInFile rewrites one substring of a file and fails the test where the
// substring was not there to replace, so an edit that silently did nothing
// cannot pass for one that happened.
func replaceInFile(t *testing.T, path, old, new string) {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(text), old) {
		t.Fatalf("%s does not carry %q, so the edit would prove nothing", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(text), old, new, 1)), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
