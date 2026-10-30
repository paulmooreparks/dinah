package main

import (
	"bytes"
	"encoding/json"
	"maps"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Uploading copies files into a workspace folder: a new name is added, a
// name the folder has is replaced, kept beside it with a number, or skipped,
// as the person chose; a folder, a hidden name and a file dropped onto its
// own folder are not copied; and only a folder on the computer takes files.
func TestUploadToWorkspaceFolder(t *testing.T) {
	ws, from := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(ws, "Sources"), 0o755)
	os.WriteFile(filepath.Join(ws, "Sources", "notes.md"), []byte("old"), 0o644)
	write := func(name, text string) string {
		p := filepath.Join(from, name)
		os.WriteFile(p, []byte(text), 0o644)
		return p
	}
	notes, plan := write("notes.md", "new"), write("plan.docx", "plan")
	hidden := write(".secret", "x")
	folder := filepath.Join(from, "folder")
	os.MkdirAll(folder, 0o755)
	a := NewApp()
	record := newWorkspaceRecord(ws)
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	dir := ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "Sources"}
	read := func(name string) string { b, _ := os.ReadFile(filepath.Join(ws, "Sources", name)); return string(b) }

	taken, err := a.UploadConflicts(dir, []string{notes, plan, folder})
	if err != nil || !slices.Equal(taken, []string{"notes.md"}) {
		t.Fatalf("conflicts %v %v", taken, err)
	}
	r, err := a.UploadToWorkspaceFolder(dir, []string{notes, plan, folder, hidden}, "skip")
	if err != nil || !slices.Equal(r.Added, []string{"plan.docx"}) || !slices.Equal(r.Skipped, []string{"notes.md"}) || len(r.Refused) != 2 || read("notes.md") != "old" || read("plan.docx") != "plan" {
		t.Fatalf("skip %+v %v", r, err)
	}
	if r, _ = a.UploadToWorkspaceFolder(dir, []string{notes}, "keep-both"); !slices.Equal(r.Kept, []string{"notes (2).md"}) || read("notes (2).md") != "new" || read("notes.md") != "old" {
		t.Fatalf("keep both %+v", r)
	}
	if r, _ = a.UploadToWorkspaceFolder(dir, []string{notes}, "replace"); !slices.Equal(r.Replaced, []string{"notes.md"}) || read("notes.md") != "new" {
		t.Fatalf("replace %+v", r)
	}
	// A file dropped onto the folder it is already in stays as it is.
	own := filepath.Join(ws, "Sources", "plan.docx")
	if r, _ = a.UploadToWorkspaceFolder(dir, []string{own}, "replace"); !slices.Equal(r.Skipped, []string{"plan.docx"}) || read("plan.docx") != "plan" {
		t.Fatalf("own folder %+v", r)
	}
	if entries, _ := os.ReadDir(filepath.Join(ws, "Sources")); len(entries) != 3 {
		t.Fatalf("a temporary file was left behind: %v", entries)
	}
	if _, err := a.UploadToWorkspaceFolder(ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "Sources/plan.docx"}, []string{notes}, "skip"); err == nil {
		t.Fatal("a file took an upload as though it were a folder")
	}
	if p, err := a.WorkspaceLocalPath(dir); err != nil || p != filepath.Join(ws, "Sources") {
		t.Fatalf("path %q %v", p, err)
	}
}

// Files dropped from the system's file manager arrive as a multipart form
// and follow the same rules as files chosen by path: a new name is added, a
// name the folder has follows the choice, and a hidden name is refused.
func TestUploadDropped(t *testing.T) {
	ws := t.TempDir()
	os.MkdirAll(filepath.Join(ws, "Sources"), 0o755)
	os.WriteFile(filepath.Join(ws, "Sources", "notes.md"), []byte("old"), 0o644)
	a := NewApp()
	record := newWorkspaceRecord(ws)
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	dir := ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "Sources"}
	read := func(name string) string { b, _ := os.ReadFile(filepath.Join(ws, "Sources", name)); return string(b) }

	if taken, err := a.UploadNameConflicts(dir, []string{"notes.md", "plan.docx"}); err != nil || !slices.Equal(taken, []string{"notes.md"}) {
		t.Fatalf("conflicts %v %v", taken, err)
	}
	post := func(existing string, files map[string]string) UploadReport {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		ref, _ := json.Marshal(dir)
		mw.WriteField("folder", string(ref))
		mw.WriteField("existing", existing)
		for _, name := range slices.Sorted(maps.Keys(files)) {
			w, _ := mw.CreateFormFile("file", name)
			w.Write([]byte(files[name]))
		}
		mw.Close()
		req := httptest.NewRequest("POST", "/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		a.uploadDropped(rec, req)
		var r UploadReport
		if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		return r
	}
	r := post("keep-both", map[string]string{"notes.md": "new", "plan.docx": "plan", ".secret": "x"})
	if !slices.Equal(r.Added, []string{"plan.docx"}) || !slices.Equal(r.Kept, []string{"notes (2).md"}) || len(r.Refused) != 1 ||
		read("plan.docx") != "plan" || read("notes (2).md") != "new" || read("notes.md") != "old" {
		t.Fatalf("keep both %+v", r)
	}
	if r = post("replace", map[string]string{"notes.md": "newer"}); !slices.Equal(r.Replaced, []string{"notes.md"}) || read("notes.md") != "newer" {
		t.Fatalf("replace %+v", r)
	}
}
