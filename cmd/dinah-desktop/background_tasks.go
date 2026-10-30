package main

import (
	"context"
	"errors"
	"time"

	"studio/internal/sessionhost"
)

// Background work is what an agent runs on after its turn has ended: a
// backgrounded command, a background subagent, a monitor, or a turn the
// agent starts of its own when such work finishes. The Claude adapter
// reports it through the AIR asyncTasks extension, which Dinah declares
// (acp.ClientCaps), as three session updates documented in the adapter's
// docs/air-extensions.md: async_task_spawned, async_task_progress (only the
// fields that changed) and async_task_state_update (running, paused,
// completed, failed or stopped). Dinah keeps the tasks that are not over,
// so that a chat whose turn ended still shows that its agent is busy, and
// so that the person can stop a task (Paul, 26 October 2026).

type BackgroundTask = sessionhost.BackgroundTask

const autonomousTurn = "turn"

// StopBackgroundTask asks the agent to stop one task of a chat's
// background work, through the extension's documented request.
func (a *App) StopBackgroundTask(sessionID, taskID string) error {
	s := a.session(sessionID)
	if s == nil {
		return errors.New("this chat is not connected to its agent")
	}

	t, found := s.BackgroundTask(taskID)
	canStop := found && t.CanStop
	provider := s.Provider()

	if !found {
		return errors.New("that background task has already finished")
	}
	if !canStop {
		return errors.New("the agent does not let that task be stopped")
	}
	conn, _, err := a.lifecycle().Live(normalizedProvider(provider))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var out struct {
		Stopped bool `json:"stopped"`
	}
	if err := conn.Call(ctx, "_session/async_task/stop", map[string]string{"sessionId": sessionID, "asyncTaskId": taskID}, &out); err != nil {
		return err
	}
	if !out.Stopped {
		return errors.New("the agent did not stop the task")
	}
	return nil
}
