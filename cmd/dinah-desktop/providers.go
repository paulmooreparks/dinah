package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/profile"
	"studio/internal/sessionhost"
)

// agentConnection is the host's internal session transport. ACP-shaped events
// keep the workbench independent of each provider's wire protocol.
type agentConnection = sessionhost.Connection

type AgentProviderInfo struct {
	ExecutableOverride string   `json:"executableOverride,omitempty"`
	Pinned             []string `json:"pinned"`
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Installed          bool     `json:"installed"`
	Path               string   `json:"path"`
	Available          bool     `json:"available"`
	Status             string   `json:"status"`
	Detail             string   `json:"detail"`
	// StatusWords say what is wrong in plain words, from the status code, and
	// are empty when nothing is. Default is true for the one provider new
	// chats use, and Facts say what differs between providers
	// (provider_registry.go).
	StatusWords string             `json:"statusWords"`
	Default     bool               `json:"default"`
	Facts       ProviderFacts      `json:"facts"`
	Catalog     []acp.ConfigOption `json:"catalog,omitempty"`
	// Choices are the Devin command lines that work when there is more than
	// one, for the person to choose from, and Likely the path of the one
	// whose release looks newest, as a suggestion.
	Choices []agents.DevinCandidate `json:"choices,omitempty"`
	Likely  string                  `json:"likely,omitempty"`
	// Claude is what Claude needs besides Node.js: the adapter, and how it
	// is billed.
	Claude *ClaudeSetup `json:"claude,omitempty"`
}

// ClaudeSetup is Claude's state in Settings > Agents
// (docs/proposals/claude-agent-integration.md). Path is then Node.js, which
// runs the adapter.
type ClaudeSetup struct {
	// AdapterVersion is the pinned adapter release, and AdapterInstalled
	// whether it is installed in Dinah's state folder.
	AdapterVersion   string `json:"adapterVersion"`
	AdapterInstalled bool   `json:"adapterInstalled"`
	// CanInstall is whether npm was found beside Node.js to install it with.
	CanInstall bool `json:"canInstall"`
	// APIKey is whether an API key is stored, which bills Claude to it; with
	// none, Claude uses the person's own Claude Code sign-in.
	APIKey bool `json:"apiKey"`
}

// agentProviderKnown says whether id is a provider the product offers
// (product.Providers).
func agentProviderKnown(id string) bool { return product.Offers(id) }

// An explicit path takes precedence even when it later becomes unavailable.
// Validation never launches the executable or changes a running connection.
func (a *App) SetAgentExecutable(provider, path string) error {
	if !agentProviderKnown(provider) {
		return errors.New("unknown agent provider")
	}
	if path != "" {
		if _, err := agents.CheckExecutable(path); err != nil {
			return err
		}
		path = filepath.Clean(path)
	}
	return a.updateSettings(func(s *Settings) error {
		if s.ProviderExecutables == nil {
			s.ProviderExecutables = map[string]string{}
		}
		if path == "" {
			delete(s.ProviderExecutables, provider)
		} else {
			s.ProviderExecutables[provider] = path
		}
		return nil
	})
}

func (a *App) ChooseAgentExecutable(provider string) (bool, error) {
	if !agentProviderKnown(provider) || a.ctx == nil {
		return false, errors.New("choose an agent executable from Settings in the open application")
	}
	directory := a.currentWorkspace()
	for _, p := range a.AgentProviders() {
		if p.ID == provider && p.Path != "" {
			directory = filepath.Dir(p.Path)
		}
	}
	// On a Mac an agent's command line can be inside an application package
	// (Devin.app), which the dialog otherwise shows as one file that cannot
	// be opened; Wails documents TreatPackagesAsDirectories for that, and
	// other systems ignore it.
	options := wruntime.OpenDialogOptions{Title: "Choose Agent Executable", DefaultDirectory: directory, TreatPackagesAsDirectories: true}
	if runtime.GOOS == "windows" {
		options.Filters = []wruntime.FileFilter{{DisplayName: "Agent Executables (*.exe;*.cmd;*.bat)", Pattern: "*.exe;*.cmd;*.bat"}}
	}
	path, err := wruntime.OpenFileDialog(a.ctx, options)
	if err != nil || path == "" {
		return false, err
	}
	if err := a.SetAgentExecutable(provider, path); err != nil {
		return false, err
	}
	return true, nil
}

// normalizedProvider is a chat's provider. A chat saved before chats had
// providers has none, and is Devin's, whatever the product's default
// provider is now.
func normalizedProvider(provider string) string { return profile.ProviderOrLegacy(provider) }

// defaultProviderOf is the provider new chats use: the person's choice in
// settings, or else the profile's default.
func defaultProviderOf(s Settings) string {
	if s.DefaultProvider == "" {
		return product.DefaultProvider
	}
	return s.DefaultProvider
}

// providerForSession is the provider a session ID belongs to, from the prefix
// in its row, or the legacy provider for an ID with none.
func providerForSession(id string) string {
	for provider, row := range providerRegistry {
		if row.SessionPrefix != "" && strings.HasPrefix(id, row.SessionPrefix) {
			return provider
		}
	}
	return profile.LegacyProvider
}

func (a *App) savedSessionProvider(id string) (string, error) {
	a.mu.Lock()
	provider := a.settings.SessionProviders[id]
	a.mu.Unlock()
	if provider == "" {
		return providerForSession(id), nil
	}
	if !supportedPreferenceProvider(provider) {
		return "", fmt.Errorf("this chat uses the unavailable provider %q", provider)
	}
	if provider != providerForSession(id) {
		return "", errors.New("this chat's saved provider conflicts with its session ID")
	}
	return provider, nil
}

func (a *App) defaultProvider() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return defaultProviderOf(a.settings)
}

