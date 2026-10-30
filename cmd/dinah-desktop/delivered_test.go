package main

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
	"studio/internal/bus"
)

// deliveredChat is a bus chat whose agent answers every prompt, with the
// agent:sent and replayed page events kept.
type deliveredChat struct {
	a     *App
	c     *testProviderConnection
	s     *Session
	me    bus.Session
	turns chan map[string]any
	mu    sync.Mutex
	sent  []any
	page  []json.RawMessage
}

func newDeliveredChat(t *testing.T) *deliveredChat {
	t.Helper()
	a, c, s, me, turns := busChat(t)
	setTestConnection(a, "codex", &answeringConnection{testProviderConnection: c, a: a})
	d := &deliveredChat{a: a, c: c, s: s, me: me, turns: turns}
	a.sink = func(name string, data any) {
		d.mu.Lock()
		defer d.mu.Unlock()
		switch name {
		case "agent:turn":
			turns <- data.(map[string]any)
		case "agent:sent":
			d.sent = append(d.sent, data)
		case "agent:updates":
			d.page = append(d.page, data.(map[string]any)["updates"].([]json.RawMessage)...)
		case "agent:update":
			d.page = append(d.page, data.(map[string]any)["update"].(json.RawMessage))
		}
	}
	return d
}

func (d *deliveredChat) saved(t *testing.T) []ConversationMessage {
	t.Helper()
	tr, err := readTranscript(d.s.ID())
	if err != nil {
		t.Fatal(err)
	}
	return tr.Messages
}

func roles(messages []ConversationMessage) string {
	var out []string
	for _, m := range messages {
		out = append(out, m.Role)
	}
	return strings.Join(out, ",")
}

// sentDelivered is the delivered entries of the last agent:sent event.
func (d *deliveredChat) sentDelivered(t *testing.T) []ConversationMessage {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.sent) == 0 {
		t.Fatal("the page was told of no prompt")
	}
	e, ok := d.sent[len(d.sent)-1].(map[string]any)
	if !ok || e["kind"] != "bus" {
		t.Fatalf("the last prompt was not the bus's: %#v", d.sent[len(d.sent)-1])
	}
	return e["delivered"].([]ConversationMessage)
}

