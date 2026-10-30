package codexagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
)

type testHandler struct {
	mu         sync.Mutex
	events     []json.RawMessage
	permission acp.RequestPermissionParams
	choice     string
}

func (h *testHandler) Notify(_ string, b json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, append(json.RawMessage{}, b...))
}
func (h *testHandler) Request(_ context.Context, _ string, b json.RawMessage) (any, *acp.Error) {
	_ = json.Unmarshal(b, &h.permission)
	return acp.Selected(h.choice), nil
}

// message is an app-server message as the fake server reads and writes it.
type message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *acp.Error      `json:"error,omitempty"`
}

func fake(t *testing.T, h acp.Handler, fn func(message, func(message))) *Conn {
	t.Helper()
	client, server := net.Pipe()
	c := connect(client, client, h, nil)
	go func() {
		sc := bufio.NewScanner(server)
		sc.Buffer(make([]byte, 4096), 1<<20)
		for sc.Scan() {
			var m message
			if json.Unmarshal(sc.Bytes(), &m) != nil {
				return
			}
			fn(m, func(r message) { b, _ := json.Marshal(r); _, _ = server.Write(append(b, '\n')) })
		}
	}()
	t.Cleanup(func() { c.Close(); server.Close(); <-c.Done() })
	return c
}
func answer(m message, v any) message { return message{ID: m.ID, Result: acp.Raw(v)} }
func catalogResult() any {
	return map[string]any{"data": []any{map[string]any{"id": "test", "model": "test", "displayName": "Test", "defaultReasoningEffort": "medium", "supportedReasoningEfforts": []any{map[string]string{"reasoningEffort": "medium", "description": "Medium"}}}}}
}

func TestSessionLifecycle(t *testing.T) {
	h := &testHandler{}
	var opened map[string]any
	var sentPrompt map[string]any
	c := fake(t, h, func(m message, send func(message)) {
		switch m.Method {
		case "initialize":
			var p map[string]any
			json.Unmarshal(m.Params, &p)
			if _, ok := p["clientInfo"]; !ok {
				t.Error("missing client info")
			}
			send(answer(m, map[string]any{}))
		case "initialized":
		case "config/read":
			send(answer(m, map[string]any{"config": map[string]any{}}))
		case "model/list":
			send(answer(m, catalogResult()))
		case "thread/start":
			json.Unmarshal(m.Params, &opened)
			send(answer(m, map[string]any{"thread": map[string]string{"id": "thread"}, "model": "test", "reasoningEffort": "medium"}))
		case "turn/start":
			json.Unmarshal(m.Params, &sentPrompt)
			send(answer(m, map[string]any{"turn": turn{ID: "turn", Status: "inProgress"}}))
			send(message{Method: "item/agentMessage/delta", Params: acp.Raw(map[string]string{"threadId": "thread", "itemId": "msg", "delta": "Hello"})})
			send(message{Method: "item/completed", Params: acp.Raw(map[string]any{"threadId": "thread", "item": map[string]string{"type": "agentMessage", "id": "msg", "text": "Hello"}})})
			send(message{Method: "turn/completed", Params: acp.Raw(map[string]any{"threadId": "thread", "turn": turn{ID: "turn", Status: "completed"}})})
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var init acp.InitializeResult
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, &init); err != nil {
		t.Fatal(err)
	}
	if !init.AgentCapabilities.LoadSession {
		t.Fatal("resume not advertised")
	}
	servers := []any{map[string]any{"name": "managed", "command": "dinah.exe", "args": []string{"applet-mcp"}, "env": []map[string]string{{"name": "TEST", "value": "value"}}}}
	var s acp.NewSessionResult
	if err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: "C:/test", McpServers: servers}, &s); err != nil {
		t.Fatal(err)
	}
	if s.SessionID != "codex:thread" || len(s.ConfigOptions) != 2 {
		t.Fatalf("session %#v", s)
	}
	if opened["sandbox"] != "workspace-write" || opened["approvalPolicy"] != "on-request" {
		t.Fatalf("unsafe config %#v", opened)
	}
	cfg := opened["config"].(map[string]any)["mcp_servers"].(map[string]any)["managed"].(map[string]any)
	if cfg["command"] != "dinah.exe" || cfg["required"] != true {
		t.Fatalf("MCP config %#v", cfg)
	}
	var r acp.PromptResult
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: s.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Hi"}}}, &r); err != nil {
		t.Fatal(err)
	}
	if r.StopReason != "end_turn" || sentPrompt["model"] != "test" {
		t.Fatalf("prompt %#v %#v", r, sentPrompt)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.events) != 1 || !bytes.Contains(h.events[0], []byte(`"sessionId":"codex:thread"`)) {
		t.Fatalf("events %s", h.events)
	}
}

