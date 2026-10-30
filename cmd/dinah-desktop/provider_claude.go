package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/claudeagent"
	"studio/internal/credentials"
)

// Claude's own work in the window: its Node.js and adapter, its API key, and
// the sign-in it leaves to the person's own Claude Code.

func init() {
	registerProviderModule("claude", providerModule{
		Info:    func(a *App, p *AgentProviderInfo, workspaces []string) { a.claudeInfo(p, workspaces) },
		Adapter: claudeAdapter,
		Picture: claudePicture,
		Judge:   acpJudgeModule(),
		SignIn: func(*App) error {
			// Dinah never offers a Claude login of its own
			// (docs/proposals/claude-agent-integration.md, Authentication).
			return errors.New(product.Name + " never signs you in to Claude itself. Run claude in a terminal and sign in there with /login, or store an API key here")
		},
	})
}

// claudeKeys keeps the person's Claude API key, in the system keychain;
// tests replace it, so that they never touch the person's keychain.
var claudeKeys = struct {
	get    func() (string, error)
	set    func(string) error
	remove func() error
}{
	get:    func() (string, error) { return credentials.Get(credentials.KeyClaudeAPIKey) },
	set:    func(key string) error { return credentials.Set(credentials.KeyClaudeAPIKey, key) },
	remove: func() error { return credentials.Remove(credentials.KeyClaudeAPIKey) },
}

// claudeInfo fills in Claude's entry: Node.js, which runs the adapter, the
// adapter, and the billing. It runs nothing.
func (a *App) claudeInfo(p *AgentProviderInfo, workspaces []string) {
	a.mu.Lock()
	p.ExecutableOverride = a.settings.ProviderExecutables["claude"]
	p.Pinned = append([]string{}, a.settings.ProviderPinned["claude"]...)
	a.mu.Unlock()
	var overrideError error
	if p.ExecutableOverride != "" {
		p.Path, overrideError = agents.CheckExecutable(p.ExecutableOverride)
	} else {
		p.Path = agents.FindExecutable("node", workspaces)
	}
	_, keyErr := claudeKeys.get()
	setup := &ClaudeSetup{AdapterVersion: claudeagent.Version, AdapterInstalled: claudeagent.Installed(configDir()), APIKey: keyErr == nil}
	setup.CanInstall = p.Path != "" && claudeagent.NPM(p.Path) != ""
	p.Claude = setup
	p.Installed = p.Path != "" && setup.AdapterInstalled
	p.Available = p.Installed
	billing := "Claude uses your own Claude Code sign-in; sign in by running claude in a terminal once."
	if setup.APIKey {
		billing = "Claude is billed to the API key you stored."
	}
	switch {
	case overrideError != nil:
		p.Status, p.Detail = "invalid-path", overrideError.Error()+". Choose another Node.js or use automatic discovery."
	case p.Path == "":
		p.Status = "runtime-missing"
		p.Detail = fmt.Sprintf("Claude runs on Node.js %d or later, which was not found. Install Node.js, or choose its executable.", claudeagent.MinimumNode)
	case !setup.AdapterInstalled:
		p.Status = "adapter-missing"
		p.Detail = "Node.js was found. Install the Claude adapter " + claudeagent.Version + " to use Claude."
		if !setup.CanInstall {
			p.Detail = "Node.js was found, but npm, which installs the Claude adapter, is not beside it. Reinstall Node.js with npm."
		}
	default:
		p.Status = "detected"
		p.Detail = "Node.js and the Claude adapter " + claudeagent.Version + " are installed. " + billing
	}
}

// InstallClaudeAdapter installs the pinned Claude adapter with the npm that
// came with Node.js. The person asks for it in Settings > Agents.
func (a *App) InstallClaudeAdapter() (AgentProviderInfo, error) {
	var info AgentProviderInfo
	for _, p := range a.AgentProviders() {
		if p.ID == "claude" {
			info = p
		}
	}
	if info.Path == "" || info.Claude == nil {
		return info, fmt.Errorf("install Node.js %d or later first", claudeagent.MinimumNode)
	}
	npm := claudeagent.NPM(info.Path)
	if npm == "" {
		return info, errors.New("npm was not found beside Node.js; reinstall Node.js with npm")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if _, err := claudeagent.CheckNode(ctx, info.Path); err != nil {
		return info, err
	}
	if err := claudeagent.Install(ctx, npm, configDir()); err != nil {
		return info, err
	}
	for _, p := range a.AgentProviders() {
		if p.ID == "claude" {
			info = p
		}
	}
	return info, nil
}

// SetClaudeAPIKey stores the person's Claude API key, which bills Claude to
// it from then on; an empty key removes it, so that Claude uses their own
// Claude Code sign-in. Claude is stopped so that its next start takes the
// change, which ends its open chats' connections.
func (a *App) SetClaudeAPIKey(key string) (AgentProviderInfo, error) {
	key = strings.TrimSpace(key)
	var err error
	if key == "" {
		err = claudeKeys.remove()
	} else {
		err = claudeKeys.set(key)
	}
	if err != nil {
		return AgentProviderInfo{}, err
	}
	a.AgentStop("claude")
	for _, p := range a.AgentProviders() {
		if p.ID == "claude" {
			return p, nil
		}
	}
	return AgentProviderInfo{}, nil
}

// The adapter that starts and watches the provider (provider_lifecycle.go).
var claudeAdapter = &providerAdapter{
	id:          "claude",
	starting:    "Starting Claude",
	stopped:     "Claude stopped",
	readyDetail: connectedDetail("Claude"),
	initWithin:  time.Minute,
	initError:   "start Claude",
	prepare: func(a *App) (providerStart, error) {
		var info AgentProviderInfo
		for _, p := range a.AgentProviders() {
			if p.ID == "claude" {
				info = p
			}
		}
		if !info.Available {
			return providerStart{}, errors.New(info.Detail)
		}
		if _, err := claudeagent.CheckNode(context.Background(), info.Path); err != nil {
			return providerStart{}, err
		}
		key := ""
		if info.Claude.APIKey {
			var err error
			if key, err = claudeKeys.get(); err != nil {
				return providerStart{}, fmt.Errorf("the Claude API key could not be read from the keychain: %w", err)
			}
		}
		neutral, err := agentStartupDir()
		if err != nil {
			return providerStart{}, err
		}
		return providerStart{launch: func(handler acp.Handler) (agentConnection, error) {
			return claudeagent.Start(info.Path, configDir(), neutral, key, handler, nil)
		}}, nil
	},
}

func (a *App) claudeAccountReport(params json.RawMessage) {
	var n struct {
		AuthStatus struct {
			Kind  string `json:"kind"`
			Label string `json:"label"`
		} `json:"authStatus"`
	}
	if json.Unmarshal(params, &n) != nil || n.AuthStatus.Kind == "" {
		return
	}
	acct := ProviderAccount{Provider: "claude", Kind: n.AuthStatus.Kind, Label: strings.TrimSpace(n.AuthStatus.Label),
		Subscription: n.AuthStatus.Kind == "account"}
	a.reportAccount(acct)
}
