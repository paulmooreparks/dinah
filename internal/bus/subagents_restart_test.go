package bus

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUnmatchedSubagentStartIsNotLiveEvidenceAfterRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bus.db")
	b, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	chat, err := b.RegisterSession(ctx, "window-chat", t.TempDir(), "Chat")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.BindAgentSession(ctx, "native-session", chat.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.SubagentStarted(ctx, "native-session", "worker"); err != nil {
		t.Fatal(err)
	}
	if count, err := b.ObservedSubagentsRunning(ctx, chat.ID, "native-session"); err != nil || count != 1 {
		t.Fatalf("live start = %d, %v", count, err)
	}
	// The native worker was stopped without a stop hook. The entire host then
	// exits. Reopening has no live turn, connection, process or fresh hook.
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	// Bus storage reopening alone cannot establish a native lifetime boundary:
	// several clients may share the file. The host resets this chat before load.
	if err := b.ResetObservedSubagents(ctx, chat.ID, "native-session"); err != nil {
		t.Fatal(err)
	}
	if count, err := b.ObservedSubagentsRunning(ctx, chat.ID, "native-session"); err != nil || count != 0 {
		t.Fatalf("restart resurrected stale subagent work: count=%d, err=%v; want no live evidence", count, err)
	}
}

func TestHookLifetimeResetDoesNotClearAnotherChat(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	first, err := b.RegisterSession(ctx, "first", workspace(t, root, "work"), "First")
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.RegisterSession(ctx, "second", first.WorkspacePath, "Second")
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct{ native, chat string }{{"native-first", first.ID}, {"native-second", second.ID}} {
		if err := b.BindAgentSession(ctx, pair.native, pair.chat); err != nil {
			t.Fatal(err)
		}
		if err := b.SubagentStarted(ctx, pair.native, "worker"); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.ResetObservedSubagents(ctx, first.ID, "native-second"); err != nil {
		t.Fatal(err)
	}
	if n, err := b.ObservedSubagentsRunning(ctx, second.ID, "native-second"); err != nil || n != 1 {
		t.Fatalf("foreign reset changed count: %d %v", n, err)
	}
	if err := b.ResetObservedSubagents(ctx, first.ID, "native-first"); err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		native, chat string
		want         int
	}{{"native-first", first.ID, 0}, {"native-second", second.ID, 1}} {
		if n, err := b.ObservedSubagentsRunning(ctx, pair.chat, pair.native); err != nil || n != pair.want {
			t.Fatalf("count %s: %d %v", pair.native, n, err)
		}
	}
}
