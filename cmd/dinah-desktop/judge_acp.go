package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/judge"
	"studio/internal/providers"
	"studio/internal/sessionhost"
)

// Judges on a provider reached through the protocol, such as Claude Code and Codex
// (docs/plans/judgement-agents.md, Judges on Every Provider). Each call opens a
// session of its own on the provider's running connection, in an empty folder, with
// no tool servers passed to it, sends one prompt, and ends the session. Every request
// the agent makes of the client in it is refused (sessionhost/judge_sessions.go). The
// agent reads its own user-wide settings, so a tool server the person put there may
// still be listed to it; it cannot use one, since every permission is refused.

// acpJudgeModule is the judge module of a provider reached through the protocol.
func acpJudgeModule() *judgeModule {
	return &judgeModule{
		Build: func(a *App, provider, model string) judge.Judge {
			return &acpJudge{app: a, provider: provider, model: model}
		},
		Installed: acpJudgeModels,
		Available: acpJudgeAvailable,
		Note:      "Each call starts a session and uses your plan's usage. Set a budget below.",
	}
}

// acpJudgeAvailable says why the provider cannot judge now, or "" when it can. Whether
// the person is signed in is found out when a call is made, and a call that finds they
// are not is answered by its rule.
func acpJudgeAvailable(a *App, provider string) string {
	info := a.providerInfoOf(provider)
	if !info.Installed || !info.Available {
		if info.StatusWords != "" {
			return info.StatusWords
		}
		return providerName(provider) + " is not installed"
	}
	return ""
}

// judgeSessionSet is the set of sessions that judgement calls have open.
func (a *App) judgeSessionSet() *sessionhost.JudgeSessions {
	j := &a.judging
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.sessions == nil {
		j.sessions = sessionhost.NewJudgeSessions()
	}
	return j.sessions
}

// acpJudge makes judgement calls on a provider's model.
type acpJudge struct {
	app      *App
	provider string
	model    string
}

// endTimeout is how long ending a call's session may take.
const judgeEndTimeout = 10 * time.Second

func (j *acpJudge) Ask(ctx context.Context, r judge.Request) (judge.Response, error) {
	a := j.app
	conn, init, err := a.lifecycle().Live(j.provider)
	if err != nil {
		return judge.Response{}, err
	}
	dir, err := os.MkdirTemp("", "dinah-judge-")
	if err != nil {
		return judge.Response{}, err
	}
	defer os.RemoveAll(dir)
	sessions := a.judgeSessionSet()
	ns, err := newJudgeSession(ctx, conn, sessions, dir)
	if err != nil {
		return judge.Response{}, err
	}
	callCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var cancelOnce sync.Once
	stop := func() {
		cancelOnce.Do(func() { _ = conn.Notify("session/cancel", acp.CancelParams{SessionID: ns.SessionID}) })
		cancel()
	}
	session := sessions.Register(ns.SessionID, stop)
	defer func() {
		endJudgeSession(conn, init, ns.SessionID)
		sessions.Unregister(ns.SessionID)
	}()
	model, err := j.chooseModel(callCtx, conn, ns)
	if err != nil {
		return judge.Response{}, err
	}
	var res acp.PromptResult
	err = conn.Call(callCtx, "session/prompt", acp.PromptParams{SessionID: ns.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: judgePrompt(r)}}}, &res)
	if asked := session.Refused(); len(asked) > 0 {
		return judge.Response{}, fmt.Errorf("the agent asked to act (%s), which a judge may not", strings.Join(asked, ", "))
	}
	if err != nil {
		return judge.Response{}, err
	}
	if res.StopReason != "" && res.StopReason != "end_turn" && res.StopReason != "max_tokens" {
		return judge.Response{}, fmt.Errorf("the agent stopped the call: %s", res.StopReason)
	}
	resp := judge.Response{Text: session.Text(), Provider: j.provider, Model: model}
	if used, reported := session.Usage(); reported {
		resp.Cost = judge.Cost{Unit: providers.Registry[j.provider].BudgetUnit, In: used}
	} else {
		resp.Cost = judge.Cost{Unknown: true}
	}
	return resp, nil
}

// judgePrompt is the one prompt of a call: what the judge is for, then its input.
func judgePrompt(r judge.Request) string {
	var b strings.Builder
	b.WriteString(r.System)
	b.WriteString("\n\nInput:\n")
	b.WriteString(r.Input)
	if r.JSON {
		b.WriteString("\n\nAnswer with the JSON only, and nothing else.")
	}
	return b.String()
}

