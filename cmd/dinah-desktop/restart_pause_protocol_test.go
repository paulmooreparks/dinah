package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/claudeagent"
	"studio/internal/codexagent"
)

// TestPauseCodexProtocolChild is a model-free app-server stand-in. It holds
// its turn until the documented turn/steer request arrives. Start hides the
// child process and no window runtime or desktop input is used.
func TestPauseCodexProtocolChild(t *testing.T) {
	if os.Getenv("PAUSE_CODEX_PROTOCOL_CHILD") != "1" {
		return
	}
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 4096), 8<<20)
	out := json.NewEncoder(os.Stdout)
	for sc.Scan() {
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Thread string                        `json:"threadId"`
				Turn   string                        `json:"expectedTurnId"`
				Input  []struct{ Type, Text string } `json:"input"`
			} `json:"params"`
		}
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			os.Exit(2)
		}
		answer := func(v any) { out.Encode(map[string]any{"id": m.ID, "result": v}) }
		notify := func(method string, v any) { out.Encode(map[string]any{"method": method, "params": v}) }
		switch m.Method {
		case "initialized":
		case "initialize":
			answer(map[string]any{})
		case "config/read":
			answer(map[string]any{"config": map[string]any{}})
		case "model/list":
			answer(map[string]any{"data": []any{map[string]any{"id": "fixture", "model": "fixture", "displayName": "Fixture", "defaultReasoningEffort": "medium", "supportedReasoningEfforts": []any{map[string]string{"reasoningEffort": "medium"}}}}})
		case "thread/start":
			answer(map[string]any{"thread": map[string]string{"id": "pause-fixture"}, "model": "fixture", "reasoningEffort": "medium"})
		case "turn/start":
			answer(map[string]any{"turn": map[string]string{"id": "running", "status": "inProgress"}})
			notify("turn/started", map[string]any{"threadId": "pause-fixture", "turn": map[string]string{"id": "running"}})
			notify("item/agentMessage/delta", map[string]string{"threadId": "pause-fixture", "turnId": "running", "itemId": "ready", "delta": "Fixture is working."})
		case "turn/steer":
			if m.Params.Thread != "pause-fixture" || m.Params.Turn != "running" || len(m.Params.Input) != 1 || m.Params.Input[0].Type != "text" || !strings.Contains(m.Params.Input[0].Text, fmt.Sprintf(pauseRequest, product.Name)) {
				os.Exit(3)
			}
			answer(map[string]string{"turnId": "running"})
			notify("turn/completed", map[string]any{"threadId": "pause-fixture", "turn": map[string]string{"id": "running", "status": "completed"}})
		case "turn/interrupt":
			os.Exit(4)
		default:
			answer(map[string]any{})
		}
	}
	os.Exit(0)
}

type pauseProtocolHandler struct{ ready chan struct{} }

func (h pauseProtocolHandler) Notify(_ string, raw json.RawMessage) {
	if strings.Contains(string(raw), "Fixture is working.") {
		select {
		case h.ready <- struct{}{}:
		default:
		}
	}
}
func (h pauseProtocolHandler) Request(context.Context, string, json.RawMessage) (any, *acp.Error) {
	return nil, &acp.Error{Code: -32601, Message: "fixture has no tools"}
}

