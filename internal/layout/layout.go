// Package layout says where Dinah keeps things on disk, so that the
// desktop app and the command line find the same records. Everything it
// says is relative to the state folder it is given.
package layout

import (
	"path/filepath"
	"regexp"
	"strings"

	"studio/internal/profile"
	"studio/internal/records"
)

// Layout is the places under one state folder.
type Layout struct {
	State string
}

var managedID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// ManagedID reports whether an ID can be a managed workspace's.
func ManagedID(id string) bool { return managedID.MatchString(id) }

// ManagedRoot is the folder a managed workspace's agents work in.
func (l Layout) ManagedRoot(id string) string {
	return filepath.Join(l.State, "workspaces", id, "files")
}

// Managed reports whether a path is a managed workspace's folder, one Dinah
// keeps in its state folder rather than one the person chose.
func (l Layout) Managed(path string) bool {
	rel, err := filepath.Rel(filepath.Join(l.State, "workspaces"), path)
	if err != nil || !filepath.IsLocal(rel) {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	return len(parts) == 2 && ManagedID(parts[0]) && parts[1] == "files"
}

// RunDir is the folder of a workspace's runs: runs in the profile's
// workspace folder (.dinah/runs for Dinah), or beside a managed workspace's folder, where its agents cannot
// write.
func (l Layout) RunDir(workspace string) string {
	if l.Managed(workspace) {
		return filepath.Join(filepath.Dir(workspace), "runs")
	}
	return filepath.Join(workspace, profile.Current().WorkspaceFolder, "runs")
}

// Runs is the store of a workspace's runs.
func (l Layout) Runs(workspace string) records.Store { return records.Store{Dir: l.RunDir(workspace)} }
