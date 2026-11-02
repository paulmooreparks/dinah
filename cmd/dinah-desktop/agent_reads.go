package main

import (
	"context"
	"errors"
	"studio/internal/acp"
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
	scope := fileAccessScope(a.settings, s.Cwd())
	scope.Chat, scope.ChatTitle = s.ID(), s.State().Title
	if module := providerModules[normalizedProvider(s.Provider())]; module.FileReadScope != nil {
		scope.LimitRoots, scope.Limited = module.FileReadScope(a.settings, s.Cwd())
	}
	a.mu.Unlock()
	service, err := a.fileAccessService()
	if err != nil {
		return "", err
	}
	text, err := service.Read(ctx, p.Path, p.Line, p.Limit, scope)
	a.fileAccessUpdated("file-access:read", nil)
	return text, err
}
