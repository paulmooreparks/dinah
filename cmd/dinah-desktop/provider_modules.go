package main

import (
	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/providers"
)

// providerModule is one provider's own work in the window: what it takes to
// find it, check it, sign in to it and open a session with it. Each provider
// registers its module from its own file (provider_<id>.go), so shared code
// calls the module of the provider it has and never compares with a provider's
// name. What differs as a fact, which the table in internal/providers declares,
// is not here.
type providerModule struct {
	// FileReadScope retains provider-specific folder restrictions before host policy.
	FileReadScope func(Settings, string) ([]string, bool)
	// Info fills in whether the provider is installed and where.
	Info func(a *App, p *AgentProviderInfo, workspaces []string)
	// Check is the part of checking a connected provider that asks the
	// provider something, and fills in what it says.
	Check func(a *App, conn agentConnection, info *AgentProviderInfo)
	// SignIn signs the person in, or says why Dinah cannot.
	SignIn func(a *App) error
	// SignInHelp says how the person signs in to a provider that signs in
	// outside the window; the page shows it when a request is refused.
	SignInHelp string
	// AuthRequired reports that a request was refused for want of a sign-in,
	// and returns what the page is told.
	AuthRequired func(a *App) error
	// SessionParams are the parameters of a session, built from the protocol's.
	SessionParams func(a *App, p acp.LoadSessionParams) any
	// BilledByKey says the person uses the provider with their own API key,
	// which is billed for each token; nil for a provider used on a sign-in.
	BilledByKey func(a *App) bool
	// PrepareApplets makes the applet integration ready for a workspace.
	PrepareApplets func(workspace string) error
	// Unattended says the person's settings for the provider let a chat run
	// without being asked about applet actions. It is called with the
	// settings locked.
	Unattended func(s Settings) bool
	// Picture is a picture as a chat of the provider is given it, in base64,
	// with a note for the agent when there is one.
	Picture func(name, mime string, data []byte, encoded string) (string, string)
	// Prices are the prices known now for the provider's models, by the model
	// option's value; only a provider whose row says it has prices has them.
	Prices func(a *App) map[string]agents.DevinModelPrice
	// Skills adds to a chat's skill catalogue the skills the provider finds
	// itself for a workspace, and any problems finding them; nil for a
	// provider that finds none of its own.
	Skills func(a *App, workspace string, catalog *SkillCatalog)
	// Context measures the provider's chats' context windows and the courses
	// for one that is full (context_hold.go); nil for a provider that does not.
	Context *contextModule
	// Adapter starts and watches the provider (provider_lifecycle.go).
	Adapter *providerAdapter
	// Judge lets the provider's models make judgement calls (judgements.go);
	// nil for a provider that cannot.
	Judge *judgeModule
	// Asking is the provider's setting for what it asks the person before
	// it acts, for the set-up to offer (setup.go); nil for a provider whose
	// setting is one of the options its own catalog offers, or that has
	// none.
	Asking *askingModule
	// OpenFailureAdvice is one sentence the page adds below the agent's own
	// words when a chat of the provider could not be opened, for every such
	// failure; empty for a provider with nothing to advise.
	OpenFailureAdvice string
	// ArchivesThreads says the provider's connection can archive a chat's
	// thread in the provider's own store (agent_threads.go).
	ArchivesThreads bool
	// ArchiveClosed says the person's settings have the thread of a chat
	// archived once the chat was closed and its session ended; nil for a
	// provider without that setting. It is called with the settings locked.
	ArchiveClosed func(s Settings) bool
}

// askingModule is a provider's setting for what it asks the person before it
// acts: its choices in the words Settings uses, in Settings' order, the one
// the saved settings give, and the way one is saved.
type askingModule struct {
	Choices []SetupChoice
	Current func(s Settings) string
	Set     func(a *App, id string) error
}

// providerModules are the modules of the providers the window can run, by ID.
var providerModules = map[string]providerModule{}

// registerProviderModule is called by each provider's file.
func registerProviderModule(id string, m providerModule) { providerModules[id] = m }

// ModelPrices returns the prices known now for a provider's models, by the model
// option's value, and an empty map when none are known. Only the providers whose
// row says they have prices are asked, each through its own module.
func (a *App) ModelPrices(provider string) map[string]agents.DevinModelPrice {
	provider = normalizedProvider(provider)
	if m, ok := providerModules[provider]; ok && m.Prices != nil && providers.For(provider).Prices {
		return m.Prices(a)
	}
	return map[string]agents.DevinModelPrice{}
}
