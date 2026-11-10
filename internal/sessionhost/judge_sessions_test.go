package sessionhost

import (
	"context"
	"encoding/json"
	"testing"

	"studio/internal/acp"
)

func judgeUpdate(t *testing.T, session string, update map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]any{"sessionId": session, "update": update})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// On a connection only judgement calls use, a call's session gets what the agent
// says in it, a tool the agent starts ends the call once, and what the agent says
// in a session no call has is dropped.
func TestAJudgeHandlerGivesACallItsUpdatesAndEndsItOnATool(t *testing.T) {
	sessions := NewJudgeSessions()
	sessions.EndOnToolCall = true
	h := &JudgeHandler{Sessions: sessions}
	ended := 0
	sessions.Begin()
	s := sessions.Register("j1", func() { ended++ })

	h.Notify("session/update", judgeUpdate(t, "j1", map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "Answer"}}))
	h.Notify("session/update", judgeUpdate(t, "other", map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "Stray"}}))
	h.Notify("session/update", judgeUpdate(t, "j1", map[string]any{"sessionUpdate": "usage_update", "used": 120}))
	if used, reported := s.Usage(); s.Text() != "Answer" || used != 120 || !reported {
		t.Fatalf("text %q, used %d, reported %v", s.Text(), used, reported)
	}
	if ended != 0 || len(s.Acted()) != 0 {
		t.Fatalf("ended %d before any tool, acted %v", ended, s.Acted())
	}

	h.Notify("session/update", judgeUpdate(t, "j1", map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t1", "title": "Read notes.txt", "kind": "read"}))
	h.Notify("session/update", judgeUpdate(t, "j1", map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t2", "kind": "execute"}))
	h.Notify("session/update", judgeUpdate(t, "j1", map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t3"}))
	if got := s.Acted(); len(got) != 3 || got[0] != "Read notes.txt" || got[1] != "execute" || got[2] != "a tool" {
		t.Fatalf("acted %v", got)
	}
	if ended != 1 {
		t.Fatalf("the call was ended %d times", ended)
	}
}

// A set that does not end calls on a tool, which is the one the chats' connection
// uses, notes nothing when the agent starts one.
func TestAJudgeSessionIgnoresAToolWhenItsSetDoesNotEndOnOne(t *testing.T) {
	sessions := NewJudgeSessions()
	h := &JudgeHandler{Sessions: sessions}
	ended := 0
	sessions.Begin()
	s := sessions.Register("j1", func() { ended++ })
	h.Notify("session/update", judgeUpdate(t, "j1", map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t1", "title": "Run echo"}))
	if ended != 0 || len(s.Acted()) != 0 {
		t.Fatalf("ended %d, acted %v", ended, s.Acted())
	}
}

// Every request the agent makes of the client on such a connection is refused: a
// permission request in a call's session with the agent's own rejection, which
// ends the call, and anything else as a method the client does not have.
func TestAJudgeHandlerRefusesEveryRequest(t *testing.T) {
	sessions := NewJudgeSessions()
	sessions.EndOnToolCall = true
	h := &JudgeHandler{Sessions: sessions}
	ended := 0
	sessions.Begin()
	s := sessions.Register("j1", func() { ended++ })

	params, _ := json.Marshal(map[string]any{"sessionId": "j1", "options": []map[string]string{
		{"optionId": "yes", "name": "Allow", "kind": "allow_once"},
		{"optionId": "no", "name": "Reject", "kind": "reject_once"},
	}})
	res, rpcErr := h.Request(context.Background(), "session/request_permission", params)
	if rpcErr != nil {
		t.Fatalf("a permission request was an error: %+v", rpcErr)
	}
	b, _ := json.Marshal(res)
	want, _ := json.Marshal(acp.Selected("no"))
	if string(b) != string(want) {
		t.Fatalf("the permission request was answered %s", b)
	}
	if got := s.Refused(); len(got) != 1 || got[0] != "session/request_permission" || ended != 1 {
		t.Fatalf("refused %v, ended %d", got, ended)
	}
	// A tool after a refusal does not end the call a second time.
	h.Notify("session/update", judgeUpdate(t, "j1", map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t1", "title": "Run echo"}))
	if ended != 1 {
		t.Fatalf("the call was ended %d times", ended)
	}

	file, _ := json.Marshal(map[string]any{"sessionId": "j1", "path": "notes.txt"})
	if _, rpcErr := h.Request(context.Background(), "fs/read_text_file", file); rpcErr == nil || rpcErr.Code != acp.CodeMethodNotFound {
		t.Fatalf("a file read in a call's session: %+v", rpcErr)
	}
	stray, _ := json.Marshal(map[string]any{"sessionId": "other", "path": "notes.txt"})
	if _, rpcErr := h.Request(context.Background(), "fs/read_text_file", stray); rpcErr == nil || rpcErr.Code != acp.CodeMethodNotFound {
		t.Fatalf("a file read outside a call: %+v", rpcErr)
	}
}
