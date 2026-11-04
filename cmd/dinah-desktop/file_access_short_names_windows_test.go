package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
	"studio/internal/agents"
	"studio/internal/applet"
	"studio/internal/bus"
	"studio/internal/fileaccess"
	"studio/internal/layout"
)

func TestWindowsShortNamesPreserveCatalogHistoryAttachmentAndState(t *testing.T) {
	longRoot := filepath.Join(t.TempDir(), "file-access-long-directory-name")
	if err := os.MkdirAll(longRoot, 0700); err != nil {
		t.Fatal(err)
	}
	wide, err := windows.UTF16PtrFromString(longRoot)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 32768)
	n, err := windows.GetShortPathName(wide, &buffer[0], uint32(len(buffer)))
	if err != nil || n == 0 || n >= uint32(len(buffer)) {
		t.Fatalf("GetShortPathNameW: %d %v", n, err)
	}
	shortRoot := windows.UTF16ToString(buffer[:n])
	if strings.EqualFold(shortRoot, longRoot) {
		t.Skip("the volume did not supply an 8.3 name; short-name creation is disabled")
	}
	state := filepath.Join(shortRoot, "state")
	previous := configOverride
	configOverride = state
	t.Cleanup(func() { configOverride = previous })
	a := NewApp()
	t.Cleanup(a.queueStore.Close)
	t.Cleanup(a.closeAppletBridge)
	t.Cleanup(a.closeFileAccess)
	t.Cleanup(a.closeChatStates)
	t.Cleanup(a.closeTeams)
	workspace := filepath.Join(shortRoot, "workspace")
	skillDir := filepath.Join(workspace, ".devin", "skills", "safe")
	if err := os.MkdirAll(skillDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Short-name skill\n---\nSynthetic instructions"), 0600); err != nil {
		t.Fatal(err)
	}
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "short-workspace", Name: "Short-name workspace", ExecutionPath: workspace, Sources: []WorkspaceSource{{ID: "source", Kind: "local-folder", Name: "Files", Path: workspace}}}}
	ctx := builtinTestContext(workspace)
	t.Run("catalogue", func(t *testing.T) {
		catalog := a.skillCatalog("short-workspace", false)
		if len(catalog.Skills) != 1 {
			t.Fatalf("short-name catalogue: %+v", catalog)
		}
		bundle, err := a.skillsAppletBundle()
		if err != nil {
			t.Fatal(err)
		}
		registry, err := applet.New(bundle)
		if err != nil {
			t.Fatal(err)
		}
		input, _ := json.Marshal(map[string]string{"id": catalog.Skills[0].ID})
		result, err := registry.Execute(ctx, "dinah.skills/read@1", string(input))
		if err != nil || !strings.Contains(result.OutputJSON, "Synthetic instructions") {
			t.Fatalf("short-name catalogue read: %s %v", result.OutputJSON, err)
		}
		if !insideFolder(filepath.Join(longRoot, "workspace"), filepath.Join(skillDir, "SKILL.md")) {
			t.Fatal("short-name workspace path was not recognised beneath its long form")
		}
		link := filepath.Join(workspace, "linked-catalogue")
		if err := os.Symlink(skillDir, link); err != nil {
			t.Fatal(err)
		}
		if file, err := agents.OpenCatalogFile(filepath.Join(link, "SKILL.md")); err == nil {
			file.Close()
			t.Fatal("short-name catalogue followed a directory link")
		}
	})
	t.Run("history", func(t *testing.T) {
		a.remember(workspace, "", "send", "Synthetic short-name prompt", nil)
		bundle, err := a.chatAppletBundle()
		if err != nil {
			t.Fatal(err)
		}
		registry, err := applet.New(bundle)
		if err != nil {
			t.Fatal(err)
		}
		result, err := registry.Execute(ctx, chatHistoryAction, `{}`)
		if err != nil || !strings.Contains(result.OutputJSON, "Synthetic short-name prompt") {
			t.Fatalf("short-name history: %s %v", result.OutputJSON, err)
		}
	})
	t.Run("attachment", func(t *testing.T) {
		b, err := bus.OpenAt(filepath.Join(state, "bus.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer b.Close()
		sender, err := b.RegisterSession(context.Background(), "short-sender", workspace, "Sender")
		if err != nil {
			t.Fatal(err)
		}
		reader, err := b.RegisterSession(context.Background(), "short-reader", workspace, "Reader")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(workspace, "note.txt")
		if err := os.WriteFile(path, []byte("Synthetic attachment"), 0600); err != nil {
			t.Fatal(err)
		}
		message, err := b.Send(context.Background(), bus.Sender{Session: &sender}, reader.Address(), "link", []string{path}, 0)
		if err != nil {
			t.Fatal(err)
		}
		read, err := b.ReadAttachment(context.Background(), reader.ID, message.ID, 1)
		if err != nil || !strings.Contains(read.Text, "Synthetic attachment") {
			t.Fatalf("short-name attachment: %+v %v", read, err)
		}
		policy, err := a.FileAccessPolicy()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = a.AddFileAccessEntry(path, policy.Version); err != nil {
			t.Fatal(err)
		}
		if read, err = b.ReadAttachment(context.Background(), reader.ID, message.ID, 1); err == nil || len(read.Data) != 0 || read.Text != "" {
			t.Fatal("short-name attachment bypassed denial", err)
		}
	})
	t.Run("protected-state", func(t *testing.T) {
		path := filepath.Join(state, "protected.txt")
		if err := os.WriteFile(path, []byte("Synthetic state"), 0600); err != nil {
			t.Fatal(err)
		}
		service, err := a.fileAccessService()
		if err != nil {
			t.Fatal(err)
		}
		scope := fileaccess.Scope{Chat: "short-reader", Workspace: workspace, Roots: []string{workspace}, Layout: layout.Layout{State: state}}
		if text, err := service.Read(context.Background(), path, nil, nil, scope); err == nil || text != "" {
			t.Fatal("short-name state was readable", err)
		}
		policy, err := service.Policy(scope)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := agents.ResolvePath(filepath.Join(longRoot, "state"))
		if err != nil {
			t.Fatal(err)
		}
		if !layout.SamePath(policy.Rules[0].ResolvedPaths[0], canonical) {
			t.Fatalf("short-name canonical state: %q, want %q", policy.Rules[0].ResolvedPaths[0], canonical)
		}
		log, err := service.Store.Log(fileaccess.Filter{Outcome: "denied"})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range log.Rows {
			if row.RuleID == "protected-state" {
				found = true
			}
		}
		if !found {
			t.Fatal("short-name state refusal was not audited")
		}
	})
}
