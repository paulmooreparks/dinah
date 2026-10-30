package main

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/workspaces"
)

func TestWorkspaceResourceGatewayEnforcesCallingScope(t *testing.T) {
	v, _ := mountTestVFS(t)
	v.allowedWorkspace = "resource-workspace"
	for _, ref := range []ResourceRef{
		{WorkspaceID: "other", SourceID: "source-two", Path: "note.txt"},
		{SourceID: workspaces.HomeSourceID, Path: "note.txt"},
	} {
		if _, err := v.readResource(ref); err == nil {
			t.Fatal("read accepted a resource outside the calling workspace")
		}
		if _, err := v.listResource(ref, 0, 50); err == nil {
			t.Fatal("list accepted a resource outside the calling workspace")
		}
		if _, err := v.previewResource(ref); err == nil {
			t.Fatal("preview accepted a resource outside the calling workspace")
		}
		if _, err := v.resourceAttachment(ref, "image.png"); err == nil {
			t.Fatal("attachment accepted a resource outside the calling workspace")
		}
	}
}

func TestLegacyWorkspaceReadUsesPrimaryVFSSource(t *testing.T) {
	v, executionPath := mountTestVFS(t)
	primary := t.TempDir()
	if err := os.WriteFile(filepath.Join(primary, "note.txt"), []byte("mounted primary source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(executionPath, "note.txt"), []byte("execution folder"), 0600); err != nil {
		t.Fatal(err)
	}
	v.service.app.settings.WorkspaceRecords[0].ExecutionPath = executionPath
	v.service.app.settings.WorkspaceRecords[0].Sources[0].Path = primary
	result, err := v.service.app.workspaceReadOperation(builtinTestContext(executionPath), `{"path":"note.txt"}`, false)
	if err != nil || !strings.Contains(result.OutputJSON, "mounted primary source") || strings.Contains(result.OutputJSON, "execution folder") {
		t.Fatalf("legacy action did not use the VFS primary source: %s, %v", result.OutputJSON, err)
	}
}

func TestLegacyDocumentUsesPrimaryVFSSource(t *testing.T) {
	v, executionPath := mountTestVFS(t)
	primary := t.TempDir()
	for _, fixture := range []struct{ root, text string }{{executionPath, "Execution Folder"}, {primary, "Mounted Source"}} {
		for _, filename := range []string{"note.md", "picture.txt"} {
			if err := os.WriteFile(filepath.Join(fixture.root, filename), []byte("# "+fixture.text), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	a := v.service.app
	a.settings.WorkspaceRecords[0].ExecutionPath = executionPath
	a.settings.WorkspaceRecords[0].Sources[0].Path = primary
	ref, err := a.DocumentRefAt(executionPath, "note.md")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := (workspaceVFS{service: a.resourceService()}).readResource(ref)
	if err != nil || !strings.Contains(doc.Text, "Mounted Source") {
		t.Fatalf("document bypassed the mounted source: %+v, %v", doc, err)
	}
	query := url.Values{"workspace": {executionPath}, "doc": {"note.md"}, "name": {"picture.txt"}}
	w := httptest.NewRecorder()
	a.docFiles().ServeHTTP(w, httptest.NewRequest("GET", "/doc-file?"+query.Encode(), nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Mounted Source") {
		t.Fatalf("attachment bypassed the mounted source: %d %s", w.Code, w.Body.String())
	}
	a.settings.WorkspaceRecords = nil
	if _, err := a.DocumentRefAt(executionPath, "note.md"); err == nil {
		t.Fatal("document accepted an unregistered workspace")
	}
}
