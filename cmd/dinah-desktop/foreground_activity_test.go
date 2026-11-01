package main

import (
	"encoding/json"
	"testing"

	"studio/internal/sessionhost"
)

func TestNextHumanTurnWithPendingToolRemainsWorkingAcrossProjections(t *testing.T) {
	a, _ := providerTestApp(t)
	s := putChat(a, "claude:foreground", sessionInitial{Open: true})
	first := sessionhost.NewPrompt("First", nil)
	s.StartTurn(&first)
	s.NextTurn(nil, nil, true)
	if st := chatStateOf(a, s.ID()); st.Exec.State != execIdle {
		t.Fatal(st)
	}
	second := sessionhost.NewPrompt("Approved", nil)
	s.AdmitSend(second)
	s.StartTurn(&second)
	s.ReleaseWork()
	a.newAgentHandler().Notify("session/update", json.RawMessage(`{"sessionId":"claude:foreground","update":{"sessionUpdate":"tool_call","toolCallId":"long-command","title":"Run tests","kind":"execute","status":"pending"}}`))
	a.sendChatStates()
	if st := chatStateOf(a, s.ID()); st.Exec.State != execWorking || !s.Info().Working || s.Info().State.Exec.State != execWorking {
		t.Fatal("native turn disappeared from session/shared state", st, s.Info())
	}
	if a.ExitStatus().Chats != 1 || a.workingChats() != 1 {
		t.Fatal("native turn disappeared from exit/restart readiness")
	}
	for _, entry := range a.chatDirectory() {
		if entry.SessionID == s.ID() && (!entry.Working || entry.Exec == nil || entry.Exec.State != execWorking) {
			t.Fatal("chat directory disagrees", entry)
		}
	}
	s.NextTurn(nil, nil, true)
	if st := chatStateOf(a, s.ID()); st.Exec.State != execIdle || a.ExitStatus().Chats != 0 {
		t.Fatal("completed second turn stayed busy", st)
	}
}
