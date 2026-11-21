package codexagent

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
)

// threadServer is a fake app-server for the thread calls: it answers what a
// session needs, records every request with its parameters, and resumes each
// thread under the name given for it.
type threadServer struct {
	mu       sync.Mutex
	requests []message
	names    map[string]string // the name thread/resume returns, by thread
	refuse   map[string]bool   // the methods answered with an error
	started  int
}

func (f *threadServer) handle(m message, send func(message)) {
	f.mu.Lock()
	f.requests = append(f.requests, m)
	refused := f.refuse[m.Method]
	f.mu.Unlock()
	if refused {
		send(message{ID: m.ID, Error: &acp.Error{Code: -1, Message: "stand-in refusal"}})
		return
	}
	var p struct {
		ThreadID string `json:"threadId"`
	}
	_ = json.Unmarshal(m.Params, &p)
	switch m.Method {
	case "initialized":
	case "model/list":
		send(answer(m, catalogResult()))
	case "config/read":
		send(answer(m, map[string]any{"config": map[string]any{}}))
	case "thread/start":
		f.mu.Lock()
		f.started++
		id := "started" + string(rune('0'+f.started))
		f.mu.Unlock()
		send(answer(m, map[string]any{"thread": map[string]any{"id": id}, "model": "test", "reasoningEffort": "medium"}))
	case "thread/read":
		send(answer(m, map[string]any{"thread": map[string]any{"id": p.ThreadID, "cwd": "C:/test"}}))
	case "thread/resume":
		f.mu.Lock()
		name := f.names[p.ThreadID]
		f.mu.Unlock()
		thread := map[string]any{"id": p.ThreadID, "cwd": "C:/test", "turns": []any{}}
		if name != "" {
			thread["name"] = name
		}
		send(answer(m, map[string]any{"thread": thread, "model": "test", "reasoningEffort": "medium"}))
	case "thread/unsubscribe":
		send(answer(m, map[string]string{"status": "unsubscribed"}))
	default:
		send(answer(m, map[string]any{}))
	}
}

// sent are the parameters of each request of a method, in order.
func (f *threadServer) sent(method string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []map[string]any
	for _, m := range f.requests {
		if m.Method == method {
			var p map[string]any
			_ = json.Unmarshal(m.Params, &p)
			out = append(out, p)
		}
	}
	return out
}

func threadConn(t *testing.T, f *threadServer) (*Conn, context.Context) {
	t.Helper()
	c := fake(t, &testHandler{}, f.handle)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, nil); err != nil {
		t.Fatal(err)
	}
	return c, ctx
}

func (c *Conn) loaded(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessions[id] != nil
}

// A bare thread is ephemeral, so it is ended by closing it: Codex is sent
// thread/unsubscribe and never thread/delete, and the adapter keeps no entry
// for it. The same holds when Codex refuses the close, since the thread
// cannot be resumed either way; the refusal is returned for the caller to
// log.
func TestEndingABareSessionClosesItAndKeepsNoEntry(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(map[bool]string{false: "closed", true: "close refused"}[refused], func(t *testing.T) {
			f := &threadServer{}
			c, ctx := threadConn(t, f)
			var s acp.NewSessionResult
			if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: "C:/test", McpServers: []any{}}, Bare: true}, &s); err != nil {
				t.Fatal(err)
			}
			if !c.SessionEphemeral(s.SessionID) {
				t.Fatal("a bare session is not reported as ephemeral")
			}
			if refused {
				f.mu.Lock()
				f.refuse = map[string]bool{"thread/unsubscribe": true}
				f.mu.Unlock()
			}
			err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: s.SessionID}, nil)
			if (err != nil) != refused {
				t.Fatalf("the close returned %v", err)
			}
			if got := f.sent("thread/unsubscribe"); len(got) != 1 || got[0]["threadId"] != "started1" {
				t.Fatalf("thread/unsubscribe was sent as %v", got)
			}
			if got := f.sent("thread/delete"); len(got) != 0 {
				t.Fatalf("ending a bare session sent thread/delete: %v", got)
			}
			if c.loaded(s.SessionID) || c.SessionEphemeral(s.SessionID) {
				t.Fatal("the ended bare session is still in the adapter's sessions")
			}
		})
	}
}

