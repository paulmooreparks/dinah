package main

import (
	"crypto/rand"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"studio/internal/layout"
	"studio/internal/workspaces"
)

//go:embed workspace_identity.go
var workspaceIdentitySource []byte

// The registry's types and lookups are package workspaces', which the
// command line shares; the app keeps the registry in its settings and
// changes it.
type (
	WorkspaceSource = workspaces.Source
	WorkspaceRecord = workspaces.Record
)

func copyWorkspaceSettings(settings Settings) Settings {
	settings.WorkspaceRecords = workspaces.Copy(settings.WorkspaceRecords)
	settings.Workspaces = append([]string{}, settings.Workspaces...)
	settings.Recent = append([]string{}, settings.Recent...)
	return settings
}

func validWorkspaceName(name string) bool {
	return name != "" && strings.TrimSpace(name) == name && utf8.ValidString(name) && utf8.RuneCountInString(name) <= 200 && strings.IndexFunc(name, unicode.IsControl) < 0
}

func defaultWorkspaceLabel(name string) string {
	if filepath.IsAbs(name) || name == "/" || name == "\\" {
		name = "Workspace"
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(name, "\uFFFD"))
	name = strings.TrimSpace(name)
	if runes := []rune(name); len(runes) > 200 {
		name = strings.TrimSpace(string(runes[:200]))
	}
	if name == "" {
		name = "Workspace"
	}
	return name
}

func newWorkspaceRecord(path string) WorkspaceRecord {
	name := defaultWorkspaceLabel(filepath.Base(filepath.Clean(path)))
	id := "src_" + rand.Text()
	return WorkspaceRecord{ID: "ws_" + rand.Text(), Name: name, ExecutionPath: path, PrimarySourceID: id, Sources: []WorkspaceSource{{ID: id, Kind: "local-folder", Name: name, Path: path}}}
}

// Public path segments follow current labels. Stable IDs retain resource identity.
func ensureWorkspaceMountAliases(records []WorkspaceRecord) error {
	used := map[string]bool{}
	ordered := make([]int, len(records))
	for i := range records {
		ordered[i] = i
	}
	sort.Slice(ordered, func(i, j int) bool { return records[ordered[i]].ID < records[ordered[j]].ID })
	for _, i := range ordered {
		record := &records[i]
		record.MountAlias = workspaces.MountAlias(record.Name, "Workspace", used)
		sources := map[string]bool{}
		order := make([]int, len(record.Sources))
		for j := range record.Sources {
			order[j] = j
		}
		sort.Slice(order, func(i, j int) bool { return record.Sources[order[i]].ID < record.Sources[order[j]].ID })
		for _, j := range order {
			source := &record.Sources[j]
			source.MountAlias = workspaces.MountAlias(source.Name, "Source", sources)
		}
	}
	return nil
}

