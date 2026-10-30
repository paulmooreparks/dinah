package codexagent

import (
	"studio/internal/acp"

	"context"
	"encoding/json"
	"testing"
)

func TestApproveEverythingRequests(t *testing.T) {
	h := &testHandler{choice: "decline"}
	c := &Conn{handler: h, sessions: map[string]*session{"codex:thread": {approvals: "ask", baseApprovals: "ask"}}}
	c.SetApprovals("all")
	for _, method := range []string{"item/commandExecution/requestApproval", "item/fileChange/requestApproval"} {
		v, err := c.request(context.Background(), method, acp.Raw(map[string]any{"threadId": "thread", "command": "echo hello", "availableDecisions": []string{"accept", "decline"}}))
		if err != nil || v.(map[string]string)["decision"] != "accept" {
			t.Fatalf("%s: %v %v", method, v, err)
		}
	}
	if h.permission.SessionID != "" {
		t.Fatal("auto approval asked the host")
	}
	for _, command := range []string{"dinah credential get", "dinah flow answer run"} {
		v, err := c.request(context.Background(), "item/commandExecution/requestApproval", acp.Raw(map[string]any{"threadId": "thread", "command": command}))
		if err != nil || v.(map[string]string)["decision"] != "decline" {
			t.Fatalf("reserved command: %v %v", v, err)
		}
	}
	permissions := json.RawMessage(`{"network":{"enabled":true}}`)
	v, err := c.request(context.Background(), "item/permissions/requestApproval", acp.Raw(map[string]any{"threadId": "thread", "permissions": permissions}))
	if err != nil || string(v.(map[string]any)["permissions"].(json.RawMessage)) != string(permissions) || v.(map[string]any)["scope"] != "turn" {
		t.Fatalf("permissions: %v %v", v, err)
	}
	v, err = c.request(context.Background(), "item/commandExecution/requestApproval", acp.Raw(map[string]any{"threadId": "thread", "command": "echo hello", "availableDecisions": []string{"decline"}}))
	if err != nil || v.(map[string]string)["decision"] != "decline" {
		t.Fatalf("unavailable acceptance: %v %v", v, err)
	}
	c.SetApprovals("ask")
	if c.sessions["codex:thread"].approvals != "ask" {
		t.Fatal("open chat did not restore asking")
	}
}

func TestLiveApprovalsOverrideNextTurn(t *testing.T) {
	var seen []map[string]any
	c := fake(t, &testHandler{}, func(m message, send func(message)) {
		if m.Method != "turn/start" {
			t.Errorf("unexpected method %s", m.Method)
			return
		}
		var q map[string]any
		_ = json.Unmarshal(m.Params, &q)
		seen = append(seen, q)
		send(answer(m, map[string]any{"turn": turn{ID: "turn"}}))
		send(message{Method: "turn/completed", Params: acp.Raw(map[string]any{"threadId": "thread", "turn": turn{ID: "turn", Status: "completed"}})})
	})
	c.sessions["codex:thread"] = &session{approvals: "ask", baseApprovals: "ask"}
	for _, mode := range []string{"all", "ask"} {
		c.SetApprovals(mode)
		if err := c.prompt(context.Background(), map[string]any{"sessionId": "codex:thread", "prompt": []any{}}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 2 {
		t.Fatal("missing turns")
	}
	if seen[0]["approvalPolicy"] != "never" || seen[0]["sandboxPolicy"].(map[string]any)["type"] != "dangerFullAccess" {
		t.Fatalf("full access not applied: %v", seen[0])
	}
	if seen[1]["approvalPolicy"] != "on-request" || seen[1]["sandboxPolicy"].(map[string]any)["type"] != "workspaceWrite" {
		t.Fatalf("sandbox not restored: %v", seen[1])
	}
}
