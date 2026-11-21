package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/applet"
	appletHost "studio/internal/applet/host"
	"studio/internal/servicechats"
	"studio/internal/services"
	"studio/internal/taskhost"
)

// serviceChatPeer is an agent for the turns of a service's chat port. It
// counts the prompts that reach it, and its mode says what a prompt does.
type serviceChatPeer struct {
	*testProviderConnection
	app     *App
	mu      sync.Mutex
	mode    string
	prompts atomic.Int32
	cancels atomic.Int32
	entered chan struct{}
	cancel  chan struct{}
	answer  chan string
}

func (p *serviceChatPeer) set(mode string) { p.mu.Lock(); p.mode = mode; p.mu.Unlock() }

func (p *serviceChatPeer) Call(ctx context.Context, method string, in, out any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.testProviderConnection.Call(ctx, method, in, out)
}

func (p *serviceChatPeer) PromptWithDispatch(_ context.Context, params acp.PromptParams, out any, dispatched func()) error {
	n := p.prompts.Add(1)
	dispatched()
	p.mu.Lock()
	mode := p.mode
	p.mu.Unlock()
	switch mode {
	case "lost":
		return errors.New("invented prompt request consumed; response lost")
	case "block":
		p.entered <- struct{}{}
		select {
		case <-p.cancel:
		case <-p.Done():
		}
		return acp.Result(acp.PromptResult{StopReason: "cancelled"}, out)
	case "stuck":
		p.entered <- struct{}{}
		<-p.Done()
		return errors.New("invented agent stopped without ending its turn")
	case "ask":
		choice, _ := p.app.permissions().Ask(context.Background(), params.SessionID, nil, json.RawMessage(`{"title":"Invented tool"}`), []acp.PermissionOption{{OptionID: "allow_once", Name: "Allow", Kind: "allow_once"}})
		p.answer <- choice
	}
	update, _ := json.Marshal(map[string]any{"sessionId": params.SessionID, "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "Answer " + string(rune('0'+n))}}})
	p.app.newAgentHandler().Notify("session/update", update)
	return acp.Result(acp.PromptResult{StopReason: "end_turn"}, out)
}

func (p *serviceChatPeer) Notify(method string, in any) error {
	if method == "session/cancel" {
		p.cancels.Add(1)
		select {
		case p.cancel <- struct{}{}:
		default:
		}
		return nil
	}
	return p.testProviderConnection.Notify(method, in)
}

// serviceChatFixture is an App with one saved workspace, an agent that takes
// owned turns, and the port of an invented service called "sample".
func serviceChatFixture(t *testing.T) (*App, *serviceChatPeer, servicechats.Port, string) {
	t.Helper()
	a, c := providerTestApp(t)
	peer := &serviceChatPeer{testProviderConnection: c, app: a, entered: make(chan struct{}, 4), cancel: make(chan struct{}, 4), answer: make(chan string, 1)}
	setTestConnection(a, "codex", peer)
	// The window watches every agent it started, and deleting a chat waits
	// for that watch when it stops an agent nobody uses any more.
	go a.lifecycle().Watch("codex", a.runtime.ProviderLocked("codex"))
	workspace := t.TempDir()
	record := newWorkspaceRecord(workspace)
	a.settings.Workspace = workspace
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	chats := servicechats.New(serviceChatBackend{a})
	a.serviceChats.window = chats
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := chats.Shutdown(ctx); err != nil {
			t.Error(err)
		}
		a.closeServiceChats()
	})
	return a, peer, chats.ForService("sample"), workspace
}

func openSampleChat(t *testing.T, peer *serviceChatPeer, port servicechats.Port, workspace, id string) servicechats.Chat {
	t.Helper()
	peer.mu.Lock()
	peer.newIDs = []string{id}
	peer.mu.Unlock()
	chat, err := port.OpenChat(context.Background(), servicechats.OpenRequest{Workspace: workspace, Program: "codex", Title: "Sample Work"})
	if err != nil {
		t.Fatal(err)
	}
	return chat
}

