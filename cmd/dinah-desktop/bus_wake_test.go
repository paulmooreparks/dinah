package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/bus"
)

// connectRequests collects the page's requests to connect a chat
// (chat:connect) and the turns that end.
type connectRequests struct {
	mu    sync.Mutex
	asked []map[string]any
	turns chan map[string]any
}

func watchConnects(a *App) *connectRequests {
	r := &connectRequests{turns: make(chan map[string]any, 16)}
	a.sink = func(name string, data any) {
		switch name {
		case "chat:connect":
			r.mu.Lock()
			r.asked = append(r.asked, data.(map[string]any))
			r.mu.Unlock()
		case "agent:turn":
			r.turns <- data.(map[string]any)
		}
	}
	return r
}

// take returns the requests made since the last call.
func (r *connectRequests) take() []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.asked
	r.asked = nil
	return out
}

// closedChat makes a chat that was open before a restart and is closed now,
// with a message from the person waiting for it. It returns the chat's
// session on the bus and its folder.
func closedChat(t *testing.T, a *App, id, title string) (string, string) {
	t.Helper()
	cwd := t.TempDir()
	busID := bus.WindowSessionPrefix + strings.ReplaceAll(id, ":", "-")
	a.saveBusSession(id, busID)
	err := a.busHost().With(func(b *bus.Bus) error {
		s, err := b.RegisterSession(context.Background(), busID, cwd, title)
		if err != nil {
			return err
		}
		if _, err := b.Send(context.Background(), bus.Sender{}, s.Address(), "A question for "+title, nil, 0); err != nil {
			return err
		}
		return b.EndSession(context.Background(), busID)
	})
	if err != nil {
		t.Fatal(err)
	}
	return busID, cwd
}

// wakeRound runs the watcher's look for chats to connect once.
func wakeRound(t *testing.T, a *App) {
	t.Helper()
	if err := a.busHost().With(func(b *bus.Bus) error { a.busCoordinator().ConnectWaking(context.Background(), b); return nil }); err != nil {
		t.Fatal(err)
	}
}

// openAsAsked opens each requested chat as the page does, through
// OpenSessionAt.
func openAsAsked(t *testing.T, a *App, asked []map[string]any) {
	t.Helper()
	for _, m := range asked {
		if _, err := a.OpenSessionAt(m["cwd"].(string), m["sessionId"].(string)); err != nil {
			t.Fatalf("opening %v: %v", m["sessionId"], err)
		}
	}
}

func askedIDs(asked []map[string]any) []string {
	var out []string
	for _, m := range asked {
		out = append(out, m["sessionId"].(string))
	}
	return out
}

func TestAWakingMessageOpensAClosedChatInTheBackground(t *testing.T) {
	a, c := providerTestApp(t)
	r := watchConnects(a)
	busID, cwd := closedChat(t, a, "codex:closed", "Parser Release")
	wakeRound(t, a)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a chat was connected before the page said which chats have windows: %v", got)
	}
	a.ChatWindows(nil)
	wakeRound(t, a)
	asked := r.take()
	if len(asked) != 1 || asked[0]["sessionId"] != "codex:closed" || asked[0]["cwd"] != cwd || asked[0]["background"] != true {
		t.Fatalf("the closed chat was not opened in the background: %v", asked)
	}
	wakeRound(t, a)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a chat being opened was asked for again: %v", got)
	}
	openAsAsked(t, a, asked)
	s := a.session("codex:closed")
	if info := s.Info(); !info.InBackground || !info.Open {
		t.Fatalf("the chat is not open in the background: %+v", info)
	}
	if state := a.busChatState("codex:closed", true); state != "background" {
		t.Fatalf("the chat's state is %q, not background", state)
	}
	ring(t, a)
	waitTurn(t, r.turns)
	if got := prompts(c); len(got) != 1 || !strings.Contains(got[0], "A question for Parser Release") {
		t.Fatalf("the waiting message was not delivered: %q", got)
	}
	if n := waiting(t, a, busID); n != 0 {
		t.Fatalf("%d messages still wait", n)
	}
	// Opening its window makes it an ordinary open chat.
	a.ChatWindows([]string{"codex:closed"})
	if s.Info().InBackground || a.busChatState("codex:closed", true) != "open" {
		t.Fatal("a chat whose window opened is still shown open in the background")
	}
}

