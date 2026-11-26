package bus

import (
	"context"
	"testing"
	"time"
)

// A subagent counts as running from its start to its stop. Starting it
// again does not count it twice, a stop for one that never started changes
// nothing, a hook that gave no usable ID is not recorded, one whose stop
// never came stops counting after a while, and a session that ends forgets
// its own.
func TestRunningSubagentsAreKeptByTheAgentsSession(t *testing.T) {
	ctx := context.Background()
	b, _ := open(t)
	now := time.Date(2026, 11, 19, 22, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	running := func(session string) int {
		t.Helper()
		n, err := b.SubagentsRunning(ctx, session)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, id := range []string{"sub-a", "sub-a", " sub-b "} {
		if err := b.SubagentStarted(ctx, "agent-1", id); err != nil {
			t.Fatal(err)
		}
	}
	for _, unusable := range []string{"", "  ", "${agent_id}"} {
		if err := b.SubagentStarted(ctx, "agent-1", unusable); err != nil {
			t.Fatal(err)
		}
	}
	b.SubagentStarted(ctx, "agent-2", "sub-a")
	if got := running("agent-1"); got != 2 {
		t.Fatalf("%d subagents are running for the first agent, want 2", got)
	}
	b.SubagentStopped(ctx, "agent-1", "never-started")
	b.SubagentStopped(ctx, "agent-1", "sub-a")
	if got := running("agent-1"); got != 1 {
		t.Fatalf("after one stopped, %d are running, want 1", got)
	}
	if got := running("agent-2"); got != 1 {
		t.Fatalf("another agent's subagent was stopped with it: %d", got)
	}
	now = now.Add(SubagentStaleAfter + time.Minute)
	if got := running("agent-1"); got != 0 {
		t.Fatalf("a subagent whose stop never came is still counted: %d", got)
	}
	b.SubagentStarted(ctx, "agent-2", "sub-c")
	if err := b.ForgetSubagents(ctx, "agent-2"); err != nil {
		t.Fatal(err)
	}
	if got := running("agent-2"); got != 0 {
		t.Fatalf("an ended session still has %d subagents", got)
	}
}

func TestSubagentWorkObservationHasNoSteeringTimeoutAndRequiresBinding(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	chat, err := b.RegisterSession(ctx, "window-observation", workspace(t, root, "work"), "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	b.now = func() time.Time { return now }
	if err := b.SubagentStarted(ctx, "native", "worker"); err != nil {
		t.Fatal(err)
	}
	check := func(session, native string, want int) {
		t.Helper()
		n, err := b.ObservedSubagentsRunning(ctx, session, native)
		if err != nil || n != want {
			t.Fatalf("observation = %d, %v; want %d", n, err, want)
		}
	}
	check(chat.ID, "native", -1)
	if err := b.BindAgentSession(ctx, "native", chat.ID); err != nil {
		t.Fatal(err)
	}
	check(chat.ID, "native", 1)
	now = now.Add(SubagentStaleAfter + time.Hour)
	check(chat.ID, "native", 1)
	check("another-chat", "native", -1)
	check(chat.ID, "other-native", -1)
	if err := b.SubagentStopped(ctx, "native", "worker"); err != nil {
		t.Fatal(err)
	}
	check(chat.ID, "native", 0)
}
