package agents

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/layout"
	"studio/internal/profile"
)

// ProtectedPath says why an agent may not write a path, or "" when it may.
// The folders that hold the approval rules are closed to it: Dinah's
// state folder, apart from the managed workspaces' folders, and Devin's own
// settings, whose config.json holds the person's Devin approvals. Anywhere
// else, the agent's approvals decide.
func ProtectedPath(l layout.Layout, p string, managedRoots ...string) string {
	if layout.Inside(l.State, p) {
		for _, root := range managedRoots {
			if l.Managed(root) && layout.Inside(root, p) {
				return ""
			}
		}
		return "it is among " + profile.Current().Name + "'s settings, which hold the approval rules"
	}
	if base, err := os.UserConfigDir(); err == nil && layout.Inside(filepath.Join(base, "devin"), p) {
		return "it is among Devin's settings, which hold the approval rules"
	}
	if home, err := os.UserHomeDir(); err == nil && layout.Inside(filepath.Join(home, ".config", "devin"), p) {
		return "it is among Devin's settings, which hold the approval rules"
	}
	return ""
}

// WriteTextFile writes a file for an agent, refusing a protected path. A
// write into a managed workspace goes through an open root, so that a link
// below its folder cannot redirect the write into Dinah's settings
// or another workspace.
func WriteTextFile(l layout.Layout, path, content string, managedRoots ...string) error {
	if reason := ProtectedPath(l, path, managedRoots...); reason != "" {
		return errors.New(profile.Current().Name + " does not let the agent write " + path + ": " + reason)
	}
	for _, managed := range managedRoots {
		if !l.Managed(managed) || !layout.Inside(managed, path) {
			continue
		}
		root, err := os.OpenRoot(managed)
		if err != nil {
			return err
		}
		defer root.Close()
		resolved, err := filepath.EvalSymlinks(managed)
		if err != nil || !layout.SamePath(resolved, managed) {
			return errors.New("the managed workspace execution folder must not redirect through a symbolic link")
		}
		opened, err := root.Stat(".")
		if err != nil {
			return err
		}
		current, err := os.Lstat(managed)
		if err != nil || !current.IsDir() || !os.SameFile(opened, current) {
			return errors.New("the managed workspace execution folder changed while opening it")
		}
		rel, err := filepath.Rel(managed, path)
		if err != nil || !filepath.IsLocal(rel) {
			return errors.New("the file is outside the managed workspace")
		}
		if err := root.MkdirAll(filepath.Dir(rel), 0o700); err != nil {
			return err
		}
		return root.WriteFile(rel, []byte(content), 0o600)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// SliceLines returns limit lines starting at the 1-based line, as
// fs/read_text_file asks; with neither, the whole text.
func SliceLines(s string, line, limit *int) string {
	if line == nil && limit == nil {
		return s
	}
	lines := strings.SplitAfter(s, "\n")
	start := 0
	if line != nil && *line > 1 {
		start = *line - 1
	}
	if start > len(lines) {
		return ""
	}
	end := len(lines)
	if limit != nil && start+*limit < end {
		end = start + *limit
	}
	return strings.Join(lines[start:end], "")
}
