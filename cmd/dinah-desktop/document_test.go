package main

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentKeepsWorkspace(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for i, ws := range []string{first, second} {
		body := []byte("# " + []string{"First", "Second"}[i] + "\n")
		if err := os.WriteFile(filepath.Join(ws, "note.md"), body, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "picture.txt"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	a := NewApp()
	a.settings.Workspace = second
	a.settings.WorkspaceRecords = []WorkspaceRecord{newWorkspaceRecord(first), newWorkspaceRecord(second)}
	q := url.Values{"workspace": {first}, "doc": {"note.md"}, "name": {"picture.txt"}}
	r := httptest.NewRequest("GET", "/doc-file?"+q.Encode(), nil)
	w := httptest.NewRecorder()
	a.docFiles().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "First") {
		t.Fatalf("attachment changed workspace: %d %s", w.Code, w.Body.String())
	}
	q.Set("doc", "../outside.md")
	w = httptest.NewRecorder()
	a.docFiles().ServeHTTP(w, httptest.NewRequest("GET", "/doc-file?"+q.Encode(), nil))
	if w.Code != 404 {
		t.Fatalf("explicit document path escaped the workspace: %d", w.Code)
	}
	prompt := withDoc("Read this", &DocRef{Workspace: first, Path: "note.md", Title: "Note"})
	if !strings.Contains(prompt, filepath.Join(first, "note.md")) {
		t.Fatal("prompt lost the document's workspace")
	}
}

// A document open in the Editor is told to the agent with its file and the
// selected passage, which stays out of the history.
func TestDraftContextNamesTheDocument(t *testing.T) {
	doc := &DocRef{Path: "notes/plan.md", Title: "Plan", DraftID: "0123abcd", Workspace: `C:\Work`, Selection: "First paragraph.\n"}
	prompt := withDoc("Shorten this.", doc)
	for _, want := range []string{`"Plan" open in the Editor`, filepath.Join(`C:\Work`, "notes", "plan.md"), "````markdown\nFirst paragraph.\n"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("the prompt lacks %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(withDoc("Hi.", &DocRef{Path: "notes/plan.md", Title: "Plan", DraftID: "0123abcd"}), "selected") {
		t.Fatal("a draft with nothing selected mentions a selection")
	}
}
