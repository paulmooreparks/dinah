package main

import (
	"encoding/json"

	"studio/internal/acp"
	"studio/internal/providers"
	"studio/internal/sessionhost"
)

// lifecycleChannel retains channel authority in the shell. The engine receives
// only Bind/Revoke, never the channel token or bridge record.
type lifecycleChannel struct {
	app     *App
	channel *appletChannel
}

func (c *lifecycleChannel) Bind(id string) error { return c.app.bindAppletChannel(c.channel, id) }
func (c *lifecycleChannel) Revoke()              { c.app.revokeAppletChannel(c.channel) }

func (a *App) sessionsLifecycle() *sessionhost.SessionLifecycle {
	a.sessionsOnce.Do(func() {
		a.sessionsService = sessionhost.NewSessionLifecycle(&a.mu, &a.runtime, sessionhost.SessionLifecycleHost{
			NormalizeProvider: normalizedProvider, PrepareApplets: prepareProviderApplets, Live: func(provider string) (agentConnection, acp.InitializeResult, error) {
				return a.lifecycle().Live(provider)
			},
			Defaults: func(provider string) map[string]json.RawMessage {
				a.mu.Lock()
				defer a.mu.Unlock()
				defaults := copyPreferenceValues(a.settings.Defaults)
				if providers.For(provider).OwnPreferences {
					defaults = copyPreferenceValues(a.settings.ProviderDefaults[provider])
				}
				return defaults
			},
			SavedOptions: func(id string) (map[string]json.RawMessage, string) {
				a.mu.Lock()
				defer a.mu.Unlock()
				return copyPreferenceValues(a.settings.SessionOptions[id]), a.settings.SessionTitles[id]
			},
			NewChannel: func(conn agentConnection, ws string) (sessionhost.ChannelCapability, []any, error) {
				channel, servers, err := a.newAppletChannel(conn, ws)
				return &lifecycleChannel{app: a, channel: channel}, servers, err
			},
			ManagedServers: managedAppletServers, BusSessionID: a.busSessionID, WithBusServer: a.withBusServer, AdoptBusServer: a.adoptBusServer, EndBusSession: func(id string) { a.busCoordinator().EndBusSession(id) },
			Call: a.call, CallWithin: a.callWithin, SessionParams: a.providerSessionParams, InitTranscript: func(s *Session, replay bool) { a.transcriptCapture().Initialize(s, replay) }, ResumeTranscript: func(s *Session) string { return a.transcriptCapture().Resume(s) }, StartWaiting: func(s *Session) { a.turnIO().StartWaiting(s) }, RestoreQueue: func(s *Session) { a.queueRestorer().Restore(s) }, AfterOpen: a.loadContextChoice, ResolveContinuation: a.ResolveSessionContinuation,
			BeginLog: func(id, provider, ws, title string) { a.chatLogs().Begin(id, provider, ws, title) }, NoteLog: func(id, note string) { a.chatLogs().Note(id, note) },
			RestoreOptions: func(conn agentConnection, session string, opts []acp.ConfigOption, chosen map[string]json.RawMessage, reopen *reopening) ([]acp.ConfigOption, []appliedSubstitution, error) {
				return a.optionService().Restore(conn, session, opts, chosen, reopen)
			}, FinishOptions: func(s *Session, opts []acp.ConfigOption) error { return a.optionService().Finish(s, opts) }, JoinBus: func(s *Session, id string) { a.busCoordinator().JoinBus(s, id) }, Emit: a.emit, ListProviders: a.listProviders,
			Hidden: func(id string) bool { a.mu.Lock(); defer a.mu.Unlock(); return a.hidden(id) }, Title: a.sessionTitle, FilterAncestors: func(in []SessionInfo, workspace string) []SessionInfo {
				return a.continuations().FilterAncestors(in, workspace)
			},
			SavedProvider: a.savedSessionProvider, WaitForEnd: func(id string) { a.closedSessions().WaitForEnd(id) }, Unpark: func(s *Session) { a.busCoordinator().Unpark(s) }, DropSubagentIndex: func(rootID string) { a.subagentService().DropIndex(rootID) },
			SettleReplayedPrompt: func(s *Session) { a.transcriptCapture().SettleReplay(s) }, FlushReplay: a.flushReplay, DisconnectSubagents: func(root *Session) { a.subagentService().Disconnect(root) },
			FinishTranscriptReplay: func(s *Session, succeeded bool) { a.transcriptCapture().FinishReplay(s, succeeded) }, Reopening: a.reopeningFor, WatchReopen: watchReopen, Connected: func(chat string, err error) { a.busCoordinator().Connected(chat, err) },
			Cancel: func(s *Session) error { return a.turnIO().CancelConversation(s) }, Tombstone: func(id string) error { return a.transcriptCapture().Tombstone(id) }, ForgetTeam: a.forgetTeam,
			ForgetOptions: func(id string) {
				_ = a.updateSettings(func(s *Settings) error { delete(s.SessionProviders, id); delete(s.SessionOptions, id); return nil })
			},
			ForgetTitle: func(id string) {
				_ = a.updateSettings(func(s *Settings) error { delete(s.SessionTitles, id); delete(s.JudgedTitles, id); return nil })
			},
			LeaveBus: func(s *Session) { a.busCoordinator().LeaveBus(s) }, RevokeSession: a.revokeAppletSession, ForgetJournal: a.forgetJournal, CancelQuestions: func(session string) { a.permissions().CancelSession(session) },
			Log: func(level, format string, args ...any) { logf("chat", level, format, args...) },
		})
	})
	return a.sessionsService
}