func TestServiceChatOpensAnOrdinaryChatInASavedWorkspace(t *testing.T) {
	a, peer, port, workspace := serviceChatFixture(t)
	ctx := context.Background()
	if _, err := port.OpenChat(ctx, servicechats.OpenRequest{Workspace: t.TempDir(), Program: "codex"}); !errors.Is(err, servicechats.ErrNotWorkspace) {
		t.Fatalf("a folder that is no workspace: %v", err)
	}
	if _, err := port.OpenChat(ctx, servicechats.OpenRequest{Workspace: "relative", Program: "codex"}); !errors.Is(err, servicechats.ErrNotWorkspace) {
		t.Fatalf("a relative folder: %v", err)
	}
	if _, err := port.OpenChat(ctx, servicechats.OpenRequest{Workspace: workspace, Program: "no-such-program"}); !errors.Is(err, servicechats.ErrProgram) {
		t.Fatalf("a program that is not offered: %v", err)
	}
	for _, call := range peer.calls.Snapshot() {
		if call.method == "session/new" {
			t.Fatal("a refused opening reached the agent")
		}
	}
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	if chat.ID != "codex:sample-a" || chat.Program != "codex" || chat.Title != "Sample Work" || !sameFile(chat.Workspace, workspace) {
		t.Fatalf("chat: %+v", chat)
	}
	// The chat is an ordinary record: it is loaded as any chat is, it has the
	// title in the person's settings, it is not hidden, and the person can
	// delete it.
	s := a.session(chat.ID)
	if s == nil || s.Provider() != "codex" {
		t.Fatal("the chat is not open in the window")
	}
	a.mu.Lock()
	title, hidden := a.settings.SessionTitles[chat.ID], a.hidden(chat.ID)
	a.mu.Unlock()
	if title != "Sample Work" || hidden {
		t.Fatalf("title %q, hidden %v", title, hidden)
	}
	if err := port.RenameChat(ctx, chat.ID, "Sample Work, Second Part"); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	title = a.settings.SessionTitles[chat.ID]
	a.mu.Unlock()
	if title != "Sample Work, Second Part" {
		t.Fatalf("renamed title %q", title)
	}
	// Another service's port does not act on this chat.
	other := a.serviceChats.window.ForService("other")
	if err := other.RenameChat(ctx, chat.ID, "Taken"); !errors.Is(err, servicechats.ErrNotOpened) {
		t.Fatalf("another service renamed the chat: %v", err)
	}
	if _, err := other.TakeTurn(ctx, chat.ID, servicechats.Prompt{Text: "hello"}); !errors.Is(err, servicechats.ErrNotOpened) {
		t.Fatalf("another service took a turn: %v", err)
	}
	if err := a.RemoveChat(chat.ID); err != nil {
		t.Fatalf("the person could not delete the chat: %v", err)
	}
	if _, err := port.TakeTurn(ctx, chat.ID, servicechats.Prompt{Text: "after deletion"}); !errors.Is(err, servicechats.ErrRemoved) {
		t.Fatalf("a turn in a deleted chat: %v", err)
	}
	if peer.prompts.Load() != 0 {
		t.Fatal("a refused turn reached the agent")
	}
}