// Each place the window delivers bus messages records each message as an
// entry of its own, with its sender, kind and readable text, and never as
// the person's words; the agent's prompt is unchanged.
func TestEveryDeliveryIsRecordedAsEntries(t *testing.T) {
	t.Run("idle chat", func(t *testing.T) {
		d := newDeliveredChat(t)
		sendTo(t, d.a, d.me, "Is the parser ready?")
		sendTo(t, d.a, d.me, "And the docs?")
		ring(t, d.a)
		waitTurn(t, d.turns)
		got := d.saved(t)
		if roles(got) != "delivered,delivered,agent,status" {
			t.Fatalf("the saved copy is %s", roles(got))
		}
		first, second := got[0], got[1]
		if first.Text != "Is the parser ready?" || first.Delivered.Kind != "text" || first.Delivered.From != bus.Human || first.Delivered.ID == 0 || first.Delivered.Time == "" {
			t.Fatalf("the first entry is %+v %+v", first, first.Delivered)
		}
		if !strings.Contains(first.Delivered.Prompt, "Dinah bus: 2 new messages.") || second.Delivered.Prompt != "" || second.Text != "And the docs?" {
			t.Fatalf("the delivery's prompt was not kept once: %+v %+v", first.Delivered, second.Delivered)
		}
		if p := prompts(d.c); len(p) != 1 || !strings.Contains(p[0], "Dinah bus: 2 new messages.") {
			t.Fatalf("the agent's prompt changed: %q", p)
		}
		if sent := d.sentDelivered(t); len(sent) != 2 || sent[0].Text != "Is the parser ready?" || sent[0].Delivered.Prompt != "" {
			t.Fatalf("the page was sent %+v", sent)
		}
		times := d.a.SessionTimes(d.s.ID())
		if len(times) == 0 || times[0].Kind != deliveredRole {
			t.Fatalf("the journal took the delivery as the person's message: %+v", times)
		}
	})
	t.Run("the person's prompt", func(t *testing.T) {
		d := newDeliveredChat(t)
		sendTo(t, d.a, d.me, "The roadmap moved.")
		if err := d.a.Send(d.s.ID(), "What changed today?", nil, nil); err != nil {
			t.Fatal(err)
		}
		waitTurn(t, d.turns)
		got := d.saved(t)
		if roles(got) != "you,delivered,agent,status" || got[0].Text != "What changed today?" || got[1].Text != "The roadmap moved." || !strings.Contains(got[1].Delivered.Prompt, "The roadmap moved.") {
			t.Fatalf("the saved copy is %s: %+v", roles(got), got)
		}
		if p := prompts(d.c); len(p) != 1 || !strings.Contains(p[0], "What changed today?") || !strings.Contains(p[0], "The roadmap moved.") {
			t.Fatalf("the agent's prompt changed: %q", p)
		}
		if sent := d.sentDelivered(t); len(sent) != 1 {
			t.Fatalf("the page was sent %+v", sent)
		}
	})
	t.Run("after a turn", func(t *testing.T) {
		d := newDeliveredChat(t)
		sendTo(t, d.a, d.me, "Arrived while you worked.")
		p := d.a.busCoordinator().AfterTurn(d.s)
		if p == nil || p.Label() != busChatLabel || len(p.Delivered()) != 1 {
			t.Fatalf("the messages after the turn were not a delivery: %+v", p)
		}

		d.s.StartWork()

		d.a.turnIO().Run(d.s, p)
		if got := d.saved(t); roles(got) != "delivered,agent,status" || got[0].Text != "Arrived while you worked." {
			t.Fatalf("the saved copy is %s", roles(got))
		}
	})
	t.Run("steering", func(t *testing.T) {
		d := newDeliveredChat(t)
		d.s.StartWork()
		setTestConnection(d.a, "codex", &steeringTestConnection{testProviderConnection: d.c, receive: func(p acp.PromptParams, delivered func()) error {
			delivered()
			return nil
		}})
		sendTo(t, d.a, d.me, "Steer this in.")
		ring(t, d.a)
		if got := d.saved(t); roles(got) != "delivered" || got[0].Text != "Steer this in." || got[0].Delivered.Prompt == "" {
			t.Fatalf("the steered delivery was saved as %s: %+v", roles(got), got)
		}
		if sent := d.sentDelivered(t); len(sent) != 1 {
			t.Fatalf("the page was sent %+v", sent)
		}
	})
}

// replay has the agent replay the prompts it was given, each block as a
// user message piece, each followed by the answer the saved copy holds, and
// then extra, then ends the replay.
func (d *deliveredChat) replay(t *testing.T, extra ...string) {
	t.Helper()
	var answers []string
	for _, m := range d.saved(t) {
		if m.Role == "agent" {
			answers = append(answers, m.Text)
		}
	}
	var calls []acp.PromptParams
	for _, call := range d.c.calls {
		if call.method == "session/prompt" {
			calls = append(calls, call.params.(acp.PromptParams))
		}
	}
	notify := func(update map[string]any) {
		data, _ := json.Marshal(map[string]any{"sessionId": d.s.ID(), "update": update})
		d.a.newAgentHandler().Notify("session/update", data)
	}
	d.mu.Lock()
	d.page = nil
	d.mu.Unlock()

	d.s.SetReplaying(true)

	d.a.transcriptCapture().Initialize(d.s, true)
	for i, p := range calls {
		for _, block := range p.Prompt {
			notify(map[string]any{"sessionUpdate": "user_message_chunk", "content": map[string]string{"type": "text", "text": block.Text}})
		}
		notify(map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": answers[i]}})
	}
	for _, text := range extra {
		notify(map[string]any{"sessionUpdate": "user_message_chunk", "content": map[string]string{"type": "text", "text": text}})
		notify(map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "Answer to: " + text}})
	}
	d.a.transcriptCapture().SettleReplay(d.s)
	d.a.flushReplay(d.s)

	d.s.SetReplaying(false)

	d.a.transcriptCapture().FinishReplay(d.s, true)
}

// pageKinds is the page's replayed updates, a word each: the kind, and for
// a person's message or a delivery the start of its text.
func (d *deliveredChat) pageKinds() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []string
	for _, raw := range d.page {
		var u struct {
			Kind    string                `json:"sessionUpdate"`
			Content struct{ Text string } `json:"content"`
			Message ConversationMessage   `json:"message"`
		}
		_ = json.Unmarshal(raw, &u)
		switch u.Kind {
		case "user_message_chunk":
			out = append(out, "you:"+u.Content.Text)
		case deliveredUpdate:
			out = append(out, "delivered:"+u.Message.Text)
		default:
			out = append(out, u.Kind)
		}
	}
	return out
}

