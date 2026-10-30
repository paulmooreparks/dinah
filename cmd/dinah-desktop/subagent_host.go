package main

import (
	"context"
	"encoding/json"

	"studio/internal/acp"
	"studio/internal/sessionhost"
)

func (a *App) subagentService() *sessionhost.SubagentService {
	a.subagentsOnce.Do(func() {
		a.subagentsService = sessionhost.NewSubagentService(&a.mu, &a.runtime, sessionhost.SubagentHost{
			Emit: a.emit, Connection: a.sessionConn, AskPermission: func(ctx context.Context, session string, sub *SubagentInfo, toolCall json.RawMessage, options []acp.PermissionOption) (string, bool) {
				return a.permissions().Ask(ctx, session, sub, toolCall, options)
			}, ProductName: product.Name,
			CancelQuestions: func(ids map[string]bool) { a.permissions().CancelSubagents(ids) },
		})
	})
	return a.subagentsService
}
