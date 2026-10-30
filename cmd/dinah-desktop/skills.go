package main

import (
	"errors"
	"sort"
	"strings"

	"studio/internal/agents"
	"studio/internal/layout"
	"studio/internal/sessionhost"
	"studio/internal/workspaces"
)

// Skill is a command the person can start with "/": a skill of the
// workspace, one of Devin's own commands, or a skill Devin imports from
// another tool.
type Skill = sessionhost.Command

// WorkspaceSkills lists the skills in the workspace's .devin/skills folder,
// where Devin documents that it finds a project's skills
// (https://docs.devin.ai/cli/extensibility/skills/overview), each with the
// description from its SKILL.md front matter. It needs no session, so the
// skills menu works before Devin has started.
func (a *App) WorkspaceSkills() []Skill {
	return workspaceSkillsAt(a.currentWorkspace())
}

func workspaceSkillsAt(ws string) []Skill {
	if ws == "" {
		return []Skill{}
	}
	// Legacy callers use the same confined provider collection adapter. The
	// desktop catalog uses the workspace registry's persisted source identities.
	a := legacySkillCatalog(ws)
	out := []Skill{}
	have := map[string]bool{}
	for _, skill := range a.skillCatalog("legacy-skills", false).Skills {
		if skill.Scope == "workspace" {
			out = append(out, skill.Skill)
			have[skill.Name] = true
		}
	}
	// The person's own skills, which Devin reads in every workspace, follow
	// the workspace's; a workspace skill of the same name is the one listed.
	for _, skill := range personalSkills() {
		if !have[skill.Name] {
			out = append(out, skill)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// legacySkillCatalog is a scratch App whose one workspace is the folder ws,
// for the skill lookups that are given a folder rather than a workspace. It
// is never started, and its settings are never saved.
func legacySkillCatalog(ws string) *App {
	records := []WorkspaceRecord{{ID: "legacy-skills", Name: "Workspace", ExecutionPath: ws, Sources: []WorkspaceSource{{ID: "legacy-source", Kind: "local-folder", Name: "Workspace", Path: ws}}}}
	return &App{settings: Settings{WorkspaceRecords: records}}
}

// Skills is the whole skills menu: the workspace's skills, then whatever
// else Devin last offered that is not one of them.
func (a *App) Skills() []Skill {
	return a.SkillsAt(a.currentWorkspace())
}

// SkillsAt resolves commands for the chat's workspace.
func (a *App) SkillsAt(workspace string) []Skill {
	out := workspaceSkillsAt(workspace)
	have := map[string]bool{}
	for _, s := range out {
		have[s.Name] = true
	}
	a.mu.Lock()
	cmds := append([]Skill(nil), a.commands...)
	a.mu.Unlock()
	for _, c := range cmds {
		if !have[c.Name] {
			out = append(out, c)
		}
	}
	return out
}

// SkillsForProviderAt keeps vendor commands out of another provider's picker.
// Workspace skill instructions have one source and can be supplied as text.
func (a *App) SkillsForProviderAt(workspace, provider string) []Skill {
	if providerRegistry[normalizedProvider(provider)].OwnCommands {
		return a.SkillsAt(workspace)
	}
	return workspaceSkillsAt(workspace)
}

func (a *App) providerSkillPrompt(s *Session, text string) (string, error) {
	if providerRegistry[normalizedProvider(s.Provider())].OwnCommands || !strings.HasPrefix(strings.TrimSpace(text), "/") {
		return text, nil
	}
	name := strings.Fields(strings.TrimSpace(text))[0][1:]
	var catalog SkillCatalog
	for _, record := range (workspaceVFS{service: a.resourceService()}).records() {
		if layout.SamePath(workspaces.ExecutionPath(record), s.Cwd()) {
			catalog = a.skillCatalog(record.ID, false)
			break
		}
	}
	if catalog.Skills == nil {
		a = legacySkillCatalog(s.Cwd())
		catalog = a.skillCatalog("legacy-skills", false)
	}
	var selected *CatalogSkill
	for _, skill := range catalog.Skills {
		if skill.Name != name {
			continue
		}
		if selected != nil {
			return "", errors.New("multiple skills have this name; choose one from the Skills browser")
		}
		selected = &skill
	}
	if selected != nil {
		read, err := (workspaceVFS{service: a.resourceService(), allowedWorkspace: selected.WorkspaceID}).readResource(*selected.Ref)
		if err != nil {
			return "", err
		}
		return agents.SkillPrompt(name, text, read.Text), nil
	}
	if instructions, ok := personalSkillText(name); ok {
		return agents.SkillPrompt(name, text, instructions), nil
	}
	return text, nil
}
