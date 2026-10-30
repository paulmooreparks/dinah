package main

import (
	"os"
	"path/filepath"
	"time"

	"studio/internal/acp"
	"studio/internal/sessionhost"
)

// Every agent provider starts, is watched and ends the same way
// (sessionhost.ProviderLifecycle). What differs between them is
// in a providerAdapter: how the process is found and launched, how long
// initialize may take, and the words the page is shown for its states.

// providerRuntime is a running provider: its connection, what it said at
// initialize, and gone, which closes once the process has ended and its
// chats have been let go, so that a stop returns only then and a restart
// cannot see the old process's "stopped" arrive after the new one's "ready".
type providerRuntime = sessionhost.Provider

// providerAdapter is what is particular to one provider in the shared
// lifecycle. The fields are words and limits; prepare does the work.
type providerAdapter struct {
	id string
	// starting and stopped are the state details the page shows around the
	// process's life; readyDetail is the detail once initialize has answered.
	starting    string
	stopped     string
	readyDetail func(init acp.InitializeResult) string
	// initWithin is how long initialize may take, and initError the wording
	// around its failure; empty keeps the agent's own error.
	initWithin time.Duration
	initError  string
	// traces says the connection writes the app's protocol trace, which a
	// stop then flushes.
	traces bool
	// prepare checks what the provider needs and returns how to launch it.
	// Preparation runs under lifecycle connect serialization.
	prepare func(a *App) (providerStart, error)
}

// providerStart is a prepared launch. launch starts the process or opens
// the connection, and ready, when set, runs once initialize has answered.
type providerStart struct {
	launch func(acp.Handler) (agentConnection, error)
	ready  func(init acp.InitializeResult)
}

// providerAdapterList are the adapters of the providers the app can run, from
// their modules.
func providerAdapterList() []*providerAdapter {
	var out []*providerAdapter
	for _, m := range providerModules {
		if m.Adapter != nil {
			out = append(out, m.Adapter)
		}
	}
	return out
}

func connectedDetail(name string) func(acp.InitializeResult) string {
	return func(acp.InitializeResult) string { return name + " is connected" }
}

func agentStartupDir() (string, error) {
	neutral := filepath.Join(configDir(), "agent-startup")
	if err := os.MkdirAll(neutral, 0o700); err != nil {
		return "", err
	}
	return neutral, nil
}

// lifecycle constructs one stable owner even for Apps built directly by tests.
// Construction performs no host callbacks and holds neither App.mu nor a
// conversation lock. Callback execution belongs to the lifecycle operations.
func (a *App) lifecycle() *sessionhost.ProviderLifecycle {
	a.lifecycleOnce.Do(func() {
		adapters := providerAdapterList()
		specs := make([]sessionhost.ProviderSpec, 0, len(adapters))
		for _, adapter := range adapters {
			specs = append(specs, sessionhost.ProviderSpec{
				ID: adapter.id, Starting: adapter.starting, Stopped: adapter.stopped,
				ReadyDetail: adapter.readyDetail, InitWithin: adapter.initWithin,
				InitError: adapter.initError, Traces: adapter.traces,
				Prepare: func() (sessionhost.ProviderStart, error) {
					start, err := adapter.prepare(a)
					return sessionhost.ProviderStart{Launch: start.launch, Ready: start.ready}, err
				},
			})
		}
		a.providerLifecycle = sessionhost.NewProviderLifecycle(&a.mu, &a.runtime, specs, sessionhost.LifecycleHost{
			Handler:          func() acp.Handler { return a.newAgentHandler() },
			TraceForProvider: func(string) sessionhost.Trace { return a.logs().trace },
			RedactTrace:      redactAppletContexts, RevokeChannels: a.revokeAppletChannels,
			CloseChatLog:    func(id string) { a.chatLogs().Close(id) },
			CancelQuestions: func(session string) { a.permissions().CancelSession(session) }, DisconnectSubagents: func(root *Session) { a.subagentService().Disconnect(root) },
			Emit: a.emitProviderEvent, ClientVersion: appVersion,
		})
	})
	return a.providerLifecycle
}

func (a *App) AgentConnect(provider string) error { return a.lifecycle().Connect(provider) }
func (a *App) AgentStop(provider string)          { a.lifecycle().Stop(provider) }
func (a *App) AgentRestart(provider string) error { return a.lifecycle().Restart(provider) }

// providerConnLocked is a provider's running connection, or nil. The caller
// holds a.mu.
func (a *App) providerConnLocked(provider string) agentConnection {
	if p := a.runtime.ProviderLocked(normalizedProvider(provider)); p != nil {
		return p.Connection()
	}
	return nil
}

// providerInitLocked returns a provider's running connection and what it
// said at initialize. The caller holds a.mu.
func (a *App) providerInitLocked(provider string) (agentConnection, acp.InitializeResult) {
	if p := a.runtime.ProviderLocked(normalizedProvider(provider)); p != nil {
		return p.Connection(), p.Initialization()
	}
	return nil, acp.InitializeResult{}
}

// providerOf is the provider whose running connection conn is, or "" when
// it is no provider's now.
func (a *App) providerOf(conn agentConnection) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, id := range a.runtime.ProviderIDsLocked() {
		p := a.runtime.ProviderLocked(id)
		if p.Connection() == conn {
			return id
		}
	}
	return ""
}
