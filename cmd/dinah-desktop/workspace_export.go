package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/layout"
	"studio/internal/statefile"
	"studio/internal/workspaces"
)

// Exporting and importing a workspace (docs/design/workspace-export.md):
// Export Workspace… writes the workspace's structure to a file, and Import
// Workspace… makes the same workspace from it on another computer, or adds
// what is missing to the workspace already on its main folder. A local
// folder that is not on the computer is imported all the same and waits
// for the folder; the folder is made only when the person asks, with
// Create the Folders…. A source or pin of a kind this build does not know
// is kept as it is (workspaces.RegisterSourceKind).

// userHome is the person's home folder, which a test replaces.
var userHome = os.UserHomeDir

// WorkspaceImportPlan is what an import would do, for the import window.
type WorkspaceImportPlan struct {
	File    string                  `json:"file"`
	Name    string                  `json:"name"`
	Sources []WorkspaceImportSource `json:"sources"`
	Pins    int                     `json:"pins"`
	// UncheckedPins are the pins among Pins of a kind this build does not
	// know, which are kept as they are.
	UncheckedPins int `json:"uncheckedPins"`
	// ExistingID and ExistingName are the workspace already on the main
	// folder, which the import adds to.
	ExistingID   string `json:"existingId,omitempty"`
	ExistingName string `json:"existingName,omitempty"`
}

// WorkspaceImportSource is one source of an import: for a local folder,
// where it is or would be and whether it is there; for a source of another
// kind, where it is when its kind says, and the problem that leaves it
// unavailable for now, if any. Checked says whether anything was checked:
// a source of a kind this build does not know is kept but not checked.
// Actions are what the person can do with it in the import window:
// "choose-folder" for another folder, and "create-folder" to make it now.
type WorkspaceImportSource struct {
	Key       string   `json:"key"`
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Main      bool     `json:"main"`
	Path      string   `json:"path,omitempty"`
	Found     bool     `json:"found"`
	Folders   int      `json:"folders"`
	Site      string   `json:"site,omitempty"`
	Reference string   `json:"reference,omitempty"`
	Problem   string   `json:"problem,omitempty"`
	Checked   bool     `json:"checked"`
	Actions   []string `json:"actions"`
}

// WorkspaceImportChoice is the person's choice for one local folder of an
// import: another folder for it, or to create it now.
type WorkspaceImportChoice struct {
	Key    string `json:"key"`
	Path   string `json:"path,omitempty"`
	Create bool   `json:"create,omitempty"`
}

// WorkspaceImportResult is an import's outcome: the state, the workspace
// made or added to, and how many sources and pins were added.
type WorkspaceImportResult struct {
	State       State  `json:"state"`
	WorkspaceID string `json:"workspaceId"`
	Existing    bool   `json:"existing"`
	Sources     int    `json:"sources"`
	Pins        int    `json:"pins"`
	Waiting     int    `json:"waiting"`
}

// workspaceFileFilter is the file dialogs' filter for export files.
func workspaceFileFilter() []wruntime.FileFilter {
	return []wruntime.FileFilter{{DisplayName: product.Name + " workspace (*" + workspaces.ExportExtension + ")", Pattern: "*" + workspaces.ExportExtension}}
}

// ExportWorkspace asks where to save a workspace's export, suggesting its
// main folder, and saves it there. It returns the file, or nothing when
// the person cancelled.
func (a *App) ExportWorkspace(id string) (string, error) {
	record, err := a.workspaceSourceRecord(id)
	if err != nil {
		return "", err
	}
	dir := mainFolderOf(record)
	if dir == "" || !folderExists(dir) {
		dir, _ = userHome()
	}
	file, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Export Workspace", DefaultDirectory: dir, DefaultFilename: exportFileName(record.Name), Filters: workspaceFileFilter()})
	if err != nil || file == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(file), workspaces.ExportExtension) {
		file += workspaces.ExportExtension
	}
	return file, a.exportWorkspaceTo(id, file)
}

// exportFileName is a workspace's export file name, with the characters a
// file name cannot have replaced.
func exportFileName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '-'
		}
		return r
	}, name)
	clean = strings.Trim(clean, ". ")
	if clean == "" {
		clean = "Workspace"
	}
	return clean + workspaces.ExportExtension
}

// mainFolderOf is a workspace's main folder, or nothing when it has none
// of its own and works in its private folder.
func mainFolderOf(record WorkspaceRecord) string {
	main := workspaces.ExecutionPath(record)
	for _, source := range record.Sources {
		if source.Kind == workspaces.LocalFolder && layout.SamePath(source.Path, main) {
			return main
		}
	}
	return ""
}