func TestAtMostThreeChatsOpenInTheBackground(t *testing.T) {
	a, _ := providerTestApp(t)
	r := watchConnects(a)
	ids := []string{"codex:one", "codex:two", "codex:three", "codex:four", "codex:five"}
	for _, id := range ids {
		closedChat(t, a, id, "Chat "+id)
	}
	a.ChatWindows(nil)
	wakeRound(t, a)
	asked := r.take()
	if got := askedIDs(asked); strings.Join(got, ",") != "codex:one,codex:two,codex:three" {
		t.Fatalf("the first three chats were not opened in order: %v", got)
	}
	openAsAsked(t, a, asked)
	wakeRound(t, a)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a fourth chat was opened in the background: %v", askedIDs(got))
	}
	// A chat that closes itself for being idle frees its slot.
	now := time.Now()
	a.wakeNow = func() time.Time { return now.Add(backgroundIdle + time.Minute) }

	a.session("codex:one").SetIdleSince(now)

	a.session("codex:two").SetIdleSince(now.Add(backgroundIdle))

	a.session("codex:three").SetIdleSince(now.Add(backgroundIdle))

	wakeRound(t, a)
	asked = r.take()
	if got := askedIDs(asked); strings.Join(got, ",") != "codex:four" {
		t.Fatalf("the queued chat did not open when a slot freed: %v", got)
	}
	// A chat that fails to open frees its slot too, and the next opens.
	a.busCoordinator().Connected("codex:four", context.DeadlineExceeded)
	wakeRound(t, a)
	if got := askedIDs(r.take()); strings.Join(got, ",") != "codex:five" {
		t.Fatalf("the last chat did not open when a failed one freed its slot: %v", got)
	}
}

