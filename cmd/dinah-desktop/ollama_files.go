package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"studio/internal/fileaccess"
	"studio/internal/layout"
	"studio/internal/ollamaagent"
)

// Where Ollama's file tools may go (docs/plans/ollama-file-limits.md). The
// person chooses under Settings > Agents > Ollama > File Access:
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
	return fileaccess.WorkspaceScope(stateLayout(), s.WorkspaceRecords, cwd).Roots
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
	c.SetHostFileReads()
	c.SetHostFileOperation(func(ctx context.Context, session, path, source string, work func(context.Context) (string, bool)) (string, bool) {
		scope, err := a.agentReadScope(c, session)
		if err != nil {
			return err.Error(), false
		}
		scope.Source = source
		service, err := a.fileAccessService()
		if err != nil {
			return err.Error(), false
		}
		text, ok := service.RunOperation(ctx, path, scope, work)
		a.fileAccessUpdated("file-access:read", nil)
		return text, ok
	})
	c.SetHostDirectoryReads(func(ctx context.Context, session, path, source string) ([]fs.DirEntry, error) {
		return a.readAgentDirectory(ctx, c, session, path, source)
	})
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
