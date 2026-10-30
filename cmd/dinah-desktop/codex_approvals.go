package main

import (
	"errors"

	"studio/internal/codexagent"
)

// A Codex chat asks before it leaves its workspace sandbox unless the person
// chooses to let it run unattended, the choice in Settings › Agents. The
// sandbox stays the bound: a command that would write outside the workspace
// or reach the network is refused rather than put to the person, so the
// agent keeps working when nobody is watching.

// codexApprovals is the policy a Codex thread runs under, whatever the saved
// settings hold: ask, unless another was chosen.
func codexApprovals(s Settings) string {
	switch s.CodexApprovals {
	case codexagent.ApprovalsWorkspace, codexagent.ApprovalsFull:
		return s.CodexApprovals
	}
	return codexagent.ApprovalsAsk
}

// SetCodexApprovals chooses how far a Codex chat may go without asking.
func (a *App) SetCodexApprovals(policy string) error {
	switch policy {
	case codexagent.ApprovalsAsk, codexagent.ApprovalsWorkspace, codexagent.ApprovalsFull:
	default:
		return errors.New("choose Ask Before Commands, Workspace, Unattended or Full Access, Unattended")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.CodexApprovals = policy
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