func TestResumeReplayAndList(t *testing.T) {
	h := &testHandler{}
	c := fake(t, h, func(m message, send func(message)) {
		switch m.Method {
		case "thread/read":
			send(answer(m, map[string]any{"thread": thread{ID: "saved", Cwd: "C:/test"}}))
		case "model/list":
			send(answer(m, catalogResult()))
		case "thread/resume":
			var p map[string]any
			json.Unmarshal(m.Params, &p)
			if p["threadId"] != "saved" {
				t.Error("native ID not decoded")
			}
			send(answer(m, map[string]any{"thread": thread{ID: "saved", Turns: []turn{{Items: []json.RawMessage{acp.Raw(map[string]any{"id": "u", "type": "userMessage", "content": []any{map[string]string{"type": "text", "text": "Question"}}}), acp.Raw(map[string]string{"id": "a", "type": "agentMessage", "text": "Answer"})}}}}, "model": "test"}))
		case "thread/list":
			send(answer(m, map[string]any{"data": []thread{{ID: "saved", Name: "Saved Chat", Cwd: "C:/test", UpdatedAt: 100}}, "nextCursor": "next"}))
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Call(ctx, "session/load", acp.LoadSessionParams{SessionID: "codex:saved", Cwd: "C:/test"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(h.events) != 2 {
		t.Fatalf("replay events %d", len(h.events))
	}
	var list acp.ListSessionsResult
	if err := c.Call(ctx, "session/list", acp.ListSessionsParams{}, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Sessions) != 1 || list.Sessions[0].SessionID != "codex:saved" || list.NextCursor != "next" {
		t.Fatalf("list %#v", list)
	}
}

// Closing a session unsubscribes from its thread, and loading it again
// resumes the thread.
func TestCloseUnsubscribesAndLoadResumes(t *testing.T) {
	var mu sync.Mutex
	var methods []string
	c := fake(t, &testHandler{}, func(m message, send func(message)) {
		mu.Lock()
		methods = append(methods, m.Method)
		mu.Unlock()
		switch m.Method {
		case "initialize":
			send(answer(m, map[string]any{}))
		case "thread/read":
			send(answer(m, map[string]any{"thread": thread{ID: "saved", Cwd: "C:/test"}}))
		case "model/list":
			send(answer(m, catalogResult()))
		case "thread/resume":
			send(answer(m, map[string]any{"thread": thread{ID: "saved"}, "model": "test"}))
		case "thread/unsubscribe":
			var p map[string]any
			json.Unmarshal(m.Params, &p)
			if p["threadId"] != "saved" {
				t.Errorf("unsubscribed from %v", p["threadId"])
			}
			send(answer(m, map[string]string{"status": "unsubscribed"}))
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var init acp.InitializeResult
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, &init); err != nil {
		t.Fatal(err)
	}
	if init.AgentCapabilities.SessionCapabilities.Close == nil {
		t.Fatal("session/close is not advertised")
	}
	var notActive *acp.Error
	if err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: "codex:saved"}, nil); !errors.As(err, &notActive) || notActive.Code != acp.CodeResourceNotFound {
		t.Fatalf("closing a session that is not active answered %v", err)
	}
	if err := c.Call(ctx, "session/load", acp.LoadSessionParams{SessionID: "codex:saved", Cwd: "C:/test"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: "codex:saved"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: "codex:saved", Prompt: []acp.ContentBlock{{Type: "text", Text: "Hi"}}}, nil); err == nil {
		t.Fatal("a closed session took a prompt")
	}
	if err := c.Call(ctx, "session/load", acp.LoadSessionParams{SessionID: "codex:saved", Cwd: "C:/test"}, nil); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	resumes, unsubscribes := 0, 0
	for _, m := range methods {
		switch m {
		case "thread/resume":
			resumes++
		case "thread/unsubscribe":
			unsubscribes++
		}
	}
	if resumes != 2 || unsubscribes != 1 {
		t.Fatalf("resumed %d times and unsubscribed %d times: %v", resumes, unsubscribes, methods)
	}
}

func TestFailureAndCancellation(t *testing.T) {
	for _, status := range []string{"failed", "interrupted"} {
		t.Run(status, func(t *testing.T) {
			c := fake(t, &testHandler{}, func(m message, send func(message)) {
				if m.Method == "turn/start" {
					send(answer(m, map[string]any{"turn": turn{ID: "t"}}))
					if status == "failed" {
						send(message{Method: "turn/completed", Params: acp.Raw(map[string]any{"threadId": "thread", "turn": map[string]any{"id": "t", "status": "failed", "error": map[string]string{"message": "quota exhausted"}}})})
					}
				}
				if m.Method == "turn/interrupt" {
					send(answer(m, map[string]any{}))
					send(message{Method: "turn/completed", Params: acp.Raw(map[string]any{"threadId": "thread", "turn": turn{ID: "t", Status: "interrupted"}})})
				}
			})
			c.sessions["codex:thread"] = &session{streamed: map[string]bool{}}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			done := make(chan error, 1)
			var r acp.PromptResult
			go func() { done <- c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: "codex:thread"}, &r) }()
			if status == "interrupted" {
				for {
					c.mu.Lock()
					ready := c.sessions["codex:thread"].turn != ""
					c.mu.Unlock()
					if ready {
						break
					}
					select {
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					case <-time.After(time.Millisecond):
					}
				}
				if err := c.Notify("session/cancel", acp.CancelParams{SessionID: "codex:thread"}); err != nil {
					t.Fatal(err)
				}
			}
			err := <-done
			if status == "failed" {
				if err == nil || err.Error() != "quota exhausted" {
					t.Fatalf("error %v", err)
				}
			} else if err != nil || r.StopReason != "cancelled" {
				t.Fatalf("cancel result %#v %v", r, err)
			}
		})
	}
}

func TestApprovalScopeAndDenyUnknownChoice(t *testing.T) {
	for _, choice := range []string{"accept", "decline", "acceptForSession", "forged"} {
		h := &testHandler{choice: choice}
		c := &Conn{handler: h}
		res, err := c.request(context.Background(), "item/commandExecution/requestApproval", acp.Raw(map[string]string{"threadId": "thread", "itemId": "tool", "command": "run test", "reason": "Needs access"}))
		if err != nil {
			t.Fatal(err)
		}
		decision := res.(map[string]string)["decision"]
		want := choice
		if choice == "acceptForSession" || choice == "forged" {
			want = "cancel"
		}
		if decision != want || h.permission.SessionID != "codex:thread" {
			t.Fatalf("choice %s: %#v %#v", choice, res, h.permission)
		}
	}
}

func TestProtocolErrorsAndUnsupported(t *testing.T) {
	c := fake(t, &testHandler{}, func(m message, send func(message)) {
		send(message{ID: m.ID, Error: &acp.Error{Code: -1, Message: "sign in"}})
	})
	if err := c.Call(context.Background(), "account/read", map[string]any{}, nil); err == nil || err.Error() != "sign in" {
		t.Fatalf("error %v", err)
	}
	if err := c.Call(context.Background(), "session/delete", nil, nil); err == nil {
		t.Fatal("unsupported operation accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Conn.Call(ctx, "model/list", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v", err)
	}
	if _, err := mcpConfig([]any{map[string]string{"name": "x", "type": "sse"}}); err == nil {
		t.Fatal("SSE accepted")
	}
	if _, err := native("other"); err == nil {
		t.Fatal("foreign session accepted")
	}
}

// syncBuffer is a buffer the transport's reader and the test share.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestTraceOmitsCredentials(t *testing.T) {
	var output syncBuffer
	client, server := net.Pipe()
	c := connect(client, client, &testHandler{}, &output)
	t.Cleanup(func() { c.Close(); server.Close(); <-c.Done() })
	// The fake server reads whatever the connection sends it.
	go func() {
		sc := bufio.NewScanner(server)
		for sc.Scan() {
		}
	}()
	if err := c.Conn.Notify("thread/start", map[string]string{"token": "secret-value"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "secret-value") || !strings.Contains(output.String(), "send thread/start") {
		t.Fatalf("trace leaked credential or lost the method: %s", output.String())
	}
	line, _ := json.Marshal(message{Method: "unknown/request", ID: json.RawMessage(`"r1"`), Params: acp.Raw(map[string]string{"token": "incoming-secret", "message": "private question"})})
	if _, err := server.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(output.String(), `recv unknown/request "r1"`) {
		if time.Now().After(deadline) {
			t.Fatalf("missing request metadata: %s", output.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.Contains(output.String(), "incoming-secret") || strings.Contains(output.String(), "private question") {
		t.Fatal("incoming trace leaked payload")
	}
}

func TestManagedMCPRejectsConfigurationCollision(t *testing.T) {
	c := fake(t, &testHandler{}, func(m message, send func(message)) {
		if m.Method != "config/read" {
			t.Errorf("unexpected call after configuration conflict: %s", m.Method)
		}
		send(answer(m, map[string]any{"config": map[string]any{"mcp_servers": map[string]any{"managed": map[string]string{"url": "https://untrusted.invalid"}}}}))
	})
	err := c.Call(context.Background(), "session/new", acp.NewSessionParams{Cwd: "C:/test", McpServers: []any{map[string]string{"name": "managed", "command": "dinah.exe"}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "reserved MCP server") {
		t.Fatalf("collision error %v", err)
	}
}

func TestManagedMCPApprovalPolicyIsScoped(t *testing.T) {
	config, err := mcpConfig([]any{
		map[string]string{"name": "dinah-applets", "command": "dinah.exe"},
		map[string]string{"name": "external", "command": "external.exe"},
	})
	if err != nil {
		t.Fatal(err)
	}
	servers := config["mcp_servers"].(map[string]any)
	managed := servers["dinah-applets"].(map[string]any)
	if managed["default_tools_approval_mode"] != "writes" || managed["tool_timeout_sec"] != 30 {
		t.Fatalf("managed approval or timeout policy missing: %#v", managed)
	}
	external := servers["external"].(map[string]any)
	for _, key := range []string{"default_tools_approval_mode", "tool_timeout_sec", "tools"} {
		if _, exists := external[key]; exists {
			t.Fatalf("managed policy changed external server: %s", key)
		}
	}
	if _, exists := managed["tools"]; exists {
		t.Fatal("a per-tool override can bypass the writes policy")
	}
}

func TestSessionAppletApprovalPreference(t *testing.T) {
	for _, method := range []string{"session/new", "session/load"} {
		for _, mode := range []string{"", "ask", "allow", "all"} {
			t.Run(method+"/"+mode, func(t *testing.T) {
				c := fake(t, &testHandler{}, func(m message, send func(message)) {
					switch m.Method {
					case "thread/read":
						send(answer(m, map[string]any{"thread": thread{ID: "thread", Cwd: "C:/test"}}))
					case "config/read":
						send(answer(m, map[string]any{"config": map[string]any{}}))
					case "model/list":
						send(answer(m, catalogResult()))
					case "thread/start", "thread/resume":
						var opened map[string]any
						if err := json.Unmarshal(m.Params, &opened); err != nil {
							t.Error(err)
						}
						servers := opened["config"].(map[string]any)["mcp_servers"].(map[string]any)
						want := "writes"
						if mode == "allow" || mode == "all" {
							want = "approve"
						}
						if got := servers["dinah-applets"].(map[string]any)["default_tools_approval_mode"]; got != want {
							t.Errorf("policy %v, expected %v", got, want)
						}
						if _, ok := servers["external"].(map[string]any)["default_tools_approval_mode"]; ok {
							t.Error("external server policy changed")
						}
						wantSandbox, wantApproval := "workspace-write", "on-request"
						if mode == "all" {
							wantSandbox, wantApproval = "danger-full-access", "never"
						}
						if opened["approvalPolicy"] != wantApproval || opened["sandbox"] != wantSandbox {
							t.Error("global approval or sandbox changed")
						}
						if _, ok := opened["dinahAppletApprovals"]; ok {
							t.Error("internal option forwarded to app-server")
						}
						send(answer(m, map[string]any{"thread": thread{ID: "thread", Cwd: "C:/test"}, "model": "test"}))
					default:
						t.Errorf("unexpected method %s", m.Method)
					}
				})
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				params := SessionParams{LoadSessionParams: acp.LoadSessionParams{SessionID: "codex:thread", Cwd: "C:/test", McpServers: []any{map[string]string{"name": "dinah-applets", "command": "dinah.exe"}, map[string]string{"name": "external", "command": "external.exe"}}}, AppletApprovals: mode}
				if err := c.Call(ctx, method, params, nil); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestResumeRejectsDifferentWorkspace(t *testing.T) {
	c := fake(t, &testHandler{}, func(m message, send func(message)) {
		if m.Method != "thread/read" {
			t.Errorf("unexpected call %s", m.Method)
		}
		send(answer(m, map[string]any{"thread": thread{ID: "saved", Cwd: "C:/elsewhere"}}))
	})
	err := c.Call(context.Background(), "session/load", acp.LoadSessionParams{SessionID: "codex:saved", Cwd: "C:/wanted"}, nil)
	if err == nil || !strings.Contains(err.Error(), "different workspace") {
		t.Fatalf("workspace error %v", err)
	}
}

func TestSessionIDsCannotEscapeJournalPaths(t *testing.T) {
	for _, id := range []string{"codex:../x", "codex:C:/x", "codex:a:b", "codex:x\\y", "codex:", "devin:id"} {
		if _, err := native(id); err == nil {
			t.Errorf("accepted %q", id)
		}
	}
}
