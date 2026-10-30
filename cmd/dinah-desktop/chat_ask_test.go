package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"studio/internal/applet"
	"studio/internal/bus"
)

// savedChat makes a chat that is closed, which Dinah knows by its saved
// copy in the folder cwd.
func savedChat(t *testing.T, id, cwd string) {
	t.Helper()
	if err := writeTranscript(&conversationTranscript{SessionID: id, Provider: "codex", Workspace: cwd, Messages: []ConversationMessage{}}); err != nil {
		t.Fatal(err)
	}
}

type askResult struct {
	out map[string]any
	err error
}

// askInBackground asks as askWithContext does, without waiting.
func askInBackground(t *testing.T, ctx context.Context, a *App, ws, caller, target, question string) chan askResult {
	done := make(chan askResult, 1)
	go func() {
		out, err := askWithContext(t, ctx, a, ws, caller, target, question)
		done <- askResult{out, err}
	}()
	return done
}

// connectRequested runs the watcher's look for chats to wake until the page
// is asked to connect one, and returns the requests.
func connectRequested(t *testing.T, a *App, r *connectRequests) []map[string]any {
	t.Helper()
	for deadline := time.Now().Add(testWait); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		wakeRound(t, a)
		if got := r.take(); len(got) > 0 {
			return got
		}
	}
	t.Fatal("no chat was connected for the question")
	return nil
}

func answer(t *testing.T, done chan askResult) askResult {
	t.Helper()
	select {
	case r := <-done:
		return r
	case <-time.After(testWait):
		t.Fatal("the question was never answered")
	}
	return askResult{}
}

func waitingQuestions(t *testing.T, a *App) []int64 {
	t.Helper()
	var ids []int64
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		ids, err = b.WaitingQuestions(context.Background())
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

// closedAskApp is askTestApp with a second workspace, Parser, in which the
// chat codex:closed is closed.
func closedAskApp(t *testing.T) (*App, string, map[string]*Session, string, *connectRequests) {
	t.Helper()
	a, ws, chats := askTestApp(t)
	r := watchConnects(a)
	other := t.TempDir()
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "studio", Name: "Studio", ExecutionPath: ws}, {ID: "parser", Name: "Parser", ExecutionPath: other}}
	a.settings.SessionTitles = map[string]string{"codex:closed": "Parser Release"}
	savedChat(t, "codex:closed", other)
	return a, ws, chats, other, r
}