// exportWorkspaceTo writes a workspace's export to file. The file is
// written as a state file is (package statefile), so an interrupted write
// leaves the old file or the new one.
func (a *App) exportWorkspaceTo(id, file string) error {
	record, err := a.workspaceSourceRecord(id)
	if err != nil {
		return err
	}
	home, _ := userHome()
	data, err := json.MarshalIndent(workspaces.BuildExport(record, file, home), "", "  ")
	if err != nil {
		return err
	}
	return statefile.Write(file, append(data, '\n'))
}

// ChooseWorkspaceImport asks for an export file and plans its import.
func (a *App) ChooseWorkspaceImport() (WorkspaceImportPlan, error) {
	home, _ := userHome()
	file, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Import Workspace", DefaultDirectory: home, Filters: workspaceFileFilter()})
	if err != nil || file == "" {
		return WorkspaceImportPlan{}, err
	}
	return a.PlanWorkspaceImport(file, nil)
}

// ChooseImportFolder asks for a folder for one of an import's local
// folders, starting at the folder proposed for it or the nearest folder
// above it that exists.
func (a *App) ChooseImportFolder(start string) (string, error) {
	for start != "" && !folderExists(start) {
		parent := filepath.Dir(start)
		if parent == start {
			start = ""
			break
		}
		start = parent
	}
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Choose the Folder", DefaultDirectory: start})
}

