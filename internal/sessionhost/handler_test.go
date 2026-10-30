package sessionhost

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"studio/internal/acp"
)

func handlerFixture(t *testing.T) (*Handler, *Conversation, *[]string) {
	t.Helper()
	c := NewConversation(Identity{ID: "chat", Provider: "codex"}, InitialActivity{Open: true}, false, false)
	order := []string{}
	note := func(step string) {
		if !c.mu.TryLock() {
			t.Fatalf("host callback %s holds conversation mutex", step)
		}
		c.mu.Unlock()
		order = append(order, step)
	}
	h := NewHandler(HandlerHost{
		RedactContexts:          func(s string) string { note("redact"); return strings.ReplaceAll(s, "secret", "redacted") },
		VisibleConversationText: func(s string) string { return s },
		SubagentUpdate: func(_ string, raw json.RawMessage) bool {
			note("subagent")
			if strings.Contains(string(raw), "secret") {
				t.Fatal("subagent received unredacted update")
			}
			return false
		},
		LookupConversation:       func(string) *Conversation { note("lookup"); return c },
		ReplayBoundary:           func(*Conversation, string) { note("boundary") },
		RecordConversationUpdate: func(*Conversation, json.RawMessage, bool) *LoggedEvent { note("transcript"); return nil },
		LogUpdate:                func(string, json.RawMessage) { note("log") },
		Journal:                  func(string, string) { note("journal") },
		Emit:                     func(name string, _ any) { note(name) },
		ReplayBatch:              200, ProductName: "Dinah",
	})
	return h, c, &order
}
func notifyHandler(h *Handler, update string) {
	h.Notify("session/update", json.RawMessage(`{"sessionId":"chat","update":`+update+`}`))
}

