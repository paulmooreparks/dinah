package main

import (
	"context"
	"errors"
	"studio/internal/acp"
	"studio/internal/agents"
)

// readAgentTextFile derives authority from the live transport and registered
// session, never from a caller-supplied workspace or provider name.
func (a *App) readAgentTextFile(ctx context.Context, conn agentConnection, p acp.ReadTextFileParams) (string, error) {
	if conn == nil {
		return "", errors.New("the provider connection is unavailable")
	}
	select {
	case <-conn.Done():
		return "", errors.New("the provider connection ended")
	default:
	}
	s, sub := a.subagentService().RequestChat(p.SessionID)
	if s == nil || !s.Open() {
		return "", errors.New("the requested chat is not open")
	}
	if sub != nil && sub.State != "running" {
		return "", errors.New("the requested subagent is not running")
	}
	a.mu.Lock()
	provider := a.runtime.ProviderLocked(s.Provider())
	if provider == nil || provider.Connection() != conn {
		a.mu.Unlock()
		return "", errors.New("this connection does not own the requested chat")
	}
	roots := managedExecutionRoots(a.settings.WorkspaceRecords)
	a.mu.Unlock()
	return agents.ReadTextFile(ctx, stateLayout(), p.Path, roots, p.Line, p.Limit)
}