// An ordinary session is not ephemeral, and a close Codex refuses leaves it
// loaded, as before.
func TestAnOrdinarySessionIsNotEphemeral(t *testing.T) {
	f := &threadServer{refuse: map[string]bool{"thread/unsubscribe": true}}
	c, ctx := threadConn(t, f)
	var s acp.NewSessionResult
	if err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: "C:/test", McpServers: []any{}}, &s); err != nil {
		t.Fatal(err)
	}
	if c.SessionEphemeral(s.SessionID) {
		t.Fatal("an ordinary session is reported as ephemeral")
	}
	if err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: s.SessionID}, nil); err == nil {
		t.Fatal("the refused close was reported as done")
	}
	if !c.loaded(s.SessionID) {
		t.Fatal("a session whose close was refused was forgotten")
	}
}

// A new thread has no name, so the first title is sent with thread/name/set,
// and the same title is not sent again. A changed title is sent.
func TestANewThreadIsGivenItsTitleOnce(t *testing.T) {
	f := &threadServer{}
	c, ctx := threadConn(t, f)
	var s acp.NewSessionResult
	if err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: "C:/test", McpServers: []any{}}, &s); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Parser Work", "Parser Work", "  Parser Work  ", "", "Parser Review"} {
		if err := c.SetSessionTitle(ctx, s.SessionID, title); err != nil {
			t.Fatal(err)
		}
	}
	got := f.sent("thread/name/set")
	if len(got) != 2 || got[0]["threadId"] != "started1" || got[0]["name"] != "Parser Work" || got[1]["name"] != "Parser Review" {
		t.Fatalf("thread/name/set was sent as %v", got)
	}
}

// A resumed thread is named only when the chat's title differs from the name
// Codex returned for it (Thread.name).
func TestAResumedThreadIsNamedOnlyWhenItsNameDiffers(t *testing.T) {
	f := &threadServer{names: map[string]string{"same": "Parser Work", "other": "Dinah bus: 3 new messages"}}
	c, ctx := threadConn(t, f)
	for _, id := range []string{"codex:same", "codex:other", "codex:unnamed"} {
		if err := c.Call(ctx, "session/load", acp.LoadSessionParams{SessionID: id, Cwd: "C:/test", McpServers: []any{}}, nil); err != nil {
			t.Fatal(err)
		}
		if err := c.SetSessionTitle(ctx, id, "Parser Work"); err != nil {
			t.Fatal(err)
		}
	}
	got := f.sent("thread/name/set")
	if len(got) != 2 || got[0]["threadId"] != "other" || got[0]["name"] != "Parser Work" || got[1]["threadId"] != "unnamed" {
		t.Fatalf("thread/name/set was sent as %v", got)
	}
}

// A chat that is not loaded can be renamed too. Its thread's name is not
// known here, so the title is always sent, and a refusal is returned.
func TestAClosedChatsThreadIsNamed(t *testing.T) {
	f := &threadServer{}
	c, ctx := threadConn(t, f)
	if err := c.SetSessionTitle(ctx, "codex:closed", "Parser Work"); err != nil {
		t.Fatal(err)
	}
	if got := f.sent("thread/name/set"); len(got) != 1 || got[0]["threadId"] != "closed" || got[0]["name"] != "Parser Work" {
		t.Fatalf("thread/name/set was sent as %v", got)
	}
	f.mu.Lock()
	f.refuse = map[string]bool{"thread/name/set": true}
	f.mu.Unlock()
	if err := c.SetSessionTitle(ctx, "codex:closed", "Parser Review"); err == nil {
		t.Fatal("a refused name was reported as set")
	}
	if err := c.SetSessionTitle(ctx, "not-a-codex-id", "Parser Review"); err == nil {
		t.Fatal("a session ID of another kind was accepted")
	}
}

// Archiving and taking out of the archive are the documented calls with the
// thread's ID and nothing else, and each returns Codex's refusal.
func TestArchiveAndUnarchiveSendTheDocumentedCalls(t *testing.T) {
	f := &threadServer{}
	c, ctx := threadConn(t, f)
	if err := c.ArchiveSession(ctx, "codex:old"); err != nil {
		t.Fatal(err)
	}
	if err := c.UnarchiveSession(ctx, "codex:old"); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"thread/archive", "thread/unarchive"} {
		got := f.sent(method)
		if len(got) != 1 || len(got[0]) != 1 || got[0]["threadId"] != "old" {
			t.Fatalf("%s was sent as %v", method, got)
		}
	}
	f.mu.Lock()
	f.refuse = map[string]bool{"thread/archive": true, "thread/unarchive": true}
	f.mu.Unlock()
	if c.ArchiveSession(ctx, "codex:old") == nil || c.UnarchiveSession(ctx, "codex:old") == nil {
		t.Fatal("a refused call was reported as done")
	}
	if c.ArchiveSession(ctx, "old") == nil || c.UnarchiveSession(ctx, "old") == nil {
		t.Fatal("a session ID of another kind was accepted")
	}
}
