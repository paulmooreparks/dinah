package main

import (
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A one-pixel PNG, for the pictures the media viewer opens.
const linkTestPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII="

func linkTestApp(t *testing.T) (*App, WorkspaceRecord, string, string) {
	t.Helper()
	isolateWorkspaceSettings(t)
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	dir, elsewhere := t.TempDir(), t.TempDir()
	png, _ := base64.StdEncoding.DecodeString(linkTestPNG)
	files := map[string][]byte{
		"plan.md": []byte("# Plan\n"), "main.go": []byte("package main\n"), "data.csv": []byte("a,b\n"),
		"picture.png": png, "drawing.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`),
		"report.pdf": []byte("%PDF-1.4\n"), "letter.docx": []byte("PK"), "sheet.xlsx": []byte("PK"), "tool.exe": []byte("MZ"),
		"big.txt": []byte(strings.Repeat("x", 1<<20+1)), "sub/inner.md": []byte("# Inner\n"),
	}
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(elsewhere, "notes.txt"), []byte("Loose notes.\n"), 0o644)
	a := NewApp()
	record := newWorkspaceRecord(dir)
	records := []WorkspaceRecord{record}
	if err := ensureWorkspaceMountAliases(records); err != nil {
		t.Fatal(err)
	}
	a.settings.WorkspaceRecords = records
	return a, records[0], dir, elsewhere
}

// Each kind of file opens in the part of the window the plan gives it: the
// Editor for the text it opens, in the view Settings chooses; the media
// viewer for pictures and SVG; Files for a folder in a workspace; and the
// system for PDF, Word and Excel files and for the rest.
func TestDocumentLinksOpenWhereTheirKindBelongs(t *testing.T) {
	a, record, dir, elsewhere := linkTestApp(t)
	cases := []struct {
		ref, opens, label, glyph, view string
		line                           int
		resource                       bool
	}{
		{"plan.md", opensEditor, "Open in the Editor", "document", "page", 0, true},
		{"main.go:42", opensEditor, "Open in the Editor", "document", "text", 42, true},
		{"main.go:42:7", opensEditor, "Open in the Editor", "document", "text", 42, true},
		{"main.go#L7-L9", opensEditor, "Open in the Editor", "document", "text", 7, true},
		{"picture.png", opensMedia, "Open in the Media Viewer", "image", "", 0, true},
		{"drawing.svg", opensMedia, "Open in the Media Viewer", "image", "", 0, true},
		{"report.pdf", opensSystem, "Open with the System's Program", "open", "", 0, true},
		{"letter.docx", opensSystem, "Open with the System's Program", "open", "", 0, true},
		{"sheet.xlsx", opensSystem, "Open with the System's Program", "open", "", 0, true},
		{"tool.exe", opensReveal, "Show in the System's File Manager", "open", "", 0, true},
		{"big.txt", opensSystem, "Open with the System's Program", "open", "", 0, true},
		{"sub", opensFiles, "Show in Files", "folder", "", 0, true},
		{filepath.Join(dir, "plan.md"), opensEditor, "Open in the Editor", "document", "page", 0, true},
		{"/" + filepath.ToSlash(filepath.Join(dir, "plan.md")), opensEditor, "Open in the Editor", "document", "page", 0, true},
		{filepath.Join(elsewhere, "notes.txt"), opensEditor, "Open in the Editor", "document", "text", 0, false},
		{(&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(filepath.Join(elsewhere, "notes.txt")), Fragment: "L3"}).String(), opensEditor, "Open in the Editor", "document", "text", 3, false},
		{elsewhere, opensReveal, "Show in the System's File Manager", "open", "", 0, false},
	}
	refs := make([]string, len(cases))
	for i, c := range cases {
		refs[i] = c.ref
	}
	got, err := a.ResolveDocumentLinks(dir, refs)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		l := got[i]
		if !l.Found || l.Opens != c.opens || l.Label != c.label || l.Glyph != c.glyph || l.View != c.view || l.Line != c.line || (l.Resource != nil) != c.resource {
			t.Errorf("%s: %+v", c.ref, l)
		}
		if c.resource && l.Resource != nil && l.Resource.WorkspaceID != record.ID {
			t.Errorf("%s: resource in the wrong workspace %+v", c.ref, l.Resource)
		}
	}
	if sub := got[11]; sub.MountPath == "" || !strings.HasSuffix(sub.MountPath, "/sub") || sub.Resource.Path != "sub" {
		t.Errorf("the folder has no place in Files: %+v", sub)
	}
	if plan := got[0]; plan.Resource.Path != "plan.md" || plan.MediaType != "text/markdown" {
		t.Errorf("plan.md: %+v", plan)
	}

	// The view is the one Settings chooses for the media type.
	if err := a.SetDocumentView("text/markdown", "split"); err != nil {
		t.Fatal(err)
	}
	if l, _ := a.ResolveDocumentLink(dir, "plan.md"); l.View != "split" {
		t.Errorf("the view Settings chose was not used: %+v", l)
	}
}

