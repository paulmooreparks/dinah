package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"studio/internal/terminals"
	"studio/internal/workspaces"
)

type TerminalShell = terminals.Shell
type TerminalShells = terminals.Shells
type TerminalProfile = terminals.Profile
type TerminalStarted = terminals.Started

const dinahShell = terminals.BuiltinShell

func (a *App) terminalService() *terminals.Service {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.terminals == nil {
		a.terminals = terminals.New(product.Name, a.emit, func() {
			logf("core", "warning", "a Terminal's shell could not be started outside %[1]s's lifetime job, so programs started from the Terminal will end when %[1]s exits", product.Name)
		})
	}
	return a.terminals
}

func (a *App) availableShells() []TerminalShell {
	return terminals.Available(product.Name, a.TerminalProfiles())
}

func (a *App) TerminalProfiles() []TerminalProfile {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]TerminalProfile{}, a.settings.TerminalProfiles...)
}
func (a *App) AddTerminalProfile(name, path string, args []string) error {
	name, path = strings.TrimSpace(name), strings.TrimSpace(path)
	if name == "" || len(name) > 100 || len(path) > 4096 || len(args) > 64 {
		return errors.New("invalid shell profile")
	}
	if _, err := exec.LookPath(path); err != nil {
		return fmt.Errorf("shell executable: %w", err)
	}
	return a.updateSettings(func(st *Settings) error {
		if len(st.TerminalProfiles) >= 32 {
			return errors.New("shell profile limit reached")
		}
		st.TerminalProfiles = append(st.TerminalProfiles, TerminalProfile{ID: "custom-" + rand.Text(), Name: name, Path: path, Args: append([]string{}, args...)})
		return nil
	})
}
func (a *App) RemoveTerminalProfile(id string) error {
	return a.updateSettings(func(st *Settings) error {
		for i, profile := range st.TerminalProfiles {
			if profile.ID == id {
				st.TerminalProfiles = append(st.TerminalProfiles[:i], st.TerminalProfiles[i+1:]...)
				if st.TerminalShell == id {
					st.TerminalShell = ""
				}
				return nil
			}
		}
		return errors.New("shell profile not found")
	})
}

func (a *App) TerminalShells() TerminalShells {
	shells := a.availableShells()
	a.mu.Lock()
	choice := a.settings.TerminalShell
	a.mu.Unlock()
	return TerminalShells{Shells: shells, Default: terminals.DefaultShell(shells, choice)}
}

// SetTerminalShell chooses the shell a new Terminal runs.
func (a *App) SetTerminalShell(id string) error {
	for _, s := range a.availableShells() {
		if s.ID == id {
			return a.updateSettings(func(st *Settings) error { st.TerminalShell = id; return nil })
		}
	}
	return fmt.Errorf("no shell called %q is on this computer", id)
}

// terminalFolder is the local folder a Terminal starts in: the folder ref is
// in, when its source is a local folder, then the workspace's own folder,
// then the person's home folder.
func (a *App) terminalFolder(ref *ResourceRef) string {
	a.mu.Lock()
	records := workspaces.Copy(a.settings.WorkspaceRecords)
	current := a.settings.WorkspaceID
	a.mu.Unlock()
	workspaceID := current
	if ref != nil && ref.WorkspaceID != "" {
		workspaceID = ref.WorkspaceID
	}
	for _, record := range records {
		if record.ID != workspaceID {
			continue
		}
		if ref != nil {
			for _, source := range record.Sources {
				if source.ID == ref.SourceID && source.Kind == "local-folder" && source.Path != "" {
					if dir := folderWithin(source.Path, ref.Path); dir != "" {
						return dir
					}
				}
			}
		}
		if dir := workspaces.ExecutionPath(record); isFolder(dir) {
			return dir
		}
	}
	home, _ := os.UserHomeDir()
	return home
}

// folderWithin is the folder at rel inside root, or the folder holding it
// when rel is a file, or "" when rel leaves root or is not there.
func folderWithin(root, rel string) string {
	p := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(rel, "/")))
	if r, err := filepath.Rel(root, p); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return ""
	}
	if isFolder(p) {
		return p
	}
	if isFolder(filepath.Dir(p)) {
		return filepath.Dir(p)
	}
	return ""
}

func isFolder(p string) bool {
	info, err := os.Stat(p)
	return p != "" && err == nil && info.IsDir()
}

func (a *App) TerminalStart(shell string, ref *ResourceRef, cols, rows int) (TerminalStarted, error) {
	return a.terminalService().StartShell(shell, a.TerminalProfiles(), a.terminalFolder(ref), cols, rows)
}
func (a *App) TerminalWrite(id, data string) error { return a.terminalService().Write(id, data) }
func (a *App) TerminalResize(id string, cols, rows int) error {
	return a.terminalService().Resize(id, cols, rows)
}
func (a *App) TerminalClose(id string) error { return a.terminalService().Close(id) }
