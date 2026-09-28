//go:build windows

package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"dinah/internal/bench"
	"dinah/internal/durable"
	"dinah/internal/verb"
)

// waitSession drives one connection whose head surfaces waits, reading every
// line it writes, notifications included.
type waitSession struct {
	t    *testing.T
	in   *io.PipeWriter
	out  *bufio.Scanner
	done chan error
}

// newWaitSession starts a head that installs its wait notices as
// durable.Waiting, the way cmd/dinah installs them.
func newWaitSession(t *testing.T, library *verb.Library) *waitSession {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	install := func(send func(durable.Wait)) { durable.Waiting = send }
	go func() {
		err := serveWith(library.Bench.Root, library, map[string]*verb.Library{}, inR, outW, ProfileAll, newChainMemory(), install)
		outW.Close()
		done <- err
	}()
	session := &waitSession{t: t, in: inW, out: bufio.NewScanner(outR), done: done}
	session.out.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	t.Cleanup(func() {
		inW.Close()
		<-done
		durable.Waiting = nil
	})
	return session
}

// send writes one request line.
func (s *waitSession) send(request map[string]any) {
	s.t.Helper()
	encoded, err := json.Marshal(request)
	if err != nil {
		s.t.Fatalf("encode: %v", err)
	}
	if _, err := s.in.Write(append(encoded, '\n')); err != nil {
		s.t.Fatalf("write: %v", err)
	}
}

// until reads lines until the response carrying id, answering every
// notification read before it and the response itself. each, when not nil,
// is called with each notification as it is read.
func (s *waitSession) until(id int, each func(map[string]any)) ([]map[string]any, map[string]any) {
	s.t.Helper()
	var notices []map[string]any
	for s.out.Scan() {
		var line map[string]any
		if err := json.Unmarshal(s.out.Bytes(), &line); err != nil {
			s.t.Fatalf("decode %q: %v", s.out.Text(), err)
		}
		if got, ok := line["id"].(float64); ok && int(got) == id {
			return notices, line
		}
		notices = append(notices, line)
		if each != nil {
			each(line)
		}
	}
	s.t.Fatalf("the connection closed before answering %d: %v", id, s.out.Err())
	return nil, nil
}

// heldBy opens path the way a process Dinah does not control might, sharing
// only what share names. The answer closes it at most once.
func heldBy(t *testing.T, path string, share uint32) func() {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("convert %s: %v", path, err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, share, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("hold %s: %v", path, err)
	}
	var once sync.Once
	closeIt := func() { once.Do(func() { windows.CloseHandle(handle) }) }
	t.Cleanup(closeIt)
	return closeIt
}

// callRequest composes one tools/call request, carrying a progress token when
// token is not empty.
func callRequest(id int, tool string, arguments map[string]any, token string) map[string]any {
	params := map[string]any{"name": tool, "arguments": arguments}
	if token != "" {
		params["_meta"] = map[string]any{"progressToken": token}
	}
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": params}
}

// TestAHeldFileDoesNotHangTheServer asserts that, with the budget at 100 ms,
// a show of a card whose card.md another process holds sharing nothing is
// answered dinah.busy within the budget and the next request on the same
// connection is answered; that initialize declares the logging capability;
// and that a move whose journal is held after its anchor write sends
// notifications/message at level warning at least twice, and
// notifications/progress carrying the request's token, before its answer.
func TestAHeldFileDoesNotHangTheServer(t *testing.T) {
	library := newLibrary(t)
	saved := durable.RetryBudget
	durable.RetryBudget = 100 * time.Millisecond
	t.Cleanup(func() { durable.RetryBudget = saved })
	session := newWaitSession(t, library)

	session.send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	_, initialized := session.until(1, nil)
	capabilities, _ := initialized["result"].(map[string]any)["capabilities"].(map[string]any)
	if _, declared := capabilities["logging"]; !declared {
		t.Errorf("initialize declares %v, wanted the logging capability", capabilities)
	}

	resolved, err := library.Bench.ResolveCard("fx-1")
	if err != nil {
		t.Fatalf("resolve fx-1: %v", err)
	}
	closeCard := heldBy(t, filepath.Join(resolved.Card.Dir, bench.CardAnchor), 0)
	began := time.Now()
	session.send(callRequest(2, "show", map[string]any{"actor": "alka", "card": "fx-1"}, ""))
	_, shown := session.until(2, nil)
	elapsed := time.Since(began)
	if refusal := toolRefusal(t, shown); refusal != "dinah.busy" || elapsed > time.Second {
		t.Errorf("a show of a held card answered %q after %v, wanted dinah.busy within the budget", refusal, elapsed)
	}
	session.send(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "ping"})
	if _, pinged := session.until(3, nil); pinged["error"] != nil {
		t.Errorf("the next request was answered %v", pinged)
	}
	closeCard()

	waiting, err := library.Bench.ResolveCard("fx-2")
	if err != nil {
		t.Fatalf("resolve fx-2: %v", err)
	}
	closeJournal := heldBy(t, waiting.Card.JournalPath(), windows.FILE_SHARE_READ)
	messages := 0
	session.send(callRequest(4, "move", map[string]any{"actor": "alka", "card": "fx-2", "column": "doing"}, "tok-4"))
	notices, moved := session.until(4, func(line map[string]any) {
		if line["method"] == "notifications/message" {
			messages++
			if messages == 2 {
				closeJournal()
			}
		}
	})
	if refusal := toolRefusal(t, moved); refusal != "" {
		t.Errorf("the waiting move answered %q, wanted ok", refusal)
	}
	logged, progressed := 0, 0
	for _, notice := range notices {
		params, _ := notice["params"].(map[string]any)
		switch notice["method"] {
		case "notifications/message":
			logged++
			if params["level"] != "warning" || params["logger"] != "dinah" || !strings.Contains(params["data"].(string), "journal.ndjson") {
				t.Errorf("a log notification carried %v", params)
			}
		case "notifications/progress":
			progressed++
			if params["progressToken"] != "tok-4" {
				t.Errorf("a progress notification carried %v", params)
			}
		}
	}
	if logged < 2 || progressed < 2 {
		t.Errorf("the move sent %d log and %d progress notifications before its answer, wanted at least two of each", logged, progressed)
	}
}

// toolRefusal answers the refusal a tools/call answer carried, empty when it
// carried none.
func toolRefusal(t *testing.T, line map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(line["result"])
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded := payload(t, &response{Result: json.RawMessage(encoded)})
	refusal, _ := decoded["refusal"].(string)
	return refusal
}
