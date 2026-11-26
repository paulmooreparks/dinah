package main

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"studio/internal/sessionhost"
)

func TestBackgroundActivityAgreesWithExitAndSharedState(t *testing.T) {
	a, _ := providerTestApp(t)
	s := putChat(a, "claude:activity", sessionInitial{Open: true, Working: true})
	h := a.newAgentHandler()
	feed := func(update string) {
		h.Notify("session/update", json.RawMessage(`{"sessionId":"claude:activity","update":`+update+`}`))
	}
	check := func(state string, tasks int) {
		t.Helper()
		st := chatStateOf(a, s.ID())
		if st.Exec == nil || st.Exec.State != state || st.Exec.Tasks != tasks {
			t.Fatalf("activity: %+v, expected %s with %d tasks", st.Exec, state, tasks)
		}
		busy := s.State().WorkActivity().Busy()
		if (a.ExitStatus().Chats == 1) != busy || (a.workingChats() == 1) != busy {
			t.Fatal("exit/restart disagree with shared activity")
		}
		a.sendChatStates()
		if own := s.Info().State; own == nil || own.Exec == nil || *own.Exec != *st.Exec {
			t.Fatalf("session and shared event differ: %+v, %+v", own, st.Exec)
		}
	}
	feed(`{"sessionUpdate":"async_task_spawned","asyncTaskId":"tests","taskType":"shell","name":"Run tests"}`)
	feed(`{"sessionUpdate":"async_task_spawned","asyncTaskId":"build","taskType":"shell","name":"Build"}`)
	check(execWorking, 2)
	s.ReleaseWork()
	check(execBackground, 2)
	s.ConnectedBackground(time.Now().Add(-time.Hour))
	if s.IdleBackground(time.Now(), time.Minute) || s.ParkAt(time.Now) {
		t.Fatal("confirmed tasks were parked as idle")
	}
	s.SetHold(&sessionhost.ContextHold{Kind: "context"})
	check(execWaiting, 2)
	noteTurnResult(s.ID(), errors.New("turn failed"))
	check(execStuck, 2)
	noteTurnResult(s.ID(), nil)
	s.SetHold(nil)
	feed(`{"sessionUpdate":"async_task_state_update","asyncTaskId":"tests","state":"completed"}`)
	check(execBackground, 1)
	feed(`{"sessionUpdate":"async_task_spawned","asyncTaskId":"own-turn","taskType":"turn","name":"Claude is working"}`)
	check(execWorking, 1)
	feed(`{"sessionUpdate":"async_task_state_update","asyncTaskId":"own-turn","state":"completed"}`)
	check(execBackground, 1)
	feed(`{"sessionUpdate":"async_task_state_update","asyncTaskId":"build","state":"failed"}`)
	check(execIdle, 0)
	feed(`{"sessionUpdate":"async_task_progress","asyncTaskId":"build","summary":"late"}`)
	check(execIdle, 0)
	feed(`{"sessionUpdate":"async_task_spawned","asyncTaskId":"cancelled","taskType":"shell"}`)
	feed(`{"sessionUpdate":"async_task_state_update","asyncTaskId":"cancelled","state":"stopped"}`)
	check(execIdle, 0)
	feed(`{"sessionUpdate":"async_task_spawned","asyncTaskId":"disconnect","taskType":"shell"}`)
	s.ProviderStopped()
	feed(`{"sessionUpdate":"async_task_spawned","asyncTaskId":"late-spawn","taskType":"shell"}`)
	if st := chatStateOf(a, s.ID()); st.Exec != nil || a.ExitStatus().Chats != 0 || len(s.Info().Background) != 0 {
		t.Fatal("disconnected session acquired live task claims", st)
	}
	s = putChat(a, "claude:activity", sessionInitial{Open: true})
	feed(`{"sessionUpdate":"async_task_progress","asyncTaskId":"disconnect","summary":"old task"}`)
	check(execIdle, 0)
	s.SetReplaying(true)
	feed(`{"sessionUpdate":"async_task_spawned","asyncTaskId":"replayed","taskType":"shell"}`)
	s.SetReplaying(false)
	check(execIdle, 0)
}

func TestChildShellWorkOutlivesChildAndParentAndKeepsItsStopTarget(t *testing.T) {
	a, conn := providerTestApp(t)
	s := putChat(a, "claude:child-shell", sessionInitial{Open: true, Working: true})
	notify(a, s.ID(), spawnUpdate("child-shell-source", "Worker", "Build"))
	notify(a, "child-shell-source", `{"sessionUpdate":"async_task_spawned","asyncTaskId":"same-id","taskType":"shell","name":"Child build","canStop":true}`)
	notify(a, s.ID(), `{"sessionUpdate":"async_task_spawned","asyncTaskId":"same-id","taskType":"shell","name":"Root build"}`)
	collisionID, _ := json.Marshal([]string{"child-shell-source", "same-id"})
	collisionSpawn, _ := json.Marshal(map[string]any{"sessionUpdate": "async_task_spawned", "asyncTaskId": string(collisionID), "taskType": "shell", "name": "Opaque root ID"})
	notify(a, s.ID(), string(collisionSpawn))
	notify(a, s.ID(), `{"sessionUpdate":"subagent_state_update","subagentSessionId":"child-shell-source","state":"completed"}`)
	a.turnSubagentsEnded(s)
	s.ReleaseWork()
	if st := chatStateOf(a, s.ID()); st.Exec == nil || st.Exec.State != execBackground || st.Exec.Tasks != 3 {
		t.Fatalf("lost child task: %+v", st.Exec)
	}
	var childTask BackgroundTask
	for _, task := range s.Info().Background {
		if task.SourceSession != "" {
			childTask = task
		}
	}
	if childTask.SourceSession != "child-shell-source" || childTask.SourceID != "same-id" || childTask.ID == "same-id" {
		t.Fatalf("stop target = %+v", childTask)
	}
	// The stand-in refuses unknown calls, but records the actual stop request.
	if err := a.StopBackgroundTask(s.ID(), childTask.ID); err == nil {
		t.Fatal("stand-in unexpectedly accepted Stop")
	}
	calls := conn.calls.Snapshot()
	last := calls[len(calls)-1]
	target, ok := last.params.(map[string]string)
	if last.method != "_session/async_task/stop" || !ok || target["sessionId"] != "child-shell-source" || target["asyncTaskId"] != "same-id" {
		t.Fatalf("Stop did not use the source identity: %+v", last)
	}
	notify(a, "child-shell-source", `{"sessionUpdate":"async_task_state_update","asyncTaskId":"same-id","state":"completed"}`)
	if got := s.State().WorkActivity().Tasks; got != 2 {
		t.Fatalf("child completion cleared root task: %d", got)
	}
	notify(a, s.ID(), `{"sessionUpdate":"async_task_state_update","asyncTaskId":"same-id","state":"completed"}`)
	if got := s.State().WorkActivity().Tasks; got != 1 {
		t.Fatalf("completion cleared a different opaque root ID: %d", got)
	}
	collisionStop, _ := json.Marshal(map[string]any{"sessionUpdate": "async_task_state_update", "asyncTaskId": string(collisionID), "state": "completed"})
	notify(a, s.ID(), string(collisionStop))
	if s.State().WorkActivity().Busy() {
		t.Fatal("completed work retained busy state")
	}
}
