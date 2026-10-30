package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/codexagent"
)

// Codex's own work in the window: finding its program, checking its connection
// and account, signing in to ChatGPT, and its approval settings.

func init() {
	registerProviderModule("codex", providerModule{
		Info:          codexInfo,
		Check:         codexCheck,
		SignIn:        (*App).signInCodex,
		AuthRequired:  codexAuthRequired,
		SessionParams: codexSessionParams,
		Adapter:       codexAdapter,
		Judge:         acpJudgeModule(),
		Unattended: func(s Settings) bool {
			how := codexApprovals(s)
			mode := codexAppletApprovals(s.CodexAppletApprovals)
			return mode == "allow" || mode == "all" || how == codexagent.ApprovalsWorkspace || how == codexagent.ApprovalsFull
		},
	})
}

// codexInfo says whether Codex's program is found, and where.
func codexInfo(a *App, p *AgentProviderInfo, workspaces []string) {
	a.mu.Lock()
	p.ExecutableOverride = a.settings.ProviderExecutables[p.ID]
	p.Pinned = append([]string{}, a.settings.ProviderPinned[p.ID]...)
	a.mu.Unlock()
	var overrideError error
	if p.ExecutableOverride != "" {
		p.Path, overrideError = agents.CheckExecutable(p.ExecutableOverride)
	} else {
		p.Path = agents.FindExecutable(p.ID, workspaces)
	}
	p.Installed = p.Path != ""
	p.Available = p.Installed
	p.Status = "not-installed"
	if p.Installed {
		p.Status = "detected"
		p.Detail = "Installation found. Check Connection verifies the agent without sending a prompt."
	}
	if overrideError != nil {
		p.Status = "invalid-path"
		p.Detail = overrideError.Error() + ". Choose another executable or use automatic discovery."
	} else if !p.Installed {
		p.Detail = "Codex CLI was not found in the executable search paths. Choose its executable if it is installed elsewhere."
	}
}

// codexCheck reads Codex's account, and its catalog of models.
func codexCheck(a *App, conn agentConnection, info *AgentProviderInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var account struct {
		Account      json.RawMessage `json:"account"`
		RequiresAuth bool            `json:"requiresOpenaiAuth"`
	}
	if err := conn.Call(ctx, "account/read", map[string]any{"refreshToken": false}, &account); err != nil {
		info.Status, info.Detail = "failed", err.Error()
		return
	}
	if account.RequiresAuth && (len(account.Account) == 0 || string(account.Account) == "null") {
		info.Status, info.Detail = "sign-in-required", "Sign in with ChatGPT to use Codex."
	} else {
		var identity struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(account.Account, &identity)
		if identity.Type == "apiKey" {
			info.Detail = "Codex is using API credentials with separate usage-based billing."
		}
		if identity.Type == "chatgpt" {
			info.Detail = "Codex is signed in with ChatGPT. Model access is checked when a turn runs."
		}
	}
	if c, ok := conn.(*codexagent.Conn); ok {
		info.Catalog, _ = c.Catalog(ctx)
	}
	if len(info.Catalog) > 0 {
		a.keepProviderCatalog(info.ID, info.Catalog)
	}
}

// signInCodex opens the browser for Codex's ChatGPT sign-in.
func (a *App) signInCodex() error {
	conn, _, err := a.lifecycle().Live("codex")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var login struct {
		AuthURL string `json:"authUrl"`
	}
	if err := conn.Call(ctx, "account/login/start", map[string]string{"type": "chatgpt"}, &login); err != nil {
		return err
	}
	if login.AuthURL == "" {
		return errors.New("Codex did not return a sign-in address")
	}
	if err := a.openInBrowser(login.AuthURL); err != nil {
		return err
	}
	a.emit("agent:auth", map[string]any{"provider": "codex", "state": "signing-in", "method": "Sign In to ChatGPT"})
	return nil
}

// codexAuthRequired is what a request Codex refused for want of a sign-in
// tells the page.
func codexAuthRequired(a *App) error {
	a.emit("agent:auth", map[string]any{"state": "required", "provider": "codex", "method": "Sign In to ChatGPT"})
	return errors.New("Codex needs you to sign in. Open Settings > Agents and choose Sign In")
}

// codexSessionParams add Codex's approval settings to a session's parameters.
func codexSessionParams(a *App, p acp.LoadSessionParams) any {
	a.mu.Lock()
	mode := codexAppletApprovals(a.settings.CodexAppletApprovals)
	how := codexApprovals(a.settings)
	a.mu.Unlock()
	return codexagent.SessionParams{LoadSessionParams: p, AppletApprovals: mode, Approvals: how}
}

func codexAppletApprovals(mode string) string {
	if mode == "allow" || mode == "all" {
		return mode
	}
	return "ask"
}

// SetCodexAppletApprovals updates open chats as well as future threads.
func (a *App) SetCodexAppletApprovals(mode string) error {
	if mode != "ask" && mode != "allow" && mode != "all" {
		return errors.New("choose Ask Before Invoking, Allow Bundled Applet Calls or Approve Everything")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.CodexAppletApprovals = mode
		// The change runs under a.mu, so the running Codex is told here.
		if p := a.runtime.ProviderLocked("codex"); p != nil {
			if live, ok := p.Connection().(*codexagent.Conn); ok {
				live.SetApprovals(mode)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// The adapter that starts and watches the provider (provider_lifecycle.go).
var codexAdapter = &providerAdapter{
	id:          "codex",
	starting:    "Starting Codex",
	stopped:     "Codex stopped",
	readyDetail: connectedDetail("Codex"),
	initWithin:  20 * time.Second,
	initError:   "initialize Codex",
	prepare: func(a *App) (providerStart, error) {
		var executable string
		for _, info := range a.AgentProviders() {
			if info.ID == "codex" {
				executable = info.Path
			}
		}
		if executable == "" {
			return providerStart{}, errors.New("Codex was not found. Install the Codex CLI and scan again in Settings > Agents")
		}
		neutral, err := agentStartupDir()
		if err != nil {
			return providerStart{}, err
		}
		return providerStart{launch: func(handler acp.Handler) (agentConnection, error) {
			return codexagent.Start(executable, []string{"app-server"}, neutral, handler, nil)
		}}, nil
	},
}
