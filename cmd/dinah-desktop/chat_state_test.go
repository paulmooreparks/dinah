package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func putChat(a *App, id string, initial sessionInitial) *Session {
	s := newSessionState(&sessionSeed{ID: id, Provider: "codex", Cwd: "C:/work"}, initial)
	a.mu.Lock()
	a.runtime.PutConversationLocked(id, s)
	a.mu.Unlock()
	return s
}

func chatStateOf(a *App, id string) ChatState {
	return a.chatState(context.Background(), nil, "", id, false, false)
}

// A chat's state has one place it is worked out: its life, whether it has a window, and, while it runs, what it is
// doing and why in words.
func TestAChatsExecutionStateAndItsReason(t *testing.T) {
	a, _ := providerTestApp(t)
	s := putChat(a, "chat-1", sessionInitial{Open: true})
	if st := chatStateOf(a, "chat-1"); st.Life != lifeOpen || !st.Window || st.Exec == nil || st.Exec.State != execIdle || st.Exec.Why != "No work in progress" {
		t.Fatalf("an open chat with nothing to do: %+v", st)
	}
	putChat(a, "chat-w", sessionInitial{Open: true, Working: true})
	if st := chatStateOf(a, "chat-w"); st.Exec == nil || st.Exec.State != execWorking || st.Exec.Why != "Turn in progress" {
		t.Fatalf("a chat running a turn: %+v", st)
	}
	// A full context is Waiting with the reason "Context full": the person has to choose, and nothing failed.
	s.SetHold(&sessionhost.ContextHold{Kind: "context", Used: 1, Size: 2, Actions: []string{sessionhost.HoldCompact}})
	if st := chatStateOf(a, "chat-1"); st.Exec.State != execWaiting || st.Exec.Why != "Context full" {
		t.Fatalf("a full context: %+v", st.Exec)
	}
	s.SetHold(nil)
	// A permission request waits for the person, even while the turn runs.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		a.permissions().Ask(ctx, "chat-w", nil, nil, []acp.PermissionOption{{OptionID: "allow", Name: "Allow", Kind: "allow_once"}})
		close(done)
	}()
	deadline := time.Now().Add(testWait)
	for !a.permissions().HasPending("chat-w") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if st := chatStateOf(a, "chat-w"); st.Exec.State != execWaiting || st.Exec.Why != "Needs your answer or approval" {
		t.Fatalf("a chat waiting for a permission: %+v", st.Exec)
	}
	cancel()
	<-done
	// A turn that ended in an error is Stuck, and outranks the rest.
	noteTurnResult("chat-1", errors.New("the agent gave up"))
	defer noteTurnResult("chat-1", nil)
	if st := chatStateOf(a, "chat-1"); st.Exec.State != execStuck || st.Exec.Why != "Last turn failed: the agent gave up" {
		t.Fatalf("a failed turn: %+v", st.Exec)
	}
	// A chat that does not run has no execution state; a retired chat says so.
	if st := chatStateOf(a, "no-such-chat"); st.Life != lifeClosed || st.Exec != nil || st.Window {
		t.Fatalf("a chat that is not open: %+v", st)
	}
	if st := a.chatState(context.Background(), nil, "", "chat-1", true, false); st.Life != lifeRetired || st.Exec != nil {
		t.Fatalf("a retired chat: %+v", st)
	}
	// A chat open in the background has no window, and is still open.
	bg := putChat(a, "chat-bg", sessionInitial{Open: true})
	bg.ConnectedBackground(time.Now())
	if st := chatStateOf(a, "chat-bg"); st.Life != lifeOpen || st.Window || st.Exec == nil {
		t.Fatalf("a chat open in the background: %+v", st)
	}
}

// A set of chats shows one state as a whole, chosen by the engine in the order Stuck, Waiting, Working, Idle; a set
// whose chats are all closed is Idle.
func TestCountsChooseTheStateAsAWholeInTheUrgencyOrder(t *testing.T) {
	exec := func(state string) ChatState {
		return ChatState{Life: lifeOpen, Window: true, Exec: &ChatExec{State: state, Why: "x"}}
	}
	closed := ChatState{Life: lifeClosed}
	retired := ChatState{Life: lifeRetired}
	for _, c := range []struct {
		states []ChatState
		top    string
		want   StateCounts
	}{
		{[]ChatState{exec(execIdle), exec(execWorking), exec(execWaiting), exec(execStuck), closed}, execStuck, StateCounts{Stuck: 1, Waiting: 1, Working: 1, Idle: 1, Closed: 1}},
		{[]ChatState{exec(execIdle), exec(execWorking), exec(execWaiting)}, execWaiting, StateCounts{Waiting: 1, Working: 1, Idle: 1}},
		{[]ChatState{exec(execIdle), exec(execWorking), exec(execWorking)}, execWorking, StateCounts{Working: 2, Idle: 1}},
		{[]ChatState{exec(execIdle), closed}, execIdle, StateCounts{Idle: 1, Closed: 1}},
		{[]ChatState{closed, retired}, execIdle, StateCounts{Closed: 2}},
		{nil, execIdle, StateCounts{}},
	} {
		got := countStates(c.states)
		c.want.Top = c.top
		if got != c.want {
			t.Errorf("counts of %v: %+v, want %+v", c.states, got, c.want)
		}
	}
}

