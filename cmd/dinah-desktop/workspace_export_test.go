package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"studio/internal/workspaces"
)

// onComputer gives a test its own settings and home folder, as if it ran
// on another computer.
func onComputer(t *testing.T, home string) *App {
	t.Helper()
	isolateWorkspaceSettings(t)
	previous := userHome
	userHome = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHome = previous })
	a := closeBusWhenDone(t, NewApp())
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}
	return a
}

func makeFolders(t *testing.T, root string, folders ...string) {
	t.Helper()
	for _, folder := range append([]string{"."}, folders...) {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(folder)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func findSource(record WorkspaceRecord, name string) WorkspaceSource {
	for _, s := range record.Sources {
		if s.Name == name {
			return s
		}
	}
	return WorkspaceSource{}
}

func rawFields(fields map[string]any) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for name, value := range fields {
		data, _ := json.Marshal(value)
		out[name] = data
	}
	return out
}

// testSiteKind is a kind of source registered as a build that has one
// would register it, which the plan checks through its own Check.
func init() {
	workspaces.RegisterSourceKind("test-site", workspaces.SourceKind{
		Describe: func(s workspaces.ExportSource) (string, string) {
			var site, page string
			json.Unmarshal(s.Extra["site"], &site)
			json.Unmarshal(s.Extra["page"], &page)
			return site, page
		},
		Check: func(_ context.Context, s workspaces.ExportSource) string {
			var site string
			json.Unmarshal(s.Extra["site"], &site)
			if site != "https://reachable.example.com" {
				return "Needs a connection to " + site
			}
			return ""
		},
	})
}

// exportFromDesktop makes the Orchard workspace on one computer, exports it
// into its main folder, and returns the export. Besides its two local
// folders it has a source and a pin of kinds this build does not know, as
// a workspace imported from a build that knows them has, and a source of
// a registered kind. The computer's folders are then removed, so that the
// test that imports it is on another computer.
func exportFromDesktop(t *testing.T) []byte {
	t.Helper()
	home, err := os.MkdirTemp("", "desktop")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	main := filepath.Join(home, "OneDrive", "Orchard")
	notes := filepath.Join(home, "OneDrive", "Notes")
	makeFolders(t, main, "Concepts/GAP 01")
	makeFolders(t, notes, "Meetings/2026", "Research")
	a := onComputer(t, home)
	st, err := a.SetWorkspace(main)
	if err != nil {
		t.Fatal(err)
	}
	id := st.WorkspaceID
	if _, err := a.RenameWorkspace(id, "Orchard"); err != nil {
		t.Fatal(err)
	}
	record, err := a.AttachLocalWorkspaceSource(id, notes, "Notes")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.updateSettings(func(s *Settings) error {
		r := &s.WorkspaceRecords[workspaces.Index(s.WorkspaceRecords, id)]
		r.Sources = append(r.Sources,
			WorkspaceSource{ID: "src_c", Kind: "web-page", Name: "Concepts", Extra: rawFields(map[string]any{"baseUrl": "https://wiki.example.com", "pageId": "123", "includeChildren": true})},
			WorkspaceSource{ID: "src_t", Kind: "test-site", Name: "Site", Extra: rawFields(map[string]any{"site": "https://elsewhere.example.com", "page": "7"})})
		r.Pins = []workspaces.Pin{
			{Kind: "resource", Title: "Meetings", Ref: &workspaces.Ref{WorkspaceID: id, SourceID: findSource(record, "Notes").ID, Path: "Meetings"}, Directory: true},
			{Kind: "flow", Title: "Review", Extra: rawFields(map[string]any{"flowScope": "workspace", "flowFile": "review.flow.md"})},
		}
		return ensureWorkspaceMountAliases(s.WorkspaceRecords)
	}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(main, "Orchard"+workspaces.ExportExtension)
	if err := a.exportWorkspaceTo(id, file); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// saveExport puts an export where the other computer has it.
func saveExport(t *testing.T, data []byte, dir string) string {
	t.Helper()
	makeFolders(t, dir)
	file := filepath.Join(dir, "Orchard"+workspaces.ExportExtension)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func mustRead(t *testing.T, file string) workspaces.Export {
	t.Helper()
	export, err := readWorkspaceImport(file)
	if err != nil {
		t.Fatal(err)
	}
	return export
}

// On the laptop, OneDrive has brought the main folder and the export but
// not yet the notes folder: the workspace is made and opened, the notes
// folder waits with its folders shown, the source and pin of kinds this
// build does not know are kept as they are, and importing again adds
// nothing.
func TestImportWorkspaceOnAnotherComputer(t *testing.T) {
	data := exportFromDesktop(t)
	laptopHome := t.TempDir()
	main := filepath.Join(laptopHome, "OneDrive", "Orchard")
	makeFolders(t, main, "Concepts/GAP 01")
	file := saveExport(t, data, main)
	a := onComputer(t, laptopHome)

	plan, err := a.PlanWorkspaceImport(file, nil)
	if err != nil || plan.ExistingID != "" || len(plan.Sources) != 4 || plan.Pins != 2 || plan.UncheckedPins != 1 {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	if s := plan.Sources[0]; !s.Main || !s.Found || s.Path != main || !slices.Equal(s.Actions, []string{"choose-folder"}) {
		t.Fatalf("main folder: %+v", s)
	}
	if s := plan.Sources[1]; s.Found || s.Path != filepath.Join(laptopHome, "OneDrive", "Notes") || s.Folders != 3 || !slices.Contains(s.Actions, "create-folder") {
		t.Fatalf("notes folder: %+v", s)
	}
	if s := plan.Sources[2]; s.Kind != "web-page" || s.Checked || s.Problem != "" || len(s.Actions) != 0 {
		t.Fatalf("a source of a kind this build does not know is kept, not checked: %+v", s)
	}
	if s := plan.Sources[3]; !s.Checked || s.Site != "https://elsewhere.example.com" || s.Reference != "7" || s.Problem != "Needs a connection to https://elsewhere.example.com" {
		t.Fatalf("a registered kind is described and checked by its kind: %+v", s)
	}

	result, err := a.ImportWorkspace(file, nil)
	if err != nil || result.Existing || result.Sources != 4 || result.Pins != 2 || result.Waiting != 1 {
		t.Fatalf("import: %+v %v", result, err)
	}
	st := a.State()
	if st.WorkspaceID != result.WorkspaceID || st.Workspace != main || len(st.MissingWorkspaces) != 0 {
		t.Fatalf("the workspace was not opened: %+v", st)
	}
	record := st.WorkspaceRecords[workspaces.Index(st.WorkspaceRecords, result.WorkspaceID)]
	notes := findSource(record, "Notes")
	if record.Name != "Orchard" || record.PrimarySourceID != findSource(record, "Orchard").ID || len(record.PlannedFolders[notes.ID]) != 3 {
		t.Fatalf("record: %+v", record)
	}
	if record.Pins[0].Ref.SourceID != notes.ID || record.Pins[0].Ref.WorkspaceID != record.ID {
		t.Fatalf("pin not moved to the new source: %+v", record.Pins[0].Ref)
	}
	if c := findSource(record, "Concepts"); string(c.Extra["pageId"]) != `"123"` || string(c.Extra["includeChildren"]) != "true" {
		t.Fatalf("the source of a kind this build does not know lost its fields: %+v", c)
	}
	if p := record.Pins[1]; p.Kind != "flow" || string(p.Extra["flowFile"]) != `"review.flow.md"` {
		t.Fatalf("the pin of a kind this build does not know lost its fields: %+v", p)
	}
	// The settings survive being saved and read again, the kept fields too.
	reloaded := closeBusWhenDone(t, NewApp())
	if reloaded.startupErr != nil {
		t.Fatal(reloaded.startupErr)
	}
	again := reloaded.State().WorkspaceRecords[workspaces.Index(reloaded.State().WorkspaceRecords, record.ID)]
	if c := findSource(again, "Concepts"); string(c.Extra["baseUrl"]) != `"https://wiki.example.com"` {
		t.Fatalf("the saved registry lost the kept fields: %+v", c)
	}
	if item := workspaceSourceResource(record, findSource(record, "Concepts")); !item.Unavailable || len(item.Actions) != 0 {
		t.Fatalf("a source of a kind this build does not know offers actions: %+v", item)
	}

	// The missing folder and the folders recorded in it can be browsed.
	root := workspaceSourceResource(record, notes)
	if !root.Missing || !slices.Contains(root.Actions, "create-folders") || !slices.Contains(root.Actions, "choose-folder") || root.Writable {
		t.Fatalf("missing source item: %+v", root)
	}
	if main := workspaceSourceResource(record, findSource(record, "Orchard")); main.Missing || slices.Contains(main.Actions, "choose-folder") {
		t.Fatal("a folder that is here was marked missing")
	}
	page, err := a.ListWorkspaceResources(ResourceRef{WorkspaceID: record.ID, SourceID: notes.ID}, 0, 100)
	if err != nil || len(page.Entries) != 2 || page.Entries[0].Title != "Meetings" || !page.Entries[0].Missing {
		t.Fatalf("planned top folders: %+v %v", page, err)
	}
	page, err = a.ListWorkspaceResources(ResourceRef{WorkspaceID: record.ID, SourceID: notes.ID, Path: "Meetings"}, 0, 100)
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Ref.Path != "Meetings/2026" {
		t.Fatalf("planned inner folders: %+v %v", page, err)
	}

	// Importing again adds nothing.
	twice, err := a.ImportWorkspace(file, nil)
	if err != nil || !twice.Existing || twice.Sources != 0 || twice.Pins != 0 || twice.WorkspaceID != result.WorkspaceID {
		t.Fatalf("second import: %+v %v", twice, err)
	}

	// Exporting the imported workspace writes the kept source and pin as
	// they came.
	out := filepath.Join(t.TempDir(), "again"+workspaces.ExportExtension)
	if err := a.exportWorkspaceTo(record.ID, out); err != nil {
		t.Fatal(err)
	}
	reexported := mustRead(t, out)
	original, _ := workspaces.ReadExport(data)
	if !workspaces.SameExtra(reexported.Sources[2].Extra, original.Sources[2].Extra) || !workspaces.SameExtra(reexported.Pins[1].Extra, original.Pins[1].Extra) {
		t.Fatalf("the kept source or pin changed:\n%+v\n%+v", reexported, original)
	}
	if notesOut := reexported.Sources[1]; len(notesOut.Local.Folders) != 3 {
		t.Fatalf("a folder still to come is exported with the folders recorded for it: %+v", notesOut.Local)
	}

	// Create the Folders… makes the folder and those inside it.
	created, err := a.CreateWorkspaceFolders(record.ID, notes.ID)
	if err != nil || created.PlannedFolders != nil {
		t.Fatalf("create: %+v %v", created, err)
	}
	for _, folder := range []string{"Meetings/2026", "Research"} {
		if !folderExists(filepath.Join(notes.Path, filepath.FromSlash(folder))) {
			t.Fatalf("%s was not created", folder)
		}
	}
	if workspaceSourceResource(created, notes).Missing {
		t.Fatal("a created folder is still missing")
	}

	// The main folder cannot be moved; another can.
	if _, err := a.RelocateWorkspaceSource(record.ID, record.PrimarySourceID); err == nil {
		t.Fatal("the main folder was moved")
	}
	other := t.TempDir()
	if moved, err := a.relocateWorkspaceSource(record.ID, notes.ID, other); err != nil || findSource(moved, "Notes").Path != other {
		t.Fatalf("relocate: %+v %v", moved, err)
	}
}

// When the main folder has not arrived, the workspace is imported and
// waits: it is listed as missing and cannot be opened until its folders
// are created or appear.
func TestImportWorkspaceWaitsForItsMainFolder(t *testing.T) {
	file := saveExport(t, exportFromDesktop(t), filepath.Join(t.TempDir(), "Downloads"))
	a := onComputer(t, t.TempDir())
	result, err := a.ImportWorkspace(file, nil)
	if err != nil || result.Waiting != 2 {
		t.Fatalf("import: %+v %v", result, err)
	}
	st := a.State()
	if st.WorkspaceID == result.WorkspaceID || !slices.Contains(st.MissingWorkspaces, result.WorkspaceID) {
		t.Fatalf("a workspace without its main folder was opened or not marked: %+v", st)
	}
	record := st.WorkspaceRecords[workspaces.Index(st.WorkspaceRecords, result.WorkspaceID)]
	if !slices.Contains(st.Recent, workspaces.ExecutionPath(record)) {
		t.Fatal("the waiting workspace is not listed")
	}
	if _, err := a.SelectWorkspace(record.ID); err == nil {
		t.Fatal("a workspace without its main folder was opened")
	}
	if _, err := a.CreateWorkspaceFolders(record.ID, record.PrimarySourceID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SelectWorkspace(record.ID); err != nil || !folderExists(filepath.Join(workspaces.ExecutionPath(record), "Concepts", "GAP 01")) {
		t.Fatalf("the created workspace could not be opened: %v", err)
	}
}

// A folder the person chose to create now is made during the import.
func TestImportWorkspaceCreatesWhatThePersonChose(t *testing.T) {
	file := saveExport(t, exportFromDesktop(t), filepath.Join(t.TempDir(), "Downloads"))
	a := onComputer(t, t.TempDir())
	plan, err := a.planLocal(file, mustRead(t, file), nil)
	if err != nil {
		t.Fatal(err)
	}
	choices := []WorkspaceImportChoice{}
	for _, s := range plan.Sources {
		if slices.Contains(s.Actions, "create-folder") {
			choices = append(choices, WorkspaceImportChoice{Key: s.Key, Create: true})
		}
	}
	result, err := a.ImportWorkspace(file, choices)
	if err != nil || result.Waiting != 0 || a.State().WorkspaceID != result.WorkspaceID {
		t.Fatalf("import: %+v %v", result, err)
	}
	if !folderExists(filepath.Join(plan.Sources[1].Path, "Meetings", "2026")) {
		t.Fatal("the chosen folder was not created with its folders")
	}
}

// A file that is not an export, and one from a newer version, are refused
// with the product's name in what the person is told.
func TestImportRefusesAFileItCannotRead(t *testing.T) {
	a := onComputer(t, t.TempDir())
	dir := t.TempDir()
	other := filepath.Join(dir, "notes.txt")
	os.WriteFile(other, []byte("hello"), 0o644)
	if _, err := a.PlanWorkspaceImport(other, nil); err == nil {
		t.Fatal("a file that is not an export was planned")
	}
	newer := filepath.Join(dir, "newer"+workspaces.ExportExtension)
	os.WriteFile(newer, []byte(`{"format":2,"name":"x","sources":[]}`), 0o644)
	if _, err := a.ImportWorkspace(newer, nil); err == nil || !strings.Contains(err.Error(), "update "+product.Name) {
		t.Fatalf("a newer export: %v", err)
	}
	if got := exportFileName(`Orchard: "draft"`); got != "Orchard- -draft-"+workspaces.ExportExtension {
		t.Fatalf("file name: %q", got)
	}
}
