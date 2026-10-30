package main

import "studio/internal/sessionhost"

const closedSessionEnd = sessionhost.ClosedSessionEnd
const closeSessionPatience = sessionhost.CloseSessionPatience

func (a *App) closedSessions() *sessionhost.ClosedSessions {
	a.closedOnce.Do(func() {
		a.closedService = sessionhost.NewClosedSessions(&a.mu, &a.runtime, sessionhost.ClosedSessionsHost{
			NormalizeProvider: normalizedProvider, Clock: a.wakeClock, LeaveBus: func(s *Session) { a.busCoordinator().LeaveBus(s) },
			Emit: a.emit, StopProvider: a.AgentStop, CallWithin: a.callWithin,
			RevokeSession: a.revokeAppletSession, CloseChatLog: func(id string) { a.chatLogs().Close(id) },
			CancelQuestions: func(session string) { a.permissions().CancelSession(session) }, DisconnectSubagents: func(root *Session) { a.subagentService().Disconnect(root) },
			Log: func(level, format string, args ...any) { logf("chat", level, format, args...) },
		})
	})
	return a.closedService
}