// migrateWorkspaceSettings preserves legacy paths, including unavailable roots.
// Filesystem existence does not determine whether an identity is retained.
func migrateWorkspaceSettings(settings Settings) (Settings, bool, error) {
	updated := copyWorkspaceSettings(settings)
	ids := map[string]bool{}
	sourceIDs := map[string]bool{}
	var paths []string
	for i := range updated.WorkspaceRecords {
		record := &updated.WorkspaceRecords[i]
		if record.ID == "" || ids[record.ID] || !validWorkspaceName(record.Name) {
			return settings, false, errors.New("the workspace registry contains an invalid or duplicate identity")
		}
		ids[record.ID] = true
		primary, ok := workspaces.PrimarySource(*record)
		if record.ExecutionPath == "" && !ok {
			return settings, false, errors.New("the workspace registry has no execution storage")
		}
		if record.ExecutionPath == "" {
			record.ExecutionPath = primary.Path
			if record.PrimarySourceID == "" {
				record.PrimarySourceID = primary.ID
			}
		}
		if !filepath.IsAbs(record.ExecutionPath) {
			return settings, false, errors.New("the workspace execution storage must be absolute")
		}
		for j := range record.Sources {
			source := &record.Sources[j]
			if source.ID == "" || sourceIDs[source.ID] {
				return settings, false, errors.New("the workspace registry contains an invalid source")
			}
			sourceIDs[source.ID] = true
			switch {
			case source.Kind == "local-folder":
				if !filepath.IsAbs(source.Path) {
					return settings, false, errors.New("the workspace registry contains an invalid local source")
				}
				if source.Name == "" {
					source.Name = defaultWorkspaceLabel(filepath.Base(filepath.Clean(source.Path)))
				}
			case workspaces.ValidKind(source.Kind) && source.Path == "":
				// A source of another kind, which an import kept for a build
				// that knows it (workspace_export.go), stays as it is.
				if source.Name == "" {
					source.Name = source.Kind
				}
			default:
				return settings, false, errors.New("the workspace registry contains an unsupported source kind")
			}
			if settings.WorkspaceRecords[i].Sources[j].Name == "" && source.ID == record.PrimarySourceID {
				source.Name = record.Name
			}
			if !validWorkspaceName(source.Name) {
				return settings, false, errors.New("the workspace registry contains an invalid source name")
			}
			for _, previous := range record.Sources[:j] {
				if sameWorkspaceSource(previous, *source) {
					return settings, false, errors.New("the workspace registry contains duplicate sources in one workspace")
				}
			}
		}
		// Recorded folders belong to a local folder of the workspace and
		// stay inside it (workspace_export.go); any others are dropped.
		for id, folders := range record.PlannedFolders {
			local := false
			for _, source := range record.Sources {
				local = local || (source.ID == id && source.Kind == "local-folder")
			}
			clean := []string{}
			for _, folder := range folders {
				if workspaces.CleanFolder(folder) {
					clean = append(clean, folder)
				}
			}
			if !local || len(clean) == 0 {
				delete(record.PlannedFolders, id)
			} else if len(clean) != len(folders) {
				record.PlannedFolders[id] = clean
			}
		}
		if len(record.PlannedFolders) == 0 {
			record.PlannedFolders = nil
		}
		for _, path := range paths {
			if layout.SamePath(path, record.ExecutionPath) {
				return settings, false, errors.New("the workspace registry assigns a primary folder to more than one identity")
			}
		}
		paths = append(paths, record.ExecutionPath)
	}
	legacy := append([]string{settings.Workspace}, settings.Workspaces...)
	legacy = append(legacy, settings.Recent...)
	for _, path := range legacy {
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			return settings, false, fmt.Errorf("the saved workspace path %q is not absolute", path)
		}
		if workspaces.IndexAt(updated.WorkspaceRecords, path) < 0 {
			updated.WorkspaceRecords = append(updated.WorkspaceRecords, newWorkspaceRecord(path))
		}
	}
	if settings.WorkspaceID != "" {
		index := workspaces.Index(updated.WorkspaceRecords, settings.WorkspaceID)
		if index < 0 {
			return settings, false, errors.New("the selected workspace identity is missing from the registry")
		}
		path := workspaces.ExecutionPath(updated.WorkspaceRecords[index])
		if settings.Workspace != "" && !layout.SamePath(settings.Workspace, path) {
			return settings, false, errors.New("the selected workspace identity conflicts with its saved folder")
		}
		if settings.Workspace == "" {
			updated.Workspace = path
		}
	} else if settings.Workspace != "" {
		updated.WorkspaceID = updated.WorkspaceRecords[workspaces.IndexAt(updated.WorkspaceRecords, settings.Workspace)].ID
	}
	if err := ensureWorkspaceMountAliases(updated.WorkspaceRecords); err != nil {
		return settings, false, err
	}
	recordsChanged := (len(settings.WorkspaceRecords) > 0 || len(updated.WorkspaceRecords) > 0) && !reflect.DeepEqual(settings.WorkspaceRecords, updated.WorkspaceRecords)
	changed := recordsChanged || settings.WorkspaceID != updated.WorkspaceID || settings.Workspace != updated.Workspace
	return updated, changed, nil
}

func selectWorkspaceSettings(settings Settings, record WorkspaceRecord) Settings {
	updated := copyWorkspaceSettings(settings)
	path := workspaces.ExecutionPath(record)
	updated.WorkspaceID, updated.Workspace = record.ID, path
	updated.addRecent(path)
	for _, existing := range updated.Workspaces {
		if layout.SamePath(existing, path) {
			return updated
		}
	}
	updated.Workspaces = append(updated.Workspaces, path)
	return updated
}

func checkWorkspaceFolder(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("choose an absolute workspace path")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a folder that can be opened", path)
	}
	return nil
}

