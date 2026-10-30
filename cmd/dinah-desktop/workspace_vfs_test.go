package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/applet"
	"studio/internal/workspaces"
)

func mountTestVFS(t *testing.T) (workspaceVFS, string) {
	t.Helper()
	root := t.TempDir()
	service, _ := resourceTestService(t, WorkspaceSource{ID: "source-one", Kind: "local-folder", Name: "Local Documents", MountAlias: "local", Path: root})
	service.app.settings.WorkspaceRecords[0].MountAlias = "first"
	service.app.settings.WorkspaceRecords[0].PrimarySourceID = "source-one"
	service.app.settings.WorkspaceRecords = append(service.app.settings.WorkspaceRecords, WorkspaceRecord{ID: "other", Name: "Another Workspace", MountAlias: "another", PrimarySourceID: "source-two", Sources: []WorkspaceSource{{ID: "source-two", Name: "Other", MountAlias: "files", Kind: "local-folder", Path: t.TempDir()}}})
	return workspaceVFS{service: service}, root
}

func TestWorkspaceMountCatalogReferenceSelectsChildSource(t *testing.T) {
	v, root := mountTestVFS(t)
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("notes"), 0600); err != nil {
		t.Fatal(err)
	}
	workspace, err := v.stat("/workspaces/first")
	if err != nil || workspace.Ref == nil || workspace.Ref.SourceID != "" {
		t.Fatalf("workspace reference %+v %v", workspace, err)
	}
	v.expected = workspace.Ref
	for _, path := range []string{"/workspaces/first/local", "/workspaces/first/local/"} {
		page, err := v.list(path, 0, 50)
		if err != nil || len(page.Entries) != 1 || page.Entries[0].Ref.SourceID != "source-one" {
			t.Fatalf("child listing %q: %+v %v", path, page, err)
		}
	}
	read, err := v.read("/workspaces/first/local/notes.md")
	if err != nil || read.Text != "notes" || read.Resource.Ref.SourceID != "source-one" {
		t.Fatalf("child read %+v %v", read, err)
	}
	if _, err := v.list("/workspaces/another/files", 0, 50); err == nil {
		t.Fatal("workspace reference allowed another workspace")
	}
	v.expected = &ResourceRef{WorkspaceID: workspace.Ref.WorkspaceID, SourceID: "detached-source"}
	if _, err := v.list("/workspaces/first/local", 0, 50); err == nil {
		t.Fatal("retained source reference allowed alias replacement")
	}
}