// A question to an open chat goes over the bus from the asking chat's
// session to the asked chat's, and is taken by the turn that answers it.
func TestAskTravelsOverTheBus(t *testing.T) {
	a, ws, _ := askTestApp(t)
	out, err := askFrom(t, a, ws, "chat-A", "chat-B", "Which parser files changed?")
	if err != nil || out["reply"] != "Answer to: Which parser files changed?" {
		t.Fatalf("the open chat answered %v %v", out, err)
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		thread, err := b.Recent(context.Background(), 10)
		if err != nil {
			return err
		}
		var q *bus.Message
		for i := range thread {
			if thread[i].Kind == bus.QuestionKind {
				q = &thread[i]
			}
		}
		if q == nil || q.FromSession != "bus-chat-A" || q.To != "Chat B" || q.Text != "Which parser files changed?" {
			t.Errorf("the question did not travel over the bus: %+v", thread)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ids := waitingQuestions(t, a); len(ids) != 0 {
		t.Fatalf("answered questions still wait: %v", ids)
	}
}

// A closed chat is opened in the background for a question, answers it, and
// the answer comes back to the asking chat.
func TestAskOpensAClosedChatInTheBackground(t *testing.T) {
	a, ws, _, other, r := closedAskApp(t)
	a.ChatWindows(nil)
	done := askInBackground(t, context.Background(), a, ws, "chat-A", "codex:closed", "Is the parser released?")
	asked := connectRequested(t, a, r)
	if len(asked) != 1 || asked[0]["sessionId"] != "codex:closed" || asked[0]["cwd"] != other || asked[0]["background"] != true {
		t.Fatalf("the closed chat was not opened in the background: %v", asked)
	}
	openAsAsked(t, a, asked)
	res := answer(t, done)
	if res.err != nil || res.out["reply"] != "Answer to: Is the parser released?" || res.out["title"] != "Parser Release" || res.out["sessionId"] != "codex:closed" {
		t.Fatalf("the closed chat answered %v %v", res.out, res.err)
	}
	if info := a.session("codex:closed").Info(); !info.InBackground || !info.Open {
		t.Fatalf("the answering chat is not open in the background: %+v", info)
	}
	if ids := waitingQuestions(t, a); len(ids) != 0 {
		t.Fatalf("an answered question still waits: %v", ids)
	}
}

// A chat whose window was restored at the prompt to reconnect is
// reconnected in its window for a question, not opened in the background.
func TestAskReconnectsARestoredWindow(t *testing.T) {
	a, ws, _, _, r := closedAskApp(t)
	a.ChatWindows([]string{"codex:closed"})
	done := askInBackground(t, context.Background(), a, ws, "chat-A", "codex:closed", "Are you back?")
	asked := connectRequested(t, a, r)
	if len(asked) != 1 || asked[0]["background"] != false {
		t.Fatalf("the restored window was not reconnected in place: %v", asked)
	}
	openAsAsked(t, a, asked)
	if res := answer(t, done); res.err != nil || res.out["reply"] != "Answer to: Are you back?" {
		t.Fatalf("the restored chat answered %v %v", res.out, res.err)
	}
	if a.session("codex:closed").Info().InBackground {
		t.Fatal("a chat with a window was opened in the background")
	}
}

// A chat that cannot be opened ends the question with the reason, and the
// question no longer wakes it.
func TestAskEndsWhenTheChatCannotOpen(t *testing.T) {
	a, ws, _, _, r := closedAskApp(t)
	{
		initial := a.runtime.ProviderLocked("codex").Initialization()
		initial.AgentCapabilities.LoadSession = false
		setTestInitialization(a, "codex", initial)
	}
	a.ChatWindows(nil)
	done := askInBackground(t, context.Background(), a, ws, "chat-A", "codex:closed", "Anyone there?")
	asked := connectRequested(t, a, r)
	if _, err := a.OpenSessionAt(asked[0]["cwd"].(string), "codex:closed"); err == nil {
		t.Fatal("the agent opened a chat it cannot reopen")
	}
	if res := answer(t, done); res.err == nil || !strings.Contains(res.err.Error(), "could not be opened") {
		t.Fatalf("the failure was not reported: %v %v", res.out, res.err)
	}
	if ids := waitingQuestions(t, a); len(ids) != 0 {
		t.Fatalf("a failed question still waits: %v", ids)
	}
}

// A caller that stops waiting withdraws its question, so the closed chat is
// not opened for it.
func TestACancelledQuestionIsWithdrawn(t *testing.T) {
	a, ws, _, _, r := closedAskApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := askInBackground(t, ctx, a, ws, "chat-A", "codex:closed", "Never mind?")
	for deadline := time.Now().Add(testWait); len(waitingQuestions(t, a)) == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the question was never sent")
		}
	}
	cancel()
	if res := answer(t, done); res.err == nil {
		t.Fatal("a cancelled question returned an answer")
	}
	if ids := waitingQuestions(t, a); len(ids) != 0 {
		t.Fatalf("a cancelled question still waits: %v", ids)
	}
	a.ChatWindows(nil)
	wakeRound(t, a)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a withdrawn question opened its chat: %v", got)
	}
}

