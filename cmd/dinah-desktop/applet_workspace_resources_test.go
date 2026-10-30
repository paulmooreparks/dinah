package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/applet"
)

func TestWorkspaceResourceAppletScopesSourcesToCallingWorkspace(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	primary, other, attached := t.TempDir(), t.TempDir(), t.TempDir()
	first, err := a.SetWorkspace(primary)
	if err != nil {
		t.Fatal(err)
	}
	record, err := a.AttachLocalWorkspaceSource(first.WorkspaceID, attached, "Reference Documents")
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.SetWorkspace(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attached, "notes.md"), []byte("# Reference\nMeeting notes."), 0600); err != nil {
		t.Fatal(err)
	}
	bundle, err := a.workspaceAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := builtinTestContext(primary)
	call := func(action string, input any) (string, error) {
		b, err := json.Marshal(input)
		if err != nil {
			return "", err
		}
		result, err := registry.Execute(ctx, action, string(b))
		return result.OutputJSON, err
	}
	output, err := call(workspaceSourcesAction, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	var roots ResourcePage
	if err := json.Unmarshal([]byte(output), &roots); err != nil {
		t.Fatal(err)
	}
	if len(roots.Entries) != 2 || roots.NextStart != -1 || roots.Entries[1].Title != "Reference Documents" || roots.Entries[1].Ref.WorkspaceID != first.WorkspaceID {
		t.Fatalf("source discovery used the selected workspace: %s", output)
	}
	if strings.Contains(output, attached) || strings.Contains(output, primary) {
		t.Fatal("source discovery exposed an absolute local path")
	}
	ref := roots.Entries[1].Ref
	output, err = call(workspaceResourcesAction, map[string]any{"sourceId": ref.SourceID, "limit": 1})
	if err != nil || !strings.Contains(output, "notes.md") {
		t.Fatalf("attached source listing: %s %v", output, err)
	}
	output, err = call(workspaceResourceReadAction, map[string]any{"sourceId": ref.SourceID, "path": "notes.md"})
	if err != nil {
		t.Fatal(err)
	}
	var read ResourceRead
	if err := json.Unmarshal([]byte(output), &read); err != nil {
		t.Fatal(err)
	}
	if read.Text != "# Reference\nMeeting notes." || read.Format != "markdown" || len(read.Revision) != 64 || read.Resource.Ref.WorkspaceID != first.WorkspaceID {
		t.Fatalf("read lost content or provenance: %s", output)
	}
	foreign := second.WorkspaceRecords[len(second.WorkspaceRecords)-1].Sources[0].ID
	for _, input := range []map[string]any{
		{"sourceId": foreign, "path": "notes.md"},
		{"sourceId": ref.SourceID, "path": "../escape"},
		{"sourceId": ref.SourceID, "path": "notes.md", "workspaceId": second.WorkspaceID},
	} {
		if _, err := call(workspaceResourceReadAction, input); err == nil {
			t.Fatalf("accepted foreign or escaping input: %v", input)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := registry.Execute(cancelled, workspaceResourcesAction, `{"sourceId":"`+ref.SourceID+`"}`); err == nil {
		t.Fatal("cancelled listing succeeded")
	}
	if _, err := a.DetachWorkspaceSource(record.ID, ref.SourceID); err != nil {
		t.Fatal(err)
	}
	if _, err := call(workspaceResourceReadAction, map[string]any{"sourceId": ref.SourceID, "path": "notes.md"}); err == nil {
		t.Fatal("detached source remained available")
	}
	if _, err := os.Stat(filepath.Join(attached, "notes.md")); err != nil {
		t.Fatal("detaching deleted the original")
	}
}