// When the chat reopens, the agent replays each delivery as a user message.
// The replay is matched against the saved copy's own record of the prompt,
// so the page draws the deliveries as entries again, and the saved copy
// keeps them, also when the replay becomes the saved copy. A replayed user
// message the copy does not hold goes on as the agent sent it.
func TestReplayKeepsDeliveriesAsEntries(t *testing.T) {
	d := newDeliveredChat(t)
	sendTo(t, d.a, d.me, "Is the parser ready?")
	sendTo(t, d.a, d.me, "And the docs?")
	ring(t, d.a)
	waitTurn(t, d.turns)
	sendTo(t, d.a, d.me, "The roadmap moved.")
	if err := d.a.Send(d.s.ID(), "What changed today?", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, d.turns)
	before := d.saved(t)

	d.replay(t)
	got := strings.Join(d.pageKinds(), " | ")
	want := "delivered:Is the parser ready? | delivered:And the docs? | agent_message_chunk | you:What changed today? | delivered:The roadmap moved. | agent_message_chunk"
	if got != want {
		t.Fatalf("the page was given\n%s\nnot\n%s", got, want)
	}
	if after := d.saved(t); roles(after) != roles(before) {
		t.Fatalf("a matching replay changed the saved copy from %s to %s", roles(before), roles(after))
	}

	// A replay holding a message the copy lacks becomes the saved copy, and
	// keeps the deliveries as entries, with their prompts for the next replay.
	d.replay(t, "Unknown words")
	if kinds := d.pageKinds(); kinds[len(kinds)-2] != "you:Unknown words" {
		t.Fatalf("an unmatched user message was not passed on: %v", kinds)
	}
	after := d.saved(t)
	if roles(after) != "delivered,delivered,agent,you,delivered,agent,you,agent" {
		t.Fatalf("the replay's copy is %s", roles(after))
	}
	if after[0].Delivered == nil || !strings.Contains(after[0].Delivered.Prompt, "Dinah bus: 2 new messages.") || after[4].Delivered.Prompt == "" {
		t.Fatalf("the replay's copy lost the deliveries' prompts: %+v", after)
	}
	d.replay(t, "Unknown words")
	if got := strings.Join(d.pageKinds()[:6], " | "); got != want {
		t.Fatalf("the replay's copy no longer matches its replay: %s", got)
	}
}

// A handoff to another provider, and the summary kept beside it, attribute a
// delivered message to its sender and never to the person.
func TestHandoffAttributesDeliveries(t *testing.T) {
	history := []ConversationMessage{
		{Role: "you", Text: "Ask PUDL about spans.", Provider: "codex"},
		{Role: deliveredRole, Text: "Request ee23 is done.\n\nReply from the chat \"PUDL Development\" (session s):\n\nUse empty groups.", Provider: "codex",
			Delivered: &DeliveredMessage{ID: 9, Kind: "result", From: "PUDL Development", Time: "2026-10-26T10:00:00Z", Status: "done", Prompt: "Dinah bus: 1 new message."}},
		{Role: deliveredRole, Text: "Shipped.", Provider: "codex", Delivered: &DeliveredMessage{ID: 10, Kind: "text", From: "pudl-1", Workspace: "PUDL", Time: "2026-10-26T10:05:00Z"}},
	}
	text, _ := continuationContext(history, 1<<20)
	for _, want := range []string{
		"[you, codex]\nAsk PUDL about spans.",
		"[delivered by the bus, Reply from PUDL Development, sent 26 Oct 2026 10:00 UTC]\nRequest ee23 is done.",
		"[delivered by the bus, Message from pudl-1 (workspace PUDL), sent 26 Oct 2026 10:05 UTC]\nShipped.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("the handoff lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Dinah bus: 1 new message.") || strings.Contains(text, "[you, codex]\nRequest") {
		t.Fatalf("the handoff gave a delivery as the person's words:\n%s", text)
	}
	if summary := continuationSummary(history, 1<<20); !strings.Contains(summary, "[delivered by the bus, Message from pudl-1 (workspace PUDL), sent 26 Oct 2026 10:05 UTC] Shipped.") {
		t.Fatalf("the summary is\n%s", summary)
	}
}