func TestHandlerUpdateRedactionAndEffectOrder(t *testing.T) {
	h, _, order := handlerFixture(t)
	notifyHandler(h, `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"secret"}}`)
	want := []string{"redact", "subagent", "lookup", "boundary", "transcript", "log", "journal", "agent:update"}
	if !reflect.DeepEqual(*order, want) {
		t.Fatalf("order %v, want %v", *order, want)
	}
}
func TestHandlerCommandsCopyAndDisplayEventOrder(t *testing.T) {
	h, c, order := handlerFixture(t)
	h.host.SetCommands = func(commands []Command) { *order = append(*order, "commands"); commands[0].Name = "mutated" }
	h.host.Emit = func(name string, value any) {
		*order = append(*order, name)
		if name == "agent:commands" && value.(map[string]any)["commands"].([]Command)[0].Name != "tool:command" {
			t.Fatal("command callback changed event values")
		}
	}
	notifyHandler(h, `{"sessionUpdate":"available_commands_update","availableCommands":[{"name":"tool:command","description":"desc","input":{"hint":"hint"}}]}`)
	if got := (*order)[len(*order)-2:]; !reflect.DeepEqual(got, []string{"commands", "agent:commands"}) {
		t.Fatalf("command order %v", got)
	}
	*order = nil
	h.host.ResolveTitle = func(string, string) string { *order = append(*order, "title"); return "shown" }
	h.host.PublishBusTitle = func(*Conversation) {
		*order = append(*order, "publish")
		if c.Title() != "shown" {
			t.Fatal("published stale title")
		}
	}
	notifyHandler(h, `{"sessionUpdate":"session_info_update","title":"provided","updatedAt":"now"}`)
	if got := (*order)[len(*order)-3:]; !reflect.DeepEqual(got, []string{"title", "agent:session", "publish"}) {
		t.Fatalf("title order %v", got)
	}
	if c.UpdatedAt() != "now" {
		t.Fatal("updated timestamp lost")
	}
	raw, _ := json.Marshal(Command{Name: "x", Source: "devin"})
	if string(raw) != `{"name":"x","description":"","source":"devin"}` {
		t.Fatalf("command JSON changed: %s", raw)
	}
}
func TestHandlerPermissionCancellationAndRedaction(t *testing.T) {
	h, c, order := handlerFixture(t)
	h.host.ResolveRequestChat = func(string) (*Conversation, *SubagentInfo) {
		*order = append(*order, "owner")
		return c, &SubagentInfo{ID: "child"}
	}
	h.host.AskAsSubagent = func(_ context.Context, chat *Conversation, sub *SubagentInfo, tool json.RawMessage, _ []acp.PermissionOption) (string, bool) {
		*order = append(*order, "ask")
		if chat != c || sub.ID != "child" || strings.Contains(string(tool), "secret") {
			t.Fatal("permission attribution or redaction changed")
		}
		return "allow", true
	}
	params := json.RawMessage(`{"sessionId":"child","toolCall":{"title":"secret"}}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := h.Request(ctx, "session/request_permission", params)
	if err != nil || !reflect.DeepEqual(got, acp.Cancelled()) || len(*order) != 0 {
		t.Fatal("cancelled permission reached host")
	}
	got, err = h.Request(context.Background(), "session/request_permission", params)
	if err != nil || !reflect.DeepEqual(got, acp.Selected("allow")) {
		t.Fatalf("permission result %v, %v", got, err)
	}
	if !reflect.DeepEqual(*order, []string{"redact", "owner", "ask"}) {
		t.Fatalf("permission order %v", *order)
	}
}
func TestHandlerFilePolicyBeforeEvent(t *testing.T) {
	h, c, order := handlerFixture(t)
	h.host.ResolveRequestChat = func(string) (*Conversation, *SubagentInfo) { return c, &SubagentInfo{ID: "child", Name: "worker"} }
	h.host.DisplayPath = func(string, string) string { return "shown.txt" }
	fail := true
	h.host.WriteTextFile = func(string, string) error {
		*order = append(*order, "write")
		if fail {
			return errors.New("protected")
		}
		return nil
	}
	h.host.Emit = func(name string, v any) {
		*order = append(*order, name)
		data := v.(map[string]any)
		if data["sessionId"] != "chat" || data["bytes"] != 2 || data["subagent"].(map[string]string)["id"] != "child" {
			t.Fatal("file attribution changed")
		}
	}
	params := json.RawMessage(`{"sessionId":"child","path":"file.txt","content":"ok"}`)
	_, err := h.Request(context.Background(), "fs/write_text_file", params)
	if err == nil || err.Message != "protected" || err.Code != acp.CodeInternal || !reflect.DeepEqual(*order, []string{"write"}) {
		t.Fatalf("write failure %v, %v", err, *order)
	}
	fail = false
	*order = nil
	_, err = h.Request(context.Background(), "fs/write_text_file", params)
	if err != nil || !reflect.DeepEqual(*order, []string{"write", "agent:file"}) {
		t.Fatalf("write order %v, %v", err, *order)
	}
	_, err = h.Request(context.Background(), "missing", nil)
	if err == nil || err.Code != acp.CodeMethodNotFound || err.Message != "Dinah does not offer missing" {
		t.Fatalf("unknown method %v", err)
	}
}

// Every usage update the engine forwards says whether its window is known: the
// size an ACP agent reports is, and an adapter that says it is not keeps saying
// so.
func TestEveryUsageUpdateSaysWhetherItsWindowIsKnown(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{"sessionUpdate":"usage_update","used":10,"size":100}`, `"windowKnown":true`},
		{`{"sessionUpdate":"usage_update","used":10,"size":100,"windowKnown":false}`, `"windowKnown":false`},
		{`{"sessionUpdate":"usage_update","used":10,"size":100,"windowKnown":true}`, `"windowKnown":true`},
	} {
		h, _, _ := handlerFixture(t)
		var seen string
		h.host.LogUpdate = func(_ string, raw json.RawMessage) { seen = string(raw) }
		notifyHandler(h, tc.in)
		if !strings.Contains(seen, tc.want) {
			t.Errorf("%s was forwarded as %s", tc.in, seen)
		}
	}
	// An update with no size is left as it is.
	h, _, _ := handlerFixture(t)
	var seen string
	h.host.LogUpdate = func(_ string, raw json.RawMessage) { seen = string(raw) }
	notifyHandler(h, `{"sessionUpdate":"usage_update","used":10,"cost":{"amount":1,"currency":"USD"}}`)
	if strings.Contains(seen, "windowKnown") {
		t.Errorf("an update with no size was given a window: %s", seen)
	}
}
