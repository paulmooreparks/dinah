package main

import (
	"encoding/json"
	"testing"
)

// The adapter's async task updates (docs/air-extensions.md in
// claude-agent-acp) become the chat's background work: a task is kept from
// its announcement until it is over, progress changes only what it carries,
// a turn of the agent's own is kept with its type, and the work holds exit.
func TestBackgroundWorkFollowsTheAdaptersReports(t *testing.T) {
	a := NewApp()
	s := newSessionState(&sessionSeed{ID: "claude:one", Provider: "claude"}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	feed := func(update string) bool {
		t.Helper()
		return s.BackgroundUpdate(json.RawMessage(update))
	}
	if feed(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hi"}}`) {
		t.Fatal("a message was taken for background work")
	}
	if !feed(`{"sessionUpdate":"async_task_progress","asyncTaskId":"ghost","summary":"x"}`) || len(s.Info().Background) != 0 {
		t.Fatal("progress for a task never announced was shown")
	}
	feed(`{"sessionUpdate":"async_task_spawned","asyncTaskId":"build","name":"Build Windows","taskType":"shell","description":"go build","showInTranscript":true,"canStop":true,"toolCallId":"tc1"}`)
	feed(`{"sessionUpdate":"async_task_spawned","asyncTaskId":"autonomous-turn-1","name":"Claude is working","taskType":"turn","showInTranscript":false,"canStop":true}`)
	feed(`{"sessionUpdate":"async_task_progress","asyncTaskId":"build","summary":"linking"}`)
	info := s.Info()
	if len(info.Background) != 2 || info.Background[0].ID != "build" || info.Background[0].Summary != "linking" || info.Background[0].Description != "go build" || !info.Background[0].CanStop || info.Background[1].Type != autonomousTurn {
		t.Fatalf("background work: %+v", info.Background)
	}
	if st := a.ExitStatus(); st.Chats != 1 {
		t.Fatalf("background work did not hold exit: %+v", st)
	}
	if n := a.workingChats(); n != 1 {
		t.Fatalf("background work did not hold the restart: %d", n)
	}
	feed(`{"sessionUpdate":"async_task_state_update","asyncTaskId":"autonomous-turn-1","state":"completed"}`)
	feed(`{"sessionUpdate":"async_task_state_update","asyncTaskId":"build","state":"paused"}`)
	if info := s.Info(); len(info.Background) != 1 || info.Background[0].State != "paused" {
		t.Fatalf("a paused task left, or a finished one stayed: %+v", info.Background)
	}
	feed(`{"sessionUpdate":"async_task_state_update","asyncTaskId":"build","state":"failed","summary":"exit 1"}`)
	if len(s.Info().Background) != 0 || a.ExitStatus().Chats != 0 {
		t.Fatal("a failed task was kept")
	}
	if err := a.StopBackgroundTask(s.ID(), "build"); err == nil {
		t.Fatal("a finished task could be stopped")
	}
}
