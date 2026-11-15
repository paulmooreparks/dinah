package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"studio/internal/applet"

	"studio/internal/applet/broker"
	"studio/internal/sessionhost"
	"studio/internal/taskhost"
)

func taskSuccess(identity string, v any) taskhost.Outcome {
	b, err := json.Marshal(v)
	if err != nil {
		return taskFailure(err)
	}
	return taskhost.Outcome{State: "succeeded", Identity: identity, Result: b}
}

func (b taskBackend) Execute(ctx context.Context, s taskhost.Scope, o taskhost.Operation, capture taskhost.Capture) taskhost.Outcome {
	a := b.app
	if err := ctx.Err(); err != nil {
		return taskhost.Outcome{State: "cancelled", Code: "cancelled", Reason: err.Error()}
	}
	ws, err := a.taskWorkspace(ctx, s)
	if err != nil {
		return taskFailure(err)
	}
	if taskRoomKind(o.Kind) {
		rooms := a.taskRooms()
		if rooms == nil {
			return taskFailure(taskRefusal("unsupported", "the complete room protocol is unavailable"))
		}
		if err := capture(taskhost.Progress{Dispatched: true}); err != nil {
			return taskFailure(err)
		}
		var result taskhost.RoomResult
		var roomErr error
		switch o.Kind {
		case "EnsureRoom":
			result, roomErr = rooms.Ensure(ctx, s, o, capture)
		case "ClaimRoom":
			result, roomErr = rooms.Claim(ctx, s, o, capture)
		case "PostRoom":
			result, roomErr = rooms.Post(ctx, s, o, capture)
		case "ObserveRoom":
			result, roomErr = rooms.Observe(ctx, s, o, capture)
		case "ReleaseRoom":
			result, roomErr = rooms.Release(ctx, s, o, capture)
		case "RecoverRoom":
			result, roomErr = rooms.Recover(ctx, s, o, capture)
		}
		if roomErr != nil {
			return taskhost.Outcome{State: "unknown", Code: "unknown_outcome", Identity: result.Identity, Result: result.Result, Reason: roomErr.Error()}
		}
		return taskhost.Outcome{State: "succeeded", Identity: result.Identity, Result: result.Result}
	}
	switch o.Kind {
	case "OpenChat":
		if o.Target == "" {
			source, _ := json.Marshal([]string{s.Service, s.Workspace, s.Task, o.ID})
			sum := sha256.Sum256(source)
			info, allocationErr := a.sessionsLifecycle().NewRecorded(sessionhost.AllocationIntent{
				Source: hex.EncodeToString(sum[:]), Workspace: ws.Path, Provider: o.Program,
				Kind: "task_binding", Revision: s.Generation,
			}, sessionhost.AllocationCallbacks{
				Dispatched: func() error {
					if err := ctx.Err(); err != nil {
						return err
					}
					return capture(taskhost.Progress{Dispatched: true})
				},
				Received: func(id string) error { return capture(taskhost.Progress{Identity: id}) },
				Finished: func(id string, err error) error {
					progress := taskhost.Progress{Identity: id, SetupFinished: true}
					if err != nil {
						progress.SetupError = err.Error()
					}
					return capture(progress)
				},
			})
			if allocationErr != nil {
				state := "unknown"
				var notCreated *sessionhost.AllocationNotCreatedError
				if errors.As(allocationErr, &notCreated) {
					state = "failed"
				}
				return taskhost.Outcome{State: state, Code: "allocation_failed", Identity: info.ID, Reason: allocationErr.Error()}
			}
			if err := a.holdTaskChat(ctx, s, o.ID, info.ID); err != nil {
				return taskhost.Outcome{State: "failed", Code: "unavailable", Identity: info.ID, Reason: err.Error()}
			}
			return taskSuccess(info.ID, info)
		}
		info, err := a.OpenSessionAt(ws.Path, o.Target)
		if err != nil {
			return taskhost.Outcome{State: "unknown", Code: "unknown_outcome", Reason: err.Error(), Identity: info.ID}
		}
		if err = capture(taskhost.Progress{Identity: info.ID}); err != nil {
			return taskhost.Outcome{State: "unknown", Code: "persistence_failed", Identity: info.ID, Reason: err.Error()}
		}
		if err := a.holdTaskChat(ctx, s, o.ID, info.ID); err != nil {
			return taskhost.Outcome{State: "failed", Code: "unavailable", Identity: info.ID, Reason: err.Error()}
		}
		return taskSuccess(info.ID, info)
	case "TakeTurn":
		var input struct{ Text, Label string }
		if err := json.Unmarshal(o.Input, &input); err != nil {
			return taskFailure(err)
		}
		if input.Text == "" {
			return taskFailure(taskRefusal("stale", "the turn has no input"))
		}
		info, err := a.OpenSessionAt(ws.Path, o.Target)
		if err != nil {
			return taskFailure(err)
		}
		if err = capture(taskhost.Progress{Identity: info.ID}); err != nil {
			return taskhost.Outcome{State: "unknown", Code: "persistence_failed", Identity: info.ID}
		}
		chat := a.session(info.ID)
		if chat == nil {
			return taskFailure(taskRefusal("unavailable", "the opened chat is no longer available"))
		}
		turnCtx, stop := context.WithCancel(ctx)
		turnDone := make(chan struct{})
		key := taskOperationKey(s, o.ID)
		a.taskServices.mu.Lock()
		if a.taskServices.turns == nil {
			a.taskServices.turns = map[string]taskOwnedTurn{}
		}
		a.taskServices.turns[key] = taskOwnedTurn{cancel: stop, done: turnDone}
		a.taskServices.mu.Unlock()
		defer func() {
			stop()
			close(turnDone)
			a.taskServices.mu.Lock()
			delete(a.taskServices.turns, key)
			a.taskServices.mu.Unlock()
		}()
		words, end := a.turnIO().TakeTurn(turnCtx, chat, input.Text, input.Label, func() error {
			if err := a.checkTaskChat(o.Target, false); err != nil {
				return err
			}
			return capture(taskhost.Progress{Dispatched: true})
		})
		if end.Err != nil {
			return taskhost.Outcome{State: "unknown", Code: "unknown_outcome", Identity: info.ID, Reason: end.Err.Error()}
		}
		if end.StopReason == "cancelled" {
			return taskhost.Outcome{State: "cancelled", Code: "cancelled", Identity: info.ID}
		}
		return taskSuccess(info.ID, map[string]string{"text": words, "stopReason": end.StopReason, "label": input.Label})
	case "UseModel":
		return a.useTaskModel(ctx, s, o, capture)
	case "CancelTurn", "ReleaseChat", "CancelAction":
		return a.finishTaskOperation(ctx, s, o, ws.Path)
	case "ReadOptions":
		return taskSuccess(o.Target, a.SessionOptions(o.Target))
	case "AskPerson":
		var input struct {
			Title   string
			Choices []string
		}
		if err := json.Unmarshal(o.Input, &input); err != nil {
			return taskFailure(err)
		}
		if input.Title == "" || len(input.Choices) < 1 || len(input.Choices) > 16 {
			return taskFailure(taskRefusal("stale", "a question needs a title and bounded choices"))
		}
		options := make([]taskhost.QuestionOption, 0, len(input.Choices))
		for _, choice := range input.Choices {
			if choice == "" {
				return taskFailure(taskRefusal("stale", "a question choice is empty"))
			}
			options = append(options, taskhost.QuestionOption{OptionID: choice, Name: choice, Kind: "allow_once"})
		}
		answer, err := a.askTaskQuestion(ctx, s, input.Title, options)
		if err != nil {
			return taskFailure(err)
		}
		for _, choice := range input.Choices {
			if answer == choice {
				return taskSuccess(o.ID, map[string]string{"answer": answer})
			}
		}
		return taskFailure(taskRefusal("not_authorized", "the answer is outside the permitted schema"))
	case "InvokeAction":
		host, err := a.appletHostService().Broker()
		if err != nil {
			return taskFailure(err)
		}
		digest, err := host.Registry.Digest(o.Target)
		if err != nil {
			return taskFailure(err)
		}
		if digest != o.Digest {
			return taskFailure(taskRefusal("stale", "the reviewed action implementation changed"))
		}
		action, err := host.Registry.Describe(o.Target)
		if err != nil {
			return taskFailure(err)
		}
		if applet.RequiresFreshApproval(action) {
			answer, err := a.askTaskQuestion(ctx, s, "Approve this invocation of "+o.Target+"?", []taskhost.QuestionOption{
				{OptionID: "allow_once", Name: "Approve This Invocation", Kind: "allow_once"},
				{OptionID: "reject_once", Name: "Don't Run", Kind: "reject_once"},
			})
			if err != nil || answer != "allow_once" {
				return taskFailure(taskRefusal("not_authorized", "this action requires fresh person approval"))
			}
		}
		key, _ := json.Marshal([]string{s.Service, s.Workspace, s.Task, o.ID})
		sum := sha256.Sum256(key)
		id := hex.EncodeToString(sum[:])
		if err = capture(taskhost.Progress{Identity: id}); err != nil {
			return taskFailure(err)
		}
		deadline, _ := ctx.Deadline()
		if deadline.IsZero() {
			return taskFailure(taskRefusal("deadline_exceeded", "the action has no task deadline"))
		}
		cancelled := make(chan struct{})
		stopCancel := context.AfterFunc(ctx, func() {
			defer close(cancelled)
			a.appletHostService().CancelOwner(ws.Path, "task", taskQuestionOwner(s))
		})
		defer func() {
			if !stopCancel() {
				<-cancelled
			}
		}()
		out, err := host.Run(ctx, ws.Path, "task", taskQuestionOwner(s), id, o.Target, string(o.Input), o.Digest, deadline)
		if err != nil {
			var state string
			v, readErr := host.Read(ws.Path, id)
			if readErr == nil {
				switch v.Status {
				case "done":
					return taskSuccess(id, json.RawMessage(v.OutputJSON))
				case "failed":
					if action.Replay != "once" {
						state = "failed"
					}
				}
			}
			if state == "" || errors.Is(err, broker.ErrRecovery) {
				state = "unknown"
			}
			return taskhost.Outcome{State: state, Code: "unknown_outcome", Identity: id, Reason: err.Error()}
		}
		return taskSuccess(id, json.RawMessage(out))
	default:
		return taskFailure(taskRefusal("unsupported", "this task operation has no complete host adapter"))
	}
}