// A reference to nothing is not found, so the chat leaves it as text; a
// relative path needs the chat's folder.
func TestDocumentLinksToNothing(t *testing.T) {
	a, _, dir, _ := linkTestApp(t)
	got, _ := a.ResolveDocumentLinks(dir, []string{"missing.md", "docs/plans/none.md:4", "", "plan.md\nmore", "README.md"})
	for _, l := range got {
		if l.Found || l.Opens != "" || l.Label != "" {
			t.Errorf("%q was found: %+v", l.Ref, l)
		}
	}
	if l, _ := a.ResolveDocumentLink("", "plan.md"); l.Found {
		t.Errorf("a relative path was found with no folder: %+v", l)
	}
	if _, err := a.ResolveDocumentLinks(dir, make([]string, 201)); err == nil {
		t.Error("more than 200 references were taken at once")
	}
}

// A file whose name ends as a line suffix does is that file, not a line in
// another.
func TestDocumentLinkLineSuffixPrefersAWholeName(t *testing.T) {
	a, _, dir, _ := linkTestApp(t)
	os.WriteFile(filepath.Join(dir, "notes#L5"), []byte("x"), 0o644)
	if l, _ := a.ResolveDocumentLink(dir, "notes#L5"); !l.Found || l.Line != 0 || filepath.Base(l.Path) != "notes#L5" {
		t.Errorf("%+v", l)
	}
}

// A media type another part of the window has claimed opens there, before
// the Editor, and a file larger than the claim takes is left to the system.
func TestClaimedMediaTypeOpensInItsClaimant(t *testing.T) {
	a, _, dir, _ := linkTestApp(t)
	claimMediaType("text/csv", mediaClaim{Opens: "table-viewer", Label: "Open in the Table Viewer", Glyph: "table", Limit: 1 << 10})
	t.Cleanup(func() {
		mediaClaims.Lock()
		delete(mediaClaims.byType, "text/csv")
		mediaClaims.Unlock()
	})
	l, _ := a.ResolveDocumentLink(dir, "data.csv:3")
	if l.Opens != "table-viewer" || l.Label != "Open in the Table Viewer" || l.Glyph != "table" || l.Line != 0 {
		t.Errorf("the claim was not used: %+v", l)
	}
	os.WriteFile(filepath.Join(dir, "large.csv"), []byte(strings.Repeat("a,b\n", 1<<10)), 0o644)
	if l, _ := a.ResolveDocumentLink(dir, "large.csv"); l.Opens != opensEditor {
		t.Errorf("a file larger than the claim takes went to it: %+v", l)
	}
}

// A hidden file in a workspace's folder, which Files does not show, opens
// as a local file rather than as the workspace's resource.
func TestHiddenWorkspaceFileIsLocal(t *testing.T) {
	a, _, dir, _ := linkTestApp(t)
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755)
	os.WriteFile(filepath.Join(dir, ".hidden", "x.md"), []byte("# X\n"), 0o644)
	l, _ := a.ResolveDocumentLink(dir, ".hidden/x.md")
	if !l.Found || l.Opens != opensEditor || l.Resource != nil {
		t.Errorf("%+v", l)
	}
}

// The media viewer is given a picture as a data: address, and nothing that
// is not a picture it opens.
func TestReadLinkedImage(t *testing.T) {
	a, _, dir, _ := linkTestApp(t)
	src, err := a.ReadLinkedImage(dir, "picture.png")
	if err != nil || !strings.HasPrefix(src, "data:image/png;base64,") {
		t.Fatalf("%.40s %v", src, err)
	}
	if src, err := a.ReadLinkedImage(dir, "drawing.svg"); err != nil || !strings.HasPrefix(src, "data:image/svg+xml;base64,") {
		t.Fatalf("%.40s %v", src, err)
	}
	for _, ref := range []string{"plan.md", "report.pdf", "missing.png"} {
		if _, err := a.ReadLinkedImage(dir, ref); err == nil {
			t.Errorf("%s was read as a picture", ref)
		}
	}
	os.WriteFile(filepath.Join(dir, "fake.png"), []byte("not a picture"), 0o644)
	if _, err := a.ReadLinkedImage(dir, "fake.png"); err == nil {
		t.Error("a file that is not a picture was read as one")
	}
}
