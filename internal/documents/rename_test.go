package documents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/layout"
	"studio/internal/workspaces"
)

func renameOps(t *testing.T) (Ops, string) {
	t.Helper()
	state, main := t.TempDir(), t.TempDir()
	record := workspaces.Record{ID: "w", Name: "Notes", Sources: []workspaces.Source{{ID: "main", Kind: "local-folder", Name: "Notes", Path: main}}}
	resolver := workspaces.Resolver{Layout: layout.Layout{State: state}, Records: func() []workspaces.Record { return []workspaces.Record{record} }}
	service, err := Service(resolver.Layout, resolver)
	if err != nil {
		t.Fatal(err)
	}
	return Ops{Ctx: context.Background(), Drafts: service, Resolver: resolver}, main
}

func write(t *testing.T, name, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A renamed file keeps its open draft, with its changes, and the draft's
// title follows the file's name.
func TestRenameTakesTheDraftAlong(t *testing.T) {
	o, main := renameOps(t)
	write(t, filepath.Join(main, "notes", "old.md"), "# Old\n")
	ref := workspaces.Ref{WorkspaceID: "w", SourceID: "main", Path: "notes/old.md"}
	v, err := o.Open(ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.Update(v.ID, v.Revision, "# Old\n\nChanged.\n", nil); err != nil {
		t.Fatal(err)
	}

	renamed, err := o.Rename(ref, "new.md")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Ref.Path != "notes/new.md" || len(renamed.Drafts) != 1 {
		t.Fatalf("renamed: %+v", renamed)
	}
	if _, err := os.Stat(filepath.Join(main, "notes", "old.md")); !os.IsNotExist(err) {
		t.Fatal("the old file is still there")
	}
	if data, err := os.ReadFile(filepath.Join(main, "notes", "new.md")); err != nil || string(data) != "# Old\n" {
		t.Fatalf("the new file: %q %v", data, err)
	}
	after, _ := o.Draft(v.ID)
	if after.Origin.Path != "notes/new.md" || after.Title != "new.md" || !strings.Contains(after.Text, "Changed.") || !after.Changed() {
		t.Fatalf("draft after rename: %+v", after)
	}
	// Opening the file by its new name finds the same draft.
	again, err := o.Open(renamed.Ref)
	if err != nil || again.ID != v.ID {
		t.Fatalf("opened again: %v %v", again.ID, err)
	}
}

// A name another file of the folder has is refused, and nothing moves.
func TestRenameReplacesNothing(t *testing.T) {
	o, main := renameOps(t)
	write(t, filepath.Join(main, "a.md"), "A\n")
	write(t, filepath.Join(main, "b.md"), "B\n")
	ref := workspaces.Ref{WorkspaceID: "w", SourceID: "main", Path: "a.md"}
	if _, err := o.Rename(ref, "b.md"); err == nil || !strings.Contains(err.Error(), "already in the folder") {
		t.Fatalf("err = %v", err)
	}
	for _, bad := range []string{"", "x/y.md", `x\y.md`, ".."} {
		if _, err := o.Rename(ref, bad); err == nil {
			t.Fatalf("the name %q was taken", bad)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(main, "b.md")); string(data) != "B\n" {
		t.Fatal("b.md was replaced")
	}
	if _, err := os.Stat(filepath.Join(main, "a.md")); err != nil {
		t.Fatal("a.md moved")
	}
}

// A title the person gave the draft stays when the file is renamed.
func TestRenameKeepsATitleThePersonGave(t *testing.T) {
	o, main := renameOps(t)
	write(t, filepath.Join(main, "a.md"), "A\n")
	ref := workspaces.Ref{WorkspaceID: "w", SourceID: "main", Path: "a.md"}
	v, err := o.Open(ref)
	if err != nil {
		t.Fatal(err)
	}
	title := "My Plan"
	if _, err := o.Update(v.ID, v.Revision, "A\n", &title); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Rename(ref, "plan.md"); err != nil {
		t.Fatal(err)
	}
	d, _ := o.Draft(v.ID)
	if d.Title != "My Plan" || d.Baseline.Title != "plan.md" || d.Origin.Path != "plan.md" {
		t.Fatalf("draft: title %q baseline %q path %q", d.Title, d.Baseline.Title, d.Origin.Path)
	}
}

// A change of case only renames the file, on a disk that ignores case too.
func TestRenameChangesCase(t *testing.T) {
	o, main := renameOps(t)
	write(t, filepath.Join(main, "plan.md"), "A\n")
	ref := workspaces.Ref{WorkspaceID: "w", SourceID: "main", Path: "plan.md"}
	if _, err := o.Rename(ref, "Plan.md"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(main)
	if err != nil || len(entries) != 1 || entries[0].Name() != "Plan.md" {
		t.Fatalf("entries: %v %v", entries, err)
	}
}