func TestPauseAndRestartReachesTheCodexAppServerActiveTurn(t *testing.T) {
	a, _ := providerTestApp(t)
	t.Setenv("PAUSE_CODEX_PROTOCOL_CHILD", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 1)
	c, err := codexagent.Start(executable, []string{"-test.run=^TestPauseCodexProtocolChild$"}, t.TempDir(), pauseProtocolHandler{ready}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	ctx, cancel := context.WithTimeout(context.Background(), testWait)
	defer cancel()
	var init acp.InitializeResult
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, &init); err != nil {
		t.Fatal(err)
	}
	setTestConnection(a, "codex", c)
	setTestInitialization(a, "codex", init)
	turns := make(chan map[string]any, 8)
	a.sink = func(name string, value any) {
		if name == "agent:turn" {
			turns <- value.(map[string]any)
		}
	}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Keep the automatic restart watcher from closing the fixture runtime.
	finishWrite, err := a.admitWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer finishWrite()
	if err := a.Send(info.ID, "Work on the invented fixture.", nil, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if !a.CanSteer(info.ID, nil) {
		t.Fatal("loaded Codex session cannot be steered")
	}
	if err := a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	r := readPaused()
	if len(r.Chats) != 1 || r.Chats[0].Chat != info.ID || !r.Chats[0].Told {
		t.Fatalf("pause record: %+v", r)
	}
	if a.session(info.ID).State().Working {
		t.Fatal("the completed Codex turn remains working")
	}
	a.CancelRestart()
}

func TestPauseAndRestartReachesAdvertisedClaudeSteeringWithoutHooks(t *testing.T) {
	a, _ := providerTestApp(t)
	client, server := net.Pipe()
	c := &claudeagent.Conn{Conn: acp.Connect(client, client, pauseProtocolHandler{make(chan struct{}, 1)}, acp.Options{})}
	t.Cleanup(func() { c.Close(); server.Close(); <-c.Done() })
	ready := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(server)
		out := json.NewEncoder(server)
		var promptID json.RawMessage
		for sc.Scan() {
			var m struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
				Params struct {
					Session string             `json:"sessionId"`
					Prompt  []acp.ContentBlock `json:"prompt"`
					Meta    struct {
						Steering struct {
							Idle string `json:"idleBehavior"`
						} `json:"steering"`
					} `json:"_meta"`
				} `json:"params"`
			}
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Error(err)
				return
			}
			answer := func(v any) { out.Encode(map[string]any{"id": m.ID, "result": v}) }
			switch m.Method {
			case "initialize":
				answer(map[string]any{"protocolVersion": 1, "_meta": map[string]any{"steering": map[string]bool{"supported": true}}})
			case "session/new":
				answer(map[string]string{"sessionId": "pause-fixture"})
			case "session/prompt":
				promptID = m.ID
				close(ready)
			case "_session/steering":
				if m.Params.Session != "pause-fixture" || len(m.Params.Prompt) != 1 || !strings.Contains(m.Params.Prompt[0].Text, fmt.Sprintf(pauseRequest, product.Name)) || m.Params.Meta.Steering.Idle != "promptRequired" {
					t.Errorf("pause request: %+v", m.Params)
				}
				answer(map[string]string{"outcome": "injected"})
				out.Encode(map[string]any{"id": promptID, "result": map[string]string{"stopReason": "end_turn"}})
			case "session/cancel":
				t.Error("pause interrupted the Claude turn")
				return
			default:
				answer(map[string]any{})
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), testWait)
	defer cancel()
	var init acp.InitializeResult
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, &init); err != nil {
		t.Fatal(err)
	}
	a.runtime.PutProviderLocked("claude", newTestProvider(c, init))
	a.settings.DefaultProvider = "claude"
	turns := make(chan map[string]any, 8)
	a.sink = func(name string, value any) {
		if name == "agent:turn" {
			turns <- value.(map[string]any)
		}
	}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	finishWrite, err := a.admitWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer finishWrite()
	if a.turnIO().HooksReach(a.session(info.ID)) {
		t.Fatal("fixture unexpectedly has reached hooks")
	}
	if err := a.Send(info.ID, "Work on the invented fixture.", nil, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if !a.CanSteer(info.ID, nil) {
		t.Fatal("advertised Claude steering is unavailable")
	}
	if err := a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	r := readPaused()
	if len(r.Chats) != 1 || !r.Chats[0].Told || a.session(info.ID).State().Working {
		t.Fatalf("pause record: %+v", r)
	}
	a.CancelRestart()
}