// Groups, the directory and the team all serve the same fields, and a group carries its counts.
func TestEverySurfaceServesTheSameChatState(t *testing.T) {
	a, ws, chats := askTestApp(t)
	noteTurnResult("chat-B", errors.New("broke"))
	defer noteTurnResult("chat-B", nil)
	var room BusRoom
	err := a.busHost().With(func(b *bus.Bus) error {
		r, err := b.CreateGroup(context.Background(), bus.Sender{}, "Review", []string{"bus-chat-A", "bus-chat-B"})
		room = a.busRoom(b, r)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	byChat := map[string]BusGroupMember{}
	for _, m := range room.Members {
		byChat[m.Chat] = m
	}
	if m := byChat["chat-A"]; m.Life != lifeOpen || m.Exec == nil || m.Exec.State != execIdle {
		t.Fatalf("an idle member: %+v", m)
	}
	if m := byChat["chat-B"]; m.Exec == nil || m.Exec.State != execStuck {
		t.Fatalf("a stuck member: %+v", m)
	}
	if room.Counts == nil || room.Counts.Stuck != 1 || room.Counts.Idle != 1 || room.Counts.Top != execStuck {
		t.Fatalf("the group's counts: %+v", room.Counts)
	}
	fromDirectory := map[string]ChatState{}
	for _, e := range a.chatDirectory() {
		fromDirectory[e.SessionID] = e.ChatState
	}
	if st := fromDirectory["chat-B"]; st.Exec == nil || st.Exec.State != execStuck {
		t.Fatalf("the directory's stuck chat: %+v", st)
	}
	chats["B"].Close()
	if st := chatStateOf(a, "chat-B"); st.Life != lifeClosed || st.Exec != nil {
		t.Fatalf("a chat that was closed: %+v", st)
	}
	_ = ws
}

// The states are sent as one numbered event when something differs, a chat's information carries its own state,
// and nothing is sent when nothing changed.
func TestChatStatesAreSentOnceWhenTheyChange(t *testing.T) {
	a, _, chats := askTestApp(t)
	var views []ChatStatesView
	a.sink = func(name string, data any) {
		if name == sessionhost.EventChatState {
			views = append(views, data.(ChatStatesView))
		}
	}
	a.sendChatStates()
	a.sendChatStates()
	if len(views) != 1 || len(views[0].Chats) < 3 || views[0].StateGen == "" {
		t.Fatalf("the first pass sent %d events, and the second should send none: %+v", len(views), views)
	}
	info := chats["A"].Info()
	if info.State == nil || info.State.Life != lifeOpen || info.State.Exec == nil || info.State.Exec.State != execIdle {
		t.Fatalf("a chat's own information: %+v", info.State)
	}
	noteTurnResult("chat-A", errors.New("broke"))
	defer noteTurnResult("chat-A", nil)
	a.sendChatStates()
	if len(views) != 2 || views[1].StateVer <= views[0].StateVer {
		t.Fatalf("a change sent %d events: %+v", len(views), views)
	}
	if st := chats["A"].Info().State; st == nil || st.Exec.State != execStuck {
		t.Fatalf("a chat's own information after the change: %+v", st)
	}
	if got := a.ChatStates(); got.StateVer <= views[1].StateVer || len(got.Chats) != len(views[1].Chats) {
		t.Fatalf("the call for a page that missed an event: %+v", got)
	}
}

// testWait is how long a test waits for something that must happen. It is a failure bound, not a measurement, so
// it is far longer than a correct run needs and a loaded machine cannot reach it; a run that is right finishes as
// fast as ever.
const testWait = 30 * time.Second

// A reason that carries a detail is the prefix, a colon and the detail; without a detail it is the prefix alone.
func TestAReasonWithoutADetailIsItsPrefixAlone(t *testing.T) {
	if got := reasonWith("Switch failed", ""); got != "Switch failed" {
		t.Fatalf("%q", got)
	}
	if got := reasonWith("Switch failed", "no such model"); got != "Switch failed: no such model" {
		t.Fatalf("%q", got)
	}
}
