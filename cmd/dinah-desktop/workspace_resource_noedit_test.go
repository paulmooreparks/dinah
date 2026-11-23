package main

import (
	"slices"
	"testing"

	"studio/internal/resource"
)

/* A source may decline Edit on an entry (resource.Entry.NoEdit,
   docs/design/source-kinds.md). A folder that is also a document then
   lists, reads and previews without the edit action, which is what the
   page builds the Edit row from. An entry without NoEdit keeps every action
   it had. */

func TestProviderItemOfADocumentFolderWithNoEdit(t *testing.T) {
	parent := ResourceRef{WorkspaceID: "w", SourceID: "s"}
	plain := providerItem(parent, WorkspaceSource{}, resource.Entry{Name: "100", Folder: true, Document: true}, false)
	declined := providerItem(parent, WorkspaceSource{}, resource.Entry{Name: "200", Folder: true, Document: true, NoEdit: true}, false)
	for _, action := range []string{"list", "read", "preview", "edit"} {
		if !slices.Contains(plain.Actions, action) {
			t.Fatalf("a document folder lacks %s: %v", action, plain.Actions)
		}
	}
	if slices.Contains(declined.Actions, "edit") {
		t.Fatalf("a document folder with NoEdit is offered for editing: %v", declined.Actions)
	}
	for _, action := range []string{"list", "read", "preview"} {
		if !slices.Contains(declined.Actions, action) {
			t.Fatalf("a document folder with NoEdit lacks %s: %v", action, declined.Actions)
		}
	}
}

func TestProviderItemOfAFileWithNoEdit(t *testing.T) {
	editable := providerItem(ResourceRef{}, WorkspaceSource{}, resource.Entry{Name: "plan.md", Size: 3, Editable: true}, false)
	declined := providerItem(ResourceRef{}, WorkspaceSource{}, resource.Entry{Name: "plan.md", Size: 3, Editable: true, NoEdit: true}, false)
	if !slices.Contains(editable.Actions, "edit") {
		t.Fatalf("an editable file lacks edit: %v", editable.Actions)
	}
	if slices.Contains(declined.Actions, "edit") || !slices.Contains(declined.Actions, "read") {
		t.Fatalf("a file marked Editable and NoEdit: %v", declined.Actions)
	}
}
