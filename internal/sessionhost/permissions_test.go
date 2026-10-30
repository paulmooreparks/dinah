package sessionhost

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
)

func TestPermissionsCallbackOrderAndRemovalBeforeAnswer(t *testing.T) {
	var p *Permissions
	var order []string
	p = NewPermissions(PermissionHost{
		NextKey: func() string { return "q7" },
		Emit: func(name string, data any) {
			order = append(order, name)
			if name == "agent:permission" {
				if !p.HasPending("chat") {
					t.Fatal("event preceded registration")
				}
				ask := data.(map[string]any)
				if !reflect.DeepEqual(ask["subagent"], map[string]string{"id": "child", "name": "Helper"}) {
					t.Fatal(ask)
				}
				p.Answer("q7", "allow")
				if p.HasPending("chat") {
					t.Fatal("answer retained waiter")
				}
				p.CancelSession("chat")
			} else if p.HasPending("chat") {
				t.Fatal("answered event preceded removal")
			}
		},
		NoticeQuestion: func(string, json.RawMessage) { order = append(order, "notice") },
	})
	id, ok := p.Ask(context.Background(), "chat", &SubagentInfo{ID: "child", Name: "Helper"}, nil, nil)
	if id != "allow" || !ok {
		t.Fatalf("answer %q %v", id, ok)
	}
	if !reflect.DeepEqual(order, []string{"agent:permission", "notice", "agent:answered"}) {
		t.Fatal(order)
	}
}

// Pending returns, oldest first, exactly what each agent:permission event
// carried for the questions still open, and nothing for one that has been
// answered or cancelled.
func TestPermissionsPendingIsWhatTheEventCarried(t *testing.T) {
	var mu sync.Mutex
	var events []map[string]any
	next := 0
	emitted := make(chan struct{}, 3)
	p := NewPermissions(PermissionHost{
		NextKey: func() string { mu.Lock(); defer mu.Unlock(); next++; return "q" + string(rune('0'+next)) },
		Emit: func(name string, data any) {
			if name == "agent:permission" {
				mu.Lock()
				events = append(events, data.(map[string]any))
				mu.Unlock()
				emitted <- struct{}{}
			}
		},
		NoticeQuestion: func(string, json.RawMessage) {},
	})
	if got := p.Pending(); len(got) != 0 {
		t.Fatalf("pending before any question: %v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i, session := range []string{"a", "b", "c"} {
		var sub *SubagentInfo
		if i == 1 {
			sub = &SubagentInfo{ID: "child", Name: "Helper"}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Ask(ctx, session, sub, json.RawMessage(`{"title":"`+session+`"}`), nil)
		}()
		<-emitted
	}
	got := p.Pending()
	if len(got) != 3 || !reflect.DeepEqual(got, events) {
		t.Fatalf("pending = %v, events = %v", got, events)
	}
	if got[1]["subagent"] == nil {
		t.Fatal("a subagent's question lost its attribution")
	}
	// The returned maps are copies.
	got[0]["key"] = "changed"
	if p.Pending()[0]["key"] != "q1" {
		t.Fatal("Pending shares its maps with the caller")
	}
	p.Answer("q2", "allow")
	p.CancelSession("a")
	left := p.Pending()
	if len(left) != 1 || left[0]["key"] != "q3" {
		t.Fatalf("after an answer and a cancellation: %v", left)
	}
	cancel()
	wg.Wait()
	if got := p.Pending(); len(got) != 0 {
		t.Fatalf("pending after every question ended: %v", got)
	}
}

func TestPermissionsCancelledContextStillPublishesAndNotifies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var order []string
	p := NewPermissions(PermissionHost{NextKey: func() string { return "q1" }, Emit: func(name string, _ any) { order = append(order, name) }, NoticeQuestion: func(string, json.RawMessage) { order = append(order, "notice") }})
	id, ok := p.Ask(ctx, "chat", nil, nil, nil)
	if id != "" || ok || p.HasPending("chat") {
		t.Fatalf("cancelled %q %v", id, ok)
	}
	if !reflect.DeepEqual(order, []string{"agent:permission", "notice", "agent:answered"}) {
		t.Fatal(order)
	}
}

func TestPermissionsConcurrentAnswerAndCancellationHaveOneOwner(t *testing.T) {
	for i := 0; i < 100; i++ {
		ready := make(chan struct{})
		done := make(chan struct{})
		var valid bool
		p := NewPermissions(PermissionHost{
			NextKey: func() string { return "q1" },
			Emit: func(name string, _ any) {
				if name == "agent:permission" {
					close(ready)
				}
			},
			NoticeQuestion: func(string, json.RawMessage) {},
		})
		go func() {
			id, ok := p.Ask(context.Background(), "chat", &SubagentInfo{ID: "child"}, nil, nil)
			valid = (ok && id == "allow") || (!ok && id == "")
			close(done)
		}()
		receiveLifecycle(t, ready)
		var wg sync.WaitGroup
		for _, action := range []func(){func() { p.Answer("q1", "allow") }, func() { p.CancelSession("chat") }, func() { p.CancelSubagents(map[string]bool{"child": true}) }} {
			wg.Add(1)
			go func(f func()) { defer wg.Done(); f() }(action)
		}
		wg.Wait()
		receiveLifecycle(t, done)
		if !valid || p.HasPending("chat") {
			t.Fatal("invalid settlement")
		}
		p.Answer("q1", "duplicate")
		p.CancelSession("chat")
	}
}

func TestPermissionsSubagentCancellationLeavesOtherWaiters(t *testing.T) {
	p := NewPermissions(PermissionHost{})
	child := &question{session: "chat", subagent: "child", ch: make(chan string, 1)}
	parent := &question{session: "chat", ch: make(chan string, 1)}
	other := &question{session: "other", subagent: "other", ch: make(chan string, 1)}
	p.questions = map[string]*question{"child": child, "parent": parent, "other": other}
	p.CancelSubagents(map[string]bool{"child": true, "": true})
	if _, ok := <-child.ch; ok {
		t.Fatal("child not cancelled")
	}
	if !p.HasPending("chat") || !p.HasPending("other") {
		t.Fatal("unrelated waiter removed")
	}
	p.CancelSession("chat")
	if _, ok := <-parent.ch; ok {
		t.Fatal("parent not cancelled")
	}
	if !p.HasPending("other") {
		t.Fatal("other session removed")
	}
}
