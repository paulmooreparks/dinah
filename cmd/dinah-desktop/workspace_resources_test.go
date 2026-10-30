package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func resourceTestService(t *testing.T, source WorkspaceSource) (workspaceResourceService, ResourceRef) {
	t.Helper()
	a := NewApp()
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "resource-workspace", Name: "Resources", Sources: []WorkspaceSource{source}}}
	return workspaceResourceService{app: a, ctx: context.Background()}, ResourceRef{WorkspaceID: "resource-workspace", SourceID: source.ID}
}
func TestWorkspaceResourcesLocalIsolationAndActions(t *testing.T) {
	root := t.TempDir()
	source := WorkspaceSource{ID: "local", Kind: "local-folder", Name: "Local", Path: root}
	service, ref := resourceTestService(t, source)
	for name, data := range map[string]string{"notes.md": "# Hello", "data.bin": "\x00\xff", "script.ps1": "Write-Host hello", ".env": "secret"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	page, err := service.list(ref, 0, 100)
	if err != nil || len(page.Entries) != 3 || page.NextStart != -1 {
		t.Fatalf("list: %+v %v", page, err)
	}
	for _, item := range page.Entries {
		switch item.Title {
		case "data.bin":
			if slices.Contains(item.Actions, "read") || !slices.Contains(item.Actions, "reveal") {
				t.Fatal("binary actions")
			}
		case "script.ps1":
			if slices.Contains(item.Actions, "open-external") {
				t.Fatal("script execution advertised")
			}
		}
	}
	ref.Path = "notes.md"
	read, err := service.read(ref)
	if err != nil || read.Text != "# Hello" || read.Revision == "" {
		t.Fatalf("read: %+v %v", read, err)
	}
	preview, err := service.preview(ref)
	if err != nil || len(preview.Blocks) == 0 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	for _, path := range []string{"../outside", ".env", "notes.md:ads"} {
		ref.Path = path
		if _, err := service.read(ref); err == nil {
			t.Errorf("read forbidden %s", path)
		}
	}
	ref.Path = "notes.md"
	ref.WorkspaceID = "other"
	if _, err := service.read(ref); err == nil {
		t.Fatal("foreign workspace accepted")
	}
	ref.WorkspaceID = "resource-workspace"
	service.app.settings.WorkspaceRecords[0].Sources = nil
	if _, err := service.read(ref); err == nil {
		t.Fatal("detached source accepted")
	}
}

func TestWorkspaceResourcesPlainTextPreservesBackticks(t *testing.T) {
	root := t.TempDir()
	text := "before ```````````` <script>alert(1)</script> after"
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	service, ref := resourceTestService(t, WorkspaceSource{ID: "local", Kind: "local-folder", Path: root})
	ref.Path = "notes.txt"
	preview, err := service.preview(ref)
	if err != nil || len(preview.Blocks) != 1 || !strings.Contains(preview.Blocks[0].HTML, "````````````") || strings.Contains(preview.Blocks[0].HTML, "<script>") {
		t.Fatalf("plain preview changed or executed text: %+v %v", preview, err)
	}
}

func TestWorkspaceResourcesPaginationAndCancellation(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("%d.txt", i)), []byte("text"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	service, ref := resourceTestService(t, WorkspaceSource{ID: "local", Kind: "local-folder", Path: root})
	seen := map[string]bool{}
	for start := 0; start != -1; {
		page, err := service.list(ref, start, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range page.Entries {
			if seen[entry.Ref.Path] {
				t.Fatal("duplicate page entry")
			}
			seen[entry.Ref.Path] = true
		}
		start = page.NextStart
	}
	if len(seen) != 5 {
		t.Fatal("missing page entries")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.ctx = ctx
	if _, err := service.list(ref, 0, 2); err == nil {
		t.Fatal("cancel ignored")
	}
	ref.Path = "0.txt"
	if _, err := service.read(ref); err == nil {
		t.Fatal("read cancel ignored")
	}
}

// The pages of one browse share the first page's reading of the folder, and
// a new browse reads it again.
func TestWorkspaceResourcesPagesShareOneListing(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("text"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	service, ref := resourceTestService(t, WorkspaceSource{ID: "local", Kind: "local-folder", Path: root})
	first, err := service.list(ref, 0, 1)
	if err != nil || len(first.Entries) != 1 || first.Entries[0].Title != "a.txt" || first.NextStart != 1 {
		t.Fatalf("first page: %+v %v", first, err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := service.list(ref, 1, 1)
	if err != nil || len(second.Entries) != 1 || second.Entries[0].Title != "c.txt" || second.NextStart != -1 {
		t.Fatalf("a later page did not use the first page's listing: %+v %v", second, err)
	}
	again, err := service.list(ref, 0, 100)
	if err != nil || len(again.Entries) != 3 {
		t.Fatalf("a new browse did not read the folder again: %+v %v", again, err)
	}
	if err := os.Remove(filepath.Join(root, "c.txt")); err != nil {
		t.Fatal(err)
	}
	gone, err := service.list(ref, 2, 1)
	if err != nil || len(gone.Entries) != 0 {
		t.Fatalf("a removed entry was listed from the kept reading: %+v %v", gone, err)
	}
}

func TestWorkspaceResourcesDeterministicDirectoryPaging(t *testing.T) {
	for _, order := range [][]string{{"z.txt", ".hidden", "b.txt", "a.txt"}, {"a.txt", "b.txt", ".hidden", "z.txt"}} {
		root := t.TempDir()
		for _, name := range order {
			if err := os.WriteFile(filepath.Join(root, name), []byte("text"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		service, ref := resourceTestService(t, WorkspaceSource{ID: "local", Kind: "local-folder", Path: root})
		var names []string
		for start := 0; start != -1; {
			page, err := service.list(ref, start, 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range page.Entries {
				names = append(names, item.Title)
			}
			if page.NextStart != -1 && page.NextStart <= start {
				t.Fatal("filtered page did not advance")
			}
			start = page.NextStart
		}
		if !slices.Equal(names, []string{"a.txt", "b.txt", "z.txt"}) {
			t.Fatalf("unstable or incomplete ordered pages: %v", names)
		}
		f, err := os.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		_, err = sortedWorkspaceDirectory(context.Background(), f, 3)
		f.Close()
		if err == nil {
			t.Fatal("oversized directory silently truncated")
		}
	}
}
