package fileaccess

import (
	"os"
	"runtime"
	"strings"

	"studio/internal/layout"
	"studio/internal/workspaces"
)

func CurrentDefaults() ([]Rule, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			if runtime.GOOS == "windows" {
				key = strings.ToUpper(key)
			}
			env[key] = value
		}
	}
	return DefaultRules(runtime.GOOS, home, os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"), env), nil
}

func addRoot(roots []string, path string) []string {
	if path == "" {
		return roots
	}
	for _, root := range roots {
		if layout.SamePath(root, path) {
			return roots
		}
	}
	return append(roots, path)
}

// RegisteredWorkspaceRoots includes folder sources regardless of path spelling.
func RegisteredWorkspaceRoots(records []workspaces.Record) []string {
	var roots []string
	for _, record := range records {
		roots = addRoot(roots, record.ExecutionPath)
		for _, source := range record.Sources {
			if workspaces.IsLocalSource(source) {
				roots = addRoot(roots, source.Path)
			}
		}
	}
	return roots
}

func WorkspaceScope(l layout.Layout, records []workspaces.Record, cwd string) Scope {
	scope := Scope{Workspace: cwd, Roots: []string{cwd}, Layout: l}
	for _, record := range records {
		if layout.SamePath(record.ExecutionPath, cwd) {
			scope.Workspace = record.ID
			for _, source := range record.Sources {
				if workspaces.IsLocalSource(source) {
					scope.Roots = addRoot(scope.Roots, source.Path)
				}
			}
			if layout.ManagedID(record.ID) && layout.SamePath(record.ExecutionPath, l.ManagedRoot(record.ID)) {
				scope.ManagedRoots = addRoot(scope.ManagedRoots, l.ManagedRoot(record.ID))
			}
		} else {
			scope.OtherRoots = addRoot(scope.OtherRoots, record.ExecutionPath)
			for _, source := range record.Sources {
				if workspaces.IsLocalSource(source) {
					scope.OtherRoots = addRoot(scope.OtherRoots, source.Path)
				}
			}
		}
	}
	return scope
}
