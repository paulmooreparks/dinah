package agents

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/layout"
	"studio/internal/profile"
)

// ProtectedPath says why host-mediated agent file access is denied, or "" when allowed.
// The folders that hold the approval rules are closed to it: Dinah's
// state folder, apart from the managed workspaces' folders, and Devin's own
// settings, whose config.json holds the person's Devin approvals. Anywhere
// else, the agent's approvals decide.
type ProtectedRoot struct{ Path, Reason string }

// ProtectedRoots is shared by lexical and opened-identity checks.
func ProtectedRoots(l layout.Layout) []ProtectedRoot {
	roots := []ProtectedRoot{{l.State, "it is among " + profile.Current().Name + "'s settings, which hold the approval rules"}}
	if base, err := os.UserConfigDir(); err == nil {
		roots = append(roots, ProtectedRoot{filepath.Join(base, "devin"), "it is among the provider's approval settings"})
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, ProtectedRoot{filepath.Join(home, ".config", "devin"), "it is among the provider's approval settings"})
	}
	return roots
}

func ProtectedPath(l layout.Layout, p string, managedRoots ...string) string {
	var err error
	p, err = NormalizeFilePath(p)
	if err != nil {
		return err.Error()
	}
	chain, err := PathIdentities(p)
	if err != nil {
		return "the file identity could not be checked"
	}
	if err = protectedIdentity(l, chain, managedRoots...); err != nil {
		return err.Error()
	}
	for i, root := range ProtectedRoots(l) {
		if root.Path == "" || !layout.Inside(root.Path, p) {
			continue
		}
		if i == 0 {
			for _, managed := range managedRoots {
				if l.Managed(managed) && layout.Inside(managed, p) {
					return ""
				}
			}
		}
		return root.Reason
	}
	return ""
}

// WriteTextFile writes through a checked parent handle. Both the requested
// spelling and opened identities must pass the protected-root policy before
// creation or truncation. Managed workspace writes also reject redirects.
func WriteTextFile(l layout.Layout, path, content string, managedRoots ...string) error {
	if reason := ProtectedPath(l, path, managedRoots...); reason != "" {
		return errors.New(profile.Current().Name + " does not let the agent write " + path + ": " + reason)
	}
	var err error
	path, err = NormalizeFilePath(path)
	if err != nil {
		return err
	}
	resolved, err := ResolveFilePath(path)
	if err != nil {
		return err
	}
	for _, managed := range managedRoots {
		if l.Managed(managed) && layout.Inside(managed, path) && !layout.SamePath(path, resolved) {
			return errors.New("a managed workspace write must not redirect through a symbolic link")
		}
	}
	return writeIdentityFile(l, resolved, content, managedRoots)
}

// ValidateLineRange rejects invalid optional 1-based line ranges.
func ValidateLineRange(line, limit *int) error {
	if line != nil && *line < 1 {
		return errors.New("line must be positive")
	}
	if limit != nil && *limit < 1 {
		return errors.New("limit must be positive")
	}
	return nil
}

// SliceLines returns limit lines starting at the 1-based line, as
// fs/read_text_file asks; with neither, the whole text.
func SliceLines(s string, line, limit *int) (string, error) {
	if err := ValidateLineRange(line, limit); err != nil {
		return "", err
	}
	if line == nil && limit == nil {
		return s, nil
	}
	lines := strings.SplitAfter(s, "\n")
	start := 0
	if line != nil && *line > 1 {
		start = *line - 1
	}
	if start >= len(lines) {
		return "", errors.New("line is past the end of the file")
	}
	end := len(lines)
	if limit != nil && *limit < end-start {
		end = start + *limit
	}
	return strings.Join(lines[start:end], ""), nil
}
