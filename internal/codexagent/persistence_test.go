package codexagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/profile"
)

// Each connection gets fresh loaded state. Only inject_items writes the shared
// store, matching the documented persistence operation rather than a file path.
type persistenceFixture struct {
	mu            sync.Mutex
	stored        map[string]string
	methods       []string
	injected      []json.RawMessage
	reject        bool
	beforeReceipt func()
}

func (f *persistenceFixture) connection(t *testing.T, cwd string) *Conn {
	loaded := false
	return fake(t, &testHandler{}, func(m message, send func(message)) {
		f.mu.Lock()
		f.methods = append(f.methods, m.Method)
		f.mu.Unlock()
		switch m.Method {
		case "model/list":
			send(answer(m, catalogResult()))
		case "thread/start":
			var p map[string]any
			_ = json.Unmarshal(m.Params, &p)
			if p["ephemeral"] != false {
				t.Error("new conversation was not explicitly persistent")
			}
			loaded = true
			send(answer(m, threadResult{Thread: thread{ID: "new-thread", Cwd: cwd}, Model: "test"}))
		case "thread/inject_items":
			f.mu.Lock()
			f.injected = append(f.injected, append(json.RawMessage{}, m.Params...))
			reject, hook := f.reject, f.beforeReceipt
			if !reject {
				if f.stored == nil {
					f.stored = map[string]string{}
				}
				f.stored["new-thread"] = cwd
			}
			f.mu.Unlock()
			if hook != nil {
				hook()
			}
			if reject {
				send(message{ID: m.ID, Error: &acp.Error{Code: acp.CodeMethodNotFound, Message: "persistence unsupported"}})
			} else {
				send(answer(m, map[string]any{}))
			}
		case "thread/read", "thread/resume":
			f.mu.Lock()
			stored := f.stored["new-thread"]
			f.mu.Unlock()
			if !loaded && stored == "" {
				send(message{ID: m.ID, Error: &acp.Error{Code: acp.CodeResourceNotFound, Message: "thread not loaded"}})
			} else {
				loaded = true
				send(answer(m, threadResult{Thread: thread{ID: "new-thread", Cwd: cwd}, Model: "test"}))
			}
		case "thread/unsubscribe":
			var p struct {
				ThreadID string `json:"threadId"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if p.ThreadID != "new-thread" {
				t.Errorf("cleanup targeted %q", p.ThreadID)
			}
			loaded = false
			send(answer(m, map[string]any{"status": "unsubscribed"}))
		case "turn/start":
			send(message{ID: m.ID, Error: &acp.Error{Code: acp.CodeInternal, Message: "synthetic first-turn failure"}})
		default:
			t.Errorf("unexpected method %s", m.Method)
		}
	})
}

func TestNewConversationPersistsBeforeAnyTurnAndResumesWithoutReinjection(t *testing.T) {
	for _, failedTurn := range []bool{false, true} {
		t.Run(fmt.Sprint(failedTurn), func(t *testing.T) {
			f := &persistenceFixture{}
			cwd := "C:/workspace with \"quotes\"/line\nbreak"
			c := f.connection(t, cwd)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var result acp.NewSessionResult
			if err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: cwd}, &result); err != nil {
				t.Fatal(err)
			}
			f.mu.Lock()
			if len(f.injected) != 1 || f.stored["new-thread"] != cwd {
				t.Fatal("published before persistence")
			}
			var payload struct {
				ThreadID string `json:"threadId"`
				Items    []struct {
					Type, Role string
					Content    []struct{ Type, Text string }
				} `json:"items"`
			}
			_ = json.Unmarshal(f.injected[0], &payload)
			f.mu.Unlock()
			want := fmt.Sprintf("This conversation is hosted by %s in workspace %q. This application context record is not a user request.", profile.Current().Name, cwd)
			if payload.ThreadID != "new-thread" || len(payload.Items) != 1 || payload.Items[0].Role != "developer" || payload.Items[0].Type != "message" || len(payload.Items[0].Content) != 1 || payload.Items[0].Content[0].Text != want || payload.Items[0].Content[0].Type != "input_text" {
				t.Fatalf("context payload %+v", payload)
			}
			if failedTurn {
				err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: result.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "fixture"}}}, nil)
				if err == nil || !strings.Contains(err.Error(), "synthetic first-turn failure") {
					t.Fatalf("turn failure %v", err)
				}
			}
			if err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: result.SessionID}, nil); err != nil {
				t.Fatal(err)
			}
			c.Close()
			restarted := f.connection(t, cwd)
			if err := restarted.Call(ctx, "session/load", acp.LoadSessionParams{SessionID: result.SessionID, Cwd: cwd}, nil); err != nil {
				t.Fatal(err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.injected) != 1 {
				t.Fatal("resume reinjected context")
			}
			turns := 0
			for _, m := range f.methods {
				if m == "turn/start" {
					turns++
				}
			}
			if turns != map[bool]int{false: 0, true: 1}[failedTurn] {
				t.Fatalf("unexpected turns %d", turns)
			}
		})
	}
}

func TestFailedOrCancelledPersistenceDoesNotPublish(t *testing.T) {
	for _, mode := range []string{"unsupported", "cancelled-receipt", "cancelled-before-write"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			f := &persistenceFixture{reject: mode == "unsupported"}
			if mode == "cancelled-receipt" {
				f.beforeReceipt = cancel
			}
			c := f.connection(t, "C:/fixture")
			if mode == "cancelled-before-write" {
				cancel()
			}
			var result acp.NewSessionResult
			err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: "C:/fixture"}, &result)
			if mode == "unsupported" {
				var rpc *acp.Error
				if !errors.As(err, &rpc) || rpc.Code != acp.CodeMethodNotFound || rpc.Message != "persistence unsupported" {
					t.Fatalf("original error %v", err)
				}
			} else if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation %v", err)
			}
			c.mu.Lock()
			registered := len(c.sessions)
			c.mu.Unlock()
			if result.SessionID != "" || registered != 0 {
				t.Fatal("failed persistence published a session")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if mode != "cancelled-before-write" && f.methods[len(f.methods)-1] != "thread/unsubscribe" {
				t.Fatalf("owned cleanup missing: %v", f.methods)
			}
		})
	}
}

func TestUnacknowledgedCleanupCannotHoldTheCreationError(t *testing.T) {
	cleanup := make(chan struct{})
	c := fake(t, &testHandler{}, func(m message, send func(message)) {
		switch m.Method {
		case "model/list":
			send(answer(m, catalogResult()))
		case "thread/start":
			send(answer(m, threadResult{Thread: thread{ID: "owned"}}))
		case "thread/inject_items":
			send(message{ID: m.ID, Error: &acp.Error{Code: acp.CodeInternal, Message: "original failure"}})
		case "thread/unsubscribe":
			var p struct {
				ThreadID string `json:"threadId"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if p.ThreadID != "owned" {
				t.Errorf("cleanup targeted %q", p.ThreadID)
			}
			close(cleanup)
			// The provider deliberately never acknowledges this owned cleanup.
		}
	})
	done := make(chan error, 1)
	go func() {
		done <- c.Call(context.Background(), "session/new", acp.NewSessionParams{Cwd: "C:/fixture"}, nil)
	}()
	select {
	case err := <-done:
		if err == nil || err.Error() != "original failure" {
			t.Fatalf("replaced original error: %v", err)
		}
		select {
		case <-cleanup:
		default:
			t.Fatal("cleanup was not attempted")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup exceeded its two-second budget")
	}
}

func TestBlockedCleanupWriteCannotHoldTheCreationError(t *testing.T) {
	release, cleanup := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	c := fake(t, &testHandler{}, func(m message, send func(message)) {
		switch m.Method {
		case "model/list":
			send(answer(m, catalogResult()))
		case "thread/start":
			send(answer(m, threadResult{Thread: thread{ID: "owned"}}))
		case "thread/inject_items":
			send(message{ID: m.ID, Error: &acp.Error{Code: acp.CodeInternal, Message: "original failure"}})
			// Stop reading requests, so unsubscribe blocks inside the pipe write.
			<-release
		case "thread/unsubscribe":
			var p struct {
				ThreadID string `json:"threadId"`
			}
			_ = json.Unmarshal(m.Params, &p)
			if p.ThreadID != "owned" {
				t.Errorf("cleanup targeted %q", p.ThreadID)
			}
			close(cleanup)
		}
	})
	done := make(chan error, 1)
	go func() {
		done <- c.Call(context.Background(), "session/new", acp.NewSessionParams{Cwd: "C:/fixture"}, nil)
	}()
	select {
	case err := <-done:
		if err == nil || err.Error() != "original failure" {
			t.Fatalf("replaced original error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocked transport write exceeded the cleanup budget")
	}
	unblock()
	select {
	case <-cleanup:
	case <-time.After(3 * time.Second):
		t.Fatal("owned cleanup did not finish when the pipe resumed")
	}
}
