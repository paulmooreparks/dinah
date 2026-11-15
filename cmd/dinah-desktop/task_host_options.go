package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"studio/internal/sessionhost"
	"studio/internal/taskhost"
)

func taskOperationKey(s taskhost.Scope, operation string) string {
	key, _ := json.Marshal([]string{s.Service, s.Workspace, s.Task, operation})
	return string(key)
}

func (a *App) recordedTaskOperation(ctx context.Context, scope taskhost.Scope, id, kind, target string) (taskhost.Receipt, error) {
	a.taskServices.mu.Lock()
	host := a.taskServices.host
	a.taskServices.mu.Unlock()
	if host == nil || id == "" {
		return taskhost.Receipt{}, taskRefusal("not_authorized", "an operation receipt from this task is required")
	}
	receipt, err := host.Read(ctx, scope, id)
	if err != nil {
		return taskhost.Receipt{}, err
	}
	targetMatches := receipt.Target == target || kind == "OpenChat" && receipt.Target == "" && receipt.Identity == target
	if receipt.Scope != scope || receipt.Kind != kind || !targetMatches {
		return taskhost.Receipt{}, taskRefusal("not_authorized", "the operation receipt belongs to different work")
	}
	return receipt, nil
}

func (a *App) useTaskModel(ctx context.Context, scope taskhost.Scope, operation taskhost.Operation, capture taskhost.Capture) taskhost.Outcome {
	var input struct {
		Option           string          `json:"option"`
		Value            json.RawMessage `json:"value"`
		RestoreOperation string          `json:"restoreOperation"`
	}
	if err := json.Unmarshal(operation.Input, &input); err != nil {
		return taskFailure(err)
	}
	save := func(change sessionhost.OptionChange) error {
		result, err := json.Marshal(change)
		if err != nil {
			return err
		}
		return capture(taskhost.Progress{Identity: operation.Target, Dispatched: true, Result: result})
	}
	var change sessionhost.OptionChange
	var err error
	if input.RestoreOperation != "" {
		receipt, readErr := a.recordedTaskOperation(ctx, scope, input.RestoreOperation, "UseModel", operation.Target)
		if readErr != nil {
			return taskFailure(readErr)
		}
		var original sessionhost.OptionChange
		if json.Unmarshal(receipt.Result, &original) != nil || original.SessionID != operation.Target || !original.Applied {
			return taskFailure(taskRefusal("not_authorized", "the stored model change is not a restoration receipt"))
		}
		_, change, err = a.optionService().RestoreRecorded(original, save)
	} else {
		model := false
		for _, option := range a.SessionOptions(operation.Target) {
			if option.ID == input.Option && option.Category == "model" {
				model = true
			}
		}
		if !model {
			return taskFailure(taskRefusal("unsupported", "the selected option is not an offered model choice"))
		}
		_, change, err = a.optionService().SetRecorded(operation.Target, input.Option, input.Value, save)
	}
	if err != nil {
		if change.Revision == "" || errors.Is(err, sessionhost.ErrOptionsChanged) || errors.Is(err, sessionhost.ErrOptionBusy) {
			return taskFailure(err)
		}
		return taskhost.Outcome{State: "unknown", Code: "unknown_outcome", Identity: operation.Target, Reason: err.Error()}
	}
	return taskSuccess(operation.Target, change)
}

func (a *App) finishTaskOperation(ctx context.Context, scope taskhost.Scope, operation taskhost.Operation, workspace string) taskhost.Outcome {
	var input struct {
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(operation.Input, &input); err != nil {
		return taskFailure(err)
	}
	kind := map[string]string{"CancelTurn": "TakeTurn", "ReleaseChat": "OpenChat", "CancelAction": "InvokeAction"}[operation.Kind]
	receipt, err := a.recordedTaskOperation(ctx, scope, input.Operation, kind, operation.Target)
	if err != nil {
		return taskFailure(err)
	}
	switch operation.Kind {
	case "CancelTurn":
		a.taskServices.mu.Lock()
		turn := a.taskServices.turns[taskOperationKey(scope, input.Operation)]
		a.taskServices.mu.Unlock()
		if turn.cancel != nil {
			turn.cancel()
			timer := time.NewTimer(2 * time.Second)
			defer timer.Stop()
			select {
			case <-turn.done:
			case <-timer.C:
				return taskhost.Outcome{State: "unknown", Code: "unknown_outcome", Identity: receipt.Identity, Reason: "The cancellation request did not settle within its cleanup budget."}
			case <-ctx.Done():
				return taskhost.Outcome{State: "unknown", Code: "unknown_outcome", Identity: receipt.Identity, Reason: ctx.Err().Error()}
			}
		}
		return taskSuccess(receipt.Identity, map[string]any{"requested": turn.cancel != nil, "receipt": receipt.ID})
	case "CancelAction":
		if receipt.Identity == "" {
			return taskFailure(taskRefusal("unknown_outcome", "no broker identity was established"))
		}
		if err := a.appletHostService().CancelKnown(workspace, "task", taskQuestionOwner(scope), receipt.Identity); err != nil {
			return taskFailure(err)
		}
		return taskSuccess(receipt.Identity, map[string]any{"requested": true, "receipt": receipt.ID})
	case "ReleaseChat":
		a.taskServices.mu.Lock()
		release := a.taskServices.interests[taskOperationKey(scope, input.Operation)]
		a.taskServices.mu.Unlock()
		if release != nil {
			release()
		}
		return taskSuccess(receipt.Identity, map[string]any{"released": release != nil, "nativeRelease": "deferred", "reason": "The shared runtime retains the chat under its normal closed-session policy."})
	}
	return taskFailure(taskRefusal("unsupported", "unknown task cleanup operation"))
}

func (a *App) holdTaskChat(ctx context.Context, scope taskhost.Scope, operation, id string) error {
	chat := a.session(id)
	if chat == nil {
		return taskRefusal("unavailable", "the task chat is no longer loaded")
	}
	release, err := chat.HoldRuntime()
	if err != nil {
		return err
	}
	key := taskOperationKey(scope, operation)
	var once sync.Once
	finish := func() {
		once.Do(func() {
			release()
			a.taskServices.mu.Lock()
			delete(a.taskServices.interests, key)
			a.taskServices.mu.Unlock()
		})
	}
	a.taskServices.mu.Lock()
	if a.taskServices.interests == nil {
		a.taskServices.interests = map[string]func(){}
	}
	a.taskServices.interests[key] = finish
	a.taskServices.mu.Unlock()
	stop := context.AfterFunc(ctx, finish)
	a.taskServices.mu.Lock()
	if a.taskServices.interests[key] != nil {
		a.taskServices.interests[key] = func() { stop(); finish() }
	}
	a.taskServices.mu.Unlock()
	return ctx.Err()
}
