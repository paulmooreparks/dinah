package main

import (
	"context"
	"testing"
	"time"

	"studio/internal/bus"
)

func TestClaudeHookWorkSurvivesTurnEndAndStopClearsIt(t *testing.T) {
	a, _ := providerTestApp(t)
	ctx := context.Background()
	s := newSessionState(&sessionSeed{ID: "claude:hook-work", Provider: "claude", Cwd: t.TempDir()}, sessionInitial{Open: true, Working: true})
	a.mu.Lock()
	a.runtime.PutConversationLocked(s.ID(), s)
	a.mu.Unlock()
	err := a.busHost().With(func(b *bus.Bus) error {
		chat, err := b.RegisterSession(ctx, "window-hook", s.Info().Cwd, "")
		if err != nil {
			return err
		}
		s.JoinBus(chat.ID)
		if err := b.BindAgentSession(ctx, "hook-work", chat.ID); err != nil {
			return err
		}
		if err := b.SubagentStarted(ctx, "hook-work", "actual-hook-id"); err != nil {
			return err
		}
		return b.SubagentStarted(ctx, "another-session", "foreign-hook-id")
	})
	if err != nil {
		t.Fatal(err)
	}
	// The ACP child ID need not equal the hook ID. No correspondence is inferred.
	s.SpawnSubagent(s.ID(), s.ID(), "claude:native-child", "Worker", "", "", false, false, "", "")
	a.turnSubagentsEnded(s)
	s.ReleaseWork()
	check := func(want string, count int) {
		t.Helper()
		st := chatStateOf(a, s.ID())
		if st.Exec == nil || st.Exec.State != want || st.Exec.Subagents != count {
			t.Fatalf("chat state = %+v, want %s with %d subagents", st.Exec, want, count)
		}
		if (a.ExitStatus().Chats == 1) != (count > 0) || (a.workingChats() == 1) != (count > 0) {
			t.Fatal("exit/restart disagree with observed activity")
		}
	}
	check(execBackground, 1)
	child, _ := s.Subagent("claude:native-child")
	if child.State != subagentRunning {
		t.Fatalf("turn end disconnected observed work: %+v", child)
	}
	s.ConnectedBackground(time.Now().Add(-time.Hour))
	if s.IdleBackground(time.Now(), time.Minute) || s.ParkAt(time.Now) {
		t.Fatal("hook work was eligible for parking")
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		if err := b.SubagentStopped(ctx, "hook-work", "actual-hook-id"); err != nil {
			return err
		}
		a.observeHookWork(ctx, b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	check(execIdle, 0)
	child, _ = s.Subagent("claude:native-child")
	if child.State != subagentDisconnected {
		t.Fatalf("unreported child outcome = %+v", child)
	}
}
