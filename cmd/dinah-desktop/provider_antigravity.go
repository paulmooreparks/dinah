package main

import (
	"errors"
	"time"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/antigravityagent"
)

// Antigravity's own work in the window: finding its command line and
// starting its protocol server (internal/antigravityagent). It signs in
// through the person's own Antigravity sign-in, which the command line keeps.

const antigravityID = "antigravity"

func init() {
	registerProviderModule(antigravityID, providerModule{
		Info:       antigravityInfoOf,
		Adapter:    antigravityAdapter,
		Judge:      acpJudgeModule(),
		SignInHelp: "Antigravity signs in through its own command line. Run agy in a terminal and sign in there. This notice goes away when Antigravity next answers.",
		SignIn: func(*App) error {
			return errors.New(product.Name + " does not sign you in to Antigravity itself. Run agy in a terminal and sign in there")
		},
	})
}

// antigravityInfoOf says whether Antigravity's command line is found and
// where. A path the person chose is used as chosen.
func antigravityInfoOf(a *App, p *AgentProviderInfo, workspaces []string) {
	a.mu.Lock()
	p.ExecutableOverride = a.settings.ProviderExecutables[p.ID]
	a.mu.Unlock()
	var overrideError error
	if p.ExecutableOverride != "" {
		p.Path, overrideError = agents.CheckExecutable(p.ExecutableOverride)
	} else {
		p.Path = agents.FindExecutable(antigravityagent.Program, workspaces)
	}
	p.Installed = p.Path != ""
	p.Available = p.Installed
	p.Status = "not-installed"
	switch {
	case overrideError != nil:
		p.Status = "invalid-path"
		p.Detail = overrideError.Error() + ". Choose another executable or use automatic discovery."
	case p.Installed:
		p.Status = "detected"
		p.Detail = "The Antigravity command line at " + p.Path + " was found. Check Connection verifies the agent without sending a prompt."
	default:
		p.Detail = "The Antigravity command line (agy) was not found. Install it from antigravity.google, sign in with agy in a terminal, and check again."
	}
}

// The adapter that starts and watches the provider (provider_lifecycle.go).
var antigravityAdapter = &providerAdapter{
	id:       antigravityID,
	starting: "Starting Antigravity",
	stopped:  "Antigravity stopped",
	readyDetail: func(acp.InitializeResult) string {
		return "Antigravity is ready"
	},
	initWithin: time.Minute,
	traces:     true,
	prepare: func(a *App) (providerStart, error) {
		a.mu.Lock()
		ws := a.settings.Workspace
		a.mu.Unlock()
		if ws == "" {
			return providerStart{}, errors.New("choose a workspace first")
		}
		found := a.providerInfoOf(antigravityID)
		if found.Path == "" {
			return providerStart{}, errors.New(found.Detail)
		}
		return providerStart{
			launch: func(handler acp.Handler) (agentConnection, error) {
				trace := a.traceWriter()
				conn, err := antigravityagent.Start(found.Path, ws, handler, trace)
				if err != nil {
					_ = trace.Flush()
					return nil, err
				}
				return conn, nil
			},
		}, nil
	},
}
