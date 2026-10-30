package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/applet"
	"studio/internal/bus"
)

// answeringConnection is an agent that replies to every prompt with the
// prompt's last line turned into an answer, as message chunks sent before the
// turn ends.
type answeringConnection struct {
	*testProviderConnection
	a  *App
	mu sync.Mutex
}

func (c *answeringConnection) Call(ctx context.Context, method string, p, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if method == "session/prompt" {
		params := p.(acp.PromptParams)
		text := params.Prompt[0].Text
		update, _ := json.Marshal(map[string]any{"sessionId": params.SessionID, "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "Answer to: " + text[strings.LastIndex(text, "\n")+1:]}}})
		c.a.newAgentHandler().Notify("session/update", update)
	}
	return c.testProviderConnection.Call(ctx, method, p, out)
}

// askTestApp has three open chats of one workspace, A, B and C, each on the
// bus, with an agent that answers.
func askTestApp(t *testing.T) (*App, string, map[string]*Session) {
	t.Helper()
	a, c := providerTestApp(t)
	setTestConnection(a, "codex", &answeringConnection{testProviderConnection: c, a: a})
	ws := t.TempDir()
	chats := map[string]*Session{}
	for _, name := range []string{"A", "B", "C"} {
		s := newSessionState(&sessionSeed{ID: "chat-" + name, Provider: "codex", Cwd: ws, Title: "Chat " + name}, sessionInitial{Open: true})
		a.runtime.PutConversationLocked(s.ID(), s)
		chats[name] = s
		testAppletBridge(t, a, ws, s.ID())
		err := a.busHost().With(func(b *bus.Bus) error {
			_, err := b.RegisterSession(context.Background(), "bus-"+s.ID(), ws, s.Title())
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		a.busCoordinator().JoinBus(s, "bus-"+s.ID())
	}
	return a, ws, chats
}

func askFrom(t *testing.T, a *App, ws, caller, target, question string) (map[string]any, error) {
	t.Helper()
	return askWithContext(t, context.Background(), a, ws, caller, target, question)
}

func askWithContext(t *testing.T, ctx context.Context, a *App, ws, caller, target, question string) (map[string]any, error) {
	t.Helper()
	bundle, err := a.chatAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	ctx = applet.WithInvocation(ctx, applet.Invocation{Workspace: ws, OwnerKind: "chat", OwnerID: caller, ID: "ask-" + caller})
	input, _ := json.Marshal(map[string]string{"sessionId": target, "question": question})
	res, err := bundle.Handlers[chatAskAction](ctx, string(input))
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
		t.Fatalf("the answer is not JSON: %v", err)
	}
	// The readable text goes beside the output under a key no output has.
	out[".text"] = res.Text
	return out, nil
}

// A question travels over the bus and runs as a turn of the other chat's own
// session, shown in its log as the bus message's card from the calling chat,
// and that turn's words come back.
func TestAskRunsAsATurnOfTheOtherChat(t *testing.T) {
	a, ws, chats := askTestApp(t)
	var mu sync.Mutex
	var sent []map[string]any
	a.sink = func(name string, data any) {
		if name == "agent:sent" {
			mu.Lock()
			sent = append(sent, data.(map[string]any))
			mu.Unlock()
		}
	}
	out, err := askFrom(t, a, ws, "chat-A", "chat-B", "Which tableau did you change?")
	if err != nil {
		t.Fatal(err)
	}
	if out["reply"] != "Answer to: Which tableau did you change?" || out["title"] != "Chat B" || out["stopReason"] != "end_turn" {
		t.Fatalf("the answer was %v", out)
	}
	if want := "Reply from the chat \"Chat B\" (session chat-B):\n\nAnswer to: Which tableau did you change?"; out[".text"] != want {
		t.Fatalf("the answer's text was %q", out[".text"])
	}
	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 1 || sent[0]["sessionId"] != "chat-B" || sent[0]["kind"] != "bus" || sent[0]["label"] != "Asked by Chat A" || !strings.Contains(sent[0]["text"].(string), "Do not ask any other chat") {
		t.Fatalf("the asked chat was told %v", sent)
	}
	cards, _ := sent[0]["delivered"].([]ConversationMessage)
	if len(cards) != 1 || cards[0].Delivered == nil || cards[0].Delivered.From != "Chat A" || cards[0].Delivered.FromSession != "bus-chat-A" || cards[0].Text != "Which tableau did you change?" {
		t.Fatalf("the question was not shown as the bus message's card: %+v", cards)
	}

	if chats["B"].Asked() || chats["B"].Capturing() || chats["B"].Working() {
		t.Fatal("the asked turn's state outlived it")
	}
}

func TestAskRefusesWhatTheDesignRulesOut(t *testing.T) {
	a, ws, chats := askTestApp(t)
	if _, err := askFrom(t, a, ws, "chat-A", "chat-A", "Me?"); err == nil || !strings.Contains(err.Error(), "itself") {
		t.Fatalf("a chat asked itself: %v", err)
	}
	if _, err := askFrom(t, a, t.TempDir(), "chat-A", "chat-B", "Elsewhere?"); err == nil {
		t.Fatal("a chat of another workspace was asked")
	}
	if _, err := askFrom(t, a, ws, "chat-A", "codex:unknown", "Anyone?"); err == nil || !strings.Contains(err.Error(), "no chat") {
		t.Fatalf("a chat the application does not know was asked: %v", err)
	}
	a.settings.Hidden = []string{"chat-C"}
	if _, err := askFrom(t, a, ws, "chat-A", "chat-C", "Hidden?"); err == nil {
		t.Fatal("a hidden chat was asked")
	}
	a.settings.Hidden = nil
	question := chats["A"].WithReply(context.Background(), newPrompt("question", nil))

	chats["A"].StartCapture(&question, false)

	if _, err := askFrom(t, a, ws, "chat-A", "chat-B", "Chain?"); err == nil || !strings.Contains(err.Error(), "answering another chat") {
		t.Fatalf("a chat answering a question asked another: %v", err)
	}

	chats["A"].StartCapture(nil, false)

	bundle, _ := a.chatAppletBundle()
	ctx := applet.WithInvocation(context.Background(), applet.Invocation{Workspace: ws, OwnerKind: "applet", OwnerID: "run", ID: "x"})
	if _, err := bundle.Handlers[chatAskAction](ctx, `{"sessionId":"chat-B","question":"From a flow?"}`); err == nil {
		t.Fatal("a flow asked a chat")
	}
	if _, err := askFrom(t, a, ws, "chat-A", "chat-B", strings.Repeat("x", maxAskQuestion+1)); err == nil {
		t.Fatal("an oversized question was taken")
	}
}

// A busy chat takes the question after its own turn, and a caller that stops
// waiting takes its question out of the queue.
func TestAskWaitsForABusyChat(t *testing.T) {
	a, ws, chats := askTestApp(t)
	b := chats["B"]
	b = replaceTestIdentity(a, b, "Cwd", t.TempDir()) // A busy recipient can belong to another workspace.
	b.StartWork()
	type result struct {
		out map[string]any
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := askFrom(t, a, ws, "chat-A", "chat-B", "After your turn?")
		done <- result{out, err}
	}()
	waitQueued(t, b, 1)
	select {
	case r := <-done:
		t.Fatalf("the question was answered during the chat's turn: %v %v", r.out, r.err)
	default:
	}
	a.turnIO().Run(b, promptPointer("Own work", []acp.ContentBlock{{Type: "text", Text: "Own work"}}))
	select {
	case r := <-done:
		if r.err != nil || r.out["reply"] != "Answer to: After your turn?" {
			t.Fatalf("the queued question was answered with %v %v", r.out, r.err)
		}
	case <-time.After(testWait):
		t.Fatal("the queued question was never answered")
	}

	b.StartWork()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_, err := askWithContext(t, ctx, a, ws, "chat-A", "chat-B", "Still there?")
		done <- result{nil, err}
	}()
	waitQueued(t, b, 1)
	cancel()
	if r := <-done; r.err == nil {
		t.Fatal("a cancelled question returned an answer")
	}
	waitQueued(t, b, 0)

	go func() {
		_, err := askFrom(t, a, ws, "chat-A", "chat-B", "Dropped?")
		done <- result{nil, err}
	}()
	waitQueued(t, b, 1)
	a.Unqueue("chat-B")
	if r := <-done; r.err == nil || !strings.Contains(r.err.Error(), "cleared") {
		t.Fatalf("a cleared question did not say so: %v", r.err)
	}
}

func waitQueued(t *testing.T, s *Session, n int) {
	t.Helper()
	for deadline := time.Now().Add(testWait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		got := s.QueueCount()
		if got == n {
			return
		}
	}
	t.Fatalf("the queue never held %d questions", n)
}

// Chats waiting on each other through asks would wait until they time out,
// so the ask that would close the loop is refused.
func TestAskRefusesALoopOfWaitingChats(t *testing.T) {
	a := &App{}
	if !a.beginAsk("A", "B") || !a.beginAsk("B", "C") {
		t.Fatal("a chain without a loop was refused")
	}
	if a.beginAsk("C", "A") || a.beginAsk("B", "A") {
		t.Fatal("a loop was allowed")
	}
	a.endAsk("A", "B")
	if !a.beginAsk("C", "A") {
		t.Fatal("an ended ask still counted")
	}
	a.endAsk("C", "A")
	a.endAsk("B", "C")
	if a.runtime.AskCountLocked() != 0 {
		t.Fatalf("ended asks were kept: %v", a.runtime.AskCountLocked())
	}
}

// Mentioning another chat tells the agent its destination across workspaces.
func TestChatMentionTellsTheAgentHowToAsk(t *testing.T) {
	a, _, chats := askTestApp(t)
	blocks, err := a.blocks(chats["A"], "Ask @Chat_B", []Attachment{{Kind: "chat", SessionID: "chat-B", Name: "Chat B"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || !strings.Contains(blocks[1].Text, chatAskAction) || !strings.Contains(blocks[1].Text, `"chat-B"`) {
		t.Fatalf("the agent was told %+v", blocks)
	}
	chats["C"] = replaceTestIdentity(a, chats["C"], "Cwd", t.TempDir())
	if _, err := a.blocks(chats["A"], "Ask @Chat_C", []Attachment{{Kind: "chat", SessionID: "chat-C", Name: "Chat C"}}); err != nil {
		t.Fatalf("a cross-workspace mention was refused: %v", err)
	}
	if _, err := a.blocks(chats["A"], "Ask @Chat_A", []Attachment{{Kind: "chat", SessionID: "chat-A", Name: "Chat A"}}); err == nil {
		t.Fatal("a chat mentioned itself")
	}
}
