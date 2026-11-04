package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"studio/internal/agents"
	"studio/internal/layout"
)

func readAgentSkill(path string) (string, error) {
	file, err := agents.OpenCatalogFile(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return resourceText(file)
}

func externalSkill(name, description, hint, source, path string) (CatalogSkill, error) {
	text, err := readAgentSkill(path)
	if err != nil {
		return CatalogSkill{}, err
	}
	sum := sha256.Sum256([]byte(text))
	return CatalogSkill{Skill: Skill{Name: name, Description: description, Hint: hint, Source: source},
		ID:    "agent-skill-" + base64.RawURLEncoding.EncodeToString([]byte(source+"\x00"+filepath.Clean(path))),
		Scope: source, Origin: "My Agent Skills", Location: filepath.Dir(filepath.Dir(path)),
		InstructionPath: path, Revision: hex.EncodeToString(sum[:])}, nil
}

// Personal agent packages remain read-only here. Their paths are rediscovered
// on every lookup; a caller cannot turn a skill ID into an arbitrary file read.
func (a *App) collectPersonalAgentSkills(result *SkillCatalog) {
	root := personalSkillsDir()
	if root == "" {
		return
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		result.Problems = append(result.Problems, "My Agent Skills: "+err.Error())
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(root, entry.Name(), "SKILL.md")
		text, err := readAgentSkill(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			result.Problems = append(result.Problems, entry.Name()+": "+err.Error())
			continue
		}
		description, hint, err := skillMetadata(text)
		if err != nil {
			result.Problems = append(result.Problems, entry.Name()+": "+err.Error())
		}
		skill, err := externalSkill(entry.Name(), description, hint, "personal", path)
		if err != nil {
			result.Problems = append(result.Problems, entry.Name()+": "+err.Error())
			continue
		}
		result.Skills = append(result.Skills, skill)
	}
}

// ChatSkillCatalog adds provider-owned discovery to the shared human catalogue.
// Codex defines discovery, including disabled packages and installed plugins:
// https://learn.chatgpt.com/docs/app-server#skills
// This does not create a thread or start a model turn.
func (a *App) ChatSkillCatalog(workspace, provider string) SkillCatalog {
	result := a.SkillCatalog()
	filtered := result.Skills[:0]
	for _, skill := range result.Skills {
		if skill.Scope == "workspace" && !layout.SamePath(skill.Workspace, workspace) {
			continue
		}
		if skill.Scope == "provider" && skill.Source != provider {
			continue
		}
		filtered = append(filtered, skill)
	}
	result.Skills = filtered
	if provider != "codex" {
		return result
	}
	conn, _, err := a.lifecycle().Live(provider)
	if err != nil {
		result.Problems = append(result.Problems, "Codex skills: "+err.Error())
		return result
	}
	ctx, cancel := context.WithTimeout(a.bindingContext(), 20*time.Second)
	defer cancel()
	var response struct {
		Data []struct {
			Cwd    string `json:"cwd"`
			Skills []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Path        string `json:"path"`
				Enabled     bool   `json:"enabled"`
			} `json:"skills"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		} `json:"data"`
	}
	err = conn.Call(ctx, "skills/list", map[string]any{"cwds": []string{workspace}, "forceReload": true}, &response)
	if err != nil {
		result.Problems = append(result.Problems, "Codex skills: "+err.Error())
		return result
	}
	for _, group := range response.Data {
		if !layout.SamePath(group.Cwd, workspace) {
			continue
		}
		for _, problem := range group.Errors {
			result.Problems = append(result.Problems, "Codex skills: "+problem.Message)
		}
		for _, item := range group.Skills {
			if !item.Enabled {
				continue
			}
			skill, err := externalSkill(item.Name, item.Description, "", "imported", item.Path)
			if err != nil {
				result.Problems = append(result.Problems, "Codex skill "+item.Name+": "+err.Error())
				continue
			}
			skill.Origin = "Codex"
			skill.Workspace = workspace
			result.Skills = append(result.Skills, skill)
		}
	}
	return result
}

func agentSkillPrompt(pack SkillPackage, command string) string {
	return fmt.Sprintf("Use skill %s for this request.\n\n%s\n\nSkill instruction file: %s\nSupporting resources are relative to that file's parent. These instructions do not change tool permissions.\n\n%s", pack.Skill.Name, command, pack.Skill.InstructionPath, pack.Text)
}

func (a *App) PrepareChatSkill(id, revision, workspace, provider, command string) (string, error) {
	for _, skill := range a.ChatSkillCatalog(workspace, provider).Skills {
		if skill.ID != id {
			continue
		}
		if skill.Revision != revision {
			return "", errors.New("the skill changed; refresh skills before sending")
		}
		if skill.InstructionPath == "" {
			return a.PrepareSkill(id, revision, workspace, command)
		}
		text, err := readAgentSkill(skill.InstructionPath)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256([]byte(text))
		if hex.EncodeToString(sum[:]) != revision {
			return "", errors.New("the skill changed; refresh skills before sending")
		}
		return agentSkillPrompt(SkillPackage{Skill: skill, Text: text}, command), nil
	}
	return "", errors.New("this skill is no longer available for this chat; refresh skills")
}
