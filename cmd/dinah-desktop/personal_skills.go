package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"studio/internal/workspaces"
)

// personalSkillsDir is the person's Devin skills folder, which Devin reads in
// every workspace and which a personal install of a package writes its skills
// to. Under a test's state folder it is a folder there, so a test never reads
// or writes the person's own.
func personalSkillsDir() string {
	dir, err := workspaces.PersonalSkillsFolder(configDir(), configOverride != "")
	if err != nil {
		return ""
	}
	return dir
}

// personalSkills are the skills in the person's Devin skills folder, each
// with the description from its SKILL.md front matter.
func personalSkills() []Skill {
	dir := personalSkillsDir()
	out := []Skill{}
	if dir == "" {
		return out
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		desc, hint, _ := skillMetadata(string(b))
		out = append(out, Skill{Name: e.Name(), Description: desc, Hint: hint, Source: "personal"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// personalSkillText is the instructions of a skill in the person's Devin
// skills folder, for a provider given them in the prompt.
func personalSkillText(name string) (string, bool) {
	dir := personalSkillsDir()
	if dir == "" || name == "" || strings.ContainsAny(name, `/\.`) {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(dir, name, "SKILL.md"))
	if err != nil {
		return "", false
	}
	return string(b), true
}
