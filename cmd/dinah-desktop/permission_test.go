package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"studio/internal/acp"
)

func TestPermissionCancellationRemovesPendingQuestion(t *testing.T) {
	for _, bySession := range []bool{false, true} {
		t.Run(map[bool]string{false: "request context", true: "session cancellation"}[bySession], func(t *testing.T) {
			a := &App{}
			asked := make(chan string, 1)
			answered := make(chan string, 1)
			a.sink = func(name string, data any) {
				switch name {
				case "agent:permission":
					asked <- data.(map[string]any)["key"].(string)
				case "agent:answered":
					answered <- data.(map[string]string)["key"]
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan any, 1)
			go func() {
				r, err := a.newAgentHandler().Request(ctx, "session/request_permission", json.RawMessage(`{"sessionId":"codex:test","options":[{"optionId":"accept","kind":"allow_once","name":"Accept"}]}`))
				if err != nil {
					t.Error(err)
				}
				result <- r
			}()
			var key string
			select {
			case key = <-asked:
			case <-time.After(time.Second):
				t.Fatal("permission was not published")
			}
			if bySession {
				a.permissions().CancelSession("codex:test")
			} else {
				cancel()
			}
			select {
			case r := <-result:
				if r.(acp.PermissionOutcome).Outcome.Outcome != "cancelled" {
					t.Fatalf("unexpected permission result: %#v", r)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled permission did not return")
			}
			if got := <-answered; got != key {
				t.Fatalf("cleared question %q, expected %q", got, key)
			}
			if a.permissions().HasPending("codex:test") {
				t.Fatal("cancelled permission remained pending")
			}
			a.Answer(key, "accept")
		})
	}
}

func TestPermissionAlreadyCancelledDoesNotPublish(t *testing.T) {
	a := &App{sink: func(name string, _ any) { t.Errorf("unexpected event %s", name) }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := a.newAgentHandler().Request(ctx, "session/request_permission", json.RawMessage(`{"sessionId":"codex:test"}`))
	if err != nil || r.(acp.PermissionOutcome).Outcome.Outcome != "cancelled" {
		t.Fatalf("unexpected permission result: %#v %v", r, err)
	}
}

// holdPermission exercises real waiter registration for continuation guards.
func holdPermission(t *testing.T, a *App, session string) func() {
	t.Helper()
	previous := a.sink
	ready, done := make(chan struct{}), make(chan struct{})
	a.sink = func(name string, data any) {
		if name == "agent:permission" {
			close(ready)
		}
		if previous != nil {
			previous(name, data)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { defer close(done); a.permissions().Ask(ctx, session, nil, nil, nil) }()
	select {
	case <-ready:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("permission was not registered")
	}
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("permission did not finish")
		}
		a.sink = previous
	}
}

func TestPermissionIDsUseTheAppCounter(t *testing.T) {
	a := &App{}
	a.qseq.Add(1) // Native windows use this same counter.
	var key string
	a.sink = func(name string, data any) {
		if name == "agent:permission" {
			key = data.(map[string]any)["key"].(string)
			a.Answer(key, "allow")
		}
	}
	id, ok := a.permissions().Ask(context.Background(), "chat", nil, nil, nil)
	if !ok || id != "allow" || key != "q2" || a.qseq.Add(1) != 3 {
		t.Fatalf("shared counter: %q %q %v", key, id, ok)
	}
}