// chooseModel selects the call's model among the session's, and returns the one
// the session runs.
func (j *acpJudge) chooseModel(ctx context.Context, conn agentConnection, ns acp.NewSessionResult) (string, error) {
	option, ok := modelOption(ns.ConfigOptions)
	if !ok {
		return j.model, nil
	}
	current := judgeOptionString(option.CurrentValue)
	if j.model == "" || j.model == current {
		return current, nil
	}
	if !slices.ContainsFunc(judgeOptionValues(option), func(v judgeOptionValue) bool { return v.Value == j.model }) {
		return "", fmt.Errorf("the agent does not offer the model %s", j.model)
	}
	value, _ := json.Marshal(j.model)
	var res acp.ConfigOptionsResult
	if err := conn.Call(ctx, "session/set_config_option", acp.SetConfigOptionParams{SessionID: ns.SessionID, ConfigID: option.ID, Value: json.RawMessage(value)}, &res); err != nil {
		return "", err
	}
	return j.model, nil
}

// newJudgeSession creates a call's session in dir, with no tool servers. The caller
// registers it at once with sessions.Register, which ends the time in which the
// session's first updates are held, because they may arrive before its id is known.
func newJudgeSession(ctx context.Context, conn agentConnection, sessions *sessionhost.JudgeSessions, dir string) (acp.NewSessionResult, error) {
	var ns acp.NewSessionResult
	sessions.Begin()
	if err := conn.Call(ctx, "session/new", acp.NewSessionParams{Cwd: dir, McpServers: []any{}}, &ns); err != nil {
		sessions.Abandon()
		return ns, err
	}
	return ns, nil
}

// endJudgeSession ends a call's session: deleted when the agent can, so that it is
// not among the agent's own sessions, else closed, else left, which the agent frees
// with its connection.
func endJudgeSession(conn agentConnection, init acp.InitializeResult, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), judgeEndTimeout)
	defer cancel()
	caps := init.AgentCapabilities.SessionCapabilities
	switch {
	case caps.Delete != nil:
		_ = conn.Call(ctx, "session/delete", acp.DeleteSessionParams{SessionID: id}, nil)
	case caps.Close != nil:
		_ = conn.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: id}, nil)
	default:
		_ = conn.Notify("session/cancel", acp.CancelParams{SessionID: id})
	}
}

// modelCategory is the category the protocol gives a session's model setting.
const modelCategory = "model"

func modelOption(options []acp.ConfigOption) (acp.ConfigOption, bool) {
	for _, o := range options {
		if o.Category == modelCategory {
			return o, true
		}
	}
	return acp.ConfigOption{}, false
}

type judgeOptionValue struct {
	Value string
	Name  string
}

// optionValues are the values a setting offers, flattened from its groups.
func judgeOptionValues(o acp.ConfigOption) []judgeOptionValue {
	var entries []struct {
		Value   string            `json:"value"`
		Name    string            `json:"name"`
		Options []json.RawMessage `json:"options"`
	}
	if json.Unmarshal(o.Options, &entries) != nil {
		return nil
	}
	var out []judgeOptionValue
	for _, e := range entries {
		if e.Value != "" {
			out = append(out, judgeOptionValue{Value: e.Value, Name: e.Name})
		}
		for _, raw := range e.Options {
			var v struct {
				Value string `json:"value"`
				Name  string `json:"name"`
			}
			if json.Unmarshal(raw, &v) == nil && v.Value != "" {
				out = append(out, judgeOptionValue{Value: v.Value, Name: v.Name})
			}
		}
	}
	return out
}

func judgeOptionString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// acpJudgeModels lists the models a provider offers, from the model setting that a
// new session reports, so that nothing is chosen here. None is resident: a hosted
// model loads nothing into this machine, and a call to it costs the person's plan, so
// only a calibration the person starts may try one.
func acpJudgeModels(a *App, ctx context.Context, provider string) ([]judge.Model, error) {
	conn, init, err := a.lifecycle().Live(provider)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "dinah-judge-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	sessions := a.judgeSessionSet()
	ns, err := newJudgeSession(ctx, conn, sessions, dir)
	if err != nil {
		return nil, err
	}
	sessions.Register(ns.SessionID, nil)
	defer func() {
		endJudgeSession(conn, init, ns.SessionID)
		sessions.Unregister(ns.SessionID)
	}()
	option, ok := modelOption(ns.ConfigOptions)
	if !ok {
		return nil, errors.New("the agent reports no model setting")
	}
	var out []judge.Model
	for _, v := range judgeOptionValues(option) {
		out = append(out, judge.Model{Name: v.Value})
	}
	return out, nil
}
