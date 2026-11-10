package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"studio/internal/acp"
)

// Devin as a judge (docs/plans/judgement-agents.md, A Process Only for Judging).
// A Devin chat may run under rules that allow every tool, and Devin reads its rules
// from the configuration file it is started with, so a judgement call cannot be
// made safely on the process the chats use. Devin judges in a process of its own
// whose configuration file denies every documented tool and pattern and allows
// nothing. Devin documents that a deny rule always wins over every other source of
// rules and that a denied action is blocked at once without a question
// (https://docs.devin.ai/cli/reference/permissions).

// devinJudgeRules deny what Devin's permissions reference lists: the bare tool
// names, reading and writing every file on the computer and under the folder the
// agent runs in, every web address, and every tool of a tool server. Nothing is
// allowed, so a tool the list does not cover is asked about, and the judge's
// connection refuses every question.
var devinJudgeRules = permissions{
	Allow: []string{},
	Deny: []string{
		"read", "edit", "grep", "glob", "exec",
		"Read(/**)", "Read(**)", "Write(/**)", "Write(**)",
		"Fetch(https://*)", "Fetch(http://*)",
		"mcp__*",
	},
}

// devinJudgeConfig writes the judge's configuration file and returns its path. It
// is written whole at every start, so nothing Devin saved in it since is kept.
func devinJudgeConfig() (string, error) {
	b, err := json.MarshalIndent(map[string]any{"permissions": devinJudgeRules}, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(configDir(), "devin-judge.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(b, '\n'), 0o600)
}

// startDevinJudge runs Devin for judging in dir.
func startDevinJudge(a *App, dir string, handler acp.Handler) (agentConnection, error) {
	found := a.providerInfoOf(devinID)
	if found.Path == "" {
		if found.Detail != "" {
			return nil, errors.New(found.Detail)
		}
		return nil, errors.New("the Devin command line was not found")
	}
	cfg, err := devinJudgeConfig()
	if err != nil {
		return nil, err
	}
	conn, err := acp.Start(found.Path, []string{"--config", cfg, "acp"}, dir, handler, acp.Options{Agent: "Devin"})
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// devinJudgeModule is Devin's judge module. The protocol reports a call's usage in
// tokens while Devin's plan is counted in its own unit, so the cost of a call is
// not known in the unit a budget is set in, and its models are not calibrated.
func devinJudgeModule() *judgeModule {
	return processJudgeModule(startDevinJudge,
		"Each call starts a session and uses your plan's usage. The cost of a call is not measured, so a budget cannot limit it.",
		"Devin's models are not calibrated, because a run tries many models on your plan and the cost of a call is not measured, so no budget could stop it. Choose a model instead.")
}
