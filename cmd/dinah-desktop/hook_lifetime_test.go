package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/bus"
)

func hookLifetimeFixture(t *testing.T) (*App, *testProviderConnection, *Session) {
	t.Helper()
	a, _ := providerTestApp(t)
	conn := newTestProviderConnection()
	t.Cleanup(conn.Close)
	a.runtime.PutProviderLocked("claude", newTestProvider(conn, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{LoadSession: true}}))
	s := newSessionState(&sessionSeed{ID: "claude:lifetime", Provider: "claude", Cwd: t.TempDir()}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	err := a.busHost().With(func(b *bus.Bus) error {
		chat, err := b.RegisterSession(context.Background(), "window-lifetime", s.Cwd(), "Lifetime")
		if err != nil {
			return err
		}
		s.JoinBus(chat.ID)
		a.saveBusSession(s.ID(), chat.ID)
		if err := b.BindAgentSession(context.Background(), "lifetime", chat.ID); err != nil {
			return err
		}
		if err := b.SubagentStarted(context.Background(), "lifetime", "worker"); err != nil {
			return err
		}
		a.observeChatHookWork(context.Background(), b, s)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if st := chatStateOf(a, s.ID()); st.Exec == nil || st.Exec.State != execBackground {
		t.Fatalf("live hook did not show background: %+v", st.Exec)
	}
	return a, conn, s
}

func hookLifetimeCount(t *testing.T, a *App, s *Session) int {
	t.Helper()
	n := -1
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		n, err = b.ObservedSubagentsRunning(context.Background(), "window-lifetime", strings.TrimPrefix(s.ID(), "claude:"))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHookLifetimeResetsAtLoadResumeAndConfirmedClose(t *testing.T) {
	for _, method := range []string{"session/load", "session/resume", "session/close"} {
		t.Run(method, func(t *testing.T) {
			a, conn, s := hookLifetimeFixture(t)
			if method == "session/load" {
				conn.onLoad = func() {
					if n := hookLifetimeCount(t, a, s); n != 0 {
						t.Errorf("native load began before old observations retired: %d", n)
					}
				}
			}
			var result acp.LoadSessionResult
			if err := a.call(conn, method, acp.LoadSessionParams{SessionID: s.ID(), Cwd: s.Cwd()}, &result); err != nil {
				t.Fatal(err)
			}
			if n := hookLifetimeCount(t, a, s); n != 0 {
				t.Fatalf("old hook remains: %d", n)
			}
			if st := chatStateOf(a, s.ID()); st.Exec != nil && st.Exec.State == execBackground {
				t.Fatalf("chip remains: %+v", st.Exec)
			}
		})
	}
}

func TestHookLifetimeFailedCloseRetainsLiveEvidence(t *testing.T) {
	a, conn, s := hookLifetimeFixture(t)
	conn.closeErr = errors.New("close refused")
	if err := a.call(conn, "session/close", acp.CloseSessionParams{SessionID: s.ID()}, nil); err == nil {
		t.Fatal("failed close succeeded")
	}
	if n := hookLifetimeCount(t, a, s); n != 1 {
		t.Fatalf("failed close discarded live evidence: %d", n)
	}
}

func TestHookLifetimeFreshStartDuringLoadIsRetained(t *testing.T) {
	a, conn, s := hookLifetimeFixture(t)
	conn.onLoad = func() {
		if err := a.busHost().With(func(b *bus.Bus) error { return b.SubagentStarted(context.Background(), "lifetime", "new-worker") }); err != nil {
			t.Error(err)
		}
	}
	var result acp.LoadSessionResult
	if err := a.call(conn, "session/load", acp.LoadSessionParams{SessionID: s.ID(), Cwd: s.Cwd()}, &result); err != nil {
		t.Fatal(err)
	}
	if n := hookLifetimeCount(t, a, s); n != 1 {
		t.Fatalf("fresh hook lost: %d", n)
	}
}

func TestHookLifetimeConnectionLossAndFreshLoadClearChip(t *testing.T) {
	a, conn, s := hookLifetimeFixture(t)
	p := a.runtime.ProviderLocked("claude")
	go a.lifecycle().Watch("claude", p)
	conn.Close()
	p.Wait()
	if n := hookLifetimeCount(t, a, s); n != 0 {
		t.Fatalf("connection loss retained hook: %d", n)
	}
	fresh := newTestProviderConnection()
	t.Cleanup(fresh.Close)
	a.runtime.PutProviderLocked("claude", newTestProvider(fresh, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{LoadSession: true}}))
	if _, err := a.OpenSessionAt(s.Cwd(), s.ID()); err != nil {
		t.Fatal(err)
	}
	if n := hookLifetimeCount(t, a, s); n != 0 {
		t.Fatalf("fresh load resurrected hook: %d", n)
	}
	if st := chatStateOf(a, s.ID()); st.Exec != nil && st.Exec.State == execBackground {
		t.Fatalf("fresh load resurrected chip: %+v", st.Exec)
	}
}
