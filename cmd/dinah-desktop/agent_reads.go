package main

import (
	"context"
	"errors"
	"io/fs"
	"studio/internal/acp"
	"studio/internal/fileaccess"
)

// readAgentTextFile derives authority from the live transport and registered
// session, never from a caller-supplied workspace or provider name.
func (a *App) readAgentTextFile(ctx context.Context, conn agentConnection, p acp.ReadTextFileParams) (string, error) {
	scope, err := a.agentReadScope(conn, p.SessionID)
	if err != nil {
		return "", err
	}
	service, err := a.fileAccessService()
	if err != nil {
		return "", err
	}
	scope.Source = "fs/read_text_file"
	text, err := service.Read(ctx, p.Path, p.Line, p.Limit, scope)
	if !fileaccess.InOperation(ctx) {
		a.fileAccessUpdated("file-access:read", nil)
	}
	return text, err
}

func (a *App) agentReadScope(conn agentConnection, sessionID string) (fileaccess.Scope, error) {
	if conn == nil {
		return fileaccess.Scope{}, errors.New("the provider connection is unavailable")
	}
	select {
	case <-conn.Done():
		return fileaccess.Scope{}, errors.New("the provider connection ended")
	default:
	}
	s, sub := a.subagentService().RequestChat(sessionID)
	if s == nil || !s.Open() {
		return fileaccess.Scope{}, errors.New("the requested chat is not open")
	}
	if sub != nil && sub.State != "running" {
		return fileaccess.Scope{}, errors.New("the requested subagent is not running")
	}
	a.mu.Lock()
	provider := a.runtime.ProviderLocked(s.Provider())
	if provider == nil || provider.Connection() != conn {
		a.mu.Unlock()
		return fileaccess.Scope{}, errors.New("this connection does not own the requested chat")
	}
	scope := fileAccessScope(a.settings, s.Cwd())
	scope.Chat, scope.ChatTitle = s.ID(), s.State().Title
	if module := providerModules[normalizedProvider(s.Provider())]; module.FileReadScope != nil {
		scope.LimitRoots, scope.Limited = module.FileReadScope(a.settings, s.Cwd())
	}
	a.mu.Unlock()

	return scope, nil
}
func (a *App) readAgentDirectory(ctx context.Context, conn agentConnection, sessionID, path, source string) ([]fs.DirEntry, error) {
	scope, err := a.agentReadScope(conn, sessionID)
	if err != nil {
		return nil, err
	}
	scope.Source = source
	service, err := a.fileAccessService()
	if err != nil {
		return nil, err
	}
	entries, err := service.ReadDirectory(ctx, path, scope)
	if !fileaccess.InOperation(ctx) {
		a.fileAccessUpdated("file-access:read", nil)
	}
	return entries, err
}