func (a *App) SetDefaultProvider(id string) error {
	if !supportedPreferenceProvider(id) {
		return errors.New("that provider is not supported yet")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.DefaultProvider = id
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	a.emitProviderFacts()
	return nil
}

// providerSessionParams are the parameters of a session of a provider: its own
// module's when it has them, and otherwise the protocol's.
func (a *App) providerSessionParams(provider, id, cwd string, servers []any) any {
	p := acp.LoadSessionParams{SessionID: id, Cwd: cwd, McpServers: servers}
	if m, ok := providerModules[provider]; ok && m.SessionParams != nil {
		return m.SessionParams(a, p)
	}
	if id == "" {
		return acp.NewSessionParams{Cwd: cwd, McpServers: servers}
	}
	return p
}

func (a *App) sessionConn(s *Session) agentConnection {
	if s == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.providerConnLocked(s.Provider())
}

// Session listing starts only the chosen provider and any already connected
// providers. Merely discovering an installation does not start it.
func (a *App) listProviders() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	selected := defaultProviderOf(a.settings)
	out := []string{selected}
	for _, id := range a.runtime.ProviderIDsLocked() {
		if id != selected {
			out = append(out, id)
		}
	}
	return out
}

// ProviderStates is the last state of each provider process that has had
// one (starting, ready, stopped or failed), as the agent:state event carried
// it. A page that missed the event reads it here. AgentProviders says whether
// a provider is installed; this says what its process is doing.
func (a *App) ProviderStates() []sessionhost.ProviderState { return a.lifecycle().States() }

func (a *App) AgentProviders() []AgentProviderInfo {
	a.mu.Lock()
	workspaces := append([]string{a.settings.Workspace}, a.settings.Workspaces...)
	a.mu.Unlock()
	out := make([]AgentProviderInfo, 0, len(product.Providers))
	for _, id := range product.Providers {
		out = append(out, AgentProviderInfo{ID: id, Name: providerName(id)})
	}
	for i := range out {
		a.providerInfo(&out[i], workspaces)
		a.finishProviderInfo(&out[i])
	}
	return out
}

// providerInfo fills in whether one agent is installed and where, by the
// provider's own module (provider_modules.go).
func (a *App) providerInfo(p *AgentProviderInfo, workspaces []string) {
	if m, ok := providerModules[p.ID]; ok && m.Info != nil {
		m.Info(a, p, workspaces)
	}
}

// providerInfoOf is one provider's entry of AgentProviders alone, for starting
// it without looking for the other agents, which can take seconds.
func (a *App) providerInfoOf(id string) AgentProviderInfo {
	a.mu.Lock()
	workspaces := append([]string{a.settings.Workspace}, a.settings.Workspaces...)
	a.mu.Unlock()
	p := AgentProviderInfo{ID: id, Name: providerName(id)}
	a.providerInfo(&p, workspaces)
	a.finishProviderInfo(&p)
	return p
}

// finishProviderInfo adds what comes from the provider's row and the status
// code: the words for what is wrong, whether it is the default, and its facts.
func (a *App) finishProviderInfo(p *AgentProviderInfo) {
	a.mu.Lock()
	p.Default = p.ID == defaultProviderOf(a.settings)
	a.mu.Unlock()
	p.StatusWords = statusWordsFor(p.Status)
	p.Facts = a.providerFacts(p.ID)
}

// AgentCheck verifies a provider's connection without sending a prompt, by its
// own module's check when it has one.
func (a *App) AgentCheck(provider string) AgentProviderInfo {
	info := AgentProviderInfo{ID: provider, Name: provider, Status: "unsupported", StatusWords: statusWordsFor("unsupported")}
	for _, p := range a.AgentProviders() {
		if p.ID == provider {
			info = p
			break
		}
	}
	if !info.Available {
		return info
	}
	conn, _, err := a.lifecycle().Live(provider)
	if err != nil {
		info.Status, info.Detail = "failed", err.Error()
		return info
	}
	info.Status, info.Detail = "connected", "Agent connected. No prompt was sent; model access has not been tested."
	if m, ok := providerModules[provider]; ok && m.Check != nil {
		m.Check(a, conn, &info)
		return info
	}
	a.mu.Lock()
	info.Catalog = a.settings.Catalog
	a.mu.Unlock()
	return info
}

// keepProviderCatalog saves the options a provider offers, for the page to
// show before the provider is next started. A save that fails is not an
// error of the check that read the catalog.
func (a *App) keepProviderCatalog(provider string, catalog []acp.ConfigOption) {
	_ = a.updateSettings(func(s *Settings) error {
		if s.ProviderCatalogs == nil {
			s.ProviderCatalogs = map[string][]acp.ConfigOption{}
		}
		s.ProviderCatalogs[provider] = catalog
		return nil
	})
}

// AgentSignIn signs the person in to a provider, by its own module's way.
func (a *App) AgentSignIn(provider string) error {
	if m, ok := providerModules[normalizedProvider(provider)]; ok && m.SignIn != nil {
		return m.SignIn(a)
	}
	return errors.New("that agent is not supported yet")
}

// prepareProviderApplets makes the applet integration ready for a provider's
// session in a workspace, by its own module when it has something to do.
func prepareProviderApplets(provider, workspace string) error {
	m, ok := providerModules[normalizedProvider(provider)]
	if !ok {
		return errors.New("that agent is not supported yet")
	}
	if m.PrepareApplets != nil {
		return m.PrepareApplets(workspace)
	}
	return nil
}

func managedAppletServers() ([]any, error) {
	executable, err := appletHelperExecutable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, err
	}
	return []any{map[string]any{"name": "dinah-applets", "command": executable, "args": []string{"--applet-mcp", "--state-dir", configDir()}, "env": []any{}}}, nil
}