// readWorkspaceImport reads an export file.
func readWorkspaceImport(file string) (workspaces.Export, error) {
	info, err := os.Stat(file)
	if err != nil {
		return workspaces.Export{}, fmt.Errorf("the workspace export could not be read: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return workspaces.Export{}, workspaces.ErrNotAnExport
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return workspaces.Export{}, fmt.Errorf("the workspace export could not be read: %w", err)
	}
	return workspaces.ReadExport(data)
}

// planLocal finds where each local folder of an export is or would be,
// with the person's choices applied, and says what can be done with each
// source, without checking any source of another kind.
func (a *App) planLocal(file string, export workspaces.Export, choices []WorkspaceImportChoice) (WorkspaceImportPlan, error) {
	home, _ := userHome()
	chosen := map[string]WorkspaceImportChoice{}
	for _, c := range choices {
		chosen[c.Key] = c
	}
	plan := WorkspaceImportPlan{File: file, Name: export.Name, Sources: []WorkspaceImportSource{}, Pins: len(export.Pins)}
	for _, pin := range export.Pins {
		if !workspaces.KnownPin(pin.Kind) {
			plan.UncheckedPins++
		}
	}
	for _, source := range export.Sources {
		item := WorkspaceImportSource{Key: source.Key, Kind: source.Kind, Name: source.Name, Main: source.Main, Actions: []string{}}
		if source.Kind == workspaces.LocalFolder {
			item.Path, item.Found = workspaces.Locate(*source.Local, file, home)
			if c, ok := chosen[source.Key]; ok && c.Path != "" {
				path := filepath.Clean(c.Path)
				if !filepath.IsAbs(path) {
					return WorkspaceImportPlan{}, fmt.Errorf("choose a full path for %s", source.Name)
				}
				item.Path, item.Found = path, folderExists(path)
			}
			if item.Path == "" {
				return WorkspaceImportPlan{}, fmt.Errorf("the folder %s has no location that this computer can use; choose a folder for it", source.Name)
			}
			item.Folders = len(source.Local.Folders)
			item.Checked = true
			item.Actions = append(item.Actions, "choose-folder")
			if !item.Found {
				item.Actions = append(item.Actions, "create-folder")
			}
		} else if kind, ok := workspaces.LookupSourceKind(source.Kind); ok && kind.Describe != nil {
			item.Site, item.Reference = kind.Describe(source)
		}
		if item.Main {
			a.mu.Lock()
			if i := workspaces.IndexAt(a.settings.WorkspaceRecords, item.Path); i >= 0 {
				plan.ExistingID, plan.ExistingName = a.settings.WorkspaceRecords[i].ID, a.settings.WorkspaceRecords[i].Name
			}
			a.mu.Unlock()
		}
		plan.Sources = append(plan.Sources, item)
	}
	return plan, nil
}

// PlanWorkspaceImport reads an export and says what importing it would
// do: where each local folder is, what each registered kind of source says
// about whether this computer can use it, and the workspace it would add
// to. A source of a kind this build does not know is kept and not
// checked. choices are the person's own folders for local folders.
func (a *App) PlanWorkspaceImport(file string, choices []WorkspaceImportChoice) (WorkspaceImportPlan, error) {
	export, err := readWorkspaceImport(file)
	if err != nil {
		return WorkspaceImportPlan{}, err
	}
	plan, err := a.planLocal(file, export, choices)
	if err != nil {
		return WorkspaceImportPlan{}, err
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	for i := range plan.Sources {
		item := &plan.Sources[i]
		if item.Kind == workspaces.LocalFolder {
			continue
		}
		kind, ok := workspaces.LookupSourceKind(item.Kind)
		if !ok || kind.Check == nil {
			continue
		}
		checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		item.Problem, item.Checked = kind.Check(checkCtx, export.Sources[i]), true
		cancel()
	}
	return plan, nil
}

// createFolders makes a folder and the folders recorded inside it, those
// that do not exist, and changes none that does.
func createFolders(root string, folders []string) error {
	if !filepath.IsAbs(root) {
		return errors.New("the folder to create has no full path")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("%s could not be created: %w", root, err)
	}
	for _, folder := range folders {
		if !workspaces.CleanFolder(folder) {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(folder))
		if !layout.Inside(root, path) {
			continue
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("%s could not be created: %w", path, err)
		}
	}
	return nil
}

// importedSource makes the registry's source for a source of an export.
func importedSource(item WorkspaceImportSource, from workspaces.ExportSource) WorkspaceSource {
	source := WorkspaceSource{Kind: item.Kind}
	name := item.Name
	if item.Kind == workspaces.LocalFolder {
		source.Path = item.Path
		if name == "" {
			name = filepath.Base(item.Path)
		}
	} else {
		kind, _ := workspaces.LookupSourceKind(item.Kind)
		if kind.Recreate != nil {
			source = kind.Recreate(from)
			source.Kind, source.Path = item.Kind, ""
		} else {
			source.Extra = from.Extra
		}
		if name == "" && kind.DefaultName != nil {
			name = kind.DefaultName(from)
		}
		if name == "" {
			name = item.Kind
		}
	}
	source.ID = "src_" + rand.Text()
	source.Name = defaultWorkspaceLabel(name)
	return source
}

// ImportWorkspace imports an export: it makes the workspace, or adds the
// sources and pins it lacks to the workspace already on its main folder.
// A local folder the person chose to create is made first. A local folder
// that is not on the computer is added all the same, with the folders its
// export recorded, and waits for the folder. A source of another kind is
// added whether or not this computer can use it yet, and one of a kind
// this build does not know is added as it is. The workspace is selected
// when its main folder is here; otherwise it is put in Recent, where it
// waits for its folder.
func (a *App) ImportWorkspace(file string, choices []WorkspaceImportChoice) (WorkspaceImportResult, error) {
	if a.startupErr != nil {
		return WorkspaceImportResult{State: a.State()}, a.startupErr
	}
	export, err := readWorkspaceImport(file)
	if err != nil {
		return WorkspaceImportResult{State: a.State()}, err
	}
	plan, err := a.planLocal(file, export, choices)
	if err != nil {
		return WorkspaceImportResult{State: a.State()}, err
	}
	creating := map[string]bool{}
	for _, c := range choices {
		creating[c.Key] = c.Create
	}
	exported := map[string]workspaces.ExportSource{}
	for _, source := range export.Sources {
		exported[source.Key] = source
	}
	for i := range plan.Sources {
		item := &plan.Sources[i]
		if item.Kind == workspaces.LocalFolder && !item.Found && creating[item.Key] {
			if err := createFolders(item.Path, exported[item.Key].Local.Folders); err != nil {
				return WorkspaceImportResult{State: a.State()}, err
			}
			item.Found = true
		}
	}
	result := WorkspaceImportResult{}
	err = a.updateSettings(func(s *Settings) error {
		updated, _, err := migrateWorkspaceSettings(*s)
		if err != nil {
			return err
		}
		var main *WorkspaceImportSource
		for i := range plan.Sources {
			if plan.Sources[i].Main {
				main = &plan.Sources[i]
			}
		}
		index := -1
		if main != nil {
			index = workspaces.IndexAt(updated.WorkspaceRecords, main.Path)
		}
		if index >= 0 {
			result.Existing = true
		} else {
			record := WorkspaceRecord{ID: "ws_" + rand.Text(), Name: defaultWorkspaceLabel(export.Name), Sources: []WorkspaceSource{}}
			if main != nil {
				record.ExecutionPath = main.Path
			} else {
				record.ExecutionPath = filepath.Join(configDir(), "workspaces", record.ID, "files")
				for _, path := range []string{filepath.Join(configDir(), "workspaces"), filepath.Dir(record.ExecutionPath), record.ExecutionPath} {
					if err := stateDirectory(path); err != nil {
						return err
					}
				}
			}
			updated.WorkspaceRecords = append(updated.WorkspaceRecords, record)
			index = len(updated.WorkspaceRecords) - 1
		}
		record := &updated.WorkspaceRecords[index]
		result.WorkspaceID = record.ID
		ids := map[string]string{}
		for _, item := range plan.Sources {
			from := exported[item.Key]
			source := importedSource(item, from)
			existing := ""
			for _, have := range record.Sources {
				if sameWorkspaceSource(have, source) {
					existing = have.ID
					break
				}
			}
			if existing != "" {
				ids[item.Key] = existing
				continue
			}
			record.Sources = append(record.Sources, source)
			ids[item.Key] = source.ID
			result.Sources++
			if item.Main && record.PrimarySourceID == "" {
				record.PrimarySourceID = source.ID
			}
			if item.Kind == workspaces.LocalFolder && !item.Found {
				result.Waiting++
				if folders := from.Local.Folders; len(folders) > 0 {
					if record.PlannedFolders == nil {
						record.PlannedFolders = map[string][]string{}
					}
					record.PlannedFolders[source.ID] = append([]string{}, folders...)
				}
			}
		}
		have := map[string]bool{}
		for _, pin := range record.Pins {
			have[workspaces.PinKey(pin)] = true
		}
		for _, p := range export.Pins {
			pin := workspaces.Pin{Kind: p.Kind, Title: p.Title, ItemKind: p.ItemKind, Directory: p.Directory, Extra: p.Extra}
			if p.Source != "" {
				pin.Ref = &workspaces.Ref{WorkspaceID: record.ID, SourceID: ids[p.Source], Path: p.Path}
			}
			if key := workspaces.PinKey(pin); key == "" || have[key] {
				continue
			} else {
				have[key] = true
			}
			record.Pins = append(record.Pins, pin)
			result.Pins++
		}
		if err := ensureWorkspaceMountAliases(updated.WorkspaceRecords); err != nil {
			return err
		}
		if main == nil || main.Found {
			updated = selectWorkspaceSettings(updated, updated.WorkspaceRecords[index])
		} else {
			updated.addRecent(main.Path)
		}
		*s = updated
		return nil
	})
	result.State = a.State()
	if err != nil {
		return result, err
	}
	a.emitSettingsChanged()
	if result.Existing {
		a.emitWorkspaceMountChanged("source-attached", result.WorkspaceID, "")
	} else {
		a.emitWorkspaceMountChanged("workspace-created", result.WorkspaceID, "")
	}
	return result, nil
}

// CreateWorkspaceFolders makes a local folder of a workspace that is not
// on the computer, with the folders its import recorded inside it, and
// changes none that exists. It is the person's choice, from the folder's
// menu or from choosing a workspace whose main folder is missing.
func (a *App) CreateWorkspaceFolders(workspaceID, sourceID string) (WorkspaceRecord, error) {
	record, err := a.workspaceSourceRecord(workspaceID)
	if err != nil {
		return WorkspaceRecord{}, err
	}
	var source *WorkspaceSource
	for i := range record.Sources {
		if record.Sources[i].ID == sourceID {
			source = &record.Sources[i]
		}
	}
	if source == nil || source.Kind != workspaces.LocalFolder {
		return WorkspaceRecord{}, errors.New("that workspace folder is not attached")
	}
	if err := createFolders(source.Path, record.PlannedFolders[sourceID]); err != nil {
		return WorkspaceRecord{}, err
	}
	return a.setPlannedFolders(workspaceID, sourceID, "", "folders-created")
}

// RelocateWorkspaceSource points a local folder of a workspace at another
// folder, which the person chooses. A workspace's main folder cannot be
// moved, since its chats find it by its path.
func (a *App) RelocateWorkspaceSource(workspaceID, sourceID string) (WorkspaceRecord, error) {
	record, err := a.workspaceSourceRecord(workspaceID)
	if err != nil {
		return WorkspaceRecord{}, err
	}
	var source WorkspaceSource
	for _, s := range record.Sources {
		if s.ID == sourceID {
			source = s
		}
	}
	if source.Kind != workspaces.LocalFolder {
		return WorkspaceRecord{}, errors.New("that workspace folder is not attached")
	}
	if layout.SamePath(source.Path, workspaces.ExecutionPath(record)) {
		return WorkspaceRecord{}, errors.New("a workspace's main folder cannot be moved")
	}
	path, err := a.ChooseImportFolder(source.Path)
	if err != nil || path == "" {
		return record, err
	}
	return a.relocateWorkspaceSource(workspaceID, sourceID, path)
}

func (a *App) relocateWorkspaceSource(workspaceID, sourceID, path string) (WorkspaceRecord, error) {
	path = filepath.Clean(path)
	if err := checkWorkspaceFolder(path); err != nil {
		return WorkspaceRecord{}, err
	}
	return a.setPlannedFolders(workspaceID, sourceID, path, "source-relocated")
}

// setPlannedFolders sets aside a local folder's recorded folders, now that
// the folder exists, and moves it to path when one is given.
func (a *App) setPlannedFolders(workspaceID, sourceID, path, kind string) (WorkspaceRecord, error) {
	var record WorkspaceRecord
	err := a.updateSettings(func(s *Settings) error {
		index := workspaces.Index(s.WorkspaceRecords, workspaceID)
		if index < 0 {
			return errors.New("that workspace is not registered")
		}
		r := &s.WorkspaceRecords[index]
		found := false
		for i := range r.Sources {
			if r.Sources[i].ID != sourceID || r.Sources[i].Kind != workspaces.LocalFolder {
				continue
			}
			found = true
			if path != "" {
				moved := r.Sources[i]
				moved.Path = path
				for j, other := range r.Sources {
					if j != i && sameWorkspaceSource(other, moved) {
						return errors.New("that folder is already a source of this workspace")
					}
				}
				r.Sources[i].Path = path
			}
		}
		if !found {
			return errors.New("that workspace folder is not attached")
		}
		delete(r.PlannedFolders, sourceID)
		if len(r.PlannedFolders) == 0 {
			r.PlannedFolders = nil
		}
		record = workspaces.Copy(s.WorkspaceRecords[index : index+1])[0]
		return nil
	})
	if err != nil {
		return WorkspaceRecord{}, err
	}
	a.emitSettingsChanged()
	a.emitWorkspaceMountChanged(kind, workspaceID, sourceID)
	return record, nil
}

// folderExists says whether a folder is on the computer.
func folderExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// plannedFolders are the folders the import of a local source recorded,
// shown while the folder is not on the computer.
func (s workspaceResourceService) plannedFolders(ref ResourceRef) []string {
	for _, record := range s.resolver().Records() {
		if record.ID == ref.WorkspaceID {
			return record.PlannedFolders[ref.SourceID]
		}
	}
	return nil
}

// plannedPage lists the recorded folders directly inside a folder of a
// local source that is not on the computer, folders that do not exist yet
// and can be browsed and created.
func plannedPage(ref ResourceRef, planned []string, start, limit int) ResourcePage {
	out := ResourcePage{Entries: []ResourceItem{}, NextStart: -1}
	parent := strings.Trim(ref.Path, "/")
	if parent == "." {
		parent = ""
	}
	names, seen := []string{}, map[string]bool{}
	for _, folder := range planned {
		rest := folder
		if parent != "" {
			if !strings.HasPrefix(folder, parent+"/") {
				continue
			}
			rest = folder[len(parent)+1:]
		}
		name, _, _ := strings.Cut(rest, "/")
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return folderOrder(true, names[i], true, names[j]) })
	if start >= len(names) {
		return out
	}
	names = names[start:]
	if len(names) > limit {
		out.NextStart = start + limit
		names = names[:limit]
	}
	for _, name := range names {
		child := ref
		child.Path = strings.TrimPrefix(parent+"/"+name, "/")
		out.Entries = append(out.Entries, ResourceItem{Ref: child, Title: name, Kind: "directory", Directory: true, Actions: []string{"list", "create-folders"}, Missing: true})
	}
	return out
}

// missingWorkspaces are the workspaces whose main folder is not on the
// computer, by ID: an imported workspace waiting for its folder, or one on
// a drive that is not connected.
func missingWorkspaces(records []WorkspaceRecord) []string {
	out := []string{}
	for _, record := range records {
		if main := mainFolderOf(record); main != "" && !folderExists(main) {
			out = append(out, record.ID)
		}
	}
	return out
}