// SetWorkspace resolves a folder to its stable identity before selecting it.
func (a *App) SetWorkspace(path string) (State, error) {
	if a.startupErr != nil {
		return a.State(), a.startupErr
	}
	path = filepath.Clean(path)
	if err := checkWorkspaceFolder(path); err != nil {
		return a.State(), err
	}
	createdID := ""
	err := a.updateSettings(func(s *Settings) error {
		updated, _, err := migrateWorkspaceSettings(*s)
		if err != nil {
			return err
		}
		index := workspaces.IndexAt(updated.WorkspaceRecords, path)
		if index < 0 {
			updated.WorkspaceRecords = append(updated.WorkspaceRecords, newWorkspaceRecord(path))
			index = len(updated.WorkspaceRecords) - 1
			createdID = updated.WorkspaceRecords[index].ID
		}
		if err = ensureWorkspaceMountAliases(updated.WorkspaceRecords); err != nil {
			return err
		}
		*s = selectWorkspaceSettings(updated, updated.WorkspaceRecords[index])
		return nil
	})
	if err == nil {
		a.emitSettingsChanged()
		if createdID != "" {
			a.emitWorkspaceMountChanged("workspace-created", createdID, "")
		}
	}
	return a.State(), err
}

func (a *App) SelectWorkspace(id string) (State, error) {
	if a.startupErr != nil {
		return a.State(), a.startupErr
	}
	err := a.updateSettings(func(s *Settings) error {
		index := workspaces.Index(s.WorkspaceRecords, id)
		if index < 0 {
			return errors.New("that workspace is not registered")
		}
		record := s.WorkspaceRecords[index]
		if err := checkWorkspaceFolder(workspaces.ExecutionPath(record)); err != nil {
			return err
		}
		*s = selectWorkspaceSettings(*s, record)
		return nil
	})
	if err == nil {
		a.emitSettingsChanged()
	}
	return a.State(), err
}

// CreateWorkspace registers an empty collection. Its execution storage is private
// application data and never becomes an attached source.
func (a *App) CreateWorkspace() (State, error) {
	if a.startupErr != nil {
		return a.State(), a.startupErr
	}
	var record WorkspaceRecord
	err := a.updateSettings(func(s *Settings) error {
		name := "Workspace"
		for n := 2; ; n++ {
			found := false
			for _, record := range s.WorkspaceRecords {
				if record.Name == name {
					found = true
					break
				}
			}
			if !found {
				break
			}
			name = fmt.Sprintf("Workspace %d", n)
		}
		record = WorkspaceRecord{ID: "ws_" + rand.Text(), Name: name, Sources: []WorkspaceSource{}}
		record.ExecutionPath = filepath.Join(configDir(), "workspaces", record.ID, "files")
		s.WorkspaceRecords = append(s.WorkspaceRecords, record)
		if err := ensureWorkspaceMountAliases(s.WorkspaceRecords); err != nil {
			return err
		}
		for _, path := range []string{filepath.Join(configDir(), "workspaces"), filepath.Dir(record.ExecutionPath), record.ExecutionPath} {
			if err := stateDirectory(path); err != nil {
				return err
			}
		}
		*s = selectWorkspaceSettings(*s, s.WorkspaceRecords[len(s.WorkspaceRecords)-1])
		return nil
	})
	if err == nil {
		a.emitSettingsChanged()
	}
	if err == nil {
		a.emitWorkspaceMountChanged("workspace-created", record.ID, "")
	}
	return a.State(), err
}

func (a *App) RenameWorkspace(id, name string) (WorkspaceRecord, error) {
	if a.startupErr != nil {
		return WorkspaceRecord{}, a.startupErr
	}
	name = strings.TrimSpace(name)
	if !validWorkspaceName(name) {
		return WorkspaceRecord{}, errors.New("use a workspace name of 1 to 200 characters without control characters")
	}
	var record WorkspaceRecord
	err := a.updateSettings(func(s *Settings) error {
		index := workspaces.Index(s.WorkspaceRecords, id)
		if index < 0 {
			return errors.New("that workspace is not registered")
		}
		s.WorkspaceRecords[index].Name = name
		if err := ensureWorkspaceMountAliases(s.WorkspaceRecords); err != nil {
			return err
		}
		record = workspaces.Copy(s.WorkspaceRecords[index : index+1])[0]
		return nil
	})
	if err != nil {
		return WorkspaceRecord{}, err
	}
	a.emitSettingsChanged()
	a.emitWorkspaceMountChanged("workspace-renamed", id, "")
	return record, nil
}

