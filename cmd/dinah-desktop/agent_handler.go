package main

import (
	"context"
	"encoding/json"
	"slices"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/sessionhost"
)

// newAgentHandler binds protocol handling to shell policy and effects. Each
// connection receives its own handler; callbacks acquire their own host locks.
func (a *App) newAgentHandler() *sessionhost.Handler {
	return sessionhost.NewHandler(sessionhost.HandlerHost{
		ClaudeAccountReport: a.claudeAccountReport, RedactContexts: redactAppletContexts,
		VisibleConversationText: visibleConversationText, LookupConversation: a.session,
		SubagentUpdate: func(sessionID string, update json.RawMessage) bool {
			return a.subagentService().Update(sessionID, update)
		}, ReplayBoundary: func(s *Session, kind string) { a.transcriptCapture().ReplayBoundary(s, kind) },
		RecordConversationUpdate: func(s *Session, update json.RawMessage, logIt bool) *sessionhost.LoggedEvent {
			return a.transcriptCapture().RecordUpdateLogged(s, update, logIt)
		},
		LogUpdate: func(id string, update json.RawMessage) { a.chatLogs().Update(id, update) },
		Journal:   a.journal, SetOptions: a.setOptions,
		SetCommands:  func(commands []sessionhost.Command) { a.mu.Lock(); a.commands = slices.Clone(commands); a.mu.Unlock() },
		ResolveTitle: a.sessionTitle, PublishBusTitle: func(s *Session) { a.busCoordinator().PublishBusTitle(s) }, FlushReplay: a.flushReplay,
		Emit: a.emit, ResolveRequestChat: func(id string) (*Session, *SubagentInfo) { return a.subagentService().RequestChat(id) }, AskPermission: func(ctx context.Context, session string, toolCall json.RawMessage, options []acp.PermissionOption) (string, bool) {
			return a.permissions().Ask(ctx, session, nil, toolCall, options)
		},
		AskAsSubagent: func(ctx context.Context, chat *Session, sub *SubagentInfo, toolCall json.RawMessage, options []acp.PermissionOption) (string, bool) {
			return a.subagentService().Ask(ctx, chat, sub, toolCall, options)
		},
		WriteTextFile: func(target, content string) error {
			a.mu.Lock()
			managedRoots := managedExecutionRoots(a.settings.WorkspaceRecords)
			a.mu.Unlock()
			return agents.WriteTextFile(stateLayout(), target, content, managedRoots...)
		},
		DisplayPath: rel, ProductName: product.Name, ReplayBatch: replayBatch,
		Judges:   a.judgeSessionSet(),
		TurnCost: a.noteTurnCost,
	})
}
