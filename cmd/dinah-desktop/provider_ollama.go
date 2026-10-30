package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"studio/internal/acp"
	"studio/internal/judge"
	"studio/internal/ollamaagent"
)

// Ollama's own work in the window: finding its server, checking its models, and
// its approval settings.

func init() {
	registerProviderModule("ollama", providerModule{
		Info:    func(_ *App, p *AgentProviderInfo, _ []string) { ollamaInfo(p) },
		Adapter: ollamaAdapter,
		Context: &contextModule{State: ollamaContextState, SetAside: ollamaSetAside, Choose: ollamaChoose},
		Judge: &judgeModule{
			Build: func(_ *App, _, model string) judge.Judge {
				return &judge.Ollama{Host: ollamaagent.Host(), Model: model}
			},
			Installed: func(_ *App, ctx context.Context, _ string) ([]judge.Model, error) {
				host := ollamaagent.Host()
				names, err := ollamaagent.ListModels(ctx, host)
				if err != nil {
					return nil, err
				}
				// A model that is already loaded, and a cloud model, run without
				// loading anything new into memory. /api/ps is documented, and the
				// cloud models are told by the name Paul chose (context_window.go).
				loaded, _ := ollamaagent.LoadedModels(ctx, host)
				out := make([]judge.Model, 0, len(names))
				for _, n := range names {
					out = append(out, judge.Model{Name: n, Resident: ollamaagent.IsCloudModel(n) || slices.Contains(loaded, n)})
				}
				return out, nil
			},
		},
		Unattended: func(s Settings) bool {
			how := ollamaApprovals(s)
			return how == ollamaagent.ApprovalsNone || how == ollamaagent.ApprovalsAll
		},
		Check: ollamaCheck,
		SignIn: func(*App) error {
			return errors.New("Ollama needs no sign-in; a cloud model uses the account Ollama itself is signed in to")
		},
		SessionParams: ollamaSessionParams,
	})
}

// releaseOldDefaultContext drops a saved context of 65536 from the options kept
// for an Ollama chat, once. That length was every chat's default before a chat
// could leave the length to Ollama, and the window saves a chat's options and
// applies them when the chat is opened, so it was applied as the person's own
// choice and put a cloud model's chat back on 64K, over its real window, after
// each restart (30 October 2026). A person who chose 65536 deliberately has it
// released once, and chooses it again if they want it. A length of any other
// size is kept.
func releaseOldDefaultContext(s *Settings) {
	if s.DefaultContextReleased {
		return
	}
	s.DefaultContextReleased = true
	old, _ := json.Marshal(strconv.Itoa(ollamaagent.OldDefaultContext))
	for id, options := range s.SessionOptions {
		if s.SessionProviders[id] != "ollama" && !strings.HasPrefix(id, "ollama:") {
			continue
		}
		if value, ok := options["context"]; ok && bytes.Equal(bytes.TrimSpace(value), old) {
			delete(options, "context")
		}
	}
}

// ollamaCheck reads Ollama's catalog of models.
func ollamaCheck(a *App, conn agentConnection, info *AgentProviderInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if c, ok := conn.(*ollamaagent.Conn); ok {
		var err error
		if info.Catalog, err = c.Catalog(ctx); err != nil {
			info.Status, info.Detail = "failed", err.Error()
			return
		}
	}
	info.Detail = "Ollama is connected. No prompt was sent."
	if len(info.Catalog) > 0 {
		a.keepProviderCatalog(info.ID, info.Catalog)
	}
}

// ollamaSessionParams add Ollama's approval settings to a session's
// parameters. Dinah runs Ollama's tool calls, so when they ask the person is
// the person's own choice, in Settings, as Codex's applet actions are.
func ollamaSessionParams(a *App, p acp.LoadSessionParams) any {
	a.mu.Lock()
	commands := ollamaApprovals(a.settings)
	a.mu.Unlock()
	return ollamaagent.SessionParams{LoadSessionParams: p, AppletApprovals: codexAppletApprovals(commands), CommandApprovals: commands}
}

// ollamaInfo says whether Ollama answers at its address.
func ollamaInfo(p *AgentProviderInfo) {
	host := ollamaagent.Host()
	p.Path = host
	version, err := ollamaagent.Probe()
	if err != nil {
		p.Status = "not-running"
		p.Detail = "Ollama did not answer at " + host + ". Start Ollama, or set OLLAMA_HOST to where it runs, and scan again."
		return
	}
	p.Installed, p.Available, p.Status = true, true, "detected"
	p.Detail = "Ollama " + version + " answers at " + host + ". " + product.Name + " runs its models' tool calls, asking you before a command runs or a file outside the workspace is written."
}

// ollamaStateDir is where Dinah keeps Ollama conversations, which
// Ollama does not keep.
func ollamaStateDir() string { return configDir() }

