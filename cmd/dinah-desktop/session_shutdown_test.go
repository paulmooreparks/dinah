package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

type shutdownCloseConnection struct {
	*testProviderConnection
	entered, release, stopped chan struct{}
	stopOnce                  sync.Once
}

func (c *shutdownCloseConnection) Call(ctx context.Context, method string, params, result any) error {
	if method == "session/close" {
		close(c.entered)
		<-c.release
	}
	return c.testProviderConnection.Call(ctx, method, params, result)
}
func (c *shutdownCloseConnection) Close() {
	c.stopOnce.Do(func() { close(c.stopped) })
	c.testProviderConnection.Close()
}

func TestShutdownJoinsTrackedSessionCloseBeforeDependencyTeardown(t *testing.T) {
	a, base := providerTestApp(t)
	c := &shutdownCloseConnection{testProviderConnection: base, entered: make(chan struct{}), release: make(chan struct{}), stopped: make(chan struct{})}
	initial := a.runtime.ProviderLocked("codex").Initialization()
	initial.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
	provider := newTestProvider(c, initial)
	a.runtime.PutProviderLocked("codex", provider)
	go a.lifecycle().Watch("codex", provider)
	a.sink = func(string, any) {}
	// Keep the actual bus handle, so reopening a closed handle cannot hide teardown.
	b, err := a.busHost().Bus()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	a.wakeNow = func() time.Time { return now }
	s := newSessionState(&sessionSeed{ID: "codex:closing", Provider: "codex"}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	a.ChatWindows(nil)
	a.closedSessions().EndClosed()
	now = now.Add(closedSessionEnd)
	a.closedSessions().EndClosed()
	select {
	case <-c.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("tracked close did not start")
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(c.release) }) }
	done := make(chan struct{})
	t.Cleanup(func() {
		release()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("shutdown did not finish")
		}
	})
	go func() { a.shutdown(context.Background()); close(done) }()
	// The close task is held at a known boundary. A provider stop here would
	// prove that shutdown crossed the dependency teardown boundary too soon.
	select {
	case <-c.stopped:
		t.Fatal("provider teardown preceded tracked session close")
	case <-done:
		t.Fatal("shutdown returned while session close was blocked")
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := b.WaitingQuestions(context.Background()); err != nil {
		t.Fatalf("bus closed before session completion: %v", err)
	}
	release()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not join released close")
	}
	select {
	case <-c.stopped:
	default:
		t.Fatal("provider was not stopped after session close")
	}
}
