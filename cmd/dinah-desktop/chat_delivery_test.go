package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/applet/broker"
	"studio/internal/bus"
	"studio/internal/communication"
	"studio/internal/sessionhost"
)

func asyncChatTestApp(t *testing.T) (*App, string, map[string]*Session) {
	t.Helper()
	a, ws, chats := askTestApp(t)
	a.settings.Approvals = ApprovalsAll
	return a, ws, chats
}

func invokeChatQuestion(t *testing.T, a *App, ws, owner, target, request string) AppletInvocation {
	t.Helper()
	_, client := testAppletBridge(t, a, ws, owner)
	catalog, err := client.call(context.Background(), appletBridgeRequest{Operation: "discover"})
	if err != nil {
		t.Fatal(err)
	}
	var discovered struct {
		Actions []struct {
			Action struct {
				Ref string `json:"ref"`
			} `json:"action"`
			Digest string `json:"digest"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(catalog, &discovered); err != nil {
		t.Fatal(err)
	}
	var digest string
	for _, action := range discovered.Actions {
		if action.Action.Ref == chatAskAction {
			digest = action.Digest
		}
	}
	input, _ := json.Marshal(map[string]string{"sessionId": target, "question": "Which parser files changed?"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	data, err := client.call(ctx, appletBridgeRequest{Operation: "invoke", Ref: chatAskAction, RequestID: request, InputJSON: string(input), Digest: digest, TimeoutSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		AppletInvocation
		Delivery   string `json:"delivery"`
		NextAction string `json:"nextAction"`
	}
	if err := json.Unmarshal(data, &receipt); err != nil || receipt.ID == "" || receipt.Delivery != "automatic" || !strings.Contains(receipt.NextAction, "Do not poll") {
		t.Fatalf("missing asynchronous receipt: %s (%v)", data, err)
	}
	return receipt.AppletInvocation
}

func reconcileResults(t *testing.T, a *App, reported map[string]bool) {
	t.Helper()
	if err := a.busHost().With(func(b *bus.Bus) error {
		a.busCoordinator().ReconcileResults(context.Background(), b, reported)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func callerTurns(a *App, id string) chan map[string]any {
	turns := make(chan map[string]any, 16)
	a.sink = func(name string, data any) {
		if name == "agent:turn" && data.(map[string]any)["sessionId"] == id {
			turns <- data.(map[string]any)
		}
	}
	return turns
}

func TestAsyncQuestionWakesOnlyItsIdleCallerWithoutPolling(t *testing.T) {
	a, ws, chats := asyncChatTestApp(t)
	turns := callerTurns(a, chats["A"].ID())
	v := invokeChatQuestion(t, a, ws, chats["A"].ID(), chats["B"].ID(), "question-one")
	awaitApplet(t, a, ws, "chat", chats["A"].ID(), v.ID, "done")
	// Retrying the invocation after losing the receipt must not ask twice.
	again := invokeChatQuestion(t, a, ws, chats["A"].ID(), chats["B"].ID(), "question-one")
	if again.ID != v.ID {
		t.Fatal("the retry started another question")
	}
	reconcileResults(t, a, map[string]bool{})
	if waiting(t, a, chats["A"].BusID()) != 1 || waiting(t, a, chats["B"].BusID()) != 0 || waiting(t, a, chats["C"].BusID()) != 0 {
		t.Fatal("the reply was not private to the caller")
	}
	ring(t, a)
	waitTurn(t, turns)
	// A new reconciliation pass models a restart after the bus commit.
	reconcileResults(t, a, map[string]bool{})
	if waiting(t, a, chats["A"].BusID()) != 0 {
		t.Fatal("reconciliation redelivered a consumed reply")
	}
	c := a.runtime.ProviderLocked("codex").Connection().(*answeringConnection)
	c.mu.Lock()
	defer c.mu.Unlock()
	var caller, recipient int
	for _, call := range c.calls {
		if call.method != "session/prompt" {
			continue
		}
		p := call.params.(acp.PromptParams)
		switch p.SessionID {
		case chats["B"].ID():
			recipient++
		case chats["A"].ID():
			caller++
			if !strings.Contains(p.Prompt[0].Text, "Answer to: Which parser files changed?") || !strings.Contains(p.Prompt[0].Text, v.ID) {
				t.Fatalf("the caller did not get the answer and correlation ID: %+v", p)
			}
		}
	}
	if caller != 1 || recipient != 1 {
		t.Fatalf("wrong turn counts: caller %d, recipient %d", caller, recipient)
	}
}

// gatedChatConnection holds one provider call while another chat answers.
// No timing delay is used to decide whether the caller is still working.
type gatedChatConnection struct {
	agentConnection
	owner   string
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *gatedChatConnection) Call(ctx context.Context, method string, p, out any) error {
	if method == "session/prompt" {
		params := p.(acp.PromptParams)
		if params.SessionID == c.owner && params.Prompt[0].Text == "Active work" {
			c.once.Do(func() { close(c.started) })
			select {
			case <-c.release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return c.agentConnection.Call(ctx, method, p, out)
}

func TestAsyncReplyArrivesAfterActiveTurnBeforeQueuedWork(t *testing.T) {
	a, ws, chats := asyncChatTestApp(t)
	caller := chats["A"]
	turns := callerTurns(a, caller.ID())
	conn := &gatedChatConnection{agentConnection: a.runtime.ProviderLocked("codex").Connection(), owner: caller.ID(), started: make(chan struct{}), release: make(chan struct{})}
	setTestConnection(a, "codex", conn)
	if err := a.Send(caller.ID(), "Active work", nil, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-conn.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the active turn did not start")
	}
	if err := a.Queue(caller.ID(), "Queued work", nil, nil); err != nil {
		t.Fatal(err)
	}
	v := invokeChatQuestion(t, a, ws, caller.ID(), chats["B"].ID(), "busy-caller")
	awaitApplet(t, a, ws, "chat", caller.ID(), v.ID, "done")
	reconcileResults(t, a, map[string]bool{})
	ring(t, a)
	if waiting(t, a, caller.BusID()) != 1 {
		t.Fatal("the result interrupted the active turn")
	}
	select {
	case <-turns:
		t.Fatal("the active turn ended before it was released")
	default:
	}
	close(conn.release)
	for i := 0; i < 3; i++ {
		waitTurn(t, turns)
	}
	inner := conn.agentConnection.(*answeringConnection)
	inner.mu.Lock()
	defer inner.mu.Unlock()
	var texts []string
	for _, call := range inner.calls {
		if call.method == "session/prompt" {
			p := call.params.(acp.PromptParams)
			if p.SessionID == caller.ID() {
				texts = append(texts, p.Prompt[0].Text)
			}
		}
	}
	if len(texts) != 3 || texts[0] != "Active work" || !strings.Contains(texts[1], v.ID) || texts[2] != "Queued work" {
		t.Fatalf("the reply did not follow the active turn: %q", texts)
	}
}

func TestAsyncResultRecoveryIncludesFailureCancellationAndExpiry(t *testing.T) {
	for _, status := range []string{"done", "failed", "cancelled", "timeout"} {
		t.Run(status, func(t *testing.T) {
			a, ws, chats := asyncChatTestApp(t)
			v := AppletInvocation{ID: "saved-" + status, Workspace: ws, OwnerKind: "chat", OwnerID: chats["A"].ID(), Action: chatAskAction,
				InputJSON: `{"sessionId":"chat-B","question":"Which parser files changed?"}`, Status: status, Deadline: time.Now().Add(time.Minute)}
			if status == "done" {
				v.OutputJSON = `{"sessionId":"chat-B","title":"Chat B","reply":"parser.go","stopReason":"end_turn"}`
			} else {
				v.Error = "The request " + status
			}
			if err := saveAppletInvocation(&v); err != nil {
				t.Fatal(err)
			}
			// A fresh App has no in-memory completion events to rely on.
			restarted := NewApp()
			t.Cleanup(func() { restarted.busHost().CloseHandle() })
			restarted.settings.BusSessions = a.settings.BusSessions
			reconcileResults(t, restarted, map[string]bool{})
			if err := a.busHost().With(func(b *bus.Bus) error {
				msgs, _, err := b.Deliver(context.Background(), chats["A"].BusID(), bus.DoorbellTurn)
				if err != nil || len(msgs) != 1 || !strings.Contains(msgs[0].Text, status) || !strings.Contains(msgs[0].Text, v.ID) {
					t.Fatalf("the saved completion was lost: %+v %v", msgs, err)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A reply saved with its text reaches the caller as that text, and one saved
// before results had text, whose report the bus already holds, is enqueued
// again on reconciliation as the same report rather than refused.
func TestChatResultTextAndLegacyReenqueue(t *testing.T) {
	a, ws, chats := asyncChatTestApp(t)
	question := `{"sessionId":"chat-B","question":"Which parser files changed?"}`
	output := `{"sessionId":"chat-B","title":"Chat B","reply":"parser.go\n\nand lexer.go","stopReason":"end_turn"}`
	legacy := AppletInvocation{ID: "legacy-done", Workspace: ws, OwnerKind: "chat", OwnerID: chats["A"].ID(), Action: chatAskAction, InputJSON: question, OutputJSON: output, Status: "done", Deadline: time.Now().Add(time.Minute)}
	if err := saveAppletInvocation(&legacy); err != nil {
		t.Fatal(err)
	}
	// What the window queued for this result before reports had text.
	before := `{"invocationId":"legacy-done","request":` + question + `,"status":"done","output":` + output + `}`
	if err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.EnqueueResult(context.Background(), sessionhost.ChatResultKey(ws, legacy.ID), chats["A"].BusID(), before)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	current := legacy
	current.ID, current.Text = "with-text", askReplyText("Chat B", "chat-B", "parser.go\n\nand lexer.go")
	if err := saveAppletInvocation(&current); err != nil {
		t.Fatal(err)
	}
	for _, v := range []AppletInvocation{legacy, current} {
		if err := a.busHost().With(func(b *bus.Bus) error { return a.busCoordinator().EnqueueResult(context.Background(), b, v) }); err != nil {
			t.Fatalf("%s: %v", v.ID, err)
		}
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		msgs, _, err := b.Deliver(context.Background(), chats["A"].BusID(), bus.DoorbellTurn)
		if err != nil || len(msgs) != 2 {
			t.Fatalf("the results were not queued once each: %+v %v", msgs, err)
		}
		var report bus.HostReport
		if err := json.Unmarshal([]byte(msgs[1].Text), &report); err != nil || report.From != "Chat B" || report.Text != current.Text {
			t.Fatalf("the report lost its text: %+v %v", report, err)
		}
		if got := bus.FormatOne(msgs[1], bus.PersonFallback); !strings.Contains(got, "Request with-text is done.\n\nReply from the chat \"Chat B\" (session chat-B):\n\nparser.go\n\nand lexer.go") {
			t.Fatalf("the reply was written as %s", got)
		}
		if got := bus.FormatOne(msgs[0], bus.PersonFallback); !strings.Contains(got, "```json") || !strings.Contains(got, `"reply": "parser.go\n\nand lexer.go"`) {
			t.Fatalf("the legacy reply was written as %s", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAsyncCancelledQuestionIsReportedWithoutRunningQueuedRecipient(t *testing.T) {
	a, ws, chats := asyncChatTestApp(t)
	chats["B"].StartWork()
	v := invokeChatQuestion(t, a, ws, chats["A"].ID(), chats["B"].ID(), "cancel-question")
	waitQueued(t, chats["B"], 1)
	if err := a.cancelAppletInvocation(ws, "chat", chats["A"].ID(), v.ID); err != nil {
		t.Fatal(err)
	}
	waitQueued(t, chats["B"], 0)
	reconcileResults(t, a, map[string]bool{})
	if waiting(t, a, chats["A"].BusID()) != 1 {
		t.Fatal("the caller was not notified of cancellation")
	}
	// The legacy action must not acquire push semantics retroactively.
	v.ID, v.Action = "legacy-result", legacyChatAskAction
	if err := saveAppletInvocation(&v); err != nil {
		t.Fatal(err)
	}
	reconcileResults(t, a, map[string]bool{})
	if waiting(t, a, chats["A"].BusID()) != 1 {
		t.Fatal("a legacy invocation acquired automatic delivery")
	}
}

func TestEveryProviderTurnReceivesCommunicationProtocol(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "devin", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			a, c, s, _, turns := busChat(t)
			runtime := a.runtime.ProviderLocked("codex")
			a.runtime.PutProviderLocked(provider, runtime)
			s = replaceTestIdentity(a, s, "Provider", provider)
			if provider == "devin" {
				// Devin's transport is a concrete ACP subprocess rather than
				// the injectable connection used by the other adapters. Check
				// the shared prompt context here without starting a real agent.
				text, err := a.appletPromptContext(s.ID())
				if err != nil || !strings.Contains(text, communication.Window(bus.PersonFallback)) {
					t.Fatalf("Devin lost its communication instructions: %v", err)
				}
				return
			}
			for _, text := range []string{"First turn", "After a model change"} {
				if err := a.Send(s.ID(), text, nil, nil); err != nil {
					t.Fatal(err)
				}
				waitTurn(t, turns)
			}
			got := prompts(c)
			if len(got) != 2 {
				t.Fatalf("provider did not receive both turns: %q", got)
			}
			for _, text := range got {
				if !strings.Contains(text, communication.Window(bus.PersonFallback)) {
					t.Fatal("the provider lost its communication instructions")
				}
			}
		})
	}
}

// The broker recovers a once-only request after a crash as a failure. The
// caller receives that failure without asking the recipient another time.
func TestInterruptedAsyncQuestionIsRecoveredWithoutReplay(t *testing.T) {
	a, ws, chats := asyncChatTestApp(t)
	v := AppletInvocation{ID: "interrupted", Workspace: ws, OwnerKind: "chat", OwnerID: chats["A"].ID(), Action: chatAskAction,
		InputJSON: `{"sessionId":"chat-B","question":"Which parser files changed?"}`, Status: "running", Deadline: time.Now().Add(time.Minute)}
	if err := saveAppletInvocation(&v); err != nil {
		t.Fatal(err)
	}
	reconcileResults(t, a, map[string]bool{})
	got, err := readAppletInvocation(ws, v.ID)
	if err != nil || got.Status != "failed" || !broker.TerminalStatus(got.Status) || waiting(t, a, chats["A"].BusID()) != 1 {
		t.Fatalf("the interrupted request was not reported: %+v %v", got, err)
	}
	c := a.runtime.ProviderLocked("codex").Connection().(*answeringConnection)
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.calls) != 0 {
		t.Fatal("recovering the interrupted request replayed the question")
	}
}
