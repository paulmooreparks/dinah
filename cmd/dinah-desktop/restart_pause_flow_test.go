package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
)

type pauseFlowConnection struct {
	*testProviderConnection
	started chan struct{}
	finish  chan struct{}
	steered chan struct{}
	settle  chan struct{}
	once    sync.Once
	mu      sync.Mutex
}

func (c *pauseFlowConnection) CanSteer(string) bool { return true }
func (c *pauseFlowConnection) Steer(_ context.Context, p acp.PromptParams, delivered func()) error {
	if !strings.Contains(p.Prompt[0].Text, "Stop at the next safe point") {
		panic("unexpected steering")
	}
	delivered()
	c.once.Do(func() { close(c.finish) })
	if c.steered != nil {
		close(c.steered)
		<-c.settle
	}
	return nil
}

func TestPauseFlowCancelDuringSteeringStillWakesChat(t *testing.T) {
	a, base, s, _, turns := busChat(t)
	c := &pauseFlowConnection{testProviderConnection: base, started: make(chan struct{}), finish: make(chan struct{}), steered: make(chan struct{}), settle: make(chan struct{})}
	setTestConnection(a, "codex", c)
	a.update = &updateWatch{ready: true}
	if err := a.Send(s.ID(), "fixture original work", nil, nil); err != nil {
		t.Fatal(err)
	}
	<-c.started
	paused := make(chan error, 1)
	go func() { paused <- a.PauseAndRestart() }()
	<-c.steered
	a.CancelRestart()
	// Exercise the interval where cancellation used to consume the record
	// before the submitting call could mark the chat as told.
	time.Sleep(600 * time.Millisecond)
	close(c.settle)
	waitTurn(t, turns)
	if err := <-paused; err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for waiting(t, a, s.BusID()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if waiting(t, a, s.BusID()) == 0 {
		t.Fatal("cancel lost the carry-on message during steering")
	}
	ring(t, a)
	waitTurn(t, turns)
	c.mu.Lock()
	got := prompts(base)
	c.mu.Unlock()
	if len(got) != 2 || !strings.Contains(got[1], "was cancelled") {
		t.Fatalf("prompts: %q", got)
	}
}
func (c *pauseFlowConnection) Call(ctx context.Context, method string, p, out any) error {
	if method == "session/prompt" && strings.Contains(p.(acp.PromptParams).Prompt[0].Text, "fixture original work") {
		close(c.started)
		select {
		case <-c.finish:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.testProviderConnection.Call(ctx, method, p, out)
}

func TestPauseFlowSteersEndsAndCancelWakesOnce(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprintf("update=%t", update), func(t *testing.T) { testPauseCancel(t, update) })
	}
}
func testPauseCancel(t *testing.T, update bool) {
	a, base, s, _, turns := busChat(t)
	c := &pauseFlowConnection{testProviderConnection: base, started: make(chan struct{}), finish: make(chan struct{})}
	setTestConnection(a, "codex", c)
	// An admitted document write keeps the idle watcher from closing this
	// fixture before cancellation. No native runtime is used by this test.
	if update {
		a.update = &updateWatch{ready: true}
	}
	done, err := a.admitWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if err = a.Send(s.ID(), "fixture original work", nil, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-c.started:
	case <-time.After(testWait):
		t.Fatal("turn did not start")
	}
	if err = a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	t.Log("turn ended")
	r := readPaused()
	if len(r.Chats) != 1 || !r.Chats[0].Told {
		t.Fatalf("pause record: %+v", r)
	}
	a.CancelRestart()
	deadline := time.Now().Add(2 * time.Second)
	for waiting(t, a, s.BusID()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	ring(t, a)
	waitTurn(t, turns)
	t.Log("turn ended")
	// Replaying the durable record simulates recovery after enqueue succeeded
	// but deleting the file did not. The bus key must suppress a second turn.
	pausedMu.Lock()
	err = writePaused(r)
	pausedMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	a.resumePaused(false)
	ring(t, a)
	c.mu.Lock()
	got := prompts(base)
	c.mu.Unlock()
	if len(got) != 2 || !strings.Contains(got[1], "was cancelled") {
		t.Fatalf("prompts: %q", got)
	}
	if waiting(t, a, s.BusID()) != 0 {
		t.Fatal("duplicate carry-on is waiting")
	}
}

func TestPauseFlowSteeredChatResumesInFreshAppOnce(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprintf("update=%t", update), func(t *testing.T) { testPauseResume(t, update) })
	}
}

func testPauseResume(t *testing.T, update bool) {
	a, base, s, _, turns := busChat(t)
	c := &pauseFlowConnection{testProviderConnection: base, started: make(chan struct{}), finish: make(chan struct{})}
	setTestConnection(a, "codex", c)
	if update {
		a.update = &updateWatch{ready: true}
	}
	if err := a.Send(s.ID(), "fixture original work", nil, nil); err != nil {
		t.Fatal(err)
	}
	<-c.started
	if err := a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	deadline := time.Now().Add(2 * time.Second)
	for {
		a.updMu.Lock()
		closing := a.update.restarting
		a.updMu.Unlock()
		if closing {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("idle restart was not reached")
		}
		time.Sleep(time.Millisecond)
	}
	r := readPaused()
	if len(r.Chats) != 1 || !r.Chats[0].Told || !r.Chats[0].Ended || r.WithoutUpdate == update {
		t.Fatalf("record: %+v", r)
	}
	b, next := restarted(t, a)
	nextTurns := make(chan map[string]any, 8)
	b.sink = func(name string, data any) {
		if name == "agent:turn" {
			nextTurns <- data.(map[string]any)
		}
	}
	if _, err := b.OpenSessionAt(s.Cwd(), s.ID()); err != nil {
		t.Fatal(err)
	}
	b.resumePaused(true)
	ring(t, b)
	waitTurn(t, nextTurns)
	pausedMu.Lock()
	err := writePaused(r)
	pausedMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	b.resumePaused(true)
	ring(t, b)
	got := prompts(next)
	if len(got) != 1 || !strings.Contains(got[0], "You paused your work") || strings.Contains(got[0], "with its update") != update {
		t.Fatalf("resumed prompts: %q", got)
	}
}
