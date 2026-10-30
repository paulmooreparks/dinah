package codexagent

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
)

func TestMCPElicitationUsesSchemaValues(t *testing.T) {
	cases := []struct {
		name, schema, selection, action string
		content                         map[string]any
	}{
		{"empty confirm", `{"type":"object","properties":{}}`, "accept", "accept", map[string]any{}},
		{"schema annotation", `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{}}`, "accept", "accept", map[string]any{}},
		{"empty decline", `{"type":"object","properties":{}}`, "decline", "decline", nil},
		{"boolean yes", `{"type":"object","properties":{"authorize":{"type":"boolean"}},"required":["authorize"]}`, "value_0", "accept", map[string]any{"authorize": true}},
		{"boolean no", `{"type":"object","properties":{"authorize":{"type":"boolean"}},"required":["authorize"]}`, "value_1", "accept", map[string]any{"authorize": false}},
		{"enum", `{"type":"object","properties":{"permission":{"type":"string","enum":["once","never"]}},"required":["permission"]}`, "value_0", "accept", map[string]any{"permission": "once"}},
		{"enum names", `{"type":"object","properties":{"permission":{"type":"string","enum":["once","never"],"enumNames":["Accept","Decline"]}},"required":["permission"]}`, "value_1", "accept", map[string]any{"permission": "never"}},
		{"titled choice", `{"type":"object","properties":{"scope":{"type":"string","oneOf":[{"const":"turn","title":"Accept"},{"const":"none","title":"Decline"}]}},"required":["scope"]}`, "value_0", "accept", map[string]any{"scope": "turn"}},
		{"optional omitted", `{"type":"object","properties":{"remember":{"type":"boolean"}}}`, "omit", "accept", map[string]any{}},
		{"required cannot omit", `{"type":"object","properties":{"remember":{"type":"boolean"}},"required":["remember"]}`, "omit", "cancel", nil},
		{"default does not approve", `{"type":"object","properties":{"approve":{"type":"boolean","default":true}},"required":["approve"]}`, "forged", "cancel", nil},
		{"cancel", `{"type":"object","properties":{"approve":{"type":"boolean"}},"required":["approve"]}`, "cancel", "cancel", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &testHandler{choice: tc.selection}
			c := &Conn{handler: h}
			res, err := c.request(context.Background(), "mcpServer/elicitation/request", acp.Raw(map[string]any{"mode": "form", "threadId": "thread", "serverName": "dinah-applets", "message": "Allow applet_list?", "requestedSchema": json.RawMessage(tc.schema)}))
			if err != nil {
				t.Fatal(err)
			}
			got := res.(map[string]any)
			if got["action"] != tc.action {
				t.Fatalf("action %v", got)
			}
			if tc.content == nil {
				if got["content"] != nil {
					t.Fatalf("unexpected content %v", got)
				}
			} else if !reflect.DeepEqual(got["content"], tc.content) {
				t.Fatalf("content %v, want %v", got["content"], tc.content)
			}
			var call struct {
				Title string `json:"title"`
			}
			json.Unmarshal(h.permission.ToolCall, &call)
			if h.permission.SessionID != "codex:thread" || !strings.Contains(call.Title, "dinah-applets") {
				t.Fatalf("permission not scoped: %#v", h.permission)
			}
		})
	}
}

func TestMCPElicitationApprovalWireResponse(t *testing.T) {
	h := &testHandler{choice: "accept"}
	response := make(chan message, 1)
	c := fake(t, h, func(m message, send func(message)) {
		switch m.Method {
		case "turn/start":
			send(answer(m, map[string]any{"turn": turn{ID: "turn", Status: "inProgress"}}))
			send(message{ID: json.RawMessage("0"), Method: "mcpServer/elicitation/request", Params: json.RawMessage(`{"threadId":"thread","turnId":"turn","serverName":"dinah-applets","mode":"form","message":"Allow applet_discover?","requestedSchema":{"type":"object","properties":{}}}`)})
		case "":
			response <- m
			send(message{Method: "turn/completed", Params: acp.Raw(map[string]any{"threadId": "thread", "turn": turn{ID: "turn", Status: "completed"}})})
		}
	})
	c.sessions["codex:thread"] = &session{streamed: map[string]bool{}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: "codex:thread"}, nil); err != nil {
		t.Fatal(err)
	}
	reply := <-response
	if string(reply.ID) != "0" || reply.Error != nil {
		t.Fatalf("invalid approval reply %#v", reply)
	}
	var got struct {
		Action  string         `json:"action"`
		Content map[string]any `json:"content"`
	}
	if err := json.Unmarshal(reply.Result, &got); err != nil {
		t.Fatal(err)
	}
	if got.Action != "accept" || got.Content == nil || len(got.Content) != 0 {
		t.Fatalf("invalid accepted form %s", reply.Result)
	}
}

func TestMCPElicitationUnsupportedFormsAreExplicit(t *testing.T) {
	for _, schema := range []string{
		`{"type":"object","properties":{"name":{"type":"string"}}}`,
		`{"type":"object","properties":{"count":{"type":"integer"}}}`,
		`{"type":"object","properties":{"choices":{"type":"array","items":{"type":"string","enum":["a"]}}}}`,
		`{"type":"object","properties":{"x":{"type":"string","enum":["a"],"enumNames":["A","B"]}}}`,
		`{"type":"object","properties":{},"required":["absent"]}`,
		`{"type":"object","properties":{"x":{"type":"boolean","unexpected":true}}}`,
	} {
		h := &testHandler{}
		c := &Conn{handler: h}
		_, err := c.request(context.Background(), "mcpServer/elicitation/request", acp.Raw(map[string]any{"mode": "form", "threadId": "thread", "requestedSchema": json.RawMessage(schema)}))
		if err == nil {
			t.Fatalf("accepted unsupported schema %s", schema)
		}
		if h.permission.SessionID != "" {
			t.Fatal("partially displayed an unsupported form")
		}
	}
	c := &Conn{}
	_, err := c.request(context.Background(), "mcpServer/elicitation/request", json.RawMessage(`{"mode":"url","threadId":"thread","url":"https://example.invalid"}`))
	if err == nil {
		t.Fatal("URL ceremony silently declined")
	}
}