func TestABackgroundChatClosesItselfWhenIdle(t *testing.T) {
	a, c := providerTestApp(t)
	r := watchConnects(a)
	busID, _ := closedChat(t, a, "codex:idle", "Idle Chat")
	var group string
	err := a.busHost().With(func(b *bus.Bus) error {
		g, err := b.CreateGroup(context.Background(), bus.Sender{}, "Release Team", []string{busID})
		group = g.Name
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
	s := a.session("codex:idle")
	// Twenty-nine minutes after its turn it is still open.
	ended := s.Info()
	last := s.IdleSince()
	if !ended.InBackground || last.IsZero() {
		t.Fatalf("the chat's turn end was not recorded: %+v", ended)
	}
	a.wakeNow = func() time.Time { return last.Add(backgroundIdle - time.Minute) }
	wakeRound(t, a)
	if !s.Info().InBackground {
		t.Fatal("the chat closed before it had been idle for thirty minutes")
	}
	a.wakeNow = func() time.Time { return last.Add(backgroundIdle) }
	wakeRound(t, a)
	info := s.Info()
	if info.InBackground || !info.Parked || a.busChatState("codex:idle", false) != "closed" {
		t.Fatalf("the idle chat did not close itself: %+v", info)
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		me, err := b.Session(context.Background(), busID)
		if err != nil {
			return err
		}
		if !me.Ended {
			t.Error("the closed chat is still open on the bus")
		}
		members, err := b.RoomMembers(context.Background(), group)
		if err != nil {
			return err
		}
		found := false
		for _, m := range members {
			found = found || m.Key == busID
		}
		if !found {
			t.Error("closing the chat took it out of its group")
		}
		// A mention in the group wakes it again.
		_, err = b.SendMentioning(context.Background(), bus.Sender{}, "#Release Team", "Are we ready?", nil, 0, []string{busID})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	wakeRound(t, a)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a chat closed in the background was opened by the page again: %v", got)
	}
	if info := s.Info(); !info.InBackground || info.Parked {
		t.Fatalf("the woken chat is not open in the background again: %+v", info)
	}
	ring(t, a)
	waitTurn(t, r.turns)
	if got := prompts(c); len(got) != 2 || !strings.Contains(got[1], "Are we ready?") {
		t.Fatalf("the mention was not delivered: %q", got)
	}
	// It came back without the agent loading it a second time.
	loads := 0
	for _, call := range c.calls {
		if call.method == "session/load" {
			loads++
		}
	}
	if loads != 1 {
		t.Fatalf("the agent loaded the chat %d times", loads)
	}
}

func TestARestoredChatWaitingToReconnectIsReconnected(t *testing.T) {
	a, c := providerTestApp(t)
	r := watchConnects(a)
	closedChat(t, a, "codex:restored", "Restored Chat")
	// The window was restored with the layout and waits at the prompt to
	// reconnect.
	a.ChatWindows([]string{"codex:restored"})
	wakeRound(t, a)
	asked := r.take()
	if len(asked) != 1 || asked[0]["sessionId"] != "codex:restored" || asked[0]["background"] != false {
		t.Fatalf("the restored chat was not reconnected in its window: %v", asked)
	}
	openAsAsked(t, a, asked)
	if a.session("codex:restored").Info().InBackground {
		t.Fatal("a chat with a window was opened in the background")
	}
	ring(t, a)
	waitTurn(t, r.turns)
	if got := prompts(c); len(got) != 1 || !strings.Contains(got[0], "A question for Restored Chat") {
		t.Fatalf("the waiting message was not delivered: %q", got)
	}
	// A chat with a window takes no background slot.
	a.mu.Lock()
	used := a.backgroundUsedLocked()
	a.mu.Unlock()
	if used != 0 {
		t.Fatalf("a reconnected window takes %d background slots", used)
	}
}

func TestARestartPokesChatsWithWakingMessages(t *testing.T) {
	a, _ := providerTestApp(t)
	closedChat(t, a, "codex:before", "Before the Restart")
	// The window restarts: a new App over the same state.
	again := NewApp()
	t.Cleanup(func() { again.busHost().CloseHandle() })
	again.settings.DefaultProvider = "codex"
	for _, id := range a.runtime.ProviderIDsLocked() {
		again.runtime.PutProviderLocked(id, a.runtime.ProviderLocked(id))
	}
	r := watchConnects(again)
	again.ChatWindows(nil)
	wakeRound(t, again)
	if got := askedIDs(r.take()); strings.Join(got, ",") != "codex:before" {
		t.Fatalf("the restart did not poke the chat with a message waiting: %v", got)
	}
}

func TestAFailedReconnectKeepsTheMessageWaiting(t *testing.T) {
	a, _ := providerTestApp(t)
	r := watchConnects(a)
	busID, _ := closedChat(t, a, "codex:broken", "Broken Chat")
	{
		initial := a.runtime.ProviderLocked("codex").Initialization()
		initial.AgentCapabilities.LoadSession = false
		setTestInitialization(a, "codex", initial)
	}
	a.ChatWindows([]string{"codex:broken"})
	wakeRound(t, a)
	asked := r.take()
	if len(asked) != 1 {
		t.Fatalf("the chat was not asked for: %v", asked)
	}
	if _, err := a.OpenSessionAt(asked[0]["cwd"].(string), "codex:broken"); err == nil {
		t.Fatal("the agent opened a chat it cannot reopen")
	}
	if n := waiting(t, a, busID); n != 1 {
		t.Fatalf("%d messages wait after the failure, not 1", n)
	}
	wakeRound(t, a)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a chat that failed was asked for again without a newer message: %v", got)
	}
	err := a.busHost().With(func(b *bus.Bus) error {
		if _, err := b.RegisterSession(context.Background(), busID, asked[0]["cwd"].(string), "Broken Chat"); err != nil {
			return err
		}
		if _, err := b.Send(context.Background(), bus.Sender{}, "Broken Chat", "Try again?", nil, 0); err != nil {
			return err
		}
		return b.EndSession(context.Background(), busID)
	})
	if err != nil {
		t.Fatal(err)
	}
	wakeRound(t, a)
	if got := r.take(); len(got) != 1 {
		t.Fatalf("a newer message did not ask for the chat again: %v", got)
	}
}

// Everyone wakes every member, so the window opens a closed member in the
// background for it.
func TestEveryoneOpensAClosedChat(t *testing.T) {
	a, _ := providerTestApp(t)
	r := watchConnects(a)
	busID, _ := closedChat(t, a, "codex:member", "Quiet Member")
	err := a.busHost().With(func(b *bus.Bus) error {
		// The message sent to it directly is taken, as if it had been read.
		if _, _, err := b.Deliver(context.Background(), busID, "tool"); err != nil {
			return err
		}
		if err := b.EndSession(context.Background(), busID); err != nil {
			return err
		}
		// A chat says it.
		speaker, err := b.RegisterSession(context.Background(), bus.WindowSessionPrefix+"speaker", t.TempDir(), "Speaker")
		if err != nil {
			return err
		}
		if _, err := b.CreateGroup(context.Background(), bus.Sender{}, "Everyone Team", []string{busID, speaker.ID}); err != nil {
			return err
		}
		_, err = b.SendMentioning(context.Background(), bus.Sender{Session: &speaker}, "#Everyone Team", "Hello, everyone", nil, 0, []string{bus.Everyone})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// The window starts more than a week after the group's last message, so
	// the pass that opens group chats at start leaves it alone and only the
	// mention could open the chat.
	a.wakeNow = func() time.Time { return time.Now().Add(8 * 24 * time.Hour) }
	a.ChatWindows(nil)
	wakeRound(t, a)
	if got := askedIDs(r.take()); len(got) != 1 || got[0] != "codex:member" {
		t.Fatalf("everyone did not open the closed member: %v", got)
	}
}

// Admission, connection completion and window changes share App.mu for the
// scheduler/session transaction, so releasing a reservation cannot free a slot
// before its session has been counted.
func TestWakeAdmissionWithConcurrentCompletionAndWindowChanges(t *testing.T) {
	a, _ := providerTestApp(t)
	a.sink = func(string, any) {}
	now := time.Now()
	paths := map[string]string{}
	a.mu.Lock()
	a.runtime.SetWindowsLocked(nil)
	a.runtime.ExpireWakeLocked(now)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("codex:wake-%d", i)
		paths[id] = "folder"
		a.runtime.ConsiderWakeLocked(id, "folder", 1, false, now)
		a.runtime.PutConversationLocked(id, newSessionState(&sessionSeed{ID: id, Provider: "codex", Cwd: "folder"}, sessionInitial{Open: true}))
	}
	a.mu.Unlock()
	a.busCoordinator().OpenQueued(paths)
	check := func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if n := a.backgroundUsedLocked(); n > backgroundSlots {
			t.Errorf("combined slot bound exceeded: %d", n)
		}
	}
	var work sync.WaitGroup
	for i := 0; i < 30; i++ {
		work.Add(3)
		id := fmt.Sprintf("codex:wake-%d", i)
		go func() { defer work.Done(); a.busCoordinator().Connected(id, nil); check() }()
		go func() { defer work.Done(); a.ChatWindows([]string{id}); check() }()
		go func() { defer work.Done(); a.busCoordinator().OpenQueued(paths); check() }()
	}
	work.Wait()
	check()
}