func TestWorkspaceMountCatalogScopeAndAliases(t *testing.T) {
	v, root := mountTestVFS(t)
	page, err := v.list("/workspaces", 0, 1)
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Path != "/workspaces/another" || page.NextStart != 1 {
		t.Fatalf("first page %+v %v", page, err)
	}
	page, err = v.list("/workspaces", page.NextStart, 1)
	if err != nil || page.Entries[0].Path != "/workspaces/first" || page.NextStart != -1 {
		t.Fatalf("second page %+v %v", page, err)
	}
	v.allowedWorkspace = "resource-workspace"
	page, err = v.list("/workspaces", 0, 100)
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Path != "/workspaces/first" {
		t.Fatalf("scoped page %+v %v", page, err)
	}
	if _, err = v.stat("/workspaces/another"); err == nil {
		t.Fatal("foreign workspace allowed")
	}
	v.service.app.settings.WorkspaceRecords[0].Name = "Renamed Label"
	item, err := v.stat("/workspaces/first")
	if err != nil || item.Title != "Renamed Label" {
		t.Fatalf("rename %+v %v", item, err)
	}
	if err = os.WriteFile(filepath.Join(root, "notes.txt"), []byte("mounted text"), 0600); err != nil {
		t.Fatal(err)
	}
	read, err := v.read("/workspaces/first/local/notes.txt")
	if err != nil || read.Text != "mounted text" || read.Resource.Ref.WorkspaceID != "resource-workspace" || read.Revision == "" {
		t.Fatalf("read %+v %v", read, err)
	}
	for _, path := range []string{"C:/Windows", "/etc", "/workspaces/first/local/../notes.txt", "/workspaces/first//local", "/workspaces/first/local/.env", "/workspaces/first/local/notes.txt:ads", "/workspaces/first/local\\notes.txt"} {
		if _, err = v.read(path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v.service.ctx = ctx
	if _, err = v.list("/", 0, 50); err != context.Canceled {
		t.Fatalf("cancellation %v", err)
	}
}

func TestWorkspaceMountHomeConfinementAndSharedResourceAPIs(t *testing.T) {
	isolateWorkspaceSettings(t)
	v, _ := mountTestVFS(t)
	v.service.app.settings.HomeName = "pparks"
	item, err := v.stat("~")
	if err != nil || item.Path != "/home/pparks" || item.Title != "Home" || item.Ref.SourceID != workspaces.HomeSourceID {
		t.Fatalf("home %+v %v", item, err)
	}
	if _, err := v.stat("/home/dinah"); err == nil {
		t.Fatal("the home folder answered to another name")
	}
	if err := os.WriteFile(filepath.Join(configDir(), "home", "notes.txt"), []byte("Home text"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir(), "outside.txt"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	page, err := v.list("~/", 0, 100)
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Path != "/home/pparks/notes.txt" {
		t.Fatalf("home list %+v %v", page, err)
	}
	v.expected = item.Ref
	read, err := v.read("~/notes.txt")
	if err != nil || read.Text != "Home text" || read.Resource.Path != "/home/pparks/notes.txt" {
		t.Fatalf("home read %+v %v", read, err)
	}
	preview, err := v.service.preview(*read.Resource.Ref)
	if err != nil || len(preview.Blocks) == 0 {
		t.Fatalf("shared preview %+v %v", preview, err)
	}
	for _, path := range []string{"/home/../outside.txt", "/home/.secret", "/home/notes.txt:ads", "/workspaces/first/local"} {
		if _, err := v.read(path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	v.expected = nil
	parent, err := v.list("/home", 0, 100)
	if err != nil || len(parent.Entries) != 1 || parent.Entries[0].Path != "/home/pparks" {
		t.Fatalf("home parent %+v %v", parent, err)
	}
	resolved, err := v.statRef(ResourceRef{SourceID: workspaces.HomeSourceID, Path: "notes.txt"})
	if err != nil || resolved.Path != "/home/pparks"+"/notes.txt" {
		t.Fatalf("retained home reference %+v %v", resolved, err)
	}
	v.allowedWorkspace = "resource-workspace"
	if _, err := v.list("/home", 0, 100); err == nil {
		t.Fatal("Home exposed to workspace-scoped action")
	}
	page, err = v.list("/", 0, 100)
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Path != "/workspaces" {
		t.Fatalf("scoped root %+v %v", page, err)
	}
	if _, err := v.list("/bin", 0, 100); err == nil {
		t.Fatal("/bin exposed to workspace-scoped action")
	}
}

func TestWorkspaceMountChangesOnlyPublishAfterSave(t *testing.T) {
	isolateWorkspaceSettings(t)
	v, _ := mountTestVFS(t)
	a := v.service.app
	var events []WorkspaceMountChange
	a.sink = func(name string, data any) {
		if name == "workspace:changed" {
			events = append(events, data.(WorkspaceMountChange))
		}
	}
	record, err := a.RenameWorkspace("resource-workspace", "Updated Name")
	if err != nil || record.MountAlias != "Updated Name" || len(events) != 1 || events[0].Kind != "workspace-renamed" {
		t.Fatalf("rename %+v events %+v %v", record, events, err)
	}
	if _, err := a.RenameWorkspace("missing", "Name"); err == nil || len(events) != 1 {
		t.Fatal("failed mutation emitted a change")
	}
	if _, err := a.AttachLocalWorkspaceSource(record.ID, t.TempDir(), "Attached"); err != nil || len(events) != 2 || events[1].Kind != "source-attached" {
		t.Fatalf("attach events %+v %v", events, err)
	}
	if _, err := a.DetachWorkspaceSource(record.ID, events[1].SourceID); err != nil || len(events) != 3 || events[2].Kind != "source-detached" {
		t.Fatalf("detach events %+v %v", events, err)
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	configOverride = blocked
	if _, err := a.RenameWorkspace(record.ID, "Failed Save"); err == nil || len(events) != 3 {
		t.Fatal("failed persistence emitted a change")
	}
	if a.settings.WorkspaceRecords[0].Name != "Updated Name" {
		t.Fatal("failed persistence changed the registry")
	}
}

func TestWorkspaceMountExpectedRejectsReusedAliases(t *testing.T) {
	v, root := mountTestVFS(t)
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	expected := ResourceRef{WorkspaceID: "resource-workspace", SourceID: "source-one", Path: "retained-directory"}
	v.expected = &expected
	if _, err := v.read("/workspaces/first/local/notes.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := v.stat("/workspaces/first"); err == nil {
		t.Fatal("source anchor accepted workspace parent")
	}
	workspaceOnly := ResourceRef{WorkspaceID: expected.WorkspaceID}
	v.expected = &workspaceOnly
	if _, err := v.list("/workspaces/first", 0, 100); err != nil {
		t.Fatal(err)
	}
	v.expected = &expected
	replacement := t.TempDir()
	if err := os.WriteFile(filepath.Join(replacement, "notes.txt"), []byte("replacement must not be read"), 0600); err != nil {
		t.Fatal(err)
	}
	v.service.app.settings.WorkspaceRecords[0].Sources[0] = WorkspaceSource{ID: "replacement-source", MountAlias: "local", Kind: "local-folder", Path: replacement}
	for _, operation := range []func() error{
		func() error { _, err := v.list("/workspaces/first/local", 0, 100); return err },
		func() error { _, err := v.stat("/workspaces/first/local/notes.txt"); return err },
		func() error { _, err := v.read("/workspaces/first/local/notes.txt"); return err },
	} {
		if err := operation(); err == nil {
			t.Fatal("reused source alias accepted")
		}
	}
	v.service.app.settings.WorkspaceRecords[0].ID = "replacement-workspace"
	v.expected = &workspaceOnly
	if _, err := v.list("/workspaces/first", 0, 100); err == nil {
		t.Fatal("reused workspace alias accepted")
	}
	v.expected = &ResourceRef{}
	if _, err := v.stat("/workspaces/first"); err == nil {
		t.Fatal("empty expected identity accepted")
	}
}

func TestWorkspaceMountSDKAllAppletsShareScope(t *testing.T) {
	v, root := mountTestVFS(t)
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("shared contract"), 0600); err != nil {
		t.Fatal(err)
	}
	a := v.service.app
	for _, tc := range []struct {
		id   string
		make func() (applet.Bundle, error)
	}{{"dinah.workspace/mount-", a.workspaceAppletBundle}, {"dinah.files/", a.filesAppletBundle}, {"dinah.terminal/", a.terminalAppletBundle}} {
		t.Run(tc.id, func(t *testing.T) {
			bundle, err := tc.make()
			if err != nil {
				t.Fatal(err)
			}
			registry, err := applet.New(bundle)
			if err != nil {
				t.Fatal(err)
			}
			ctx := builtinTestContext(root)
			result, err := registry.Execute(ctx, tc.id+"list@1", `{"path":"/workspaces"}`)
			if err != nil {
				t.Fatal(err)
			}
			var page MountPage
			if err = json.Unmarshal([]byte(result.OutputJSON), &page); err != nil {
				t.Fatal(err)
			}
			if len(page.Entries) != 1 || page.Entries[0].Path != "/workspaces/first" {
				t.Fatalf("catalog %s", result.OutputJSON)
			}
			result, err = registry.Execute(ctx, tc.id+"read@1", `{"path":"/workspaces/first/local/notes.txt"}`)
			if err != nil {
				t.Fatal(err)
			}
			var read MountRead
			_ = json.Unmarshal([]byte(result.OutputJSON), &read)
			if read.Text != "shared contract" {
				t.Fatalf("read %s", result.OutputJSON)
			}
			if _, err = registry.Execute(ctx, tc.id+"stat@1", `{"path":"/workspaces/another"}`); err == nil {
				t.Fatal("foreign workspace accepted")
			}
			if _, err = registry.Execute(context.Background(), tc.id+"list@1", `{"path":"/"}`); err == nil {
				t.Fatal("missing invocation accepted")
			}
		})
	}
}
