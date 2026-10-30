package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
)

// withSessionClose has the test agent advertise session/close.
func withSessionClose(a *App) {
	p := a.runtime.ProviderLocked("codex")
	initial := p.Initialization()
	initial.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
	setTestInitialization(a, "codex", initial)
}

// endRound runs the watcher's round once and waits for the sessions it
// began to end.
func endRound(t *testing.T, a *App) {
	t.Helper()
	wakeRound(t, a)
	a.closedSessions().Wait()
}

func countCalls(c *testProviderConnection, method string) int {
	n := 0
	for _, call := range c.calls {
		if call.method == method {
			n++
		}
	}
	return n
}

// A chat that closed itself in the background keeps its session for four
// hours, then Dinah closes the session, and a message that wakes the chat
// loads it again.
func TestAChatClosedForFourHoursHasItsSessionClosed(t *testing.T) {
	a, c := providerTestApp(t)
	withSessionClose(a)
	r := watchConnects(a)
	busID, _ := closedChat(t, a, "codex:parked", "Parked Chat")
	err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.CreateGroup(context.Background(), bus.Sender{}, "Parked Team", []string{busID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	a.ChatWindows(nil)
	wakeRound(t, a)
	openAsAsked(t, a, r.take())
	ring(t, a)
	waitTurn(t, r.turns)
	s := a.session("codex:parked")
	last := s.IdleSince()
	parkedAt := last.Add(backgroundIdle)
	a.wakeNow = func() time.Time { return parkedAt }
	endRound(t, a)
	if info := s.Info(); !info.Parked {
		t.Fatalf("the idle chat did not close itself: %+v", info)
	}
	a.wakeNow = func() time.Time { return parkedAt.Add(closedSessionEnd - time.Minute) }
	endRound(t, a)
	if a.session("codex:parked") != s || countCalls(c, "session/close") != 0 {
		t.Fatal("the chat's session ended before it had been closed for four hours")
	}
	a.wakeNow = func() time.Time { return parkedAt.Add(closedSessionEnd) }
	endRound(t, a)
	if countCalls(c, "session/close") != 1 {
		t.Fatalf("session/close was called %d times", countCalls(c, "session/close"))
	}
	if a.session("codex:parked") != nil {
		t.Fatal("the chat whose session was closed is still kept as loaded")
	}
	if info := s.Info(); info.Open || info.Parked || info.InBackground {
		t.Fatalf("the chat is not closed as a saved chat is: %+v", info)
	}
	if state := a.busChatState("codex:parked", false); state != "closed" {
		t.Fatalf("the chat's state is %q", state)
	}
	// A mention wakes it, and it is loaded again as a saved chat is.
	err = a.busHost().With(func(b *bus.Bus) error {
		_, err := b.SendMentioning(context.Background(), bus.Sender{}, "#Parked Team", "Back again?", nil, 0, []string{busID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	wakeRound(t, a)
	asked := r.take()
	if len(asked) != 1 || asked[0]["sessionId"] != "codex:parked" {
		t.Fatalf("the chat was not opened again for its message: %v", asked)
	}
	openAsAsked(t, a, asked)
	if countCalls(c, "session/load") != 2 {
		t.Fatalf("the chat was loaded %d times", countCalls(c, "session/load"))
	}
	ring(t, a)
	waitTurn(t, r.turns)
	if got := prompts(c); !strings.Contains(got[len(got)-1], "Back again?") {
		t.Fatalf("the message was not delivered: %q", got)
	}
}

// A chat whose window the person closed is closed too, and opening its
// window again before four hours starts its clock afresh.
func TestReopeningAClosedChatWindowCancelsTheEnd(t *testing.T) {
	a, c := providerTestApp(t)
	withSessionClose(a)
	cwd := t.TempDir()
	start := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	now := start
	a.wakeNow = func() time.Time { return now }
	a.ChatWindows([]string{"codex:windowed"})
	if _, err := a.OpenSessionAt(cwd, "codex:windowed"); err != nil {
		t.Fatal(err)
	}
	s := a.session("codex:windowed")
	endRound(t, a)
	now = start.Add(closedSessionEnd * 2)
	endRound(t, a)
	if a.session("codex:windowed") != s {
		t.Fatal("a chat with a window had its session ended")
	}
	// The person closes the window, and opens it again three hours later.
	a.ChatWindows(nil)
	closed := now
	endRound(t, a)
	now = closed.Add(3 * time.Hour)
	endRound(t, a)
	a.ChatWindows([]string{"codex:windowed"})
	endRound(t, a)
	// Closed again, the clock starts from this closing.
	a.ChatWindows(nil)
	closedAgain := now
	endRound(t, a)
	now = closedAgain.Add(closedSessionEnd - time.Minute)
	endRound(t, a)
	if a.session("codex:windowed") != s || countCalls(c, "session/close") != 0 {
		t.Fatal("the chat's session ended though it had been open within four hours")
	}
	now = closedAgain.Add(closedSessionEnd)
	endRound(t, a)
	if a.session("codex:windowed") != nil || countCalls(c, "session/close") != 1 {
		t.Fatal("the chat whose window was closed for four hours kept its session")
	}
	// Opening the window loads the chat again.
	a.ChatWindows([]string{"codex:windowed"})
	if _, err := a.OpenSessionAt(cwd, "codex:windowed"); err != nil {
		t.Fatal(err)
	}
	if countCalls(c, "session/load") != 2 {
		t.Fatalf("the chat was loaded %d times", countCalls(c, "session/load"))
	}
}

// A session the agent fails to close stays loaded, closed to the person.
func TestASessionTheAgentFailsToCloseStaysLoaded(t *testing.T) {
	a, c := providerTestApp(t)
	withSessionClose(a)
	c.closeErr = &acp.Error{Code: acp.CodeInternal, Message: "busy"}
	start := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	now := start
	a.wakeNow = func() time.Time { return now }
	a.ChatWindows(nil)
	if _, err := a.OpenSessionAt(t.TempDir(), "codex:stubborn"); err != nil {
		t.Fatal(err)
	}
	s := a.session("codex:stubborn")
	endRound(t, a)
	now = start.Add(closedSessionEnd)
	endRound(t, a)
	if countCalls(c, "session/close") != 1 || a.session("codex:stubborn") != s {
		t.Fatal("a session the agent did not close was let go")
	}
	if info := s.Info(); !info.Open || !info.Parked {
		t.Fatalf("the chat is not closed with its session loaded: %+v", info)
	}
	// It is tried again only after another four hours.
	c.closeErr = nil
	now = now.Add(closedSessionEnd - time.Minute)
	endRound(t, a)
	if countCalls(c, "session/close") != 1 {
		t.Fatal("closing the session was tried again too soon")
	}
	now = now.Add(time.Minute)
	endRound(t, a)
	if countCalls(c, "session/close") != 2 || a.session("codex:stubborn") != nil {
		t.Fatal("the session was not closed when the agent agreed")
	}
}

// An agent without session/close is stopped only when every chat it has
// loaded is due, and only when it can load them again.
func TestAnAgentWithoutSessionCloseIsStoppedOnlyWhenEveryChatIsDue(t *testing.T) {
	a, c := providerTestApp(t)
	var watched []*providerRuntime
	watchCurrent := func() {
		p := a.runtime.ProviderLocked("codex")
		watched = append(watched, p)
		go a.lifecycle().Watch("codex", p)
	}
	watchCurrent()
	t.Cleanup(func() {
		c.Close()
		for _, p := range watched {
			p.Wait()
		}
	})

	start := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	now := start
	a.wakeNow = func() time.Time { return now }
	a.ChatWindows([]string{"codex:open"})
	for _, id := range []string{"codex:open", "codex:closed"} {
		if _, err := a.OpenSessionAt(t.TempDir(), id); err != nil {
			t.Fatal(err)
		}
	}
	endRound(t, a)
	now = start.Add(closedSessionEnd)
	endRound(t, a)
	select {
	case <-c.Done():
		t.Fatal("the agent was stopped while one of its chats had a window")
	default:
	}
	if a.session("codex:closed") == nil {
		t.Fatal("a chat lost its session though its agent cannot close one")
	}
	// Without loadSession the chats could not come back, so nothing stops.
	p := a.runtime.ProviderLocked("codex")
	initial := p.Initialization()
	initial.AgentCapabilities.LoadSession = false
	setTestInitialization(a, "codex", initial)
	watchCurrent()
	a.ChatWindows(nil)
	endRound(t, a)
	now = now.Add(closedSessionEnd)
	endRound(t, a)
	select {
	case <-c.Done():
		t.Fatal("an agent that cannot load a session again was stopped")
	default:
	}
	initial.AgentCapabilities.LoadSession = true
	setTestInitialization(a, "codex", initial)
	p = a.runtime.ProviderLocked("codex")
	watchCurrent()
	endRound(t, a)
	select {
	case <-c.Done():
	case <-time.After(testWait):
		t.Fatal("the agent was not stopped when all its chats had been closed for four hours")
	}
	<-p.Finished()
	if a.session("codex:open") != nil || a.session("codex:closed") != nil {
		t.Fatal("the stopped agent's chats are still kept as loaded")
	}
	if countCalls(c, "session/close") != 0 {
		t.Fatal("session/close was called on an agent that does not advertise it")
	}
}
