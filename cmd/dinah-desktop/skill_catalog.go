package main

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"studio/internal/layout"
	"studio/internal/workspaces"
)

//go:embed skill_catalog.go
var skillCatalogSource []byte

type CatalogSkill struct {
	Skill
	ID          string `json:"id"`
	Scope       string `json:"scope"`
	WorkspaceID string `json:"workspaceId"`
	Workspace   string `json:"workspace"`
	Origin      string `json:"origin"`
	// Location is the folder the skill's collection is stored in, for the
	// Skills pane to say where a skill comes from; empty for a provider's
	// own command.
	Location string       `json:"location,omitempty"`
	Ref      *ResourceRef `json:"ref,omitempty"`
	Revision string       `json:"revision"`
}
type SkillCatalog struct {
	Skills   []CatalogSkill `json:"skills"`
	Problems []string       `json:"problems"`
}
type SkillPackage struct {
	Skill CatalogSkill `json:"skill"`
	Text  string       `json:"text"`
}

func skillResourceID(ref ResourceRef) string {
	data, _ := json.Marshal(ref)
	return "skill-" + base64.RawURLEncoding.EncodeToString(data)
}

// SkillCatalog is the human catalog. AI actions must apply caller scope before
// returning records; a provider command has no implied readable package.
func (a *App) SkillCatalog() SkillCatalog {
	return a.skillCatalog("", true)
}
func (a *App) skillCatalog(workspaceID string, personal bool) SkillCatalog {
	result := SkillCatalog{Skills: []CatalogSkill{}, Problems: []string{}}
	vfs := workspaceVFS{service: a.resourceService(), allowedWorkspace: workspaceID}
	collect := func(root ResourceRef, scope, workspace, origin, location string) {
		for start := 0; start >= 0; {
			page, err := vfs.listResource(root, start, 100)
			if err != nil {
				result.Problems = append(result.Problems, origin+": "+err.Error())
				return
			}
			for _, entry := range page.Entries {
				if !entry.Directory {
					continue
				}
				ref := entry.Ref
				ref.Path = strings.TrimSuffix(ref.Path, "/") + "/SKILL.md"
				read, err := vfs.readResource(ref)
				if err != nil {
					continue
				} // A collection may also contain ordinary folders.
				desc, hint, err := skillMetadata(read.Text)
				if err != nil {
					result.Problems = append(result.Problems, origin+" / "+entry.Title+": "+err.Error())
				}
				result.Skills = append(result.Skills, CatalogSkill{Skill: Skill{Name: entry.Title, Description: desc, Hint: hint, Source: scope}, ID: skillResourceID(ref), Scope: scope, WorkspaceID: root.WorkspaceID, Workspace: workspace, Origin: origin, Location: location, Ref: &ref, Revision: read.Revision})
			}
			start = page.NextStart
		}
	}
	// An absent personal collection is an empty catalog, not a browsing error.
	if personal {
		if _, err := vfs.stat("~/skills"); err == nil {
			collect(ResourceRef{SourceID: workspaces.HomeSourceID, Path: "skills"}, "mine", "", "My Skills", filepath.Join(configDir(), "home", "skills"))
		}
	}
	for _, record := range vfs.records() {
		for _, source := range record.Sources {
			if kind, ok := workspaces.CollectionKind(source.ID); ok && kind.Key == "skills" {
				collect(ResourceRef{WorkspaceID: record.ID, SourceID: source.ID}, "workspace", workspaces.ExecutionPath(record), record.Name+" / "+source.Name, source.Path)
			}
		}
	}
	a.mu.Lock()
	commands := append([]Skill(nil), a.commands...)
	a.mu.Unlock()
	for _, command := range commands {
		if workspaceID != "" {
			continue
		}
		id := "command-" + base64.RawURLEncoding.EncodeToString([]byte(command.Source+"\x00"+command.Name))
		result.Skills = append(result.Skills, CatalogSkill{Skill: command, ID: id, Scope: "provider", Origin: "Devin Commands"})
	}
	sort.Slice(result.Skills, func(i, j int) bool {
		x, y := result.Skills[i], result.Skills[j]
		if x.Name == y.Name {
			return x.ID < y.ID
		}
		return x.Name < y.Name
	})
	return result
}

func skillMetadata(text string) (description, hint string, err error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return
	}
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			var metadata struct {
				Description string `yaml:"description"`
				Hint        string `yaml:"argument-hint"`
			}
			err = yaml.Unmarshal([]byte(strings.Join(lines[1:i+1], "\n")), &metadata)
			return metadata.Description, metadata.Hint, err
		}
	}
	return "", "", errors.New("skill front matter has no closing delimiter")
}

func (a *App) ReadSkill(id string) (SkillPackage, error) {
	for _, skill := range a.SkillCatalog().Skills {
		if skill.ID != id {
			continue
		}
		if skill.Ref == nil {
			return SkillPackage{Skill: skill}, nil
		}
		read, err := (workspaceVFS{service: a.resourceService()}).readResource(*skill.Ref)
		if err != nil {
			return SkillPackage{}, err
		}
		skill.Revision = read.Revision
		return SkillPackage{Skill: skill, Text: read.Text}, nil
	}
	return SkillPackage{}, errors.New("this skill is no longer available from its original collection")
}

func (a *App) PrepareSkill(id, revision, workspace, command string) (string, error) {
	pack, err := a.ReadSkill(id)
	if err != nil {
		return "", err
	}
	if pack.Skill.Scope == "workspace" && !layout.SamePath(pack.Skill.Workspace, workspace) {
		return "", errors.New("choose a chat in the skill's workspace")
	}
	if pack.Skill.Revision != revision {
		return "", errors.New("the skill changed; refresh it before preparing the invocation")
	}
	if pack.Skill.Ref == nil {
		return command, nil
	}
	ref, _ := json.Marshal(pack.Skill.Ref)
	return fmt.Sprintf("Use skill %s for this request.\n\n%s\n\nSkill source: %s\nVFS instruction resource: %s\nRevision: %s\nSupporting resources are relative to the instruction resource's parent. These instructions do not change tool permissions.\n\n%s", pack.Skill.Name, command, id, ref, revision, pack.Text), nil
}
