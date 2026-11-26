package main

import (
	"context"
	"path/filepath"

	"studio/internal/agents"
	"studio/internal/layout"
	"studio/internal/taskhost"
)

func (a *App) taskHostOperations(context.Context, taskhost.Scope) ([]string, error) {
	ops := []string{"OpenChat", "TakeTurn", "ReadOptions", "UseModel", "CancelTurn", "ReleaseChat", "InvokeAction", "CancelAction", "AskPerson"}
	if a.taskRooms() != nil {
		ops = append(ops, "EnsureRoom", "ClaimRoom", "PostRoom", "ObserveRoom", "ReleaseRoom", "RecoverRoom")
	}
	return ops, nil
}

func (a *App) describeTaskChats(ctx context.Context, scope taskhost.Scope) ([]taskhost.ChatDescription, error) {
	binding, err := a.taskWorkspace(ctx, scope)
	if err != nil {
		return nil, err
	}
	view, err := a.OrgChart()
	if err != nil {
		return nil, err
	}
	result := []taskhost.ChatDescription{}
	for _, chat := range view.Chats {
		if !filepath.IsAbs(chat.Workspace) {
			continue
		}
		workspace, err := agents.ResolvePath(chat.Workspace)
		if err != nil || !layout.SamePath(workspace, binding.Path) {
			continue
		}
		state := string(chat.State.Life)
		if chat.Resolution != "resolved" {
			state = "repair"
		}
		result = append(result, taskhost.ChatDescription{ChatID: chat.ChatID, Head: chat.Head, Title: chat.Title, Workspace: scope.Workspace, Program: chat.Provider, State: state, Reason: chat.Reason})
	}
	return result, nil
}

func (a *App) checkTaskProgram(ctx context.Context, scope taskhost.Scope, program string) (taskhost.ProgramDescription, error) {
	if err := ctx.Err(); err != nil {
		return taskhost.ProgramDescription{}, err
	}
	value := taskhost.ProgramDescription{Program: program, Offered: program != "" && product.Offers(program)}
	if !value.Offered {
		value.Reason = "This program is not offered by the current host configuration."
	}
	return value, nil
}

func (a *App) describeTaskAction(ctx context.Context, scope taskhost.Scope, reference string) (taskhost.ActionDescription, error) {
	if err := ctx.Err(); err != nil {
		return taskhost.ActionDescription{}, err
	}
	registry, err := a.appletRegistry()
	if err != nil {
		return taskhost.ActionDescription{}, err
	}
	action, err := registry.Describe(reference)
	if err != nil {
		return taskhost.ActionDescription{}, err
	}
	digest, err := registry.Digest(reference)
	if err != nil {
		return taskhost.ActionDescription{}, err
	}
	return taskhost.ActionDescription{Reference: reference, Title: action.Title, Description: action.Description, InputSchema: action.InputSchema, OutputSchema: action.OutputSchema, Replay: action.Replay, Capabilities: action.Capabilities, Digest: digest}, nil
}
