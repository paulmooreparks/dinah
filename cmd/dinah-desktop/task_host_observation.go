package main

import (
	"context"
	"studio/internal/taskhost"
)

func (a *App) pendingServiceQuestions(ctx context.Context, scope taskhost.Scope) ([]taskhost.Question, error) {
	if err := a.checkTaskIdentity(ctx, scope); err != nil {
		return nil, err
	}
	result := []taskhost.Question{}
	for _, q := range a.PendingTaskPermissions() {
		if q.Service == scope.Service && q.Workspace == scope.Workspace && q.Task == scope.Task && q.Generation == scope.Generation {
			result = append(result, q)
		}
	}
	return result, nil
}
