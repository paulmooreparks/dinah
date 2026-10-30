package main

import (
	"crypto/rand"
	_ "embed"
	"errors"
	"path/filepath"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/layout"
	"studio/internal/workspaces"
)

//go:embed workspace_sources.go
var workspaceSourcesSource []byte

// sameWorkspaceSource reports whether two sources are one: local folders at
// one path, or sources of another kind with the same fields this build
// does not know, which identify them.
func sameWorkspaceSource(a, b WorkspaceSource) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == "local-folder" {
		return layout.SamePath(a.Path, b.Path)
	}
	return workspaces.SameExtra(a.Extra, b.Extra)
}

func (a *App) workspaceSourceRecord(id string) (WorkspaceRecord, error) {
	if a.startupErr != nil {
		return WorkspaceRecord{}, a.startupErr
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	index := workspaces.Index(a.settings.WorkspaceRecords, id)
	if index < 0 {
		return WorkspaceRecord{}, errors.New("that workspace is not registered")
	}
	return workspaces.Copy(a.settings.WorkspaceRecords[index : index+1])[0], nil
}

func (a *App) attachWorkspaceSource(id string, source WorkspaceSource) (WorkspaceRecord, error) {
	if a.startupErr != nil {
		return WorkspaceRecord{}, a.startupErr
	}
	var record WorkspaceRecord
	err := a.updateSettings(func(s *Settings) error {
		index := workspaces.Index(s.WorkspaceRecords, id)
		if index < 0 {
			return errors.New("that workspace is not registered")
		}
		for _, existing := range s.WorkspaceRecords[index].Sources {
			if sameWorkspaceSource(existing, source) {
				record = workspaces.Copy(s.WorkspaceRecords[index : index+1])[0]
				return errSettingsUnchanged
			}
		}
		s.WorkspaceRecords[index].Sources = append(s.WorkspaceRecords[index].Sources, source)
		if err := ensureWorkspaceMountAliases(s.WorkspaceRecords); err != nil {
			return err
		}
		record = workspaces.Copy(s.WorkspaceRecords[index : index+1])[0]
		return nil
	})
	if errors.Is(err, errSettingsUnchanged) {
		return record, nil
	}
	if err != nil {
		return WorkspaceRecord{}, err
	}
	a.emitSettingsChanged()
	a.emitWorkspaceMountChanged("source-attached", id, source.ID)
	return record, nil
}

func (a *App) AttachLocalWorkspaceSource(workspaceID, path, name string) (WorkspaceRecord, error) {
	if _, err := a.workspaceSourceRecord(workspaceID); err != nil {
		return WorkspaceRecord{}, err
	}
	path = filepath.Clean(path)
	if err := checkWorkspaceFolder(path); err != nil {
		return WorkspaceRecord{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultWorkspaceLabel(filepath.Base(path))
	}
	if !validWorkspaceName(name) {
		return WorkspaceRecord{}, errors.New("use a source name of 1 to 200 characters without control characters")
	}
	return a.attachWorkspaceSource(workspaceID, WorkspaceSource{ID: "src_" + rand.Text(), Kind: "local-folder", Name: name, Path: path})
}

func (a *App) ChooseWorkspaceSource(workspaceID string) (WorkspaceRecord, error) {
	record, err := a.workspaceSourceRecord(workspaceID)
	if err != nil {
		return WorkspaceRecord{}, err
	}
	path, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Attach a Folder", DefaultDirectory: workspaces.ExecutionPath(record)})
	if err != nil || path == "" {
		return record, err
	}
	return a.AttachLocalWorkspaceSource(workspaceID, path, "")
}

func (a *App) DetachWorkspaceSource(workspaceID, sourceID string) (WorkspaceRecord, error) {
	if a.startupErr != nil {
		return WorkspaceRecord{}, a.startupErr
	}
	var record WorkspaceRecord
	err := a.updateSettings(func(s *Settings) error {
		index := workspaces.Index(s.WorkspaceRecords, workspaceID)
		if index < 0 {
			return errors.New("that workspace is not registered")
		}
		current := s.WorkspaceRecords[index]
		out := make([]WorkspaceSource, 0, len(current.Sources))
		for _, source := range current.Sources {
			if source.ID != sourceID {
				out = append(out, source)
			}
		}
		if len(out) == len(current.Sources) {
			return errors.New("that workspace source is not attached")
		}
		s.WorkspaceRecords[index].Sources = out
		// A pin into the source goes with it, since it can no longer open.
		pins := []workspaces.Pin{}
		for _, p := range s.WorkspaceRecords[index].Pins {
			if p.Ref == nil || p.Ref.SourceID != sourceID {
				pins = append(pins, p)
			}
		}
		s.WorkspaceRecords[index].Pins = pins
		delete(s.WorkspaceRecords[index].PlannedFolders, sourceID)
		if len(s.WorkspaceRecords[index].PlannedFolders) == 0 {
			s.WorkspaceRecords[index].PlannedFolders = nil
		}
		record = workspaces.Copy(s.WorkspaceRecords[index : index+1])[0]
		return nil
	})
	if err != nil {
		return WorkspaceRecord{}, err
	}
	a.emitSettingsChanged()
	a.emitWorkspaceMountChanged("source-detached", workspaceID, sourceID)
	return record, nil
}
