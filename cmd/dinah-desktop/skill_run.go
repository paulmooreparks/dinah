package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"studio/internal/agents"
	"studio/internal/layout"
	"studio/internal/workspaces"
)

// SkillChat is the chat a skill runs in: its workspace and session.
type SkillChat struct {
	Workspace string `json:"workspace"`
	SessionID string `json:"sessionId"`
}

// StartSkill runs a skill in a new chat of the workspace, with the provider
// and, when one is given, the model chosen. The chat's first prompt is the
// skill's prepared instructions, so Markdown and template syntax in a
// package are text the agent reads and nothing else. The person follows,
// answers and stops the run in the chat like any other.
func (a *App) StartSkill(id, revision, workspace, provider, model, command string) (SkillChat, error) {
	if !supportedPreferenceProvider(provider) {
		return SkillChat{}, errors.New("choose a supported skill provider")
	}
	if strings.ContainsAny(model, "\r\n") {
		return SkillChat{}, errors.New("invalid model identifier")
	}
	registered := false
	for _, record := range (workspaceVFS{service: a.resourceService()}).records() {
		registered = registered || layout.SamePath(workspaces.ExecutionPath(record), workspace)
	}
	if !registered {
		return SkillChat{}, errors.New("choose a registered workspace")
	}
	pack, err := a.ReadSkill(id)
	if err != nil {
		return SkillChat{}, err
	}
	if pack.Skill.Ref == nil {
		return SkillChat{}, errors.New("provider commands must be used in a compatible chat")
	}
	prompt, err := a.PrepareSkill(id, revision, workspace, command)
	if err != nil {
		return SkillChat{}, err
	}
	info, err := a.NewSessionWithProviderAt(workspace, provider)
	if err != nil {
		return SkillChat{}, err
	}
	s := a.session(info.ID)
	if s == nil {
		return SkillChat{}, errors.New("the new chat could not be found")
	}
	if model != "" {
		if err := a.useSkillModel(s, model); err != nil {
			return SkillChat{}, err
		}
	}
	title := "Skill: " + pack.Skill.Name
	if titles, err := a.RenameSession(s.ID(), title); err == nil {
		a.emit("agent:titles", titles)
	}
	if err := a.turnIO().SendAs(s, title, prompt); err != nil {
		return SkillChat{}, err
	}
	return SkillChat{Workspace: workspace, SessionID: s.ID()}, nil
}

// useSkillModel selects a model through the session's model option.
func (a *App) useSkillModel(s *Session, model string) error {
	value, _ := json.Marshal(model)
	for _, option := range a.SessionOptions(s.ID()) {
		if option.Category != "model" || option.Type != "select" {
			continue
		}
		if !agents.Allowed(option, value) {
			return fmt.Errorf("the agent does not offer model %q", model)
		}
		_, err := a.SetSessionOption(s.ID(), option.ID, value)
		return err
	}
	return errors.New("the agent offers no model selector for this chat")
}