// A question no invocation of this run waits for, such as one a window left
// when it stopped, is withdrawn at the watcher's next round; one that an
// invocation waits for stays.
func TestOrphanQuestionsAreWithdrawn(t *testing.T) {
	a, _, chats := askTestApp(t)
	var orphan, live int64
	err := a.busHost().With(func(b *bus.Bus) error {
		from, err := b.Session(context.Background(), chats["A"].BusID())
		if err != nil {
			return err
		}
		m, err := b.Ask(context.Background(), from, chats["B"].BusID(), "Left behind?")
		orphan = m.ID
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if live, err = a.busHost().PostQuestion(context.Background(), chats["A"].BusID(), chats["C"].BusID(), "Still waited for?"); err != nil {
		t.Fatal(err)
	}
	if err := a.busHost().With(func(b *bus.Bus) error { a.busHost().WithdrawOrphans(context.Background(), b); return nil }); err != nil {
		t.Fatal(err)
	}
	if ids := waitingQuestions(t, a); len(ids) != 1 || ids[0] != live || ids[0] == orphan {
		t.Fatalf("the questions waiting are %v, not only %d", ids, live)
	}
	a.busHost().DropQuestion(live)
	if ids := waitingQuestions(t, a); len(ids) != 0 {
		t.Fatalf("a dropped question still waits: %v", ids)
	}
}

// Through the broker, a question that fails reaches the asking chat as a
// completion report, as an answer does.
func TestAFailedQuestionReachesTheAskingChat(t *testing.T) {
	a, ws, chats, _, r := closedAskApp(t)
	a.settings.Approvals = ApprovalsAll
	{
		initial := a.runtime.ProviderLocked("codex").Initialization()
		initial.AgentCapabilities.LoadSession = false
		setTestInitialization(a, "codex", initial)
	}
	a.ChatWindows(nil)
	v := invokeChatQuestion(t, a, ws, chats["A"].ID(), "codex:closed", "fails")
	asked := connectRequested(t, a, r)
	_, _ = a.OpenSessionAt(asked[0]["cwd"].(string), "codex:closed")
	got := awaitApplet(t, a, ws, "chat", chats["A"].ID(), v.ID, "failed")
	if !strings.Contains(got.Error, "could not be opened") {
		t.Fatalf("the failure says %q", got.Error)
	}
	reconcileResults(t, a, map[string]bool{})
	err := a.busHost().With(func(b *bus.Bus) error {
		msgs, _, err := b.Deliver(context.Background(), chats["A"].BusID(), bus.DoorbellTurn)
		if err != nil || len(msgs) != 1 || msgs[0].Kind != bus.ResultKind || !strings.Contains(msgs[0].Text, v.ID) || !strings.Contains(msgs[0].Text, "failed") {
			t.Errorf("the asking chat was not told of the failure: %+v %v", msgs, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// list@2 and find@1 include closed chats, marked closed, and chats open in
// the background, marked so; an untitled chat is shown as the bus shows it.
func TestTheDirectoryListsClosedChats(t *testing.T) {
	a, ws, chats, other, _ := closedAskApp(t)
	savedChat(t, "codex:untitled", other)
	savedChat(t, "codex:hidden", other)
	savedChat(t, "codex:deleted", other)
	savedChat(t, "codex:elsewhere", t.TempDir())
	a.settings.Hidden = []string{"codex:hidden"}
	if err := writeTranscript(&conversationTranscript{SessionID: "codex:deleted", Provider: "codex", Workspace: other, Deleted: true}); err != nil {
		t.Fatal(err)
	}
	chats["C"].ConnectedBackground(time.Time{})
	bundle, err := a.chatAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	run := func(ref, input string) (out struct {
		Chats []chatDirectoryEntry `json:"chats"`
		Match string               `json:"match"`
	}) {
		t.Helper()
		res, err := r.Execute(builtinTestContext(ws), ref, input)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	states := map[string]string{}
	titles := map[string]string{}
	for _, c := range run(chatDirectoryAction, `{}`).Chats {
		states[c.SessionID], titles[c.SessionID] = c.State, c.Title
	}
	want := map[string]string{"chat-A": chatStateOpen, "chat-B": chatStateOpen, "chat-C": chatStateBackground, "codex:closed": chatStateClosed, "codex:untitled": chatStateClosed}
	if len(states) != len(want) {
		t.Fatalf("list@2 listed %v", states)
	}
	for id, state := range want {
		if states[id] != state {
			t.Fatalf("list@2 listed %s as %q, not %q: %v", id, states[id], state, states)
		}
	}
	if titles["codex:untitled"] != "Untitled Chat in Parser" || titles["codex:closed"] != "Parser Release" {
		t.Fatalf("the closed chats were titled %v", titles)
	}
	if out := run(chatFindAction, `{"name":"Parser Release"}`); out.Match != "unique" || out.Chats[0].SessionID != "codex:closed" || out.Chats[0].State != chatStateClosed || out.Chats[0].WorkspaceID != "parser" {
		t.Fatalf("find@1 did not find the closed chat: %+v", out)
	}
	if out := run(chatFindAction, `{"name":"Untitled Chat in Parser"}`); out.Match != "unique" || out.Chats[0].SessionID != "codex:untitled" {
		t.Fatalf("find@1 did not find the untitled chat by its bus title: %+v", out)
	}
	// Open chats come first.
	if list := run(chatDirectoryAction, `{"limit":3}`).Chats; len(list) != 3 || list[0].State == chatStateClosed || list[2].State == chatStateClosed {
		t.Fatalf("closed chats came before open ones: %+v", list)
	}
}

// Asking keeps its contract: the same names, input and output schemas, and
// the digest still covers the implementation, now including the delivery
// over the bus, so a change to it asks agents to discover again.
func TestAskKeepsItsSchemaAndDigest(t *testing.T) {
	a, _, _ := askTestApp(t)
	bundle, err := a.chatAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	input := `{"type":"object","additionalProperties":false,"required":["sessionId","question"],"properties":{"sessionId":{"type":"string","minLength":1,"maxLength":512},"question":{"type":"string","minLength":1,"maxLength":32768}}}`
	output := `{"type":"object","additionalProperties":false,"required":["sessionId","title","reply","stopReason"],"properties":{"sessionId":{"type":"string"},"title":{"type":"string"},"reply":{"type":"string"},"stopReason":{"type":"string"}}}`
	found := 0
	for _, action := range bundle.Manifest.Actions {
		if action.Ref != chatAskAction && action.Ref != legacyChatAskAction {
			continue
		}
		found++
		if string(action.InputSchema) != input || string(action.OutputSchema) != output || action.Replay != "once" || len(action.Capabilities) != 1 || action.Capabilities[0] != "chat.ask" {
			t.Fatalf("%s changed its contract: %+v", action.Ref, action)
		}
	}
	if found != 2 {
		t.Fatalf("found %d ask actions, not ask@1 and ask@2", found)
	}
	for _, name := range []string{"applet_chat.go", "chat_ask.go", "bus_wake.go", "session.go", "chat_delivery.go", "bushost/scheduler.go", "bushost/service.go", "sessionhost/bus_results.go", "sessionhost/bus_delivery.go", "sessionhost/bus_chats.go", "sessionhost/values.go", "sessionhost/transcript.go", "sessionhost/handoff.go", "sessionhost/continuation.go", "sessionhost/transcript_capture.go", "sessionhost/copies.go", "sessionhost/prompt.go", "sessionhost/turns.go", "sessionhost/details.go", "sessionhost/updates.go", "sessionhost/background.go", "sessionhost/subagents.go", "sessionhost/subagent_service.go", "agent_handler.go", "turn_io.go", "provider_lifecycle.go", "session_lifecycle.go", "closed_sessions.go", "subagent_host.go", "subagents.go", "continuation_host.go", "continuation.go", "session_recovery.go", "transcript_capture.go", "delivered.go", "options.go", "option_substitutions.go", "steering_recovery.go", "sessionhost/history.go", "sessionhost/activity.go", "sessionhost/links.go", "sessionhost/conversation.go", "sessionhost/conversation_ops.go", "sessionhost/conversation_state.go", "sessionhost/runtime.go", "sessionhost/provider.go", "sessionhost/lifecycle.go", "sessionhost/handler.go", "sessionhost/turn_io.go", "sessionhost/steering_io.go", "sessionhost/turn_format.go", "sessionhost/journal.go", "sessionhost/options.go", "sessionhost/session_lifecycle.go", "sessionhost/closed_sessions.go", "sessionhost/option_restore.go", "sessionhost/option_substitutions.go", "sessionhost/permissions.go", "sessionhost/steering_ledger.go", "sessionhost/handler_updates.go", "sessionhost/chat_events.go"} {
		if len(bundle.Sources[name]) == 0 {
			t.Fatalf("the digest does not cover %s", name)
		}
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := r.Digest(chatAskAction)
	again, _ := a.chatAppletBundle()
	r2, _ := applet.New(again)
	if same, _ := r2.Digest(chatAskAction); same != digest || digest == "" {
		t.Fatal("the digest is not stable")
	}
	for _, name := range []string{"chat_ask.go", "bushost/scheduler.go", "bushost/service.go", "sessionhost/bus_results.go", "sessionhost/bus_delivery.go", "sessionhost/bus_chats.go", "sessionhost/values.go", "sessionhost/transcript.go", "sessionhost/handoff.go", "sessionhost/continuation.go", "sessionhost/transcript_capture.go", "sessionhost/copies.go", "sessionhost/prompt.go", "sessionhost/turns.go", "sessionhost/details.go", "sessionhost/updates.go", "sessionhost/background.go", "sessionhost/subagents.go", "sessionhost/subagent_service.go", "agent_handler.go", "turn_io.go", "provider_lifecycle.go", "session_lifecycle.go", "closed_sessions.go", "subagent_host.go", "subagents.go", "continuation_host.go", "continuation.go", "session_recovery.go", "transcript_capture.go", "delivered.go", "options.go", "option_substitutions.go", "steering_recovery.go", "sessionhost/history.go", "sessionhost/activity.go", "sessionhost/links.go", "sessionhost/conversation.go", "sessionhost/conversation_ops.go", "sessionhost/conversation_state.go", "sessionhost/runtime.go", "sessionhost/provider.go", "sessionhost/lifecycle.go", "sessionhost/handler.go", "sessionhost/turn_io.go", "sessionhost/steering_io.go", "sessionhost/turn_format.go", "sessionhost/journal.go", "sessionhost/options.go", "sessionhost/session_lifecycle.go", "sessionhost/closed_sessions.go", "sessionhost/option_restore.go", "sessionhost/option_substitutions.go", "sessionhost/permissions.go", "sessionhost/steering_ledger.go", "sessionhost/handler_updates.go", "sessionhost/chat_events.go"} {
		changedBundle, _ := a.chatAppletBundle()
		changedBundle.Sources[name] = append([]byte("// changed\n"), changedBundle.Sources[name]...)
		r3, _ := applet.New(changedBundle)
		if changed, _ := r3.Digest(chatAskAction); changed == digest {
			t.Fatalf("a change to %s left the digest as it was", name)
		}
	}
}
