package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"path"
	"sort"
)

// Built-in skills are skills that are part of the program, kept in builtin-skills as a folder with a SKILL.md file
// each, as any collection of skills is. They are listed in every catalogue, to the person and to agents through
// dinah.skills/list@1, and are read only. A product built from this source can point its own skills at them.

//go:embed builtin-skills
var builtinSkillFiles embed.FS

// builtinSkillScope is the scope of a built-in skill in the catalogue.
const builtinSkillScope = "builtin"

// builtinSkills are the built-in skills, by name, in name order.
func builtinSkills() []CatalogSkill {
	entries, err := fs.ReadDir(builtinSkillFiles, "builtin-skills")
	if err != nil {
		return nil
	}
	var out []CatalogSkill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := builtinSkillFiles.ReadFile(path.Join("builtin-skills", entry.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		text := string(data)
		description, hint, _ := skillMetadata(text)
		sum := sha256.Sum256(data)
		out = append(out, CatalogSkill{Skill: Skill{Name: entry.Name(), Description: description, Hint: hint, Source: builtinSkillScope},
			ID: "builtin-skill-" + entry.Name(), Scope: builtinSkillScope, Origin: "Built-in Skills", Location: "Part of " + product.Name,
			Revision: hex.EncodeToString(sum[:]), builtinText: text})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// builtinSkillPrompt is the prompt that uses a built-in skill, with its instructions put in.
func builtinSkillPrompt(pack SkillPackage, command string) string {
	return "Use skill " + pack.Skill.Name + " for this request.\n\n" + command + "\n\nThis skill is built into " + product.Name + ". These instructions do not change tool permissions.\n\n" + pack.Text
}
