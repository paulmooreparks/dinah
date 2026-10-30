package main

import (
	"studio/internal/acp"
	"studio/internal/providers"
	"studio/internal/sessionhost"
)

func (a *App) continuations() *sessionhost.Continuations {
	a.continuationOnce.Do(func() {
		a.continuationService = sessionhost.NewContinuations(&a.cmu, &a.mu, &a.runtime, sessionhost.ContinuationHost{
			StateDir: configDir, SupportedProvider: supportedPreferenceProvider, NormalizeProvider: normalizedProvider,
			Session: a.session, Invocations: conversationAppletInvocations,
			HasPending:  func(id string) bool { return a.permissions().HasPending(id) },
			BusIdentity: a.busIdentityOf, MoveBusIdentity: a.moveBusSession,
			NewSession: func(ws, provider, adopt string) (SessionInfo, error) {
				return a.sessionsLifecycle().NewAdopting(ws, provider, adopt)
			}, RevokeSession: a.revokeAppletSession, Emit: a.emit,
			RenameSession: a.RenameSession, Budget: continuationBudget, Redact: redactAppletContexts,
			ProviderName: providers.Name, WorkspaceName: a.chatWorkspaceName, ApplyOptions: a.carryOptions,
			Formatter: a.judgedHandoffFormatter(), InterruptedNote: interruptedTurnNote, SummaryBudget: summaryBudget,
			HideSession: func(id string) error {
				return a.updateSettings(func(s *Settings) error { s.Hidden = append(s.Hidden, id); return nil })
			},
			Title: func(id string) string { a.mu.Lock(); defer a.mu.Unlock(); return a.settings.SessionTitles[id] },
			ListPreferences: func() (map[string]bool, map[string]string) {
				a.mu.Lock()
				defer a.mu.Unlock()
				hidden := map[string]bool{}
				titles := map[string]string{}
				for _, id := range a.settings.Hidden {
					hidden[id] = true
				}
				for id, title := range a.settings.SessionTitles {
					titles[id] = title
				}
				return hidden, titles
			},
		})
	})
	return a.continuationService
}

// carryOptions gives the chat with id the settings the person had chosen in the chat
// it continues in another workspace, one by one, as choosing them in the chat would,
// and returns the names of those the provider refused. The settings that grant
// authority are not among them: the engine leaves those out (sessionhost/switch.go).
func (a *App) carryOptions(id string, options []acp.ConfigOption) []string {
	var refused []string
	for _, o := range options {
		if len(o.CurrentValue) == 0 {
			continue
		}
		if _, err := a.optionService().Set(id, o.ID, o.CurrentValue); err != nil {
			logf("chat", "warning", "the setting %q of chat %s was not carried to its continuation: %v", o.ID, id, err)
			refused = append(refused, o.Name)
		}
	}
	return refused
}
