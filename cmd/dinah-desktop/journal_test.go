package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJournal(t *testing.T) {
	a := NewApp()
	a.sink = func(string, any) {}
	id := "journal-test-session"
	a.forgetJournal(id)
	defer a.forgetJournal(id)
	s := newSessionState(&sessionSeed{ID: id}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(id, s)
	update := func(kind string) {
		p, _ := json.Marshal(map[string]any{"sessionId": id, "update": map[string]any{"sessionUpdate": kind, "content": map[string]string{"type": "text", "text": "x"}, "toolCallId": "t"}})
		a.newAgentHandler().Notify("session/update", p)
	}
	// A replay is not journalled.
	s.SetReplaying(true)
	update("user_message_chunk")
	update("agent_message_chunk")
	s.EndReplay()
	// A live turn: the person's message, two chunks of one agent message, a
	// tool call, and a second agent message.
	a.journal(id, "you")
	update("agent_message_chunk")
	update("agent_message_chunk")
	update("tool_call")
	update("agent_message_chunk")
	var kinds []string
	for _, e := range a.SessionTimes(id) {
		kinds = append(kinds, e.Kind)
	}
	if got := strings.Join(kinds, " "); got != "you agent agent" {
		t.Fatalf("journal %q", got)
	}
	a.forgetJournal(id)
	if len(a.SessionTimes(id)) != 0 {
		t.Fatal("the journal outlived the session")
	}
}
