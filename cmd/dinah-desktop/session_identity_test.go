package main

import (
	"studio/internal/acp"
	"studio/internal/sessionhost"
)

func newTestProvider(conn agentConnection, initial acp.InitializeResult) *providerRuntime {
	return sessionhost.NewProvider(conn, initial, nil)
}
func setTestConnection(a *App, id string, conn agentConnection) {
	a.runtime.PutProviderLocked(id, newTestProvider(conn, a.runtime.ProviderLocked(id).Initialization()))
}
func setTestInitialization(a *App, id string, initial acp.InitializeResult) {
	a.runtime.PutProviderLocked(id, newTestProvider(a.runtime.ProviderLocked(id).Connection(), initial))
}

// replaceTestIdentity builds another fixture before its behavior is exercised.
// Production conversations keep their identity throughout their lifetime.
func replaceTestIdentity(a *App, old *Session, field, value string) *Session {
	state := old.State()
	seed := sessionSeed{ID: old.ID(), Provider: old.Provider(), Cwd: old.Cwd(), Title: state.Title, UpdatedAt: state.UpdatedAt}
	switch field {
	case "Provider":
		seed.Provider = value
	case "Cwd":
		seed.Cwd = value
	default:
		panic("unknown fixture identity field")
	}
	s := newLoadingSessionState(&seed, state.Replaying, state.Restoring, sessionInitial{Open: state.Open, Working: state.Working, Switching: state.Switching})
	s.UpdateOptions(old.Options())
	if state.BusID != "" {
		s.JoinBus(state.BusID)
	}
	a.runtime.PutConversationLocked(s.ID(), s)
	return s
}
func setTestTitle(s *Session, title string) { s.UpdateDisplay(&title, nil) }