func TestServiceChatTurnReturnsTheReplyAndABusyChatGetsNothing(t *testing.T) {
	a, peer, port, workspace := serviceChatFixture(t)
	ctx := context.Background()
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	var sent []string
	var mu sync.Mutex
	a.sink = func(name string, data any) {
		if strings.HasPrefix(name, "agent:") {
			mu.Lock()
			sent = append(sent, name)
			mu.Unlock()
		}
	}
	reply, err := port.TakeTurn(ctx, chat.ID, servicechats.Prompt{Text: "Summarize the invented report.", Label: "Sample"})
	if err != nil || reply.StopReason != "end_turn" || reply.Text != "Answer 1" {
		t.Fatalf("turn: %+v %v", reply, err)
	}
	// A later prompt may contain an earlier answer. Nothing was reviewed.
	reply, err = port.TakeTurn(ctx, chat.ID, servicechats.Prompt{Text: "Go on from: " + reply.Text, Label: "Sample"})
	if err != nil || reply.Text != "Answer 2" {
		t.Fatalf("second turn: %+v %v", reply, err)
	}
	mu.Lock()
	events := len(sent)
	mu.Unlock()
	if events == 0 {
		t.Fatal("the turns sent the chat's page no events")
	}
	// A chat that is working is refused at once. Nothing is sent or queued.
	s := a.session(chat.ID)
	if !s.AdmitLabelled() {
		t.Fatal("the fixture could not make the chat work")
	}
	started := time.Now()
	if _, err := port.TakeTurn(ctx, chat.ID, servicechats.Prompt{Text: "while working"}); !errors.Is(err, servicechats.ErrBusy) {
		t.Fatalf("a turn in a working chat: %v", err)
	}
	if _, err := port.TakeTurnInExistingChat(ctx, chat.ID, servicechats.Prompt{Text: "while working"}); !errors.Is(err, servicechats.ErrBusy) {
		t.Fatalf("a turn by identity in a working chat: %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("the refusal of a working chat waited")
	}
	if _, err := port.UseModel(ctx, chat.ID, "changed"); !errors.Is(err, servicechats.ErrBusy) {
		t.Fatalf("a model change in a working chat: %v", err)
	}
	s.ReleaseWork()
	if peer.prompts.Load() != 2 {
		t.Fatalf("%d prompts reached the agent; a refused turn was sent", peer.prompts.Load())
	}
	// The chat takes a turn again once it is idle, and only the new prompt.
	if _, err := port.TakeTurn(ctx, chat.ID, servicechats.Prompt{Text: "after the work"}); err != nil {
		t.Fatal(err)
	}
	if peer.prompts.Load() != 3 {
		t.Fatalf("%d prompts after the chat was idle; a refused prompt was kept and sent", peer.prompts.Load())
	}
}

func TestServiceChatUnknownOutcomeIsNeverSentAgain(t *testing.T) {
	_, peer, port, workspace := serviceChatFixture(t)
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	peer.set("lost")
	_, err := port.TakeTurn(context.Background(), chat.ID, servicechats.Prompt{Text: "an effect that must happen once"})
	if !errors.Is(err, servicechats.ErrUnknownOutcome) || errors.Is(err, servicechats.ErrNotSent) {
		t.Fatalf("a lost response: %v", err)
	}
	// The port waits a moment, as a resending caller would, and sends nothing.
	time.Sleep(100 * time.Millisecond)
	if peer.prompts.Load() != 1 {
		t.Fatalf("the prompt reached the agent %d times", peer.prompts.Load())
	}
}

func TestServiceChatFailureBeforeTheAgentIsMarkedNotSent(t *testing.T) {
	a, peer, port, workspace := serviceChatFixture(t)
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	// This agent connection has no owned dispatch, so the turn fails before
	// anything is handed to it.
	setTestConnection(a, "codex", peer.testProviderConnection)
	_, err := port.TakeTurn(context.Background(), chat.ID, servicechats.Prompt{Text: "not sent"})
	if !errors.Is(err, servicechats.ErrNotSent) || errors.Is(err, servicechats.ErrUnknownOutcome) {
		t.Fatalf("a failure before dispatch: %v", err)
	}
	for _, call := range peer.calls.Snapshot() {
		if call.method == "session/prompt" {
			t.Fatal("the prompt reached the agent")
		}
	}
	if a.session(chat.ID).Working() {
		t.Fatal("the failed turn left the chat working")
	}
}

func TestServiceChatCancelledContextAndCancelTurnStopTheTurn(t *testing.T) {
	a, peer, port, workspace := serviceChatFixture(t)
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	peer.set("block")
	type result struct {
		reply servicechats.Reply
		err   error
	}
	run := func(ctx context.Context) chan result {
		done := make(chan result, 1)
		go func() {
			reply, err := port.TakeTurn(ctx, chat.ID, servicechats.Prompt{Text: "long work", Label: "Sample"})
			done <- result{reply, err}
		}()
		select {
		case <-peer.entered:
		case <-time.After(testWait):
			t.Fatal("the turn did not reach the agent")
		}
		return done
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := run(ctx)
	cancel()
	select {
	case r := <-done:
		if !errors.Is(r.err, context.Canceled) || r.reply.StopReason != "cancelled" {
			t.Fatalf("a cancelled context: %+v", r)
		}
	case <-time.After(testWait):
		t.Fatal("a cancelled context did not stop the turn")
	}
	if peer.cancels.Load() != 1 {
		t.Fatalf("the agent was asked to stop %d times", peer.cancels.Load())
	}
	for deadline := time.Now().Add(testWait); a.session(chat.ID).Working() && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	done = run(context.Background())
	requested, err := port.CancelTurn(context.Background(), chat.ID)
	if !requested || err != nil {
		t.Fatalf("CancelTurn: %v %v", requested, err)
	}
	select {
	case r := <-done:
		if r.err != nil || r.reply.StopReason != "cancelled" {
			t.Fatalf("a turn stopped by CancelTurn: %+v", r)
		}
	default:
		t.Fatal("CancelTurn returned before the turn did")
	}
	if peer.cancels.Load() != 2 || peer.prompts.Load() != 2 {
		t.Fatalf("cancels %d, prompts %d", peer.cancels.Load(), peer.prompts.Load())
	}
	if requested, err := port.CancelTurn(context.Background(), chat.ID); requested || err != nil {
		t.Fatalf("CancelTurn with no turn of the port: %v %v", requested, err)
	}
}

func TestServiceChatAgentThatDoesNotStopLeavesAnUnknownOutcome(t *testing.T) {
	_, peer, port, workspace := serviceChatFixture(t)
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	peer.set("stuck")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := port.TakeTurn(ctx, chat.ID, servicechats.Prompt{Text: "work that will not stop", Label: "Sample"})
		done <- err
	}()
	select {
	case <-peer.entered:
	case <-time.After(testWait):
		t.Fatal("the turn did not reach the agent")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, servicechats.ErrUnknownOutcome) || !errors.Is(err, context.Canceled) {
			t.Fatalf("a turn the agent did not end: %v", err)
		}
	case <-time.After(testWait):
		t.Fatal("the port waited without end for an agent that does not stop")
	}
	if peer.cancels.Load() == 0 {
		t.Fatal("the agent was not asked to stop")
	}
	// The chat still works, so a new turn is refused, and the prompt whose
	// outcome is unknown is not sent again.
	if _, err := port.TakeTurn(context.Background(), chat.ID, servicechats.Prompt{Text: "again"}); !errors.Is(err, servicechats.ErrBusy) {
		t.Fatalf("a turn in a chat whose agent has not settled: %v", err)
	}
	if peer.prompts.Load() != 1 {
		t.Fatalf("%d prompts reached the agent", peer.prompts.Load())
	}
}