// RemoveWorkspace takes a workspace out of Dinah: its record, with its
// sources and pins, and its place in the recent and open lists. Nothing on
// disk is touched; its main folder can be made a
// workspace again with New Workspace…. A workspace whose chat is working is
// refused. When it was the workspace in front, the most recent other one
// takes its place, or none when it was the last.
func (a *App) RemoveWorkspace(id string) (State, error) {
	if a.startupErr != nil {
		return a.State(), a.startupErr
	}
	a.mu.Lock()
	index := workspaces.Index(a.settings.WorkspaceRecords, id)
	if index < 0 {
		a.mu.Unlock()
		return a.State(), errors.New("that workspace is not registered")
	}
	path := workspaces.ExecutionPath(a.settings.WorkspaceRecords[index])
	sessions := make([]*Session, 0, a.runtime.ConversationCountLocked())
	for _, s := range a.runtime.ConversationsLocked() {
		sessions = append(sessions, s)
	}
	a.mu.Unlock()
	for _, s := range sessions {
		busy := s.Working() && path != "" && layout.SamePath(s.Cwd(), path)

		if busy {
			return a.State(), errors.New("a chat in this workspace is working; stop it before removing the workspace")
		}
	}

	err := a.updateSettings(func(updated *Settings) error {
		index = workspaces.Index(updated.WorkspaceRecords, id)
		if index < 0 {
			return errors.New("that workspace is not registered")
		}
		updated.WorkspaceRecords = append(updated.WorkspaceRecords[:index:index], updated.WorkspaceRecords[index+1:]...)
		keep := func(list []string) []string {
			out := []string{}
			for _, p := range list {
				if path == "" || !layout.SamePath(p, path) {
					out = append(out, p)
				}
			}
			return out
		}
		updated.Recent = keep(updated.Recent)
		updated.Workspaces = keep(updated.Workspaces)
		if updated.WorkspaceID == id || (path != "" && layout.SamePath(updated.Workspace, path)) {
			updated.WorkspaceID, updated.Workspace = "", ""
			next := -1
			for _, p := range updated.Recent {
				if i := workspaces.IndexAt(updated.WorkspaceRecords, p); i >= 0 {
					next = i
					break
				}
			}
			if next < 0 && len(updated.WorkspaceRecords) > 0 {
				next = 0
			}
			if next >= 0 {
				*updated = selectWorkspaceSettings(*updated, updated.WorkspaceRecords[next])
			}
		}
		return nil
	})
	if err != nil {
		return a.State(), err
	}
	a.emitSettingsChanged()
	a.emitWorkspaceMountChanged("workspace-removed", id, "")
	return a.State(), nil
}

func (a *App) RenameWorkspaceSource(workspaceID, sourceID, name string) (WorkspaceRecord, error) {
	if a.startupErr != nil {
		return WorkspaceRecord{}, a.startupErr
	}
	name = strings.TrimSpace(name)
	if !validWorkspaceName(name) {
		return WorkspaceRecord{}, errors.New("use a source name of 1 to 200 characters without control characters")
	}
	var record WorkspaceRecord
	err := a.updateSettings(func(s *Settings) error {
		index := workspaces.Index(s.WorkspaceRecords, workspaceID)
		if index < 0 {
			return errors.New("that workspace is not registered")
		}
		found := false
		for i := range s.WorkspaceRecords[index].Sources {
			if s.WorkspaceRecords[index].Sources[i].ID == sourceID {
				s.WorkspaceRecords[index].Sources[i].Name = name
				found = true
				break
			}
		}
		if !found {
			return errors.New("that workspace source is not attached")
		}
		if err := ensureWorkspaceMountAliases(s.WorkspaceRecords); err != nil {
			return err
		}
		record = workspaces.Copy(s.WorkspaceRecords[index : index+1])[0]
		return nil
	})
	if err != nil {
		return WorkspaceRecord{}, err
	}
	a.emitSettingsChanged()
	a.emitWorkspaceMountChanged("source-renamed", workspaceID, sourceID)
	return record, nil
}