// The adapter that starts and watches the provider (provider_lifecycle.go).
var ollamaAdapter = &providerAdapter{
	id:          "ollama",
	starting:    "Connecting to Ollama",
	stopped:     "The connection to Ollama stopped",
	readyDetail: connectedDetail("Ollama"),
	initWithin:  20 * time.Second,
	initError:   "connect to Ollama",
	prepare: func(a *App) (providerStart, error) {
		return providerStart{launch: func(handler acp.Handler) (agentConnection, error) {
			conn, err := ollamaagent.Start(ollamaagent.Host(), ollamaStateDir(), handler)
			if err != nil {
				return nil, err
			}
			a.useOllamaFileScope(conn)
			a.useOllamaRounds(conn)
			return conn, nil
		}}, nil
	},
}

// agentStartupDir is the folder Codex and Claude start in, outside every
// workspace, so that discovery cannot load a project's configuration; each
// session is given its workspace.

// Dinah runs Ollama's tool calls itself, so the questions they raise are
// Dinah's rather than a provider's: a shell command, a write outside the
// workspace, and an applet action that changes something. Settings › Agents
// chooses when they are asked, as Codex's applet actions have their own
// choice there.

// ollamaApprovals is the policy an Ollama session runs under, whatever the
// saved settings hold: once in each chat, unless another was chosen.
func ollamaApprovals(s Settings) string {
	switch s.OllamaApprovals {
	case ollamaagent.ApprovalsEvery, ollamaagent.ApprovalsNone, ollamaagent.ApprovalsAll:
		return s.OllamaApprovals
	}
	return ollamaagent.ApprovalsChat
}

// SetOllamaApprovals chooses when Ollama's own tools ask the person.
func (a *App) SetOllamaApprovals(policy string) error {
	switch policy {
	case ollamaagent.ApprovalsEvery, ollamaagent.ApprovalsChat, ollamaagent.ApprovalsNone, ollamaagent.ApprovalsAll:
	default:
		return errors.New("choose Once in Each Chat, Every Command, Never or Approve Everything")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.OllamaApprovals = policy
		// The change runs under a.mu, so the running Ollama is told here.
		if p := a.runtime.ProviderLocked("ollama"); p != nil {
			if live, ok := p.Connection().(*ollamaagent.Conn); ok {
				live.SetApprovals(policy)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// ollamaContextState asks the adapter where a conversation's window stands.
func ollamaContextState(a *App, conn agentConnection, id string) (contextWindow, error) {
	var st ollamaagent.ContextState
	if err := a.callWithin(conn, 15*time.Second, ollamaagent.ContextMethod, map[string]string{"sessionId": id}, &st); err != nil {
		return contextWindow{}, err
	}
	return contextWindow{Used: st.Used, Size: st.Size, Full: st.Full, Choice: st.Choice, CanSetAside: st.CanSetAside}, nil
}

// ollamaSetAside asks the adapter to set a conversation's oldest turns aside.
func ollamaSetAside(a *App, conn agentConnection, id string) (int, error) {
	var res ollamaagent.SetAsideResult
	// A judge may choose the turns, from what the adapter offers. The adapter checks
	// the choice against the costs it recorded and refuses one the oldest-first rule
	// could not make, in which case, as when the judge has no answer, the oldest turns
	// are set aside as before.
	var offered ollamaagent.AsideTurns
	if err := a.callWithin(conn, 15*time.Second, ollamaagent.TurnsMethod, map[string]string{"sessionId": id}, &offered); err == nil && offered.Needed > 0 {
		turns := make([]setAsideTurn, len(offered.Turns))
		for i, t := range offered.Turns {
			turns[i] = setAsideTurn{N: t.N, Said: t.Said, Answered: t.Answered, Cost: t.Cost}
		}
		if chosen, ok := a.judgeSetAside(id, offered.Needed, turns); ok {
			err := a.callWithin(conn, 30*time.Second, ollamaagent.SetAsideMethod, map[string]any{"sessionId": id, "turns": chosen}, &res)
			if err == nil {
				return res.Turns, nil
			}
			logf("judge", "warning", "the turns a judge chose for chat %s were refused, so the oldest are set aside: %v", id, err)
		}
	}
	if err := a.callWithin(conn, 30*time.Second, ollamaagent.SetAsideMethod, map[string]string{"sessionId": id}, &res); err != nil {
		return 0, err
	}
	return res.Turns, nil
}

// ollamaChoose keeps the course the person chose for a conversation, or with ""
// forgets it.
func ollamaChoose(a *App, conn agentConnection, id, choice string) error {
	return a.callWithin(conn, 15*time.Second, ollamaagent.ChoiceMethod, map[string]string{"sessionId": id, "choice": choice}, nil)
}