func TestServiceChatPermissionRequestAppearsInTheChat(t *testing.T) {
	a, peer, port, workspace := serviceChatFixture(t)
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	peer.set("ask")
	asked := make(chan map[string]any, 1)
	a.sink = func(name string, data any) {
		if name == "agent:permission" {
			asked <- data.(map[string]any)
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := port.TakeTurn(context.Background(), chat.ID, servicechats.Prompt{Text: "use the invented tool", Label: "Sample"})
		done <- err
	}()
	var question map[string]any
	select {
	case question = <-asked:
	case <-time.After(testWait):
		t.Fatal("the agent's permission request was not shown")
	}
	if question["sessionId"] != chat.ID {
		t.Fatalf("the request belongs to %v", question["sessionId"])
	}
	// The port answers nothing. The turn waits for the person.
	select {
	case err := <-done:
		t.Fatalf("the turn ended before the person answered: %v", err)
	case choice := <-peer.answer:
		t.Fatalf("the request was answered with %q without the person", choice)
	case <-time.After(150 * time.Millisecond):
	}
	if err := a.Answer(question["key"].(string), "allow_once"); err != nil {
		t.Fatal(err)
	}
	if choice := <-peer.answer; choice != "allow_once" {
		t.Fatalf("the agent received %q", choice)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(testWait):
		t.Fatal("the turn did not end after the answer")
	}
}

func TestServiceChatExistingChatNeedsItsOwnMethod(t *testing.T) {
	a, peer, port, workspace := serviceChatFixture(t)
	ctx := context.Background()
	peer.mu.Lock()
	peer.newIDs = []string{"codex:persons-chat"}
	peer.mu.Unlock()
	info, err := a.NewSessionWithProviderAt(workspace, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := port.TakeTurn(ctx, info.ID, servicechats.Prompt{Text: "hello"}); !errors.Is(err, servicechats.ErrNotOpened) {
		t.Fatalf("TakeTurn in a chat the service did not open: %v", err)
	}
	if _, err := port.Model(ctx, info.ID); !errors.Is(err, servicechats.ErrNotOpened) {
		t.Fatalf("Model of a chat the service did not open: %v", err)
	}
	if err := port.ReleaseChat(ctx, info.ID); !errors.Is(err, servicechats.ErrNotOpened) {
		t.Fatalf("ReleaseChat of a chat the service did not open: %v", err)
	}
	if peer.prompts.Load() != 0 {
		t.Fatal("a refused turn reached the agent")
	}
	reply, err := port.TakeTurnInExistingChat(ctx, info.ID, servicechats.Prompt{Text: "hello", Label: "Sample"})
	if err != nil || reply.StopReason != "end_turn" {
		t.Fatalf("a turn in the chat the person chose: %+v %v", reply, err)
	}
	if err := a.RemoveChat(info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := port.TakeTurnInExistingChat(ctx, info.ID, servicechats.Prompt{Text: "again"}); !errors.Is(err, servicechats.ErrRemoved) {
		t.Fatalf("a turn in a deleted chat: %v", err)
	}
	if peer.prompts.Load() != 1 {
		t.Fatalf("%d prompts reached the agent", peer.prompts.Load())
	}
}

func TestServiceChatModelIsSetAndPutBack(t *testing.T) {
	a, peer, port, workspace := serviceChatFixture(t)
	ctx := context.Background()
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	model, err := port.Model(ctx, chat.ID)
	if err != nil || model.Value != "initial" || model.Name != "Initial" || len(model.Offered) != 4 {
		t.Fatalf("model: %+v %v", model, err)
	}
	if _, err := port.UseModel(ctx, chat.ID, "no-such-model"); !errors.Is(err, servicechats.ErrModel) {
		t.Fatalf("a model the agent does not offer: %v", err)
	}
	restore, err := port.UseModel(ctx, chat.ID, "changed")
	if err != nil {
		t.Fatal(err)
	}
	if model, _ = port.Model(ctx, chat.ID); model.Value != "changed" {
		t.Fatalf("after UseModel: %+v", model)
	}
	if err := restore(ctx); err != nil {
		t.Fatal(err)
	}
	if model, _ = port.Model(ctx, chat.ID); model.Value != "initial" {
		t.Fatalf("after the restore: %+v", model)
	}
	// The person's own later choice is never undone by a restore.
	restore, err = port.UseModel(ctx, chat.ID, "changed")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetSessionOption(chat.ID, "model", json.RawMessage(`"devin-default"`)); err != nil {
		t.Fatal(err)
	}
	if err := restore(ctx); !errors.Is(err, servicechats.ErrModelChanged) {
		t.Fatalf("a restore over the person's choice: %v", err)
	}
	if model, _ = port.Model(ctx, chat.ID); model.Value != "devin-default" {
		t.Fatalf("the person's choice was lost: %+v", model)
	}
}

func TestServiceChatReleaseEndsTheServicesUse(t *testing.T) {
	a, peer, port, workspace := serviceChatFixture(t)
	ctx := context.Background()
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	a.serviceChats.mu.Lock()
	held := len(a.serviceChats.holds)
	a.serviceChats.mu.Unlock()
	if held != 1 {
		t.Fatalf("%d chats kept loaded", held)
	}
	if err := port.ReleaseChat(ctx, chat.ID); err != nil {
		t.Fatal(err)
	}
	if err := port.ReleaseChat(ctx, chat.ID); err != nil {
		t.Fatalf("a second release: %v", err)
	}
	a.serviceChats.mu.Lock()
	held = len(a.serviceChats.holds)
	a.serviceChats.mu.Unlock()
	if held != 0 {
		t.Fatal("a released chat is still kept loaded")
	}
	if _, err := port.TakeTurn(ctx, chat.ID, servicechats.Prompt{Text: "after release"}); !errors.Is(err, servicechats.ErrReleased) {
		t.Fatalf("a turn in a released chat: %v", err)
	}
	if peer.prompts.Load() != 0 {
		t.Fatal("a refused turn reached the agent")
	}
	// The chat stays the person's.
	if a.session(chat.ID) == nil || a.removedChat(chat.ID) {
		t.Fatal("the release closed or deleted the chat")
	}
	// The record of the opening outlives the window.
	saved, err := readServiceChats("sample")
	if err != nil || !saved.Chats[chat.ID].Released || saved.Chats[chat.ID].Program != "codex" {
		t.Fatalf("saved record: %+v %v", saved, err)
	}
}

func TestServiceChatSkillsProgramsAndChangeNotice(t *testing.T) {
	a, _, port, workspace := serviceChatFixture(t)
	ctx := context.Background()
	programs, err := port.Programs(ctx)
	if err != nil || len(programs) != len(product.Providers) {
		t.Fatalf("programs: %+v %v", programs, err)
	}
	state, err := port.CheckProgram(ctx, "no-such-program")
	if err != nil || state.Offered || state.Reachable || state.Reason == "" {
		t.Fatalf("a program that is not offered: %+v %v", state, err)
	}
	available, err := port.SkillAvailable(ctx, workspace, "codex", "no-such-skill")
	if err != nil || available {
		t.Fatalf("an invented skill: %v %v", available, err)
	}
	if _, err := port.SkillAvailable(ctx, t.TempDir(), "codex", "anything"); !errors.Is(err, servicechats.ErrNotWorkspace) {
		t.Fatalf("a skill in a folder that is no workspace: %v", err)
	}
	// Text that starts with no skill's name is the prompt as it is, as it is in a chat.
	for _, text := range []string{"plain words", "/no-such-skill with arguments"} {
		prepared, err := port.SkillPrompt(ctx, workspace, "codex", text)
		want, wantErr := a.skillPromptFor("codex", workspace, text)
		if err != nil || wantErr != nil || prepared != want {
			t.Fatalf("prepared %q, the chat's preparation gives %q (%v, %v)", prepared, want, err, wantErr)
		}
	}
	var events []string
	var change servicechats.Change
	a.sink = func(name string, data any) {
		if strings.HasPrefix(name, "service:") {
			events = append(events, name)
			change = data.(servicechats.Change)
		}
	}
	if err := port.PublishChange(ctx, workspace, "record-7"); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0] != "service:sample:changed" || change != (servicechats.Change{Service: "sample", Workspace: workspace, Key: "record-7"}) {
		t.Fatalf("events %v, change %+v", events, change)
	}
	if err := port.PublishChange(ctx, workspace, ""); err == nil {
		t.Fatal("a notice without a key was sent")
	}
}

func TestServiceChatRunsAppletActionsAsTheService(t *testing.T) {
	a, _, port, workspace := serviceChatFixture(t)
	ctx := context.Background()
	const plain, guarded, once = "fixture.sample/effect@1", "fixture.sample/guarded@1", "fixture.sample/once@1"
	var runs atomic.Int32
	var owner atomic.Value
	var lose atomic.Bool
	handler := func(ctx context.Context, input string) (applet.Result, error) {
		runs.Add(1)
		if v, ok := applet.InvocationFromContext(ctx); ok {
			owner.Store(v.OwnerKind + ":" + v.OwnerID)
		}
		if lose.Load() {
			return applet.Result{}, errors.New("invented action consumed the request and lost the response")
		}
		return applet.Result{OutputJSON: `{"done":true}`}, nil
	}
	schema := json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"],"additionalProperties":false}`)
	action := func(ref, replay string, capabilities []string) applet.Action {
		return applet.Action{Ref: ref, Title: "Invented effect", Description: "Consumes invented input", Replay: replay, Effect: "read", Capabilities: capabilities, InputSchema: schema, OutputSchema: json.RawMessage(`{"type":"object"}`)}
	}
	bundle := applet.Bundle{Manifest: applet.Manifest{ID: "fixture.sample", Version: "1.0.0", Title: "Invented peer", APIVersion: 1, Actions: []applet.Action{
		action(plain, "pure", []string{}), action(guarded, "pure", []string{"person.approval"}), action(once, "once", []string{}),
	}}, Handlers: map[string]applet.Handler{plain: handler, guarded: handler, once: handler}, Sources: map[string][]byte{"peer.go": []byte("invented action peer")}}
	a.appletHost = appletHost.New(appletHost.Options{State: configDir(), Builtins: func() ([]applet.Bundle, error) { return []applet.Bundle{bundle}, nil }, Extensions: func() ([]applet.Bundle, error) { return nil, nil }, Revision: func() string { return "fixture" }, Emit: func(string, any) {}, Log: func(string, string, ...any) {}})

	described, err := port.DescribeAction(ctx, plain)
	if err != nil || described.Digest == "" || described.NeedsPerson || described.Title != "Invented effect" {
		t.Fatalf("description: %+v %v", described, err)
	}
	if digest, err := port.ActionDigest(ctx, plain); err != nil || digest != described.Digest {
		t.Fatalf("digest %q %v", digest, err)
	}
	deadline := time.Now().Add(time.Minute)
	result, err := port.RunAction(ctx, servicechats.ActionRun{Workspace: workspace, Reference: plain, Input: json.RawMessage(`{"text":"x"}`), Digest: described.Digest, Deadline: deadline})
	if err != nil || string(result.Output) != `{"done":true}` || result.ID == "" {
		t.Fatalf("run: %+v %v", result, err)
	}
	if owner.Load() != "service:sample" {
		t.Fatalf("the invocation was recorded for %v", owner.Load())
	}
	// A changed implementation, a missing deadline and an action that needs
	// the person's approval of every invocation are refused before they run.
	if _, err := port.RunAction(ctx, servicechats.ActionRun{Workspace: workspace, Reference: plain, Input: json.RawMessage(`{"text":"x"}`), Digest: "another-digest", Deadline: deadline}); err == nil {
		t.Fatal("an action with another digest ran")
	}
	if _, err := port.RunAction(ctx, servicechats.ActionRun{Workspace: workspace, Reference: plain, Input: json.RawMessage(`{"text":"x"}`)}); err == nil {
		t.Fatal("an action without a deadline ran")
	}
	if needs, _ := port.DescribeAction(ctx, guarded); !needs.NeedsPerson {
		t.Fatal("the description does not say that the action needs the person")
	}
	if _, err := port.RunAction(ctx, servicechats.ActionRun{Workspace: workspace, Reference: guarded, Input: json.RawMessage(`{"text":"x"}`), Deadline: deadline}); !errors.Is(err, servicechats.ErrNeedsPerson) {
		t.Fatalf("an action that needs the person: %v", err)
	}
	if runs.Load() != 1 {
		t.Fatalf("%d actions ran; a refused one ran", runs.Load())
	}
	// An action that may have had its effect is an unknown outcome, and it
	// is not run again.
	lose.Store(true)
	if _, err := port.RunAction(ctx, servicechats.ActionRun{Workspace: workspace, Reference: once, Input: json.RawMessage(`{"text":"x"}`), Deadline: deadline}); !errors.Is(err, servicechats.ErrUnknownOutcome) {
		t.Fatalf("a lost action response: %v", err)
	}
	if runs.Load() != 2 {
		t.Fatalf("%d actions ran after a lost response", runs.Load())
	}
	if err := port.CancelAction(ctx, workspace, result.ID); err != nil {
		t.Fatalf("cancelling a finished invocation: %v", err)
	}
}

// The window closes the port's admission, stops its turns, asks the
// services to finish and joins the calls, in that order, and lets go of the
// chats the services kept loaded.
func TestServiceChatWindowShutdownStopsTurnsAndJoins(t *testing.T) {
	a, peer, port, workspace := serviceChatFixture(t)
	chat := openSampleChat(t, peer, port, workspace, "codex:sample-a")
	host, err := taskhost.New(taskhost.Options{State: configDir(), Resolve: a.taskWorkspace,
		Check: func(context.Context, taskhost.Scope) error { return nil },
		Admit: a.admitTaskOperation, TargetRevision: a.taskTargetRevision, Confirm: a.confirmTaskReview,
		Backend: taskBackend{a}, Track: func(taskhost.Scope, taskhost.Operation) (func(), error) { return func() {}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	window, err := services.NewWindowWithChats(host, a.serviceChats.window)
	if err != nil {
		t.Fatal(err)
	}
	a.taskServices.mu.Lock()
	a.taskServices.host, a.taskServices.window = host, window
	a.taskServices.mu.Unlock()
	peer.set("block")
	done := make(chan error, 1)
	go func() {
		_, err := port.TakeTurn(context.Background(), chat.ID, servicechats.Prompt{Text: "long work", Label: "Sample"})
		done <- err
	}()
	select {
	case <-peer.entered:
	case <-time.After(testWait):
		t.Fatal("the turn did not reach the agent")
	}
	a.closeTaskServices()
	// The join is over when closeTaskServices returns, so the turn has
	// returned already.
	select {
	case err := <-done:
		if !errors.Is(err, servicechats.ErrClosed) {
			t.Fatalf("a turn stopped by the window's shutdown: %v", err)
		}
	default:
		t.Fatal("the window's shutdown returned while a turn of the port was running")
	}
	if peer.cancels.Load() != 1 {
		t.Fatalf("the agent was asked to stop %d times", peer.cancels.Load())
	}
	if a.serviceChats.window.Running() != 0 {
		t.Fatal("calls of the port outlived the shutdown")
	}
	a.serviceChats.mu.Lock()
	held := len(a.serviceChats.holds)
	a.serviceChats.mu.Unlock()
	if held != 0 {
		t.Fatal("the shutdown kept a chat loaded for a service")
	}
	if _, err := port.TakeTurn(context.Background(), chat.ID, servicechats.Prompt{Text: "late"}); !errors.Is(err, servicechats.ErrClosed) {
		t.Fatalf("a turn after shutdown: %v", err)
	}
	if _, err := port.OpenChat(context.Background(), servicechats.OpenRequest{Workspace: workspace, Program: "codex"}); !errors.Is(err, servicechats.ErrClosed) {
		t.Fatalf("an opening after shutdown: %v", err)
	}
	if peer.prompts.Load() != 1 {
		t.Fatalf("%d prompts reached the agent", peer.prompts.Load())
	}
}
