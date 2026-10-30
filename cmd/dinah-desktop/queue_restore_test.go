package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/sessionhost"
)

// restarted is the app as it is after a restart: a new one over the same state folder,
// with the agent's connection answering as before. The earlier app's queues are
// written first, as they would be by the time a person restarts it.
func restarted(t *testing.T, old *App) (*App, *testProviderConnection) {
	t.Helper()
	old.queueStore.Flush()
	a := NewApp()
	a.connectGraceOverride = -1
	a.settings.DefaultProvider = "codex"
	c := newTestProviderConnection()
	a.runtime.PutProviderLocked("codex", newTestProvider(c, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{LoadSession: true}}))
	t.Cleanup(a.closeAppletBridge)
	t.Cleanup(func() { a.busHost().CloseHandle() })
	a.judging.noAuto = true
	t.Cleanup(a.closeJudges)
	t.Cleanup(a.queueStore.Close)
	return a, c
}

func queuedChatWithHold(t *testing.T) (*App, *testProviderConnection, SessionInfo, string, []string) {
	t.Helper()
	a, c := providerTestApp(t)
	withContextModule(t, "codex")
	ws := t.TempDir()
	info, err := a.NewSessionAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	old := a.session(info.ID)
	a.transcriptCapture().RecordMessage(old, "you", "Earlier question", false, false)
	a.transcriptCapture().RecordMessage(old, "agent", "Earlier answer", false, false)
	a.transcriptCapture().FinishTurn(old, nil, "end_turn")
	c.window = fullWindow
	for i, text := range []string{"One", "Two", "Three"} {
		if err := a.Send(info.ID, text, nil, nil); err != nil {
			t.Fatal(err)
		}
		waitHeldQueue(t, old, i+1)
	}
	return a, c, info, ws, old.Info().Queued
}

// A restart picks the queue up again in order, and a full window holds the first as it
// did, with all of them behind it.
func TestARestartPicksUpTheQueueAndAFullWindowHoldsItAgain(t *testing.T) {
	a, _, info, ws, queued := queuedChatWithHold(t)
	b, c2 := restarted(t, a)
	withContextModule(t, "codex")
	c2.window = fullWindow
	if _, err := b.OpenSessionAt(ws, info.ID); err != nil {
		t.Fatal(err)
	}
	s := b.session(info.ID)
	hold := holdFor(t, s)
	if !slices.Equal(s.Info().Queued, queued) || len(hold.Actions) != 2 {
		t.Fatalf("after the restart: queue %v, want %v; hold %+v", s.Info().Queued, queued, hold)
	}
	if countCalls(c2, "session/prompt") != 0 {
		t.Fatal("a message was sent into a window that is full")
	}
}

// With room in the window the restored messages run on their own, in order.
func TestARestartRunsTheQueueInOrderWhenThereIsRoom(t *testing.T) {
	a, _, info, ws, queued := queuedChatWithHold(t)
	b, c2 := restarted(t, a)
	if _, err := b.OpenSessionAt(ws, info.ID); err != nil {
		t.Fatal(err)
	}
	waitForPrompts(t, c2, 3)
	got := prompts(c2)
	for i, want := range queued {
		if !strings.Contains(got[i], want) {
			t.Fatalf("prompt %d is %q, want %q (%q)", i, got[i], want, got)
		}
	}
}

// A closed chat's queue waits for the person to open it and nothing runs before.
func TestAClosedChatsQueueWaitsUntilItIsOpened(t *testing.T) {
	a, _, info, _, _ := queuedChatWithHold(t)
	b, c2 := restarted(t, a)
	b.queueRestorer().Sweep()
	if countCalls(c2, "session/new")+countCalls(c2, "session/load")+countCalls(c2, "session/prompt") != 0 {
		t.Fatalf("a closed chat's queue ran: %+v", c2.calls)
	}
	if kept, _ := b.queueStore.Load(info.ID); len(kept) != 3 {
		t.Fatalf("the queue of the closed chat is %d messages", len(kept))
	}
}

// The person clearing the queue clears the file, so a restart does not bring it back.
func TestClearingTheQueueClearsTheFile(t *testing.T) {
	a, _, info, ws, _ := queuedChatWithHold(t)
	a.Unqueue(info.ID)
	b, c2 := restarted(t, a)
	if _, err := b.OpenSessionAt(ws, info.ID); err != nil {
		t.Fatal(err)
	}
	if n := len(b.session(info.ID).Info().Queued); n != 0 || countCalls(c2, "session/prompt") != 0 {
		t.Fatalf("%d messages came back", n)
	}
}

// A compaction carries the queue to the new chat and leaves nothing kept for the earlier
// chat, so a restart does not bring the messages back a second time.
func TestACompactionLeavesNothingKeptForTheEarlierChat(t *testing.T) {
	a, c, info, _, _ := queuedChatWithHold(t)
	c.newIDs = []string{"codex:first", "codex:second"}
	watchConnects(a)
	if _, err := a.ResolveHold(info.ID, sessionhost.HoldCompact); err != nil {
		t.Fatal(err)
	}
	waitForPrompts(t, c, 1)
	a.queueStore.Flush()
	if kept, _ := a.queueStore.Load(info.ID); len(kept) != 0 {
		t.Fatalf("the earlier chat still keeps %d messages", len(kept))
	}
}

// When a queued message's turn starts in a chat, the chat's saved copy records the
// message's id, which is how a message carried there and run is known afterwards.
func TestAStartedQueuedMessagesIdIsKeptInTheSavedCopy(t *testing.T) {
	a, c, info, _, _ := queuedChatWithHold(t)
	a.queueStore.Flush()
	kept, err := a.queueStore.Load(info.ID)
	if err != nil || len(kept) != 3 {
		t.Fatalf("kept %v %v", kept, err)
	}
	c.newIDs = []string{"codex:first", "codex:second"}
	watchConnects(a)
	next, err := a.ResolveHold(info.ID, sessionhost.HoldCompact)
	if err != nil {
		t.Fatal(err)
	}
	waitForPrompts(t, c, 3)
	if next.ID != "codex:first" {
		t.Fatalf("the continuation is %s", next.ID)
	}
	var started []string
	for deadline := 0; deadline < 200; deadline++ {
		tr, err := readTranscript(next.ID)
		if err == nil && len(tr.StartedQueueIDs) == 3 {
			started = tr.StartedQueueIDs
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var want []string
	for _, sp := range kept {
		want = append(want, sp.ID)
	}
	if !slices.Equal(started, want) {
		t.Fatalf("the new chat's saved copy records %v, want %v", started, want)
	}
	// The earlier chat's file, left as a crash would have left it, does not bring them
	// back: after a restart its messages have all started, so none is kept again for the
	// new chat.
	a.queueStore.AppendFile(info.ID, kept)
	b, c2 := restarted(t, a)
	b.queueRestorer().Sweep()
	b.queueStore.Flush()
	if again, _ := b.queueStore.Load(next.ID); len(again) != 0 {
		t.Fatalf("%d started messages were kept again for the new chat", len(again))
	}
	if old, _ := b.queueStore.Load(info.ID); len(old) != 0 {
		t.Fatalf("the earlier chat's file remains with %d messages", len(old))
	}
	_ = c2
}
