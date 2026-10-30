package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
)

// busChat opens a chat in a provider test app and returns it with its
// session on the bus. turns receives each turn's end.
func busChat(t *testing.T) (*App, *testProviderConnection, *Session, bus.Session, chan map[string]any) {
	t.Helper()
	a, c := providerTestApp(t)
	turns := make(chan map[string]any, 8)
	a.sink = func(name string, data any) {
		if name == "agent:turn" {
			turns <- data.(map[string]any)
		}
	}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	var me bus.Session
	err = a.busHost().With(func(b *bus.Bus) error {
		var err error
		me, err = b.Session(context.Background(), s.BusID())
		return err
	})
	if err != nil {
		t.Fatalf("the chat is not on the bus: %v", err)
	}
	return a, c, s, me, turns
}

// sendTo sends text to a bus session as Paul, by its title and workspace.
func sendTo(t *testing.T, a *App, to bus.Session, text string) {
	t.Helper()
	err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.Send(context.Background(), bus.Sender{}, to.Address(), text, nil, 0)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ring runs one round of the watcher over every open chat.
func ring(t *testing.T, a *App) {
	t.Helper()
	err := a.busHost().With(func(b *bus.Bus) error {
		a.mu.Lock()
		open := make([]*Session, 0, a.runtime.ConversationCountLocked())
		for _, s := range a.runtime.ConversationsLocked() {
			open = append(open, s)
		}
		a.mu.Unlock()
		for _, s := range open {
			a.busCoordinator().Ring(context.Background(), b, s)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func waitTurn(t *testing.T, turns chan map[string]any) {
	t.Helper()
	select {
	case <-turns:
	case <-time.After(testWait):
		t.Fatal("the turn did not end")
	}
}

// prompts returns the text of each prompt the agent was given.
func prompts(c *testProviderConnection) []string {
	var out []string
	for _, call := range c.calls {
		if call.method != "session/prompt" {
			continue
		}
		var b strings.Builder
		for _, block := range call.params.(acp.PromptParams).Prompt {
			b.WriteString(block.Text)
			b.WriteString("\n")
		}
		out = append(out, b.String())
	}
	return out
}

func waiting(t *testing.T, a *App, id string) int {
	t.Helper()
	var n int
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		n, _, err = b.Doorbell(context.Background(), id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAnIdleChatIsWokenByAMessage(t *testing.T) {
	a, c, s, me, turns := busChat(t)
	sendTo(t, a, me, "Is the parser release ready?")
	ring(t, a)
	waitTurn(t, turns)
	got := prompts(c)
	if len(got) != 1 || !strings.Contains(got[0], "Is the parser release ready?") || !strings.Contains(got[0], "Dinah bus: 1 new message.") {
		t.Fatalf("the chat was not given the message: %q", got)
	}
	if n := waiting(t, a, s.BusID()); n != 0 {
		t.Fatalf("%d messages still wait after delivery", n)
	}
	ring(t, a)
	if len(prompts(c)) != 1 {
		t.Fatal("a delivered message started a second turn")
	}
}

func TestAWorkingChatIsNotInterrupted(t *testing.T) {
	a, c, s, me, _ := busChat(t)
	s.StartWork()
	sendTo(t, a, me, "Later, please.")
	ring(t, a)
	if len(prompts(c)) != 0 {
		t.Fatal("a working chat was given a prompt")
	}
	if n := waiting(t, a, s.BusID()); n != 1 {
		t.Fatalf("the message should still wait, but %d do", n)
	}
}

func TestThePersonsPromptCarriesWaitingMessages(t *testing.T) {
	a, c, s, me, turns := busChat(t)
	sendTo(t, a, me, "The roadmap moved.")
	if err := a.Send(s.ID(), "What changed today?", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	got := prompts(c)
	if len(got) != 1 || !strings.Contains(got[0], "What changed today?") || !strings.Contains(got[0], "The roadmap moved.") {
		t.Fatalf("the person's prompt did not carry the waiting message: %q", got)
	}
	if n := waiting(t, a, s.BusID()); n != 0 {
		t.Fatalf("%d messages still wait", n)
	}
}

// The Bus pane sees a chat's answer in Paul's inbox, opens its thread from
// the answer, and Paul's reply in the thread reaches the chat.
func TestThePaneShowsAndAnswersAThread(t *testing.T) {
	a, _, s, me, _ := busChat(t)
	ask, err := a.BusSend(me.Address(), "Is the parser release ready?", 0)
	if err != nil {
		t.Fatal(err)
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		_, err := b.Send(context.Background(), bus.Sender{Session: &me}, "", "Nearly.", nil, ask.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	o, err := a.BusOverview()
	if err != nil || len(o.Inbox) != 1 || o.Unread != 1 || o.Inbox[0].Text != "Nearly." {
		t.Fatalf("the inbox is wrong: %+v %v", o, err)
	}
	if len(o.Workspaces) != 1 || len(o.Workspaces[0].Chats) != 1 || o.Workspaces[0].Chats[0].Title != me.Display() || o.Workspaces[0].Chats[0].Address != me.Address() {
		t.Fatalf("who is on the bus is wrong: %+v", o.Workspaces)
	}
	if o.Inbox[0].From != me.Display() || o.Inbox[0].FromSession != me.ID || o.Inbox[0].FromPerson {
		t.Fatalf("the answer is from %q (%q)", o.Inbox[0].From, o.Inbox[0].FromSession)
	}
	thread, err := a.BusThread(o.Inbox[0].ID)
	if err != nil || len(thread) != 2 || thread[0].ID != ask.ID {
		t.Fatalf("the thread is wrong: %+v %v", thread, err)
	}
	reply, err := a.BusReplyInThread(o.Inbox[0].ID, "Tell me when it is.")
	if err != nil || reply.To != me.Display() || reply.ReplyTo != ask.ID {
		t.Fatalf("the reply is wrong: %+v %v", reply, err)
	}
	if err := a.MarkBusInboxRead(); err != nil {
		t.Fatal(err)
	}
	if o, _ := a.BusOverview(); o.Unread != 0 {
		t.Fatalf("%d messages are still unread", o.Unread)
	}
	if n := waiting(t, a, s.BusID()); n != 2 {
		t.Fatalf("the chat should have the question and the reply waiting, but has %d", n)
	}
}

// The picker's chats make a group, an open chat by its session on the bus
// and a closed one by a session the bus is given for it, which the chat
// keeps when it opens. The group's window sees each member's state, the
// person's message with its mentions, and the person's unread count, and a
// closed chat renamed in the window is renamed in the group.
func TestThePickerStartsAGroupOfOpenAndClosedChats(t *testing.T) {
	a, _, s, me, _ := busChat(t)
	if _, err := a.RenameSession(s.ID(), "Lead"); err != nil {
		t.Fatal(err)
	}
	closedDir := t.TempDir()
	g, err := a.BusStartGroup("", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Lead"}, {ID: "closed-chat", Cwd: closedDir, Title: "Timesheet Applet"}})
	if err != nil {
		t.Fatal(err)
	}
	closedBus := a.busSessionID("closed-chat")
	if g.Title != "Lead and Timesheet Applet" || len(g.Members) != 3 || !strings.HasPrefix(closedBus, bus.WindowSessionPrefix) {
		t.Fatalf("the group is %+v; the closed chat is %s on the bus", g, closedBus)
	}
	states := []string{}
	for _, m := range g.Members {
		states = append(states, m.Title+"="+m.State+"@"+m.Chat)
	}
	if strings.Join(states, ",") != "Lead=open@"+s.ID()+",Timesheet Applet=closed@closed-chat,"+speakerName(bus.Human)+"=person@" {
		t.Fatalf("the members are %v", states)
	}
	sent, err := a.BusSendToGroup(g.Name, "Does the grid render?", []string{closedBus}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sent.FromPerson || len(sent.Mentions) != 1 || sent.Mentions[0].Title != "Timesheet Applet" || sent.RoomTitle != g.Title {
		t.Fatalf("the person's message: %+v", sent)
	}
	if n := waiting(t, a, closedBus); n != 1 {
		t.Fatalf("the message does not wait for the closed chat: %d", n)
	}
	if _, err := a.BusSendToGroup(g.Name, "Lead?", []string{"window-nobody"}, nil); err == nil {
		t.Fatal("a mention of a chat outside the group was sent")
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		_, err := b.Send(context.Background(), bus.Sender{Session: &me}, "#"+g.Name, "It renders.", nil, 0)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if o, _ := a.BusOverview(); len(o.Rooms) != 1 || o.Rooms[0].Unread != 1 || o.Unread != 0 {
		t.Fatalf("the overview's groups: %+v, inbox unread %d", o.Rooms, o.Unread)
	}
	if err := a.BusMarkGroupRead(g.Name); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.BusGroup(g.Name); got.Unread != 0 {
		t.Fatalf("the group still has %d unread", got.Unread)
	}
	if _, err := a.RenameSession("closed-chat", "Timesheet Grid"); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.BusGroup(g.Title); got.Members[1].Title != "Timesheet Grid" {
		t.Fatalf("the renamed closed chat is %q in the group", got.Members[1].Title)
	}
	if err := a.BusRemoveFromGroup(g.Name, closedBus); err != nil {
		t.Fatal(err)
	}
	msgs, _ := a.BusGroupMessages(g.Name)
	if last := msgs[len(msgs)-1]; last.Kind != bus.MemberKind || !strings.HasSuffix(last.Line, " removed Timesheet Grid from the group.") {
		t.Fatalf("the last line is %+v", last)
	}
	if err := a.BusDeleteGroup(g.Name); err != nil {
		t.Fatal(err)
	}
	if o, _ := a.BusOverview(); len(o.Rooms) != 0 {
		t.Fatal("the group was not deleted")
	}
}

func TestBusWorkspaceLabelsUseExecutionPaths(t *testing.T) {
	a, _ := providerTestApp(t)
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "one", Name: "First", ExecutionPath: "folder"}, {ID: "two", Name: "Second", ExecutionPath: "folder"}, {ID: "empty", Name: "Empty"}}
	labels := a.busWorkspaceLabels()
	if len(labels) != 1 || labels["folder"] != "First" {
		t.Fatal(labels)
	}
}

// A chat is on the bus by the title the person sees: untitled in its
// workspace's name at first, then its agent's title, then the name the
// person gives it, each at once.
func TestAChatsTitleReachesTheBus(t *testing.T) {
	a, _ := providerTestApp(t)
	folder := filepath.Join(t.TempDir(), "parser")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "w1", Name: "Parser Library", ExecutionPath: folder}}
	info, err := a.NewSessionAt(folder)
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	onBus := func() bus.Session {
		t.Helper()
		var me bus.Session
		if err := a.busHost().With(func(b *bus.Bus) error {
			var err error
			me, err = b.Session(context.Background(), s.BusID())
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return me
	}
	if got := onBus().Display(); got != "Untitled Chat in Parser Library" {
		t.Fatalf("a new chat is on the bus as %q", got)
	}
	a.newAgentHandler().Notify("session/update", json.RawMessage(`{"sessionId":"`+s.ID()+`","update":{"sessionUpdate":"session_info_update","title":"Release Notes"}}`))
	if got := onBus().Display(); got != "Release Notes" {
		t.Fatalf("after the agent titled it, the chat is on the bus as %q", got)
	}
	if _, err := a.RenameSession(s.ID(), "Parser 2.4 Release"); err != nil {
		t.Fatal(err)
	}
	if got := onBus(); got.Display() != "Parser 2.4 Release" || got.Address() != "Parser 2.4 Release (parser)" {
		t.Fatalf("after the person renamed it, the chat is on the bus as %q", got.Address())
	}
	sendTo(t, a, onBus(), "Is it tagged?")
	if n := waiting(t, a, s.BusID()); n != 1 {
		t.Fatalf("a message by the new title did not reach the chat: %d wait", n)
	}
	if _, err := a.RenameSession(s.ID(), ""); err != nil {
		t.Fatal(err)
	}
	if got := onBus().Display(); got != "Release Notes" {
		t.Fatalf("with the person's name taken off, the chat is on the bus as %q", got)
	}
}

// An Editor selection goes with a message once; when the next message
// carries the same one, it is left out, and a new one goes again.
func TestAnUnchangedSelectionIsLeftOut(t *testing.T) {
	a, c, s, _, turns := busChat(t)
	doc := &DocRef{DraftID: "0123456789abcdef0123456789abcdef", Title: "Untitled", Selection: "C:\\Users\\paul\\left\\selected"}
	send := func(text string, d *DocRef) string {
		t.Helper()
		if err := a.Send(s.ID(), text, d, nil); err != nil {
			t.Fatal(err)
		}
		waitTurn(t, turns)
		got := prompts(c)
		return got[len(got)-1]
	}
	if p := send("First.", doc); !strings.Contains(p, "left\\selected") {
		t.Fatalf("the first message lost its selection: %q", p)
	}
	if p := send("Second.", doc); strings.Contains(p, "left\\selected") || strings.Contains(p, "selected this passage") {
		t.Fatalf("an unchanged selection was sent again: %q", p)
	}
	// A prompt the app makes, such as a steering message already weighed or
	// a bus delivery, leaves the record of the person's selection alone.
	if _, err := a.promptFrom(s, "A prompt the app made.", nil); err != nil {
		t.Fatal(err)
	}
	if p := send("Again.", doc); strings.Contains(p, "left\\selected") {
		t.Fatalf("a prompt the app made let an unchanged selection go again: %q", p)
	}
	changed := *doc
	changed.Selection = "A new passage."
	if p := send("Third.", &changed); !strings.Contains(p, "A new passage.") {
		t.Fatalf("a changed selection was left out: %q", p)
	}
}

// A Claude chat whose hooks have reached it is steered without stopping
// its turn: the steer waits on the bus, the page is told it is pending, then
// delivered when a hook takes it; one no hook takes goes as the next
// message when the turn ends. A chat the hooks have not reached is stopped
// and steered as before.
func TestSteeringAClaudeChatKeepsItsTurn(t *testing.T) {
	a, c, s, me, _ := busChat(t)
	var states []string
	sink := a.sink
	a.sink = func(name string, data any) {
		if name == "agent:steer" {
			states = append(states, data.(map[string]any)["state"].(string))
		}
		sink(name, data)
	}

	s = replaceTestIdentity(a, s, "Provider", "claude")
	s.StartWork()

	a.runtime.PutProviderLocked("claude", a.runtime.ProviderLocked("codex"))
	cancels := func() int {
		n := 0
		for _, call := range c.calls {
			if call.method == "session/cancel" {
				n++
			}
		}
		return n
	}
	if a.turnIO().HooksReach(s) {
		t.Fatal("a chat no hook has seen was taken as reached")
	}
	err := a.busHost().With(func(b *bus.Bus) error { return b.BindAgentSession(context.Background(), "claude-own-id", me.ID) })
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Steer(s.ID(), "Use port 8081.", nil, nil); err != nil {
		t.Fatal(err)
	}
	if cancels() != 0 {
		t.Fatal("steering a reached Claude chat stopped its turn")
	}
	a.busHost().With(func(b *bus.Bus) error {
		got, _ := b.TakeSteers(context.Background(), me.ID)
		if len(got) != 1 || !strings.Contains(got[0].Text, "Use port 8081.") {
			t.Fatalf("the hook took %+v", got)
		}
		a.turnIO().NoticeSteers(b.SteerDelivered, s)
		return nil
	})
	if err := a.Steer(s.ID(), "And skip the docs.", nil, nil); err != nil {
		t.Fatal(err)
	}
	late := a.turnIO().SettleSteers(s)
	if len(late) != 1 || !late[0].Late() || !strings.Contains(promptText(&late[0]), "And skip the docs.") {
		t.Fatalf("the steer no hook took was not sent next: %+v", late)
	}
	if strings.Join(states, ",") != "pending,delivered,pending,queued" {
		t.Fatalf("the page was told %v", states)
	}
	if err := a.StopAndSteer(s.ID(), "Drop it.", nil, nil); err != nil {
		t.Fatal(err)
	}
	if cancels() != 1 {
		t.Fatal("Stop and Steer did not stop the turn")
	}
}

func TestAReopenedChatKeepsItsPlaceOnTheBus(t *testing.T) {
	a, _, s, me, _ := busChat(t)
	a.sessionsLifecycle().Forget(s.ID())
	var ended bus.Session
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		ended, err = b.Session(context.Background(), me.ID)
		return err
	})
	if err != nil || !ended.Ended {
		t.Fatalf("a forgotten chat is still live on the bus: %+v %v", ended, err)
	}
	if got := a.busSessionID(s.ID()); got != me.ID {
		t.Fatalf("the chat's bus session changed from %s to %s", me.ID, got)
	}
}

func TestBusyBusDeliveryUsesSteeringCapability(t *testing.T) {
	for _, mode := range []string{"accepted", "pending", "unsupported", "uncertain"} {
		t.Run(mode, func(t *testing.T) {
			a, base, s, me, _ := busChat(t)
			s.StartWork()
			count := 0
			setTestConnection(a, "codex", &steeringTestConnection{testProviderConnection: base, unsupported: mode == "unsupported", receive: func(p acp.PromptParams, delivered func()) error {
				count++
				if !strings.Contains(p.Prompt[0].Text, "Incoming chat message") {
					t.Fatal("missing bus content")
				}
				if mode == "accepted" {
					delivered()
				}
				if mode == "uncertain" {
					return errors.New("uncertain delivery")
				}
				return nil
			}})
			sendTo(t, a, me, "Incoming chat message")
			ring(t, a)
			want := 1
			if mode == "unsupported" {
				want = 0
			}
			if count != want {
				t.Fatalf("%d steering calls, want %d", count, want)
			}
			if mode == "unsupported" {
				if waiting(t, a, me.ID) != 1 {
					t.Fatal("unsupported busy chat consumed bus message")
				}
			} else if waiting(t, a, me.ID) != 0 {
				t.Fatal("steered bus message still waiting")
			}
			late := a.turnIO().SettleSteers(s)
			if mode == "pending" {
				if len(late) != 1 || late[0].Label() != busChatLabel {
					t.Fatalf("fallback lost attribution: %+v", late)
				}
			} else if len(late) != 0 {
				t.Fatal("delivered or uncertain message was replayed")
			}
			for _, call := range base.calls {
				if call.method == "session/cancel" || call.method == "session/prompt" {
					t.Fatal("busy delivery started or cancelled a turn")
				}
			}
		})
	}
}
