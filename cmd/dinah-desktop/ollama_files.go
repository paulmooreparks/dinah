package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"studio/internal/layout"
	"studio/internal/ollamaagent"
)

// Where Ollama's file tools may go (docs/plans/ollama-file-limits.md). The
// person chooses under Settings › Agents › Ollama › File Access:
//
//   - workspace: the chat's own folder and the local folders among the sources
//     of the workspace it belongs to,
//   - folders: those and a list of other folders,
//   - anywhere: no limit.
//
// A settings file without the choice means anywhere, which is what Ollama
// could do before the choice existed, so no existing install loses a path it
// uses. An install that starts without a settings file gets workspace.
const (
	ollamaScopeWorkspace = "workspace"
	ollamaScopeFolders   = "folders"
	ollamaScopeAnywhere  = "anywhere"
)

// ollamaFileScope is the effective choice, whatever the saved settings hold.
func ollamaFileScope(s Settings) string {
	switch s.OllamaFileScope {
	case ollamaScopeWorkspace, ollamaScopeFolders:
		return s.OllamaFileScope
	}
	return ollamaScopeAnywhere
}

// ollamaFileFolders are the other folders the person listed, copied.
func ollamaFileFolders(s Settings) []string { return slices.Clone(s.OllamaFileFolders) }

// workspaceFolders are the folders of the workspace a chat belongs to: its
// own folder first, then every local source of the workspace whose execution
// folder it is. A chat in no registered workspace has its own folder alone.
func workspaceFolders(s Settings, cwd string) []string {
	out := []string{filepath.Clean(cwd)}
	for _, w := range s.WorkspaceRecords {
		if !layout.SamePath(w.ExecutionPath, cwd) {
			continue
		}
		for _, src := range w.Sources {
			if src.Kind == "local" && src.Path != "" && !slices.ContainsFunc(out, func(o string) bool { return layout.SamePath(o, src.Path) }) {
				out = append(out, filepath.Clean(src.Path))
			}
		}
	}
	return out
}

// ollamaFileFolderList is the folders a chat's file tools may reach under the
// settings, and whether they are limited to them.
func ollamaFileFolderList(s Settings, cwd string) ([]string, bool) {
	switch ollamaFileScope(s) {
	case ollamaScopeWorkspace:
		return workspaceFolders(s, cwd), true
	case ollamaScopeFolders:
		return append(workspaceFolders(s, cwd), s.OllamaFileFolders...), true
	}
	return nil, false
}

// ollamaFileScopeFor is what the Ollama connection asks at every tool call,
// so that a change in Settings applies at once.
func (a *App) ollamaFileScopeFor(_, cwd string) ([]string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return ollamaFileFolderList(a.settings, cwd)
}

// useOllamaFileScope gives a connection to Ollama the function that says where
// its file tools may go.
func (a *App) useOllamaFileScope(c *ollamaagent.Conn) {
	c.SetFileScope(func(session, cwd string) ([]string, bool) { return a.ollamaFileScopeFor(session, cwd) })
}

// SetOllamaFileScope chooses where Ollama's file tools may go: "workspace",
// "folders" with the other folders to allow, or "anywhere". A folder must be
// an absolute path; it need not exist yet.
func (a *App) SetOllamaFileScope(scope string, folders []string) error {
	switch scope {
	case ollamaScopeWorkspace, ollamaScopeFolders, ollamaScopeAnywhere:
	default:
		return errors.New("choose Workspace Folders, Workspace Folders and Others, or Anywhere")
	}
	var clean []string
	for _, f := range folders {
		if f == "" {
			continue
		}
		if !filepath.IsAbs(f) {
			return fmt.Errorf("%q is not a full path; give each folder as a full path", f)
		}
		f = filepath.Clean(f)
		if !slices.ContainsFunc(clean, func(o string) bool { return layout.SamePath(o, f) }) {
			clean = append(clean, f)
		}
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.OllamaFileScope, s.OllamaFileFolders = scope, clean
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
